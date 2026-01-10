# Route Types Architecture

## Overview

ZeBGP has multiple route representations serving different purposes in the data flow pipeline.

## Route Structs Inventory

| Struct | Location | Purpose | Key Fields |
|--------|----------|---------|------------|
| `RouteSpec` | `pkg/plugin/types.go` | API input for announce/withdraw | Prefix, NextHop, PathAttributes |
| `PathAttributes` | `pkg/plugin/types.go` | Shared BGP attributes | Origin, ASPath, MED, LocalPref, Communities |
| `rib.Route` | `pkg/plugin/rib/rib.go` | Full storage for replay/resend | All attributes + MsgID |
| `rr.Route` | `pkg/plugin/rr/rib.go` | Minimal for zero-copy forwarding | MsgID, Family, Prefix only |
| `RIBRoute` | `pkg/plugin/types.go` | Query output | Peer, Prefix, NextHop, ASPath |

## Route Type Families

### Unicast Routes
- `RouteSpec` - IPv4/IPv6 unicast with PathAttributes

### VPN Routes
- `L3VPNRoute` - MPLS VPN (RFC 4364) - embeds PathAttributes
- `LabeledUnicastRoute` - MPLS labeled unicast - embeds PathAttributes

### EVPN Routes
- `L2VPNRoute` - EVPN types (RFC 7432) - separate structure

### Special Routes
- `FlowSpecRoute` - FlowSpec (RFC 8955) - separate structure
- `MUPRouteSpec` - Mobile User Plane - embeds PathAttributes

## Data Flow

```
Command Input                    Wire Reception
     │                                │
     ▼                                ▼
┌─────────────┐               ┌─────────────┐
│ RouteSpec   │               │ RawMessage  │
│ (API input) │               │ (lazy parse)│
└─────────────┘               └─────────────┘
     │                                │
     ▼                                ▼
┌─────────────────────────────────────────────┐
│              Reactor                         │
│  AnnounceRoute() / MessageReceived()        │
└─────────────────────────────────────────────┘
     │                                │
     ▼                                ▼
┌─────────────┐               ┌─────────────┐
│ rib.Route   │               │ rib.Route   │
│ (full store)│               │ (full store)│
└─────────────┘               └─────────────┘
     │
     ▼
┌─────────────┐
│ RIBRoute    │
│ (query out) │
└─────────────┘
```

## PathAttributes Embedding Pattern

Routes that embed `PathAttributes`:
- `RouteSpec`
- `L3VPNRoute`
- `LabeledUnicastRoute`
- `MUPRouteSpec`

Routes with separate structure:
- `FlowSpecRoute` (actions instead of attributes)
- `L2VPNRoute` (EVPN-specific fields)

## Package Organization

```
pkg/plugin/
├── types.go          # RouteSpec, PathAttributes, RIBRoute, route families
├── nexthop.go        # RouteNextHop (policy: explicit/self)
├── route.go          # Parsing: ParseRouteAttributes(), parseCommonAttribute()
├── json.go           # JSON encoding: RouteAnnounce(), RouteWithdraw()
│
├── rib/
│   └── rib.go        # rib.Route (full storage)
│
└── rr/
    └── rib.go        # rr.Route (minimal/zero-copy)

pkg/selector/
└── selector.go       # Peer selectors (*, IP, !IP, ip,ip,ip)

pkg/bgp/attribute/
├── text.go           # Text formatting: FormatASPath(), FormatCommunities()
└── *.go              # Wire format encoding/decoding
```

## Consolidation Opportunities

### Current Issue
4 Route structs represent the same logical concept differently.

### Recommended Approach
Single `rib.Route` as source of truth with view methods:

```go
type Route struct {
    MsgID              uint64
    Family             string
    Prefix             string
    PathID             uint32
    NextHop            string
    Origin             string
    ASPath             []uint32
    MED                uint32
    LocalPref          uint32
    Communities        []uint32
    LargeCommunities   []LargeCommunity
    ExtendedCommunities []ExtendedCommunity
}

func (r *Route) ForZeroCopy() ZeroCopyRoute { ... }
func (r *Route) ForAPI() RIBRoute { ... }
func (r *Route) MarshalJSON() ([]byte, error) { ... }
```

## Related Documentation

- `docs/architecture/UPDATE_BUILDING.md` - Wire format construction
- `docs/architecture/ENCODING_CONTEXT.md` - Zero-copy patterns
- `docs/architecture/api/ARCHITECTURE.md` - Plugin API design
