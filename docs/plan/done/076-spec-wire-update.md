# Spec: Wire Update Types

## Status: Phase 1 & 2 Complete

API-only mode. No pool, no RIB. Concrete wire types, no interface indirection.

## Task

Implement zero-copy UPDATE message parsing using concrete wire types.

## Required Reading (MUST complete before implementation)

- [x] `.claude/zebgp/ENCODING_CONTEXT.md` - Zero-copy pattern, ContextID comparison
- [x] `.claude/zebgp/UPDATE_BUILDING.md` - Forward path vs Build path (WireUpdate is Forward path)
- [x] `pkg/bgp/attribute/wire.go` - AttributesWire pattern to follow
- [x] `pkg/plugin/mpwire.go` - MPReachWire/MPUnreachWire already implemented

**Key insights from docs:**
- Zero-copy rule: `sourceCtxID == destCtxID` → return original bytes
- WireUpdate is receive-side (Forward path), not for building UPDATEs
- AttributesWire already exists with lazy parsing + sync.Once pattern
- MPReachWire/MPUnreachWire are `[]byte` type aliases with accessor methods

---

## 🧪 TDD Test Plan (MANDATORY - Write tests FIRST)

### Unit Tests
| Test | File | What it validates |
|------|------|-------------------|
| `TestWireUpdate_Derived` | `pkg/plugin/wire_update_test.go` | Withdrawn/Attrs/NLRI return correct slices |
| `TestWireUpdate_Empty` | `pkg/plugin/wire_update_test.go` | Empty sections return nil |
| `TestWireUpdate_Malformed` | `pkg/plugin/wire_update_test.go` | Truncated data returns nil gracefully |
| `TestWireUpdate_MPReach` | `pkg/plugin/wire_update_test.go` | MP_REACH_NLRI extraction |
| `TestWireUpdate_MPUnreach` | `pkg/plugin/wire_update_test.go` | MP_UNREACH_NLRI extraction |
| `TestWireUpdate_SourceCtxID` | `pkg/plugin/wire_update_test.go` | Context ID preserved |
| `TestWireUpdate_Payload` | `pkg/plugin/wire_update_test.go` | Raw payload access (zero-copy) |
| `TestWireUpdate_AttrsCached` | `pkg/plugin/wire_update_test.go` | AttributesWire caching with sync.Once |

### Integration Tests
| Test | File | What it validates |
|------|------|-------------------|
| `TestNotifyMessageReceiverWireUpdate` | `pkg/reactor/reactor_test.go` | WireUpdate set in RawMessage |

---

## Files to Modify

### New Files
- `pkg/plugin/wire_update.go` - WireUpdate struct and methods
- `pkg/plugin/wire_update_test.go` - Unit tests

### Modified Files
- `pkg/plugin/types.go` - Add WireUpdate field to RawMessage
- `pkg/plugin/decode.go` - Remove ExtractAttributeBytes (replaced by WireUpdate)
- `pkg/plugin/decode_test.go` - Remove ExtractAttributeBytes tests
- `pkg/plugin/text_test.go` - Update to use WireUpdate
- `pkg/plugin/filter_test.go` - Update to use WireUpdate
- `pkg/reactor/reactor.go` - Create WireUpdate in notifyMessageReceiver
- `pkg/reactor/reactor_test.go` - Add integration test

---

## Current State

- **Last commit:** `26969b6` feat(api): implement WireUpdate for zero-copy UPDATE parsing
- **Tests:** All pass (9 WireUpdate tests + integration test)
- **Lint:** Clean (only pre-existing deprecation warnings)

---

## Overview

Parse BGP UPDATE messages using concrete wire types. Direct method calls, no interface overhead. API receives updates, handles storage/resend on session restart. GC manages memory.

## Memory Model

```
┌─────────────────────────────────────────────────────────────────────────┐
│  SIMPLE: Allocate per read, GC frees when done                          │
│                                                                         │
│  buf := make([]byte, msgLen) ──► conn.Read(buf) ──► WireUpdate{buf}    │
│                                                           │             │
│                                                     API callback        │
│                                                           │             │
│                                                     GC frees when       │
│                                                     unreferenced        │
└─────────────────────────────────────────────────────────────────────────┘
```

No pool. No interning. No reference counting. No interfaces.

---

## RFC 4271 UPDATE Format

```
UPDATE Message (Section 4.3):
┌────────────────────────────────────────────────────────┐
│  Withdrawn Routes Length      (2 octets)               │
├────────────────────────────────────────────────────────┤
│  Withdrawn Routes             (variable)               │
├────────────────────────────────────────────────────────┤
│  Total Path Attribute Length  (2 octets)               │
├────────────────────────────────────────────────────────┤
│  Path Attributes              (variable)               │
├────────────────────────────────────────────────────────┤
│  Network Layer Reachability Information (variable)     │
└────────────────────────────────────────────────────────┘
```

All positions derivable from the two length fields.

---

## Concrete Types

### WireUpdate

```go
// WireUpdate holds UPDATE bytes. GC manages lifetime.
// All methods return concrete types, no interface indirection.
// Thread-safe for concurrent read access.
type WireUpdate struct {
    payload     []byte
    sourceCtxID ContextID

    // Cached AttributesWire - lazily initialized, thread-safe
    attrsOnce sync.Once
    attrs     *attribute.AttributesWire
}

func NewWireUpdate(payload []byte, ctxID ContextID) *WireUpdate

func (u *WireUpdate) Withdrawn() []byte          // Returns nil if empty/malformed
func (u *WireUpdate) Attrs() *AttributesWire     // Cached - same instance per WireUpdate
func (u *WireUpdate) NLRI() []byte               // Returns nil if empty/malformed
func (u *WireUpdate) MPReach() MPReachWire       // Uses cached Attrs(), returns []byte alias
func (u *WireUpdate) MPUnreach() MPUnreachWire   // Uses cached Attrs(), returns []byte alias
func (u *WireUpdate) SourceCtxID() ContextID
func (u *WireUpdate) Payload() []byte
```

### AttributesWire

```go
// AttributesWire wraps path attribute bytes.
// Concrete type, no interface.
type AttributesWire struct {
    packed      []byte
    sourceCtxID ContextID
    index       []attrIndex
    parsed      map[AttributeCode]Attribute
    mu          sync.RWMutex
}

func (a *AttributesWire) GetRaw(code AttributeCode) ([]byte, error)
func (a *AttributesWire) Get(code AttributeCode) (Attribute, error)
func (a *AttributesWire) Has(code AttributeCode) bool
func (a *AttributesWire) Packed() []byte
func (a *AttributesWire) Codes() []AttributeCode
```

### MPReachWire (RFC 4760)

```go
// MPReachWire wraps MP_REACH_NLRI attribute (code 14)
// Format: [AFI:2][SAFI:1][NH_Len:1][NextHop:var][Reserved:1][NLRI:var]
// Type alias for []byte with accessor methods.
type MPReachWire []byte

func (m MPReachWire) AFI() uint16
func (m MPReachWire) SAFI() uint8
func (m MPReachWire) Family() nlri.Family
func (m MPReachWire) NextHop() netip.Addr
func (m MPReachWire) NLRI() []byte
```

### MPUnreachWire (RFC 4760)

```go
// MPUnreachWire wraps MP_UNREACH_NLRI attribute (code 15)
// Format: [AFI:2][SAFI:1][Withdrawn:var]
// Type alias for []byte with accessor methods.
type MPUnreachWire []byte

func (m MPUnreachWire) AFI() uint16
func (m MPUnreachWire) SAFI() uint8
func (m MPUnreachWire) Family() nlri.Family
func (m MPUnreachWire) Withdrawn() []byte
```

---

## Implementation

### Actual Implementation (matches code)

```go
// MPReach extracts MP_REACH_NLRI as MPReachWire ([]byte alias)
func (u *WireUpdate) MPReach() MPReachWire {
    attrs := u.Attrs()  // Uses cached AttributesWire
    if attrs == nil {
        return nil
    }
    raw, err := attrs.GetRaw(attribute.AttrMPReachNLRI)
    if err != nil || len(raw) < 5 {
        return nil
    }
    return MPReachWire(raw)  // Type conversion, not struct creation
}

// MPUnreach extracts MP_UNREACH_NLRI as MPUnreachWire ([]byte alias)
func (u *WireUpdate) MPUnreach() MPUnreachWire {
    attrs := u.Attrs()  // Uses cached AttributesWire
    if attrs == nil {
        return nil
    }
    raw, err := attrs.GetRaw(attribute.AttrMPUnreachNLRI)
    if err != nil || len(raw) < 3 {
        return nil
    }
    return MPUnreachWire(raw)  // Type conversion, not struct creation
}
```

---

## Usage

### Receiving Updates (API callback)

```go
// In RawMessage (types.go)
type RawMessage struct {
    Type       message.MessageType
    RawBytes   []byte
    WireUpdate *WireUpdate  // Set for UPDATE messages
    AttrsWire  *attribute.AttributesWire  // Derived from WireUpdate.Attrs()
    // ...
}

// In message receiver callback
func (h *Handler) OnMessageReceived(peer api.PeerInfo, msg api.RawMessage) {
    if msg.WireUpdate != nil {
        // Access UPDATE sections
        withdrawn := msg.WireUpdate.Withdrawn()
        nlri := msg.WireUpdate.NLRI()
        attrs := msg.WireUpdate.Attrs()
        mpReach := msg.WireUpdate.MPReach()
        mpUnreach := msg.WireUpdate.MPUnreach()
    }
}
```

---

## Checklist

### 🧪 TDD (MUST complete in order)
- [x] Unit tests written (`pkg/plugin/wire_update_test.go`)
- [x] Integration test written (`pkg/reactor/reactor_test.go`)
- [x] Tests run and FAIL (before implementation)
- [x] Implementation complete
- [x] Tests run and PASS (9 tests pass)

### Phase 1: Wire Types
- [x] Implement `WireUpdate` struct and methods (`pkg/plugin/wire_update.go`)
- [x] Implement `MPReachWire` struct and methods (already in `pkg/plugin/mpwire.go`)
- [x] Implement `MPUnreachWire` struct and methods (already in `pkg/plugin/mpwire.go`)
- [x] Implement derived accessors with bounds checking
- [x] Ensure `AttributesWire` exists (already in `pkg/bgp/attribute/wire.go`)
- [x] Unit tests: `pkg/plugin/wire_update_test.go` (9 tests pass)

### Phase 2: Integration
- [x] Update connection read loop to create `*WireUpdate` (reactor.go:notifyMessageReceiver)
- [x] Add `WireUpdate` field to `RawMessage` (types.go)
- [x] Derive `AttrsWire` from `WireUpdate.Attrs()` for backward compat
- [x] Cache `AttributesWire` in `WireUpdate` with `sync.Once` for efficiency
- [x] Integration test: `TestNotifyMessageReceiverWireUpdate`
- [x] Remove redundant `ExtractAttributeBytes` (replaced by `WireUpdate.Attrs()`)
- [x] Tests pass (existing tests + WireUpdate tests)

### Phase 2.5: Zero-Copy End-to-End
- [x] Add buffer pool to session (`readBufPool sync.Pool` in session.go)
- [x] Create `WireUpdate` in `session.processMessage()` for UPDATE messages
- [x] Transfer buffer ownership to `WireUpdate` (session gets fresh buffer from pool)
- [x] Update `MessageCallback` signature: add `wireUpdate *api.WireUpdate`, `ctxID bgpctx.ContextID`
- [x] Add `session.recvCtxID` field (set by Peer via `SetRecvCtxID()` after negotiation)
- [x] Update `handleUpdate(*api.WireUpdate)` signature
- [x] Update `notifyMessageReceiver` - no copy for received UPDATE with WireUpdate
- [x] All tests pass with race detector

### Phase 3: ReceivedUpdate Migration
- [x] Update `ReceivedUpdate` struct to use `*WireUpdate` instead of `RawBytes + Attrs + SourceCtxID`
- [x] Update `ReceivedUpdate.ConvertToRoutes()` to use `WireUpdate`
- [x] Update cache operations to use `WireUpdate`
- [x] Update all tests to use new struct

### Verification
- [x] `make test` passes
- [x] `make lint` passes
- [x] `make functional` passes (18 tests)

### Documentation
- [x] Required docs read
- [x] RFC references added to protocol code (RFC 4271 Section 4.3, RFC 4760)
- [x] Updated `.claude/zebgp/ENCODING_CONTEXT.md` - added WireUpdate zero-copy receive path
- [x] Updated `.claude/zebgp/UPDATE_BUILDING.md` - added Path 0: Receive Path

### Completion
- [x] All phases complete
- [ ] Move spec to `docs/plan/done/NNN-wire-update.md`

---

## What's NOT in This Spec

| Feature | Reason |
|---------|--------|
| Interfaces | Concrete types only, no indirection |
| Pool | No deduplication needed, GC handles memory |
| RIB | API manages route state |
| Reference counting | GC handles lifetime |
| Compaction | No pool, no fragmentation |
| Mode selection | Always API mode |

---

**Created:** 2025-01-03
**Phase 1+2 Complete:** 2025-01-05
**Phase 2.5+3 Complete:** 2026-01-05
**Status:** Complete - ready for docs/plan/done/
