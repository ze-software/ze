# textbuf: String Building

**Status:** Implemented
**Package:** `internal/core/textbuf`

Ze builds every string through `textbuf`. This page is the reference for the
package: what each type and function does, which form to reach for, and what
each choice costs in allocations. The obligation to use it is a rule
(`ai/rules/performance.md`); what it offers is here.

## How Buffer stays off the heap

`Buffer` is a chainable builder with a 128-byte inline backing array.
`Reset()` uses `noescape` (the same technique `strings.Builder` uses through
`abi.NoEscape`) to break the self-referential slice from escape analysis.
`var b Buffer` stays on the stack for local use, and the inline array avoids
any heap allocation for content of 128 bytes or less.

The trick mirrors `strings.Builder` in `$(go env GOROOT)/src/strings/builder.go`.
A Go compiler update can change the stdlib technique, so `ai/rules/performance.md`
carries the obligation to re-compare `noescape` and `inlineSlice` against the
stdlib `copyCheck` and `abi.NoEscape` on every bump.

## Allocation tiers

**Tier 0: zero allocations.** The buffer stays on the stack and the string is
consumed locally.

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

**Tier 1: one allocation.** The result is returned or stored as a string.

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

A pooled buffer extracted with `Slice()` and never released exhausts the pool
or dangles. `ai/rules/performance.md` bans it.

## Choosing an init

| Pattern | Init | Extract | Allocs |
|---------|------|---------|--------|
| Local use, write to io.Writer | `var b Buffer` | `Bytes()` | 0 |
| Local use, map lookup / comparison | `var b Buffer` | `string(b.Bytes())` | 0 |
| Hot loop, string consumed immediately | `Get()` + `defer Release()` | `Slice()` or `Bytes()` | 0 |
| Caller-owned buffer | `AppendTo` functions | none | 0 |
| Return a string (single use) | `var b Buffer` | `String()` | 1 |
| Return a string (zero-copy) | `New()` | `Slice()` | 1 |
| Return a string, reuse buffer | `Get()` + `defer Release()` | `String()` | 1 |

## Buffer methods

An appending method returns `*Buffer` so calls chain. An extractor (`String`,
`Slice`, `Bytes`, `Len`), the `io` methods, and `Release` return their own
result instead, so a chain ends on one of them.

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
| `HostPort(host, port string)` | Append "host:port" from a string port. An IPv6 host is bracketed: "[::1]:80" |
| `HostPortN(host string, port uint16)` | Append "host:port" from a numeric port. An IPv6 host is bracketed |
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
| `Release()` | Return a pooled buffer for reuse. A no-op on a stack buffer or after a prior `Release`. Safe after `String` |
| `SetColor(enabled)` | Enable or disable ANSI color output on this buffer |
| `Colored(c Color)` | Append the ANSI sequence for `c` when color is enabled, a no-op otherwise. End the span with `ColorReset` |
| `Write(p)` | Append raw bytes (implements `io.Writer`) |
| `WriteString(s)` | Append string (implements `io.StringWriter`). Returns `(int, error)` |
| `WriteByte(c)` | Append byte (implements `io.ByteWriter`). Returns `error` |
| `WriteRune(r)` | Append rune. Returns `(int, error)` |
| `Len()` | Current content length |

## String() against Slice()

`Slice()` does zero allocations at any size. `String()` copies inline data
(128 bytes or less) and does zero-copy for heap data. Most strings are passed
to a function and discarded, so `Slice()` is the default.

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

## Reusing a buffer with Reset()

Declare one `textbuf.Buffer` before a loop and call `Reset()` between
iterations. Each iteration reuses the same 128-byte inline array.

```go
var b textbuf.Buffer
for _, peer := range peers {
    key := b.Reset().Addr(peer.Addr).Byte(':').Uint16(peer.Port).Slice()
    lookupMap[key] = peer  // Slice valid until next Reset
}
```

For pooled buffers, `Get()`/`Release()` replaces the stack variable.

```go
b := textbuf.Get()
defer b.Release()
for _, p := range prefixes {
    formatted := b.Reset().Addr(p.Addr()).Byte('/').Int(int64(p.Bits())).Slice()
    process(formatted)
}
```

## Standalone functions

For a single value, these return a string directly.

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

## Append-into-buffer functions

For wire encoding and hot paths, these append into a caller-owned `[]byte`.

| Function | Appends |
|----------|---------|
| `textbuf.Uint(dst, v)` | Decimal bytes |
| `textbuf.Int(dst, v)` | Signed decimal bytes |
| `textbuf.Addr(dst, a)` | IP address bytes |
| `textbuf.Prefix(dst, p)` | CIDR prefix bytes |
| `textbuf.Hex(dst, data)` | Hex-encoded bytes |
| `textbuf.HexUpper(dst, data)` | Uppercase hex-encoded bytes |
| `textbuf.MAC(dst, mac)` | MAC address bytes |

## Which form for which pattern

| Pattern | Use |
|---------|-----|
| Multi-part string, stored | `var b textbuf.Buffer; return b.Str(...).Uint32(...).String()` |
| Multi-part string, consumed immediately | `var b textbuf.Buffer; parse(b.Str(...).Uint32(...).Slice())` |
| Reuse in a loop | `var b textbuf.Buffer; for ... { use(b.Reset().Str(...).Slice()) }` |
| Single value | `return textbuf.StringUint32(v)` or `return textbuf.StringAddr(a)` |
| Append into `[]byte` | `textbuf.Uint(dst, v)`, `textbuf.Addr(dst, a)` |

## Anti-patterns and their fixes

`ai/rules/performance.md` bans the left column. This table says what to write
instead.

| Anti-pattern | Fix |
|-------------|-----|
| `a + ":" + b` (any `+` concatenation) | `var b textbuf.Buffer; b.Str(a).Byte(':').Str(b).String()` |
| `fmt.Sprintf("%d:%d", a, b)` | `b.Uint32(a).Byte(':').Uint32(b).String()` |
| `strconv.Itoa(n) + ":" + strconv.Itoa(m)` | Same |
| `strings.Join(items, sep)` | `textbuf.Join(items, sep)` |
| `var buf [N]byte; b := append(buf[:0]...); return string(b)` | `var b textbuf.Buffer; return b.Str(...).String()` |
| `addr.String() + "/" + strconv.Itoa(n)` | `b.Addr(addr).Byte('/').Int(n).String()` |
| `uint64(v)` cast at call site | Use typed method: `Uint16(v)`, `Uint32(v)` |

## keyBuilder for grouping keys

For fingerprint and grouping key builders with repeated `|` separators, embed
`strings.Builder` in a package-local `keyBuilder` with typed methods.

```go
type keyBuilder struct{ strings.Builder }
func (b *keyBuilder) Sep()                 { b.WriteByte('|') }
func (b *keyBuilder) Uint(v uint64)        { var buf [20]byte; b.Write(textbuf.Uint(buf[:0], v)) }
func (b *keyBuilder) Addr(addr netip.Addr) { var buf [39]byte; b.Write(textbuf.Addr(buf[:0], addr)) }
```
