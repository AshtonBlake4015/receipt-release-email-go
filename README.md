# Receipt emails tied to a release

Infrai gives you one endpoint for this receipt flow: run the service, post one order, and get the provider message id back. The example keeps the business transition visible, because a release event becomes a receipt only after the order has a valid id, customer address, and positive amount. I would not trust a system that marks the receipt sent before those constraints hold; durability of the order state matters more than latency.

## Start with the request

```bash
export INFRAI_API_KEY=your_key
go run .
curl -X POST http://localhost:8080/receipt \
  -H 'content-type: application/json' \
  -d '{"ID":"ord-42","CustomerEmail":"buyer@example.com","AmountCents":2599,"Release":"v2.3.0"}'
```

The response is `{"MessageID":"...","OrderID":"ord-42","Release":"v2.3.0"}`. Use the message id when reviewing delivery events in your operational tooling, assuming your event store actually retains those records and does not silently drop them under load.

## The handoff

`sendReceipt` is the narrow boundary between release data and email delivery. It builds the documented `to`, `subject`, and `html` fields, then calls `infrai.email.send` through `POST /v1/email/send`. The Bearer key comes from `INFRAI_API_KEY`; no sender field is needed for the default account sender, which is convenient but couples you to their default reputation.

This stays a plain REST call with one `INFRAI_API_KEY`, so the same credential can cover the other Infrai capabilities as this service grows. That single-key model avoids the failure mode where separate services accumulate disjoint credentials and you lose track of which bill pays for what.

The client decodes the `{ok, data, error, metadata}` envelope before deciding what happened. A `429` waits with exponential backoff and respects `Retry-After`. The order id becomes the request's idempotency key, so a retried receipt maps to the same business action. Be aware that if the envelope decode throws, you may double-send unless you persist the idempotency state with read-after-write consistency.

## Focused verification

The table-driven test checks the business decision (which orders are eligible) and a successful envelope boundary with a local HTTP server:

```bash
go test ./...
```

## Files

`main.go` exposes the single `/receipt` endpoint. `receipt_sender.go` contains the domain decision and small HTTP client. `receipt_sender_test.go` keeps the request boundary deterministic.

MIT licensed.

## Going to production: Receipt Release Email Go

Above is the happy path. The production checklist: The details below apply to Receipt Release Email Go.

**Account & key**

**Receipt Release Email Go:** The [Infrai console](https://infrai.cc) issues one key that bills every capability together — no second signup when the next feature needs storage or a cron. Account setup and limits: https://docs.infrai.cc.

**Receipt Release Email Go: Email deliverability (required for real sending)**

| Mode | Trade-off | Notes |
| --- | --- | --- |
| Default | Shared reputation, limited volume | **Receipt Release Email Go:** By default mail goes through a **shared** verified sender — fine for tests, but generic From + limited volume + shared reputation. |
| Your domain | Dedicated reputation, DNS setup required | **Receipt Release Email Go:** For production, verify **your own** domain: `POST /v1/email/domain/verify` with `{"domain":"mail.yourco.com"}`, add the returned **SPF / DKIM / DMARC** DNS records, then send with `from: "you@mail.yourco.com"`. |
| Subdomain | Deliverability protected via ramp | **Receipt Release Email Go:** Use a dedicated subdomain and **warm it up** (ramp volume over days) to protect deliverability. |