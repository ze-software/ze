# Spec: fleet-3 -- Fleet Audit Trail

| Field | Value |
|-------|-------|
| Status | done |
| Priority | P3 |
| Release reason | Adds a capability absent from the supported release; existing supported behavior and its obligations remain required. |
| Depends | spec-fleet-1-device-registry (closed 2026-11-25; documented in `docs/architecture/fleet-config.md`, "Device registry") |
| Phase | 3/10 of plan/post-release/features/spec-fleet-0-umbrella.md |
| Updated | 2026-11-28 |

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
| Fleet operation to Audit | `ManagedServer.RecordAudit` -> `ManagedAuditStore.Append` non-blocking enqueue | [x] |
| Audit to ZeFS | Immutable pages (`meta/fleet/audit/page/{page}`) + atomic manifest (`meta/fleet/audit/manifest`) | [x] |
| Audit to CLI/Web | `show fleet audit` (`ze-managed:show-fleet-audit`) with `device`/`type`/`since`/`until` pipes and `FleetDeviceScope` | [x] |

### Integration Points
- `ManagedConfigService` -- audit calls in `managed_serve.go`, `managed_registry.go`, `managed_held.go`, `cmd_divergence.go`, and `cmd_fleet.go`
- `DeviceRegistry` from fleet-1 -- device context and per-device authorization scope
- CLI -- `show fleet audit` command family across CLI, SSH, REST, gRPC, MCP, and web command surfaces
- Web -- Fleet UI-6 consumes the same authorized query contract

### Architectural Verification
- [x] No bypassed layers (audit hooks into existing operation paths)
- [x] No unintended coupling (audit is non-blocking append-only observer; failures never stall config delivery)
- [x] No duplicated functionality (separate from config archive and local JSONL audit)
- [x] Zero-copy and bounded allocations preserved (pinned generations, pre-decode byte bounds)

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
| `TestAuditLogAppend` | `internal/component/plugin/server/audit_test.go` | Entry appended and retrievable | PASS |
| `TestAuditLogPersist` | `internal/component/plugin/server/audit_test.go` | Entries survive restart from the live store | PASS |
| `TestAuditLogFilterDevice` | `internal/component/plugin/server/audit_test.go` | Filter by device name | PASS |
| `TestAuditLogFilterType` | `internal/component/plugin/server/audit_test.go` | Filter by event type | PASS |
| `TestAuditLogFilterSince` | `internal/component/plugin/server/audit_test.go` | Filter by timestamp | PASS |
| `TestAuditLogPrune` | `internal/component/plugin/server/audit_test.go` | Oldest entries pruned at max capacity | PASS |
| `TestAuditLogAsyncNonBlocking` | `internal/component/plugin/server/audit_test.go` | Append does not block caller | PASS |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `fleet-audit-log` | `test/managed/fleet-audit-log.ci` | Device connects, fetches config, disconnects; all events in audit log | PASS |

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

---

## Implementation Summary

### What Was Implemented
- **ZeFS namespace & live-mutation protection** (`pkg/zefs/keys.go`, `pkg/zefs/keys_test.go`, `internal/component/config/storage/cli/data_rpc.go`, `data_rpc_test.go`, `data_rpc_update_test.go`): registered `KeyFleetAuditManifest`, `KeyFleetAuditPage`, `KeyManagedAuditReceipts`, and `KeyManagedAuditIntent` with explicit `Private: true` and `ValueMax` ceilings, plus `IsFleetAuditProtectedKey`. `handleDataKeyDelete` and `handleDataKeyUpdate` refuse direct live mutation of all four key families and any key under `meta/fleet/audit/`.
- **Wire envelope for offline device receipts** (`pkg/fleet/envelope.go`, `pkg/fleet/envelope_test.go`): added `VerbFleetAuditReceipts` (`fleet-audit-receipts`), `AuditReceiptEntry`, `AuditReceiptBatch`, and `AuditReceiptAck` with explicit bounds (`FleetAuditReceiptsOutboxMax = 1024`, `FleetAuditReceiptsBatchMax = 32`, `FleetAuditReceiptEntryBytesMax = 1024`, `FleetAuditReceiptBatchBytesMax = 64 KiB`, `FleetAuditReceiptAckBytesMax = 4 KiB`) and strict validation.
- **YANG configuration & live reload** (`internal/component/plugin/yang/ze-plugin-conf.yang`, `internal/component/plugin/types.go`, `internal/component/config/loader_extract.go`, `hub_audit_config_test.go`, `internal/component/plugin/server/managed_apply.go`, `cmd/ze/hub/managed_server.go`): added `bus hub audit max-entries` (`100..100000`, default `10000`), `HubAuditConfig` / `ValidateHubAuditConfig`, `extractHubAuditConfig`, and `PrepareManagedListenersWithHistoryAlarmsAndAudit` + `ApplyManagedListeners` live ceiling updates.
- **Centralized hub audit store & live producers** (`internal/component/plugin/server/audit.go`, `audit_test.go`, `audit_wiring_test.go`, `managed_serve.go`, `managed_registry.go`, `managed_held.go`, `cmd/ze/hub/main.go`, `internal/component/managed/cmd_divergence.go`): implemented `ManagedAuditStore` with a single long-lived async writer goroutine draining a bounded 1,024-entry queue into immutable 128-entry pages + atomic manifest, generation-pinned concurrent readers, orphan/obsolete page GC, startup corrupt-key recovery, per-device receipt deduplication across lost acks and retention pruning, and live event hooks for `device.connect`, `device.disconnect`, `config.fetch`, `config.push`, `config.ack`, `config.reject` (safe enum classification, never raw peer errors), `config.held`, `divergence.adopt`, `divergence.revert`, `fleet.disable`, `fleet.enable`, and `fleet.receipt.gap`.
- **Device-side offline receipt outbox & recoverable write intent** (`internal/component/managed/audit_receipts.go`, `audit_receipts_test.go`, `cmd_fleet.go`, `client.go`, `cmd/ze/hub/managed.go`): guarded `request fleet disable` and `request fleet enable` (running daemon and stopped store) under `executeManagedModeChange` and `recoverAuditReceiptIntent`, storing receipts in `meta/managed/audit-receipts` (max 1,024 with overflow gap tracking) and draining batches on authenticated reconnect via `drainAuditReceipts` with backward compatibility for older hubs returning `unknown method`.
- **Authorized query command & pipes** (`internal/component/managed/yang/ze-managed-cmd.yang`, `internal/component/managed/cmd_audit.go`, `cmd_audit_test.go`, `register.go`, `internal/component/plugin/all/testdata/wire-methods.snapshot`): registered `show fleet audit` (`ze-managed:show-fleet-audit`, `ShapeTab`) with command-specific pipes `device`, `type`, `since`, and `until`, enforcing per-device `FleetDeviceScope` authorization (`show fleet device name <device>`) with per-query caching before predicates.
- **Runtime and post-config doctor checks** (`internal/component/plugin/server/managed_doctor.go`, `doctor_codes.go`, `internal/component/plugin/doctor/check_managed_runtime.go`, `check_managed_runtime_test.go`, `register_external.go`, `internal/component/managed/doctor.go`, `register_doctor_codes.go`): registered `doctor-managed-audit-write-failed`, `doctor-managed-audit-dropped`, `doctor-managed-audit-corrupt`, and `doctor-managed-audit-receipt-gap`.
- **End-to-end functional test & news fragment** (`test/managed/fleet-audit-log.ci`, `internal/test/fixture/misc_fixture_managed_fleet_audit.go`, `register_managed_fleet_audit.go`, `website/changes/fragments/fleet-3-audit-trail.md`).

### Bugs Found/Fixed
- **Run 1 BLOCKER 1 (`wire-methods.snapshot`):** added `ze-managed:show-fleet-audit` in sorted order; covered by `TestRegisteredWireMethods`.
- **Run 1 BLOCKER 2 (pre-decode byte bounds):** added `FleetAuditReceiptBatchBytesMax` (64 KiB) check before `json.NewDecoder` in `ManagedServer.handleFleetAuditReceipts` and `FleetAuditReceiptAckBytesMax` (4 KiB) check in `decodeAuditReceiptAck`; covered by `TestManagedServerAuditReceiptsRejectsOversizedPayload` and `TestAuditReceiptAckRejectsOversizedPayload`.
- **Run 1 ISSUE 3 (32-bit `strconv.Atoi` allowlist):** switched `extractHubAuditConfig` to 64-bit `strconv.ParseInt` with `[math.MinInt32, math.MaxInt32]` clamp; covered by `TestExtractHubConfigAudit` and `./le repo check`.
- **Run 1 ISSUE 4 (missing hub runtime doctor unit tests):** added `TestCheckManagedAuditWrite`, `TestCheckManagedAuditDropped`, and `TestCheckManagedAuditCorrupt` in `internal/component/plugin/doctor/check_managed_runtime_test.go`.
- **Run 1 ISSUE 5 (missing discovery docs):** added `show fleet audit` entries and source anchors to `ai/INDEX.md`, `docs/guide/command-reference.md`, and `docs/architecture/api/commands.md`.
- **Run 1 ISSUE 6 (unnecessary exports in `audit_receipts.go`):** unexported `recoverAuditReceiptIntent`, `executeManagedModeChange`, `drainAuditReceipts`, and `auditReceiptOutboxHealth` and updated callers and doc anchors.
- **Run 1 ISSUE 7 (shared dirty `internal/component/plugin/types.go`):** isolated only the `HubAuditConfig` hunk into `tmp/fleet3-types.patch` for `hunks` recording and Commit A.

### Documentation Updates
- `docs/architecture/zefs-format.md`: documented `KeyFleetAuditManifest`, `KeyFleetAuditPage`, `KeyManagedAuditReceipts`, `KeyManagedAuditIntent`, `ValueMax` bounds, and `IsFleetAuditProtectedKey` live-mutation refusal (`<!-- source: pkg/zefs/keys.go ... -->`, `<!-- source: internal/component/config/storage/cli/data_rpc.go ... -->`, `<!-- source: internal/component/plugin/server/audit.go ... -->`, `<!-- source: internal/component/managed/audit_receipts.go ... -->`).
- `docs/architecture/fleet-config.md`: added `Fleet audit trail` section (observation points, async writer/pages/retention, offline receipt outbox & write intent, doctor codes, authorized query contract), `fleet-audit-receipts` wire verb row, doctor table rows, and `bus/hub/audit/max-entries` config rows with source anchors and test anchor `<!-- test: test/managed/fleet-audit-log.ci -->`.
- `docs/guide/audit.md`: added `## Fleet Audit Trail` operator guide covering event types, `show fleet audit` pipes, per-device authorization, retention config, offline receipts, and doctor diagnostics with source anchors.
- `docs/guide/command-reference.md`: added `show fleet audit` row anchored to `internal/component/managed/yang/ze-managed-cmd.yang -- container audit`.
- `docs/architecture/api/commands.md`: documented `ze-managed:show-fleet-audit` and `show fleet audit` pipe filters anchored to `internal/component/managed/cmd_audit.go -- registerAuditCommands, handleFleetAudit`.
- `ai/INDEX.md`: added keyword row for `fleet audit, show fleet audit, ManagedAuditStore, bus hub audit, fleet-audit-receipts` and all four audit doctor codes.
- `./le doc ste check`: `ste check: OK -- no habit grew in 5 changed document(s)`.
- `./le doc check all`: 0 findings across all fleet-3 docs and source anchors (3 pre-existing unrelated findings in `catalog.go`, `ze-policy-conf`, and `flow-export-1-counter-export.md`).

### Deviations from Plan
- **Reload helper naming:** added `PrepareManagedListenersWithHistoryAlarmsAndAudit` beside `PrepareManagedListenersWithHistoryAndAlarms` in `internal/component/plugin/server/managed_apply.go` so existing history and alarm unit tests stay intact while `cmd/ze/hub/managed_server.go` passes `cfg.Audit`.
- **Extra test and fixture files:** added dedicated test files (`audit_wiring_test.go`, `audit_receipts_test.go`, `cmd_audit_test.go`, `hub_audit_config_test.go`) and the compiled functional fixture (`misc_fixture_managed_fleet_audit.go`, `register_managed_fleet_audit.go`) to keep tests focused and match the managed suite's fixture pattern.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | First implementation pass omitted `ze-managed:show-fleet-audit` from `internal/component/plugin/all/testdata/wire-methods.snapshot` and omitted pre-decode byte caps on `fleet-audit-receipts` request/ack payloads | New wire methods require updating the hand-maintained golden snapshot, and every network JSON decoder requires an explicit byte cap before `json.NewDecoder` | Closure Run 1 independent review (`aed4514b`) | Updated `wire-methods.snapshot`, added `FleetAuditReceiptBatchBytesMax` / `FleetAuditReceiptAckBytesMax` pre-decode checks with regression tests, and added hub doctor unit tests and discovery docs |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Append-only hub audit log in live ZeFS store keyed by hub commit sequence | Done | `internal/component/plugin/server/audit.go` (`ManagedAuditStore`, `commitMutation`, `persistAuditGeneration`) | Immutable 128-row pages + atomic manifest under `FleetAuditLockKey` |
| Structured JSON entries with sequence, UTC event/receipt times, type, device, actor, surface, outcome, safe metadata | Done | `internal/component/plugin/server/audit.go` (`FleetAuditEntry`, `sanitizeAuditEntry`) | No raw config text, secrets, or free-form peer errors |
| Configurable retention (default 10,000; range 100..100,000) with oldest pruned on overflow and live reload | Done | `internal/component/plugin/yang/ze-plugin-conf.yang`, `internal/component/plugin/types.go`, `internal/component/config/loader_extract.go`, `internal/component/plugin/server/managed_apply.go` | `bus hub audit max-entries` validated and applied on start and reload |
| Non-blocking async hub writer that never stalls config delivery | Done | `internal/component/plugin/server/audit.go` (`Append`, `runWriter`, `Stop`) | Bounded 1,024-entry queue; drops and write errors surfaced in doctor |
| Live fleet event hooks (connect, disconnect, fetch, push, ack, reject, held, adopt, revert) | Done | `internal/component/plugin/server/managed_registry.go`, `managed_serve.go`, `managed_held.go`, `internal/component/managed/cmd_divergence.go`, `cmd/ze/hub/main.go` | Holding vs ordinary reject distinguished; safe enum diagnostics only |
| Offline device `fleet disable`/`enable` durable outbox, write intent, bounded replay, and dedup | Done | `pkg/fleet/envelope.go`, `internal/component/managed/audit_receipts.go`, `cmd_fleet.go`, `client.go`, `cmd/ze/hub/managed.go`, `internal/component/plugin/server/managed_serve.go` (`handleFleetAuditReceipts`) | Max 1,024 outbox, 32-entry batches, epoch+seq dedup across lost acks and pruning |
| Shared dispatcher query `show fleet audit` with `device`, `type`, `since`, `until` pipes and per-device RBAC | Done | `internal/component/managed/yang/ze-managed-cmd.yang`, `internal/component/managed/cmd_audit.go`, `internal/component/plugin/server/audit.go` (`Query`, `auditEntryAuthorized`) | Per-device `FleetDeviceScope` cached per query before filters |
| Live-store mutation refusal for audit & receipt keys | Done | `pkg/zefs/keys.go` (`IsFleetAuditProtectedKey`), `internal/component/config/storage/cli/data_rpc.go` (`handleDataKeyDelete`, `handleDataKeyUpdate`) | Refuses direct `update data key` and `delete data key` while daemon runs |
| Doctor diagnostics for write failures, drops, corruption, and receipt gaps | Done | `internal/component/plugin/server/managed_doctor.go`, `doctor_codes.go`, `internal/component/plugin/doctor/check_managed_runtime.go`, `register_external.go`, `internal/component/managed/doctor.go`, `register_doctor_codes.go` | 3 hub runtime checks + 1 device post-config check registered |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `ManagedServer.admitOnListener` (`managed_registry.go`); `TestManagedServerAuditWiringEndToEnd`, `test/managed/fleet-audit-log.ci` | Records `device.connect` with `ConnectedSince` and source address metadata |
| AC-2 | Done | `ManagedServer.disconnectDevice` (`managed_registry.go`); `TestManagedServerAuditWiringEndToEnd`, `test/managed/fleet-audit-log.ci` | Records `device.disconnect` at disconnect time |
| AC-3 | Done | `ManagedServer.handleRequest` (`managed_serve.go`); `TestManagedServerAuditWiringEndToEnd`, `test/managed/fleet-audit-log.ci` | Records `config.fetch` with version, previous-version, and mode; current fetch emits none |
| AC-4 | Done | `ManagedServer.handleConfigAck` (`managed_serve.go`); `TestManagedServerAuditWiringEndToEnd`, `test/managed/fleet-audit-log.ci` | Records `config.ack` with `outcome=success` and version |
| AC-5 | Done | `ManagedServer.recordConfigRejectAudit` (`managed_serve.go`); `TestManagedServerAuditWiringEndToEnd` | Records `config.reject` with safe enum `DiagnosticCode` and `Summary`, never raw `ack.Error` |
| AC-6 | Done | `ManagedServer.pushConfigChanged` (`managed_serve.go`); `TestManagedServerAuditWiringEndToEnd`, `test/managed/fleet-audit-log.ci` | Records `config.push` when `config-changed` succeeds |
| AC-7 | Done | `handleFleetAudit` (`cmd_audit.go`), `ManagedAuditStore.Query` (`audit.go`); `TestShowFleetAuditQueryFiltersAndGenericPipes`, `test/managed/fleet-audit-log.ci` | Returns authorized rows newest first (`seq` descending) |
| AC-8 | Done | `parseAuditPipeQuery` (`cmd_audit.go`), `auditEntryMatchesQuery` (`audit.go`); `TestAuditLogFilterDevice`, `TestShowFleetAuditQueryFiltersAndGenericPipes`, `test/managed/fleet-audit-log.ci` | Filters exact device after per-device RBAC |
| AC-9 | Done | `parseAuditPipeQuery` (`cmd_audit.go`), `auditEntryMatchesQuery` (`audit.go`); `TestAuditLogFilterType`, `TestShowFleetAuditQueryFiltersAndGenericPipes`, `test/managed/fleet-audit-log.ci` | Filters exact dot-separated kebab-case type |
| AC-10 | Done | `parseAuditInstant` (`cmd_audit.go`), `auditEntryMatchesQuery` (`audit.go`); `TestAuditLogFilterSince`, `TestShowFleetAuditQueryFiltersAndGenericPipes`, `test/managed/fleet-audit-log.ci` | Inclusive RFC 3339 lower (`since`) and upper (`until`) bounds |
| AC-11 | Done | `buildNextAuditPages` (`audit.go`), `SetMaxEntries`, `extractHubAuditConfig` (`loader_extract.go`); `TestAuditLogPrune`, `TestExtractHubConfigAudit`, `TestManagedServerAuditReloadRetentionAndDoctorHealth` | Default 10,000, configurable 100..100,000, oldest pruned on publication |
| AC-12 | Done | `ManagedAuditStore.LoadFromStore` & `Stop` (`audit.go`), `ManagedServer.Start` & `Stop` (`managed_serve.go`); `TestAuditLogPersist`, `TestAuditLogDiskTreeAndBlobSequentialBatches`, `test/managed/fleet-audit-log.ci` | Preserves pages, cursors, and monotonic sequence across hub restart |
| AC-13 | Done | `ManagedAuditStore.Append` & `publishAsyncBatch` (`audit.go`), `checkManagedAuditWrite` (`check_managed_runtime.go`); `TestAuditLogAsyncNonBlocking`, `TestAuditLogWriteFailureRecovery`, `TestCheckManagedAuditWrite` | Non-blocking enqueue; failed writes preserve prior generation and surface in doctor |
| AC-14 | Done | `executeManagedModeChange`, `recoverAuditReceiptIntent`, `drainAuditReceipts` (`audit_receipts.go`), `handleFleetAuditReceipts` (`managed_serve.go`), `IngestReceipts` (`audit.go`); `audit_receipts_test.go`, `TestAuditLogIngestReceipts`, `test/managed/fleet-audit-log.ci` | Offline disable/enable recorded via write intent, replayed on reconnect, deduplicated across lost ack |
| AC-15 | Done | `handleConfigHeld` (`managed_held.go`), `handleDivergenceAdopt` & `handleDivergenceRevert` (`cmd_divergence.go`), `cmd/ze/hub/main.go`; `TestDivergenceAdoptAndRevertEmitAudit`, `TestManagedServerAuditWiringEndToEnd` | Separate `config.held`, `divergence.adopt`, and `divergence.revert` rows with actor, winner, and version |
| AC-16 | Done | `ValidFleetAuditEventType`, `auditEntryAuthorized`, `Query` (`audit.go`), `handleFleetAudit` (`cmd_audit.go`); `TestAuditLogPerDeviceAuthorizationAndCaching`, `TestShowFleetAuditPerDeviceAuthorization` | Producer-agnostic types; no secrets/config text; per-device RBAC cached before filters |
| AC-17 | Done | `commitMutation`, `LoadFromStore`, `Health` (`audit.go`), `check_managed_runtime.go`, `doctor.go`, `data_rpc.go`; `audit_test.go`, `audit_wiring_test.go`, `check_managed_runtime_test.go`, `data_rpc_test.go`, `data_rpc_update_test.go` | Equal-time ordering, overflow/corrupt/write-failure diagnostics, retention reduction, protected keys |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestAuditLogAppend` | Done | `internal/component/plugin/server/audit_test.go` | Plus 12 additional unit tests in `audit_test.go` and 3 wiring/regression tests in `audit_wiring_test.go` |
| `TestAuditLogPersist` | Done | `internal/component/plugin/server/audit_test.go` | Also disk tree + blob sequential batches in `TestAuditLogDiskTreeAndBlobSequentialBatches` |
| `TestAuditLogFilterDevice` | Done | `internal/component/plugin/server/audit_test.go` | Also CLI pipe tests in `internal/component/managed/cmd_audit_test.go` |
| `TestAuditLogFilterType` | Done | `internal/component/plugin/server/audit_test.go` | Also CLI pipe validation in `TestShowFleetAuditPipeValidationRefusals` |
| `TestAuditLogFilterSince` | Done | `internal/component/plugin/server/audit_test.go` | Covers inclusive `since` and `until` bounds |
| `TestAuditLogPrune` | Done | `internal/component/plugin/server/audit_test.go` | Also live reload pruning in `TestManagedServerAuditReloadRetentionAndDoctorHealth` |
| `TestAuditLogAsyncNonBlocking` | Done | `internal/component/plugin/server/audit_test.go` | Verifies queue saturation drops without blocking caller |
| `fleet-audit-log` | Done | `test/managed/fleet-audit-log.ci` | Driven by `internal/test/fixture/misc_fixture_managed_fleet_audit.go` |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/plugin/server/managed_serve.go` | Done | Session fetch/push/ack/reject hooks, `handleFleetAuditReceipts`, `ManagedAuditQuery`, `RecordAudit`, `Start`/`Stop` |
| `internal/component/plugin/server/managed_registry.go` | Done | `device.connect` and `device.disconnect` hooks |
| `internal/component/plugin/server/managed_held.go` | Done | `config.held` hook |
| `internal/component/plugin/server/managed_apply.go` | Done | `PrepareManagedListenersWithHistoryAlarmsAndAudit` and live `SetMaxEntries` apply |
| `internal/component/plugin/server/managed_doctor.go` | Done | `ManagedDoctorRecord.Audit` and `appendAuditHealth` |
| `internal/component/plugin/server/doctor_codes.go` | Done | Registered 3 hub audit doctor codes |
| `internal/component/plugin/doctor/check_managed_runtime.go` | Done | `checkManagedAuditWrite`, `checkManagedAuditDropped`, `checkManagedAuditCorrupt` |
| `internal/component/plugin/doctor/check_managed_runtime_test.go` | Done | `TestCheckManagedAuditWrite`, `TestCheckManagedAuditDropped`, `TestCheckManagedAuditCorrupt` |
| `internal/component/plugin/doctor/register_external.go` | Done | Registered 3 runtime doctor checks (orders 724–726) |
| `internal/component/managed/cmd_divergence.go` | Done | `divergence.adopt` and `divergence.revert` audit emission |
| `internal/component/managed/cmd_fleet.go` | Done | `executeManagedModeChange` wrapping daemon and stopped disable/enable |
| `internal/component/managed/client.go` | Done | `ReceiptStore` and `drainAuditReceipts` in `runConnection` |
| `internal/component/managed/doctor.go` | Done | `managedAuditReceiptsDoctorCheck` and `checkManagedAuditReceipts` |
| `internal/component/managed/register.go` | Done | `registerAuditCommands` and `managedAuditReceiptsDoctorCheck` registration |
| `internal/component/managed/register_doctor_codes.go` | Done | `CodeDoctorManagedAuditReceiptGap` registration |
| `internal/component/managed/yang/ze-managed-cmd.yang` | Done | `show fleet audit` (`ze-managed:show-fleet-audit`) |
| `internal/component/plugin/all/testdata/wire-methods.snapshot` | Done | Added `ze-managed:show-fleet-audit` |
| `internal/component/plugin/yang/ze-plugin-conf.yang` | Done | `bus/hub/audit/max-entries` (`100..100000`, default `10000`) |
| `internal/component/config/loader_extract.go` | Done | `extractHubAuditConfig` + `ExtractHubConfig` wiring |
| `internal/component/config/hub_audit_config_test.go` | Done | `TestExtractHubConfigAudit` |
| `internal/component/plugin/types.go` | Done | `HubAuditConfig` hunk isolated via `tmp/fleet3-types.patch` |
| `cmd/ze/hub/managed_server.go`, `cmd/ze/hub/main.go`, `cmd/ze/hub/managed.go` | Done | Wired `Audit`, `RecordAudit`, and `ReceiptStore` |
| `pkg/fleet/envelope.go`, `pkg/fleet/envelope_test.go` | Done | `VerbFleetAuditReceipts`, batch/entry/ack types, bounds, validation, round-trip tests |
| `internal/component/config/storage/cli/data_rpc.go`, `data_rpc_test.go`, `data_rpc_update_test.go` | Done | Refuses live delete/update of protected fleet audit & receipt keys |
| `pkg/zefs/keys.go`, `pkg/zefs/keys_test.go` | Done | 4 registered key families + `IsFleetAuditProtectedKey` |
| `internal/component/plugin/server/audit.go`, `audit_test.go`, `audit_wiring_test.go` | Done | Created hub audit store, writer, query, unit & wiring tests |
| `internal/component/managed/cmd_audit.go`, `cmd_audit_test.go` | Done | Created `show fleet audit` command, pipes, RBAC tests |
| `internal/component/managed/audit_receipts.go`, `audit_receipts_test.go` | Done | Created device outbox, recoverable write intent, drain, tests |
| `test/managed/fleet-audit-log.ci`, `internal/test/fixture/misc_fixture_managed_fleet_audit.go`, `register_managed_fleet_audit.go` | Done | Created end-to-end functional test and compiled fixture |
| `docs/architecture/zefs-format.md`, `docs/architecture/fleet-config.md`, `docs/guide/audit.md`, `docs/guide/command-reference.md`, `docs/architecture/api/commands.md`, `ai/INDEX.md` | Done | Updated architecture, guide, command reference, and index with source anchors |
| `website/changes/fragments/fleet-3-audit-trail.md` | Done | Operator-facing change fragment |

### Audit Summary
- **Total items:** 17 ACs, 8 TDD plan tests (+ 25 extra unit/wiring/regression tests), 48 files (including 7 repointed citer specs and `types.go` hunk)
- **Done:** 17/17 ACs, 8/8 TDD plan tests, all planned files
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 2 (recorded in Deviations from Plan)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Record every significant fleet operation centrally on the hub with monotonic sequence, UTC timestamps, actor, device, outcome, and safe metadata (no secrets/config text) | Unit + Wiring + Functional | `TestManagedServerAuditWiringEndToEnd` (`audit_wiring_test.go`), `TestDivergenceAdoptAndRevertEmitAudit` (`audit_receipts_test.go`), `test/managed/fleet-audit-log.ci` (`misc_fixture_managed_fleet_audit.go`) |
| Persist append-only bounded history in the hub's ZeFS store surviving restarts and retention pruning without blocking config delivery | Unit + Functional | `TestAuditLogPersist`, `TestAuditLogPrune`, `TestAuditLogAsyncNonBlocking`, `TestAuditLogWriteFailureRecovery`, `TestAuditLogCorruptRecoveryAndOrphanGC`, `TestAuditLogDiskTreeAndBlobSequentialBatches` (`audit_test.go`), `test/managed/fleet-audit-log.ci` |
| Capture offline device `fleet disable`/`enable` mode transitions with crash-safe write intent and deduplicated reconnect delivery | Unit + Wiring + Functional | `TestAuditReceiptsRecoverableIntentCrashBeforeAndAfterFlagWrite`, `TestAuditReceiptsOfflineDisableEnableLostAckDedup`, `TestAuditReceiptsOverflowGapDoctorAndHubReport`, `TestAuditReceiptsUnknownMethodBackwardCompatible` (`audit_receipts_test.go`), `TestAuditLogIngestReceipts` (`audit_test.go`), `test/managed/fleet-audit-log.ci` |
| Query audit history through the shared dispatcher (`show fleet audit`) with `device`, `type`, `since`, `until` pipes and per-device RBAC | Unit + Functional | `TestFleetAuditCommandRegistrationAndCompletion`, `TestShowFleetAuditQueryFiltersAndGenericPipes`, `TestShowFleetAuditPerDeviceAuthorization` (`cmd_audit_test.go`), `test/managed/fleet-audit-log.ci` |
| Protect live audit and receipt keys from direct daemon mutation and surface audit health in `ze doctor` | Unit | `TestDeleteKeyRefusesFleetAuditProtectedKeys` (`data_rpc_test.go`), `TestUpdateKeyRefusesFleetAuditProtectedKeys` (`data_rpc_update_test.go`), `TestCheckManagedAuditWrite`, `TestCheckManagedAuditDropped`, `TestCheckManagedAuditCorrupt` (`check_managed_runtime_test.go`) |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| Template change (`template.change`) producer hook | Config templates do not exist yet; this spec provides the producer-agnostic `ManagedAuditStore.Append` + `ValidFleetAuditEventType` contract that `fleet-2` calls when committing template changes | `plan/post-release/features/spec-fleet-2-config-templates.md` |
| Staged rollout producer hooks | Staged rollouts do not exist yet; `fleet-5` calls the producer-agnostic audit contract | `plan/post-release/features/spec-fleet-5-staged-rollout.md` |
| Web UI audit page and export table | Task explicitly assigns web audit views and export to `fleet-ui-6`, consuming the authorized `show fleet audit` contract built here | `plan/post-release/features/spec-fleet-ui-6-config-rollout-audit-admin.md` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fleet-3-audit-trail-0cfcb484-f28a-4e4e-b1d1-85693900af06.md` |
| `./le spec review check` | clean |
| Rounds | 2 (Run 1 `aed4514b` found 2 BLOCKERs + 5 ISSUEs; all fixed; Run 2 `9e5357b5` independent reviewer verified CLEAN: 0 BLOCKER, 0 ISSUE) |
| Reviewer lenses used | logic + wiring, security + edge-cases + bounds, config/YANG/snapshot surface, doctor/docs/anchors, Ze Go style pass (7 questions) |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | New wire method `ze-managed:show-fleet-audit` missing from golden snapshot | `internal/component/plugin/all/testdata/wire-methods.snapshot` | Added `ze-managed:show-fleet-audit` in sorted order; `TestRegisteredWireMethods` passes |
| 2 | BLOCKER | `handleFleetAuditReceipts` and `decodeAuditReceiptAck` lacked pre-decode raw byte bounds before `json.NewDecoder` | `pkg/fleet/envelope.go`, `internal/component/plugin/server/managed_serve.go`, `internal/component/managed/audit_receipts.go` | Added `FleetAuditReceiptBatchBytesMax` (64 KiB) and `FleetAuditReceiptAckBytesMax` (4 KiB) pre-decode checks + `TestManagedServerAuditReceiptsRejectsOversizedPayload` and `TestAuditReceiptAckRejectsOversizedPayload` |
| 3 | ISSUE | `extractHubAuditConfig` used 32-bit `strconv.Atoi` violating `numberparse-allowlist` | `internal/component/config/loader_extract.go` | Switched to 64-bit `strconv.ParseInt` with `[math.MinInt32, math.MaxInt32]` clamp; `./le repo check` allowlist clean |
| 4 | ISSUE | New hub runtime audit doctor checks lacked dedicated unit tests | `internal/component/plugin/doctor/check_managed_runtime_test.go` | Added `TestCheckManagedAuditWrite`, `TestCheckManagedAuditDropped`, `TestCheckManagedAuditCorrupt` |
| 5 | ISSUE | `show fleet audit` missing from discovery surfaces | `ai/INDEX.md`, `docs/guide/command-reference.md`, `docs/architecture/api/commands.md` | Added entries and source anchors across all three discovery surfaces (`./le doc ste check` OK, `./le doc check all` 0 fleet-3 findings) |
| 6 | ISSUE | Four package-internal helpers in `audit_receipts.go` were exported with no cross-package non-test caller | `internal/component/managed/audit_receipts.go` | Unexported `recoverAuditReceiptIntent`, `executeManagedModeChange`, `drainAuditReceipts`, `auditReceiptOutboxHealth` and updated callers + doc anchors |
| 7 | ISSUE | `internal/component/plugin/types.go` carried a concurrent session's unrelated line-963 comment hunk | `internal/component/plugin/types.go`, `tmp/fleet3-types.patch` | Isolated only the `HubAuditConfig` hunk into `tmp/fleet3-types.patch` for `hunks` recording and Commit A |

### Run 2 (final)
- Independent fresh reviewer (`9e5357b5`) verified all 7 Run 1 fixes against source, audited AC-1..AC-17, security/bounds/concurrency, `Design documents`, docs/anchors, and all 7 `ze-go-style.md` questions (`./le go style check`: `go style: OK`; `./le doc ste check`: `OK`; `./le test weakened check`: 0 fleet-3 findings; `./le spec citation`: 0 fleet-3 citations).
- Verdict: **CLEAN** (`0 BLOCKER, 0 ISSUE, 0 NOTE`).

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/plugin/server/audit.go` | Yes | `-rw-r--r-- 1 thomas thomas 48093 Nov 28 02:34 internal/component/plugin/server/audit.go` |
| `internal/component/plugin/server/audit_test.go` | Yes | `-rw-r--r-- 1 thomas thomas 32142 Nov 28 02:35 internal/component/plugin/server/audit_test.go` |
| `internal/component/plugin/server/audit_wiring_test.go` | Yes | `-rw-r--r-- 1 thomas thomas 13908 Nov 28 02:35 internal/component/plugin/server/audit_wiring_test.go` |
| `internal/component/managed/cmd_audit.go` | Yes | `-rw-r--r-- 1 thomas thomas 9050 Nov 28 01:23 internal/component/managed/cmd_audit.go` |
| `internal/component/managed/cmd_audit_test.go` | Yes | `-rw-r--r-- 1 thomas thomas 16582 Nov 28 01:23 internal/component/managed/cmd_audit_test.go` |
| `internal/component/managed/audit_receipts.go` | Yes | `-rw-r--r-- 1 thomas thomas 22571 Nov 28 02:37 internal/component/managed/audit_receipts.go` |
| `internal/component/managed/audit_receipts_test.go` | Yes | `-rw-r--r-- 1 thomas thomas 18055 Nov 28 02:38 internal/component/managed/audit_receipts_test.go` |
| `internal/component/config/hub_audit_config_test.go` | Yes | `-rw-r--r-- 1 thomas thomas 3613 Nov 28 00:54 internal/component/config/hub_audit_config_test.go` |
| `internal/test/fixture/misc_fixture_managed_fleet_audit.go` | Yes | `-rw-r--r-- 1 thomas thomas 15791 Nov 28 02:37 internal/test/fixture/misc_fixture_managed_fleet_audit.go` |
| `internal/test/fixture/register_managed_fleet_audit.go` | Yes | `-rw-r--r-- 1 thomas thomas 294 Nov 28 01:23 internal/test/fixture/register_managed_fleet_audit.go` |
| `test/managed/fleet-audit-log.ci` | Yes | `-rw-r--r-- 1 thomas thomas 1461 Nov 28 01:23 test/managed/fleet-audit-log.ci` |
| `website/changes/fragments/fleet-3-audit-trail.md` | Yes | `-rw-r--r-- 1 thomas thomas 2407 Nov 28 01:24 website/changes/fragments/fleet-3-audit-trail.md` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | `device.connect` recorded on authenticated connect | `internal/component/plugin/server/managed_registry.go` (`admitOnListener` -> `FleetAuditDeviceConnect`); `TestManagedServerAuditWiringEndToEnd`; `test/managed/fleet-audit-log.ci` |
| AC-2 | `device.disconnect` recorded on disconnect | `internal/component/plugin/server/managed_registry.go` (`disconnectDevice` -> `FleetAuditDeviceDisconnect`); `TestManagedServerAuditWiringEndToEnd`; `test/managed/fleet-audit-log.ci` |
| AC-3 | `config.fetch` recorded on served config | `internal/component/plugin/server/managed_serve.go` (`handleRequest` -> `FleetAuditConfigFetch`); `TestManagedServerAuditWiringEndToEnd`; `test/managed/fleet-audit-log.ci` |
| AC-4 | `config.ack` recorded on ok ack | `internal/component/plugin/server/managed_serve.go` (`handleConfigAck` -> `FleetAuditConfigAck`); `TestManagedServerAuditWiringEndToEnd`; `test/managed/fleet-audit-log.ci` |
| AC-5 | `config.reject` recorded with safe diagnostic | `internal/component/plugin/server/managed_serve.go` (`recordConfigRejectAudit` -> `safeRejectDiagnosticCode`/`safeRejectSummary`, never `ack.Error`); `TestManagedServerAuditWiringEndToEnd` |
| AC-6 | `config.push` recorded on `config-changed` | `internal/component/plugin/server/managed_serve.go` (`pushConfigChanged` -> `FleetAuditConfigPush`); `TestManagedServerAuditWiringEndToEnd`; `test/managed/fleet-audit-log.ci` |
| AC-7 | `show fleet audit` newest first, authorized only | `internal/component/managed/cmd_audit.go` (`handleFleetAudit`), `internal/component/plugin/server/audit.go` (`Query`); `TestShowFleetAuditQueryFiltersAndGenericPipes`; `test/managed/fleet-audit-log.ci` |
| AC-8 | `show fleet audit \| device <name>` | `internal/component/managed/cmd_audit.go` (`parseAuditPipeQuery`), `audit.go` (`auditEntryMatchesQuery`); `TestAuditLogFilterDevice`; `test/managed/fleet-audit-log.ci` |
| AC-9 | `show fleet audit \| type <event-type>` | `internal/component/managed/cmd_audit.go` (`parseAuditPipeQuery`), `audit.go` (`auditEntryMatchesQuery`); `TestAuditLogFilterType`; `test/managed/fleet-audit-log.ci` |
| AC-10 | `show fleet audit \| since <RFC3339>` (and `until`) | `internal/component/managed/cmd_audit.go` (`parseAuditInstant`), `audit.go` (`auditEntryMatchesQuery`); `TestAuditLogFilterSince`; `test/managed/fleet-audit-log.ci` |
| AC-11 | Max entries (default 10,000, configurable 100..100,000) prunes oldest | `internal/component/plugin/server/audit.go` (`buildNextAuditPages`), `loader_extract.go` (`extractHubAuditConfig`), `managed_apply.go` (`SetMaxEntries`); `TestAuditLogPrune`, `TestExtractHubConfigAudit`, `TestManagedServerAuditReloadRetentionAndDoctorHealth` |
| AC-12 | Hub restart preserves audit log and monotonic seq | `internal/component/plugin/server/audit.go` (`LoadFromStore`, `Stop`), `managed_serve.go` (`Start`, `Stop`); `TestAuditLogPersist`, `TestAuditLogDiskTreeAndBlobSequentialBatches`, `test/managed/fleet-audit-log.ci` |
| AC-13 | Storage failure never blocks config-fetch; exposed in doctor | `internal/component/plugin/server/audit.go` (`Append`, `publishAsyncBatch`), `check_managed_runtime.go` (`checkManagedAuditWrite`); `TestAuditLogAsyncNonBlocking`, `TestAuditLogWriteFailureRecovery`, `TestCheckManagedAuditWrite` |
| AC-14 | Offline disable/enable receipts, write intent, reconnect dedup | `internal/component/managed/audit_receipts.go` (`executeManagedModeChange`, `recoverAuditReceiptIntent`, `drainAuditReceipts`), `managed_serve.go` (`handleFleetAuditReceipts`), `audit.go` (`IngestReceipts`); `audit_receipts_test.go`, `TestAuditLogIngestReceipts`, `test/managed/fleet-audit-log.ci` |
| AC-15 | Held copy, adopt, and revert recorded distinctly with actor/winner/version | `internal/component/plugin/server/managed_held.go` (`handleConfigHeld`), `internal/component/managed/cmd_divergence.go` (`handleDivergenceAdopt`, `handleDivergenceRevert`); `TestDivergenceAdoptAndRevertEmitAudit`, `TestManagedServerAuditWiringEndToEnd` |
| AC-16 | Producer-agnostic types; secret-free; per-device RBAC before filters | `internal/component/plugin/server/audit.go` (`ValidFleetAuditEventType`, `auditEntryAuthorized`), `cmd_audit.go` (`handleFleetAudit`); `TestAuditLogPerDeviceAuthorizationAndCaching`, `TestShowFleetAuditPerDeviceAuthorization` |
| AC-17 | Equal-time ordering, queue overflow/corrupt/write-fail diagnostics, retention shrink, key protection | `audit.go`, `check_managed_runtime.go`, `doctor.go`, `data_rpc.go`; `TestAuditLogCorruptRecoveryAndOrphanGC`, `TestCheckManagedAuditDropped`, `TestCheckManagedAuditCorrupt`, `TestDeleteKeyRefusesFleetAuditProtectedKeys`, `TestUpdateKeyRefusesFleetAuditProtectedKeys` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Device TLS connect -> `device.connect` -> `show fleet audit` over SSH CLI | `test/managed/fleet-audit-log.ci` | Yes (read `test/managed/fleet-audit-log.ci` and `internal/test/fixture/misc_fixture_managed_fleet_audit.go`: verifies connect, fetch, ack, push, deduplicated receipts, disconnect, pipes, and hub restart survival) |
| Hub write-observer config import -> `config.push` + `config.fetch` -> `show fleet audit` | `test/managed/fleet-audit-log.ci` | Yes (`s.hubImport` -> `verifyPushAndReceipts` verifies `config.push` version and newest-first sequence) |
| `fleet-audit-receipts` replay + hub restart on disk ZeFS -> `show fleet audit \| device/type/since` | `test/managed/fleet-audit-log.ci` | Yes (`sendAndReplayAuditReceipts` sends batch twice confirming `count == 1`, `actor == "device:edge-01"`, then `s.restartHub` confirms exact row preservation and `Seq > beforeMaxSeq` on reconnect) |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| none | confirmed | Spec declared no separate `A-N` Risks & Assumptions table; all design constraints in `Required Reading`, `Existing Mechanisms`, and `Design documents` were verified against live source (`ManagedAuditStore` non-blocking async queue, ZeFS immutable pages + atomic manifest, `FleetDeviceScope` per-device RBAC, pre-decode wire byte bounds, and live-store mutation refusal). |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| ZeFS key registry & protected live mutation (`docs/architecture/zefs-format.md`) | `pkg/zefs/keys.go` (`KeyFleetAuditManifest`, `KeyFleetAuditPage`, `KeyManagedAuditReceipts`, `KeyManagedAuditIntent`, `IsFleetAuditProtectedKey`), `internal/component/config/storage/cli/data_rpc.go` (`handleDataKeyDelete`, `handleDataKeyUpdate`) | Yes (`./le doc check all`: 0 fleet-3 anchor issues; `./le doc ste check`: OK) |
| Hub audit architecture, wire verb, doctor codes, config leaves (`docs/architecture/fleet-config.md`) | `internal/component/plugin/server/audit.go`, `managed_serve.go`, `managed_apply.go`, `managed_doctor.go`, `doctor_codes.go`, `internal/component/managed/audit_receipts.go`, `cmd_audit.go`, `pkg/fleet/envelope.go`, `ze-plugin-conf.yang` | Yes (`./le doc check all`: 0 fleet-3 anchor issues; `./le doc ste check`: OK) |
| Operator audit guide (`docs/guide/audit.md`) | `internal/component/managed/cmd_audit.go`, `ze-managed-cmd.yang`, `audit.go`, `audit_receipts.go`, `check_managed_runtime.go`, `doctor.go` | Yes (`./le doc check all`: 0 fleet-3 anchor issues; `./le doc ste check`: OK) |
| CLI & API command reference (`docs/guide/command-reference.md`, `docs/architecture/api/commands.md`, `ai/INDEX.md`) | `internal/component/managed/yang/ze-managed-cmd.yang` (`container audit`), `internal/component/managed/cmd_audit.go` (`registerAuditCommands`, `handleFleetAudit`) | Yes (`./le doc check all`: 0 fleet-3 anchor issues; `./le doc ste check`: OK) |
| News fragment (`website/changes/fragments/fleet-3-audit-trail.md`) | Verified front matter (`spec: fleet-3-audit-trail`, `topic: Fleet`, `feature: fleet-management`) and six fixed headings (`Before`, `After`, `Corrects`, `Exposure`, `Upstream`, `Asked by`) | Yes |
