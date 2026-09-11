# Spec: the package map is derived on demand, and staleness is loud

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | `plan/spec-commit-stages-in-a-private-index.md` (both touch `internal/le/commit/prepare.go`) |
| Phase | 1-4/6 |
| Handoff | - |
| Updated | 2026-09-11 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`ai/PACKAGE-MAP.md` is a pure function of the tree that costs a commit-time gate,
a verify stage, a doc-wiring stage and an override keyword to keep in git. Every
one of those is the same byte comparison: `discoveryindex.Check`
(`internal/le/discoveryindex/report.go`) calls `survey`, which walks 769 packages
in 0.51 seconds and returns the content the file should hold, then compares that
against the file. The walk that decides the verdict is the walk that could have
written the answer.

Because the generator reads the shared WORKING TREE, a regeneration writes rows
for packages that only exist in another session's uncommitted work.
`plan/journal/concurrent-session-corruption.md` records that on 2026-08-23:
`internal/core/configorder` and `internal/core/configvalue` reached the tracked
map while no commit provided them. `plan/journal/gate-fires-outside-its-population.md`
and `gate-excludes-part-of-its-population.md` each carry a 2026-09-08 row about
the same gate refusing commits it should not and missing drift it should catch.

The route out is already proven here. `c03dbe18a8` (2026-08-18) stopped tracking
`ai/CODE-TO-DOCS.md` and `ai/DOCS-TO-CODE.md`, which have not appeared in a
commit since and rebuild in 1.1 seconds together. That change left one hole: the
rebuild in `hookSessionStart` (`internal/le/hookruntime/lifecycle.go`) fires on
ABSENCE, so an index that exists and no longer matches the tree is never rebuilt.
Edit a `// Design:` header and every later grep answers from before the edit,
with nothing to say so. `ai/rules/principles.md` forbids exactly that: a value
that is silently wrong must not be reachable.

This spec makes the three artifacts one mechanism. An artifact declares itself,
the inputs that feed it, and how to rebuild it. A write to one of its inputs
DELETES it. A read of it MATERIALISES it. Nothing compares a re-render against a
committed copy, because there is no committed copy.

The RFC ledger family and `docs/features/rfc-status.md` are the larger churn (17
of the last 200 commits, nine journal rows) and are NOT in this spec: their
readers parse the rendered markdown, so they are blocked behind
`plan/spec-derived-indexes-answer-a-query.md`, which owns that move.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - le's composition, one import per tool
  → Constraint: a tool is one package with one `register.go` and one `init()`; adding a
    derived-artifact registration must hang off that same `init()` rather than a new file
  → Decision: `internal/le/register.go` is the composition root that blank-imports every
    tool, so a registry populated by `init()` is only populated for a binary that imports it
- [ ] `docs/contributing/navigating-the-code.md` - the consumer contract for the indexes
  → Constraint: "Grep an index; do not read one" -- every consumer is a point query, so an
    absent artifact must announce itself rather than answer emptily
  → Decision: the page's index table names the three artifacts an agent greps, so it is the
    page this change makes wrong and must edit in the same work
- [ ] `docs/contributing/committing.md` - the commit keywords and their refusals
  → Constraint: `ai/PACKAGE-MAP.md` is named there as "the one derivable bookkeeping file
    still tracked", a sentence this spec makes false
- [ ] `ai/rules/documentation.md` - the page edit lands in the same work as the code
  → Constraint: every sentence this change falsifies is repaired in this spec's commit, not
    at closure and not in a follow-up

### Rules
- [ ] `ai/rules/principles.md`
  → Constraint: a new feature registers itself and is discovered; it must not require an edit
    to a switch, a case or any other central enumeration. The two hardcoded `os.Stat` blocks in
    `hookSessionStart` are that enumeration, and the registry replaces them
  → Constraint: a value that is silently wrong must not be reachable, which is why invalidation
    deletes rather than leaving a stale file in place
- [ ] `ai/rules/simplicity.md`
  → Decision: the artifacts with zero commits in the last 200 (`ai/rules/TRIGGERS.md`,
    `ai/rules/CORE.md`, `ai/rules/INDEX.md`, `docs/features/test-health.md`) stay tracked.
    Untracking them cuts no churn and risks a fresh clone resolving `@ai/rules/CORE.md` to nothing

**Key insights:** (minimal context to resume after compaction)
- `discoveryindex.IsSource(path, headerText)` already exists as a standalone predicate with two
  independent callers, and derives from the same `roots`/`skipDirs` the generator walks. It is
  the invalidation predicate, already written.
- `nativeHookActions` (`internal/le/hookruntime/runtime.go`) already dispatches
  `posttool-writeedit` and `pretool-bash`. Both entry points this design needs exist.
- `.gitignore` has no effect on a tracked path: `git check-ignore` consults the index. The
  removal and the ignore entry must land in one commit.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/le/discoveryindex/report.go` - `survey` walks and renders, `Check` compares
      against the file, `Update` writes it. Absence reads as `""`, so `Stale` is true
- [ ] `internal/le/discoveryindex/sources.go` - `IsSource`, `HeaderText`, `feeds`, `inPopulation`
- [ ] `internal/le/commit/prepare.go` - `checkDiscoveryIndex` has two refusals: a commit that
      changes an index-feeding source and omits `ai/PACKAGE-MAP.md`, and a stale report
- [ ] `internal/le/commit/debt.go` - `gateStaleIndexOK` is the `stale-index-ok` keyword and
      `{gateStaleIndexOK, "discovery-index freshness", true, nil}` is its debt row
- [ ] `internal/le/hookruntime/lifecycle.go` - `hookSessionStart` stats two paths and rebuilds
      each when absent
- [ ] `internal/le/hookruntime/runtime.go` - `nativeHookActions` maps hook name to check list
- [ ] `internal/le/hookruntime/postwrite.go` - the nine `posttool-writeedit` checks
- [ ] `internal/le/verify/engine/stages.go` - `fullStages` carries `discovery-index check`
- [ ] `internal/le/doc/wiring/docverify.go` - `discoveryIndexesStage` is the second caller
- [ ] `internal/le/repository/generate.go` - `generationActions` and `generationChecks`

**Behavior to preserve:**
- `./le discovery-index update` keeps writing `ai/PACKAGE-MAP.md` and keeps its report shape
  (`Report.Packages`, `| json`), because `internal/test/fixture/ui_fixture_le_discovery_answers.go`
  and the UI fixtures assert on it
- `./le docs-to-code update` and `index-update` are unchanged as writers
- `discoveryindex.IsSource` and `HeaderText` are DELETED, not preserved: their only production
  callers were `checkDiscoveryIndex` and `isDiscoverySource`, and this spec deletes both. The
  invalidation hook needs a predicate over the path alone, which is `IsSourcePath`
- Every rendered rule under `ai/rules/*.md` stays tracked and keeps `rules render-check` and
  `rules points-roundtrip-check`: those gate an AUTHORED source against its render, not a
  derivation against the tree

**Behavior to change:**
- `ai/PACKAGE-MAP.md` leaves git and is rebuilt on demand
- Staleness stops being invisible: an input write deletes the artifact, a read materialises it
- `checkDiscoveryIndex`, the `stale-index-ok` keyword and its debt row are deleted
- `discoveryindex.Check` and the three stages that call it are deleted
- `hookSessionStart` stops naming artifacts and iterates the registry

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A `Write` or `Edit` tool call on any file (the `posttool-writeedit` hook payload)
- A `Bash` tool call whose command text names a registered artifact path (the `pretool-bash` payload)
- A session start (the `SessionStart` hook payload)

### Transformation Path
1. Each generator's `init()` calls `derived.Register` with its output path, its input predicate
   and its rebuild function, beside the existing `leroot.Register`
2. `postInvalidateDerived` reads `derived.All()`, asks each artifact whether the written path is
   one of its inputs, and removes the output of every artifact that answers yes
3. `preMaterialiseDerived` reads `derived.All()`, finds the artifacts whose path appears in the
   Bash command text, and rebuilds any that is absent before the command runs
4. `hookSessionStart` iterates `derived.All()` and rebuilds every artifact that is absent

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| generator package → registry | `derived.Register` from the generator's own `init()` | No |
| registry → hook runtime | `derived.All()`, read-only | No |
| hook runtime → filesystem | `os.Remove` of the output, `Rebuild(root)` to write it | No |

### Integration Points
- `internal/le/discoveryindex/register.go` - registers `ai/PACKAGE-MAP.md`, predicate `IsSource`
- `internal/le/docstocode/register.go` - registers the two indexes the hook hardcodes today
- `internal/le/hookruntime/runtime.go` - `postInvalidateDerived` joins the `posttool-writeedit`
  list, `preMaterialiseDerived` joins `pretool-bash`
- `internal/le/register.go` - already blank-imports both generator packages, so the registry is
  populated for every `le` binary with no new import

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | `hookruntime` imports `derived` only; it drops its direct `docstocode` calls |
| No duplicated functionality (extends existing, does not recreate) | No | `IsSource` is reused, not re-derived |
| Zero-copy preserved where applicable (refs, not copies) | No | N-A: no wire or buffer path |
| Registration over hardcoding, outbound | No | a new derived artifact is one `derived.Register` in its own package |
| Registration over hardcoding, inbound | No | `hookSessionStart` and both hooks iterate `derived.All()`; no list learns an artifact's name |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `discoveryindex.IsSource` answers the same population the generator walks | `feeds` and `inPopulation` read the same `roots`/`skipDirs` as `Build`/`scanDir` | invalidation misses an input and the map silently keeps a stale row, which is the defect this spec exists to remove | `TestFeedsNamesTheIndexEachSourceDrifts` over the predicate's whole population, and `TestEditingAnIndexFeedingSourceRemovesThePackageMap` over the hook | confirmed |
| A-2 | `pretool-bash` receives the full command text, so a grep naming the artifact path can be recognized | `nativeHookActions` dispatches `pretool-bash`; `context` carries `input` | the read path never materializes, and a grep hits an absent file | `TestReadingAnAbsentArtifactMaterializesItBeforeTheCommandRuns`, and `test/runner/le-derived-artifact-lifecycle.ci` through the real binary | confirmed |
| A-3 | No CI job reads `ai/PACKAGE-MAP.md` | no `.github/workflows/` file names it; `verify.yml` runs `./le verify list mode full` | CI reds on a fresh checkout | `grep -rn "PACKAGE-MAP\|discovery-index" .github/` answers nothing | confirmed |
| A-4 | The thirteen tests setting `Options.StaleIndexOK` are about something else and keep passing once the field goes | all thirteen carry the same reason string, "fixture repository has no generated index" | the untracking lands with red commit tests | thirteen assignments removed across four files; `go test ./internal/le/commit/` green | confirmed |
| A-5 | Removing `ai/PACKAGE-MAP.md` from git does not orphan a verification-debt row | `debtGateAt` returns -1 for an undeclared gate name and an unrecognized row is never cleared | a `plan/verification-debt/*.md` row naming "discovery-index freshness" becomes permanently uncleanable | grep found 326 rows across 110 shards, 290 of them open | BROKEN: the `debtGates` row MUST NOT be deleted. `TestEveryLedgerGateNameIsDeclared` (`internal/le/commit/ledger_test.go`) refuses a ledger row no table declares, and `clearDebtWith` never clears one. The declaration stays; the KEYWORD and the gate are gone |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The artifact is absent for most of a session, because nearly every `.go` edit invalidates it, so every read pays a 0.51 s rebuild | a session reports repeated rebuild lines | the read path materialises on demand, so the cost is one rebuild per read-after-write rather than one per edit. If it still bites, raise the artifact to rebuild-on-write |
| R-2 | An agent reads the artifact with the `Read` or `Grep` TOOL rather than Bash, which no hook intercepts, and meets an absent file | a session reports "file does not exist" for a path a rule told it to grep | the failure is loud and names the command; `docs/contributing/navigating-the-code.md` gains the sentence. A silent stale answer, which is today's behavior, is the worse failure |
| R-3 | `.gitignore` is added without the removal, so the entry does nothing and the file keeps being committed | `git check-ignore -v ai/PACKAGE-MAP.md` reports nothing | one commit carries both, and `./le commit create` names the removal with `remove` |
| R-4 | Deleting `discoveryindex.Check` breaks a caller this spec did not find | build failure in `doc/wiring` or `repository` | `gopls references` on `Check` before the deletion, named in the implementation step |
| R-5 | Another session is mid-commit when the keyword disappears, and its prepared script names `stale-index-ok` | that session's script fails at run time with an unknown keyword | the keyword is removed from the parser and the debt vocabulary in one commit; a prepared script older than the commit is regenerated, which `./le commit create` already requires |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing an operator meets. The failure surface is developer tooling: a commit refused, a verify stage red, or an agent meeting an absent index |
| How is it reverted? | Single commit revert. The artifact is a build product either way, so no data is lost by reverting in either direction |
| Who else touches this path? | `plan/spec-commit-stages-in-a-private-index.md` (ready) edits `internal/le/commit/prepare.go`; coordinate the `checkSourceGates` edit with it |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le hook-check posttool-writeedit` with a payload naming an edited `register.go` | → | `postInvalidateDerived` → `derived.All()` → `os.Remove` | `TestEditingAnIndexFeedingSourceRemovesThePackageMap` |
| `./le hook-check pretool-bash` with a command naming `ai/PACKAGE-MAP.md` | → | `preMaterialiseDerived` → `Rebuild` | `TestReadingAnAbsentArtifactMaterialisesItBeforeTheCommandRuns` |
| `./le hook-check session-start` in a tree with no derived artifact | → | `hookSessionStart` → `derived.All()` | `TestSessionStartBuildsEveryRegisteredDerivedArtifact` |
| `./le commit create` over a commit changing a package doc comment | → | `checkSourceGates` with no index gate | `TestACommitChangingAPackageCommentNeedsNoIndex` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `git ls-files ai/PACKAGE-MAP.md` after this lands | answers nothing, and `git check-ignore -v ai/PACKAGE-MAP.md` names the `.gitignore` line |
| AC-2 | a commit that changes a package doc comment and carries no index | is prepared and runs, with no refusal and no debt row |
| AC-3 | `stale-index-ok` passed to `./le commit create` | is refused as an unknown keyword |
| AC-4 | a `Write` to a file `IsSource` accepts | `ai/PACKAGE-MAP.md` no longer exists afterwards |
| AC-5 | a `Write` to a file `IsSource` rejects, for example a `.md` under `docs/` | `ai/PACKAGE-MAP.md` is untouched, with the same mtime |
| AC-6 | a Bash command naming `ai/PACKAGE-MAP.md` while the file is absent | the file exists and matches a fresh `survey` before the command runs |
| AC-7 | a session start in a tree where all three derived artifacts are absent | all three exist afterwards, and the hook prints one line per artifact it built |
| AC-8 | `./le verify current mode full` | carries no stage whose only verdict is a byte comparison against a committed derived copy; `discovery-index check` is absent from the stage list |
| AC-9 | a new derived artifact added by a future generator | needs one `derived.Register` call in its own package and no edit to `internal/le/hookruntime` |
| AC-10 | `./le discovery-index update` | still writes the map and still answers the same `Report` shape through `\| json` |
| AC-11 | a write that invalidates a registered artifact whose path git still TRACKS | the artifact is left in place, unmodified, and the hook says the registration and the tracking disagree |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | edits a Go file, then greps the package map for what a package does | Write → `postInvalidateDerived` removes it → Bash grep → `preMaterialiseDerived` rebuilds it → grep reads a map that includes the edit | `TestAGrepAfterAnEditReadsTheEditedPackage` |
| 2 | commits work that adds a package, without regenerating anything | `./le commit create` → `checkSourceGates` → no index gate → script runs | `TestACommitChangingAPackageCommentNeedsNoIndex` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRegisteredArtifactsCarryAPredicateAndARebuild` | `internal/le/derived/derived_test.go` | every registered artifact answers a non-nil predicate and rebuild, so a half-registered artifact cannot be silently inert | |
| `TestRegisterRefusesADuplicatePath` | `internal/le/derived/derived_test.go` | two artifacts cannot claim one output path | |
| `TestEditingAnIndexFeedingSourceRemovesThePackageMap` | `internal/le/hookruntime/postwrite_test.go` | the invalidation predicate is wired to the hook | |
| `TestEditingANonSourceLeavesEveryArtifactInPlace` | `internal/le/hookruntime/postwrite_test.go` | invalidation is scoped, not blanket | |
| `TestReadingAnAbsentArtifactMaterialisesItBeforeTheCommandRuns` | `internal/le/hookruntime/bash_test.go` | the read path closes the loop | |
| `TestMaterialisingLeavesAPresentArtifactAlone` | `internal/le/hookruntime/bash_test.go` | the read path does not rewrite on every read, which would be `check-mode-mutates-the-tree` again | |
| `TestSessionStartBuildsEveryRegisteredDerivedArtifact` | `internal/le/hookruntime/lifecycle_test.go` | the loop replaced the hardcoded pair | |
| `TestACommitChangingAPackageCommentNeedsNoIndex` | `internal/le/commit/commit_test.go` | the commit gate is gone | |
| `TestStaleIndexOKIsNotAKeyword` | `internal/le/commit/commit_test.go` | the override and its debt row are gone together | |
| `TestAGrepAfterAnEditReadsTheEditedPackage` | `internal/le/derived/derived_test.go` | story 1 end to end, over a temporary checkout | |

### Boundary Tests (numeric inputs)
N-A: this spec introduces no numeric input.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `le-derived-artifact-lifecycle` | `test/runner/le-derived-artifact-lifecycle.ci` | a user edits a source file, greps the map, and reads a row that reflects the edit | |

### Interop Tests (Scope: protocol)
N-A: Scope is tooling, no wire-visible behavior.

## Files to Modify
- `internal/le/discoveryindex/register.go` - register the artifact beside the existing `leroot.Register`
- `internal/le/discoveryindex/report.go` - delete `Check`; `survey` and `Update` stay
- `internal/le/discoveryindex/actions.go` - drop the `check` verb
- `internal/le/docstocode/register.go` - register both doc indexes
- `internal/le/hookruntime/runtime.go` - add `postInvalidateDerived` and `preMaterialiseDerived` to their check lists
- `internal/le/hookruntime/postwrite.go` - `postInvalidateDerived`
- `internal/le/hookruntime/lifecycle.go` - `hookSessionStart` iterates `derived.All()`
- `internal/le/commit/prepare.go` - delete `checkDiscoveryIndex`, its call in `checkSourceGates`, and `Options.StaleIndexOK`
- `internal/le/commit/debt.go` - delete `gateStaleIndexOK` and its `debtGates` row
- `internal/le/commit/actions.go` - delete the keyword from the parser and the values table
- `internal/le/verify/engine/stages.go` - delete the `discovery-index check` row
- `internal/le/doc/wiring/docverify.go` - delete the package-map arm of `discoveryIndexesStage`
- `internal/le/repository/generate.go` - drop the `discovery-index check` row from `generationChecks`
- `internal/le/commit/*_test.go` - thirteen `StaleIndexOK` assignments, and the two tests that call `checkDiscoveryIndex`
- `internal/test/fixture/ui_fixture_le_discovery_answers.go` - the `check` answers
- `cmd/ze/ze_le_personality_test.go` - the `discovery-index check` expectation
- `.gitignore` - `/ai/PACKAGE-MAP.md`, beside the two entries `c03dbe18a8` added
- `docs/contributing/navigating-the-code.md` - say that an absent index is rebuilt on read, and that a grep never reads a stale one
- `docs/contributing/committing.md` - delete the paragraph naming `ai/PACKAGE-MAP.md` as the one tracked derivable file
- `ai/INDEX.md` - "gated fresh, so they never lie about the current code" is false; say derived on demand
- `docs/contributing/go-conventions.md` - the `<!-- source: ai/PACKAGE-MAP.md -->` anchor
- `docs/architecture/core-design.md` - the derived-artifact registry is a new composition seam
- `docs/architecture/testing/verify-freshness-scope.md` - declared by `internal/le/commit/debt.go`
  and `internal/le/verify/engine/stages.go`: the freshness scope loses the discovery-index gate,
  and the debt vocabulary loses one row
- `docs/features/ai-first.md` - declared by `internal/le/commit/prepare.go`: the commit keyword
  surface loses `stale-index-ok`

## Files to Create
- `internal/le/derived/derived.go` - `Artifact`, `Register`, `All`
- `internal/le/derived/derived_test.go`
- `test/runner/le-derived-artifact-lifecycle.ci`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | no operator-facing config; this is `le` developer tooling |
| YANG validation constraints | N-A | as above |
| YANG custom validators | N-A | as above |
| CLI commands/flags | No | no new verb: `./le discovery-index update` is unchanged and `check` is deleted |
| CLI grammar (keyword before value) | N-A | no new grammar |
| Editor autocomplete | N-A | no YANG leaf |
| Functional test for new RPC/API | Yes | `test/runner/le-derived-artifact-lifecycle.ci` |
| Pipe completeness | N-A | the `Report` shape is unchanged and already pipes |
| Env var registration | N-A | no env var |
| Doctor check for runtime dependencies | N-A | no new file path, socket, port, module or binary at daemon runtime; the artifact is a developer build product |
| Prometheus counters/metrics | N-A | no daemon-observable state |
| BGP family surface | N-A | Scope is tooling |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | N-A | developer tooling only |
| 2 | Config syntax changed? | N-A | no config |
| 3 | CLI command added/changed? | Yes | `./le discovery-index check` is deleted: `docs/guide/command-reference.md` if it names the verb |
| 4 | API/RPC added/changed? | N-A | no RPC |
| 5 | Plugin added/changed? | N-A | no plugin |
| 6 | Has a user guide page? | Yes | `docs/contributing/navigating-the-code.md` |
| 7 | Wire format changed? | N-A | no wire |
| 8 | Plugin SDK/protocol changed? | N-A | no SDK change |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | no RFC surface: the RFC artifacts are the other spec |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` for the new `.ci` |
| 11 | Affects daemon comparison? | N-A | no daemon behavior |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` gains the derived-artifact seam |
| 13 | Route metadata keys added/changed? | N-A | no route metadata |
| 14 | Prometheus counters added/changed? | N-A | none |
| 15 | Registered plugin, event type, command or inventory changed? | Yes | the `discovery-index` verb list changes: `docs/guide/status.md` if it enumerates verbs |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED: `./le spec citation anchors spec plan/spec-derived-artifacts-are-not-committed.md`, run before the review round |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/contributing/committing.md` shows the `stale-index-ok` keyword |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the registry exists and the hooks reach it
   - Tests: `TestRegisteredArtifactsCarryAPredicateAndARebuild`, `TestSessionStartBuildsEveryRegisteredDerivedArtifact`
   - Files: `internal/le/derived/derived.go`, the three `register.go` files, `internal/le/hookruntime/lifecycle.go`
   - Verify: the session-start loop builds what the two hardcoded blocks built, and the tests fail before the registry exists
2. **Phase: Invalidate on write**
   - Tests: `TestEditingAnIndexFeedingSourceRemovesThePackageMap`, `TestEditingANonSourceLeavesEveryArtifactInPlace`
   - Files: `internal/le/hookruntime/postwrite.go`, `runtime.go`
   - Verify: `IsSource` decides, and a docs edit removes nothing
3. **Phase: Materialise on read**
   - Tests: `TestReadingAnAbsentArtifactMaterialisesItBeforeTheCommandRuns`, `TestMaterialisingLeavesAPresentArtifactAlone`
   - Files: `internal/le/hookruntime/runtime.go` and the `pretool-bash` check
   - Verify: a present artifact is never rewritten, so no read mutates the tree
4. **Phase: Delete the gates**
   - Tests: `TestACommitChangingAPackageCommentNeedsNoIndex`, `TestStaleIndexOKIsNotAKeyword`
   - Files: `commit/prepare.go`, `commit/debt.go`, `commit/actions.go`, `verify/engine/stages.go`,
     `doc/wiring/docverify.go`, `repository/generate.go`, `discoveryindex/report.go`, `discoveryindex/actions.go`
   - Verify: `gopls references` on `discoveryindex.Check` is empty before the deletion; grep
     `plan/verification-debt/` for "discovery-index freshness" before deleting the debt row
5. **Phase: Untrack, in ONE commit with the ignore entry**
   - Files: `.gitignore`, and `rm ai/PACKAGE-MAP.md` passed to `./le commit create ... remove`
   - Verify: `git check-ignore -v` names the line, and the file rebuilds from the hook afterwards
6. **Phase: Documentation**
   - Files: every row of the Documentation Update Checklist answered Yes
   - Verify: `./le spec citation anchors spec plan/spec-derived-artifacts-are-not-committed.md` is clean

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC-N has an implementation at file:line, and AC-9 is proved by a registration that touches no central list |
| Correctness | invalidation removes exactly the artifacts whose predicate accepts the path, and the read path never rewrites a present file |
| Guard removal | `checkDiscoveryIndex` is a GUARD being deleted: state what it protected and why nothing is now unprotected. The answer must be that a derived file cannot drift from a tree it is rebuilt from |
| Fail-closed | `Rebuild` failing in the read path must say so, never leave the command reading a file it did not build |
| Data flow | no generator imports `hookruntime`; the dependency runs one way |
| Rule: `ai/rules/principles.md` | the registry is discovered, not enumerated; show the diff adds no case to a switch |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `ai/PACKAGE-MAP.md` untracked | `git ls-files ai/PACKAGE-MAP.md` is empty |
| the override is gone | `grep -rn "stale-index-ok" internal/ docs/ ai/` finds nothing outside history |
| no byte-compare stage remains for it | `./le verify list mode full \| grep discovery-index` is empty |
| the registry is the only enumeration | `grep -n "CODE-TO-DOCS" internal/le/hookruntime/` is empty |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Path handling | the artifact path comes from the registry, never from hook payload text; a Bash command naming a path must not cause a write outside the checkout root |
| Removal safety | `postInvalidateDerived` removes ONLY registered output paths, and a registration is refused if its path is tracked in git |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Review Gate

Round 1 scope: the WHOLE diff of this spec's phases 1 to 6, over every file named
in Files to Modify and Files to Create, plus `.gitignore`, `internal/le/commit/debt.go`
and `plan/verification-debt/discharged/`. At least two lenses. The eight
always-in-scope classes apply wherever they surface.

Round 2 scope: the round 1 fixes only, plus the sibling call sites they touched.
Round 3 scope: the round 2 fixes only, which are commit `38c3c8882`, plus the
sibling call sites they touched. The eight always-in-scope classes apply in every
round wherever they surface.

| Round | Scope | BLOCKER | ISSUE | NOTE | Recorded |
|-------|-------|---------|-------|------|----------|
| 1 | whole diff, two lenses | 2 | 6 | 3 | fixed, then re-reviewed as round 2 |
| 2 | the round 1 fixes | 2 | 3 | 1 | fixed in `38c3c8882` |
| 3 | the round 2 fixes (`38c3c8882`) | 0 | 1 | 4 | fixed below |
| 4 | the round 3 fix only: the corrected weakening reason and the `hookSessionStart` budget comment | | | | |

Round 3's ISSUE was a FALSE reason in `test/weakened/4350711e.md`, which said the
inverted session-start test had put "a 4 to 6 second rebuild" inside a 5 second
budget. Measured, the rebuild is about one second and the hook's own debt-ledger
read is the rest. The row now says that, and the `hookSessionStart` comment says
it too, so neither sends the next reader at the rendering.

## Design Insights

- The registry went live in a shared checkout while `ai/PACKAGE-MAP.md` was still
  tracked, so for the length of that window the invalidation hook was deleting a file
  git holds. Git reads that as a deletion to stage, and another session's commit script
  would have carried it. The removal path now refuses a tracked path outright (AC-11).
  The lesson generalises: a mechanism that DELETES must not trust a registration to
  have finished the migration that makes the deletion safe.
- The verification-debt ledger had no vocabulary for a RETIRED gate.
  `dischargeRow` refuses a discharge for a gate declared `Runnable`, and
  `verifyKindNotApplicable` carries derivations for two gates only, so the 290 open
  rows naming this gate could be neither cleared nor discharged. `Runnable` became
  false because it is now false, and the rows were discharged under `kind owner`
  with the owner's authorisation of 2026-09-11.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Invalidate by DELETING, not by rebuilding | rebuild in the write hook; stamp the header with an input digest and warn | a rebuild on every write costs 0.51 s per edit for an artifact most edits never read; a stamp leaves the stale file where a grep still reads it, which is the reader that has the defect today. Owner chose delete on 2026-09-11 |
| Materialise in the `pretool-bash` hook | let the reader meet an absent file and run the named command itself | the artifact would be absent for most of a session, so every consumer would pay. Materialising on read makes the cost one rebuild per read-after-write, which is the minimum possible |
| Leave `TRIGGERS.md`, `CORE.md`, `ai/rules/INDEX.md` and `docs/features/test-health.md` tracked | untrack every derived artifact uniformly | measured: zero commits each in the last 200, against five for the package map and seventeen for the RFC family. Untracking them cuts no churn, and `CLAUDE.md` resolves `@ai/rules/CORE.md` before any hook can rebuild it |
| Leave `website/data/repo-facts.json` tracked | untrack with the rest | it is a snapshot of git index state that a build cannot re-derive, and `docs/architecture/site-facts.md` argues for committing it. It is a record, not a derivation |
| A new `internal/le/derived` package | hang the metadata off `leroot.Register`'s `registry.Meta`, or off `leaction.Action` | neither carries an output path or an input predicate, and `leroot.GroupGenerate` names the tool rather than the artifact and omits `rfc` entirely. A separate registry imports cleanly into `hookruntime` with no cycle |

## Known Limitations
- The RFC ledger family (`ai/RFC-REQUIREMENTS.md`, `rfc/requirements/*.md`, `rfc/enrolled.txt`,
  `rfc/not-enrolled.txt`) and `docs/features/rfc-status.md` stay tracked here. They carry the
  larger churn and all nine rows of `plan/journal/concurrent-rfc-gate-stale.md`, and they are
  blocked behind readers that parse the rendered markdown:
  `plan/spec-derived-indexes-answer-a-query.md`.
- The `Read` and `Grep` TOOLS are not intercepted, only Bash. An agent using them on an absent
  artifact meets a missing file rather than a rebuilt one.
- Invalidation is keyed to the `Write` and `Edit` TOOLS (`nativeHookActions`, `posttool-writeedit`),
  so a write that reaches a file another way moves an input with no hook in the path: `sed -i`,
  a shell heredoc, `git rebase`, `git stash pop`, `git checkout` and `./le repository generate`.
  Each leaves the artifact PRESENT and stale. Neither `preMaterializeDerived` nor `hookSessionStart`
  rebuilds one that is present, so it stays stale until the next `Write` or `Edit` to one of its
  inputs, or until a generator is run by hand.
- `hookSessionStart` rebuilds only an ABSENT artifact, because `.claude/settings.json` gives the
  hook 5 seconds and rendering all three does not fit. A hook killed at its timeout leaves every
  artifact after the kill point untouched AND discards the whole session-start message, the
  BLOCKING LSP notice and the verification-debt warning included, so the unconditional rebuild is
  strictly worse than the staleness it was meant to remove. Measure before revisiting:
  `echo '{}' | time ./le hook-check session-start`.
- `preMaterializeDerived` matches the artifact's path, or a directory holding it WITH a trailing
  slash. A search naming neither reads a tree without the artifact and answers no match:
  `rg <symbol>`, `grep -rn foo .`, `grep -rn X ai`, and any `./le` action that reads an artifact
  in its own process. Catching those has no bound, so the limit is stated in
  `commandNamesArtifact` and in `docs/contributing/navigating-the-code.md` instead.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional `.ci` tests for end-to-end behavior

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
