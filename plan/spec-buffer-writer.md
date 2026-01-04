# Spec: buffer-writer

## Task

Implement the buffer writer architecture described in `.claude/zebgp/wire/BUFFER_WRITER.md` - zero-allocation UPDATE message building with fixed session buffers and BufWriter interface.

## Required Reading (MUST complete before implementation)

- [x] `.claude/zebgp/wire/BUFFER_WRITER.md` - Core design
- [x] `.claude/zebgp/UPDATE_BUILDING.md` - Build vs Forward paths
- [x] `.claude/zebgp/ENCODING_CONTEXT.md` - Context-dependent encoding
- [x] `.claude/zebgp/POOL_ARCHITECTURE.md` - Memory patterns reference

**Key insights from docs:**
- Build path benefits from buffer optimization; Forward path uses zero-copy wire cache
- Session already has `readBuf` pattern - add parallel `writeBuf`
- Context-dependent encoding (ASN4, ADD-PATH) must be preserved in WriteTo variants
- Extended Message capability changes max size (4096 → 65535)

## Design Decisions

1. **Buffer lifecycle**: Allocate 4096 at session start, re-allocate to 65535 if Extended Message negotiated
2. **Backward compatibility**: Keep Pack() methods indefinitely for external callers
3. **Interface addition**: Add WriteTo() alongside existing Pack() - incremental migration
4. **No io.Writer**: Use `[]byte` + offset for zero overhead

## Files to Modify

### Phase 1: Core Infrastructure
- `pkg/bgp/wire/writer.go` - **NEW**: BufWriter interface, SessionBuffer type

### Phase 2: Attribute Writers
- `pkg/bgp/attribute/attribute.go` - Extend Attribute interface
- `pkg/bgp/attribute/origin.go` - Implement WriteTo
- `pkg/bgp/attribute/aspath.go` - Implement WriteTo with ASN4 context
- `pkg/bgp/attribute/simple.go` - NextHop, MED, LocalPref, Aggregator, etc.
- `pkg/bgp/attribute/community.go` - All community types
- `pkg/bgp/attribute/mpnlri.go` - MP_REACH_NLRI, MP_UNREACH_NLRI
- `pkg/bgp/attribute/opaque.go` - OpaqueAttribute

### Phase 3: NLRI Writers
- `pkg/bgp/nlri/nlri.go` - Extend NLRI interface
- `pkg/bgp/nlri/ipv4.go` - IPv4 unicast
- `pkg/bgp/nlri/ipv6.go` - IPv6 unicast
- `pkg/bgp/nlri/vpn.go` - VPN routes
- `pkg/bgp/nlri/evpn.go` - EVPN routes
- `pkg/bgp/nlri/flowspec.go` - FlowSpec
- `pkg/bgp/nlri/labeled.go` - Labeled unicast

### Phase 4: Session Integration
- `pkg/reactor/session.go` - Add writeBuf, resize on Extended Message

### Phase 5: Message Builder Migration
- `pkg/bgp/message/update_build.go` - Convert to SessionBuffer
- `pkg/rib/commit.go` - Convert packAttributesWithASPath
- `pkg/bgp/message/chunk_mp_nlri.go` - Convert NLRI chunking
- `pkg/rib/update.go` - Convert buildNLRIBytes

## Implementation Steps

### Phase 1: BufWriter Interface (TDD)

```go
// pkg/bgp/wire/writer.go

// BufWriter writes directly into a pre-allocated buffer.
type BufWriter interface {
    // WriteTo writes wire representation into buf at offset.
    // Returns number of bytes written.
    // Caller guarantees sufficient capacity.
    WriteTo(buf []byte, off int) int
}

// SessionBuffer wraps a fixed buffer for message building.
type SessionBuffer struct {
    buf    []byte
    offset int
}

func NewSessionBuffer(extended bool) *SessionBuffer
func (sb *SessionBuffer) Reset()
func (sb *SessionBuffer) Write(w BufWriter) int
func (sb *SessionBuffer) WriteBytes(data []byte) int
func (sb *SessionBuffer) Bytes() []byte
func (sb *SessionBuffer) Len() int
func (sb *SessionBuffer) Remaining() int
func (sb *SessionBuffer) Resize(extended bool)  // Re-allocate if needed
```

### Phase 2: Attribute Interface Extension

```go
// pkg/bgp/attribute/attribute.go

type Attribute interface {
    Code() AttributeCode
    Flags() AttributeFlags
    Len() int
    Pack() []byte                                              // Keep for compat
    PackWithContext(src, dst *bgpctx.EncodingContext) []byte   // Keep for compat

    // NEW: Zero-alloc writers
    WriteTo(buf []byte, off int) int
    WriteToWithContext(buf []byte, off int, src, dst *bgpctx.EncodingContext) int
}

// WriteAttrTo writes header + value, returns total bytes written
func WriteAttrTo(attr Attribute, buf []byte, off int) int
func WriteAttrToWithContext(attr Attribute, buf []byte, off int, src, dst *bgpctx.EncodingContext) int
```

### Phase 3: NLRI Interface Extension

```go
// pkg/bgp/nlri/nlri.go

type NLRI interface {
    Family() Family
    Pack(ctx *PackContext) []byte           // Keep for compat

    // NEW: Zero-alloc writer
    WriteTo(buf []byte, off int, ctx *PackContext) int
}
```

### Phase 4: Session Buffer Integration

```go
// pkg/reactor/session.go

type Session struct {
    // ... existing fields ...
    readBuf  []byte  // existing
    writeBuf []byte  // NEW: for message building
}

// At session creation (before OPEN):
writeBuf: make([]byte, 4096)

// After capability negotiation, if Extended Message:
func (s *Session) resizeWriteBuffer() {
    if s.negotiated.ExtendedMessage && len(s.writeBuf) < 65535 {
        s.writeBuf = make([]byte, 65535)
    }
}
```

### Phase 5: Convert Builders

Convert high-impact locations first:
1. `commit.go:packAttributesWithASPath` - 3 loops → 1 loop with copy
2. `update_build.go:BuildUnicast` - sequential appends → SessionBuffer.Write
3. `update.go:buildNLRIBytes` - size already calculated, use copy
4. `origin.go:PackAttributesOrdered` - 2 loops → 1 loop with copy

## Test Strategy

1. **Unit tests**: Each WriteTo produces identical bytes to Pack()
2. **Benchmark tests**: Measure allocation reduction
3. **Integration tests**: Full UPDATE building with SessionBuffer

```go
func TestOriginWriteToMatchesPack(t *testing.T) {
    o := Origin(OriginIGP)

    packed := o.Pack()

    buf := make([]byte, 100)
    n := o.WriteTo(buf, 0)

    if !bytes.Equal(packed, buf[:n]) {
        t.Errorf("WriteTo != Pack")
    }
}

func BenchmarkUpdateBuildOld(b *testing.B) { ... }
func BenchmarkUpdateBuildNew(b *testing.B) { ... }
```

## Checklist

- [ ] Required docs read
- [ ] Phase 1: BufWriter interface - test fails first, then passes
- [ ] Phase 2: Attribute WriteTo - test fails first, then passes
- [ ] Phase 3: NLRI WriteTo - test fails first, then passes
- [ ] Phase 4: Session writeBuf - test fails first, then passes
- [ ] Phase 5: Convert builders - test fails first, then passes
- [ ] make test passes
- [ ] make lint passes
- [ ] make functional passes
- [ ] Update `.claude/zebgp/wire/BUFFER_WRITER.md` with implementation notes

## Migration Order

Recommended order to minimize risk:

1. **Infrastructure first**: BufWriter, SessionBuffer (no existing code changes)
2. **Leaf types**: Origin, MED, LocalPref (simple, low risk)
3. **Complex types**: ASPath, Communities (context-dependent)
4. **MP types**: MP_REACH, MP_UNREACH (contain NLRI)
5. **NLRI types**: All NLRI implementations
6. **Session**: Add writeBuf
7. **Builders**: Convert one at a time, verify identical output

## Rollback Plan

- WriteTo methods are additive - old Pack() still works
- SessionBuffer is optional - builders can use either
- No breaking changes to public API
