# Spec: Route Type Consolidation

## Task

Consolidate multiple Route struct representations into a unified architecture.

## Problem

4 Route structs represent the same logical concept differently:

| Struct | Location | Purpose | Issue |
|--------|----------|---------|-------|
| `RouteSpec` | `pkg/plugin/types.go` | API input | Full attributes |
| `rib.Route` | `pkg/plugin/rib/rib.go` | Storage | Full attributes (duplicate) |
| `rr.Route` | `pkg/plugin/rr/rib.go` | Zero-copy | Minimal fields |
| `RIBRoute` | `pkg/plugin/types.go` | Query output | Minimal fields |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/ROUTE_TYPES.md` - Current state analysis
- [ ] `docs/architecture/ENCODING_CONTEXT.md` - Zero-copy patterns
- [ ] `docs/architecture/UPDATE_BUILDING.md` - Wire format building

## Proposed Solution

### Single Source of Truth

Create unified `rib.Route` with view methods:

```go
// pkg/rib/route.go (or pkg/plugin/rib/route.go)
type Route struct {
    MsgID               uint64
    Family              string
    Prefix              string
    PathID              uint32
    NextHop             string
    Origin              string
    ASPath              []uint32
    MED                 uint32
    LocalPref           uint32
    Communities         []uint32
    LargeCommunities    []LargeCommunity
    ExtendedCommunities []ExtendedCommunity
}

// Views for different use cases
func (r *Route) ForZeroCopy() ZeroCopyRoute { ... }
func (r *Route) ForAPI() RIBRoute { ... }
func (r *Route) MarshalJSON() ([]byte, error) { ... }
```

### RouteBase Interface

Optional: Formalize route type hierarchy:

```go
type RouteBase interface {
    GetPrefix() netip.Prefix
    GetNextHop() RouteNextHop
    GetAttributes() PathAttributes
}
```

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates |
|------|------|-----------|
| `TestRouteForZeroCopy` | `pkg/plugin/rib/route_test.go` | Zero-copy view |
| `TestRouteForAPI` | `pkg/plugin/rib/route_test.go` | API view |
| `TestRouteMarshalJSON` | `pkg/plugin/rib/route_test.go` | JSON encoding |

## Files to Modify

- `pkg/plugin/rib/rib.go` - Add view methods to Route
- `pkg/plugin/rr/rib.go` - Use rib.Route.ForZeroCopy()
- `pkg/plugin/types.go` - Possibly remove duplicate RIBRoute

## Implementation Steps

1. **Analyze usage** - Find all Route struct usages
2. **Add view methods** - To existing rib.Route
3. **Update rr plugin** - Use ForZeroCopy() view
4. **Update API output** - Use ForAPI() view
5. **Remove duplicates** - If safe
6. **Run tests** - `make test && make lint && make functional`

## Priority

**LOW** - Current duplication works correctly. This is cleanup, not bugfix.

## Checklist

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)

### Verification
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make functional` passes

### Completion
- [ ] Spec moved to `docs/plan/done/NNN-route-type-consolidation.md`
