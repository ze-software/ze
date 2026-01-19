# Spec: nlri-writeto-zero-alloc

## Task

Fix non-zero-alloc WriteTo in NLRI types. Currently several NLRI types allocate in WriteTo by calling Pack()/Bytes() internally.

## Required Reading (MUST complete before implementation)

- [x] `.claude/zebgp/wire/NLRI.md` - NLRI wire formats and ADD-PATH handling
- [x] `.claude/zebgp/wire/BUFFER_WRITER.md` - Zero-alloc architecture

**Key insights from docs:**
- NLRI types should implement `WriteTo(buf, off, ctx) int` for zero-allocation
- ADD-PATH adds/removes 4-byte path ID based on context
- Some types cache wire bytes in `cached`/`data` fields

## Current State (After Implementation)

| NLRI Type | WriteTo Implementation | Allocates? | Has Cached Bytes? |
|-----------|----------------------|------------|-------------------|
| INET | Direct buffer write | ❌ No | N/A - computes |
| IPVPN | Direct buffer write | ❌ No | N/A - computes |
| LabeledUnicast | Direct buffer write | ❌ No | N/A - computes |
| EVPN (all 6 types) | `copy(buf, Pack(ctx))` | ✅ Yes | No (needs separate fix) |
| FlowSpec | Direct buffer write | ❌ No* | Fallback to cached |
| FlowSpecVPN | Direct buffer write | ❌ No* | Fallback to cached |
| BGPLS (4 types) | Direct buffer write | ❌ No* | Fallback to cached |
| MVPN/VPLS/RTC/MUP | Direct buffer write | ❌ No* | Fallback to cached |

*Uses fallback to cached bytes if components not populated (e.g., parsed NLRIs)*

## Files to Modify

### Phase 1: Types with cached bytes (low effort)
- `internal/bgp/nlri/flowspec.go` - Copy from `cached` field directly
- `internal/bgp/nlri/bgpls.go` - Copy from `cached` field directly
- `internal/bgp/nlri/other.go` - Copy from `data` field directly (MVPN/VPLS/RTC/MUP)

### Phase 2: LabeledUnicast (medium effort)
- `internal/bgp/nlri/labeled.go` - Implement direct buffer write like INET/IPVPN

### Phase 3: EVPN (higher effort - has pre-existing bug)
- `internal/bgp/nlri/evpn.go` - Fix ADD-PATH bug AND implement zero-alloc WriteTo

## Pre-existing Bug: EVPN ADD-PATH

**Critical:** EVPN types have broken ADD-PATH handling discovered in critical review:
- `Bytes()` does NOT include path ID
- `packEVPN()` assumes `Bytes()` includes path ID when `hasPath=true`
- Result: Path ID is lost in wire format

This needs investigation and fixing before/during EVPN WriteTo implementation.

## Open Questions (investigate when work picked)

1. **EVPN scope**: Should EVPN ADD-PATH bug fix be in this PR or separate?
2. **Priority**: Implement all types or just most common (LabeledUnicast)?
3. **Cached types**: For FlowSpec/BGPLS, is copying from cache sufficient or should we avoid even the cache allocation?

## Implementation Steps

### Phase 1: Cached bytes types
1. Write test verifying WriteTo matches Pack output
2. Modify WriteTo to copy from cached/data field directly
3. Ensure Bytes() is called first to populate cache if needed
4. Run `make test && make lint`

### Phase 2: LabeledUnicast
1. Write test for WriteTo with all ADD-PATH combinations
2. Implement direct buffer write following INET pattern
3. Handle label stack encoding
4. Run `make test && make lint`

### Phase 3: EVPN (if in scope)
1. First fix ADD-PATH bug in Bytes()/Pack()
2. Add tests for ADD-PATH scenarios
3. Implement zero-alloc WriteTo
4. Run `make test && make lint`

## Checklist

- [x] Required docs read
- [x] Phase 1: Test fails first → passes (FlowSpec, BGPLS, MVPN/VPLS/RTC/MUP)
- [x] Phase 2: LabeledUnicast already zero-alloc (confirmed during investigation)
- [ ] Phase 3: EVPN skipped (separate PR for ADD-PATH bug fix)
- [x] make test passes
- [x] make lint passes (no new issues in nlri package)
- [x] Update `.claude/zebgp/wire/NLRI.md` - no update needed, docs already accurate

## Implementation Notes

**Decision: Full zero-alloc WriteTo with fallback**
- User requested full zero-alloc (not just copy from cache)
- Added `Len()` and `WriteTo()` methods to FlowComponent, descriptors
- WriteTo writes directly to buffer without allocating
- Fallback: if cached bytes exist but components empty, use cached (for parsed NLRIs)

**Files modified:**
- `internal/bgp/nlri/flowspec.go` - FlowSpec/FlowSpecVPN zero-alloc WriteTo
- `internal/bgp/nlri/bgpls.go` - BGPLS types zero-alloc WriteTo + descriptor WriteTo
- `internal/bgp/nlri/other.go` - MVPN/VPLS/RTC/MUP zero-alloc WriteTo
- `internal/bgp/nlri/ipvpn.go` - RouteDistinguisher.WriteTo helper
- `internal/bgp/nlri/writeto_test.go` - New tests verifying WriteTo matches Bytes()
