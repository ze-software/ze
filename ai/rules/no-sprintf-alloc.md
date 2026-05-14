# No Printf Allocations

**BLOCKING:** Never use `fmt.Sprintf`, `fmt.Fprintf`, or `fmt.Errorf` when a
zero-allocation or lower-allocation alternative exists.
Reference: `plan/analysis-printf-allocations.md`

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
   → netip.AddrFrom4([4]byte{...}).String()
   → or appendDottedDecimal(buf, a, b, c, d)

6. Is it hex formatting (%x, %02x, %X)?
   → hex.EncodeToString(data) for cold paths
   → hex.AppendEncode(buf, data) for hot paths
   → hex digit table for single bytes

7. Is it on a hot path (per-UPDATE, per-route, per-NLRI)?
   → AppendTo(buf []byte) []byte pattern (see text_append.go)
   → strconv.AppendUint, netip.Addr.AppendTo, hex.AppendEncode
   → Never fmt.Sprintf. No exceptions.

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
| `fmt.Sprintf("%d", n)` | `strconv.Itoa(n)` |
| `fmt.Sprintf("%s", s)` | `s` |
| `fmt.Sprintf("%s/%s", a, b)` | `a + "/" + b` |
| `fmt.Sprintf("%s:%d", s, n)` | `s + ":" + strconv.Itoa(n)` |
| `fmt.Sprintf("%d:%d", a, b)` | `strconv.Itoa(a) + ":" + strconv.Itoa(b)` |
| `fmt.Sprintf("%d.%d.%d.%d", a,b,c,d)` | `appendDottedDecimal` helper or `netip.AddrFrom4` |
| `fmt.Sprintf("%02x:%02x:...", mac...)` | hex digit table or `appendMAC` helper |
| `fmt.Sprintf("%x", data)` | `hex.EncodeToString(data)` |
| `fmt.Errorf("constant string")` | `var ErrFoo = errors.New("constant string")` at package level |
| `fmt.Fprintf(w, "%s", s)` | `io.WriteString(w, s)` |
| `fmt.Fprintf(w, "%d", n)` | `io.WriteString(w, strconv.Itoa(n))` |
| Sprintf in a function that discards the result | Split into no-alloc + with-string variants |

## Hot Path Rule

Code in these paths MUST NOT use any `fmt` function:

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

## AppendTo Pattern (canonical)

For any type that needs a string representation on a hot path:

```go
func (t *MyType) AppendTo(buf []byte) []byte {
    buf = append(buf, "prefix "...)
    buf = strconv.AppendUint(buf, uint64(t.Field), 10)
    buf = append(buf, ':')
    buf = t.Addr.AppendTo(buf)
    return buf
}

func (t *MyType) String() string {
    var scratch [64]byte
    return string(t.AppendTo(scratch[:0]))
}
```

Allowed primitives: `strconv.AppendUint`, `strconv.AppendInt`,
`netip.Addr.AppendTo`, `hex.AppendEncode`, `append(buf, "literal"...)`.

## Self-Check

Before submitting code with `fmt.Sprintf` / `fmt.Fprintf` / `fmt.Errorf`:

1. Is this on a hot path? If yes, use append-based alternative. No exceptions.
2. Does a simpler alternative exist per the decision tree above?
3. Could this error be a package-level sentinel?
4. Am I building a string that gets immediately discarded? Split the function.
