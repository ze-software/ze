# No Printf Allocations

**BLOCKING:** Never use `fmt.Sprintf`, `fmt.Fprintf`, or `fmt.Errorf` when a
zero-allocation or lower-allocation alternative exists. Never use `.String()`
concatenation on a hot path when an append-into-buffer pattern exists.
Conceptual model: `rules/memory-architecture.md` -- data lifecycle, caller-owned buffers.
Reference: git log -- plan/analysis-printf-allocations.md (completed, removed from tree)

## Three Rules

1. **No `fmt` on hot paths.** Use append-based primitives instead.
2. **No `.String()` on hot paths.** Use `AppendTo` into a stack buffer instead.
3. **Store typed values, not strings.** Compare `netip.Addr` directly, not string representations.

## Decision Tree

Before writing any `fmt.Sprintf` (or `Fprintf`, `Errorf`):

```
1. Is the format string a constant with no verbs?
   → errors.New (for Errorf), literal string (for Sprintf)

2. Is the only verb %d?
   → textbuf.Uint(uint64(n)) or textbuf.Int(int64(n))

3. Is it a string with separators or mixed types?
   → textbuf.Buffer chain: b.Str(a).Byte(':').Str(b).String()

4. Is it formatting an IP address?
   → textbuf.Addr(a) or textbuf.Prefix(p)

5. Is it formatting a MAC address?
   → textbuf.MAC(mac)

6. Is it host:port?
   → textbuf.HostPort(host, port)

7. Is it hex formatting (%x, %02x, %X)?
   → textbuf.Hex(data) or textbuf.HexUpper(data)
   → textbuf.AppendHex(buf, data) for hot paths

8. Is it joining strings with a separator?
   → textbuf.Join(items, sep) or b.Join(items, sep)

9. Is it on a hot path (per-UPDATE, per-route, per-NLRI)?
   → AppendTo(buf []byte) []byte pattern (see text_append.go)
   → textbuf.AppendUint, textbuf.AppendAddr, textbuf.AppendPrefix
   → Never fmt.Sprintf. No exceptions.
   → Never .String() + concatenation. Use stack buffer.

10. Is it writing to an io.Writer?
    → w.Write([]byte(...)) or io.WriteString(w, s)
    → strconv.AppendInt into a [20]byte scratch, then w.Write

11. None of the above?
    → fmt.Sprintf is acceptable (cold path, complex format)
```

## Banned Patterns

### String concatenation with `+` is BANNED

**BLOCKING.** Never use `+` to concatenate strings. Every `+` between strings
allocates a new backing array and copies both sides. Use `textbuf.Buffer` instead.

The only exception is a compile-time constant expression where both sides are
untyped string literals: `const x = "foo" + "bar"` (the compiler folds these).

| `+` pattern | Replacement |
|-------------|-------------|
| `a + "/" + b` | `var b textbuf.Buffer; b.Str(a).Byte('/').Str(b).String()` |
| `"prefix:" + s` | `var b textbuf.Buffer; b.Str("prefix:").Str(s).String()` |
| `s + strconv.Itoa(n)` | `var b textbuf.Buffer; b.Str(s).Int(int64(n)).String()` |
| `addr.String() + "/" + strconv.Itoa(n)` | `var b textbuf.Buffer; b.Addr(addr).Byte('/').Int(int64(n)).String()` |
| `">" + textbuf.Uint(v)` | `var b textbuf.Buffer; b.Byte('>').Uint(v).String()` |
| `strings.Join(items, sep)` | `textbuf.Join(items, sep)` |

### fmt patterns

| Pattern | Replacement |
|---------|-------------|
| `fmt.Sprintf("%d", n)` | `textbuf.Int(int64(n))` (standalone) or `b.Int(int64(n))` (in chain) |
| `fmt.Sprintf("%s", s)` | `s` |
| `fmt.Sprintf("%s:%d", s, n)` | `var b textbuf.Buffer; b.Str(s).Byte(':').Int(int64(n)).String()` |
| `fmt.Sprintf("%d:%d", a, b)` | `var b textbuf.Buffer; b.Int(int64(a)).Byte(':').Int(int64(b)).String()` |
| `fmt.Sprintf("%d.%d.%d.%d", a,b,c,d)` | `netip.AddrFrom4` + `textbuf.Addr` |
| `fmt.Sprintf("%02x:%02x:...", mac...)` | `textbuf.MAC(mac)` |
| `fmt.Sprintf("%x", data)` | `textbuf.Hex(data)` or `textbuf.AppendHex(buf, data)` |
| `fmt.Sprintf("%X", data)` | `textbuf.HexUpper(data)` |
| `fmt.Sprintf("%q", s)` | `var b textbuf.Buffer; b.Quoted(s).String()` |
| `fmt.Sprintf("%.1f", v)` | `var b textbuf.Buffer; b.Float(v, 1).String()` |
| `fmt.Sprintf("ctx: %v", err)` | `var b textbuf.Buffer; b.Str("ctx: ").Err(err).String()` |
| `fmt.Sprintf("%-20s", s)` | `var b textbuf.Buffer; b.PadRight(s, 20).String()` |
| `fmt.Sprintf("%6s", s)` | `var b textbuf.Buffer; b.PadLeft(s, 6).String()` |
| `fmt.Sprintf("127.0.0.1:%d", port)` | `textbuf.HostPort("127.0.0.1", port)` |
| `fmt.Errorf("constant string")` | `var ErrFoo = errors.New("constant string")` at package level |
| `fmt.Fprintf(w, "%s", s)` | `io.WriteString(w, s)` |
| `fmt.Fprintf(w, "%d", n)` | `io.WriteString(w, textbuf.Int(int64(n)))` |
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

When a struct needs both the typed value (for comparison) and the string (for
map keys or JSON), store both. Parse once at construction time:

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

## textbuf.Buffer (canonical string builder)

**Use `textbuf.Buffer` for all string building.** Package: `internal/core/textbuf`.

`Buffer` is a stack-allocated chainable builder. 128-byte backing array stays
on the stack; only the final `String()` allocates.

```go
import "codeberg.org/thomas-mangin/ze/internal/core/textbuf"

var b textbuf.Buffer
return b.Str("0:").Uint16(asn).Byte(':').Uint32(assigned).String()
```

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

| Result lifetime | Use | Allocations |
|----------------|-----|-------------|
| Stored in a struct field or returned | `String()` | 1 (inline copy) or 0 (heap transfer) |
| Consumed before `Reset()`/`Release()` | `Slice()` | 0 |
| Passed to `netip.ParsePrefix()` etc. | `Slice()` | 0 (parser copies internally if needed) |

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
| `textbuf.Uint(v)`, `Uint8(v)`, `Uint16(v)`, `Uint32(v)` | Decimal string |
| `textbuf.Int(v)` | Signed decimal string |
| `textbuf.Addr(a)` | IP address string |
| `textbuf.Prefix(p)` | CIDR prefix string |
| `textbuf.Hex(data)`, `HexUpper(data)` | Hex-encoded string |
| `textbuf.MAC(mac)` | MAC address string (e.g. "de:ad:be:ef:ca:fe") |
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
| `textbuf.AppendUint(dst, v)` | Decimal bytes |
| `textbuf.AppendInt(dst, v)` | Signed decimal bytes |
| `textbuf.AppendAddr(dst, a)` | IP address bytes |
| `textbuf.AppendPrefix(dst, p)` | CIDR prefix bytes |
| `textbuf.AppendHex(dst, data)` | Hex-encoded bytes |
| `textbuf.AppendMAC(dst, mac)` | MAC address bytes |

| Pattern | Use |
|---------|-----|
| Multi-part string, stored | `var b textbuf.Buffer; return b.Str(...).Uint32(...).String()` |
| Multi-part string, consumed immediately | `var b textbuf.Buffer; parse(b.Str(...).Uint32(...).Slice())` |
| Reuse in a loop | `var b textbuf.Buffer; for ... { use(b.Reset().Str(...).Slice()) }` |
| Single value | `return textbuf.Uint32(v)` or `return textbuf.Addr(a)` |
| Append into `[]byte` | `textbuf.AppendUint(dst, v)`, `textbuf.AppendAddr(dst, a)` |

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
func (b *keyBuilder) Uint(v uint64)        { var buf [20]byte; b.Write(textbuf.AppendUint(buf[:0], v)) }
func (b *keyBuilder) Addr(addr netip.Addr) { var buf [39]byte; b.Write(textbuf.AppendAddr(buf[:0], addr)) }
```

## Types Own Their Serialization

**BLOCKING.** Named types MUST have an `AppendTo([]byte) []byte` method.
Callers never format a type from the outside.

When a struct field is a plain `uint8`/`uint32` but represents a domain concept
(Origin, MED, ASN), it should use a named type with formatting methods. If
changing the field type is too large for the current task, use `textbuf.Buffer`
methods as a stepping stone, but track the typed refactor as follow-up.

### AppendTo Pattern (for types)

```go
func (t *MyType) AppendTo(buf []byte) []byte {
    buf = append(buf, "prefix "...)
    buf = textbuf.AppendUint(buf, uint64(t.Field))
    buf = append(buf, ':')
    buf = textbuf.AppendAddr(buf, t.Addr)
    return buf
}

func (t *MyType) String() string {
    var b textbuf.Buffer
    // Call AppendTo on Buffer's internal slice... or just chain:
    return textbuf.Addr(t.Addr)
}
```

## String `+` Concatenation is BANNED

**BLOCKING.** Never use `+` to build strings. Every `+` between non-constant
strings allocates a new backing array and copies both operands. This applies
everywhere, not just loops.

The only exception: compile-time constant expressions where both sides are
untyped string literals (`const x = "foo" + "bar"` -- the compiler folds these
at compile time, zero runtime cost).

```go
// BAD: 2 allocations (Uint result + concat)
return "peer:" + textbuf.Uint32(asn)

// GOOD: 1 allocation (String())
var b textbuf.Buffer
return b.Str("peer:").Uint32(asn).String()
```

In loops the cost compounds. Each iteration allocates, and collecting into
`[]string` + `strings.Join` adds yet another:

```go
// BAD: N*2 + 1 allocations
parts := make([]string, len(items))
for i, m := range items {
    parts[i] = ">" + textbuf.Uint(m.Value)
}
return strings.Join(parts, " ")

// GOOD: 1 allocation (String())
var b textbuf.Buffer
for _, m := range items {
    b.Byte(' ').Byte('>').Uint(m.Value)
}
return b.String()
```

The standalone `textbuf.Uint32(v)` functions are for **single-value returns**
(the entire result is one formatted value). For anything multi-part, use a Buffer.

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
