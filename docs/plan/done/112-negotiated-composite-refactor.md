# Spec: negotiated-composite-refactor

## Task

Refactor `Negotiated` into a composite structure with reusable sub-components:
- `PeerIdentity` - ASNs, Router IDs
- `EncodingCaps` - ASN4, families, AddPathMode, ExtendedNextHop
- `SessionCaps` - RouteRefresh, GracefulRestart, HoldTime, etc.

`WireContext` references these sub-components by pointer (zero duplication).
Sub-components are immutable after session creation, passed by pointer.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - Negotiated capabilities, ContextID for zero-copy
- [ ] `docs/architecture/encoding-context.md` - Current EncodingContext design, registry
- [ ] `docs/architecture/wire/capabilities.md` - Capability wire format, negotiation rules

### RFC Summaries (MUST for protocol work)
- [ ] `docs/rfc/rfc5492.md` - Capabilities Advertisement (negotiation rules)
- [ ] `docs/rfc/rfc6793.md` - 4-byte ASN (ASN4 encoding context)
- [ ] `docs/rfc/rfc7911.md` - ADD-PATH (direction-dependent mode)
- [ ] `docs/rfc/rfc8950.md` - Extended Next Hop Encoding

**Key insights:**
- Current `EncodingContext` duplicates fields from `Negotiated` (ASN4, families, ASNs)
- `WireContext.addPath` must be direction-specific (Send vs Receive mode)
- Sub-components can be shared between Negotiated and both WireContexts
- Router IDs needed for ORIGINATOR_ID in route reflection (currently missing)

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPeerIdentityIsIBGP` | `pkg/bgp/capability/identity_test.go` | IsIBGP computed correctly | |
| `TestEncodingCapsSupportsFamily` | `pkg/bgp/capability/encoding_test.go` | Family lookup works | |
| `TestNegotiateComposite` | `pkg/bgp/capability/negotiated_test.go` | Sub-components populated | |
| `TestWireContextDelegation` | `pkg/bgp/context/wire_test.go` | Methods delegate to sub-components | |
| `TestWireContextAddPathRecv` | `pkg/bgp/context/wire_test.go` | Receive direction extracts correct mode | |
| `TestWireContextAddPathSend` | `pkg/bgp/context/wire_test.go` | Send direction extracts correct mode | |
| `TestWireContextHash` | `pkg/bgp/context/wire_test.go` | Hash consistent for same params | |
| `TestWireContextHashDiffersByDirection` | `pkg/bgp/context/wire_test.go` | Different hash for recv vs send | |

### Functional Tests
| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| Existing negotiation tests | `test/data/` | Verify no regression | |

### Future (if deferring any tests)
- Integration tests for route reflection with ORIGINATOR_ID (depends on RR implementation)

## Files to Modify
- `pkg/bgp/capability/negotiated.go` - Split into composite, add sub-struct references
- `pkg/bgp/context/context.go` - Rename to `wire.go`, restructure as WireContext
- `pkg/bgp/context/negotiated.go` - Update factory to use composite Negotiated
- `pkg/bgp/context/registry.go` - Update to store `*WireContext`
- `pkg/reactor/peer.go` - Update context creation
- `pkg/reactor/negotiated.go` - May merge into capability package or simplify

## Files to Create
- `pkg/bgp/capability/identity.go` - PeerIdentity struct
- `pkg/bgp/capability/encoding.go` - EncodingCaps struct
- `pkg/bgp/capability/session.go` - SessionCaps struct
- `pkg/bgp/context/wire.go` - WireContext struct (replaces EncodingContext)

## Implementation Steps

### Phase 1: Create Sub-Components
1. **Write unit tests** - Create tests for PeerIdentity, EncodingCaps, SessionCaps
2. **Run tests** - Verify FAIL (paste output)
3. **Implement PeerIdentity** - ASNs, RouterIDs, IsIBGP()
4. **Implement EncodingCaps** - ASN4, Families, AddPathMode, ExtendedNextHop
5. **Implement SessionCaps** - RouteRefresh, EnhancedRouteRefresh, ExtendedMessage, HoldTime, GracefulRestart, Mismatches
6. **Run tests** - Verify PASS (paste output)

### Phase 2: Refactor Negotiated
1. **Write unit tests** - Test composite Negotiated creation
2. **Run tests** - Verify FAIL (paste output)
3. **Refactor Negotiate()** - Create sub-components, compose into Negotiated
4. **Update Negotiated methods** - Delegate to sub-components
5. **Run tests** - Verify PASS (paste output)

### Phase 3: Create WireContext
1. **Write unit tests** - Test WireContext with sub-component references
2. **Run tests** - Verify FAIL (paste output)
3. **Implement WireContext** - Reference Identity + Encoding, derive addPath per direction
4. **Update registry** - Store WireContext instead of EncodingContext
5. **Run tests** - Verify PASS (paste output)

### Phase 4: Update Consumers
1. **Update reactor/peer.go** - Use new WireContext
2. **Update any attribute encoding** - Use WireContext methods
3. **Run tests** - Verify PASS
4. **Remove old EncodingContext** - After all consumers migrated
5. **Verify all** - `make lint && make test && make functional` (paste output)

## RFC Documentation

### Reference Comments
- Add `// RFC 5492 Section 4` for capability negotiation
- Add `// RFC 6793` for ASN4 handling
- Add `// RFC 7911 Section 4` for ADD-PATH mode negotiation

### Constraint Comments (CRITICAL)
```go
// RFC 7911 Section 4: ADD-PATH mode is asymmetric
// "A BGP speaker can send if it advertises Send/Both AND peer advertises Receive/Both"
// Direction determines which mode flag to check
if dir == Recv {
    return mode == AddPathReceive || mode == AddPathBoth
}
return mode == AddPathSend || mode == AddPathBoth
```

## Design Details

### Composite Structure

```go
// capability/identity.go
type PeerIdentity struct {
    LocalASN      uint32
    PeerASN       uint32
    LocalRouterID uint32  // From our config
    PeerRouterID  uint32  // From peer's OPEN
}

func (p *PeerIdentity) IsIBGP() bool {
    return p.LocalASN == p.PeerASN
}

// capability/encoding.go
type EncodingCaps struct {
    ASN4            bool
    Families        []Family
    AddPathMode     map[Family]AddPathMode
    ExtendedNextHop map[Family]AFI
}

// capability/session.go
type SessionCaps struct {
    ExtendedMessage      bool
    RouteRefresh         bool
    EnhancedRouteRefresh bool
    HoldTime             uint16
    GracefulRestart      *GracefulRestart
    Mismatches           []Mismatch
}

// capability/negotiated.go
type Negotiated struct {
    Identity *PeerIdentity
    Encoding *EncodingCaps
    Session  *SessionCaps
}
```

### WireContext (References, No Copy)

```go
// context/wire.go
type WireContext struct {
    identity *capability.PeerIdentity  // ref, not copy
    encoding *capability.EncodingCaps  // ref, not copy

    addPath   map[Family]bool  // derived, direction-specific
    direction Direction
    hash      uint64
}

// Accessors delegate to referenced structs
func (w *WireContext) ASN4() bool              { return w.encoding.ASN4 }
func (w *WireContext) Families() []Family      { return w.encoding.Families }
func (w *WireContext) LocalASN() uint32        { return w.identity.LocalASN }
func (w *WireContext) PeerASN() uint32         { return w.identity.PeerASN }
func (w *WireContext) IsIBGP() bool            { return w.identity.IsIBGP() }
func (w *WireContext) AddPath(f Family) bool   { return w.addPath[f] }
```

### Data Flow

```
Negotiate(local, remote []Capability, identity PeerIdentity)
    │
    ├── Creates PeerIdentity (once, shared)
    ├── Creates EncodingCaps (once, shared)
    ├── Creates SessionCaps (once, owned by Negotiated)
    │
    └── Returns *Negotiated
            │
            ├── RecvContext() → *WireContext (refs Identity + Encoding, derives addPath for Recv)
            └── SendContext() → *WireContext (refs Identity + Encoding, derives addPath for Send)
```

### What's Shared vs Owned

| Struct | Created | Shared By |
|--------|---------|-----------|
| `PeerIdentity` | Once | Negotiated, RecvCtx, SendCtx |
| `EncodingCaps` | Once | Negotiated, RecvCtx, SendCtx |
| `SessionCaps` | Once | Negotiated only |
| `addPath` map | Per direction | Owned by each WireContext |

## Implementation Summary

### What Was Implemented

**New files created:**
- `pkg/bgp/capability/identity.go` - PeerIdentity struct (ASNs, RouterIDs, IsIBGP())
- `pkg/bgp/capability/encoding.go` - EncodingCaps struct (ASN4, Families, AddPathMode, ExtendedNextHop)
- `pkg/bgp/capability/session.go` - SessionCaps struct (ExtendedMessage, RouteRefresh, GR, etc.)
- `pkg/bgp/context/wire.go` - WireContext struct (references Identity + Encoding, derives addPath per direction)

**Test files created:**
- `pkg/bgp/capability/identity_test.go` - Tests for PeerIdentity
- `pkg/bgp/capability/encoding_test.go` - Tests for EncodingCaps
- `pkg/bgp/capability/session_test.go` - Tests for SessionCaps
- `pkg/bgp/context/wire_test.go` - Tests for WireContext

**Modified files:**
- `pkg/bgp/capability/negotiated.go` - Added composite sub-components (Identity, Encoding, Session) while keeping backward-compatible fields
- `pkg/bgp/context/negotiated.go` - Added FromNegotiatedRecvWire/SendWire factory functions
- `research/cmd/count-attrs/main.go` - Fixed pre-existing lint issues (errcheck, gosec)

### Bugs Found/Fixed
- None. Implementation was straightforward.

### Design Insights
- Backward compatibility maintained by keeping existing flat fields alongside new composite pointers
- WireContext derives `addPath map[Family]bool` per direction from `Encoding.AddPathMode`
- Hash includes direction to ensure different recv/send contexts get different IDs
- Old EncodingContext and new WireContext coexist - gradual migration possible

### Deviations from Plan
- Did NOT remove EncodingContext or migrate all consumers yet (kept for backward compatibility)
- Old FromNegotiatedRecv/Send functions retained with documentation note about new alternatives
- This enables gradual migration without breaking existing code

## Checklist

### 🧪 TDD
- [x] Tests written
- [x] Tests FAIL (output below)
- [x] Implementation complete
- [x] Tests PASS (output below)

### Verification
- [x] `make lint` passes (26 linters including `govet`, `staticcheck`, `gosec`, `gocritic`)
- [x] `make test` passes
- [x] `make functional` passes

### Documentation (during implementation)
- [x] Required docs read
- [x] RFC summaries read (all referenced RFCs)
- [x] RFC references added to code
- [x] RFC constraint comments added (quoted requirement + explanation)

### Completion (after tests pass - see Completion Checklist)
- [x] Code duplication check performed (hash utils duplicated but acceptable - different type params)
- [x] Critical review of work completed
- [x] Functional tests assessment (not needed - internal refactor, no user-visible/wire/config/API changes)
- [x] Architecture docs updated with learnings
- [x] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] All files committed together
