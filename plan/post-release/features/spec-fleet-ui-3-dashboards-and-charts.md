# Spec: fleet-ui-3 -- Estate, Group and Device Dashboards

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | web, fleet |
| Priority | P3 |
| Release reason | Adds a capability absent from the supported release; existing supported behavior and its obligations remain required. |
| RFC gaps | - |
| Depends | spec-fleet-ui-2-estate-tree-and-device-tables.md, spec-fleet-4-inventory-health (closed 2026-11-26; documented in `docs/architecture/fleet-config.md`, "Inventory and health reporting"), plan/release/features/spec-fleet-8-metric-history.md |
| Phase | 3/9 of plan/post-release/features/spec-fleet-ui-0-umbrella.md |
| Handoff | - |
| Updated | 2026-11-25 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

**Re-read after compaction:** this file, `spec-fleet-ui-0-umbrella.md`,
`plan/release/features/spec-fleet-8-metric-history.md`.

## Task

Make the Dashboard tab of the estate, of a group node and of a device functional.
The Dashboard is the default tab of every object, so it is the first thing an
operator sees: it must answer "is anything wrong, and where" at a glance, and
lead to the rows behind every figure.

The owner's ruling binds every card: a dashboard must not hide detail behind
vague health cards. So a card is a count of something specific with a link to
the table showing exactly those things. A card with nothing behind it is not
built.

### Card rules

| Rule | Detail |
|------|--------|
| Title | Names what is counted, in the card title style |
| Link | The title links to the table or tab listing what the card counts, already filtered |
| Segments | Every segment of a donut or stacked bar, and every row of a list, links to its own filtered view |
| Loads alone | Each card is its own fragment. A slow or failed card shows its own state and does not delay the others |
| Refresh | Cards over hub-held state refresh from the live event stream when what they count changes. Charts refresh at the sample interval |
| Empty | A card with nothing to count says so in one line and keeps its size, so the grid does not jump |
| Size | Cards in a row share one height |

### Card appearance

Checked against two screenshots of the reference layout on 2026-11-18. These
add to the appearance table of `spec-fleet-ui-0-umbrella.md`.

| Part | Appearance |
|------|------------|
| First row of the estate dashboard | Four cards of equal width and height: three status donuts and one figures card. This row is what an operator reads first, so it is never pushed down by anything |
| Status donut card | Title at the top left. The ring at the left with the object's icon in its centre, drawn with small gaps between segments; the legend to its right, one line per state. TOTAL and its figure at the bottom right of the card |
| Legend line | A round dot in the state's colour, the state word, then the count in a slightly lighter weight. A state that needs explaining carries a small round information mark after its word, whose tooltip says what the state means. A state with a count of zero is still listed, so the legend's shape never changes |
| Figures card | Pairs of an uppercase caption and a large figure, laid out in two columns. A figure with a unit shows the unit as a small suffix on the baseline, for example 2.3 with a small TiB |
| Grouped donuts | Where two related things are counted, they share one wide card: each has its own uppercase caption, ring and legend, and a thin line separates them |
| Grouped usage | Related usage bars share one full-width card, split into columns by thin vertical lines. Each column has a title, a one-line subtitle in `--text-secondary`, the bar, its legend, and a foot of caption and figure pairs such as USED, AVAILABLE and TOTAL |
| Usage bar with a figure inside | When the filled part is wide enough, its percentage is written inside it; otherwise beside it. A second category is a second colour in the same track |
| Page | Cards sit on the page background with a small even gap. Space left under the last row stays empty; cards are not stretched to fill it |

### Estate dashboard

Twelve columns. The number after each card is its width.

```
+----------------+----------------+----------------+----------------+
| DEVICES    (3) | CONFIGURATION(3)| SOFTWARE   (3) | ESTATE     (3) |
|  (donut)       |  (donut)       |  (donut)       |  44 devices    |
|  36 Healthy    |  39 In sync    |  1.4.2   31    |  4 sites       |
|   3 Degraded   |   2 Pending    |  1.4.1   12    |  412 peers up  |
|   1 Down       |   1 Rejected   |  1.3.9    1    |  of 420        |
|   3 Offline    |   1 Diverged   |                |  9 open alarms |
|   1 Diverged   |   1 Never      |                |                |
+----------------+----------------+----------------+----------------+
| ALARMS                                  (8)     | NEEDS ATTENTION (4) |
|  Critical  edge-ams-02  bgp down      14:02     |  1 rollout paused   |
|  Warning   core-lon-01  memory 91 %   13:40     |  1 device diverged  |
|  ...                          see all 9         |  1 config rejected  |
|                                                 |  1 offline > 1 day  |
+--------------------------+----------------------+---------------------+
| ROLLOUTS             (4) | RECENT CHANGES   (4) | BUSIEST DEVICES (4) |
|  edge-refresh   paused   |  14:10 thomas pushed |  by CPU | Memory |.. |
|  2 of 5 batches          |  config to 4 devices |  core-lon-01   91 % |
+--------------------------+----------------------+---------------------+
| DEVICES CONNECTED, LAST WEEK       (6) | TRAFFIC, ESTATE TOTAL    (6) |
|  (line chart)                          |  (line chart, in and out)    |
+----------------------------------------+------------------------------+
```

| Card | Shows | Leads to | Data |
|------|-------|----------|------|
| Devices | Donut by display state, with a legend of word and count | Devices tab filtered by that state | Registry (`fleet-1`), health (`fleet-4`) |
| Configuration | Donut by configuration state | Devices tab filtered by that configuration state | Registry |
| Software | Donut by software version, newest first, at most four versions named and the rest grouped as Older | Devices tab filtered by that version | Inventory (`fleet-4`) |
| Estate | Figures: devices, sites, BGP peers established of configured, open alarms | Devices tab, alarms tab | Registry, latest samples (`fleet-8`) |
| Alarms | The eight most severe open alarms: severity word, device, summary, since | Alarms tab; each row to the device | Alarms (`fleet-ui-5`); the count of alarms hidden behind their cause, shown with it, is `fleet-ui-9`'s |
| Needs attention | One line per condition needing an operator: paused rollouts, diverged devices, rejected configurations, devices offline for more than a day, devices more than one version behind | The matching filtered view | Registry, rollouts (`fleet-5`) |
| Rollouts | Running and recently finished rollouts with state and progress | Rollouts tab | `fleet-5` |
| Recent changes | The last eight audit entries: time, actor, what, how many devices | Audit page | `fleet-3` |
| Busiest devices | Five devices by CPU, memory or traffic, chosen by a small tab inside the card, each with a usage bar | The device | Latest samples |
| Devices connected | Connected device count over the chart range | None: it is the estate's own series | History (`fleet-8`) |
| Traffic, estate total | Sum of reported interface traffic, in and out | None | History |

### Group node dashboard

The same cards as the estate, scoped to the node's devices, without Rollouts and
Recent changes, and with the Estate card replaced by a Group card naming the
label values that define the node. The charts aggregate the node's devices.

### Device dashboard

Eight columns.

```
+-----------------------------------------------------------------------+
| QUICK INFO                                                        (8) |
| edge-lon-04   Healthy   site=lon role=edge   model X   serial ...     |
| version 1.4.2   up 41 d   config in sync (a3f9)   seen now   10.0.4.1 |
+-----------------------------------------------+-----------------------+
| ALARMS                                    (5) | CONFIGURATION     (3) |
+-----------+-----------+-----------+-----------+-----------------------+
| PEERS (2) | IFACES (2)| MEMORY (2)| STORAGE(2)|
+-----------+-----------+-----------+-----------+
| CPU   (2) | MEMORY (2)| TRAFFIC(2)| LOAD   (2)|   line charts
+-----------+-----------+-----------+-----------+
```

| Card | Shows | Leads to | Data |
|------|-------|----------|------|
| Quick info | Name, state, labels, model, serial number, software version, uptime, configuration state and version, last seen, management address; each identifier with a copy control | System tab | Registry, inventory |
| Alarms | The device's open alarms | The device's alarms | Alarms |
| Configuration | State, version, time of last fetch and its result, and the divergence banner when diverged | Config tab | Registry |
| Peers | Donut: established and remaining configured peers; distinguish administrative shutdown only when the source supplies that count | Routing tab | Latest samples |
| Interfaces | Donut: up and remaining configured interfaces; distinguish administrative shutdown only when the source supplies that count | Interfaces tab | Latest samples |
| Memory, Storage | Usage bars with used and total | System tab | Latest samples, inventory |
| CPU, Memory, Traffic, Load | Line charts over the chart range | None | History |

When the device is offline the dashboard shows Quick info and one full-width
offline state naming when it was last seen, in place of the live cards. The
charts stay, ending where the samples end, because the history is held by the hub.

### Charts

| Rule | Detail |
|------|--------|
| Chart card | Title at the top left with the period under it as a one-line subtitle in `--text-secondary` ("Last week"). The plot fills the card: faint horizontal and vertical grid lines, value labels with their unit on the left axis, day and time labels on two lines under the horizontal axis. The legend is centred under the plot: a short line sample in the series colour, then the series name |
| Chart row | Four chart cards of equal width form the last row of an object's dashboard. The row above them holds the status donut and the usage cards; the Quick info card spans the full width above both |
| Series colours | The first series is `--chart-1`. A second series on the same plot (traffic out beside traffic in) is `--chart-3`, so two lines are told apart without relying on a legend alone |
| Range | One selector per dashboard: one hour, one day, one week. Default one day. Carried in the URL as `range` |
| Drawing | The line chart component of the umbrella, on the vendored uPlot, reading its colours from the chart variables at draw time |
| Series | At most four per chart, each named in a legend with its latest value |
| Gaps | A period with no samples is drawn as a gap, never joined by a line |
| Hover | A cursor line with the time and each series value |
| Axes | Time on the horizontal axis in the operator's local time; the unit named once on the vertical axis |
| Data | Fetched as JSON from the existing chart fragment, reduced to its existing ceiling of 300 points per series. Each nonempty bucket retains its average and maximum; an empty bucket stays a gap |
| No history yet | "No samples have been reported yet" with the time the first is expected |

### Key design decisions

| Decision | Detail |
|----------|--------|
| Estate and group dashboards never ask a device | They read the registry, the latest reported samples and the history, all held by the hub. They render the same whether devices are reachable or not |
| Cards are registered entries | A dashboard is an ordered list of registered cards with their width, so a later feature adds a card without editing the dashboard |
| Reduction happens on the hub | The browser never receives more points than it can draw |
| One range per dashboard | Charts beside each other always show the same period, so they can be compared |

### Data this spec reads

| Data | Expected from | State |
|------|---------------|-------|
| Counts by state, configuration state and version, for the estate or a set of labels | Fleet UI-2’s scoped estate query over `show fleet devices`, joined to retained health | Count the same authorized set used by the linked table |
| Latest samples per device and summed over a set | `fleet-8` | Missing |
| A reduced series for a metric, a device or a set, and a range | `fleet-8` | Missing |
| Open alarms, most severe first | `fleet-ui-5` | Missing |
| Rollouts and audit entries | `fleet-5`, `fleet-3` | Missing; the two cards are added when those land |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | The estate dashboard | Every count equals the number of rows in the view its link opens | A count may trail a change by one event, because cards refresh from the event stream |
| AC-2 | A device changes state | The Devices card, the Needs attention card and the tree change without a reload | Charts do not redraw on state changes, because they follow the sample interval |
| AC-3 | Devices unreachable | The estate and group dashboards render completely from hub-held state | Latest sample figures are marked with their age when older than three sample intervals |
| AC-4 | The chart range is changed | Every chart on the dashboard redraws for the new range and the URL carries it | None |
| AC-5 | One card's data source fails | That card shows the failed state with a retry; every other card renders | None |
| AC-6 | An offline device's dashboard | Quick info, the offline state with the last-seen time, and charts ending at the last sample | Live cards are absent, because they would show stale values as current |
| AC-7 | A period without samples | The chart shows a gap | None |

### Moved in (owner ruling 2026-11-23)

Thomas ruled (voice-transcribed answer "1 b") that the fleet features of the
first release are reached through commands on every command surface, and that
their web pages and views come after the first release (`ai/META-DECISIONS.md`,
"Goal and scope"). The dashboard views the first-release back ends named moved
here; each back end keeps its commands.

| AC ID | From | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|------|-------------------|-------------------|-----------------------------|
| AC-8 | `spec-fleet-4-inventory-health` AC-4, its web half | A device reports a degraded component (`fleet-4`'s health-report), on a real build with the inventory-health part registered (`fleet-ui-2` AC-17) | The estate and group dashboards' Devices card counts it Degraded, its segment links to the devices table filtered to `state:degraded`, and the device's Quick info shows Degraded; the count equals what `show fleet health` answers | A count may trail the report by one event, because cards refresh from the event stream |
| AC-9 | The `HistoryPart` registration, given to `plan/release/features/spec-fleet-8-metric-history.md` by `fleet-1`'s part table, moved here with the part table (`plan/post-release/features/spec-fleet-ui-2-estate-tree-and-device-tables.md`, "Moved from fleet-1") | A real build with the history part registered over the dispatcher | Every chart draws exactly the series `fleet-8`'s series query answers for the same figure, device or set, and range; the history administration figures (`fleet-ui-6`) are `fleet-8`'s store status answer; a refused dispatch is an operator error the card shows, never an empty chart | A figure `fleet-8` answers as absent is drawn as absent, never as zero (`fleet-8` AC-11) |

`fleet-8` carries no chart: every chart over its history is this spec's.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/api-surfaces.md` - the web UI is a thin adapter: a card reads commands through the dispatcher, never a back end's package
  → Constraint: the history and inventory-health parts read `fleet-8`'s and `fleet-4`'s commands through `web.RouteDeps.Dispatch`.
- [ ] `docs/architecture/fleet-interface.md` - the data source seam and its parts (`fleet-ui-2`, "Moved from fleet-1")

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/web/fleetui/source.go` - `Source` is the union of six interfaces, `HistorySource` among them; one source per build today. `fleet-ui-2` splits it into parts, `HistoryPart` among them.

## Existing Mechanisms

| Mechanism | Owner | Current behavior | Design |
|---|---|---|---|
| Dashboard source | `source_history.go`, `HistorySource`; `handler_chart.go`, `chartSeries` and `chartLine` | Chart requests select the device or estate query and represent missing buckets as gaps. | Supply a dispatcher-backed history part and preserve the existing gap and refusal behavior. |
| Cards | `register_dashboard_pages.go` and `handler_dashboard_cards.go` | Individual card handlers consume source facts. | Reuse the card registration and keep a failed card independent of the rest of the dashboard. |
| Series and card derivation | fleetui/summary.go, Reduce, CountByState and NeedsAttention | Folds raw points and derives card counts from the selected estate. | Keep Reduce for the sample source. The live history query returns buckets with coverage, average and peak. Pass them through without running Reduce again. Counts use the exact authorized target set of the link. |
| Dashboard and invalidation | handler_dashboard.go, handler_dashboard_device.go, handler_events.go | Register and render scoped cards; the device path suppresses live cards when offline; the event stream identifies changed views. | Retain offline slots and subscribe to UI-9's scoped events for the user's current object. |
| History administration and chart script | handler_admin.go, adminHistoryContent; assets/fleet-chart.js | Render the store usage and draw JSON gaps. | Adapt store usage from the hub status query; pass absent values, peaks and gaps through the existing JSON drawing path. |
| Service wiring | web.RouteDeps.Dispatch, fleetui/register.go and the hub's build-tagged provider import | The daemon builds routes with its dispatcher. | UI-2 supplies the authenticated request and UI-3 registers the history provider on the real build. |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|---|---|---|---|---|---|
| `ai/doors/system.md` | One schema, one command registry, every interface | the door every change reads | "A command is declared once and reaches every interface." | agrees | The web and command presentations read facts through the registered operation under the caller’s identity. |
| `ai/doors/api-surfaces.md` | Boundaries | the door covering internal/component/web/fleetui/handler_chart.go | "Never add a command in an adapter." | agrees | The fleet view consumes registered operations. Each operation is implemented by the owner of its data. |
| `ai/doors/le-tooling.md` | Where it sits | the door covering test/web/fleet-dashboard-history.wb | "Functional `.ci` files live under `test/<suite>/`" | agrees | Use the repository’s functional harness for managed behavior and its headless web harness for the views. |
| `docs/architecture/fleet-interface.md` | Design decisions | the // Design: page of internal/component/web/fleetui/handler_chart.go | "The proposed alarm defaults apply together. A missing relay grant stays visible while the device is offline. A measured alarm uses its existing threshold duration after reconnect. Causes match within one device. Thomas selected “Use all three defaults”." | agrees | A relay grant is independent of connectivity. Use the hub’s cause result for one device and its configured resource duration. |
| `docs/architecture/fleet-interface.md` | Design decisions | the // Design: page of internal/component/web/fleetui/handler_chart.go | "Thomas chose the following rule for Resource high at reconnect. “End the old occurrence. Show a new alarm only if fresh samples qualify.”" | agrees | A reconnect ends the old resource occurrence. Its historical opening and clearing survive, and any new qualifying interval has its own identity. |
| `docs/architecture/fleet-interface.md` | Design decisions | the // Design: page of internal/component/web/fleetui/handler_chart.go | "When a critical alarm becomes visible again after its cause clears, show its toast to an authorized viewer of the affected device. Thomas selected “Show the critical toast”." | agrees | The post-release notification provider checks visibility before delivering the critical recovery toast. |
| `docs/architecture/fleet-interface.md` | Design decisions | the // Design: page of internal/component/web/fleetui/handler_chart.go | "For Force complete on a staged rollout, Thomas chose “Leave unstarted devices on their prior version”." | agrees | The rollout result distinguishes devices admitted to the change from devices that keep their prior version. |
| `docs/architecture/fleet-interface.md` | Design decisions | the // Design: page of internal/component/web/fleetui/handler_chart.go | "Thomas chose “Add the device Alarms tab”. The post-release device page offers the same authorized alarm rows as the device filter on the estate or group Alarms tab." | agrees | The shared device navigation includes its Alarms tab and uses the same authorized device filter as the estate view. |
| `docs/architecture/fleet-interface.md` | Dashboards | Live fleet interface | "the series reduced by `Reduce`" | departs | The sample may supply raw points; the live store supplies reduced, aligned buckets. Preserve the limit and rendering for both. |
| `docs/architecture/fleet-interface.md` | A source answers facts, the package derives | Live fleet interface | "A source answers facts only" | departs | The history producer supplies bucket coverage and extrema as part of its query answer; cards still derive their presentation from source facts. |
| `ai/doors/hub-engine.md` | Boundaries | The hub registration for the post-release history provider | "No always-on file imports a gated package." | agrees | The optional history provider is linked by its build-tagged hub registration file. The hub build without the web component has no import or dashboard route. |

## Data Flow (MANDATORY)

### Entry Point
- A dashboard request for the estate, a group node or a device.

### Transformation Path
1. Each card asks its part through the dispatcher-backed source; the history part asks `fleet-8`'s series query.
2. The card renders its count or chart, or its own failed or not-available state.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Web to daemon | `web.RouteDeps.Dispatch` with the request's caller identity | No |

### Integration Points
- `register_dashboard_pages.go` owns card registration. Supply the history part from the real-source build, alongside the estate part introduced by Fleet UI-2. `handler_chart.go` keeps the gap for a period without a sample.
- Initial rendering and explicit refresh require only the hub-held facts. The live-refresh test of AC-2 additionally uses the authorized event source from Fleet UI-9. A build missing that optional part still renders and explains the availability of live updates.

### Changed-Wire Census

**Search evidence:** Searched the in-tree Go, templ, JS, HTML and functional fixtures for dashboard-chart, data-chart-url, DeviceSeries, EstateSeries, HistoryUsage, chartDataSeries and fleet-chart.js. chartAnswer writes the current chart JSON; lines in fleet-chart.js reads average and maximum series and the sample package supplies raw points. The live provider is new.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|---|---|---|---|---|
| selected history series and usage | writer | fleet-8 managed/cmd_history.go history queries; fleetui/history/history.go adapter; fleetui/sample/history.go existing sample source | source_history.go defines DeviceSeries, EstateSeries and HistoryUsage. | Carry aligned coverage, average and maximum from the authorized live daemon. Preserve the raw sample fixture contract. |
| same | reader | fleetui/handler_chart.go chartSeries, chartAnswer; handler_dashboard_cards.go; handler_admin.go adminHistoryContent | The live hub has already reduced its series, and the sample fixture still feeds Reduce. | Draw the live bucket once, retain gaps and preserve the chart point limit. |
| dashboard-chart HTTP JSON | writer | fleetui/handler_chart.go dashboardChartFragment and chartAnswer; component_card.templ and its generated file place the chart URL/legend data in the DOM | chartData and chartDataSeries carry the time axis, unit, named values, maxima and empty/refusal text. | Keep parallel arrays aligned with UTC slots and use null for gaps. |
| same | reader | fleetui/assets/fleet-chart.js lines and chart drawing; dashboards in fleetui/dashboard_test.go and the headless web suite; browsers loading the chart URL | The JS matches series to legend data and consumes the optional maximum; golden ChartCard and Dashboard fixtures preserve the DOM contract. | Retain the existing chart envelope; consume typed missing and failure states inside the relevant card. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A device dashboard on a real build with a reporting device | → | the history part over `fleet-8`'s series query, the chart card | `test/web/fleet-dashboard-history.wb` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestHistoryPartDrawsFleet8Series` | `internal/component/web/fleetui/history/history_test.go` | AC-9 | |
| `TestDevicesCardCountsDegraded` | `internal/component/web/fleetui/dashboard_test.go` | AC-8 | |

## Files to Modify
- `internal/component/web/fleetui/handler_chart.go` and `internal/component/web/fleetui/handler_dashboard_cards.go` render the live data, its age, gaps and typed refusals.
- `internal/component/web/fleetui/source_history.go` defines the dashboard contract used by the dispatcher-backed adapter.
- `docs/architecture/fleet-interface.md` documents available cards and data owners for the live build.

## Files to Create
- `cmd/ze/hub/register_fleet_ui_history.go` links the registered history adapter under the same real-fleet-only build constraint as the estate provider.
- `internal/component/web/fleetui/history/register.go` and `internal/component/web/fleetui/history/history.go` register and serve the live history part through the caller's dispatcher.
- `test/web/fleet-dashboard-history.wb` exercises an authorized real-device series, a gap and a failed source beside a working card.

## Design presentation

### What changes

Use the registered history and inventory providers for dashboard cards. The selected device, group, time range and caller identity reach the same hub queries as the command surface.

### Before and after

On a real hub an offline device shows its Quick info, offline state and historical charts. Its live cards are absent. Estate and group summaries can show retained readings with their age. An hour without reports appears as a gap. An unauthorized or unavailable series produces a card-level message while the other cards still render.

### Alternatives set aside

Recomputing history in the browser would create another aggregation rule and send per-device samples for an estate query. Scraping devices live would make historical cards unavailable during outages.

### Documents departed from

`docs/architecture/fleet-interface.md`, A source answers facts, the package derives, and Dashboards, describe raw points reduced by the view. The sample keeps that path. For live data the hub history query reduces the full estate before transport and returns timestamped buckets of coverage, average and maximum. The view draws the returned buckets once, keeping the same 300-point cap.

### Choices for Thomas

The history provider must preserve missing figures and gaps during downsampling. Live refresh exercises the event provider from Fleet UI-9.

## Implementation Steps

1. **Phase: Wiring** - mount a real dashboard over the estate and history parts, compare its data to the command for that same caller, then exercise the view transitions in AC-1 through AC-9.
2. Register the live history adapter in the hub build. Pass the caller, object or label scope, metric and range through the dispatcher. Retain errors, units, coverage, peaks and gaps when converting the answer.
3. Bind cards to the estate, health, alarm and sample sources. Exercise a disconnected device, a missing optional figure, a failed card and a label-scoped caller. The count links open the same filtered set.
4. Subscribe live cards to the scoped invalidations from UI-9 and refetch on reconnect. Verify changed device state updates linked counts, cancellation releases subscriptions and chart range survives navigation.
   - Verify the dashboard tests in the fleetui package and the named `fleet-dashboard-history.wb` fixture through the headless web harness.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-9 all demonstrated

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
