# Spec: wireupdate-split

## Task

Implement wire-level UPDATE splitting without parsing to Route objects.

## Required Reading

- [x] `.claude/zebgp/UPDATE_BUILDING.md` - Build vs Forward paths
- [x] `.claude/zebgp/ENCODING_CONTEXT.md` - Zero-copy wire handling
- [x] `.claude/zebgp/wire/MESSAGES.md` - UPDATE structure
- [x] `.claude/zebgp/wire/NLRI.md` - NLRI formats
- [x] `.claude/zebgp/wire/ATTRIBUTES.md` - MP_REACH/MP_UNREACH
- [x] `.claude/zebgp/edge-cases/ADDPATH.md` - ADD-PATH path-id handling

**Key insights:**
- Forward path preserves wire bytes, no Route parsing needed
- WireUpdate holds raw payload, can slice without allocation
- MP_REACH/MP_UNREACH are attributes containing NLRIs
- NLRI format varies by AFI/SAFI (critical for splitting)
- ADD-PATH prepends 4-byte path-id to each NLRI

## Problem

Current split path:
```
WireUpdate → ConvertToRoutes() → []*Route → Build new UPDATEs
```

Issues:
- `ConvertToRoutes()` requires `Announces`/`Withdraws` fields (never populated)
- Full Route parsing is wasteful for simple split operation
- Creates unnecessary objects

## Target Design

```
WireUpdate → SplitUpdate(maxSize) → []*WireUpdate
```

**Principle:** Rare operation → simplicity over performance. Just copy raw bytes.

**Accept invalid, emit valid:**
- API may send invalid UPDATEs (multiple MP_REACH/MP_UNREACH for different AFI/SAFIs)
- Split into RFC-compliant UPDATEs (at most one MP_REACH + one MP_UNREACH each)

**What one UPDATE can contain (RFC 4760):**
- IPv4 withdraws (Withdrawn field)
- IPv4 announces (NLRI field)
- One MP_UNREACH (one AFI/SAFI)
- One MP_REACH (one AFI/SAFI)
- Base attrs required if any announces present

**Core algorithm:**
```
1. Extract components: IPv4 withdraws, IPv4 announces, []MP_REACH, []MP_UNREACH, baseAttrs
2. Fast path: if everything fits in one UPDATE → copy and done
3. Slow path: for each (MP_REACH[i], MP_UNREACH[j]) pair:
   a. Build UPDATE with: IPv4 withdraws + MP_UNREACH[j] + baseAttrs + MP_REACH[i] + IPv4 announces
   b. If too large: split NLRIs, fill until can't, emit, repeat
   c. Clear IPv4 portions after first UPDATE (only include once)
4. Handle remaining IPv4 if no MP_* attrs
```

**Output guarantees:**
- Each UPDATE: at most one MP_REACH, one MP_UNREACH (RFC compliant)
- NLRIs split at boundaries if too large
- Single NLRI > maxSize → error

## UPDATE Structure (RFC 4271)

```
+-----------------------------------------------------+
| Withdrawn Routes Length (2 bytes)                   |
+-----------------------------------------------------+
| Withdrawn Routes (variable)                         |  ← IPv4 withdraws
+-----------------------------------------------------+
| Total Path Attribute Length (2 bytes)               |
+-----------------------------------------------------+
| Path Attributes (variable)                          |  ← Contains MP_REACH/MP_UNREACH
+-----------------------------------------------------+
| NLRI (variable)                                     |  ← IPv4 announces
+-----------------------------------------------------+
```

**Split targets:**
- IPv4 withdraws: Withdrawn Routes field
- IPv4 announces: NLRI field
- IPv6/VPN/etc: MP_REACH_NLRI / MP_UNREACH_NLRI attributes

## NLRI Format by AFI/SAFI

**Critical:** Different AFI/SAFIs have different NLRI encodings. Split functions MUST understand these.

| AFI/SAFI | Format | Notes |
|----------|--------|-------|
| IPv4 Unicast (1/1) | `len(1) + prefix` | len = prefix bits, data = ceil(len/8) bytes |
| IPv6 Unicast (2/1) | `len(1) + prefix` | len = prefix bits, data = ceil(len/8) bytes |
| VPNv4 (1/128) | `len(1) + label(3) + RD(8) + prefix` | len includes label+RD bits |
| VPNv6 (2/128) | `len(1) + label(3) + RD(8) + prefix` | len includes label+RD bits |
| FlowSpec IPv4 (1/133) | `len(1-2) + filter` | len>240 uses 2-byte length |
| FlowSpec IPv6 (2/133) | `len(1-2) + filter` | len>240 uses 2-byte length |

**ADD-PATH:** If negotiated, each NLRI is prefixed with 4-byte path-id:
```
path-id(4) + normal_nlri
```

## Implementation

### 1. NLRI Splitter Registry

```go
// internal/bgp/message/nlri_split.go

// NLRISplitter knows how to find NLRI boundaries for a specific AFI/SAFI.
type NLRISplitter interface {
    // NextNLRI returns the length of the next NLRI in data.
    // Returns 0 if data is empty, error if malformed.
    NextNLRI(data []byte, addPath bool) (int, error)
}

// nlriSplitters maps AFI/SAFI to splitter implementation.
var nlriSplitters = map[uint32]NLRISplitter{
    afiSafiKey(1, 1):   prefixNLRISplitter{},   // IPv4 Unicast
    afiSafiKey(2, 1):   prefixNLRISplitter{},   // IPv6 Unicast
    afiSafiKey(1, 128): vpnNLRISplitter{},      // VPNv4
    afiSafiKey(2, 128): vpnNLRISplitter{},      // VPNv6
    afiSafiKey(1, 133): flowspecNLRISplitter{}, // FlowSpec IPv4
    afiSafiKey(2, 133): flowspecNLRISplitter{}, // FlowSpec IPv6
}

func afiSafiKey(afi uint16, safi uint8) uint32 {
    return uint32(afi)<<8 | uint32(safi)
}

// GetNLRISplitter returns splitter for AFI/SAFI, or prefixNLRISplitter as default.
func GetNLRISplitter(afi uint16, safi uint8) NLRISplitter {
    if s, ok := nlriSplitters[afiSafiKey(afi, safi)]; ok {
        return s
    }
    return prefixNLRISplitter{} // Safe default: length-prefixed
}
```

### 2. Splitter Implementations

```go
// prefixNLRISplitter handles standard length-prefixed NLRIs (IPv4/IPv6 unicast).
type prefixNLRISplitter struct{}

func (prefixNLRISplitter) NextNLRI(data []byte, addPath bool) (int, error) {
    if len(data) == 0 {
        return 0, nil
    }
    offset := 0
    if addPath {
        if len(data) < 4 {
            return 0, fmt.Errorf("truncated path-id")
        }
        offset = 4
    }
    if len(data) <= offset {
        return 0, fmt.Errorf("truncated NLRI")
    }
    prefixLen := int(data[offset])
    dataLen := (prefixLen + 7) / 8
    total := offset + 1 + dataLen
    if len(data) < total {
        return 0, fmt.Errorf("truncated prefix data")
    }
    return total, nil
}

// vpnNLRISplitter handles VPNv4/VPNv6 (label + RD + prefix).
type vpnNLRISplitter struct{}

func (vpnNLRISplitter) NextNLRI(data []byte, addPath bool) (int, error) {
    if len(data) == 0 {
        return 0, nil
    }
    offset := 0
    if addPath {
        if len(data) < 4 {
            return 0, fmt.Errorf("truncated path-id")
        }
        offset = 4
    }
    if len(data) <= offset {
        return 0, fmt.Errorf("truncated NLRI")
    }
    // Length is in bits, includes label(24) + RD(64) + prefix
    bitLen := int(data[offset])
    // Subtract label+RD bits to get prefix bits
    prefixBits := bitLen - 24 - 64
    if prefixBits < 0 {
        return 0, fmt.Errorf("invalid VPN NLRI length: %d", bitLen)
    }
    // Total bytes: length(1) + label(3) + RD(8) + prefix_bytes
    prefixBytes := (prefixBits + 7) / 8
    total := offset + 1 + 3 + 8 + prefixBytes
    if len(data) < total {
        return 0, fmt.Errorf("truncated VPN NLRI")
    }
    return total, nil
}

// flowspecNLRISplitter handles FlowSpec (variable length encoding).
type flowspecNLRISplitter struct{}

func (flowspecNLRISplitter) NextNLRI(data []byte, addPath bool) (int, error) {
    if len(data) == 0 {
        return 0, nil
    }
    offset := 0
    if addPath {
        if len(data) < 4 {
            return 0, fmt.Errorf("truncated path-id")
        }
        offset = 4
    }
    if len(data) <= offset {
        return 0, fmt.Errorf("truncated NLRI")
    }
    // FlowSpec length encoding: if first byte >= 240, use 2 bytes
    lenBytes := 1
    nlriLen := int(data[offset])
    if nlriLen >= 240 {
        if len(data) < offset+2 {
            return 0, fmt.Errorf("truncated FlowSpec length")
        }
        lenBytes = 2
        nlriLen = int(data[offset])<<8 | int(data[offset+1])
    }
    total := offset + lenBytes + nlriLen
    if len(data) < total {
        return 0, fmt.Errorf("truncated FlowSpec NLRI")
    }
    return total, nil
}
```

### 3. Split Functions

```go
// internal/plugin/wire_update_split.go

// SplitUpdate splits a WireUpdate into multiple RFC-compliant UPDATEs.
// Each output fits within maxBodySize (excludes 19-byte header).
// Returns original in single-element slice if no split needed.
// Returns error if single NLRI > maxSize.
//
// RFC 4271 Section 4.3 - UPDATE Message Handling
// RFC 4760 Section 4 - MP_REACH_NLRI and MP_UNREACH_NLRI
func SplitUpdate(wu *WireUpdate, maxBodySize int, addPath bool) ([]*WireUpdate, error) {
    payload := wu.Payload()

    // Fast path: no split needed
    if len(payload) <= maxBodySize {
        return []*WireUpdate{wu}, nil
    }

    // Parse structure (offsets only, no allocation)
    if len(payload) < 4 {
        return nil, fmt.Errorf("UPDATE too short: %d bytes", len(payload))
    }
    withdrawnLen := int(binary.BigEndian.Uint16(payload[0:2]))
    withdrawnEnd := 2 + withdrawnLen
    if len(payload) < withdrawnEnd+2 {
        return nil, fmt.Errorf("UPDATE truncated at attr length")
    }
    attrLen := int(binary.BigEndian.Uint16(payload[withdrawnEnd : withdrawnEnd+2]))
    attrStart := withdrawnEnd + 2
    attrEnd := attrStart + attrLen
    if len(payload) < attrEnd {
        return nil, fmt.Errorf("UPDATE truncated at attributes")
    }

    // Extract components as wire slices
    ipv4Withdraws := payload[2:withdrawnEnd]
    attrs := payload[attrStart:attrEnd]
    ipv4NLRI := payload[attrEnd:]

    // Separate MP_REACH/MP_UNREACH from base attributes
    baseAttrs, mpReaches, mpUnreaches, err := separateMPAttributes(attrs)
    if err != nil {
        return nil, fmt.Errorf("parsing attributes: %w", err)
    }

    var results []*WireUpdate

    // Track remaining IPv4
    remIPv4W := ipv4Withdraws
    remIPv4A := ipv4NLRI

    // Process each MP_* combination (or just IPv4 if no MP_*)
    maxIter := max(len(mpReaches), len(mpUnreaches), 1)

    for i := 0; i < maxIter; i++ {
        var mpReach, mpUnreach []byte
        if i < len(mpReaches) {
            mpReach = mpReaches[i]
        }
        if i < len(mpUnreaches) {
            mpUnreach = mpUnreaches[i]
        }

        // Include IPv4 only in first iteration
        var useIPv4W, useIPv4A []byte
        if i == 0 {
            useIPv4W = remIPv4W
            useIPv4A = remIPv4A
        }

        // Build UPDATEs for this combination
        updates, err := buildCombinedUpdates(
            useIPv4W, baseAttrs, mpUnreach, mpReach, useIPv4A,
            maxBodySize, addPath, wu.SourceCtxID())
        if err != nil {
            return nil, err
        }
        results = append(results, updates...)
    }

    if len(results) == 0 {
        // Empty UPDATE - return original
        return []*WireUpdate{wu}, nil
    }

    return results, nil
}
```

### 4. Attribute Separation

```go
// separateMPAttributes extracts MP_REACH and MP_UNREACH from attributes.
// Returns: baseAttrs (without MP_*), []mpReach, []mpUnreach.
// Each mpReach/mpUnreach is a complete attribute with header.
func separateMPAttributes(attrs []byte) (base []byte, mpReaches, mpUnreaches [][]byte, err error) {
    var baseBuilder []byte
    pos := 0

    for pos < len(attrs) {
        if len(attrs) < pos+2 {
            return nil, nil, nil, fmt.Errorf("truncated attribute at %d", pos)
        }

        flags := attrs[pos]
        typeCode := attrs[pos+1]
        headerLen := 3 // flags + type + len(1)
        if flags&0x10 != 0 { // Extended length
            headerLen = 4
        }

        if len(attrs) < pos+headerLen {
            return nil, nil, nil, fmt.Errorf("truncated attribute header at %d", pos)
        }

        var attrLen int
        if flags&0x10 != 0 {
            attrLen = int(binary.BigEndian.Uint16(attrs[pos+2 : pos+4]))
        } else {
            attrLen = int(attrs[pos+2])
        }

        totalLen := headerLen + attrLen
        if len(attrs) < pos+totalLen {
            return nil, nil, nil, fmt.Errorf("truncated attribute value at %d", pos)
        }

        attrBytes := attrs[pos : pos+totalLen]

        switch typeCode {
        case 14: // MP_REACH_NLRI
            mpReaches = append(mpReaches, attrBytes)
        case 15: // MP_UNREACH_NLRI
            mpUnreaches = append(mpUnreaches, attrBytes)
        default:
            baseBuilder = append(baseBuilder, attrBytes...)
        }

        pos += totalLen
    }

    return baseBuilder, mpReaches, mpUnreaches, nil
}
```

### 5. NLRI Splitting

```go
// splitNLRIs splits NLRIs to fit within maxBytes.
// Returns (fitting, remaining, error). Empty fitting if first NLRI > maxBytes.
func splitNLRIs(data []byte, maxBytes int, splitter NLRISplitter, addPath bool) (fitting, remaining []byte, err error) {
    if len(data) == 0 || len(data) <= maxBytes {
        return data, nil, nil
    }

    pos := 0
    for pos < len(data) {
        nlriLen, err := splitter.NextNLRI(data[pos:], addPath)
        if err != nil {
            return nil, nil, fmt.Errorf("at offset %d: %w", pos, err)
        }
        if nlriLen == 0 {
            break
        }

        newPos := pos + nlriLen
        if newPos > maxBytes {
            // This NLRI doesn't fit
            if pos == 0 {
                // First NLRI too large
                return nil, nil, fmt.Errorf("single NLRI (%d bytes) exceeds max (%d)", nlriLen, maxBytes)
            }
            break
        }
        pos = newPos
    }

    return data[:pos], data[pos:], nil
}

// splitIPv4NLRIs splits IPv4 unicast NLRIs (legacy UPDATE fields).
func splitIPv4NLRIs(data []byte, maxBytes int, addPath bool) (fitting, remaining []byte, err error) {
    return splitNLRIs(data, maxBytes, prefixNLRISplitter{}, addPath)
}
```

### 6. MP Attribute Splitting

```go
// splitMPReach splits MP_REACH_NLRI to fit within maxBytes.
// Returns complete attributes with headers. NextHop preserved in each split.
func splitMPReach(attr []byte, maxBytes int, addPath bool) (fitting, remaining []byte, err error) {
    if len(attr) <= maxBytes {
        return attr, nil, nil
    }

    // Parse MP_REACH structure
    // Header: flags(1) + type(1) + len(1-2)
    flags := attr[0]
    headerLen := 3
    if flags&0x10 != 0 {
        headerLen = 4
    }

    // AFI(2) + SAFI(1) + NH_Len(1) + NextHop(var) + Reserved(1) + NLRIs
    if len(attr) < headerLen+4 {
        return nil, nil, fmt.Errorf("MP_REACH too short")
    }

    afi := binary.BigEndian.Uint16(attr[headerLen : headerLen+2])
    safi := attr[headerLen+2]
    nhLen := int(attr[headerLen+3])
    nlriStart := headerLen + 4 + nhLen + 1 // +1 for reserved byte

    if len(attr) < nlriStart {
        return nil, nil, fmt.Errorf("MP_REACH truncated at NextHop")
    }

    // Fixed part: header + AFI/SAFI + NH (must be in every split)
    fixedPart := attr[:nlriStart]
    nlris := attr[nlriStart:]

    // Available space for NLRIs
    availableForNLRI := maxBytes - len(fixedPart)
    if availableForNLRI <= 0 {
        return nil, nil, fmt.Errorf("MP_REACH fixed part (%d) exceeds max (%d)", len(fixedPart), maxBytes)
    }

    splitter := GetNLRISplitter(afi, safi)
    fitNLRI, restNLRI, err := splitNLRIs(nlris, availableForNLRI, splitter, addPath)
    if err != nil {
        return nil, nil, err
    }

    // Build fitting attribute
    fitting = buildMPAttribute(flags, 14, fixedPart[headerLen:nlriStart], fitNLRI)

    // Build remaining if any
    if len(restNLRI) > 0 {
        remaining = buildMPAttribute(flags, 14, fixedPart[headerLen:nlriStart], restNLRI)
    }

    return fitting, remaining, nil
}

// splitMPUnreach splits MP_UNREACH_NLRI to fit within maxBytes.
func splitMPUnreach(attr []byte, maxBytes int, addPath bool) (fitting, remaining []byte, err error) {
    if len(attr) <= maxBytes {
        return attr, nil, nil
    }

    // Parse MP_UNREACH structure
    // Header: flags(1) + type(1) + len(1-2)
    flags := attr[0]
    headerLen := 3
    if flags&0x10 != 0 {
        headerLen = 4
    }

    // AFI(2) + SAFI(1) + NLRIs
    if len(attr) < headerLen+3 {
        return nil, nil, fmt.Errorf("MP_UNREACH too short")
    }

    afi := binary.BigEndian.Uint16(attr[headerLen : headerLen+2])
    safi := attr[headerLen+2]
    nlriStart := headerLen + 3

    fixedPart := attr[:nlriStart]
    nlris := attr[nlriStart:]

    availableForNLRI := maxBytes - len(fixedPart)
    if availableForNLRI <= 0 {
        return nil, nil, fmt.Errorf("MP_UNREACH fixed part (%d) exceeds max (%d)", len(fixedPart), maxBytes)
    }

    splitter := GetNLRISplitter(afi, safi)
    fitNLRI, restNLRI, err := splitNLRIs(nlris, availableForNLRI, splitter, addPath)
    if err != nil {
        return nil, nil, err
    }

    fitting = buildMPAttribute(flags, 15, fixedPart[headerLen:nlriStart], fitNLRI)

    if len(restNLRI) > 0 {
        remaining = buildMPAttribute(flags, 15, fixedPart[headerLen:nlriStart], restNLRI)
    }

    return fitting, remaining, nil
}

// buildMPAttribute constructs MP_REACH or MP_UNREACH with correct length/flags.
func buildMPAttribute(origFlags byte, typeCode byte, afiSafiNH []byte, nlris []byte) []byte {
    valueLen := len(afiSafiNH) + len(nlris)

    // Determine if extended length needed
    useExtended := valueLen > 255

    headerLen := 3
    if useExtended {
        headerLen = 4
    }

    buf := make([]byte, headerLen+valueLen)

    // Flags: preserve Optional/Transitive/Partial, set Extended if needed
    flags := origFlags & 0xE0 // Keep O/T/P bits
    if useExtended {
        flags |= 0x10
    }
    buf[0] = flags
    buf[1] = typeCode

    if useExtended {
        binary.BigEndian.PutUint16(buf[2:4], uint16(valueLen))
    } else {
        buf[2] = byte(valueLen)
    }

    copy(buf[headerLen:], afiSafiNH)
    copy(buf[headerLen+len(afiSafiNH):], nlris)

    return buf
}
```

### 7. Combined UPDATE Builder

```go
// buildCombinedUpdates builds UPDATEs with mixed components, splitting if needed.
func buildCombinedUpdates(
    ipv4W, baseAttrs, mpUnreach, mpReach, ipv4A []byte,
    maxSize int, addPath bool, sourceCtx uint64,
) ([]*WireUpdate, error) {
    // Fast path: everything fits
    total := 4 + len(ipv4W) + len(mpUnreach) // 4 = length fields
    hasAnnounces := len(mpReach) > 0 || len(ipv4A) > 0
    if hasAnnounces {
        total += len(baseAttrs) + len(mpReach) + len(ipv4A)
    }

    if total <= maxSize {
        if total == 4 && len(ipv4W) == 0 && len(mpUnreach) == 0 {
            return nil, nil // Empty
        }
        payload := buildUpdatePayload(ipv4W, baseAttrs, mpUnreach, mpReach, ipv4A)
        return []*WireUpdate{NewWireUpdateFromPayload(payload, sourceCtx)}, nil
    }

    // Slow path: iteratively fill and emit
    var results []*WireUpdate
    remIPv4W, remMPU, remMPR, remIPv4A := ipv4W, mpUnreach, mpReach, ipv4A

    for len(remIPv4W) > 0 || len(remMPU) > 0 || len(remMPR) > 0 || len(remIPv4A) > 0 {
        // Calculate overhead for this iteration
        iterHasAnnounces := len(remMPR) > 0 || len(remIPv4A) > 0
        overhead := 4 // Length fields
        if iterHasAnnounces {
            overhead += len(baseAttrs)
        }

        available := maxSize - overhead
        var fitIPv4W, fitMPU, fitMPR, fitIPv4A []byte

        // Fill in order: IPv4 withdraws, MP_UNREACH, MP_REACH, IPv4 announces
        if len(remIPv4W) > 0 && available > 0 {
            fit, rest, err := splitIPv4NLRIs(remIPv4W, available, addPath)
            if err != nil {
                return nil, fmt.Errorf("split IPv4 withdraws: %w", err)
            }
            fitIPv4W, remIPv4W = fit, rest
            available -= len(fit)
        }

        if len(remMPU) > 0 && available > 0 {
            // Extract AFI/SAFI for splitter lookup
            fit, rest, err := splitMPUnreach(remMPU, available, addPath)
            if err != nil {
                return nil, fmt.Errorf("split MP_UNREACH: %w", err)
            }
            if len(fit) > 0 {
                fitMPU, remMPU = fit, rest
                available -= len(fit)
            }
        }

        if len(remMPR) > 0 && available > 0 {
            fit, rest, err := splitMPReach(remMPR, available, addPath)
            if err != nil {
                return nil, fmt.Errorf("split MP_REACH: %w", err)
            }
            if len(fit) > 0 {
                fitMPR, remMPR = fit, rest
                available -= len(fit)
            }
        }

        if len(remIPv4A) > 0 && available > 0 {
            fit, rest, err := splitIPv4NLRIs(remIPv4A, available, addPath)
            if err != nil {
                return nil, fmt.Errorf("split IPv4 announces: %w", err)
            }
            if len(fit) > 0 {
                fitIPv4A, remIPv4A = fit, rest
            }
        }

        // Emit UPDATE
        payload := buildUpdatePayload(fitIPv4W, baseAttrs, fitMPU, fitMPR, fitIPv4A)
        results = append(results, NewWireUpdateFromPayload(payload, sourceCtx))
    }

    return results, nil
}

// buildUpdatePayload builds UPDATE body with any combination of components.
// Includes baseAttrs only if announces present (MP_REACH or IPv4 NLRI).
func buildUpdatePayload(ipv4Withdraws, baseAttrs, mpUnreach, mpReach, ipv4NLRI []byte) []byte {
    wLen := len(ipv4Withdraws)
    hasAnnounces := len(mpReach) > 0 || len(ipv4NLRI) > 0

    // Attrs: baseAttrs only if announces present
    var aLen int
    if hasAnnounces {
        aLen = len(baseAttrs) + len(mpUnreach) + len(mpReach)
    } else {
        aLen = len(mpUnreach)
    }

    buf := make([]byte, 2+wLen+2+aLen+len(ipv4NLRI))

    // Withdrawn routes
    binary.BigEndian.PutUint16(buf[0:], uint16(wLen))
    copy(buf[2:], ipv4Withdraws)

    // Attributes
    pos := 2 + wLen
    binary.BigEndian.PutUint16(buf[pos:], uint16(aLen))
    pos += 2

    if hasAnnounces {
        copy(buf[pos:], baseAttrs)
        pos += len(baseAttrs)
    }
    copy(buf[pos:], mpUnreach)
    pos += len(mpUnreach)
    copy(buf[pos:], mpReach)
    pos += len(mpReach)

    // NLRI
    copy(buf[pos:], ipv4NLRI)

    return buf
}
```

### 8. Update ForwardUpdateByID

```go
// In internal/reactor/reactor.go ForwardUpdateByID

if updateSize > maxMsgSize {
    // Split path: UPDATE too large for this peer
    maxBody := maxMsgSize - message.HeaderLen
    addPath := peer.Capabilities().AddPath(update.AFI, update.SAFI)
    splits, err := api.SplitUpdate(update.WireUpdate, maxBody, addPath)
    if err != nil {
        errs = append(errs, fmt.Errorf("peer %s: split failed: %w", peer.Settings().Address, err))
        continue
    }
    for _, split := range splits {
        if err := peer.SendRawUpdateBody(split.Payload()); err != nil {
            errs = append(errs, fmt.Errorf("peer %s: %w", peer.Settings().Address, err))
        }
    }
}
```

## Files to Modify

- `internal/bgp/message/nlri_split.go` - **NEW:** NLRI splitter registry and implementations
- `internal/bgp/message/nlri_split_test.go` - **NEW:** Tests for NLRI splitters
- `internal/plugin/wire_update_split.go` - **NEW:** SplitUpdate and helpers
- `internal/plugin/wire_update_split_test.go` - **NEW:** Split function tests
- `internal/reactor/reactor.go` - Update ForwardUpdateByID to use SplitUpdate
- `internal/reactor/received_update.go` - Delete `ConvertToRoutes()` and related fields
- `internal/reactor/received_update_test.go` - Delete `ConvertToRoutes` tests

## Edge Cases

| Case | Handling |
|------|----------|
| Single NLRI > maxSize | Return error (cannot split) |
| Empty UPDATE | Return original (fast path) |
| Only attributes (no NLRI) | Return original (fast path) |
| All components fit | Single UPDATE (fast path) |
| Multiple MP_REACH different AFI/SAFIs | Separate into multiple UPDATEs |
| ADD-PATH enabled | Include 4-byte path-id in NLRI boundary calc |
| Extended-length attr becomes short | Recalculate flags in buildMPAttribute |
| Malformed NLRI | Return error with offset |

## TDD Test Plan

### Unit Tests - NLRI Splitters

| Test | File | Validates |
|------|------|-----------|
| `TestPrefixSplitter_Empty` | `nlri_split_test.go` | Empty → 0 |
| `TestPrefixSplitter_IPv4` | `nlri_split_test.go` | /24 = 4 bytes total |
| `TestPrefixSplitter_IPv6` | `nlri_split_test.go` | /64 = 9 bytes total |
| `TestPrefixSplitter_AddPath` | `nlri_split_test.go` | +4 bytes for path-id |
| `TestPrefixSplitter_Truncated` | `nlri_split_test.go` | Error on short data |
| `TestVPNSplitter_Basic` | `nlri_split_test.go` | label+RD+prefix |
| `TestFlowSpecSplitter_ShortLen` | `nlri_split_test.go` | 1-byte length |
| `TestFlowSpecSplitter_LongLen` | `nlri_split_test.go` | 2-byte length (>=240) |

### Unit Tests - Split Functions

| Test | File | Validates |
|------|------|-----------|
| `TestSplitNLRIs_AllFit` | `wire_update_split_test.go` | No split needed |
| `TestSplitNLRIs_Partial` | `wire_update_split_test.go` | Split at boundary |
| `TestSplitNLRIs_FirstTooLarge` | `wire_update_split_test.go` | Error returned |
| `TestSplitNLRIs_AddPath` | `wire_update_split_test.go` | Respects path-id |
| `TestSeparateMPAttributes_Empty` | `wire_update_split_test.go` | Empty → empty |
| `TestSeparateMPAttributes_Mixed` | `wire_update_split_test.go` | Separates correctly |
| `TestSeparateMPAttributes_Multiple` | `wire_update_split_test.go` | Multiple MP_* |
| `TestSeparateMPAttributes_ExtendedLen` | `wire_update_split_test.go` | Handles ext len flag |
| `TestSplitMPReach_NoSplit` | `wire_update_split_test.go` | Fits → original |
| `TestSplitMPReach_Split` | `wire_update_split_test.go` | NextHop in both |
| `TestSplitMPReach_ExtendedToShort` | `wire_update_split_test.go` | Flag recalculated |
| `TestSplitMPUnreach_NoSplit` | `wire_update_split_test.go` | Fits → original |
| `TestSplitMPUnreach_Split` | `wire_update_split_test.go` | AFI/SAFI in both |
| `TestBuildUpdatePayload_WithdrawsOnly` | `wire_update_split_test.go` | No baseAttrs |
| `TestBuildUpdatePayload_Mixed` | `wire_update_split_test.go` | All components |
| `TestSplitUpdate_NoSplit` | `wire_update_split_test.go` | Small UPDATE as-is |
| `TestSplitUpdate_IPv4Only` | `wire_update_split_test.go` | Legacy format |
| `TestSplitUpdate_IPv6Only` | `wire_update_split_test.go` | MP_REACH only |
| `TestSplitUpdate_Mixed` | `wire_update_split_test.go` | IPv4 + IPv6 |
| `TestSplitUpdate_MultipleMPAttrs` | `wire_update_split_test.go` | Invalid → valid |
| `TestSplitUpdate_RFCCompliant` | `wire_update_split_test.go` | ≤1 MP_REACH, ≤1 MP_UNREACH |
| `TestSplitUpdate_AddPath` | `wire_update_split_test.go` | Path-id preserved |
| `TestSplitUpdate_VPNv4` | `wire_update_split_test.go` | Label+RD boundaries |

### Functional Tests

Existing `make functional` should pass - split behavior tested via ForwardUpdateByID.

## Implementation Steps

1. **Write NLRI splitter tests** - Create `nlri_split_test.go`
2. **Run tests** - Verify FAIL
3. **Implement NLRI splitters** - `nlri_split.go`
4. **Run tests** - Verify PASS
5. **Write split function tests** - Create `wire_update_split_test.go`
6. **Run tests** - Verify FAIL
7. **Implement split functions** - `wire_update_split.go`
8. **Run tests** - Verify PASS
9. **Update reactor** - ForwardUpdateByID
10. **Delete ConvertToRoutes** - Clean up old code
11. **Verify all** - `make lint && make test && make functional`

## RFC Documentation

- `// RFC 4271 Section 4.3` - UPDATE message format
- `// RFC 4760 Section 3` - MP_REACH_NLRI format
- `// RFC 4760 Section 4` - MP_UNREACH_NLRI format
- `// RFC 7911 Section 3` - ADD-PATH encoding

## Implementation Deviations

**From spec:** `SplitUpdate(wu *WireUpdate, maxBodySize int, addPath bool)`
**Actual:** `SplitWireUpdate(wu *WireUpdate, maxBodySize int, srcCtx *bgpctx.EncodingContext)`

**Reason:** ADD-PATH is negotiated per AFI/SAFI, not globally. Passing full context enables per-family lookup when splitting MP_REACH/MP_UNREACH attributes.

**Additional fixes during implementation:**
1. Added infinite loop guard when baseAttrs > maxSize
2. Added `madeProgress` tracking to detect stuck iterations
3. Added `maxBytes <= 0` validation in split functions

## Test Output

**Tests FAIL (before implementation):**
```
internal/plugin/wire_update_split_test.go:26:15: undefined: SplitWireUpdate
FAIL    codeberg.org/thomas-mangin/zebgp/internal/plugin [build failed]
```

**Tests PASS (after implementation):**
```
ok      codeberg.org/thomas-mangin/zebgp/internal/plugin    0.585s
```

**Verification:**
```
✅ make test: all packages pass
✅ make lint: no new issues in modified files
✅ make functional: 37 passed, 0 failed
```

## Checklist

### 🧪 TDD
- [x] NLRI splitter tests written (existing in chunk_mp_nlri.go)
- [x] NLRI splitter tests FAIL (N/A - leveraged existing)
- [x] NLRI splitters implemented (existing in chunk_mp_nlri.go)
- [x] NLRI splitter tests PASS
- [x] Split function tests written
- [x] Split function tests FAIL (verified - see output above)
- [x] Split functions implemented
- [x] Split function tests PASS (verified - see output above)

### Implementation
- [x] Add `NLRISplitter` interface and registry (existing ChunkMPNLRI)
- [x] Add `prefixNLRISplitter` (IPv4/IPv6 unicast) (existing basicNLRISize)
- [x] Add `vpnNLRISplitter` (VPNv4/v6) (existing vpnNLRISize)
- [x] Add `flowspecNLRISplitter` (existing flowSpecNLRISize)
- [x] Add `separateMPAttributes()` with error handling
- [x] Add `splitNLRIs()` with splitter param (using ChunkMPNLRI)
- [x] Add `splitMPReach()` with extended-length handling
- [x] Add `splitMPUnreach()` with extended-length handling
- [x] Add `buildMPAttribute()` with flag recalculation
- [x] Add `buildUpdatePayload()`
- [x] Add `buildCombinedUpdates()` with proper error propagation
- [x] Add `SplitWireUpdate()` main entry point
- [x] Update `ForwardUpdate` with wire-level split
- [x] Delete `ConvertToRoutes()` (N/A - already removed in prior commit)

### Verification
- [x] `make lint` passes (no new issues)
- [x] `make test` passes
- [x] `make functional` passes

### Documentation
- [x] Required docs read
- [x] RFC references added
- [x] `.claude/zebgp/` docs reviewed (no schema changes needed)

### Completion
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`

## Files Created/Modified

**New files:**
- `internal/plugin/wire_update_split.go` - Main split implementation
- `internal/plugin/wire_update_split_test.go` - 28 tests

**Modified files:**
- `internal/reactor/reactor.go` - ForwardUpdate uses SplitWireUpdate

## Known Limitations

1. **Re-encode path still uses single addPath bool:** The `message.SplitUpdateWithAddPath` function in the re-encode path accepts `addPath bool` not `*EncodingContext`. Marked with TODO for future fix.

---

**Created:** 2025-01-05
**Updated:** 2025-01-05 (implemented + reviewed)
**Status:** ✅ Complete - ready for `docs/plan/done/`
