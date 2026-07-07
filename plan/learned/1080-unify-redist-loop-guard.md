# 1080 -- unify-redist-loop-guard

## Context

The single redistribution invariant "a protocol's routes must never be redistributed
back into itself" was implemented three times with two different identity
representations (DESIGN-REVIEW.md finding 2). Two runtime guards in
`redistribute_egress` compared a numeric `ProtocolID` (`ProtocolIDOf(x); ok && id == b.Protocol`);
one config-evaluator guard in `ImportRule.Accept` compared protocol name strings
(`route.Origin == importingProtocol`). The goal was to collapse the three inline
comparisons into one documented predicate while keeping all three call sites (they
guard genuinely different execution edges with different side effects).

## Decisions

- Added one shared predicate `redistevents.WouldLoop(source, dest string) bool` (pure `source == dest`), kept all three call sites -- chose this over merging/deleting the runtime guards because each guards a distinct edge with a distinct side effect: config evaluator `return false`, egress fan-out per-consumer `continue` + `ze_bgp_redistribute_filtered_protocol_total`, late-join replay whole-batch early `return`. The duplication was the COMPARISON, not the call sites.
- Name-keyed, NOT ID-keyed -- chose string names over `WouldLoop(a, b ProtocolID)` because Guard 1 must reject `origin==importing` even for config-only source names that are never registered as `redistevents` producers; an ID-keyed predicate would fail-open (`ProtocolIDOf` returns `!ok`) and regress that case.
- Pure `==`, no empty-string special case -- chose exact equality over `source != "" && source == dest` to preserve Guard 1's prior outcome bit-for-bit (including empty/empty → reject). The empty case is unreachable at the runtime guards because `b.Protocol` is validated non-empty first.
- Predicate lives in `redistevents/registry.go` beside `ProtocolName`/`ProtocolIDOf` -- chose the existing identity API over a new `redistcore` package; `component/config/redistribute` → `core/redistevents` is a legal downward import, verified acyclic.

## Consequences

- `WouldLoop` is now the single definition of the loop invariant; future guard sites call it rather than re-inlining a comparison. Its doc comment warns against "optimizing" it to compare `ProtocolID`s (that regresses the config-only-name case).
- The name↔ID reconciliation is load-bearing: the `redistevents` registry is a bijection (`RegisterProtocol` allocates a fresh ID per name; `byName` is injective), so for any registered protocol `ProtocolName(b.Protocol) == cname` is exactly equivalent to the old `ProtocolIDOf(cname).ok && id == b.Protocol`. Both runtime guards already resolve `name := ProtocolName(b.Protocol)` before the guard, so name-keying reproduces the ID comparison with no new lookup.
- Out of scope, left redundant on purpose: the runtime guards remain redundant-in-outcome with the evaluator's Guard 1 (which runs again via `ev.Accept`). They are intentional early-exits (short-circuit + distinct metric). Collapsing that redundancy is a separate design question. `consumerProtocolIDs()`/`skipIDs` (a debug-log membership set) is a different name→ID builder, not a loop guard, left unchanged.

## Gotchas

- Migrating Guards 2/3 from the ID comparison to name-keyed `WouldLoop(name, ...)` incidentally REMOVED the per-iteration `ProtocolIDOf` lookups the old code performed inside the consumer fan-out loop (the source `name` is already in hand). Behavior is identical; the loop does slightly less work. Not a behavior change, but worth knowing the old lookups are gone.
- Equivalence proof for the reviewer: old guard fires ⟺ `cname` registered AND `ProtocolIDOf(cname)==b.Protocol`; by the registry bijection that is exactly `ProtocolName(b.Protocol)==cname`, i.e. `name==cname`. So the name-keyed form is provably identical for registered protocols and additionally correct for unregistered config-only names (Guard 1). If someone later reports the guard "changed behavior," it did not -- re-read this equivalence before assuming a regression.
- `ze-verify-wiring-docs` emits a non-blocking ADVISORY ("user-facing code changed without a functional-test change" for `route.go`) because the functional-test gate keys on the file path, not on whether behavior changed. Expected for a behavior-preserving refactor; the existing `redist-*` interop scenarios provide the coverage. Do not add a redundant `.ci` to silence an advisory.
- `ze-verify-changed` was RED only from a pre-existing, documented, non-deterministic `-race` flake in `internal/component/l2tp` (`TestPeerTeardownWithdrawsSubscriberRoute`, `plan/known-failures.md`) that this change never touched (reproduced flaky in isolation: run1 FAIL / run2 PASS). All 5 structural gates + functional + exabgp were green, and every `redist-*` functional scenario passed. Committed scoped-to-changed with owner direction.
- `ze-validate` (`scripts/dev/validate.py`) is DIFF-SCOPED (`git diff --name-only HEAD`): editing a file re-checks ALL its exported symbols, so a behavior-preserving refactor can surface a pre-existing latent finding in a file it merely touches. Here it correctly flagged `Evaluate` (an exported free function used only same-package — genuinely should be unexported). But unexporting it (`evaluate`) forced an edit to `evaluator.go`, which pulled the `Evaluator` TYPE into scope and produced a FALSE positive: the checker's wiring grep is by bare name, and `Evaluator` is reached only through `NewEvaluator()`/`Global()` (idiomatic callers write `ev := configredist.Global()`, never spelling the type). Root fix was in the CHECK, not the code: added `_type_returned_by_wired_func` (a type is wired if an exported same-package func returns it AND that func has a cross-package caller), a sibling to the existing constants and struct-field exceptions. Lesson: when a diff-scoped gate flags a symbol your diff didn't author, first ask "did I author this, or did I just bring the file into scope?" and "is the finding real, or a checker blind spot?" — then fix the real layer (unexport the truly-internal symbol; teach the checker the legitimate seam it doesn't model).

## Files

- `internal/core/redistevents/registry.go` -- added `WouldLoop(source, dest string) bool`.
- `internal/core/redistevents/registry_test.go` -- new; `TestWouldLoop`, `TestWouldLoopNoAlloc`.
- `internal/component/config/redistribute/route.go` -- Guard 1 via `WouldLoop`; added redistevents import; unexported `Evaluate`→`evaluate`.
- `internal/component/config/redistribute/evaluator.go`, `route_test.go`, `evaluator_test.go` -- `evaluate` rename fan-out (same-package).
- `internal/component/bgp/plugins/redistribute_egress/redistribute.go` -- Guard 2 via `WouldLoop`.
- `internal/component/bgp/plugins/redistribute_egress/replay.go` -- Guard 3 via `WouldLoop`.
- `scripts/dev/validate.py` + `scripts/dev/validate_test.py` -- constructor-seam wiring exception + 2 regression tests.
