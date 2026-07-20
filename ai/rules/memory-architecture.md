# Memory Architecture

**When:** Conceptual model for Ze's memory management
**Severity:** advisory

## Directives

Conceptual model for Ze's memory management. Ties together `buffer-first.md`,
`no-sprintf-alloc.md`, and `design-principles.md` into a coherent picture.
Read this before making any allocation or memory-lifecycle decision.

## The Core Idea

Ze processes millions of BGP UPDATEs per second. Each UPDATE touches: wire
parsing, attribute extraction, pool dedup, RIB storage, route selection,
filter evaluation, UPDATE building, and TCP write. Every allocation on
this path adds GC pressure and latency. Ze eliminates allocations through
three interlocking strategies:

1. **Buffer ownership** -- caller owns the buffer, callee writes into it
2. **Pool lifecycle** -- bounded pools replace unbounded `make()`
3. **Lazy parsing** -- raw byte slices with offset iterators, no parsed structs

## Data Lifecycle (Wire to Wire)

```
TCP recv → readBufPool (4K/64K)
    → WireUpdate (lazy, references readBuf)
    → attribute extraction (lazy iterators, no copy)
    → pool dedup (per-attribute-type, refcounted)
    → RIB entry (NLRI → attribute Handle refs)
    → route selection (operates on Handles)
    → outbound building (WriteTo into peerPool buffer)
    → TCP send → release peerPool buffer
```

### Ownership Rules

| Stage | Buffer owner | What happens |
|---|---|---|
| TCP read | Incoming Peer Pool (ring of 64 buffers) | Wire bytes land in pool buffer |
| Parsing | Same buffer | WireUpdate holds byte slice into pool buffer, no copy |
| Attribute extract | Same buffer | Lazy iterators return sub-slices, no copy |
| Pool dedup | Attribute pool | First time: copy into pool slab. Subsequent: refcount++ |
| Forwarding (same context) | Same pool buffer | Zero-copy: ContextID match means wire bytes are reusable |
| Forwarding (different context) | Outgoing Peer Pool | Copy-on-modify: build modified UPDATE into outgoing buffer |
| Overflow | Global Shared Pool (MixedBufMux) | Byte-budgeted, mixed 4K/64K blocks |

### When Copies Happen

Copies are deliberate, never accidental:

| Copy trigger | Why |
|---|---|
| Attribute enters pool for first time | Pool owns the canonical copy; wire buffer will be reused |
| ContextID mismatch on forward | Wire bytes encoded with different capabilities need re-encoding |
| Filter modifies attributes | Modified attributes are written into outgoing buffer |
| JSON serialization for external plugins | External plugins need formatted text, not wire bytes |

If you're adding a copy that doesn't fit one of these categories, you're
probably doing it wrong. Ask why the copy is needed.

## Pool Types

| Pool | Location | Shape | Purpose |
|---|---|---|---|
| `readBufPool4K` | reactor | `sync.Pool` | Standard message TCP reads |
| `readBufPool64K` | reactor | `sync.Pool` | Extended message TCP reads |
| `buildBufPool` | reactor | `sync.Pool` | UPDATE building workspace |
| `peerPool` (per-peer) | `forward_pool.go` | Ring (64 slots) | Per-peer inbound/outbound buffers |
| `MixedBufMux` | `bufmux.go` | Byte-budgeted | Global overflow, mixed block sizes |
| `textbuf.Buffer` pool | `internal/core/textbuf` | `sync.Pool` | String building (via `Get()`/`Release()`) |
| Attribute pools | `attrpool/` | Per-type dedup slabs | RIB attribute deduplication |
| NLRI pools | per-family plugins | Per-family dedup | RIB NLRI deduplication |

### Pool Strategy by Goroutine Shape

This is a load-bearing design decision:

| Goroutine pattern | Pool strategy | Example |
|---|---|---|
| Single goroutine, sequential processing | Ring buffer (fixed array, index rotation) | `peerPool` -- reactor loop processes one UPDATE at a time per peer |
| Multiple goroutines, concurrent access | `sync.Pool` seeded for peak | `readBufPool` -- multiple peers reading concurrently |

All buffers in a pool are the **same maximum size**. No variable-sized allocation.

## Key Wire Abstractions

| Type | Location | Purpose | Lifecycle |
|---|---|---|---|
| `WireUpdate` | `wireu/wire_update.go` | Lazy-parsed UPDATE message | Lives as long as the readBuf it references |
| `PackContext` | `attribute/pack_context.go` | Negotiated capabilities for encoding | Per-peer, created from OPEN exchange |
| `ContextID` | `context/registry.go` | Compact uint16 identifying encoding context | Same ID = same encoding = zero-copy forward |
| `BufHandle` | `reactor/bufmux.go` | Reference to a pool-managed buffer | Tracks which pool + slot owns the buffer |
| `BufWriter` | `wire/writer.go` | Interface: `WriteTo(buf, off) int` | Implemented by all wire-encodable types |

### BufWriter Interface

Every type that produces wire bytes implements:

```go
type BufWriter interface {
    WriteTo(buf []byte, off int) int
    Len() int
}
```

Context-dependent types (AS_PATH, Aggregator) also have:

```go
WriteToWithContext(buf []byte, off int, src, dst *EncodingContext) int
```

The caller **always** owns the buffer. The callee writes into `buf[off:]` and
returns the number of bytes written. No allocations.

## String Building

Ze has two non-allocating string building patterns:

### textbuf.Buffer (stack-allocated chainable builder)

```go
var b textbuf.Buffer
return b.Str("peer ").Addr(addr).Byte(':').Uint16(port).String()
```

128-byte inline backing array stays on the stack. Two terminals:

- **`String()`** -- copies inline data (<=128B), zero-copy for heap data (>128B). Use when the result is **stored**.
- **`Slice()`** -- zero-copy at any size. Result is only valid until `Reset()` or `Release()`. Use when the result is **consumed immediately**.

**Prefer `Slice()` by default.** Most strings are passed to a function
(ParsePrefix, map lookup, Write) and discarded. `Slice()` saves the copy.

```go
// Slice: zero allocations, consumed by ParsePrefix
entry, _ := netip.ParsePrefix(b.Reset().Addr(addr).Byte('/').Int(int64(bits)).Slice())

// String: one allocation, stored in struct field
peer.Label = b.Reset().Str("AS").Uint32(asn).String()
```

**Reuse with `Reset()`** -- declare one Buffer before a loop, call `Reset()`
between iterations. Each iteration reuses the same inline array:

```go
var b textbuf.Buffer
for _, peer := range peers {
    key := b.Reset().Addr(peer.Addr).Byte(':').Uint16(peer.Port).Slice()
    lookupMap[key] = peer
}
```

**Pooled usage** for high-frequency paths across goroutines:

```go
b := textbuf.Get()
defer b.Release()
return b.Str("prefix").Uint32(v).String()
```

### AppendTo pattern (for types)

Named types implement `AppendTo([]byte) []byte` for composable formatting:

```go
func (t *MyType) AppendTo(buf []byte) []byte {
    buf = append(buf, "prefix "...)
    buf = textbuf.Uint(buf, uint64(t.Field))
    return buf
}
```

Callers chain: `buf = typeA.AppendTo(typeB.AppendTo(buf[:0]))`.

## Caller-Owned Resources (BLOCKING on hot paths)

The single most common allocation mistake: a callee allocates a buffer
internally when the caller could pass one down.

### The Principle

**Allocate once at the outermost scope, pass inward.** The caller knows:
- How many times the callee will be called (loop count)
- What buffer size is needed (often a bounded maximum)
- When the buffer can be released (after all callees are done)

The callee should never guess at these. It writes into what it's given.

### Anti-Pattern: Per-Call Allocation in a Loop

```go
// BAD: allocates N times inside the loop
for _, attr := range attributes {
    packed := attr.Pack()         // make([]byte, attr.Len()) inside
    copy(buf[off:], packed)       // copy into caller's buffer
    off += len(packed)
}

// GOOD: zero allocations
for _, attr := range attributes {
    off += attr.WriteTo(buf, off) // writes directly into caller's buffer
}
```

### Anti-Pattern: Helper That Allocates Internally

```go
// BAD: format() allocates a string each call
func format(addr netip.Addr, port uint16) string {
    return fmt.Sprintf("%s:%d", addr, port)  // 3 allocations
}

// GOOD: caller owns the buffer
func appendFormat(buf []byte, addr netip.Addr, port uint16) []byte {
    buf = addr.AppendTo(buf)
    buf = append(buf, ':')
    buf = textbuf.Uint(buf, uint64(port))
    return buf
}

// Or with textbuf.Buffer when you need a string:
var b textbuf.Buffer
return b.Addr(addr).Byte(':').Uint16(port).String()  // 1 allocation
```

### Anti-Pattern: Sub-Function Allocates What Caller Already Has

```go
// BAD: buildPayload allocates its own scratch buffer
func buildPayload(attrs []Attribute) []byte {
    buf := make([]byte, totalLen(attrs))   // ALLOCATION
    off := 0
    for _, a := range attrs {
        off += a.WriteTo(buf, off)
    }
    return buf
}

// GOOD: caller passes its buffer
func writePayload(buf []byte, off int, attrs []Attribute) int {
    start := off
    for _, a := range attrs {
        off += a.WriteTo(buf, off)
    }
    return off - start
}
```

### sync.Pool for Reusable Scratch Buffers

When a function needs a scratch buffer that the caller cannot provide (e.g.,
the function is called from many sites, or the scratch size varies), use
`sync.Pool`:

```go
var scratchPool = sync.Pool{
    New: func() any { return make([]byte, 0, 4096) },
}

func process(data []byte) Result {
    scratch := scratchPool.Get().([]byte)[:0]
    defer scratchPool.Put(scratch)

    // use scratch for intermediate work
    scratch = append(scratch, data...)
    // ... transform scratch ...

    return buildResult(scratch)
}
```

**When to use sync.Pool vs caller-passed buffer:**

| Situation | Use |
|---|---|
| Caller has a buffer in scope | Pass it as parameter |
| Function called from 1-2 call sites | Add buf parameter to those callers |
| Function called from many sites, scratch is internal | `sync.Pool` |
| Buffer needed across goroutines | `sync.Pool` (goroutine-safe) |
| Single goroutine, sequential processing | Ring buffer or struct field |

**sync.Pool sizing:** Seed with the common-case size. The pool holds
same-max-size buffers. If a caller needs more, `append()` will grow the
slice and the grown slice returns to the pool for future reuse.

### Tracing Data Lifecycle Before Writing Code

Before writing any buffer/pool/allocation code, answer these questions
(from `ai/rules/before-writing-code.md`):

1. **Where is the buffer allocated?** Name the function and pool.
2. **Who holds it?** Which goroutine/struct owns the reference.
3. **When is it copied?** Only at the boundaries listed in "When Copies Happen."
4. **When is it released?** After TCP write, after pool dedup, after use.
5. **Could the caller provide this buffer?** If yes, change the signature.
6. **Could a pool provide this buffer?** If yes, Get/Put around the use.

## Decision Tree: Where Should This Buffer Come From?

```
Is this on a per-UPDATE / per-route / per-NLRI path?
├── YES: Does the caller already have a buffer in scope?
│   ├── YES: Add buf/off parameters, write into it (WriteTo pattern)
│   └── NO: Is there a pool for this goroutine shape?
│       ├── YES: Get from pool, use, Put back
│       └── NO: Can the buffer be a struct field reused across calls?
│           ├── YES: Store on the struct, reset between uses
│           └── NO: Create a sync.Pool for this use case
└── NO (startup, config, CLI, one-shot):
    ├── One-shot allocation → make() is fine
    ├── String building → textbuf.Buffer (stack, 128B inline)
    └── fmt.Sprintf → acceptable on cold paths
```

## Common Mistakes

| Mistake | Why it's wrong | Fix |
|---|---|---|
| `make([]byte, n)` in a per-UPDATE function | Allocates on every UPDATE | Get from pool or write into caller's buffer |
| `func Encode() []byte` returning allocated bytes | Caller must copy into its buffer | Change to `WriteTo(buf, off) int` |
| `fmt.Sprintf` in reactor/wire/attribute code | 2+ allocations per call | `textbuf.Buffer` or `textbuf.StringUint32()` |
| `addr.String()` in a loop | Allocates per iteration | `addr.AppendTo(buf[:0])` into stack buffer |
| Holding WireUpdate past readBuf return | WireUpdate references readBuf memory | Copy needed data before returning readBuf to pool |
| Building `[]string` + `strings.Join` in a loop | N+1 allocations | Single `textbuf.Buffer` outside the loop |
| `string(bytes)` + comparison in a filter | Allocates the string | Compare bytes directly or use typed value |
| `map[string]V` keyed by value from a known set | String keys cost: hash over bytes, GC scans pointers | `map[uint16]V` or `map[TypedEnum]V`; parse string at boundary (`ai/rules/enum-over-string.md`) |
| `BufHandle{Buf: make(...)}` | Corrupts pool tracking | Only use pool-issued BufHandles (hook `block-fake-bufhandle.sh` enforces) |

## Related Documents

| Document | Covers |
|---|---|
| `ai/rules/buffer-first.md` | Mechanical rules for wire encoding |
| `ai/rules/no-sprintf-alloc.md` | String formatting alternatives, textbuf reference |
| `ai/rules/design-principles.md` | Encapsulation onion, lazy-over-eager, pool strategy |
| `docs/architecture/pool-architecture.md` | API program attribute dedup pools |
| `docs/architecture/encoding-context.md` | ContextID, zero-copy forwarding |
| `docs/architecture/forward-congestion-pool.md` | Two-tier forward pool, per-peer workers |
| `docs/architecture/buffer-architecture.md` | Overall buffer design |
| `docs/architecture/core-design.md` | System architecture |
| `/ze-find-alloc` | Audit for remaining allocations |
| `/ze-fix-alloc` | Fix a specific allocation |
