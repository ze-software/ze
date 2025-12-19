# TDD Enforcement Protocol

**BLOCKING RULE: This protocol MUST be followed. No exceptions.**

---

## The Iron Rule of TDD

```
┌─────────────────────────────────────────────────────────────────┐
│                                                                 │
│   TESTS MUST EXIST AND FAIL BEFORE IMPLEMENTATION BEGINS        │
│                                                                 │
│   Writing implementation code without failing tests first       │
│   is a PROTOCOL VIOLATION that must be immediately corrected.   │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## TDD Workflow (MANDATORY)

### Phase 1: Write Test FIRST (With Documentation)

**Every test MUST document:**
1. What behavior it validates
2. What bug/issue it prevents
3. Why this test exists

```go
// internal/pool/pool_test.go

// TestInternDeduplication verifies that interning identical byte sequences
// returns the same handle and increments the reference count.
//
// PREVENTS: Memory bloat from storing duplicate data. Without deduplication,
// a router with 1M routes sharing the same AS_PATH would store 1M copies
// instead of 1 copy with refCount=1M.
//
// VALIDATES: RFC-compliant behavior where identical path attributes
// should be shared across routes.
func TestInternDeduplication(t *testing.T) {
    p := pool.New(1024)

    // Intern same data twice - MUST deduplicate
    h1 := p.Intern([]byte("hello"))
    h2 := p.Intern([]byte("hello"))

    // Same handle proves deduplication worked
    require.Equal(t, h1, h2, "identical data must return same handle")

    // RefCount=2 proves both references are tracked
    require.Equal(t, 2, p.RefCount(h1), "refCount must be 2 after two interns")
}
```

### Phase 2: Run Test - MUST FAIL

```bash
go test -race ./internal/pool/... -v
# EXPECTED: compilation error or test failure
# This PROVES the test is valid
```

**If test passes before implementation: TEST IS WRONG**

### Phase 3: Implement Minimum Code

Write ONLY enough code to make the test pass:

```go
// internal/pool/pool.go
func (p *Pool) Intern(data []byte) Handle {
    // Implementation here
}
```

### Phase 4: Run Test - MUST PASS

```bash
go test -race ./internal/pool/... -v
# EXPECTED: PASS
```

### Phase 5: Refactor (Optional)

If needed, refactor while keeping tests green.

---

## Violation Detection

### I am VIOLATING TDD if I:

1. **Write implementation before tests exist**
   - Auto-fix: STOP. Delete implementation. Write test first.

2. **Write tests that pass immediately**
   - Auto-fix: Tests must fail first. Add assertion that requires implementation.

3. **Write multiple features before testing each**
   - Auto-fix: ONE feature at a time. Test → Implement → Test.

4. **Claim "done" without showing test output**
   - Auto-fix: Run `go test`, paste output, show test file.

5. **Skip the "test fails" verification step**
   - Auto-fix: MUST show test failure before showing implementation.

---

## Planning Phase: Test Specifications Required

### Every plan file MUST include:

```markdown
## Feature: [Name]

### Test Specification (WRITE FIRST)

**Test file:** `pkg/X/feature_test.go`

**Test cases:**
1. `TestFeature_Success` - [expected behavior]
2. `TestFeature_EmptyInput` - [expected behavior]
3. `TestFeature_InvalidData` - [expected behavior]

**Test code outline:**
\```go
func TestFeature_Success(t *testing.T) {
    // Setup
    // Action
    // Assert
}
\```

### Implementation (WRITE AFTER TESTS FAIL)

**Implementation file:** `pkg/X/feature.go`

[Implementation details AFTER test spec]
```

### Plan Review Checklist

Before implementing ANY plan item:

- [ ] Test file path specified
- [ ] Test function names listed
- [ ] Expected behaviors documented
- [ ] Test code outline provided
- [ ] Implementation blocked until tests exist

---

## Development Session Workflow

### Starting Work on a Feature

1. **Read the plan** - identify test specifications
2. **Create test file** - write test functions
3. **Run tests** - MUST FAIL (compile error or assertion)
4. **Paste failure output** - prove tests are valid
5. **Write implementation** - minimum to pass
6. **Run tests** - MUST PASS
7. **Paste pass output** - prove implementation works
8. **Move to next feature** - repeat

### Example Session

```
=== Feature: Pool.Intern() ===

Step 1: Write test
Created: internal/pool/pool_test.go

Step 2: Run test (expecting failure)
$ go test -race ./internal/pool/... -v
--- FAIL: TestInternDeduplication (0.00s)
    pool_test.go:15: undefined: pool.New
FAIL

Step 3: Write implementation
Created: internal/pool/pool.go

Step 4: Run test (expecting pass)
$ go test -race ./internal/pool/... -v
=== RUN   TestInternDeduplication
--- PASS: TestInternDeduplication (0.00s)
PASS

=== Feature: Pool.Intern() COMPLETE ===
```

---

## Test-First Checklist (Per Feature)

Copy this for EVERY feature:

```markdown
### Feature: [Name]

#### Test Phase
- [ ] Test file created: `___test.go`
- [ ] Test functions written
- [ ] Tests run: FAILED (expected)
- [ ] Failure output pasted:
\```
[paste here]
\```

#### Implementation Phase
- [ ] Implementation written
- [ ] Tests run: PASSED
- [ ] Pass output pasted:
\```
[paste here]
\```

#### Completion
- [ ] Feature verified working
- [ ] Ready for next feature
```

---

## Forbidden Actions

### NEVER Do These:

| Action | Why Forbidden | Correction |
|--------|---------------|------------|
| Write impl before test | Skips validation | Delete impl, write test |
| Write test that passes | Test doesn't verify anything | Add failing assertion |
| Batch multiple features | Impossible to isolate failures | One at a time |
| Skip failure verification | Can't prove test is valid | Show failure first |
| Claim done without output | No proof of correctness | Paste test output |

---

## Enforcement Checklist

### Before claiming ANY feature complete:

```
=== TDD VERIFICATION ===

Feature: [name]

Phase 1 - Test Written:
- [ ] Test file: [path]
- [ ] Test functions: [list]

Phase 2 - Test Failed:
$ go test -race ./[pkg]/... -v
[PASTE FAILURE OUTPUT HERE]

Phase 3 - Implementation Written:
- [ ] Implementation file: [path]

Phase 4 - Test Passed:
$ go test -race ./[pkg]/... -v
[PASTE PASS OUTPUT HERE]

=== TDD VERIFICATION COMPLETE ===
```

**If ANY section incomplete: STOP. Complete it.**

---

## Test Documentation Requirements (MANDATORY)

### Every Test Function MUST Have:

```go
// TestFunctionName documents the test purpose in the first line.
//
// VALIDATES: [What RFC/behavior/contract this test verifies]
//
// PREVENTS: [What bug/issue/regression this test catches]
// [Describe the failure scenario this test protects against]
//
// REPRODUCES: [If fixing a bug, reference to issue or describe the bug]
func TestFunctionName(t *testing.T) {
    // Test implementation with commented assertions
}
```

### Documentation Checklist

For EACH test function:
- [ ] First line: One-sentence summary
- [ ] VALIDATES: What correct behavior looks like
- [ ] PREVENTS: What failure this catches
- [ ] REPRODUCES: (for bug fixes) Original bug description
- [ ] Assertions have explanation strings

### Example: Complete Test Documentation

```go
// TestPoolGetAfterCompaction verifies that handles remain valid after
// the pool performs compaction, returning correct data.
//
// VALIDATES: Handle stability guarantee - callers can hold handles
// across compaction cycles without invalidation.
//
// PREVENTS: Data corruption where compaction moves data but handles
// still point to old offsets. This would cause Get() to return
// garbage or panic on out-of-bounds access.
//
// SCENARIO: Pool has entries A, B, C. B is released (dead). Compaction
// runs, moving C to B's location. Handle for C must still work.
func TestPoolGetAfterCompaction(t *testing.T) {
    p := pool.New(1024)

    // Create entries
    hA := p.Intern([]byte("AAA"))
    hB := p.Intern([]byte("BBB"))
    hC := p.Intern([]byte("CCC"))

    // Release B - creates dead space
    p.Release(hB)

    // Force compaction
    p.Compact()

    // Handles A and C must still return correct data
    require.Equal(t, []byte("AAA"), p.Get(hA), "handle A must survive compaction")
    require.Equal(t, []byte("CCC"), p.Get(hC), "handle C must survive compaction")
}
```

### Example: Bug Regression Test

```go
// TestInternCollisionHandling verifies that hash collisions are handled
// correctly, storing both entries separately.
//
// VALIDATES: Correctness under hash collision - two different byte
// sequences with the same hash must be stored independently.
//
// PREVENTS: Data corruption where hash collision causes incorrect
// deduplication. Entry B would silently return Entry A's data.
//
// REPRODUCES: Issue discovered during design review. Original design
// used hash as map key without verifying data equality, causing
// silent data corruption on collision.
func TestInternCollisionHandling(t *testing.T) {
    // ... test implementation
}
```

---

## Why TDD is Mandatory

| Without TDD | With TDD |
|-------------|----------|
| "I think it works" | Proof it works |
| Discover bugs later | Catch bugs immediately |
| Fear of refactoring | Confidence to improve |
| Undocumented behavior | Tests ARE documentation |
| Regression risk | Regression impossible |

---

## Quick Reference

```
TDD Cycle:
   ┌──────────┐
   │  RED     │  ← Write test, watch it fail
   └────┬─────┘
        │
   ┌────▼─────┐
   │  GREEN   │  ← Write minimum code to pass
   └────┬─────┘
        │
   ┌────▼─────┐
   │ REFACTOR │  ← Improve while green
   └────┬─────┘
        │
        └──────→ Repeat
```

**RED → GREEN → REFACTOR**

---

---

## Fuzzing Requirements (Wire Format Code)

**MANDATORY for any code that parses external input:**
- Message parsing (OPEN, UPDATE, NOTIFICATION, KEEPALIVE)
- Attribute parsing
- NLRI parsing
- Capability parsing

### Fuzz Test Template

```go
// FuzzParseHeader tests header parsing with random/malformed input.
//
// VALIDATES: Parser robustness against malformed data.
//
// PREVENTS: Crashes, panics, buffer overflows, infinite loops
// when receiving malformed BGP messages from peers.
func FuzzParseHeader(f *testing.F) {
    // Seed corpus with valid examples
    f.Add([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
                 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
                 0x00, 0x13, 0x04}) // Valid KEEPALIVE header

    // Seed with edge cases
    f.Add([]byte{})                    // Empty
    f.Add([]byte{0x00})                // Too short
    f.Add([]byte{0xFF, 0xFF, 0xFF})    // Truncated

    f.Fuzz(func(t *testing.T, data []byte) {
        // Parser MUST NOT panic on any input
        header, err := ParseHeader(data)

        if err != nil {
            // Error is acceptable - malformed input
            return
        }

        // If no error, result must be valid
        if header.Length < 19 || header.Length > 4096 {
            t.Errorf("invalid length: %d", header.Length)
        }
    })
}
```

### Fuzzing Commands

```bash
# Run fuzz test for 30 seconds
go test -fuzz=FuzzParseHeader -fuzztime=30s ./pkg/bgp/message/...

# Run fuzz test until stopped (Ctrl+C)
go test -fuzz=FuzzParseHeader ./pkg/bgp/message/...

# Run with specific corpus
go test -fuzz=FuzzParseHeader -fuzztime=1m ./pkg/bgp/message/...
```

### Fuzz Test Requirements

| Code Type | Fuzz Test Required | Why |
|-----------|-------------------|-----|
| **Wire Format (Network Input)** | | |
| Message parsing | YES | Untrusted peer data |
| Attribute parsing | YES | Untrusted peer data |
| NLRI parsing | YES | Untrusted peer data |
| Capability parsing | YES | Untrusted peer data |
| **Configuration (User Input)** | | |
| Config tokenizer | YES | User-provided file |
| Config parser | YES | User-provided file |
| Neighbor definitions | YES | Complex syntax |
| Route definitions | YES | Complex syntax |
| **API (External Input)** | | |
| CLI command parsing | YES | User commands |
| JSON input parsing | YES | External API calls |
| **Internal** | | |
| Internal-only code | Optional | No external input |

### Fuzz Test Documentation

```go
// FuzzParseUpdate tests UPDATE message parsing robustness.
//
// VALIDATES: Parser handles arbitrary byte sequences without crashing.
//
// PREVENTS:
// - Remote crash via malformed UPDATE (DoS)
// - Buffer overflow leading to memory corruption
// - Infinite loop causing CPU exhaustion
// - Panic propagating to peer handler
//
// SECURITY: This fuzz test is critical because UPDATE messages come
// directly from potentially malicious BGP peers.
func FuzzParseUpdate(f *testing.F) {
    // ...
}
```

### Corpus Management

Store interesting test cases in `testdata/fuzz/`:

```
testdata/
└── fuzz/
    ├── FuzzParseHeader/
    │   ├── valid_keepalive
    │   ├── valid_open
    │   └── truncated_marker
    └── FuzzParseUpdate/
        ├── empty_update
        └── max_size_update
```

---

## See Also

- TESTING_PROTOCOL.md - Test command reference
- CI_TESTING.md - Full test suite
- CODING_STANDARDS.md - Go patterns

---

**Updated:** 2025-12-19
