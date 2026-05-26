# Spec: Configurable Default CLI Output Format

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/4 |
| Updated | 2026-05-26 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/rules/pipe-completeness.md` - pipe operator requirements
4. `ai/rules/config-design.md` - YANG config patterns
5. `ai/patterns/config-option.md` - config option pipeline
6. `internal/component/command/pipe.go` - pipe operator implementation
7. `internal/component/config/apply_env.go` - YANG env plumbing
8. `internal/component/hub/schema/ze-hub-conf.yang` - environment YANG schema

## Task

Add a configurable default output format for CLI commands. Currently the default is
hardcoded to `pipeTable` in `ProcessPipesDefaultTable` and `ProcessPipesDetectLog`.
Users should be able to set their preferred format via config:

```
environment {
    cli {
        format {
            default text;
        }
    }
}
```

Supported values: text (default), table, json, yaml, ndjson.

Additionally, provide a session-level `set cli format <fmt>` command in
operational mode that overrides the config default (process-global via env.Set).

## Required Reading

### Architecture Docs
- [ ] `ai/rules/pipe-completeness.md` - pipe operator requirements
  -> Constraint: every command must route through ApplyPipes or ProcessPipes* wrapper
  -> Constraint: format operators are mutually exclusive (one per pipe chain)
- [ ] `ai/rules/config-design.md` - YANG config patterns
  -> Constraint: every YANG environment/ leaf MUST have matching env.MustRegister()
  -> Constraint: augment only for cross-component; same-component uses grouping
- [ ] `ai/patterns/config-option.md` - config option end-to-end pipeline
  -> Constraint: pipeline is YANG leaf -> module registration -> env.MustRegister -> envPlumbingTable -> functional test
  -> Constraint: defaults come from YANG, not Go
- [ ] `docs/architecture/api/commands.md` - CLI pipe operators
  -> Decision: pipe operators are client-side filters applied after command execution

### RFC Summaries (MUST for protocol work)
N/A - no protocol work.

**Key insights:**
- Default format is hardcoded to pipeTable in two ProcessPipes* functions
- Three callers: model_mode.go:153 (all operational), model_ping.go:213, model_traceroute.go:277
- command package currently has no env dependency; adding env.Get is a leaf dependency
- environment/ leaves follow: YANG -> extractSections -> envPlumbingTable -> env.Get
- env.Get/Set is thread-safe, per-process (process-global session override is acceptable)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/command/pipe.go` - defines pipe operators, ProcessPipes* functions
  -> Constraint: pipeKind enum (text, table, json, yaml, ndjson, match, count, resolve, origin, log, no-more)
  -> Constraint: ProcessPipesDefaultTable hardcodes pipeTable as default (line 456)
  -> Constraint: ProcessPipesDetectLog hardcodes pipeTable, overrides to pipeText for resolve/origin (lines 432-437)
  -> Constraint: HasFormatOp checks json, ndjson, table, text, yaml
- [ ] `internal/component/cli/model_mode.go` - executeOperationalCommand calls ProcessPipesDefaultTable (line 153)
  -> Constraint: Model struct has commandExecutor injected, no config/env access currently
- [ ] `internal/component/cli/model_ping.go` - calls ProcessPipesDetectLog (line 213)
- [ ] `internal/component/cli/model_traceroute.go` - calls ProcessPipesDetectLog (line 277)
- [ ] `internal/component/cli/model_keys.go` - handleEnter dispatches operational commands (line 524-552)
  -> Constraint: local commands (clear, stop, restart) intercepted before executeOperationalCommand
- [ ] `internal/component/config/environment.go` - env var registrations for environment/ leaves
  -> Constraint: env.MustRegister with Key, Type, Default, Description
- [ ] `internal/component/config/constants.go` - extractSections lists environment sub-containers
  -> Constraint: must add "cli" to extractSections for ExtractEnvironment to walk it
- [ ] `internal/component/config/apply_env.go` - envPlumbingTable maps YANG to env vars
  -> Constraint: must add cli.format -> ze.cli.format entry
- [ ] `internal/component/hub/schema/ze-hub-conf.yang` - environment YANG container
  -> Constraint: environment has daemon, log, chaos, exabgp sub-containers; no cli yet
- [ ] `internal/core/env/env.go` - centralized env var lookup
  -> Constraint: Get/Set are thread-safe, case-insensitive, separator-agnostic
  -> Constraint: mustBeRegistered panics on unregistered key

**Behavior to preserve:**
- All existing pipe operators and their parsing
- Explicit `| json`, `| table` etc. always wins over any default
- resolve/origin heuristic: when no explicit pipe and default is text, resolve/origin still picks text
- ProcessPipes (no default) and ProcessPipesDefaultFunc signatures unchanged

**Behavior to change:**
- ProcessPipesDefaultTable: read configured default instead of hardcoding pipeTable
- ProcessPipesDetectLog: read configured default instead of hardcoding pipeTable
- Rename ProcessPipesDefaultTable -> ProcessPipesDefaultFormat (and update callers)
- Add `set cli format <fmt>` command in operational mode

## Data Flow (MANDATORY)

### Entry Point -- Config Default
- User writes `environment { cli { format { default json; } } }` in config file
- Parsed by config loader into Tree

### Transformation Path -- Config Default
1. `ExtractEnvironment(tree)` walks environment/ containers including new "cli" section
2. Returns `map["cli.format"]["default"] = "json"`
3. `ApplyEnvConfig` matches via envPlumbingTable: `{section: "cli.format", option: "default", envKey: "ze.cli.format"}`
4. Calls `env.Set("ze.cli.format", "json")`
5. At command time, `configuredDefault()` in pipe.go calls `env.Get("ze.cli.format")` -> pipeJSON
6. ProcessPipesDefaultFormat/ProcessPipesDetectLog use configuredDefault() instead of pipeTable

### Entry Point -- Session Override
- User types `set cli format yaml` in operational mode
- handleEnter intercepts before executeOperationalCommand

### Transformation Path -- Session Override
1. handleEnter detects `set cli format <value>`
2. Validates value against allowed set (text, table, json, yaml, ndjson)
3. Calls `env.Set("ze.cli.format", "yaml")`
4. Subsequent commands pick up new default via configuredDefault()

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config -> Env | envPlumbingTable + ApplyEnvConfig | [ ] |
| Env -> Pipe formatting | env.Get in configuredDefault() | [ ] |
| CLI input -> Env | handleEnter intercept + env.Set | [ ] |

### Integration Points
- `ExtractEnvironment` - existing function, walks new "cli" section automatically via extractSections
- `ApplyEnvConfig` - existing function, plumbs new entry via envPlumbingTable
- `ProcessPipesDefaultFormat` (renamed) - reads default via configuredDefault()
- `ProcessPipesDetectLog` - reads default via configuredDefault()
- `handleEnter` - intercepts new `set cli format` command

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config `environment { cli { format { default json; } } }` | -> | `env.Get("ze.cli.format")` returns "json" | `TestApplyEnvConfigCLIFormat` in `apply_env_test.go` |
| `ProcessPipesDefaultFormat("peer list")` with env set | -> | `configuredDefault()` returns configured pipeKind | `TestProcessPipesDefaultFormat_Configured` in `pipe_test.go` |
| `set cli format yaml` in operational mode | -> | `env.Set("ze.cli.format", "yaml")` | `TestSetCLIFormat` in `model_commands_test.go` |
| Config parse of `environment { cli { format { default json; } } }` | -> | Tree contains cli.format.default | functional test `test/editor/cli-format-default.et` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config has `environment { cli { format { default json; } } }` | Commands without explicit pipe produce JSON output |
| AC-2 | Config has `environment { cli { format { default table; } } }` | Commands without explicit pipe produce table output |
| AC-3 | Config has `environment { cli { format { default yaml; } } }` | Commands without explicit pipe produce YAML output |
| AC-4 | Config has `environment { cli { format { default ndjson; } } }` | Commands without explicit pipe produce NDJSON output |
| AC-5 | Config has `default json`, user types `show bgp peer \| table` | Explicit pipe wins: table output |
| AC-6 | No config, no session override | Default is text (YANG default) |
| AC-7 | User types `set cli format json` in operational mode | Subsequent commands default to JSON |
| AC-8 | User types `set cli format invalid` in operational mode | Error message listing valid values |
| AC-9 | Config has `default json`, command uses `\| resolve` without explicit format | resolve/origin heuristic yields to config default (JSON, not text) |
| AC-10 | YANG schema validation | `environment { cli { format { default bogus; } } }` rejected at parse time |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestConfiguredDefault` | `internal/component/command/pipe_test.go` | configuredDefault() maps env value to pipeKind, falls back to pipeText | |
| `TestConfiguredDefaultInvalid` | `internal/component/command/pipe_test.go` | configuredDefault() falls back to pipeText for invalid env value | |
| `TestProcessPipesDefaultFormat_Configured` | `internal/component/command/pipe_test.go` | ProcessPipesDefaultFormat uses configured default instead of pipeTable | |
| `TestProcessPipesDetectLog_ConfiguredDefault` | `internal/component/command/pipe_test.go` | ProcessPipesDetectLog uses configured default | |
| `TestProcessPipesDetectLog_ResolveWithConfiguredJSON` | `internal/component/command/pipe_test.go` | resolve/origin yields to non-text config default | |
| `TestApplyEnvConfigCLIFormat` | `internal/component/config/apply_env_test.go` | cli.format.default plumbed to ze.cli.format env var | |
| `TestSetCLIFormat` | `internal/component/cli/model_commands_test.go` | `set cli format json` sets env var, shows confirmation | |
| `TestSetCLIFormatInvalid` | `internal/component/cli/model_commands_test.go` | `set cli format bogus` returns error | |
| `TestSetCLIFormatShow` | `internal/component/cli/model_commands_test.go` | `set cli format` (no value) shows current setting | |

### Boundary Tests (MANDATORY for numeric inputs)
N/A - no numeric inputs (enum-valued leaf).

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `cli-format-default` | `test/editor/cli-format-default.et` | Config sets default format; verify operational command output uses it | |

### Interop Tests (MANDATORY for protocol features)
N/A - no protocol work.

### Future (if deferring any tests)
None.

## Files to Modify
- `internal/component/hub/schema/ze-hub-conf.yang` - add cli/format container under environment
- `internal/component/config/environment.go` - register ze.cli.format env var
- `internal/component/config/constants.go` - add "cli" to extractSections
- `internal/component/config/apply_env.go` - add cli.format plumbing entry to envPlumbingTable
- `internal/component/command/pipe.go` - add configuredDefault(), rename ProcessPipesDefaultTable, update defaults
- `internal/component/cli/model_mode.go` - update caller to use renamed function
- `internal/component/cli/model_keys.go` - intercept `set cli format` in handleEnter
- `internal/component/cli/model_render.go` - add `set cli format` to help text (if pipe help is listed)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [x] | `internal/component/hub/schema/ze-hub-conf.yang` |
| CLI commands/flags | [x] | `internal/component/cli/model_keys.go` (session override) |
| CLI grammar (action before identifier) | [x] | `set cli format <value>` follows action-before-identifier |
| Editor autocomplete | [ ] | YANG-driven (automatic for config editor) |
| Functional test for new RPC/API | [x] | `test/editor/cli-format-default.et` |
| Doctor check for runtime dependencies | [ ] | No runtime dependencies added |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` - add CLI output format config |
| 2 | Config syntax changed? | [x] | `docs/guide/configuration.md` - environment cli format section |
| 3 | CLI command added/changed? | [x] | `docs/guide/command-reference.md` - set cli format |
| 4 | API/RPC added/changed? | [ ] | N/A |
| 5 | Plugin added/changed? | [ ] | N/A |
| 6 | Has a user guide page? | [ ] | Could add to existing pipe/output guide |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [ ] | N/A |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [ ] | N/A |
| 12 | Internal architecture changed? | [ ] | N/A |

## Files to Create
- `test/editor/cli-format-default.et` - functional test for configurable default format

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan -- check what exists |
| 3. Wiring phase | Wiring Test table -- YANG leaf, env var, plumbing entry |
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

1. **Phase: Wiring (MANDATORY FIRST)** -- YANG leaf + env registration + plumbing
   - Tests: `TestApplyEnvConfigCLIFormat`
   - Files: `ze-hub-conf.yang`, `environment.go`, `constants.go`, `apply_env.go`
   - Verify: env var is set when config is loaded with cli format block

2. **Phase: Pipe default** -- configuredDefault() + rename ProcessPipesDefaultTable
   - Tests: `TestConfiguredDefault`, `TestConfiguredDefaultInvalid`, `TestProcessPipesDefaultFormat_Configured`, `TestProcessPipesDetectLog_ConfiguredDefault`, `TestProcessPipesDetectLog_ResolveWithConfiguredJSON`
   - Files: `pipe.go`, `model_mode.go`
   - Verify: ProcessPipesDefaultFormat uses configured default; explicit pipes still win

3. **Phase: Session override** -- `set cli format` command
   - Tests: `TestSetCLIFormat`, `TestSetCLIFormatInvalid`, `TestSetCLIFormatShow`
   - Files: `model_keys.go`, `model_commands_test.go`
   - Verify: set command updates env var; invalid values rejected

4. **Functional tests** -- Create after feature works
   - Tests: `test/editor/cli-format-default.et`
   - Verify: end-to-end config -> output format

5. **Documentation** -- Update feature docs, command reference
   - Files: `docs/features.md`, `docs/guide/command-reference.md`

6. **Full verification** -- `make ze-verify`

7. **Complete spec** -- Fill audit tables, write learned summary, delete spec

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | configuredDefault() falls back to pipeText for empty/invalid env value |
| Naming | YANG uses kebab-case (default), env var uses dots (ze.cli.format) |
| Data flow | config -> ExtractEnvironment -> ApplyEnvConfig -> env.Set -> env.Get -> pipe default |
| CLI grammar | `set cli format <value>` follows action-before-identifier |
| Rule: pipe-completeness | Renamed function still called at all 3 sites |
| Rule: config-design | env.MustRegister matches YANG leaf |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| YANG cli/format container | `grep "container cli" internal/component/hub/schema/ze-hub-conf.yang` |
| Env var registered | `grep "ze.cli.format" internal/component/config/environment.go` |
| extractSections has "cli" | `grep '"cli"' internal/component/config/constants.go` |
| Plumbing entry | `grep "cli.format" internal/component/config/apply_env.go` |
| configuredDefault function | `grep "configuredDefault" internal/component/command/pipe.go` |
| ProcessPipesDefaultTable renamed | `grep -r "ProcessPipesDefaultTable" internal/` returns 0 matches |
| ProcessPipesDefaultFormat exists | `grep "ProcessPipesDefaultFormat" internal/component/command/pipe.go` |
| Session override handler | `grep "set cli format" internal/component/cli/model_keys.go` |
| Functional test | `ls test/editor/cli-format-default.et` |
| Unit tests pass | `go test ./internal/component/command/ ./internal/component/config/ ./internal/component/cli/` |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Input validation | `set cli format` validates against fixed enum; no injection risk |
| Config validation | YANG enum rejects unknown values at parse time |
| No sensitive data | format preference is not sensitive |

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

- env.Get in command package is a leaf dependency; keeps ProcessPipes* signatures simpler than caller-passed default
- Process-global session override via env.Set is acceptable since environment/ values are process-scoped by design
- resolve/origin heuristic should yield to explicit config preference (not just to explicit pipe)

## RFC Documentation

N/A - no protocol work.

## Implementation Summary

### What Was Implemented
- [pending]

### Bugs Found/Fixed
- [pending]

### Documentation Updates
- [pending]

### Deviations from Plan
- [pending]

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
| Config default format used by CLI commands | functional test | `test/editor/cli-format-default.et` |
| Session override works | unit test | `TestSetCLIFormat` |
| Explicit pipe wins over default | unit test | `TestProcessPipesDefaultFormat_Configured` |

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

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
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
- [ ] Interop tests for protocol features (or N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary. One commit = code + tests + summary.
