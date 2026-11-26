# Spec: BGP per-peer pre-policy received counter

| Field | Value |
|-------|-------|
| Status | design |
| Scope | plugin |
| Priority | P1 |
| Release reason | Operators who enable per-family `count installed` identity tracking need truthful pre-policy `routes-received` distinct from post-policy `routes-accepted` without paying default per-peer NLRI-key RAM. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Owner decisions (2026-11-24)

| Topic | Decision |
|-------|----------|
| AC-5 (`offered`/`installed` metric count-mode labels) vs AC-1..AC-4 (pre-policy received count) | Split AC-5 as a v1.0 release fix now so enforcement metrics identify their count mode (`offered` vs `installed`). Do not pay per-peer pre-policy NLRI-key RAM by default; populate pre-policy `routes-received` only when identity tracking is enabled via `count installed` or post-release `plan/post-release/features/spec-bgp-filtered-route-storage.md`. |

## Historical Context (2026-07-16 to 2026-08-08)

- **2026-07-16 pre-code audit:** Proved the original `prefixCounts` wire-event tally drifted upward under BGP implicit withdrawal (re-announcement) and could not be exposed directly as a pre-policy received count. Also established that `Peer` already uses lock-free atomics (`peer_stats.go`) for `reactorAPIAdapter.Peers()`. Two findings from that audit are now retired in HEAD: `session.go` fixed the stale `prefixCounts` initialization comment (old F-2), and `forEachPrefixEntry` (`session_prefix.go`) now uses registered `nlrisplit.Get` / `nlrisplit.GetWithdraw` framing for every family with no CIDR approximation (old R-3).
- **2026-08-07 / 2026-08-08 `count offered|installed` landing:** Thomas ruled that prefix-limit counting mode is an operator choice per family (`prefix { count offered|installed; }` in `internal/component/bgp/yang/ze-bgp-conf.yang`, default `offered`). `count installed` landed in `session_prefix.go` with per-family NLRI identity sets (`prefixCounts.sets`, keyed by registered `nlrisplit.GetPrefixKey` with ADD-PATH Path ID and RFC 8277 label-independent identity) and all-or-nothing refusal rollback (`rollbackPrefixSets`, `restoreInstalledPrefixCounts`).
- **2026-11-24 owner split:** Thomas split enforcement metric count-mode labels (old AC-5) into `plan/release/fixes/spec-bgp-prefix-count-mode-metric.md` and settled this spec's memory and source contract: pay zero default per-peer pre-policy NLRI-key RAM, and populate pre-policy `routes-received` only when identity tracking is enabled via `count installed` (or post-release `plan/post-release/features/spec-bgp-filtered-route-storage.md`).

## Task

Surface a truthful, lock-free per-peer pre-policy received prefix count in `show bgp`, `show bgp <afi/safi>`, and the birdwatcher Looking Glass (`/api/looking-glass/protocols/bgp`) whenever per-family identity tracking (`prefix { count installed; }`) is enabled, without allocating per-peer pre-policy NLRI sets under the default `count offered` mode and without changing post-policy `routes-accepted` / `routes_imported` semantics.

When `count installed` is enabled for a family, `Session.prefixCounts.sets[fk]` already maintains the exact set of distinct pre-policy prefixes admitted into the inbound pipeline before import policy runs (immune to implicit-withdraw inflation, route-refresh replay, and unmatched withdrawals, and rolled back atomically if prefix-limit enforcement refuses an UPDATE). Today that count lives only on the session read goroutine with no race-safe publication path to `reactorAPIAdapter.Peers()`, so `show bgp` (`summary.go`) always copies post-policy Adj-RIB-In size (`route-counts.in`) into both `routes-received` and `routes-accepted`, and birdwatcher (`lg/handler_api.go`) always reports `routes_received == routes_imported`.

Goal:
1. Publish each `count installed` family's set cardinality from `Session` to `Peer` via a fixed, zero-allocation-per-UPDATE atomic slot table allocated once per session only when at least one family configures `count installed` (nil under default `count offered`), cleared on session teardown.
2. Carry the lock-free snapshot on `plugin.PeerInfo.PrePolicyReceived` (`map[family.Family]int64`, nil when no family tracks `installed`) from `reactorAPIAdapter.Peers()`.
3. In `handleBgpSummary` (`summary.go`), preserve `routes-accepted` and `routes-sent` as Adj-RIB-In/Out sizes, populate `routes-received` from `PrePolicyReceived` with `routes-received-source: "pre-policy"` when `count installed` covers the requested scope (the requested family for `show bgp <afi/safi>`, or every negotiated family for unfiltered `show bgp`), and keep post-policy `routes-received` with `routes-received-source: "accepted"` otherwise (including default `count offered` and mixed-mode peers on unfiltered `show bgp`, so `routes-received` never mixes partial pre-policy counts with total accepted counts). Distinguish an available-but-empty `bgp-rib` from an absent `bgp-rib` so a `count installed` peer whose routes were 100% rejected by import policy still reports `routes-received = N` and `routes-accepted = 0`.
4. Keep `transformProtocols` (`lg/handler_api.go`) mapping `routes_received` from `routes-received` and `routes_imported` from `routes-accepted`, and tighten `routeCountsAvailable` to verify both `routes-received` and `routes-accepted` are numeric so a `count installed` peer without `bgp-rib` never misreports placeholder zeros as available.
5. Update `docs/architecture/api/birdwatcher-compat.md` (Section 3.2, Section 7.3, and Section 8), `docs/guide/command-reference.md`, and `docs/guide/looking-glass.md` to document the opt-in pre-policy semantics and `routes-received-source`.

Enforcement metric count-mode labels (former AC-5) were split to `plan/release/fixes/spec-bgp-prefix-count-mode-metric.md`. Retained import-filtered route storage remains post-release in `plan/post-release/features/spec-bgp-filtered-route-storage.md`, which may later also feed pre-policy counts when `keep-filtered` is enabled.

## Required Reading

<!-- NEVER tick [ ] to [x]. Capture insights as → Decision: / → Constraint: annotations. -->
- [ ] `ai/doors/system.md` - platform boundaries, cross-boundary value types, and registration rules
  → Constraint: `plugin.PeerInfo` crosses the engine-to-command boundary as a self-contained value snapshot with no pointer to session-owned maps.
- [ ] `ai/doors/bgp-engine.md` - BGP reactor concurrency and hot-path performance rules
  → Constraint: `Session.prefixCounts` is mutated only on the session read goroutine with no lock; cross-goroutine publication must be lock-free with zero per-NLRI and zero per-UPDATE heap allocation.
- [ ] `ai/doors/bgp-plugins.md` - command plugin boundaries
  → Constraint: `cmd/peer` (`summary.go`) reads reactor state only via `ReactorIntrospector.Peers()` and RIB state only via `ForwardToPlugin("show bgp rib status")`.
- [ ] `ai/doors/api-surfaces.md` - Looking Glass adapter boundaries and birdwatcher contract
  → Constraint: `internal/component/lg/handler_api.go` reads only the JSON returned by `show bgp` and must preserve `routes_counts_available` honesty.
- [ ] `docs/architecture/api/birdwatcher-compat.md` - normative birdwatcher API contract (Sections 3.2, 5, 7, 8)
  → Decision: this spec intentionally departs from Section 3.2's unconditional `route-counts.in` binding for `routes_received` and Section 7.3's unconditional "`routes_imported` MUST be expected to equal `routes_received`" for peers whose negotiated families enable `count installed`, and updates Sections 3.2, 7.3, and 8 in the same change.
- [ ] `docs/architecture/behavior/fsm-established.md` - UPDATE receive order and prefix-limit route identity
  → Constraint: `checkPrefixLimits` runs after RFC 7606 validation and `receiveCeiling`, and before plugin/filter dispatch (`reactor_notify.go`), so `count installed` reflects admitted pre-policy distinct prefixes.
- [ ] `rfc/short/rfc4271.md` Section 3.2 (Adj-RIB-In definition)
  → Constraint: Adj-RIB-In (`routes-accepted` / `routes_imported`) remains post-policy; the pre-policy count is an opt-in pre-filter distinct-prefix cardinality.

**Key insights:**
- `count installed` already pays for and maintains exact pre-policy distinct-prefix identity sets (`prefixCounts.sets`); reusing its cardinality via a fixed atomic slot table adds zero NLRI-key RAM by default and zero lock/allocation on the UPDATE hot path.
- `birdwatcher-compat.md` Section 3.2 and Section 7.3 currently bind `routes_received` unconditionally to `route-counts.in` (`routes_imported == routes_received`); changing that under `count installed` is a deliberate, documented departure.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/session_prefix.go` - `newPrefixCounts` allocates `pc.sets` only for families in `pc.installed` (from `PrefixCountInstalled`) or `pc.retained` (`rs-client` with limits); default `count offered` allocates no set. `applyInstalledPrefixSections` maintains `pc.sets[fk]` using registered route identity (`nlrisplit.GetPrefixKey`, ADD-PATH Path ID, RFC 8277 label-independent identity) before import policy, updates `pc.counts[fk]` to `len(pc.sets[fk])`, and rolls back completely on refusal via `rollbackPrefixSets` and `restoreInstalledPrefixCounts`.
- [ ] `internal/component/bgp/reactor/session_read.go` - `processMessage` calls `checkPrefixLimits(wireUpdate)` before `onMessageReceived` plugin/filter dispatch.
- [ ] `internal/component/bgp/reactor/reactor_notify.go` - import filter reject gate returns `false` before caching or dispatching rejected routes, so `bgp-rib` Adj-RIB-In holds only post-policy accepted routes.
- [ ] `internal/component/bgp/reactor/peer_stats.go` - `Peer` embeds lock-free atomic counters (`peerCounters`) read by `Peer.Stats()` without locking the session goroutine.
- [ ] `internal/component/bgp/reactor/reactor_api.go` - `reactorAPIAdapter.Peers()` builds `[]plugin.PeerInfo` under `a.r.mu.RLock()` from `Peer` atomic/guarded accessors and carries `NegotiatedFamilies` but no pre-policy count.
- [ ] `internal/component/plugin/types_bgp.go` - `PeerInfo` carries message counters and `NegotiatedFamilies`, with no pre-policy prefix count field.
- [ ] `internal/component/bgp/plugins/rib/rib_commands.go` - `RIBManager.status(famFilter)` builds `route-counts` (`in`, `out`) from `r.bgpPeers` (post-policy Adj-RIB-In and Adj-RIB-Out). A peer whose routes were all rejected has no `PeerRIB` entry in `r.bgpPeers`.
- [ ] `internal/component/bgp/plugins/cmd/peer/summary.go` - `fetchRibRouteCounts` returns `nil` both when `bgp-rib` is absent and when `route-counts` is empty; `mergeRibRouteCounts` sets both `routes-received` and `routes-accepted` to `c.in` and emits no source indicator.
- [ ] `internal/component/lg/handler_api.go` - `transformProtocols` already maps `"routes_received"` from `"routes-received"` and `"routes_imported"` from `"routes-accepted"`; `routeCountsAvailable` probes only `"routes-received"`.

**Behavior to preserve:**
- Under default `count offered`, zero per-peer pre-policy NLRI-key RAM is allocated, `PeerInfo.PrePolicyReceived` is `nil`, `show bgp` keeps `routes-received == routes-accepted`, and birdwatcher keeps `routes_received == routes_imported`.
- `routes-accepted` / `routes_imported` MUST remain the post-policy Adj-RIB-In size, and `routes-sent` / `routes_exported` MUST remain the Adj-RIB-Out size.
- When `bgp-rib` is not loaded, `routes-accepted` and `routes-sent` stay omitted (never faked to 0), and birdwatcher reports `routes_counts_available: false`.
- Prefix-limit enforcement on the session read goroutine takes no new lock and allocates nothing per UPDATE.

**Behavior to change:**
- When `count installed` is enabled, `Session` publishes per-family installed set cardinality through a lock-free atomic slot table on `Peer`, exposed on `PeerInfo.PrePolicyReceived`.
- `show bgp` and `show bgp <afi/safi>` populate `routes-received` from `PrePolicyReceived` with `routes-received-source: "pre-policy"` when `count installed` covers the requested scope (and `routes-received-source: "accepted"` when falling back to Adj-RIB-In), including peers whose routes were 100% rejected by import policy while `bgp-rib` is loaded.
- Birdwatcher `/api/looking-glass/protocols/bgp` surfaces `routes_received = N` (pre-policy) and `routes_imported = N - M` (post-policy) for peers whose negotiated families use `count installed`, and `routeCountsAvailable` checks both `routes-received` and `routes-accepted`.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Per-family `count installed` identity sets | `internal/component/bgp/reactor/session_prefix.go` `prefixCounts.sets`, `applyInstalledPrefixSections`, `rollbackPrefixSets`, `restoreInstalledPrefixCounts` | Maintains per-family distinct pre-policy NLRI identity sets (using `nlrisplit.GetPrefixKey`, ADD-PATH Path ID, RFC 8277 label-independent identity) only for families configured with `count installed`, with all-or-nothing rollback on prefix-limit refusal | Extends: reuses `len(pc.sets[fk])` directly as the pre-policy count source, avoiding any default per-peer NLRI-key RAM allocation |
| Lock-free per-peer stats publication | `internal/component/bgp/reactor/peer_stats.go` `peerCounters`, `Peer.Stats`; `internal/component/bgp/reactor/reactor_api.go` `reactorAPIAdapter.Peers` | Reads per-peer atomic counters into `[]plugin.PeerInfo` under `r.mu.RLock()` without touching session-owned state | Extends: adds an `atomic.Pointer[installedCountTable]` on `Peer` holding fixed per-family `atomic.Int64` slots (nil under `count offered`), read lock-free in `Peers()` |
| Peer summary route-count merge | `internal/component/bgp/plugins/cmd/peer/summary.go` `fetchRibRouteCounts`, `mergeRibRouteCounts`, `handleBgpSummary`; `internal/component/bgp/plugins/rib/rib_commands.go` `RIBManager.status` | Fetches family-scoped or aggregate post-policy Adj-RIB-In/Out counts from `bgp-rib` via `ForwardToPlugin` and merges `routes-received`, `routes-accepted`, `routes-sent` into each peer row | Extends: overlays `PeerInfo.PrePolicyReceived` onto `routes-received` when `count installed` covers the scope, adds `routes-received-source` (`pre-policy` vs `accepted`), and separates RIB availability from an empty peer table |
| Birdwatcher Looking Glass protocol transform | `internal/component/lg/handler_api.go` `transformProtocols`, `routeCountsAvailable`; `docs/architecture/api/birdwatcher-compat.md` Section 7 | Maps `routes-received` -> `routes_received`, `routes-accepted` -> `routes_imported`, and probes `routes-received` for `routes_counts_available` | Extends: keeps the existing field mapping (so pre-policy `routes-received` flows through automatically) and requires both `routes-received` and `routes-accepted` to be numeric in `routeCountsAvailable` |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | A plugin can run in another process | the door every change reads | "No pointer into data another plugin or component allocated, even when its package is shared core" | agrees | `PeerInfo.PrePolicyReceived` is a freshly copied `map[family.Family]int64` snapshot built in `reactorAPIAdapter.Peers()`, sharing no pointer with `Peer` or `Session`. |
| `ai/doors/bgp-engine.md` | Where it sits | the door covering `internal/component/bgp/reactor/*` | "The engine speaks the protocol and holds no policy." | agrees | Publishes only the cardinality of the pre-policy `count installed` set already maintained by `checkPrefixLimits`; adds no RIB or policy logic to the engine. |
| `ai/doors/plugin-infra.md` | Boundaries | the door covering `internal/component/plugin/types_bgp.go` | "No plugin imports another plugin's implementation." | agrees | `PeerInfo.PrePolicyReceived` uses `internal/core/family.Family` on `internal/component/plugin/types_bgp.go` without importing any BGP package into `internal/component/plugin/`. |
| `ai/doors/bgp-plugins.md` | Boundaries | the door covering `internal/component/bgp/plugins/cmd/peer/*` | "Reach the engine through the bus, DirectBridge or a text command, never a plain call into `reactor` or another component." | agrees | `summary.go` reads `PeerInfo.PrePolicyReceived` through `ReactorIntrospector.Peers()` and RIB counts through `ForwardToPlugin("show bgp rib status")`. |
| `ai/doors/api-surfaces.md` | Boundaries | the door covering `internal/component/lg/*` | "Never import a protocol package to read state." | agrees | `internal/component/lg/handler_api.go` continues reading only the `show bgp` JSON payload via `CommandDispatcher`. |
| `ai/doors/le-tooling.md` | Boundaries | the door covering `test/plugin/bgp-summary-received-prepolicy.ci` and `test/plugin/lg-received-prepolicy.ci` | "A compiled Go fixture returns an error when an assertion fails" | agrees | Both new `.ci` functional tests live under `test/plugin/`, follow the runner contract, and fail if pre-policy received and post-policy accepted counts disagree with expected values. |
| `docs/architecture/api/birdwatcher-compat.md` | 3.2 Which direction each endpoint answers | anchored to `summary.go`, `handler_api.go`, `handler_api_test.go`, `summary_fixture_test.go`; modified by this spec | "`route-counts.in` of `show bgp rib status`" | departs | Deliberately departs from the Section 3.2 table row binding both `routes_received` and `routes_imported` (Section 5) to `route-counts.in` of `show bgp rib status`: when all negotiated families enable `count installed`, `routes_received` comes from `show bgp`'s pre-policy `routes-received` (`PeerInfo.PrePolicyReceived`) rather than `bgp-rib`'s `route-counts.in`, while `routes_imported` remains `route-counts.in` and default `count offered` keeps both on `route-counts.in`. This spec updates Section 3.2 alongside Section 7.3 and Section 8 of `birdwatcher-compat.md` in the same change. |
| `docs/architecture/api/birdwatcher-compat.md` | 7.3 Known equalities and absences | anchored to `summary.go`, `handler_api.go`, `handler_api_test.go`, `summary_fixture_test.go`; modified by this spec | "`routes_imported` MUST be expected to equal `routes_received`. Both are the Adj-RIB-In size, because Ze drops rejected routes before storage and keeps no separate pre-policy count. A client MUST NOT infer a policy drop from the difference, which is always zero." | departs | Deliberately departs for peers whose negotiated families enable `count installed`: `routes_received` reports the pre-policy distinct-prefix count (`routes_received >= routes_imported`) while default `count offered` remains equal to `routes_imported`. This spec updates Sections 3.2, 7.3, and 8 of `birdwatcher-compat.md` in the same change. |
| `docs/architecture/core-design.md` | 18a. BGP Peer Reload: Swap or Restart | `// Design:` of `session_prefix.go`, `peer.go`, `peer_run.go`, `reactor_api.go` | "Every other field forces a restart, so a field added to `PeerSettings` tomorrow is restart-scoped until somebody classifies it on purpose." | agrees | `PrefixCount` stays restart-scoped, so each `Session`'s `installedCountTable` has a fixed family set for its entire lifetime and clears cleanly on teardown. |
| `docs/architecture/api/commands.md` | Overview | `// Design:` of `summary.go`, `peer.go`; anchored to `session_prefix.go`, `peer.go`, `peer_run.go`, `reactor_api.go`, `summary.go` | "Ze uses verb-first command paths with JSON or text encoding." | agrees | Keeps `show bgp` and `show bgp <afi/safi>` paths unchanged and adds kebab-case `routes-received-source` alongside existing route-count keys in `summary.go` and `registerColumns` (`cmd/peer/peer.go`). |
| `docs/architecture/behavior/fsm-established.md` | Prefix-limit route identity | anchored to `session_prefix.go`, `peer_run.go` | "The default `offered` mode counts announcement and withdrawal events using the family's registered framing. The `installed` mode counts a set of logical routes using the registered semantic key, not the complete NLRI bytes." | agrees | Reuses `installed` mode's exact set cardinality without modifying `offered` mode or allocating default NLRI sets. |
| `docs/architecture/behavior/fsm.md` | Peer lifecycle observers | anchored to `peer.go`, `peer_run.go`, `reactor_api.go` | "Reactor maintains a list of observers notified on state changes." | agrees | Does not alter FSM transitions, callbacks, or observer notifications. |
| `docs/architecture/behavior/peer-lifecycle.md` | `runOnce`: one session lifecycle | anchored to `peer.go`, `peer_run.go`, `reactor_api.go` | "`runOnce` publishes, drives and retires one selected `Session`." | agrees | Publishes `Peer.installedPrefixCounts` only inside `Peer.runOnce` when `p.session = session; published = true` (never in `peer_session_factory.go`, which also constructs unpublished incoming collision candidates that must not overwrite a live Established incumbent) and clears `Peer.installedPrefixCounts.Store(nil)` in `runOnce`'s teardown defer beside `p.negotiated.Store(nil)`. |
| `docs/architecture/api/architecture.md` | RIB Ownership | anchored to `peer.go`, `peer_run.go`, `reactor_api.go`, `types_bgp.go` | "API programs own all RIB data and logic." | agrees | Preserves `bgp-rib` ownership of Adj-RIB-In (`routes-accepted`) and Adj-RIB-Out (`routes-sent`), publishing only the engine's existing `count installed` cardinality. |
| `docs/architecture/api/process-protocol.md` | Overview | `// Design:` of `types_bgp.go`; anchored to `peer.go`, `peer_run.go`, `reactor_api.go`, `types_bgp.go` | "The protocol is the same for all invocation modes (internal goroutine, external subprocess)." | agrees | `PeerInfo.PrePolicyReceived` is used only by the in-daemon `ReactorIntrospector.Peers()` command snapshot and changes no plugin RPC or event wire format. |
| `docs/architecture/web-interface.md` | LG Data Access | `// Design:` of `handler_api.go`; anchored to `handler_api.go` | "The LG never imports RIB or peer plugin packages. The dispatcher routes commands to the engine, preserving plugin isolation." | agrees | `handler_api.go` keeps reading `show bgp` JSON over `CommandDispatcher` and updates `routeCountsAvailable` strictly from JSON fields. |
| `docs/architecture/plugin/rib-storage-design.md` | Route-server pre-policy Adj-RIB-In retention (RFC 7947 Section 2.1) | anchored to `session_prefix.go`, `peer.go`, `peer_run.go` | "For `rs-client true` peers with a configured prefix maximum in offered mode, `session_prefix.go` maintains a parallel unique retained key set so unmatched withdrawals cannot grow retained entries beyond the configured maximum." | agrees | `installedCountTable` tracks only `pc.installed` families and never touches or publishes `pc.retained`, keeping RFC 7947 route-server offered-mode retention strictly separate from `PrePolicyReceived`. |
| `docs/architecture/api/capability-contract.md` | Implementation Status | anchored to `peer.go` | - | silent | Covers BGP capability negotiation and OPEN injection; silent on summary route counts. |
| `docs/architecture/api/ipc_protocol.md` | Overview | anchored to `peer.go` | - | silent | Covers plugin IPC framing; silent on `show bgp` route counts. |
| `docs/architecture/api/json-format.md` | Field naming | anchored to `peer.go`, `handler_api.go` | "JSON keys are lowercase kebab-case, never camelCase and never snake_case." | agrees | The new `show bgp` peer-summary key `routes-received-source` uses lowercase kebab-case, while `internal/component/lg/handler_api.go` keeps its documented birdwatcher `snake_case` exemption (`routes_received`, `routes_imported`, `routes_counts_available`). |
| `docs/architecture/api/update-syntax.md` | Status | anchored to `peer.go` | - | silent | Covers `send bgp ... update text` grammar; silent on pre-policy received counters. |
| `docs/architecture/behavior/fsm-open-sent.md` | TL;DR | anchored to `peer.go`, `peer_run.go` | - | silent | Covers OpenSent FSM state; silent on Established pre-policy route counts. |
| `docs/architecture/bgp/structural-forwarding.md` | Four costs, and what replaced them | anchored to `peer.go` | - | silent | Covers forwarding pool dedup; silent on `show bgp` route counts. |
| `docs/architecture/config/syntax.md` | TL;DR (Read This First) | anchored to `peer.go` | - | silent | Covers configuration syntax; this spec adds no new config syntax. |
| `docs/architecture/encoding-context.md` | TL;DR (Read This First) | anchored to `peer.go` | - | silent | Covers `EncodingContext` and `ContextID`; silent on summary route counts. |
| `docs/architecture/forward-congestion-pool.md` | Invariant | anchored to `peer.go` | - | silent | Covers forward pool weighting and congestion backpressure; silent on summary route counts. |
| `docs/architecture/meta/role.md` | Keys Set | anchored to `peer.go` | - | silent | Covers RFC 9234 role metadata keys; silent on summary route counts. |
| `docs/architecture/plugin/plugin-system.md` | Cross-boundary value types | anchored to `peer.go`, `peer_run.go`, `types_bgp.go` | "A payload that crosses a plugin or component boundary is a self-contained value type. A shared type definition such as `family.Family` or `RouteChangeBatch` is a compile-time contract and stays allowed. What is forbidden is one plugin holding a pointer to data another plugin allocated." | agrees | `PeerInfo.PrePolicyReceived` copies `map[family.Family]int64` by value in `reactorAPIAdapter.Peers()`, so `cmd/peer` holds no pointer to session- or peer-allocated tables. |
| `docs/architecture/testing/ci-format.md` | Syntax Overview | anchored to `peer.go` | - | silent | Covers `.ci` runner directives; silent on summary route counts. |
| `docs/architecture/update-building.md` | TL;DR (Read This First) | anchored to `peer.go` | - | silent | Covers outbound UPDATE construction; silent on inbound pre-policy route counts. |
| `docs/architecture/wire/capabilities.md` | TL;DR (Read This First) | anchored to `peer.go` | - | silent | Covers wire capability encoding; silent on summary route counts. |
| `docs/architecture/behavior/fsm-active.md` | TL;DR | anchored to `peer_run.go` | - | silent | Covers Active FSM state; silent on pre-policy route counts. |
| `docs/architecture/behavior/fsm-connect.md` | TL;DR | anchored to `peer_run.go` | - | silent | Covers Connect FSM state; silent on pre-policy route counts. |
| `docs/architecture/behavior/fsm-idle.md` | TL;DR | anchored to `peer_run.go` | - | silent | Covers Idle FSM state; silent on pre-policy route counts. |
| `docs/architecture/bgp/protocol-event-capture.md` | Enabling it | anchored to `peer_run.go` | - | silent | Covers protocol event capture files; silent on summary route counts. |
| `docs/architecture/diagnostics/event-history-rings.md` | The decisions | anchored to `peer_run.go` | - | silent | Covers diagnostic event rings; silent on summary route counts. |
| `docs/architecture/edge-cases/extended-message.md` | TL;DR | anchored to `peer_run.go` | - | silent | Covers RFC 8654 extended message buffers; silent on summary route counts. |
| `docs/architecture/memory/lifetime-contracts.md` | TL;DR | anchored to `peer_run.go` | - | silent | Covers pooled read buffer lifetime contracts; silent on atomic integer count publication. |
| `docs/architecture/mrt.md` | Packages | anchored to `peer_run.go` | - | silent | Covers MRT dumps; silent on summary route counts. |
| `docs/architecture/api/text-format.md` | Current Format | anchored to `reactor_api.go` | - | silent | Covers text event format; silent on `show bgp` JSON route counts. |
| `docs/architecture/config/transaction-protocol.md` | 1. Design Principles | anchored to `reactor_api.go` | - | silent | Covers config verify/apply/rollback protocol; silent on operational route counts. |
| `docs/architecture/wire/nlri-bgpls.md` | MP_REACH next hop | anchored to `reactor_api.go` | - | silent | Covers BGP-LS NLRI wire format; silent on summary route counts. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | Preserves kernel/VPP mismatch validation untouched; BGP summary counts are data-plane agnostic. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | Preserves the unified mismatch refusal path untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | Preserves VPP firewall verification untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | Preserves data-plane selector naming untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | Preserves egress next-hop chain semantics untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | Preserves `policychain.Sets` next-hop authoring untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | Preserves built-in next-hop engine steps untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | Preserves next-hop provenance folding untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | Preserves explicit next-hop validation untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | Preserves originated vs forwarded next-hop behavior untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | Preserves the implied-filter seam untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | "Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code. Examples are a web certificate name that does not resolve, a looking-glass `server` entry with no certificate to present, and a looking-glass address list that does not parse. Where a service has several endpoints, only the endpoint that cannot be served stays down and the others serve. A reload whose config cannot be served stays refused as a whole, and the running services keep what they serve. In Thomas's words (voice-transcribed): "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; asked about a reload, "reload stay refused"; asked "Should only that endpoint stay down while the others serve", "yes". The reason is that a daemon that does not start brings up no networking, so the operator cannot reach the box to correct it." | agrees | Preserves daemon start resilience and reload refusal semantics untouched; pre-policy count publication adds no startup refusal. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | "The rule of 2026-11-23 above, that a configuration which holds a feature that programs only the Linux kernel while the interfaces run on VPP "is refused at daemon start", is superseded for the start: ze starts, that feature stays off, and `ze doctor` reports the mismatch with its fix. A commit, a reload and `ze config validate` still refuse such a configuration, by the same verdict as before. Thomas was asked whether the rule that ze always starts replaces the refusal at start, "(a) yes, ze starts, that feature stays off and ze doctor reports the mismatch, while commit and reload still refuse it; (b) no", and answered "2a" (voice-transcribed). The reason is the one of the row above: a daemon that does not start brings up no networking, so the operator cannot reach the box to correct the mismatch." | agrees | Preserves start vs commit/reload mismatch behavior untouched. |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-08-21 | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | Uses existing `show bgp` and `show bgp <afi/safi>` command paths and adds no scope/presentation subcommand. |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-20 | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | Adds no pipe overrides and preserves `show bgp` answer data shape. |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-23 | "A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does" | agrees | Preserves pipe semantics untouched; `routes-received` and `routes-received-source` are plain peer-row fields rendered by the shared pipeline. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "When `\| implied` modifies an existing config line (such as inserting automatic members into a `filters [ ... ];` list), it keeps the line intact and places an inline `# implied: ...` comment on the same line after the statement naming what was added and why, rather than placing a comment only above the line or splitting the list across multiple lines. In Thomas's words (2026-11-23): "No, it should be on the same line and say what was added"." | agrees | Preserves `\| implied` rendering untouched. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "Everything on the bus is a participant named by its configuration path. A participant registers the messages it sends and receives, events and commands alike, and with nothing written that registration is what runs" | agrees | Preserves bus participant registration untouched. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A `send > to` list written on a participant replaces the destinations its messages have by registration; it never adds to them. Present and empty means nobody" | agrees | Preserves `send > to` semantics untouched. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A `receive > from` list limits who may address a participant. Absent means everybody; present and empty means nobody" | agrees | Preserves `receive > from` semantics untouched. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A send and a receive allow-list of message names (`send > allow`, `receive > allow`) exist on every participant: absent means everything declared, empty means nothing. The bus enforces them when it receives the message" | agrees | Preserves bus allow-list enforcement untouched. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "What is written on the participant and what is written in the `bus` section compile into one filter per participant. A disagreement between the two places, or between an allow-list and a destination on one participant, is refused at commit naming both lines" | agrees | Preserves bus filter compilation untouched. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "The bus does not arbitrate between participants: no exclusive-role rule, no commit refusal and no silent winner. A plugin that must not duplicate another limits itself" | agrees | Preserves bus non-arbitration untouched. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "One display shows every registered default beside what the operator wrote, the defaults a written list replaced, and the line that produced each effective entry" | agrees | Preserves effective-bus display untouched. |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-23 | "Every engine operation names the message it carries or `none` with a reason, and a plugin identity is authorized by a profile compiled from its filter" | agrees | Adds no engine operation and preserves plugin authorization profiles untouched. |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-25 | "Engine-to-plugin event delivery never drops events on queue overflow; endpoints expect every subscribed network event losslessly" | agrees | Preserves lossless event delivery untouched. |
| `docs/architecture/plugin/plugin-system.md` | Design decisions | decision of 2026-11-23 | "With no daemon running, a command that parses a config starts each declared external program in query mode, never its run loop, and validates against its declaration" | agrees | Preserves offline query-mode schema validation untouched. |
| `docs/architecture/update-building.md` | Design decisions | decision of 2026-11-23 | "Gate draft-simpson 0x0800 FlowSpec MP_REACH next-hop emission behind an explicit per-session opt-in (`session { flowspec-redirect-next-hop true; }`, default `false`), written automatically by `ze exabgp migrate` when a migrated neighbor uses Simpson actions or serves an API process on a FlowSpec family" | agrees | Preserves FlowSpec MP_REACH next-hop gating untouched. |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Inbound BGP UPDATE messages arrive on the session read goroutine (`session_read.go` -> `checkPrefixLimits` in `session_prefix.go`), and operators or Looking Glass clients invoke `show bgp`, `show bgp <afi/safi>`, or `GET /api/looking-glass/protocols/bgp`.
- Format at entry: wire UPDATE body (`*wireu.WireUpdate`) on ingest; text command dispatched to `handleBgpSummary` and HTTP GET handled by `handleAPIProtocols` on read.

### Transformation Path
1. **Session construction and publication (`session_prefix.go`, `peer_run.go`):** `newPrefixCounts(settings)` resolves `pc.installed`. Only when `len(pc.installed) > 0` does `newInstalledCountTable(pc.installed)` allocate one fixed slice of `{fam family.Family; fk uint32; count atomic.Int64}` slots (one per `count installed` family, initialized to 0) on `session.installedCounts`. Under default `count offered`, the table is `nil` (zero extra RAM). `Peer.runOnce` (`peer_run.go`) stores `p.installedPrefixCounts.Store(session.installedCounts)` only when publishing the selected `Session` under `p.mu` (`p.session = session; published = true`) — never in `peer_session_factory.go`, because `newSessionWithEpoch` also constructs unpublished incoming collision candidates (`newIncomingSession`) that must not overwrite a live Established incumbent. `runOnce`'s teardown defer clears `p.installedPrefixCounts.Store(nil)` beside `p.negotiated.Store(nil)` before clearing `p.session`.
2. **Inbound UPDATE settlement (`session_prefix.go`):** `applyInstalledPrefixSections` updates `pc.sets[fk]` and `pc.counts[fk]` (once per family per UPDATE, after refusal check). Whenever `pc.counts[fk]` moves (including `restoreInstalledPrefixCounts` on refusal rollback), `Session` stores the new count into that family's `atomic.Int64` slot in `installedCountTable` (one atomic store, zero lock, zero allocation). Messages refused by prefix-limit enforcement or earlier RFC 7606 / `receiveCeiling` checks are rolled back and do not inflate the count; routes subsequently rejected by import policy (`reactor_notify.go`) remain in `pc.sets[fk]`, so the slot holds `N` while Adj-RIB-In holds `N - M`.
3. **Reactor snapshot (`reactor_api.go`):** `reactorAPIAdapter.Peers()` calls `p.installedPrefixCountsSnapshot()`, which loads `p.installedPrefixCounts.Load()` lock-free and copies each slot's `count.Load()` into a fresh `map[family.Family]int64` stored on `plugin.PeerInfo.PrePolicyReceived` (or `nil` when the pointer is `nil`). `Session.prefixCounts` is never read outside the session read goroutine.
4. **Summary assembly (`summary.go`, `cmd/peer/peer.go`):** `handleBgpSummary` calls `fetchRibRouteCounts(ctx, familyFilter)` returning `(ribCounts, ribAvailable)` (where `ribAvailable` is true whenever `show bgp rib status` answered `StatusDone`, even if `route-counts` is empty). For each peer row:
   - `mergeRibRouteCounts` populates `routes-accepted = c.in`, `routes-sent = c.out`, `routes-received = c.in`, and `routes-received-source = "accepted"` when the peer has a RIB entry.
   - `mergePrePolicyReceived` checks whether `p.PrePolicyReceived` covers the requested scope: for `show bgp <afi/safi>`, `famFilter` is a key in `p.PrePolicyReceived`; for unfiltered `show bgp`, `len(p.NegotiatedFamilies) > 0` and every negotiated family is a key in `p.PrePolicyReceived` (summing them). When covered, it sets `routes-received` to the pre-policy count and `routes-received-source = "pre-policy"`; if `ribAvailable` is true and the peer had no RIB entry (100% of its routes were rejected by import policy), it also sets `routes-accepted = 0` and `routes-sent = 0`. When `ribAvailable` is false (`bgp-rib` absent), `routes-accepted` and `routes-sent` stay omitted.
   - `registerColumns` (`cmd/peer/peer.go`) places `"routes-received-source"` beside `"routes-received"` in `cmdBgp`'s `ColumnOrder`.
5. **Looking Glass (`lg/handler_api.go`):** `transformProtocols` reads `"routes-received"` into `"routes_received"` and `"routes-accepted"` into `"routes_imported"`. `routeCountsAvailable` checks that both `"routes-received"` and `"routes-accepted"` parse as numbers, keeping `routes_counts_available` false whenever `bgp-rib` is absent.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Session read goroutine <-> Reactor API (`Peers`) | Lock-free `atomic.Pointer[installedCountTable]` + per-family `atomic.Int64` slots; never reads session-owned map | [ ] |
| Reactor <-> `cmd/peer` summary plugin | Self-contained `map[family.Family]int64` copy on `plugin.PeerInfo.PrePolicyReceived` | [ ] |
| `cmd/peer` summary <-> `bgp-rib` plugin | Existing `ForwardToPlugin("show bgp rib status")` JSON response | [ ] |
| Summary JSON <-> Looking Glass | Kebab-case `routes-received` / `routes-accepted` read by `transformProtocols` and `routeCountsAvailable` | [ ] |

### Changed-Wire Census

**Search evidence:** Searched `internal/component/bgp/plugins/cmd/peer/`, `internal/component/lg/`, `docs/architecture/api/birdwatcher-compat.md`, and `test/` (`git grep routes-received` and `git grep routes_received`). Positive control: `mergeRibRouteCounts` (`summary.go`) is the sole producer of `routes-received`/`routes-accepted` in `show bgp`, and `registerColumns` (`cmd/peer/peer.go`) orders them for table/text rendering; `transformProtocols` (`lg/handler_api.go`), `lg/summary_fixture_test.go`, `lg/summary_producer_test.go`, and `test/plugin/bgp-summary-route-counts.ci` are the in-repo readers; CLI/REST/gRPC/MCP/Alice-LG clients are outside-repo readers. No BGP protocol wire format or plugin IPC RPC wire format changes; only the `show bgp` JSON row gains optional additive key `routes-received-source` and pre-policy value for `routes-received` under `count installed`.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| `show bgp` JSON peer row (`routes-received`, `routes-received-source`) | writer | `internal/component/bgp/plugins/cmd/peer/summary.go` (`handleBgpSummary`, `mergeRibRouteCounts`, `mergePrePolicyReceived`), `internal/component/bgp/plugins/cmd/peer/peer.go` (`registerColumns`) | `git grep routes-received` confirmed `summary.go` is the sole producer and `peer.go` declares column order | Change: populate `routes-received` from pre-policy `PrePolicyReceived` with `routes-received-source: "pre-policy"` when `count installed` covers scope; preserve `routes-received == routes-accepted` with `routes-received-source: "accepted"` under default `count offered`; place `"routes-received-source"` beside `"routes-received"` in `registerColumns` |
| `show bgp` JSON peer row (`routes-received`, `routes-received-source`) | reader | `internal/component/lg/handler_api.go` (`transformProtocols`, `routeCountsAvailable`), `internal/component/lg/summary_fixture_test.go`, `internal/component/lg/summary_producer_test.go` (`TestMockSummaryMatchesRegisteredProducer`), `test/plugin/bgp-summary-route-counts.ci`, `test/plugin/bgp-summary-received-prepolicy.ci` | `git grep routes-received` found all in-repo JSON readers | Preserve & extend: `transformProtocols` already reads `routes-received` and `routes-accepted` separately; `routeCountsAvailable` requires both to be numeric; `summary_producer_test.go` strips `"routes-received-source"` alongside `"routes-received"`, `"routes-accepted"`, `"routes-sent"` in its mock-vs-producer key check; existing default-mode tests stay green because default `count offered` keeps `routes-received == routes-accepted` |
| Birdwatcher `/api/looking-glass/protocols/bgp` JSON (`routes_received`, `routes_imported`, `routes_counts_available`) | writer | `internal/component/lg/handler_api.go` (`transformProtocols`, `routeCountsAvailable`) | `git grep routes_received` confirmed `transformProtocols` (and BMP placeholder in `transformBMPProtocols`) are the sole writers | Change semantics under `count installed`: `routes_received >= routes_imported` matches BIRD when all negotiated families use `count installed`; default `count offered` preserves equality |
| Birdwatcher `/api/looking-glass/protocols/bgp` JSON (`routes_received`, `routes_imported`, `routes_counts_available`) | reader | Alice-LG and external birdwatcher clients (`docs/architecture/api/birdwatcher-compat.md`), `test/plugin/lg-received-prepolicy.ci` | `docs/architecture/api/birdwatcher-compat.md` Sections 3.2, 5, and 7.3 | Preserve wire schema (all existing snake_case keys and types unchanged); update `birdwatcher-compat.md` Sections 3.2, 7.3, and 8 to record opt-in `routes_received >= routes_imported` |

### Integration Points
- `plugin.PeerInfo` (`internal/component/plugin/types_bgp.go`) - adds `PrePolicyReceived map[family.Family]int64`.
- `Peer` & `Session` (`internal/component/bgp/reactor/peer.go`, `peer_run.go`, `session_prefix.go`) - lock-free `installedCountTable` construction in `NewSession`, publication in `Peer.runOnce` when `p.session = session; published = true`, and teardown clear in `runOnce` defer.
- `reactorAPIAdapter.Peers()` (`internal/component/bgp/reactor/reactor_api.go`) - snapshots `PrePolicyReceived` lock-free.
- `handleBgpSummary` & `registerColumns` (`internal/component/bgp/plugins/cmd/peer/summary.go`, `peer.go`) - merges pre-policy count and `routes-received-source` with RIB counts and declares column order.
- `routeCountsAvailable` (`internal/component/lg/handler_api.go`) - checks both `routes-received` and `routes-accepted`.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Count flows `Session` -> `Peer` atomic table -> `reactorAPIAdapter.Peers()` -> `PeerInfo` -> `summary.go` -> `lg/handler_api.go`; neither `cmd/peer` nor `lg` reaches into reactor internals |
| No unintended coupling (components stay isolated) | Yes | `lg` reads only `show bgp` JSON; `cmd/peer` reads only `PeerInfo` and `ForwardToPlugin("show bgp rib status")` |
| No duplicated functionality (extends existing, does not recreate) | Yes | Reuses `prefixCounts.sets` cardinality maintained by `count installed` and the existing `Peer` atomic stats + `PeerInfo` snapshot path |
| Zero-copy preserved where applicable (refs, not copies) | Yes | Zero per-NLRI or per-UPDATE allocation: fixed atomic slots allocated once per session only for `count installed` families, nil under default `count offered` |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | Rides existing `PeerInfo` and `handleBgpSummary` merge path; per-family identity keys come from registered `nlrisplit.GetPrefixKey` |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes | `git grep routes-received` verified only `summary.go`, `cmd/peer/peer.go` (`registerColumns`), `lg/handler_api.go`, and `lg/summary_producer_test.go` list these summary keys; no central family switch or command table is edited |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `prefixCounts.sets[fk]` under `count installed` is strictly pre-policy and immune to implicit-withdraw drift | `session_read.go` runs `checkPrefixLimits` before `onMessageReceived` / `reactor_notify.go` import filter; `applyInstalledPrefixSections` keys `pc.sets[fk]` by `nlrisplit.GetPrefixKey` with ADD-PATH Path ID | Exposed count would duplicate accepted or drift under re-announcement | `session_prefix_count_test.go`, `session_prefix_receive_test.go`, and `test/plugin/bgp-summary-received-prepolicy.ci` | confirmed (2026-11-26) |
| A-2 | `PrefixCount` is immutable for the lifetime of a `Session` | `peer_settings_apply.go` excludes `PrefixCount` from `hotSwappableSettings`, so changing mode restarts the peer and creates a new `Session` | Fixed slot table could miss a family added mid-session | `peer_settings_apply.go` + unit test | confirmed (2026-11-26) |
| A-3 | Fixed per-family `atomic.Int64` slots on `Peer` remove the data race with `reactorAPIAdapter.Peers()` | `reactor_api.go` reads under `r.mu.RLock()`; session read goroutine writes without `r.mu` | Data race under concurrent UPDATEs and `show bgp` | `go test -race` (`TestPeersIncludesPrePolicyCount`) | unvalidated |
| A-4 | Under `count installed`, `routes-received >= routes-accepted` holds across all registered families, including VPN and FlowSpec | `forEachPrefixEntry` (`session_prefix.go`) uses registered `nlrisplit.Get` / `GetWithdraw` and `GetPrefixKey` for every family (old CIDR-approximation R-3 is resolved in HEAD) | `routes_received < routes_imported` could violate birdwatcher expectations | unit tests + `test/plugin/lg-received-prepolicy.ci` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Unfiltered `show bgp` on a mixed-mode peer (`ipv4/unicast` = `installed`, `ipv6/unicast` = `offered`) could under-report `routes-received` if partial installed counts were emitted beside all-family `routes-accepted` | `routes-received < routes-accepted` on a mixed-mode peer | On unfiltered `show bgp`, emit `routes-received-source: "pre-policy"` only when every negotiated family is tracked in `PrePolicyReceived`; otherwise fall back to `routes-received-source: "accepted"` while `show bgp <afi/safi>` still shows pre-policy for each `installed` family |
| R-2 | 100% import-rejected peer has no `PeerRIB` entry in `bgp-rib`'s `r.bgpPeers`, so `mergeRibRouteCounts` would omit `routes-accepted` | `show bgp` shows `routes-received = N` with no `routes-accepted` even though `bgp-rib` is running | Distinguish `ribAvailable` (from `show bgp rib status` `StatusDone`) from `len(RouteCounts) == 0`; emit `routes-accepted = 0`, `routes-sent = 0` for `count installed` peers when `ribAvailable` is true and the peer has no RIB entry |
| R-3 | `bgp-rib` absent while `count installed` is enabled causes `lg` `routeCountsAvailable` to misreport placeholder zeros for `routes_imported` as real | `routes_counts_available: true` with `routes_imported: 0` when `bgp-rib` is unloaded | Require both `routes-received` and `routes-accepted` to be numeric in `routeCountsAvailable` (`lg/handler_api.go`) |
| R-4 | Pre-policy vs accepted gap includes both import-policy rejects and downstream drops until `spec-bgp-filtered-route-storage.md` lands | Operators assume `received - accepted` equals policy-filtered alone | Document clearly in `docs/guide/command-reference.md` and `docs/guide/looking-glass.md` that the gap is "not accepted" (pre-policy admitted minus post-policy Adj-RIB-In) |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Wrong `routes-received` count or data race on `show bgp` for `count installed` peers; default `count offered` peers are untouched |
| How is it reverted? | Single commit revert; additive `PeerInfo` field and summary key require no config migration |
| Who else touches this path? | `plan/release/fixes/spec-bgp-prefix-count-mode-metric.md` edits `session_prefix.go`, `peer.go`, `peer_run.go`, and `peer_removal.go` on disjoint metric code; `plan/post-release/features/spec-bgp-filtered-route-storage.md` will later touch `summary.go` for `routes-filtered` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `show bgp` and `show bgp <afi/safi>` after a `count installed` peer sends N routes with M rejected by import policy | → | `installedCountTable` -> `Peer.installedPrefixCountsSnapshot` -> `reactorAPIAdapter.Peers` -> `handleBgpSummary` (`mergePrePolicyReceived`) | `test/plugin/bgp-summary-received-prepolicy.ci` |
| `GET /api/looking-glass/protocols/bgp` after the same exchange | → | `handleAPIProtocols` -> `transformProtocols` -> `routeCountsAvailable` | `test/plugin/lg-received-prepolicy.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Peer configured with `prefix { count installed; }` advertises N distinct prefixes (including re-announcements and an explicit withdrawal), and import policy rejects M (0 < M <= N) | `show bgp` reports `routes-received = N`, `routes-accepted = N - M`, and `routes-received-source = "pre-policy"` (including when M = N and `bgp-rib` is loaded); re-announcements do not inflate `N`, explicit withdrawals decrement `N`, and messages refused by prefix-limit enforcement do not inflate `N` | Messages refused before plugin delivery (prefix-limit overflow, RFC 7606 malformed drops, PATHS-LIMIT receive ceiling) are excluded from `N` because `count installed` counts admitted pre-policy prefixes |
| AC-2 | Default peer with `count offered` (no `count installed` configured) advertises routes while concurrent `show bgp` and UPDATEs run on both `offered` and `installed` peers | Default `count offered` peer allocates no `installedCountTable` and no `prefixCounts.sets` (`PrePolicyReceived == nil`), keeps `routes-received == routes-accepted` with `routes-received-source = "accepted"`, and `go test -race` is clean across concurrent UPDATEs and `Peers()` snapshots | `rs-client` peers with a `prefix maximum` keep their existing `pc.retained` set required by RFC 7947 Section 2.1, which is separate from `PrePolicyReceived` publication |
| AC-3 | `GET /api/looking-glass/protocols/bgp` is queried after a `count installed` peer sends N prefixes with M rejected (0 < M < N), and for a peer when `bgp-rib` is absent | Birdwatcher returns `routes_received = N`, `routes_imported = N - M`, and `routes_counts_available = true` when `bgp-rib` is loaded; when `bgp-rib` is absent, `routes_counts_available = false` even if `count installed` populated `routes-received` | Default `count offered` peers and mixed-mode peers on the unfiltered endpoint continue to report `routes_received == routes_imported` |
| AC-4 | `show bgp <afi/safi>` is queried on a two-family peer (both families `count installed`, and a mixed peer with `ipv4/unicast` `installed` and `ipv6/unicast` `offered`) | `show bgp ipv4/unicast` reports the IPv4-scoped pre-policy `routes-received` with `routes-received-source = "pre-policy"`; `show bgp ipv6/unicast` reports IPv6-scoped pre-policy when `installed` or accepted fallback (`routes-received-source = "accepted"`) when `offered`; unfiltered `show bgp` reports pre-policy sum only when all negotiated families use `count installed`, and falls back to `routes-received-source = "accepted"` on mixed-mode peers | Unfiltered `show bgp` intentionally falls back to `accepted` on mixed-mode peers rather than mixing partial pre-policy counts with all-family accepted counts |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Enables `count installed` and runs `show bgp` / `show bgp <afi/safi>` to compare pre-policy received vs post-policy accepted routes | `prefixCounts.sets` -> `installedCountTable` -> `PeerInfo.PrePolicyReceived` -> `handleBgpSummary` | `test/plugin/bgp-summary-received-prepolicy.ci` |
| 2 | Leaves default `count offered` and runs `show bgp` with zero added per-peer NLRI-key RAM | `installedCountTable == nil` -> `PrePolicyReceived == nil` -> `mergeRibRouteCounts` (`routes-received-source: "accepted"`) | `test/plugin/bgp-summary-received-prepolicy.ci` |
| 3 | Queries `/api/looking-glass/protocols/bgp` for a `count installed` peer and sees `routes_received >= routes_imported` | `show bgp` JSON -> `transformProtocols` -> birdwatcher response | `test/plugin/lg-received-prepolicy.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestInstalledCountTableLifecycleAndZeroDefaultRAM` | `internal/component/bgp/reactor/session_prefix_test.go` | AC-1/AC-2: `installedCountTable` is nil under `count offered`, tracks re-announce/withdraw/rollback accurately under `count installed`, and clears on session teardown | |
| `TestPeersIncludesPrePolicyCount` | `internal/component/bgp/reactor/reactor_api_test.go` | AC-2: `reactorAPIAdapter.Peers()` reads `PrePolicyReceived` lock-free under concurrent UPDATEs (`-race` clean) and returns `nil` under default `count offered` | |
| `TestBgpSummaryPrePolicyReceivedCounts` | `internal/component/bgp/plugins/cmd/peer/summary_test.go` | AC-1/AC-4: `count installed` single-family and multi-family pre-policy counts, 100%-rejected peer with available RIB, `bgp-rib` absent, default `count offered`, and mixed-mode unfiltered fallback | |
| `TestTransformProtocolsPrePolicyAndAvailability` | `internal/component/lg/handler_api_test.go` | AC-3: `routes_received > routes_imported` passes through unchanged; `routeCountsAvailable` returns `false` when `routes-accepted` is absent | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `PrePolicyReceived[fam]` | `0..int64 max` | `int64 max` | N/A (set cardinality `len(pc.sets[fk])` is always `>= 0`) | N/A |
| `M` rejected of `N` advertised | `0..N` | `M = N` (100% rejected, `routes-accepted = 0`, `routes-received = N`) | `M = 0` (`routes-received == routes-accepted == N`) | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-summary-received-prepolicy` | `test/plugin/bgp-summary-received-prepolicy.ci` | Peer with `count installed` sends N routes across two families, M rejected by import policy, plus re-announcements and a withdrawal; verifies `show bgp`, `show bgp <afi/safi>`, and a sibling `count offered` peer | |
| `lg-received-prepolicy` | `test/plugin/lg-received-prepolicy.ci` | Birdwatcher `/api/looking-glass/protocols/bgp` reports `routes_received = N` and `routes_imported = N - M` for `count installed` peer | |

### Interop Tests
- N-A: No BGP wire bytes or protocol messages change; per-family `count installed` identity sets are already covered by reactor prefix tests.

## Files to Modify
- `internal/component/bgp/reactor/session_prefix.go` - add `installedCountTable` on `Session` (`session.installedCounts`, allocated once per session only when `len(pc.installed) > 0`), update slots on `applyInstalledPrefixSections` and `restoreInstalledPrefixCounts` without locking or per-UPDATE allocation.
- `internal/component/bgp/reactor/peer.go` - add `installedPrefixCounts atomic.Pointer[installedCountTable]` and lock-free `installedPrefixCountsSnapshot() map[family.Family]int64` on `Peer`.
- `internal/component/bgp/reactor/peer_run.go` - publish `p.installedPrefixCounts.Store(session.installedCounts)` in `runOnce` under `p.mu` when `p.session = session; published = true`, and clear `p.installedPrefixCounts.Store(nil)` in `runOnce`'s teardown defer beside `p.negotiated.Store(nil)`.
- `internal/component/bgp/reactor/reactor_api.go` - populate `info.PrePolicyReceived` from `p.installedPrefixCountsSnapshot()` in `reactorAPIAdapter.Peers()`.
- `internal/component/plugin/types_bgp.go` - add `PrePolicyReceived map[family.Family]int64` to `PeerInfo`.
- `internal/component/bgp/plugins/cmd/peer/summary.go` - update `fetchRibRouteCounts` to report RIB availability separately from an empty peer map; add `mergePrePolicyReceived` with `routes-received-source` (`pre-policy` vs `accepted`); rewrite the stale single-count comment on `mergeRibRouteCounts`.
- `internal/component/bgp/plugins/cmd/peer/peer.go` - place `"routes-received-source"` beside `"routes-received"` in `registerColumns` (`cmdBgp` `ColumnOrder`).
- `internal/component/lg/handler_api.go` - require both `routes-received` and `routes-accepted` to be numeric in `routeCountsAvailable`.
- `internal/component/bgp/reactor/session_prefix_test.go` - add `TestInstalledCountTableLifecycleAndZeroDefaultRAM`.
- `internal/component/bgp/reactor/reactor_api_test.go` - add `-race` `TestPeersIncludesPrePolicyCount`.
- `internal/component/bgp/plugins/cmd/peer/summary_test.go` - add `TestBgpSummaryPrePolicyReceivedCounts`.
- `internal/component/lg/handler_api_test.go` - add `TestTransformProtocolsPrePolicyAndAvailability`.
- `internal/component/lg/summary_fixture_test.go` - keep producer fixture contract aligned with `routes-received-source`.
- `internal/component/lg/summary_producer_test.go` - strip `"routes-received-source"` alongside `"routes-received"`, `"routes-accepted"`, `"routes-sent"` in `TestMockSummaryMatchesRegisteredProducer`.
- `docs/architecture/api/birdwatcher-compat.md` - update Section 3.2, Section 7.3, and Section 8 for opt-in `count installed` pre-policy `routes_received >= routes_imported`.
- `docs/guide/command-reference.md` - document `routes-received`, `routes-received-source` (`pre-policy` vs `accepted`), and `count installed` opt-in semantics on `show bgp`.
- `docs/guide/looking-glass.md` - document when `routes_received` reflects pre-policy received routes (`count installed`) vs post-policy accepted routes (default `count offered`).

## Files to Create
- `test/plugin/bgp-summary-received-prepolicy.ci` - functional test for `show bgp` and `show bgp <afi/safi>` pre-policy vs accepted counts across `installed` and `offered`.
- `test/plugin/lg-received-prepolicy.ci` - functional test for birdwatcher `/api/looking-glass/protocols/bgp` pre-policy `routes_received` vs `routes_imported`.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | Reuses existing `prefix { count offered\|installed; }` leaf in `internal/component/bgp/yang/ze-bgp-conf.yang` |
| YANG validation constraints | No | No new config leaf added |
| YANG custom validators | No | No new config leaf added |
| CLI commands/flags | No | Uses existing `show bgp` and `show bgp <afi/safi>` commands |
| CLI grammar (keyword before value) | No | No grammar change |
| Editor autocomplete | No | No new config leaf or command |
| Functional test for new RPC/API | Yes | `test/plugin/bgp-summary-received-prepolicy.ci` and `test/plugin/lg-received-prepolicy.ci` |
| Pipe completeness | No | Existing `show bgp` already routes through shared command pipe rendering |
| Env var registration | No | No env var added |
| Doctor check for runtime dependencies | No | No new runtime dependency |
| Doctor code per problem class | No | No new refusal, warning, or error log class |
| Prometheus counters/metrics | No | Split to `plan/release/fixes/spec-bgp-prefix-count-mode-metric.md` per 2026-11-24 owner ruling |
| BGP family surface (new SAFI / capability / attribute) | No | Uses existing `nlrisplit.GetPrefixKey` across all registered families |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Surfaces existing `count installed` cardinality on existing `show bgp` and birdwatcher fields |
| 2 | Config syntax changed? | No | Reuses existing `prefix { count installed; }` leaf |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` (`show bgp` `routes-received` and `routes-received-source`) |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/birdwatcher-compat.md` (Section 3.2, Section 7.3, and Section 8) |
| 5 | Plugin added/changed? | No | No new plugin |
| 6 | Has a user guide page? | Yes | `docs/guide/looking-glass.md` |
| 7 | Wire format changed? | No | No BGP wire change |
| 8 | Plugin SDK/protocol changed? | No | `PeerInfo` is internal to `internal/component/plugin/types_bgp.go`; `pkg/plugin` unchanged |
| 9 | RFC behavior implemented, changed, or newly proven? | No | RFC 4271 Adj-RIB-In semantics preserved |
| 10 | Test infrastructure changed? | No | Uses existing `.ci` runner |
| 11 | Affects daemon comparison? | No | No daemon comparison table change |
| 12 | Internal architecture changed? | Yes | `docs/architecture/api/birdwatcher-compat.md` (Section 3.2, Section 7.3, and Section 8); verified other anchored architecture pages remain accurate via `./le spec citation anchors` |
| 13 | Route metadata keys added/changed? | No | `routes-received-source` is a `show bgp` peer-summary row key, not a per-UPDATE `RawMessage.Meta` key (`docs/architecture/meta/README.md` unaffected) |
| 14 | Prometheus counters added/changed? | No | Split to `plan/release/fixes/spec-bgp-prefix-count-mode-metric.md` |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | No inventory change |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Checked via `./le spec citation anchors spec plan/release/fixes/spec-bgp-per-peer-received-counter.md`; `docs/architecture/api/birdwatcher-compat.md` is updated. Declared design docs `docs/architecture/core-design.md`, `docs/architecture/api/commands.md`, `docs/architecture/api/process-protocol.md`, and `docs/architecture/web-interface.md` are unaffected because lock-free `PrePolicyReceived` publication and additive `routes-received-source` preserve their existing reactor, command, protocol, and LG-isolation designs. |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/command-reference.md` and `docs/guide/looking-glass.md` |

## Design presentation

### What changes

Peers configured with `prefix { count installed; }` publish their per-family pre-policy distinct-prefix cardinality lock-free from the BGP reactor into `show bgp`, `show bgp <afi/safi>`, and the birdwatcher Looking Glass (`/api/looking-glass/protocols/bgp`), alongside a `routes-received-source` indicator (`"pre-policy"` vs `"accepted"`). Peers left on the default `count offered` allocate no per-peer NLRI identity set, pay zero additional RAM, and keep `routes-received` equal to post-policy `routes-accepted` (`routes-received-source: "accepted"`).

### Before and after

1. **`show bgp` for a `count installed` peer sending 10 routes with 4 rejected by import policy:**
   - **Before:** `"routes-received": 6, "routes-accepted": 6, "routes-sent": 0` (no visibility into the 4 pre-policy routes not accepted).
   - **After:** `"routes-received": 10, "routes-accepted": 6, "routes-sent": 0, "routes-received-source": "pre-policy"`.
2. **`show bgp` for a default `count offered` peer sending 10 routes with 4 rejected:**
   - **Before:** `"routes-received": 6, "routes-accepted": 6, "routes-sent": 0` (zero NLRI-key RAM).
   - **After:** `"routes-received": 6, "routes-accepted": 6, "routes-sent": 0, "routes-received-source": "accepted"` (still zero NLRI-key RAM, and the row explicitly states the count came from accepted routes).
3. **`GET /api/looking-glass/protocols/bgp` for the `count installed` peer:**
   - **Before:** `"routes_received": 6, "routes_imported": 6, "routes_counts_available": true`.
   - **After:** `"routes_received": 10, "routes_imported": 6, "routes_counts_available": true`.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Always-on per-peer pre-policy NLRI set for every peer by default | Rejected by Thomas on 2026-11-24: paying per-peer NLRI-key RAM by default duplicates RIB memory on full-table routers that did not opt in |
| Exposing the default `count offered` event tally as `routes-received` | Disproved by the 2026-07-16 audit: `offered` counts wire events and drifts upward forever under BGP implicit withdrawal (re-announcement) |
| Mixing partial `count installed` counts with total `routes-accepted` on unfiltered `show bgp` for mixed-mode peers | Could report `routes-received < routes-accepted` when an `offered` family holds more accepted routes than the `installed` family; falling back to `accepted` on unfiltered `show bgp` while still showing pre-policy per family on `show bgp <afi/safi>` stays truthful |

### Documents departed from

- **`docs/architecture/api/birdwatcher-compat.md`**, section `3.2 Which direction each endpoint answers`: currently binds ``"`routes_received`, `routes_imported` (Section 5)"`` unconditionally to ``"`route-counts.in` of `show bgp rib status`"``. After this work, Section 3.2 notes that `routes_received` reads `show bgp`'s pre-policy `routes-received` (sourced from `PeerInfo.PrePolicyReceived`) when all negotiated families enable `prefix { count installed; }`, while `routes_imported` remains `route-counts.in` and default `count offered` keeps both on `route-counts.in`.
- **`docs/architecture/api/birdwatcher-compat.md`**, section `7.3 Known equalities and absences`: currently states ``"`routes_imported` MUST be expected to equal `routes_received`. Both are the Adj-RIB-In size, because Ze drops rejected routes before storage and keeps no separate pre-policy count. A client MUST NOT infer a policy drop from the difference, which is always zero."`` After this work, Section 7.3 (and Section 8 Divergence summary row 2) states that `routes_received` reports the pre-policy distinct-prefix count (`routes_received >= routes_imported`) when all negotiated families of the peer enable `prefix { count installed; }` (and remains equal to `routes_imported` under the default `count offered` or when not all negotiated families use `count installed`).

### Choices for Thomas

none

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- add `PeerInfo.PrePolicyReceived` (`types_bgp.go`), wire `installedCountTable` stub from `Session`/`Peer` to `reactorAPIAdapter.Peers()`, and write failing wiring tests (`test/plugin/bgp-summary-received-prepolicy.ci`, `test/plugin/lg-received-prepolicy.ci`, `TestPeersIncludesPrePolicyCount`).
   - Tests: `TestPeersIncludesPrePolicyCount`, `test/plugin/bgp-summary-received-prepolicy.ci`, `test/plugin/lg-received-prepolicy.ci`
   - Files: `types_bgp.go`, `peer.go`, `reactor_api.go`, `test/plugin/bgp-summary-received-prepolicy.ci`, `test/plugin/lg-received-prepolicy.ci`
   - Verify: wiring tests fail because `PrePolicyReceived` is not yet populated/merged.
2. **Phase: Lock-free reactor publication** -- implement `installedCountTable` in `session_prefix.go` (nil when `len(pc.installed) == 0`), publish `p.installedPrefixCounts.Store(session.installedCounts)` in `peer_run.go` (`runOnce` when `p.session = session; published = true`), clear it in `runOnce`'s teardown defer, and snapshot in `reactorAPIAdapter.Peers()`.
   - Tests: `TestInstalledCountTableLifecycleAndZeroDefaultRAM`, `TestPeersIncludesPrePolicyCount` under `-race`
   - Files: `session_prefix.go`, `peer.go`, `peer_run.go`, `reactor_api.go`, `session_prefix_test.go`, `reactor_api_test.go`
   - Verify: unit tests pass under `-race`; zero allocations on `count offered` path.
3. **Phase: Summary merge and Looking Glass** -- update `fetchRibRouteCounts`, `mergeRibRouteCounts`, and `mergePrePolicyReceived` in `summary.go` (with `routes-received-source`), add `"routes-received-source"` to `registerColumns` in `cmd/peer/peer.go`, tighten `routeCountsAvailable` in `lg/handler_api.go`, update `lg/summary_fixture_test.go` and `lg/summary_producer_test.go`, and prove AC-1..AC-4.
   - Tests: `TestBgpSummaryPrePolicyReceivedCounts`, `TestTransformProtocolsPrePolicyAndAvailability`, `test/plugin/bgp-summary-received-prepolicy.ci`, `test/plugin/lg-received-prepolicy.ci`
   - Files: `summary.go`, `cmd/peer/peer.go`, `summary_test.go`, `lg/handler_api.go`, `lg/handler_api_test.go`, `lg/summary_fixture_test.go`, `lg/summary_producer_test.go`
   - Verify: unit tests and both `.ci` tests pass green.
4. **Phase: Documentation** -- update `docs/architecture/api/birdwatcher-compat.md` (Section 3.2, Section 7.3, and Section 8), `docs/guide/command-reference.md`, and `docs/guide/looking-glass.md`.
   - Files: `docs/architecture/api/birdwatcher-compat.md`, `docs/guide/command-reference.md`, `docs/guide/looking-glass.md`
   - Verify: `./le doc check all` clean.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-4 has file:line implementation and passing test proof |
| Zero default RAM | Default `count offered` allocates neither `prefixCounts.sets` nor `installedCountTable` |
| Race safety | `reactorAPIAdapter.Peers()` never reads `Session.prefixCounts`; only reads `Peer.installedPrefixCounts` atomics, proven under `-race`; unpublished incoming collision candidates never touch `Peer.installedPrefixCounts` |
| Scope honesty | Mixed-mode peers fall back to `routes-received-source: "accepted"` on unfiltered `show bgp` rather than reporting `routes-received < routes-accepted` |
| LG honesty | `routeCountsAvailable` requires both `routes-received` and `routes-accepted` to be numeric |
| Departure updated | `docs/architecture/api/birdwatcher-compat.md` Section 3.2, Section 7.3, and Section 8 updated in the same change |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `PrePolicyReceived` on `PeerInfo` | `git grep -n 'PrePolicyReceived' internal/component/` |
| Race-clean reactor snapshot and zero-default RAM tests | `./le test unit pkg path internal/component/bgp/reactor/ run 'Test(InstalledCountTable|PeersIncludesPrePolicyCount)'` |
| Summary and LG unit tests | `./le test unit pkg path internal/component/bgp/plugins/cmd/peer/ run TestBgpSummaryPrePolicyReceivedCounts` and `./le test unit pkg path internal/component/lg/` |
| Functional `.ci` proof | `./le test functional plugin` (`bgp-summary-received-prepolicy.ci` and `lg-received-prepolicy.ci`) |
| Docs and source anchors clean | `./le doc check all` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Resource exhaustion | Pre-policy NLRI set memory is strictly opt-in (`count installed`) and bounded by configured `prefix maximum` (`applyInstalledPrefixSection` caps growth at `maximum + 1` before rollback); default `count offered` pays zero set RAM |
| Error / data leakage | Exposed values are non-negative integers and fixed strings (`"pre-policy"` / `"accepted"`); no peer-controlled bytes reach CLI or public Looking Glass output |

### Failure Routing
| Failure | Route To |
|---------|----------|
| `routes-received == routes-accepted` under `count installed` with import rejects | Verify `installedCountTable` is bound on `Peer` before `Session.Run` and `mergePrePolicyReceived` runs after `mergeRibRouteCounts` |
| `-race` failure on `Peers()` | Verify `Peers()` reads only `p.installedPrefixCounts.Load()` atomic slots and never touches `Session.prefixCounts` |
| `routes_counts_available` true when `bgp-rib` absent | Verify `routeCountsAvailable` requires both `routes-received` and `routes-accepted` |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Work Not Done

| Item | Owning spec |
|------|-------------|
| Former AC-5: count-mode (`offered`/`installed`) labels on `ze_bgp_prefix_count`, `ze_bgp_prefix_ratio`, `ze_bgp_prefix_warning_exceeded`, `ze_bgp_prefix_maximum_exceeded_total` and obsolete-series cleanup | `plan/release/fixes/spec-bgp-prefix-count-mode-metric.md` |
| Retained import-filtered route storage (`keep-filtered`, real `routes_filtered`, and alternative pre-policy tracking without `count installed`) | `plan/post-release/features/spec-bgp-filtered-route-storage.md` |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Populate pre-policy `routes-received` only when `count installed` identity tracking is enabled (nil under default `count offered`) | Always-on per-peer pre-policy NLRI set; exposing drifting `count offered` tally | Ordered by Thomas on 2026-11-24: avoids default per-peer NLRI-key RAM while giving accurate, drift-free counts to opted-in peers |
| Publish counts through a fixed per-session `installedCountTable` of `atomic.Int64` slots behind `atomic.Pointer` on `Peer` | Locking `Session.prefixCounts` under `r.mu`; per-UPDATE map copy snapshot | Zero lock on hot path, zero per-UPDATE allocation, clean `-race` snapshot in `Peers()`, and clean teardown reset |
| Emit `routes-received-source` (`"pre-policy"` vs `"accepted"`) on `show bgp` rows and fall back to `"accepted"` on unfiltered `show bgp` for mixed-mode peers | Mixing partial installed counts with all-family accepted counts; omitting source label | Prevents `routes-received < routes-accepted` on mixed peers and makes the source of `routes-received` unambiguous to operators |

## Known Limitations
- The `routes-received - routes-accepted` gap under `count installed` reflects admitted pre-policy prefixes minus post-policy Adj-RIB-In; full attribution of import-policy rejects (and real `routes_filtered`) lives in `plan/post-release/features/spec-bgp-filtered-route-storage.md`.
- Peers using default `count offered` intentionally do not allocate a pre-policy NLRI set and continue reporting `routes-received == routes-accepted` (`routes-received-source: "accepted"`).

## Review Gate
Pending implementation and independent `/ze-review` + `/ze-review-spec`.

## Checklist

### Pre-Spec Verification
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior, Existing Mechanisms, Design documents, Data Flow, and Design presentation completed
- [ ] AC-N rows carry testable positive assertions and Permitted Survivors
- [ ] `./le spec design check spec plan/release/fixes/spec-bgp-per-peer-received-counter.md` reports only `no judge has judged this design`

### Goal Gates (MUST pass)
- [ ] AC-1..AC-4 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] Scoped tests for changed packages and both `.ci` tests pass in the main checkout
- [ ] Feature code integrated (`internal/*`)
- [ ] Integration and Documentation checklists answered Yes/No with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `./le spec review record`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/fixes/spec-bgp-per-peer-received-counter.md` only, in the same `./le commit create` script

## Work Inherited From a Deferral Row

<!-- The deferral directory was deleted on 2026-09-05. A row that named this spec as
     its destination is reproduced here, so the item and the reasoning behind it
     survive the directory. Each row is outstanding work this spec owns. -->

### From `bgp-summary-route-counts.md`, 2026-07-16

Deferred by spec-bgp-summary-route-counts (Known Limitations).

Surface a per-peer pre-policy received count, distinct from accepted. This is the inherited requirement; per the 2026-11-24 owner decision, it is satisfied without default per-peer NLRI-key RAM by populating pre-policy `routes-received` when `count installed` identity tracking is enabled (or post-release filtered-route storage).
