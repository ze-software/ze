# Spec: fleet-ui-2 -- Estate Tree, Device Tables, Filters and Bulk Actions

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | web, fleet |
| Priority | P3 |
| Release reason | Adds a capability absent from the supported release; existing supported behavior and its obligations remain required. |
| RFC gaps | - |
| Depends | spec-fleet-1-device-registry.md |
| Phase | 2/6 |
| Handoff | - |
| Updated | 2026-11-19 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

**Re-read after compaction:** this file, `spec-fleet-ui-0-umbrella.md`,
`spec-fleet-1-device-registry.md`.

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
| Run diagnostic | A menu of registered read-only tools; running one creates a task with one result per device (`fleet-ui-5`, `fleet-9`) |
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
| The real enabling condition | The fleet routes mount only when a data source is registered, and the only source is the sample one, which no shipped build links (`sourceRegistered` in `internal/component/web/fleetui/register.go`) | Register the real source, built over the command dispatcher, and mount the routes when the daemon serves managed devices |
| Sign out | The account menu holds the theme switch and the link to the other web modes. The web server has no sign-out route in any mode | Add the sign-out entry the umbrella's header row names, over a sign-out route of the web package's session handling |
| Side panel and a later sort | Opening a row asks for the side panel fragment alone, so the table's sort and page links keep the address they were drawn with, and following one closes the panel (`PanelLink` paragraph of `docs/architecture/fleet-interface.md`). Approved as built | Decide with the owner whether a sort keeps the open row, when the table becomes functional |

### Data this spec reads

| Data | Command it expects from the `fleet` set | State |
|------|------------------------------------------|-------|
| Device list with name, state, labels, group, version, configuration state, last seen | `show fleet devices`, with filter, sort and page arguments, answering JSON | Missing: `fleet-1` is a skeleton. The filter grammar above is input to its design |
| Latest CPU, memory, storage, uptime per device | Part of the registry record, filled by `fleet-4` and `fleet-8` | Missing |
| Label keys and values in use | `show fleet labels` | Missing |
| Open alarm and running task counts per device | `show fleet alarm`, `show fleet task` (`fleet-ui-5`) | Missing |

### Draft acceptance criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A hub with labelled devices | The tree groups them by the levels of the view in use, with correct counts | Devices lacking a label appear under `(no <key>)`, because hiding them would hide devices |
| AC-2 | The operator changes the tree view | The tree regroups, the URL carries the view, and the selected device stays selected | The expansion state of the previous view is dropped, because its nodes no longer exist |
| AC-3 | A filter in the query bar | The table shows exactly the matching devices, the count names them, and the URL reproduces the view | Selection is cleared when the filter changes, because selected rows may no longer be visible |
| AC-4 | A malformed filter | The table keeps its rows and a message names the bad term and the valid fields | None |
| AC-5 | A selection and a bulk label edit | The edits are staged, shown as pending, and applied on commit, after which the tree regroups | A device removed from the estate meanwhile is reported and skipped, because the rest of the edit is still wanted |
| AC-6 | An estate of 2000 devices | The tree, the table and a filter each answer within the time bound fixed at design, and no fragment carries more rows than it shows | None |
| AC-7 | A row's eye control | The side panel opens with the sections above, the URL carries the row, and a reload restores it | None |
