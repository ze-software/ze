# Testing Protocol: TDD-First Development

**BLOCKING RULE: Tests MUST be written BEFORE implementation.**

See `TDD_ENFORCEMENT.md` for complete workflow.

---

## The TDD Cycle (MANDATORY)

```
   ┌──────────────────────────────────────────────────────────────┐
   │                                                              │
   │   1. WRITE TEST (with documentation) → 2. TEST FAILS →      │
   │   3. WRITE IMPLEMENTATION → 4. TEST PASSES → 5. REFACTOR    │
   │                                                              │
   │   Skipping step 1 or 2 is a PROTOCOL VIOLATION              │
   │                                                              │
   └──────────────────────────────────────────────────────────────┘
```

---

## Test Documentation Requirements

**Every test MUST document what it prevents:**

```go
// TestInternDeduplication verifies identical data returns same handle.
//
// VALIDATES: Memory efficiency through deduplication.
//
// PREVENTS: Memory bloat - without this, 1M routes with same AS_PATH
// would store 1M copies instead of 1 with refCount=1M.
func TestInternDeduplication(t *testing.T) {
    // ...
}
```

**Required sections:**
- First line: What the test verifies
- `VALIDATES:` What correct behavior looks like
- `PREVENTS:` What bug/failure this catches
- `REPRODUCES:` (for bug fixes) Original issue description

---

## Development Workflow

### Step 1: Write Test First

```go
// internal/pool/pool_test.go
func TestGet_ReturnsInternedData(t *testing.T) {
    // This test will fail - Pool doesn't exist yet
    p := pool.New(1024)
    h := p.Intern([]byte("hello"))
    got := p.Get(h)
    require.Equal(t, []byte("hello"), got)
}
```

### Step 2: Run Test - MUST FAIL

```bash
$ go test -race ./internal/pool/... -v
# EXPECTED OUTPUT:
# undefined: pool.New
# FAIL
```

**Paste this failure. It proves the test is valid.**

### Step 3: Write Implementation

```go
// internal/pool/pool.go
func New(size int) *Pool { ... }
func (p *Pool) Intern(data []byte) Handle { ... }
func (p *Pool) Get(h Handle) []byte { ... }
```

### Step 4: Run Test - MUST PASS

```bash
$ go test -race ./internal/pool/... -v
# EXPECTED OUTPUT:
# === RUN   TestGet_ReturnsInternedData
# --- PASS: TestGet_ReturnsInternedData (0.00s)
# PASS
```

**Paste this pass. It proves implementation works.**

---

## Forbidden Actions

| Action | Why Forbidden | Correction |
|--------|---------------|------------|
| Write impl before test | No validation | Delete impl, write test first |
| Write passing test first | Test proves nothing | Add assertion that requires impl |
| Skip "test fails" step | Can't prove test is valid | Show failure before impl |
| Claim done without output | No proof | Paste full test output |

---

## Test Commands Reference

| Command | Purpose |
|---------|---------|
| `make test` | Full test suite with race detector |
| `make lint` | golangci-lint |
| `go test -race ./pkg/X/... -v` | Single package, verbose |
| `go test ./pkg/X/... -run TestName -v` | Single test |
| `go test ./... -cover` | Coverage report |
| `go test -race -tags=debug ./...` | With debug build tags |

---

## Test File Structure

```
pkg/bgp/message/
├── header.go           # Implementation
├── header_test.go      # Tests for header.go
├── open.go
├── open_test.go
└── test/data/           # Test fixtures
    ├── valid_open.bin
    └── truncated_open.bin
```

---

## Table-Driven Test Pattern

```go
// TestParseHeader verifies BGP header parsing for valid and invalid inputs.
//
// VALIDATES: RFC 4271 Section 4.1 - Message Header Format
//
// PREVENTS: Parsing failures, buffer overflows, incorrect message routing.
func TestParseHeader(t *testing.T) {
    tests := []struct {
        name    string
        input   []byte
        want    *Header
        wantErr error
    }{
        {
            name:  "valid_keepalive_header",
            input: []byte{0xFF, 0xFF, ...}, // 19 bytes
            want:  &Header{Type: TypeKEEPALIVE, Length: 19},
        },
        {
            name:    "truncated_header",
            input:   []byte{0xFF, 0xFF, 0xFF}, // only 3 bytes
            wantErr: ErrShortRead,
        },
        {
            name:    "invalid_marker",
            input:   []byte{0x00, 0x00, ...}, // bad marker
            wantErr: ErrInvalidMarker,
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

---

## Round-Trip Testing (Wire Format)

**Every pack/unpack MUST pass round-trip:**

```go
// TestHeaderRoundTrip verifies pack/unpack preserves all fields.
//
// VALIDATES: Wire format correctness per RFC 4271.
//
// PREVENTS: Data loss or corruption during serialization.
func TestHeaderRoundTrip(t *testing.T) {
    original := &Header{
        Marker: Marker,
        Length: 19,
        Type:   TypeKEEPALIVE,
    }

    packed := original.Pack()
    unpacked, err := ParseHeader(packed)

    require.NoError(t, err)
    assert.Equal(t, original, unpacked)
}
```

---

## Coverage Requirements

| Code Type | Minimum Coverage |
|-----------|------------------|
| Wire format (pack/unpack) | 90%+ |
| Public functions | 100% |
| Error paths | 100% |
| Edge cases | Explicitly tested |

```bash
# Check coverage
go test -race -cover ./internal/pool/...

# HTML report
go test -race -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

---

## Fuzzing (MANDATORY for Wire Format)

**All code parsing external input MUST have fuzz tests.**

### When Fuzzing is Required

| Code Type | Fuzz Required | Why |
|-----------|---------------|-----|
| **Wire Format** | | |
| Message parsing | YES | Untrusted network data |
| Attribute parsing | YES | Untrusted network data |
| NLRI parsing | YES | Untrusted network data |
| Capability parsing | YES | Untrusted network data |
| **Configuration** | | |
| Config tokenizer | YES | User-provided file |
| Config parser | YES | User-provided file |
| Neighbor/route syntax | YES | Complex user input |
| **API** | | |
| CLI command parsing | YES | User commands |
| JSON input parsing | YES | External API |
| **Internal** | | |
| Internal-only code | Optional | No external input |

### Fuzz Test Template

```go
// FuzzParseNLRI tests NLRI parsing robustness against malformed input.
//
// VALIDATES: Parser handles arbitrary bytes without crashing.
//
// PREVENTS: Remote crash via malformed UPDATE (DoS), buffer overflow,
// infinite loops, panics propagating to peer handler.
//
// SECURITY: Critical - NLRI comes from untrusted BGP peers.
func FuzzParseNLRI(f *testing.F) {
    // Seed with valid examples
    f.Add([]byte{24, 10, 0, 0})  // 10.0.0.0/24

    // Seed with edge cases
    f.Add([]byte{})              // Empty
    f.Add([]byte{33, 10, 0, 0})  // Invalid prefix length

    f.Fuzz(func(t *testing.T, data []byte) {
        // MUST NOT panic
        _, _ = ParseNLRI(data)
    })
}
```

### Fuzz Commands

```bash
# Run fuzz test for 30 seconds
go test -fuzz=FuzzParseNLRI -fuzztime=30s ./pkg/bgp/nlri/...

# Run until stopped
go test -fuzz=FuzzParseNLRI ./pkg/bgp/nlri/...

# Run all fuzz tests briefly (CI)
go test -fuzz=. -fuzztime=10s ./...
```

### Fuzz Corpus

Store interesting inputs in `test/data/fuzz/<FuzzName>/`:

```
test/data/fuzz/
├── FuzzParseHeader/
├── FuzzParseUpdate/
└── FuzzParseNLRI/
```

---

## Enforcement Checklist

Before claiming ANY feature complete:

```
=== TDD VERIFICATION ===

Feature: [name]
Test file: [path]

1. Test written: [ ] Yes
2. Test failed (paste output):
   $ go test -race ./[pkg]/... -v
   [PASTE FAILURE HERE]

3. Implementation written: [ ] Yes

4. Test passed (paste output):
   $ go test -race ./[pkg]/... -v
   [PASTE PASS HERE]

=== VERIFIED ===
```

**If ANY step missing: NOT DONE. Complete it.**

---

## Violation Detection

**If I do ANY of these, I'm violating TDD:**

- Write implementation code before test exists
- Show test passing without showing it failed first
- Skip test documentation (VALIDATES/PREVENTS)
- Claim "done" without pasting test output

**Auto-fix:** STOP. Delete implementation. Write test. Show failure. Then implement.

---

## See Also

- **TDD_ENFORCEMENT.md** - Complete TDD workflow and enforcement
- CI_TESTING.md - Full test suite commands
- CODING_STANDARDS.md - Go patterns

---

**Updated:** 2025-12-19
