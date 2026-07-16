# Spec: fixit-doc-gate-and-refs

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-07-16 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `scripts/status/verify_run.go` - `stagesForMode` (the verify pipeline that CI runs)
4. `scripts/docvalid/doc_drift.go`, `scripts/dev/check_doc_links.py` - the two dark gates
5. `mk/inventory.mk` (`ze-doc-test`), `Makefile` (`ze-doc-links`, `ze-regen-check`)

## Task

**[HIGH]** The documentation-consistency gates are DARK: they exist but nothing runs them.
`stagesForMode` (`scripts/status/verify_run.go:112-158`) enumerates every stage of `make
ze-verify` and neither branch includes `ze-doc-test`, `ze-doc-links`, or `ze-regen-check`;
CI runs only `make ze-verify` (`.woodpecker/verify.yml:19`). `doc_drift.go`'s docstring
claims it is "Called by: ... `.claude/hooks/check-doc-drift.sh`" (`scripts/docvalid/doc_drift.go:8`)
but that hook file is absent. Proof the gate is dark: `check_doc_links.py --md-only` exits 1
with 16 broken references while CI stays green, and the broken references are in the
DISCOVERY LAYER itself (index files agents read to find code). Fold in the MEDIUM
count-drift: the headline test counts contradict each other across four files, and
`doc_drift.go` only flags `N+` over-claims, never bare exact counts or undercounts.

Wire the dark gates into the pipeline, fix the 16 stale references, tighten `doc_drift.go`
to catch bare/undercount claims, and unify the contradictory headline counts.

## Required Reading

- [ ] `scripts/status/verify_run.go` - `stagesForMode(mode, makeCmd)` builds the stage list CI executes
  → Constraint: both the `ze-verify-changed` branch (:121-137) and the `default` branch (:138-157) must gain the doc gate, or `ze-verify-changed` sessions skip it.
  → Decision: wiring the gate here (CI-enforced) is the real fix; a `.claude/hooks/*` alternative only fires for Claude, never for CI or other agents.
- [ ] `scripts/dev/check_doc_links.py` - corpus path-reference checker; `--md-only` skips Go `// Design:` refs
  → Constraint: `ze-doc-test` (`mk/inventory.mk:72`) does NOT run this; only `ze-doc-links` (`Makefile:444`) and `ze-regen-check` (`Makefile:441`) do. Wiring `ze-doc-test` alone leaves the 16 broken refs uncaught.
  → Constraint: a line carrying `doc-links: ignore` is skipped; use it for deliberate references to removed paths, not for live stale ones.
- [ ] `scripts/docvalid/doc_drift.go` - `checkReadmeMD` (:544-567) and `extractAtLeast`/`extractApprox` (:797-803)
  → Constraint: today it only flags `([\d,]+)\+` over-claims (`fuzzCount < m`); a bare `57 fuzz targets` or an undercount `10,000+` slips through.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `scripts/status/verify_run.go` - `stagesForMode` returns `[]stage`; no `ze-doc-*`/`ze-regen-check` stage in either branch (:121-157)
  → Constraint: adding one `mk("<gate>")` per branch is the wiring point; `mk` shells `make --no-print-directory <gate>`.
- [ ] `scripts/dev/check_doc_links.py` - `main` exits 1 on any broken reference; `check_markdown` walks the `MD_GLOBS` corpus (:75-93)
  → Constraint: exits 1 today with 16 broken refs (verified 2026-07-16); the gate is correct, it is simply unwired.
- [ ] `scripts/docvalid/doc_drift.go` - `checkReadmeMD` scans README lines for `N+` claims only (:544-567)
  → Constraint: `35 Docker-based interop scenarios` and `57 fuzz targets` (both bare, `README.md:5`) are invisible to it.
- [ ] `.woodpecker/verify.yml` - single step `make ze-verify` (:19); no separate doc job
- [ ] the 8 gate-flagged corpus files (`ai/INDEX.md`, `ai/LEARNED-INDEX.md`, `ai/rules/appliance-dep-bumps.md`, `ai/rules/deferral-tracking.md`, `ai/rules/project-knowledge.md`, `ai/rules/zefs-persistence.md`, `ai/skills/ze-hunt.md`, `ai/skills/ze-weekly-update.md`) plus `README.md` counts
  → Constraint: `internal/core/bgp/{attribute,capability}` and `internal/plugins/ospf/v3` exist (verified); the refs point at pre-move paths, a tier change (`component` → `core`).

**Behavior to preserve:**
- Every currently-passing `ze-verify` stage keeps passing and running in the same order.
- The corpus checker's existing pass/fail semantics and `doc-links: ignore` escape hatch.
- Legitimate historical records (`plan/learned/`, `plan/handover/`) stay excluded from the corpus check.

**Behavior to change:**
- The doc gate (drift + corpus refs) runs inside `make ze-verify`, so a broken reference or count drift fails CI.
- The 16 stale references resolve (path fix, deletion, or `doc-links: ignore` where the target is intentionally gone).
- `doc_drift.go` also flags bare exact counts and gross undercounts; the headline counts are unified.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
CI invokes `make ze-verify` (`.woodpecker/verify.yml:19`), which runs `ze` (or `bin/ze status verify`)
→ `runVerify` → `stagesForMode` (`scripts/status/verify_run.go:112`) → each `stage.Command` is a `make <gate>` shell-out.

### Transformation Path
1. `stagesForMode` returns the stage list; a new `mk("ze-doc-test")` and a corpus-ref stage join both branches.
2. `execStage` shells `make ze-doc-test` → `doc_drift.go` (registry/count drift) and the corpus check → `check_doc_links.py --md-only` (path refs).
3. A broken reference or a drifted count now returns a non-zero stage → `runVerify` fails → CI red.
4. `doc_drift.go`'s new exact/undercount regexes flag counts the `N+`-only path misses.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CI ↔ verify pipeline | `make ze-verify` → `stagesForMode` stage list | [ ] |
| Verify ↔ doc gate | new `ze-doc-test` + corpus-ref stages shelled by `execStage` | [ ] |
| Gate ↔ corpus/README | `check_doc_links.py`/`doc_drift.go` read the discovery files | [ ] |

### Integration Points
`scripts/status/verify_run.go` (stage list), `mk/inventory.mk`/`Makefile` (gate targets),
`scripts/docvalid/doc_drift.go` (count checks), the 8 corpus files, `README.md` (counts),
and the user's memory `reference_python_uv.md`/`MEMORY.md` (also cite deleted `test/stress/bgpgen.py`).

### Architectural Verification
- [ ] No bypassed layers (the gate runs through the same `stagesForMode` → `execStage` path as every other stage, not an ad-hoc script)
- [ ] No duplicated functionality (reuse existing `ze-doc-test`/`ze-doc-links` targets; do not fork a parallel checker)
- [ ] Registration over hardcoding — the gate is discovered via the `stagesForMode` stage list and the `MD_GLOBS`/registry the checkers already enumerate, not a hardcoded file list (`ai/rules/discovery-updates.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | `ze-doc-test` runs clean once refs+counts are fixed | `doc_drift.go` + sub-checks pass locally today except counts | gate stays red; more drift to fix | run `make ze-doc-test` after fixes | unvalidated (resolve during implement) |
| A-2 | Wiring adds bounded time to `ze-verify` | doc checks are `go run`/`python3` scans, seconds each | CI slows unacceptably | time the added stages | unvalidated |
| A-3 | The 2 `deferral-tracking.md` refs and the `zefs-persistence.md:12` symbol ref are intentional examples/false-positives, not live paths | they cite closed specs / a `statestore.Put(` symbol span | wrong fix (deleting a live path) | inspect each line before editing | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Fixing counts to a live-computed value re-drifts as the tree grows | count changes next week | prefer soft `N+`/`roughly N` phrasings the gate tolerates, over brittle exact numbers |
| R-2 | The absent `.claude/hooks/check-doc-drift.sh` in the docstring is load-bearing elsewhere | grep finds callers | either create the hook or correct the docstring; do not leave a phantom reference |
| R-3 | `ze-verify-changed` branch skips the doc gate if only one branch is wired | changed-mode run stays green on a broken ref | wire BOTH branches (:121-137 and :138-157) |

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| `stagesForMode("ze-verify")` | -> | stage list contains the doc gate | `TestStagesForModeIncludesDocGate` |
| `stagesForMode("ze-verify-changed")` | -> | changed-mode stage list contains the doc gate | `TestStagesForModeChangedIncludesDocGate` |
| README bare count vs actual | -> | `checkReadmeMD` flags exact/undercount | `TestCheckReadmeMDFlagsBareAndUndercount` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `stagesForMode` both branches | each list includes the doc-consistency gate (drift + corpus refs); asserted by unit test |
| AC-2 | `check_doc_links.py --md-only` on HEAD | exits 0; all 16 formerly-broken references resolve or carry `doc-links: ignore` |
| AC-3 | `doc_drift.go` on a bare `57 fuzz targets` and an undercount `10,000+ unit tests` | reports an issue for each (new exact + undercount checks) |
| AC-4 | headline counts (fuzz, unit, interop) across `README.md`, `docs/functional-tests.md`, `docs/comparison.md`, `docs/DESIGN.md` | mutually consistent and gate-clean; `test/stress/bgpgen.py` no longer cited anywhere |
| AC-5 | `make ze-verify` with a deliberately broken ref/count | fails (gate is live), demonstrating the dark gate is now enforced |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestStagesForModeIncludesDocGate` | `scripts/status/verify_run_test.go` | AC-1 (default branch) | |
| `TestStagesForModeChangedIncludesDocGate` | `scripts/status/verify_run_test.go` | AC-1 (changed branch) | |
| `TestCheckReadmeMDFlagsBareAndUndercount` | `scripts/docvalid/doc_drift_test.go` | AC-3 | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| README fuzz count | 0..actual | `== actual` (exact) or `N+ <= actual` | undercount `N < actual` | over-claim `N+ > actual` |
| broken references | 0 | 0 (gate green) | N/A | >= 1 (gate red) |

### Functional Tests
Test infrastructure only; no user-facing features. The change is a verification-gate wiring plus
doc/reference corrections; correctness is proven by the unit tests above plus `make ze-verify` going
red on an injected broken ref and green once fixed. No `.ci` scenario applies.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Wire `ze-doc-test` + the corpus-ref check into `stagesForMode` | create `.claude/hooks/check-doc-drift.sh` | A hook only fires for Claude; CI and other agents would still miss it. The gate belongs in the pipeline CI actually runs. |
| Add BOTH `ze-doc-test` and a corpus-ref stage (`ze-doc-links` or `ze-regen-check`) | wire `ze-doc-test` alone | `ze-doc-test` does not run `check_doc_links.py`; only `ze-doc-links`/`ze-regen-check` do (`Makefile:441,444`). Alone it misses all 16 refs. |
| Reconcile the phantom `check-doc-drift.sh` docstring by correcting `doc_drift.go:8` | create the hook to match the docstring | The doc gate now lives in `ze-verify`; a Claude-only hook is redundant. Fix the docstring to name the real caller (`make ze-doc-test`). |
| Prefer `N+`/`roughly N` phrasings when unifying counts | hardcode today's exact numbers | Soft claims the gate tolerates avoid re-drift as the tree grows (R-1). |

## Files to Modify
- `scripts/status/verify_run.go` - add the doc gate to both `stagesForMode` branches (feature wiring)
- `scripts/docvalid/doc_drift.go` - flag bare exact counts and gross undercounts; correct the `:8` docstring caller list
- `mk/inventory.mk` / `Makefile` - only if the gate composition needs a combined target for the pipeline
- `README.md` - unify fuzz/unit/interop headline counts
- `ai/INDEX.md`, `ai/LEARNED-INDEX.md`, `ai/rules/appliance-dep-bumps.md`, `ai/rules/deferral-tracking.md`, `ai/rules/project-knowledge.md`, `ai/rules/zefs-persistence.md`, `ai/skills/ze-hunt.md`, `ai/skills/ze-weekly-update.md` - fix stale references
- `docs/functional-tests.md`, `docs/comparison.md`, `docs/DESIGN.md` - reconcile the drifted counts
- user memory `reference_python_uv.md`, `MEMORY.md` - drop the deleted `test/stress/bgpgen.py` reference

## Files to Create
- `scripts/status/verify_run_test.go` (if absent) - stage-list assertions
- `scripts/docvalid/doc_drift_test.go` (if absent) - `checkReadmeMD` bare/undercount assertions

## Implementation Steps

### Implementation Phases
1. **Phase: Wiring (MANDATORY FIRST)** — add the doc gate to both `stagesForMode` branches; write `TestStagesForModeIncludesDocGate`/`...Changed...` first (fail), then wire (pass).
   - Verify: unit tests pass; `make ze-verify` now shells the doc gate.
2. **Phase: fix references** — resolve all 16 broken refs (repoint moved paths, delete dead ones, or `doc-links: ignore` intentional gaps per A-3); rerun `check_doc_links.py --md-only` to 0.
3. **Phase: tighten drift** — add exact-count + undercount regexes to `checkReadmeMD`; `TestCheckReadmeMDFlagsBareAndUndercount` fails then passes.
4. **Phase: unify counts** — reconcile fuzz/unit/interop headline numbers across the four docs to gate-clean values (prefer soft phrasings, R-1); drop `bgpgen.py` from repo + user memory.
5. **Full verification** — `make ze-verify` green; inject a broken ref locally to prove the gate goes red (AC-5), then revert.
6. **Complete spec** — audit tables, `plan/learned/NNN-<name>.md`, two-commit closure.

### Discovery-Update Obligation (`ai/rules/discovery-updates.md`)
- Source of truth: `scripts/status/verify_run.go` `stagesForMode` — the verify pipeline; the doc gate now discoverable there.
- Rule reinforced: `ai/rules/canonical-sources.md` — the discovery corpus (INDEX files) is the thing kept honest; a dark gate let it rot.
- No new make target family or runtime dependency is introduced (reuses `ze-doc-test`/`ze-doc-links`); no `ai/INDEX.md`/hook-mapping/doctor updates required beyond the reference fixes.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] `make ze-verify` runs the doc gate and is green
- [ ] Registration over hardcoding respected (gate discovered via `stagesForMode`, not an ad-hoc list)
- [ ] Discovery update done (references fixed, docstring corrected)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] Implementation Audit complete

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary cases (bare count, undercount, zero broken refs) present

## Notes
- Skeleton captured from the 2026-07-16 repository audit (HIGH dark-gate + folded MEDIUM count-drift). Verified: `stagesForMode` (`verify_run.go:112-158`) omits every doc gate in both branches; CI runs only `make ze-verify` (`.woodpecker/verify.yml:19`); `.claude/hooks/check-doc-drift.sh` absent though `doc_drift.go:8` names it; `check_doc_links.py --md-only` exits 1 with 16 broken refs.
- Verified count contradictions: fuzz `57` (`README.md:5`) / `50+` (`README.md:62`) / `54` (`docs/functional-tests.md:1617`) / `55+` (`docs/comparison.md:445`), with `mk/test-fuzz.mk` registering 61 and 69 distinct `func Fuzz` names; unit `12,800+` (`README.md:5`) vs `10,000+` (`README.md:59`); interop `35` (`README.md:5`) vs `101` (`docs/DESIGN.md:797`).
- Open question for the user: fix the drifted counts to exact live values, or switch to `N+`/`roughly N` soft phrasings the gate tolerates (R-1)? And should `.claude/hooks/check-doc-drift.sh` be created (to match the docstring) or the docstring corrected (recommended, since the gate now lives in `ze-verify`)?
