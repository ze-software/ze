# Pool System Integration Plan

**Created:** 2025-12-22
**Status:** Planned

## Problem Summary

RouteStore exists but is **completely unused**:
- Created in `Reactor.New()` at line 883
- Only accessed for `Stop()` cleanup at line 1128
- Never passed to Peer, Session, API, or CommitManager
- **60+ direct attribute constructions** bypass deduplication

## Architecture Decision

**Chosen Approach: Pass RouteStore Through Components**

Why not singleton/global:
- Harder to test (global state)
- Violates existing patterns (everything passed explicitly)
- Can't have multiple reactors (testing, future)

## Files to Modify

| File | Changes |
|------|---------|
| `pkg/reactor/reactor.go` | Pass `ribStore` to Peer creation |
| `pkg/reactor/peer.go` | Store reference, use for attribute interning |
| `pkg/api/commit.go` | Access store via CommandContext |
| `pkg/rib/commit.go` | Accept store parameter |
| `pkg/rib/update.go` | Accept store parameter |

## Implementation Phases

### Phase 1: Wire RouteStore to Peer

**Goal:** Make RouteStore accessible where routes are created

1. Add `routeStore *rib.RouteStore` field to `Peer` struct
2. Pass `r.ribStore` in `NewPeer()` call (reactor.go)
3. Store reference in Peer

```go
// peer.go - add field
type Peer struct {
    // ...existing fields...
    routeStore *rib.RouteStore
}

// reactor.go - pass in NewPeer
peer := NewPeer(ctx, settings, r.ribStore, ...)
```

### Phase 2: Create Attribute Factory Methods

**Goal:** Centralize attribute creation with optional interning

Add factory methods to RouteStore:

```go
// pkg/rib/store.go
func (rs *RouteStore) NewASPath(segments []attribute.ASPathSegment) *attribute.ASPath {
    asp := &attribute.ASPath{Segments: segments}
    return rs.InternAttribute(asp).(*attribute.ASPath)
}

func (rs *RouteStore) NewNextHop(addr netip.Addr) *attribute.NextHop {
    nh := &attribute.NextHop{Addr: addr}
    return rs.InternAttribute(nh).(*attribute.NextHop)
}
```

### Phase 3: Replace Direct Constructions

**Batch 1: peer.go (20 violations)**
- Lines 458, 465, 468: ASPath → `p.routeStore.NewASPath(...)`
- Lines 478, 724, 1065: NextHop → `p.routeStore.NewNextHop(...)`
- Lines 504, 744: Aggregator → `p.routeStore.NewAggregator(...)`
- Lines 706, 712, 714, 1052, 1054, 1265, 1267, 1457, 1459, 1645, 1647: ASPath

**Batch 2: reactor.go (4 violations)**
- Line 162, 268: ASPath
- Line 276: NextHop
- Line 782: MPUnreachNLRI

**Batch 3: commit.go / rib files (12 violations)**
- `pkg/api/commit.go`: Lines 214, 259, 280, 284, 301
- `pkg/rib/commit.go`: Lines 214, 259, 280, 284, 301, 346
- `pkg/rib/update.go`: Line 53

### Phase 4: Add Release Calls

Where routes are withdrawn, add release:

```go
// When removing route from RIB
routeStore.ReleaseRoute(route)
```

Key locations:
- `OutgoingRIB.Withdraw()` - when route removed from sent
- `CommitManager` transaction cleanup
- Peer shutdown

### Phase 5: Update Production Code Tests

1. Update existing tests to use factory methods
2. Add integration test: route lifecycle through pool
3. Add metrics test: verify dedup rate > 0

### Phase 6: Update All Tests

Search and replace in test files:
```bash
grep -rn "NewRoute(" pkg/ --include="*_test.go"
grep -rn "&attribute\." pkg/ --include="*_test.go"
```

**Approach:** Create `rib.NewTestRouteStore()` that tests can use.

## Execution Order

```
1. Phase 1 (wiring)       - 2 files, low risk
2. Phase 2 (factories)    - 1 file, additive
3. Phase 3 (replacements) - 4 files, mechanical
4. Phase 4 (releases)     - 3 files, careful
5. Phase 5 (tests)        - validation
6. Phase 6 (all tests)    - 50+ files
```

## Risks

| Risk | Mitigation |
|------|------------|
| Reference count bugs | Add debug assertions |
| Performance regression | Benchmark before/after |
| Breaking tests | Run `make test` after each phase |

## Success Criteria

- [ ] `make test && make lint` passes
- [ ] No direct `&attribute.*{}` in production code
- [ ] RouteStore metrics show dedup hits > 0
- [ ] Memory usage lower with duplicate routes

## Decisions

- **MPReachNLRI/MPUnreachNLRI**: ✅ Intern for consistency
- **All attributes**: ✅ Intern everything through pool
- **Test files**: ✅ Update all 50+ locations for consistency
