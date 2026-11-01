# Spec: structure-checkin

| Field | Value |
|-------|-------|
| Status | design |
| Scope | tooling |
| Priority | P3 |
| Release reason | Development tooling that proposes refactors; the release ships without it and no shipped behavior changes. |
| RFC gaps | - |
| Depends | `plan/post-release/engineering/spec-journal-resolution.md` (its Phase 3 adds the `standing` status this spec takes once its tooling lands) |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-01 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Thomas approved a structural check-in that triggers itself, adapted from the
BAMA skill "improve-architecture". In his words it "fires when a file grows too
large, one feature has to touch too many files, or the same area keeps
collecting defects; it proposes graded refactors, and an agent left running
alone acts only on the strongest". He then agreed that the check-in takes over
the existing 1,000-line size check: a file both oversized and recently changed
comes up for a check-in, and every other oversized file is reported for
information only.

Today the size check is `checkFileSizes` in `./le doc consistency`. It reports
an ERROR on 143 non-test Go files at HEAD (counted 2026-11-01), and no gate,
verify stage or commit runs it, so the red is unread. Nothing else in the
tooling looks at a file's growth, at how widely one feature spreads, or at a
package that keeps collecting journal rows.

This spec owns that check-in. It is LONG-LIVED and never closes: its tooling
phases run once, and after that each check-in pass is one dated row in the Pass
Log below, appended by the session that ran it, the same shape as
`plan/post-release/engineering/spec-journal-resolution.md`.

Goal: a structural problem is noticed when its signal appears, every signal the
tooling raises reaches a graded proposal, and each proposal Thomas could act on
exists as a spec skeleton he can approve or drop. No refactor runs without his
decision.

Three points come from Ze's own rules and are recorded as such, not as new
decisions:

| Point | Rule it comes from |
|-------|--------------------|
| No 600-line signal | Thomas retired it on 2026-08-01; the comment above `fileSizeMax` (`internal/le/doc/consistency/consistency.go`) records it |
| The size check moves; it is not kept twice or behind a flag | `ai/rules/no-layering.md` |
| An unattended agent applies no refactor: a refactor is not a fix, so its proposals become spec skeletons and Thomas decides which run | `ai/META-DECISIONS.md` ("A logic bug or misbehaviour is fixed"; a refactor is neither), `ai/rules/principles.md` (scope is the owner's) |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - the design facts every change respects
  → Constraint: a feature registers and is discovered; the new command is one package calling `leroot.Register`, plus one blank import, and touches no central list.
  → Decision: the skill's refactor vocabulary is the system door's own: "a registry or a seam", "registration", "dependency direction". No parallel vocabulary (BAMA's "deepening", "module depth") is introduced.
- [ ] `ai/doors/le-tooling.md` - le's boundaries
  → Constraint: `le arch hotspots` lives at `internal/le/arch/hotspots` (`TestEveryCommandIsFoundAtThePathItsNamePredicts`).
  → Constraint: an action answers through the shared renderer, never a private JSON mode.
  → Constraint: the session-start hook runs in every concurrent session; a step that panics, waits or overruns harms every session at once (`plan/learned/HOOK-FRICTION.md`).
- [ ] `ai/doors/commands-cli.md` - the command model le shares
  → Constraint: help, usage and completion are derived from the registration; the new command's argument is a declared keyword before its value (`since <ref>`), checked by `./le cli grammar`.
- [ ] `docs/architecture/core-design.md`, "Command engine and composition roots" - how an le tool registers; the `// Design:` page of the `internal/le/arch` gates
  → Constraint: this page names the arch gates; it gains the hotspots command in the same change.
- [ ] `docs/contributing/documentation-testing.md` - the `./le doc consistency` row
  → Constraint: the row lists what consistency checks; it already omits file size, so after this spec it is correct as written, and the page states where the size check went.
- [ ] `docs/contributing/ze-go-style.md` (lines ~489 and ~620) - "Past 1000 lines, look for a second concern. Split only when the separation is right." and "1000 lines is where you look for a second concern. It is the only threshold"
  → Constraint: the style guide owns the meaning of the threshold; the tool owns the number. The guide gains a pointer to `./le arch hotspots` and restates no number the tool declares.
- [ ] `plan/journal/README.md` - row format, DUE, owned rows
  → Constraint: "the same kind of defect twice" is already the journal's DUE (`captureKinds`, `internal/le/spec/journal/report.go`); this spec does not recount it.
  → Constraint: the Surface cell is free text; about half of the 3,012 rows at HEAD (1,460 on 2026-11-01) name a path under `internal/`, `cmd/`, `pkg/` or `test/`. A grouping over Surface counts only those and says how many it could not place.
- [ ] `plan/post-release/engineering/spec-journal-resolution.md` - the never-closing owner spec this one copies
  → Decision: a pass is a Pass Log row, never a spec of its own.
  → Constraint: the `standing` status is approved there and not yet in `Vocabulary` (`internal/le/spec/status/specstatus.go`); this spec takes it once that phase lands.
- [ ] `ai/rules/simplicity.md` - the shapes table
  → Decision: the skill's deletion test is the table's "a wrapper that transforms nothing" and "an interface with one implementation", applied to existing code.
- [ ] `ai/rules/architecture.md` - "a wrapper that transforms nothing MUST be deleted"
  → Constraint: a refactor card that proposes a new layer must name the rule that requires it, or it fails the deletion test.
- [ ] `plan/learned/DESIGN-HISTORY.md` - the `UpdateHandle` identity wrapper, rejected (around line 272)
  → Constraint: a proposal that adds a wrapper or a layer fails the same test the history applied.
- [ ] `ai/skills/ze-hunt.md` - the read-only sweep skill
  → Decision: the new skill follows its shape: read-only, a grep or a measure is a candidate never a finding, every candidate read at the producer and against its page.

**Key insights:**
- The full 90-day `git log --numstat` over the tree took 14.8 s on the Linux host (2026-11-01, 8,992 commits). Path-limited to the 143 oversized files it took 7.2 s. The session-start hook has 3.5 s for all its steps (`sessionStartBudget`, `internal/le/hookruntime/lifecycle.go`). The trigger therefore cannot compute churn over a fixed window at session start; it measures only since the last pass, and bounds even that.
- The `Spec-work:` trailer exists only since 2026-10-29 (379 commits in the last 90 days carry one). Fan-out per feature has days of data, not months.
- The last pass is found from git, not from this file: every commit of a pass session carries `Spec-work: structure-checkin`, written by `./le commit create` from the session's claim.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/doc/consistency/consistency.go` - `fileSizeMax` (1000), `checkFileSizes` reports SeverityError category `file-size` for every non-test `.go` file over it, walking the WORKING TREE through `walkGoFiles` (skips `vendor`, `node_modules`, `research`, `modcache`, `tmp`, `gokrazy`, dot-directories). `countLines` returns -1 and reports `unreadable` on a partial read. `isExemptFile` (register.go, embed.go, doc.go, all.go, `_gen.go`) is NOT applied by `checkFileSizes`, only by the design-ref and cross-ref checks.
- [ ] `internal/le/doc/consistency/consistency_test.go` - `TestCheckReportsFileSize` pins 1000 passes and 1001 fails with message `1001 lines (max 1000)`; its comment, and the comment above `fileSizeMax`, cite `ai/rules/go-standards.md`, which no longer states the threshold.
- [ ] `internal/le/hookruntime/lifecycle.go` - `sessionStartSteps` runs tree, specs, verification debt, session state, journal, derived artifacts, agent files in order within `sessionStartBudget` (3.5 s); `reportDueJournal` prints `journal: <N> problem classes are due for a fix pass (./le spec journal report)` from `specjournal.Check`, and on failure writes a note line and carries on.
- [ ] `internal/le/spec/journal/report.go` - `captureKinds`, `DueRowsMin`, `DueRowsAfterCheck`, `DueCount`.
- [ ] `internal/le/spec/journal/rows.go` - `HeadRows` answers every well-formed row at HEAD with its Surface cell; its one consumer today is `internal/le/feature/check.go`.
- [ ] `internal/le/commit/actions.go` - `trailerSpecWork` (`Spec-work: `), written from the session's spec claim.
- [ ] `internal/le/repo/rewrite/activity.go` - `collectActivity` runs `git log --numstat` for `./le repo rewrite loc-activity`; `numstatTargetPath` resolves a rename to its target path.
- [ ] `internal/le/arch/` - gates `compoundguard`, `enumeration`, `fspersistence`, `ifaceresolution`, `refusebydefault`, `testonly`, `tier`; none measures size, churn or fan-out.
- [ ] `internal/le/ai/sync/ai.go` - `ai/skills/<name>.md` is the canonical skill, mirrored to the gitignored `.claude/skills/` and `.agents/` trees.
- [ ] `ai/skills/ze-review-deep.md` (line 319) - a diff lens "go-standards.md: Files under 1000 lines, single concern per file"; go-standards.md does not carry that line.

**Behavior to preserve:**
- The threshold: a non-test Go file of 1000 lines is not oversized, 1001 is.
- The one declaration of that threshold (`fileSizeMax`); it moves, it is not copied.
- An unreadable file is reported, never counted as small.
- Every other `./le doc consistency` check, its exit codes and its renderer.
- The journal banner line and its step.

**Behavior to change:**
- `./le doc consistency` stops reporting `file-size`; the size check, its threshold and its test move to `./le arch hotspots` (no-layering).
- A new command reports the three signals; a new session-start step prints one line when a check-in is due.
- A new read-only skill grades refactor proposals; a pass records its result here.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| The 1,000-line size check | `internal/le/doc/consistency/consistency.go` `checkFileSizes`, `fileSizeMax`, `countLines` | ERROR per oversized non-test Go file in the working tree; run only by hand | replaces: moves whole into `internal/le/arch/hotspots`, reading HEAD; deleted from consistency in the same commit (no-layering), so one mechanism judges size |
| Journal DUE | `internal/le/spec/journal/report.go` `captureKinds`, `DueCount`; `reportDueJournal` | marks a problem class due for a fix pass; one session-start line | does not apply as a signal: "same kind of defect twice" is already DUE, so hotspots does not recount it; extends its pattern: the new thresholds live in one kind table shaped like `captureKinds`, and the new step sits beside `reportDueJournal` |
| Journal rows at HEAD | `internal/le/spec/journal/rows.go` `HeadRows` | parses every row through the reader `Check` uses | extends: the defect-area signal groups `HeadRows` Surface cells by package; no second journal parser |
| `Spec-work:` trailer | `internal/le/commit/actions.go` `trailerSpecWork` | groups a spec's commits for the closure review | extends: groups commits per feature for fan-out, and marks the last pass (commits under this spec's claim) |
| Git numstat reader | `internal/le/repo/rewrite/activity.go` `collectActivity`, `numstatTargetPath` | per-day additions for the activity dashboard | extends the rename handling: `numstatTargetPath` moves to a leaf both packages import, rather than a second rename parser; the per-day aggregation does not apply (hotspots counts per file) |
| Session-start step table | `internal/le/hookruntime/lifecycle.go` `sessionStartSteps`, `sessionStartBudget` | ordered steps, each cut at the shared deadline | extends: one more step, `structure`, after `journal` |
| Shared renderer | `internal/component/command` (le's answers, `| json`, `| count`) | renders every le action | extends: the command answers through it, no private JSON |
| arch gates | `internal/le/arch/*` | import direction, central enumerations, fail-open guards | extends the namespace: `le arch hotspots` is information, not a gate; it never refuses a commit |
| `./le repo changed` | `internal/le/repo/changed/scope.go` | the packages and suites a change reaches | does not apply: it scopes one change's tests; hotspots aggregates history |
| `/ze-hunt` | `ai/skills/ze-hunt.md` | read-only sweep for recorded bug classes | does not apply as the analysis (it hunts known defects, not structure); the new skill copies its read-only, candidate-then-producer method |
| `/ze-review-deep` size lens | `ai/skills/ze-review-deep.md` line 319 | reviewer guidance on a diff | extends: the lens cites the style guide and `./le arch hotspots` instead of restating the number and a rule file that no longer holds it |
| `/ze-extract` | `ai/skills/ze-extract.md` | moves named symbols between files of one package | does not apply here; a child refactor spec uses it for its moves |
| Simplicity shapes table, identity-wrapper history | `ai/rules/simplicity.md`; `plan/learned/DESIGN-HISTORY.md` (`UpdateHandle`) | names the shapes that add machinery for nothing | extends: the skill's deletion test applies the same shapes to existing code |
| Never-closing owner spec | `plan/post-release/engineering/spec-journal-resolution.md`, "Pass Log" | one row per fix pass | extends: the same Pass Log shape and claiming rules |
| Proposals bucket | `plan/README.md`, `plan/proposals/` | unapproved ideas, counted but not committed to | extends: each refactor skeleton lands there until Thomas approves it |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le arch hotspots` (optionally `since <ref>`), run by hand or by a pass.
- The session-start hook's `structure` step.
- `/ze-structure`, the read-only skill, run by a `ze-read` agent during a pass.

### Transformation Path
1. Find the base: the newest commit reachable from HEAD whose message carries `Spec-work: structure-checkin`. With none, the base is HEAD minus the declared window in days. `since <ref>` overrides it.
2. One `git log <base>..HEAD` read gives, per commit, its `Spec-work:` stem and its changed paths (renames resolved to their target).
3. Size: every non-test Go file at HEAD over `fileSizeMax` lines (line counts from the blobs at HEAD) is oversized. An oversized file with commits since the base at or above the churn minimum is due; every other oversized file is informational.
4. Defect area: `HeadRows` open rows dated after the base commit's date; each full path in the Surface cell maps to its package directory; a package at or above the area minimum is due. Rows with no path are counted as unplaced.
5. Fan-out: per `Spec-work:` stem since base, the distinct packages its non-test Go changes touched; above the fan-out minimum it is due.
6. The answer renders through the shared renderer: due items first, then informational oversized files, then the unplaced row count, with every threshold and the base.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| le to git | `git log`, `git cat-file --batch` at HEAD, through le's existing git runner | No |
| hotspots to journal | `specjournal.HeadRows` | No |
| session-start hook to hotspots | a Go call to the hotspots package with a commit budget | No |
| skill to tooling | the skill runs `./le arch hotspots | json` and `./le spec journal report` | No |

### Integration Points
- `fileSizeMax` - moves into `internal/le/arch/hotspots`; the one declaration.
- `HeadRows` - the area signal's only journal reader.
- `numstatTargetPath` - one rename resolver for both readers.
- `sessionStartSteps` - one new entry.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | journal read through `HeadRows`; git through le's runner; answer through the shared renderer |
| No unintended coupling (components stay isolated) | Yes | le-only code; nothing under `internal/le` is imported by product code (`TestNormalZeLinksNoInternalLe`) |
| No duplicated functionality (extends existing, does not recreate) | Yes | the size check moves rather than duplicates; rename parsing shared; DUE not recounted |
| Zero-copy preserved where applicable (refs, not copies) | N-A | development tooling, no wire path |
| Registration over hardcoding, outbound | Yes | the command registers from its own `register.go`; one blank import in `internal/le/register.go`; the skill is one file the sync discovers |
| Registration over hardcoding, inbound | Yes | searched: `sessionStartSteps` is the declared step list and gains its entry by design; no help string, completion table or verify stage names the command (help and completion derive from the registry) |

## The Check-in Process

### Signals

Every threshold is declared once, in one kind table in
`internal/le/arch/hotspots`, shaped like `captureKinds`. This spec names the
fields, not the values the implementation calibrates.

| Kind | Subject | Due when | Value source |
|------|---------|----------|--------------|
| size-churn | a non-test Go file at HEAD | over `fileSizeMax` lines AND touched by at least the churn minimum of commits since the base | `fileSizeMax` is 1000 (moved, unchanged); the churn minimum is calibrated at implementation, its measurement written beside it |
| defect-area | a package directory | at least the area minimum of open journal rows dated after the base name a file in it | the area minimum is 3 ("the same area keeps collecting defects", three entries) |
| fan-out | a `Spec-work:` stem | its non-test Go changes since the base touch more than the fan-out minimum of distinct packages | calibrated at implementation from the trailer history then present; counted in packages, not files, because a Ze feature owes its doc, registration and test together |

An oversized file below the churn minimum is informational: listed, never due.
"The same kind of defect twice" is the journal's DUE and is not recounted. A
type over 400 lines and an untestable dependency are lenses of the skill, run
only on the candidates, never measured by the command.

### Whole-tree cost

The command is git I/O: one `git log` since the base, one batch read of the
non-test Go blobs at HEAD for their line counts, and the journal at HEAD. It
builds, compiles, lints and tests nothing, so it is not the whole-tree check
`ai/META-DECISIONS.md` keeps off the Linux host. It is not free either: over 90
days the history read took 7.2 to 14.8 s here. That is why the base is the last
pass, not a fixed window, and why the session-start step reads line counts only
for files changed since the base and refuses to measure past its commit budget.

### Trigger

| State at session start | Line printed |
|------------------------|--------------|
| commits since the base within the step's commit budget, N due | `structure: N hot spots are due a check-in (./le arch hotspots)` |
| within budget, nothing due | none |
| more commits since the base than the budget | `structure: M commits since the last check-in pass, more than session start can measure; run ./le arch hotspots` |
| no pass ever recorded | the same over-budget line, counting from the window |
| git or the journal unreadable | a note line naming the cause; the rest of the hook runs |

The commit budget is declared in the same kind table and calibrated so the step
stays inside `sessionStartBudget` beside the other steps.

### Running a pass

| Step | Rule |
|------|------|
| 1 | The session claims this spec (`./le spec claim plan/post-release/engineering/spec-structure-checkin.md`), appends a Pass Log row with result `running` and its session id, and commits it alone, so its commits from here carry `Spec-work: structure-checkin` |
| 2 | It runs `./le arch hotspots | json` and hands the due items to a `ze-read` agent running `/ze-structure` |
| 3 | The skill returns cards graded Strong, Worth exploring or Speculative. Each card names the signal, the producer read, the deletion-test result, the existing mechanism the change would extend (registry, seam, dependency direction, from `ai/doors/`), and what a reader holds in their head before and after (`ai/rules/simplicity.md`) |
| 4 | Thomas present: Strong and Worth-exploring cards become spec skeletons in `plan/proposals/`, Existing Mechanisms filled, and he picks which move to `plan/post-release/engineering/` |
| 5 | Thomas away: only Strong cards become skeletons; Worth-exploring cards are listed in the Pass Log row; Speculative cards live only in the report |
| 6 | No pass commit edits source. A refactor runs only as an approved child spec through `/ze-implement`: one commit per step, independent review, `/ze-extract` for moves, and a fresh discrimination record for every moved RFC-tagged unit (`ai/rules/rfc-compliance.md`) |
| 7 | The pass's last commit rewrites its Pass Log row: due items seen, cards per grade, skeleton paths, SHAs. That commit is the new base |

A `running` row whose session directory is no longer written is abandoned, as in
the journal-resolution spec, and the next session may run the pass.

## Pass Log

| Date | Session | Due items | Strong / Worth exploring / Speculative | Skeletons written | Commits |
|------|---------|-----------|----------------------------------------|-------------------|---------|

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Commits since the last pass fit the session-start budget most days | ~1.6 ms per commit for a full numstat read here (14.8 s / 8,992) | the step prints the over-budget line every session | timing the step at implementation, written beside the budget | unvalidated |
| A-2 | The newest `Spec-work: structure-checkin` commit is the last pass | the trailer comes from the claim; a pass claims this spec | the tooling commits of this spec reset the base before any pass | the first pass is this spec's last implementation phase | unvalidated |
| A-3 | A Surface path names the package the defect lives in | 1,460 of 3,012 cells name a full path | area counts mislead | the skill reads each row at the producer before a card | unvalidated |
| A-4 | Packages, not files, measure fan-out fairly | Ze owes doc, registration and test with a feature | fan-out fires on every feature | calibration over the trailer history at implementation | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The session-start step overruns and the harness kills the hook | the hook's cut note names `structure` | commit budget, measured; the step is cut like any other |
| R-2 | Removing the size check from consistency hides oversized files | none: by design the same files appear in hotspots | AC-2 lists every oversized file at HEAD as informational |
| R-3 | Passes write skeletons nobody reads, and proposals pile up | `plan/proposals/` grows by pass | the Pass Log row names each; Strong only when Thomas is away |
| R-4 | A pass edits source under the claim | a pass commit carrying a `.go` file | step 6; the claim stays on a spec whose edits are spec and Pass Log only |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing user-visible: le output and one session-start line. A step that panics or overruns degrades every session's start |
| How is it reverted? | One commit revert per phase; the consistency move is one commit |
| Who else touches this path? | `spec-journal-resolution` (journal package, session-start step table, `standing` status) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le arch hotspots` | → | hotspots `Answer` | `TestHotspotsAnswersThroughTheRegistry` |
| session-start hook | → | `reportStructure` step | `TestSessionStartPrintsStructureLine` |
| `./le doc consistency` | → | consistency `Check` | `TestConsistencyReportsNoFileSize` |
| `ai/skills/ze-structure.md` | → | `./le ai sync` mirror | `TestSkillMirrorsCarryZeStructure` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A non-test Go file at HEAD over 1000 lines, touched by at least the churn minimum of commits since the base | `./le arch hotspots` lists it as due with its path, line count and commit count |
| AC-2 | An oversized file below the churn minimum | listed as informational, not counted due; every oversized non-test Go file at HEAD appears in one of the two lists |
| AC-3 | Files of 1000 and 1001 lines | 1000 is not oversized, 1001 is |
| AC-4 | A file that cannot be read in full, or git that fails | the command exits non-zero naming the path or the git error and the fix; it never answers an empty list |
| AC-5 | Three open journal rows dated after the base name files in one package; two in another | the first package is due, the second is not; rows with no path appear as an unplaced count |
| AC-6 | A `Spec-work:` stem whose changes since the base touch more than the fan-out minimum of packages | listed as due with its package count |
| AC-7 | Commits before the newest `Spec-work: structure-checkin` commit | not counted; with no such commit the window applies; `since <ref>` overrides |
| AC-8 | `./le arch hotspots | json` and `| count` | JSON carries kind, subject, measures, the thresholds and the base; count is the due total |
| AC-9 | Session start, N due within the commit budget | one line `structure: N hot spots are due a check-in (./le arch hotspots)`; none when nothing is due |
| AC-10 | Session start, more commits since the base than the budget | the over-budget line naming M and the command; the step does no per-commit read |
| AC-11 | `./le doc consistency` over a tree with a 1001-line Go file | no `file-size` finding; no size threshold remains in the consistency package |
| AC-12 | `/ze-structure` on the due items | read-only; every card graded, naming signal, producer, deletion-test result, existing mechanism, and before/after reading cost in Ze's door vocabulary |
| AC-13 | A pass run under The Check-in Process | its Pass Log row is complete; skeletons exist in `plan/proposals/` with Existing Mechanisms filled for the grades step 4 or 5 allows; no pass commit carries a `.go` file |
| AC-14 | Session start right after a pass with no new signal | no structure line |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSizeBoundary` | `internal/le/arch/hotspots/hotspots_test.go` | AC-3, replacing `TestCheckReportsFileSize` | |
| `TestOversizedChurnIsDue` | `internal/le/arch/hotspots/hotspots_test.go` | AC-1, AC-2 on a scratch repository | |
| `TestUnreadableFailsClosed` | `internal/le/arch/hotspots/hotspots_test.go` | AC-4 | |
| `TestDefectAreaGroupsSurfacePaths` | `internal/le/arch/hotspots/hotspots_test.go` | AC-5 | |
| `TestFanOutCountsPackages` | `internal/le/arch/hotspots/hotspots_test.go` | AC-6 | |
| `TestBaseIsLastPass` | `internal/le/arch/hotspots/hotspots_test.go` | AC-7 | |
| `TestHotspotsAnswersThroughTheRegistry` | `internal/le/arch/hotspots/report_test.go` | AC-8 | |
| `TestSessionStartPrintsStructureLine` | `internal/le/hookruntime/lifecycle_test.go` | AC-9, AC-14 | |
| `TestSessionStartStructureOverBudget` | `internal/le/hookruntime/lifecycle_test.go` | AC-10 | |
| `TestConsistencyReportsNoFileSize` | `internal/le/doc/consistency/consistency_test.go` | AC-11 | |
| `TestSkillMirrorsCarryZeStructure` | `internal/le/ai/sync/` tests | the skill is mirrored | |

`TestCheckReportsFileSize` is deleted with the code it tests; its replacement
`TestSizeBoundary` proves the same boundary and message. The deletion goes to a
`ze-test-judge` agent before it lands (`ai/META-DECISIONS.md`, "Test changes").

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| file lines | 0 upward | 1000 not oversized | N/A | 1001 oversized |
| area rows | 0 upward | 2 not due | N/A | 3 due |
| churn, fan-out, commit budget | 0 upward | calibrated value minus one not due | N/A | calibrated value due |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `le-arch-hotspots` | `test/` le suite, a fixture beside `internal/test/fixture/ui_fixture_le_consistency_answers.go` | the built le answers `arch hotspots` over an owned scratch repository, plain, `| json` and `| count` | |

### Interop Tests (Scope: protocol)
N-A: tooling, no protocol peer.

## Files to Modify
- `internal/le/doc/consistency/consistency.go` - delete `checkFileSizes`, `fileSizeMax`, `countLines` and the check's call
- `internal/le/doc/consistency/consistency_test.go` - delete `TestCheckReportsFileSize`, add `TestConsistencyReportsNoFileSize`
- `internal/le/repo/rewrite/activity.go` - import the shared rename resolver
- `internal/le/hookruntime/lifecycle.go` - the `structure` step
- `internal/le/register.go` - one blank import
- `docs/contributing/ze-go-style.md` - point the threshold at `./le arch hotspots`
- `docs/contributing/documentation-testing.md` - say where the size check went
- `docs/architecture/core-design.md` - the hotspots command among the arch tools
- `ai/INDEX.md` - consistency row drops "file sizes"; keyword rows for hotspots and the skill
- `ai/skills/ze-review-deep.md` - line 319 cites the style guide and the command
- `docs/contributing/claude-code-cheatsheet.md` - `/ze-structure` row
- `.claude/hooks/README.md` - the new session-start line

## Files to Create
- `internal/le/arch/hotspots/` - `register.go`, the kind table, the reader, the report, tests
- a leaf package for `numstatTargetPath` under `internal/le` (the implementation names it)
- `ai/skills/ze-structure.md` - the read-only skill
- a le functional test for the command, beside the consistency fixture

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | le command, no YANG |
| YANG validation constraints | N-A | no config |
| YANG custom validators | N-A | no config |
| CLI commands/flags | Yes | `le arch hotspots` registers through `leroot` |
| CLI grammar (keyword before value) | Yes | `since <ref>`; `./le cli grammar` |
| Editor autocomplete | N-A | le completion derives from the registry |
| Functional test for new RPC/API | Yes | `le-arch-hotspots` |
| Pipe completeness | Yes | shared renderer, `| json`, `| count` |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | N-A | reads git and files only |
| Prometheus counters/metrics | N-A | none |
| BGP family surface (new SAFI / capability / attribute) | N-A | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | developer tooling, no `features/` entry |
| 2 | Config syntax changed? | No | none |
| 3 | CLI command added/changed? | No | le, not ze: `docs/architecture/core-design.md` and `le help` |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | No | none |
| 7 | Wire format changed? | No | none |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | No | none |
| 10 | Test infrastructure changed? | Yes | `docs/contributing/documentation-testing.md` |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | le command only |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | derived at implementation with `./le spec citation anchors spec plan/post-release/engineering/spec-structure-checkin.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/contributing/ze-go-style.md`, the `ai/INDEX.md` line for `./le doc consistency` |

## Implementation Steps

1. **Phase: Wiring** -- register `le arch hotspots` answering an empty report, add the session-start step calling it, write the four wiring tests red.
2. **Phase: size move** -- move `fileSizeMax` and the line counter into hotspots reading HEAD, delete them from consistency, judge the test deletion (AC-2, AC-3, AC-4, AC-11).
3. **Phase: churn and base** -- base finding, the one `git log` read, the shared rename resolver (AC-1, AC-7).
4. **Phase: defect area and fan-out** -- `HeadRows` grouping, `Spec-work:` grouping, calibration measurements written beside the kind table (AC-5, AC-6, AC-8).
5. **Phase: trigger** -- commit budget measured, the step's lines (AC-9, AC-10, AC-14).
6. **Phase: skill and docs** -- `ai/skills/ze-structure.md`, the doc edits above (AC-12).
7. **Phase: first pass** -- this spec takes `standing` if journal-resolution's Phase 3 has landed, and runs the first pass under its own process (AC-13). From then on, passes repeat; the spec never closes.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | the base excludes the tooling commits only after the first pass; renamed files keep their churn |
| Rule: no-layering | no size check, threshold or test left in consistency |
| Rule: principles, single declaration | every threshold in one kind table; the style guide and the review lens restate no number |
| Rule: principles, silent wrong value | unreadable input and a git failure refuse; no empty list in their place |
| Hook | the step cannot panic, wait or print outside its report; it fits the measured budget |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| hotspots command | the unit tests above under `./le job run`, and the functional test |
| size move | `TestConsistencyReportsNoFileSize`; grep shows `fileSizeMax` declared once |
| trigger | `TestSessionStartPrintsStructureLine`, `TestSessionStartStructureOverBudget` |
| skill | `TestSkillMirrorsCarryZeStructure` |
| first pass | a Pass Log row and the skeletons it names |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | `since <ref>` is passed to git as one argument after a ref check, never through a shell; Surface paths are only grouped, never opened |

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

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The base is the last pass, found by its `Spec-work:` trailer | a fixed 90-day window; a cache keyed by HEAD | the window costs 7.2 to 14.8 s, over the 3.5 s hook budget; a cache is a second store to keep fresh; "newer than the last pass" is what the check-in fires on |
| Over budget, the step prints the commit count | compute anyway; print nothing | computing overruns the hook; printing nothing hides a due check-in |
| The size check moves out of consistency | keep both | no-layering; two judges of one fact disagree |
| Fan-out counts packages | files | Ze owes doc, registration and test together, so file counts fire on every feature |
| Unattended passes skeleton Strong cards only | skeleton Strong and Worth exploring always | Thomas: "an agent left running alone acts only on the strongest" |
| Skeletons land in `plan/proposals/` | `post-release/engineering/` directly | a refactor is unapproved until Thomas picks it |

## Known Limitations
- Half the journal rows name no path, so the area signal is blind to them; the command prints how many it could not place.
- Fan-out has only the trailer history since 2026-10-29 to calibrate on.

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
- [ ] AC-1..AC-14 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass: the scoped package tests under `./le job run`, the le functional test, and `./le verify changed` over the phase commits; the full verification is Thomas's release check
- [ ] Integration and Documentation checklists answered with evidence
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional test for the command
- [ ] Interop tests N-A: tooling

### Closure
- [ ] This spec never closes. Each tooling phase commits through `./le commit create` with its review recorded; each pass appends its Pass Log row
