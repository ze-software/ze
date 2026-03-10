# Spec: cli-log

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/api/commands.md` - command handler patterns
4. `internal/core/slogutil/slogutil.go` - current logging infrastructure
5. `internal/component/bgp/plugins/cmd/cache/cache.go` - reference command plugin

## Task

Add `ze bgp log` CLI commands to view and change log levels at runtime. Two subcommands: `bgp log show` (list all subsystems and their current levels) and `bgp log set <subsystem> <level>` (change a subsystem's log level at runtime).

This requires infrastructure changes to `slogutil`: currently, loggers are created with fixed `slog.Level` values and `sync.Once` lazy initialization. Runtime level changes require switching to `slog.LevelVar` and maintaining a registry of created loggers so they can be enumerated and modified after creation.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/commands.md` - command handler patterns, RPC registration
  → Decision: all commands follow RPCRegistration pattern with YANG schema
  → Constraint: handler signature is `func(ctx *CommandContext, args []string) (*plugin.Response, error)`
- [ ] `docs/architecture/config/environment.md` - logging configuration
  → Constraint: hierarchical env var lookup: ze.log.<path> from most-specific to least-specific

### RFC Summaries (MUST for protocol work)
N/A - no protocol work.

**Key insights:**
- `slogutil.Logger()` creates loggers with fixed `slog.Level` via `createHandler()`. The level is determined once at creation time from env vars and cannot be changed later.
- `slogutil.LazyLogger()` uses `sync.Once` to defer creation until first use, but once created, the level is fixed.
- `slog.HandlerOptions` accepts a `Leveler` interface. `slog.LevelVar` implements `Leveler` and supports atomic `Set()`. Switching from `slog.Level` to `*slog.LevelVar` enables runtime changes with no API breakage.
- There is no global list of created loggers. To implement `show`, a `LevelRegistry` must track all loggers created via `Logger()` and `LazyLogger()`.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/core/slogutil/slogutil.go` - Logger(), LazyLogger(), PluginLogger(), parseLevel(), createHandler()
  → Constraint: `Logger()` returns `*slog.Logger` with fixed level from env vars
  → Constraint: `LazyLogger()` returns `func() *slog.Logger` with `sync.Once`
  → Constraint: `parseLevel()` maps string to `slog.Level` + enabled bool
  → Constraint: levels are: disabled, debug, info, warn/warning, err/error
- [ ] `internal/core/slogutil/parse.go` - level parsing helper (if separate)
- [ ] `internal/component/bgp/plugins/cmd/cache/cache.go` - reference command plugin
  → Constraint: handlers use pluginserver.RegisterRPCs in init(), schema in schema/ subdir
- [ ] `internal/component/bgp/plugins/cmd/peer/peer.go` - handler registration with ReadOnly and RequiresSelector
  → Constraint: ReadOnly: true makes command accessible via `ze show`
- [ ] `internal/component/plugin/server/command.go` - CommandContext, Dispatcher, Handler type
  → Constraint: handler accesses reactor via ctx.Reactor(), server via ctx.Server

**Behavior to preserve:**
- Hierarchical env var lookup for initial log levels (ze.log.<path>)
- Priority: CLI flag > env var > config > default (WARN)
- All existing Logger() and LazyLogger() callers continue to work unchanged
- parseLevel() string-to-level mapping
- Plugin loggers write to stderr (stdout = protocol)

**Behavior to change:**
- `Logger()` and `LazyLogger()`: switch from fixed `slog.Level` to `*slog.LevelVar` in handler options, so levels can be changed at runtime
- Add a `LevelRegistry` that tracks subsystem name to `*slog.LevelVar` mapping
- Add `ListLevels() map[string]slog.Level` and `SetLevel(subsystem string, level slog.Level) error` to LevelRegistry
- Register the LevelRegistry so command handlers can access it (package-level variable in slogutil, similar to how metrics uses registry.GetMetricsRegistry)

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- User runs `ze show bgp log show` or `ze run bgp log set <subsystem> <level>`
- CLI dispatches to unix socket, which reaches Server.Dispatch

### Transformation Path -- `bgp log show`
1. CLI text command arrives at Dispatcher via socket or text session
2. Dispatcher matches `bgp log show` to registered handler
3. Handler calls `slogutil.ListLevels()` to get subsystem -> level map
4. Handler formats result as JSON map (subsystem names as keys, level strings as values)
5. Response returns to CLI caller

### Transformation Path -- `bgp log set`
1. CLI text command arrives: `bgp log set <subsystem> <level>`
2. Dispatcher matches `bgp log set` to registered handler
3. Handler parses args: args[0] = subsystem, args[1] = level
4. Handler validates level string via `slogutil.ParseLevel()` (exported version)
5. Handler calls `slogutil.SetLevel(subsystem, level)` to update the LevelVar
6. If subsystem not found: error response "unknown subsystem: <name>"
7. If level invalid: error response "invalid level: <name>"
8. Success: response with subsystem + new level

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI ↔ Server | Unix socket IPC, JSON-RPC or text command | [ ] |
| Handler ↔ slogutil | Direct function calls to ListLevels() and SetLevel() | [ ] |
| slogutil ↔ slog.LevelVar | Atomic level change via LevelVar.Set() | [ ] |

### Integration Points
- `internal/core/slogutil/slogutil.go` - LevelRegistry, ListLevels(), SetLevel() additions
- `internal/component/plugin/server/command.go` - CommandContext, Handler type
- `internal/component/plugin/server/rpc_register.go` - RegisterRPCs() for init-time registration

### Architectural Verification
- [ ] No bypassed layers (handler calls slogutil directly, which owns the loggers)
- [ ] No unintended coupling (slogutil is already imported everywhere, no new coupling)
- [ ] No duplicated functionality (no existing runtime log level control exists)
- [ ] Zero-copy preserved where applicable (N/A - not wire path)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | | Feature Code | Test |
|-------------|---|--------------|------|
| `ze show bgp log show` via CLI | -> | `handleLogShow` handler | `test/plugin/cli-log-show.ci` |
| `ze run bgp log set <subsystem> <level>` via CLI | -> | `handleLogSet` handler | `test/plugin/cli-log-set.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `bgp log show` after server startup | Returns JSON map of subsystem names to current level strings |
| AC-2 | `bgp log set server debug` | Changes server subsystem to debug level, returns success with subsystem + level |
| AC-3 | `bgp log set nonexistent info` | Returns error: "unknown subsystem: nonexistent" |
| AC-4 | `bgp log set server badlevel` | Returns error: "invalid level: badlevel" |
| AC-5 | `bgp log set` with missing args | Returns usage error message |
| AC-6 | `bgp log show` is ReadOnly | Command accessible via `ze show` path |
| AC-7 | `bgp log set` is NOT ReadOnly | Command requires `ze run` path |
| AC-8 | Existing Logger() callers unchanged | All existing loggers work without modification |
| AC-9 | LevelVar change takes effect | After `set`, new log calls at the new level are emitted (or suppressed) |
| AC-10 | YANG module `ze-bgp-cmd-log-api` registered | CLI autocomplete and command tree include log commands |

## TDD Test Plan

### Unit Tests -- slogutil
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestLevelRegistryTracksLoggers` | `internal/core/slogutil/slogutil_test.go` | Logger() registers subsystem in LevelRegistry | |
| `TestLevelRegistryListLevels` | `internal/core/slogutil/slogutil_test.go` | ListLevels() returns all tracked subsystems with current levels | |
| `TestLevelRegistrySetLevel` | `internal/core/slogutil/slogutil_test.go` | SetLevel() changes level and logger output reflects new level | |
| `TestLevelRegistrySetLevelUnknown` | `internal/core/slogutil/slogutil_test.go` | SetLevel() for unknown subsystem returns error | |
| `TestLazyLoggerRegistered` | `internal/core/slogutil/slogutil_test.go` | LazyLogger() registers on first call, not at creation | |

### Unit Tests -- command handlers
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestLogShowHandler` | `internal/component/bgp/plugins/cmd/log/log_test.go` | Handler returns subsystem list from registry | |
| `TestLogSetHandler` | `internal/component/bgp/plugins/cmd/log/log_test.go` | Handler changes level via SetLevel() | |
| `TestLogSetMissingArgs` | `internal/component/bgp/plugins/cmd/log/log_test.go` | Handler returns usage error with no args | |
| `TestLogSetInvalidLevel` | `internal/component/bgp/plugins/cmd/log/log_test.go` | Handler returns error for bad level string | |
| `TestLogDispatch` | `internal/component/bgp/plugins/cmd/log/dispatch_test.go` | Verifies RPCs are registered and dispatchable | |

### Boundary Tests (MANDATORY for numeric inputs)
No numeric inputs for these commands.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `cli-log-show` | `test/plugin/cli-log-show.ci` | User runs `ze show bgp log show`, sees subsystem levels | |
| `cli-log-set` | `test/plugin/cli-log-set.ci` | User runs `ze run bgp log set server debug`, level changes | |

### Future (if deferring any tests)
- None deferred

## Files to Modify
- `internal/core/slogutil/slogutil.go` - add LevelRegistry, switch Logger()/LazyLogger() to use slog.LevelVar, export ParseLevel, add ListLevels()/SetLevel()
- `docs/architecture/api/commands.md` - add log command documentation

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | Yes | `internal/component/bgp/plugins/cmd/log/schema/ze-bgp-cmd-log-api.yang` |
| RPC count in architecture docs | Yes | `docs/architecture/api/architecture.md` |
| CLI commands/flags | No | N/A - auto-registered via RegisterRPCs |
| CLI usage/help text | Yes | Help strings in RPCRegistration |
| API commands doc | Yes | `docs/architecture/api/commands.md` |
| Plugin SDK docs | No | N/A |
| Editor autocomplete | Yes | YANG-driven (automatic if YANG updated) |
| Functional test for new RPC/API | Yes | `test/plugin/cli-log-show.ci`, `test/plugin/cli-log-set.ci` |

## Files to Create
- `internal/component/bgp/plugins/cmd/log/doc.go` - package doc with `// Design:` annotation + blank import of schema
- `internal/component/bgp/plugins/cmd/log/schema/embed.go` - `//go:embed` of YANG file
- `internal/component/bgp/plugins/cmd/log/schema/register.go` - `init()` calling `yang.RegisterModule()`
- `internal/component/bgp/plugins/cmd/log/schema/ze-bgp-cmd-log-api.yang` - YANG RPC definitions
- `internal/component/bgp/plugins/cmd/log/log.go` - handlers + `init()` calling `pluginserver.RegisterRPCs()`
- `internal/component/bgp/plugins/cmd/log/log_test.go` - unit tests for handlers
- `internal/component/bgp/plugins/cmd/log/dispatch_test.go` - dispatch registration tests
- `test/plugin/cli-log-show.ci` - functional test for `bgp log show`
- `test/plugin/cli-log-set.ci` - functional test for `bgp log set`

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase 1: slogutil LevelRegistry infrastructure**
   a. Write unit tests for LevelRegistry (track, list, set) -> Verify FAIL
   b. Implement LevelRegistry: `sync.Map` of subsystem name to `*slog.LevelVar`
   c. Modify `Logger()`: create `*slog.LevelVar`, register in LevelRegistry, pass as `Leveler` to HandlerOptions
   d. Modify `LazyLogger()`: register on first call (inside the Once closure)
   e. Export `ParseLevel()` (rename existing `parseLevel` or add public wrapper)
   f. Add `ListLevels() map[string]string` and `SetLevel(subsystem, levelStr string) error`
   g. Run tests -> Verify PASS
   h. Run existing slogutil tests -> Verify still PASS (backward compat)

2. **Phase 2: command handler plugin**
   a. Write unit tests for handlers -> Verify FAIL
   b. Create file structure: doc.go, schema/, log.go
   c. Implement handlers using slogutil.ListLevels() and slogutil.SetLevel()
   d. Run tests -> Verify PASS

3. **Phase 3: functional tests and wiring**
   a. Create `.ci` tests for both commands
   b. Verify all -> `make ze-test`

4. **Critical Review** -> All 6 checks from `rules/quality.md` must pass
5. **Complete spec** -> Fill audit tables, write learned summary

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Step 1c/2b (fix syntax/types) |
| Test fails wrong reason | Step 1a/2a (fix test) |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Existing slogutil tests break | Step 1 - backward compat issue, fix LevelRegistry integration |
| Audit finds missing AC | Back to IMPLEMENT for that criterion |

## Design Decisions

### LevelRegistry Location
The LevelRegistry lives in `slogutil` alongside `Logger()` and `LazyLogger()`, because it tracks the loggers that slogutil creates. It is not stored in `plugin/registry` (which is for plugin metadata). The command handler calls `slogutil.ListLevels()` and `slogutil.SetLevel()` directly.

### slog.LevelVar vs. Custom Leveler
`slog.LevelVar` is the stdlib solution for mutable log levels. It uses `atomic.Int64` internally, making it safe for concurrent reads and writes. No custom type needed.

### Disabled Loggers
Currently, disabled loggers use `discardHandler{}`. After the change, disabled loggers still use `discardHandler{}` and are NOT registered in the LevelRegistry (since there is no LevelVar to change). If a user wants to enable a previously-disabled subsystem, they must restart with the appropriate env var. This matches the principle that `disabled` is opt-out, not dynamically toggleable.

Alternative considered: register disabled loggers with a LevelVar set to a very high level (above ERROR). Rejected because discardHandler's `Enabled()` returns false regardless, so changing the LevelVar would have no effect. This would confuse users who `set` a disabled logger to `debug` and see no output.

### LazyLogger Registration Timing
LazyLoggers register in the LevelRegistry when first called (inside the `sync.Once`), not at creation time. This means `bgp log show` only shows subsystems whose loggers have actually been initialized. This is correct: showing uninitialized subsystems would be misleading since their level is not yet determined.

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

N/A - not protocol code.

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
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
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
