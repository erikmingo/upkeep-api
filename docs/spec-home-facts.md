# Spec: home facts drive upkeep

Status: reviewed 2026-10-09. Vocabulary: `domain-model.md`.

## Problem

Nobody knows what their house needs. Blank-list chore apps make the user author the schedule, which is the hard part. Upkeep asks about the home instead, the way an iBuyer form or a Zillow "Facts & features" page does, and derives the schedule from the answers.

## Shape

1. **Wizard** collects Facts, one tap per question, grouped in sections. "Not sure" picks the common default for the home's type and age. About 25 questions; sections can be skipped and returned to.
2. **Rule library** maps Facts to Tasks. Rules are data (YAML in the repo, loaded into the DB by `cmd/seed` and a loader), reviewed in PRs like code.
3. **Materialization** runs when Facts change: for every Rule whose condition matches, ensure one Task exists on the Home; archive Tasks whose Rule no longer matches.
4. **List** shows Tasks sorted by next-due, overdue first. Any Member taps done → Completion → next-due moves.

## Fact taxonomy (v1)

Only facts with a Task consequence. Keys are dotted paths; values are enums, ints or dates.

| Section | Keys | Unlocks |
|---|---|---|
| basics | `home.type` (house, condo, townhouse), `home.year_built`, `home.stories`, `home.sqft`, `home.beds`, `home.baths` | type gates every section; year hints at old plumbing and wiring; sqft/beds/baths are profile facts with no task consequence in v1 |
| hvac | `furnace.type` (gas, electric, heat_pump, boiler, none), `furnace.filter_size`, `ac.type` (central, window, mini_split, none), `humidifier.present` | filter 1–3 mo, furnace service yearly, AC coil clean, boiler bleed |
| water | `water_heater.type` (tank, tankless, none), `water_heater.installed`, `softener.present`, `water.source` (city, well), `sewer.type` (sewer, septic), `sump_pump.present` | tank flush yearly, anode 3 yr, softener salt monthly, septic pump 3–5 yr, sump test spring, well test yearly |
| exterior | `roof.material` (asphalt, metal, tile, flat), `roof.installed`, `gutters.present`, `gutters.guards`, `deck.material` (wood, composite, none) | gutter clean spring + fall, roof inspection, deck seal 2 yr |
| outdoor | `lawn.present`, `sprinkler.present`, `trees.near_house`, `pool.type` (none, pool, hot_tub) | sprinkler blowout before freeze, tree trim, pool chemistry weekly in season |
| safety | `detectors.smoke_count`, `detectors.co_count`, `extinguisher.present`, `dryer.vent_length` (short, long), `radon.last_test` | battery swap, extinguisher check, dryer vent clean yearly, radon retest 2 yr |
| appliances | `fridge.water_filter`, `dishwasher.present`, `disposal.present`, `range_hood.present` | fridge filter 6 mo, dishwasher filter, hood filter degrease |
| garage | `garage.door_opener`, `garage.present` | door balance test, lubricate |

## Rule format

```yaml
- id: furnace.filter.change
  when: { furnace.type: [gas, electric, heat_pump] }
  task:
    title: Change furnace filter
    interval: 90d
    detail: "Size {{furnace.filter_size}}. Arrow points toward the furnace."
- id: sprinkler.blowout
  when: { sprinkler.present: true }
  task:
    title: Blow out sprinkler lines
    interval: 365d
    season: { start: 10-01, end: 11-15 }
```

`when` is an AND of key → allowed values; an empty `when` matches every home (detector batteries). No expressions in v1; anything that needs one becomes two rules (`dryer.vent.clean.long` / `.short`). Rule ids are stable; Tasks reference them. The library lives in `internal/rules/rules.yaml`; the loader rejects unknown keys and values against `internal/rules/taxonomy.go`.

## Worked example: 1920s brick house, Denver

Facts: house, 1925, 1 story, 1,100 sqft, 2 bed / 1 bath, gas furnace 16x25x1, central AC, tank water heater 2019, city water, sewer, no sump, asphalt roof 2015, gutters no guards, wood deck, lawn + sprinkler, big tree near house, 3 smoke / 1 CO, long dryer vent, gas range, garage with opener.

Materialized (23 tasks, as `internal/rules/rules.yaml` computes it; `TestDenverMatchesExactly` pins the list): furnace filter 90d · furnace service yearly (Sep–Oct) · AC coil clean yearly (May) · water heater flush yearly · anode rod 3 yr · roof inspection yearly · gutter clean spring (Apr) · gutter clean fall (Nov) · deck seal 2 yr (Jun–Aug) · sprinkler blowout (Oct) · sprinkler startup (Apr–May) · tree trim yearly · lawn aerate (Sep) · detector batteries 6 mo · CO detectors replace 7 yr · smoke detectors replace 10 yr · dryer vent clean yearly · radon test 2 yr · dishwasher filter 90d · disposal clean 30d · hood filter 90d · garage door balance 6 mo · garage door lube yearly.

No softener salt, no septic, no pool, no fridge filter: the facts said so.

## Out of scope for v1

Server push (reminders are local), assignment and rotation, cost tracking, contractor contacts, photos, multiple homes per user, rule expressions, climate zones (seasonal windows are fixed months in v1; zip → climate comes later), siding and fence facts, vehicles, interior facts (flooring, fireplace, basement).

## Tickets

1. Glossary + this spec (this PR).
2. Schema: homes, members, facts, rules, tasks, completions; seed with the Denver example.
3. Rule library v1 as YAML + loader; the ~40 rules behind the taxonomy table.
4. Materializer: facts → tasks, idempotent, archives non-matching; next-due query.
5. API: home facts (get/put), tasks list sorted by next-due, complete task; dev `X-Home` header until auth.
6. iOS: project skeleton on Xcode 26 (existing ios#1).
7. iOS: wizard, one question per screen, sections, "not sure" defaults.
8. iOS: task list, overdue first, mark done.
9. API: magic-link auth (Resend behind a Mailer interface, log adapter in dev); iOS sign-in.
10. Invite a member by link.
