# Spec: rfc-approval-lives-in-the-commit

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-15 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Changing a test that carries an `RFC requirement:` tag needs the owner's
approval. Today that approval is a row in a tracked ledger shard,
`test/rfc-changed/<session>.md`: the edit-time hook refuses the edit until a
row names the test, `./le commit create` refuses the commit until the shard is
named and carries a row per changed unit, prunes the rows an earlier commit
landed, refuses a shard of another session, and the guide spends a section on
who may write a row and why a row without an answer behind it is a forgery.
The owner's verdict (2026-09-15): the refusal is the value, the ledger is bean
counting. Make it lighter.

Goal: keep the two refusals (the edit-time hook and the commit-time gate) and
delete the tracked ledger. The approval is written ONCE, when the owner gives
it, into a per-session file under `tmp/` that git never sees; the hook and the
gate read that file; and `./le commit create` carries each approval it used
into the commit message as a trailer line, so the permanent record is the
commit that changed the test, where a reader looks first.

| Today | After |
|-------|-------|
| `test/rfc-changed/<session>.md`, tracked, 17 shards in the tree | no directory; the 17 shards are deleted, history keeps them |
| the author writes a row by hand in the row grammar | `./le rfc approve unit <package>.<TestName> reason "<the owner's words>"` writes the row into `tmp/commit-rfc-approved-<session>.md` |
| the hook reads the session's tracked shard | the hook reads the session's `tmp/` file |
| `commit create` requires the shard in the file population, prunes landed rows, refuses foreign shards | `commit create` reads the `tmp/` file, requires one row per changed tagged unit, writes each used row as `RFC-approved: <package>.<TestName>: <reason>` in the message body, and drops the used rows from the file after the commit succeeds |
| the guide's "Who writes an owner-approval row" section | one paragraph: the command, the trailer, the refusal |

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/rfc-implementation-guide.md` - "Never change a tagged test to make it pass" and "Who writes an owner-approval row"
  → Decision: the author still cannot approve a change; the `reason` is the owner's words, and the hook and gate refuse without a row
  → Constraint: the trailer is the record; a row that no commit used never reaches git
- [ ] `docs/architecture/testing/test-health.md` - "one ledger shard per commit session"
  → Constraint: `test/weakened/<session>.md` (the author's own justification of a weakening) is NOT in scope and keeps its shard, its prune and the audit
- [ ] `docs/contributing/committing.md` - the keywords, `rfc-change-ok`, the message contract
  → Constraint: `rfc-change-ok "<reason>"` stays: it records a debt row when the owner has not answered yet
- [ ] `ai/rules/testing.md` - the point that a weakening row is not approval and names `test/rfc-changed/<session>.md`
  → Constraint: edit the point under `ai/rules/points/testing/`, regenerate with `./le rules render-update`, `condensed-update`, `index-update`, `lint`

**Key insights:**
- `testweakened.RFCChangedDir` is the one constant every reader of the ledger goes through; `ShardPath`, `sessionShard`, `LandedRows`, `PruneLanded`, `ForeignShardProblems` and `ReadShards` serve both ledgers, so the weakened ledger keeps them and the RFC side stops calling them.
- `rfcChangeProblems` (`internal/le/commit/rfcchange.go`) already parses rows with `ParseLedger` and matches them with `RowMatches`; only WHERE it reads and what it does after the match change.
- The message file `Message` (`internal/le/commit/input.go`) enforces the subject and wraps the body; the trailer lines are appended after the body, one per used row, and must not be wrapped.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/testweakened/shard.go` - `RFCChangedDir`, `ShardPath`, `sessionShard`, `LandedRows`, `PruneLanded`, `ForeignShardProblems`
- [ ] `internal/le/testweakened/proposed.go` - `Proposed` reads the RFC shard at edit time (`proposedLedger`) and refuses an edit to a tagged unit with no row
- [ ] `internal/le/commit/rfcchange.go` - `rfcChangeProblems` requires the shard in the population and a row per changed unit, computes `keep`
- [ ] `internal/le/commit/prepare.go` - calls `rfcChangeProblems`, prunes the shard, writes the debt row under `rfc-change-ok`
- [ ] `internal/le/commit/input.go` - `Message`
- [ ] `internal/le/rfc/tagged_scope_action.go` - the message that tells the author to record the approval in `test/rfc-changed`
- [ ] `internal/le/hookcheck/fixtures.go`, `fixture_catalog.go` - the nine `rfc-changed-ledger` fixtures
- [ ] `internal/le/doc/check/links.go` - exempts `test/rfc-changed/` from the link check
- [ ] `internal/le/rfc/register.go` - where a new `rfc approve` action registers

**Behavior to preserve:**
- The edit-time refusal: a Write or Edit that changes a tagged unit is refused until an approval names that unit.
- The commit-time refusal: a commit that changes a tagged unit is refused until an approval names that unit, or `rfc-change-ok` records the debt.
- `test/weakened/` and its audit, unchanged.

**Behavior to change:**
- Where the approval lives (a `tmp/` file per commit session), how it is written (a command), and where it is recorded for good (the commit message trailer).

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- The owner's approval, given in conversation; the author runs `./le rfc approve unit <package>.<TestName> reason "<words>"`.
- Format at entry: one row in the existing ledger row grammar (`| Test | Reason |`), in `tmp/commit-rfc-approved-<session>.md`, where `<session>` is `./le commit session`'s eight-hex namespace.

### Transformation Path
1. `./le rfc approve` appends the row to the session file (creating it).
2. The pretool hook (`Proposed`) reads that file instead of the shard; an edit to a tagged unit with no row is refused with the command to run.
3. `./le commit create` (`rfcChangeProblems`) reads that file; a changed tagged unit with no row is refused unless `rfc-change-ok` is given; each used row becomes one `RFC-approved:` trailer line in the message file.
4. The generated commit script, after the commit succeeds, drops the used rows from the session file (the same place it prunes the weakened shard today).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Hook to session file | `tmp/commit-rfc-approved-<session>.md` | No |
| Commit command to git | the trailer lines in the message file | No |

### Integration Points
- `rfcChangeProblems` - reads the session file; returns the used rows for the trailer and the prune.
- `Message` - gains the trailer lines after the body.
- `Proposed` - reads the session file.
- `./le rfc approve unit` - new action beside the `rfc` namespace's others.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound | No | |
| Registration over hardcoding, inbound | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The hook and the commit command resolve the same eight-hex commit session for one harness session | `./le commit session`, `lepath/commitsession.go` | the hook reads a file the commit never writes | `TestApproveHookAndCommitReadOneFile` | confirmed: both call `lepath.CommitSession(root, "")` and derive `rfc.ApprovalPath(session)` |
| A-2 | A trailer line survives the message wrap unchanged | `Message` in `internal/le/commit/input.go` | a long reason is wrapped and the record is cut | `TestMessageKeepsTrailerLinesWhole` | confirmed |
| A-3 | No gate other than `rfcChangeProblems` and `Proposed` reads `test/rfc-changed/` | grep `RFCChangedDir` and `rfc-changed` | a reader keeps looking for a directory that is gone | `grep -rn "rfc-changed\|RFCChangedDir" internal cmd pkg docs ai .claude` empty | confirmed: the grep found the link-check exemption, the hookcheck fixtures, six comments and five pages beside the two gates; all edited. Three rows in `test/weakened/` cite the old path as history and stay |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | An approval given for one commit is reused by a later one because the prune failed | the same trailer appears in two commits | the prune runs in the script after `git commit` succeeds, and a row used by a landed commit (found by the trailer in `git log`) is dropped even when the file still holds it |
| R-2 | The two open debt rows of session `1bfe298a` ("owner approval owed") can no longer be answered by a ledger row | `./le commit debt-status` | `./le commit debt-discharge kind owner` is the route for a row naming an act a person performs; the guide says so |
| R-3 | A `tmp/` file is lost with the session and the approval with it | the hook refuses again after a restart | the command is one line to re-run; the record that matters is in git |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A tagged test could be changed without an approval, or a commit refused for want of a file. No product code, no wire |
| How is it reverted? | single commit revert |
| Who else touches this path? | `test/weakened/` shares the row grammar and the shard helpers |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le rfc approve unit pkg.TestX reason "..."` | → | the `rfc approve` action writes the session file | `TestApproveWritesTheSessionRow` in `internal/le/rfc` |
| The pretool hook on an edit to a tagged unit | → | `Proposed` reads the session file | hookcheck fixtures `rfc-approval-missing-refuses-the-edit`, `rfc-approval-row-opens-the-gate` |
| `./le commit create` with a changed tagged unit | → | `rfcChangeProblems` reads the session file, `Message` carries the trailer | `TestCommitCarriesTheApprovalTrailer`, `TestCommitRefusesAChangedTaggedUnitWithNoApproval` in `internal/le/commit` |
| The commit script after a success | → | the prune of used rows | `TestCommitDropsTheUsedApprovalRows` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `./le rfc approve unit <package>.<Test> reason "<words>"` | one row in `tmp/commit-rfc-approved-<session>.md`; a second call for the same unit replaces the reason; a missing reason is refused |
| AC-2 | An edit to a tagged unit with no row in the session file | the hook refuses and names the command to run; with a row, the edit lands |
| AC-3 | A commit changing a tagged unit with no row and no `rfc-change-ok` | refused, naming the unit and the command; with a row, the commit succeeds and its message carries `RFC-approved: <package>.<Test>: <reason>` |
| AC-4 | The commit succeeded | the used rows are gone from the session file; an unused row stays |
| AC-5 | The tree | `test/rfc-changed/` does not exist; no Go, doc, rule, hook fixture or link-check exemption names it; the nine `rfc-changed-ledger` fixtures are replaced by fixtures for the session file |
| AC-6 | `test/weakened/` | unchanged behavior: its shard, audit and prune pass the same tests as before |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Asks Thomas, gets a yes, runs `./le rfc approve`, edits the tagged test, commits | approve -> session file -> hook -> commit gate -> trailer -> prune | `TestCommitCarriesTheApprovalTrailer` over a real temporary repository, plus the two hookcheck fixtures |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestApproveWritesTheSessionRow` | `internal/le/rfc/approve_test.go` | AC-1 | |
| `TestApproveHookAndCommitReadOneFile` | `internal/le/testweakened/proposed_test.go` | A-1, AC-2 | |
| `TestCommitCarriesTheApprovalTrailer` | `internal/le/commit/rfcchange_test.go` | AC-3 | |
| `TestCommitRefusesAChangedTaggedUnitWithNoApproval` | `internal/le/commit/rfcchange_test.go` | AC-3 | |
| `TestCommitDropsTheUsedApprovalRows` | `internal/le/commit/rfcchange_test.go` | AC-4, R-1 | |
| `TestMessageKeepsTrailerLinesWhole` | `internal/le/commit/input_test.go` | A-2 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| none | N-A | N-A | N-A | N-A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rfc-approval-missing-refuses-the-edit`, `rfc-approval-row-opens-the-gate`, `rfc-approval-row-for-another-unit-buys-nothing` | `internal/le/hookcheck/fixture_catalog.go` | the hook's two answers | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | - | - | tooling | |

## Files to Modify
- `internal/le/testweakened/shard.go`, `proposed.go` - the RFC side reads the session file; `RFCChangedDir` goes
- `internal/le/commit/rfcchange.go`, `prepare.go`, `input.go`, `script.go` - the session file, the trailer, the prune after success
- `internal/le/rfc/register.go` and a new `approve.go` - the `rfc approve unit` action
- `internal/le/rfc/tagged_scope_action.go` - the message names the command
- `internal/le/hookcheck/fixtures.go`, `fixture_catalog.go` - fixtures for the session file
- `internal/le/doc/check/links.go` - the exemption goes
- `docs/contributing/rfc-implementation-guide.md`, `docs/contributing/committing.md`, `docs/architecture/testing/test-health.md`, `.claude/hooks/README.md` - the command, the trailer, the refusal
- `ai/rules/points/testing/` - the point that names the ledger; regenerate `ai/rules/testing.md`

## Files to Create
- `internal/le/rfc/approve.go`, `approve_test.go`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | tooling |
| YANG validation constraints | N-A | tooling |
| YANG custom validators | N-A | tooling |
| CLI commands/flags | Yes | `./le rfc approve unit`, registered in `internal/le/rfc/register.go` |
| CLI grammar (keyword before value) | Yes | `unit <name> reason <text>` |
| Editor autocomplete | N-A | `./le` action |
| Functional test for new RPC/API | Yes | the hookcheck fixtures |
| Pipe completeness | N-A | `./le` action |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | N-A | none |
| Prometheus counters/metrics | N-A | none |
| BGP family surface | N-A | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | developer tooling |
| 2 | Config syntax changed? | No | none |
| 3 | CLI command added/changed? | Yes | `docs/contributing/rfc-implementation-guide.md`, `docs/contributing/committing.md` |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | No | none |
| 7 | Wire format changed? | No | none |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | No | none |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/test-health.md`, `.claude/hooks/README.md` |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | No | none |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | the `// Design:` headers of `shard.go` (`test-health.md`) and `rfcchange.go` (`rfc-implementation-guide.md`) name the two pages above; `docs/architecture/core-design.md` is declared by a file in the list (`internal/le/rfc/register.go` or `links.go`) and is unaffected: it describes the daemon's architecture, not the ledger |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | the guide's row example becomes the command |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the six unit tests and the three fixtures exist and fail
2. **Phase: the session file** -- `rfc approve unit`, `Proposed` and `rfcChangeProblems` read it
3. **Phase: the trailer and the prune** -- `Message`, the script step after success
4. **Phase: delete the ledger** -- `test/rfc-changed/` (17 tracked shards and the untracked `1bfe298a.md`), the fixtures, the exemption, `RFCChangedDir`; the pages and the rule point

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | the story's chain runs end to end in one test over a real temporary repository |
| Correctness | the hook and the gate refuse without a row; a used row is dropped; `test/weakened/` unchanged |
| Naming | the trailer key is `RFC-approved:`; the file is `tmp/commit-rfc-approved-<session>.md` |
| Data flow | one reader of the session file shared by the hook and the gate |
| Rule: `ai/rules/no-layering.md` | the shard route is deleted, not kept beside the new one |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `./le rfc approve unit` | `./le rfc approve` prints its usage |
| the trailer in a commit | `git log -1 --format=%B` of the test repository shows `RFC-approved:` |
| no ledger | `ls test/rfc-changed` fails; the grep in A-3 is empty |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | the unit name is `<package>.<TestName>` and nothing else; the reason carries no newline, so a trailer stays one line |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The approval is written once by a command into a `tmp/` file | keep a tracked ledger but drop the shards | a tracked file needs a prune, an audit and a foreign-shard rule; a `tmp/` file needs none, and git holds the record where the change is |
| The record is a commit trailer | the commit body prose | a trailer is one line per unit a script can read back; prose is not |
| `rfc-change-ok` stays | fold it into the trailer | it records a debt when the owner has not answered, which is a different fact |

## Known Limitations
- The two open debt rows of session `1bfe298a` are answered by `./le commit debt-discharge kind owner`, an owner attestation, not by this spec.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] **Commit A:** code + tests + docs + spec
- [ ] **Commit B:** remove the spec

## Implementation Notes

Implemented 2026-09-15 in one phase agent (all four phases). Scoped package
runs: `go test -tags "ze_core ze_le"` under `./le job run`.

| AC | Producer | Test | RED | GREEN |
|----|----------|------|-----|-------|
| AC-1 | `rfc.Approve`, `approveAnswer`, the `approve` row of `internal/le/rfc/actions.go` | `TestApproveWritesTheSessionRow` (`internal/le/rfc/approve_test.go`) | `FAIL github.com/ze-software/ze/internal/le/rfc [build failed]` (job `rfc-red`) | `ok` in job `rfc-final` (the package's two remaining reds are another session's, below) |
| AC-2 | `testweakened.Proposed` -> `proposedApprovals` (`internal/le/testweakened/proposed.go`) | `TestApproveHookAndCommitReadOneFile`, `TestProposedRFCChangeRequiresOwnerLedgerBeforeWeakeningLedger`; hookcheck category `rfc-approval` (`rfcApprovalTree`) | `FAIL .../internal/le/testweakened [build failed]` (job `rfc-red`) | `ok .../testweakened 1.559s` (job `pkg-green`); `fixture-rfc-approval` allow and refuse probes pass (job `hookcheck-green3`) |
| AC-3 | `rfcChangeProblems` (`internal/le/commit/rfcchange.go`), `Message` trailers (`input.go`), `Create` (`prepare.go`) | `TestCommitRefusesAChangedTaggedUnitWithNoApproval`, `TestCommitCarriesTheApprovalTrailer`, `TestMessageKeepsTrailerLinesWhole` | `FAIL .../internal/le/commit [build failed]` (job `rfc-red`) | `ok` (job `commit-green`, job `final-fast`) |
| AC-4, R-1 | `renderApprovalPrune` (`internal/le/commit/script.go`), `trailerLanded` (`rfcchange.go`) | `TestCommitDropsTheUsedApprovalRows` | as AC-3 | as AC-3 |
| AC-5 | `RFCChangedDir` deleted from `shard.go`; exemption removed from `internal/le/doc/check/links.go`; fixtures `rfc-approval-*` replace the ten `rfc-changed-*` rows | `grep -rn "rfc-changed\|RFCChangedDir" internal cmd pkg docs ai .claude test` answers only three historical rows in `test/weakened/` | - | `ok .../doc/check` (job `final-fast`) |
| AC-6 | `test/weakened/` path untouched: `ShardPath`, `LandedRows`, `PruneLanded`, `ForeignShardProblems` keep their weakened callers | the existing `internal/le/testweakened` and `internal/le/commit` suites | - | `ok` (jobs `pkg-green`, `final-fast`) |

→ Decision: the session-file writer lives in package `rfc` (`approve.go`), because `testweakened` imports `rfc` and the reverse import would cycle; the readers parse it with `testweakened.ParseLedger` and match with `RowMatches`, so the row grammar is declared once.
→ Decision: the prune is a `grep -v -x -F` step the generated script runs after `git commit` under `set -e` (`renderApprovalPrune`), so a row leaves the file only once git holds its trailer; `grep`'s exit 1 (nothing left to print) is admitted, any other failure stops the script.
→ Decision: a row whose trailer `git log --fixed-strings --grep` already finds is dead: it claims no change and is added to the drop list of the next successful commit (R-1).
→ Decision: an unused row is not a problem and stays in the file (AC-4); the prepare-time prune of the weakened shard is unchanged.
→ Decision: the ten `rfc-changed-*` hookcheck rows (the spec counted nine) became three `rfc-approval-*` rows; the five ratchet digests and the four population counts in `internal/le/hookcheck` were recomputed from the parity test's own output.
→ Decision: the native RFC fixture digest (`internal/le/rfc/native_fixture_test.go`) was NOT updated: it seals HEAD's blobs, was already red at HEAD `13f98cb7ef` (another session's rfc(5082) commits), and goes stale again at commit A; the committing session recomputes it. `TestSupportedRowsHaveDerivableScope` (46 rows in `rfc/short`, want 45) is red for the same reason. The hookcheck `yang-description` probe is red too; its producer `writeYangDescription` (`internal/le/hookruntime/writeedit.go`) and its fixture rows are untouched by this diff, so the red is not this spec's (unverified whether it is red at HEAD).
→ Decision (parent): that red WAS this session's, from the morning's swap commit b57ec4ab6b: the hook caps the `ze:help` now, and the self-check's refusal probe still wrote an over-cap `description`, which the gate rightly allows. `yangProbeAllow` and `yangProbeRefuse` (`internal/le/hookcheck/fixtures.go`) now write a `ze:help`, the `help-restates-description` fixture label reads `description-restates-short-help`, and the three ratchet digests are recomputed; the package is green (`scratch/hookcheck-yang3.log`).

## Review Gate

### Round 1 (2026-09-15, independent Opus 5 reader, whole uncommitted diff at 13f98cb7ef)

| # | Sev | File / symbol | What is wrong | What settles it |
|---|-----|---------------|---------------|-----------------|
| 1 | ISSUE | `internal/le/commit/input.go` `Message`, `wrapBody` | A body line the author types beginning `RFC-approved:` is wrapped into the commit and reads as the owner's record. It opens no gate (the route still needs `rfc-change-ok`, which lands a debt row), but it forges the permanent record the spec makes authoritative | `Message` refuses a body line that starts with the trailer key; one test in `input_test.go` |
| 2 | ISSUE | `internal/le/commit/rfcchange.go` `trailerLanded` and the refusal text in `rfcChangeProblems` | `git log --fixed-strings --grep` matches a substring: a landed trailer whose reason extends a new row's reason marks the new row landed, and the refusal then says no approval names the unit while the file holds one | Compare the whole trailer line against `%B` of the commits; name the landed commit in the refusal; extend `TestCommitDropsTheUsedApprovalRows` |
| 3 | ISSUE | `internal/le/hookcheck/fixtures.go`, two comments near `probeLedgerTree` | Still say "the RFC-changed ledger" | Reword the two comments |
| 4 | NOTE | `internal/le/hookcheck/fixtures.go` `probeLedgerRows`, `rfcApprovalTree` | The three fixtures write a bare `\| TestLedgered \|` row by hand, a form `./le rfc approve` never emits | Optional: write the tree's row through `rfc.Approve` |
| 5 | NOTE | `rfcchange.go` gate against `proposed.go` hook | The hook honours a path-scoped row, the gate does not; pre-existing, fail-closed at the gate | None owed |
| 6 | NOTE | `test/rfc-changed/1bfe298a.md` | Another session's untracked file deleted as the spec's step 4 plans; R-2 names the discharge route | The parent confirms |

AC verdicts: AC-1, AC-2, AC-3, AC-4, AC-6 proven; AC-5 weak (#3). Round 1 does not end clean: 3 ISSUE.

### Round 2 (scope: the fixes to #1, #2, #3 and their sibling call sites)

#### Round 2 evidence

Fix agent, 2026-09-15. Scoped runs under `./le job run`, logs in `scratch/round2-*.log`.

| # | File / symbol | Test | RED | GREEN |
|---|---------------|------|-----|-------|
| 1 | `internal/le/commit/input.go` `Message`, `refuseHandWrittenTrailer` (subject and every body line; the key inside a line stays prose) | `TestMessageRefusesAHandWrittenApprovalTrailer` (`input_test.go`) | `a hand-written approval trailer in the body was accepted` (job `round2-red-1`) | `ok .../internal/le/commit 38.833s` (job `round2-green-3`) |
| 2 | `internal/le/commit/rfcchange.go` `trailerLanded` (`--grep` narrows the candidates, the verdict is `TrimSpace(line) == trailer` over `%B`, answers `<short hash> <subject>`); `rfcChangeProblems` names that commit in the refusal | `TestCommitDropsTheUsedApprovalRows` (`rfcchange_test.go`): the R-1 refusal must name the landed commit; a landed trailer whose reason EXTENDS the new row's reason (the new trailer is its prefix) must not read as landed | `the refusal does not name "76aef73"` (job `round2-red-1`); with the verdict broken back to a substring match: `a landed trailer extending the row's reason read the row as landed` (job `round2-red-2`, break restored) | as #1 |
| 3 | `internal/le/hookcheck/fixtures.go`, comments above `probeLedgerRows` and `rfcGuardTree` | `grep -rn "rfc-changed\|RFC-changed\|RFCChanged" internal cmd pkg docs ai .claude test` answers the three `test/weakened/` history rows only | - | `ok .../internal/le/hookcheck 37.147s` (job `round2-hookcheck-pkg`, no digest changed) |

Pages carried with the code: `docs/contributing/committing.md` (the refused hand-written trailer), `docs/contributing/rfc-implementation-guide.md` (the whole-line match and the named commit). `gofmt -l` and `go vet` over both packages are clean (job `round2-vet`); `./le verify lint run` is owed by the main thread.

#### Round 2 findings (independent Opus 5 reader, a third context)

| # | Sev | File / symbol | What is wrong | What settles it |
|---|-----|---------------|---------------|-----------------|
| 7 | ISSUE | `internal/le/commit/input.go` `wrapBody` | The refusal ran on the INPUT line before the wrap; a key past column 72 lands at the start of the emitted line, where `trailerLanded` reads it as the record | Run the refusal over the emitted lines; one more case in `TestMessageRefusesAHandWrittenApprovalTrailer` |
| 8 | NOTE | `internal/le/commit/rfcchange.go` `trailerLanded` | A body holding a raw `\x1e` byte hides a trailer after it (false negative, fail-closed at the gate) | None owed now |

Round 2 does not end clean: 1 ISSUE (#7).

### Round 3 (scope: the fix to #7)

`wrapBody` now runs `refuseHandWrittenTrailer` over the lines it emits, after the wrap; the input-line check is gone. RED with the refusal moved back to the input lines, in `TestMessageRefusesAHandWrittenApprovalTrailer`: "an approval trailer the wrap moved to the start of a line was accepted" (`scratch/commit-wrap-red2.log`); GREEN `ok internal/le/commit` over the whole package (`scratch/commit-wrap-green.log`).

Round 3 (independent Opus 5 reader, a fourth context) read `Message`, `wrapBody`, `refuseHandWrittenTrailer` and `trailerLanded`: the refusal covers the subject and every emitted line, is a strict superset of what `trailerLanded` accepts, the input-line check is gone, and the new case discriminates. Round 3 ends clean: 0 BLOCKER, 0 ISSUE.

---

## Implementation Summary

### What Was Implemented
- `./le rfc approve unit <package>.<TestName> reason "<words>"` (`rfc.Approve`, `approveAnswer`, `internal/le/rfc/approve.go`; the `approve` row of `internal/le/rfc/actions.go`) writes one row into `tmp/commit-rfc-approved-<session>.md` (`rfc.ApprovalPath`), replacing a row that names the same unit; a unit that is not `<package>.<TestName>`, an empty reason and a reason carrying a newline or `|` are refused.
- The edit-time hook reads that file: `testweakened.Proposed` -> `proposedApprovals` (`internal/le/testweakened/proposed.go`); an absent file is a file with no row, and the refusal prints the command (`rfc.ApproveCommand`).
- The commit gate reads it: `rfcChangeProblems` (`internal/le/commit/rfcchange.go`) returns `RFCApprovals{Trailers, Dropped}`; `Message` (`input.go`) appends one unwrapped `RFC-approved: <unit>: <reason>` line per used row and refuses a subject or an EMITTED body line that starts with the key (`refuseHandWrittenTrailer`, run inside `wrapBody` after the wrap); `renderApprovalPrune` (`script.go`) emits the `grep -v -x -F` step after `git commit` under `set -e`.
- `trailerLanded` (`rfcchange.go`) compares each `%B` line of the `git log --fixed-strings --grep` candidates whole against the trailer, names the carrier `<short hash> <subject>`, and a landed row joins `Dropped` and opens nothing.
- The ledger is gone: 16 tracked shards under `test/rfc-changed/` deleted (the untracked `1bfe298a.md` with them), `testweakened.RFCChangedDir` deleted, `ForeignShardProblems` reads `WeakenedDir` only, the `links.go` exemption removed, ten `rfc-changed-*` hookcheck fixtures replaced by three `rfc-approval-*` (`rfcApprovalTree`, `internal/le/hookcheck/fixtures.go`).
- Pages and rules: the guide's section is "The owner's approval lives in the commit"; `committing.md`, `test-health.md`, `rfc-conformance-gates.md`, `functional-tests.md`, `.claude/hooks/README.md`, `ai/skills/ze-rfc.md`, the `never-edit-an-rfc-tagged-test-to-match-the-code` point and its rendered `ai/rules/testing.md`; six code comments that named the old path.
- Rode along (parent session, same diff): the hook self-check's YANG probes `yangProbeAllow` and `yangProbeRefuse` write a `ze:help` and the `description-restates-short-help` label replaces `help-restates-description`, because the swap commit b57ec4ab6b moved the cap from `description` to `ze:help` and left the self-check red.

### Bugs Found/Fixed
- Round 1 #1: a hand-written `RFC-approved:` body line forged the record. `refuseHandWrittenTrailer`; `TestMessageRefusesAHandWrittenApprovalTrailer`.
- Round 1 #2: the `git log --grep` substring match read a landed trailer whose reason EXTENDS a new row's as that row landed. Whole-line verdict in `trailerLanded`; `TestCommitDropsTheUsedApprovalRows` extended (job `round2-red-2` was the forced red).
- Round 2 #7: the forgery guard ran over the INPUT line, and a key the wrap moved to column 0 reached the commit. The guard runs over the emitted lines; one more case in the same test (`scratch/commit-wrap-red2.log` red, `commit-wrap-green.log` green).

### Documentation Updates
- `docs/contributing/rfc-implementation-guide.md` "The owner's approval lives in the commit" (anchors `internal/le/testweakened/proposed.go`, `ai/skills/ze-rfc.md` kept below it); `docs/contributing/committing.md` (the `Message` paragraph, the `rfcChangeProblems` paragraph); `docs/architecture/testing/test-health.md`; `docs/contributing/rfc-conformance-gates.md`; `docs/functional-tests.md`; `.claude/hooks/README.md` gate table row `rfc-approval`.
- `./le doc check verify`: runs inside `./le verify worktree` (closure step 3, result in the closure report); `./le doc check` unit package `ok` (job `final-fast`).

### Deviations from Plan
- The spec counted nine `rfc-changed-ledger` fixtures; there were ten. They became three `rfc-approval` fixtures.
- The forgery refusal (`refuseHandWrittenTrailer`) and the whole-line landed match were not in the plan; both came out of the Review Gate.
- The `approve` action registers in `internal/le/rfc/actions.go`, where the area's action table lives, not in `register.go`.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | `Message` wrapped any body line into the commit, so an author could type the trailer key at the start of a body line and land the owner's record by hand (round 1 #1) | the trailer is the owner's record and only `create` may write it, from the session file | round 1 reader | `refuseHandWrittenTrailer` over the subject and the body |
| approach | `trailerLanded` used `git log --grep`'s substring match as the verdict (round 1 #2) | a landed trailer whose reason extends the new row's reason is a different approval | round 1 reader | the verdict is `TrimSpace(line) == trailer` over `%B`; the refusal names the carrier commit |
| approach | the forgery guard ran on the INPUT body line, before `wrapBody` (round 2 #7) | the commit carries the EMITTED lines, and a key past column 72 lands at the start of one | round 2 reader | the guard runs inside `wrapBody` over the emitted lines; row in `plan/journal/check-cannot-see-the-change-it-looks-for.md` |
| escalation | the morning's swap commit b57ec4ab6b moved the YANG length cap from `description` to `ze:help` and left the hook self-check's `yang-description` probes writing an over-cap `description` the gate now rightly allows; this spec's implementer read that red as foreign | the red was this session's | the parent read the producer `writeYangDescription` against the probes | `yangProbeAllow`/`yangProbeRefuse` write a `ze:help`; the label is `description-restates-short-help`; three ratchet digests recomputed; rides on commit A |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| The two refusals stay | Done | `proposedApprovals` (`internal/le/testweakened/proposed.go`), `rfcChangeProblems` (`internal/le/commit/rfcchange.go`) | fixtures `rfc-approval-*`; `TestCommitRefusesAChangedTaggedUnitWithNoApproval` |
| The tracked ledger is deleted | Done | 16 `test/rfc-changed/*.md` removed; `RFCChangedDir` gone from `shard.go` | `ls test/rfc-changed` fails |
| The approval is written once by a command into a `tmp/` file | Done | `rfc.Approve` (`internal/le/rfc/approve.go`) | `TestApproveWritesTheSessionRow` |
| The hook and the gate read that file | Done | `proposedApprovals`, `rfcChangeProblems`, both through `rfc.ApprovalPath(session)` | `TestApproveHookAndCommitReadOneFile` |
| The commit carries each used approval as a trailer | Done | `Message` (`input.go`), `Create` (`prepare.go`) | `TestCommitCarriesTheApprovalTrailer` |
| The used rows leave the file after the commit | Done | `renderApprovalPrune` (`script.go`) | `TestCommitDropsTheUsedApprovalRows` |
| The guide's section becomes one paragraph | Done | `docs/contributing/rfc-implementation-guide.md` "The owner's approval lives in the commit" | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestApproveWritesTheSessionRow` (`internal/le/rfc/approve_test.go`) | `rfc.Approve` replaces a row naming the same unit (`rowNames`); an empty reason is refused |
| AC-2 | Done | fixtures `rfc-approval-missing-refuses-the-edit` (exit 2, prints `./le rfc approve unit probe.TestUnledgered reason`), `rfc-approval-row-opens-the-gate` (exit 0), `rfc-approval-row-for-another-unit-buys-nothing` (exit 2) | `ok internal/le/hookcheck` (`scratch/round2-hookcheck-pkg.log`, `hookcheck-yang3.log`) |
| AC-3 | Done | `TestCommitRefusesAChangedTaggedUnitWithNoApproval`, `TestCommitCarriesTheApprovalTrailer` (`internal/le/commit/rfcchange_test.go`) | `ok internal/le/commit` (`scratch/commit-wrap-green.log`) |
| AC-4 | Done | `TestCommitDropsTheUsedApprovalRows` | the unused row stays; a landed row is dropped |
| AC-5 | Done | `ls test/rfc-changed` fails; `grep -rn "rfc-changed\|RFC-changed\|RFCChanged" internal cmd pkg docs ai .claude test` answers three `test/weakened/` history rows only | `rfcApprovalTree` replaces the ten `rfc-changed-*` fixtures |
| AC-6 | Done | `internal/le/testweakened` suite `ok` (jobs `pkg-green`, `final-fast`); `internal/le/commit` suite `ok` | `WeakenedDir`, `ShardPath`, `PruneLanded`, `ForeignShardProblems` keep their weakened callers |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestApproveWritesTheSessionRow` | Done | `internal/le/rfc/approve_test.go` | |
| `TestApproveHookAndCommitReadOneFile` | Done | `internal/le/testweakened/proposed_test.go` | |
| `TestCommitCarriesTheApprovalTrailer` | Done | `internal/le/commit/rfcchange_test.go` | |
| `TestCommitRefusesAChangedTaggedUnitWithNoApproval` | Done | `internal/le/commit/rfcchange_test.go` | |
| `TestCommitDropsTheUsedApprovalRows` | Done | `internal/le/commit/rfcchange_test.go` | extended in round 2 (#2) |
| `TestMessageKeepsTrailerLinesWhole` | Done | `internal/le/commit/input_test.go` | |
| `TestMessageRefusesAHandWrittenApprovalTrailer` | Done (added) | `internal/le/commit/input_test.go` | round 1 #1, round 2 #7 |
| three `rfc-approval-*` fixtures | Done | `fixtureSites` in `internal/le/hookcheck/fixture_catalog.go` | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/le/testweakened/shard.go`, `proposed.go` | Done | `RFCChangedDir` gone; `proposedApprovals` |
| `internal/le/commit/rfcchange.go`, `prepare.go`, `input.go`, `script.go` | Done | |
| `internal/le/rfc/actions.go` (the `approve` row), `approve.go`, `approve_test.go` | Changed | the row registers in `actions.go`, not `register.go` |
| `internal/le/rfc/tagged_scope_action.go` | Done | `taggedScopeBlockedMessage` prints the command per unit |
| `internal/le/hookcheck/fixtures.go`, `fixture_catalog.go` | Done | plus the parent's YANG probe fix |
| `internal/le/doc/check/links.go` | Done | the exemption and its test row removed |
| the four pages, `.claude/hooks/README.md` | Done | plus `rfc-conformance-gates.md`, `functional-tests.md`, `ai/skills/ze-rfc.md` |
| `ai/rules/points/testing/...` and `ai/rules/testing.md` | Done | rendered; `TRIGGERS.md`, `CORE.md`, `INDEX.md` unchanged by the render |

### Audit Summary
- **Total items:** 21
- **Done:** 20
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (the action registers in `actions.go`)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| The command writes the approval into `tmp/commit-rfc-approved-<session>.md` | unit test | `TestApproveWritesTheSessionRow` (`internal/le/rfc/approve_test.go`) over `rfc.Approve`: creates the file, replaces the row, refuses a missing reason |
| The hook reads the session's `tmp/` file | hookcheck fixtures | `rfc-approval-missing-refuses-the-edit`, `rfc-approval-row-opens-the-gate`, `rfc-approval-row-for-another-unit-buys-nothing` through `./le hook-check pretool-writeedit`; `TestApproveHookAndCommitReadOneFile` proves the hook and the gate derive one path |
| `commit create` reads the file, requires a row per changed unit, writes the trailer | unit tests over a real temporary repository | `TestCommitRefusesAChangedTaggedUnitWithNoApproval`, `TestCommitCarriesTheApprovalTrailer` (`git log -1 --format=%B` shows `RFC-approved:`), `TestMessageKeepsTrailerLinesWhole`, `TestMessageRefusesAHandWrittenApprovalTrailer` |
| The used rows are dropped after the commit succeeds | unit test | `TestCommitDropsTheUsedApprovalRows`: the used row leaves, the unused row stays, the landed row is refused naming the carrier commit |
| No directory; the shards are deleted | ls and grep | `ls test/rfc-changed` answers "No such file or directory"; `grep -rn "rfc-changed\|RFC-changed\|RFCChanged" internal cmd pkg docs ai .claude test` answers only three `test/weakened/` history rows |
| The guide holds one paragraph: the command, the trailer, the refusal | grep | `docs/contributing/rfc-implementation-guide.md` "The owner's approval lives in the commit"; "Who writes an owner-approval row" is gone |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| none | round 2 #8 (a raw `\x1e` byte in a hand-crafted body hides a trailer after it, fail-closed at the gate) is a NOTE with no work owed | - |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/rfc-approval-lives-in-the-commit-d19fb97b-265c-45eb-a3c1-a3078ceebb08.md` (42 files, verdict=clean) |
| `review check` | clean |
| Rounds | 3 |
| Reviewer lenses used | round 1: logic+wiring over the whole diff at 13f98cb7ef; round 2: the three fixes and their sibling sites, forgery and match edge cases; round 3: the wrap-order fix |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | a hand-written `RFC-approved:` body line lands as the record | `Message` (`internal/le/commit/input.go`) | `refuseHandWrittenTrailer`; `TestMessageRefusesAHandWrittenApprovalTrailer` |
| 2 | ISSUE | the substring `--grep` reads an extending landed trailer as this row's | `trailerLanded` (`internal/le/commit/rfcchange.go`) | whole-line verdict over `%B`, the carrier named in the refusal; `TestCommitDropsTheUsedApprovalRows` |
| 3 | ISSUE | two comments still said "the RFC-changed ledger" | `internal/le/hookcheck/fixtures.go` | reworded |
| 7 | ISSUE | the forgery guard ran before the wrap | `wrapBody` (`input.go`) | the guard runs over the emitted lines |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/le/rfc/approve.go` | yes | `git status --porcelain`: `?? internal/le/rfc/approve.go` |
| `internal/le/rfc/approve_test.go` | yes | `?? internal/le/rfc/approve_test.go` |
| `internal/le/commit/rfcchange_test.go`, `input_test.go` | yes | `??` both |
| `test/rfc-changed/` | no | `ls: cannot access 'test/rfc-changed': No such file or directory` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | `Approve` writes and replaces | `rowNames` and the `Replaced` branch read in `approve.go`; `TestApproveWritesTheSessionRow` passes in job `rfc-final` (the package's two reds are `TestNativeImplementationFixture` and `TestSupportedRowsHaveDerivableScope`, another session's) |
| AC-2 | the hook refuses without a row and prints the command | `proposedApprovals` returns `Missing` on `os.ErrNotExist`; `ProposedReport.Text` prints `rfc.ApproveCommand`; `ok internal/le/hookcheck` (`scratch/hookcheck-yang3.log`) |
| AC-3 | the gate refuses, then carries the trailer | `rfcChangeProblems` builds `problems` from unclaimed changes; `Create` composes `Message(..., result.RFCApprovals.Trailers)`; `ok internal/le/commit` (`scratch/commit-wrap-green.log`) |
| AC-4 | the prune runs after `git commit` | `renderBlock` appends `renderApprovalPrune` after the commit line; `TestCommitDropsTheUsedApprovalRows` |
| AC-5 | nothing names the ledger | the grep and the `ls` above |
| AC-6 | `test/weakened/` unchanged | `git status --porcelain test/weakened` empty; `ok internal/le/testweakened` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `./le rfc approve unit ...` | `internal/le/rfc/approve_test.go` (Go: a `./le` action) | yes, `approveAnswer` -> `Approve` |
| the pretool hook | hookcheck fixtures `rfc-approval-*` | yes, `Proposed` -> `proposedApprovals` |
| `./le commit create` | `internal/le/commit/rfcchange_test.go` over a real temporary repository | yes, `Create` -> `checkSourceGates` -> `rfcChangeProblems` -> `Message` |
| the script after success | `TestCommitDropsTheUsedApprovalRows` runs the generated script | yes |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `approveAnswer` and `Proposed` both resolve `lepath.CommitSession(root, "")` and `rfc.ApprovalPath(session)`; `TestApproveHookAndCommitReadOneFile` |
| A-2 | confirmed | `Message` joins `trailers` after `wrapBody`, never through it; `TestMessageKeepsTrailerLinesWhole` |
| A-3 | confirmed | the grep answers three `test/weakened/` rows (`cf1378ad.md`, `c9a630a5.md`), another ledger's history, which stay |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| #3 CLI command: `rfc-implementation-guide.md`, `committing.md` describe the command, the trailer, the refusal, the whole-line match | `Approve`, `Message`, `refuseHandWrittenTrailer`, `trailerLanded` read | yes |
| #10 test infrastructure: `test-health.md`, `.claude/hooks/README.md` | `proposedApprovals`, `rfcChangeProblems` | yes |
| #16 anchors: `shard.go` -> `test-health.md`, `rfcchange.go` -> `rfc-implementation-guide.md` | both pages edited in this diff | yes |
| #17 the guide's example is the command | the fenced `./le rfc approve unit` line | yes |
| Every No | `grep -rn "rfc-changed" docs` answers nothing | yes |
