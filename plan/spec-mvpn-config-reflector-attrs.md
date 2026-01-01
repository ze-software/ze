# Spec: MVPN Config Route Reflector Attributes

## Task

Add OriginatorID and ClusterList fields to MVPNRouteConfig struct and convertMVPNRoute function so users can configure route reflector attributes (RFC 4456) for MVPN routes.

## Context

The grouping fix in `issue-mvpn-batch-interface.md` added OriginatorID and ClusterList to:
- `MVPNRoute` struct in `pkg/reactor/peersettings.go`
- `MVPNParams` struct in `pkg/bgp/message/update_build.go`
- `toMVPNParams` conversion in `pkg/reactor/peer.go`

However, the config layer was not updated, so users cannot configure these fields.

## Files to Read

- `pkg/config/bgp.go:590-602` - MVPNRouteConfig (missing fields)
- `pkg/config/bgp.go:605-621` - VPLSRouteConfig (reference: has OriginatorID/ClusterList)
- `pkg/config/loader.go:502-570` - convertMVPNRoute (needs parsing)
- `pkg/config/loader.go:632-663` - convertVPLSRoute (reference: parsing pattern)
- `.claude/zebgp/config/SYNTAX.md` - config syntax docs

## Current State

- Tests: All passing (11 MVPN grouping tests)
- Last commit: ee13d3d (docs: document BuildMVPN batch interface limitation)

## Implementation Steps

### 1. Write test (TDD)

**File: `pkg/config/loader_test.go`**

```go
// TestConvertMVPNRoute_OriginatorID verifies RFC 4456 originator-id parsing.
//
// VALIDATES: OriginatorID is parsed from config IP string to uint32.
// PREVENTS: Route reflector config silently ignored.
func TestConvertMVPNRoute_OriginatorID(t *testing.T) {
    mr := MVPNRouteConfig{
        RouteType:    "source-ad",
        OriginatorID: "192.168.1.1",
    }
    route, err := convertMVPNRoute(mr)
    require.NoError(t, err)
    require.Equal(t, uint32(0xC0A80101), route.OriginatorID)
}

// TestConvertMVPNRoute_ClusterList verifies RFC 4456 cluster-list parsing.
//
// VALIDATES: ClusterList is parsed from space-separated IPs to []uint32.
// PREVENTS: Route reflector config silently ignored.
func TestConvertMVPNRoute_ClusterList(t *testing.T) {
    mr := MVPNRouteConfig{
        RouteType:   "source-ad",
        ClusterList: "192.168.1.1 192.168.1.2",
    }
    route, err := convertMVPNRoute(mr)
    require.NoError(t, err)
    require.Equal(t, []uint32{0xC0A80101, 0xC0A80102}, route.ClusterList)
}
```

### 2. See test fail

```bash
go test ./pkg/config/... -run "TestConvertMVPNRoute_Originator|TestConvertMVPNRoute_Cluster"
# Should fail: unknown field OriginatorID/ClusterList
```

### 3. Implement

**File: `pkg/config/bgp.go` (lines 590-602)**

Add fields to MVPNRouteConfig:
```go
type MVPNRouteConfig struct {
    RouteType         string
    IsIPv6            bool
    RD                string
    SourceAS          uint32
    Source            string
    Group             string
    NextHop           string
    Origin            string
    LocalPreference   uint32
    MED               uint32
    ExtendedCommunity string
    OriginatorID      string // ADD: RFC 4456
    ClusterList       string // ADD: RFC 4456
}
```

**File: `pkg/config/loader.go` (after line 568)**

Add parsing in convertMVPNRoute (follow VPLSRoute pattern):
```go
// Parse originator-id (RFC 4456)
if mr.OriginatorID != "" {
    ip, err := netip.ParseAddr(mr.OriginatorID)
    if err != nil {
        return route, fmt.Errorf("parse originator-id: %w", err)
    }
    if ip.Is4() {
        b := ip.As4()
        route.OriginatorID = uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
    }
}

// Parse cluster-list (RFC 4456, space-separated IPs)
if mr.ClusterList != "" {
    parts := strings.Fields(mr.ClusterList)
    for _, p := range parts {
        p = strings.Trim(p, "[]")
        if p == "" {
            continue
        }
        ip, err := netip.ParseAddr(p)
        if err != nil {
            return route, fmt.Errorf("parse cluster-list: %w", err)
        }
        if ip.Is4() {
            b := ip.As4()
            route.ClusterList = append(route.ClusterList, uint32(b[0])<<24|uint32(b[1])<<16|uint32(b[2])<<8|uint32(b[3]))
        }
    }
}
```

### 4. See test pass

```bash
go test ./pkg/config/... -run "TestConvertMVPNRoute_Originator|TestConvertMVPNRoute_Cluster"
```

### 5. Run full suite

```bash
make test && make lint
```

## Config Syntax (for documentation)

After implementation, MVPN routes support:
```
announce {
    mvpn source-ad {
        rd 100:100;
        source 10.0.0.1;
        group 239.1.1.1;
        next-hop 192.168.1.1;
        originator-id 192.168.1.1;      # NEW
        cluster-list 10.0.0.1 10.0.0.2; # NEW
    }
}
```

## Checklist

- [ ] Test fails first
- [ ] Test passes after impl
- [ ] make test passes
- [ ] make lint passes
