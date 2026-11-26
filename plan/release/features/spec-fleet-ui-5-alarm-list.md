# Spec: fleet-ui-5 -- The Hub Alarm List and Its Commands

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | fleet, cli |
| Priority | P2 |
| Release reason | Thomas put the fleet alarm list in the first release, reached through its commands on every command surface (`ai/META-DECISIONS.md`, "Goal and scope"); existing supported behavior and its obligations remain required. Its pages, the hub's task model and panel, notifications and live page updates come after the first release, in `plan/post-release/features/spec-fleet-ui-9-tasks-notifications-and-alarm-pages.md`. |
| RFC gaps | - |
| Depends | spec-fleet-1-device-registry (closed 2026-11-25; documented in `docs/architecture/fleet-config.md`, "Device registry"), spec-fleet-4-inventory-health (closed 2026-11-26; documented in `docs/architecture/fleet-config.md`, "Inventory and health reporting"), plan/release/features/spec-fleet-8-metric-history.md |
| Phase | 5/9 of plan/post-release/features/spec-fleet-ui-0-umbrella.md |
| Handoff | - |
| Updated | 2026-11-25 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

**Re-read after compaction:** this file, `plan/post-release/features/spec-fleet-ui-0-umbrella.md`,
`plan/post-release/features/spec-fleet-ui-9-tasks-notifications-and-alarm-pages.md`,
`internal/core/report/report.go`.

## Task

Make what is wrong in the estate visible at all times. Every condition that
needs attention is listed once, with its device, its severity and since when,
by the hub's alarm model and the commands that expose it on every command
surface: the CLI, MCP, REST, gRPC, SSH and the web command surface. Nothing
that is wrong has to be discovered by opening devices one by one.

On 2026-11-23 Thomas ruled that the fleet features of the first release are
reached through commands, and that their web pages, the hub's task model and
panel, notifications and live page updates come after it (`ai/META-DECISIONS.md`,
"Goal and scope"). This spec was written with all of those; it keeps the
alarm model and its commands, and the rest moved, unchanged, to
`plan/post-release/features/spec-fleet-ui-9-tasks-notifications-and-alarm-pages.md`
(see "Moved to fleet-ui-9" below).

### Alarms

Ze has no alarm manager. An alarm here is a condition that is true now, derived
by the hub from what it already holds. It opens when the condition becomes true
and clears when it stops being true. Nothing is acknowledged or silenced.

| Alarm | Opens when | Severity |
|-------|-----------|----------|
| Device offline | A device has had no managed connection for longer than a configured time | Critical |
| Component down | A device's health report names a component down | Critical |
| Component degraded | A device's health report names a component degraded | Warning |
| Configuration diverged | A device holds a locally edited configuration (`fleet-6`) | Warning |
| Configuration rejected | A device refused the configuration the hub sent | Warning |
| Resource high | A reported CPU, memory or storage figure stays above a configured threshold for a configured time | Warning |
| Software behind | A device runs a version older than a configured number of versions behind the newest in the estate | Informational |
| Relay not granted | Once fleet-9 supplies the relay capability and an explicit grant state, a relay-capable device reports that it has no grant. An unavailable provider or an unknown grant supplies no such alarm. | Informational |
| History full | The metric history reached its size cap and is dropping samples early | Warning |

The list is open: an alarm kind is a registered entry with its condition, its
severity and its sentence, so a later feature adds one without editing the list.

Alarm causes, which hide an alarm while the alarm that causes it is open, are
specified in `plan/release/features/spec-fleet-ui-8-alarm-causes.md`.

Where the alarms appear on the web (the Alarms tabs, the Alarms card, the tree
badges, the toasts and a row's side panel) is
`plan/post-release/features/spec-fleet-ui-9-tasks-notifications-and-alarm-pages.md`,
"Alarm pages".

### Commands this spec adds

Every operator command must run on every surface, so the alarm list is a
command first.

| Command | Answers |
|---------|---------|
| `show fleet alarm` | Open alarms, filterable by severity, kind and device |
| `show fleet alarm history device <name>` | A device's alarm openings and clearings over the retention period |

The task commands (`show fleet task`, `show fleet task id <id>`,
`clear fleet task id <id>`) moved with the task model to `fleet-ui-9`.

### Key design decisions

| Decision | Detail |
|----------|--------|
| Alarms are derived, not stored events | The hub already holds every fact an alarm states. Deriving them means an alarm can never disagree with the device's state shown beside it |
| No acknowledgement | An acknowledged alarm is a second state to keep true. The first version shows what is wrong now; a fixed condition clears itself |

An occurrence is identified by its registered kind, device (or hub for History full), subject and a unique opening identifier. The subject distinguishes two down components on one device and CPU from memory pressure. Polling the same state preserves its identifier and opening time. A false-to-true transition starts another occurrence. Transition processing records an opening or clearing once, independently of whether any operator happens to read the list. The current condition comes from its producer. The historical record answers when it held, including after the source changes again.

The latest health report and metric sample keep their original receipt times across a disconnect. An already open Resource high occurrence remains open on the offline device until reconnect, subject to the visibility rules of fleet-ui-8. Reconnect ends that occurrence. Neither the retained value nor a missing interval advances the new threshold timer. Only new observations spanning the configured duration qualify for another opening. A component that remains down keeps its original occurrence. A missing relay grant, once the capability exists and its absence is known, remains visible while offline.

The decisions "Tasks are the hub's own model", "Events name fragments" and
"One stream per page" moved to `fleet-ui-9`.

### Dependencies and readiness

| Dependency | State |
|------------|-------|
| Device registry and its change notifications | Missing: `fleet-1` |
| Health reports | Missing: `fleet-4` |
| Latest samples for the resource alarms | Missing: `fleet-8` |
| The report bus and its bounded lists | Available on each device; the hub's own alarms use the same shape |

The task dependencies (the actions that create tasks, the state store key for
finished tasks) moved to `fleet-ui-9`.

## Acceptance Criteria

The identifiers are those this spec was written with, so the references other
specs hold stay true. AC-1, AC-2, AC-3, AC-5, AC-6 and AC-7 moved whole to
`fleet-ui-9`; AC-4 and AC-8 keep their command part here and their page part
moved (table below).

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-4 | A condition in the alarm table becomes true, then false | The alarm opens in `show fleet alarm` with its kind, device, severity and since, then clears from it; `show fleet alarm history device <name>` lists its opening and its clearing | Its history row remains, because the history is the record that it happened |
| AC-8 | `show fleet alarm` and `show fleet alarm history device <name>` on every command surface: the CLI, MCP, REST, gRPC, SSH and the web command surface | The same alarms on every surface, as data answering every pipe | None |
| AC-9 | Two components fail on one device; a repeated report, reads on two command surfaces and a subsequent clearing follow | Open two occurrence IDs. Their opening times stay stable on repeat, each clear is recorded once, and repeated reads produce no transitions. | Other devices' occurrences remain distinct even when their component names match. |
| AC-10 | A resource occurrence is open when its device drops and reconnects with the last sample still high | Keep the previous measured value with its age while offline; close the prior occurrence at reconnect. Only qualifying new samples open a new ID. A fresh below-threshold reading resets the interval. | An unresolved component-down condition retains its own opening time. |
| AC-11 | No relay provider, an unknown grant on a capable device, a known absent grant and a known grant | Only the known absent grant on a relay-capable device opens this kind. Its own registration accompanies fleet-9. | Other first-release alarms are available with the entire relay capability absent. |
| AC-12 | The hub restarts with an open condition and a closed occurrence in retention | Restore open occurrence identity and history. Reconcile with fresh facts, preserving timestamps for conditions still true and recording a new observed clear when a fresh fact ends one. Never invent a precise transition time for an interval during which the hub collected no facts. | Old entries beyond the published retention boundary may be absent. |
| AC-13 | More conditions match than fit in one answer; the caller can see only a subset of devices | Return a bounded page, a stable continuation for each remaining page, and the total number of matching authorized occurrences. Counts by severity and visibility cover the same complete authorized result. History reports its retained boundary. | A page renders its own rows. Its heading and badges use the counts for the full authorized result. |
| AC-14 | Declare a new device, reload or restart the hub during its grace period, and leave it unconnected. | The hub opens Device offline when the original grace period expires. Connecting before expiry cancels the pending opening. Removing then re-adding the name starts a new interval. | Devices declared before the hub has any durable timestamp start at the first recorded observation, because the earlier declaration time is unknown. |

### Moved to fleet-ui-9

Every item below is owned by
`plan/post-release/features/spec-fleet-ui-9-tasks-notifications-and-alarm-pages.md`,
which names this spec as the source of each.

| Item moved | Was here | Now in `fleet-ui-9` |
|------------|----------|---------------------|
| AC-1, a task exists for every fleet action | AC-1 | AC-1 |
| AC-2, a task that partly fails | AC-2 | AC-2 |
| AC-3, a task across a hub restart | AC-3 | AC-3 |
| AC-4, the alarm appears in and clears from every page place | AC-4, page part | AC-4 |
| AC-5, live change without a reload | AC-5 | AC-5 |
| AC-6, no event about a device the operator may not see | AC-6 | AC-6 |
| AC-7, the stream is lost | AC-7 | AC-7 |
| AC-8, `show fleet task` on every command surface | AC-8, task part | AC-8 |
| AC-8, the commands answer the same data as the pages | AC-8, the "same data as the pages" clause for alarms | AC-9 |
| The task model, its kinds, its places and its rules | "Tasks" | "Tasks" |
| Where alarms appear on the web, and the side panel's week of history | "Alarms", "Where alarms appear" | "Alarm pages" |
| Notifications | "Notifications" | "Notifications" |
| Live updates | "Live updates" | "Live updates" |
| `show fleet task`, `show fleet task id <id>`, `clear fleet task id <id>` | "Commands this spec adds" | "Commands this spec adds" |
| Decisions "Tasks are the hub's own model", "Events name fragments", "One stream per page" | "Key design decisions" | "Key Design Decisions" |
| The actions that create tasks; a state store key for finished tasks | "Dependencies and readiness" | "Dependencies and readiness" |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| The task model, notifications, live updates and the alarm pages, with the ACs in "Moved to fleet-ui-9" | Thomas ruled on 2026-11-23 that they come after the first release (`ai/META-DECISIONS.md`, "Goal and scope") | `plan/post-release/features/spec-fleet-ui-9-tasks-notifications-and-alarm-pages.md` |

## Existing Mechanisms

| Mechanism | Source | Use here |
|-----------|--------|----------|
| Declared-client snapshots and managed acknowledgements | `internal/component/plugin/server/managed_registry.go`, `ManagedRegistrySnapshot`, `disconnectDevice`, `recordAck` | Derive connectivity and configuration state from the authenticated client and the actual fetch or ACK result, including a held local edit. |
| Component health and sample history | Fleet-4's retained health report and fleet-8's accepted, timestamped metric set | Treat the last observation, its age and the current connection as separate facts. A missing figure cannot become a threshold violation. |
| Issue identities and settling | `internal/core/report/report.go`, `RaiseWarningFix`, `ClearWarning`; `internal/core/stats/hysteresis.go`, `Reset` | The existing report bus keys issues by source, code and subject. Fleet occurrences also distinguish the device. A sample-count latch alone cannot measure wall-clock duration across a gap. |
| Existing web contract | `internal/component/web/fleetui/source_activity.go`, `Alarm`, `AlarmEvent`, `AlarmsMax`, `AlarmHistoryMax` | The post-release page expects stable IDs, device, severity, summary and time. The backend publishes these facts through data commands on every surface; web code is optional. |
| Command registration and caller | `internal/component/managed/register.go`, `pluginserver.RegisterRPCs`, `command.RegisterShape`, `RegisterColumns`, `RegisterPipeFilters` | Register the two read commands alongside the device commands. Use their trusted request context for device-scope checks before computing counts and issuing a cursor. |
| Declaration and first admission | `internal/component/plugin/server/managed_apply.go`, `ApplyManagedListeners`; `managed_registry.go`, `admitOnListener` | Record the start of each new declaration. Compare declaration membership across reloads to keep the start for an unchanged device. On admission the current generation's first connection cancels its unexpired timer. Re-addition after removal starts a new declaration even if the device has a remembered record. |
| Borrowed state store and restart | `internal/component/plugin/server/managed_serve.go`, `ManagedServerConfig.Store`; `managed_record_store.go`, `loadDeviceRecords`, `stopDeviceRecords`; `pkg/zefs/keys.go` | Use the same borrowed storage and lifecycle for separately registered alarm shards. Load the recorded declaration start and open IDs before evaluating deadlines. Drain the alarm writer at shutdown. |
| Config preparation and application | `internal/component/config/loader_extract.go`, `ExtractHubConfig`; `cmd/ze/hub/managed_server.go`, `prepareManagedReload` | Extract and validate the grace and alarm settings once. Pass them through server construction and the prepared reload plan, applying them only with the accepted generation. |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | One schema, one command registry, every interface | the door every change reads | "A command is declared once and reaches every interface." | agrees | The daemon command supplies one authorized answer to all transports. |
| `docs/architecture/api/commands.md` | Transport Request Metadata | anchored to internal/component/managed/yang/ze-managed-cmd.yang | "Caller identity is injected only by trusted transport wiring." | agrees | Apply device visibility from the authenticated caller before computing totals or enumerating devices. |
| `docs/architecture/fleet-config.md` | RPCs | anchored to internal/component/managed/yang/ze-managed-cmd.yang | "All RPCs are scoped to that name implicitly." | agrees | Bind each input to the registered authenticated session and reject stale sessions. The payload cannot select a different device. |
| `ai/doors/hub-engine.md` | Boundaries | the door covering internal/component/managed/yang/ze-managed-cmd.yang | "No always-on file imports a gated package." | agrees | Discover optional producers by registration. The hub and managed client import no optional dataplane feature to obtain its measurements. |
| `docs/architecture/config/syntax.md` | Overview | anchored to internal/component/plugin/yang/ze-plugin-conf.yang | "The parser is YANG-driven: each config node's type (leaf, leaf-list, container, list) determines how it is parsed." | agrees | Declare operator settings and bounds through YANG and its existing schema validation. |
| `ai/doors/plugin-infra.md` | Boundaries | the door covering internal/component/plugin/yang/ze-plugin-conf.yang | "The SDK stays generic." | agrees | Hub-specific state belongs to the managed service. A forked producer supplies facts across its supported boundary. |
| `docs/architecture/zefs-format.md` | Key Registry | the // Design: page of pkg/zefs/keys.go | "All known key patterns are registered in `pkg/zefs/keys.go` via `MustRegister()`" | agrees | Register alarm key families and check each encoded shard against its declared ValueMax in the alarm writer before calling WriteKey. Bound and validate reads before decoding. |
| `docs/architecture/container/image-store.md` | State keys | anchored to pkg/zefs/keys.go | — | silent | The container store owns its image and project records. Fleet uses separate registered state keys and never presents a fleet reading as image state. |
| `docs/architecture/diagnostics/debug-filtering.md` | One ZeFS trap | anchored to pkg/zefs/keys.go | "`List(prefix)` fails when the prefix ends in a slash." | agrees | Enumerate alarm shards through the borrowed storage.Storage.ListKeys with the registered directory prefix, including its slash. The adapter handles the lower-level listing rules. |
| `docs/architecture/irr-owner.md` | Who may touch which key | anchored to pkg/zefs/keys.go | — | silent | IRR grants dataset and interest keys to named routing consumers. These grants do not cover fleet state, which belongs to the managed hub. |
| `docs/architecture/storage-backends.md` | Durable publication | anchored to pkg/zefs/keys.go | "Sync failures are returned to the caller." | agrees | Retain dirty state on failure, expose degraded durability and retry publication without marking an uncommitted record flushed. |
| `ai/doors/core.md` | Boundaries | the door covering pkg/zefs/keys.go | "Runtime state goes through the daemon's one store." | agrees | Use the hub’s borrowed store and registered keys. Optional device producers never open the hub store. |
| `ai/doors/le-tooling.md` | Where it sits | the door covering test/managed/fleet-alarm-lifecycle.ci | "Functional `.ci` files live under `test/<suite>/`" | agrees | The managed suite drives authenticated devices and the same commands served to operators. |
| `docs/architecture/api/commands.md` | Design decisions | Decision of 2026-08-21 | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | Command paths identify the subject. Presentation and generic filtering use the command registry’s existing pipe operations. |
| `docs/architecture/api/commands.md` | Design decisions | Decision of 2026-11-20 | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | These commands return structured data and inherit generic filtering and output shapes. |
| `docs/architecture/api/commands.md` | Design decisions | Decision of 2026-11-23 | "A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does" | agrees | Reading or piping an answer never triggers a device sample, a state transition or a configuration write. |
| `docs/architecture/fleet-config.md` | Design decisions | Decision of 2026-11-23 | "A reload that changes the certificate a serving hub's managed listener presents installs it on the running listener at that reload: the next handshake presents the certificate the configuration names, the listener keeps its socket and no session is closed, as the web and looking glass listeners do. The certificate stays in the configuration's `pki` block. In Thomas's words (voice-transcribed): "Keep the certificate in the configuration." and "In that case on commit we can perform the certificate change on the listener."" | agrees | Existing authenticated sessions feed fleet state. The certificate reload path and its live socket continue to operate independently. |
| `docs/architecture/fleet-interface.md` | Design decisions | Fleet interface owner ruling | "The proposed alarm defaults apply together. A missing relay grant stays visible while the device is offline. A measured alarm uses its existing threshold duration after reconnect. Causes match within one device. Thomas selected “Use all three defaults”." | agrees | Use the same per-device identity and elapsed threshold interval; the first release keeps visibility as command data. |
| `docs/architecture/fleet-interface.md` | Design decisions | Fleet interface owner ruling | "Thomas chose the following rule for Resource high at reconnect. “End the old occurrence. Show a new alarm only if fresh samples qualify.”" | agrees | A fresh receipt after reconnect is distinct from the last known value kept during disconnect. Terminate the old resource occurrence and require new qualifying samples. |
| `docs/architecture/fleet-interface.md` | Design decisions | Fleet interface owner ruling | "When a critical alarm becomes visible again after its cause clears, show its toast to an authorized viewer of the affected device. Thomas selected “Show the critical toast”." | agrees | The release command carries the state. Transient page notifications belong to fleet-ui-9 and are scoped to the viewer. |
| `docs/architecture/fleet-interface.md` | Design decisions | Fleet interface owner ruling | "For Force complete on a staged rollout, Thomas chose “Leave unstarted devices on their prior version”." | agrees | These queries never change a rollout or cause a device to receive configuration. |
| `docs/architecture/fleet-interface.md` | Design decisions | Fleet interface owner ruling | "Thomas chose “Add the device Alarms tab”. The post-release device page offers the same authorized alarm rows as the device filter on the estate or group Alarms tab." | agrees | The later device tab consumes the same authorized per-device facts. All first-release answers are available through commands. |
| `docs/architecture/fleet-alarms.md` | Device offline | Hub alarm design | "A restart preserves the recorded start of a continuing declaration." | agrees | AC-14 verifies the original deadline after a restart or reload. |
| `docs/architecture/fleet-alarms.md` | Design decisions | Hub alarm design | "For a declared device that has never connected, Thomas chose “After the configured offline grace period from declaration”." | agrees | The declared-device observer persists the start of the grace period before the first connection. |
| `ai/rules/repo-maintenance.md` | Directives | Hub and managed service wiring | "the owning `docs/architecture/` page for a contract" | agrees | Update the fleet architecture page with the new lifecycle, command and storage contracts. |
| `docs/architecture/doctor-and-health-checks.md` | Recent events: the report bus in show doctor | Hub and managed service wiring | "Its producer raises it on the report bus" | agrees | Publish degraded evidence and its recovery through the existing managed doctor and report mechanisms. |
| `docs/architecture/pki/tls-listeners.md` | Decision: rotation does not rebind the hub listeners | Hub and managed service wiring | "The listeners keep their sockets, and the next handshake serves the new certificate." | agrees | A certificate-only reload leaves device sessions, declaration membership and their timing intact. |
| `docs/architecture/api/architecture.md` | Command parameter values | Hub and managed service wiring | "The command model then accepts or refuses that value." | agrees | The handler and command model validate the requested subject and query inputs on every surface. |
| `docs/architecture/core-design.md` | 19. Component Boundaries | Hub and managed service wiring | "Each component under `internal/component/` is independently removable." | agrees | Only the feature that owns a measurement contributes it. The managed service consumes registered observations. |
| `docs/architecture/hub-architecture.md` | SSH session model factory | Hub and managed service wiring | "Both owners finish before the hub returns its store to the launcher." | agrees | The managed service drains outstanding state writers before returning the borrowed store. |
| `ai/doors/config-yang.md` | Where it sits | Hub and managed service wiring | "The schema is spread across components." | agrees | Declare the typed settings in the owning module and have the config loader carry them to the hub. |
| `docs/architecture/fleet-config.md` | Design decisions | Decision of 2026-11-25 | "Thomas chose “Allow first provisioning and later edits from the device’s web Config tab”. An authorized operator can commit the first or a later version of a declared device’s configuration while it is offline, through the post-release fleet web surface." | agrees | Offline provisioning remains available to the authorized device editor. Reading fleet observations cannot modify or apply device configuration. |
| `docs/architecture/pki/tls-listeners.md` | Design decisions | Decision of 2026-11-23 | "The looking glass may serve plain HTTP, because what it publishes is public, and one looking glass serves HTTP and HTTPS at the same time. Thomas's words, as voice-transcribed: "We should allow the looking glass to be HTTP. The information is public." and "We should be able to serve on both HTTP and HTTPS, not either." Asked where the choice lives and what happens to `ze.looking-glass.tls`, he answered "1a 2a": the choice is the `tls` leaf of each `server` entry, default true, and the environment variable is removed. The no-store rule applies to each entry: an entry that left `tls` unset serves HTTP with a warning on a daemon with no `database/` store and no named certificate, an entry that says `tls true` there is refused at start and at reload, and a reload does what a restart with the same config does" | agrees | The hub consumes existing observations across certificate rotation. It does not change any listener protocol, certificate source or socket. |
| `docs/architecture/pki/tls-listeners.md` | Design decisions | Decision of 2026-11-23 | "The web and looking glass use the shared `listenerTLSMaterial` precedence. The approved Fleet reload design shares `managedCertificate` with startup because the daemon authority issues its unnamed leaf." | agrees | The hub consumes existing observations across certificate rotation. It does not change any listener protocol, certificate source or socket. |
| `docs/architecture/api/architecture.md` | Design decisions | Decision of 2026-11-23 | "When `\| implied` modifies an existing config line (such as inserting automatic members into a `filters [ ... ];` list), it keeps the line intact and places an inline `# implied: ...` comment on the same line after the statement naming what was added and why, rather than placing a comment only above the line or splitting the list across multiple lines. In Thomas's words (2026-11-23): "No, it should be on the same line and say what was added"." | agrees | Fleet reads register their result shapes and do not alter configuration display, including inline implied annotations. |
| `docs/architecture/core-design.md` | Design decisions | Decision of 2026-11-23 | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | Reading fleet facts never selects a data plane or installs configuration on kernel or VPP. Existing backend compatibility checks apply on every configuration path. |
| `docs/architecture/core-design.md` | Design decisions | Decision of 2026-11-23 | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | Reading fleet facts never selects a data plane or installs configuration on kernel or VPP. Existing backend compatibility checks apply on every configuration path. |
| `docs/architecture/core-design.md` | Design decisions | Decision of 2026-11-23 | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | Reading fleet facts never selects a data plane or installs configuration on kernel or VPP. Existing backend compatibility checks apply on every configuration path. |
| `docs/architecture/core-design.md` | Design decisions | Decision of 2026-11-23 | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | Reading fleet facts never selects a data plane or installs configuration on kernel or VPP. Existing backend compatibility checks apply on every configuration path. |
| `docs/architecture/core-design.md` | Design decisions | Decision of 2026-11-23 | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | The fleet consumes observations and does not register an import or export filter, rewrite any next hop or change implied filter display. |
| `docs/architecture/core-design.md` | Design decisions | Decision of 2026-11-23 | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | The fleet consumes observations and does not register an import or export filter, rewrite any next hop or change implied filter display. |
| `docs/architecture/core-design.md` | Design decisions | Decision of 2026-11-23 | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | The fleet consumes observations and does not register an import or export filter, rewrite any next hop or change implied filter display. |
| `docs/architecture/core-design.md` | Design decisions | Decision of 2026-11-23 | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | The fleet consumes observations and does not register an import or export filter, rewrite any next hop or change implied filter display. |
| `docs/architecture/core-design.md` | Design decisions | Decision of 2026-11-23 | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | The fleet consumes observations and does not register an import or export filter, rewrite any next hop or change implied filter display. |
| `docs/architecture/core-design.md` | Design decisions | Decision of 2026-11-23 | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | The fleet consumes observations and does not register an import or export filter, rewrite any next hop or change implied filter display. |
| `docs/architecture/core-design.md` | Design decisions | Decision of 2026-11-23 | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | The fleet consumes observations and does not register an import or export filter, rewrite any next hop or change implied filter display. |

## Data Flow

1. Register a condition in its producer's package. A declaration crossing a process boundary carries stable names, severity and subject semantics. Supply facts over the existing event and dispatcher paths as needed. The built-in hub may evaluate a condition derived from its own registry. Reject duplicate kind registrations. Removing the producer removes the capability and its kinds; an unavailable measurement is distinct from a recovery.
2. Evaluate on a new registry, health, ACK or metric fact and on deadline expiry for offline detection. Use the authenticated name and accepted receipt time. A previously connected device uses its disconnect time for the delay. Use the recorded declaration time while the current declaration has never admitted a connection. FirstConnected measures admission, whereas LastFetch measures a later, successful config fetch. A client can connect without fetching. Record admission for the current declaration separately when a re-added name still has a FirstConnected from its previous declaration. Restore that declaration’s start and admission state on restart.
3. Serialize transition evaluation and recording for a given subject. Persist the current occurrence's ID and opening time alongside its bounded transition history in one atomic shard update. Repeated reports, retries and restart reconciliation must be idempotent. A failed write marks history degraded and remains eligible for retry. Continue to report the producer's live condition and explicitly identify the gap in durable evidence. Reject stale updates from superseded sessions.
4. Derive Software behind only when the device's software version and the configured comparison baseline can be ordered. Expose the version used and skip unknown versions. A device refusing an invalid configuration and a device holding a local edit yield different conditions. History full is scoped to the hub and carries the history store's actual cap, use and effective retention.
5. Authorize scope before filtering, sorting and counting. Propose a page of up to 1,024 open occurrences (including hidden ones) and up to 512 history results per page; the existing optional web interface has those bounds. Return a stable continuation tied to the authorized caller, filters, ordering and snapshot, together with its full matching total and severity and visibility counts. Consume every page to enumerate all matching occurrences, even when 2,000 devices disconnect. A new occurrence cannot displace an older one in an existing listing. Recheck access on every continuation; a changed grant, changed filter or expired snapshot requires an explicit restart. Headings and badges use the counts across all authorized pages.
6. Propose a seven-day episode horizon, up to 512 retained closed episodes per device and 65,536 across the hub. Evict the oldest closed episode first under pressure, expose the earliest retained time and truncation, and preserve the opening identity of every active condition. Retain a cause's overlap for the retained portion of a dependent episode, or mark earlier visibility unknown when required evidence was evicted. Fleet-ui-8 derives hidden intervals from occurrence history without storing a parallel hidden flag.

### Changed-Wire Census

**Search evidence:** Searched the tracked Go, templates, JS, CLI fixtures and web test data for show fleet alarm, AlarmEvent, AlarmHistoryMax and AlarmsMax. The current bound is in source_activity.go; scopedAlarms renders the resulting rows and sample/activity.go already produces sample events. The first-release daemon command and persistent alarm record are new.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|---|---|---|---|---|
| open-alarm and alarm-history answers | writer | managed/cmd_alarms.go registered query handlers; plugin/server/managed_alarms.go occurrence snapshots and cursor generation | source_activity.go defines the later Alarm/AlarmEvent consumer vocabulary. | Return authorized total and severity/visibility counts, stable occurrence IDs, timestamp and a bounded page with cursor. |
| same | reader | CLI, SSH, REST, MCP, gRPC and web command dispatchers; later fleetui/source_activity.go, handler_alarms.go scopedAlarms and per-device history page; external operator clients | Page bounds, the sample activity producer and fleet dashboard tests are existing controls. | Read all pages of the same authorized snapshot; restart a listing on an expired or invalidated continuation. |
| alarm occurrence shards and declaration timing | writer | plugin/server/managed_alarms.go serialized transitions; managed_apply.go accepted declaration generation; managed_registry.go admission and disconnect events | ManagedServerConfig.Store is borrowed from the daemon. | Persist the occurrence with its time and declaration generation in bounded versioned shards. |
| same | reader | plugin/server/managed_alarms.go startup/reconciliation before deadline evaluation; history query snapshot | Restart must preserve opening IDs and the original grace start. | An old store without declaration timing begins at the first recorded observation. Reject corrupt shards and expose degraded evidence. |


`show fleet alarm` and `show fleet alarm history device <name>` publish the same daemon answer through the CLI, SSH, REST, gRPC, MCP and the web command line. The typed web adapter in `internal/component/web/fleetui/source_activity.go` consumes the result when the later pages ship. Its answer bounds limit the rows returned per page. The cursor, authorized total and severity and visibility counts describe the complete filtered snapshot. Exercise a second page on the same snapshot while another incident opens, permission loss between reads, and an estate larger than the first page. These commands read the hub’s facts. Device registration, health reporting and sampling retain their own managed protocol owners.

## Wiring Test

| Entry point | Observation |
|-------------|-------------|
| `test/managed/fleet-alarm-lifecycle.ci` | Connect a declared device, change its health, lose the session and reconnect. Compare the command's IDs, times and scope with independently observed events, and require a new Resource high occurrence only after fresh samples qualify. |
| `test/managed/fleet-alarm-history.ci` | Drive opening, repeat, clear and reopen while nobody reads the list. Restart the hub and compare live and retained rows, including truncation under a reduced test budget. |
| Command registration and bounded answer tests | Exercise every command surface, filter a multi-device estate with mixed permissions, and traverse the full authorized result across page boundaries. |

## Files to Modify
- `internal/component/managed/register.go` -- install daemon handlers, shapes, columns and pipe filters
- `internal/component/plugin/server/managed_apply.go` -- publish committed declaration additions and removals, including re-addition
- `internal/component/plugin/server/managed_registry.go` -- publish accepted connections, disconnects, ACKs and declaration generations
- `internal/component/plugin/server/managed_serve.go` -- start and stop alarm evaluation with the borrowed hub store
- `internal/component/plugin/server/managed_record_store.go` -- coordinate record readiness at startup and alarm flush before shutdown
- `internal/component/config/loader_extract.go` -- parse and validate alarm settings into the hub configuration
- `cmd/ze/hub/managed_server.go` -- pass alarm settings through startup and prepare/apply on reload
- `docs/architecture/fleet-alarms.md` owns the declaration grace period and the hub alarm condition model.
- `internal/component/managed/yang/ze-managed-cmd.yang` -- declare the open-alarm and history command shapes
- `internal/component/plugin/yang/ze-plugin-conf.yang` -- declare offline delay, thresholds and alarm history configuration
- `pkg/zefs/keys.go` -- register raw state keys for occurrence history
- `docs/architecture/fleet-config.md` -- document the live fact sources, occurrence identity, retention and commands

## Files to Create
- `internal/component/plugin/server/managed_alarms.go` -- hub condition registry, serialized transitions and bounded occurrence history
- `internal/component/managed/cmd_alarms.go` -- registered live and history data commands
- `internal/component/plugin/server/managed_alarms_test.go` -- fact transitions, retained timestamps, reconnect, concurrent requests and state-write recovery
- `test/managed/fleet-alarm-lifecycle.ci` -- real command results through disconnect and reconnect
- `test/managed/fleet-alarm-history.ci` -- bounded history and restart recovery

## Design presentation

### What changes

Operators can ask the hub which conditions need attention and when each occurrence opened or cleared. Each page shows the authorized total and counts for the entire filtered result. A continuation reaches every matching occurrence. The alarm's kind, subject and occurrence ID let them follow one incident through repeat reports, loss of contact and restart.

### Before and after

If two interfaces report a component failure, the operator sees two distinct rows for that device. Disconnect does not manufacture two recoveries. The offline condition opens after its configured delay. On reconnect a retained high CPU value cannot open another resource incident; new high samples must qualify, and history preserves the old occurrence.

### Alternatives set aside

Building history by comparing answers to successive CLI calls loses any incident that opens and clears between calls. Keeping an independently editable alarm list could diverge from the registry. Evaluating changes at the producer boundary preserves both the live condition and its bounded history.

### Documents departed from

None for the documented first-release behavior. The existing web limits apply to a returned page; the fleet command needs a way to reach every matching live condition.

### Choices for Thomas

On 2026-11-25 Thomas chose “After the configured offline grace period from declaration”. An unchanged declaration keeps its clock through restart and reload. See `docs/architecture/fleet-alarms.md`, Device offline. The proposed history limits above make truncation visible under churn. The owner has already settled Resource high qualification after reconnect and how a known missing relay grant appears while offline.

## Implementation Steps

1. Wire one producer to the daemon's open-list and history commands. Exercise the transition through a real managed session, including a repeat that changes no occurrence.
2. Add registered condition owners, clock-driven thresholds, history persistence and retained-state reconciliation. Test eviction, failed writes, current-session fencing and restart.
3. Publish permission-scoped command answers and verify all surfaces. Exercise a 2,000-device loss of connectivity. Fleet-ui-8 adds cause resolution; fleet-ui-9 owns the web presentation.
