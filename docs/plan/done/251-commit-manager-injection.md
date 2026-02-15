# Spec: 250-commit-manager-injection

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/plugin/server.go:170-270` - Server struct, NewServer, CommitManager getter
4. `internal/plugin/command.go:115-121` - CommandContext.CommitManager delegate
5. `internal/plugins/bgp/handler/commit.go` - all CommitManager callers
6. `internal/plugins/bgp/reactor/reactor.go:4270` - ServerConfig construction

## Task

Remove the `commit` import from `internal/plugin/server.go` and `internal/plugin/command.go` by changing `commitManager` from concrete type `*commit.CommitManager` to `any`. The CommitManager is injected via `ServerConfig` (like BGPHooks) rather than created inside `NewServer()`. BGP handler code type-asserts when needed.

Part of the three-spec effort to eliminate all BGP imports from `internal/plugin/`:
- 249: removes `bgptypes` from `command.go`
- **250** (this): removes `commit` from `server.go` and `command.go`
- 251: removes `bgptypes` from `types.go`

## Required Reading

### Architecture Docs
- [ ] `.claude/rules/plugin-design.md` - plugin architecture
  → Constraint: Infrastructure code MUST NOT directly import plugin implementation packages
  → Decision: BGPHooks uses `any` for BGP-specific types — same pattern applies here

### Source Files
- [ ] `internal/plugin/server.go:177,220,267` - commitManager field, creation, getter
  → Decision: Currently creates CommitManager inside NewServer — must move to caller
- [ ] `internal/plugin/command.go:116` - CommitManager() delegate returns concrete type
  → Constraint: Return type must change to `any`
- [ ] `internal/plugins/bgp/handler/commit.go` - 6 call sites via ctx.CommitManager()
  → Constraint: Must type-assert from `any` to `*commit.CommitManager`
- [ ] `internal/plugins/bgp/reactor/reactor.go:4270` - ServerConfig construction site
  → Decision: Add `CommitManager: commit.NewCommitManager()` here

**Key insights:**
- `commit.CommitManager` stores `nlri.NLRI` and `rib.Route` — deeply BGP-specific, cannot live in `internal/plugin/`
- BGPHooks established the pattern: generic infra stores opaque value, BGP code type-asserts
- Only 6 call sites in `handler/commit.go` need the concrete type
- reactor.go already imports commit-adjacent packages

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugin/server.go` - Server creates CommitManager in NewServer(), stores as `*commit.CommitManager`
- [ ] `internal/plugin/command.go` - CommandContext.CommitManager() returns `*commit.CommitManager`

**Behavior to preserve:**
- CommitManager is created once at server startup
- Accessible via `ctx.CommitManager()` from any handler
- Thread-safe transaction management (Start, End, Rollback, Get, List)

**Behavior to change:**
- CommitManager creation moves from NewServer() to reactor's ServerConfig construction
- Field type changes from `*commit.CommitManager` to `any`
- Getter return type changes from `*commit.CommitManager` to `any`
- Handlers type-assert to `*commit.CommitManager`

## Data Flow (MANDATORY)

### Entry Point
- CommitManager created in reactor.go, passed via `ServerConfig.CommitManager`

### Transformation Path
1. Reactor creates `commit.NewCommitManager()` and puts it in `ServerConfig.CommitManager`
2. `NewServer()` stores `config.CommitManager` as `any`
3. Handler calls `ctx.CommitManager()` → returns `any`
4. Handler type-asserts to `*commit.CommitManager`

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| reactor → plugin | Via ServerConfig.CommitManager (any) | [ ] |
| handler → commit | Type assertion from any | [ ] |

### Integration Points
- `ServerConfig` already has `BGPHooks *BGPHooks` — same injection pattern
- `reactor.go:4270` already constructs ServerConfig with BGPHooks

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling (reduces coupling — removes commit import from generic infra)
- [ ] No duplicated functionality
- [ ] Zero-copy preserved where applicable

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `grep 'bgp/commit' internal/plugin/server.go` | Zero matches |
| AC-2 | `grep 'bgp/commit' internal/plugin/command.go` | Zero matches |
| AC-3 | `handler/commit.go` type-asserts successfully | Commit operations work |
| AC-4 | `make verify` | All tests + lint + functional pass |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| Existing commit handler tests | `handler/commit_test.go` | Commit operations work via type assertion | |
| TestCommandContextAccessors | `plugin/command_test.go` | CommitManager() returns injected value | |
| Existing server tests | `plugin/server_test.go` | Server works with injected CommitManager | |

### Boundary Tests
Not applicable — no numeric inputs.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| All existing functional tests | `test/` | Commit operations unchanged | |

## Files to Modify
- `internal/plugin/types.go` - add `CommitManager any` to ServerConfig
- `internal/plugin/server.go` - field type `any`, use `config.CommitManager`, getter returns `any`, remove `commit` import
- `internal/plugin/command.go` - `CommitManager()` delegate returns `any`, remove `commit` import
- `internal/plugins/bgp/handler/commit.go` - add `requireCommitManager()` helper, update 6 call sites
- `internal/plugins/bgp/reactor/reactor.go:4270` - add `CommitManager: commit.NewCommitManager()` to ServerConfig
- `internal/plugin/command_test.go` - use `any` value, remove `commit` import

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | |
| RPC count in architecture docs | No | |
| CLI commands/flags | No | |
| Functional test for new RPC/API | No | |

## Files to Create
None.

## Implementation Steps

1. **Add `CommitManager any` to ServerConfig** in types.go
   → **Review:** Field name clear? Position logical?

2. **Change server.go** — field type, constructor, getter, remove import
   → **Review:** Nil handling preserved? `config.CommitManager` may be nil?

3. **Change command.go** — delegate return type, remove import
   → **Review:** After Spec 249, only `commit` import remains; this removes it

4. **Add `requireCommitManager()` in handler/commit.go** — type assertion helper
   → **Review:** Error message clear? Nil-safe?

5. **Update reactor.go:4270** — inject CommitManager in ServerConfig
   → **Review:** `commit.NewCommitManager()` created at right point?

6. **Update command_test.go** — use `any` value
   → **Review:** Test still validates accessor round-trip?

7. **Run tests** - `make verify`
   → **Review:** Zero failures? Zero lint issues?

## Implementation Summary

### What Was Implemented
- Added `CommitManager any` field to `ServerConfig` in `types.go`
- Changed `server.go`: field type `*commit.CommitManager` → `any`, constructor uses `config.CommitManager`, getter returns `any`, `commit` import removed
- Changed `command.go`: `CommitManager()` returns `any`, `commit` import removed
- Added `requireCommitManager()` helper in `handler/commit.go` with nil-check and type assertion
- Updated all 6 call sites in `handler/commit.go` to use `requireCommitManager()`
- Updated `reactor.go` to inject `commit.NewCommitManager()` in ServerConfig
- Updated `mock_reactor_test.go` `newTestContext()` to inject CommitManager
- Updated `dispatch_test.go` `newDispatchContext()` to inject CommitManager

### Bugs Found/Fixed
- Test helpers `newTestContext()` and `newDispatchContext()` created ServerConfig without CommitManager, causing "commit manager not available" failures after injection change. Fixed by adding `CommitManager: commit.NewCommitManager()`.

### Design Insights
- The pattern of "generic infra stores `any`, domain code type-asserts" is now used for both BGPHooks and CommitManager, establishing it as the standard DI pattern for this codebase.

### Documentation Updates
- None — no architectural changes, just dependency direction fix.

### Deviations from Plan
- `command_test.go` required no changes (test imports `commit` directly, and `assert.Equal` works with `any` return type).

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Remove `commit` import from `server.go` | ✅ Done | `server.go` — verified zero matches | |
| Remove `commit` import from `command.go` | ✅ Done | `command.go` — verified zero matches | |
| CommitManager injected via ServerConfig | ✅ Done | `types.go:ServerConfig.CommitManager` | |
| Handlers type-assert when needed | ✅ Done | `handler/commit.go:requireCommitManager()` | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | ✅ Done | `grep 'bgp/commit' server.go` = 0 matches | |
| AC-2 | ✅ Done | `grep 'bgp/commit' command.go` = 0 matches | |
| AC-3 | ✅ Done | All commit handler tests pass | |
| AC-4 | ✅ Done | `make verify` — 0 lint, all tests, 243 functional | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Existing commit handler tests | ✅ Pass | `handler/commit_test.go` | |
| TestCommandContextAccessors | ✅ Pass | `plugin/command_test.go` | No changes needed |
| Existing server tests | ✅ Pass | `plugin/server_test.go` | |
| Dispatch tests | ✅ Pass | `handler/dispatch_test.go` | Fixed CommitManager injection |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/plugin/types.go` | ✅ Modified | Added `CommitManager any` |
| `internal/plugin/server.go` | ✅ Modified | Field→any, no commit import |
| `internal/plugin/command.go` | ✅ Modified | Returns any, no commit import |
| `internal/plugins/bgp/handler/commit.go` | ✅ Modified | requireCommitManager + 6 call sites |
| `internal/plugins/bgp/reactor/reactor.go` | ✅ Modified | Injects CommitManager |
| `internal/plugin/command_test.go` | 🔄 Changed | No changes needed — works with any |

### Audit Summary
- **Total items:** 14
- **Done:** 13
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (command_test.go — no changes needed, deviation documented)

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| Test helpers would work without CommitManager | Test helpers create ServerConfig without it | 12 test failures | Fixed both helpers |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|
| Test helpers not updated for DI changes | First occurrence | — | Monitor |

## Checklist

### Goal Gates (MUST pass — cannot defer)
- [x] Acceptance criteria AC-1..AC-4 all demonstrated
- [x] Tests pass (`make test`)
- [x] No regressions (`make functional`)
- [x] Feature code integrated into codebase

### Quality Gates (SHOULD pass)
- [x] `make lint` passes
- [x] Implementation Audit fully completed

### 🧪 TDD
- [x] Tests written
- [x] Tests FAIL (existing tests fail after type change, before handler fix)
- [x] Implementation complete
- [x] Tests PASS — `make verify` all green
