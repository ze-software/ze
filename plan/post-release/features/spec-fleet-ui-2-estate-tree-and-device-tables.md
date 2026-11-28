# Spec: fleet-ui-2 -- Estate Tree, Device Tables, Filters and Bulk Actions

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | web, fleet |
| Priority | P3 |
| Release reason | Adds a capability absent from the supported release; existing supported behavior and its obligations remain required. |
| RFC gaps | - |
| Depends | spec-fleet-1-device-registry (closed 2026-11-25; documented in `docs/architecture/fleet-config.md`, "Device registry"), spec-fleet-4-inventory-health (closed 2026-11-26; documented in `docs/architecture/fleet-config.md`, "Inventory and health reporting"), spec-fleet-8-metric-history (closed 2026-11-27; documented in `docs/architecture/fleet-config.md`, "Device metric sampling and hub-side history") |
| Phase | 2/9 of plan/post-release/features/spec-fleet-ui-0-umbrella.md |
| Handoff | - |
| Updated | 2026-11-25 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

**Re-read after compaction:** this file, `spec-fleet-ui-0-umbrella.md`,
`docs/architecture/fleet-config.md` (`spec-fleet-1-device-registry`, closed 2026-11-25).

## Task

Make finding a device work on the real estate. This spec turns the tree and the
devices table of the prototype into functional views over the device registry
of `fleet-1`, and specifies how devices are grouped, filtered, selected and
acted on together.

An operator must be able to answer, in a few seconds and without a command:
which devices exist, where, in what state, running what, and which of them match
a condition.

### The estate tree

The tree is computed, not stored. A tree view is an ordered list of label keys;
each key is one level.

| Rule | Detail |
|------|--------|
| Root | One node, the estate, named after the hub |
| Levels | One level per label key of the tree view in use. The default view is `site`, then `role` |
| Leaves | Devices, in name order |
| Missing label | A device lacking the label of a level sits under a node named `(no <key>)`, listed last |
| Group as a level | The `group` of `fleet-1` (the unit that assigns a configuration template) may be used as a level key like any label |
| Views | Named views are declared in the hub's configuration. The operator picks one from a menu above the tree; the choice is in the URL (`view`) and remembered per browser |
| Built-in views | `site, role`; `role`; `software version`; `state`. The last two group on registry fields, not labels |
| Counts | A group row shows connected devices over total, for example `(11/12)`. The count turns to the warning colour when any device below is not healthy |
| Device row | State icon, name, and a badge with the number of open alarms when there is one |
| Row menu | The same Actions menu as the object header of that node |
| Pending work | A spinner replaces the state icon while a task targets the node, with the task's name as its tooltip |
| Filter box | Narrows the tree to devices whose name or any label value contains the text, keeping their ancestors; runs on the server; the keyboard shortcut `/` focuses it |
| Loading | Branches below the second level load when first expanded, with skeleton rows meanwhile |
| Selection | Selecting a node opens its page. After a reload the tree expands to the selected node and scrolls it into view |
| Keyboard | Up and down move, right expands, left collapses, enter opens |

### The devices table

Shown on the Devices tab of the estate and of every group node, scoped to that
node's devices. The stack is the same wherever a table appears: query bar, bulk
bar when rows are selected, table, pagination.

```
+---------------------------------------------------------------------------------+
| [ state:degraded,down site:lon                    ] [State v][Label v][Version v]|
| 4 of 44 devices                                      [Saved filters v] [Save]    |
+---------------------------------------------------------------------------------+
| 2 selected    [Add label] [Remove label] [Run diagnostic v] [Push config] [More v]|
+---+--------------+----------+-----------------+---------+---------+------+-------+
|[ ]| DEVICE       | STATE    | LABELS          | VERSION | CONFIG  | CPU  |  ...  |
+---+--------------+----------+-----------------+---------+---------+------+-------+
|[x]| edge-lon-01  | Degraded | site=lon role=..| 1.4.2   | In sync | 71 % | (o) : |
|[x]| edge-lon-04  | Down     | site=lon role=..| 1.4.2   | Pending | 12 % | (o) : |
+---+--------------+----------+-----------------+---------+---------+------+-------+
| |<  <   1 to 4 of 4   >  >|                                    Show [50 v]       |
+---------------------------------------------------------------------------------+
```

| Column | Content | Sortable |
|--------|---------|----------|
| Selection | A checkbox; the header checkbox selects the page | No |
| Device | State icon and name, linking to the device page | Yes, default ascending |
| State | The state word | Yes, by severity |
| Labels | Tags, at most two shown, the rest behind a count whose tooltip lists them. The model label is not shown here: it is inventory, shown in the side panel. Owner decision, 2026-11-19: two rather than three, so every column stays visible at normal widths | No |
| Version | Software version, in the monospace font | Yes |
| Config | In sync, Pending, Rejected, Diverged or Never fetched | Yes |
| Uptime | Since the device's daemon started | Yes |
| Last seen | "now" when connected, otherwise the time | Yes |
| CPU, Memory | Latest reported percentage, with a thin usage bar | Yes |
| Alarms | Count of open alarms, linking to the device's alarms | Yes |
| Row actions | An eye control opening the side panel, and the Actions menu | No |

The operator chooses which columns show and in what order; the choice is
remembered per browser. Device, State and the row actions cannot be hidden.

### Filter language

One text box, one small grammar, the same in every fleet table. The filter is
parsed on the server and carried in the URL as `q`.

| Form | Meaning | Example |
|------|---------|---------|
| A bare word | Devices whose name or any label value contains it, ignoring case | `lon` |
| `field:value` | The field equals the value | `state:offline` |
| `field:a,b` | The field equals any of the values | `state:degraded,down` |
| `field:!value` | The field does not equal the value | `version:!1.4.2` |
| `label:key=value` | The device carries that label | `label:customer=acme` |
| `key:value` where `key` is a label key in use | Shorthand for the line above | `site:lon` |
| `field:>n`, `field:<n` | Numeric comparison | `cpu:>80` |
| `has:alarm`, `has:task` | The device has an open alarm or a running task | `has:alarm` |
| Several terms | All must hold | `site:lon state:!healthy` |

Fields: `name`, `state`, `version`, `config`, `group`, `cpu`, `memory`,
`uptime`, `last-seen`, `model`. An unknown field or a malformed term is refused
with a message naming the term and listing the fields, and the table keeps its
previous rows.

The assisted menus beside the box (State, Label, Version) write terms into it,
so the box always shows the whole filter. A filter can be saved under a name,
per user; saved filters are listed in a menu and on the administration page.
Each user may mark one saved filter as the default for the estate devices tab.

### Selection and bulk actions

| Rule | Detail |
|------|--------|
| Selecting | Click a checkbox; shift and click selects the range; the header checkbox selects the page; a link in the bulk bar extends the selection to every row matching the filter |
| Bulk bar | Appears when at least one row is selected, naming the count |
| Add label, Remove label | Stage the label edits in the operator's candidate configuration on the hub and show the pending change bar; nothing changes until commit |
| Run diagnostic | A menu of registered read-only tools; running one creates a task with one result per device (`fleet-ui-9`, `fleet-9`) |
| Push config | Opens the rollout form with the selection as its target (`fleet-ui-6`) |
| More | Actions valid for the selection only; an action not valid for every selected device is listed disabled with the reason |
| Destructive actions | Listed last, apart from the routine ones, and confirmed by typing the count of devices |

### The side panel

Opened by the eye control; the row is named in the URL as `id`.

| Section | Rows |
|---------|------|
| Identity | Name, state, model, serial number, software version, management address, each with a copy control |
| Labels | Tags, with add and remove for an operator allowed to edit |
| Connection | Connected since or last seen, source address, last configuration fetch and its result |
| Health | One row per reported component with its state word |
| Now | CPU, memory and storage usage bars; uptime |
| Open alarms | The three most severe, with a link to all |
| Actions | Open device, Open console, Run diagnostic, the Actions menu |

### Key design decisions

| Decision | Detail |
|----------|--------|
| Tree views are configuration | A view is declared in the hub's configuration, so it is explicit, versioned and the same for every operator. The built-in views exist without configuration |
| Filtering, sorting and paging happen on the server | The page never holds more rows than it shows |
| One grammar for every table | The alarms, tasks and audit tables use the same forms with their own fields |
| Label edits are configuration edits | Labels live in the hub's configuration (`fleet-1`), so a bulk label edit goes through the editor and commit like any other change, and is reviewable before it applies |
| Saved filters are per user | Stored in the hub's state store under the user's name; they are a personal convenience, not configuration |

### Carried from the prototype

The prototype (`spec-fleet-ui-1-interactive-prototype`, closed 2026-11-19,
approved by the owner as built) left these to this spec. The package, the
seam and the sample build are described in `docs/architecture/fleet-interface.md`.

| Item | What the prototype does | What this spec owes |
|------|-------------------------|---------------------|
| Fleet as a web mode | Fleet is reached at `/fleet/` and through its portal entry only | Make Fleet a selectable web mode in the existing shells' mode switch, with their markup and fixtures |
| The real enabling condition | The fleet routes mount only when a data source is registered, and the only source is the sample one, which no shipped build links (`sourceRegistered` in `internal/component/web/fleetui/register.go`) | Register the real source, built over the command dispatcher, and mount the routes when the daemon serves managed devices. Amended 2026-11-19 (owner decision at `fleet-1`'s gates): `spec-fleet-1-device-registry` splits the `Source` contract into one part per back end and registers the estate part (`Now`, `Estate`, `Devices`, `Device`) over the dispatcher, and the routes mount while the estate part is registered. This spec still owes the views part (`TreeViews`, `SavedFilters`), the filter grammar, sort and page over the estate rows, the web mode, sign-out and the CI stage below; `fleet-1`'s section "The fleet interface parts after the split" names the owner of every other part. Amended 2026-11-23 (owner ruling, "1 b": the first release reaches the fleet features through commands, their pages come after it): the split of the `Source` contract, the estate part and the live-estate browser test moved from `fleet-1` to this spec, with that section, under "Moved from fleet-1 (owner ruling 2026-11-23)" below |
| Sign out | The account menu holds the theme switch and the link to the other web modes. The web server has no sign-out route in any mode | Add the sign-out entry the umbrella's header row names, over a sign-out route of the web package's session handling |
| Side panel and a later sort | Opening a row asks for the side panel fragment alone, so the table's sort and page links keep the address they were drawn with, and following one closes the panel (`PanelLink` paragraph of `docs/architecture/fleet-interface.md`). Approved as built | Decide with the owner whether a sort keeps the open row, when the table becomes functional |
| Fleet browser tests in CI | The thirteen `test/web/fleet-*.wb` cases declare `option=needs-build-tag:value=ze_fleet_ui_sample:state=present`, and no CI run builds `ze` with that tag, so every CI run skips them; they pass only when run locally (`runWebTests`, `internal/test/cli/cmd_web.go`) | The owner decided on 2026-11-19 that CI runs them. Add a verifier stage (the native verifier owns CI's stage population, `internal/le/verify/engine/stages.go`) that builds `ze` with `ze_fleet_ui_sample` and runs the fleet browser suite on a change that touches the fleet interface, with failure text naming the case and the rerun command |

### Data this spec reads

| Data | Command it expects from the `fleet` set | State |
|------|------------------------------------------|-------|
| Device list with name, state, labels, group, version, configuration state, last seen | `show fleet devices` and `show fleet device name <name>` (`spec-fleet-1-device-registry`, first release), answering JSON; filter, sort and page are this spec's, over the estate part's rows | `fleet-1` is ready; its approved design has no filter, sort or page arguments, so the grammar above runs in this spec over the estate rows |
| Latest CPU, memory, storage, uptime per device | `fleet-8`'s `show fleet sample latest` query (`spec-fleet-8-metric-history`, closed 2026-11-27; `docs/architecture/fleet-config.md`, "Device metric sampling and hub-side history"), read by the latest-sample part this spec registers | Implemented (`spec-fleet-8-metric-history`, closed 2026-11-27) |
| Inventory and health per device | `fleet-4`'s `show fleet device inventory name <name>`, `show fleet device health name <name>` and `show fleet health` (`spec-fleet-4-inventory-health`, closed 2026-11-26; `docs/architecture/fleet-config.md`, "Inventory and health reporting"), read by the inventory-health part this spec registers | Implemented (`spec-fleet-4-inventory-health`, closed 2026-11-26) |
| Label keys and values in use | `show fleet labels` | Missing |
| Open alarm and running task counts per device | `show fleet alarm` (`spec-fleet-ui-5-alarm-list`, closed 2026-11-27; `docs/architecture/fleet-alarms.md`), `show fleet task` (`plan/post-release/features/spec-fleet-ui-9-tasks-notifications-and-alarm-pages.md`) | Alarms implemented (`spec-fleet-ui-5-alarm-list`, closed 2026-11-27); tasks missing |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A hub with labelled devices | The tree groups them by the levels of the view in use, with correct counts | Devices lacking a label appear under `(no <key>)`, because hiding them would hide devices |
| AC-2 | The operator changes the tree view | The tree regroups, the URL carries the view, and the selected device stays selected | The expansion state of the previous view is dropped, because its nodes no longer exist |
| AC-3 | A filter in the query bar | The table shows exactly the matching devices, the count names them, and the URL reproduces the view | Selection is cleared when the filter changes, because selected rows may no longer be visible |
| AC-4 | A malformed filter | The table keeps its rows and a message names the bad term and the valid fields | None |
| AC-5 | A selection and a bulk label edit | The edits are staged, shown as pending, and applied on commit, after which the tree regroups | A device removed from the estate meanwhile is reported and skipped, because the rest of the edit is still wanted |
| AC-6 | An estate of 2000 devices | For the full 2,000-device fixture, each tree, table and filtered-table handler responds within a proposed 1-second p95 over 100 warm requests on an idle, identified release-test host. Record response bytes, page size, the slowest request and the host alongside the result. The returned table fragment contains only the requested page | None |
| AC-7 | A row's eye control | The side panel opens with the sections above, the URL carries the row, and a reload restores it | None |

### Moved from fleet-1 (owner ruling 2026-11-23)

Thomas ruled (voice-transcribed answer "1 b") that the fleet features of the
first release are reached through commands on every command surface, and that
their web pages and views come after the first release (`ai/META-DECISIONS.md`,
"Goal and scope"). The fleet interface work that
`spec-fleet-1-device-registry` carried moved here
whole: the split of the `Source` contract into one part per back end, the
estate part over the dispatcher, the join by device name, the not-available
state, the sample build registering every part and the live-estate browser
test. `fleet-1` keeps the registry, its commands on every command surface, the
reload apply and the connection bound; its section "Owner ruling of
2026-11-23" lists each criterion that moved and names its AC here.

The owner decided this design at `fleet-1`'s gates of 2026-11-19 ("Data seam",
"Device join", "strengthen the split", "removed devices are listed by the CLI
only"). Those decisions stand here as decided; this spec stays a skeleton
until its own design is presented.

| Owner decision carried (from `fleet-1`'s gates, 2026-11-19) | Decision |
|------------------------------------------------------------|----------|
| Data seam | The `Source` contract is split so each back end registers the part it owns. A part no back end answers shows a plain "not available on this hub yet" state |
| Device join | The interface joins per-part facts by device name. Each back end answers only its own fields. A part no back end answers leaves its fields "not reported": a connected device's health is Unknown |
| The split is specified method by method | Every later back end implements a stated contract, and the not-available state of every part is built and proven with the split |
| One part per owning spec | `ConfigSource` splits into device config, config history, divergence, rollout, template, access and settings parts; `DeviceSystem` is joined from `DeviceViewPart` and `InventoryHealthPart` |
| Removed devices | Listed by the CLI only; the interface's estate part lists declared devices only. `fleetui` has no removed state; adding one is this spec's design |

#### The fleet interface parts after the split

The single `Source` of `internal/component/web/fleetui/source.go` becomes one
interface per back end, each held by its own slot in `source_parts.go` and
registered from `init()` by one call per part (`MustRegisterEstatePart`,
`MustRegisterViewsPart`, and so on, one name per row below). A nil part or a
second registration of one part panics, as `MustRegisterSource` does today.
`MustRegisterSource` is deleted, not kept beside the parts (no layering).
Method names, argument lists, answer types and bounds are the ones
`source_*.go` declares today (read on 2026-11-19); the split moves them, it
does not change them, so every page handler keeps its calls. The sample
(`internal/component/web/fleetui/sample/source.go`, one `estate` value that
answers every method today) registers itself as every part from
`sample/register/register.go`.

The Owner column names the spec that registers the part on a real build. A
part over a first-release back end is web code, so after the owner ruling of
2026-11-23 it is registered by the post-release fleet-ui spec whose page first
reads it, over the back end's commands through the dispatcher; the back end's
spec keeps the commands.

| Part (interface) | Method: inputs → answer, bound | Owner |
|------------------|-------------------------------|-------|
| `EstatePart` | `Now(ctx)` → `time.Time`; `Estate(ctx)` → `Estate`; `Devices(ctx)` → `[]Device`, at most `DevicesMax` (4,096), each with `Labels` at most `LabelsMax` (32); `Device(ctx, name)` → `Device`, an unknown name is `DeviceUnknownError`. The estate part fills only `Name`, `Labels`, `SecretSet`, `Connection.Connected`, `Since`, `EverConnected`, `LastSeen`, `SourceAddr`, and `Config.State`, `HubVersion`, `DeviceVersion`, `Fetched`, `LastFetch`, `LastFetchResult` | This spec, over `spec-fleet-1-device-registry`'s `show fleet devices` and `show fleet device name <name>` |
| `ViewsPart` | `TreeViews(ctx)` → `[]TreeView`, at most `TreeViewsMax` (32), each at most `TreeLevelsMax` (8) levels; `SavedFilters(ctx, user)` → `[]SavedFilter`, at most `SavedFiltersMax` (64) | This spec |
| `InventoryHealthPart` (device facts) | `InventoryHealth(ctx)` → one entry per device name holding `Inventory`, `HealthReported`, `Health` (at most `HealthComponentsMax`, 32) and the hardware list of `DeviceSystem.Hardware` (at most `HardwareItemsMax`, 64), at most `DevicesMax` entries; `InventoryHealthOf(ctx, name)` → the same entry for one device, an absent device answering the entry with `Reported` and `HealthReported` false | This spec, over `spec-fleet-4-inventory-health`'s commands (was `fleet-4`'s before the ruling) |
| `LatestSamplePart` (device facts) | `LatestSamples(ctx)` → one `LatestSample` per device name, at most `DevicesMax`; `LatestSampleOf(ctx, name)` → `LatestSample` | This spec, over `spec-fleet-8-metric-history`'s latest-samples query (was `fleet-8`'s before the ruling) |
| `RelayGrantPart` (device facts) | `RelayGrants(ctx)` → per device name `RelayGranted` and `RelayProfile`, at most `DevicesMax`; `RelayGrantOf(ctx, name)` → the same for one | `plan/post-release/features/spec-fleet-9-device-command-relay.md` |
| `AssignmentPart` (device facts) | `Assignments(ctx)` → per device name `Group` and `Config.Template`, at most `DevicesMax`; `AssignmentOf(ctx, name)` → the same for one | `plan/post-release/features/spec-fleet-2-config-templates.md` |
| `HistoryPart` | `DeviceSeries(ctx, device, metric Metric, span TimeSpan)` → `Series`, at most `SeriesPointsMax` (20,160) points; `EstateSeries(ctx, devices []string, metric, span)` → `Series`, at most `SeriesPointsMax`, `devices` at most `DevicesMax`; `HistoryUsage(ctx)` → `HistoryUsage`, `Largest` at most `HistoryDevicesMax` (10) | `plan/post-release/features/spec-fleet-ui-3-dashboards-and-charts.md`, over `fleet-8`'s series and store status queries (was `fleet-8`'s before the ruling) |
| `ActivityPart` | `Alarms(ctx)` → `[]Alarm`, at most `AlarmsMax` (1,024); `AlarmHistory(ctx, device)` → `[]AlarmEvent`, at most `AlarmHistoryMax` (512), newest first; `Tasks(ctx)` → `[]Task`, at most `TasksMax` (512), newest first; `Task(ctx, id)` → `TaskDetail`, each answer at most `TaskAnswerBytesMax` (16,384), an unknown id wraps `ErrObjectUnknown` | `plan/post-release/features/spec-fleet-ui-9-tasks-notifications-and-alarm-pages.md` (split from `fleet-ui-5` on 2026-11-23, owner ruling "2 b"): `Alarms` and `AlarmHistory` answered from `spec-fleet-ui-5-alarm-list`'s commands (`docs/architecture/fleet-alarms.md`), `Tasks` and `Task` from `fleet-ui-9`'s task model |
| `DeviceConfigPart` | `DeviceConfig(ctx, device)` → `DeviceConfig`, `Text` at most `ConfigTextBytesMax` (1 MiB) | `plan/post-release/features/spec-fleet-2-config-templates.md` (the source comment names `fleet-1` and `fleet-2`; the owner gave `fleet-1` the estate part only, so `fleet-2`, which renders what the hub serves, owns it) |
| `ConfigHistoryPart` | `ConfigVersions(ctx, device)` → `[]ConfigVersion`, at most `ConfigVersionsMax` (100), newest first; `ConfigText(ctx, device, version)` → text, at most `ConfigTextBytesMax`, an unknown version wraps `ErrObjectUnknown`; `AuditEntries(ctx)` → `[]AuditEntry`, at most `AuditEntriesMax` (1,000), newest first | `spec-fleet-3-audit-trail` (closed 2026-11-28; documented in `docs/architecture/fleet-config.md` and `docs/guide/audit.md`) |
| `DivergencePart` | `Divergence(ctx, device)` → `Divergence`, texts at most `ConfigTextBytesMax`; a device that is not diverged wraps `ErrObjectUnknown` | `plan/post-release/features/spec-fleet-ui-6-config-rollout-audit-admin.md`, over `show fleet divergence name <name>`. Was `spec-fleet-7-config-reconnect-resolution`, closed on 2026-11-19 with no part registered, so the page family that shows the divergence (`fleet-ui-6`) owns it |
| `RolloutPart` | `Rollouts(ctx)` → `[]Rollout`, at most `RolloutsMax` (128), newest first; `Rollout(ctx, name)` → `RolloutDetail`, at most `RolloutBatchesMax` (100) batches, an unknown name wraps `ErrObjectUnknown` | `plan/post-release/features/spec-fleet-5-staged-rollout.md` |
| `TemplatePart` | `Templates(ctx)` → `[]Template`, at most `TemplatesMax` (256) | `plan/post-release/features/spec-fleet-2-config-templates.md` |
| `AccessPart` | `AccessProfiles(ctx)` → `[]AccessProfile`, at most `AccessProfilesMax` (64) | `plan/post-release/features/spec-fleet-ui-6-config-rollout-audit-admin.md` (the source comment names `fleet-1`; the access administration page is `fleet-ui-6`'s) |
| `SettingsPart` | `Settings(ctx)` → `Settings` | `plan/post-release/features/spec-fleet-ui-9-tasks-notifications-and-alarm-pages.md`: the alarm settings are `fleet-ui-5`'s configuration, the finished task count and the stream maximum `fleet-ui-9`'s |
| `DeviceViewPart` | `DeviceSystem(ctx, device)` → `DeviceSystem` (`Resources`, `Time`; `Hardware` comes from `InventoryHealthPart` by the join); `DeviceInterfaces(ctx, device)` → `[]Interface`, at most `InterfacesMax` (1,024); `DevicePeers(ctx, device)` → `[]Peer`, at most `PeersMax` (4,096); `DeviceFamilies(ctx, device)` → `[]FamilySummary`, at most `FamiliesMax` (64); `DeviceAdjacencies(ctx, device)` → `[]ProtocolAdjacencies`, at most `ProtocolsMax` (16) of at most `AdjacenciesMax` (1,024) rows; `DeviceLogs(ctx, device)` → `[]LogLine`, at most `LogLinesMax` (500), newest first; `Console(ctx, device, user, command)` → `ConsoleAnswer`, at most `ConsoleOutputBytesMax` (65,536) | `plan/post-release/features/spec-fleet-9-device-command-relay.md` |
| `EventPart` | `Subscribe(ctx)` → `*Subscription`, at most `EventSubscriptionsMax` (64) at once, each holding at most `EventBufferSize` (64) unread events; beyond the maximum wraps `ErrStreamsFull` | `plan/post-release/features/spec-fleet-ui-9-tasks-notifications-and-alarm-pages.md` (live page updates, split from `fleet-ui-5` on 2026-11-23) |

**The join by device name.** `fleetui` answers a page's `Devices` and
`Device` by asking the estate part first: its answer is the set of devices
and their order, and a device the estate does not hold is not a device of the
page. For each registered device-fact part, it asks the batch method once per
request (or the `...Of` method for one device) and copies into each device
exactly the fields the table gives that part, matched by `Name`. A name a part
answers that the estate does not hold is ignored, because the estate is the
set; a device a part does not answer keeps that part's fields at their zero
value, which every fact struct already reads as "not reported"
(`Inventory.Reported`, `HealthReported`, `LatestSample.Reported` false,
`RelayGranted` false, `Group` and `Template` empty). No field is written by
two parts. A part that is not registered leaves its fields not reported for
every device; a registered part that answers an error leaves them not
reported and the page shows that part's error text in place of its fields
(an operator error, `FailureOf`), never failing the page. `DeviceDisplayState`
is unchanged, so a connected device with no health part is Unknown.

**The not-available state.** While no back end has registered a part, every
place that shows that part's answer renders one component instead: the text
"Not available on this hub yet." in `--text-secondary`, inside an element
whose id is `fleet-not-available-<part>` (`<part>` is the part's kebab-case
name: `views`, `inventory-health`, `latest-sample`, `relay-grant`,
`assignment`, `history`, `activity`, `device-config`, `config-history`,
`divergence`, `rollout`, `template`, `access`, `settings`, `device-view`,
`events`). It is a state of the page, not an error: the HTTP status stays 200,
and `ReadSnapshot` records which parts are absent instead of failing. Where
each part shows:

| Part | Pages, cards and sections that show its not-available state |
|------|--------------------------------------------------------------|
| `ViewsPart` | The tree view menu lists the built-in views only, with the declared section showing the state; the saved filter menu of every table; the tree views and filters administration pages |
| `InventoryHealthPart` | The Software card of the estate and group dashboards; the Version column cell of the devices table shows a dash; the Identity and Health sections of the side panel; the hardware section of the System tab |
| `LatestSamplePart` | The Busiest devices card; the CPU and Memory cells of the devices table show a dash; the Now section of the side panel |
| `RelayGrantPart` | The relay grant row of the side panel's Connection section |
| `AssignmentPart` | The group level of a tree view and the template row of the Config tab show a dash |
| `HistoryPart` | Every chart of the estate, group and device dashboards; the history administration page |
| `ActivityPart` | The Alarms card; the alarm count of the Needs attention card; the alarm badges of the tree; the Alarms tabs; the task panel, the Tasks tabs and the Tasks page; the header task count; the Open alarms section of the side panel |
| `DeviceConfigPart` | The device Config tab |
| `ConfigHistoryPart` | The history section of the Config tab; the Recent changes card; the audit administration page and the audit section of the device Tasks tab |
| `DivergencePart` | The divergence banner and diff of the device Dashboard and Config tabs (the diverged state itself still comes from the estate part) |
| `RolloutPart` | The Rollouts card; the estate Rollouts tab and the rollout page; the rollout count of the Needs attention card |
| `TemplatePart` | The templates administration page |
| `AccessPart` | The access administration page |
| `SettingsPart` | The settings administration page |
| `DeviceViewPart` | The System (resources and clock), Interfaces, Routing, Logs and Console tabs of a device |
| `EventPart` | Not shown as text: the page works without live updates, as it does today when the stream answers 503 |

The estate part is the one part without a not-available state: the fleet
routes mount only while it is registered (`sourceRegistered` reads its slot),
and the two prototype-only routes mount only while the event part is a
`Stepper`.

**The estate part on a real build.** `internal/component/web/fleetui/estate`
registers it from `init()` with a builder over `web.RouteDeps.Dispatch`,
built once when the first fleet route is built. `Devices` dispatches
`show fleet devices`, `Device` dispatches `show fleet device name <name>`, as
the request's account; a refusal or an error is an operator error wrapping
the `source_errors.go` error that fits. `Now` is the hub process's clock and
`Estate.Name` the hub's host name (A-1); a host name that cannot be read is an
operator error, never an empty name. `cmd/ze/hub/register_fleet_ui_estate.go`
links it under `ze_web && ze_fleet_ui && !ze_fleet_ui_sample`.

**The live-estate browser test.** The browser test runner gains a server kind,
`fleet-hub` (`WBServerFleetHub` in `internal/component/web/testing/parser.go`,
started by a new `zeTestStartFleetHubServer` in `internal/test/cli/cmd_web.go`,
the way `zeTestStartLGServer` starts a peer beside the daemon). It starts the
hub daemon with web enabled and a serving block declaring one client,
`edge-01`, with a fixed secret, the label `site lon`, and a fixed client
config written to `file/active/client-edge-01.conf`; then it starts a second
`ze` as the managed client `edge-01` against that block and waits until the
hub answers `show fleet device name edge-01` with `connected` true and
`ack-result` `applied`. `test/web/fleet-estate-live.wb` carries these option
lines:

| Option line | Why |
|-------------|-----|
| `option=needs-build-tag:value=ze_fleet_ui:state=present` | The fleet routes are linked |
| `option=needs-build-tag:value=ze_fleet_ui_sample:state=absent` | A real build: the sample would register every part |
| `option=server:kind=fleet-hub` | The hub with one real managed client |
| `option=timeout:value=90s` | Two daemons and a TLS handshake |
| `option=viewport:width=1600:height=1000` | The devices table shows every column |

It opens `/fleet/` and then the estate Devices tab, and expects: a row named
`edge-01`; its state cell "Unknown" (connected, no health part registered);
its Config cell "In sync"; its Labels cell `site=lon`; the side panel's
Connection section naming the client config's version hash, the 16 hex
characters `fleet.VersionHash` gives for the fixed config (computed when the
test is written and stated in its header with the bytes it hashes); on the
estate dashboard, the element `fleet-not-available-activity` inside the Alarms
card with the text "Not available on this hub yet."; and no element with the
sample marker "Sample data". The fleet browser tests run in CI once this
spec's verifier stage ("Carried from the prototype") builds the fleet tags;
until then the main thread runs this one locally as an owed command.

#### Criteria moved in

| AC ID | From | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|------|-------------------|-------------------|-----------------------------|
| AC-8 | `fleet-1` AC-14 | A fleet page on a real `ze_fleet_ui` build with the estate part registered and no other part | The dashboard and devices table show the declared devices from the registry; alarms, tasks, history, device tabs and declared tree views show "not available on this hub yet"; built-in tree views work | The live event stream is absent (the page works without live updates), because the events part is not registered |
| AC-9 | `fleet-1` AC-15 | A connected device with no health part registered | Its display state is Unknown, never Healthy | None |
| AC-10 | `fleet-1` AC-28 | `fleetui.DevicesMax` and the registry's `ManagedClientsMax` | Equal, checked by a test | None |
| AC-11 | `fleet-1` AC-29 | `test/web/fleet-estate-live.wb` on a build with `ze_fleet_ui` and without `ze_fleet_ui_sample`, one real managed client `edge-01` connected | The estate devices table shows `edge-01` connected (state Unknown), Config "In sync", label `site=lon`, and its config version hash in the side panel; the estate dashboard's Alarms card shows `fleet-not-available-activity` with "Not available on this hub yet." | No sample marker anywhere, because the sample registers nothing on this build |
| AC-12 | `fleet-1` AC-30 | Each part of the split table unregistered, with the estate part registered | Every page, card and section the not-available table names for that part renders "Not available on this hub yet." in `fleet-not-available-<part>`, with status 200, and every other part's content still renders | The event part shows no text, because a page without live updates is its stated absent state |
| AC-13 | `fleet-1` AC-31 | A device-fact part registered that answers facts for some estate devices and one name the estate does not hold | Each estate device carries exactly that part's fields for its name; devices it did not answer keep them not reported; the extra name appears nowhere; no field of another part changes | None |
| AC-14 | `fleet-1` AC-32 | A second registration of any one part, or a nil part | Panics at process start naming the part | None: it is a build composition defect |
| AC-15 | `fleet-1` AC-33 | The sample build (`ze_fleet_ui_sample`) | Every existing `test/web/fleet-*.wb` passes unchanged, and no not-available element appears | None: the sample registers every part |
| AC-16 | `fleet-4` AC-12 (its devices table half) | Devices whose inventory and health `fleet-4` reports, on a real build with the inventory-health part registered | The devices table's Version column shows each device's reported software version; the side panel's Identity section shows model, serial number and version, and its Health section one row per reported component with its state word; a device that has not reported shows not reported, never a blank or a zero | None |
| AC-17 | Part registration moved from `fleet-4` and `fleet-8` (`fleet-1`'s part table) | A real build with the inventory-health and latest-sample parts registered over the dispatcher | Each part answers, per estate device, exactly what `fleet-4`'s inventory and health commands and `fleet-8`'s latest-samples query answer for that device; a refused dispatch is an operator error, never an empty answer | None |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/api-surfaces.md`, `ai/doors/hub-engine.md` - the web UI is a thin adapter over the dispatcher; `cmd/ze/hub` reaches a gated surface only through a gated file
  → Constraint: `cmd/ze/hub` is always on; the fleet interface is gated `ze_web && ze_fleet_ui`, so the real estate part is linked from a hub file under that build constraint, never imported by always-on code.
  → Constraint: a web surface never imports a protocol package to read state; it asks the dispatcher. The estate part reads `show fleet devices` through `web.RouteDeps.Dispatch`.
- [ ] `docs/architecture/fleet-interface.md` - the data source seam, "A source answers facts, the package derives", the failures a source reports
  → Constraint: a source MUST answer an operator error wrapping a `source_errors.go` error where one fits, never an empty answer in its place (`internal/component/web/fleetui/source.go` `Source` doc).
  → Decision: the split keeps derivation in `fleetui` (`DeviceDisplayState`, the tree, the table): a part answers facts only.

## Current Behavior (MANDATORY)

**Source files read (moved from `fleet-1`, read on 2026-11-19; re-read on 2026-11-23):**
- [ ] `internal/component/web/fleetui/source.go` - `Source` is the union of six interfaces (`RegistrySource`, `HistorySource`, `ActivitySource`, `ConfigSource`, `DeviceViewSource`, `EventSource`); `sourceSlot` holds one; `MustRegisterSource` panics on a nil source and on a second.
- [ ] `internal/component/web/fleetui/source_devices.go` - `RegistrySource` (`Now`, `Estate`, `Devices`, `Device`, `TreeViews`, `SavedFilters`), `Device` and its fact structs, `DevicesMax` = 4096, `LabelsMax` = 32.
- [ ] `internal/component/web/fleetui/state.go` - `DeviceDisplayState`: offline when not connected, then diverged, down, degraded, and Unknown while `HealthReported` is false. `ConfigState`: in-sync, pending, rejected, diverged, never-fetched.
- [ ] `internal/component/web/fleetui/pages.go` - `ReadSnapshot` fails the whole page when `Alarms`, `Tasks` or `TreeViews` errors.
- [ ] `internal/component/web/fleetui/register.go` - every route mounts while `sourceRegistered`; a route's builder receives `web.RouteDeps`, which carries `Dispatch`.
- [ ] `internal/component/web/fleetui/sample/register/register.go` - the sample registers one source for every concern (`MustRegisterSource(newSource(time.Now))`).
- [ ] `cmd/ze/hub/register_fleet_ui.go` (`ze_web && ze_fleet_ui`) and `register_fleet_ui_sample.go` (`... && ze_fleet_ui_sample`) - the hub-side gates.
- [ ] `internal/component/web/testing/parser.go` - the server kinds `WBServerWeb`, `WBServerLG`, `WBServerChaos`, `WBServerLGNoEngine`, `WBServerDaemon`.
- [ ] `internal/test/cli/cmd_web.go` - `zeTestStartLGServer` starts a sink peer, then the daemon beside it.

## Existing Mechanisms

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Fleet interface seam | `internal/component/web/fleetui/source.go` `sourceSlot`, `MustRegisterSource`; `register.go` `sourceRegistered` | One source per build, all concerns | Replaces the single slot with one slot per part; the sample registers every part |
| Device display state | `internal/component/web/fleetui/state.go` `DeviceDisplayState` | Derives Unknown while `HealthReported` is false | Extends unchanged: an absent health part leaves `HealthReported` false, which is the owner's "health Unknown" |
| Route deps | `internal/component/web/webroute.go` `RouteDeps.Dispatch` | Gives a route builder the web dispatcher | Extends: the estate, inventory-health and latest-sample parts are built over it |
| Device registry commands | `spec-fleet-1-device-registry`, `show fleet devices`, `show fleet device name <name>` | First release: answered by the daemon on every command surface | Extends: the estate part reads them; no second reader of the registry |
| Browser test server kinds | `internal/component/web/testing/parser.go` `WBServer`; `internal/test/cli/cmd_web.go` `zeTestStartLGServer` | Starts the process a browser test drives, with a peer beside it for the looking glass | Extends: a `fleet-hub` kind with a managed client beside the hub |
| Registered tree, table, filter and actions | fleetui/handler_tree.go, handler_devices.go, handler_devices_action.go and table.go | Render and filter the sample estate and dispatch selected actions. | Keep the last valid filter in the per-view URL or fragment request. On malformed input rerun the last valid query on current authorized rows, with a visible error. A fresh direct URL with no prior filter uses the unfiltered authorized set. |
| Candidate and account | web.EditorManager (editor.go), SessionStore (auth.go), ReadUIModeFromRequest (ui_mode.go) | Authenticate the request, select the shell and isolate a user's candidate for hub configuration. | Reuse these mechanisms for bulk labels. Enforce mutation and target permission both on stage and commit. Ordinary navigation neither grants a role nor modifies the candidate. |
| Preferences and CI stage | internal/core/statestore, pkg/zefs/keys.go; internal/le/verify/engine/stages.go | Register permitted persistent keys and select the test build per verification stage. | Put saved preferences under the owning account's key and give the live fixture its own real fleet build. Keep the existing prototype stage separate. |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|---|---|---|---|---|---|
| `ai/doors/system.md` | One schema, one command registry, every interface | the door every change reads | "A command is declared once and reaches every interface." | agrees | The web and command presentations read facts through the registered operation under the caller’s identity. |
| `docs/architecture/fleet-interface.md` | A source answers facts, the package derives | the // Design: page of internal/component/web/fleetui/source.go | "A source answers facts only" | agrees | Use caller-scoped source facts and the shared derivations to keep rows, totals and linked detail views consistent. |
| `ai/doors/api-surfaces.md` | Boundaries | the door covering internal/component/web/fleetui/source.go | "Never add a command in an adapter." | agrees | The fleet view consumes registered operations. Each operation is implemented by the owner of its data. |
| `docs/architecture/web-interface.md` | Component Structure | anchored to internal/component/web/fleetui/register.go | "Every page, fragment and editor is a templ component." | agrees | Use the current fleet templates and fragment handlers for page rendering. |
| `docs/architecture/testing/ci-format.md` | Syntax Overview | the // Design: page of internal/component/web/testing/parser.go | "All lines use key=value format" | agrees | Extend only the named fleet-hub fixture option. Existing test directives retain their grammar. |
| `docs/architecture/testing/runner-architecture.md` | Web test format (`.wb`) | anchored to internal/component/web/testing/parser.go | "Web tests live in `test/web/*.wb`" | agrees | The live-estate case uses the same headless browser driver and fixture lifecycle as the existing sample cases. |
| `ai/doors/le-tooling.md` | Where it sits | the door covering internal/test/cli/cmd_web.go | "Functional `.ci` files live under `test/<suite>/`" | agrees | Use the repository’s functional harness for managed behavior and its headless web harness for the views. |
| `ai/doors/hub-engine.md` | Boundaries | the door covering cmd/ze/hub/register_fleet_ui_estate.go | "No always-on file imports a gated package." | agrees | The build-gated composition file registers its provider. Builds without the optional UI still serve the fleet commands. |
| `docs/architecture/fleet-interface.md` | Design decisions | the // Design: page of internal/component/web/fleetui/source.go | "The proposed alarm defaults apply together. A missing relay grant stays visible while the device is offline. A measured alarm uses its existing threshold duration after reconnect. Causes match within one device. Thomas selected “Use all three defaults”." | agrees | A relay grant is independent of connectivity. Use the hub’s cause result for one device and its configured resource duration. |
| `docs/architecture/fleet-interface.md` | Design decisions | the // Design: page of internal/component/web/fleetui/source.go | "Thomas chose the following rule for Resource high at reconnect. “End the old occurrence. Show a new alarm only if fresh samples qualify.”" | agrees | A reconnect ends the old resource occurrence. Its historical opening and clearing survive, and any new qualifying interval has its own identity. |
| `docs/architecture/fleet-interface.md` | Design decisions | the // Design: page of internal/component/web/fleetui/source.go | "When a critical alarm becomes visible again after its cause clears, show its toast to an authorized viewer of the affected device. Thomas selected “Show the critical toast”." | agrees | The post-release notification provider checks visibility before delivering the critical recovery toast. |
| `docs/architecture/fleet-interface.md` | Design decisions | the // Design: page of internal/component/web/fleetui/source.go | "For Force complete on a staged rollout, Thomas chose “Leave unstarted devices on their prior version”." | agrees | The rollout result distinguishes devices admitted to the change from devices that keep their prior version. |
| `docs/architecture/fleet-interface.md` | Design decisions | the // Design: page of internal/component/web/fleetui/source.go | "Thomas chose “Add the device Alarms tab”. The post-release device page offers the same authorized alarm rows as the device filter on the estate or group Alarms tab." | agrees | The shared device navigation includes its Alarms tab and uses the same authorized device filter as the estate view. |
| `docs/architecture/fleet-interface.md` | The devices table | Live fleet interface | "over the unfiltered table" | departs | An invalid filter on an existing view keeps its last valid query and exposes the error. A fresh URL can use the unfiltered authorized set. |

| `docs/architecture/zefs-format.md` | Key Registry | Registered fleet preference storage | "All known key patterns are registered in `pkg/zefs/keys.go` via `MustRegister()`" | agrees | Register bounded per-account view and saved-filter preference keys. Validate the account in the trusted request before selecting the namespace. |
| `docs/architecture/container/image-store.md` | State keys | Registered fleet preference storage | — | silent | Saved fleet views are independent of container images and projects. They use separate registered key patterns. |
| `docs/architecture/diagnostics/debug-filtering.md` | One ZeFS trap | Registered fleet preference storage | "`List(prefix)` fails when the prefix ends in a slash." | agrees | Read the authorized account prefix without a trailing slash when enumerating saved views. |
| `docs/architecture/irr-owner.md` | Who may touch which key | Registered fleet preference storage | — | silent | The preferences owner writes only its own keys; these keys grant no IRR or route access. |
| `docs/architecture/storage-backends.md` | Durable publication | Registered fleet preference storage | "Sync failures are returned to the caller." | agrees | Publish a saved view only after the store confirms durable success and surface its failure to the operator. |
| `ai/doors/core.md` | Boundaries | Registered fleet preference storage | "Runtime state goes through the daemon's one store." | agrees | Borrow the existing statestore. The fleet view package owns the optional registered feature and the core needs no web import. |
| `docs/architecture/fleet-config.md` | Config Storage (Hub Side) | Managed client and preference keys | "The hub may write the configuration of a declared client at any time" | agrees | Bulk labels stage changes to the hub declaration. Saved per-account filters never overwrite a managed client’s desired configuration. |
| `docs/architecture/testing/verify-freshness-scope.md` | Certificate | The CI stage selection in stages.go | "The manifest records each path at the content the stages read." | agrees | Name the estate fixture, stage runner and real build-tag inputs in its freshness accounting. The separate sample stage must not stand in for the operational fixture. |
| `docs/architecture/plugin/feature-gates.md` | Dependent gates and dependent files | Gated real and sample binaries | "the tagged blank import is the gate" | agrees | Keep separate real and sample composition imports. Verify the ordinary shipped flavor cannot link the sample package. |
| `docs/architecture/plugin/plugin-system.md` | Registration fields | The fullStages registration checks | "Every call to one sits in a `register*.go` file" | agrees | The operational adapters register from init in their owning register files. The targeted stage sits alongside existing plugin registration checks. |
| `docs/architecture/site-facts.md` | The defect this page exists to remove | The site facts check shares fullStages | — | silent | The website facts producer is independent of the operational fleet browser fixture. Its stage remains reachable in the CI stage list. |
| `docs/architecture/fleet-config.md` | Design decisions | Fleet managed service design | "A reload that changes the certificate a serving hub's managed listener presents installs it on the running listener at that reload: the next handshake presents the certificate the configuration names, the listener keeps its socket and no session is closed, as the web and looking glass listeners do. The certificate stays in the configuration's `pki` block. In Thomas's words (voice-transcribed): "Keep the certificate in the configuration." and "In that case on commit we can perform the certificate change on the listener."" | agrees | UI-2 consumes authenticated registry facts. The listener and its certificates retain their existing reload behavior. |
| `docs/architecture/fleet-config.md` | Design decisions | Fleet managed service design | "Thomas chose “Allow first provisioning and later edits from the device’s web Config tab”. An authorized operator can commit the first or a later version of a declared device’s configuration while it is offline, through the post-release fleet web surface." | agrees | The estate tree may list devices before first contact. Bulk labels edit the hub’s declarations. Fleet-ui-6 owns the separately authorized creation and later editing of client config from its device tab. |

## Data Flow (MANDATORY)

### Entry Point
- A fleet web page request on a build with `ze_web && ze_fleet_ui`.

### Transformation Path

Use the registered web route’s authenticated request, fleet mode and session principal for every mutation. Requesting a different shell does not give the browser a different identity or rights. For a bulk label change, resolve each selected device against the current authorized estate; reject missing or newly disallowed targets before staging any change. Edit each matching server/client declaration in the caller’s existing hub candidate, keeping duplicate-listener declarations consistent, then display the staged diff and commit through the hub’s normal transaction. A concurrent registry change or abandoned editor must never stage or commit another user’s candidate. The authorization, session cookie and general editor already exist. Reuse them through RouteDeps rather than introducing a separate fleet login or candidate.

Keep preference writes scoped to the authenticated account. A private filter string is not data authorization: run it only against the current permitted facts. The real adapter must pass the caller’s trusted dispatcher identity on each read, not bind the first visitor at process startup. The global registration retains builder functions and shared transport only. If the registry refuses a single named lookup, expose the appropriate typed refusal. Missing and unavailable data do not turn into an empty successful result.
1. The estate part dispatches `show fleet devices` as the request's account and decodes the rows into `fleetui.Device` facts.
2. The interface joins every registered device-fact part by name; an absent part leaves its fields not reported.
3. The page renders; a part that is not registered renders its not-available state.

### Changed-Wire Census

**Search evidence:** Searched the fleet UI and its sample, the web test parser, CLI runner, stage selection, browser JavaScript and repository .wb, .ci, shell and Python files for MustRegisterSource, ReadSnapshot, queryDevicesTable, devicesActionAnswer, needs-build-tag and WBServerLG. The existing sample registration and the looking-glass fixture are positive controls for the two build paths. The browser tests fleet-devices-filter.wb and fleet-tree-selects-device.wb exercise the existing readers.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Fleet typed source | writer | `fleetui/sample/register/register.go` and `fleetui/sample/source.go`, estate data and SavedFilters | The review build registers one value implementing the complete Source. | Register that same sample behind each part. Preserve sample values and method bounds. |
| Fleet typed source | writer | Planned `fleetui/estate/estate.go`, inventory.go, samples.go and views.go plus the tagged hub registration | The current `show fleet devices` and named-device dispatcher results supply estate registry facts. | Operational parts project only their existing public fields and preserve errors and unavailable states. |
| Fleet typed source | reader | `internal/component/web/fleetui/pages.go` ReadSnapshot and the object and fragment handlers; `register.go` mountedSource | The page builds its Snapshot from Source before it derives displays. | Join by device name inside the request. An absent optional part is marked unavailable; a missing estate part leaves the routes unmounted. |
| Devices query and actions | writer | The fleet page templates, links, bulk-action selection and `assets/fleet.js`; browser users and saved URLs | The devices pages encode the tree, current filter, selected targets and page in server requests. | Carry the last valid filter as described in the Task. Derive the acting identity on the server. |
| Devices query and actions | reader | `fleetui/handler_devices.go` queryDevicesTable, `handler_tree.go`, `handler_devices_action.go` devicesActionAnswer and browser .wb tests | The table parses the selected view and gives an error on a bad term. The bulk handler reads the selected action from form values. | Keep existing link shapes and replay saved URLs. Revalidate current selections and account rights at stage and commit. |
| Per-account preferences | writer | Planned views.go stored-preference actions using the borrowed statestore and registered ZeFS keys | The current sample supplies SavedFilters; it has no durable account-scoped preference writer. | Write bounded, versioned account-specific values. Report a failed persistence attempt to the owner. |
| Per-account preferences | reader | ViewsPart.SavedFilters and TreeViews, `handler_admin.go` adminFiltersContent and the devices filter menu | The existing sample returns typed SavedFilter records for callers. | Use the authenticated account to select keys. An absent record takes the documented default without loading another user’s settings. |
| Browser fixture declaration | writer | `test/web/fleet-estate-live.wb` (planned) and existing .wb authors | Existing fixture files use option=server and needs-build-tag to describe a test daemon. | Introduce the fleet-hub server kind with the real/synthetic tag expectations. The existing sample fixtures retain their own build. |
| Browser fixture declaration | reader | `internal/component/web/testing/parser.go` ParseWBFile, `internal/test/cli/cmd_web.go` suite server selection, `internal/le/verify/engine/stages.go` | The parser enumerates WBServer kinds. The test runner already starts a peer alongside the LG daemon. | Parse the new kind, start and join the managed test peer, and select the real tagged fixture separately in CI. |

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Web to daemon | `web.RouteDeps.Dispatch` with the request's caller identity | No |
| Back ends to fleet interface | One registration per part from `init()`; a join by device name | No |

### Integration Points
- `internal/component/web/fleetui` - one slot per part; `ReadSnapshot` tolerates absent parts; the join.
- `cmd/ze/hub/register_fleet_ui_estate.go` - links the estate part on a real fleet build.

## Risks & Assumptions

| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 (was `fleet-1` A-5) | The web interface runs inside the hub daemon, so the process clock and host name are the hub's | `fleetui` routes are mounted by the daemon's web server through `web.RegisterWebRoute` | `Now` or `Estate` would answer another host's values | `TestEstatePartAnswersHubClock` | unvalidated |
| A-2 (was `fleet-1` A-8) | The browser test runner can start a hub daemon with a serving block and a real managed client beside it once it gains a server kind for it | `internal/test/cli/cmd_web.go` already starts a second process for a server kind (`zeTestStartLGServer` starts `ze-test test peer` beside the daemon); kinds are declared in `internal/component/web/testing/parser.go` (`WBServerLG`, `WBServerDaemon`); `option=needs-build-tag` accepts `state=absent` (`internal/test/runner/build_tags.go`) | The harness shape changes; the test stays a deliverable either way (AC-11) | Writing the `fleet-hub` kind and `test/web/fleet-estate-live.wb` | unvalidated |

| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 (was `fleet-1` R-11) | The sample build and the real estate part both register the estate part | Panic at start | The real part's hub file is constrained `!ze_fleet_ui_sample`; the second registration of a part panics as a composition defect, as today |
| R-2 (was `fleet-1` R-14) | The estate part dispatches as an identity that may not read `show fleet devices` | 403 on the page | It dispatches as the request's account; a refusal is an operator error the page shows (`FailureUnauthorized`) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A fleet page on a real `ze_fleet_ui` build (no `ze_fleet_ui_sample`) with one real managed client | → | the estate part over the dispatcher, the join, the not-available component | `test/web/fleet-estate-live.wb` |
| A page asking an unregistered part | → | the not-available component | `TestEveryAbsentPartRendersNotAvailable` |
| The tree, the devices table, a filter, a bulk label edit, the side panel | → | `handler_devices.go`, tree and filtered page handlers | `test/web/fleet-estate-live.wb` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEstatePartRequiredToMount` | `internal/component/web/fleetui/source_parts_test.go` | Routes mount only with the estate part | |
| `TestSecondPartRegistrationPanics` | same | AC-14, R-1 | |
| `TestReadSnapshotMarksAbsentActivity` | same | AC-8 alarms and tasks not available | |
| `TestReadSnapshotBuiltinViewsWithoutViewsPart` | same | AC-8 tree views | |
| `TestAbsentHealthPartLeavesUnknown` | same | AC-9 | |
| `TestJoinFillsDeviceFactsByName` | same | AC-13: a registered device-fact part fills only its fields | |
| `TestJoinIgnoresNameOutsideEstate` | same | AC-13: a name the estate does not hold appears nowhere | |
| `TestDeviceFactPartErrorLeavesFieldsNotReported` | same | A part answering an error leaves its fields not reported and the page renders | |
| `TestEveryAbsentPartRendersNotAvailable` | same | AC-12, one subtest per part of the split table, each asserting its `fleet-not-available-<part>` element on the pages the table names and status 200 | |
| `TestDeviceSystemJoinsHardwareFromInventoryPart` | same | `DeviceSystem.Hardware` from `InventoryHealthPart`, resources from `DeviceViewPart` | |
| `TestSampleRegistersEveryPart` | `internal/component/web/fleetui/sample/register/register_test.go` | AC-15: every slot holds the sample | |
| `TestEstatePartMapsRowsToDevices` | `internal/component/web/fleetui/estate/estate_test.go` | Row to `fleetui.Device` mapping | |
| `TestEstatePartDispatchErrorIsOperatorError` | same | A refused dispatch is an operator error, never empty | |
| `TestEstatePartAnswersHubClock` | same | A-1 | |
| `TestFleetDevicesMaxAgreesWithManagedClientsMax` | `cmd/ze/hub/fleet_ui_bounds_test.go` (`ze_web ze_fleet_ui`) | AC-10 | |
| `TestInventoryHealthPartMapsReports` | `internal/component/web/fleetui/estate/estate_test.go` | AC-16, AC-17 | |
| `TestLatestSamplePartMapsSamples` | `internal/component/web/fleetui/estate/estate_test.go` | AC-17 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `fleet-estate-live` | `test/web/fleet-estate-live.wb` (`option=server:kind=fleet-hub`, `ze_fleet_ui` present, `ze_fleet_ui_sample` absent) | AC-11: a real managed client listed online with its config version; the Alarms card not available | |

## Files to Modify
- `internal/component/web/fleetui/register.go` — retain the request’s authenticated session and existing RouteDeps.EditorManager in the operational route wrapper. Reuse `internal/component/web/editor.go`, `internal/component/web/auth.go` and `internal/component/web/ui_mode.go` for candidate, session and mode without replacing their behavior.
- `internal/component/web/fleetui/handler_tree.go`, `internal/component/web/fleetui/handler_devices.go`, `internal/component/web/fleetui/handler_devices_action.go` and `internal/component/web/fleetui/table.go` handle the scoped tree, filtered result and staged actions.
- `pkg/zefs/keys.go` registers account-scoped fleet preference keys used by the existing state store.
- `internal/le/verify/engine/stages.go` selects the tagged live fixture in CI; cover its freshness inputs. The real and sample builds have distinct cache identities.
- `internal/component/web/fleetui/source.go`, `source_devices.go`, `source_activity.go`, `source_config.go`, `source_history.go`, `source_views.go`, `source_events.go` - interfaces regrouped per part, bounds unchanged.
- `internal/component/web/fleetui/register.go`, `pages.go`, every page handler that renders a part's answer - parts, join, not-available markers.
- `internal/component/web/fleetui/sample/register/register.go` - register every part.
- `internal/component/web/testing/parser.go`, `internal/test/cli/cmd_web.go` - the `fleet-hub` server kind.
- `docs/architecture/fleet-interface.md` (parts, join, not available), `docs/guide/web-interface.md` (real estate, not-available parts) - the pages this work makes wrong.

## Files to Create
- `internal/component/web/fleetui/estate/views.go` and `internal/component/web/fleetui/estate/views_test.go` — ViewsPart for named tree views and per-account saved filters, storage bounds and user isolation.
- `internal/component/web/fleetui/estate/inventory.go` and `internal/component/web/fleetui/estate/samples.go` — the operational inventory/health and latest-sample parts over the first-release dispatcher commands. Only a valid returned device identity participates in the join.
- `internal/component/web/fleetui/source_parts.go` - one slot per part and the join; `source_parts_test.go`.
- `internal/component/web/fleetui/estate/estate.go`, `register.go`, `estate_test.go` - the estate part over the dispatcher.
- `cmd/ze/hub/register_fleet_ui_estate.go` - `ze_web && ze_fleet_ui && !ze_fleet_ui_sample` blank import.
- `cmd/ze/hub/fleet_ui_bounds_test.go`
- `internal/component/web/fleetui/component_unavailable.templ` - the not-available component.
- `internal/component/web/fleetui/sample/register/register_test.go`
- `test/web/fleet-estate-live.wb` - the live-estate browser test (AC-11).

## Design presentation

### What changes

Use the registered estate source to populate the tree, devices table, filter results and detail panel from the authenticated hub. Optional report parts add their sections without enabling sample data on a real install.

### Before and after

A device labelled site=west appears under West and in the matching table on the same request. Editing a label stages a change; a successful commit and refresh move it to its new group. If health reporting is unavailable the table keeps the device and explains the missing health field.

### Alternatives set aside

A browser-owned copy of the whole estate would duplicate visibility and filtering rules. Making every source part mandatory would remove useful inventory views on installations lacking history.

### Documents departed from

`docs/architecture/fleet-interface.md`, The devices table, currently shows filter errors above the unfiltered table. After applying a valid filter, a typo will keep those matching rows visible and show the error. On a fresh URL without a prior valid filter it shows the unfiltered authorized rows.

### Choices for Thomas

The one-second p95 for 2,000 devices is a proposed acceptance target. Record the host, query mix, response size and latency separately for the initial tree, table and filter.

## Implementation Steps

1. **Phase: Fleet interface parts (moved from `fleet-1`'s Phase 8)** - one slot per part, join, not-available markers in `ReadSnapshot` and the pages, sample registers every part, estate part over the dispatcher
   - Tests: `source_parts_test.go` (every test of AC-12 to AC-14), `estate_test.go`, `TestSampleRegistersEveryPart`, `TestFleetDevicesMaxAgreesWithManagedClientsMax`, `fleet-estate-live.wb` (AC-11), the existing sample `.wb` suite unchanged (AC-15)
   - Verify: the browser test is red before the estate part is registered (the routes do not mount) and green after, run with `./le test functional web`
2. **Phase: Inventory-health and latest-sample parts** - over `fleet-4`'s and `fleet-8`'s commands (AC-16, AC-17)
3. **Phase: Tree, table, filters, bulk actions, side panel** - connect the existing filter, pagination and selection handlers to the authorized registry snapshot. Exercise malformed filters, two different page sizes, stale selected targets on commit, and URL restoration after navigation

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-17 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass (`./le test functional web`, the scoped package tests), run in the main checkout by the main thread

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
