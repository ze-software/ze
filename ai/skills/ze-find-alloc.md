# Find Encoding Allocations

Scan encoding paths for `make([]byte, ...)` allocations that should use buffer-writing instead.

See also: `/ze-fix-alloc` (fix a specific allocation)

## Instructions

1. Use ULTRATHINK for thorough analysis
2. Accept an optional `$ARGUMENTS` parameter:
   - If provided: scope search to that path (e.g., `internal/component/bgp/reactor/`)
   - If empty: scan all production encoding paths under `internal/`
3. Exclude `_test.go` files -- tests legitimately allocate for test data
4. Exclude these legitimate uses:
   - `internal/component/bgp/attrpool/pool.go` -- pool infrastructure (manages buffers)
   - `internal/component/bgp/wire/writer.go` -- buffer infrastructure itself
   - `sync.Pool` factory functions -- session buffer pools
   - `internal/component/bgp/message/open.go` parsing paths -- decoding allocates for parsed data
   - JSON marshal paths (`MarshalJSON`) -- JSON encoding is separate
   - IPC framing (`ipc/`) -- not wire encoding
   - Config loading (`config/loader.go` line 1069) -- config processing
   - `tmpfs/` -- virtual filesystem, not wire encoding

## What to Find

Search for these patterns in production (non-test) Go files:

### Pattern 1: Functions returning allocated `[]byte`
Functions named `Pack*`, `Encode*`, `encode*` that return `[]byte`:
```
func.*Pack.*\[\]byte
func.*[Ee]ncode.*\[\]byte
```

### Pattern 2: `make([]byte, ...)` in encoding contexts
Any `make([]byte, N)` where the bytes are used to build wire data:
```
make\(\[\]byte
```

### Pattern 3: `[]byte{...}` literals for wire data
Inline byte slice literals that build wire fragments:
```
\[\]byte\{byte\(
```

### Pattern 4: `append()` building wire bytes from scratch
```
append\(\[\]byte\{
```

## Classification

For each finding, classify:

| Category | Description | Priority |
|----------|-------------|----------|
| **Hot path** | Reactor/peer UPDATE forwarding, RIB commit | Critical -- per-UPDATE allocation |
| **Attribute Pack()** | `Pack() []byte` methods on attribute types | High -- called by hot path |
| **NLRI encoding** | NLRI type encoding returning `[]byte` | High -- called by hot path |
| **Capability Pack()** | Capability encoding for OPEN messages | Low -- once per session |
| **API builder** | `update_build.go` text/hex command path | Medium -- API-only path |
| **Plugin types** | EVPN/FlowSpec/BGP-LS encoding | Medium -- plugin decode path |
| **Utility** | One-off helpers, config building | Low -- not hot path |

## Report Format

### Summary

| Category | Count | Files |
|----------|-------|-------|
| Hot path | N | `reactor.go`, `peer.go`, ... |
| ... | ... | ... |

### Findings by File

For each file with findings, report:

```
### path/to/file.go

| Line | Function | Allocation | Category | Target Pattern |
|------|----------|-----------|----------|----------------|
| 123 | funcName() | make([]byte, N) | Hot path | WriteTo(buf, off) |
```

Where "Target Pattern" is one of:
- `WriteTo(buf, off)` -- implement BufWriter interface
- `SessionBuffer.Write()` -- use wire.SessionBuffer
- `WriteHeaderTo()` -- use existing attribute.WriteHeaderTo
- `binary.BigEndian.PutUintNN(buf[off:])` -- direct write
- `copy(buf[off:], data)` -- direct copy into buffer
- `Keep` -- legitimate allocation (explain why)

### Existing WriteTo Coverage

Also report which types ALREADY have `WriteTo` alongside `Pack`:

```
### Types with WriteTo (migration-ready)

| Type | File | Has Pack | Has WriteTo | Has WriteToWithContext |
|------|------|----------|-------------|----------------------|
```

### Migration Priority

Rank files by impact (hot-path frequency x allocation count):

```
### Migration Priority

1. `reactor.go` -- N allocs, every forwarded UPDATE
2. `peer.go` -- N allocs, every route announcement
3. ...
```

## Do NOT

- DO NOT modify any files -- this is a read-only audit
- DO NOT report test file allocations (they are expected)
- DO NOT report pool/writer infrastructure allocations
- DO NOT count the same allocation twice (e.g., Pack() calling PackHeader())
