# Memory and Encoding

**When:** before writing buffer, pool, allocation, string-building, or wire-encoding code
**Severity:** blocking
**Related:** architecture, go-standards, repo-maintenance

## Directives

- **All wire encoding MUST write into pooled, bounded buffers.**
- **Never use `fmt.Sprintf`, `fmt.Fprintf`, or `fmt.Errorf` when a zero-allocation or lower-allocation alternative exists.**
- **Never use `.String()` concatenation on a hot path when an append-into-buffer pattern exists.**
- **Read this file before any allocation or memory-lifecycle decision.** It carries the conceptual model (data lifecycle, caller-owned buffers, pool strategy) and the mechanical rules for encoding and string formatting.
- Principle: `ai/rules/architecture.md` -- Encapsulation onion + Buffer-first encoding.
- Rationale: `ai/rationale/buffer-first.md`
- Reference: git log -- plan/analysis-printf-allocations.md (completed, removed from tree)

## The Core Idea

Ze processes millions of BGP UPDATEs per second. Each UPDATE touches: wire
parsing, attribute extraction, pool dedup, RIB storage, route selection,
filter evaluation, UPDATE building, and TCP write. Every allocation on
this path adds GC pressure and latency. Ze eliminates allocations through
three interlocking strategies:

1. **Buffer ownership** -- the caller MUST own the buffer, and the callee MUST write into it
2. **Pool lifecycle** -- bounded pools MUST replace unbounded `make()`
3. **Lazy parsing** -- raw byte slices with offset iterators MUST be used, not parsed structs

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

- **A copy that fits none of these four categories SHOULD be treated as wrong. The reason the copy is needed MUST be asked.**

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

- **All buffers in a pool MUST be the same maximum size. Variable-sized allocation MUST NOT occur.**

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

- **The caller MUST own the buffer. The callee MUST write into `buf[off:]` and return the number of bytes written. Allocations MUST NOT occur.**

## Buffer-First Encoding: Mechanical Reference

| Pool | Size | Purpose |
|------|------|---------|
| `readBufPool4K` | 4096 | Standard message reads |
| `readBufPool64K` | 65535 | Extended message reads |
| `buildBufPool` | 4096 | UPDATE building |
| Per-plugin `nlriBufPool` | 4096 | NLRI encoding |

### Pattern: Get → Write → Put

Get buffer from pool → write with `WriteTo(buf, off) int` → return to pool.

### Pattern: Skip-and-Backfill (hot path)

For messages with variable-length sections and fixed-position length fields:

**Skip-and-backfill MUST follow these steps:**

1. Write fixed bytes (marker, type)
2. **Skip** length field -- save position (`lengthPos := off; off += 2`)
3. Write payload forward at advancing offset
4. **Backfill** length at saved position (`buf[lengthPos] = byte(totalLen >> 8)`)

This avoids the `Len()`-then-`WriteTo()` double traversal. See `reactor_wire.go` for the canonical implementation.

### Banned in Encoding Code

| Banned | Use Instead |
|--------|-------------|
| `append(buf, ...)` | Pre-computed size + write at offset |
| `make([]byte, N)` in helpers | Write into caller's pool buffer |
| `buildFoo() ([]byte, error)` | `writeFoo(buf, off) int` |
| `.Bytes()` | `.WriteTo(buf, off)` + `.Len()` |
| `.Pack()` | `.WriteTo(buf, off)` |
| `x.Len()` then `x.WriteTo()` in hot path | Skip-and-backfill, or `WriteAttrToWithLen()` |

### `make([]byte)` IS OK For

Pool `New` func, session buffer creation, cached encoding, result copies to callers, JSON marshaling, tests, IPC framing, config parsing.

### Before Writing Encoding Code

**These questions MUST be answered before writing encoding code:**

1. Buffer from? → Pool or caller-provided
2. `append()`? → Offset writes
3. Returning `[]byte` from helper? → `writeFoo(buf, off) int`
4. `make([]byte)`? → Get from pool
5. Type has `WriteTo`? → Use it

`writeGoPatterns` in `internal/le/hookruntime/writeedit.go` blocks allocation-heavy formatting and fake buffer handles. Audit the full encoding path with `/ze-find-alloc`; fix it with `/ze-fix-alloc file:line`.

### Text/JSON Format Generation

The same rule covers the BGP text/JSON format-generation files migrated by
fmt-0 and fmt-2-json-append (the `format-alloc` hook check is currently a
no-op, see `ai/rules/repo-maintenance.md`; the rule still applies): every file
that emits OPEN / NOTIFICATION / ROUTE-REFRESH / NEGOTIATED text or JSON is
covered, and `fmt.Sprintf`, `fmt.Fprintf`, `strings.Builder`,
`strings.Join`, `strings.NewReplacer`, `strings.ReplaceAll`,
`strconv.FormatUint`, `strconv.FormatInt` are rejected at Write/Edit time.
Allowed helpers: `strconv.AppendUint`, `netip.Addr.AppendTo`,
`hex.AppendEncode`, or a local `[N]byte` scratch plus `append`. `json.go`
is intentionally excluded while its `map[string]any` + `json.Marshal`
idiom remains.

## Caller-Owned Resources (BLOCKING on hot paths)

The single most common allocation mistake: a callee allocates a buffer
internally when the caller could pass one down.

### The Principle

**Allocation MUST happen once, at the outermost scope, and MUST pass inward.** The caller knows:
- How many times the callee will be called (loop count)
- What buffer size is needed (often a bounded maximum)
- When the buffer can be released (after all callees are done)

- **The callee MUST NOT guess at these. It MUST write into what it is given.**

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

**Whether to use sync.Pool or a caller-passed buffer MUST be decided using the criteria below:**

| Situation | Use |
|---|---|
| Caller has a buffer in scope | Pass it as parameter |
| Function called from 1-2 call sites | Add buf parameter to those callers |
| Function called from many sites, scratch is internal | `sync.Pool` |
| Buffer needed across goroutines | `sync.Pool` (goroutine-safe) |
| Single goroutine, sequential processing | Ring buffer or struct field |

**sync.Pool sizing:** The pool MUST be seeded with the common-case size. The pool holds
same-max-size buffers. If a caller needs more, `append()` will grow the
slice and the grown slice returns to the pool for future reuse.

### Tracing Data Lifecycle Before Writing Code

Before writing any buffer/pool/allocation code, answer these questions
(from `ai/rules/architecture.md`):

**A buffer's lifecycle MUST be traced before writing code:**

1. **Where is the buffer allocated?** The function and pool MUST be named.
2. **Who holds it?** Which goroutine/struct owns the reference.
3. **When is it copied?** Only at the boundaries listed in "When Copies Happen."
4. **When is it released?** After TCP write, after pool dedup, after use.
5. **Could the caller provide this buffer?** If yes, the signature MUST change.
6. **Could a pool provide this buffer?** If yes, Get/Put MUST wrap the use.

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
| `map[string]V` keyed by value from a known set | String keys cost: hash over bytes, GC scans pointers | `map[uint16]V` or `map[TypedEnum]V`; parse string at boundary (`ai/rules/go-standards.md`) |
| `BufHandle{Buf: make(...)}` | Corrupts pool tracking | Only use pool-issued BufHandles; `writeGoPatterns` in `internal/le/hookruntime/writeedit.go` enforces |

## Three Rules

1. **`fmt` MUST NOT be used on hot paths.** Append-based primitives MUST be used instead.
2. **`.String()` MUST NOT be used on hot paths.** `AppendTo` MUST be used into a stack buffer instead.
3. **Typed values MUST be stored, not strings.** `netip.Addr` MUST be compared directly, not string representations.

## Decision Tree: Before Writing Any `fmt.Sprintf`

Before writing any `fmt.Sprintf` (or `Fprintf`, `Errorf`):

```
1. Is the format string a constant with no verbs?
   → errors.New (for Errorf), literal string (for Sprintf)

2. Is the only verb %d?
   → textbuf.StringUint(uint64(n)) or textbuf.StringInt(int64(n))

3. Is it a string with separators or mixed types?
   → textbuf.Buffer chain: b.Str(a).Byte(':').Str(b).String()

4. Is it formatting an IP address?
   → textbuf.StringAddr(a) or textbuf.StringPrefix(p)

5. Is it formatting a MAC address?
   → textbuf.StringMAC(mac)

6. Is it host:port?
   → textbuf.HostPort(host, port)

7. Is it hex formatting (%x, %02x, %X)?
   → textbuf.StringHex(data) or textbuf.StringHexUpper(data)
   → textbuf.Hex(buf, data) for hot paths

8. Is it joining strings with a separator?
   → textbuf.Join(items, sep) or b.Join(items, sep)

9. Is it on a hot path (per-UPDATE, per-route, per-NLRI)?
   → AppendTo(buf []byte) []byte pattern (see text_append.go)
   → textbuf.Uint, textbuf.Int, textbuf.Addr, textbuf.Prefix, textbuf.Hex, textbuf.HexUpper, textbuf.MAC
   → Never fmt.Sprintf. No exceptions.
   → Never .String() + concatenation. Use stack buffer.

10. Is it writing to an io.Writer?
    → w.Write([]byte(...)) or io.WriteString(w, s)
    → strconv.AppendInt into a [20]byte scratch, then w.Write

11. None of the above?
    → fmt.Sprintf is acceptable (cold path, complex format)
```

## Banned Patterns

### String concatenation with `+` is BANNED in new code

**BLOCKING for new code and all hot paths** (hook-enforced at edit time by
`c_string_concat`). The `+` operator MUST NOT be used between strings: it
allocates a new backing array and copies both sides. `textbuf.Buffer` MUST be
used instead.

The only exception is a compile-time constant expression where both sides are
untyped string literals: `const x = "foo" + "bar"` (the compiler folds these).

**Existing cold-path concatenation is cleanup-on-touch**, not a sweep target:
the tree carries ~300 legacy cold-path `+` sites (web page rendering, one-shot
CLI output). They MUST be converted when the surrounding code is edited; one
MUST NOT survive on a hot path (the Hot Path Rule below has no legacy carve-out).

| `+` pattern | Replacement |
|-------------|-------------|
| `a + "/" + b` | `var tb textbuf.Buffer; tb.Str(a).Byte('/').Str(b).String()` |
| `"prefix:" + s` | `var b textbuf.Buffer; b.Str("prefix:").Str(s).String()` |
| `s + strconv.Itoa(n)` | `var b textbuf.Buffer; b.Str(s).Int(int64(n)).String()` |
| `addr.String() + "/" + strconv.Itoa(n)` | `var b textbuf.Buffer; b.Addr(addr).Byte('/').Int(int64(n)).String()` |
| `">" + textbuf.StringUint(v)` | `var b textbuf.Buffer; b.Byte('>').Uint(v).String()` |
| `strings.Join(items, sep)` | `textbuf.Join(items, sep)` |

In loops the cost compounds. Each iteration allocates, and collecting into
`[]string` + `strings.Join` adds yet another:

```go
// BAD: 2 allocations (Uint result + concat)
return "peer:" + textbuf.StringUint32(asn)

// GOOD: 1 allocation (String())
var b textbuf.Buffer
return b.Str("peer:").Uint32(asn).String()

// BAD: N*2 + 1 allocations
parts := make([]string, len(items))
for i, m := range items {
    parts[i] = ">" + textbuf.StringUint(m.Value)
}
return strings.Join(parts, " ")

// GOOD: 1 allocation (String())
var b textbuf.Buffer
for _, m := range items {
    b.Byte(' ').Byte('>').Uint(m.Value)
}
return b.String()
```

- **The standalone `textbuf.StringUint32(v)` functions MUST be used only for single-value returns (the entire result is one formatted value). For anything multi-part, a Buffer MUST be used.**

### fmt patterns

| Pattern | Replacement |
|---------|-------------|
| `fmt.Sprintf("%d", n)` | `textbuf.StringInt(int64(n))` (standalone) or `b.Int(int64(n))` (in chain) |
| `fmt.Sprintf("%s", s)` | `s` |
| `fmt.Sprintf("%s:%d", s, n)` | `var b textbuf.Buffer; b.Str(s).Byte(':').Int(int64(n)).String()` |
| `fmt.Sprintf("%d:%d", a, b)` | `var b textbuf.Buffer; b.Int(int64(a)).Byte(':').Int(int64(b)).String()` |
| `fmt.Sprintf("%d.%d.%d.%d", a,b,c,d)` | `netip.AddrFrom4` + `textbuf.StringAddr(addr)` |
| `fmt.Sprintf("%02x:%02x:...", mac...)` | `textbuf.StringMAC(mac)` |
| `fmt.Sprintf("%x", data)` | `textbuf.StringHex(data)` or `textbuf.Hex(buf, data)` |
| `fmt.Sprintf("%X", data)` | `textbuf.StringHexUpper(data)` or `textbuf.HexUpper(buf, data)` |
| `fmt.Sprintf("%q", s)` | `var b textbuf.Buffer; b.Quoted(s).String()` |
| `fmt.Sprintf("%.1f", v)` | `var b textbuf.Buffer; b.Float(v, 1).String()` |
| `fmt.Sprintf("ctx: %v", err)` | `var b textbuf.Buffer; b.Str("ctx: ").Err(err).String()` |
| `fmt.Sprintf("%-20s", s)` | `var b textbuf.Buffer; b.PadRight(s, 20).String()` |
| `fmt.Sprintf("%6s", s)` | `var b textbuf.Buffer; b.PadLeft(s, 6).String()` |
| `fmt.Sprintf("127.0.0.1:%d", port)` | `textbuf.HostPort("127.0.0.1", port)` |
| `fmt.Errorf("constant string")` | `var ErrFoo = errors.New("constant string")` at package level |
| `fmt.Fprintf(w, "%s", s)` | `io.WriteString(w, s)` |
| `fmt.Fprintf(w, "%d", n)` | `io.WriteString(w, textbuf.StringInt(int64(n)))` |
| Sprintf in a function that discards the result | Split into no-alloc + with-string variants |

### strings patterns

| Pattern | Replacement |
|---------|-------------|
| `strings.Join(items, ", ")` | `textbuf.Join(items, ", ")` |
| `strings.Builder` + loop | `textbuf.Buffer` + loop with `Reset()` |
| `strings.Repeat(s, n)` | `var b textbuf.Buffer; b.Repeat(s, n).String()` |
| `b.WriteString(strings.Repeat("\t", indent))` | `b.Repeat("\t", indent)` |

### Other

| Pattern | Replacement |
|---------|-------------|
| Storing `string` then parsing back for comparison | Store `netip.Addr` (or typed value), compare directly |
| `net.ParseIP(s)` in a comparison function | Store `netip.Addr` at construction, use `.Compare()` |
| `parts[i] = prefix + strconv.Itoa(n)` in a loop | Single Buffer: `b.Str(prefix).Uint(uint64(n))` |
| `fmt.Fprintf(b, "%-*s...", width, s, ...)` | `b.PadRight(s, width).Str(...)` |

## Typed Comparison Rule

**BLOCKING on hot paths.** An IP address MUST NOT be stored as a string when it
will be compared. `netip.Addr` MUST be stored and `.Compare()` MUST be used
directly.

| Anti-pattern | Fix |
|-------------|-----|
| `type Foo struct { Addr string }` then `net.ParseIP(a.Addr).Compare(...)` | `type Foo struct { Addr netip.Addr }` then `a.Addr.Compare(b.Addr)` |
| Formatting to string, storing, parsing back for comparison | Parse once at construction, store typed, format only for display |
| `compareAddrs(a.PeerAddr, b.PeerAddr)` with string parsing | `a.PeerIP.Compare(b.PeerIP)` with `netip.Addr` field |

- **When a struct needs both the typed value (for comparison) and the string (for map keys or JSON), both MUST be stored. Parsing MUST happen once, at construction time.**

```go
type Candidate struct {
    PeerAddr string     // for map keys, JSON, interning
    PeerIP   netip.Addr // for comparison (zero-alloc)
}

// At construction:
c.PeerAddr = peerAddr
c.PeerIP, _ = netip.ParseAddr(peerAddr)
```

## Hot Path Rule

Code in these paths MUST NOT use any `fmt` function or `.String()` concatenation:

| Path | Examples |
|------|----------|
| Wire encoding/decoding | `message/`, `wireu/`, `attribute/` |
| Per-UPDATE processing | `reactor/`, `adj_rib_in/`, `persist/`, `rr/`, `rs/` |
| Per-route evaluation | `rib/bestpath.go`, `rib/event.go`, `rib/route.go` |
| NLRI parsing/formatting | `nlri/*/`, `rib_nlri.go` |
| Filter chain | `reactor/filter_*.go` |

Use the `AppendTo(buf []byte) []byte` pattern from `attribute/text_append.go` instead.

## Allowed fmt Usage

| Context | Why |
|---------|-----|
| CLI output (`fmt.Println`, `fmt.Fprintf(os.Stdout, ...)`) | User-facing, runs once |
| Startup/shutdown messages | Runs once |
| Config parsing/validation errors | Runs once at load |
| Web page rendering (cold path) | Acceptable if not in a per-route loop |
| Test assertions and sub-test naming | Not production code |
| `fmt.Errorf("context: %w", err)` with non-constant context | Error wrapping is the intended use |
| `fmt.Sprintf("%T", value)` | Reflect-based type name; no textbuf equivalent exists |
| `fmt.Sprintf("%v", data)` where `data` is `any`/`interface{}` | Arbitrary-type formatting; no textbuf path |
| `http.Error(w, fmt.Sprintf(...))` | One-shot error response, not in a loop |

## Patterns That Must NOT Be Converted

| Pattern | Why |
|---------|-----|
| `net.JoinHostPort(host, port)` where port is already a string and no numeric conversion is needed | Acceptable when the port comes from `net.SplitHostPort` or config as a string; prefer `textbuf.HostPort` for new code |
| `strings.Builder` as a long-lived `io.Writer` field | Struct fields like `pasteBuffer *strings.Builder` that accumulate writes over time are not string-building helpers. `textbuf.Buffer` freezes on `Slice()` and its pool semantics differ |
| `strconv.Itoa(n)` passed to sysctl/procfs map values | Writing to kernel interfaces that require `string`; the allocation is once per config reload, not per-packet |

## textbuf.Buffer (canonical string builder)

**`textbuf.Buffer` MUST be used for all string building.** Package: `internal/core/textbuf`. Its methods, the standalone helpers, the allocation tiers and the anti-pattern table are `docs/architecture/textbuf-string-building.md`.

- **`Slice()` SHOULD be used by default.** Most strings are passed to a function (ParsePrefix, map lookup, Write) and discarded, and `Slice()` saves the copy.

- **Pool + Slice without Release exhausts the pool or dangles. It MUST NOT be used.**

**On every Go compiler update, `noescape` and `inlineSlice` MUST be compared
against the stdlib `copyCheck` and `abi.NoEscape` in
`$(go env GOROOT)/src/strings/builder.go`, and MUST be updated to match when
the stdlib changes technique.** It MUST then be verified with
`go build -gcflags='-m=2'` that `var b Buffer` does not show `moved to heap`.
Why the trick is there is `docs/architecture/textbuf-string-building.md`.

## Types Own Their Serialization

- **Named types MUST have an `AppendTo([]byte) []byte` method. Callers never format a type from the outside.**
- **Callers chain: `buf = typeA.AppendTo(typeB.AppendTo(buf[:0]))`.**

When a struct field is a plain `uint8`/`uint32` but represents a domain concept
(Origin, MED, ASN), it should use a named type with formatting methods. If
changing the field type is too large for the current task, use `textbuf.Buffer`
methods as a stepping stone, but track the typed refactor as follow-up.

### AppendTo Pattern (for types)

```go
func (t *MyType) AppendTo(buf []byte) []byte {
    buf = append(buf, "prefix "...)
    buf = textbuf.Uint(buf, uint64(t.Field))
    buf = append(buf, ':')
    buf = textbuf.Addr(buf, t.Addr)
    return buf
}

func (t *MyType) String() string {
    var b textbuf.Buffer
    // Call AppendTo on Buffer's internal slice... or just chain:
    return textbuf.StringAddr(t.Addr)
}
```

## Self-Check

Before submitting code that builds strings:

1. Am I using `+` to concatenate strings? `textbuf.Buffer` chain MUST be used instead.
2. Am I using `fmt.Sprintf`? `textbuf.Buffer` or standalone functions MUST be used.
3. Am I using `strings.Join`? `textbuf.Join` or `b.Join(items, sep)` MUST be used.
4. Am I using `strings.Builder`? `textbuf.Buffer` MUST be used (128B inline, poolable).
5. Am I calling `.String()` just to concatenate? `textbuf.Buffer.Addr()` etc. MUST be used.
6. Am I storing a string that will be parsed back for comparison? The typed value MUST be stored.
7. Am I building a string that gets immediately discarded? The function MUST be split.
8. Could this error be a package-level sentinel?
9. Do I have multiple `var tb textbuf.Buffer` in one function? ONE buffer MUST be used, with `Reset()`.
10. Is my `.String()` result consumed immediately (function arg, comparison)? `.Slice()` MUST be used.

## Conversion Anti-Patterns (BLOCKING)

These errors recur during mechanical `+` → textbuf conversion. Check explicitly.

### Multiple buffers in one function

```go
// BAD: tb2, tb3, tb4 waste stack space
var tb2 textbuf.Buffer
msg2 := tb2.Str("Deleted ").Join(path, " ").String()
var tb3 textbuf.Buffer
msg3 := tb3.Str("Renamed ").Str(name).String()

// GOOD: one buffer, Reset between uses
var tb textbuf.Buffer
msg2 := tb.Str("Deleted ").Join(path, " ").String()
msg3 := tb.Reset().Str("Renamed ").Str(name).String()
```

### String() where Slice() suffices

```go
// BAD: String() copies when the value is consumed immediately
emitLine(b, tb.Reset().Str(prefix).Str(name).String())

// GOOD: Slice() is zero-copy; valid until next Reset()
emitLine(b, tb.Reset().Str(prefix).Str(name).Slice())
```

Use `Slice()` when the result is:
- Returned from a function (Buffer is heap-allocated; GC keeps it alive)
- Passed directly to a function call (consumed immediately)
- Used as a map key lookup (not insertion)
- Compared with `==` or passed to `strings.HasPrefix`
- The last extraction before the buffer goes out of scope

Use `String()` when the result is:
- Stored in a struct field or variable that outlives the next `Reset()`
- Inserted as a map key (must own the memory)
- Extracted mid-chain and the buffer is reused afterward

### Slice() from stack-allocated Buffer (the escape trap)

During bulk `+` → textbuf conversions, it is tempting to use `Slice()` everywhere
for zero-copy. But `var b Buffer` uses `noescape` to stay on the stack, so
`Slice()` returns a string pointing into stack memory. If that string escapes
the function (returned, stored in a struct, sent to a goroutine), it dangles.

```go
// BAD: Slice points into stack buffer; dangling after return
var b textbuf.Buffer
return b.Reset().Str("peer:").Uint32(asn).Slice()

// GOOD: String copies data out; safe to return
var b textbuf.Buffer
return b.Reset().Str("peer:").Uint32(asn).String()

// ALSO GOOD: heap Buffer, Slice is safe (GC traces interior pointer)
b := textbuf.New()
return b.Str("peer:").Uint32(asn).Slice()
```

**Mechanical rule for bulk conversions:** when the result leaves the current
scope (return, struct field, map insert, channel send), `String()` with
`var b Buffer` MUST be used, or `Slice()` with `New()` MUST be used.
Slice-from-stack MUST be consumed before the buffer goes out of scope
(function arg, map lookup, comparison).

### Unnecessary scratch buffer when output buffer exists

```go
// BAD: scratch tb just to write into b
func render(b *textbuf.Buffer, name string) {
    var tb textbuf.Buffer
    b.Str(tb.Str("prefix:").Str(name).String())
}

// GOOD: write directly to the output buffer
func render(b *textbuf.Buffer, name string) {
    b.Str("prefix:").Str(name)
}
```

## Related Documents

| Document | Covers |
|---|---|
| `ai/rules/architecture.md` | Encapsulation onion, lazy-over-eager, pool strategy |
| `docs/architecture/pool-architecture.md` | API program attribute dedup pools |
| `docs/architecture/encoding-context.md` | ContextID, zero-copy forwarding |
| `docs/architecture/forward-congestion-pool.md` | Two-tier forward pool, per-peer workers |
| `docs/architecture/buffer-architecture.md` | Overall buffer design |
| `docs/architecture/core-design.md` | System architecture |
| `/ze-find-alloc` | Audit for remaining allocations |
| `/ze-fix-alloc` | Fix a specific allocation |
