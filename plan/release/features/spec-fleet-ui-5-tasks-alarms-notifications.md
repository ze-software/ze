# Spec: fleet-ui-5 -- The Hub Alarm List and Its Commands

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | fleet, cli |
| Priority | P2 |
| Release reason | Thomas put the fleet alarm list in the first release, reached through its commands on every command surface (`ai/META-DECISIONS.md`, "Goal and scope"); existing supported behavior and its obligations remain required. Its pages, the hub's task model and panel, notifications and live page updates come after the first release, in `plan/post-release/features/spec-fleet-ui-9-tasks-notifications-and-alarm-pages.md`. |
| RFC gaps | - |
| Depends | plan/release/features/spec-fleet-1-device-registry.md, plan/release/features/spec-fleet-4-inventory-health.md, plan/release/features/spec-fleet-8-metric-history.md |
| Phase | 5/9 of plan/post-release/features/spec-fleet-ui-0-umbrella.md |
| Handoff | - |
| Updated | 2026-11-23 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

**Re-read after compaction:** this file, `plan/post-release/features/spec-fleet-ui-0-umbrella.md`,
`plan/post-release/features/spec-fleet-ui-9-tasks-notifications-and-alarm-pages.md`,
`internal/core/report/report.go`.

## Task

Make what is wrong in the estate visible at all times. Every condition that
needs attention is listed once, with its device, its severity and since when,
by the hub's alarm model and the commands that expose it on every command
surface: the CLI, MCP, REST, gRPC, SSH and the web command surface. Nothing
that is wrong has to be discovered by opening devices one by one.

On 2026-11-23 Thomas ruled that the fleet features of the first release are
reached through commands, and that their web pages, the hub's task model and
panel, notifications and live page updates come after it (`ai/META-DECISIONS.md`,
"Goal and scope"). This spec was written with all of those; it keeps the
alarm model and its commands, and the rest moved, unchanged, to
`plan/post-release/features/spec-fleet-ui-9-tasks-notifications-and-alarm-pages.md`
(see "Moved to fleet-ui-9" below).

### Alarms

Ze has no alarm manager. An alarm here is a condition that is true now, derived
by the hub from what it already holds. It opens when the condition becomes true
and clears when it stops being true. Nothing is acknowledged or silenced.

| Alarm | Opens when | Severity |
|-------|-----------|----------|
| Device offline | A device has had no managed connection for longer than a configured time | Critical |
| Component down | A device's health report names a component down | Critical |
| Component degraded | A device's health report names a component degraded | Warning |
| Configuration diverged | A device holds a locally edited configuration (`fleet-6`) | Warning |
| Configuration rejected | A device refused the configuration the hub sent | Warning |
| Resource high | A reported CPU, memory or storage figure stays above a configured threshold for a configured time | Warning |
| Software behind | A device runs a version older than a configured number of versions behind the newest in the estate | Informational |
| Relay not granted | A device grants no relay profile, so its tabs cannot show live state | Informational |
| History full | The metric history reached its size cap and is dropping samples early | Warning |

The list is open: an alarm kind is a registered entry with its condition, its
severity and its sentence, so a later feature adds one without editing the list.

Alarm causes, which hide an alarm while the alarm that causes it is open, are
specified in `plan/release/features/spec-fleet-ui-8-alarm-causes.md`.

Where the alarms appear on the web (the Alarms tabs, the Alarms card, the tree
badges, the toasts and a row's side panel) is
`plan/post-release/features/spec-fleet-ui-9-tasks-notifications-and-alarm-pages.md`,
"Alarm pages".

### Commands this spec adds

Every operator command must run on every surface, so the alarm list is a
command first.

| Command | Answers |
|---------|---------|
| `show fleet alarm` | Open alarms, filterable by severity, kind and device |
| `show fleet alarm history device <name>` | A device's alarm openings and clearings over the retention period |

The task commands (`show fleet task`, `show fleet task id <id>`,
`clear fleet task id <id>`) moved with the task model to `fleet-ui-9`.

### Key design decisions

| Decision | Detail |
|----------|--------|
| Alarms are derived, not stored events | The hub already holds every fact an alarm states. Deriving them means an alarm can never disagree with the device's state shown beside it |
| No acknowledgement | An acknowledged alarm is a second state to keep true. The first version shows what is wrong now; a fixed condition clears itself |

The decisions "Tasks are the hub's own model", "Events name fragments" and
"One stream per page" moved to `fleet-ui-9`.

### Dependencies and readiness

| Dependency | State |
|------------|-------|
| Device registry and its change notifications | Missing: `fleet-1` |
| Health reports | Missing: `fleet-4` |
| Latest samples for the resource alarms | Missing: `fleet-8` |
| The report bus and its bounded lists | Available on each device; the hub's own alarms use the same shape |

The task dependencies (the actions that create tasks, the state store key for
finished tasks) moved to `fleet-ui-9`.

### Draft acceptance criteria

The identifiers are those this spec was written with, so the references other
specs hold stay true. AC-1, AC-2, AC-3, AC-5, AC-6 and AC-7 moved whole to
`fleet-ui-9`; AC-4 and AC-8 keep their command part here and their page part
moved (table below).

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-4 | A condition in the alarm table becomes true, then false | The alarm opens in `show fleet alarm` with its kind, device, severity and since, then clears from it; `show fleet alarm history device <name>` lists its opening and its clearing | Its history row remains, because the history is the record that it happened |
| AC-8 | `show fleet alarm` and `show fleet alarm history device <name>` on every command surface: the CLI, MCP, REST, gRPC, SSH and the web command surface | The same alarms on every surface, as data answering every pipe | None |

### Moved to fleet-ui-9

Every item below is owned by
`plan/post-release/features/spec-fleet-ui-9-tasks-notifications-and-alarm-pages.md`,
which names this spec as the source of each.

| Item moved | Was here | Now in `fleet-ui-9` |
|------------|----------|---------------------|
| AC-1, a task exists for every fleet action | AC-1 | AC-1 |
| AC-2, a task that partly fails | AC-2 | AC-2 |
| AC-3, a task across a hub restart | AC-3 | AC-3 |
| AC-4, the alarm appears in and clears from every page place | AC-4, page part | AC-4 |
| AC-5, live change without a reload | AC-5 | AC-5 |
| AC-6, no event about a device the operator may not see | AC-6 | AC-6 |
| AC-7, the stream is lost | AC-7 | AC-7 |
| AC-8, `show fleet task` on every command surface | AC-8, task part | AC-8 |
| AC-8, the commands answer the same data as the pages | AC-8, the "same data as the pages" clause for alarms | AC-9 |
| The task model, its kinds, its places and its rules | "Tasks" | "Tasks" |
| Where alarms appear on the web, and the side panel's week of history | "Alarms", "Where alarms appear" | "Alarm pages" |
| Notifications | "Notifications" | "Notifications" |
| Live updates | "Live updates" | "Live updates" |
| `show fleet task`, `show fleet task id <id>`, `clear fleet task id <id>` | "Commands this spec adds" | "Commands this spec adds" |
| Decisions "Tasks are the hub's own model", "Events name fragments", "One stream per page" | "Key design decisions" | "Key Design Decisions" |
| The actions that create tasks; a state store key for finished tasks | "Dependencies and readiness" | "Dependencies and readiness" |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| The task model, notifications, live updates and the alarm pages, with the ACs in "Moved to fleet-ui-9" | Thomas ruled on 2026-11-23 that they come after the first release (`ai/META-DECISIONS.md`, "Goal and scope") | `plan/post-release/features/spec-fleet-ui-9-tasks-notifications-and-alarm-pages.md` |
