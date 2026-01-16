---
paths:
  - "**/*.go"
---

# Test-Driven Development

**BLOCKING:** Tests must exist and fail before implementation.

## TDD Cycle

1. Write test with `VALIDATES:` and `PREVENTS:` comments
2. Run test → MUST FAIL (paste output)
3. Write minimum implementation
4. Run test → MUST PASS (paste output)
5. Refactor while green

## Test Documentation Required

```go
// TestFeatureName verifies [behavior].
//
// VALIDATES: [what correct behavior looks like]
// PREVENTS: [what bug this catches]
// REPRODUCES: (for bug fixes) [original issue description]
func TestFeatureName(t *testing.T) { ... }
```

## Table-Driven Test Pattern

```go
func TestParseHeader(t *testing.T) {
    tests := []struct {
        name    string
        input   []byte
        want    *Header
        wantErr error
    }{
        {
            name:  "valid_keepalive_header",
            input: []byte{0xFF, 0xFF, ...},
            want:  &Header{Type: TypeKEEPALIVE, Length: 19},
        },
        {
            name:    "truncated_header",
            input:   []byte{0xFF, 0xFF, 0xFF},
            wantErr: ErrShortRead,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := ParseHeader(tt.input)
            if tt.wantErr != nil {
                require.ErrorIs(t, err, tt.wantErr)
                return
            }
            require.NoError(t, err)
            assert.Equal(t, tt.want, got)
        })
    }
}
```

## Round-Trip Testing (Wire Format)

Every pack/unpack MUST pass round-trip:

```go
func TestHeaderRoundTrip(t *testing.T) {
    original := &Header{Marker: Marker, Length: 19, Type: TypeKEEPALIVE}
    packed := original.Pack()
    unpacked, err := ParseHeader(packed)
    require.NoError(t, err)
    assert.Equal(t, original, unpacked)
}
```

## Fuzzing (MANDATORY for Wire Format)

All code parsing external input MUST have fuzz tests.

| Code Type | Fuzz Required | Why |
|-----------|---------------|-----|
| Message parsing | YES | Untrusted network data |
| Attribute parsing | YES | Untrusted network data |
| NLRI parsing | YES | Untrusted network data |
| Config tokenizer | YES | User-provided file |
| CLI command parsing | YES | User commands |
| Internal-only code | Optional | No external input |

```go
// FuzzParseNLRI tests NLRI parsing robustness.
//
// VALIDATES: Parser handles arbitrary bytes without crashing.
// PREVENTS: Remote crash via malformed UPDATE, buffer overflow, panics.
// SECURITY: Critical - NLRI comes from untrusted BGP peers.
func FuzzParseNLRI(f *testing.F) {
    f.Add([]byte{24, 10, 0, 0})  // Valid: 10.0.0.0/24
    f.Add([]byte{})              // Empty
    f.Add([]byte{33, 10, 0, 0})  // Invalid prefix length
    f.Fuzz(func(t *testing.T, data []byte) {
        _, _ = ParseNLRI(data)  // MUST NOT panic
    })
}
```

## Coverage Requirements

| Code Type | Minimum Coverage |
|-----------|------------------|
| Wire format (pack/unpack) | 90%+ |
| Public functions | 100% |
| Error paths | 100% |

## Boundary Testing (MANDATORY)

**BLOCKING:** All numeric ranges MUST test 3 points:
1. **Last valid value** - highest/lowest value that should succeed
2. **First invalid below** - value just below valid range (if applicable)
3. **First invalid above** - value just above valid range

### Examples

| Range | Last Valid | First Invalid Below | First Invalid Above |
|-------|------------|---------------------|---------------------|
| Port (1-65535) | 65535 | 0 | 65536 |
| Hold time (0, 3+) | 0, 3 | 1, 2 | N/A (unbounded) |
| Prefix len IPv4 (0-32) | 32 | N/A (0 is valid) | 33 |
| Prefix len IPv6 (0-128) | 128 | N/A | 129 |
| ASN 2-byte (1-65535) | 65535 | 0 | 65536 |
| MPLS label (0-1048575) | 1048575 | N/A | 1048576 |
| DSCP (0-63) | 63 | N/A | 64 |
| AS_PATH segment (0-255) | 255 | N/A | 256 (auto-split) |
| Message length (19-4096) | 4096 | 18 | 4097 |

### Boundary Test Pattern

```go
func TestPortBoundary(t *testing.T) {
    // VALIDATES: Port validation accepts valid range, rejects invalid.
    // PREVENTS: Off-by-one errors in range validation.
    tests := []struct {
        name    string
        port    int
        wantErr bool
    }{
        // Last valid
        {"max_valid_65535", 65535, false},
        {"min_valid_1", 1, false},
        // First invalid below
        {"invalid_below_0", 0, true},
        // First invalid above
        {"invalid_above_65536", 65536, true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidatePort(tt.port)
            if tt.wantErr {
                require.Error(t, err)
            } else {
                require.NoError(t, err)
            }
        })
    }
}
```

### Boundary Analysis in Test Comments

Document boundary logic:

```go
// TestMPLSLabelBoundary verifies MPLS label validation.
//
// VALIDATES: Labels 0-1048575 (20-bit max 0xFFFFF) accepted.
// PREVENTS: Label overflow silently truncating to lower bits.
// BOUNDARY: 1048575 (valid), 1048576 (invalid - requires 21 bits).
func TestMPLSLabelBoundary(t *testing.T) { ... }
```

### When to Add Boundary Tests

| Situation | Required Tests |
|-----------|----------------|
| New numeric field | All 3 boundary points |
| New length field | Max valid, max+1 invalid |
| New enum/range | First/last valid, outside range |
| Bug fix for range | Test that catches the exact bug |

### Boundary Test Checklist

Before marking TDD complete:

```
[ ] All numeric inputs have boundary tests
[ ] Last valid value tested (should pass)
[ ] First invalid below tested (should fail)
[ ] First invalid above tested (should fail)
[ ] Boundary logic documented in test comment
```

## Non-Default Parameter Testing (MANDATORY)

**BLOCKING:** Always test with non-default parameter values.

### Why This Matters

Bugs hide when tests only use default values. Example:
- `New()` defaults to `idx=0`
- With `idx=0`, handle value equals slot value
- Bug in `p.slots[handle]` instead of `p.slots[handle.Slot()]` is masked
- Bug only manifests when `idx > 0`

### Rule

If a constructor or function has parameters with defaults or common values:
1. **Test with non-default values** - not just zero/empty/default
2. **Test with boundary values** - max valid, unusual but valid
3. **Test combinations** - multiple non-default params together

### Examples

```go
// BAD: Only tests default
func TestPool(t *testing.T) {
    p := New(1024)  // idx defaults to 0
    // ... bugs with idx>0 are hidden
}

// GOOD: Tests non-default values
func TestPoolWithIdx(t *testing.T) {
    p := NewWithIdx(5, 1024)  // Non-zero idx
    // ... exposes bugs in handle encoding
}
```

### Checklist

```
[ ] Constructors tested with non-default parameters
[ ] Optional parameters tested when provided
[ ] Config structs tested with non-zero field values
[ ] Mode/flag parameters tested in all valid states
```

## Quick Commands

```bash
go test -race ./pkg/bgp/message/... -v   # Single package
make test                                  # All tests
go test -fuzz=FuzzParseNLRI -fuzztime=30s ./pkg/bgp/nlri/...  # Fuzz
```

## Forbidden

- Implementation before test exists → Delete impl, write test
- Test that passes immediately → Invalid test, add failing assertion
- Claiming "done" without pasting test output → Run test, paste output
