# Spec: writeto-error-return

## Task
Add `CheckedWriteTo` methods that validate before writing, returning `(int, error)`.
Keep existing `WriteTo` methods unchanged (unchecked, returns `int`).

**This is an ADDITIVE change:**
- `WriteTo` - unchanged, caller guarantees capacity (fast path)
- `CheckedWriteTo` - validates capacity and state, returns error if invalid (safe path)

**Why error return matters:**
1. **Buffer overflow detection** - undefined behavior → defined error
2. **Invalid state detection** - malformed objects caught at serialization
3. **Disambiguate empty writes** - `return 0` is ambiguous; `(0, nil)` vs `(0, err)` is clear

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - [canonical architecture, buffer-first principles]
- [ ] `docs/architecture/buffer-architecture.md` - [WriteTo is core interface]
- [ ] `docs/architecture/encoding-context.md` - [WriteToWithContext uses encoding context]
- [ ] `docs/architecture/update-building.md` - [how WriteTo is used in UPDATE construction]

### RFC Summaries
N/A - internal refactoring, no protocol changes.

**Key insights:**
- WriteTo is part of zero-allocation buffer-first architecture
- Every type with WriteTo has matching Len() method (verified by analyze-writeto.go)
- WriteTo contract unchanged; CheckedWriteTo adds optional validation layer

## Error Types

```go
// internal/bgp/wire/errors.go
var ErrBufferTooSmall = errors.New("wire: buffer too small")
```

| Error | Cause | Example |
|-------|-------|---------|
| `ErrBufferTooSmall` | Buffer can't fit data | `len(buf) < off + needed` |

## Implementation Pattern

### CheckedWriteTo Validates, Then Calls WriteTo

```go
// CheckedWriteTo validates capacity, then delegates to WriteTo
func (x *Foo) CheckedWriteTo(buf []byte, off int) (int, error) {
    needed := x.Len()
    if len(buf) < off+needed {
        return 0, wire.ErrBufferTooSmall
    }
    return x.WriteTo(buf, off), nil
}

// WriteTo unchanged - caller guarantees capacity
func (x *Foo) WriteTo(buf []byte, off int) int {
    buf[off] = x.value
    return 1
}
```

### Composite Pattern

```go
// CheckedWriteTo validates total capacity, then calls WriteTo
func (c *Composite) CheckedWriteTo(buf []byte, off int) (int, error) {
    needed := c.Len()
    if len(buf) < off+needed {
        return 0, wire.ErrBufferTooSmall
    }
    return c.WriteTo(buf, off), nil
}

// WriteTo unchanged - calls child WriteTo (all unchecked)
func (c *Composite) WriteTo(buf []byte, off int) int {
    pos := off
    pos += c.child1.WriteTo(buf, pos)
    pos += c.child2.WriteTo(buf, pos)
    return pos - off
}
```

### Partial Write Semantics

On error from `CheckedWriteTo`, return `(bytesWritten, err)`:
- Caller knows how much was written
- Buffer state may be inconsistent on error
- Similar to `io.Writer` semantics

## Scope

| Interface | Existing (unchanged) | New (added) |
|-----------|---------------------|-------------|
| BufWriter | `WriteTo(buf, off) int` | `CheckedWriteTo(buf, off) (int, error)` |
| NLRI | `WriteTo(buf, off, ctx) int` | `CheckedWriteTo(buf, off, ctx) (int, error)` |
| WriteToWithContext | `WriteToWithContext(...) int` | `CheckedWriteToWithContext(...) (int, error)` |
| Builder | `WriteTo(buf) int` | `CheckedWriteTo(buf) (int, error)` |

**New methods to add:** ~70 `CheckedWriteTo` methods
**Existing call sites:** unchanged (opt-in to checked version where needed)

## 🧪 TDD Test Plan

**TDD applies:** This is additive - we can write tests for `CheckedWriteTo` first, see them fail (method doesn't exist), then implement.

### Unit Tests
| Test | File | Validates |
|------|------|-----------|
| `TestCheckedWriteReturnsErrBufferTooSmall` | `internal/bgp/wire/writer_test.go` | Returns ErrBufferTooSmall when buffer insufficient |
| `TestCheckedWriteSuccess` | `internal/bgp/wire/writer_test.go` | Returns (n, nil) on success |

### Functional Tests
| Test | Location | Scenario |
|------|----------|----------|
| All existing | `qa/tests/*` | No regression (WriteTo unchanged) |

## Files to Modify

### 1. Error Types + Interface Definitions
1. Create `internal/bgp/wire/errors.go`
2. `internal/bgp/wire/writer.go` - BufWriter interface
3. `internal/bgp/nlri/nlri.go` - NLRI interface
4. `internal/bgp/attribute/attribute.go` - Attribute interface

### 2. Leaf Implementations (No Dependencies)
5. `internal/cbor/*.go` (3 files)
6. `internal/bgp/attribute/origin.go`
7. `internal/bgp/attribute/simple.go`
8. `internal/bgp/attribute/opaque.go`
9. `internal/bgp/nlri/inet.go`
10. `internal/bgp/nlri/wire.go`

### 3. Mid-Level Implementations
11. `internal/bgp/attribute/aspath.go`
12. `internal/bgp/attribute/as4.go`
13. `internal/bgp/attribute/community.go`
14. `internal/bgp/nlri/labeled.go`
15. `internal/bgp/nlri/ipvpn.go`

### 4. Complex Composite Implementations
16. `internal/bgp/attribute/mpnlri.go`
17. `internal/bgp/nlri/flowspec.go`
18. `internal/bgp/nlri/bgpls.go`
19. `internal/bgp/nlri/evpn.go`
20. `internal/bgp/nlri/other.go`

### 5. Builder + Message
21. `internal/bgp/attribute/builder.go`
22. `internal/bgp/message/update.go`
23. `internal/bgp/message/update_build.go`

### 6. Call Sites (Opt-In to CheckedWriteTo)
24. `internal/reactor/session.go` - **critical path, should use CheckedWriteTo**
25. Other call sites - remain on WriteTo (unchanged)

### 7. Tests (New CheckedWriteTo Tests)
28. `internal/bgp/attribute/builder_test.go`
29. `internal/bgp/attribute/len_writeto_test.go`
30. `internal/bgp/attribute/community_test.go`
31. `internal/bgp/attribute/aspath_test.go`
32. `internal/bgp/message/update_test.go`
33. `internal/bgp/nlri/writeto_test.go`
34. `internal/bgp/nlri/ipvpn_test.go`
35. `internal/bgp/nlri/base_len_test.go`
36. `internal/bgp/nlri/wire_test.go`
37. `internal/cbor/cbor_test.go`, `internal/cbor/hex_test.go`, `internal/cbor/base64_test.go`
38. `internal/rib/commit_wire_test.go`

## Implementation Steps

1. **Verify Len() coverage** - `go run scripts/analyze-writeto.go` (paste output)
2. **Create error types** - `internal/bgp/wire/errors.go`
3. **Write tests first (TDD)** - `TestCheckedWriteTo*` tests, verify FAIL (paste output)
4. **Add CheckedWriteTo to interfaces** - BufWriter, NLRI, Attribute
5. **Implement CheckedWriteTo methods** - ~70 methods, each validates then calls WriteTo
6. **Run tests** - Verify PASS (paste output)
7. **Opt-in at critical call sites** - `internal/reactor/session.go` uses CheckedWriteTo
8. **Verify all** - `make lint && make test && make functional` (paste output)

## Checklist

### Pre-Implementation
- [x] `go run scripts/analyze-writeto.go` verifies all WriteTo have Len()
- [x] Required docs read

### 🧪 TDD Cycle
- [x] Error types created (`internal/bgp/wire/errors.go`)
- [x] Tests written
- [x] Tests FAIL (before implementation)
- [x] `CheckedWriteTo` added to interfaces
- [x] All `CheckedWriteTo` methods implemented
- [x] `CheckedWriteToWithContext` added for context-dependent types (Aggregator, ASPath)
- [x] WireNLRI refactored: LenWithContext calculates length, WriteTo writes directly, Pack calls WriteTo
- [x] Tests PASS

### Verification
- [x] `go test ./internal/bgp/... ./internal/cbor/...` passes
- [x] All 26 modified files compile and pass tests

### Documentation
- [x] `docs/architecture/wire/buffer-writer.md` updated with CheckedWriteTo

### Completion
- [x] Spec moved to `docs/plan/done/103-writeto-error-return.md`
