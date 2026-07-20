# Spec: fixit-verify-stage-ssot

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | - |
| Updated | 2026-07-20 |

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

### AC evidence (2026-07-20)

| AC | Evidence |
|----|----------|
| AC-1 | Both targets and their comment block deleted; the `.PHONY` entry at `Makefile:6` dropped them too. `grep -rn '_ze-verify-impl\|_ze-verify-changed-impl' Makefile mk/*.mk` -> NONE. Remaining mentions are prose in `plan/learned/*` (historical record) and deliberate past-tense comments in `verify_run.go` / `verify_run_test.go` / `commit_helper_test.py` explaining why the duplicate must not come back. |
| AC-2 | `TestStagesForModeMatchesGolden` (`scripts/status/verify_run_test.go`). ORDERED equality per mode against two committed goldens, plus a duplicate-name check -- strictly stronger than set-equality (a silent reorder also fails). RED before the fix: `"ze-verify" has 20 stages, golden has 21` and `"ze-verify-changed" has 17, golden has 18`. GREEN after. |
| AC-3 | `TestYANGGlueCurrent` (`internal/component/plugin/all/yang_glue_check_test.go`). Non-vacuity PROVEN empirically: dropping a stray `ze-fixit-vacuity-probe.yang` into `internal/plugins/ping-cmd/yang/` and running with `-count=1` fails with `stale: .../embed.go`, `stale: .../register.go`; removing it returns to green. → Constraint: `-count=1` is REQUIRED to see it. Without it `go test` served a cached PASS, because a `.yang` is not a declared input of package `all`. The uncached backstop is the `ze-regen-check-readonly` make stage, which runs the same `yang_glue.go --check` from a recipe. The same caching caveat applies to the pre-existing `TestGeneratedPluginImportsCurrent`. |
| AC-4 | `TestStagesForModeIncludesRegenCheck` asserts `ze-regen-check-readonly` present in BOTH branches AND that the mutating `ze-regen-check` is absent from both. Write-safety verified empirically: `git status --porcelain` identical before and after `make ze-regen-check-readonly`, on both a stale tree (the run that failed on `.counter`) and a clean one. R-1 satisfied. |
| AC-5 | `ze-cli-grammar-check` removed from `STRUCTURAL_GATES` (`commit_helper.py`). `TestStructuralGatesAreLiveStages` (`scripts/dev/commit_helper_test.py`, run under `go test` via `TestPythonUnitTests`) derives the live names by parsing `mk("...")` out of `stagesForMode` and asserts `STRUCTURAL_GATES ⊆ live`, with a `len(live) > 10` anti-vacuity guard and a non-empty-frozenset guard. RED before the fix (`['ze-cli-grammar-check'] != []`), GREEN after. |

### Coverage obligation (R-1 resolution) -- discharged

Every `generate` / `ze-regen` output has a read-only check reachable from
`ze-regen-check-readonly`, EXCEPT the two documented exclusions at the bottom.
The target is composed of prerequisite TARGETS, not re-typed recipes, so three
previously-callerless check targets now have a caller.

| Generator | Output(s) | Covering prerequisite |
|-----------|-----------|-----------------------|
| `plugin_imports.go` | `internal/component/plugin/all/all.go` | `ze-plugin-imports-check` |
| `yang_glue.go` | `yang/*/register.go`, `embed.go` | `ze-yang-glue-check` (+ unit-stage feeder) |
| `feature_tags.go` | `.golangci.yml`, `gokrazy/ze/config.json`, `docs/guide/quickstart.md` | `ze-feature-tags-check` **(was diff-only)** |
| `fuzz-targets.py` | `mk/test-fuzz-targets.mk` | `ze-fuzz-targets-check` **(was diff-only)** |
| `code_to_docs.py` | `ai/CODE-TO-DOCS.md` | `ze-doc-check-stale` |
| `rules_index.py` | `ai/rules/INDEX.md` | `ze-rules-index-check` |
| `arch_map.py` | architecture lists in `ai/INSTRUCTIONS.md` | `ze-arch-map-check` (NEW target; note this output is **not** covered by `ze-doc-test`, so it is the one prerequisite with no second guard) |
| `package_map.py`, `docs_to_code.py`, `learned_index.py`, `learned_numbers.py` | the `ai/` discovery indexes + learned numbering | `ze-discovery-index-check` |

→ Constraint: this table is no longer the guarantee -- `TestRegenCheckReadonlyCoversGenerators`
is. It asserts the prerequisite set EXACTLY (not a floor), and walks both the
`generate:` recipe and `ze-regen`'s prerequisite targets, failing on any producer
with no recorded check. Proven by injection three ways: dropping the four doc
prerequisites, adding a generator to `generate:` behind a blank recipe line, and
adding one to `ze-regen` instead of `generate`.

**Two deliberate exclusions:**

→ Decision: `skill_sync.sh --check` (`CLAUDE.md`, `AGENTS.md`, the three skill
mirrors) is NOT wired in. Every one of its outputs is gitignored, so they do not
exist at all in the fresh checkout CI runs against, and the check exits 1 there:
it would have reddened every CI run. Nothing committed can drift, so CI has
nothing to catch; the guard stays where it already was, the warn-only
`.claude/hooks/session-start.sh`. This supersedes the skeleton's "plus
`ze-ai-check`" line under Files to Modify, written before the fresh-checkout
behavior was known.

→ Decision: `check_doc_links.py --md-only` was NOT carried over from the
`ze-regen-check` recipe. It checks references, not generated-file staleness, and
the `ze-doc-links` stage runs the FULL check (a superset) in the slot immediately
before this one.

### Reds this newly-wired gate found on its first run (all pre-existing, all fixed)

| Red | Fix |
|-----|-----|
| `plan/learned/.counter` was 1221 with the highest summary at 1221 (must be >= highest+1) | `learned_numbers.py` allocation via `commit_helper.py learned-next` |
| skill mirrors stale (`ze-hunt`, `ze-weekly-update`) | `make ze-ai-sync` (gitignored outputs only, no tracked change) |
| `ze-doc-links` red at HEAD: 4 `// Design:` refs to `plan/spec-fixit-bcrypt-hash-credential.md`, deleted at closure by `e355eb715` | repointed to `plan/learned/1181-fixit-bcrypt-hash-credential.md` (established convention); `ai/DOCS-TO-CODE.md` regenerated |
| `ze-doc-links` red at HEAD: `ai/patterns/functional-test.md` row for `test/pppoe/`, a suite that has never existed (only `test/pppoe-interop/`) | phantom row removed |
| `ze-doc-test` red at HEAD: docs and `make help` claimed 22 release-gate suites, Makefile has 23 (`runner` added without doc updates) | both counts corrected; `make ze-doc-drift` now clean |

→ Constraint: `ze-rfc-check` is ALSO red at HEAD (39 stale RFC7606 audit verdicts,
caused by the requirement-text edits in `24c7ca1a5`/`feaac5617`). Confirmed
pre-existing by running HEAD's own `scripts/dev/rfc_requirements.py --check`:
same 39 violations. Out of scope here, and an active concurrent session owns that
area (`plan/spec-rfc-gate-regression-ratchets.md`, uncommitted edits to
`rfc_requirements.py`). Fix is `/ze-rfc-audit rfc7606`. Likewise 3 pre-existing
`unparam` lint findings in `internal/plugins/{isis,ospf}` (files untouched here);
`golangci-lint run` over only this change's packages returns `0 issues`.

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

## Review Gate

<!-- INDEPENDENT reviewer subagents over the working-tree diff
     (ai/rules/critical-review.md). Artifact recorded via review_gate.py. -->

### Run 1 (2026-07-20) -- 2 independent subagents, distinct lenses

Lens A: wiring / coverage regressions / removed behavior. Lens B: test quality and
vacuity (mutation-tested every new test via `go test -overlay=` and a mirrored fake
repo root). Both reviewers reproduced their findings empirically. **2 BLOCKER,
10 ISSUE, 11 NOTE.** Every BLOCKER and ISSUE was independently re-verified by the
author before acting (`ai/rules/critical-review.md` step 3) -- one reviewer repro
turned out to be right for a reason the author's first attempt got wrong, recorded
below.

| # | Sev | Finding | Location | Action |
|---|-----|---------|----------|--------|
| 1 | **BLOCKER** | `skill_sync.sh --check` in the new stage reds **every CI run**: all its targets (`CLAUDE.md`, `AGENTS.md`, `.claude`/`.codex`/`.agents/skills`) are gitignored, so on the fresh checkout CI runs `make ze-verify` against they do not exist at all | `Makefile` `ze-regen-check-readonly` | **FIXED** -- removed from the target, with the reasoning and the reproduction recorded in a comment block so it is not "helpfully" re-added. Verified: `make -n ze-regen-check-readonly` no longer mentions `skill_sync`. The warn-only `.claude/hooks/session-start.sh` check stays as the (correct) home for it |
| 2 | **BLOCKER** | `code_to_docs.py --check` is **fail-open**: it builds `content` then validates only that anchor paths exist, never comparing against `output_file`. So `ai/CODE-TO-DOCS.md` can be arbitrarily stale while the check prints "all references valid" and exits 0. The spec's own coverage obligation named this file as covered -- a false claim | `scripts/dev/code_to_docs.py` | **FIXED** -- added the content comparison, exit 1 + "run: make ze-doc-index" on mismatch. Author-verified the fail-open empirically first: the file was silently stale by **24 code paths** (recorded 1439 vs live 1463) while `--check` passed. Regenerated. Non-vacuity proven: appending one stray line makes `--check` exit 1 |
| 3 | ISSUE | `ze-regen-check-readonly` was not added to `STRUCTURAL_GATES`; a stale generated file is deterministic and must never be parkable as known-red | `scripts/dev/commit_helper.py` | **FIXED** -- added |
| 4 | ISSUE | `ai/rules/git-safety.md` and `plan/known-failures.md` still enumerate `ze-cli-grammar-check` as an enforced structural gate; false after the removal | both files | **FIXED** -- swapped for `ze-regen-check-readonly`; added a paragraph in `git-safety.md` explaining the invariant (every name must be a real `stagesForMode` stage) and naming the two tests that now enforce it |
| 5 | ISSUE (both reviewers, independently) | **AC-2's "fails if the two branches diverge" was NOT met.** Each mode is pinned to its own independent golden, so wiring a gate into one branch fails exactly one sub-test; updating that one golden ships the divergence green. Lens B PROVED the blessed-divergence pass. Worse, the failure message said "Update the golden in this file if the change is deliberate" -- steering the reader straight into it | `scripts/status/verify_run_test.go` | **FIXED** -- new `TestStagesForModeBranchesAgree` asserts the two branches carry an identical gate set after removing a documented `modeSpecificStages` allowlist (which itself fails if it names a stage the function no longer emits). Message rewritten to "add the stage to BOTH branches, then update BOTH goldens". Proven: injecting a gate into one branch now fails |
| 6 | ISSUE | The Python `STRUCTURAL_GATES` check runs as a `python3` subprocess, so `verify_run.go` is **not a `go test` cache input** for it. Editing the stage list alone serves a cached PASS; under `ze-verify-changed`, `changed-pkgs.sh` maps a `*.go` edit to `./scripts/status` only, never `./scripts/dev`. Same caching hole the author documented for the yang feeder, in a test with no backstop | `scripts/dev/commit_helper_test.py` | **FIXED** -- added the Go twin `TestStructuralGatesAreLiveStages` in `scripts/status`, which calls `stagesForMode` in-process (so a `verify_run.go` edit invalidates it) and parses the frozenset from `commit_helper.py`. Kept BOTH: an edit to `commit_helper.py` invalidates `./scripts/dev` and re-runs the Python one. Each closes the other's blind side; both doc comments say so |
| 7 | ISSUE | The Python regex matched `mk("...")` in **comments**, so a commented-out or historical stage counted as live and silently re-admitted a dead `STRUCTURAL_GATES` entry. Lens B proved the false PASS | `scripts/dev/commit_helper_test.py` | **FIXED** -- strip `//` comments before matching |
| 8 | ISSUE | `TestYANGGlueCurrent` passes **vacuously** if `discoverYangDirs` matches nothing: `yang_glue.go` prints "no yang/ directories with .yang files found" and exits 0 | `internal/component/plugin/all/yang_glue_check_test.go` | **FIXED** -- assert the reported directory count against a floor of 100 (149 live), mirroring `TestPythonUnitTests`' empty-glob guard |
| 9 | ISSUE | The `PREVENTS:` on `TestStagesForModeIncludesRegenCheck` **over-claimed**: `all.go` was already covered by `TestGeneratedPluginImportsCurrent` and `TestVerifyWiringDocsChecksPluginImports`, and `CLAUDE.md`/`AGENTS.md` by the session-start hook | `scripts/status/verify_run_test.go` | **FIXED** -- rewritten to separate NEW coverage from REDUNDANT coverage from NOT-covered (`ai/rules/no-fabrication.md`) |
| 10 | ISSUE | No test enforced the coverage obligation the Makefile comment asserts: a new generator in `generate` with no `--check` leaves its output unguarded. The drift class the spec exists to kill, reintroduced one level up | `Makefile` | **FIXED** -- `TestRegenCheckReadonlyCoversGenerators` parses the `generate:` recipe and the target's prerequisites and fails on an unmapped generator. Proven: dropping `ze-feature-tags-check` from the prerequisites fails with the exact generator named |
| 11 | ISSUE | `_live_stage_names` raised a bare `ValueError: substring not found` if `stagesForMode` is renamed or moved | `scripts/dev/commit_helper_test.py` | **FIXED** -- explicit `self.fail` messages on all three failure paths |
| 12 | NOTE→FIXED | The new target re-typed recipes instead of depending on the existing `ze-yang-glue-check` / `ze-feature-tags-check` / `ze-fuzz-targets-check` targets (all of which still had zero callers), creating a fifth copy of "how to check yang glue" | `Makefile` | **FIXED** -- rebuilt as 8 prerequisite targets. This also revives three dead targets, which is squarely the spec's SSOT intent |
| 13 | NOTE→FIXED | Nothing asserted a stage name is a real make target; a consistent typo in `stagesForMode` **and** its golden passes every unit test and only explodes mid-verify as `make: *** No rule to make target` | `scripts/status/verify_run_test.go` | **FIXED** -- `TestStagesAreRealMakeTargets` parses rule heads from `Makefile` + `mk/*.mk`. Proven by injection |
| 14 | NOTE→FIXED | The caching caveat was written as if yang-specific; `TestGeneratedPluginImportsCurrent` has the identical hole and kept an unqualified VALIDATES/PREVENTS | `internal/component/plugin/all/all_test.go` | **FIXED** -- caveat added there too, naming its backstop |
| 15 | NOTE→FIXED | Discovery: the new target was absent from `make help-dev` and from the documented verify order | `Makefile`, `docs/functional-tests.md` | **FIXED** -- help entry added (marking `ze-regen-check` as the one that REGENERATES first); the documented order rewritten to the full live list, which had omitted **eight** stages, and annotated with where the SSOT lives |
| 16 | NOTE | `skill_sync.sh` never removes a mirror whose canonical `ai/skills/<name>.md` was deleted, so an orphan makes `--check` fail with a remediation ("run: make ze-regen") that cannot fix it | `scripts/dev/skill_sync.sh` | **ACKNOWLEDGED, out of scope.** Moot for verify now that the check is not wired there (finding 1). Real latent bug in the session-start advisory path; recorded here rather than fixed, because it is a separate concern from the stage list |
| 17 | NOTE | `ai/INDEX.md` says `learned_numbers.py --check` is folded into `ze-doc-test` and `ze-regen-check`; now also `ze-regen-check-readonly` | `ai/INDEX.md` | **NOT FIXED, deliberately.** That file carries an active concurrent session's uncommitted edits; touching it would force a cross-session commit. Left for whoever owns that change |
| 18 | NOTE | The mutating `ze-regen-check` survives with a hand-maintained 10-path `git diff` list nothing tests; `ze-yang-glue-check` etc. were dead targets | `Makefile` | Partly addressed by finding 12 (the dead targets now have a caller) and finding 10 (the generator→check mapping is tested). The mutating target's `git diff` list remains untested; it runs nowhere, so it cannot mislead a gate |

### Fixes applied

All 2 BLOCKER and all 10 ISSUE findings fixed; 4 NOTEs promoted and fixed; 3 NOTEs
acknowledged with reasons above. Every fix that changes behavior was proven by
injection (see the AC evidence table and the mutation results quoted per row).

### Run 2 (2026-07-20) -- fresh adversarial pass over the round-1 fixes

Round-1 fixes are new code, so they get their own pass (`ai/rules/critical-review.md`
step 4: "Every fix is new code that needs a fresh pass"). **This pass earned its
keep: it found a BLOCKER that a round-1 FIX had introduced.** 1 BLOCKER, 7 ISSUE.
All re-verified by the author before acting.

| # | Sev | Finding | Location | Action |
|---|-----|---------|----------|--------|
| R2-1 | **BLOCKER** (regression from the round-1 BLOCKER-2 fix) | The new freshness comparison could **never pass** when any doc anchor was broken, because `stale` is populated only in check mode yet fed a `## Stale References` section into `content`. So check-mode `content` differed from what generate mode writes, the error said "run: make ze-doc-index", and running it **did not help** -- an unfixable loop on a commit-blocking gate. Worse, it short-circuited before the `MISSING: <path>` report, making the tool's original diagnostic dead code | `scripts/dev/code_to_docs.py` | **FIXED** -- the stale table no longer contributes to `content`, so it is identical in both modes; broken anchors are reported to stdout as before. Author-reproduced first (add a bogus `<!-- source: -->`, regenerate, watch `--check` still fail). The section had never reached disk anyway: `grep -c 'Stale References' ai/CODE-TO-DOCS.md` = 0. All four combinations now behave: fresh+valid exit 0; stale FILE exits 1 naming the regen target; broken ANCHOR exits 1 with `MISSING:` and its doc:line; and regenerating clears the file complaint so the anchor complaint is reachable |
| R2-2 | ISSUE | `ze-regen-check-readonly` in `STRUCTURAL_GATES` makes some reds un-parkable that are parkable when the same underlying check reds `ze-doc-test` | `scripts/dev/commit_helper.py` | **KEPT, with the inconsistency recorded.** The stage meets the rule's own definition: deterministic, reproducible, source-fixable, never flaky or environmental. R2-1 was what made the remediation impossible, and that is fixed. The `ze-doc-test` asymmetry predates this change; documented in `ai/rules/git-safety.md` with a note that the open question is whether `ze-doc-test` belongs in the set, not whether this one does |
| R2-3 | ISSUE | The coverage guard used a `len(prereqs) >= 4` FLOOR, so half the prerequisites -- including `ze-arch-map-check`, the **only** guard for `ai/INSTRUCTIONS.md`'s architecture lists (not covered by `ze-doc-test`) -- could be deleted with every test still green. Reviewer proved it | `scripts/status/verify_run_test.go` | **FIXED** -- `regenCheckPrereqs` asserts the EXACT set both ways (missing and undocumented-extra), each entry carrying the generator and output it guards. Proven by deleting the four doc prerequisites |
| R2-4 | ISSUE | The Makefile comment claimed the guard "derives the required set from the `generate` / `ze-regen` recipes"; it only parsed `generate:`, so a generator added to `ze-regen` got no check at all (`ai/rules/no-fabrication.md`) | `Makefile` | **FIXED** -- the guard now walks `ze-regen`'s prerequisite targets too, with each mapped to its covering check or recorded as an explicit exclusion. Proven by adding a generator to `ze-regen` only |
| R2-5 | ISSUE | `(?ms)^generate:.*?\n\n` truncates at the first blank line, and GNU make **permits blank lines inside a recipe** -- so a stray blank line hid every generator below it. Reviewer proved a new unguarded generator being accepted silently | `scripts/status/verify_run_test.go` | **FIXED** -- replaced with a line-based `recipeBody()` that ends the recipe at the first line starting at column 0. Proven by re-running the reviewer's exact mutation. (Also checked by hand: no rule-head prefix collisions -- `"ze-regen:"` cannot match `ze-regen-check:`, whose next character is `-`; and no continuation lines in either recipe) |
| R2-6 | ISSUE | `commit_helper.py`'s operator-facing comment and `UsageError` text still enumerated `cli-grammar` and omitted the gate that had just been added. The rules docs were updated; the code that actually prints to the operator was not | `scripts/dev/commit_helper.py` | **FIXED** -- both updated; no `cli-grammar` references remain in that file |
| R2-7 | ISSUE | The rewritten verify-order paragraph created a **third unguarded copy of the stage list**, in prose, with nothing checking it (`doc_drift.go` does not parse that sentence) -- the exact failure class this spec exists to eliminate | `docs/functional-tests.md` | **FIXED** -- the enumeration is gone, replaced by a pointer to `stagesForMode`, a `make -n ze-verify` recipe for reading the live list, a shape-level summary, and an explicit "an earlier version of this paragraph had silently gone eight stages out of date; please do not re-add one" |
| R2-8 | ISSUE | The spec's own coverage table had gone false: it still listed `skill_sync.sh` as covered, and Files to Modify still said "plus `ze-ai-check`" | this spec | **FIXED** -- table rewritten around the prerequisite targets, both exclusions stated with reasons, and the guarantee re-pointed at `TestRegenCheckReadonlyCoversGenerators` rather than the prose table |
| R2-9..14 | NOTE | 6 of 8 prerequisites duplicate `ze-doc-test` (measured 3.97 s serial / 2.17 s `-j8`, so cost is not the issue); `git-safety.md` prose omitted `ze-lint-changed`; `documentation-testing.md` still described the old `code_to_docs --check` behavior; a `modeSpecificStages` edge case reported the opposite of the truth; an ungrammatical Makefile comment and a "named below" that named nothing; `TestYANGGlueCurrent`'s no-yang-dirs path blamed the wrong cause | various | **ALL FIXED** except the duplication NOTE, which is deliberate (the two stages have different parkability, per R2-2, and the cost is ~2 s) |

Reviewer also independently confirmed CORRECT: the BLOCKER-1 fix (all 8 prerequisites
pass on a reconstructed fresh checkout; `skill_sync.sh --check` fails there),
write-safety under `make -j8`, `.PHONY` completeness, `code_to_docs.py` output
determinism (every emission path `sorted()`, so no walk/dict/locale flakiness),
the closed blessed-divergence hole, `makeTargetRE` safety (223 targets parsed, no
`::=` in the corpus), and the `check_doc_links.py --md-only` exclusion reasoning.

### Run 3 (2026-07-20) -- pass over the round-2 fixes

1 BLOCKER, 7 ISSUE. The BLOCKER was in a line this spec's round-2 fix had ADDED
to the docs.

| # | Sev | Finding | Location | Action |
|---|-----|---------|----------|--------|
| R3-1 | **BLOCKER** | R2-7's rewrite told readers to run `make -n ze-verify` to see the stage list. That is not a dry run: `ze-verify`'s recipe contains `$(MAKE)`, which GNU make executes even under `-n`, propagating `-n` to all 21 stage sub-makes -- each then echoes its recipe and exits 0. `runVerify` has no dry-run detection, so it would write an all-green failures JSON and `tmp/ze-verify.status` with `exit=0` and the CURRENT tree hash; `verify-status.sh` then reports FRESH and `structural_gate_reds` sees nothing. **One documented command would certify a completely unverified tree as fully verified.** | `docs/functional-tests.md`, `scripts/status/verify_run.go` | **FIXED AT BOTH ENDS.** (a) `verify_run.go` now refuses to run under make's no-execute modes (`makeDryRun`, exit 2) -- a fail-closed guard, not a doc fix. (b) New `make ze-verify-list` target gives the question a safe answer; the doc points there and warns against `-n` explicitly. Proven: `make -n ze-verify` exits 2, writes NO status file, and `verify-status.sh` still reports STALE |
| R3-2 | ISSUE | The coverage guard parsed generators out of `generate:`'s recipe but read only the NAMES of `ze-regen`'s prerequisites, so a script added inside `ze-discovery-index`'s own recipe was invisible | `scripts/status/verify_run_test.go` | **FIXED** -- the walk descends into sub-target recipes and one level of their prerequisites. Doing so immediately surfaced **7 generator scripts that had never been mapped**; all are now classified. Proven with the reviewer's mutation |
| R3-3 | ISSUE | The producer regex matched only `scripts/**.{go,py}`, so `@go run ./cmd/newgen` in `generate:` was silently exempt | same | **FIXED** -- see R4-2, where this was widened further after a second reviewer found `$(GO) run` still escaping |
| R3-4 | ISSUE | `recipeBody` folded no backslash continuations, so a wrapped prerequisite list (valid make) dropped four prerequisites and emitted a literal `\` field -- four FALSE "missing prerequisite" errors | same | **FIXED** -- continuations folded into the rule head. Proven: the wrapped form now parses to the same 8 prerequisites and the test stays green |
| R3-5 | ISSUE | `ai/rules/cli-grammar.md` still claimed `make ze-cli-grammar-check` runs "(in `make ze-verify`)". It never has | `ai/rules/cli-grammar.md` | **FIXED** -- corrected, and points at `TestCLIGrammarGateStatic` as the gate's actual route into CI |
| R3-6 | ISSUE | The `>= 5` floor on `ze-regen`'s prerequisites had slack, so one could be DELETED unnoticed -- `make ze-regen` would stop regenerating a file the un-parkable gate still checks | `scripts/status/verify_run_test.go` | **FIXED** -- exact count. Proven by deleting `ze-doc-index` |
| R3-7 | ISSUE | The Makefile coverage comment listed `learned_numbers.py` as a `ze-regen` generator; `ze-discovery-index` never runs it (only its `--check` twin does) | `Makefile` | **FIXED** in the rewritten comment block |
| R3-8 | ISSUE | The comment directly above `STRUCTURAL_GATES` never mentioned a stale generated file, while the frozenset below it now contains `ze-regen-check-readonly` | `scripts/dev/commit_helper.py` | **FIXED**, plus a pointer to the two tests that enforce the live-stage invariant |
| R3-9 | NOTE→FIXED | `code_to_docs.py`'s missing-file path gave no remediation, unlike its stale path | `scripts/dev/code_to_docs.py` | **FIXED** -- names the regen target. Proven by moving the file away |
| R3-10 | NOTE | Reviewer's firm opinion: my R2-2 parkability paragraph named the wrong mechanism. A shared check reds BOTH stages in the same run (`runVerify` has no `break`), so the harmful direction is unreachable; the real gap is `ze-doc-test`'s EXCLUSIVE checks | `ai/rules/git-safety.md` | **FIXED** -- paragraph rewritten to say that, and to point the follow-up at `ze-doc-test`. Refined again in R4-9 |

### Run 4 (2026-07-20) -- pass over the round-3 fixes

1 BLOCKER, 4 ISSUE, 5 NOTE. The BLOCKER was a hole in round 3's own fix.

| # | Sev | Finding | Location | Action |
|---|-----|---------|----------|--------|
| R4-1 | **BLOCKER** | `makeDryRun` tested only `'n'`, but `make -t`/`--touch` has the identical two properties: it still EXECUTES `$(MAKE)` recipe lines, and it makes every other recipe a no-op. For a `.PHONY` stage it prints "Nothing to be done" and exits **0** -- QUIETER than `-n`, which at least echoes. So `make -t ze-verify` forged exactly the FRESH status the round-3 guard was written to prevent | `scripts/status/verify_run.go` | **FIXED** -- guard widened to `ntq` (`-q` runs nothing and exits 1: not a forgery, but it would clobber a good record with a false red). Author-verified before acting: with `MAKEFLAGS=t`, a `.PHONY` target whose recipe is `exit 9` exits **0**, and a `$(MAKE)` recipe line still executes. Both directions pinned in `TestMakeDryRunDetectsDashN` |
| R4-2 | ISSUE | `producerScripts` missed `$(GO) run` -- which is the house idiom at the `ze-verify` and `ze-verify-changed` recipes themselves -- plus `${GO} run`, `uv run` (used by the ExaBGP suite), and `bash`/`sh <script>.sh` | `scripts/status/verify_run_test.go` | **FIXED** -- interpreter prefix now accepts a literal name or a `$(VAR)`/`${VAR}` reference, and the interpreter set covers every form this repo uses. Proven with `$(GO) run` and `uv run` generators hidden inside `ze-discovery-index` |
| R4-3 | ISSUE | The continuation bug R3-4 fixed in the rule head was still live in recipe BODIES: a wrapped `@python3 \` + script yielded a literal `\` as a "generator" (a false red), and a commented-out `# go run x.go` yielded a phantom | same | **FIXED** -- continuations folded and `#` comments stripped before matching. Proven both ways: the wrapped form now reports the REAL script name, the commented-out form reports nothing |
| R4-4 | ISSUE | `stagesForMode`'s `default` branch returned the FULL list for ANY unrecognized mode, so `verify_run.go ze-verify-chnaged` ran full verify and wrote `mode=ze-verify-chnaged`, which `verify-status.sh` renders as `FRESH(ze-verify-chnaged)`. `--list` had the same fail-open, and `$(or $(MODE),...)` imported `MODE` from the ENVIRONMENT | `scripts/status/verify_run.go`, `Makefile` | **FIXED** -- unknown mode returns no stages (fail closed; `runVerify` turns that into exit 2), `--list` exits 2 naming the valid modes, and the make variable is now `ZE_VERIFY_MODE`. New `TestStagesForModeRejectsUnknownMode` pins both directions. Proven end to end |
| R4-5 | ISSUE | The deeper walk called `recipeBody` on every prerequisite field, whose `t.Fatalf` would fire on a legal file/variable/order-only prerequisite, blaming the reader | `scripts/status/verify_run_test.go` | **FIXED** -- `optionalRecipeBody` skips non-rule-heads instead of failing |
| R4-6 | NOTE→FIXED | `recipeBody` could slice past the end if a continued rule head ran off the corpus | same | **FIXED** -- index clamped |
| R4-7 | NOTE→FIXED | `ze-verify-list` was undiscoverable: absent from `make help-dev`, though the same change added `ze-regen-check-readonly` there | `Makefile` | **FIXED** -- added, with the "never use `make -n ze-verify`" warning inline |
| R4-8 | NOTE | The refusal happens AFTER `verify-lock.sh` takes the lock, so a no-execute invocation still waits on a concurrent verify and appends a 0-second duration row | `Makefile` | **ACCEPTED, documented in the recipe comment.** Guarding earlier means re-implementing the MAKEFLAGS parse in shell or a make conditional, and that parse is subtle: measured, a naive `$(findstring n,$(MAKEFLAGS))` MATCHES `--no-print-directory` and would refuse every real verify. One tested implementation beats two, and nothing is forged either way |
| R4-9 | NOTE→FIXED | My R3-10 rewrite overstated one claim: `rfc_requirements.py` is not `ze-doc-test`-exclusive (its `--selftest`/`--check` run as the `ze-rfc-check` stage); only `--check-fresh` is | `ai/rules/git-safety.md` | **FIXED** -- narrowed to the `--check-fresh` invocation |
| R4-10 | NOTE→FIXED | `ze-arch-map` was missing from `.PHONY` although `ze-arch-map-check` was added | `Makefile` | **FIXED** |

Reviewer independently confirmed CORRECT: **no `makeDryRun` false positives** across
`-j8`, `-k`, `-B`, `-s`, `-w`, `-i`, `-e`, `-p`, `-d`, `-l2`, `-O`, `--output-sync`,
`-r -R`, `--warn-undefined-variables`, `-S`, `-C`, `-I`, combinations, and recursive
`$(MAKE)` children (I probed the same set independently and agree); exit code 2
propagates with no stale lock; `--list` does not collide with the positional mode
and is correctly ordered BEFORE the refusal; five independent `Fatalf` tripwires
make the coverage test non-vacuous; `runVerify` really has no `break` in its stage
loop, which is what makes the git-safety claim true; and `ze-regen-check-readonly`
still leaves the tree byte-identical.

### Run 5 (2026-07-20) -- pass over the round-4 fixes

Triggered by the gate, not by me: `review_gate.py check` reported the round-2
artifact STALE for `Makefile`, `verify_run.go` and `verify_run_test.go`, because
I recorded it before rounds 3 and 4 produced their fixes. That is exactly the
"review then quietly edit" hole the gate exists to close, and it caught me.

**0 BLOCKER, 3 ISSUE, 6 NOTE.** The security-critical change is CONFIRMED sound:
the reviewer probed ~45 GNU make flag forms and found `n`, `t`, `q` are the only
letters that reach MAKEFLAGS' first field for a no-execute mode -- `-B` forces
execution, and `-o`/`-W`/`--old-file`/`--assume-old`/`--new-file`/`--what-if` are
not propagated into MAKEFLAGS at all, so they cannot make a stage no-op. No false
positives (short flags always precede argument-bearing options in field 0, checked
for `-j8 -n`, `-n -j8`, `-s -j8 -n`, `--jobs=4 --touch`, `-C . -n`). End-to-end in
a sandbox: `n`/`t`/`q` -> exit 2 with NO status file written; `""`/`-j8`/
`--no-print-directory` -> runs normally.

The damage was in round 4's two SECONDARY fixes, both mine:

| # | Sev | Finding | Action |
|---|-----|---------|--------|
| R5-1 | ISSUE | The `MODE` -> `ZE_VERIFY_MODE` rename BROKE the command round 3 had just documented. `docs/functional-tests.md` still said `MODE=ze-verify-changed`, which silently prints the FULL list -- so a reader concludes `ze-verify-changed` runs `ze-alloc-gate`/`ze-vet-evidence`/`ze-unit-test-race-changed` when it does not. Exit 0, no warning, and nothing checks that prose | **FIXED** -- doc corrected; verified the command now prints the changed list |
| R5-2 | ISSUE | My comment-stripping fix traded a false positive for a FALSE NEGATIVE, the dangerous direction: `#` was stripped unconditionally, including inside shell quotes, so `@echo "# hi" && python3 .../newgen.py` hid a generator entirely | **FIXED** -- `stripShellComment` tracks quote state. Proven both ways: the quoted-`#` generator is now CAUGHT, a genuinely commented-out one is still IGNORED |
| R5-3 | ISSUE | The `producerScripts` doc comment claimed an unlisted form "is NOT silently ignored" because the count assertions would fire. False twice over: there is NO count assertion for generators inside SUB-TARGET recipes, and a no-capture form leaves `generate:`'s count at 4. Measured misses: `go run -tags x`, `python3 -m pkg.mod`, `$(MAKE) target` | **FIXED** -- comment rewritten as an HONEST LIMIT naming what still slips through; the first two forms are now matched (both proven caught). `$(MAKE)`-recursive generators remain undetected, stated plainly rather than implied covered |
| R5-4..9 | NOTE | Stale "21 stages" counts in two places; the doc warned only about `-n` when `-t` is the quieter forgery; the `-q` code comment mis-described the behaviour (the top-level `$(MAKE)` line DOES run; it is the sub-makes that no-op); asymmetric unknown-mode diagnostics; `optionalRecipeBody` applied at one level only and without continuation folding | Counts, `-t` warning and `-q` prose **FIXED**. The last two are cosmetic/unreachable today (the `!= 7` count check runs first) and are left recorded |

### Final status
- [x] Reviewers report 0 BLOCKER, 0 ISSUE outstanding: all 4 BLOCKERs and all 22
      ISSUEs across four rounds are fixed, each proven by injection; 3 NOTEs are
      accepted with recorded reasons (R1-16 skill_sync orphans, R1-17 `ai/INDEX.md`
      owned by a concurrent session, R4-8 lock ordering).
- [x] All NOTEs recorded above.
- [x] Closure re-review (2026-07-20): a fresh independent subagent re-reviewed the
      last uncommitted residue (`code_to_docs.py --check`, `all_test.go` caveat) and
      confirmed 0 BLOCKER / 0 ISSUE -- the `--check` is fail-closed on all three
      failure modes (missing file, content mismatch, broken anchors) and wired into
      BOTH verify branches (`ze-doc-test` and `ze-regen-check-readonly`->`ze-doc-check-stale`),
      test-guarded by `TestRegenCheckReadonlyCoversGenerators`. One cosmetic NIT
      fixed: `code_to_docs.py` no longer prints "up to date" one line above a
      non-zero broken-anchor exit; freshness is announced only on the all-clear path.

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
