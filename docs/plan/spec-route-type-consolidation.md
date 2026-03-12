# Spec: Route Type Consolidation

| Field | Value |
|-------|-------|
| Status | deferred |
| Depends | - |
| Phase | - |
| Updated | 2026-03-03 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `docs/architecture/route-types.md` — current route type analysis
3. `internal/plugin/types.go` — RouteSpec, PathAttributes, RIBRoute
4. `internal/plugin/rib/rib.go` — plugin rib.Route

## Task

Consolidate multiple Route struct representations into a unified architecture. LOW priority — current duplication works correctly. This is cleanup, not bugfix.

## Problem

5 Route structs represent the same logical concept differently:

| Struct | Location | Purpose | Issue |
|--------|----------|---------|-------|
| `RouteSpec` | `internal/plugin/types.go` | API input | Full attributes (parsed) |
| `PathAttributes` | `internal/plugin/types.go` | Embedded attrs | Shared by RouteSpec, L3VPNRoute, etc. |
| `rib.Route` | `internal/plugin/rib/rib.go` | Plugin storage | Full attributes as strings |
| `rr.Route` | `internal/plugin/rr/rib.go` | Zero-copy | Minimal (MsgID, Family, Prefix) |
| `RIBRoute` | `internal/plugin/types.go` | Query output | Minimal strings for JSON |
| `rib.Route` | `internal/rib/route.go` | Core engine | NLRI, Attrs, wire cache, refcount |

**Note:** Two different `rib.Route` types exist in different packages.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/route-types.md` - Current state analysis
  → Decision: TBD after reading
  → Constraint: TBD after reading
- [ ] `docs/architecture/encoding-context.md` - Zero-copy patterns
  → Decision: TBD after reading
- [ ] `docs/architecture/update-building.md` - Wire format building
  → Constraint: TBD after reading
- [ ] `docs/architecture/rib-transition.md` - RIB → API transition (affects scope)
  → Decision: TBD after reading — may limit scope to plugin-only

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/plugin/types.go` - RouteSpec, PathAttributes, RIBRoute definitions
- [ ] `internal/plugin/rib/rib.go` - plugin rib.Route definition
- [ ] `internal/plugin/rr/rib.go` - rr.Route zero-copy definition
- [ ] `internal/rib/route.go` - core engine rib.Route definition

**Behavior to preserve:**
- All current JSON output formats
- Zero-copy forwarding path (rr.Route minimal fields)
- Plugin storage patterns (rib.Route with full attributes)

**Behavior to change:**
- Reduce route struct duplication via unified type with view methods

## Data Flow (MANDATORY)

### Entry Point
- Routes enter from wire parsing (BGP UPDATE) and API commands

### Transformation Path
1. Wire bytes parsed into WireUpdate (lazy iterators)
2. Attributes extracted to different Route types depending on consumer
3. Plugin storage uses full attribute Route (rib.Route)
4. Zero-copy forwarding uses minimal Route (rr.Route — MsgID only)
5. API output uses RIBRoute (string-based for JSON)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine → Plugin | JSON event with route data | [ ] |
| Plugin storage → API | RIBRoute conversion | [ ] |
| Plugin storage → Forward | Zero-copy via MsgID reference | [ ] |

### Integration Points
- All route consumers would need updating if types are unified

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality
- [ ] Zero-copy preserved where applicable

## Proposed Solution

### Single Source of Truth

Create unified `rib.Route` with view methods:

| Field | Type | Description |
|-------|------|-------------|
| `MsgID` | `uint64` | Wire message reference for zero-copy forwarding |
| `Family` | `string` | Address family |
| `Prefix` | `string` | NLRI prefix |
| `PathID` | `uint32` | ADD-PATH path identifier |
| `NextHop` | `string` | Next-hop address |
| `Origin` | `string` | Origin attribute |
| `ASPath` | `[]uint32` | AS path |
| `MED` | `uint32` | Multi-exit discriminator |
| `LocalPref` | `uint32` | Local preference |
| `Communities` | `[]uint32` | Standard communities |
| `LargeCommunities` | `[]LargeCommunity` | Large communities |
| `ExtendedCommunities` | `[]ExtendedCommunity` | Extended communities |

**View methods:**

| Method | Returns | Purpose |
|--------|---------|---------|
| `ForZeroCopy()` | `ZeroCopyRoute` | Minimal fields for forwarding |
| `ForAPI()` | `RIBRoute` | String-based fields for JSON output |
| `MarshalJSON()` | JSON bytes | Direct JSON encoding |

### RouteBase Interface (optional)

| Method | Returns | Purpose |
|--------|---------|---------|
| `GetPrefix()` | `netip.Prefix` | Route destination |
| `GetNextHop()` | `RouteNextHop` | Next-hop address |
| `GetAttributes()` | `PathAttributes` | Route attributes |

## Scope Decision

**Question:** Given `docs/architecture/rib-transition.md` (RIB moves to API programs), should this consolidation:

1. **Plugin-only** - Consolidate `internal/plugin/` routes only (RouteSpec, rib.Route, rr.Route, RIBRoute)
2. **Full** - Also include `internal/rib/route.go` (core engine Route)

The RIB transition suggests plugin routes become the primary storage, making option 1 more relevant.

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRouteForZeroCopy` | `internal/plugin/rib/route_test.go` | Zero-copy view | |
| `TestRouteForAPI` | `internal/plugin/rib/route_test.go` | API view | |
| `TestRouteMarshalJSON` | `internal/plugin/rib/route_test.go` | JSON encoding | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|

## Files to Modify

- `internal/plugin/rib/rib.go` - Add view methods to Route
- `internal/plugin/rr/rib.go` - Use rib.Route.ForZeroCopy()
- `internal/plugin/types.go` - Possibly remove duplicate RIBRoute
- `internal/rib/route.go` - (if full scope) Align with plugin Route

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | |
| CLI commands/flags | No | |
| Plugin SDK docs | No | |
| Functional test | [ ] | |

## Implementation Steps

**Self-Critical Review:** After each step, review for issues and fix before proceeding.

1. **Analyze usage** - Find all Route struct usages
2. **Write unit tests** → Review: edge cases?
3. **Run tests** → Verify FAIL
4. **Add view methods** - To existing rib.Route
5. **Update rr plugin** - Use ForZeroCopy() view
6. **Update API output** - Use ForAPI() view
7. **Remove duplicates** - If safe
8. **Run tests** → Verify PASS
9. **Verify all** → `make ze-verify`

### Failure Routing

| Failure | Route To |
|---------|----------|

## Design Principles

**Modularity for plugins:** Route types should be designed so any plugin can easily:
- Store routes in whatever structure suits its needs
- Convert between representations without coupling to engine internals
- Use only the fields it needs (zero-copy uses MsgID only, full RIB uses all attrs)

**API stability guarantee:** Only the text and JSON APIs are stable. Go package structure, types, and interfaces may change without notice. Plugins should communicate via text/JSON protocol, not by importing Go packages.

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Design Insights

## Implementation Summary

### What Was Implemented
- (pending — spec is deferred)

### Documentation Updates
- (pending)

### Deviations from Plan
- (pending)

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

## Checklist

### Goal Gates (MUST pass)
- [ ] AC defined and demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-unit-test` passes
- [ ] `make ze-functional-test` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` — no failures)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] `make ze-lint` passes
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

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `docs/learned/NNN-route-type-consolidation.md`
- [ ] Summary included in commit
