# Route Types Architecture

## Overview

Ze has multiple route representations serving different purposes in the data flow pipeline.

## Route Structs Inventory

| Struct | Location | Purpose | Key Fields |
|--------|----------|---------|------------|
| `RouteSpec` | `internal/component/bgp/types/types.go` | API input for announce/withdraw | Prefix, NextHop, Wire (AttributesWire) |
| `FlowSpecRoute` | `internal/component/bgp/types/types.go` | FlowSpec route announcement | Family, DestPrefix, SourcePrefix, Actions |
| `L3VPNRoute` | `internal/component/bgp/types/types.go` | MPLS VPN route (RFC 4364) | Prefix, NextHop, RD, Labels, Wire |
| `LabeledUnicastRoute` | `internal/component/bgp/types/types.go` | MPLS labeled unicast (RFC 8277) | Prefix, NextHop, Labels, PathID, Wire |
| `L2VPNRoute` | `internal/component/bgp/types/types.go` | L2VPN/EVPN route | RouteType, RD, MAC, IP, Labels |
| `MUPRouteSpec` | `internal/component/bgp/types/types.go` | Mobile User Plane (SAFI 85) | RouteType, Prefix, RD, TEID, Wire |
| `VPLSRoute` | `internal/component/bgp/types/types.go` | VPLS route | RD, VEBlockOffset, LabelBase |
| `rib.Route` | `internal/component/bgp/rib/route.go` | Core engine storage | NLRI, Attrs, ASPath, wire cache, refcount |
<!-- source: internal/component/bgp/types/types.go -- RouteSpec, FlowSpecRoute, L3VPNRoute, L2VPNRoute, MUPRouteSpec -->
<!-- source: internal/component/bgp/rib/route.go -- rib.Route (core engine) -->

**Note:** `rib.Route` in `internal/component/bgp/rib/` is the core engine storage type.

## Route Type Families

### Unicast Routes
- `RouteSpec` - IPv4/IPv6 unicast with Wire (AttributesWire)

### VPN Routes
- `L3VPNRoute` - MPLS VPN (RFC 4364) - has Wire (AttributesWire)
- `LabeledUnicastRoute` - MPLS labeled unicast - has Wire (AttributesWire)

### EVPN Routes
- `L2VPNRoute` - EVPN types (RFC 7432) - separate structure

### Special Routes
- `FlowSpecRoute` - FlowSpec (RFC 8955) - separate structure
- `MUPRouteSpec` - Mobile User Plane - has Wire (AttributesWire)
- `VPLSRoute` - VPLS - separate structure

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
```

## Wire Attributes Pattern

Routes that carry `*attribute.AttributesWire` for path attributes in wire format:
- `RouteSpec`
- `L3VPNRoute`
- `LabeledUnicastRoute`
- `MUPRouteSpec`

Routes with separate structure (no Wire field):
- `FlowSpecRoute` (actions instead of attributes)
- `L2VPNRoute` (EVPN-specific fields)
- `VPLSRoute` (VPLS-specific fields)

## Package Organization

```
internal/component/bgp/types/
└── types.go          # RouteSpec, FlowSpecRoute, L3VPNRoute, L2VPNRoute, etc.

internal/component/bgp/rib/
└── route.go          # rib.Route (core engine storage)

internal/core/selector/
└── selector.go       # Peer selectors (*, IP, !IP, ip,ip,ip)

internal/component/bgp/attribute/
├── wire.go           # AttributesWire (read/iterate received wire bytes)
├── builder.go        # Builder (construct new attribute wire bytes)
├── text.go           # Text formatting: FormatASPath(), FormatCommunities()
└── *.go              # Wire format encoding/decoding
```
<!-- source: internal/component/bgp/types/types.go -- route type definitions -->
<!-- source: internal/core/selector/selector.go -- peer selector implementation -->

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
