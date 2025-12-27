# UPDATE Message Builder Pattern

**Status:** Planning
**Created:** 2025-12-26

## Problem Statement

Current UPDATE building involves repetitive `append()` calls:

```go
var attrBytes []byte
attrBytes = append(attrBytes, attribute.PackAttribute(attribute.Origin(attribute.OriginIGP))...)
attrBytes = append(attrBytes, attribute.PackAttribute(attribute.ASPath([]uint32{}))...)
attrBytes = append(attrBytes, attribute.PackAttribute(attribute.LocalPref(100))...)
// ... 10+ similar lines
return &message.Update{PathAttributes: attrBytes, NLRI: nlriBytes}
```

Issues:
1. **Verbose** - Same pattern repeated in 10+ functions
2. **Error-prone** - Easy to forget attributes or get ordering wrong
3. **No validation** - Missing required attributes not caught
4. **Scattered logic** - Each builder reimplements attribute handling

## Proposed Solution

Fluent builder pattern with automatic attribute ordering:

```go
update, err := message.NewUpdateBuilder().
    Origin(attribute.OriginIGP).
    ASPath([]uint32{65001, 65002}).
    NextHop(netip.MustParseAddr("192.168.1.1")).
    Communities(community.NoExport).
    NLRI(prefix1, prefix2).
    Build()
```

---

## Critical Design Decisions

### 1. Attribute Ordering (RFC 4271 Section 5)

RFC 4271: "Path attributes...SHOULD be ordered by the type code."

**Decision:** Builder stores typed attributes, sorts by type code at `Build()` time.

Order enforced:
| Priority | Type | Attribute |
|----------|------|-----------|
| 1 | 1 | ORIGIN |
| 2 | 2 | AS_PATH |
| 3 | 3 | NEXT_HOP (IPv4 unicast only) |
| 4 | 4 | MULTI_EXIT_DISC |
| 5 | 5 | LOCAL_PREF |
| 6 | 6 | ATOMIC_AGGREGATE |
| 7 | 7 | AGGREGATOR |
| 8 | 8 | COMMUNITIES |
| 9 | 9 | ORIGINATOR_ID |
| 10 | 10 | CLUSTER_LIST |
| 11 | 14 | MP_REACH_NLRI |
| 12 | 15 | MP_UNREACH_NLRI |
| 13 | 16 | EXTENDED_COMMUNITIES |
| 14 | 32 | LARGE_COMMUNITIES |

### 2. Required Attributes Validation

**Announcements require:**
- ORIGIN (mandatory)
- AS_PATH (mandatory)
- Next-hop (NEXT_HOP attr for IPv4 unicast, in MP_REACH for others)

**Decision:** `Build()` returns error if required attributes missing.

```go
update, err := builder.Build()
if errors.Is(err, message.ErrMissingOrigin) { ... }
```

### 3. Address Family Handling

**Problem:** IPv4 unicast vs MP families differ significantly:

| Family | NLRI Location | Next-Hop Location |
|--------|---------------|-------------------|
| IPv4 unicast | UPDATE.NLRI field | NEXT_HOP attribute (type 3) |
| IPv6 unicast | MP_REACH_NLRI | Inside MP_REACH_NLRI |
| L3VPN | MP_REACH_NLRI | Inside MP_REACH_NLRI (with RD) |
| FlowSpec | MP_REACH_NLRI | Empty (no next-hop) |

**Decision:** Builder auto-detects family from NLRI type and routes accordingly.

```go
// IPv4 - uses traditional NLRI field
builder.NLRI(ipv4Prefix).NextHop(v4Addr)

// IPv6 - builder internally creates MP_REACH_NLRI
builder.NLRI(ipv6Prefix).NextHop(v6Addr)
```

### 4. iBGP vs eBGP Context

**iBGP requires:**
- LOCAL_PREF (default 100)
- Empty AS_PATH (don't prepend)

**eBGP requires:**
- Prepend local AS to AS_PATH
- No LOCAL_PREF (unless explicitly set)

**Decision:** Builder accepts session context:

```go
builder.ForSession(session) // Sets iBGP/eBGP, local AS, 4-byte AS support
// OR
builder.IBGP(true).LocalAS(65001).ASN4(true)
```

### 5. Message Size & Chunking

Standard BGP max: 4096 bytes. Extended: 65535 bytes.

**Decision:** `Build()` returns slice for potential chunking:

```go
func (b *UpdateBuilder) Build() ([]*Update, error)
```

Single UPDATE returned in slice for API consistency. Multiple returned if NLRI exceeds size limit.

### 6. Withdrawal Support

**Problem:** Withdrawals differ by family:
- IPv4 unicast: `WithdrawnRoutes` field (no attributes)
- Others: `MP_UNREACH_NLRI` attribute

**Decision:** Single builder with announce/withdraw methods:

```go
// Announce
builder.NLRI(prefix).NextHop(nh).Build()

// Withdraw
builder.Withdraw(prefix).Build()
```

### 7. Buffer Management

**Options:**
1. Fresh allocation each time (current)
2. sync.Pool for buffer reuse
3. Pre-allocated growing buffer

**Decision:** Pre-allocate buffer based on negotiated max message size from OPEN.

During session establishment, both peers negotiate maximum message size:
- Standard BGP: 4096 bytes (RFC 4271)
- Extended Messages: up to 65535 bytes (RFC 8654)

Builder requires max size at construction - no throwaway allocations:

```go
// Constructor requires max size - we always know this by the time we build updates
func NewUpdateBuilder(maxSize int) *UpdateBuilder {
    return &UpdateBuilder{
        maxSize: maxSize,
        buf:     make([]byte, 0, maxSize),
    }
}

// Convenience constructor from session
func NewUpdateBuilderForSession(s *Session) *UpdateBuilder {
    return NewUpdateBuilder(s.NegotiatedMaxMessageSize())
}
```

This eliminates:
1. Append reallocations (capacity known upfront)
2. Throwaway default allocations (size required at construction)

Buffer reuse via `Reset()` avoids repeated allocations when building multiple updates for same session.

---

## Potential Problems & Mitigations

### P1: Breaking Existing Code

**Risk:** Refactoring breaks existing `build*Update` functions.

**Mitigation:**
1. Add builder alongside existing code
2. Add tests comparing builder output to existing function output (byte-for-byte)
3. Migrate functions one-by-one, keeping tests green
4. Remove old code only after full migration

### P2: MP_REACH_NLRI Complexity

**Risk:** Next-hop encoding varies wildly by family:
- IPv6: 16 bytes (global) or 32 bytes (global + link-local)
- VPN: 8 bytes RD + address
- FlowSpec: 0 bytes (no next-hop)

**Mitigation:** Delegate to existing `attribute.MPReachNLRI` which handles this. Builder just needs to detect family and route correctly.

### P3: AS_PATH Format (2-byte vs 4-byte)

**Risk:** AS_PATH encoding differs based on 4-byte AS capability negotiation.

**Mitigation:** Builder stores AS numbers as `[]uint32`, encodes based on `ASN4` flag set via session context.

### P4: Attribute Deduplication

**Risk:** Setting same attribute twice - overwrite or error?

**Decision:** Last-write-wins (overwrite). Allows:
```go
builder.Communities(c1, c2).Communities(c3) // Has c1, c2, c3
```

For replace semantics, add explicit method:
```go
builder.SetCommunities(c3) // Replaces all communities
```

### P5: Extended Communities Complexity

**Risk:** Extended communities have multiple sub-types (route-target, route-origin, etc.)

**Mitigation:** Accept typed extended communities:
```go
builder.ExtendedCommunities(
    extcomm.RouteTarget(65001, 100),
    extcomm.RouteOrigin(65001, 200),
)
```

### P6: NLRI Type Mixing

**Risk:** Caller adds IPv4 and IPv6 prefixes to same builder.

**Decision:** Error at `Build()` time if NLRI families don't match.

```go
builder.NLRI(v4prefix).NLRI(v6prefix).Build()
// Error: mixed address families
```

### P7: Atomic Aggregate + Aggregator Coupling

**Risk:** RFC 4271 says ATOMIC_AGGREGATE should accompany AGGREGATOR.

**Mitigation:** Warning log if AGGREGATOR set without ATOMIC_AGGREGATE, but don't error (implementations vary).

### P8: Path ID (Add-Path)

**Risk:** Add-Path extension adds path ID to NLRIs.

**Mitigation:** Builder accepts path ID:
```go
builder.NLRI(prefix).PathID(42)
// OR
builder.NLRIWithPathID(prefix, 42)
```

---

## API Design

### Core Builder

```go
package message

type UpdateBuilder struct {
    // Session context
    ibgp    bool
    localAS uint32
    asn4    bool
    maxSize int // 4096 or 65535

    // Attributes (stored typed, encoded at Build)
    origin           *attribute.Origin
    asPath           []uint32
    nextHop          netip.Addr
    med              *uint32
    localPref        *uint32
    atomicAggregate  bool
    aggregator       *attribute.Aggregator
    communities      []uint32
    extCommunities   []attribute.ExtendedCommunity
    largeCommunities []attribute.LargeCommunity
    originatorID     netip.Addr
    clusterList      []netip.Addr

    // NLRI
    nlris     []nlri.NLRI
    withdraws []nlri.NLRI

    // Raw attributes (for pass-through)
    rawAttrs []attribute.Attribute
}

func NewUpdateBuilder() *UpdateBuilder

// Session context
func (b *UpdateBuilder) ForSession(s *Session) *UpdateBuilder
func (b *UpdateBuilder) IBGP(ibgp bool) *UpdateBuilder
func (b *UpdateBuilder) LocalAS(as uint32) *UpdateBuilder
func (b *UpdateBuilder) ASN4(enabled bool) *UpdateBuilder
func (b *UpdateBuilder) MaxSize(bytes int) *UpdateBuilder

// Well-known mandatory
func (b *UpdateBuilder) Origin(o attribute.OriginType) *UpdateBuilder
func (b *UpdateBuilder) ASPath(path []uint32) *UpdateBuilder
func (b *UpdateBuilder) NextHop(addr netip.Addr) *UpdateBuilder

// Well-known discretionary
func (b *UpdateBuilder) LocalPref(pref uint32) *UpdateBuilder
func (b *UpdateBuilder) AtomicAggregate() *UpdateBuilder

// Optional transitive
func (b *UpdateBuilder) MED(med uint32) *UpdateBuilder
func (b *UpdateBuilder) Aggregator(as uint32, addr netip.Addr) *UpdateBuilder
func (b *UpdateBuilder) Communities(comms ...uint32) *UpdateBuilder
func (b *UpdateBuilder) ExtendedCommunities(comms ...attribute.ExtendedCommunity) *UpdateBuilder
func (b *UpdateBuilder) LargeCommunities(comms ...attribute.LargeCommunity) *UpdateBuilder

// Optional non-transitive
func (b *UpdateBuilder) OriginatorID(id netip.Addr) *UpdateBuilder
func (b *UpdateBuilder) ClusterList(ids ...netip.Addr) *UpdateBuilder

// NLRI
func (b *UpdateBuilder) NLRI(prefixes ...nlri.NLRI) *UpdateBuilder
func (b *UpdateBuilder) Withdraw(prefixes ...nlri.NLRI) *UpdateBuilder

// Raw attribute pass-through
func (b *UpdateBuilder) RawAttribute(attr attribute.Attribute) *UpdateBuilder

// Build
func (b *UpdateBuilder) Build() ([]*Update, error)
func (b *UpdateBuilder) MustBuild() []*Update // panics on error

// Reset for reuse
func (b *UpdateBuilder) Reset() *UpdateBuilder
```

### Usage Examples

```go
// Simple IPv4 announcement
updates, err := message.NewUpdateBuilder().
    Origin(attribute.OriginIGP).
    ASPath([]uint32{65001}).
    NextHop(netip.MustParseAddr("192.168.1.1")).
    NLRI(nlri.NewINET(family.IPv4Unicast, prefix, 0)).
    Build()

// iBGP with communities
updates, err := message.NewUpdateBuilder().
    IBGP(true).
    LocalAS(65001).
    Origin(attribute.OriginIGP).
    NextHop(nextHop).
    Communities(community.NoExport, community.NoPeer).
    LargeCommunities(lc1, lc2).
    NLRI(prefixes...).
    Build()

// IPv6 (auto-uses MP_REACH_NLRI)
updates, err := message.NewUpdateBuilder().
    Origin(attribute.OriginIGP).
    ASPath([]uint32{65001}).
    NextHop(netip.MustParseAddr("2001:db8::1")).
    NLRI(nlri.NewINET(family.IPv6Unicast, v6prefix, 0)).
    Build()

// Withdrawal
updates, err := message.NewUpdateBuilder().
    Withdraw(nlri.NewINET(family.IPv4Unicast, prefix, 0)).
    Build()
```

---

## Implementation Plan

### Phase 1: Core Builder (TDD)

1. **Test:** Builder with Origin, ASPath, NextHop, NLRI for IPv4
2. **Implement:** Basic builder structure
3. **Test:** Attribute ordering verification
4. **Implement:** Sort-by-type-code logic
5. **Test:** Required attribute validation
6. **Implement:** Error returns for missing attrs

### Phase 2: Full Attribute Support

1. **Test:** Each optional attribute type
2. **Implement:** All attribute setters
3. **Test:** Communities, extended communities, large communities
4. **Implement:** Community handling with dedup

### Phase 3: MP Family Support

1. **Test:** IPv6 unicast generates MP_REACH_NLRI
2. **Implement:** Family detection and MP routing
3. **Test:** L3VPN, FlowSpec families
4. **Implement:** Family-specific next-hop encoding

### Phase 4: Session Context

1. **Test:** iBGP defaults (LOCAL_PREF, empty AS_PATH)
2. **Implement:** Session context methods
3. **Test:** eBGP AS prepending
4. **Implement:** AS_PATH modification logic

### Phase 5: Message Chunking

1. **Test:** Large NLRI set splits into multiple UPDATEs
2. **Implement:** Size checking and chunking
3. **Test:** Extended message size support
4. **Implement:** MaxSize configuration

### Phase 6: Migration

1. Refactor `buildStaticRouteUpdate` to use builder
2. Verify byte-for-byte output match
3. Refactor remaining `build*Update` functions
4. Remove duplicate code

---

## File Locations

| File | Purpose |
|------|---------|
| `pkg/bgp/message/builder.go` | UpdateBuilder implementation |
| `pkg/bgp/message/builder_test.go` | Unit tests |
| `pkg/bgp/message/errors.go` | ErrMissingOrigin, etc. (if not exists) |

---

## Success Criteria

1. ✅ All existing `build*Update` tests pass with builder
2. ✅ Byte-for-byte output compatibility verified
3. ✅ Attribute ordering always correct (RFC 4271)
4. ✅ Required attribute validation works
5. ✅ IPv4 and IPv6 families handled correctly
6. ✅ `make test && make lint` passes
7. ✅ Code reduction: 10+ functions → single builder

---

## Open Questions

1. **Pool buffers?** - Defer until profiling shows need
2. **Parallel builds?** - Builder is not goroutine-safe (by design, create new per goroutine)
3. **Withdrawal + Announce same builder?** - Yes, single UPDATE can have both

---

## References

- RFC 4271 Section 4.3 - UPDATE Message Format
- RFC 4271 Section 5 - Path Attributes
- RFC 4760 - Multiprotocol Extensions (MP_REACH_NLRI, MP_UNREACH_NLRI)
- RFC 7606 - Error Handling
