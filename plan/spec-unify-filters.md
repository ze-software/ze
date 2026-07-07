# Spec: unify-filters

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 4/4 |
| Updated | 2026-07-07 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/api/architecture.md` - BGP route filter pipeline contract (the `filterapi` seam)
4. `internal/component/bgp/reactor/reactor_notify.go` (the two back-to-back ingress blocks at :393-443 and :458-531), `internal/component/bgp/filterapi/filterapi.go`, `internal/component/bgp/reactor/filter_chain.go`, `internal/component/bgp/reactor/reactor_api_forward.go` (`forwardUpdateCore` :245, egress in-process :474 + export policy chain :498)
5. Session digest: `tmp/session/session-state-unify-filters-*.md` (verified current behavior, the complete 5-filter set, the egress heterogeneity table, and the order correction)

## Task

DESIGN-REVIEW.md finding 2 (row "BGP filtering", the review's #1 priority, correctness-relevant): a received UPDATE is run through TWO independent filtering systems back-to-back inside `notifyMessageReceiver`, and the ordering BETWEEN the two systems is purely positional code-order in that one function, not a declared, inspectable property.

- System 1: the in-process `filterapi` pipeline. Operates on wire bytes (zero-copy). Explicit ordering by Stage, then Priority, then name. Applied at `reactor_notify.go:393-443`.
- System 2: the `PolicyFilterChain`. Serializes the UPDATE to text, RPCs out to external plugin filters, converts the returned text delta back into wire attribute modifications. Ordering is positional over the peer's configured filter list only. Applied at `reactor_notify.go:458-531` (call at :466).

Both run only on `DirectionReceived`, back-to-back, System 1 fully before System 2. The relative order of an in-process filter and the external policy chain is fixed by which block appears first in the function body, not by any declared stage. That implicit ordering is the correctness concern.

Goal: pick a winner (`filterapi`), port the loser's distinguishing behavior onto it as ONE ordered step, and route ALL received-UPDATE filtering through ONE stage-ordered pipeline so the cross-system order becomes EXPLICIT (a declared stage) instead of positional. Preserve every externally observable filtering outcome (this is a refactor).

**Scope decision (user, 2026-07-07):** Ingress is fully unified (the correctness-relevant path DESIGN-REVIEW finding 2 cites). Egress is **order-only**: the SAME ordered-step seam is applied ONLY where both filter kinds already run together (`forwardUpdateCore`), making that one duplicated site declared-order. The RS fast path, injected-route path, and default-originate decision keep their existing filter membership unchanged (see Egress Heterogeneity below). This keeps the whole change a pure refactor.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/architecture.md` - BGP route filter pipeline (the `filterapi` seam header references it)
  → Decision: `filterapi` is the BGP-owned filter seam; protocol filter knowledge stays out of the generic plugin registry, so any unified ordering must live in `filterapi` (ordering primitive) or the reactor (executor binding), never in `internal/component/plugin/registry`.
  → Constraint: `filterapi` is a leaf package (stdlib only, verified: imports errors/fmt/maps/net/netip/sort/sync only) so filter plugins, the reactor, and protocol filters import it without cycles; the reactor may not push reactor-instance state (`r.api`) back into it.
- [ ] `ai/rules/plugin-self-containment.md` - registration over hardcoding
  → Constraint: in-process filters register via `filterapi.Register` at init(); the new ordered policy-chain step must be discovered through a declared Stage (`FilterStagePeerChain`), merge-sorted into the chain, not a new hardcoded code-position block in `notifyMessageReceiver`.
- [ ] `ai/rules/buffer-first.md` - wire encoding on the hot path
  → Constraint: the received-UPDATE path is the hot path; text serialization (`AppendUpdateForFilter`) is paid only when a peer has configured RPC policy filters and must stay gated that way. The ordered step list is built once at `startAPIServer`, never per UPDATE.

**Key insights:**
- The two systems are the SAME layer (ingress route filtering on received UPDATEs) with two complementary execution models (in-process wire-bytes vs out-of-process text/RPC). What is genuinely redundant and buggy is the ORDERING mechanism, not the executors. Keep both executors; unify only the ordering.
- The external per-peer policy chain currently runs **LAST** (after every in-process filter, including the Annotation-stage OTC filter), because System 1 runs the entire sorted `r.ingressFilters` list before System 2 starts. Preserving that means the policy-chain step is a **terminal** stage, not a Policy-stage step. (This corrects the original design; see Mistake Log → Wrong Assumptions.)

## Current Behavior (MANDATORY)

**Source files read (producers, verified file:line):**
- [ ] `internal/component/bgp/reactor/reactor_notify.go` - `notifyMessageReceiver` (:219). System 1 ingress filters at :393-443 (guard `DirectionReceived && wireUpdate != nil && len(r.ingressFilters) > 0`, loop `for _, filter := range r.ingressFilters` at :415 calling `safeIngressFilter` on `wireUpdate.Payload()`; reject → `return false`; `modifiedPayload` → rebuild `WireUpdate` + update `msg` at :423-438). System 2 policy chain at :458-531 (guard adds `hasPeer` + `peer.settings.ImportFilters` + `r.api != nil`; `PolicyFilterChain(...)` at :466; teardown :473, reject :485, raw override :491, text-delta :499-528), back-to-back. `safeIngressFilter` :28, `safeEgressFilter` :41 (both panic-recover, fail-closed).
- [ ] `internal/component/bgp/filterapi/filterapi.go` - registry for System 1. `Filter{Name, Stage, Priority, Ingress, Egress}` (:162). `IngressFilters()` (:261) / `EgressFilters()` (:290) return **only the funcs** (`[]IngressFilterFunc` / `[]EgressFilterFunc`), sorted by `sortedFilters` (:239) on Stage, then Priority, then name — the ordering keys are discarded. Stage constants (:134): Protocol=0, Policy=100, Annotation=200. `ingressFilterNames()` :275 / `egressFilterNames()` :304 expose the same order for reporting. Also `AttrModHandler` progressive-build handlers and `EnableRSForwarding`.
- [ ] `internal/component/bgp/reactor/filter_chain.go` - System 2 executor. `PolicyFilterChain(filterRefs, direction, ...)` (:175) iterates `for _, ref := range filterRefs` (:181), positional; `inactive:` skip (:182); reject short-circuits (:199); text-delta modify (`applyFilterDelta` :206); raw full-payload override (terminal, :203); import-only `Teardown` (:189). `policyFilterFunc` (:366, method on `*Reactor`) does the RPC (`r.api.CallFilterUpdate`), per-filter on-error (`OnErrorAccept`/reject :396), 5s timeout (:360), declared-attr validation (`validateModifyDelta` :411), raw mode (:376).
- [ ] `internal/component/bgp/reactor/reactor.go` - `r.ingressFilters = filterapi.IngressFilters()` (:1189) and `r.egressFilters = filterapi.EgressFilters()` (:1190) and `r.attrModHandlers` (:1191) built once **inside `startAPIServer()` (:1142)** (NOT the constructor `New` :355; the original spec said "at construction" — corrected). Caller holds `r.mu`. Fields: `ingressFilters []filterapi.IngressFilterFunc` :281, `egressFilters []filterapi.EgressFilterFunc` :282, `attrModHandlers` :285, `api *pluginserver.Server` :246 (set at :554 SetPluginServer / :561 SetPluginServerAny / :1174 self-host), `rsForwardingEnabled` :291. `r.api` is available in `startAPIServer` — the natural binding point for the reactor-bound policy-chain step.
- [ ] `internal/component/bgp/config/peers.go` - builds per-peer `ImportFilters`/`ExportFilters` (:155-167, cumulative bgp+group+peer), validates + canonicalizes refs to `<plugin>:<filter>` (:185-202), prepends loop-detection default filters (`prependDefaultFilters` :649/:672), applies loop-detection config by matching filter names (`applyLoopDetectionConfig` :596/:607-641). An `inactive:`-prefixed loop-detection name in `ImportFilters` sets `ps.LoopDisabled = true` (:619), which suppresses the in-process `LoopIngress` for that peer via `PeerFilterInfo.LoopDisabled`.
- [ ] Complete registered `filterapi.Filter` set (grep `filterapi.Register`, 5 filters):

  | Name | Stage | Priority | Ingress | Egress | Registration |
  |------|-------|----------|---------|--------|--------------|
  | `loop` | Protocol (0) | 0 | `LoopIngress` | - | `reactor/filter/register.go:14` |
  | `bgp-filter-community` | Policy (100) | 0 | yes | yes | `plugins/filter_community/register.go:26` |
  | `bgp-redistribute` | Policy (100) | 0 | yes | - | `plugins/redistribute_ingress/register.go:16` |
  | `bgp-role` (OTC) | Annotation (200) | 0 | `OTCIngressFilter` | `OTCEgressFilter` | `plugins/role/register.go:23` |
  | `bgp-gr` (LLGR) | Annotation (200) | 0 | - | `LLGREgressFilter` | `plugins/gr/register.go:32` |

  Ingress order: `loop → bgp-filter-community → bgp-redistribute → bgp-role`. Egress order: `bgp-filter-community → bgp-gr → bgp-role`. `filter_community` also registers a generic `registry.Registration` engine + community `AttrModHandler`s (`filter_community/register.go:15-45`); `role` registers the OTC `AttrModHandler` + a generic engine. `OTCIngressFilter` (`role/otc.go:311`) can reject (`otcRejectLeak`), convert to withdrawal (`otcTreatWithdraw`), or stamp OTC into the payload — so its position relative to the policy chain is observable.
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go` - `forwardUpdateCore` (:245). Per destination peer (`for _, peer := range matchingPeers` :443): fresh `var mods filterapi.ModAccumulator` :469; RS-client community strip :471; **in-process egress filters** `for _, filter := range a.r.egressFilters` at :474-487 via `safeEgressFilter` (suppress → `continue`); **then export `PolicyFilterChain(facts.exportFilters, "export", ...)`** at :498 (reject → `continue`; text-delta builds a separate `exportMods` :511). Back-to-back, same shape as ingress. `forward_rs.go` (RS fast path :319-328) applies in-process egress filters but NOT the export policy chain.
- [ ] `internal/component/bgp/reactor/egress_inject_filter.go` - `exportFilterForBody` (:33), the export gate for **non-forwarded** outbound routes (API/plugin injection, redistribute, adj-rib-in replay, static). Runs ONLY the export `PolicyFilterChain` (:46) on the wire body; does NOT apply in-process egress filters.
- [ ] `internal/component/bgp/reactor/peer_initial_sync.go` - `defaultOriginateFilterAccepts` (:709), a **single-filter dry-run** of the export chain against a synthesized default route (`PolicyFilterChain` at :734) to decide default-originate. Not a route-modification pass.

**Behavior to preserve:** (unless user explicitly said to change)
- The net filtering outcome (accept/reject/modify) for every received and forwarded UPDATE under every existing `.ci` scenario. In particular the current effective **ingress** order: `loop` (Protocol) → `bgp-filter-community` (Policy) → `bgp-redistribute` (Policy) → `bgp-role`/OTC (Annotation) → the per-peer RPC policy chain (LAST). **The policy chain runs after OTC, not before it.**
- System 2 rich semantics: reject short-circuit, text-delta modify, raw hex full-payload override (terminal), import-only `Teardown` (NOTIFICATION + session close), per-filter `OnErrorAccept`/reject, 5s per-filter timeout, declared-attribute validation, `inactive:` skip.
- Loop-detection coupling: an `inactive:`-prefixed loop-detection name in `ImportFilters` still suppresses the in-process `LoopIngress` for that peer (`ps.LoopDisabled`).
- Hot-path gating: no text serialization (`AppendUpdateForFilter`) unless the peer has at least one configured policy filter and `r.api != nil`.
- `parseFilterAttrs` still runs exactly once per filter text on the modify path (`TestFilterDeltaParseCallCount`, `filter_delta_test.go:1264`).
- Egress: `forward_rs.go`, `exportFilterForBody`, and `defaultOriginateFilterAccepts` keep their exact current filter membership. Only `forwardUpdateCore`'s two positional blocks are unified.

**Behavior to change:** None - internal refactor, all filtering outcomes preserved. The only change is structural: the cross-system order becomes a declared Stage property (`FilterStagePeerChain`) instead of implicit code-position, and the second back-to-back block is deleted.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Received BGP UPDATE bytes arrive on the peer session read goroutine and reach `notifyMessageReceiver` (`reactor_notify.go:219`) as a `*wireu.WireUpdate` (lazy-parsed, zero-copy) with `direction == rpc.DirectionReceived`.
- Format at entry: UPDATE body wire bytes (`wireUpdate.Payload()`), no BGP header.

### Transformation Path
1. Received UPDATE reaches `notifyMessageReceiver`; per-peer counters and observers run.
2. **Target:** one stage-ordered ingress pass over `r.orderedIngressSteps` (built once at `startAPIServer`). Each step is either an in-process `filterapi` ingress func (adapted through `safeIngressFilter`) or the reactor-bound policy-chain step (declared Stage `FilterStagePeerChain`, sorts last). The pass reproduces the current `loop → community → redistribute → OTC → policy-chain` order.
3. Each step returns a common `ingressStepResult {accept, modifiedPayload, teardown, notifyCode, notifySubcode}`. `accept==false` drops the route; `teardown` queues NOTIFICATION + close and drops; `modifiedPayload != nil` rebuilds the `WireUpdate` and updates `msg` (the current :423-438 / :491-528 rebuild logic, consolidated).
4. The policy-chain step keeps the hot-path gate: it serializes to text (`AppendUpdateForFilter`) and RPCs only when the peer has `ImportFilters` and `r.api != nil`; otherwise it is a no-op accept.
5. The (possibly modified) `WireUpdate` is cached (`recentUpdates.Add`) and dispatched to consumers. `routeMeta` still comes from the shared in-process `ingressMeta` map.
6. **Egress twin (order-only):** `forwardUpdateCore` runs one stage-ordered pass over `r.orderedEgressSteps` (replacing ONLY `reactor_api_forward.go:474-528`) per destination peer — in-process egress filters (Stages ≤ 200, writing the shared `ModAccumulator`) then the export policy-chain step (`FilterStagePeerChain`, reading the ORIGINAL payload, producing `exportWireOverride`). Unlike ingress, egress steps do NOT compose sequentially on the payload: in-process filters defer to `mods`, the policy chain overrides separately, and both are combined after the pass (`peerBaseWire = exportWireOverride ?? original`, then `mods` applied in the progressive build). The `mods` writers surrounding the pass (RS strip `:471`, RR injection `:530`, nexthop/community `:536`) stay in place. `ModAccumulator` stays fresh per peer. RS/injected/default-originate paths untouched.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Wire bytes ↔ in-process filter | `IngressFilterFunc(source, payload, meta)` over `wireUpdate.Payload()` (zero-copy) | [ ] (verified `filterapi.go:50`, `reactor_notify.go:415`) |
| Wire bytes ↔ text ↔ external plugin | `AppendUpdateForFilter` to text, `CallFilterUpdate` RPC, `textDeltaToModOps`/`buildModifiedPayload` back to wire | [ ] (verified `reactor_notify.go:462-528`, `filter_chain.go:366`) |
| Reactor ↔ filterapi registry | `filterapi.IngressFilters()`/`EgressFilters()` read once in `startAPIServer` (`reactor.go:1189-1190`); new ordered accessors + `FilterStagePeerChain` added | [ ] (verified `reactor.go:1142/1189`) |
| Config ↔ per-peer chain | `peers.go` builds/canonicalizes `ImportFilters`/`ExportFilters` into `peer.settings` | [ ] (verified `peers.go:155-202`) |

### Integration Points
- `filterapi.sortedFilters` (Stage/Priority/name ordering) - the existing ordering primitive the unified pipeline reuses and extends; new `FilterStagePeerChain=300` slots the policy-chain step after Annotation.
- `PolicyFilterChain` / `policyFilterFunc` - becomes the body of the single ordered policy-chain step (executor unchanged).
- `peer.settings.ImportFilters` / `ExportFilters` - already in scope inside `notifyMessageReceiver` / `forwardUpdateCore`; the policy-chain step reads them directly (no `PeerFilterInfo` change needed — corrects original A-3).
- `safeIngressFilter` / `safeEgressFilter` - panic-recovering wrappers the unified pass keeps for every in-process step.

### Architectural Verification
- [ ] No bypassed layers (received UPDATE still flows session → notify → ordered pipeline → cache → dispatch).
- [ ] No unintended coupling (`filterapi` stays stdlib-only leaf; reactor-instance state is not pushed into it — it gains only a stage constant + ordering accessors/comparator).
- [ ] No duplicated functionality (two ordering mechanisms collapse to one; executors kept, not recreated).
- [ ] Zero-copy preserved (in-process steps still operate on `wireUpdate.Payload()`; text serialization stays gated to peers with policy filters; step list built once).
- [ ] Registration over hardcoding - the policy-chain step joins via a declared Stage merge-sort, not a hardcoded code-position block; in-process filters continue to register via `filterapi.Register` and the reactor discovers them.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Current runtime order is System 1 (whole sorted `r.ingressFilters`) fully, THEN System 2 (positional policy chain), for every received UPDATE — so the policy chain runs LAST, after OTC | `reactor_notify.go:415` loops all of `r.ingressFilters` (incl. `bgp-role`/Annotation) at :393-443, before `PolicyFilterChain` at :466 | The order to reproduce is different | Code read (done) + `TestUnifiedIngressReproducesLegacyOrder` characterization | confirmed |
| A-2 | `filter_community`'s in-process filterapi filter and its RPC policy filter are not double-applied in a way the merge would change | `filter_community/register.go` registers both; the in-process one always runs, the RPC one only if configured in `ImportFilters` | Unification must dedup the two community paths, larger scope | `test/plugin/community-strip.ci`, `test/plugin/community-cumulative.ci` pass unchanged | confirmed (baseline 2026-07-07: both PASS) |
| A-3 | The policy-chain step can read the per-peer `ImportFilters`/`ExportFilters` directly from `peer.settings` (in scope at the call site); `PeerFilterInfo` does NOT need new fields | `peer.settings.ImportFilters` used at `reactor_notify.go:461`; `ExportFilters` via `facts.exportFilters` at `reactor_api_forward.go:498` | Need a different context seam | Compile + existing filter plugin unit tests | confirmed (corrects original A-3 premise) |
| A-4 | Building `r.orderedIngressSteps`/`orderedEgressSteps` once in `startAPIServer` (where `r.api` exists and `r.mu` is held) is safe for all server-attach paths (self-host, SetPluginServer, SetPluginServerAny) | `startAPIServer` runs regardless of attach path; sets `r.ingressFilters` there today (`reactor.go:1189`) | Step list built with nil `r.api` or not built | Existing startup path tests + compile; `.ci` suite | confirmed (2026-07-07: all 8 filter `.ci` pass through the real `startAPIServer` build path) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Reordering changes outcomes: the policy-chain step's assigned Stage does not reproduce the current order (policy chain must run AFTER OTC/Annotation, not before) | `.ci` diffs in `community-*`, `role-otc-*`, `aspath-filter-reject` | Assign the step `FilterStagePeerChain = 300` (strictly after Annotation=200) so `loop → community → redistribute → OTC → policy-chain` is reproduced exactly; lock with `TestUnifiedIngressReproducesLegacyOrder` BEFORE touching production code |
| R-2 | `filterapi.Register` is init-only/stateless, but the policy-chain step needs the reactor instance (`r.api`) for RPC | nil `api` at runtime; import cycle if reactor state pushed into filterapi | Reactor BINDS the policy-chain step at `startAPIServer` (not plugin init); `filterapi` gains only `FilterStagePeerChain` + ordering accessors/comparator, never reactor state |
| R-3 | Egress parity over-reach: forcing RS/injected/default-originate paths through one pass would ADD filters they don't run today (behavior change) | `.ci` diffs on RS forwarding / injected routes / default-originate | Per user scope: unify ONLY `forwardUpdateCore`; leave the other three paths' membership unchanged. AC-6 asserts one ordered site *for the paths that run both kinds* |
| R-5 | The common `ingressStepResult` type drops a policy-chain semantic (teardown / raw-terminal / reject vs in-process accept/modify) | AC-3 `.ci` failures (teardown, remove-private-as, community modify) | Outcome struct is a superset of both executors' outcomes; each System-2 semantic maps to a field; AC-3 exercises every one |
| R-6 | Egress merge changes `forwardUpdateCore` result: in-process egress filters mutate via `ModAccumulator` (deferred) while the policy chain reads the ORIGINAL payload and produces `exportWireOverride`; and the two blocks are surrounded by other `mods` writers (RS strip before, RR injection/nexthop/community after) | `role-otc-egress-stamp.ci`, `policy-test-remove-private-as.ci`, `forward_pool_test.go` byte diffs | Replace ONLY `reactor_api_forward.go:474-528`; leave the surrounding `mods` writers (`:471`, `:530-537`) and the `peerBaseWire` combine (`:539`) exactly in place. In-process egress steps Stage ≤ 200, policy-chain step Stage 300 → pass runs in-process-then-policy-chain exactly as today. Separate `egressStepResult {accept, wireOverride}` type (NOT shared with ingress); in-process steps write `mods`, policy-chain step sets `wireOverride`, both read the original payload |
| R-7 | Merged ingress pass allocates per UPDATE (e.g. closures capturing per-call state) on the hot path | `ze-perf` UPDATE throughput regression; `filter_dispatch_alloc_test.go` | Steps are built once at `startAPIServer`; per-UPDATE state passed as function args, not captured; zero-alloc dispatch test extended to the merged pass |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Received UPDATE with a loop-detection filter (Protocol stage) | → | unified ordered ingress pipeline runs loop first | `test/plugin/loop-cluster-list.ci` |
| Received UPDATE, community strip via configured policy filter | → | policy-chain step inside the unified chain | `test/plugin/community-strip.ci` |
| Received UPDATE, `filter_family` teardown | → | policy-chain step teardown honored, route dropped | `test/plugin/filter-family-import-teardown.ci` |
| Received UPDATE, AS-path reject via configured policy filter | → | policy-chain step reject short-circuits | `test/plugin/aspath-filter-reject.ci` |
| Received UPDATE, OTC ingress before policy chain | → | Annotation stage runs before `FilterStagePeerChain` | `internal/component/bgp/reactor/unified_filter_order_test.go` (`TestUnifiedIngressReproducesLegacyOrder`) |
| Forwarded UPDATE, OTC egress stamp (Annotation stage) | → | unified ordered egress pass runs OTC stamp then policy chain | `test/plugin/role-otc-egress-stamp.ci` |
| Received UPDATE, plain-name policy chain ordering | → | canonicalized refs run in declared order | `test/plugin/policy-chain-plain-names.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A received UPDATE with in-process filters (loop, community, redistribute, OTC) and configured RPC policy filters | Every filter runs in ONE ordered pass; relative order is determined by declared Stage, then Priority, then name, never by code position |
| AC-2 | Received UPDATE where loop (Protocol), community + redistribute (Policy), OTC (Annotation), and the policy chain all apply | Observable order matches the pre-refactor order exactly: `Protocol < Policy in-process < Annotation (OTC) < policy-chain step`. The policy chain runs LAST (`FilterStagePeerChain=300`), after OTC — not before it |
| AC-3 | Configured policy filter returns reject / text-delta modify / raw override / teardown / errors | All System 2 semantics preserved through the common step outcome: reject short-circuit, delta modify, terminal raw override, import-only teardown, per-filter on-error, 5s timeout, declared-attr validation, `inactive:` skip |
| AC-4 | Received UPDATE from a peer with no configured policy filters | The policy-chain step is a no-op accept; no text serialization occurs (`AppendUpdateForFilter` not called); gating unchanged |
| AC-5 | UPDATE forwarded via `forwardUpdateCore` to multiple destination peers with egress filters and export policy filters | One declared-order pass per destination applies in-process egress filters (≤ Stage 200) then the export policy chain (`FilterStagePeerChain`); per-peer `ModAccumulator` semantics preserved; RS/injected/default-originate paths unchanged |
| AC-6 | Source audit after refactor | The second back-to-back ingress block in `reactor_notify.go` is gone; exactly ONE ordered ingress invocation site (`notifyMessageReceiver`) and ONE ordered egress invocation site for the both-kinds path (`forwardUpdateCore`) remain. The three single-kind egress paths (RS, injected, default-originate) are documented as intentionally separate |

## End-to-End User Stories (MANDATORY for new features)

Internal refactor — no new user-facing behavior. The operator-observable invariant: for every existing config (loop detection, community filters, OTC roles, external RPC policy filters, remove-private-as, filter-family teardown), a received or forwarded UPDATE produces byte-identical filtering outcomes before and after this change. Evidenced by the unchanged `.ci` suite.

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestUnifiedIngressReproducesLegacyOrder` | `internal/component/bgp/reactor/unified_filter_order_test.go` | The merged ingress pass executes `loop → community → redistribute → OTC → policy-chain` — the exact pre-refactor order, policy chain LAST (R-1, AC-2). Written and green against the CURRENT two-block code first (characterization) | new |
| `TestUnifiedIngressOrder` | `internal/component/bgp/reactor/unified_filter_order_test.go` | The merged pass runs in-process filters and the policy-chain step in declared Stage/Priority/name order; a filter registered at a new stage interleaves correctly (AC-1) | new |
| `TestPeerChainStageSortsLast` | `internal/component/bgp/filterapi/filterapi_test.go` | `FilterStagePeerChain` sorts after Protocol/Policy/Annotation; ordered accessors return keys with the funcs | new |
| `TestFilterPriorityOrdering` / `TestFilterSameStageNameBreaksTie` / `TestEgressFilterOrdering` | `internal/component/bgp/filterapi/filterapi_test.go` | Existing Stage/Priority/name sort still holds (:302/:336/:359) | existing |
| `TestFilterDeltaParseCallCount` | `internal/component/bgp/reactor/filter_delta_test.go` | Each filter text still parsed exactly once on the modify path (:1264) | existing |
| `TestPolicyFilterChain*` | `internal/component/bgp/reactor/filter_chain_test.go` | `PolicyFilterChain` executor semantics unchanged (accept/reject/modify/teardown/raw/inactive) | existing |
| zero-alloc dispatch | `internal/component/bgp/reactor/filter_dispatch_alloc_test.go` | Merged ingress pass adds no per-UPDATE allocation (R-7) | extend |

### Boundary Tests (MANDATORY for numeric inputs)
- Empty `r.orderedIngressSteps` (no plugins linked): pass is a no-op, route accepted unchanged.
- Peer with `ImportFilters` but `r.api == nil`: policy-chain step is a no-op accept (fail-open per current guard), no serialization.
- `inactive:`-only import chain: loop-detection suppressed (`LoopDisabled`), policy-chain step skips all refs.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `community-strip` | `test/plugin/community-strip.ci` | Configured community filter still strips on received UPDATE | must pass unchanged |
| `community-cumulative` | `test/plugin/community-cumulative.ci` | Cumulative community tags accumulate (A-2 dedup check) | must pass unchanged |
| `role-otc-egress-stamp` | `test/plugin/role-otc-egress-stamp.ci` | OTC still stamped at Annotation stage on egress | must pass unchanged |
| `loop-cluster-list` | `test/plugin/loop-cluster-list.ci` | Loop detection still runs first (Protocol stage) | must pass unchanged |
| `aspath-filter-reject` | `test/plugin/aspath-filter-reject.ci` | AS-path policy reject still drops the route | must pass unchanged |
| `filter-family-import-teardown` | `test/plugin/filter-family-import-teardown.ci` | Import teardown still triggers NOTIFICATION + close | must pass unchanged |
| `policy-chain-plain-names` | `test/plugin/policy-chain-plain-names.ci` | Plain-name chain still resolves and orders correctly | must pass unchanged |
| `policy-test-remove-private-as` | `test/plugin/policy-test-remove-private-as.ci` | remove-private-AS modify still applied | must pass unchanged |

### Interop Tests (MANDATORY for protocol features)
No new protocol behavior. The existing OTC (RFC 9234), loop-detection (RFC 4456), and LLGR (RFC 9494) `.ci` scenarios above are the interop coverage; all must pass byte-unchanged. No new interop scenario is required (refactor, not a protocol change).

## Files to Modify
- `internal/component/bgp/filterapi/filterapi.go` - add `FilterStagePeerChain int = 300` constant (declared terminal stage, doc: "external per-peer configured filter chain; runs after all in-process filters"); add ordered accessors that return entries WITH their Stage/Priority/Name (e.g. `IngressOrdered()`/`EgressOrdered()` returning sorted `[]Filter`) plus the Stage/Priority/Name comparator so the reactor can merge-sort a reactor-bound step. No reactor state.
- `internal/component/bgp/reactor/reactor.go` - add `orderedIngressSteps`/`orderedEgressSteps` fields; build them once in `startAPIServer` (:1189 area) from `filterapi.IngressOrdered()`/`EgressOrdered()` plus the reactor-bound policy-chain step (`FilterStagePeerChain`), merge-sorted.
- `internal/component/bgp/reactor/reactor_notify.go` - collapse the two back-to-back ingress blocks (:393-443, :458-531) into ONE stage-ordered pass over `orderedIngressSteps`; extract the System-2 body into a reactor method the policy-chain step invokes; define the common `ingressStepResult`; delete the second block.
- `internal/component/bgp/reactor/reactor_api_forward.go` - in `forwardUpdateCore` (:443-528), collapse the in-process egress loop (:474-487) and the export policy chain (:488-528) into ONE stage-ordered pass over `orderedEgressSteps`; preserve `ModAccumulator` per-peer semantics.
- `internal/component/bgp/reactor/filter_chain.go` - `PolicyFilterChain` becomes the body of the single ordered policy-chain step; no change to its RPC/teardown/raw/inactive semantics.
- (No change to `egress_inject_filter.go`, `peer_initial_sync.go`, `forward_rs.go` — intentionally out of scope per the egress decision; documented in AC-6 and Known Limitations.)

## Files to Create
- `internal/component/bgp/reactor/unified_filter_order_test.go` - characterization + order tests (`TestUnifiedIngressReproducesLegacyOrder`, `TestUnifiedIngressOrder`).

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | N/A — no config surface; internal ordering only | - |
| YANG validation constraints | N/A | - |
| YANG custom validators | N/A | - |
| CLI commands/flags | N/A — no new command | - |
| CLI grammar (action before identifier) | N/A | - |
| Editor autocomplete | N/A | - |
| Functional test for new RPC/API | N/A (no new RPC); existing `.ci` suite guards behavior | `test/plugin/*.ci` |
| Pipe completeness | N/A | - |
| Env var registration | N/A | - |
| Doctor check for runtime dependencies | N/A — no new file path/socket/port/module/binary/cert; reuses existing `r.api` RPC path | - |
| Prometheus counters/metrics | N/A — no new observable state; filtering outcomes already counted | - |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No — internal refactor, no user-visible change | - |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No — registered filter set unchanged | - |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No — external text/RPC protocol unchanged | - |
| 9 | RFC behavior implemented? | No — OTC/loop/LLGR behavior byte-preserved | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | **Yes** — the filter-ordering model gains a terminal stage and a single ordered ingress/egress pass | `docs/architecture/core-design.md` (filter pipeline section) and/or `docs/architecture/api/architecture.md`: document `FilterStagePeerChain` and that the external per-peer chain is now a declared terminal stage merge-sorted into the same pass, with a `<!-- source: internal/component/bgp/filterapi/filterapi.go -- FilterStagePeerChain -->` anchor |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin/event/command/capability inventory changed? | No | - |
| 16 | Any changed source file referenced by existing doc source anchors? | **Check** — `filter_chain.go` / `reactor_notify.go` / `reactor.go` / `filterapi.go` have `// Design:` anchors; grep `docs/` for `source:` pointing at them and update stale claims | grep `docs/` during stage 14 |
| 17 | Existing docs show config/CLI/API examples for this area? | No config/CLI examples for filter ordering | - |

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table — `unified_filter_order_test.go` characterization first |
| 4. Implement (TDD) | Implementation Phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-verify` |
| 7. Critical review | Critical Review Checklist below |
| 8-10. Fix/re-verify/repeat | Until clean |
| 11. Deliverables review | Deliverables Checklist below |
| 12. Security review | Security Review Checklist below |
| 13. Re-verify | Re-run stage 6 |
| 14. Present summary | Executive Summary |

### Implementation Phases

1. **Phase: Characterization (MANDATORY FIRST)** — lock the current cross-system order before any production change.
   - Tests: `TestUnifiedIngressReproducesLegacyOrder` — an order-recording harness (register probe filters at Protocol/Policy/Annotation + a probe policy filter, run a received UPDATE through the CURRENT two-block code, assert observed order is `loop-position → community-position → redistribute-position → OTC-position → policy-chain`). Run the existing `.ci` suite as the behavioral baseline.
   - Files: `internal/component/bgp/reactor/unified_filter_order_test.go` (new). No production change.
   - Verify: green against current code (captures baseline, policy chain LAST). Self-critical review.
2. **Phase: filterapi ordered-step seam** — add `FilterStagePeerChain=300`, ordered accessors returning keyed entries, and the comparator.
   - Tests: `TestPeerChainStageSortsLast`; existing `TestFilterPriorityOrdering`/`TestEgressFilterOrdering` still green.
   - Files: `filterapi/filterapi.go`, `filterapi/filterapi_test.go`.
   - Verify: leaf stays stdlib-only (no reactor import); sort correct. Self-critical review.
3. **Phase: Unified ingress pipeline** — build `orderedIngressSteps` at `startAPIServer`; run one pass in `notifyMessageReceiver`; define `ingressStepResult`; extract the policy-chain body; delete the second block.
   - Tests: `TestUnifiedIngressOrder`, characterization still green; ingress `.ci` (`community-strip`, `community-cumulative`, `loop-cluster-list`, `aspath-filter-reject`, `filter-family-import-teardown`, `policy-chain-plain-names`, `policy-test-remove-private-as`); `TestFilterDeltaParseCallCount`; alloc test.
   - Files: `reactor.go`, `reactor_notify.go`, `filter_chain.go`.
   - Verify: characterization green (order preserved); grep shows one ingress site (AC-6). Self-critical review.
4. **Phase: Unified egress pass (order-only)** — collapse ONLY the two adjacent egress filter blocks in `forwardUpdateCore` (`reactor_api_forward.go:474-528`) into one declared-order pass over `orderedEgressSteps`. Everything around that region stays byte-identical.
   - **Precise scope (replace `:474-528` only):** the in-process egress loop (`:474-487`, writes into the shared `&mods`, `safeEgressFilter`, suppress → `continue`) and the export policy chain (`:488-528`, reads the ORIGINAL `update.WireUpdate`, produces `exportWireOverride` via raw or text-delta `buildModifiedPayload`, reject → `continue`). Do NOT touch: the RS-client community strip (`:471-473`, writes `mods` BEFORE), the RFC 4456 RR CLUSTER_LIST/ORIGINATOR_ID injection (`:530-534`, writes `mods` AFTER), `applyFactsNextHop`/`applyFactsSendCommunity` (`:536-537`, AFTER), or the `peerBaseWire = exportWireOverride ?? update.WireUpdate` combine (`:539-541`).
   - **Egress step model differs from ingress (do NOT share the type):** an `orderedEgressStep` runs `func(src, dest, payload, meta, *mods) egressStepResult` where `egressStepResult { accept bool; wireOverride *wireu.WireUpdate }`. In-process egress steps write into the shared `&mods` and return `accept` (never set `wireOverride`) — exactly `safeEgressFilter` today. The policy-chain step reads the ORIGINAL payload (`update.WireUpdate`, NOT mods-adjusted — mods are deferred), keeps the `len(facts.exportFilters) > 0 && a.r.api != nil` gate, and returns `accept` + optional `wireOverride` (from `decodeFilterRawOverride` or `buildModifiedPayload`). `accept == false` from any step → `continue` to next peer (both suppress and reject collapse to skip, as today). Teardown does NOT apply on egress (import-only, gated in `policyFilterFunc`).
   - **Why bytes stay identical:** in-process egress steps sort at Stage ≤ 200, the policy-chain step at `FilterStagePeerChain=300`, so the pass runs in-process-then-policy-chain — the exact current sequence. In-process steps and the policy-chain step both read the original payload independently (egress in-process filters defer to `mods`; the policy chain produces a full override), so their relative order does not change today's output; it only makes a FUTURE stage-300+ egress filter interleave correctly. Per-peer `ModAccumulator` freshness (`var mods` at `:469`) is untouched.
   - Tests: `role-otc-egress-stamp.ci`, `policy-test-remove-private-as.ci`, export-side `.ci`; `forward_update_test.go` green; `forward_pool_test.go` / `forward_pool_supersede_test.go` green (progressive build unchanged).
   - Files: `reactor.go` (build `orderedEgressSteps`), `reactor_api_forward.go` (replace `:474-528`).
   - Verify: `mods` write order preserved (RS strip → in-process egress → RR injection → nexthop/community); `grep` shows one ordered egress site for the both-kinds path and the surrounding `mods.Op` calls unchanged; RS/injected/default-originate paths unchanged (`grep`). Self-critical review.
5. **Docs** → update `docs/architecture/core-design.md` (or `api/architecture.md`) per Documentation Update Checklist row 12/16 with a source anchor; `make ze-doc-test`.
6. **Full verification** → `make ze-verify` (lint + all ze tests except fuzz).
7. **Complete spec** → fill audit tables, write learned summary to `plan/learned/NNN-unify-filters.md`, two commits per planning.md.

### Critical Review Checklist (/implement stage 7)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness (order) | Merge reproduces `loop → community → redistribute → OTC → policy-chain` EXACTLY; policy chain runs LAST (Stage 300), after OTC (R-1); characterization test green |
| Correctness (semantics) | reject / text-delta modify / raw terminal / import teardown / on-error / 5s timeout / declared-attr validation / `inactive:` skip all preserved through `ingressStepResult` (AC-3) |
| Data flow | In-process steps operate on `wireUpdate.Payload()` zero-copy; text serialization stays gated to peers with policy filters AND `r.api != nil` (AC-4); step list built once at `startAPIServer` (R-7) |
| Egress scope | ONLY `forwardUpdateCore` unified; `forward_rs.go`, `exportFilterForBody`, `defaultOriginateFilterAccepts` membership byte-unchanged (AC-5, AC-6, R-3); `ModAccumulator` per-peer preserved (R-6) |
| Registration over hardcoding | Policy-chain step joins via declared `FilterStagePeerChain` merge-sort, not a hardcoded block; in-process filters still register via `filterapi.Register` (`ai/rules/plugin-self-containment.md`) |
| Leaf constraint | `filterapi` gains only the stage constant + ordering accessors/comparator; no reactor import, no `r.api` (R-2) |
| Rule: no-layering | The second back-to-back ingress block and the egress two-block structure are fully deleted; no dead ordering path remains |
| Performance | `filter_dispatch_alloc_test` shows no new per-UPDATE allocation; `ze-perf` UPDATE throughput not regressed |

### Deliverables Checklist (/implement stage 11)
| Deliverable | Verification method |
|-------------|---------------------|
| `FilterStagePeerChain=300` declared | `grep -n FilterStagePeerChain internal/component/bgp/filterapi/filterapi.go` |
| Single ordered ingress site | `grep -n "range r.orderedIngressSteps\|PolicyFilterChain(" internal/component/bgp/reactor/reactor_notify.go` shows one ordered pass, no second block |
| Single ordered egress site (both-kinds path) | `grep -n "range r.orderedEgressSteps" internal/component/bgp/reactor/reactor_api_forward.go`; the two positional blocks gone |
| Three single-kind egress paths unchanged | `grep -n PolicyFilterChain internal/component/bgp/reactor/{egress_inject_filter,peer_initial_sync}.go` unchanged; `forward_rs.go` still no PolicyFilterChain |
| Characterization test | `bin/ze-test` unit run of `TestUnifiedIngressReproducesLegacyOrder` green |
| `.ci` suite unchanged | `make ze-plugin-test` all pass |
| No new allocation | `go test -run FilterDispatchAlloc` / alloc test green |
| Doc + anchor | `grep -rn "FilterStagePeerChain" docs/` returns the anchored claim; `make ze-doc-test` green |

### Security Review Checklist (/implement stage 12)
| Check | What to look for |
|-------|-----------------|
| Untrusted input (wire UPDATE) | The merged pass processes attacker-controlled UPDATE bytes; every in-process step keeps its `safeIngressFilter`/`safeEgressFilter` panic guard (fail-closed). Verify no step bypasses the guard |
| Fail-closed on panic | A panicking step still rejects (ingress) / suppresses (egress) the route — no accept-on-panic regression |
| Teardown authority | Import-only teardown still gated to `direction == import` (`filter_chain.go:426`); a misbehaving export filter cannot drop sessions |
| Resource exhaustion | No unbounded allocation added on the hot path; step list bounded and built once; text serialization still gated so a no-policy-filter peer cannot be made to pay serialization cost |
| Error leakage | Filter panic / RPC error logging unchanged; no new sensitive data (peer keys, tokens) in logs |
| RPC timeout | 5s per-filter timeout preserved (`policyFilterTimeout`); merged pass does not remove the context deadline |

## Mistake Log

### Wrong Assumptions
| Assumption | Reality | Corrected by |
|-----------|---------|--------------|
| (Original spec) Current order is `loop → community → policy-chain → OTC`; place the policy-chain step at Stage Policy, priority above in-process Policy filters | `r.ingressFilters` runs ALL in-process filters (incl. `bgp-role`/OTC at Annotation) in System 1 (`reactor_notify.go:415`) BEFORE the policy chain in System 2 (:466). The policy chain runs LAST, after OTC | Read producers; `FilterStagePeerChain=300` (terminal), AC-2 rewritten; locked by `TestUnifiedIngressReproducesLegacyOrder` |
| (Original spec) filter set is loop/community/policy/OTC (4) | 5 registered filters: adds `bgp-redistribute` (Policy, ingress) and `bgp-gr` (Annotation, egress LLGR) | grep of `filterapi.Register` |
| (Original spec) slices built "at construction (:1189-1190)" | Built in `startAPIServer()` (`reactor.go:1142`), not `New` (:355). Line numbers right | Read `reactor.go` |
| (Original spec) `TestFilterOrderingByStagePriorityName` is the existing ordering test | That name does not exist; real tests are `TestFilterPriorityOrdering`/`TestFilterSameStageNameBreaksTie`/`TestEgressFilterOrdering` | grep of `filterapi_test.go` |
| (Original spec A-3) `PeerFilterInfo` must be extended to carry the per-peer refs | The policy-chain step runs where `peer.settings.ImportFilters`/`ExportFilters` are already in scope; no `PeerFilterInfo` change needed | Read call sites |
| (Original spec R-3) 3 export `PolicyFilterChain` sites are symmetric and all route through one egress pass | The 3 export sites are asymmetric (forward = both kinds; RS = in-process only; injected = policy-chain only; default-originate = single-filter dry-run). Only `forwardUpdateCore` is unified | Egress heterogeneity census |

### Failed Approaches
(none yet)

## Design Insights
- The correctness fix is entirely about ordering, not executors. Making the external per-peer chain a *declared terminal stage* (`FilterStagePeerChain`) turns "System 1 block then System 2 block" from a code-position accident into an inspectable property, while reproducing the exact current behavior.
- The egress side is genuinely heterogeneous; "one ordered pass" is only meaningful where both filter kinds already co-occur (`forwardUpdateCore`). Recognizing that keeps the change a pure refactor instead of an accidental behavior change.
- **Ingress and egress step models are asymmetric — do not force a shared step type.** Ingress in-process filters compose *sequentially* (each rebuilds the payload the next sees; the policy chain serializes the post-in-process payload) → `ingressStepResult {accept, modifiedPayload, teardown, notify}`. Egress in-process filters run in *parallel/deferred* fashion (each writes into a shared `ModAccumulator`; the policy chain reads the ORIGINAL payload and produces a separate `exportWireOverride`) → `egressStepResult {accept, wireOverride}` with no teardown (import-only). The two loops merge into declared-order passes independently; the outcome types are different by design.

## Core Insight
Unify the ORDERING mechanism (one Stage-ordered pass), keep BOTH executors (in-process wire-bytes and out-of-process text/RPC). The policy chain becomes one step tagged `FilterStagePeerChain=300`, merge-sorted last — declared, not positional.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Winner: `filterapi` (in-process, wire-bytes, Stage/Priority ordering) | Make `filterapi` a special case of `PolicyFilterChain` (text/RPC) | `filterapi` matches Ze's zero-copy wire-bytes identity and already has the ONE thing the bug needs: explicit, inspectable ordering. `PolicyFilterChain` has only positional order and pays text serialization on the hot path |
| Keep BOTH executors; unify only the ORDERING; policy chain = ONE ordered step | Delete `PolicyFilterChain` and reimplement all policy/RPC filters in-process | Same LAYER, complementary EXECUTORS. The external plugin SDK depends on the text/RPC protocol; deleting it is a much larger, separate effort |
| Policy-chain step = new terminal stage `FilterStagePeerChain=300` (after Annotation) | Original spec: Stage Policy, priority above in-process Policy filters (before Annotation) | The original placement is WRONG: the code runs the policy chain LAST, after OTC/Annotation. A terminal stage reproduces the current order exactly (R-1). Inventing a stage after Annotation is the smallest change that makes the true order declared |
| Reactor binds the step at `startAPIServer`; `filterapi` gains only a stage constant + ordering accessors/comparator | Register the step globally via `filterapi.Register` at init() | The step needs `r.api` for RPC; `filterapi.Register` is init-only/stateless, and pushing reactor state into the stdlib-only leaf would break layering (R-2) |
| Egress: unify ONLY `forwardUpdateCore`; other 3 export paths untouched | Route all export sites through one identical pass | The 4 egress paths run different filter combinations by design; forcing uniformity is a behavior change, not a refactor (R-3). User-approved scope |
| Common `ingressStepResult` superset outcome | Two separate loops kept | A single pass needs one outcome type both executors produce; the struct is a superset (accept/modify/teardown/notify) so no semantic is dropped (R-5) |

## Known Limitations
- This spec unifies ORDERING, not execution model. External RPC policy filters still serialize the UPDATE to text (the plugin SDK's text protocol). Text serialization stays gated to peers with configured policy filters, so the no-policy-filter hot path is unaffected, but a peer WITH policy filters still pays it.
- Fully eliminating text serialization for policy filters that could run in-process on wire bytes (community, prefix, aspath) is a phased follow-on per-filter migration, out of scope here.
- `filter_community`'s dual registration (in-process filterapi filter plus RPC-capable engine) is preserved as-is; deduplicating it is deferred pending A-2 validation.
- Egress is order-only per the scope decision: `forward_rs.go` (RS fast path, in-process filters only), `exportFilterForBody` (injected routes, policy chain only), and `defaultOriginateFilterAccepts` (single-filter dry-run) keep their current filter membership. Fully unifying their egress filtering is a separate spec (would change behavior).

## RFC Documentation
No RFC behavior changes. OTC (RFC 9234), loop-detection (RFC 4456), LLGR (RFC 9494) outcomes byte-preserved. RFC short summaries confirmed present: `rfc/short/rfc4271.md`, `rfc4456.md`, `rfc9234.md`, `rfc9494.md`, `rfc7313.md`.

## Implementation Summary

### What Was Implemented
- `filterapi.FilterStagePeerChain = 300` (terminal stage), `IngressOrdered()`/`EgressOrdered()` (sorted entries with keys), and exported `LessOrder` comparator; `sortedFilters` refactored to use `LessOrder` (single source of truth). `filterapi.go`.
- Reactor-local `orderedIngressStep`/`orderedEgressStep`, `ingressStepResult`/`egressStepResult`, `buildOrderedIngressSteps`/`buildOrderedEgressSteps`, and the extracted executors `runIngressPolicyChain`/`runEgressPolicyChain`. New file `filter_ordered.go`.
- `reactor.go`: removed the `ingressFilters` field (no remaining consumer); added `orderedIngressSteps`/`orderedEgressSteps`; built both in `startAPIServer` alongside `egressFilters`.
- `reactor_notify.go`: the two back-to-back ingress blocks collapsed into ONE ordered pass over `orderedIngressSteps`; second block deleted; `unsafe` import dropped.
- `reactor_api_forward.go`: `forwardUpdateCore`'s two egress blocks (`:474-528`) collapsed into ONE ordered pass over `orderedEgressSteps`; surrounding `mods` writers and the `peerBaseWire` combine unchanged; `unsafe` import dropped.
- Tests: `unified_filter_order_test.go` (order/characterization), `filterapi_test.go` (+`TestPeerChainStageSortsLast`, `TestLessOrderMatchesSort`), and a test helper `orderedEgressStepsFromFuncs` + updated 4 `forward_update_test.go` constructions.
- Docs: `docs/architecture/core-design.md` Ingress Filter Pipeline + Policy Filter Chain sections rewritten with source anchors; `ai/digests/bgp-reactor.md` anchors updated; discovery indexes regenerated.

### Bugs Found/Fixed
- **Original spec had the target order backwards** (would have placed the policy chain at Policy stage, before OTC). Code shows the policy chain runs LAST, after OTC. Fixed by the terminal `FilterStagePeerChain=300` and a characterization test. See Mistake Log.

### Documentation Updates
- `docs/architecture/core-design.md`: "Ingress Filter Pipeline" now describes one stage-ordered pass; "Policy Filter Chain" notes the merge-sort at `FilterStagePeerChain`. Source anchors point at `reactor_notify.go`, `filter_ordered.go`, `filterapi.go`.
- `ai/digests/bgp-reactor.md`: line anchors updated for the shrunk `reactor_notify.go` (508/568/573) and the ingress prose updated to the unified pass.

### Deviations from Plan
- Minor: the consolidated `modifiedPayload` rebuild logs `sessionLogger().Debug("modified WireUpdate.Attrs error", ...)` for the policy-chain path too, where the original raw/text-delta paths assigned the parse error without a debug log. Debug-only, error-path-only; benign and intentional (one rebuild path).
- Egress debug log for attr-extraction error uses the peer's string address (`destAddrStr`) rather than the `netip.Addr` (`facts.addr`). Cosmetic, debug-only.
- Implementation order was Phase 2 (filterapi seam) before the reactor-level characterization test, because the merged-order unit test depends on the seam. The behavioral baseline (8 filter `.ci` tests, all green pre-change) served as the characterization lock, per spec Phase 1.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Evidence |
|-------------|--------|----------|
| All received-UPDATE filtering through ONE stage-ordered pipeline | done | `reactor_notify.go:417` single pass; no `PolicyFilterChain` in that file |
| Cross-system order is a declared Stage, not code position | done | `filterapi.FilterStagePeerChain`; `TestUnifiedIngressReproducesLegacyOrder` |
| Preserve every observable filtering outcome | done | 8 filter `.ci` pass byte-unchanged; full functional suite green |
| Egress order-only at `forwardUpdateCore`; other paths unchanged | done | `reactor_api_forward.go:484`; grep shows RS/injected/default-originate untouched |

### Acceptance Criteria
| AC | Status | Evidence (file:line / test) |
|----|--------|------------------------------|
| AC-1 one ordered pass | met | `reactor_notify.go:417`, `TestUnifiedIngressOrder` |
| AC-2 order = Protocol<Policy<Annotation<policy-chain | met | `TestUnifiedIngressReproducesLegacyOrder`, `TestPeerChainStageSortsLast` |
| AC-3 System-2 semantics preserved | met | `runIngressPolicyChain` (filter_ordered.go); `filter-family-import-teardown.ci`, `aspath-filter-reject.ci`, `policy-test-remove-private-as.ci`, `community-strip.ci` |
| AC-4 no serialization without policy filters | met | `runIngressPolicyChain` early return; `TestOrderedStepsEmptyRegistry` |
| AC-5 egress one declared-order pass, ModAccumulator preserved | met | `reactor_api_forward.go:480`; `role-otc-egress-stamp.ci`, `forward_update_test.go` (Mods/DirectCopy) |
| AC-6 one ingress + one egress site; second block gone | met | grep evidence (Deliverables Checklist) |

### Tests from TDD Plan
| Test | Status |
|------|--------|
| `TestUnifiedIngressReproducesLegacyOrder`, `TestUnifiedIngressOrder`, `TestUnifiedEgressOrder`, `TestOrderedStepsEmptyRegistry` | added, pass |
| `TestPeerChainStageSortsLast`, `TestLessOrderMatchesSort` | added, pass |
| `TestFilterPriorityOrdering`/`TestFilterSameStageNameBreaksTie`/`TestEgressFilterOrdering` | pass (unchanged) |
| `TestFilterDeltaParseCallCount` | pass (parse-once preserved) |
| 8 filter `.ci` | pass byte-unchanged |

### Files from Plan
All planned files modified; `filter_ordered.go` created; `egress_inject_filter.go`/`peer_initial_sync.go`/`forward_rs.go` intentionally NOT changed (per scope).

### Audit Summary
Every AC met with test/grep evidence; no deferrals; no in-scope work outstanding.

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| ALL received-UPDATE filtering flows through ONE stage-ordered pipeline; cross-system order is a declared Stage, not code position | functional test + source audit | `TestUnifiedIngressReproducesLegacyOrder` green; `grep` shows one ordered ingress site and no second block (AC-6); `FilterStagePeerChain` declared |
| Every externally observable filtering outcome preserved (pure refactor) | functional test suite | `make ze-plugin-test` — all 8 filter `.ci` tests pass byte-unchanged |
| Policy-chain step reproduces the exact pre-refactor order (policy chain LAST, after OTC) | characterization test | `TestUnifiedIngressReproducesLegacyOrder` asserts `loop → community → redistribute → OTC → policy-chain` |
| Egress `forwardUpdateCore` runs one declared-order pass; other egress paths unchanged | functional test + audit | `role-otc-egress-stamp.ci` + `policy-test-remove-private-as.ci` pass; `grep` shows RS/injected/default-originate untouched |
| No hot-path regression | benchmark | `ze-perf` UPDATE throughput unchanged; `filter_dispatch_alloc_test` no new alloc |

## Review Gate

Adversarial diff review (fresh agent) run against the 9 divergence risks in the
Critical Review Checklist. Result: faithful pure-refactor of filtering outcomes.
All 9 risks CONFIRMED-OK (ingress order, teardown types/guards, hot-path gating,
modifiedPayload rebuild, egress mods order, override-vs-mods, `unsafe.String`
lifetime, three untouched egress paths, guard/meta/sourcePeerStr placement).

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | LOW (observability-only, NOT a bug) | Consolidated `modifiedPayload` rebuild logs a `Debug` line on `Attrs()` parse error for the policy-chain raw/text-delta path, where the original System-2 rebuild suppressed it. `msg.RawBytes/WireUpdate/AttrsWire/ParseError` identical; only reachable via a filter that emits a malformed UPDATE body. | `reactor_notify.go` unified rebuild | KEEP (intentional: one rebuild path; extra diagnostic on a malformed-filter-output error is benign/beneficial). Recorded in Deviations. |

No behavioral divergence remains; no re-run needed.

## Pre-Commit Verification

### Files Exist (ls / grep)
- `internal/component/bgp/reactor/filter_ordered.go` (new), `internal/component/bgp/reactor/unified_filter_order_test.go` (new).
- `grep FilterStagePeerChain internal/component/bgp/filterapi/filterapi.go` → present.

### AC Verified (grep/test)
- One ordered ingress site `reactor_notify.go:417`; no `PolicyFilterChain` in `reactor_notify.go`.
- One ordered egress site `reactor_api_forward.go:484`; `r.ingressFilters` fully removed.
- `TestUnifiedIngressReproducesLegacyOrder` + `TestPeerChainStageSortsLast` green.

### Wiring Verified (end-to-end)
- All 8 filter `.ci` scenarios pass byte-unchanged (baseline captured green pre-change; re-run green post-change).
- `role-otc-egress-stamp.ci` proves egress OTC stamp still ordered before the export chain.

### Assumptions Resolved
| ID | Status | Evidence |
|----|--------|----------|
| A-1 | confirmed | code read + `TestUnifiedIngressReproducesLegacyOrder` |
| A-2 | confirmed | `community-strip.ci` + `community-cumulative.ci` pass |
| A-3 | confirmed | `runIngressPolicyChain`/`runEgressPolicyChain` read `peer.settings` directly; no `PeerFilterInfo` change |
| A-4 | confirmed | all `.ci` run through the real `startAPIServer` build path |

### Documentation Verified
- `make ze-doc-test` PASS (all source anchors valid, 3204 digest anchors resolve, indexes fresh).

### Test/Verify Results
- `make ze-lint-changed`: 0 issues. `make ze-unit-test`: 451 packages ok, 0 failures. `make ze-functional-test`: all 18 suites PASS. `make ze-doc-test`: PASS.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Wiring Test table complete - every row has a concrete test name, none deferred
- [ ] Characterization test proves the merged order reproduces the pre-refactor order, policy chain LAST (R-1)
- [ ] Exactly one ordered ingress site and one ordered egress (both-kinds) site remain; three single-kind egress paths documented as separate (AC-6, grep evidence)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/component/bgp/...`)
- [ ] Risks & Assumptions: every A-N confirmed or broken; surviving risks copied forward
- [ ] Goal Validation table filled with concrete evidence

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Implementation Audit complete
- [ ] Docs updated (`docs/architecture/*` + source anchor) and `make ze-doc-test` green
- [ ] Known Limitations reviewed with user (phased in-process migration + egress scope)
