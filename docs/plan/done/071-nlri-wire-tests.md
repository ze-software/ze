# Spec: NLRI Wire Format Tests

## Task

Add comprehensive NLRI wire format tests to catch ADD-PATH encoding bugs like the one that led to the ADD-PATH simplification (spec 070).

## Required Reading (completed)

- [x] `docs/plan/done/070-addpath-simplification.md` - Original problem and solution
- [x] `internal/bgp/nlri/len_test.go` - Existing consistency tests
- [x] `internal/bgp/nlri/base_len_test.go` - Existing WriteNLRI tests
- [x] `internal/bgp/nlri/evpn.go` - EVPN types (lines 1018-1075 for constructors)
- [x] `rfc/rfc7911.txt` Section 3 - ADD-PATH NLRI encoding

**Key insight from original bug:**
- EVPN's `packEVPN()` assumed `Bytes()` included pathID when `hasPath=true`
- But `Bytes()` did NOT include pathID for EVPN types
- This caused wire format corruption when ADD-PATH was negotiated
- TDD missed this because EVPN wasn't in the consistency tests

## Files Modified

- `internal/bgp/nlri/len_test.go` - Added EVPN test cases
- `internal/bgp/nlri/wire_format_test.go` - New file (7 test functions)
- `internal/bgp/nlri/evpn.go` - Fixed gateway encoding bug (line 919)

## Bug Found During Implementation

**EVPN Type 5 gateway panic** (`evpn.go:919`):
- `e.gateway.As4()` panicked when gateway was empty `netip.Addr{}`
- Fixed by checking `e.gateway.IsValid()` before As4()/As16()
- Per RFC 9136 Section 3.1: Gateway IP is 0 when not used

## Implementation

### 1. EVPN Added to Consistency Tests

`internal/bgp/nlri/len_test.go`:
```go
{"EVPNType2_MAC", mustParseEVPNType2(t), true},
{"EVPNType5_Prefix", mustParseEVPNType5(t), true},
```

### 2. Wire Format Verification Tests

`internal/bgp/nlri/wire_format_test.go`:
- `TestWireFormat_AddPath` - INET with/without ADD-PATH, hex verification
- `TestWireFormat_IPVPN` - VPN label encoding (RFC 4364/4659)
- `TestWireFormat_LabeledUnicast` - Labeled unicast (RFC 8277)
- `TestWireFormat_EVPN` - EVPN Type 2/5 length verification (RFC 7432)

### 3. Round-Trip Tests

- `TestRoundTrip_INET` - IPv4/IPv6 encode→decode→encode
- `TestRoundTrip_IPVPN` - VPN routes with RD/labels
- `TestRoundTrip_EVPN` - EVPN Type 2/5 routes

## Checklist

- [x] Read required docs
- [x] Add EVPN to `TestLenWithContext_MatchesWriteNLRI_AllTypes`
- [x] Add `TestWireFormat_AddPath` with hex verification
- [x] Add `TestRoundTrip_NLRI` for all types
- [x] Verify EVPN types work correctly
- [x] make test passes
- [x] make lint passes (nlri package: 0 issues)

## Why This Matters

The original EVPN bug existed because:
1. EVPN wasn't in consistency tests
2. No wire format verification (actual bytes vs expected)
3. No round-trip testing

These tests would have caught the bug immediately.
