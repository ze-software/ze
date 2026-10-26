# Spec: structural-gate-charges-owned-reds

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Priority | P2 |
| Release reason | Owner rule (`ai/META-DECISIONS.md`, "Ze is pre-release"): a red prevents a release, never a commit. The commit gate's structural red check contradicts it and is deleted. Thomas ordered it implemented now, ahead of the post-release queue (2026-10-26). No shipped behaviour changes |
| RFC gaps | - |
| Depends | - |
| Phase | 3/3 |
| Handoff | - |
| Updated | 2026-10-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Owner authorisations for this spec only (2026-10-26, relayed by the main session): the claim runs over the WIP cap (`ZE_SPEC_WIP_CAP=14 ./le spec claim`), and the work is implemented now rather than after the first release. The file keeps its path under `post-release/engineering/` because that folder is where tooling lives (`plan/README.md`); only Priority changed. The slug is historical: the first design (owned-only charging, superseded the same day) named it.

## Task

`./le commit create` refuses a commit for a structural red. `checkVerificationGates` (`internal/le/commit/prepare.go`) calls `structuralGateReds` (`internal/le/commit/verification.go`), which reads `tmp/ze-verify-failures.json` and charges reds, and two admissions get past it: `structural-red-ok "<reason>"` (trailer `Structural-red-ok:`) and `broken-head-fix "<reason>"` (trailer `Broken-head-fix:`, the sole `repo compiles/check` red).

The owner rule committed in 9df98834d3 removes the ground for that gate:

> A red prevents a release, never a commit. No gate refuses a commit for a red, whether a test, lint, a structural check or a generated-file check, and whether or not the commit caused it. The commit names the reds it knows of in its body. Reds are owed at release, where Thomas's release check must be green.

Measured on 2026-10-26 with `git log --since=2026-10-23 --format=%b | grep '^Structural-red-ok'`: 286 trailers in 491 commits, each one re-diagnosing the same foreign reds. The gate cost an admission on more than half of all commits and held nothing.

| # | Design (owner rule, 9df98834d3) |
|---|---------------------------------|
| D1 | The commit-time structural red check is deleted entirely: `structuralGateReds`, its classification and charging, the refusal in `checkVerificationGates`, and every helper only it uses. No narrowed version, no fallback, no flag. `./le commit create` no longer reads the failure index |
| D2 | The admissions `structural-red-ok` and `broken-head-fix` are retired, with their `Options` fields, policy entries, trailers and `declaredTrailers` rows. The create grammar keeps each as a `Retired` keyword, the mechanism `missing-full-verify-ok` and `unverified` already use, so typing one is refused naming the replacement: name the red in the body |
| D3 | What is not a red stays: the independent-review artifact gate (`review-override`), the RFC-approval and test-weakening approval gates (`rfc-change-ok`), test coverage (`no-test`), the message rules and declared-trailer refusal, `hunks`. The verification certificate is still reported (`verify=FRESH/STALE`), never refusing |
| D4 | The release check (`./le verify worktree`, Thomas's) and the push derivation (`deriveDebt`, `internal/le/commit/owed.go`) are unchanged. The verify engine still writes the failure index, carries verdicts forward and declares failure groups: `./le verify reds` and the text index read them, and the release check is where a red is owed |
| D5 | No session-start reds line. The rule asks a commit to name the reds its author knows of. No existing reader turns the failure index into one line: after D1 the only classifier is gone, and a new one would be a new surface whose index is often days old (the 2026-09-13 journal row). The verify run that finds a red prints it, and `./le verify reds file <path>` answers per path |

## Required Reading

### Architecture Docs
- [ ] `ai/META-DECISIONS.md` - "Ze is pre-release", the red-never-holds-a-commit rule
  → Decision: no gate refuses a commit for a red, owned or not; the commit body names known reds
- [ ] `docs/contributing/committing.md` - keyword table (`attest` row lists the admissions), admission table, hunks paragraph's structural-gate bullet
  → Constraint: another session holds uncommitted hunks at lines ~61-77 (approval wording); this work commits its own hunks only (`hunks <path> <patch-file>`)
- [ ] `docs/architecture/testing/verify-freshness-scope.md` - "Failure attribution" (the commit-package paragraph and table), "Asking before the run ends", the publication paragraph that says commit preparation charges from the index
  → Constraint: the engine half of those paragraphs (carryForward, JudgesInPart, interrupted marking, publication) stays true and stays; only what the commit package did with the index is removed
- [ ] `docs/contributing/running-commands.md` - the "It is a query, never a certificate" paragraph and "Stage times, and stopping at the first red" (the commit gate charging unreached and carried reds, the clearing line)
- [ ] `docs/architecture/testing/tracked-build-gate.md` - "STRUCTURAL_GATES ... the next commit is refused" and "The deadlock the structural listing created" (`broken-head-fix`)
  → Constraint: `./le repo compiles check` after a script that carried Go stays (`ai/rules/precommit-verify.md`); only the commit refusal it fed goes
- [ ] `.claude/hooks/README.md` - commit-gate table row `structural-gate` and the trailer sentence
- [ ] `le help commit` - the create page is rendered from `createParameters`, so a retired keyword disappears from help and completion with no edit (`leaction` leaves Retired keywords out)

**Key insights:**
- `Retired` (`internal/le/le/action/parameter.go`) refuses with `<command> no longer takes "<kw>": use <replacement>`, and completion, help and the manifest leave a retired keyword out. That is the existing removed-keyword refusal, so D2 reuses it.
- `TestStaleIndexOKIsNotAKeyword` shows the other precedent (a keyword dropped with no refusal). `Retired` is chosen because the create grammar already refuses removed keywords by name.
- `verificationState` appended `UnjudgedCauses` and `Unreadable` from `structuralGateReds` to the stale detail. Both explain why a red was not charged; with no charging, both lose their meaning and go with D1.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/commit/verification.go` (386L) - `VerificationState`, `verificationState` (certificate, then the index-derived unjudged/unreadable detail), and the whole index reader: `structuralStages`, `structuralReds`, `failureIndex`, `failureOrigin`, `failureStage`, `failureGroup`, `stageCommand`, `failureIndexPath`, `unreadableIndex`, `structuralGateReds`, `redCause`, `unjudgedCause`, `clearingRuns`, `inPopulation`, `judgedBy`, `groupRelatedPaths`, const `compilesStage`
  → Decision: only `VerificationState`, the two state constants and the certificate half of `verificationState` survive
- [ ] `internal/le/commit/prepare.go` - `Options.StructuralRedOK`, `Options.BrokenHeadFix`; `Prepared.Trailers` comment; `checkVerificationGates` (the `result.Verify.State != verifyFresh` block)
- [ ] `internal/le/commit/actions.go` - `keywordStructuralRedOK`, `keywordBrokenHeadFix`, `trailerStructuralRedOK`, `trailerBrokenHeadFix`, two `declaredTrailers` rows, two `reasonParameter` rows, two `createPolicy` rows, `optionsOf`; `Retired` rows for `push`, `unverified`, `missing-full-verify-ok`
- [ ] `internal/le/le/action/parameter.go` - `Retired` refusal text
- [ ] `internal/test/fixture/misc_fixture_runner.go` - `verifyScopeWiringDriver` (functional `test/runner/verify-scope-wiring-attribution.ci`) asserts foreign red not charged, own-file red refuses, blind red charged
- [ ] `internal/le/verify/engine/artifacts.go`, `internal/le/verify/reds.go`, `internal/le/verify/failuregroup/failuregroup.go`, `internal/le/go/lint/failuregroup.go`, `internal/le/doc/wiring/delegate.go`, `internal/le/repo/compiles/report.go`, `internal/le/verify/deps/verifydeps.go` - comments that name `structuralGateReds` as the reason groups carry paths
  → Decision: comment edits only (`ai/rules/stale-comments.md`); the attribution stays for `./le verify reds` and the text index

**Behavior to preserve:**
- Every non-red gate and its admission (D3), the message and declared-trailer rules for the trailers that remain, `hunks`, the certificate state in the `create` report
- The push derivation and the `verification debt:` line (`owed`)
- The verify engine's index, carry-forward and failure groups (D4)

**Behavior to change:**
- A commit is never refused for anything the failure index holds (D1)
- `structural-red-ok` and `broken-head-fix` are refused as retired keywords naming `body` as the replacement (D2)
- The stale verify detail no longer carries index-derived unjudged/unreadable text

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le commit create ... file <path>` → `Answer` → `parseCreate` → `Create` → `checkVerificationGates`

### Transformation Path
1. `parseCreate` refuses a retired keyword before `Options` exists (`leaction` `Retired`)
2. `checkVerificationGates` sets `result.Verify` from the certificate and goes straight to the review gate; no index read

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| verify engine ↔ commit gate | was: the JSON failure index; after: none | `grep -rn 'ze-verify-failures' internal/le/commit` answers nothing outside tests |

### Integration Points
- `leaction.Parameter.Retired` - existing removed-keyword refusal
- `verifyengine.CheckCertificate` - still the certificate source of `verificationState`

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Fewer layers: the commit package stops reading a verify artifact |
| No unintended coupling (components stay isolated) | Yes | `internal/le/commit` drops its import of `internal/le/verify/failuregroup` |
| No duplicated functionality (extends existing, does not recreate) | Yes | Retirement reuses `Retired` |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Tooling, no wire path |
| Registration over hardcoding, outbound | N-A | Deletion only |
| Registration over hardcoding, inbound | N-A | Deletion only; no list added |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Nothing outside `internal/le/commit` reads `structuralReds`, the commit package's index types or the two Options fields | grep for the symbols over `internal`, `cmd`, `test` | A build break elsewhere | `./le repo compiles check` and the scoped build | confirmed: grep finds no reader; `verifyengine.Structural` was the one indirect dependency and is deleted with it |
| A-2 | The engine's carry-forward and interrupted marking keep their own tests after the commit-side tests go | `internal/le/verify/engine/carry_test.go`, `carry_partial_test.go`, `artifacts_interrupted_test.go` | Engine semantics lose coverage | Read the named engine tests before deleting the commit-side ones | confirmed: `TestAPieceCarriesAnEarlierRedForAStageItDidNotRun`, `TestAFullGreenRunAfterACarriedRedCarriesNothing`, `TestACutFullRunPieceIsAFullJudgement`, `TestAnInterruptedStageIsWrittenAsNeverJudged` and siblings stay |
| A-3 | A `Retired` keyword with a value (`structural-red-ok "<reason>"`) is refused before its value is read | `parameter.go` refuses on the keyword | The reason would parse as an unknown word and the message would not name the replacement | `TestTheRetiredRedAdmissionsNameTheBody` | confirmed: green, refusal names the keyword and `use body` |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A real breakage lands on main unrefused | A red the release check finds | The owner's accepted trade: reds are owed at release. `./le repo compiles check` after Go commits stays a rule |
| R-2 | Agents keep typing `structural-red-ok` from habit or old handovers | The retired refusal | The refusal names the replacement (`body`) |
| R-3 | Existing tests encode the old refusal | They go red after the deletion | Each is deleted or rewritten, listed by name below, and judged by the `ze-test-judge` skill run by a separate agent before the edit lands; the owner rule decides the behaviour |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Commits that the old gate refused now land (the intended change); nothing user-visible in ze |
| How is it reverted? | Single commit revert |
| Who else touches this path? | Any session editing `internal/le/commit` (another session holds uncommitted `owed.go`, `rfcchange.go`, `ledger_test.go` hunks: not touched here) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `le commit create ... file mine.go` with the index holding an owned, a blind and an unreached structural red | → | `checkVerificationGates` (no index read) | `test/runner/commit-not-refused-for-a-red.ci` (markers `owned-red-not-refused`, `blind-red-not-refused`, `unreached-red-not-refused`) |
| `le commit create ... structural-red-ok "<r>"` / `broken-head-fix "<r>"` | → | `Retired` refusal | `test/runner/commit-not-refused-for-a-red.ci` (marker `retired-red-admissions-name-the-body`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Index holds a red structural stage whose group names a path the commit carries; no admission | `create` succeeds; the message carries no `Structural-red-ok:` |
| AC-2 | Index holds a red with a group naming no path, and a structural stage `reached:false` from a first-red stop | `create` succeeds |
| AC-3 | Index is unreadable JSON | `create` succeeds; the verify detail carries no index text |
| AC-4 | `create ... structural-red-ok "<r>"` or `broken-head-fix "<r>"` | Refused (usage, exit 2) with `no longer takes "<kw>": use body "<...>"` |
| AC-5 | `le help commit` create page, completion and the manifest | Neither keyword appears |
| AC-6 | A body line starting `Structural-red-ok:` | Accepted as prose (no declared trailer by that key) |
| AC-7 | `review-override`, `rfc-change-ok`, `no-test`, weakening approvals, `hunks` | Unchanged (their existing tests stay green) |
| AC-8 | `grep -rn 'structuralGateReds\|structural-red-ok\|broken-head-fix\|Structural-red-ok\|Broken-head-fix' internal docs ai .claude` | Only the `Retired` rows, their tests, and history (`plan/`) answer |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCreateIsNotRefusedForARedInTheIndex` (new, replaces `TestCreateRefusesChargedStructuralRedWithoutRecordedOverride`) | `internal/le/commit/commit_test.go` | AC-1, AC-2, AC-3 through `Create` | |
| `TestTheRetiredRedAdmissionsNameTheBody` (new) | `internal/le/commit/actions_test.go` | AC-4, A-3 | |
| `TestCommitCreateHelpRendersEveryLiveKeyword` (edited: the two keywords move from the live list to the retired list) | `internal/le/commit/actions_test.go` | AC-5 | |
| `TestStaleIndexOKIsNotAKeyword` (edited: the discrimination parses `review-override` instead of `structural-red-ok`) | `internal/le/commit/commit_test.go` | precedent kept | |
| `TestCreateRefusesBadTagBeforeTheGates` (edited: drops the `StructuralRedOK` field it set only to pass the old gate) | `internal/le/commit/commit_test.go` | unchanged assertion | |
| `TestMessageRefusesDeclaredTrailerInBody` (edited: the two keys leave the declared list), `TestMessageWritesOwedTrailers` (edited: the two audit subtests and the granted-but-unused subtest go) | `internal/le/commit/input_test.go` | AC-6, AC-7 | |
| `TestAttestRefusesAdmissionKeywords` (edited: two rows go) | `internal/le/commit/owed_consumers_test.go` | AC-7 | |
| create golden (`create_golden_test.go`, edited: the `structural-red-ok` argument goes; golden file regenerated if it renders it) | `internal/le/commit/create_golden_test.go` | AC-7 | |
| Deleted, subject gone: `TestStructuralRedsAttributeOnlyPathBearingGroups`, `TestStructuralRedsSeeNothingUntilARunPublishesItsIndex`, `TestCreateRefusesChargedStructuralRedWithoutRecordedOverride`, `TestAnUnreachedStructuralStageIsCharged`, `TestAPieceKeepsAnEarlierStructuralRedCharged`, `TestACarriedRedNamesTheRunThatClearsIt`, `TestACarriedRedWithNoRecordedCommitSaysSo`, `TestAGreenFastRunChargesNoStructuralStage`, `TestAGreenSweepOfEveryPieceChargesNoStructuralStage` (`commit_test.go`); `clearing_test.go` whole; `verification_interrupted_test.go` whole | `internal/le/commit` | no-layering: the gate they assert is deleted | |

### Boundary Tests (numeric inputs)
N-A: no numeric input.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `commit-not-refused-for-a-red` (new, replaces `verify-scope-wiring-attribution`) | `test/runner/commit-not-refused-for-a-red.ci`, driver `commitNotRefusedForARedDriver` in `internal/test/fixture/misc_fixture_runner.go` | In a scratch checkout, `le commit create` passes with an owned, a blind and an unreached red in the index, and refuses each retired admission naming `body` | |

### Interop Tests (Scope: protocol)
N-A: tooling, no protocol peer.

## Files to Modify
- `internal/le/commit/verification.go` - delete the index reader (every symbol listed in Current Behavior except `VerificationState`, the state constants and the certificate half of `verificationState`)
- `internal/le/commit/prepare.go` - delete the two `Options` fields, the structural block in `checkVerificationGates`, the doc comments naming it, the `Prepared.Trailers` mention
- `internal/le/commit/actions.go` - delete the two keyword, trailer, `declaredTrailers`, `reasonParameter` and `createPolicy` entries and the `optionsOf` reads; add two `Retired` rows
- `internal/le/commit/commit_test.go`, `input_test.go`, `owed_consumers_test.go`, `actions_test.go`, `create_golden_test.go` (+ its golden file if it renders the keyword) - per the TDD plan
- `internal/test/fixture/misc_fixture_runner.go` - replace `verifyScopeWiringDriver` and its registration with `commitNotRefusedForARedDriver`; `internal/test/fixture/constants.go` loses the six constants only the old driver used
- `internal/le/verify/engine/stages.go` - delete `Stage.Structural`, `Structural(mode)` and the `structural(...)` wrapper (each stage becomes `stage(...)`, populations unchanged): their only reader was `commit.structuralStages`, and their contract ("the stages whose red refuses a commit") is false under the owner rule. Tests: `TestStructuralStagesAreMembersOfThePopulation`, `TestStructuralIsASubsetOfFull` deleted, one assertion of `TestChangedPopulationReadsScope` removed, `TestAnIndexNamesEveryFullStageItDidNotRun` comment and case-field rename (all judged)
- Comment-only: `internal/le/verify/engine/artifacts.go`, `internal/le/verify/reds.go`, `internal/le/verify/failuregroup/failuregroup.go`, `internal/le/go/lint/failuregroup.go`, `internal/le/doc/wiring/delegate.go`, `internal/le/repo/compiles/report.go`, `internal/le/verify/deps/verifydeps.go`, and the test-file header comments in `verify/lifecycle_test.go`, `verify/engine/artifacts_test.go`, `verify/failuregroup/failuregroup_test.go`, `verify/deps/verifydeps_test.go`, `doc/wiring/delegate_paths_test.go`, `go/lint/failuregroup_test.go`, `go/staticcheck/failuregroup_test.go`, `repo/compiles/failuregroup_test.go`, `setup/probes_toolchain_test.go`
- `docs/contributing/committing.md` - `attest` row, admission table rows, hunks bullet, source anchor
- `docs/architecture/testing/verify-freshness-scope.md` - publication paragraph sentence, "Failure attribution" commit-package text and table, "Asking before the run ends", anchors
- `docs/contributing/running-commands.md` - query paragraph, first-red paragraph's commit-gate sentences, anchor
- `docs/architecture/testing/tracked-build-gate.md` - the refusal sentence and the deadlock section
- `docs/functional-tests.md` - runner row names the new test instead of structural-red attribution
- `.claude/hooks/README.md` - the `structural-gate` row and the trailer sentence (another session holds hunks in this file: own hunks only)
- `ai/INDEX.md` - the compiles row ("is a structural gate in `internal/le/commit`") and the failure-group row naming `internal/le/commit.Answer` (foreign hunks present: own hunks only)
- `ai/rules/points/completion/directives/verification-debt-is-not-defect-debt.md` - the admission list drops the two keywords and says no commit is refused for a red (Edit tool, then `./le ai rules update render`, `condensed`, `index`)
- `ai/rules/completion.md`, `ai/rules/CORE.md`, `ai/rules/TRIGGERS.md` - regenerated only
- No skill under `ai/skills/` names either keyword (grep 2026-10-26), so no `./le ai sync update`

## Files to Create
- `test/runner/commit-not-refused-for-a-red.ci`

## Files to Delete
- `internal/le/commit/clearing_test.go`, `internal/le/commit/verification_interrupted_test.go`, `test/runner/verify-scope-wiring-attribution.ci` (through `remove` in the commit)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | le tooling |
| YANG validation constraints | N-A | le tooling |
| YANG custom validators | N-A | le tooling |
| CLI commands/flags | Yes | two create keywords retired; help and completion derive from `createParameters` |
| CLI grammar (keyword before value) | N-A | no grammar change |
| Editor autocomplete | Yes | derived: `Retired` keywords are left out |
| Functional test for new RPC/API | Yes | `test/runner/commit-not-refused-for-a-red.ci` |
| Pipe completeness | N-A | no new answer |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | N-A | none |
| Prometheus counters/metrics | N-A | tooling |
| BGP family surface | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | le tooling |
| 2 | Config syntax changed? | No | none |
| 3 | CLI command added/changed? | Yes | `docs/contributing/committing.md` |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | No | developer docs only |
| 7 | Wire format changed? | No | none |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | No | none |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/verify-freshness-scope.md`, `docs/contributing/running-commands.md`, `docs/architecture/testing/tracked-build-gate.md`, `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | No | `docs/architecture/core-design.md` needs nothing: no hook added |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | the `verification.go -- structuralGateReds` anchors in committing.md, running-commands.md, verify-freshness-scope.md are removed or repointed. `docs/features/ai-first.md` (declared by `internal/le/commit/prepare.go`) is unaffected: its commit paragraph names `commit.Answer` and the script, never the structural gate |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | the refusal text quoted in running-commands.md and verify-freshness-scope.md |

## Implementation Steps

1. **Phase: Tests first** - write `TestCreateIsNotRefusedForARedInTheIndex`, `TestTheRetiredRedAdmissionsNameTheBody` and the new `.ci` driver; see them fail against the current gate
2. **Phase: Deletion** - delete the gate, fields, keywords, trailers; add `Retired` rows; route each existing test edit or deletion through the `ze-test-judge` skill (separate agent) and apply the judged ones
3. **Phase: Comments, docs, rules** - stale comments; the pages in Files to Modify; the rule point, then `./le ai rules update render`, `condensed`, `index`
4. **Evidence owed** - scoped package tests for `internal/le/commit`, `internal/le/le/action`, `internal/test/fixture` build, the touched comment-only packages' build; `./le test runner commit-not-refused-for-a-red` by name. Discrimination: restore the refusal block in `checkVerificationGates` and see `TestCreateIsNotRefusedForARedInTheIndex` go red

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | No code path in `internal/le/commit` reads `ze-verify-failures` |
| Rule: no-layering | No flag, no narrowed charge, no fallback survives |
| Rule: stale comments | No comment names `structuralGateReds` |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Gate deleted | AC-8 grep |
| Commit not refused for a red | `TestCreateIsNotRefusedForARedInTheIndex`, `commit-not-refused-for-a-red.ci` |
| Retired admissions refused by name | `TestTheRetiredRedAdmissionsNameTheBody` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The commit package reads no index, so no index input remains |
| Fail-open | Intended by the owner rule for reds; every non-red gate keeps failing closed |

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
- Forced red (before the deletion): `TestCreateIsNotRefusedForARedInTheIndex` failed all three cases with `deterministic structural gate(s) block this commit` (owned, blind+unreached) and the index text in the verify detail (unparsable); `TestTheRetiredRedAdmissionsNameTheBody` failed with `create accepted "structural-red-ok"`. Both green after.
- Test judge (`ze-test-judge`, separate agent): 24 units approved in batch one (16 deleted commit tests, 7 edited, runner `verify-scope-wiring-attribution`), 3 in batch two (engine Structural tests); rows in the session's weakening approval file.
- `TestDispatchStopsACanceledStageWithSIGTERM` (`internal/le/verify/dispatch`) is red alone and repeatably (`stage was not given SIGTERM: ""`); this work does not touch that package (last changed by 140c318edd, another session's).
- After D1 no commit-side program reads the JSON failure index, but the engine does, so it stays (D4). `writeFailureIndex` reads the checkout's earlier index through `readEarlierIndex` to carry forward the verdict of every stage a run did not hold (`carryForward`), and `PublishFailureIndex` (`internal/le/verify/engine/artifacts.go`, called from `run` in `internal/le/verify/lifecycle.go`) reads a worktree run's index to merge it into the checkout's. `./le verify reds` reads stage logs, not the index (`scanStages`, `internal/le/verify/reds.go`).
  → Decision (closure): the JSON index keeps its writer; removing it would delete the carry-forward and worktree publication, which D4 keeps.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `plan/journal/stale-artifact-reused.md` | 2026-09-13 | `structuralGateReds` charges stale blind reds (`tier/check`, `iface-resolution`) to every commit |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Delete the gate whole | Owned-only charging (this spec's first design, agreed 2026-10-26 and superseded the same day) | The owner rule forbids any commit refusal for a red, owned or not |
| Retire the two keywords with `Retired` naming `body` | Drop them as unknown words (`stale-index-ok` precedent) | The create grammar already refuses `push`, `unverified`, `missing-full-verify-ok` by name; a habit-typed keyword then teaches the replacement |
| No session-start reds line | A new index reader at session start | Not trivial after D1, and the index is often stale; the verify run and `./le verify reds` already report reds |
| Drop the unjudged/unreadable index text from the stale verify detail | Keep a reader for the detail alone | Its only content was why a red was not charged |

## Known Limitations
- A red lands on main unrefused and is owed at the release check. This is the owner rule.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches, and `./le verify changed` over the closing commits (`/ze-close` step 6f). The full verification is Thomas's release check: no agent runs it, and one that is owed is handed to him as its exact command. A red there gets a fix commit
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied: lint fixed rather than disabled, the focused check for the changed behavior run once with its OUTPUT PASTED, and any red named with the one-line reason it is scaffolding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script (commit A preserves the spec in history)

## Implementation Summary

### What Was Implemented
- 8bbde623e1: `structuralGateReds` and every helper only it used are deleted from `internal/le/commit/verification.go`; `checkVerificationGates` (`prepare.go`) reports the certificate and reads no failure index; `Options.StructuralRedOK`/`BrokenHeadFix`, their keywords, trailers, `declaredTrailers` rows and `createPolicy` entries are gone; `structural-red-ok` and `broken-head-fix` are `Retired` rows naming `redsInTheBody` (`actions.go`); `Stage.Structural`, `Structural(mode)` and `structural(...)` are deleted from `internal/le/verify/engine/stages.go`.
- `test/runner/commit-not-refused-for-a-red.ci` with `commitNotRefusedForARedDriver` replaces `verify-scope-wiring-attribution`.
- Closure (commit A): stale comments in `internal/le/verify/lifecycle.go` (`run`) and `internal/le/go/staticcheck/report.go` (`Verdict.Text`), six test doc comments, the git-safety rule point, a dangling anchor in `verify-freshness-scope.md`, `plan/known-failures/README.md`, and the owned journal row marked `fixed:`.

### Bugs Found/Fixed
- None in product code. The closure findings are stale comments, docs and a rule sentence (Review Gate below).

### Documentation Updates
- In 8bbde623e1: `docs/contributing/committing.md` (attest row, admission table, "No admission exists for a red", hunks bullet), `docs/architecture/testing/verify-freshness-scope.md`, `docs/contributing/running-commands.md`, `docs/architecture/testing/tracked-build-gate.md`, `.claude/hooks/README.md`, `ai/INDEX.md`, rule point `verification-debt-is-not-defect-debt.md` + `ai/rules/completion.md`; `docs/functional-tests.md` row landed in da69c61bdb.
- In closure: `verify-freshness-scope.md` anchor `stages.go -- Structural` repointed to `stages.go -- stage, fullStages` under "Structural stages"; git-safety point `never-defer-a-shared-file-to-an-absent-session.md` drops "The structural gate", `ai/rules/git-safety.md` and `ai/rules/CORE.md` regenerated (`./le ai rules update render`, `condensed`, `index`); `plan/known-failures/README.md` no longer claims `prepare.go` refuses for a structural red.
- `./le --name session doc check verify`: exit 0.

### Deviations from Plan
- None in scope. The spec's open question on the JSON failure index is answered from source (Design Insights, Decision): the engine reads it, so it stays.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The implementation's comment and doc sweep stopped at the files the spec listed | Two shipped-code comments, six test comments, a rule point, an anchor and `plan/known-failures/README.md` still described the deleted gate | Closure grep for `charge`, `structural gate`, `structuralGateReds` over the whole tree | Fixed in commit A; no new rule (`ai/rules/stale-comments.md` already requires it) |
| assumption | Design Insight said the JSON index had no in-tree program reader | `writeFailureIndex` reads it through `readEarlierIndex`, and `PublishFailureIndex` reads a worktree's copy | Closure grep for `FailuresJSONPath` | Insight corrected, Decision recorded |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| D1 gate deleted, no index read | Done | `internal/le/commit/verification.go` `verificationState`; `prepare.go` `checkVerificationGates` | no `ze-verify-failures` outside tests in `internal/le/commit` |
| D2 admissions retired | Done | `internal/le/commit/actions.go` `createParameters` Retired rows, `redsInTheBody` | |
| D3 non-red gates unchanged | Done | `prepare.go`, `review.go`, `rfcchange.go` untouched by the gate deletion | package tests green |
| D4 release check, push derivation, engine index unchanged | Done | `owed.go` `deriveDebt`; `engine/artifacts.go` `writeFailureIndex` | |
| D5 no session-start reds line | Done | nothing added | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestCreateIsNotRefusedForARedInTheIndex` "a red naming the committed file"; runner marker `owned-red-not-refused` | |
| AC-2 | Done | same test, "a red naming no path and a stage not reached"; markers `blind-red-not-refused`, `unreached-red-not-refused` | |
| AC-3 | Done | same test, "an index that does not parse" (asserts no index text in `Verify.Detail`) | |
| AC-4 | Done | `TestTheRetiredRedAdmissionsNameTheBody`; marker `retired-red-admissions-name-the-body`; live refusal exit 2 | |
| AC-5 | Done | `TestCommitCreateHelpRendersEveryLiveKeyword`; `le commit create --help` holds neither keyword | |
| AC-6 | Done | `TestMessageRefusesDeclaredTrailerInBody` (keys left `declaredTrailers`) | |
| AC-7 | Done | `go test -race ./internal/le/commit/` green | |
| AC-8 | Done | grep over `internal docs ai .claude cmd test`: only the Retired rows, their tests, the fixture and history sentences in docs | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestCreateIsNotRefusedForARedInTheIndex` | Done | `internal/le/commit/commit_test.go` | red before, green after (Design Insights) |
| `TestTheRetiredRedAdmissionsNameTheBody` | Done | `internal/le/commit/actions_test.go` | |
| edited and deleted tests | Done | `internal/le/commit`, `internal/le/verify/engine` | judged by ze-test-judge; `Weakening-approved:` trailers on 8bbde623e1 |
| `commit-not-refused-for-a-red` | Done | `test/runner/commit-not-refused-for-a-red.ci` | PASS at closure |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| every file in Files to Modify, Create, Delete | Done | 8bbde623e1 stat (43 files); `docs/functional-tests.md` via da69c61bdb |

### Audit Summary
- **Total items:** 5 requirements, 8 ACs, 4 test rows, 1 file row
- **Done:** all
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 0

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| No gate refuses a commit for a red, owned, blind, unreached or unreadable | functional + unit | `./le test runner commit-not-refused-for-a-red`: `PASS 3 commit-not-refused-for-a-red` (real `le commit create` in a scratch checkout, three indexes); `TestCreateIsNotRefusedForARedInTheIndex` went red against the old gate and green after |
| A habit-typed retired admission teaches the replacement | functional | `le commit create subject s file a structural-red-ok "x"` exits 2: `no longer takes "structural-red-ok": use body "<each red you know of and its cause>"` |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| none | every agreed item is done | - |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/structural-gate-charges-owned-reds-b8e3721e-66b0-4a06-9363-b20f131aeb0f.md` |
| `./le spec review check` | clean (8 code files, hashes match) |
| Rounds | 2 |
| Reviewer lenses used | wiring + removed-behavior + docs drift (run 1, full diff of 8bbde623e1 and a tree-wide grep for the removed mechanism); the fixes only (run 2) |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | Comment says commit preparation charges structural reds from the index | `internal/le/verify/lifecycle.go` `run` | names the `writeFailureIndex` carry-forward |
| 2 | ISSUE | Comment says `commit/verification.go` charges a pathless red to every commit | `internal/le/go/staticcheck/report.go` `Verdict.Text` | names `readReds` answering undetermined |
| 3 | ISSUE | Rule says "the structural gate" still judges the working-tree file | `ai/rules/points/git-safety/directives/never-defer-a-shared-file-to-an-absent-session.md` | sentence fixed; `git-safety.md`, `CORE.md` regenerated |
| 4 | ISSUE | Test doc comments: a stray group "would charge a red to the commits" | six `failuregroup`/`verifydeps`/`carry` test files | comment-only rewording; no assertion changed |
| 5 | ISSUE | Source anchor names deleted `Structural` | `docs/architecture/testing/verify-freshness-scope.md` | `stage, fullStages`, moved under "Structural stages" |
| 6 | ISSUE | Page claims `prepare.go` refuses while a structural gate is red | `plan/known-failures/README.md` | states no commit gate enforces it |
| 7 | ISSUE | Owned row left `open:`; removing the spec would orphan it | `plan/journal/stale-artifact-reused.md` 2026-09-13 | `fixed:` naming 8bbde623e1 and the test |

NOTEs (not fixed): `ai/rules/completion.md` calls `./le doc wiring` a "structural stage"; `ai/INDEX.md` keeps the keyword "attributing a structural red". Both read as descriptions, not as the deleted mechanism.

Run 2 (the seven fixes): 0 BLOCKER, 0 ISSUE.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `test/runner/commit-not-refused-for-a-red.ci` | yes | `git show --stat 8bbde623e1` lists it (+17); runner ran it |
| `internal/le/commit/clearing_test.go`, `verification_interrupted_test.go`, `test/runner/verify-scope-wiring-attribution.ci` | removed | deleted in 8bbde623e1 (-116, -157, -14) |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1..AC-3, AC-6, AC-7 | create succeeds over any index red | `go test -race -count=1 ./internal/le/commit/`: `ok 43.048s` (closure run) |
| AC-4 | retired refusal names body | live `le --name session commit create ... structural-red-ok "x"`: exit 2, `use body "<each red you know of and its cause>"` |
| AC-5 | help holds neither keyword | `le --name session commit create --help`: 0 matches |
| AC-8 | nothing else names the gate | grep answers only the `createParameters` Retired rows, `TestTheRetiredRedAdmissionsNameTheBody`, `TestCommitCreateHelpRendersEveryLiveKeyword`, `TestCreateIsNotRefusedForARedInTheIndex`, `commitNotRefusedForARedDriver`, and history sentences in `committing.md`, `tracked-build-gate.md` and `verify-freshness-scope.md` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `le commit create ... file mine.go` over three indexes | `test/runner/commit-not-refused-for-a-red.ci` | read: driver writes each index, runs the real binary with no admission, prints each marker on exit 0; PASS |
| `le commit create ... structural-red-ok` / `broken-head-fix` | same | driver asserts non-zero exit, quoted keyword and `use body`; PASS |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | grep for `StructuralRedOK`, `BrokenHeadFix`, `structuralStages`, `verifyengine.Structural` over `internal cmd test`: none; package builds green |
| A-2 | confirmed | `go test -race ./internal/le/verify/engine/`: ok 173.5s (carry and interrupted tests kept) |
| A-3 | confirmed | `TestTheRetiredRedAdmissionsNameTheBody` green; live refusal names the keyword |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| CLI reference: `committing.md` admissions | `actions.go` `createParameters`, `redsInTheBody` | yes |
| Test infrastructure: `verify-freshness-scope.md`, `running-commands.md`, `tracked-build-gate.md` | `engine/artifacts.go` `writeFailureIndex`, `carryForward`; `reds.go` `readReds`, `verdictOf`, `attributionOf` | yes; `./le doc check verify` exit 0 |
| Source anchors on changed files | the only dangling one (`stages.go -- Structural`) repointed | yes |
