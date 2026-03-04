# Spec: watchdog-test-coverage

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/bgp/plugins/bgp-watchdog/server.go` — command dispatch, state-up/down
4. `internal/component/bgp/plugins/bgp-watchdog/pool.go` — PoolSet, RoutePool, PoolEntry
5. `internal/component/bgp/plugins/bgp-watchdog/watchdog.go` — SDK wiring, event parsing

## Task

Improve test coverage for the bgp-watchdog plugin. The plugin has 25 unit tests and 1 functional test, but critical scenarios are untested: rapid peer flapping, wildcard dispatch with mixed peer states, multi-pool interactions, and reconnect after explicit withdraw override.

**Scope:** Unit tests for server.go and pool.go behavior gaps, plus 1-2 additional functional tests. No feature code changes (bug fix for mixed initial state was already applied separately).

**Why now:** Watchdog has the worst test-to-risk ratio of the non-trivial plugins. Routes silently announced or withdrawn wrong cause traffic blackholes in production.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/rib-transition.md` — watchdog plugin extraction context
  → Constraint: watchdog manages per-peer config-based route pools, injected via "update text" commands

### RFC Summaries (MUST for protocol work)
Not applicable — watchdog is application-level, not protocol.

**Key insights:**
- watchdogServer decoupled from SDK via sendRoute callback — fully testable without plugin infra
- Per-peer state: `peerPools` (config-driven route definitions) + `peerUp` (session state)
- Pool state is per-peer: same route can be announced for peer A, withdrawn for peer B
- `AnnounceInitial` (new) handles first-session for `initiallyAnnounced` routes only
- Wildcard peer `*` dispatches to all peers with the named pool

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `server.go` (199L) — watchdogServer: handleCommand (announce/withdraw dispatch), handlePoolAction (single/wildcard), handleStateUp (reconnect resend with AnnounceInitial), handleStateDown
  → Constraint: handlePoolAction flips state even if peer is down (for reconnect correctness)
  → Constraint: wildcard dispatches to all peers, silently skips peers without the pool
- [ ] `pool.go` (305L) — PoolSet (thread-safe multi-pool), RoutePool (per-name route store), PoolEntry (per-peer announced state)
  → Constraint: PoolSet.AnnouncePool marks ALL routes as announced for a peer (bulk)
  → Constraint: PoolSet.AnnounceInitial marks only initiallyAnnounced routes (selective)
  → Constraint: empty pools auto-cleaned on last route removal
- [ ] `server_test.go` — 9 tests: command announce/withdraw, unknown group, reconnect resend, disconnected state update, initially announced/withdrawn, state-down prevents sends, mixed initial state
- [ ] `pool_test.go` — 7 tests: CRUD, duplicate, announce/withdraw state, nonexistent pool, entry state, auto-cleanup, concurrency
- [ ] `watchdog_test.go` — 1 table test: parseStateEvent text parsing
- [ ] `config_test.go` — 9 tests: config parsing variants
- [ ] `test/plugin/watchdog.ci` — single functional test: config routes + announce/withdraw cycle x3

**Behavior to preserve:**
- announce/withdraw commands accepted while peer is down (state saved for reconnect)
- reconnect resends all announced routes
- initiallyAnnounced routes auto-sent on first session
- initially-withdrawn routes require explicit command
- wildcard `*` dispatches to all peers

**Behavior to change:**
- None — test-only spec

## Data Flow (MANDATORY)

### Entry Point
- State events arrive as text: `"peer 10.0.0.1 asn 65001 state up\n"` via SDK OnEvent
- Commands arrive via SDK OnExecuteCommand: `"bgp watchdog announce"`, args `["dnsr"]`, peer `"10.0.0.1"`

### Transformation Path
1. `parseStateEvent` extracts peer address + state from text
2. `handleStateUp`/`handleStateDown` updates `peerUp` map, triggers route resend
3. `handleCommand` dispatches to `handlePoolAction` (single or wildcard)
4. `handlePoolAction` flips pool state, sends routes if peer is up

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin ↔ Engine | `sendRoute(peer, cmd)` — "update text" commands | [ ] existing tests |

### Integration Points
- `sdk.NewWithConn` — plugin SDK wiring in watchdog.go
- `p.UpdateRoute` — actual sendRoute in production (SDK RPC)

### Architectural Verification
- [ ] No bypassed layers — tests use same watchdogServer as production
- [ ] No unintended coupling — sendRoute callback isolates from SDK
- [ ] No duplicated functionality — extends existing test suite
- [ ] Zero-copy N/A — text commands, not wire bytes

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Config route + peer up/down/up | → | handleStateUp resend | `test/plugin/watchdog-reconnect.ci` |
| Config + wildcard announce cmd | → | handlePoolActionAll | `TestWildcardMixedPeerStates` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Rapid flap: up→down→up in quick succession | Only the final state's routes sent; no sends while down |
| AC-2 | Wildcard announce with 3 peers: 1 up, 1 down, 1 no pool | Routes sent to up peer only; down peer state updated; no-pool peer skipped |
| AC-3 | Peer has 2 pools: announce pool-A, withdraw pool-B concurrently | Each pool's state independent; correct routes sent for each |
| AC-4 | Explicit withdraw then reconnect | Withdrawn route NOT resent on reconnect |
| AC-5 | Peer reconnects after session established | All announced routes resent, withdrawn routes not resent |
| AC-6 | Wildcard withdraw with no peers having the pool | Returns success with 0 peers affected |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRapidFlap` | `server_test.go` | AC-1: up/down/up sequence, only final announced routes sent | |
| `TestWildcardMixedPeerStates` | `server_test.go` | AC-2: wildcard with up/down/no-pool peers | |
| `TestMultiPoolIndependence` | `server_test.go` | AC-3: two pools, announce one + withdraw other | |
| `TestExplicitWithdrawSurvivesReconnect` | `server_test.go` | AC-4: withdraw command → reconnect → route not resent | |
| `TestReconnectResendAfterEstablished` | `server_test.go` | AC-5: full cycle up→announce→down→up | |
| `TestWildcardNonexistentPool` | `server_test.go` | AC-6: wildcard on pool no peer has | |
| `TestAnnounceInitialPool` | `pool_test.go` | AnnounceInitial only marks initiallyAnnounced entries | |

### Boundary Tests (MANDATORY for numeric inputs)
Not applicable — watchdog has no numeric range inputs.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `watchdog-reconnect` | `test/plugin/watchdog-reconnect.ci` | AC-5: peer drops and reconnects, announced routes resent | |

### Future (if deferring any tests)
- Concurrent command + state-event stress test (needs careful goroutine coordination, low risk given mutex design)

## Files to Modify
- `internal/component/bgp/plugins/bgp-watchdog/pool.go` — added AnnounceInitial method (bug fix for over-announce)
- `internal/component/bgp/plugins/bgp-watchdog/server.go` — handleStateUp uses AnnounceInitial instead of AnnouncePool (bug fix)
- `internal/component/bgp/plugins/bgp-watchdog/server_test.go` — new test functions
- `internal/component/bgp/plugins/bgp-watchdog/pool_test.go` — AnnounceInitial test

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A |
| RPC count in architecture docs | No | N/A |
| CLI commands/flags | No | N/A |
| CLI usage/help text | No | N/A |
| API commands doc | No | N/A |
| Plugin SDK docs | No | N/A |
| Editor autocomplete | No | N/A |
| Functional test for new RPC/API | No | N/A |

## Files to Create
- `test/plugin/watchdog-reconnect.ci` — functional test for reconnect resend

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Write unit tests for AC-1 through AC-6** → Review: edge cases? Right assertions?
2. **Run tests** → Verify FAIL (paste output). Fail for RIGHT reason?
3. **All tests should PASS immediately** — these test existing behavior, not new features. If any fail, that's a bug to fix.
4. **Write AnnounceInitial pool test** → Verify the new method from the bug fix
5. **Write functional test** → `watchdog-reconnect.ci`
6. **Verify all** → `make ze-test`
7. **Critical Review** → All 6 checks from `rules/quality.md`
8. **Complete spec** → Audit, learned summary

### Failure Routing

| Failure | Route To |
|---------|----------|
| Test fails unexpectedly | Investigate — may be a new bug (like the mixed initial state bug) |
| Functional test setup issues | Check test/plugin/watchdog.ci for working patterns |
| Lint failure | Fix inline |

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

## RFC Documentation

Not applicable — watchdog is application-level.

## Implementation Summary

### What Was Implemented

### Bugs Found/Fixed
- `handleStateUp` over-announce bug: `AnnouncePool` marked ALL routes including initially-withdrawn ones. Fixed by adding `AnnounceInitial` method that only marks `initiallyAnnounced` entries. Test: `TestStateUpMixedInitialState`.

### Documentation Updates

### Deviations from Plan

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

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` — no failures)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `docs/learned/NNN-<name>.md`
- [ ] **Summary included in commit** — NEVER commit implementation without the completed summary. One commit = code + tests + summary.
