# Spec: test-diagnostics

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/test/runner/report.go` - failure reporting
4. `internal/test/runner/json.go` - JSON comparison
5. `internal/test/runner/parsing.go` - parse test runner
6. `Makefile` lines 74-87 (functional-test), 108-114 (fuzz)

## Task

Improve test diagnostic output so that every failure gives the developer a clear, copy-pasteable path to resolution. Currently, test failures require scrolling, guessing, and manual hex truncation to debug. Seven concrete issues need fixing.

### Issue Summary

| # | Issue | Impact | File |
|---|-------|--------|------|
| 1 | Fuzz tests broken — `-fuzz=.` matches multiple tests in attribute package | Tests never run; silent CI gap | `Makefile:111` |
| 2 | Debug commands truncated to 64 chars — not copy-pasteable | Developer must manually find full hex | `report.go:280` |
| 3 | Generic failure has no structured diagnosis | Raw output dump; no guidance | `report.go:239-268` |
| 4 | No "likely cause" hints for common failure patterns | Developer must memorize patterns | `report.go` |
| 5 | JSON comparison shows two full dumps, no field-level diff | Must visually scan large blobs | `json.go:195-197` |
| 6 | Functional test suite summary doesn't name failed suites | Must scroll up to find which suite failed | `Makefile:83` |
| 7 | Parse test failures have no reproduction command | Developer must reconstruct the command | `parsing.go:317-322` |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/ci-format.md` - test runner framework design
  → Constraint: functional test output must follow ci-format conventions

### Source Files
- [ ] `internal/test/runner/report.go` (330L) - failure report generation; three report types (timeout, mismatch, generic)
  → Decision: debug commands use `ze bgp decode update <hex>` format
  → Constraint: report output goes to `r.output` (configurable writer), not hardcoded stdout
- [ ] `internal/test/runner/json.go` (280L) - JSON comparison with normalize but no field-level diff
  → Constraint: `comparePluginJSON()` returns error with diff description
- [ ] `internal/test/runner/parsing.go` (409L) - parse test runner with `ze validate` invocation
  → Constraint: failure callback at line 317 prints name + error, no reproduction command
- [ ] `internal/test/runner/display.go` (433L) - live TTY status + summary line
  → Decision: summary format uses `═══ PASS/FAIL` prefix
- [ ] `Makefile` lines 74-87 — functional test target with suite counting
  → Constraint: uses shell `failed` counter, prints count but not names
- [ ] `Makefile` lines 108-114 — fuzz test targets using `-fuzz=.`
  → Constraint: Go `-fuzz` flag only accepts patterns matching exactly one fuzz test

**Key insights:**
- Report struct writes to configurable `io.Writer` — testable without stdout capture
- Fuzz target for attribute package has 7 tests; `-fuzz=.` matches all 7, Go refuses
- wireu, storage, pool packages each have 1-2 fuzz tests, so `-fuzz=.` works there
- Makefile functional test already counts failed suites but doesn't track names
- JSON comparison already normalizes both sides — adding field-level diff is straightforward
- Parse test failure callback has access to `test.File` (config path) for reproduction

## Current Behavior (MANDATORY)

**Source files read:**
- [x] `internal/test/runner/report.go` - Three report types: timeout (progress + last/next message), mismatch (expected vs received + hex diff), generic (error + raw output dump). Debug commands section truncates hex to 64 chars with "..."
- [x] `internal/test/runner/json.go` - `comparePluginJSON()` returns error with two full JSON blobs separated by "Expected:" / "Actual:" labels
- [x] `internal/test/runner/parsing.go` - Failure callback prints "X name: error" and optional output; no `ze validate <path>` reproduction command
- [x] `Makefile` - `ze-fuzz-test` uses `go test -fuzz=. -fuzztime=10s` per package; attribute package has 7 fuzz tests causing multi-match failure. `ze-functional-test` counts failures but doesn't track suite names
- [x] `internal/test/runner/display.go` - Summary line shows pass/fail/rate/duration per-test in encode/decode suites

**Behavior to preserve:**
- Report structure: header, config/CI file info, type-specific body, debug commands, separator
- Color scheme: red for failures, yellow for labels, cyan for identifiers, gray for hints
- Timeout report: progress section with expected/received counts, last received, expected next
- Mismatch report: expected vs received with semantic BGP decode + hex diff
- Display summary format with `═══ PASS/FAIL` prefix
- All output goes through `r.output` writer (not hardcoded stdout)

**Behavior to change:**
- Debug command hex: show full hex, not truncated to 64 chars
- Generic report: add structured diagnosis with likely cause hints
- JSON mismatch: show field-level diff instead of two full dumps
- Fuzz Makefile target: enumerate individual fuzz tests for attribute package
- Functional test summary: track and name which suites failed
- Parse test failure: include `ze validate <path>` reproduction command

## Data Flow (MANDATORY)

### Entry Point
- Test runner discovers `.ci` files and runs BGP encode/decode/plugin/parse tests
- Each test produces a `Record` (encode/decode/plugin) or `ParsingTest` (parse)
- On failure, `Report.PrintFailure(rec)` or the parse failure callback is invoked

### Transformation Path
1. Test execution populates `Record` fields: `Messages`, `ReceivedRaw`, `Error`, `PeerOutput`, `ClientOutput`, `FailureType`
2. `PrintFailure` dispatches to type-specific report (timeout/mismatch/generic)
3. Type-specific report formats fields into colored, structured output
4. `printDebugCommands` appends copy-pasteable commands
5. For JSON tests, `comparePluginJSON` returns error message with diff description
6. For parse tests, failure callback prints error line
7. Makefile collects per-suite exit codes into `failed` counter

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Test runner → Report | `Record` struct with all failure data | [x] |
| Report → stdout | Via `r.output` writer | [x] |
| Makefile → test binaries | Exit codes (0 = pass, non-zero = fail) | [x] |

### Integration Points
- `Report.PrintFailure()` — main entry for all failure output
- `comparePluginJSON()` — returns error with diff description
- `ParsingRunner.SetOnFail()` callback — parse test failure output
- Makefile `ze-fuzz-test` target — fuzz test invocation
- Makefile `ze-functional-test` target — suite-level orchestration

### Architectural Verification
- [x] No bypassed layers — all changes in existing report/runner code
- [x] No unintended coupling — each fix is self-contained
- [x] No duplicated functionality — extends existing output, doesn't recreate
- [x] Zero-copy preserved — no wire encoding changes

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `make ze-fuzz-test` with attribute package containing 7 fuzz tests | All 7 fuzz tests run individually without "matches more than one fuzz test" error |
| AC-2 | Mismatch failure with hex longer than 64 chars | Debug commands show full hex, directly copy-pasteable to terminal |
| AC-3 | Generic failure (non-timeout, non-mismatch) | Output includes structured sections: error, likely cause hint, raw output |
| AC-4 | Timeout failure with empty client output | "Likely cause" section suggests common reasons (missing feature, wrong config, process crash) |
| AC-5 | JSON mismatch between expected and actual | Error shows which fields differ (added/removed/changed), not two full dumps |
| AC-6 | `make ze-functional-test` with one suite failing | Final summary names the failed suite(s), not just count |
| AC-7 | Parse test failure (positive test fails) | Output includes `ze validate <config-path>` reproduction command |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDebugCommandsFullHex` | `internal/test/runner/report_test.go` | AC-2: debug commands contain full hex, no truncation | |
| `TestGenericReportStructured` | `internal/test/runner/report_test.go` | AC-3: generic report has ERROR + LIKELY CAUSE + output sections | |
| `TestLikelyCauseTimeout` | `internal/test/runner/report_test.go` | AC-4: timeout report includes likely cause hints | |
| `TestJSONDiffFieldLevel` | `internal/test/runner/json_test.go` | AC-5: JSON mismatch error names differing fields | |

### Boundary Tests (MANDATORY for numeric inputs)

No numeric range inputs in this spec — all changes are string formatting and output structure.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Fuzz target execution | `make ze-fuzz-test` | AC-1: all fuzz tests run successfully | |
| Suite summary naming | `make ze-functional-test` (manual) | AC-6: failed suite names visible in summary | |
| Parse repro command | `make ze-parse-test` (manual with bad config) | AC-7: reproduction command shown on failure | |

## Files to Modify
- `Makefile` - Fix fuzz target (AC-1), fix functional test suite naming (AC-6)
- `internal/test/runner/report.go` - Full hex in debug commands (AC-2), structured generic report with likely cause hints (AC-3, AC-4)
- `internal/test/runner/json.go` - Field-level JSON diff (AC-5)
- `internal/test/runner/parsing.go` - Add reproduction command to failure callback (AC-7)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | |
| RPC count in architecture docs | No | |
| CLI commands/flags | No | |
| CLI usage/help text | No | |
| API commands doc | No | |
| Plugin SDK docs | No | |
| Editor autocomplete | No | |
| Functional test for new RPC/API | No | |

## Files to Create
- `internal/test/runner/report_test.go` - Unit tests for report improvements (AC-2, AC-3, AC-4)
- `internal/test/runner/json_test.go` - Unit tests for JSON field-level diff (AC-5) — only if file doesn't exist yet

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

### Step 1: Fix fuzz Makefile target (AC-1)

Replace `go test -fuzz=. -fuzztime=10s ./internal/plugins/bgp/attribute/...` with individual invocations for each of the 7 fuzz tests in the attribute package. The other packages (wireu, storage, pool) have 1-2 fuzz tests each and can keep `-fuzz=.`.

→ **Review:** Does `make ze-fuzz-test` complete without "matches more than one" errors?

### Step 2: Write unit tests for report improvements (AC-2, AC-3, AC-4)

Create `report_test.go` with tests that construct `Record` structs and verify output via `Report.SetOutput()`. Tests should FAIL before implementation.

→ **Review:** Do tests fail for the right reasons? Are edge cases covered (empty hex, no messages, no client output)?

### Step 3: Fix debug command truncation (AC-2)

In `printDebugCommands`, remove the `[:min(64, ...)]+"..."` truncation. Print full hex for both expected and received messages.

→ **Review:** Are commands directly copy-pasteable? No trailing "..."?

### Step 4: Add structured generic report with likely cause hints (AC-3, AC-4)

Restructure `printGenericReport` to add a LIKELY CAUSE section that pattern-matches on common failure types. Add similar hints to timeout report.

Common failure patterns to detect:
- Empty client output → "Client produced no output — may have crashed or missing feature"
- Error contains "connection refused" → "Server not listening — check config address/port"
- Error contains "exec:" → "Binary not found — check build"
- Timeout with 0 received → "No messages received — check connectivity or OPEN negotiation"
- Timeout with some received → "Partial exchange — check message N expectations"

→ **Review:** Are hints actionable? Do they avoid false positives?

### Step 5: Write unit test for JSON field-level diff (AC-5)

Create test in `json_test.go` that verifies field-level diff output when JSON objects differ.

→ **Review:** Does test fail before implementation?

### Step 6: Implement field-level JSON diff (AC-5)

Replace the two-full-dump error in `comparePluginJSON` with a field-level diff showing added, removed, and changed fields.

→ **Review:** Is the diff output concise? Does it handle nested objects?

### Step 7: Fix functional test suite naming in Makefile (AC-6)

Track suite names alongside the failure counter in `ze-functional-test`. Print failed suite names in the summary line.

→ **Review:** Does the summary clearly identify which suite(s) failed?

### Step 8: Add parse test reproduction command (AC-7)

In `parsing.go` failure callback, print `ze validate <config-path>` as a reproduction command.

→ **Review:** Is the path correct (accounts for both file-based and inline configs)?

### Step 9: Verify all

Run `make ze-lint && make ze-unit-test && make ze-functional-test && make ze-fuzz-test`. Paste output.

→ **Review:** Zero lint issues? All tests deterministic?

### Failure Routing

| Failure | Symptom | Route To |
|---------|---------|----------|
| Fuzz still fails multi-match | "matches more than one" | Step 1 — enumerate missed test |
| Report test fails wrong | Setup error, not behavior | Step 2 — fix test construction |
| Lint failure | New code triggers linter | Fix inline, re-run |
| JSON diff too verbose | Nested objects explode diff | Step 6 — add depth limit |

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

## Implementation Summary

### What Was Implemented

### Bugs Found/Fixed

### Documentation Updates

### Deviations from Plan

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Fix broken fuzz tests | | | |
| Full-length debug commands | | | |
| Structured generic failure report | | | |
| Likely cause hints | | | |
| Field-level JSON diff | | | |
| Suite-level failure naming | | | |
| Parse reproduction command | | | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | | | |
| AC-2 | | | |
| AC-3 | | | |
| AC-4 | | | |
| AC-5 | | | |
| AC-6 | | | |
| AC-7 | | | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestDebugCommandsFullHex | | | |
| TestGenericReportStructured | | | |
| TestLikelyCauseTimeout | | | |
| TestJSONDiffFieldLevel | | | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| Makefile | | |
| internal/test/runner/report.go | | |
| internal/test/runner/json.go | | |
| internal/test/runner/parsing.go | | |
| internal/test/runner/report_test.go | | |
| internal/test/runner/json_test.go | | |

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Checklist

### Goal Gates (MUST pass)
- [ ] Acceptance criteria AC-1..AC-7 all demonstrated
- [ ] Tests pass (`make ze-unit-test`)
- [ ] No regressions (`make ze-functional-test`)
- [ ] Fuzz tests run (`make ze-fuzz-test`)
- [ ] Integration test: fuzz tests proven to execute from `make ze-fuzz-test` (not just in isolation)

### Quality Gates (SHOULD pass)
- [ ] `make ze-lint` passes
- [ ] Implementation Audit fully completed
- [ ] Mistake Log escalation candidates reviewed

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)

### Documentation (during implementation)
- [ ] Required docs read

### Completion (after tests pass)
- [ ] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] All files committed together
