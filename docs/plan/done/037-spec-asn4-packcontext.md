# Spec: ASN4 in PackContext (Phase 2)

## Task
Add ASN4 field to PackContext for unified capability-aware encoding. Completes Phase 2 of negotiated packing pattern.

## Current State (verified)
- Functional tests: 28 passed, 9 failed [0, N, Q, S, T, U, V, Z, a]
- Last commit: `81b9ed9` (refactor: rename NLRIHashable.Bytes to Key)
- Phase 1 complete: Pack(ctx) pattern for NLRI encoding

## Problem

**Current:** ASN4 passed separately from PackContext:
```go
// rib/commit.go:275
return attribute.PackASPathAttribute(asPath, c.negotiated.ASN4)

// reactor/peer.go:1031
func buildRIBRouteUpdate(route *rib.Route, localAS uint32, isIBGP, asn4 bool, ctx *nlri.PackContext)
```

**Desired:** Unified context for all encoding decisions:
```go
// Single context for both NLRI and attribute encoding
ctx := peer.packContext(family)
nlriBytes := nlri.Pack(ctx)           // ADD-PATH aware
asPathBytes := asPath.Pack(ctx)       // ASN4 aware
```

## Embedded Protocol Requirements

### Default Rules (ALL tasks)
- **FIRST:** Run `git status` - if modified files exist, ASK user before proceeding
- **FIRST:** Read `docs/plan/CLAUDE_CONTINUATION.md` for current state
- Tests MUST exist and FAIL before implementation code exists
- Run `make test && make lint` before claiming done
- NEVER discard uncommitted work without explicit user permission
- Verify before claiming: run commands, paste output as proof
- Tests passing is NOT permission to commit - wait for user

### From TDD_ENFORCEMENT.md
- Write test FIRST with VALIDATES/PREVENTS documentation
- Run test → MUST FAIL (paste failure output)
- Write implementation (minimum to pass)
- Run test → MUST PASS (paste pass output)

### From CODING_STANDARDS.md
- Never ignore errors
- Context usage for cancellation
- Table-driven tests with testify
- RFC references in code comments

### From RFC 6793
- Section 4.1: NEW→NEW speakers use 4-byte AS numbers
- Section 4.2.2: NEW→OLD speakers use 2-byte with AS_TRANS (23456)
- AS4_PATH attribute carries real 4-byte path for OLD speaker traversal
- ZeBGP already implements AS_TRANS conversion in `aspath.go:169-174`

## Codebase Context

**Key files:**
| File | Role |
|------|------|
| `pkg/bgp/nlri/pack.go` | PackContext struct (add ASN4 here) |
| `pkg/reactor/peer.go:322` | `Peer.packContext()` - needs ASN4 |
| `pkg/rib/commit.go:197` | `CommitService.packContext()` - needs ASN4 |
| `pkg/bgp/attribute/aspath.go` | Already has `PackWithASN4(bool)` |

**Current PackContext:**
```go
// pkg/bgp/nlri/pack.go
type PackContext struct {
    AddPath bool  // RFC 7911
    // Future: ASN4 bool  ← ADD THIS
}
```

**Current usage pattern (to be unified):**
```go
// reactor/peer.go - asn4 passed separately
buildRIBRouteUpdate(route, localAS, isIBGP, asn4, ctx)
buildAnnounceUpdate(route, localAS, isIBGP, asn4, ctx)
buildVPLSUpdate(route, localAS, isIBGP, asn4, ctx)
```

## Implementation Steps

### Step 1: Add ASN4 to PackContext

**Test file:** `pkg/bgp/nlri/pack_test.go`

```go
// TestPackContextASN4 verifies ASN4 field exists and defaults correctly.
//
// VALIDATES: PackContext carries ASN4 capability for attribute encoding.
//
// PREVENTS: Missing ASN4 field causing compilation errors or incorrect encoding.
func TestPackContextASN4(t *testing.T) {
    // Default context - ASN4 should be false
    ctx := &PackContext{}
    assert.False(t, ctx.ASN4)

    // Explicit ASN4=true
    ctx = &PackContext{ASN4: true, AddPath: false}
    assert.True(t, ctx.ASN4)
    assert.False(t, ctx.AddPath)
}
```

**Implementation:** Add `ASN4 bool` field to PackContext struct.

### Step 2: Update Peer.packContext() to include ASN4

**Test file:** `pkg/reactor/peer_test.go` (extend existing tests)

```go
// TestPeerPackContextASN4 verifies packContext includes ASN4 from session.
//
// VALIDATES: PackContext.ASN4 reflects negotiated 4-byte AS capability.
//
// PREVENTS: ASN4 not propagating to attribute encoding.
func TestPeerPackContextASN4(t *testing.T) {
    // Setup peer with ASN4=true session
    // Verify packContext returns ASN4=true
}
```

**Implementation:** Update `Peer.packContext()` to read ASN4 from session/negotiated state.

### Step 3: Update CommitService.packContext() to include ASN4

**Test file:** `pkg/rib/commit_test.go` (extend existing tests)

```go
// TestCommitServicePackContextASN4 verifies packContext includes ASN4.
//
// VALIDATES: CommitService passes ASN4 through PackContext.
//
// PREVENTS: Attribute encoding ignoring negotiated ASN4 capability.
func TestCommitServicePackContextASN4(t *testing.T) {
    neg := &message.Negotiated{ASN4: true}
    // Verify packContext returns ctx.ASN4 == true
}
```

### Step 4: Refactor callers to use ctx.ASN4 (optional cleanup)

Once PackContext has ASN4, callers can be updated incrementally:
- Replace `buildRIBRouteUpdate(..., asn4, ctx)` with `buildRIBRouteUpdate(..., ctx)` using `ctx.ASN4`
- Same for `buildAnnounceUpdate`, `buildVPLSUpdate`, etc.

This step is optional for Phase 2 - can be done incrementally.

## Verification Checklist

- [ ] Test written for PackContext.ASN4 field
- [ ] Test shows FAIL before implementation
- [ ] PackContext struct has ASN4 bool field
- [ ] Test shows PASS after implementation
- [ ] Peer.packContext() test written and passes
- [ ] CommitService.packContext() test written and passes
- [ ] `make test` passes
- [ ] `make lint` passes
- [ ] No functional test regressions (still 28 passed, 9 failed)

## Out of Scope (Future Work)

- Refactoring all callers to use `ctx.ASN4` instead of separate parameter
- Adding ASPath.Pack(ctx *PackContext) method
- AS4_PATH generation when sending to OLD speakers (already works via PackWithASN4)
- Extended Next Hop (RFC 8950) - separate spec

## Notes

- PackContext is in `pkg/bgp/nlri/` package but used for both NLRI and attributes
- Consider moving to `pkg/bgp/encoding/` in future if more capabilities added
- ASN4 is already in `message.Negotiated` and `capability.Negotiated` structs

---

**Created:** 2025-12-28
