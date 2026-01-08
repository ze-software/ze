# Spec: Wire UPDATE Splitting Bounds Safety

## Task
Prevent buffer overflow when forwarding wire UPDATEs to peers with smaller buffers.

## Scope: Wire Path Only

| Path | Use Case | Code | This Spec |
|------|----------|------|-----------|
| **Wire** | Forward received UPDATE to peer | `SplitUpdateWithAddPath` | ✅ YES |
| **API** | Generate UPDATE from API input | `UpdateBuilder.Build*()` | ❌ NO (separate spec) |

**Call site**: `ForwardUpdate()` in `pkg/reactor/reactor.go` calls split before sending to peers.

## Problem
- Received UPDATE from Extended Message peer (65535) may need forwarding to standard peer (4096)
- Current approach pre-calculates exact sizes - works but complex
- Want simpler approach: check if room for one more NLRI after each write

## Design: Check-After-Write

### Core Principle
After writing each NLRI, check if another could fit. Return subslices of original buffer.

### Algorithm

Note: `maxSize` parameter is already the available space for NLRIs (caller subtracts header+attrs).

```
offset = 0
chunk_start = 0

while offset < len(nlri_data):
    nlri_size = size_of_nlri(nlri_data[offset:])

    # Single NLRI too large for ANY message?
    if nlri_size > maxSize:
        return error("NLRI too large for message size")

    # Would this NLRI overflow current chunk?
    chunk_size = offset - chunk_start
    if chunk_size + nlri_size > maxSize:
        # Emit current chunk as subslice
        emit(nlri_data[chunk_start:offset])
        chunk_start = offset

    offset += nlri_size

# Emit final chunk
if chunk_start < len(nlri_data):
    emit(nlri_data[chunk_start:])
```

### Memory Model
- `emit()` returns `nlri_data[start:end]` - subslice of original buffer
- No `append()` in hot path - avoids copying bytes
- Allocation acceptable when building output UPDATE (pool memory)
- Buffer lifetime: owned until send completes, then returned to pool

### Size Calculation for Variable-Length NLRIs

For families with variable-length components (labels), calculate actual size:

```go
func labeledNLRISize(data []byte) (int, error) {
    if len(data) < 1 {
        return 0, ErrNLRIMalformed
    }
    totalBits := int(data[0])
    totalBytes := (totalBits + 7) / 8
    return 1 + totalBytes, nil
}
```

The length byte encodes total bits (labels + prefix), so size is always calculable from wire format.

### Family-Specific Considerations

| Family | Max NLRI Size | Notes |
|--------|---------------|-------|
| FlowSpec | 4095 bytes | RFC 5575: length encoding caps at 4095. CAN split. |
| BGP-LS | 65535 bytes | RFC 7752: 2-byte length field. Single NLRI can exceed 4096. |

**FlowSpec:** Individual NLRIs bounded at 4095 bytes. Splitting works normally.
RFC doesn't address oversized rules - implementations shouldn't generate them.

**BGP-LS:** Single NLRI can exceed standard message size (4096). If NLRI > available space → return error. Cannot split a single oversized NLRI.

### IPv4 NLRI Field (not MP)

Same logic applies to `Update.NLRI` and `Update.WithdrawnRoutes` fields:
- These are IPv4 unicast only
- Size: 1 (length byte) + ceil(prefix_bits/8)
- With AddPath: +4 bytes for path ID

## Required Reading

### Architecture Docs
- [x] `.claude/zebgp/wire/NLRI.md` - NLRI formats and wire encoding for all families
- [x] `.claude/zebgp/UPDATE_BUILDING.md` - UPDATE construction and splitting context

### RFC Summaries (MUST for protocol work)
- [x] RFC 4271 Section 4.3 - UPDATE message format, max 4096 bytes
- [x] RFC 8654 - Extended Message capability raises max to 65535 bytes
- [x] RFC 4760 - MP_REACH_NLRI / MP_UNREACH_NLRI wire format
- [x] RFC 7911 - ADD-PATH adds 4-byte path-id before each NLRI
- [x] RFC 8277 - Labeled unicast: length byte includes label bits
- [x] RFC 4364 - VPN NLRI: labels + 8-byte RD + prefix
- [x] RFC 7432 - EVPN NLRI: [route-type:1][length:1][payload]
- [x] RFC 5575 - FlowSpec: max 4095 bytes per NLRI (CAN split)
- [x] RFC 7752 - BGP-LS: 2-byte length field, single NLRI can exceed 4096

**Key insights:**
- Extended Message (RFC 8654) creates need to split when forwarding to standard peers
- Each NLRI family has different wire format - size functions must be family-aware
- BGP-LS is the only family where a single NLRI can exceed standard message size
- FlowSpec length encoding caps at 4095 bytes, so splitting always works

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates |
|------|------|-----------|
| `TestSplitMPNLRI_Subslice` | `pkg/bgp/message/chunk_mp_nlri_test.go` | Returns subslices of original buffer |
| `TestSplitMPNLRI_NoAllocHotPath` | `pkg/bgp/message/chunk_mp_nlri_test.go` | `testing.AllocsPerRun` verifies no alloc in split loop |
| `TestSplitMPNLRI_BoundaryExact` | `pkg/bgp/message/chunk_mp_nlri_test.go` | Exactly maxSize bytes handled |
| `TestSplitMPNLRI_SingleNLRIFillsBuffer` | `pkg/bgp/message/chunk_mp_nlri_test.go` | Single NLRI at exact limit |
| `TestSplitUpdate_CheckAfterWrite` | `pkg/bgp/message/update_split_test.go` | Splits when next NLRI won't fit |
| `TestSplitUpdate_IPv4Field` | `pkg/bgp/message/update_split_test.go` | IPv4 NLRI field handled |
| `TestSplitUpdate_FlowSpec_Split` | `pkg/bgp/message/update_split_test.go` | FlowSpec splits normally |
| `TestSplitUpdate_BGPLS_TooLarge` | `pkg/bgp/message/update_split_test.go` | BGP-LS single NLRI > maxSize → error |
| `TestSplitUpdate_AttributesTooLarge` | `pkg/bgp/message/update_split_test.go` | Attributes alone > maxSize → error (pre-existing) |
| `TestSplitUpdate_EmptyNLRI` | `pkg/bgp/message/update_split_test.go` | Empty NLRI list handled |
| `TestSplitUpdate_BothMPReachAndUnreach` | `pkg/bgp/message/update_split_test.go` | Both MP attrs need splitting (pre-existing) |

### Functional Tests
| Test | Location | Scenario |
|------|----------|----------|
| N/A | - | Wire splitting is unit-tested; functional tests cover end-to-end forwarding |

## Files to Modify
- `pkg/bgp/message/chunk_mp_nlri.go` - Add RFC reference/constraint comments to all size functions
- `pkg/bgp/message/chunk_mp_nlri_test.go` - Add SplitMPNLRI tests
- `pkg/bgp/message/update_split_test.go` - Add bounds safety tests

**Note on ChunkMPNLRI vs SplitMPNLRI:**
- `ChunkMPNLRI` - returns copies via `append()`, used when caller needs independent chunks
- `SplitMPNLRI` - returns subslices (zero-copy), used for wire forwarding hot path
- This spec's subslice requirement applies to `SplitMPNLRI` (already implemented correctly)

## Implementation Steps
1. **Write tests** - Test subslice returns and boundary conditions
2. **Run tests** - Verify FAIL (paste output)
3. **Modify ChunkMPNLRI** - Return subslices of original buffer
4. **Update callers** - Ensure they handle subslice semantics
5. **Run tests** - Verify PASS (paste output)
6. **Verify all** - `make lint && make test && make functional`
7. **RFC refs** - Add RFC reference comments to code
8. **RFC constraints** - Add constraint comments with quoted requirements

## Design Decisions
- **Check-after-write**: Simpler than pre-calculation
- **Subslices in hot path**: No append in split loop
- **Pool allocation OK**: Building output UPDATE can allocate from pool
- **Actual size calculation**: Read length byte from wire, no max-size lookup table
- **BGP-LS only unsplittable**: Single NLRI can exceed 4096 (2-byte length field)

## RFC Documentation

### Reference Comments
Added to `pkg/bgp/message/chunk_mp_nlri.go`:
- `// RFC 4271 Section 4.3 - UPDATE message format, max 4096 bytes.`
- `// RFC 8654 - Extended Message raises max to 65535 bytes.`
- `// RFC 4760 - MP_REACH_NLRI / MP_UNREACH_NLRI wire format.`
- `// RFC 7911 Section 3 - ADD-PATH NLRI encoding.`
- `// RFC 8277 Section 2 - Labeled unicast NLRI encoding.`
- `// RFC 4364 Section 4.3.4 - VPN-IPv4 NLRI encoding.`
- `// RFC 7432 Section 7 - EVPN NLRI encoding.`
- `// RFC 5575 Section 4 - FlowSpec NLRI encoding (max 4095 bytes).`
- `// RFC 7752 Section 3.2 - BGP-LS NLRI encoding (2-byte length, can exceed 4096).`

### Constraint Comments
Key constraints documented in code:

```go
// RFC 7752 Section 3.2: BGP-LS NLRI uses 2-byte length field
// Single NLRI can exceed standard 4096-byte message size
// MUST return error if single NLRI > maxSize (cannot split)
if nlriSize > maxSize {
    return nil, nil, fmt.Errorf("%w: %d bytes, max %d", ErrNLRITooLarge, nlriSize, maxSize)
}
```

```go
// RFC 8654: Extended Message raises UPDATE max to 65535 bytes
// When forwarding to standard peer (4096), MUST split large UPDATEs
// SplitMPNLRI returns subslices to avoid allocation in hot path
```

## Per-Family AddPath: Pre-Split Approach

Current: `SplitUpdateWithAddPath(u *Update, maxSize int, addPath bool)`

Problem: UPDATE can have MP_REACH (e.g., IPv6) + MP_UNREACH (e.g., IPv4) with **different** AddPath settings.

**Decision**: Caller pre-splits by family.

ForwardUpdate logic:
1. Check if UPDATE has both MP_REACH and MP_UNREACH
2. If same AddPath setting for both families → single call
3. If different → split UPDATE by family, call `SplitUpdateWithAddPath` separately

Benefits:
- No indirect function call in hot path
- Caller already has capability state from negotiation
- `SplitUpdateWithAddPath` signature stays simple
- Split function doesn't need to parse AFI/SAFI to determine AddPath

## Checklist

### 🧪 TDD
- [x] Tests written
- [x] Tests FAIL (N/A - implementation already existed)
- [x] Implementation complete
- [x] Tests PASS

### Verification
- [x] `make lint` passes (pre-existing warnings in other files)
- [x] `make test` passes
- [x] `make functional` passes (43/43 tests)

### Test Output
```
=== RUN   TestSplitMPNLRI_Subslice
--- PASS: TestSplitMPNLRI_Subslice (0.00s)
=== RUN   TestSplitMPNLRI_NoAllocHotPath
--- PASS: TestSplitMPNLRI_NoAllocHotPath (0.00s)
=== RUN   TestSplitMPNLRI_BoundaryExact
--- PASS: TestSplitMPNLRI_BoundaryExact (0.00s)
=== RUN   TestSplitMPNLRI_SingleNLRIFillsBuffer
--- PASS: TestSplitMPNLRI_SingleNLRIFillsBuffer (0.00s)
=== RUN   TestSplitUpdate_CheckAfterWrite
--- PASS: TestSplitUpdate_CheckAfterWrite (0.00s)
=== RUN   TestSplitUpdate_IPv4Field
--- PASS: TestSplitUpdate_IPv4Field (0.00s)
=== RUN   TestSplitUpdate_FlowSpec_Split
--- PASS: TestSplitUpdate_FlowSpec_Split (0.00s)
=== RUN   TestSplitUpdate_BGPLS_TooLarge
--- PASS: TestSplitUpdate_BGPLS_TooLarge (0.00s)
=== RUN   TestSplitUpdate_EmptyNLRI
--- PASS: TestSplitUpdate_EmptyNLRI (0.00s)
PASS
```

### Documentation
- [x] Required docs read
- [x] RFC summaries read (all referenced RFCs)
- [x] RFC references added to code
- [x] RFC constraint comments added (quoted requirement + explanation)
- [ ] `.claude/zebgp/` updated if schema changed (N/A - no schema changes)

### Completion
- [ ] Spec moved to `plan/done/NNN-<name>.md`
