package main

import (
	"encoding/json"
	"fmt"
	"time"

	"delayed-logistics-followup/infrai"
)

const followupQueue = "shipment-followups"

type ShipmentEvent struct {
	ShipmentID string    `json:"shipment_id"`
	Kind       string    `json:"kind"`
	OccurredAt time.Time `json:"occurred_at"`
}

type ProofOfDelivery struct {
	ShipmentID string `json:"shipment_id"`
	ObjectKey  string `json:"object_key"`
	MediaType  string `json:"media_type"`
}

type Followup struct {
	ShipmentID string    `json:"shipment_id"`
	Action     string    `json:"action"`
	RunAfter   time.Time `json:"run_after"`
}

type Exception struct {
	Message string         `json:"message"`
	Context map[string]any `json:"context,omitempty"`
}

func shouldFollowUp(event ShipmentEvent, proof ProofOfDelivery) bool {
	return event.Kind == "delivered" && proof.ShipmentID == event.ShipmentID && proof.ObjectKey != ""
}

func scheduleFollowUp(client *infrai.Client, event ShipmentEvent, proof ProofOfDelivery, now time.Time) error {
	if !shouldFollowUp(event, proof) {
		return nil
	}
	followup := Followup{ShipmentID: event.ShipmentID, Action: "send-delivery-confirmation", RunAfter: now.Add(6 * time.Hour)}
	return client.QueuePublish(followupQueue, followup)
}

func handleDelayedFollowup(client *infrai.Client, message infrai.Message) error {
	var followup Followup
	if err := json.Unmarshal(message.Payload, &followup); err != nil {
		_ = client.CaptureException(Exception{Message: err.Error(), Context: map[string]any{"shipment_id": "unknown"}})
		_ = client.QueueNack(followupQueue, message.MessageID)
		return err
	}
	if followup.Action != "send-delivery-confirmation" {
		_ = client.CaptureException(Exception{Message: "delayed follow-up action could not be processed", Context: map[string]any{"shipment_id": followup.ShipmentID}})
		_ = client.QueueNack(followupQueue, message.MessageID)
		return fmt.Errorf("unknown follow-up action %q", followup.Action)
	}
	if err := client.QueueAck(followupQueue, message.MessageID); err != nil {
		return err
	}
	return nil
}

func main() {
	client, err := infrai.NewFromEnvironment()
	if err != nil {
		panic(err)
	}
	if err := client.QueueCreate(followupQueue); err != nil {
		panic(err)
	}
	event := ShipmentEvent{ShipmentID: "SHP-2048", Kind: "delivered", OccurredAt: time.Now().UTC()}
	proof := ProofOfDelivery{ShipmentID: event.ShipmentID, ObjectKey: "proof/SHP-2048.jpg", MediaType: "image/jpeg"}
	if err := scheduleFollowUp(client, event, proof, time.Now().UTC()); err != nil {
		panic(err)
	}
	messages, err := client.QueueConsume(followupQueue, 1)
	if err != nil {
		panic(err)
	}
	for _, message := range messages {
		if err := handleDelayedFollowup(client, message); err != nil {
			panic(err)
		}
	}
	jobID, err := client.CronCreate("0 * * * *", "https://example.com/internal/shipment-followups")
	if err != nil {
		panic(err)
	}
	fmt.Println("scheduled shipment follow-up with cron job", jobID)
}
