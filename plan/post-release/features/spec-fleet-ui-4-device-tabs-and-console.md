# Spec: fleet-ui-4 -- Device Tabs and the Per-Device Console

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | web, fleet |
| Priority | P3 |
| Release reason | Adds a capability absent from the supported release; existing supported behavior and its obligations remain required. |
| RFC gaps | - |
| Depends | spec-fleet-ui-2-estate-tree-and-device-tables.md, spec-fleet-9-device-command-relay.md |
| Phase | 4/8 of plan/post-release/features/spec-fleet-ui-0-umbrella.md |
| Handoff | - |
| Updated | 2026-11-18 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

**Re-read after compaction:** this file, `spec-fleet-ui-0-umbrella.md`,
`spec-fleet-9-device-command-relay.md`,
`docs/architecture/web-workbench-pages.md`.

## Task

Let an operator inspect and operate one device from the hub, without knowing
its address and without the device being reachable from the operator's network.
This spec makes the device tabs functional. Every one of them shows live state
that only the device holds, fetched through the command relay of `fleet-9`.

The tabs of a device, in order:

| Tab | Purpose | Source |
|-----|---------|--------|
| Dashboard | The device at a glance | `fleet-ui-3`, hub-held state |
| Console | A command line on the device, in the browser | This spec, relay |
| System | What the device is: hardware, software, resources, health | This spec, hub-held inventory plus relay |
| Interfaces | Interfaces with their live state and counters | This spec, relay |
| Routing | BGP peers and routing protocol state | This spec, relay |
| Config | The device's configuration, its history and its diff | `fleet-ui-6`, hub-held |
| Logs | Recent log lines, warnings and errors of the device | This spec, relay |
| Tasks | Tasks that targeted this device | `fleet-ui-5`, hub-held |

### How a relayed tab works

1. The tab's handler holds a fixed list of read-only commands, written in the handler, never taken from the browser.
2. It runs each through the relay dispatcher bound to the device, asking for the JSON answer.
3. It renders the answers with the shared table, key and value and usage bar components.
4. Each section of a tab is its own fragment with its own state, so one slow or refused command does not blank the tab.
5. A section refreshes on a timer while the tab is open, at the interval the table below gives, and stops when the tab is left.

The relay dispatcher has the same type as the dispatcher the workbench pages
use. Where an existing workbench page builder reads only through its
dispatcher, the device tab reuses that builder with the relay dispatcher in its
place. Where a builder reads the local daemon directly (the interface backend,
the applied firewall state, the process memory), it cannot be reused, and the
tab is built here over commands.

### System tab

| Section | Rows | Source |
|---------|------|--------|
| Identity | Name, model, serial number, software version, build, kernel, uptime, system time and its offset from the hub's | Inventory (`fleet-4`); time through the relay |
| Hardware | Processor, memory, network interfaces, storage with health summary, temperature | Inventory |
| Resources | CPU, memory, storage and load as usage bars with figures | Relay, refreshed every 10 seconds |
| Health | One row per component: name, state word, detail | Relay, the device's health report |
| Labels and group | Tags, editable by an operator allowed to edit; the configuration group | Registry |
| Management | Management address, link to the device's own interface when an address is configured, connected since, relay profile granted | Registry |

### Interfaces tab

One table: name, type, administrative state, link state, addresses, speed,
MTU, traffic in and out as rates, errors and drops. The row's side panel shows
the full counters and a traffic chart from the hub's history. Refreshed every 5
seconds. Row actions are the device's registered related tools for an interface
(counters detail, ping from the interface), run through the relay and shown in
an overlay that leaves the table in place.

### Routing tab

A row of sub-tabs, one per protocol the device runs, with BGP first.

| Sub-tab | Table |
|---------|-------|
| BGP peers | Peer, group, remote AS, state word, up or down since, prefixes received and sent, last error. Side panel: negotiated capabilities, families, counters, state history. Row actions: the registered related tools for a peer |
| BGP summary | Per family: peers established, routes, best routes |
| Other protocols | One sub-tab per protocol the device answers for, each a table of adjacencies or neighbours with their state |

A sub-tab is shown only when the device answers its command. A protocol the
device does not run has no sub-tab, rather than an empty one.

### Logs tab

Three sub-tabs: Recent, Warnings, Errors. Each is a table of time, level,
source and message, newest first, bounded to the last 500 lines, with the
umbrella's query bar filtering by text, level and source. Recent follows the
device's log while the tab is open, through a relayed stream, with a pause
control.

### Console tab

The existing web command line, with the relay dispatcher in place of the local
one. The header above the terminal names the device, the hub user, and the
profile the device grants to relayed commands.

| Rule | Detail |
|------|--------|
| Operational commands | Typed commands run on the device through the relay, with completion and help answered by the device, so they match its version |
| Authorization | The hub checks the hub user's rights to use the console on this device; the device then applies the profile its own configuration grants to relayed commands (`fleet-9`) |
| Refusal | A device that grants no relay profile refuses, and the terminal shows the device's refusal word for word, including the configuration line that grants it |
| Configuration mode | Not offered. The device's configuration is written by the hub (`fleet-6`), so the console says so and links to the Config tab |
| Streaming commands | `monitor` commands stream through the relay until the operator stops them or leaves the tab |
| Audit | Every command is audited on the hub with the hub user and on the device with the hub user's name as carried by the relay |
| Session | Closing the tab or losing the device ends the session; nothing keeps running on the device |
| Offline device | The offline state, with the last-seen time. No terminal is shown |

### States particular to device tabs

| Condition | What the section shows |
|-----------|------------------------|
| Device offline | The umbrella's offline state. Sections fed by hub-held inventory still show, marked with the time they were reported |
| Relay not granted on the device | The device's refusal and the line that grants it |
| The device does not know the command (older or differently built software) | "This device's software does not answer this view", with its version |
| The command exceeds its deadline | "The device did not answer within N seconds", with a retry |
| The operator's profile on the hub does not allow it | The unauthorized state |

### Key design decisions

| Decision | Detail |
|----------|--------|
| Tabs render command answers, not the device's own pages | One authorization and audit path, no markup from another software version rendered inside the hub's page, and no second session |
| Commands are fixed in the handler | The browser sends a tab and a device name. Only the Console tab carries typed text, through the command line path |
| Sub-tabs follow what the device answers | The hub does not assume the device's build |
| Device tabs are registered | A feature adds a device tab or a routing sub-tab by registration, so a protocol's page is owned by that protocol |
| A link to the device's own interface stays | Offered in the object header when an address is configured, for what the tabs do not cover |

### Dependencies and readiness

| Dependency | State |
|------------|-------|
| Relay dispatcher bound to a device, with streaming, deadline and cancellation | Missing: `fleet-9` is a skeleton |
| The device's `show` commands with JSON answers | Available on the device; their reach through the relay depends on `fleet-9` |
| The web command line and its terminal component | Available |
| Hub-held inventory | Missing: `fleet-4` is a skeleton |
| Registered related tools | Available for the workbench; their use through the relay is specified here |

### Draft acceptance criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A connected device granting a relay profile | Each relayed tab shows the device's live state, matching what the same commands show at the device's own command line | Values may differ by one refresh interval |
| AC-2 | A device reachable only through its managed connection | Every tab works | None: this is the reason the relay exists |
| AC-3 | A device that grants no relay profile | Every relayed section shows the device's refusal with the granting line; hub-held sections still show | None |
| AC-4 | One command of a tab fails or is unknown to the device | Its section shows the matching state; the other sections render | None |
| AC-5 | A command typed in the Console tab | It runs on that device, its output appears in the terminal, and it is audited on both sides with the hub user's name | Configuration mode is refused with a link to the Config tab, because the hub is the single writer |
| AC-6 | The operator leaves a tab | Its refresh timers and relayed streams stop | None |
| AC-7 | An offline device | Relayed sections show the offline state with the last-seen time; the Console shows no terminal | Hub-held inventory remains shown, marked with its age |
