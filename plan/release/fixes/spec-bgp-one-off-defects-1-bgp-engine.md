# Spec: BGP one-off defects — 1: BGP engine

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Priority | P1 |
| Release reason | Each item repairs incorrect shipped BGP engine behavior (wire framing, session lifecycle, reactor forwarding/recovery packing, config validation, and CLI decode) owned under `ai/doors/bgp-engine.md` |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Design tier | strongest tier |
| Implementation tier | development tier |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Parent umbrella: `plan/release/fixes/spec-bgp-one-off-defects.md`. This child owns the 60 open BGP engine journal rows under `ai/doors/bgp-engine.md` (`internal/component/bgp/{reactor,fsm,message,wireu,config,cli,server,format}` and `internal/core/bgp/`).

## Task

Repair the remaining open one-off defects in the BGP engine discovered during the journal sweep and subsequent verification passes, after retiring the 33 already-fixed rows documented in the parent umbrella (`plan/release/fixes/spec-bgp-one-off-defects.md`). Every defect follows TDD: read the producer at HEAD, reproduce with a red test, implement the minimal repair preserving buffer-first zero-copy encoding and existing mechanisms, and close the corresponding `plan/journal/` row with `fixed: `.

Standing architectural rules applied to every item here:
- **Ze always starts (2026-11-26 ruling, `docs/architecture/core-design.md`)**: any configuration refusal added by AC-11 (`buildOpen` > 4096 octets) or AC-21 (peer/group name collision) refuses at commit, reload, and `ze config validate`, while `ze start` leaves only the invalid peer or endpoint unstarted and reports it with a declared doctor code in logs and `ze doctor` rather than exiting `ze start`.
- **Doctor coverage for `Warn`/`Error` and report-bus raises**: every newly added `Warn`/`Error` line (such as static-route send failures in `peer_static_wire.go`, unsplit MP NLRI logging in `text_human.go`, and recovered forward-batch panics in `forward_pool.go`) carries a literal or constant declared doctor code under `code`, and lasting/transient conditions (including clearing `op-queue-full` on drain and cleanup) use `internal/core/report` under their registered code.
- **No non-Linux release blocker**: the macOS GTSM socket setup row (`silent-fall-through` 2026-09-27) is retained in ownership here so no open row is orphaned, with production Linux behavior unchanged and macOS-only work explicitly marked non-blocking for first release per `ai/META-DECISIONS.md` ("Production is Linux. A check, test or defect that exists only on macOS waits until after the first release.").

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - platform contracts, process boundary, public `pkg/` API, compile-out boundaries
  → Constraint: no BGP engine fix introduces a direct call across a plugin process boundary or leaks gated imports into always-on code.
- [ ] `ai/doors/bgp-engine.md` - reactor, FSM, wire/wireu, pools, filter pipeline, and encoding contexts
  → Constraint: keep `WriteTo(buf, off) int` buffer-first encoding, per-family/per-direction ADD-PATH contexts, and FIFO cache ack order.
- [ ] `docs/architecture/core-design.md` - 2026-11-26 "Ze always starts" ruling, WireUpdate, buffer-first encoding, and policy filter chain
  → Constraint: invalid per-peer configuration never exits `ze start`; `ze` starts healthy peers and reports the unstarted peer under a declared doctor code.
- [ ] `docs/architecture/bgp/structural-forwarding.md` - forward rails, recovery admission, and packing
  → Constraint: recovery packing and overflow release preserve route delivery and per-source FIFO ordering.
- [ ] `docs/architecture/buffer-architecture.md` - `bufMux` pool sizing and ownership lifetimes
  → Constraint: pooled buffers are never returned to callers after release; idle 64K blocks yield budget to 4K demand.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4271.md` - Sections 4.3, 5 (Partial bit, combined withdraw/announce)
- [ ] `rfc/short/rfc2545.md` - Section 3 (IPv6 link-local next hop scope)
- [ ] `rfc/short/rfc6793.md` - Section 4.2.2 (AS_TRANS and AS4_PATH on two-octet sessions)
- [ ] `rfc/short/rfc8654.md` - Section 6 (OPEN 4096-octet ceiling)
- [ ] `rfc/short/rfc8955.md` - Section 4 (FlowSpec zero-length next hop)
- [ ] `rfc/short/rfc9012.md`, `rfc/short/rfc9552.md`, `rfc/short/rfc7752.md` - Tunnel Encap, BGP-LS fixtures and length-sum validation

**Key insights:**
- `spec-rfc2545-readvertise-on-condition-flip.md` owns condition-flip re-advertisement (`reactor_readvertise.go`); this child owns only the receiving-interface scope guard in `peerOnLink`.
- AC-7 storage (`c0fad815db`), AS4 conversion (`30d31d0b49`), community rebuild (`313de90fee`), and Prefix-SID cleanup (`6c22530c23`) have already landed; only typed `Flags()` and peer-visible end-to-end wire proof remain here.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/peer.go` - `getPluginFamilies` returns global `r.api.GetDecodeFamilies()`; `raiseOpQueueFull` raises `op-queue-full` without a matching clear on drain or cleanup; `lowerReplayFence` has no timeout/owner-exit release.
- [ ] `internal/component/bgp/reactor/session_negotiate.go` - `offeredMultiprotocol` falls back to global `pluginFamilies()`; `buildOpen` lacks a 4096-octet cap refusal.
- [ ] `internal/component/bgp/reactor/recovery_owed.go`, `forward_bucket.go`, `relay_recovery.go`, `forward_rs.go` - recovery packing intermittently splits 300 prefixes across 10–21 UPDATEs in `TestRecoveryPackLiveDepartureOneUpdatePerFit`.
- [ ] `internal/component/bgp/reactor/forward_pool.go` - `safeBatchHandle` recovers panics without tearing down the affected session; `overflowPending` misses pool channel items.
- [ ] `internal/component/bgp/reactor/link_scope.go` - `peerOnLink` scope check for relayed IPv6 link-local next hops.
- [ ] `internal/component/bgp/reactor/reactor.go`, `listener.go`, `peer_run.go`, `reactor_dynamic.go` - shutdown accept/reconnect sealing, per-peer listener vs dynamic group inbound, dynamic group listener startup ordering, and `BGPRawCaptureSnapshot` second-granularity timestamps.
- [ ] `internal/component/bgp/reactor/session_prefix.go` - `checkPrefixLimits` offered count accounting on unknown withdrawals.
- [ ] `internal/component/bgp/reactor/reactor_api_batch.go` - `AnnounceNLRIBatch` FlowSpec zero-length next hop handling.
- [ ] `internal/component/bgp/reactor/reactor_api_relay.go`, `bufmux.go` - relay pool dry handling and `bufMuxStd`/`bufMuxExt` shared byte budget.
- [ ] `internal/component/bgp/reactor/reactor_api.go` - `recordCreatedPeerLocked` overwrites an operator-configured peer of the same name at a different address.
- [ ] `internal/component/bgp/reactor/operation.go` - `peerSettingsFromOperationConfig` calls `parsePeerFromTree` without setting `settings.Name = peerName`.
- [ ] `internal/component/bgp/reactor/update_delay.go`, `peer_static_wire.go`, `peer_rib_routes.go`, `session_bfd_strict.go`, `reactor_notify.go` - update-delay peer-up replay loss, DEBUG-only static send drop, unused `buildWithdrawNLRI`, `handleBFDEvent` holding `openMu`, and manual `RUnlock` under recover.
- [ ] `internal/component/bgp/wireu/split.go`, `aspath_transcode.go` - `buildCombinedUpdates` same-prefix withdraw+announce relay, and malformed `AS_PATH` transcode consuming NLRI bytes.
- [ ] `internal/component/bgp/format/text_human.go`, `internal/component/bgp/cli/decode_human.go`, `internal/core/bgp/attribute/builder_parse.go` - unsplit MP NLRI text formatting, `<state-event>` multi-word reason, human decode unknown attributes, and `ParseSingleExtCommunity` MUP forms.

**Behavior to preserve:**
- Zero-copy buffer-first wire encoding (`WriteTo(buf, off) int`), existing `ContextID` fast paths, RFC 8654 65535-octet UPDATE support, and all already-retired fixes listed in the parent umbrella.

**Behavior to change:**
- Only the 23 engine ACs listed in `## Acceptance Criteria` below.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Recovery admission and packing | `internal/component/bgp/reactor/recovery_owed.go` `recoveryWorker.decide`; `forward_bucket.go` `recoveryBucketMerge` | Creates one admission per lookup chunk and packs compatible updates within that admission | Extends: AC-2 repairs the packing recurrence while preserving route delivery and `TestRecoveryPackLiveDepartureOneUpdatePerFit` |
| Operator warnings on the report bus | `internal/core/report` `RaiseWarning`/`ClearWarning`; `reactor/peer.go` `Peer.raiseOpQueueFull`; `reactor/peer_run.go` `Peer.cleanup` | Raises lasting peer conditions under the peer address and clears `prefix-hold` on cleanup | Extends: AC-23 adds the missing clear half for `op-queue-full` on queue drain and `Peer.cleanup` |
| Buffer multiplexer (`bufMux`) | `internal/component/bgp/reactor/bufmux.go` | Allocates pooled 4K standard and 64K extended session read buffers under a shared byte budget | Extends: AC-14 lets an idle 64K pool yield blocks to 4K demand and makes peer-up replay wait instead of dropping routes |
| Wire UPDATE splitter & AS_PATH transcoder | `internal/component/bgp/wireu/split.go` `buildCombinedUpdates`; `wireu/aspath_transcode.go` `TranscodeASPath` | Splits mixed UPDATE payloads and transcodes 2-octet/4-octet AS_PATH | Extends: AC-5 relays same-prefix withdraw+announce as announce-only; AC-18 guards malformed AS_PATH bounds and 2-octet prepend |
| Stored Partial preservation (AC-7 phases 1–4) | `storage/attrparse.go` (`c0fad815db`), `wireu/aspath_collapse.go` (`30d31d0b49`), `filter_community/handler.go` (`313de90fee`), `reactor/filter_delta_handlers.go` & `forward_prefix_sid.go` (`6c22530c23`) | Preserves received Partial bits through storage, AS4 conversion, community rebuilds, and Prefix-SID cleanup | Extends: AC-7 finishes typed `Flags()` preservation and peer-visible end-to-end wire proof |
| Peer config parsing & API peer creation | `reactor/config.go` `PeersFromTree`, `parsePeerFromTree`; `reactor/operation.go` `peerSettingsFromOperationConfig`; `reactor/reactor_api.go` `recordCreatedPeerLocked` | Parses peer trees and records runtime-created peers in `r.configTree` | Extends: AC-16 sets `settings.Name` in `peerSettingsFromOperationConfig`; AC-22 refuses name collisions against operator peers |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | A plugin can run in another process | the door every change reads | "Every plugin, ours included, runs either internal (a goroutine in the daemon) or external (a forked subprocess that talks only over TLS and RPC). The SDK hides which. Code that only works internal is broken." | agrees | Every fix keeps full internal and external process-boundary parity with zero shared-memory shortcuts. |
| `docs/architecture/core-design.md` | Executive Summary | the // Design: page of internal/component/bgp/reactor/peer.go and 25 sibling engine files | "`ze` and `le` are personalities of the single `cmd/ze` codebase." | agrees | All repairs stay inside the modular component/plugin architecture without breaking component isolation or startup resilience. |
| `docs/architecture/api/architecture.md` | Implementation Status | anchored to internal/component/bgp/reactor/peer.go and sibling engine files | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP engine defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/api/capability-contract.md` | Implementation Status | anchored to internal/component/bgp/reactor/peer.go; anchored to internal/component/bgp/reactor/reactor.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP engine defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/api/commands.md` | Overview | anchored to internal/component/bgp/reactor/peer.go and sibling engine files | "Ze uses verb-first command paths with JSON or text encoding." | agrees | Command repairs preserve verb-first paths, YANG parameter validation, and shared answer-shape conversion. |
| `docs/architecture/api/ipc_protocol.md` | Overview | anchored to internal/component/bgp/reactor/peer.go; reactor.go; format/text_human.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP engine defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/api/json-format.md` | Overview | the // Design: page of internal/component/bgp/format/text_human.go and anchored reactor/server files | "Ze outputs JSON messages to external processes via stdout." | agrees | JSON event and decode repairs keep existing envelope keys intact. |
| `docs/architecture/api/process-protocol.md` | Overview | the // Design: page of internal/component/bgp/reactor/reactor_api_relay.go and anchored engine files | "Ze plugins communicate with the engine via newline-framed YANG RPCs" | agrees | All engine/plugin fixes stay on the newline-framed YANG RPC and DirectBridge contracts. |
| `docs/architecture/api/update-syntax.md` | Status | anchored to internal/component/bgp/reactor/peer.go; reactor_api_batch.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP engine defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/behavior/fsm-open-sent.md` | TL;DR | anchored to internal/component/bgp/reactor/peer.go; session_negotiate.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP engine defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/behavior/fsm.md` | TL;DR (Read This First) | anchored to internal/component/bgp/reactor/peer.go; reactor.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP engine defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/behavior/peer-lifecycle.md` | TL;DR | anchored to internal/component/bgp/reactor/peer.go; session_negotiate.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP engine defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/bgp/structural-forwarding.md` | The ordering constraint | anchored to internal/component/bgp/reactor/peer.go; recovery_owed.go; forward_rs.go | "**`extractWireNLRIRecords` MUST run before forwarding.**" | agrees | Recovery packing and RS forwarding fixes preserve the pre-forwarding extraction order and zero-loss route delivery. |
| `docs/architecture/config/syntax.md` | TL;DR (Read This First) | anchored to internal/component/bgp/reactor/peer.go; reactor.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP engine defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/encoding-context.md` | TL;DR (Read This First) | anchored to internal/component/bgp/reactor/peer.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP engine defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/forward-congestion-pool.md` | Invariant | the // Design: page of internal/component/bgp/reactor/forward_bucket.go and anchored pool files | "**Congestion does not silently discard a current route.**" | agrees | Dry-pool retry, `bufMux` 64K-to-4K yielding, and `overflowPending` pool-channel coverage enforce zero silent route loss. |
| `docs/architecture/meta/role.md` | Keys Set | anchored to internal/component/bgp/reactor/peer.go; reactor_dynamic.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP engine defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/plugin/plugin-system.md` | Where each layer lives | anchored to internal/component/bgp/reactor/peer.go; forward_pool.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP engine defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/plugin/rib-storage-design.md` | Overview | the // Design: page of internal/component/bgp/reactor/relay_recovery.go and anchored reactor files | "This is the **implementation reference** for Pool + Wire design in API programs:" | agrees | Recovery admission and relay fixes preserve the existing pool and wire ownership design. |
| `docs/architecture/testing/ci-format.md` | Syntax Overview | anchored to internal/component/bgp/reactor/peer.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP engine defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/update-building.md` | TL;DR (Read This First) | the // Design: page of internal/component/bgp/reactor/recovery_owed.go and anchored files | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP engine defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/wire/capabilities.md` | TL;DR (Read This First) | anchored to internal/component/bgp/reactor/peer.go; session_negotiate.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP engine defect pass; its documented invariants are preserved untouched. |
| `ai/doors/bgp-engine.md` | Boundaries | the door covering internal/component/bgp/reactor/* and sibling engine files | "**Encoding is buffer-first:** `WriteTo(buf, off) int` into a pooled buffer" | agrees | Every BGP engine wire fix preserves buffer-first encoding into pooled buffers and respects engine/plugin boundaries. |
| `docs/architecture/bgp/protocol-event-capture.md` | Enabling it | anchored to internal/component/bgp/reactor/session_negotiate.go; peer_run.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP engine defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/edge-cases/extended-message.md` | TL;DR | anchored to internal/component/bgp/reactor/session_negotiate.go; peer_run.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP engine defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/bgp/egress-attribute-rules.md` | The two rules | anchored to internal/component/bgp/reactor/recovery_owed.go; forward_rs.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP engine defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/testing/interop.md` | The suites, one per protocol area | anchored to internal/component/bgp/reactor/recovery_owed.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP engine defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/buffer-architecture.md` | Executive Summary | the // Design: page of internal/component/bgp/reactor/bufmux.go and anchored forward files | "Ze uses a **buffer-first** architecture where BGP messages are represented as byte buffers with iterators and partial parsers." | agrees | Every wire, `bufMux`, and relay fix keeps buffer-first encoding and bounded pooled lifetimes. |
| `docs/architecture/rib/unified-locrib.md` | Owner requirement: internal and external system RIB | anchored to internal/component/bgp/reactor/relay_recovery.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP engine defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/memory/lifetime-contracts.md` | TL;DR | anchored to internal/component/bgp/reactor/forward_rs.go; peer_run.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP engine defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/route-selection.md` | Overview | anchored to internal/component/bgp/reactor/forward_rs.go; forward_pool.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP engine defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/behavior/fsm-established.md` | TL;DR | anchored to internal/component/bgp/reactor/forward_pool.go; peer_run.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP engine defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/congestion-industry.md` | Sources | anchored to internal/component/bgp/reactor/forward_pool.go; reactor.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP engine defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/bgp/interface-event-reactions.md` | The decisions | anchored to internal/component/bgp/reactor/link_scope.go; reactor.go | "**Watch the kernel address table, not the interface bus topics.**" | agrees | Keeps `peerOnLink` receiving-interface scope aligned with the kernel neighbor table; condition-flip re-advertisement stays owned by `spec-rfc2545-readvertise-on-condition-flip.md`. |
| `docs/architecture/behavior/signals.md` | Supported Signals | anchored to internal/component/bgp/reactor/reactor.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP engine defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/config/apply-ordering.md` | The requirement | anchored to internal/component/bgp/reactor/reactor.go; reactor_dynamic.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP engine defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/config/environment.md` | Overview | anchored to internal/component/bgp/reactor/reactor.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP engine defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/overview.md` | 1. What is Ze? | anchored to internal/component/bgp/reactor/reactor.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP engine defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/rib-transition.md` | Executive Summary | anchored to internal/component/bgp/reactor/reactor.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP engine defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/behavior/fsm-active.md` | TL;DR | anchored to internal/component/bgp/reactor/peer_run.go; session_bfd_strict.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP engine defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/behavior/fsm-connect.md` | TL;DR | anchored to internal/component/bgp/reactor/peer_run.go; session_bfd_strict.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP engine defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/behavior/fsm-idle.md` | TL;DR | anchored to internal/component/bgp/reactor/peer_run.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP engine defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/diagnostics/event-history-rings.md` | The decisions | anchored to internal/component/bgp/reactor/peer_run.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP engine defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/mrt.md` | Packages | anchored to internal/component/bgp/reactor/peer_run.go; reactor_notify.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP engine defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/bgp/on-demand-origination.md` | The decisions | anchored to internal/component/bgp/reactor/peer_initial_sync.go | "**Announce uses the existing local build, export and socket path.**" | agrees | FlowSpec zero-length next hop batch announcements extend the existing origination path. |
| `docs/architecture/edge-cases/as4.md` | Overview | anchored to internal/component/bgp/reactor/reactor_dynamic.go; wireu/aspath_transcode.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP engine defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/bgp/replay-cursor.md` | The decisions | anchored to internal/component/bgp/reactor/reactor_api_batch.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP engine defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/wire/rfc7606-relay-shape.md` | The requirement | anchored to internal/component/bgp/reactor/reactor_api_batch.go; wireu/split.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP engine defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/api/text-format.md` | Current Format | a page this spec modifies; anchored to internal/component/bgp/format/text_human.go | "All parsed text events start with `peer <address> remote as <asn>`." | agrees | AC-6 and AC-21 update `text-format.md` in lockstep with `appendFilterResultText` and `<state-event>` multi-word reason handling. |
| `docs/architecture/config/transaction-protocol.md` | 1. Design Principles | anchored to internal/component/bgp/reactor/reactor_api.go; operation.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP engine defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/wire/nlri-bgpls.md` | MP_REACH next hop | anchored to internal/component/bgp/reactor/reactor_api.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP engine defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/behavior/fsm-open-confirm.md` | TL;DR | anchored to internal/component/bgp/reactor/session_bfd_strict.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP engine defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/exabgp-bridge.md` | The bridge translates in two directions | anchored to internal/component/bgp/reactor/reactor_notify.go; server/events.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP engine defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/api/text-coverage.md` | Message Type Coverage | anchored to internal/component/bgp/format/text_human.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP engine defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/api/text-parser.md` | Current Parser | anchored to internal/component/bgp/format/text_human.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP engine defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/wire/messages.md` | TL;DR (Read This First) | the // Design: page of internal/component/bgp/wireu/split.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP engine defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/wire/mp-nlri-ordering.md` | Key Insight | anchored to internal/component/bgp/wireu/split.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP engine defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/wire/attributes.md` | TL;DR (Read This First) | the // Design: page of internal/component/bgp/wireu/aspath_transcode.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP engine defect pass; its documented invariants are preserved untouched. |
| `ai/doors/le-tooling.md` | Boundaries | the door covering test/plugin/flowspec-batch-zero-nexthop.ci | "**A compiled Go fixture returns an error when an assertion fails** (`ai/patterns/functional-test.md`, `ai/rules/testing.md`)." | agrees | Every functional `.ci` test created or modified asserts on live state and fails closed on unmet assertions. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | Preserved unchanged: these BGP engine repairs touch no data-plane backend selector or VPP verifier. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | Preserved unchanged (as amended for `ze start` by the 2026-11-26 decision below): these BGP engine repairs touch no data-plane mismatch check. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | Preserved unchanged: these BGP engine repairs touch no VPP firewall verifier. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | Preserved unchanged: these BGP engine repairs touch no data-plane backend selector. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | Preserved unchanged: this one-off fix pass adds no session-level next-hop override and keeps export-chain next-hop rewriting intact. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | Preserved unchanged: export-chain next-hop authoring forms remain untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | Preserved unchanged: built-in export next-hop steps and config-compile tail append remain untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | Preserved unchanged: next-hop folding and raw-override provenance remain untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | Preserved unchanged: explicit next-hop validation on import and export chains remains untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | Preserved unchanged: locally originated FlowSpec batches preserve their originator-chosen zero-length next hop. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | Preserved unchanged: implied filter publication remains intact. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | "Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code. Examples are a web certificate name that does not resolve, a looking-glass `server` entry with no certificate to present, and a looking-glass address list that does not parse. Where a service has several endpoints, only the endpoint that cannot be served stays down and the others serve. A reload whose config cannot be served stays refused as a whole, and the running services keep what they serve. In Thomas's words (voice-transcribed): "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; asked about a reload, "reload stay refused"; asked "Should only that endpoint stay down while the others serve", "yes". The reason is that a daemon that does not start brings up no networking, so the operator cannot reach the box to correct it." | agrees | Honored directly: no configuration refusal added here (AC-11 OPEN > 4096, AC-21 peer/group collision) exits `ze start`; invalid peers stay down with a declared doctor code while commit, reload, and `ze config validate` still refuse. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | "The rule of 2026-11-23 above, that a configuration which holds a feature that programs only the Linux kernel while the interfaces run on VPP "is refused at daemon start", is superseded for the start: ze starts, that feature stays off, and `ze doctor` reports the mismatch with its fix. A commit, a reload and `ze config validate` still refuse such a configuration, by the same verdict as before. Thomas was asked whether the rule that ze always starts replaces the refusal at start, "(a) yes, ze starts, that feature stays off and ze doctor reports the mismatch, while commit and reload still refuse it; (b) no", and answered "2a" (voice-transcribed). The reason is the one of the row above: a daemon that does not start brings up no networking, so the operator cannot reach the box to correct the mismatch." | agrees | Honored directly: startup resilience holds across all BGP config checks added by this spec. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "When `\| implied` modifies an existing config line (such as inserting automatic members into a `filters [ ... ];` list), it keeps the line intact and places an inline `# implied: ...` comment on the same line after the statement naming what was added and why, rather than placing a comment only above the line or splitting the list across multiple lines. In Thomas's words (2026-11-23): "No, it should be on the same line and say what was added"." | agrees | Preserved unchanged: `\| implied` rendering is untouched by these BGP engine fixes. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "Everything on the bus is a participant named by its configuration path. A participant registers the messages it sends and receives, events and commands alike, and with nothing written that registration is what runs" | agrees | Preserved unchanged: all repairs stay on existing bus participant registrations. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A `send > to` list written on a participant replaces the destinations its messages have by registration; it never adds to them. Present and empty means nobody" | agrees | Preserved unchanged: bus destination filtering semantics are untouched. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A `receive > from` list limits who may address a participant. Absent means everybody; present and empty means nobody" | agrees | Preserved unchanged: bus receive-source filtering semantics are untouched. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A send and a receive allow-list of message names (`send > allow`, `receive > allow`) exist on every participant: absent means everything declared, empty means nothing. The bus enforces them when it receives the message" | agrees | Preserved unchanged: bus message allow-list enforcement is untouched. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "What is written on the participant and what is written in the `bus` section compile into one filter per participant. A disagreement between the two places, or between an allow-list and a destination on one participant, is refused at commit naming both lines" | agrees | Preserved unchanged: bus filter compilation is untouched. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "The bus does not arbitrate between participants: no exclusive-role rule, no commit refusal and no silent winner. A plugin that must not duplicate another limits itself" | agrees | Preserved unchanged: bus participant non-arbitration is untouched. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "One display shows every registered default beside what the operator wrote, the defaults a written list replaced, and the line that produced each effective entry" | agrees | Preserved unchanged: bus default/effective display is untouched. |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-08-21 | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | Preserved unchanged: no command path or pipe grammar changes here. |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-20 | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | Preserved unchanged: pipe override semantics are untouched. |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-23 | "A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does" | agrees | Preserved unchanged: pipe side-effect isolation is untouched. |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-23 | "Every engine operation names the message it carries or `none` with a reason, and a plugin identity is authorized by a profile compiled from its filter" | agrees | Preserved unchanged: engine operation message declarations and authorization profiles are untouched. |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-25 | "Engine-to-plugin event delivery never drops events on queue overflow; endpoints expect every subscribed network event losslessly" | agrees | Honored directly: dry-pool retry and lossless delivery preserve subscribed events without silent queue drops. |
| `docs/architecture/plugin/plugin-system.md` | Design decisions | decision of 2026-11-23 | "With no daemon running, a command that parses a config starts each declared external program in query mode, never its run loop, and validates against its declaration" | agrees | Preserved unchanged: Stage 1 declare-mode purity and offline schema probing are untouched. |
| `docs/architecture/update-building.md` | Design decisions | decision of 2026-11-23 | "Gate draft-simpson 0x0800 FlowSpec MP_REACH next-hop emission behind an explicit per-session opt-in (`session { flowspec-redirect-next-hop true; }`, default `false`), written automatically by `ze exabgp migrate` when a migrated neighbor uses Simpson actions or serves an API process on a FlowSpec family" | agrees | Honored directly: AC-15 emits RFC 8955 zero-length next hops for default FlowSpec batches while preserving explicit `flowspec-redirect-next-hop true` opt-in. |
| `docs/architecture/testing/interop.md` | Design decisions | decision of 2026-11-23 | "Each Docker interop suite and scenario checks only the kernel capabilities it actually requires (`Suite.KernelCapabilities` unioned with selected `ScenarioPlan.KernelCapabilities`); suites whose selected scenarios require none run without unrelated kernel features (such as SRv6 or IPsec)" | agrees | Preserved unchanged: new BGP interop scenarios declare only the kernel capabilities they require. |
| `docs/architecture/bgp/interface-event-reactions.md` | Design decisions | decision of 2026-11-22 | "Interface and neighbor change re-advertisement runs the first flip immediately, then coalesces bursts behind a per-peer cooldown with at most one queued trailing re-pass; BGP route flap dampening (RFC 2439) is a separate concept and, if ever implemented, must be an option off by default ("flap dampening should be an option and off by default as since the RFC it was found to not help. 1A is fine.")" | agrees | Preserved unchanged: RFC 2545 condition-flip re-advertisement and 1A burst coalescing remain owned by `spec-rfc2545-readvertise-on-condition-flip.md`. |
| `docs/architecture/exabgp-bridge.md` | Design decisions | decision of 2026-11-23 | "An ExaBGP script is an ordinary bus endpoint that runs the wrapper; the bridge config section and the internal bridge mode do not exist" | agrees | Preserved unchanged: ExaBGP scripts stay ordinary external bus endpoints. |
| `docs/architecture/exabgp-bridge.md` | Design decisions | decision of 2026-11-26 | "ExaBGP wrapper endpoints request `base64` (`plugin.FormatBase64`) UPDATE delivery at Stage 5 `ready` using Ze's positional counted wire grammar (`upd <flags> <local-as> <remote-as> <conn-id> <n>:<peer> <n>:<local> <n>:<router-id> <n>:<extra-addpath> <n>:<b64-body>`, zero key names, `0:` when absent) over pooled `WriteBatchFrame` `MuxConn` batching while keeping low-frequency control/session events as JSON; `ze exabgp plugin` renders hot IPv4/IPv6 unicast UPDATEs directly into a `[]byte` buffer from wire bytes and falls back to the wire-backed `Event` renderer for non-unicast families and error cases" | agrees | Preserved unchanged: ExaBGP base64 UPDATE transport is untouched. |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Wire OPEN/UPDATE/NOTIFICATION packets on peer TCP sessions (`reactor/session_*.go`).
- Operator API/CLI commands (`create bgp peer`, `ze bgp decode`, `announce` batches) and config load/reload (`reactor/operation.go`, `bgp/config/`).

### Transformation Path
1. Session ingress validates OPEN/UPDATE headers, prefix counts, and attributes (`message/`, `reactor/session_*.go`).
2. Reactor notifies plugins over structured/JSON/text rails (`reactor/reactor_notify.go`, `format/text_human.go`, `server/events.go`).
3. Forward pool, recovery worker, and wire splitter pack and transcode egress UPDATEs (`reactor/forward_*.go`, `recovery_owed.go`, `wireu/split.go`, `wireu/aspath_transcode.go`).
4. Session write loop transmits framed UPDATEs under the reactor clock and `bufMux` buffer pool.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Peer TCP wire ↔ BGP engine | Framed BGP OPEN/UPDATE/NOTIFICATION bytes (`wireu`, `message`, `reactor`) | No |
| BGP engine ↔ Plugins | EventBus / DirectBridge / text events (`server/events.go`, `format/text_human.go`) | No |
| Config / API ↔ Reactor | `ConfigOperation`, `recordCreatedPeerLocked`, `AnnounceNLRIBatch` | No |

### Changed-Wire Census

**Search evidence:** Searched `internal/component/bgp/{reactor,wireu,message,format,cli,server,config}` and `internal/core/bgp/attribute` for `buildOpen`, `buildCombinedUpdates`, `TranscodeASPath`, `AnnounceNLRIBatch`, `appendFilterResultText`, `Flags()`, and `BGPRawCaptureSnapshot`; positive controls confirmed in `wireu/split_one_field_per_message_test.go`, `wireu/aspath_transcode.go`, `reactor/session_negotiate.go`, `format/text_human.go`, and `reactor/reactor_api_batch.go`.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| BGP OPEN (RFC 8654 §6 4096-octet ceiling & per-peer MP capabilities) | writer | `internal/component/bgp/reactor/session_negotiate.go` `Session.buildOpen`, `offeredMultiprotocol` | Builds OPEN capabilities and serializes via `message.Open.WriteTo` | Change: enforce 4096-octet cap before send and offer per-peer configured families; preserve all valid <= 4096 OPENs |
| BGP OPEN (RFC 8654 §6 4096-octet ceiling & per-peer MP capabilities) | reader | External BGP peers (FRR, BIRD, GoBGP) and `internal/component/bgp/message/open.go` `UnpackOpen` | External peers reject > 4096 OPENs with 1/2 Bad Message Length and negotiate only advertised MP capabilities | Preserve: external peers and `UnpackOpen` already expect <= 4096 OPENs |
| BGP UPDATE (combined withdraw+announce, Partial bit, AS_PATH 2-octet transcode, FlowSpec zero-length next hop) | writer | `internal/component/bgp/wireu/split.go` `buildCombinedUpdates`; `wireu/aspath_transcode.go` `TranscodeASPath`; `reactor/reactor_api_batch.go` `AnnounceNLRIBatch`; `internal/core/bgp/attribute/` `Flags()` | Encodes relayed, transcoded, and batch-announced UPDATE wire bytes into pooled buffers | Change: drop duplicate withdrawn prefix when announced in same UPDATE, retain Partial on typed rebuilds, emit AS_TRANS/AS4_PATH on 2-octet prepend without overrunning NLRI, allow zero-length FlowSpec next hop |
| BGP UPDATE (combined withdraw+announce, Partial bit, AS_PATH 2-octet transcode, FlowSpec zero-length next hop) | reader | External BGP peers (FRR, BIRD, GoBGP) and `internal/component/bgp/message/update.go` `UnpackUpdate` | Peers parse standard RFC 4271 / 6793 / 8955 UPDATE frames | Preserve: fixes bring Ze output into strict RFC compliance for peers |
| Plugin text event format (`<state-event>` reason & unsplit MP NLRI) | writer | `internal/component/bgp/format/text_human.go` `appendFilterResultText`; `internal/component/bgp/format/text_update.go` | Formats human/text event lines emitted to text-mode plugins and ExaBGP bridge | Change: document and emit unsplit MP NLRI form and align `<state-event>` multi-word reason with BNF |
| Plugin text event format (`<state-event>` reason & unsplit MP NLRI) | reader | Text-mode external plugins, `internal/exabgp/` text parsers, and `docs/architecture/api/text-format.md` | Parsers consume text event tokens per `text-format.md` BNF | Change: update the three text parsers and `docs/architecture/api/text-format.md` in lockstep |

### Integration Points
- `internal/core/report` - warning raise/clear lifecycle for `op-queue-full`.
- `internal/core/diagnostic` - registered doctor codes for every new `Warn`/`Error` log line and unstarted-peer condition.
- `internal/component/bgp/server/events.go` - state event delivery to dynamic group subscribers.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | Verified at implementation per AC |
| No unintended coupling (components stay isolated) | No | All edits stay under `internal/component/bgp/` (non-plugin) and `internal/core/bgp/` |
| No duplicated functionality (extends existing, does not recreate) | No | Extends existing `bufMux`, `report`, `wireu`, and `PeersFromTree` mechanisms |
| Zero-copy preserved where applicable (refs, not copies) | No | Wire fixes keep `WriteTo(buf, off) int` buffer-first encoding |
| Registration over hardcoding, outbound | No | Doctor codes register via `diagnostic.Register`; no hardcoded feature switches |
| Registration over hardcoding, inbound | No | `getPluginFamilies` stops blindly enumerating all decode plugins for unconfigured peers |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Each open row reproduces at HEAD before its fix | Audited against HEAD source on 2026-11-26 | If a red-first test passes at HEAD, retire that row into `## Existing Mechanisms` with commit evidence | Red-first test per AC | unvalidated |
| A-2 | `spec-rfc2545-readvertise-on-condition-flip.md` handles condition-flip re-advertisement separately | Sibling spec is `ready` and edits `reactor_readvertise.go` | Coordinate edits on `link_scope.go` | Review against sibling spec before editing `link_scope.go` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Recovery packing fix alters route delivery order or triggers race under `-count=20` | `TestRecoveryPackLiveDepartureOneUpdatePerFit` fails or `-race` trips | Reproduce under `-race -count=20`, attribute root cause before editing, never weaken packing assertion |
| R-2 | Adding `buildOpen` 4096 refusal or name-collision refusal breaks `ze start` | Daemon exits at startup on invalid peer | Follow 2026-11-26 "Ze always starts": refuse at commit/reload/`ze config validate`, keep only the invalid peer down with doctor code at `ze start` |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Live BGP sessions dropped, routes mis-encoded, or recovery packing regressed |
| How is it reverted? | Single commit revert per AC increment |
| Who else touches this path? | `spec-rfc2545-readvertise-on-condition-flip.md`, `spec-bgp-ls-receiver-accept-and-limit-controls.md`, `spec-doctor-coverage-6-bgp-engine.md` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Peer OPEN construction with > 4096 bytes of capabilities | → | `Session.buildOpen` (`reactor/session_negotiate.go`) | `TestBuildOpenRefusesOver4096` and `.ci` verifying peer stays down with doctor code while healthy peers start |
| Received UPDATE withdrawing and announcing one prefix | → | `wireu.buildCombinedUpdates` (`wireu/split.go`) | `TestBuildCombinedUpdatesSamePrefixAnnounceOnly` |
| FlowSpec batch with zero-length next hop | → | `Reactor.AnnounceNLRIBatch` (`reactor/reactor_api_batch.go`) | `test/plugin/flowspec-batch-zero-nexthop.ci` |
| `create bgp peer` colliding with an existing operator peer name | → | `Reactor.recordCreatedPeerLocked` (`reactor/reactor_api.go`) | `TestRecordCreatedPeerRefusesOperatorNameCollision` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Peer configured without FlowSpec in `family` block negotiates OPEN (`Peer.getPluginFamilies`, old AC-4) | OPEN offers only the families configured for that peer plus documented auto-advertised defaults decided per peer; `flowspec-open-capability.ci` passes and a peer with no FlowSpec config offers no FlowSpec | Explicitly configured families and documented IPv4 unicast fallback remain advertised so standard peers establish normally |
| AC-2 | Route server relay convergence, withdrawal replacement, overflow suppression, and recovery departure packing (`recovery_owed.go`, `forward_bucket.go`, `forward_rs.go`, old AC-5) | `TestRecoveryPackLiveDepartureOneUpdatePerFit` delivers all 300 prefixes in 1 UPDATE under `-race -count=20`; `modify-oversize-suppress.ci`, `forward-overflow-two-tier.ci`, and `flowspec-redirect-check-refused.ci` are deterministic across repeated runs | Unchanged packing assertions and zero-loss route delivery remain enforced; no assertion weakening is permitted |
| AC-3 | Relayed IPv6 route with link-local next hop evaluated by `peerOnLink` (`reactor/link_scope.go`, old AC-10 scope half) | Link-local next hop is kept only for a destination peer sharing the receiving interface's common subnet per RFC 2545 Section 3, and stripped to global-only otherwise | Condition-flip re-advertisement of already-sent routes stays owned by sibling `spec-rfc2545-readvertise-on-condition-flip.md` |
| AC-4 | Received known optional transitive attribute with Partial=1 re-encoded through typed `Flags()` and relayed peer-to-peer (`internal/core/bgp/attribute/`, old AC-7 remaining wire proof) | End-to-end wire test observes Partial=1 preserved on the transmitted UPDATE (hex assertion) per RFC 4271 Section 5 | Complete attributes (Partial=0) stay Partial=0; explicit operator policies and unknown-attribute paths remain intact |
| AC-5 | Received UPDATE naming the same prefix in both withdrawn and NLRI fields (`wireu/split.go` `buildCombinedUpdates`, old AC-15) | Relayed UPDATE carries that prefix as an announcement only, across IPv4 unicast and MP families, per RFC 4271 Section 4.3 | Distinct withdrawn prefixes in the same UPDATE still emit withdrawals normally |
| AC-6 | Text-mode event contains an MP NLRI that did not split (`format/text_human.go` `appendFilterResultText`, old AC-13) | Text event formats the unsplit MP NLRI using the form documented in `docs/architecture/api/text-format.md` and logs a `Warn` with a declared doctor code | Split MP NLRIs retain their existing per-prefix text rendering |
| AC-7 | Inbound TCP accept or reconnect races against `Reactor.Stop` (`reactor/reactor.go`, `listener.go`, `peer_run.go`, old AC-18) | Once `Reactor.Stop` begins, no inbound accept or reconnect opens a new session; concurrent race test exits cleanly | Active sessions already tearing down complete their graceful Cease path |
| AC-8 | Peer receives N announcements, M withdrawals of unknown prefixes, then 1 announcement against prefix limit (`reactor/session_prefix.go`, old AC-21) | Unknown withdrawals leave the offered prefix count unchanged, so the (N+1)th distinct prefix trips the limit accurately | Withdrawals of prefixes actually held in the peer's set continue to decrement the count |
| AC-9 | Recovered panic in `reactorForwardRSSection` / `notifyMessageReceiver` (`reactor/forward_rs.go`, `reactor_notify.go`, old AC-23 RUnlock) | Every read lock on the path uses `defer r.mu.RUnlock()`, leaving no lock held after a recovered panic under `-race` | Normal lock acquisition and release semantics stay identical |
| AC-10 | Initial-sync EOR ordering in `TestMRTCollisionWinnerReservation` and raw capture timestamp precision (`reactor/peer_initial_sync.go`, `reactor/reactor.go` `BGPRawCaptureSnapshot`, old AC-24 non-MRT halves) | Initial-sync EOR follows the routes it brackets; `BGPRawCaptureSnapshot` formats timestamps with sub-second precision (RFC 3339 Nano) | Existing RFC 3339 parsers accept fractional seconds without breaking consumers |
| AC-11 | Peer configured with capabilities exceeding 4096 octets (`reactor/session_negotiate.go` `buildOpen`, old AC-30) | `buildOpen` refuses an OPEN over 4096 octets before transmit per RFC 8654 Section 6 with an operator error naming the capability set; commit/reload/`ze config validate` refuse, while `ze start` keeps only that peer down under a declared doctor code and `ze doctor` finding | Valid OPENs <= 4096 octets and extended UPDATEs up to 65535 octets remain unaffected |
| AC-12 | `TestHoldDownRefusesInboundAndEndsOnStop` (`reactor/peer_prefix_hold_test.go`, old AC-26) | Test arms its deadline before the pipe can close and passes `-count=50` in the full reactor package run | Existing hold-down behavior assertions remain unchanged |
| AC-13 | Dynamic group member connects during daemon startup (`reactor/reactor_dynamic.go`, `listener.go`, `server/events.go`, old AC-29) | Dynamic group listener binds only after plugins start, or replays session-up to late subscribers, so `attach-process-dynamic-group.ci` reliably observes the session-up event | Dialed static peers keep their existing `OnPostStartup` ordering |
| AC-14 | `ze bgp decode` human mode on UPDATE carrying aggregator, originator-id, cluster-list, atomic-aggregate, BGP-LS, or unknown attribute codes (`internal/component/bgp/cli/decode_human.go` `formatAttributesHuman`, old AC-36 human half) | Prints every present attribute, rendering unknown codes as code plus hex | Existing seven rendered attributes keep their exact field labels and formatting |
| AC-15 | `AnnounceNLRIBatch` announces a FlowSpec batch with no next hop (`reactor/reactor_api_batch.go`, old AC-37) | Emits the FlowSpec UPDATE with RFC 8955 zero-length next hop; functional `.ci` verifies the wire UPDATE | Explicit `session flowspec-redirect-next-hop true` redirect target behavior remains preserved |
| AC-16 | Idle 64K `bufMuxExt` pool under 4K `bufMuxStd` pressure and peer-up replay meeting a dry read-buffer pool (`reactor/bufmux.go`, `reactor_api_relay.go`, old AC-38 and AC-46) | Idle 64K blocks release budget so the 4K pool grows without killing sessions; peer-up replay waits and retries when pool is dry, delivering 100/100 routes with pool size 1 | Bounded memory cap remains enforced; no unbounded heap fallback is added |
| AC-17 | `SendRoutes` encounters a `Commit` error (`reactor/`, old AC-44 `SendRoutes`) | Returns the `Commit` error to the caller together with the true count of transmitted routes | Successful commits return nil error and exact route count |
| AC-18 | API add-peer constructed from reload config (`reactor/operation.go` `peerSettingsFromOperationConfig`, old AC-45) | Sets `settings.Name = peerName`, matching `PeersFromTree` | All other parsed fields in `parsePeerFromTree` remain unchanged |
| AC-19 | `ParseSingleExtCommunity` parses MUP extended communities (`internal/core/bgp/attribute/builder_parse.go`, old AC-47) | Parses every administrator form and sub-type defined by the MUP draft with an accurate doc comment | Existing `target`/`rt` and `origin`/`soo` formats remain unchanged |
| AC-20 | AS_PATH width handling: two-octet prepend and malformed AS_PATH transcoding (`wireu/aspath_transcode.go`, old AC-49) | Prepending a 4-octet ASN on a 2-octet session emits `AS_TRANS` plus `AS4_PATH` per RFC 6793 Section 4.2.2; malformed `AS_PATH` never reads past attribute bounds into NLRI bytes | Same-width 4-octet and valid 2-octet fast paths remain zero-copy |
| AC-21 | Multi-word `<state-event>` reason (`connection lost`), reactor races (`PeerSettings`, `sendCtxID`/`locRIB`), and engine population gaps (`overflowPending`, listener vs dynamic group, peer/group name collision) (old AC-51, AC-52, AC-53 engine halves) | `<state-event>` BNF and parsers agree on multi-word reasons; all cross-goroutine reads synchronize cleanly under `-race -count=20`; `overflowPending` includes pool channel items; per-peer listeners do not swallow dynamic group connections; peer/group name collisions refuse at validate/reload and isolate at `ze start` under a doctor code | Non-colliding peers, groups, and single-token reasons stay unchanged |
| AC-22 | `create bgp peer` whose derived name collides with an operator's configured peer at another address (`reactor/reactor_api.go` `recordCreatedPeerLocked`, old AC-61) | Refuses with what, why, and the exact fix, leaving the operator's running peer entry untouched | Creating a peer with a non-colliding name succeeds and updates running config as before |
| AC-23 | Open reactor lifecycle, fixture, and warning hygiene rows (`update_delay.go` peer-up replay, `peer_static_wire.go` send error, `op-queue-full` warning clear, `safeBatchHandle` panic teardown, `lowerReplayFence` timeout/owner-exit, `buildWithdrawNLRI` dead code, `handleBFDEvent` `openMu` lock order, `fwdBatchHandler` AIGP discard, engine impossible fixtures and weak RFC tags in `rfc9012_tunnel_encap_carry_test.go`, `rfc7752_length_sums_test.go`, RFC 9552, mpls-vpn, `originatedNextHopIsPeerOwn`, `RFC2545-3-1`, `RFC2545-3-3`, `RFC4271-5.1.2-3`, `injected-route-never-announced`, `api-peer-create-delete-rib`, `api-peer-save`) | Each producer bug is repaired with a red-first test: update-delay delivers peer-up replay; static send errors log at `Warn` with a doctor code instead of DEBUG-dropping later routes; `op-queue-full` clears on drain and `Peer.cleanup`; recovered batch panics tear down the session; `lowerReplayFence` clears on owner exit/timeout; `buildWithdrawNLRI` is wired or deleted; `handleBFDEvent` releases `openMu` before join; fixtures/tags prove valid RFC states (with `ze-test-judge` approval for any test change) | macOS-only GTSM socket setup (`silent-fall-through` 2026-09-27) waits until post-release per `ai/META-DECISIONS.md` ("Production is Linux") while remaining tracked here |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures a peer whose capability set exceeds 4096 octets alongside healthy peers | `config` → `reactor` → `Session.buildOpen` → doctor finding | `TestBuildOpenRefusesOver4096` and `.ci` verifying healthy peers establish |
| 2 | Announces a FlowSpec batch with no next hop over API | `AnnounceNLRIBatch` → `wireu` → TCP UPDATE | `test/plugin/flowspec-batch-zero-nexthop.ci` |
| 3 | Runs `ze bgp decode` on an UPDATE with AGGREGATOR, ORIGINATOR_ID, and unknown attributes | `cmd/ze` → `internal/component/bgp/cli/decode_human.go` | `TestFormatAttributesHumanAllAndUnknown` |
| 4 | Runs `create bgp peer` whose name collides with an operator peer | `cmd/peer` → `reactor_api.go` `recordCreatedPeerLocked` | `TestRecordCreatedPeerRefusesOperatorNameCollision` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| Red-first unit tests for AC-1..AC-23 | `internal/component/bgp/{reactor,wireu,format,cli,server,config}/*_test.go`, `internal/core/bgp/attribute/*_test.go` | Each engine AC fails before repair and passes under `-race` after | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| OPEN total message length (`buildOpen`) | 29–4096 | 4096 | 28 | 4097 |
| Prefix limit offered count (`session_prefix.go`) | 0–configured limit | limit | 0 (floor on unknown withdraw) | limit + 1 |
| `bufMux` shared byte budget | 1–configured cap | cap | 0 | cap + 1 (waits/yields) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `flowspec-batch-zero-nexthop.ci` | `test/plugin/flowspec-batch-zero-nexthop.ci` | FlowSpec batch announces with zero-length next hop | |
| Existing `.ci` stability runs | `test/plugin/flowspec-open-capability.ci`, `modify-oversize-suppress.ci`, `forward-overflow-two-tier.ci`, `flowspec-redirect-check-refused.ci`, `attach-process-dynamic-group.ci` | Determinism across repeated runs | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-rfc4271-partial-bit-frr` (or existing Partial/AS4 scenario) | `test/interop/scenarios/` | FRR | Received Partial bit survives relay on live wire (AC-4) | |
| `bgp-rfc4271-combined-withdraw-announce-frr` | `test/interop/scenarios/` | FRR | Combined withdraw+announce relayed as announce-only (AC-5) | |
| `bgp-rfc6793-two-octet-prepend-frr` | `test/interop/scenarios/` | FRR | 4-octet ASN prepend toward 2-octet peer emits AS_TRANS + AS4_PATH (AC-20) | |

## Files to Modify
- `internal/component/bgp/reactor/peer.go` - per-peer `getPluginFamilies`, `op-queue-full` clear, `lowerReplayFence` owner-exit/timeout
- `internal/component/bgp/reactor/session_negotiate.go` - `offeredMultiprotocol` per-peer filtering, `buildOpen` 4096 cap
- `internal/component/bgp/reactor/recovery_owed.go` - recovery chunk sizing and packing determinism
- `internal/component/bgp/reactor/forward_bucket.go` - recovery bucket merge
- `internal/component/bgp/reactor/relay_recovery.go` - recovery admission
- `internal/component/bgp/reactor/forward_rs.go` - RS forwarding and deferred `RUnlock`
- `internal/component/bgp/reactor/forward_pool.go` - `safeBatchHandle` session teardown on panic, `overflowPending` pool channel coverage
- `internal/component/bgp/reactor/link_scope.go` - `peerOnLink` receiving-interface scope check
- `internal/component/bgp/reactor/reactor.go` - shutdown sealing and sub-second `BGPRawCaptureSnapshot` timestamp
- `internal/component/bgp/reactor/listener.go` - shutdown accept seal and per-peer vs dynamic group dispatch
- `internal/component/bgp/reactor/peer_run.go` - shutdown reconnect seal and `op-queue-full` clear in `cleanup`
- `internal/component/bgp/reactor/session_prefix.go` - offered prefix count unchanged on unknown withdrawals
- `internal/component/bgp/reactor/peer_initial_sync.go` - initial-sync EOR ordering
- `internal/component/bgp/reactor/reactor_dynamic.go` - dynamic group listener startup ordering
- `internal/component/bgp/reactor/reactor_api_batch.go` - FlowSpec zero-length next hop
- `internal/component/bgp/reactor/reactor_api_relay.go` - peer-up replay wait/retry on dry pool
- `internal/component/bgp/reactor/reactor_api.go` - `recordCreatedPeerLocked` name collision refusal
- `internal/component/bgp/reactor/bufmux.go` - idle 64K pool releasing blocks for 4K pool
- `internal/component/bgp/reactor/operation.go` - `peerSettingsFromOperationConfig` sets `settings.Name`
- `internal/component/bgp/reactor/update_delay.go` - preserve peer-up replay under update-delay hold
- `internal/component/bgp/reactor/peer_static_wire.go` - `Warn` with doctor code on static send failure, do not silently abandon later routes
- `internal/component/bgp/reactor/peer_rib_routes.go` - wire or remove unused `buildWithdrawNLRI`
- `internal/component/bgp/reactor/session_bfd_strict.go` - release `openMu` before negotiation publication join
- `internal/component/bgp/reactor/reactor_notify.go` - deferred `RUnlock` under recover
- `internal/component/bgp/format/text_human.go` - unsplit MP NLRI formatting and logging
- `internal/component/bgp/wireu/split.go` - `buildCombinedUpdates` same-prefix announce-only
- `internal/component/bgp/wireu/aspath_transcode.go` - malformed AS_PATH bounds and 2-octet prepend
- `internal/component/bgp/cli/decode_human.go` - `formatAttributesHuman` all attributes + unknown code/hex
- `internal/component/bgp/server/events.go` - dynamic group subscriber replay / `<state-event>` reason
- `internal/core/bgp/attribute/builder_parse.go` - `ParseSingleExtCommunity` MUP forms and comment
- `docs/architecture/api/text-format.md` - document unsplit MP NLRI and `<state-event>` reason grammar

## Files to Create
- `test/plugin/flowspec-batch-zero-nexthop.ci` - end-to-end FlowSpec zero-length next hop batch test

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No new config leaves or RPCs |
| YANG validation constraints | No | No schema additions |
| YANG custom validators | No | Existing validation hooks cover peer/group name collision and OPEN size |
| CLI commands/flags | No | `ze bgp decode` output expands within existing command |
| CLI grammar (keyword before value) | No | No new verbs or keywords |
| Editor autocomplete | No | No schema additions |
| Functional test for new RPC/API | Yes | `test/plugin/flowspec-batch-zero-nexthop.ci` |
| Pipe completeness | No | Existing pipe paths unchanged |
| Env var registration | No | No new env vars |
| Doctor check for runtime dependencies | Yes | Any peer kept down at `ze start` (OPEN > 4096, peer/group name collision) reports via existing BGP doctor check under its declared code |
| Doctor code per problem class | Yes | Every new `Warn`/`Error` (`peer_static_wire.go`, `text_human.go`, `forward_pool.go`) and `op-queue-full` report warning carries a registered doctor code |
| Prometheus counters/metrics | No | Existing reactor counters remain sufficient |
| BGP family surface (new SAFI / capability / attribute) | No | No new family or attribute added |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|----------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Bug fixes only |
| 2 | Config syntax changed? | No | Existing syntax preserved |
| 3 | CLI command added/changed? | No | Existing `ze bgp decode` command |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/text-format.md` (AC-6, AC-21) |
| 5 | Plugin added/changed? | No | Engine only |
| 6 | Has a user guide page? | No | Existing guides unaffected |
| 7 | Wire format changed? | Yes | `docs/architecture/wire/messages.md` and `docs/architecture/wire/attributes.md` if behavior descriptions change |
| 8 | Plugin SDK/protocol changed? | No | Existing process protocol preserved |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc4271.md`, `rfc/short/rfc2545.md`, `rfc/short/rfc6793.md`, `rfc/short/rfc8654.md`, `rfc/short/rfc8955.md` |
| 10 | Test infrastructure changed? | No | Uses existing test runners |
| 11 | Affects daemon comparison? | No | No comparison change |
| 12 | Internal architecture changed? | No | Bug fixes within existing architecture |
| 13 | Route metadata keys added/changed? | No | No metadata key changes |
| 14 | Prometheus counters added/changed? | No | No new counters |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | No registration changes |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Verified via `./le spec citation anchors spec plan/release/fixes/spec-bgp-one-off-defects-1-bgp-engine.md`; `docs/architecture/api/text-format.md` updated, other declared pages checked and unaffected |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Verify `docs/architecture/api/text-format.md` examples match updated BNF |

## Design presentation

### What changes

This child spec repairs the 60 remaining open BGP engine defects under `ai/doors/bgp-engine.md`: per-peer OPEN family negotiation and RFC 8654 4096-octet OPEN cap (honoring 2026-11-26 "Ze always starts"), deterministic route-server recovery packing, RFC 4271 combined withdraw+announce deduplication, RFC 4271 Partial bit end-to-end wire proof, RFC 2545 `peerOnLink` receiving-interface scope, FlowSpec zero-length batch announcements, `bufMux` 64K-to-4K memory yielding and dry-pool retry, RFC 6793 two-octet AS_PATH prepend and bounds safety, `recordCreatedPeerLocked` name-collision protection, `op-queue-full` warning clearing, and engine race/shutdown/fixture hygiene.

### Before and after

1. **OPEN > 4096 octets (AC-11):** Before, `Session.buildOpen` serialized and transmitted an oversized OPEN, causing the remote peer to reset the session with NOTIFICATION 1/2. After, `ze config validate`, commit, and reload refuse the oversized capability set naming the offending capabilities; at `ze start`, only that peer stays unstarted with a declared doctor code in logs and `ze doctor` while healthy peers run.
2. **Same-prefix withdraw and announce (AC-5):** Before, `buildCombinedUpdates` relayed both a withdrawal and an announcement for the same prefix. After, the relayed UPDATE carries only the announcement across IPv4 unicast and MP families per RFC 4271 Section 4.3.
3. **`create bgp peer` name collision (AC-22):** Before, `recordCreatedPeerLocked` silently overwrote an operator-configured peer sharing the derived name at a different address. After, the command refuses with what collided, why, and the fix, leaving the operator peer intact.
4. **`op-queue-full` report warning (AC-23):** Before, `Peer.raiseOpQueueFull` raised a persistent warning that remained stuck in `show warnings` forever even after the queue drained or the peer shut down. After, queue drain below cap and `Peer.cleanup` clear the warning via `report.ClearWarning`.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Exit `ze start` on an oversized OPEN or peer/group name collision | Violates the 2026-11-26 "Ze always starts" ruling in `docs/architecture/core-design.md` |
| Weaken `TestRecoveryPackLiveDepartureOneUpdatePerFit` packing assertion | Violates `ai/rules/testing.md` and hides a real recovery packing regression |
| Add a second warning store for `op-queue-full` | Duplicates `internal/core/report` (`RaiseWarning`/`ClearWarning`) |

### Documents departed from

none

### Choices for Thomas

none

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** — reproduce each target AC with red-first unit and wiring tests (`TestBuildOpenRefusesOver4096`, `TestBuildCombinedUpdatesSamePrefixAnnounceOnly`, `TestRecordCreatedPeerRefusesOperatorNameCollision`, `test/plugin/flowspec-batch-zero-nexthop.ci`).
2. **Phase: Wire & OPEN compliance (AC-1, AC-4, AC-5, AC-11, AC-15, AC-19, AC-20)** — per-peer family advertisement, 4096 OPEN cap, combined withdraw+announce, Partial bit end-to-end wire proof, FlowSpec zero-length next hop, MUP extended communities, AS_PATH width/bounds.
3. **Phase: Reactor forwarding, recovery, and buffers (AC-2, AC-3, AC-7, AC-8, AC-9, AC-10, AC-13, AC-16, AC-21)** — recovery packing determinism, `peerOnLink`, shutdown sealing, prefix limit count, deferred `RUnlock`, EOR/capture timestamp, dynamic group listener, `bufMux` yielding & dry-pool retry, reactor races and population gaps.
4. **Phase: Operator surfaces, lifecycle hygiene, and engine fixtures (AC-6, AC-12, AC-14, AC-17, AC-18, AC-22, AC-23)** — `text-format.md` & `appendFilterResultText`, `decode_human.go`, `SendRoutes` Commit error, `peerSettingsFromOperationConfig` Name, `recordCreatedPeerLocked` collision guard, `op-queue-full` clear, `update_delay` replay, `peer_static_wire` Warn+code, `safeBatchHandle`, `lowerReplayFence`, `handleBFDEvent`, and engine unit fixtures/tags (via `ze-test-judge`).

## Work Not Done

- BGP plugin defects (`ai/doors/bgp-plugins.md`) are owned by `plan/release/fixes/spec-bgp-one-off-defects-2-bgp-plugins.md`.
- Functional `.ci` reds, Docker interop scenarios, and `rfc/short/` enrollment (`ai/doors/le-tooling.md`) are owned by `plan/release/fixes/spec-bgp-one-off-defects-3-tests-and-interop.md`.
- RFC 2545 condition-flip re-advertisement is owned by `plan/release/fixes/spec-rfc2545-readvertise-on-condition-flip.md`.
- Already-fixed defects (33 journal rows) are retired in `plan/release/fixes/spec-bgp-one-off-defects.md`.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `goroutine-waits-on-its-own-exit` | 2026-11-20 | peer-up replay lost while the update-delay hold owns the initial update |
| `silent-fall-through` | 2026-10-31 | static-route send error logged at DEBUG, later routes abandoned |
| `lock-held-across-a-join` | 2026-10-27 | handleBFDEvent holds openMu across negotiation publication |
| `injected-route-never-announced` | 2026-10-30 | filter-modified route never reaches the egress peer |
| `injected-route-never-announced` | 2026-10-30 | route added by a config reload never sent |
| `retained-entry-still-answers-lookups` | 2026-10-27 | api-peer-create-delete-rib |
| `package-red-not-reproduced` | 2026-10-27 | api-peer-save reload generation |
| `conflicting-feature-contracts` | 2026-10-04 | `Peer.getPluginFamilies` |
| `conflicting-feature-contracts` | 2026-10-22 | Reactor RS convergence tests |
| `derived-flags-overwrite-what-the-wire-carried` | 2026-09-20 | `Flags()` |
| `error-path-discards-data-already-received` | 2026-10-24 | `fwdBatchHandler` AIGP receipt discard |
| `escape-hatch-scoped-wider-than-its-justification` | 2026-10-04 | `peerOnLink` |
| `fixture-encodes-an-impossible-state` | 2026-10-05 | `rfc9012_tunnel_encap_carry_test.go` |
| `fixture-encodes-an-impossible-state` | 2026-08-28 | reactor receive path, RFC 9552 |
| `fixture-encodes-an-impossible-state` | 2026-10-14 | reactor tests, mpls-vpn decoder |
| `fixture-encodes-an-impossible-state` | 2026-09-06 | `originatedNextHopIsPeerOwn` |
| `format-cannot-hold-a-state-the-tree-holds` | 2026-10-21 | `appendFilterResultText` |
| `guard-added-to-one-half-of-a-pair` | 2026-10-21 | `buildCombinedUpdates` |
| `guard-added-to-one-half-of-a-pair` | 2026-10-06 | `TestRouteServerWithdrawalKeepsTheReplacementRoute` |
| `guard-added-to-one-half-of-a-pair` | 2026-08-11 | bgp reactor shutdown |
| `invariant-enforced-by-an-absent-call-site` | 2026-08-23 | `count offered` |
| `invariant-enforced-by-an-absent-call-site` | 2026-08-23 | RFC 2545 link-local next hop |
| `lock-released-by-hand-under-a-recover` | 2026-10-13 | `reactorForwardRSSection` |
| `marker-overtakes-the-queue-it-brackets` | 2026-10-13 | `TestMRTCollisionWinnerReservation` |
| `mtime-granularity-stamp` | 2026-09-05 | raw capture timestamp |
| `negative-test-asserts-the-verdict-not-the-reason` | 2026-10-05 | `rfc7752_length_sums_test.go` |
| `net-pipe-deadlock` | 2026-08-15 | `TestHoldDownRefusesInboundAndEndsOnStop` |
| `queued-request-waits-for-an-unrelated-trigger` | 2026-10-21 | `flowspec-redirect-check-refused.ci` |
| `session-up-before-its-subscriber` | 2026-08-15 | dynamic group listener |
| `silent-fall-through` | 2026-10-14 | `Session.buildOpen` |
| `bound-too-small-for-its-own-burst` | 2026-10-21 | `bufMuxStd`/`bufMuxExt` shared byte budget |
| `comment-describes-superseded-behaviour` | 2026-10-21 | `ParseSingleExtCommunity` `mup` case |
| `declared-format-contradicts-payload` | 2026-09-06 | `as-path-prepend` four-octet segment on a two-octet path |
| `declared-format-contradicts-payload` | 2026-10-05 | `TranscodeASPath` malformed AS_PATH consumes NLRI |
| `declared-format-contradicts-payload` | 2026-10-14 | `<state-event>` reason one token vs `connection lost` |
| `false-synchronization-claim` | 2026-08-23 | readers of mutable `PeerSettings` fields |
| `false-synchronization-claim` | 2026-10-05 | RIB `locRIB` and session `sendCtxID` races |
| `gate-excludes-part-of-its-population` | 2026-08-11 | forward rails `overflowPending` misses the pool channel |
| `gate-excludes-part-of-its-population` | 2026-08-12 | per-peer listener swallows a dynamic group's inbound |
| `gate-excludes-part-of-its-population` | 2026-08-13 | peer and group name namespaces can collide |
| `silent-fall-through` | 2026-09-27 | GTSM socket setup, macOS |
| `silent-fall-through` | 2026-08-19 | `ze bgp decode` human |
| `silent-fall-through` | 2026-09-06 | `AnnounceNLRIBatch` |
| `suppression-decision-not-deterministic` | 2026-08-14 | plugin relay |
| `suppression-decision-not-deterministic` | 2026-08-15 | plugin relay |
| `suppression-decision-not-deterministic` | 2026-08-21 | plugin relay |
| `suppression-decision-not-deterministic` | 2026-09-19 | plugin relay |
| `suppression-decision-not-deterministic` | 2026-09-24 | `forward-overflow-two-tier.ci` |
| `transient-failure-treated-as-fatal` | 2026-08-15 | bgp-rs peer-up replay |
| `reference-checked-claim-unchecked` | 2026-10-06 | RFC2545-3-1 `nextHopGlobalOctets` |
| `reference-checked-claim-unchecked` | 2026-10-06 | RFC2545-3-3 `egressNextHopGlobalHalf` |
| `reference-checked-claim-unchecked` | 2026-10-06 | RFC4271-5.1.2-3 `ASPathEdit.recordPrepend` |
| `unwired-feature` | 2026-10-26 | `buildWithdrawNLRI` left with no production caller |
| `validated-value-discarded-by-its-caller` | 2026-09-05 | `SendRoutes` discards `Commit` error |
| `zero-value-as-valid-answer` | 2026-10-10 | `peerSettingsFromOperationConfig` sets no `Name` |
| `bound-too-small-for-its-own-burst` | 2026-11-13 | AC-5: recovery departure packing, all routes received but UPDATE count exceeds fit |
| `guard-added-to-one-half-of-a-pair` | 2026-11-09 | `op-queue-full` report warning raised, never cleared |
| `two-owners-share-one-name` | 2026-10-10 | AC-61: `recordCreatedPeerLocked` overwrites an operator's peer of the same name |
| `silent-fall-through` | 2026-11-14 | `fwdPool.safeBatchHandle` keeps the session after a recovered batch panic, leaving a hole in what the peer was sent |
| `wait-outlives-the-process-it-waits-on` | 2026-11-14 | replay fence stays up when its owner never reports (`Peer.lowerReplayFence`) |

## Review Gate

Pending implementation and independent review.
