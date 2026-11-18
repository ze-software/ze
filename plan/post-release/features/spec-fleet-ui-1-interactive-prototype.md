# Spec: fleet-ui-1 -- Interactive Prototype: Shell, Components and Every Page on Sample Data

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | web, fleet |
| Priority | P3 |
| Release reason | Adds a capability absent from the supported release; existing supported behavior and its obligations remain required. |
| RFC gaps | - |
| Depends | - |
| Phase | 1/6 |
| Handoff | - |
| Updated | 2026-11-18 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

**Re-read after compaction:** this file, then `spec-fleet-ui-0-umbrella.md`
(the shell, URL scheme, tokens, component set and states this spec builds), then
the page descriptions in `spec-fleet-ui-2` to `spec-fleet-ui-6`.

## Task

Build the whole fleet interface as something the owner can open in a browser,
click through and correct, before any of the work that makes it act on a real
estate. The owner asked for this on 2026-11-18: "before you go deep in
development you provide me a mildly interactive interface I can give you
feedback on which you can later make functional".

The prototype is not a mock-up and is not thrown away. It is the real Fleet
mode: the real routes, shell, components, styles, htmx fragments and event
stream, exactly as `spec-fleet-ui-0-umbrella.md` fixes them. Only its data is
not real. The pages read through the command dispatcher, and in the prototype a
sample dispatcher answers those commands from recorded answers. Later children
replace the sample answers with the real commands, page by page, and keep the
markup.

This spec owns four things:

1. The Fleet web mode and its shell.
2. Every shared component in the umbrella's component set.
3. Every page of the set, as described in children 2 to 6, rendered on sample data.
4. The owner's review, and the corrections that follow from it.

### What the owner must be able to do in the prototype

| Area | Interaction |
|------|-------------|
| Shell | Switch between the light and dark themes; collapse, resize and pin the sidebar; switch between the Estate and Administration sidebar tabs; use the header search to jump to a device or a page |
| Tree | Expand and collapse branches; type in the filter box and see the tree narrow; change the tree view (by site then role, by role, by software version); select the estate, a group or a device; see the selected node stay marked and scrolled into view after a reload |
| Object pages | Open every tab of the estate, of a group and of a device; follow the breadcrumb upward |
| Tables | Sort by a column; type a filter and use the assisted filter menus; change the page size and move between pages; select rows one by one, by range and by page; see the bulk bar offer actions for the selection; open and close the side panel for a row |
| Dashboards | Read every card; follow each card's link to the table it counts; change the chart range between one hour, one day and one week |
| Device | Open an offline device and see the offline state; open a diverged device and see the divergence banner and its diff; open the Console tab and run the sample commands the prototype answers |
| Tasks and alarms | Open the task panel; watch a scripted task progress across several devices and finish with one failure; open its per-device results; see an alarm open and clear |
| Live updates | Watch a sample device go offline and return without reloading the page, in the tree, in the table and on the dashboard at once |
| Actions | Open the Actions menu of a device and of a selection; open the confirm dialog of a routine action and of a destructive one; confirm, and receive the toast saying the prototype changed nothing |
| States | Reach every state in the umbrella's states table through a labelled entry on a prototype-only page listing them, so each can be judged without having to provoke it |

### The sample estate

The sample answers describe one believable estate, large enough to exercise
grouping, pagination and counts, small enough to read.

| Property | Value |
|----------|-------|
| Devices | 44, over four sites and one group with no site label |
| Roles | edge, core, route server, customer premises |
| States | 36 healthy, 3 degraded, 1 down, 3 offline (one of them for six days), 1 diverged |
| Software | Three versions in use, one device two versions behind |
| Configuration | 39 in sync, 2 pending, 1 rejected, 1 diverged, 1 never fetched |
| History | One week of samples per device, with a visible daily pattern, one traffic spike and one gap where a device was offline |
| Alarms | 9 open: 1 critical, 5 warning, 3 informational |
| Tasks | 2 running, 1 failed, 12 finished in the last day |
| Rollouts | 1 paused at the second batch after a rejection, 1 complete |
| Labels | `site`, `role`, `vendor-model`, `customer`; three devices with no `site` |

The sample answers are the proposed answer shapes of the real commands. They
are handed to the `fleet` children as input to their design. A shape a `fleet`
child changes is changed in the samples in the same work
(`spec-fleet-ui-0-umbrella.md`, risk R-8).

### The owner's review

The review is part of this spec, not something after it.

1. The prototype is started in a development build on the owner's machine and its address is given to him.
2. He reviews it page by page. Each correction is recorded in this spec as a row: the page, what he said, what changes.
3. A correction to a shared rule (a token, a component, the shell, a state) is applied to `spec-fleet-ui-0-umbrella.md` in the same work, so the later children inherit it.
4. A correction to one page is applied to the child that owns that page.
5. The prototype is changed and shown again until he says it is right.
6. Only then do children 2 to 6 start.

### Key design decisions (from the umbrella and the scope gates)

| Decision | Detail |
|----------|--------|
| The seam is the dispatcher | The prototype differs from the functional interface only in which dispatcher answers. No page has a prototype branch |
| Compile tag | Every fleet page, route, asset and command is behind `ze_fleet_ui`, declared in `feature-gates.txt` and requiring `ze_web`. Always-on code reaches the fleet package only through registration, so a build without the tag compiles and has no Fleet mode |
| Development only | The sample dispatcher and its answers are behind a second tag, `ze_fleet_ui_sample`, which no shipped flavour sets. A test of every shipped flavour proves they are not linked |
| The web mode set becomes registered | The mode value set in `internal/component/web/ui_mode.go` is turned into a registered set, and Fleet registers with its enabling condition: a daemon serving managed clients, or the prototype form of a development build |
| Shared components carry no fleet knowledge | The tree, tab row, card, query bar, pagination, state hero and confirm dialog live in the web package and take plain view structs, so the workbench can use them later |
| Tabs and cards are registered entries | A device tab or a dashboard card is a registration, so a later feature adds one without editing the shell |
| Sidebar width without a style attribute | The shell script sets a custom property on the shell element when the edge is dragged; markup never carries a `style` attribute. If the content security policy refuses this, the fallback is three fixed widths chosen by class |
| No new third-party asset | Charts use the vendored uPlot. Icons are an inline-free sprite served from the assets directory |

### Draft acceptance criteria

To be completed at the design gate of this spec.

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A development build in prototype form | `/fleet/` opens the estate dashboard of the sample estate inside the shell | The other three web modes stay offered, because the prototype runs on a normal daemon |
| AC-2 | Each interaction in the table above | It works as described, on every page where it applies | Actions that would change the estate stop at a toast, because the prototype judges the interface |
| AC-3 | Every page of children 2 to 6 | The page exists, is reachable from the tree or the tabs, and renders in both themes | A page may show fewer sample rows than the real one would, because the sample estate is small |
| AC-4 | Any shipped build | The sample dispatcher is not linked, and no fleet route answers on a daemon serving no managed client | None |
| AC-5 | The owner's review | Every correction he states is recorded here and applied, and he states that the prototype is right | Corrections he defers are recorded as deferred with the child that will take them |

### Dependencies and readiness

| Dependency | State | Effect |
|------------|-------|--------|
| The web server, session, authorization and security headers | Available | Used as they are |
| `RegisterWebRoute` | Available | Used for every fleet route |
| The table and side panel components | Available | Extended with selection, URL sorting and pagination |
| The vendored uPlot and the L2TP chart script | Available | Generalized into the line chart component |
| The `fleet` set's commands | Missing | Not needed: the sample dispatcher answers |
| The contribution registry of `spec-web-plugin-extension-registry.md` | In design | Not needed: routes register directly, and move onto the registry when it exists |

### Owner review record

| # | Page | What the owner said | Change made | Applied to |
|---|------|---------------------|-------------|------------|
| | | | | |
