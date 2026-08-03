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

- **A copy that fits none of these four categories is probably wrong. Ask why the copy is needed.**

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

- **All buffers in a pool are the same maximum size. No variable-sized allocation.**

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

- **The caller always owns the buffer. The callee writes into `buf[off:]` and returns the number of bytes written. No allocations.**

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

1. Buffer from? → Pool or caller-provided
2. `append()`? → Offset writes
3. Returning `[]byte` from helper? → `writeFoo(buf, off) int`
4. `make([]byte)`? → Get from pool
5. Type has `WriteTo`? → Use it

Enforced by the `encoding-alloc` check in `.claude/hooks/pretool-writeedit.py`
(BLOCKING). Audit: `/ze-find-alloc`. Fix: `/ze-fix-alloc file:line`.

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

**Allocate once at the outermost scope, pass inward.** The caller knows:
- How many times the callee will be called (loop count)
- What buffer size is needed (often a bounded maximum)
- When the buffer can be released (after all callees are done)

- **The callee never guesses at these. It writes into what it is given.**

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
(from `ai/rules/architecture.md`):

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
| `map[string]V` keyed by value from a known set | String keys cost: hash over bytes, GC scans pointers | `map[uint16]V` or `map[TypedEnum]V`; parse string at boundary (`ai/rules/go-standards.md`) |
| `BufHandle{Buf: make(...)}` | Corrupts pool tracking | Only use pool-issued BufHandles (hook `block-fake-bufhandle.sh` enforces) |

## Three Rules

1. **No `fmt` on hot paths.** Use append-based primitives instead.
2. **No `.String()` on hot paths.** Use `AppendTo` into a stack buffer instead.
3. **Store typed values, not strings.** Compare `netip.Addr` directly, not string representations.

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
`c_string_concat`). Every `+` between strings allocates a new backing array
and copies both sides. Use `textbuf.Buffer` instead.

The only exception is a compile-time constant expression where both sides are
untyped string literals: `const x = "foo" + "bar"` (the compiler folds these).

**Existing cold-path concatenation is cleanup-on-touch**, not a sweep target:
the tree carries ~300 legacy cold-path `+` sites (web page rendering, one-shot
CLI output). Convert them when you edit the surrounding code; never let one
survive on a hot path (the Hot Path Rule below has no legacy carve-out).

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

- **The standalone `textbuf.StringUint32(v)` functions are for single-value returns (the entire result is one formatted value). For anything multi-part, use a Buffer.**

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

**BLOCKING on hot paths.** Never store an IP address as a string when it will be
compared. Store `netip.Addr` and use `.Compare()` directly.

| Anti-pattern | Fix |
|-------------|-----|
| `type Foo struct { Addr string }` then `net.ParseIP(a.Addr).Compare(...)` | `type Foo struct { Addr netip.Addr }` then `a.Addr.Compare(b.Addr)` |
| Formatting to string, storing, parsing back for comparison | Parse once at construction, store typed, format only for display |
| `compareAddrs(a.PeerAddr, b.PeerAddr)` with string parsing | `a.PeerIP.Compare(b.PeerIP)` with `netip.Addr` field |

- **When a struct needs both the typed value (for comparison) and the string (for map keys or JSON), store both. Parse once at construction time.**

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

**Use `textbuf.Buffer` for all string building.** Package: `internal/core/textbuf`.

`Buffer` is a chainable builder with a 128-byte inline backing array.
`Reset()` uses `noescape` (same technique as `strings.Builder` via
`abi.NoEscape`) to break the self-referential slice from escape analysis.
`var b Buffer` stays on the stack for local use; the inline array avoids
any heap allocation for content <= 128B.

**Go compiler review gate:** The `noescape` trick mirrors `strings.Builder`
in `$(go env GOROOT)/src/strings/builder.go`. On every Go compiler update,
compare our `noescape` + `inlineSlice` against the stdlib `copyCheck` +
`abi.NoEscape`. If the stdlib changes technique, update ours to match.
Verify with `go build -gcflags='-m=2'` that `var b Buffer` does not show
`moved to heap`.

### Allocation tiers

**Tier 0: Zero allocations.** Buffer on the stack, string consumed locally.

```go
// Local use: 0 alloc (buffer on stack, inline array, no string created)
var b textbuf.Buffer
w.Write(b.Reset().Addr(addr).Byte(':').Uint16(port).Bytes())

// Map lookup: 0 alloc (compiler elides string([]byte) for map/switch)
var b textbuf.Buffer
val := m[string(b.Reset().Str(key).Bytes())]

// Pool loop: 0 alloc (amortized, string consumed before next Reset)
b := textbuf.Get()
defer b.Release()
for _, peer := range peers {
    key := b.Reset().Addr(peer.Addr).Byte(':').Uint16(peer.Port).Slice()
    val := lookupMap[key]
}

// AppendTo: 0 alloc (caller-owned buffer, no pool needed)
func (p *Peer) AppendTo(dst []byte) []byte {
    dst = textbuf.Addr(dst, p.Addr)
    dst = append(dst, ':')
    dst = textbuf.Uint(dst, uint64(p.Port))
    return dst
}
```

**Tier 1: One allocation (must return or store a string).**

```go
// var + String(): 1 alloc (string copy, buffer stays on stack)
var b textbuf.Buffer
return b.Reset().Str("0:").Uint16(asn).Byte(':').Uint32(assigned).String()

// New() + Slice(): 1 alloc (Buffer struct ~160B, zero-copy extract)
b := textbuf.New()
return b.Str("0:").Uint16(asn).Byte(':').Uint32(assigned).Slice()

// Pool + String(): 1 alloc (string copy, buffer returns to pool)
b := textbuf.Get()
defer b.Release()
return b.Str("0:").Uint16(asn).Byte(':').Uint32(assigned).String()
```

- **Pool + Slice without Release exhausts the pool or dangles. Do not use.**

### Choosing an init

| Pattern | Init | Extract | Allocs |
|---------|------|---------|--------|
| Local use, write to io.Writer | `var b Buffer` | `Bytes()` | 0 |
| Local use, map lookup / comparison | `var b Buffer` | `string(b.Bytes())` | 0 |
| Hot loop, string consumed immediately | `Get()` + `defer Release()` | `Slice()` or `Bytes()` | 0 |
| Caller-owned buffer | `AppendTo` functions | none | 0 |
| Return a string (single use) | `var b Buffer` | `String()` | 1 |
| Return a string (zero-copy) | `New()` | `Slice()` | 1 |
| Return a string, reuse buffer | `Get()` + `defer Release()` | `String()` | 1 |

Methods (all return `*Buffer` for chaining):

| Method | Use |
|--------|-----|
| `Str(s)` | Append string literal or variable |
| `Byte(c)` | Append separator or single char |
| `Uint(v uint64)` | Append decimal uint64 |
| `Uint8(v)`, `Uint16(v)`, `Uint32(v)` | Typed variants (no cast at call site) |
| `Int(v int64)` | Append decimal int64 |
| `Float(v, prec)` | Append float with N decimal places |
| `Float2(v)` | Append float with 2 decimal places |
| `Bool(v)` | Append "true" or "false" |
| `Addr(a netip.Addr)` | Append IP address |
| `Prefix(p netip.Prefix)` | Append CIDR prefix (e.g. "10.0.0.0/24") |
| `Hex(data []byte)` | Append lowercase hex |
| `HexUpper(data)` | Append uppercase hex |
| `MAC(mac []byte)` | Append MAC address (e.g. "de:ad:be:ef:ca:fe") |
| `Quoted(s)` | Append Go-quoted string with escapes (wraps in `"..."`) |
| `Err(err)` | Append error string (nil-safe, no-op on nil) |
| `Join(items, sep)` | Append strings joined by separator |
| `PadRight(s, width)` | Append `s` then spaces to fill `width` (rune-aware) |
| `PadLeft(s, width)` | Prepend spaces then `s` to fill `width` (rune-aware) |
| `Repeat(s, n)` | Append `s` N times (indentation, padding) |
| `Grow(n)` | Pre-grow capacity to avoid mid-chain reallocation |
| `String()` | Return built string (single alloc for inline, zero-copy for heap). Does NOT freeze: writes continue safely |
| `Slice()` | Return string **zero-copy at any size**. Freezes buffer: writes panic until `Reset()` |
| `Bytes()` | Return raw `[]byte` (shares buffer memory). For `w.Write()` or `string()` in map/switch (compiler elides alloc) |
| `Reset()` | Clear the buffer for reuse. Resets to inline array. Chainable |
| `Write(p)` | Append raw bytes (implements `io.Writer`) |
| `WriteString(s)` | Append string (implements `io.StringWriter`). Returns `(int, error)` |
| `WriteByte(c)` | Append byte (implements `io.ByteWriter`). Returns `error` |
| `WriteRune(r)` | Append rune. Returns `(int, error)` |
| `Len()` | Current content length |

### String() vs Slice()

**Prefer `Slice()` when the string is consumed immediately** (passed to a
function, used as a map lookup, parsed, or appended into another buffer).
`Slice()` does zero allocations at any size. `String()` copies inline data
(<=128B) and does zero-copy for heap data (>128B).

- **Prefer `Slice()` by default.** Most strings are passed to a function (ParsePrefix, map lookup, Write) and discarded, and `Slice()` saves the copy.

| Result lifetime | Use | Allocations |
|----------------|-----|-------------|
| Returned from function (single use) | `Slice()` | 0 (Buffer on heap; GC traces interior pointer) |
| Stored in a struct field | `String()` | 1 (inline copy) or 0 (heap transfer) |
| Consumed before `Reset()`/`Release()` | `Slice()` | 0 |
| Passed to `netip.ParsePrefix()` etc. | `Slice()` | 0 (parser copies internally if needed) |
| Buffer reused after extraction | `String()` | 1 or 0 (does not freeze buffer) |

```go
// Slice: zero-copy, consumed immediately by ParsePrefix
entry, _ := netip.ParsePrefix(b.Reset().Addr(addr).Byte('/').Int(int64(bits)).Slice())

// String: result stored in a struct field
peer.Label = b.Reset().Str("AS").Uint32(asn).String()
```

### Reusing a Buffer with Reset()

Declare one `textbuf.Buffer` before a loop and call `Reset()` between
iterations. Each iteration reuses the same 128-byte inline array:

```go
var b textbuf.Buffer
for _, peer := range peers {
    key := b.Reset().Addr(peer.Addr).Byte(':').Uint16(peer.Port).Slice()
    lookupMap[key] = peer  // Slice valid until next Reset
}
```

For pooled buffers, `Get()`/`Release()` replaces the stack variable:

```go
b := textbuf.Get()
defer b.Release()
for _, p := range prefixes {
    formatted := b.Reset().Addr(p.Addr()).Byte('/').Int(int64(p.Bits())).Slice()
    process(formatted)
}
```

Standalone functions for single-value returns:

| Function | Returns |
|----------|---------|
| `textbuf.StringUint(v)`, `textbuf.StringUint8(v)`, `textbuf.StringUint16(v)`, `textbuf.StringUint32(v)` | Decimal string |
| `textbuf.StringInt(v)` | Signed decimal string |
| `textbuf.StringAddr(a)` | IP address string |
| `textbuf.StringPrefix(p)` | CIDR prefix string |
| `textbuf.StringHex(data)`, `textbuf.StringHexUpper(data)` | Hex-encoded string |
| `textbuf.StringMAC(mac)` | MAC address string (e.g. "de:ad:be:ef:ca:fe") |
| `textbuf.HostPort(host, port)` | "host:port" string |
| `textbuf.Join(items, sep)` | Joined string (replaces `strings.Join`) |
| `textbuf.StrInt(prefix, v)` | "prefix" + decimal |
| `textbuf.StrUint(prefix, v)` | "prefix" + unsigned decimal |
| `textbuf.IntStr(v, suffix)` | Decimal + "suffix" |
| `textbuf.UintStr(v, suffix)` | Unsigned decimal + "suffix" |
| `textbuf.StrIntStr(prefix, v, suffix)` | "prefix" + decimal + "suffix" |
| `textbuf.StrUintStr(prefix, v, suffix)` | "prefix" + unsigned decimal + "suffix" |

Append-into-buffer functions (for wire encoding / hot paths):

| Function | Appends |
|----------|---------|
| `textbuf.Uint(dst, v)` | Decimal bytes |
| `textbuf.Int(dst, v)` | Signed decimal bytes |
| `textbuf.Addr(dst, a)` | IP address bytes |
| `textbuf.Prefix(dst, p)` | CIDR prefix bytes |
| `textbuf.Hex(dst, data)` | Hex-encoded bytes |
| `textbuf.HexUpper(dst, data)` | Uppercase hex-encoded bytes |
| `textbuf.MAC(dst, mac)` | MAC address bytes |

| Pattern | Use |
|---------|-----|
| Multi-part string, stored | `var b textbuf.Buffer; return b.Str(...).Uint32(...).String()` |
| Multi-part string, consumed immediately | `var b textbuf.Buffer; parse(b.Str(...).Uint32(...).Slice())` |
| Reuse in a loop | `var b textbuf.Buffer; for ... { use(b.Reset().Str(...).Slice()) }` |
| Single value | `return textbuf.StringUint32(v)` or `return textbuf.StringAddr(a)` |
| Append into `[]byte` | `textbuf.Uint(dst, v)`, `textbuf.Addr(dst, a)` |

### Banned

| Anti-pattern | Fix |
|-------------|-----|
| `a + ":" + b` (any `+` concatenation) | `var b textbuf.Buffer; b.Str(a).Byte(':').Str(b).String()` |
| `fmt.Sprintf("%d:%d", a, b)` | `b.Uint32(a).Byte(':').Uint32(b).String()` |
| `strconv.Itoa(n) + ":" + strconv.Itoa(m)` | Same |
| `strings.Join(items, sep)` | `textbuf.Join(items, sep)` |
| `var buf [N]byte; b := append(buf[:0]...); return string(b)` | `var b textbuf.Buffer; return b.Str(...).String()` |
| `addr.String() + "/" + strconv.Itoa(n)` | `b.Addr(addr).Byte('/').Int(n).String()` |
| `uint64(v)` cast at call site | Use typed method: `Uint16(v)`, `Uint32(v)` |

### keyBuilder (for grouping keys with separators)

For fingerprint/grouping key builders with repeated `|` separators, embed
`strings.Builder` in a package-local `keyBuilder` with typed methods:

```go
type keyBuilder struct{ strings.Builder }
func (b *keyBuilder) Sep()                 { b.WriteByte('|') }
func (b *keyBuilder) Uint(v uint64)        { var buf [20]byte; b.Write(textbuf.Uint(buf[:0], v)) }
func (b *keyBuilder) Addr(addr netip.Addr) { var buf [39]byte; b.Write(textbuf.Addr(buf[:0], addr)) }
```

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

1. Am I using `+` to concatenate strings? Use `textbuf.Buffer` chain instead.
2. Am I using `fmt.Sprintf`? Use `textbuf.Buffer` or standalone functions.
3. Am I using `strings.Join`? Use `textbuf.Join` or `b.Join(items, sep)`.
4. Am I using `strings.Builder`? Use `textbuf.Buffer` (128B inline, poolable).
5. Am I calling `.String()` just to concatenate? Use `textbuf.Buffer.Addr()` etc.
6. Am I storing a string that will be parsed back for comparison? Store the typed value.
7. Am I building a string that gets immediately discarded? Split the function.
8. Could this error be a package-level sentinel?
9. Do I have multiple `var tb textbuf.Buffer` in one function? Use ONE buffer with `Reset()`.
10. Is my `.String()` result consumed immediately (function arg, comparison)? Use `.Slice()`.

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
scope (return, struct field, map insert, channel send), use `String()` with
`var b Buffer`, or `Slice()` with `New()`. Slice-from-stack is only safe when
consumed before the buffer goes out of scope (function arg, map lookup, comparison).

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
