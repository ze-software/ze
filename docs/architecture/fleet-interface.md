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
| `register.go` | The route registrations and the function that enables them |
| `register_tabs.go`, `register_devices.go` | The tabs, and the filter fields, sort columns and tree levels of the devices table |
| `handler_shell.go` | The estate page handler, its view type, and the assets handler |
| `shell.templ` | The shell: header, sidebar, main pane and side panel region |
| `assets/` | `fleet-shell.css` and `fleet.js`, embedded in the package |
| `guard_test.go` | The markup guards, run over this package |

The package embeds its own assets, so a build without the package carries none
of them. The stylesheet declares the theme variables under the names and with
the values the web stylesheet uses, in a dark default and a light theme, and
adds the spacing, sidebar and chart variables. The script applies the theme
stored in the browser setting `ze-theme`, which the other web modes write, so
the choice carries across modes.
<!-- source: internal/component/web/fleetui/handler_shell.go -- assetsFS, ShellView, newShellHandler, newAssetHandler -->
<!-- source: internal/component/web/fleetui/assets/fleet-shell.css -- the :root and [data-theme="light"] blocks -->
<!-- source: internal/component/web/fleetui/assets/fleet.js -- the ze-theme read -->

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
<!-- source: internal/component/web/fleetui/sample/source.go -- estate, devices, notInSample -->
<!-- source: internal/component/web/fleetui/sample/register.go -- init -->

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
| Fragment | `/fleet/fragment/<name>/<page path>`, read by `ParseFragment`. It carries the page it belongs to with its view, and `item` for a fragment a page holds several of |
| Tabs | A registered set per kind of object (`MustRegisterTab`, `Tabs`). The parser accepts only a registered tab, so a page family adds a tab from its own register file |
| View parameters | `q`, `sort`, `page`, `size`, `id`, `range`, `view`. A page size other than 25, 50 or 100 is 50; a page below one is one; a chart range other than `hour`, `day` or `week` is `day`; an absent tree view is `site-role`. A default is left out of a rendered address |
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

## Routes

The package registers its routes with the web route registry from `init()`,
and the hub mounts them when the web server starts.

| Pattern | Serves |
|---------|--------|
| `GET /fleet/` | The whole `/fleet/` subtree. `/fleet/` itself is the estate page; any other address under it answers 404 until its page exists |
| `GET /fleet/assets/` | The embedded styles and scripts, with `Cache-Control: no-cache` |

Both routes use the authenticated wrap. Both carry the same enabling function,
which answers true only when a data source is registered. The hub calls it
once, at web server start, after every `init()` has run, so registration order
does not matter. A route whose enabling function answers false is not mounted,
and its portal entry is not registered.

The portal entry, `Fleet`, rides the shell route and opens `/fleet/` directly,
because the interface has its own shell and the server forbids framing a page.

So a build that links the package and no source serves no fleet route and
lists no fleet entry. That is every shipped build today.
<!-- source: internal/component/web/fleetui/register.go -- init, sourceRegistered, buildShellHandler -->
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
