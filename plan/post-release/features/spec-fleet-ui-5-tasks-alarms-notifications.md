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

Where alarms appear:

| Place | What it shows |
|-------|---------------|
| Alarms tab of the estate, a group node and a device | The umbrella's table stack: severity word with icon, device, alarm, detail, since, duration. Default order: severity, then oldest first. Filter fields: `severity`, `kind`, `device`, and the device fields of `fleet-ui-2` |
| Alarms card on dashboards | The most severe, with a link to the tab (`fleet-ui-3`) |
| Tree | A badge on a device row with its open alarm count; a group row's count turns to the warning colour |
| Toasts | One when a critical alarm opens on a device in the operator's current view |

A row's side panel shows the alarm's history on that device over the last week
and links to the device tab where the condition can be examined.

### Alarm causes

One failure opens several alarms. When a device goes offline, the health report
and the samples the hub holds for it are the last ones it received, so the
component and resource alarms of that device state what was true before the
failure, and the operator reads several rows for one event. The alarm list
then hides the one row that matters among rows that follow from it.

An alarm kind may name the kinds that cause it. While an alarm of a cause kind
is open on the same device, the alarm is **hidden behind** that cause. A hidden
alarm is still open, is still derived from what the hub holds, and is still in
the history. It has no row of its own: it is listed under its cause. When the
cause clears and the condition is still true, the alarm is shown.

Hiding is not silencing. No operator hides an alarm, nothing records that one
was hidden, and nothing has to be undone: whether an alarm is hidden is
computed, for each answer, from the set of open alarms and the registered
causes.

What a kind's registration adds:

| Part | Meaning |
|------|---------|
| Causes | The names of the kinds that can cause this one. None for most kinds. The kinds that cause a cause are causes too, so the declarations form a tree read from the leaf up |
| Settles | Whether the kind's figure needs time to become true again after a cause clears. Set by kinds that read a measured figure |

| Rule | Detail |
|------|--------|
| The kind that is hidden declares it | A kind names its own causes in its own registration. No list of causes exists anywhere else, so a later feature adds a kind and its causes without an edit to this spec's code |
| A declaration is a value | Names and one boolean. It holds no function, so it reads the same when a kind is declared by a plugin that runs in another process |
| Match | A cause matches when an alarm of that kind is open on the alarm's device. An alarm of the hub itself names no device, so a kind of the hub declares no cause |
| Refused at start | A cause name that no kind registered, a kind that is its own cause through any chain, and a cause less severe than the kind it would hide. Each is a defect of the build's composition and is reported with both kind names, as the other registries of the interface report theirs. A misspelt cause would otherwise hide nothing and say nothing, and a warning must never hide a critical alarm |
| Bound | A hidden alarm counts toward the bound on open alarms (`AlarmsMax`), because it is open |

The causes among the kinds this spec lists:

| Kind | Caused by | Why |
|------|-----------|-----|
| Component down | Device offline | The health report is the last one received. Nothing says the component is down now |
| Component degraded | Device offline | The same |
| Resource high | Device offline | No sample arrives while the device is offline. Settles |

Configuration diverged, Configuration rejected and Software behind name no
cause: the hub holds those facts itself, and they stay true and stay the
operator's to fix while the device is offline. History full is the hub's own.
Relay not granted is decision O-1.

How an alarm moves between its three conditions:

| From | Event | To | What the operator is told |
|------|-------|----|---------------------------|
| Not open | The condition becomes true, no cause open on the device | Shown | The alarm-opened event; a toast when critical |
| Not open | The condition becomes true, a cause is open on the device | Hidden | The event that marks the lists stale; no toast |
| Shown | A cause opens on the device | Hidden | The event that marks the lists stale; no toast, and no cleared event, because nothing cleared |
| Hidden | The cause clears, the condition is still true, the kind does not settle | Shown | As an opening: the event, and a toast when critical. `Since` stays the time the condition became true |
| Hidden | The cause clears and the kind settles | Hidden until the settle rule below is met, then Shown if the condition is true | As above, when shown |
| Hidden | The condition stops being true | Not open | The history row only |
| Shown | The condition stops being true | Not open | The alarm-cleared event |

The third row is where this differs from a product that stores alarms as sent
messages: an alarm already shown is hidden when its cause opens later. That
follows from "Alarms are derived, not stored events". Keeping it shown would
need a record of what was shown before, which is the second state that ruling
refuses. The operator loses nothing: a resource alarm that opened two minutes
before its device went offline is listed under the offline alarm with its own
`Since`, which is where it explains the failure.

**Settling.** A measured figure can stay high for some minutes after its device
is back. A kind that settles counts its configured time again from the moment
its cause cleared: Resource high is shown only when the figure has stayed above
the threshold for the configured time, in samples received after the device
came back. No second timer and no new setting are added (decision O-2).

Where a hidden alarm appears:

| Place | A hidden alarm |
|-------|----------------|
| Alarms tab | No row of its own. The row of the shown alarm at the top of its chain says how many alarms are hidden behind it, and that row's side panel lists them: kind, summary, since. The line above the table gives both figures, for example "9 open, 3 hidden behind their cause" |
| Alarms tab with the filter term `has:cause` | The hidden alarms as rows, with the cause each is hidden behind. This is the table every count of hidden alarms links to |
| Tree badge, the device's open alarm count, a group row's colour, the Alarms card | Not counted. The Alarms card adds one line with the count of hidden alarms, linked to `has:cause` |
| The device side panel's open alarms | Listed under its cause |
| Toasts | None while hidden |
| Alarm history | Its row, marked as hidden for the time a cause was open. The mark is computed from the cause's own history rows on that device, so the history stores nothing new |
| `show fleet alarm` | Every open alarm, hidden ones included. A hidden alarm carries `hidden-behind`, the identifier of the nearest open cause alarm. A shown alarm does not carry the field. The filter `hidden` selects one set or the other |
| Live events | An alarm that becomes hidden or shown makes the same fragments stale as one that opens or clears |

A count and the rows behind its link cannot differ
(`docs/architecture/fleet-interface.md`, "A source answers facts, the package
derives"): every count of shown alarms is taken on the default table, and every
count of hidden alarms on the `has:cause` table.

**Checked against this spec's own statements.**

| Statement | Effect |
|-----------|--------|
| "Nothing is acknowledged or silenced", "No acknowledgement" | Kept. Hiding is computed from the open alarms and holds no state an operator set |
| "Alarms are derived, not stored events" | Kept, and it decides two points: an alarm shown first is hidden when its cause opens later, and there is no "raise again" step when a cause clears, because the alarm never stopped being open |
| "Every condition that needs attention is listed once" | Kept. A hidden alarm is listed once, under its cause |
| AC-4, "appears in every place listed" | Narrowed: a hidden alarm appears where the table above says. AC-4 names it as a permitted survivor |
| `Alarm` names one device and no other target | The cause match is by device only. A match on an interface or on either end of a tunnel has no alarm kind to serve today (decision O-3) |

**Existing mechanisms.**

| Mechanism | Where | What it does today | This design |
|-----------|-------|--------------------|-------------|
| The alarm-kind registry of this spec | "Alarms" above: a registered entry with its condition, severity and sentence | Planned, not built. Makes the list of kinds open | Extends: two more parts on the same entry, Causes and Settles. No second registry |
| The alarm tab registry | `internal/component/web/fleetui/handler_alarms.go` `mustRegisterAlarmTab`, filled by `register_activity_pages.go` `registerAlarmTabs` | Maps a kind's name to the device tab where it is examined. Keyed by kind name, panics on a kind registered twice | Leaves. It answers a page question for a kind and stays the page's. The causes belong to the hub's kind entry, because `show fleet alarm` answers them with no page |
| `ActivitySource`, `Alarm`, `AlarmEvent` | `internal/component/web/fleetui/source_activity.go`; the same methods as `ActivityPart` in `plan/post-release/features/spec-fleet-1-device-registry.md` | `Alarms` answers the open alarms, at most `AlarmsMax`. `Alarm` holds one device and no cause | Extends: `Alarm` gains the identifier of the alarm it is hidden behind, empty when shown, and `AlarmEvent` gains the hidden mark. The methods, their arguments and their bounds do not change, so `fleet-1`'s part contract stands. The source answers the fact; the counts are derived in the package, as every other count is |
| The display state precedence | `internal/component/web/fleetui/state.go` `DeviceDisplayState` | One state per device, by precedence: offline, diverged, down, degraded. An offline device already shows as offline whatever its last health report says | Leaves, and agrees with it: the alarm list will say what the state mark already says. The precedence is a fixed order of one device's states. The causes are declared per kind, because the kinds are an open set |
| The sample alarms | `internal/component/web/fleetui/sample/activity.go` `alarmRows`, `alarmsAt` | Twelve open alarms and a script in which a device goes offline and comes back | Extends: the sample answers the hidden-behind fact for its rows, so the prototype shows the hidden form and a browser test can step through it |
| The report bus | `internal/core/report/report.go` `RaiseWarning`, `ClearWarning` | One active warning per (source, code, subject) on a device; raising again refreshes it. Codes are free strings with no registry | Leaves. The hub's alarms keep the same identity shape, a kind and its target. The bus gets no causes: it has no registered kind to declare one on, and its readers are one device's own commands |
| The stand-in finding of `ze doctor` | `docs/architecture/doctor-and-health-checks.md`; `show.go` `collectDoctorChecksFrom` | When a plugin cannot answer its checks, one `doctor-plugin-unreachable` finding stands in for them | Leaves. It is the same idea on one device: one finding for the cause, not a guess about each thing behind it. It replaces answers that cannot be had; an alarm cause hides answers the hub does hold, so the two do not share code |
| `stats.Hysteresis` | `internal/core/stats/hysteresis.go`; used by the host trend monitor and the DDoS detector | Confirms a condition after a count of holding evaluations and clears it after a count of failing ones | Applies to Settles, at design: "the configured time counts again" is a machine returned to idle when the cause clears. Whether Resource high runs one machine per device or reads the stored samples is `fleet-8`'s question |

**Open decisions (owner).**

| ID | Question | Recommended | What each answer changes |
|----|----------|-------------|--------------------------|
| O-1 | Is Relay not granted caused by Device offline? | No | Yes: the informational row leaves the list while the device is offline, and returns with it. No: it stays, because the grant is a fact the hub was told and the operator can prepare the fix while the device is away |
| O-2 | How long does a settling alarm wait after its cause clears? | The kind's own configured time, counted again from the clear | The kind's own time: no new setting, and the wait is as long as the operator already chose for that alarm. A separate settle time per kind: one more setting, and a short alarm time can be combined with a long wait after recovery. No wait: a figure that is still high in the first sample after recovery is shown at once |
| O-3 | Does the cause match carry targets other than one device now? | No, until a kind needs it | Now: `Alarm` gains an interface target and a list of devices, so a tunnel alarm is hidden when either end is offline, and the part contract of `fleet-1` changes for kinds that do not exist yet. Later: the match is by device only, and the first kind about an interface or a tunnel adds its target with its own design |
| O-4 | Does an alarm that becomes shown after its cause clears raise a toast when critical? | Yes | Yes: an operator who saw only "device offline" is told what is still wrong when the device is back. No: a recovery that leaves five alarms raises no toast, and the list is the only place that says so |

**Assumption to settle with `fleet-4` and `fleet-8`.** The hub keeps the last
health report and the last sample of a device that is offline. Neither spec
says so today. If either drops them when the device disconnects, the alarms
read from them clear at that moment and the matching cause row above has
nothing to hide.

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
| `show fleet alarm` | Open alarms, filterable by severity, kind and device, and by whether they are hidden behind a cause |
| `show fleet alarm history device <name>` | A device's alarm openings and clearings over the retention period |

### Key design decisions

| Decision | Detail |
|----------|--------|
| Alarms are derived, not stored events | The hub already holds every fact an alarm states. Deriving them means an alarm can never disagree with the device's state shown beside it |
| No acknowledgement | An acknowledged alarm is a second state to keep true. The first version shows what is wrong now; a fixed condition clears itself |
| Causes are declared by the kind that is hidden | A list of causes kept in one place is a second declaration of the kinds. Hiding is computed from the open alarms, so it is not a state either ("Alarm causes") |
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
| AC-4 | A condition in the alarm table becomes true, then false | The alarm opens, appears in every place listed, and clears from all of them | Its history row remains, because the history is the record that it happened. An alarm hidden behind its cause appears only where "Alarm causes" says, because its cause is the row to act on |
| AC-5 | A device changes state while a page is open | The tree row, the table row and the cards that count it change without a reload, within the bound fixed at design | Pages the operator is not viewing do not update, because they have no stream |
| AC-6 | An operator who may not see a device | No event about it reaches the operator's stream | None |
| AC-7 | The stream is lost | The stale banner shows with the time, the page reconnects, and the live fragments refresh once | None |
| AC-8 | `show fleet task` and `show fleet alarm` on any command surface | The same data as the pages, as data answering every pipe | None |
| AC-9 | A device with a component down goes offline | The device has one shown alarm, Device offline. Component down is open and hidden behind it: no row of its own, not in the tree badge or the Alarms card count, listed in the offline row's side panel, and present in `show fleet alarm` with `hidden-behind` naming the offline alarm | The `has:cause` table lists it, because that table is the hidden alarms |
| AC-10 | The device of AC-9 comes back and its report still names the component down | Component down is shown, with the `Since` of the moment the component went down, and a toast is raised as for an opening | None |
| AC-11 | A component alarm opens and its condition stops while the device is offline | The alarm is never shown and raises no toast. Its history row exists and is marked hidden | None |
| AC-12 | Resource high is shown, then its device goes offline | The row leaves the default table with no cleared event, and is listed under Device offline with its own `Since` | None |
| AC-13 | A device comes back and its first samples are above the resource threshold | Resource high stays hidden until the figure has stayed above the threshold for the configured time in samples received after the device came back, then is shown | A figure that falls below the threshold in that time opens nothing |
| AC-14 | A device is offline and its configuration is diverged or rejected, or its software is behind | Those alarms are shown beside Device offline | They name no cause, because the hub holds the fact and the operator can act on it |
| AC-15 | A kind registered with a cause name no kind holds, with a chain of causes that returns to itself, or with a cause less severe than itself | The hub refuses to start its alarms and names both kinds and the defect | None |
| AC-16 | A later feature registers a kind that names Device offline as its cause | Its alarms are hidden behind Device offline with no edit to a file of this spec | None |
| AC-17 | Any count of alarms on a page, shown or hidden | Following its link lists exactly that many rows | None |

### Draft tests for the alarm causes

The package of the hub's alarm model is fixed at design, so the unit tests are
named and not yet placed.

| Test | Kind | Proves |
|------|------|--------|
| `TestAlarmHiddenBehindOpenCause` | Unit, over the function that computes hiding from the open alarms and the registry | AC-9, AC-12, AC-14: table cases of open sets and the alarms each one shows and hides, with the nearest open cause named |
| `TestAlarmShownWhenCauseClears` | Unit | AC-10, AC-11: `Since` is kept, and an alarm that clears while hidden was never shown |
| `TestAlarmSettlesAfterCauseClears` | Unit | AC-13: hidden until the configured time has passed in samples after the clear, at the boundary one sample short and one sample past |
| `TestAlarmKindRegistryRefusesBadCauses` | Unit | AC-15: an unknown cause, a cycle of two and of three kinds, a warning that would hide a critical |
| `TestAlarmKindRegisteredElsewhereIsHidden` | Unit, with a kind registered from the test | AC-16 |
| `TestAlarmCountsMatchTheirRows` | Unit in `internal/component/web/fleetui`, beside `TestDashboardAlarmLinksCountTheirRows` | AC-17: the tree badge, the card count and the hidden count against the tables their links open |
| `test/web/fleet-alarm-cause.wb` | Browser test over the sample source, stepped | AC-9, AC-10, AC-12 as the operator sees them: the row leaves, the offline row carries the count, the side panel lists it, and it returns when the device does |
| `test/managed/alarm-hidden-behind-offline.ci` | Functional, a real hub and a real managed client that reports a component down and is then stopped and started | AC-9, AC-10 through the hub's own path: `show fleet alarm` on the hub answers `hidden-behind` while the client is stopped and not after it is back. Red when the hiding is computed only in the page |
