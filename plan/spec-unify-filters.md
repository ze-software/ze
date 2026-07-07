# Spec: unify-filters

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-07-06 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/api/architecture.md` - BGP route filter pipeline contract (the `filterapi` seam)
4. `internal/component/bgp/reactor/reactor_notify.go` (the two back-to-back ingress blocks), `internal/component/bgp/filterapi/filterapi.go`, `internal/component/bgp/reactor/filter_chain.go`

## Task

DESIGN-REVIEW.md finding 2 (row "BGP filtering", the review's #1 priority, correctness-relevant): a received UPDATE is run through TWO independent filtering systems back-to-back inside `notifyMessageReceiver`, and the ordering BETWEEN the two systems is purely positional code-order in that one function, not a declared, inspectable property.

- System 1: the in-process `filterapi` pipeline. Operates on wire bytes (zero-copy). Explicit ordering by Stage, then Priority, then name. Applied at `reactor_notify.go:393-443`.
- System 2: the `PolicyFilterChain`. Serializes the UPDATE to text, RPCs out to external plugin filters, converts the returned text delta back into wire attribute modifications. Ordering is positional over the peer's configured filter list only. Applied at `reactor_notify.go:458-531`.

Both run only on `DirectionReceived`, back-to-back, System 1 fully before System 2. The relative order of, say, an in-process community filter (System 1, policy stage) and an RPC policy filter (System 2) is fixed by which block appears first in the function body, not by any declared stage. That implicit ordering is the correctness concern.

Goal: pick a winner, port the loser's distinguishing features onto it, and route ALL received-UPDATE filtering through ONE stage-ordered pipeline so the cross-system order becomes EXPLICIT (declared stages) instead of positional. Preserve every externally observable filtering outcome (this is a refactor).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/architecture.md` - BGP route filter pipeline (the `filterapi` seam header references it)
  → Decision: `filterapi` is the BGP-owned filter seam; protocol filter knowledge stays out of the generic plugin registry, so any unified ordering must live in `filterapi` or the reactor, not in `internal/component/plugin/registry`.
  → Constraint: `filterapi` is a leaf package (stdlib only) so filter plugins, the reactor, and protocol filters import it without cycles; the reactor may not push reactor-instance state back into it.
- [ ] `ai/rules/plugin-self-containment.md` - registration over hardcoding
  → Constraint: filters register via `filterapi.Register` at init(); a new ordered "policy-chain" step must be discovered through declared Stage/Priority, not a new hardcoded code-position in `notifyMessageReceiver`.
- [ ] `ai/rules/buffer-first.md` - wire encoding on the hot path
  → Constraint: the received-UPDATE path is the hot path; text serialization (`AppendUpdateForFilter`) is paid only when a peer has configured RPC policy filters and must stay gated that way.

**Key insights:**
- The two systems are the SAME layer (ingress route filtering on received UPDATEs) with two complementary execution models (in-process wire-bytes vs out-of-process text/RPC). What is genuinely redundant and buggy is the ORDERING mechanism, not the executors.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/component/bgp/reactor/reactor_notify.go` - `notifyMessageReceiver` (starts :219). Runs System 1 ingress filters at :393-443 (guarded by `DirectionReceived && wireUpdate != nil && len(r.ingressFilters) > 0`, loop over `r.ingressFilters` calling `safeIngressFilter` on `wireUpdate.Payload()`), then System 2 policy chain at :458-531 (same received guard plus `peer.settings.ImportFilters`), back-to-back.
- [ ] `internal/component/bgp/filterapi/filterapi.go` - registry for System 1. `Filter{Name, Stage, Priority, Ingress, Egress}` (:162). `IngressFilters()` (:261) / `EgressFilters()` (:290) return funcs sorted by `sortedFilters` (:239) on Stage, then Priority, then name. Stage constants (:134): Protocol=0, Policy=100, Annotation=200. Also `AttrModHandler` progressive-build handlers and `EnableRSForwarding`.
- [ ] `internal/component/bgp/reactor/filter_chain.go` - System 2. `PolicyFilterChain(filterRefs, direction, ...)` (:175) iterates `for _, ref := range filterRefs` (:181), positional; reject short-circuits; supports text-delta modify (`applyFilterDelta`), raw full-payload override (terminal), and import-only `Teardown`. `policyFilterFunc` (:366) does the RPC (`CallFilterUpdate`), per-filter on-error (`OnErrorAccept`/reject), 5s timeout, declared-attr validation (`validateModifyDelta`).
- [ ] `internal/component/bgp/reactor/reactor.go` - `r.ingressFilters = filterapi.IngressFilters()` and `r.egressFilters = filterapi.EgressFilters()` built once at construction (:1189-1190); struct fields at :281-282.
- [ ] `internal/component/bgp/config/peers.go` - builds per-peer `ImportFilters`/`ExportFilters` (:155-167), validates and canonicalizes refs to `<plugin>:<filter>` (:185-202), prepends loop-detection default filters (:672), applies loop-detection config by matching filter names (:607-641).
- [ ] `internal/component/bgp/plugins/filter_community/register.go` - registers BOTH an in-process `filterapi.Filter` at Stage Policy priority 0 (ingress+egress) AND a generic `registry.Registration` engine; also registers community `AttrModHandler`s.
- [ ] `internal/component/bgp/plugins/role/register.go`, `internal/component/bgp/reactor/filter/register.go` - `bgp-role` (Annotation stage, OTC) and `loop` (Protocol stage) in-process filterapi filters.
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go` - export-side `PolicyFilterChain` (:498) plus `EgressFilters` application on the forward path (the egress twin of the ingress duplication).
- [ ] `internal/component/bgp/reactor/peer_initial_sync.go`, `internal/component/bgp/reactor/egress_inject_filter.go` - the two other export-side `PolicyFilterChain` call sites (:734, :46).

**Behavior to preserve:** (unless user explicitly said to change)
- The net filtering outcome (accept/reject/modify) for every received and forwarded UPDATE under every existing `.ci` scenario. In particular the current effective order: Protocol-stage in-process (loop) first, then Policy-stage in-process (community), then the per-peer RPC policy chain, then Annotation-stage in-process (OTC).
- System 2 rich semantics: reject short-circuit, text-delta modify, raw hex full-payload override (terminal), import-only `Teardown` (NOTIFICATION + session close), per-filter `OnErrorAccept`/reject, 5s per-filter timeout, declared-attribute validation, `inactive:` skip.
- Loop-detection coupling: an `inactive:`-prefixed loop-detection name in `ImportFilters` still suppresses the in-process `LoopIngress` for that peer (`ps.LoopDisabled`).
- Hot-path gating: no text serialization (`AppendUpdateForFilter`) unless the peer has at least one configured policy filter.
- `parseFilterAttrs` still runs exactly once per filter text on the modify path (`TestFilterDeltaParseCallCount`).

**Behavior to change:** None - internal refactor, all filtering outcomes preserved. The only change is structural: the cross-system order becomes a declared Stage/Priority property instead of implicit code-position.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Received BGP UPDATE bytes arrive on the peer session read goroutine and reach `notifyMessageReceiver` (`reactor_notify.go:219`) as a `*wireu.WireUpdate` (lazy-parsed, zero-copy) with `direction == rpc.DirectionReceived`.
- Format at entry: UPDATE body wire bytes (`wireUpdate.Payload()`), no BGP header.

### Transformation Path
1. Received UPDATE reaches `notifyMessageReceiver`; per-peer counters and observers run.
2. System 1 ingress pass (`:393-443`): `r.ingressFilters` (already Stage/Priority/name sorted) run over the payload; any filter may reject (drop) or return a `modifiedPayload` that rebuilds the `WireUpdate`.
3. System 2 policy pass (`:458-531`): if the peer has `ImportFilters`, the payload is serialized to text (`AppendUpdateForFilter`), `PolicyFilterChain` runs the peer's refs positionally over RPC, and the returned text delta / raw override / teardown is converted back to wire attribute modifications (`textDeltaToModOps`, `buildModifiedPayload`).
4. The (possibly modified) `WireUpdate` is cached (`recentUpdates.Add`) and dispatched to consumers.
5. Target design: steps 2 and 3 collapse into ONE stage-ordered ingress pass. The per-peer policy chain becomes a single ordered step tagged with a declared Stage (Policy) and Priority, merge-sorted into the same list as the in-process filters and executed in that one pass.
6. Egress twin: `ForwardUpdate` applies `r.egressFilters` and the export `PolicyFilterChain` (3 call sites); the same collapse applies per destination peer, preserving `ModAccumulator` semantics.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Wire bytes ↔ in-process filter | `IngressFilterFunc(source, payload, meta)` over `wireUpdate.Payload()` (zero-copy) | [ ] |
| Wire bytes ↔ text ↔ external plugin | `AppendUpdateForFilter` to text, `CallFilterUpdate` RPC, `textDeltaToModOps`/`buildModifiedPayload` back to wire | [ ] |
| Reactor ↔ filterapi registry | `filterapi.IngressFilters()`/`EgressFilters()` read once at construction (`reactor.go:1189-1190`) | [ ] |
| Config ↔ per-peer chain | `peers.go` builds/canonicalizes `ImportFilters`/`ExportFilters` into `peer.settings` | [ ] |

### Integration Points
- `filterapi.sortedFilters` (Stage/Priority/name ordering) - the existing ordering primitive the unified pipeline reuses and extends to include the policy-chain step.
- `PolicyFilterChain` / `policyFilterFunc` - becomes the body of the single ordered policy-chain step (unchanged internally).
- `PeerFilterInfo` (`filterapi.go:31`) - the per-peer context value the reactor already passes to ingress filters; candidate carrier for the per-peer filter refs the policy-chain step needs.
- `safeIngressFilter` - panic-recovering wrapper the unified pass keeps for every in-process step.

### Architectural Verification
- [ ] No bypassed layers (received UPDATE still flows session → notify → ordered pipeline → cache → dispatch).
- [ ] No unintended coupling (`filterapi` stays a stdlib-only leaf; reactor-instance state is not pushed into it).
- [ ] No duplicated functionality (two ordering mechanisms collapse to one; executors kept, not recreated).
- [ ] Zero-copy preserved where applicable (in-process steps still operate on `wireUpdate.Payload()`; text serialization stays gated to peers with policy filters).
- [ ] Registration over hardcoding - the policy-chain step joins the ingress/egress chain through a declared Stage/Priority merge-sort, not a new hardcoded code-position block in `notifyMessageReceiver`; in-process filters continue to register via `filterapi.Register` and the reactor discovers them (small-core/registration; `ai/rules/plugin-self-containment.md`).

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Current runtime order is System 1 (stage-ordered) fully, THEN System 2 (positional), for every received UPDATE | `reactor_notify.go:393-443` precedes `:458-531` in one function | The characterization baseline is wrong; ported order would differ | Characterization test capturing execution order across loop/community/policy/OTC before refactor | unvalidated |
| A-2 | `filter_community`'s in-process filterapi filter and its RPC policy filter are not double-applied in a way the merge would change | `filter_community/register.go` registers both; community `.ci` tests currently pass | Unification must dedup the two community paths, larger scope | `test/plugin/community-strip.ci`, `test/plugin/community-cumulative.ci` pass unchanged | unvalidated |
| A-3 | `PeerFilterInfo` can carry the per-peer `ImportFilters`/`ExportFilters` (or the step closure can read peer settings) without breaking existing filter plugins | `PeerFilterInfo` is a value type in `filterapi.go:31` passed by the reactor; adding fields is backward compatible | Need a different context seam for the policy-chain step | Compile + existing filter plugin unit tests + `filterapi_test.go` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Reordering changes outcomes: the policy-chain step's assigned Stage/Priority does not reproduce the current System-1-then-System-2 order (e.g. community add now lands after OTC stamp) | `.ci` diffs in `community-*`, `role-otc-*` tests | Assign the policy-chain step FilterStagePolicy with a Priority strictly greater than the in-process Policy-stage filters so the current order is reproduced exactly; lock order with a characterization test before touching production code |
| R-2 | `reactor.api` access from a globally-registered filterapi entry: `filterapi.Register` is init-only and stateless, but the policy-chain step needs the reactor instance (`r.api`) | nil `api` at runtime; import cycle if reactor state pushed into filterapi | Reactor BINDS the policy-chain step at construction (not plugin init) and merge-sorts it into the ordered list using a `filterapi` ordering helper; `filterapi` gains ordering support, never reactor state |
| R-3 | Egress parity missed: 3 export `PolicyFilterChain` call sites (`reactor_api_forward.go:498`, `peer_initial_sync.go:734`, `egress_inject_filter.go:46`) plus `EgressFilters` | grep shows more than one export ordering after refactor | Enumerate all export call sites (done here) and route each through the unified egress pass; a grep gate asserts a single ordering site |
| R-4 | Full text-serialization elimination over-promised: external RPC filters inherently need the text protocol | Attempt to remove `AppendUpdateForFilter` breaks external plugins | Scope this spec to ordering unification; full in-process migration of specific policy filters (community/prefix/aspath on wire bytes) is a phased follow-on, recorded in Known Limitations |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Received UPDATE with a loop-detection filter (Protocol stage) | → | unified ordered ingress pipeline runs loop first | `test/plugin/loop-cluster-list.ci` |
| Received UPDATE, community strip via configured policy filter | → | policy-chain step inside the unified chain | `test/plugin/community-strip.ci` |
| Received UPDATE, `filter_family` teardown | → | policy-chain step teardown honored, route dropped | `test/plugin/filter-family-import-teardown.ci` |
| Received UPDATE, AS-path reject via configured policy filter | → | policy-chain step reject short-circuits | `test/plugin/aspath-filter-reject.ci` |
| Forwarded UPDATE, OTC egress stamp (Annotation stage) | → | unified ordered egress pass runs OTC stamp | `test/plugin/role-otc-egress-stamp.ci` |
| Received UPDATE, plain-name policy chain ordering | → | canonicalized refs run in declared order | `test/plugin/policy-chain-plain-names.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A received UPDATE with in-process filters (loop, community, OTC) and configured RPC policy filters | Every filter runs in ONE ordered pass; relative order is determined by declared Stage, then Priority, then name, never by code position |
| AC-2 | Received UPDATE where loop (Protocol), community (Policy), policy-chain (Policy), OTC (Annotation) all apply | Observable order matches the pre-refactor order exactly: Protocol < Policy in-process < policy-chain step < Annotation |
| AC-3 | Configured policy filter returns reject / text-delta modify / raw override / teardown / errors | All System 2 semantics preserved: reject short-circuit, delta modify, terminal raw override, import-only teardown, per-filter on-error, 5s timeout, declared-attr validation |
| AC-4 | Received UPDATE from a peer with no configured policy filters | No text serialization occurs (hot path pays zero `AppendUpdateForFilter` cost); gating unchanged |
| AC-5 | UPDATE forwarded to multiple destination peers with egress filters and export policy filters | ForwardUpdate applies egress filterapi filters and export policy chain in one declared-order pass per destination; per-peer `ModAccumulator` semantics preserved |
| AC-6 | Source audit after refactor | The second back-to-back ingress block in `reactor_notify.go` is gone; exactly one ordered ingress invocation site and one ordered egress invocation site remain |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestUnifiedIngressOrder` | `internal/component/bgp/reactor/unified_filter_order_test.go` | The merged ingress pipeline executes in-process filters and the policy-chain step in declared Stage/Priority/name order (AC-1, AC-2) | new |
| `TestUnifiedIngressReproducesLegacyOrder` | `internal/component/bgp/reactor/unified_filter_order_test.go` | The policy-chain step's assigned Priority reproduces the exact pre-refactor order (R-1 characterization) | new |
| `TestFilterOrderingByStagePriorityName` | `internal/component/bgp/filterapi/filterapi_test.go` | Existing Stage/Priority/name sort still holds | existing |
| `TestFilterDeltaParseCallCount` | `internal/component/bgp/reactor/filter_delta_test.go` | Each filter text still parsed exactly once on the modify path | existing |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `community-strip` | `test/plugin/community-strip.ci` | Configured community filter still strips on received UPDATE | must pass unchanged |
| `role-otc-egress-stamp` | `test/plugin/role-otc-egress-stamp.ci` | OTC still stamped at Annotation stage on egress | must pass unchanged |
| `loop-cluster-list` | `test/plugin/loop-cluster-list.ci` | Loop detection still runs first (Protocol stage) | must pass unchanged |
| `aspath-filter-reject` | `test/plugin/aspath-filter-reject.ci` | AS-path policy reject still drops the route | must pass unchanged |
| `filter-family-import-teardown` | `test/plugin/filter-family-import-teardown.ci` | Import teardown still triggers NOTIFICATION + close | must pass unchanged |
| `policy-chain-plain-names` | `test/plugin/policy-chain-plain-names.ci` | Plain-name chain still resolves and orders correctly | must pass unchanged |
| `policy-test-remove-private-as` | `test/plugin/policy-test-remove-private-as.ci` | remove-private-AS modify still applied | must pass unchanged |

This is an internal refactor with no user-facing behavior change; the existing `.ci` test suite must pass with no regressions.

## Files to Modify
- `internal/component/bgp/reactor/reactor_notify.go` - collapse the two back-to-back ingress blocks (`:393-443`, `:458-531`) into one stage-ordered pass; delete the second block.
- `internal/component/bgp/reactor/reactor.go` - at construction (`:1189-1190`) build a unified, stage-ordered ingress/egress pipeline that includes the registered filterapi funcs AND the reactor-bound policy-chain step tagged with declared Stage/Priority.
- `internal/component/bgp/filterapi/filterapi.go` - add an ordering helper (or an ordered-step type) so the reactor can merge-sort a reactor-bound step into the same Stage/Priority order without pushing reactor state into the leaf package.
- `internal/component/bgp/reactor/filter_chain.go` - `PolicyFilterChain` becomes the body of the single ordered policy-chain step; no change to its RPC/teardown/raw semantics.
- `internal/component/bgp/reactor/reactor_api_forward.go` - route the export `PolicyFilterChain` (`:498`) through the unified egress ordered pass.
- `internal/component/bgp/reactor/peer_initial_sync.go` - route its export `PolicyFilterChain` (`:734`) through the same unified egress pass.
- `internal/component/bgp/reactor/egress_inject_filter.go` - route its export `PolicyFilterChain` (`:46`) through the same unified egress pass.
- `internal/component/bgp/reactor/peersettings.go` - if needed, thread `ImportFilters`/`ExportFilters` into the policy-chain step context (or extend `PeerFilterInfo`).

## Implementation Steps

### Implementation Phases

1. **Phase: Wiring / characterization (MANDATORY FIRST)** - lock the current cross-system order before touching production code.
   - Tests: `TestUnifiedIngressReproducesLegacyOrder` (asserts the exact loop → community → policy-chain → OTC order the current two-block code produces); run the existing `.ci` suite as the behavioral baseline.
   - Files: `internal/component/bgp/reactor/unified_filter_order_test.go` (new, order-recording harness), no production change yet.
   - Verify: test passes against current two-block code (captures baseline); it will guard the refactor.
2. **Phase: Ordered-step seam in filterapi** - add an ordering helper / ordered-step type so a reactor-bound step can be merge-sorted by Stage/Priority/name alongside registered filters.
   - Tests: `filterapi_test.go` extended to cover a mixed list including a non-registered ordered step.
   - Files: `internal/component/bgp/filterapi/filterapi.go`.
   - Verify: helper sorts correctly; leaf package stays stdlib-only (no reactor import).
3. **Phase: Unified ingress pipeline** - build the merged ordered ingress list at reactor construction; execute it in one pass in `notifyMessageReceiver`; delete the second block.
   - Tests: `TestUnifiedIngressOrder`; all ingress `.ci` tests (`community-strip`, `loop-cluster-list`, `aspath-filter-reject`, `filter-family-import-teardown`, `policy-chain-plain-names`).
   - Files: `reactor.go`, `reactor_notify.go`, `filter_chain.go`, `peersettings.go`.
   - Verify: characterization test still green (order preserved); grep shows a single ingress invocation site (AC-6).
4. **Phase: Unified egress pipeline** - route all 3 export `PolicyFilterChain` call sites and `EgressFilters` through one ordered egress pass.
   - Tests: `role-otc-egress-stamp.ci`, export-side policy `.ci` tests.
   - Files: `reactor_api_forward.go`, `peer_initial_sync.go`, `egress_inject_filter.go`.
   - Verify: `ModAccumulator` per-peer semantics preserved; grep shows a single egress ordering site (AC-6).
5. **Full verification** → `make ze-test` (lint + all ze tests).
6. **Complete spec** → fill audit tables, write learned summary, two commits per planning.md.

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Merge order reproduces the pre-refactor loop → community → policy-chain → OTC order exactly (R-1); reject/modify/raw/teardown/on-error/timeout/validation all preserved (AC-3) |
| Data flow | In-process steps still operate on `wireUpdate.Payload()` zero-copy; text serialization stays gated to peers with configured policy filters (AC-4) |
| Egress parity | All 3 export `PolicyFilterChain` call sites plus `EgressFilters` route through one ordered pass (AC-5, AC-6, R-3) |
| Registration over hardcoding | The policy-chain step joins via declared Stage/Priority merge-sort, not a new hardcoded block in `notifyMessageReceiver`; in-process filters still register via `filterapi.Register` and the reactor discovers them (`ai/rules/plugin-self-containment.md`) |
| Rule: no-layering | The second back-to-back ingress block is fully deleted; no dead ordering path remains |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Winner: `filterapi` (in-process, wire-bytes, Stage/Priority ordering) | Make `filterapi` a special case of `PolicyFilterChain` (text/RPC) | `filterapi` matches Ze's zero-copy wire-bytes identity and already has the ONE thing the bug needs: an explicit, inspectable ordering (Stage, Priority, name). `PolicyFilterChain` has only positional order and pays text serialization on the hot path. |
| Keep BOTH executors; unify only the ORDERING | Delete `PolicyFilterChain` and reimplement all policy/RPC filters in-process | The two systems are the same LAYER (ingress route filtering) but complementary EXECUTORS (in-process wire vs out-of-process text/RPC). The redundant, buggy part is the ordering mechanism, not the executors. The external plugin SDK depends on the text/RPC protocol; deleting it is a much larger, separate effort. So: express the per-peer policy chain as ONE ordered step (declared Stage Policy + Priority) merge-sorted into the same chain as the in-process filters. |
| Policy-chain step assigned FilterStagePolicy + Priority above in-process Policy filters | Give it its own new stage between Policy and Annotation | Reproducing the current System-1-then-System-2 order exactly (R-1) is the safest refactor; a Priority strictly above the in-process Policy-stage filters keeps community-before-policy-chain and both-before-OTC without inventing a new stage. |
| Reactor binds the policy-chain step at construction, `filterapi` gains only an ordering helper | Register the step globally via `filterapi.Register` at init() | The step needs `r.api` (reactor instance) for RPC; `filterapi.Register` is init-only and stateless, and pushing reactor state into the stdlib-only leaf package would break the layering (R-2). |

## Known Limitations
- This spec unifies ORDERING, not execution model. External RPC policy filters still serialize the UPDATE to text (the plugin SDK's text protocol). Text serialization stays gated to peers with configured policy filters, so the no-policy-filter hot path is unaffected, but a peer WITH policy filters still pays it.
- Fully eliminating text serialization for policy filters that could run in-process on wire bytes (community, prefix, aspath) is a phased follow-on per-filter migration, out of scope here. It becomes tractable once all filtering shares one ordered pipeline.
- `filter_community`'s dual registration (in-process filterapi filter plus RPC-capable engine) is preserved as-is; deduplicating it is deferred pending A-2 validation.

## Implementation Summary

## Implementation Audit

## Pre-Commit Verification

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Wiring Test table complete - every row has a concrete `.ci` test name, none deferred
- [ ] Characterization test proves the merged order reproduces the pre-refactor order (R-1)
- [ ] Exactly one ordered ingress invocation site and one ordered egress invocation site remain (AC-6, grep evidence)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/component/bgp/...`)
- [ ] Risks & Assumptions: every A-N confirmed or broken; surviving risks copied forward

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Implementation Audit complete
- [ ] Known Limitations reviewed with user (phased in-process migration scope)
