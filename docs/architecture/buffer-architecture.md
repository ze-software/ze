# Buffer-First Architecture

**Status:** Implemented (all development should follow this pattern)
**Last Updated:** 2026-01-30

> **See also:** `docs/architecture/core-design.md` for the canonical architecture reference
> covering WireUpdate, RIB storage model, factory pattern, and type consolidation.

## Implementation Progress

| Phase | Status | Description |
|-------|--------|-------------|
| Phase 1 | ✅ Done | Core iterator types (`NLRIIterator`, `AttrIterator`, `ASPathIterator`) |
| Phase 2 | ✅ Done | WireUpdate integration (iterator methods) |
| Phase 3 | ✅ Done | Direct formatting functions (iterator-based AS_PATH / communities / NLRI emission in `text.go`, `text_json.go`, `text_update.go`) |
| Phase 4 | ✅ Done | RIB migration (Route.AttrIterator, Route.ASPathIterator) |
| Phase 5 | ✅ Done | Deprecate parsed types (PathAttributes, RouteUpdate, UpdateInfo) |
| Phase 6 | ✅ Done | RouteJSON, Builder done; PathAttributes removed (retired summary 105; see `plan/learned/DESIGN-HISTORY.md`, "BGP engine: wire encoding and RIB" > Abandoned approaches) |

The buffer-first migration plan was summary 102, retired on 2026-08-01. See
`plan/learned/DESIGN-HISTORY.md`, "BGP engine: wire encoding and RIB": Evolution
for the `Pack()` to `WriteTo(buf, off)` arc, and Abandoned approaches for the
Span type that was introduced and removed during it.

---

## Executive Summary

Ze uses a **buffer-first** architecture where BGP messages are represented as byte buffers with iterators and partial parsers. This eliminates duplication between wire format and parsed representations, enables zero-copy operations, and provides a single source of truth.

**Core principle:** One representation (bytes). Everything else is views/iterators.

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                      Message Buffer                         │
│  ┌──────────┬───────────┬──────────────┬─────────────────┐  │
│  │  Header  │ Withdrawn │  Attributes  │      NLRI       │  │
│  │ 19 bytes │   (var)   │    (var)     │     (var)       │  │
│  └──────────┴───────────┴──────────────┴─────────────────┘  │
└─────────────────────────────────────────────────────────────┘
                              │
            ┌─────────────────┼─────────────────┐
            ▼                 ▼                 ▼
      ┌───────────┐    ┌───────────┐     ┌───────────┐
      │ AttrIter  │    │ NLRIIter  │     │ASPathIter │
      │ (offset)  │    │ (offset)  │     │ (offset)  │
      └───────────┘    └───────────┘     └───────────┘
            │                 │                 │
            ▼                 ▼                 ▼
      ┌───────────┐    ┌───────────┐     ┌───────────┐
      │  Accessor │    │  Accessor │     │  Accessor │
      │ (no alloc)│    │ (no alloc)│     │ (no alloc)│
      └───────────┘    └───────────┘     └───────────┘
```

---

## Design Principles

### 1. Bytes Are the Source of Truth

- Store wire bytes, not parsed structs
- Parse on demand via iterators
- Never duplicate data in different representations

### 2. Iterators Instead of Slices

```go
// ❌ OLD: Allocates slice
func (u *Update) NLRIs() []nlri.NLRI

// ✅ NEW: Iterator, zero allocation
func (u *Update) NLRIIterator() *NLRIIterator

type NLRIIterator struct {
    data    []byte
    offset  int
    family  Family
    addPath bool
}

func (it *NLRIIterator) Next() (prefix []byte, pathID uint32, ok bool)
```

### 3. Partial Parsers (Stateless Functions)

```go
// Parse only what you need, where you need it
func ParseNLRI(buf []byte, off int, addPath bool) (prefix []byte, pathID uint32, nextOff int, err error)
func ParseASPathSegment(buf []byte, off int) (segType uint8, asns []byte, nextOff int, err error)

// Iterators return []byte views directly - no intermediate Span type
```

### 4. Context-Aware Parsing

Parsing depends on negotiated capabilities (ADD-PATH, ASN4).
Context is passed as parameters, not as a struct:

```go
// ADD-PATH: NLRI includes 4-byte path-id prefix
// ASN4: AS numbers are 4 bytes (not 2)

// Iterators accept context as constructor parameter
func NewNLRIIterator(data []byte, addPath bool) *NLRIIterator
func NewASPathIterator(data []byte, asn4 bool) *ASPathIterator
```

### 5. Direct Formatting (No Intermediate Structs)

```go
// ❌ OLD: Parse to struct, then marshal
attrs := parseAttributes(buf)
json.Marshal(attrs)

// ✅ NEW: Format directly from buffer
func FormatAttributesJSON(buf []byte, asn4 bool, w io.Writer) error {
    iter := NewAttrIterator(buf)
    for typeCode, value, ok := iter.Next(); ok; typeCode, value, ok = iter.Next() {
        switch typeCode {
        case ORIGIN:
            fmt.Fprintf(w, `"origin":%d`, value[0])
        case AS_PATH:
            formatASPathJSON(value, asn4, w)
        // ...
        }
    }
}
```

---

## Component Design

### Message Buffer

The core type wrapping raw BGP message bytes:

```go
// WireUpdate wraps UPDATE message payload (after BGP header)
// Location: internal/component/bgp/wireu/wire_update.go
type WireUpdate struct {
    payload     []byte
    sourceCtxID ContextID      // For zero-copy decisions
    messageID   uint64         // Unique identifier
    sourceID    source.SourceID // Source that sent/created this message
}

// Existing section accessors (return raw bytes)
func (u *WireUpdate) Withdrawn() ([]byte, error)
func (u *WireUpdate) Attrs() (*AttributesWire, error)
func (u *WireUpdate) NLRI() ([]byte, error)

// Iterator accessors (Phase 2 - implemented)
func (u *WireUpdate) WithdrawnIterator(addPath bool) (*nlri.NLRIIterator, error)
func (u *WireUpdate) AttrIterator() (*attribute.AttrIterator, error)
func (u *WireUpdate) NLRIIterator(addPath bool) (*nlri.NLRIIterator, error)
```
<!-- source: internal/component/bgp/wireu/wire_update.go -- WireUpdate struct -->

### Attribute Iterator

```go
type AttrIterator struct {
    data   []byte
    offset int
}

func NewAttrIterator(data []byte) AttrIterator  // value return — stack-allocated

// Next returns the next attribute
// Returns (0, 0, nil, false) when exhausted
func (it *AttrIterator) Next() (typeCode uint8, flags uint8, value []byte, ok bool)

// Convenience: find specific attribute
func (it *AttrIterator) Find(typeCode uint8) ([]byte, bool)

// Zero-alloc standalone find — no pointer receiver, no heap escape
func AttrFind(data []byte, code AttributeCode) (hdrStart int, flags AttributeFlags, value []byte, found bool)
```
<!-- source: internal/core/bgp/attribute/ -- AttrIterator -->

### NLRI Iterator

```go
type NLRIIterator struct {
    data    []byte
    offset  int
    addPath bool
}

func NewNLRIIterator(data []byte, addPath bool) *NLRIIterator

// Next returns next NLRI
// prefix is a view into the buffer (not a copy)
// pathID is 0 if addPath is false
// Returns (nil, 0, false) when exhausted
func (it *NLRIIterator) Next() (prefix []byte, pathID uint32, ok bool)
```
<!-- source: internal/core/bgp/nlri/ -- NLRIIterator -->

### AS-PATH Iterator

```go
type ASPathIterator struct {
    data   []byte
    offset int
    asn4   bool
}

func NewASPathIterator(data []byte, asn4 bool) *ASPathIterator

// Next returns next segment
// asns is a view into buffer (raw bytes, 2 or 4 bytes per ASN)
// Returns (0, nil, false) when exhausted
func (it *ASPathIterator) Next() (segType uint8, asns []byte, ok bool)

// Convenience: iterate ASNs within current segment
func (it *ASPathIterator) ASNIterator(asns []byte) *ASNIterator
```
<!-- source: internal/core/bgp/attribute/ -- ASPathIterator -->

### Update Builder (For Creating Messages)

```go
type UpdateBuilder struct {
    buf       []byte
    attrsOff  int
    nlriOff   int
    ctx       BuildContext
}

type BuildContext struct {
    ASN4      bool
    AddPath   bool
    MaxSize   int  // 4096 or 65535
}

func NewUpdateBuilder(ctx BuildContext) *UpdateBuilder

// Attribute writers
func (b *UpdateBuilder) WriteOrigin(origin uint8) error
func (b *UpdateBuilder) WriteASPath(segments []ASPathSegment) error
func (b *UpdateBuilder) WriteNextHop(addr netip.Addr) error
func (b *UpdateBuilder) WriteMED(med uint32) error
func (b *UpdateBuilder) WriteLocalPref(pref uint32) error
func (b *UpdateBuilder) WriteCommunities(comms []uint32) error

// NLRI writers
func (b *UpdateBuilder) WriteNLRI(prefix netip.Prefix, pathID uint32) error
func (b *UpdateBuilder) WriteWithdrawn(prefix netip.Prefix, pathID uint32) error

// Finalize
func (b *UpdateBuilder) Build() ([]byte, error)
func (b *UpdateBuilder) Reset()
```

---

## RIB Storage

Routes store wire bytes as source of truth:

```go
// internal/component/bgp/rib/route.go
type Route struct {
    // Wire bytes (source of truth)
    wireBytes     []byte           // packed path attributes
    nlriWireBytes []byte           // packed NLRI
    sourceCtxID   ContextID        // for zero-copy compatibility check

    // Parsed attributes (cached)
    nlri       nlri.NLRI
    nextHop    netip.Addr
    attributes []attribute.Attribute
    asPath     *attribute.ASPath

    // Reference counting
    refCount   atomic.Int32
}

// Access via iterators - parse on demand
func (r *Route) AttrIterator() AttrIterator {
    return NewAttrIterator(r.wireBytes)
}

func (r *Route) ASPathIterator(asn4 bool) *ASPathIterator {
    // Find AS_PATH attribute
    iter := r.AttrIterator()
    for typeCode, _, value, ok := iter.Next(); ok; typeCode, _, value, ok = iter.Next() {
        if typeCode == AS_PATH_TYPE {
            return NewASPathIterator(value, asn4)
        }
    }
    return nil
}

// Zero-copy forwarding
func (r *Route) CanForwardDirect(destCtxID ContextID) bool {
    return r.sourceCtxID == destCtxID
}

func (r *Route) WireBytes() []byte {
    return r.wireBytes
}
```
<!-- source: internal/component/bgp/rib/route.go -- Route struct -->

---

## API Layer

### JSON Formatting (Direct from Buffer)

```go
// Format UPDATE event directly to JSON writer
func FormatUpdateEventJSON(u *WireUpdate, addPath bool, w io.Writer) error {
    w.Write([]byte(`{"type":"update"`))

    // Announce section
    w.Write([]byte(`,"announce":{`))
    nlriIter, _ := u.NLRIIterator(addPath)
    first := true
    for prefix, pathID, ok := nlriIter.Next(); ok; prefix, pathID, ok = nlriIter.Next() {
        if !first {
            w.Write([]byte(`,`))
        }
        formatPrefixJSON(prefix, pathID, w)
        first = false
    }
    w.Write([]byte(`}`))

    // Attributes
    w.Write([]byte(`,"attr":{`))
    iter, _ := u.AttrIterator()
    formatAttributesJSON(iter, w)
    w.Write([]byte(`}}`))

    return nil
}
```

### Text Formatting

```go
func FormatUpdateText(u *WireUpdate, addPath bool, w io.Writer) error {
    // "update text as-path set [65001 65002] nhop set 192.168.1.1 nlri ipv4/unicast add 10.0.0.0/24"
    // Format directly from buffer bytes
}
```

---

## Migration Path

### Phase 1: Add Iterators (Non-Breaking)

Add iterator types alongside existing slice-returning methods:

```go
// Keep existing (deprecated)
func (u *Update) NLRIs() []nlri.NLRI

// Add new
func (u *Update) NLRIIterator() *NLRIIterator
```

### Phase 2: Migrate Internal Code

Update internal consumers to use iterators:
- RIB storage
- Route forwarding
- UPDATE building

### Phase 3: Migrate API Layer

Update API formatting to use direct buffer access:
- JSON encoder
- Text encoder

### Phase 4: Remove Parsed Types

Once all consumers migrated:
- Remove `PathAttributes` struct
- Remove `RouteUpdate` struct
- Remove slice-returning methods

---

## What Gets Removed

| Current Type | Replacement |
|--------------|-------------|
| `plugin.PathAttributes` | `AttrIterator` over buffer |
| `plugin.RouteUpdate` | Direct formatting from buffer |
| `[]attribute.Attribute` | `AttrIterator` |
| `[]nlri.NLRI` | `NLRIIterator` |
| `[]uint32` (AS-PATH) | `ASPathIterator` |
| `rr.UpdateInfo` | `WireUpdate` + iterators |
| `plugin.RawMessage` | Simplified to buffer ref |

---

## Benefits

| Benefit | Description |
|---------|-------------|
| **Zero-copy passthrough** | Route reflection = memcpy of buffer |
| **Single source of truth** | No sync between wire/parsed representations |
| **Parse on demand** | Only parse attributes API actually needs |
| **Memory efficient** | No slice allocations for AS-PATH, communities |
| **Consistent** | API and wire code use identical primitives |
| **Simpler code** | One way to do things, not three |

---

## Guidelines for New Code

1. **Never store parsed slices** - Store wire bytes, iterate on demand
2. **Never return slices from iterators** - Return views (subslices) or format directly
3. **Use builders for construction** - `UpdateBuilder` for new messages
4. **Pass context as params** - Context-dependent parsing (addPath bool, asn4 bool)
5. **Format directly to Writer** - No intermediate JSON structs

---

## Why allocations on the UPDATE path cost

Ze processes millions of BGP UPDATEs per second. Each UPDATE touches wire
parsing, attribute extraction, pool dedup, RIB storage, route selection, filter
evaluation, UPDATE building, and the TCP write. Every allocation on that path
adds GC pressure and latency, so the path is built from three interlocking
strategies: the caller owns the buffer, bounded pools replace `make()`, and the
read side stays lazy (raw byte slices plus offset iterators, never parsed
structs). `ai/rules/performance.md` carries the obligations.

## The wire-to-wire buffer path

```
TCP recv → bufMuxStd / bufMuxExt block-backed buffer
    → WireUpdate (lazy, references the pool buffer)
    → attribute extraction (lazy iterators, no copy)
    → pool dedup (per-attribute-type, refcounted)
    → RIB entry (NLRI → attribute handle refs)
    → route selection (operates on handles)
    → outbound building (WriteTo into the destination peerPool buffer)
    → TCP send → return the peerPool buffer
```

### Who owns the buffer at each stage

| Stage | Buffer owner | What happens |
|---|---|---|
| TCP read | `bufMuxStd` (4K) or `bufMuxExt` (64K) | Wire bytes land in a block-backed pool buffer |
| Parsing | Same buffer | `WireUpdate` holds a slice into the pool buffer, no copy |
| Attribute extract | Same buffer | Lazy iterators return sub-slices, no copy |
| Pool dedup | Attribute pool | First time: copy into the pool slab. Afterwards: refcount++ |
| Forwarding, same context | Same pool buffer | Zero-copy: matching `ContextID` means the wire bytes are reusable |
| Forwarding, different context | Destination `peerPool` | Copy-on-modify: the modified UPDATE is built into the outgoing buffer |
| Overflow | `MixedBufMux` | Byte-budgeted, mixed 4K and 64K blocks |

<!-- source: internal/component/bgp/reactor/session.go -- bufMuxStd, bufMuxExt, getReadBuffer -->
<!-- source: internal/component/bgp/reactor/forward_pool.go -- peerPool, peerPoolSize -->
<!-- source: internal/component/bgp/reactor/bufmux.go -- BufMux, MixedBufMux, BufHandle -->

### When a copy is deliberate

A copy outside these four triggers is a defect until its reason is stated.

| Copy trigger | Why |
|---|---|
| An attribute enters the pool for the first time | The pool owns the canonical copy; the wire buffer will be reused |
| `ContextID` mismatch on forward | Wire bytes encoded for other capabilities need re-encoding |
| A filter modifies attributes | The modified attributes are written into the outgoing buffer |
| JSON serialization for an external plugin | An external plugin needs formatted text, not wire bytes |

## The pools Ze runs

| Pool | Location | Shape | Purpose |
|---|---|---|---|
| `bufMuxStd` | `internal/component/bgp/reactor/session.go` | Block-backed multiplexer, `message.MaxMsgLen` (4096) buffers | Session reads before Extended Message negotiation, and UPDATE attribute building |
| `bufMuxExt` | `internal/component/bgp/reactor/session.go` | Block-backed multiplexer, `message.ExtMsgLen` (65535) buffers | Session reads after RFC 8654 Extended Message negotiation |
| `peerPool` | `internal/component/bgp/reactor/forward_pool.go` | Ring of `peerPoolSize` (64) slots over one contiguous backing array | Per-peer outgoing buffers for copy-on-modify forwarding |
| `MixedBufMux` | `internal/component/bgp/reactor/bufmux.go` | Byte-budgeted, mixed 4K and 64K blocks | Forward overflow when a peer pool is exhausted |
| `modBufPool` | `internal/component/bgp/reactor/forward_build.go` | `sync.Pool` of 4096-byte buffers | Progressive-build scratch when no peer-pool slot is free |
| Per-attribute-type pools | `internal/component/bgp/plugins/rib/pool/attributes.go` | `attrpool.Pool` dedup slabs, one per attribute type | RIB attribute deduplication |
| `textbuf` pool | `internal/core/textbuf/textbuf.go` | `sync.Pool` reached through `Get()` and `Release()` | String building |

The `bufMuxStd` and `bufMuxExt` names are the two read multiplexers
`Session.getReadBuffer` and `getReadBuf` select between; there is no
`readBufPool4K`, `readBufPool64K`, or `buildBufPool` in the tree.

### Pool shape follows goroutine shape

| Goroutine pattern | Pool strategy | Example |
|---|---|---|
| Single goroutine, sequential processing | Ring over a contiguous backing array, index stack | `peerPool`: one reactor loop builds one modified payload at a time per destination |
| Multiple goroutines, concurrent access | Block-backed multiplexer or `sync.Pool` | `bufMuxStd`: many peers read at once |

Every buffer in one pool is the same maximum size. Variable-sized allocation
defeats the block accounting that lets a block be released whole.

## Key wire abstractions

| Type | Location | Purpose | Lifecycle |
|---|---|---|---|
| `WireUpdate` | `internal/component/bgp/wireu/wire_update.go` | Lazy-parsed UPDATE message, with `sync.Once`-guarded section, attribute and shape caches | Lives as long as the pool buffer its payload references |
| `EncodingContext` | `internal/core/bgp/context/context.go` | Negotiated capabilities for one peer and one direction; derives the per-family ADD-PATH and paths-limit maps | Created once per peer per direction at session establishment |
| `ContextID` | `internal/core/bgp/context/registry.go` | `uint16` naming a distinct encoding context | Same ID means same encoding, so wire bytes forward unchanged |
| `BufHandle` | `internal/component/bgp/reactor/bufmux.go` | `{ID, idx, Buf}`: the block, the slot inside it, and the buffer slice | Zero `Buf` means the pool was exhausted; the caller returns it exactly once |
| `BufWriter` | `internal/core/bgp/wire/writer.go` | `WriteTo(buf []byte, off int) int` | Implemented by every wire-encodable type |
| `CheckedBufWriter` | `internal/core/bgp/wire/writer.go` | `BufWriter` plus `CheckedWriteTo(buf, off) (int, error)` and `Len() int` | Implemented where capacity is validated or a length is needed before the write |

`Len()` is on `CheckedBufWriter`, not on `BufWriter`.

Context-dependent encoding adds a second method, carried by the `Attribute`
interface in `internal/core/bgp/attribute/attribute.go`:

```go
WriteToWithContext(buf []byte, off int, srcCtx, dstCtx *bgpctx.EncodingContext) int
```

## Encoding patterns

**Get, write, put.** Take a buffer from the pool, write with
`WriteTo(buf, off) int`, return it to the pool.

**Skip and backfill.** For a message with variable-length sections and a
fixed-position length field: write the fixed bytes, skip the length field and
save its offset (`lengthPos := off; off += 2`), write the payload forward at the
advancing offset, then backfill the length at the saved position. This avoids
the `Len()`-then-`WriteTo()` double traversal. `reactor_wire.go` holds the
canonical implementation.

### Where the buffer comes from

```
Is this on a per-UPDATE / per-route / per-NLRI path?
├── YES: Does the caller already have a buffer in scope?
│   ├── YES: Add buf/off parameters, write into it (WriteTo pattern)
│   └── NO: Is there a pool for this goroutine shape?
│       ├── YES: Get from pool, use, put back
│       └── NO: Can the buffer be a struct field reused across calls?
│           ├── YES: Store it on the struct, reset between uses
│           └── NO: Create a sync.Pool for this use case
└── NO (startup, config, CLI, one-shot):
    ├── One-shot allocation → make() is fine
    ├── String building → textbuf.Buffer (stack, 128B inline)
    └── fmt.Sprintf → acceptable on cold paths
```

## Caller-owned buffers

The most common allocation mistake is a callee allocating a buffer the caller
could have passed down. The caller knows the loop count, the bounded maximum
size, and when the buffer can be released; the callee knows none of the three.

```go
// BAD: allocates N times inside the loop
for _, attr := range attributes {
    packed := attr.Pack()         // make([]byte, attr.Len()) inside
    copy(buf[off:], packed)
    off += len(packed)
}

// GOOD: zero allocations
for _, attr := range attributes {
    off += attr.WriteTo(buf, off) // writes directly into the caller's buffer
}
```

```go
// BAD: buildPayload allocates its own scratch buffer
func buildPayload(attrs []Attribute) []byte {
    buf := make([]byte, totalLen(attrs))   // ALLOCATION
    off := 0
    for _, a := range attrs {
        off += a.WriteTo(buf, off)
    }
    return buf
}

// GOOD: the caller passes its buffer
func writePayload(buf []byte, off int, attrs []Attribute) int {
    start := off
    for _, a := range attrs {
        off += a.WriteTo(buf, off)
    }
    return off - start
}
```

When the caller cannot supply a buffer, because the function has many call
sites or the scratch size varies, use a `sync.Pool` seeded with the
common-case size. A caller needing more grows the slice with `append`, and the
grown slice returns to the pool for later reuse.

```go
var scratchPool = sync.Pool{
    New: func() any { return make([]byte, 0, 4096) },
}

func process(data []byte) Result {
    scratch := scratchPool.Get().([]byte)[:0]
    defer scratchPool.Put(scratch)

    scratch = append(scratch, data...)
    return buildResult(scratch)
}
```

| Situation | Use |
|---|---|
| The caller has a buffer in scope | Pass it as a parameter |
| The function has one or two call sites | Add a buf parameter to those callers |
| The function has many call sites and the scratch is internal | `sync.Pool` |
| The buffer is needed across goroutines | `sync.Pool` (goroutine-safe) |
| Single goroutine, sequential processing | Ring buffer or struct field |

## Common allocation mistakes

| Mistake | Why it is wrong | Fix |
|---|---|---|
| `make([]byte, n)` in a per-UPDATE function | Allocates on every UPDATE | Get from a pool, or write into the caller's buffer |
| `func Encode() []byte` returning allocated bytes | The caller must copy into its own buffer | `WriteTo(buf, off) int` |
| `fmt.Sprintf` in reactor, wire, or attribute code | Two or more allocations per call | `textbuf.Buffer` or a `textbuf.String*` helper |
| `addr.String()` in a loop | Allocates per iteration | `textbuf.Addr(buf[:0], addr)` into a stack buffer |
| Holding a `WireUpdate` past the return of its pool buffer | The payload is a slice into that buffer | Copy what is needed before returning the buffer |
| Building `[]string` then `strings.Join` in a loop | N+1 allocations | One `textbuf.Buffer` outside the loop |
| `string(bytes)` for a comparison in a filter | Allocates the string | Compare bytes, or compare a typed value |
| `map[string]V` keyed by a value from a known set | String keys hash over bytes and the GC scans their pointers | `map[uint16]V` or `map[TypedEnum]V`, parsing at the boundary |
| `BufHandle{Buf: make(...)}` | Corrupts pool tracking: the `ID` names no block | Only use pool-issued handles |

## Related Documentation

- `docs/architecture/encoding-context.md` - Context-dependent encoding
- `docs/architecture/update-building.md` - Wire format construction
- `docs/architecture/rib-transition.md` - RIB ownership model
- `plan/spec-buffer-first-migration.md` - Implementation spec

---

**Last Updated:** 2026-01-30
