# Spec: PathAttributes Removal

## Task

Replace `plugin.PathAttributes` struct with `attribute.Builder` for wire-first attribute construction. This completes the buffer-first migration for INPUT paths (building routes from user commands).

## Context

The buffer-first migration (spec-buffer-first-migration.md) completed OUTPUT paths:
- Iterators for reading wire bytes
- Direct formatting from buffers
- `rib.RouteJSON` for JSON serialization

This spec addresses INPUT paths:
- Building wire bytes from user commands
- Eliminating intermediate parsed representations

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/buffer-architecture.md` - Target architecture
- [ ] `docs/architecture/update-building.md` - Wire format construction

### Source Code
- [ ] `pkg/bgp/attribute/builder.go` - New Builder (already implemented)
- [ ] `pkg/plugin/types.go` - PathAttributes (to be replaced)
- [ ] `pkg/plugin/route.go` - Route parsing
- [ ] `pkg/plugin/update_text.go` - Text command parsing

## Problem Statement

`PathAttributes` is an intermediate representation for building routes:

```
text command → PathAttributes → []attribute.Attribute → wire bytes
```

With `attribute.Builder`, we can go directly:

```
text command → attribute.Builder → wire bytes
```

## Current Usage Analysis

### PathAttributes Definition (pkg/plugin/types.go:98)
```go
type PathAttributes struct {
    Origin              *uint8
    LocalPreference     *uint32
    MED                 *uint32
    ASPath              []uint32
    Communities         []uint32
    LargeCommunities    []LargeCommunity
    ExtendedCommunities []ExtendedCommunity
    AtomicAggregate     bool
    Wire                *AttributesWire  // Pre-built wire (for forwarding)
}
```

### Types Embedding PathAttributes
| Type | File | Purpose |
|------|------|---------|
| `RouteSpec` | types.go:122 | IPv4/IPv6 unicast routes |
| `L3VPNRouteSpec` | types.go:190 | L3VPN routes |
| `FlowSpecRoute` | types.go:202 | FlowSpec routes |
| `MUPRouteSpec` | types.go:220 | MUP routes |
| `parsedAttrs` | update_text.go:108 | Accumulated attrs during parsing |
| `NLRIGroup` | types.go:591 | Grouped NLRIs with shared attrs |
| `NLRIBatch` | types.go:611 | Batch for UPDATE building |

### Key Functions Using PathAttributes
| Function | File | Purpose |
|----------|------|---------|
| `parseCommonAttribute` | route.go:448 | Parses origin, as-path, community, etc. |
| `buildBatchAttributes` | reactor.go:1675 | Converts to []attribute.Attribute |
| `snapshot` | update_text.go:319 | Creates PathAttributes copy |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates |
|------|------|-----------|
| `TestBuilderFromText` | `pkg/bgp/attribute/builder_text_test.go` | Parse text → Builder |
| `TestRouteSpecWithBuilder` | `pkg/plugin/route_test.go` | RouteSpec uses Builder |
| `TestParseOrigin` | `pkg/plugin/route_test.go` | origin igp/egp/incomplete |
| `TestParseASPath` | `pkg/plugin/route_test.go` | as-path [65001 65002] |
| `TestParseCommunity` | `pkg/plugin/route_test.go` | community 65000:100 |
| `TestParseLargeCommunity` | `pkg/plugin/route_test.go` | large-community 65000:1:2 |

### Functional Tests
| Test | Location | Scenario |
|------|----------|----------|
| `announce-builder` | `qa/tests/plugin/` | Full announce via Builder |
| `withdraw-builder` | `qa/tests/plugin/` | Withdraw uses same path |

## Implementation Phases

### Phase 1: Add Builder Text Parsing

Extend `attribute.Builder` with text parsing methods:

```go
// pkg/bgp/attribute/builder.go
func (b *Builder) ParseOrigin(s string) error      // "igp", "egp", "incomplete"
func (b *Builder) ParseASPath(s string) error      // "[65001 65002]" or "65001 65002"
func (b *Builder) ParseCommunity(s string) error   // "65000:100" or "no-export"
func (b *Builder) ParseLargeCommunity(s string) error
func (b *Builder) ParseExtCommunity(s string) error
```

**Files to modify:**
- `pkg/bgp/attribute/builder.go` - Add parse methods
- `pkg/bgp/attribute/builder_test.go` - Add parse tests

### Phase 2: Create RouteBuilder

New type that combines NLRI + Builder:

```go
// pkg/plugin/route_builder.go
type RouteBuilder struct {
    Prefix    netip.Prefix
    PathID    uint32
    NextHop   netip.Addr
    Family    nlri.Family
    Attrs     *attribute.Builder
}

func NewRouteBuilder() *RouteBuilder
func (rb *RouteBuilder) Build() (*rib.Route, error)
```

**Files to create:**
- `pkg/plugin/route_builder.go`
- `pkg/plugin/route_builder_test.go`

### Phase 3: Migrate parseCommonAttribute

Update to populate Builder instead of PathAttributes:

```go
// Current:
func parseCommonAttribute(key string, args []string, idx int, attrs *PathAttributes) (int, error)

// New:
func parseCommonAttribute(key string, args []string, idx int, builder *attribute.Builder) (int, error)
```

**Files to modify:**
- `pkg/plugin/route.go`
- `pkg/plugin/update_text.go`

### Phase 4: Migrate Route Spec Types

Replace embedded PathAttributes with Builder:

```go
// Current:
type RouteSpec struct {
    PathAttributes
    Prefix  string
    NextHop string
    PathID  uint32
}

// New:
type RouteSpec struct {
    Prefix  string
    NextHop string
    PathID  uint32
    Attrs   *attribute.Builder
}
```

**Files to modify:**
- `pkg/plugin/types.go` - All route spec types
- `pkg/plugin/route.go` - Parsing functions
- `pkg/reactor/reactor.go` - buildBatchAttributes → use Builder.Build()

### Phase 5: Remove PathAttributes

After all usages migrated:
- Remove `PathAttributes` struct from types.go
- Remove `buildBatchAttributes` from reactor.go
- Update all tests

### Phase 6: Remove rr.UpdateInfo (Optional)

If feasible, also migrate:
- `rr.UpdateInfo` - JSON event input
- Requires updating rr server event handling

## Files Summary

### Create
| File | Purpose |
|------|---------|
| `pkg/plugin/route_builder.go` | RouteBuilder type |
| `pkg/plugin/route_builder_test.go` | RouteBuilder tests |

### Modify
| File | Changes |
|------|---------|
| `pkg/bgp/attribute/builder.go` | Add text parse methods |
| `pkg/plugin/types.go` | Replace PathAttributes with Builder |
| `pkg/plugin/route.go` | Update parsing to use Builder |
| `pkg/plugin/update_text.go` | Update parsing to use Builder |
| `pkg/reactor/reactor.go` | Remove buildBatchAttributes |

### Remove
| File/Type | Replacement |
|-----------|-------------|
| `PathAttributes` | `*attribute.Builder` |
| `buildBatchAttributes` | `Builder.Build()` |

## Migration Strategy

To minimize breakage, migrate in order:
1. Add new Builder parsing (non-breaking)
2. Create RouteBuilder (non-breaking)
3. Add Attrs *Builder field to route specs (parallel to PathAttributes)
4. Migrate parsing one function at a time
5. Remove PathAttributes after all parsing migrated

## Checklist

### 🧪 TDD
- [ ] Phase 1 tests written and FAIL
- [ ] Phase 1 implementation complete, tests PASS
- [ ] Phase 2 tests written and FAIL
- [ ] Phase 2 implementation complete, tests PASS
- [ ] Phase 3 implementation complete, tests PASS
- [ ] Phase 4 implementation complete, tests PASS
- [ ] Phase 5 PathAttributes removed, tests PASS

### Verification
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make functional` passes
- [ ] All announce/withdraw commands work

### Documentation
- [ ] `docs/architecture/buffer-architecture.md` updated
- [ ] Builder usage documented

### Completion
- [ ] Spec moved to `docs/plan/done/NNN-pathattributes-removal.md`
