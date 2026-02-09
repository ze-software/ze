# Fix Encoding Allocation

Convert a `make([]byte, ...)` allocation to buffer-writing pattern.

## Instructions

1. Use ULTRATHINK for careful analysis
2. `$ARGUMENTS` is REQUIRED and specifies the target: `file.go:line` or `file.go:functionName`
   - Example: `internal/plugin/bgp/attribute/simple.go:MED.Pack`
   - Example: `internal/plugin/bgp/reactor/reactor.go:4926`
   - If no argument: print "Usage: /fix-alloc file.go:line-or-function" and stop
3. Read the target file and understand the allocation context
4. Read callers of the function to understand how the result is used
5. Apply the appropriate fix pattern (see below)
6. Run `make lint && make test` to verify

## Pre-Flight Checks

Before modifying ANYTHING:

1. **Read the function** containing the allocation
2. **Read ALL callers** of that function (use Grep to find them)
3. **Determine the category** (see table below)
4. **Check if WriteTo already exists** on the type — if so, callers should use it instead of Pack()
5. **Check if the result is stored** vs used immediately — stored results may need different treatment

## Fix Patterns by Category

### Category A: Type has Pack() but no WriteTo — Add WriteTo

The type implements `Pack() []byte` but not `WriteTo(buf, off) int`.

**Steps:**
1. Add `WriteTo(buf []byte, off int) int` method that writes directly
2. Add `WriteToWithContext` if `PackWithContext` exists
3. Add `CheckedWriteTo` for safety
4. Mark `Pack()` with deprecation comment if not already present
5. Do NOT delete `Pack()` yet — callers may still use it

**Template** (follow existing pattern from `MED` in `simple.go`):

```go
// WriteTo writes the TYPE value into buf at offset.
func (x Type) WriteTo(buf []byte, off int) int {
    // Write directly into buf[off:] instead of allocating
    // Return bytes written
}

// WriteToWithContext writes TYPE value - context-independent.
func (x Type) WriteToWithContext(buf []byte, off int, _, _ *bgpctx.EncodingContext) int {
    return x.WriteTo(buf, off)
}

// CheckedWriteTo validates capacity before writing.
func (x Type) CheckedWriteTo(buf []byte, off int) (int, error) {
    if len(buf) < off+x.Len() {
        return 0, wire.ErrBufferTooSmall
    }
    return x.WriteTo(buf, off), nil
}
```

**Reference implementations** (read these for patterns):
- `MED.WriteTo` — `internal/plugin/bgp/attribute/simple.go:99`
- `LocalPref.WriteTo` — `internal/plugin/bgp/attribute/simple.go:152`
- `Origin.WriteTo` — `internal/plugin/bgp/attribute/origin.go:76`
- `ASPath.WriteTo` — `internal/plugin/bgp/attribute/aspath.go`
- `Communities.WriteTo` — `internal/plugin/bgp/attribute/community.go`

### Category B: Caller uses Pack() when WriteTo exists — Switch caller

The type already has `WriteTo` but the caller uses `Pack()`.

**Steps:**
1. Change caller to allocate (or reuse) a buffer
2. Call `WriteTo(buf, off)` instead of `Pack()`
3. If caller already has a `wire.SessionBuffer` or `[]byte` buffer, use it

**Before:**
```go
packed := attr.Pack()
copy(buf[off:], packed)  // Double copy!
```

**After:**
```go
n := attr.WriteTo(buf, off)  // Direct write, zero alloc
```

### Category C: Helper function returning []byte — Convert to write-into

A helper function like `encodeTEIDFieldWithBits` that allocates and returns.

**Steps:**
1. Change signature from `func encode...() []byte` to `func encode...(buf []byte, off int, ...) int`
2. Write directly into `buf[off:]`
3. Return bytes written
4. Update all callers

**Before:**
```go
func encodeTEIDFieldWithBits(teid uint32, bits int) []byte {
    if bits <= 0 { return nil }
    byteLen := (bits + 7) / 8
    result := make([]byte, byteLen)
    for i := range byteLen {
        shift := (byteLen - 1 - i) * 8
        result[i] = byte(teid >> shift)
    }
    return result
}
```

**After:**
```go
func writeTEIDFieldWithBits(buf []byte, off int, teid uint32, bits int) int {
    if bits <= 0 { return 0 }
    byteLen := (bits + 7) / 8
    for i := range byteLen {
        shift := (byteLen - 1 - i) * 8
        buf[off+i] = byte(teid >> shift)
    }
    return byteLen
}
```

### Category D: Reactor/peer building path — Use SessionBuffer

Hot-path code in reactor/peer that builds UPDATE wire bytes.

**Steps:**
1. Check if a `wire.SessionBuffer` or reusable `[]byte` is available in scope
2. If yes: write into it using `WriteTo` / `WriteHeaderTo` / direct writes
3. If no: check if one can be passed from the caller or stored on the struct
4. Use `attribute.WriteHeaderTo(buf, off, flags, code, length)` instead of `PackHeader()`
5. Use `attribute.WriteAttributeTo(buf, off, attr)` instead of `PackAttribute()`

**Before:**
```go
nlriBytes := make([]byte, totalNLRILen)
off := 0
for _, n := range nlris {
    packed := n.Pack()
    copy(nlriBytes[off:], packed)
    off += len(packed)
}
```

**After:**
```go
// Assume buf is a pre-allocated session buffer
off := startOff
for _, n := range nlris {
    off += n.WriteTo(buf, off)
}
nlriLen := off - startOff
```

### Category E: Cached encoding (computed once, stored) — Keep make()

If the allocation result is stored in a struct field and reused across calls
(e.g., `cached` fields in NLRI types), the `make()` is appropriate.

**Action:** Leave as-is. Document with a comment explaining why:
```go
// Cached wire encoding — allocated once, reused on subsequent WriteTo calls.
n.cached = make([]byte, 2+len(data))
```

## Verification

After making changes:

1. **Compile:** `go build ./...`
2. **Lint:** `make lint`
3. **Test:** `make test`
4. **Check for Pack() deprecation:** If ALL callers now use WriteTo, add deprecation comment to Pack()

## Common Mistakes to Avoid

| Mistake | Why it's wrong | Fix |
|---------|---------------|-----|
| Deleting `Pack()` | Other callers may still use it | Keep Pack(), add deprecation comment |
| Not returning bytes written | Caller can't advance offset | Always return `int` from WriteTo |
| Not checking `Len()` matches WriteTo | Buffer overflow risk | Verify Len() returns same as WriteTo writes |
| Ignoring context params | AS_PATH/Aggregator need ASN4 | Use WriteToWithContext for context-dependent types |
| Writing past buffer end | Caller trusts Len() for sizing | Ensure WriteTo writes exactly Len() bytes |
| Changing cached fields | Cached encoding is intentional | See Category E — leave cached allocs alone |

## Do NOT

- DO NOT delete `Pack()` methods — mark as deprecated, keep for compatibility
- DO NOT change function signatures that are part of the `Attribute` interface without updating the interface
- DO NOT modify test files unless tests need updating for new signatures
- DO NOT fix more than the specified target — one function per invocation
- DO NOT add `WriteTo` to the `Attribute` interface itself (it's separate: `wire.BufWriter`)
