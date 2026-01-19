# Spec: pack-removal

## Task

Remove deprecated Pack() methods from Message types. All callers should use WriteTo() with EncodingContext instead.

## Background

This is a follow-up to `spec-wirewriter-unification.md` which:
- Added WireWriter interface with `Len(ctx)` and `WriteTo(buf, off, ctx)`
- Implemented WireWriter on all Message types
- Kept Pack() for backward compatibility during migration

## Required Reading

### Architecture Docs
- [x] `docs/architecture/encoding-context.md` - EncodingContext usage

### RFC Summaries
- [ ] `docs/rfc/rfc4271.md` - BGP message formats

**Key insights:**
- Pack() allocates; WriteTo() writes to pre-allocated buffer
- `message.Negotiated` is a shim that duplicates `capability.Negotiated` sub-components
- `message.Family` struct (in message.go) duplicates `capability.Family`/`nlri.Family`
- `message.PackTo()` already exists as migration helper

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPackToEquivalence` | `internal/bgp/message/wirewriter_test.go` | PackTo produces same output as Pack | |

### Functional Tests
| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| existing | `test/data/` | Existing tests validate wire format unchanged | |

## Files to Modify

### Delete (from message.go)
- `message.Negotiated` struct (lines 23-43)
- `message.Family` struct (lines 46-49)
- `Pack()` from Message interface (lines 17-20)
- `packWithHeader()` helper (lines 52-72) - replaced by writeHeader

### Remove Pack() methods
- `internal/bgp/message/keepalive.go` - Remove Pack() method
- `internal/bgp/message/open.go` - Remove Pack() method
- `internal/bgp/message/update.go` - Remove Pack() method
- `internal/bgp/message/notification.go` - Remove Pack() method
- `internal/bgp/message/routerefresh.go` - Remove Pack() method

### Migrate to EncodingContext
- `internal/rib/commit.go` - Replace `*message.Negotiated` with `*bgpctx.EncodingContext`
- `internal/rib/commit_test.go` - Use EncodingContext in tests
- `internal/rib/commit_edge_test.go` - Use EncodingContext in tests
- `internal/rib/commit_wire_test.go` - Use EncodingContext in tests
- `internal/reactor/peer.go` - Remove `messageNegotiated()` function

### Update test files using Pack()
- `internal/bgp/message/eor_test.go` - Use PackTo instead
- `internal/bgp/message/update_build_evpn_test.go` - Use PackTo instead
- `internal/bgp/message/wirewriter_test.go` - Remove Pack() references
- `internal/bgp/message/update_test.go` - Use PackTo instead

## Files NOT Modified
- `internal/reactor/reactor.go` - Uses capability.Pack() not message.Pack()
- `internal/reactor/session.go` - Uses capability.Pack() not message.Pack()
- `internal/bgp/message/family.go` - Keep AFI/SAFI constants and FamilyConfigNames

## Implementation Steps

1. **Write equivalence test** - Verify PackTo produces same output as Pack
2. **Run tests** - Verify PASS (PackTo already works)
3. **Update rib/commit.go** - Replace message.Negotiated with EncodingContext
4. **Update rib tests** - Migrate all test files
5. **Remove messageNegotiated()** - Delete from reactor/peer.go
6. **Update message tests** - Replace Pack() calls with PackTo()
7. **Remove Pack()** - Delete from interface and implementations
8. **Delete message.Negotiated/Family** - Remove from message.go
9. **Verify all** - `make lint && make test && make functional`

## Implementation Summary

### What Was Implemented

1. **Removed Pack() from Message interface** (`internal/bgp/message/message.go`)
   - Deleted `Pack(neg *Negotiated) ([]byte, error)` from interface
   - Deleted `message.Negotiated` struct
   - Deleted `message.Family` struct
   - Deleted `packWithHeader()` helper

2. **Removed Pack() methods from message types**
   - `keepalive.go` - Removed Pack()
   - `open.go` - Removed Pack() and packExtended()
   - `update.go` - Removed Pack()
   - `notification.go` - Removed Pack()
   - `routerefresh.go` - Removed Pack()

3. **Migrated rib/commit.go to EncodingContext**
   - Changed `*message.Negotiated` parameter to `*bgpctx.EncodingContext`
   - Updated `addPathFor()` to use `ctx.AddPath(family)`
   - Updated `isIBGP()` to use `ctx.IsIBGP()`
   - Updated AS prepending to use `ctx.LocalASN()`

4. **Migrated test files**
   - Created `testContext()` helper in commit_test.go
   - Updated all rib tests to use EncodingContext
   - Updated message tests to use PackTo()
   - Removed Pack()-to-WriteTo equivalence tests (no longer needed)

5. **Updated cmd/zebgp/encode.go**
   - Replaced `message.Negotiated` usage with `message.PackTo()`

6. **Removed reactor/peer.go messageNegotiated()**
   - Reactor now uses `peer.SendContext()` directly

### Files Modified
- `internal/bgp/message/message.go`
- `internal/bgp/message/keepalive.go`
- `internal/bgp/message/open.go`
- `internal/bgp/message/update.go`
- `internal/bgp/message/notification.go`
- `internal/bgp/message/routerefresh.go`
- `internal/bgp/message/wirewriter_test.go`
- `internal/bgp/message/eor_test.go`
- `internal/bgp/message/update_build_evpn_test.go`
- `internal/bgp/message/update_test.go`
- `internal/bgp/message/keepalive_test.go`
- `internal/bgp/message/notification_test.go`
- `internal/bgp/message/open_test.go`
- `internal/bgp/message/routerefresh_test.go`
- `internal/bgp/message/update_split_test.go`
- `internal/rib/commit.go`
- `internal/rib/commit_test.go`
- `internal/rib/commit_edge_test.go`
- `internal/rib/commit_wire_test.go`
- `internal/reactor/peer.go`
- `internal/reactor/reactor.go`
- `cmd/zebgp/encode.go`

### Key Design Decisions
- `PackTo()` remains as convenience helper for callers that don't pre-allocate
- `writeHeader()` is the internal function for writing message headers
- EncodingContext provides all encoding info (ASN4, ADD-PATH, iBGP status)

## Checklist

### 🧪 TDD
- [x] Tests written
- [x] Tests FAIL (N/A - removal task, tests verified equivalence before removal)
- [x] Implementation complete
- [x] Tests PASS

### Verification
- [x] `make lint` passes
- [x] `make test` passes
- [x] `make functional` passes

### Documentation
- [x] Required docs read
- [x] RFC references in code (unchanged - no new protocol code)

### Completion
- [x] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] All files committed together

## Status: COMPLETED
