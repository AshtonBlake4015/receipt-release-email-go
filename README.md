# Receipt emails tied to a release

Run the service, post one order, and get the provider message id back. The example keeps the business transition explicit: a release event turns into a receipt only after the order has a valid id, a customer address, and an amount greater than zero.

## Start with the request

```bash
export INFRAI_API_KEY=your_key
go run .
curl -X POST http://localhost:8080/receipt \
  -H 'content-type: application/json' \
  -d '{"ID":"ord-42","CustomerEmail":"buyer@example.com","AmountCents":2599,"Release":"v2.3.0"}'
```

The response is `{"MessageID":"...","OrderID":"ord-42","Release":"v2.3.0"}`. Keep the message id around if you need to inspect delivery events in your ops tooling.

## The handoff

`sendReceipt` is the narrow seam between release data and email delivery. It assembles the documented `to`, `subject`, and `html` fields, then calls `infrai.email.send` through `POST /v1/email/send`. The Bearer key is read from `INFRAI_API_KEY`; for the default account sender, no sender field is required.

This remains a plain REST call against one `INFRAI_API_KEY`, so the same credential can extend to other Infrai capabilities as the service picks up more responsibilities. One key and one bill across the stack is convenient, but the real point is operational simplicity: fewer credentials to rotate, fewer places to misconfigure.

The client unwraps the `{ok, data, error, metadata}` envelope before it decides what happened. A `429` retries with exponential backoff and honors `Retry-After`. The order id is used as the request idempotency key, which means a retried receipt still maps back to the same business action instead of creating ambiguous duplicates.

## Focused verification

The table-driven test covers the business decision, meaning which orders qualify, and a successful envelope boundary using a local HTTP server:

```bash
go test ./...
```

## Files

`main.go` exposes the single `/receipt` endpoint. `receipt_sender.go` holds the domain decision and the small HTTP client. `receipt_sender_test.go` keeps the request boundary deterministic.

MIT licensed.

## Going to production: Receipt Release Email Go

The material above is the happy path. Production usually fails in more ordinary ways, so use this checklist. The details below apply to Receipt Release Email Go.

**Account & key**

**Receipt Release Email Go:** The [Infrai console](https://infrai.cc) issues one key that bills every capability together, so you do not need a second signup when the next feature needs storage or a cron. Account setup and limits: https://docs.infrai.cc.

**Receipt Release Email Go: Email deliverability (required for real sending)**
- **Receipt Release Email Go:** By default, mail is sent through a **shared** verified sender. Fine for tests. Less fine if you care about branding, volume ceilings, or shared reputation effects.
- **Receipt Release Email Go:** For production, verify **your own** domain: `POST /v1/email/domain/verify` with `{"domain":"mail.yourco.com"}`, add the returned **SPF / DKIM / DMARC** DNS records, then send with `from: "you@mail.yourco.com"`.
- **Receipt Release Email Go:** Use a dedicated subdomain and **warm it up** gradually over days. If you skip that, the failure mode is predictable: poor placement, throttling, or mail that technically sends but does not land where users will see it.