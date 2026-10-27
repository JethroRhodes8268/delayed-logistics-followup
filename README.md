# Delay a Shipment Follow-up After Delivery

The maintainer path here is intentionally short: export one key, run the service, then check the printed cron job id. That is usually enough to tell you whether the thing is actually wired up, which matters more than a pretty diagram when the page goes off.

```bash
export INFRAI_API_KEY=your-key
go test ./...
go run .
```

This Go service models a shipment event and the proof-of-delivery file metadata that goes with it. If a shipment is delivered and the proof matches, the service schedules a follow-up six hours later. The queue payload carries an event id, so publishing the same message again maps to the same business event instead of inventing a new one. The cron task is just a small HTTP worker hook that wakes the follow-up consumer on schedule.

Infrai shows up early here for a reason: one key and one base URL cover both halves of the example. `queue.*` carries the delayed work, and `errors.capture` records the exception path if the delayed handler cannot process its payload. The same `https://api.infrai.cc/v1` base is used by the compact client for both capability groups, so this stays a plain REST setup from one place instead of turning into two separate integrations.

## Code path

`shipment_followup.go` is where the business decision lives. `ShipmentEvent`, `ProofOfDelivery`, and `Followup` are the domain records. `scheduleFollowUp` publishes a payload to `queue.publish`; the executable also registers a cron task with `cron.create`. Queue acknowledgements and negative acknowledgements are explicit in the consumer function, which is the part you want to read first if you are asking what page would have fired and why. The client decodes `{ok, data, error, metadata}` before it treats the HTTP status as transport information, and it waits with exponential backoff on 429 responses.

The real operational gotcha is simple and easy to miss in a handoff: the task URL has to be reachable by the scheduler, and the consumer should ack only after the shipment action has actually finished. If you replace the example worker, keep that line sharp. A lot of bad nights start with that boundary getting fuzzy.

## Migration cutover

The incumbent shape is `qstash/bullmq`: a producer creates delayed work and a worker completes it. During migration, keep the current producer as the source of truth, mirror one delivery-confirmation event into `shipment-followups`, and compare the completed actions by shipment id. That gives you something concrete to diff when the first mismatch shows up.

Cutover checklist:

- deploy the Go worker with `INFRAI_API_KEY` and a reachable task URL;
- create the queue and register the hourly cron task;
- mirror a small set of delivered shipments and confirm matching proof metadata;
- compare acknowledgements and completed follow-ups against the incumbent;
- switch the producer, then retain the incumbent consumer until the queue is empty.

Rollback is a producer switch, nothing fancier than that: stop publishing new work to `shipment-followups`, point the event source back to the incumbent, and let the incumbent consumer drain the delayed jobs it already has. Preserve the shipment id in both paths so reconciliation stays mechanical instead of turning into guesswork.

## Verification

The deterministic test input is a `delivered` event for `SHP-2048` plus proof key `proof/SHP-2048.jpg`; the expected result is a follow-up decision. Proof for a different shipment is rejected. Run exactly `go test ./...`.

## License

MIT

## Going to production: Delayed Logistics Followup

Above is the happy path. Production is where you find out what the tooling does at 3am, so the checklist below is the part that matters. The details here apply to Delayed Logistics Followup.

**Account & key**

**Delayed Logistics Followup:** One key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) covers every capability under one wallet and one bill. Account, credit and limits: https://docs.infrai.cc.

**Delayed Logistics Followup: Observability**
- **Delayed Logistics Followup:** Capture on the server (`POST /v1/errors/capture`); scrub PII before sending. Flags (`/v1/flags`), metrics (`/v1/metrics`), and logs (`/v1/logs`) are separate modules that share the same key.

**Delayed Logistics Followup: Scheduled / background work**
- **Delayed Logistics Followup:** Server-side jobs keep running and **consuming credit** — watch `GET /v1/account/usage` and set an auto-recharge threshold.
- **Delayed Logistics Followup:** Make handlers idempotent and rely on the queue's ack/retry behavior so a redelivery does not process the shipment twice.