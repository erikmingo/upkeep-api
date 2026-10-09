# API v1

Base: `http://<host>:8099`. JSON in and out. Dev auth: `X-Home: <home id>` on every `/v1/*` request (see README). Errors: `{"error": "message"}` with 400 / 401 / 404 / 500. Dates are `YYYY-MM-DD` (UTC days).

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
Body optional: `{"member_id":1}`; defaults to the home's first member until sessions exist. Returns the task with its new `next_due` and `last_completed`. 404 for another home's task or an archived one; 400 for a member not in this home.

## POST /v1/tasks
Hand-added task, not tied to a rule. `season_start`/`season_end` both `MM-DD` or both absent.
```json
{"title":"Fix the gate latch","interval_days":365,"detail":"","season_start":null,"season_end":null}
```
→ 201 with the task shape above.
