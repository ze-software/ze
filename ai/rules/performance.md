# Memory and Encoding

**When:** before writing buffer, pool, allocation, string-building, or wire-encoding code
**Severity:** blocking
**Related:** architecture, go-standards, repo-maintenance

## Directives

- **All wire encoding MUST write into pooled, bounded buffers.**
- **`fmt.Sprintf`, `fmt.Fprintf` and `fmt.Errorf` MUST NOT be used where a zero-allocation or lower-allocation alternative exists.**
- **`.String()` concatenation MUST NOT be used on a hot path where an append-into-buffer pattern exists.**
- **This file MUST be read before any allocation or memory-lifecycle decision.** It carries the obligations; the model behind them (data lifecycle, caller-owned buffers, pool strategy, the wire abstractions) is `docs/architecture/buffer-architecture.md`, and the string-building reference is `docs/architecture/textbuf-string-building.md`.
- Principle: `ai/rules/architecture.md` -- encapsulation onion plus buffer-first encoding.

## The Core Idea

1. **Buffer ownership** -- the caller MUST own the buffer, and the callee MUST write into it
2. **Pool lifecycle** -- bounded pools MUST replace unbounded `make()`
3. **Lazy parsing** -- raw byte slices with offset iterators MUST be used, not parsed structs

## Data Lifecycle (Wire to Wire)

- **A copy on the UPDATE path MUST match one of the four deliberate copy triggers in `docs/architecture/buffer-architecture.md` ("When a copy is deliberate"). A copy that matches none of them SHOULD be treated as wrong, and the reason it is needed MUST be asked before the code lands.**

## Pool Types

- **All buffers in a pool MUST be the same maximum size. Variable-sized allocation MUST NOT occur.**

## Key Wire Abstractions

- **The caller MUST own the buffer. The callee MUST write into `buf[off:]` and return the number of bytes written. Allocations MUST NOT occur.**

## Buffer-First Encoding

**These questions MUST be answered before writing encoding code:**

1. Buffer from? → Pool or caller-provided
2. `append()`? → Offset writes
3. Returning `[]byte` from helper? → `writeFoo(buf, off) int`
4. `make([]byte)`? → Get from pool
5. Type has `WriteTo`? → Use it

**Skip-and-backfill MUST follow these steps:**

1. Write fixed bytes (marker, type)
2. **Skip** length field -- save position (`lengthPos := off; off += 2`)
3. Write payload forward at advancing offset
4. **Backfill** length at saved position (`buf[lengthPos] = byte(totalLen >> 8)`)

**Encoding code MUST NOT call anything in the left column. The right column is what it MUST call instead.**

| Banned | Use Instead |
|--------|-------------|
| `append(buf, ...)` | Pre-computed size, then a write at the offset |
| `make([]byte, N)` in a helper | A write into the caller's pool buffer |
| `buildFoo() ([]byte, error)` | `writeFoo(buf, off) int` |
| `.Bytes()` | `.WriteTo(buf, off)` plus `.Len()` |
| `.Pack()` | `.WriteTo(buf, off)` |
| `x.Len()` then `x.WriteTo()` on a hot path | Skip-and-backfill, or `WriteAttrToWithLen()` |
| `BufHandle{Buf: make(...)}` | A pool-issued handle only; a hand-built one names no block and corrupts pool tracking |

- **`make([]byte, N)` MAY be used in a pool `New` func, session buffer creation, cached encoding, a result copy handed to a caller, JSON marshaling, tests, IPC framing, and config parsing. Anywhere else on the UPDATE path it MUST come from a pool.**

**Every file that emits OPEN, NOTIFICATION, ROUTE-REFRESH or NEGOTIATED text or JSON is encoding code, and MUST NOT call `fmt.Sprintf`, `fmt.Fprintf`, `strings.Builder`, `strings.Join`, `strings.NewReplacer`, `strings.ReplaceAll`, `strconv.FormatUint` or `strconv.FormatInt`.** `strconv.AppendUint`, `netip.Addr.AppendTo`, `hex.AppendEncode`, and a local `[N]byte` scratch with `append` MAY be used instead. `json.go` is excluded while its `map[string]any` plus `json.Marshal` idiom remains.

- **`writeGoPatterns` in `internal/le/hookruntime/writeedit.go` refuses allocation-heavy formatting and a hand-built buffer handle at edit time. The full encoding path SHOULD be audited with `/ze-find-alloc`, and a finding fixed with `/ze-fix-alloc file:line`.**

## Caller-Owned Resources (BLOCKING on hot paths)

**Allocation MUST happen once, at the outermost scope, and MUST pass inward.** The caller knows:
- How many times the callee will be called (loop count)
- What buffer size is needed (often a bounded maximum)
- When the buffer can be released (after all callees are done)

- **The callee MUST NOT guess at these. It MUST write into what it is given.**

- **The choice between a `sync.Pool` and a caller-passed buffer MUST be made from the table in `docs/architecture/buffer-architecture.md` ("Caller-owned buffers"), never from habit. A caller that already holds a buffer in scope MUST pass it down.**

**sync.Pool sizing:** The pool MUST be seeded with the common-case size. The pool holds
same-max-size buffers. If a caller needs more, `append()` will grow the
slice and the grown slice returns to the pool for future reuse.

**A buffer's lifecycle MUST be traced before writing code:**

1. **Where is the buffer allocated?** The function and the pool MUST be named.
2. **Who holds it?** The goroutine or struct owning the reference MUST be named.
3. **When is it copied?** Only at a deliberate copy trigger (`docs/architecture/buffer-architecture.md`).
4. **When is it released?** After the TCP write, after pool dedup, or after use.
5. **Could the caller provide this buffer?** If yes, the signature MUST change.
6. **Could a pool provide this buffer?** If yes, Get and Put MUST wrap the use.

## Common Mistakes

- **A per-UPDATE, per-route or per-NLRI function MUST NOT allocate: no `make([]byte, n)`, no `func Encode() []byte`, no `fmt.Sprintf`, no `.String()` in a loop, no `[]string` plus `strings.Join`. It MUST take a pool buffer or the caller's buffer, and MUST return bytes written.**
- **A `BufHandle` MUST NOT be hand-built. `BufHandle{Buf: make(...)}` names no block and corrupts pool tracking, so only a pool-issued handle is valid. `writeGoPatterns` in `internal/le/hookruntime/writeedit.go` refuses both at edit time.**
- **A `WireUpdate` MUST NOT be held past the return of the pool buffer its payload references. Anything still needed MUST be copied first.**
- The full mistake-and-fix table, with the reason each one costs, is `docs/architecture/buffer-architecture.md` ("Common allocation mistakes").

## Three Rules

1. **`fmt` MUST NOT be used on hot paths.** Append-based primitives MUST be used instead.
2. **`.String()` MUST NOT be used on hot paths.** `AppendTo` MUST be used into a stack buffer instead.
3. **Typed values MUST be stored, not strings.** `netip.Addr` MUST be compared directly, not string representations.

## Banned Patterns

**BLOCKING for new code and all hot paths** (hook-enforced at edit time by
`c_string_concat`). The `+` operator MUST NOT be used between strings: it
allocates a new backing array and copies both sides. `textbuf.Buffer` MUST be
used instead.

- **A compile-time constant expression whose sides are both untyped string literals MAY use `+`: `const x = "foo" + "bar"`. The compiler folds it, so nothing is allocated. No other `+` between strings is permitted.**

**Existing cold-path concatenation is cleanup-on-touch, not a sweep target: a legacy `+` site MUST be converted when the surrounding code is edited.** A `+` between strings MUST NOT survive on a hot path, where the Hot Path Rule below carries no legacy carve-out.

- **The standalone `textbuf.StringUint32(v)` functions MUST be used only for single-value returns (the entire result is one formatted value). For anything multi-part, a Buffer MUST be used.**

## Typed Comparison Rule

**BLOCKING on hot paths.** An IP address MUST NOT be stored as a string when it
will be compared. `netip.Addr` MUST be stored and `.Compare()` MUST be used
directly.

- **When a struct needs both the typed value (for comparison) and the string (for map keys or JSON), both MUST be stored. Parsing MUST happen once, at construction time.**

## Hot Path Rule

**Code on these paths MUST NOT call any `fmt` function, and MUST NOT concatenate the result of `.String()`. It MUST use the `AppendTo(buf []byte) []byte` pattern from `attribute/text_append.go`.**

| Path | Examples |
|------|----------|
| Wire encoding and decoding | `message/`, `wireu/`, `attribute/` |
| Per-UPDATE processing | `reactor/`, `adj_rib_in/`, `persist/`, `rr/`, `rs/` |
| Per-route evaluation | `rib/bestpath.go`, `rib/event.go`, `rib/route.go` |
| NLRI parsing and formatting | `nlri/*/`, `rib_nlri.go` |
| Filter chain | `reactor/filter_*.go` |

## Allowed fmt Usage

**`fmt` MAY be used in these contexts, and MUST NOT be used anywhere else on a hot path.** Each one runs once, or has no textbuf equivalent. The replacement tables are `docs/architecture/textbuf-string-building.md`.

| Context | Why |
|---------|-----|
| CLI output (`fmt.Println`, `fmt.Fprintf(os.Stdout, ...)`) | User-facing, runs once |
| Startup and shutdown messages | Runs once |
| Config parsing and validation errors | Runs once at load |
| Web page rendering on a cold path | Acceptable outside a per-route loop |
| Test assertions and sub-test naming | Not production code |
| `fmt.Errorf("context: %w", err)` with non-constant context | Error wrapping is the intended use |
| `fmt.Sprintf("%T", value)` | Reflect-based type name; textbuf has no equivalent |
| `fmt.Sprintf("%v", data)` where `data` is `any` | Arbitrary-type formatting; textbuf has no path |
| `http.Error(w, fmt.Sprintf(...))` | One-shot error response, not in a loop |

## Patterns That Must NOT Be Converted

**These three patterns MUST NOT be converted to `textbuf` during a sweep. Each one is correct as written.**

| Pattern | Why |
|---------|-----|
| `net.JoinHostPort(host, port)` where the port is already a string | Correct when the port came from `net.SplitHostPort` or from config as a string. New code SHOULD prefer `textbuf.HostPort` |
| `strings.Builder` as a long-lived `io.Writer` field | A field such as `pasteBuffer *strings.Builder` accumulates writes over time. `textbuf.Buffer` freezes on `Slice()` and its pool semantics differ |
| `strconv.Itoa(n)` passed to a sysctl or procfs map value | The kernel interface requires a `string`, and the allocation happens once per config reload, not per packet |

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

- **A struct field that is a plain `uint8` or `uint32` but represents a domain concept (Origin, MED, ASN) SHOULD be a named type carrying formatting methods. When changing the field type is too large for the task in hand, `textbuf.Buffer` methods MAY be used as a stepping stone, and the typed refactor MUST be tracked as follow-up.**

## Self-Check

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

**Mechanical rule for bulk conversions:** when the result leaves the current
scope (return, struct field, map insert, channel send), `String()` with
`var b Buffer` MUST be used, or `Slice()` with `New()` MUST be used.
Slice-from-stack MUST be consumed before the buffer goes out of scope
(function arg, map lookup, comparison).

## Related Documents

**The detail behind these directives lives in the pages below, and the relevant page SHOULD be read before allocation or string-building work.**

| Document | Covers |
|---|---|
| `docs/architecture/buffer-architecture.md` | The wire-to-wire buffer path, who owns each buffer, the pools Ze runs, the key wire abstractions, caller-owned buffers, common allocation mistakes |
| `docs/architecture/textbuf-string-building.md` | `textbuf.Buffer` methods, the standalone and append helpers, allocation tiers, and the `fmt`, `strings` and `+` replacement tables |
| `docs/architecture/pool-architecture.md` | Attribute dedup pools, handles, sharding, compaction |
| `docs/architecture/encoding-context.md` | `ContextID` and zero-copy forwarding |
| `docs/architecture/forward-congestion-pool.md` | The two-tier forward pool and per-peer workers |
| `docs/architecture/core-design.md` | System architecture and data flow |
| `ai/rules/architecture.md` | Encapsulation onion, lazy over eager, pool strategy |
| `/ze-find-alloc`, `/ze-fix-alloc` | Audit for a remaining allocation, and fix one |
