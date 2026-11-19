# Fleet Interface

The fleet interface is the web interface an operator uses to see and manage
every device a hub manages. It is served by the web server under `/fleet/`.
This page describes its package, how it reads its data, the two build tags
that decide whether it is linked and whether it is served, and how a review
build is made.

The interface is being built as a prototype first: the real routes, shell and
markup, reading a sample estate. No page acts on a real estate yet.

## The package

`internal/component/web/fleetui` holds the whole interface: its handlers, its
templates, its styles and scripts. It imports the web package for two things,
the route registry and the username of the request. The web package never
imports it.

| File | Holds |
|------|-------|
| `source.go` | The data source interface, and the slot that holds the one source of a build |
| `source_devices.go`, `source_history.go`, `source_activity.go`, `source_config.go`, `source_views.go` | The questions of the interface, one file per concern, with their value types and bounds |
| `source_events.go` | The live event subscription: `Event`, `Feed`, `Subscription` |
| `source_errors.go` | The failures a source reports, and `FailureOf`, which reads the state a page shows for one |
| `state.go` | The display state and its precedence, and the configuration, health, alarm and task states with their words and keys |
| `derive.go` | `DeviceView`: one device with everything derived from its facts, built once per request |
| `url.go`, `tabs.go` | The addresses of the interface, parsed and rendered, and the registered tabs of each kind of object |
| `filter.go` | The filter language of the tables |
| `tree.go` | The estate tree: tree views, grouping, lookup of a node by its address |
| `table.go` | Sorting, paging, and the one query function of the devices table |
| `summary.go` | The counts, rankings and lines a dashboard shows, and the reduction of a series to the width of a chart |
| `register.go` | The route registrations and the functions that enable them |
| `register_tabs.go`, `register_devices.go` | The tabs, and the filter fields, sort columns and tree levels of the devices table |
| `pages.go` | The page registry and the fragment registry, and `PageRequest`, which every builder is handed |
| `handler_shell.go` | The page handler: an address resolved to a registered page inside the shell, and the assets handler |
| `handler_fragment.go` | The fragment handler, and the fragments of the shell's own regions |
| `handler_tree.go`, `handler_search.go` | The estate tree of the sidebar with its two fragments, and the header search |
| `handler_events.go` | The live event stream of one page, and the prototype-only step and hold routes |
| `handler_action.go` | The action registry: `MustRegisterAction`, and the handler of `POST /fleet/action/` that answers a registered action's toast |
| `registry.go` | The panic every registry of the package raises for a registration it cannot accept |
| `shell_view.go`, `shell_states.go` | The view data of the shell, the states it shows in place of a page, and the task panel it fills |
| `shell.templ`, `tree.templ`, `search.templ` | The shell (header, sidebar, main pane, side panel region), the tree, the search results |
| `assets/fleet-shell.css`, `fleet-theme.js`, `fleet.js` | The styles and the behavior of the shell |
| `icons.templ` | The icon set, the state marks and the illustrations |
| `component_*.templ`, `component_*.go` | The components and their view data, one family per file: actions, cards, donut, usage, table, side panel, pane states, toasts, tasks, terminal and diff, gallery |
| `assets/fleet-components.css`, `fleet-components.js`, `fleet-chart.js`, `fleet-terminal.js` | The styles and the behavior of the components |
| `register_dashboard_pages.go`, `handler_dashboard.go`, `handler_dashboard_cards.go`, `handler_dashboard_device.go`, `handler_chart.go`, `page_dashboard.templ`, `assets/fleet-dashboard.css` | The dashboards: the card registry, the cards of the estate and group dashboards, the cards of a device's, and the JSON of the chart cards |
| `register_devices_pages.go`, `handler_devices.go`, `handler_devices_action.go`, `page_devices.templ`, `assets/fleet-devices.css` | The devices table of the estate and of a group node, with its query row, selection, bulk actions and side panel |
| `register_device_pages.go`, `handler_device_system.go`, `handler_device_interfaces.go`, `handler_device_routing.go`, `handler_device_logs.go`, `handler_device_console.go`, `page_device.templ`, `assets/fleet-device.css` | The tabs of a device: System, Interfaces, Routing, Logs and Console, and the console route |
| `register_activity_pages.go`, `handler_tasks.go`, `handler_alarms.go`, `handler_prototype.go`, `page_activity.templ`, `assets/fleet-activity.css` | The Tasks and Alarms tabs, the task list, the alarm-kind tab registry, and the States page of the prototype |
| `register_manage_pages.go`, `handler_config.go`, `handler_rollouts.go`, `handler_audit.go`, `handler_admin.go`, `page_config.templ`, `page_rollouts.templ`, `page_admin.templ`, `assets/fleet-manage.css` | A device's Config tab, the rollouts, the audit trail and its export, and the administration pages |
| `guard_test.go`, `style_test.go`, `golden_test.go` | The markup guards, the style rule and the golden capture, run over this package |
| `golden_<family>_test.go` | The capture plan of each family's templates, which `golden_test.go` joins |
| `<family>_test.go`, `fixture_test.go`, `fixture_source_test.go` | The tests of each page family, over a fixed test source |

The package embeds its own assets, so a build without the package carries none
of them. The shell also links the vendored htmx and uPlot the web package
serves at `/assets/`.
<!-- source: internal/component/web/fleetui/handler_shell.go -- assetsFS, fleetHandler, newAssetHandler -->
<!-- source: internal/component/web/fleetui/shell.templ -- Shell, the head block -->

## The data source seam

The fleet pages never read an estate themselves. They ask a `Source`, and a
build links exactly one. The page markup does not change when the source does.

| Part | Contract |
|------|----------|
| `Source` | The questions the pages ask, one method per question. It is the union of six interfaces, one per concern |
| A method | Takes the request's context and answers a value and an error. Its comment names the pages that ask it and the `fleet` spec expected to answer it for real |
| A slice answer | Bounded by a named constant of the same file, such as `DevicesMax` |
| An answer | The caller's own copy. Plain structs with exported fields, enums whose zero value is not a state, `time.Time` for every time |
| An implementation | Safe for concurrent use. When it cannot answer it returns an operator error and never an empty answer, because the page shows the error text as it is |
| `MustRegisterSource` | Called from `init()` in the source package's `register.go`. A nil source and a second registration panic at process start: both are defects of the build's composition |

| Interface | Answers | Expected from |
|-----------|---------|---------------|
| `RegistrySource` | The hub's clock, the estate's name, every device with its facts, one device, the declared tree views, a user's saved filters | `fleet-1`, with `fleet-4` and `fleet-8` for the inventory, the health and the last sample of a device |
| `HistorySource` | The samples of one metric for a device, or summed over named devices, and how much the history holds | `fleet-8` |
| `ActivitySource` | The open alarms, a device's alarm history, the tasks, one task with its per-device results | `fleet-ui-5` |
| `ConfigSource` | A device's configuration, its versions and their text, a divergence, the rollouts, the audit trail, the templates, the access table, the settings | `fleet-2`, `fleet-3`, `fleet-5`, `fleet-7` |
| `DeviceViewSource` | What only the device holds: system, interfaces, BGP peers and summary, other adjacencies, logs, and one console command | `fleet-9`, through the relay |
| `EventSource` | A subscription to what changes in the estate | `fleet-ui-5` |

Two sources are planned. The sample source, `internal/component/web/fleetui/sample`,
answers from a fixed estate and exists so the interface can be judged before a
real estate does. The real source will be built over the command dispatcher
by the specs that make each page functional.
<!-- source: internal/component/web/fleetui/source.go -- Source, sourceSlot, MustRegisterSource -->
<!-- source: internal/component/web/fleetui/source_devices.go -- RegistrySource, Device, DevicesMax -->
<!-- source: internal/component/web/fleetui/source_history.go -- HistorySource, Series -->
<!-- source: internal/component/web/fleetui/source_activity.go -- ActivitySource, Alarm, Task -->
<!-- source: internal/component/web/fleetui/source_config.go -- ConfigSource -->
<!-- source: internal/component/web/fleetui/source_views.go -- DeviceViewSource -->
<!-- source: internal/component/web/fleetui/sample/source.go -- estate, newEstate, Stepper -->
<!-- source: internal/component/web/fleetui/sample/register.go -- init -->

### The sample source

The sample source answers every method of `Source` from a fixed estate of 44
devices, so every page can be opened and judged. `newEstate` makes it, and
its register file makes it the build's source.

| Part | Rule |
|------|------|
| Tables | The estate is written as tables, one file per concern: the devices and their faults, the alarms and the tasks, the rollouts and the audit trail, the configuration texts and the administration answers, what a device answers through the relay, the console's commands |
| Clock | Every time in a table is an age, measured back from the moment the source was made, so "last seen" and a chart range read the same whenever the build is started |
| Alarms | Twelve open: four critical (one component down, three devices offline), five warnings, three informational. The sample hub opens the offline alarm five minutes after a device was last seen (its `Settings`) |
| History | Generated per request from a fixed seed, so every start draws the same charts. It goes back `historyRetention` (one week), and one series holds at most `historyPointsMax` points, at an interval of 30 seconds, 5 minutes or 30 minutes chosen by the period asked |
| Timed script | The first subscription starts a script that applies one step every five seconds (`scriptPeriod`): a device goes offline and comes back, with its offline alarm; a memory alarm opens and clears; a task records its results, ends, and a new one starts. Each step publishes the event of what it changed |
| Stepped script | `Stepper` (`Step`, `Hold`) takes the script from the timer for good: a step is applied only when asked, which the prototype-only routes and a browser test do |

The only state that changes is the script's, and every list it grows is cut
to a bound, so the source stays the same size however long the build runs.
<!-- source: internal/component/web/fleetui/sample/source.go -- estate, newEstate, Stepper -->
<!-- source: internal/component/web/fleetui/sample/history.go -- historyRetention, historyPointsMax -->
<!-- source: internal/component/web/fleetui/sample/script.go -- scriptPeriod, scriptSteps -->
<!-- source: internal/component/web/fleetui/sample/activity.go -- alarmRows, alarmsAt -->
<!-- source: internal/component/web/fleetui/sample/config.go -- offlineAlarmDelay, settings -->

### A source answers facts, the package derives

A source answers facts only: devices with their labels, connection,
configuration state, health components, inventory and last sample; samples;
alarms; tasks; rollouts; audit entries; configuration text; what a device
answers. Everything a page shows beyond a fact is computed in the fleet
package by pure functions, so the real source has only facts to supply, and a
device is in the same state in the tree, in every table, on every card and in
its own header.

| Derived | Function | From |
|---------|----------|------|
| The display state | `DeviceDisplayState` | Connection, configuration state, health components. Precedence: offline, diverged, down, degraded, then healthy or unknown |
| One view per device | `DeriveDevices` | Devices, open alarms, tasks and the hub's clock: the state, the open alarm count, the active task count, the uptime, how long ago the device was seen |
| The tree | `BuildTree`, `Tree.Find`, `GroupDevices`, `FilterTree` | The views and a tree view |
| A page of the devices table | `QueryDevices` | The views, a filter, a sort and a page. It is the one function a handler calls, so a hub that filters, sorts and pages on its side replaces it alone |
| Counts and lines of a dashboard | `CountByState`, `CountByConfig`, `CountByVersion`, `Figures`, `NeedsAttention`, `Busiest`, `TopAlarms` | The views, the alarms, the rollouts |
| A series as wide as its chart | `Reduce` | A series, a period and a point budget. A period with no sample stays a gap, and each point keeps the average and the maximum |

No function reads the wall clock. The source answers the hub's clock (`Now`),
and every age and chart period is computed against it.

A count on a card and the rows behind its link cannot differ: a count is taken
on the value the filter field reads, and each device line of the Needs
attention card is counted by applying the filter its link carries.
<!-- source: internal/component/web/fleetui/state.go -- DeviceDisplayState, DisplayState.Severity -->
<!-- source: internal/component/web/fleetui/derive.go -- DeriveDevices, DeviceView -->
<!-- source: internal/component/web/fleetui/table.go -- QueryDevices, Paginate, ColumnSet.Sort -->
<!-- source: internal/component/web/fleetui/summary.go -- NeedsAttention, Reduce, CountByVersion -->

### Live events

`Subscribe` answers a `Subscription`, which only a `Feed` makes. A source
holds one `Feed` and publishes typed events into it: a device changed, an
alarm opened or cleared, a task was created, progressed or ended, the registry
changed. An event names what changed and carries no data, so a page fetches
again what the event made stale.

| Rule | Detail |
|------|--------|
| Goroutines | None. `Publish` runs on the publisher's goroutine and never blocks |
| Buffer | `EventBufferSize` events per subscription. A subscription with no room left is ended and marked as having fallen behind, so the page fetches every live fragment again when it subscribes anew |
| Limit | `EventSubscriptionsMax` subscriptions per feed. One more is refused with `ErrStreamsFull`, and the page works without live updates |
| Ownership | The feed creates each subscription's channel and is the only one that closes it, once: when the subscriber calls `Close`, when it falls behind, or when the feed is closed. A subscriber calls `Close` when it stops reading; a source that stops closes its feed |
<!-- source: internal/component/web/fleetui/source_events.go -- Feed, Subscription, EventKind -->

### Failures

A source that cannot answer returns an operator error. Where a page shows a
state for the reason (not found, device offline, relay refused, not supported
by the device, no answer in time, unauthorized), the operator error wraps the
matching error of `source_errors.go`, and `FailureOf` reads the state back.
Every question of `DeviceViewSource` can fail on its own, so a page shows the
state in that section and keeps its others.
<!-- source: internal/component/web/fleetui/source_errors.go -- FailureOf, ErrDeviceOffline, DeviceUnknownError -->

## Addresses, filters, trees and tables

| Part | Rule |
|------|------|
| Address | `ParseAddress` reads a page address into an `Address` (kind of object, group path, device, tab, view) and `Address.String` renders it. `/fleet/` is the estate dashboard. A group segment is `key=value`, each escaped once, and a segment with no `=` is the node of the devices that lack the key. An address that names no page wraps `ErrAddressUnknown` |
| Fragment | `/fleet/fragment/<name>/<page path>`, read by `ParseFragment`. It carries the page it belongs to with its view, and `item` for a fragment a page holds several of or that takes one argument. "Fragments" lists the ones the shell registers |
| Tabs | A registered set per kind of object (`MustRegisterTab`, `Tabs`). The parser accepts only a registered tab, so a page family adds a tab from its own register file |
| View parameters | `q`, `sort`, `page`, `size`, `id`, `range`, `view`, `sub`. `sub` names a sub-view of the page, which the page judges: a device tab's sub-tab, the opened divergence diff (`sub=divergence`), the new-rollout form (`sub=new`). A page size other than 25, 50 or 100 is 50; a page below one is one; a chart range other than `hour`, `day` or `week` is `day`; an absent tree view is `site-role`. A default is left out of a rendered address |
| Filter | `FilterSchema` holds the grammar of one table: bare word, `field:value`, `field:a,b`, `field:!value`, `field:>n`, `field:<n`, `label:key=value`, a label key in use as a field, `has:<flag>`, terms ANDed. Fields and flags are registered. A filter is at most 512 characters and 32 terms. A refusal wraps `ErrFilterRefused`, names the term and lists the fields. `SetFilterTerm` writes the choice of an assisted menu into the filter text |
| Tree | A tree view is an ordered list of level keys. A key is a label key, unless a tree level is registered under it (`version`, `state`, `group`), which reads a field of the device. Built-in views: `site-role`, `role`, `version`, `state`. At most eight levels |
| Sort | `ColumnSet` holds the sortable columns of one table, registered. The sort is stable; `sort=-column` is descending; an unknown column wraps `ErrSortRefused`. The state column sorts by severity: down, offline, diverged, degraded, unknown, healthy |
| Page | `Paginate` clamps the page to the last one and answers the figures "first to last of total" |

The task state the fleet specs spell "Cancelled" is `Canceled` in the code and
on the page, because the repository uses US spelling.
<!-- source: internal/component/web/fleetui/url.go -- ParseAddress, Address.String, ParseFragment, ParseViewParams -->
<!-- source: internal/component/web/fleetui/tabs.go -- MustRegisterTab, Tabs -->
<!-- source: internal/component/web/fleetui/filter.go -- FilterSchema.Parse, SetFilterTerm -->
<!-- source: internal/component/web/fleetui/tree.go -- BuildTree, ResolveTreeView, MustRegisterTreeLevel -->
<!-- source: internal/component/web/fleetui/register_devices.go -- registerDeviceFields, registerDeviceColumns, registerDeviceLevels -->

## Components

Every page is assembled from one set of components. Each is a templ component
taking one named view struct, which a page handler fills from the data
layer's answers: words, numbers, addresses and small enums. No component reads
a source, and no Go file builds markup.

| Family | Components | View data |
|--------|------------|-----------|
| Marks | `Icon`, `StatusMark`, `StatusWord`, `StateMark`, `StateWord`, `ObjectIcon`, `Illustration` | `Status`, `Tone`, `ObjectIconView` |
| Actions | `NavLink`, `ActionLink`, `ActionButton`, `PrimaryButton`, `IconButton`, `Menu`, `DotsMenu`, `Tag`, `CopyButton` | `ActionView`, `MenuView`, `ConfirmView`, `TagView`, `CopyView` |
| Cards | `CardGrid`, `Card`, `Figure`, `FigureList`, `FiguresCard`, `ChartCard`, `QuickInfo`, `Callout` | `CardView`, `FigureView`, `ChartCardView`, `QuickInfoView`, `CalloutView` |
| Counts | `Donut`, `DonutCard`, `StackedBar`, `UsageBar`, `UsageMini`, `UsageGroup` | `DonutView`, `DonutSegmentView`, `UsageBarView`, `UsageGroupView` |
| Tables | `TableSection`, `QueryRow`, `TableActions`, `SelectionRow`, `Table`, `Cell`, `Pager` | `TableSectionView`, `QueryRowView`, `BulkView`, `TableView`, `RowView`, `CellView`, `PagerView` |
| Side panel | `PanelContent`, `KeyValueList` | `PanelView`, `PanelCardView`, `ValueView` |
| Pane states | `StateHero`, `Skeleton`, `StaleBanner`, `GoodNews` | `StateHeroView`, `HeroKind`, `SkeletonView` |
| Overlays | `ToastArea`, `Toast`, `ToastOOB`, `ConfirmPreview`, `TaskPanel`, `TaskList`, `TaskRow`, `AlarmList` | `ToastView`, `TaskPanelView`, `TaskRowView`, `AlarmListView` |
| Console and diff | `Terminal`, `TerminalEntry`, `Diff` | `TerminalView`, `TerminalEntryView`, `DiffView` |
| Gallery | `ComponentGallery` | `GalleryView`, from `NewGalleryView` |

| Rule | How a component keeps it |
|------|--------------------------|
| A state is never colour alone | A `Status` is a `Tone` and a word. The tone chooses one round mark, drawn beside the word. `DisplayStatus`, `HealthStatus`, `ConfigStatus`, `AlarmStatus` and `TaskStatus` answer the status of each state |
| No style attribute | A donut is SVG whose arcs `DonutArcs` computes as dash lengths. A bar's width is one of the classes `fleet-w-0` to `fleet-w-100`, which `StackedShares` and `Percent` feed |
| An unknown value is not zero | A usage bar whose figure was not reported shows a dash and no fill. An absent text is a dash |
| Every view has an address | A link is a `NavLink`: a real link whose address htmx loads into `#fleet-content` and pushes. A sort, a page, a filter and an opened row are addresses the page handler renders with `ViewParams.Query` |
| Behavior is attached by data attributes | `fleet-components.js` delegates from the document: row selection with shift ranges, the bulk count, menus, copy controls, the page size select, toasts and the confirm dialog |
| A destructive action is confirmed | A control carrying a `ConfirmView` opens the one confirm dialog that `ToastArea` draws. A destructive action is confirmed by typing the target's name or the count |
| A toast arrives with an answer | A handler adds `ToastOOB` to an htmx answer. Informational and success toasts leave after six seconds; warnings and errors stay. Four show, the rest are counted |
| A chart reads its colours when drawn | `fleet-chart.js` fetches the JSON its card names, whose shape the head of that file states, and draws it with uPlot. A null value is a gap. The shell redraws every chart by dispatching the document event `fleet:theme` |
| The console appends fragments | `fleet-terminal.js` posts a command and appends the `TerminalEntry` fragment the handler answers. A refusal closes the input |

The gallery draws every component in every state from fixed data. It is the
states page of the prototype, and `TestComponentGalleryDrawsEveryComponent`
looks in it for the root class of each component.
<!-- source: internal/component/web/fleetui/component_view.go -- Tone, Status, DisplayStatus, ActionView, Percent, RelativeTime -->
<!-- source: internal/component/web/fleetui/icons.templ -- Icon, IconNames, StatusWord, ObjectIcon -->
<!-- source: internal/component/web/fleetui/component_action.templ -- NavLink, actionControl, Menu -->
<!-- source: internal/component/web/fleetui/component_donut.go -- DonutArcs, StackedShares, UsageTone -->
<!-- source: internal/component/web/fleetui/component_table.go -- TableSectionView, CellView, PagerRange -->
<!-- source: internal/component/web/fleetui/component_toast.templ -- ToastArea, ToastOOB, confirmDialog -->
<!-- source: internal/component/web/fleetui/component_gallery.go -- NewGalleryView -->
<!-- source: internal/component/web/fleetui/assets/fleet-components.js -- refresh, openConfirm, arrangeToasts -->
<!-- source: internal/component/web/fleetui/assets/fleet-chart.js -- the JSON shape, draw -->
<!-- source: internal/component/web/fleetui/assets/fleet-terminal.js -- run, appendAnswer -->
<!-- source: internal/component/web/fleetui/component_gallery_test.go -- TestComponentGalleryDrawsEveryComponent -->

## The shell and its pages

Every fleet page is the same shell around one page. The shell knows no page:
a page family registers each of its pages, and the shell resolves an address
to the registered one.

| Part | Contract |
|------|----------|
| `MustRegisterPage(Page)` | Called from `init()` in the register file of the family that owns the page. A `Page` names its kind of object and its tab, and holds `Content`, the builder of the tab's content. `SidePanel` builds the details of the row the address names in `id`, and is called only while it names one. `Primary` and `Actions` answer the object header's primary action and the entries of its Actions menu, which the object's tree row shows too. `Static` marks a page whose content holds state the browser owns, such as a terminal: a live event then leaves it alone |
| `PageBuilder` | `func(ctx, *PageRequest) (templ.Component, error)`. It answers a component, or the data layer's operator error |
| `PageRequest` | The address with its view parameters, the `Source`, the account name, and the `Snapshot`: the estate, the hub's clock, one `DeviceView` per device, the open alarms, the tasks and the declared tree views, read once for the request. `NewPageRequest` builds one |
| A tab with no registered page | Renders the "not built yet" state. The shell is complete on its own |
| A builder that fails | The content region shows the state `FailureOf` names, in the operator error's own words: its problem is the sentence, its reason the second sentence, its fix the line the operator can copy. Not found answers 404, unauthorized 403, a failure with no state of its own 500; a state that is an operating condition of a device (offline, relay refused, not supported, no answer in time) answers 200, because the page rendered |
| An address that names nothing | An unknown device, a group no device is in, and an address that names no page each answer 404 with the not-found state inside the shell, so the tree and the header stay. A tree view the hub does not have answers 400 with the list of views, and the tree is drawn under the default view |

The shell reads its view data from named structs (`ShellView`, `HeaderView`,
`SidebarView`, `TreePaneView`, `TreeRowView`, `MainView`, `ObjectHeaderView`,
`SidePanelView`). A view holds words, addresses, counts and small enums only:
the typed-data guard refuses a field whose type another package declares, so a
time is turned into its words before it reaches a view.

A link inside the shell is a real link that htmx follows: it asks for the
page's address, swaps the answer into the content region and pushes the
address. The page handler answers such a request (`HX-Request`, with the
request type `partial`) with the content region, then the regions that follow
the page (the object header with the breadcrumb and the tab row, the tree, the
tree view selector, the administration list, the side panel) as out-of-band
swaps. Any other request, the one that restores a page from history included,
gets the whole shell. Either way the address reproduces the view.
<!-- source: internal/component/web/fleetui/pages.go -- MustRegisterPage, Page, PageRequest, NewPageRequest, ReadSnapshot -->
<!-- source: internal/component/web/fleetui/handler_shell.go -- fleetHandler.ServeHTTP, resolvePage, describeObject, failureStatus, asksPart -->
<!-- source: internal/component/web/fleetui/shell_states.go -- failureHeroView, notBuiltHeroView -->
<!-- source: internal/component/web/fleetui/shell.templ -- Shell, pageUpdate -->

### The tree

The sidebar's tree is built for every page from the request's snapshot, with
`BuildTree`, under the tree view of the address.

| Rule | Detail |
|------|--------|
| Rows | The estate, then one row per group node, then the devices. A group row shows connected over total, in the warning colour when a device below is not healthy; a device row shows its state mark and its open alarms; a spinner replaces the mark while a task targets the row |
| Loading | Rows down to the second level are always drawn. The children of a deeper row are left out and load from the `tree-branch` fragment when the row is first expanded, with skeleton rows meanwhile |
| Selection | The server draws the rows on the way to the selected row expanded, and marks it. The script scrolls it into view |
| Expansion | The rows the operator opened or closed are kept in the browser session, per tree view, and applied again after every load. A row that leads to the selected row stays open |
| Filter | The filter box asks the `tree` fragment for the tree narrowed by its text, a quarter of a second after the last key. A filter draws everything it keeps, expanded. The `/` key focuses the box |
| Keyboard | Up and down move, right expands, left collapses, enter opens |
| Tree view | The selector's entries are the address in hand under each view. The choice is in the address and remembered per browser: an address that names no view opens under the view chosen last |
<!-- source: internal/component/web/fleetui/handler_tree.go -- treeBuild, treeRowsView, treeFragment, treeBranchFragment -->
<!-- source: internal/component/web/fleetui/tree.templ -- treePane, treeRows, treeRow -->

### Fragments

A fragment is the inside of one region, fetched on its own:
`/fleet/fragment/<name>/<page path>`, with the page's view parameters and, for
a fragment that takes one argument, `item`. `MustRegisterFragment(name,
build)` registers one from a register file; the shell registers its own the
same way. A fragment that fails answers the matching state under status 200,
so the region shows it. An `item` longer than a filter may be is refused.

| Fragment | Answers | `item` |
|----------|---------|--------|
| `tree` | The tree | The filter text |
| `tree-branch` | The rows under one tree row | The row's key |
| `object` | The object header, the breadcrumb and the tab row | - |
| `content` | The tab's content | - |
| `panel` | The side panel's content | - |
| `tasks` | The task panel, whole | The tab to show it on; empty for the closed panel |
| `header` | The Tasks button with its count and its failed dot | - |
| `search` | The devices, then the pages, matching typed text, at most ten | The text |
| `dashboard-card` | One card of a dashboard | The card's name |
| `dashboard-chart` | The JSON of one chart card, answered as `application/json` | The chart's key |
| `device-identity`, `device-hardware`, `device-resources`, `device-health`, `device-labels`, `device-management` | One section of a device's System tab | - |
| `device-interfaces`, `device-routing`, `device-logs` | The section of the Interfaces, Routing and Logs tabs | - |
| `device-tool` | The toast of a related tool run from a row: nothing runs | `<tool>/<target>` |
| `config-state`, `config-text`, `config-history`, `config-divergence` | One section of a device's Config tab | - |
| `rollout-preview` | The preview of the new rollout form | The batch size, as a percentage |
| `toast-demo` | One toast of the States page | The level's key |
<!-- source: internal/component/web/fleetui/handler_fragment.go -- registerShellFragments, fleetHandler.serveFragment -->
<!-- source: internal/component/web/fleetui/pages.go -- MustRegisterFragment, FragmentLink -->
<!-- source: internal/component/web/fleetui/handler_search.go -- searchView -->

A fragment answers markup, or JSON when its builder answers `fragmentJSON`.
<!-- source: internal/component/web/fleetui/handler_fragment.go -- fragmentJSON, jsonData -->
<!-- source: internal/component/web/fleetui/handler_shell.go -- renderAnswer -->

### The live event stream

`GET /fleet/events?scope=<page path>` is a server-sent stream for one open
page. It carries no data of the estate. Each event is named `stale` and lists
the regions of the page that are stale, among `tree`, `content`, `tasks` and
`header`, and the script fetches again the fragment of every element marked
with that region (`data-fleet-region`, `data-fleet-name`).

| Rule | Detail |
|------|--------|
| Scope | The content region is stale for a change of a device the page shows: every device for the estate, the task list and the administration; the devices of the node for a group, read again when a device outside it changes, because it may have joined; the device itself for a device. The tree, the task panel and the header show the whole estate on every page, so a device change always names the tree, and a task event names every region |
| Coalescing | Events are collected and written once a second, so a burst is one event per stale region per second |
| Bounds | One subscription per stream, whose buffer the feed bounds (`EventBufferSize`). The feed serves `EventSubscriptionsMax` streams; one more answers 503 with the feed's operator error, and the page works without live updates |
| Ending | The stream ends when the browser leaves, when a write does not complete in ten seconds, and when the feed ends the subscription. In the last case it first names every region, so the page fetches them again and subscribes anew |
| Goroutines | None. The stream runs on the goroutine of its request |
| Loss | The script shows a banner with the time the updates stopped, tries again after one second and then twice as long each time, up to thirty seconds and eight attempts, and fetches every region once when the stream is back |
| Stepping | `POST /fleet/prototype/step` advances a scripted source by one event and answers the kind of the event and the device it is about. `POST /fleet/prototype/hold` stops the source from advancing its script on its own and changes nothing else. Both are mounted only when the registered source implements `Stepper`, which only the sample source does, so a browser test decides when the estate changes |
<!-- source: internal/component/web/fleetui/handler_events.go -- eventStream.ServeHTTP, eventStream.run, eventScope.stale, eventScope.settle, Stepper, newStepHandler, newHoldHandler -->
<!-- source: internal/component/web/fleetui/assets/fleet.js -- openStream, streamFailed, refreshRegion -->

### Scripts and styles of the shell

The content security policy allows no inline script, no `style` attribute and
no event attribute, so behavior reaches the scripts through ids and
`data-fleet-*` attributes, and every listener is delegated from the document.

| File | Holds |
|------|-------|
| `fleet-theme.js` | Applies the theme stored in the browser setting `ze-theme`, which the other web modes write, before the first paint. It is the one script loaded without `defer` |
| `fleet.js` | The shell: the theme switch; the sidebar's collapse, pin, two tabs and width; the tree's expansion, lazy branches, filter and keyboard; the side panel pin; the header search; the event stream client and the region refresh |
| `fleet-shell.css` | The theme variables under the names and with the values the web stylesheet uses, in a dark default and a light theme; the spacing, sidebar and chart variables; the surface variables; the regions, the tree, the object header, the breadcrumb and the tab row; the one responsive form below 60rem |

The sidebar's width is the custom property `--fleet-sidebar-width`, which the
script sets on the root element through the object model, clamped to 12rem to
28rem, by dragging the handle or with the arrow keys on it. Where a browser
refuses that, the script falls back to three widths chosen by a class. The
width, both pins and the collapsed state are browser settings, not part of an
address.

The surface variables carry the relationship between the page and what lies
on it. In the dark theme the page is `--bg-primary`, a card and the sidebar
are `--bg-secondary`, and the header is `--bg-inset`. In the light theme the
page is `--bg-secondary`, a card and the sidebar are `--bg-primary`, and the
header is `--bg-secondary` with a line under it.

Two rules hold for every fleet stylesheet, and `TestFleetStylesUseVariablesOnly`
enforces them over every sheet under `assets/`. A color is written as a
literal only as the value of a custom property, and only in `fleet-shell.css`.
A gap, which is the value of `gap`, `margin` and `padding` in every form, is
`0`, `auto` or one of the six spacing variables. A size is not a gap: widths,
heights, offsets, border widths, radii and font sizes are free.
<!-- source: internal/component/web/fleetui/assets/fleet-theme.js -- the ze-theme read -->
<!-- source: internal/component/web/fleetui/assets/fleet.js -- applyWidth, applyTreeState, start -->
<!-- source: internal/component/web/fleetui/assets/fleet-shell.css -- the :root and [data-theme="light"] blocks -->
<!-- source: internal/component/web/fleetui/style_test.go -- TestFleetStylesUseVariablesOnly, styleFindings -->

## The pages

Five page families register the pages, each from its own register file, with
its own stylesheet. A section of a page that can fail on its own is a fragment
of its own, drawn with the page and fetched again alone.

### Dashboards

A dashboard is an ordered list of registered cards, each with its width
(`dashboardCard`: name, title, span, live, builder, registered with
`mustRegisterDashboardCard`). The estate, a group node and a device each have
one. Every card is drawn in place and is also the `dashboard-card` fragment,
so a card whose source fails shows the failed state with a retry that asks
for that card alone, and every other card renders.

| Rule | Detail |
|------|--------|
| Live cards | A card marked live shows what a device reports now. On the dashboard of an offline device a live card shows the offline state in its place and asks the device nothing |
| Range | The range selector writes `range` (`hour`, `day`, `week`) into the address, and every chart card reads it |
| Charts | A chart card names the `dashboard-chart` fragment, which answers JSON: the series reduced by `Reduce` to at most `dashboardChartBudget` (300) points, each the average and the maximum of its period, `null` for a period with no sample, which the script draws as a gap. A series with no sample answers "No samples have been reported yet" and when one is expected; a source that fails answers an empty chart carrying its refusal |
| Busiest devices | The metric the card ranks by is the `sort` parameter of the dashboard's address, `cpu` by default |
| Links | A count on a card links to the table that lists exactly those rows, with the filter that selects them: the devices table, a device's peers or interfaces by `state:`, the alarms by `device:`, the rollouts by `state:paused` |
<!-- source: internal/component/web/fleetui/handler_dashboard.go -- dashboardCard, mustRegisterDashboardCard, dashboardContent, dashboardView, renderDashboardCard, dashboardOfflineSlot, dashboardCardFragment -->
<!-- source: internal/component/web/fleetui/handler_chart.go -- dashboardCharts, dashboardChartBudget, chartAnswer -->
<!-- source: internal/component/web/fleetui/handler_dashboard_cards.go -- busiestCardView, alarmsCardView -->
<!-- source: internal/component/web/fleetui/links_test.go -- TestDashboardLinkCountsEqualTargetRows -->

### The devices table

The Devices tab of the estate and of a group node is one table over the
devices of the scope, queried by `QueryDevices` through `queryDevicesTable`.
A filter the schema refuses is shown with the refusal, naming the term and
the fields, over the unfiltered table. Rows are selected for the bulk actions,
which post to the action registry. The side panel shows the identity, labels,
connection, health, the last sample and the open alarms of the opened row.
<!-- source: internal/component/web/fleetui/handler_devices.go -- devicesContent, queryDevicesTable, devicesPanel -->
<!-- source: internal/component/web/fleetui/handler_devices_action.go -- devicesActionAnswer -->

### Device tabs

A device has the tabs Dashboard, Console, System, Interfaces, Routing, Config,
Logs and Tasks. A tab with sub-tabs names the one shown in `sub`
(`subTabsView`). Each section is a fragment, and a section that shows what the
device reports now refreshes itself on a timer. The Console posts each
command to `POST /fleet/console/`, under the mutation wrap, and appends the
`TerminalEntry` it answers; `ConsoleSuggestions` lists the commands it offers.
An interface row has no traffic chart, because `DeviceSeries` takes no
interface, and a BGP peer's panel shows its session and prefixes only.
<!-- source: internal/component/web/fleetui/register_device_pages.go -- consolePattern, subTabsView -->
<!-- source: internal/component/web/fleetui/handler_device_console.go -- consolePath, newConsoleHandler, ConsoleSuggestions -->

### Activity

The Tasks tab of the estate, a group node and a device, and the task list,
list the tasks with their state, progress and duration; a row expands to its
per-device results (`activityTaskRow`), which the header's task panel shows
too. The Alarms tab of the estate and of a group node lists the open alarms,
the most severe first and the oldest first within a severity.

The alarm side panel shows the alarm, its device's alarms of the week, and a
link to the device tab where its cause is examined. That tab comes from the
alarm-kind registry: `mustRegisterAlarmTab(kind, tab)` maps an alarm kind to
a device tab, and a kind with no entry links the device's dashboard.

The States page (`/fleet/admin/states/`) belongs to the prototype: it shows
every toast level through the `toast-demo` fragment, and, over a source that
is a `Stepper`, the controls that step and hold the sample script. A step
answers the event's line and a toast naming it.
<!-- source: internal/component/web/fleetui/handler_tasks.go -- tasksContent, activityTaskRow, taskActionAnswer -->
<!-- source: internal/component/web/fleetui/handler_alarms.go -- mustRegisterAlarmTab, alarmTab, scopedAlarms, alarmSidePanel -->
<!-- source: internal/component/web/fleetui/register_activity_pages.go -- registerAlarmTabs -->
<!-- source: internal/component/web/fleetui/handler_prototype.go -- statesContent, toastDemoFragment -->
<!-- source: internal/component/web/fleetui/handler_events.go -- scriptAnswer -->

### Manage

| Page | Shows |
|------|-------|
| A device's Config tab | The configuration state, the text the hub holds, its versions, and a divergence with the diff opened by `sub=divergence`. Edit answers the prototype toast |
| Rollouts | The table of rollouts, one rollout's page at `id=<name>`, and the new-rollout form at `sub=new`, whose preview is the `rollout-preview` fragment |
| Audit | The audit trail, filtered by actor, event, device, result and time, and exported by `GET /fleet/export/audit` as CSV or JSON, at most `AuditEntriesMax` entries |
| Administration | Devices (with the join panel), labels, tree views, saved filters, templates, access, audit, history, settings, and the prototype's States page last |
<!-- source: internal/component/web/fleetui/handler_config.go -- configContent, divergenceFragment, manageActionAnswer -->
<!-- source: internal/component/web/fleetui/handler_rollouts.go -- rolloutsContent, rolloutPreviewFragment -->
<!-- source: internal/component/web/fleetui/handler_audit.go -- auditContent, AuditExportPath, newAuditExportHandler -->
<!-- source: internal/component/web/fleetui/register_tabs.go -- init -->

### Actions

Nothing in the prototype changes anything. Every action that would change the
estate posts to `/fleet/action/<name>/<page path>` (`ActionAddress`,
`ParseAction`). `MustRegisterAction(name, build)` registers the builder of one
action, which answers the toast saying the prototype changed nothing, naming
the action and its targets, or a toast saying why the action is refused. The
handler reads a form of at most `actionBytesMax` bytes, and answers 404 for an
action or a page it does not know. The registered actions are `devices`, `task`
and `manage`. A destructive action is confirmed first, by typing the name or
the count.
<!-- source: internal/component/web/fleetui/handler_action.go -- MustRegisterAction, newActionHandler, actionBytesMax -->
<!-- source: internal/component/web/fleetui/url.go -- ActionAddress, ParseAction -->

## Routes

The package registers its routes with the web route registry from `init()`,
and the hub mounts them when the web server starts.

| Pattern | Serves |
|---------|--------|
| `GET /fleet/` | Every page address under the prefix |
| `GET /fleet/fragment/` | The fragments |
| `GET /fleet/events` | The live event stream |
| `GET /fleet/assets/` | The embedded styles and scripts, with `Cache-Control: no-cache` |
| `POST /fleet/action/` | The action registry, under the mutation wrap |
| `POST /fleet/console/` | One Console command of a device, under the mutation wrap |
| `GET /fleet/export/audit` | The audit export |
| `POST /fleet/prototype/step`, `POST /fleet/prototype/hold` | The two prototype-only routes, under the mutation wrap |

The `GET` routes use the authenticated wrap and carry the same enabling
function, which answers true only when a data source is registered. That of
the prototype-only routes answers true only when that source can be stepped. The hub calls each
once, at web server start, after every `init()` has run, so registration order
does not matter. A route whose enabling function answers false is not mounted,
and its portal entry is not registered.

The portal entry, `Fleet`, rides the shell route and opens `/fleet/` directly,
because the interface has its own shell and the server forbids framing a page.

So a build that links the package and no source serves no fleet route and
lists no fleet entry. That is every shipped build today.
<!-- source: internal/component/web/fleetui/register.go -- init, sourceRegistered, stepperRegistered, buildShellHandler, buildActionHandler, buildAuditExportHandler -->
<!-- source: cmd/ze/hub/service_web.go -- startWebServer, the RegisteredWebRoutes loop -->
<!-- source: internal/component/web/fleetui/register_test.go -- TestFleetRoutesAbsentWithoutSource -->

## The two build tags

| Tag | In `feature-gates.txt` | Links | Set by |
|-----|------------------------|-------|--------|
| `ze_fleet_ui` | Yes | `internal/component/web/fleetui`, through `cmd/ze/hub/register_fleet_ui.go` (`//go:build ze_web && ze_fleet_ui`) | Every standard build, like every gate |
| `ze_fleet_ui_sample` | No | `internal/component/web/fleetui/sample`, through `cmd/ze/hub/register_fleet_ui_sample.go` (`//go:build ze_web && ze_fleet_ui && ze_fleet_ui_sample`) | A review build only |

`ze_fleet_ui` is a dependent gate: the package nests under the web package, so
it exists only in a `ze_web` build. Neither package carries a source build
tag. The tagged blank import in the hub is the gate, and
`docs/architecture/plugin/feature-gates.md`, "Dependent gates and dependent
files", states why.

The sample tag is outside the manifest because every manifest tag is set by
every shipped build, and sample data in a shipped build would be shown as a
real estate. `TestShippedZeLinksNoFleetSample` lists the dependencies of every
shipped flavor and refuses the sample package.
<!-- source: feature-gates.txt -- the ze_fleet_ui row -->
<!-- source: cmd/ze/hub/register_fleet_ui.go -- the tagged blank import -->
<!-- source: cmd/ze/hub/register_fleet_ui_sample.go -- the tagged blank import -->
<!-- source: cmd/ze/fleet_sample_absent_test.go -- TestShippedZeLinksNoFleetSample -->

## Making and starting a review build

A review build is the distro daemon with the sample tag added, built into a
scratch directory. It never replaces `bin/ze`.

```
scratch=$(./le session scratch ensure)
tags="ze_core ze_distro $(awk '$1 ~ /^ze_/ {print $1}' feature-gates.txt | sort -u | tr '\n' ' ')ze_fleet_ui_sample"
./le job run label fleet-review-build command go build -tags "$tags" -o "$scratch/ze-fleet-review" ./cmd/ze
```

The tag list is the recipe of `docs/contributing/running-commands.md`, "A bare
`go test` is not `./le test unit`", with `ze_distro` and the sample tag added.

Start it web-only and without login, with its configuration directory in
scratch:

```
ZE_CONFIG_DIR="$scratch/fleet-review-config" "$scratch/ze-fleet-review" start --web <port> --web-only --insecure-web
```

| Option | Effect |
|--------|--------|
| `--web-only` | Starts the web interface alone, with no daemon. It still mounts every registered web route |
| `--insecure-web` | Binds to `127.0.0.1` only and replaces the login with a fixed account named `insecure`, so the authenticated wrap lets every request through |
| `ZE_CONFIG_DIR` | The `ze.config.dir` setting. The directory must hold a store, which `ze init` creates: the web server does not start without one |

The interface is then at `/fleet/` on `127.0.0.1:<port>`. The browser test
harness starts its web server the same way, and seeds the store itself.
<!-- source: cmd/ze/ze_core_start.go -- the --web, --web-only and --insecure-web handling -->
<!-- source: cmd/ze/hub/service_web.go -- runWebOnly, startWebServer -->
<!-- source: internal/component/web/auth.go -- InsecureMiddleware -->
<!-- source: internal/test/cli/cmd_web.go -- zeTestStartWebServer, zeTestSeedWebUsers -->
