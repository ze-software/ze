# Spec: dropped-error-1-bgp-engine

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | protocol |
| Priority | P1 |
| Release reason | 48 errors in the BGP engine (reactor, `ze bgp decode --plugin`, the BGP server) are caught and logged only below WARN (9 of the original 57 were already resolved at HEAD): a next hop that fails to resolve drops a static route, a failed collision handoff or teardown write leaves no trace at the default level |
| RFC gaps | - |
| Depends | `plan/release/validation/spec-doctor-coverage-6-bgp-engine.md`, its Data Flow "Class to code map": every answer-2 `Warn`/`Error` this spec adds is a problem site judged by `./le arch doctor-coverage check` (`docs/architecture/doctor-and-health-checks.md`, "Every problem site names a doctor code"), so each added `Warn`/`Error` MUST emit a `code` attribute registered in its owning package's doctor-code registration (`internal/component/bgp/reactor/doctor_codes.go` and `register_diagnostic.go`, or `internal/component/bgp/server/register_doctor_codes.go` if `server/events.go` adds a `Warn`) and declared by a doctor check or `diagnostic.EventCode`; where `spec-doctor-coverage-6-bgp-engine.md` has not yet registered the code, the `Warn` lands with its registration here |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Child of `plan/release/fixes/spec-dropped-error-0-umbrella.md`, which holds the
resolution policy and the tree-wide acceptance. Door: `ai/doors/bgp-engine.md`.

Population: every `error-dropped-below-warn` finding of
`./le arch refuse-by-default report` under `internal/component/bgp/` outside
`internal/component/bgp/plugins/`, and under `internal/core/bgp/`. At HEAD on
2026-11-26: **48 instances in 18 files** (reactor 33 across 16 files,
`bgp/cli` 14 in 1 file, `bgp/server` 1 in 1 file; 1 site already annotated at
`aigp_readvertise.go:576`). Nine of the original 57 instances from 2026-11-02
(including three whole files: `aigp_readvertise.go`, `filter_ordered.go`, and
`reactor_notify.go`) are already resolved at HEAD and retired into
`## Existing Mechanisms`. Files to Modify lists each remaining file with its
verified functions and line numbers.

Each instance is resolved by exactly one answer, judged in this order
(umbrella, Key Design Decisions):

| Order | When | Answer |
|-------|------|--------|
| 1 | Going on is wrong: the caller would act on the failure | Return the error, or fix the behavior the drop hid |
| 2 | Going on is right and an operator can act | `Warn` with what, why and the fix (`ai/rules/user-facing-errors.md`) and a `code` attribute registered in the owning package's doctor-code registration (`internal/component/bgp/reactor/doctor_codes.go` + `register_diagnostic.go`, or `internal/component/bgp/server/register_doctor_codes.go`) naming the class code from `spec-doctor-coverage-6-bgp-engine`'s map (and raised on the report bus for transient problems); on a per-route or per-message path, count it in the reactor's existing metrics and `Warn` once per condition |
| 3 | Going on is right and nobody can act (a close at teardown, a write to a connection already lost) | `// refuse-by-default: guards <Symbol> -- <why>` on the line above |

Known overlaps, resolved where they are owned:

| Instance | Owner of the defect | What this child does |
|----------|---------------------|----------------------|
| `peer_static_wire.go` `sendStaticRoutes` (line 66), `sendStaticRoutesGrouped` (line 143) | `plan/release/fixes/spec-bgp-one-off-defects.md` owned the static-route send-error row (2026-10-31); at HEAD those send-error sites already classify `isRouteScopedSendError(err)` and are not in the report | The 2 remaining findings at lines 66 and 143 are `resolveNextHop` (`nhErr != nil`) drops, owned directly by this child and resolved under answer 2 |
| `bgp/cli/decode_plugin.go` (14) | spec-command-surface-parity, closed (unwired-feature row 2026-11-02, `--plugin` parsed and ignored) | That spec's change to `decode_plugin.go` landed in 442e4e810e; resolve the 14 remaining dropped errors: an operator ran this command, so failures are returned to them (answer 1) |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`
  → Constraint: `pkg/` is public API; no plugin RPC, SDK, or external wire contract changes here
- [ ] `ai/doors/bgp-engine.md`
  → Constraint: the engine holds no policy; a failure that changes a route decision belongs to the plugin that owns it, the engine reports it
  → Constraint: received bytes are borrowed; a `Warn` that formats a message body copies nothing out of the read buffer it does not already own
  → Constraint: every BGP package test runs with `-tags ze_bgp`
- [ ] `ai/digests/bgp-reactor.md` - peer lifecycle, session read loop, delivery to plugins
  → Constraint: a write error on a session is followed by its teardown; a `Warn` there must not duplicate the teardown's own report
- [ ] `docs/architecture/doctor-and-health-checks.md` - "Every problem site names a doctor code"
  → Constraint: `./le arch doctor-coverage check` refuses any added `Warn` or `Error` log line whose `code` attribute is missing, dynamic, or undeclared by a doctor check or `diagnostic.EventCode`
- [ ] `ai/rules/user-facing-errors.md`
  → Constraint: every `Warn` names the peer and the operation in operator terms, why it matters, the fix, and its registered doctor `code`; transient problems also raise on the report bus
- [ ] `docs/contributing/ze-go-style.md` - "A zero value is never an answer", "Error dropped below WARN"
  → Constraint: return, count, or `Warn`; the annotation names a guard visible in the file

**Key insights:**
- Most remaining reactor instances are per-peer lifecycle events (teardown, collision, BFD, listener, event-bus emit), where an answer-2 `Warn` or answer-3 guard annotation is right; the static-route and initial-sync loops are per route, where counting or rate-limiting avoids log floods.
- Nine of the original 57 sites from 2026-11-02 have already been resolved in HEAD (including all sites in `aigp_readvertise.go`, `filter_ordered.go`, and `reactor_notify.go`), leaving 48 active findings across 18 files.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/arch/refusebydefault/dropped.go` - the `error-dropped-below-warn` detector
- [ ] `internal/le/arch/doctorcoverage/doctorcoverage.go`, `codes.go` - the `doctor-coverage` commit gate
- [ ] `internal/component/bgp/cli/decode_plugin.go` - 14 dropped pipe/write/close/wait errors in `invokePluginDecodeRequest`, `invokePluginPath`, `invokePluginSubprocess`, `invokePluginInternal` logged at `Debug`
- [ ] `internal/component/bgp/reactor/listener.go`, `operation.go`, `peer.go`, `peer_admin.go`, `peer_bfd.go`, `peer_connection.go`, `peer_initial_sync.go`, `peer_static_wire.go`, `reactor.go`, `reactor_api_forward_batch.go`, `reactor_connection.go`, `reactor_iface.go`, `session.go`, `session_bfd_strict.go`, `session_connection.go`, `session_validation.go` - 33 remaining reactor sites logged at `Debug` across 16 files
- [ ] `internal/component/bgp/server/events.go` - `onEORReceived` line 1171 logs `eorEventBus.Emit` failure at `Debug`
- [ ] `internal/component/bgp/reactor/doctor_codes.go`, `register_diagnostic.go`, `pending_refusal_diagnostic.go` - existing reactor doctor-code and `EventCode` registrations
- [ ] `internal/component/bgp/reactor/aigp_readvertise.go`, `filter_ordered.go`, `reactor_notify.go`, `session_hold_expiry.go` - verified retired sites at HEAD

**Behavior to preserve:**
- Session behavior on the wire. No instance changes what is sent to a peer unless answer 1 finds the drop was a defect, which then owes its own test.
- Existing `ze bgp decode --plugin` decoded output for valid inputs (`capability-llgr`).

**Behavior to change:**
- Each of the 48 remaining instances resolved by one answer (return error, covered `Warn` with registered doctor code + report-bus raise for transient problems, or guard annotation).

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| refuse-by-default dropped-error shape | `internal/le/arch/refusebydefault/dropped.go` `droppedError` | Lists the 48 remaining active findings across 18 files at HEAD (2026-11-26) | Applies: the acceptance check (AC-1) |
| Guard annotation | `internal/le/arch/refusebydefault/annotation.go` | Accepts `// refuse-by-default: guards <Symbol> -- <why>` | Applies: answer 3 only |
| Reactor doctor codes and event-code registration | `internal/component/bgp/reactor/doctor_codes.go` (`doctorDiagnosticCodes`, `registerDoctorCodes`), `internal/component/bgp/reactor/register_diagnostic.go` (`init`, `diagnostic.RegisterEventCode`), `pending_refusal_diagnostic.go` (`reportPendingRefusalFailure`) | Registers `diagnostic.CodeMeta` and `diagnostic.EventCode` in `init()` so `recent-events` and `./le arch doctor-coverage check` cover reactor `Warn`/`Error` sites | Extends: every new reactor answer-2 `Warn` registers its code here (or reuses a code already registered here or by `spec-doctor-coverage-6-bgp-engine.md`) |
| Owning-package doctor code registration pattern | `internal/component/bgp/config/register_doctor_codes.go`, `internal/component/bgp/plugins/bmp/register_doctor_codes.go` | Registers package-owned `diagnostic.CodeMeta` in `register_doctor_codes.go` `init()` | Extends: if `internal/component/bgp/server/events.go:1171` becomes an answer-2 `Warn`, `internal/component/bgp/server/register_doctor_codes.go` follows this pattern |
| Reactor metrics | `internal/component/bgp/reactor` `prefixMetrics` and the reactor's Prometheus families (`docs/plugin-development/metrics.md`) | Per-peer counters | Extends: answer 2 on per-route or per-message paths counts here, no new family |
| Operator warnings and error bus | `internal/core/report` (`show warnings`, `show errors`, `docs/architecture/core-design.md` section 15) | Holds active warnings and recent error events rendered by `recent-events` | Applies: transient answer-2 failures raise under their registered doctor code |
| Subsystem logger | `routesLogger()`, `peerLogger()`, `sessionLogger()`, `reactorLogger()`, `cacheLogger()`, `server.logger()` (`slogutil.Logger`) | Per-subsystem levels | Applies: each answer-2 `Warn` uses the existing subsystem logger |
| Retired site: AIGP re-advertisement outcome | `internal/component/bgp/reactor/aigp_readvertise.go:575-579` (`Reactor.readvertiseAIGP`, formerly `replayAIGP` 511) | Already carries `// refuse-by-default: guards Reactor.readvertiseAIGP` and appears in `annotated` | Does not apply: already resolved at HEAD; file removed from Files to Modify |
| Retired site: Egress policy chain | `internal/component/bgp/reactor/filter_ordered.go:290-357` (`Reactor.runEgressPolicyChainASN4`, formerly line 393) | Uncompiled export chain logs `slog.Warn` and fails closed at line 354; `ingressChainWithdrawal` logs `fwdLogger().Warn` at line 378 | Does not apply: already resolved at HEAD; file removed from Files to Modify |
| Retired sites: Received/modified `WireUpdate.Attrs()` parse errors (2) | `internal/component/bgp/reactor/reactor_notify.go:619-622,833-836` (`notifyOneMessage`, formerly `notifyMessageReceiver` 444, 638) | Both branches assign `msg.ParseError = parseErr`, recording the parse failure (`recordsMiss`) | Does not apply: already resolved at HEAD; file removed from Files to Modify |
| Retired site: BFD session event delivery | `internal/component/bgp/reactor/peer_bfd.go:684-690` (`deliverSessionBFDEvent`, formerly `Peer.deliverBFDEvent` 663) | Sets close reason, closes connection, and calls `session.handleConnectionClose()` on error | Does not apply: already resolved at HEAD; only lines 332 and 751 remain in `peer_bfd.go` |
| Retired site: Direct forward batch error | `internal/component/bgp/reactor/reactor_api_forward_batch.go:201-208` (`reactorAPIAdapter.ForwardUpdatesDirect`, formerly line 190) | Joins non-suppressed `fwdErr` into `dispatchErr` and returns it | Does not apply: already resolved at HEAD; only `ackBatch` line 238 remains |
| Retired site: Pending admission refusal TCP tuning | `internal/component/bgp/reactor/reactor_connection.go:354-366` (`rejectConnectionAdmissionWithSettings`, formerly `rejectConnectionCollisionWithSettings` 277) | Calls `reportPendingRefusalFailure`, logging `Warn` under `doctor-bgp-pending-refusal-undelivered` and raising on report bus | Does not apply: already resolved at HEAD; only `closeConnQuietly` line 349 remains |
| Retired site: HoldTimer expiry notification | `internal/component/bgp/reactor/session.go:966-974` and `session_hold_expiry.go:34-76` (`Session.holdTimerExpiredLocked`, formerly `NewSession.func` 796) | Logs strict-mode expiry at `Warn` under `codeBFDWaitHoldExpired` and routes notification errors through `s.logNotifyOutcome` | Does not apply: already resolved at HEAD; only `OnKeepaliveTimerExpires` line 994 remains in `session.go` |
| Retired site: Panic cease notification | `internal/component/bgp/reactor/session_connection.go:532-565` (`Session.ceaseAfterPanic`, formerly line 636) | Logs non-busy notification send failures at `sessionLogger().Warn("notification send failed", ...)` | Does not apply: already resolved at HEAD; only `SO_RCVBUF` 207 and `SO_SNDBUF` 210 remain |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | One schema, one command registry, every interface | the door every change reads | "Every message a user sees names the problem, why, and the exact fix" | agrees | Every answer-2 `Warn` and answer-1 CLI error follows `ai/rules/user-facing-errors.md` with problem, why, and fix, and no `pkg/` public contract changes |
| `docs/architecture/core-design.md` | 15. Operational Report Bus | the `// Design:` page of `decode_plugin.go`, `listener.go`, `peer.go`, `peer_admin.go`, `peer_connection.go`, `peer_initial_sync.go`, `peer_static_wire.go`, `reactor.go`, `reactor_api_forward_batch.go`, `reactor_connection.go`, `session.go`, `session_connection.go`, `session_validation.go`, `server/events.go`; anchored to `peer_bfd.go`, `register_diagnostic.go` | "Ze's `internal/core/report/` package is the single cross-subsystem place where operator-visible warnings and errors live." | agrees | Transient answer-2 failures raise through `internal/core/report` under registered doctor codes; no new warning store or parallel bus is introduced |
| `ai/doors/bgp-engine.md` | Boundaries | the door covering all 18 target files, `doctor_codes.go`, `register_diagnostic.go`, `peer_static_wire_test.go`, `server/register_doctor_codes.go` | "Received bytes are borrowed." | agrees | Per-route/per-message paths count or rate-limit and never copy borrowed receive-buffer bytes; all engine code stays under `ze_bgp` with no plugin imports |
| `docs/architecture/behavior/peer-lifecycle.md` | Entry points | anchored to `listener.go`, `peer.go`, `peer_connection.go`, `peer_initial_sync.go`, `reactor.go`, `reactor_connection.go`, `reactor_iface.go`, `session.go`, `session_connection.go`, `server/events.go`, `register_diagnostic.go` | "A reload's binding restart uses `teardownOf` with the session it decided on instead: it does nothing once the peer has left that session, and the drain drops it if still queued when a later session holds the peer." | agrees | Resolving `Peer.teardownOf:2407`, `finishCollisionHandoff:170`, `runQueuedTeardown:760`, and `closeConnQuietly:349` preserves peer teardown, collision, and reconnect semantics unchanged |
| `docs/architecture/config/apply-ordering.md` | Phases 2 and 5, for BGP | the `// Design:` page of `operation.go`; anchored to `operation.go`, `reactor.go`, `reactor_iface.go` | "`handleAddressRemoved` stops the LISTENER bound to a removed address and `handleAddressAdded` starts it again, both run by the reactor's address table watch." | agrees | Resolving `emitOperationListenerReady:203` and `handleAddressAdded:331` EventBus emit error drops preserves operation ordering and listener lifecycle |
| `docs/architecture/api/architecture.md` | TL;DR (Read This First) | anchored to `operation.go`, `peer.go`, `peer_initial_sync.go`, `reactor.go`, `session.go`, `server/events.go` | - | silent | This page specifies plugin RIB ownership, bus wiring, and RPC/event architecture; it sets no rule on internal `slog` error-handling levels or guard annotations at our 48 sites |
| `docs/architecture/config/transaction-protocol.md` | 1. Design Principles | anchored to `operation.go` | - | silent | This page specifies config transaction phases, timeouts, and rollback; `operation.go:203` only handles a post-apply `EventListenerReady` EventBus emit failure and changes no transaction behavior |
| `docs/architecture/api/capability-contract.md` | TL;DR | anchored to `peer.go`, `reactor.go` | - | silent | This page specifies OPEN capability injection and BGP cache control commands; none of the 48 dropped-error sites changes capability negotiation or cache control contracts |
| `docs/architecture/api/commands.md` | Operational Report Bus (ze-cmd:show-warnings, ze-cmd:show-errors) | anchored to `peer.go`, `reactor.go`, `reactor_api_forward_batch.go` | "The report bus is a single in-process place for Ze subsystems to push operator-visible issues." | agrees | Transient answer-2 failures raise onto the existing operational report bus so `show warnings` / `show errors` and `recent-events` surface them |
| `docs/architecture/api/ipc_protocol.md` | Overview | anchored to `peer.go`, `reactor.go` | - | silent | This page specifies IPC transport framing and correlation between daemon and plugins; no IPC wire format or correlation behavior changes here |
| `docs/architecture/api/json-format.md` | Overview | anchored to `peer.go`, `server/events.go` | - | silent | This page specifies JSON event field schemas; `server/events.go:1171` only resolves `eorEventBus.Emit` error handling without changing JSON payload fields |
| `docs/architecture/api/process-protocol.md` | Overview | anchored to `peer.go`, `peer_initial_sync.go`, `reactor.go`, `reactor_api_forward_batch.go`, `reactor_connection.go`, `server/events.go` | - | silent | This page specifies the 5-stage plugin startup protocol and runtime RPC framing; none of the 48 sites alters plugin startup, RPC framing, or delivery guarantees |
| `docs/architecture/api/update-syntax.md` | Overview | anchored to `peer.go` | - | silent | This page specifies `update text` command grammar; no command syntax or parser behavior is modified by resolving `Peer.teardownOf:2407` |
| `docs/architecture/behavior/fsm-open-sent.md` | TL;DR | anchored to `peer.go`, `session.go`, `session_bfd_strict.go`, `session_connection.go` | - | silent | This runbook documents OpenSent FSM transitions and timers; resolving `session_bfd_strict.go` error-drop logging does not alter OpenSent FSM transitions |
| `docs/architecture/behavior/fsm.md` | The six BFD strict-mode events | the `// Design:` page of `session_bfd_strict.go`; anchored to `peer.go`, `peer_bfd.go`, `reactor.go`, `reactor_connection.go`, `session.go`, `session_bfd_strict.go`, `session_connection.go` | "draft-ietf-idr-bgp-bfd-strict-mode Section 4 adds six events, registered as FSM events 30 to 35 by its Section 13.3." | agrees | Resolving the four error drops in `session_bfd_strict.go` (`raiseBFDHoldTimerExpired`, `releaseBFDHoldDown`, `raiseBFDStrictConfigChanged`) preserves all six BFD strict-mode FSM events and ConnectRetryCounter rules |
| `docs/architecture/bgp/structural-forwarding.md` | Four costs, and what replaced them | anchored to `peer.go`, `peer_initial_sync.go`, `session_connection.go` | - | silent | This page documents forwarding critical-path optimizations; none of the 48 sites touches the forwarding fast path or batch wire builders |
| `docs/architecture/config/syntax.md` | Overview | anchored to `peer.go`, `reactor.go`, `reactor_connection.go` | - | silent | This page specifies Ze's configuration syntax; this spec adds or changes no configuration statement or YANG leaf |
| `docs/architecture/encoding-context.md` | Overview | anchored to `peer.go`, `session.go` | - | silent | This page specifies `EncodingContext` and `ContextID` capability matching; resolving `NewSession.func:994` (`OnKeepaliveTimerExpires`) and `Peer.teardownOf:2407` leaves encoding contexts untouched |
| `docs/architecture/forward-congestion-pool.md` | Invariant | anchored to `peer.go`, `reactor.go`, `session.go` | "Congestion does not silently discard a current route." | agrees | Resolving `Reactor.emitCongestionEventBus:996` and `Peer.teardownOf:2407` preserves congestion buffering and teardown guarantees without discarding routes |
| `docs/architecture/meta/role.md` | Keys Set | anchored to `peer.go`, `server/events.go` | - | silent | This page documents route metadata keys set and read by `bgp-role`; none of the 48 sites modifies route metadata keys |
| `docs/architecture/plugin/plugin-system.md` | Where each layer lives | anchored to `peer.go`, `peer_initial_sync.go`, `reactor.go`, `server/events.go` | - | silent | This page specifies plugin registration layers, setup records, and Stage 1 checks; none of those registration contracts changes here |
| `docs/architecture/plugin/rib-storage-design.md` | Overview | anchored to `peer.go` | - | silent | This page specifies `bgp-rib` storage layout and pools; `Peer.teardownOf:2407` does not touch RIB storage |
| `docs/architecture/testing/ci-format.md` | Syntax Overview | anchored to `peer.go` | - | silent | This page specifies the `.ci` functional test file grammar; this spec makes no change to `.ci` harness syntax |
| `docs/architecture/update-building.md` | TL;DR (Read This First) | anchored to `peer.go`, `peer_initial_sync.go`, `peer_static_wire.go`, `session.go`, `session_connection.go` | - | silent | This page specifies `UpdateBuilder` and `*Params` construction paths; raising `peer_static_wire.go` next-hop resolution failures to `Warn` leaves UPDATE wire building unchanged |
| `docs/architecture/wire/capabilities.md` | TL;DR (Read This First) | anchored to `peer.go`, `peer_initial_sync.go` | - | silent | This page specifies OPEN capability TLV wire layouts; no capability encoding or negotiation changes here |
| `rfc/short/rfc5882.md` | Service Interface | the `// Design:` page of `peer_bfd.go` | "A client can establish or destroy a session at any time. BFD does not require the peer to agree that a session exists." | agrees | Resolving `Peer.openBFDSession:332` and `Peer.releaseBFDClient:751` `ReleaseSession` error drops preserves RFC 5882 BFD session creation, sharing, and release semantics |
| `docs/architecture/behavior/fsm-connect.md` | TL;DR | anchored to `peer_connection.go`, `session.go`, `session_bfd_strict.go`, `session_connection.go` | - | silent | This runbook documents Connect-state FSM transitions; resolving error-drop logging in `peer_connection.go` and `session_bfd_strict.go` alters no Connect-state transition |
| `docs/architecture/bgp/on-demand-origination.md` | The decisions | anchored to `peer_initial_sync.go` | - | silent | This page specifies CLI announce/withdraw commands and tag tracking; `peer_initial_sync.go` sites only resolve queued/plugin/default-originate send error drops without changing origination policy or grammar |
| `docs/architecture/behavior/signals.md` | Supported Signals | anchored to `reactor.go` | - | silent | This page documents SIGTERM/SIGHUP/SIGALRM handling; `reactor.go:996,1016` only resolve EventBus emit error drops and do not touch signal handling |
| `docs/architecture/bgp/interface-event-reactions.md` | Constraints | the `// Design:` page of `reactor_iface.go`; anchored to `reactor.go`, `reactor_connection.go`, `reactor_iface.go` | "BGP sends that event once every listener the address owes is up." | agrees | Resolving `Reactor.handleAddressAdded:331` (`EventListenerReady` emit error drop) preserves the listener-ready emission contract and address/neighbor watch behavior |
| `docs/architecture/config/environment.md` | Overview | anchored to `reactor.go`, `session_connection.go` | - | silent | This page catalogs `ze.*` environment variables; this spec adds or changes no environment variable |
| `docs/architecture/congestion-industry.md` | Universal Rule | anchored to `reactor.go`, `session.go` | - | silent | This survey documents industry BGP congestion strategies; resolving `emitCongestionEventBus:996` and `OnKeepaliveTimerExpires:994` changes no congestion or queue policy |
| `docs/architecture/overview.md` | 1. What is Ze? | anchored to `reactor.go` | - | silent | This high-level overview describes Ze's components and directory layout; it sets no constraints on per-site error logging in `reactor.go` |
| `docs/architecture/rib-transition.md` | Executive Summary | anchored to `reactor.go`, `session.go` | - | silent | This historical transition doc explains moving RIB storage to `bgp-rib`; none of the 48 sites alters RIB ownership |
| `docs/architecture/behavior/fsm-established.md` | TL;DR | anchored to `reactor_connection.go`, `session.go`, `session_connection.go` | - | silent | This runbook documents Established-state FSM events and timers; resolving `closeConnQuietly:349`, `OnKeepaliveTimerExpires:994`, and `SO_RCVBUF`/`SO_SNDBUF` drops alters no Established transition |
| `docs/architecture/chaos-web-dashboard.md` | Architecture Overview | anchored to `reactor_connection.go` | - | silent | This page documents the chaos simulator web UI; `reactor_connection.go:349` (`closeConnQuietly`) has no interaction with the chaos dashboard |
| `docs/architecture/route-selection.md` | Overview | anchored to `reactor_iface.go`, `session_validation.go` | - | silent | This page specifies RIB route-selection phases and AIGP policy; `reactor_iface.go:331` and `session_validation.go:507` do not alter route selection |
| `docs/architecture/buffer-architecture.md` | Why allocations on the UPDATE path cost | the `// Design:` page of `session.go`; anchored to `session.go` | "Every allocation on that path adds GC pressure and latency, so the path is built from three interlocking strategies: the caller owns the buffer, bounded pools replace `make()`, and the read side stays lazy" | agrees | No hot-path buffer allocation is added; per-route/per-message error paths count or rate-limit without formatting or copying wire bodies |
| `docs/architecture/behavior/fsm-active.md` | TL;DR | anchored to `session.go`, `session_bfd_strict.go`, `session_connection.go` | - | silent | This runbook documents Active-state FSM transitions; resolving error-drop logging in these files alters no Active-state transition |
| `docs/architecture/behavior/fsm-idle.md` | TL;DR | anchored to `session.go`, `session_connection.go` | - | silent | This runbook documents Idle-state FSM transitions; resolving error-drop logging in `session.go` and `session_connection.go` alters no Idle-state transition |
| `docs/architecture/behavior/fsm-open-confirm.md` | TL;DR | anchored to `session.go`, `session_bfd_strict.go`, `session_connection.go` | - | silent | This runbook documents OpenConfirm-state FSM transitions; resolving error-drop logging in `session_bfd_strict.go` alters no OpenConfirm-state transition |
| `docs/architecture/edge-cases/extended-message.md` | TL;DR | anchored to `session.go` | - | silent | This page specifies RFC 8654 65535-byte message handling; `session.go:994` (`OnKeepaliveTimerExpires`) does not touch extended-message framing |
| `docs/architecture/memory/lifetime-contracts.md` | TL;DR | anchored to `session.go` | "On the hot path Ze **borrows** bytes (zero-copy slices) rather than copying them." | agrees | No site in this spec retains a borrowed buffer past its boundary or copies wire bytes onto the hot path |
| `docs/architecture/message-buffer-design.md` | Overview | anchored to `session.go` | - | silent | This page specifies message buffer layouts and peer buffer pools; `session.go:994` (`OnKeepaliveTimerExpires`) does not alter buffer pool design |
| `docs/architecture/edge-cases/as4.md` | Overview | anchored to `session_validation.go` | - | silent | This page specifies RFC 6793 4-byte ASN reconstruction; `session_validation.go:507` only handles a `wu.Attrs()` error drop in `publishBase` without changing AS4 logic |
| `docs/architecture/rfc-may-decisions.md` | RFC 4760 Section 7 - Non-Negotiated AFI/SAFI Handling | anchored to `session_validation.go` | - | silent | This page records RFC MAY policy choices (non-negotiated AFI/SAFI and MP attribute placement); `session_validation.go:507` changes none of them |
| `docs/architecture/wire/attributes.md` | TL;DR (Read This First) | anchored to `session_validation.go` | - | silent | This page specifies BGP path attribute wire formats; `session_validation.go:507` only resolves `wu.Attrs()` error handling without changing attribute wire encoding |
| `docs/architecture/wire/messages.md` | TL;DR (Read This First) | anchored to `session_validation.go` | - | silent | This page specifies BGP message header and body wire formats; no message framing or parsing rules change here |
| `docs/architecture/exabgp-bridge.md` | The bridge translates in two directions | anchored to `server/events.go` | - | silent | This page specifies the ExaBGP compatibility wrapper; `server/events.go:1171` only resolves `eorEventBus.Emit` error handling for internal EventBus consumers |
| `docs/architecture/doctor-and-health-checks.md` | Every problem site names a doctor code | the `// Design:` page of `doctor_codes.go`, `register_diagnostic.go` | "A site is covered when its code is one that a doctor check declares." | agrees | Every new `Warn`/`Error` added by this spec emits a constant `code` attribute registered via `diagnostic.Register` and `diagnostic.RegisterEventCode` in `doctor_codes.go` + `register_diagnostic.go` (or `server/register_doctor_codes.go`), passing `./le arch doctor-coverage check` |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | This spec changes no data-plane backend validation, `fib` config, or `sysctl` handling |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | This spec adds no config mismatch rule or start/reload verdict divergence |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | This spec does not touch firewall verification or YANG backend annotations |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | This spec introduces no data-plane selector or backend naming |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | This spec does not alter egress next-hop rewriting or export filter chains; `peer_static_wire.go` only reports existing `resolveNextHop` failures at `Warn` |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | This spec makes no change to `policychain.Sets` or policy authoring syntax |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | This spec does not touch `policychain.Chain` compilation or built-in next-hop steps |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | This spec does not alter export-chain folding or `wireOverride` provenance checks |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | This spec does not modify `NextHopProfile` or import/export chain validation |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | `peer_static_wire.go` next-hop resolution logging changes no next-hop policy on locally originated or forwarded routes |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | This spec does not alter implied-filter publication or `\| implied` output |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | "Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code. Examples are a web certificate name that does not resolve, a looking-glass `server` entry with no certificate to present, and a looking-glass address list that does not parse. Where a service has several endpoints, only the endpoint that cannot be served stays down and the others serve. A reload whose config cannot be served stays refused as a whole, and the running services keep what they serve. In Thomas's words (voice-transcribed): "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; asked about a reload, "reload stay refused"; asked "Should only that endpoint stay down while the others serve", "yes". The reason is that a daemon that does not start brings up no networking, so the operator cannot reach the box to correct it." | agrees | This spec never refuses or aborts `ze start`; every answer-2 runtime failure logs at `Warn` under a registered doctor code with its fix |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | "The rule of 2026-11-23 above, that a configuration which holds a feature that programs only the Linux kernel while the interfaces run on VPP "is refused at daemon start", is superseded for the start: ze starts, that feature stays off, and `ze doctor` reports the mismatch with its fix. A commit, a reload and `ze config validate` still refuse such a configuration, by the same verdict as before. Thomas was asked whether the rule that ze always starts replaces the refusal at start, "(a) yes, ze starts, that feature stays off and ze doctor reports the mismatch, while commit and reload still refuse it; (b) no", and answered "2a" (voice-transcribed). The reason is the one of the row above: a daemon that does not start brings up no networking, so the operator cannot reach the box to correct the mismatch." | agrees | This spec adds no startup refusal and aligns with reporting actionable failures via registered doctor codes |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "When `\| implied` modifies an existing config line (such as inserting automatic members into a `filters [ ... ];` list), it keeps the line intact and places an inline `# implied: ...` comment on the same line after the statement naming what was added and why, rather than placing a comment only above the line or splitting the list across multiple lines. In Thomas's words (2026-11-23): "No, it should be on the same line and say what was added"." | agrees | This spec does not touch `\| implied` rendering |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "Everything on the bus is a participant named by its configuration path. A participant registers the messages it sends and receives, events and commands alike, and with nothing written that registration is what runs" | agrees | This spec preserves all bus participant naming and default message registrations |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A `send > to` list written on a participant replaces the destinations its messages have by registration; it never adds to them. Present and empty means nobody" | agrees | This spec does not alter `send > to` bus routing |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A `receive > from` list limits who may address a participant. Absent means everybody; present and empty means nobody" | agrees | This spec does not alter `receive > from` bus filtering |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A send and a receive allow-list of message names (`send > allow`, `receive > allow`) exist on every participant: absent means everything declared, empty means nothing. The bus enforces them when it receives the message" | agrees | This spec does not alter `send > allow` or `receive > allow` enforcement |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "What is written on the participant and what is written in the `bus` section compile into one filter per participant. A disagreement between the two places, or between an allow-list and a destination on one participant, is refused at commit naming both lines" | agrees | This spec does not touch bus filter compilation or commit-time disagreement checks |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "The bus does not arbitrate between participants: no exclusive-role rule, no commit refusal and no silent winner. A plugin that must not duplicate another limits itself" | agrees | This spec adds no bus arbitration or exclusive-role rules |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "One display shows every registered default beside what the operator wrote, the defaults a written list replaced, and the line that produced each effective entry" | agrees | This spec does not modify effective bus-wiring display |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-08-21 | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | This spec adds no CLI command path or selector word |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-20 | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | This spec does not override or modify any generic pipe operation |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-23 | "A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does" | agrees | This spec touches no pipe operation semantics |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-23 | "Every engine operation names the message it carries or `none` with a reason, and a plugin identity is authorized by a profile compiled from its filter" | agrees | This spec adds no engine operation and preserves plugin profile authorization |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-25 | "Engine-to-plugin event delivery never drops events on queue overflow; endpoints expect every subscribed network event losslessly" | agrees | This spec never drops events on queue overflow; it only surfaces or guards already-present `Emit`/delivery error branches |
| `docs/architecture/plugin/plugin-system.md` | Design decisions | decision of 2026-11-23 | "With no daemon running, a command that parses a config starts each declared external program in query mode, never its run loop, and validates against its declaration" | agrees | This spec does not modify config-parse query-mode startup; `decode_plugin.go` only surfaces decode subprocess/pipe failures for `ze bgp decode --plugin` |
| `docs/architecture/update-building.md` | Design decisions | decision of 2026-11-23 | "Gate draft-simpson 0x0800 FlowSpec MP_REACH next-hop emission behind an explicit per-session opt-in (`session { flowspec-redirect-next-hop true; }`, default `false`), written automatically by `ze exabgp migrate` when a migrated neighbor uses Simpson actions or serves an API process on a FlowSpec family" | agrees | This spec makes no change to FlowSpec MP_REACH next-hop encoding or opt-in gating |
| `docs/architecture/bgp/interface-event-reactions.md` | Design decisions | decision of 2026-11-22 | "Interface and neighbor change re-advertisement runs the first flip immediately, then coalesces bursts behind a per-peer cooldown with at most one queued trailing re-pass; BGP route flap dampening (RFC 2439) is a separate concept and, if ever implemented, must be an option off by default ("flap dampening should be an option and off by default as since the RFC it was found to not help. 1A is fine.")" | agrees | Resolving `Reactor.handleAddressAdded:331` (`EventListenerReady` emit drop) preserves interface/neighbor re-advertisement coalescing and makes no dampening change |
| `docs/architecture/exabgp-bridge.md` | Design decisions | decision of 2026-11-23 | "An ExaBGP script is an ordinary bus endpoint that runs the wrapper; the bridge config section and the internal bridge mode do not exist" | agrees | This spec does not touch ExaBGP wrapper configuration or bus endpoint wiring |
| `docs/architecture/exabgp-bridge.md` | Design decisions | decision of 2026-11-26 | "ExaBGP wrapper endpoints request `base64` (`plugin.FormatBase64`) UPDATE delivery at Stage 5 `ready` using Ze's positional counted wire grammar (`upd <flags> <local-as> <remote-as> <conn-id> <n>:<peer> <n>:<local> <n>:<router-id> <n>:<extra-addpath> <n>:<b64-body>`, zero key names, `0:` when absent) over pooled `WriteBatchFrame` `MuxConn` batching while keeping low-frequency control/session events as JSON; `ze exabgp plugin` renders hot IPv4/IPv6 unicast UPDATEs directly into a `[]byte` buffer from wire bytes and falls back to the wire-backed `Event` renderer for non-unicast families and error cases" | agrees | `server/events.go:1171` touches only `onEORReceived` EventBus emission, leaving ExaBGP `base64` UPDATE delivery and `WriteBatchFrame` batching untouched |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A failed call inside the reactor, the decode CLI, or the BGP server: socket write/option/close, BFD release, event-bus emit, cache ack, or decode plugin process I/O.

### Transformation Path
1. The branch catches the error.
2. Answer 1: returns the error to the caller (e.g., `ze bgp decode --plugin` reports the plugin failure to the operator).
3. Answer 2: counts it on a per-route/per-message path and logs a `Warn` (once per condition on loops) carrying what failed, why it matters, the fix, and a registered doctor `code` (plus a report-bus raise for transient problems).
4. Answer 3: carries `// refuse-by-default: guards <Symbol> -- <why>` when going on is right and nobody can act.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Reactor ↔ operator | Log at `Warn` with registered doctor `code`, existing metric, `show warnings` / `show errors` via `recent-events` | No |
| BGP server ↔ operator | Log at `Warn` with registered doctor `code` (if answer 2) or annotated non-actionable drop (if answer 3) | No |
| CLI ↔ operator | Returned error printed by `ze bgp decode` | No |

### Changed-Wire Census

No BGP protocol wire format, plugin RPC schema, JSON event schema, or YANG configuration syntax changes. Search evidence: inspected all 18 remaining target files in `internal/component/bgp/{cli,reactor,server}` and all 48 `error-dropped-below-warn` sites from `./le arch refuse-by-default report '|' json` at HEAD (2026-11-26); no file under `pkg/`, no message encoder/decoder wire framing, and no YANG module is modified. Positive controls: `decode_plugin.go` decodes existing `"decoded json ..."` lines and only surfaces I/O/process errors to the CLI caller; reactor/server sites only change internal error handling, log severity/doctor-code attributes, existing counters, or guard annotations.

### Integration Points
- Existing subsystem loggers (`routesLogger`, `peerLogger`, `sessionLogger`, `reactorLogger`, `cacheLogger`, `server.logger`), reactor metric families (`prefixMetrics`), and doctor-code / `EventCode` registration (`internal/component/bgp/reactor/doctor_codes.go`, `internal/component/bgp/reactor/register_diagnostic.go`, and `internal/component/bgp/server/register_doctor_codes.go` if needed).

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Errors flow through existing return values, loggers, metrics, and report bus |
| No unintended coupling (components stay isolated) | Yes | No new cross-component imports; `doctor_codes.go` and `register_diagnostic.go` already import `internal/core/diagnostic` |
| No duplicated functionality (extends existing, does not recreate) | Yes | Reuses existing loggers, metrics, `doctor_codes.go`, `register_diagnostic.go`, and `report` bus |
| Zero-copy preserved where applicable (refs, not copies) | Yes | Per-route/per-message paths count or rate-limit and never copy borrowed read-buffer bytes |
| Registration over hardcoding, outbound | Yes | Any new doctor code registers via `diagnostic.Register` and `diagnostic.RegisterEventCode` in its owning package's `init()` |
| Registration over hardcoding, inbound | Yes | Searched `internal/le/arch/doctorcoverage/codes.go` and `internal/component/doctor/checks_recent_events.go`: both derive covered codes from `diagnostic.EventCode` / check declarations with no central list to edit |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The 48 findings across 18 files are the exact population at HEAD | `./le arch refuse-by-default report '|' json` filtered to `internal/component/bgp/` minus `plugins/` and `internal/core/bgp/` on 2026-11-26 (48 findings, 1 annotated at `aigp_readvertise.go:576`, 9 retired) | Re-run report and work from the fresh list | Re-run filtered report before first implementation edit | confirmed 2026-11-26 |
| A-2 | Teardown-path and already-closed-socket drops are answer 3 | A close or write error on a connection already being torn down has nothing left to undo | A drop that hides a peer left half up | Read each teardown site's caller before annotating | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A `Warn` per UPDATE or route floods the log under a broken peer | A plugin `.ci` log grows by thousands of lines | Count in existing reactor metrics and `Warn` once per peer and condition |
| R-2 | Returning an error where the caller treats any error as a teardown | A `.ci` peer session drops | Keep the caller's existing error handling in view; answer 1 only where the caller acts correctly |
| R-3 | `./le arch doctor-coverage check` refuses an added `Warn`/`Error` line | Commit gate fails with `no-code` or `code-not-doctor` | Register every new doctor code in `doctor_codes.go` + `register_diagnostic.go` (or `server/register_doctor_codes.go`), emit `"code", <const>` on every added `Warn`/`Error`, and raise transient problems on the report bus |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Log noise, commit refused by `doctor-coverage`, or a session torn down by a newly returned error |
| How is it reverted? | Single commit revert |
| Who else touches this path? | `spec-bgp-one-off-defects` (historical static-route send-error row already resolved on HEAD), `plan/release/validation/spec-doctor-coverage-6-bgp-engine.md` (shares doctor codes for reactor/server log lines, its R-3) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le arch refuse-by-default report` | → | `droppedError` over this child's paths | Filtered report over `internal/component/bgp/` minus `plugins/` and `internal/core/bgp/` returns 0 `error-dropped-below-warn` findings |
| A next hop that fails to resolve for a static route | → | `Peer.sendStaticRoutes` / `Peer.sendStaticRoutesGrouped` | `TestSendStaticRoutesReportsUnresolvedNextHop` (`internal/component/bgp/reactor/peer_static_wire_test.go`) asserts the `Warn` carrying its registered doctor `code` and fix |
| `ze bgp decode --plugin <path>` encountering a plugin I/O or exit failure | → | `decode_plugin.go` (`invokePluginPath`, `invokePluginInternal`) | `TestInvokePluginPathReturnsFailure` (`internal/component/bgp/cli/decode_plugin_named_test.go`) asserts the caller observes the error |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | `./le arch refuse-by-default report '|' json`, filtered to `internal/component/bgp/` (excluding `plugins/`) and `internal/core/bgp/` | The filtered report succeeds and outputs an empty `findings` list for rule `error-dropped-below-warn`, with every answer-3 site listed in `annotated` | Only properly annotated answer-3 sites in `annotated` (plus the pre-existing `aigp_readvertise.go:576` annotation), because going on is right and nobody can act on those drops |
| AC-2 | Each instance resolved by answer 1 (including `bgp/cli/decode_plugin.go` failures) | A unit test triggers the failure and asserts the caller receives and surfaces the non-nil error | None: every answer-1 instance must propagate its error to a caller that acts on it |
| AC-3 | Each file with an answer-2 `Warn` or `Error` | A unit test captures one `Warn` line at the default level and asserts it names what failed, why it matters, the fix, and carries a registered doctor `code` declared in the owning package (`doctor_codes.go` + `register_diagnostic.go` or `server/register_doctor_codes.go`); `./le arch doctor-coverage check` passes with every added site marked `covered` | Existing pre-existing uncovered sites outside lines added/changed by this spec remain owned by `spec-doctor-coverage-6-bgp-engine.md` |
| AC-4 | Each answer-3 guard annotation | Every annotation names a guard symbol visible in the same file and states why going on is correct, and appears with its `guard` field populated in the report's `annotated` list | None: unannotated drops or annotations naming non-existent symbols are rejected by `refuse-by-default` |
| AC-5 | Scoped BGP package tests (`-race -tags ze_bgp`) and reached functional `.ci` suites (`test/plugin`, `test/decode`, `test/encode`, including `capability-llgr`) | All scoped package tests and reached `.ci` tests pass green | None: regressions in touched packages or `.ci` scenarios are fixed before closure |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSendStaticRoutesReportsUnresolvedNextHop` | `internal/component/bgp/reactor/peer_static_wire_test.go` | AC-3 for `sendStaticRoutes` and `sendStaticRoutesGrouped` next-hop resolution failures | |
| `TestInvokePluginPathReturnsFailure` and one test per answer-1 failure path in `decode_plugin.go` | `internal/component/bgp/cli/decode_plugin_named_test.go` | AC-2 for `decode_plugin.go` answer-1 error returns | |
| One test per reactor/server file with an answer-2 `Warn` asserting default-level capture, operator fix text, and registered doctor `code` | Owning package's `*_test.go` | AC-3 and doctor-coverage registration | |
| `TestRefuseByDefaultReportListsTree` | `internal/le/arch/refusebydefault/refusebydefault_test.go` | Detector unchanged | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| unannotated findings under this child's paths | 0 | 0 | N/A | 1 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| The `test/plugin`, `test/decode` and `test/encode` `.ci` that reach changed files | `test/` | Unchanged protocol behavior, plus the `Warn` where a `.ci` drives the failure | |
| `capability-llgr` | `test/decode/capability-llgr.ci` | `ze bgp decode --plugin bgp-gr --open` still answers through `decode_plugin.go`, the path whose dropped errors answer 1 returns | |

## Files to Modify
- `internal/component/bgp/cli/decode_plugin.go` - 14: `invokePluginDecodeRequest` 243, 246; `invokePluginPath` 349, 352, 362, 369; `invokePluginSubprocess` 427, 430; `invokePluginInternal.func` 497, 502, 505; `invokePluginInternal` 515, 520, 526
- `internal/component/bgp/reactor/listener.go` - 3: `Listener.acceptLoop.func` 172; `Listener.acceptLoop` 198; `Listener.safeHandle.func` 211
- `internal/component/bgp/reactor/operation.go` - 1: `reactorAPIAdapter.emitOperationListenerReady` 203
- `internal/component/bgp/reactor/peer.go` - 1: `Peer.teardownOf` 2407
- `internal/component/bgp/reactor/peer_admin.go` - 1: `rejectAdminDownConnection` 132
- `internal/component/bgp/reactor/peer_bfd.go` - 2: `Peer.openBFDSession` 332; `Peer.releaseBFDClient` 751
- `internal/component/bgp/reactor/peer_connection.go` - 1: `Peer.finishCollisionHandoff` 170
- `internal/component/bgp/reactor/peer_initial_sync.go` - 9: `Peer.sendInitialRoutes` 333; `Peer.drainAndCloseQueueGate` 621, 644, 670; `Peer.runQueuedTeardown` 760; `Peer.sendPluginRoutesVia` 1071; `Peer.sendPluginRouteGroup.func` 1114; `Peer.sendPluginRouteGroup` 1144; `Peer.sendDefaultOriginateRoutes` 1286
- `internal/component/bgp/reactor/peer_static_wire.go` - 2: `Peer.sendStaticRoutes` 66; `Peer.sendStaticRoutesGrouped` 143
- `internal/component/bgp/reactor/reactor.go` - 2: `Reactor.emitCongestionEventBus` 996; `Reactor.emitUpdateNotificationEvent` 1016
- `internal/component/bgp/reactor/reactor_api_forward_batch.go` - 1: `reactorAPIAdapter.ackBatch` 238
- `internal/component/bgp/reactor/reactor_connection.go` - 1: `closeConnQuietly` 349
- `internal/component/bgp/reactor/reactor_iface.go` - 1: `Reactor.handleAddressAdded` 331
- `internal/component/bgp/reactor/session.go` - 1: `NewSession.func` 994
- `internal/component/bgp/reactor/session_bfd_strict.go` - 4: `Session.raiseBFDHoldTimerExpired` 106; `Session.releaseBFDHoldDown` 124, 130; `Session.raiseBFDStrictConfigChanged` 223
- `internal/component/bgp/reactor/session_connection.go` - 2: `tuneTCPConnectionForSettings.func` 207, 210
- `internal/component/bgp/reactor/session_validation.go` - 1: `Session.publishBase` 507
- `internal/component/bgp/server/events.go` - 1: `onEORReceived` 1171
- `internal/component/bgp/reactor/doctor_codes.go` - register `diagnostic.CodeMeta` for any doctor code emitted by new reactor answer-2 `Warn`/`Error` lines
- `internal/component/bgp/reactor/register_diagnostic.go` - register `diagnostic.EventCode` in `init()` for any transient reactor answer-2 doctor code so `recent-events` and `./le arch doctor-coverage check` cover it
- `internal/component/bgp/reactor/peer_static_wire_test.go` - add `TestSendStaticRoutesReportsUnresolvedNextHop`

Lines are verified against HEAD on 2026-11-26; the live report remains the authoritative population at implementation time.

## Files to Create
- `internal/component/bgp/server/register_doctor_codes.go` - register `diagnostic.CodeMeta` and `diagnostic.EventCode` in `init()` if `internal/component/bgp/server/events.go:1171` is resolved as an answer-2 `Warn` (omit if resolved as an answer-3 annotation)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No config |
| YANG validation constraints | N-A | No leaf |
| YANG custom validators | N-A | No leaf |
| CLI commands/flags | N-A | No command added; `ze bgp decode --plugin` errors reach the operator |
| CLI grammar (keyword before value) | N-A | No command |
| Editor autocomplete | N-A | No command |
| Functional test for new RPC/API | N-A | No RPC |
| Pipe completeness | N-A | No output change |
| Env var registration | N-A | No env var |
| Doctor check for runtime dependencies | N-A | No new runtime dependency |
| Doctor code per problem class | Yes | Every new `Warn`/`Error` log added by this spec emits a registered doctor `code` (from `spec-doctor-coverage-6-bgp-engine.md`'s map) registered via `diagnostic.Register` and `diagnostic.RegisterEventCode` in `internal/component/bgp/reactor/doctor_codes.go` + `register_diagnostic.go` (and `internal/component/bgp/server/register_doctor_codes.go` if `server/events.go` adds a `Warn`), covered by `recent-events` (`runtime` phase) and passing `./le arch doctor-coverage check` |
| Prometheus counters/metrics | Yes, if answer 2 counts | Existing reactor families (`prefixMetrics`); name each counter used at implementation |
| BGP family surface (new SAFI / capability / attribute) | N-A | None |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | If answer 2 adds a counter | `docs/plugin-development/metrics.md` |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Declared `// Design:` docs: `docs/architecture/behavior/fsm.md`, `docs/architecture/bgp/interface-event-reactions.md`, `docs/architecture/buffer-architecture.md`, `docs/architecture/config/apply-ordering.md`, `docs/architecture/core-design.md`, `docs/architecture/doctor-and-health-checks.md`, `rfc/short/rfc5882.md`. None of their described behavior changes unless an answer-1 fix alters behavior they specify; confirm with `./le spec citation anchors spec plan/release/fixes/spec-dropped-error-1-bgp-engine.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | No | |

## Design presentation

### What changes

All 48 remaining `error-dropped-below-warn` sites across 18 files in `internal/component/bgp/{cli,reactor,server}` (9 of the original 57 from 2026-11-02 were already fixed at HEAD and are retired in Existing Mechanisms) are resolved under the umbrella's three-tier policy: failures where going on is wrong (such as `ze bgp decode --plugin` subprocess/pipe I/O errors) return errors to their callers; actionable runtime failures (such as static-route next-hop resolution failures) emit default-level `Warn` logs with what failed, why, the fix, and a registered doctor `code` (registered in `doctor_codes.go` + `register_diagnostic.go` or `server/register_doctor_codes.go`, with existing metric counting/rate-limiting on per-route loops); and non-actionable teardown/already-closed-socket drops carry explicit `// refuse-by-default: guards <Symbol> -- <why>` annotations.

### Before and after

1. Static-route next-hop resolution failure (`peer_static_wire.go` `sendStaticRoutes` / `sendStaticRoutesGrouped`):
   - Before: `routesLogger().Debug("next-hop resolution failed", ...)` hides the dropped static route at the default `WARN` log level; the peer silently lacks the configured route.
   - After: `routesLogger().Warn(...)` at the default level naming the peer, prefix, cause, fix, and registered doctor `code` (rate-limited/counted on multi-route passes), passing `./le arch doctor-coverage check`.
2. `ze bgp decode --plugin` subprocess/pipe/wait failure (`bgp/cli/decode_plugin.go`):
   - Before: pipe write, close, read, and wait errors are logged only at `slog.Debug` and the decode helper returns `nil`, presenting no explanation to the operator who invoked `--plugin`.
   - After: errors propagate back to the decode caller so the operator sees why the named plugin failed, while valid decodes (such as `test/decode/capability-llgr.ci`) continue to produce identical output.
3. Non-actionable teardown/close drops (e.g., `listener.go` close on cancel, `reactor_connection.go` `closeConnQuietly`, `peer_admin.go` `rejectAdminDownConnection` write on a peer already closing):
   - Before: bare `Debug` log flagged as an unreviewed dropped error by `./le arch refuse-by-default report`.
   - After: explicit `// refuse-by-default: guards <Symbol> -- <why>` naming the guard symbol visible in the file so the report lists 0 unannotated findings under this child's paths.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Raise all 48 `Debug` lines blindly to `Warn` | Violates the umbrella policy and `ze-go-style.md` ("a WARN does not make a default correct"): hides answer-1 bugs in `decode_plugin.go`, floods logs on per-route paths, and emits noisy `Warn`s on normal teardown closes where no operator can act |
| Emit new `Warn` lines without registering doctor codes | Blocked by the `doctor-coverage` commit gate (`./le arch doctor-coverage check`) and `ai/rules/user-facing-errors.md` ("Every `Warn` or `Error` log line ... MUST name, under its `code` attribute, a doctor code") |
| Re-edit the 9 sites already resolved at HEAD (`aigp_readvertise.go`, `filter_ordered.go`, `reactor_notify.go`, etc.) | Redundant churn on files already fixed or annotated; retiring them into `## Existing Mechanisms` keeps the diff minimal and accurate to HEAD |

### Documents departed from

none

### Choices for Thomas

none

## Implementation Steps

1. **Phase: Wiring** -- run `./le arch refuse-by-default report '|' json` filtered to `internal/component/bgp/` minus `plugins/` and `internal/core/bgp/` to confirm the 48-finding population and wire the failing tests for `peer_static_wire.go` and `decode_plugin.go`.
2. **Phase: Classify** -- for each of the 48 instances, read its caller and record the answer (1, 2, or 3) with a one-line reason and (for answer 2) its doctor code in Design Insights.
3. **Phase: Resolve** -- per file: register doctor codes in `doctor_codes.go` + `register_diagnostic.go` (and `server/register_doctor_codes.go` if `server/events.go` takes answer 2); write failing tests first for answers 1 and 2, then apply the fix; add verified answer-3 guard annotations last.
4. **Phase: Prove** -- verify AC-1 through AC-5 with scoped main-thread runs (`./le arch refuse-by-default report`, `./le arch doctor-coverage check`, scoped `-race -tags ze_bgp` package tests, and reached `.ci` functional tests).

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every one of the 48 remaining instances has its answer at file:line |
| Correctness | No answer 3 where the caller would act; no `Warn` per route or message without rate-limiting/counting |
| Operator errors and doctor codes | Every added `Warn`/`Error` names the peer/plugin, operation, why, fix, and a registered doctor `code` declared in `doctor_codes.go` + `register_diagnostic.go` (or `server/register_doctor_codes.go`), and passes `./le arch doctor-coverage check` |
| Rule: user-facing-errors | Each `Warn` names the peer, the operation, why, and the fix |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Zero findings under this child's paths | `./le arch refuse-by-default report '|' json` filtered to `internal/component/bgp/` (excluding `plugins/`) and `internal/core/bgp/` |
| Doctor-coverage clean on every added `Warn`/`Error` | `./le arch doctor-coverage check` |
| Scoped package tests (run by main thread) | `./le job run label bgp command go test -race -tags ze_bgp ./internal/component/bgp/reactor/ ./internal/component/bgp/cli/ ./internal/component/bgp/server/` |
| Scoped functional tests (run by main thread) | `./le test bgp decode capability-llgr` and `./le test bgp plugin <name>` for each `.ci` listed by `./le repo changed scope print both` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Error leakage | A `Warn` or returned error that prints a TCP MD5 key or a peer password |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|

The class row for the whole population is owned by the umbrella.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The umbrella's three answers, in order | Raise every line to `Warn` | A `Warn` does not make a default correct |
| Register every new answer-2 doctor code in the owning package (`doctor_codes.go` + `register_diagnostic.go`, or `server/register_doctor_codes.go`) | Add bare `Warn` lines and defer codes to `spec-doctor-coverage-6-bgp-engine.md` | `./le arch doctor-coverage check` blocks commits that add uncovered `Warn`/`Error` sites; owning-package registration satisfies the gate immediately and stays compatible with child 6's class-to-code map |
| Retire the 9 sites already fixed at HEAD into Existing Mechanisms | Keep stale 57-instance list | Keeps Files to Modify and derived design documents faithful to HEAD |

## Known Limitations
- None.

## Review Gate

Pending implementation; populated during `/ze-review` and `/ze-close`.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] Scoped package tests (`go test -race -tags ze_bgp`), reached `.ci` functional tests, `./le arch refuse-by-default report`, and `./le arch doctor-coverage check` pass (run in the main checkout by the main thread)
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason): N-A unless answer 1 changes wire behavior

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** code + tests + docs + edited spec
- [ ] **Commit B:** `remove plan/release/fixes/spec-dropped-error-1-bgp-engine.md` only, in the same `./le commit create` script
