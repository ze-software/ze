# Buffer-First Encoding

**BLOCKING:** All wire encoding MUST write into caller-provided buffers. Never `make([]byte, ...)` and return.

## Why This Rule Exists

`Pack() []byte` is the legacy pattern. It allocates on every call. On a busy router processing thousands of UPDATEs/second, each `Pack()` creates GC pressure. The codebase is migrating to `WriteTo(buf, off) int` — write directly into a pre-allocated buffer, zero allocations.

This is the same evolution as `fork()+shared memory` → goroutines, or `sprintf()` returning a string → `fmt.Fprintf(w, ...)` writing to an `io.Writer`. The old way works. The new way scales.

## The Rule

When writing ANY code that produces wire bytes (BGP messages, attributes, NLRI, capabilities):

| Action | Use | Never |
|--------|-----|-------|
| Encode a value | `WriteTo(buf, off) int` | `Pack() []byte` |
| Write attr header | `WriteHeaderTo(buf, off, flags, code, len)` | `PackHeader(flags, code, len)` |
| Build UPDATE section | Write into `wire.SessionBuffer` or `[]byte` | `make([]byte, N)` then return |
| Encode fixed-size field | `binary.BigEndian.PutUintNN(buf[off:], v)` | `make([]byte, 4)` + put + return |
| Copy existing bytes | `copy(buf[off:], data)` | `make([]byte, len(data))` + copy + return |

## Build/Helper Functions

**BLOCKING:** Build and helper functions MUST NOT allocate and return `[]byte`. They MUST write into a caller-provided buffer.

| Pattern | Status |
|---------|--------|
| `buildFoo() ([]byte, error)` with internal `make` | **Forbidden** |
| `writeFoo(buf, off) int` writing into caller's buffer | **Required** |
| `append()` in encoding code | **Forbidden** |

### Required Two-Phase Pattern

When encoding requires parsing before writing (e.g., need parsed values to compute size):

1. **Parse phase** — validate inputs, compute sizes, return parsed values + total size (NO allocation)
2. **Allocate once** — at the top-level caller, using the computed size
3. **Write phase** — `writeFoo(buf, off, parsedValues) int` writes into the pre-allocated buffer

This means: allocation happens at ONE level only (the top-level builder), never inside helper functions. Sub-functions compute sizes and write into buffers — they never `make([]byte)`.

## How to Check

Before writing encoding code, ask:

1. **"Am I returning `[]byte`?"** → Refactor to `writeFoo(buf, off) int`
2. **"Am I calling `make([]byte, ...)`?"** → Move allocation to the caller
3. **"Does the type already have `WriteTo`?"** → Use it, don't call `Pack()` or `Bytes()`
4. **"Am I adding a new type?"** → Implement `wire.BufWriter` from the start, `Pack()` is optional
5. **"Am I using `append()`?"** → Use pre-computed size + offset writes instead

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

## Reference Pattern

**MED** (`internal/plugins/bgp/attribute/simple.go`) shows both patterns side by side:

```
Pack() []byte        — allocates 4 bytes, writes, returns     ← LEGACY
WriteTo(buf, off)    — writes directly into buf at offset      ← CORRECT
```

## Pool-First Allocation

**BLOCKING:** Encoding functions MUST use `sync.Pool` for scratch buffers, not `make([]byte, ...)`.

| Pattern | Status |
|---------|--------|
| `buf := pool.Get().([]byte)` → write → `pool.Put(buf)` | **Required** |
| `buf := make([]byte, N)` in encoding hot path | **Forbidden** |
| `make([]byte, N)` for result copies returned to callers | Acceptable (caller needs owned memory) |

### Pool Pattern

```
scratch := pool.Get().([]byte)    // Get reusable buffer
n := writeFoo(scratch, 0, ...)    // Write into pool buffer
result := hex.EncodeToString(scratch[:n])  // Use the data
pool.Put(scratch)                  // Return to pool
```

When the caller needs owned bytes (return value), copy the relevant portion:
```
scratch := pool.Get().([]byte)
n := writeFoo(scratch, 0, ...)
owned := make([]byte, n)           // Only allocation: result copy
copy(owned, scratch[:n])
pool.Put(scratch)
return owned
```

### Existing Pools

| Pool | Location | Buffer Size | Purpose |
|------|----------|-------------|---------|
| `readBufPool4K` | `reactor/session.go` | 4096 | Standard message reads |
| `readBufPool64K` | `reactor/session.go` | 65535 | Extended message reads |
| `buildBufPool` | `reactor/session.go` | 4096 | UPDATE building |
| `nlriBufPool` | Plugin encode packages | 128-256 | NLRI encoding scratch |

## When make([]byte) IS Acceptable

| Context | Why it's OK |
|---------|------------|
| Pool infrastructure (pool `New` func) | Creates the pooled buffers themselves |
| Session buffer creation (`wire.SessionBuffer`) | Allocated once per session, reused |
| Cached encoding (`n.cached = make(...)`) | Allocated once, stored, reused on every WriteTo |
| Result copies returned to callers | Caller needs owned memory, not pooled |
| JSON marshaling | Not wire encoding, different concerns |
| Test files | Test data construction, not production path |
| IPC framing | Not BGP wire encoding |
| Config parsing | One-time loading, not hot path |

## When Reviewing Code

If you see encoding code that uses `make([]byte, ...)`:
1. Flag it
2. Suggest the `WriteTo` alternative
3. Reference this rule

## Enforcement

- `/find-alloc` — Audit for violations
- `/fix-alloc file:line` — Convert a specific violation
- Post-edit hooks may flag new `Pack()` calls in encoding paths
