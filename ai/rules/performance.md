# Memory and Encoding

**When:** before writing buffer, pool, allocation, string-building, or wire-encoding code, or converting a value between representations on a hot path
**Severity:** blocking
**Related:** architecture, go-standards, repo-maintenance

## Directives

**All wire encoding MUST write into a pooled, bounded buffer the caller owns: the callee writes into `buf[off:]`, returns the bytes written, and allocates nothing.** Encoding code MUST NOT call `append(buf, ...)`, `make([]byte, N)` in a helper, `buildFoo() ([]byte, error)`, `.Bytes()`, `.Pack()`, `x.Len()` before `x.WriteTo()` on a hot path, or a hand-built `BufHandle{Buf: make(...)}`, and MUST use an offset write, a pool-issued handle, `writeFoo(buf, off) int`, `.WriteTo(buf, off)` and skip-and-backfill for a length field instead. `make([]byte, N)` stays correct in a pool `New` func, session buffer creation, cached encoding, a result copy handed to a caller, JSON marshaling, tests, IPC framing and config parsing. Every buffer in one pool is the same maximum size; the model behind all of it is `docs/architecture/buffer-architecture.md`, and `/ze-find-alloc` and `/ze-fix-alloc` audit and repair a path.

**On a hot path a value MUST change form once, from its source form to its destination form, and MUST NOT pass through a third form that it then leaves.** A typed value converted to bytes and back to a typed value (`netip.AddrFrom16(a.As16())`), and wire bytes parsed into a struct only to be encoded back to wire bytes, are the shapes. Inlining does not remove the cost: Vincent Bernat measured that round trip at eight times a direct method, and the compiler passes that could cancel it change order between Go releases. When the type has no direct operation, the operation MUST be added to the type that owns the representation; for a standard library type the round trip MAY stay off the hot path. A claim that the compiler removes the cost MUST cite `-gcflags=-m` or `-gcflags=-S` output. `/ze-find-alloc` hunts the pattern.

## Common Mistakes

**A per-UPDATE, per-route or per-NLRI function MUST NOT allocate: no `make([]byte, n)`, no `func Encode() []byte`, no `fmt.Sprintf`, no `.String()` in a loop, no `[]string` plus `strings.Join`.** It MUST take a pool buffer or the caller's buffer and return the bytes written, and a `WireUpdate` MUST NOT be held past the return of the pool buffer its payload references. The mistake-and-fix table, with the cost of each, is `docs/architecture/buffer-architecture.md`.

## Three Rules

**On a hot path `fmt` MUST NOT be called, `.String()` MUST NOT be concatenated, `+` MUST NOT join two strings, and a value that will be compared MUST be stored typed (`netip.Addr` with `.Compare()`), never as a string.** Build text with `textbuf.Buffer` or an `AppendTo(buf []byte) []byte` method instead, and release a pooled buffer whose `Slice()` you took. `fmt` MAY still be used where it runs once and has no textbuf equivalent: CLI output, startup and shutdown messages, config load errors, a cold web render, tests, `fmt.Errorf("...: %w", err)`, `%T`, `%v` over an `any`, and a one-shot `http.Error`; a compile-time `const x = "foo" + "bar"` MAY use `+` because the compiler folds it, and a cold-path `+` is converted on touch rather than swept. The replacement tables are `docs/architecture/textbuf-string-building.md`.

## Hot Path Rule

**These paths are the hot paths the ban above governs, and code on them MUST use the `AppendTo(buf []byte) []byte` pattern from `attribute/text_append.go`:** wire encoding and decoding (`message/`, `wireu/`, `attribute/`), per-UPDATE processing (`reactor/`, `adj_rib_in/`, `persist/`, `rr/`, `rs/`), per-route evaluation (`rib/bestpath.go`, `rib/event.go`, `rib/route.go`), NLRI parsing and formatting (`nlri/*/`, `rib_nlri.go`), and the filter chain (`reactor/filter_*.go`).
