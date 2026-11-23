# Spec: fleet-ui-0 -- Fleet Web Interface (Umbrella)

| Field | Value |
|-------|-------|
| Status | design |
| Scope | web, fleet |
| Priority | P3 |
| Release reason | Adds a capability absent from the supported release; existing supported behavior and its obligations remain required. |
| RFC gaps | - |
| Depends | spec-fleet-0-umbrella.md |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-23 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The fleet work gives a hub remote control of the Ze devices it manages, but an
operator has nothing to see or manage that estate with: no view of which devices
exist, which are healthy, what each is running, or what the hub is doing to them.

This set specifies a web interface, served by the hub, for managing the whole
estate from one place. It keeps the existing web stack: server-rendered templ
markup, htmx fragments and server-sent events, with no client-side application
framework.

The interface follows the layout that operators of infrastructure managers
already know: a header, a sidebar holding a searchable tree of the estate, and a
main pane made of an object header, a row of tabs and the tab's content. The
intent is that an operator who has used such a tool finds every control where
they expect it, and that the result looks finished: consistent spacing, one
table pattern, one card pattern, clear states for loading, empty and failed.

This umbrella is the contract every child shares. It fixes the shell, the URL
scheme, the object model, the visual tokens, the component set and the states.
A child spec never redefines any of them: it names the pages it adds and the
components it composes.

### Decisions agreed with the owner (2026-11-18)

| Topic | Decision |
|-------|----------|
| Spec home | A sibling set `fleet-ui`, separate from the `fleet` set. `fleet-ui` owns every fleet web view. The `fleet` children keep the data, the protocol and the CLI, and their one-line web rows move here |
| Layout model | Header with a task button, a resizable sidebar with a searchable estate tree, an object header with a tab row, dashboard card grids, one table pattern with a side panel, light and dark themes |
| Colour | Ze's own colours, built on the theme variables the device web interface already defines |
| Density | Ze's existing density: 14px base text and the existing spacing and radius. The structure is the familiar one, the density is Ze's |
| Web mode | A fourth web mode, Fleet, offered only on a hub. The CLI, YANG editor and workbench modes are unchanged and remain what an operator sees for a single device |
| Estate tree | Built by grouping devices on flat key=value labels. No hierarchy is added to the data model |
| Device commands through the hub | In scope. The hub relays operator commands to a selected device over the existing managed channel (`spec-fleet-9-device-command-relay.md`) |
| Relay identity | A relayed command runs under an authorization profile that the device's own configuration names. With no such line the device refuses every relayed command and names the line that adds it. The hub user's name travels with the command for the device's audit log |
| Hub-side metric history | In scope, persisted and bounded: about a week of what devices report, kept across a hub restart (`plan/release/features/spec-fleet-8-metric-history.md`) |
| Browser CLI per device | In scope: the existing web CLI opened against a chosen device through the relay |
| Compile tag | The whole interface is one compiled-out feature under the tag `ze_fleet_ui`, declared in `feature-gates.txt` and dependent on `ze_web`. A build without the tag has no fleet route, no Fleet mode, no fleet asset and no fleet command. The prototype's sample dispatcher sits behind a second, development-only tag, `ze_fleet_ui_sample`, which no shipped flavour sets |
| Prototype first | The first deliverable is an interactive prototype of the whole interface on sample data, for the owner to review and correct. No child after `fleet-ui-1` starts until he has approved it. The prototype is built from the real shell, components and pages, so later children make it functional by replacing its data, never by rewriting it |

### Child specs

| Phase | Spec | Scope | Depends |
|-------|------|-------|---------|
| 1 | `spec-fleet-ui-1-interactive-prototype` (closed 2026-11-19, approved by the owner as built; the result is described in `docs/architecture/fleet-interface.md`) | The Fleet mode shell, the URL scheme, the visual tokens, every shared component and every page of this set, navigable in a browser on sample data. Ends with the owner's review; his corrections are applied here and to this umbrella | - |
| 2 | `plan/post-release/features/spec-fleet-ui-2-estate-tree-and-device-tables.md` | Tree views built from labels, the devices table, the filter language, saved filters, selection and bulk actions, the device side panel | fleet-ui-1 (approved), fleet-1 |
| 3 | `plan/post-release/features/spec-fleet-ui-3-dashboards-and-charts.md` | Estate, group and device dashboards, their cards and charts | fleet-ui-2, fleet-4, fleet-8 |
| 4 | `plan/post-release/features/spec-fleet-ui-4-device-tabs-and-console.md` | The device tabs that show live device state through the relay, and the per-device browser CLI | fleet-ui-2, fleet-9 |
| 5 | `plan/release/features/spec-fleet-ui-5-tasks-alarms-notifications.md` | The hub task model and its panel, the alarm list, toasts and the live event stream | fleet-ui-1 (approved), fleet-1, fleet-4 |
| 6 | `plan/post-release/features/spec-fleet-ui-6-config-rollout-audit-admin.md` | Per-device config view and diff, divergence resolution, rollouts, the audit trail and the administration pages | fleet-ui-2, fleet-ui-5, fleet-2, fleet-3, fleet-5, fleet-7 |
| 7 | `spec-fleet-ui-7-recorded-demonstration` (closed 2026-11-19) | A recorded demonstration of the interface on the sample estate, played from the website checkout with no daemon, written where the site does not publish it, for the owner's review. It is rendered on request by `./le site fleet-demo render`, never by the site build, into the output's unpublished `tmp/fleet-demo/` (`docs/contributing/gh-pages.md`, "Render the fleet demonstration") | fleet-ui-1 |
| 8 | `plan/release/features/spec-fleet-ui-8-alarm-causes.md` | Alarm causes: an alarm kind names the kinds that cause it in its own registration; an alarm whose cause is open on its device is hidden behind it, listed under its cause, and shown again with its original `Since` when the cause clears. Narrows `fleet-ui-5` AC-4 for a hidden alarm and adds the `has:cause` filter and the `hidden-behind` field of `show fleet alarm` | fleet-ui-5, fleet-1, fleet-4, fleet-8 |

Back-end work the interface needs, owned by the `fleet` set:

| Spec | What the interface needs from it |
|------|----------------------------------|
| `plan/release/features/spec-fleet-1-device-registry.md` | The persistent registry, labels, groups, `show fleet` commands. Every page reads it |
| `plan/release/features/spec-fleet-4-inventory-health.md` | Inventory and health per device. Dashboards, alarms and the System tab read it |
| `plan/release/features/spec-fleet-8-metric-history.md` | The sample reports and the bounded history every chart reads |
| `plan/post-release/features/spec-fleet-9-device-command-relay.md` | The relay every device tab and the per-device CLI use |
| `plan/post-release/features/spec-fleet-2-config-templates.md`, `spec-fleet-3-audit-trail.md`, `spec-fleet-5-staged-rollout.md`, `spec-fleet-7-config-reconnect-resolution` | The data behind the config, rollout and audit pages of `fleet-ui-6` |

### Prototype first, then functional

The pages read everything through the command dispatcher. That gives the
prototype a clean seam: in the prototype, a sample dispatcher answers the
`show fleet ...` commands and the relayed device commands from recorded sample
answers describing a believable estate (about forty devices over four sites,
with offline, degraded, down and diverged devices, running tasks, open alarms
and a week of history). Everything above that seam is the real thing: the
routes, the shell, the components, the styles, the htmx fragments and the live
event stream.

| Stage | What the operator can do | What is behind it |
|-------|--------------------------|-------------------|
| Prototype (`fleet-ui-1`) | Open every page, expand and filter the tree, sort and filter tables, select rows, open the side panel, switch tabs, change the chart range, switch theme, resize and pin the sidebar, open the task panel, see a scripted task progress and a device change state live, open confirm dialogs | Sample answers. Actions that would change something show their confirm dialog and then a toast saying the prototype changed nothing |
| Functional (`fleet-ui-2` to `fleet-ui-6`) | The same pages, acting on the real estate | The real commands of the `fleet` set, replacing the sample answers page by page |

The sample dispatcher is development tooling. It is compiled only into a
development build and is never part of a shipped binary. A page whose real
command does not exist yet keeps its sample answer in a development build and
shows the "not supported" state in a shipped one, so a half-finished page
never presents sample data as real.

### Object model

The interface shows three kinds of object. Each has a header, a tab row and a
default tab named Dashboard.

| Object | What it is | Where it comes from |
|--------|------------|---------------------|
| Estate | Every device the hub manages. One per hub, named after the hub | The registry of `fleet-1` |
| Group node | Every device sharing the label values of one tree branch, for example site `lon`, then role `edge` | Computed from labels by the tree view in use. Not stored |
| Device | One managed Ze instance | One registry record |

A device carries exactly one display state, chosen in this order of precedence:

| State | Shown when | Word | Colour variable |
|-------|-----------|------|-----------------|
| Offline | The device has no managed connection | Offline | `--text-muted` |
| Diverged | Connected, and holding a locally edited configuration (`fleet-6`) | Diverged | `--warning` |
| Down | Connected, and at least one health component reports down | Down | `--error` |
| Degraded | Connected, and at least one health component reports degraded | Degraded | `--warning` |
| Healthy | Connected, and every reported component is healthy | Healthy | `--success` |
| Unknown | Connected, and no health report has arrived yet | Unknown | `--text-secondary` |

The state is always written as its word beside an icon. Colour alone never
carries it.

### Shell

The shell has four regions. The wireframe gives their arrangement; the tables
after it give their contents.

```
+--------------------------------------------------------------------------------+
| [=] Ze Fleet   hub-lon-01        [ search devices, pages, tools ]  (3) Tasks  U |
+----------------------+---------------------------------------------------------+
| ESTATE | ADMIN  [pin]| (icon) edge-lon-04   Healthy   + Run diagnostic  [...] |
| [ filter tree    Q]  | Estate > lon > edge > edge-lon-04                      |
| v Estate    (41/44)  | Dashboard | Console | System | Interfaces | Routing | .. |
|   v lon      (12/12) +---------------------------------------------------------+
|     v edge    (4/4)  |                                                         |
|       o edge-lon-01  |   tab content: card grid, table, form or terminal       |
|       o edge-lon-04  |                                                         |
|     > core    (8/8)  |                                                         |
|   > ams      (9/10)  |                                              +---------+ |
|   > (no site) (2/2)  |                                              | side    | |
|                      |                                              | panel   | |
|    <drag to size>    |                                              +---------+ |
+----------------------+---------------------------------------------------------+
```

| Region | Contents, left to right or top to bottom |
|--------|------------------------------------------|
| Header (2.5rem high, the height of the existing workbench top bar) | Sidebar toggle; product name linking to the estate dashboard; hub name; search box; Tasks button with the count of running tasks; account menu holding the theme switch, the web mode switch and sign out |
| Sidebar (default 17rem, 12rem to 28rem, resizable by dragging its edge) | Two tabs, Estate and Administration, with a pin control at the right end of their row; a filter box, shown on the Estate tab; the tree. Unpinned, the sidebar overlays the content and closes after a selection. The width and the pin state are remembered per browser |
| Main pane | Object header: state icon, name, state word, a spinner while a task targets the object, an Actions menu. Breadcrumb line. Tab row. Tab content |
| Side panel (22rem, right edge of the main pane) | Details of the table row selected in the tab content. Its row is named in the URL, so the view can be bookmarked and shared |

Below 60rem of viewport width the layout takes one form: the sidebar overlays,
card grids become one column and the side panel covers the full width.

### URL scheme

Every view has a URL that reproduces it. All fleet routes sit under one prefix
that collides with no existing route.

| URL | View |
|-----|------|
| `/fleet/` | Estate dashboard |
| `/fleet/estate/<tab>/` | Estate tabs: `dashboard`, `devices`, `alarms`, `rollouts`, `tasks` |
| `/fleet/group/<key>=<value>/.../<tab>/` | Group node tabs: `dashboard`, `devices`, `alarms`, `tasks`. One path segment per tree level, each escaped once |
| `/fleet/device/<name>/<tab>/` | Device tabs: `dashboard`, `console`, `system`, `interfaces`, `routing`, `config`, `logs`, `tasks` |
| `/fleet/tasks/` | Every task, whatever its target |
| `/fleet/admin/<page>/` | Administration pages: `labels`, `tree-views`, `filters`, `access`, `audit`, `settings` |
| `/fleet/fragment/...` | htmx fragments. Never linked to directly |
| `/fleet/events` | The live event stream of the current view |

Query parameters carry view state: `q` the filter, `sort` the sort column and
direction, `page` and `size` the pagination, `id` the row shown in the side
panel, `range` the chart range, `view` the tree view.

### Visual tokens

The fleet shell uses the theme variables that `internal/component/web/assets/style.css`
already defines, in both themes, and adds only what is missing. No colour is
written as a literal in fleet styles.

| Purpose | Variable | Dark | Light |
|---------|----------|------|-------|
| Page background | `--bg-primary` | #0d1117 | #ffffff |
| Sidebar, header and card background | `--bg-secondary` | #161b22 | #f6f8fa |
| Hovered row, selected tab background | `--bg-surface` | #1c2128 | #f0f2f5 |
| Terminal and code background | `--bg-inset` | #010409 | #e8eaed |
| Text | `--text-primary` | #c9d1d9 | #1f2328 |
| Secondary text, captions | `--text-secondary` | #8b949e | #656d76 |
| Disabled text, Offline state | `--text-muted` | #484f58 | #8b949e |
| Brand, links, selected item, info | `--accent` | #58a6ff | #0969da |
| Selected row background | `--accent-subtle` | rgba(56,139,253,0.1) | rgba(9,105,218,0.1) |
| Borders | `--border` | #30363d | #d0d7de |
| Healthy | `--success` | #3fb950 | #1a7f37 |
| Degraded, Diverged, warning alarms | `--warning` | #d29922 | #9a6700 |
| Down, critical alarms, destructive actions | `--error` | #ff6b6b | #cf222e |

New variables this set adds, valid in both themes:

| Variable | Value | Use |
|----------|-------|-----|
| `--space-1` to `--space-6` | 0.25rem, 0.5rem, 0.75rem, 1rem, 1.5rem, 2rem | Every gap and padding in fleet styles is one of these six |
| `--fleet-sidebar-width` | 17rem by default | Set by the shell script when the operator drags the sidebar edge |
| `--chart-1` to `--chart-6` | `--accent`, `--success`, `--warning`, `--error`, #a371f7 dark and #8250df light, `--text-secondary` | Chart series, in order. Charts read them at draw time, so a theme change recolours them |

Typography and shape:

| Element | Rule |
|---------|------|
| Font | `--font-ui` for everything, `--font-mono` for identifiers, addresses, versions, hashes and terminal output |
| Base text | 14px, line height 1.5 |
| Object name in the header | 1.25rem, weight 600 |
| Card title, table header, form label | 0.8rem, weight 600, uppercase, letter spacing 0.04em, `--text-secondary` |
| Table cell, tree row | 0.85rem |
| Large figure on a card | 1.75rem, weight 600, tabular figures |
| Card | Background `--bg-secondary`, 1px `--border`, radius `--radius`, padding `--space-4`, gap `--space-3` between its parts |
| Card grid | 12 columns, gap `--space-3`, margin `--space-3` |
| Row heights | Tree row 1.75rem, table row 2.25rem, tab row 2.25rem |
| Focus | 2px `--border-focus` outline on every focusable element. Nothing is reachable by mouse only |

Appearance of the parts an operator recognises first. These were checked
against a screenshot of the reference layout on 2026-11-18 and are binding on
every page.

| Part | Appearance |
|------|------------|
| Header | The darkest surface of the page (`--bg-inset`), full width. Product name at the far left with the sidebar toggle beside it; at the far right the Tasks button as an icon carrying a small dot in the error colour when a task failed, then the account avatar. Nothing in the header is boxed except the search field |
| Sidebar tabs | Two labels in the card title style (uppercase, weight 600, letter spaced), side by side at the top of the sidebar. The active one has the `--bg-surface` background and a 2px `--accent` line under it. The pin control sits at the right end of the same row |
| Tree filter | A full-width field directly under the sidebar tabs, with its placeholder naming the tree and a search icon at its right edge |
| Tree row | Left to right: a chevron when the row has children; the object's icon with a small round state mark overlapping its lower right corner; the name; then, right aligned, a count in a small pill and a three-dot menu control. Thin vertical guide lines in `--border` join a parent to its children. The selected row is a rounded block in `--accent-subtle` spanning the sidebar's width |
| Object header | The object's icon and its name at the left, in the object name style; at the right, the object's primary action as a text link with a leading plus sign, and the Actions menu. A thin `--border` line separates it from the tab row |
| Tab row | Labels in the card title style at 0.85rem. The active tab has the `--bg-surface` background, `--text-primary` text and a 2px `--accent` line under it; the others are `--text-secondary`. A tab that opens outside the fleet mode carries a small external link mark |
| Card | One step lighter than the page (`--bg-secondary` on `--bg-primary`), with a 1px border and rounded corners. The title is at the top left in plain weight 600 text, not uppercase; a count or a short link (for example "Top 5", or a figure in the error colour) sits at the top right on the same line |
| Status donut | A thick ring with the object's icon in its centre. Beside it, a caption in the card title style naming what is counted, then one legend line per state: a round dot in the state's colour, the state word, the count. Under the ring, the caption TOTAL and the figure. Two donuts may share one card, stacked, with a thin line between them |
| Usage bar | Above the bar, a scale reading 0%, 50%, 100% in `--text-secondary`. The bar is a thin rounded track in `--bg-surface` with its filled part in `--accent`. Under it, right aligned, a dot, the name and the percentage. At the foot of the card, pairs of an uppercase caption and a large figure, for example TOTAL USED and TOTAL FREE |
| Table inside a card | Uppercase header labels in `--accent`, rows separated by thin lines, figures right aligned in the monospace font |
| Table section | A tab's content is one or more sections stacked in a single content card. Each section has its title in `--accent` at the top left with a full-width thin `--accent` rule under it, and its creating action at the top right (a text link with a plus sign, or a rounded button with a menu when there are several) |
| Query row | Under the section title: the caption "Filter", a wide text field with a placeholder inviting a query, a bordered Search button, and a text link opening the assisted filter builder |
| Pagination row | Right aligned, both above and below the table: "1 - 24 of 49 objects", then first, previous, next and last controls as double and single chevrons, then "Show by" and a small select |
| Table rows | Header labels uppercase in `--accent`. The first cell holds the object's icon and its name as an underlined link in `--accent`. A state cell is a small round icon in the state's colour followed by the state word. Rows are separated by thin lines, with no zebra striping |
| Row controls | The last column is fixed at the right edge while the others scroll under it, marked by a thin vertical `--accent` line: an eye control that opens the side panel, then a three-dot menu |
| Table actions row | Under the query row, when the table allows selection: the caption "Table actions", then each action as an icon and a word (for example Edit, Delete), dimmed while nothing is selected |
| Selection row | Directly above and below the table, left aligned: "2 selected of 44 objects", then the text links "Select all" and "Unselect all" |
| Header cell | A small icon naming the kind of value (text, state, address, number) before the uppercase label, both in `--accent`. The first header cell is the page's selection checkbox |
| Selected or opened row | The whole row carries the `--accent-subtle` band. It marks the row whose details are in the side panel |
| Side panel content | At its top right, the row's own actions as an icon and a word (Edit, Delete). Below, a stack of small cards, each with a title in weight 600 and rows of a label in `--text-secondary` and a value. A value that is an identifier or an address has a copy control at the row's right edge; a value that is a state has its round icon before the word; an absent value is a dash |
| Buttons | The action that runs the query is a filled button in `--accent`. Secondary actions are bordered buttons with an icon. Creating actions in a section header are bordered buttons with a leading plus sign |
| Side panel | Unpinned by default: with no row open it is absent and the table takes the full width, and a row's eye opens it over the right edge of the content, so a reload of that address reproduces it. A pinned placeholder cost a 22rem column on every table page with nothing in it, which pushed the last columns of the devices table under its fixed controls. The pin control at its top right pins it; pinned, it stays beside the table and with no row chosen shows a centred sentence, "Click the eye to see details", over an illustration. The pin is remembered per browser and applied before the first paint |
| Task and event rows | A list, not a grid: an expand chevron when the row has parts, the name, a small round result mark, a count pill of its parts, then right aligned "Ended 59 minutes ago" in `--text-secondary`, a larger round result indicator, and the eye control |
| Quick info card | Label and value pairs in four columns. Labels are `--text-secondary`, values `--text-primary`. A value that is a state carries its icon; a value that is another object is that object's icon and a link; labels shown as tags are small filled chips |
| Callout inside a card | A box with a 1px border in the warning colour, a round warning icon at its left, a first line in weight 600 and a second line that may hold a link. Used when a card must tell the operator to act |
| Header actions of an object | At the right of the object header: an optional text link, then the main action as a rounded bordered button with a menu chevron, then a three-dot menu for the rest |
| Count beside a title | A small filled round badge in the error colour holding the number, directly after the card title, when the card lists things needing attention |
| "See all" | A small underlined link at the top right of a card whose content is a summary of a longer list |
| Light theme | The page is the light grey of `--bg-secondary`, cards and the sidebar are white (`--bg-primary`), and the header is the same light grey with a thin line under it. Borders are fine and pale. The dark theme inverts the relationship: the page is darkest and cards one step lighter |
| Empty list on a dashboard | A centred illustration with two short lines in `--success`, for example "All good" and "No alarms open". An empty list that is good news says so; it is not left blank and not styled as an error |

### Component set

Each component is one templ component taking one named view struct, with no
inline script or style. Components marked reuse exist today and are used as
they are or extended in place.

| Component | New or reuse | What it shows | Used by |
|-----------|--------------|---------------|---------|
| Fleet shell | New | The four regions and the page asset set | Every fleet page |
| Estate tree | New | Nested rows with a chevron, a state icon, a label, a count or badge and a row menu; keyboard movement with the arrow keys | Sidebar |
| Object header | New | State icon, name, state word, task spinner, Actions menu | Every object page |
| Tab row | New | The tabs of the current object, the active one marked, each a link | Every object page |
| Card grid and card | New | A titled box in the 12-column grid, with a header link to the table behind it | Dashboards |
| Status donut | New | A ring split by state with a legend of word, count and link | Dashboards |
| Usage bar | New | One labelled bar with value and maximum; turns `--warning` at 80% and `--error` at 90% | Dashboards, System tab |
| Stacked bar | New | One bar split by category with a legend | Dashboards |
| Figure tile | New | One large figure with a caption and an optional trend line | Dashboards |
| Line chart | Extends the L2TP quality chart | A time series drawn with the vendored uPlot, with a range selector | Dashboards, device tabs |
| Query bar | New | A filter text box, assisted filter menus that write into it, a saved filter menu, the count of matches | Every table |
| Bulk bar | New | The number of selected rows and the actions valid for them | Tables with selection |
| Table | Reuse (`WorkbenchTableData`) | Columns, sortable headers, flags, cells, row actions, empty state with headers visible | Every table |
| Pagination | New | First, previous, next, last, "x to y of z", page size 25, 50 or 100 | Every table |
| Side panel | Reuse (`WorkbenchDetailData`) | Tabs, key and value rows, tools, a close link | Tables |
| Key and value list | Reuse (`component_detail_kv.templ`) | Labelled values, each with a copy control where the value is an identifier | Side panel, System tab |
| Tag | New | One label as `key=value`, removable where the operator may edit it | Tables, side panel |
| State hero | New | A centred icon, a sentence and one action, for the states in the table below | Every pane |
| Toast | Extends `notification.js` | A short message with a level and an optional link | Shell |
| Confirm dialog | New | The action, its targets and a confirm control; a destructive action also asks the operator to type the target's name or the count | Destructive actions |
| Task panel | New | Tasks by state with counts, each with progress and a link | Header |
| Terminal | Reuse (`component_cli_terminal.templ`) | The web CLI | Console tab |
| Diff view | Reuse (`component_diff_modal.templ`) | A commit-style diff | Config tab |

### States every pane must show

A pane never renders blank. Each of these states is a State hero or a table
empty state naming what is missing and the next action.

| State | Sentence shape | Action offered |
|-------|----------------|----------------|
| Loading | Skeleton rows or a skeleton card, never a spinner alone | None |
| No data | "No devices match this filter" | Clear the filter |
| Nothing configured | "No devices are declared on this hub" | Link to the administration page that adds one |
| Device offline | "edge-lon-04 has been offline since 14:02. The values below were reported then" | Open the device's tasks |
| Not supported by the device | "This device runs a version that does not answer this view" | Show the device's version |
| Relay refused | The device's own refusal, word for word, including the configuration line that grants the relay | Copy the line |
| Unauthorized | "Your profile does not allow this view" with the profile name | None |
| Failed | What failed and the command that shows the same data at the CLI | Retry |
| Stale | A banner over the content: "Live updates stopped at 14:02" | Reconnect |

## Required Reading

### Architecture Docs
- [ ] `plan/design/web-operator-workbench-definition-of-good.md` - the owner's rulings on the web interface
  → Decision: rule 0 named three peer web modes. The owner agreed on 2026-11-18 to a fourth, Fleet, on a hub; the page is amended in this work
  → Constraint: named data defaults to a table; status uses words plus restrained indicators, never colour alone
  → Constraint: the browser submits tool identifiers and context, never raw operational command text, outside the CLI mode
  → Constraint: a design that renders unbounded data in one page, shows stale state as live, or makes a destructive action look routine is rejected
  → Constraint: "A dashboard that hides configuration detail behind vague health cards" is named as not good, so every card carries a count and a link to the table behind it
  → Constraint: a feature-owned page is registered by its owner, never added to a central switch
- [ ] `docs/architecture/web-interface.md` - routes, authentication, security headers, the event broker
  → Constraint: new routes must not collide with `/show/`, `/fragment/`, `/config/`, `/cli`, `/admin/`, `/tools/`, `/login`, `/assets/`, `/events`, `/l2tp/`, `/ospf`; the `/fleet/` prefix is free
  → Constraint: the content security policy sets `style-src 'self'` and `script-src 'self'`, so no `style` attribute and no inline script appears in markup
  → Constraint: every mutation passes the same-origin wrapper and the edit authorization check that existing routes use
  → Constraint: the event broker has no topics: a broadcast reaches every client, so per-view data must not use it
- [ ] `docs/architecture/web-components.md` and `docs/architecture/web-workbench-pages.md` - the reusable components
  → Decision: tables reuse `WorkbenchTableData` and the side panel reuses `WorkbenchDetailData`
  → Constraint: no tree, tab row, card grid or query bar component exists; they are new and shared
  → Decision: a per-subject live feed follows the L2TP quality feed, a stream per connection, because a broadcast "would leak one subscriber's data to another"
  → Decision: chart colours are custom properties read at draw time, as the L2TP chart does
- [ ] `plan/post-release/features/spec-web-plugin-extension-registry.md` - how a feature registers its interface
  → Constraint: its contribution registry is still in design and does not exist; what exists is `RegisterWebRoute`, which the fleet pages use
  → Constraint: a second page registry beside it is refused; when that spec lands, the fleet pages move onto it
- [ ] `docs/architecture/fleet-config.md` - the managed channel
  → Constraint: four verbs exist, all about configuration; the hub can call a device, and the device answers hub requests one at a time
  → Constraint: `pkg/fleet` is public API, so a new verb is a contract change made with that page
- [ ] `ai/patterns/web-endpoint.md` - what a new page owes
  → Constraint: a handler file, a templ component with a named view struct, route registration, a JSON form of the same data, and tests under `test/web/`

**Key insights:**
- The hub today distributes configuration and nothing else. Every fleet page depends on `fleet-1`, and most on `fleet-4`, `fleet-8` or `fleet-9`.
- Reuse covers tables, side panel, key and value lists, terminal, diff and themes. The tree, the tab shell, the card grid and the query bar are the new shared components.
- All state is read through the command dispatcher (`show fleet ...`), so every page has a CLI equivalent and the same authorization.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/web/ui_mode.go` - selects the web mode from `ze.web.ui-mode` and the `ze-ui-mode` cookie; knows `finder` and `workbench`
- [ ] `internal/component/web/webroute.go` - `RegisterWebRoute` lets a package register a route with its wrapper, builder and enabling condition
- [ ] `internal/component/web/workbench_table.go` and `workbench_detail.go` - the table and side panel view structs
- [ ] `internal/component/web/workbench_sections.go` - the two-level workbench navigation, listed by hand
- [ ] `internal/component/web/sse.go` - the event broker: 100 clients, 16 buffered events each, broadcast or one user
- [ ] `internal/component/web/auth.go` - session cookie and security headers
- [ ] `internal/component/web/page_system.go` - `CollectFleetPeers` renders a selector of links to other devices' own addresses
- [ ] `internal/component/web/assets/style.css` - the theme variables, in a dark default and a light override
- [ ] `internal/component/plugin/server/managed_serve.go` - the hub side of the managed channel: connection set in memory, a cap of 128 held connections
- [ ] `internal/component/managed/client.go` - the device side: a serial loop for hub requests, unknown verbs dropped without an answer
- [ ] `pkg/fleet/envelope.go` - the four verbs

**Behavior to preserve:**
- The CLI, YANG editor and workbench modes, their routes and their appearance, on a device and on a hub
- The session, authorization and audit path of the existing web server
- The managed channel's four verbs and their meaning
- The existing theme variables and their values
- The selector of links to other devices' own addresses, which stays in the workbench

**Behavior to change:**
- A hub offers a fourth web mode, Fleet, under `/fleet/`
- The web mode value set gains `fleet`, accepted only on a hub
- The one-line web views listed in the `fleet` children are specified here instead

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Web route registration | `internal/component/web/webroute.go`, `RegisterWebRoute` | A package registers a route, its wrapper, its builder and the condition that enables it | Extends: every fleet route registers through it, enabled only when the daemon serves managed clients |
| Web mode selection | `internal/component/web/ui_mode.go`, `GetUIMode` | Chooses between the YANG editor and the workbench | Extends: gains the value `fleet`, valid on a hub |
| Command dispatcher | `internal/component/plugin/dispatch.go`, `CommandDispatcher` | Runs a command for a caller with authorization and audit; every web read uses it | Extends: fleet pages read `show fleet ...` through it; the relay of `fleet-9` supplies a dispatcher of the same type bound to one device |
| Workbench table and side panel | `internal/component/web/workbench_table.go`, `workbench_detail.go` | The table, its flags, row actions and empty state; the detail panel with tabs and tools | Extends: used for every fleet table and side panel; gains selection, sorting by URL and pagination where missing |
| Theme variables | `internal/component/web/assets/style.css`, `:root` and the light override | Colours, fonts and radius in two themes | Extends: used unchanged; spacing and chart variables are added |
| Chart drawing | `internal/component/web/assets/l2tp-chart.js` with the vendored uPlot | Draws the L2TP quality series, reading colours from custom properties | Extends: generalized to a line chart component fed by a data attribute |
| Per-connection live feed | `internal/component/web/handler_l2tp.go`, the samples stream | Streams one subject's data to one connection on a ticker | Extends: the fleet event stream is per connection and scoped to the view |
| Event broker | `internal/component/web/sse.go`, `EventBroker` | Broadcasts configuration and log events to every client | Does not apply to per-view data: a broadcast would send one view's devices to every client. Still used for hub-wide notices |
| Toasts | `internal/component/web/assets/notification.js` | Shows notices | Extends: gains levels and an optional link |
| Related tools | The `ze:related` extension and `/tools/related/run` | Resolves a tool identifier and a context into a command on the server | Extends: device row and header actions post tool identifiers; the resolved command runs through the relay |
| Web CLI | `internal/component/web`, `component_cli_terminal.templ` and its handlers | A terminal in the browser against the local daemon | Extends: opened with a relay dispatcher for one device (`fleet-ui-4`) |
| Contribution registry for workbench pages | `plan/post-release/features/spec-web-plugin-extension-registry.md` | In design; not built | Does not apply yet: fleet pages use `RegisterWebRoute` now and move onto the registry when it exists |
| Selector of other devices' addresses | `internal/component/web/page_system.go`, `CollectFleetPeers` | Links to each device's own interface from configured addresses | Does not apply: it links out and holds no state. The fleet device header offers the same link when an address is configured |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A browser request to `/fleet/...` on the hub's web server, carrying the existing session cookie
- A live event from the hub: a device changes state, a task progresses, an alarm opens or clears

### Transformation Path
1. The web server authenticates the session and authorizes the route, as for every existing route.
2. The fleet handler parses the URL into an object (estate, group node or device), a tab and view state.
3. The handler reads data through the command dispatcher: registry, inventory, health, alarms, tasks and history through `show fleet ...` on the hub; live device state through the relay dispatcher bound to the device.
4. The handler builds a named view struct and renders the shell, or a fragment when htmx asked for one.
5. The browser opens the event stream for its view; the hub writes an event when something in that view changes, and htmx swaps the fragment the event names.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Browser to hub web server | HTTPS, session cookie, htmx requests and server-sent events | No |
| Fleet pages to hub fleet state | The command dispatcher, `show fleet ...` with JSON answers | No |
| Hub to device | The managed channel, through the relay verbs of `fleet-9` | No |
| Fleet pages to hub configuration | The existing per-user editor and commit path, for labels, tree views and device configuration | No |

### Changed-Wire Census

This umbrella changes no wire. It specifies markup, URLs and the composition of
pages over commands. The wires the interface depends on are changed and
censused in the specs that own them: the report and history verbs in
`plan/release/features/spec-fleet-8-metric-history.md`, the relay verbs in
`plan/post-release/features/spec-fleet-9-device-command-relay.md`, and the
`show fleet` answer shapes in
`plan/release/features/spec-fleet-1-device-registry.md` and
`plan/release/features/spec-fleet-4-inventory-health.md`.

**Search evidence:** `pkg/fleet/envelope.go` was read whole and holds four verb constants (`config-fetch`, `config-changed`, `config-ack`, `ping`); a search of `internal` and `pkg` for `fleet` in YANG files returns nothing, so no fleet command or answer shape exists yet for this set to change. Positive control: the same search for `managed` finds the client and server packages.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Managed channel verbs | writer | `internal/component/plugin/server/managed_serve.go`, `pushConfigChanged` | Read: sends `config-changed` with a 10 second deadline | Preserved unchanged by this set; extended by `fleet-8` and `fleet-9` |
| Managed channel verbs | reader | `internal/component/managed/client.go`, `handleHubRequest` | Read: handles hub requests in one loop, drops unknown verbs | Preserved unchanged by this set; changed by `fleet-9` |

### Integration Points
- `RegisterWebRoute` - the fleet package registers its routes and their enabling condition
- `GetUIMode` - learns the `fleet` value
- `CommandDispatcher` - every read and every action
- `WorkbenchTableData`, `WorkbenchDetailData` - table and side panel
- The per-user editor and commit bar - label and configuration edits

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Every read is a command through the dispatcher; no fleet page imports the managed server package |
| No unintended coupling (components stay isolated) | Yes | Shared components carry no fleet knowledge; fleet pages live in their own package |
| No duplicated functionality (extends existing, does not recreate) | Yes | The Existing Mechanisms table names what each part extends |
| Zero-copy preserved where applicable (refs, not copies) | Yes | Not applicable: no wire encoding path is touched |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | Routes register through `RegisterWebRoute`; tabs and cards are registered entries, so a later feature adds a device tab or a card without editing the shell |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | No | One list must learn the name: the web mode value set in `ui_mode.go`. `fleet-ui-1` turns it into a registered set so the fourth mode is the last one added by an edit |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The registry, inventory and health data of `fleet-1` and `fleet-4` are reachable as `show fleet ...` commands with JSON answers | Their task statements name CLI and web views | Pages have no data source and must wait or read state another way | The wiring tests of `fleet-ui-1` and `fleet-ui-3` | unvalidated |
| A-2 | A script may set a custom property on an element through the object model under the current content security policy | The policy restricts markup attributes and inline blocks, not script-driven style changes | The sidebar cannot be resized by dragging and falls back to three fixed widths | A browser test in `fleet-ui-1` that drags the edge and reads the width | unvalidated |
| A-3 | An estate is at most a few thousand devices | `fleet-0` targets 10 to 1000 instances | The tree and the status counts need server-side windowing beyond what is specified | A page test with 2000 devices in `fleet-ui-2` | unvalidated |
| A-4 | A device and its hub may run different Ze versions | Devices are upgraded in stages | If versions are always equal, the "not supported by the device" state is dead code | Owner confirmation at the `fleet-ui-4` design gate | unvalidated |
| A-5 | Operators accept Ze's density in a layout they know from a more spacious tool | Owner decision of 2026-11-18 | Pages feel cramped and a comfortable density setting is added | Owner review of the first rendered pages of `fleet-ui-1` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The interface is specified ahead of its data: `fleet-1`, `fleet-4`, `fleet-8` and `fleet-9` are skeletons | A child reaches implementation with its dependency undesigned | The prototype needs none of them. Each later child names its dependency and is not started before it |
| R-8 | The sample answers and the real command answers diverge, so a page that worked in the prototype breaks when made functional | A `fleet` child designs an answer shape unlike the sample | The sample answers of `fleet-ui-1` are the proposed answer shapes, handed to the `fleet` children as input to their design; a shape they change is changed in the samples in the same work |
| R-9 | Sample data reaches an operator as if it were real | A shipped build renders a sample device | The sample dispatcher is behind a development-only build tag, and a test of every shipped flavour proves it is not linked |
| R-2 | Children drift apart visually | A child adds a colour, a spacing value or a component of its own | The token and component tables here are closed lists; a child that needs more amends this umbrella first. A style test refuses a literal colour in fleet styles |
| R-3 | A dashboard becomes decoration | A card shows a figure with nothing behind it | Every card carries a link to a filtered table; the card component refuses to render without one |
| R-4 | One slow or offline device stalls a page that shows many | An estate page takes as long as its slowest device | Estate and group pages read only hub-held state. Only device pages use the relay, each card or tab loading on its own with its own failure state |
| R-5 | A view's live events reach another operator who may not see those devices | An event stream carries a device outside the operator's view | The stream is per connection and filtered to the view and to the operator's authorization; no fleet data uses the broadcast broker |
| R-6 | A tree with thousands of rows is slow to render and to filter | The tree fragment exceeds a few hundred kilobytes | Branches below the second level load on expansion; the filter runs on the server |
| R-7 | The fourth mode confuses an operator on a plain device | The mode switch offers Fleet where no hub runs | The mode is registered with an enabling condition and is absent from the switch on a device |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing on a device. On a hub, the fleet pages mislead or fail; the three existing modes and the managed channel are untouched |
| How is it reverted? | The fleet package is one registration; removing it removes every fleet route. The shared components stay harmless |
| Who else touches this path? | `spec-web-plugin-extension-registry.md` (page registration), the `fleet` children (data), the session working on fleet configuration fixes |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Development build started in prototype form, browser opens `/fleet/` | → | Fleet shell renders the sample estate | `test/web/fleet-prototype-renders.wb` |
| Browser opens `/fleet/` on a hub | → | Fleet shell renders with the estate tree | `test/web/fleet-shell-renders.wb` |
| Browser opens `/fleet/` on a daemon that serves no managed client | → | The route is absent and the mode switch omits Fleet | `test/web/fleet-absent-on-device.wb` |
| Operator selects a device in the tree | → | Device page renders its header and tab row | `test/web/fleet-tree-selects-device.wb` |
| Operator filters the devices table | → | The table shows only matching rows and the URL carries the filter | `test/web/fleet-devices-filter.wb` |
| A device goes offline | → | Its tree row and table row change state without a reload | `test/web/fleet-live-device-state.wb` |
| Operator opens a device's Console tab | → | A command typed there runs on that device | `test/web/fleet-device-console.wb` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A hub with managed devices; an operator signs in | The Fleet mode is offered and opens on the estate dashboard, inside the shell described here | The CLI, YANG editor and workbench modes stay offered: they remain the interface to the hub itself |
| AC-2 | Any fleet page | The page is reproduced by its URL, including filter, sort, page, selected row, chart range and tree view | Sidebar width and pin state stay per browser, not in the URL: they are a preference, not a view |
| AC-3 | Any fleet page, in both themes | Every colour comes from a theme variable and every gap from the spacing variables | The L2TP colour variables stay as they are: they belong to another feature |
| AC-4 | Any pane with no data, no permission, an offline device or a failed read | The pane shows the matching state from the states table, naming what is missing and the next action | Skeleton placeholders remain permitted while loading, because they are replaced as soon as data arrives |
| AC-5 | Any device state shown anywhere | The state is written as its word beside its icon | Colour remains as a second cue, because it speeds scanning for operators who see it |
| AC-6 | A page showing many devices | The page reads hub-held state only and renders in bounded time whatever the devices' reachability | Device pages still depend on the device, because they show live device state |
| AC-7 | A viewport narrower than 60rem | The sidebar overlays, grids become one column and the side panel takes the full width; every control stays reachable | No further breakpoints, because one switch is what the layout defines |
| AC-8 | Keyboard only | Every control is reachable and operable, with a visible focus outline | Dragging the sidebar edge stays pointer-only, because the width is also adjustable from the keyboard on the handle |
| AC-9 | An operator whose profile does not allow a view or an action | The server refuses it and the page shows the unauthorized state; the control is not shown | A hidden control remains refused on the server, because hiding is not authorization |
| AC-10 | A daemon that serves no managed client | No fleet route answers and the mode switch omits Fleet | The three existing modes stay, unchanged |
| AC-13 | A build without the `ze_fleet_ui` tag | The build compiles in every flavour, no fleet route, mode, asset or command exists, and always-on code imports nothing from the fleet package | The shared components in the web package remain, because they carry no fleet knowledge and the workbench may use them |
| AC-11 | A development build started in prototype form | Every page of this set opens and responds to the interactions listed for the prototype stage, on the sample estate | Actions that would change the estate stop at a toast, because the prototype exists to judge the interface, not to act |
| AC-12 | Any shipped build | The sample dispatcher and its sample answers are not linked | None: sample data in a shipped build would be shown as real |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Opens the hub, sees the estate at a glance, and finds the devices that need attention | Session, fleet shell, estate dashboard cards, link to the filtered devices table | `test/web/fleet-estate-to-failing-devices.wb` |
| 2 | Finds one device by typing part of its name and opens it | Tree filter, device page | `test/web/fleet-tree-selects-device.wb` |
| 3 | Checks a device's peers and interfaces without leaving the hub | Device tabs, relay dispatcher, device | `test/web/fleet-device-routing-tab.wb` |
| 4 | Runs a command on a device from the browser | Console tab, relay dispatcher, device | `test/web/fleet-device-console.wb` |
| 5 | Labels a set of devices and sees the tree regroup | Devices table selection, bulk action, hub editor, commit, tree | `test/web/fleet-bulk-label.wb` |
| 6 | Watches an operation across many devices finish or fail | Action, task, task panel, live events | `test/web/fleet-task-progress.wb` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestFleetURLRoundTrip` | `internal/component/fleetui/url_test.go` | A parsed fleet URL renders back to the same URL | |
| `TestDeviceDisplayStatePrecedence` | `internal/component/fleetui/state_test.go` | The display state follows the precedence table | |
| `TestFleetStylesUseVariablesOnly` | `internal/component/fleetui/style_test.go` | Fleet styles hold no literal colour and no spacing outside the six spacing variables | |
| `TestFleetModeEnabledOnlyOnHub` | `internal/component/web/ui_mode_test.go` | The fleet mode is offered only when its enabling condition holds | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Page size | 25, 50, 100 | 100 | Any other value falls back to 50 | Any other value falls back to 50 |
| Sidebar width | 12rem to 28rem | 28rem | Clamped to 12rem | Clamped to 28rem |
| Page number | 1 to last page | Last page | 0 shows page 1 | Beyond the last page shows the last page |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `fleet-prototype-renders` | `test/web/fleet-prototype-renders.wb` | In prototype form, every page opens on the sample estate | |
| `fleet-shell-renders` | `test/web/fleet-shell-renders.wb` | The operator opens the hub and sees the shell and the tree | |
| `fleet-absent-on-device` | `test/web/fleet-absent-on-device.wb` | On a device, no fleet page or mode exists | |
| `fleet-tree-selects-device` | `test/web/fleet-tree-selects-device.wb` | Selecting a device opens its page | |
| `fleet-devices-filter` | `test/web/fleet-devices-filter.wb` | Filtering narrows the table and survives a reload | |
| `fleet-live-device-state` | `test/web/fleet-live-device-state.wb` | A state change appears without a reload | |
| `fleet-device-console` | `test/web/fleet-device-console.wb` | A command runs on the chosen device | |
| `fleet-estate-to-failing-devices` | `test/web/fleet-estate-to-failing-devices.wb` | A dashboard card leads to the devices it counts | |
| `fleet-device-routing-tab` | `test/web/fleet-device-routing-tab.wb` | A device's peers are shown through the hub | |
| `fleet-bulk-label` | `test/web/fleet-bulk-label.wb` | A bulk label edit regroups the tree after commit | |
| `fleet-task-progress` | `test/web/fleet-task-progress.wb` | A task's progress and result are visible as they happen | |

## Files to Modify
- `feature-gates.txt` - declares `ze_fleet_ui` (requires `ze_web`) and the development-only `ze_fleet_ui_sample`
- `internal/component/web/ui_mode.go` - the web mode set becomes registered and gains `fleet`
- `internal/component/web/workbench_table.go` - selection, URL sorting and pagination fields where missing
- `internal/component/web/assets/style.css` - the spacing and chart variables
- `internal/component/web/assets/notification.js` - levels and an optional link
- `plan/design/web-operator-workbench-definition-of-good.md` - four modes on a hub
- `docs/architecture/web-interface.md` - the fleet routes and the fourth mode
- `docs/architecture/web-components.md` - the new shared components
- `plan/post-release/features/spec-fleet-0-umbrella.md` - children 8 and 9, web views moved here

## Files to Create
- `internal/component/web/component_tree.templ`, `component_object_header.templ`, `component_tab_row.templ`, `component_card.templ`, `component_query_bar.templ`, `component_pagination.templ`, `component_state_hero.templ`, `component_confirm.templ` - shared components, free of fleet knowledge
- `internal/component/web/assets/shell.js`, `chart.js` - sidebar sizing and pinning; the line chart
- `internal/component/fleetui/` - the fleet pages, their handlers, view structs and route registration
- `internal/component/fleetui/sample/` - the sample dispatcher and its recorded answers, behind a development-only build tag
- `test/web/fleet-*.wb` - the functional tests named above
- `docs/guide/fleet-interface.md` - the operator guide
- `docs/architecture/fleet-interface.md` - the design this umbrella fixes, once built

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | Tree views and saved filters, declared in `fleet-ui-2`; no schema in this umbrella itself |
| YANG validation constraints | Yes | In the children that add leaves |
| YANG custom validators | N-A | No leaf here needs more than native constraints |
| CLI commands/flags | Yes | `show fleet task` and `show fleet alarm` in `fleet-ui-5`; every other read uses commands the `fleet` set owns |
| CLI grammar (keyword before value) | Yes | In the children that add commands |
| Editor autocomplete | N-A | Automatic for the leaves added |
| Functional test for new RPC/API | Yes | The `.wb` tests above and the `.ci` tests of the children |
| Pipe completeness | Yes | Every fleet read is a command answering data, so pipes apply |
| Env var registration | Yes | `ze.web.ui-mode` gains the value `fleet`; no new variable |
| Doctor check for runtime dependencies | N-A | The interface adds no path, socket, port or binary: it is served by the existing web listener |
| Doctor code per problem class | Yes | A hub whose web mode is set to `fleet` while it serves no managed client is reported by a check in `fleet-ui-1` |
| Prometheus counters/metrics | Yes | Open fleet event streams and fleet page render failures, defined in `fleet-ui-1` |
| BGP family surface (new SAFI / capability / attribute) | N-A | No BGP wire surface |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/fleet-management.md` gains the interface in its scope |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md`, in the children that add leaves |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md`, in `fleet-ui-5` |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md`, in `fleet-ui-5` |
| 5 | Plugin added/changed? | No | No plugin is added |
| 6 | Has a user guide page? | Yes | `docs/guide/fleet-interface.md` |
| 7 | Wire format changed? | No | Owned by `fleet-8` and `fleet-9` |
| 8 | Plugin SDK/protocol changed? | No | The managed channel is not the plugin protocol |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No RFC behavior |
| 10 | Test infrastructure changed? | No | The existing browser suite is used as it is |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` gains fleet management interface |
| 12 | Internal architecture changed? | Yes | `docs/architecture/web-interface.md`, `docs/architecture/web-components.md`, `docs/architecture/fleet-interface.md` |
| 13 | Route metadata keys added/changed? | No | None |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/guide/status.md` for the new commands |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived at implementation with `./le spec citation anchors` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/fleet-config.md` and `docs/guide/web-interface.md`, checked against the code |

## Implementation Steps

Each phase is a child spec. A child is designed with the owner before it is
implemented, and is not started before the specs it depends on.

1. **Phase: Interactive prototype (fleet-ui-1)** -- the mode, the shell, the tokens, the shared components and every page, on sample data
   - Tests: `fleet-prototype-renders`, `fleet-absent-on-device`, `fleet-tree-selects-device`, the four unit tests above
   - Files: `internal/component/fleetui/`, the shared component files, `ui_mode.go`, `style.css`, the sample dispatcher and its answers
   - Verify: the owner opens the prototype, reviews every page and states his corrections; they are applied and this umbrella is amended. No later phase starts before his approval
2. **Phase: Tree views and device tables (fleet-ui-2)** -- label grouping, the devices table, filters, bulk actions, side panel
   - Tests: `fleet-devices-filter`, `fleet-bulk-label`
   - Verify: an operator finds any device by name, label, state or version
3. **Phase: Dashboards and charts (fleet-ui-3)** -- the three dashboards
   - Tests: `fleet-estate-to-failing-devices`
   - Verify: every card leads to the rows it counts; charts draw a week of history
4. **Phase: Device tabs and console (fleet-ui-4)** -- live device state and the per-device CLI
   - Tests: `fleet-device-routing-tab`, `fleet-device-console`
   - Verify: a device behind address translation is inspected from the hub
5. **Phase: Tasks, alarms and notifications (fleet-ui-5)** -- the task panel, the alarm list, toasts, live events
   - Tests: `fleet-task-progress`, `fleet-live-device-state`
   - Verify: nothing the hub does to a device is invisible
6. **Phase: Config, rollout, audit and administration (fleet-ui-6)** -- the remaining fleet views
   - Verify: every fleet capability with a CLI command has a page
8. **Phase: Alarm causes (fleet-ui-8)** -- an alarm hidden behind its open cause, listed under it, shown again when the cause clears
   - Tests: `fleet-alarm-cause`, `alarm-hidden-behind-offline`
   - Verify: a device that goes offline shows one alarm, and its other alarms return with it

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every child's pages use only the components and tokens listed here |
| Feature completeness | Every user story has a working path across the children |
| Correctness | A device's display state is the same in the tree, every table, every card and the object header |
| Naming | URL segments and tab names match the tables here exactly |
| Data flow | No fleet page imports the managed server package; every read is a command |
| Operator errors and doctor codes | Every refusal shown in a fleet page carries the fix, and the relay refusal is shown word for word |
| Rule: owner rulings on the web interface | Named data is a table, state is a word, no raw command text leaves the browser outside the Console tab |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Six child specs and two back-end specs exist | `ls plan/post-release/features/spec-fleet-ui-*.md plan/post-release/features/spec-fleet-8-*.md plan/post-release/features/spec-fleet-9-*.md` |
| The fleet mode is offered only on a hub | `./le test functional web` over `fleet-shell-renders` and `fleet-absent-on-device` |
| Fleet styles hold no literal colour | `TestFleetStylesUseVariablesOnly` |
| The design pages name four modes on a hub | `grep -n "Fleet" plan/design/web-operator-workbench-definition-of-good.md` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Authorization | Every fleet route and fragment is authorized on the server; a hidden control is still refused |
| Event stream scope | A stream carries only devices the operator may see |
| Input validation | Device names, label keys and values, filter text and sort keys are validated before they reach a command |
| Command text | No page control sends command text; only the Console tab does, through the CLI path and its authorization |
| Output | Relayed output is bounded, stripped of control sequences and escaped before rendering |
| Destructive actions | Each is confirmed, names its targets and is audited with the hub user |
| Resource use | Tables and trees are bounded on the server; a stream that is not read is closed |

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

- The interface has two speeds. Pages about many devices read what the hub already holds and are fast whatever the devices do. Pages about one device ask that device and can fail. Keeping the two apart is what stops one unreachable device from breaking an estate view.
- The familiar layout is structure, not decoration. What makes it feel known is that the same three things repeat at every level: an object header, the same tab names, the same table stack.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| An interactive prototype on sample data comes first, built from the real pages | Static mock-ups or a separate throwaway prototype; building each page against its real data first | The owner judges the look and the feel in a browser before the costly back-end work, and nothing is thrown away: the sample answers sit behind the same dispatcher the real commands will, so making a page functional replaces its data and keeps its markup |
| A fourth web mode on a hub | A section inside the workbench; replacing the workbench shell | The workbench shell has no tree and no object tabs, and replacing it would reopen a design in progress. A fourth mode leaves the three existing ones untouched |
| Ze's density with the familiar structure | The more spacious density of comparable tools; a per-user density setting | The owner ruled the web interface compact and calm. One density keeps the device and fleet pages one product and halves the pages to test |
| Device tabs render relayed command answers | Proxying each device's own web pages through the hub | Commands answer data with one authorization and audit path and no trust in markup produced by another software version. A proxied page would carry the device's own session, routes and assets through the hub |
| The tree is computed from labels | A stored hierarchy of sites and groups | Labels are already decided as flat pairs. A tree view is an ordered list of label keys, so the same estate can be read by site, by role or by version without changing any device |
| Estate and group pages read hub state only | Asking every device when a page opens | A page over hundreds of devices must not wait for the slowest one |
| A per-connection event stream | The existing broadcast broker | The broker reaches every client, which would show one operator another's devices |
| One compile tag, `ze_fleet_ui`, for the whole interface | Reusing `ze_web`; one tag per page family; the name `ze_control_ui` | A device build has no use for the fleet pages and should not carry them, so they cannot ride on `ze_web`. One tag keeps the gate simple to reason about. The name follows the spec set and the package, as `ze_web` and `ze_lg` follow theirs |
| Shared components live in the web package, fleet pages in their own | Everything in the web package | The shell and components are domain-free and reusable; the fleet pages are a feature that registers and can be compiled out |

## Known Limitations
- The interface manages one hub. Several hubs seen as one estate is not specified, because `fleet-0` rules out replication between hubs.
- Alarms are conditions that are true now. Acknowledgement, silencing and notification by mail or chat are not specified.
- The more spacious density of comparable tools is not offered.

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
- [ ] AC-1..AC-13 all demonstrated across the children
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
- [ ] **Commit B:** `remove` this spec's path only, once every child has closed
