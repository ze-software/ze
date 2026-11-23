# Spec: fleet-ui-5 -- Tasks, Alarms, Notifications and Live Updates

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | web, fleet |
| Priority | P3 |
| Release reason | Adds a capability absent from the supported release; existing supported behavior and its obligations remain required. |
| RFC gaps | - |
| Depends | spec-fleet-1-device-registry.md, spec-fleet-4-inventory-health.md |
| Phase | 5/6 |
| Handoff | - |
| Updated | 2026-11-23 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

**Re-read after compaction:** this file, `spec-fleet-ui-0-umbrella.md`,
`internal/component/web/sse.go`, `internal/core/report/report.go`.

## Task

Make what the hub is doing, and what is wrong in the estate, visible at all
times. Three things are specified here, each with the hub-side model it needs,
the commands that expose it on every command surface, and its pages:

1. **Tasks.** Every operation the hub performs on devices is a task an operator can watch, open and, where possible, cancel.
2. **Alarms.** Every condition that needs attention is listed once, with its device, its severity and since when.
3. **Live updates.** Trees, tables, cards, the task panel and toasts change as the estate changes, without a reload.

Nothing the hub does to a device is invisible, and nothing that is wrong has to
be discovered by opening devices one by one.

### Tasks

A task is one operator-started or hub-started operation on one or more devices.

| Field | Meaning |
|-------|---------|
| Identifier | Unique on the hub, shown in the monospace font, with a copy control |
| Kind | Configuration push, rollout batch, diagnostic run, divergence resolution, relayed command batch, software update |
| Actor | The hub user who started it, or the hub itself |
| Targets | The devices, and the estate or group node they were chosen from |
| State | Pending, Running, Succeeded, Failed, Cancelled, Partly failed |
| Progress | Devices finished over devices targeted |
| Started, ended | Times; duration once ended |
| Per-device result | For each target: state, the device's answer or refusal, duration |

Where tasks appear:

| Place | What it shows |
|-------|---------------|
| Header Tasks button | The count of running tasks; a dot in the error colour when a task failed since the panel was last opened |
| Task panel (opened from the button, 30rem wide, over the content) | Four tabs with counts: Running, Succeeded, Failed, All. Each task is a row with kind, target summary, a progress bar and the actor; it expands to its per-device results |
| Tasks tab of the estate, a group node and a device | The umbrella's table stack over the tasks that targeted that object: kind, targets, state word, progress, actor, started, duration; side panel with the per-device results |
| `/fleet/tasks/` | The same table over every task |
| Tree rows and object headers | A spinner while a task targets the object |
| Toasts | One when a task the operator started ends: its kind, its result, and a link to it |

| Rule | Detail |
|------|--------|
| Retention | Running tasks are always kept. Finished tasks are kept to a configured count, oldest dropped, and survive a hub restart |
| Restart | A task running when the hub stops is recorded as Failed with the reason "hub restarted", never left Running |
| Cancel | Offered while Pending or Running, for kinds that can stop. Devices already done keep their result |
| Retry | Offered on a Failed or Partly failed task: starts a new task over the failed devices only |
| Output | Each per-device result is bounded and escaped; a result too large is cut with a note saying so |
| Relation to the audit trail | The audit trail of `fleet-3` is the permanent record of who did what. A task is the live view of an operation and links to its audit entries |

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
specified in `plan/post-release/features/spec-fleet-ui-8-alarm-causes.md`.

Where alarms appear:

| Place | What it shows |
|-------|---------------|
| Alarms tab of the estate, a group node and a device | The umbrella's table stack: severity word with icon, device, alarm, detail, since, duration. Default order: severity, then oldest first. Filter fields: `severity`, `kind`, `device`, and the device fields of `fleet-ui-2` |
| Alarms card on dashboards | The most severe, with a link to the tab (`fleet-ui-3`) |
| Tree | A badge on a device row with its open alarm count; a group row's count turns to the warning colour |
| Toasts | One when a critical alarm opens on a device in the operator's current view |

A row's side panel shows the alarm's history on that device over the last week
and links to the device tab where the condition can be examined.

### Notifications

| Rule | Detail |
|------|--------|
| Levels | Informational, Success, Warning, Error, each with its icon, its word and its colour variable |
| Content | One line saying what happened, an optional second line, an optional link |
| Placement | Bottom right, stacked, newest lowest; at most four shown, the rest counted |
| Lifetime | Informational and Success leave after six seconds. Warning and Error stay until dismissed |
| Source | Only events about the operator's own actions or the operator's current view. A toast is never the only record of anything: every toast links to a task, an alarm or a page |

### Live updates

| Rule | Detail |
|------|--------|
| Stream | One event stream per open page, at `/fleet/events`, scoped to the view: the estate, a group node or a device |
| Events | Device state changed; alarm opened or cleared; task created, progressed or ended; registry changed (device added, removed, relabelled) |
| Effect | Each event names the fragments it makes stale. htmx refetches those fragments, so the browser holds no model of the estate |
| Authorization | A stream carries only what the operator's profile may see |
| Not the broadcast broker | The existing broker reaches every client. Fleet data uses a per-connection stream, as the L2TP quality feed does |
| Bounds | A stream that is not read is closed. Bursts are coalesced: many changes in one second become one event per fragment |
| Loss of the stream | The umbrella's stale banner appears with the time of the last event, and the page reconnects with backoff. On reconnect every live fragment is refetched once |
| Limit | A configured maximum of open fleet streams. Beyond it the page works without live updates and says so |

### Commands this spec adds

Every operator command must run on every surface, so the task and alarm lists
are commands first and pages second.

| Command | Answers |
|---------|---------|
| `show fleet task` | The task list, filterable by state, kind, actor and device |
| `show fleet task id <id>` | One task with its per-device results |
| `clear fleet task id <id>` | Cancels a pending or running task |
| `show fleet alarm` | Open alarms, filterable by severity, kind and device |
| `show fleet alarm history device <name>` | A device's alarm openings and clearings over the retention period |

### Key design decisions

| Decision | Detail |
|----------|--------|
| Alarms are derived, not stored events | The hub already holds every fact an alarm states. Deriving them means an alarm can never disagree with the device's state shown beside it |
| No acknowledgement | An acknowledged alarm is a second state to keep true. The first version shows what is wrong now; a fixed condition clears itself |
| Tasks are the hub's own model | Every fleet action creates a task through one entry point, so a new action cannot be invisible |
| Events name fragments | The server decides what is stale. The browser stays a renderer |
| One stream per page | A page's stream is scoped to what the page shows, which is also what the operator may see |

### Dependencies and readiness

| Dependency | State |
|------------|-------|
| Device registry and its change notifications | Missing: `fleet-1` |
| Health reports | Missing: `fleet-4` |
| Latest samples for the resource alarms | Missing: `fleet-8` |
| The actions that create tasks | `fleet-5` rollouts, `fleet-7` resolution, `fleet-9` relayed runs; each registers its task kind when it lands |
| The report bus and its bounded lists | Available on each device; the hub's own alarms use the same shape |
| A state store key for finished tasks | To be registered here |

### Draft acceptance criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Any fleet action on devices | A task exists for it from the moment it starts, visible in the panel, in the Tasks tab of each target and in `show fleet task` | Reads create no task, because they change nothing |
| AC-2 | A task over several devices where one fails | The task ends Partly failed, each device shows its own result, and Retry targets only the failed device | None |
| AC-3 | The hub restarts during a task | After restart the task shows Failed with the reason "hub restarted" | Devices that had finished keep their result |
| AC-4 | A condition in the alarm table becomes true, then false | The alarm opens, appears in every place listed, and clears from all of them | Its history row remains, because the history is the record that it happened |
| AC-5 | A device changes state while a page is open | The tree row, the table row and the cards that count it change without a reload, within the bound fixed at design | Pages the operator is not viewing do not update, because they have no stream |
| AC-6 | An operator who may not see a device | No event about it reaches the operator's stream | None |
| AC-7 | The stream is lost | The stale banner shows with the time, the page reconnects, and the live fragments refresh once | None |
| AC-8 | `show fleet task` and `show fleet alarm` on any command surface | The same data as the pages, as data answering every pipe | None |
