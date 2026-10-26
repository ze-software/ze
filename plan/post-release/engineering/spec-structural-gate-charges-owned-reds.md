# Spec: structural-gate-charges-owned-reds

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | tooling |
| Priority | P3 |
| Release reason | Developer tooling: the commit gate charges reds no agent can clear, and 286 of the 491 commits since 2026-10-23 carry a `Structural-red-ok:` trailer re-diagnosing the same foreign reds. No shipped behaviour changes |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The commit gate's structural check (`structuralGateReds`, `internal/le/commit/verification.go`) reads `tmp/ze-verify-failures.json`, the failure index the last verify run wrote over the shared working tree, and refuses a commit for every structural red it charges. Since 2026-10-23 (commit 2688449b1b derived verification debt from git, and the 10-24 rule says agents never run the full verify) three of its charges cannot be cleared by the agent it refuses:

| Charge today | Why no agent clears it |
|--------------|------------------------|
| A group that names no path ("blind", `Unattributed`) | Charged to EVERY commit, whatever it carries |
| A stage only the release check reaches | Its own refusal says it "clears only when the release check reaches it, which you never run" |
| A first-red run's unreached structural stage | Charged to every commit until a later run reaches it |
| A red fixed and rerun green through the stage's own command | The rerun publishes nothing to the index, so the red stays charged |

Measured on 2026-10-26 with `git log --since=2026-10-23 --format=%b | grep '^Structural-red-ok'`: 286 trailers in 491 commits. The trailer reasons name `doc wiring` 208 times, `arch tier` 156, `staticcheck` 65 and `colima.go` 59. Each is the same foreign red re-diagnosed.

The owner agreed this design (2026-10-26):

| # | Agreed design |
|---|---------------|
| D1 | A structural red is charged to a commit only when a failure group names a path the commit carries (the current `owned` case). Blind or unattributed groups, and stages only the release check reaches, are NOT charged at commit |
| D2 | Those uncharged reds are reported at session start next to the verification debt, and remain owed at push through the existing push derivation and the release check, which is Thomas's |
| D3 | A scoped rerun of a stage that passes clears that stage's entry in the failure index, so a red fixed and rerun green stops being charged |
| D4 | `structural-red-ok` stays the admission for an owned red. Its doc in `docs/contributing/committing.md` and every rule point that tells agents to justify foreign reds are updated in the implementation |
| D5 | No layering: the charging logic is replaced, with no fallback to the old one |

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/committing.md` - the admission table (`structural-red-ok`, `broken-head-fix`) and the hunks paragraph that names the structural gate
  → Constraint: `Structural-red-ok:` and `Broken-head-fix:` are audit-only trailers ("Read by the push derivation: no"); D1 changes only WHEN they are needed, never their trailer shape or the declared-trailer refusal in `Message`
  → Constraint: an admission no gate used records nothing (`TestMessage...` in `input_test.go` asserts `Structural-red-ok:` is absent when granted but unused); that stays true when fewer reds are charged
- [ ] `docs/architecture/testing/verify-freshness-scope.md` - the failure-index paragraph (the one beginning "`structuralGateReds` ... reports four sets"), the session-start debt paragraph and the scoped record section
  → Decision: the index names the whole full population; a stage a run did not hold carries the earlier verdict (`carryForward`), and a partial population's green never replaces a red another population judged (`JudgesInPart`). A standalone rerun must enter that model as a FULL judgement of one stage, never as a partial one
  → Constraint: the session-start line is one predicate (`owed`, `internal/le/commit/owed.go`) shared with `debt-status` and the `create` report; the uncharged-red report is a separate line and MUST NOT change what `owed` answers
- [ ] `docs/contributing/running-commands.md` - "Stage times, and stopping at the first red": the carried-red text, the "diagnose with" vs clearing-line text, and the statement that a stage's own rerun "publishes no index"
  → Constraint: every one of those sentences becomes false under D1 and D3 and is rewritten in the same piece of work (`ai/rules/documentation.md`)
- [ ] `docs/architecture/core-design.md` - the before-dispatch hook paragraph (`leroot.RegisterBeforeDispatch`)
  → Decision: an area that needs a turn on every le invocation registers from its own `register.go`; the root handler names no tool package. D3's recorder follows the same registration pattern as an after-dispatch hook
- [ ] `docs/architecture/testing/tracked-build-gate.md` - why `broken-head-fix` exists for the compiles red
  → Constraint: `repo compiles/check` reads HEAD, so its red is cleared BY the next commit; `broken-head-fix` must still admit the sole compiles red when it is charged (owned)

**Key insights:**
- The changed population (`changedStages`, `internal/le/verify/engine/stages.go`) holds only two structural kinds: `go lint/run/changed` and the six Staticcheck parts. `doc wiring`, `arch tier/check`, `arch iface-resolution`, `plugin boundary/check`, `repo compiles/check`, full `go lint/run` and `verify deps/evidence-vet` are full-only. So "only the release check reaches it" cannot mean "absent from the changed population": that reading would leave nothing chargeable but lint-changed and Staticcheck, and owned doc-wiring reds would go uncharged.
- After D3, a stage whose own command an agent may run is cleared by that command. "Only the release check reaches it" therefore means: a stage whose own command no agent runs. That set is declared on the stage (see Key Design Decisions, and question Q1 in the report).
- A standalone stage run publishes no change scope, and an unset scope judges the whole subject: `ScopeTagsKey` says "Unset means every row is judged, which is what a standalone gate invocation gets" (`internal/le/repo/changed/scope.go`). A standalone green is a full judgement of that stage.
- Verify runs each stage as a child `le` process (`RunAction`, `internal/le/verify/dispatch/dispatch.go`) with `ZE_VERIFY_MODE=1` set by `enterVerifyEnvironment` (`internal/le/verify/engine/run.go`). The recorder must stay silent inside a verify run, or the child would race the run's own index publication.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/commit/verification.go` (386L) - `structuralGateReds` reads the index; per structural stage: interrupted → `Unjudged`; `reached:false` without `outside-run` → `Charged` + `Unreached`; exit 0 → skip; else groups: a group with no usable path (`groupRelatedPaths` empty) is blind, a path covered by the commit (`failuregroup.Covers`) is owned; owned or blind → `Charged` (+ `Unattributed` when not owned), neither → `Foreign`. `redCause` and `clearingRuns` build the refusal text; `clearingRuns` answers "release check only" for a stage outside the changed population or judged in part by it
  → Constraint: `Unjudged`/`UnjudgedCauses` and `Unreadable` stay as they are (reported in the stale verify detail by `verificationState`, never charged)
- [ ] `internal/le/commit/prepare.go` - `checkVerificationGates` (around line 401): when the certificate is not FRESH, charged reds refuse unless `broken-head-fix` (sole `repo compiles/check`) or `structural-red-ok`; the refusal prints `charged for want of path attribution: ...` and each cause
  → Constraint: the FRESH short-circuit stays: a FRESH certificate for the commit's paths charges nothing
- [ ] `internal/le/verify/engine/artifacts.go` - `writeFailureIndex` merges under `lockFailureIndex` with `carryForward`; `artifactStage` carries `Reached`, `ExitCode`, `Groups`, `OutsideRun`, `CarriedFrom` (`carriedOrigin`: commit, run-dir, generated-at, mode), `Interrupted`, `Rerun`; `PublishFailureIndex` copies a worktree run's index into the checkout
  → Constraint: any new writer of the index takes `lockFailureIndex` and replaces the JSON file atomically, as `writeFailureIndex` does, because readers take no lock
- [ ] `internal/le/verify/engine/stages.go` - `Stage{Identity, Structural, Whole}`, `structural(...)`, `whole(...)`, `StagesForMode`, `JudgesInPart`, `BaseMode`, `Structural(mode)`
  → Decision: a per-stage fact is declared on the stage with a wrapper (`whole(...)` is the precedent), never in a list in the commit package
- [ ] `internal/le/verify/dispatch/dispatch.go` - `RunAction` re-execs le per stage; the child inherits `ZE_VERIFY_MODE=1`
- [ ] `internal/le/le/root/beforedispatch.go`, `dispatch.go` - `RegisterBeforeDispatch(area, hook)` panics on nil or a second hook per area; `RunBeforeDispatch` runs hooks in area order and is called by `internal/le/register.go` before `Dispatch`; `Dispatch` returns the exit code
- [ ] `internal/le/le/action/leaction.go` - `Action.WholeTree`: a terminal-only action (`admitWholeTree = human.RequireTerminal`); declared on `verify current`, `verify worktree` and the evidence action
- [ ] `internal/le/hookruntime/lifecycle.go` - `sessionStartSteps` (tree, specs, verification debt, session state, journal, derived artifacts, agent files) under a 3.5 s budget; `reportDerivedDebt` prints `lecommit.DebtWarning(root)`
  → Constraint: a step writes only into its own report and must not wait for slow work; reading one JSON file is within budget
- [ ] `internal/le/commit/push.go` - `DebtWarning`: `verification debt: <headline>, push refused until cleared` and at most five items (`debtWarningItemsMax`)
- [ ] `internal/le/commit/owed.go` - `deriveDebt`/`owed`: no read of the failure index; the push is refused until a scoped record (`./le verify changed`, green) covers what it publishes
  → Decision: D2's "remain owed at push" needs no new push code: a red in a changed-population stage reddens `./le verify changed`, which then writes no scoped record, so the push stays refused; a red in a full-only stage is the release check's
- [ ] `internal/test/fixture/misc_fixture_runner.go` - `verifyScopeWiringDriver` (functional `test/runner/verify-scope-wiring-attribution.ci`) asserts a foreign wiring red is not charged, an own-file red refuses, and a blind red IS charged (`unattributable-wiring-red-is-charged`)
- [ ] `internal/le/commit/commit_test.go`, `clearing_test.go`, `verification_interrupted_test.go` - the structural-gate unit tests (listed in the TDD plan)

**Behavior to preserve:**
- An owned red (a group names a carried path, `failuregroup.Covers`) in a chargeable stage refuses the commit with no admission, and `structural-red-ok "<reason>"` admits it with the `Structural-red-ok:` trailer
- `broken-head-fix` admits the sole charged `repo compiles/check` red with the `Broken-head-fix:` trailer
- A FRESH certificate charges nothing; an interrupted stage is never judged and never charged; an unreadable index or entry charges nothing and is reported
- `carryForward` semantics for verify runs: a partial population's green never replaces another population's red
- The `owed` predicate and the `verification debt:` line are unchanged
- The refusal names each charged stage, why, the command that diagnoses it and the command that clears it

**Behavior to change:**
- Blind groups, release-only stages and unreached stages are no longer charged (D1)
- The uncharged structural reds are printed at session start (D2)
- A green standalone run of a stage's own command rewrites that stage's index entry (D3)
- The refusal text drops `charged for want of path attribution`, and its clearing line names the stage's own command for any stage an agent may run (D3)

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le commit create ... file <path>` → `Create` → `checkVerificationGates` → `structuralGateReds(root, paths)`
- Session start hook → `sessionStartSteps` → new step beside `reportDerivedDebt`
- `./le <stage command>` typed by an agent → `internal/le/register.go` → `leroot.Dispatch` → (new) after-dispatch hooks with the args and exit code

### Transformation Path
1. Verify run (Thomas's, or `./le verify changed`) writes `tmp/ze-verify-failures.json` via `writeFailureIndex`/`PublishFailureIndex`
2. A standalone stage run that exits 0, outside any verify run, rewrites that one stage's entry under the index lock (D3)
3. `structuralGateReds` classifies every structural red as charged (owned, chargeable stage) or uncharged with its reason (names no path, foreign, release-only, unreached)
4. `checkVerificationGates` refuses or admits only the charged set
5. The session-start step reads the same classification with no commit paths and prints the uncharged set

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| leroot ↔ verify area | after-dispatch hook registered from `internal/le/verify/register.go`; leroot names no tool package | No |
| verify engine ↔ commit gate | the JSON failure index on disk, format `FailureIndexFormat` | No |
| commit package ↔ hookruntime | an exported function in `internal/le/commit` read by the session-start step, as `DebtWarning` is | No |

### Integration Points
- `failuregroup.Covers`, `groupRelatedPaths` - unchanged attribution
- `lockFailureIndex`, the atomic JSON write in `writeFailureIndex` - reused by the rerun writer
- `verifyengine.StagesForMode(Mode)` - the population the recorder matches an invocation against

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The rerun verdict enters the index through the engine's own locked writer |
| No unintended coupling (components stay isolated) | Yes | The recorder registers into leroot like `RegisterBeforeDispatch`; leroot imports no verify package |
| No duplicated functionality (extends existing, does not recreate) | Yes | Classification stays in `structuralGateReds`; the session-start report calls it with no paths |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Tooling, no wire path |
| Registration over hardcoding, outbound | Yes | After-dispatch hook registered per area from `register.go` |
| Registration over hardcoding, inbound | Yes | The release-only fact is declared on the stage in `stages.go` (wrapper like `whole(...)`); the commit package holds no stage-name list. Searched: `clearingRuns`, `inPopulation`, `structuralStages` derive from `verifyengine`; the recorder matches against `StagesForMode(Mode)` |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A standalone stage invocation judges the stage's whole subject | `ScopeTagsKey` description ("Unset means every row is judged, which is what a standalone gate invocation gets"), `internal/le/repo/changed/scope.go` | A narrowed green would clear a full red it never re-judged | `TestARunWithAPublishedScopeRecordsNothing`: the recorder refuses when `ScopeFileKey` or `ScopeTagsKey` is set | unvalidated |
| A-2 | Every stage child of a verify run sees `ZE_VERIFY_MODE` set | `enterVerifyEnvironment` sets it before stages run; `RunAction` children inherit the environment | A verify child would write the index mid-run, racing the run's own publication | `TestAStageInsideAVerifyRunRecordsNothing` | unvalidated |
| A-3 | The stage identity words (`Identity.Command` split on spaces, then `Identity.Args`) equal the argv le receives for that stage | `identity()` and `stageCommand` in the commit package build `./le ` + name with `/` → space | A typed rerun would not match and nothing would clear | `TestAStandaloneGreenStageRunClearsItsEntry` drives `Dispatch` with the words of a real population stage | unvalidated |
| A-4 | The 59 `colima.go` trailers are staticcheck or lint groups naming a path the committing session did not carry | trailer text; the 2026-10-23 index names `internal/...` files for those stages | They are owned reds and stay charged, so the trailer count drops less than expected | Recount trailers over the first 50 commits after landing, stated in the closure | unvalidated |
| A-5 | The release check (`./le verify worktree`) still reaches every full-only stage, so uncharged reds stay owed somewhere | `fullStages`, D2 | An uncharged red would be owed nowhere | `TestEveryUnchargedReasonNamesWhereItIsOwed`: each uncharged entry's text names `./le verify changed` or the release-check command | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A real breakage that names no path (blind) lands on main unrefused | Session-start report lists a blind red that grows across commits | D2 prints it every session; `./le verify changed` reddens on changed-population stages and holds the push; the release check catches the rest. This is the owner's accepted trade |
| R-2 | A standalone green written while another session is mid-edit clears a red that session's half-edit would still produce | A red reappears in the next verify run | The standalone run judged the same shared tree the index describes; the next verify run re-judges. Same exposure as a verify run today |
| R-3 | Two writers (a verify publication and a rerun) interleave | Torn or lost entry | The rerun writer takes `lockFailureIndex` and re-reads the index under the lock before replacing one entry |
| R-4 | A red standalone rerun is silently ignored, leaving a stale green | `./le verify reds` disagrees with the terminal | Decision: a red rerun leaves the entry unchanged and the recorder says so on stderr in one line naming the stage; its groups are printed to the terminal, not captured, so writing a red without groups would make an owned red blind |
| R-5 | The recorder runs on every le call and slows hooks | Session-start or pretool hooks slow down | The hook returns at once unless the argv matches a population stage; matching is a slice walk over about 50 identities, no I/O |
| R-6 | Existing tests encode the old charging (blind, unreached, carried-without-group charged) | They go red after the replacement | Each is rewritten to assert the agreed behaviour, listed by name below; the owner approved the behaviour change, and any edit that lowers what a test proves is put to the `ze-test-judge` skill before the edit |
| R-7 | `repo compiles/check` red names no path and becomes uncharged, so `broken-head-fix` loses its purpose | A compile break on HEAD lands under a later commit | The compiles stage emits package groups (`internal/le/repo/compiles/report.go` attributes by path); an unattributed compiles red is uncharged like any blind red and is in the session-start report |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Commits that should be refused land (tooling only), or commits keep being refused; nothing user-visible in ze |
| How is it reverted? | Single commit revert |
| Who else touches this path? | Any session editing `internal/le/commit`, `internal/le/verify/engine`, `internal/le/le/root`, or the hookruntime session-start steps |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `le commit create ... file theirs-unrelated` with a blind red in the index | → | `structuralGateReds` → `checkVerificationGates` | `test/runner/verify-scope-wiring-attribution.ci` (marker `unattributable-wiring-red-not-charged`) |
| `le commit create ... file mine.go` with a group naming `mine.go` | → | `structuralGateReds` owned case | `test/runner/verify-scope-wiring-attribution.ci` (marker `wiring-red-of-my-own-file-still-refuses`) |
| `le <stage command>` exits 0 outside verify | → | after-dispatch recorder → engine rerun writer | `test/runner/verify-stage-rerun-clears-red.ci` |
| session start | → | uncharged-red step | `TestSessionStartReportsUnchargedStructuralReds` (`internal/le/hookruntime/lifecycle_test.go`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Index holds a red structural stage whose only group names no path; commit carries any file | Commit is not refused; no `Structural-red-ok:` trailer is written even when `structural-red-ok` is given |
| AC-2 | Index holds a red structural stage with a group naming `mine.go`; commit carries `mine.go`; stage is not release-only | Commit refused naming the stage, its cause, the diagnosing command and the clearing command; `structural-red-ok "<reason>"` admits it with `Structural-red-ok: <reason>` |
| AC-3 | Same red, commit carries only `other.txt` | Commit not refused |
| AC-4 | Red group naming `mine.go` in a release-only stage (full `go lint/run`, a full Staticcheck part, `verify deps/evidence-vet`); commit carries `mine.go` | Commit not refused; the stage is listed uncharged with reason release-only and the release-check command for Thomas |
| AC-5 | First-red index with a structural stage `reached:false`, no `outside-run` | Not charged; listed uncharged with reason not reached and the command that reaches it |
| AC-6 | Carried red (`carried-from` set) whose group names a carried path, chargeable stage | Charged, refusal names the run it was carried from |
| AC-7 | Sole charged red is `repo compiles/check`, owned; `broken-head-fix "<reason>"` | Admitted with `Broken-head-fix: <reason>` |
| AC-8 | Session start with uncharged structural reds in the index | One line beginning `structural reds not charged to commits:` with the count, then at most five items each naming stage, reason and where it is owed (`./le verify changed` at push, or the release-check command for Thomas); nothing printed when none; an unreadable index says so on stderr and is never read as none |
| AC-9 | Session start: the `verification debt:` line and `owed` | Byte-identical to today for the same git state |
| AC-10 | `./le doc wiring` (or any population stage's exact words) exits 0, `ZE_VERIFY_MODE` unset, no scope key set | That stage's index entry becomes reached, exit 0, no groups, marked judged by a standalone run at HEAD with the command; every other entry unchanged; a later commit is not charged for it |
| AC-11 | Same invocation exits non-zero | Entry unchanged; one stderr line says the red rerun was not recorded and why |
| AC-12 | Stage invocation inside a verify run (`ZE_VERIFY_MODE` set), or with a scope key set, or with words that differ from the stage identity (extra or fewer args) | Index unchanged |
| AC-13 | No index exists when a standalone green runs | No index is created (nothing to clear) |
| AC-14 | A later verify run after a standalone green | `carryForward` treats the standalone entry as a full-population judgement: a partial run's red over it is held as that run's own red; a full run replaces it |
| AC-15 | Refusal text | Contains no `charged for want of path attribution`; for a chargeable stage the clearing line names the stage's own command (`stageCommand`) as the command that clears it |
| AC-16 | Every stage whose le action is declared `WholeTree` | Is declared release-only on its `Stage` (test-enforced) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestStructuralRedsChargeOnlyGroupsNamingACarriedPath` (replaces `TestStructuralRedsAttributeOnlyPathBearingGroups`) | `internal/le/commit/commit_test.go` | AC-1, AC-2, AC-3: owned charged, blind and foreign uncharged with reasons | |
| `TestAnUnreachedStructuralStageIsNotCharged` (replaces `TestAnUnreachedStructuralStageIsCharged`) | `internal/le/commit/commit_test.go` | AC-5 | |
| `TestAReleaseOnlyStageRedIsNotChargedWhenOwned` | `internal/le/commit/commit_test.go` | AC-4 | |
| `TestAPieceKeepsAnEarlierStructuralRedCharged`, `TestACarriedRedNamesTheRunThatClearsIt`, `TestACarriedRedWithNoRecordedCommitSaysSo` (rewritten: the red carries a group naming `mine.txt`) | `internal/le/commit/commit_test.go` | AC-6: carry-forward still charges an owned red | |
| `TestCreateRefusesChargedStructuralRedWithoutRecordedOverride` (kept) | `internal/le/commit/commit_test.go` | AC-2 through `Create` | |
| `TestCreateAdmitsBrokenHeadFixForAnOwnedCompilesRed` | `internal/le/commit/commit_test.go` | AC-7 | |
| `TestRefusalNamesTheStageCommandAsTheClear` (replaces the release-only assertions in `TestUnjudgedStageNamesNoAgentFullRun`, `TestAFullRedOnASharedStageNamesNoChangedClear`, `TestAFullRedOnAFullIdentityStageClearsThroughTheScopedRun`) | `internal/le/commit/clearing_test.go` | AC-15 and A-5 | |
| `TestEveryUnchargedReasonNamesWhereItIsOwed` | `internal/le/commit/verification_uncharged_test.go` | AC-8 text, A-5 | |
| `TestUnchargedRedsWarningIsEmptyWithNoIndex`, `TestUnchargedRedsWarningCapsAtFive` | `internal/le/commit/verification_uncharged_test.go` | AC-8 | |
| `TestSessionStartReportsUnchargedStructuralReds` | `internal/le/hookruntime/lifecycle_test.go` | AC-8, AC-9 wiring | |
| `TestAStandaloneGreenStageRunClearsItsEntry` | `internal/le/verify/engine/rerun_test.go` | AC-10, A-3 | |
| `TestAStandaloneRedRunLeavesTheEntry` | `internal/le/verify/engine/rerun_test.go` | AC-11 | |
| `TestAStageInsideAVerifyRunRecordsNothing`, `TestARunWithAPublishedScopeRecordsNothing`, `TestANarrowedInvocationRecordsNothing` | `internal/le/verify/rerun_hook_test.go` | AC-12, A-1, A-2 | |
| `TestAStandaloneGreenWithNoIndexWritesNothing` | `internal/le/verify/engine/rerun_test.go` | AC-13 | |
| `TestCarryForwardTreatsAStandaloneEntryAsFull` | `internal/le/verify/engine/carry_test.go` | AC-14 | |
| `TestRunAfterDispatchPassesArgsAndExitCode`, `TestRegisterAfterDispatchRefusesNilAndSecondHook` | `internal/le/le/root/afterdispatch_test.go` | hook registration | |
| `TestEveryWholeTreeStageIsReleaseOnly` | `internal/le/verify/engine/stages_structural_test.go` | AC-16 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Session-start item cap | 0..5 items, then `... and N more` | 5 | N-A | 6 (prints the overflow line) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `verify-scope-wiring-attribution` (changed: blind red not charged, marker `unattributable-wiring-red-not-charged`; foreign and owned cases kept) | `test/runner/verify-scope-wiring-attribution.ci`, driver `verifyScopeWiringDriver` | A commit touching only an unrelated file passes with a blind red in the index; a commit touching the failing file is charged | |
| `verify-stage-rerun-clears-red` (new) | `test/runner/verify-stage-rerun-clears-red.ci`, new driver in `internal/test/fixture/misc_fixture_runner.go` | In a scratch checkout, an owned red refuses `le commit create`; the agent runs the stage's own command green; the same `create` then passes with no admission | |

### Interop Tests (Scope: protocol)
N-A: tooling, no protocol peer.

## Files to Modify
- `internal/le/commit/verification.go` - replace the charging: `structuralReds` keeps `Charged`, `Causes`, `Unjudged`, `UnjudgedCauses`, `Unreadable`, and replaces `Foreign`/`Unattributed`/`Unreached` with one uncharged list (stage, reason, where owed); `redCause`/`clearingRuns` name the stage command as the clear for a chargeable stage; new exported uncharged-red warning for session start
- `internal/le/commit/prepare.go` - `checkVerificationGates` refusal text (drop `charged for want of path attribution`)
- `internal/le/verify/engine/stages.go` - release-only declaration on `Stage` (wrapper like `whole(...)`) for full `go lint/run`, the six full Staticcheck parts, `verify deps/evidence-vet`
- `internal/le/verify/engine/artifacts.go` - exported standalone-rerun writer (one entry, under `lockFailureIndex`, atomic write); `carryForward`/`judged` accept the standalone marker as a full judgement
- `internal/le/le/root/beforedispatch.go` (or a sibling `afterdispatch.go`) - `RegisterAfterDispatch`/`RunAfterDispatch`
- `internal/le/register.go` - call `RunAfterDispatch` with the args and exit code after `Dispatch`
- `internal/le/verify/register.go` - register the recorder
- `internal/le/hookruntime/lifecycle.go` - new session-start step after `verification debt`
- `internal/test/fixture/misc_fixture_runner.go` - flip the blind case in `verifyScopeWiringDriver`; add the rerun driver
- `test/runner/verify-scope-wiring-attribution.ci` - marker and header comment
- `internal/le/commit/commit_test.go`, `clearing_test.go`, `verification_interrupted_test.go` (check only), `input_test.go` (check only) - as in the TDD plan
- `docs/contributing/committing.md` - admission table row for `structural-red-ok` ("with a charged structural red" → an owned red), the hunks paragraph's structural-gate bullet
- `docs/architecture/testing/verify-freshness-scope.md` - the `structuralGateReds` paragraph, the session-start paragraph (new line), the standalone rerun
- `docs/contributing/running-commands.md` - the `checkVerificationGates` sentence near "It is a query, never a certificate", and the carried-red / "publishes no index" / clearing-line text in "Stage times, and stopping at the first red"
- `docs/architecture/core-design.md` - the after-dispatch hook beside the before-dispatch one
- `docs/architecture/testing/tracked-build-gate.md` - `broken-head-fix` applies to an owned compiles red
- `ai/rules/points/completion/directives/verification-debt-is-not-defect-debt.md` - `structural-red-ok` is for an owned red; foreign reds are not charged and need no reason
- `ai/rules/points/pre-release/never/what-stays-exactly-as-it-was.md` - check the "structural gate charged to your own commit" row still reads true (expected: no edit)
- `ai/rules/completion.md`, `ai/rules/pre-release.md`, `ai/rules/CORE.md`, `ai/rules/TRIGGERS.md` - regenerated by `./le ai rules update condensed`, never hand-edited
- `.claude/hooks/README.md` - check its structural-gate mention

## Files to Create
- `internal/le/verify/engine/rerun_test.go`
- `internal/le/verify/rerun_hook.go`, `internal/le/verify/rerun_hook_test.go` - the recorder (argv match, environment refusals, call into the engine writer)
- `internal/le/le/root/afterdispatch.go`, `afterdispatch_test.go` (if not folded into `beforedispatch.go`)
- `internal/le/commit/verification_uncharged_test.go`
- `test/runner/verify-stage-rerun-clears-red.ci`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | le tooling, no YANG |
| YANG validation constraints | N-A | no YANG |
| YANG custom validators | N-A | no YANG |
| CLI commands/flags | No | No new command or keyword; existing stage commands gain a side effect |
| CLI grammar (keyword before value) | N-A | no grammar change |
| Editor autocomplete | N-A | no new words |
| Functional test for new RPC/API | Yes | `test/runner/verify-stage-rerun-clears-red.ci`, `test/runner/verify-scope-wiring-attribution.ci` |
| Pipe completeness | N-A | no new structured answer |
| Env var registration | No | reads existing `ZE_VERIFY_MODE`, `ze.verify.scope.packages`, `ze.verify.scope.tags` only |
| Doctor check for runtime dependencies | N-A | no new runtime dependency; `tmp/ze-verify-failures.json` already exists |
| Prometheus counters/metrics | N-A | tooling |
| BGP family surface | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | le tooling, not a ze feature |
| 2 | Config syntax changed? | No | none |
| 3 | CLI command added/changed? | Yes | `docs/contributing/committing.md`, `docs/contributing/running-commands.md` (le commit refusal, stage reruns) |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | No | developer docs only (rows 3, 10, 12) |
| 7 | Wire format changed? | No | none |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | No | none |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/verify-freshness-scope.md` |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` (after-dispatch hook), `docs/architecture/testing/tracked-build-gate.md` |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/features/ai-first.md` (declared by `internal/le/hookruntime/lifecycle.go`): checked in phase 4, edited if it lists the session-start lines, otherwise named unaffected with that reason; `docs/functional-tests.md` (declared by `internal/test/fixture/misc_fixture_runner.go`): unaffected, the fixture-driver mechanism is unchanged (one driver's assertions change and one driver is added). Also run `./le spec citation anchors spec plan/post-release/engineering/spec-structural-gate-charges-owned-reds.md` in the implementation; known anchors on `verification.go`, `prepare.go`, `artifacts.go`, `beforedispatch.go`, `lifecycle.go` in the pages above |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | the refusal text quoted in `running-commands.md` and `verify-freshness-scope.md` |

Discovery (`ai/rules/repo-maintenance.md`): an agent finds the gate from `ai/INDEX.md` (structural gate row → `docs/contributing/committing.md`); `TestEveryWholeTreeStageIsReleaseOnly` keeps the release-only declaration from drifting from `leaction.WholeTree`; the functional tests prove the commit path.

## Implementation Steps

1. **Phase: Wiring** - `RegisterAfterDispatch`/`RunAfterDispatch` in leroot, called from `internal/le/register.go`; stub recorder registered from `internal/le/verify/register.go`; flip the blind marker in `verifyScopeWiringDriver` and write `verify-stage-rerun-clears-red.ci` (both fail)
   - Tests: `TestRunAfterDispatchPassesArgsAndExitCode`, `TestRegisterAfterDispatchRefusesNilAndSecondHook`, the two `.ci`
2. **Phase: Charging replaced** - release-only declaration in `stages.go`; `structuralGateReds` charges owned reds in chargeable stages only; uncharged list with reasons; refusal text; delete the old `Unattributed`/`Foreign`/`Unreached` paths (no fallback)
   - Tests: the commit-package rows of the TDD plan, `TestEveryWholeTreeStageIsReleaseOnly`
3. **Phase: Standalone rerun clears** - engine writer, `carryForward` marker, recorder argv match and refusals
   - Tests: `rerun_test.go`, `rerun_hook_test.go`, `TestCarryForwardTreatsAStandaloneEntryAsFull`
4. **Phase: Session-start report** - exported warning in `internal/le/commit`, new step in `sessionStartSteps`
   - Tests: `verification_uncharged_test.go`, `TestSessionStartReportsUnchargedStructuralReds`
5. **Phase: Docs and rules** - every page in Files to Modify, each in the phase whose code made it wrong; `./le ai rules update condensed`
6. **Evidence owed** - scoped package tests under `./le job run` for `internal/le/commit`, `internal/le/verify/...`, `internal/le/le/root`, `internal/le/hookruntime`, `internal/test/fixture`; the two `.ci` by name; `./le verify changed` over the closing commits. Discrimination: revert the owned-only condition and see `verify-scope-wiring-attribution` go red at the blind case; remove the recorder registration and see `verify-stage-rerun-clears-red` go red

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | A standalone green never clears a stage it did not judge in full (scope keys, `ZE_VERIFY_MODE`, argv mismatch each refuse) |
| Correctness | No code path still charges a blind, foreign, release-only or unreached red (grep for `Unattributed`, `Foreign`, `Unreached` in `internal/le/commit`) |
| Data flow | The commit package holds no stage-name list; release-only is read from `verifyengine` |
| Rule: no-layering | The old charging branches are deleted, not guarded behind a flag |
| Rule: principles (silent zero) | An unreadable index at session start is reported, never printed as "no uncharged reds" |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Owned-only charging | `test/runner/verify-scope-wiring-attribution.ci` green; unit tests above |
| Rerun clears | `test/runner/verify-stage-rerun-clears-red.ci` green |
| Session-start line | `TestSessionStartReportsUnchargedStructuralReds` |
| Docs updated | `grep -rn "charged for want of path attribution\|publishes no failure index\|publishes no index" docs ai internal` answers nothing stale |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Index JSON from disk: an unreadable entry charges nothing and is reported (unchanged); the rerun writer never writes an entry for a stage name outside the population |
| Fail-open | The recorder refuses on every doubt (scope set, verify mode, argv mismatch, red exit); a gate that cannot read the index says so |

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

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `plan/journal/stale-artifact-reused.md` | 2026-09-13 | `structuralGateReds` charges stale blind reds (`tier/check`, `iface-resolution`) to every commit |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| "Release-only" is a per-stage declaration in `stages.go` (full `go lint/run`, the six full Staticcheck parts, `verify deps/evidence-vet`), test-held to include every `leaction.WholeTree` stage | (a) absent from the changed population; (b) derive only from `leaction.WholeTree` | (a) makes `doc wiring`, `arch tier`, `repo compiles` uncharged even when owned, which empties the gate. (b) marks only evidence-vet, yet `ai/META-DECISIONS.md` says agents never run a full `go lint run` or a Staticcheck matrix piece on the busy host. The declaration states that fact once, beside the stage. Put to the owner as Q1 |
| A standalone green is recorded as a full-population judgement at HEAD naming the command | Record it under a new mode name | A new mode is refused by `BaseMode` and would break `carryForward`; an unset scope judges the whole subject (A-1), which is what the full population's entry means |
| A red standalone rerun leaves the entry unchanged | Record the red with no groups | The terminal run's groups are not captured; a group-less red reads as blind and would drop an owned red from the charge |
| Recorder is an after-dispatch hook registered by the verify area | Hard-code the recording into each stage action | Registration: no stage action learns about the index, and a new stage is covered with no edit |
| Session-start report is a separate line beside `verification debt:` | Fold into `DebtWarning` | `owed` must stay one predicate shared with push and `debt-status` (AC-9) |

## Known Limitations
- A blind red is no longer charged to anybody at commit; it is owed at push through `./le verify changed` for changed-population stages and at the release check otherwise. This is the agreed trade (D1, D2).

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
