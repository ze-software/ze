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
| A-1 | The hook and the commit command resolve the same eight-hex commit session for one harness session | `./le commit session`, `lepath/commitsession.go` | the hook reads a file the commit never writes | `TestApproveHookAndCommitReadOneFile` | unvalidated |
| A-2 | A trailer line survives the message wrap unchanged | `Message` in `internal/le/commit/input.go` | a long reason is wrapped and the record is cut | `TestMessageKeepsTrailerLinesWhole` | unvalidated |
| A-3 | No gate other than `rfcChangeProblems` and `Proposed` reads `test/rfc-changed/` | grep `RFCChangedDir` and `rfc-changed` | a reader keeps looking for a directory that is gone | `grep -rn "rfc-changed\|RFCChangedDir" internal cmd pkg docs ai .claude` empty | unvalidated |

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
