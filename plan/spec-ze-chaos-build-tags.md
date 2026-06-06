# Spec: ze-chaos-build-tags

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/6 |
| Updated | 2026-06-06 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/learned/861-ze-test-build-tags.md` - ze-test migration lessons (same pattern)
4. `cmd/ze-chaos/main.go` - ze-chaos entry point and dispatch
5. `cmd/ze/ze_test_main.go` - reference for minimal main pattern

## Task

Build ze-chaos as a build-tag variant of `cmd/ze` instead of a separate `cmd/ze-chaos` binary, following the same pattern used for ze-test (learned summary 861). The `ze-chaos` binary is built via `go build -tags ze_chaos ./cmd/ze`. The `cmd/ze-chaos/` directory is deleted.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/chaos-web-dashboard.md` - chaos test architecture
  -> Constraint: ze-chaos is a monolithic command with flags, not subcommands. Single `run()` function dispatches via flag values (--replay, --shrink, --diff, --config-only, --pipe, or default orchestrator).
- [ ] `plan/learned/861-ze-test-build-tags.md` - ze-test migration pattern
  -> Decision: Use `//go:build !ze_chaos` on existing cmd/ze files (already have `!ze_test`; combine as `!ze_test && !ze_chaos`). Minimal `ze_chaos_main.go` provides main(). Centralized `zeTestRegisterAll` pattern for registration.

**Key insights:**
- ze-chaos is simpler than ze-test: one `run(args)` function, no subcommand dispatch table
- ze-chaos already accepts `args []string` via `run(os.Args[1:])`, so handler adaptation is trivial
- ze-chaos imports `plugin/all` (line 50 of main.go) for in-process mode, same as ze-test editor
- 10 source files, 3373 lines total, 48MB binary
- The `_test.go` naming trap from ze-test applies: don't name anything `*_ze_chaos_test.go` that isn't an actual test file
- The test runner builds ze-chaos via `SetExtraBinaries({"ze-chaos": "./cmd/ze-chaos"})` in ze_test_bgp.go; this path needs updating

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze-chaos/main.go` (936L) - Entry point. `main()` calls `run(os.Args[1:])`. Flag parsing via `flag.NewFlagSet`. Dispatches to: replay, shrink, diff, config-only, pipe, or orchestrator. Imports `plugin/all`, `internal/chaos/*`, `internal/component/mcp`, `internal/core/env`.
- [ ] `cmd/ze-chaos/orchestrator.go` (186L) - Orchestrator config types, established state, event processing.
- [ ] `cmd/ze-chaos/orchestrator_run.go` (604L) - Orchestrator run loop, reporting setup, convergence checking.
- [ ] `cmd/ze-chaos/scheduler.go` (294L) - Chaos and route dynamics schedulers.
- [ ] `cmd/ze-chaos/subcommand.go` (237L) - Replay, shrink, diff subcommands and network utilities.
- [ ] `cmd/ze-chaos/conflict.go` (169L) - Port conflict detection logic.
- [ ] `cmd/ze-chaos/fork.go` (181L) - Daemon forking logic (start/stop ze, FRR, BIRD).
- [ ] `cmd/ze-chaos/main_test.go` (321L) - Tests for run(), flag parsing, config generation.
- [ ] `cmd/ze-chaos/orchestrator_test.go` (273L) - Orchestrator unit tests.
- [ ] `cmd/ze-chaos/conflict_test.go` (172L) - Port conflict unit tests.
- [ ] `cmd/ze-test/bgp.go` -> now `cmd/ze/ze_test_bgp.go` - Lines 259-264 call `SetExtraBinaries({"ze-chaos": "./cmd/ze-chaos"})`.
- [ ] `internal/test/runner/runner.go` - `SetExtraBinaries` builds extra binaries from Go package paths (line 174-183).

**Behavior to preserve:**
- `ze-chaos [options]` CLI keeps working (same flags and behavior)
- `ze-chaos --replay`, `--shrink`, `--diff` subcommands unchanged
- CI pipeline uses `bin/ze-chaos` for chaos tests
- In-process mode (`--in-process`) needs `plugin/all`
- The test runner's `SetExtraBinaries` must build ze-chaos correctly

**Behavior to change:**
- ze-chaos binary built from `cmd/ze` with `-tags ze_chaos` instead of `go build ./cmd/ze-chaos`
- `cmd/ze-chaos/` directory deleted
- Existing cmd/ze build tags updated: `!ze_test` becomes `!ze_test && !ze_chaos`
- Makefile targets updated
- `SetExtraBinaries` in ze_test_bgp.go updated for new build path

## Data Flow (MANDATORY)

### Entry Point
- User runs `ze-chaos [options]`
- Binary is `cmd/ze` compiled with `-tags ze_chaos`

### Transformation Path
1. `main()` in `ze_chaos_main.go` stamps version, registers the chaos command, dispatches
2. `dispatchRegisteredRoot("chaos", rctx, args)` finds the handler
3. Handler calls `zeTestChaosRun(args)` which is the adapted `run()` function
4. `run()` parses flags and dispatches to replay/shrink/diff/config-only/pipe/orchestrator

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Build system -> binary | `ze_chaos` build tag controls compilation | [ ] |
| CLI dispatch -> chaos handler | `registry.MustRegisterRootHandler` + `dispatchRegisteredRoot()` | [ ] |

### Integration Points
- `command/registry.MustRegisterRootHandler` - registration
- `internal/test/runner` - `SetExtraBinaries` build path
- `ze_test_bgp.go` - chaos-web test suite builds ze-chaos

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality
- [ ] Zero-copy preserved where applicable

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze-chaos --help` | -> | chaos handler registered via `MustRegisterRootHandler` | `TestZeChaosHelp` |
| `go build -tags ze_chaos ./cmd/ze` | -> | binary contains chaos command | `TestBuildZeChaosVariant` |
| `go build ./cmd/ze` (no tag) | -> | binary does NOT contain chaos command | `TestBuildZeNoChaos` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `go build -tags ze_chaos ./cmd/ze` | Produces a binary with the chaos command |
| AC-2 | `bin/ze-chaos --help` | Shows chaos usage (same as today) |
| AC-3 | `bin/ze-chaos --seed 42 --peers 2 --config-only` | Generates config (same as today) |
| AC-4 | `go build ./cmd/ze` (no ze_chaos tag) | Binary does NOT contain chaos code |
| AC-5 | `cmd/ze-chaos/` directory | Deleted |
| AC-6 | Makefile `bin/ze-chaos` target | Builds via `go build -tags ze_chaos ./cmd/ze` |
| AC-7 | `bin/ze-test bgp chaos -a` | Chaos integration tests still build and run ze-chaos correctly |
| AC-8 | Existing `!ze_test` build tags | Updated to `!ze_test && !ze_chaos` |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestZeChaosHelp` | `cmd/ze/ze_chaos_main_test.go` | --help output matches | |
| `TestZeChaosConfigOnly` | `cmd/ze/ze_chaos_main_test.go` | --config-only generates config | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| chaos integration | existing `test/chaos/*.ci` | ze-test bgp chaos -a passes | |

## Files to Modify

- `cmd/ze/main.go` - Update build tag from `!ze_test` to `!ze_test && !ze_chaos`
- `cmd/ze/health_revert.go` - Same build tag update
- `cmd/ze/help_ai.go` - Same build tag update
- `cmd/ze/help_command.go` - Same build tag update
- `cmd/ze/pprof.go` - Update from `!tinygo && !ze_test` to `!tinygo && !ze_test && !ze_chaos`
- `cmd/ze/pprof_tinygo.go` - Update from `tinygo && !ze_test` to `tinygo && !ze_test && !ze_chaos`
- `cmd/ze/pushed_config.go` - Same build tag update
- `cmd/ze/update_serve.go` - Same build tag update
- 9 test files with `!ze_test` - Same build tag update
- `cmd/ze/ze_test_bgp.go` - Update `SetExtraBinaries` path from `"./cmd/ze-chaos"` to `"-tags ze_chaos ./cmd/ze"` (or equivalent)
- `internal/test/runner/runner.go` - Update `SetExtraBinaries` to support build tags for extra binaries
- `Makefile` - Update chaos/bin/ze-chaos targets and lint path
- `scripts/checks/command_ownership.go` - Update `hasZeTestBuildTag` to also skip `ze_chaos` tagged files

## Files to Create

- `cmd/ze/ze_chaos_main.go` (`//go:build ze_chaos`) - Minimal main() for ze-chaos builds
- `cmd/ze/ze_chaos_run.go` (`//go:build ze_chaos`) - Adapted `run()` and flag parsing from main.go
- `cmd/ze/ze_chaos_orchestrator.go` (`//go:build ze_chaos`) - From orchestrator.go
- `cmd/ze/ze_chaos_orchestrator_run.go` (`//go:build ze_chaos`) - From orchestrator_run.go
- `cmd/ze/ze_chaos_scheduler.go` (`//go:build ze_chaos`) - From scheduler.go
- `cmd/ze/ze_chaos_subcommand.go` (`//go:build ze_chaos`) - From subcommand.go
- `cmd/ze/ze_chaos_conflict.go` (`//go:build ze_chaos`) - From conflict.go
- `cmd/ze/ze_chaos_fork.go` (`//go:build ze_chaos`) - From fork.go
- `cmd/ze/ze_chaos_main_test.go` (`//go:build ze_chaos`) - From main_test.go
- `cmd/ze/ze_chaos_orchestrator_test.go` (`//go:build ze_chaos`) - From orchestrator_test.go
- `cmd/ze/ze_chaos_conflict_test.go` (`//go:build ze_chaos`) - From conflict_test.go

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| CLI commands/flags | Yes | cmd/ze/ze_chaos_*.go register via MustRegisterRootHandler |
| Doctor check | No | N/A |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` - update build instructions for ze-chaos |
| 12 | Internal architecture changed? | Yes | `docs/architecture/cli/plugin-modes.md` - add ze_chaos variant |
| 16 | Changed source files referenced by doc anchors? | Yes | Grep docs/ for cmd/ze-chaos references |

## Implementation Steps

### Implementation Phases

1. **Phase: Wiring** - Register chaos command, create minimal main, verify build
   - Files: `ze_chaos_main.go`, build tag updates on existing files
   - Verify: `go build -tags ze_chaos ./cmd/ze` compiles

2. **Phase: Migrate source files** - Copy and adapt all 7 non-test source files
   - Files: `ze_chaos_run.go`, `ze_chaos_orchestrator.go`, etc.
   - Verify: `ze-chaos --help` and `--config-only` work

3. **Phase: Migrate test files** - Copy and adapt 3 test files
   - Files: `ze_chaos_main_test.go`, `ze_chaos_orchestrator_test.go`, `ze_chaos_conflict_test.go`
   - Verify: `go test -tags ze_chaos ./cmd/ze/...` passes

4. **Phase: Update extra binary build** - Fix SetExtraBinaries for ze-chaos
   - Files: `ze_test_bgp.go`, `internal/test/runner/runner.go`
   - Verify: `bin/ze-test bgp chaos -l` lists chaos tests

5. **Phase: Makefile + cleanup** - Update targets, delete cmd/ze-chaos/
   - Files: `Makefile`, delete `cmd/ze-chaos/`
   - Verify: `make build` produces both `bin/ze` and `bin/ze-chaos`

6. **Full verification** - `make ze-verify`

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All 10 source files migrated, all ACs demonstrated |
| Build tags | `!ze_test` updated to `!ze_test && !ze_chaos` everywhere |
| SetExtraBinaries | ze-chaos build path updated in runner and ze_test_bgp.go |
| No duplicate main | Only one main() compiles per build configuration |
| Binary size | ze-chaos binary excludes daemon code |
| Ownership check | `hasZeTestBuildTag` updated to also skip ze_chaos files |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| ze-chaos builds from cmd/ze | `grep 'ze_chaos' Makefile` shows `-tags ze_chaos ./cmd/ze` |
| All source files guarded by ze_chaos | `head -3 cmd/ze/ze_chaos_*.go` shows build tag |
| cmd/ze-chaos/ deleted | `ls cmd/ze-chaos/` fails |
| Chaos integration tests pass | `bin/ze-test bgp chaos -l` lists tests |
| Regular ze unaffected | `go build ./cmd/ze` produces normal binary |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Build isolation | ze_chaos code must not compile into production ze binary |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Build tag `ze_chaos` | `ze_chaostest`, `ze_monkey` | Follows `ze_test`/`ze_distro` convention. Short, descriptive. |
| Single `chaos` root command | 4 subcommands (run, replay, shrink, diff) | ze-chaos is already a single command with flag dispatch. No subcommand table to unify. Register as one `chaos` root handler. |
| Combine build tags as `!ze_test && !ze_chaos` | Separate exclusion file per variant | Simpler. Each new variant adds one `&& !ze_<name>` clause. |

## Known Limitations
- ze-chaos binary size is 48MB (same as current) because it imports `plugin/all` for in-process mode.
- The `SetExtraBinaries` mechanism in the test runner may need enhancement to support build-tag-based builds (currently just uses `go build -o <name> <pkg>`).

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |

### Failed Approaches
| Approach | Why abandoned | Replacement |

## Implementation Summary

### What Was Implemented
- [pending]

## Implementation Audit

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Audit Summary
- **Total items:**
- **Done:**

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-verify` passes (our changes only)
- [ ] Feature code integrated
- [ ] Documentation Update Checklist answered

### Completion (BLOCKING)
- [ ] Implementation Summary filled
- [ ] Write learned summary to `plan/learned/862-ze-chaos-build-tags.md`
- [ ] **Commit A:** code + tests + spec + learned summary
- [ ] **Commit B:** `git rm plan/spec-ze-chaos-build-tags.md`
