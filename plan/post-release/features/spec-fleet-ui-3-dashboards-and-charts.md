# Spec: fleet-ui-3 -- Estate, Group and Device Dashboards

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | web, fleet |
| Priority | P3 |
| Release reason | Adds a capability absent from the supported release; existing supported behavior and its obligations remain required. |
| RFC gaps | - |
| Depends | spec-fleet-ui-2-estate-tree-and-device-tables.md, plan/release/features/spec-fleet-4-inventory-health.md, plan/release/features/spec-fleet-8-metric-history.md |
| Phase | 3/8 of plan/post-release/features/spec-fleet-ui-0-umbrella.md |
| Handoff | - |
| Updated | 2026-11-18 |

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
| Alarms | The eight most severe open alarms: severity word, device, summary, since | Alarms tab; each row to the device | Alarms (`fleet-ui-5`) |
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
| Peers | Donut: established, not established, administratively down | Routing tab | Latest samples |
| Interfaces | Donut: up, down, administratively down | Interfaces tab | Latest samples |
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
| Data | Fetched as JSON from a fragment route, already reduced by the hub to at most the pixels available |
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
| Counts by state, configuration state and version, for the estate or a set of labels | `fleet-1`, as a summary form of `show fleet devices` | Missing |
| Latest samples per device and summed over a set | `fleet-8` | Missing |
| A reduced series for a metric, a device or a set, and a range | `fleet-8` | Missing |
| Open alarms, most severe first | `fleet-ui-5` | Missing |
| Rollouts and audit entries | `fleet-5`, `fleet-3` | Missing; the two cards are added when those land |

### Draft acceptance criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | The estate dashboard | Every count equals the number of rows in the view its link opens | A count may trail a change by one event, because cards refresh from the event stream |
| AC-2 | A device changes state | The Devices card, the Needs attention card and the tree change without a reload | Charts do not redraw on state changes, because they follow the sample interval |
| AC-3 | Devices unreachable | The estate and group dashboards render completely from hub-held state | Latest sample figures are marked with their age when older than three sample intervals |
| AC-4 | The chart range is changed | Every chart on the dashboard redraws for the new range and the URL carries it | None |
| AC-5 | One card's data source fails | That card shows the failed state with a retry; every other card renders | None |
| AC-6 | An offline device's dashboard | Quick info, the offline state with the last-seen time, and charts ending at the last sample | Live cards are absent, because they would show stale values as current |
| AC-7 | A period without samples | The chart shows a gap | None |
