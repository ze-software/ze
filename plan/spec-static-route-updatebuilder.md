# Spec: Convert Static Route Functions to UpdateBuilder

## MANDATORY READING (BEFORE IMPLEMENTATION)

```
┌─────────────────────────────────────────────────────────────────┐
│  STOP. Read these files FIRST before ANY implementation:        │
│                                                                 │
│  1. .claude/ESSENTIAL_PROTOCOLS.md - Session rules, TDD         │
│  2. .claude/INDEX.md - Find what docs to load                   │
│  3. plan/CLAUDE_CONTINUATION.md - Current state                 │
│  4. THIS SPEC FILE - Design requirements                        │
│  5. pkg/reactor/peer.go, pkg/bgp/message/update_build.go        │
│                                                                 │
│  DO NOT PROCEED until all are read and understood.              │
└─────────────────────────────────────────────────────────────────┘
```

## Task

Convert legacy `buildStaticRouteUpdate`, `buildGroupedUpdate`, and `buildRIBRouteUpdate` functions in `pkg/reactor/peer.go` to use the UpdateBuilder pattern from `pkg/bgp/message/update_build.go`.

## Current State (verified)

- Functional tests: 24 passed, 13 failed [0, 7, 8, J, L, N, Q, S, T, U, V, Z, a]
- `make test && make lint`: Pass (unit tests)
- Last commit: `5a5c5a8`

## Embedded Protocol Requirements

### Default Rules (ALL tasks)

- **FIRST:** Run `git status` - if modified files exist, ASK user before proceeding
- Tests MUST exist and FAIL before implementation code exists
- Run `make test && make lint` before claiming done
- NEVER discard uncommitted work without explicit user permission
- Verify before claiming: run commands, paste output as proof
- Tests passing is NOT permission to commit - wait for user

### From ESSENTIAL_PROTOCOLS.md

1. **RFC 4271 Appendix F.3:** Attributes MUST be ordered by type code
2. **Work Preservation:** Save work before any destructive action
3. **Verification:** Never claim "done" without running tests and pasting output
4. **One function at a time:** Refactoring protocol requires step-by-step execution
5. **Self-review:** After completion, critically review changes

### From TDD_ENFORCEMENT.md

1. For refactoring, existing tests serve as regression suite
2. Wire compat tests verify OLD == NEW behavior
3. Run tests after each function conversion
4. Paste full test output as proof

## Codebase Context

### Files to Modify

- `pkg/reactor/peer.go` - Contains legacy functions
- `pkg/bgp/message/update_build.go` - Contains UpdateBuilder (reference)

### Legacy Functions

1. **`buildRIBRouteUpdate`** (line 1080, ~100 LOC)
   - Builds UPDATE for RIB routes
   - Used by `sendInitialRoutes` for API-announced routes

2. **`buildStaticRouteUpdate`** (line 1306, ~260 LOC)
   - Builds UPDATE for configured static routes
   - Handles unicast (IPv4/IPv6) and VPN routes
   - Complex attribute construction with RFC 8950 extended next-hop

3. **`buildGroupedUpdate`** (line 1573, ~unknown LOC)
   - Builds UPDATE for multiple routes with same attributes
   - Performance optimization for bulk announcements

### Pattern to Follow

Previous session converted send* functions using this approach:
1. Create conversion helpers (toUnicastParams, toVPNParams, etc.)
2. Call UpdateBuilder.BuildXxx() with converted params
3. Wire compat tests verify OLD == NEW

## Implementation Steps

### Phase 1: Analysis

1. Read all 3 legacy functions completely
2. Map each to existing UpdateBuilder.BuildXxx() methods
3. Identify gaps (methods UpdateBuilder doesn't have)

### Phase 2: RIB Route Conversion

1. Add wire compat test for `buildRIBRouteUpdate`
2. Create `toRIBRouteParams()` conversion helper
3. Replace implementation with UpdateBuilder call
4. Verify wire compat test passes

### Phase 3: Static Route Conversion

1. Add wire compat test for `buildStaticRouteUpdate`
2. Create `toStaticRouteParams()` conversion helper
3. Replace implementation with UpdateBuilder call
4. Verify wire compat test passes

### Phase 4: Grouped Update Conversion

1. Add wire compat test for `buildGroupedUpdate`
2. Determine if UpdateBuilder needs new method or can reuse existing
3. Convert or add new BuildGrouped method
4. Verify wire compat test passes

### Phase 5: Cleanup

1. Remove old function bodies (keep signatures if needed)
2. Run full `make test && make lint`
3. Run functional tests
4. Self-review

## Verification Checklist

- [ ] Wire compat test exists for each converted function
- [ ] Wire compat tests pass (OLD == NEW)
- [ ] `make test` passes
- [ ] `make lint` passes
- [ ] No RFC compliance regressions
- [ ] Self-review completed
- [ ] LOC reduction documented

## Notes

- The UpdateBuilder already handles RFC 4271 attribute ordering
- VPN routes use rawAttribute for MP_REACH_NLRI (VPN next-hop format differs)
- Extended next-hop (RFC 8950) support needs verification
- Grouped updates may need new UpdateBuilder method
