# Spec: the two test ledgers become one shard per commit session

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-07 |

The original migration landed in `27a41cb32`.

<!-- Backfilled after work commissioned from a journal row. The original
     two-ledger migration landed in the commit above; the current contract
     and the evidence still owed are recorded below. -->

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`test/weakened.md` and `test/rfc-changed.md` were single shared paths whose own
contract said "this file is REPLACED for each commit; it never accumulates".
That works for one author and cannot work for five sessions committing from one
checkout. Following the contract is what causes the collision.

Evidence is five occurrences across four sessions and six days in
`plan/journal/concurrent-session-corruption.md`. The benign failure is a commit
refused over a peer's rows. The failure that matters is silent: session A writes
its rows, session B replaces the file, A commits, and A's commit lands carrying
B's approval record. For `test/rfc-changed.md` that publishes a forged-by-accident
record of an OWNER decision, which is the one thing that file exists to make
impossible. The rows are not restated here.

Goal: two writers cannot reach one path, and an author can see whose rows are in
the ledger WITHOUT preparing a commit.

The original migration covered both ledgers. The current RFC approval contract
has since moved to session-local approval files and `RFC-approved:` commit
trailers, as documented in `docs/architecture/testing/test-health.md`.
`rfcChangeProblems` in `internal/le/commit/rfcchange.go` reads only the current
session's approval file; `renderApprovalPrune` in `script.go` removes used rows
after the commit. Preserve that ownership and approval obligation. This spec
must not recreate `test/rfc-changed/` to match its original design.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/test-health.md` - the ledgers and what they record
  → Constraint: a row in `test/rfc-changed.md` with no owner answer behind it is
    a forgery, not a shortcut. That is what makes the silent case severe.
- [ ] `ai/rules/testing.md` - who writes a weakened row and when
  → Constraint: the row is written BEFORE the edit, so it exists in the tree
    while other sessions are also writing.
- [ ] `ai/rules/never-destroy-work.md`
  → Decision: clearing a peer's pending rows is banned at rung 1, so two correct
    sessions deadlock. The mechanism must remove the need to clear anything.

**Key insights:**
- The coordination remedy the earlier rows fell back on, message the peer and
  agree who commits first, does not scale to a sixteen-agent fleet: there is no
  peer to message when the colliding writers are siblings.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/test/weakened/shard.go` - `ShardPath`, `ShardSession`, `ReadShards`, `ForeignShardProblems`, `LandedRows`, `PruneLanded`
- [ ] `internal/le/test/weakened/ledger.go` - the row grammar every shard shares
- [ ] `internal/le/test/weakened/actions.go` - the `check` verb and its contract
- [ ] `internal/le/le/path/commitsession.go` - `CommitSession`, the eight-hex identity a shard is named after
- [ ] `internal/le/commit/rfcchange.go` - the gate that reads the rfc-changed ledger
- [ ] `internal/le/commit/actions.go` - the gate that reads the weakened ledger
- [ ] `docs/architecture/testing/test-health.md` - the published contract

**Behavior to preserve:**
- The row grammar is unchanged: one shard uses the same row format the flat file
  used, so no author relearns it.
- The gate still refuses a commit that weakens a test without a row.

**Current contract to preserve and prove:**
- Test weakenings use `test/weakened/<session>.md`, named by
  `lepath.CommitSession`. `ForeignShardProblems` in
  `internal/le/test/weakened/shard.go` refuses a commit naming a foreign
  weakened-test shard.
- RFC-tagged changes require the current session's owner approval through
  `rfcChangeProblems`; the commit carries its `RFC-approved:` trailer.
  The original `test/rfc-changed/<session>.md` layout is historical.
- A row whose text git already holds at HEAD no longer refuses anything: the gate
  proves it landed and drops it from the shard the commit carries. That retires
  the delete-the-last-commit's-rows chore three sessions performed by hand.
- `./le test weakened check` prints every shard and its rows, so an author sees
  the population without preparing a commit.

## Data Flow (MANDATORY)

### Entry Point
- An author weakening a test writes a row into their own shard. Entry format is
  a markdown table row, the grammar `ledger.go` defines.
- `./le commit create` reads the shards when it prepares a commit.

### Transformation Path
1. `lepath.CommitSession` answers the eight-hex namespace for this session.
2. `ShardPath` maps that to `test/weakened/<session>.md`.
3. `ReadShards` reads every shard under the directory, marking which is `Mine`.
4. `ForeignShardProblems` refuses a commit that names another session's shard.
5. `LandedRows` and `PruneLanded` drop a row git already holds at HEAD.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| session ↔ session | one shard per session; no path is reachable by two writers | Yes, by construction |
| author ↔ gate | `./le test weakened check` prints the weakened-test population | command exists; functional proof remains |
| working tree ↔ HEAD | `LandedRows` asks git what already landed | regression test exists; current execution remains owed |

### Integration Points
- `internal/le/le/path` (imported as `lepath`) - the identity, shared with the commit script and message.
- `internal/le/commit/rfcchange.go` and `actions.go` - the two gates.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | the weakening gate reads its session shard; the RFC gate reads `rfc.ApprovalPath(session)` |
| No unintended coupling | Yes | `lepath` holds the shared session identity |
| No duplicated functionality | Yes | weakened-test shards replace the flat ledger; RFC approval files and commit trailers supersede the second ledger |
| Zero-copy preserved where applicable | N-A | tooling path |
| Registration over hardcoding | Yes | a shard is discovered by reading the directory, never listed |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The eight-hex commit session is stable for the life of a session | `lepath.CommitSession` stores it under `tmp/` and reuses it | a session writes two shards | `commitsession.go`, read at the producer | confirmed |
| A-2 | Every existing row in the two flat files has an owning session that can claim it | the flat files held rows from landed commits and from pending ones | a row is orphaned at migration | audited from git 2026-10-07: at `27a41cb32^` the flat `test/weakened.md` held 5 rows, all added by the landed `18d7ffc8e0`, and `test/rfc-changed.md` held 3 rows, all added by the landed `5001022d13` (`git log -S` on each row). Every row the migration removed was already carried by a commit, so none lost its owner. A row pending only in some session's working tree at that moment is not observable from git | confirmed |
| A-3 | `LandedRows` correctly proves a row's text is at HEAD | current regression covers landed pruning and unlanded refusal | a landed row keeps refusing, or an unlanded one is dropped | `TestALandedRowIsDroppedRatherThanBlockingTheNextCommit` in `internal/le/commit/ledger_test.go`, run 2026-10-07: PASS | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A session invents the shard layout by hand and a second session overwrites it | already happened: `test/weakened/c7ef7dc3.md` was committed by `8c7f0a5bf2` and a second session overwrote its rows | landing the mechanism is the mitigation |
| R-2 | An abandoned session leaves a shard nobody claims | `check` prints a shard whose session is gone | the row is dropped once git holds it at HEAD; an unlanded orphan needs a human |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | commits are refused, or an owner-decision record is published under the wrong subject |
| How is it reverted? | The migration is recorded landed. Any reversal must preserve pending weakened rows and the current RFC approval trail; restoring the old shared ledgers would restore the collision |
| Who else touches this path? | every session that weakens a test or changes an RFC-tagged one, and the private-index spec, which edits `prepare.go` too |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le test weakened check` | → | `ReadShards` (`internal/le/test/weakened/shard.go`) | `TestCheckPrintsEveryShardAndItsRows` (`internal/le/test/weakened/parity_test.go`) |
| `./le commit create` naming a foreign shard | → | `ForeignShardProblems` (`shard.go`) | `internal/le/commit/ledger_test.go` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | two sessions each write a weakened row | each writes its own shard; neither can reach the other's path |
| AC-2 | a commit names another session's shard | `create` REFUSES, naming the shard and its session |
| AC-3 | a shard holds a row whose text git already has at HEAD | the gate proves it landed and drops it; the commit is not refused over it |
| AC-4 | an author runs `./le test weakened check` | every shard and its rows are printed, with this session's marked, and no commit is prepared |
| AC-5 | a commit weakens a test and its own shard holds the row | the commit is admitted, exactly as with the flat file |
| AC-6 | a commit weakens a test and NO shard holds the row | the commit is refused, exactly as with the flat file |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| shard reading and foreign-shard refusal | `internal/le/commit/ledger_test.go` | AC-2, AC-4 | landed `27a41cb32` |
| ledger audit over shards | `internal/le/test/weakened/audit_test.go` | AC-1, AC-5, AC-6 | landed `27a41cb32` |
| parity between the ledger reader and the gate | `internal/le/test/weakened/parity_test.go` | AC-5, AC-6 | landed `27a41cb32` |
| landed row pruning with an unlanded-row refusal control | `internal/le/commit/ledger_test.go`, `TestALandedRowIsDroppedRatherThanBlockingTheNextCommit` | AC-3 | PASS 2026-10-07 |
| `TestCheckPrintsEveryShardAndItsRows` | `internal/le/test/weakened/parity_test.go` | AC-4: a peer's shard and its rows are printed beside this session's, and only this session's is marked | added at closure. Green; red with `shardPopulation` skipping every shard that is not `Mine` |
| `TestAnotherSessionsApprovalAdmitsNothing` | `internal/le/commit/rfcchange_test.go` | RFC approval ownership under the superseding approval-file contract: session B's approval for the unit session A changed does not admit A's commit | added 2026-10-07. Green; red with `rfcChangeProblems` pointed at the foreign session's approval path |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| session name length | exactly 8 hex characters | 8 | 7, refused by `commitSessionPattern` | 9, refused |
| shards in a directory | 0..n | n | 0, which is an empty population and the state between commits | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `./le test weakened check` over a populated directory | `TestCheckMatchesProducerDiagnosticsAndExitCodes` (`internal/le/test/weakened/parity_test.go`) asserts the rendered line `holds 0 row(s), and is yours`; live run below | an author asks whose rows are in the ledger | PASS 2026-10-07. Live run over this checkout printed `test/weakened parses (57 session(s), 326 row(s))`, every shard with its rows, and `test/weakened/cf36fbca.md holds 0 row(s), and is yours` for this commit session. No commit was prepared |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| none | - | - | tooling, no wire-visible behavior | N-A |

## Files to Modify
- `internal/le/test/weakened/ledger.go`, `audit.go`, `actions.go`, `proposed.go`, `selftest.go`, `testweakened.go` - shard-aware reading and the `check` verb
- `internal/le/commit/rfcchange.go`, `actions.go`, `prepare.go` - the two gates
- `docs/architecture/testing/test-health.md`, `docs/features/test-health.md` - the contract
- `ai/rules/testing.md` and its two points under `ai/rules/points/testing/` - where an author is told to write the row

## Files to Create
- `internal/le/test/weakened/shard.go` - the shard model and its readers
- `internal/le/le/path/commitsession.go` - the eight-hex identity
- `internal/le/commit/ledger_test.go` - the gate's shard tests

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema | N-A | development tooling, no operator config |
| CLI commands/flags | Yes | `./le test weakened check`, registered through `leaction.Action` in `internal/le/test/weakened/actions.go` |
| Functional test for new RPC/API | N-A | no RPC |
| Doctor check for runtime dependencies | N-A | no new runtime dependency |
| Env var registration | N-A | none added |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 3 | CLI command added/changed? | Yes | `docs/architecture/testing/test-health.md` and `docs/contributing/testing.md` (`docs/features/test-health.md` is the suite-health dashboard and carries no ledger text, verified by grep at closure) |
| 10 | Test infrastructure changed? | Yes | same two pages, plus `docs/contributing/testing.md` |
| 16 | Any changed source file referenced by existing doc anchors? | Yes | `docs/architecture/testing/test-health.md` covers weakened shards and RFC approval trailers. `docs/features/ai-first.md` covers the commit namespace. `docs/contributing/rfc-implementation-guide.md` must retain the current approval-file and trailer contract; no RFC ledger is restored |
| 1, 2, 4-9, 11-15, 17 | - | No | no operator-facing surface changed |

## Implementation Steps

The original steps, kept as the landed migration record.

1. **Phase: Wiring (MANDATORY FIRST)** - `lepath.CommitSession` and `ShardPath`,
   with a failing test that two sessions resolve two paths.
2. **Phase: Reading** - `ReadShards` and `ForeignShardProblems`; the gates read
   shards instead of the flat file. DELETE the flat-file reader in the same step.
3. **Phase: Landed rows** - `LandedRows` and `PruneLanded`, so a row git holds at
   HEAD refuses nothing.
4. **Phase: Visibility** - the `check` verb prints the whole population.
5. **Phase: Migration and documentation** - move the live rows into their owners'
   shards, and reconcile the three pages and `ai/rules/testing.md`.
6. **Phase: Land it** - DONE, `27a41cb32` on 2026-09-06.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | account for both original ledgers: prove weakened-shard isolation and preserve the superseding RFC approval-file/trailer contract |
| Correctness | `LandedRows` proves the row is at HEAD, and does not merely find similar text |
| Naming | the shard's eight hex characters ARE the commit session's, not a second identity |
| Rule: `ai/rules/never-destroy-work.md` | nothing in the migration drops a row an author has not landed |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| the flat files are gone | `ls test/weakened.md test/rfc-changed.md` reports absence |
| the population is visible without committing | `./le test weakened check` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Path traversal | `ShardSession` refuses a name carrying `/`, and `commitSessionPattern` admits only eight lowercase hex characters |
| Forged approval | `ForeignShardProblems` is the guard that stops a commit publishing another session's owner-decision record |

### Failure Routing
| Failure | Route To |
|---------|----------|
| A landed row still refuses a commit | `LandedRows` is wrong; re-read the producer |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
- An unowned shared artifact in a five-writer checkout fails the same way
  whatever it holds. Give it an owner, or make its refresh a step the gate
  performs rather than a chore a reader notices.
- A gate that teaches its contract only by refusing has made a first offence
  mandatory. `check` exists so the contract is readable before the refusal.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| A directory of shards | key each ROW to its commit session inside one file | one file still has one writer at a time for the write itself; a directory removes the shared path entirely |
| Reuse the commit-session identity | a new per-ledger id | the script, the message and the verification-debt shard already carry it, so an author reads one namespace rather than two |
| Drop a row git holds at HEAD | keep the delete-the-last-commit's-rows chore | three sessions performed that chore by hand, and each performance was a chance to delete a peer's pending row |

## Known Limitations
- A shard belonging to a session that ended without committing is an orphan until
  its row lands. `check` shows it; nothing collects it.

## Checklist

### Pre-Spec Verification
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated, not library-only
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean

## Current Condition and What Remains

The September 6 record says the original product code, three unit-test files
and documentation edits landed as `27a41cb32`. The current weakening path still
uses shards; the RFC path now uses approval files and commit trailers.
`TestALandedRowIsDroppedRatherThanBlockingTheNextCommit` in
`internal/le/commit/ledger_test.go` now covers the formerly missing AC-3 case,
including refusal of an unlanded stale row. Source inspection establishes that
the test exists, but no current run is claimed here.

| Item | State |
|------|-------|
| Product code | `internal/le/test/weakened/shard.go` and `internal/le/le/path/commitsession.go` (new), plus the two gates. Landed `27a41cb32` |
| Prior art in the tree | `test/weakened/c7ef7dc3.md` was committed by `8c7f0a5bf2` (reachable from HEAD, count 1) by a session that invented this layout BY HAND, and a second session had already overwritten its rows there. That is the sixth occurrence, found while implementing the fix |
| Documentation | the three pages and `ai/rules/testing.md` landed in `27a41cb32` |
| Journal row | written, `plan/journal/concurrent-session-corruption.md`, fifth occurrence, marked FIXED 2026-09-06; the work it names landed the same day as `27a41cb32` |
| Historical proof record | September 6 recorded AC-1, AC-2, AC-4, AC-5 and AC-6 through `ledger_test.go`, `audit_test.go` and `parity_test.go`; this is not a new pass |
| Current evidence, 2026-10-07 | `./le job run label unit-pkg quiet command go test -count=1 ./internal/le/commit/... ./internal/le/test/weakened/...` exit 0. AC-1 and AC-2 by `TestTwoSessionsKeepTheirOwnLedgerRowsThroughCreate`, AC-3 by `TestALandedRowIsDroppedRatherThanBlockingTheNextCommit`, AC-4 by `TestCheckMatchesProducerDiagnosticsAndExitCodes` plus the live `./le test weakened check` run in the Functional Tests table, AC-5 by `TestAuditHonoursWeakeningRowOnlyInTheCommitThatCarriesIt`, AC-6 by `TestCreateStillRefusesAWeakeningItsOwnShardDoesNotName`. The package moved from `internal/le/testweakened` to `internal/le/test/weakened`, and `lepath` to `internal/le/le/path`, after this spec was written; closure corrected every path in this spec |
| Migration audit | A-2 confirmed from git, see the Assumptions table |
| RFC approval ownership | `TestAnotherSessionsApprovalAdmitsNothing` proves the superseding contract keeps the owner's approval with the session that recorded it, observed red under a mutation |
| Owed by the main thread | `./le verify worktree`, not run at closure: another session's in-flight BGP edits break the tree-wide build, so the closure commits record verification debt. Scoped `./le go lint run scope` over both packages is clean |
| Remains | nothing: closed by an independent `/ze-close` on 2026-10-07 |

## Implementation Summary

### What Was Implemented
- The migration itself landed in `27a41cb32` (2026-09-06): `ShardPath`, `ShardSession`, `ReadShards`, `ForeignShardProblems`, `LandedRows`, `PruneLanded` in `internal/le/test/weakened/shard.go`, called from `checkSourceGates` in `internal/le/commit/prepare.go` and from `Check` in `internal/le/test/weakened/testweakened.go`.
- `c14ec85d74` added `TestAnotherSessionsApprovalAdmitsNothing` for the RFC approval-file contract that superseded `test/rfc-changed.md`.
- Closure added `TestCheckPrintsEveryShardAndItsRows` for AC-4.

### Bugs Found/Fixed
- AC-4 had no discriminating test: the only `check` fixture held one empty shard of this session's, so a renderer that printed only the author's own shard stayed green. Fixed by `TestCheckPrintsEveryShardAndItsRows`, observed red with `shardPopulation` skipping non-`Mine` shards.

### Documentation Updates
- None at closure. `docs/architecture/testing/test-health.md` ("The shard is named after your commit session"), `docs/contributing/testing.md` (lines 240-273) and `ai/rules/testing.md` already state the shard contract and the `check` verb, and name `internal/le/test/weakened` and `internal/le/le/path.CommitSession`.
- `./le doc check verify` exit 1 on one finding only, the live help catalog, because `internal/component/bgp/wireu/aspath_slot.go` (another session's in-flight edit) does not build. Nothing in this spec's pages was flagged.

### Deviations from Plan
- The RFC ledger was not sharded: the approval-file and `RFC-approved:` trailer contract superseded it, as the Task section records.
- Package paths moved after the spec was written (`internal/le/testweakened` to `internal/le/test/weakened`, `lepath` to `internal/le/le/path`); closure corrected them here.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | AC-4 was recorded as demonstrated by a single-shard fixture plus a live run | the fixture could not see a peer's shard, so the regression it names had no test | closure review, reading `TestCheckMatchesProducerDiagnosticsAndExitCodes` against AC-4's wording | added `TestCheckPrintsEveryShardAndItsRows` |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| two writers cannot reach one path | Done | `internal/le/test/weakened/shard.go` `ShardPath`, `ForeignShardProblems` | |
| an author sees whose rows are in the ledger without preparing a commit | Done | `Check` with no paths in `testweakened.go`, `shardPopulation` in `shard.go` | |
| RFC approval ownership preserved under the superseding contract | Done | `rfcChangeProblems` in `internal/le/commit/rfcchange.go` reads `rfc.ApprovalPath(session)` | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestTwoSessionsKeepTheirOwnLedgerRowsThroughCreate` | |
| AC-2 | Done | `TestTwoSessionsKeepTheirOwnLedgerRowsThroughCreate`, last assertion | refusal text names the path and `session bbbbbbbb` |
| AC-3 | Done | `TestALandedRowIsDroppedRatherThanBlockingTheNextCommit` | with the unlanded-row control |
| AC-4 | Done | `TestCheckPrintsEveryShardAndItsRows`, `TestCheckMatchesProducerDiagnosticsAndExitCodes` | |
| AC-5 | Done | `TestAuditHonoursWeakeningRowOnlyInTheCommitThatCarriesIt`, `TestCreateStillRefusesAWeakeningItsOwnShardDoesNotName` (own-row admission) | |
| AC-6 | Done | `TestCreateStillRefusesAWeakeningItsOwnShardDoesNotName` | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| ledger, audit, parity tests | Done | `internal/le/commit/ledger_test.go`, `internal/le/test/weakened/audit_test.go`, `parity_test.go` | |
| `TestAnotherSessionsApprovalAdmitsNothing` | Done | `internal/le/commit/rfcchange_test.go` | |
| `TestCheckPrintsEveryShardAndItsRows` | Done | `internal/le/test/weakened/parity_test.go` | added at closure |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/le/test/weakened/shard.go` | Done | landed `27a41cb32` |
| `internal/le/le/path/commitsession.go` | Done | landed `27a41cb32`, since moved |
| `internal/le/commit/ledger_test.go` | Done | |

### Audit Summary
- **Total items:** 15
- **Done:** 15
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 2 (recorded in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| two writers cannot reach one path | functional, through `Create` | `TestTwoSessionsKeepTheirOwnLedgerRowsThroughCreate`: two sessions prepare commits from one checkout, each shard keeps only its own row, and a commit naming the peer's shard is refused |
| an author sees whose rows are in the ledger without preparing a commit | functional, through `Check` | `TestCheckPrintsEveryShardAndItsRows` (red under mutation), and the live `./le test weakened check` run recorded in Functional Tests (57 sessions, 326 rows) |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| none | every AC is demonstrated; the RFC ledger was superseded by design, not left undone | - |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/ledger-shards-per-commit-session-450bc92b-6ac1-4190-bd40-b427ecba17bf.md` |
| `./le spec review check` | clean (2 code files, hashes match) |
| Rounds | 2 |
| Reviewer lenses used | wiring and AC-to-test discrimination, security (path and session naming), logic of `PruneLanded` before later gates, Go style pass |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | AC-4 "every shard and its rows" had no test that could fail if a peer's shard were hidden | `internal/le/test/weakened/parity_test.go` | `TestCheckPrintsEveryShardAndItsRows`, red under the `shardPopulation` mutation, green restored |

NOTEs, not blocking: `ShardSession` admits a non-hex name, which only widens what `ForeignShardProblems` refuses; a file under `test/weakened/<dir>/` is not a shard and no gate reads it. `PruneLanded` runs before the coverage gate, and drops only rows `unmatchedProblems` already proved landed, so a later refusal loses nothing. Round 2 read the new test and the corrected spec: 0 BLOCKER, 0 ISSUE. Style pass over both changed test files: no `panic`, no unbounded loop, names state the value.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/le/test/weakened/shard.go`, `internal/le/le/path/commitsession.go`, `internal/le/commit/ledger_test.go` | Yes | `git ls-files` at closure lists all three |
| `test/weakened.md`, `test/rfc-changed.md` | Absent, as the deliverable requires | `ls` reports "No such file or directory" for both |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1..AC-6 | the named tests pass | `./le job run label unit-pkg quiet command go test -count=1 ./internal/le/commit/... ./internal/le/test/weakened/...` exit 0 after the closure fix: `ok internal/le/commit 40.439s`, `ok internal/le/test/weakened 1.669s` |
| AC-4 | the new test discriminates | mutation run exit 1: `population = code 0, text "... 0f1e2d3c.md holds 1 row(s), and is yours\n    \| TestMine \|\n"`; restored, green |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `./le test weakened check` | none; Go test over `Check`, the function the registered action calls | `TestCheckPrintsEveryShardAndItsRows` read at closure; live run recorded in Functional Tests |
| `./le commit create` naming a foreign shard | none; Go test over `Create` | `checkSourceGates` in `prepare.go` calls `ForeignShardProblems`; `TestTwoSessionsKeepTheirOwnLedgerRowsThroughCreate` drives `Create` |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `CommitSession` in `internal/le/le/path/commitsession.go` |
| A-2 | confirmed | git audit of `27a41cb32^`, see the Assumptions table |
| A-3 | confirmed | `TestALandedRowIsDroppedRatherThanBlockingTheNextCommit` PASS at closure |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| shard named after the commit session; `check` prints the population | `ShardPath`, `ReadShards`, `shardPopulation` in `shard.go`; `docs/architecture/testing/test-health.md` lines 140-156 | Yes |
| RFC approval in a session file, carried as a trailer | `rfcChangeProblems` reads `rfc.ApprovalPath(session)`; `docs/contributing/rfc-implementation-guide.md` line 726 | Yes |
| `docs/features/test-health.md` | `grep -n -i "ledger\|shard\|weaken"` finds only the known-failures line; no ledger claim to keep current | No update needed |
