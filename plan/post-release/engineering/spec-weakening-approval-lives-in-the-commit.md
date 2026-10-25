# Spec: weakening-approval-lives-in-the-commit

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Priority | P2 |
| Release reason | Tooling only: the shipped binary is unchanged. Owner decision ccf8a56009 |
| Depends | - |
| Phase | 7/7 |
| Handoff | - |
| Updated | 2026-10-25 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`test/weakened/` is a tracked ledger the author writes for itself: the agent
writes the row, makes the weakening edit and commits it, so the owner never sees
the weakening. It records nothing the commit does not already show. Owner
decision (`ai/META-DECISIONS.md`, "Test weakening", ccf8a56009): weakening a
test needs Thomas's approval, asked before the edit; his words are recorded in
an untracked file the edit hook and the commit gate read; the commit carries one
`Weakening-approved:` line per test; no tracked ledger is kept.

Folded in by owner decision at the scope gate: `./le commit audit` reads the
commit's approval lines, both `Weakening-approved:` and `RFC-approved:`, so an
owner-approved commit stops reading WEAKENED in every later review.

Owner decisions at the research gate (2026-10-24):

| Question | Owner answer |
|----------|--------------|
| May one weakening approval cover a tree or a file scope? | No. One approval names one `<package>.<TestName>`, the RFC approval grammar. The scope matcher goes |
| An RFC-tagged test that is also weakened: one approval or two? | One. An `RFC-approved:` row for a unit also accepts that unit's weakening |

Owner constraint: every change to an existing test, le tooling tests included,
is listed in "Existing tests changed" below and approved by Thomas before the
edit. A test not in that table is not edited.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/le-tooling.md` - the door for `internal/le`
  → Constraint: its Gates table names `./le test weakened check`; that row goes with the verb
- [ ] `docs/architecture/testing/test-health.md`, section "The per-commit weakening record" - the design this spec replaces
  → Decision: the section is rewritten to describe the owner approval route; anchors on `PruneLanded`, `ForeignShardProblems`, `CarriedShards` are removed with those symbols
- [ ] `docs/contributing/testing.md`, section "When a test must be weakened" - the author's route
  → Decision: the three-step route (write row, edit, name shard) becomes: ask Thomas, record his words with the approve command, edit, commit; nothing extra is named in the commit
- [ ] `plan/learned/025-rfc-approval-lives-in-the-commit.md` - the precedent
  → Constraint: approvals keyed on the main thread's namespace (`lepath.SessionCommitSession`), file under `tmp/`, trailer appended unwrapped by `Message`, rows pruned by the script after `git commit` under `set -e` (`renderApprovalPrune`), a row whose whole trailer line already landed approves nothing further (`trailerLanded`)
  → Constraint: a body line beginning with a declared trailer key is refused (`refuseHandWrittenTrailer`, run inside `wrapBody`); adding the key to `declaredTrailers` is what extends that guard
- [ ] `docs/contributing/committing.md`, keyword and trailer tables
  → Decision: `Weakening-approved:` is documented beside `RFC-approved:`; no new `create` keyword (no owed-debt escape, see Key Design Decisions)

**Key insights:**
- The detector (`detector.go`, `scope.go`, `prospective.go`) is untouched: only what accepts a finding changes.
- `ParseLedger` and `RowMatches` (`ledger.go`) are shared with the RFC approval file and stay.
- The RFC approval mechanism already does everything the weakening path needs; this spec routes weakening through the same loop and the same writer.
- The audit's RFC branch is file-level today and accepts nothing, which is the three open journal rows.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/test/weakened/shard.go` - shard directory, per-session shard paths, `ReadShards`, `ForeignShardProblems`, `CarriedShards`, `LandedRows`, `PruneLanded` (unlocked read-rewrite)
- [ ] `internal/le/test/weakened/testweakened.go` - `Check`/`CheckCommit`: detector findings matched against shard rows; refuses unmatched findings, unmatched unlanded rows, and a commit that does not carry the shard; prints the shard population when given no paths; supports scoped rows (`liveScopeProblems`, `scopedRowMatches`, `isScopedRowName`) and bare names (`distinctPackages`)
- [ ] `internal/le/test/weakened/proposed.go` - the Write/Edit hook judge: RFC owner ledger first (`proposedApprovals`), then the weakening shard (`proposedLedger`, keyed on the AGENT's namespace `ProposedRequest.Session`); its footer calls the weakening row self-service
- [ ] `internal/le/test/weakened/audit.go` - `Audit` over `anchor..HEAD`: `acceptedRows` parses the shards each commit changed; `auditDiff` skips units a row accepts, and its `rfc.ChangedTags` branch emits a file-level WEAKENED with no acceptance check (`!rfcReported`); the final worktree pass has no rows
- [ ] `internal/le/test/weakened/actions.go`, `register.go` - verbs `check`, `selftest`, `proposed`
- [ ] `internal/le/test/weakened/selftest.go` - fixture proof writing a shard
- [ ] `internal/le/rfc/approve.go` - `Approve` writes `tmp/commit-rfc-approved-<ns>.md`, unit pattern `<package>.<TestName>`, one-line pipe-free reason, replaces an existing row for the unit
- [ ] `internal/le/commit/prepare.go` `checkSourceGates` - children, foreign-shard refusal, `CheckCommit(..., carried)`, RFC approvals, `PruneLanded` loop
- [ ] `internal/le/commit/rfcchange.go` - `rfcChangeProblems`, `trailerLanded`, `changedRFCUnits`, `tagFallsOutsideFunction`, `fileStem`
- [ ] `internal/le/commit/script.go` `renderApprovalPrune`, `commitBlock.ApprovalsPath/ApprovalsDropped` - one approval file per block
- [ ] `internal/le/commit/actions.go` `declaredTrailers`
- [ ] `internal/le/hookruntime/writeedit.go` `writeWeakening` - builds the agent namespace for the shard and the main-thread namespace for approvals
- [ ] `internal/le/le/path/commitsession.go` `ChildCommitSessions` - only non-test caller is `checkSourceGates`
- [ ] `internal/le/verify/engine/stages.go` - stage `test weakened/check`
- [ ] `internal/le/ai/hooks/fixtures.go`, `fixture_catalog.go` - weakening hook self-check probes write a shard; site messages cite `test/weakened/`; digests seal the catalog
- [ ] `internal/le/doc/citation/policed.go`, `internal/le/doc/check/retired.go` - `test/weakened/` prefix entries

**Behavior to preserve:**
- The detector's verdicts (which edits weaken which unit), its count-only notice, its rename pairing.
- The RFC approval route end to end: `./le rfc approve`, `RFC-approved:` trailers, `rfc-change-ok`, its push derivation.
- The hook and the commit gate read the same file with the same parser.
- `./le test weakened selftest` and `./le test weakened proposed` verbs and their exit codes (0 allow, 2 refuse).
- `./le commit audit` output shape and exit codes.

**Behavior to change:**
- A weakening is accepted only by an owner approval row in `tmp/commit-weakening-approved-<ns>.md` (or an `RFC-approved:` row for the same unit), never by a tracked file.
- The commit carries one `Weakening-approved: <package>.<TestName>: <owner words>` line per used row; the script prunes used and landed rows after `git commit`.
- `test/weakened/` stops existing in the tree; no gate reads or refuses it.
- `./le test weakened check` and its verify stage are removed.
- The audit accepts a unit named by a `Weakening-approved:` or `RFC-approved:` line of the audited commit, per unit, RFC branch included; the worktree pass reads the two live approval files.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Thomas answers in the main thread; the agent runs `./le test weakened approve unit <package>.<TestName> reason "<owner words>"`.
- An agent's Write/Edit on a test file (pretool hook, JSON payload on stdin).
- `./le commit create` over named paths.
- `./le commit audit base <rev>`.

### Transformation Path
1. Approve: validate unit and reason, write or replace one `| unit | reason |` row in the main thread's weakening approval file.
2. Hook: `Proposed` runs the detector over the proposed text; each finding must be named by a row of the weakening file or the RFC file; otherwise refuse with the approve command.
3. Commit: `checkSourceGates` runs `CheckCommit` over the prospective commit; each finding is matched against both approval files; each used weakening row becomes a trailer; landed rows (`trailerLanded`) and used rows are listed for the script's prune.
4. Message: trailers appended unwrapped after a blank line; hand-written trailer keys refused.
5. Script: `git commit`, then one prune step per approval file.
6. Audit: per commit, read the message, collect units from both trailer keys, accept findings and RFC unit changes those units name.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Hook payload → `testweakened.Proposed` | JSON on stdin, `ApprovalSession` from the main thread | No |
| `testweakened` → `rfc` | `testweakened` imports `rfc` (never the reverse) | No |
| `commit` → generated script | `commitBlock` approval list rendered into bash | No |

### Integration Points
- `rfc.Approve` - generalised to take the target path and header so one writer serves both files.
- `rfcChangeProblems` / `trailerLanded` - reused for the weakening rows.
- `declaredTrailers` - gains the new key, so `refuseHandWrittenTrailer` covers it.
- `changedRFCUnits` and helpers move from `commit` to `rfc` so the audit and the commit gate split RFC changes into units with one producer.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | hook and gate both call `testweakened`; the script prune stays the only writer that removes rows |
| No unintended coupling (components stay isolated) | Yes | import direction `commit → testweakened → rfc` unchanged; `changedRFCUnits` moves down into `rfc` |
| No duplicated functionality (extends existing, does not recreate) | Yes | one approve writer, one trailer-landed check, one unit splitter |
| Zero-copy preserved where applicable (refs, not copies) | N-A | tooling, no hot path |
| Registration over hardcoding, outbound | Yes | the new verb registers through `leaction.New` in `actions.go` |
| Registration over hardcoding, inbound | Yes | `declaredTrailers` is the one trailer list; `refuseHandWrittenTrailer` derives from it; no other switch names the key |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Keying approvals on the main thread namespace lets a subagent's edit see Thomas's approval | `writeWeakening` passes `ApprovalSession` from `CommitSessionFor(id, "")`; `rfc.approveAnswer` keys on `SessionCommitSession` | a subagent's weakening is refused though approved | `TestWeakeningApprovalReachesASubagentEdit` | confirmed: `TestWeakeningApprovalReachesASubagentEdit` (`internal/le/hookruntime/writeedit_test.go`) passes; `writeWeakening` keys `ApprovalSession` on `lepath.CommitSessionFor(root, id, "")`, and an approval under the agent's own namespace admits nothing |
| A-2 | No `.ci` or `.et` exercises `test weakened check` or `commit audit` | research grep: only a comment in `test/reload/signal-stop-cease.ci` | a functional test breaks | grep of `test/` at implementation | confirmed: no `.ci` or `.et` under `test/` names `weakened check`, `weakened/check` or `commit audit` (2026-10-24) |
| A-3 | Plan records citing `test/weakened/<shard>.md` are not policed for dead links | `citation.excludePrefixes` covers `plan/handover/`, `plan/journal/`, `plan/learned/`, `plan/records/`, every spec bucket | `./le doc check links` turns red | `./le doc check links` after the removal | confirmed by source, run owed with the new binary: every tracked file citing a `test/weakened/<8 hex>.md` path (`git grep`, 2026-10-24) sits under `plan/handover/`, `plan/journal/`, `plan/records/` or a bucket `spec-` prefix, all in `excludes()` (`internal/le/doc/citation/policed.go`); `./le doc check links` with the OLD bin/le, before the removal, flags no file this spec touched (its 10 dead references are `rfc/audit/` and `rfc/discrimination/` JSON paths) |
| A-4 | Moving `changedRFCUnits` into `rfc` creates no import cycle | `rfc` imports neither `commit` nor `testweakened` | the move fails to compile | `go build ./internal/le/...` | confirmed: `rfc.ChangedUnits` in `internal/le/rfc/changedunits.go`; `go vet ./internal/le/rfc/` clean, `gopls check` clean on `commit/rfcchange.go`, `commit/owed_answers.go`, `test/weakened/audit.go` |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Other sessions hold uncommitted rows in shards (4 modified, 1 untracked on 2026-10-24) | `git status --short test/weakened` non-empty | never delete or overwrite them; the removal commit removes tracked shards whose working copy equals HEAD; modified and untracked ones are listed to Thomas by name with the holder check (`ai/rules/git-safety.md`); their next commit needs his approval |
| R-2 | The removal commit is refused by the old foreign-shard gate | `create` prints "is session ...'s ledger shard" | the code change and the shard removal land in the same commit, prepared by an `le` built from the changed tree; if refused, land the code commit first, then the removal |
| R-3 | A forged `Weakening-approved:` line in a body is read as approval | review | key added to `declaredTrailers`; `TestMessageRefusesDeclaredTrailerInBody` iterates it |
| R-4 | A landed approval row is reused for a second weakening of the same unit | second commit passes with no new approval | `trailerLanded` marks it used; `TestALandedWeakeningApprovalApprovesNothingFurther` |
| R-5 | A row survives because its commit failed, then approves a later edit Thomas did not see | row present after a failed script | prune runs only after `git commit` succeeds, as for RFC; a still-unused row is the owner's live approval and stays by design |
| R-6 | The audit's RFC unit split diverges from the commit gate's | audit and gate disagree on one commit | one producer (`changedRFCUnits` in `rfc`) called by both |
| R-7 | Hook self-check digests go stale after the catalog edit | `./le ai hooks unit` red on digest | re-seal `fixtureSiteDigest`/`fixtureCatalogDigest` in the same phase |
| R-8 | Deleting `ChildCommitSessions` breaks a caller research missed | compile error | `gopls references` before deleting |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Agent commits that weaken tests are refused or wrongly admitted; nothing user-visible in the shipped binary |
| How is it reverted? | Single commit revert; the shards come back from history |
| Who else touches this path? | Any session committing a test weakening; `spec-gates-share-one-semantic-population` (owned the three audit rows), `spec-snapshot-builds-and-built-commits` (its AC-8 names `PruneLanded`), `spec-prose-cites-symbols-not-facts` (owns the shard citation row) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le test weakened approve unit X reason Y` | → | approve writer | `TestWeakeningApproveWritesTheSessionFile` |
| Write/Edit hook payload weakening a test | → | `Proposed` reading the approval files | `TestProposedWeakeningNeedsAnOwnerApproval` |
| `./le commit create` over a weakening | → | `checkSourceGates` weakening approval loop and trailer | `TestCreateCarriesWeakeningApprovedTrailer` |
| generated commit script | → | prune of the weakening approval file | `TestScriptPrunesUsedWeakeningApprovals` |
| `./le commit audit base <rev>` | → | `acceptedRows` reading commit trailers | `TestAuditHonoursApprovalTrailersOfTheAuditedCommit` |
| Hook self-check | → | `weakened-*` sites against the approval file | `./le ai hooks unit` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `./le test weakened approve unit pkg.TestX reason "words"` | one row naming `pkg.TestX` with reason `words` in `tmp/commit-weakening-approved-<main ns>.md`; a second call for the same unit replaces the reason |
| AC-2 | approve with a bare name, a path, an empty reason, or a reason holding a pipe or a newline | refused, exit 2, nothing written |
| AC-3 | Write/Edit that weakens `pkg.TestX` with no approval row | refused, exit 2; the message names the unit, says Thomas must be asked first, and prints the approve command |
| AC-4 | the same edit after approval, or with an `RFC-approved:` row for `pkg.TestX` | allowed, exit 0 |
| AC-5 | the same edit made by a subagent after the main thread recorded the approval | allowed |
| AC-6 | `./le commit create` over a weakening with no approval row | refused, naming the unit and the approve command |
| AC-7 | `create` over an approved weakening | message ends with `Weakening-approved: pkg.TestX: words` (unwrapped); no file is named or staged for it |
| AC-8 | the script after a successful commit | the used row is gone from the approval file; other rows stay |
| AC-9 | a second commit weakening `pkg.TestX` with the row whose trailer already landed | refused; the message names the carrier commit |
| AC-10 | a `body` line starting `Weakening-approved:` | refused by `create` |
| AC-11 | `./le commit audit` over a commit carrying `Weakening-approved: pkg.TestX: ...` that weakened `TestX` | no WEAKENED finding for it |
| AC-12 | `./le commit audit` over a commit carrying `RFC-approved: pkg.TestR: ...` that changed RFC-tagged `TestR` | no finding for it; an RFC-tagged unit changed in the same commit with no trailer is still reported |
| AC-13 | `./le commit audit` worktree pass with a live approval row for a weakened unit | not reported |
| AC-14 | the tree after the work | no tracked file under `test/weakened/`; `./le test weakened check` is unknown; `./le verify list full` has no `test weakened/check` stage |
| AC-15 | grep of `internal`, `docs`, `ai`, `.claude` for `test/weakened` | no hit except the historical records named in Deliverables |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestWeakeningApproveWritesTheSessionFile` | `internal/le/test/weakened/approve_test.go` | AC-1 | |
| `TestWeakeningApproveRefusesBadUnitOrReason` | `internal/le/test/weakened/approve_test.go` | AC-2 | |
| `TestProposedWeakeningNeedsAnOwnerApproval` | `internal/le/test/weakened/proposed_test.go` | AC-3, AC-4 | |
| `TestProposedRFCApprovalAcceptsTheUnitsWeakening` | `internal/le/test/weakened/proposed_test.go` | AC-4 (RFC row) | |
| `TestWeakeningApprovalReachesASubagentEdit` | `internal/le/hookruntime/writeedit_test.go` | AC-5, A-1 | |
| `TestCreateRefusesAnUnapprovedWeakening` | `internal/le/commit/ledger_test.go` | AC-6 | |
| `TestCreateCarriesWeakeningApprovedTrailer` | `internal/le/commit/ledger_test.go` | AC-7 | |
| `TestScriptPrunesUsedWeakeningApprovals` | `internal/le/commit/ledger_test.go` | AC-8 | |
| `TestALandedWeakeningApprovalApprovesNothingFurther` | `internal/le/commit/ledger_test.go` | AC-9, R-4 | |
| `TestMessageRefusesDeclaredTrailerInBody` (existing, iterates `declaredTrailers`, unchanged) | `internal/le/commit/input_test.go` | AC-10 | |
| `TestAuditHonoursApprovalTrailersOfTheAuditedCommit` | `internal/le/test/weakened/audit_test.go` | AC-11, AC-12 | |
| `TestAuditWorktreePassReadsLiveApprovals` | `internal/le/test/weakened/audit_test.go` | AC-13 | |
| `TestAuditReadsPastAProseLineThatStartsWithATrailerKey` | `internal/le/test/weakened/approval_unit_test.go` | AC-11 (a message line naming no unit approves nothing and blocks nothing) | second review, issue 1 |
| `TestAuditRefusesAnApprovalNamingTwoDirectoriesUnits` | `internal/le/test/weakened/approval_unit_test.go` | AC-11 (one unit in two directories is admitted by no line) | second review, issue 2 |
| `TestCreateRefusesAnApprovalNamingTwoDirectoriesUnits` | `internal/le/commit/ledger_test.go` | AC-6 (weakening and RFC rows: one unit in two directories is refused) | second review, issue 2 |
| `TestRFCChangeOKAdmitsNoUnitInTwoDirectories` | `internal/le/commit/ledger_test.go` | AC-6 (`rfc-change-ok` admits no unit changed in two directories) | third review |
| `TestAMovedWeakenedTestIsOneSite` | `internal/le/commit/ledger_test.go` | AC-6, AC-7 (a weakened test moved between directories is one site and its one approval admits it) | third review |
| `TestTrailerLandedAnswersOnlyWhatGitCanAnswer` | `internal/le/rfc/trailerlanded_test.go` | AC-9, R-4 (only "no repository" and "no commit" answer not landed; any other git failure is a problem) | third review |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | no numeric input | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `./le ai hooks unit` weakening sites | `internal/le/ai/hooks/fixture_catalog.go` | an agent's weakening edit is refused until the owner approval is recorded, through the real hook binary | |
| `./le test weakened selftest` | `internal/le/test/weakened/selftest.go` | the checker refuses an unapproved weakening and accepts it once approved, on a fixture repository | |

### Interop Tests (Scope: protocol)
N-A: tooling, no protocol peer.

## Existing tests changed (OWNER APPROVAL REQUIRED BEFORE EACH EDIT)

Every row is an existing test this work edits or deletes. New tests are in the
TDD plan. Product tests are touched only in comments.

Owner approval: Thomas approved this table as written on 2026-10-24, at the
write gate. It covers exactly these rows and changes; any other existing test
stops the work for a new question. At the same gate he approved R-1's handling:
shards holding other sessions' uncommitted rows stay on disk and are reported.

Later approvals by Thomas on 2026-10-24, each for exactly the change stated:

| Test | Change approved |
|------|-----------------|
| `commit` comment in `owed_answers_test.go` | the stale `rfcTagPattern` name becomes `rfc.HasRequirementTag` |
| `commit.TestACarriedRedNamesTheRunThatClearsIt` | the fixture derives, from the verify engine's part dealing, a part that does not hold the stage under test and publishes that part green, instead of the hard-coded part 3 of 3; the assertions stay unchanged |
| `ai/hooks` `hookSourcesDigest` | re-sealed with `fixtureSiteDigest` and `fixtureCatalogDigest` |
| `ai/hooks` `probeLedgerRows` (`fixtures.go`) and `weakened.TestRowMatchesExactAndQualifiedFormsAreUnaffectedByScope` | the probe row names `<pkg>.TestLedgered`; the "plain name must still match" case is removed, because bare names are refused (review issue 1) |
| `weakened` helper `newParityRepository` (`parity_test.go`) | its incidental shard write is removed; no assertion involved |
| `weakened.TestCheckDoesNotReadLedgerWhenPopulationIsClean` | writes a malformed approval file instead of none, so a check that read the file would fail (stricter) |

### `internal/le/test/weakened`
| Test | Change | Why |
|------|--------|-----|
| `TestCheckPrintsEveryShardAndItsRows` | delete | tests the shard population, which no longer exists |
| `TestForeignShardProblemsAdmitsOnlyThisSessionsChildren` | delete | shard ownership only |
| `TestPruneLandedKeepsRowsWrittenAfterTheCheck` | delete | shard prune only; the script prune replaces it |
| `TestPruneLandedNeverDropsAnUnlandedRow` | delete | same |
| `TestScopedRowCoversEveryFindingUnderItsTreeAndCommitPasses` | delete | owner decision: no scoped approvals |
| `TestScopedRowCoveringNothingIsReportedAsLeftover` | delete | same |
| `TestScopedRowLeavesAFindingOutsideItsTreeUnexplained` | delete | same |
| `TestFileScopedRowCoversItsFileAndNotASiblingFile` | delete | same |
| `TestAScopeOverALivePathIsRefused` | delete | same |
| `TestScopedRowDoesNotMatchOutsideItsTree` | delete | same |
| `TestRowMatchesExactAndQualifiedFormsAreUnaffectedByScope` | edit: keep the exact and qualified cases, drop the scope cases | scopes go, `RowMatches` stays |
| `TestCheckQualifiesAmbiguousRemovedUnits` | delete | only bare names were ambiguous; units are always `<package>.<TestName>` now |
| `TestAuditExitCodesCoverCleanFindingAndCannotRun` | edit: drop the shard fixture from the baseline | incidental; assertions unchanged |
| `TestAuditHonoursWeakeningRowOnlyInTheCommitThatCarriesIt` | rewrite: acceptance comes from a `Weakening-approved:` line in that commit's message | same property, new carrier |
| `TestCheckMatchesProducerDiagnosticsAndExitCodes` | rewrite: expected messages name the approve command, not a shard row; drop the population text | the message changes with the route |
| `TestCheckFailsClosedForEveryLedgerShape` | edit: repoint the malformed-file cases at the approval file | same fail-closed property |
| `TestCheckDoesNotReadLedgerWhenPopulationIsClean` | edit: the absent file is the approval file | same property |
| `TestWeakenedAreaPublishesNativeActions` | edit: verbs become `selftest`, `proposed`, `approve` | `check` removed, `approve` added |
| `TestSelfTestCoversEveryDetectorVerdictThroughCheck` | edit only if the population cases change the counts | detector verdict counts unchanged |
| `TestProposedWriteRequiresMatchingWeakeningRow` | rewrite: the row is written by the approve writer into the approval file | same property, owner file |
| `TestProposedRefusesAMalformedNamespace` | edit: the namespace case moves from `Session` to `ApprovalSession` | the agent namespace field goes |
| `TestProposedEditAndMultiEditReconstructWholeFile` | edit: fixture writes the approval file | incidental |
| `TestProposedRFCChangeRequiresOwnerLedgerBeforeWeakeningLedger` | rewrite: an RFC approval alone admits the tagged unit's weakening | owner decision: one approval |
| `TestProposedInvalidUTF8LedgerFailsClosedAndBase64UsesReplacement` | edit: invalid bytes written to the approval file | same fail-closed property |
| `TestProposedCleansAPathThatLeavesItsDirectory` | edit: fixture writes the approval file | incidental |
| `TestProspectiveCommitPreservesSharedIndexAndPairsRenameWithSpaces` | edit: drop the incidental shard write | assertions unchanged |
| `TestProspectiveCommitIgnoresUnrelatedWorktreeTests` | edit: same | assertions unchanged |
| helper `proposedFixture` and constants `fixtureSession`, `fixtureShard` | edit: drop `Session`, point at the approval path | reaches the 4 `TestProposedRFCLock*` tests through the helper; their bodies stay |

### `internal/le/commit`
| Test | Change | Why |
|------|--------|-----|
| `TestTwoSessionsKeepTheirOwnLedgerRowsThroughCreate` | delete | two shards and the foreign-shard refusal |
| `TestTheParentCommitsItsSubagentsShardAndNoOneElses` | delete | child and foreign shards |
| `TestCreateStillRefusesAWeakeningItsOwnShardDoesNotName` | rewrite as `TestCreateRefusesAnUnapprovedWeakening` | same property, approve command in the message |
| `TestALandedRowIsDroppedRatherThanBlockingTheNextCommit` | rewrite as `TestALandedWeakeningApprovalApprovesNothingFurther` | the landed check becomes `trailerLanded` |
| `TestANoFindingsCommitKeepsASiblingsUnlandedRow` | rewrite: an unused approval row survives the script prune | same property, new file |
| helpers `ledgerHeader`, `newLedgerRepository`, `ledgerCreate`, `readLedgerFile` | edit: write the approval file | |
| `TestCommitSessionIsStableAndExplicitlyReplaceable` | comment only: drop "the two test-ledger shards" | stale comment |

### Other `internal/le` packages
| Test | Change | Why |
|------|--------|-----|
| `le/path` `TestChildCommitSessionsAreThisMainThreadsAgentsOnly` | delete | `ChildCommitSessions` loses its only caller and is deleted |
| `le/path` `TestAgentIdentityEncodingIsInjective` | edit: drop the `ChildCommitSessions` calls | same |
| `le/path` `TestSiblingAgentsGetDistinctCommitSessions` | comment only: drop "one weakened-test shard" | stale comment |
| `hookruntime` `TestWriteRuntimeWeakeningRefusesAnAgentWithNoSessionID` | edit: expected text drops "subagent's shard"; the refusal for a missing session id stays | the approval namespace still needs it |
| `hookruntime` `TestBashRuntimeExportsTheAgentIdentity` | comment only | stale comment |
| `ai/hooks` weakening sites in `fixture_catalog.go`, `weakenedHatchTree`, both digests | edit: probes write the approval file; messages name the approve command; re-seal digests | the 11 `weakened-*` sites keep their exit codes |
| `doc/check` `TestRecordTreesAreExcludedFromCitationPolicing` | edit: drop the `test/weakened/` row | the prefix leaves the exclusion list |
| `doc/citation` `TestPolicedKeepsCorpusFilesUnderRecordTrees` | edit: drop the `test/weakened/` row | same |
| `rfc` `TestRenameLeavesUnpolicedRecordsUntouched` | edit: use another record tree (`plan/journal/`) | same property |
| `verify/engine` `TestFullStagesMatchesNativeActionPopulation` | edit: drop `test weakened/check` | stage removed |
| `verify` `TestStageLogNamesFollowTheCommandName` | edit: drop its log-name row | stage removed |
| `internal/le` `TestEveryMakeProducerHasAReachableNativeAction` | edit: `ze-test-weakened-check` moves from ported to retired producers | verb removed |

### Product tests (comments only, no assertion changes)
| File | Change |
|------|--------|
| `internal/component/bgp/wireu/payload_fixtures_test.go` | file comment citing shard 49b0956f → the commit that carried that map |
| `internal/component/bgp/wireu/rfc4271_aspath_slot_test.go` | comment citing shard 49b0956f → the same commit |
| `internal/component/bgp/wireu/tombstone_test.go` | comment citing shard 49b0956f → the same commit |
| `internal/component/config/transaction/orchestrator_test.go` | two comments citing shard 4c26aef3 → the commit that carried it |
| `internal/plugins/iface/netlink/vlanqoslab_integration_linux_test.go` | comment citing the long-gone flat ledger → "the commit that made it" |

## Files to Modify
- `internal/le/test/weakened/testweakened.go` - acceptance from the two approval files; drop shards, scopes, bare names, population text
- `internal/le/test/weakened/proposed.go` - read both approval files under `ApprovalSession`; drop `Session`; refusal text names the approve command
- `internal/le/test/weakened/audit.go` - `acceptedRows` reads commit trailers; RFC branch per unit; worktree pass reads live files
- `internal/le/test/weakened/ledger.go` - drop `Row.Key`
- `internal/le/test/weakened/shard.go` - deleted
- `internal/le/test/weakened/actions.go`, `register.go` - drop `check`, add `approve`, ShortHelp
- `internal/le/test/weakened/selftest.go` - fixture writes the approval file
- `internal/le/rfc/approve.go` - writer takes path and header
- `internal/le/rfc/` new home of `changedRFCUnits`, `tagFallsOutsideFunction`, `fileStem`; `internal/le/commit/rfcchange.go` calls them there
- `internal/le/commit/prepare.go` - weakening approval loop, trailers, no shard logic
- `internal/le/commit/script.go` - prune one step per approval file
- `internal/le/commit/actions.go` - `declaredTrailers` gains `Weakening-approved: `
- `internal/le/hookruntime/writeedit.go`, `session.go` - drop the agent shard namespace and stale wording
- `internal/le/le/path/commitsession.go` - delete `ChildCommitSessions`
- `internal/le/verify/engine/stages.go` - drop the stage
- `internal/le/ai/hooks/fixtures.go`, `fixture_catalog.go`
- `internal/le/doc/citation/policed.go`, `internal/le/doc/check/retired.go`
- `internal/le/completeness_record_test.go` and the tests in "Existing tests changed"
- Docs and rules: see the Documentation checklist
- `test/weakened/*.md` - removed (R-1 governs the five held by other sessions)

## Files to Create
- `internal/le/test/weakened/approve.go` - the approve verb body
- `internal/le/test/weakened/approve_test.go`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | tooling, no YANG |
| YANG validation constraints | N-A | no YANG |
| YANG custom validators | N-A | no YANG |
| CLI commands/flags | Yes | `./le test weakened approve` in `internal/le/test/weakened/actions.go`; `check` removed |
| CLI grammar (keyword before value) | Yes | `unit <u> reason "<r>"`, same as `./le rfc approve` (`ai/rules/cli.md`) |
| Editor autocomplete | N-A | le actions, no YANG completion |
| Functional test for new RPC/API | Yes | hook self-check sites and `selftest`; no `.ci` exercises le gates |
| Pipe completeness | N-A | le action output, not a ze CLI command |
| Env var registration | N-A | none added |
| Doctor check for runtime dependencies | N-A | no runtime dependency; the `tmp/` file is le-only |
| Prometheus counters/metrics | N-A | none |
| BGP family surface | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | N-A | tooling only |
| 2 | Config syntax changed? | N-A | |
| 3 | CLI command added/changed? | N-A | ze CLI unchanged; le commands documented in rows 10 and 12 |
| 4 | API/RPC added/changed? | N-A | |
| 5 | Plugin added/changed? | N-A | |
| 6 | Has a user guide page? | N-A | |
| 7 | Wire format changed? | N-A | |
| 8 | Plugin SDK/protocol changed? | N-A | |
| 9 | RFC behavior changed? | N-A | |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` (weakening section), `docs/contributing/testing.md` "When a test must be weakened", `docs/architecture/testing/test-health.md` "The per-commit weakening record", `docs/architecture/testing/verify-freshness-scope.md` (stage list), `docs/contributing/committing.md` (trailer table), `docs/contributing/running-commands.md` (identity table and `ChildCommitSessions` paragraph), `docs/contributing/rfc-conformance-gates.md` (two mentions), `docs/contributing/rfc-implementation-guide.md` (approval section) |
| 11 | Affects daemon comparison? | N-A | |
| 12 | Internal architecture changed? | Yes | `ai/doors/le-tooling.md` Gates row; `.claude/hooks/README.md` gate table; `ai/INDEX.md` weakened row; rules: `ai/rules/points/testing/fix-code-not-tests/write-the-weakened-row-before-the-edit.md` (rewritten as the owner-approval point), `ai/rules/points/testing/manifest.md`, `ai/rules/points/testing/rfc-tagged-tests-blocking/never-edit-an-rfc-tagged-test-to-match-the-code.md`, `ai/rules/points/planning/directives/brief-every-subagent-with-its-spec-phase-and-rules.md`, then `./le ai rules render-update` and `./le ai rules condensed-update`; skills `ai/skills/ze-rfc.md`, `ai/skills/ze-implement.md`, then `./le ai sync` |
| 13 | Route metadata keys? | N-A | |
| 14 | Prometheus counters? | N-A | |
| 15 | Registered inventory changed? | N-A | ze inventory unchanged |
| 16 | Changed source files referenced by doc anchors? | Yes | derived by `./le spec citation anchors spec plan/post-release/engineering/spec-weakening-approval-lives-in-the-commit.md` at implementation; known: `test-health.md` anchors on `PruneLanded`, `ForeignShardProblems`, `CarriedShards`; `running-commands.md` anchors on `ChildCommitSessions` and `shard.go`. Unaffected: `docs/architecture/core-design.md` (names the `test weakened` area as a member of `test`, which stays) and `docs/features/ai-first.md` (does not describe weakening or approvals) |
| 17 | Existing docs show examples for this area? | Yes | the shard row examples in `test-health.md` and `testing.md` are replaced by the approve command |

`docs/architecture/testing/TEST-RELAX-AUDIT.md` is a historical record: it is left as is unless `./le doc check links` flags it.

## Implementation Steps

1. **Phase: Wiring** - register `approve`, remove `check` and its verify stage; write the wiring tests failing.
   - Tests: `TestWeakeningApproveWritesTheSessionFile`, `TestWeakenedAreaPublishesNativeActions` (approved edit)
   - Files: `actions.go`, `approve.go`, `rfc/approve.go`, `stages.go`
2. **Phase: Acceptance** - `Check`/`CheckCommit` and `Proposed` read both approval files; delete shard, scope and bare-name code; delete `ChildCommitSessions`.
   - Tests: `TestProposedWeakeningNeedsAnOwnerApproval`, `TestProposedRFCApprovalAcceptsTheUnitsWeakening`, `TestWeakeningApprovalReachesASubagentEdit`, and the approved edits of the `testweakened`, `le/path` and `hookruntime` tests
3. **Phase: Commit** - approval loop in `checkSourceGates`, trailer, `declaredTrailers`, script prune list.
   - Tests: the `commit` rows of the TDD plan and the approved `commit` edits
4. **Phase: Audit** - move `changedRFCUnits` into `rfc`; `acceptedRows` reads trailers; per-unit RFC acceptance; worktree pass.
   - Tests: `TestAuditHonoursApprovalTrailersOfTheAuditedCommit`, `TestAuditWorktreePassReadsLiveApprovals`, the approved audit edits
5. **Phase: Hook self-check and policing** - catalog sites, probes, digests, citation prefixes, completeness record, verify stage tests.
6. **Phase: Docs and rules** - every row of the Documentation checklist, regenerate rules and skills; product test comments.
7. **Phase: Removal and ownership** - remove tracked shards (R-1, R-2); report the held ones; take ownership of the five journal rows in "Owned journal rows" by removing their lines from `spec-gates-share-one-semantic-population.md`, `spec-snapshot-builds-and-built-commits.md` (its AC-8 clause naming `PruneLanded` goes) and `spec-prose-cites-symbols-not-facts.md`.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | an approval row is consumed only after `git commit` succeeds; a landed row never approves again |
| Correctness | the audit and the commit gate split RFC changes into units with the same function |
| Owner constraint | every existing test edited appears in "Existing tests changed", with Thomas's approval recorded in session state |
| Rule: never-destroy-work | no other session's shard deleted or overwritten |
| Rule: no-layering | no code path still reads `test/weakened/` |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| no tracked shard | `git ls-files test/weakened` empty |
| verb removed | `./le test weakened check` exits with unknown verb |
| stage removed | `./le verify list full` has no `test weakened` |
| new verb | `./le test weakened approve unit a.TestB reason "x"` writes the file |
| no live reference | grep for `test/weakened` over `internal`, `docs`, `ai`, `.claude` shows only `TEST-RELAX-AUDIT.md` and plan records |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | unit pattern, reason one line, no pipe, file under `tmp/` only |
| Forgery | hand-written `Weakening-approved:` refused at every emitted line |
| Fail closed | unreadable or malformed approval file refuses, never admits |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup, only if the test is in the approved table |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| An unlisted existing test needs a change | STOP and ask Thomas |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| check-cannot-see-the-change-it-looks-for | 2026-10-03 | `auditDiff` (`internal/le/test/weakened/audit.go`), run by `./le commit audit` |
| check-cannot-see-the-change-it-looks-for | 2026-10-11 | `Audit` and `auditDiff`, run by `./le commit audit base origin/main` |
| check-cannot-see-the-change-it-looks-for | 2026-10-15 | `auditDiff`, run by `./le commit audit base 224b0ef514~1` |
| concurrent-session-corruption | 2026-10-14 | `testweakened.PruneLanded` (`internal/le/test/weakened/shard.go`) |
| closure-deletes-a-cited-document | 2026-09-07 | `test/weakened/8c4ad6c3.md`, against the tracked-file sweep in `trackedSweep` |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Separate `tmp/commit-weakening-approved-<ns>.md` with its own trailer key | one shared approval file with a kind column | keeps `./le rfc approve` and the RFC push derivation untouched; the reader is the same parser |
| One writer: `rfc.Approve` generalised to a path and header | a twin writer in `testweakened` | one declaration of the row grammar and the replace-on-same-unit rule |
| An `RFC-approved:` row also accepts the unit's weakening | two approvals | owner decision |
| One `<package>.<TestName>` per approval, no scopes, no bare names | keep scopes | owner decision; same grammar as RFC |
| No owed-debt escape keyword | a `weakening-ok` like `rfc-change-ok` | the gate has none today, and an escape would let an agent commit a weakening Thomas never saw, which is the defect being removed |
| Delete `./le test weakened check` | keep it to parse the tmp files | the files are parsed when used, by the hook and the gate; a stage reading another session's tmp file judges nothing in the tree |
| Move `changedRFCUnits` into `rfc` | audit accepts any approved unit in the file | per-unit acceptance is correct; a file-level answer hides an unapproved unit beside an approved one |

## Known Limitations
- A worktree agent cannot see the main checkout's `tmp/` approval file, as for RFC approvals today; worktree agents never commit to main.
- `create ... session <ns>` naming an agent namespace reads that namespace's approval file, as for RFC today.
- A unit keeps only its directory's base name, so `cmd.TestX` names `TestX` in every `cmd` directory. A commit that changes one unit in more than one directory is refused by the commit gate, which names the directories and asks for the commit to be split; the audit lets no approval line admit either test. The owner grammar stays `<package>.<TestName>`.
- Two kinds of test cannot be approved at all: a `.ci` or `.et` file whose stem holds a dot (the unit then has two dots), and a test file at the repository root (its package side is empty). `approvalUnitPattern` (`internal/le/rfc/approve.go`) refuses both, at the writer and at every reader.

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
- [ ] Every user story has a working path and a passing test (N-A when Scope is tooling or docs, which delete that section)
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches, and `./le verify current fast`
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
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

---

## Implementation Summary

### What Was Implemented
- `./le test weakened approve unit <package>.<TestName> reason "<owner words>"` (`approveAnswer`, `Approve` in `internal/le/test/weakened/approve.go`) writes one row to `tmp/commit-weakening-approved-<main ns>.md` through the one writer `rfc.ApproveInto` (`internal/le/rfc/approve.go`).
- One loader, `LoadApprovals` (`internal/le/test/weakened/ledger.go`), serves the edit hook (`proposedApprovals`), the commit gate (`Check`, `rfcChangeProblems`) and the audit's worktree pass (`liveApprovalRows`). It fails closed on a malformed file or a bare unit, and marks a row whose trailer landed (`rfc.TrailerLanded`).
- `checkSourceGates` (`internal/le/commit/prepare.go`) carries one `Weakening-approved:` line per used row. A unit changed in two directories is refused (`SharedUnitProblems`). An RFC row that admits a weakening is carried once (`mergeRFCApprovals`). The script prunes each approval file after `git commit` (`renderApprovalPrune`, `commitBlock.Approvals`).
- `declaredTrailers` (`internal/le/commit/actions.go`) gains `Weakening-approved: `, so `refuseHandWrittenTrailer` refuses a forged line.
- The audit (`acceptedRows`, `auditFile.judge` in `internal/le/test/weakened/audit.go`) reads the audited commit's approval lines per unit. Its RFC branch splits units with `rfc.ChangedUnits` (`internal/le/rfc/changedunits.go`), which the commit gate also calls.
- Removed: `shard.go` (ReadShards, ForeignShardProblems, CarriedShards, PruneLanded), scoped and bare rows, `ChildCommitSessions`, the `check` verb, the `test weakened/check` verify stage, and 148 tracked shards plus the three transitional ones.

### Bugs Found/Fixed
- Review round 1: bare units admitted, a spent row reusable at the hook, the RFC/weakening key split in the audit, duplicate RFC trailers. Pinned by `TestABareApprovalUnitIsRefusedByEveryReader`, `TestProposedRefusesAnApprovalALandedCommitAlreadyUsed`, `TestAuditHonoursApprovalTrailersOfTheAuditedCommit`, `TestOneRFCApprovalCarriesOneTrailerForATaggedWeakening`.
- Review round 2: a prose message line could block every later audit, and one unit in two directories was admitted. Pinned by `TestAuditReadsPastAProseLineThatStartsWithATrailerKey`, `TestAuditRefusesAnApprovalNamingTwoDirectoriesUnits`, `TestCreateRefusesAnApprovalNamingTwoDirectoriesUnits`.
- Review round 3: `rfc-change-ok` admitted a two-directory unit, a moved test, and `TrailerLanded` exemption width. Pinned by `TestRFCChangeOKAdmitsNoUnitInTwoDirectories`, `TestAMovedWeakenedTestIsOneSite`, `TestTrailerLandedAnswersOnlyWhatGitCanAnswer`.

### Documentation Updates
- `docs/architecture/testing/test-health.md` "The per-commit weakening record" (anchors on `Check`, `loadApprovalFile`, `claimFinding`, `Approve`, `LoadApprovals`, `SharedUnits`, `acceptedRows`, `checkSourceGates`, `writeWeakening`, `renderApprovalPrune`, `TrailerLanded`); `docs/contributing/testing.md`; `docs/contributing/committing.md`; `docs/contributing/running-commands.md` (1d2c330f99); `docs/functional-tests.md`; `docs/contributing/rfc-conformance-gates.md`; `docs/contributing/rfc-implementation-guide.md`; `docs/architecture/testing/verify-freshness-scope.md`; `.claude/hooks/README.md`; `ai/INDEX.md`; `ai/doors/le-tooling.md`; three rule points and `ai/skills/ze-rfc.md`, regenerated.
- `./le doc check verify` 2026-10-25: "Documentation tests PASSED".

### Deviations from Plan
- AC-14: four shards stay tracked, `test/weakened/{485edca9,5122291c,920463e1,fd04e285}.md`. Each holds another session's uncommitted rows, and Thomas approved leaving them on disk and reporting them (R-1). Deleting them is his pending decision.
- 518a1e3b63 was made with the old binary, which had no `Weakening-approved:` key. It carries its approvals as the "Existing tests changed" tables of this spec, plus two `RFC-approved:` lines. So `./le commit audit base 518a1e3b63~1` still reports its approved test edits, and that commit's message cannot be changed. Each finding maps to an approved row (Pre-Commit Verification, AC-11).
- The stage removal in `internal/le/verify/engine/stages.go` and its two tests landed in another session's commit 711d82b4d2, because those files carried that session's hunks.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The old binary's edit hook refused new fixtures that hold `t.Skip(` inside a Go string. Three transitional shards were needed | `skipPattern` counts by regular expression over raw source | the same fixtures read `adding t.Skip` at file level in `./le commit audit base 518a1e3b63~1` | journal row 2026-10-25 in `plan/journal/gate-fires-outside-its-population.md`, owned by `spec-gates-share-one-semantic-population` |
| approach | The owned 2026-10-14 row (`PruneLanded` unlocked rewrite) was resolved by deleting its producer. The successor files keep the same unlocked read-rewrite | `rfc.ApproveInto` and `renderApprovalPrune` share no lock; a lost row fails closed | closure review | journal row 2026-10-25 in `plan/journal/concurrent-session-corruption.md`, owned by `spec-snapshot-builds-and-built-commits` (its AC-8) |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Weakening needs Thomas's approval, asked before the edit | Done | `Proposed`, `unapprovedProblem`, `ProposedReport.Text` | refusal text says "ask Thomas first" |
| His words in an untracked file the hook and the gate read | Done | `ApprovalPath`, `LoadApprovals` | `tmp/` |
| One `Weakening-approved:` line per test | Done | `claimFinding`, `checkSourceGates` | |
| No tracked ledger | Done | b55bbfea0a | four held shards per R-1 |
| Audit reads both approval keys | Done | `acceptedRows`, `auditFile.judge` | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestWeakeningApproveWritesTheSessionFile` | |
| AC-2 | Done | `TestWeakeningApproveRefusesBadUnitOrReason` | |
| AC-3 | Done | `TestProposedWeakeningNeedsAnOwnerApproval` | |
| AC-4 | Done | `TestProposedWeakeningNeedsAnOwnerApproval`, `TestProposedRFCApprovalAcceptsTheUnitsWeakening` | |
| AC-5 | Done | `TestWeakeningApprovalReachesASubagentEdit` | |
| AC-6 | Done | `TestCreateRefusesAnUnapprovedWeakening` | |
| AC-7 | Done | `TestCreateCarriesWeakeningApprovedTrailer` | |
| AC-8 | Done | `TestScriptPrunesUsedWeakeningApprovals` | |
| AC-9 | Done | `TestALandedWeakeningApprovalApprovesNothingFurther` | |
| AC-10 | Done | `TestMessageRefusesDeclaredTrailerInBody` | iterates `declaredTrailers` |
| AC-11 | Done | `TestAuditHonoursApprovalTrailersOfTheAuditedCommit` | |
| AC-12 | Done | same test; live: audit over 224b0ef514 | |
| AC-13 | Done | `TestAuditWorktreePassReadsLiveApprovals` | |
| AC-14 | Changed | `git ls-files test/weakened`, `./le test weakened check`, `./le verify list full` | four held shards, owner-approved (Deviations) |
| AC-15 | Done | `git grep test/weakened` over `internal docs ai .claude` | only `TEST-RELAX-AUDIT.md`, the Go package path, the selftest fixture path and one retired-producer reason |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| all 18 rows of the TDD table | Done | files named in the table | each `func Test...(` found by `git grep` 2026-10-25 |
| `./le ai hooks unit` | Done | `internal/le/ai/hooks/fixture_catalog.go` | 30/30 after `./le --update` |
| `./le test weakened selftest` | Done | `internal/le/test/weakened/selftest.go` | PASS after `./le --update` |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| every row of Files to Modify and Files to Create | Done | 518a1e3b63, b55bbfea0a, 1d2c330f99, and 711d82b4d2 for `stages.go` |

### Audit Summary
- **Total items:** 5 requirements, 15 AC, 20 tests, all planned files
- **Done:** all but AC-14
- **Partial:** 0
- **Skipped:** 0
- **Changed:** AC-14 (four held shards, owner-approved at the write gate, recorded in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| An agent cannot weaken a test without Thomas's recorded approval | functional (hook binary) | `./le ai hooks unit` 30/30: site `weakened-missing-row-refuses-the-edit` exits 2 and prints `./le test weakened approve unit ` and "ask Thomas first"; `weakened-row-opens-the-hatch` exits 0 |
| The commit is the record; no tracked ledger | functional | `./le test weakened selftest` PASS (it checks that each used approval becomes one trailer); `./le test weakened check` answers "no such action"; `git ls-files test/weakened` holds only the four owner-held shards |
| An owner-approved commit stops reading WEAKENED in later reviews | live audit | `./le commit audit base 224b0ef514~1` (2026-10-25) reports none of the eight units 224b0ef514's `RFC-approved:` lines name; `./le commit audit base 518a1e3b63~1` omits the two units its `RFC-approved:` lines name |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| One lock over the owner approval files' writer and prune | the shape predates this spec in the RFC route, and a lost row fails closed | `plan/post-release/engineering/spec-snapshot-builds-and-built-commits.md` (AC-8, owned row 2026-10-25) |
| `skipPattern` reads a `t.Skip(` inside a Go string as a skip | detector untouched by design (Key insights) | `plan/post-release/engineering/spec-gates-share-one-semantic-population.md` (owned row 2026-10-25) |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/weakening-approval-lives-in-the-commit-1f613300-e8c7-4c35-93e0-dab8663705f9.md` (60 files) |
| `./le spec review check` | clean: "OK (46 code files, clean, hashes match)" |
| Rounds | 4 |
| Reviewer lenses used | rounds 1-3: logic and wiring, edge cases and security, fail-closed guards; round 4 (closure, over 518a1e3b63 and 1d2c330f99): wiring, removed behavior, guard audit, forgery, style pass |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | bare approval unit admitted by a reader | `LoadApprovals` | round 1 fix, `TestABareApprovalUnitIsRefusedByEveryReader` |
| 2 | ISSUE | spent row still admitted an edit at the hook | `proposedApprovals` | round 1 fix, `TestProposedRefusesAnApprovalALandedCommitAlreadyUsed` |
| 3 | ISSUE | `Weakening-approved:` admitted an RFC-tagged change in the audit | `auditApprovals` | round 1 fix |
| 4 | ISSUE | two `RFC-approved:` lines for one row | `mergeRFCApprovals` | round 1 fix, `TestOneRFCApprovalCarriesOneTrailerForATaggedWeakening` |
| 5 | ISSUE | a prose message line refused every later audit | `acceptedRows` | round 2 fix |
| 6 | ISSUE | one unit in two directories admitted by one approval | `SharedUnits`, `checkSourceGates` | round 2 fix |
| 7 | ISSUE | `TrailerLanded` exemption had no test | `rfc.TrailerLanded` | round 3, `TestTrailerLandedAnswersOnlyWhatGitCanAnswer` |

### Run 4 (closure)
0 BLOCKER, 0 ISSUE. NOTEs: (a) the unlocked approval-file read-rewrite, homed above; (b) the detector's string-literal skip, homed above; (c) `rfc.TrailerLanded` returns both `carrier` and `landed`, and `landed` is always `carrier != ""`. That return shape moved unchanged from `commit.trailerLanded`. Style pass over every changed Go file: no `panic()`, every new loop is over a bounded file or message, names say what each value is.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/le/test/weakened/approve.go` | yes | `ls`: 3.0K, 2026-10-24 |
| `internal/le/test/weakened/approve_test.go` | yes | `ls`: 3.0K, 2026-10-24 |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1..AC-13 | each named test exists and passed | `git grep 'func <Test>('` finds all 18 (2026-10-25); scoped `go test -race` green for rfc, test/weakened, commit, hookruntime, le/path, ai/hooks, doc/check, doc/citation, verify, verify/engine (state file, third-review handoff) |
| AC-11 | every finding `./le commit audit base 518a1e3b63~1` reports in this spec's files is an approved edit | 25 findings: 22 name units that are rows of "Existing tests changed" (`readPrunedShard` is the helper of the approved deletion of `shard_prune_test.go`); the 3 file-level `adding t.Skip` findings (`ledger_test`, `writeedit_test`, `audit_test`) are fixture strings (journal row 2026-10-25). `internal/le/changescope_test.go` findings are another session's uncommitted edit |
| AC-14 | verb, stage and shards | `./le test weakened check` -> "no such action in test weakened: check"; `./le verify list full` has 0 lines naming `weakened`; HEAD `stages.go` has 0 `test weakened`; `git ls-files test/weakened` = 4 held shards |
| AC-15 | no live reference | `git grep test/weakened -- internal docs ai .claude` outside the package path and fixtures: `TEST-RELAX-AUDIT.md` (historical) and the retired-producer reason in `completeness_record_test.go` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `./le test weakened approve` | none (le tooling, no `.ci`) | `approve` registered in `actions` (`actions.go`), `TestWeakenedAreaPublishesNativeActions` |
| Write/Edit hook | hook self-check, `./le ai hooks unit` | `writeWeakening` -> `testweakened.Proposed`; 30/30 |
| `./le commit create` | none | `checkSourceGates` -> `testweakened.Check`; `TestCreateCarriesWeakeningApprovedTrailer` |
| `./le commit audit` | none | `Audit` -> `acceptedRows`; live run over 224b0ef514~1 |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `TestWeakeningApprovalReachesASubagentEdit` |
| A-2 | confirmed | no `.ci`/`.et` names `weakened check` or `commit audit` |
| A-3 | confirmed | `./le doc check links` 2026-10-25 with the new binary: 10 broken references, all `rfc/audit/` or `rfc/discrimination/` JSON paths, none under `test/weakened/` |
| A-4 | confirmed | `rfc.ChangedUnits` builds; `commit` and `test/weakened` call it |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| test-health.md: refused at commit time by `checkSourceGates` running `testweakened.Check` | `checkSourceGates` (`prepare.go`) calls `testweakened.Check` | yes |
| test-health.md: a row is pruned only after `git commit` | `renderBlock` renders `renderApprovalPrune` after the publish under `set -e` | yes |
| test-health.md: the audit admits a tagged unit's change only on `RFC-approved:` | `acceptedRows` appends to `accepted.rfc` only for `rfc.ApprovalTrailer` | yes |
| all anchors | `./le doc check verify` PASSED 2026-10-25 | yes |
