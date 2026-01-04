# Spec: NLRI Wire Format Tests

## Task

Add comprehensive NLRI wire format tests to catch ADD-PATH encoding bugs like the one that led to the ADD-PATH simplification (spec 070).

## Required Reading (MUST complete before implementation)

- [ ] `plan/done/070-addpath-simplification.md` - Original problem and solution
- [ ] `pkg/bgp/nlri/len_test.go` - Existing consistency tests
- [ ] `pkg/bgp/nlri/base_len_test.go` - Existing WriteNLRI tests
- [ ] `pkg/bgp/nlri/evpn.go` - EVPN types (lines 1018-1075 for constructors)
- [ ] `rfc/rfc7911.txt` Section 3 - ADD-PATH NLRI encoding

**Key insight from original bug:**
- EVPN's `packEVPN()` assumed `Bytes()` included pathID when `hasPath=true`
- But `Bytes()` did NOT include pathID for EVPN types
- This caused wire format corruption when ADD-PATH was negotiated
- TDD missed this because EVPN wasn't in the consistency tests

## Implementation Steps

### 1. Add EVPN to Consistency Tests

Update `pkg/bgp/nlri/len_test.go` `TestLenWithContext_MatchesWriteNLRI_AllTypes`:

```go
// Add EVPN test cases (currently missing!)
{"EVPNType2_MAC", mustParseEVPNType2(t), true},  // EVPN supports ADD-PATH
{"EVPNType5_Prefix", mustParseEVPNType5(t), true},
```

Add helper functions:

```go
func mustParseEVPNType2(t *testing.T) *EVPNType2 {
    t.Helper()
    rd := RouteDistinguisher{Type: 0, Value: [6]byte{0, 0, 0, 0, 0, 1}}
    return NewEVPNType2(rd, [10]byte{}, 0, [6]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55},
        netip.MustParseAddr("10.0.0.1"), []uint32{100})
}

func mustParseEVPNType5(t *testing.T) *EVPNType5 {
    t.Helper()
    rd := RouteDistinguisher{Type: 0, Value: [6]byte{0, 0, 0, 0, 0, 1}}
    return NewEVPNType5(rd, [10]byte{}, 0, netip.MustParsePrefix("10.0.0.0/24"),
        netip.Addr{}, []uint32{100})
}
```

### 2. Add Wire Format Verification Tests

Create `pkg/bgp/nlri/wire_format_test.go`:

```go
// TestWireFormat_AddPath verifies actual wire bytes match expected format.
//
// VALIDATES: Wire format is [pathID][payload] when AddPath=true.
// PREVENTS: Path ID in wrong position or missing entirely.
func TestWireFormat_AddPath(t *testing.T) {
    tests := []struct {
        name     string
        nlri     NLRI
        ctx      *PackContext
        wantHex  string // Expected wire format in hex
    }{
        {
            name:    "INET_10.0.0.0/24_noAddPath",
            nlri:    NewINET(IPv4Unicast, netip.MustParsePrefix("10.0.0.0/24"), 42),
            ctx:     &PackContext{AddPath: false},
            wantHex: "180a0000", // Just prefix, no path ID
        },
        {
            name:    "INET_10.0.0.0/24_withAddPath",
            nlri:    NewINET(IPv4Unicast, netip.MustParsePrefix("10.0.0.0/24"), 42),
            ctx:     &PackContext{AddPath: true},
            wantHex: "0000002a180a0000", // pathID=42 + prefix
        },
        // Add EVPN, IPVPN, LabeledUnicast cases
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            buf := make([]byte, 100)
            n := WriteNLRI(tt.nlri, buf, 0, tt.ctx)
            got := hex.EncodeToString(buf[:n])
            if got != tt.wantHex {
                t.Errorf("wire format = %s, want %s", got, tt.wantHex)
            }
        })
    }
}
```

### 3. Add Round-Trip Tests

```go
// TestRoundTrip_NLRI verifies encode → decode → encode produces identical bytes.
//
// VALIDATES: Parsing preserves all NLRI data including path ID.
// PREVENTS: Data loss during parse/pack cycle.
func TestRoundTrip_NLRI(t *testing.T) {
    tests := []struct {
        name    string
        nlri    NLRI
        addPath bool
    }{
        {"INET_IPv4", NewINET(IPv4Unicast, netip.MustParsePrefix("10.0.0.0/24"), 0), false},
        {"INET_IPv4_AddPath", NewINET(IPv4Unicast, netip.MustParsePrefix("10.0.0.0/24"), 42), true},
        // Add all NLRI types including EVPN
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            ctx := &PackContext{AddPath: tt.addPath}

            // Encode
            buf := make([]byte, 100)
            n := WriteNLRI(tt.nlri, buf, 0, ctx)
            wire := buf[:n]

            // Decode (need to call appropriate Parse function based on type)
            parsed, _, err := parseNLRI(tt.nlri.Family(), wire, tt.addPath)
            if err != nil {
                t.Fatalf("parse failed: %v", err)
            }

            // Re-encode
            buf2 := make([]byte, 100)
            n2 := WriteNLRI(parsed, buf2, 0, ctx)
            wire2 := buf2[:n2]

            // Compare
            if !bytes.Equal(wire, wire2) {
                t.Errorf("round-trip mismatch:\n  orig: %x\n  trip: %x", wire, wire2)
            }
        })
    }
}
```

## Files to Modify

- `pkg/bgp/nlri/len_test.go` - Add EVPN to existing tests
- `pkg/bgp/nlri/wire_format_test.go` - New file for wire format tests

## TDD Workflow

1. Write test for EVPN consistency → should PASS (after Phase 3 fix)
2. Write wire format test with expected hex → verify against RFC
3. Write round-trip test → verify data preservation
4. Run `make test && make lint`

## Checklist

- [ ] Read required docs
- [ ] Add EVPN to `TestLenWithContext_MatchesWriteNLRI_AllTypes`
- [ ] Add `TestWireFormat_AddPath` with hex verification
- [ ] Add `TestRoundTrip_NLRI` for all types
- [ ] Verify EVPN types work correctly
- [ ] make test passes
- [ ] make lint passes

## Why This Matters

The original EVPN bug existed because:
1. EVPN wasn't in consistency tests
2. No wire format verification (actual bytes vs expected)
3. No round-trip testing

These tests would have caught the bug immediately.
