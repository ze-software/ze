# Spec: fixit-vacuous-eor-family-tests

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/fixit-vacuous-eor-family-tests.md` |
| Updated | 2026-08-02 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Three tests in `internal/component/bgp/reactor/peer_test.go` claim to validate
RFC 4724 End-of-RIB family tracking and validate nothing.

| Test | What it does |
|------|--------------|
| `TestFamiliesSentTracking` | Its own comment says "Simulate the familiesSent tracking logic from `sendInitialRoutes`". It populates a local map and asserts on that map |
| `TestFamiliesSentEmpty` | Builds an empty map, asserts it is empty. Four lines, zero production calls. Passes against any implementation |
| `TestFamiliesSentOnlyVPN` | Same shape: local map, inline logic, assertion on the simulation |

**`familiesSent` does not exist in production code.** These tests validate a
variable that lives only inside themselves, while their names and `VALIDATES:`
comments claim they cover End-of-RIB behaviour.

### Why this is worse than dead weight

A session reading `TestFamiliesSentEmpty` concluded ze deviates from RFC 4724
Section 2 by sending End-of-RIB only for families that carried routes, and was
about to escalate a conformance violation to the owner. The producing function
is conformant: `peer_initial_sync.go` `sendInitialRoutes` iterates
`nc.Families()` and sends a marker for every negotiated family, quoting the RFC
line "including the case when there is no update to send".

The tests actively misled a reader into a false finding.

### What the fix is NOT

Do not delete and stop. RFC 4724 Section 2 deserves real coverage and
`ai/rules/testing.md` requires replacement in the same change. The fix
drives `sendInitialRoutes` and asserts on what reaches the wire.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/testing.md` - AC-linked tests
  → Constraint: "If the assertion would still pass with a stub implementation that does nothing, the test is invalid." All three qualify.
- [ ] `ai/rules/testing.md` - governs the replacement
  → Constraint: removal is legitimate only when replacing with better coverage, and the replacement lands in the same change.
- [ ] `ai/rules/testing.md` - the sensitivity ratchets
  → Constraint: the assert-nothing detector did NOT catch these, because they do assert; they assert on themselves. This is a distinct defect class.
- [ ] `ai/rules/rfc-compliance.md` - governs the requirement under test
  → Constraint: a test proving an RFC MUST earns an `RFC requirement:` tag, and `make ze-rfc-index` must be re-run with the ledger committed in the SAME commit.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4724.md` - the End-of-RIB obligation
  → Constraint: Section 2 requires the marker "including the case when there is no update to send", which is exactly the case the vacuous test pretended to cover.

**Key insights:**
- The producing function is CONFORMANT. This spec fixes tests, not protocol behaviour.
- If the replacement needs a production change to pass, a real gap exists and that is a different spec.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/peer_test.go` - the three tests, each building a local `familiesSent` map
- [ ] `internal/component/bgp/reactor/peer_initial_sync.go` - `sendInitialRoutes`: iterates `nc.Families()`, calls `ClaimInitialSyncEOR(fam)` then sends, and counts only frames that reached the socket
- [ ] `ai/rules/testing.md` - to judge whether the ratchet can widen to this class

**Behavior to preserve:**
- `sendInitialRoutes` is correct and does not change.
- The one-End-of-RIB-per-family-per-session claim via `ClaimInitialSyncEOR`.
- Every other test in `peer_test.go`.

**Behavior to change:**
- The three tests are replaced by coverage that drives production code.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A peer reaches Established and initial sync runs.

### Transformation Path
1. `sendInitialRoutes` sends configured static routes for the peer.
2. It iterates `nc.Families()` and, for each, claims the End-of-RIB via
   `ClaimInitialSyncEOR` then sends `message.BuildEOR(fam)`.
3. `IncrEORSent` counts only frames that reached the socket.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Reactor ↔ wire | `message.BuildEOR(fam)` per negotiated family | Yes, read in `peer_initial_sync.go` |
| Reactor ↔ route server | `AnnounceEOR` reaches the same wire; the claim prevents a duplicate | Yes, the comment states it |
| Test ↔ production | currently NONE. That is the defect | Yes |

### Integration Points
- `sendInitialRoutes` is the function under test.
- `ClaimInitialSyncEOR` is why a naive replacement may see one marker where it expects two.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | The replacement must not re-implement the tracking a third time |
| Zero-copy preserved where applicable (refs, not copies) | No | N-A, tests only |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | N-A |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | `sendInitialRoutes` is drivable from a unit test | It is a method on `Peer` and the reactor has test scaffolding | The replacement becomes a `.ci` functional test, a larger change | Read the existing reactor test helpers first | unvalidated |
| A-2 | No other test depends on the three | They assert on local state only | Removal breaks a suite | `go test ./internal/component/bgp/reactor/` after removal | unvalidated |
| A-3 | The ratchet can widen to this class without flooding | It already detects assert-nothing tests | The class stays undetectable by any gate, which is worth stating plainly | Attempt it; if noisy, record the class instead | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The replacement re-implements the logic a third time | The new test contains a local `familiesSent` map | Mutation-verify: disable the End-of-RIB loop and confirm the new test goes red |
| R-2 | `ClaimInitialSyncEOR` makes the replacement see one marker where it expects two | The test fails for a reason unrelated to the assertion | Read the claim before designing the fixture; drive a fresh session per case |
| R-3 | Widening the ratchet floods on table-driven tests that build local fixtures | Hundreds of findings | Widening is optional. Record the class in `ai/rules/testing.md` and stop |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A test suite. No daemon behaviour changes; `sendInitialRoutes` is untouched |
| How is it reverted? | Single commit revert |
| Who else touches this path? | Any session working RFC 4724 or peer initial sync |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A peer negotiating two families with routes for neither | → | `sendInitialRoutes` End-of-RIB loop | `TestEORSentForEveryNegotiatedFamily` in `internal/component/bgp/reactor/peer_initial_sync_test.go` |
| A peer with routes in one family only | → | same | `TestEORSentForSilentFamilyToo` in the same file |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A peer negotiates two families, no routes for either | An End-of-RIB marker is sent for BOTH, asserted on what reached the wire, never on a local map |
| AC-2 | A peer negotiates two families, routes for one | A marker is sent for both, including the silent family (RFC 4724 Section 2) |
| AC-3 | The End-of-RIB loop in `sendInitialRoutes` is disabled | Both new tests go RED; restored, both green. Output pasted for each direction |
| AC-4 | `peer_test.go` after this spec | No test builds a local `familiesSent` map, and the identifier appears in no test file |
| AC-5 | The new tests | Carry an `RFC requirement:` tag for RFC 4724 Section 2, with `ai/RFC-REQUIREMENTS.md` regenerated and committed in the same commit |
| AC-6 | The sensitivity ratchet | Either detects the re-implements-the-logic class, or `ai/rules/testing.md` records the class and states that no gate catches it |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEORSentForEveryNegotiatedFamily` | `internal/component/bgp/reactor/peer_initial_sync_test.go` | AC-1 | |
| `TestEORSentForSilentFamilyToo` | `internal/component/bgp/reactor/peer_initial_sync_test.go` | AC-2 | |
| `TestNoTestBuildsItsOwnFamiliesSentMap` | `internal/component/bgp/reactor/peer_initial_sync_test.go` | AC-4, guards the defect returning | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Negotiated families with no routes | 0-N | N (all silent) | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| survey existing coverage | `test/plugin/*.ci` | Determine whether a `.ci` already proves the silent-family case. If one does, say so and add none; if not, add one | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| survey existing GR scenarios | `test/interop/scenarios/` | FRR or BIRD | Whether a peer already observes End-of-RIB for a silent family. Add only if absent | |

## Files to Modify

- `internal/component/bgp/reactor/peer_test.go` - remove the three vacuous tests
- `ai/rules/testing.md` - record the re-implements-the-logic defect class (AC-6)
- `ai/RFC-REQUIREMENTS.md` - regenerated by `make ze-rfc-index` (AC-5)
- `docs/features/rfc-status.md` - RFC 4724 row, if coverage becomes newly proven

## Files to Create

- `internal/component/bgp/reactor/peer_initial_sync_test.go` - the replacement
- `plan/deferrals/fixit-vacuous-eor-family-tests.md` - deferral shard

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | Tests only |
| YANG validation constraints | N-A | Tests only |
| YANG custom validators | N-A | Tests only |
| CLI commands/flags | N-A | Tests only |
| CLI grammar (keyword before value) | N-A | Tests only |
| Editor autocomplete | N-A | Tests only |
| Functional test for new RPC/API | N-A | No new RPC; existing `.ci` coverage is surveyed |
| Pipe completeness | N-A | No command output |
| Env var registration | N-A | No env var |
| Doctor check for runtime dependencies | N-A | No runtime dependency |
| Prometheus counters/metrics | N-A | `IncrEORSent` exists and is unchanged |
| BGP family surface (new SAFI / capability / attribute) | N-A | No new family |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Test-only change |
| 2 | Config syntax changed? | No | No config |
| 3 | CLI command added/changed? | No | No command |
| 4 | API/RPC added/changed? | No | No RPC |
| 5 | Plugin added/changed? | No | No plugin |
| 6 | Has a user guide page? | No | Test-only |
| 7 | Wire format changed? | No | `sendInitialRoutes` unchanged |
| 8 | Plugin SDK/protocol changed? | No | No SDK surface |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | RFC 4724 Section 2 becomes newly PROVEN: `rfc/short/rfc4724.md` and the `docs/features/rfc-status.md` row |
| 10 | Test infrastructure changed? | Yes | `ai/rules/testing.md` gains the defect class |
| 11 | Affects daemon comparison? | No | No capability change |
| 12 | Internal architecture changed? | No | No architecture change |
| 13 | Route metadata keys added/changed? | No | No metadata |
| 14 | Prometheus counters added/changed? | No | No counters |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | No registration |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Grep `docs/` for anchors naming `peer_test.go` or `peer_initial_sync.go` |
| 17 | Existing docs show config/CLI/API examples for this area? | No | No examples touched |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- create `peer_initial_sync_test.go`
   with both replacement tests written to FAIL, driving `sendInitialRoutes`
   rather than a local map
   - Verify: they compile and fail for the right reason
2. **Phase: Make them pass** against the CURRENT `sendInitialRoutes`. No
   production change should be needed. **If one IS needed, STOP**: a real
   conformance gap exists and it is a different spec
3. **Phase: Mutation-verify** -- disable the End-of-RIB loop, confirm both red,
   restore, confirm green, paste both (AC-3)
4. **Phase: Remove the three vacuous tests**, add the guard against the pattern
5. **Phase: Tag and survey** -- add the `RFC requirement:` tag, run
   `make ze-rfc-index`, survey existing `.ci` and interop coverage before adding

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation |
| Correctness | The replacement drives `sendInitialRoutes`. A local map means the defect was reproduced |
| Correctness | Phase 2 needed no production change |
| Correctness | `ClaimInitialSyncEOR` is accounted for, so a second marker is not expected where the claim suppresses it |
| Evidence | AC-3's mutation output pasted in both directions, not summarised |
| Rule: `ai/rules/testing.md` | Replacement lands in the same change as removal |
| Rule: `ai/rules/rfc-compliance.md` | Ledger regenerated and committed with the tag |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Three vacuous tests gone | `grep -c familiesSent internal/component/bgp/reactor/peer_test.go` returns 0 |
| Replacement drives production | `grep sendInitialRoutes internal/component/bgp/reactor/peer_initial_sync_test.go` |
| Mutation proves it gates | pasted red and green output |
| Ledger fresh | `make ze-rfc-check` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | N-A. Tests only, no new input path |

### Failure Routing
| Failure | Route To |
|---------|----------|
| The replacement needs a production change to pass | STOP. A real RFC 4724 gap exists; raise it per `ai/rules/rfc-compliance.md` |
| `sendInitialRoutes` is not unit-drivable | A-1 broken. Write it as a `.ci` and record the deviation |
| Widening the ratchet floods | R-3. Record the class in `ai/rules/testing.md` and stop |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- A test that re-implements the logic it names is a distinct defect class from a
  test with no assertion. The ratchet catches the second and not the first,
  because these tests DO assert; they assert on themselves.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Replace, do not delete | Delete the three and stop | RFC 4724 Section 2 deserves real coverage, and `testing.md` requires replacement |
| Assert on what reaches the wire | Assert on `IncrEORSent` alone | A counter can be incremented by the wrong path. The frame is the behaviour |
| Widening the ratchet is optional | Make it mandatory | A noisy detector gets switched off; recording the class honestly is the better outcome |

## Known Limitations

- The guard greps one identifier. It cannot catch the same defect written with a
  different variable name.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
