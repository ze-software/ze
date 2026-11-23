# Spec: fleet-ui-8 -- Alarm Causes

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | fleet, cli |
| Priority | P2 |
| Release reason | Thomas put this fleet capability (alarm causes: an alarm hidden while the alarm that causes it is open) in the first release, reached through `show fleet alarm` on every command surface (`ai/META-DECISIONS.md`, "Goal and scope"); existing supported behavior and its obligations remain required. How the pages show a hidden alarm comes after the first release, in `plan/post-release/features/spec-fleet-ui-9-tasks-notifications-and-alarm-pages.md`. |
| RFC gaps | - |
| Depends | plan/release/features/spec-fleet-ui-5-tasks-alarms-notifications.md, plan/release/features/spec-fleet-1-device-registry.md, plan/release/features/spec-fleet-4-inventory-health.md, plan/release/features/spec-fleet-8-metric-history.md |
| Phase | 8/9 of plan/post-release/features/spec-fleet-ui-0-umbrella.md |
| Handoff | - |
| Design tier | strongest tier |
| Implementation tier | development tier |
| Updated | 2026-11-23 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Thomas asked for the alarm cause hierarchy as a spec of its own, after a
review of flexiWAN showed its controller hides derived alarms (tunnel,
latency) while their cause (device offline) is open, and on 2026-11-23 put it
in the first release. The design was first
written into `fleet-ui-5` (commit `526e749e13`) and is moved here unchanged;
`fleet-ui-5` keeps the alarm list, and this spec refines it.

On 2026-11-23 Thomas also ruled that the fleet features of the first release
are reached through commands, and that their web pages come after it
(`ai/META-DECISIONS.md`, "Goal and scope"). This spec keeps the cause model,
its refusals, the `hidden-behind` field and the `hidden` filter of
`show fleet alarm`, and the hidden mark of the alarm history. How the pages
show a hidden alarm (the side panel listing under its cause, the `has:cause`
table filter, the hidden count of the Alarms card, the badge exclusion, the
toast rules and choice O-4) moved, unchanged, to
`plan/post-release/features/spec-fleet-ui-9-tasks-notifications-and-alarm-pages.md`,
"Alarm causes on the pages" (see "Moved to fleet-ui-9" below).

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
| Kinds registered before this spec | This spec writes the Causes and Settles parts into the registration of each `fleet-ui-5` kind below that names a cause, in that kind's own registration file. A kind another spec registers after this one writes its own |

The causes among the kinds `fleet-ui-5` lists, and the kinds other specs add:

| Kind | Caused by | Why |
|------|-----------|-----|
| Component down | Device offline | The health report is the last one received. Nothing says the component is down now |
| Component degraded | Device offline | The same |
| Resource high | Device offline | No sample arrives while the device is offline. Settles |
| Configuration resync failed (`fleet-10`) | Device offline | No resync runs while the device is offline, and its reconnect starts one, so the failure is of an attempt the reconnect replaces. Declared in `fleet-10`'s own registration of the kind, not here (`plan/release/features/spec-fleet-10-drift-hash-chain.md`, AC-2); it is the first kind of another spec that AC-8 covers |

Configuration diverged, Configuration rejected and Software behind name no
cause: the hub holds those facts itself, and they stay true and stay the
operator's to fix while the device is offline. History full is the hub's own.
Relay not granted is decision O-1.

How an alarm moves between its three conditions. What the pages and the
toasts do at each move is `fleet-ui-9`'s, in "Alarm causes on the pages".

| From | Event | To | What `show fleet alarm` and the alarm history answer |
|------|-------|----|------------------------------------------------------|
| Not open | The condition becomes true, no cause open on the device | Shown | Listed with no `hidden-behind`; the history holds its opening |
| Not open | The condition becomes true, a cause is open on the device | Hidden | Listed with `hidden-behind`; the history holds its opening, marked hidden |
| Shown | A cause opens on the device | Hidden | Gains `hidden-behind`. The history holds no clearing, because nothing cleared |
| Hidden | The cause clears, the condition is still true, the kind does not settle | Shown | Loses `hidden-behind`. `Since` stays the time the condition became true |
| Hidden | The cause clears and the kind settles | Hidden until the settle rule below is met, then Shown if the condition is true | As above, when shown |
| Hidden | The condition stops being true | Not open | Leaves `show fleet alarm`; its history row remains, marked hidden |
| Shown | The condition stops being true | Not open | Leaves `show fleet alarm`; the history holds its clearing |

The third row is where this differs from a product that stores alarms as sent
messages: an alarm already shown is hidden when its cause opens later. That
follows from `fleet-ui-5`'s "Alarms are derived, not stored events". Keeping
it shown would need a record of what was shown before, which is the second
state that ruling refuses. The operator loses nothing: a resource alarm that
opened two minutes before its device went offline is listed under the offline
alarm with its own `Since`, which is where it explains the failure.

**Settling.** A measured figure can stay high for some minutes after its device
is back. A kind that settles counts its configured time again from the moment
its cause cleared: Resource high is shown only when the figure has stayed above
the threshold for the configured time, in samples received after the device
came back. No second timer and no new setting are added (decision O-2).

Where a hidden alarm appears in the commands. Where it appears on the pages
(the Alarms tab, the `has:cause` table, the tree badge, the Alarms card, the
device side panel, the toasts and the live events) is `fleet-ui-9`'s:

| Place | A hidden alarm |
|-------|----------------|
| Alarm history (`show fleet alarm history device <name>`) | Its row, marked as hidden for the time a cause was open. The mark is computed from the cause's own history rows on that device, so the history stores nothing new |
| `show fleet alarm` | Every open alarm, hidden ones included. A hidden alarm carries `hidden-behind`, the identifier of the nearest open cause alarm. A shown alarm does not carry the field. The filter `hidden` selects one set or the other |

### What this spec changes in `fleet-ui-5`

`fleet-ui-5` is left as it is written. Each change below is this spec's work,
carried by its own acceptance criterion, and made to whatever page documents
`fleet-ui-5`'s alarm list once that spec has landed. The changes to the alarm
pages (the `has:cause` term of the Alarms tab filter, the Alarms card line) are
`fleet-ui-9`'s.

| `fleet-ui-5` statement | Change this spec makes | AC |
|------------------------|------------------------|----|
| AC-4, the alarm "opens in `show fleet alarm` ... then clears from it" | Narrowed for a hidden alarm: it is listed with `hidden-behind` while its cause is open, and its history row is marked hidden. The page part of the old AC-4 is `fleet-ui-9` AC-4, which `fleet-ui-9` AC-15 narrows the same way | AC-10 |
| "An alarm kind is a registered entry with its condition, its severity and its sentence" | The entry gains Causes and Settles. No second registry | AC-7, AC-8 |
| `show fleet alarm`: "Open alarms, filterable by severity, kind and device" | Gains the filter `hidden` and, on a hidden alarm, the field `hidden-behind` | AC-11 |

**Checked against `fleet-ui-5`'s own statements.**

| Statement | Effect |
|-----------|--------|
| "Nothing is acknowledged or silenced", "No acknowledgement" | Kept. Hiding is computed from the open alarms and holds no state an operator set |
| "Alarms are derived, not stored events" | Kept, and it decides two points: an alarm shown first is hidden when its cause opens later, and there is no "raise again" step when a cause clears, because the alarm never stopped being open |
| "Every condition that needs attention is listed once" | Kept. A hidden alarm is listed once, under its cause |
| AC-4, the alarm "opens in `show fleet alarm`" | Narrowed by AC-10 of this spec: a hidden alarm is listed there with `hidden-behind` |
| `Alarm` names one device and no other target | The cause match is by device only. A match on an interface or on either end of a tunnel has no alarm kind to serve today (decision O-3) |

### Moved to fleet-ui-9

Every item below is owned by
`plan/post-release/features/spec-fleet-ui-9-tasks-notifications-and-alarm-pages.md`,
which names this spec as the source of each. The identifiers of the ACs that
stay here are unchanged.

| Item moved | Was here | Now in `fleet-ui-9` |
|------------|----------|---------------------|
| AC-1, page part: no row of its own, not in the tree badge or the Alarms card count, listed in the offline row's side panel, listed by `has:cause` | AC-1 | AC-10 |
| AC-2, page part: the toast raised as for an opening | AC-2 | AC-11 |
| AC-3, page part: no toast | AC-3 | AC-12 |
| AC-4, page part: the row leaves the default table with no cleared event and is listed under Device offline | AC-4 | AC-13 |
| AC-9, every count of alarms on a page matches the rows behind its link | AC-9 | AC-14 |
| AC-10, page part: the hidden alarm appears only where the pages say, and clears from all of them | AC-10 | AC-15 |
| AC-12, the `has:cause` table | AC-12 | AC-16 |
| The page rows of "Where a hidden alarm appears", and the rule that a count and the rows behind its link cannot differ | "Alarm causes" | "Alarm causes on the pages" |
| The pages' and toasts' column of the move table ("What the operator is told") | "Alarm causes" | "Alarm causes on the pages" |
| The `fleet-ui-5` changes to the Alarms tab filter (`has:cause`) and to the Alarms card | "What this spec changes in `fleet-ui-5`" | "Alarm causes on the pages" |
| Choice O-4, the toast when an alarm is shown again | "Choices for Thomas" | "Choices for Thomas", C-1 |
| `ActivitySource` and the sample carrying the hidden-behind fact to the pages (`source_activity.go`, `sample/activity.go`) | "Files to Modify", "Existing Mechanisms" | "Files to Modify", "Existing Mechanisms" |
| `test/web/fleet-alarm-cause.wb`, `TestAlarmCountsMatchTheirRows` | "Wiring Test", "🧪 TDD Test Plan" | "Wiring Test", "🧪 TDD Test Plan" |
| R-2, a count and the rows behind its link differ | "Risks" | "Risks" |
| Authorization of a hidden alarm in a count or a side panel | "Security Review Checklist" | "Security Review Checklist" |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/api-surfaces.md`, `ai/doors/commands-cli.md`, `ai/doors/hub-engine.md`, `ai/doors/core.md` - the web surface, the command registry, the managed hub, the report bus
  → Constraint: a feature registers and is discovered; the causes live on each kind's own registration, never in a central list
  → Constraint: a declaration that crosses a process boundary is a pure value
- [ ] `plan/release/features/spec-fleet-ui-5-tasks-alarms-notifications.md` - the alarm list this spec refines
  → Decision: alarms are derived from what the hub holds, never stored events; no acknowledgement
  → Constraint: an alarm kind is a registered entry with its condition, severity and sentence
- [ ] `plan/post-release/features/spec-fleet-ui-9-tasks-notifications-and-alarm-pages.md` - the pages that show what this spec computes
  → Constraint: the pages read `hidden-behind` from the hub and compute no hiding of their own
- [ ] `docs/architecture/fleet-interface.md` - the fleet interface as built by `fleet-ui-1`
  → Constraint: "A source answers facts, the package derives": `hidden-behind` is a fact the hub answers; the counts are derived by the pages (`fleet-ui-9`)
- [ ] `plan/release/features/spec-fleet-1-device-registry.md` - the `ActivityPart` contract
  → Constraint: the part's methods, arguments and bounds stay as `fleet-1` fixes them
- [ ] `plan/release/features/spec-fleet-4-inventory-health.md`, `plan/release/features/spec-fleet-8-metric-history.md` - the health reports and samples the caused kinds read
  → Constraint: assumption A-1 is settled with these two specs

**Key insights:**
- Hiding is computed for each answer from the open alarms and the registry, so it holds no state.
- `DeviceDisplayState` already shows an offline device as offline whatever its last report says; the alarm list will say the same.
- The alarm kind names in use today are those `registerAlarmTabs` registers: `device-offline`, `component-down`, `component-degraded`, `config-diverged`, `config-rejected`, `resource-high`, `software-behind`, `relay-not-granted`, `history-full`.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/web/fleetui/handler_alarms.go` - `mustRegisterAlarmTab` maps a kind's name to the device tab where it is examined; panics on an empty kind or tab and on a kind registered twice
- [ ] `internal/component/web/fleetui/register_activity_pages.go` - `registerAlarmTabs` registers the tab of the nine kinds the hub derives
- [ ] `internal/component/web/fleetui/source_activity.go` - `ActivitySource`, `Alarm` (ID, Kind, Severity, Device, Summary, Detail, Since), `AlarmEvent`; `Alarms` answers the open alarms, at most `AlarmsMax` (1024)
- [ ] `internal/component/web/fleetui/state.go` - `DeviceDisplayState`, one state per device by precedence: offline, diverged, down, degraded, then healthy or unknown
- [ ] `internal/component/web/fleetui/sample/activity.go` - `alarmRows`, `alarmsAt`: twelve open alarms and a script in which a device goes offline and comes back
- [ ] `internal/core/report/report.go` - `RaiseWarning`, `ClearWarning`: one active warning per (source, code, subject); raising again refreshes it
- [ ] `internal/core/stats/hysteresis.go` - `Hysteresis`: confirms a condition after a count of holding evaluations, clears after a count of failing ones; `Reset` returns it to idle

**Behavior to preserve:**
- Every behavior of `fleet-ui-5`'s alarm list for an alarm whose device has no open cause
- `fleet-1`'s `ActivityPart` methods, arguments and bounds
- The alarm tab registry of the page, keyed by kind name

**Behavior to change:**
- An alarm whose cause is open on its device is hidden behind it, in `show fleet alarm` and its history as "Where a hidden alarm appears in the commands" says
- The changes to `fleet-ui-5` listed in "What this spec changes in `fleet-ui-5`"

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| The alarm-kind registry of `fleet-ui-5` | `plan/release/features/spec-fleet-ui-5-tasks-alarms-notifications.md`, "Alarms": a registered entry with its condition, severity and sentence | Planned, not built. Makes the list of kinds open | Extends: two more parts on the same entry, Causes and Settles. No second registry |
| The alarm tab registry | `internal/component/web/fleetui/handler_alarms.go` `mustRegisterAlarmTab`, filled by `register_activity_pages.go` `registerAlarmTabs` | Maps a kind's name to the device tab where it is examined. Keyed by kind name, panics on a kind registered twice | Does not apply: it answers a page question for a kind and stays the page's. The causes belong to the hub's kind entry, because `show fleet alarm` answers them with no page |
| `ActivitySource`, `Alarm`, `AlarmEvent` | `internal/component/web/fleetui/source_activity.go`; the same methods as `ActivityPart` in `plan/release/features/spec-fleet-1-device-registry.md` | `Alarms` answers the open alarms, at most `AlarmsMax`. `Alarm` holds one device and no cause | Does not apply here: carrying the hidden-behind fact to the pages is `fleet-ui-9`'s (its "Existing Mechanisms"). This spec makes the hub answer the fact in `show fleet alarm` |
| The display state precedence | `internal/component/web/fleetui/state.go` `DeviceDisplayState` | One state per device, by precedence: offline, diverged, down, degraded. An offline device already shows as offline whatever its last health report says | Does not apply, and agrees with it: the alarm list will say what the state mark already says. The precedence is a fixed order of one device's states. The causes are declared per kind, because the kinds are an open set |
| The sample alarms | `internal/component/web/fleetui/sample/activity.go` `alarmRows`, `alarmsAt` | Twelve open alarms and a script in which a device goes offline and comes back | Does not apply here: the sample answering the hidden-behind fact for the prototype is `fleet-ui-9`'s |
| The report bus | `internal/core/report/report.go` `RaiseWarning`, `ClearWarning` | One active warning per (source, code, subject) on a device; raising again refreshes it. Codes are free strings with no registry | Does not apply: the hub's alarms keep the same identity shape, a kind and its target. The bus gets no causes: it has no registered kind to declare one on, and its readers are one device's own commands |
| The stand-in finding of `ze doctor` | `docs/architecture/doctor-and-health-checks.md`; `show.go` `collectDoctorChecksFrom` | When a plugin cannot answer its checks, one `doctor-plugin-unreachable` finding stands in for them | Does not apply: it is the same idea on one device, one finding for the cause, not a guess about each thing behind it. It replaces answers that cannot be had; an alarm cause hides answers the hub does hold, so the two do not share code |
| `stats.Hysteresis` | `internal/core/stats/hysteresis.go`; used by the host trend monitor and the DDoS detector | Confirms a condition after a count of holding evaluations and clears it after a count of failing ones | Extends, at design: "the configured time counts again" is a machine returned to idle when the cause clears. Whether Resource high runs one machine per device or reads the stored samples is `fleet-8`'s question |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `docs/architecture/fleet-interface.md` | A source answers facts, the package derives | The `// Design:` page of `source_activity.go` | "A source answers facts only" | agrees | The hub answers `hidden-behind` as a fact in `show fleet alarm`; every count of shown or hidden alarms is derived by the pages from the table its link opens (`fleet-ui-9` AC-14) |
| `ai/doors/system.md` | The core knows nothing, and everything registers | System door | "A feature registers from `init()` in its own `register.go` and is discovered" | agrees | Each kind declares its causes in its own registration; no list of causes exists anywhere else (AC-8) |
| `ai/doors/system.md` | A plugin can run in another process | System door | "A declaration is a pure value." | agrees | A cause declaration is names and one boolean, so it reads the same from a plugin in another process |
| `ai/doors/le-tooling.md` | Where it sits | The door covering `test/managed/alarm-hidden-behind-offline.ci` | "Functional `.ci` files live under `test/<suite>/` and run through" | agrees | `alarm-hidden-behind-offline` lives under `test/managed/` and runs through `./le test managed` |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An alarm of a cause kind opens or clears on a device, on the hub
- A request for alarms: `show fleet alarm` or `show fleet alarm history device <name>` on any command surface

### Transformation Path
1. The hub derives the open alarms from what it holds, as `fleet-ui-5` does.
2. For each open alarm, the hub reads its kind's registered causes and finds the nearest open alarm of a cause kind on the same device; that alarm's identifier is `hidden-behind`.
3. A kind that settles stays hidden after its cause clears until its configured time has passed in samples received after the clear.
4. `show fleet alarm` answers the alarms with the hidden-behind fact, and the alarm history marks the rows of the time a cause was open. The pages of `fleet-ui-9` read the same fact and derive their counts from it.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Hub alarm model ↔ Command surfaces | `show fleet alarm` through the command dispatcher, with the `hidden` filter and `hidden-behind` field | No |

### Changed-Wire Census

**Search evidence:** [pending research: the `show fleet alarm` answer shape of `fleet-ui-5`]

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| `show fleet alarm` answer | [pending research] | [pending research] | [pending research] | Change: adds `hidden-behind` on a hidden alarm |

### Integration Points
- The alarm-kind registry of `fleet-ui-5` - gains the Causes and Settles parts
- The `show fleet alarm` answer of `fleet-ui-5` - carries the hidden-behind fact, which `fleet-ui-9`'s pages read
- The kinds other specs register: `fleet-10`'s Configuration resync failed declares Device offline as its cause in its own registration

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
| A-1 | The hub keeps the last health report and the last sample of a device that is offline | Neither `plan/release/features/spec-fleet-4-inventory-health.md` nor `plan/release/features/spec-fleet-8-metric-history.md` says so today | If either drops them when the device disconnects, the alarms read from them clear at that moment and the matching cause row has nothing to hide | Settled with `fleet-4` and `fleet-8` at their design; a unit test in each that a disconnect keeps the last report and sample | unvalidated |
| A-2 | `fleet-ui-5` builds the alarm kinds as registered entries, as it plans | `plan/release/features/spec-fleet-ui-5-tasks-alarms-notifications.md`, "Alarms": "an alarm kind is a registered entry" | Causes have no entry to live on | `fleet-ui-5`'s implementation, read before this spec's design | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A misspelt or cyclic cause hides nothing, or hides forever | An alarm never shown, or never hidden, in the sample script | Refused at start with both kind names (AC-7) |

R-2, a count and the rows behind its link differing, moved to `fleet-ui-9`.

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | [pending design] |
| How is it reverted? | [pending design] |
| Who else touches this path? | `fleet-ui-5` (the alarm list), `fleet-ui-9` (the alarm pages and their counts), `fleet-ui-3` (the Alarms card), `fleet-10` (a kind that declares a cause) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `show fleet alarm` on a hub whose managed client reports a component down and is stopped | → | The hub computes hiding from the open alarms and the registry | `test/managed/alarm-hidden-behind-offline.ci` |

The browser wiring test `test/web/fleet-alarm-cause.wb` moved to `fleet-ui-9`.

## Acceptance Criteria

The identifiers are unchanged. AC-9 and AC-12 moved whole to `fleet-ui-9`;
AC-1, AC-2, AC-3, AC-4 and AC-10 keep their command part here and their page
part moved ("Moved to fleet-ui-9").

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A device with a component down goes offline | The device has one shown alarm, Device offline. Component down is open and hidden behind it: `show fleet alarm` lists it with `hidden-behind` naming the offline alarm, and lists Device offline with no `hidden-behind` | `show fleet alarm` with the filter `hidden` lists it, because that filter selects the hidden alarms |
| AC-2 | The device of AC-1 comes back and its report still names the component down | `show fleet alarm` lists Component down with no `hidden-behind`, with the `Since` of the moment the component went down | None |
| AC-3 | A component alarm opens and its condition stops while the device is offline | `show fleet alarm` lists the alarm only with `hidden-behind` while it is open. Its history row exists and is marked hidden | None |
| AC-4 | Resource high is shown, then its device goes offline | `show fleet alarm` lists it with `hidden-behind` naming Device offline and its own `Since`, and the alarm history holds no clearing for it | None |
| AC-5 | A device comes back and its first samples are above the resource threshold | Resource high stays hidden until the figure has stayed above the threshold for the configured time in samples received after the device came back, then is shown | A figure that falls below the threshold in that time opens nothing |
| AC-6 | A device is offline and its configuration is diverged or rejected, or its software is behind | Those alarms are shown beside Device offline | They name no cause, because the hub holds the fact and the operator can act on it |
| AC-7 | A kind registered with a cause name no kind holds, with a chain of causes that returns to itself, or with a cause less severe than itself | The hub refuses to start its alarms and names both kinds and the defect | None |
| AC-8 | A later feature registers a kind that names Device offline as its cause | Its alarms are hidden behind Device offline with no edit to a file of this spec | None |
| AC-10 | A condition in `fleet-ui-5`'s alarm table becomes true while a cause is open on its device, then false | `show fleet alarm` lists it with `hidden-behind` while it is open and no longer lists it once the condition is false. This narrows `fleet-ui-5` AC-4 for a hidden alarm | Its history row remains, marked hidden, because the history is the record that it happened |
| AC-11 | `show fleet alarm` on any command surface, with and without the filter `hidden` | Unfiltered, every open alarm is listed and each hidden one carries `hidden-behind` with the identifier of the nearest open cause alarm; the filter selects the hidden set or the shown set | A shown alarm carries no `hidden-behind`, because it is hidden behind nothing |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Sees one offline alarm instead of the stale alarms of the device behind it, and sees them return when the device does | Hub alarm model, registry causes, `show fleet alarm` | `test/managed/alarm-hidden-behind-offline.ci` |

The same story on the Alarms tab is `fleet-ui-9`'s, proved by `test/web/fleet-alarm-cause.wb`.

## 🧪 TDD Test Plan

### Unit Tests

The package of the hub's alarm model is fixed at `fleet-ui-5`'s design, so the
unit tests are named and not yet placed. `TestAlarmCountsMatchTheirRows`
moved to `fleet-ui-9`.

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestAlarmHiddenBehindOpenCause` | [pending design: the hub alarm model package], over the function that computes hiding from the open alarms and the registry | AC-1, AC-4, AC-6: table cases of open sets and the alarms each one shows and hides, with the nearest open cause named | |
| `TestAlarmShownWhenCauseClears` | [pending design] | AC-2, AC-3: `Since` is kept, and an alarm that clears while hidden was never shown | |
| `TestAlarmSettlesAfterCauseClears` | [pending design] | AC-5: hidden until the configured time has passed in samples after the clear, at the boundary one sample short and one sample past | |
| `TestAlarmKindRegistryRefusesBadCauses` | [pending design] | AC-7: an unknown cause, a cycle of two and of three kinds, a warning that would hide a critical | |
| `TestAlarmKindRegisteredElsewhereIsHidden` | [pending design], with a kind registered from the test | AC-8 | |
| `TestAlarmShowHiddenFilter` | [pending design: the `show fleet alarm` handler package] | AC-10, AC-11: the `hidden` filter and the `hidden-behind` field on hidden and shown alarms | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Settle time after a cause clears | The kind's configured time (decision O-2) | One sample past the configured time shows the alarm | One sample short keeps it hidden | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `alarm-hidden-behind-offline` | `test/managed/alarm-hidden-behind-offline.ci` | A real hub and a real managed client that reports a component down and is then stopped and started. AC-1, AC-2, AC-11 through the hub's own path: `show fleet alarm` on the hub answers `hidden-behind` while the client is stopped and not after it is back. Red when the hiding is computed only in the page | |

The browser test `fleet-alarm-cause` (`test/web/fleet-alarm-cause.wb`) moved to `fleet-ui-9`.

## Files to Modify
- [pending design: the hub alarm model and the kind registrations `fleet-ui-5` creates, the `show fleet alarm` YANG and handler]

`internal/component/web/fleetui/source_activity.go` and
`internal/component/web/fleetui/sample/activity.go`, which carry the fact to
the pages, moved to `fleet-ui-9`.

## Files to Create
- `test/managed/alarm-hidden-behind-offline.ci` - functional test through the hub
- [pending design]

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `show fleet alarm` gains the `hidden` filter and the `hidden-behind` answer leaf |
| YANG validation constraints | | |
| YANG custom validators | | |
| CLI commands/flags | Yes | `show fleet alarm` filter |
| CLI grammar (keyword before value) | | |
| Editor autocomplete | | |
| Functional test for new RPC/API | Yes | `test/managed/alarm-hidden-behind-offline.ci` |
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
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md`, `show fleet alarm` |
| 4 | API/RPC added/changed? | | |
| 5 | Plugin added/changed? | | |
| 6 | Has a user guide page? | | |
| 7 | Wire format changed? | | |
| 8 | Plugin SDK/protocol changed? | | |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | No RFC behavior |
| 10 | Test infrastructure changed? | | |
| 11 | Affects daemon comparison? | | |
| 12 | Internal architecture changed? | Yes | The page that documents `fleet-ui-5`'s alarm model once it has landed. `docs/architecture/fleet-interface.md`, the pages' side, is `fleet-ui-9`'s |
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
| A list of causes kept in one place | A second declaration of the kinds: a later feature would edit it to add a kind |
| Keep an alarm shown when its cause opens after it | Needs a record of what was shown before, the second state `fleet-ui-5`'s "Alarms are derived, not stored events" refuses |

### Documents departed from

none

### Choices for Thomas

Choice O-4, whether an alarm shown again after its cause clears raises a toast,
moved to `fleet-ui-9` as its C-1, because a toast is a page's.

| # | Choice | Options | Decision |
|---|--------|---------|----------|
| O-1 | Is Relay not granted caused by Device offline? | (a) recommended, No: it stays, because the grant is a fact the hub was told and the operator can prepare the fix while the device is away; (b) Yes: the informational row leaves the list while the device is offline, and returns with it | |
| O-2 | How long does a settling alarm wait after its cause clears? | (a) recommended, the kind's own configured time, counted again from the clear: no new setting, and the wait is as long as the operator already chose for that alarm; (b) a separate settle time per kind: one more setting, and a short alarm time can be combined with a long wait after recovery; (c) no wait: a figure that is still high in the first sample after recovery is shown at once | |
| O-3 | Does the cause match carry targets other than one device now? | (a) recommended, No, until a kind needs it: the match is by device only, and the first kind about an interface or a tunnel adds its target with its own design; (b) Now: `Alarm` gains an interface target and a list of devices, so a tunnel alarm is hidden when either end is offline, and the part contract of `fleet-1` changes for kinds that do not exist yet | |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- [pending design]
   - Tests: `test/managed/alarm-hidden-behind-offline.ci`
   - Verify: `./le test managed` runs the wiring test, red until the hiding is computed

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | Hiding is computed on the hub, so every command surface answers the same and the pages of `fleet-ui-9` read it rather than compute it |
| Rule: registration | No list of causes exists outside the kinds' own registrations |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| [pending design] | [pending design] |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | [pending design] |

The authorization row (a hidden alarm on a device the operator may not see
reaches neither a count nor a side panel) moved to `fleet-ui-9`.

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
| Causes are declared by the kind that is hidden | A list of causes kept in one place | A list of causes kept in one place is a second declaration of the kinds. Hiding is computed from the open alarms, so it is not a state either ("Alarm causes") |

## Known Limitations
- The cause match is by device only (decision O-3).

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| How the pages and toasts show a hidden alarm, with the ACs, tests, risk, security row and choice in "Moved to fleet-ui-9" | Thomas ruled on 2026-11-23 that the fleet web pages come after the first release (`ai/META-DECISIONS.md`, "Goal and scope") | `plan/post-release/features/spec-fleet-ui-9-tasks-notifications-and-alarm-pages.md` |

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
