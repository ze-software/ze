# Spec: bgp-chaos-shrink (Phase 8 of 9) — SKELETON

**Master design:** `docs/plan/spec-bgp-chaos.md`
**Previous spec:** `spec-bgp-chaos-properties.md`
**Next spec:** `spec-bgp-chaos-inprocess.md`
**DST reference:** `docs/plan/deterministic-simulation-analysis.md` (Section 11.4: Test Case Shrinking)

**Status:** Skeleton — to be fleshed out after Phase 7 completes.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `docs/plan/spec-bgp-chaos.md` - master design
3. `docs/plan/deterministic-simulation-analysis.md` Section 11.4 - shrinking algorithm
4. Phase 6-7 done specs - event log format, property interface
5. `.claude/rules/planning.md` - workflow rules

## Task

When a property violation is found during a chaos run, automatically minimize the reproduction to the smallest sequence of events that still triggers the failure.

A 10-minute chaos run may produce thousands of events, but the actual bug may require only 3 steps to reproduce: connect peer A, announce route, disconnect peer B. Finding that minimal sequence is critical for debugging — it's the difference between a 200-line trace and a 3-line repro.

**Scope:**
- Shrinking engine that takes a failing event log and produces a minimal failing log
- Binary search to find the approximate boundary, then single-step elimination
- Deterministic: shrinking itself is reproducible (driven by the same seed)
- `--shrink <events.jsonl>` CLI mode: reads a failing log, outputs a minimal log
- `--auto-shrink` flag: when a violation is found during a live run, automatically shrink
- Output: minimal event log + human-readable summary of the minimal reproduction

**Relationship to DST:**
This directly implements DST Section 11.4 (Turso's shrink pattern). The algorithm is independent of execution mode — it works on event logs whether produced by external TCP chaos or future in-process simulation. In-process mode (Phase 9) will make shrinking much faster since each replay iteration runs without TCP overhead.

**Dependencies:**
- Phase 6 (event log) — provides the log format and replay capability
- Phase 7 (properties) — properties define the failure criterion

## Required Reading

### Architecture Docs
- [ ] `docs/plan/deterministic-simulation-analysis.md` Section 11.4 - shrinking algorithm
  → Decision: Binary search + single-step elimination
  → Constraint: Shrinking must preserve the same failure (same property violated)
- [ ] `docs/plan/spec-bgp-chaos-eventlog.md` - event log format and replay engine
  → Constraint: Replay must be deterministic for shrinking to work
- [ ] `docs/plan/spec-bgp-chaos-properties.md` - property interface
  → Constraint: Property violations are the failure criterion for shrinking

### Source Code
- [ ] Phase 6 replay engine (`replay/replay.go`)
  → Implemented: `Run(r, w) int` reads NDJSON header, creates Model/Tracker/Convergence, processes events, returns pass/fail via summary
- [ ] Phase 7 property engine (`validation/property.go`)
- [ ] Phase 6 event log format (`report/eventlog.go`)

**Key insights:**
- Shrinking replays the event log through the validation model — no Ze instance needed
- Each shrink iteration is a replay with fewer events, checking if the same property still fails
- Some events are dependent (e.g., can't withdraw a route that was never announced) — shrinking must respect causal ordering
- The binary search phase is fast (O(log n) replays); single-step elimination is slower (O(n) replays in worst case) but produces minimal result
- Dependent events form a DAG — shrinking removes events whose descendants don't include the failure trigger

## Current Behavior (MANDATORY)

**Source files read:** (to be re-read after Phase 7 completes)
- [ ] `cmd/ze-bgp-chaos/replay/replay.go` — replay engine
- [ ] `cmd/ze-bgp-chaos/validation/property.go` — property interface
- [ ] `cmd/ze-bgp-chaos/peer/event.go` — event types and dependencies

**Behavior to preserve:**
- All Phase 1-7 functionality
- Event log format unchanged
- Property interface unchanged

**Behavior to change:**
- Add shrinking as new CLI mode and auto-shrink flag

## Data Flow (MANDATORY)

### Entry Point
- Failing event log file (from `--shrink`) or in-memory event list (from `--auto-shrink`)

### Transformation Path
1. Load full event log (the "failing plan")
2. Verify it actually fails (replay through property engine)
3. Binary search: try first half, second half, narrow to approximate boundary
4. Single-step elimination: try removing each remaining event
5. Causal check: if removing event E, also remove events that depend on E
6. Output: minimal event log + summary

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Shrink engine ↔ Replay engine | Function call (in-process) | [ ] |
| Shrink engine ↔ Property engine | Function call (check after replay) | [ ] |
| CLI ↔ Shrink engine | File I/O (read failing log, write minimal log) | [ ] |

### Integration Points
- Phase 6 replay engine (used for each shrink iteration)
- Phase 7 property engine (defines failure criterion)
- Phase 9 in-process mode (faster replay iterations)

### Architectural Verification
- [ ] Shrinking is pure (no side effects, no TCP, no Ze instance needed)
- [ ] Shrinking preserves the specific property violation (not just any failure)
- [ ] Causal dependencies respected (no impossible event sequences in output)
- [ ] Shrinking terminates (bounded by event count, no infinite loops)

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `--shrink failing.jsonl` | Produces minimal.jsonl that still triggers same violation |
| AC-2 | Minimal log replayed | Same property fails with same violation type |
| AC-3 | 1000-event log with late failure | Shrinks to <20 events in <30 seconds |
| AC-4 | 3-event minimal failure | Returns those 3 events unchanged (already minimal) |
| AC-5 | `--auto-shrink` during live run | Violation triggers automatic shrinking, prints minimal repro |
| AC-6 | Shrunk log | Human-readable summary: "Minimal reproduction: N steps" with step list |
| AC-7 | Causal dependency | Removing announce also removes its withdrawal (not left dangling) |
| AC-8 | Multiple violations | Shrinks for the first violation found |
| AC-9 | No violation in log | `--shrink` reports "no violation found" and exits |
| AC-10 | Shrink progress | Prints progress: "Trying N events... still fails / passes" |

## Shrinking Algorithm

### Phase 1: Verify Failure

Replay full event log through property engine. If no violation, abort.

### Phase 2: Binary Search (coarse)

Split log in half. If first half still fails, recurse on first half. If second half still fails, recurse on second half. If neither fails alone, the failure requires events from both halves — try progressively larger windows around the midpoint.

This phase reduces a 10000-event log to roughly 100-500 events in O(log n) replay iterations.

### Phase 3: Single-Step Elimination (fine)

For each event in the remaining log (in reverse order):
1. Build candidate log without this event (and without its causal dependents)
2. Replay candidate log
3. If same violation occurs: keep the removal (event was unnecessary)
4. If violation disappears: restore the event (it's required)

This phase produces a minimal log where removing any single event causes the violation to disappear.

### Causal Dependencies

Events form a DAG. Key dependencies:

| Event | Depends On |
|-------|------------|
| Withdrawal of prefix P | Announcement of prefix P by same peer |
| UPDATE received | Peer is in Established state |
| Chaos event on peer X | Peer X exists and is connected |
| Reconnect | Prior disconnect of same peer |
| Convergence check | At least one announcement exists |

Removing an event also removes all events that depend on it.

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestShrinkAlreadyMinimal` | `shrink/shrink_test.go` | 3-event minimal → returns same 3 events | |
| `TestShrinkBinarySearch` | `shrink/shrink_test.go` | 100-event log → finds approximate boundary | |
| `TestShrinkSingleStep` | `shrink/shrink_test.go` | Eliminates unnecessary events | |
| `TestShrinkCausalDeps` | `shrink/shrink_test.go` | Removing announce removes its withdrawal | |
| `TestShrinkPreservesViolation` | `shrink/shrink_test.go` | Shrunk log triggers same property | |
| `TestShrinkNoViolation` | `shrink/shrink_test.go` | No violation → returns error | |
| `TestShrinkMultipleViolations` | `shrink/shrink_test.go` | Shrinks for first violation | |
| `TestShrinkDeterministic` | `shrink/shrink_test.go` | Same input → same output | |
| `TestCausalDependencyGraph` | `shrink/causal_test.go` | Dependency DAG correctly computed | |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Event count | 1 - 1M | 1M (may be slow) | 0 (empty log) | N/A |
| Shrink iterations | bounded by event count | N/A | N/A | N/A |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `chaos-shrink` | `test/chaos/shrink.ci` | Produce failing log, shrink it, verify minimal | |

## Files to Create

- `cmd/ze-bgp-chaos/shrink/shrink.go` — shrinking engine (binary search + single-step)
- `cmd/ze-bgp-chaos/shrink/shrink_test.go`
- `cmd/ze-bgp-chaos/shrink/causal.go` — causal dependency graph
- `cmd/ze-bgp-chaos/shrink/causal_test.go`

## Files to Modify

- `cmd/ze-bgp-chaos/main.go` — add `--shrink` and `--auto-shrink` flags
- `cmd/ze-bgp-chaos/orchestrator.go` — wire auto-shrink on first violation

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | N/A |
| Makefile | No | Already has ze-bgp-chaos target |

## Implementation Steps

1. **Read Phase 6-7 learnings** — understand replay engine and property interface
   → Review: How fast is replay? What property violation data is available?

2. **Define causal dependency model**
   → Review: What event types depend on each other?

3. **Write causal dependency tests**
   → Run: Tests FAIL

4. **Implement causal dependency graph**
   → Run: Tests PASS

5. **Write shrinking engine tests** (binary search + single-step)
   → Run: Tests FAIL

6. **Implement shrinking engine**
   → Run: Tests PASS

7. **Wire into CLI** — `--shrink`, `--auto-shrink`

8. **Add progress output** — "Trying N events..."

9. **Verify** — `make lint && make test`

## Spec Propagation Task

**MANDATORY at end of this phase:**

Update the following spec:
1. **`spec-bgp-chaos-inprocess.md`** — shrinking performance expectations with in-process replay

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

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Shrinking engine | | | |
| Binary search phase | | | |
| Single-step elimination | | | |
| Causal dependency graph | | | |
| --shrink CLI mode | | | |
| --auto-shrink flag | | | |
| Human-readable summary | | | |
| Progress output | | | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | | | |
| AC-2 | | | |
| AC-3 | | | |
| AC-4 | | | |
| AC-5 | | | |
| AC-6 | | | |
| AC-7 | | | |
| AC-8 | | | |
| AC-9 | | | |
| AC-10 | | | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 demonstrated
- [ ] Tests pass (`make test`)
- [ ] No regressions (`make functional`)

### Quality Gates (SHOULD pass)
- [ ] `make lint` passes
- [ ] Follow-on spec updated (Spec Propagation Task)
- [ ] Implementation Audit completed

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Implementation complete
- [ ] Tests PASS
- [ ] Boundary tests for numeric inputs

### Completion
- [ ] Spec Propagation Task completed
- [ ] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-bgp-chaos-shrink.md`
