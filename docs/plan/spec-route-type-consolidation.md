# Spec: Route Type Consolidation

## Task

Consolidate multiple Route struct representations into a unified architecture.

## Problem

5 Route structs represent the same logical concept differently:

| Struct | Location | Purpose | Issue |
|--------|----------|---------|-------|
| `RouteSpec` | `pkg/plugin/types.go` | API input | Full attributes (parsed) |
| `PathAttributes` | `pkg/plugin/types.go` | Embedded attrs | Shared by RouteSpec, L3VPNRoute, etc. |
| `rib.Route` | `pkg/plugin/rib/rib.go` | Plugin storage | Full attributes as strings |
| `rr.Route` | `pkg/plugin/rr/rib.go` | Zero-copy | Minimal (MsgID, Family, Prefix) |
| `RIBRoute` | `pkg/plugin/types.go` | Query output | Minimal strings for JSON |
| `rib.Route` | `pkg/rib/route.go` | Core engine | NLRI, Attrs, wire cache, refcount |

**Note:** Two different `rib.Route` types exist in different packages.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/route-types.md` - Current state analysis
- [ ] `docs/architecture/encoding-context.md` - Zero-copy patterns
- [ ] `docs/architecture/update-building.md` - Wire format building
- [ ] `docs/architecture/rib-transition.md` - RIB → API transition (affects scope)

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

## Scope Decision

**Question:** Given `docs/architecture/rib-transition.md` (RIB moves to API programs), should this consolidation:

1. **Plugin-only** - Consolidate `pkg/plugin/` routes only (RouteSpec, rib.Route, rr.Route, RIBRoute)
2. **Full** - Also include `pkg/rib/route.go` (core engine Route)

The RIB transition suggests plugin routes become the primary storage, making option 1 more relevant.

## Files to Modify

- `pkg/plugin/rib/rib.go` - Add view methods to Route
- `pkg/plugin/rr/rib.go` - Use rib.Route.ForZeroCopy()
- `pkg/plugin/types.go` - Possibly remove duplicate RIBRoute
- `pkg/rib/route.go` - (if full scope) Align with plugin Route

## Implementation Steps

1. **Analyze usage** - Find all Route struct usages
2. **Add view methods** - To existing rib.Route
3. **Update rr plugin** - Use ForZeroCopy() view
4. **Update API output** - Use ForAPI() view
5. **Remove duplicates** - If safe
6. **Run tests** - `make test && make lint && make functional`

## Design Principles

**Modularity for plugins:** Route types should be designed so any plugin can easily:
- Store routes in whatever structure suits its needs
- Convert between representations without coupling to engine internals
- Use only the fields it needs (zero-copy uses MsgID only, full RIB uses all attrs)

**API stability guarantee:** Only the text and JSON APIs are stable. Go package structure, types, and interfaces may change without notice. Plugins should communicate via text/JSON protocol, not by importing Go packages.

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
