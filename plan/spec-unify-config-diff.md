# Spec: unify-config-diff

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-07-06 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/rules/module-tiers.md` - dependency direction decides package placement
4. `internal/component/plugin/server/reload.go` (the duplicate), `internal/component/config/diff.go` (the canonical)

## Task

DESIGN-REVIEW.md finding 2 (row "Config diff") flags that the same deep map-diff
algorithm is implemented twice, with the second copy justified by a comment claiming
an import cycle:

- `internal/component/config/diff.go:27` `DiffMaps` returns `*config.ConfigDiff` -- the
  canonical implementation, covered by `internal/component/config/diff_test.go` (12 unit
  tests) and consumed by the `ze config diff` CLI at `internal/component/config/cli/cmd_diff.go:91`.
- `internal/component/plugin/server/reload.go:313` `diffMaps` returns `*configDiff` -- a
  private copy in package `server`. The comment at `reload.go:296-297` reads "Local to the
  plugin package to avoid import cycles with internal/config" and at `reload.go:312`
  "Equivalent to config.DiffMaps -- duplicated here to avoid import cycle."

The two bodies are structurally identical (nil-fill both maps, collect removed keys, then
walk new keys emitting added / recursing on nested maps / `reflect.DeepEqual` on leaves,
joining paths with `config.PathSep`). They differ only in identifier visibility
(`config.ConfigDiff{Added,Removed,Changed}` + `config.DiffPair` versus the private
`configDiff{added,removed,changed}` + `diffPair`).

Goal: collapse the two into one implementation, preserving all externally observable
behavior (JSON payloads sent to plugins, CLI diff output). This spec establishes that the
cited import cycle does not exist today and plans the migration of package `server` onto
`config.DiffMaps`, deleting the duplicate.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
- [ ] `ai/rules/module-tiers.md` - decides whether the shared diff stays in `config` or moves to a `internal/core/` leaf.
  → Decision: `internal/component/plugin/server` already imports `internal/component/config` (reload.go:16, uses `config.AppendPath`, `config.PathSep`, `config.ExtractConfigSubtree`), so `plugin/server -> config` is an established, tier-legal edge. Calling `config.DiffMaps` adds no new package dependency and cannot change the `dep_audit.py` graph.
  → Constraint: a new package only earns its place when it reduces coupling; moving diff to `internal/core/` would not, because `plugin/server` keeps importing `config` regardless.
- [ ] `ai/rules/no-fabrication.md` - claims about the cycle must cite the producing code.
  → Constraint: the "no cycle" claim is proven by an existing compiling import (reload.go:16) plus a negative grep, not by narrative.
- [ ] `DESIGN-REVIEW.md` finding 2 - source of this task.
  → Decision: the finding's premise ("to avoid an import cycle") is stale; the fix is to delete the duplicate, not to relocate anything.
- [ ] `ai/rules/no-partial-completion.md` - refactor is done only when the duplicate is deleted and every caller compiles against the shared type.
  → Constraint: leaving both implementations "wired but one unused" is not done.

**Key insights:**
- The duplication predates the current package layout: the stale comment says "internal/config", but the canonical package is `internal/component/config`. The cycle (if it ever existed) was dissolved when `server` started importing `config` for path helpers.
- `config.DiffMaps` is the better-tested, already-shared implementation. Package `server` should consume it; `config` should not consume anything from `server`.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
<!-- Same rule: never tick [ ] to [x]. Write → Constraint: annotations instead. -->
- [ ] `internal/component/config/diff.go` - canonical `DiffMaps(old, new) *ConfigDiff` with exported `ConfigDiff{Added,Removed,Changed map[string]...}` and `DiffPair{Old,New any}` (json tags `old`/`new`). Recurses via `diffMapsRecursive`, joins paths with `AppendPath`, compares leaves with `reflect.DeepEqual`.
  → Constraint: JSON tags `old`/`new` on `DiffPair` are load-bearing (they flow into plugin apply payloads).
- [ ] `internal/component/config/diff_test.go` - 12 unit tests (`TestDiffMapsEmpty/Added/Removed/Changed/Nested/NestedAdd/NestedRemove/NilOld/NilNew/BothNil/TypeChange/SliceChange`) that pin the canonical behavior.
- [ ] `internal/component/config/path.go` - `AppendPath`, `PathSep` ("/"); both `DiffMaps` and the `server` copy already join paths through this.
- [ ] `internal/component/config/cli/cmd_diff.go` - second consumer of `config.DiffMaps` (`ze config diff`), proving the canonical type is a shared public API.
- [ ] `internal/component/plugin/server/reload.go` - the duplicate `diffMaps`/`configDiff`/`diffPair`/`diffMapsRecursive`/`diffJoinPath` (lines 296-365), plus consumers `rootHasChanges(diff *configDiff)` (369), `buildDiffSections(diff *configDiff)` (402), and the `reloadConfig` body reading `diff.added/removed/changed` (178-205). Already imports `config` at line 16.
  → Constraint: `buildDiffSections` marshals `diff.changed` values (currently `diffPair`) into `rpc.ConfigDiffSection.Changed`; the JSON must stay byte-identical after the switch.
- [ ] `internal/component/plugin/server/reload_tx.go` - `runTxCoordinator(..., diff *configDiff, ...)` (39) and `buildTxInputs(affected, diff *configDiff)` (261) take the private type and forward it into `buildDiffSections`.
- [ ] `internal/component/plugin/server/reload_test.go` - `TestDiffMapsLocal` (839) tests the private `diffMaps`; `TestRootHasChanges` (874), `TestDiffPairJSONKeys` (967), and `TestBuildDiffSections` (985) construct `configDiff`/`diffPair` literals; integration tests `TestReloadConfigVerifyThenApply` (601), `TestReloadConfigPerRootFiltering` (633), `TestReloadConfigRootRemoved` (894), `TestReloadConfigWildcardRoot` (932), `TestReloadTxVerifyReceivesFullSubtree` (1217) drive `reloadConfig` end-to-end.

**Behavior to preserve:**
- The diff shape: added / removed / changed key sets keyed by slash-joined config paths, identical to what `config.DiffMaps` produces (the two algorithms are already structurally identical).
- The JSON emitted to plugins via `rpc.ConfigDiffSection` (`Added`/`Removed`/`Changed` strings), including the `old`/`new` keys inside changed pairs. `DiffPair` and the private `diffPair` share identical json tags, so output stays byte-identical.
- `TestDiffPairJSONKeys`' guarantee that changed values marshal with `old`/`new` keys, and `reload_test.go:1249-1250`' guarantee that plugins do not see raw `old`/`new` at the wrong level.
- `ze config diff` output (unchanged; `config.DiffMaps` is untouched).
- `rootHasChanges` root-prefix matching (including the `*` wildcard) and `buildDiffSections` top-level-root grouping.

**Behavior to change:**
- None - internal refactor, behavior preserved. Only the private duplicate is removed; package `server` computes the same diff via `config.DiffMaps`.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- A config reload is triggered by SIGHUP, a CLI/API commit, or a hub full-reload, landing in `Server.reloadConfig` (`internal/component/plugin/server/reload.go:161`) with the candidate `map[string]any` tree.
- The running tree is fetched from `s.reactor.GetConfigTree()`; both trees are plain `map[string]any` (parsed YANG config).

### Transformation Path
1. `reloadConfig` computes the diff. Today: `diffMaps(running, newTree)` (reload.go:178). After: `config.DiffMaps(running, newTree)` returning `*config.ConfigDiff`.
2. The diff drives three decisions in `reloadConfig`: removed-key collection for deferred stop (192), added-key auto-load (203), and per-plugin section building (215-250) gated by `rootHasChanges(diff, root)`.
3. `runTxCoordinator(ctx, affected, diff, running, newTree)` (reload.go:271, defined reload_tx.go:39) forwards the diff into `buildTxInputs` (reload_tx.go:261), which calls `buildDiffSections(diff)` (reload.go:402) to produce `[]rpc.ConfigDiffSection` grouped by top-level root.
4. `rpc.ConfigDiffSection` JSON crosses the plugin IPC boundary to each affected plugin's `OnConfigVerify` / `OnConfigApply` handler.
5. On success `s.reactor.SetConfigTree(newTree)` commits the running view.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| server ↔ config package | `config.DiffMaps` / `config.ConfigDiff` (already imports `config` at reload.go:16) | [ ] |
| Engine ↔ Plugin | `rpc.ConfigDiffSection` JSON (`Added`/`Removed`/`Changed` strings) over plugin IPC | [ ] |
| config ↔ path helpers | `config.AppendPath` / `config.PathSep` join diff key paths | [ ] |

### Integration Points
- `Server.reloadConfig` (reload.go:161) - sole producer of the diff inside package `server`.
- `rootHasChanges` (reload.go:369) and `buildDiffSections` (reload.go:402) - consume the diff; signatures change from `*configDiff` to `*config.ConfigDiff`.
- `runTxCoordinator` / `buildTxInputs` (reload_tx.go:39,261) - forward the diff type.
- `config.DiffMaps` (diff.go:27) - the shared producer; also used by `internal/component/config/cli/cmd_diff.go:91` (`ze config diff`), which is unaffected.

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (`config` does not import `server`; edge stays one-directional `server -> config`)
- [ ] No duplicated functionality (the private `diffMaps` is deleted; one implementation remains)
- [ ] Zero-copy preserved where applicable (diff maps reference the same values; no extra copies introduced)
- [ ] Registration over hardcoding — this refactor adds no new command, view, family, or handler; it removes a hand-rolled duplicate and routes package `server` through the existing `config.DiffMaps` API. No new per-feature field, switch case, or factory is added to any core/shared package (small-core/registration; `ai/rules/plugin-self-containment.md`).

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | No import cycle blocks `server` from calling `config.DiffMaps`. | `reload.go:16` already imports `internal/component/config` and the repo compiles; grep shows `config` (and the `internal/component/plugin`, `internal/component/plugin/registry` packages it imports) never import `internal/component/plugin/server`. | Would need option (a): move diff to a `internal/core/` leaf. | `go build ./internal/component/plugin/server/...` after switching the call, plus `grep -rl plugin/server internal/component/config` returns nothing. | unvalidated |
| A-2 | `config.DiffMaps` produces a byte-identical diff and JSON to the private `diffMaps`. | Both bodies read identically (nil-fill, removed, added/recurse/`reflect.DeepEqual`); `DiffPair` and `diffPair` share json tags `old`/`new`. | Plugin apply payloads or `ze config diff` output would drift. | `TestDiffPairJSONKeys`, `TestBuildDiffSections`, and the `TestReloadConfig*` integration tests pass unchanged after migration. | unvalidated |
| A-3 | `ze-tier-check` / `dep_audit.py` stay green. | The `server -> config` edge already exists; a method call adds no graph edge. | Would require category reclassification. | `make ze-tier-check`. | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Field-visibility rename missed at one call site (added→Added etc.) leaves a compile error or, worse, a shadowed local. | `go build ./...` fails, or a `_test.go` still constructs `configDiff{}`. | Migrate every site listed in Files to Modify in one change; rely on the compiler to catch the rename; delete the private types so no stale reference can compile. |
| R-2 | `TestDiffMapsLocal` deleted without equivalent coverage. | Coverage report drops on the diff algorithm. | Coverage is already provided by `config/diff_test.go` (12 tests) on the surviving implementation; delete the redundant local test rather than duplicate it. |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| SIGHUP / commit triggers `reloadConfig` with a changed tree | → | `config.DiffMaps` (replacing local `diffMaps`) feeds `buildDiffSections` | `TestReloadConfigVerifyThenApply` (`internal/component/plugin/server/reload_test.go`) |
| Per-root change filtering | → | `rootHasChanges(*config.ConfigDiff)` | `TestReloadConfigPerRootFiltering`, `TestReloadConfigWildcardRoot` |
| Changed value crosses to plugin as JSON | → | `buildDiffSections(*config.ConfigDiff)` marshals `DiffPair` with `old`/`new` | `TestDiffPairJSONKeys`, `TestBuildDiffSections` |
| SIGHUP reload re-applies traffic-control block end-to-end | → | full `reloadConfig` path over plugin IPC | `test/traffic/002-reload-apply.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Same running vs candidate trees fed to old private `diffMaps` and to `config.DiffMaps`. | Identical added/removed/changed key sets and values (behavior parity, proven by the config-package tests plus surviving `server` tests). |
| AC-2 | A reload changes a nested leaf under one root. | The plugin receives an identical `rpc.ConfigDiffSection` JSON (changed value with `old`/`new` keys) as before the refactor. |
| AC-3 | Repo grep for `diffMaps`, `configDiff`, `diffPair`, `diffMapsRecursive`, `diffJoinPath` in package `server`. | Zero matches: the duplicate is fully deleted, only `config.DiffMaps`/`config.ConfigDiff`/`config.DiffPair` remain. |
| AC-4 | `make ze-tier-check`. | Passes: no new package edge introduced (`server -> config` already existed). |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDiffMapsEmpty/Added/Removed/Changed/Nested/...` | `internal/component/config/diff_test.go` | Canonical `config.DiffMaps` behavior (unchanged; now the only implementation) | existing |
| `TestRootHasChanges` | `internal/component/plugin/server/reload_test.go` | Root-prefix matching after switching literal to `config.ConfigDiff` | migrate |
| `TestDiffPairJSONKeys` | `internal/component/plugin/server/reload_test.go` | Changed values marshal with `old`/`new` after switching to `config.DiffPair` | migrate |
| `TestBuildDiffSections` | `internal/component/plugin/server/reload_test.go` | Flat keys grouped by top-level root, using `config.ConfigDiff` literal | migrate |
| `TestDiffMapsLocal` | `internal/component/plugin/server/reload_test.go` | (removed - superseded by `config/diff_test.go`) | delete |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `002-reload-apply` | `test/traffic/002-reload-apply.ci` | SIGHUP reload mutates a config block; plugin re-verifies/applies from the diff | existing -- must pass unchanged |
| `web-commit-transactional` | `test/ui/web-commit-transactional.ci` | Web commit drives the transactional reload path (diff -> verify -> apply) | existing -- must pass unchanged |
| `config-push-transactional` | `test/managed/config-push-transactional.ci` | Managed config push through the same reload/diff path | existing -- must pass unchanged |

No user-facing behavior change; existing test suite passes with no regressions.

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files -->
- `internal/component/plugin/server/reload.go` - delete `configDiff`, `diffPair`, `diffMaps`, `diffMapsRecursive`, `diffJoinPath` (lines 296-365); change `diffMaps(...)` to `config.DiffMaps(...)`; rename field reads `diff.added/removed/changed` to `diff.Added/Removed/Changed`; change `rootHasChanges` and `buildDiffSections` parameter type to `*config.ConfigDiff`.
- `internal/component/plugin/server/reload_tx.go` - change `runTxCoordinator` and `buildTxInputs` parameter type from `*configDiff` to `*config.ConfigDiff`.
- `internal/component/plugin/server/reload_test.go` - update `TestRootHasChanges`, `TestDiffPairJSONKeys`, `TestBuildDiffSections` to construct `config.ConfigDiff`/`config.DiffPair`; delete `TestDiffMapsLocal`.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | [ ] No - pure internal refactor | - |
| CLI commands/flags | [ ] No | - |
| Functional test for new RPC/API | [ ] No new RPC; existing `.ci` reload tests cover the path | `test/traffic/002-reload-apply.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 12 | Internal architecture changed? | [ ] Maybe - if any doc names the private `diffMaps` as intentional | grep `docs/` for `diffMaps` / "import cycle"; update if a stale claim exists |
| 16 | Any changed source file referenced by doc source anchors? | [ ] Check | Grep `docs/` for `source: internal/component/plugin/server/reload.go` and `.../config/diff.go` |

## Files to Create
- None - this is a deletion/consolidation refactor; no new files.

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist below |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring / characterization (MANDATORY FIRST)** — confirm the current path is captured by tests before touching it.
   - Tests: run `TestReloadConfigVerifyThenApply`, `TestReloadConfigPerRootFiltering`, `TestDiffPairJSONKeys`, `TestBuildDiffSections`, and `test/traffic/002-reload-apply.ci` on the untouched tree; record them GREEN as the behavior baseline.
   - Files: none changed yet.
   - Verify: baseline captured; A-1 validated by `grep -rl plugin/server internal/component/config` returning nothing and `go build ./internal/component/plugin/server/...` succeeding.
2. **Phase: Switch the producer** — replace `diffMaps` with `config.DiffMaps` and rename field reads in `reloadConfig`.
   - Tests: `TestReloadConfigVerifyThenApply`, `TestReloadConfigRootRemoved`, `TestReloadConfigWildcardRoot`.
   - Files: `reload.go` (call site + `diff.Added/Removed/Changed` reads).
   - Verify: package compiles with the private `diffMaps` still present but now unused; tests still green.
3. **Phase: Migrate consumers and delete the duplicate** — retype `rootHasChanges`, `buildDiffSections`, `runTxCoordinator`, `buildTxInputs` to `*config.ConfigDiff`; delete `configDiff`, `diffPair`, `diffMaps`, `diffMapsRecursive`, `diffJoinPath`; update the three literal-constructing tests; delete `TestDiffMapsLocal`.
   - Tests: `TestRootHasChanges`, `TestBuildDiffSections`, `TestDiffPairJSONKeys`, full `TestReloadConfig*` suite.
   - Files: `reload.go`, `reload_tx.go`, `reload_test.go`.
   - Verify: AC-3 grep returns zero; `make ze-tier-check` green (AC-4).
4. **Functional tests** → run the three `.ci` reload tests; confirm no regression.
5. **Full verification** → `make ze-verify` (lint + all ze tests except fuzz).
6. **Complete spec** → fill audit tables, write learned summary, two-commit closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1..AC-4 each demonstrated (parity tests, JSON test, deletion grep, tier-check). |
| Correctness | JSON emitted to plugins is byte-identical (`DiffPair` json tags `old`/`new` match the deleted `diffPair`). |
| Data flow | Diff produced only via `config.DiffMaps`; `config` still does not import `server`. |
| Rule: no-layering | Old `diffMaps` and its private types fully deleted, not left dead. |
| Registration over hardcoding | No new command/view/family/handler; no new per-feature field or switch added to a core/shared struct. The change removes a duplicate and routes through the existing `config.DiffMaps` API (`ai/rules/plugin-self-containment.md`). |
| Rule: module-tiers | No new package edge; `server -> config` pre-existed, so `dep_audit.py` graph is unchanged. |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error (missed rename) | Fix in Phase 3; rely on compiler after deleting private types |
| Test fails behavior mismatch | Re-read `config/diff.go` vs deleted body; the two must be identical |
| `ze-tier-check` fails | Re-examine assumption A-1/A-3; fall back to `internal/core/` leaf option |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

- The "import cycle" justification is a fossil: the comment references `internal/config`, a path that no longer exists, and package `server` already imports the current `internal/component/config` for path helpers. Stale defensive comments can freeze a duplication in place long after the constraint that motivated it is gone. Verifying the constraint (one grep + the fact that the file already compiles with the import) is cheaper than trusting the comment.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Winner: `config.DiffMaps`. Package `server` calls it directly and deletes its private `diffMaps`. | (a) Move `DiffMaps` + result types into a new `internal/core/` leaf package that both import. | `server` already depends on `config` (reload.go:16); calling `config.DiffMaps` adds zero new package edge. `config.DiffMaps` is the better-tested (12 tests) and already-shared (`ze config diff`) implementation. Option (a) would create a new package and force `config` + its CLI consumer to migrate, all for no coupling reduction, violating "no premature abstraction". |
| Override the kit's suggested option (a) leaf-package move. | Keep the kit default. | The premise for (a) is a real cycle; there is none. With no cycle, the minimal, honest fix is direct reuse of the existing public API. |
| Keep `config.DiffMaps` where it lives (in `config`), not relocate. | Relocate to `internal/core/`. | `config` is the natural owner (it parses the trees being diffed) and is the tier that both consumers already sit above or beside; relocation would be churn without benefit per `ai/rules/module-tiers.md`. |
| Delete `TestDiffMapsLocal` rather than repoint it. | Repoint it at `config.DiffMaps`. | It would exactly duplicate `config/diff_test.go`; the surviving implementation is already covered there. |

## Known Limitations
- The refactor is confined to package `server` and its tests. `config.DiffMaps` itself is untouched, so `ze config diff` and all `config` consumers are unaffected by design.
- If a future change makes `config` depend (transitively) on `server`, the direct call would break; the tier-check (`dep_audit.py`) guards against that regression.

## RFC Documentation

Not applicable - no protocol or wire behavior changes.

## Implementation Summary

### What Was Implemented
-

### Bugs Found/Fixed
-

### Documentation Updates
-

### Deviations from Plan
-

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| One diff implementation, duplicate deleted | functional test + grep | `test/traffic/002-reload-apply.ci` green; AC-3 grep returns zero matches in package `server` |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE |  | file:line |  |

### Fixes applied
-

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-4 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/component/plugin/server/reload.go`, `reload_tx.go`)
- [ ] Duplicate deleted: grep for `diffMaps`/`configDiff`/`diffPair` in package `server` returns zero
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (reuse existing `config.DiffMaps`, add no new package)
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling (no new package edge)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior (existing `.ci` reload tests)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all checks documented pass in spec
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-unify-config-diff.md`
- [ ] **Commit A:** code + tests + spec + learned summary
- [ ] **Commit B:** `git rm plan/spec-unify-config-diff.md`
