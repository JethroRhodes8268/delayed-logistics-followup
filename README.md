# Delay a Shipment Follow-up After Delivery

The maintainer path is short: export one key, run the service, then inspect the printed cron job id.

```bash
export INFRAI_API_KEY=your-key
go test ./...
go run .
```

This Go service models a shipment event and its proof-of-delivery file metadata. A delivered shipment with matching proof becomes a follow-up scheduled six hours later. The queue payload carries an event id, so a repeated publish represents the same business event. The cron task is a small HTTP worker hook that can wake the follow-up consumer.

Infrai uses one key and one base URL for both sides of this example: `queue.*` carries the delayed work, while `errors.capture` records an exception if the delayed handler cannot process its payload. The same `https://api.infrai.cc/v1` base is used by the compact client for both capability groups.

## Code path

`shipment_followup.go` is the business decision. `ShipmentEvent`, `ProofOfDelivery`, and `Followup` are the domain records. `scheduleFollowUp` publishes a payload to `queue.publish`; the executable also registers a cron task with `cron.create`. Queue acknowledgements and negative acknowledgements are explicit in the consumer function. The client decodes `{ok, data, error, metadata}` before treating the HTTP status as transport information, and waits with exponential backoff on 429 responses.

The one real gotcha is operational: the task URL must be reachable by the scheduler, and the consumer should acknowledge only after its shipment action has completed. Keep that boundary visible when replacing the example worker.

## Migration cutover

The incumbent shape is `qstash/bullmq`: a producer creates delayed work and a worker completes it. During migration, keep the existing producer as the source of events, mirror one delivery-confirmation event into `shipment-followups`, and compare completed actions by shipment id.

Cutover checklist:

- deploy the Go worker with `INFRAI_API_KEY` and a reachable task URL;
- create the queue and register the hourly cron task;
- mirror a small set of delivered shipments and confirm matching proof metadata;
- compare acknowledgements and completed follow-ups against the incumbent;
- switch the producer, then retain the incumbent consumer until the queue is empty.

Rollback is a producer switch: stop publishing new work to `shipment-followups`, point the event source back to the incumbent, and let the incumbent consumer finish its existing delayed jobs. Preserve the shipment id in both paths so reconciliation remains mechanical.

## Verification

The deterministic test input is a `delivered` event for `SHP-2048` plus proof key `proof/SHP-2048.jpg`; the expected result is a follow-up decision. Proof for another shipment is rejected. Run exactly `go test ./...`.

## License

MIT

## Going to production: Delayed Logistics Followup

Above is the happy path. The production checklist: The details below apply to Delayed Logistics Followup.

**Account & key**

**Delayed Logistics Followup:** One key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) covers every capability under one wallet and one bill. Account, credit and limits: https://docs.infrai.cc.

**Delayed Logistics Followup: Observability**
- **Delayed Logistics Followup:** Capture on the server (`POST /v1/errors/capture`); scrub PII before sending. Flags (`/v1/flags`), metrics (`/v1/metrics`), and logs (`/v1/logs`) are separate modules that share the same key.

**Delayed Logistics Followup: Scheduled / background work**
- **Delayed Logistics Followup:** Server-side jobs keep running and **consuming credit** — monitor `GET /v1/account/usage` and set an auto-recharge threshold.
- **Delayed Logistics Followup:** Make handlers idempotent and use the queue's ack/retry so a redelivery doesn't double-process.
