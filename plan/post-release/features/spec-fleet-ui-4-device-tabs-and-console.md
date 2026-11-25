# Spec: fleet-ui-4 -- Device Tabs and the Per-Device Console

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | web, fleet |
| Priority | P3 |
| Release reason | Adds a capability absent from the supported release; existing supported behavior and its obligations remain required. |
| RFC gaps | - |
| Depends | spec-fleet-ui-2-estate-tree-and-device-tables.md, spec-fleet-9-device-command-relay.md, plan/release/features/spec-fleet-4-inventory-health.md, plan/release/features/spec-fleet-8-metric-history.md |
| Phase | 4/9 of plan/post-release/features/spec-fleet-ui-0-umbrella.md |
| Handoff | - |
| Updated | 2026-11-25 |

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
| Tasks | Tasks that targeted this device | `fleet-ui-9`, hub-held |
| Alarms | The authorized device alarm list | Fleet UI-9 |

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
| Health | One row per component: name, state word, detail | Fleet-4’s retained health report and receipt time |
| Labels and group | Tags, editable by an operator allowed to edit; the configuration group | Registry |
| Management | Management address, link to the device's own interface when an address is configured, connected since, relay profile granted | Registry |

### Interfaces tab

One table: name, type, administrative state, link state, addresses, speed,
MTU, traffic in and out as rates, errors and drops. The row's side panel shows
the full counters and a traffic chart from the hub's history. Refreshed every 5
seconds. Row actions are the device's registered related tools for an interface
(counters detail, ping from the interface), run through the relay and shown in
an overlay that leaves the table in place.

### Interface history after the first release

Collect independently of browser sessions. Extend Fleet-8's negotiated sample report with an optional bounded interface block only when both peers advertise support. Record interface identity, transmitted and received octets, device observation time and counter-generation identity. A first observation establishes a baseline. Consecutive observations from the same generation yield rates in bits per second; a reset, rename, disconnect, missing interval or counter decrease starts a new baseline and leaves a gap. Unsupported peers continue to send their existing whole-device totals.

Proposal for sign-off: admit at most 64 explicitly selected interfaces per device, at most one report per normal sample interval, and at most seven days. Reject a configuration that selects more interfaces than the limit and identify the device. Use the history store's charged byte budget, an independent interface allocation ceiling per device and across the hub, and the existing history checkpoint. Reserve the first-release device series budget first. Evict the oldest interface buckets under pressure, report the shortened coverage, and reject a new series if even its minimum state cannot fit. Bound names and encoded frames before allocation. Device removal drops its interface series; a new interface generation never inherits its predecessor's counters.

The hub's registered per-interface series command authorizes the device before looking up the interface, restricts range and output count, and returns source timestamps, gaps and units. The tab passes the caller and interface selector through the same operation. Test retained history after restart with two interfaces whose rates differ and the tab closed during collection.

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
| Relay dispatcher bound to a device, with streaming, deadline and cancellation | Planned by Fleet-9; enforce its request identity and admission bounds |
| The device's `show` commands with JSON answers | Available on the device; their reach through the relay depends on `fleet-9` |
| The web command line and its terminal component | Available |
| Hub-held inventory | Planned by Fleet-4; use its timestamped report |
| Registered related tools | Available for the workbench; their use through the relay is specified here |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A connected device granting a relay profile | Each relayed tab shows the device's live state, matching what the same commands show at the device's own command line | Values may differ by one refresh interval |
| AC-2 | A device reachable only through its managed connection | Every tab works | None: this is the reason the relay exists |
| AC-3 | A device that grants no relay profile | Every relayed section shows the device's refusal with the granting line; hub-held sections still show | None |
| AC-4 | One command of a tab fails or is unknown to the device | Its section shows the matching state; the other sections render | None |
| AC-5 | A command typed in the Console tab | It runs on that device, its output appears in the terminal, and it is audited on both sides with the hub user's name | Configuration mode is refused with a link to the Config tab, because the console routes operational commands and the Config tab owns hub-side edits |
| AC-6 | The operator leaves a tab | Its refresh timers and relayed streams stop | None |
| AC-7 | An offline device | Relayed sections show the offline state with the last-seen time; the Console shows no terminal | Hub-held inventory remains shown, marked with its age |
| AC-9 | Two selected interfaces have different rates while the panel is closed. Reopen after a hub restart. | The panel matches the authorized per-interface query for each interface. It returns retained points with distinct rates, a gap after a counter reset, and no samples from any other interface. | An unsupported peer or evicted interval shows its actual coverage. |
| AC-10 | Add a 65th selected interface or fill the charged byte budget | The invalid selection is refused. Under byte pressure the hub stays within both interface budgets; a shortened series reports its coverage and the whole-device history survives. | None. |

### Moved in (owner ruling 2026-11-23)

Thomas ruled (voice-transcribed answer "1 b") that the fleet features of the
first release are reached through commands on every command surface, and that
their web pages and views come after the first release (`ai/META-DECISIONS.md`,
"Goal and scope"). The device detail view that
`plan/release/features/spec-fleet-4-inventory-health.md` named (its AC-12,
"device detail page with full inventory/health") is this spec's System tab;
`fleet-4` keeps `show fleet device inventory name <name>` and
`show fleet device health name <name>`.

| AC ID | From | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|------|-------------------|-------------------|-----------------------------|
| AC-8 | `fleet-4` AC-12, its device detail half | A device whose inventory and health `fleet-4` holds, on a real build with the inventory-health part registered (`plan/post-release/features/spec-fleet-ui-2-estate-tree-and-device-tables.md` AC-17) | The System tab's Identity and Hardware sections show the reported inventory (processor count and model, network interfaces and drivers, total memory, storage count and size) and the Health section one row per reported component with its state word, each equal to what `show fleet device inventory name <name>` and `show fleet device health name <name>` answer; a device that has not reported shows not reported | An offline device's hub-held sections stay shown, marked with the time they were reported |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/api-surfaces.md` - a tab reads commands through a dispatcher, never a back end's package
  → Constraint: the System tab's hub-held sections read `fleet-4`'s commands through the inventory-health part of `fleet-ui-2`.
- [ ] `docs/architecture/web-workbench-pages.md` - the workbench page builders a relayed tab may reuse

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/web/fleetui/source.go` - `Source` is the union of six interfaces, `DeviceViewSource` among them; `fleet-ui-2` splits it into parts, `DeviceViewPart` and `InventoryHealthPart` among them.

## Existing Mechanisms

| Mechanism | Owner | Current behavior | Design |
|---|---|---|---|
| Device inventory | `handler_device_system.go`, `hardwareSection`; Fleet UI-2 InventoryHealthPart | The current hardwareSection calls the relayed DeviceSystem operation. | Hub-held reports remain available with timestamps when the live relay is unavailable. |
| Console | `handler_device_console.go`, `newConsoleHandler`; `source_views.go`, `DeviceViewSource` | A POST waits for one completed command answer. | Preserve finite answers and add a cancellable streaming operation for monitors through the authorized device relay. |
| History queries | Fleet-8’s planned sample report and fixed device rings; `fleetui.HistorySource.DeviceSeries` | The existing source queries whole-device traffic by device and metric. The production sampler and store are Fleet-8 prerequisites. | Add an optional per-interface sample block and separate interface key after release. Retain the device-series query. |
| Live and retained data | handler_device_system.go, hardwareSection; handler_device_interfaces.go, handler_device_routing.go, handler_device_logs.go | The System hardware section currently reads DeviceSystem via the source. Interfaces, peer detail and logs consume finite device answers. | Bind hardware and health to Fleet-4 retained reports and separately request live-only sections. Extend the interface and peer detail contracts and cancel log streams on exit. |
| Chart and terminal | handler_chart.go, assets/fleet-chart.js, assets/fleet-terminal.js | Charts read device or estate series and the terminal appends one response fragment per POST. | Add the authorized per-interface query and a cancellable monitor stream while preserving finite output. |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|---|---|---|---|---|---|
| `docs/architecture/fleet-interface-history.md` | Query and retention | Post-release history contract | "The hub retains interface observations independently of page visits." | agrees | UI-4 owns bounded collection and the authorized query. AC-9 closes the panel during collection and crosses a restart. |
| `docs/architecture/fleet-interface-history.md` | Design decisions | Post-release history contract | "For the post-release device Interfaces panel, Thomas chose “Add bounded per-interface history in the post-release design”." | agrees | AC-9 and AC-10 exercise separate interface rates, gaps, and hard limits. |
| `docs/architecture/fleet-interface.md` | Device tabs | Console and interface panel | "An interface row has no traffic chart" | departs | The post-release panel adds an interface-keyed chart and negotiated peer detail. The console also streams monitor output and can cancel it. |
| `ai/doors/system.md` | One schema, one command registry, every interface | the door every change reads | "A command is declared once and reaches every interface." | agrees | The web and command presentations read facts through the registered operation under the caller’s identity. |
| `docs/architecture/fleet-interface.md` | A source answers facts, the package derives | the // Design: page of internal/component/web/fleetui/source_views.go | "A source answers facts only" | agrees | Use caller-scoped source facts and the shared derivations to keep rows, totals and linked detail views consistent. |
| `ai/doors/api-surfaces.md` | Boundaries | the door covering internal/component/web/fleetui/source_views.go | "Never add a command in an adapter." | agrees | The fleet view consumes registered operations. Each operation is implemented by the owner of its data. |
| `ai/doors/le-tooling.md` | Where it sits | the door covering test/web/fleet-device-live.wb | "Functional `.ci` files live under `test/<suite>/`" | agrees | Use the repository’s functional harness for managed behavior and its headless web harness for the views. |
| `docs/architecture/fleet-interface.md` | Design decisions | the // Design: page of internal/component/web/fleetui/source_views.go | "The proposed alarm defaults apply together. A missing relay grant stays visible while the device is offline. A measured alarm uses its existing threshold duration after reconnect. Causes match within one device. Thomas selected “Use all three defaults”." | agrees | A relay grant is independent of connectivity. Use the hub’s cause result for one device and its configured resource duration. |
| `docs/architecture/fleet-interface.md` | Design decisions | the // Design: page of internal/component/web/fleetui/source_views.go | "Thomas chose the following rule for Resource high at reconnect. “End the old occurrence. Show a new alarm only if fresh samples qualify.”" | agrees | A reconnect ends the old resource occurrence. Its historical opening and clearing survive, and any new qualifying interval has its own identity. |
| `docs/architecture/fleet-interface.md` | Design decisions | the // Design: page of internal/component/web/fleetui/source_views.go | "When a critical alarm becomes visible again after its cause clears, show its toast to an authorized viewer of the affected device. Thomas selected “Show the critical toast”." | agrees | The post-release notification provider checks visibility before delivering the critical recovery toast. |
| `docs/architecture/fleet-interface.md` | Design decisions | the // Design: page of internal/component/web/fleetui/source_views.go | "For Force complete on a staged rollout, Thomas chose “Leave unstarted devices on their prior version”." | agrees | The rollout result distinguishes devices admitted to the change from devices that keep their prior version. |
| `docs/architecture/fleet-interface.md` | Design decisions | the // Design: page of internal/component/web/fleetui/source_views.go | "Thomas chose “Add the device Alarms tab”. The post-release device page offers the same authorized alarm rows as the device filter on the estate or group Alarms tab." | agrees | The shared device navigation includes its Alarms tab and uses the same authorized device filter as the estate view. |
| `docs/architecture/fleet-interface.md` | The data source seam | Live fleet interface | "one console command" | departs | Add optional streaming and per-interface series operations alongside the existing finite and whole-device source methods. |
| `docs/architecture/fleet-interface.md` | Components | Live fleet interface | "appends the `TerminalEntry` fragment" | departs | Append bounded chunks to a running terminal entry with a completion or refusal state. |
| `docs/architecture/fleet-interface.md` | Fragments | Live fleet interface | "The JSON of one chart card" | departs | Add a registered interface-series fragment scoped to one authorized device and interface, with a time range and bounded response. |
| `docs/architecture/fleet-interface.md` | Routes | Live fleet interface | "One Console command of a device, under the mutation wrap" | departs | The same wrapped POST can open a bounded response stream and cancel the named request when the browser aborts. |
| `ai/doors/hub-engine.md` | Boundaries | the door covering pkg/fleet/sample.go | "No always-on file imports a gated package." | agrees | The real hub binds the live provider; the sample provider stays optional and disabled builds import no gated surface. |
| `docs/architecture/fleet-config.md` | RPCs | the // Design: page of pkg/fleet/envelope.go | "All RPCs are scoped to that name implicitly." | agrees | Bind capabilities and observations to their authenticated managed session. The payload cannot choose another device. |
| `docs/architecture/api/commands.md` | Transport Request Metadata | anchored to pkg/fleet/envelope.go | "Caller identity is injected only by trusted transport wiring." | agrees | The trusted request supplies the caller. Authorize its target before reading a retained series or opening a live command. |
| `ai/doors/plugin-infra.md` | Boundaries | the door covering internal/component/plugin/server/managed.go | "The SDK stays generic." | agrees | The optional report is compact JSON on the existing managed connection. A transport plugin carries no UI-specific policy. |
| `ai/doors/forwarding.md` | Where it sits | the door covering internal/component/iface/fleet_samples.go | "Components with a pluggable backend." | agrees | Read per-interface counters from the device’s actual forwarding backend. Unavailable counters leave gaps. |
| `ai/rules/repo-maintenance.md` | Directives | the // Design: page of internal/component/managed/register.go | "the owning `docs/architecture/` page for a contract" | agrees | Document capability negotiation, history semantics, the command and the affected fleet pages alongside the change. |
| `docs/architecture/doctor-and-health-checks.md` | Recent events: the report bus in show doctor | anchored to internal/component/managed/register.go | "Its producer raises it on the report bus" | agrees | Read current history and producer status through the owner. Missing data or a disconnected producer cannot produce a healthy or zero measurement. |
| `docs/architecture/config/syntax.md` | Overview | anchored to internal/component/plugin/yang/ze-plugin-conf.yang | "The parser is YANG-driven: each config node's type (leaf, leaf-list, container, list) determines how it is parsed." | agrees | The selected interface list belongs to the managed YANG configuration and is checked at load and reload. |
| `docs/architecture/pki/tls-listeners.md` | Decision: rotation does not rebind the hub listeners | anchored to internal/component/config/loader_extract.go | "The listeners keep their sockets, and the next handshake serves the new certificate." | agrees | Collection uses the existing authenticated managed session. Named certificate resolution and listener transport stay with their owner. |
| `ai/doors/config-yang.md` | Where it sits | the door covering internal/component/config/loader_extract.go | "The schema is spread across components." | agrees | Parse and validate the selected names through the owning YANG schema before producing a grant. |
| `docs/architecture/api/process-protocol.md` | Wire Format | the // Design: page of internal/component/plugin/types.go | "Every message is a single newline-terminated line:" | agrees | Bound interface records before serialization and reject an oversized block under the transport’s existing frame ceiling. |
| `docs/architecture/api/architecture.md` | Command parameter values | anchored to internal/component/plugin/types.go | "The command model then accepts or refuses that value." | agrees | The request’s authenticated identity follows the registered query and relay. Device names and route parameters do not grant permissions. |
| `docs/architecture/api/ipc_protocol.md` | Overview | anchored to internal/component/plugin/types.go | "Ze uses correlated, bidirectional RPC between the engine and plugins." | agrees | Use the existing UTF-8 line framing for managed negotiation and report envelopes. |
| `docs/architecture/api/json-format.md` | Field naming | anchored to internal/component/plugin/types.go | "JSON keys are lowercase kebab-case, never camelCase and never snake_case." | agrees | Give optional sample fields lowercase kebab-case keys and preserve the current sample shape when the extension is absent. |
| `docs/architecture/cli/command-verbs.md` | Read verbs and their guarantee | anchored to internal/component/plugin/types.go | "That promise is what lets a read-only operator run it" | agrees | The query names its device and interface. Output operators apply to its authorized series. |
| `docs/architecture/cli/error-surface.md` | The rule | anchored to internal/component/plugin/types.go | "A fault goes to the error zone." | agrees | Missing grants, refused commands and invalid intervals render their structured operational reason and the matching fix. |
| `docs/architecture/config/transaction-protocol.md` | 1. Design Principles | anchored to internal/component/plugin/types.go | "Plugins decide what they need." | agrees | Only an accepted configuration publishes a new interface selection. A refused selection leaves the prior limits and collectors in place. |
| `docs/architecture/core-design.md` | 19. Component Boundaries | anchored to internal/component/plugin/types.go | "Each component under `internal/component/` is independently removable." | agrees | The normal program provides device queries through its command registration. History reads do not program a data plane. |
| `docs/architecture/encoding-context.md` | Overview | anchored to internal/component/plugin/types.go | — | silent | BGP capability-dependent wire encoding does not cover the managed report or history store. |
| `docs/architecture/plugin-manager-wiring.md` | Solution: Two-Phase Startup | anchored to internal/component/plugin/types.go | "The server is constructed in the hub before Engine.Start." | agrees | Plugin Stage 1 startup budgets do not cover the optional managed report or interface retention. |
| `docs/architecture/web-interface.md` | Admin command visibility | anchored to internal/component/plugin/types.go | "Admin command discovery omits schema-hidden nodes and their descendants." | agrees | Keep route and command discovery under the same authorization as execution. The device tabs render their data through the existing web components. |
| `docs/architecture/fleet-config.md` | Design decisions | the // Design: page of pkg/fleet/envelope.go | "A reload that changes the certificate a serving hub's managed listener presents installs it on the running listener at that reload: the next handshake presents the certificate the configuration names, the listener keeps its socket and no session is closed, as the web and looking glass listeners do. The certificate stays in the configuration's `pki` block. In Thomas's words (voice-transcribed): "Keep the certificate in the configuration." and "In that case on commit we can perform the certificate change on the listener."" | agrees | The managed listener keeps its authenticated sessions when its certificate changes. Collection and relay use those sessions; subsequent handshakes use the configured certificate. |
| `docs/architecture/fleet-config.md` | Design decisions | the // Design: page of pkg/fleet/envelope.go | "Thomas chose “Allow first provisioning and later edits from the device’s web Config tab”. An authorized operator can commit the first or a later version of a declared device’s configuration while it is offline, through the post-release fleet web surface." | agrees | UI-6 owns offline first provisioning and later desired-config edits; the UI-4 console requires a live grant, while retained inventory and history work offline. |
| `docs/architecture/api/commands.md` | Design decisions | anchored to pkg/fleet/envelope.go | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | The chart query selects one permitted device and interface. Filtering or rendering acts on the resulting answer, and never changes the target or executes an additional command. |
| `docs/architecture/api/commands.md` | Design decisions | anchored to pkg/fleet/envelope.go | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | The chart query selects one permitted device and interface. Filtering or rendering acts on the resulting answer, and never changes the target or executes an additional command. |
| `docs/architecture/api/commands.md` | Design decisions | anchored to pkg/fleet/envelope.go | "A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does" | agrees | The chart query selects one permitted device and interface. Filtering or rendering acts on the resulting answer, and never changes the target or executes an additional command. |
| `docs/architecture/pki/tls-listeners.md` | Design decisions | anchored to internal/component/config/loader_extract.go | "The looking glass may serve plain HTTP, because what it publishes is public, and one looking glass serves HTTP and HTTPS at the same time. Thomas's words, as voice-transcribed: "We should allow the looking glass to be HTTP. The information is public." and "We should be able to serve on both HTTP and HTTPS, not either." Asked where the choice lives and what happens to `ze.looking-glass.tls`, he answered "1a 2a": the choice is the `tls` leaf of each `server` entry, default true, and the environment variable is removed. The no-store rule applies to each entry: an entry that left `tls` unset serves HTTP with a warning on a daemon with no `database/` store and no named certificate, an entry that says `tls true` there is refused at start and at reload, and a reload does what a restart with the same config does" | agrees | Use the authenticated managed session. The device tabs and history scope do not change certificate selection on reload or the allowed protocols of a looking glass. |
| `docs/architecture/pki/tls-listeners.md` | Design decisions | anchored to internal/component/config/loader_extract.go | "The web and looking glass use the shared `listenerTLSMaterial` precedence. The approved Fleet reload design shares `managedCertificate` with startup because the daemon authority issues its unnamed leaf." | agrees | Use the authenticated managed session. The device tabs and history scope do not change certificate selection on reload or the allowed protocols of a looking glass. |
| `docs/architecture/api/architecture.md` | Design decisions | anchored to internal/component/plugin/types.go | "When `\| implied` modifies an existing config line (such as inserting automatic members into a `filters [ ... ];` list), it keeps the line intact and places an inline `# implied: ...` comment on the same line after the statement naming what was added and why, rather than placing a comment only above the line or splitting the list across multiple lines. In Thomas's words (2026-11-23): "No, it should be on the same line and say what was added"." | agrees | The tabs use the normal command display; a returned implied filter entry retains the normal inline explanation. |
| `docs/architecture/core-design.md` | Design decisions | anchored to internal/component/plugin/types.go | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | The device reads observations from the configured forwarding plane. A read cannot install a filter, change route attributes or bypass backend compatibility checks. |
| `docs/architecture/core-design.md` | Design decisions | anchored to internal/component/plugin/types.go | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | The device reads observations from the configured forwarding plane. A read cannot install a filter, change route attributes or bypass backend compatibility checks. |
| `docs/architecture/core-design.md` | Design decisions | anchored to internal/component/plugin/types.go | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | The device reads observations from the configured forwarding plane. A read cannot install a filter, change route attributes or bypass backend compatibility checks. |
| `docs/architecture/core-design.md` | Design decisions | anchored to internal/component/plugin/types.go | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | The device reads observations from the configured forwarding plane. A read cannot install a filter, change route attributes or bypass backend compatibility checks. |
| `docs/architecture/core-design.md` | Design decisions | anchored to internal/component/plugin/types.go | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | The device reads observations from the configured forwarding plane. A read cannot install a filter, change route attributes or bypass backend compatibility checks. |
| `docs/architecture/core-design.md` | Design decisions | anchored to internal/component/plugin/types.go | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | The device reads observations from the configured forwarding plane. A read cannot install a filter, change route attributes or bypass backend compatibility checks. |
| `docs/architecture/core-design.md` | Design decisions | anchored to internal/component/plugin/types.go | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | The device reads observations from the configured forwarding plane. A read cannot install a filter, change route attributes or bypass backend compatibility checks. |
| `docs/architecture/core-design.md` | Design decisions | anchored to internal/component/plugin/types.go | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | The device reads observations from the configured forwarding plane. A read cannot install a filter, change route attributes or bypass backend compatibility checks. |
| `docs/architecture/core-design.md` | Design decisions | anchored to internal/component/plugin/types.go | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | The device reads observations from the configured forwarding plane. A read cannot install a filter, change route attributes or bypass backend compatibility checks. |
| `docs/architecture/core-design.md` | Design decisions | anchored to internal/component/plugin/types.go | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | The device reads observations from the configured forwarding plane. A read cannot install a filter, change route attributes or bypass backend compatibility checks. |
| `docs/architecture/core-design.md` | Design decisions | anchored to internal/component/plugin/types.go | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | The device reads observations from the configured forwarding plane. A read cannot install a filter, change route attributes or bypass backend compatibility checks. |

## Data Flow (MANDATORY)

### Changed-Wire Census

**Search evidence:** Fleet-8 supplies the base device-wide report. Its proposed `pkg/fleet/sample.go`, managed sender, receiver and history command are prerequisite files extended by this child. Search the existing managed send/receive routes in `internal/component/managed/client.go`, `internal/component/plugin/server/managed.go`, `managed_serve.go` and the bootstrap reader in `cmd/ze/ze_core_start.go`. Future Fleet-8 producers are named in its census.

| Wire | Role | Carrier | Source evidence | Disposition |
|---|---|---|---|---|
| Config-fetch interface-history support | writer | Device `fetchConfigWithin` | `internal/component/managed/client.go` | On a live managed connection advertise the optional block version. Bootstrap does not collect interface history. |
| Config-fetch interface-history support | reader | Hub `ManagedServer.handleRequest` | `internal/component/plugin/server/managed_serve.go` | Accept the negotiation only for the authenticated device. |
| Config-fetch selection | writer | Hub `ManagedConfigService.HandleConfigFetch` | `internal/component/plugin/server/managed.go` | On a mutually supported session advertise the bounded selected names and version on either successful fetch shape. |
| Config-fetch selection | reader | Steady-state device fetch and bootstrap | `internal/component/managed/client.go`; `cmd/ze/ze_core_start.go` | Steady-state sender uses the advertised accepted selection. Bootstrap and old devices ignore the extra fields. |
| Sample report optional interface block | writer | Fleet-8 device sampler | `internal/component/managed/sample.go`, Fleet-8 planned sender | Send bounded selected observations only after mutual agreement. |
| Sample report optional interface block | reader | Fleet-8 hub receiver | `internal/component/plugin/server/managed_history.go`, Fleet-8 planned receiver | Verify current session, supported type, granted names and lengths, then charge the per-device and hub budgets. |

### Entry Point
- A device tab request on the hub's fleet interface.

### Transformation Path
1. A hub-held section asks the inventory-health part; a relayed section runs its fixed commands through the relay dispatcher of `fleet-9`.
2. Each section renders its own answer or its own state.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Web to hub daemon | `web.RouteDeps.Dispatch` | No |
| Hub to device | The command relay of `fleet-9` | No |

### Integration Points
- The System tab reads timestamped inventory and health from the hub. Each live section uses the relay for that device and caller. A missing grant or an offline device produces the appropriate section refusal; hub-held sections remain available.
- The posted Console command currently returns a single bounded answer. Add a streaming source operation beside it for a live request. The hub checks access before dispatch, passes the request context to the relay, writes each bounded output fragment as received, and cancels the relay when the browser aborts or navigates away. End the request on refusal, disconnect, timeout, output cap or completion. The finite sample source still returns a terminal entry.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| The System tab of a reporting device | → | the inventory-health part, the System tab sections | `test/web/fleet-device-live.wb` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSystemTabShowsReportedInventory` | `internal/component/web/fleetui/device_test.go` | AC-8 | |

## Files to Modify

- `pkg/fleet/sample.go` extends Fleet-8's public sample report with the optional interface block.
- `pkg/fleet/envelope.go` declares the optional capability on the fetch negotiation.
- `internal/component/plugin/server/managed.go` advertises acceptance of the extension on this session.
- `internal/component/managed/client.go` negotiates the supported extension on its normal fetch.
- `internal/component/managed/sample.go` collects the selected observations only on supported sessions.
- `internal/component/iface/fleet_samples.go` supplies per-interface byte counters, sample times and counter generation at the interface owner.
- `internal/component/plugin/server/managed_history.go` admits bounded series and checkpoints retained buckets and gaps.
- `internal/component/managed/cmd_history.go` authorizes and answers the query for one named device and interface.
- `internal/component/managed/yang/ze-managed-cmd.yang` declares that query and its bounds.
- `internal/component/managed/register.go` registers the query with the shared dispatcher.
- `internal/component/plugin/yang/ze-plugin-conf.yang` declares selected interfaces per device.
- `internal/component/config/loader_extract.go` validates the selection before configuration acceptance.
- `internal/component/plugin/types.go` carries the selection in the managed configuration.
- `internal/component/web/fleetui/source_views.go` exposes the live view and cancellable console operation.
- `internal/component/web/fleetui/source_history.go` exposes the optional authorized interface history query.
- `internal/component/web/fleetui/handler_device_console.go` keeps the active request and renders each finite or streamed response.
- `internal/component/web/fleetui/assets/fleet-terminal.js` aborts the owned stream on exit.
- `internal/component/web/fleetui/handler_device_system.go` renders retained inventory and health separately from live answers.
- `internal/component/web/fleetui/handler_device_interfaces.go` selects the exact interface for its chart.
- `internal/component/web/fleetui/handler_device_routing.go` renders authorized capabilities and counters in BGP detail.
- `internal/component/web/fleetui/handler_device_logs.go` scopes recent and live events to the current authorized device.
- `internal/component/web/fleetui/handler_chart.go` scopes the chart fragment to the authorized device, interface and time range.
- `internal/component/web/fleetui/assets/fleet-chart.js` selects the interface series and displays gaps.
- `docs/architecture/fleet-config.md` documents the version negotiation, limits and session identity for the optional report.
- `docs/architecture/fleet-interface.md` describes live, retained and streamed results.
- `docs/architecture/fleet-interface-history.md` documents panel semantics and the bounded per-interface history that this spec delivers.

## Files to Create

- `cmd/ze/hub/register_fleet_ui_deviceview.go` links the live device provider in eligible builds.
- `internal/component/web/fleetui/deviceview/register.go` registers the optional live source.
- `internal/component/web/fleetui/deviceview/deviceview.go` adapts the caller's request to the Fleet-9 relay and Fleet-8 history.
- `test/web/fleet-device-live.wb` checks tab data, refusal and cancellation.
- `test/managed/fleet-interface-history.ci` proves old/new peer negotiation, separate interfaces, restart, bounds and coverage while the panel is closed.

## Design presentation

### What changes

Use hub-held reports for inventory and a caller-scoped relay for current device data. The interface panel reads retained per-interface history collected even when the tab is closed. Return finite console results and streamed output through the existing terminal view.

### Before and after

An offline device still shows the time of its last hardware report. A connected device with no relay grant explains its refusal. With a grant, a monitor prints its first output before completion and leaving the tab cancels it on the device.

### Alternatives set aside

A direct browser connection cannot reach devices behind NAT. Buffering an entire command into one completed response prevents cancellation and live output for a monitor.

### Documents departed from

`docs/architecture/fleet-interface.md`, Device tabs, currently limits a device interface row to counters, the BGP peer panel to session and prefix facts, and each Console POST to one completed terminal entry. The post-release design adds a bounded interface traffic chart, authorized capabilities and counters from the peer detail command, and cancellable streamed entries for monitors. The data source seam gains the two optional methods. The Components section gains a streaming terminal entry. Fragments gains an interface-history chart query. Routes retains the protected console POST and adds a streamed response mode. The command POST retains its existing mutation protection. Finite commands still return a completed entry. A missing optional peer detail is reported as unavailable.

### Choices for Thomas

Thomas chose “Add bounded per-interface history in the post-release design” on 2026-11-25. The proposed 64-interface ceiling limits collection and storage; the implementation must measure the full charged size at this bound.

## Implementation Steps

1. **Phase: Wiring** - open a real device tab under a scoped caller, stream the first console output before completion, then navigate away and observe the request end on the device.
2. Connect the System, Interfaces, Routing and Logs tabs to their authorized sources. Preserve Fleet-4’s report after disconnect and show the source age. Exercise individual refusals and partial data.
3. Negotiate optional interface observations between new peers, persist the bounded series and register the scoped history query. Exercise cap exhaustion, counter resets, restart, removal and a legacy client. Bind the range-selected panel to the authorized query.
4. Stream Console and following Logs through the session-bound Fleet-9 request. Limit browser output to the existing ConsoleOutputBytesMax per command; on truncation visibly end the output and cancel the relay. Prove cancellation after navigation without ending another operator’s session.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
