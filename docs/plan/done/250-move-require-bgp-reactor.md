# Spec: 249-move-require-bgp-reactor

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/plugin/command.go` - source of RequireBGPReactor
4. `internal/plugins/bgp/handler/` - all callers

## Task

Move `RequireBGPReactor()` from `internal/plugin/command.go` to `internal/plugins/bgp/handler/`. All 14 callers are in `bgp/handler/`. The function becomes package-private `requireBGPReactor()`. This removes the `bgptypes` import from `command.go`.

Part of the three-spec effort to eliminate all BGP imports from `internal/plugin/`:
- **249** (this): removes `bgptypes` from `command.go`
- 250: removes `commit` from `server.go` and `command.go`
- 251: removes `bgptypes` from `types.go`

## Required Reading

### Architecture Docs
- [ ] `.claude/rules/plugin-design.md` - plugin architecture
  → Constraint: Infrastructure code MUST NOT directly import plugin implementation packages

### Source Files
- [ ] `internal/plugin/command.go:143-162` - RequireBGPReactor definition
  → Decision: Function type-asserts ReactorLifecycle to bgptypes.BGPReactor
- [ ] `internal/plugins/bgp/handler/cache.go` - 6 call sites
  → Constraint: All callers use same pattern: `r, errResp, err := plugin.RequireBGPReactor(ctx)`

**Key insights:**
- 14 callers, all in `bgp/handler/` — zero callers outside
- Function is BGP-specific (type-asserts to BGPReactor)
- After move, `command.go` no longer needs `bgptypes` import

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugin/command.go` - exports `RequireBGPReactor()` used by handler/ files

**Behavior to preserve:**
- Function returns `(bgptypes.BGPReactor, *Response, error)` with same nil/error semantics
- Two error cases: reactor nil, reactor doesn't implement BGPReactor

**Behavior to change:**
- Function moves from `plugin` package (exported) to `handler` package (unexported)
- Callers change from `plugin.RequireBGPReactor(ctx)` to `requireBGPReactor(ctx)`

## Data Flow (MANDATORY)

### Entry Point
- Handler functions receive `*plugin.CommandContext` from dispatcher

### Transformation Path
1. Handler calls `requireBGPReactor(ctx)` (was `plugin.RequireBGPReactor(ctx)`)
2. Function calls `ctx.Reactor()` → `ReactorLifecycle` interface
3. Type-asserts to `bgptypes.BGPReactor`
4. Returns typed reactor or error response

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| handler → plugin | `ctx.Reactor()` returns `plugin.ReactorLifecycle` | [ ] |
| handler → bgptypes | Type assertion to `bgptypes.BGPReactor` | [ ] |

### Integration Points
- `plugin.CommandContext.Reactor()` — already used, no change
- `bgptypes.BGPReactor` — already imported by handler/, no new import

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling (reduces coupling — removes BGP import from generic infra)
- [ ] No duplicated functionality
- [ ] Zero-copy preserved where applicable

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `grep bgptypes internal/plugin/command.go` | Zero matches |
| AC-2 | All handler callers use `requireBGPReactor(ctx)` | Compiles, tests pass |
| AC-3 | `make verify` | All tests + lint + functional pass |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| Existing handler tests | `handler/*_test.go` | Handlers still work with local function | |
| Existing command tests | `plugin/command_test.go` | CommandContext still works without RequireBGPReactor | |

### Boundary Tests
Not applicable — no numeric inputs.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| All existing functional tests | `test/` | No behavior change | |

## Files to Modify
- `internal/plugin/command.go` - delete `RequireBGPReactor()` (lines 143-162), remove `bgptypes` import
- `internal/plugins/bgp/handler/commit.go` - `plugin.RequireBGPReactor` → `requireBGPReactor`
- `internal/plugins/bgp/handler/update_text.go` - same (2 sites)
- `internal/plugins/bgp/handler/raw.go` - same (2 sites)
- `internal/plugins/bgp/handler/cache.go` - same (6 sites)
- `internal/plugins/bgp/handler/refresh.go` - same (2 sites)
- `internal/plugins/bgp/handler/route_watchdog.go` - same (1 site)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | |
| RPC count in architecture docs | No | |
| CLI commands/flags | No | |
| Functional test for new RPC/API | No | |

## Files to Create
- `internal/plugins/bgp/handler/require.go` - moved function as `requireBGPReactor()`

## Implementation Steps

1. **Create `handler/require.go`** with the moved function (package-private)
   → **Review:** Same logic as original? Same error messages?

2. **Update all 14 call sites** in handler/ files
   → **Review:** `plugin.RequireBGPReactor(ctx)` → `requireBGPReactor(ctx)` everywhere?

3. **Delete original** from `command.go`, remove `bgptypes` import
   → **Review:** `command.go` still has `commit` import (addressed by Spec 250)?

4. **Run tests** - `make verify`
   → **Review:** Zero failures? Zero lint issues?

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

## Checklist

### Goal Gates (MUST pass — cannot defer)
- [ ] Acceptance criteria AC-1..AC-3 all demonstrated
- [ ] Tests pass (`make test`)
- [ ] No regressions (`make functional`)
- [ ] Feature code integrated into codebase

### Quality Gates (SHOULD pass)
- [ ] `make lint` passes
- [ ] Implementation Audit fully completed

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)
