# Spec: journal-resolution

| Field | Value |
|-------|-------|
| Status | ready |
| Priority | P3 |
| Release reason | Release-independent journal resolution work; historical findings do not establish a current product defect. |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-22 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The owner's standing decisions (`ai/META-DECISIONS.md`) want a long-lived spec
that decides how journal rows get resolved. Every open row is listed in the
owned journal rows table of the spec that will fix it, and a commit adding or
changing an open row no spec owns is refused (`plan/journal/README.md`, "Owned
rows").

The journal route (`ai/rules/rule-precedence.md`, `ai/rules/completion.md`) records
a defect walked into as one row in `plan/journal/<class>.md`. Recording is fast, and
nothing resolves what is recorded. On 2026-10-22 the committed journal holds 236
class files and 2736 rows, 1334 of them `open:`; `./le spec journal report` marks
56 classes DUE, and only 2 classes at HEAD carry a capture line, both `rule`.
`plan/journal/README.md` already declares how a class becomes DUE and what one fix
pass on a DUE class delivers. Nothing declares which class goes next, how two
sessions avoid running the same pass, what closes a row whose code is gone, when a
row is promoted to a spec, or what evidence a closed row carries.

This spec owns that process. It is LONG-LIVED: it is never closed. Its design and
its tooling phases run once; after that each fix pass is one dated row in the Pass
Log below, appended by the session that ran the pass.

Goal: every DUE class reaches a capture and leaves DUE, every `open:` row reaches
`fixed:` or names the spec that carries it, and the DUE count the session-start
banner prints goes down rather than up.

## Required Reading

### Architecture Docs
- [ ] `plan/journal/README.md` - row format, Fix marker, capture line, DUE, fix-pass and check-repair procedures
  → Constraint: the Fix marker has exactly two values, `fixed: ` and `open: `; "there is no third value". A stale or promoted row MUST use one of the two, so this spec adds no marker.
  → Constraint: DUE thresholds are declared once in `captureKinds` (`internal/le/spec/journal/report.go`: check 1 row, rule 10, uncaptured 10). This spec MUST NOT restate the numbers; it reads the report.
  → Constraint: the five deliverables of a check-capture pass and the five steps of a check repair are declared in the README. This spec references them and MUST NOT copy them.
  → Decision: a pass that cannot build a check sets a `rule` capture and says why in the commit body (README); a pass MUST NOT replace a check capture with a rule capture.
- [ ] `ai/rules/rule-precedence.md` - the three-step journal route and the "deliberate pass over the journal"
  → Constraint: a walked-into defect gets one row, the work in hand closes, and the session stops. A pass is the deliberate later work, run as its own work item, never folded into a closing commit.
  → Decision: the owner's journal rule lists every open row in the owned journal rows table of the spec that will fix it, refused at commit when unowned; the point `rule-precedence/directives/close-the-work-first-then-ask-about-the-unrelated-defect` states it and points here.
- [ ] `ai/rules/completion.md` - recording versus fixing
  → Constraint: a row is a record, never a fix; a row stays `open:` until code changes (`principles.md`, "Recording a defect MUST NOT be presented as fixing the product").
- [ ] `ai/rules/planning.md` - spec lifecycle, journal row format, closure repoint
  → Constraint: closure commit A MUST repoint every citation of the closing spec; a journal row naming a promoted spec is such a citation.
  → Constraint: a phase runs through its own skill; review runs in a context that did not produce the work.
- [ ] `docs/contributing/spec-workflow.md` - status vocabulary
  → Constraint: Status values are declared once in `Vocabulary` (`internal/le/spec/status/specstatus.go`): skeleton, design, ready, in-progress, verification, blocked, deferred, done. None means "never closes".
- [ ] `plan/README.md` - buckets and lifecycle
  → Decision: this spec sits at `plan/` level: the release ships without the resolution process. A spec promoted from a row is bucketed by its own defect, independently.
- [ ] `ai/META-DECISIONS.md` - goal and scope, the journal rule
  → Constraint: a logic bug or misbehavior is fixed without asking, so running a fix pass needs no owner go-ahead per pass.
- [ ] `docs/architecture/core-design.md` - declared by the `// Design:` header of every file this spec changes (le's spec inventory, the native edit hook policy)
  → Constraint: the page describes le's tooling at the level of commands and hooks; a new status or report line is checked against it at implementation, and it is named in Documentation row 16 as affected or unaffected with the reason.

**Key insights:**
- The README declares DUE and the content of a pass; the report's sort is the pass order; this spec adds ordering for check gaps, pass claiming, stale closure, promotion, closure evidence, and the never-closing status.
- `writeSpecStatus` (`internal/le/hookruntime/writeedit.go`) matches the literal strings `| Status | in-progress |` and `| Status | verification |`, a second declaration of which statuses permit a source edit.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/spec/journal/journal.go` - `Report` reads every class at git HEAD, appends each class, then sorts: DUE classes first, most counted rows first, the rest stable in name order. A check-captured class DUE at 1 counted row therefore sorts behind every uncaptured class with 10 or more.
- [ ] `internal/le/spec/journal/report.go` - `CaptureKind`, `DueRowsMin` (10), `DueRowsAfterCheck` (1), `captureKinds` (the one threshold table), `Class` fields Rows, CountedRows, Open, DueAt, Due, Captured, SpanDays, First, Last.
- [ ] `internal/le/spec/journal/validate.go` - `ValidateFile` checks the five cells, the 600-character limit on rows added or rewritten against HEAD (`RowCharsMax`), the Fix marker, and the capture line. It reads no path inside a Fix cell.
- [ ] `internal/le/spec/status/specstatus.go` - `Vocabulary`, `Declared`, `Category` (in-progress, verification, ready, design map to Backlog).
- [ ] `internal/le/hookruntime/writeedit.go` - `writeSpecStatus` refuses a source edit while the claimed spec's text holds neither `| Status | in-progress |` nor `| Status | verification |`.
- [ ] `internal/le/hookruntime/lifecycle.go` - `hookValidateSpec` / `validateSpecText` refuse a Status outside `Vocabulary`.

**Behavior to preserve:**
- The report's exit codes: 0 when classes are DUE, non-zero only for malformed rows or captures.
- The two Fix markers and the README's pairing rules for measured rows.
- The DUE thresholds and their single declaration.
- The session-start banner text `journal: <N> problem classes are due for a fix pass (./le spec journal report)`.

**Behavior to change:**
- The report orders a DUE class captured by a check ahead of every other DUE class (AC-1).
- The report and validate name an `open:` row whose Fix cell cites a spec path that does not exist (AC-2, AC-3).
- The status vocabulary gains a never-closing status, subject to the owner question in Known Limitations (AC-4, AC-5).

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le spec journal report` and `./le spec journal validate`, reading committed class files (report) or the working tree (validate).
- A spec's metadata table, read by `hookValidateSpec`, `writeSpecStatus` and `./le spec status`.

### Transformation Path
1. `readClasses` parses each class file at HEAD into rows and an optional capture.
2. `appendClass` counts rows after the capture, computes Due from `captureKinds`.
3. The sort orders the classes; the text and JSON renderers print them.
4. `ValidateFile` measures added or rewritten rows of one file against HEAD.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Report to session-start hook | The hook reads the DUE count | No |
| Spec metadata to edit hook | `writeSpecStatus` string match on the claimed spec | No |

### Integration Points
- `captureKinds` - the pass order reads the capture kind from the same table that sets DUE.
- `Vocabulary` - the new status is one more entry; `writeSpecStatus` MUST derive its permitted set rather than add a third literal.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | ordering stays in `Report`; pointer check sits in `ValidateFile` and `Report` |
| No unintended coupling (components stay isolated) | Yes | journal reads spec paths as files; it imports no spec package |
| No duplicated functionality (extends existing, does not recreate) | Yes | extends the existing report and validator |
| Zero-copy preserved where applicable (refs, not copies) | N-A | development tooling, no wire path |
| Registration over hardcoding, outbound | Yes | the permitted-edit statuses become a property of `Vocabulary`, not a literal in the hook |
| Registration over hardcoding, inbound | Yes | searched `writeSpecStatus`, `Category`, `validateSpecText`, the roadmap collector for status literals; each MUST read `internal/le/spec/status` |

## The Resolution Process

This section is the process every pass follows. Thresholds, the pass deliverables
and the check-repair steps are in `plan/journal/README.md` and are not repeated.

### Choosing the next pass

| Step | Rule |
|------|------|
| 1 | Run `./le spec journal report`. The first DUE line not already running in the Pass Log is the next pass |
| 2 | One pass works one class. A class that one check cannot cover is split first (README step 1), and each part is its own pass |
| 3 | After AC-1 lands, the report lists check-gap classes first: a check with a gap is a gate reporting green on the shape it claims to refuse, so it outranks a larger uncaptured class |
| 4 | Running a pass needs no owner question: a misbehavior is fixed without asking (`ai/META-DECISIONS.md`) |

### Claiming a pass

| Step | Rule |
|------|------|
| 1 | The session appends a Pass Log row with result `running`, its session id and the class, and commits it alone before any other edit, so a second session reading HEAD skips that class |
| 2 | A `running` row whose session directory `tmp/session/<date>-<id>/` is no longer written is abandoned: the next session rewrites its result to `abandoned` and may take the class |
| 3 | The pass session claims this spec (`./le spec claim spec plan/post-release/engineering/spec-journal-resolution.md`) for the edit hook, and works the class through `/ze-explore` (read every row), `/ze-implement` (check, proof, fixes) and `/ze-review` in an independent context |

### Resolving a row

| Row state found | Resolution | Evidence the row carries |
|-----------------|------------|--------------------------|
| The defect is fixed by the pass | `fixed: ` and what changed | the fix commit's short SHA, and the test that goes red without it |
| The defect was fixed earlier by unrelated work | `fixed: ` | the commit that fixed it, found with `git log -S` or `git log -L` on the producer, and the test or check that now covers it |
| The code the row names is gone at HEAD (stale) | `fixed: ` saying the producer was removed | the commit that removed it, and the search over HEAD that finds no remaining producer of the same shape; a renamed or moved producer is not stale and is followed instead |
| The defect is still open after the pass | stays `open: ` | the owned journal rows table line of the spec that will fix it, an existing spec whose mechanism fits or a new one written per `ai/rules/planning.md` |
| The row is wrong about what happened | `fixed: ` saying the report was mistaken | the producer read at HEAD that shows the behavior is correct |

Choosing the owning spec: an open row is owned by an existing spec whose
mechanism fits it; it gets a new spec of its own when no existing spec fits and
ANY of these holds.

| Condition | Why it needs a spec of its own |
|-----------|-------------------|
| The fix needs a design decision, an acceptance criterion, or more than one commit | a row holds no AC and no phase |
| Symptom and fix do not fit in 600 characters without dropping the cause | `RowCharsMax` refuses it, and a truncated cause is a lost one |
| The fix crosses subsystems or touches an RFC-tagged test | it needs a Files list and a review scope |
| Its resolution cannot be proven by one named test or check | a `fixed:` row would be a claim with nothing to arbitrate it |

When the owning spec closes, its commit A rewrites the row to `fixed: ` with
that commit's SHA, under the planning rule that repoints every citation.

### Resolving a class

A class is resolved when it is not DUE and carries a capture. A `check` capture
follows the README's five deliverables; a `rule` capture names the point and the
commit body says why no check can be built. A class whose every row is `fixed:`
and that carries no capture is still unresolved: it will collect rows again.

### Recording the pass

The pass's last commit rewrites its Pass Log row: result `check`, `rule`,
`split` or `abandoned`, the rows fixed, the rows promoted with their spec paths,
and the commit SHAs. The Pass Log is append-only otherwise.

## Pass Log

| Date | Session | Class | Result | Rows fixed / promoted | Commits |
|------|---------|-------|--------|-----------------------|---------|

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A row's Fix cell cites a spec by its repo-relative path under `plan/` | 32 committed `open:` rows already cite `plan/...spec-*.md` (grep, 2026-10-22) | the pointer check misses citations by bare stem | grep the journal for `spec-` without `plan/` before AC-2 lands | unvalidated |
| A-2 | Committing a `running` Pass Log row is visible to other sessions in time to prevent a double pass | sessions read HEAD; the report reads HEAD | two sessions run one class | the second session's claim commit conflicts on the same table row | unvalidated |
| A-3 | No existing gate already refuses a dangling `plan/` path inside a journal Fix cell | `ValidateFile` reads no paths; doc-link gates were not checked over `plan/journal/` | AC-2 duplicates an existing check | read the doc-links gate's file set before implementing | unvalidated |
| A-4 | The closure-candidate detector does not treat this spec as closable | `docs/contributing/spec-workflow.md` says committed rows only prompt a check of an in-progress spec | passes trigger false closure prompts | run the detector after the first pass | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Passes run per-row patches without a check, so DUE never drops | Pass Log rows with result `rule` dominate | README forbids a capture without steps 2 and 3; review checks the commit body states why no check exists |
| R-2 | Stale closure hides a live defect that moved | a `fixed:` stale row whose shape reappears in a later row | the stale rule demands a HEAD search for the same shape, and a moved producer is followed |
| R-3 | Promotion becomes the default, recreating the 2026-08-08 spec-per-defect route the journal rule supersedes | promoted rows outnumber fixed rows in a pass | the one-row test is ANY-of four concrete conditions; a row passing none stays a row |
| R-4 | A never-closing status weakens `writeSpecStatus`, letting any session edit source under this claim | source edits under the claim outside a running pass | the claim is taken only by a session holding a `running` Pass Log row (Claiming a pass, step 3) |
| R-5 | Ordering check gaps first starves the largest classes | the same uncaptured class leads the non-gap DUE list for many passes | check gaps are few (0 at HEAD today); each repair is small |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing user-visible: development tooling and process only |
| How is it reverted? | Single-commit revert per tooling phase |
| Who else touches this path? | Every session that writes a journal row; spec-lifecycle hooks |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le spec journal report` | → | `Report` sort | `TestReportOrdersCheckGapClassesFirst` |
| `./le spec journal report` | → | `Report` spec-pointer check | `TestReportNamesUnresolvedSpecPointer` |
| `./le spec journal validate` and the post-write hook | → | `ValidateFile` | `TestValidateFileRefusesOpenRowNamingMissingSpec` |
| spec write through Write/Edit | → | `validateSpecText` via `Vocabulary` | `TestValidateSpecTextAcceptsStanding` |
| source edit under a claimed standing spec | → | `writeSpecStatus` | `TestWriteSpecStatusAllowsStandingClaim` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | HEAD holds a check-captured class DUE at 1 counted row and an uncaptured class at 40 | the report prints the check-captured class first among DUE classes, then the rest by counted rows descending; text and JSON agree |
| AC-2 | HEAD holds an `open:` row whose Fix cell cites a spec path that does not exist at HEAD | the report names the class, the row date and the path under an unresolved-pointer line and still exits 0 |
| AC-3 | A row added in the working tree is `open:` and cites a `plan/` spec path that does not exist on disk | `./le spec journal validate` names the line with a problem kind for the unresolved pointer and exits non-zero; a `fixed:` row citing a closed spec is not refused |
| AC-4 | A spec declares `| Status | standing |` (if the owner approves the status) | the spec validation hook accepts it, `./le spec status` reports it, and the roadmap counts it as Backlog |
| AC-5 | The session's claimed spec is `standing` and the session edits a `.go` file | `writeSpecStatus` allows the edit; the permitted set is read from `internal/le/spec/status`, and the hook holds no status literal |
| AC-6 | A pass is run on a DUE class per this process | its Pass Log row names class, result, rows fixed and promoted, and SHAs; the class is no longer DUE in the report at the pass's last commit |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestReportOrdersCheckGapClassesFirst` | `internal/le/spec/journal/journal_test.go` | AC-1 | |
| `TestReportNamesUnresolvedSpecPointer` | `internal/le/spec/journal/journal_test.go` | AC-2 | |
| `TestValidateFileRefusesOpenRowNamingMissingSpec` | `internal/le/spec/journal/validate_test.go` | AC-3, including the `fixed:` row accepted | |
| `TestValidateSpecTextAcceptsStanding` | `internal/le/hookruntime/lifecycle_test.go` | AC-4 | |
| `TestWriteSpecStatusAllowsStandingClaim` | `internal/le/hookruntime/writeedit_test.go` | AC-5, and a `design` claim still refused | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| counted rows of a check-captured class | 0 and up | 1 is DUE | 0 is not DUE | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `TestReportActionPrintsUnresolvedPointer` | `internal/le/spec/journal/journal_test.go` | the native action, run over a fixture git tree, prints the pointer line | |

## Files to Modify
- `internal/le/spec/journal/journal.go` - check-gap ordering; unresolved spec pointers at HEAD
- `internal/le/spec/journal/validate.go` - unresolved pointer problem kind for added or rewritten `open:` rows
- `internal/le/spec/status/specstatus.go` - `standing` in `Vocabulary`, its `Category`, and the set of statuses that permit a source edit
- `internal/le/hookruntime/writeedit.go` - `writeSpecStatus` reads the permitted set from `internal/le/spec/status`
- `plan/journal/README.md` - the pointer problem kind and the report order
- `docs/contributing/spec-workflow.md` - the `standing` row in both status tables

## Files to Create
- None beyond tests named above.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | development tooling, no YANG |
| YANG validation constraints | N-A | no YANG |
| YANG custom validators | N-A | no YANG |
| CLI commands/flags | No | existing `./le spec journal` actions, output gains one line |
| CLI grammar (keyword before value) | N-A | no new arguments |
| Editor autocomplete | N-A | no YANG |
| Functional test for new RPC/API | N-A | no RPC; the action test above covers the report |
| Pipe completeness | No | the report's `| json` gains the pointer field, via the existing pipe path |
| Env var registration | N-A | no env var |
| Doctor check for runtime dependencies | N-A | no runtime dependency |
| Prometheus counters/metrics | N-A | no daemon state |
| BGP family surface (new SAFI / capability / attribute) | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | N-A | development process, not a product feature |
| 2 | Config syntax changed? | N-A | none |
| 3 | CLI command added/changed? | N-A | `ze` CLI untouched; `./le` output documented in `plan/journal/README.md` |
| 4 | API/RPC added/changed? | N-A | none |
| 5 | Plugin added/changed? | N-A | none |
| 6 | Has a user guide page? | N-A | none |
| 7 | Wire format changed? | N-A | none |
| 8 | Plugin SDK/protocol changed? | N-A | none |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | none |
| 10 | Test infrastructure changed? | No | the journal is not test infrastructure |
| 11 | Affects daemon comparison? | N-A | none |
| 12 | Internal architecture changed? | Yes | `plan/journal/README.md`, `docs/contributing/spec-workflow.md` |
| 13 | Route metadata keys added/changed? | N-A | none |
| 14 | Prometheus counters added/changed? | N-A | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | N-A | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/architecture/core-design.md` (declared by the `// Design:` header of every file changed) and `docs/contributing/spec-workflow.md` (anchors `internal/le/spec/status`); `./le spec citation anchors spec plan/post-release/engineering/spec-journal-resolution.md` at implementation |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | the report sample lines in `plan/journal/README.md` |

## Implementation Steps

1. **Phase: Wiring** -- the five wiring tests written and failing against the current report, validator and hooks.
   - Tests: all five Wiring Test rows
   - Files: the test files above
   - Verify: each fails for the missing behavior, not a fixture error
2. **Phase: report and validator** -- AC-1, AC-2, AC-3, with the README paragraph in the same phase.
   - Tests: the three journal tests and the action test
   - Files: `journal.go`, `validate.go`, `plan/journal/README.md`
   - Verify: red then green, the report over HEAD prints any existing unresolved pointers, and those found are listed in the phase's commit body
3. **Phase: standing status** -- AC-4, AC-5, only after the owner answers the status question; this spec's own Status becomes `standing` in the same commit.
   - Tests: the two hookruntime tests
   - Files: `specstatus.go`, `writeedit.go`, `docs/contributing/spec-workflow.md`
   - Verify: red then green; a `design` claim still blocks a source edit
4. **Passes** -- from then on, repeated: one class per pass under The Resolution Process, one Pass Log row each (AC-6). Never closes.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | the sort keeps non-DUE classes in name order; a `fixed:` row citing a removed spec is never refused |
| Data flow | thresholds and statuses each read from their one declaration |
| Rule: principles, single declaration | no status literal remains in `writeSpecStatus`; no threshold restated in this spec or a rule |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| check-gap ordering | `TestReportOrdersCheckGapClassesFirst` |
| pointer refusal | `TestValidateFileRefusesOpenRowNamingMissingSpec` |
| standing status | `TestValidateSpecTextAcceptsStanding`, `TestWriteSpecStatusAllowsStandingClaim` |
| first pass | a Pass Log row and the report at its last commit |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | a Fix-cell path with `..` or an absolute path is refused, never resolved outside the repository |

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
| No third Fix marker; stale and mistaken rows close as `fixed:` with evidence | a `stale:` or `wontfix:` marker | the README says a half still broken is open; a stale row's occurrence no longer exists, so it is repaired, and a new marker would reopen every consumer of the two |
| Pass order comes from the report, not this spec | a priority table in this spec | a rule MUST NOT copy what a command prints; the tool answers over the tree in hand |
| Check-gap classes first | largest counted rows first for all | a check gap is a gate that is wrong today; size measures history |
| Passes are Pass Log rows in this spec | one spec per pass | a spec per pass recreates the per-defect spec route the journal rule supersedes and costs a closure per pass |
| A never-closing status | keep this spec `in-progress` forever | `in-progress` claims implementation is under way and counts as unfinished work; owner question below |

## Known Limitations

- Owner question, status, ANSWERED 2026-10-22 (Thomas: "add standing"; spec approved: "approve"): Phase 3 adds the `standing` status ("a process spec that never closes; work is recorded as dated entries"), mapped to Backlog and permitted by `writeSpecStatus`; this spec becomes `standing` in that phase's commit.

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
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
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
- [ ] This spec never closes. Each tooling phase commits through `./le commit create` with its review recorded; each pass appends its Pass Log row
- [ ] `/ze-review` gate clean for each tooling phase, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`
