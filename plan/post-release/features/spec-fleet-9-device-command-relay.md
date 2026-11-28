# Spec: fleet-9 -- Device Command Relay Through the Hub

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | fleet, protocol, cli |
| Priority | P3 |
| Release reason | Adds a capability absent from the supported release; existing supported behavior and its obligations remain required. |
| RFC gaps | - |
| Depends | spec-fleet-1-device-registry (closed 2026-11-25; documented in `docs/architecture/fleet-config.md`, "Device registry"), spec-fleet-3-audit-trail (closed 2026-11-28; documented in `docs/architecture/fleet-config.md` and `docs/guide/audit.md`) |
| Phase | 9/10 of plan/post-release/features/spec-fleet-0-umbrella.md |
| Handoff | - |
| Updated | 2026-11-25 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

**Re-read after compaction:** this file, `spec-fleet-0-umbrella.md`,
`docs/architecture/fleet-config.md`, `internal/component/managed/client.go`
(the device's serial loop for hub requests),
`internal/component/plugin/server/managed_serve.go`,
`internal/component/plugin/dispatch.go` (`CommandDispatcher`).

## Task

Let an operator on the hub run a command on a managed device over the
connection the device already holds to the hub. The device may be behind
address translation, with no address the operator can reach; the managed
channel is the only path, and it already exists.

The managed channel already carries config-fetch, config-ack, config-changed, config-held and ping. This spec adds
the relay: the hub sends a command to a device, the device runs it through its
own command dispatcher under an authority its own configuration grants, and the
answer returns to the hub, streamed when the command streams.

This is what the device tabs and the per-device console of the fleet web
interface run on (`spec-fleet-ui-4-device-tabs-and-console.md`). It is also a
command-line feature in its own right: an operator at the hub's CLI, or any
client of the hub's API, can run a command on a named device.

### What this changes in the fleet umbrella

`spec-fleet-0-umbrella.md` listed "Web per-device command routing" as out of
scope and described it as the hub proxying HTTP to a device. The owner brought
it into scope on 2026-11-18, and the mechanism is different: the hub relays
commands, not HTTP. A command answers data under one authorization and audit
path. A proxied page would carry another daemon's session, routes and markup
through the hub.

### Identity and authorization (owner decision, 2026-11-18)

A device does not know the hub's users, and nothing is granted by implication.

| Rule | Detail |
|------|--------|
| The grant is the device's | The device's own configuration names the authorization profile that relayed commands run under |
| No grant, no relay | A device with no such line refuses every relayed command, and the refusal names the line that adds it |
| The profile bounds everything | A relayed command is authorized on the device exactly as if a user holding that profile had typed it. A read-only profile makes the relay read-only |
| The hub checks first | Before relaying, the hub checks that its own user may relay to that device and may run that command class |
| The actor is carried | The hub user's name travels with the command and is written to the device's audit log beside the profile used |
| The hub is authenticated, not trusted further | A hub that authenticates gains only what the device's profile allows |

The two sides, as an operator sees them:

| On the device | On the hub |
|---------------|------------|
| On the outbound managed client entry use `bus hub client edge-01 relay-profile fleet-reader` (the named profile must exist on that device) | Authorization profiles that allow or deny the relay, per command class (read, operate, console) |
| `show fleet status` names the profile granted, or says that none is | The device page states the profile the device grants |
| The audit log records each relayed command with the hub user's name | The audit log records each relayed command with the device's name |

### The relay on the channel

| Rule | Proposal |
|------|----------|
| Verbs | One verb, hub to device, carrying the command text, the actor's name, a deadline and a request identifier; its answer is streamed. One verb to cancel a running request. Live monitor output uses acknowledged event frames scoped to the same operation |
| Not serial | The device runs relayed commands on a small bounded pool of workers, apart from the loop that handles configuration notices, so a slow command can never delay a configuration push |
| Concurrency | A stated maximum of relayed commands running at once per device; beyond it the device refuses with a message saying so |
| Deadline | Every request carries one. The device stops the command when it passes, and the hub stops waiting |
| Cancellation | Closing the operator's tab, cancelling a task or losing the connection cancels the command on the device |
| Streaming | Commands that stream (`monitor` and long listings) send their output as it is produced, with flow control, inside the frame bound of the channel |
| Bounds | Output is capped at a stated size; a capped answer says it was cut |
| Unknown verb | A device answers an unknown verb with an error instead of dropping it, so a hub never waits out its deadline against a device that does not know the relay |
| Capability | The device states at connect which verbs it supports, so the hub knows before asking whether a device can relay |
| Disconnect | A lost connection ends every relayed command of that device, on both sides |

### What the hub exposes

| Surface | Form |
|---------|------|
| A dispatcher bound to one device | The same type as the existing command dispatcher, so anything written against a dispatcher (the web command line, the workbench page builders that read only through one, the related tools) works against a device unchanged |
| A hub command | Runs one command on one named device and answers its data, so pipes and formats apply on the hub |
| A hub command over several devices | Runs one read-only command on a set and answers one result per device, as a task (`plan/post-release/features/spec-fleet-ui-9-tasks-notifications-and-alarm-pages.md`) |
| Completion and help | Answered by the device, so they match the device's software and build |

For a single device the hub registers `request fleet command device edge-01 run "show system uptime"`. The inner command is one argument, parsed and classified as a daemon command at dispatch. The outer answer keeps the device's shape, so ordinary result pipes on the hub see typed data. Completion and help are explicitly scoped and authorized on the target. Group reads use the admitted set from the registry and a read-only dispatcher for each member. The relay supplies a batch action and per-device results through registration. Fleet UI-9's task supervisor consumes it and owns cancellation/progress and durable task IDs. The single-device relay starts and works without a task package. No task package is imported by the relay.

### What is not relayed

| Not relayed | Reason |
|-------------|--------|
| Configuration editing sessions | The hub is the single writer of a managed device's configuration (`fleet-6`); it edits its own copy and pushes it |
| Commands answered in the caller's own process | They would answer about the hub. The device's daemon form of the command is what is relayed |
| File transfer | Not a command. Software update has its own path |

### Scale

The hub holds each managed connection for its life under a cap of 128
(`managedMaxConns`). An estate larger than that cannot connect, whatever the
interface shows. Lifting that cap belongs to the registry work
(`spec-fleet-1-device-registry`, closed 2026-11-25; documented in `docs/architecture/fleet-config.md`); this spec states the dependency and adds
its own per-device and hub-wide bounds on relayed commands.

### Key design decisions (to confirm at the design gate)

| Decision | Alternative | Reason |
|----------|-------------|--------|
| Relay commands | Proxy HTTP to the device's web server | One authorization and audit path; no trust in another version's markup; no second session; works for the CLI and the API, not only the browser |
| The device's configuration grants the profile | The hub decides alone; a same-named user on each device | The owner's rule: what a device does is what its operator configured. The hub-decides form makes a compromised hub the owner of every device. Same-named users defeat the purpose of fleet management |
| A worker pool on the device | Running relayed commands in the existing serial loop | A slow command would block configuration notices |
| A dispatcher of the existing type | A relay-specific interface | Everything written against a dispatcher works against a device with no change |
| Streamed answers | One answer per command | `monitor` commands and long listings are part of operating a device |
| Unknown verbs are answered | Left dropped | A silent drop costs the hub its whole deadline for every device that does not know the verb |

### Risks to resolve in design

| Risk | Question |
|------|----------|
| A compromised or mistaken hub | What can it do with the profile granted? The answer must be exactly "what a user with that profile can do", proven by test |
| Runaway output | A relayed listing of a full routing table must be bounded and cancellable |
| Abandoned streams | A stream whose operator has gone must end on the device, not only on the hub |
| Version difference | The device and the hub may run different software. The relay carries text and data, and assumes nothing about the device's command set |
| Public contract | `pkg/fleet` is public API. The new verbs are a contract change, made with `docs/architecture/fleet-config.md` |
| Audit completeness | A relayed command must be audited on both sides even when it fails or is refused |

## Existing Mechanisms

| Mechanism | Source | How the relay extends it |
|---|---|---|
| Authenticated named sessions and configuration notifications | `internal/component/plugin/server/managed_serve.go`, `ManagedServer.serve`, `NotifyConfigChanged`; device `internal/component/managed/client.go`, `notificationLoop` | Resolve the connected, authenticated session for one declared device and dispatch on its existing mux. Keep the device's configuration processing independent of the relay work pool. |
| Finite and row-generator answers | `pkg/plugin/rpc/mux.go`, `CallAnswer`, `AnswerWriter`, `Done`; `internal/component/plugin/dispatch.go`, `CommandDispatcher`, `WriteAnswer` | Reuse request/reply framing, typed answer records and existing result/fault terminal frames. A slow or abandoned reader releases its own relay resources. |
| Monitor command path | `internal/component/plugin/server/handler.go`, `GetStreamingHandlerRendered`; `monitor.go`, `MonitorManager`; `cmd/ze/hub/service_ssh.go`, `sshStreamingExecutorFactory` | Follow the existing separately authorized streaming route for monitor commands. Apply the pinned argument policy before the handler, give it the operation context, and frame each emitted text chunk in an acknowledged relay event without buffering the open monitor. The subscribed monitor unregisters on cancellation. |
| Managed wire and session metadata | `pkg/fleet/envelope.go`, `ConfigFetchRequest`; `internal/component/plugin/server/managed_registry.go`, `ManagedRegistrySnapshot`; `internal/component/plugin/server/managed_alarms_kinds.go`, `RegisterAlarmKind` | Add optional capability fields to fetch, retain last-known relay capability and grant state across disconnect so `relay-not-granted` stays visible while offline, and expose the effective grant to authorized callers only. Reconnect replaces previous metadata. |
| Managed client config, live owner, and startup doctor | `internal/component/config/loader_extract.go`, `ExtractHubConfig` and `extractHubClientConfig`; `internal/component/plugin/types.go`, `HubClientConfig`; `internal/component/managed/client.go`, `DeclaredEndpoint`, `adoptEndpoint`; `cmd/ze/hub/managed_owner.go`, `managedClientOwner.target`; `internal/component/managed/cmd_fleet.go`, `FleetTarget`/`ClientState`; `internal/component/managed/doctor.go`, `register_doctor_codes.go` | Carry the relay grant from local configuration to the running client and its published status. Refuse an unknown or reserved `relay-profile` at `ze config validate`, commit, and reload; at `ze start`, keep the daemon and managed config channel running with relay refused, log under `doctor-managed-relay-profile-invalid`, and report the finding and fix in `ze doctor`. After successful local or hub-pushed reload, publish the new grant and cancel revoked work through the owner without waiting for reconnect. An unsuccessful reload retains the prior grant. |
| Caller authorization and profile binding | `internal/component/plugin/dispatch.go`, `CallerIdentity`; `internal/component/authz/authz.go`, `AuthorizeWithProfiles`; `register.go`, `StoreAuthorizer.BindProfiles` | Evaluate the hub caller's allowed target/class and the device's exact locally configured profile. Verify an available, non-nil policy store and the configured profile before calling BindProfiles. Device dispatch uses an ordinary authenticated relay identity and pinned local authorizer. Carry the original hub username separately solely for correlation/audit. |
| Structured audit | `internal/core/audit/audit.go`, `Recorder` | Record the normalized command path, correlation ID, local profile, actor, device, outcome and a bounded refusal reason. The hub journal producer is a Fleet-3 prerequisite. Store no credentials, free-form argument text or returned data. |
| Fleet device views and console | `internal/component/web/fleetui/source_views.go`, `DeviceViewSource` | Fleet UI-4 adapts its existing page and console consumers to the same typed, authorized dispatcher. |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|---|---|---|---|---|---|
| `ai/doors/system.md` | Ze is a platform, and other people's code runs against it | the door every change reads | "`pkg/plugin/sdk`, `pkg/plugin/rpc`, `pkg/ze`, `pkg/zefs` and `pkg/fleet` are imported by code outside this repository." | agrees | The new envelope is documented with the managed protocol. Existing peers keep their configuration path when the new feature is absent. |
| `docs/architecture/fleet-config.md` | Key Rules | the // Design: page of pkg/fleet/envelope.go | "The client name in `client edge-01 { }` IS the client's identity" | agrees | Per-client authentication binds the device name to this session; optional metadata and the new verb leave ordinary configuration and holds intact. |
| `docs/architecture/fleet-config.md` | Design decisions | the // Design: page of pkg/fleet/envelope.go | "Thomas chose “Allow first provisioning and later edits from the device’s web Config tab”. An authorized operator can commit the first or a later version of a declared device’s configuration while it is offline, through the post-release fleet web surface." | agrees | Offline provisioning and edits write the declared device’s desired configuration through UI-6. A device command requires a live relay session, and the relay cannot open a configuration editing session. |
| `docs/architecture/api/commands.md` | Dispatch keys are YANG paths | anchored to pkg/fleet/envelope.go | "The daemon registers each built-in handler under its YANG command path" | agrees | The registered subject returns typed data. Filtering and formatting run on the result. |
| `ai/doors/hub-engine.md` | Boundaries | the door covering pkg/fleet/envelope.go | "SIGHUP and a managed push both reach `doReload` (`cmd/ze/hub/main_reload.go`). A new trigger joins it rather than building a second path." | agrees | The managed client and hub own session cancellation and worker shutdown. The existing configuration reload path stays authoritative. |
| `docs/architecture/api/architecture.md` | REST and gRPC authentication flow | anchored to internal/component/plugin/server/managed_serve.go | "Only surviving stored users retain their recovery profile." | agrees | Authorization remains bound to the actual hub user. Device names and asserted actor strings cannot acquire a locally reserved or recovery identity. |
| `docs/architecture/core-design.md` | Command engine and composition roots | anchored to internal/component/plugin/server/managed_serve.go | "Composition, not dispatch, separates the programs." | agrees | Fleet commands enter the normal product registration; the existing command engine serves them. |
| `docs/architecture/hub-architecture.md` | Startup Sequence | anchored to internal/component/plugin/server/managed_serve.go | "Cancellation still ends its manager lifetime." | agrees | Session workers stop and join before the managed owner releases the daemon store. |
| `docs/architecture/pki/tls-listeners.md` | Decision: a named certificate fails closed | anchored to internal/component/plugin/server/managed_serve.go | "A configured name that does not resolve returns an error and no material." | agrees | An unresolved configured certificate prevents the handshake. Only an authenticated live session carries this operation. |
| `ai/doors/plugin-infra.md` | Traps | the door covering internal/component/plugin/server/managed_serve.go | "so a payload must be compact `json.Marshal` output." | agrees | Wire values use compact JSON, bounded before decoding, carried by the authenticated named session. |
| `docs/architecture/config/syntax.md` | Overview | anchored to internal/component/plugin/yang/ze-plugin-conf.yang | "The parser is YANG-driven: each config node's type (leaf, leaf-list, container, list) determines how it is parsed." | agrees | The new leaf is declared in YANG and validated by the existing parser. |
| `ai/rules/repo-maintenance.md` | Directives | the // Design: page of internal/component/managed/register.go | "A change that adds or changes a surface future agents use, verify, document or avoid MUST update its discovery path in the same work" | agrees | The managed guide, command documentation and source anchors follow the new registered surface. |
| `docs/architecture/doctor-and-health-checks.md` | The runtime phase: checks only the daemon runs | anchored to internal/component/managed/register.go | "A check over such state registers in the `runtime` phase, through the same `diagnostic.RegisterDoctorCheck` every other check uses" | agrees | The daemon supplies live connectivity, profile-grant and outstanding work status. An offline caller never invents live session state. |
| `ai/doors/le-tooling.md` | Where it sits | the door covering test/managed/fleet-command-relay.ci | "Functional `.ci` files live under `test/<suite>/` and run through `./le test <suite>`" | agrees | A test/managed functional test exercises a real managed connection through the normal launcher. |
| `docs/architecture/fleet-config.md` | Design decisions | the // Design: page of pkg/fleet/envelope.go | "A reload that changes the certificate a serving hub's managed listener presents installs it on the running listener at that reload: the next handshake presents the certificate the configuration names, the listener keeps its socket and no session is closed, as the web and looking glass listeners do. The certificate stays in the configuration's `pki` block. In Thomas's words (voice-transcribed): "Keep the certificate in the configuration." and "In that case on commit we can perform the certificate change on the listener."" | agrees | Updating listener credentials changes subsequent handshakes, and keeps existing authenticated sessions and their outstanding work. |
| `docs/architecture/api/commands.md` | Design decisions | anchored to pkg/fleet/envelope.go | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | The hub relays the admitted command. Hub formatting pipes act on its returned answer. A pipe cannot expand the target or the grant. |
| `docs/architecture/api/commands.md` | Design decisions | anchored to pkg/fleet/envelope.go | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | The hub relays the admitted command. Hub formatting pipes act on its returned answer. A pipe cannot expand the target or the grant. |
| `docs/architecture/api/commands.md` | Design decisions | anchored to pkg/fleet/envelope.go | "A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does" | agrees | The hub relays the admitted command. Hub formatting pipes act on its returned answer. A pipe cannot expand the target or the grant. |
| `docs/architecture/api/architecture.md` | Design decisions | anchored to internal/component/plugin/server/managed_serve.go | "When `\| implied` modifies an existing config line (such as inserting automatic members into a `filters [ ... ];` list), it keeps the line intact and places an inline `# implied: ...` comment on the same line after the statement naming what was added and why, rather than placing a comment only above the line or splitting the list across multiple lines. In Thomas's words (2026-11-23): "No, it should be on the same line and say what was added"." | agrees | Operator-visible implied configuration continues to be rendered by the existing config display. The fleet code does not rewrite filters or their inline comments. |
| `docs/architecture/core-design.md` | Design decisions | anchored to internal/component/plugin/server/managed_serve.go | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | The existing dataplane validation still judges an applied configuration at validate, startup and reload. This feature does not select a forwarding backend. |
| `docs/architecture/core-design.md` | Design decisions | anchored to internal/component/plugin/server/managed_serve.go | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | The existing dataplane validation still judges an applied configuration at validate, startup and reload. This feature does not select a forwarding backend. |
| `docs/architecture/core-design.md` | Design decisions | anchored to internal/component/plugin/server/managed_serve.go | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | The existing dataplane validation still judges an applied configuration at validate, startup and reload. This feature does not select a forwarding backend. |
| `docs/architecture/core-design.md` | Design decisions | anchored to internal/component/plugin/server/managed_serve.go | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | The existing dataplane validation still judges an applied configuration at validate, startup and reload. This feature does not select a forwarding backend. |
| `docs/architecture/core-design.md` | Design decisions | anchored to internal/component/plugin/server/managed_serve.go | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | The fleet operation neither interprets BGP policies nor writes a route. The owning BGP engine retains this export/import behavior. |
| `docs/architecture/core-design.md` | Design decisions | anchored to internal/component/plugin/server/managed_serve.go | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | The fleet operation neither interprets BGP policies nor writes a route. The owning BGP engine retains this export/import behavior. |
| `docs/architecture/core-design.md` | Design decisions | anchored to internal/component/plugin/server/managed_serve.go | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | The fleet operation neither interprets BGP policies nor writes a route. The owning BGP engine retains this export/import behavior. |
| `docs/architecture/core-design.md` | Design decisions | anchored to internal/component/plugin/server/managed_serve.go | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | The fleet operation neither interprets BGP policies nor writes a route. The owning BGP engine retains this export/import behavior. |
| `docs/architecture/core-design.md` | Design decisions | anchored to internal/component/plugin/server/managed_serve.go | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | The fleet operation neither interprets BGP policies nor writes a route. The owning BGP engine retains this export/import behavior. |
| `docs/architecture/core-design.md` | Design decisions | anchored to internal/component/plugin/server/managed_serve.go | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | The fleet operation neither interprets BGP policies nor writes a route. The owning BGP engine retains this export/import behavior. |
| `docs/architecture/core-design.md` | Design decisions | anchored to internal/component/plugin/server/managed_serve.go | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | The fleet operation neither interprets BGP policies nor writes a route. The owning BGP engine retains this export/import behavior. |
| `docs/architecture/pki/tls-listeners.md` | Design decisions | anchored to internal/component/plugin/server/managed_serve.go | "The looking glass may serve plain HTTP, because what it publishes is public, and one looking glass serves HTTP and HTTPS at the same time. Thomas's words, as voice-transcribed: "We should allow the looking glass to be HTTP. The information is public." and "We should be able to serve on both HTTP and HTTPS, not either." Asked where the choice lives and what happens to `ze.looking-glass.tls`, he answered "1a 2a": the choice is the `tls` leaf of each `server` entry, default true, and the environment variable is removed. The no-store rule applies to each entry: an entry that left `tls` unset serves HTTP with a warning on a daemon with no `database/` store and no named certificate, an entry that says `tls true` there is refused at start and at reload, and a reload does what a restart with the same config does" | agrees | The feature borrows the existing managed session; its certificate selection remains with managedCertificate. Looking glass keeps its own listener choices. |
| `docs/architecture/pki/tls-listeners.md` | Design decisions | anchored to internal/component/plugin/server/managed_serve.go | "The web and looking glass use the shared `listenerTLSMaterial` precedence. The approved Fleet reload design shares `managedCertificate` with startup because the daemon authority issues its unnamed leaf." | agrees | The feature borrows the existing managed session; its certificate selection remains with managedCertificate. Looking glass keeps its own listener choices. |
| `ai/doors/config-yang.md` | Traps | the door covering internal/component/config/loader_extract.go | "`ze config validate` never calls a live `OnConfigVerify`." | agrees | Extraction and semantic validation run against the configuration tree without a live daemon. |
| `docs/architecture/api/process-protocol.md` | Wire Format | the // Design: page of internal/component/plugin/types.go | "Every message is a single newline-terminated line:" | agrees | Use the existing framed transport and answer protocol. The operation bounds applied here fit within its message ceiling. |
| `docs/architecture/api/ipc_protocol.md` | Message Framing | anchored to internal/component/plugin/types.go | "All plugin RPC messages are UTF-8, newline-delimited lines:" | agrees | The new verb and answers use the existing correlated newline-delimited frames. |
| `docs/architecture/api/json-format.md` | Field naming | anchored to internal/component/plugin/types.go | "JSON keys are lowercase kebab-case, never camelCase and never snake_case." | agrees | Use lowercase kebab-case keys for new JSON fields in the protocol and command answer. |
| `docs/architecture/cli/command-verbs.md` | Narrowing and shaping: the pipe layer | anchored to internal/component/plugin/types.go | "An owned filter takes the name of the field it selects on." | agrees | The outer command selects its subject. Filters apply to already authorized data. |
| `docs/architecture/cli/error-surface.md` | The operator error | anchored to internal/component/plugin/types.go | "A refusal an operator reads is an `opererr.Error`" | agrees | Return structured operator errors giving the cause and the applicable fix. |
| `docs/architecture/config/transaction-protocol.md` | 1. Design Principles | anchored to internal/component/plugin/types.go | "The hub promotes a candidate after runtime reload succeeds." | agrees | Publish the effective setting only on acceptance. A rejected reload retains the live setting and ongoing state. |
| `docs/architecture/encoding-context.md` | TL;DR (Read This First) | anchored to internal/component/plugin/types.go | "" | silent | The page defines BGP negotiated data-plane encoding and does not specify managed RPC or fleet configuration. |
| `docs/architecture/plugin-manager-wiring.md` | Stage 1 Registration: Config Transaction Fields | anchored to internal/component/plugin/types.go | "" | silent | These startup fields describe plugin transports and budgets. The managed service adds no plugin Stage 1 field. |
| `docs/architecture/web-interface.md` | Component Structure | anchored to internal/component/plugin/types.go | "Every page, fragment and editor is a templ component." | agrees | Fleet UI children own the templ pages. This backend supplies the same authorized answer to the UI and other command surfaces. |
| `docs/architecture/aaa-tacacs.md` | Authorization arguments are enforced | anchored to cmd/ze/hub/main_reload.go | "Command authorization uses `service=shell`, `cmd` and ordered `cmd-arg` values." | agrees | Apply the hub caller’s full command and argument policy and separately evaluate the target command, pipes and arguments under the device grant. |
| `docs/architecture/behavior/signals.md` | Ze Signal Mapping | anchored to cmd/ze/hub/main_reload.go | "`handleSIGHUPReload` (`cmd/ze/hub/main_reload.go`), then `reactor.SignalHandler.OnReload`" | agrees | SIGHUP goes through the normal successful reload before the managed client owner publishes its grant. |
| `docs/architecture/plugin/component-boundaries.md` | What this buys | anchored to cmd/ze/hub/main_reload.go | "No plugin package imports another." | agrees | The composition root injects dispatch and streaming callbacks. The plugin server never imports a subsystem to run a relayed command. |
| `docs/architecture/appliance/remote-operations.md` | Decisions | anchored to cmd/ze/hub/main.go | "" | silent | This concerns the appliance SSH/offline operations and transfer, which the managed command channel does not provide. |
| `docs/architecture/cli/command-completion.md` | The producers | anchored to cmd/ze/hub/main.go | "Every producer asks one predicate what its surface owes a command:" | agrees | Help and completion use the device producer and the same scoped command capability and permission as dispatch. |
| `docs/architecture/config/yang-config-design.md` | API Schemas: `ze-bgp-api.yang` and `ze-*-cmd.yang` | anchored to cmd/ze/hub/main.go | "The `-cmd.yang` modules define the CLI navigation hierarchy using" | agrees | Declare the public command subject and its arguments in the owning module. |
| `docs/architecture/diagnostics/crash-capture.md` | Recovered panics | anchored to cmd/ze/hub/main.go | "below that boundary ends only the unit the input belongs to" | agrees | An uncaught handler fault retires only its bounded execution/session. Other clients and configuration notifications keep operating. |
| `docs/architecture/pki/pki-store.md` | Traps this code exists to avoid | anchored to cmd/ze/hub/main.go | "A YANG list is built with the list-entry API, not with nested containers." | agrees | The new grant leaf uses the existing managed client entry; model validation builds the list entry through its typed API. |
| `docs/architecture/storage-backends.md` | Opening and ownership | anchored to cmd/ze/hub/main.go | "A second writer receives `ErrBusy`" | agrees | Read the grant and journal through the running daemon’s state. No second store opener is introduced. |
| `docs/architecture/system-architecture.md` | Composition roots | anchored to cmd/ze/hub/main.go | "Both personalities use the command registry and pipe engine." | agrees | The relay composes through the shipped hub command registry, independent of development-only commands. |
| `docs/architecture/testing/ci-format.md` | Syntax Overview | anchored to cmd/ze/hub/main.go | "All lines use key=value format" | agrees | The real-connection test uses the managed harness syntax to exercise denial, live streaming, cancellation and continued config delivery. |
| `docs/architecture/fleet-alarms.md` | Alarm kinds | anchored to internal/component/plugin/server/managed_serve.go | "An alarm is a derived condition true right now; nothing is acknowledged or silenced." | agrees | Fleet-9 supplies relay capability and effective grant state on the authenticated session so `relay-not-granted` joins the existing hub alarm registry without altering alarm derivation. |
| `docs/architecture/diagnostics/production-diagnostics.md` | CLI first, MCP for free | anchored to cmd/ze/hub/main.go | "Every diagnostic is a YANG RPC registered as a CLI command." | agrees | Relayed commands execute the target device's registered YANG RPCs through its normal command dispatcher; no separate diagnostic transport is added. |
| `docs/architecture/fleet-alarms.md` | Design decisions | decision of 2026-11-25 on `fleet-alarms.md` | "For a declared device that has never connected, Thomas chose “After the configured offline grace period from declaration”." | agrees | Relay capability and grant advertisements arrive only after an authenticated connection; offline grace period derivation for never-connected devices is unchanged. |
| `docs/architecture/pki/tls-listeners.md` | Design decisions | decision of 2026-11-26 on `tls-listeners.md` | "A certificate that cannot be presented never stops ze from starting. A web server whose certificate name does not resolve is not started. The looking glass is judged one `server` entry at a time: an entry that says `tls true` with no certificate to present stays down, every entry that asks for TLS stays down when the named certificate does not resolve, and the other entries serve. Each is reported with its fix by an error line in the log under a doctor code and by `ze doctor`. A reload whose config cannot be served stays refused. This supersedes the start half of the row of 2026-11-23 above, which says a `tls true` entry "is refused at start": at start that entry stays down and the others serve, and at reload the config is still refused. Thomas's words, as voice-transcribed: "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; "reload stay refused"; "Should only that endpoint stay down while the others serve: yes"" | agrees | Relay borrows the existing authenticated managed TLS listener; startup and reload certificate handling for web, looking glass, and managed listeners remain unchanged. |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-23 on `process-protocol.md` | "Every engine operation names the message it carries or `none` with a reason, and a plugin identity is authorized by a profile compiled from its filter" | agrees | Relayed commands run on the target device under the locally configured operator authorization profile, leaving plugin-identity compiled bus filters unchanged. |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-25 on `process-protocol.md` | "Engine-to-plugin event delivery never drops events on queue overflow; endpoints expect every subscribed network event losslessly" | agrees | Relay monitor streaming uses acknowledged per-operation `relay-event` RPC frames on the managed channel and never alters lossless engine-to-plugin bus delivery. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 on `api/architecture.md` | "Everything on the bus is a participant named by its configuration path. A participant registers the messages it sends and receives, events and commands alike, and with nothing written that registration is what runs" | agrees | Relay adds no bus participant semantics and leaves registered bus message routing untouched. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 on `api/architecture.md` | "A `send > to` list written on a participant replaces the destinations its messages have by registration; it never adds to them. Present and empty means nobody" | agrees | Relay does not modify bus participant send lists or destination resolution. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 on `api/architecture.md` | "A `receive > from` list limits who may address a participant. Absent means everybody; present and empty means nobody" | agrees | Relay does not modify bus participant receive lists. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 on `api/architecture.md` | "A send and a receive allow-list of message names (`send > allow`, `receive > allow`) exist on every participant: absent means everything declared, empty means nothing. The bus enforces them when it receives the message" | agrees | Relay does not modify bus message allow-lists or enforcement. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 on `api/architecture.md` | "What is written on the participant and what is written in the `bus` section compile into one filter per participant. A disagreement between the two places, or between an allow-list and a destination on one participant, is refused at commit naming both lines" | agrees | Relay adds only `relay-profile` on `bus hub client`; bus participant filter compilation and conflict refusal are unchanged. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 on `api/architecture.md` | "The bus does not arbitrate between participants: no exclusive-role rule, no commit refusal and no silent winner. A plugin that must not duplicate another limits itself" | agrees | Relay does not introduce bus participant arbitration. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 on `api/architecture.md` | "One display shows every registered default beside what the operator wrote, the defaults a written list replaced, and the line that produced each effective entry" | agrees | Relay leaves bus default display unchanged. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 on `core-design.md` | "Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code. Examples are a web certificate name that does not resolve, a looking-glass `server` entry with no certificate to present, and a looking-glass address list that does not parse. Where a service has several endpoints, only the endpoint that cannot be served stays down and the others serve. A reload whose config cannot be served stays refused as a whole, and the running services keep what they serve. In Thomas's words (voice-transcribed): "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; asked about a reload, "reload stay refused"; asked "Should only that endpoint stay down while the others serve", "yes". The reason is that a daemon that does not start brings up no networking, so the operator cannot reach the box to correct it." | agrees | A missing or unresolvable `relay-profile` at startup refuses relayed commands cleanly without stopping `ze start` or the managed config channel; invalid `relay-profile` edits at commit/reload remain refused. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 on `core-design.md` | "The rule of 2026-11-23 above, that a configuration which holds a feature that programs only the Linux kernel while the interfaces run on VPP "is refused at daemon start", is superseded for the start: ze starts, that feature stays off, and `ze doctor` reports the mismatch with its fix. A commit, a reload and `ze config validate` still refuse such a configuration, by the same verdict as before. Thomas was asked whether the rule that ze always starts replaces the refusal at start, "(a) yes, ze starts, that feature stays off and ze doctor reports the mismatch, while commit and reload still refuse it; (b) no", and answered "2a" (voice-transcribed). The reason is the one of the row above: a daemon that does not start brings up no networking, so the operator cannot reach the box to correct the mismatch." | agrees | Relay selects no forwarding backend; dataplane startup degradation and commit/reload refusal remain unchanged. |

## Data Flow

The device advertises the versions of the relay verbs it supports and the currently granted profile as optional capabilities on its initial authenticated config-fetch. Absence disables the relay immediately. The hub retains the last-known relay capability and grant state across disconnect and drives the registered `relay-not-granted` informational alarm (`RegisterAlarmKind`, `AlarmEvalDevice`, no declared cause): when a relay-capable device explicitly advertises no grant, `relay-not-granted` opens and stays visible while offline; advertising a grant or lacking relay capability clears it, and an unknown pre-contact state opens no alarm. `relay-profile` is declared as a string leaf on `bus hub client <name>` (never a YANG `leafref` that would block `LoadConfigForStart` for the whole managed client). Semantic validation at `ze config validate`, commit, and reload refuses any non-empty `relay-profile` that does not name a defined non-reserved profile in `system authorization`. At `ze start`, if the persisted config carries an unresolvable `relay-profile` (or `system authorization` is absent), the daemon and managed config channel still start, relay stays disabled/refused, the device logs an error under `doctor-managed-relay-profile-invalid`, and `ze doctor` reports the finding with its fix (`set bus hub client <name> relay-profile <profile>`). Keep the relay-profile outside the transport endpoint comparison in adoptEndpoint. A grant-only reload updates the running session in place. Profile removal takes effect locally at reload, cancels active requests under that grant, and updates the hub on the next authenticated exchange. A cached announcement never authorizes execution.

The hub checks its caller, target and operation class before reading the session or sending any request. An untrusted command string can never choose its own read-only class. Group reads require the device to verify the resolved command is read-only before invoking it, including any command-specific pipe. The device resolves its grant from its live configuration and applies its own command and argument authorizer, with identity/profile pinned for the accepted operation. Never use a protocol-supplied username as the device dispatcher identity. Keep that name as audit attribution only. A missing grant, an unknown local profile, a reserved profile or an unavailable policy refuses the request. Reject editor sessions, daemon-local-only operations, file transfer and local file sinks on the device. Help/completion follow the same authorization. A forbidden target reveals no presence or profile.

Each relayed operation has a nonzero identifier unique within the authenticated session; the hub allocates it once and includes it in both the execution and cancellation payloads. Mux framing IDs still correlate the individual RPC calls. Repeated or stale cancellation is idempotent, and a reconnect invalidates IDs from the old session. A live duplicate execution ID is refused. The device advertises its capability on every new connection. Already running operations are never retried on a replacement connection.

Bound one session to four concurrent requests, and the hub to 64 admitted relay operations in total, with immediate busy refusal at either limit. No unbounded admission queue. A request carries the remaining duration and a hard deadline. The device uses the lesser of the supplied duration and its own remaining maximum (five minutes); the hub tracks its own deadline independently. Responses use the mux's bounded streamed records, with at most 4 MiB (`fleet.RelayOutputBytesMax`) of encoded payload per operation across frames, and at most 1 MiB (`fleet.RelayFrameBytesMax`) per individual wire frame so every line stays strictly below the hub listener's `managedReadMax` (~1.40 MiB) and can never trigger `ManagedRejectOversize` or close the managed session. On overflow of either the per-operation or single-frame cap, preserve complete delivered rows and terminate with an explicit truncation fault. Context cancellation, deadline expiry, grant withdrawal, connection close or a slow reader cancels the worker. Stop and join workers on session teardown. The hub never holds its session lock during dispatch. Configuration pings and notifications retain admission even when all relay slots are occupied. A finite single-document answer is checked against `fleet.RelayFrameBytesMax` (1 MiB) before sending; if it exceeds the single-frame ceiling, the device emits an explicit truncation fault instead of writing an oversize line.

Finite commands use the existing command answer transport, including its 256-record threshold for a row generator. A live monitor instead uses a bounded live operation on the same multiplexed connection. Opening a monitor returns an immediate operation acknowledgement. The device's streaming handler writes to a context-bound adapter which sends `relay-event` to the hub with the session operation ID, monotonic ordinal and a bounded text chunk. The hub acknowledges once its bounded consumer has taken the chunk. Only one event per operation may await an acknowledgement. A final event carries completion, truncation, fault or cancellation. Transfer whole UTF-8 text at rune boundaries, with at most 16 KiB of decoded bytes per chunk and 4 MiB per operation. Reject unknown IDs and duplicate or out-of-order chunks, without admitting work on another session. A stalled consumer cancels the operation. Keep finite command rows typed and use this byte stream only for handlers whose existing output is text. Register the hub streaming route beside the finite dispatcher command: streaming-capable CLI, SSH, web, REST and gRPC routes consume monitor events immediately; a one-shot API/MCP invocation of an unbounded monitor receives a specific streaming-required result before remote dispatch. A bounded finite read remains available on every command surface.

### Changed-Wire Census

**Search evidence:** Searched `pkg/`, `cmd/`, `internal/`, `test/` (Go, shell, Python and functional suite bodies) for `VerbConfigFetch`, `ConfigFetchRequest`, `config-fetch`, `relay-command`, `relay-cancel`, `relay-event` and `fleet-audit-receipts`. The current positive control is `managed.fetchConfigWithin` sending to `ManagedServer.handleRequest`. The new relay and receipt verbs have no existing production speaker. The scoped shell, Python and functional suite search found no writers or readers of those verbs.

| Wire | Role | Carrier | Source evidence | Disposition |
|---|---|---|---|---|
| `config-fetch` optional relay advertisement | writer | `internal/component/managed/client.go`, `fetchConfigWithin` and `FetchConfig`; bootstrap caller `cmd/ze/ze_core_start.go`, `fetchInitialConfig` | `pkg/fleet/envelope.go`, `ConfigFetchRequest` | Steady connection advertises versions and effective profile. Bootstrap advertises no live relay. |
| `config-fetch` optional relay advertisement | reader | `internal/component/plugin/server/managed_serve.go`, `ManagedServer.handleRequest` | Decodes `fleet.ConfigFetchRequest` on the authenticated session | Absent advertisement leaves all config functions available. |
| `relay-command` | writer | New hub `managed_relay.go`, built on `pkg/plugin/rpc/mux.go`, `CallAnswer` | The hub already writes requests to a selected session in `pushConfigChanged` | Send operation ID, actor, class limit, command, duration and deadline. |
| `relay-command` | reader | `internal/component/managed/client.go`, `notificationLoop`, `handleHubRequest` | The device currently dispatches config-changed and ping here | Bound admission and submit an authorized device dispatch without blocking the notice reader. |
| Finite relay answer frames | writer | Device `internal/component/managed/relay.go`, using `plugin.WriteAnswer` and `MuxConn.AnswerWriter` | `internal/component/plugin/dispatch.go`, `WriteAnswer` | Preserve typed items, faults and terminal status under the operation byte bound. |
| Finite relay answer frames | reader | New hub `managed_relay.go` and `cmd_relay.go`, using `MuxConn.CallAnswer` | `pkg/plugin/rpc/mux.go`, `answerRecords` | Preserve the bounded stream and cancel on consumer termination. |
| Live monitor opening acknowledgement | writer | The device’s managed relay handler returns the accepted operation ID with the existing `MuxConn.SendResult` | `internal/component/managed/client.go`, `handleHubRequest` | Admit before starting the first live event; return a refusal immediately if admission fails. |
| Live monitor opening acknowledgement | reader | Hub relay adapter consumes the `CallRPC` result | `pkg/plugin/rpc/mux.go`, `CallRPC` | Match the accepted operation to the authorized waiting caller. |
| `relay-event` data and terminal | writer | New device relay writer wraps the registered `StreamingHandler` output, using `MuxConn.CallRPC` | `internal/component/plugin/server/handler.go`, `StreamingHandler` | One bounded UTF-8 chunk or terminal event per acknowledged request. |
| `relay-event` data and terminal | reader | Hub `ManagedServer.handleRequest` and relay consumer | `internal/component/plugin/server/managed_serve.go` | Verify session, operation and ordinal. Pass text to this consumer only, apply backpressure and return its acknowledgement. |
| `relay-event` acknowledgement | writer | Hub relay consumer calls the managed session's `MuxConn.SendResult` | `pkg/plugin/rpc/mux.go` | Acknowledge the event accepted by this consumer or return a refusal. |
| `relay-event` acknowledgement | reader | Device relay writer checks its `CallRPC` result | `pkg/plugin/rpc/mux.go` | Send the next event only after acknowledgement, or cancel on error or deadline. |
| `relay-cancel` request | writer | Hub caller cancellation callback in `managed_relay.go` | `pkg/plugin/rpc/mux.go`, `CallRPC` | Name the stable execution ID on the same session. |
| `relay-cancel` request | reader | Device `handleHubRequest` and execution registry in `relay.go` | `internal/component/managed/client.go` | Cancel only that session's execution. Return an idempotent ack. |
| `relay-cancel` acknowledgement | writer | Device `handleHubRequest` with `MuxConn.SendOK` | The same reader already acknowledges ping | Return completion of the cancellation request. |
| `relay-cancel` acknowledgement | reader | Hub `managed_relay.go` cancellation callback and mux | Existing `MuxConn.CallRPC` routes ack to its own caller | Consume the ack. An absent connection cancels locally on both sides. |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A device granting a read-only profile; a hub user allowed to relay | A read command run from the hub answers the device's data, identical to the same command on the device | Formatting pipes run on the hub, because the answer is data |
| AC-2 | The same device; a command the profile forbids | The device refuses it as it would refuse that profile locally, and the refusal reaches the operator word for word | None |
| AC-3 | A device with no grant | Every relayed command is refused with the line that grants it | The configuration channel keeps working, because it does not use the relay |
| AC-4 | A hub user not allowed to relay | The hub refuses before anything reaches the device | None |
| AC-5 | A slow relayed command and a configuration push at the same time | The push is delivered and acknowledged without waiting for the command | None |
| AC-6 | A streaming command, then the operator leaves | The command stops on the device | None |
| AC-7 | A command past its deadline, or over the output cap | It is stopped, and the answer says which bound it hit | Output already received is kept and marked as cut |
| AC-8 | Requests refused before dispatch on the hub, and allowed or refused requests received by the device | The hub records every attempt, including refusals. Every attempt received on the device is also audited there with hub actor and local profile. Hub and device records share the request identifier; neither records sensitive arguments. | A pre-dispatch refusal reaches the hub log alone because no request was sent to the device. |
| AC-9 | A device whose software does not know the relay | The hub learns it at connect and answers at once that the device cannot relay | None |
| AC-10 | A standalone device | Its own dispatcher answers a local read. A hub relay request receives the unavailable-device result, and startup creates zero relay workers. | Ordinary local commands remain available because their dispatcher is independent of a managed session. |
| AC-11 | Four device slots or 64 total hub slots are occupied, followed by a fifth device request, a new config and cancellation | The excess relay request is refused; config is applied without waiting. Cancellation frees the correct slot; reconnect and reused transport numbers cannot stop a newer operation. | Other sessions remain usable. |
| AC-12 | Authorized help, completion and group read, mixed with a target that rejects the group request | The target's authorized grammar drives help/completion. Group work yields one result per target and read-only verification on the device rejects a mutating operation. | An individual offline or refused target does not discard other device results. |
| AC-13 | A relay-capable device advertises an explicit absent grant, disconnects, then reconnects with a grant | Hub alarm `relay-not-granted` (registered via `RegisterAlarmKind`, informational, no cause) opens on explicit absent grant, stays visible while the device is offline, clears once a grant is advertised, and never opens for a non-relay device or before first capability advertisement. | Other device alarms remain governed by their own registered kinds. |
| AC-14 | Persisted config names an unresolvable `relay-profile` at `ze start`, versus an invalid `relay-profile` edited at commit/reload/`ze config validate` | `ze start` succeeds and keeps the managed config channel running while refusing relayed commands, logging an error under `doctor-managed-relay-profile-invalid` and reporting the finding and fix in `ze doctor`. Commit, reload, and `ze config validate` refuse the invalid `relay-profile` edit without changing the live grant. | Existing valid managed config sync stays available at start. |

## Wiring Test

| Entry point | Evidence |
|---|---|
| Hub dispatcher to a real managed client's dispatcher and back | `test/managed/fleet-command-relay.ci` covers a read of device state, a hub denial, a device denial, loss of connection during output, a profile reload, and a simultaneous accepted configuration update. The ordinary web command line, REST, gRPC, SSH and MCP receive the same authorized finite answer. Live consumers see a single monitor event without waiting for another; one-shot consumers receive the streaming-required outcome. |

## TDD Test Plan

| Test | Proves |
|---|---|
| `TestRelayGrantAndHubScope` | Named grant, unknown grant, no grant, local-only, editor and read-only refusal; supplied actor ignored for privilege; startup degradation with `doctor-managed-relay-profile-invalid` vs commit/reload/validate refusal. |
| `TestRelayAdmissionAndCancel` | Session generation, concurrent and total bounds, expiry, cancel/completion race, disconnect and no surviving worker. |
| `TestRelayAnswerBounds` | Streamed 4 MiB operation limit and 1 MiB single-frame (`RelayFrameBytesMax` < `managedReadMax`) document cap, complete records before truncation, slow reader and typed failure propagation. |
| `TestRelayAuditAndCapabilities` | One local outcome per reached boundary, redaction, unknown verb and absent capabilities; missing capability never delays a hub query; `relay-not-granted` alarm opens on absent grant, survives disconnect, clears on grant. |
| `TestRelayGroupResult` | One authorized result per device, refused individual targets, read-only commands, task consumer optional. |

## Files to Modify
- `internal/component/config/loader_extract.go` — extract the declared grant on the outbound managed entry
- `internal/component/plugin/types.go` — carry it in HubClientConfig
- `cmd/ze/hub/main.go` — inject the normal dispatcher and authorized streaming callback after composition
- `cmd/ze/hub/managed.go` — wire the client execution callbacks and relay lifecycle
- `cmd/ze/hub/managed_server.go` — register the hub stream and its authenticated session resolver
- `cmd/ze/hub/managed_owner.go` — publish the live grant and cancel revoked workers
- `cmd/ze/hub/main_reload.go` — give the owner the successful local or managed reload result
- `internal/component/plugin/server/managed_registry.go` — expose and retain last-known relay capability and grant metadata across disconnect
- `internal/component/plugin/server/managed_alarms.go` — pass retained relay grant state into `AlarmEvalDevice` and refresh `relay-not-granted` on capability/grant changes
- `pkg/fleet/envelope.go` — capability advertisement, relay frame/output bounds (`RelayFrameBytesMax`, `RelayOutputBytesMax`), and relay execution/cancel/event wire values
- `internal/component/plugin/server/managed_serve.go` — resolve the active authenticated session and own its relay lifetime
- `internal/component/managed/client.go` — admit relay work independently of config notifications and cancel it on disconnect
- `internal/component/managed/doctor.go` and `internal/component/managed/register_doctor_codes.go` — register `doctor-managed-relay-profile-invalid` and report unresolvable startup `relay-profile` findings with fix
- `internal/component/plugin/yang/ze-plugin-conf.yang` — optional `relay-profile` string leaf on the outbound client entry
- `internal/component/managed/yang/ze-managed-cmd.yang` — declare relay command, scoped completion and help
- `internal/component/managed/register.go` — register the hub commands, `relay-not-granted` alarm kind, and device status fields
- `internal/component/managed/cmd_fleet.go` — include effective relay grant in device status
- `docs/architecture/fleet-config.md` — describe grants, authenticated wire, single-frame vs operation bounds, and lifetime
- `docs/architecture/fleet-alarms.md` — mark `relay-not-granted` registered and document its source anchors
- `docs/guide/fleet-config.md` — document limits and authorized relay use

## Files to Create
- `internal/component/plugin/server/managed_relay.go` — bounded session lookup, streaming, cancel and audit
- `internal/component/managed/alarm_relay.go` — register `relay-not-granted` alarm kind via `RegisterAlarmKind`
- `internal/component/managed/relay.go` — device-side admission, profile-bound dispatch, single-frame/operation caps, and cleanup
- `internal/component/managed/cmd_relay.go` — authorized dispatcher adapter, command, completion, help and group action
- `internal/component/managed/relay_test.go` — authority, output and lifecycle tests
- `internal/component/plugin/server/managed_relay_test.go` — hub bounds, cancellation, auditing and group results
- `test/managed/fleet-command-relay.ci` — end-to-end wiring through real sessions

## Design presentation

### What changes

The device applies local grant changes at each successful reload and retires running work when authority is withdrawn. Monitor commands use the device’s authorized streaming handlers. A hub operator can run a daemon command on a named connected device using the same managed TLS session that already carries its configuration. The hub admits the operator and target, then the device applies its locally granted profile. Streaming and cancellation follow the resulting operation, including a user leaving the console.

### Before and after

A device behind NAT currently answers its local command surface. With the read-only grant in its outbound hub block, an authorized operator at the hub can request its uptime and receive device data. A missing grant returns a refusal with the device-specific configuration line. A full pool rejects additional relay work while the next configuration push still proceeds.

### Alternatives set aside

An HTTP proxy would introduce a second web session and would not reach the shared command surfaces. Running commands inline in the device’s notification loop would put the next configuration delivery behind an open monitor. A hub-only authorization decision would erase the grant chosen by the device’s operator.

### Documents departed from

None.

### Choices for Thomas

The choices in the Task are settled. No further operator decision is needed for this design.

## Implementation Steps

1. Register the command and wire one authenticated read end to end with the normal dispatcher, then prove both refusal boundaries.
2. Add capability advertisement, profile reload, bounded streaming and cancellation. Exercise disconnect, an unresponsive reader and a live configuration push during maximum relay load.
3. Wire scoped completion/help, audit outcomes and the bounded group-read action. The task and page children consume the registered operation and supply their own UI tests.
