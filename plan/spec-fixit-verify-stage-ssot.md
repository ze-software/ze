# Spec: fixit-verify-stage-ssot

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | - |
| Phase | - |
| Updated | 2026-07-17 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file.
2. `scripts/status/verify_run.go` `stagesForMode` (:112-158) - the LIVE stage list.
3. `Makefile` :279-298 (`ze-verify` invocation + the two DEAD `_impl` targets) and :105-106, :427.
4. `scripts/status/verify_run_test.go` `TestStagesForModeIncludesStaticAnalysisGates` (:345-363).

## Task

**[HIGH]** The `make ze-verify` / `ze-verify-changed` stage list lives in 3-4 divergent copies and has
already drifted. The LIVE list is `stagesForMode` (`scripts/status/verify_run.go:112-158`), invoked by
`ze-verify` (`Makefile:279-280`) via `verify_run.go`. A DEAD `_ze-verify-impl` / `_ze-verify-changed-impl`
pair (`Makefile:290,297`) has zero callers and its own comment (`Makefile:282-289`) admits it "silently
drifted out of sync for an unknown period." The guard `TestStagesForModeIncludesStaticAnalysisGates`
(`verify_run_test.go:345-363`) is a positive allowlist of 4, not a set-equality check, so it cannot catch a
dropped stage or a divergence between the two mode branches. Separately: `ze-yang-glue-check`
(`Makefile:105-106`, `yang_glue.go --check` writes `register.go`) is defined-but-UNWIRED (no caller, no
feeder), so a `.yang` added without `make generate` leaves a stale `register.go` and the module is silently
never wired (contrast `plugin_imports.go --check`, exercised by `all_test.go:170-182`). `ze-regen-check`
(`Makefile:427`) also runs nowhere, and `commit_helper.py:499` STRUCTURAL_GATES carries a dead
`ze-cli-grammar-check` entry that can never match a `stagesForMode` stage name.

Make the verify stage list a single source of truth (SSOT): delete the dead `_impl` targets, make the guard
assert SET-EQUALITY against a committed golden list (covering both mode branches); add a `TestYANGGlueCurrent`
feeder mirroring the plugin-imports feeder so `ze-yang-glue-check` rides the already-staged unit stage; add
`ze-regen-check` to `stagesForMode`; drop the dead `commit_helper.py` frozenset entry.

## Required Reading

- [ ] `scripts/status/verify_run.go` `stagesForMode` (:112-158), `defaultVerifyConfig` (:96-110)
  → Constraint: this function is the ONLY live stage list; both mode branches must stay in sync.
  → Decision: ~~add `ze-regen-check` here (both branches)~~ → add the write-safe read-only regen stage (`ze-regen-check-readonly`, R-1 2026-07-17) here (both branches), NOT the mutating `ze-regen-check`; the guard's set-equality golden is derived from this function, never hand-maintained in parallel (`ai/rules/derive-not-hardcode.md`).
- [ ] `Makefile` :279-298 (invocation + dead `_impl`), :105-106 (`ze-yang-glue-check`), :427 (`ze-regen-check`)
  → Constraint: `ze-verify`/`ze-verify-changed` shell out to `verify_run.go`; the `_impl` targets are unreachable and must be deleted, not repaired.
- [ ] `scripts/status/verify_run_test.go` `TestStagesForModeIncludesStaticAnalysisGates` (:345-363)
  → Constraint: today it is a subset check of 4 names; strengthen to SET-EQUALITY against a committed golden for both modes.
- [ ] `internal/component/plugin/all/all_test.go` `TestGeneratedPluginImportsCurrent` (:170-182)
  → Decision: the yang-glue feeder mirrors this exactly (`go run .../yang_glue.go --check`, fail on non-zero), placed in a package the unit stage already runs.
- [ ] `scripts/dev/commit_helper.py` STRUCTURAL_GATES (:492-503)
  → Constraint: entries are matched against `tmp/ze-verify-failures.json` stage names; a name absent from `stagesForMode` (e.g. `ze-cli-grammar-check`) is dead and must be removed.
- [ ] `scripts/codegen/yang_glue.go` (`--check` at :26, writes `register.go` at :63) - what the unwired check verifies.
- [ ] `.woodpecker/verify.yml:19` - CI runs only `make ze-verify`, so a stage absent from `stagesForMode` never runs in CI.

## Current Behavior

**Source files read:**
- [ ] `scripts/status/verify_run.go` - `stagesForMode(mode, makeCmd)` (:112-158) returns the ordered stage list; `ze-verify-changed` branch (:121-137) and default `ze-verify` branch (:138-157) are hand-duplicated.
  → Constraint: neither branch lists `ze-yang-glue-check`, `ze-regen-check`, nor `ze-cli-grammar-check`.
- [ ] `Makefile` - `ze-verify` (:279), `ze-verify-changed` (:293) call `verify_run.go`; `_ze-verify-impl` (:290) and `_ze-verify-changed-impl` (:297) are dead duplicate copies; `ze-yang-glue-check` (:105), `ze-regen-check` (:427).
  → Constraint: the dead `_impl` copies still list `ze-cli-grammar-check`, matching the stale frozenset entry.
- [ ] `scripts/status/verify_run_test.go` - `TestStagesForModeIncludesStaticAnalysisGates` (:345) asserts 4 required names are present (subset), for both modes.
- [ ] `scripts/dev/commit_helper.py` - STRUCTURAL_GATES frozenset (:492) includes `ze-cli-grammar-check` (:499), a name never present in `stagesForMode`.
- [ ] `internal/component/plugin/all/all_test.go` - `TestGeneratedPluginImportsCurrent` (:170) runs `go run ... plugin_imports.go --check` and fails on non-zero; the model feeder.
- [ ] `scripts/codegen/yang_glue.go` - `--check` mode (:26) verifies every `yang/` `register.go` is current without writing.

**Behavior to preserve:**
- `make ze-verify` and `ze-verify-changed` run all currently-live stages, in order, for both modes.
- The 4 static-analysis gates the guard already requires stay present in both modes.
- CI (`.woodpecker/verify.yml`) continues to run `make ze-verify` unchanged; the failure-routing artifacts (`tmp/ze-verify-failures.json`) keep their shape so `commit_helper.py` structural-gate reading still works.

**Behavior to change:**
- Delete the two dead `_impl` targets; add `ze-regen-check` to `stagesForMode` (both branches); strengthen the guard to set-equality; add the yang-glue feeder; remove the dead `ze-cli-grammar-check` frozenset entry.

## Data Flow

### Entry Point [real]
`make ze-verify` (`Makefile:279`) -> `scripts/dev/verify-lock.sh ze-verify env ... go run ./scripts/status/verify_run.go ze-verify` -> `main()` (`verify_run.go:80`) -> `defaultVerifyConfig` (:96) -> `stagesForMode(mode, makeCmd)` (:104,:112). CI reaches the identical entry point via `.woodpecker/verify.yml:19`.

### Transformation Path
1. `main` picks `mode` from `os.Args[1]` (default `ze-verify`).
2. `defaultVerifyConfig` calls `stagesForMode(mode, makeCmd)` to build the ordered `[]stage`.
3. `runVerify` (:160) executes each stage via `execStage`, writing `tmp/ze-verify-failures.json` (`stageResult.Stage` = stage name).
4. `commit_helper.py structural_gate_reds` (:506) reads that JSON and matches `st.stage` against STRUCTURAL_GATES.
5. The guard test recomputes `stagesForMode` and (new) compares it for set-equality against a committed golden per mode.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Make target ↔ verify runner | `ze-verify` shells out to `go run verify_run.go` | [ ] |
| Runner ↔ commit gate | `tmp/ze-verify-failures.json` stage names read by `commit_helper.py` | [ ] |
| Stage list ↔ guard | `stagesForMode` compared set-equal to committed golden (both modes) | [ ] |
| yang codegen ↔ unit stage | new `TestYANGGlueCurrent` runs `yang_glue.go --check` in an already-staged package | [ ] |

### Integration Points
- `scripts/status/verify_run.go` `stagesForMode` (the SSOT), `verify_run_test.go` (set-equality guard + golden), `Makefile` (delete dead `_impl`), `scripts/dev/commit_helper.py` STRUCTURAL_GATES, and the new yang-glue feeder test.
- Registration over hardcoding: the stage list becomes ONE committed/registered source guarded by set-equality, so a gate can no longer be added to a dead copy (or a single mode branch) and silently skipped.

### Architectural Verification
- [ ] No bypassed layers (guard drives `stagesForMode` directly; feeder drives the real `yang_glue.go --check`).
- [ ] No duplicated functionality (the dead `_impl` duplicate copies are deleted, not maintained).
- [ ] Registration over hardcoding respected (single guarded stage source; golden derived from `stagesForMode`).

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | Nothing calls the `_impl` targets | grep across Makefile/mk/scripts/.woodpecker shows only the definitions (`Makefile:290,297`) | deletion breaks a caller | re-grep before deleting | unvalidated |
| A-2 | `ze-cli-grammar-check` never appears as a `stagesForMode` stage name | `stagesForMode` (:112-158) omits it; STRUCTURAL_GATES matches stage names only | removing the frozenset entry weakens a live gate | confirm name absent from both branches | confirmed by read |
| A-3 | The yang-glue feeder placed in an already-staged package runs under `ze-unit-test*` | plugin-imports feeder lives in `internal/component/plugin/all` and runs there | feeder never runs; check stays unwired | place beside `TestGeneratedPluginImportsCurrent` and confirm it runs in the unit stage | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-1 | `ze-regen-check` mutates the tree (`ze-regen` runs `generate` + index regen) during verify | verify leaves a dirty working tree | run `--check`-style targets only, or confirm `ze-regen-check` fails-without-writing; if it writes, gate on a clean-tree assertion instead (open question) |
| R-2 | Set-equality golden becomes churny | frequent golden edits | derive golden from `stagesForMode` in-test (single const list) rather than a second hand-kept file |

→ R-1 RESOLVED (AUTONOMOUS DEFAULT 2026-07-17): the mutating `ze-regen-check` (`Makefile:430` → `ze-regen`, `Makefile:427`) is NOT wired into verify; a write-safe read-only `--check` stage (`ze-regen-check-readonly`) is wired instead. Full decision + coverage obligation recorded under the "Open question" resolution in Notes. Thomas: override for the clean-tree-assertion alternative.

## Wiring Test

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| guard recomputes stage list | -> | `stagesForMode` set-equals committed golden (both modes) | `TestStagesForModeMatchesGolden` |
| unit stage runs | -> | `yang_glue.go --check` passes (no stale `register.go`) | `TestYANGGlueCurrent` |
| `make ze-verify` runs | -> | ~~`ze-regen-check`~~ write-safe read-only regen stage (`ze-regen-check-readonly`; R-1) present in `stagesForMode` | `TestStagesForModeIncludesRegenCheck` |
| commit structural-gate read | -> | STRUCTURAL_GATES ⊆ live stage names | `test_structural_gates_are_live_stages` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Dead `_ze-verify-impl` / `_ze-verify-changed-impl` targets | deleted from `Makefile`; grep finds no `_impl` verify targets |
| AC-2 | Guard test for both modes | asserts SET-EQUALITY of `stagesForMode` against a committed golden; fails if any stage added/removed or the two branches diverge from their goldens |
| AC-3 | A `.yang` edited without `make generate` | `TestYANGGlueCurrent` fails under the unit stage (via `yang_glue.go --check`) |
| AC-4 | `stagesForMode` for both modes | ~~includes `ze-regen-check`~~ → includes the write-safe read-only regen stage (`ze-regen-check-readonly`, NOT the mutating `ze-regen-check`; R-1 resolution 2026-07-17) in BOTH branches |
| AC-5 | STRUCTURAL_GATES | contains only names present in `stagesForMode`; `ze-cli-grammar-check` removed |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestStagesForModeMatchesGolden` | `scripts/status/verify_run_test.go` | AC-2 | |
| `TestStagesForModeIncludesRegenCheck` | `scripts/status/verify_run_test.go` | AC-4 | |
| `TestYANGGlueCurrent` | `internal/component/plugin/all/yang_glue_check_test.go` | AC-3 | |
| `test_structural_gates_are_live_stages` | `scripts/dev/test_commit_helper.py` (or existing gate test) | AC-5 | |

### Functional Tests
Test infrastructure only; no user-facing feature. The verify gate and its guards are validated by `make ze-verify` and the unit stage; no `.ci` functional suite applies.

## Files to Modify
- `scripts/status/verify_run.go` - ~~add `ze-regen-check` to `stagesForMode` (both branches)~~ → add the write-safe read-only regen stage (`ze-regen-check-readonly`, NOT the mutating `ze-regen-check`; R-1 2026-07-17) to `stagesForMode` (both branches).
- `scripts/status/verify_run_test.go` - strengthen guard to set-equality + committed golden; add regen-check assertion.
- `Makefile` - delete `_ze-verify-impl` and `_ze-verify-changed-impl` and their stale comment block.
- `Makefile` - add a write-safe read-only `ze-regen-check-readonly` target (composed of the existing `--check` invocations at `:436-444` plus `ze-plugin-imports-check` and `ze-ai-check`; NO `ze-regen` prerequisite, no tree writes); leave the mutating `ze-regen-check` for manual/regen use (R-1 2026-07-17).
- `scripts/dev/commit_helper.py` - drop `ze-cli-grammar-check` from STRUCTURAL_GATES.

## Files to Create
- `internal/component/plugin/all/yang_glue_check_test.go` - `TestYANGGlueCurrent` feeder (mirrors `TestGeneratedPluginImportsCurrent`).

## Implementation Steps
1. **Wiring first**: add `TestStagesForModeMatchesGolden` (set-equality, both modes) and `TestYANGGlueCurrent`; watch them FAIL against the current tree.
2. Add ~~`ze-regen-check`~~ the write-safe read-only regen stage (`ze-regen-check-readonly`; R-1 resolved 2026-07-17 — see Notes) to both `stagesForMode` branches; do NOT wire the mutating `ze-regen-check` (it runs `ze-regen`).
3. Delete the dead `_impl` targets and their comment; re-grep to confirm no caller (A-1).
4. Remove `ze-cli-grammar-check` from STRUCTURAL_GATES; add the Python assertion that STRUCTURAL_GATES ⊆ live stage names.
5. Run `make ze-verify` (or the unit + guard subset) and confirm green.
6. Complete spec: audit tables, `plan/learned/NNN-verify-stage-ssot.md`, two-commit closure.

## Checklist

### Goal Gates
- [ ] AC-1..AC-5 all demonstrated
- [ ] Wiring Test table complete, every row a concrete test name
- [ ] `make ze-test` passes
- [ ] Registration over hardcoding respected (single guarded stage source)
- [ ] Dead `_impl` targets deleted, no orphaned callers

### Quality Gates
- [ ] Implementation Audit complete
- [ ] R-1 (tree mutation by `ze-regen-check`) resolved and documented

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] `make ze-test` green

## Notes
- Skeleton captured from the 2026-07-16 repository audit. All citations verified against source: `stagesForMode` omits `ze-cli-grammar-check`/`ze-regen-check`/`ze-yang-glue-check`; the `_impl` targets have zero callers; the guard is a 4-name subset check; CI runs only `make ze-verify`.
- Open question for design step: should `ze-regen-check` join both mode branches given it regenerates files (R-1)? Confirm whether it can run write-safe under verify, or gate it behind a clean-tree assertion.

→ AUTONOMOUS DEFAULT (2026-07-17): NO — do not wire the mutating `ze-regen-check` into `stagesForMode`. Grounded finding: `ze-regen-check` (`Makefile:430`) is `ze-regen-check: ze-regen`; its prerequisite `ze-regen` (`Makefile:427`) runs `generate ze-ai-instructions ze-ai-sync ze-doc-index ze-rules-index ze-discovery-index`, which WRITES generated files (`all.go`, yang `register.go`/`embed.go`, `CLAUDE.md`/`AGENTS.md`, the `ai/*` doc indexes) and only THEN asserts `git diff --quiet` (`Makefile:431-435`). It therefore mutates the working tree before checking cleanliness — putting it in verify makes `make ze-verify` leave a dirty tree whenever anything is stale, violating R-1 (verify must not leave a dirty tree).
DECISION: wire a WRITE-SAFE, read-only `--check`-style stage instead (proposed target `ze-regen-check-readonly`, implementer's name choice) that NEVER runs `ze-regen`. Compose it only from existing read-only checks that already detect the same staleness without writing: the recipe's own `--check` calls (`Makefile:436-444`: `code_to_docs.py --check`, `rules_index.py --check`, `arch_map.py --check`, `package_map.py --check`, `docs_to_code.py --check`, `learned_index.py --check`, `learned_numbers.py --check`, `skill_sync.sh --check`, `check_doc_links.py --md-only`) plus the standalone codegen checks `ze-plugin-imports-check` (`Makefile:102`, covers `all.go`) and `ze-ai-check` (`Makefile:424`, covers `CLAUDE.md`/`AGENTS.md`/skill mirrors). yang `register.go`/`embed.go` staleness is covered write-safe by the new `TestYANGGlueCurrent` feeder in the already-run unit stage. Wire `ze-regen-check-readonly` into BOTH `stagesForMode` branches; leave the mutating `ze-regen-check` as a manual/regen convenience, never in verify.
COVERAGE OBLIGATION: before treating the read-only stage as equivalent, the implementer MUST confirm every `generate`/`ze-regen` output that the mutating `git diff` (`Makefile:431-435`) previously covered has a standalone `--check` (or a unit-stage feeder). Any `generate` output lacking one must get a read-only `--check` (or a feeder) — otherwise its staleness is silently unguarded. Fail loud; never silently regenerate under verify.
Rationale: R-1 — verify must never leave a dirty working tree; the read-only `--check` variants give identical staleness detection without writing. STAKES: scope. Thomas: override if you would instead keep the single mutating `ze-regen-check` and guard verify with a clean-tree assertion that fails loudly when the tree is dirty after regen; the write-safe read-only variant is the conservative default.
- Citation drift confirmed & corrected on 2026-07-17 (behavior verified real, line numbers re-grounded): `ze-regen-check` is at `Makefile:430` (its `ze-regen` prerequisite is `Makefile:427`), not `:427` as first written; `ze-cli-grammar-check` in `commit_helper.py` STRUCTURAL_GATES is at `:507` (frozenset opens `:500`), not `:499`. All other citations verified exact: `verify_run.go` `stagesForMode` :112-158 (both branches omit `ze-regen-check`/`ze-yang-glue-check`/`ze-cli-grammar-check`), dead `_impl` targets `Makefile:290`/`:297` (both still list `ze-cli-grammar-check`), guard `verify_run_test.go:345-363` (subset-of-4), `all_test.go:170` feeder, `yang_glue.go` `--check` :26, `.woodpecker/verify.yml:19` runs only `make ze-verify`.
