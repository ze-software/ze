# Spec: pack-removal

## Task

Remove `PackContext` and `Pack()` method from NLRI types. These were duplicating functionality now provided by `EncodingContext` and `WriteTo()`.

## Required Reading

### Architecture Docs
- [x] `docs/architecture/core-design.md` - NLRI encoding architecture
- [x] `docs/architecture/encoding-context.md` - EncodingContext design

## Rationale

`PackContext` was a simplified struct containing:
- `AddPath bool` - whether to include path-id
- `ASN4 bool` - 4-byte ASN support (unused by NLRI)

This duplicated `EncodingContext` which already provides:
- `AddPath(family Family) bool` - per-family ADD-PATH status
- `ASN4() bool` - ASN encoding mode

The `Pack(ctx *PackContext) []byte` method allocated new slices, while `WriteTo(buf, off)` writes to pre-allocated buffers (zero-allocation).

## Implementation Summary

### What Was Removed
- `pkg/bgp/nlri/pack.go` - PackContext type definition
- `pkg/bgp/nlri/pack_test.go` - PackContext tests
- `Pack(ctx *PackContext) []byte` method from NLRI interface
- `ToPackContext(family Family) *PackContext` from EncodingContext
- `ctx *PackContext` parameter from `WriteTo()` methods

### What Was Changed
- `WriteTo(buf, off, ctx)` → `WriteTo(buf, off)` across all NLRI types
- `LenWithContext(n, ctx)` → `LenWithContext(n, addPath bool)`
- `WriteNLRI(n, buf, off, ctx)` → `WriteNLRI(n, buf, off, addPath bool)`

### Bugs Found/Fixed
1. **WriteNLRI inconsistency**: Was adding path-id for all NLRI types when `addPath=true`, but FlowSpec/BGPLS don't support ADD-PATH. Fixed by adding `supportsAddPath(n)` check.
2. **TestWireNLRI_Len**: Expected `Len()` to return full data length (8) but interface contract says payload only (4). Fixed test expectation.
3. **TestBuildGroupedUnicast_WithAddPath**: Had `AddPath=false` but expected ADD-PATH behavior. Fixed parameter.
4. **Stale test names**: `TestPeerPackContext*` renamed to `TestPeerAddPath*`
5. **Stale assertions**: Removed "should return non-nil context" (bool can't be nil)

### Files Modified
| Category | Files | Changes |
|----------|-------|---------|
| Deleted | `pack.go`, `pack_test.go` | -51 lines |
| NLRI types | `inet.go`, `ipvpn.go`, `labeled.go`, `evpn.go`, `bgpls.go`, `flowspec.go`, `other.go`, `wire.go` | Remove Pack(), update WriteTo() |
| Interface | `nlri.go` | Remove Pack from interface, fix WriteNLRI |
| Context | `context.go`, `context_test.go` | Remove ToPackContext |
| Message | `update_build.go`, `update_build_test.go` | Use addPath bool |
| Reactor | `peer.go`, `reactor.go`, `session.go` | Use EncodingContext.AddPath() |
| RIB | `commit.go`, `update.go`, `route.go` | Use addPath bool |
| Tests | 15+ test files | Update to new signatures |

## Checklist

### TDD
- [x] Tests written
- [x] Tests FAIL (before fixes)
- [x] Implementation complete
- [x] Tests PASS

### Verification
- [x] `make lint` passes (0 issues)
- [x] `make test` passes
- [x] `make functional` passes (83 tests)

### Documentation
- [x] Spec created (retrospective)

## Statistics

- 44 files changed
- 718 insertions, 1204 deletions
- Net: -486 lines
