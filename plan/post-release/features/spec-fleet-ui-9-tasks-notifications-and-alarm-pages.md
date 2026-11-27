# Spec: fleet-ui-9 -- Tasks, Notifications, Live Updates and the Alarm Pages

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | web, fleet, cli |
| Priority | P3 |
| Release reason | Thomas ruled on 2026-11-23 that the fleet features of the first release are reached through commands on every command surface, and that their web pages, the hub's task model and panel, notifications and live page updates come after the first release (`ai/META-DECISIONS.md`, "Goal and scope"). Split out of `plan/release/features/spec-fleet-ui-5-alarm-list.md` and `plan/release/features/spec-fleet-ui-8-alarm-causes.md`, which keep the alarm model and its commands; existing supported behavior and its obligations remain required. |
| RFC gaps | - |
| Depends | plan/release/features/spec-fleet-ui-5-alarm-list.md, plan/release/features/spec-fleet-ui-8-alarm-causes.md, plan/post-release/features/spec-fleet-ui-2-estate-tree-and-device-tables.md, spec-fleet-1-device-registry (closed 2026-11-25), spec-fleet-4-inventory-health (closed 2026-11-26), spec-fleet-8-metric-history (closed 2026-11-27) |
| Phase | 9/9 of plan/post-release/features/spec-fleet-ui-0-umbrella.md |
| Handoff | - |
| Updated | 2026-11-25 |

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
(`ai/META-DECISIONS.md`, "Goal and scope"). The split preserved the original acceptance criteria; the table "Where each part came from" names its source. The alarm
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
| Hidden | The cause clears and the kind settles | Not open | Close the old occurrence and refresh history and counts. If fresh samples later qualify, display a new occurrence with its own identity |
| Hidden | The condition stops being true | Not open | The history row only |
| Shown | The condition stops being true | Not open | The alarm-cleared event |

A count and the rows behind its link cannot differ
(`docs/architecture/fleet-interface.md`, "A source answers facts, the package
derives"): every count of shown alarms is taken on the default table, and every
count of hidden alarms on the `has:cause` table. Count across all authorized pages of the same filtered snapshot. A bounded page displays its own rows and a continuation, and its header shows the matching total. The default and hidden totals cover disjoint rows.

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
| `clear fleet task id <id>` | Stops outstanding work for a cancellable task and keeps completed targets' results |
| `request fleet task retry id <id>` | Starts a linked task over the original failed targets after checking current permissions and the kind's retry preconditions |

### Dependencies and readiness

| Dependency | State |
|------------|-------|
| The alarm model, `show fleet alarm` and `show fleet alarm history` | Missing: `fleet-ui-5` |
| The hidden-behind fact and the `hidden` filter | Missing: `fleet-ui-8` |
| Device registry and its change notifications | The managed hub has device queries. The operational event path must be exercised through the real source. |
| The actions that create tasks | `fleet-5` rollouts, `fleet-7` resolution, `fleet-9` relayed runs; each registers its task kind when it lands |
| A state store key family for task snapshots | To be registered here |
| The data parts the pages read (`fleet-ui-2`, "Moved from fleet-1 (owner ruling 2026-11-23)", "The fleet interface parts after the split") | `ActivityPart`: `Alarms` and `AlarmHistory` answered from `fleet-ui-5`'s commands, `Tasks` and `Task` from this spec's task model. `EventPart`: this spec. `SettingsPart`: the alarm settings (`OfflineAlarmDelay`, `ResourceAlarmPercent`, `ResourceAlarmDelay`) are `fleet-ui-5`'s configuration; the finished task count (`FinishedTasksKept`) and the open stream maximum (`EventStreamsMax`) are this spec's |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/api-surfaces.md`, `ai/doors/commands-cli.md`, `ai/doors/hub-engine.md`, `ai/doors/core.md` - the web surface, the command registry, the managed hub, the state store
  → Constraint: a command is declared once and reaches every interface; the task commands are registered with their owner, never added in an adapter
  → Constraint: runtime state (task snapshots including unfinished work) goes through `statestore` under a registered key
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
- `EventKind` has no kind for an alarm becoming hidden or shown; "Live events" of "Where a hidden alarm appears on the pages" uses a new internal AlarmVisibilityChanged event carrying the occurrence and the visibility transition. The on-wire SSE frame still names stale fragments.

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
| `ActivitySource`, `Alarm`, `AlarmEvent` | `internal/component/web/fleetui/source_activity.go`; the same methods as `ActivityPart` in `plan/post-release/features/spec-fleet-ui-2-estate-tree-and-device-tables.md` | `Alarms` answers the open alarms, at most `AlarmsMax`. `Alarm` holds one device and no cause. Answered only by the sample source | Extends: answered for real, the alarms from `fleet-ui-5`'s commands and the tasks from this spec's model. `Alarm` gains the identifier of the alarm it is hidden behind, empty when shown, and `AlarmEvent` gains the hidden mark (moved from `fleet-ui-8`). Preserve the bounded part methods. Register an additional paged alarm query capability on the real and sample adapters using the release commands’ continuation, authorized total and counts for the same filter. Pages, badges and cards use the same complete result when its rows span multiple pages |
| `EventSource`, `Feed`, `Subscription` | `internal/component/web/fleetui/source_events.go`; `docs/architecture/fleet-interface.md`, "Live events" | Typed events into one feed per source, bounded buffers and subscriptions, no goroutine | Extends: the real source publishes changes into the feed. Subscription admission reads the configured cap under the same lock that inserts the subscriber. Alarm visibility changes invalidate the relevant authorized view. |
| The fleet event stream | `internal/component/web/fleetui/handler_events.go`; `docs/architecture/fleet-interface.md`, "The live event stream" | Per-page stream, scoped to the view, coalesced once a second, bounded by `EventSubscriptionsMax` | Extends: check the actor and visible targets before computing stale regions. A hidden target must produce no frame, refresh or toast for that viewer. Recheck membership and permissions when they change. |
| Event broker | `internal/component/web/sse.go`, `EventBroker` | Broadcasts configuration and log events to every client | Does not apply to per-view data: a broadcast would send one view's devices to every client (umbrella, "Existing Mechanisms") |
| Per-connection live feed | `internal/component/web/handler_l2tp.go`, the samples stream | Streams one subject's data to one connection on a ticker | Extends, as the umbrella records: the fleet stream is per connection and scoped to the view |
| Toast component | `internal/component/web/fleetui/component_toast.templ`, `ToastLevel`, `ToastView`, `ToastOOB` | Four levels with word, tone and lifetime; the prototype's toasts | Extends: real toasts for task ends and critical alarm openings use it unchanged |
| The prototype task and alarm pages | `internal/component/web/fleetui/handler_tasks.go`, `handler_alarms.go` | Draw the task panel, the Tasks and Alarms tabs, the task list and the side panels over the sample source | Extends: the same handlers over the real source |
| The task kind words | `internal/component/web/fleetui/handler_tasks.go`, `taskKindWordTable`, `TaskKindWords` | A literal map of six kinds to their words; an unlisted kind is shown by its name | Replaces, at design: the kinds are registered by the actions that create tasks ("Dependencies and readiness"), so the words come from that registration and the literal list is deleted (`ai/rules/principles.md`, a central enumeration) |
| The alarm tab registry | `internal/component/web/fleetui/handler_alarms.go` `mustRegisterAlarmTab`, filled by `register_activity_pages.go` `registerAlarmTabs` | Maps a kind's name to the device tab where it is examined | Extends: unchanged; a kind with no entry links the device's dashboard |
| The sample alarms | `internal/component/web/fleetui/sample/activity.go` `alarmRows`, `alarmsAt` | Twelve open alarms and a script in which a device goes offline and comes back | Extends: the sample answers the hidden-behind fact for its rows, so the prototype shows the hidden form and a browser test can step through it (moved from `fleet-ui-8`) |
| Runtime state store | `internal/core/statestore`, keys in `pkg/zefs/keys.go` (`ai/doors/core.md`) | The daemon's one store for runtime state | Extends: persist each task before dispatch and persist per-device completion before publishing progress |
| The audit trail | `plan/post-release/features/spec-fleet-3-audit-trail.md`; `TaskDetail.AuditIDs`, `AuditEntry.TaskID` | Planned: the permanent record of who did what. The page types already link a task and its audit entries both ways | Extends: keep task progress separately from the audit, link each authorized entry by its stable ID, and expose an unavailable link when the optional audit owner is absent |
| Dashboard counts and device rows | `DeriveDevices`, `Figures`, `TopAlarms`, `alarmsCardView` and `devicesAlarmRows` in the fleet UI | Derive device and estate displays from source facts | Feed the same authorized, visibility-filtered rows into the card, the badge and its link. |
| Task panel and stream settings | `taskPanelTab`, `Settings` and `Feed.Subscribe` in the fleet UI | The panel selects active and completed tasks. The feed currently checks a fixed subscription limit. | Use the same durable task results as the task page, and honor the configured stream limit at subscription. A full stream pool still permits ordinary page reads. |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `docs/architecture/fleet-interface.md` | A source answers facts, the package derives | The `// Design:` page of `source_activity.go` | "A source answers facts only" | agrees | The hub answers alarms, tasks and `hidden-behind`; every count is derived in the package from the table its link opens (AC-14) |
| `docs/architecture/fleet-interface.md` | Live events | The `// Design:` page of `source_events.go` | "An event names what changed and carries no data, so a page fetches again what the event made stale." | agrees | The stream authorizes each event before choosing regions to refresh. The page refetches authorized fragments for task results and notifications (AC-5 and AC-6). |
| `ai/doors/system.md` | One schema, one command registry, every interface | System door | "A command is declared once and reaches every interface." | agrees | The task commands are registered once and reach every surface (AC-8) |
| `ai/doors/api-surfaces.md` | Boundaries | The door covering `internal/component/web/fleetui/` | "Never add a command in an adapter." | agrees | The task commands are registered with the hub's task model; the pages reach them and the alarm commands through the dispatcher |
| `ai/doors/le-tooling.md` | Where it sits | The door covering `test/web/` and `test/managed/` | "Functional `.ci` files live under `test/<suite>/` and run through" | agrees | `fleet-task-show` lives under `test/managed/` and the browser tests under `test/web/`, run by `./le test managed` and `./le test functional web` |
| `docs/architecture/fleet-interface.md` | Device tabs | Device navigation | "A device has the tabs Dashboard, Console, System, Interfaces, Routing, Config, Logs and Tasks." | departs | The device Alarms tab Thomas chose lists that device’s alarms directly. Update the documented tab set when the device page is wired. |
| `docs/architecture/fleet-interface.md` | Activity | Device alarm page | "The Alarms tab of the estate and of a group node lists the open alarms," | departs | Thomas selected the single-device view with the same ordering, visibility checks and row details. |
| `docs/architecture/fleet-interface.md` | Design decisions | Fleet alarm visibility | "The proposed alarm defaults apply together. A missing relay grant stays visible while the device is offline. A measured alarm uses its existing threshold duration after reconnect. Causes match within one device. Thomas selected “Use all three defaults”." | agrees | The hub derives visibility from same-device causes. The resource condition uses the kind’s own duration. |
| `docs/architecture/fleet-interface.md` | Design decisions | Fleet alarm visibility | "Thomas chose the following rule for Resource high at reconnect. “End the old occurrence. Show a new alarm only if fresh samples qualify.”" | agrees | Both the command and page histories close the pre-disconnect occurrence at reconnect. Fresh samples may open another. |
| `docs/architecture/fleet-interface.md` | Design decisions | Fleet alarm visibility | "When a critical alarm becomes visible again after its cause clears, show its toast to an authorized viewer of the affected device. Thomas selected “Show the critical toast”." | agrees | The page displays a critical recovery toast only for an authorized viewer in scope. |
| `docs/architecture/fleet-interface.md` | Design decisions | Fleet interface contract | "For Force complete on a staged rollout, Thomas chose “Leave unstarted devices on their prior version”." | agrees | A rollout task reports skipped targets without treating them as successful deliveries. |
| `docs/architecture/fleet-interface.md` | Design decisions | Fleet interface contract | "Thomas chose “Add the device Alarms tab”. The post-release device page offers the same authorized alarm rows as the device filter on the estate or group Alarms tab." | agrees | Register Alarms for a device. Its filtered rows, order and permissions use the same source as the estate table. |

| `docs/architecture/api/commands.md` | Transport Request Metadata | anchored to internal/component/managed/yang/ze-managed-cmd.yang | "Caller identity is injected only by trusted transport wiring." | agrees | Authenticate the trusted caller before reading targets, counting them or acting. Preserve the registered command’s typed output on each surface. |
| `docs/architecture/fleet-config.md` | RPCs | anchored to internal/component/managed/yang/ze-managed-cmd.yang | "All RPCs are scoped to that name implicitly." | agrees | A task targets only declared, authorized clients. Delivery requiring a connected device uses its authenticated session and the action owner’s validation. |
| `docs/architecture/fleet-config.md` | Design decisions | Owner decision of 2026-11-25 | "Thomas chose “Allow first provisioning and later edits from the device’s web Config tab”. An authorized operator can commit the first or a later version of a declared device’s configuration while it is offline, through the post-release fleet web surface." | agrees | The task and alarm views respect the target of an authorized configuration edit; a saved candidate is distinct from a device acknowledgement. |
| `ai/doors/hub-engine.md` | Boundaries | the door covering internal/component/managed/yang/ze-managed-cmd.yang | "No always-on file imports a gated package." | agrees | The build-tagged registration links the operational adapter with the hub. The managed task owner imports no web or optional action package. |
| `docs/architecture/config/syntax.md` | Overview | anchored to internal/component/plugin/yang/ze-plugin-conf.yang | "The parser is YANG-driven: each config node's type (leaf, leaf-list, container, list) determines how it is parsed." | agrees | Declare and validate the two numeric configuration leaves in YANG. |
| `ai/doors/plugin-infra.md` | Boundaries | the door covering internal/component/plugin/yang/ze-plugin-conf.yang | "The SDK stays generic." | agrees | Keep the operation service in the managed hub. The generic SDK gains no fleet task command or web types. |
| `docs/architecture/zefs-format.md` | Key Registry | the // Design: page of pkg/zefs/keys.go | "All known key patterns are registered in `pkg/zefs/keys.go` via `MustRegister()`" | agrees | Register raw key families for bounded headers and per-target results. Retention removes each task’s subordinate keys. |
| `docs/architecture/container/image-store.md` | State keys | anchored to pkg/zefs/keys.go | — | silent | Image content and container-project keys remain outside the task store. |
| `docs/architecture/diagnostics/debug-filtering.md` | One ZeFS trap | anchored to pkg/zefs/keys.go | "`List(prefix)` fails when the prefix ends in a slash." | agrees | Enumerate registered task key prefixes without a trailing slash at restart and eviction. |
| `docs/architecture/irr-owner.md` | Who may touch which key | anchored to pkg/zefs/keys.go | — | silent | IRR key grants do not cover the task namespace. Only the managed hub writes task results. |
| `docs/architecture/storage-backends.md` | Durable publication | anchored to pkg/zefs/keys.go | "Sync failures are returned to the caller." | agrees | A store write failure prevents dispatch of an unrecorded new task and withholds uncommitted progress. |
| `ai/doors/core.md` | Boundaries | the door covering pkg/zefs/keys.go | "Runtime state goes through the daemon's one store." | agrees | The optional task owner borrows the daemon store. Core does not import the managed hub. |
| `docs/architecture/api/commands.md` | Design decisions | Decision of 2026-08-21 | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | The read command returns structured data and inherits generic presentation, paging and filters. A pipe cannot start, cancel or retry a task. |
| `docs/architecture/api/commands.md` | Design decisions | Decision of 2026-11-20 | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | The read command returns structured data and inherits generic presentation, paging and filters. A pipe cannot start, cancel or retry a task. |
| `docs/architecture/api/commands.md` | Design decisions | Decision of 2026-11-23 | "A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does" | agrees | The read command returns structured data and inherits generic presentation, paging and filters. A pipe cannot start, cancel or retry a task. |
| `docs/architecture/fleet-config.md` | Design decisions | Decision of 2026-11-23 | "A reload that changes the certificate a serving hub's managed listener presents installs it on the running listener at that reload: the next handshake presents the certificate the configuration names, the listener keeps its socket and no session is closed, as the web and looking glass listeners do. The certificate stays in the configuration's `pki` block. In Thomas's words (voice-transcribed): "Keep the certificate in the configuration." and "In that case on commit we can perform the certificate change on the listener."" | agrees | Operational actions use already-authenticated sessions. The managed listener follows its existing reload and session rules. |
| `ai/doors/commands-cli.md` | Boundaries | The four task commands | "A handler answers data, never text." | agrees | Declare each action and its typed parameters in the managed owner’s YANG and register a typed answer common to all surfaces. |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A fleet action started by an operator or by the hub, which creates a task
- `show fleet task`, `show fleet task id <id>` or `clear fleet task id <id>` or `request fleet task retry id <id>` on any command surface
- A browser request to a fleet page that shows tasks or alarms, and the page's `/fleet/events` stream
- A change on the hub: a device's state, an alarm opening, clearing, hiding or showing, a task's progress, the registry

### Transformation Path
1. Every fleet action creates its task through one entry point of the hub's task model, with its registered kind.
2. Persist a task snapshot with its identifier, actor and targets before dispatch. Commit each per-device result before reporting progress. If the store refuses a write, report the failure and do not advertise that transition. At startup mark unfinished tasks Failed with reason "hub restarted", retain completed targets and identify interrupted targets. Persist these changes before accepting retries. Running tasks are exempt from finished-task eviction. A Retry creates a separate task linked to the original, covering only failed targets and subject to the current actor's permissions.
3. The fleet pages read tasks through `show fleet task` and alarms through `show fleet alarm` and `show fleet alarm history` on the hub, through the command dispatcher.
4. The fleet package derives every count, badge and card line from those answers, the hidden ones from `hidden-behind`.
5. Accept the authenticated principal and route scope at subscription, and filter before enqueueing in that subscriber’s bounded buffer. Revalidate membership and rights on a scope change, before emitting a frame and when serving each fragment. Identify both old and new membership on relabel/removal so a previously visible row disappears. Task events scope their target changes to the viewer. A target the viewer cannot see cannot wake the stream or appear in progress totals, results or audit links.
6. Publish internal AlarmVisibilityChanged for hidden/shown transitions, without fabricating an opening or clearing in alarm history. A shown critical alarm and the completion of a task started by the viewer queue an authorized notification for that page. Assign a transition ID so repeated invalidations do not duplicate a toast and repeated hide/show episodes can be distinguished. The stream emits only the new fragment name `notifications` alongside the existing stale regions. Issue an opaque page token in the rendered page, tied to its authenticated web session and scope. The stream and fragment present that token. Its fragment uses the page’s subscription token, rechecks identity and current scope, and returns escaped ToastOOB markup. Bound pending notifications per subscription to EventBufferSize. A lagged or expired page refetches current state. Durable task or alarm history covers the missed interval. Initial page load does not replay historical critical toasts.
7. Validate the configured stream limit against the existing ceiling of 64, allowing zero to disable subscriptions. Admit under the feed lock and release the slot on cancellation, lag or failed writes. Lowering the cap blocks new admissions until the pool falls below the limit. The sample uses the present default of 64. Ordinary authenticated page reads work with the pool full.
8. A task-kind registration supplies its name, display words, start and optional cancel/retry capabilities. The action owner supplies its effect and tests. Retrying excludes succeeded and skipped targets. A concurrent retry with the same submitted action token returns the existing child ID. A rollout target skipped by Force complete stays on its prior version. Results and summary distinguish skipped and failed. A task action fails closed when its owner or durable store is unavailable.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Hub task model ↔ Command surfaces | `show fleet task`, `clear fleet task` through the command dispatcher | No |
| Fleet pages ↔ Hub | `ActivitySource` and `EventSource` answered through the dispatcher | No |
| Browser ↔ Hub web server | The per-page server-sent stream and htmx fragment requests | No |

### Changed-Wire Census

**Search evidence:** Searched the fleetui handlers, sources, sample and browser assets for TaskDetail, AlarmEvent, Subscribe, regionSet.frame and openStream, and the managed command registration for existing fleet RPCs. The eventStream.run writer and the stale listener in assets/fleet.js are the working producer and reader.

| Wire | Role (writer / reader) | Carrier | Source evidence | Migration or preservation disposition |
|---|---|---|---|---|
| Task list, detail, cancellation and retry | Writer | The managed task owner registers YANG and handlers once. List answers contain ID, registered kind and words, actor, permitted targets, counts, state and times. Detail adds bounded outcomes, truncation and visible audit references. Retry returns child and original IDs. | Managed task and alarm contracts; existing web event writer | Publish only the authorized result for the current caller. |
| Task list, detail, cancellation and retry | Reader | CLI, SSH, REST, MCP, gRPC and the web command line use the same typed result. The activity adapter maps to `Task`, `TaskDetail` and `TaskResult` in `source_activity.go`. Sample callers keep their bounded method signatures. | Fleet activity source and existing fleet browser client | Preserve bounded typed answers and the existing event regions. |
| Open and historical alarms | Writer | Fleet-ui-5 and fleet-ui-8 publish occurrences, history, hidden-behind and authorized counts and cursor for a snapshot. | Managed task and alarm contracts; existing web event writer | Publish only the authorized result for the current caller. |
| Open and historical alarms | Reader | The paged adapter and bounded ActivityPart methods share these commands. Table, card, badges and side panel read the cause. Continue through the same snapshot to reach all matching rows. | Fleet activity source and existing fleet browser client | Preserve bounded typed answers and the existing event regions. |
| `/fleet/events` | Writer | `handler_events.go`’s `eventStream.run` writes `regionSet.frame` as a stale SSE event with comma-separated region names. | Managed task and alarm contracts; existing web event writer | Publish only the authorized result for the current caller. |
| `/fleet/events` | Reader | `assets/fleet.js`’s `openStream` listens for stale, filters against its region list and refetches. Preserve tree, content, tasks and header. Add notifications in both producer and reader. Older clients ignore the extra region. The SSE carries no device, task or notification content. | Fleet activity source and existing fleet browser client | Preserve bounded typed answers and the existing event regions. |

### Integration Points
- `ActivitySource` and `EventSource` - answered by the real source over the hub
- The alarm commands of `fleet-ui-5` and the `hidden-behind` fact of `fleet-ui-8` - read, never recomputed
- The actions of `fleet-5`, `fleet-7` and `fleet-9` - each registers its task kind
- `statestore` - task snapshots and bounded per-target progress

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| Dispatcher and web boundary | Designed | The operational source goes through the shared commands. The sample retains the same rendering and test seam. |
| Component imports and state | Designed | The gated hub registers the adapter. The operation owner writes through statestore and never imports the UI. |
| Duplicate state and payloads | Designed | Visibility and history remain owned by the release commands. Per-page streams carry fragment names. Durable per-device text has the 16 KiB bound. |
| Outbound registration | Designed | Register the four daemon commands, three source parts, paged alarm capability, notification fragment, and device alarm page. Each action registers its own kind and capabilities. |
| Inbound registration | Designed | Replace taskKindWordTable with registered labels. Populate filters from the same declarations. Existing page, tab and fragment registries discover new members. Update the fixed region reader and writer together. |

Store a versioned task header and individually bounded target records. Reserve the ID and actor-scoped request token before any external call. Commit the pending target list and task header before dispatch. Publish completion for a target only after its result write succeeds; reconstruct aggregate progress from those committed results if restart finds a stale summary. Fence start, cancel, retry and completion against the serialized task state. A crash after dispatch and before recording its outcome leaves that target interrupted with outcome unknown. Automatic replay is unsafe. The registered kind must establish that retry is permitted before external work can run again. Evicting a finished task removes its bounded children, preserving records needed by active tasks. Recovery completes interrupted eviction and discards uncommitted staging.

Keep active tasks independent of the finished-task cap. Ask the operational task command for a bounded page and the full authorized running count wherever active plus retained finished tasks exceed TasksMax. Include paging for remaining task results when response byte limits would otherwise truncate the target list. The legacy Tasks method continues to return at most TasksMax rows. Map the operational page into the same visible table and panel shapes, keeping an explicit continuation to all rows. Store the panel's last opened transition cursor for the authenticated viewer; another viewer opening a panel never clears that viewer’s failure indicator.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `show fleet alarm` answers every field the alarm pages show: kind, severity, device, summary, detail, since, and `hidden-behind` | The prototype's `Alarm` (`source_activity.go`) and `fleet-ui-8` AC-11 | A page column has no source and the real source must ask a second command | `fleet-ui-5`'s and `fleet-ui-8`'s answer shapes, read before this spec's design | confirmed by the release command designs; verify fields through the real dispatcher wiring test |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A count and the rows behind its link differ | A card figure unlike its table | Counts derived from the tables their links open (AC-14) |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Actions could lose progress or dispatch twice, counts could omit off-page alarms, and a stream could reveal a target outside its viewer’s scope. Test at the owner, dispatcher and two-principal browser boundaries. |
| How is it reverted? | Removing the operational adapter returns the affected parts to the existing unavailable presentation. Preserve versioned records for restart recovery and keep the release alarm commands available. |
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
| AC-5 | A device changes state while a page is open | The tree row, the table row and the cards that count it change without a reload, at the next one-second coalescing tick after the authorized event, plus the time to fetch the fragment | Pages the operator is not viewing do not update, because they have no stream |
| AC-6 | An operator who may not see a device | With an allowed and a forbidden device changed in the same fixture, the operator receives the allowed device’s invalidation. The event for the forbidden device is excluded. | None |
| AC-7 | The stream is lost | The stale banner shows with the time, the page reconnects, and the live fragments refresh once | None |
| AC-8 | `show fleet task` on any command surface: the CLI, MCP, REST, gRPC, SSH and the web command surface | The same data as the pages, as data answering every pipe | None |
| AC-9 | The alarm pages and `show fleet alarm` on the same hub | The pages show the same open alarms `show fleet alarm` answers on every command surface | None |
| AC-10 | A device with a component down goes offline | On the pages the device has one shown alarm, Device offline. Component down has no row of its own, is not in the tree badge or the Alarms card count, and is listed in the offline row's side panel | The `has:cause` table lists it, because that table is the hidden alarms |
| AC-11 | The device of AC-10 comes back and its report still names the component down | Component down has its row again, with the `Since` of the moment the component went down, and a toast is raised as for an opening (choice C-1) | None |
| AC-12 | A component alarm opens and its condition stops while the device is offline | The default table continues to show its offline cause and its side panel records the hidden component occurrence. Its opening and clearing generate neither a top-level row nor a toast. | Its history row shows in the side panel, marked hidden, because the history is the record that it happened |
| AC-13 | Resource high is shown, then its device goes offline | The row leaves the default table with no alarm-cleared event, and is listed under Device offline with its own `Since` | None |
| AC-14 | Any count of alarms on a page, shown or hidden | Following its link lists exactly that many authorized rows across all pages of the same snapshot | None |
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
| `TestTaskCreatedForEveryAction` | `internal/component/plugin/server/managed_tasks_test.go` | AC-1: every registered action creates a task through the one entry point | |
| `TestTaskPartlyFailedRetriesFailedOnly` | `internal/component/plugin/server/managed_tasks_test.go` | AC-2 | |
| `TestTaskRunningAtRestartRecordedFailed` | `internal/component/plugin/server/managed_tasks_test.go` | AC-3 | |
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

- `internal/component/managed/yang/ze-managed-cmd.yang` — declare both task reads and two authenticated actions with typed answers.
- `internal/component/plugin/yang/ze-plugin-conf.yang` — constrain finished-task retention and event-stream admission.
- `pkg/zefs/keys.go` — register versioned and bounded task state keys.
- `internal/component/web/fleetui/source_activity.go` — extend alarm and history facts, registered task labels and the separate paged alarm capability. Preserve existing bounded methods.
- `internal/component/web/fleetui/source_events.go` — internal visibility transitions, scoped admission and configured pool limit.
- `internal/component/web/fleetui/source_config.go` — name each settings owner and expose its live values.
- `internal/component/web/fleetui/register_activity_pages.go` and `internal/component/web/fleetui/register_tabs.go` — register the device’s Alarms page, tab and notification fragment.
- `internal/component/web/fleetui/handler_events.go` and `internal/component/web/fleetui/assets/fleet.js` — authorize, coalesce and consume fragment invalidations, including notifications.
- `internal/component/web/fleetui/handler_alarms.go` and `internal/component/web/fleetui/page_activity.templ` — page links, hidden-row filter, history, authorized totals and truncation.
- `internal/component/web/fleetui/handler_tasks.go` and `internal/component/web/fleetui/shell_states.go` — dispatch real task actions, consume registered kind words, render bounded results and progress, and update the panel and header.
- `internal/component/web/fleetui/derive.go`, `internal/component/web/fleetui/summary.go`, `internal/component/web/fleetui/handler_devices.go` and `internal/component/web/fleetui/handler_dashboard_cards.go` — shown and hidden badge/card figures and links for the entire authorized result.
- `internal/component/web/fleetui/sample/activity.go` and `internal/component/web/fleetui/sample/register/register.go` — sample facts, paging and registrations. Sample actions remain demonstrative.
- `docs/architecture/fleet-interface.md` — task state, stream admission, paged results and the device tab.

## Files to Create

- `internal/component/plugin/server/managed_tasks.go` and `internal/component/plugin/server/managed_tasks_test.go` — operation registry, durable task state and effects, restart, retention, cancel and retry.
- `internal/component/managed/cmd_tasks.go` — typed command registrations and dispatch to the serving hub’s task service.
- `internal/component/web/fleetui/activity/register.go` and `internal/component/web/fleetui/activity/source.go` — register the activity, event and settings parts and paged alarms using the real dispatcher and authenticated event source.
- `cmd/ze/hub/register_fleet_ui_activity.go` — link the operational adapter under fleet web tags on non-sample builds.
- `internal/component/web/fleetui/handler_live_notifications.go` — bounded per-subscription notification fragment and scoped lookup.
- `internal/component/web/fleetui/activity_live_test.go` — page totals and ordering beyond 1,024 authorized alarms, mixed permissions, real and sample adapters, and task and toast scope.
- `test/web/fleet-task-progress.wb`, `test/web/fleet-live-device-state.wb`, `test/web/fleet-alarm-pages.wb`, `test/web/fleet-alarm-cause.wb` — exercise real reads and mutations and the stepped cause view.
- `test/managed/fleet-task-show.ci` — drive the operations through the real hub and compare progress and restart outcomes.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `show fleet task`, `clear fleet task`, the retry action, the finished task count and the open stream maximum |
| YANG validation constraints | Yes | Use model ranges for stream and retention bounds; validate an ID and request token before state lookup. |
| YANG custom validators | No | The settings are scalar ranges. State, permission and retry eligibility are checked by the serving command. |
| CLI commands/flags | Yes | `show fleet task`, `clear fleet task`, `request fleet task retry` |
| CLI grammar (keyword before value) | Yes | `id <id>` |
| Editor autocomplete | Yes | Generate schema-driven help and completion for the task actions and typed ID. |
| Functional test for new RPC/API | Yes | `test/managed/fleet-task-show.ci` |
| Pipe completeness | Yes | Exercise registered typed answers through the generic JSON and table pipes. |
| Env var registration | No | Both settings are modeled configuration. |
| Doctor check for runtime dependencies | Yes | Expose persistence failure through the existing managed store integrity diagnosis including the failed operation and operator remedy. |
| Doctor code per problem class | Yes | Reuse the managed store integrity class for persistence faults. Unavailable optional web adapters return their existing typed refusal. |
| Prometheus counters/metrics | No | Progress, refusals and availability are returned by the task commands and stream admission response. |
| BGP family surface (new SAFI / capability / attribute) | N-A | No BGP wire surface |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | docs/features/fleet-management.md |
| 2 | Config syntax changed? | Yes | docs/guide/fleet-config.md |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md`, `show fleet task`, `clear fleet task`, `request fleet task retry` |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | No | No standalone plugin is introduced. |
| 6 | Has a user guide page? | Yes | docs/guide/fleet-config.md |
| 7 | Wire format changed? | Yes | docs/architecture/fleet-interface.md documents the added stale region and typed page answers. |
| 8 | Plugin SDK/protocol changed? | No | The generic plugin SDK and managed device wire protocol are unchanged. |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | No RFC behavior |
| 10 | Test infrastructure changed? | No | Use the existing managed and browser harnesses. |
| 11 | Affects daemon comparison? | No | The fleet UI introduces no routing protocol or daemon comparison. |
| 12 | Internal architecture changed? | Yes | `docs/architecture/fleet-interface.md` |
| 13 | Route metadata keys added/changed? | N-A | No route metadata keys |
| 14 | Prometheus counters added/changed? | No | This design adds no Prometheus metric. |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | The managed command registrations and the fleet page, part and event inventories. |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Update fleet-interface.md source anchors for the modified activity and stream handlers. |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | docs/guide/fleet-config.md and docs/guide/command-reference.md. |

## Design presentation

### What changes

The hub records work before it sends it to devices. The task panel and task commands use the same record to show progress, cancel outstanding work and retry failed targets. The alarm pages read the hub's visibility decision. A viewer receives changes only for actions or devices their current permissions allow.

### Before and after

Today Retry in the prototype reports that no task was started. With the real hub, if two devices succeeded and one failed, Retry returns a new task ID targeting just the failed device. The original task keeps all three outcomes. If the hub restarts partway through an operation, completed results survive and the interrupted task explains why it stopped. On the device Alarms tab an authorized viewer can follow the offline alarm to the component failure it hides; an operator without access to that device receives no event about it.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| The pages compute which alarm is hidden | The commands and the pages would disagree; `fleet-ui-8` computes it on the hub and the pages read `hidden-behind` |
| Fleet data over the broadcast broker | A broadcast would send one view's devices to every client (umbrella, "Existing Mechanisms") |

### Documents departed from

The Device tabs and Activity sections of `docs/architecture/fleet-interface.md` currently put an Alarms tab on the estate and group pages; device pages offer the dashboard and detail tabs. This design adds an Alarms tab on each device page. Its rows and counts come from the same filtered answer as the estate and group views. On 2026-11-25 Thomas chose “Add the device Alarms tab”. This records the navigation choice; the whole spec still requires independent review and owner sign-off.

### Choices for Thomas

| # | Choice | Options | Decision |
|---|--------|---------|----------|
| C-1 | Does an alarm that becomes shown after its cause clears raise a toast when critical? (was `fleet-ui-8` O-4) | (a) recommended, Yes: an operator who saw only "device offline" is told what is still wrong when the device is back; (b) No: a recovery that leaves five alarms raises no toast, and the list is the only place that says so | (a), 2026-11-25. Thomas selected “Show the critical toast”. |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- register one operational task owner and its commands, link the real activity and event adapters in the gated hub build, start an action through the dispatcher, then inspect its durable outcome through the command and browser. Exercise the sample and compiled-out builds.
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
| Durable model and commands | Restart with an active task, retry with the same token and verify a single linked child. Compare permitted results on all command surfaces and in the web panel. |
| Alarm navigation and counts | Compare estate, group and device tabs with the full paged command answer, including hidden rows. |
| Scoped updates and notifications | Exercise differently authorized viewers, group relabel, revoked access, coalescing, stream loss and cap exhaustion with a real feed. |

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
