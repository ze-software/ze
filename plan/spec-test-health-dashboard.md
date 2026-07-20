# Spec: test-health-dashboard

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | plan/spec-rfc-gate-regression-ratchets.md (in-progress, coordination only) |
| Phase | - |
| Updated | 2026-07-20 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/rules/testing.md`, `ai/rules/fail-closed-guards.md`, `ai/rules/derive-not-hardcode.md`
4. `scripts/status/verify_run.go` (`stagesForMode`), `scripts/dev/rfc_requirements.py`, `scripts/dev/mutation_history.py`, `scripts/checks/port_defaults.go` (gate pattern), `../gh-pages/tools/page_registry.py`

## Task

Build a tool that renders the project's testing state as a single generated Markdown
file in this repo, published as HTML on the website, with a committed KPI time series
so evolution is visible.

The tool must answer "is our testing correct?", not "is our testing large?". Those are
different questions and the project currently publishes answers to the second while
implying the first. Two verified examples motivating the work:

| Published claim | Source | Reality |
|-----------------|--------|---------|
| "121,500+" unit tests, "570+" fuzz targets | `../gh-pages/data/site-facts.json`, produced by `count_go_tests` (`../gh-pages/tools/sitefacts.py:99`) walking `rglob("*_test.go")` (`:104`) with `SKIP_DIRS` (`:25`) that omits `gokrazy/` | 19,856 `func Test` and 72 `func Fuzz` are in-repo; 64,052 and 255 respectively come from `gokrazy/modcache`, which is third-party module cache |
| "0 MUST-level requirement(s) still owe work" across 165 enrolled RFCs | `ai/RFC-REQUIREMENTS.md:11` | Of 2,716 gated MUST requirements, 966 (35.6%) are proven by a positive+negative test pair; the remaining 1,750 are annotated `{not-applicable}`, `{gap}`, or `{single-polarity}`. 36 enrolled RFCs have zero test-proven requirements |

Neither is dishonest: the annotations are deliberate and `ai/rules/rfc-compliance.md` says a
green gate is bounded by what was extracted. The defect is that the summary line compresses
"proven", "not applicable", "known gap" and "half-tested" into one number that reads as
complete. The dashboard's job is to decompress exactly that.

### Success criteria

A developer who reads the page and sees green is justified in believing a regression would
be caught; and every place where that belief is *not* justified is visible on the page,
not merely derivable from it.

Operationally, the page must answer three questions, and every metric on it must belong
to one of them. A candidate belonging to none is volume and is excluded.

| # | Question | What it detects |
|---|----------|-----------------|
| Q1 | Sensitivity: if the code were wrong, would something go red? | Tests that cannot fail, oracles copied from the implementation, packages whose mutants all survive |
| Q2 | Intent coverage: are the things that matter checked, or only the happy path? | Requirements annotated away rather than proven, subsystems with no negative tests, techniques never back-filled to old code |
| Q3 | Integrity: when something goes red, does it stop the line? | Advisory gates permanently red, suites stale for hundreds of commits, tests no target runs, ratchets with accumulating slack |

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
- [ ] `ai/rules/testing.md` - the testing contract this page reports on
  → Constraint: "Back-Fill New Test Types" (`:39`) names the exact failure the age-bucketed
    adoption metric is designed to expose; the metric must be grep/registry-driven and
    enumerate every applicable site, not sample.
  → Constraint: `make ze-verify` is the final gate; a new stage must be cheap enough not to
    push it past its 4-10 minute budget.
- [ ] `ai/rules/fail-closed-guards.md` - every ratchet added here is a guard
  → Constraint: a miss, error, or empty result must not return the permissive value. An
    empty metric set fails rather than reporting clean, matching `rfc_requirements.py`'s
    treatment of an empty `enrolled.txt`.
  → Constraint: drive each guard's test from its entry point, not the helper alone.
- [ ] `ai/rules/derive-not-hardcode.md` - the ledger precedent
  → Decision: metrics are derived from the tree on every run; no hand-maintained counts.
    The only hand-committed numbers are ratchet baselines, which are floors, not facts.
- [ ] `ai/rules/canonical-sources.md` - generated files
  → Constraint: never edit a generated file; the generated Markdown carries a
    GENERATED banner and a staleness check, matching `ai/RFC-REQUIREMENTS.md:3`.
- [ ] `ai/rules/discovery-updates.md` - new gate and tool must be discoverable
  → Constraint: a new make target, gate and test format requires updates to `ai/INDEX.md`,
    `ai/rules/testing.md`, and operator docs in the same change.
- [ ] `docs/contributing/gh-pages.md` - website generation contract
  → Constraint: do not add a parallel hand-written list in the website; add a structured
    field to the data source first, then render from that data.

### RFC Summaries (MUST for protocol work)
- N/A. This spec adds verification and reporting machinery. It implements, changes, and
  newly proves no `rfc/short/` requirement.

**Key insights:**
- The repo already owns every input this needs. Nothing new must be measured; the work is
  aggregation, honest presentation, and ratcheting.
- `scripts/dev/mutation_history.py` already established the committed time-series
  convention, and its own docstring (`:4-7`) records why: gomu's report is gitignored, so
  "mutation coverage had no trend and no baseline". The KPI history follows that format
  rather than inventing one.
- `stagesForMode` (`scripts/status/verify_run.go:179`) is the single source of truth for what
  runs anywhere, in CI or locally. Its own comment (`:169-178`) records that a duplicate
  Makefile copy drifted and was deleted; a gate absent from this list runs nowhere.
- The website renders main-repo Markdown listed in `../gh-pages/tools/page_registry.py`
  `DOCS_MANIFEST` through `render-doc.py`, whose Python-Markdown configuration
  (`render-doc.py:510`, extensions `tables, fenced_code, sane_lists, toc`) passes raw
  block-level HTML through untouched. Inline SVG in the Markdown therefore renders as a
  chart with no chart library and no JavaScript, satisfying the site's stated rule that
  page content must be meaningful without JS.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `scripts/status/verify_run.go` - the verify runner. `stagesForMode` (`:179`) lists 21
  stages for `ze-verify` (`:210-232`) and 18 for `ze-verify-changed` (`:189-208`);
  `defaultVerifyConfig` (`:149`) builds the run; the runner writes
  `tmp/ze-verify-failures.json`, `tmp/ze-verify.log`, `tmp/ze-verify-failures.log`,
  `tmp/ze-verify.status` and `tmp/verify/NN-<stage>.log`.
  → Constraint: the two stage lists are hand-duplicated on purpose and pinned by
    `TestStagesForModeMatchesGolden` (`scripts/status/verify_run_test.go:532`) against
    ~~goldens in `scripts/status/testdata/`~~ hand-maintained literal slices
    `goldenStagesZeVerify` (`verify_run_test.go:480`) and `goldenStagesZeVerifyChanged`
    (`:504`). Corrected 2026-07-20: `scripts/status/testdata/` holds failure-classification
    fixtures, not stage goldens. The comment at `:470-474` states the goldens are
    deliberately NOT derived from `stagesForMode`, so a new gate must be added to BOTH
    branches and BOTH literal slices in the same change. Comparison is ordered
    (`:476-478`), so insertion position is load-bearing.
  → Constraint: `TestStagesForModeBranchesAgree` (`:586`) is a second, independent
    divergence check; a stage added to one branch only fails it even if both goldens were
    edited consistently.
  → Constraint: the runner refuses to run under `make -n` (`:129`) so a dry run cannot forge
    a FRESH status. Any new gate inherits this and needs no separate guard.
  → Constraint: `ze-regen-check-readonly` is already position 14 in both lists, so hanging
    the staleness check off it needs no new stage; only the ratchet needs one.
- [ ] `scripts/dev/mutation_history.py` - appends one NDJSON row per package per run to
  `test/mutation/history.ndjson`; schema `{ts, sha, package, mutants, killed, score}`
  (`:76-83`); called by `mk/test-mutation.mk` after each gomu run.
  → Constraint: it is explicitly advisory and returns 0 when the report is unreadable
    (`:43`) or empty (`:56`). A missing mutation run therefore leaves NO row and NO error,
    which is exactly why an absent series must render `unknown` rather than green or zero.
  → Constraint: it stamps a wall-clock `ts` (`:67`). The KPI history may do the same because
    it is append-only, but the generated Markdown may not, or the staleness check flaps.
- [ ] `scripts/dev/rfc_requirements.py` - the RFC gate, stage 3 of both verify modes. Writes
  `ai/RFC-REQUIREMENTS.md`; `SUMMARY_DIR`/`ENROLLED_FILE`/`STATUS_FILE`/`LEDGER_FILE`/
  `AUDIT_DIR`/`TEST_ROOTS` at `:50-59`; checks include `check_enrolment`,
  `check_coverage_ratchet`, `check_status_agreement`, `check_ledger_fresh`.
  → Constraint: `check_coverage_ratchet` already ratchets per-RFC coverage against a git
    baseline, and `plan/spec-rfc-gate-regression-ratchets.md` G2 is adding a per-requirement
    proof ratchet. This spec must NOT add a third RFC ratchet; it displays these and
    ratchets only metrics that nothing else ratchets.
- [ ] `ai/RFC-REQUIREMENTS.md` - generated ledger, 5286 lines. Header at `:5` and `:11`;
  per-RFC table with columns `RFC | Gated | Both | One polarity | Annotated | No test |
  Outstanding | State` starting `:14`.
  → Constraint: this table is the parse target for the headline metric. It is generated, so
    the parser must fail loudly if the column set changes rather than silently reporting zero.
- [ ] `scripts/inventory/inventory.go` - `--json` emits `test-counts` (30 suites),
  `total-tests` (1443), `rpc-list` with a per-RPC `covered` flag (600 of 868 covered),
  `package-stats`. Derived from the real plugin registry, not regex.
- [ ] `scripts/checks/port_defaults.go` and siblings - the eight existing source gates. Each
  is `//go:build ignore`, run via `go run`, and owns a `--selftest` that proves the detector
  fires on known-bad fixtures before it judges the live tree.
  → Constraint: a new AST gate follows this shape exactly, including `--selftest` and `--json`.
- [ ] `Makefile:501` - `ze-regen-check-readonly` is a prerequisite list of eight staleness
  targets; it is stage 14 of `ze-verify`.
- [ ] `test/.ci-sleep-baseline` (value 125) and `scripts/dev/core_import_baseline.txt`,
  `scripts/dev/tier_migration_baseline.txt` - the existing ratchet-baseline convention: a
  committed floor file, lowered in the same change that improves the number.
- [ ] `scripts/dev/loc_activity.py` - the closest existing precedent for a standalone
  stdlib-Python HTML/JSON generator in this repo (1275 lines, no third-party imports).
- [ ] `../gh-pages/tools/page_registry.py` - `DOCS_MANIFEST` (`:28`) maps main-repo
  `docs/*.md` to site destinations with a category colour; `QUALITY_PAGES` (`:176`) holds the
  six hand-written quality pages; `page_root_for_dest` (`:227`) derives relative depth from
  the destination.
- [ ] `../gh-pages/tools/build.py` - orchestrator with a 33-entry `STEPS` list, `--only`,
  drift-check hooks that call `sitelib.warn()`, and a non-zero exit when any warning fired.
- [ ] `../gh-pages/tools/sitefacts.py` - produces `data/site-facts.json`. `TEST_FUNC_RE`
  (`:23`), `SKIP_DIRS` (`:25`, omits `gokrazy`), `count_go_tests` (`:99`) whose `rglob` walk
  (`:104`) is the producer of the inflated figure.
  → Constraint: this is the source of the inflated public counts. It must be corrected in
    the same change, or the site will publish two contradictory numbers.

**Behavior to preserve:**
- `stagesForMode` remains the single source of truth; the new gate is added to it, not
  bolted onto a Makefile target that CI does not reach.
- The existing `.ci` sleep ratchet, RFC coverage ratchet, and alloc-gate ceilings keep
  their current owners. The dashboard reads and displays them; it does not reimplement them.
- `ai/RFC-REQUIREMENTS.md` stays the generated ledger; the dashboard consumes it and never
  rewrites it.
- `test/mutation/history.ndjson` keeps its schema and its advisory appender.

**Behavior to change:**
- `count_go_tests` (`../gh-pages/tools/sitefacts.py:99`) gains the skip directories that make
  its counts wrong, so the published headline stops counting vendored dependency tests.
  Requested implicitly by the task ("what a developer would want to see ... to decide if the
  testing is correct"); a knowingly inflated public count contradicts that goal.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- `make ze-test-health` (write mode) and `make ze-test-health-check` (gate mode), the latter
  reached from `stagesForMode` on every `ze-verify` and `ze-verify-changed` run.
- Inputs are the committed working tree plus committed artifacts. No live test run is
  required to produce the page.

### Transformation Path
1. **Collect (Go).** `scripts/checks/test_sensitivity.go` walks the Go AST of every in-repo
   `_test.go` file and emits, as JSON, the assert-nothing test list and the tag-orphaned
   test-file list.
2. **Collect (Python).** `scripts/dev/test_health.py` runs that gate's JSON mode, runs
   `scripts/inventory/inventory.go --json`, parses `ai/RFC-REQUIREMENTS.md`'s coverage table
   and the annotation kinds in `rfc/short/*.md`, reads `test/mutation/history.ndjson`,
   `test/.ci-sleep-baseline`, `plan/known-failures.md`, and asks git for each package's
   first-commit date for the age buckets.
3. **Reduce.** Metrics are assembled into one structured record. Every ratio carries its
   numerator and denominator; no ratio is stored alone.
4. **Emit.** Three committed artifacts: the Markdown page, the structured `latest.json`,
   and (in `--record` mode only) one appended row on `history.ndjson`.
5. **Publish.** The website's new renderer reads `latest.json` and `history.ndjson` from
   `../main` and renders the site page. It performs no computation.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Go AST gate ↔ Python aggregator | Subprocess, JSON on stdout, non-zero exit propagates | [ ] |
| Aggregator ↔ verify runner | `make ze-test-health-check` listed in `stagesForMode`, both branches | [ ] |
| Main repo ↔ website | Website reads committed `test/health/*.json` from `../main`; no logic crosses | [ ] |
| Committed state ↔ generated page | Page content is a pure function of committed state; no wall-clock timestamp | [ ] |

### Integration Points
- `stagesForMode` (`scripts/status/verify_run.go:179`) plus its two goldens in
  `scripts/status/testdata/` - the new gate stage.
- `Makefile:501` `ze-regen-check-readonly` - the staleness check for the generated Markdown.
- `TestPythonUnitTests` (`scripts/dev/python_tests_test.go`) - picks up the new
  `scripts/dev/test_health_test.py` automatically by glob; no make target needed.
- `scripts/checks/checks_test.go` - the existing per-gate selftest harness.
- `../gh-pages/tools/build.py` `STEPS` and `../gh-pages/data/page-links.json` - publication.

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies) — N/A, offline tooling
- [ ] Registration over hardcoding — the gate joins the existing `scripts/checks` gate set
      and the existing `stagesForMode` list; no new per-feature dispatch is introduced

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Raw block-level SVG survives the site's Markdown renderer | `../gh-pages/tools/render-doc.py:510` extension list | Charts must move to the gh-pages renderer only, and the in-repo Markdown carries tables alone | Rendered a fixture through the same extension set; output preserved the `<svg>` verbatim | confirmed |
| A-2 | The generated Markdown is a pure function of committed state, so a staleness check is stable | Inputs are committed files plus `git log`; no wall-clock value is embedded | `--check` flaps and blocks unrelated commits; would have to leave `ze-regen-check-readonly` | `test_deterministic_output`, plus two live `--write` runs diffed byte-for-byte on a stable tree | confirmed |
| A-3 | `ai/RFC-REQUIREMENTS.md`'s coverage-table columns are stable enough to parse | Generated by `rfc_requirements.py`; `render_ledger` owns the format | Parser silently reports zero and the headline metric becomes a lie | `test_rfc_ledger_parse` (4 cases: parse, header drift, missing file, zero rows) all fail closed | confirmed |
| A-4 | Assert-nothing detection has a low enough false-positive rate to ratchet | Detector follows same-package helpers one level and honours an explicit escape comment | Ratchet blocks legitimate smoke tests; would ship as report-only | Reviewed by hand; 3 false-positive classes found and fixed (benchmarks, fuzz targets, compile-time interface assertions), 283 -> 136. Two survivors verified true positives by reading them | confirmed |
| A-5 | Adding one gate stage keeps `ze-verify` inside its 4-10 minute budget | Gate is an AST walk plus one `inventory.go` run, both already done elsewhere in under a minute | Gate moves to `ze-verify` only, not `ze-verify-changed` | `ze-test-sensitivity-check` runs in ~20s (AST walk over 2568 files), well inside budget | confirmed |
| A-6 | `gokrazy/` is the only unignored tree inflating the public counts | Measured: 64,052 of 83,909 non-vendor `func Test` are under `gokrazy/`; `third_party/`, `cache/`, `build/`, `iso/` hold zero | The corrected public count is still wrong, just less so | Superseded: `count_go_tests` now READS `test/health/latest.json` instead of recounting, so divergence is impossible by construction rather than asserted | confirmed (design changed) |
| A-7 | Correcting the public test counts is wanted, not merely noticed | Task asks for what a developer needs "to decide if the testing is correct"; a knowingly inflated count defeats that | Leave `sitefacts.py` alone and note the discrepancy on the page instead | User confirmation at the spec gate | confirmed (2026-07-20, "Fix it, show honest numbers") |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The page becomes a green wall nobody reads | Every tile green for weeks while known problems persist | Exceptions-first rendering: healthy rows collapse to one line; every tile states the action its degradation implies; a rotating spotlight names the single worst package each run |
| R-2 | The dashboard's own collector breaks and the page shows stale green | `latest.json` HEAD sha older than the tree | Freshness is rendered first and "unknown" is its own state, never green; the staleness gate makes a stale page fail `ze-verify` |
| R-3 | Ratchets set from a blind baseline block unrelated work | First unrelated commit fails the new gate | Baselines are committed only after a hand-reviewed full-tree run; the gate names the exact offending files and the command to refresh; `plan/known-failures.md` explicitly may not absorb structural gates, so a red must be fixed at source |
| R-4 | A ratio improves because its denominator shrank | Numerator and denominator trends diverge | No ratio is stored or drawn without both components; a falling denominator renders amber regardless of the ratio's direction |
| R-5 | Metric duplication with the in-flight RFC ratchet spec | Both specs add a proof ratchet | This spec adds no RFC ratchet; it displays `rfc_requirements.py`'s numbers and ratchets only assert-nothing and tag-orphan counts |
| R-6 | Mutation trend is too sparse to plot honestly (45 rows, irregular, advisory appender) | Sparkline implies a trend from three points | Sample count `n` is printed beside every sampled statistic; a series under a minimum sample count renders as "insufficient data", not as a flat line |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `make ze-verify` / `ze-verify-changed` | → | new `ze-test-health-check` stage in `stagesForMode` | `TestStagesForModeMatchesGolden` (updated goldens, both modes) |
| `make ze-test-health` | → | `scripts/dev/test_health.py --write` | `TestTestHealthWritesArtifacts` |
| `make ze-test-health-check` on a doctored tree | → | staleness + ratchet failure path | `TestTestHealthCheckFailsOnStaleDoc`, `TestTestSensitivityRatchetFailsOnRegression` |
| `go run scripts/checks/test_sensitivity.go --selftest` | → | AST detectors | `TestTestSensitivitySelftest` |
| `make ze-regen-check-readonly` | → | `ze-test-health-check` prerequisite | `TestRegenCheckIncludesTestHealth` |
| Website build `--only test-health` | → | `../gh-pages/tools/render-test-health.py` | `test_render_test_health.py` (gh-pages selftest) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `make ze-test-health` on a clean tree | Writes `docs/features/test-health.md`, `test/health/latest.json`; exit 0; no wall-clock timestamp in either |
| AC-2 | `make ze-test-health` run twice with no tree change | Byte-identical outputs both times. ~~and the committed report is byte-gated~~ superseded by D-5: `--check` gates the STRUCTURAL facts; volume counters are published ungated |
| AC-3 | Generated Markdown | Headline is RFC proof density expressed as pairs-proven over gated, with the `{not-applicable}` / `{gap}` / `{single-polarity}` split shown separately, never merged |
| AC-4 | Generated Markdown | Reports in-repo test counts only; states the counting boundary explicitly; the numbers agree with `../gh-pages/data/site-facts.json` after the `count_go_tests` correction |
| AC-5 | Any metric expressed as a ratio | Numerator and denominator both present in `latest.json` and both rendered on the page |
| AC-6 | A metric whose input artifact is absent (no mutation history, no verify status) | Renders as `unknown` with the reason, sorted above healthy rows; never renders as green or as zero |
| AC-7 | A Go test function with no reachable failure call and no escape comment | Listed by `test_sensitivity.go --json` under assert-nothing |
| AC-8 | A test function carrying the documented escape comment | Not listed |
| AC-9 | A `_test.go` file whose build tags no `mk/*.mk` target enables | Listed under tag-orphaned |
| AC-10 | Assert-nothing or tag-orphan count above its committed baseline | `make ze-test-health-check` exits non-zero, naming each offending file and the refresh command |
| AC-11 | Counts at or below baseline | Gate exits 0 |
| AC-12 | ~~Committed page differs from freshly generated~~ A STRUCTURAL fact (orphan list, unproven RFCs, any metric status) differs from the tree | `make ze-test-health-check` exits non-zero and names which fact moved. Pure volume drift does NOT fail it (D-5) |
| AC-13 | `make ze-verify-list` | Shows the new stage in both verify modes |
| AC-14 | `make ze-test-health --record` after a mutation or verify run | Appends exactly one row to `test/health/history.ndjson`, schema-compatible with the mutation-history convention |
| AC-15 | `history.ndjson` with fewer than the minimum sample count for a series | That series renders "insufficient data", not a trend line |
| AC-16 | Website build `--only test-health` | Produces the site page from `latest.json` and `history.ndjson` with no metric computation of its own |
| AC-17 | Empty or unparseable `ai/RFC-REQUIREMENTS.md` coverage table | Generator fails loudly; does not emit a page reporting zero |
| AC-18 | Age-bucketed adoption metric | Reports fuzz-target, `.ci` and RFC-tag presence per package bucketed by package first-commit date, so forward-only adoption is visible as a step |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Reads `docs/features/test-health.md` in the repo to judge test state | committed Markdown, self-contained, tables plus inline SVG | `TestTestHealthDocHasRequiredSections` |
| 2 | Reads the same page on the website | `latest.json` + `history.ndjson` → `render-test-health.py` → site HTML | `test_render_test_health.py` |
| 3 | Runs `make ze-verify` and is blocked after adding an assert-nothing test | new gate stage → `test_sensitivity.go --check` → non-zero → verify failure index | `TestTestSensitivityRatchetFailsOnRegression` |
| 4 | Changes code without regenerating, then commits | `ze-regen-check-readonly` → `ze-test-health-check` staleness → non-zero | `TestTestHealthCheckFailsOnStaleDoc` |
| 5 | Runs mutation testing, then records the KPI row | `ze-mutation-*` → `history.ndjson` (existing) → `--record` → health history row | `test_record_appends_one_row` |
| 6 | Looks at the trend after several weeks | committed `history.ndjson` → sparkline with `n` printed | `test_sparkline_sample_count` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestTestSensitivitySelftest` | `scripts/checks/test_sensitivity_test.go` | Detectors fire on known-bad fixtures and stay silent on known-good | |
| `TestAssertNothingEscapeComment` | `scripts/checks/test_sensitivity_test.go` | AC-8 | |
| `TestTagOrphanDetection` | `scripts/checks/test_sensitivity_test.go` | AC-9, against the real `mk/*.mk` tag set | |
| `TestTestSensitivityRatchetFailsOnRegression` | `scripts/checks/test_sensitivity_test.go` | AC-10, AC-11, driven from the gate entry point | |
| `TestTestSensitivityFailsClosedOnEmptyScan` | `scripts/checks/test_sensitivity_test.go` | Empty result set is an error, not a pass | |
| `TestTestHealthWritesArtifacts` | `scripts/dev/test_health_test.go` | AC-1, from the make entry point | |
| `TestTestHealthCheckFailsOnStaleDoc` | `scripts/dev/test_health_test.go` | AC-12 | |
| `TestTestHealthDocHasRequiredSections` | `scripts/dev/test_health_test.go` | Story 1 | |
| `TestRegenCheckIncludesTestHealth` | `scripts/dev/test_health_test.go` | Wiring into `ze-regen-check-readonly` | |
| `test_rfc_ledger_parse` | `scripts/dev/test_health_test.py` | AC-3, AC-17: exact header contract, fail-closed on drift | |
| `test_ratio_carries_denominator` | `scripts/dev/test_health_test.py` | AC-5 | |
| `test_missing_artifact_is_unknown` | `scripts/dev/test_health_test.py` | AC-6 | |
| `test_deterministic_output` | `scripts/dev/test_health_test.py` | AC-2 | |
| `test_record_appends_one_row` | `scripts/dev/test_health_test.py` | AC-14 | |
| `test_sparkline_sample_count` | `scripts/dev/test_health_test.py` | AC-15 | |
| `test_age_buckets` | `scripts/dev/test_health_test.py` | AC-18 | |
| `test_site_facts_matches_test_health` | `scripts/dev/test_health_test.py` | AC-4, A-6 | |
| `TestStagesForModeMatchesGolden` | `scripts/status/verify_run_test.go` | AC-13, both modes (existing test, updated goldens) | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| assert-nothing baseline | 0..count | baseline | N/A (lower always allowed, prompts refresh) | baseline+1 fails |
| tag-orphan baseline | 0..count | baseline | N/A | baseline+1 fails |
| history minimum sample count | 1..n | threshold | threshold-1 renders "insufficient data" | N/A |
| RFC gated total | must be > 0 | 1 | 0 fails closed (AC-17) | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `TestTestHealthGateOnDoctoredTree` | `scripts/dev/test_health_test.go` | Fixture tree with an added assert-nothing test fails the gate with a named file | |
| `TestTestHealthStaleDocOnDoctoredTree` | `scripts/dev/test_health_test.go` | Edited generated doc fails the staleness check | |

Rationale for not using `.ci`: `ai/rules/testing.md:61` reserves `test/parse/` for config-parse
cases and directs pure-logic, reactor-free code to Go tests. This tool never starts the daemon,
so its end-to-end scenarios are Go tests that shell out to the entry points, matching the
`scripts/dev/migrate_module_test.go` precedent named at `ai/rules/testing.md:266`.

### Interop Tests (MANDATORY for protocol features)
N/A. No wire protocol behavior is added or changed.

### Future (if deferring any tests)
- None. Every AC has a test in this plan.

## Files to Modify
- `scripts/status/verify_run.go` - add `ze-test-health-check` to both branches of `stagesForMode` (`:189-208`, `:210-232`)
- `scripts/status/testdata/` - both stage goldens
- `Makefile` - `ze-test-health`, `ze-test-health-check`; add the check to `ze-regen-check-readonly` (`:501`); help text
- `ai/rules/testing.md` - document the new gate, target, and what the page reports
- `ai/INDEX.md` - tool navigation entry
- `docs/features.md` - link the new page
- `../gh-pages/tools/sitefacts.py` - correct the skip set used by `count_go_tests` (`:99`)
- `../gh-pages/tools/build.py` - new `test-health` step
- `../gh-pages/tools/page_registry.py` - destination for the new page
- `../gh-pages/data/page-links.json`, `../gh-pages/data/nav.json` - navigation and related links
- `../gh-pages/quality/quality.md` - cross-link from the quality hub to the state page

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | No | Offline developer tooling; no runtime config surface |
| YANG validation constraints | No | — |
| YANG custom validators | No | — |
| CLI commands/flags | No | Make targets only, matching the other eight source gates |
| CLI grammar | No | — |
| Editor autocomplete | No | — |
| Functional test for new RPC/API | No | No RPC added |
| Pipe completeness | No | Not a `ze` command |
| Env var registration | No | — |
| Doctor check for runtime dependencies | No | No runtime dependency; tooling reads committed files |
| Prometheus counters/metrics | No | No runtime observable state |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` links the new page |
| 2 | Config syntax changed? | No | No config surface added |
| 3 | CLI command added/changed? | No | Make targets only |
| 4 | API/RPC added/changed? | No | — |
| 5 | Plugin added/changed? | No | — |
| 6 | Has a user guide page? | Yes | `docs/features/test-health.md` is itself the page |
| 7 | Wire format changed? | No | — |
| 8 | Plugin SDK/protocol changed? | No | — |
| 9 | RFC behavior implemented, changed, or newly proven? | No | Reporting only; no `rfc/short/` requirement changes state |
| 10 | Test infrastructure changed? | Yes | `ai/rules/testing.md`, `docs/architecture/testing/` |
| 11 | Affects daemon comparison? | No | — |
| 12 | Internal architecture changed? | No | — |
| 13 | Route metadata keys added/changed? | No | — |
| 14 | Prometheus counters added/changed? | No | — |
| 15 | Registered plugin, event type, command, capability or runtime inventory changed? | No | — |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Grep `docs/` for anchors on `verify_run.go` and the verify targets; update stage-count claims |
| 17 | Existing docs show examples for this area? | Yes | `../gh-pages/quality/*.md` describes the test layers; add the cross-link, verify claims still hold |

## Files to Create
- `scripts/checks/test_sensitivity.go` - AST gate: assert-nothing tests, tag-orphaned test files; `--selftest`, `--json`, `--check`
- `scripts/checks/test_sensitivity_test.go` - gate tests
- `scripts/dev/test_health.py` - aggregator and Markdown renderer; `--write`, `--check`, `--record`, `--json`, `--selftest`
- `scripts/dev/test_health_test.py` - unittest, auto-discovered by `TestPythonUnitTests`
- `scripts/dev/test_health_test.go` - entry-point tests that shell out to the make targets
- `docs/features/test-health.md` - the generated single Markdown page (committed)
- `test/health/latest.json` - structured metrics (committed)
- `test/health/history.ndjson` - append-only KPI series (committed)
- `test/health/sensitivity-baseline.json` - ratchet floors (committed)
- `test/health/README.md` - what each artifact is and how to refresh it
- `../gh-pages/tools/render-test-health.py` - site renderer, presentation only
- `../gh-pages/tools/test_render_test_health.py` - renderer selftest

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 6. Critical review | Critical Review Checklist below |
| 7. Fix issues | Every issue from critical review |
| 8. Re-verify | Re-run stage 5 |
| 9. Repeat 6-8 | Until clean |
| 10. Deliverables review | Deliverables Checklist below |
| 11. Security review | Security Review Checklist below |
| 12. Documentation review | Documentation Update Checklist above |
| 13. `/ze-review` gate | Review Gate section |
| 14. Present summary + close | Executive Summary Report; two-commit closure |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** — add the `ze-test-health-check` stage to both branches
   of `stagesForMode`, add both make targets as failing stubs, add the check to
   `ze-regen-check-readonly`.
   - Tests: `TestStagesForModeMatchesGolden` (updated goldens), `TestRegenCheckIncludesTestHealth`
   - Files: `scripts/status/verify_run.go`, `scripts/status/testdata/`, `Makefile`
   - Verify: `make ze-verify-list` shows the stage in both modes; the stage fails because the
     tool is a stub
2. **Phase: Sensitivity gate** — the Go AST detectors and their ratchet.
   - Tests: `TestTestSensitivitySelftest`, `TestAssertNothingEscapeComment`,
     `TestTagOrphanDetection`, `TestTestSensitivityRatchetFailsOnRegression`,
     `TestTestSensitivityFailsClosedOnEmptyScan`
   - Files: `scripts/checks/test_sensitivity.go`, `scripts/checks/test_sensitivity_test.go`
   - Verify: `--selftest` passes before any live-tree judgement; full-tree run reviewed by
     hand; baseline committed only after that review (A-4)
3. **Phase: Aggregator and page** — collectors, the reduce step, Markdown and JSON emission.
   - Tests: `test_rfc_ledger_parse`, `test_ratio_carries_denominator`,
     `test_missing_artifact_is_unknown`, `test_deterministic_output`, `test_age_buckets`,
     `TestTestHealthWritesArtifacts`, `TestTestHealthDocHasRequiredSections`
   - Files: `scripts/dev/test_health.py`, `scripts/dev/test_health_test.py`,
     `scripts/dev/test_health_test.go`, `docs/features/test-health.md`, `test/health/*`
   - Verify: two consecutive runs are byte-identical; absent artifacts render `unknown`
4. **Phase: KPI history and trends** — `--record`, sparkline rendering with sample counts.
   - Tests: `test_record_appends_one_row`, `test_sparkline_sample_count`
   - Files: `scripts/dev/test_health.py`, `test/health/history.ndjson`
   - Verify: minimum-sample threshold suppresses a misleading trend line
5. **Phase: Publication** — website renderer, navigation, and the `count_go_tests` correction.
   - Tests: `test_render_test_health.py`, `test_site_facts_matches_test_health`
   - Files: `../gh-pages/tools/render-test-health.py`, `build.py`, `page_registry.py`,
     `data/page-links.json`, `data/nav.json`, `sitefacts.py`, `quality/quality.md`
   - Verify: `tools/build.py --only test-health,links` succeeds with no `sitelib.warn()`
6. **Phase: Documentation and discovery** — `ai/rules/testing.md`, `ai/INDEX.md`,
   `docs/features.md`, `test/health/README.md`, source anchors.
7. **Full verification** → `make ze-verify`
8. **Complete spec** → audit tables, learned summary, two-commit closure

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Feature completeness | Every user story path works end to end; the page renders in both the repo and the site |
| Fail-closed | Every collector returns an error, not zero, on missing or unparseable input (AC-6, AC-17). Each guard is driven from its entry point, not its helper |
| No ratio without denominator | Grep the `latest.json` emission for any percentage lacking its pair (AC-5) |
| Determinism | No wall-clock value, no map-iteration order, no absolute path in any emitted artifact (AC-2) |
| No duplicated ratchet | The RFC coverage and per-requirement ratchets stay owned by `rfc_requirements.py`; the `.ci` sleep ratchet stays owned by `verify_wiring_docs.py` (R-5) |
| Honest counting | Test counts state their boundary and exclude the module cache; site and repo agree (AC-4) |
| Green-wall resistance | Healthy rows collapse; every tile names its action; `unknown` sorts above green (R-1, R-2) |
| Registration over hardcoding | The gate joins the existing `scripts/checks` set and `stagesForMode`; no bespoke dispatch |
| Rule: discovery-updates | New target, gate and artifacts are reachable from `ai/INDEX.md` and `ai/rules/testing.md` |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Generated Markdown page | `ls -la docs/features/test-health.md`; `make ze-test-health-check` exits 0 |
| Structured metrics | `ls -la test/health/latest.json`; parses as JSON with every ratio paired |
| KPI history | `wc -l test/health/history.ndjson`; each line parses |
| Ratchet baseline | `ls -la test/health/sensitivity-baseline.json` |
| Verify stage present | `make ze-verify-list` output contains the stage in both modes |
| Gate detects regression | Doctored fixture run exits non-zero and names the file |
| Site page | `tools/build.py --only test-health` produces the destination and its `index.md` sibling |
| Public counts corrected | `../gh-pages/data/site-facts.json` unit count equals the in-repo count |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Ledger and NDJSON parsing must reject malformed input rather than coerce; no `eval`-style parsing |
| Subprocess handling | Gate invocations use argument lists, never a shell string; non-zero exit propagates |
| Path handling | All paths resolved under the repo root; no absolute path leaks into a committed artifact |
| Resource exhaustion | AST walk and git queries bounded to in-repo trees; module cache and vendor excluded by construction |
| Information disclosure | The published page names test files and packages only; no environment, no host paths, no credentials |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior → RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural → DESIGN phase |
| Staleness check flaps | A-2 is broken: find the non-deterministic input, remove it; do not weaken the check |
| Ratchet blocks unrelated work | A-4 is broken: reduce to report-only and re-review the detector, do not raise the baseline silently |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| A `str.replace`-based edit had applied, so the fix could be recorded as done | Two edits silently no-opped because a formatter had reflowed their target text, and I wrote "fixed" into the Review Gate without re-reading the file | Run 3's independent audit grepped for `PYTHONHASHSEED` and for `do_write` in the test file; both returned nothing | **The most serious process failure in this spec.** A review record that claims fixes which do not exist is worse than no record: it stops the next reviewer looking. Rule going forward: after any scripted edit, grep for the new text before recording the fix. Two Review Gate rows corrected in place rather than silently rewritten |
| The `stagesForMode` goldens live in `scripts/status/testdata/` | They are hand-maintained literal slices in `scripts/status/verify_run_test.go:480` and `:504`; `testdata/` holds failure-classification log fixtures | Listed `testdata/` during Phase 1 wiring, found only `*.log` fixtures, then grepped for `MatchesGolden` | Spec citation corrected in Current Behavior; no design change. Reinforces `ai/rules/no-fabrication.md`: the claim came from a survey summary, not from reading the producer |
| One target (`ze-test-health-check`) could carry both the ratchet and the doc-staleness check | Wiring it into both `stagesForMode` and `ze-regen-check-readonly`, as the spec first said, would run it twice per verify | Noticed while writing the Phase 1 edit | Split into two targets (see Deviations); strictly better because each lands in the failure index with its own rerun command |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

- The repo already measures nearly everything this page needs. What was missing was not
  measurement but aggregation and honest framing: three separate green summaries
  (`site-facts.json` counts, the RFC "0 outstanding" line, an all-green verify) each true in
  isolation and collectively misleading about whether a regression would be caught.
- Making the generated page a pure function of committed state is what allows it to be gated
  the way every other generated file here is gated. That single constraint decides the whole
  architecture: run-dependent metrics cannot go straight onto the page, so they are appended
  to a committed history and rendered from there.
- `mutation_history.py` is advisory by design and silently records nothing when the report is
  missing. Any consumer that treats "no rows" as "score zero" or as "healthy" inherits a
  silent sensor failure, which is why `unknown` must be a first-class rendered state.

## Core Insight

A test dashboard that reports on the test suite is a scoreboard. A test dashboard that is
adversarial toward the test suite is an instrument. In a project where an AI writes most of
the tests and no human can review 2,565 test files, only the adversarial version is worth
building, because the characteristic failure of AI-written tests is not absence but
inertness, and inertness is invisible to every volume metric.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Headline is RFC proof density, not coverage or test count | Line coverage; total tests; enrolment count | Coverage measures execution, not observation, and the `.ci` suites drive the binary so unit coverage under-reports them in a biased way. Proof density is already computed, already ratcheted upstream, and directly answers Q2 |
| Generated page is a pure function of committed state | Embed live run results | Enables the staleness gate; removes the sensor-rot failure mode where a broken collector shows old green |
| Detectors in Go under `scripts/checks/`, aggregation in Python under `scripts/dev/` | All-Python; all-Go | Matches both existing conventions: eight Go AST gates with `--selftest`, and Python for report generation (`loc_activity.py`, `rfc_requirements.py`). Go AST parsing is materially more robust than regex for AC-7 |
| Ratchet only assert-nothing and tag-orphan counts | Ratchet proof density too | `rfc_requirements.py` already ratchets coverage and `spec-rfc-gate-regression-ratchets.md` G2 is adding per-requirement proof; a third would duplicate and conflict (R-5) |
| Charts as inline SVG in Markdown | Chart library; CSS-grid like `render-activity.py`; images | Site policy requires content meaningful without JS; SVG survives the Markdown renderer verbatim (A-1); `internal/perf/report/html.go` already sets the in-house inline-SVG precedent |
| Correct `count_go_tests` in the same change | Leave it, note the discrepancy on the page | Publishing two contradictory test counts on one site is worse than either alone |

## Known Limitations
- Mutation kill rate is only as current as the last recorded mutation run; with 45 rows over
  irregular intervals, early trend lines will show "insufficient data" by design.
- Assert-nothing detection follows same-package helpers one level only. Deeper indirection
  and table-driven assertions routed through a shared runner can evade it.
- Age-bucketed adoption uses each package's first-commit date, which a directory rename
  resets. Renamed packages will read as younger than they are.
- The page reports on the tests that exist. It cannot report on behavior nobody thought to
  test, which is the failure mode that only the RFC extraction pass addresses.
- Per-suite staleness depends on run records that do not persist today; until a run history
  accumulates, those rows render `unknown` rather than green.

## RFC Documentation
N/A. No protocol behavior is implemented or changed by this spec.

## Implementation Summary

### What Was Implemented

| Component | File | Role |
|-----------|------|------|
| AST gate | `scripts/checks/inert_tests.go` | assert-nothing + tag-orphan detectors; `--selftest`, `--json`, `--check`, `--root=` |
| Gate tests | `scripts/checks/inert_tests_test.go` | 8 tests, all driven from the entry point via `--root` fixture trees |
| Aggregator | `scripts/dev/testing_health.py` | 10 collectors, Markdown + JSON emission, `--write`/`--check`/`--record`/`--json` |
| Aggregator tests | `scripts/dev/testing_health_test.py` | 26 unittest cases, auto-discovered by `TestPythonUnitTests` |
| Site renderer tests | `scripts/dev/site_health_render_test.py` | 16 cases; placed in main, not gh-pages (Deviations D-2) |
| Generated page | `docs/features/test-health.md` | The single Markdown deliverable, 193 lines |
| Artifacts | `test/health/{latest.json,history.ndjson,sensitivity-baseline.json,README.md}` | Metrics, KPI series, ratchet floors, operator doc |
| Site renderer | `../gh-pages/tools/render-test-health.py` | Presentation only; publishes `quality/health/` |
| Verify wiring | `scripts/status/verify_run.go` + `verify_run_test.go` goldens | `ze-test-sensitivity-check` as stage 10 of both modes |
| Make targets | `Makefile` | `ze-test-health`, `-check`, `-record`, `ze-test-sensitivity-check`; check added to `ze-regen-check-readonly`, write added to `ze-regen` |

Measured state at implementation (2026-07-20): RFC proof density 970/2716 (35.7%);
36 of 165 enrolled RFCs with zero test-proven requirements; 136 assert-nothing tests
of 19,856; 12 tag-orphaned files; mutation 60.4% over 22 packages; 122 `.ci` sleeps
against a floor of 125; 2 live known-failures.

### Bugs Found/Fixed

| # | Bug | Where | How found |
|---|-----|-------|-----------|
| 1 | The tag-orphan detector evaluated the build constraint ONCE with every available tag on, condemning every negated constraint. `//go:build !linux` (the non-Linux stubs) and `ze_core && !ze_web` (the compile-out checks `GO_TEST_CORE_TAGS` exists to run) were all reported dead: 34 findings, 22 of them false | `scripts/checks/inert_tests.go` `tagOrphan` | Reviewing live-tree output before setting a baseline. Fixed by replacing the single evaluation with a satisfiability search (`satisfiable`, `classifyTags`); five regression cases added to `--selftest` |
| 2 | The assert-nothing detector judged benchmarks and fuzz targets, which legitimately carry no assertion (a benchmark measures; a fuzz target's oracle is the engine's crash detection), and missed compile-time interface assertions (`var _ Clock = (*V)(nil)`), which fail the BUILD | same, `isTestFunc` / `canFail` | Hand-review of the first 283 findings; 283 -> 136 after the fix |
| 3 | `known-failures` counted 32 entries, including struck-through RESOLVED ones and the whole historical `## Resolved` section, reporting debt already paid off | `scripts/dev/testing_health.py` `collect_known_failures` | Sanity-checking a number that looked too high. Real figure is 2 |
| 4 | `--record` appended to the history the page renders from, leaving the committed page stale immediately after recording, so the next `ze-verify` would fail on a staleness the recorder itself caused | `scripts/dev/testing_health.py` `do_record` | Running the two commands in sequence. `--record` now regenerates the page |
| 5 | Two independent test counters (site and repo) disagreed by 30 (19,850 vs 19,880) because they differ on accepted directories and function-name shapes | `../gh-pages/tools/sitefacts.py` `count_go_tests` | Comparing corrected site facts against the tool's inventory. Fixed structurally: `count_go_tests` now READS `test/health/latest.json` instead of recounting, so divergence is impossible by construction |

### Documentation Updates
- `ai/rules/testing.md` - new "Test Sensitivity Ratchets (BLOCKING)" section, plus three
  rows in the Make Targets table
- `ai/INDEX.md` - three dev-tool rows and one keyword-navigation row
- `docs/features.md` - Testing Health feature row with three source anchors
- `test/health/README.md` - operator doc for the artifacts and the ratchets
- `../gh-pages/`: `data/nav.json` (menu entry), `data/page-links.json` (sidebar both ways),
  `tools/build.py` (`test-health` step), `tools/sitefacts.py` (single source of truth)
- Regenerated after the new source anchors: `ai/CODE-TO-DOCS.md`, `ai/DOCS-TO-CODE.md`
- `make ze-doc-test` PASSED, `make ze-doc-links` PASSED, `make ze-regen-check-readonly` PASSED

### Bugs Found/Fixed
- (fill during implementation)

### Documentation Updates
- (fill during implementation)

### Deviations from Plan

**D-1 (Phase 1): one gate target split into two.** The spec as approved wired a single
`ze-test-health-check` into both `stagesForMode` and `ze-regen-check-readonly`, which would
execute it twice on every verify run. Split by concern, each landing where the existing
convention puts it:

| Target | Does | Wired into | Why there |
|--------|------|-----------|-----------|
| `ze-test-sensitivity-check` | Ratchet: assert-nothing and tag-orphan counts vs committed baseline | `stagesForMode`, both branches, immediately after `ze-port-defaults-check` | It is a source gate exactly like the eight `scripts/checks` gates it sits beside, and it earns its own failure-index group with its own rerun command |
| `ze-test-health-check` | Staleness: committed Markdown matches freshly generated | `ze-regen-check-readonly` prerequisites (`Makefile:501`) | It is a generated-file staleness check exactly like the eight already listed there |

No acceptance criterion changes. AC-10/AC-11 now name `ze-test-sensitivity-check` and AC-12
names `ze-test-health-check`; AC-13 expects `ze-test-sensitivity-check` in
`make ze-verify-list` (the staleness check is reached through its parent stage, as
`ze-doc-check-stale` already is).

**A-7 resolved: confirmed.** User approved correcting `count_go_tests` at the spec gate on
2026-07-20, choosing "Fix it, show honest numbers": `unit_display` 121,500+ → ~19,800+,
`fuzz_display` 570+ → 70+, `e2e_display` 1,400+ unchanged. Delivered: `site-facts.json`
now reads `unit: 19880, fuzz: 72, e2e: 1443`.

**D-2 (Phase 5): the site renderer's tests live in main, not gh-pages.** The gh-pages
convention is `tools/test_*.py`, but nothing executes those files: `tools/build.py`,
`update-website.sh` and `.github/workflows/pages.yml` contain no test invocation, so the
existing `tools/test_render_doc.py` has never run. Adding a second unexecuted test file
while shipping a tool whose purpose is to find tests nothing runs would have been
self-refuting. The renderer's tests are therefore `scripts/dev/site_health_render_test.py`,
which `TestPythonUnitTests` globs and runs on every `make ze-unit-test`; they load the
hyphenated renderer by path exactly as `build.py`'s `load_module` does, and skip cleanly
when the gh-pages worktree is not checked out beside main.

**D-3: the counting fix became a structural fix.** A-6 planned to assert that the corrected
`sitefacts.py` counter agreed with the tool's independent count. In practice the two
disagreed by 30 the moment both existed, because they differ on accepted directories and
on which function-name shapes count. Asserting equality between two counters is a test that
will keep failing for reasons nobody cares about; deleting one counter is a fix. `count_go_tests`
now reads `test/health/latest.json`, warns via `sitelib.warn` when it is absent, and keeps
the local walk only as a fallback. The site can no longer publish a figure the repository
disagrees with.

**D-4 (user-directed): the Markdown lives in the main repository, and the site mirrors it.**
User instruction, 2026-07-20: "quality/health/index.md should be in the main repo not the
website."

The site renderer had a `render_markdown()` that COMPOSED its own summary table from
`latest.json`, so `../gh-pages/quality/health/index.md` was a second, independently authored
document about the same subject: 18 lines against the 193 in
`docs/features/test-health.md`, already differing in wording and in which metrics each
mentioned. Two documents about one subject drift by construction, which is the same
two-sources-of-truth defect D-3 removed between the site and the repository's test counts.

`render_markdown()` is deleted. `page_markdown()` reads
`../main/docs/features/test-health.md` and `write_markdown_sibling` publishes it verbatim,
so the site cannot drift from it and there is exactly one Markdown document to maintain.
Verified byte-identical after a full `tools/build.py`. Four tests in
`TestMarkdownSiblingMirrorsTheRepository`, including a grep-level guard that fails if a
local composer is ever reintroduced.

This also settles the open question about the original requirement ("a single MD file in
the project which is then converted in HTML"): there is now exactly one Markdown file, it
lives in the repository it describes, and it is what the website publishes. Registering it
additionally in `page_registry.DOCS_MANIFEST` was considered and rejected: that would mint
a second HTML page at `/docs/test-health/` carrying the same content as `/quality/health/`,
which is duplication rather than fidelity.

**D-5 (user-directed, after Run 4): gate the structural facts, publish the counters.**
User decision, 2026-07-20, choosing option 2 of three presented.

The clean-room reviewer measured that byte-gating the whole report charged a
regenerate-and-commit to **18 of the last 30 commits (60%)**, because every added test
moves a denominator, and that the two generated files (193 + 393 lines) would collide
across this repo's parallel sessions. It fired twice during the review itself, both times
from another session's commits.

The decisive observation is that the anti-regression guarantee never rested on the report:
`ze-test-sensitivity-check` (stage 10) enforces both ratchets from the tree, reading only
`sensitivity-baseline.json`, and never looks at the page. Byte-gating the report bought
only "the published numbers cannot lag" -- at the price of a check firing on most commits
for cosmetic reasons, which is the *"advisory gates permanently red"* failure the page's own
Q3 criterion condemns. The design was violating its own thesis.

`do_check` now compares `structural_facts()`: the orphaned-test-file list, the unproven-RFC
list, and every metric's status. Each of those changing is an event; a status flipping to
`unknown` in particular means a collector stopped measuring, which is the sensor rot the
whole report exists to surface. Volume counters are published and refreshed by
`make ze-regen`, and the page now says so in place of the old byte-reproducibility claim.

| Change | Before | After |
|--------|--------|-------|
| Commits forced to regenerate | ~60% | only when an orphan, an unproven RFC, or a status changes |
| Ratchet enforcement | stage 10, from the tree | unchanged |
| Sensor rot detectable | yes | yes (status is a gated fact) |
| Published counters can lag | no | yes, by up to one `make ze-regen`, and the page discloses it |

Verified by construction: tampering `inventory.test_funcs` to 999999 leaves `--check` green;
flipping one metric's status fails it and names exactly which. Six tests in `TestEntryPoints`
pin both directions. **This supersedes the byte-exact halves of AC-2 and AC-12**, which are
reworded accordingly rather than quietly dropped.

**D-6 (after Run 4): quality floor replaces the three absolute thresholds.**

The clean-room reviewer's remaining design objection: `collect_rfc`, `collect_mutation`
and `collect_negative_tests` decided status against bare constants (50/75/40) far out of
reach, so five rows sat in "Needs attention" permanently. That is the green-wall failure
from the other side, and the page's own Q3 criterion condemns it ("advisory gates
permanently red").

`quality-baseline.json` (committed) holds the best percentage ever seen for each of the
three higher-is-better metrics. Status is WARN only when a metric drops BELOW its
locked-in best, so the attention table lists regressions rather than a fixed target. It
ratchets UP, the mirror of `sensitivity-baseline.json`. A missing quality floor defaults
to OK (no regression yet) rather than erroring, because unlike the sensitivity floor it
gates no exit code and the low number it would flag is published regardless, so deleting
it to hide a warning is self-defeating. Known trade: a mistaken high floor can only be
lowered by deleting the file and re-bootstrapping, same as any ratchet.

**Observation (not a deviation): shared-worktree races, not non-determinism.** During
implementation another session committed five times into the same working tree and left
uncommitted work in `internal/plugins/static`. The generated page changed between two runs
(RFC pairs 966 → 970) because that session regenerated `ai/RFC-REQUIREMENTS.md` in between.
The generator is deterministic on a fixed tree (proved twice by byte-comparison). A
byte-exact staleness gate over a concurrently-edited tree will flap for this reason, but that
property is already accepted repo-wide: `check_ledger_fresh` gates `ai/RFC-REQUIREMENTS.md`
the same way. No design change.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Single generated MD file in the repo | Done | `docs/features/test-health.md` | 119 lines, committed |
| Converted to HTML | Done | `../gh-pages/quality/health/index.html` | Rendered from `latest.json`; the MD is mirrored verbatim as `index.md` (D-4) |
| Answers "is testing correct", not "large" | Done | 10 metrics under 3 questions; volume figures state their boundary | The RFC "0 outstanding" and site "121,500+" green numbers are decompressed |
| KPI recording over time | Done | `test/health/history.ndjson`, wired to `mk/test-mutation.mk` (finding 57) | |
| Better presented on the website | Done | `quality/health/`, linked from nav, the quality hub, sitemap, search | |

### Acceptance Criteria
See the AC table under "Run 4"/Deviations for the running verdicts; final state:
| AC | Status | Demonstrated By |
|----|--------|-----------------|
| AC-1, AC-2, AC-12 | Done (reworded by D-5) | `TestEntryPoints`, `TestCrossProcessDeterminism` |
| AC-3, AC-5, AC-6, AC-15, AC-17 | Done | `TestRfcLedgerParse`, `TestRatio`, `TestMissingArtifactIsUnknown`, `TestSparkline`/site trend tests |
| AC-4 | Done | `TestInventoryBoundary`, `TestSiteFactsSkipDirs`; site and repo counts now share one source (finding 7) |
| AC-7..AC-11 | Done | `inert_tests_test.go` (15 entry-point tests) |
| AC-13 | Done | `TestStagesForModeMatchesGolden` + live `ze-verify-list` |
| AC-14 | Done | `test_record_appends_exactly_one_row_and_refreshes_the_page` |
| AC-16 | Done (CI caveat) | `site_health_render_test.py` (34); skips when `../gh-pages` absent |
| AC-18 | Done | `.ci` column added (finding 6); `test_ci_presence_is_reported` |

### Tests from TDD Plan
| Test (planned name) | Status | Actual name |
|---------------------|--------|-------------|
| `TestTestSensitivity*` | Done | renamed `TestInertTests*` in `inert_tests_test.go` |
| `TestTestHealthWrites/Check/Record` | Done | `TestEntryPoints` in `testing_health_test.go`... note: implemented in Python (`TestEntryPoints`), not Go; the planned `test_health_test.go` was not created (D-2 rationale) |
| `test_rfc_ledger_parse`, `test_missing_artifact_is_unknown`, `test_age_buckets`, etc. | Done | renamed to `TestRfcLedgerParse`, `TestMissingArtifactIsUnknown`, `TestAdoptionBuckets` |
| `test_site_facts_matches_test_health` | Superseded | D-3 made the counters one source; `TestSiteFactsSkipDirs` covers the boundary |

70 Python + 34 site + 15 Go tests. The spec's original TDD Plan names are stale
where implementation renamed them; the Wiring Test table names four tests that
were renamed (recorded in Run 3's AC re-audit).

### Files from Plan
| File | Status |
|------|--------|
| `scripts/checks/inert_tests.go` (+test) | Done (renamed from `test_sensitivity.go`) |
| `scripts/dev/testing_health.py` (+test) | Done (renamed from `test_health.py`) |
| `scripts/dev/test_health_test.go` | Not created; entry-point coverage is in Python `TestEntryPoints` instead |
| `scripts/dev/site_health_render_test.py` | Done (relocated from gh-pages, D-2) |
| `docs/features/test-health.md`, `test/health/*` | Done |
| `../gh-pages/tools/render-test-health.py` | Done |
| `docs/architecture/testing/test-health.md` | Done (finding 10) |

### Audit Summary
- **Total ACs:** 18
- **Done:** 18 (AC-1/2/12 reworded by D-5, AC-16 with a CI-skip caveat, all with user-visible approval of D-4/D-5)
- **Partial:** none remaining
- **Skipped:** none
- **Changed:** AC-2, AC-12 (D-5), AC-4 (D-3) reworded in place, not dropped

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| A developer can decide whether testing is correct, not merely large | Generated artifact | `docs/features/test-health.md` shows proof density with its split, and in-repo-only counts |
| Places where green is not justified are visible, not derivable | Functional test | `TestTestHealthDocHasRequiredSections` asserts the `unknown` and gap sections render |
| Regression in test sensitivity is blocked, not just reported | Gate test | `TestTestSensitivityRatchetFailsOnRegression` from the gate entry point |
| KPI evolution over time is recorded | Committed artifact | `test/health/history.ndjson` grows one row per `--record` |
| Result is a single MD file converted to HTML | Build output | `docs/features/test-health.md` plus the site destination from `tools/build.py --only test-health` |

## Review Gate

### Run 1 (initial)

Four independent reviewers, one per surface, plus the automated pre-checks.
`audit-test-relaxation.py`: clean. `make ze-validate`: one issue, in
`internal/component/cli/editor_mask.go`, which belongs to a concurrent session
and was not touched.

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | BLOCKER | `--json --check` could never fail: the JSON branch keyed its exit on `result.Valid`, which `scanTree` set unconditionally to `true`. The gate reported findings and exited 0 | `inert_tests.go` `main`, `scanTree` | fixed: enforcement moved ahead of every output branch; `Valid` now set by `enforce`. Test `TestJSONCheckStillEnforcesTheRatchet` |
| 2 | BLOCKER | Tag universe was seeded straight from `feature-gates.txt`, so a tag was reachable by DECLARATION, not by any `go test -tags` reference. Removing `$(ZE_FEATURES)` from `GO_TEST_TAGS` would strand every gated test with the gate still green | `inert_tests.go` `testTagUniverse` | fixed: the manifest now binds the `ZE_FEATURES` make VARIABLE; only a `go test -tags` reference admits a tag. Test `TestTagUniverseRequiresAGoTestReference` |
| 3 | BLOCKER | `do_write` rendered the page BEFORE tightening the baseline, and the floor is part of the rendered value, so any tightening run wrote a page that was stale on arrival. Following the ratchet's own advice broke the staleness gate | `testing_health.py` `do_write` | fixed: `tighten_baseline` runs first and the metrics are rebuilt when a floor moves. Verified idempotent over two live runs |
| 4 | BLOCKER | The page was a function of the WORKING TREE, so untracked work-in-progress tests moved the published numbers and a clean CI checkout disagreed with the committed page | all collectors; `inert_tests.go` `collectTestFiles` | fixed: collectors read `git ls-files` (`tracked_files`); the gate keeps scanning the working tree via a new `--tracked-only` split. Tests `TestTrackedFilesDriveThePage`, `TestTrackedOnlyIgnoresUntrackedFiles` |
| 5 | BLOCKER | A missing baseline key defaulted to the current count, so `min()` could not prevent a raise: a regression could be laundered into the floor | `testing_health.py` `do_write` | fixed: a missing/non-integer/negative floor is now a `CollectError`. Seven tests in `TestBaselineOnlyTightens` |
| 6 | BLOCKER | `SKIP_DIRS` matched any path component, so `"cache"` dropped five first-party packages and `"gokrazy"` dropped `internal/component/gokrazy` | `sitefacts.py` | fixed: split into `SKIP_TOP_LEVEL` / `SKIP_ANY_LEVEL`. Verified 0 first-party tracked tests now dropped, 4 modcache files still are |
| 7 | ISSUE | Same-named helper in `foo` and `foo_test` collapsed into one map, so the verdict depended on Go's randomised map iteration and the count flapped between runs | `inert_tests.go` `packageFuncs` | fixed: indexed per package. Test `TestSameNameHelperAcrossPackagesIsDeterministic` runs the gate six times |
| 8 | ISSUE | False positives: testify under a non-canonical alias, helpers in non-test files, and assertions via a receiver method were all invisible | `inert_tests.go` `canFail` | fixed: `assertAliases` resolves import aliases; method names are followed. Three selftest regressions |
| 9 | ISSUE | The escape annotation only worked in a doc comment; in-body placement was silently ignored while the failure message said "annotate the test" | `inert_tests.go` `hasEscape` | fixed: comments are matched by position range against `File.Comments`. Selftest regression |
| 10 | ISSUE | Any `//go:build`-shaped comment anywhere in a file was read as its constraint, fabricating findings against files that build everywhere | `inert_tests.go` `tagOrphan` | fixed: only comments before the package clause count. Selftest regression |
| 11 | ISSUE | `collect_adoption` depended on the user's `diff.renames` git config: 515 directories changed first-commit stamp, 258 changed year bucket, so two machines rendered two pages | `testing_health.py` | fixed: `-c diff.renames=false -c core.quotePath=false --no-renames` |
| 12 | ISSUE | A single-slot directory sentinel re-counted a package whenever a subdirectory sorted between two of its files: 490 packages published where 481 exist | `testing_health.py` `collect_adoption` | fixed: per-bucket set. Tests in `TestAdoptionBuckets` |
| 13 | ISSUE | `NEGATIVE_ASSERT` measured close to the opposite of its name: it missed the idiomatic multi-line form and matched one-line setup guards and comments | `testing_health.py` | fixed: error-expectation tokens only, comments stripped, label corrected. Tests in `TestNegativeAssertTokens` |
| 14 | ISSUE | Missing `plan/known-failures.md` rendered as a green `0`; a `###` before any `##` counted as live debt | `testing_health.py` `collect_known_failures` | fixed: absent input is `unknown`; section-less entries are not live |
| 15 | ISSUE | Raw tracebacks instead of `CollectError` on: corrupt baseline, non-integer sleep baseline, corrupt history line, null mutation score, package-less mutation row, zero mutants | `testing_health.py` | fixed across all six paths, each with a test |
| 16 | ISSUE | An unknown CLI argument was silently ignored, dropping the gate into report-only mode which always exits 0 | `inert_tests.go` `main` | fixed: unknown argument exits 2. Test `TestUnknownArgumentIsRejected` |
| 17 | ISSUE | A missing test root was skipped silently, shrinking the scan; `--write` would then bake the small number into the floor | `inert_tests.go` `collectTestFiles` | fixed: a missing root is an error. Test `TestMissingTestRootFailsClosed` |
| 18 | ISSUE | `meter()` interpolated numerator and denominator into an attribute and body text with no escaping | `render-test-health.py` | fixed, plus `question` pipe-escaping in the Markdown mirror. Six tests in `TestEscaping` |
| 19 | ISSUE | Twelve malformed-shape inputs raised rather than degrading, failing the site build with a traceback that named the step but not the metric | `render-test-health.py` | fixed: defensive `.get`/`str`/type checks throughout. Tests in `TestMalformedMetricsDegrade` |
| 20 | ISSUE | `count_go_tests` fell through silently and reported "missing" for a file that was present but shapeless; a top-level JSON array escaped the `except` | `sitefacts.py` | fixed: distinct messages per cause, `TypeError` caught, absent `../main` no longer warns (it is graceful degradation, not drift) |
| 21 | ISSUE | An unrecognised status sorted BELOW `ok` and rendered as a neutral card: sensor rot presenting as calm | `render-test-health.py` | fixed: `rank()`/`status_class()` treat anything unrecognised as `unknown` |
| 22 | ISSUE | A comment asserted a cross-check between `SKIP_DIRS` and `TEST_ROOTS` that no test performed | `sitefacts.py` | fixed: claim removed; the counters were unified instead, so divergence is impossible rather than merely asserted |
| 23 | ISSUE | Tautological tests: `test_deterministic_output` called a pure function twice; `test_module_cache_is_not_counted` put its only decoy outside `TEST_ROOTS`, so it passed with the filter deleted | `testing_health_test.py` | fixed: decoys moved inside a test root; real determinism now covered by the tracked-files and idempotence tests |
| 24 | ISSUE | `render-test-health.py` docstring claimed staleness detection it does not have | `render-test-health.py` | fixed: docstring states where freshness IS enforced |
| 25 | ISSUE | `data/nav.json` was reformatted wholesale, turning a 6-line addition into a 434-line diff | `../gh-pages/data/nav.json` | fixed: restored to the committed compact style, diff now 1 line |
| 26 | NOTE | `.Error()` on an error value counted as an assertion | `inert_tests.go` | fixed: zero-argument `.Error()` excluded. Selftest regression |
| 27 | NOTE | Unsatisfiable-over-reachable constraints printed an empty "requires" list | `inert_tests.go` `tagOrphan` | fixed: reports the constraint text |
| 28 | NOTE | Dead `uniq()`; stale filename in a header comment; `test/health/README.md` said "all four files" for three | several | fixed |
| 29 | NOTE | No `timeout=` on any subprocess: a hung `go run` or `git log` would hang `ze-verify` with no diagnostic | `testing_health.py` | fixed: `SUBPROCESS_TIMEOUT` on every call |
| 30 | NOTE | `sitelib` and `sitefacts` now form an import cycle | `sitefacts.py` | verified harmless by executing all three import orders; neither touches the other at module scope |

Not fixed, and why:

| Finding | Severity | Disposition |
|---------|----------|-------------|
| Several ACs had no test (AC-1, AC-2, AC-12, AC-14, AC-16, AC-18): the planned `scripts/dev/test_health_test.go` was never written | BLOCKER | Genuine gap in the first pass. Now covered by 46 Python and 15 Go tests; the AC audit is re-run below |
| `go run` collapses exit 2 into exit 1, so callers cannot distinguish a fail-closed refusal from a ratchet breach | NOTE | Accepted: a Go toolchain behaviour, and both are failures that stop the build. Messages distinguish them |
| gh-pages `tools/test_*.py` are never executed by anything | ISSUE | Pre-existing, not introduced here. Recorded as the reason D-2 put the renderer's tests in main, where they run |
| `assets/demos/manifest.json` missing locally fails the site `docs` step for 18 documents | ISSUE | Pre-existing; CI builds the demos first. None of the affected documents is part of this change |

### Fixes applied
- `scripts/checks/inert_tests.go`: enforcement ordering, tag-universe derivation, per-package
  helper indexing, import-alias resolution, method following, positional escape-comment
  matching, build-constraint position bound, zero-arg `.Error()` exclusion, unknown-argument
  rejection, missing-root failure, `--tracked-only` population, dead code removed
- `scripts/checks/inert_tests_test.go`: 7 new entry-point tests, split stdout/stderr capture
- `scripts/dev/testing_health.py`: `tighten_baseline` extracted and hardened, tracked-file
  collectors, git-config-independent `git log`, adoption set tracking, negative-assert
  redefinition, six fail-closed parse paths, subprocess timeouts
- `scripts/dev/testing_health_test.py`: 20 new tests, two tautological ones repaired
- `../gh-pages/tools/render-test-health.py`: escaping, malformed-shape tolerance, status
  fallback, docstring correction
- `../gh-pages/tools/sitefacts.py`: skip-set semantics, control flow, warning accuracy
- `../gh-pages/data/nav.json`: restored formatting

### Run 2 (independent verification of the Run 1 fixes)

A fresh reviewer re-read every changed file and mutation-tested the fixes
(reverting fix 2 to confirm its new test genuinely fails). Fixes 1, 2, 3, 6, 7,
9, 10, 11 and 14 were verified correct by execution. Nine further findings, of
which one was a defect the fixes themselves introduced.

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 31 | ISSUE | **Introduced by fix 8.** Discriminating `err.Error()` from `t.Error()` by ARGUMENT COUNT condemned correct code: `t.Error()` with no arguments is legal Go and does fail the test | `inert_tests.go` `canFail` | fixed: discriminate on the RECEIVER. `testingIdents` collects identifiers bound to `*testing.T/B/F`, including subtest-closure parameters. Three selftest regressions |
| 32 | ISSUE | "Subprocess timeouts everywhere" was false: the `go run` call (the likeliest to hang, and the one the comment named) plus two `git` calls were unbounded; `repo_root` used `check=True` so running outside a repository raised a traceback | `testing_health.py` | fixed: all four bounded; `repo_root` raises `CollectError` |
| 33 | ISSUE | `m["question"]` was the one un-guarded access left in the renderer, so a metric missing that key took the whole site build down. The new test could not catch it: all four "bad" fixtures carried `question` | `render-test-health.py` `render()` | fixed: `.get()`; fixture corrected |
| 34 | ISSUE | The malformed-shape hardening was applied to the site's `detail_table` but not to its twin `_detail_tables` in the main repository, which kept both defects and escaped no `\|` | `testing_health.py` | fixed: union-of-keys, non-dict rows skipped, `_cell()` escapes the separator |
| 35 | ISSUE | "Every collector reads git's index" was overstated: `collect_rfc` still globbed `rfc/short/`, file CONTENTS always come from the working tree, and `git ls-files` reads the index rather than HEAD | `testing_health.py` | fixed both ways: `collect_rfc` now reads tracked summaries and fails closed on none; the docstring states the two remaining limits plainly instead of overclaiming |
| 36 | ISSUE | Fix 5 guarded a missing baseline KEY but not a missing baseline FILE: `rm sensitivity-baseline.json && make ze-test-health` minted today's counts as the floors and verify then passed | `testing_health.py` `tighten_baseline` | fixed: a missing file is an error; `--bootstrap-baseline` makes first-time creation a deliberate act. Three tests |
| 37 | ISSUE | Fix 3 had no regression test: nothing drove the write -> check round trip it exists for | `testing_health_test.py` | ~~fixed: `TestWriteCheckRoundTrip` tightens a slack floor and asserts the second pass is a fixed point~~ **THIS CLAIM WAS FALSE, corrected in Run 3.** That class only ever called `tighten_baseline`; it never touched `do_write` or `do_check`, so the very regression it was named for would still have shipped. Genuinely fixed in Run 3 by `TestEntryPoints` (7 cases driving `do_write`/`do_check`/`do_record`) |
| 38 | ISSUE | `collect_negative_tests` bucketed by `rel.parts[:3]`, so a three-component path yielded the FILE as the "area": 117 of 318 areas were single files, all dropped by the `>=5` filter, so whole trees could never appear in the table the metric's action points at | `testing_health.py` | fixed: bucket by directory. `TestNegativeTestAreaBuckets` |
| 39 | ISSUE | The `SKIP_TOP_LEVEL`/`SKIP_ANY_LEVEL` split shipped with no executed test, in a spec about tests nothing runs; `SKIP_DIRS` was left dead and its comment contradicted the new code | `sitefacts.py` | fixed: dead constant removed, comment corrected, four tests in `TestSiteFactsSkipDirs` including one driven from the real repository's tracked files |
| 40 | NOTE | `MIN_SAMPLES` duplicated across the two sides with no cross-check | both | fixed: `TestThresholdsAgreeAcrossTheBoundary` asserts equality |
| 41 | NOTE | Six weak or tautological tests named individually, including `test_deterministic_output` (same objects, one process, cannot detect cross-process ordering) and two that restated the `STATUS_ORDER` literal | both test files | ~~fixed: determinism now runs two subprocesses under different `PYTHONHASHSEED`~~ **THIS CLAIM WAS FALSE, corrected in Run 3.** The edit silently no-opped (its `str.replace` target had been reformatted) and I recorded the fix without verifying it landed; `grep PYTHONHASHSEED` returned nothing. The ordering and duplicate-helper halves DID land. Genuinely fixed in Run 3 by `TestCrossProcessDeterminism`, which renders under three different `PYTHONHASHSEED` values in subprocesses |

Accepted without change:

| Finding | Why |
|---------|-----|
| `--json` alone always emits `"valid": false` | No consumer reads it; enforcement is the exit code. Noted as a trap rather than reshaped mid-review |
| `goTestTagsRe` misses double-quoted `-tags` and `+=` variables | Neither shape exists in the repository (verified by grep), and a total parse loss fails loud via the empty-universe check |
| `assertAliases` seeds six common names unconditionally | Over-approximation: false negatives only, which is the safe direction for a ratchet |
| `enforce` prints "baseline is slack" on a successful run | Correct information; it fires when the working tree is cleaner than the index, which is exactly when a developer should tighten |
| Fix 12 (comment stripping) changes no verdict on this repo today | Confirmed defensive-only; retained because it can only under-count, never over-count |

Evidence after both rounds: `gofmt -l` clean for the two new Go files
(`command_ownership.go` and `direct_fs_persistence.go` carry pre-existing drift
and are untouched); `go vet` clean; `go test ./scripts/checks/ ./scripts/status/`
PASS; `testing_health_test.py` **50** PASS; `site_health_render_test.py` **34**
PASS; `make ze-test-sensitivity-check` exit 0 (assert-nothing 134/134,
tag-orphan 12/12, 2576 files); `make ze-test-health-check` exit 0;
`make ze-regen-check-readonly` exit 0; `quality/health/index.md` byte-identical
to `docs/features/test-health.md`.

### Run 3 (four fresh lenses: red team, truth audit, unreviewed-code pass, AC/doc re-audit)

The truth audit recomputed every published number independently. **All ten metrics
were arithmetically correct.** What was wrong was the PROSE around them, on a page
whose entire value is honesty. That is the most important result of this review.

| # | Severity | Finding | Action |
|---|----------|---------|--------|
| 42 | BLOCKER | The annotation split (855+540+371=1766) was presented as the breakdown of a remainder of 1746. Different populations: the ledger counts gated levels only, the grep counted MAY/SHOULD/OPTIONAL too. One file (rfc4577) supplied all 20 | fixed: counted per gated-level requirement line, and a `CollectError` now refuses to publish a split that is not a partition. Two tests |
| 43 | BLOCKER | "The remainder is annotated, **not tested**" was false for 371 single-polarity requirements: `rfc_requirements.py:628` errors out if such a requirement has no test, so all 371 have a passing tagged test | fixed: the detail now separates not-applicable (no test owed), gap (genuinely untested), and single-polarity (tested, one side) |
| 44 | BLOCKER | "every requirement is annotated rather than proven" for the 36 unproven RFCs was false: rfc7871 and rfc2132 both carry positive-tagged tests | fixed: "no requirement proven by BOTH polarities; some carry positive-only tests" |
| 45 | BLOCKER | known-failures published **2**; the true live count is **6**. Excluding any section whose heading contains "not failures" zeroed `## Harness notes (not failures)`, which holds three live reds -- one a deterministic `slice bounds out of range` panic in ze. The prose also called every live entry "flaky or environmental", laundering a memory-safety defect | fixed: only strike-through or `## Resolved` excludes an entry; prose corrected. Test expectation updated with the reason recorded |
| 46 | BLOCKER | `failureSelectors` matched the METHOD NAME with no receiver check, so `fmt.Errorf(...)`, `log.Fatalf(...)` and a business method `Fail()` all credited a test as asserting. 78 live tests took that path, 66 via `fmt.Errorf` | fixed: a failure method counts only on a testing receiver or an assertion alias. Four selftest regressions |
| 47 | BLOCKER | `isAssertionImport` matched `"/is"` as a SUBSTRING, so `internal/plugins/isis/...` registered `types`, `packet`, `lsdb`, ... as assertion aliases. **All of ISIS was exempt** -- 143 tests, hiding 2 genuinely inert ones | fixed: match the final path element. Count rose 134 -> 136, which is the detector getting more accurate |
| 48 | BLOCKER | `assertAliases` seeded `assert/require/is/must/should/qt` unconditionally, so a local variable named `assert` credited the test | fixed: aliases come only from real imports. Caught by the new selftest case before it could ship |
| 49 | BLOCKER | `collect_inert` read the baseline raw, so a corrupt one crashed `build()` with a traceback before `tighten_baseline`'s careful guards ran. Its three "fails closed" tests all called the helper directly and passed while the entry point crashed | fixed: one validated `read_baseline` used by both |
| 50 | BLOCKER | `--record` appended a history row and THEN failed if the baseline was missing, leaving the page stale and adding another row on every retry -- the exact staleness it regenerates the page to prevent. `--bootstrap-baseline` was accepted and silently ignored | fixed: validated before the append; the flag is threaded through |
| 51 | BLOCKER | `--check` gated only the Markdown. `latest.json` -- what the WEBSITE publishes its test count from -- could be stale or deleted with `ze-verify` fully green | fixed: `latest.json` is byte-compared too, via one shared serialiser. Three tests |
| 52 | BLOCKER | `test/health/README.md` and the generated page's banner both still claimed "every collector reads git's index" / "reproducible from any checkout". Run 2 fixed this in the module docstring ONLY | fixed in all three places, stating the two real limits |
| 53 | BLOCKER | `sparkline`'s docstring justified emitting raw SVG by claiming the site's Markdown renderer turns it into a chart. It does not: the site page is built from JSON by its own renderer, and the mirrored `.md` is served raw. The SVG becomes a chart on no site surface | fixed: the docstring now says where it does and does not render |
| 54 | ISSUE | `--json --check` aside, the two `str.replace` edits that silently no-opped (findings 37, 41) | fixed; both Review Gate rows corrected in place and a Mistake Log row added |

**Verification after Run 3:** 59 Python + 34 site + 15 Go tests pass; `make ze-test-health`
byte-identical over consecutive runs; both gates exit 0 (assert-nothing 136/136,
tag-orphan 12/12, 2576 files); site mirror byte-identical to the repository page.

### Known limitations, recorded rather than fixed

The red team defeated the assert-nothing detector in ways that are real but not
worth more machinery today. Recording them honestly, because a guard whose limits
are undocumented invites false confidence:

| Limitation | Why not fixed now |
|-----------|-------------------|
| Helper chains deeper than one level read as inert (`c.run(t)` -> `c.assert(t)`) | Raising `depth` to 4 changes the live count by zero, so it costs runtime for no current benefit. Documented in Known Limitations |
| Cross-package test helpers (`testutil.RequireEqual(t, ...)`) read as inert | Needs type resolution, not AST names. Repo exposure is 5 importers today |
| `var _ int = 0` and dead-code `panic` credit a test | Deliberate dodges, not accidents. The escape annotation is the honest route anyway |
| The ratchet is count-only: one commit may fix one inert test and add another | Per-file identity would make the baseline a 136-entry list and every unrelated rename a diff. The count floor is the cheaper 90% |
| The escape annotation is unlimited, unratcheted and needs no reason | Worth a follow-up: count annotations as their own ratcheted series |
| `// +build` legacy constraints are invisible to tag-orphan | Zero such files in the repo; latent |
| A test root that is a symlink or a regular file is skipped silently | `chmod 000` IS caught; these two shapes are not |
| Makefile reformatting (`$(GO) test`, double quotes, line continuation) collapses the tag universe | Total loss fails closed and loudly; a PARTIAL loss would wrongly condemn ~31 files |
| `testingIdents` misses `testing.TB`, variadic `...*testing.T`, and a `*testing.T` struct field | Affects only the zero-argument `Error()` discriminator; every other failure call still matches |
| `page_markdown()` publishes an empty mirror silently, and crashes on non-UTF-8 | Both require a broken main-side page, which `--check` already gates |

### Run 4 (clean room)

A reviewer with no knowledge of Runs 1-3, barred from reading this Review Gate,
given the deliverables and an unsteered brief ("is this a good idea, executed
well? would you have built it this way?"). It independently recomputed the
page's numbers and mutation-probed both test suites (9 of 10 injected mutations
caught in the Go selftest, 7 of 8 in the Python suite).

Its verdict: **approve the idea, request changes before publishing.**

| # | Severity | Finding | Action |
|---|----------|---------|--------|
| 55 | BLOCKER | The negative-test metric measured assertion-LIBRARY adoption, not error assertions. This project's house style is plain Go (`if err := ...; err == nil { t.Fatal }`), invisible to the token list. Measured: 418 -> **828** files (16.2% -> 32.2%), and every one of the ten "0.0%" rows was false -- `internal/component/bfd` published 0/31 while having 13 files of forged-packet and tampered-auth rejection tests. A maintainer following the stated action would have spent a day adding negative tests to a subsystem that already had them | fixed: the plain-Go idioms are matched. **Introduced by me in Run 2**: correcting a regex that over-matched setup guards, I narrowed it so far it missed the house style. Over-correction is still a correctness bug |
| 56 | ISSUE | In a shallow clone git attributes every file to the graft commit, so `collect_adoption` produced one bucket instead of two, the page read stale, and the fix-instruction ("regenerate and commit") would then break every full clone -- an unescapable red | fixed: `git rev-parse --is-shallow-repository` renders the metric `unknown` with that reason |
| 57 | ISSUE | `ze-test-health-record` had **no caller anywhere** -- not a Makefile, not `mk/*.mk`, not `.woodpecker/`. The committed KPI series could never grow, so trends were permanently "insufficient data". A deliverable that ships unwired, which `ai/rules/wiring-completeness.md` exists to prevent. Its stated precedent, `mutation_history.py`, IS invoked from three recipes | fixed: wired into all three `mk/test-mutation.mk` mutation targets, beside `mutation_history.py`, where a fresh sample is meaningful |
| 58 | ISSUE | The two committed samples were identical, 24 seconds apart, at the same sha. Nothing deduped, so repeated `--record` would stack duplicate points and overstate the `n` the page prints to stay honest | fixed: a sample identical to the previous one at the same commit is skipped; the duplicate row removed |

Not fixed, and why:

| Finding | Severity | Disposition |
|---------|----------|-------------|
| The staleness gate taxes ~57% of commits (17 of the last 30 add or remove a test function), and two high-churn generated files will conflict across parallel worktrees | ISSUE | **A real design objection, and the reviewer is right that it is a cost.** Splitting the gated structural artifact from an ungated volume snapshot would keep the anti-rot property without the per-commit tax. That is a design change, not a bug fix, and it is the user's call |
| All 34 site-renderer tests skip in CI (`HAVE_SITE` guard; `.woodpecker/verify.yml` checks out only `main`) | ISSUE | Inherent to the two-repo split. Recorded; the honest options are to move the renderer into `main` or to state the gap on the page |
| Three OK/WARN thresholds (50/75/40) are undocumented magic numbers, and all three are far out of reach, so the attention table is permanently populated -- the same failure as a green wall, from the other side | ISSUE | Legitimate. The right fix is to derive status from a committed floor as the two ratcheted metrics already do, which needs a floor per metric and a decision about starting values |
| `count_e2e_files` still walks the working tree (1445) while `latest.json` says 1443 | NOTE | The same two-counter defect, surviving for `.ci` after being removed for unit tests |
| `var _ int = 1` and dead-code `panic` still credit a test | NOTE | Already recorded under Known Limitations |
| The commit is entangled with `spec-fixit-verify-stage-ssot.md` in `verify_run.go` | NOTE | Independently confirmed by this reviewer. Still blocking |

What it judged genuinely good, unprompted: the RFC partition invariant ("a real
cross-source invariant, not a comment"), the floor-embedded-in-the-rendered-value
coupling that forces self-tightening, the `--tracked-only` population split, and
the comment density ("nearly every non-obvious line carries the bug it prevents,
with the observed magnitude"). It confirmed `inert_tests_test.go` is not itself
inert, and that the 12 tag-orphans and 136 assert-nothing findings are real.

### Run 5 (closing work, at user request "finish so we can close")

Not a review pass; the remaining functional gaps and one design fix were
implemented.

| # | Was | Action |
|---|-----|--------|
| 59 | AC-18 implemented only 2 of 3 techniques (no `.ci` column) | fixed: `.ci` presence per package, keyed by the `.ci` file's own first-commit date; `test_ci_presence_is_reported`. Both renderers gain the column |
| 60 | `count_e2e_files` still walked the tree (1445) vs `latest.json` (1443) | fixed: `inventory_counts()` is one source for unit, fuzz AND `.ci`; site and repo now agree by construction |
| 61 | The three OK/WARN thresholds (50/75/40) put five rows in the attention table permanently | fixed (D-6): status is a regression signal against `quality-baseline.json`, which ratchets UP. The two genuine signals (`rfc-unproven`, `known-failures`) remain; the three arbitrary-threshold rows are now OK, locked at their current best. `TestQualityFloor`, 7 cases |
| 62 | `ze-test-health-check` undiscoverable; no arch doc; no quality-hub link; not in `make help` | fixed: `ai/INDEX.md` dev-tools row, `docs/architecture/testing/test-health.md`, `quality/quality.md` link, `make help-test` block |
| 63 | No learned summary | written: `plan/learned/1226-test-health-dashboard.md` |
| 64 | Implementation Audit, Pre-Commit Verification, Goal Validation empty | filled with fresh evidence |

**D-6 (quality floor)** is recorded in Deviations. It resolves the clean-room's
threshold objection with the same mechanism the reviewer praised for the
sensitivity ratchet, so the attention table now lists regressions, not a
permanent gap to an invented target.

### Final status
- [x] Independent review ran across five rounds (four adversarial + one clean-room); every BLOCKER and ISSUE fixed or recorded with rationale
- [x] All NOTEs recorded (Known Limitations table + Mistake Log)

Remaining open, NOT closed by me:
- AC-16's 34 site tests skip in CI (two-repo split; recorded, not a defect in this code).
- The commit is blocked: `scripts/status/verify_run.go` carries another session's
  uncommitted work alongside this feature's two stage lines, and the change is
  atomic (the new stage reads a baseline that must land in the same commit).
  This is a coordination problem, not an implementation gap. Run 3's AC re-audit found AC-18 only partially implemented (the
`.ci`-presence third of the adoption metric was never written), and several spec
sections (Implementation Audit, Pre-Commit Verification, Goal Gates) are still
empty. Those are tracked above, not silently closed.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `scripts/checks/inert_tests.go` + `_test.go` | yes | `go test ./scripts/checks/` ok |
| `scripts/dev/testing_health.py` + `_test.py` | yes | 70 tests OK |
| `scripts/dev/site_health_render_test.py` | yes | 34 tests OK |
| `docs/features/test-health.md` | yes | 119 lines |
| `test/health/{latest.json,history.ndjson,sensitivity-baseline.json,quality-baseline.json,README.md}` | yes | committable, none gitignored |
| `docs/architecture/testing/test-health.md` | yes | linked from `ai/INDEX.md` |
| `../gh-pages/tools/render-test-health.py`, `quality/health/index.{html,md}` | yes | full build renders them |

### AC Verified (grep/test) — re-checked independently, not copied from the audit
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-7/10 | Inert test fails the ratchet, names the file | `TestInertTestsRatchetFailsOnRegression` PASS |
| AC-9 | Tag-orphan detected, negated constraints not | `TestTagOrphanDetection` PASS |
| AC-12 | A status change fails `--check`; volume drift does not | `test_check_fails_when_a_metric_status_changes` + `test_check_ignores_pure_volume_drift` PASS |
| AC-13 | Stage present both modes | `make ze-verify-list` shows `ze-test-sensitivity-check` at position 10 |
| AC-18 | `.ci` column present | `test_ci_presence_is_reported` PASS; page table has 5 columns |
| D-5 | quality metric warns on regression only | `TestQualityFloor.test_below_floor_warns` + live simulation |

### Wiring Verified (end-to-end)
| Entry Point | Mechanism | Verified |
|-------------|-----------|----------|
| `make ze-verify` | `ze-test-sensitivity-check` in `stagesForMode`, both branches | `ze-verify-list`, both modes |
| `make ze-regen-check-readonly` | `ze-test-health-check` prerequisite | `Makefile:545`, gate exits 0 |
| `make ze-mutation-*` | `testing_health.py --record` (3 recipes) | `mk/test-mutation.mk:39,65,109` |
| `TestPythonUnitTests` | globs `scripts/dev/*_test.py` | both Python files discovered and run |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed, later moot | SVG-through-Markdown works, but D-4 means the site never uses that path (recorded) |
| A-2 | confirmed | determinism proven cross-process; D-5 narrowed what is gated |
| A-3 | confirmed | `TestRfcLedgerParse` 4 cases |
| A-4 | confirmed | detector reviewed by hand across rounds; false positives fixed |
| A-5 | confirmed | gate runs in ~2.4s, well inside budget |
| A-6 | confirmed (design changed) | counters unified rather than asserted equal (D-3) |
| A-7 | confirmed | user approved the count correction |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `ai/rules/testing.md` ratchet section | target names, stage 10, exemptions all match code | yes |
| `ai/INDEX.md` rows (4 targets + keyword + arch doc) | grep resolves each | yes |
| `docs/features.md` row + 3 source anchors | anchored files contain the claim | yes |
| `test/health/README.md` | tracked/working-tree split, 5 files, command table | corrected, matches code |
| generated page banner | "structural gated, counters published" | matches `do_check` |
| `docs/architecture/testing/test-health.md` | new; describes both mechanisms | matches code |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-18 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md` — no failures)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC constraint comments added — N/A
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] Abstract when you can (2+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `ai/rules/quality.md` documented pass in spec
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-test-health-dashboard.md` only
