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

- **IMPLEMENTED** (pending commit)
- Tests: All passing (11 MVPN grouping + 4 new config tests)
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

## Additional Tests (from review)

### Error handling test

```go
// TestConvertMVPNRoute_InvalidOriginatorID verifies error on bad IP.
//
// VALIDATES: Invalid originator-id returns descriptive error.
// PREVENTS: Silent failure on malformed config.
func TestConvertMVPNRoute_InvalidOriginatorID(t *testing.T) {
    mr := MVPNRouteConfig{
        RouteType:    "source-ad",
        OriginatorID: "not-an-ip",
    }
    _, err := convertMVPNRoute(mr)
    require.Error(t, err)
    require.Contains(t, err.Error(), "originator-id")
}

// TestConvertMVPNRoute_InvalidClusterList verifies error on bad cluster-list IP.
//
// VALIDATES: Invalid cluster-list IP returns descriptive error.
// PREVENTS: Silent failure on malformed config.
func TestConvertMVPNRoute_InvalidClusterList(t *testing.T) {
    mr := MVPNRouteConfig{
        RouteType:   "source-ad",
        ClusterList: "192.168.1.1 bad-ip",
    }
    _, err := convertMVPNRoute(mr)
    require.Error(t, err)
    require.Contains(t, err.Error(), "cluster-list")
}
```

## Notes

- IPv6 silently ignored (matches VPLS pattern for consistency)
- FlowSpec/MUP config intentionally lacks these fields (only BuildFlowSpec/BuildMUP use them)

## Checklist

- [x] Test fails first
- [x] Test passes after impl
- [x] make test passes
- [x] make lint passes
- [x] Update .claude/zebgp/config/SYNTAX.md
