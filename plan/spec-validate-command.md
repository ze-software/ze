# Spec: Post-Implementation Validation Tool

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/6 |
| Updated | 2026-06-13 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/rules/INDEX.md` - all 74 rules this tool checks
4. `scripts/dev/verify-lock.sh` - existing verify infrastructure

## Task

Add `make ze-validate` as a fast (~2s) Python script that catches recurring
implementation mistakes that `ze-verify` misses but code review always finds.
Each check derives from a documented defect pattern in this project.

Runs after `ze-verify` passes, before presenting work as complete.
Not a replacement for review -- a pre-screen that eliminates the most common
review-fix-review cycles.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/INDEX.md` - all 74 rules; the tool enforces a subset programmatically
- [ ] `plan/learned/RECURRING-PATTERNS.md` - defect patterns this tool targets
- [ ] `scripts/dev/verify-lock.sh` - existing verify infrastructure pattern
- [ ] `mk/test-functional.mk` - existing make target patterns

### RFC Summaries (MUST for protocol work)
N/A - developer tooling, no protocol.

**Key insights:**
- Every check in this tool corresponds to a rule that reviews catch repeatedly
- The tool runs on the working tree (uncommitted changes), not on committed code
- Python-only (no Go compilation), so it's fast

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `scripts/dev/verify-lock.sh` - verify infrastructure (flock-based single-instance)
- [ ] `scripts/dev/verify_run.go` - verify stage runner
- [ ] `.claude/hooks/posttool-writeedit.py` - existing Python-based checks (inline)

**Behavior to preserve:**
- `make ze-verify` remains the primary gate (lint, tests, wiring, functional)
- No changes to existing verify stages

**Behavior to change:**
- Add a new `make ze-validate` target that runs after verify
- The new target runs a single Python script with multiple check functions

## Data Flow (MANDATORY)

### Entry Point
- `make ze-validate` invokes `scripts/dev/validate.py`
- Script reads: working tree Go files, spec files, test files, docs

### Transformation Path
1. Parse `git diff --name-only HEAD` to find changed files
2. For each check function, scan relevant files
3. Collect findings as (severity, file, line, message) tuples
4. Print findings grouped by severity
5. Exit 0 if no BLOCKER/ISSUE, exit 1 otherwise

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Makefile -> Python | `make` target invokes script | [ ] |
| Python -> filesystem | reads .go, .md, .ci files | [ ] |

### Integration Points
- `Makefile` - new `ze-validate` target
- `mk/` - may add `mk/validate.mk` include

### Architectural Verification
- [ ] No bypassed layers (runs after verify, not instead of)
- [ ] No unintended coupling (standalone script, no Go imports)
- [ ] No duplicated functionality (checks things verify does NOT check)
- [ ] Zero-copy preserved where applicable (N/A - Python script)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | Python 3.9+ available on all dev machines | macOS ships Python 3; CI has it | Script won't run | Check `python3 --version` | confirmed |
| A-2 | `git diff` output is sufficient to identify changed files | Standard git workflow | Need alternative file discovery | Test with uncommitted + untracked | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | False positives annoy developers | User reports | Each check has a comment explaining WHY; easy to add exceptions |
| R-2 | Slow on large diffs | > 5s runtime | Limit to changed files only, not full repo scan |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `make ze-validate` | -> | `scripts/dev/validate.py` | Makefile target exists and script runs |
| Script exit code | -> | CI integration | Exit 0 on clean, 1 on findings |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Source anchor with line number `*.go:47` | Reports ISSUE with fix suggestion |
| AC-2 | Source anchor pointing to non-existent file | Reports ISSUE |
| AC-3 | Exported symbol with no cross-package non-test caller | Reports ISSUE |
| AC-4 | Spec AC row with empty "Demonstrated By" column | Reports ISSUE |
| AC-5 | Changed user-facing CLI handler with no .ci test mentioning it | Reports ISSUE |
| AC-6 | Clean codebase (no violations) | Exit 0, prints "all checks passed" |
| AC-7 | `make ze-validate` target exists and invokes the script | Target runs successfully |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `test_line_number_anchor` | `scripts/dev/validate_test.py` | Detects `:NN` anchors | |
| `test_stale_anchor_path` | `scripts/dev/validate_test.py` | Detects missing files | |
| `test_cross_package_wiring` | `scripts/dev/validate_test.py` | Detects same-package-only exports | |
| `test_spec_ac_completeness` | `scripts/dev/validate_test.py` | Detects empty AC cells | |
| `test_clean_passes` | `scripts/dev/validate_test.py` | No false positives on clean code | |

### Boundary Tests (MANDATORY for numeric inputs)
N/A - no numeric inputs.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-validate-clean` | `test/plugin/validate-clean.ci` | `make ze-validate` exits 0 on committed clean code | |

### Interop Tests (MANDATORY for protocol features)
N/A - developer tooling, no protocol.

## Files to Modify
- `Makefile` - add `ze-validate` target

## Files to Create
- `scripts/dev/validate.py` - main validation script
- `scripts/dev/validate_test.py` - unit tests for check functions
- `mk/validate.mk` - make include (if separated)

## Implementation Steps

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Script skeleton** -- argparse, file discovery, finding collector
   - Tests: `test_clean_passes`
   - Files: `scripts/dev/validate.py`
   - Verify: script runs, exits 0 on clean tree

2. **Phase: Source anchor checks** -- line-number detection, path/symbol validation
   - Tests: `test_line_number_anchor`, `test_stale_anchor_path`
   - Files: `scripts/dev/validate.py`
   - Verify: catches `*.go:47` anchors and missing paths

3. **Phase: Cross-package wiring** -- exported symbol analysis
   - Tests: `test_cross_package_wiring`
   - Files: `scripts/dev/validate.py`
   - Verify: detects symbols called only from same package

4. **Phase: Spec completeness** -- AC table parsing
   - Tests: `test_spec_ac_completeness`
   - Files: `scripts/dev/validate.py`
   - Verify: detects empty AC "Demonstrated By" cells

5. **Phase: Functional test coverage** -- .ci VALIDATES cross-ref
   - Tests: (manual verification)
   - Files: `scripts/dev/validate.py`
   - Verify: flags ACs without .ci coverage

6. **Phase: Make target + integration**
   - Tests: `test-validate-clean.ci`
   - Files: `Makefile`, `mk/validate.mk`
   - Verify: `make ze-validate` works

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All 6 check categories implemented |
| False positives | Run on current codebase, verify no spurious findings |
| Performance | Runs in < 5s on full repo |
| Exit codes | 0 = clean, 1 = findings, 2 = script error |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `make ze-validate` target | `make ze-validate` runs without error |
| Line-number anchor detection | Plant a `:47` anchor, verify caught |
| Cross-package wiring detection | Create unexported-only export, verify caught |
| Spec AC completeness | Leave an AC empty, verify caught |
| No false positives on clean tree | Run on committed code, exit 0 |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Path traversal | Script reads files by path; verify no user input flows to open() |
| Resource exhaustion | Large repos; verify no unbounded recursion |

### Documentation Update Checklist
| Category | Applies | File + Update |
|----------|---------|---------------|
| Feature list | No | Developer tool, not user feature |
| CLI reference | No | Make target, not CLI command |
| Architecture | No | Tooling, not architecture |
| Test infrastructure | Yes | `docs/functional-tests.md` -- mention ze-validate as post-verify check |

## Implementation Summary

### What Was Implemented
- `scripts/dev/validate.py`: 5 check functions (source anchor line numbers, stale anchor paths, cross-package wiring, spec AC completeness, CLI handler coverage)
- `scripts/dev/validate_test.py`: 11 unit tests covering all checks plus clean-pass and edge cases
- `Makefile`: `ze-validate` target with `.PHONY` declaration and help text
- `docs/functional-tests.md`: documented ze-validate as post-verify check

### Deviations from Plan
- Skipped `mk/validate.mk`: target is a single line, added directly to Makefile
- Skipped `test/plugin/validate-clean.ci`: the `.ci` format is for BGP protocol testing via `ze-test`; a Makefile target test does not fit that framework. Coverage provided by `validate_test.py` unit tests and `make ze-validate` integration
- Source anchor stale-path check skips URLs (`https://...`) and paths without `/` to avoid false positives on external references and inline documentation examples
- Exit code 2 reserved for script errors (not in original spec exit code table which had 0/1 only)

## Implementation Audit

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | done | `test_detects_line_number_anchor` | Planted `internal/foo.go:47` anchor detected |
| AC-2 | done | `test_detects_missing_file` | Non-existent path detected |
| AC-3 | done | `test_detects_same_package_only_export` | `UnusedExport` flagged when only test-file caller |
| AC-4 | done | `test_detects_empty_demonstrated_by` | Empty AC-2 cell detected in in-progress spec |
| AC-5 | done | `check_cli_handler_coverage` | Changed CLI files with `MustRegister*` checked against `.ci` content |
| AC-6 | done | `test_no_findings_on_clean_tree`, `make ze-validate` exits 0 | Clean tree produces no findings |
| AC-7 | done | `make ze-validate` | Target exists, invokes script, exits 0 |

### Audit Summary
- **Total items:** 7/7 complete

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Catches line-number anchors | unit test | `test_detects_line_number_anchor` finds `*.go:47` |
| Catches unwired exports | unit test | `test_detects_same_package_only_export` finds `UnusedExport` |
| Catches incomplete spec ACs | unit test | `test_detects_empty_demonstrated_by` finds empty AC-2 cell |
| No false positives | integration | `make ze-validate` exits 0 on real codebase |
| Fast (< 5s) | timing | 0.2s measured on real codebase |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | False positive: URL source anchor | `docs/comparison.md:316` | Fixed: skip URLs and paths without `/` |
| 2 | ISSUE | False positive: placeholder `...` in doc table | `docs/contributing/documentation-testing.md` | Fixed: skip paths without `/` |
| 3 | ISSUE | Unused import `os` | `validate.py` | Removed |
| 4 | ISSUE | Unused constant `DIM` | `validate.py` | Removed |
| 5 | ISSUE | Test assertion checked wrong string | `test_detects_line_number_anchor` | Fixed: include anchor path in message |

### Final status
- [x] All findings fixed, `make ze-validate` exits 0, all 11 tests pass
