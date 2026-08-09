# Spec: fixit-vacuous-eor-family-tests

| Field | Value |
|-------|-------|
| Status | done |
| Scope | protocol |
| Depends | - |
| Phase | closed |
| Deferral shard | `plan/deferrals/fixit-vacuous-eor-family-tests.md` (created 2026-08-09 on the first deferral, which is a defect review pass 1 found in a neighbouring tool, not a reduction of this spec's scope) |
| Updated | 2026-08-09 |

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
| A-1 | `sendInitialRoutes` is drivable from a unit test | It is a method on `Peer` and the reactor has test scaffolding | The replacement becomes a `.ci` functional test, a larger change | Read the existing reactor test helpers first | CONFIRMED, and stronger than assumed: `newInitialSyncPeer` (`peer_initial_sync_test.go`) already drives it against a `recordingConn`, so no new scaffolding was written |
| A-2 | No other test depends on the three | They assert on local state only | Removal breaks a suite | `go test ./internal/component/bgp/reactor/` after removal | CONFIRMED, `make ze-test-pkg PKG=./internal/component/bgp/reactor` exits 0 after removal |
| A-3 | The ratchet can widen to this class without flooding | It already detects assert-nothing tests | The class stays undetectable by any gate, which is worth stating plainly | Attempt it; if noisy, record the class instead | BROKEN. Every table-driven test builds local fixtures, so a detector for "asserts on a local it filled itself" fires on hundreds of correct tests. Recorded instead, per R-3 and the Key Design Decision |

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
| A peer negotiating two families with routes for neither | → | `sendInitialRoutes` End-of-RIB loop | `TestInitialSyncEORCountedOncePerFamilyOnTheWire` in `internal/component/bgp/reactor/peer_initial_sync_test.go`. It already existed when this spec was written (2026-07-26 against 2026-08-02) and covers AC-1 exactly, so the spec's `TestEORSentForEveryNegotiatedFamily` would have duplicated it |
| A peer with routes in one family only | → | same | `TestInitialSyncEORReachesTheSilentFamilyToo` in the same file |
| A peer negotiating two families over the daemon, route in one | → | same, through the real socket | `test/encode/eor-silent-family.ci` |

## Acceptance Criteria

**Narrowing, 2026-08-09.** AC-4 originally claimed "the identifier appears in no
test file", which reads repo-wide. The guard cannot make that claim:
`filepath.Glob("*_test.go")` resolves against the package directory, so it reaches
`internal/component/bgp/reactor` and nothing else. The repo-wide fact is true today
and is recorded as a grep; the ENFORCED claim is the package one. Walking the repo
root from a unit test was rejected: a repo-wide identifier gate belongs in a repo
gate, not in one package's test binary.

**Correction, 2026-08-09.** AC-2 and AC-5 cite "RFC 4724 Section 2" for the clause
"including the case when there is no update to send". That clause is Section 4
(`rfc/full/rfc4724.txt:303-306`), requirement `RFC4724-4-1`. Section 2 defines the
marker's encoding and recommends its use (`RFC4724-2-1`, RECOMMENDED). The tags name
the section the obligation actually lives in.

| AC ID | Input / Condition | Expected Behavior | Status |
|-------|-------------------|-------------------|--------|
| AC-1 | A peer negotiates two families, no routes for either | An End-of-RIB marker is sent for BOTH, asserted on what reached the wire, never on a local map | MET by `TestInitialSyncEORCountedOncePerFamilyOnTheWire`, which already existed and asserts `conn.written()` equals both markers in AFI order |
| AC-2 | A peer negotiates two families, routes for one | A marker is sent for both, including the silent family (RFC 4724 Section 4, `RFC4724-4-1`) | MET by `TestInitialSyncEORReachesTheSilentFamilyToo` and `test/encode/eor-silent-family.ci` |
| AC-3 | The End-of-RIB loop in `sendInitialRoutes` is disabled | Both new tests go RED; restored, both green. Output pasted for each direction | MET, two mutations, both pasted in the Goal Validation section |
| AC-4 | `peer_test.go` after this spec | No test builds a local `familiesSent` map, and the identifier appears in no test file **of the `reactor` package** (narrowed 2026-08-09, see below) | MET. `git grep -n familiesSent -- '*.go'` returns NOTHING, and `TestNoTestBuildsItsOwnFamiliesSentMap` holds the `reactor` package to it |
| AC-5 | The new tests | Carry an `RFC requirement:` tag for the End-of-RIB obligation, with `ai/RFC-REQUIREMENTS.md` regenerated and committed in the same commit | MET, `RFC4724-4-1` and `RFC4724-4.2-9` each gained a reactor-level positive |
| AC-6 | The sensitivity ratchet | Either detects the re-implements-the-logic class, or `ai/rules/testing.md` records the class and states that no gate catches it | MET by recording. A-3 is broken: a detector for this class fires on every table-driven test |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestInitialSyncEORCountedOncePerFamilyOnTheWire` | `internal/component/bgp/reactor/peer_initial_sync_test.go` | AC-1 | PASS, already existed |
| `TestInitialSyncEORReachesTheSilentFamilyToo` | `internal/component/bgp/reactor/peer_initial_sync_test.go` | AC-2 | PASS, new |
| `TestNoTestBuildsItsOwnFamiliesSentMap` | `internal/component/bgp/reactor/peer_initial_sync_test.go` | AC-4, guards the defect returning | PASS, new, and proven to go red against a tripwire |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Negotiated families with no routes | 0-N | N (all silent) | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| survey existing coverage | `test/plugin/*.ci`, `test/encode/*.ci` | Determine whether a `.ci` already proves the silent-family case. If one does, say so and add none; if not, add one | DONE. `test/plugin/eor.ci` proves the two-family case with NO routes at all, where every family is silent and both readings agree. The candidate set is the `.ci` files matching all three of `grep -q ipv4/unicast`, `grep -q ipv6/unicast` and `grep -qi eor` across `test/plugin/*.ci test/encode/*.ci`: 18 at survey time, 19 now that this spec's own `test/encode/eor-silent-family.ci` joined it. None of the original 18 puts a route in one family and leaves the other silent |
| `eor-silent-family` | `test/encode/eor-silent-family.ci` | A peer negotiates ipv4 and ipv6, ze announces one ipv4 prefix, and the peer receives the route then BOTH markers | PASS, new (56/56 in `make ze-encode-test`). Drafted in `test/draft/encode/` and promoted green |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| survey existing GR scenarios | `test/interop/scenarios/` | FRR or BIRD | Whether a peer already observes End-of-RIB for a silent family. Add only if absent | N-A, and the reason is the rule's own exemption: this spec changes no wire-visible behavior (`git diff --stat` on `peer_initial_sync.go` is empty), so `ai/rules/interop-and-goal-validation.md` "When interop tests are NOT required" applies on its first row. Existing interop scenarios cover the End-of-RIB path |

## Files to Modify

- `internal/component/bgp/reactor/peer_test.go` - remove the three vacuous tests
- `internal/component/bgp/reactor/peer_initial_sync_test.go` - the replacement, ADDED to a file that already existed
- `ai/rules/points/testing/test-sensitivity-ratchets-blocking/a-test-that-re-implements-the-logic-it-names.md` plus its manifest line - record the defect class (AC-6). `ai/rules/testing.md` is GENERATED from these
- `ai/RFC-REQUIREMENTS.md` - regenerated by `make ze-rfc-index` (AC-5)
- `docs/features/rfc-status.md` - NOT touched. RFC 4724's `{gap}` count is unchanged, so `check_gap_count_agreement` and `check_status_completeness` both stay satisfied; `make ze-rfc-check` exits 0

## Files to Create

- `test/encode/eor-silent-family.ci` - the functional half of AC-2
- `plan/deferrals/fixit-vacuous-eor-family-tests.md` - created, one row, LIVE (`deferred`)
- `plan/spec-fixit-relax-audit-reports-the-wrong-token.md` - the found-problem spec that row points at

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
| 9 | RFC behavior implemented, changed, or newly proven? | Yes, but not the section this row first named (corrected 2026-08-09) | `RFC4724-4-1` (Section 4) and `RFC4724-4.2-9` (Section 4.2) each gain a reactor-level positive in `ai/RFC-REQUIREMENTS.md`. NO Section 2 row moved: `RFC4724-2-1` is RECOMMENDED, covers the marker's encoding and the recommendation to send it, and still has no test. `rfc/short/rfc4724.md` and `docs/features/rfc-status.md` are unchanged, because no `{gap}` moved |
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
| Replace, do not delete | Delete the three and stop | The End-of-RIB obligation deserves real coverage, and `testing.md` requires replacement. The obligation is `RFC4724-4-1` (Section 4), not Section 2: see the Correction above the Acceptance Criteria |
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

---

## Implementation Summary

### What Was Implemented

- `TestInitialSyncEORReachesTheSilentFamilyToo` (`internal/component/bgp/reactor/peer_initial_sync_test.go`):
  two negotiated families, an ipv4 static route, no ipv6 route. It drives
  `sendInitialRoutes` and asserts the wire bytes end with BOTH markers and that the
  route UPDATE precedes them. Tagged `RFC4724-4-1` and `RFC4724-4.2-9`, positive.
- `TestNoTestBuildsItsOwnFamiliesSentMap` in the same file: greps this package's
  `*_test.go` for the identifier and refuses its return.
- The three vacuous tests deleted from `peer_test.go`, replaced by a `test-relax:`
  gravestone naming what they claimed, why they were green against any
  implementation, and where the replacement coverage lives.
- `test/encode/eor-silent-family.ci`: the same scenario through the daemon, exact
  hex for the route UPDATE and both markers.
- A new rule point recording the defect class, in
  `ai/rules/points/testing/test-sensitivity-ratchets-blocking/`.

### Bugs Found/Fixed

- No product bug. `sendInitialRoutes` was already conformant, which is what this
  spec predicted and what `git diff --stat` on it now proves.

### Documentation Updates

- `ai/rules/testing.md`, `ai/rules/TRIGGERS.md`, `ai/rules/CORE.md`,
  `ai/rules/INDEX.md`: regenerated from the new point (`make ze-rules-render`,
  `ze-rules-condensed`, `ze-rules-index`; `ze-rules-lint` exits 0).
- `ai/RFC-REQUIREMENTS.md`: regenerated (`make ze-rfc-index`).
- Four `docs/` source anchors DO name these files, and each was read rather than
  assumed away. Two name the unchanged `peer_initial_sync.go`
  (`docs/guide/quickstart.md`, `docs/guide/plugins.md`). Two name the changed
  `peer_test.go` (`docs/architecture/behavior/peer-lifecycle.md`,
  `docs/architecture/behavior/fsm-established.md`), and both describe it as
  covering peer lifecycle, teardown, reconnect backoff and the FSM state-change
  callback. Removing three End-of-RIB family tests touches none of that, so both
  claims stay true. `make ze-doc-test` exits 0, but note what that proves: its
  `code_to_docs.py --check` verifies the anchored PATH exists, never that the
  sentence above the anchor is still current. Reading them is the only check.

### Deviations from Plan

- **AC-1 needed no new test, and that was true before the spec was written.**
  `TestInitialSyncEORCountedOncePerFamilyOnTheWire` landed 2026-07-26 in
  `756a4514e`; this spec landed 2026-08-02 in `98ef9cf16`, seven days later. So
  the spec commissioned `TestEORSentForEveryNegotiatedFamily` while an equivalent
  test already existed in the file it named as a file to CREATE. Writing it would
  have duplicated live coverage.
- **`peer_initial_sync_test.go` was a "Files to Create" entry that already
  existed**, with the `newInitialSyncPeer` / `recordingConn` scaffolding A-1 hoped
  for. The replacement was added to it.
- **The functional test went to `test/encode/`, not `test/plugin/`**, because it
  needs no plugin: `ai/rules/testing.md` maps BGP wire behavior with a hex match to
  that suite.
- **A deferral shard WAS created, and it stays live.** The implementation deferred
  nothing, but review pass 1 found a defect in a neighbouring tool, so
  `plan/deferrals/fixit-vacuous-eor-family-tests.md` exists with one `deferred` row
  naming `plan/spec-fixit-relax-audit-reports-the-wrong-token.md`. It is not
  removed at closure.
- **`docs/features/rfc-status.md` was NOT modified**, though "Files to Modify"
  named it conditionally. RFC 4724's `{gap}` count did not move, so
  `check_gap_count_agreement` and `check_status_completeness` stay satisfied and
  `make ze-rfc-check` exits 0. Editing the page would have been a change with no
  fact behind it.
- **AC-6 was met by recording, not by widening the ratchet.** A-3 is broken.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-3 assumed the sensitivity ratchet could grow a detector for this class | Every table-driven test builds a local fixture and asserts on it, so the detector's signal is indistinguishable from correct practice | Reasoning about the detector's input before writing it; the spec's own Key Design Decision had already reached the same place | Recorded the class in the rule instead, and said plainly that no gate catches it |
| approach | The first `.ci` was written expecting it to discriminate the narrowing mutation | It stays GREEN under that mutation, three consecutive runs, because `AnnounceEOR` is a second path to the same wire | Ran the mutated daemon against the draft | Kept the test for the daemon-level claim it does prove, and wrote the limitation into the file rather than letting the name oversell it |
| approach | The route UPDATE hex was first guessed by editing a sample frame from `test/encode/ipv46routes4family.ci` | The guess was 4 octets long: it kept an extended-community attribute the config does not carry | The draft run printed expected against received | Recomputed the frame field by field and wrote the arithmetic into the `.ci`, so the hex is derived rather than copied |
| escalation | Two evidence cells cited `grep -rn familiesSent --include='*.go' .` as returning "only the guard's own split literal" | Both halves were false. That form walks `.claude/worktrees/` and returned 19 hits from another session's checkout, while the guard's split needle matches nothing at all. The true evidence is `git grep -n familiesSent -- '*.go'`, which returns nothing | Review pass 2 ran both commands | Replaced both cells. The lesson is in `plan/learned/1368-vacuous-eor-family-tests.md`: in a repo with agent worktrees, `git grep` is the evidence and `grep -rn .` is not. Pasting an unrun command as evidence is the same defect class this spec exists to remove, one layer up |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Three tests validating nothing are removed | Done | `peer_test.go`, where the gravestone comment now stands | |
| The fix drives `sendInitialRoutes` and asserts what reaches the wire | Done | `peer_initial_sync_test.go` `TestInitialSyncEORReachesTheSilentFamilyToo` | Asserts `conn.written()` |
| Do not delete and stop: RFC 4724 gets real coverage | Done | same, plus `test/encode/eor-silent-family.ci` | Replacement lands in the same change |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestInitialSyncEORCountedOncePerFamilyOnTheWire` | Pre-existing since `756a4514e` (2026-07-26), seven days BEFORE this spec. Verified it covers the AC by reading it |
| AC-2 | Done | `TestInitialSyncEORReachesTheSilentFamilyToo`, `test/encode/eor-silent-family.ci` | |
| AC-3 | Done | Two mutations, output in Goal Validation | |
| AC-4 | Done | `TestNoTestBuildsItsOwnFamiliesSentMap` | Proven to go red against a tripwire |
| AC-5 | Done | `ai/RFC-REQUIREMENTS.md` rows for `RFC4724-4-1`, `RFC4724-4.2-9` | `make ze-rfc-check` exits 0 |
| AC-6 | Done | `ai/rules/points/testing/test-sensitivity-ratchets-blocking/a-test-that-re-implements-the-logic-it-names.md` | Records that no gate catches the class, and why |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| AC-1 test | Changed | `peer_initial_sync_test.go` `TestInitialSyncEORCountedOncePerFamilyOnTheWire` | Existing test used instead of the spec's invented name |
| AC-2 test | Done | `peer_initial_sync_test.go` | |
| Guard test | Done | `peer_initial_sync_test.go` | |
| Functional survey | Done | see Functional Tests table | Produced one new `.ci` |
| Interop survey | Done | see Interop Tests table | N-A: no wire-visible change |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/bgp/reactor/peer_test.go` | Done | Three tests removed |
| `internal/component/bgp/reactor/peer_initial_sync_test.go` | Changed | Existed already; extended |
| `ai/rules/testing.md` | Done | Via its point directory; the rendered file is generated |
| `ai/RFC-REQUIREMENTS.md` | Done | Regenerated |
| `docs/features/rfc-status.md` | Changed | Not touched: no gap count moved |
| `plan/deferrals/fixit-vacuous-eor-family-tests.md` | Changed | Created, but for a reason the plan did not anticipate: a defect review pass 1 found in a neighbouring tool. Its one row is live |

### Audit Summary
- **Total items:** 20 (Requirements 3, Acceptance Criteria 6, Tests 5, Files 6)
- **Done:** 16 (3 + 6 + 4 + 3)
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 4 (0 + 0 + 1 + 3), each recorded in Deviations: the AC-1 test, the
  pre-existing `peer_initial_sync_test.go`, `docs/features/rfc-status.md` left
  untouched, and the deferral shard created for a reason the plan did not foresee

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| The three tests no longer mislead a reader into a false RFC 4724 finding | functional (grep) | `git grep -n familiesSent -- '*.go'` returns NOTHING, and `TestNoTestBuildsItsOwnFamiliesSentMap` keeps the `reactor` package that way. Use `git grep`, not `grep -rn --include='*.go' .`: the latter returns 19 hits from another session's checkout under `.claude/worktrees/`, which `.git/info/exclude` keeps out of the tracked tree. The guard's own occurrence is a split literal, so it matches neither command |
| RFC 4724's End-of-RIB obligation has real coverage that drives production code | unit, mutation-proven | Mutation M1 (loop disabled) and M2 (loop narrowed to route-carrying families), both below |
| The same behavior is proven through the daemon | functional | `test/encode/eor-silent-family.ci`, 56/56 in `make ze-encode-test` |
| `sendInitialRoutes` was not changed | source | `git diff --stat -- internal/component/bgp/reactor/peer_initial_sync.go` prints nothing |

All three blocks below come from `make ze-test-pkg
PKG=./internal/component/bgp/reactor
RUN='TestInitialSyncEORCountedOncePerFamilyOnTheWire|TestInitialSyncEORReachesTheSilentFamilyToo'`,
run with `GOFLAGS=-v` so the per-test result lines exist at all: the target passes
no `-v` of its own (`mk/test-unit.mk`), so a bare run prints only `ok` or `FAIL`.
They are CONDENSED for width: make's banner lines, testify's `Error Trace:` /
`Test:` / location lines, its byte-by-byte `Diff:` dump, and blank and `=== RUN`
lines are gone, and some multi-line assertions are folded onto one.

**The claim made about them is narrow, and it is the one that matters: no failing
assertion is omitted, and every `Error`, `expected`, `actual` and `Messages` value
is quoted unaltered.** Earlier drafts here tried to enumerate every dropped line
kind and to end on "nothing else is removed". Three review passes each found
another kind, which is what an exhaustiveness claim over hand-edited text is worth.
The verbatim runs are the artifact; re-derive them with the command above rather
than trusting a transcript of a transcript.

The filter selects exactly those two tests: no third name in the package contains
either substring. Widening it reddens `TestInitialSyncEORWaitsForPeerUpBarrier` as well,
because its fixture carries no route and M2 therefore leaves its family list empty;
that is expected and is not part of AC-3.

**Mutation M1 -- `families := nc.Families()` replaced by an empty slice:**

```
    Error: Not equal:
        expected: []byte{0xff, ... 0x0, 0x2, 0x1}
        actual  : []byte(nil)
    Messages: both negotiated families' End-of-RIB markers must reach the wire, AFI order
    Error: Not equal: expected: 0x2  actual: 0x0
    Messages: one counted EOR per family sent
--- FAIL: TestInitialSyncEORCountedOncePerFamilyOnTheWire (0.00s)
    Error: Should be true
    Messages: both negotiated families' markers must close the initial update, the silent one included
    Error: "47" is not greater than "53"
    Messages: the ipv4 route UPDATE must precede the markers, or the fixture sent no route and the silent-family case is not the one under test
    Error: Not equal: expected: 0x2  actual: 0x0
    Messages: one counted EOR per negotiated family
--- FAIL: TestInitialSyncEORReachesTheSilentFamilyToo (0.00s)
FAIL	github.com/ze-software/ze/internal/component/bgp/reactor	0.709s
```

**Mutation M2 -- the loop narrowed to the families that carried a static route,
which is exactly the reading the three deleted tests encoded:**

```
    Error: Not equal: expected: []byte{0xff, ... 0x0, 0x2, 0x1}  actual: []byte(nil)
    Messages: both negotiated families' End-of-RIB markers must reach the wire, AFI order
    Error: Not equal: expected: 0x2  actual: 0x0
    Messages: one counted EOR per family sent
--- FAIL: TestInitialSyncEORCountedOncePerFamilyOnTheWire (0.00s)
    Error: Should be true
    Messages: both negotiated families' markers must close the initial update, the silent one included
    Error: Not equal: expected: 0x2  actual: 0x1
    Messages: one counted EOR per negotiated family
--- FAIL: TestInitialSyncEORReachesTheSilentFamilyToo (0.00s)
FAIL	github.com/ze-software/ze/internal/component/bgp/reactor	0.723s
```

The silent-family test shows only TWO failures here against M1's three: its
`assert.Greater` "the ipv4 route UPDATE must precede the markers" PASSES under M2,
because the route and the ipv4 marker both went out. That absence is the
discrimination.

**The two tests fail DIFFERENTLY under M2, and that difference is the whole point.**
The all-silent test carries no static route, so M2 leaves its family list empty and
it counts 0 markers, the same shape M1 produced. The silent-family test carries one
ipv4 route, so M2 emits the ipv4 marker and drops the ipv6 one: `actual: 0x1`, with
"the route UPDATE precedes the markers" still PASSING. Only that second failure
discriminates the narrowing from a dead loop, which is why the fixture that puts a
route in one family and leaves the other silent had to exist.

**Restored, both green:**

```
--- PASS: TestInitialSyncEORCountedOncePerFamilyOnTheWire (0.00s)
--- PASS: TestInitialSyncEORReachesTheSilentFamilyToo (0.00s)
ok  	github.com/ze-software/ze/internal/component/bgp/reactor	1.586s
```

**What the functional test does NOT prove, measured:** under M2 the rebuilt daemon
left `test/encode/eor-silent-family.ci` GREEN across three consecutive runs. The
ipv6 marker still reaches the peer through `AnnounceEOR`
(`internal/component/bgp/reactor/reactor_api_forward.go`), whose suppression only
holds while the peer is draining its initial route sync. The `.ci` proves ze's
output; the unit test proves which producer made it. The `.ci` says so in its own
header rather than leaving the next reader to find out.

## The Relaxation Audit's Two Findings, Answered With Evidence

`python3 scripts/dev/audit-test-relaxation.py` (step 0 of `/ze-review`) exits 1 on
this change with `[WEAKENED] peer_initial_sync_test.go` and `[RELAXED]
peer_test.go`, both saying "RFC-TAGGED test changed without an approval token".

**No `rfc-test-change-approved:` token is written here, and the answer is not an
escalation either.** `plan/learned/1340-fixit-bgp-per-family-prefix-enforcement.md`
already ruled on this exact shape: the audit is FILE-scoped where the pre-write
hook is FUNCTION-scoped, both call the same `_rfc_tagged_change_err`, and editing
an untagged function in a file that carries a tag elsewhere passes the hook and is
reported by the audit. "That is a candidate finding, not a verdict, and the answer
is evidence in the review, never a self-written token." The evidence:

| File | The tagged requirements the audit names | Where they live | Where this change edits | Verdict |
|------|------------------------------------------|-----------------|-------------------------|---------|
| `internal/component/bgp/reactor/peer_initial_sync_test.go` | `RFC2545-3-1` .. `RFC2545-3-4` | the link-local next-hop tests near the end of the file | two hunks only: the import block, and a new block of test functions. +79 / -0 | No tagged assertion changed. The diff removes nothing |
| `internal/component/bgp/reactor/peer_test.go` | `RFC5549-4-1`, `RFC5549-4-4`, `RFC8950-4-1` | the extended-next-hop tests | one hunk: the three vacuous tests out, the 13-line `test-relax:` gravestone in. The assertion count drops by exactly 10, which is exactly their 6 + 1 + 3 | No tagged assertion changed. The drop is fully accounted for, and a comment carries none |

The delta of 10 is reproducible two ways, and only the delta is: `grep -c 'require\.\|assert\.'` over the file gives 126 at HEAD and 116 in the worktree, while the audit's own wider counter gives 129 and 119. Both drop by 10. Per test: `TestFamiliesSentTracking` 6, `TestFamiliesSentEmpty` 1, `TestFamiliesSentOnlyVPN` 3.

The finer gate agrees: `_rfc_tagged_change_err` scopes to the enclosing tagged
function (`.claude/hooks/pretool-writeedit.py` passes
`tag_scope=_enclosing_tagged_scope(...)`, while `run_audit` passes none), and it
passed every edit in this change as it was made.

**Do not read the audit's printed `reason:` line as this change's justification.**
It quotes the pre-existing MVPN relaxation further down `peer_test.go`, not the
gravestone added here. That mis-pairing is the positional-slice defect filed as
`plan/spec-fixit-relax-audit-reports-the-wrong-token.md`, still unfixed.

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| The `run_audit` positional-slice defect in `scripts/dev/audit-test-relaxation.py`, found by review pass 1 | deferred | `plan/spec-fixit-relax-audit-reports-the-wrong-token.md`. It blocks no goal of this spec, so it gets a spec and an owner decision, not a same-session fix |

The row stays `deferred`, NOT `done`. Filing work in a spec is not finishing it
(`ai/rules/planning.md`: "If the work is not in the tree, the row is not `done`"),
and `run_audit` is unfixed. A `done` row is never destination-checked again, which
is exactly how filed work stops being watched. It closes when that spec lands.

So the shard `plan/deferrals/fixit-vacuous-eor-family-tests.md` is LIVE and
outlives this spec by design; it is not residue and is not removed at closure. It
was created when pass 1 found the defect, which supersedes the Status table's
earlier "nothing deferred" note. Nothing in the implementation itself was deferred.

One FOREIGN row was resolved: `plan/deferrals/knowledge-0-umbrella.md` carried the
`TestFamiliesSentEmpty` row whose Destination is this spec, and that work IS in the
tree, so it moves to `done`. Its three sibling rows stay `deferred`, so that shard
is not residue either.

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/vacuous-eor-family-tests-3566d4e9-6b83-485f-964b-72a31d838f77.md`. The stem is the LEARNED summary's, not the spec file's: `spec_closure_stem` in `scripts/dev/commit_helper.py` derives what it looks for from `plan/learned/1368-vacuous-eor-family-tests.md` |
| `review_gate.py check` | clean, hashes match |
| Reviewer lenses used | 7 independent `ze-read` passes on Opus 5 through `/ze-review`: logic+wiring, hex arithmetic re-derived from RFC 4271 Section 4.3 and RFC 4724 Section 2, mutation discrimination re-run under `go build -overlay`, the test-deletion audit, and three sweeps of the closure record for claims a command would contradict |

**The code was clean after pass 1. Every one of the eleven findings that followed
was in this spec's own prose.** That is the headline, not a footnote: a spec about
a test that asserted on its own fill produced eleven statements that asserted on
their author's memory. The lesson is in
`plan/learned/1368-vacuous-eor-family-tests.md`.

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | M2's pasted evidence showed M1's numbers (`actual: 0x0`, real value `0x1`) | spec, Goal Validation | Re-read the captured log; added why the two tests fail differently under M2 |
| 2 | ISSUE | AC-4 and the guard's comment claimed repo scope the glob does not have | `peer_initial_sync_test.go`, spec | Narrowed both claims; guard stays package-scoped |
| 3 | ISSUE | The step-0 relaxation audit exits 1, unaddressed | spec | Answered with assertion-level evidence per `plan/learned/1340-fixit-bgp-per-family-prefix-enforcement.md`. No self-written token |
| 4 | ISSUE | Two evidence cells cited a grep that had never been run | spec | Replaced with `git grep`, the form that excludes `.claude/worktrees/` |
| 5 | ISSUE | The deferral row was closed on filing | `plan/deferrals/fixit-vacuous-eor-family-tests.md` | Set `deferred`; three contradicting spec cells corrected |
| 6 | ISSUE | Audit Summary arithmetic wrong | spec | Recounted by parsing the tables: 20 / 16 / 4 |
| 7 | ISSUE | "postdates this spec" inverted by seven days | spec, three places | Corrected against `git log -S` |
| 8 | ISSUE | "No `docs/` anchor names these files" | spec | Four exist; all named, the two live ones read and confirmed accurate |
| 9 | ISSUE | Doc checklist #9 claimed Section 2 newly proven | spec | Only the Section 4 and 4.2 requirements gained evidence |
| 10 | ISSUE | The `.ci` survey count reproduced under no filter | spec | Recorded the exact command; 18 then, 19 now |
| 11 | ISSUE | The condensation rule was claimed exhaustive and was not, three passes running | spec | Removed the exhaustiveness claim rather than extending it again. The narrower replacement is machine-checked against the three captured logs |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `test/encode/eor-silent-family.ci` | Yes | `-rw-r--r-- 1 thomas staff 3.1K Aug  9 01:13 test/encode/eor-silent-family.ci` |
| `internal/component/bgp/reactor/peer_initial_sync_test.go` | Yes | `git diff --numstat` reports it at 79/0, which no absent file can be. The glob inside `TestNoTestBuildsItsOwnFamiliesSentMap` proves only that SOME test file exists, so it is not the evidence here |
| `ai/rules/points/testing/test-sensitivity-ratchets-blocking/a-test-that-re-implements-the-logic-it-names.md` | Yes | `make ze-rules-render` exits 0, and it refuses a point the manifest does not list |
| `plan/deferrals/fixit-vacuous-eor-family-tests.md` | Yes | Created 2026-08-09. Its one row is LIVE (`deferred`), naming `plan/spec-fixit-relax-audit-reports-the-wrong-token.md`, so the shard outlives this spec and is not removed at closure |
| `plan/spec-fixit-relax-audit-reports-the-wrong-token.md` | Yes | The found-problem spec; `validate-spec.sh` accepts it with 3 warnings and 0 errors |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Both markers on the wire with no routes | `--- PASS: TestInitialSyncEORCountedOncePerFamilyOnTheWire (0.00s)` |
| AC-2 | Silent family still gets its marker | `--- PASS: TestInitialSyncEORReachesTheSilentFamilyToo (0.00s)` and `1.0s 13/56 PASS 13 eor-silent-family` |
| AC-3 | Mutation red, restored green | M1 and M2 blocks above, plus the restored block |
| AC-4 | The identifier is gone | `--- PASS: TestNoTestBuildsItsOwnFamiliesSentMap (0.01s)`, and it fails when a tripwire re-adds the identifier |
| AC-5 | Ledger carries the tags | the `RFC4724-4-1` row in `ai/RFC-REQUIREMENTS.md` now names `peer_initial_sync_test.go` beside `message/eor_test.go`; `make ze-rfc-check` exits 0 |
| AC-6 | The class is recorded | `make ze-rules-lint` exits 0 with the point rendered into `ai/rules/testing.md` |
| step-0 audit | `audit-test-relaxation.py` exits 1 | Answered with assertion-level evidence in "The Relaxation Audit's Two Findings", per the `plan/learned/1340-fixit-bgp-per-family-prefix-enforcement.md` ruling. No token written |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| A peer negotiating two families, route in one | `test/encode/eor-silent-family.ci` | Yes, read: its config declares both families and one ipv4 prefix, and its three `expect=bgp` lines are the route UPDATE then both markers |
| A peer negotiating two families, no routes | `test/plugin/eor.ci` | Yes, read: pre-existing, unchanged by this spec |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | Confirmed | `newInitialSyncPeer` in `peer_initial_sync_test.go` already drove `sendInitialRoutes` |
| A-2 | Confirmed | `make ze-test-pkg PKG=./internal/component/bgp/reactor` exits 0 after removal |
| A-3 | Broken | Recorded in the Mistake Log and in the new rule point |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| #9 RFC behavior newly proven | `ai/RFC-REQUIREMENTS.md` rows regenerated; `docs/features/rfc-status.md` untouched because no `{gap}` moved and `make ze-rfc-check` exits 0 | Yes |
| #10 Test infrastructure changed | the new point renders into `ai/rules/testing.md`; `make ze-rules-lint` and `make ze-doc-test` both exit 0 | Yes |
| #16 Changed source referenced by doc anchors | `make ze-doc-test` exits 0, and it is the gate that reads `source:` anchors | Yes |
| Every other row answered No | no production file changed, so no user-visible surface moved | Yes |

## Core Insight

A test that re-implements the logic it names is invisible to every gate this
repository has, because it DOES assert. The assert-nothing detector looks for a
missing oracle; this defect has an oracle, pointed at itself. The only reliable
tell is mechanical and human: **the test names a function it never calls.** Trying
to automate it fails on its own terms, because a local fixture asserted against is
also what every correct table-driven test looks like.
