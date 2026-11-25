# Spec: fleet-3 -- Fleet Audit Trail

| Field | Value |
|-------|-------|
| Status | ready |
| Priority | P3 |
| Release reason | Adds a capability absent from the supported release; existing supported behavior and its obligations remain required. |
| Depends | spec-fleet-1-device-registry (closed 2026-11-25; documented in `docs/architecture/fleet-config.md`, "Device registry") |
| Phase | 3/10 of plan/post-release/features/spec-fleet-0-umbrella.md |
| Updated | 2026-11-25 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `internal/component/plugin/server/managed.go` -- ManagedConfigService (hook points)
4. `spec-fleet-0-umbrella.md` -- umbrella design decisions
5. `docs/architecture/fleet-config.md`, "Device registry" (`spec-fleet-1-device-registry`, closed 2026-11-25) -- device registry (dependency)

## Task

Add a centralized, append-only audit log for fleet operations. Every significant fleet
event is recorded with a timestamp, actor, device, and structured payload. The log is
stored in the hub's ZeFS store and queryable through the shared dispatcher. Fleet UI-6 owns the web audit view and export.

Currently there is no record of who pushed what config to which device and when. The
per-device transaction protocol logs locally, but there is no hub-side view. An operator
investigating "who changed edge-42's config at 3am?" has no answer.

### Key Design Decisions (from umbrella)

| Decision | Detail |
|----------|--------|
| Storage | Append-only entries in the live hub store. Keyed by hub commit sequence, with event time retained as data |
| No external dependency | No syslog, no database. ZeFS is sufficient at fleet scale (hundreds of devices) |
| Structured entries | JSON-encoded event with type, timestamp, device, actor, payload |
| Retention | Configurable max entries (default 10000). Oldest pruned on overflow |
| Query | By device, by event type, by time range. Dispatcher answers on every surface, including the web command surface. UI-6 owns the audit page |
| Fleet-6/7 event types | Audit must also record the new fleet-control events: `fleet disable` and `fleet enable` (actor, device, time), reconnect `diverged` detected, `config-held` received, and divergence resolution (`adopt` / `revert`, with which config won). These are high-value security/forensic events. Add them to the event-type set when fleet-3 is designed |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/fleet-config.md` -- existing fleet config protocol
  -> Constraint: the hub observes live activity at its authenticated session. A durable device receipt is needed only for an action the hub cannot observe (offline fleet disable or enable)
- [ ] `internal/component/plugin/server/managed.go` -- ConfigReader, HandleConfigFetch and BuildConfigChanged; managed_serve.go owns session admission and disconnect
  -> Decision: audit calls added after each operation (non-blocking)

**Key insights:**
- Audit entries must not block the config-fetch path (async append)
- config-ack carries ok/error; both outcomes are audit-worthy
- Device connect/disconnect from fleet-1 registry are natural audit points

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/server/managed.go` -- no audit logging exists
  -> Constraint: audit is purely additive; all existing behavior unchanged
- [ ] `internal/component/config/archive/archive.go` -- config archive system (different purpose: archives config content, not operational events)
  -> Decision: audit log is separate from config archive (different granularity and purpose)

**Behavior to preserve:**
- All existing ManagedConfigService behavior unchanged
- Config archive system unchanged (complementary, not replaced)
- Config-fetch latency not impacted by audit (async writes)

**Behavior to change:**
- Audit entry appended on: device connect, device disconnect, config-fetch, config-ack (ok), config-ack (reject), config-changed sent, template change (fleet-2)
- The registered `show fleet audit` row answer supports the command-specific pipes `device`, `type`, `since` and `until`
- Fleet UI-6 renders the hub audit rows through its authorized source and owns the web view and export

## Existing Mechanisms

| Mechanism | Source | What this design uses |
|---|---|---|
| Daemon audit entries and outcome reporting | `internal/core/audit/audit.go` and `write_outcome.go` | Reuse the actor, surface, timestamp and outcome conventions and retention bounds (100 to 100,000, default 10,000). The local JSON-lines audit remains independent. It rewrites its file on every write and cannot supply a nonblocking estate history in ZeFS. |
| Managed observations | `managed_serve.go`, `managed_registry.go` and `managed_held.go` under `internal/component/plugin/server/` | Record successful admissions, disconnects, fetch, notification delivery and ack at the operation performing them. A receipt of held bytes and a later decision are separate events. |
| Divergence decisions | `internal/component/managed/cmd_divergence.go`, `revertHeldLocked`, `adoptHeldLocked`, and the existing local audit action declarations | Share one success/outcome at the guarded mutation. Record which version won. The local command forwards to the running hub. A stopped hub refuses adopt and revert. The daemon records the successful decision once and a later acknowledgement is a distinct event. |
| Local fleet mode and receipts | `internal/component/managed/cmd_fleet.go`, `disableOnDaemon` and `enableOnDaemon` and their local command paths; `internal/component/managed/client.go`, `runConnection`; `pkg/fleet/envelope.go`, `ConfigFetchRequest` and existing named wire verbs | Enqueue the locally observed enable/disable with the mode transition. On the next authenticated session, drain the bounded outbox and acknowledge the persisted prefix. |
| Runtime diagnostics and persistence | `internal/component/plugin/server/managed_doctor.go`, `ManagedDoctorSnapshot`; `internal/component/plugin/doctor/check_managed_runtime.go`, `checkManagedDeviceRecordUnwritable`; `managed_record_store.go` in the server package | Follow the existing retained failure counters and registered runtime check for journal persistence, loss or unreadable history. Journal entries have a separate namespace and immutable generations. |
| Managed configuration and lifetime | `internal/component/config/loader_extract.go`, `ExtractHubConfig`; `internal/component/plugin/types.go`, `HubServerConfig`; `cmd/ze/hub/managed_server.go`, `startManagedServer` and `prepareManagedReload` | Read the effective retention bound on startup and accepted reload, and give the writer the same lifetime as the serving hub. |
| Per-device authorization | `internal/component/plugin/server/command.go`, `CommandContext` and the dispatcher argument authorizer; `internal/component/authz/authz.go`, `Store.AuthorizeWithProfiles` | Before reading a device’s rows, use the caller’s request-bound authorizer on the existing read subject `show fleet device name <device>`, including the actual device argument. Retain the outer `show fleet audit` permission as a separate prerequisite. Cache each device verdict for this query only. Filter, count, and export use that same admitted set. |
| General live-store mutation | `internal/component/config/storage/cli/data_rpc.go`, `handleDataKeyDelete`, `handleDataKeyUpdate` | Both commands currently permit generic runtime keys. Refuse the new audit manifest, journal pages, cursors and device outbox keys before touching the live store. Match the registered key patterns, including newly created keys. The owner’s internal writer can still publish and prune them. |
| Command filters | `internal/component/command/pipe_filter.go`, `RegisterPipeFilters` | Expose a typed row set and exact device/type and inclusive RFC 3339 interval selectors. Filter authorization precedes pruning by user-supplied predicates, sorting and limiting. |
| Web consumers | `internal/component/web/fleetui/source_config.go`, `ConfigSource.AuditEntries`, `ConfigVersions`, `ConfigText` | The UI-6 adapter asks the authorized hub history for at most 1,000 audit rows and uses the existing version store for configuration history/text. Audit records carry version references only. |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|---|---|---|---|---|---|
| `ai/doors/system.md` | Ze is a platform, and other people's code runs against it | the door every change reads | "`pkg/plugin/sdk`, `pkg/plugin/rpc`, `pkg/ze`, `pkg/zefs` and `pkg/fleet` are imported by code outside this repository." | agrees | The new envelope is documented with the managed protocol. Existing peers keep their configuration path when the new feature is absent. |
| `docs/architecture/fleet-config.md` | Key Rules | the // Design: page of internal/component/plugin/server/managed_serve.go | "The client name in `client edge-01 { }` IS the client's identity" | agrees | Per-client authentication binds the device name to this session; optional metadata and the new verb leave ordinary configuration and holds intact. |
| `docs/architecture/fleet-config.md` | Design decisions | the // Design: page of pkg/fleet/envelope.go | "Thomas chose “Allow first provisioning and later edits from the device’s web Config tab”. An authorized operator can commit the first or a later version of a declared device’s configuration while it is offline, through the post-release fleet web surface." | agrees | Record a first or later device configuration commit with its authenticated actor even while the device is offline. Delivery, acknowledgement and rejection are distinct later observations. |
| `docs/architecture/api/architecture.md` | REST and gRPC authentication flow | anchored to internal/component/plugin/server/managed_serve.go | "Only surviving stored users retain their recovery profile." | agrees | Authorization remains bound to the actual hub user. Device names and asserted actor strings cannot acquire a locally reserved or recovery identity. |
| `docs/architecture/core-design.md` | Command engine and composition roots | anchored to internal/component/plugin/server/managed_serve.go | "Composition, not dispatch, separates the programs." | agrees | Fleet commands enter the normal product registration; the existing command engine serves them. |
| `docs/architecture/hub-architecture.md` | Startup Sequence | anchored to internal/component/plugin/server/managed_serve.go | "The hub also cancels and joins the complete outbound managed client, because its callbacks can read or write storage before reload admission. A managed start holds that client in one owner, which `request fleet disable` and `enable` also stop and start; at shutdown the owner joins the running client and refuses every later start. It retains the managed server and joins its active config requests and notifications through `Stop`. Both owners finish before the hub returns its store to the launcher." | agrees | Session workers stop and join before the managed owner releases the daemon store. |
| `docs/architecture/pki/tls-listeners.md` | Decision: a named certificate fails closed | anchored to internal/component/plugin/server/managed_serve.go | "A configured name that does not resolve returns an error and no material." | agrees | An unresolved configured certificate prevents the handshake. Only an authenticated live session carries this operation. |
| `ai/doors/plugin-infra.md` | Traps | the door covering internal/component/plugin/server/managed_serve.go | "so a payload must be compact `json.Marshal` output." | agrees | Wire values use compact JSON, bounded before decoding, carried by the authenticated named session. |
| `docs/architecture/api/commands.md` | Dispatch keys are YANG paths | anchored to internal/component/plugin/server/managed_held.go | "The daemon registers each built-in handler under its YANG command path" | agrees | The registered subject returns typed data. Filtering and formatting run on the result. |
| `docs/architecture/doctor-and-health-checks.md` | The runtime phase: checks only the daemon runs | anchored to internal/component/plugin/server/managed_held.go | "A check over such state registers in the `runtime` phase, through the same `diagnostic.RegisterDoctorCheck` every other check uses" | agrees | The live writer publishes loss or failure for a runtime doctor check; a later successful write clears a write failure while evidence of a real gap is retained. |
| `ai/doors/hub-engine.md` | Boundaries | the door covering internal/component/managed/cmd_divergence.go | "SIGHUP and a managed push both reach `doReload` (`cmd/ze/hub/main_reload.go`). A new trigger joins it rather than building a second path." | agrees | The managed client and hub own session cancellation and worker shutdown. The existing configuration reload path stays authoritative. |
| `ai/rules/repo-maintenance.md` | Directives | the // Design: page of internal/component/managed/register.go | "A change that adds or changes a surface future agents use, verify, document or avoid MUST update its discovery path in the same work" | agrees | The managed guide, command documentation and source anchors follow the new registered surface. |
| `docs/architecture/config/syntax.md` | Overview | anchored to internal/component/plugin/yang/ze-plugin-conf.yang | "The parser is YANG-driven: each config node's type (leaf, leaf-list, container, list) determines how it is parsed." | agrees | The new leaf is declared in YANG and validated by the existing parser. |
| `docs/architecture/zefs-format.md` | Key Namespaces | the // Design: page of pkg/zefs/keys.go | "Keys follow a `<namespace>/<qualifier>/<path>` convention to prevent collisions between metadata and config files." | agrees | The retained fleet audit and device receipts use owned runtime metadata keys outside meta/config, with the namespace declared once. |
| `docs/architecture/container/image-store.md` | The data directory and the store layout | anchored to pkg/zefs/keys.go | "" | silent | The audit rows are fleet events. Container image keys and image collection do not participate. |
| `docs/architecture/diagnostics/debug-filtering.md` | Profiles are one JSON value | anchored to pkg/zefs/keys.go | "" | silent | Debug profile selection does not participate in recording fleet events. |
| `docs/architecture/irr-owner.md` | Where it sits | anchored to pkg/zefs/keys.go | "" | silent | The IRR dataset owner does not read or write fleet audit history. |
| `docs/architecture/storage-backends.md` | Opening and ownership | anchored to pkg/zefs/keys.go | "`Open(configDir)` opens the live tree and holds `configDir/database.lock` until `Close`." | agrees | Use the store already opened by the daemon or the offline command holding its ownership lock. Join the writer before closing it. |
| `ai/doors/core.md` | Boundaries | the door covering pkg/zefs/keys.go | "In the daemon, use `statestore` with a key registered in `pkg/zefs/keys.go`" | agrees | The daemon borrows its live store and declares all new persistent keys in pkg/zefs. |
| `ai/doors/le-tooling.md` | Where it sits | the door covering test/managed/fleet-audit-log.ci | "Functional `.ci` files live under `test/<suite>/` and run through `./le test <suite>`" | agrees | A test/managed functional test exercises a real managed connection through the normal launcher. |
| `docs/architecture/fleet-config.md` | Design decisions | the // Design: page of internal/component/plugin/server/managed_serve.go | "A reload that changes the certificate a serving hub's managed listener presents installs it on the running listener at that reload: the next handshake presents the certificate the configuration names, the listener keeps its socket and no session is closed, as the web and looking glass listeners do. The certificate stays in the configuration's `pki` block. In Thomas's words (voice-transcribed): "Keep the certificate in the configuration." and "In that case on commit we can perform the certificate change on the listener."" | agrees | Updating listener credentials changes subsequent handshakes, and keeps existing authenticated sessions and their outstanding work. |
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
| `docs/architecture/api/commands.md` | Design decisions | anchored to internal/component/plugin/server/managed_held.go | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | Fleet audit filters act only on the authorized rows. The generic format and delivery operators keep their normal meanings. |
| `docs/architecture/api/commands.md` | Design decisions | anchored to internal/component/plugin/server/managed_held.go | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | Fleet audit filters act only on the authorized rows. The generic format and delivery operators keep their normal meanings. |
| `docs/architecture/api/commands.md` | Design decisions | anchored to internal/component/plugin/server/managed_held.go | "A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does" | agrees | Fleet audit filters act only on the authorized rows. The generic format and delivery operators keep their normal meanings. |
| `ai/doors/config-yang.md` | Traps | the door covering internal/component/config/loader_extract.go | "`ze config validate` never calls a live `OnConfigVerify`." | agrees | Extraction and semantic validation run against the configuration tree without a live daemon. |
| `docs/architecture/api/process-protocol.md` | Wire Format | the // Design: page of internal/component/plugin/types.go | "Maximum message size is 16 MB." | agrees | Use the existing framed transport. The operation bounds applied here fit within its message ceiling. |
| `docs/architecture/api/ipc_protocol.md` | Message Framing | anchored to internal/component/plugin/types.go | "All plugin RPC messages are UTF-8, newline-delimited lines:" | agrees | The new verb and answers use the existing correlated newline-delimited frames. |
| `docs/architecture/api/json-format.md` | Field naming | anchored to internal/component/plugin/types.go | "JSON keys are lowercase kebab-case, never camelCase and never snake_case." | agrees | Use lowercase kebab-case keys for new JSON fields in the protocol and command answer. |
| `docs/architecture/cli/command-verbs.md` | Narrowing and shaping: the pipe layer | anchored to internal/component/plugin/types.go | "An owned filter takes the name of the field it selects on." | agrees | The outer command selects its subject. Filters apply to already authorized data. |
| `docs/architecture/cli/error-surface.md` | The operator error | anchored to internal/component/plugin/types.go | "A refusal an operator reads is an `opererr.Error`" | agrees | Return structured operator errors giving the cause and the applicable fix. |
| `docs/architecture/config/transaction-protocol.md` | 1. Design Principles | anchored to internal/component/plugin/types.go | "The hub promotes a candidate after runtime reload succeeds." | agrees | Publish the effective setting only on acceptance. A rejected reload retains the live setting and ongoing state. |
| `docs/architecture/encoding-context.md` | TL;DR (Read This First) | anchored to internal/component/plugin/types.go | "" | silent | The page defines BGP negotiated data-plane encoding and does not specify managed RPC or fleet configuration. |
| `docs/architecture/plugin-manager-wiring.md` | Stage 1 Registration: Config Transaction Fields | anchored to internal/component/plugin/types.go | "" | silent | These startup fields describe plugin transports and budgets. The managed service adds no plugin Stage 1 field. |
| `docs/architecture/web-interface.md` | Component Structure | anchored to internal/component/plugin/types.go | "Every page, fragment and editor is a templ component." | agrees | Fleet UI children own the templ pages. This backend supplies the same authorized answer to the UI and other command surfaces. |

### Record and retention

An entry has a hub-assigned monotonically increasing sequence, UTC event and receipt times, action, device, authenticated actor and surface, outcome and bounded, typed metadata (for example the version, template or rollout). Device-side receipts identify their authenticated sender. A peer’s text cannot supply the hub actor. Protocol automation uses a named service actor. Config text, command arguments containing secrets, tokens and free-form peer errors never enter the record. Rejections retain their safe diagnostic code and bounded, redacted explanation.

A single hub writer drains a bounded queue of 1,024 immutable events in admission order. It appends a batch under the single runtime-state writer and trims the oldest positions to the configured ceiling. Write immutable pages of at most 128 rows, then atomically replace one manifest key containing the page references, retained sequence bounds and device receipt cursors. That single write publishes the batch. Garbage-collect unreferenced pages after publication or at recovery. Pin the visible generation while a reader copies rows. On crash the prior or next manifest remains readable. Read one published generation per query. Equal timestamps and restarts retain distinct sequence numbers. Scan newest first, authorized rows only, then apply the caller's selectors. A changed retention bound prunes on publication. Report unreadable history, dropped records and persistence failures with the count and time through doctor. A full queue or a failed append cannot block config delivery. Shutdown stops admission then drains and joins the writer before the store closes. An abrupt power loss may lose queued observations. Confirmed events and decisions are distinct from intents.

An offline fleet disable is invisible to the hub. The device records disable/enable with the local mode transition, including the actor and event time, in a bounded ZeFS outbox. On authenticated reconnect it sends a bounded batch using a dedicated `fleet-audit-receipts` verb on the existing channel. The hub assigns receipt times and keys receipts by authenticated device plus a persistent device epoch and sequence. It commits entries and the last accepted sequence together before acknowledging. A retry after loss of an acknowledgement yields the same logical entries, even if a previously retained entry has aged out. The device advances its outbox only after acknowledgement. On overflow it preserves the sequence gap and number lost and reports the gap to doctor and the hub. Prior to reconnection the hub can attest only to disconnection. Older peers without receipt support still exchange configurations.

The device uses a bounded receipt journal in which each local event carries a persistent monotonic sequence and an origin epoch. Guard the mode transition and receipt under a recoverable write intent: persist the intent before changing mode, finish or discard it against the recorded old/new value on recovery, and expose a receipt only for a completed transition. Keep at most 1,024 receipts, send up to 32 entries of at most 1 KiB each per message, validate all fields, and acknowledge only the contiguous prefix recorded or explicitly declared lost. Ordinary reconnect alone cannot fabricate a mode change.

A producer added later (templates, rollout or relay) registers its structured fleet events beside the operation that owns them. The fleet audit package accepts a value and knows no list of producers. A template change records actor, old/new version and affected devices when committed; an attempted delivery, device ack, rejection, hold and final resolution share identifiers without implying one another. A device's ordinary negative ack and a held-version ack remain distinguishable.

## Data Flow (MANDATORY)

### Changed-Wire Census

**Search evidence:** Searched `pkg/`, `cmd/`, `internal/`, `test/` (Go, shell, Python and functional suite bodies) for `VerbConfigFetch`, `ConfigFetchRequest`, `config-fetch`, `relay-command`, `relay-cancel` and `fleet-audit-receipts`. The current positive control is `managed.fetchConfigWithin` sending to `ManagedServer.handleRequest`. The new relay and receipt verbs have no existing production speaker. The scoped shell, Python and functional suite search found no writers or readers of those verbs.

| Wire | Role | Carrier | Source evidence | Disposition |
|---|---|---|---|---|
| `fleet-audit-receipts` batch | writer | Device `internal/component/managed/audit_receipts.go`, new outbox sender in the session owned by `runConnection` | `internal/component/managed/client.go`, `runConnection` and `FetchConfig` | Drain at most 32 receipts after authentication and normal config negotiation. |
| `fleet-audit-receipts` batch | reader | Hub `internal/component/plugin/server/managed_serve.go`, `ManagedServer.handleRequest` and new journal writer | Same serving loop currently reads `VerbConfigAck` and `VerbConfigHeld` | Bind a batch to the session's authenticated name, validate bounds/order and persist its contiguous prefix. |
| Receipt acknowledgement | writer | Hub journal writer after manifest publication, returned through `ManagedServer.handleRequest` | `pkg/plugin/rpc/mux.go`, `SendResult` | Echo device epoch and committed prefix; a lost ack is safely replayed. |
| Receipt acknowledgement | reader | Device outbox sender in `internal/component/managed/audit_receipts.go` | `pkg/plugin/rpc/mux.go`, `CallRPC` | A matching epoch and bounded prefix permit trimming. An error preserves pending receipts. |
| Fleet audit query rows | writer | New `internal/component/managed/cmd_audit.go` registered under managed YANG | `internal/component/managed/cmd_devices.go`, `handleFleetDevices` illustrates the serving pattern | Emit authorized typed rows with sequence, actor, device, event, version, time and outcome. |
| Fleet audit query rows | reader | Shared command surfaces and post-release UI-6 audit source adapter | `internal/component/web/fleetui/source_config.go`, `ConfigSource.AuditEntries` | CLI/REST/gRPC/MCP/SSH/web command readers see the same per-caller row set. UI-6 separately renders/exports a bounded authorized selection. |

### Entry Point
- Fleet operations trigger audit entries (connect, disconnect, config events)
- Operator queries audit via CLI or web

### Transformation Path
1. Fleet operation occurs (e.g., HandleConfigFetch completes)
2. Audit entry constructed: type, timestamp, device name, actor, payload
3. Entry appended to audit log in the live hub store (async, non-blocking)
4. CLI/web queries read the published audit generation with optional filters

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Fleet operation to Audit | Audit.Append() call after operation | [ ] |
| Audit to ZeFS | Publish a durable bounded history generation | [ ] |
| Audit to CLI/Web | Query API with filters | [ ] |

### Integration Points
- `ManagedConfigService` -- audit calls in ConfigReader, HandleConfigFetch and BuildConfigChanged; managed_serve.go owns session admission and disconnect
- `DeviceRegistry` from fleet-1 -- device context for audit entries
- CLI -- `show fleet audit` command family
- Web -- audit log table on fleet page

### Architectural Verification
- [ ] No bypassed layers (audit hooks into existing operation path)
- [ ] No unintended coupling (audit is append-only observer; removal has no functional impact)
- [ ] No duplicated functionality (separate from config archive)
- [ ] Zero-copy preserved where applicable

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Device connects to hub | -> | Audit entry: device.connect | `test/managed/fleet-audit-log.ci` |
| Device completes config-fetch | -> | Audit entry: config.fetch | `test/managed/fleet-audit-log.ci` |
| `show fleet audit` CLI | -> | Audit log query with entries | `test/managed/fleet-audit-log.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|---|---|---|---|
| AC-1 | Device connects to hub | Audit entry recorded: type=device.connect, device=name, timestamp | Rejected connections can be separately diagnosed; this entry attests to a successful authenticated connection. |
| AC-2 | Device disconnects | Audit entry: type=device.disconnect, device=name | An unrelated active device keeps its own connection observation. |
| AC-3 | Device completes config-fetch (new config) | Audit entry: type=config.fetch, device=name, version=hash | A current fetch has no new version applied. |
| AC-4 | Device sends config-ack (ok) | Audit entry: type=config.ack, device=name, version=hash, ok=true | The earlier offer remains recorded because it was a separate hub observation. |
| AC-5 | Device sends config-ack (reject) | Audit entry: type=config.reject, device=name, version=hash, safe diagnostic | The rejected version stays queryable, even though it never became active. |
| AC-6 | Hub sends config-changed | Audit entry: type=config.push, device=name, version=hash | An unacknowledged delivery stays an offer until an ack arrives. |
| AC-7 | `show fleet audit` | All recent audit entries shown, newest first | Only caller-authorized retained entries belong to this answer. |
| AC-8 | `show fleet audit \| device edge-01` | Only entries for edge-01 | Every authorized edge-01 event type still qualifies. |
| AC-9 | `show fleet audit \| type config.fetch` | Only config.fetch entries | All authorized devices with such an event still qualify. |
| AC-10 | `show fleet audit \| since 2026-05-15T00:00:00Z` | Only entries at or after the RFC 3339 instant | An equal timestamp qualifies at the inclusive lower bound. |
| AC-11 | Audit log exceeds max entries (default 10000) | Oldest entries pruned | Keep the newest configured number, in sequence order. |
| AC-12 | Hub restarts | Audit log loaded from the hub’s live store; entries preserved | Unpublished queued observations can be lost on an abrupt crash. |
| AC-13 | Config-fetch with audit append failure (live-store write error) | Config-fetch still succeeds; audit failure logged and exposed in doctor without failing the fetch | All previously published rows stay readable. |
| AC-14 | A client disables fleet mode while disconnected, edits locally and later enables and reconnects | The hub reports the original disable and enable actor/times separately from transport disconnect/connect and a held upload. Losing the receipt ack and reconnecting duplicates no operation. | An offline device that never reconnects cannot have its local action witnessed by the hub. |
| AC-15 | A held version is adopted or a revert is decided and later applied | The row names the actor, device, outcome and version chosen. A rejection or superseded decision never says a config was applied. | Prior attempts remain separate from the operator’s final resolution. |
| AC-16 | A template or rollout fans out to devices | A committed template change, per-device offer, ack and rejection can be correlated without storing credentials or config text. A user scoped to one device can read only its rows, including through pipes and export. | Rows on the caller’s allowed devices remain available. |
| AC-17 | Many equal-time events, a queue overflow, a failed batch, restart and retention reduction | Confirmed sequence order survives restart, the newest permitted entries remain queryable, loss is diagnosed, and the managed config path remains available. | A queue loss remains visible as a diagnosed gap. |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestAuditLogAppend` | `internal/component/plugin/server/audit_test.go` | Entry appended and retrievable | |
| `TestAuditLogPersist` | `internal/component/plugin/server/audit_test.go` | Entries survive restart from the live store | |
| `TestAuditLogFilterDevice` | `internal/component/plugin/server/audit_test.go` | Filter by device name | |
| `TestAuditLogFilterType` | `internal/component/plugin/server/audit_test.go` | Filter by event type | |
| `TestAuditLogFilterSince` | `internal/component/plugin/server/audit_test.go` | Filter by timestamp | |
| `TestAuditLogPrune` | `internal/component/plugin/server/audit_test.go` | Oldest entries pruned at max capacity | |
| `TestAuditLogAsyncNonBlocking` | `internal/component/plugin/server/audit_test.go` | Append does not block caller | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `fleet-audit-log` | `test/managed/fleet-audit-log.ci` | Device connects, fetches config, disconnects; all events in audit log | |

## Files to Modify
- `internal/component/plugin/server/managed_serve.go` — observe authenticated sessions, offers and acknowledgements and accept receipt batches
- `internal/component/plugin/server/managed_held.go` — record acceptance of a held version
- `internal/component/managed/cmd_divergence.go` — publish guarded adopt/revert outcomes on the running hub
- `internal/component/managed/cmd_fleet.go` — record local disable/enable receipts including offline changes
- `internal/component/managed/client.go` — deliver retained receipts on the managed session
- `internal/component/managed/register.go` — register the daemon audit reader, pipes and authorization
- `internal/component/managed/yang/ze-managed-cmd.yang` — declare show fleet audit
- `internal/component/plugin/yang/ze-plugin-conf.yang` — declare the fleet audit-retention bound under the hub managed server configuration
- `internal/component/config/loader_extract.go` and `internal/component/plugin/types.go` — extract and carry the configured retention bound
- `cmd/ze/hub/managed_server.go` — install the writer at start and apply the retention bound on an accepted reload
- `internal/component/plugin/server/managed_doctor.go` — publish bounded failure observations
- `internal/component/plugin/doctor/check_managed_runtime.go` — expose journal loss and persistence failures through daemon doctor
- `pkg/fleet/envelope.go` — declare bounded receipt batches and ack cursor
- `internal/component/config/storage/cli/data_rpc.go` — refuse direct live mutation of journal, manifest and receipt keys
- `pkg/zefs/keys.go` — name fleet audit and receipt namespaces and identify their protected keys
- `docs/architecture/zefs-format.md` — document the new key patterns, writer, reader and operator access
- `docs/architecture/fleet-config.md` — document observation points, receipt wire format and durability
- `docs/guide/audit.md` — document fleet rows, permissions, loss reporting and pipes

## Files to Create
- `internal/component/plugin/server/audit.go` — ordered hub writer, store and query
- `internal/component/plugin/server/audit_test.go` — exercise ordering, retention, overload, recovery and retry
- `internal/component/managed/cmd_audit.go` — present authorized rows on every dispatcher surface
- `internal/component/managed/audit_receipts.go` — persist, bound and replay locally observed mode changes
- `test/managed/fleet-audit-log.ci` — exercise a real hub, client, operator and restart

## Design presentation

### What changes

The hub keeps the latest 10,000 fleet events by default. Each row ties an observed action or decision to its authenticated source, device, versions and outcome. Operators query it on all command surfaces. Fleet UI-6 owns the web table and export.

### Before and after

After an operator adopts edge-01’s held version, the hub shows the chosen version and actor. A later successful ack is its own event. A disconnected device appears offline immediately; a local disable performed there becomes known on authenticated receipt at reconnect. Filtering for edge-01 shows its records only.

### Alternatives set aside

Scraping device logs would lose the hub’s template, admission and resolution decisions. Logging every fetch synchronously in the local audit file would make its filesystem latency part of configuration delivery. The hub writer keeps a bounded estate history and reports loss explicitly.

### Documents departed from

None.

### Choices for Thomas

The choices in the Task are settled. No further operator decision is needed for this design.

## Implementation Steps

### Implementation Phases

1. Wire a successful session observation to the typed query and prove it through the normal hub command. Verify the returned data through the CLI, REST, gRPC, MCP, SSH and web command surface.
2. Add sequenced persistence, retention changes, recovery and the writer lifecycle. Test queue saturation, repeated timestamps, startup with corrupt storage, restart after a failed batch, and filter ordering.
3. Extend the live producers, guarded hold resolution and device receipt outbox. Prove actor attribution, disconnected enable/disable, bounded replay and deduplication across an ack loss. Verify sensitive bytes never occur in the stored payload.
4. Register query authorization and command pipes, update the operator guide and document the consumer contract used by UI-6.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All AC-1 through AC-17 have implementation |
| Correctness | Audit does not block config-fetch path; async append |
| Naming | Event types use dot-separated kebab-case (device.connect, config.fetch) |
| Data flow | Operation -> bounded enqueue -> live-store publication -> CLI/web query |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Audit integrity | Append-only through fleet operations and the running daemon’s data commands. Owner maintenance of a stopped store or restoration of a backup is outside the live journal contract. The journal prunes only records past retention. |
| Information leakage | Audit entries do not contain config content or secrets; only version hashes |
| Resource exhaustion | Max entries bounded; pruning prevents unbounded growth |

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
<!-- Add a new block per re-run. Final run MUST show zero BLOCKER/ISSUE. -->
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1 through AC-17 demonstrated
- [ ] Wiring Test table complete
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`)

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
