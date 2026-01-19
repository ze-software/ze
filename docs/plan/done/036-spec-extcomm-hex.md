# Spec: Extended Community Hex Format Parsing

## Task
Add hex format parsing for extended-community (`0x...`) to fix test Z (vpn).

## Current State (verified 2025-12-28)
- Functional tests: 28 passed, 9 failed
- Failing: 0, N, Q, S, T, U, V, Z, a
- Test Z error: `invalid extended-community "0x0002fde800000001": expected format like target:ASN:NN`
- Last commit: `ef4ebf1`

## Problem Analysis
ZeBGP's `parseOneExtCommunity()` in `internal/config/routeattr.go` doesn't handle hex format.

ExaBGP's approach in `_extended_community_hex()`:
1. Check for `0x` prefix with no colons
2. Strip `0x`, convert hex pairs to bytes
3. Return raw bytes as-is (already wire format)

## Embedded Protocol Requirements

### Default Rules (ALL tasks)
- Tests MUST exist and FAIL before implementation
- Run `make test && make lint` before claiming done
- NEVER discard uncommitted work without explicit user permission
- Verify before claiming: run commands, paste output as proof
- Tests passing is NOT permission to commit - wait for user

### From TDD_ENFORCEMENT.md
- Write test FIRST with VALIDATES/PREVENTS documentation
- Run test → MUST FAIL (prove test is valid)
- Write implementation (minimum to pass)
- Run test → MUST PASS (paste output)

### From CODING_STANDARDS.md
- Never ignore errors - always check
- Use `fmt.Errorf` with `%w` for wrapping
- No panic for errors

## Codebase Context
- **File to modify:** `internal/config/routeattr.go`
- **Function:** `parseOneExtCommunity()` (lines 187-248)
- **Existing pattern:** Uses `encoding/hex` package (already imported)
- **Test file:** `internal/config/routeattr_test.go` (if exists) or create it

## Implementation Steps

### 1. Write Test (TDD Phase 1)
Add test case for hex format parsing:
```go
// TestParseExtendedCommunityHex verifies hex format parsing.
//
// VALIDATES: ExaBGP-compatible 0x... format for extended communities.
//
// PREVENTS: Config rejection for valid ExaBGP configs using hex format.
func TestParseExtendedCommunityHex(t *testing.T) {
    // Test cases:
    // - "0x0002fde800000001" -> 8 bytes [0x00, 0x02, 0xfd, 0xe8, 0x00, 0x00, 0x00, 0x01]
    // - Invalid: "0x12345" (odd length)
    // - Invalid: "0xGGGG" (not hex)
}
```

### 2. Run Test → MUST FAIL
```bash
go test -race ./internal/config/... -v -run TestParseExtendedCommunityHex
```

### 3. Implement Hex Parsing
Modify `parseOneExtCommunity()` to handle hex format:
```go
func parseOneExtCommunity(s string) ([]byte, error) {
    // NEW: Check for hex format (0x prefix, no colons)
    if strings.HasPrefix(s, "0x") && !strings.Contains(s, ":") {
        return parseExtCommunityHex(s)
    }
    // ... existing code ...
}

func parseExtCommunityHex(s string) ([]byte, error) {
    // Strip 0x prefix
    hexStr := strings.TrimPrefix(s, "0x")
    hexStr = strings.TrimPrefix(hexStr, "0X")

    // Must be exactly 16 hex chars (8 bytes)
    if len(hexStr) != 16 {
        return nil, fmt.Errorf("invalid extended-community %q: hex format must be 16 chars (8 bytes)", s)
    }

    // Decode hex
    raw, err := hex.DecodeString(hexStr)
    if err != nil {
        return nil, fmt.Errorf("invalid extended-community %q: %w", s, err)
    }

    return raw, nil
}
```

### 4. Run Test → MUST PASS
```bash
go test -race ./internal/config/... -v -run TestParseExtendedCommunityHex
```

### 5. Run Functional Test Z
```bash
go run ./test/cmd/functional encoding Z
```

### 6. Run Full Test Suite
```bash
make test && make lint
```

## Verification Checklist
- [ ] Test written with VALIDATES/PREVENTS documentation
- [ ] Test shown to FAIL first
- [ ] Implementation makes test pass
- [ ] `go test ./internal/config/...` passes
- [ ] Functional test Z passes
- [ ] `make test` passes
- [ ] `make lint` passes
