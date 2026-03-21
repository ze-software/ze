# Ze Core Design

**Status:** Canonical Architecture Reference
**Date:** 2026-01-11

This document captures the fundamental design principles for Ze.
All new code MUST follow these patterns.

---

## Executive Summary

| Concept | Description |
|---------|-------------|
| **Transport Unit** | `WireUpdate` - BGP UPDATE message as bytes |
| **Storage Unit** | NLRI → Attribute references (not WireUpdate) |
| **Deduplication** | Per-attribute-type pools + per-family NLRI pools |
| **API Model** | Pipe communication with text OR raw wire bytes |
| **Route Building** | Unified parser with family-specific NLRI builders |

---

## 1. System Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    BGP Subsystem  (internal/component/bgp/)                   │
│                                                                             │
│   ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌────────────────────────────┐    │
│   │ Peer 1  │  │ Peer 2  │  │ Peer N  │  │ Capability Negotiation     │    │
│   │  FSM    │  │  FSM    │  │  FSM    │  │ (ASN4 · AddPath · ExtNH)  │    │
│   └────┬────┘  └────┬────┘  └────┬────┘  │ ContextID · EncodingContext│    │
│        │            │            │        └────────────────────────────┘    │
│        └────────────┼────────────┘                                          │
│                     ▼                                                       │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │  Wire Layer  (Session Buffer · Message Parse · WireUpdate)         │   │
│   └────────────────────────────────┬────────────────────────────────────┘   │
│                                    ▼                                        │
│   ┌─────────────────────┐  ┌──────────────────┐                            │
│   │   Reactor           │─▶│ EventDispatcher  │                            │
│   │ (event loop,        │  │ (type-safe bridge,│                            │
│   │  BGP cache)         │  │  JSON encoder)   │                            │
│   └─────────────────────┘  └────────┬─────────┘                            │
└─────────────────────────────────────┼──────────────────────────────────────┘
                                      │  formatted events
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  Config Pipeline  (internal/component/config/)                                        │
│  File → Tree → ResolveBGPTree()                                             │
│    ├─ PeersFromTree()            → peer definitions → Reactor               │
│    └─ ExtractPluginsFromTree()   → plugin config   → Plugin Infrastructure  │
└─────────────────────────────────────────────────────────────────────────────┘
┌─────────────────────────────────────────────────────────────────────────────┐
│               Plugin Infrastructure  (internal/component/plugin/)                     │
│    Plugin Registry · Process Manager · Hub · SDK · DirectBridge             │
└─────────────────────────────────────────────────────────────────────────────┘
                              │                 ▲
          JSON events (down)  │                 │  commands (up)
          + base64 wire bytes │                 │  update/forward/withdraw
                              ▼                 │
═══════════════════════ PROCESS BOUNDARY (TLS / net.Pipe) ══════════════════
                              │                 ▲
                              ▼                 │
                      ┌───────────────┐
                      │    Plugin     │  (Go/Python/Rust/etc.)
                      │  (RIB / RR)   │
                      └───────────────┘
```

**Key principles:**
- **BGP Subsystem** handles BGP protocol, TCP, FSM, wire parsing, event dispatch
- **Config Pipeline** parses config and feeds both BGP Subsystem and Plugin Infrastructure
- **Plugin Infrastructure** manages plugin lifecycle, process spawning, message routing
- **Plugins** implement RIB storage, policy, route reflection
- **Pipes** carry JSON events (with base64 wire bytes) and text commands
- **BGP cache** enables zero-copy forwarding (`bgp cache 123 forward <sel>`)
- **Dynamic event types** -- plugins declare event types they produce via `Registration.EventTypes`. Engine registers them into `ValidEvents` at startup, so subscribe-events and emit-event validation accept them. Follows the same pattern as dynamic family registration.
- **Three-phase startup** -- Phase 1: explicit plugins. Phase 2: auto-load for unclaimed families. Phase 3: auto-load for custom event types referenced in config `receive [ ]` (e.g., `update-rpki` auto-loads `bgp-rpki-decorator` and its dependencies).

---

## 2. Peer Context & Negotiated Capabilities

Decoding/encoding BGP messages requires **negotiated capabilities** from OPEN exchange:

```go
// Simplified view - see internal/component/bgp/capability/negotiated.go for full struct
type Negotiated struct {
    ASN4            bool                   // AS_PATH: 2-byte or 4-byte ASNs
    AddPath         map[Family]AddPathMode // NLRI: Receive/Send/Both path-id
    ExtendedMessage bool                   // Max message: 4096 or 65535 bytes
    ExtendedNextHop map[Family]AFI         // Per-family next-hop AFI mapping
    Families()      []Family               // Method returning negotiated families
    GracefulRestart *GracefulRestart       // RFC 4724 graceful restart state
    RouteRefresh    bool                   // RFC 2918 route refresh support
}
```

**Why it matters:**
- Same wire bytes parse differently based on negotiated caps
- `AS_PATH [00 01 FD E8]` = ASN 65000 (ASN4) or two ASNs 1, 64488 (ASN2)
- NLRI `[00 00 00 01 18 0a 00 00]` = path-id + prefix (ADD-PATH) or two prefixes (no ADD-PATH)

**ContextID:** Identifies encoding context for zero-copy forwarding decisions.
- Same ContextID = same negotiated caps = can forward wire bytes unchanged
- Different ContextID = must re-encode for target peer's capabilities

```go
// internal/component/bgp/context/registry.go
type ContextID uint16  // Unique ID per distinct capability set (65535 max)

// Zero-copy decision
if sourceCtxID == destCtxID {
    // Forward wire bytes directly
} else {
    // Parse and re-encode for destination caps
}
```

---

## 3. BGP UPDATE as Container

BGP UPDATE is an **encapsulation format**. It contains:

```
UPDATE Message (wire bytes)
├── Header (19 bytes: marker + length + type)
├── Withdrawn Routes Length (2 bytes)
├── Withdrawn Routes (IPv4 unicast only)
├── Path Attributes Length (2 bytes)
├── Path Attributes
│   ├── ORIGIN, AS_PATH, NEXT_HOP, MED, LOCAL_PREF, ...
│   ├── MP_REACH_NLRI (NLRI for non-IPv4-unicast families)
│   └── MP_UNREACH_NLRI (withdrawals for non-IPv4-unicast)
└── NLRI (IPv4 unicast announce only)
```

**Key insight:** Attributes are WITHIN the UPDATE. NLRI location depends on family:
- IPv4 unicast: NLRI in trailing section, NEXT_HOP as attribute
- All other families: NLRI inside MP_REACH_NLRI attribute

### WireUpdate Type

```go
type WireUpdate struct {
    payload     []byte           // UPDATE body (after BGP header)
    sourceCtxID bgpctx.ContextID // For zero-copy forwarding decisions
    messageID   uint64           // Unique ID for forward-by-id
    sourceID    source.SourceID  // Source that sent/created this message
}

// Lazy-parsed views into payload (zero-copy)
func (u *WireUpdate) Withdrawn() ([]byte, error)
func (u *WireUpdate) Attrs() (*AttributesWire, error)
func (u *WireUpdate) NLRI() ([]byte, error)
func (u *WireUpdate) MPReach() (MPReachWire, error)
func (u *WireUpdate) MPUnreach() (MPUnreachWire, error)

// Iterators (parse on demand)
func (u *WireUpdate) AttrIterator() (AttrIterator, error)
func (u *WireUpdate) NLRIIterator(addPath bool) (*NLRIIterator, error)
```

---

## 4. RIB Storage Model

**RIB does NOT store WireUpdate.** It stores individual routes with deduplicated attributes.
RIB storage lives in plugins (`bgp-rib`, `bgp-adj-rib-in`), not in the engine reactor.

### Why Not Store WireUpdate?

A single WireUpdate contains multiple NLRIs sharing the same attributes:
```
WireUpdate:
  Attributes: {ORIGIN=IGP, AS_PATH=[65001], LOCAL_PREF=100}
  NLRIs: [10.0.0.0/24, 10.0.1.0/24, 10.0.2.0/24]
```

In the RIB, we need:
- Individual NLRI lookup (route key)
- Attribute deduplication (many routes share same attrs)
- Per-attribute-type deduplication (many routes share same LOCAL_PREF)

### RIB Structure

```go
type RIB struct {
    // Routes: NLRI key → attribute references
    routes map[NLRIKey]*RouteEntry

    // NLRI pools - one per family (different wire formats)
    nlriPools map[nlri.Family]*Pool[nlri.NLRI]

    // Attribute pools - per-type deduplication
    originPool         *Pool[Origin]
    asPathPool         *Pool[ASPath]
    localPrefPool      *Pool[uint32]
    medPool            *Pool[uint32]
    communityPool      *Pool[Communities]
    largeCommunityPool *Pool[LargeCommunities]
    extCommunityPool   *Pool[ExtendedCommunities]
    clusterListPool    *Pool[ClusterList]
    originatorPool     *Pool[OriginatorID]

    // Next-hop: pooled but special encoding rules
    nextHopPool *Pool[NextHop]
}
```

### Route Entry (Pool Handles, Not Copies)

```go
// internal/component/plugin/rib/storage/routeentry.go
type RouteEntry struct {
    // All fields are opaque handles into attribute pools (not copies)
    // Use pool.Handle for indirection - enables refcounting and deduplication
    Origin           pool.Handle // ORIGIN (type 1)
    ASPath           pool.Handle // AS_PATH (type 2)
    NextHop          pool.Handle // NEXT_HOP (type 3)
    LocalPref        pool.Handle // LOCAL_PREF (type 5)
    MED              pool.Handle // MULTI_EXIT_DISC (type 4)
    Communities      pool.Handle // COMMUNITIES (type 8)
    LargeCommunities pool.Handle // LARGE_COMMUNITIES (type 32)
    ExtCommunities   pool.Handle // EXTENDED_COMMUNITIES (type 16)
    ClusterList      pool.Handle // CLUSTER_LIST (type 10)
    OriginatorID     pool.Handle // ORIGINATOR_ID (type 9)
    // ... other attributes
}
```

### Per-Attribute-Type Deduplication

Each attribute type has its own pool because:
- ORIGIN has only 3 possible values (IGP, EGP, INCOMPLETE)
- LOCAL_PREF typically has few unique values (100, 200, etc.)
- AS_PATH has many unique values but still shares across routes
- Communities have moderate sharing

```
Route 1: 10.0.0.0/24          Route 2: 10.0.1.0/24
  │                              │
  ├─ ORIGIN ──────────────────────┼──→ Pool: IGP (shared)
  ├─ AS_PATH ─→ [65001,65002]    │
  │                              ├─ AS_PATH ─→ [65001,65003] (different)
  ├─ LOCAL_PREF ──────────────────┼──→ Pool: 100 (shared)
  └─ COMMUNITY ───────────────────┴──→ Pool: [65000:100] (shared)
```

### NLRI Pools by Family

Different families have different NLRI wire formats:

```go
nlriPools map[nlri.Family]*Pool[nlri.NLRI]

// Contents:
//   ipv4/unicast  → Pool[*INETPrefix]
//   ipv6/unicast  → Pool[*INETPrefix]
//   ipv4/mpls     → Pool[*LabeledPrefix]
//   ipv4/mpls-vpn → Pool[*VPNPrefix]
//   ipv4/flowspec → Pool[*FlowSpecRule]
//   l2vpn/evpn    → Pool[*EVPNRoute]
//   ...
```

All NLRI types implement the NLRI interface:

```go
// Base interface - caller guarantees buffer capacity
type BufWriter interface {
    WriteTo(buf []byte, off int) int
}

// Checked interface - validates capacity before writing
type CheckedBufWriter interface {
    BufWriter
    CheckedWriteTo(buf []byte, off int) (int, error)
    Len() int
}

// NLRI interface (internal/component/bgp/nlri/nlri.go)
type NLRI interface {
    Family() Family
    Bytes() []byte                    // Wire-format encoding (payload only)
    Len() int                         // Payload length (no path ID)
    String() string                   // Human-readable representation
    PathID() uint32                   // ADD-PATH path identifier (0 if not present)
    WriteTo(buf []byte, off int) int  // Write payload (no path ID)
    SupportsAddPath() bool            // Whether this NLRI type supports ADD-PATH
}

// LenWithContext is a standalone function for ADD-PATH aware length:
func LenWithContext(n NLRI, addPath bool) int
// Returns Len() if addPath=false, Len()+4 if addPath=true
```

**ADD-PATH encoding:** Use `WriteNLRI()` helper function for ADD-PATH aware encoding,
which prepends the 4-byte path ID when needed.

---

## 5. Next-Hop Special Handling

Next-hop encoding varies by family:

| Family | Next-Hop Location |
|--------|-------------------|
| IPv4 unicast | NEXT_HOP attribute (type 3) |
| IPv6 unicast | Inside MP_REACH_NLRI |
| VPNv4/VPNv6 | Inside MP_REACH_NLRI |
| FlowSpec | Inside MP_REACH_NLRI |
| EVPN | Inside MP_REACH_NLRI |

The NextHop type must handle this context-dependent encoding.

---

## 6. API Pipe Communication

Ze engine communicates with plugins via stdin/stdout pipes.

### Two Input Modes

**Mode A: Text (human readable, attributes parsed)**
```
"update text origin set igp as-path set [65001] community set [65000:100]
        nhop set 1.1.1.1 nlri ipv4/unicast add 10.0.0.0/24"
                    │
                    ▼
             Parser → family NLRI builder → builds wire
                    │
                    ▼
             WireUpdate{payload: [wire bytes]}
```

**Mode B: Binary (raw wire bytes, hex/base64)**
```
"update hex attr set 400101... nlri ipv4/unicast add 180a00"
                    │
                    ▼
             Direct decode (no parsing)
                    │
                    ▼
             WireUpdate{payload: [wire bytes]}
```

Both modes produce the same result: `WireUpdate` with wire bytes.

See `docs/architecture/api/update-syntax.md` for full syntax specification.

### JSON Events (Engine → Plugin)

Engine sends events with base64-encoded wire bytes:

```json
{
  "message": {"type": "update", "id": 12345, "direction": "received"},
  "peer": {"address": "10.0.0.1", "context-id": 42},
  "raw-attributes": "QAEBAQA=",
  "raw-nlri": "GApAAA==",
  "parsed": { ... }
}
```

**`context-id`**: Plugin uses this for zero-copy forwarding decisions. If source and dest peers have same context-id, forward wire bytes unchanged.

Plugin can:
- Use `parsed` for decisions
- Store `raw-*` bytes directly (for forwarding)
- Forward by ID: `"bgp cache 12345 forward !10.0.0.1"` (or batch: `"bgp cache 1,2,3 forward !10.0.0.1"`)

### What Engine Stores vs Plugin Stores

| Component | Engine Stores | Plugin Stores |
|-----------|---------------|---------------|
| **BGP cache** | WireUpdate by ID (for `bgp cache <id>[,<id>...] forward`) | - |
| **Peer state** | Negotiated caps, FSM state | - |
| **RIB** | - | NLRI → attribute refs (with pools) |
| **Policy** | - | Route filters, preferences |

Engine is stateless for routes. It forwards wire bytes to plugins and caches for zero-copy forwarding.

---

## 7. Route Building

### Unified Parser with Family Dispatch

One parser handles all families. Family is determined by `nlri <family>` keyword:

```go
// Single entry point
func ParseUpdate(cmd string, ctx *PackContext) (*WireUpdate, error) {
    // 1. Tokenize command
    // 2. Parse attributes (origin, as-path, community, nhop, etc.)
    // 3. On "nlri <family>", dispatch to family-specific NLRI builder
    // 4. Build wire bytes
    // 5. Return WireUpdate
}
```

### Family-Specific NLRI Builders

Each family has different NLRI wire format:

```go
// NLRI builders - called by parser when it sees "nlri <family>"
func buildIPv4UnicastNLRI(prefixes []string, ctx *PackContext) ([]byte, error)
func buildFlowSpecNLRI(rules []FlowSpecRule, ctx *PackContext) ([]byte, error)
func buildL3VPNNLRI(rd string, labels []uint32, prefix string, ctx *PackContext) ([]byte, error)
// etc.
```

### Intermediate Structs (Parsing Only)

Family-specific structs exist for complex NLRI types during parsing:

```go
// Used during parsing only - NOT stored
type FlowSpecRule struct {
    DestPrefix   *netip.Prefix
    SourcePrefix *netip.Prefix
    Protocols    []uint8
    Ports        []uint16
    Actions      FlowSpecActions
}

// Parsed → built to wire → struct discarded
```

**Key point:** These structs are temporary. Only wire bytes are stored/transmitted.

---

## 8. Attribute Handling

`Builder` and `AttributesWire` are intentionally separate types with distinct roles:
- **`AttributesWire`** — reads/iterates received wire bytes (zero-copy, lazy parsing)
- **`Builder`** — constructs new attribute wire bytes for outgoing UPDATEs

A merged type was considered but rejected: the read path (iterator-based, context-dependent
parsing) and write path (field-at-a-time construction) have fundamentally different lifecycles
and usage patterns. Keeping them separate avoids state confusion and keeps each type focused.

### Builder/Wire Interface (reference)

```go
type Attributes struct {
    // Wire bytes (source of truth)
    wire      []byte
    sourceCtx bgpctx.ContextID

    // Build state (for constructing new attributes)
    building  bool
    origin    *uint8
    asPath    []uint32
    // ... other fields
}

// Reading (from received wire)
func (a *Attributes) Get(code AttributeCode) (Attribute, error)
func (a *Attributes) Iterator() AttrIterator
func (a *Attributes) Packed() []byte

// Building (to wire)
func (a *Attributes) SetOrigin(o uint8) *Attributes
func (a *Attributes) SetASPath(asns []uint32) *Attributes
func (a *Attributes) AddCommunity(c uint32) *Attributes
func (a *Attributes) Build() []byte
func (a *Attributes) WriteTo(buf []byte, off int) int           // pre-allocated buffer
func (a *Attributes) CheckedWriteTo(buf []byte, off int) (int, error)
```

---

## 9. Data Flow Summary

### Receive Path

```
Network recv() → WireUpdate → Reactor → EventDispatcher → Plugin (JSON + base64)
                                                                │
                                                                ├─ Parse (lazy)
                                                                ├─ Extract NLRIs (iterator)
                                                                ├─ Extract attributes (iterator)
                                                                ├─ Intern each in pools
                                                                └─ Create RouteEntry with refs
```

### API Announce Path

```
Text command → ParseUpdate() → WireUpdate → Send to peer
                    │
                    ├─ Parse text → intermediate struct
                    ├─ Build wire bytes
                    └─ Create WireUpdate (struct discarded)
```

### Forwarding Path

```
Receive UPDATE → Assign msg-id → Cache WireUpdate → API event
                                                        │
                                                        ▼
                                               Plugin decides
                                                        │
                                                        ▼
                          "bgp cache 123 forward" → Lookup cache → Send wire
```

---

## 10. What Gets Eliminated

> **Note:** These are planned refactorings.

| Current Type | Status | Action |
|--------------|--------|--------|
| `message.Update` | Keep | Share parsing with WireUpdate via `wire.UpdateSections` (see `plan/spec-update-shared-parsing.md`) |
| `rib.Route` with parsed attrs | Refactor | `RouteEntry` with pool refs |
| `plugin/rib.Route` (strings) | Remove | Use core RIB |
| `plugin/rr.Route` | Remove | Use core RIB |
| `RouteSpec`, `FlowSpecRoute`, etc. | Keep | Parsing intermediates (not stored) |
| `attribute.AttributesWire` | Merge | `Attributes` (read + write) |
| `attribute.Builder` | Merge | `Attributes` (read + write) |

---

## 11. Implementation Priority

1. **Merge Attributes types** - Single type for read + write
2. **Implement RIB with pools** - Per-attribute-type deduplication
3. **Unified parser** - Family-specific NLRI builders
4. **Remove duplicates** - plugin/rib, plugin/rr; share UPDATE parsing between message.Update and WireUpdate

---

## Related Documents

- `buffer-architecture.md` - Iterators and lazy parsing
- `pool-architecture.md` - Deduplication pool design
- `update-building.md` - Wire format construction
- `api/architecture.md` - Pipe communication protocol

---

**Last Updated:** 2026-01-30 (RouteEntry updated to match pool.Handle implementation)
