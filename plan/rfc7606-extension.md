# RFC 7606 Extension Plan

**Purpose:** Complete RFC 7606 compliance by addressing gaps identified in critical review.

**Current Status:** ~80% compliant (Phase 1 complete)

**Target:** Full RFC 7606 Section 7 compliance

---

## Phase 1: Critical Validations ✅ COMPLETE

**Completed:** 2024-12-22

**Tests Added:** 25 new tests in `rfc7606_test.go`

**Known Limitation:** AS_PATH validation uses hardcoded `asn4=false` (2-byte ASN).
This will be parameterized in Phase 3 when `ValidateUpdateRFC7606` signature is updated.

### 1.1 AS_PATH Segment Validation (RFC 7606 Section 7.2)

**Gap:** AS_PATH only tracked for presence, no content validation.

**RFC Requirements:**
- Unrecognized segment type → treat-as-withdraw
- Segment overrun (length exceeds remaining data) → treat-as-withdraw
- Segment underrun (< 1 octet after last segment) → treat-as-withdraw
- Zero segment length → treat-as-withdraw

**Implementation:**
```go
// In validateAttribute() for attrCodeASPath:
func validateASPath(data []byte, asn4 bool) *RFC7606ValidationResult {
    pos := 0
    for pos < len(data) {
        if pos+2 > len(data) {
            // Underrun: not enough for segment header
            return treatAsWithdraw(attrCodeASPath, "segment underrun")
        }
        segType := data[pos]
        segLen := int(data[pos+1])
        pos += 2

        // Validate segment type (1=AS_SET, 2=AS_SEQUENCE)
        if segType != 1 && segType != 2 {
            return treatAsWithdraw(attrCodeASPath, "unrecognized segment type")
        }

        // Validate segment length > 0
        if segLen == 0 {
            return treatAsWithdraw(attrCodeASPath, "zero segment length")
        }

        // Calculate AS size (2 or 4 bytes)
        asSize := 2
        if asn4 { asSize = 4 }

        // Check for overrun
        if pos + segLen*asSize > len(data) {
            return treatAsWithdraw(attrCodeASPath, "segment overrun")
        }
        pos += segLen * asSize
    }
    return nil
}
```

**Files:** `pkg/bgp/message/rfc7606.go`

**Tests:**
- `TestRFC7606ASPathSegmentOverrun`
- `TestRFC7606ASPathSegmentUnderrun`
- `TestRFC7606ASPathZeroSegmentLength`
- `TestRFC7606ASPathUnrecognizedSegmentType`
- `TestRFC7606ASPathValidSequence`
- `TestRFC7606ASPathValidSet`

---

### 1.2 ORIGIN Value Validation (RFC 7606 Section 7.1)

**Gap:** Length checked but value (0, 1, 2) not validated.

**RFC Requirements:**
- Value must be 0 (IGP), 1 (EGP), or 2 (INCOMPLETE)
- Undefined value → treat-as-withdraw

**Implementation:**
```go
// In validateAttribute() for attrCodeOrigin:
case attrCodeOrigin:
    if length != 1 {
        return treatAsWithdraw(code, "length != 1")
    }
    // Validate value
    if attrData[0] > 2 {
        return treatAsWithdraw(code, fmt.Sprintf("invalid value %d", attrData[0]))
    }
```

**Change Required:** Use `attrData` parameter (currently ignored as `_`).

**Tests:**
- `TestRFC7606OriginValueIGP` (value=0, valid)
- `TestRFC7606OriginValueEGP` (value=1, valid)
- `TestRFC7606OriginValueIncomplete` (value=2, valid)
- `TestRFC7606OriginValueInvalid` (value=3+, treat-as-withdraw)

---

### 1.3 MP_REACH_NLRI Next-Hop Length (RFC 7606 Section 7.11)

**Gap:** Not validated.

**RFC Requirements:**
- Inconsistent next-hop length → session-reset or AFI/SAFI disable
- Expected lengths vary by AFI/SAFI:
  - IPv4 unicast: 4 bytes
  - IPv6 unicast: 16 bytes (or 32 for link-local)
  - VPNv4: 12 bytes (RD + IPv4)
  - VPNv6: 24 bytes (RD + IPv6)

**Implementation:**
```go
case attrCodeMPReachNLRI:
    mpReachCount++
    hasNextHop = true

    // Validate MP_REACH structure
    if length < 5 {
        return sessionReset("MP_REACH_NLRI too short")
    }

    afi := binary.BigEndian.Uint16(attrData[0:2])
    safi := attrData[2]
    nhLen := int(attrData[3])

    // Validate next-hop length per AFI/SAFI
    if !isValidNextHopLength(afi, safi, nhLen) {
        return sessionReset(fmt.Sprintf("invalid next-hop length %d for AFI=%d SAFI=%d", nhLen, afi, safi))
    }
```

**Tests:**
- `TestRFC7606MPReachIPv4NextHopValid` (4 bytes)
- `TestRFC7606MPReachIPv4NextHopInvalid` (wrong length)
- `TestRFC7606MPReachIPv6NextHopValid` (16 or 32 bytes)
- `TestRFC7606MPReachVPNv4NextHopValid` (12 bytes)

---

### 1.4 NLRI Syntactic Validation (RFC 7606 Section 5.3)

**Gap:** Not implemented.

**RFC Requirements:**
- IPv4 NLRI prefix length ≤ 32 bits
- IPv6 NLRI prefix length ≤ 128 bits
- NLRI must not overrun field bounds

**Implementation:**
```go
func ValidateNLRISyntax(nlri []byte, isIPv6 bool) *RFC7606ValidationResult {
    maxLen := 32
    if isIPv6 { maxLen = 128 }

    pos := 0
    for pos < len(nlri) {
        if pos >= len(nlri) {
            return treatAsWithdraw(0, "NLRI overrun")
        }
        prefixLen := int(nlri[pos])
        if prefixLen > maxLen {
            return treatAsWithdraw(0, fmt.Sprintf("prefix length %d > %d", prefixLen, maxLen))
        }
        prefixBytes := (prefixLen + 7) / 8
        pos += 1 + prefixBytes
        if pos > len(nlri) {
            return treatAsWithdraw(0, "NLRI overrun")
        }
    }
    return nil
}
```

**Tests:**
- `TestRFC7606NLRIPrefixLengthValid`
- `TestRFC7606NLRIPrefixLengthTooLong`
- `TestRFC7606NLRIOverrun`

---

## Phase 2: IBGP Context

### 2.1 Add Peer Type to Validator

**Gap:** Validator doesn't know if peer is IBGP or EBGP.

**Change Function Signature:**
```go
// Before:
func ValidateUpdateRFC7606(pathAttrs []byte, hasNLRI bool) *RFC7606ValidationResult

// After:
func ValidateUpdateRFC7606(pathAttrs []byte, hasNLRI, isIBGP bool) *RFC7606ValidationResult
```

**Session Integration:**
```go
// session.go:validateUpdateRFC7606()
isIBGP := s.settings.IsIBGP()
result := message.ValidateUpdateRFC7606(pathAttrs, hasNLRI, isIBGP)
```

---

### 2.2 LOCAL_PREF Context (RFC 7606 Section 7.5)

**Current:** Always treat-as-withdraw for length != 4.

**RFC Requirement:**
- From EBGP → attribute-discard (ignore)
- From IBGP with length != 4 → treat-as-withdraw

```go
case attrCodeLocalPref:
    if !isIBGP {
        // EBGP: discard attribute per RFC 7606 Section 7.5
        return attributeDiscard(code, "LOCAL_PREF from EBGP")
    }
    if length != 4 {
        return treatAsWithdraw(code, "length != 4")
    }
```

---

### 2.3 ORIGINATOR_ID Context (RFC 7606 Section 7.9)

**Current:** Always treat-as-withdraw for length != 4.

**RFC Requirement:**
- From EBGP → attribute-discard
- From IBGP with length != 4 → treat-as-withdraw

```go
case attrCodeOriginatorID:
    if !isIBGP {
        return attributeDiscard(code, "ORIGINATOR_ID from EBGP")
    }
    if length != 4 {
        return treatAsWithdraw(code, "length != 4")
    }
```

---

### 2.4 CLUSTER_LIST Context (RFC 7606 Section 7.10)

**Current:** Always treat-as-withdraw for invalid length.

**RFC Requirement:**
- From EBGP → attribute-discard
- From IBGP with length not multiple of 4 → treat-as-withdraw

```go
case attrCodeClusterList:
    if !isIBGP {
        return attributeDiscard(code, "CLUSTER_LIST from EBGP")
    }
    if length == 0 || length%4 != 0 {
        return treatAsWithdraw(code, "length not multiple of 4")
    }
```

---

## Phase 3: Additional Validations

### 3.1 Attribute Flags Validation (RFC 7606 Section 3.c)

**Gap:** Not implemented.

**RFC Requirement:** If Optional or Transitive bits conflict with spec → treat-as-withdraw.

```go
func validateAttributeFlags(code uint8, flags uint8) *RFC7606ValidationResult {
    optional := (flags & 0x80) != 0
    transitive := (flags & 0x40) != 0

    // Well-known mandatory: must NOT be optional
    wellKnownMandatory := map[uint8]bool{
        attrCodeOrigin: true, attrCodeASPath: true, attrCodeNextHop: true,
    }

    if wellKnownMandatory[code] && optional {
        return treatAsWithdraw(code, "well-known mandatory marked as optional")
    }

    // Well-known: must be transitive
    wellKnown := map[uint8]bool{
        attrCodeOrigin: true, attrCodeASPath: true, attrCodeNextHop: true,
        attrCodeMED: true, attrCodeLocalPref: true, attrCodeAtomicAgg: true,
    }

    if wellKnown[code] && !transitive {
        return treatAsWithdraw(code, "well-known attribute not transitive")
    }

    return nil
}
```

---

### 3.2 Multiple Attribute Handling (RFC 7606 Section 3.g)

**Gap:** Only MP_REACH/MP_UNREACH counted; other duplicates not handled.

**RFC Requirement:**
- Multiple MP_REACH or MP_UNREACH → session-reset
- Multiple of other attributes → discard all but first

```go
// Track all attribute occurrences
seenCodes := make(map[uint8]bool)

for pos < len(pathAttrs) {
    // ... parse attribute header ...

    if seenCodes[attrCode] {
        if attrCode == attrCodeMPReachNLRI || attrCode == attrCodeMPUnreachNLRI {
            return sessionReset("multiple MP_REACH/MP_UNREACH")
        }
        // Other duplicate: skip (discard), continue parsing
        pos += attrLen
        continue
    }
    seenCodes[attrCode] = true

    // ... validate attribute ...
}
```

---

### 3.3 4-Octet AS Context for AGGREGATOR (RFC 7606 Section 7.7)

**Gap:** Validates length 6 or 8, doesn't check against negotiated capability.

**RFC Requirement:**
- Length 6 when 4-octet AS NOT negotiated
- Length 8 when 4-octet AS negotiated

```go
// Add asn4 parameter to validator
func ValidateUpdateRFC7606(pathAttrs []byte, hasNLRI, isIBGP, asn4 bool) ...

case attrCodeAggregator:
    expectedLen := 6
    if asn4 { expectedLen = 8 }
    if length != expectedLen {
        return attributeDiscard(code, fmt.Sprintf("length %d, expected %d", length, expectedLen))
    }
```

---

## Implementation Order

| Priority | Item | Effort | Impact | Status |
|----------|------|--------|--------|--------|
| 🔴 1 | AS_PATH segment validation | 2h | Critical | ✅ Done |
| 🔴 2 | ORIGIN value validation | 30m | Critical | ✅ Done |
| 🔴 3 | MP_REACH next-hop length | 1h | Critical | ✅ Done |
| 🔴 4 | NLRI syntactic validation | 1h | Critical | ✅ Done |
| 🟡 5 | IBGP context (Phase 2) | 2h | Medium | ⏳ Pending |
| 🟢 6 | Attribute flags | 1h | Low | ⏳ Pending |
| 🟢 7 | Multiple attribute handling | 1h | Low | ⏳ Pending |
| 🟢 8 | 4-octet AS for AGGREGATOR/AS_PATH | 30m | Low | ⏳ Pending |

**Total Estimated Effort:** 8-10 hours (Phase 1: ~4h complete)

---

## Testing Strategy

### Unit Tests (rfc7606_test.go)
- One test per validation rule
- Both valid and invalid cases
- Edge cases (boundary lengths, empty data)

### Integration Tests (session_test.go)
- End-to-end malformed UPDATE handling
- Verify session stays up for treat-as-withdraw
- Verify session resets for session-reset actions

### Fuzz Testing (optional)
- Random attribute data to find edge cases
- Ensure no panics on malformed input

---

## Verification Checklist

After implementation, verify against RFC 7606 Section 7:

- [x] 7.1 ORIGIN: length=1, value 0-2 ✅ Phase 1
- [x] 7.2 AS_PATH: segment structure valid ✅ Phase 1 (asn4=false, deferred to Phase 3)
- [x] 7.3 NEXT_HOP: length=4 ✅ pre-existing
- [x] 7.4 MED: length=4 ✅ pre-existing
- [ ] 7.5 LOCAL_PREF: EBGP=discard, IBGP=length=4 ⏳ Phase 2
- [x] 7.6 ATOMIC_AGGREGATE: length=0 ✅ pre-existing
- [x] 7.7 AGGREGATOR: length=6|8 ✅ pre-existing (ASN4 context deferred to Phase 3)
- [x] 7.8 Community: length % 4 = 0 ✅ pre-existing
- [ ] 7.9 ORIGINATOR_ID: EBGP=discard, IBGP=length=4 ⏳ Phase 2
- [ ] 7.10 CLUSTER_LIST: EBGP=discard, IBGP=length % 4 = 0 ⏳ Phase 2
- [x] 7.11 MP_REACH_NLRI: next-hop length valid for AFI/SAFI ✅ Phase 1
- [x] 7.14 Extended Community: length % 8 = 0 ✅ pre-existing
- [x] 5.3 NLRI: prefix length valid, no overrun ✅ Phase 1

---

## Notes

- All changes should follow TDD (test first)
- Run `make test && make lint` before each commit
- Update `plan/exabgp-alignment.md` when complete
- Consider ExaBGP behavior for compatibility
