# Spec: Update Wire Input (hex/b64)

## Task

Implement `handleUpdateHex()`, `handleUpdateB64()` handlers for the `peer X update <encoding> ...` command. These parse wire-encoded attributes and NLRIs, then send via reactor.

**Motivation:** Reduce parsing overhead and allow unknown/future BGP extensions to be announced without ZeBGP-specific support (requires future OPEN capability passthrough).

## Required Reading

- [ ] `docs/architecture/api/ARCHITECTURE.md` - API command structure
- [ ] `docs/architecture/api/CAPABILITY_CONTRACT.md` - API capability negotiation
- [ ] `docs/architecture/UPDATE_BUILDING.md` - UPDATE message building
- [ ] `docs/architecture/ENCODING_CONTEXT.md` - Context for wire encoding
- [ ] `docs/architecture/wire/ATTRIBUTES.md` - Wire format reference
- [ ] `docs/architecture/edge-cases/ADDPATH.md` - ADD-PATH path-id handling

**Key insights:**
- Existing `handleUpdateText()` parses text → NLRIBatch → reactor
- Existing `handleRaw()` sends raw bytes without structure (different use case)
- Wire mode reuses same reactor methods (`AnnounceNLRIBatch`, `WithdrawNLRIBatch`)
- Wire bytes: no validation of raw payload, pass through unchanged (context re-encoding deferred to spec-wire-recode.md)
- Reuse existing `WireEncoding` enum from `types.go`
- Wire mode uses `PathAttributes.Wire` field (`*attribute.AttributesWire`)
- NLRI splitting uses `GetNLRISizeFunc()` from `internal/bgp/message/chunk_mp_nlri.go` (export needed)
- API context (`APIContextID`) with ASN4=true for wire attribute encoding

## Design

### ADD-PATH Support

**Wire mode (hex/b64):** Path-id is embedded in wire data. Use `addpath` flag per-family:

```
nlri <family> [addpath] add <data>... [del <data>...]
```

**Examples:**
```bash
# Without ADD-PATH: data = [prefix-len][prefix]
peer 10.0.0.1 update hex nlri ipv4/unicast add 180a0000

# With ADD-PATH: data = [path-id:4][prefix-len][prefix]
peer 10.0.0.1 update hex nlri ipv4/unicast addpath add 00000001180a0000
#                                                      ^^^^^^^^ path-id=1
```

**Text mode:** Use `path-information` keyword as accumulator BEFORE nlri (ExaBGP compatible):
```bash
# path-information accumulates like nhop
peer 10.0.0.1 update text path-information 1 nlri ipv4/unicast add 10.0.0.0/24 10.1.0.0/24

# Change path-id mid-command
peer 10.0.0.1 update text path-information 1 nlri ipv4/unicast add 10.0.0.0/24 path-information 2 nlri ipv4/unicast add 10.1.0.0/24
```

**Splitting:** Parser uses `addpath` flag to call `GetNLRISizeFunc(afi, safi, addPath=true/false)` for correct NLRI boundary detection.

### Difference: `update hex` vs `raw update hex`

| Command | Structure | Validation | Use case |
|---------|-----------|------------|----------|
| `update hex attr ... nlri ...` | Structured (attr + family/nlri) | Decode check | Normal API with wire data |
| `raw update hex <bytes>` | Unstructured (full payload) | None | Debug/testing |

### Command Format

```
peer <addr> update <hex|b64|text> [attr set <data>] [nhop set <data>] [nhop del] [path-information <id>] [nlri <family> [addpath] add <data>... [del <data>...]]... [watchdog <name>]
```

**Accumulators:** `nhop`, `path-information` apply to subsequent `nlri` sections. Can appear multiple times to change value.

**Note:** Wire mode (hex/b64) only supports `attr set`. Use text mode for `attr add/del`. `path-information` only applies to text mode (wire mode embeds path-id in data).

**Examples:**
```bash
# Hex mode
peer 10.0.0.1 update hex attr set 400101400206020100001f94 nhop set 0a000001 nlri ipv4/unicast add 180a0000

# Base64 mode
peer 10.0.0.1 update b64 attr set QAEBQAIGAgEAAB+U nhop set CgAAAQ== nlri ipv4/unicast add GAEKAA==

# Text mode (existing behavior)
peer 10.0.0.1 update text attr set origin igp nhop set 10.0.0.1 nlri ipv4/unicast add 10.0.0.0/24

# Reset next-hop between families (nhop del then set new)
peer 10.0.0.1 update text attr set origin igp nhop set 10.0.0.1 nlri ipv4/unicast add 1.0.0.0/24 nhop del nhop set 2001:db8::1 nlri ipv6/unicast add 2001:db8::/32

# Withdrawal (no nhop needed)
peer 10.0.0.1 update hex nlri ipv4/unicast del 180a0000
```

### Wire Data Parsing

**Whitespace handling:** All whitespace stripped before decode. User may add spaces for readability; bytes are concatenated.

**Attributes (hex):** Concatenated path attributes in wire format
- Example: `400101 400206020100001f94` → `400101400206020100001f94` = ORIGIN(IGP) + AS_PATH(65000)
- Attributes packed in order, no validation of flags/structure
- MP_REACH/MP_UNREACH constructed by reactor based on family

**Next-Hop:** Follows same encoding as command (`hex`/`b64`/`text`)
- `nhop set 0a000001` (hex) = 10.0.0.1
- `nhop set CgAAAQ==` (b64) = 10.0.0.1
- `nhop set 10.0.0.1` (text) = parsed as IP
- `nhop set self` = use local address as next-hop ("self" is special value)
- `nhop del` = unset next-hop, takes no arguments (useful to reset between families)
- For IPv4 unicast: goes in NEXT_HOP attribute
- For MP families: goes in MP_REACH_NLRI next-hop field
- **Note:** nhop is required for announce (reactor validates at send time)

**NLRI (hex):** Concatenated, then split per-family wire format:
- IPv4/IPv6 unicast: `<length-byte><prefix-bytes>` (e.g., `180a0000` = 10.0.0.0/24)
- Labeled unicast: `<total-bits><labels><prefix-bytes>`
- VPN: `<total-bits><labels><RD:8><prefix-bytes>`
- Example: `180a0000 180b0000` → two NLRIs (10.0.0.0/24, 11.0.0.0/24) after decode and split

### Validation

| Check | Wire Mode |
|-------|-----------|
| Hex/b64 decode | Yes |
| Attribute flags valid | No (raw payload untouched) |
| NLRI structural parse | Yes (for splitting only, uses `GetNLRISizeFunc`) |
| NLRI semantic valid | No (raw payload untouched) |
| Family/prefix match | No (raw payload untouched) |
| Peer established | Yes |

**Note:** NLRI splitting requires structural parsing to find boundaries. Malformed length bytes cause split failure. Raw payload bytes are passed through without validation—user is responsible for correctness.

### Flow

```
Wire input → Strip whitespace → Decode (hex/b64) → Concatenate
    ↓
Attributes → NewAttributesWire(bytes, APIContextID) → PathAttributes{Wire: ...}
    ↓
Split NLRIs per family (GetNLRISizeFunc) → [][]byte slices
    ↓
Wrap each slice → []*WireNLRI (implements nlri.NLRI interface)
    ↓
NLRIGroup{Family, Announce: []nlri.NLRI, Withdraw: []nlri.NLRI, Attrs, NextHop}
    ↓
NLRIBatch → AnnounceNLRIBatch() / WithdrawNLRIBatch()
    ↓
Reactor builds UPDATE (adds MP_REACH/NEXT_HOP if needed) → Split if oversized
```

### Parser

```go
// ParseUpdateWire parses wire-encoded update command.
// Same structure as text but attrs/nlris are decoded wire bytes.
// Reuses existing NLRIGroup and WireEncoding types.
func ParseUpdateWire(args []string, encoding WireEncoding) (*UpdateTextResult, error)
```

Returns same `UpdateTextResult` as `ParseUpdateText()`. For wire mode:
- Attributes stored in `PathAttributes.Wire` field (`*attribute.AttributesWire`)
- Wire bytes created with `APIContextID` (ASN4=true)
- NLRIs split by family format, wrapped in `*WireNLRI` (implements `nlri.NLRI`)
- Context re-encoding deferred (user ensures matching contexts for now)

## Files to Modify

| File | Change |
|------|--------|
| `internal/plugin/types.go` | Add `Wire *attribute.AttributesWire` to `PathAttributes` |
| `internal/bgp/context/api.go` | **New:** `APIContextID` with ASN4=true |
| `internal/bgp/nlri/wire.go` | **New:** `WireNLRI` type implementing `NLRI` interface |
| `internal/bgp/nlri/wire_test.go` | **New:** Tests for `WireNLRI` |
| `internal/bgp/message/chunk_mp_nlri.go` | Export `GetNLRISizeFunc` (rename from `getNLRISizeFunc`) |
| `internal/plugin/update_wire.go` | **New:** `ParseUpdateWire()`, wire handlers |
| `internal/plugin/update_wire_test.go` | **New:** Tests for wire parsing |
| `internal/plugin/update_text.go` | Replace `next-hop`/`next-hop-self` with `nhop` keyword; update dispatcher |
| `internal/reactor/announce.go` | Handle `PathAttributes.Wire` in `AnnounceNLRIBatch()` (see Reactor Changes) |

### Type Changes

```go
// internal/plugin/types.go - Add Wire field to PathAttributes
type PathAttributes struct {
    Origin              *uint8
    LocalPreference     *uint32
    MED                 *uint32
    ASPath              []uint32
    Communities         []uint32
    LargeCommunities    []LargeCommunity
    ExtendedCommunities []attribute.ExtendedCommunity

    // Wire mode: lazy-parsed wire bytes (excludes NEXT_HOP/MP_REACH).
    // If set, semantic fields above are ignored.
    // Uses APIContextID as source context (ASN4=true).
    Wire *attribute.AttributesWire
}
```

```go
// internal/bgp/context/api.go - API context for wire input
package context

// APIContextID identifies API-originated wire data.
// Registered at init with ASN4=true for modern encoding.
//
// Init safety: Registry is package-level var (registry.go), initialized before
// init() runs. Go guarantees package-level vars init before init() functions.
var APIContextID ContextID

func init() {
    APIContextID = Registry.Register(&EncodingContext{
        ASN4: true,
    })
}
```

```go
// internal/bgp/nlri/wire.go - Wire NLRI for opaque wire bytes
package nlri

import (
    "encoding/binary"
    "fmt"
)

// WireNLRI wraps raw wire-encoded NLRI bytes.
// Implements NLRI interface for use in NLRIGroup.
// Used for wire mode API input where bytes are passed through without parsing.
//
// IMPORTANT: Caller must not modify data after calling NewWireNLRI.
// WireNLRI takes ownership of the slice (no copy for zero-allocation).
type WireNLRI struct {
    family     Family
    data       []byte // Raw wire bytes (with or without path-id based on hasAddPath)
    hasAddPath bool   // True if data starts with 4-byte path-id
}

// NewWireNLRI creates a WireNLRI from raw bytes.
// Data should be a single NLRI in wire format (already split from concatenated input).
// hasAddPath indicates if data includes 4-byte path-id prefix.
// Takes ownership of data slice - caller must not modify after this call.
// Returns error if hasAddPath but len(data) < 4 (malformed).
func NewWireNLRI(family Family, data []byte, hasAddPath bool) (*WireNLRI, error) {
    if hasAddPath && len(data) < 4 {
        return nil, fmt.Errorf("malformed NLRI: addpath flag set but data < 4 bytes")
    }
    return &WireNLRI{family: family, data: data, hasAddPath: hasAddPath}, nil
}

func (w *WireNLRI) Family() Family { return w.family }
func (w *WireNLRI) Len() int       { return len(w.data) } // Full raw length including path-id if present
func (w *WireNLRI) String() string { return fmt.Sprintf("wire[%s](%d bytes)", w.family, len(w.data)) }

// HasAddPath returns true if data includes path-id prefix.
func (w *WireNLRI) HasAddPath() bool { return w.hasAddPath }

// PathID extracts path-id from data (0 if !hasAddPath or data too short).
func (w *WireNLRI) PathID() uint32 {
    if !w.hasAddPath || len(w.data) < 4 {
        return 0
    }
    return binary.BigEndian.Uint32(w.data[:4])
}

// Bytes returns raw data as-is (including path-id if present).
func (w *WireNLRI) Bytes() []byte {
    return w.data
}

// Pack returns wire bytes adapted for context.
// Handles ADD-PATH mismatch (loses zero-copy benefit):
// - Source has path-id, target doesn't: strip 4 bytes (RFC 7911: path-id is always 4 bytes, any AFI)
// - Source no path-id, target expects: prepend NOPATH (0x00000000)
// Cannot fail: NewWireNLRI validates data length at construction.
func (w *WireNLRI) Pack(ctx *PackContext) []byte {
    targetAddPath := ctx != nil && ctx.AddPath

    if w.hasAddPath && !targetAddPath {
        // Strip 4-byte path-id (RFC 7911: same size for IPv4/IPv6/any AFI)
        // Safe: NewWireNLRI guarantees len >= 4 when hasAddPath
        return w.data[4:]
    }

    if !w.hasAddPath && targetAddPath {
        // Prepend NOPATH (path-id = 0) - allocates new slice
        buf := make([]byte, 4+len(w.data))
        // buf[0:4] already zero (NOPATH)
        copy(buf[4:], w.data)
        return buf
    }

    return w.data
}

// WriteTo writes the NLRI into buf at offset, adapting for context.
// Cannot fail: Pack() is guaranteed to succeed.
func (w *WireNLRI) WriteTo(buf []byte, off int, ctx *PackContext) int {
    return copy(buf[off:], w.Pack(ctx))
}
```

### Reactor Changes

When `batch.Attrs.Wire != nil`, reactor uses wire mode:

```go
// In AnnounceNLRIBatch:
if batch.Attrs.Wire != nil {
    // Wire mode: use raw bytes, only add next-hop handling
    baseAttrs := batch.Attrs.Wire.Bytes()
    peerCtx := peer.EncodingContext()

    // Context mismatch: future spec (see spec-wire-recode.md skeleton below)
    // For now, pass through unchanged. User responsible for matching contexts.

    if batch.Family.IsIPv4Unicast() {
        // Add NEXT_HOP attribute, NLRIs in UPDATE NLRI field
        nhAttr := attribute.NextHop(batch.NextHop.Resolve(peer)).Bytes()
        attrs := appendBytes(baseAttrs, nhAttr)
        nlriBytes := packWireNLRIs(batch.NLRIs, peerCtx)
        return buildUpdate(nil, attrs, nlriBytes)
    } else {
        // Construct MP_REACH_NLRI with next-hop + NLRIs
        mpReach := buildMPReachWire(batch.Family, batch.NextHop.Resolve(peer), batch.NLRIs, peerCtx)
        attrs := appendBytes(baseAttrs, mpReach)
        return buildUpdate(nil, attrs, nil)
    }
}

// In WithdrawNLRIBatch (similar pattern):
if batch.Attrs.Wire != nil {
    if batch.Family.IsIPv4Unicast() {
        // Withdrawn routes in UPDATE withdrawn field
        wdBytes := packWireNLRIs(batch.NLRIs, peerCtx)
        return buildUpdate(wdBytes, nil, nil)
    } else {
        // Construct MP_UNREACH_NLRI
        mpUnreach := buildMPUnreachWire(batch.Family, batch.NLRIs, peerCtx)
        return buildUpdate(nil, mpUnreach, nil)
    }
}
```

**Helper functions:**
```go
// packWireNLRIs packs []nlri.NLRI (containing WireNLRI) into wire bytes.
func packWireNLRIs(nlris []nlri.NLRI, ctx *PackContext) []byte {
    var buf bytes.Buffer
    for _, n := range nlris {
        buf.Write(n.Pack(ctx))
    }
    return buf.Bytes()
}

// buildMPReachWire constructs MP_REACH_NLRI from wire NLRIs.
// Next-hop length derived from addr type (4=IPv4, 16=IPv6, 32=IPv6+link-local).
func buildMPReachWire(family Family, nhop netip.Addr, nlris []nlri.NLRI, ctx *PackContext) []byte

// buildMPUnreachWire constructs MP_UNREACH_NLRI from wire NLRIs.
func buildMPUnreachWire(family Family, nlris []nlri.NLRI, ctx *PackContext) []byte
```

**Key points:**
- Wire attrs exclude NEXT_HOP and MP_REACH (reactor adds those)
- ADD-PATH mismatch handled by WireNLRI.Pack()
- Context mismatch (ASN4/ASN2): **deferred to spec-wire-recode.md**

### Future: Wire Re-encoding (spec-wire-recode.md)

**Skeleton for separate spec:**

```markdown
# Spec: Wire Re-encoding

## Task
Implement context-aware re-encoding for wire mode when source/target contexts differ.

## Scope
- ASN4 → ASN2: Parse AS_PATH, create AS4_PATH for AS > 65535, truncate AS_PATH
- ASN2 → ASN4: Merge AS4_PATH into AS_PATH if present
- ADD-PATH: Already handled by WireNLRI.Pack()

## Required Reading
- `docs/architecture/edge-cases/AS4.md`
- RFC 6793 (4-byte AS)

## Design
- reencodeAttrs(src []byte, srcCtx, dstCtx) []byte
- Lazy: only recode if contexts actually differ in relevant ways
- Cache: store re-encoded attrs per (srcCtxID, dstCtxID) pair?

## Deferred
- Not blocking for wire input MVP
- Wire mode users can ensure matching contexts
```

### Reused Types (no changes needed)

- `WireEncoding` - already in `types.go`
- `UpdateTextResult` - already in `types.go`
- `NLRIGroup` - already in `types.go`
- `NLRIBatch` - already in `types.go`
- `RouteNextHop` - already in `nexthop.go` (handles `self` policy)
- `AnnounceNLRIBatch()` / `WithdrawNLRIBatch()` - already in `ReactorInterface`

### Watchdog

Watchdog tags routes for bulk withdrawal. When `withdraw watchdog <name>` is called, all routes announced with that watchdog name are withdrawn. Useful for health-check integration (route removed when service fails).

### nhop Migration

**Breaking change:** `nhop` keyword replaces `next-hop`/`next-hop-self` inside `attr set`.

| Old (deprecated) | New |
|------------------|-----|
| `attr set next-hop 10.0.0.1` | `nhop set 10.0.0.1` |
| `attr set next-hop-self` | `nhop set self` |

**Migration path:** Old syntax returns error with migration hint: `"next-hop inside attr is deprecated, use: nhop set <addr>"`

### Empty Attributes

**Text mode:** If attributes missing, reactor adds defaults (ORIGIN IGP, empty AS_PATH).

**Wire mode:** Raw bytes are untouched. `attr set` is **required for announce** (reactor validates). Withdrawal-only commands need no attributes.

**Validation:** Parser errors if announce has NLRIs but no `attr set`:
```
"wire mode requires attr set for announce"
```

**Both modes:**
- Next-hop required for announce (reactor validates at send time)
- Withdrawal-only commands need no attributes

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates |
|------|------|-----------|
| `TestNewWireNLRI` | `internal/bgp/nlri/wire_test.go` | Constructor creates WireNLRI |
| `TestWireNLRI_Family` | `internal/bgp/nlri/wire_test.go` | Family() returns correct value |
| `TestWireNLRI_Bytes_NoAddPath` | `internal/bgp/nlri/wire_test.go` | Bytes() returns full data when no addpath |
| `TestWireNLRI_Bytes_WithAddPath` | `internal/bgp/nlri/wire_test.go` | Bytes() returns full data including path-id |
| `TestWireNLRI_Len` | `internal/bgp/nlri/wire_test.go` | Len() returns full data length |
| `TestWireNLRI_PathID_NoAddPath` | `internal/bgp/nlri/wire_test.go` | PathID() returns 0 when !hasAddPath |
| `TestWireNLRI_PathID_WithAddPath` | `internal/bgp/nlri/wire_test.go` | PathID() extracts from data when hasAddPath |
| `TestWireNLRI_Pack_NoMismatch` | `internal/bgp/nlri/wire_test.go` | Pack() returns data when no mismatch |
| `TestWireNLRI_Pack_StripPathID` | `internal/bgp/nlri/wire_test.go` | Pack() strips path-id when src has, target doesn't |
| `TestWireNLRI_Pack_PrependNOPATH` | `internal/bgp/nlri/wire_test.go` | Pack() prepends NOPATH when src lacks, target expects |
| `TestNewWireNLRI_Malformed` | `internal/bgp/nlri/wire_test.go` | Constructor returns error when hasAddPath but len < 4 |
| `TestWireNLRI_WriteTo` | `internal/bgp/nlri/wire_test.go` | WriteTo() copies packed data to buffer |
| `TestWireNLRI_String` | `internal/bgp/nlri/wire_test.go` | String() returns readable format |
| `TestAPIContextID` | `internal/bgp/context/api_test.go` | APIContextID registered with ASN4=true |
| `TestParseUpdateWire_HexAttrs` | `update_wire_test.go` | Hex attribute decoding |
| `TestParseUpdateWire_HexNLRI` | `update_wire_test.go` | Hex NLRI decoding + split |
| `TestParseUpdateWire_HexNhop` | `update_wire_test.go` | Hex next-hop decoding |
| `TestParseUpdateWire_NhopDel` | `update_wire_test.go` | nhop del unsets next-hop |
| `TestParseUpdateWire_NhopSetSelf` | `update_wire_test.go` | nhop set self uses local address |
| `TestParseUpdateWire_B64Attrs` | `update_wire_test.go` | Base64 attribute decoding |
| `TestParseUpdateWire_B64NLRI` | `update_wire_test.go` | Base64 NLRI decoding |
| `TestParseUpdateWire_B64Nhop` | `update_wire_test.go` | Base64 next-hop decoding |
| `TestParseUpdateWire_InvalidHex` | `update_wire_test.go` | Invalid hex rejected |
| `TestParseUpdateWire_InvalidB64` | `update_wire_test.go` | Invalid base64 rejected |
| `TestParseUpdateWire_SpacesStripped` | `update_wire_test.go` | Whitespace handling |
| `TestParseUpdateWire_MultipleNLRI` | `update_wire_test.go` | Concatenated NLRIs split correctly |
| `TestParseUpdateWire_AddDel` | `update_wire_test.go` | Mixed add/del in wire mode |
| `TestParseUpdateWire_NhopPerFamily` | `update_wire_test.go` | nhop snapshot per nlri section |
| `TestParseUpdateWire_AddPath` | `update_wire_test.go` | addpath flag enables path-id in split |
| `TestParseUpdateWire_AddPathSplit` | `update_wire_test.go` | Correct NLRI splitting with addpath |
| `TestParseUpdateWire_NoAttrsAnnounce` | `update_wire_test.go` | Error when announce without attr set |
| `TestParseUpdateWire_NoAttrsWithdraw` | `update_wire_test.go` | OK when withdraw-only without attr set |
| `TestHandleUpdateHex_Integration` | `update_wire_test.go` | Full handler flow |
| `TestHandleUpdateB64_Integration` | `update_wire_test.go` | Full handler flow |
| `TestParseUpdateText_NhopSet` | `update_text_test.go` | `nhop set` in text mode |
| `TestParseUpdateText_NhopSetSelf` | `update_text_test.go` | `nhop set self` in text mode |
| `TestParseUpdateText_NhopDel` | `update_text_test.go` | `nhop del` in text mode |
| `TestParseUpdateText_PathInfo` | `update_text_test.go` | `path-information` as accumulator |
| `TestParseUpdateText_PathInfoChange` | `update_text_test.go` | `path-information` changes mid-command |
| `TestReactor_WireMode_IPv4` | `internal/reactor/announce_test.go` | Wire mode with IPv4 unicast |
| `TestReactor_WireMode_IPv6` | `internal/reactor/announce_test.go` | Wire mode with IPv6 unicast (MP_REACH) |
| `TestReactor_WireMode_Withdraw` | `internal/reactor/announce_test.go` | Wire mode withdrawal |
| `TestReactor_WireMode_AddPathMismatch` | `internal/reactor/announce_test.go` | ADD-PATH strip/prepend |

### Functional Tests

| Test | Location | Scenario |
|------|----------|----------|
| `update-hex` | `qa/tests/api/` | Hex encoding end-to-end |
| `update-b64` | `qa/tests/api/` | Base64 encoding end-to-end |

## Implementation Steps

### Phase 0: Type Setup

1. Add `Wire *attribute.AttributesWire` field to `PathAttributes` in `internal/plugin/types.go`
2. Create `internal/bgp/context/api.go` with `APIContextID` (ASN4=true)
3. Create `internal/bgp/nlri/wire.go` with `WireNLRI` type (TDD: write tests first)
4. Export `GetNLRISizeFunc` in `internal/bgp/message/chunk_mp_nlri.go` (rename from `getNLRISizeFunc`)
5. Run `make test` to verify no regressions

### Phase 1: Text Mode nhop + path-information (TDD)

1. Write test `TestParseUpdateText_NhopSet` → MUST FAIL
2. Add `nhop` keyword parsing to `update_text.go` → MUST PASS
3. Write test `TestParseUpdateText_PathInfo` → MUST FAIL
4. Add `path-information` keyword as accumulator → MUST PASS
5. Verify existing text tests still pass

### Phase 2: Hex/B64 Parser (TDD)

1. Write test `TestParseUpdateWire_HexAttrs` → MUST FAIL
2. Implement `ParseUpdateWire()` hex attribute decoding → MUST PASS
3. Write test `TestParseUpdateWire_HexNLRI` → MUST FAIL
4. Implement NLRI decode + per-family split → MUST PASS
5. Write test `TestParseUpdateWire_HexNhop` → MUST FAIL
6. Implement nhop decoding → MUST PASS
7. Write test `TestParseUpdateWire_B64*` → MUST FAIL
8. Implement base64 support → MUST PASS
9. Write test `TestParseUpdateWire_SpacesStripped` → MUST FAIL
10. Implement whitespace stripping → MUST PASS
11. Write test `TestParseUpdateWire_Invalid*` → MUST FAIL
12. Implement error handling → MUST PASS

### Phase 3: Handlers (TDD)

1. Write test `TestHandleUpdateHex_Integration` → MUST FAIL
2. Implement `handleUpdateHex()` → MUST PASS
3. Write test `TestHandleUpdateB64_Integration` → MUST FAIL
4. Implement `handleUpdateB64()` → MUST PASS
5. Update dispatcher in `handleUpdate()` to call new handlers

### Phase 4: Reactor Wire Mode (TDD)

1. Write test `TestReactor_WireMode_IPv4` → MUST FAIL
2. Implement wire mode detection in `AnnounceNLRIBatch()` → MUST PASS
3. Write test `TestReactor_WireMode_IPv6` → MUST FAIL
4. Implement `buildMPReachWire()` → MUST PASS
5. Write test `TestReactor_WireMode_Withdraw` → MUST FAIL
6. Implement wire mode in `WithdrawNLRIBatch()` → MUST PASS
7. Write test `TestReactor_WireMode_AddPathMismatch` → MUST FAIL
8. Verify WireNLRI.Pack() handles mismatch → MUST PASS

## Error Messages

| Condition | Message |
|-----------|---------|
| Invalid hex | `invalid hex in attr: encoding/hex: invalid byte` |
| Invalid base64 | `invalid base64 in attr: illegal base64 data` |
| Empty attr | `attr section requires wire data after set` |
| Empty nhop set | `nhop set requires data` |
| nhop del with data | `nhop del takes no arguments` |
| Invalid nhop | `invalid next-hop: <details>` |
| Unknown family | `unknown family: <name>` |
| Missing family | `nlri requires family: nlri <family> add ...` |
| Empty nlri | `nlri section requires wire data after add/del` |
| NLRI split fail | `failed to split NLRIs for <family>: <details>` |
| NLRI malformed (constructor) | `malformed NLRI: addpath flag set but data < 4 bytes` |
| Unknown encoding | `unknown encoding: <name>, expected hex/b64/text` |
| Multiple attr set | `attr set can only appear once` |
| Wire announce no attrs | `wire mode requires attr set for announce` |
| Invalid path-info | `invalid path-information: <details>` |
| path-info in wire mode | `path-information only valid in text mode, use addpath flag for wire` |

## RFC Documentation

- RFC 4271 Section 4.3 - UPDATE Message Format (attributes, NLRI fields)
- RFC 4760 Section 3 - MP_REACH_NLRI / MP_UNREACH_NLRI for non-IPv4 families
- RFC 7911 Section 3 - ADD-PATH path identifier encoding (4-byte prefix)

Add `// RFC NNNN Section X.Y` comments to protocol code.
If RFC missing: `curl -o rfc/rfcNNNN.txt https://www.rfc-editor.org/rfc/rfcNNNN.txt`

## Checklist

### Phase 0: Type Setup ✅
- [x] `Wire` field added to `PathAttributes`
- [x] `internal/bgp/context/api_test.go` written (TDD)
- [x] `internal/bgp/context/api.go` created with `APIContextID`
- [x] `internal/bgp/nlri/wire_test.go` written (TDD)
- [x] `internal/bgp/nlri/wire.go` created with `WireNLRI`
- [x] `GetNLRISizeFunc` exported in `chunk_mp_nlri.go` (uses `nlri.AFI`/`nlri.SAFI` types)
- [x] `make test` passes (no regressions)

### Phase 1: Text Mode nhop + path-information ✅
- [x] Test fails first (`TestParseUpdateText_NhopSet`)
- [x] Test passes after impl
- [x] Test fails first (`TestParseUpdateText_PathInfo`)
- [x] Test passes after impl
- [x] Existing text tests updated to new syntax
- [x] Old `next-hop`/`next-hop-self` syntax returns deprecation error

### Phase 2: Hex/B64 Parser ✅
- [x] Test fails first (hex attrs)
- [x] Test passes after impl
- [x] Test fails first (hex nlri + split)
- [x] Test passes after impl
- [x] Test fails first (hex nhop)
- [x] Test passes after impl
- [x] Test fails first (b64)
- [x] Test passes after impl
- [x] Test fails first (whitespace)
- [x] Test passes after impl
- [x] Test fails first (invalid input)
- [x] Test passes after impl
- [x] `ParseUpdateWire()` implemented in `internal/plugin/update_wire.go`
- [x] 22 tests in `internal/plugin/update_wire_test.go`

### Phase 3: Handlers
- [x] Test fails first (handleUpdateHex)
- [x] Test passes after impl
- [x] Test fails first (handleUpdateB64)
- [x] Test passes after impl
- [x] Dispatcher updated

### Phase 4: Reactor Wire Mode
- [x] Test fails first (IPv4 wire mode)
- [x] Test passes after impl
- [x] Test fails first (IPv6/MP wire mode)
- [x] Test passes after impl
- [x] Test fails first (withdraw wire mode)
- [x] Test passes after impl
- [ ] Test fails first (ADD-PATH mismatch) - deferred (requires peer capability checking)
- [ ] Test passes after impl

### Verification
- [x] `make lint` passes (no new issues in our code)
- [x] `make test` passes
- [x] `make functional` passes

### Documentation
- [x] Required docs read
- [x] RFC references added (RFC 4271, 4760, 7911 comments in code)
- [ ] `docs/architecture/api/ARCHITECTURE.md` updated if API changes

### Completion
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`

---

**Created:** 2026-01-07
