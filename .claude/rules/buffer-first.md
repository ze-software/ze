# Buffer-First Encoding

**BLOCKING:** All wire encoding MUST write into pooled, bounded buffers. Never allocate per-call.

## Why This Rule Exists

BGP messages have RFC-mandated maximum lengths: 4096 bytes (standard) or 65535 bytes (Extended Message, RFC 8654). The pool buffer IS the length limit — getting a 4096-byte buffer from `sync.Pool` simultaneously provides scratch space AND enforces the message size bound. Every `WriteTo(buf, off)` call writes into this bounded buffer. If it doesn't fit, the encoding fails — which is correct.

This is NOT an optimization. It is the design:

| Concern | How the pool buffer solves it |
|---------|-------------------------------|
| RFC length enforcement | Buffer size = max message length |
| Zero per-call allocation | Pool recycles buffers |
| Single allocation site | Pool `New` func, once |
| Natural capacity checking | `WriteTo` writes into bounded space |

## The Rule

When writing ANY code that produces wire bytes (BGP messages, attributes, NLRI, capabilities):

### 1. Get buffer from pool

Every encoding path starts by getting a pooled buffer. The buffer size matches the RFC maximum.

| Pool | Buffer Size | Purpose |
|------|-------------|---------|
| `readBufPool4K` | 4096 | Standard message reads |
| `readBufPool64K` | 65535 | Extended message reads |
| `buildBufPool` | 4096 | UPDATE building |
| Per-plugin `nlriBufPool` | 4096 | NLRI encoding (MP_REACH can carry multiple NLRIs) |

### 2. Write into it with WriteTo

| Action | Use | Never |
|--------|-----|-------|
| Encode a value | `WriteTo(buf, off) int` | `Pack() []byte` |
| Build helper | `writeFoo(buf, off, ...) int` | `buildFoo() ([]byte, error)` |
| Write attr header | `WriteHeaderTo(buf, off, flags, code, len)` | `PackHeader(flags, code, len)` |
| Encode fixed-size field | `binary.BigEndian.PutUintNN(buf[off:], v)` | `make([]byte, 4)` + put + return |
| Copy existing bytes | `copy(buf[off:], data)` | `make([]byte, len(data))` + copy + return |
| Grow a result | Write at offset, advance offset | `append()` |

### 3. Return buffer to pool

After the data is consumed (sent, hex-encoded, copied to owned slice), `pool.Put(buf)`.

## Banned Patterns in Encoding Code

| Pattern | Why banned | Use instead |
|---------|-----------|-------------|
| `append(buf, ...)` | Allocates, grows, copies | Pre-computed size + write at offset |
| `make([]byte, N)` in helpers | Per-call allocation | Write into caller's pool buffer |
| `buildFoo() ([]byte, error)` | Allocates and returns | `writeFoo(buf, off) int` |
| `.Bytes()` | Allocates a copy | `.WriteTo(buf, off)` with `.Len()` for size |
| `.Pack()` | Legacy allocating pattern | `.WriteTo(buf, off)` |

## Two-Phase Encoding Pattern

When encoding requires parsing before writing (e.g., need parsed values to compute size):

1. **Parse phase** — validate inputs, compute sizes, return parsed values + total size (NO allocation)
2. **Write phase** — `writeFoo(buf, off, parsedValues) int` writes into the pool buffer

Allocation happens at ONE level only: the pool. Sub-functions compute sizes and write into buffers — they never `make([]byte)`.

## How to Check

Before writing encoding code, ask:

1. **"Where does the buffer come from?"** → Must be a pool or caller-provided buffer
2. **"Am I using `append()`?"** → Use pre-computed size + offset writes instead
3. **"Am I returning `[]byte` from a helper?"** → Refactor to `writeFoo(buf, off) int`
4. **"Am I calling `make([]byte, ...)`?"** → Get from pool instead
5. **"Does the type already have `WriteTo`?"** → Use it, don't call `Pack()` or `Bytes()`

## Existing Infrastructure

These already exist — USE THEM:

| Tool | Location | Purpose |
|------|----------|---------|
| `wire.BufWriter` interface | `internal/plugins/bgp/wire/writer.go` | `WriteTo(buf, off) int` |
| `wire.CheckedBufWriter` | `internal/plugins/bgp/wire/writer.go` | `WriteTo` + capacity check |
| `wire.SessionBuffer` | `internal/plugins/bgp/wire/writer.go` | Reusable per-session buffer |
| `attribute.WriteHeaderTo()` | `internal/plugins/bgp/attribute/attribute.go` | Zero-alloc attr header |
| `attribute.WriteAttributeTo()` | `internal/plugins/bgp/attribute/attribute.go` | Header + value into buffer |
| `nlri.WriteTo()` | Various NLRI types | Zero-alloc NLRI encoding |

## When make([]byte) IS Acceptable

| Context | Why it's OK |
|---------|------------|
| Pool `New` func | Creates the pooled buffers themselves |
| Session buffer creation (`wire.SessionBuffer`) | Allocated once per session, reused |
| Cached encoding (`n.cached = make(...)`) | Allocated once, stored, reused on every WriteTo |
| Result copies returned to callers | Caller needs owned memory, not pooled |
| JSON marshaling | Not wire encoding, different concerns |
| Test files | Test data construction, not production path |
| IPC framing | Not BGP wire encoding |
| Config parsing | One-time loading, not hot path |

## Enforcement

- **Hook: `block-encoding-alloc.sh`** — Blocks `append()`, `make([]byte`, `.Bytes()`, `.Pack()` in encoding paths
- `/find-alloc` — Audit for violations
- `/fix-alloc file:line` — Convert a specific violation
