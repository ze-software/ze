# RFC 7606 Extension Plan

**Purpose:** Complete RFC 7606 compliance by addressing gaps identified in critical review.

**Current Status:** ✅ COMPLETE - Full RFC 7606 Section 7 compliance

**Target:** Full RFC 7606 Section 7 compliance

---

## Phase 1: Critical Validations ✅ COMPLETE

**Completed:** 2024-12-22

**Tests Added:** 25 new tests in `rfc7606_test.go`

### 1.1 AS_PATH Segment Validation (RFC 7606 Section 7.2)

- Unrecognized segment type → treat-as-withdraw
- Segment overrun/underrun → treat-as-withdraw
- Zero segment length → treat-as-withdraw
- Supports CONFED segment types (3, 4)

### 1.2 ORIGIN Value Validation (RFC 7606 Section 7.1)

- Value must be 0 (IGP), 1 (EGP), or 2 (INCOMPLETE)
- Undefined value → treat-as-withdraw

### 1.3 MP_REACH_NLRI Next-Hop Length (RFC 7606 Section 7.11)

- Validates next-hop length per AFI/SAFI
- Supports IPv4, IPv6, VPNv4, VPNv6, MPLS
- RFC 5549 extended next-hop supported

### 1.4 NLRI Syntactic Validation (RFC 7606 Section 5.3)

- IPv4 prefix length ≤ 32, IPv6 ≤ 128
- NLRI overrun detection

---

## Phase 2: IBGP Context ✅ COMPLETE

**Completed:** 2024-12-22

**Tests Added:** 9 new tests

### 2.1 Function Signature Update

```go
func ValidateUpdateRFC7606(pathAttrs []byte, hasNLRI, isIBGP bool) *RFC7606ValidationResult
```

### 2.2 LOCAL_PREF Context (RFC 7606 Section 7.5)

- EBGP → attribute-discard
- IBGP with length != 4 → treat-as-withdraw

### 2.3 ORIGINATOR_ID Context (RFC 7606 Section 7.9)

- EBGP → attribute-discard
- IBGP with length != 4 → treat-as-withdraw

### 2.4 CLUSTER_LIST Context (RFC 7606 Section 7.10)

- EBGP → attribute-discard
- IBGP with length not multiple of 4 → treat-as-withdraw

---

## Phase 3: Additional Validations ✅ COMPLETE

**Completed:** 2024-12-22

**Tests Added:** 11 new tests

### 3.1 Attribute Flags Validation (RFC 7606 Section 3.c)

- Well-known attributes must NOT have Optional bit set
- Well-known attributes MUST have Transitive bit set
- Violation → treat-as-withdraw

### 3.2 Multiple Attribute Handling (RFC 7606 Section 3.g)

- Multiple MP_REACH or MP_UNREACH → session-reset
- Other duplicates → silently discard all but first

### 3.3 4-Octet AS Context (RFC 7606 Section 7.7, 7.2)

```go
func ValidateUpdateRFC7606(pathAttrs []byte, hasNLRI, isIBGP, asn4 bool) *RFC7606ValidationResult
```

- AGGREGATOR: length 6 (asn4=false) or 8 (asn4=true)
- AS_PATH: segment validation uses correct ASN size
- session.go passes `neg.ASN4` from negotiated capabilities

---

## Implementation Summary

| Priority | Item | Status |
|----------|------|--------|
| 🔴 1 | AS_PATH segment validation | ✅ Done |
| 🔴 2 | ORIGIN value validation | ✅ Done |
| 🔴 3 | MP_REACH next-hop length | ✅ Done |
| 🔴 4 | NLRI syntactic validation | ✅ Done |
| 🟡 5 | IBGP context (Phase 2) | ✅ Done |
| 🟢 6 | Attribute flags | ✅ Done |
| 🟢 7 | Multiple attribute handling | ✅ Done |
| 🟢 8 | 4-octet AS for AGGREGATOR/AS_PATH | ✅ Done |

**Total Tests Added:** 45 new tests across all phases

---

## Verification Checklist ✅ ALL COMPLETE

| Section | Requirement | Status |
|---------|-------------|--------|
| 3.c | Attribute flags validation | ✅ |
| 3.d | Missing mandatory attributes | ✅ |
| 3.g | Duplicate attribute handling | ✅ |
| 5.3 | NLRI syntactic validation | ✅ |
| 7.1 | ORIGIN: length=1, value 0-2 | ✅ |
| 7.2 | AS_PATH: segment structure valid | ✅ |
| 7.3 | NEXT_HOP: length=4 | ✅ |
| 7.4 | MED: length=4 | ✅ |
| 7.5 | LOCAL_PREF: EBGP=discard, IBGP=length=4 | ✅ |
| 7.6 | ATOMIC_AGGREGATE: length=0 | ✅ |
| 7.7 | AGGREGATOR: length per asn4 capability | ✅ |
| 7.8 | Community: length % 4 = 0 | ✅ |
| 7.9 | ORIGINATOR_ID: EBGP=discard, IBGP=length=4 | ✅ |
| 7.10 | CLUSTER_LIST: EBGP=discard, IBGP=length % 4 = 0 | ✅ |
| 7.11 | MP_REACH_NLRI: next-hop length valid | ✅ |
| 7.14 | Extended Community: length % 8 = 0 | ✅ |

---

## Files Modified

- `pkg/bgp/message/rfc7606.go` - Core validation logic
- `pkg/bgp/message/rfc7606_test.go` - 68 total tests
- `pkg/reactor/session.go` - Integration with negotiated capabilities

---

## Commits

1. `bc7c32c` - Implement RFC 7606 Phase 1: critical attribute validations
2. `a44a6c1` - Implement RFC 7606 Phase 2: IBGP context validation
3. `b0abb6c` - Implement RFC 7606 Phase 3: flags, duplicates, and 4-octet AS
