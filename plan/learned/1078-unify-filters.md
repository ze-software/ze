# 1078 -- Unify Filters

## Context

A received BGP UPDATE was run through two independent filtering systems back-to-back inside `notifyMessageReceiver`: the in-process `filterapi` pipeline (zero-copy wire bytes, explicit Stage/Priority ordering) and the external `PolicyFilterChain` (serialize-to-text, RPC to plugin filters). The relative order of the two systems was fixed only by which code block appeared first in the function, not by any declared property. DESIGN-REVIEW.md finding 2 flagged that implicit ordering as the review's #1 correctness concern. Goal: route all received-UPDATE filtering through ONE stage-ordered pipeline so the cross-system order is declared, preserving every observable filtering outcome (pure refactor).

## Decisions

- Winner is `filterapi` (in-process, wire-bytes, Stage/Priority ordering) over making it a special case of `PolicyFilterChain`: `filterapi` already has the one thing the bug needs -- explicit inspectable ordering -- and matches Ze's zero-copy identity.
- Keep BOTH executors, unify only the ORDERING, over deleting `PolicyFilterChain` and reimplementing all RPC filters in-process: the external plugin SDK depends on the text/RPC protocol; that migration is a separate, larger effort.
- The external per-peer chain is ONE ordered step at a NEW terminal stage `filterapi.FilterStagePeerChain=300` (after Annotation), over the original spec's "Policy stage, before Annotation": the code runs the chain LAST, after OTC, so a terminal stage reproduces the true order. This corrected a wrong assumption baked into the spec.
- Reactor binds the policy-chain step at `startAPIServer` (where `r.api` exists), and `filterapi` gains only the stage constant + ordered accessors + an exported `LessOrder` comparator -- over global `filterapi.Register` of the step, which would push reactor state into the stdlib-only leaf package.
- Egress is order-only, unifying ONLY `forwardUpdateCore` (the one path running both filter kinds), over forcing all export paths through one pass: the four egress paths (forward, RS fast path, injected routes, default-originate) run different filter combinations by design; uniformity would be a behavior change.
- Ingress and egress use SEPARATE step-result types: ingress composes sequentially (`ingressStepResult{accept, modifiedPayload, teardown}`), egress runs deferred/parallel (`egressStepResult{accept, wireOverride}`; in-process filters defer into a shared `ModAccumulator`, the policy chain reads the ORIGINAL payload).

## Consequences

- Cross-system filter order is now a declared, inspectable Stage. A future filter registered at any stage below 300 interleaves correctly; the external chain always runs last. Ordering can be reasoned about without reading `notifyMessageReceiver`.
- `filterapi` stays a stdlib-only leaf: it owns the sort key (`LessOrder`, reused by `sortedFilters`) but never reactor state. Consumers merge-sort reactor-bound steps into the same order via `LessOrder`.
- Text serialization is still paid by peers WITH configured policy filters (the hot-path gate moved into `runIngressPolicyChain`/`runEgressPolicyChain` early-return). Eliminating serialization for policy filters that could run in-process (community/prefix/aspath on wire bytes) is a tractable phased follow-on now that all filtering shares one ordered pipeline.
- The three single-kind egress paths (`egress_inject_filter.go`, `defaultOriginateFilterAccepts`, `forward_rs.go`) are intentionally NOT unified; a future spec that unifies them WILL change behavior (adding/removing a filter kind per path) and must be treated as such, not a refactor.
- `filter_community`'s dual registration (in-process filterapi filter + RPC engine) is preserved; deduplicating it is still deferred.

## Gotchas

- The original spec characterized the order as `loop -> community -> policy-chain -> OTC` and told the implementer to place the policy-chain step at Policy stage. The CODE does the opposite: `r.ingressFilters` runs the WHOLE in-process pass (including OTC at Annotation) in System 1, THEN the policy chain in System 2. Reading the producer (`reactor_notify.go` loops all filters before `:466`), not the spec's prose, caught it. A `TestUnifiedIngressReproducesLegacyOrder` locking `... -> OTC -> policy-chain` guards against re-introducing the wrong order.
- The registered filter set is 5, not the 4 the spec enumerated: `bgp-redistribute` (Policy, ingress) and `bgp-gr` (Annotation, egress LLGR) were missing from the spec's list. Always grep `filterapi.Register` for the complete set.
- Egress is NOT symmetric with ingress: egress in-process filters defer into `ModAccumulator` and the export policy chain reads the ORIGINAL payload (not the mods-adjusted one), so the two are independent contributors combined at the end (`peerBaseWire = exportWireOverride ?? original`, then mods applied). Merging them "like ingress" (sequential payload rewrite) would be wrong.
- `forwardUpdateCore` interleaves other `ModAccumulator` writers around the two filter blocks (RS community strip before; RR CLUSTER_LIST/ORIGINATOR_ID injection, next-hop, send-community after). The unified pass must replace ONLY the two adjacent filter blocks (`reactor_api_forward.go`), leaving those surrounding writers exactly in place, or the emitted bytes change.
- Reactor tests build `&Reactor{egressFilters: ...}` by hand, bypassing `startAPIServer`. Moving forwarding to read `orderedEgressSteps` broke those tests until they also set `orderedEgressSteps` (added a test helper). The field a hand-built reactor must populate changed.
- The filter slices are built in `startAPIServer` (`reactor.go`), not the constructor `New` -- convenient, because `r.api` (needed to bind the policy-chain step) is available there.

## Files

- `internal/component/bgp/filterapi/filterapi.go` -- `FilterStagePeerChain`, `IngressOrdered`/`EgressOrdered`, `LessOrder`; `sortedFilters` reuses `LessOrder`.
- `internal/component/bgp/filterapi/filterapi_test.go` -- `TestPeerChainStageSortsLast`, `TestLessOrderMatchesSort`.
- `internal/component/bgp/reactor/filter_ordered.go` (new) -- ordered step types, `build*` functions, `runIngressPolicyChain`/`runEgressPolicyChain`.
- `internal/component/bgp/reactor/reactor.go` -- removed `ingressFilters` field; added `orderedIngressSteps`/`orderedEgressSteps`, built in `startAPIServer`.
- `internal/component/bgp/reactor/reactor_notify.go` -- unified ingress pass; second block deleted.
- `internal/component/bgp/reactor/reactor_api_forward.go` -- unified egress pass in `forwardUpdateCore`.
- `internal/component/bgp/reactor/unified_filter_order_test.go` (new), `forward_update_test.go` -- order tests + harness update.
- `docs/architecture/core-design.md`, `ai/digests/bgp-reactor.md` -- filter-pipeline docs and digest anchors.
