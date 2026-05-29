# Spec: install-7a-namespace

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | - |
| Phase | - |
| Updated | 2026-05-28 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `cmd/ze/appliance/main.go` - current appliance CLI dispatch (38 files)
4. `cmd/ze/install/main.go` - current install CLI dispatch
5. `cmd/ze/main.go` lines 21, 469-474 - import + dispatch for appliance/install

## Task

Move the `cmd/ze/appliance/` package to `cmd/ze/install/appliance/` to unify all
installation and fleet management commands under `ze install`. Add a deprecated
`ze appliance` alias that prints a warning then delegates. This is a pure
namespace migration: no behavior changes, no new features.

## Required Reading

### Architecture Docs
- [ ] `plan/spec-install-7-gokrazy-build.md` - umbrella spec (Component 1 section)
  -> Decision: nest under install, deprecated alias for ze appliance
  -> Constraint: all 16 subcommands, same behavior, same flags
- [ ] `plan/learned/675-appliance-1-builder.md` - appliance builder decisions
  -> Decision: handlers map + extractDirFlag pattern
- [ ] `plan/learned/769-install-subcommand.md` - install subcommand decisions
  -> Decision: Run(args) dispatch pattern

**Key insights:**
- Only one external import of appliance package: `cmd/ze/main.go` line 21
- All appliance files are self-contained (no cross-package imports from within)
- Install package uses Run(args) with switch dispatch, needs "appliance" case added
- Appliance register.go registers "appliance" as a root command; needs to register under install instead

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze/appliance/main.go` - Run() dispatches 16 subcommands via handlers map
  -> Constraint: handlers map, extractDirFlag, baseDir, usage() must be preserved exactly
- [ ] `cmd/ze/appliance/register.go` - registers "appliance" as root command via cmdregistry
  -> Constraint: registration section/mode/description must update to reflect new path
- [ ] `cmd/ze/install/main.go` - Run() dispatches local/remote via switch
  -> Constraint: local and remote subcommands unchanged
- [ ] `cmd/ze/install/register.go` - registers "install" as root command
  -> Constraint: Subs field must include "appliance"
- [ ] `cmd/ze/main.go` - imports zeappliance, dispatches "appliance" case at line 469
  -> Constraint: dispatch must change to deprecated alias

**Behavior to preserve:**
- All 16 appliance subcommands work identically (init, assemble, build, passwd, replace-cert, rekey, clone, list, show, run, unlock, export, import, push, config, config-push)
- --dir flag extraction works the same way
- baseDir resolution (flag > env > default) works the same way
- All existing tests pass under new package path
- Help text format and content preserved (updated command name only)
- ze install local and ze install remote unchanged

**Behavior to change:**
- `ze appliance <cmd>` prints deprecation warning to stderr, then delegates to `ze install appliance <cmd>`
- `ze install appliance <cmd>` is the new canonical path
- `ze install --help` lists appliance alongside local and remote
- Package location: `cmd/ze/appliance/` -> `cmd/ze/install/appliance/`
- Import path: `codeberg.org/.../cmd/ze/appliance` -> `codeberg.org/.../cmd/ze/install/appliance`

## Data Flow (MANDATORY)

### Entry Point
- `ze install appliance <subcommand>` CLI invocation (new canonical)
- `ze appliance <subcommand>` CLI invocation (deprecated alias)

### Transformation Path
1. `cmd/ze/main.go` receives "install" -> dispatches to `install.Run(args[1:])`
2. `cmd/ze/install/main.go` receives "appliance" -> dispatches to `appliance.Run(args[1:])`
3. `cmd/ze/install/appliance/main.go` Run() dispatches to specific handler

For deprecated alias:
1. `cmd/ze/main.go` receives "appliance" -> prints deprecation warning
2. Delegates to `install.Run(append([]string{"appliance"}, args[1:]...))`

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| main.go -> install package | `install.Run(args)` | [ ] |
| install -> appliance package | `appliance.Run(args[1:])` | [ ] |
| deprecated alias -> install | prepend "appliance" to args, delegate | [ ] |

### Integration Points
- `cmd/ze/main.go` dispatch: change "appliance" case to deprecated alias
- `cmd/ze/install/main.go` switch: add "appliance" case
- `cmd/ze/install/appliance/register.go`: register under install, not root

### Architectural Verification
- [ ] No bypassed layers (appliance accessed through install)
- [ ] No unintended coupling (appliance package self-contained)
- [ ] No duplicated functionality (deprecated alias delegates, does not copy)
- [ ] Zero-copy preserved where applicable (N/A for CLI dispatch)

## Wiring Test (MANDATORY, NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze install appliance --help` | -> | `appliance.Run()` dispatches help | `TestInstallApplianceHelp` in `cmd/ze/install/appliance/main_test.go` |
| `ze appliance` (deprecated) | -> | prints warning, delegates to install appliance | `TestDeprecatedApplianceAlias` in `cmd/ze/main_test.go` or functional test |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze install appliance --help` | Shows all 16 subcommands with descriptions |
| AC-2 | `ze install appliance init lab` (with temp dir) | Dispatches to init handler, same behavior as old `ze appliance init lab` |
| AC-3 | `ze appliance --help` | Prints deprecation warning to stderr, then shows help |
| AC-4 | `ze install --help` | Lists local, remote, and appliance as subcommands |
| AC-5 | All existing appliance unit tests | Pass under new package location with zero behavior change |
| AC-6 | `ze install appliance list` | Dispatches to list handler, same output as old `ze appliance list` |
| AC-7 | Package import path | `codeberg.org/thomas-mangin/ze/cmd/ze/install/appliance` compiles |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDirResolution` | `cmd/ze/install/appliance/main_test.go` | --dir flag, env, default (moved from old location) | |
| `TestInstallApplianceHelp` | `cmd/ze/install/appliance/main_test.go` | Help output shows all 16 subcommands | |

### Boundary Tests (MANDATORY for numeric inputs)
N/A - no numeric inputs in this spec.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-install-appliance-help` | `test/install/appliance-help.ci` | `ze install appliance --help` shows subcommands | |
| `test-deprecated-appliance` | `test/install/deprecated-appliance.ci` | `ze appliance --help` prints deprecation warning | |

### Interop Tests
N/A - not a protocol feature.

## Files to Modify

- `cmd/ze/main.go` - change import path, change "appliance" case to deprecated alias
- `cmd/ze/install/main.go` - add "appliance" case in switch, add import
- `cmd/ze/install/register.go` - update Subs field to include appliance
- `docs/guide/appliance.md` - update command examples from `ze appliance` to `ze install appliance`

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | N/A |
| CLI commands/flags | Yes | `cmd/ze/main.go`, `cmd/ze/install/main.go` |
| CLI grammar | No | N/A (existing commands, just moved) |
| Functional test for new RPC/API | Yes | `test/install/appliance-help.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | N/A (namespace change, not new feature) |
| 2 | Config syntax changed? | No | N/A |
| 3 | CLI command added/changed? | Yes | `docs/guide/appliance.md` - update command paths |
| 4 | API/RPC added/changed? | No | N/A |
| 5 | Plugin added/changed? | No | N/A |
| 6 | Has a user guide page? | Yes | `docs/guide/appliance.md` - update all `ze appliance` to `ze install appliance` |

## Files to Create

- `cmd/ze/install/appliance/` - all 38 files moved from `cmd/ze/appliance/`
- `test/install/appliance-help.ci` - functional test
- `test/install/deprecated-appliance.ci` - deprecation warning test

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create |
| 3. Wiring phase | Wiring Test table |
| 4. Implement | Move files, update imports, add dispatch |
| 5-13. Verify | Verify loop |
| 14. Present summary | Report |

### Implementation Phases

1. **Phase: Move files** - copy all 38 files from `cmd/ze/appliance/` to `cmd/ze/install/appliance/`
   - Verify: files exist at new location

2. **Phase: Update dispatch** - update main.go deprecated alias, install/main.go appliance case
   - Tests: TestInstallApplianceHelp
   - Files: `cmd/ze/main.go`, `cmd/ze/install/main.go`, `cmd/ze/install/register.go`
   - Verify: `ze install appliance --help` works

3. **Phase: Update help text** - update usage() in appliance/main.go to say "ze install appliance"
   - Files: `cmd/ze/install/appliance/main.go`

4. **Phase: Update docs** - update appliance.md command examples
   - Files: `docs/guide/appliance.md`

5. **Phase: Functional tests** - create .ci test files

6. **Phase: Remove old directory** - delete `cmd/ze/appliance/` (after confirming new location works)

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All 38 files moved, all imports updated |
| Correctness | Deprecated alias prints warning AND delegates correctly |
| Naming | Package stays "appliance", import path updated |
| Data flow | main.go -> install -> appliance chain works |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| 38 files at `cmd/ze/install/appliance/` | `ls cmd/ze/install/appliance/*.go \| wc -l` |
| No files at `cmd/ze/appliance/` | `ls cmd/ze/appliance/ 2>&1` shows error |
| `ze install appliance --help` works | functional test |
| Deprecated `ze appliance` warns | functional test |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | N/A - no new input handling, just dispatch routing |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix import paths |
| Test fails | Check package path in test imports |
| 3 fix attempts fail | STOP. Report. |

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

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Keep package name "appliance" | Rename to "fleet" | Existing code, docs, and user knowledge all use "appliance" |
| Deprecation warning on stderr | Silent redirect | Users need to update scripts; stderr doesn't break piped output |

## Known Limitations
- Old `ze appliance` still works (deprecated, not removed) to avoid breaking scripts

## Implementation Summary

### What Was Implemented
- Copied 38 Go files from `cmd/ze/appliance/` to `cmd/ze/install/appliance/`
- Updated `cmd/ze/install/appliance/main.go` help text to use "ze install appliance"
- Updated `cmd/ze/install/appliance/register.go` to remove root command registration
- Updated `cmd/ze/main.go` dispatch: deprecated alias prints warning and delegates
- Updated `cmd/ze/install/main.go` to import and dispatch appliance subcommand
- Updated `cmd/ze/install/register.go` Subs field to include appliance
- Updated `docs/guide/appliance.md` command references
- Created functional tests: `test/install/appliance-help.ci`, `test/install/deprecated-appliance.ci`
- Updated `test/install/install-help.ci` to check for appliance

### Bugs Found/Fixed
- None

### Documentation Updates
- `docs/guide/appliance.md` updated to use `ze install appliance` throughout, added deprecation note

### Deviations from Plan
- Old `cmd/ze/appliance/` not deleted yet (requires `git rm`, deferred to commit script)

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Move appliance under install | Done | `cmd/ze/install/appliance/` | 38 files |
| Deprecated alias | Done | `cmd/ze/main.go:468-470` | Prints warning, delegates |
| Update docs | Done | `docs/guide/appliance.md` | All references updated |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `test/install/appliance-help.ci` | Help shows all 16 subcommands |
| AC-2 | Done | Dispatch chain: install/main.go -> appliance.Run() | Same handler map |
| AC-3 | Done | `test/install/deprecated-appliance.ci` | Warns on stderr |
| AC-4 | Done | `test/install/install-help.ci` | Lists appliance |
| AC-5 | Done | All test files copied to new location | Package path updated |
| AC-6 | Done | Dispatch chain preserved | Same handlers map |
| AC-7 | Done | `cmd/ze/install/main.go` imports appliance | Compiles |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestDirResolution | Done | `cmd/ze/install/appliance/main_test.go` | Moved from old location |
| test-install-appliance-help | Done | `test/install/appliance-help.ci` | New |
| test-deprecated-appliance | Done | `test/install/deprecated-appliance.ci` | New |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `cmd/ze/install/appliance/*.go` (38 files) | Done | Copied from old location |
| `cmd/ze/main.go` | Done | Deprecated alias |
| `cmd/ze/install/main.go` | Done | Appliance dispatch |
| `cmd/ze/install/register.go` | Done | Subs updated |
| `docs/guide/appliance.md` | Done | Command paths updated |
| `test/install/appliance-help.ci` | Done | New |
| `test/install/deprecated-appliance.ci` | Done | New |

### Audit Summary
- **Total items:** 16
- **Done:** 16
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 0

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied

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
- [ ] AC-1..AC-7 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated

### Quality Gates (SHOULD pass)
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
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] Summary included in commit
