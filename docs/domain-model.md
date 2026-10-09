# Domain model

## Terminology Glossary

| Term | Meaning |
|---|---|
| **Home** | The physical place being kept up. Owns Facts and Tasks. One Home per household in v1; a User may belong to several Homes later. _Avoid_: house, property, household (household is the people, see Member). |
| **Member** | A User's membership in a Home. Any Member can complete any Task. _Avoid_: owner, tenant, assignee. |
| **Fact** | One answer about the Home from the wizard: `furnace.type = gas`, `roof.material = asphalt`, `water_heater.installed = 2019`. Keyed by a stable path, typed value, source (wizard, user edit, inferred). _Avoid_: attribute, property, feature. |
| **Rule** | A template in the rule library: "when these Facts hold, this Task exists with this default interval and season". Content, not code; versioned seed data. _Avoid_: template, recipe. |
| **Task** | A recurring chore on a Home, materialized from a Rule (or added by hand). Carries title, interval, optional season window, the Rule it came from. _Avoid_: chore, todo, reminder (a Reminder is the notification about a Task). |
| **Completion** | A record that a Member did a Task at a time. Next-due = latest Completion + interval; no Completion → due from the Task's start date. _Avoid_: done, check-off. |
| **Reminder** | The local notification scheduled on the phone for a Task's next-due. Not stored server-side in v1. |
| **Next-due** | Derived, never stored: `coalesce(last completion, task start) + interval`, clamped into the season window when one exists. |

## Core loop

```
wizard answers → Facts on the Home → Rules match Facts → Tasks materialize → list sorted by next-due
                                                               ↑                     │
                                                               └── Completion ───────┘
```

Editing a Fact (new water heater) re-evaluates Rules: new Tasks appear, obsolete ones are archived, existing ones keep their history.
