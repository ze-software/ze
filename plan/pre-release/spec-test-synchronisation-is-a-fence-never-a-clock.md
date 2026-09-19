# Spec: test-synchronisation-is-a-fence-never-a-clock

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | testing |
| Depends | - |
| Phase | - |
| Handoff | verify |
| Updated | 2026-09-19 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**A test synchronises on something that TRAVELS THE SAME PATH AS THE WORK, never
on a duration. A check peer behaves like a peer: it holds its session open, and
a test that needs a close asks the DAEMON to close (owner rulings, 2026-09-18
and 2026-09-19).**

Three rulings, one behaviour:

- **A sleep is never the right answer**, and a bounded poll is a sleep wearing a
  loop: `Poll(ctx, 60, 100*time.Millisecond, ...)` encodes "six seconds ought to
  be enough", which is true on an idle host and false on a loaded one. It passes
  for the wrong reason and, when the guess runs out, reports a product defect
  that is not there.
- **Send a route through the software and wait for THAT.** The fence is the
  synchronisation point: when it arrives, everything queued ahead of it on the
  same path has been processed. There is no deadline to tune and no host-speed
  assumption, and a fence that never arrives is a loud failure.
- **Fix the root cause, never hide a race.** Where a test loses a race, why the
  race exists is a product question before it is a test question.

**The root cause this spec closes.** A check peer with no expectation left closes
its TCP connection the instant its last write lands. ze reads that as
session-down and withdraws the peer's routes from the adj-RIB-in, which is what
RFC 4271 owes. An observer reading that state then finds it gone. The peer is
the artifact: a real peer stays up. `option=linger:value=true` makes it behave
like one, and `(*Peer).completed` (`internal/test/peer/reject.go`) already
implements the hold behind that flag.

**Why the default is the fix, and 34 files were not.** This class has been paid
for at least nine times. `plan/journal/false-synchronization-claim.md` recorded
on 2026-08-15: "THE CLASS HAS NOW EARNED A FIX. Five rows here plus three more
fixtures share one mechanism ... The structural answer is for the runner to
REFUSE the shape, or default linger on, when a fixture pairs an observer plugin
with a check-mode peer, since the author who writes that pair never intends the
teardown." On 2026-09-18 a further 34 files were repaired one at a time, which is
the ninth payment on the same debt. The default costs one line and covers every
test not yet written.

**What a test that needs the close does instead.** Three files broke when the
hold was applied, and each ends only because its peer disconnects, which is
teardown used as a control signal. They ask the DAEMON to end the session:
fence on the work first (`updates-sent - eor-sent >= 2`, the idiom
`plugin_fixture_13.go` already uses), then dispatch `request shutdown`. TCP
ordering puts the route on the wire before the Cease on the same socket,
`holdSession` sees the remote close and returns, and the close that ends the
test is the daemon's, deliberate and observable.

**What the hold already exposed.** `bgp-capture-replay` asserts that ze closes
its capture file cleanly on shutdown. With the peer closing first, the
session-end defer ran for the wrong reason, so that AC had never been proven.
Holding the peer exercised the shutdown path for the first time and it produced
no terminator: **stop ze while a session is up and the operator's capture file
is handed over unterminated.** That is one product defect this change found by
refusing to let a test lie about why it passed, and it is the argument for the
default in one sentence.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/ci-format.md` - the `linger`, `tcp_connections` and `silent` rows
  → Constraint: linger's documented meaning is "the check peer never closes a connection itself", and the row already states the failure this spec generalises: "Without it a completed peer closes, which ze correctly treats as session-down; that withdraws the peer's routes and races any forwarding still in flight"
  → Decision: [fill during research] whether the row becomes the description of the DEFAULT, with an opt-out naming the tests that assert a peer-initiated close
- [ ] `plan/journal/false-synchronization-claim.md` - nine rows of this class
  → Constraint: the 2026-08-15 row names both candidate fixes, refuse-the-shape and default-on, and leaves the choice to the owner

**Key insights:** (minimal context to resume after compaction)
- The default is one line: `(*Peer).completed` returns success at once unless `p.config.Linger`.
- Flipping it changes what a peer IS for every suite, so validation is every suite, not one.
- A test that needs a close asks the daemon to close, and fences on the work first.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/test/peer/reject.go` - `completed` returns `Result{Success: true}` at once when `!p.config.Linger`; with it, `holdSession` answers KEEPALIVEs, re-checks every frame against the rejections, and returns when the REMOTE closes or the budget ends
- [ ] `internal/test/peer/peer.go` - every `p.completed(ctx, conn)` call site is guarded by `p.checker.Completed()`, so the hold is reached only with every expectation already matched; the accept goroutine's `defer c.Close()` drops the TCP today
- [ ] `internal/test/runner/runner_exec.go` - the only end for a peer test is `waitForPeers`, so "the peer process ended" IS the barrier and the close is how it ends
- [ ] `internal/component/bgp/plugins/adj_rib_in/rib.go` - `handleState` deletes the peer's routes on session-down, which is correct and stays

**Behavior to preserve:** (unless the user explicitly said to change it)
- ze withdrawing a peer's routes when its session goes down (RFC 4271)
- Every assertion any `.ci` makes today, including the `reject=bgp` patterns a lingering peer keeps re-checking
- A test that genuinely asserts a peer-initiated close, which MUST be able to say so

**Behavior to change:** (only what the user asked for)
- A check peer holds its session open by default; the flag becomes the opt-OUT
- `plugin-refresh`, `rfc7606-withdraw` and `bgp-capture-replay` end on a daemon close after a fence, not on their peer disconnecting

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `(*Peer).completed` (`internal/test/peer/reject.go`): every check peer in every suite, at the moment its expectations are met.

### Transformation Path
1. Expectations match, the peer holds the session, the observer reads live state that is still there, the scenario asks the daemon to stop, the daemon closes, `holdSession` returns, the peer process exits, `waitForPeers` returns.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Test peer ↔ daemon | the close moves from the peer to the daemon | No |

### Integration Points
- Every suite with a check peer: plugin, bgp, reload, editor, ipsec, isis, ospf and the wire suites

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | the hold already exists; only its default moves |
| No duplicated functionality (extends existing, does not recreate) | Yes | 34 files stop carrying an option the default provides |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A peer that holds weakens no assertion | every `completed` call site is guarded by `checker.Completed()`, and `holdSession` re-checks rejections | a test passes that should fail | the RFC 7606 re-audit reached exactly this conclusion for three verdicts on 2026-09-18 | confirmed for rfc7606-reset |
| A-2 | Few tests need a peer-initiated close | three of the 63 examined; only `plugin-reconnect` asserts a reconnect | the opt-out list is longer and each file needs reading | the full-suite run this spec requires | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A test elsewhere ends only because its peer closes | it times out | the failure is LOUD, never silent: the test hangs to its budget and names itself. Each one gets the daemon-close ending |
| R-2 | The hold masks a product defect the close was hiding | a test failing for a NEW reason | that is the point. `bgp-capture-replay` did exactly this and exposed the unterminated capture file |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Test peers hold sessions they should drop; a suite slows or hangs, loudly |
| How is it reverted? | One line |
| Who else touches this path? | every functional suite |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| a `.ci` with a check peer and no `option=linger` | → | `completed` holds | the 34 files of 3476bfc9d pass with their explicit option REMOVED |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | a check peer meets its last expectation | it holds the session open to teardown, answering KEEPALIVEs and re-checking rejections, with no `option=linger` in the file |
| AC-2 | a file that declares the opt-out | the peer closes as it does today, and `plugin-reconnect` still asserts its reconnect |
| AC-3 | `plugin-refresh`, `rfc7606-withdraw`, `bgp-capture-replay` | each fences on the work, then asks the daemon to close, and ends on that close |
| AC-4 | every functional suite | green, and the run names any test that ends only because its peer disconnected |
| AC-5 | the 34 files of 3476bfc9d | their explicit `option=linger` is removed and they still pass, because the default provides it |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | writes a new `.ci` pairing an observer with a check peer | the peer holds, the observer reads live state, nothing is timed | AC-1 |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCompletedHoldsByDefault`, `TestCompletedClosesWhenOptedOut` | `internal/test/peer/reject_test.go` | AC-1, AC-2 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| the hold budget | the file's own timeout | [fill during design] | [fill during design] | [fill during design] |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| every suite | `test/**` | AC-4 | |

## Files to Modify
- `internal/test/peer/reject.go` - `completed` holds unless the file opts out. Design doc: `docs/architecture/testing/ci-format.md`
- `internal/test/peer/expect.go` - the option becomes the opt-OUT
- `test/plugin/plugin-refresh.ci`, `test/plugin/rfc7606-withdraw.ci`, `test/plugin/bgp-capture-replay.ci` and their fixtures - fence, then daemon close
- `docs/architecture/testing/ci-format.md` - the `linger` row describes the default

## Files to Create
- none

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | test tooling |
| CLI commands/flags | N-A | none |
| Functional test for new RPC/API | Yes | every suite is the test |
| Prometheus counters/metrics | No | none |
| BGP family surface (new SAFI / capability / attribute) | N-A | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | test tooling only |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/ci-format.md`, the `linger` and `tcp_connections` rows |
| 12 | Internal architecture changed? | Yes | the same page |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the default flips, the opt-out parses, the unit tests fail first
   - Files: `internal/test/peer/reject.go`, `expect.go`, `reject_test.go`
2. **Phase: The three that need a close** -- fence, then daemon close
   - Verify: AC-3
3. **Phase: Every suite** -- run them all, name every test that ends only on a peer close, give each the same ending
   - Verify: AC-4
4. **Phase: Remove the now-redundant options** -- the 34 files
   - Verify: AC-5

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Correctness | no assertion is weakened; every test that needed a close asks the daemon for one |
| Rule: `ai/rules/no-layering.md` | the explicit options are removed once the default provides them, never left beside it |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| The default holds | AC-1's unit test |
| No suite regressed | AC-4 |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | none: test tooling, no external input |

### Failure Routing

| Failure | Route To |
|---------|----------|
| A suite times out | that test ends on a peer close; give it the daemon-close ending, and do NOT lengthen a budget |

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] No code snippets
- [ ] Current Behavior and Data Flow sections completed
- [ ] Every assumption has a Basis and a validation method

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated, not library-only
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional `.ci` tests for end-to-end behavior

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
