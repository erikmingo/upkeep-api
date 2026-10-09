# API v1

Base: `http://<host>:8099`. JSON in and out. Auth: `Authorization: Bearer <session_token>` on every `/v1/*` request except `/v1/auth/*`; 401 means sign in again. Errors: `{"error": "message"}` with 400 / 401 / 404 / 500. Dates are `YYYY-MM-DD` (UTC days).

## POST /v1/auth/magic-link
`{"email":"you@example.com"}` → 202 `{"status":"sent"}` whether or not the address is known. The email carries a one-time code (15 minutes). Without `RESEND_API_KEY` the API logs it instead.

## POST /v1/auth/verify
`{"token":"<code>"}` (or `GET /v1/auth/verify?token=`) → 200
```json
{"session_token":"…","user":{"id":1,"email":"you@example.com","display_name":"you"},"home":{"id":1,"name":"My home"}}
```
A new address gets a user and an empty home. Session tokens last 30 days; 401 with a message on a used, expired or unknown code.

## POST /v1/auth/logout
Bearer session → 204; the token stops working.

## GET /health
`{"status":"ok","service":"upkeep-api","db":"ok"}` or 503 with `"status":"degraded","db":"unreachable"`.

## GET /v1/home
```json
{"id":1,"name":"1201 Newton St",
 "facts":{"furnace.type":"gas","home.year_built":1925,"gutters.present":true, "...": "..."},
 "members":[{"id":1,"email":"amanda.sanders7191@example.com","display_name":"Amanda Sanders"}]}
```
Fact values are typed JSON (string, number, boolean) keyed by the taxonomy in `docs/spec-home-facts.md`.

## PUT /v1/home/facts
Upserts the given facts and re-materializes tasks in one transaction. Unknown keys or wrong value types → 400, nothing written.
```json
{"facts":{"water_heater.type":"tankless","home.beds":3}}
```
→ `{"facts":2,"tasks_created":1,"tasks_archived":2}`

## GET /v1/tasks
Active tasks, soonest `next_due` first; overdue ones come first by construction.
```json
{"tasks":[
 {"id":12,"rule_id":"furnace.filter.change","title":"Change furnace filter","detail":"Size 16x25x1. Arrow points toward the furnace.",
  "interval_days":90,"season_start":null,"season_end":null,"next_due":"2026-09-29","overdue":true,"last_completed":"2026-07-01"},
 {"id":30,"rule_id":null,"title":"Fix the gate latch","detail":"","interval_days":365,"season_start":null,"season_end":null,
  "next_due":"2026-10-09","overdue":false,"last_completed":null}
]}
```

## POST /v1/tasks/{id}/complete
The completing member is the signed-in user. Returns the task with its new `next_due` and `last_completed`. 404 for another home's task or an archived one.

## POST /v1/tasks
Hand-added task, not tied to a rule. `season_start`/`season_end` both `MM-DD` or both absent.
```json
{"title":"Fix the gate latch","interval_days":365,"detail":"","season_start":null,"season_end":null}
```
→ 201 with the task shape above.
