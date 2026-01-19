# Spec: Route Grouping for Efficient UPDATE Packing

## Status: Implemented

## Task

Optimize `sendRoutesWithLimit()` to group routes by attributes and pack multiple NLRIs per UPDATE message, reducing UPDATE count from O(routes) to O(routes/capacity).

## Required Reading (MUST complete before implementation)

- [x] `.claude/zebgp/UPDATE_BUILDING.md` - Build vs Forward paths
- [x] `.claude/zebgp/wire/NLRI.md` - MP_REACH_NLRI structure
- [x] `internal/bgp/message/update_build.go` - Existing `BuildGroupedUnicastWithLimit`

## Approach: Reuse Existing Infrastructure

Use existing `BuildGroupedUnicastWithLimit` pattern:

```go
// Existing for static routes (peer.go:1125)
ub := message.NewUpdateBuilder(localAS, isIBGP, ctx)
params := toStaticRouteUnicastParams(routes, nf)
updates, err := ub.BuildGroupedUnicastWithLimit(params, maxMsgSize)
```

**For forwarded routes:** Create `toRIBRouteUnicastParams()` adapter function.

## Files to Modify

| File | Changes |
|------|---------|
| `internal/reactor/peer.go` | Add `GroupUpdate bool` to `PeerSettings` (default: true) |
| `internal/reactor/reactor.go` | Add `toRIBRouteUnicastParams()`, `groupRoutesByAttributes()`, modify `sendRoutesWithLimit()` |
| `internal/config/neighbor.go` | Add `group-update` config option |
| `internal/bgp/attribute/attributes.go` | Add `Hash()` method |
| `internal/bgp/message/update_build.go` | Add `BuildGroupedMPReachWithLimit()`, `buildMPReachUpdate()`, `packGroupedAttributesBase()` |
| `internal/bgp/message/errors.go` | Add `ErrAttributesTooLarge`, `ErrNLRITooLarge` |

## Current State

- `sendRoutesWithLimit()` at `internal/reactor/reactor.go:1777` sends one route per UPDATE
- `BuildGroupedUnicastWithLimit` exists for IPv4 unicast (static routes)
- No grouping for MP families (IPv6, VPN)

## Problem

```go
for _, route := range routes {
    update := buildRIBRouteUpdate(route, ...)
    peer.sendUpdateWithSplit(update, maxMsgSize, family)  // One UPDATE per route
}
```

**Impact:** 1000 routes = 1000 UPDATEs instead of 1-2.

## Configuration

```
neighbor 10.0.0.1 {
    group-update true;   # Default - pack multiple NLRIs per UPDATE
}

neighbor 10.0.0.2 {
    group-update false;  # One route per UPDATE (legacy)
}
```

## Design

### Phase 1: Add Attributes.Hash()

Prerequisite for grouping - routes with identical hash can share an UPDATE:

```go
// Hash returns a hash of attribute values for grouping.
// Routes with identical Hash() can share an UPDATE.
// Note: PathID is NOT included - routes with different path-IDs but same attrs CAN be grouped.
func (a *Attributes) Hash() uint64 {
    h := fnv.New64a()
    binary.Write(h, binary.BigEndian, a.Origin)
    for _, asn := range a.ASPath.ToSlice() {
        binary.Write(h, binary.BigEndian, asn)
    }
    h.Write(a.NextHop.AsSlice())
    binary.Write(h, binary.BigEndian, a.MED)
    binary.Write(h, binary.BigEndian, a.LocalPreference)
    for _, c := range a.Communities {
        binary.Write(h, binary.BigEndian, c)
    }
    h.Write(a.ExtCommunityBytes)
    // ... other fields (ORIGINATOR_ID, CLUSTER_LIST, etc.)
    return h.Sum64()
}
```

### Phase 2: Convert RIB Route to UnicastParams

```go
// toRIBRouteUnicastParams converts a RIB route to UnicastParams for grouping.
func toRIBRouteUnicastParams(route *rib.Route) message.UnicastParams {
    attrs := route.Attributes()

    return message.UnicastParams{
        Prefix:            route.NLRI().Prefix(),
        PathID:            route.PathID(),
        NextHop:           route.NextHop(),
        Origin:            attrs.Origin,
        ASPath:            attrs.ASPath.ToSlice(),
        MED:               attrs.MED,
        LocalPreference:   attrs.LocalPreference,
        Communities:       attrs.Communities,
        ExtCommunityBytes: attrs.ExtCommunityBytes,
        LargeCommunities:  attrs.LargeCommunities,
        AtomicAggregate:   attrs.AtomicAggregate,
        HasAggregator:     attrs.HasAggregator,
        AggregatorASN:     attrs.AggregatorASN,
        AggregatorIP:      attrs.AggregatorIP,
        OriginatorID:      attrs.OriginatorID,
        ClusterList:       attrs.ClusterList,
    }
}
```

### Phase 3: Group by Attributes

Routes can be grouped if they have identical attributes AND same family:

```go
// routeGroupKey identifies routes that can share an UPDATE.
type routeGroupKey struct {
    attrHash uint64       // Hash of attribute values
    family   nlri.Family  // Must match - IPv4 vs IPv6 use different encoding
}

// groupRoutesByAttributes groups routes with identical attributes.
func groupRoutesByAttributes(routes []*rib.Route) map[routeGroupKey][]*rib.Route {
    groups := make(map[routeGroupKey][]*rib.Route)
    for _, route := range routes {
        key := routeGroupKey{
            attrHash: route.Attributes().Hash(),
            family:   route.NLRI().Family(),
        }
        groups[key] = append(groups[key], route)
    }
    return groups
}
```

### Phase 4: Add BuildGroupedMPReachWithLimit

For MP families (IPv6, VPN), NLRIs go inside MP_REACH_NLRI:

```go
// BuildGroupedMPReachWithLimit builds multiple UPDATEs for MP families.
// Similar to BuildGroupedUnicastWithLimit but packs NLRIs into MP_REACH_NLRI.
func (ub *UpdateBuilder) BuildGroupedMPReachWithLimit(routes []UnicastParams, family nlri.Family, maxSize int) ([]*Update, error) {
    if len(routes) == 0 {
        return nil, nil
    }

    // Build base attributes (without MP_REACH_NLRI) from first route
    baseAttrs := ub.packGroupedAttributesBase(routes[0])

    // Calculate overhead
    // Header(19) + WithdrawnLen(2) + AttrLen(2) + BaseAttrs + MP_REACH header(4) + AFI(2) + SAFI(1) + NH_len(1) + NH + Reserved(1)
    nhLen := nextHopLength(family, routes[0].NextHop)
    mpReachOverhead := 4 + 2 + 1 + 1 + nhLen + 1
    overhead := HeaderLen + 4 + len(baseAttrs) + mpReachOverhead

    if overhead > maxSize {
        return nil, ErrAttributesTooLarge
    }

    available := maxSize - overhead

    var updates []*Update
    var nlriBytes []byte

    for _, r := range routes {
        inet := nlri.NewINET(family, r.Prefix, r.PathID)
        packed := inet.Pack(ub.Ctx)

        if len(packed) > available {
            return nil, ErrNLRITooLarge
        }

        if len(nlriBytes)+len(packed) > available && len(nlriBytes) > 0 {
            update := ub.buildMPReachUpdate(baseAttrs, family, routes[0].NextHop, nlriBytes)
            updates = append(updates, update)
            nlriBytes = nil
        }
        nlriBytes = append(nlriBytes, packed...)
    }

    if len(nlriBytes) > 0 {
        update := ub.buildMPReachUpdate(baseAttrs, family, routes[0].NextHop, nlriBytes)
        updates = append(updates, update)
    }

    return updates, nil
}

// buildMPReachUpdate constructs UPDATE with MP_REACH_NLRI containing packed NLRIs.
func (ub *UpdateBuilder) buildMPReachUpdate(baseAttrs []byte, family nlri.Family, nextHop netip.Addr, nlriBytes []byte) *Update {
    // Build MP_REACH_NLRI: AFI(2) + SAFI(1) + NH_len(1) + NextHop + Reserved(1) + NLRIs
    nhBytes := nextHop.AsSlice()
    nhLen := len(nhBytes)
    valueLen := 2 + 1 + 1 + nhLen + 1 + len(nlriBytes)

    value := make([]byte, valueLen)
    binary.BigEndian.PutUint16(value[0:2], uint16(family.AFI))
    value[2] = byte(family.SAFI)
    value[3] = byte(nhLen)
    copy(value[4:4+nhLen], nhBytes)
    value[4+nhLen] = 0 // Reserved
    copy(value[5+nhLen:], nlriBytes)

    // Build attribute header (Optional, 0x80 or 0x90 for extended)
    var mpReach []byte
    if valueLen > 255 {
        mpReach = make([]byte, 4+valueLen)
        mpReach[0] = 0x90 // Optional + Extended
        mpReach[1] = byte(attribute.AttrMPReachNLRI)
        binary.BigEndian.PutUint16(mpReach[2:4], uint16(valueLen))
        copy(mpReach[4:], value)
    } else {
        mpReach = make([]byte, 3+valueLen)
        mpReach[0] = 0x80 // Optional
        mpReach[1] = byte(attribute.AttrMPReachNLRI)
        mpReach[2] = byte(valueLen)
        copy(mpReach[3:], value)
    }

    // Combine base attrs + MP_REACH_NLRI
    allAttrs := make([]byte, len(baseAttrs)+len(mpReach))
    copy(allAttrs, baseAttrs)
    copy(allAttrs[len(baseAttrs):], mpReach)

    return &Update{PathAttributes: allAttrs}
}

// packGroupedAttributesBase builds attribute bytes WITHOUT NEXT_HOP (for MP families).
// For MP families, next-hop goes in MP_REACH_NLRI, not as a separate attribute.
func (ub *UpdateBuilder) packGroupedAttributesBase(r UnicastParams) []byte {
    var buf bytes.Buffer

    // ORIGIN (code 1) - Well-known mandatory
    buf.Write([]byte{0x40, 0x01, 0x01, byte(r.Origin)})

    // AS_PATH (code 2) - Well-known mandatory
    asPathBytes := ub.packASPath(r.ASPath)
    buf.Write(asPathBytes)

    // NOTE: NEXT_HOP (code 3) is OMITTED for MP families
    // Next-hop goes in MP_REACH_NLRI instead

    // MED (code 4) - Optional non-transitive
    if r.MED != 0 {
        buf.Write([]byte{0x80, 0x04, 0x04})
        binary.Write(&buf, binary.BigEndian, r.MED)
    }

    // LOCAL_PREF (code 5) - Well-known (iBGP only)
    if ub.IsIBGP {
        buf.Write([]byte{0x40, 0x05, 0x04})
        binary.Write(&buf, binary.BigEndian, r.LocalPreference)
    }

    // ... other attributes (communities, etc.)
    // Same as packGroupedAttributes but without NEXT_HOP

    return buf.Bytes()
}

// nextHopLength returns the next-hop length for a given family and address.
func nextHopLength(family nlri.Family, nh netip.Addr) int {
    if nh.Is4() {
        return 4
    }
    // IPv6: 16 bytes (global only)
    // Note: Link-local (32 bytes) not currently supported
    return 16
}
```

### Phase 5: Integrate into sendRoutesWithLimit

```go
func (a *reactorAPIAdapter) sendRoutesWithLimit(peer *Peer, routes []*rib.Route, maxMsgSize int) error {
    if len(routes) == 0 {
        return nil
    }

    // Check config
    if !peer.settings.GroupUpdate {
        return a.sendRoutesIndividually(peer, routes, maxMsgSize)
    }

    // Group routes by attributes
    groups := groupRoutesByAttributes(routes)

    var errs []error

    for key, groupRoutes := range groups {
        family := key.family
        ctx := peer.packContext(family)
        ub := message.NewUpdateBuilder(peer.settings.LocalAS, peer.settings.IsIBGP(), ctx)

        // Convert to UnicastParams
        params := make([]message.UnicastParams, len(groupRoutes))
        for i, r := range groupRoutes {
            params[i] = toRIBRouteUnicastParams(r)
        }

        var updates []*message.Update
        var err error

        if family.AFI == nlri.AFIIPv4 && family.SAFI == nlri.SAFIUnicast {
            // IPv4 unicast: use existing function
            updates, err = ub.BuildGroupedUnicastWithLimit(params, maxMsgSize)
        } else {
            // MP families: use new function
            updates, err = ub.BuildGroupedMPReachWithLimit(params, family, maxMsgSize)
        }

        if err != nil {
            errs = append(errs, err)
            continue
        }

        for _, update := range updates {
            if err := peer.SendUpdate(update); err != nil {
                errs = append(errs, err)
            }
        }
    }

    if len(errs) > 0 {
        return errors.Join(errs...)
    }
    return nil
}

func (a *reactorAPIAdapter) sendRoutesIndividually(peer *Peer, routes []*rib.Route, maxMsgSize int) error {
    var errs []error
    for _, route := range routes {
        family := route.NLRI().Family()
        ctx := peer.packContext(family)
        update := buildRIBRouteUpdate(route, peer.settings.LocalAS, peer.settings.IsIBGP(), ctx)
        if err := peer.sendUpdateWithSplit(update, maxMsgSize, family); err != nil {
            errs = append(errs, err)
        }
    }
    if len(errs) > 0 {
        return errors.Join(errs...)
    }
    return nil
}
```

## Expected Results

| Scenario | Before | After |
|----------|--------|-------|
| 1000 IPv4 /24 routes, same attrs | 1000 UPDATEs | 1-2 UPDATEs |
| 1000 IPv4 routes, 10 attr groups | 1000 UPDATEs | 10-20 UPDATEs |
| 1000 IPv6 /48 routes, same attrs | 1000 UPDATEs | 2-5 UPDATEs |

## Implementation Steps

1. Add `GroupUpdate bool` to `PeerSettings` (default: true)
2. Add `group-update` config parsing
3. Add error types `ErrAttributesTooLarge`, `ErrNLRITooLarge`
4. Write test `TestAttributesHash` (TDD)
5. Implement `Attributes.Hash()`
6. Write test `TestToRIBRouteUnicastParams` (TDD)
7. Implement `toRIBRouteUnicastParams()`
8. Write test `TestGroupRoutesByAttributes` (TDD)
9. Implement `groupRoutesByAttributes()`
10. Write test `TestBuildMPReachUpdate` (TDD)
11. Implement `nextHopLength()`, `buildMPReachUpdate()`
12. Write test `TestPackGroupedAttributesBase` (TDD)
13. Implement `packGroupedAttributesBase()`
14. Write test `TestBuildGroupedMPReachWithLimit` (TDD)
15. Implement `BuildGroupedMPReachWithLimit()`
16. Modify `sendRoutesWithLimit()` to use grouping
17. Write integration test `TestGroupingReducesUpdateCount`
18. Run `make test && make lint && make functional`

## Checklist

- [x] `GroupUpdates` added to PeerSettings (default: true)
- [x] Error types added (`ErrAttributesTooLarge`, `ErrNLRITooLarge`) - already existed
- [x] `toRIBRouteUnicastParams()` implemented and tested
- [x] `rib.GroupByAttributesTwoLevel()` used for grouping (already existed)
- [x] `sendGroupedMPFamily()` implemented for MP families
- [x] `buildGroupedMPUpdate()` implemented
- [x] `sendRoutesWithLimit()` modified to use grouping
- [x] `TestGroupingReducesUpdateCount` passes (as TestGroupedSendReducesUpdateCount)
- [x] make test passes
- [x] make lint passes
- [x] make functional passes

## Testing Strategy

```go
// TestAttributesHash verifies attribute hashing for grouping.
// VALIDATES: Same attributes produce same hash, different attrs produce different hash.
// PREVENTS: Incorrect grouping due to hash collisions or misses.
func TestAttributesHash(t *testing.T)

// TestToRIBRouteUnicastParams verifies Route to UnicastParams conversion.
// VALIDATES: All attribute fields correctly extracted.
// PREVENTS: Lost attributes during conversion.
func TestToRIBRouteUnicastParams(t *testing.T)

// TestGroupRoutesByAttributes verifies attribute-based grouping.
// VALIDATES: Routes with identical attrs grouped, different attrs separated.
// VALIDATES: Routes with different families NOT grouped together.
// PREVENTS: Wrong routes grouped together.
func TestGroupRoutesByAttributes(t *testing.T)

// TestBuildMPReachUpdate verifies MP_REACH_NLRI construction.
// VALIDATES: Correct wire format, AFI/SAFI/NextHop/NLRIs properly encoded.
// PREVENTS: Malformed MP_REACH_NLRI attribute.
func TestBuildMPReachUpdate(t *testing.T)

// TestPackGroupedAttributesBase verifies attribute packing without NEXT_HOP.
// VALIDATES: NEXT_HOP attribute NOT included (goes in MP_REACH instead).
// PREVENTS: RFC violation - duplicate next-hop in MP families.
func TestPackGroupedAttributesBase(t *testing.T)

// TestBuildGroupedMPReachWithLimit verifies MP family packing.
// VALIDATES: Multiple NLRIs packed into MP_REACH_NLRI, respects size limit.
// PREVENTS: Oversized UPDATEs, incorrect MP_REACH encoding.
func TestBuildGroupedMPReachWithLimit(t *testing.T)

// TestGroupUpdateOptionDisabled verifies legacy behavior.
// VALIDATES: group-update=false sends one route per UPDATE.
// PREVENTS: Grouping when disabled.
func TestGroupUpdateOptionDisabled(t *testing.T)

// TestGroupingReducesUpdateCount verifies efficiency.
// VALIDATES: 1000 same-attr routes → 1-2 UPDATEs.
// PREVENTS: Regression to one-route-per-UPDATE.
func TestGroupingReducesUpdateCount(t *testing.T)
```

## Advantages of This Approach

1. **Code reuse:** Builds on existing `BuildGroupedUnicastWithLimit`
2. **Tested patterns:** Same approach as static route grouping
3. **Less new code:** ~100 lines vs ~300 lines
4. **Simpler:** No wire-format parsing, just attribute extraction

## Trade-offs

1. **Re-encoding:** Attributes are re-encoded from parsed structures
2. **CPU cost:** Slightly higher than wire-format preservation
3. **Acceptable:** We're already in the slow path (can't do zero-copy)

---

**Created:** 2026-01-02
**Updated:** 2026-01-02 - Simplified to reuse existing infrastructure
