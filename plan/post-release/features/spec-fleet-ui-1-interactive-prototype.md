# Spec: fleet-ui-1 -- Interactive Prototype: Shell, Components and Every Page on Sample Data

| Field | Value |
|-------|-------|
| Status | done |
| Scope | web, fleet |
| Priority | P3 |
| Release reason | Adds a capability absent from the supported release; existing supported behavior and its obligations remain required. |
| RFC gaps | - |
| Depends | - |
| Phase | 6/6 |
| Handoff | - |
| Updated | 2026-11-19 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

**Re-read after compaction:** this file, then `spec-fleet-ui-0-umbrella.md`
(the shell, URL scheme, tokens, appearance table, component set and states this
spec builds), then the page descriptions in `spec-fleet-ui-2` to `spec-fleet-ui-6`.

## Task

Build the whole fleet interface as something the owner can open in a browser,
click through and correct, before any of the work that makes it act on a real
estate. The owner asked for this on 2026-11-18: "before you go deep in
development you provide me a mildly interactive interface I can give you
feedback on which you can later make functional".

The prototype is not a mock-up and is not thrown away. It is the real Fleet
interface: the real routes, shell, components, styles, htmx fragments and event
stream, exactly as `spec-fleet-ui-0-umbrella.md` fixes them. Only its data is
not real. The pages read through one data source, and in the prototype a sample
source answers from recorded answers. Later children replace the sample answers
with the real commands, page by page, and keep the markup.

This spec owns four things:

1. The fleet package, its compile gate and its routes.
2. Every component in the umbrella's component set.
3. Every page of the set, as described in children 2 to 6, rendered on sample data.
4. The owner's review, and the corrections that follow from it.

### Decisions agreed with the owner (2026-11-18)

| Topic | Decision |
|-------|----------|
| Order | Every page is built before the review. One review covers the whole interface |
| Viewing | The implementing session builds a separate binary carrying the sample data into a scratch directory, starts it on the owner's machine bound to the loopback address with no login, gives him the address, and stops it when the review ends. The normal `bin/ze` is not touched |
| Compile tag | `ze_fleet_ui`, the owner's request, name left to the implementer |

### What the owner must be able to do in the prototype

| Area | Interaction |
|------|-------------|
| Shell | Switch between the light and dark themes; collapse, resize and pin the sidebar; switch between the Estate and Administration sidebar tabs; use the header search to jump to a device or a page |
| Tree | Expand and collapse branches; type in the filter box and see the tree narrow; change the tree view (by site then role, by role, by software version, by state); select the estate, a group or a device; see the selected node stay marked and in view after a reload |
| Object pages | Open every tab of the estate, of a group and of a device; follow the breadcrumb upward |
| Tables | Sort by a column; type a filter and use the assisted filter menus; change the page size and move between pages; select rows one by one, by range and by page; see the bulk bar offer actions for the selection; open and close the side panel for a row; pin and unpin the side panel |
| Dashboards | Read every card; follow each card's link to the table it counts; change the chart range between one hour, one day and one week |
| Device | Open an offline device and see the offline state; open a diverged device and see the divergence banner and its diff; open the Console tab and run the sample commands the prototype answers |
| Tasks and alarms | Open the task panel; watch a scripted task progress across several devices and finish with one failure; open its per-device results; see an alarm open and clear |
| Live updates | Watch a sample device go offline and return without reloading the page, in the tree, in the table and on the dashboard at once |
| Actions | Open the action menu of a device and of a selection; open the confirm dialog of a routine action and of a destructive one; confirm, and receive the toast saying the prototype changed nothing |
| States | Reach every state in the umbrella's states table through a labelled entry on a prototype-only page listing them, so each can be judged without having to provoke it |

### The sample estate

| Property | Value |
|----------|-------|
| Devices | 44, over four sites (`lon`, `ams`, `fra`, `par`) and three devices with no `site` label |
| Roles | edge, core, route server, customer premises |
| States | 35 healthy, 3 degraded, 1 down, 3 offline (one of them for six days), 1 diverged, 1 unknown |
| Software | Three versions in use, one device two versions behind |
| Configuration | 39 in sync, 2 pending, 1 rejected, 1 diverged, 1 never fetched |
| History | One week of samples per device, generated from a fixed seed so every start draws the same charts: a daily pattern, one traffic spike, one gap where a device was offline |
| Alarms | 12 open: 4 critical (one component down, three devices offline), 5 warning, 3 informational. The sample hub opens the offline alarm five minutes after a device was last seen. Owner decision, 2026-11-19: an offline device raises a critical alarm, replacing the earlier table of 9 open alarms with one critical |
| Tasks | 2 running, 1 failed, 12 finished in the last day |
| Rollouts | 1 paused at the second batch after a rejection, 1 complete |
| Labels | `site`, `role`, `model`, `customer` |
| Clock | Every time in the sample estate is relative to the moment the binary started, so "last seen" and chart ranges always read sensibly |

The sample answers are the proposed answer shapes of the real commands. They
are handed to the `fleet` children as input to their design
(`spec-fleet-ui-0-umbrella.md`, risk R-8).

### Package layout and the two tags

| Item | Decision |
|------|----------|
| Package | `internal/component/web/fleetui`. Nested under the web package, so the gate tooling derives its dependency on `ze_web` from the path, as it does for packages nested under BGP |
| Gate | `ze_fleet_ui`, one line in `feature-gates.txt` naming the package. Like every gate it is on in the standard builds and removable by building without it |
| Reaching the package | A hand-written `cmd/ze/hub/register_fleet_ui.go` under the compound build constraint, holding the blank import only, following `cmd/ze/hub/register_ike.go` |
| Sample package | `internal/component/web/fleetui/sample`, under a third tag, `ze_fleet_ui_sample`, that is NOT in `feature-gates.txt` (every tag in that file is set by every shipped build). It is reached by `cmd/ze/hub/register_fleet_ui_sample.go` under that tag, following the `zetest` precedent in `cmd/ze/plugins_zetest.go` |
| The seam | The fleet package declares one data source interface: the questions its pages ask (devices, tree, summaries, samples, alarms, tasks, rollouts, audit, device views, console). It holds a registry slot for one source. The sample package registers a source from `init()`. Later children register the real one, built over the command dispatcher |
| Enabling | The fleet routes are mounted only when a source is registered. In this spec that means only in a build carrying the sample tag. A standard build carries the fleet package dormant: no route, no portal entry |
| Entry | The fleet interface is reached at `/fleet/` and through a portal entry registered with its route. Making Fleet a selectable web mode in the existing shells is `fleet-ui-2`, because it changes their markup and fixtures |
| Assets | The fleet package embeds its own styles and scripts and serves them at `/fleet/assets/`, so a build without the tag carries none. It links the vendored htmx and uPlot the web package already serves at `/assets/` |
| Components | Built in the fleet package. The web package's table, panel and terminal components are unexported and cannot express an icon, a link, a checkbox or a sort control in a cell, so they are not reused here. A component moves to the web package when the workbench wants it |
| Console | A simple terminal component with its own small script, answering a fixed list of sample commands. Rebinding the real web command line to a device is `fleet-ui-4` |
| Diff | A read-only two-column diff component, new. The existing diff modal is the commit review and is bound to the commit routes |
| Theme | The fleet shell script reads and writes the same browser setting the existing shells use (`ze-theme`), so the choice carries across modes |

## Required Reading

### Architecture Docs
- [ ] `plan/post-release/features/spec-fleet-ui-0-umbrella.md` - the contract
  → Constraint: tokens, appearance table, component set, URL scheme and states are closed lists; a page uses only them
  → Constraint: no literal colour and no spacing outside the six spacing variables in fleet styles
- [ ] `docs/architecture/plugin/feature-gates.md` - how a feature is compiled out
  → Constraint: every non-test file that imports a gated package carries a build line naming its tag; a generated templ file included
  → Constraint: a present test and an absent test are owed for a new gate, under `cmd/ze/hub/`
- [ ] `docs/architecture/web-interface.md` and `docs/architecture/web-components.md` - routes, headers, markup guards
  → Constraint: `script-src 'self'` and `style-src 'self'`: no inline script, no `style` attribute, no `on*` attribute, no `hx-on`
  → Constraint: a component takes one named view struct, never a map; no Go file builds markup
  → Constraint: the markup guards are per-package tests, so the fleet package carries its own copies
- [ ] `docs/contributing/ze-go-style.md` - read in full before the first Go edit
  → Constraint: no `fmt.Sprintf` family, use `textbuf.Buffer`; no `go func(`; `init()` only in `register*.go`; every file opens with a `// Design:` line; every bound stated
- [ ] `docs/contributing/testing.md`, the templ generation command
  → Constraint: generated templ output is committed and produced only by the documented `./le job run` command, never a bare generator run
- [ ] `docs/architecture/testing/runner-architecture.md`, "Web test format"
  → Constraint: no blind wait in a browser test; assert on a positive expectation

**Key insights:**
- No package outside the web package registers a web route today. The fleet package is the first, and it needs a tier manifest row and a hand-written hub import.
- `WebRoute.Enabled` is evaluated once, when the web server starts.
- The web server started with the web-only and insecure options binds to the loopback address, needs no login and runs no daemon. That is the form the owner reviews.
- htmx here is version 4: inheritance of `hx-target` and `hx-swap` is explicit, events are named with colons, and the web package uses its own event client rather than the htmx extension.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/web/webroute.go` - the route registry; wrap kinds are authenticated read and authenticated mutation
- [ ] `internal/component/web/ui_mode.go` - a two-value mode switch; unknown values select the YANG editor
- [ ] `cmd/ze/hub/service_web.go` - mounts registered routes once at server start, skipping those whose enabling function answers false
- [ ] `cmd/ze/hub/register_ike.go` - a hand-written, tagged file holding blank imports only
- [ ] `feature-gates.txt` - one line per gated package; no dependency column
- [ ] `internal/component/web/render.go` - embeds the web assets directory whole and serves it at `/assets/`
- [ ] `internal/component/web/assets/style.css` - the theme variables in both themes
- [ ] `internal/component/web/assets/l2tp-chart.js` - draws with uPlot, reading colours from custom properties
- [ ] `internal/component/web/handler_golden_input_test.go` - a test dispatcher answering fixed JSON per command, the model for the sample source

**Behavior to preserve:**
- Every existing route, page, asset and fixture of the web package, unchanged
- Every existing build flavour compiles, with and without the new tag
- A standard build mounts no fleet route

**Behavior to change:**
- A build carrying `ze_fleet_ui_sample` serves the fleet interface at `/fleet/` on the sample estate

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Web route registry | `internal/component/web/webroute.go`, `RegisterWebRoute` | Registers a route with its wrap kind, builder, enabling function and portal entry | Extends: the fleet package registers every route through it, enabled when a data source is registered |
| Portal entry | `internal/component/web/webroute.go`, `WebRoute.Portal` | Adds a feature to the portal menu when its route is mounted | Extends: the fleet interface is listed there |
| Feature gates | `feature-gates.txt` and `docs/architecture/plugin/feature-gates.md` | Declares each compiled-out package and derives the tag lists | Extends: one line for the fleet package |
| Tagged hub import | `cmd/ze/hub/register_ike.go` | Links a gated package the plugin generator does not discover | Extends: the same shape links the fleet package and, under its own tag, the sample package |
| Tag outside the manifest | `cmd/ze/plugins_zetest.go`, the `zetest` tag | Links test-only code that no shipped build carries | Extends: `ze_fleet_ui_sample` follows it |
| Proof of absence from shipped builds | `cmd/ze/ze_le_personality_test.go`, `TestNormalZeLinksNoInternalLe` | Lists the dependencies of every shipped flavour and refuses a development package | Extends: a sibling test refuses the sample package |
| Theme variables | `internal/component/web/assets/style.css` | Colours, fonts and radius in two themes | Extends: the fleet styles declare the same variable names with the same values and add the spacing and chart variables; the web stylesheet is not edited |
| Theme setting | `internal/component/web/assets/cli.js`, the `ze-theme` browser setting | Remembers the chosen theme | Extends: the fleet shell script uses the same setting |
| Charts | The vendored uPlot under `/assets/`, and `l2tp-chart.js` | Draws one feature's series | Extends: the fleet line chart uses the same library and the same way of reading colours, and redraws on a theme change |
| Markup guards | `internal/test/templcheck`, `internal/test/markupcheck`, used by the web package's tests | Refuse map-typed view data, markup built in Go, inline script and style | Extends: the fleet package runs the same guards over itself |
| Fixed-answer dispatcher | `internal/component/web/handler_golden_input_test.go`, `webGoldenDispatch` | Answers fixed JSON per command in tests | Does not apply directly: it is test-only. The sample source follows its shape as a tagged, non-test package |
| Workbench table and detail components | `internal/component/web/workbench_table.go`, `workbench_detail.go` | Tables of plain text cells and a detail panel | Does not apply: unexported, and a cell cannot hold an icon, a link, a checkbox or a sort control. The fleet package builds its own; the umbrella's reuse rows are corrected |
| Web command line | `internal/component/web/component_cli_terminal.templ` and `cli.js` | A terminal bound to fixed addresses and the hub's dispatcher | Does not apply here: rebinding it to a device is `fleet-ui-4` |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A browser request to `/fleet/...`
- A scripted event from the sample source: a device changes state, a task progresses, an alarm opens or clears

### Transformation Path
1. The hub mounts the fleet routes at web server start because a data source is registered.
2. A fleet handler parses the URL into an object, a tab and view state.
3. The handler asks the data source its question and receives typed values.
4. The handler builds a named view struct and renders the shell, or a fragment when htmx asked for one.
5. The browser opens the event stream for its view. The source emits events naming the fragments that are stale, and the shell script asks htmx to fetch them again.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Browser to web server | HTTPS on the loopback address, htmx requests and server-sent events | No |
| Fleet pages to data | The data source interface, one registered source | No |
| Always-on code to the fleet package | A tagged blank import in the hub only | No |

### Changed-Wire Census

No wire changes. This spec adds HTTP routes that render markup and a data
source interface internal to one package. It sends and reads no managed-channel
verb, no command and no stored format.

**Search evidence:** the fleet package does not exist yet (`ls internal/component/web/fleetui` finds nothing), and `pkg/fleet/envelope.go`, read whole, holds the four configuration verbs this spec neither sends nor handles. Positive control: a search for `RegisterWebRoute` finds its nine callers, all inside the web package.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Managed channel verbs | writer | `internal/component/plugin/server/managed_serve.go`, `pushConfigChanged` | Read: unchanged by this spec | Preserved: not touched |
| Managed channel verbs | reader | `internal/component/managed/client.go`, `handleHubRequest` | Read: unchanged by this spec | Preserved: not touched |

### Integration Points
- `web.RegisterWebRoute` - every fleet route
- `web.RouteDeps` - the username of the request, for the header and for sample audit entries
- `feature-gates.txt` and the tier manifest - the new package
- `cmd/ze/hub` - the two tagged import files

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Pages read only through the data source interface |
| No unintended coupling (components stay isolated) | Yes | The web package gains no import of the fleet package; the fleet package imports the web package for route registration only |
| No duplicated functionality (extends existing, does not recreate) | Yes | The Existing Mechanisms table states why the web package's table, panel and terminal cannot be used |
| Zero-copy preserved where applicable (refs, not copies) | Yes | Not applicable: no wire encoding |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | Routes register; tabs and dashboard cards are registered entries inside the fleet package; the data source registers |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes | The only lists that learn the name are the gate manifest and the tier manifest, which are the declared registries for a new gated package. The web mode switch is not touched in this spec |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A build line written before the package clause of a templ file is copied into its generated file | The templ parser keeps the header lines | The fleet templates cannot be gated and the package must be gated another way | Generate once and read the output, in the wiring phase | confirmed: by the parser read (`TemplateFileParser.Parse` keeps every line before `package` in `Header`, `generator.writeHeader` writes them before the package clause) and by a generation run. Not load-bearing: the phase 1 finding below needs no build line in the package |
| A-2 | A script may set a custom property on an element under the current content security policy | The policy restricts markup, not script-driven style changes | The sidebar falls back to three fixed widths chosen by class | `test/web/fleet-shell-preferences.wb`: the End key on the handle, then the root element's `style` read by the `attribute` expectation, which holds the custom property only on the object-model path | confirmed: `test/web/fleet-shell-preferences.wb` ran green in the seven-case fleet browser run on the final code, so the `style` read found the custom property set through the object model under the content security policy. The drag itself was the owner's browser check, and he approved the prototype as built |
| A-3 | The gate tooling derives the `ze_web` dependency for a package nested under the web package | It does so for packages nested under BGP | The compound build line is written by hand in every file | `./le arch tier check` in the wiring phase | broken as worded, fallback taken. The generator derives a compound constraint only for a package it discovers (`discoverPlugins` over `pluginDirs`), and it does not discover this one, so nothing is derived. The fallback is lighter than stated: `sameFeatureImporter` exempts every file under the web package's directory, so the compound line is written by hand in the two hub import files only. The wrong assumption read the BGP precedent as general |
| A-4 | A route under the authenticated wrap is reachable without login when the server runs with the insecure option | The browser test harness works this way | The owner logs in with a sample user | Opening `/fleet/` in the review build | confirmed: by reading (`startWebServer` sets `authWrap = zeweb.InsecureMiddleware`, and the route loop wraps `WrapAuth` with it) and by a browser session against a review build started with the insecure option, which opened `/fleet/` with no login |
| A-5 | The asset and htmx checks accept a shell in a directory they do not list, or accept a fourth listed directory | Their surface list names three directories | The fleet directory is added to their list in this work | Running both checks in the wiring phase | confirmed. The asset check reads each listed directory with `os.ReadDir` and does not descend, so it never reads the fleet directory; the fleet package carries its own asset guard. The htmx check walks the web package recursively (`collectFiles`), so it reads the fleet directory with no new row |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The sample package reaches a shipped build | A shipped flavour lists the sample package among its dependencies | The absence test over every shipped flavour; the tag is not in the gate manifest |
| R-2 | Pages drift from the umbrella's appearance table as many are built at once | A page file defines its own colour, spacing or component | The style test over fleet styles; page families share the components and are reviewed against the appearance table before the owner sees them |
| R-3 | Parallel work on the package collides | Two page families edit one file | Each page family owns its own handler, template and style files; the shell, the components and the data source land first and are not edited by page work |
| R-4 | The sample source grows a shape no real command can answer | A page needs data the `fleet` children cannot provide | Every question in the data source interface names the `fleet` child expected to answer it |
| R-5 | The scripted live events make a browser test flaky | A test depends on the timing of an event | The script is driven by a counter the test can step through a prototype-only route, never by wall time, when the test option is set |
| R-6 | The review build is left running, or collides with another session's port | A listener remains after the review | The port is chosen free at start, recorded in the spec with the process and its removal condition, and stopped by the session that started it |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing in a standard build: the fleet package is dormant. A build flavour could fail to compile if a build line is wrong |
| How is it reverted? | Remove the gate line, the tier row, the two hub import files and the package |
| Who else touches this path? | The session fixing fleet configuration works on the managed channel, not on the web package. `spec-web-plugin-extension-registry.md` will later own page registration |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Sample build, `GET /fleet/` | → | The fleet shell renders the estate dashboard of the sample estate | `TestFleetShellRendersSampleEstate` |
| Sample build, browser opens `/fleet/` | → | Shell, tree and dashboard are visible | `test/web/fleet-prototype-renders.wb` (green under `ze.tags=ze_fleet_ui_sample`) |
| Standard build | → | No fleet route is mounted | `TestFleetRoutesAbsentWithoutSource` |
| Every shipped flavour | → | The sample package is not linked | `TestShippedZeLinksNoFleetSample` |
| Build without `ze_fleet_ui` | → | The binary holds no fleet symbol | `cmd/ze/hub/build_tag_fleet_ui_absent_test.go` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A build carrying the sample tag, started web-only | `/fleet/` opens the estate dashboard of the sample estate inside the shell, in both themes | The other web modes stay reachable at their own addresses, because the prototype runs on the normal web server |
| AC-2 | Each interaction in the interaction table | It works as described on every page where it applies | Actions that would change the estate stop at a toast saying the prototype changed nothing, because the prototype judges the interface |
| AC-3 | Every page of children 2 to 6 | The page exists, is reachable from the tree, the tabs or the Administration tab, and renders with sample data in both themes | A page may show fewer rows than a real estate would, because the sample estate has 44 devices |
| AC-4 | Any view | Its address reproduces it, including filter, sort, page, selected row, chart range and tree view | Sidebar width, pin states and theme stay in the browser, because they are preferences |
| AC-5 | The fleet styles | Every colour is a variable and every gap is one of the six spacing variables | The variable declarations themselves hold the literal values |
| AC-6 | Every state of the umbrella's states table | It is reachable from the prototype-only states page and renders as specified | None |
| AC-7 | A standard build | The web server starts and serves every existing page as before, and a request to `/fleet/` receives the server's ordinary not-found answer; the portal menu lists the same entries as before | The fleet package is linked but dormant, because the gate is on by default like every gate |
| AC-8 | Every shipped flavour | Its dependency list is produced and names the fleet package, and does not name the sample package | Nothing of the sample package may remain, because sample data in a shipped build would be shown as real |
| AC-9 | A build without `ze_fleet_ui` | It compiles in every flavour, starts, and serves every existing page; its symbol table holds no fleet symbol | Nothing of the fleet package remains, because that is what the gate promises |
| AC-10 | The owner's review | Every correction he states is recorded in this spec and applied, and he states that the prototype is right | A correction he defers is recorded with the child that takes it |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Opens the estate and finds the devices needing attention | Shell, estate dashboard, card link, filtered devices table | `test/web/fleet-estate-to-failing-devices.wb` |
| 2 | Finds one device by typing part of its name and opens it | Tree filter, device page | `test/web/fleet-tree-selects-device.wb` |
| 3 | Filters and sorts the devices table and opens a row's details | Query row, sort link, eye control, side panel | `test/web/fleet-devices-filter.wb` |
| 4 | Watches a task progress and a device change state without reloading | Event stream, fragment refresh | `test/web/fleet-live-updates.wb` |
| 5 | Runs a command in a device's console | Console tab, sample answer | `test/web/fleet-device-console.wb` |
| 6 | Switches theme and resizes the sidebar | Shell script, browser settings | `test/web/fleet-shell-preferences.wb` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestFleetURLRoundTrip` | `internal/component/web/fleetui/url_test.go` | A parsed fleet address renders back to itself | |
| `TestDeviceDisplayStatePrecedence` | `internal/component/web/fleetui/state_test.go` | The display state follows the umbrella's precedence | |
| `TestFilterParse` | `internal/component/web/fleetui/filter_test.go` | Each form of the filter grammar parses, and a malformed term is refused naming the term | |
| `TestTreeGrouping` | `internal/component/web/fleetui/tree_test.go` | Devices group by a view's levels, with a node for a missing label and correct counts | |
| `TestFleetStylesUseVariablesOnly` | `internal/component/web/fleetui/style_test.go` | No literal colour outside variable declarations; no gap outside the spacing variables | |
| `TestFleetViewDataIsTyped`, `TestFleetNoGoFileBuildsMarkup`, `TestFleetTemplatesAvoidInlineScriptAndStyle` | `internal/component/web/fleetui/guard_test.go` | The markup guards hold for the fleet package | |
| `TestFleetShellRendersSampleEstate` | `internal/component/web/fleetui/sample/handler_test.go` | The shell and estate dashboard render from the sample source | |
| `TestFleetPagesRender` | `internal/component/web/fleetui/sample/pages_test.go` | Every page address of the set answers 200 with its object header and tab row, as a table of cases | |
| `TestFleetRoutesAbsentWithoutSource` | `internal/component/web/fleetui/register_test.go` | With no source registered, the enabling function answers false | |
| `TestSampleEstateInvariants` | `internal/component/web/fleetui/sample/estate_test.go` | The sample estate holds the counts this spec states | |
| `TestShippedZeLinksNoFleetSample` | `cmd/ze/fleet_sample_absent_test.go` | No shipped flavour links the sample package | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Page size | 25, 50, 100 | 100 | Any other value falls back to 50 | Any other value falls back to 50 |
| Page number | 1 to last page | Last page | 0 shows page 1 | Beyond the last page shows the last page |
| Chart range | hour, day, week | week | An unknown value falls back to day | An unknown value falls back to day |
| Filter length | 0 to 512 characters | 512 | N-A | 513 is refused with a message |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `fleet-prototype-renders` | `test/web/fleet-prototype-renders.wb` | The interface opens on the sample estate | green on the final code |
| `fleet-tree-selects-device` | `test/web/fleet-tree-selects-device.wb` | Filtering the tree and selecting a device opens its page | green on the final code |
| `fleet-devices-filter` | `test/web/fleet-devices-filter.wb` | A filter narrows the table, survives a reload, and a row opens the side panel | green on the final code; the sort of user story 3 is not asserted (a sort header has no unique accessible name to click) |
| `fleet-estate-to-failing-devices` | `test/web/fleet-estate-to-failing-devices.wb` | A dashboard count leads to exactly the rows it counts | green on the final code |
| `fleet-live-updates` | `test/web/fleet-live-updates.wb` | A stepped event changes the tree, the table and the header's running-task count without a reload | green on the final code; started held (`ze.web.fleet.sample.held=true`), stepped by the header's Step (`fleet-proto-step`) once the page's stream is open (`data-fleet-stream`): step 1 adds cpe-lon-03 to the table filtered to `state:offline` and puts Offline in its tree row, step 9 ends T-2051 and the count goes from 2 to 1 |
| `fleet-device-console` | `test/web/fleet-device-console.wb` | A sample command typed in the console shows its answer | green on the final code |
| `fleet-shell-preferences` | `test/web/fleet-shell-preferences.wb` | Theme and sidebar width persist across a reload | green on the final code; asserts the theme and the width (custom property) across a reload; the drag stays a human check |

## Files to Modify
- `feature-gates.txt` - the `ze_fleet_ui` line
- `internal/le/arch/tier/testdata/tier_non_engine_categories.txt` - the fleet package's category row
- `internal/le/web/assets/webassets.go` - the fleet directory as a fourth surface, if the asset check requires it (assumption A-5)
- `docs/architecture/web-interface.md` - the fleet routes
- `docs/architecture/plugin/feature-gates.md` - the new gate and the sample tag outside the manifest
- `docs/architecture/core-design.md` - the section 19 row for the fleet package
- `plan/post-release/features/spec-fleet-ui-0-umbrella.md` - the reuse rows corrected

## Files to Create
- `internal/component/web/fleetui/register.go` - route registration and the enabling function
- `internal/component/web/fleetui/source.go` - the data source interface, its value types and the registry slot
- `internal/component/web/fleetui/url.go`, `state.go`, `filter.go`, `tree.go` - address parsing, display state, the filter grammar, tree grouping
- `internal/component/web/fleetui/shell.templ` and `handler_shell.go` - the shell, the tree fragment, the search, the event stream
- `internal/component/web/fleetui/component_*.templ` - the components of the umbrella's set
- `internal/component/web/fleetui/page_*.templ` and `handler_*.go` - one pair per page family: estate, group, device, tables, tasks, alarms, config, rollouts, audit, administration, states
- `internal/component/web/fleetui/assets/` - `fleet-shell.css`, `fleet-components.css`, one stylesheet per page family, `fleet.js`, `fleet-chart.js`, `fleet-terminal.js`, an icon sprite
- `internal/component/web/fleetui/sample/` - the sample estate, its history generator, its event script, the source registration
- `cmd/ze/hub/register_fleet_ui.go`, `cmd/ze/hub/register_fleet_ui_sample.go` - the tagged imports
- `cmd/ze/hub/build_tag_fleet_ui_present_test.go`, `build_tag_fleet_ui_absent_test.go`, `cmd/ze/fleet_sample_absent_test.go`
- `test/web/fleet-*.wb` - the functional tests above
- `docs/architecture/fleet-interface.md` - the package, the seam, the tags and how to start the review build

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | The prototype reads no configuration |
| YANG validation constraints | N-A | No leaf |
| YANG custom validators | N-A | No leaf |
| CLI commands/flags | N-A | No command is added; the real commands belong to the `fleet` children |
| CLI grammar (keyword before value) | N-A | No command |
| Editor autocomplete | N-A | No leaf |
| Functional test for new RPC/API | Yes | `test/web/fleet-*.wb` |
| Pipe completeness | N-A | No command answer |
| Env var registration | N-A | The sample form is selected by the tag alone; no variable |
| Doctor check for runtime dependencies | N-A | No new path, socket, port or binary: the existing web listener serves it |
| Doctor code per problem class | N-A | The prototype raises no refusal or log line an operator can meet in a shipped build, where it is dormant |
| Prometheus counters/metrics | N-A | Deferred with the real event stream to `fleet-ui-5`, which owns the stream's bounds |
| BGP family surface (new SAFI / capability / attribute) | N-A | No BGP surface |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Nothing reaches a user of a shipped build; `features/fleet-management.md` changes when a page becomes functional |
| 2 | Config syntax changed? | No | None |
| 3 | CLI command added/changed? | No | None |
| 4 | API/RPC added/changed? | No | None |
| 5 | Plugin added/changed? | No | None |
| 6 | Has a user guide page? | No | The operator guide is written when the interface is functional |
| 7 | Wire format changed? | No | None |
| 8 | Plugin SDK/protocol changed? | No | None |
| 9 | RFC behavior implemented, changed, or newly proven? | No | None |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`, if the browser harness gains a way to select the sample build |
| 11 | Affects daemon comparison? | No | Not until functional |
| 12 | Internal architecture changed? | Yes | `docs/architecture/fleet-interface.md` (new), `docs/architecture/web-interface.md`, `docs/architecture/core-design.md` section 19, `docs/architecture/plugin/feature-gates.md` |
| 13 | Route metadata keys added/changed? | No | None |
| 14 | Prometheus counters added/changed? | No | None |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | None |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived with `./le spec citation anchors` once files change |
| 17 | Existing docs show config/CLI/API examples for this area? | No | None exist for the fleet interface |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the package, the gate, the tier row, the two hub imports, the data source slot, one route rendering a minimal shell from a minimal sample source
   - Tests: `TestFleetShellRendersSampleEstate`, `TestFleetRoutesAbsentWithoutSource`, `TestShippedZeLinksNoFleetSample`, the present and absent tag tests
   - Files: `register.go`, `source.go`, `shell.templ`, `handler_shell.go`, `sample/`, `feature-gates.txt`, the tier manifest, the hub import files
   - Verify: assumptions A-1, A-3 and A-5 are settled; a sample build serves `/fleet/`; a standard build does not
2. **Phase: Foundation** -- the data source interface in full, the sample estate with its history and event script, address parsing, display state, filter grammar, tree grouping
   - Tests: `TestFleetURLRoundTrip`, `TestDeviceDisplayStatePrecedence`, `TestFilterParse`, `TestTreeGrouping`, `TestSampleEstateInvariants`
   - Verify: every question a page will ask has a typed answer from the sample source
3. **Phase: Shell and components** -- the shell with header, sidebar, tree, object header, tab row and side panel; every component of the umbrella's set; the shell script; the shared styles; the states page
   - Tests: the guard tests, `TestFleetStylesUseVariablesOnly`, `fleet-prototype-renders`, `fleet-shell-preferences`, `fleet-tree-selects-device`
   - Verify: the states page shows every component and every state in both themes, matching the umbrella's appearance table
4. **Phase: Pages** -- page families built side by side, each in its own files: devices table and side panel; dashboards and charts; device tabs and console; tasks, alarms, toasts and live updates; config, rollouts, audit and administration
   - Tests: `TestFleetPagesRender`, the remaining functional tests
   - Verify: every page of children 2 to 6 opens and every interaction of the interaction table works
5. **Phase: Review build** -- build the sample binary to scratch, start it for the owner, record the address, the process and its removal condition in this spec
   - Verify: the owner opens it
6. **Phase: Owner review** -- record and apply each correction, amend the umbrella and the children, show again until he says it is right; then stop the review build

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC has an implementation at file and line |
| Feature completeness | Every page named in children 2 to 6 has an address that answers |
| Correctness | A device shows the same state in the tree, the table, the cards and its header; every count on a card equals the rows its link opens |
| Naming | Address segments and tab names match the umbrella's tables exactly |
| Data flow | No page reads the sample package; pages read only the data source interface |
| Operator errors and doctor codes | A malformed filter names the term and the valid fields |
| Rule: umbrella appearance table | Every part listed there looks as described, in both themes |
| Rule: build constraints | Every non-test file of the fleet package and its generated files carries the build line; the sample tag appears in no manifest |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The gate is declared and derived lists are current | `./le repo feature-tags check` |
| Placement and imports are accepted | `./le arch tier check` |
| Generated templ output is current | `./le doc check templ-output` |
| The package tests pass under its tags | `./le test unit pkg path ./internal/component/web/fleetui/` with the fleet and sample tags |
| The browser tests pass | `./le test functional web` over the `fleet-*` cases, with the sample tag selected |
| The tree compiles in every flavour | `./le repo compiles check` |
| The review build answers | A request to `/fleet/` on the recorded address |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Sample data in a shipped build | The absence test; the tag is in no manifest |
| Input validation | Device names, label values, filter text, sort keys, page and size are validated before use; an unknown device answers the not-found state |
| Output | Every sample value is escaped by templ; no value is written as raw markup |
| Markup rules | No inline script or style, no event attribute; the guards run |
| Review build exposure | Bound to the loopback address only, stopped after the review |
| Event stream | Bounded buffer per connection; a stream that is not read is closed; a stated maximum of streams |

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

- A gate declared in the manifest is on in every shipped build. A thing that must never ship is a tag outside the manifest, with a test that proves its absence.
- The web package's components are private to it. A second interface that wants them either lives in that package or builds its own; sharing is a later, deliberate step.

### Phase 1 findings (wiring)

- No file of the fleet package or of the sample package carries a build line. The tier check asks for none (`sameFeatureImporter`), the web package's own files carry none, and the `zetest` precedent tags the importer only. The gate is the tagged blank import in the hub. This corrects the Required Reading constraint and the "Rule: build constraints" review row, which now apply to the two hub import files.
- No tier manifest row is needed. `Classify` reads the top-level directories of `internal/component` only, and the web row covers the nested package. "Files to Modify" lists that file for no change.
- Nesting under the web package put the fleet templates inside the web package's asset guard, whose walk was recursive: a `/fleet/assets/` path is a finding against the `/assets/` tree. `markupcheck.assetFindings` now reads the directory it is given and no package under it, with `TestAssetFindingsLeavesANestedPackageToItsOwnScan`. No template or `page_assets.go` existed under a guarded directory, so no existing coverage moved. The change to a shared guard is the main thread's to send to `ze-test-judge`.
- The same nesting put `fleetui/shell.templ` inside the web package's golden walk: `TestWebGoldenOutput` demanded it of `webTemplGoldenSpec`, which cannot name another package's components. With `Ext` set, `golden.Set.Files` now leaves out a directory below `Dir` that holds a Go source file, and still demands a template in a nested directory that is no package (`TestFilesLeavesANestedPackageToItsOwnSet`, `TestFilesWalksEveryDirectoryOfATemplateFS`). The two callers, the web and looking-glass packages, hold no other nested `.templ` file, so no existing coverage moved. No rule obliges a templ package to carry a golden capture, and the fleet package declares none. The change to a shared guard is the main thread's to send to `ze-test-judge`.
- A tag outside the manifest is linted by no pass until it joins the `capability` flavor of `internal/le/go/lint/matrix.go`. `ze_fleet_ui_sample` is added there, and the two pins in `parity_test.go` follow.
- The fleet asset guard takes one prefix, `/fleet/assets/`. A fleet template that links the web package's `/assets/htmx.min.js` or uPlot is a finding until `AssertAssetsResolve` can be given a second served tree. Phase 3 owns that.
- AC-8 says every shipped flavor names the fleet package. A flavor that does not link the hub cannot. `TestShippedZeLinksNoFleetSample` requires the sample package absent from every flavor and the fleet package present in at least one.
- AC-7 says `/fleet/` receives the ordinary not-found answer. The web server's catch-all hands an unowned address to the fragment handler, so the answer is whatever that handler gives an unknown address, not necessarily 404.

### Phase 2 findings (foundation: the interface and the pure logic)

- The data source answers facts in separate lists (devices, open alarms, tasks), and the fleet package joins them once per request into one view per device. A device record carries no alarm or task count of its own, so a count on a card cannot disagree with the list beside it. This is input to `fleet-1`: its device record does not need those counts.
- Each device line of the Needs attention card is counted by applying the filter its link carries, so "a count equals the rows its link opens" holds by construction for that card.
- The umbrella's URL scheme does not say how the node of the devices that lack a label is addressed. A group segment with no `=` is that node: `/fleet/group/site/dashboard/`. An equals sign inside a key or a value is escaped, so the split is never ambiguous.
- A fragment address is `/fleet/fragment/<name>/<page path>` with the page's view parameters, and `item` for a fragment a page holds several of. The umbrella fixes only the prefix. The address of the event stream carries nothing yet: the shell phase decides how a stream names its view.
- The state column sorts by severity in this order: down, offline, diverged, degraded, unknown, healthy. The umbrella fixes the precedence that chooses a state and not the order a column sorts by. This order follows the alarm severities of `fleet-ui-5`.
- A device whose health report holds a component state the hub does not know is Unknown, unless another component is down or degraded. The umbrella's table has no row for it.
- The task state `fleet-ui-5` spells "Cancelled" is `Canceled` in the code and on the page: the repository's lint refuses the other spelling.
- A sort column the table does not have is refused with the list of columns, like a malformed filter. The Boundary Tests table has no row for it.
- The duration fields of the filter (`uptime`, `last-seen`) take a number and a unit, as in `last-seen:>1d`. `fleet-ui-2` lists the fields and gives no form for a duration.
- An address with no `view` parameter asks for the tree view `site-role`, and a `view` that names no view is refused with the list of views. No tree is regrouped by a default in silence.

### Phase 2 findings (foundation: the sample source)

- The sample estate table first stated one critical alarm while `fleet-ui-5` makes "device offline" critical and three devices are offline, so phase 2 set the offline alarm delay to seven days and opened none. The integration pass changed the table instead: the sample hub opens the offline alarm five minutes after a device was last seen (its `Settings`), so each offline device has a critical alarm open (4 critical, 12 open), the two devices that were away during the week have the cleared alarm of that outage, and the script device's alarm opens and clears with its outage, the script standing for an outage longer than the delay.
- "12 finished" is read as eleven succeeded and one partly failed, beside the one failed and the two running: fifteen tasks. The sample hub keeps thirteen ended tasks, so an ended scripted task pushes out the oldest of the table.
- Two devices grant no relay profile, not one: two of the three informational alarms are "relay not granted", and a device with that alarm has to refuse its views. One of them is the device in the Unknown state, which never fetched a configuration.
- A series is answered at 30 seconds, 5 minutes or 30 minutes according to the period asked for, at most 360 points. `Series.Interval` carries which. This is input to `fleet-8`: a history that thins its older samples fits the interface as it is.
- The scripted events run on a timer that the first subscription starts, one step every five seconds, and `sample.Stepper` (`Step`, `Hold`) takes the script away from the timer for good. Holding through the route only after a page opened left the first step to the timer, so the step a test saw depended on load: the test option of risk R-5 is the env key `ze.web.fleet.sample.held`, registered by `sample/register` (so only a sample build knows it), which holds the script before the source is registered. The step control is in the header over any `Stepper` (Step and Hold, `fleet-proto-step`, `fleet-proto-hold`), so a test and the owner step the estate from the page they watch; the root's `data-fleet-stream` says the page's stream is open, so a test steps after the hub subscribed it.
- What the data source interface cannot say for this data, each an input to the child that owns it: an audit entry names its target as text and holds no device list, so the audit section of a device's Tasks tab can only match that text (`fleet-3`); a rollout holds no task identifier and a task no rollout name, so a rollout page cannot link to the tasks of its batches (`fleet-5`); `DeviceSeries` takes no interface, so the traffic chart of an interface row's side panel has no question to ask (`fleet-ui-4`, `fleet-8`); the side panel of a BGP peer (capabilities, counters, state history) has no question either (`fleet-ui-4`); `Source` has no close, so the timer of a registered source lives as long as the process.

### Phase 3 findings (the shell)

- The shell knows no page. A page family registers each page under a kind of object and a tab, and a tab with no registered page shows a "not built yet" state, which the umbrella's states table does not list. It is a state of the prototype's construction and disappears when the last page is registered.
- A link inside the shell is followed by htmx into the content region, and the page handler answers such a request with the content and, out of band, the regions that follow the page. The umbrella says only that fragments exist; this is how one address serves both a first load and a navigation.
- The event stream names its page in a `scope` parameter, and scopes the content region only. The tree, the task panel and the header show the whole estate on every page, so a change of any device names the tree for every stream. No event carries a device name or any data, so a stream cannot show an operator a device outside their view (umbrella R-5).
- A task event names its task and no device, so it makes every region of every page stale. Scoping it would need the task's devices at event time.
- The tree filter is not part of an address. The umbrella's URL scheme gives it no parameter, and `q` is the filter of the page's table.
- The umbrella's header row lists "sign out" in the account menu. The web server has no sign-out route, in any mode, so the menu holds the theme switch and the link to the other web modes only. A sign-out route belongs to the web package's session handling and is not built here.
- The prototype-only routes are two, `POST /fleet/prototype/step` and `POST /fleet/prototype/hold`, one per method of the sample source's stepped mode. Both are mounted only over a source that has that mode.
- The fleet asset guard resolves `/assets/` against the directory the web package embeds, through a new `markupcheck.AssertAssetsResolveIn` that takes several served trees. The existing single-tree assertion and its messages are unchanged. The change to a shared guard is the main thread's to send to `ze-test-judge`.
- `sample/handler_test.go`, written for the phase 1 shell, expects every device name in the estate page. The devices of the estate page now sit below the second level of the tree and load on expansion, so that expectation no longer holds and the sample test owes a change by the agent that owns the sample package.
- Assumption A-2 is written both ways: the script sets `--fleet-sidebar-width` through the object model, reads it back, and falls back to three widths chosen by a class when the property did not take. Which path a browser takes under the content security policy is still to be observed.

### Phase 4 findings (the page families and their integration)

What the five page families decided that departs from or adds to this spec and its children:

- Edit on a device's Config tab answers the prototype toast. It does not embed the YANG editor: the editor belongs to the web package, and nothing in the prototype changes anything.
- A template's preview shows only what the data source answers. The source answers no template text, variables or rendering, so the panel says so instead of drawing a preview.
- The devices table has no column chooser: its columns are fixed.
- An interface row's side panel has no traffic chart: `DeviceSeries` takes no interface (phase 2 findings).
- A BGP peer's side panel shows the session and its prefixes only: `Peer` carries no capabilities, families, counters or state history.
- The account menu has no sign out: the web server has no sign-out route (phase 3 findings).
- Event scoping narrows the content region only. The tree stays global, so a change of any device names the tree for every stream.
- The opened divergence diff and the new-rollout form are values of the `sub` parameter (`?sub=divergence`, `?sub=new`), the parameter that names a page's sub-view. The umbrella's URL scheme has no other place for them.
- The Busiest devices card takes its metric from the `sort` parameter of the dashboard's address.
- A device's Logs tab does not stream in the prototype: the Recent sub-tab draws the follow control, and the lines are those of one request to the source.
- Every action that would change the estate posts to one action registry, `POST /fleet/action/<name>/<page>`, which answers a toast and changes nothing. The Console keeps its own route, `POST /fleet/console/`, because it answers a terminal line, not a toast.
- The audit export is `GET /fleet/export/audit`, bounded to `AuditEntriesMax` (1000) entries, in CSV or JSON.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The seam is a data source interface inside the fleet package | A sample command dispatcher answering `show fleet` text | The commands do not exist yet, so a fake dispatcher would invent their grammar as well as their answers. A typed interface states exactly what each page asks, and the real source later implements it over whatever commands the `fleet` children settle |
| The fleet package is nested under the web package | A sibling component; files inside the web package under a file tag | Nesting derives the dependency on `ze_web` from the path. A sibling needs every compound constraint written by hand. Files inside the web package cannot be gated as a package and would raise its exact component count |
| Own components, not the workbench's | Exporting and extending the workbench table and panel | Their cells are plain text and their fixtures are pinned; extending them moves every workbench fixture for a prototype. Sharing waits until the workbench needs it |
| Sample tag outside the gate manifest | Declaring it in the manifest | Every manifest tag is set by every shipped build |
| Routes mounted only when a source is registered | An environment variable selecting the prototype | The tag already says it; a variable would be a second switch that could disagree |
| Scripted events driven by a steppable counter under test | Wall-clock timers only | A browser test must not depend on timing |
| Reached at its own address and the portal menu | Adding Fleet to the web mode switch now | The switch lives in the existing shells' markup and fixtures; changing it belongs with the functional work |

## Known Limitations
- Nothing in the prototype changes anything. Every action ends at a toast.
- The Console answers a fixed list of sample commands.
- Fleet is not yet a selectable web mode in the existing shells; that is `plan/post-release/features/spec-fleet-ui-2-estate-tree-and-device-tables.md`.
- The real enabling condition, a daemon serving managed devices, is `plan/post-release/features/spec-fleet-ui-2-estate-tree-and-device-tables.md`.

## Review build record

| Field | Value |
|-------|-------|
| Binary | `tmp/session/2026-11-18-205620/scratch/ze-fleet-review`, built with the sample tag and without the `ze_l2tp` and `ze_vpp` gates, because another session's in-progress change breaks the L2TP build in the shared checkout; the fleet interface depends on neither |
| Address | `https://127.0.0.1:59440/fleet/` (loopback, self-signed certificate, no login) |
| Process | pid 46189, recorded in `tmp/session/2026-11-18-205620/scratch/fleet-review.pid`; config store `tmp/session/2026-11-18-205620/scratch/fleet-review-config`, log `tmp/session/2026-11-18-205620/scratch/fleet-review.log` |
| Started by | The main session of this spec, on 2026-11-19 |
| Removal condition | Stopped and the scratch binary removed when the owner ends the review |
| Ended | The owner ended the review with his approval on 2026-11-19. At closure the review build was still running (pid 95652 by the pid file, `ze-fleet-review start --web 59440 --web-only`) and the scratch binary was still present: the main session that started it owes the stop and the removal (closure report, owed commands) |

## Owner review record

| # | Page | What the owner said | Change made | Applied to |
|---|------|---------------------|-------------|------------|
| 1 | Whole prototype (review build at commit `fc44d67279`) | 2026-11-19: "the prototype is excellent". No correction stated | None | AC-10 met: the prototype is approved as built; children 2 to 6 may start |

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
- [ ] AC-1..AC-10 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove` this spec's path only, in the same `./le commit create` script

## Implementation Summary

### What Was Implemented
- The fleet package `internal/component/web/fleetui`, gated by `ze_fleet_ui` (`feature-gates.txt`), linked by `cmd/ze/hub/register_fleet_ui.go`. Its routes register through `web.RegisterWebRoute` from `init()` in `register.go` and mount only when a source is registered (`sourceRegistered`); the two prototype-only routes mount only over a `Stepper` (`stepperRegistered`).
- The data source seam (`source.go` and the `source_*.go` files), the address scheme (`url.go`, `ParseAddress`), the display state (`state.go`), the filter grammar (`filter.go`, `ParseFilter`), tree grouping (`tree.go`), and the joins of facts into views (`derive.go`, `summary.go`).
- The shell, the tree, the header search, the event stream and the component set (`shell.templ`, `handler_shell.go`, `handler_tree.go`, `handler_search.go`, `handler_events.go`, `component_*.templ`), with its own assets under `/fleet/assets/`.
- Every page family of children 2 to 6 on the sample estate: devices, dashboards and charts, device tabs and console, tasks, alarms and states, config, rollouts, audit and administration (`handler_*.go`, `page_*.templ`, `register_*_pages.go`).
- The sample source `fleetui/sample` and its registration `fleetui/sample/register`, linked only under the tag `ze_fleet_ui_sample`, outside the manifest (`cmd/ze/hub/register_fleet_ui_sample.go`), with the held start `ze.web.fleet.sample.held` for the browser tests.
- Shared guards extended for a nested package: `markupcheck.AssertAssetsResolveIn` and `assetFindings` reading one directory (`internal/test/markupcheck/assets.go`), `golden.Set.Files` leaving a nested Go package to its own set (`internal/test/golden/golden.go`), and the `attribute` expectation of the browser harness (`internal/component/web/testing/expect.go`).
- Seven browser tests `test/web/fleet-*.wb` and the tag selection of the web runner (`internal/test/runner/build_tags.go`).

### Bugs Found/Fixed
- Closure: three source anchors named symbols not declared where named (Review Gate, finding 1).

### Documentation Updates
- `docs/architecture/fleet-interface.md` (new), `docs/architecture/web-interface.md` (the fleet routes), `docs/architecture/web-components.md`, `docs/architecture/core-design.md` section 19, `docs/architecture/plugin/feature-gates.md`, `docs/functional-tests.md`, `docs/architecture/testing/runner-architecture.md` and `ci-format.md`, all in the implementation commits.
- Closure: `fleet-interface.md` line 462 repointed to the four link tests of `links_test.go`; `web-interface.md` line 128 repointed to `newActionHandler` (`handler_action.go`) and `newAuditExportHandler` (`handler_audit.go`).
- `./le doc check all` after the fix: the source anchor section went from 8 undeclared symbols to 5, none under a fleet page. The remaining failures belong to other sessions' work in the shared checkout: 4 local handlers of `show fleet divergence` and `request fleet divergence revert` with no YANG command, `ze-cos:cleanup` with no handler, and anchors under `l2tp`, `iface`, `bgp/plugins/rib` and `core-design.md` line 3522.

### Deviations from Plan
- Opening a side panel fetches the panel alone, so the table's sort and page links keep the address they were drawn with, and a later sort closes the panel (`PanelLink` paragraph of `docs/architecture/fleet-interface.md`, `panelLink` in `handler_shell.go`). Approved by the owner as built; carried into `spec-fleet-ui-2-estate-tree-and-device-tables.md`, "Carried from the prototype".
- `TestFleetPagesRender` was not written under that name. Every registered page is proven answered by `TestFleetDemoTourCoversEveryRegisteredPage` (`internal/le/site/fleetdemo/tour_test.go`, over the sample source) and every gallery address by `TestGalleryAddressesAreAnswered` (`gallery_links_test.go`), beside one render test per page family.
- AC-7: an address under `/fleet/` in a standard build reaches the web server's catch-all, so the answer is whatever that handler gives an unknown address (phase 1 findings).
- AC-8: a flavor that does not link the hub cannot name the fleet package, so `TestShippedZeLinksNoFleetSample` requires the sample absent from every flavor and the fleet package present in at least one.
- The phase findings above record the rest: no build line in the package, no tier row, the held start, the action registry, the sub-view parameter.
- Closure departed from the skill's uncommitted-diff assumption: the entry status was `in-progress`, but every implementation commit had landed, so the review read `git diff 7380b7d204..HEAD` over the spec's files, as the `verification` row of the skill prescribes, and commit A carries the closure edits only.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-3: the gate tooling would derive the `ze_web` dependency for a package nested under the web package | The generator derives a constraint only for a package it discovers, and it does not discover this one | `./le arch tier check` in phase 1 | Compound build line written in the two hub import files; recorded in A-3 |
| approach | Source anchors were written for symbols that a later refactor renamed or that were never committed under that name | `links_test.go` holds four link tests; the action and audit routes are built by closures over `newActionHandler` and `newAuditExportHandler` | `./le doc check all` at closure | Repointed in this closure |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| The fleet package, its compile gate and its routes | Done | `feature-gates.txt` line 258, `fleetui/register.go` `init`, `cmd/ze/hub/register_fleet_ui.go` | |
| Every component of the umbrella's set | Done | `fleetui/component_*.templ`, `component_gallery.go` | `TestComponentGalleryDrawsEveryComponent` |
| Every page of children 2 to 6 on sample data | Done | `fleetui/register_*_pages.go`, `handler_*.go`, `page_*.templ` | `TestFleetDemoTourCoversEveryRegisteredPage` |
| The owner's review and its corrections | Done | Owner review record row 1 | Approved as built, no correction |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestFleetShellRendersSampleEstate`, `fleet-prototype-renders.wb`, `fleet-shell-preferences.wb` (theme) | |
| AC-2 | Done | The seven `fleet-*.wb`, `TestTaskActionsAnswerThePrototypeToast`, `TestDevicesBulkPostAnswersPrototypeToast`, the owner's review | Sort click not asserted in a browser test (no unique accessible name); sorting proven by the unit tests of `table.go` |
| AC-3 | Changed | `TestFleetDemoTourCoversEveryRegisteredPage`, `TestGalleryAddressesAreAnswered` | In place of `TestFleetPagesRender` |
| AC-4 | Done | `TestFleetURLRoundTrip`, `fleet-devices-filter.wb` (filter survives a reload) | |
| AC-5 | Done | `TestFleetStylesUseVariablesOnly`, `TestFleetStylesDeclareEveryVariable` | |
| AC-6 | Done | `TestStatesPageRendersEveryGallerySection`, `TestGalleryAddressesAreAnswered` | |
| AC-7 | Done | `TestFleetRoutesAbsentWithoutSource`, `TestBuildTag_FleetUI_DormantWithoutSource` | Catch-all answer, Deviations |
| AC-8 | Done | `TestShippedZeLinksNoFleetSample`, `TestFleetSampleTagLinksTheSample` | |
| AC-9 | Done | `TestBuildTag_FleetUI_AbsentBinaryDropsFleetSymbols` | Rerun on the final code owed |
| AC-10 | Done | Owner review record row 1, commit `290951e355` | "the prototype is excellent" |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestFleetURLRoundTrip`, `TestDeviceDisplayStatePrecedence`, `TestFilterParse`, `TestTreeGrouping`, `TestFleetStylesUseVariablesOnly` | Done | `fleetui/url_test.go`, `state_test.go`, `filter_test.go`, `tree_test.go`, `style_test.go` | |
| `TestFleetViewDataIsTyped`, `TestFleetNoGoFileBuildsMarkup`, `TestFleetTemplatesAvoidInlineScriptAndStyle` | Done | `fleetui/guard_test.go` | |
| `TestFleetShellRendersSampleEstate`, `TestSampleEstateInvariants` | Done | `fleetui/sample/handler_test.go`, `estate_test.go` | |
| `TestFleetPagesRender` | Changed | `internal/le/site/fleetdemo/tour_test.go` | `TestFleetDemoTourCoversEveryRegisteredPage` |
| `TestFleetRoutesAbsentWithoutSource` | Done | `fleetui/register_test.go` | |
| `TestShippedZeLinksNoFleetSample` | Done | `cmd/ze/fleet_sample_absent_test.go` | |
| The seven functional tests | Done | `test/web/fleet-*.wb` | Green in the seven-case run |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `feature-gates.txt`, `docs/architecture/web-interface.md`, `feature-gates.md`, `core-design.md`, `spec-fleet-ui-0-umbrella.md` | Done | |
| `internal/le/arch/tier/testdata/tier_non_engine_categories.txt` | Changed | No row needed (phase 1 findings) |
| `internal/le/web/assets/webassets.go` | Changed | Not needed: A-5 confirmed |
| `fleetui/` sources, assets, `sample/` | Done | |
| `cmd/ze/hub/register_fleet_ui.go`, `register_fleet_ui_sample.go`, the three tag tests, `cmd/ze/fleet_sample_absent_test.go` | Done | Plus `build_tag_fleet_ui_dormant_test.go` |
| `docs/architecture/fleet-interface.md` | Done | |

### Audit Summary
- **Total items:** 4 requirements, 10 AC, 7 test rows, 6 file rows
- **Done:** 23
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 4 (AC-3, `TestFleetPagesRender`, two file rows), each in Deviations or the phase findings

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| The owner can open the whole interface in a browser, click through and correct it | User review over the real entry point | Review build at commit `fc44d67279` on the loopback address; the owner's approval, Owner review record row 1 |
| The prototype is the real interface: real routes, shell, components, htmx fragments and event stream | Functional (browser) | Seven `test/web/fleet-*.wb` green through the web server's mounted routes, `fleet-live-updates.wb` over the event stream |
| Only the data is sample, and no shipped build carries it | Build-flavor test | `TestShippedZeLinksNoFleetSample` and `TestFleetSampleTagLinksTheSample`, green on the final code |
| A standard build carries the package dormant | Unit over the registry | `TestFleetRoutesAbsentWithoutSource`, `TestBuildTag_FleetUI_DormantWithoutSource` |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| Fleet as a selectable web mode in the existing shells | Changes their markup and fixtures, decided with the functional work | `plan/post-release/features/spec-fleet-ui-2-estate-tree-and-device-tables.md` (amended: "Carried from the prototype") |
| The real enabling condition and the real source | The commands do not exist yet | `plan/post-release/features/spec-fleet-ui-2-estate-tree-and-device-tables.md` (amended) |
| Sign out in the account menu | The web server has no sign-out route in any mode | `plan/post-release/features/spec-fleet-ui-2-estate-tree-and-device-tables.md` (amended) |
| The devices table column chooser | Columns are fixed in the prototype | `plan/post-release/features/spec-fleet-ui-2-estate-tree-and-device-tables.md` ("The operator chooses which columns show") |
| An interface row's traffic chart, a BGP peer's capabilities, families, counters and state history, the streaming Logs tab, the console bound to a device | `DeviceSeries` takes no interface, `Peer` carries none of them, the console answers fixed sample commands | `plan/post-release/features/spec-fleet-ui-4-device-tabs-and-console.md` |
| Template preview and the YANG editor on a device's configuration | The source answers no template text; the editor belongs to the web package | `plan/post-release/features/spec-fleet-ui-6-config-rollout-audit-admin.md` |
| Stream metrics and the real stream's bounds | Deferred with the real event stream | `plan/post-release/features/spec-fleet-ui-5-tasks-alarms-notifications.md` |
| An audit entry's device list | The audit target is text | `plan/post-release/features/spec-fleet-3-audit-trail.md` |
| The link from a rollout's batches to their tasks | A rollout holds no task identifier | `plan/post-release/features/spec-fleet-5-staged-rollout.md` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fleet-ui-1-interactive-prototype-15269d3e-0e9f-48a2-918c-3e6749565424.md` |
| `./le spec review check` | clean, over the files commit A carries |
| Rounds | 2 |
| Reviewer lenses used | Round 1: wiring and security over the route registration, the hub imports, the auth wrap, the event stream, the request bounds, the panic traces, the shared guard changes, and documentation drift. Round 2: the anchor fixes and the spec amendments only |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | Three source anchors named symbols not declared where named: `TestDashboardLinkCountsEqualTargetRows` never existed in a committed `links_test.go`, and `buildActionHandler` and `buildAuditExportHandler` left `register.go` in `84008023ac` | `docs/architecture/fleet-interface.md:462`, `docs/architecture/web-interface.md:128` | Repointed to the four link tests, `newActionHandler` and `newAuditExportHandler` |

### Run 1 notes (no fix owed)
- `web.AccountMiddleware` (`internal/component/web/auth.go`) is an exported wrap that names an account with no authentication. Its one non-test caller is `fleetui.NewRoutes`, whose one caller is `internal/le/site/fleetdemo/render.go`; the hub mounts the fleet routes through `RegisterWebRoute` under `WrapAuth` and `WrapMutation` only.
- Every request input is bounded: filter 512 characters and 32 terms (`ParseFilter`), stream scope 2048 (`parseScope`), action and console bodies by `http.MaxBytesReader`, 64 subscriptions of 64 events each (`Feed.Subscribe`, `Feed.Publish`, which ends a lagging subscription), 10 search results (`searchView`). No `bugAddressKind` panic is reachable from a request: `parseObject` never answers `ObjectUnspecified` without an error.
- The two shared guard scope changes (`assetFindings` reading one directory, `golden.Set.Files` leaving a nested package) move no existing coverage: the web and looking-glass packages, their only other callers, hold no `.templ` or `page_assets.go` below their root, and the fleet package runs both guards over itself. No ze-test-judge verdict for them is recorded in a commit trailer.
- `./le test weakened check base 7380b7d204` reports no finding under the files of this spec.

### Run 2
- Scope: the two anchor repoints and the five spec edits. `./le doc check all` shows no undeclared anchor under a fleet page. 0 BLOCKER, 0 ISSUE.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `cmd/ze/hub/register_fleet_ui.go`, `register_fleet_ui_sample.go`, `build_tag_fleet_ui_{absent,dormant,present}_test.go` | Yes | `git ls-files cmd/ze/hub` |
| `test/web/fleet-{device-console,devices-filter,estate-to-failing-devices,live-updates,prototype-renders,shell-preferences,tree-selects-device}.wb` | Yes | `git ls-files test/web` |
| `docs/architecture/fleet-interface.md`, the twelve files of `fleetui/assets/` | Yes | `git ls-files` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 to AC-6 | Shell, pages, addresses, styles and states on the sample estate | `./le test unit pkg path ./internal/component/web/fleetui/` and the sample packages green (200 golden fixtures); the seven fleet browser tests green, on the final code |
| AC-7, AC-9 | Dormant without a source; no fleet symbol without the gate | The hub tag tests, owed again on the final code |
| AC-8 | No shipped flavor links the sample | `TestShippedZeLinksNoFleetSample`, `TestFleetSampleTagLinksTheSample` green on 2026-11-19 |
| AC-10 | Owner approval | Owner review record row 1 |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Sample build, browser opens `/fleet/` | `test/web/fleet-prototype-renders.wb` | Yes: green in the seven-case run |
| Standard build | `TestFleetRoutesAbsentWithoutSource` (`register_test.go`) | Yes: reads `sourceRegistered` over an empty slot |
| Every shipped flavour | `TestShippedZeLinksNoFleetSample` | Yes |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `TemplateFileParser.Parse` keeps the header lines; not load-bearing |
| A-2 | confirmed | `fleet-shell-preferences.wb` green |
| A-3 | broken | Mistake Log row 1 |
| A-4 | confirmed | `startWebServer` sets `InsecureMiddleware`; the review build opened `/fleet/` with no login |
| A-5 | confirmed | The asset check does not descend; the htmx check walks the fleet directory |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Fleet routes (`web-interface.md`) | `register.go` `init`, `handler_action.go` `newActionHandler`, `handler_audit.go` `newAuditExportHandler` | Yes, after finding 1 |
| Dashboard links (`fleet-interface.md`) | `links_test.go`, the four link tests | Yes, after finding 1 |
| Side panel behavior (`fleet-interface.md`, `PanelLink`) | `handler_shell.go` `panelLink`, `assets/fleet.js` `followPanel` | Yes |
| Feature list, user guide, CLI, config: No | The package is dormant in every shipped build; `features/fleet-management.md` changes when a page becomes functional | Yes |
