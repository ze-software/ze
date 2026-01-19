# Route Types Architecture

## Overview

ZeBGP has multiple route representations serving different purposes in the data flow pipeline.

## Route Structs Inventory

| Struct | Location | Purpose | Key Fields |
|--------|----------|---------|------------|
| `RouteSpec` | `internal/plugin/types.go` | API input for announce/withdraw | Prefix, NextHop, PathAttributes |
| `PathAttributes` | `internal/plugin/types.go` | Shared BGP attributes | Origin, ASPath, MED, LocalPref, Communities |
| `rib.Route` | `internal/plugin/rib/rib.go` | Plugin storage for replay/resend | All attributes as strings + MsgID |
| `rr.Route` | `internal/plugin/rr/rib.go` | Minimal for zero-copy forwarding | MsgID, Family, Prefix only |
| `RIBRoute` | `internal/plugin/types.go` | Query output | Peer, Prefix, NextHop, ASPath (strings) |
| `rib.Route` | `internal/rib/route.go` | Core engine storage | NLRI, Attrs, ASPath, wire cache, refcount |

**Note:** Two different `rib.Route` types exist - one in `internal/plugin/rib/` (plugin) and one in `internal/rib/` (core engine).

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
internal/plugin/
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

internal/selector/
└── selector.go       # Peer selectors (*, IP, !IP, ip,ip,ip)

internal/bgp/attribute/
├── text.go           # Text formatting: FormatASPath(), FormatCommunities()
└── *.go              # Wire format encoding/decoding
```

## Design Principles

### Modularity for Plugins
Route types are intentionally flexible to support diverse plugin needs:
- Zero-copy plugins need only MsgID + Family + Prefix
- Full RIB plugins need all attributes for replay
- Custom plugins can define their own storage

### Stability Guarantee
**Only text and JSON APIs are stable.** Go package structure, types, and interfaces may change without notice. Plugins should communicate via text/JSON protocol, not by importing Go packages.

See `docs/architecture/rib-transition.md` → "Stability Guarantees" for details.

## Consolidation Opportunities

### Current Issue
Multiple Route structs represent the same logical concept differently.

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

- `docs/architecture/update-building.md` - Wire format construction
- `docs/architecture/encoding-context.md` - Zero-copy patterns
- `docs/architecture/api/architecture.md` - Plugin API design
- `docs/architecture/rib-transition.md` - RIB → API transition
