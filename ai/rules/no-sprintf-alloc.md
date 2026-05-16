# No Printf Allocations

**BLOCKING:** Never use `fmt.Sprintf`, `fmt.Fprintf`, or `fmt.Errorf` when a
zero-allocation or lower-allocation alternative exists. Never use `.String()`
concatenation on a hot path when an append-into-buffer pattern exists.
Reference: `plan/analysis-printf-allocations.md`

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
   → strconv.Itoa(n) or strconv.FormatUint(uint64(n), 10)

3. Is the only verb %s?
   → string concatenation: "prefix " + s + " suffix"

4. Are there 2-3 string/int args with fixed separators?
   → concatenation: a + ":" + b
   → or strconv.Itoa(a) + ":" + strconv.Itoa(b)

5. Is it formatting an IP address (%d.%d.%d.%d)?
   → netip.AddrFrom4([4]byte{...}).AppendTo(buf[:0])
   → Never .String() on hot paths (see stack buffer pattern below)

6. Is it hex formatting (%x, %02x, %X)?
   → hex.EncodeToString(data) for cold paths
   → hex.AppendEncode(buf, data) for hot paths
   → hex digit table for single bytes

7. Is it on a hot path (per-UPDATE, per-route, per-NLRI)?
   → AppendTo(buf []byte) []byte pattern (see text_append.go)
   → strconv.AppendUint, netip.Addr.AppendTo, hex.AppendEncode
   → Never fmt.Sprintf. No exceptions.
   → Never .String() + concatenation. Use stack buffer.

8. Is it building a string in a loop?
   → strings.Builder, or append into []byte + final string()

9. Is it writing to an io.Writer?
   → w.Write([]byte(...)) or io.WriteString(w, s)
   → strconv.AppendInt into a [20]byte scratch, then w.Write

10. None of the above?
    → fmt.Sprintf is acceptable (cold path, complex format)
```

## Banned Patterns

| Pattern | Replacement |
|---------|-------------|
| `fmt.Sprintf("%d", n)` | `textbuf.Int(int64(n))` (standalone) or `b.Int(int64(n))` (in chain) |
| `fmt.Sprintf("%s", s)` | `s` |
| `fmt.Sprintf("%s/%s", a, b)` | `a + "/" + b` (pure string concat, no numeric conversion) |
| `fmt.Sprintf("%s:%d", s, n)` | `var b textbuf.Buffer; b.Str(s).Byte(':').Int(int64(n)).String()` |
| `fmt.Sprintf("%d:%d", a, b)` | `var b textbuf.Buffer; b.Int(int64(a)).Byte(':').Int(int64(b)).String()` |
| `fmt.Sprintf("%d.%d.%d.%d", a,b,c,d)` | `netip.AddrFrom4` + `AppendTo` into stack buffer |
| `fmt.Sprintf("%02x:%02x:...", mac...)` | hex digit table or `appendMAC` helper |
| `fmt.Sprintf("%x", data)` | `hex.EncodeToString(data)` or `hex.AppendEncode` |
| `fmt.Errorf("constant string")` | `var ErrFoo = errors.New("constant string")` at package level |
| `fmt.Fprintf(w, "%s", s)` | `io.WriteString(w, s)` |
| `fmt.Fprintf(w, "%d", n)` | `io.WriteString(w, strconv.Itoa(n))` |
| Sprintf in a function that discards the result | Split into no-alloc + with-string variants |
| `addr.String() + "/" + strconv.Itoa(n)` | `var b textbuf.Buffer; b.Addr(addr).Byte('/').Int(int64(n)).String()` |
| Storing `string` then parsing back for comparison | Store `netip.Addr` (or typed value), compare directly |
| `net.ParseIP(s)` in a comparison function | Store `netip.Addr` at construction, use `.Compare()` |
| `">" + textbuf.Uint(v)` in a loop | `textbuf.Buffer` outside loop: `b.Byte('>').Uint(v)` |
| `parts[i] = prefix + strconv.Itoa(n)` in a loop | Single Buffer: `b.Str(prefix).Uint(uint64(n))` |
| `strings.Join(parts, " ")` after building parts from values | Build directly into a Buffer with `.Byte(' ')` separators |

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
| `Addr(a netip.Addr)` | Append IP address |
| `Hex(data []byte)` | Append lowercase hex |
| `String()` | Terminal: return built string (single alloc) |

Standalone functions for single-value returns: `textbuf.Uint32(v)`,
`textbuf.Addr(a)`, `textbuf.Hex(data)`, etc.

| Pattern | Use |
|---------|-----|
| Multi-part string | `var b textbuf.Buffer; return b.Str(...).Uint32(...).String()` |
| Single value | `return textbuf.Uint32(v)` or `return textbuf.Addr(a)` |
| Append into `[]byte` | `textbuf.AppendUint(dst, v)`, `textbuf.AppendAddr(dst, a)` |

### Banned

| Anti-pattern | Fix |
|-------------|-----|
| `fmt.Sprintf("%d:%d", a, b)` | `b.Uint32(a).Byte(':').Uint32(b).String()` |
| `strconv.Itoa(n) + ":" + strconv.Itoa(m)` | Same |
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

## Loop Anti-Pattern

**BLOCKING.** Never build strings with `+` concatenation inside a loop. Each
`">" + textbuf.Uint(v)` allocates twice (the Uint result and the concat),
and collecting into `[]string` + `strings.Join` adds a third. Use a single
`textbuf.Buffer` declared before the loop:

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
outside loops. Inside a loop, always chain into a Buffer.

## Self-Check

Before submitting code that builds strings:

1. Am I using `fmt.Sprintf`? Use `textbuf.Buffer` or standalone `textbuf.Uint32` etc.
2. Am I using `strconv.FormatUint` + `"` concatenation? Use `textbuf.Buffer` chain.
3. Am I writing `var buf [N]byte` + `append` + `string(b)`? Use `textbuf.Buffer`.
4. Am I calling `.String()` just to concatenate? Use `textbuf.Buffer.Addr()` etc.
5. Am I storing a string that will be parsed back for comparison? Store the typed value.
6. Am I building a string that gets immediately discarded? Split the function.
7. Am I building strings with `+` inside a loop? Use a single Buffer outside the loop.
7. Could this error be a package-level sentinel?
