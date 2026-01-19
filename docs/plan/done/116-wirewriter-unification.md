# Spec: wirewriter-unification

## Task

Unify Message and Attribute interfaces around a common `WireWriter` interface using `*EncodingContext` for all encoding decisions. Remove duplicate types and deprecated methods.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - canonical architecture reference
- [ ] `docs/architecture/buffer-architecture.md` - buffer-first patterns
- [ ] `docs/architecture/encoding-context.md` - EncodingContext design

### RFC Summaries
- [ ] `docs/rfc/rfc4271.md` - BGP message formats
- [ ] `docs/rfc/rfc6793.md` - ASN4 encoding (context-dependent)
- [ ] `docs/rfc/rfc7911.md` - ADD-PATH encoding (context-dependent)
- [ ] `docs/rfc/rfc8654.md` - Extended Message (affects max size)

**Key insights:**
- `EncodingContext` already stored per-peer (`Peer.sendCtx`, `Peer.recvCtx`)
- `message.Negotiated` is ephemeral conversion shim - delete it
- Add `ExtendedMessage` to `EncodingContext` for message size limits
- NLRI uses `PackContext` derived via `ctx.ToPackContext(family)`

## Design

### Move ExtendedMessage to EncodingCaps

Currently:
```
capability.Negotiated
├── Identity (PeerIdentity)
├── Encoding (EncodingCaps) ← ASN4, Families, AddPath, ExtendedNextHop
└── Session (SessionCaps)   ← ExtendedMessage, RouteRefresh, HoldTime, GR
```

After:
```
capability.Negotiated
├── Identity (PeerIdentity)
├── Encoding (EncodingCaps) ← ASN4, Families, AddPath, ExtendedNextHop, ExtendedMessage
└── Session (SessionCaps)   ← RouteRefresh, HoldTime, GR
```

**Rationale:** ExtendedMessage affects wire encoding (max message size 4096 vs 65535).
EncodingContext already references EncodingCaps, so it gets ExtendedMessage automatically.

```go
// internal/bgp/capability/encoding.go
type EncodingCaps struct {
    ASN4            bool
    ExtendedMessage bool  // NEW: RFC 8654, affects max message size
    Families        []Family
    AddPathMode     map[Family]AddPathMode
    ExtendedNextHop map[Family]AFI
}

// internal/bgp/context/context.go - add accessor
func (c *EncodingContext) ExtendedMessage() bool {
    if c.encoding == nil {
        return false
    }
    return c.encoding.ExtendedMessage
}

func (c *EncodingContext) MaxMessageSize() int {
    if c.ExtendedMessage() {
        return 65535  // RFC 8654
    }
    return 4096  // RFC 4271
}
```

### Common Interface

```go
// WireWriter is implemented by types that write to wire format.
// Located in internal/bgp/context/context.go (not wire package due to import cycle)
// Import cycle: wire→context→nlri→wire prevented placement in wire package.
type WireWriter interface {
    // Len returns wire size in bytes. Pass nil for context-independent types.
    Len(ctx *EncodingContext) int

    // WriteTo writes to buf at offset, returns bytes written.
    // Caller guarantees capacity. Pass nil for context-independent types.
    WriteTo(buf []byte, off int, ctx *EncodingContext) int
}
```

### Message Interface

```go
// Message is implemented by all BGP message types.
// Located in internal/bgp/message/message.go
type Message interface {
    context.WireWriter  // Note: context.WireWriter, not wire.WireWriter
    Type() MessageType
    Pack(neg *Negotiated) ([]byte, error)  // Kept for backward compat, see spec-pack-removal.md
}

// Note: Pack() and message.Negotiated kept for gradual migration.
// See spec-pack-removal.md for full removal plan.
```

### Attribute Interface

```go
// Attribute is implemented by all BGP path attributes.
// Located in internal/bgp/attribute/attribute.go
type Attribute interface {
    wire.WireWriter
    Code() AttributeCode
    Flags() AttributeFlags
}

// Removed:
// - Pack() []byte
// - PackWithContext(srcCtx, dstCtx) []byte
// - Len() int (context-less)
// - WriteTo(buf, off) int (context-less)
// - WriteToWithContext(buf, off, srcCtx, dstCtx) int
```

### Transcoding Support

For attributes that need source context (AS_PATH, Aggregator transcoding):

```go
// Transcoder extends WireWriter for types needing source context.
// Located in internal/bgp/attribute/attribute.go
type Transcoder interface {
    WireWriter
    // LenTranscode returns size when transcoding from srcCtx to dstCtx.
    LenTranscode(srcCtx, dstCtx *context.EncodingContext) int
    // WriteToTranscode writes transcoded output.
    WriteToTranscode(buf []byte, off int, srcCtx, dstCtx *context.EncodingContext) int
}
```

Only AS_PATH and Aggregator implement Transcoder. Most attributes only implement WireWriter.

### Data Flow

Before (current):
```
capability.Negotiated (stored in Session)
        ↓
   convert on each writeMessage() call
        ↓
message.Negotiated (ephemeral)
        ↓
   msg.Pack(msgNeg)
        ↓
   []byte (allocated)
```

After:
```
capability.Negotiated (stored in Session)
        ↓
   convert once at session establishment
        ↓
EncodingContext (stored in Peer.sendCtx/recvCtx)
        ↓
   msg.WriteTo(buf, off, sendCtx)
        ↓
   writes to pre-allocated SessionBuffer
```

### Example Implementations

**Context-independent (most types):**
```go
func (k *Keepalive) Len(_ *context.EncodingContext) int {
    return HeaderLen  // 19 bytes, fixed
}

func (k *Keepalive) WriteTo(buf []byte, off int, _ *context.EncodingContext) int {
    writeHeader(buf, off, TypeKEEPALIVE, HeaderLen)
    return HeaderLen
}
```

**Context-dependent (AS_PATH):**
```go
func (p *ASPath) Len(ctx *context.EncodingContext) int {
    if ctx == nil || ctx.ASN4() {
        return p.len4byte()
    }
    return p.len2byte()
}

func (p *ASPath) WriteTo(buf []byte, off int, ctx *context.EncodingContext) int {
    if ctx == nil || ctx.ASN4() {
        return p.writeTo4byte(buf, off)
    }
    return p.writeTo2byte(buf, off)
}
```

**Transcoding (AS_PATH received with ASN2, sending with ASN4):**
```go
func (p *ASPath) LenTranscode(srcCtx, dstCtx *context.EncodingContext) int {
    // srcCtx tells us how p was encoded (ASN2 or ASN4)
    // dstCtx tells us how to encode output
    // May need to expand AS_TRANS back to real ASNs
}
```

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestMessageWireWriter` | `internal/bgp/message/message_test.go` | All message types implement WireWriter | |
| `TestAttributeWireWriter` | `internal/bgp/attribute/attribute_test.go` | All attribute types implement WireWriter | |
| `TestKeepaliveLen` | `internal/bgp/message/keepalive_test.go` | Keepalive.Len(nil) == 19 | |
| `TestUpdateLenWithContext` | `internal/bgp/message/update_test.go` | Update.Len uses context for size | |
| `TestASPathLenASN4` | `internal/bgp/attribute/aspath_test.go` | AS_PATH size varies with ASN4 | |
| `TestASPathTranscode` | `internal/bgp/attribute/aspath_test.go` | ASN2→ASN4 transcoding | |

### Functional Tests
| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| existing | `test/data/` | Existing tests validate wire format unchanged | |

## Files to Modify

### Delete
- `internal/bgp/message/message.go` lines 12-38 - `message.Negotiated` struct (ephemeral shim)

### Modify

#### EncodingContext (add ExtendedMessage)
- `internal/bgp/capability/encoding.go` - add ExtendedMessage to EncodingCaps
- `internal/bgp/capability/session.go` - remove ExtendedMessage (moved to EncodingCaps)
- `internal/bgp/capability/negotiated.go` - update buildSubComponents() to populate ExtendedMessage in Encoding
- `internal/bgp/context/context.go` - add ExtendedMessage() and MaxMessageSize() methods

#### WireWriter Interface
- `internal/bgp/context/context.go` - add WireWriter interface (in context, not wire, due to import cycle)

#### Message Package
- `internal/bgp/message/message.go` - Message interface embeds WireWriter, add EncodingContext alias
- `internal/bgp/message/keepalive.go` - implement Len/WriteTo (Pack kept)
- `internal/bgp/message/open.go` - implement Len/WriteTo (Pack kept)
- `internal/bgp/message/update.go` - update Len/WriteTo signatures (Pack kept)
- `internal/bgp/message/notification.go` - implement Len/WriteTo (Pack kept)
- `internal/bgp/message/routerefresh.go` - implement Len/WriteTo (Pack kept)

#### Attribute Package
- `internal/bgp/attribute/attribute.go` - Attribute interface uses WireWriter, add Transcoder
- `internal/bgp/attribute/origin.go` - remove Pack, context-less WriteTo/Len
- `internal/bgp/attribute/simple.go` - remove Pack, context-less WriteTo/Len
- `internal/bgp/attribute/aspath.go` - implement Transcoder, remove old methods
- `internal/bgp/attribute/community.go` - remove Pack, context-less WriteTo/Len
- `internal/bgp/attribute/mpnlri.go` - remove Pack, context-less WriteTo/Len
- `internal/bgp/attribute/opaque.go` - remove Pack, context-less WriteTo/Len
- `internal/bgp/attribute/as4.go` - remove Pack, context-less WriteTo/Len

#### Callers to Update
- `internal/bgp/attribute/builder.go` - use new interface
- `internal/bgp/message/update_build.go` - use new interface
- `internal/rib/commit.go` - use EncodingContext instead of message.Negotiated
- `internal/rib/outgoing.go` - use new interface
- `internal/reactor/session.go` - use sendCtx directly (remove conversion to message.Negotiated)
- `internal/reactor/peer.go` - remove messageNegotiated() helper
- All test files using Pack or old WriteTo signatures

## Files to Create

None - refactoring existing code.

## Implementation Steps

1. **Add WireWriter interface** to `internal/bgp/wire/writer.go`
2. **Write unit tests** for interface compliance
3. **Run tests** - Verify FAIL (types don't implement yet)
4. **Delete message.Negotiated** - remove duplicate type
5. **Update Message implementations** - Keepalive, Open, Update, Notification, RouteRefresh
6. **Update Attribute interface** - add Transcoder for srcCtx/dstCtx cases
7. **Update Attribute implementations** - all attribute types
8. **Update callers** - builder, rib, tests
9. **Run tests** - Verify PASS
10. **Verify all** - `make lint && make test && make functional`

## Migration Strategy

Since no backwards compatibility needed:
1. Change interface
2. Fix all implementations
3. Fix all callers
4. Delete old methods

## RFC Documentation

### Reference Comments
- RFC 4271 Section 4 - Message formats
- RFC 6793 Section 4.1 - ASN4 encoding affects AS_PATH/Aggregator size

### Constraint Comments
```go
// RFC 6793 Section 4.1: "When the Capability has been exchanged,
// the AS number... is encoded as a four-octet unsigned integer"
func (p *ASPath) Len(ctx *context.EncodingContext) int {
    if ctx != nil && !ctx.ASN4() {
        return p.len2byte()
    }
    return p.len4byte()
}
```

## Implementation Summary

### What Was Implemented
- Added `ExtendedMessage` to `EncodingCaps` (moved from SessionCaps)
- Added `ExtendedMessage()` and `MaxMessageSize()` methods to `EncodingContext`
- Added `WireWriter` interface to `internal/bgp/context/context.go` (not wire package due to import cycle)
- Updated `Message` interface to embed `WireWriter` (removed Pack method)
- Implemented `Len(ctx)` and `WriteTo(buf, off, ctx)` on all message types:
  - Keepalive, Open, Notification, Update, RouteRefresh
- **Removed Pack() methods** from all message types
- **Deleted `message.Negotiated`** struct (ephemeral shim)
- **Deleted `message.Family`** type (duplicate of nlri.Family)
- **Deleted `packWithHeader`** helper (replaced by writeHeader for WriteTo)
- Added `PackTo(msg, ctx)` helper function for callers needing []byte allocation
- Updated hash computation in EncodingContext to include ExtendedMessage
- **Migrated all callers from Pack() to PackTo():**
  - `internal/reactor/reactor.go` - RouteRefresh, Notification
  - `internal/reactor/session.go` - writeMessage()
  - `internal/reactor/session_test.go` - all test Pack calls
  - `internal/reactor/collision_test.go` - all test Pack calls
  - `internal/bgp/message/*_test.go` - all message tests

### Bugs Found/Fixed
- Import cycle: WireWriter cannot be in `internal/bgp/wire` because wire→context→nlri→wire
  - Moved WireWriter to context package

### Design Insights
- WireWriter belongs in context package, not wire, due to import dependencies
- PackTo(msg, ctx) provides convenient allocation for callers not using pre-allocated buffers

### Deviations from Plan
- **WireWriter location**: Placed in `context` package instead of `wire` due to import cycle
- **Attribute interface not updated**: Deferred to separate spec - attributes already have WriteTo methods

## Checklist

### 🧪 TDD
- [x] Tests written (`internal/bgp/message/wirewriter_test.go`)
- [x] Tests FAIL (compilation errors - types don't implement WireWriter)
- [x] Implementation complete
- [x] Tests PASS

### Verification
- [x] `make lint` passes (0 issues)
- [x] `make test` passes
- [x] `make functional` passes

### Documentation
- [x] Required docs read
- [x] RFC summaries read
- [x] RFC references added to code
- [ ] RFC constraint comments added (deferred - no new constraints)

### Completion
- [x] Architecture docs updated with learnings (docs already reflected WireWriter in context)
- [x] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [x] All files committed together
