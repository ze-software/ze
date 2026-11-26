# Spec: fleet-4 -- Inventory and Health Reporting

| Field | Value |
|-------|-------|
| Status | in-progress |
| Priority | P2 |
| Release reason | Thomas put this fleet capability (device health and inventory reports to the hub) in the first release; existing supported behavior and its obligations remain required. |
| Depends | spec-fleet-1-device-registry (closed 2026-11-25; documented in `docs/architecture/fleet-config.md`, "Device registry") |
| Phase | 4/10 of plan/post-release/features/spec-fleet-0-umbrella.md |
| Updated | 2026-11-26 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `internal/component/managed/client.go` -- RunManagedClient (add reporting here)
4. `internal/core/health/` -- health registry (source data)
5. `internal/component/host/inventory.go` -- hardware inventory (source data)
6. `plan/post-release/features/spec-fleet-0-umbrella.md` -- umbrella design decisions
7. `docs/architecture/fleet-config.md`, "Device registry" (`spec-fleet-1-device-registry`, closed 2026-11-25) -- device registry (dependency)

## Task

Add two new RPC verbs (`inventory-report` and `health-report`) so managed devices push
their hardware inventory and component health to the hub. The hub stores this data in
the device registry and exposes it through commands on every command surface (CLI, SSH,
MCP, REST, gRPC, the web command line).

### Owner ruling of 2026-11-23: commands in the first release, pages after it

Thomas ruled (voice-transcribed answer "1 b") that the fleet features of the first
release are reached through commands on every command surface, and that their web
pages and views come after the first release (`ai/META-DECISIONS.md`, "Goal and
scope"). This spec keeps the verbs, the hub's storage and the three commands; every web
view it named moved to the post-release fleet-ui spec that owns the page, and the
inventory-health part of the fleet interface (which `spec-fleet-1-device-registry.md`
had given this spec) moved to `fleet-ui-2`, which builds it over this spec's commands.

| Moved out | Destination spec | Its AC there |
|-----------|------------------|--------------|
| AC-4, its web half ("fleet dashboard shows degraded") | `plan/post-release/features/spec-fleet-ui-3-dashboards-and-charts.md` | AC-8 |
| AC-12, the inventory and health columns of the device table and the side panel | `plan/post-release/features/spec-fleet-ui-2-estate-tree-and-device-tables.md` | AC-16 |
| AC-12, the device detail page with full inventory and health | `plan/post-release/features/spec-fleet-ui-4-device-tabs-and-console.md` | AC-8 |
| The `InventoryHealthPart` registration (from `fleet-1`'s part table) | `plan/post-release/features/spec-fleet-ui-2-estate-tree-and-device-tables.md` | AC-17 |

AC-4 keeps its command half, and AC-13, added with this ruling, proves the three
commands on every command surface, so what the moved views showed stays reachable and
tested in the first release.

Currently each device has rich local data (CPU, NIC, DMI, memory, thermal, storage, SMART
via `host/inventory.go`; component health via `core/health/`) but none of it is reported
to the hub. An operator cannot answer "which devices have ECC errors?", "which devices
have degraded BGP?", or "what hardware is in edge-42?" without SSHing to each device.

The managed client gains two periodic reports. The protocol gains two new verbs on the existing MuxConn transport.

### Key Design Decisions (from umbrella)

| Decision | Detail |
|----------|--------|
| New RPC verbs | `inventory-report` and `health-report` on existing `#id verb [json]\n` framing |
| Client pushes, hub stores | Device initiates reports; hub is passive receiver. No hub-to-device query |
| Report timing | Inventory: on connect + daily. Health: on connect + every 5 minutes |
| Data scope | Inventory summary (no raw SMART data). Health: same structure as `/health` HTTP endpoint |
| Storage | Per-device fields in DeviceRegistry (from fleet-1) |

## Required Reading

| Source | Constraint |
|--------|------------|
| `docs/architecture/fleet-config.md`, RPCs and Device connections and configuration history | Authentication determines the report owner. A device can be declared before it has ever connected. |
| `internal/core/health/registry.go`, `Registry.Check` and `Registry.Handler` | The HTTP handler obtains a report from the same registry used by the managed client. A check whose applicability predicate is false contributes no component. |
| `internal/component/host/inventory.go`, `Detect` and `Detector.Detect` | Use the daemon's configured cached detector when present. A partial scan can return a populated inventory with per-section errors and a nil top-level error. |
| `internal/component/managed/client.go`, `runConnection` and `fetchAndProcess` | Start reporting after the initial fetch succeeds. A client holding local edits can still report hardware and health. |
| `internal/component/plugin/server/managed_serve.go`, `handleRequest` | The hub returns an explicit error for an unknown method. Its serving loop can read subsequent requests. |
| `pkg/plugin/rpc/conn.go` and `pkg/plugin/rpc/framing.go` | The frame bound and default 30-second write deadline protect transport. A report needs its own size limit and reply deadline. |
| `docs/architecture/fleet-config.md`, "Device registry" (`spec-fleet-1-device-registry`, closed 2026-11-25) | Extend its per-name record and coalescing writer. The managed server owns the registry, its store lifetime and its snapshot. |
| `plan/post-release/features/spec-fleet-ui-2-estate-tree-and-device-tables.md`, AC-16 and AC-17 | The future inventory-health source reads the device's model, serial, software version and component rows from these command answers. |

## Current Behavior

A hub knows a declared client's identity, connection state, configuration fetch and acknowledgement. The managed configuration channel has five existing verbs. Reporting has no producer on that channel today. The device can already collect its hardware and check its components locally. Fleet-1 is in progress; the registry symbols below exist in the shared checkout. Its record writer and read-back are present in the shared checkout and supply the persistence path for these tests.

## Existing Mechanisms

| Mechanism | Producer | Current behavior | Use in Fleet-4 |
|-----------|----------|------------------|----------------|
| Hardware inventory | `host.Detect` and `Detector.Detect` in `internal/component/host/inventory.go` | Collect summary and detailed Linux hardware facts, including section-specific problems. The daemon can supply a shared cached detector. | Project the Ze software release, CPU logical count and model, NIC count and driver names, memory bytes and ECC counters when `MemoryInfo.ECCPresent` is true, storage device count and total bytes, DMI system model and its serial. Carry availability per section. Read the Ze software release from `internal/core/version.Release`. |
| Component health | `health.Check`, `Registry.Check` and `Registry.Handler` | Return the overall status and check time plus the applicable components in name order, each with name, status, optional reason and check time. Down takes precedence over degraded. | Preserve these JSON fields and status meanings. Define the public envelope in `pkg/fleet` without importing internal health types there. Compare its JSON with the actual health producer in a test. |
| Managed session | `RunManagedClient` in `internal/component/managed/client.go`, `ManagedServer.handleRequest` | Own authentication, the initial fetch, heartbeat, notifications and reconnect. The server dispatches by method on the authenticated connection. | Add a per-session report worker with bounded calls, cancellation and a join on exit. Introduce a registered report method lookup at dispatch so an additional report registers beside its producer and handler. Preserve the existing response to all unregistered methods. |
| Device records | `ManagedDeviceRecord`, `admitDevice`, `disconnectDevice`, `ManagedRegistrySnapshot` in `managed_registry.go` | Keep one record per registry identity, one current connection and a snapshot owned by the reader. The fleet-1 writer persists dirty records in the hub store. | Replace each accepted report and its hub receipt time atomically on the active authenticated session, mark the record dirty, and deep-copy the added slices on snapshot. Read reports absent from older records as unreported. |
| Daemon fleet commands | `RegisterRPCs` and shape registrations in `internal/component/managed/register.go`, `handleFleetDevice` | The YANG-declared device query reads the server snapshot and supplies a structured answer to each command surface. | Register inventory detail, health detail and fleet health here. Use the same registry refusals and device-name completion as the registry commands. |
| Durable writer and deletion reconciliation | `internal/component/plugin/server/managed_record_store.go`: `syncDeviceRecord`, `ObserveDeviceRecordKey`, `managedRecordObservation.afterDelete` and `decodeDeviceRecord` | Under the mutex the writer takes a record value and compares it with the name-only empty record. It marshals after unlocking, compares the live record with the saved value on completion and retains facts recorded after deletion. | Keep the record comparable with one optional pointer to an immutable snapshot for each report kind. The snapshot owns its collection storage and includes receipt time. Validate and copy every input before publishing its pointer. Each accepted report, including a report with identical content, installs a fresh snapshot; absence is nil. The writer and key observer can retain these immutable references safely across unlock. A separate copy of every report returned by the query path keeps mutation by a caller away from live state. Extend `afterDelete` to retain a report only when its pointer differs from the one observed at deletion. On restart validate stored report fields using the receiver’s checks, accepting absent fields in older records. |
| Fleet store and diagnostics | Fleet-1's registered device-record key, dirty writer and persistence/doctor state | Record write failures surface through the registry, and operators can inspect them. | Reports share the device record, store ordering, shutdown flush and failure reporting. Surface report collection or transmission refusals through the existing report bus with the device, report kind, cause and corrective action. |

Report registrations have separate producer and receiver roles. The shared fleet package declares the method name, wire contract and producer registration. The host package registers the inventory projection from beside its detector; the managed package registers health using the core health producer. The managed client schedules registered producers, and the server looks up registered receivers. A registration missing a required method, a duplicate registration or a duplicate method name is a startup defect. A build without a hardware producer leaves inventory unreported and continues reporting health. Server code imports neither the host component nor the managed client component.

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | One schema, one command registry, every interface | System door | "A command is declared once and reaches every interface." | agrees | All three commands are daemon-owned and YANG-declared; terminal and remote surfaces dispatch the same requests. |
| `ai/doors/hub-engine.md` | Boundaries | Managed client and public envelopes | "No always-on file imports a gated package." | agrees | The client discovers registered report producers. The host detector registers its own projection. |
| `ai/doors/plugin-infra.md` | Traps | Managed server | "so a payload must be compact `json.Marshal` output." | agrees | The two methods use compact JSON on the existing framed connection. |
| `ai/doors/le-tooling.md` | Where it sits | Managed functional fixtures | "Functional `.ci` files live under `test/<suite>/` and run through `./le test <suite>`" | agrees | The live tests run from the managed suite. |
| `ai/doors/commands-cli.md` | Boundaries | Command ownership and shape | "A handler answers data, never text." | agrees | Device commands return structured documents and the estate summary returns counts and device details. |
| `ai/doors/core.md` | Boundaries | Fleet-1 record persistence | "A registry holds value types and is the only list." | agrees | Each snapshot owns its values, including the mutable slices held in its report. |
| `docs/architecture/fleet-config.md` | RPCs | Published contract updated by this spec | "After `#0 auth`, the hub knows the client's name. All RPCs are scoped to that name implicitly." | agrees | Reports carry no device identity. Each receiver checks the current authenticated session before replacing data. |
| `docs/architecture/fleet-config.md` | Design decisions | Owner decision of 2026-11-25 | "Thomas chose “Allow first provisioning and later edits from the device’s web Config tab”. An authorized operator can commit the first or a later version of a declared device’s configuration while it is offline, through the post-release fleet web surface." | agrees | Device health and inventory read the registry after provisioning and do not require the client to be online to prepare a configuration. |
| `docs/architecture/fleet-config.md` | Device connections and configuration history | Fleet-1 registry | "Each entry admits one session." | agrees | One accepted report belongs to that session. Its receipt time and device observations stay on the record after disconnect. |
| `docs/architecture/api/commands.md` | Which surfaces offer a command | Published command contract | "Offered: the daemon serves each through its handler" | agrees | Daemon registration and the YANG paths publish the three read-only fleet commands. The generic pipe operators act on their answers. |
| `docs/architecture/api/architecture.md` | Command parameter values | Managed server anchors | "The command model then accepts or refuses that value." | agrees | The device selector uses the existing command grammar and follows registry lookup and refusal on each surface. |
| `docs/architecture/core-design.md` | 19. Component Boundaries | Managed server anchors | "Each component under `internal/component/` is independently removable." | agrees | The hardware collector is registered by host. The server receives fleet envelopes without importing host or the managed client. |
| `docs/architecture/hub-architecture.md` | SSH session model factory | Managed server anchors | "Both owners finish before the hub returns its store to the launcher." | agrees | The report worker is cancelled and joined with the managed client, and the registry flushes within server shutdown. |
| `ai/rules/repo-maintenance.md` | Directives | Managed command registration | "the owning `docs/architecture/` page for a contract" | agrees | Update fleet-config and the command page with the receiver, wire and answer contracts and their source anchors. |
| `docs/architecture/doctor-and-health-checks.md` | Recent events: the report bus in show doctor | Managed client and command registration | "Its producer raises it on the report bus" | agrees | Report failures travel through the reporting component to the existing recent-events check. A failed write of the device record uses Fleet-1’s registry diagnostics. |
| `docs/architecture/fleet-config.md` | Design decisions | Decision of 2026-11-23 | "A reload that changes the certificate a serving hub's managed listener presents installs it on the running listener at that reload: the next handshake presents the certificate the configuration names, the listener keeps its socket and no session is closed, as the web and looking glass listeners do. The certificate stays in the configuration's `pki` block. In Thomas's words (voice-transcribed): "Keep the certificate in the configuration." and "In that case on commit we can perform the certificate change on the listener."" | agrees | Reports share the authenticated connection. Rotation affects subsequent handshakes; the hub keeps its session and scheduled reports. |
| `docs/architecture/api/commands.md` | Design decisions | Decision of 2026-08-21 | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | Device selectors identify the subject. The generic pipes filter or format the resulting health and inventory facts. |
| `docs/architecture/api/commands.md` | Design decisions | Decision of 2026-11-20 | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | All three query commands use generic operators on their declared response shapes. |
| `docs/architecture/api/commands.md` | Design decisions | Decision of 2026-11-23 | "A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does" | agrees | Piping an answer cannot trigger a new device report or mutate registry state. |
| `docs/architecture/api/architecture.md` | Design decisions | Decision of 2026-11-23 | "When `\| implied` modifies an existing config line (such as inserting automatic members into a `filters [ ... ];` list), it keeps the line intact and places an inline `# implied: ...` comment on the same line after the statement naming what was added and why, rather than placing a comment only above the line or splitting the list across multiple lines. In Thomas's words (2026-11-23): "No, it should be on the same line and say what was added"." | agrees | Report commands read the device registry. They neither render nor annotate implied configuration. |
| `docs/architecture/core-design.md` | Design decisions | Decision of 2026-11-23 | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | Collecting observations never commits configuration or attempts to program the kernel or VPP. |
| `docs/architecture/core-design.md` | Design decisions | Decision of 2026-11-23 | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | The managed reporter attaches to a connected session without changing start, validate or reload outcomes. |
| `docs/architecture/core-design.md` | Design decisions | Decision of 2026-11-23 | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | A reported health status gives the managed reporter no authority over firewall validation or lowering. |
| `docs/architecture/core-design.md` | Design decisions | Decision of 2026-11-23 | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | Inventory describes detected hardware. It adds no back-end selection setting. |
| `docs/architecture/core-design.md` | Design decisions | Decision of 2026-11-23 | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | Health status is read from the BGP component and does not change BGP session options or outbound policy. |
| `docs/architecture/core-design.md` | Design decisions | Decision of 2026-11-23 | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | Serializing BGP health does not add an export-chain intent or a policy authoring form. |
| `docs/architecture/core-design.md` | Design decisions | Decision of 2026-11-23 | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | The registered operations in this design are fleet RPCs; they add no filter to the BGP engine. |
| `docs/architecture/core-design.md` | Design decisions | Decision of 2026-11-23 | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | Reports read component observations and never transform an UPDATE or set next-hop. |
| `docs/architecture/core-design.md` | Design decisions | Decision of 2026-11-23 | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | Neither the health status nor its reason changes import policy or address-family validation. |
| `docs/architecture/core-design.md` | Design decisions | Decision of 2026-11-23 | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | Device health measures registered checks without touching local origination or route forwarding. |
| `docs/architecture/core-design.md` | Design decisions | Decision of 2026-11-23 | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | The report registry dispatches managed RPCs. Its entries do not alter the implied-filter inventory. |
| `docs/architecture/pki/tls-listeners.md` | Decision: rotation does not rebind the hub listeners | Managed serving loop anchor | "The listeners keep their sockets, and the next handshake serves the new certificate." | agrees | Reporting runs within an authenticated session and does not replace the listening socket during certificate rotation. |
| `docs/architecture/pki/tls-listeners.md` | Design decisions | Decision of 2026-11-23 | "The looking glass may serve plain HTTP, because what it publishes is public, and one looking glass serves HTTP and HTTPS at the same time. Thomas's words, as voice-transcribed: "We should allow the looking glass to be HTTP. The information is public." and "We should be able to serve on both HTTP and HTTPS, not either." Asked where the choice lives and what happens to `ze.looking-glass.tls`, he answered "1a 2a": the choice is the `tls` leaf of each `server` entry, default true, and the environment variable is removed. The no-store rule applies to each entry: an entry that left `tls` unset serves HTTP with a warning on a daemon with no `database/` store and no named certificate, an entry that says `tls true` there is refused at start and at reload, and a reload does what a restart with the same config does" | agrees | Each report uses the managed TLS session. The looking glass continues serving each configured HTTP or HTTPS endpoint. |
| `docs/architecture/pki/tls-listeners.md` | Design decisions | Decision of 2026-11-23 | "The web and looking glass use the shared `listenerTLSMaterial` precedence. The approved Fleet reload design shares `managedCertificate` with startup because the daemon authority issues its unnamed leaf." | agrees | Reporting starts after admission. Managed certificate selection at startup and reload stays with managedCertificate. |

## Data Flow

1. After the authenticated session completes its first configuration fetch, the report worker samples inventory and health immediately. It sends them independently. A refused inventory report cannot suppress a health report. Inventory repeats every 24 hours and health every five minutes. A reconnect starts a new schedule. Standalone devices start no reporter.
2. The compact JSON payload carries the report, never a routing identity. Inventory excludes raw SMART samples and drive serials. Each unavailable hardware section is identified so a missing reading has no numerical interpretation. Both report bodies are bounded at 64 KiB before sending and before decoding. Check typed fields, collection lengths within the body limit, valid health statuses, unique component names, and timestamps. A refused body leaves the previous complete snapshot readable.
3. The server looks up registered report handlers after dispatching its existing configuration methods. A known report can update only the device whose authenticated connection is still current. Accept the first inventory and health reports of a new connection. Thereafter permit one inventory each 24 hours and health no faster than once per minute per connection. A rejected attempt cannot postpone the next permitted report. The sender's regular health cadence remains five minutes.
4. Each successful handler records the hub's receipt timestamp separately from the device's own check time, schedules a write through Fleet-1, and acknowledges the RPC. A current report replaces the previous report of that kind. Disconnect and hub restart preserve the last accepted reports with their times. Unknown devices and store errors keep the registry's distinct refusals.
5. The device paths are `show fleet device inventory name <name>` and `show fleet device health name <name>`. Each declares the `name` command node and its mandatory length-bounded string leaf following the existing `show fleet device name` declaration. Register each daemon handler and device-name completer under the owning managed module. Verify both text and structured invocations resolve the same selector, and omission is refused. Device commands return reported, receipt time, connection state and the last data, or an explicit unreported result. The fleet summary classifies each declared device once. It shows last reported healthy, degraded and down counts, counts devices with no report separately, and lists degraded/down devices with name, receipt time and connection state. Disconnection by itself does not turn a component down. A new report after reconnect replaces the previous observation.
6. A peer built before the report methods answers `unknown method: inventory-report` or `unknown method: health-report`. Suppress only that report kind for the rest of this connection and retry on the next connection. A timeout waiting for a report reply ends that call; a transport close or stalled socket write uses the existing reconnect path. Neither a refusal nor a missing reply makes the reporter block configuration pushes, acknowledgements or heartbeat.

### Changed-wire census

| Carrier | Writers and readers | Preservation and proof |
|---------|--------------------|------------------------|
| Managed RPC requests | The report worker writes through `rpc.MuxConn.CallRPC`; the hub serves them from `ManagedServer.handleRequest`. Managed peer fixtures under `internal/test/fixture` and `test/managed` also speak the protocol. | Two new methods. Keep framing, authentication, all five existing methods, their replies and their handling of unknown names. Exercise a new client against an old-method fixture and an old client against the new hub. |
| Persisted device record | The report receiver updates Fleet-1's device record. The registry writer serializes it and the registry reader restores it. | Both snapshots and receipt times are optional. A record saved before Fleet-4 remains readable. Prove reconnect, process restart, and a failed update preserving the previous value. |
| Command answers | Daemon-owned builtin handlers write structured replies. Each command surface consumes the same registration. Fleet UI-2 will consume the same fields. | Preserve the shapes of existing registry commands. Test the two detail commands and aggregate on a live hub as well as command-surface discovery. |

### Risks and assumptions

| Condition | Consequence | Proof or control |
|-----------|-------------|------------------|
| A report RPC never gets an answer | Config notifications or shutdown could hang | Bound the complete call to 30 seconds, cancel the worker with the session and join it before the next session. Test a missing report reply while ping and config-changed still work. |
| An earlier connection completes work after a replacement connects | Stale data could replace a newer report | Under the registry lock compare the receiving connection with the live entry. Test the old session is refused and the current value survives. |
| Hardware detection gives partial data, or the hub restarts between report kinds | Zero values could be misread as observed facts | Check availability separately for each section and report, and read older records with absent fields. |
| Health callbacks may be slow | Report generation could delay liveness or configuration | The report worker runs separately from the connection's read loop. Its lifetime is one authenticated session; it starts no goroutine per component or tick. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Client connects and completes config-fetch | -> | Client sends inventory-report | `test/managed/fleet-inventory-report.ci` |
| Client health timer fires | -> | Client sends health-report | `test/managed/fleet-health-report.ci` |
| `show fleet device inventory name edge-01` | -> | Hub returns stored inventory | `test/managed/fleet-inventory-report.ci` |
| `show fleet health` | -> | Hub aggregates health across devices | `test/managed/fleet-health-report.ci` |
| The three commands on SSH, MCP, REST, gRPC and the web command line | -> | The same builtins through the daemon dispatcher | `TestEveryCommandOnEverySurface` (`cmd/ze/hub/surface_parity_test.go`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Client connects and completes config-fetch | Client sends `inventory-report` RPC with hardware summary, then sends it again after 24 hours on the same session |
| AC-2 | Hub receives inventory-report | Inventory stored in DeviceRegistry for that device |
| AC-3 | Client completes its first fetch or its five-minute health timer fires | Client sends `health-report` RPC with component statuses |
| AC-4 | Hub receives health-report with degraded component | Device health updated in registry; `show fleet device health name <name>` shows the component degraded and `show fleet health` counts the device degraded (the dashboard half moved to `fleet-ui-3` AC-8) |
| AC-5 | `show fleet device inventory name edge-01` | Shows CPU, NIC, memory, storage summary for edge-01 |
| AC-6 | `show fleet device health name edge-01` | Shows per-component health status (healthy/degraded/down) |
| AC-7 | `show fleet health` | Aggregated view: N healthy, N degraded, N down. Lists degraded/down devices |
| AC-8 | Device has not reported yet (first connect in progress) | Inventory and health show "pending" or "not reported" |
| AC-9 | Hub receives inventory-report from a declared client (`fleet-1` admits only a client its hub declares, on the address of the block that declares it) | Report accepted and stored under the name the session authenticated as. A name no client entry declares is refused at authentication (`AuthenticateWithLookup` refuses an empty expected token), so no report from it is ever stored; `fleet-1` has no auto-discovery (its table "Superseded from the May skeleton") |
| AC-10 | Old hub (without fleet-4) receives inventory-report | Hub sends the matching RPC error `unknown method: inventory-report` and accepts a later ping and configuration fetch on the same session |
| AC-11 | Client connected to old hub (without fleet-4) | A report refusal or bounded reply timeout leaves the configuration and heartbeat paths usable. An unknown method disables only that report kind until reconnect |
| AC-13 | `show fleet device inventory name edge-01`, `show fleet device health name edge-01` and `show fleet health` asked on the CLI, SSH, MCP, REST, gRPC and the web command line | Each surface lists the three commands and the daemon dispatcher serves them there (`TestEveryCommandOnEverySurface`); every surface's answer is the builtin handler's, whose content `fleet-inventory-report.ci` and `fleet-health-report.ci` assert. Added by the owner ruling of 2026-11-23 |
| AC-14 | Disconnect and restart after accepting reports | Both detail commands show the last snapshot, receipt time and disconnected state. A device that never reported remains explicitly unreported. |
| AC-15 | Invalid, oversized, too-frequent or superseded-session report | The hub returns an error and preserves the accepted snapshot, other devices and the next allowed interval. |
| AC-16 | Partial hardware inventory, inapplicable health check and unreported device | The inventory flags the unavailable section, health omits the inapplicable component, and the estate counts the unreported device separately. |
| AC-17 | Inventory with model, DMI serial, software version and ECC counters | These fields reach the command answer where the host supplies them; raw SMART and drive serials are absent. |
| AC-18 | Concurrent report replacement, store completion, and operator deletion | A delayed write cannot mark a newer report durable. A report received after an observed deletion is retained, a report still identical to its old occurrence is forgotten, and a query caller cannot mutate either the stored or live result. |

AC-12 (the web dashboard's inventory and health columns and the device detail page)
moved: the table and side panel to `fleet-ui-2` AC-16, the detail page to `fleet-ui-4`
AC-8 (the Task section's ruling table).

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestInventoryReportPayload` | `pkg/fleet/report_test.go` | Inventory report JSON marshal/unmarshal | PASS |
| `TestHealthReportPayload` | `pkg/fleet/report_test.go` | Health report JSON marshal/unmarshal | PASS |
| `TestInventoryReportHandler` | `internal/component/plugin/server/fleet_report_test.go` | Hub stores inventory from report | PASS |
| `TestHealthReportHandler` | `internal/component/plugin/server/fleet_report_test.go` | Hub stores health from report | PASS |
| `TestClientSendsInventoryReport` | `internal/component/managed/report_test.go` | Client sends inventory after config-fetch | PASS |
| `TestClientSendsHealthReport` | `internal/component/managed/report_test.go` | Client sends health periodically | PASS |
| `TestReportDisconnectRestart` | `internal/component/plugin/server/fleet_report_test.go` | A disconnected snapshot survives hub restart, old records remain unreported and a new report replaces the old one | PASS |
| `TestReportStoreSnapshotOwnership` | `internal/component/plugin/server/fleet_report_test.go` | AC-18: pause the writer after it observes a report, replace that report, mutate a caller snapshot and delete the record with a later report pending; prove the right report persists and earlier data is forgotten | PASS |
| `TestReportRefusalsPreserveLastValue` | `internal/component/plugin/server/fleet_report_test.go` | Invalid, oversized, stale-session and rate-limited requests preserve earlier data and acknowledge nothing | PASS |
| `TestReportUnknownMethodAndTimeout` | `internal/component/managed/report_test.go` | Independent report schedules, unknown-method handling, no reply, reconnect and concurrent ping/config notification | PASS |
| `TestFleetHealthAggregation` | `internal/component/plugin/server/fleet_report_test.go` | Aggregate health across multiple devices | PASS |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `fleet-inventory-report` | `test/managed/fleet-inventory-report.ci` | Device connects, reports inventory, queryable via CLI | PASS |
| `fleet-health-report` | `test/managed/fleet-health-report.ci` | Device reports health, aggregated view via CLI | PASS |

## Files to Modify
- `pkg/fleet/envelope.go` -- add VerbInventoryReport, VerbHealthReport constants
- `internal/component/managed/client.go` -- add report calls to client loop
- `internal/component/plugin/server/managed_serve.go` -- registered report dispatch before the existing unknown-method refusal
- `internal/component/plugin/server/managed_registry.go` -- per-device snapshot, deep copy, timestamps and current-session check
- `internal/component/plugin/server/managed_record_store.go` -- restore both report kinds, reconcile later reports on record deletion, preserve comparable immutable writer snapshots
- `internal/component/managed/register.go` -- daemon RPCs, shapes and completion
- `internal/component/managed/yang/ze-managed-cmd.yang` -- the three inventory and health commands
- `docs/architecture/fleet-config.md` -- managed report methods, timing and the facts stored for a disconnected device
- `docs/architecture/api/commands.md` -- the shared daemon command answers

## Files to Create
- `pkg/fleet/report.go` -- InventoryReport and HealthReport payload types
- `internal/component/host/fleet_inventory.go` -- registers the host-owned summary producer and its daily interval
- `pkg/fleet/report_test.go` -- payload marshal/unmarshal tests
- `internal/component/managed/report.go` -- client-side report sending logic
- `internal/component/managed/report_test.go` -- client-side tests
- `internal/component/managed/cmd_reports.go` -- inventory and health command answers
- `internal/component/plugin/server/fleet_report.go` -- hub-side report handlers
- `internal/component/plugin/server/fleet_report_test.go` -- hub-side tests
- `test/managed/fleet-inventory-report.ci` -- functional test
- `test/managed/fleet-health-report.ci` -- functional test

## Design presentation

### What changes

A managed device sends the hub its inventory and health after connecting. The operator can read the last observations from the hub even after the device disconnects. Device and receipt timestamps distinguish an old reading from a new one. A device that has never sent a report is identified explicitly.

### Before and after

| Operator action | Today | With Fleet-4 |
|-----------------|-------|--------------|
| Run `show fleet device inventory name edge-01` on the hub | The operator must ask edge-01 for its hardware. | The hub answers the last model, DMI serial, Ze version, CPU, NIC, memory/ECC and storage summary, with receipt time and availability. |
| Run `show fleet device health name edge-01` after the device disconnects | The device's local health endpoint is unreachable from the hub. | The answer shows the disconnected device's last check time and component states and reasons. |
| Run `show fleet health` with one healthy device, one degraded device and one that has never reported | No fleet command supplies these counts. | The hub reports one healthy, one degraded, zero down and one unreported, with the degraded device named. |

### Alternatives set aside

Polling each device from the hub would require an additional inbound route to the device and fail on disconnect. Pushing a full hardware dump would expose drive identifiers and make readings of limited value consume transport and record space. The existing managed connection, a bounded projection and Fleet-1's writer address those constraints. Reusing ping for reporting would put inventory collection and failure handling on the liveness path.

### Documents departed from

None. The managed protocol and command documentation gain the report methods, their bounds and their structured answers.

### Choices for Thomas

The first-release command surfaces, data flow and reporting intervals follow the existing fleet scope. The proposed operator-visible rule for sign-off is that a disconnected device keeps its last reported health in the aggregate, with its connection state and receipt time available alongside the result.

## Implementation Steps

### Implementation Phases

1. **Phase: Wiring and payload types** -- InventoryReport, HealthReport in pkg/fleet/
   - First run the named live command tests against the unimplemented reporting path and observe the missing report.
   - Tests: `TestInventoryReportPayload`, `TestHealthReportPayload`
   - Files: `report.go`, `report_test.go`
   - Verify: JSON round-trip tests pass

2. **Phase: Hub-side handlers** -- parse reports, store in DeviceRegistry
   - Tests: `TestInventoryReportHandler`, `TestHealthReportHandler`, `TestFleetHealthAggregation`
   - Files: `fleet_report.go`, `fleet_report_test.go`, `managed_registry.go`, `managed_serve.go`
   - Verify: reports stored and queryable

3. **Phase: Client-side reporting** -- send reports after config-fetch and periodically
   - Tests: `TestClientSendsInventoryReport`, `TestClientSendsHealthReport`
   - Files: `report.go` (managed), `report_test.go` (managed), `client.go`
   - Verify: client sends reports on connect and periodically

4. **Phase: Commands** -- the inventory and health commands, on every command surface (the web views are `fleet-ui-2`, `fleet-ui-3` and `fleet-ui-4`'s, owner ruling of 2026-11-23)
   - Tests: `fleet-inventory-report.ci`, `fleet-health-report.ci`, `TestEveryCommandOnEverySurface` (AC-13)
   - Files: the command registrations and their YANG `ze:command` nodes
   - Verify: `show fleet device inventory/health name <name>` and `show fleet health` work on every command surface, run with `./le test managed`

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All AC-N have implementation |
| Correctness | Report failure does not disrupt config-fetch or heartbeat |
| Naming | RPC verbs are `inventory-report` and `health-report` (kebab-case) |
| Data flow | Client local data -> RPC -> hub handler -> registry -> commands on every command surface |
| Rule: backward-compat | Old hub replies with an unknown-method error; the new client keeps running and retries the method on reconnect |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Report payloads bounded (max 64KB); field counts validated |
| Rate limiting | Accept inventory at connection and daily thereafter. Accept the first health report at connection and subsequent reports at least one minute apart. |
| Information exposure | Inventory summary only (no raw SMART data, no serial numbers beyond what DMI provides) |

## Checklist

### Goal Gates (MUST pass)
- [x] AC-1..AC-11 and AC-13..AC-18 all demonstrated (AC-12 moved to `fleet-ui-2` AC-16 and `fleet-ui-4` AC-8)
- [x] Wiring Test table complete
- [x] The targeted tests for what changed pass, run in the main checkout by the main thread. No agent runs any `./le verify` command: GitHub CI verifies the pushed commit
- [x] Feature code integrated (`internal/*`, `cmd/*`)

### Design
- [x] No premature abstraction
- [x] No speculative features
- [x] Single responsibility per component

### TDD
- [x] Tests written
- [x] Tests FAIL (observed before implementation of reporting phases)
- [x] Tests PASS (verified by main thread across all 4 `Spec-work: fleet-4-inventory-health` commits)

---

## Implementation Summary

### What Was Implemented
- **Phase 1 (`5e50b70671`) — Wire payloads and producer registry (`pkg/fleet/`):** Added `VerbInventoryReport` and `VerbHealthReport` in `pkg/fleet/envelope.go`; added `InventoryReport`, `HealthReport`, typed `HealthStatus`, strict 64 KiB `Encode*`/`Decode*` codecs (`ReportBodyMax`), `Clone()` deep copies, and concurrent `ReportProducer` registry in `pkg/fleet/report.go` with unit tests in `pkg/fleet/report_test.go`.
- **Phase 2 (`cdc5dfeed5`) — Hub receivers, immutable snapshots, and fleet health aggregation (`internal/component/plugin/server/`):** Added `ManagedReportReceiver` registry (24h inventory, 1m health per-connection rate limits, 64 KiB body bound), `handleReport`, and `ManagedFleetHealth`/`FleetHealthSummary` in `fleet_report.go`; wired registration in `register.go` and dispatch in `managed_serve.go` (`handleRequest`); added immutable `ManagedInventorySnapshot` and `ManagedHealthSnapshot` pointers with `Clone()` and active-session checks (`recordInventoryReport`, `recordHealthReport`) in `managed_registry.go`; extended `decodeDeviceRecord` and added `afterObserve` test hook in `managed_record_store.go`; added unit tests in `fleet_report_test.go`.
- **Phase 3 (`9101bad6fe`) — Host inventory producer and client report worker (`internal/component/host/`, `internal/component/managed/`):** Added `ProjectFleetInventory`/`collectFleetInventory` (`FleetInventoryInterval` 24h, per-section availability, ECC presence semantics, raw SMART and drive serial exclusion) in `host/fleet_inventory.go` and registered it in `host/register.go`; added `ProjectFleetHealth`/`collectFleetHealth` (`FleetHealthInterval` 5m, `/health` JSON parity) and single-goroutine `startSessionReporter` worker (30s call timeout, independent schedules, unknown-method per-session suppression, `CodeDoctorManagedReportFailed` report-bus warnings, cancel+join stop) in `managed/report.go`, `managed/register_report.go`, `managed/register_doctor_codes.go`, and `managed/client.go` (`runConnection`), with unit tests in `host/fleet_inventory_test.go`, `managed/report_test.go`, and `managed/client_fetch_deadline_test.go`.
- **Phase 4 (`18e0f40199`) — Commands, live functional tests, and architecture docs:** Declared `show fleet device inventory name`, `show fleet device health name`, and `show fleet health` in `internal/component/managed/yang/ze-managed-cmd.yang`; implemented builtin RPC handlers in `managed/cmd_reports.go` and registered handlers, shapes, column orders, and device-name completion in `managed/register.go` and `managed/cmd_devices.go`; updated `internal/component/plugin/all/testdata/wire-methods.snapshot`; added unit tests in `managed/cmd_reports_test.go` and live functional drivers/tests in `internal/test/fixture/register_managed_fleet_reports.go`, `internal/test/fixture/misc_fixture_managed_fleet_reports.go`, `test/managed/fleet-inventory-report.ci`, and `test/managed/fleet-health-report.ci`; updated `docs/architecture/fleet-config.md` and `docs/architecture/api/commands.md`.

### Bugs Found/Fixed
- `completeFleetDevices` (`internal/component/managed/cmd_devices.go`) rejected interactive Tab completion directly after the command path (`len(completed) == 0`); fixed in `18e0f40199` to accept both direct Tab and explicit `name` keyword, covered by live `TreeCompleter` assertions in `TestShowFleetReportsRefusalsAndCompletion` (`internal/component/managed/cmd_reports_test.go`).
- `TestReportRefusalsPreserveLastValue` (`internal/component/plugin/server/fleet_report_test.go`) initially checked only `ping` after an unknown-method refusal; extended in `18e0f40199` to also assert `config-fetch` succeeds on the same session (AC-10).

### Documentation Updates
- `docs/architecture/fleet-config.md` (commit `18e0f40199`): documented `ManagedInventorySnapshot`/`ManagedHealthSnapshot` on `ManagedDeviceRecord`, the three commands, `inventory-report`/`health-report` RPC rows, and `### Inventory and health reporting` with source anchors (`pkg/fleet/envelope.go`, `pkg/fleet/report.go`, `internal/component/host/fleet_inventory.go`, `internal/component/managed/report.go`, `internal/component/managed/register_doctor_codes.go`, `internal/component/plugin/server/fleet_report.go`, `internal/component/plugin/server/managed_registry.go`).
- `docs/architecture/api/commands.md` (commit `18e0f40199`): updated `## Managed Config RPCs` with `ze-managed:fleet-device-inventory`, `ze-managed:fleet-device-health`, `ze-managed:fleet-health`, and `inventory-report`/`health-report` RPC table rows with source anchors (`internal/component/managed/cmd_reports.go`, `internal/component/managed/register.go`).
- Closure documentation sweep (Commit A): updated `features/fleet-management.md` (added `fleet-inventory-report.ci` and `fleet-health-report.ci`), `docs/features/fleet-management.md`, `docs/features/cli-commands.md`, `docs/guide/fleet-config.md` (capability table, `## Inspect the Fleet`, `doctor-managed-report-failed` in `## Troubleshooting`), `docs/guide/status.md`, `docs/guide/command-reference.md`, and `docs/architecture/doctor-and-health-checks.md` (`doctor-managed-report-failed`), each with verified `<!-- source: -->` anchors.
- `./le doc check all`: 0 fleet-4 source anchor errors, 564/564 command-contract and usage-contract checks OK, 0 fleet-4 help-shape errors (pre-existing unrelated `ze-policy-conf` char-cap and stale pre-`18e0f40199` `le` binary wiki catalog drift; main thread owed `./le doc check all` re-check after Commit A edits).

### Deviations from Plan
- Fleet-1's runtime store observation mechanism (`ObserveDeviceRecordKey` / `managedRecordObservation.afterDelete`) had been unified onto `storage.RuntimeSubscription` (`observeDeviceRecord` and `forgetDeviceRecordLocked` in `internal/component/plugin/server/managed_record_store.go`) before Fleet-4 landed; Fleet-4 preserves the exact AC-18 deletion and concurrent-write reconciliation semantics on top of that mechanism, proven by `TestReportStoreSnapshotOwnership`.
- `internal/component/managed/register_report.go` and `internal/component/managed/register_doctor_codes.go` were added/updated alongside `internal/component/host/register.go` and `internal/component/plugin/server/register.go` so all `init()` registrations live in `register*.go` per Ze Go style.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | Phase 4 updated `docs/architecture/fleet-config.md` and `docs/architecture/api/commands.md` but left operator guides (`docs/guide/fleet-config.md`, `docs/guide/command-reference.md`, `docs/guide/status.md`), feature pages (`features/fleet-management.md`, `docs/features/fleet-management.md`, `docs/features/cli-commands.md`), and `docs/architecture/doctor-and-health-checks.md` without the new commands and `doctor-managed-report-failed` code. | All user-facing docs and feature declarations citing fleet commands or doctor codes must reflect the new surface in the same work. | Closure Step 4 documentation review (`git grep "show fleet" docs/ features/`). | Updated all 7 doc pages with verified source anchors in Commit A. |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| `inventory-report` and `health-report` RPC verbs on existing `#id verb [json]\n` MuxConn framing | Done | `pkg/fleet/envelope.go` (`VerbInventoryReport`, `VerbHealthReport`), `internal/component/plugin/server/managed_serve.go` (`handleRequest`) | Bounded at 64 KiB (`fleet.ReportBodyMax`), compact JSON |
| Client pushes hardware inventory on connect + 24h and component health on connect + 5m | Done | `internal/component/host/register.go` (`init`), `internal/component/host/fleet_inventory.go` (`FleetInventoryInterval`, `collectFleetInventory`), `internal/component/managed/register_report.go` (`init`), `internal/component/managed/report.go` (`FleetHealthInterval`, `startSessionReporter`), `internal/component/managed/client.go` (`runConnection`) | Independent schedules, 30s call bound, cancel+join on session end |
| Hub stores per-device inventory and health in DeviceRegistry across disconnect and restart | Done | `internal/component/plugin/server/fleet_report.go` (`handleReport`, `handleInventoryReportPayload`, `handleHealthReportPayload`), `managed_registry.go` (`ManagedInventorySnapshot`, `ManagedHealthSnapshot`, `recordInventoryReport`, `recordHealthReport`), `managed_record_store.go` (`decodeDeviceRecord`) | Active-session check, per-connection rate limits, deep-copy snapshots |
| Expose `show fleet device inventory name <name>`, `show fleet device health name <name>`, and `show fleet health` on every command surface | Done | `internal/component/managed/yang/ze-managed-cmd.yang`, `internal/component/managed/register.go` (`init`), `internal/component/managed/cmd_reports.go` (`handleFleetDeviceInventory`, `handleFleetDeviceHealth`, `handleFleetHealth`) | Verified across all surfaces by `TestEveryCommandOnEverySurface` |
| Web pages and `InventoryHealthPart` | Changed | Moved to `fleet-ui-2`, `fleet-ui-3`, `fleet-ui-4` | Per owner ruling of 2026-11-23; recorded in Work Not Done |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestClientSendsInventoryReport`, `test/managed/fleet-inventory-report.ci` | Immediate send after config-fetch + 24h repeat (`FleetInventoryInterval`) |
| AC-2 | Done | `TestInventoryReportHandler`, `test/managed/fleet-inventory-report.ci` | Stored in `ManagedDeviceRecord.Inventory` with UTC `ReceivedAt` |
| AC-3 | Done | `TestClientSendsHealthReport`, `TestHealthReportJSONMatchesHealthHandler`, `test/managed/fleet-health-report.ci` | Immediate send + 5m repeat (`FleetHealthInterval`), `/health` JSON parity |
| AC-4 | Done | `TestHealthReportHandler`, `TestFleetHealthAggregation`, `TestShowFleetReportsUnreportedAndReportedAndDisconnected`, `test/managed/fleet-health-report.ci` | Command half done; web dashboard half moved to `fleet-ui-3` AC-8 |
| AC-5 | Done | `TestShowFleetReportsUnreportedAndReportedAndDisconnected`, `test/managed/fleet-inventory-report.ci` | CPU, NIC, memory/ECC, storage, DMI summary |
| AC-6 | Done | `TestShowFleetReportsUnreportedAndReportedAndDisconnected`, `test/managed/fleet-health-report.ci` | Per-component healthy/degraded/down status and reasons |
| AC-7 | Done | `TestFleetHealthAggregation`, `TestShowFleetReportsUnreportedAndReportedAndDisconnected`, `test/managed/fleet-health-report.ci` | Total, healthy, degraded, down, unreported counts + degraded/down device list |
| AC-8 | Done | `TestShowFleetReportsUnreportedAndReportedAndDisconnected`, `test/managed/fleet-inventory-report.ci`, `test/managed/fleet-health-report.ci` | Explicit `reported: false`, `status: not-reported`, empty `components: []` |
| AC-9 | Done | `TestInventoryReportHandler`, `TestHealthReportHandler`, `TestManagedUnknownNameEmptyTokenRefused` | Scoped to authenticated session; undeclared names refused at auth |
| AC-10 | Done | `TestReportRefusalsPreserveLastValue` | `unknown method: <verb>` reply preserves connection for `ping` and `config-fetch` |
| AC-11 | Done | `TestReportUnknownMethodAndTimeout` | Unknown method disables only that kind until reconnect; timeout leaves ping/config-changed working |
| AC-12 | Changed | Moved to `plan/post-release/features/spec-fleet-ui-2-estate-tree-and-device-tables.md` AC-16 and `plan/post-release/features/spec-fleet-ui-4-device-tabs-and-console.md` AC-8 | Owner ruling of 2026-11-23 |
| AC-13 | Done | `TestEveryCommandOnEverySurface`, `TestShowFleetReportsUnreportedAndReportedAndDisconnected`, `TestShowFleetReportsRefusalsAndCompletion` | 0 fleet failures across all 8 surfaces; positional/keyed/prematched selector parity + Tab completion |
| AC-14 | Done | `TestReportDisconnectRestart`, `TestShowFleetReportsUnreportedAndReportedAndDisconnected`, `test/managed/fleet-inventory-report.ci`, `test/managed/fleet-health-report.ci` | Snapshots and `received-at` survive disconnect and hub restart; pre-Fleet-4 records stay unreported |
| AC-15 | Done | `TestInventoryReportPayload`, `TestHealthReportPayload`, `TestReportRefusalsPreserveLastValue` | Invalid, oversized, rate-limited, and superseded-session reports refused without mutating last snapshot or postponing next interval |
| AC-16 | Done | `TestProjectFleetInventory`, `TestHealthReportJSONMatchesHealthHandler`, `TestFleetHealthAggregation`, `TestShowFleetReportsUnreportedAndReportedAndDisconnected`, `test/managed/fleet-health-report.ci` | Per-section `available: false`, inapplicable health check omitted, unreported counted separately |
| AC-17 | Done | `TestProjectFleetInventory`, `TestInventoryReportPayload`, `TestInventoryReportHandler`, `TestShowFleetReportsUnreportedAndReportedAndDisconnected`, `test/managed/fleet-inventory-report.ci` | Model, DMI serial, software version, ECC counters present; raw SMART and drive serials absent |
| AC-18 | Done | `TestReportStoreSnapshotOwnership` | Delayed write cannot mark newer identical/different report durable; deletion forgets old occurrence and retains later reports; caller mutations isolated |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestInventoryReportPayload` | Done | `pkg/fleet/report_test.go` | PASS (`./le test unit pkg path ./pkg/fleet/`) |
| `TestHealthReportPayload` | Done | `pkg/fleet/report_test.go` | PASS (`./le test unit pkg path ./pkg/fleet/`) |
| `TestInventoryReportHandler` | Done | `internal/component/plugin/server/fleet_report_test.go` | PASS |
| `TestHealthReportHandler` | Done | `internal/component/plugin/server/fleet_report_test.go` | PASS |
| `TestClientSendsInventoryReport` | Done | `internal/component/managed/report_test.go` | PASS |
| `TestClientSendsHealthReport` | Done | `internal/component/managed/report_test.go` | PASS |
| `TestReportDisconnectRestart` | Done | `internal/component/plugin/server/fleet_report_test.go` | PASS |
| `TestReportStoreSnapshotOwnership` | Done | `internal/component/plugin/server/fleet_report_test.go` | PASS |
| `TestReportRefusalsPreserveLastValue` | Done | `internal/component/plugin/server/fleet_report_test.go` | PASS |
| `TestReportUnknownMethodAndTimeout` | Done | `internal/component/managed/report_test.go` | PASS |
| `TestFleetHealthAggregation` | Done | `internal/component/plugin/server/fleet_report_test.go` | PASS |
| `fleet-inventory-report` | Done | `test/managed/fleet-inventory-report.ci` | PASS (`./le --name fleet4 test managed --pattern fleet-inventory-report`, 48.5s) |
| `fleet-health-report` | Done | `test/managed/fleet-health-report.ci` | PASS (`./le --name fleet4 test managed --pattern fleet-health-report`, 11.2s) |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `pkg/fleet/envelope.go` | Done | Modified in `5e50b70671` |
| `pkg/fleet/report.go` | Done | Created in `5e50b70671` |
| `pkg/fleet/report_test.go` | Done | Created in `5e50b70671` |
| `internal/component/host/fleet_inventory.go` | Done | Created in `9101bad6fe` (+ `host/register.go`, `host/fleet_inventory_test.go`) |
| `internal/component/managed/client.go` | Done | Modified in `9101bad6fe` (+ `client_fetch_deadline_test.go`) |
| `internal/component/managed/report.go` | Done | Created in `9101bad6fe` (+ `register_report.go`, `register_doctor_codes.go`) |
| `internal/component/managed/report_test.go` | Done | Created in `9101bad6fe` |
| `internal/component/managed/cmd_reports.go` | Done | Created in `18e0f40199` (+ `cmd_reports_test.go`, `cmd_devices.go`) |
| `internal/component/managed/register.go` | Done | Modified in `18e0f40199` |
| `internal/component/managed/yang/ze-managed-cmd.yang` | Done | Modified in `18e0f40199` (+ `wire-methods.snapshot`) |
| `internal/component/plugin/server/managed_serve.go` | Done | Modified in `cdc5dfeed5` (+ `server/register.go`) |
| `internal/component/plugin/server/managed_registry.go` | Done | Modified in `cdc5dfeed5` |
| `internal/component/plugin/server/managed_record_store.go` | Done | Modified in `cdc5dfeed5` |
| `internal/component/plugin/server/fleet_report.go` | Done | Created in `cdc5dfeed5` |
| `internal/component/plugin/server/fleet_report_test.go` | Done | Created in `cdc5dfeed5`, extended in `18e0f40199` |
| `test/managed/fleet-inventory-report.ci` | Done | Created in `18e0f40199` (+ fixture drivers) |
| `test/managed/fleet-health-report.ci` | Done | Created in `18e0f40199` (+ fixture drivers) |
| `docs/architecture/fleet-config.md` | Done | Modified in `18e0f40199` |
| `docs/architecture/api/commands.md` | Done | Modified in `18e0f40199` |

### Audit Summary
- **Total items:** 54 (5 Task requirements, 18 ACs, 13 TDD tests, 19 planned files)
- **Done:** 52
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 2 (AC-12 and web half of AC-4 moved to post-release fleet-ui specs by owner ruling of 2026-11-23)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Managed devices push bounded hardware inventory (`inventory-report`, on connect + daily, excluding raw SMART/drive serials) and hub stores it across disconnect/restart | functional + unit | `test/managed/fleet-inventory-report.ci` (`./le --name fleet4 test managed --pattern fleet-inventory-report`, PASS 48.5s); `TestProjectFleetInventory` (`internal/component/host/fleet_inventory_test.go`, PASS 1.040s); `TestInventoryReportHandler` and `TestReportDisconnectRestart` (`internal/component/plugin/server/fleet_report_test.go`, PASS) |
| Managed devices push component health (`health-report`, on connect + 5m, matching `/health`) and hub stores and aggregates it across declared devices | functional + unit | `test/managed/fleet-health-report.ci` (`./le --name fleet4 test managed --pattern fleet-health-report`, PASS 11.2s); `TestHealthReportJSONMatchesHealthHandler` and `TestClientSendsHealthReport` (`internal/component/managed/report_test.go`, PASS); `TestHealthReportHandler` and `TestFleetHealthAggregation` (`internal/component/plugin/server/fleet_report_test.go`, PASS) |
| Operators inspect per-device inventory, per-device health, and fleet health summary on every command surface (CLI, SSH, MCP, REST, gRPC, web CLI), including explicit `not-reported` state and offline preservation | functional + surface parity | `TestEveryCommandOnEverySurface` (`cmd/ze/hub/surface_parity_test.go`, 0 failures on any fleet command across all 8 surfaces); `TestShowFleetReportsUnreportedAndReportedAndDisconnected` and `TestShowFleetReportsRefusalsAndCompletion` (`internal/component/managed/cmd_reports_test.go`, PASS 24.856s); `test/managed/fleet-inventory-report.ci` and `test/managed/fleet-health-report.ci` (PASS) |
| Input bounds, per-connection rate limits, forward compatibility with pre-Fleet-4 hubs, and concurrent store ownership hold without disrupting config sync or heartbeats | security / fault-injection unit | `TestInventoryReportPayload` and `TestHealthReportPayload` (`pkg/fleet/report_test.go`, PASS 1.054s); `TestReportRefusalsPreserveLastValue` (`internal/component/plugin/server/fleet_report_test.go`, PASS 40.935s); `TestReportStoreSnapshotOwnership` (PASS); `TestReportUnknownMethodAndTimeout` (`internal/component/managed/report_test.go`, PASS) |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| AC-4 web half ("fleet dashboard shows degraded") | Owner ruling of 2026-11-23 ("1 b"): first-release fleet features ship through commands on every command surface; web pages come after first release | `plan/post-release/features/spec-fleet-ui-3-dashboards-and-charts.md` (AC-8) |
| AC-12 device table inventory/health columns and side panel | Owner ruling of 2026-11-23 ("1 b"): web views move to post-release fleet-ui specs | `plan/post-release/features/spec-fleet-ui-2-estate-tree-and-device-tables.md` (AC-16) |
| AC-12 device detail page with full inventory and health | Owner ruling of 2026-11-23 ("1 b"): web views move to post-release fleet-ui specs | `plan/post-release/features/spec-fleet-ui-4-device-tabs-and-console.md` (AC-8) |
| `InventoryHealthPart` web source registration | Owner ruling of 2026-11-23 ("1 b"): web source parts over first-release commands move to the post-release fleet-ui spec that first reads them | `plan/post-release/features/spec-fleet-ui-2-estate-tree-and-device-tables.md` (AC-17) |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fleet-4-inventory-health-0d4a383c-79d8-4d26-887f-bf4fda8abb1c.md` |
| `./le spec review check` | `review_gate: OK (27 code files, clean, hashes match tmp/review/fleet-4-inventory-health-0d4a383c-79d8-4d26-887f-bf4fda8abb1c.md)` |
| Rounds | 1 |
| Reviewer lenses used | logic+wiring, security+allocation+edge-cases, spec-completeness (`/ze-review-spec`), design-documents, Ze Go style 7 questions |

### Run 1 (independent closure review over commits `5e50b70671`, `cdc5dfeed5`, `9101bad6fe`, `18e0f40199`)
- **Automated pre-checks:** `./le repo check` (0 issues on fleet-4 files; 2 unrelated pre-existing `numberparse-allowlist` issues in `bgp/plugins/rib/aggregate.go` and `internal/le/test/deployment/vppevidencecleartext.go`); `./le test weakened check base origin/main` (`clean`, 28 changed test files examined); `./le doc check all` (0 fleet-4 source anchor errors, 564/564 command-contract and usage-contract OK).
- **Ze Go Style 7-question pass:** Ran over all 24 changed/created Go files — 0 socket-reachable panics (`MustRegister*` and `RegisterEventCode` called only from `init()` in `register*.go`), all loops/maps/payloads bounded (`ReportBodyMax`, `InventoryDriversMax`, `HealthComponentsMax`, `ManagedClientsMax`, `defaultReportCallTimeout`), lifecycle obligations stated with `MUST` on `startSessionReporter` and `Snapshot*`, deep copies prevent snapshot aliasing, zero values refused (`HealthStatusUnspecified`), names carry concepts.
- **Design documents lens:** Verified all 12 derived design pages and decision rows (`agrees`; 0 `departs` rows; no conflict with the 2026-11-26 `Ze always starts` and listener-certificate decisions).
- **Verdict:** 0 BLOCKER, 0 ISSUE, 1 NOTE.

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | NOTE | `(*ManagedServer).FleetHealthSummary()` returns `(ManagedFleetHealthSummary, error)` with a constant `nil` error (matching `ManagedFleetHealth()`'s signature). Does not affect correctness or callers. | `internal/component/plugin/server/fleet_report.go` (`FleetHealthSummary`) | acknowledged |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| none | — | Run 1 independent review over all 27 code files returned 0 BLOCKER and 0 ISSUE (all Phase 4 review findings were already fixed before closure in commit `18e0f40199`) | — | — |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `pkg/fleet/report.go` | Yes | `-rw-r--r-- 1 thomas thomas 19849 Nov 26 12:45 pkg/fleet/report.go` |
| `pkg/fleet/report_test.go` | Yes | `-rw-r--r-- 1 thomas thomas 22799 Nov 26 12:39 pkg/fleet/report_test.go` |
| `internal/component/host/fleet_inventory.go` | Yes | `-rw-r--r-- 1 thomas thomas 5208 Nov 26 13:09 internal/component/host/fleet_inventory.go` |
| `internal/component/host/fleet_inventory_test.go` | Yes | `-rw-r--r-- 1 thomas thomas 7919 Nov 26 12:54 internal/component/host/fleet_inventory_test.go` |
| `internal/component/managed/report.go` | Yes | `-rw-r--r-- 1 thomas thomas 9566 Nov 26 12:52 internal/component/managed/report.go` |
| `internal/component/managed/report_test.go` | Yes | `-rw-r--r-- 1 thomas thomas 21119 Nov 26 13:09 internal/component/managed/report_test.go` |
| `internal/component/managed/register_report.go` | Yes | `-rw-r--r-- 1 thomas thomas 452 Nov 26 12:52 internal/component/managed/register_report.go` |
| `internal/component/managed/cmd_reports.go` | Yes | `-rw-r--r-- 1 thomas thomas 7355 Nov 26 13:14 internal/component/managed/cmd_reports.go` |
| `internal/component/managed/cmd_reports_test.go` | Yes | `-rw-r--r-- 1 thomas thomas 17870 Nov 26 17:16 internal/component/managed/cmd_reports_test.go` |
| `internal/component/plugin/server/fleet_report.go` | Yes | `-rw-r--r-- 1 thomas thomas 10000 Nov 26 13:09 internal/component/plugin/server/fleet_report.go` |
| `internal/component/plugin/server/fleet_report_test.go` | Yes | `-rw-r--r-- 1 thomas thomas 31071 Nov 26 16:52 internal/component/plugin/server/fleet_report_test.go` |
| `internal/test/fixture/register_managed_fleet_reports.go` | Yes | `-rw-r--r-- 1 thomas thomas 374 Nov 26 13:15 internal/test/fixture/register_managed_fleet_reports.go` |
| `internal/test/fixture/misc_fixture_managed_fleet_reports.go` | Yes | `-rw-r--r-- 1 thomas thomas 16046 Nov 26 13:18 internal/test/fixture/misc_fixture_managed_fleet_reports.go` |
| `test/managed/fleet-inventory-report.ci` | Yes | `-rw-r--r-- 1 thomas thomas 866 Nov 26 13:16 test/managed/fleet-inventory-report.ci` |
| `test/managed/fleet-health-report.ci` | Yes | `-rw-r--r-- 1 thomas thomas 914 Nov 26 13:16 test/managed/fleet-health-report.ci` |
| `website/changes/fragments/fleet-4-inventory-health.md` | Yes | `-rw-r--r-- 1 thomas thomas 1881 Nov 26 17:40 website/changes/fragments/fleet-4-inventory-health.md` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1, AC-2 | Client sends `inventory-report` after config-fetch + 24h; hub stores it | Verified `runConnection` (`internal/component/managed/client.go`) calls `startSessionReporter` after `fetchAndProcess`, `host/register.go` registers `FleetInventoryInterval` (24h), `server/fleet_report.go` `handleInventoryReportPayload` stores via `recordInventoryReport`; `TestClientSendsInventoryReport`, `TestInventoryReportHandler`, `fleet-inventory-report.ci` PASS |
| AC-3, AC-4 | Client sends `health-report` after fetch + 5m; hub stores and aggregates degraded component | Verified `managed/register_report.go` registers `FleetHealthInterval` (5m), `ProjectFleetHealth` matches `/health`, `recordHealthReport` and `FleetHealthSummary` aggregate status; `TestClientSendsHealthReport`, `TestHealthReportJSONMatchesHealthHandler`, `TestHealthReportHandler`, `TestFleetHealthAggregation`, `fleet-health-report.ci` PASS |
| AC-5, AC-6, AC-7, AC-8 | Three commands show inventory, health, estate aggregate, and explicit `not-reported` | Verified `handleFleetDeviceInventory`, `handleFleetDeviceHealth`, `handleFleetHealth` (`internal/component/managed/cmd_reports.go`) and `ze-managed-cmd.yang`; `TestShowFleetReportsUnreportedAndReportedAndDisconnected` (PASS 24.856s), `fleet-inventory-report.ci` (PASS 48.5s), `fleet-health-report.ci` (PASS 11.2s) |
| AC-9, AC-10, AC-11 | Session-scoped auth, old-hub `unknown method:` preserving ping + config-fetch, client per-session suppression and 30s call timeout | Verified `managed_registry.go` (`device.conn == mc`), `managed_serve.go` (`handleRequest` default), `managed/report.go` (`isUnknownMethodError`, `defaultReportCallTimeout`); `TestReportRefusalsPreserveLastValue` (PASS 40.935s), `TestReportUnknownMethodAndTimeout` (PASS) |
| AC-13 | All three commands offered on CLI, SSH, MCP, REST, gRPC, web CLI | Verified `managed/register.go` `RegisterRPCs`, `ze-managed-cmd.yang`, `wire-methods.snapshot`; `TestRegisteredWireMethods` (PASS 1.148s), `TestEveryCommandOnEverySurface` (0 failures on fleet commands across all 8 surfaces), `./le doc yang-contract command-contract` (564 commands OK) |
| AC-14, AC-15 | Disconnect + restart preserve snapshots and `received-at`; refusals preserve prior data and interval | Verified `decodeDeviceRecord` (`managed_record_store.go`), `handleReport` (`fleet_report.go`); `TestReportDisconnectRestart` and `TestReportRefusalsPreserveLastValue` PASS |
| AC-16, AC-17, AC-18 | Per-section availability, inapplicable health omitted, ECC/DMI/version included, SMART/drive serials excluded, concurrent store ownership and deletion reconciliation | Verified `host/fleet_inventory.go` (`ProjectFleetInventory`), `pkg/fleet/report.go` (`Validate`, `Clone`), `managed_record_store.go` (`syncDeviceRecord`, `observeDeviceRecord`, `forgetDeviceRecordLocked`); `TestProjectFleetInventory` (PASS 1.040s), `TestInventoryReportPayload` (PASS 1.054s), `TestReportStoreSnapshotOwnership` (PASS) |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Client connects, completes `config-fetch`, sends `inventory-report`, queried via `show fleet device inventory name` online and after disconnect | `test/managed/fleet-inventory-report.ci` | Read `.ci` and `managedFleetInventoryReportDriver` (`internal/test/fixture/misc_fixture_managed_fleet_reports.go`): checks unreported `edge-par`, waits for live inventory report over TLS MuxConn, checks absence of SMART/drive-serials, verifies offline retention with identical `received-at`, checks unknown-device refusal; PASS (48.5s) |
| Client sends `health-report`, queried via `show fleet device health name` and `show fleet health` online and after disconnect | `test/managed/fleet-health-report.ci` | Read `.ci` and `managedFleetHealthReportDriver` (`internal/test/fixture/misc_fixture_managed_fleet_reports.go`): waits for live health report, verifies unreported `edge-par`, checks `show fleet health` aggregate (`total: 2`, `unreported: 1`), verifies disconnect preserves status/`received-at`/`checked-at` with `connected: false`, checks unknown-device refusal; PASS (11.2s) |
| The three commands on CLI, SSH, MCP, REST, gRPC, and web CLI | `cmd/ze/hub/surface_parity_test.go` (`TestEveryCommandOnEverySurface`) | Verified `ze-managed-cmd.yang`, `managed/register.go` (`RegisterRPCs`), `wire-methods.snapshot`; `TestEveryCommandOnEverySurface` reported 0 failures on any fleet command across all 8 surfaces |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| R-1 (report RPC never gets an answer) | confirmed | `defaultReportCallTimeout` (30s) bounds each call in `sessionReporter.sendOnce` (`internal/component/managed/report.go`), `stopReporter` cancels and joins before `mc.Close()`; proven by `TestReportUnknownMethodAndTimeout` (missing reply times out while `ping` and `config-changed` succeed) |
| R-2 (stale/superseded connection tries to write report) | confirmed | `recordInventoryReport` and `recordHealthReport` (`internal/component/plugin/server/managed_registry.go`) require `device.conn == mc` under `s.mu`; proven by `TestReportRefusalsPreserveLastValue` |
| R-3 (partial hardware inventory or hub restart between report kinds) | confirmed | Per-section `Available` flags in `pkg/fleet/report.go` + `host/fleet_inventory.go` and optional `Inventory`/`Health` pointers in `managed_record_store.go` (`decodeDeviceRecord`); proven by `TestProjectFleetInventory`, `TestInventoryReportPayload`, `TestReportDisconnectRestart` |
| R-4 (slow health callbacks delaying liveness/config) | confirmed | `startSessionReporter` runs a single session-scoped worker separate from `notificationLoop` and heartbeat goroutines in `internal/component/managed/client.go`; proven by `TestReportUnknownMethodAndTimeout` |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Architecture / wire protocol (`docs/architecture/fleet-config.md`) | Updated in `18e0f40199` with `ManagedInventorySnapshot`/`ManagedHealthSnapshot`, RPC table rows, and `### Inventory and health reporting` anchored to `pkg/fleet/report.go`, `host/fleet_inventory.go`, `managed/report.go`, `plugin/server/fleet_report.go` | Verified against source and `./le doc check all` (0 fleet-4 anchor errors) |
| Command & API surface (`docs/architecture/api/commands.md`, `docs/guide/command-reference.md`, `docs/features/cli-commands.md`) | Updated in `18e0f40199` (`commands.md`) and Commit A (`command-reference.md`, `cli-commands.md`) anchored to `managed/cmd_reports.go`, `managed/register.go`, `ze-managed-cmd.yang` | Verified against `ze-managed-cmd.yang` and `./le doc yang-contract command-contract` (564 commands OK) |
| User guide & status (`docs/guide/fleet-config.md`, `docs/guide/status.md`, `docs/features/fleet-management.md`, `features/fleet-management.md`) | Updated in Commit A with the 3 commands, `not-reported`/disconnect semantics, `doctor-managed-report-failed`, and `fleet-inventory-report.ci` / `fleet-health-report.ci` | Verified against `cmd_reports.go`, `report.go`, `register_doctor_codes.go`, and `ls` of `.ci` files |
| Doctor & report bus (`docs/architecture/doctor-and-health-checks.md`) | Updated in Commit A with `doctor-managed-report-failed` (`CodeDoctorManagedReportFailed`) anchored to `register_doctor_codes.go` and `report.go` | Verified against `register_doctor_codes.go` and `./le arch doctor-coverage check` (0 findings on fleet-4) |
| Config syntax / RFC compliance / comparison table | No config YANG (`-conf`) leaves or RFC wire protocols changed; `git grep` confirms no stale claims in `docs/guide/configuration.md` or `docs/comparison.md` | Verified (N/A) |
