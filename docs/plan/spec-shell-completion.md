# Spec: shell-completion

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `.claude/rules/cli-patterns.md` - CLI dispatch patterns
4. `cmd/ze/main.go` - top-level dispatch and usage
5. `cmd/ze/completion/main.go` - implementation file

## Task

Add `ze completion bash` and `ze completion zsh` commands that output shell completion scripts to stdout. The completion scripts provide static completions for the CLI command tree and dynamic YANG-driven completions for plugin names and schema modules by calling back to `ze` subcommands at completion time.

## Required Reading

### Architecture Docs
- [ ] `.claude/rules/cli-patterns.md` - CLI dispatch patterns
  -> Constraint: Each domain has `func Run(args []string) int`. Own `flag.NewFlagSet`. Exit codes 0/1/2.
- [ ] `docs/architecture/config/yang-config-design.md` - YANG schema system
  -> Decision: YANG is the single source of truth for config schema and RPC definitions.

**Key insights:**
- Static command tree is hardcoded in Go dispatch (main.go, bgp/main.go, config/main.go, etc.)
- Dynamic data comes from YANG: `ze --plugins --json` (plugin names), `ze schema list` (module names)
- Existing pattern: `ze config completion` already queries YANG for config editor completions

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze/main.go` - top-level dispatch: bgp, config, cli, validate, schema, plugin, exabgp, signal, version, help, --plugins
- [ ] `cmd/ze/bgp/main.go` - subcommands: decode, encode
- [ ] `cmd/ze/config/main.go` - subcommands: edit, check, migrate, fmt, dump, diff, completion
- [ ] `cmd/ze/schema/main.go` - subcommands: list, show, handlers, methods, events, protocol
- [ ] `cmd/ze/plugin/main.go` - dynamic dispatch via registry.Lookup(args[0])
- [ ] `cmd/ze/signal/main.go` - subcommands: reload, stop, quit, status
- [ ] `cmd/ze/exabgp/main.go` - subcommands: plugin, migrate
- [ ] `cmd/ze/cli/main.go` - subcommands: bgp (default)
- [ ] `cmd/ze/validate/main.go` - flags: -v, -q, --json
- [ ] `cmd/ze/hub/main.go` - not a user command (internal dispatcher)

**Behavior to preserve:**
- All existing CLI dispatch and exit code patterns
- `ze --plugins --json` output format (used by completion scripts)

**Behavior to change:**
- Add `completion` to top-level dispatch in `cmd/ze/main.go`
- Add `completion` to usage text

## Data Flow (MANDATORY)

### Entry Point
- User runs `ze completion bash` or `ze completion zsh`
- Shell receives script on stdout

### Transformation Path
1. `cmd/ze/main.go` dispatches to `completion.Run(args[1:])`
2. `cmd/ze/completion/main.go` checks first arg for "bash" or "zsh"
3. Generates shell script with static command tree + dynamic callbacks
4. Outputs to stdout

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Go -> Shell | Script output to stdout | [ ] |
| Shell -> Ze (dynamic) | Completion script calls `ze --plugins --json`, `ze schema list` at tab-completion time | [ ] |

### Integration Points
- `cmd/ze/main.go` dispatch switch — add "completion" case
- `cmd/ze/main.go` usage() — add completion to command list

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling (completion package has zero internal imports)
- [ ] No duplicated functionality
- [ ] Zero-copy N/A (no wire encoding)

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze completion bash` CLI | -> | `completion.Run(["bash"])` | `TestCompletionBashOutput` |
| `ze completion zsh` CLI | -> | `completion.Run(["zsh"])` | `TestCompletionZshOutput` |
| Top-level dispatch | -> | `completion.Run` | Functional test `test/completion/bash.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze completion bash` | Outputs bash completion script to stdout, exit 0 |
| AC-2 | `ze completion zsh` | Outputs zsh completion script to stdout, exit 0 |
| AC-3 | Bash script content | Contains all top-level commands (bgp, config, cli, validate, schema, plugin, exabgp, signal, completion, version) |
| AC-4 | Bash script content | Contains subcommand completions (bgp: decode/encode, config: edit/check/migrate/fmt/dump/diff/completion, etc.) |
| AC-5 | Bash script dynamic | Script calls `ze --plugins --json` for `ze plugin <TAB>` completions |
| AC-6 | `ze completion` with no shell arg | Shows usage, exit 1 |
| AC-7 | `ze completion unknown` | Shows error + usage, exit 1 |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRunBash` | `cmd/ze/completion/main_test.go` | Bash output contains COMPREPLY, _ze function | |
| `TestRunZsh` | `cmd/ze/completion/main_test.go` | Zsh output contains compdef, _ze function | |
| `TestRunNoArgs` | `cmd/ze/completion/main_test.go` | No args returns exit 1 | |
| `TestRunUnknown` | `cmd/ze/completion/main_test.go` | Unknown shell returns exit 1 | |
| `TestBashContainsCommands` | `cmd/ze/completion/main_test.go` | All top-level commands present | |
| `TestBashContainsSubcommands` | `cmd/ze/completion/main_test.go` | bgp/config/schema subcommands present | |
| `TestZshContainsCommands` | `cmd/ze/completion/main_test.go` | All top-level commands present | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-bash-output` | `test/completion/bash.ci` | `ze completion bash` outputs valid script | |
| `test-zsh-output` | `test/completion/zsh.ci` | `ze completion zsh` outputs valid script | |
| `test-no-args` | `test/completion/no-args.ci` | `ze completion` shows usage | |

## Files to Modify
- `cmd/ze/main.go` - add completion dispatch + usage text

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A |
| CLI commands/flags | Yes | `cmd/ze/main.go` |
| CLI usage/help text | Yes | `cmd/ze/main.go` |

## Files to Create
- `cmd/ze/completion/main.go` - completion subcommand
- `cmd/ze/completion/main_test.go` - unit tests
- `test/completion/bash.ci` - functional test
- `test/completion/zsh.ci` - functional test
- `test/completion/no-args.ci` - functional test

## Implementation Steps

1. **Write unit tests** -> Review: covers bash, zsh, error cases
2. **Run tests** -> Verify FAIL
3. **Implement completion package** -> Static command tree + dynamic callbacks
4. **Run tests** -> Verify PASS
5. **Wire into main.go** -> Add dispatch + usage
6. **Functional tests** -> .ci files
7. **Verify all** -> `make ze-verify`

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |

### Failed Approaches
| Approach | Why abandoned | Replacement |

## Design Insights

## Implementation Summary

### What Was Implemented

### Documentation Updates

### Deviations from Plan

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |

### Tests from TDD Plan
| Test | Status | Location | Notes |

### Files from Plan
| File | Status | Notes |

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-verify` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `docs/learned/365-shell-completion.md`
- [ ] Summary included in commit
