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

## How to Check

Before writing encoding code, ask:

1. **"Am I returning `[]byte`?"** → Refactor to `WriteTo(buf, off) int`
2. **"Am I calling `make([]byte, ...)`?"** → Write into the caller's buffer instead
3. **"Does the type already have `WriteTo`?"** → Use it, don't call `Pack()`
4. **"Am I adding a new type?"** → Implement `wire.BufWriter` from the start, `Pack()` is optional

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

## When make([]byte) IS Acceptable

| Context | Why it's OK |
|---------|------------|
| Pool infrastructure (`pool/pool.go`) | Manages the buffers themselves |
| Session buffer creation (`wire.SessionBuffer`) | Allocated once per session, reused |
| Cached encoding (`n.cached = make(...)`) | Allocated once, stored, reused on every WriteTo |
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
