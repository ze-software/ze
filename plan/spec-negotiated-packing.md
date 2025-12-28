# Spec: Unified Negotiated Packing Pattern

## Task
Refactor wire format encoding to use consistent `Pack(negotiated)` pattern across NLRI, attributes, and messages.

## Problem Analysis

Current code has inconsistent patterns for capability-dependent encoding:

```go
// Pattern 1: Ignores negotiation
nlri.Bytes()  // No ADD-PATH awareness

// Pattern 2: Explicit parameter
attribute.PackASPathAttribute(asPath, asn4 bool)  // ASN4 as param

// Pattern 3: Negotiated struct
message.Pack(neg *Negotiated)  // Full negotiated state
```

This causes:
1. ADD-PATH bug (current issue - test R failing)
2. Potential ASN4 issues if we receive 4-byte and send to 2-byte peer
3. Inconsistent API making code harder to maintain

## Proposed Pattern

### Unified Interface

```go
// In pkg/bgp/message/negotiated.go (expand existing)
type Negotiated struct {
    ASN4            bool              // RFC 6793
    AddPath         map[Family]bool   // RFC 7911 - send capability per family
    ExtendedMessage bool              // RFC 8654
    ExtendedNextHop map[Family]AFI    // RFC 8950
    LocalAS         uint32
    PeerAS          uint32
    HoldTime        uint16
}

// Packable is the interface for capability-aware wire encoding
type Packable interface {
    Pack(neg *Negotiated) []byte
}
```

### NLRI Changes

```go
// Before
type NLRI interface {
    Bytes() []byte  // Ignores negotiation
}

// After
type NLRI interface {
    Bytes() []byte           // For internal use, RIB keys, etc.
    Pack(neg *Negotiated) []byte  // For wire encoding
}
```

### Attribute Changes

```go
// Before
func PackASPathAttribute(asPath *ASPath, asn4 bool) []byte

// After - ASPath implements Packable
func (a *ASPath) Pack(neg *Negotiated) []byte {
    if neg.ASN4 {
        return a.pack4Byte()
    }
    return a.pack2ByteWithAS4Path()  // Returns AS_PATH + AS4_PATH if needed
}
```

## Capabilities Affected

### 1. ADD-PATH (RFC 7911) - NLRI

| Scenario | Action |
|----------|--------|
| Send=true, has path ID | Include 4-byte path ID |
| Send=true, no path ID | Prepend NOPATH (4 zeros) |
| Send=false | Omit path ID |

### 2. ASN4 (RFC 6793) - AS_PATH

| Scenario | Action |
|----------|--------|
| ASN4=true | 4-byte AS numbers |
| ASN4=false, all ASNs ≤65535 | 2-byte AS numbers |
| ASN4=false, any ASN >65535 | AS_PATH with AS_TRANS (23456), plus AS4_PATH |

**Note:** AS4_PATH (type 17) and AS4_AGGREGATOR (type 18) are transitive optional attributes that carry the real 4-byte AS path when communicating with 2-byte peers.

### 3. Extended Message (RFC 8654) - Message Header

| Scenario | Action |
|----------|--------|
| ExtendedMessage=true | Allow messages up to 65535 bytes |
| ExtendedMessage=false | Max 4096 bytes, split if needed |

### 4. Extended Next Hop (RFC 8950) - MP_REACH_NLRI

| Scenario | Action |
|----------|--------|
| ExtNH negotiated for family | Can use IPv6 next-hop for IPv4 NLRI |
| Not negotiated | Must use matching AFI for next-hop |

## Implementation Steps

### Phase 1: NLRI (Current Priority)

1. Add `Pack(neg *Negotiated) []byte` to NLRI interface
2. Implement for INET with ADD-PATH handling
3. Implement for other NLRI types (IPVPN, EVPN, FlowSpec, BGPLS)
4. Update CommitService to use `Pack(neg)` instead of `Bytes()`

### Phase 2: AS_PATH (Future)

1. Change ASPath to implement Packable
2. Handle AS4_PATH generation for 2-byte peers
3. Update attribute packing to use `Pack(neg)`

### Phase 3: Other Attributes (Future)

1. Extended Communities with ASN fields
2. Aggregator attribute (2 vs 4 byte AS)

## Codebase Context

**Key files:**
- `pkg/bgp/message/message.go` - Negotiated struct
- `pkg/bgp/nlri/nlri.go` - NLRI interface
- `pkg/bgp/nlri/inet.go` - INET implementation
- `pkg/bgp/attribute/aspath.go` - AS_PATH encoding
- `pkg/rib/commit.go` - Uses NLRI.Bytes() currently

**ExaBGP reference:**
- All NLRI types have `pack_nlri(negotiated)` method
- Consistent pattern throughout codebase

## Verification Checklist

### Phase 1 (This PR)
- [ ] NLRI interface has Pack method
- [ ] INET.Pack handles ADD-PATH correctly
- [ ] Other NLRI types have Pack (can delegate to Bytes for now)
- [ ] CommitService uses Pack
- [ ] Test R passes
- [ ] `make test && make lint` passes

### Phase 2 (Future PR)
- [ ] ASPath.Pack handles ASN4
- [ ] AS4_PATH generated when needed
- [ ] Tests for 4-to-2 byte AS path conversion

## Migration Strategy

1. **Add Pack method** alongside existing Bytes()
2. **Update callers** to use Pack where negotiation matters
3. **Keep Bytes()** for internal use (RIB keys, debugging)
4. **No breaking changes** - existing code continues to work

## Example Usage

```go
// Before
nlriBytes := route.NLRI().Bytes()

// After
family := route.NLRI().Family()
neg := &message.Negotiated{
    AddPath: map[message.Family]bool{
        {AFI: uint16(family.AFI), SAFI: uint8(family.SAFI)}: true,
    },
}
nlriBytes := route.NLRI().Pack(neg)
```

## Open Questions

1. Should we pass full Negotiated or just relevant fields?
   - Full: Consistent API, future-proof
   - Partial: Smaller interface, clearer dependencies

   **Recommendation:** Full Negotiated for consistency with messages

2. Should Pack return error?
   - Current Bytes() doesn't
   - Pack could fail if capabilities incompatible

   **Recommendation:** No error - caller ensures valid negotiation

3. Should we rename Bytes() to something else?
   - `RawBytes()` - emphasizes it's unprocessed
   - `Index()` - if only used for RIB keys

   **Recommendation:** Keep Bytes() - clear meaning, no breaking changes
