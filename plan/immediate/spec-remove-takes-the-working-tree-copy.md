# Spec: a removal takes the working-tree copy with it

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | tooling |
| Depends | `plan/spec-commit-stages-in-a-private-index.md` (in-progress, same file `internal/le/commit/script.go`, uncommitted in this tree) |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-07 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Closing a spec removes it from Git and leaves a byte-identical untracked copy on
disk. Three closures have leaked this way across three sessions
(`plan/journal/removal-leaves-the-file-on-disk.md`, rows dated 2026-09-06 and
2026-09-07), and every leaked spec is then reported as open work.

**The commit machinery is not the defect.** `renderSharedIndexRepair` and
`renderPrivateIndex` (`internal/le/commit/script.go`) emit
`git update-index --force-remove`, which clears an index entry and leaves the
working tree alone. That is what `docs/contributing/committing.md` documents:
step 5 of "What the generated script contains" says "A removal no longer deletes
the working-tree file, so `rm` the file first". The code honors its contract.

The defect is that the closure route never performs the caller half of that
contract, and nothing detects the omission:

- `ai/skills/ze-close.md` tells the closing session to run
  `./le commit create append remove plan/<spec-name>` and never tells it to
  delete the file. Its own rationale states the opposite of what happens: "Two
  commits because removing the spec destroys the working copy." Commit B does
  not destroy the working copy. The instruction is written from a belief about
  the command that the command has never held.
- `ai/skills/ze-progress.md` gives a different wrong answer for the same act:
  it says to stage `git rm plan/spec-<name>.md`, which `ai/rules/git-safety.md`
  forbids outright as a direct call.
- `validateRemovePath` (`internal/le/commit/input.go`) checks only that the path
  is tracked, through `git ls-files --error-unmatch`. A `remove` declared over a
  file still sitting on disk passes silently, so nothing anywhere observes that
  the contract was broken.

The symptom an operator meets: `plan/spec-firewall-domain-group.md` is untracked
in this checkout, `f339bf0ee7` removed it, its bytes match
`6c403e84b3:plan/spec-firewall-domain-group.md`, and its header still reads
`| Status | in-progress |`. `./le spec status` lists
`firewall-domain-group 8/9 in-progress`, so a session asking what is in progress
claims a spec that closed four commits ago. `./le spec citation` counts the same
file and the session-start banner counts it too.

The goal is that a removal removes. This spec CHANGES the contract rather than
enforcing the current one: the generated script deletes the working-tree copy
after the block's commit succeeds, and only when Git demonstrably holds that
copy's exact content. A copy that differs is left where it is and named to the
operator. The three instruction sites are corrected to the new contract.

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/committing.md` - the page that states the contract this spec changes
  → Constraint: step 5 currently reads "A removal no longer deletes the working-tree file, so `rm` the file first", and the `remove` keyword row promises "One tracked path to delete". The change makes the second true and retires the first, so both are edited in the same work as the code (`ai/rules/documentation.md`). The page must state the NEW contract: the script deletes the copy after the commit, on proof, and leaves it otherwise.
  → Decision: the page already describes the drift note as staging "the working tree into a throwaway index", so the guard is written in vocabulary the page holds.
- [ ] `docs/features/ai-first.md` - declared by the `// Design:` header of `internal/le/commit/script.go`
  → Decision: it describes commit preparation at the `commit.Answer` level and says nothing about the working tree, so it is named as unaffected rather than edited.

### Skills and Instructions
- [ ] `ai/skills/ze-close.md` - step 6d, the two-commit closure, and its "Why one script, two commits" rationale
  → Constraint: commit A names `file plan/<spec-name>` "to preserve all implementation edits in git history". Under the new contract that inclusion is what MAKES commit B's deletion safe, so the skill must say so rather than leave it as bookkeeping.
  → Decision: the rationale sentence "removing the spec destroys the working copy" is false today and true after this change, so it is corrected to state what commit B does and what commit A must have carried for it to happen.
- [ ] `ai/skills/ze-progress.md` - the closure row telling the reader to stage `git rm plan/spec-<name>.md`
  → Constraint: `git rm` is a banned direct call (`ai/rules/git-safety.md`), and it is a second wrong answer to the same question. It is the sibling instruction of the one that caused the leak, so it is repaired in the same work (`ai/rules/principles.md`, the work a change owes is measured by what it can reach).
- [ ] `ai/INSTRUCTIONS.md` - "To delete a tracked file use plain `rm` and pass the path to `remove`"
  → Constraint: the `rm` half of that sentence stops being required. `CLAUDE.md` and `AGENTS.md` are generated from this file, and `ai/skills/*.md` mirror into `.claude/skills/*/SKILL.md`; both regenerate with `./le ai skills-sync` (`ai/rules/repo-maintenance.md`).

### Rules
- [ ] `ai/rules/never-destroy-work.md` - always-on, governs every deletion of a user-visible file
  → Constraint: the only deletion permitted without asking is one whose content Git provably holds. That is why the deletion runs AFTER the commit and only on a proven match, and why a divergent copy is left rather than deleted.
  → Decision: the rule also bans leaving a file undeleted as a workaround, so "report and never delete" is not an acceptable design either.
- [ ] `ai/rules/principles.md` - fail-closed guards, and the reach of a change
  → Constraint: the guard answers "delete" only on positive proof. A missing file, an unreadable file and a file Git cannot stage each produce no proof, so each leaves the file alone.
- [ ] `ai/rules/git-safety.md` - the sanctioned commit route
  → Constraint: the generated script is the only place the raw verbs are spelled, and the same verbs inside it are allowed. The deletion belongs in the emitted script.
- [ ] `ai/rules/simplicity.md` - the simplest fully correct answer
  → Decision: one guard, at one moment, on one predicate. A second check inside `create` is rejected below on correctness grounds, not only on cost.
- [ ] `plan/journal/removal-leaves-the-file-on-disk.md` - the recurrence record
  → Decision: the row of 2026-09-07 states the cost of leaving this alone: "every closure from now on either leaks or spends a turn on `never-destroy-work` deciding whether it may delete its own output". A design that keeps a manual `rm` in the routine path keeps paying that.

### RFC Summaries (Scope: protocol)
- Not applicable. Scope is tooling and no wire protocol is involved.

**Key insights:** (minimal context to resume after compaction)
- `git update-index --force-remove` is an index verb, and the page says so. The code is not wrong; the callers and the contract are.
- The script runs under `set -euo pipefail` (`composeScript`, `internal/le/commit/prepare.go`), so a section emitted after `git commit` runs only when that commit succeeded. At that moment Git holds the content, which is the whole safety story.
- A check inside `create` cannot use HEAD as its reference. In the closure flow, commit A has not run when commit B is prepared, so the spec legitimately differs from HEAD and a HEAD comparison would refuse every closure.
- `renderDriftNote` already compares the working tree against an index by staging into a THROWAWAY index and differencing `git ls-files -s` entry lines, and its comment records why `git update-index --refresh` is the wrong tool.
- `specpath.All` globs and `specpath.Find` stats the working tree by design: a spec exists on disk before it is ever committed, and `Claim` (`internal/le/spec/session/session.go`) resolves a session's spec that way.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/le/commit/script.go` - `renderBlock` emits, in order, the private index, `git commit` against it, and the shared-index repair. `renderPrivateIndex` seeds the private index with `git read-tree HEAD`, feeds the snapshot entries in, then emits `git update-index --force-remove` for `block.Removed`. `renderSharedIndexRepair` points the shared index at the new HEAD for `block.Paths`, emits the same verb for `block.Removed`, then deletes the private index file. `renderDriftNote` holds the throwaway-index comparison. Nothing here touches the working tree, by design.
- [ ] `internal/le/commit/prepare.go` - `Create` validates, snapshots, and writes the message and the script. Its only `os.Remove` deletes an empty message reservation. `composeScript` writes the `#!/bin/bash` and `set -euo pipefail` header and the `cd` to the checkout root.
- [ ] `internal/le/commit/input.go` - `validateRemovePath` refuses a path `git ls-files --error-unmatch` does not match. That reads the shared INDEX, so a path staged but never committed passes and is absent from HEAD. Nothing checks the working tree.
- [ ] `internal/le/commit/snapshot.go` - `snapshotIndexEntries` stages `block.Paths` into a temporary index at CREATE time, which writes those blobs into the repository object database. Removals get no snapshot.
- [ ] `internal/le/commit/commit_test.go` - `TestGeneratedBlockQuotesPathsAndCommitsFromItsOwnIndex` asserts the emitted text, including `git update-index --force-remove -- 'old name.txt'`.
- [ ] `internal/le/commit/snapshot_test.go` - `TestATwoBlockScriptCommitsEachBlockFromItsOwnIndex` runs a real two-block script through `bash` against a Git fixture and asserts the removal reached the commit. Its doc comment claims the test PREVENTS "a removal that deletes the working-tree file as a side effect". `runCommitScript`, `newCommitRepository`, `configureCommitAuthor` and `writeCommitFixture` are the fixture helpers a new run test reuses.
- [ ] `ai/skills/ze-close.md` - step 6d names commit A with `file plan/<spec-name>` and commit B with `remove plan/<spec-name>`, and the rationale below claims commit B destroys the working copy.
- [ ] `ai/skills/ze-progress.md` - the closure row instructing `git rm plan/spec-<name>.md`.
- [ ] `internal/le/spec/specpath/specpath.go` - `All` globs the bucket directories, `Find` stats the three bucket paths for one name. Both read the working tree.
- [ ] `internal/le/spec/session/session.go` - `Claim` resolves a spec name through `specpath.Find`, so a spec written but not yet committed must resolve from the filesystem.
- [ ] `internal/le/spec/status/specstatus.go`, `internal/le/spec/citation/speccitation.go` - both take their population from `specpath.All`.
- [ ] `docs/contributing/committing.md` - the keyword table and the eight numbered parts of a commit block.

**Behavior to preserve:**
- The private index isolation: the shared index is never written before `git commit` succeeds.
- The content binding: a block commits the blob each named path held when `create` ran, never the working tree at run time.
- The drift note, its wording, and its throwaway-index technique.
- Both `force-remove` lines, against the private and the shared index. They are correct and unchanged; the fix ADDS a section.
- `set -euo pipefail` and the failure ordering it gives.
- `validateRemovePath` staying an index test with no working-tree condition, so a caller who already deleted the file can still prepare the removal.
- The working tree as the authority for the spec population (see the ruling under Key Design Decisions).

**Behavior to change:**
- The `remove` contract. After a block's commit succeeds, the working-tree copy of each path in `block.Removed` is deleted when it is identical, in content and in mode, to what the commit removed. Every other copy is left in place and named on stderr with the reason. The caller is no longer required to delete the file first.
- The three instruction sites that describe the old contract or contradict it.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le commit create ... remove <path>` (or `remove-list <file>`), run by a closing session, by `/ze-progress`, or by the owner.
- Format at entry: repository-relative paths, validated by `normalizePath` and `validateRemovePath`.

### Transformation Path
1. `Create` (`internal/le/commit/prepare.go`) normalizes the removals into `commitBlock.Removed` and calls `composeScript`. No working-tree condition is added here.
2. `renderBlock` (`internal/le/commit/script.go`) turns the block into script text. No git command runs at this stage.
3. The operator runs the script with `bash`. The lines from `renderPrivateIndex` seed the private index from HEAD, capture the index entries of the removed paths, clear those entries, and the block commits.
4. The new section runs after the shared-index repair, compares the working-tree copies against the captured entries, deletes only the paths that match, and reports the rest.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Go (`renderBlock`) ↔ generated shell | Rendered text only. Nothing in `internal/le/commit` runs git or deletes a file for a removal | No |
| Generated shell ↔ Git object database | The captured entry lines name the blob Git holds after the commit | No |
| Generated shell ↔ working tree | One deletion per proven-identical path, and nothing else | No |
| `ai/skills/*.md` ↔ `.claude/skills/*/SKILL.md` | `./le ai skills-sync` regenerates the mirrors, and `ai/INSTRUCTIONS.md` regenerates `CLAUDE.md` and `AGENTS.md` | No |

### Integration Points
- `renderBlock` - gains one rendered section, emitted only when `block.Removed` is non-empty.
- `renderPrivateIndex` - gains the capture, emitted under the same condition, immediately before the existing `force-remove` line.
- `ai/skills/ze-close.md` - commit A's inclusion of the spec becomes the stated precondition for commit B's deletion.
- `ai/skills/ze-progress.md` - the `git rm` row becomes the native route.
- `ai/INSTRUCTIONS.md`, `docs/contributing/committing.md` - the contract statement.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | The deletion is emitted by the function that emits every other verb and runs inside the sanctioned script |
| No unintended coupling (components stay isolated) | No | `internal/le/commit` gains no import and no knowledge of `plan/`, specs, or any caller. The skills change, the package does not learn about them |
| No duplicated functionality (extends existing, does not recreate) | No | The comparison reuses the throwaway-index technique `renderDriftNote` established rather than adding a second way to ask the same question |
| Zero-copy preserved where applicable (refs, not copies) | No | Not applicable: string rendering on a cold path, once per prepared commit |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | No registry, switch, or central list is touched. `commitBlock.Removed` already carries the population |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `set -euo pipefail` makes the deletion section unreachable when `git commit` fails | `composeScript` (`internal/le/commit/prepare.go`) writes the header, and `TestABlockLeavesTheSharedIndexAloneWhenItsCommitFails` already relies on this ordering | A failed commit could delete a file Git does not hold, the exact loss this design exists to avoid | `TestAFailedBlockDeletesNothing` | unvalidated |
| A-2 | Two identical `git ls-files -s` entry lines mean the working-tree file and the removed content are the same file to Git, in content and in mode | `renderDriftNote` decides drift on the same comparison, and `snapshotIndexEntries` binds commit content to the same line format | A file differing in a way the entry line hides would be deleted | `TestADivergentWorkingTreeCopySurvivesTheRemoval` and `TestAModeOnlyDifferenceLeavesTheFile` | unvalidated |
| A-3 | When block B of a closure script runs, HEAD is commit A, which carries the spec's current content, so the captured entry matches the working-tree copy | `renderPrivateIndex` emits `git read-tree HEAD` per block, and `ai/skills/ze-close.md` requires commit A to name `file plan/<spec-name>` | The closure case would report instead of deleting, leaving the defect unfixed for its main caller | `TestATwoBlockScriptCommitsEachBlockFromItsOwnIndex`, extended to assert the removed file is gone | unvalidated |
| A-4 | A `remove` path can be tracked in the shared index and absent from HEAD, because `validateRemovePath` tests the index | `validateRemovePath` (`internal/le/commit/input.go`) | An unguarded design would delete a file whose content no commit holds | `TestARemovalOfAPathHeadDoesNotHoldDeletesNothing` | unvalidated |
| A-5 | The working tree is the correct authority for the spec population, so `specpath.All` and `specpath.Find` need no change | `Claim` (`internal/le/spec/session/session.go`) resolves a session claim through `specpath.Find`, and `/ze-spec` writes the spec file before it is ever committed | The file-existence test would be a second defect and would need its own spec | Ruling recorded under Key Design Decisions, re-checked at closure against the claim path | unvalidated |
| A-6 | No caller depends on a removed path's file surviving | The `remove` keyword is documented as "One tracked path to delete", and the callers are the two skills plus site republish through `remove-list` | A workflow that untracks a file while keeping it locally would break | Grep every `remove` and `remove-list` caller in `ai/skills/`, `.claude/`, and `docs/` before landing, and name each in the closure report | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A wrong edit to `renderBlock` stops every session in this checkout committing | The commit package tests go red, or a prepared script fails at its first line | The unit tests assert the emitted text before any run test executes it. Land the rendering change and the run tests in the same commit |
| R-2 | The guard deletes a file whose content diverged, losing an operator's edit | A run test where the file must survive goes red | The deletion set is an intersection: a path is deleted only on a matching entry line, never on the absence of a mismatch |
| R-3 | `set -u` aborts the script because the captured variable is unset in a block with no removals | Any script with a removal-free block fails immediately | The capture and the deletion section are emitted under the same `len(block.Removed) != 0` condition, so neither exists without the other |
| R-4 | A second block in an appended script reuses the first block's captured entries | A two-block script deletes the wrong file | Each block emits its own capture immediately before its own `force-remove`, and its own deletion section immediately after its own commit |
| R-5 | The throwaway index collides with the private index or with another script's | A run fails with a Git index error, or two concurrent scripts interfere | The throwaway index is named from the script path, which already carries a random suffix (`indexFileFor`), and it is deleted by the section that created it |
| R-6 | A large `remove-list`, as a site republish uses, overruns the shell argument limit | A run fails with "argument list too long" | The section reuses the same quoted list the existing `force-remove` line already carries, so it adds no path to any command line that did not already hold them all |
| R-7 | The three instruction edits land and the mirrors do not, so agents read the old text | `./le ai skills-sync` leaves a diff, or `.claude/skills/ze-close/SKILL.md` still carries the old sentence | Run `./le ai skills-sync` in the same work and name the regenerated files in the commit |
| R-8 | `plan/spec-commit-stages-in-a-private-index.md` is in-progress with uncommitted work in the same file | A lost hunk or a conflicting edit in `script.go` | Read the working-tree state of `script.go` before editing, and land this change on top of what is there rather than beside it |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Every commit in this checkout. `renderBlock` renders the script every session runs, and a wrong line stops all of them. A wrong guard deletes a working-tree file whose content is not in Git |
| How is it reverted? | Single commit revert. The change adds rendered lines and edits no existing one, so reverting restores the previous script text exactly. The instruction edits revert with it |
| Who else touches this path? | `plan/spec-commit-stages-in-a-private-index.md` (in-progress, uncommitted work in `internal/le/commit/`), `/ze-close` commit B as the highest-volume caller of `remove`, and any site republish using `remove-list` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le commit create ... remove <path>` writes a script | → | `renderBlock` emits the capture and the deletion section | `TestGeneratedBlockQuotesPathsAndCommitsFromItsOwnIndex` |
| The operator runs that script with `bash` | → | the emitted section deletes the working-tree copy | `TestARemovalDeletesTheWorkingTreeCopyItCommitted` |
| The operator runs a script whose removed path diverged after preparation | → | the emitted guard leaves the file and reports it | `TestADivergentWorkingTreeCopySurvivesTheRemoval` |
| A closure runs the two-block shape `/ze-close` prepares | → | block B deletes the spec commit A preserved | `TestATwoBlockScriptCommitsEachBlockFromItsOwnIndex` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A prepared commit removes a tracked path whose working-tree copy matches what the commit removes, and the script runs | The commit removes the path from Git AND no file remains at that path. `git status --porcelain` shows no entry for it |
| AC-2 | The working-tree copy diverged between preparation and the run | The commit still removes the path from Git, the file stays on disk with its bytes unchanged, stderr names the path and the reason, and the script exits 0 |
| AC-3 | The working-tree copy has the same content and a different file mode | The file stays on disk and is reported. Mode is part of the comparison |
| AC-4 | The path is already absent from the working tree when the script runs | The script succeeds, deletes nothing else, and writes no report line for that path |
| AC-5 | The working-tree copy cannot be read or cannot be staged | The file stays on disk and is reported. No proof, no deletion |
| AC-6 | The removed path is tracked in the shared index and absent from HEAD | No entry was captured for it, so the file stays on disk and is reported |
| AC-7 | `git commit` for the block fails, for example because the message file is missing | The script stops at the failed commit and deletes nothing from the working tree |
| AC-8 | A block names files to add and no removals | The rendered script contains neither the capture nor the deletion section, and runs unchanged |
| AC-9 | A removed path contains a space or a single quote | The rendered lines quote it the way `quotePaths` quotes every other path, and the file is deleted |
| AC-10 | The two-block closure shape: commit A carries `file plan/spec-x.md` and commit B carries `remove plan/spec-x.md` | After the run, the spec is absent from Git and absent from disk, and commit A still holds its content in history |
| AC-11 | `./le commit create ... remove <path>` is prepared while the file is still on disk | The command succeeds. No working-tree condition is added to `create`, and the caller is never told to delete the file first |
| AC-12 | An agent reads `ai/skills/ze-close.md`, `ai/skills/ze-progress.md` or `ai/INSTRUCTIONS.md` for how to remove a spec | Each names the native `remove` route, none instructs `git rm`, and none requires a manual delete. The `.claude/` mirrors carry the same text |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestGeneratedBlockQuotesPathsAndCommitsFromItsOwnIndex` | `internal/le/commit/commit_test.go` | The rendered block carries the capture before the `force-remove` and the deletion section after the shared-index repair, with the removed path quoted (AC-9) | |
| `TestABlockWithoutRemovalsRendersNoWorkingTreeDeletion` | `internal/le/commit/commit_test.go` | A block with `Paths` only renders neither new line, so `set -u` cannot meet an unset variable (AC-8, R-3) | |
| `TestAddAndRemoveValidationProtectExplicitStaging` | `internal/le/commit/commit_test.go` | Existing test, extended: `validateRemovePath` still accepts a tracked path whose file is present, so `create` gained no working-tree condition (AC-11) | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `len(block.Removed)` | 0 to n | 0 renders nothing, 1 and 2 render one section naming every removed path | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `TestARemovalDeletesTheWorkingTreeCopyItCommitted` | `internal/le/commit/snapshot_test.go` | A commit removes a tracked file, and the file is gone from disk afterwards (AC-1) | |
| `TestADivergentWorkingTreeCopySurvivesTheRemoval` | `internal/le/commit/snapshot_test.go` | The file changed after preparation. It survives with its bytes intact and stderr names it (AC-2) | |
| `TestAModeOnlyDifferenceLeavesTheFile` | `internal/le/commit/snapshot_test.go` | Content identical, execute bit set. The file survives and is reported (AC-3) | |
| `TestARemovalOfAnAbsentPathReportsNothing` | `internal/le/commit/snapshot_test.go` | The author already deleted the file. The run succeeds and says nothing about it (AC-4) | |
| `TestARemovalOfAPathHeadDoesNotHoldDeletesNothing` | `internal/le/commit/snapshot_test.go` | The path is staged and never committed. Nothing is deleted (AC-6) | |
| `TestAFailedBlockDeletesNothing` | `internal/le/commit/snapshot_test.go` | The message file is removed before the run. The commit fails and the file is still there (AC-7) | |
| `TestATwoBlockScriptCommitsEachBlockFromItsOwnIndex` | `internal/le/commit/snapshot_test.go` | Existing test, extended to the real closure shape: block A commits the spec file, block B removes it, and afterwards the spec is absent from Git and from disk while commit A still holds its content (AC-10) | |
| `.ci` functional test | N-A | `./le` is development tooling behind the `ze_le` build tag and is not a shipped CLI surface. The run tests above ARE the end-to-end path: each prepares a real commit with `Create`, runs the generated script with `bash` in a real Git repository, and reads the working tree afterwards | |

### Instruction Checks
| Check | Location | Validates | Status |
|-------|----------|-----------|--------|
| `grep -n "git rm" ai/skills/ze-progress.md` returns nothing | repository | The banned staging verb is gone from the closure instruction (AC-12) | |
| `grep -rn "destroys the working copy" ai/skills/ .claude/skills/` matches only the corrected sentence | repository | The rationale states what commit B does under the new contract, in the source and in the mirror (AC-12) | |
| `./le ai skills-sync` leaves no diff | repository | `CLAUDE.md`, `AGENTS.md` and the `.claude/skills/` mirrors carry the edited text (AC-12, R-7) | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | N-A | N-A | No wire protocol and no peer implementation. `ai/rules/interop-and-goal-validation.md` exempts tooling with no protocol peer | |

### Red-Phase Proof (BLOCKING)
Every test is written against the CURRENT code first and must go RED before the
rendering change lands. `TestARemovalDeletesTheWorkingTreeCopyItCommitted` and the
extended `TestATwoBlockScriptCommitsEachBlockFromItsOwnIndex` are the
discriminating pair: today the file survives every removal, so both fail on the
present tree. `TestADivergentWorkingTreeCopySurvivesTheRemoval` and
`TestAModeOnlyDifferenceLeavesTheFile` PASS today and would pass against a stub,
so each is also run against a deliberately unguarded rendering that deletes every
removed path, and each must go RED there. All four red results are recorded.

## Goal Validation

| Goal | Evidence that proves it |
|------|------------------------|
| A closure leaves no spec file on disk | The extended `TestATwoBlockScriptCommitsEachBlockFromItsOwnIndex` runs the exact two-block shape `/ze-close` prepares, observed RED on the unchanged tree and GREEN after, both outputs recorded |
| A removal removes the file for every caller, not only for closures | `TestARemovalDeletesTheWorkingTreeCopyItCommitted` on a plain single-block removal, RED then GREEN |
| No operator edit is destroyed | `TestADivergentWorkingTreeCopySurvivesTheRemoval` and `TestAModeOnlyDifferenceLeavesTheFile` observed RED against an unguarded rendering and GREEN against the guarded one |
| The guard fails closed | `TestARemovalOfAPathHeadDoesNotHoldDeletesNothing`, `TestARemovalOfAnAbsentPathReportsNothing` and `TestAFailedBlockDeletesNothing`: three ways to have no proof, three files left alone |
| The instructions no longer contradict the command | The three Instruction Checks above, run after `./le ai skills-sync` |
| Every session's commits still work | `./le verify worktree` green, and the commit that lands this change is itself prepared and run through the changed `renderBlock` |

## Files to Modify
- `internal/le/commit/script.go` - `renderPrivateIndex` emits the capture before the existing `force-remove`; `renderBlock` emits one new section after `renderSharedIndexRepair`; the new render function carries a doc comment stating why the deletion runs after the commit and why the guard is an intersection.
- `internal/le/commit/commit_test.go` - the script-text assertions and the `create`-accepts-a-present-file assertion (AC-8, AC-9, AC-11).
- `internal/le/commit/snapshot_test.go` - the six new run tests, the extension of `TestATwoBlockScriptCommitsEachBlockFromItsOwnIndex` to the closure shape, and the correction of that test's doc comment, whose PREVENTS clause claims a removal must not delete the working-tree file (`ai/rules/stale-comments.md`).
- `ai/skills/ze-close.md` - commit B's bullet states that the script deletes the spec file once the commit succeeds, and names commit A's `file plan/<spec-name>` as the precondition. The "Why one script, two commits" rationale is corrected.
- `ai/skills/ze-progress.md` - the closure row replaces `git rm plan/spec-<name>.md` with the native `remove` route.
- `ai/INSTRUCTIONS.md` - the sentence requiring a manual `rm` before `remove`.
- `docs/contributing/committing.md` - step 5, the new step in the numbered list, and the `remove` keyword row, all stating the new contract.
- `plan/journal/removal-leaves-the-file-on-disk.md` - the closing row naming this spec (`plan/journal/README.md`).
- Regenerated by `./le ai skills-sync`, never edited by hand: `CLAUDE.md`, `AGENTS.md`, `.claude/skills/ze-close/SKILL.md`, `.claude/skills/ze-progress/SKILL.md`.

## Files to Create
- None. The change is rendered lines, tests, and instruction edits in files that already exist.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No config surface. The behavior is unconditional inside an existing script |
| YANG validation constraints | N-A | No leaf added |
| YANG custom validators | N-A | No leaf added |
| CLI commands/flags | No | No keyword is added or changed. `remove` and `remove-list` keep their spellings and gain the behavior their documentation already promises |
| CLI grammar (keyword before value) | N-A | No new keyword |
| Editor autocomplete | N-A | No YANG leaf |
| Functional test for new RPC/API | N-A | No RPC. Coverage is the run tests in `internal/le/commit/snapshot_test.go`, which execute the generated script |
| Pipe completeness | N-A | The `./le commit create` response payload is unchanged |
| Env var registration | N-A | No env var |
| Doctor check for runtime dependencies | N-A | No new file path, socket, service, port, module, or binary. `git` and `bash` are already required by the existing script |
| Prometheus counters/metrics | N-A | Development tooling, no daemon state |
| BGP family surface (new SAFI / capability / attribute) | N-A | No SAFI, capability, or attribute |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Development tooling behind the `ze_le` build tag, absent from a shipped build, so `docs/features.md` does not list it |
| 2 | Config syntax changed? | No | No config is read or written |
| 3 | CLI command added/changed? | Yes | `docs/contributing/committing.md`, the `remove` keyword row. `docs/guide/command-reference.md` documents the shipped `ze` surface and does not carry `le` |
| 4 | API/RPC added/changed? | No | No RPC |
| 5 | Plugin added/changed? | No | No plugin |
| 6 | Has a user guide page? | Yes | `docs/contributing/committing.md` is that page and it is edited |
| 7 | Wire format changed? | No | No wire format |
| 8 | Plugin SDK/protocol changed? | No | No SDK surface |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No RFC governs a commit script |
| 10 | Test infrastructure changed? | No | The new tests use the existing `runCommitScript` fixture helpers and add no infrastructure |
| 11 | Affects daemon comparison? | No | Not a daemon feature |
| 12 | Internal architecture changed? | Yes | `docs/contributing/committing.md`, "What the generated script contains". `docs/features/ai-first.md` is declared by the `// Design:` header of `script.go`, describes commit preparation at the `commit.Answer` level only, and is named here as unaffected |
| 13 | Route metadata keys added/changed? | No | No route metadata |
| 14 | Prometheus counters added/changed? | No | No counters |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | No registration changes. The skill mirrors are regenerated, not registered |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED at implementation time: run `./le spec citation anchors spec plan/immediate/spec-remove-takes-the-working-tree-copy.md` and name every page it lists. `ai/CODE-TO-DOCS.md` already maps `internal/le/commit/script.go` to `docs/contributing/committing.md`, and `snapshot.go` declares the same page |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/contributing/committing.md` shows a spec-closure example using `remove`. Confirm it reads correctly once the "rm the file first" instruction is gone |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- assert the emitted text
   - Tests: `TestGeneratedBlockQuotesPathsAndCommitsFromItsOwnIndex` extended, `TestABlockWithoutRemovalsRendersNoWorkingTreeDeletion` added
   - Files: `internal/le/commit/commit_test.go`
   - Verify: both FAIL, because `renderBlock` emits no capture and no deletion section. The entry point is `renderBlock` and the tests read what it produces
2. **Phase: Render the capture and the deletion**
   - Tests: the two above go green
   - Files: `internal/le/commit/script.go`. `renderPrivateIndex` captures the removed paths' index entries into a shell variable immediately before the existing `force-remove` line, while the private index still holds them from `git read-tree HEAD`. `renderBlock` appends one new section after `renderSharedIndexRepair`. That section stages the current working-tree copies of the removed paths into a throwaway index named from the script path, lists its entries, intersects them with the captured entries, deletes the paths in the intersection, reports every remaining removed path that still exists on disk to stderr, and removes the throwaway index. Both parts are emitted only when `block.Removed` is non-empty
   - Verify: the rendered text is what the unit tests assert
3. **Phase: Prove it on a real repository**
   - Tests: the six new run tests plus the extended two-block test, driven to the closure shape
   - Files: `internal/le/commit/snapshot_test.go`
   - Verify: the Red-Phase Proof above. Record the RED output of the two discriminating tests on the unchanged tree, and the RED output of the two guard tests against an unguarded rendering
4. **Phase: The three instruction sites and the page**
   - Files: `ai/skills/ze-close.md`, `ai/skills/ze-progress.md`, `ai/INSTRUCTIONS.md`, `docs/contributing/committing.md`, then `./le ai skills-sync`
   - The page must STATE the new contract, not merely drop the old sentence: `remove` deletes the tracked path and its working-tree copy, the deletion happens after the commit succeeds, it happens only when the copy matches what the commit removed, and a copy that differs is left and named on stderr
   - Verify: the three Instruction Checks, and `./le spec citation anchors spec plan/immediate/spec-remove-takes-the-working-tree-copy.md` leaving no page unexplained
5. **Phase: Confirm no caller wanted the file to survive (A-6)**
   - Grep every `remove` and `remove-list` caller across `ai/skills/`, `.claude/`, `docs/` and `internal/le/`, and name each in the closure report with what it removes
   - A caller that untracks a path while keeping the file would be broken by this change. If one exists, STOP and report it before landing
6. **Phase: The existing residue (OWNER GATE, no deletion without his word)**
   - The change repairs the future only. Nothing in it walks history
   - Enumerate the residue: for each untracked path under `plan/`, find the commit that removed it and compare the on-disk bytes against the blob that commit removed. Report the list with the commit and the blob for each
   - `plan/spec-firewall-domain-group.md` is the one known survivor. `f339bf0ee7` removed it and its bytes match `6c403e84b3:plan/spec-firewall-domain-group.md`. It belongs to another session's closure, so do NOT delete it: report it and ask the owner (`ai/rules/never-destroy-work.md`, precedence rung 1)
   - Build no sweep command. One known instance does not earn a tool, and the enumeration above is a git query

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has a named test, and AC-1 through AC-7 and AC-10 each have a run test rather than a text assertion |
| Feature completeness | The path from `./le commit create ... remove` to a deleted file is exercised end to end, and the closure shape specifically is one of those runs |
| Correctness | The deletion runs AFTER `git commit`, never before, so Git holds the content before any file is deleted |
| Correctness | No working-tree condition was added to `create`. A create-time check cannot use HEAD as its reference and would refuse every closure |
| Naming | The new render function names the working tree, so a reader does not confuse it with the two index removals |
| Data flow | Nothing in `internal/le/commit` executes a deletion. Go renders text, and the script the operator runs performs the deletion |
| Rule: `ai/rules/never-destroy-work.md` | The only file deleted automatically is one whose exact content the commit just made holds. Every other case leaves the file and tells the operator |
| Rule: `ai/rules/principles.md` | The guard is an intersection, so no error path can fall through to a deletion |
| Rule: `ai/rules/stale-comments.md` | The PREVENTS clause on `TestATwoBlockScriptCommitsEachBlockFromItsOwnIndex`, step 5 of `docs/contributing/committing.md`, and the `ze-close.md` rationale sentence all describe the old contract and are corrected here |
| Rule: `ai/rules/no-layering.md` | The manual `rm` is REPLACED, not kept beside the new behavior. No instruction offers both routes |
| Rule: `ai/rules/git-safety.md` | `ai/skills/ze-progress.md` no longer instructs a banned staging verb |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The rendered script carries the deletion section | `go test ./internal/le/commit/ -run TestGeneratedBlockQuotesPathsAndCommitsFromItsOwnIndex -v` |
| A removal leaves no file behind | `go test ./internal/le/commit/ -run TestARemovalDeletesTheWorkingTreeCopyItCommitted -v` |
| A closure leaves no spec behind | `go test ./internal/le/commit/ -run TestATwoBlockScriptCommitsEachBlockFromItsOwnIndex -v` |
| A divergent file survives | `go test ./internal/le/commit/ -run TestADivergentWorkingTreeCopySurvivesTheRemoval -v` |
| The closure instruction is right | `grep -n "git rm" ai/skills/ze-progress.md` returns nothing |
| The instructions match the code | `grep -n "use plain" ai/INSTRUCTIONS.md` returns nothing, and `./le ai skills-sync` leaves no diff |
| The page states the new contract | `grep -n "rm the file first" docs/contributing/committing.md` returns nothing, and the numbered list names the deletion step |
| The whole commit route still works | `./le verify worktree`, and this spec's own closure commit prepared and run through the changed renderer |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Removed paths are validated by `normalizePath` (no `..`, no `.git/`, no control character or line break) and by `validateRemovePath` (tracked). The rendered lines quote every path through `quotePaths`, which the existing test proves round-trips through `parseShellWords` |
| Command injection | The paths reach the script as single-quoted shell words, never as unquoted interpolation, and `shellQuote` escapes an embedded quote |
| Destructive scope | The deletion names exactly the paths in `block.Removed` that matched. No glob, no directory, no recursion |
| Fail open | The guard's default answer is "do not delete". Confirm no error path falls through to a deletion |
| Privilege of the new act | The script now deletes files. Confirm it can only delete a path the caller explicitly named as a `remove`, and never a path derived from the working tree |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| Step 5 finds a caller that wants the file kept | STOP. The contract change is wrong for that caller. Report before landing |
| A prepared script fails at run time in this checkout | STOP. Every session commits through this script. Report before touching anything else |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
- An instruction that contradicts the command it invokes is a defect with no test that can see it. `ze-close.md` has said "removing the spec destroys the working copy" for as long as the command has not done that, and three sessions followed it. The repair is the instruction AND a command whose behavior makes the instruction true, because only the second one holds after the next edit to the first.
- The safety of an automatic deletion is decided by its ORDER as much as by its guard. After the commit, the content is one `git show` away, so the deletion is recoverable by construction. Before the commit, only a blob hash buried in the script holds it.
- Guarding on EXISTENCE and guarding on DIVERGENCE look like the same guard and train opposite habits. A refusal that fires whenever the file is present teaches the caller to delete without looking, because the fix is always the same. A guard that fires only when content would be lost stops the caller exactly when stopping is warranted.

## Key Design Decisions

The three options considered for where the guard lives, and why the chosen one wins:

| Option | What it does | Verdict |
|--------|--------------|---------|
| (i) `create` refuses a `remove` whose working-tree file still exists | Enforces the current contract. Simple, one predicate, no script change, no page change | REJECTED. The predicate is EXISTENCE, so the refusal's only remedy is "delete it and retry", whether or not the file holds uncommitted work. It makes every closure perform a manual deletion of a user-visible file, which is a rung-1 act (`ai/rules/never-destroy-work.md`) placed in the routine path and repeated forever. And in the closure flow that deletion necessarily precedes commit A, because both blocks are prepared before the one script runs, so the content lives only as a blob hash inside the script text at the moment it is deleted |
| (ii) The working-tree copy is deleted when it matches what the commit removed, and left with a report when it does not | CHOSEN, with one correction to where the test runs (below) | The predicate is DIVERGENCE, which is the thing that can be lost. The deletion happens after the commit, where Git already holds the content, so the routine path performs no unrecoverable act and no session spends a turn on `never-destroy-work` deciding whether it may delete its own output |
| (iii) Repair `ai/skills/ze-close.md` only, leaving `create` and the script alone | Cheapest, and it does address the proximate cause | REJECTED as a complete answer. A skill instruction is not a guard: it works only while every reader obeys it, and three sessions already did not. `ai/skills/ze-progress.md` is a second instruction for the same act that gets it wrong a different way, which is the evidence that the instruction layer does not hold. The instruction repair is kept, as part of the chosen option rather than instead of it |

**The correction to (ii): the divergence test cannot run inside `create`.** As
specified it would compare the working-tree file against HEAD when the commit is
prepared. In the closure flow that comparison refuses EVERY closure: `/ze-close`
prepares commit A and commit B into one script before either runs, commit A is
what carries the spec's edited content, and at preparation time HEAD still holds
the pre-edit version. The spec legitimately differs from HEAD at exactly the
moment `create` would judge it. The test therefore runs in the SCRIPT, right
after the block's commit, where the reference is exact: the private index was
seeded by `git read-tree HEAD` for that block, so for block B of a closure HEAD
is commit A and the captured entry is the spec's committed content.

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The deletion is rendered into the generated script and runs after the block's commit | Perform it in Go, inside `Create` | `Create` prepares. The script can be a dry run, can never be run, and can fail at any earlier line, so a deletion at preparation time removes content no commit holds. `ai/rules/git-safety.md` puts the raw verbs in the script, and `set -euo pipefail` gives the after-the-commit ordering for free |
| No working-tree condition is added to `create` | Refuse there as well, as an earlier warning | It cannot be computed correctly at that moment, per the correction above, and a create-time pass would not be a run-time guarantee anyway because the tree moves between the two. Two checks on one property, one of them unable to be right, is machinery the problem does not need (`ai/rules/simplicity.md`) |
| The deletion is a new rendered section rather than lines inside `renderSharedIndexRepair` | Extend `renderSharedIndexRepair`, as both journal rows propose | That function points the shared index at what was committed. The working-tree deletion is a different job with a guard of its own, and folding it in leaves one function with two reasons to change |
| The guard compares `git ls-files -s` entry lines from a throwaway index against the entries captured from the private index before the removal | Compare a hash of the file against the blob in `HEAD^` | The entry line carries the mode as well as the blob, so a mode change is a difference. Staging through Git applies the filters Git would apply, so "identical" means identical to GIT rather than to `cmp`. `renderDriftNote` already answers the same question this way, with a recorded reason for rejecting `git update-index --refresh` |
| A path is deleted only on a positive entry match | Delete unless a difference is detected | The two read the same in the happy path and differently in every failure. A missing file, an unreadable file, a path absent from HEAD and a failed staging each produce no proof, and the intersection leaves the file alone in all four |
| `ai/skills/ze-progress.md` is repaired in this work | Leave it, journal it, spec it separately | It is the sibling instruction for the same act, it instructs a banned verb, and the change makes its neighbour's text wrong anyway. The unit fixed is the PROBLEM, not the files the defect was first noticed in (`ai/rules/principles.md`) |
| `specpath.All` and `specpath.Find` keep the filesystem as their authority | Resolve a spec through `git ls-files`, so a removed spec stops counting | NOT a second defect. A spec exists on disk before it is ever committed: `/ze-spec` writes the file and `Claim` (`internal/le/spec/session/session.go`) resolves it through `specpath.Find`, so a tracked-only test would refuse to resolve a spec the session just created and would make `./le spec citation` blind to every uncommitted spec. The over-count is the working tree being WRONG, not the test asking the wrong question. Once the removal takes the file with it, the filesystem and Git agree and both counts are right |

## Known Limitations
- A file that survives under AC-2 still counts as an open spec in `./le spec status`. That is correct rather than a gap: a file holding content no commit carries IS open work, and the stderr report is what tells the operator it is there.
- `validateRemovePath` accepts a path tracked in the shared index and absent from HEAD, so such a `remove` clears an index entry and changes no tree. This spec leaves that behavior alone and only guarantees the file is not deleted for it (AC-6).
- The three recorded instances are repaired by hand under an owner gate, not by this change. Nothing here walks history.

## RFC Documentation (Scope: protocol)

Not applicable. No RFC governs a commit script.

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
- [ ] AC-1..AC-12 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
- [ ] Feature code integrated (`internal/le/commit/script.go`), not test-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** the spec removal only (commit A preserves the spec in history)

## Review Gate

<!-- Filled at implementation time by /ze-review, not now. -->

### Run 1
| Severity | Finding | File | Resolution |
|----------|---------|------|------------|

### Run 2
| Severity | Finding | File | Resolution |
|----------|---------|------|------------|
