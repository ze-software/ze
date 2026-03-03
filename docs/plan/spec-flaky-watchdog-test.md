# Spec: flaky-watchdog-test

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `test/plugin/watchdog.ci`
4. Watchdog handling in `internal/plugins/bgp/reactor/`

## Task

Investigate and fix the flaky `watchdog` plugin functional test (`test/plugin/watchdog.ci`), which intermittently fails with a message mismatch.

**Discovery:** Observed during `make ze-verify` on 2026-02-22 while working on `spec-remove-ze-syntax.md`. Passed on immediate re-run, confirming timing/ordering issue.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` — watchdog command flow through reactor
  → Decision: TBD after reading
  → Constraint: TBD after reading

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `test/plugin/watchdog.ci` — Python script loops `bgp watchdog announce/withdraw dnsr` with 0.2s sleeps
- [ ] `internal/plugins/bgp/reactor/reactor.go` — watchdog command handling (not yet read)

**Failure signature:**
- CI type: `mismatch`
- Expected: UPDATE with WITHDRAWN 77.77.77.77/32
- Received: duplicate UPDATE announcements (same prefix re-announced instead of withdrawn)
- Script has a comment noting timing sensitivity around session establishment

**Observed message sequence (failure):**

| # | Message | Expected |
|---|---------|----------|
| 1 | UPDATE announce 77.77.77.77/32 (config) | Yes |
| 2 | UPDATE announce (other routes) | Yes |
| 3 | EOR | Yes |
| 4 | UPDATE announce 77.77.77.77/32 (watchdog) | Yes |
| 5 | UPDATE announce 77.77.77.77/32 (duplicate) | No — expected WITHDRAWN |

**Likely root cause:** The 0.2s sleep between announce/withdraw is a heuristic. Under parallel load (55 plugin tests), the timing window may not suffice, causing command processing to race with UPDATE sending.

**Behavior to preserve:**
- Watchdog announce/withdraw semantics
- Test validates the correct message sequence

**Behavior to change:**
- Make the test deterministic instead of timing-dependent

## Data Flow (MANDATORY)

### Entry Point
- Plugin script sends `bgp watchdog announce dnsr` / `bgp watchdog withdraw dnsr` commands via stdout

### Transformation Path
1. Plugin command → engine parses watchdog command
2. Reactor processes watchdog state change (announce/withdraw)
3. Reactor sends UPDATE or WITHDRAWN to peer

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin → Engine | Text command over stdout pipe | [ ] |
| Reactor → Peer | BGP UPDATE message | [ ] |

### Integration Points
- Watchdog command handler in reactor
- UPDATE building and sending pipeline

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality
- [ ] Zero-copy preserved where applicable

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `watchdog.ci` run 100 times in a loop | Zero failures |
| AC-2 | `watchdog.ci` run under parallel load (`make ze-functional-test`) | Passes reliably |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| TBD after investigation | TBD | TBD | |

### Boundary Tests (MANDATORY for numeric inputs)

No new numeric fields.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `watchdog` | `test/plugin/watchdog.ci` | Watchdog announce/withdraw cycle produces correct UPDATE sequence | Flaky |

## Files to Modify

- `internal/plugins/bgp/reactor/reactor.go` — if watchdog command processing has a race, fix ordering guarantees
- `test/plugin/watchdog.ci` — likely fix: make script deterministic (wait for confirmation before cycling)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | |
| CLI commands/flags | No | |
| Plugin SDK docs | No | |
| Functional test | Already exists | `test/plugin/watchdog.ci` |

## Files to Create

None expected.

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|

## Implementation Steps

1. **Investigate** — read watchdog command handling in reactor, understand timing assumptions in test script
2. **Write test** — add unit test reproducing the race condition → Tests FAIL
3. **Reproduce** — run `ze-test bgp plugin --server s` in a loop (50+ iterations) to reproduce
4. **Identify** — determine whether issue is test script timing, command processing race, or message ordering
5. **Fix** — make test deterministic or fix underlying race → Tests PASS
6. **Verify** — run in loop to confirm fix, then `make ze-verify`

### Failure Routing

| Failure | Route To |
|---------|----------|
| Cannot reproduce | Increase loop count or add artificial load |
| Race in reactor | Fix reactor, add unit test for ordering |
| Test timing only | Fix test script, no reactor changes needed |

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

## Implementation Summary

### What Was Implemented
- Not yet started

### Documentation Updates
- None

### Deviations from Plan
- None

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
- **Total items:** TBD
- **Done:** TBD
- **Partial:** TBD
- **Skipped:** TBD
- **Changed:** TBD

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1, AC-2 demonstrated
- [ ] `make ze-unit-test` passes
- [ ] `make ze-functional-test` passes
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] `make ze-lint` passes
- [ ] Implementation Audit complete

### Design
- [ ] No premature abstraction
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior

### TDD
- [ ] Tests written → FAIL → implement → PASS
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `docs/learned/NNN-<name>.md`
- [ ] Summary included in commit
