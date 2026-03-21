# Spec: Shell Completion v2 -- Dynamic Value Completions

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-03-21 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `cmd/ze/completion/` - current completion implementation
4. `cmd/ze/cli/` - command tree construction

## Task

Ze has shell completion for bash, zsh, and fish (subcommand dispatch + YANG-driven dynamic
words via `ze completion words`). The current system completes **command names** well but
does not complete **argument values** -- peer addresses, family names, flag values, config
keys, or schema module names at tab time.

This spec covers extending completion to provide contextual value suggestions:

- Peer addresses (from running daemon via socket query)
- Address family names (from registry: `ipv4/unicast`, `ipv6/vpn`, etc.)
- Log levels (`debug`, `info`, `warn`, `err`, `disabled`)
- Config keys (from YANG schema tree)
- Flag values (`--json`, `--text`, `--no-header` where applicable)
- File paths (for config file arguments)
- Schema module names (from `ze schema list`)

The goal: a user typing `ze bgp rib show <TAB>` sees peer addresses; typing
`ze bgp rib show --family <TAB>` sees family names; typing `ze config edit <TAB>`
sees config section paths.

### Inspiration

rustbgpd pre-generates completions for bash/zsh/fish. Ze's dynamic approach via
`ze completion words` is already superior for command names. This spec extends that
advantage to argument values, which rustbgpd does not complete dynamically either.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/architecture.md` - API command structure
  -> Decision:
  -> Constraint:
- [ ] `docs/architecture/api/wire-format.md` - how CLI communicates with daemon
  -> Decision:
  -> Constraint:

### RFC Summaries (MUST for protocol work)
- N/A (not protocol work)

**Key insights:**
- To be filled during design phase

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze/completion/main.go` - dispatch: bash/zsh/fish/words subcommands
- [ ] `cmd/ze/completion/words.go` - dynamic word generation from `cli.BuildCommandTree()`
- [ ] `cmd/ze/completion/bash.go` - bash script with static command lists + dynamic callbacks
- [ ] `cmd/ze/completion/zsh.go` - zsh script with `_arguments` patterns
- [ ] `cmd/ze/completion/fish.go` - fish script with `complete` commands
- [ ] `cmd/ze/cli/` - `BuildCommandTree()` constructs command tree from registered RPCs
- [ ] `cmd/ze/internal/cmdutil/` - `DescribeCommand()` for completion descriptions
- [ ] `internal/component/bgp/message/family.go` - family constant definitions

**Behavior to preserve:**
- All existing subcommand completions (bash, zsh, fish)
- `ze completion words show|run [path...]` interface
- Dynamic YANG-driven command discovery
- Installation methods (`eval "$(ze completion bash)"`, file redirect)

**Behavior to change:**
- `ze completion words` only returns child command names, not argument values
- Bash/zsh/fish scripts have no hooks for value completion after flags
- No daemon connectivity for live data (peer names, route counts)

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- Shell tab key triggers completion function
- Completion function calls `ze completion words <mode> <path...>` or flag-specific helper
- For live data: completion function calls `ze bgp peer --json` (or similar) via socket

### Transformation Path
1. Shell detects cursor position and current word
2. Shell completion function determines context (subcommand vs flag vs value)
3. For static values: built-in word list in completion script (families, log levels)
4. For dynamic values: `ze completion words` extended to return value completions
5. For live data: separate `ze` invocation querying the running daemon

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Shell -> ze binary | exec `ze completion words` with context args | [ ] |
| ze binary -> daemon | SSH/socket query for live data (peer names) | [ ] |
| YANG schema -> completion | Registry query for families, modules | [ ] |

### Integration Points
- `cli.BuildCommandTree()` - extend to include value hints per argument position
- `completion/words.go` - extend `words` subcommand to return values, not just commands
- Each shell script - add flag-value completion logic

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze completion bash` | -> | bash script with value completion hooks | TBD |
| `ze completion zsh` | -> | zsh script with value completion hooks | TBD |
| `ze completion fish` | -> | fish script with value completion hooks | TBD |
| `ze completion words values <context>` | -> | `words.go` value resolver | TBD |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze bgp peer <TAB>` (daemon running) | Lists peer addresses from running session |
| AC-2 | `ze bgp rib show --family <TAB>` | Lists registered address families |
| AC-3 | `ze bgp log set <TAB>` | Lists log levels: debug, info, warn, err, disabled |
| AC-4 | `ze config edit <TAB>` | Lists top-level config sections |
| AC-5 | `ze schema show <TAB>` | Lists YANG module names |
| AC-6 | `ze completion words values families` | Outputs registered family names |
| AC-7 | Completions work offline (no daemon) for static values | Family names, log levels, flag values always available |
| AC-8 | Completions degrade gracefully when daemon unreachable | No error output, just no live suggestions |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestWordsValuesFamilies` | `cmd/ze/completion/words_test.go` | AC-6: family list output | |
| `TestWordsValuesLogLevels` | `cmd/ze/completion/words_test.go` | AC-3: log level output | |
| `TestWordsValuesConfigSections` | `cmd/ze/completion/words_test.go` | AC-4: config section output | |
| `TestWordsValuesSchemaModules` | `cmd/ze/completion/words_test.go` | AC-5: schema module output | |
| `TestBashValueCompletionHooks` | `cmd/ze/completion/main_test.go` | AC-1,2: bash script contains value hooks | |
| `TestGracefulDegradation` | `cmd/ze/completion/words_test.go` | AC-8: no error when daemon down | |

### Boundary Tests (MANDATORY for numeric inputs)
- N/A (no numeric inputs in completion)

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-completion-values` | `test/ui/completion-values.ci` | `ze completion words values families` outputs family names | |

### Future (if deferring any tests)
- Live daemon completion (AC-1) may require integration test with running ze instance

## Files to Modify
- `cmd/ze/completion/words.go` - extend with `values` subcommand
- `cmd/ze/completion/bash.go` - add value completion hooks per context
- `cmd/ze/completion/zsh.go` - add value completion via `_arguments` specs
- `cmd/ze/completion/fish.go` - add value completion via `complete -a` args
- `cmd/ze/completion/main.go` - potentially add new dispatch paths

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [ ] | N/A |
| RPC count in architecture docs | [ ] | N/A |
| CLI commands/flags | [x] | `cmd/ze/completion/` |
| CLI usage/help text | [x] | `ze completion --help` |
| API commands doc | [ ] | N/A |
| Plugin SDK docs | [ ] | N/A |
| Editor autocomplete | [ ] | N/A |
| Functional test for new RPC/API | [x] | `test/ui/completion-values.ci` |

## Files to Create
- `test/ui/completion-values.ci` - functional test for value completions

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan -- check what exists |
| 3. Implement (TDD) | Implementation phases below |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report per `rules/planning.md` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Static value completions** -- extend `words.go` with `values` subcommand for families, log levels, config sections
   - Tests: `TestWordsValuesFamilies`, `TestWordsValuesLogLevels`, `TestWordsValuesConfigSections`
   - Files: `words.go`
   - Verify: tests fail -> implement -> tests pass
2. **Phase: Shell script hooks** -- add value completion logic to bash/zsh/fish scripts
   - Tests: `TestBashValueCompletionHooks`
   - Files: `bash.go`, `zsh.go`, `fish.go`
   - Verify: tests fail -> implement -> tests pass
3. **Phase: Live daemon queries** -- optional peer address completion when daemon reachable
   - Tests: `TestGracefulDegradation`
   - Files: `words.go`, shell scripts
   - Verify: tests fail -> implement -> tests pass
4. **Functional tests** -> Create after feature works. Cover user-visible behavior.
5. **Full verification** -> `make ze-verify` (lint + all ze tests except fuzz)
6. **Complete spec** -> Fill audit tables, write learned summary to `docs/learned/NNN-<name>.md`, delete spec from `docs/plan/`. BLOCKING: summary is part of the commit, not a follow-up.

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Family names match `make ze-inventory` output exactly |
| Naming | Completion output uses kebab-case family format (afi/safi) |
| Data flow | Static values never require daemon connection |
| Rule: no-layering | New value completions extend existing `words` mechanism, not parallel |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| `ze completion words values families` outputs family list | Run command, check output |
| `ze completion words values log-levels` outputs level list | Run command, check output |
| Bash script contains value completion hooks | grep for `values` in bash output |
| Zsh script contains value completion hooks | grep for `values` in zsh output |
| Fish script contains value completion hooks | grep for `values` in fish output |
| Graceful degradation when daemon down | Kill daemon, run completion, no stderr |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | Completion context args sanitized before passing to daemon query |
| No command injection | Shell scripts escape user input in completion functions |
| Timeout | Daemon queries have short timeout (1-2s) to avoid hanging tab completion |

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

## RFC Documentation

N/A -- not protocol work.

## Implementation Summary

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered -- add test for each]

### Documentation Updates
- [Docs updated, or "None"]

### Deviations from Plan
- [Differences from original plan and why]

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
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

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
- [ ] AC-1..AC-N all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
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

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `docs/learned/NNN-<name>.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary. One commit = code + tests + summary.
