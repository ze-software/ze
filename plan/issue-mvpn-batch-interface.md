# Issue: MVPN Route Grouping Missing Attribute Key

**Priority:** Medium (VPN isolation risk)
**Created:** 2026-01-01

## Problem

`groupMVPNRoutesByNextHop` groups routes **only by NextHop**, ignoring all other shared attributes:

```go
func groupMVPNRoutesByNextHop(routes []MVPNRoute) map[string][]MVPNRoute {
    for _, route := range routes {
        key := route.NextHop.String()  // ← Missing Origin, LocalPref, MED, ExtCommunityBytes!
        groups[key] = append(groups[key], route)
    }
}
```

This means routes with different attributes can be grouped together. When `BuildMVPN` takes `routes[0]` for shared attributes, the other routes' attribute values are silently lost.

Compare with `routeGroupKey` for unicast which includes **16 fields**.

## Impact

| Scenario | Consequence |
|----------|-------------|
| Different ExtCommunityBytes | **VPN isolation failure** - Route Targets lost, traffic leakage between customers |
| Different LocalPref/MED | Incorrect route selection, suboptimal forwarding |
| Different OriginatorID | Route reflector loop risk (RFC 4456 violation) |
| Different ClusterList | Route reflector loop risk (RFC 4456 violation) |

## Secondary Issue: Missing Reflector Attributes

`MVPNRoute` struct lacks OriginatorID and ClusterList:

```go
type MVPNRoute struct {
    // Has: Origin, LocalPreference, MED, ExtCommunityBytes
    // Missing: OriginatorID, ClusterList
}
```

## Current Behavior

```go
routes := []MVPNRoute{
    {NextHop: nh, ExtCommunityBytes: rtCustomerA, ...},
    {NextHop: nh, ExtCommunityBytes: rtCustomerB, ...},  // Different RT!
}
// groupMVPNRoutesByNextHop groups these together (same NextHop)
// BuildMVPN uses rtCustomerA for BOTH routes
// Customer B's routes get Customer A's Route Target → VPN isolation failure
```

## Fix

Follow the established pattern from unicast routes (`routeGroupKey`).

## Files to Modify

- `pkg/reactor/peersettings.go`: Add OriginatorID, ClusterList to MVPNRoute
- `pkg/reactor/peer.go`: Add mvpnRouteGroupKey, replace groupMVPNRoutesByNextHop with groupMVPNRoutesByKey, update toMVPNParams, delete old function
- `pkg/reactor/peer_test.go`: Add tests

## TDD Implementation

### Phase 1: Write Tests (MUST FAIL)

**File: `pkg/reactor/peer_test.go`**

```go
// TestMVPNRouteGroupKey_SeparatesDifferentExtCommunities verifies VPN isolation.
//
// VALIDATES: Routes with different Route Targets get different keys.
// PREVENTS: VPN isolation failure from incorrect grouping.
func TestMVPNRouteGroupKey_SeparatesDifferentExtCommunities(t *testing.T) {
    nh := netip.MustParseAddr("192.168.1.1")
    r1 := MVPNRoute{NextHop: nh, ExtCommunityBytes: []byte{0x00, 0x02, 0x00, 0x01}}
    r2 := MVPNRoute{NextHop: nh, ExtCommunityBytes: []byte{0x00, 0x02, 0x00, 0x02}}

    if mvpnRouteGroupKey(r1) == mvpnRouteGroupKey(r2) {
        t.Error("routes with different ExtCommunityBytes should have different keys")
    }
}

// TestMVPNRouteGroupKey_SeparatesDifferentOrigin verifies attribute separation.
//
// VALIDATES: Routes with different Origin get different keys.
// PREVENTS: Silent attribute loss in grouped updates.
func TestMVPNRouteGroupKey_SeparatesDifferentOrigin(t *testing.T) {
    nh := netip.MustParseAddr("192.168.1.1")
    r1 := MVPNRoute{NextHop: nh, Origin: 0}
    r2 := MVPNRoute{NextHop: nh, Origin: 1}

    if mvpnRouteGroupKey(r1) == mvpnRouteGroupKey(r2) {
        t.Error("routes with different Origin should have different keys")
    }
}

// TestMVPNRouteGroupKey_SeparatesDifferentOriginatorID verifies RR attribute separation.
//
// VALIDATES: Routes with different OriginatorID get different keys.
// PREVENTS: Route reflector loop from incorrect grouping.
func TestMVPNRouteGroupKey_SeparatesDifferentOriginatorID(t *testing.T) {
    nh := netip.MustParseAddr("192.168.1.1")
    r1 := MVPNRoute{NextHop: nh, OriginatorID: 0xC0A80101}
    r2 := MVPNRoute{NextHop: nh, OriginatorID: 0xC0A80102}

    if mvpnRouteGroupKey(r1) == mvpnRouteGroupKey(r2) {
        t.Error("routes with different OriginatorID should have different keys")
    }
}

// TestMVPNRouteGroupKey_SeparatesDifferentClusterList verifies RR attribute separation.
//
// VALIDATES: Routes with different ClusterList get different keys.
// PREVENTS: Route reflector loop from incorrect grouping.
func TestMVPNRouteGroupKey_SeparatesDifferentClusterList(t *testing.T) {
    nh := netip.MustParseAddr("192.168.1.1")
    r1 := MVPNRoute{NextHop: nh, ClusterList: []uint32{0xC0A80101}}
    r2 := MVPNRoute{NextHop: nh, ClusterList: []uint32{0xC0A80102}}

    if mvpnRouteGroupKey(r1) == mvpnRouteGroupKey(r2) {
        t.Error("routes with different ClusterList should have different keys")
    }
}

// TestMVPNRouteGroupKey_SameAttributesSameKey verifies batching preserved.
//
// VALIDATES: Routes with identical attributes get same key.
// PREVENTS: Unnecessary UPDATE fragmentation.
func TestMVPNRouteGroupKey_SameAttributesSameKey(t *testing.T) {
    nh := netip.MustParseAddr("192.168.1.1")
    rt := []byte{0x00, 0x02, 0x00, 0x01}
    r1 := MVPNRoute{NextHop: nh, Origin: 0, LocalPreference: 100, ExtCommunityBytes: rt}
    r2 := MVPNRoute{NextHop: nh, Origin: 0, LocalPreference: 100, ExtCommunityBytes: rt}

    if mvpnRouteGroupKey(r1) != mvpnRouteGroupKey(r2) {
        t.Error("routes with identical attributes should have same key")
    }
}

// TestGroupMVPNRoutesByKey_SeparatesDifferentRT verifies VPN isolation.
//
// VALIDATES: Routes with different Route Targets are in separate groups.
// PREVENTS: VPN traffic leakage between customers.
func TestGroupMVPNRoutesByKey_SeparatesDifferentRT(t *testing.T) {
    nh := netip.MustParseAddr("192.168.1.1")
    routes := []MVPNRoute{
        {NextHop: nh, ExtCommunityBytes: []byte{0x00, 0x02, 0x00, 0x01}},
        {NextHop: nh, ExtCommunityBytes: []byte{0x00, 0x02, 0x00, 0x02}},
    }
    groups := groupMVPNRoutesByKey(routes)
    if len(groups) != 2 {
        t.Errorf("expected 2 groups for different RTs, got %d", len(groups))
    }
}
```

Run tests → MUST FAIL (functions don't exist)

### Phase 2: Add Fields to MVPNRoute

**File: `pkg/reactor/peersettings.go`**

```go
type MVPNRoute struct {
    RouteType         uint8
    IsIPv6            bool
    RD                [8]byte
    SourceAS          uint32
    Source            netip.Addr
    Group             netip.Addr
    NextHop           netip.Addr
    Origin            uint8
    LocalPreference   uint32
    MED               uint32
    ExtCommunityBytes []byte
    OriginatorID      uint32     // ADD: RFC 4456
    ClusterList       []uint32   // ADD: RFC 4456
}
```

### Phase 3: Implement Grouping Function

**File: `pkg/reactor/peer.go`**

```go
// mvpnRouteGroupKey generates a grouping key for MVPN routes.
// Routes with identical keys can share path attributes in one UPDATE.
//
// Fields NOT in key (per-NLRI, not per-UPDATE):
// - IsIPv6: Routes pre-separated by AFI before grouping
// - RouteType: Multiple types allowed in same UPDATE
// - RD: Per-NLRI field in MP_REACH_NLRI
func mvpnRouteGroupKey(r MVPNRoute) string {
    return fmt.Sprintf("%s|%d|%d|%d|%s|%d|%v",
        r.NextHop.String(),
        r.Origin,
        r.LocalPreference,
        r.MED,
        hex.EncodeToString(r.ExtCommunityBytes),
        r.OriginatorID,
        r.ClusterList,
    )
}

func groupMVPNRoutesByKey(routes []MVPNRoute) map[string][]MVPNRoute {
    groups := make(map[string][]MVPNRoute)
    for _, route := range routes {
        key := mvpnRouteGroupKey(route)
        groups[key] = append(groups[key], route)
    }
    return groups
}
```

Run tests → MUST PASS

### Phase 4: Update Conversion and Callers

**File: `pkg/reactor/peer.go`**

Update toMVPNParams:
```go
func toMVPNParams(routes []MVPNRoute) []message.MVPNParams {
    params := make([]message.MVPNParams, len(routes))
    for i, r := range routes {
        params[i] = message.MVPNParams{
            RouteType: r.RouteType, IsIPv6: r.IsIPv6, RD: r.RD,
            SourceAS: r.SourceAS, Source: r.Source, Group: r.Group,
            NextHop: r.NextHop, Origin: attribute.Origin(r.Origin),
            LocalPreference: r.LocalPreference, MED: r.MED,
            ExtCommunityBytes: r.ExtCommunityBytes,
            OriginatorID: r.OriginatorID,   // ADD
            ClusterList:  r.ClusterList,    // ADD
        }
    }
    return params
}
```

Update sendMVPNRoutes:
```go
ipv4Groups := groupMVPNRoutesByKey(ipv4Routes)
ipv6Groups := groupMVPNRoutesByKey(ipv6Routes)
```

### Phase 5: Delete Dead Code and Verify

```bash
# Delete groupMVPNRoutesByNextHop
# Run full test suite
make test && make lint
```

## Design Notes

### Fields Excluded from Key (Correct)

| Field | Reason |
|-------|--------|
| IsIPv6 | Routes pre-separated by IPv4/IPv6 in sendMVPNRoutes before grouping |
| RouteType | Per-NLRI field; multiple route types (5,6,7) can share UPDATE attributes |
| RD | Per-NLRI field in MP_REACH_NLRI; multiple RDs allowed in one UPDATE |

### Key Format

Uses `%v` for ClusterList (`[]uint32`), consistent with `routeGroupKey` pattern. Not ideal but matches existing codebase style.

## Scope Check

Other grouping functions verified safe:
- FlowSpec, MUP, VPLS, VPN, LabeledUnicast: Single-route APIs, no grouping bug possible
- Unicast: Uses `routeGroupKey` with 16 fields, correct

## Why Not Change BuildMVPN API?

The `BuildMVPN` API is correct. The caller's contract is to pass routes with identical shared attributes. The bug is that `groupMVPNRoutesByNextHop` violates this contract.

This matches how unicast routes work: `routeGroupKey` ensures only compatible routes are grouped, then `BuildGroupedUnicast` safely uses `routes[0]`.

## Rejected Alternatives

| Option | Why Rejected |
|--------|--------------|
| Validate in BuildMVPN | Wrong layer - caller should never pass bad data |
| Single-route API | Loses MVPN batching performance unnecessarily |
| Split shared/per-route params | Adds complexity, doesn't fix root cause |
| Document only | Acknowledges bug, doesn't fix it |
