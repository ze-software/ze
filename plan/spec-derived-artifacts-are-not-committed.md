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
sibling call sites they touched. Round 4 scope: the round 3 fix only, which is
commit `940fb364f`. Round 5 scope: the round 4 fix only, which is the
`hookSessionStart` comment naming `commit.ListDebt` as the call above it, and
the Review Gate prose that records why the correction cannot land in the
weakening row. The eight always-in-scope classes apply in every round wherever
they surface.

Each round ran in its own independent `ze-read` context on Opus 5, so no round
judged a fix its own context wrote.

| Round | Scope | BLOCKER | ISSUE | NOTE | Recorded |
|-------|-------|---------|-------|------|----------|
| 1 | whole diff, two lenses | 2 | 6 | 3 | fixed, then re-reviewed as round 2 |
| 2 | the round 1 fixes | 2 | 3 | 1 | fixed in `38c3c8882` |
| 3 | the round 2 fixes (`38c3c8882`) | 0 | 1 | 4 | fixed in `940fb364f` |
| 4 | the round 3 fix only (`940fb364f`) | 0 | 1 | 0 | fixed in this closure's commit A |
| 5 | the round 4 fix only | 0 | 0 | 0 | loop closed |

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/derived-artifacts-are-not-committed-ce8ca907-f5a8-42ae-8448-e021346de13d.md` |
| `./le spec session review check` | clean |
| Rounds | 5 |
| Reviewer lenses used | round 1 ran logic-and-wiring plus security-and-edge-cases; rounds 2 to 5 each ran the always-in-scope classes over the previous round's fixes |

### Findings fixed

| # | Round | Severity | Finding | Location | Fixed by |
|---|-------|----------|---------|----------|----------|
| 1 | 1 | BLOCKER | the invalidation hook deleted a path git still TRACKS, which stages a deletion another session's commit script would carry | `postInvalidateDerived` (`internal/le/hookruntime/postwrite.go`) | the removal path refuses a tracked path and says the registration and the tracking disagree (AC-11), `TestATrackedArtifactIsNeverRemoved` |
| 2 | 1 | BLOCKER | deleting the `debtGates` row for the retired gate stranded 290 open ledger rows: `debtGateAt` answers -1 for an undeclared gate and `clearDebtWith` never clears one | `debtGates` (`internal/le/commit/debt.go`) | the declaration stays with `Runnable` false; the KEYWORD and the gate are gone. `TestEveryLedgerGateNameIsDeclared` |
| 3 | 2 | BLOCKER | the session-start rebuild was unconditional and did not fit the hook's 5 second budget: 4.3, 4.8 and 5.9 seconds over three warm runs, and a killed hook discards the whole session-start message | `hookSessionStart` (`internal/le/hookruntime/lifecycle.go`) | absent-only rebuild in `38c3c8882`, `TestSessionStartLeavesAPresentArtifactAlone` |
| 4 | 2 | BLOCKER | `IsSource`, `HeaderText`, `feeds`, `hasPackageHeader` and `generator` were dead the moment `7629e9013` deleted their two callers, and `./le repository check` cannot see it because it reads only changed files | `internal/le/discoveryindex/sources.go` | the five symbols deleted in `38c3c8882` |
| 5 | 3 | ISSUE | `test/weakened/4350711e.md` gave a FALSE reason: it put "a 4 to 6 second rebuild" inside a 5 second budget, when the rebuild is about one second and the hook's own `commit.ListDebt` read is most of the rest | `test/weakened/4350711e.md` | the measured shape written into the `hookSessionStart` comment in `940fb364f` |
| 6 | 4 | ISSUE | the round 3 correction could not land where the error is, because `PruneLanded` empties the weakening shard once its commit lands, so `940fb364f`'s message claiming "the row says the measured shape now" is false as committed | `PruneLanded` (`internal/le/testweakened/shard.go`), called from `internal/le/commit/prepare.go` | the comment says `commit.ListDebt` is the call ABOVE it rather than below, and the Review Gate prose below records why the row cannot carry the correction |

The table carries every BLOCKER and every ISSUE of rounds 3, 4 and 5, and the
four BLOCKERs of rounds 1 and 2. The nine round 1 and round 2 ISSUEs are not
itemized here: they were fixed inside `7629e9013` and `38c3c8882`, and the
record of each lives in the round's own context rather than in this file. What
is recoverable from the tree is the fix, not the finding.

Round 3's ISSUE was a FALSE reason in `test/weakened/4350711e.md`, which said the
inverted session-start test had put "a 4 to 6 second rebuild" inside a 5 second
budget. Measured, the rebuild is about one second and the hook's own debt-ledger
read is most of the rest. The share is not written down here: the two numbers
this work produced for it disagree, and a third guess is what round 3 was about.

The correction could not land where the error is, and round 4 caught that. A
weakening row is consumed by the commit it explains: `PruneLanded`
(`internal/le/testweakened/shard.go`, called from `internal/le/commit/prepare.go`)
drops every row once its commit lands, so `test/weakened/4350711e.md` is empty at
HEAD and the false reason survives only inside `38c3c8882`'s own tree, where no
later edit can reach it. `940fb364f`'s message claiming "the row says the measured
shape now" is false as committed, for that reason. What carries the correction is
the `hookSessionStart` comment, which is in the code a reader actually opens.

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

---

## Implementation Summary

### What Was Implemented
- `internal/le/derived` is a registry of derived artifacts. One `Artifact` carries
  an output path, a `Feeds(root, path)` predicate over its inputs, and a `Rebuild`.
  `Register` refuses an empty, absolute or climbing path, a duplicate path, a nil
  predicate and a nil rebuild, so a half-registered artifact cannot be inert.
- Three artifacts register from their own generator's `init()`:
  `ai/PACKAGE-MAP.md` (`internal/le/discoveryindex/register.go`), `ai/DOCS-TO-CODE.md`
  and `ai/CODE-TO-DOCS.md` (`internal/le/docstocode/register.go`).
- The lifecycle is three hooks over `derived.All()`, none of which names an artifact:
  `postInvalidateDerived` (`internal/le/hookruntime/postwrite.go`) REMOVES every
  artifact whose predicate accepts the written path, `preMaterializeDerived`
  (`internal/le/hookruntime/bash.go`) builds an ABSENT artifact a Bash command
  names before that command runs, and `hookSessionStart`
  (`internal/le/hookruntime/lifecycle.go`) builds every artifact the tree does
  not hold.
- `ai/PACKAGE-MAP.md` left git and gained a `/ai/PACKAGE-MAP.md` line in
  `.gitignore`, in the one commit that removed it.
- Every gate that byte-compared the committed copy is deleted: `checkDiscoveryIndex`
  and `Options.StaleIndexOK` (`internal/le/commit/prepare.go`), the `stale-index-ok`
  keyword (`internal/le/commit/actions.go`), `discoveryindex.Check` and the
  `check` verb, the `discovery-index check` row of `fullStages`
  (`internal/le/verify/engine/stages.go`) and of `generationChecks`
  (`internal/le/repository/generate.go`), and the package-map arm of
  `discoveryIndexesStage` (`internal/le/doc/wiring/docverify.go`).
- The 290 open verification-debt rows naming `discovery-index freshness` were
  discharged under `kind owner`, on the owner's authorisation of 2026-09-11. The
  `debtGates` DECLARATION stays, with `Runnable` false.

### Bugs Found/Fixed
- The invalidation hook would have deleted a path git still TRACKS while the
  migration was half done, staging a deletion another session's commit script
  would have carried. Fixed by refusing a tracked path outright, and by failing
  CLOSED when the checkout cannot be read: `TestATrackedArtifactIsNeverRemoved`
  and `TestTrackedFailsClosedWhenTheCheckoutCannotBeRead`
  (`internal/le/hookruntime/postwrite_test.go`).
- Deleting the retired gate's `debtGates` row would have stranded 290 open ledger
  rows for good, because `debtGateAt` answers -1 for an undeclared gate name and
  `clearDebtWith` never clears one. Covered by `TestEveryLedgerGateNameIsDeclared`
  (`internal/le/commit/ledger_test.go`).
- The unconditional session-start rebuild did not fit the hook's 5 second budget.
  Fixed to absent-only in `38c3c8882`; `TestSessionStartLeavesAPresentArtifactAlone`.
- Five symbols in `internal/le/discoveryindex/sources.go` (`IsSource`, `HeaderText`,
  `feeds`, `hasPackageHeader`, `generator`) went dead the moment `7629e9013`
  deleted their two callers. `./le repository check` cannot see it, because it
  reads only changed files and the orphaning commit is not the commit that shows
  the orphan. Deleted in `38c3c8882`.

### Documentation Updates
- `ai/INDEX.md` -- the three orientation indexes are DERIVED and untracked, and
  what each hook does. Closure corrected a sentence `38c3c8882` made false:
  it said a session start "rebuilds all three", when the hook builds only what
  the tree does not hold.
- `docs/contributing/navigating-the-code.md` -- the consumer contract: name the
  path so the read hook can see it, and the stated limit on writes no hook sees.
- `docs/contributing/committing.md` -- the paragraph naming `ai/PACKAGE-MAP.md`
  as the one tracked derivable file is gone, and the retired gate is named.
- `docs/architecture/core-design.md` -- the derived-artifact registry as a
  composition seam, with `<!-- source: internal/le/derived/derived.go -- Artifact,
  Register, All -->`. Closure tightened the Bash sentence, which claimed a rebuild
  on every naming command rather than on an absent artifact.
- `docs/architecture/testing/verify-freshness-scope.md` -- `discovery-index
  freshness` prints UNRUNNABLE, and why.
- `docs/contributing/go-conventions.md` -- the `ai/PACKAGE-MAP.md` source anchor
  says the map is derived on demand and untracked.
- `docs/functional-tests.md` -- the `runner` suite row names
  `le-derived-artifact-lifecycle` and the three facts it proves.
- `./le doc check verify` exit 1 over the whole tree. No failure names a file this
  spec touched: the failures are `../gh-pages/reference/command-equivalents/`
  rows for `request bgp rib mark-stale` and `purge-stale`, 8 YANG summary rules,
  and 4 unresolved anchors in `commands.md`, `exabgp-bridge.md` and
  `firewall-irr.md`. A grep of the log for `PACKAGE-MAP`, `core-design`,
  `navigating-the-code`, `ai/INDEX.md`, `le/derived`, `committing.md` and
  `go-conventions` answers nothing.

### Deviations from Plan
- The plan spelled three test names `Materialise`. The code spells them
  `Materialize`, because `ai/rules/writing.md` makes US English the project
  language.
- `internal/le/derived/derived_internal_test.go` was created and is not in Files
  to Create. `Register`'s refusals are unexported state, so they cannot be driven
  from the external test package.
- `docs/features/ai-first.md` was named in Files to Modify and needed no edit:
  `git show 7629e9013^:docs/features/ai-first.md` grepped for `stale-index`,
  `PACKAGE-MAP` and `discovery-index` answers nothing, so the page never carried
  the keyword.
- Documentation checklist rows 3 and 15 named `docs/guide/command-reference.md`
  and `docs/guide/status.md` conditionally. Neither names `discovery-index`, so
  neither was edited.
- The plan deleted the `debtGates` row for the retired gate. A-5 broke: the row
  is a DECLARATION the ledger needs, so only its `Runnable` flag changed.
- Six tests exist beyond the TDD plan, each covering a refusal the plan did not
  foresee: `TestRegisterRefusesAnUnusablePath`, `TestATrackedArtifactIsNeverRemoved`,
  `TestRemovingAPackageHeaderStillInvalidatesTheMap`,
  `TestTrackedFailsClosedWhenTheCheckoutCannotBeRead`,
  `TestAGrepOverTheDirectoryMaterializesTheArtifact`,
  `TestSessionStartLeavesAPresentArtifactAlone`.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-5 assumed the retired gate's `debtGates` row could be deleted with the gate | `debtGateAt` answers -1 for an undeclared gate name, `clearDebtWith` never clears an unrecognized row, and `TestEveryLedgerGateNameIsDeclared` refuses a ledger row no table declares, so deleting the row strands all 290 open rows | round 1 of the independent review, then confirmed at the producer | the declaration stays with `Runnable` false; the keyword and the gate are gone |
| approach | the session-start rebuild was written unconditional | the hook has 5 seconds and took 4.3, 4.8 and 5.9 seconds over three warm runs, most of it `commit.ListDebt` reading every verification-debt shard. A killed hook discards the whole session-start message | round 2 of the independent review, measured with `echo '{}' \| time ./le hook-check session-start` | absent-only rebuild, and the budget written into the `hookSessionStart` comment |
| escalation | a round diagnosed a guard by reading the PRODUCER of the data it reads rather than the function that turns that data into a verdict, and reported the spec-citation guard as failing open for a quarter of the tree | `AuditAnchors` (`internal/le/spec/citation/anchors.go`) builds the refusing half from each file's own `// Design:` header and never consults the index, so the guard had always fired, and 0 of 321 specs newly failed once the parser was widened | the sibling spec's author re-read the consumer | routed to `ai/rules/points/evidence/directives/five-biases-that-are-invisible-from-inside.md` and rendered with `./le rules render-update` |
| approach | a documentation page this spec's own round 2 made false was not repaired in that round | `ai/INDEX.md` said a session start "rebuilds all three" after `38c3c8882` made the rebuild absent-only | closure step 4, reading every page the three commits touched against the current producer | corrected in commit A, together with the same over-claim in `docs/architecture/core-design.md` |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| an artifact declares itself, its inputs and how to rebuild it | Done | `internal/le/derived/derived.go`: `Artifact`, `Register`, `All` | `Register` refuses a nil predicate, a nil rebuild, a duplicate path and an unusable path |
| a write to one of its inputs DELETES it | Done | `postInvalidateDerived` (`internal/le/hookruntime/postwrite.go`) | refuses to delete a path git tracks, and fails closed when the checkout cannot be read |
| a read of it MATERIALIZES it | Done | `preMaterializeDerived` (`internal/le/hookruntime/bash.go`) | matches the artifact path, or a directory holding it with a trailing slash |
| nothing compares a re-render against a committed copy | Done | `fullStages` (`internal/le/verify/engine/stages.go`), `generationChecks` (`internal/le/repository/generate.go`), `discoveryIndexesStage` (`internal/le/doc/wiring/docverify.go`), `checkSourceGates` (`internal/le/commit/prepare.go`) | `./le verify list mode full` is 49 stages, 0 of them naming discovery-index |
| the session-start hook stops naming artifacts | Done | `hookSessionStart` (`internal/le/hookruntime/lifecycle.go`) | the two hardcoded `os.Stat` blocks are one `for _, artifact := range derived.All()` |
| the RFC ledger family stays out of scope | Done | Known Limitations | owned by `spec-derived-indexes-answer-a-query` |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `git ls-files ai/PACKAGE-MAP.md` empty; `git check-ignore -v` names the `.gitignore` line holding `/ai/PACKAGE-MAP.md` | |
| AC-2 | Done | `TestACommitChangingAPackageCommentNeedsNoIndex` | PASS |
| AC-3 | Done | `TestStaleIndexOKIsNotAKeyword` | PASS; `parseCreate` refuses the keyword |
| AC-4 | Done | `TestEditingAnIndexFeedingSourceRemovesThePackageMap`, `TestRemovingAPackageHeaderStillInvalidatesTheMap` | PASS |
| AC-5 | Done | `TestEditingANonSourceLeavesEveryArtifactInPlace` | PASS |
| AC-6 | Done | `TestReadingAnAbsentArtifactMaterializesItBeforeTheCommandRuns`, `TestAGrepOverTheDirectoryMaterializesTheArtifact` | PASS |
| AC-7 | Done | `TestSessionStartBuildsEveryRegisteredDerivedArtifact` | PASS; the hook prints `Built %s (derived, not tracked)` per artifact |
| AC-8 | Done | `./le verify list mode full` over 49 stages, `grep -c discovery-index` answers 0 | |
| AC-9 | Done | `internal/le/docstocode/register.go` registers two artifacts in its own `init()`; no production file under `internal/le/hookruntime/` names an artifact path | the one hit is a comment recording the deleted enumeration |
| AC-10 | Done | `./le discovery-index update \| json` exit 0, answering `file` then `packages[].{path,registered,responsibility}` | shape unchanged, so the UI fixtures keep asserting on it |
| AC-11 | Done | `TestATrackedArtifactIsNeverRemoved`, `TestTrackedFailsClosedWhenTheCheckoutCannotBeRead` | PASS |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestRegisteredArtifactsCarryAPredicateAndARebuild` | Done | `internal/le/derived/derived_test.go` | PASS |
| `TestRegisterRefusesADuplicatePath` | Done | `internal/le/derived/derived_internal_test.go` | PASS; internal test package, because the refusal is over unexported state |
| `TestEditingAnIndexFeedingSourceRemovesThePackageMap` | Done | `internal/le/hookruntime/postwrite_test.go` | PASS |
| `TestEditingANonSourceLeavesEveryArtifactInPlace` | Done | `internal/le/hookruntime/postwrite_test.go` | PASS |
| `TestReadingAnAbsentArtifactMaterializesItBeforeTheCommandRuns` | Changed | `internal/le/hookruntime/bash_test.go` | US spelling, per `ai/rules/writing.md` |
| `TestMaterializingLeavesAPresentArtifactAlone` | Changed | `internal/le/hookruntime/bash_test.go` | US spelling |
| `TestSessionStartBuildsEveryRegisteredDerivedArtifact` | Done | `internal/le/hookruntime/lifecycle_test.go` | PASS |
| `TestACommitChangingAPackageCommentNeedsNoIndex` | Done | `internal/le/commit/commit_test.go` | PASS |
| `TestStaleIndexOKIsNotAKeyword` | Done | `internal/le/commit/commit_test.go` | PASS |
| `TestAGrepAfterAnEditReadsTheEditedPackage` | Done | `internal/le/derived/derived_test.go` | PASS |
| `le-derived-artifact-lifecycle` | Done | `test/runner/le-derived-artifact-lifecycle.ci` | PASS as case 3 of `./le functional runner`, 12/12 |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/le/derived/derived.go` | Done | created |
| `internal/le/derived/derived_test.go` | Done | created |
| `internal/le/derived/derived_internal_test.go` | Changed | created, not in the plan: `Register`'s refusals are unexported |
| `test/runner/le-derived-artifact-lifecycle.ci` | Done | created |
| the three `register.go` files, the three hook files, `commit/{prepare,debt,actions}.go`, `verify/engine/stages.go`, `doc/wiring/docverify.go`, `repository/generate.go`, `discoveryindex/{report,actions,sources}.go`, `docstocode/*`, the test files, `.gitignore` | Done | 65 files in `7629e9013`, 14 in `38c3c8882`, 4 in `940fb364f` |
| `docs/features/ai-first.md` | Changed | the page never carried `stale-index-ok`, so no edit was owed. Evidence in Deviations |
| `docs/guide/command-reference.md`, `docs/guide/status.md` | Changed | neither names `discovery-index`, so neither was edited |

### Audit Summary
- **Total items:** 28 (6 requirements, 11 ACs, 11 tests)
- **Done:** 26
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 2 test spellings, 1 extra test file, and 3 plan FILE rows whose page
  did not carry the text the plan predicted. Each is recorded in Deviations

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| the package map stops costing a commit gate, a verify stage, a doc-wiring stage and an override keyword | functional | `./le verify list mode full` is 49 stages and 0 name discovery-index; `TestACommitChangingAPackageCommentNeedsNoIndex` and `TestStaleIndexOKIsNotAKeyword` both PASS |
| a regeneration can no longer write another session's uncommitted packages into a tracked file | functional | `git ls-files ai/PACKAGE-MAP.md` is empty and `git check-ignore -v` names the `.gitignore` line, so no regeneration can reach the index at all |
| staleness stops being silent: an index that exists and no longer matches the tree is not read as current | functional | `le-derived-artifact-lifecycle` (`./le functional runner`, 12/12) drives the real `le hook-check` binary over a scratch checkout and asserts all four markers, `input-write-removed-the-artifact` and `read-materialized-the-artifact` among them. `TestAGrepAfterAnEditReadsTheEditedPackage` proves the same loop for the package map, over a temporary checkout |
| the hook stops being a central enumeration a third artifact must be added to | functional | no production file under `internal/le/hookruntime/` names an artifact path; `internal/le/docstocode/register.go` adds two artifacts with two `derived.Register` calls in its own `init()` |
| the artifact's absence is LOUD where its staleness was silent | functional | `hookSessionStart` prints `Built %s (derived, not tracked)` per artifact and a `Warning:` line naming the error when a build fails; `TestSessionStartBuildsEveryRegisteredDerivedArtifact` asserts the build |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| the RFC ledger family (`ai/RFC-REQUIREMENTS.md`, `rfc/requirements/*.md`, `rfc/enrolled.txt`, `rfc/not-enrolled.txt`) and `docs/features/rfc-status.md` stay tracked | their readers parse the rendered markdown as a data format, so they cannot be untracked until those readers query a structure instead | `plan/spec-derived-indexes-answer-a-query.md` |
| the `Read` and `Grep` TOOLS still meet an absent artifact, because no hook intercepts them | the hook surface `.claude/settings.json` exposes is `posttool-writeedit`, `pretool-bash` and `SessionStart`; there is no read-tool hook to register against | none: a STATED limitation, recorded in Known Limitations and in `docs/contributing/navigating-the-code.md`, not deferred work |
| a write that reaches an input with no hook in its path (`sed -i`, a heredoc, `git rebase`, `git stash pop`, `./le repository generate`) leaves the artifact present and stale | catching every write path has no bound, and the alternative, deleting at session start, leaves every artifact absent, so the first search that names no path reads nothing and says nothing | none: a STATED limitation, in Known Limitations, in `commandNamesArtifact` and in `docs/contributing/navigating-the-code.md` |
| `hookSessionStart` is over its 5 second budget WITHOUT the rebuild, because `commit.ListDebt` reads every verification-debt shard and that read grows with the ledger | separable from this spec's goal, and it is a defect walked into rather than one this work created | one row in `plan/journal/test-gate-repeats-expensive-work.md`, landed with `38c3c8882` |
| the weakened-row reader takes only the first table in a shard | the same: a defect met while landing this work, not one it created | one row in `plan/journal/gate-excludes-part-of-its-population.md`, landed with `38c3c8882` |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/le/derived/derived.go` | Yes | `ls -l` answers `5.6K Sep 11 17:32` |
| `internal/le/derived/derived_test.go` | Yes | `ls -l` answers `5.3K Sep 11 17:15` |
| `internal/le/derived/derived_internal_test.go` | Yes | `ls -l` answers `4.3K Sep 11 17:15` |
| `test/runner/le-derived-artifact-lifecycle.ci` | Yes | `ls -l` answers `1.4K Sep 11 16:27` |
| `ai/PACKAGE-MAP.md` | Yes, and untracked | `ls -l` answers `95K Sep 11 22:27`; `git ls-files` answers nothing |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | the map is untracked and ignored | `git ls-files ai/PACKAGE-MAP.md` -> empty; `git check-ignore -v ai/PACKAGE-MAP.md` -> the `.gitignore` line holding `/ai/PACKAGE-MAP.md` |
| AC-2 | a package-comment commit needs no index | `--- PASS: TestACommitChangingAPackageCommentNeedsNoIndex (0.20s)` |
| AC-3 | `stale-index-ok` is refused | `--- PASS: TestStaleIndexOKIsNotAKeyword (0.00s)` |
| AC-4 | an accepted write removes the map | `--- PASS: TestEditingAnIndexFeedingSourceRemovesThePackageMap (0.02s)`, `--- PASS: TestRemovingAPackageHeaderStillInvalidatesTheMap (0.03s)` |
| AC-5 | a rejected write leaves it alone | `--- PASS: TestEditingANonSourceLeavesEveryArtifactInPlace (0.02s)` |
| AC-6 | a naming command rebuilds an absent map | `--- PASS: TestReadingAnAbsentArtifactMaterializesItBeforeTheCommandRuns (0.04s)`, `--- PASS: TestAGrepOverTheDirectoryMaterializesTheArtifact (0.03s)` |
| AC-7 | a session start builds all three and says so | `--- PASS: TestSessionStartBuildsEveryRegisteredDerivedArtifact (0.05s)`; `--- PASS: TestSessionStartLeavesAPresentArtifactAlone (0.03s)` bounds it to ABSENT |
| AC-8 | no byte-compare stage remains | `./le verify list mode full` -> 49 lines, `grep -c discovery-index` -> 0 |
| AC-9 | a new artifact is one call in its own package | `internal/le/docstocode/register.go` holds two `derived.Register` calls; grepping `internal/le/hookruntime/*.go` for the three artifact names, tests excluded, answers one comment line and no code |
| AC-10 | the report shape is unchanged | `./le discovery-index update \| json` exit 0, first key `file`, then `packages[].{path,registered,responsibility}` |
| AC-11 | a tracked artifact is never removed | `--- PASS: TestATrackedArtifactIsNeverRemoved (0.06s)`, `--- PASS: TestTrackedFailsClosedWhenTheCheckoutCannotBeRead (0.00s)` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `./le hook-check posttool-writeedit` over an edited `register.go` | `test/runner/le-derived-artifact-lifecycle.ci`, marker `input-write-removed-the-artifact` | Yes: read the file, the fixture drives the REAL `le hook-check` binary over a scratch checkout. PASS as case 3 of `./le functional runner` |
| `./le hook-check pretool-bash` naming a registered path | the same `.ci`, marker `read-materialized-the-artifact` | Yes |
| `./le hook-check session-start` in a tree with no artifact | the same `.ci`, marker `session-start-rendered-the-artifact` | Yes |
| `./le commit create` over a package-comment change | no `.ci`: the entry point is the Go API `Prepare`, driven by `TestACommitChangingAPackageCommentNeedsNoIndex` | Yes |

The `.ci` rides `ai/CODE-TO-DOCS.md` rather than `ai/PACKAGE-MAP.md`, because the
`posttool-writeedit` chain also lints an edited Go file and would put
golangci-lint's verdict inside this loop's exit code. The package map's own loop
is proved by `TestAGrepAfterAnEditReadsTheEditedPackage`, over the same two hooks.

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `IsSourcePath` (`internal/le/discoveryindex/sources.go`) reads the same `outputs` and `inPopulation` the generator walks; `TestEditingAnIndexFeedingSourceRemovesThePackageMap` and `TestRemovingAPackageHeaderStillInvalidatesTheMap` both PASS |
| A-2 | confirmed | `TestReadingAnAbsentArtifactMaterializesItBeforeTheCommandRuns` PASS, and `le-derived-artifact-lifecycle` PASS through the real binary |
| A-3 | confirmed | grepping `.github/` for `PACKAGE-MAP` and `discovery-index` answers nothing |
| A-4 | confirmed | the thirteen `StaleIndexOK` assignments are gone across four files, and `go test ./internal/le/commit/` answers `ok ... 19.301s` |
| A-5 | BROKEN | the `debtGates` DECLARATION must stay: `debtGateAt` answers -1 for an undeclared gate, `clearDebtWith` never clears an unrecognized row, and `TestEveryLedgerGateNameIsDeclared` PASS refuses one. Only `Runnable` changed, to false. Mistake Log row 1, Deviations row 5 |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `ai/INDEX.md`: the three indexes are derived, untracked, and rebuilt only when ABSENT | `hookSessionStart` skips a path `os.Stat` finds (`internal/le/hookruntime/lifecycle.go`), and `preMaterializeDerived` (`internal/le/hookruntime/bash.go`) does the same | Yes, and CORRECTED at closure: the page claimed a session start "rebuilds all three" |
| `docs/architecture/core-design.md`: the derived-artifact seam | `internal/le/derived/derived.go`: `Artifact`, `Register`, `All`, named in the page's `<!-- source: -->` anchor | Yes, and CORRECTED at closure: the page claimed a naming Bash command rebuilds the artifact, when it builds only an absent one |
| `docs/contributing/navigating-the-code.md`: present-and-stale is the bound, and neither the read hook nor a session start rebuilds a present artifact | the page's own paragraph on unhooked writes, read against the two `os.Stat` guards | Yes, already correct after `38c3c8882` |
| `docs/contributing/committing.md`: `ai/PACKAGE-MAP.md` is no longer a tracked derivable file, and `discovery-index freshness` was retired | grepping `docs/` for `stale-index-ok` answers nothing; `debtGates` (`internal/le/commit/debt.go`) carries `{gateStaleIndexOK, "discovery-index freshness", false, nil}` | Yes |
| `docs/architecture/testing/verify-freshness-scope.md`: the gate prints UNRUNNABLE | the `Runnable` field of that `debtGates` row is false | Yes |
| `docs/contributing/go-conventions.md`: the anchor names the map as derived on demand and untracked | `checkAnchors` (`internal/le/docstocode/codetodocs.go`) skips a non-`.go` anchor path, so the anchor is safe whether the file exists or not | Yes |
| `docs/functional-tests.md`: the `runner` suite row names the new `.ci` and the three facts it proves | `test/runner/le-derived-artifact-lifecycle.ci` and its `expect=stdout:contains=` markers | Yes |
| CLI reference (`docs/guide/command-reference.md`), status (`docs/guide/status.md`), `docs/features/ai-first.md` | grepping all three for `discovery-index`, `stale-index` and `PACKAGE-MAP` answers nothing, and the same grep over `docs/features/ai-first.md` at `7629e9013^` answered nothing either | Yes, no update owed |
| RFC status, YANG, wire format, plugin SDK, comparison table, route metadata, Prometheus | Scope is `tooling`: no protocol, no config, no RPC, no plugin, no daemon-observable state | Yes, N-A |

## Core Insight

A gate that compares a derived file against a re-render of the tree is running
the generator to decide a verdict, and then throwing the answer away. The walk
that decides the verdict is the walk that could have written the file. Once the
artifact is derived on demand there is no second copy to disagree with anything,
so the gate, the override keyword, the debt row and the verify stage all go
together, and the failure mode they existed to catch cannot occur.

What the move costs is not staleness but REACHABILITY: the tree can only
invalidate what it can see a write to. That is why the answer is a REGISTRY of
predicates rather than a list of paths, and why the hook that DELETES must
refuse a path git still tracks. A mechanism that deletes cannot trust a
registration to have finished the migration that makes the deletion safe.
