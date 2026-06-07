# Spec: gomu-mutation-testing

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/5 |
| Updated | 2026-06-07 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/rules/testing.md` - test infrastructure, make targets, iteration workflow
4. `ai/rules/discovery-updates.md` - must update discovery paths for new tools
5. `mk/test-unit.mk` - pattern for component-group test targets
6. `scripts/dev/changed-pkgs.sh` - changed-package detection
7. `scripts/dev/changed-groups.sh` - changed-group detection

## Task

Integrate gomu (`github.com/sivchari/gomu`) mutation testing into Ze as an advisory development and CI tool.

gomu is a Go mutation testing tool that uses the `go -overlay` flag (no in-place file modification), supports parallel workers, incremental analysis via SHA256 + git diff, and has 7 mutation operators (arithmetic, conditional, logical, bitwise, branch, return value, error handling). MIT license, Go 1.21+, v0.2.1 (May 2026).

Goals:
1. Makefile targets for mutation testing (full, changed-only, per-component group)
2. `.gomuignore` for excluding generated code, vendor, platform-specific files
3. Integration with existing `changed-pkgs.sh` / `changed-groups.sh` infrastructure
4. Sensible defaults (threshold, workers, timeout)
5. Advisory-only initially (not gating `ze-verify`)
6. Developer documentation and discovery updates

Constraints:
- gomu is young (< 1 year, single maintainer, 30 stars); integration must be resilient to gomu failures
- Must handle Ze's build tag system (files with `//go:build ze_test`, `ze_chaos`, etc.)
- Must not slow down the existing verify pipeline
- Per-package or per-change scoping essential given Ze's scale (486 packages, ~5 min full test)
- CGO_ENABLED=0 compatibility required

## Required Reading

### Architecture Docs
- [ ] `ai/rules/testing.md` - test infrastructure, make targets, component groups
  → Constraint: component groups (bgp, core, plugins, config, cli, rest) define scoped test targets; mutation targets must mirror this pattern
  → Constraint: `GO_TEST` uses `GOMAXPROCS=$(GO_TEST_PROCS)` with ncpu-3; mutation workers should be informed by this
  → Decision: `make ze-verify` is the pre-commit gate; mutation testing is NOT part of verify
- [ ] `ai/rules/discovery-updates.md` - must update discovery paths for new tools
  → Constraint: new make targets must appear in `ai/INDEX.md` Dev Tools section
  → Constraint: new test infrastructure must appear in `ai/rules/testing.md`
- [ ] `ai/rules/bash-output.md` - no pipes on expensive commands
  → Constraint: mutation test output must not be piped; write to log files in `tmp/`
- [ ] `mk/test-unit.mk` - component-group test target pattern
  → Constraint: groups use `$(GO_TEST) -race $(ZE_GROUP_XXX)` pattern; mutation targets mirror naming
  → Constraint: ZE_GROUP_* variables define package sets; mutation targets reuse them
- [ ] `scripts/dev/changed-pkgs.sh` - changed package detection
  → Constraint: outputs `./`-prefixed existing package directories, sorted/deduped
  → Constraint: uses verify-status baseline SHA for committed-since-last-green detection
- [ ] `scripts/dev/changed-groups.sh` - changed-group detection
  → Constraint: `--pkgs` mode outputs Go package patterns; default mode outputs group names
  → Constraint: maps `PREFIX_GROUP` associations; unmapped files go to "rest"

### RFC Summaries (MUST for protocol work)
N/A - tooling infrastructure, not protocol work.

**Key insights:**
- Component groups (bgp ~1:30, core ~30s, plugins ~40s, config ~20s, cli ~10s, rest ~1:00) are the natural scoping unit; mutation testing will multiply these times by mutation count per package
- `changed-groups.sh --pkgs` provides changed Go package patterns suitable for scoped mutation runs
- gomu's `--incremental` flag + `--base-branch` maps directly to Ze's changed-file workflow
- Ze's `GO_TEST_PROCS` (ncpu-3) should inform gomu's `--workers` default
- **gomu has NO build tag support**: no `--tags` flag, no `//go:build` inspection, no passthrough to `go test`. Files with build constraints must be excluded via `.gomuignore` path patterns. This means `cmd/ze/` files using `ze_test`, `ze_chaos`, `ze_perf`, `ze_analyze` tags get zero mutation coverage unless gomu adds `--tags` support.
- gomu uses `go build -overlay` and `go test -overlay` internally; the overlay approach is safe for parallel workers but build-tag conflicts in `cmd/ze/` will cause compilation failures without `.gomuignore` exclusion
- gomu's default 30s per-test timeout may be too short for bgp group tests (~1:30 total suite); `--timeout` should be set higher
- gomu writes reports to the current directory by default; Ze should redirect to `tmp/` to keep the working tree clean

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `mk/test-unit.mk` - component-group test targets pattern
  → Constraint: groups use `$(GO_TEST) -race $(ZE_GROUP_XXX)` pattern
  → Constraint: ZE_GROUP_BGP = `./internal/component/bgp/...`, ZE_GROUP_CORE = `./internal/core/...`, etc.
  → Constraint: ZE_GROUP_REST is computed by exclusion (go list minus named groups)
- [ ] `scripts/dev/changed-pkgs.sh` - changed package detection via git diff + verify baseline
  → Constraint: outputs `./`-prefixed existing package directories, sorted/deduped
- [ ] `scripts/dev/changed-groups.sh` - maps changed files to component groups
  → Constraint: `--pkgs` mode outputs Go package patterns; default mode outputs group names
- [ ] `Makefile` - defines GO_TEST, GO_TEST_PROCS, ZE_PACKAGES
  → Constraint: ZE_PACKAGES = `go list ./... | grep -v root`; GO_TEST_PROCS = ncpu - 3 (min 1)
  → Constraint: GO_TEST = `GOMAXPROCS=$(GO_TEST_PROCS) go test`

**Behavior to preserve:**
- All existing make targets unchanged
- `make ze-verify` pipeline unaffected (mutation testing is separate)
- `changed-pkgs.sh` / `changed-groups.sh` used read-only (no modifications)

**Behavior to change:**
- None - this is additive only

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- Developer runs `make ze-mutation-test` (or scoped variant)
- gomu CLI invoked with configured flags

### Transformation Path
1. Makefile target resolves package scope (all, changed, or component group)
2. Check if gomu is installed (`command -v gomu`); if not, print advisory and exit 0
3. gomu invoked with `--workers`, `--timeout`, `--threshold`, `--output` flags
4. gomu reads `.gomuignore`, excludes matching files
5. gomu parses Go AST, generates mutations (overlay-based, no file modification)
6. gomu runs `go test -overlay` per mutation
7. gomu produces report (console + JSON to `tmp/`)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Make -> gomu CLI | Shell invocation with flags | [ ] |
| gomu -> go test | `-overlay` flag on `go build`/`go test` | [ ] |
| gomu -> filesystem | `.gomuignore` read, report write to `tmp/` | [ ] |

### Integration Points
- `changed-pkgs.sh` output feeds gomu package list for scoped runs
- `changed-groups.sh --pkgs` feeds gomu for group-scoped runs
- `GO_TEST_PROCS` informs `--workers` default

### Architectural Verification
- [ ] No bypassed layers (gomu invoked via make, not directly)
- [ ] No unintended coupling (mutation testing is advisory, never blocks verify)
- [ ] No duplicated functionality (extends existing test infra, doesn't recreate)
- [ ] Zero-copy preserved where applicable (N/A - tooling only)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `make ze-mutation-test` | -> | `mk/test-mutation.mk` target | `make -n ze-mutation-test` exits 0 |
| `make ze-mutation-changed` | -> | `mk/test-mutation.mk` + `changed-pkgs.sh` | `make -n ze-mutation-changed` exits 0 |
| `.gomuignore` | -> | gomu reads it | Manual: `gomu run --dry-run` shows excluded files |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `make ze-mutation-test` on a clean checkout | gomu runs on all non-excluded packages, produces report in `tmp/mutation-report.json` |
| AC-2 | `make ze-mutation-changed` with changed .go files | gomu runs only on packages with changes (via `changed-pkgs.sh`), report in `tmp/` |
| AC-3 | ~~`make ze-mutation-bgp`~~ | Dropped: gomu has no package-path CLI support; scans entire module. Per-group targets would all run the same scope. |
| AC-4 | Files with build tags (`ze_test`, `ze_chaos`, `linux`) | Excluded from mutation via `.gomuignore` patterns |
| AC-5 | gomu not installed | Make target prints advisory message and exits 0 (not a blocking failure) |
| AC-6 | `ai/INDEX.md` Dev Tools section | Lists mutation testing make targets |
| AC-7 | `ai/rules/testing.md` Make Targets section | Documents mutation testing targets |
| AC-8 | gomu produces HTML report | `make ze-mutation-report` generates `tmp/mutation-report.html` |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| N/A - Makefile targets | `mk/test-mutation.mk` | Validated by `make -n` dry-run | |

### Boundary Tests (MANDATORY for numeric inputs)
N/A - no numeric inputs in this integration.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Verify make target syntax | Manual: `make -n ze-mutation-test` | Target parses without error | |
| Verify .gomuignore excludes tagged files | Manual: gomu dry-run | Files with build tags not listed | |
| Verify graceful fallback | Manual: uninstall gomu, run target | Advisory message, exit 0 | |

### Interop Tests (MANDATORY for protocol features)
N/A - tooling infrastructure, not protocol.

### Future (if deferring any tests)
- CI integration (Woodpecker pipeline) deferred until gomu proves stable in local use
- Threshold enforcement deferred until mutation score baseline established
- Upstream `--tags` flag contribution deferred; `.gomuignore` workaround is sufficient for now

## Files to Modify
- `Makefile` - include `mk/test-mutation.mk`
- `ai/INDEX.md` - add mutation testing to Dev Tools and keyword map
- `ai/rules/testing.md` - add mutation testing section

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | No | N/A |
| YANG validation constraints | No | N/A |
| YANG custom validators | No | N/A |
| CLI commands/flags | No | N/A |
| CLI grammar (action before identifier) | No | N/A |
| Editor autocomplete | No | N/A |
| Functional test for new RPC/API | No | N/A |
| Pipe completeness | No | N/A |
| Env var registration | No | N/A |
| Doctor check for runtime dependencies | No | gomu is optional; missing gomu handled gracefully in make target |
| Prometheus counters/metrics | No | N/A |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | N/A |
| 2 | Config syntax changed? | No | N/A |
| 3 | CLI command added/changed? | No | N/A |
| 4 | API/RPC added/changed? | No | N/A |
| 5 | Plugin added/changed? | No | N/A |
| 6 | Has a user guide page? | No | N/A |
| 7 | Wire format changed? | No | N/A |
| 8 | Plugin SDK/protocol changed? | No | N/A |
| 9 | RFC behavior implemented? | No | N/A |
| 10 | Test infrastructure changed? | Yes | `ai/rules/testing.md` - add Mutation Testing section |
| 11 | Affects daemon comparison? | No | N/A |
| 12 | Internal architecture changed? | No | N/A |
| 13 | Route metadata keys added/changed? | No | N/A |
| 14 | Prometheus counters added/changed? | No | N/A |
| 15 | Registered plugin, event type, send type, command, capability, or runtime inventory changed? | No | N/A |
| 16 | Any changed source file is referenced by existing doc source anchors? | No | N/A |
| 17 | Existing docs show config/CLI/API examples for this area? | No | N/A |

## Files to Create
- `mk/test-mutation.mk` - mutation testing make targets
- `.gomuignore` - gomu exclusion patterns

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table - Makefile include, make target existence |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist below |
| 8. Fix issues | Fix every issue from critical review |
| 9. Re-verify | Re-run stage 6 |
| 10. Repeat 7-9 | Until clean |
| 11. Deliverables review | Deliverables Checklist below |
| 12. Security review | Security Review Checklist below |
| 13. Re-verify | Re-run stage 6 |
| 14. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** -- create mk file, include in Makefile
   - Tests: `make -n ze-mutation-test` parses without error
   - Files: `mk/test-mutation.mk`, `Makefile` (add include)
   - Verify: target exists but gomu not yet configured with all options

2. **Phase: .gomuignore** -- create exclusion patterns
   - Tests: file exists, patterns cover build-tagged files, generated code, vendor, tmp
   - Files: `.gomuignore`
   - Verify: patterns are correct for Ze's layout

3. **Phase: Full targets** -- implement all mutation make targets
   - Tests: `make -n ze-mutation-test`, `make -n ze-mutation-changed`, `make -n ze-mutation-bgp`
   - Files: `mk/test-mutation.mk`
   - Verify: targets invoke gomu with correct flags, scoping, output paths

4. **Phase: Graceful fallback** -- handle missing gomu
   - Tests: without gomu installed, targets print advisory and exit 0
   - Files: `mk/test-mutation.mk`
   - Verify: exit code 0, clear message

5. **Phase: Discovery updates** -- documentation and indexes
   - Tests: grep `ai/INDEX.md` for mutation, grep `ai/rules/testing.md` for mutation
   - Files: `ai/INDEX.md`, `ai/rules/testing.md`
   - Verify: agents can discover mutation testing from standard navigation

6. **Full verification** -- `make ze-verify`
7. **Complete spec** -- fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Make targets invoke gomu with correct flags; .gomuignore patterns match intended files |
| Naming | Make targets follow `ze-mutation-*` pattern consistent with `ze-unit-test-*`, `ze-test-*` |
| Data flow | Changed-pkgs/groups output feeds gomu correctly (quoting, empty-set handling) |
| Graceful degradation | Missing gomu exits 0 with advisory message, not error |
| Discovery | `ai/INDEX.md` and `ai/rules/testing.md` updated |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| `mk/test-mutation.mk` exists | `ls mk/test-mutation.mk` |
| `.gomuignore` exists | `ls .gomuignore` |
| Makefile includes mutation mk | `grep 'test-mutation.mk' Makefile` |
| `make -n ze-mutation-test` parses | `make -n ze-mutation-test` exits 0 |
| `ai/INDEX.md` updated | `grep -i mutation ai/INDEX.md` |
| `ai/rules/testing.md` updated | `grep -i mutation ai/rules/testing.md` |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Input validation | Make targets pass package paths to gomu; verify no shell injection via changed-pkgs.sh output |
| gomu trust | gomu is installed by user (`go install`), not downloaded at runtime; no supply chain risk from make targets |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

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

## Core Insight

gomu's overlay-based architecture makes it the only Go mutation testing tool safe for parallel execution without file corruption risk. Its lack of build tag support is the primary limitation, but `.gomuignore` path exclusion is an acceptable workaround because Ze's interesting mutation targets (logic, encoding, protocol) live in `internal/` where build tags are rare.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Thin mk wrapper (Approach A) | Wrapper script (Approach B) | Follows existing test-unit.mk pattern; logic visible in mk file |
| Advisory-only, not gating | Gate on mutation score threshold | gomu too young; false positives would erode trust; establish baseline first |
| Reuse ZE_GROUP_* variables | Define mutation-specific groups | Same packages, no reason to diverge |
| `--timeout 120` default | gomu default 30s | bgp group takes ~1:30 for full test run; 30s would timeout most mutations |
| `--workers $(GO_TEST_PROCS)` | Fixed worker count | Matches existing CPU reservation policy (ncpu-3) |
| Reports to `tmp/` | Current directory | Keep working tree clean; `tmp/` is gitignored |
| Exclude `cmd/ze/`, `*_linux.go`, `*_darwin.go` | Include everything | Build tag compilation failures without `--tags`; platform files fail cross-platform |
| Upstream `--tags` PR as follow-up | Block on support / ignore | Small change (2 lines in engine.go); .gomuignore workaround covers initial integration |

## Known Limitations
- gomu has NO build tag support: no `--tags` flag, no `//go:build` inspection. Files with build constraints excluded via `.gomuignore` path patterns only. `cmd/ze/` files using `ze_test`, `ze_chaos`, `ze_perf`, `ze_analyze` tags get zero mutation coverage.
- gomu is young (< 1 year, single maintainer, 30 stars); may need to be replaced or forked
- Only 7 mutation operators; more comprehensive tools exist but have worse architecture
- No per-test-function scoping (runs full package test suite per mutation)
- 30-second default timeout may be too short for some Ze packages; overridden in targets
- `_linux.go` files will cause gomu compilation failures on macOS unless excluded

## RFC Documentation

N/A - tooling infrastructure, not protocol.

## Implementation Summary

### What Was Implemented
- `mk/test-mutation.mk` with 3 targets: `ze-mutation-test`, `ze-mutation-changed`, `ze-mutation-report`
- `.gomuignore` excluding `cmd/ze/`, generated code, vendor, gokrazy modcache, test/, tmp/, internal/appliance/
- Graceful fallback when gomu not installed (advisory message, exit 0)
- Discovery updates in `ai/INDEX.md` (Dev Tools + keyword map) and `ai/rules/testing.md` (Mutation Testing section)

### Bugs Found/Fixed
- Make `define`/`$(call)` pattern caused `exit 0` to exit only the subshell, not the recipe. Fixed by using single-shell-block recipes with `;\ ` continuations.

### Documentation Updates
- `ai/rules/testing.md`: added Mutation Testing section and 3 targets to Verification Targets table
- `ai/INDEX.md`: added 3 mutation targets to Dev Tools table and keyword map entry

### Deviations from Plan
- AC-3 (per-component-group targets) dropped: gomu has no package-path CLI support; it always scans the entire module. Per-group targets would all run the same scope.
- AC-8 renamed from `ze-mutation-report` "generates" to "runs full mutation test with HTML output" (gomu must re-run mutations, not re-render from JSON)

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
| Mutation testing available via make | Manual test | `make -n ze-mutation-test` |
| Changed-only scoping works | Manual test | `make -n ze-mutation-changed` with dirty tree |
| Component group scoping works | Manual test | `make -n ze-mutation-bgp` |
| Graceful degradation | Manual test | Uninstall gomu, run target |
| Discovery updated | grep | `grep mutation ai/INDEX.md ai/rules/testing.md` |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- [pending]

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

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

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated (`mk/test-mutation.mk`, `.gomuignore`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (N/A)
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (N/A)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/<spec>` only
