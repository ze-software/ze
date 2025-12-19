# ZeBGP: ExaBGP Rewrite in Go

**Status:** 📋 Planning
**Created:** 2025-12-19
**Target:** ExaBGP-compatible BGP implementation in Go with novel architecture

---

## Executive Summary

This document outlines the implementation plan to rewrite ExaBGP in Go ("ZeBGP"). The goal is to create a fully compatible implementation while introducing architectural innovations for improved performance and memory efficiency.

### Key Innovations

1. **AS-PATH as NLRI extension** - Treat AS-PATH as part of NLRI (like ADD-PATH path-id), not as an attribute, enabling better attribute deduplication and caching
2. **Per-attribute goroutine stores** - Centralized deduplication with one goroutine per known attribute type
3. **Zero-copy message passing** - Pass unchanged messages between peers without repacking
4. **Go-routine per peer** - Replace Python's async reactor with native Go concurrency
5. **Struct embedding for wire format** - Direct RFC structure mapping for efficient parsing

### Design Principles

- **ExaBGP API compatibility** - Same JSON output, same API commands
- **Configuration compatibility** - Same config file format, same environment variables
- **Test compatibility** - Convert and use ExaBGP's test suite
- **No FIB manipulation** - BGP protocol only (like ExaBGP)
- **Future-ready** - Design for potential FIB support later

---

## Phase 1: Project Foundation (Week 1-2)

### 1.1 Repository Setup

```
zebgp/
├── cmd/
│   ├── zebgp/           # Main daemon entry point
│   ├── zebgp-cli/       # CLI client
│   └── zebgp-decode/    # Message decoder tool
├── pkg/
│   ├── bgp/             # BGP protocol implementation
│   │   ├── message/     # Message types (OPEN, UPDATE, etc.)
│   │   ├── attribute/   # Path attributes
│   │   ├── nlri/        # NLRI types
│   │   ├── capability/  # BGP capabilities
│   │   └── fsm/         # Finite state machine
│   ├── reactor/         # Event loop and peer management
│   ├── rib/             # Routing Information Base
│   ├── config/          # Configuration parsing
│   ├── api/             # External API (Unix socket, JSON)
│   └── wire/            # Wire format utilities
├── internal/
│   ├── store/           # Attribute/NLRI deduplication stores
│   └── pool/            # Buffer pools
├── testdata/            # Test fixtures from ExaBGP
└── doc/                 # Documentation
```

### 1.2 Dependencies Selection

| Purpose | Library | Rationale |
|---------|---------|-----------|
| CLI framework | `cobra` + `viper` | Industry standard, subcommand support |
| Logging | `slog` (stdlib) | Go 1.21+ structured logging |
| Configuration | `viper` | File + env + defaults |
| Testing | `testify` | Assertions and mocking |
| JSON | `sonic` or stdlib | High-performance JSON |
| Unix socket | stdlib `net` | No external deps needed |

### 1.3 Core Type Definitions

```go
// pkg/bgp/types.go

// AFI represents Address Family Identifier (RFC 4760)
type AFI uint16

const (
    AFIIPv4  AFI = 1
    AFIIPv6  AFI = 2
    AFIL2VPN AFI = 25
    AFIBGPLS AFI = 16388
)

// SAFI represents Subsequent Address Family Identifier
type SAFI uint8

const (
    SAFIUnicast   SAFI = 1
    SAFIMulticast SAFI = 2
    SAFIMPLSLabel SAFI = 4
    SAFIVPN       SAFI = 128
    SAFIFlowSpec  SAFI = 133
    SAFIEVPN      SAFI = 70
    // ... all 42 combinations from ExaBGP
)

// Family combines AFI and SAFI
type Family struct {
    AFI  AFI
    SAFI SAFI
}
```

### 1.4 Build System

```makefile
# Makefile
.PHONY: build test lint

build:
	go build -o bin/zebgp ./cmd/zebgp
	go build -o bin/zebgp-cli ./cmd/zebgp-cli
	go build -o bin/zebgp-decode ./cmd/zebgp-decode

test:
	go test -race -v ./...

test-encoding:
	./scripts/run-encoding-tests.sh

lint:
	golangci-lint run
```

**Deliverables:**
- [ ] Go module initialized
- [ ] Directory structure created
- [ ] Dependencies added to go.mod
- [ ] Basic build system working
- [ ] CI pipeline configured

---

## Phase 2: Wire Format Foundation (Week 2-3)

### 2.1 Buffer Protocol

Implement zero-copy parsing using Go's slice semantics:

```go
// pkg/wire/buffer.go

// Buffer wraps a byte slice for zero-copy parsing
type Buffer struct {
    data   []byte
    offset int
}

func NewBuffer(data []byte) *Buffer {
    return &Buffer{data: data, offset: 0}
}

func (b *Buffer) ReadByte() (byte, error) {
    if b.offset >= len(b.data) {
        return 0, io.EOF
    }
    v := b.data[b.offset]
    b.offset++
    return v, nil
}

func (b *Buffer) ReadUint16() (uint16, error) {
    if b.offset+2 > len(b.data) {
        return 0, io.EOF
    }
    v := binary.BigEndian.Uint16(b.data[b.offset:])
    b.offset += 2
    return v, nil
}

func (b *Buffer) ReadBytes(n int) ([]byte, error) {
    if b.offset+n > len(b.data) {
        return nil, io.EOF
    }
    // Return slice of original buffer (zero-copy)
    v := b.data[b.offset : b.offset+n]
    b.offset += n
    return v, nil
}

func (b *Buffer) Remaining() []byte {
    return b.data[b.offset:]
}
```

### 2.2 Message Header Structure

```go
// pkg/bgp/message/header.go

const (
    MarkerLen  = 16
    HeaderLen  = 19
    MaxMsgLen  = 4096
    ExtMsgLen  = 65535  // RFC 8654
)

var Marker = [16]byte{
    0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
    0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
}

type MessageType uint8

const (
    TypeOPEN         MessageType = 1
    TypeUPDATE       MessageType = 2
    TypeNOTIFICATION MessageType = 3
    TypeKEEPALIVE    MessageType = 4
    TypeROUTEREFRESH MessageType = 5
)

// Header represents BGP message header (RFC 4271)
// Struct embedding matches wire format exactly
type Header struct {
    Marker [16]byte
    Length uint16
    Type   MessageType
}

func ParseHeader(data []byte) (*Header, error) {
    if len(data) < HeaderLen {
        return nil, ErrShortRead
    }
    h := &Header{
        Length: binary.BigEndian.Uint16(data[16:18]),
        Type:   MessageType(data[18]),
    }
    copy(h.Marker[:], data[:16])

    // Validate marker
    if h.Marker != Marker {
        return nil, ErrInvalidMarker
    }
    return h, nil
}
```

### 2.3 Struct Embedding Pattern

Define structs that directly map to RFC wire formats:

```go
// pkg/bgp/message/update/nlri/evpn.go

// EVPNType2Header - MAC/IP Advertisement Route (RFC 7432)
// Wire format struct embedding for zero-copy parsing
type EVPNType2Header struct {
    RD              [8]byte  // Route Distinguisher
    ESI             [10]byte // Ethernet Segment Identifier
    EthernetTagID   uint32   // Ethernet Tag ID
    MACAddrLen      uint8    // Always 48 (bits)
    MACAddr         [6]byte  // MAC Address
    IPAddrLen       uint8    // 0, 32, or 128 (bits)
    // IPAddr follows (variable: 0, 4, or 16 bytes)
    // Labels follow (variable)
}

func (h *EVPNType2Header) Parse(data []byte) ([]byte, error) {
    if len(data) < 33 { // minimum size
        return nil, ErrShortRead
    }
    copy(h.RD[:], data[0:8])
    copy(h.ESI[:], data[8:18])
    h.EthernetTagID = binary.BigEndian.Uint32(data[18:22])
    h.MACAddrLen = data[22]
    copy(h.MACAddr[:], data[23:29])
    h.IPAddrLen = data[29]

    // Return remaining bytes for variable-length fields
    return data[30:], nil
}
```

**Deliverables:**
- [ ] Buffer type with zero-copy reading
- [ ] Message header parsing
- [ ] Struct embedding pattern established
- [ ] Unit tests for wire parsing

---

## Phase 3: Message Types (Week 3-4)

### 3.1 Message Interface

```go
// pkg/bgp/message/message.go

// Message is the interface all BGP messages implement
type Message interface {
    Type() MessageType
    Pack(negotiated *Negotiated) ([]byte, error)
}

// MessageUnpacker creates messages from wire format
type MessageUnpacker interface {
    Unpack(data []byte, negotiated *Negotiated) (Message, error)
}

// Registry for message types
var messageRegistry = map[MessageType]MessageUnpacker{}

func RegisterMessage(t MessageType, u MessageUnpacker) {
    messageRegistry[t] = u
}

func UnpackMessage(msgType MessageType, data []byte, neg *Negotiated) (Message, error) {
    if u, ok := messageRegistry[msgType]; ok {
        return u.Unpack(data, neg)
    }
    return nil, fmt.Errorf("unknown message type: %d", msgType)
}
```

### 3.2 OPEN Message

```go
// pkg/bgp/message/open.go

type Open struct {
    Version       uint8
    MyAS          uint16     // 2-byte AS (use AS_TRANS if 4-byte)
    HoldTime      uint16
    BGPIdentifier uint32     // Router ID
    Capabilities  []Capability

    // Parsed from capabilities
    ASN4          uint32     // 4-byte AS if supported
}

func (o *Open) Pack(neg *Negotiated) ([]byte, error) {
    // ... packing logic
}

type OpenUnpacker struct{}

func (OpenUnpacker) Unpack(data []byte, neg *Negotiated) (Message, error) {
    if len(data) < 10 {
        return nil, ErrShortRead
    }
    o := &Open{
        Version:       data[0],
        MyAS:          binary.BigEndian.Uint16(data[1:3]),
        HoldTime:      binary.BigEndian.Uint16(data[3:5]),
        BGPIdentifier: binary.BigEndian.Uint32(data[5:9]),
    }

    optParamLen := data[9]
    if len(data) < 10+int(optParamLen) {
        return nil, ErrShortRead
    }

    // Parse optional parameters (capabilities)
    o.Capabilities = parseCapabilities(data[10 : 10+optParamLen])

    return o, nil
}
```

### 3.3 UPDATE Message

```go
// pkg/bgp/message/update.go

// Update represents a BGP UPDATE message
// Note: We separate wire representation from semantic representation
type Update struct {
    // Raw wire data for pass-through optimization
    rawData []byte

    // Parsed semantic content (lazy parsed)
    parsed     bool
    withdrawn  []NLRI
    attributes *Attributes  // Points to deduplicated store
    announced  []NLRI
}

// ForPassthrough returns the raw message for unchanged forwarding
func (u *Update) ForPassthrough() []byte {
    return u.rawData
}

// Parse performs lazy parsing of the UPDATE message
func (u *Update) Parse(neg *Negotiated) error {
    if u.parsed {
        return nil
    }
    // ... parsing logic
    u.parsed = true
    return nil
}
```

### 3.4 NOTIFICATION Message

```go
// pkg/bgp/message/notification.go

type Notification struct {
    ErrorCode    uint8
    ErrorSubcode uint8
    Data         []byte
}

// Standard error codes (RFC 4271)
const (
    ErrMessageHeader uint8 = 1
    ErrOpenMessage   uint8 = 2
    ErrUpdateMessage uint8 = 3
    ErrHoldTimer     uint8 = 4
    ErrFSMError      uint8 = 5
    ErrCease         uint8 = 6
)
```

### 3.5 KEEPALIVE Message

```go
// pkg/bgp/message/keepalive.go

type Keepalive struct{}

// Singleton - KEEPALIVE has no body
var keepaliveSingleton = &Keepalive{}

func (Keepalive) Type() MessageType { return TypeKEEPALIVE }

func (Keepalive) Pack(neg *Negotiated) ([]byte, error) {
    // Just header, no body
    return packHeader(TypeKEEPALIVE, nil), nil
}
```

**Deliverables:**
- [ ] Message interface defined
- [ ] OPEN message with capability parsing
- [ ] UPDATE message with lazy parsing
- [ ] NOTIFICATION message
- [ ] KEEPALIVE message
- [ ] ROUTE_REFRESH message
- [ ] Message registry working
- [ ] Unit tests for all message types

---

## Phase 4: Capabilities (Week 4)

### 4.1 Capability Interface

```go
// pkg/bgp/capability/capability.go

type CapabilityCode uint8

const (
    CapMultiprotocol    CapabilityCode = 1
    CapRouteRefresh     CapabilityCode = 2
    CapExtendedNextHop  CapabilityCode = 5
    CapExtendedMessage  CapabilityCode = 6
    CapGracefulRestart  CapabilityCode = 64
    CapASN4             CapabilityCode = 65
    CapAddPath          CapabilityCode = 69
    CapFQDN             CapabilityCode = 73
    CapSoftwareVersion  CapabilityCode = 75
)

type Capability interface {
    Code() CapabilityCode
    Pack() []byte
}

var capabilityRegistry = map[CapabilityCode]func([]byte) (Capability, error){}
```

### 4.2 Key Capabilities

```go
// Multiprotocol Extensions (RFC 4760)
type MultiprotocolCap struct {
    AFI  AFI
    SAFI SAFI
}

// 4-Byte AS Numbers (RFC 6793)
type ASN4Cap struct {
    ASN uint32
}

// ADD-PATH (RFC 7911)
type AddPathCap struct {
    Families []AddPathFamily
}

type AddPathFamily struct {
    AFI   AFI
    SAFI  SAFI
    Flags uint8 // 1=receive, 2=send, 3=both
}

// Extended Message (RFC 8654)
type ExtendedMessageCap struct{}
```

### 4.3 Negotiated State

```go
// pkg/bgp/capability/negotiated.go

// Negotiated holds the result of capability negotiation
type Negotiated struct {
    Families        map[Family]bool
    ASN4            bool
    LocalASN        uint32
    PeerASN         uint32
    AddPath         map[Family]AddPathMode
    ExtendedMessage bool
    HoldTime        uint16

    // Cached for fast lookup
    familySlice []Family
}

type AddPathMode uint8

const (
    AddPathNone    AddPathMode = 0
    AddPathReceive AddPathMode = 1
    AddPathSend    AddPathMode = 2
    AddPathBoth    AddPathMode = 3
)
```

**Deliverables:**
- [ ] Capability interface and registry
- [ ] All 14+ capability types implemented
- [ ] Negotiated state structure
- [ ] Capability negotiation logic
- [ ] Unit tests

---

## Phase 5: Path Attributes (Week 5-6)

### 5.1 Attribute Interface

```go
// pkg/bgp/attribute/attribute.go

type AttributeCode uint8

const (
    AttrOrigin          AttributeCode = 1
    AttrASPath          AttributeCode = 2
    AttrNextHop         AttributeCode = 3
    AttrMED             AttributeCode = 4
    AttrLocalPref       AttributeCode = 5
    AttrAtomicAggregate AttributeCode = 6
    AttrAggregator      AttributeCode = 7
    AttrCommunity       AttributeCode = 8
    AttrOriginatorID    AttributeCode = 9
    AttrClusterList     AttributeCode = 10
    AttrMPReachNLRI     AttributeCode = 14
    AttrMPUnreachNLRI   AttributeCode = 15
    AttrExtCommunity    AttributeCode = 16
    AttrAS4Path         AttributeCode = 17
    AttrAS4Aggregator   AttributeCode = 18
    AttrPMSI            AttributeCode = 22
    AttrAIGP            AttributeCode = 26
    AttrBGPLS           AttributeCode = 29
    AttrLargeCommunity  AttributeCode = 32
    AttrPrefixSID       AttributeCode = 40
)

type AttributeFlags uint8

const (
    FlagOptional   AttributeFlags = 0x80
    FlagTransitive AttributeFlags = 0x40
    FlagPartial    AttributeFlags = 0x20
    FlagExtLength  AttributeFlags = 0x10
)

type Attribute interface {
    Code() AttributeCode
    Flags() AttributeFlags
    Pack(neg *Negotiated) []byte

    // For deduplication
    Hash() uint64
    Equal(other Attribute) bool
}
```

### 5.2 Core Attributes

```go
// pkg/bgp/attribute/origin.go

type Origin uint8

const (
    OriginIGP        Origin = 0
    OriginEGP        Origin = 1
    OriginIncomplete Origin = 2
)

// Singletons for common origins
var (
    originIGP        = Origin(OriginIGP)
    originEGP        = Origin(OriginEGP)
    originIncomplete = Origin(OriginIncomplete)
)
```

```go
// pkg/bgp/attribute/aspath.go

// ASPath is now treated as part of NLRI for better deduplication
// This is a key innovation: AS-PATH extends NLRI like ADD-PATH does
type ASPath struct {
    Segments []ASPathSegment
}

type ASPathSegment struct {
    Type ASPathSegmentType
    ASNs []uint32
}

type ASPathSegmentType uint8

const (
    ASPathSet      ASPathSegmentType = 1
    ASPathSequence ASPathSegmentType = 2
)

func (a *ASPath) Hash() uint64 {
    // Fast hash for deduplication
    h := fnv.New64a()
    for _, seg := range a.Segments {
        binary.Write(h, binary.BigEndian, seg.Type)
        for _, asn := range seg.ASNs {
            binary.Write(h, binary.BigEndian, asn)
        }
    }
    return h.Sum64()
}
```

### 5.3 Attribute Deduplication Store

```go
// internal/store/attribute_store.go

// AttributeStore provides centralized attribute deduplication
// Each known attribute type has its own goroutine for concurrent access
type AttributeStore struct {
    stores map[AttributeCode]*typedStore
}

type typedStore struct {
    mu      sync.RWMutex
    items   map[uint64]Attribute
    lru     *list.List
    maxSize int
}

// Intern returns a deduplicated attribute (or stores new one)
func (s *AttributeStore) Intern(attr Attribute) Attribute {
    store := s.stores[attr.Code()]
    if store == nil {
        // Use generic store for unknown attributes
        store = s.stores[0]
    }
    return store.intern(attr)
}

func (ts *typedStore) intern(attr Attribute) Attribute {
    hash := attr.Hash()

    ts.mu.RLock()
    if existing, ok := ts.items[hash]; ok && existing.Equal(attr) {
        ts.mu.RUnlock()
        return existing
    }
    ts.mu.RUnlock()

    ts.mu.Lock()
    defer ts.mu.Unlock()

    // Double-check after acquiring write lock
    if existing, ok := ts.items[hash]; ok && existing.Equal(attr) {
        return existing
    }

    ts.items[hash] = attr
    // LRU eviction if needed
    if len(ts.items) > ts.maxSize {
        ts.evictOldest()
    }

    return attr
}
```

### 5.4 Communities

```go
// pkg/bgp/attribute/community/community.go

// Standard Community (RFC 1997)
type Community uint32

// Well-known communities
const (
    CommunityNoExport         Community = 0xFFFFFF01
    CommunityNoAdvertise      Community = 0xFFFFFF02
    CommunityNoExportSubconfed Community = 0xFFFFFF03
    CommunityNoPeer           Community = 0xFFFFFF04
)

// Extended Community (RFC 4360)
type ExtendedCommunity [8]byte

// Large Community (RFC 8092)
type LargeCommunity struct {
    GlobalAdmin uint32
    LocalData1  uint32
    LocalData2  uint32
}
```

**Deliverables:**
- [ ] Attribute interface and registry
- [ ] All 23+ attribute types implemented
- [ ] Attribute deduplication store
- [ ] Community types (standard, extended, large)
- [ ] Segment routing attributes
- [ ] BGP-LS attributes
- [ ] Unit tests

---

## Phase 6: NLRI Types (Week 6-8)

### 6.1 NLRI Interface with AS-PATH Extension

```go
// pkg/bgp/nlri/nlri.go

// NLRI represents Network Layer Reachability Information
// Innovation: AS-PATH is part of NLRI for better attribute sharing
type NLRI interface {
    Family() Family
    Pack(neg *Negotiated) []byte

    // Index returns a unique identifier for this NLRI
    // INCLUDES AS-PATH hash for deduplication (novel approach)
    Index() []byte

    // JSON serialization
    JSON(compact bool) string
}

// NLRIWithPath combines NLRI with AS-PATH for the novel indexing approach
type NLRIWithPath struct {
    NLRI   NLRI
    ASPath *ASPath // Part of index, not attributes
}

func (np *NLRIWithPath) Index() []byte {
    // Index = Family + NLRI wire format + AS-PATH hash
    // This allows sharing all other attributes when AS-PATH differs
    buf := make([]byte, 0, 64)
    buf = append(buf, byte(np.NLRI.Family().AFI>>8), byte(np.NLRI.Family().AFI))
    buf = append(buf, byte(np.NLRI.Family().SAFI))
    buf = append(buf, np.NLRI.Pack(nil)...)
    if np.ASPath != nil {
        h := np.ASPath.Hash()
        buf = binary.BigEndian.AppendUint64(buf, h)
    }
    return buf
}
```

### 6.2 NLRI Registry

```go
// pkg/bgp/nlri/registry.go

type NLRIUnpacker interface {
    Unpack(afi AFI, safi SAFI, data []byte, addpath bool, neg *Negotiated) (NLRI, []byte, error)
}

var nlriRegistry = map[Family]NLRIUnpacker{}

func RegisterNLRI(family Family, u NLRIUnpacker) {
    nlriRegistry[family] = u
}
```

### 6.3 IPv4/IPv6 Unicast (INET)

```go
// pkg/bgp/nlri/inet.go

type INET struct {
    family   Family
    prefix   netip.Prefix
    pathID   uint32 // ADD-PATH path identifier
    _packed  []byte // Cached wire format
}

func (i *INET) Family() Family { return i.family }

func (i *INET) Pack(neg *Negotiated) []byte {
    if i._packed != nil {
        return i._packed
    }
    // Pack prefix with optional path ID
    // ...
}

type INETUnpacker struct{}

func (INETUnpacker) Unpack(afi AFI, safi SAFI, data []byte, addpath bool, neg *Negotiated) (NLRI, []byte, error) {
    offset := 0
    var pathID uint32

    if addpath {
        if len(data) < 4 {
            return nil, nil, ErrShortRead
        }
        pathID = binary.BigEndian.Uint32(data[:4])
        offset = 4
    }

    if len(data) <= offset {
        return nil, nil, ErrShortRead
    }

    prefixLen := int(data[offset])
    offset++
    prefixBytes := (prefixLen + 7) / 8

    if len(data) < offset+prefixBytes {
        return nil, nil, ErrShortRead
    }

    // Build netip.Prefix from wire format
    // ...

    return &INET{
        family:  Family{afi, safi},
        prefix:  prefix,
        pathID:  pathID,
        _packed: data[:offset+prefixBytes],
    }, data[offset+prefixBytes:], nil
}
```

### 6.4 VPN (IPVPN)

```go
// pkg/bgp/nlri/ipvpn.go

type IPVPN struct {
    family Family
    rd     RouteDistinguisher
    labels LabelStack
    prefix netip.Prefix
    pathID uint32
}

type RouteDistinguisher struct {
    Type  uint16
    Value [6]byte
}

type LabelStack []uint32
```

### 6.5 EVPN

```go
// pkg/bgp/nlri/evpn.go

type EVPNRouteType uint8

const (
    EVPNEthernetAutoDiscovery EVPNRouteType = 1
    EVPNMACIPAdvertisement    EVPNRouteType = 2
    EVPNInclusiveMulticast    EVPNRouteType = 3
    EVPNEthernetSegment       EVPNRouteType = 4
    EVPNIPPrefix              EVPNRouteType = 5
)

type EVPN interface {
    NLRI
    RouteType() EVPNRouteType
}

// Type 2: MAC/IP Advertisement
type EVPNMACIPRoute struct {
    RD          RouteDistinguisher
    ESI         [10]byte
    EthernetTag uint32
    MAC         [6]byte
    IPAddr      netip.Addr // optional
    Label1      uint32
    Label2      uint32 // optional
}
```

### 6.6 FlowSpec

```go
// pkg/bgp/nlri/flowspec.go

type FlowSpec struct {
    family     Family
    components []FlowComponent
}

type FlowComponentType uint8

const (
    FlowDestPrefix   FlowComponentType = 1
    FlowSourcePrefix FlowComponentType = 2
    FlowIPProtocol   FlowComponentType = 3
    FlowPort         FlowComponentType = 4
    FlowDstPort      FlowComponentType = 5
    FlowSrcPort      FlowComponentType = 6
    FlowICMPType     FlowComponentType = 7
    FlowICMPCode     FlowComponentType = 8
    FlowTCPFlags     FlowComponentType = 9
    FlowPacketLen    FlowComponentType = 10
    FlowDSCP         FlowComponentType = 11
    FlowFragment     FlowComponentType = 12
)

type FlowComponent interface {
    Type() FlowComponentType
    Pack() []byte
}
```

### 6.7 BGP-LS

```go
// pkg/bgp/nlri/bgpls.go

type BGPLSNLRIType uint16

const (
    BGPLSNode      BGPLSNLRIType = 1
    BGPLSLink      BGPLSNLRIType = 2
    BGPLSPrefixV4  BGPLSNLRIType = 3
    BGPLSPrefixV6  BGPLSNLRIType = 4
    BGPLSSRv6SID   BGPLSNLRIType = 6
)

type BGPLSNode struct {
    ProtocolID   uint8
    Identifier   uint64
    LocalNode    NodeDescriptor
}

type NodeDescriptor struct {
    ASN         uint32
    BGPLSIdentifier uint32
    OSPFAreaID  uint32
    IGPRouterID []byte
}
```

### 6.8 MUP (Mobile User Plane)

```go
// pkg/bgp/nlri/mup.go

type MUPRouteType uint16

const (
    MUPISD  MUPRouteType = 1 // Interwork Segment Discovery
    MUPDSD  MUPRouteType = 2 // Direct Segment Discovery
    MUPT1ST MUPRouteType = 3 // Type 1 Session Transformed
    MUPT2ST MUPRouteType = 4 // Type 2 Session Transformed
)
```

### 6.9 NLRI Deduplication Store

```go
// internal/store/nlri_store.go

// NLRIStore provides centralized NLRI deduplication
type NLRIStore struct {
    stores map[Family]*nlriTypedStore
    mu     sync.RWMutex
}

type nlriTypedStore struct {
    mu    sync.RWMutex
    items map[string]NLRI // key = hex(Index())
}

func (s *NLRIStore) Intern(nlri NLRI) NLRI {
    store := s.getOrCreateStore(nlri.Family())
    return store.intern(nlri)
}
```

**Deliverables:**
- [ ] NLRI interface with AS-PATH extension
- [ ] NLRI registry
- [ ] INET (IPv4/IPv6 unicast/multicast)
- [ ] IPVPN (VPNv4/VPNv6)
- [ ] EVPN (all 5 route types)
- [ ] FlowSpec
- [ ] BGP-LS (Node, Link, Prefix, SRv6)
- [ ] MUP
- [ ] MVPN
- [ ] VPLS
- [ ] RTC
- [ ] NLRI deduplication store
- [ ] Unit tests for all NLRI types

---

## Phase 7: RIB (Routing Information Base) (Week 8-9)

### 7.1 Route Structure

```go
// pkg/rib/route.go

// Route represents a BGP route with AS-PATH as part of identity
type Route struct {
    // Identity (used for deduplication)
    nlriWithPath *NLRIWithPath

    // Shared attributes (deduplicated)
    attributes *Attributes

    // Next-hop (stored separately, not in NLRI)
    nextHop netip.Addr

    // Reference counting for memory management
    refCount atomic.Int32

    // Cached index for fast lookup
    _index []byte
}

func (r *Route) Index() []byte {
    if r._index == nil {
        r._index = r.nlriWithPath.Index()
    }
    return r._index
}
```

### 7.2 RIB Structure

```go
// pkg/rib/rib.go

type RIB struct {
    incoming *IncomingRIB
    outgoing *OutgoingRIB

    // Global route store for deduplication
    globalStore *RouteStore
}

type IncomingRIB struct {
    // peer -> family -> nlri_index -> route
    routes map[string]map[Family]map[string]*Route
    mu     sync.RWMutex
}

type OutgoingRIB struct {
    // family -> attr_index -> nlri_index -> route
    pending map[Family]map[string]map[string]*Route

    // Cached routes for resend
    cache map[Family]map[string]*Route

    mu sync.RWMutex
}
```

### 7.3 Route Store with AS-PATH Separation

```go
// pkg/rib/store.go

// RouteStore implements the novel AS-PATH-as-NLRI approach
type RouteStore struct {
    // Routes indexed by (NLRI + AS-PATH)
    // This allows maximum attribute sharing
    routes sync.Map // map[string]*Route (key = route.Index())

    // Attribute deduplication
    attrStore *AttributeStore

    // NLRI deduplication
    nlriStore *NLRIStore
}

func (s *RouteStore) Intern(nlri NLRI, asPath *ASPath, attrs *Attributes, nextHop netip.Addr) *Route {
    // Create NLRIWithPath for indexing
    nlriWithPath := &NLRIWithPath{
        NLRI:   s.nlriStore.Intern(nlri),
        ASPath: asPath, // AS-PATH is part of identity
    }

    // Deduplicate attributes (excluding AS-PATH which is now in NLRI)
    attrsWithoutASPath := s.attrStore.InternWithout(attrs, AttrASPath)

    // Create or reuse route
    key := string(nlriWithPath.Index())
    if existing, ok := s.routes.Load(key); ok {
        route := existing.(*Route)
        route.refCount.Add(1)
        return route
    }

    route := &Route{
        nlriWithPath: nlriWithPath,
        attributes:   attrsWithoutASPath,
        nextHop:      nextHop,
    }
    route.refCount.Store(1)

    s.routes.Store(key, route)
    return route
}
```

**Deliverables:**
- [ ] Route structure with AS-PATH extension
- [ ] IncomingRIB (Adj-RIB-In)
- [ ] OutgoingRIB (Adj-RIB-Out)
- [ ] Route deduplication store
- [ ] Reference counting for memory management
- [ ] Unit tests

---

## Phase 8: FSM (Finite State Machine) (Week 9-10)

### 8.1 States and Events

```go
// pkg/bgp/fsm/fsm.go

type State int

const (
    StateIdle State = iota
    StateConnect
    StateActive
    StateOpenSent
    StateOpenConfirm
    StateEstablished
)

type Event int

const (
    EventManualStart Event = iota
    EventManualStop
    EventConnectRetryTimerExpires
    EventHoldTimerExpires
    EventKeepaliveTimerExpires
    EventTCPConnectionConfirmed
    EventTCPConnectionFails
    EventBGPOpen
    EventBGPHeaderErr
    EventBGPOpenMsgErr
    EventNotifMsgVerErr
    EventNotifMsg
    EventKeepaliveMsg
    EventUpdateMsg
    EventUpdateMsgErr
)
```

### 8.2 FSM Implementation

```go
// pkg/bgp/fsm/machine.go

type FSM struct {
    state         State
    neighbor      *Neighbor

    // Timers
    connectRetry  *time.Timer
    holdTimer     *time.Timer
    keepaliveTimer *time.Timer

    // Channel for events
    events        chan Event

    // Connection
    conn          net.Conn

    // Negotiated capabilities
    negotiated    *Negotiated
}

func NewFSM(neighbor *Neighbor) *FSM {
    return &FSM{
        state:    StateIdle,
        neighbor: neighbor,
        events:   make(chan Event, 16),
    }
}

func (f *FSM) Run(ctx context.Context) error {
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case event := <-f.events:
            if err := f.handleEvent(event); err != nil {
                return err
            }
        }
    }
}

func (f *FSM) handleEvent(event Event) error {
    // State machine transitions based on RFC 4271 Section 8
    switch f.state {
    case StateIdle:
        return f.handleIdle(event)
    case StateConnect:
        return f.handleConnect(event)
    // ... other states
    }
    return nil
}
```

**Deliverables:**
- [ ] FSM states and events defined
- [ ] State transition logic (RFC 4271)
- [ ] Timer management
- [ ] Event handling
- [ ] Integration with peer
- [ ] Unit tests

---

## Phase 9: Reactor (Week 10-12)

### 9.1 Reactor Architecture

```go
// pkg/reactor/reactor.go

type Reactor struct {
    config     *Config
    peers      map[string]*Peer
    listener   *Listener
    api        *API

    // Signals
    sigChan    chan os.Signal

    // Shutdown coordination
    ctx        context.Context
    cancel     context.CancelFunc
    wg         sync.WaitGroup

    // Stats
    stats      *Stats
}

func New(config *Config) *Reactor {
    ctx, cancel := context.WithCancel(context.Background())
    return &Reactor{
        config: config,
        peers:  make(map[string]*Peer),
        ctx:    ctx,
        cancel: cancel,
    }
}

func (r *Reactor) Run() error {
    // Setup signal handling
    r.sigChan = make(chan os.Signal, 1)
    signal.Notify(r.sigChan, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGUSR1)

    // Start listener
    if err := r.startListener(); err != nil {
        return err
    }

    // Start API server
    if err := r.startAPI(); err != nil {
        return err
    }

    // Start peers
    for _, neighbor := range r.config.Neighbors {
        r.startPeer(neighbor)
    }

    // Main loop
    return r.mainLoop()
}
```

### 9.2 Peer Goroutine

```go
// pkg/reactor/peer.go

type Peer struct {
    neighbor   *Neighbor
    fsm        *FSM
    rib        *RIB

    // Channels for coordination
    incoming   chan Message
    outgoing   chan Message

    // State
    state      atomic.Value // PeerState

    ctx        context.Context
    cancel     context.CancelFunc
}

func (p *Peer) Run() {
    defer p.cleanup()

    for {
        select {
        case <-p.ctx.Done():
            return
        default:
            if err := p.runOnce(); err != nil {
                log.Error("peer error", "peer", p.neighbor.Address, "err", err)
                p.reconnect()
            }
        }
    }
}

func (p *Peer) runOnce() error {
    // Connect
    if err := p.connect(); err != nil {
        return err
    }
    defer p.disconnect()

    // Run FSM
    return p.fsm.Run(p.ctx)
}
```

### 9.3 Per-Attribute Goroutine Stores

```go
// pkg/reactor/attribute_workers.go

// AttributeWorker manages a single attribute type's deduplication
type AttributeWorker struct {
    code     AttributeCode
    store    *typedStore
    incoming chan Attribute
    ctx      context.Context
}

func (w *AttributeWorker) Run() {
    for {
        select {
        case <-w.ctx.Done():
            return
        case attr := <-w.incoming:
            w.store.intern(attr)
        }
    }
}

// AttributeWorkerPool manages all attribute workers
type AttributeWorkerPool struct {
    workers map[AttributeCode]*AttributeWorker
}

func NewAttributeWorkerPool(ctx context.Context) *AttributeWorkerPool {
    pool := &AttributeWorkerPool{
        workers: make(map[AttributeCode]*AttributeWorker),
    }

    // Create worker for each known attribute type
    for code := range knownAttributeTypes {
        pool.workers[code] = newAttributeWorker(ctx, code)
        go pool.workers[code].Run()
    }

    // Generic worker for unknown attributes
    pool.workers[0] = newAttributeWorker(ctx, 0)
    go pool.workers[0].Run()

    return pool
}
```

### 9.4 Zero-Copy Message Passing

```go
// pkg/reactor/passthrough.go

// PassthroughMessage represents an UPDATE that can be forwarded unchanged
type PassthroughMessage struct {
    rawData    []byte
    families   []Family  // Which families are affected
    fromPeer   string

    // Lazy parsed content
    parsed     atomic.Bool
    parsedData *Update
}

func (p *PassthroughMessage) CanForwardTo(peer *Peer) bool {
    // Check if peer supports all families
    for _, f := range p.families {
        if !peer.SupportsFamily(f) {
            return false
        }
    }
    // Check if AS-PATH manipulation is needed
    // ... (eBGP vs iBGP, AS prepending, etc.)
    return true
}

func (p *PassthroughMessage) ForwardData() []byte {
    return p.rawData
}
```

**Deliverables:**
- [ ] Reactor main loop
- [ ] Peer goroutine management
- [ ] Listener for incoming connections
- [ ] Signal handling (SIGHUP, SIGUSR1, etc.)
- [ ] Per-attribute worker goroutines
- [ ] Zero-copy message passing
- [ ] Configuration reload
- [ ] Graceful shutdown
- [ ] Unit and integration tests

---

## Phase 10: Configuration (Week 12-13)

### 10.1 Configuration Structure

```go
// pkg/config/config.go

type Config struct {
    // Global settings
    Process    ProcessConfig    `yaml:"process"`
    Log        LogConfig        `yaml:"log"`
    API        APIConfig        `yaml:"api"`

    // BGP settings
    Neighbors  []NeighborConfig `yaml:"neighbor"`
    Templates  []TemplateConfig `yaml:"template"`
}

type NeighborConfig struct {
    Description   string           `yaml:"description"`
    RouterID      string           `yaml:"router-id"`
    LocalAddress  string           `yaml:"local-address"`
    LocalAS       uint32           `yaml:"local-as"`
    PeerAddress   string           `yaml:"peer-address"`
    PeerAS        uint32           `yaml:"peer-as"`
    HoldTime      uint16           `yaml:"hold-time"`
    Passive       bool             `yaml:"passive"`
    Families      []string         `yaml:"family"`

    // Capabilities
    ASN4          bool             `yaml:"asn4"`
    AddPath       AddPathConfig    `yaml:"add-path"`

    // Static routes
    Static        []StaticRoute    `yaml:"static"`

    // API processes
    Processes     []ProcessRef     `yaml:"process"`
}
```

### 10.2 Environment Variables

```go
// pkg/config/env.go

// EnvConfig maps ExaBGP environment variables
type EnvConfig struct {
    TCP struct {
        Bind     []string `env:"EXABGP_TCP_BIND"`
        Port     int      `env:"EXABGP_TCP_PORT" default:"179"`
        Attempts int      `env:"EXABGP_TCP_ATTEMPTS" default:"0"`
    }
    BGP struct {
        OpenWait int  `env:"EXABGP_BGP_OPENWAIT" default:"60"`
        Passive  bool `env:"EXABGP_BGP_PASSIVE" default:"false"`
    }
    Log struct {
        Enable      bool   `env:"EXABGP_LOG_ENABLE" default:"true"`
        Level       string `env:"EXABGP_LOG_LEVEL" default:"INFO"`
        Destination string `env:"EXABGP_LOG_DESTINATION" default:"stdout"`
    }
    API struct {
        Encoder    string `env:"EXABGP_API_ENCODER" default:"json"`
        Ack        bool   `env:"EXABGP_API_ACK" default:"true"`
        SocketName string `env:"EXABGP_API_SOCKETNAME" default:"zebgp"`
    }
    Daemon struct {
        Daemonize bool   `env:"EXABGP_DAEMON_DAEMONIZE" default:"false"`
        User      string `env:"EXABGP_DAEMON_USER" default:"nobody"`
        PID       string `env:"EXABGP_DAEMON_PID"`
    }
}
```

### 10.3 Configuration Parser

```go
// pkg/config/parser.go

// Parser handles ExaBGP-compatible configuration format
type Parser struct {
    // Tokenizer for ExaBGP config format
    tokenizer *Tokenizer
}

func ParseFile(path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }

    // Detect format: YAML or ExaBGP native
    if strings.HasPrefix(strings.TrimSpace(string(data)), "{") ||
       strings.HasPrefix(strings.TrimSpace(string(data)), "process") {
        return parseExaBGPFormat(data)
    }
    return parseYAMLFormat(data)
}
```

**Deliverables:**
- [ ] Configuration structures
- [ ] Environment variable support
- [ ] ExaBGP config format parser
- [ ] YAML config format support
- [ ] Configuration validation
- [ ] Template system
- [ ] Unit tests

---

## Phase 11: CLI (Week 13-14)

### 11.1 CLI Commands

```go
// cmd/zebgp/main.go

func main() {
    rootCmd := &cobra.Command{
        Use:   "zebgp",
        Short: "ZeBGP - BGP daemon",
    }

    // Subcommands
    rootCmd.AddCommand(
        runCmd(),
        configCmd(),
        encodeCmd(),
        decodeCmd(),
        versionCmd(),
    )

    rootCmd.Execute()
}

func runCmd() *cobra.Command {
    return &cobra.Command{
        Use:   "run [config-file]",
        Short: "Run the BGP daemon",
        RunE: func(cmd *cobra.Command, args []string) error {
            config, err := loadConfig(args[0])
            if err != nil {
                return err
            }
            reactor := reactor.New(config)
            return reactor.Run()
        },
    }
}

func encodeCmd() *cobra.Command {
    return &cobra.Command{
        Use:   "encode <route-spec>",
        Short: "Encode route to BGP UPDATE hex",
        RunE: func(cmd *cobra.Command, args []string) error {
            // Parse route specification
            // Encode to UPDATE message
            // Output hex
        },
    }
}

func decodeCmd() *cobra.Command {
    return &cobra.Command{
        Use:   "decode <hex>",
        Short: "Decode BGP UPDATE from hex",
        RunE: func(cmd *cobra.Command, args []string) error {
            // Parse hex input
            // Decode UPDATE message
            // Output JSON
        },
    }
}
```

### 11.2 Interactive CLI

```go
// cmd/zebgp-cli/main.go

type CLI struct {
    conn      net.Conn
    completer *Completer
    history   *History
}

func (c *CLI) Run() error {
    // Connect to daemon Unix socket
    if err := c.connect(); err != nil {
        return err
    }
    defer c.conn.Close()

    // Setup readline
    rl, err := readline.New("ZeBGP> ")
    if err != nil {
        return err
    }
    defer rl.Close()

    rl.Config.AutoComplete = c.completer

    // REPL
    for {
        line, err := rl.Readline()
        if err != nil {
            if err == readline.ErrInterrupt {
                continue
            }
            break
        }

        response, err := c.execute(line)
        if err != nil {
            fmt.Println("Error:", err)
            continue
        }
        fmt.Println(response)
    }

    return nil
}
```

**Deliverables:**
- [ ] Main daemon command
- [ ] Configuration validation command
- [ ] Encode/decode commands
- [ ] Interactive CLI client
- [ ] Tab completion
- [ ] Command history
- [ ] Unit tests

---

## Phase 12: API (Week 14-15)

### 12.1 Unix Socket API

```go
// pkg/api/server.go

type Server struct {
    reactor    *Reactor
    socketPath string
    listener   net.Listener
    clients    map[string]*Client
    mu         sync.RWMutex
}

func (s *Server) Run(ctx context.Context) error {
    listener, err := net.Listen("unix", s.socketPath)
    if err != nil {
        return err
    }
    s.listener = listener

    go s.acceptLoop(ctx)

    <-ctx.Done()
    return s.listener.Close()
}

func (s *Server) acceptLoop(ctx context.Context) {
    for {
        conn, err := s.listener.Accept()
        if err != nil {
            select {
            case <-ctx.Done():
                return
            default:
                log.Error("accept error", "err", err)
                continue
            }
        }

        client := newClient(conn, s)
        s.addClient(client)
        go client.handle()
    }
}
```

### 12.2 API Commands

```go
// pkg/api/command/commands.go

type CommandRegistry struct {
    commands map[string]CommandHandler
}

type CommandHandler func(reactor *Reactor, args []string) (string, error)

func init() {
    // Register all API commands
    Register("show neighbor", showNeighbor)
    Register("show neighbor summary", showNeighborSummary)
    Register("show adj-rib in", showAdjRibIn)
    Register("show adj-rib out", showAdjRibOut)
    Register("announce route", announceRoute)
    Register("withdraw route", withdrawRoute)
    Register("announce eor", announceEOR)
    Register("announce flow", announceFlow)
    Register("teardown", teardown)
    Register("shutdown", shutdown)
    // ... all ExaBGP commands
}

func showNeighborSummary(r *Reactor, args []string) (string, error) {
    neighbors := r.GetNeighborSummaries()

    return json.Marshal(map[string]interface{}{
        "neighbors": neighbors,
    })
}
```

### 12.3 JSON Output Format

```go
// pkg/api/json.go

// JSONEncoder produces ExaBGP-compatible JSON output
type JSONEncoder struct {
    version string
}

func (e *JSONEncoder) EncodeUpdate(update *Update, neighbor *Neighbor) ([]byte, error) {
    output := map[string]interface{}{
        "exabgp": e.version,
        "type":   "update",
        "neighbor": map[string]interface{}{
            "address": map[string]string{
                "local": neighbor.LocalAddress,
                "peer":  neighbor.PeerAddress,
            },
            "asn": map[string]uint32{
                "local": neighbor.LocalAS,
                "peer":  neighbor.PeerAS,
            },
            "direction": "in",
            "message": map[string]interface{}{
                "update": e.encodeUpdateBody(update),
            },
        },
    }

    return json.Marshal(output)
}
```

### 12.4 External Process Communication

```go
// pkg/api/process.go

// ProcessManager handles external API processes
type ProcessManager struct {
    processes map[string]*Process
    reactor   *Reactor
}

type Process struct {
    name    string
    cmd     *exec.Cmd
    stdin   io.WriteCloser
    stdout  io.ReadCloser
    stderr  io.ReadCloser
}

func (p *Process) SendUpdate(json []byte) error {
    _, err := p.stdin.Write(append(json, '\n'))
    return err
}

func (p *Process) ReadCommands() <-chan string {
    ch := make(chan string)
    go func() {
        scanner := bufio.NewScanner(p.stdout)
        for scanner.Scan() {
            ch <- scanner.Text()
        }
        close(ch)
    }()
    return ch
}
```

**Deliverables:**
- [ ] Unix socket server
- [ ] Client connection handling
- [ ] All ExaBGP API commands
- [ ] JSON output format (ExaBGP compatible)
- [ ] External process management
- [ ] Ping/pong health monitoring
- [ ] Unit tests

---

## Phase 13: Testing Infrastructure (Week 15-17)

### 13.1 Test Conversion Strategy

Convert ExaBGP's Python tests to Go:

```go
// testdata/encoding/conf_ebgp_test.go

func TestConfEBGP(t *testing.T) {
    tests := []struct {
        name     string
        command  string
        expected string // hex
        json     string
    }{
        {
            name:    "announce_route_with_aspath",
            command: "announce route 10.0.0.0/24 next-hop 10.0.1.254 origin igp as-path [65533]",
            expected: "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF002F02000000144001010040020602010000FFFD4003040A0001FE180A0000",
        },
        {
            name:    "announce_eor",
            command: "announce eor ipv4 unicast",
            expected: "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00170200000000",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            update, err := parseCommand(tt.command)
            require.NoError(t, err)

            packed, err := update.Pack(nil)
            require.NoError(t, err)

            assert.Equal(t, tt.expected, hex.EncodeToString(packed))
        })
    }
}
```

### 13.2 Functional Test Runner

```go
// testdata/functional_test.go

func TestFunctionalEncoding(t *testing.T) {
    tests := loadTestCases("testdata/encoding/*.ci")

    for _, tc := range tests {
        t.Run(tc.Name, func(t *testing.T) {
            // Start server
            server := startTestServer(tc.Config)
            defer server.Stop()

            // Start client
            client := startTestClient(tc.Config)
            defer client.Stop()

            // Send commands
            for _, cmd := range tc.Commands {
                err := client.Send(cmd)
                require.NoError(t, err)
            }

            // Verify messages
            for _, expected := range tc.Expected {
                msg := server.ReceiveMessage()
                assert.Equal(t, expected.Hex, hex.EncodeToString(msg))
            }
        })
    }
}
```

### 13.3 Round-Trip Tests

```go
// testdata/roundtrip_test.go

func TestRoundTrip(t *testing.T) {
    // Test that pack(unpack(data)) == data
    messages := loadTestMessages("testdata/messages/*.hex")

    for _, msg := range messages {
        t.Run(msg.Name, func(t *testing.T) {
            data, err := hex.DecodeString(msg.Hex)
            require.NoError(t, err)

            parsed, err := UnpackMessage(data)
            require.NoError(t, err)

            repacked, err := parsed.Pack(nil)
            require.NoError(t, err)

            assert.Equal(t, data, repacked)
        })
    }
}
```

### 13.4 Test Data Conversion Script

```bash
#!/bin/bash
# scripts/convert-tests.sh

# Convert ExaBGP .ci files to Go test cases

for ci_file in ../qa/encoding/*.ci; do
    name=$(basename "$ci_file" .ci)
    go_file="testdata/encoding/${name}_test.go"

    python3 scripts/ci_to_go.py "$ci_file" > "$go_file"
done
```

**Deliverables:**
- [ ] Test case loader from .ci files
- [ ] Unit tests for all components
- [ ] Encoding/decoding tests
- [ ] Round-trip tests
- [ ] Functional test runner
- [ ] Test data conversion scripts
- [ ] CI/CD integration
- [ ] Coverage reporting

---

## Phase 14: Integration & Polish (Week 17-20)

### 14.1 End-to-End Testing

```go
// integration_test.go

func TestE2EIBGP(t *testing.T) {
    // Start two ZeBGP instances
    peer1 := startZeBGP(peer1Config)
    peer2 := startZeBGP(peer2Config)
    defer peer1.Stop()
    defer peer2.Stop()

    // Wait for session establishment
    require.Eventually(t, func() bool {
        return peer1.IsEstablished(peer2.Address()) &&
               peer2.IsEstablished(peer1.Address())
    }, 30*time.Second, 100*time.Millisecond)

    // Announce route from peer1
    peer1.API("announce route 10.0.0.0/24 next-hop 192.168.1.1")

    // Verify peer2 receives it
    require.Eventually(t, func() bool {
        return peer2.HasRoute("10.0.0.0/24")
    }, 10*time.Second, 100*time.Millisecond)
}
```

### 14.2 Performance Benchmarks

```go
// benchmark_test.go

func BenchmarkUpdateParsing(b *testing.B) {
    data := loadUpdateMessage("testdata/large_update.bin")

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, err := UnpackUpdate(data, nil)
        if err != nil {
            b.Fatal(err)
        }
    }
}

func BenchmarkAttributeDedup(b *testing.B) {
    store := NewAttributeStore()
    attrs := generateRandomAttributes(10000)

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        store.Intern(attrs[i%len(attrs)])
    }
}

func BenchmarkRIBInsert(b *testing.B) {
    rib := NewRIB()
    routes := generateRandomRoutes(100000)

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        rib.Insert(routes[i%len(routes)])
    }
}
```

### 14.3 Documentation

- API documentation (godoc format)
- Configuration reference
- Migration guide from ExaBGP
- Architecture documentation
- Performance tuning guide

### 14.4 Release Preparation

- Version numbering (match ExaBGP 6.x)
- Changelog
- Release binaries (Linux, macOS, Windows)
- Docker image
- Homebrew formula

**Deliverables:**
- [ ] End-to-end tests passing
- [ ] Performance benchmarks
- [ ] Memory profiling
- [ ] Documentation complete
- [ ] Release artifacts
- [ ] Docker image
- [ ] CI/CD for releases

---

## Risk Mitigation

### Technical Risks

| Risk | Impact | Mitigation |
|------|--------|------------|
| Complex NLRI types (FlowSpec, BGP-LS) | High | Start with simpler types, iterate |
| AS-PATH-as-NLRI approach issues | Medium | Fallback to traditional approach if needed |
| Performance not meeting goals | Medium | Profile early, optimize hot paths |
| ExaBGP config compatibility | High | Extensive testing with real configs |

### Schedule Risks

| Risk | Impact | Mitigation |
|------|--------|------------|
| Scope creep | High | Strict phase deliverables |
| Underestimated complexity | Medium | Buffer time in schedule |
| Testing takes longer | Medium | Parallel test development |

---

## Success Criteria

### Functional Requirements

- [ ] All ExaBGP API commands work identically
- [ ] Same JSON output format
- [ ] Same configuration file support
- [ ] All 72 encoding tests pass
- [ ] All 18 decoding tests pass
- [ ] Interoperates with ExaBGP

### Performance Requirements

- [ ] 10x faster UPDATE parsing than Python
- [ ] 50% lower memory usage for large RIB
- [ ] Handle 1M routes per peer
- [ ] Startup time < 1 second

### Quality Requirements

- [ ] 80%+ code coverage
- [ ] Zero known race conditions
- [ ] Graceful degradation under load
- [ ] Clean shutdown with route withdrawal

---

## Appendix A: Go Libraries Comparison

| Purpose | Option 1 | Option 2 | Recommendation |
|---------|----------|----------|----------------|
| CLI | cobra | urfave/cli | cobra (more mature) |
| Config | viper | koanf | viper (ExaBGP compat) |
| Logging | slog | zap | slog (stdlib) |
| Testing | testify | ginkgo | testify (simpler) |
| JSON | sonic | stdlib | stdlib (sufficient) |

---

## Appendix B: ExaBGP API Command Matrix

| Command | Priority | Status |
|---------|----------|--------|
| show neighbor | P0 | - |
| show neighbor summary | P0 | - |
| show adj-rib in | P0 | - |
| show adj-rib out | P0 | - |
| announce route | P0 | - |
| withdraw route | P0 | - |
| announce eor | P0 | - |
| announce flow | P1 | - |
| announce vpls | P1 | - |
| announce l2vpn | P1 | - |
| teardown | P0 | - |
| shutdown | P0 | - |
| reload | P1 | - |

---

## Appendix C: NLRI Type Priority

| NLRI Type | Priority | Complexity |
|-----------|----------|------------|
| IPv4/IPv6 Unicast | P0 | Low |
| IPv4/IPv6 VPN | P0 | Medium |
| FlowSpec | P1 | High |
| EVPN | P1 | High |
| BGP-LS | P2 | High |
| VPLS | P2 | Medium |
| MUP | P2 | Medium |
| MVPN | P2 | Medium |

---

**Document Version:** 1.0
**Last Updated:** 2025-12-19
