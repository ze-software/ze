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
