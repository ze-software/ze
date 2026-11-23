# Spec: fleet-ui-9 -- Tasks, Notifications, Live Updates and the Alarm Pages

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | web, fleet, cli |
| Priority | P3 |
| Release reason | Thomas ruled on 2026-11-23 that the fleet features of the first release are reached through commands on every command surface, and that their web pages, the hub's task model and panel, notifications and live page updates come after the first release (`ai/META-DECISIONS.md`, "Goal and scope"). Split out of `plan/release/features/spec-fleet-ui-5-alarm-list.md` and `plan/release/features/spec-fleet-ui-8-alarm-causes.md`, which keep the alarm model and its commands; existing supported behavior and its obligations remain required. |
| RFC gaps | - |
| Depends | plan/release/features/spec-fleet-ui-5-alarm-list.md, plan/release/features/spec-fleet-ui-8-alarm-causes.md, plan/post-release/features/spec-fleet-ui-2-estate-tree-and-device-tables.md, plan/release/features/spec-fleet-1-device-registry.md, plan/release/features/spec-fleet-4-inventory-health.md, plan/release/features/spec-fleet-8-metric-history.md |
| Phase | 9/9 of plan/post-release/features/spec-fleet-ui-0-umbrella.md |
| Handoff | - |
| Updated | 2026-11-23 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

**Re-read after compaction:** this file, `plan/post-release/features/spec-fleet-ui-0-umbrella.md`,
`plan/release/features/spec-fleet-ui-5-alarm-list.md`,
`plan/release/features/spec-fleet-ui-8-alarm-causes.md`,
`docs/architecture/fleet-interface.md`, `internal/component/web/sse.go`.

## Task

Make what the hub is doing, and what is wrong in the estate, visible on the
fleet pages at all times. Nothing the hub does to a device is invisible, and
nothing that is wrong has to be discovered by opening devices one by one.

This spec was cut out of `fleet-ui-5` and `fleet-ui-8` on 2026-11-23, when
Thomas ruled that the fleet features of the first release are reached through
commands on every command surface, and that their web pages, the hub's task
model and panel, notifications and live page updates come after it
(`ai/META-DECISIONS.md`, "Goal and scope"). Every section below was moved
unchanged; the table "Where each part came from" names its source. The alarm
model, its kinds, `show fleet alarm`, `show fleet alarm history`, the alarm
causes, their refusals, `hidden-behind` and the `hidden` filter stay in the
two release specs. This spec shows them on the pages and reads them from the
hub: it computes no alarm and no hiding of its own.

It holds four things, each with the hub-side model it needs, the commands that
expose it on every command surface, and its pages:

1. **Tasks.** Every operation the hub performs on devices is a task an operator can watch, open and, where possible, cancel.
2. **Alarm pages.** The alarms `fleet-ui-5` derives, and the hiding `fleet-ui-8` computes, shown on the Alarms tabs, the Alarms card, the tree badges, the toasts and a row's side panel.
3. **Notifications.** Toasts about the operator's own actions and current view.
4. **Live updates.** Trees, tables, cards, the task panel and toasts change as the estate changes, without a reload.

### Where each part came from

| Part | Source spec | Source section or AC |
|------|-------------|----------------------|
| Tasks, its places and its rules | `fleet-ui-5` | "Tasks" |
| Alarm pages | `fleet-ui-5` | "Alarms", "Where alarms appear" and the side panel paragraph |
| Alarm causes on the pages | `fleet-ui-8` | The page rows of "Where a hidden alarm appears", the "What the operator is told" column of the move table, the count rule, the `has:cause` and Alarms card rows of "What this spec changes in `fleet-ui-5`" |
| Notifications | `fleet-ui-5` | "Notifications" |
| Live updates | `fleet-ui-5` | "Live updates" |
| `show fleet task`, `show fleet task id <id>`, `clear fleet task id <id>` | `fleet-ui-5` | "Commands this spec adds" |
| AC-1, AC-2, AC-3, AC-5, AC-6, AC-7 | `fleet-ui-5` | The same identifiers |
| AC-4 | `fleet-ui-5` AC-4 | Its page part; the command part stays as `fleet-ui-5` AC-4 |
| AC-8 | `fleet-ui-5` AC-8 | Its `show fleet task` part |
| AC-9 | `fleet-ui-5` AC-8 | Its "same data as the pages" clause, for the alarms |
| AC-10, AC-11, AC-12, AC-13 | `fleet-ui-8` AC-1, AC-2, AC-3, AC-4 | Their page parts; the command parts stay in `fleet-ui-8` under the same identifiers |
| AC-14 | `fleet-ui-8` AC-9 | Whole |
| AC-15 | `fleet-ui-8` AC-10 | Its page part |
| AC-16 | `fleet-ui-8` AC-12 | Whole |
| C-1 | `fleet-ui-8` O-4 | Whole |
| R-1 | `fleet-ui-8` R-2 | Whole |
| `test/web/fleet-alarm-cause.wb`, `TestAlarmCountsMatchTheirRows` | `fleet-ui-8` | "Wiring Test", "🧪 TDD Test Plan" |
| `test/web/fleet-task-progress.wb`, `test/web/fleet-live-device-state.wb` | `fleet-ui-0` | Phase 5's tests, now this phase's |

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

### Alarm pages

The alarms are `fleet-ui-5`'s (`plan/release/features/spec-fleet-ui-5-alarm-list.md`,
"Alarms"): derived on the hub, answered by `show fleet alarm` and
`show fleet alarm history device <name>`. The pages read them from the hub.

Where alarms appear:

| Place | What it shows |
|-------|---------------|
| Alarms tab of the estate, a group node and a device | The umbrella's table stack: severity word with icon, device, alarm, detail, since, duration. Default order: severity, then oldest first. Filter fields: `severity`, `kind`, `device`, and the device fields of `fleet-ui-2` |
| Alarms card on dashboards | The most severe, with a link to the tab (`fleet-ui-3`) |
| Tree | A badge on a device row with its open alarm count; a group row's count turns to the warning colour |
| Toasts | One when a critical alarm opens on a device in the operator's current view |

A row's side panel shows the alarm's history on that device over the last week
and links to the device tab where the condition can be examined.

### Alarm causes on the pages

Which alarm is hidden behind which cause is `fleet-ui-8`'s
(`plan/release/features/spec-fleet-ui-8-alarm-causes.md`, "Alarm causes"):
the hub answers it as `hidden-behind` in `show fleet alarm`, and the pages read
that fact.

Where a hidden alarm appears on the pages:

| Place | A hidden alarm |
|-------|----------------|
| Alarms tab | No row of its own. The row of the shown alarm at the top of its chain says how many alarms are hidden behind it, and that row's side panel lists them: kind, summary, since. The line above the table gives both figures, for example "9 open, 3 hidden behind their cause" |
| Alarms tab with the filter term `has:cause` | The hidden alarms as rows, with the cause each is hidden behind. This is the table every count of hidden alarms links to |
| Tree badge, the device's open alarm count, a group row's colour, the Alarms card | Not counted. The Alarms card adds one line with the count of hidden alarms, linked to `has:cause` |
| The device side panel's open alarms | Listed under its cause |
| Toasts | None while hidden |
| Alarm history in the side panel | Its row, marked as hidden for the time a cause was open, as `show fleet alarm history` answers it |
| Live events | An alarm that becomes hidden or shown makes the same fragments stale as one that opens or clears |

What the operator is told on the pages at each move of `fleet-ui-8`'s move
table:

| From | Event | To | What the operator is told |
|------|-------|----|---------------------------|
| Not open | The condition becomes true, no cause open on the device | Shown | The alarm-opened event; a toast when critical |
| Not open | The condition becomes true, a cause is open on the device | Hidden | The event that marks the lists stale; no toast |
| Shown | A cause opens on the device | Hidden | The event that marks the lists stale; no toast, and no cleared event, because nothing cleared |
| Hidden | The cause clears, the condition is still true, the kind does not settle | Shown | As an opening: the event, and a toast when critical (choice C-1). `Since` stays the time the condition became true |
| Hidden | The cause clears and the kind settles | Hidden until the settle rule of `fleet-ui-8` is met, then Shown if the condition is true | As above, when shown |
| Hidden | The condition stops being true | Not open | The history row only |
| Shown | The condition stops being true | Not open | The alarm-cleared event |

A count and the rows behind its link cannot differ
(`docs/architecture/fleet-interface.md`, "A source answers facts, the package
derives"): every count of shown alarms is taken on the default table, and every
count of hidden alarms on the `has:cause` table.

What this changes in the alarm pages above, each carried by its own AC:

| Statement | Change | AC |
|-----------|--------|----|
| AC-4, "The alarm opens, appears in every place listed, and clears from all of them" | Narrowed for a hidden alarm: it appears only where "Where a hidden alarm appears on the pages" says, because its cause is the row to act on | AC-15 |
| Alarms tab filter fields `severity`, `kind`, `device` | Gains the term `has:cause` | AC-16 |
| Alarms card on dashboards | Gains the line counting hidden alarms, linked to `has:cause` | AC-14 |

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

Every operator command must run on every surface, so the task list is a
command first and a page second.

| Command | Answers |
|---------|---------|
| `show fleet task` | The task list, filterable by state, kind, actor and device |
| `show fleet task id <id>` | One task with its per-device results |
| `clear fleet task id <id>` | Cancels a pending or running task |

### Dependencies and readiness

| Dependency | State |
|------------|-------|
| The alarm model, `show fleet alarm` and `show fleet alarm history` | Missing: `fleet-ui-5` |
| The hidden-behind fact and the `hidden` filter | Missing: `fleet-ui-8` |
| Device registry and its change notifications | Missing: `fleet-1` |
| The actions that create tasks | `fleet-5` rollouts, `fleet-7` resolution, `fleet-9` relayed runs; each registers its task kind when it lands |
| A state store key for finished tasks | To be registered here |
| The data parts the pages read (`fleet-ui-2`, "Moved from fleet-1 (owner ruling 2026-11-23)", "The fleet interface parts after the split") | `ActivityPart`: `Alarms` and `AlarmHistory` answered from `fleet-ui-5`'s commands, `Tasks` and `Task` from this spec's task model. `EventPart`: this spec. `SettingsPart`: the alarm settings (`OfflineAlarmDelay`, `ResourceAlarmPercent`, `ResourceAlarmDelay`) are `fleet-ui-5`'s configuration; the finished task count (`FinishedTasksKept`) and the open stream maximum (`EventStreamsMax`) are this spec's |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/api-surfaces.md`, `ai/doors/commands-cli.md`, `ai/doors/hub-engine.md`, `ai/doors/core.md` - the web surface, the command registry, the managed hub, the state store
  → Constraint: a command is declared once and reaches every interface; the task commands are registered with their owner, never added in an adapter
  → Constraint: runtime state (the finished tasks) goes through `statestore` under a registered key
  → Constraint: markup is `.templ`, with no inline script or style
- [ ] `plan/post-release/features/spec-fleet-ui-0-umbrella.md` - the shell, the table stack, the side panel, the states, the URL scheme
  → Constraint: every page reads through the command dispatcher; the browser holds no model of the estate
- [ ] `plan/release/features/spec-fleet-ui-5-alarm-list.md` - the alarm model and its commands
  → Decision: alarms are derived from what the hub holds, never stored events; no acknowledgement
- [ ] `plan/release/features/spec-fleet-ui-8-alarm-causes.md` - which alarm is hidden behind which cause
  → Constraint: the hiding is computed on the hub and answered as `hidden-behind`; the pages compute none
- [ ] `docs/architecture/fleet-interface.md` - "The data source seam", "Live events", "The live event stream", "Activity"
  → Constraint: "A source answers facts only"; every count is derived in the package from the rows its link opens
  → Constraint: the event stream carries no data of the estate; each event names the stale regions
- [ ] `plan/post-release/features/spec-fleet-ui-2-estate-tree-and-device-tables.md` - "Moved from fleet-1 (owner ruling 2026-11-23)", "The fleet interface parts after the split": `ActivityPart`, `EventPart`, `SettingsPart` (the section moved there from `fleet-1` by the owner ruling of 2026-11-23)
  → Constraint: the part's methods, arguments and bounds stay as `fleet-ui-2` fixes them
- [ ] `plan/post-release/features/spec-fleet-3-audit-trail.md` - the permanent record a task links to

**Key insights:**
- The prototype (`fleet-ui-1`) already draws every page, panel, toast and the event stream over the sample source; this spec answers them for real.
- The task kind words of the page are a literal table today (`taskKindWordTable`); the kinds are an open set registered by the actions that create tasks.
- `EventKind` has no kind for an alarm becoming hidden or shown; "Live events" of "Where a hidden alarm appears on the pages" needs one of the existing kinds or a new one, settled at design.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/web/fleetui/source_activity.go` - `ActivitySource` (`Alarms`, `AlarmHistory`, `Tasks`, `Task`), `Alarm`, `AlarmEvent`, `Task`, `TaskDetail`, `TaskResult`; bounds `AlarmsMax` (1024), `AlarmHistoryMax` (512), `TasksMax` (512), `TaskAnswerBytesMax` (16384). Each method's comment says "Answered by: fleet-ui-5"
- [ ] `internal/component/web/fleetui/source_events.go` - `EventSource` (its comment says "Answered by: fleet-ui-5"), `EventKind` (device changed, alarm opened, alarm cleared, task created, task progressed, task ended, registry changed), `Event`, `Feed` (`Subscribe`, `Publish`, `Close`), `Subscription` (`Events`, `Close`, `Lagged`); `EventBufferSize` and `EventSubscriptionsMax` are 64
- [ ] `internal/component/web/fleetui/handler_tasks.go` - `tasksContent`, `activityTaskRow`, `taskSidePanel`, `taskSideView` (Cancel while active, Retry over the failed devices), `taskActionAnswer`, whose toasts say the prototype changed nothing; `taskKindWordTable`, a literal map of six task kinds to their words, and `TaskKindWords`, which shows an unlisted kind by its own name
- [ ] `internal/component/web/fleetui/handler_alarms.go` - `mustRegisterAlarmTab`, `alarmTab`, `registerAlarmFields` (`severity`, `kind`, `device`, `state`), `scopedAlarms`, `alarmsContent`, `alarmSidePanel`, `alarmHistoryCard` (the week of history, `alarmHistorySpan`), `alarmExamineLink`
- [ ] `internal/component/web/fleetui/handler_events.go` - `eventStream.ServeHTTP` and `run`: per-page stream, `eventScope.stale` naming the regions `tree`, `content`, `tasks`, `header`, a write once per second, `scopeRefusalStatus`; the `Stepper` routes of the sample source
- [ ] `internal/component/web/fleetui/component_toast.templ` - `ToastLevel` with its word, key, tone and lifetime (`toastShortSeconds`, six seconds; warning and error stay), `ToastView`, `ToastArea` (at most four shown, the rest counted)
- [ ] `internal/component/web/fleetui/source_config.go` - `Settings`: `OfflineAlarmDelay`, `ResourceAlarmPercent`, `ResourceAlarmDelay`, `FinishedTasksKept`, `EventStreamsMax`; its `Settings` method's comment says "Answered by: fleet-ui-5 (hub configuration)"
- [ ] `internal/component/web/fleetui/sample/activity.go` - `alarmRows`, `alarmsAt`, `alarmHistoryAt`: twelve open alarms, the week's cleared ones, and the script's offline and resource alarms
- [ ] `internal/component/web/sse.go` - `EventBroker`: `Subscribe`, `Broadcast`, `SendTo`, `SendNotice`; reaches every client, or every client of one user

**Behavior to preserve:**
- Every page, panel, toast and stream rule `docs/architecture/fleet-interface.md` describes for the prototype
- `fleet-ui-2`'s `ActivityPart`, `EventPart` and `SettingsPart` methods, arguments and bounds
- The alarm tab registry of the page, keyed by kind name

**Behavior to change:**
- The pages, panels, toasts and stream answer from the hub's task model and from `fleet-ui-5`'s and `fleet-ui-8`'s alarm commands instead of the sample source
- A hidden alarm appears on the pages only where "Where a hidden alarm appears on the pages" says

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| `ActivitySource`, `Alarm`, `AlarmEvent` | `internal/component/web/fleetui/source_activity.go`; the same methods as `ActivityPart` in `plan/post-release/features/spec-fleet-ui-2-estate-tree-and-device-tables.md` | `Alarms` answers the open alarms, at most `AlarmsMax`. `Alarm` holds one device and no cause. Answered only by the sample source | Extends: answered for real, the alarms from `fleet-ui-5`'s commands and the tasks from this spec's model. `Alarm` gains the identifier of the alarm it is hidden behind, empty when shown, and `AlarmEvent` gains the hidden mark (moved from `fleet-ui-8`). The methods, their arguments and their bounds do not change, so `fleet-ui-2`'s part contract stands. The source answers the fact; the counts are derived in the package, as every other count is |
| `EventSource`, `Feed`, `Subscription` | `internal/component/web/fleetui/source_events.go`; `docs/architecture/fleet-interface.md`, "Live events" | Typed events into one feed per source, bounded buffers and subscriptions, no goroutine | Extends: the real source publishes into the same feed when a device, an alarm, a task or the registry changes |
| The fleet event stream | `internal/component/web/fleetui/handler_events.go`; `docs/architecture/fleet-interface.md`, "The live event stream" | Per-page stream, scoped to the view, coalesced once a second, bounded by `EventSubscriptionsMax` | Extends: unchanged, fed by the real source |
| Event broker | `internal/component/web/sse.go`, `EventBroker` | Broadcasts configuration and log events to every client | Does not apply to per-view data: a broadcast would send one view's devices to every client (umbrella, "Existing Mechanisms") |
| Per-connection live feed | `internal/component/web/handler_l2tp.go`, the samples stream | Streams one subject's data to one connection on a ticker | Extends, as the umbrella records: the fleet stream is per connection and scoped to the view |
| Toast component | `internal/component/web/fleetui/component_toast.templ`, `ToastLevel`, `ToastView`, `ToastOOB` | Four levels with word, tone and lifetime; the prototype's toasts | Extends: real toasts for task ends and critical alarm openings use it unchanged |
| The prototype task and alarm pages | `internal/component/web/fleetui/handler_tasks.go`, `handler_alarms.go` | Draw the task panel, the Tasks and Alarms tabs, the task list and the side panels over the sample source | Extends: the same handlers over the real source |
| The task kind words | `internal/component/web/fleetui/handler_tasks.go`, `taskKindWordTable`, `TaskKindWords` | A literal map of six kinds to their words; an unlisted kind is shown by its name | Replaces, at design: the kinds are registered by the actions that create tasks ("Dependencies and readiness"), so the words come from that registration and the literal list is deleted (`ai/rules/principles.md`, a central enumeration) |
| The alarm tab registry | `internal/component/web/fleetui/handler_alarms.go` `mustRegisterAlarmTab`, filled by `register_activity_pages.go` `registerAlarmTabs` | Maps a kind's name to the device tab where it is examined | Extends: unchanged; a kind with no entry links the device's dashboard |
| The sample alarms | `internal/component/web/fleetui/sample/activity.go` `alarmRows`, `alarmsAt` | Twelve open alarms and a script in which a device goes offline and comes back | Extends: the sample answers the hidden-behind fact for its rows, so the prototype shows the hidden form and a browser test can step through it (moved from `fleet-ui-8`) |
| Runtime state store | `internal/core/statestore`, keys in `pkg/zefs/keys.go` (`ai/doors/core.md`) | The daemon's one store for runtime state | Extends: the finished tasks survive a hub restart under a key registered here |
| The audit trail | `plan/post-release/features/spec-fleet-3-audit-trail.md`; `TaskDetail.AuditIDs`, `AuditEntry.TaskID` | Planned: the permanent record of who did what. The page types already link a task and its audit entries both ways | Does not apply: a task links to its audit entries and keeps no second record |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `docs/architecture/fleet-interface.md` | A source answers facts, the package derives | The `// Design:` page of `source_activity.go` | "A source answers facts only" | agrees | The hub answers alarms, tasks and `hidden-behind`; every count is derived in the package from the table its link opens (AC-14) |
| `docs/architecture/fleet-interface.md` | Live events | The `// Design:` page of `source_events.go` | "An event names what changed and carries no data, so a page fetches again what the event made stale." | agrees | The real source publishes the same events and the pages refetch (AC-5) |
| `ai/doors/system.md` | One schema, one command registry, every interface | System door | "A command is declared once and reaches every interface." | agrees | The task commands are registered once and reach every surface (AC-8) |
| `ai/doors/api-surfaces.md` | Boundaries | The door covering `internal/component/web/fleetui/` | "Never add a command in an adapter." | agrees | The task commands are registered with the hub's task model; the pages reach them and the alarm commands through the dispatcher |
| `ai/doors/le-tooling.md` | Where it sits | The door covering `test/web/` and `test/managed/` | "Functional `.ci` files live under `test/<suite>/` and run through" | agrees | `fleet-task-show` lives under `test/managed/` and the browser tests under `test/web/`, run by `./le test managed` and `./le test functional web` |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A fleet action started by an operator or by the hub, which creates a task
- `show fleet task`, `show fleet task id <id>` or `clear fleet task id <id>` on any command surface
- A browser request to a fleet page that shows tasks or alarms, and the page's `/fleet/events` stream
- A change on the hub: a device's state, an alarm opening, clearing, hiding or showing, a task's progress, the registry

### Transformation Path
1. Every fleet action creates its task through one entry point of the hub's task model, with its registered kind.
2. The task model records progress and per-device results, bounded and escaped, and keeps finished tasks in the state store to the configured count.
3. The fleet pages read tasks through `show fleet task` and alarms through `show fleet alarm` and `show fleet alarm history` on the hub, through the command dispatcher.
4. The fleet package derives every count, badge and card line from those answers, the hidden ones from `hidden-behind`.
5. A change publishes an event into the feed; each open stream in whose view it falls names the stale regions, and the page refetches them. A toast is raised for a task the operator started that ends, and for a critical alarm shown on a device in the operator's view.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Hub task model ↔ Command surfaces | `show fleet task`, `clear fleet task` through the command dispatcher | No |
| Fleet pages ↔ Hub | `ActivitySource` and `EventSource` answered through the dispatcher | No |
| Browser ↔ Hub web server | The per-page server-sent stream and htmx fragment requests | No |

### Changed-Wire Census

**Search evidence:** [pending research: the `show fleet task` answer shape, the `show fleet alarm` answer `fleet-ui-5` and `fleet-ui-8` fix, the `/fleet/events` frames of `handler_events.go`]

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| `show fleet task` answer | [pending research] | [pending research] | [pending research] | New |
| `/fleet/events` stream | Writer: `eventStream.run`; reader: `assets/fleet.js` | `internal/component/web/fleetui/handler_events.go` | [pending research] | Preserve: the prototype's `stale` frames and regions |

### Integration Points
- `ActivitySource` and `EventSource` - answered by the real source over the hub
- The alarm commands of `fleet-ui-5` and the `hidden-behind` fact of `fleet-ui-8` - read, never recomputed
- The actions of `fleet-5`, `fleet-7` and `fleet-9` - each registers its task kind
- `statestore` - the finished tasks

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `show fleet alarm` answers every field the alarm pages show: kind, severity, device, summary, detail, since, and `hidden-behind` | The prototype's `Alarm` (`source_activity.go`) and `fleet-ui-8` AC-11 | A page column has no source and the real source must ask a second command | `fleet-ui-5`'s and `fleet-ui-8`'s answer shapes, read before this spec's design | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A count and the rows behind its link differ | A card figure unlike its table | Counts derived from the tables their links open (AC-14) |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | [pending design] |
| How is it reverted? | [pending design] |
| Who else touches this path? | `fleet-ui-5` and `fleet-ui-8` (the alarms the pages read), `fleet-ui-3` (the Alarms card that carries the hidden line), `fleet-ui-2` and `fleet-ui-4` (the Tasks tab and the actions that create tasks; `fleet-ui-2` also owns the parts, moved there from `fleet-1`), `fleet-ui-6` (rollouts and resolution create tasks) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| An action over several devices started from a page | → | The hub's task model, the task panel and the live events | `test/web/fleet-task-progress.wb` |
| `show fleet task` on a hub running a task over a managed client | → | The hub's task model and its command | `test/managed/fleet-task-show.ci` |
| A device changes state while a page is open | → | The real source's feed and the page's stream | `test/web/fleet-live-device-state.wb` |
| A browser on the Alarms tab while a device's alarm opens and clears | → | The alarm pages over `show fleet alarm` | `test/web/fleet-alarm-pages.wb` |
| A browser on the Alarms tab while a sample device goes offline | → | The page lists the hidden alarm under its cause | `test/web/fleet-alarm-cause.wb` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Any fleet action on devices | A task exists for it from the moment it starts, visible in the panel, in the Tasks tab of each target and in `show fleet task` | Reads create no task, because they change nothing |
| AC-2 | A task over several devices where one fails | The task ends Partly failed, each device shows its own result, and Retry targets only the failed device | None |
| AC-3 | The hub restarts during a task | After restart the task shows Failed with the reason "hub restarted" | Devices that had finished keep their result |
| AC-4 | A condition in `fleet-ui-5`'s alarm table becomes true, then false | The alarm opens, appears in every place "Alarm pages" lists, and clears from all of them | Its history row remains, because the history is the record that it happened |
| AC-5 | A device changes state while a page is open | The tree row, the table row and the cards that count it change without a reload, within the bound fixed at design | Pages the operator is not viewing do not update, because they have no stream |
| AC-6 | An operator who may not see a device | No event about it reaches the operator's stream | None |
| AC-7 | The stream is lost | The stale banner shows with the time, the page reconnects, and the live fragments refresh once | None |
| AC-8 | `show fleet task` on any command surface: the CLI, MCP, REST, gRPC, SSH and the web command surface | The same data as the pages, as data answering every pipe | None |
| AC-9 | The alarm pages and `show fleet alarm` on the same hub | The pages show the same open alarms `show fleet alarm` answers on every command surface | None |
| AC-10 | A device with a component down goes offline | On the pages the device has one shown alarm, Device offline. Component down has no row of its own, is not in the tree badge or the Alarms card count, and is listed in the offline row's side panel | The `has:cause` table lists it, because that table is the hidden alarms |
| AC-11 | The device of AC-10 comes back and its report still names the component down | Component down has its row again, with the `Since` of the moment the component went down, and a toast is raised as for an opening (choice C-1) | None |
| AC-12 | A component alarm opens and its condition stops while the device is offline | The alarm never has a row of its own on the default Alarms table and raises no toast | Its history row shows in the side panel, marked hidden, because the history is the record that it happened |
| AC-13 | Resource high is shown, then its device goes offline | The row leaves the default table with no alarm-cleared event, and is listed under Device offline with its own `Since` | None |
| AC-14 | Any count of alarms on a page, shown or hidden | Following its link lists exactly that many rows | None |
| AC-15 | A condition in `fleet-ui-5`'s alarm table becomes true while a cause is open on its device, then false | The alarm appears on the pages only where "Where a hidden alarm appears on the pages" says, and clears from all of them. This narrows AC-4 for a hidden alarm | Its history row remains, marked hidden, because the history is the record that it happened |
| AC-16 | The Alarms tab with the filter term `has:cause` | The hidden alarms are listed as rows, each with the alarm it is hidden behind | None |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Watches an operation across many devices finish or fail | Action, task, task panel, live events | `test/web/fleet-task-progress.wb` |
| 2 | Lists what the hub is doing from any command surface | `show fleet task` through the dispatcher | `test/managed/fleet-task-show.ci` |
| 3 | Sees a device change state without a reload | Hub change, feed, the page's stream, fragment refetch | `test/web/fleet-live-device-state.wb` |
| 4 | Sees the estate's open alarms on the Alarms tab, and sees one clear | `show fleet alarm`, the Alarms tab, the stream | `test/web/fleet-alarm-pages.wb` |
| 5 | Sees one offline alarm instead of the stale alarms of the device behind it, and sees them return when the device does | `show fleet alarm` with `hidden-behind`, the Alarms tab, the side panel | `test/web/fleet-alarm-cause.wb` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestTaskCreatedForEveryAction` | [pending design: the hub task model package] | AC-1: every registered action creates a task through the one entry point | |
| `TestTaskPartlyFailedRetriesFailedOnly` | [pending design] | AC-2 | |
| `TestTaskRunningAtRestartRecordedFailed` | [pending design] | AC-3 | |
| `TestEventStreamCarriesOnlyVisibleDevices` | `internal/component/web/fleetui/`, beside the stream's tests | AC-6 | |
| `TestAlarmCountsMatchTheirRows` | `internal/component/web/fleetui/`, beside `TestDashboardAlarmLinksCountTheirRows` | AC-14, AC-15: the tree badge, the card count and the hidden count against the tables their links open | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Per-device result | 0 to `TaskAnswerBytesMax` (16384) bytes | 16384 bytes shown whole | N/A | 16385 bytes cut, with the note saying so |
| Toasts shown at once | 0 to 4 | Four shown | N/A | A fifth is counted, not shown |
| Open fleet streams | 0 to the configured maximum | The maximum | N/A | One more: the page works without live updates and says so |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `fleet-task-progress` | `test/web/fleet-task-progress.wb` | A task's progress and result are visible as they happen | |
| `fleet-task-show` | `test/managed/fleet-task-show.ci` | A real hub running a task over a real managed client answers it in `show fleet task` and `show fleet task id <id>` | |
| `fleet-live-device-state` | `test/web/fleet-live-device-state.wb` | A state change appears without a reload | |
| `fleet-alarm-pages` | `test/web/fleet-alarm-pages.wb` | AC-4, AC-9: an alarm opens on the Alarms tab, the tree badge and the card, and clears from them | |
| `fleet-alarm-cause` | `test/web/fleet-alarm-cause.wb` | Browser test over the sample source, stepped. AC-10, AC-11, AC-13, AC-16 as the operator sees them: the row leaves, the offline row carries the count, the side panel lists it, `has:cause` lists it, and it returns when the device does | |

## Files to Modify
- `internal/component/web/fleetui/source_activity.go` - `Alarm` gains the hidden-behind identifier, `AlarmEvent` the hidden mark (moved from `fleet-ui-8`); the "Answered by" comments name the spec that answers each method
- `internal/component/web/fleetui/source_events.go` - the "Answered by" comment of `Subscribe`
- `internal/component/web/fleetui/sample/activity.go` - the sample answers the hidden-behind fact for its rows (moved from `fleet-ui-8`)
- `internal/component/web/fleetui/handler_alarms.go` - the hidden form of the Alarms tab, the side panel and `has:cause`
- `internal/component/web/fleetui/handler_tasks.go` - the task kind words from the kinds' registration
- `docs/architecture/fleet-interface.md` - the real sources and the hidden form
- [pending design: the hub task model, its YANG and handlers, the real `ActivitySource` and `EventSource`]

## Files to Create
- `test/web/fleet-task-progress.wb` - browser test of a task's progress
- `test/web/fleet-live-device-state.wb` - browser test of a live change
- `test/web/fleet-alarm-pages.wb` - browser test of the alarm pages
- `test/web/fleet-alarm-cause.wb` - browser test of the hidden form
- `test/managed/fleet-task-show.ci` - functional test of the task commands through the hub
- [pending design]

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `show fleet task`, `clear fleet task`, the finished task count and the open stream maximum |
| YANG validation constraints | | |
| YANG custom validators | | |
| CLI commands/flags | Yes | `show fleet task`, `clear fleet task` |
| CLI grammar (keyword before value) | Yes | `id <id>` |
| Editor autocomplete | | |
| Functional test for new RPC/API | Yes | `test/managed/fleet-task-show.ci` |
| Pipe completeness | | |
| Env var registration | | |
| Doctor check for runtime dependencies | | |
| Doctor code per problem class | | |
| Prometheus counters/metrics | | |
| BGP family surface (new SAFI / capability / attribute) | N-A | No BGP wire surface |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | | |
| 2 | Config syntax changed? | | |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md`, `show fleet task`, `clear fleet task` |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | | |
| 6 | Has a user guide page? | | |
| 7 | Wire format changed? | | |
| 8 | Plugin SDK/protocol changed? | | |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | No RFC behavior |
| 10 | Test infrastructure changed? | | |
| 11 | Affects daemon comparison? | | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/fleet-interface.md` |
| 13 | Route metadata keys added/changed? | N-A | No route metadata keys |
| 14 | Prometheus counters added/changed? | | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | | |
| 16 | Any changed source file referenced by existing doc source anchors? | | |
| 17 | Existing docs show config/CLI/API examples for this area? | | |

## Design presentation

### What changes

[pending design]

### Before and after

[pending design]

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| The pages compute which alarm is hidden | The commands and the pages would disagree; `fleet-ui-8` computes it on the hub and the pages read `hidden-behind` |
| Fleet data over the broadcast broker | A broadcast would send one view's devices to every client (umbrella, "Existing Mechanisms") |

### Documents departed from

none

### Choices for Thomas

| # | Choice | Options | Decision |
|---|--------|---------|----------|
| C-1 | Does an alarm that becomes shown after its cause clears raise a toast when critical? (was `fleet-ui-8` O-4) | (a) recommended, Yes: an operator who saw only "device offline" is told what is still wrong when the device is back; (b) No: a recovery that leaves five alarms raises no toast, and the list is the only place that says so | |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- [pending design]
   - Tests: `test/managed/fleet-task-show.ci`, `test/web/fleet-task-progress.wb`, `test/web/fleet-live-device-state.wb`, `test/web/fleet-alarm-pages.wb`, `test/web/fleet-alarm-cause.wb`
   - Verify: `./le test managed` and `./le test functional web` run the wiring tests, red until the real sources answer
2. **Phase: Task model and commands** -- the task, its kinds, retention, restart, cancel and retry; `show fleet task`, `clear fleet task`
   - Tests: `TestTaskCreatedForEveryAction`, `TestTaskPartlyFailedRetriesFailedOnly`, `TestTaskRunningAtRestartRecordedFailed`, `fleet-task-show`
3. **Phase: Task pages** -- the header button, the task panel, the Tasks tabs, `/fleet/tasks/`, the spinners
   - Tests: `fleet-task-progress`
4. **Phase: Notifications and live updates** -- the toasts, the real feed and the streams
   - Tests: `TestEventStreamCarriesOnlyVisibleDevices`, `fleet-live-device-state`
5. **Phase: Alarm pages and the hidden form** -- the Alarms tabs, card, badges, side panel, `has:cause`, the hidden counts
   - Tests: `TestAlarmCountsMatchTheirRows`, `fleet-alarm-pages`, `fleet-alarm-cause`
   - Verify: nothing the hub does to a device is invisible, and a device that goes offline shows one alarm

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | The pages show what `show fleet alarm` and `show fleet task` answer, and compute no alarm and no hiding |
| Rule: registration | No list of task kinds or alarm kinds exists in the page package beyond what their registrations answer |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| [pending design] | [pending design] |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Each per-device result is bounded and escaped |
| Authorization | A stream carries only what the operator's profile may see (AC-6). A hidden alarm on a device the operator may not see reaches neither a count nor a side panel (moved from `fleet-ui-8`) |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Tasks are the hub's own model (from `fleet-ui-5`) | Each action reports its own progress | Every fleet action creates a task through one entry point, so a new action cannot be invisible |
| Events name fragments (from `fleet-ui-5`) | Events carry the estate's data | The server decides what is stale. The browser stays a renderer |
| One stream per page (from `fleet-ui-5`) | One stream per operator, or the broadcast broker | A page's stream is scoped to what the page shows, which is also what the operator may see |

## Known Limitations
- None recorded yet.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test (N-A when Scope is tooling or docs, which delete that section)
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches. No agent runs any `./le verify` command: GitHub CI verifies the pushed commit, and the full verification is Thomas's release check. A red CI reports gets a fix commit
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied: lint fixed rather than disabled, the focused check for the changed behavior run once with its OUTPUT PASTED, and any red named with the one-line reason it is scaffolding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script (commit A preserves the spec in history)
