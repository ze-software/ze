# Spec: bgp-chaos-validation (Phase 2 of 5) — SKELETON

**Master design:** `docs/plan/spec-bgp-chaos.md`
**Previous spec:** `spec-bgp-chaos-session.md`
**Next spec:** `spec-bgp-chaos-chaos.md`

**Status:** Skeleton — to be fleshed out after Phase 1 completes.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `docs/plan/spec-bgp-chaos.md` - master design
3. `docs/plan/done/NNN-bgp-chaos-session.md` - Phase 1 learnings
4. `.claude/rules/planning.md` - workflow rules
5. Source files from Phase 1 (listed in Phase 1's Implementation Summary)

## Task

Add multi-peer orchestration and route propagation validation to `ze-bgp-chaos`.

**Scope:**
- Orchestrator: launch and coordinate N peer simulators concurrently
- Route receiver: parse incoming UPDATEs forwarded by the route reflector
- Validation model: expected route state per peer (based on what others announced + family compatibility)
- Convergence tracking: measure announcement-to-receipt latency
- Withdrawal validation: verify withdrawals propagate correctly
- Disconnect validation: verify all routes withdrawn when peer drops
- Reconnect validation: verify all routes replayed when peer returns
- Exit summary: route stats, convergence metrics, pass/fail

**NOT in scope:** Chaos injection (Phase 3), non-v4 families (Phase 4), fancy reporting (Phase 5).

## Required Reading

### Architecture Docs
- [ ] `docs/plan/spec-bgp-chaos.md` - master design (validation model, component architecture)
  → Decision: Validation compares expected state model against actual received routes
  → Constraint: Convergence deadline per route (default 5s)
- [ ] `docs/architecture/core-design.md` - route reflector forwarding behavior
  → Constraint: RR forwards to all compatible established peers except source

### Source Code
- [ ] Phase 1 implementation files (paths TBD after Phase 1)
  → Constraint: Must use Phase 1's actual data structures and interfaces

**Key insights:**
- _To be filled after Phase 1 completes_

## Current Behavior (MANDATORY)

**Source files read:** (to be re-read after Phase 1 completes)
- [ ] `cmd/ze-bgp-chaos/main.go` — CLI entry point, flag parsing
- [ ] `cmd/ze-bgp-chaos/scenario/generator.go` — seed-based PeerProfile generation
- [ ] `cmd/ze-bgp-chaos/scenario/profile.go` — PeerProfile type definition
- [ ] `cmd/ze-bgp-chaos/scenario/routes.go` — IPv4 route generation
- [ ] `cmd/ze-bgp-chaos/scenario/config.go` — Ze config file generation
- [ ] `cmd/ze-bgp-chaos/peer/simulator.go` — per-peer goroutine skeleton
- [ ] `cmd/ze-bgp-chaos/peer/session.go` — TCP + OPEN + KEEPALIVE exchange
- [ ] `cmd/ze-bgp-chaos/peer/sender.go` — route UPDATE building and sending

**Behavior to preserve:**
- Phase 1 CLI interface and config generation
- Phase 1 single-peer session functionality

**Behavior to change:**
- Extend from single peer to multi-peer orchestration
- Add route receiving (Phase 1 only sends)

## Data Flow (MANDATORY)

### Entry Point
- Orchestrator launches N peer simulators (goroutines)
- Each peer reports events via channels to orchestrator

### Transformation Path
1. Orchestrator starts all peers
2. Peers establish sessions, announce routes
3. Peers receive forwarded UPDATEs from RR → parse → report to tracker
4. Validation engine periodically compares expected vs actual
5. On shutdown: final validation pass → exit summary

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Peer goroutine ↔ Orchestrator | Go channels | [ ] |
| Orchestrator ↔ Validation | Direct function calls | [ ] |

### Integration Points
- Phase 1 peer simulator (extended with receiver)
- Phase 1 route data structures (used as validation keys)

### Architectural Verification
- [ ] No shared mutable state between peer goroutines
- [ ] Channel-based communication only

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | 4 peers, all established | All peers running concurrently, each sending routes |
| AC-2 | Peer A announces route R | Peers B, C, D receive R from RR (within convergence deadline) |
| AC-3 | Peer A withdraws route R | Peers B, C, D receive withdrawal |
| AC-4 | Peer A disconnects | Peers B, C, D receive withdrawals for all of A's routes |
| AC-5 | Peer A reconnects | A receives routes from B, C, D |
| AC-6 | --chaos-rate 0 --duration 30s | Pure propagation test: all routes converge, exit summary shows PASS |
| AC-7 | Convergence timeout exceeded | Reported as failure in exit summary |
| AC-8 | All routes propagated correctly | Exit code 0 |
| AC-9 | Missing route detected | Exit code 1, details in summary |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestValidationModelAnnounce` | `validation/model_test.go` | Announce from A → expected at B, C, D | |
| `TestValidationModelWithdraw` | `validation/model_test.go` | Withdraw from A → removed from expected | |
| `TestValidationModelDisconnect` | `validation/model_test.go` | Disconnect A → all A's routes removed | |
| `TestValidationModelReconnect` | `validation/model_test.go` | Reconnect A → A gets all others' routes | |
| `TestValidationModelFamilyFilter` | `validation/model_test.go` | Peer without family F doesn't expect F routes | |
| `TestConvergenceTracking` | `validation/convergence_test.go` | Latency recorded, timeout detected | |
| `TestTrackerConcurrency` | `validation/tracker_test.go` | Concurrent updates from N peers are safe | |
| `TestCheckerMissingRoutes` | `validation/checker_test.go` | Detects routes in expected but not actual | |
| `TestCheckerExtraRoutes` | `validation/checker_test.go` | Detects routes in actual but not expected | |
| `TestOrchestratorStartup` | `orchestrator_test.go` | N peers launched and coordinated | |

_Additional tests to be identified after Phase 1._

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| convergence-deadline | >0 | any duration | 0 | N/A |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `chaos-validation-propagation` | `test/chaos/validation-propagation.ci` | 4 peers, verify all routes propagated | |
| `chaos-validation-withdrawal` | `test/chaos/validation-withdrawal.ci` | 3 peers, withdraw routes, verify propagation | |

## Files to Create

- `cmd/ze-bgp-chaos/orchestrator.go` - multi-peer lifecycle coordination
- `cmd/ze-bgp-chaos/orchestrator_test.go`
- `cmd/ze-bgp-chaos/validation/model.go` - expected state model
- `cmd/ze-bgp-chaos/validation/model_test.go`
- `cmd/ze-bgp-chaos/validation/tracker.go` - actual received state
- `cmd/ze-bgp-chaos/validation/tracker_test.go`
- `cmd/ze-bgp-chaos/validation/checker.go` - expected vs actual comparison
- `cmd/ze-bgp-chaos/validation/checker_test.go`
- `cmd/ze-bgp-chaos/validation/convergence.go` - latency tracking
- `cmd/ze-bgp-chaos/validation/convergence_test.go`
- `cmd/ze-bgp-chaos/peer/receiver.go` - incoming UPDATE parsing
- `cmd/ze-bgp-chaos/peer/receiver_test.go`
- `cmd/ze-bgp-chaos/report/summary.go` - exit summary
- `cmd/ze-bgp-chaos/report/summary_test.go`

## Files to Modify

- `cmd/ze-bgp-chaos/main.go` - wire orchestrator into CLI
- `cmd/ze-bgp-chaos/peer/simulator.go` - add receiver, event reporting
- `cmd/ze-bgp-chaos/peer/session.go` - add incoming message parsing

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | N/A |
| Makefile | No | Already added in Phase 1 |

## Files to Create

_Listed above._

## Implementation Steps

1. **Read Phase 1 learnings** - understand actual data structures and interfaces
   → Review: Do I know the PeerProfile struct, route types, session API?

2. **Write validation model tests** - expected state tracking
   → Run: Tests FAIL

3. **Implement validation model** - announce/withdraw/disconnect/reconnect logic
   → Run: Tests PASS

4. **Write tracker tests** - concurrent actual state tracking
   → Run: Tests FAIL

5. **Implement tracker** - thread-safe route tracking
   → Run: Tests PASS

6. **Write checker tests** - expected vs actual comparison
   → Run: Tests FAIL

7. **Implement checker** - missing/extra route detection, convergence timeout
   → Run: Tests PASS

8. **Write orchestrator tests** - multi-peer coordination
   → Run: Tests FAIL

9. **Implement orchestrator** - launch N peers, coordinate lifecycle
   → Run: Tests PASS

10. **Implement receiver** - parse incoming UPDATEs, feed to tracker
    → Run: Integration test with Ze

11. **Implement exit summary** - report stats and pass/fail
    → Run: Manual verification

12. **Verify** - `make lint && make test`

13. **Update follow-on specs** (Spec Propagation Task)

## Spec Propagation Task

**MANDATORY at end of this phase:**

Before marking this spec complete, update the following specs:

1. **`spec-bgp-chaos-chaos.md`** — Update with:
   - How the orchestrator coordinates peer lifecycle (for chaos to interrupt)
   - Validation model API (so chaos events can trigger expected-state updates)
   - How reconnection works end-to-end (what the chaos executor needs to do)

2. **`spec-bgp-chaos-families.md`** — Update with:
   - Route key format (does it already support multi-family?)
   - Validation model family-awareness (how family filtering works in practice)
   - Receiver parsing (can it already handle MP_REACH/MP_UNREACH?)

3. **`spec-bgp-chaos-reporting.md`** — Update with:
   - Event bus message types (what events are available for reporting)
   - Convergence data structures (what metrics are tracked)
   - Summary format (what data is available for the exit report)

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
| Multi-peer orchestration | | | |
| Route receiving | | | |
| Validation model | | | |
| Convergence tracking | | | |
| Withdrawal validation | | | |
| Disconnect validation | | | |
| Reconnect validation | | | |
| Exit summary | | | |

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
- [ ] AC-1..AC-9 demonstrated
- [ ] Tests pass (`make test`)
- [ ] No regressions (`make functional`)

### Quality Gates (SHOULD pass)
- [ ] `make lint` passes
- [ ] Follow-on specs updated (Spec Propagation Task)
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
- [ ] Spec moved to `docs/plan/done/NNN-bgp-chaos-validation.md`
