package main

import (
	"testing"
	"time"
)

func TestDeliveredShipmentWithProofGetsDelayedFollowup(t *testing.T) {
	now := time.Date(2026, 10, 23, 8, 0, 0, 0, time.UTC)
	event := ShipmentEvent{ShipmentID: "SHP-2048", Kind: "delivered", OccurredAt: now}
	proof := ProofOfDelivery{ShipmentID: "SHP-2048", ObjectKey: "proof/SHP-2048.jpg"}
	if !shouldFollowUp(event, proof) {
		t.Fatal("delivered shipment with proof should be scheduled")
	}
	wrongProof := ProofOfDelivery{ShipmentID: "SHP-9999", ObjectKey: "proof/SHP-9999.jpg"}
	if shouldFollowUp(event, wrongProof) {
		t.Fatal("proof for another shipment must not be scheduled")
	}
}
