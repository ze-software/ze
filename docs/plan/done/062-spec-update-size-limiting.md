# Spec: UPDATE Size Limiting

## Task

Implement UPDATE message size limiting with two complementary approaches:
- **Option A:** Size-aware builders (`BuildGroupedUnicastWithLimit`) - proactive limiting
- **Option B:** Post-build split utility (`SplitUpdate`) - reactive splitting

Both are needed: Option A for efficient building, Option B for forwarding/replay paths.

## Required Reading (MUST complete before implementation)

- [ ] `.claude/zebgp/UPDATE_BUILDING.md` - Build vs Forward paths, *Params design
- [ ] `.claude/zebgp/ENCODING_CONTEXT.md` - Context-dependent encoding, zero-copy
- [ ] `.claude/zebgp/wire/MESSAGES.md` - Max sizes (4096/65535), header format
- [ ] `.claude/zebgp/wire/NLRI.md` - NLRI formats, MP_REACH_NLRI structure

**Key insights from docs:**
- **Two paths:** Build path (low volume, local origination) vs Forward path (high volume, zero-copy)
- **Max sizes:** 4096 bytes standard (RFC 4271), 65535 bytes extended (RFC 8654)
- **UPDATE overhead:** Header(19) + WithdrawnLen(2) + AttrsLen(2) = 23 bytes minimum
- **MP_REACH_NLRI:** For IPv6/VPN/EVPN, NLRI is *inside* the attribute, not UPDATE.NLRI field
- **ChunkNLRI limitation:** Existing function is IPv4-only, cannot handle MP families

## Problem

UPDATE messages can exceed max size (4096 standard, 65535 extended) when:
1. Many NLRIs with same attributes are grouped
2. Large attributes (communities, AS_PATH) combined with multiple NLRIs
3. Extended Message peer's UPDATE forwarded to non-Extended peer

Current state:
- `ForwardUpdate` has size check + ad-hoc split logic
- `sendInitialRoutes` **skips** oversized (data loss!)
- `BuildGroupedUnicast` packs all NLRIs without limit
- No consistent enforcement at RIB→send boundary

## Goal

Size limiting enforced consistently: when routes leave adj-rib-out for wire.

## Files to Modify

| File | Changes |
|------|---------|
| `pkg/bgp/message/update_split.go` | **New:** SplitUpdate, SplitMPReachNLRI |
| `pkg/bgp/message/update_build.go` | Add BuildGroupedUnicastWithLimit, helpers |
| `pkg/bgp/message/update.go` | Document ChunkNLRI limitations |
| `pkg/reactor/peer.go` | Replace skip with split in sendInitialRoutes |
| `pkg/reactor/reactor.go` | Use SplitUpdate in ForwardUpdate |

## Current State

**Existing infrastructure:**
- `ChunkNLRI(nlri []byte, maxSize int) [][]byte` at `update.go:182`
  - ✅ IPv4 unicast (parses prefix length byte)
  - ✅ Variable prefix lengths
  - ✅ Best-effort for oversized single prefix (returns it, doesn't error)
  - ❌ No Add-Path support (no path-id parsing)
  - ❌ No labeled unicast (no label stack parsing)
  - ❌ No VPN (no RD parsing)
  - ❌ Cannot split MP_REACH_NLRI (NLRI inside attribute)

**Tests:** Existing ChunkNLRI tests pass
**Last working:** ForwardUpdate has partial implementation

## Design

### Option A: Size-Aware Builder

Add `maxSize` parameter to builders. Returns multiple UPDATEs if needed.

```go
// BuildGroupedUnicastWithLimit builds UPDATEs respecting size limit.
// Returns slice of UPDATEs, each <= maxSize bytes.
// Returns error if attributes alone exceed maxSize.
//
// RFC 4271 Section 4.3: Each UPDATE is self-contained with full attributes.
func (ub *UpdateBuilder) BuildGroupedUnicastWithLimit(
    routes []UnicastParams,
    maxSize int,
) ([]*Update, error)
```

**Pros:** Single point of enforcement, efficient (no re-parsing)
**Cons:** Changes builder interface, caller must handle slice

### Option B: Post-Build Split Function

Split any UPDATE after construction.

```go
// SplitUpdate splits an oversized UPDATE into valid chunks.
// Reuses attributes, splits NLRI/Withdrawn across messages.
// Returns error if single NLRI or attributes exceed maxSize.
//
// RFC 4271 Section 4.3: Each UPDATE is self-contained with full attributes.
// RFC 4760 Section 3: MP_REACH_NLRI carries NLRI for non-IPv4 families.
func SplitUpdate(u *Update, maxSize int) ([]*Update, error)
```

**Pros:** Works with existing code, good for forwarding path
**Cons:** May require attribute re-parsing for MP_REACH_NLRI

### Why Both?

| Use Case | Best Option |
|----------|-------------|
| Building grouped routes from config | Option A (efficient) |
| Forwarding received UPDATE | Option B (already have Update) |
| Replaying adj-rib-out | Option B (routes → Update → split) |
| Static route batching | Option A (building fresh) |

## Implementation Steps

**Phase Parallelization:**
- Phase 1 (SplitUpdate IPv4) and Phase 3 (BuildWithLimit) are independent - can run in parallel
- Phase 2 (MP_REACH_NLRI) extends Phase 1
- Phase 4 (Integration) can start after Phase 1 for IPv4-only paths, extend after Phase 2

### Phase 1: SplitUpdate for IPv4 (Option B)

**TDD Cycle:**

1. **Write test** `TestSplitUpdateIPv4NLRI`
2. **See FAIL** - SplitUpdate doesn't exist
3. **Implement** `SplitUpdate()` for IPv4 (uses existing ChunkNLRI)
4. **See PASS**
5. `make test && make lint`

```go
// Location: pkg/bgp/message/update_split.go

// SplitUpdate splits an UPDATE into chunks respecting maxSize.
//
// For IPv4 announcements: attributes preserved, NLRI split via ChunkNLRI.
// For IPv4 withdrawals: withdrawn routes split via ChunkNLRI.
// For MP families: delegates to SplitMPReachNLRI (Phase 2).
// Mixed updates: split into separate announce/withdraw UPDATEs.
//
// WIRE CACHE PRESERVATION:
// - u.PathAttributes is raw wire bytes - reused directly in all chunks (zero-copy)
// - For MP_REACH_NLRI: must rebuild attribute (NLRIs inside), but other attrs preserved
//
// Returns error if:
// - Attributes alone exceed maxSize (ErrAttributesTooLarge)
// - Single NLRI exceeds available space (ErrNLRITooLarge)
//
// Note: maxSize is always 4096 or 65535 from MaxMessageLength() - no validation needed.
//
// RFC 4271 Section 4.3: Each UPDATE is self-contained with full attributes.
func SplitUpdate(u *Update, maxSize int) ([]*Update, error) {
    // Calculate fixed overhead: header(19) + withdrawn_len(2) + attr_len(2)
    overhead := HeaderLen + 4
    attrSize := len(u.PathAttributes)

    // Check if attributes alone exceed limit
    if overhead + attrSize > maxSize {
        return nil, ErrAttributesTooLarge
    }

    // Available space for NLRI per message
    nlriSpace := maxSize - overhead - attrSize

    // Handle withdrawals (no attributes needed)
    // Handle announcements (need attributes - reuse u.PathAttributes directly)
    // Handle mixed (separate into two sets of UPDATEs)
    // ...
}

var (
    ErrAttributesTooLarge = errors.New("attributes exceed max message size")
    ErrNLRITooLarge       = errors.New("single NLRI exceeds available space")
)
```

### Phase 2: MP_REACH_NLRI Splitting

**TDD Cycle:**

1. **Write test** `TestSplitUpdateMPReachNLRI`
2. **See FAIL** - MP splitting not implemented
3. **Implement** `SplitMPReachNLRI()`
4. **See PASS**
5. `make test && make lint`

```go
// SplitMPReachNLRI splits an UPDATE containing MP_REACH_NLRI.
//
// MP_REACH_NLRI structure (RFC 4760 Section 3):
//   +---------------------------------------------------------+
//   | AFI (2 octets)                                          |
//   +---------------------------------------------------------+
//   | SAFI (1 octet)                                          |
//   +---------------------------------------------------------+
//   | Length of Next Hop (1 octet)                            |
//   +---------------------------------------------------------+
//   | Next Hop (variable)                                     |
//   +---------------------------------------------------------+
//   | Reserved (1 octet, must be 0)                           |
//   +---------------------------------------------------------+
//   | NLRI (variable)                                         |
//   +---------------------------------------------------------+
//
// Splitting requires:
// 1. Parse MP_REACH_NLRI to extract header (AFI/SAFI/NH) and NLRI list
// 2. Chunk NLRIs respecting maxSize
// 3. Rebuild MP_REACH_NLRI attributes with chunked NLRIs
// 4. Create separate UPDATEs with other attributes preserved
//
// RFC 4760 Section 3: MP_REACH_NLRI is non-transitive, cannot be partial.
func SplitMPReachNLRI(u *Update, maxSize int) ([]*Update, error) {
    // Find MP_REACH_NLRI in PathAttributes
    // Parse header (AFI, SAFI, NH length, NH, reserved)
    // Extract NLRI bytes
    // Chunk using family-aware chunker
    // Rebuild attributes for each chunk
    // ...
}

// ChunkMPNLRI splits MP family NLRIs respecting maxSize.
// Unlike ChunkNLRI, handles:
// - Add-Path (4-byte path-id prefix)
// - Labeled unicast (3-byte label stack per label)
// - VPN (8-byte RD prefix)
//
// RFC 4760, RFC 7911 (Add-Path), RFC 8277 (Labeled), RFC 4364 (VPN)
func ChunkMPNLRI(nlri []byte, family nlri.Family, addPath bool, maxSize int) ([][]byte, error)
```

### Phase 3: Size-Aware Builder (Option A)

**TDD Cycle:**

1. **Write test** `TestBuildGroupedUnicastWithLimit`
2. **See FAIL** - method doesn't exist
3. **Implement** `BuildGroupedUnicastWithLimit()`
4. **See PASS**
5. `make test && make lint`

```go
// BuildGroupedUnicastWithLimit builds multiple UPDATEs if needed.
//
// All routes MUST have identical attributes (caller's responsibility).
// Returns error if attributes exceed maxSize.
//
// Design: Build-and-tally approach - pack incrementally, flush when full.
// This avoids wasteful pre-calculation of sizes.
//
// RFC 4271 Section 4.3: Multiple UPDATEs may advertise same attributes.
func (ub *UpdateBuilder) BuildGroupedUnicastWithLimit(
    routes []UnicastParams,
    maxSize int,
) ([]*Update, error) {
    if len(routes) == 0 {
        return nil, nil
    }

    // Build attributes once (shared across all routes in batch)
    attrBytes := ub.packAttributes(routes[0])
    overhead := HeaderLen + 4 + len(attrBytes)

    if overhead > maxSize {
        return nil, ErrAttributesTooLarge
    }

    nlriSpace := maxSize - overhead

    var updates []*Update
    var nlriBytes []byte

    for _, r := range routes {
        // Pack NLRI directly - no separate size calculation
        nlri := ub.packNLRI(r)

        // Single NLRI too large?
        if len(nlri) > nlriSpace {
            return nil, fmt.Errorf("%w: %d bytes, available %d", ErrNLRITooLarge, len(nlri), nlriSpace)
        }

        // Would overflow? Flush current batch
        if len(nlriBytes) + len(nlri) > nlriSpace && len(nlriBytes) > 0 {
            updates = append(updates, &Update{
                PathAttributes: attrBytes,
                NLRI:           nlriBytes,
            })
            nlriBytes = nil
        }
        nlriBytes = append(nlriBytes, nlri...)
    }

    // Flush remainder
    if len(nlriBytes) > 0 {
        updates = append(updates, &Update{
            PathAttributes: attrBytes,
            NLRI:           nlriBytes,
        })
    }
    return updates, nil
}

// packAttributes builds attribute wire bytes for a route.
// Called once per batch (all routes share same attributes).
func (ub *UpdateBuilder) packAttributes(r UnicastParams) []byte

// packNLRI builds NLRI wire bytes for a single route.
func (ub *UpdateBuilder) packNLRI(r UnicastParams) []byte
```

### Phase 4: Integration

**TDD Cycle:**

1. **Write integration test** for sendInitialRoutes with oversized routes
2. **See FAIL** - still skipping
3. **Replace skip with split** in peer.go
4. **See PASS**
5. `make test && make lint && make functional`

**Integration points:**

| Location | Current | After |
|----------|---------|-------|
| `peer.go:sendInitialRoutes` adj-rib-out replay | Skip oversized | Split and send all |
| `peer.go:sendInitialRoutes` pending routes | Skip oversized | Split and send all |
| `peer.go:opQueue` PeerOpAnnounce | Skip oversized | Split and send all |
| `peer.go:sendStaticRoutes` grouped | No check | Use BuildGroupedUnicastWithLimit |
| `reactor.go:ForwardUpdate` | Custom split | Use SplitUpdate |

## Test Plan

```go
// === Phase 1: SplitUpdate IPv4 ===

// --- Basic Functionality ---

// TestSplitUpdate_SmallFits verifies small UPDATE passes through.
// VALIDATES: UPDATE < maxSize returns single UPDATE unchanged.
// PREVENTS: Unnecessary splitting of small messages.
func TestSplitUpdate_SmallFits(t *testing.T)

// TestSplitUpdate_ExactFit verifies boundary condition.
// VALIDATES: UPDATE == maxSize returns single UPDATE (no split).
// PREVENTS: Off-by-one splitting at exact boundary.
func TestSplitUpdate_ExactFit(t *testing.T)

// TestSplitUpdate_NLRIOverflow verifies NLRI splitting.
// VALIDATES: Large NLRI split into N chunks, each <= maxSize.
// PREVENTS: Oversized UPDATE sent to peer.
func TestSplitUpdate_NLRIOverflow(t *testing.T)

// TestSplitUpdate_WithdrawnOverflow verifies withdrawal splitting.
// VALIDATES: Large withdrawal split into N chunks (no attributes).
// PREVENTS: Oversized withdrawal message.
func TestSplitUpdate_WithdrawnOverflow(t *testing.T)

// TestSplitUpdate_MixedSeparates verifies mixed UPDATE handling.
// VALIDATES: Announce + Withdraw split into separate UPDATE sets.
// PREVENTS: Invalid mixed splitting losing routes.
func TestSplitUpdate_MixedSeparates(t *testing.T)

// --- Edge Cases ---

// TestSplitUpdate_EndOfRIB verifies EoR passthrough.
// VALIDATES: Empty UPDATE (End-of-RIB) returns single unchanged UPDATE.
// PREVENTS: EoR marker corruption.
func TestSplitUpdate_EndOfRIB(t *testing.T)

// TestSplitUpdate_WithdrawalOnly verifies withdrawal-only structure.
// VALIDATES: Withdrawal-only UPDATE has empty PathAttributes.
// PREVENTS: Adding spurious attributes to withdrawals.
func TestSplitUpdate_WithdrawalOnly(t *testing.T)

// TestSplitUpdate_OneByteOver verifies minimal overflow.
// VALIDATES: UPDATE at maxSize+1 splits into exactly 2 chunks.
// PREVENTS: Off-by-one non-splitting.
func TestSplitUpdate_OneByteOver(t *testing.T)

// --- Wire Cache Preservation ---

// TestSplitUpdate_AttributesBytesPreserved verifies zero-copy.
// VALIDATES: All chunks share same PathAttributes slice (pointer equality).
// PREVENTS: Unnecessary attribute re-serialization.
func TestSplitUpdate_AttributesBytesPreserved(t *testing.T)

// TestSplitUpdate_AttributesContentIdentical verifies attribute integrity.
// VALIDATES: Each chunk has byte-identical attributes.
// PREVENTS: Malformed UPDATE with missing/corrupted attributes.
func TestSplitUpdate_AttributesContentIdentical(t *testing.T)

// --- Error Conditions ---

// TestSplitUpdate_AttributesTooLarge verifies error on huge attributes.
// VALIDATES: ErrAttributesTooLarge returned when attrs > maxSize.
// PREVENTS: Panic or invalid split attempt.
func TestSplitUpdate_AttributesTooLarge(t *testing.T)

// TestSplitUpdate_SingleNLRITooLarge verifies error on huge single NLRI.
// VALIDATES: ErrNLRITooLarge returned when single NLRI > available space.
// PREVENTS: Silent data loss or infinite loop.
func TestSplitUpdate_SingleNLRITooLarge(t *testing.T)

// --- Chunk Verification ---

// TestSplitUpdate_AllChunksValid verifies chunk structure.
// VALIDATES: Each chunk is a valid UPDATE (correct length fields, parseable).
// PREVENTS: Malformed UPDATE messages.
func TestSplitUpdate_AllChunksValid(t *testing.T)

// TestSplitUpdate_NLRICountPreserved verifies no data loss.
// VALIDATES: Sum of NLRIs across chunks equals original NLRI count.
// PREVENTS: Route loss during splitting.
func TestSplitUpdate_NLRICountPreserved(t *testing.T)

// === Phase 2: MP_REACH_NLRI Splitting ===

// --- MP_REACH_NLRI ---

// TestSplitUpdate_MPReachIPv6 verifies IPv6 unicast splitting.
// VALIDATES: MP_REACH_NLRI parsed, NLRIs chunked, attribute rebuilt.
// PREVENTS: Malformed MP_REACH_NLRI or lost routes.
func TestSplitUpdate_MPReachIPv6(t *testing.T)

// TestSplitUpdate_MPReachVPN verifies VPN splitting.
// VALIDATES: VPN NLRIs (with RD) split correctly.
// PREVENTS: Incorrect RD handling during split.
func TestSplitUpdate_MPReachVPN(t *testing.T)

// TestSplitUpdate_MPReachAddPath verifies Add-Path splitting.
// VALIDATES: Path-ID preserved in each chunk.
// PREVENTS: Path-ID loss during split.
func TestSplitUpdate_MPReachAddPath(t *testing.T)

// TestSplitUpdate_MPReachLabeled verifies labeled unicast splitting.
// VALIDATES: Label stack preserved in each chunk.
// PREVENTS: Label corruption during split.
func TestSplitUpdate_MPReachLabeled(t *testing.T)

// TestSplitUpdate_MPOtherAttrsPreserved verifies non-MP attributes.
// VALIDATES: ORIGIN, AS_PATH, etc. preserved byte-for-byte.
// PREVENTS: Corruption of non-MP attributes when rebuilding MP_REACH.
func TestSplitUpdate_MPOtherAttrsPreserved(t *testing.T)

// --- MP_UNREACH_NLRI ---

// TestSplitUpdate_MPUnreachIPv6 verifies IPv6 withdrawal splitting.
// VALIDATES: MP_UNREACH_NLRI split correctly (no next-hop).
// PREVENTS: Malformed MP_UNREACH_NLRI.
func TestSplitUpdate_MPUnreachIPv6(t *testing.T)

// --- ChunkMPNLRI ---

// TestChunkMPNLRI_IPv6 verifies raw IPv6 NLRI chunking.
// VALIDATES: Chunks respect maxSize, split at prefix boundaries.
// PREVENTS: Prefix corruption from mid-prefix split.
func TestChunkMPNLRI_IPv6(t *testing.T)

// TestChunkMPNLRI_VPN verifies VPN NLRI chunking.
// VALIDATES: RD + prefix kept together.
// PREVENTS: RD separated from prefix.
func TestChunkMPNLRI_VPN(t *testing.T)

// TestChunkMPNLRI_AddPath verifies Add-Path NLRI chunking.
// VALIDATES: Path-ID + prefix kept together.
// PREVENTS: Path-ID separated from prefix.
func TestChunkMPNLRI_AddPath(t *testing.T)

// TestChunkMPNLRI_Labeled verifies labeled unicast chunking.
// VALIDATES: Labels + prefix kept together.
// PREVENTS: Label stack corruption.
func TestChunkMPNLRI_Labeled(t *testing.T)

// === Phase 3: Size-Aware Builder ===

// --- Basic Functionality ---

// TestBuildWithLimit_Empty verifies empty input.
// VALIDATES: Empty routes returns nil, nil.
// PREVENTS: Panic on empty input.
func TestBuildWithLimit_Empty(t *testing.T)

// TestBuildWithLimit_SingleRoute verifies single route.
// VALIDATES: Single route returns single UPDATE.
// PREVENTS: Unnecessary splitting.
func TestBuildWithLimit_SingleRoute(t *testing.T)

// TestBuildWithLimit_AllFit verifies multiple routes fitting.
// VALIDATES: N routes that fit return single UPDATE.
// PREVENTS: Unnecessary splitting.
func TestBuildWithLimit_AllFit(t *testing.T)

// TestBuildWithLimit_Overflow verifies route batching.
// VALIDATES: N routes overflow into M UPDATEs.
// PREVENTS: Single oversized UPDATE from builder.
func TestBuildWithLimit_Overflow(t *testing.T)

// --- Boundary Conditions ---

// TestBuildWithLimit_ExactFit verifies exact boundary.
// VALIDATES: Routes exactly filling maxSize return single UPDATE.
// PREVENTS: Off-by-one extra UPDATE.
func TestBuildWithLimit_ExactFit(t *testing.T)

// TestBuildWithLimit_OneByteOver verifies minimal overflow.
// VALIDATES: Routes 1 byte over maxSize return 2 UPDATEs.
// PREVENTS: Off-by-one single UPDATE.
func TestBuildWithLimit_OneByteOver(t *testing.T)

// --- Error Conditions ---

// TestBuildWithLimit_AttrsTooBig verifies attribute overflow.
// VALIDATES: ErrAttributesTooLarge when attrs > maxSize.
// PREVENTS: Panic on huge attributes.
func TestBuildWithLimit_AttrsTooBig(t *testing.T)

// TestBuildWithLimit_SingleRouteTooLarge verifies single route overflow.
// VALIDATES: ErrNLRITooLarge when one route > available space.
// PREVENTS: Silent truncation.
func TestBuildWithLimit_SingleRouteTooLarge(t *testing.T)

// === Phase 4: Integration ===

// TestSendInitialRoutes_SplitsOversized verifies adj-rib-out replay.
// VALIDATES: Oversized routes split and sent, not skipped.
// PREVENTS: Data loss on peer reconnect.
func TestSendInitialRoutes_SplitsOversized(t *testing.T)

// TestForwardUpdate_ExtendedToStandard verifies capability mismatch.
// VALIDATES: 65535-byte UPDATE split for 4096-byte peer.
// PREVENTS: Oversized message rejection by non-Extended peer.
func TestForwardUpdate_ExtendedToStandard(t *testing.T)

// TestForwardUpdate_UsesSplitUpdate verifies refactoring.
// VALIDATES: ForwardUpdate uses SplitUpdate instead of custom logic.
// PREVENTS: Duplicate splitting implementations.
func TestForwardUpdate_UsesSplitUpdate(t *testing.T)

// TestOpQueue_SplitsOversized verifies PeerOpAnnounce handling.
// VALIDATES: Large PeerOpAnnounce routes split before send.
// PREVENTS: Oversized message in op queue processing.
func TestOpQueue_SplitsOversized(t *testing.T)
```

## Checklist

### Phase 1: SplitUpdate IPv4
- [ ] `pkg/bgp/message/update_split.go` created
- [ ] `ErrAttributesTooLarge`, `ErrNLRITooLarge` errors defined
- [ ] Tests written and FAIL:
  - [ ] `TestSplitUpdate_SmallFits`
  - [ ] `TestSplitUpdate_ExactFit`
  - [ ] `TestSplitUpdate_NLRIOverflow`
  - [ ] `TestSplitUpdate_WithdrawnOverflow`
  - [ ] `TestSplitUpdate_MixedSeparates`
  - [ ] `TestSplitUpdate_EndOfRIB`
  - [ ] `TestSplitUpdate_WithdrawalOnly`
  - [ ] `TestSplitUpdate_OneByteOver`
  - [ ] `TestSplitUpdate_AttributesBytesPreserved`
  - [ ] `TestSplitUpdate_AttributesTooLarge`
  - [ ] `TestSplitUpdate_SingleNLRITooLarge`
  - [ ] `TestSplitUpdate_AllChunksValid`
  - [ ] `TestSplitUpdate_NLRICountPreserved`
- [ ] `SplitUpdate()` implemented
- [ ] All Phase 1 tests PASS
- [ ] `make test && make lint` passes

### Phase 2: MP_REACH_NLRI Splitting
- [ ] Tests written and FAIL:
  - [ ] `TestSplitUpdate_MPReachIPv6`
  - [ ] `TestSplitUpdate_MPReachVPN`
  - [ ] `TestSplitUpdate_MPReachAddPath`
  - [ ] `TestSplitUpdate_MPReachLabeled`
  - [ ] `TestSplitUpdate_MPOtherAttrsPreserved`
  - [ ] `TestSplitUpdate_MPUnreachIPv6`
  - [ ] `TestChunkMPNLRI_IPv6`
  - [ ] `TestChunkMPNLRI_VPN`
  - [ ] `TestChunkMPNLRI_AddPath`
  - [ ] `TestChunkMPNLRI_Labeled`
- [ ] `SplitMPReachNLRI()` implemented
- [ ] `SplitMPUnreachNLRI()` implemented
- [ ] `ChunkMPNLRI()` implemented
- [ ] All Phase 2 tests PASS
- [ ] `make test && make lint` passes

### Phase 3: Size-Aware Builder (can run parallel with Phase 1)
- [ ] Tests written and FAIL:
  - [ ] `TestBuildWithLimit_Empty`
  - [ ] `TestBuildWithLimit_SingleRoute`
  - [ ] `TestBuildWithLimit_AllFit`
  - [ ] `TestBuildWithLimit_Overflow`
  - [ ] `TestBuildWithLimit_ExactFit`
  - [ ] `TestBuildWithLimit_OneByteOver`
  - [ ] `TestBuildWithLimit_AttrsTooBig`
  - [ ] `TestBuildWithLimit_SingleRouteTooLarge`
- [ ] `packAttributes()` helper added
- [ ] `packNLRI()` helper added
- [ ] `BuildGroupedUnicastWithLimit()` implemented
- [ ] All Phase 3 tests PASS
- [ ] `make test && make lint` passes

### Phase 4: Integration (start after Phase 1, extend after Phase 2)
- [ ] Tests written and FAIL:
  - [ ] `TestSendInitialRoutes_SplitsOversized`
  - [ ] `TestForwardUpdate_ExtendedToStandard`
  - [ ] `TestForwardUpdate_UsesSplitUpdate`
  - [ ] `TestOpQueue_SplitsOversized`
- [ ] `sendInitialRoutes` uses SplitUpdate instead of skip
- [ ] `ForwardUpdate` uses SplitUpdate (replace custom logic)
- [ ] `sendStaticRoutes` uses BuildGroupedUnicastWithLimit
- [ ] opQueue PeerOpAnnounce uses SplitUpdate
- [ ] All Phase 4 tests PASS
- [ ] `make test && make lint && make functional` passes

### Documentation
- [ ] RFC comments added (4271, 4760, 8654, 7911, 8277, 4364)
- [ ] ChunkNLRI limitations documented in update.go
- [ ] `.claude/zebgp/UPDATE_BUILDING.md` updated with split info

## RFC References

| RFC | Section | Relevance |
|-----|---------|-----------|
| RFC 4271 | 4.3 | UPDATE format, max 4096 bytes |
| RFC 4760 | 3 | MP_REACH_NLRI/MP_UNREACH_NLRI format |
| RFC 8654 | 2 | Extended Message capability, max 65535 bytes |
| RFC 7911 | 3 | ADD-PATH NLRI format (path-id prefix) |
| RFC 8277 | 2 | Labeled unicast NLRI format |
| RFC 4364 | 4 | VPN NLRI format (RD prefix) |

## Dependencies

**Existing:**
- `ChunkNLRI()` at `pkg/bgp/message/update.go:182` - IPv4 NLRI splitting
- `MaxMessageLength()` - returns 4096 or 65535 based on Extended Message
- `HeaderLen` constant (19)

**New (to implement):**
- `ChunkMPNLRI()` - family-aware NLRI splitting
- `calculateAttributeSize()` - wire size calculation
- `calculateNLRISize()` - wire size calculation

## Known Limitations

1. **Attribute-only UPDATE unsplittable:** If attributes alone exceed maxSize, cannot split (error returned). This is pathological (4KB of communities/AS_PATH) but theoretically possible.

2. **Single NLRI too large:** VPN prefix with long RD + labels + /128 = ~30 bytes. With 4KB attributes, single NLRI could exceed 4096. Error returned.

3. **MP_UNREACH_NLRI:** Similar to MP_REACH_NLRI but simpler (no next-hop). Implementation should handle both.

4. **FlowSpec:** FlowSpec NLRIs can be large (complex match rules). May need special handling.

5. **Ordering:** Splitting one UPDATE into N chunks doesn't guarantee ordering. BGP doesn't require it, but some implementations may depend on it.

---

**Created:** 2025-01-01
**Updated:** 2026-01-01 - Restructured to /prep format, added Option A+B, MP_REACH_NLRI, error handling
