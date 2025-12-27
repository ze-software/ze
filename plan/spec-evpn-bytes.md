# Spec: EVPN Bytes() Methods

## Task

Implement EVPN `Bytes()` methods for all 5 route types (currently return nil), blocking EVPN route announcements.

## Embedded Protocol Requirements

### Default Rules (ALL tasks)
- Tests MUST exist and FAIL before implementation code exists
- Run `make test && make lint` before claiming done
- NEVER discard uncommitted work without explicit user permission
- Verify before claiming: run commands, paste output as proof
- For BGP code: Read RFC first, check ExaBGP reference
- Tests passing is NOT permission to commit - wait for user

### From TDD_ENFORCEMENT.md
- Write test with VALIDATES/PREVENTS documentation
- Run test → MUST FAIL first (paste output)
- Write minimum implementation to pass
- Run test → MUST PASS (paste output)
- Every test function must document what it validates and prevents

### From RFC_DOCUMENTATION_PROTOCOL.md
- Wire format already documented in evpn.go (good)
- Tests should use known wire bytes for round-trip verification
- Pack/Unpack round-trip tests required

### From NLRI_EVPN.md
- Common header: `[type:1][length:1][payload...]`
- Type 1: RD(8) + ESI(10) + EthTag(4) + Labels(3+)
- Type 2: RD(8) + ESI(10) + EthTag(4) + MACLen(1) + MAC(6) + IPLen(1) + IP(0/4/16) + Labels(3+)
- Type 3: RD(8) + EthTag(4) + IPLen(1) + IP(4/16)
- Type 4: RD(8) + ESI(10) + IPLen(1) + IP(4/16)
- Type 5: RD(8) + ESI(10) + EthTag(4) + PrefixLen(1) + Prefix(4/16) + GW(4/16) + Labels(3)

## Codebase Context

### Existing Files
- `pkg/bgp/nlri/evpn.go` - EVPN types with parsing (Bytes() returns nil)
- `pkg/bgp/nlri/evpn_test.go` - Existing parse tests
- `pkg/bgp/nlri/ipvpn.go` - Has `EncodeLabelStack()` and `RouteDistinguisher.Bytes()`

### Existing Helpers
```go
// RouteDistinguisher.Bytes() - returns 8 bytes
func (rd RouteDistinguisher) Bytes() []byte

// EncodeLabelStack - encodes labels with BOS bit
func EncodeLabelStack(labels []uint32) []byte
```

### Pattern to Follow
From `pkg/bgp/nlri/other.go` (VPLS.Bytes):
```go
func (v *VPLS) Bytes() []byte {
    buf := make([]byte, 19)
    binary.BigEndian.PutUint16(buf[0:2], 17) // Length
    copy(buf[2:10], v.rd.Bytes())
    // ... encode fields
    return buf
}
```

## Implementation Steps

### Step 1: Add round-trip tests for all 5 types
- **File:** `pkg/bgp/nlri/evpn_test.go`
- **Tests:** `TestEVPNType1RoundTrip`, `TestEVPNType2RoundTrip`, etc.
- **Pattern:** Parse known bytes → call Bytes() → compare with original
- **MUST FAIL** before implementation

### Step 2: Implement EVPNType1.Bytes()
Wire format (25+ bytes):
```
[type:1][len:1][RD:8][ESI:10][EthTag:4][Labels:3+]
```

### Step 3: Implement EVPNType2.Bytes()
Wire format (33-54 bytes):
```
[type:1][len:1][RD:8][ESI:10][EthTag:4][MACLen:1][MAC:6][IPLen:1][IP:0/4/16][Labels:3+]
```

### Step 4: Implement EVPNType3.Bytes() and fix Len()
Wire format (17/29 bytes):
```
[type:1][len:1][RD:8][EthTag:4][IPLen:1][IP:4/16]
```

### Step 5: Implement EVPNType4.Bytes()
Wire format (23/35 bytes):
```
[type:1][len:1][RD:8][ESI:10][IPLen:1][IP:4/16]
```

### Step 6: Implement EVPNType5.Bytes() and fix Len()
Wire format (34/58 bytes per RFC 9136):
```
[type:1][len:1][RD:8][ESI:10][EthTag:4][PrefixLen:1][Prefix:4/16][GW:4/16][Labels:3]
```

## Verification Checklist

- [ ] Round-trip tests written for all 5 types
- [ ] Tests shown to FAIL first (paste output)
- [ ] All 5 Bytes() methods implemented
- [ ] Tests shown to PASS (paste output)
- [ ] EVPNType3.Len() and EVPNType5.Len() fixed (currently return 0)
- [ ] `make test` passes
- [ ] `make lint` passes
- [ ] Self-review performed

## Test Documentation Template

```go
// TestEVPNTypeNRoundTrip verifies that parsing and encoding are symmetric.
//
// VALIDATES: Bytes() produces wire format that ParseEVPN() can read back.
//
// PREVENTS: Asymmetric encoding that would corrupt routes on re-advertisement.
func TestEVPNTypeNRoundTrip(t *testing.T) {
    // Build wire bytes manually
    // Parse with ParseEVPN()
    // Call Bytes()
    // Compare original == encoded
}
```

---

**Created:** 2025-12-27
