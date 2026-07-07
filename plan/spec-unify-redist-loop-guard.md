# Spec: unify-redist-loop-guard

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
3. `docs/architecture/core-design.md` - redistribution loop-prevention model
4. `internal/core/redistevents/registry.go` - the name<->ID registry (bijection)
5. The three guard sites: `internal/component/config/redistribute/route.go:46`, `internal/component/bgp/plugins/redistribute_egress/redistribute.go:215`, `internal/component/bgp/plugins/redistribute_egress/replay.go:215`

## Task

DESIGN-REVIEW.md finding 2, row "Redistribution loop prevention": the single
invariant "a protocol's routes must not be redistributed back into itself" is
implemented three times, with two different identity representations. Two runtime
guards compare a numeric `ProtocolID`; one config-evaluator guard compares protocol
name strings. This spec inventories the three guards, confirms they express one
invariant, and unifies the comparison behind ONE shared predicate
(`redistevents.WouldLoop(source, dest string)`) called from all three sites. It is a
small, low-risk, behavior-preserving refactor: the three call sites stay (they guard
genuinely different execution edges with different side effects), but the three
independent inline comparisons collapse to one documented definition of the invariant.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as → Decision: / → Constraint: annotations — these survive compaction. -->
- [ ] `docs/architecture/core-design.md` - redistribution route types and loop prevention (the `// Design:` anchor on `route.go` and `registry.go`)
  → Decision: loop prevention is defined as "a route is never redistributed back into its origin protocol"; `RedistRoute.Origin` is set once and never modified.
- [ ] `internal/core/redistevents/registry.go` - the ProtocolID registry that both runtime guards use
  → Constraint: the registry is a bijection (`byName` maps name->ID injectively, `entries[id].name` maps back); `ProtocolIDOf` bridges name->ID and `ProtocolName` bridges ID->name, so name-equality and ID-equality are interchangeable for any registered protocol.
- [ ] `ai/rules/plugin-self-containment.md` - no plugin spelling in generic packages
  → Constraint: the shared predicate must be protocol-agnostic (takes two opaque names); it must not hardcode "bgp"/"ospf"/etc. The "bgp" constant stays local to the replay call site.

**Key insights:**
- All three guards test the same proposition: source protocol == destination protocol.
- The runtime guards already compute the source's canonical name (`name := redistevents.ProtocolName(b.Protocol)`) before the guard, so a name-keyed predicate fits all three sites with no new lookup.
- A name-keyed predicate (not ID-keyed) is required so the config evaluator keeps working for source names that are not registered `redistevents` producers.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
<!-- Same rule: never tick [ ] to [x]. Write → Constraint: annotations instead. -->
- [ ] `internal/component/config/redistribute/route.go` - Guard 1 at :46-48. `ImportRule.Accept(route, importingProtocol)` first rejects with `if route.Origin == importingProtocol { return false }`. Pure string equality; no registry dependency; works even for names never registered as `redistevents` producers.
  → Constraint: this is the authoritative evaluator check; it must reject origin==importing regardless of registry membership.
- [ ] `internal/component/bgp/plugins/redistribute_egress/redistribute.go` - Guard 2 at :215-220 inside `handleBatch`. After `name := redistevents.ProtocolName(b.Protocol)` (:181), the consumer fan-out loop does `if id, ok := redistevents.ProtocolIDOf(cname); ok && id == b.Protocol { m.filteredProtocolTotal.Inc(); continue }`. Numeric ID comparison; skips only that one consumer and increments `ze_bgp_redistribute_filtered_protocol_total`.
  → Constraint: the distinct metric bucket (`filteredProtocolTotal` vs `filteredRuleTotal`) and the per-consumer `continue` (not a whole-batch drop) must be preserved.
- [ ] `internal/component/bgp/plugins/redistribute_egress/replay.go` - Guard 3 at :214-217 inside `handleReplayBatch`. After `name := redistevents.ProtocolName(b.Protocol)` (:203), it does `if id, ok := redistevents.ProtocolIDOf(bgpDestination); ok && id == b.Protocol { return }`. Numeric ID comparison; drops the WHOLE replay batch (single-destination path targets only `bgpDestination == "bgp"`).
  → Constraint: early `return` (whole-batch drop) semantics and the local `bgpDestination` constant must be preserved.
- [ ] `internal/core/redistevents/registry.go` - `ProtocolIDOf(name) (ProtocolID, bool)` (:119), `ProtocolName(id) string` (:106), `RegisterProtocol` (:46). Confirms the name<->ID bijection that makes name-equality equivalent to ID-equality for registered protocols.
  → Constraint: `ProtocolUnspecified == 0`; `b.Protocol` is validated non-zero/registered before each runtime guard (redistribute.go:182-185, replay.go:210-213).

**Behavior to preserve:**
- Guard 1: `ImportRule.Accept` rejects when origin name equals importing protocol name, for any name (registered or not). Exact `==` outcome, including the empty/empty case (both empty stays a reject).
- Guard 2: bgp-sourced batch skips only the bgp consumer, still fans out to non-bgp consumers, and increments `ze_bgp_redistribute_filtered_protocol_total` (not `filteredRuleTotal`).
- Guard 3: bgp-sourced replay batch is dropped whole via early `return`; no injection into bgp on peer-up.
- All existing redistribution unit tests and interop scenarios pass unchanged.

**Behavior to change:**
- None - internal refactor, behavior preserved. Only the comparison MECHANISM is unified; every observable outcome (accept/reject, skip vs drop, metric bucket) is identical.

**Feature inventory (guard x attribute):**

| Attribute | Guard 1 (evaluator) | Guard 2 (egress fan-out) | Guard 3 (late-join replay) |
|-----------|---------------------|--------------------------|----------------------------|
| Location | `config/redistribute/route.go:46-48` | `redistribute_egress/redistribute.go:215-220` | `redistribute_egress/replay.go:214-217` |
| Layer | config-defined evaluator (`ImportRule.Accept`) | runtime egress orchestrator (`handleBatch` consumer loop) | runtime late-join replay (`handleReplayBatch`) |
| Identity compared | origin name (string) vs importing protocol name (string) | source `ProtocolID` (`b.Protocol`) vs consumer name->ID (`ProtocolIDOf(cname)`) | source `ProtocolID` (`b.Protocol`) vs "bgp" name->ID (`ProtocolIDOf(bgpDestination)`) |
| Control-flow effect | `return false` (reject this route for this rule) | `continue` (skip THIS consumer, keep fanning to others) | `return` (drop the WHOLE replay batch) |
| Side effect | none | increments `ze_bgp_redistribute_filtered_protocol_total` | none |
| Registry dependency | none (pure `==`, works for unregistered names) | `ProtocolIDOf` (name->ID) | `ProtocolIDOf` (name->ID) |
| Failure if removed | evaluator accepts origin==importing: a protocol re-imports its own routes (config-time loop) | source batch re-dispatched to the consumer of the same protocol: immediate runtime loop plus wrong metric attribution | bgp-sourced replay re-injected into bgp on peer-up: loop on every late join |

**Are they identical?** They enforce ONE invariant (source protocol == destination protocol
implies do-not-deliver). They guard DIFFERENT edges: the evaluator's accept decision, the
multi-consumer fan-out (per-consumer skip with a distinct metric), and the single-destination
replay (whole-batch early return). The runtime guards are even redundant-in-outcome with the
evaluator (Guard 1 runs again via `ev.Accept` right after Guard 2/Guard 3), but exist as
early-exits that (a) short-circuit before the evaluator and (b) attribute the filter to the
protocol-skip metric bucket. The duplication is the COMPARISON, not the call sites.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- A producer protocol (bgp, ospf, isis, connected, ...) publishes a `redistevents.RouteChangeBatch` on the event bus, tagged with its numeric source `Protocol` (a `ProtocolID`).
- In parallel, config load builds `ImportRule` entries from the YANG `destination <proto> { import { source ... } }` blocks; these rules are evaluated per route at event time.

### Transformation Path
1. Egress: `redistribute_egress.handleBatch` receives the batch, resolves `name := redistevents.ProtocolName(b.Protocol)`, and iterates registered consumers. Guard 2 (self-loop, per-consumer) fires here before `ev.Accept`.
2. Replay: on peer-up, `redistribute_egress.handleReplayBatch` resolves the same `name` and, for the single `bgpDestination` target, applies Guard 3 (self-loop, whole-batch) before `ev.Accept`.
3. Evaluator: `configredist.Global().Accept(route, dest)` runs `ImportRule.Accept`, whose first check is Guard 1 (origin name vs importing protocol name).
4. Unification: Guards 1/2/3 all call `redistevents.WouldLoop(source, dest string)` (returns `source == dest`). Names are the universal identity; the runtime guards pass the already-resolved `name`, the evaluator passes `route.Origin`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Producer plugin -> egress orchestrator | `RouteChangeBatch` event on the bus, carrying numeric `ProtocolID` | [ ] |
| Egress orchestrator -> config evaluator | `configredist.Evaluator.Accept(route, destName)` (value types, names) | [ ] |
| Runtime guards -> redistevents registry | `ProtocolName(b.Protocol)` (ID->name) already called before each guard | [ ] |
| Config evaluator -> redistevents (new) | new import of `internal/core/redistevents` for `WouldLoop`; component->core direction, no cycle (registry.go does not import config) | [ ] |

### Integration Points
- `redistevents.WouldLoop` (new, in `registry.go`) - the single definition of the loop invariant, sitting beside `ProtocolName`/`ProtocolIDOf`.
- `ImportRule.Accept` (`route.go`) - calls `WouldLoop(route.Origin, importingProtocol)`.
- `handleBatch` (`redistribute.go`) - calls `WouldLoop(name, cname)`, keeps metric + `continue`.
- `handleReplayBatch` (`replay.go`) - calls `WouldLoop(name, bgpDestination)`, keeps early `return`.

### Architectural Verification
- [ ] No bypassed layers (guards stay at their existing sites; only the comparison is factored out).
- [ ] No unintended coupling (`config/redistribute` gains one import of `internal/core/redistevents`, a lower tier; no cycle).
- [ ] No duplicated functionality (three inline comparisons become one shared predicate).
- [ ] Zero-copy preserved where applicable (predicate takes two strings by value; no allocation).
- [ ] Registration over hardcoding — `WouldLoop` is protocol-agnostic (two opaque names); it hardcodes no protocol identity, and the "bgp" spelling stays local to the replay call site (`bgpDestination`). No new per-feature field/switch/case/factory is added to a core or shared struct; producers still register protocols via `RegisterProtocol` and the core discovers them (small-core/registration; `ai/rules/plugin-self-containment.md`).

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The redistevents registry is a bijection: name==dest iff `ProtocolIDOf(name).id == ProtocolIDOf(dest).id`, so name-equality is exactly equivalent to the current ID-equality at Guards 2/3 | `registry.go:46-66,119-124` (`RegisterProtocol` allocates a fresh ID per new name; `byName` injective) | if two names shared an ID, name-equality could differ from ID-equality | grep of `RegisterProtocol`/`byName`; unit test `TestWouldLoop` plus existing `TestHandleBatchConsumerSourceSkipped` still passing | unvalidated |
| A-2 | Both runtime guards run only after `b.Protocol` is validated as a registered, non-zero ID, so `ProtocolName(b.Protocol)` yields the canonical source name | `redistribute.go:182-185`, `replay.go:210-213` | a `name==""` could slip through and mis-fire the predicate | read the guard-preceding validation; `name` non-empty check already present | unvalidated |
| A-3 | Adding `internal/core/redistevents` as an import of `internal/component/config/redistribute` introduces no import cycle | grep: registry.go imports only `slices`,`sync`; redistevents does not import config/redistribute | build fails with import cycle | `make ze-lint` / `go build`; grep already shows no reverse import | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A name-keyed predicate changes the empty-string edge (`""=="" `) versus the current ID-keyed `ok && ...` at Guards 2/3 | a redistribution unit test flips | define `WouldLoop` as pure `source == dest` (matches Guard 1 exactly); the empty case is unreachable because `b.Protocol` is validated non-empty first (A-2) |
| R-2 | Someone later "optimizes" `WouldLoop` to compare IDs and regresses Guard 1 for unregistered source names | Guard 1 test for a config-only source loop fails | document in the predicate comment that it is name-keyed BY DESIGN so it works for names outside the producer registry |

## Wiring Test (MANDATORY — NOT deferrable)

This is an internal refactor: the wiring is proven by the existing test suite. Each row
names the existing test that exercises the migrated guard end to end (no new user-facing
behavior, so no new `.ci` is required).

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Producer publishes a bgp-sourced `RouteChangeBatch` on the bus | → | `handleBatch` consumer loop applies `WouldLoop(name, cname)`, skips bgp consumer, bumps `filteredProtocolTotal` | `TestHandleBatchConsumerSourceSkipped` |
| Peer-up triggers replay of a bgp-sourced batch | → | `handleReplayBatch` applies `WouldLoop(name, bgpDestination)`, early return | `TestOrchestratorTargetsNewPeerOnly` |
| Config evaluator sees origin == importing protocol | → | `ImportRule.Accept` applies `WouldLoop(route.Origin, importingProtocol)`, rejects | `TestAcceptLoopPrevention` |
| FRR redistributes routes to ze and back across a late join | → | full egress + replay path with the unified guards | `test/interop/scenarios/redist-late-join-dynamic-frr` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ImportRule.Accept` with `route.Origin == importingProtocol` | returns false (reject), unchanged from before, now via `WouldLoop` |
| AC-2 | bgp-sourced batch, bgp consumer registered alongside non-bgp consumers | bgp consumer skipped, non-bgp consumers still dispatched, `ze_bgp_redistribute_filtered_protocol_total` incremented once |
| AC-3 | bgp-sourced replay batch on peer-up | `handleReplayBatch` returns early, no route injected into bgp |
| AC-4 | `redistevents.WouldLoop(source, dest)` | returns true iff `source == dest`; false when they differ (including one empty); no allocation |
| AC-5 | Full redistribution unit suite + `redist-*` interop scenarios | all pass unchanged (behavior preserved) |

## End-to-End User Stories (MANDATORY for new features)

Not a user-facing feature. This is an internal refactor with no new operator-visible
behavior; the redistribution flows below are exercised by the existing test suite.

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures `destination bgp { import { source ospf } }` and expects bgp not to re-import bgp | evaluator `ImportRule.Accept` -> `WouldLoop` | `TestAcceptLoopPrevention` |
| 2 | Runs FRR<->ze redistribution with a peer joining late | egress + replay -> unified guards | `test/interop/scenarios/redist-late-join-dynamic-frr` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestWouldLoop` | `internal/core/redistevents/registry_test.go` | `WouldLoop(a,b)` returns true iff `a==b`; false for differing and one-empty inputs | new |
| `TestAcceptLoopPrevention` | `internal/component/config/redistribute/route_test.go` | evaluator rejects origin==importing (now via `WouldLoop`) | existing, must still pass |
| `TestAcceptLoopPreventionBGPSubSources` | `internal/component/config/redistribute/route_test.go` | ibgp/ebgp share origin "bgp" and stay blocked | existing, must still pass |
| `TestHandleBatchConsumerSourceSkipped` | `internal/component/bgp/plugins/redistribute_egress/redistribute_test.go` | bgp-source batch skips bgp consumer, metric bumped | existing, must still pass |
| `TestOrchestratorTargetsNewPeerOnly` | `internal/component/bgp/plugins/redistribute_egress/replay_test.go` | replay path does not loop bgp back | existing, must still pass |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `source`/`dest` name | opaque strings | equal non-empty -> loop; differing -> no loop | empty vs empty -> matches current `==` (reject) | N/A (no numeric range; predicate is string equality) |

### Functional Tests
This is an internal refactor with no user-facing behavior change; the existing test suite
passes with no regressions. No new `.ci` file is required. The existing interop scenarios
below cover the full redistribution path and must remain green.

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `redist-late-join-dynamic-frr` | `test/interop/scenarios/redist-late-join-dynamic-frr/` | late-joining peer receives redistributed routes without loop | existing, must still pass |
| `isis-redist-frr` | `test/interop/scenarios/isis-redist-frr/` | isis routes redistributed to bgp, not looped back | existing, must still pass |
| `ospf-v6-redist-frr` | `test/interop/scenarios/ospf-v6-redist-frr/` | ospfv3 routes redistributed to bgp | existing, must still pass |

### Interop Tests (MANDATORY for protocol features)
No wire protocol behavior changes. This refactor does not touch any BGP/IPsec/L2TP wire
encoding or negotiation; it only factors an in-process comparison. The redistribution
interop scenarios above are re-run as regression coverage, not new interop work.

### Future (if deferring any tests)
- None deferred.

## Files to Modify
- `internal/core/redistevents/registry.go` - add `WouldLoop(source, dest string) bool` (the single definition of the loop invariant) beside `ProtocolName`/`ProtocolIDOf`.
- `internal/component/config/redistribute/route.go` - Guard 1: replace `route.Origin == importingProtocol` (:46) with `redistevents.WouldLoop(route.Origin, importingProtocol)`; add the `internal/core/redistevents` import.
- `internal/component/bgp/plugins/redistribute_egress/redistribute.go` - Guard 2: replace the `ProtocolIDOf(cname); ok && id == b.Protocol` comparison (:215) with `redistevents.WouldLoop(name, cname)`; keep the metric increment and `continue`.
- `internal/component/bgp/plugins/redistribute_egress/replay.go` - Guard 3: replace the `ProtocolIDOf(bgpDestination); ok && id == b.Protocol` comparison (:215) with `redistevents.WouldLoop(name, bgpDestination)`; keep the early `return`.
- `internal/core/redistevents/registry_test.go` - add `TestWouldLoop`.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | [ ] No | no config surface change |
| CLI commands/flags | [ ] No | no CLI change |
| Prometheus counters/metrics | [ ] No | `ze_bgp_redistribute_filtered_protocol_total` unchanged (Guard 2 keeps incrementing it) |
| Doctor check for runtime dependencies | [ ] No | no new runtime dependency |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 12 | Internal architecture changed? | [ ] Maybe | `docs/architecture/core-design.md` - if it enumerates the loop-prevention sites, note they now share `WouldLoop` |
| 16 | Any changed source file referenced by existing doc source anchors? | [ ] Check | grep `docs/` for `route.go`, `registry.go`, `redistribute.go`, `replay.go` anchors |

## Files to Create
- None. All changes are edits to existing files (new test function lands in the existing `registry_test.go`).

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, TDD Test Plan |
| 3. Wiring phase | Wiring Test table (existing tests characterize current behavior) |
| 4. Implement (TDD) | Implementation Phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7-10. Critical review + fixes | Critical Review Checklist |
| 11-14. Deliverables, security, summary | tail sections |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring / characterization (MANDATORY FIRST)** — capture current behavior before refactoring.
   - Tests: run `TestAcceptLoopPrevention`, `TestHandleBatchConsumerSourceSkipped`, the replay suite; confirm green baseline. Add `TestWouldLoop` asserting `source == dest` semantics (fails first because `WouldLoop` does not exist yet).
   - Files: `internal/core/redistevents/registry_test.go`.
   - Verify: `TestWouldLoop` fails to compile/run (predicate absent); the three existing guards still pass.
2. **Phase: Add the shared predicate** — introduce `WouldLoop(source, dest string) bool` returning `source == dest` in `registry.go`, with a comment documenting it is name-keyed BY DESIGN (works for names outside the producer registry) and is the single definition of the redistribution loop invariant.
   - Tests: `TestWouldLoop` passes.
   - Files: `internal/core/redistevents/registry.go`.
   - Verify: predicate compiles; unit test green.
3. **Phase: Migrate the three call sites** — replace each inline comparison with a `WouldLoop` call, preserving each site's control flow and side effects (Guard 1 reject, Guard 2 skip+metric, Guard 3 early return). Add the redistevents import to `route.go`.
   - Tests: all five Unit Tests pass; no behavior change.
   - Files: `route.go`, `redistribute.go`, `replay.go`.
   - Verify: `make ze-lint-changed`; existing redistribution unit tests green; grep confirms zero remaining inline `== importingProtocol` / `== b.Protocol` loop comparisons.
4. **Functional tests** → re-run the `redist-*` interop scenarios as regression coverage.
5. **RFC refs** → N/A (no protocol/wire behavior).
6. **Full verification** → `make ze-verify`.
7. **Complete spec** → fill audit tables, write learned summary, two-commit closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line; all three guards migrated |
| Feature completeness | Each guard's control flow + side effect preserved (reject / skip+metric / early return) |
| Correctness | `WouldLoop` is pure `source == dest`; matches the exact prior outcome at every site, including the empty edge |
| Naming | predicate named `WouldLoop`; parameters `source`, `dest`; protocol-agnostic (no hardcoded protocol names) |
| Data flow | runtime guards pass the already-resolved `name`; evaluator passes `route.Origin`; no new registry lookups added |
| Registration over hardcoding | `WouldLoop` hardcodes no protocol identity; "bgp" stays local to the replay call site; no per-feature field/switch added to a core/shared struct (`ai/rules/plugin-self-containment.md`) |
| Rule: no-layering | all three inline comparisons deleted; only `WouldLoop` remains |
| Rule: no import cycle | `config/redistribute` -> `core/redistevents` verified acyclic |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| `WouldLoop` exists and is called by all three sites | `grep -rn 'WouldLoop' internal/` shows 1 definition + 3 non-test call sites |
| No inline loop comparison remains | `grep -rn '== importingProtocol\|== b.Protocol' internal/component/config/redistribute internal/component/bgp/plugins/redistribute_egress` returns nothing (outside the predicate) |
| Metric preserved | `TestHandleBatchConsumerSourceSkipped` asserts `filteredProtocolTotal` increment |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | predicate takes two in-process strings from the trusted registry/config; no untrusted input, no injection surface |
| Resource exhaustion | O(1) string compare; no allocation, no new lock acquisition |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Import cycle | reconsider predicate placement (fallback: a tiny `redistcore` leaf package) |
| A redistribution test flips | the migration changed behavior: re-check the site's control flow/side effect against Current Behavior |
| 3 fix attempts fail | STOP, report, ask user |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One shared predicate `redistevents.WouldLoop(source, dest string)`, three call sites kept | Merge the three guards into one; delete two as redundant | The three guard DIFFERENT edges with different side effects (evaluator reject, per-consumer skip + distinct metric, whole-batch early return). Merging would lose the metric bucket and the early-exit optimizations. The duplication is the COMPARISON, not the call sites, so we unify only the comparison. |
| Name-keyed predicate (`source == dest` on strings), NOT ID-keyed | `WouldLoop(a, b ProtocolID)` with each site converting to IDs | Guard 1 must reject origin==importing even for names NOT registered as producers (config-only source names); an ID-keyed predicate would fail-open (`ProtocolIDOf` returns !ok) and regress. Both runtime guards already hold the source name (`ProtocolName(b.Protocol)`), so name-keying adds no lookup. |
| Pure equality, no empty-string special case | `source != "" && source == dest` | Pure `==` preserves Guard 1's exact prior outcome (including empty/empty). The empty case is unreachable at the runtime guards because `b.Protocol` is validated non-empty first. Refining it would be a (albeit unreachable) behavior change, which this refactor must avoid. |
| Predicate lives in `redistevents` (registry.go) | a new `redistcore` package | It belongs beside `ProtocolName`/`ProtocolIDOf`, the existing identity API; no new package needed; component->core import is legal and acyclic. |

**Name<->ID reconciliation (the crux):** the redistevents registry is a bijection.
`RegisterProtocol` allocates a fresh `ProtocolID` for each new name and records it in
`byName` (name->ID) and `entries[id].name` (ID->name). Therefore, for any registered
protocol, `ProtocolName(b.Protocol) == cname` is equivalent to
`ProtocolIDOf(cname).ok && ProtocolIDOf(cname).id == b.Protocol` (the current Guard 2/3
condition). Because both runtime guards run only after `b.Protocol` is validated as a
registered ID and already compute `name := ProtocolName(b.Protocol)`, a name-keyed
`WouldLoop(name, dest)` reproduces the ID comparison exactly, while also satisfying Guard 1's
requirement to work on names that may not be in the producer registry at all. `ProtocolIDOf`
remains the bridge for any caller that holds only an ID (it would call `ProtocolName` first,
which the runtime guards already do).

## Known Limitations
- `consumerProtocolIDs()` / the `skipIDs` map (`redistribute.go:105-113`, used only for a debug log at :186) is a related-but-separate name->ID set builder, not one of the three loop guards. It is out of scope: it builds a membership set, it does not compare source vs destination. Left unchanged.
- The runtime guards remain redundant-in-outcome with the evaluator's Guard 1 (which runs again via `ev.Accept`). This spec does NOT remove that redundancy: the early exits are intentional (short-circuit + distinct metric). Collapsing them further is a separate design question, not a duplication fix.

## RFC Documentation

N/A - no RFC-governed protocol behavior; internal redistribution loop guard only.

## Implementation Summary

### What Was Implemented
- [pending implementation]

### Bugs Found/Fixed
- [pending implementation]

### Documentation Updates
- [pending implementation]

### Deviations from Plan
- [pending implementation]

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Three inline loop comparisons unified behind one predicate | functional + unit test | [pending] |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [pending] | file:line | [pending] |

### Fixes applied
- [pending]

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md` — no failures)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC constraint comments added (N/A here)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases? yes — three call sites)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (N/A — string predicate; documented)
- [ ] Functional tests for end-to-end behavior (existing interop scenarios)
- [ ] Interop tests for protocol features (N/A — no wire change, justified above)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `ai/rules/quality.md` documented pass in spec
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-unify-redist-loop-guard.md`
- [ ] **Commit A:** code + tests + docs + spec (with all edits) + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-unify-redist-loop-guard.md` only
