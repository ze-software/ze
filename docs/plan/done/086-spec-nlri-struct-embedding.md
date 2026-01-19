# Spec: nlri-struct-embedding

## Task
Refactor NLRI types to use struct embedding for architectural clarity:
1. Prefix-based types (INET, LabeledUnicast) - share family, prefix, pathID
2. RD-based types (MVPN, MUP) - share rd, data, cached fields and buildData pattern

**Primary goal:** Idiomatic Go - explicit type relationships over implicit duplication.
**Secondary goal:** ~30 LOC reduction.

Note: IPVPN has different field order (rd before prefix) so stays separate.

## Required Reading
- [ ] `.claude/zebgp/wire/NLRI.md` - NLRI wire formats, class hierarchy
- [ ] `.claude/zebgp/UPDATE_BUILDING.md` - Build path uses *Params structs, forward path needs zero-copy
- [ ] `.claude/zebgp/ENCODING_CONTEXT.md` - Zero-copy forwarding depends on context matching
- [ ] `.claude/zebgp/edge-cases/ADDPATH.md` - PathID handling separate from payload encoding

**Key insights:**
- NLRI interface methods return payload-only (no path ID); `WriteNLRI()` handles ADD-PATH
- Zero-copy forwarding is critical for scale; refactor must not break `CanForwardDirect()` pattern
- `EncodeLabelStack()` and `writeLabelStack()` already exist in ipvpn.go - reuse these
- `encodeLabel()` in labeled.go is redundant - should use existing helpers
- `hasRD()` helper already exists in other.go:883
- Prefix calculation `(bits + 7) / 8` duplicated in INET, LabeledUnicast, IPVPN
- IPVPN field order: family, rd, labels, prefix, pathID (rd BEFORE prefix - can't embed PrefixNLRI)

## 🧪 TDD Test Plan

### New Unit Tests
| Test | File | Validates |
|------|------|-----------|
| `TestPrefixBytes` | `internal/bgp/nlri/helpers_test.go` | `PrefixBytes(bits)` returns correct byte count |
| `TestWriteLabelStack` | `internal/bgp/nlri/helpers_test.go` | `WriteLabelStack()` encodes labels with BOS |
| `TestWriteLabelStackOffset` | `internal/bgp/nlri/helpers_test.go` | `WriteLabelStack()` respects buffer offset |
| `TestRDNLRIBaseBuildData` | `internal/bgp/nlri/base_test.go` | `buildData()` returns rd+data or data only |
| `TestRDNLRIBaseBuildDataNoAlias` | `internal/bgp/nlri/base_test.go` | `buildData()` returns copy, no aliasing |

### Existing Tests (Regression)
| File | Purpose |
|------|---------|
| `inet_test.go` | INET wire format unchanged after embedding |
| `labeled_test.go` | LabeledUnicast wire format unchanged |
| `ipvpn_test.go` | IPVPN wire format unchanged |
| `other_test.go` | MVPN/MUP wire format unchanged after embedding |
| `wire_format_test.go` | Cross-type wire format validation |
| `pack_test.go` | Pack() behavior unchanged |
| `writeto_test.go` | WriteTo() behavior unchanged |

### Functional Tests
| Test | Location | Scenario |
|------|----------|----------|
| All existing | `qa/tests/` | Full suite must pass - no behavioral changes |

## Files to Modify
- `internal/bgp/nlri/helpers.go` - NEW: `PrefixBytes()`, move `WriteLabelStack()` from ipvpn.go
- `internal/bgp/nlri/base.go` - NEW: `PrefixNLRI` and `RDNLRIBase` embedded types
- `internal/bgp/nlri/inet.go` - Embed `PrefixNLRI`, update `NewINET`, `ParseINET`
- `internal/bgp/nlri/labeled.go` - Embed `PrefixNLRI`, update constructor/parser, delete `encodeLabel()`
- `internal/bgp/nlri/ipvpn.go` - Use `PrefixBytes()`, export `WriteLabelStack()` to helpers.go
- `internal/bgp/nlri/other.go` - Embed `RDNLRIBase` in MVPN/MUP, update parsers

## Implementation Steps

### Phase 1: Extract Helpers
1. **Write test** - Create `helpers_test.go` with `TestPrefixBytes`, `TestWriteLabelStack`
2. **Run tests** - Verify FAIL (paste output)
3. **Implement** - Create `helpers.go`:
   ```go
   // PrefixBytes returns bytes needed for a prefix of given bit length.
   func PrefixBytes(bits int) int { return (bits + 7) / 8 }

   // WriteLabelStack writes MPLS labels to buf at offset. Returns bytes written.
   // Moved from ipvpn.go for reuse by labeled.go.
   func WriteLabelStack(buf []byte, off int, labels []uint32) int
   ```
4. **Run tests** - Verify PASS (paste output)
5. **Update callers** - Replace `(x.Bits() + 7) / 8` in inet.go, labeled.go, ipvpn.go
6. **Move writeLabelStack** - From ipvpn.go to helpers.go, rename to `WriteLabelStack`

### Phase 2: PrefixNLRI Base Type (INET + LabeledUnicast only)
1. **Run existing tests** - Baseline (paste output)
2. **Implement PrefixNLRI** - Create `base.go`:
   ```go
   type PrefixNLRI struct {
       family Family
       prefix netip.Prefix
       pathID uint32
   }
   func (p *PrefixNLRI) Family() Family { return p.family }
   func (p *PrefixNLRI) Prefix() netip.Prefix { return p.prefix }
   func (p *PrefixNLRI) PathID() uint32 { return p.pathID }
   ```
3. **Refactor INET** - Embed PrefixNLRI, update:
   - `NewINET()` constructor
   - `ParseINET()` parser (struct literal syntax changes)
4. **Refactor LabeledUnicast** - Embed PrefixNLRI, update:
   - `NewLabeledUnicast()` constructor
   - Parser if exists
   - Note: field order becomes family, prefix, pathID, labels (pathID moves before labels)
5. **Note: IPVPN stays separate** - Field order (rd before prefix) incompatible
6. **Run tests** - Verify PASS (paste output)

### Phase 3: Consolidate Label Encoding
1. **Delete `encodeLabel()`** - In labeled.go:90-101
2. **Update LabeledUnicast.Bytes()** - Use `EncodeLabelStack()` (stays in ipvpn.go, returns []byte)
3. **Update LabeledUnicast.WriteTo()** - Use `WriteLabelStack()` from helpers.go (zero-alloc)
4. **Run tests** - Verify PASS (paste output)

### Phase 4: RDNLRIBase for MVPN/MUP
1. **Run existing tests** - Baseline (paste output)
2. **Implement RDNLRIBase** - Add to `base.go`:
   ```go
   type RDNLRIBase struct {
       rd     RouteDistinguisher
       data   []byte
       cached []byte
   }
   func (r *RDNLRIBase) RD() RouteDistinguisher { return r.rd }
   // buildData returns rd+data or data only. ALLOCATES - use only in Bytes().
   func (r *RDNLRIBase) buildData() []byte {
       if hasRD(r.rd) { return append(r.rd.Bytes(), r.data...) }
       return r.data
   }
   ```
3. **Refactor MVPN** - Embed RDNLRIBase, update:
   - `ParseMVPN()` parser
   - `Bytes()` to use `buildData()`
   - `WriteTo()` keeps direct buffer writes (zero-alloc)
4. **Refactor MUP** - Embed RDNLRIBase, update:
   - `ParseMUP()` parser
   - `Bytes()` to use `buildData()`
   - `WriteTo()` keeps direct buffer writes (zero-alloc)
5. **Run tests** - Verify PASS (paste output)

### Phase 5: Final Verification
1. **Run full suite** - `make lint && make test && make functional`
2. **Verify no behavioral changes** - All tests pass with same output

## Design Decisions

### Embedding vs Interface
- **Decision:** Use struct embedding, not interfaces
- **Rationale:** Go embedding promotes fields/methods automatically; interfaces would require explicit delegation

### PrefixNLRI Scope
- **Decision:** Embed in INET and LabeledUnicast only, NOT IPVPN
- **Rationale:** IPVPN field order is `family, rd, labels, prefix, pathID` - rd comes before prefix, incompatible with PrefixNLRI embedding

### Reuse Existing Label Helpers
- **Decision:** Use existing `EncodeLabelStack()` and `writeLabelStack()` from ipvpn.go
- **Rationale:** Already implemented correctly; delete redundant `encodeLabel()` in labeled.go

### RDNLRIBase Scope
- **Decision:** For MVPN, MUP only
- **Rationale:** Share rd/data/cached fields + buildData() pattern; VPLS/RTC have different structures

### Zero-Copy Preservation
- **Decision:** No changes to `Pack()`, `WriteTo()`, `Bytes()` semantics
- **Rationale:** Forward path depends on existing wire cache behavior

### WriteTo() Must Stay Zero-Alloc
- **Decision:** `buildData()` only used in `Bytes()`, not `WriteTo()`
- **Rationale:** `WriteTo()` writes directly to buffer without allocation; `buildData()` allocates via `append()`. Keep `WriteTo()` implementations writing directly.

## RFC Documentation
- RFC 4271 Section 4.3 - NLRI format (prefix encoding)
- RFC 3107 - MPLS label encoding in BGP
- RFC 4364 - VPN NLRI with RD
- RFC 7911 - ADD-PATH (path ID separate from payload)

Existing RFC comments in code are sufficient; no new RFCs to download.

## Additional Improvements (discovered during review)

### Thread Safety
- Added `sync.Once` to `RDNLRIBase` for thread-safe `Bytes()` lazy initialization
- MVPN/MUP `Bytes()` methods now use `cacheOnce.Do()` to prevent race conditions

### Zero-Copy Optimization
- Removed unnecessary `make()+copy()` in `ParseMVPN()` and `ParseMUP()`
- Both `cached` and `data` fields are now zero-copy slices of the original wire buffer
- Consistent zero-copy design throughout

### Slice Aliasing Fix
- `buildData()` now returns a copy when no RD (prevents caller mutation affecting original)
- Test `TestRDNLRIBaseBuildDataNoAlias` validates this behavior

## Checklist

### 🧪 TDD
- [x] Tests written (helpers_test.go, base_test.go)
- [x] Tests FAIL before implementation
- [x] Implementation complete
- [x] Tests PASS after implementation

### Verification
- [x] `make lint` passes (pre-existing deprecation warnings only)
- [x] `make test` passes
- [x] `make functional` passes (37 tests)
- [x] `go test -race` passes (no race conditions)

### Documentation
- [x] Required docs read
- [x] RFC references added (existing refs sufficient)
- [x] `.claude/zebgp/wire/NLRI.md` updated with embedding hierarchy

### Completion
- [ ] Spec moved to `docs/plan/done/NNN-nlri-struct-embedding.md`

## Appendix: Identified Duplication

### A. Prefix Byte Calculation (3 locations)
```go
// inet.go:169, 181-182
prefixBytes := (prefixLen + 7) / 8

// labeled.go:111, 146, 161
prefixBytes := (prefixBits + 7) / 8

// ipvpn.go:424, 441, 467
prefixBytes := (prefixBits + 7) / 8
```

### B. Label Encoding (redundant function)
```go
// labeled.go:90-101 - encodeLabel() returns []byte
// ipvpn.go:231-243 - EncodeLabelStack() returns []byte  <- KEEP
// ipvpn.go:507-518 - writeLabelStack() writes to buffer <- KEEP
// labeled.go:174-182 - inline in WriteTo (same as writeLabelStack)
```
**Action:** Delete `encodeLabel()`, have labeled.go use existing helpers.

### C. RD + Data Building (2 locations)
```go
// MVPN.Bytes() lines 230-235 and MUP.Bytes() lines 838-843:
var totalData []byte
if m.rd.Type != 0 || m.rd.Value != [6]byte{} {
    totalData = append(m.rd.Bytes(), m.data...)
} else {
    totalData = m.data
}
```
**Action:** Extract to `RDNLRIBase.buildData()` method.

### D. PathID Methods (3 locations in scope)
```go
// Identical in: INET, LabeledUnicast (can share via PrefixNLRI)
func (x *Type) PathID() uint32 { return x.pathID }
```

## Appendix: Proposed Hierarchy

```
NLRI (interface)
├── PrefixNLRI [embed: family, prefix, pathID]
│   ├── INET
│   └── LabeledUnicast [+labels]
├── IPVPN [standalone - field order: family, rd, labels, prefix, pathID]
├── RDNLRIBase [embed: rd, data, cached]
│   ├── MVPN [+afi, +routeType]
│   └── MUP [+afi, +archType, +routeType]
└── Standalone (EVPNType1-5, BGP-LS, FlowSpec, VPLS, RTC)
```

**Estimated impact:** ~30 LOC reduction + architectural clarity
- PrefixBytes helper: ~0 LOC (same call length, better readability)
- Delete encodeLabel(): 12 LOC saved
- PrefixNLRI embedding: ~6 LOC net (saves fields/methods, adds base.go)
- RDNLRIBase embedding: ~10 LOC net (saves duplication, adds base.go)

**Primary value:** Explicit type relationships - idiomatic Go over implicit duplication.
