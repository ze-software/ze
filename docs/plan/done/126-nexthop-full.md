# Spec: Full Extended Next Hop Support (RFC 8950)

## Task

Complete RFC 8950 Extended Next Hop support:
1. Add `nexthop` capability to ExaBGP schema for migration
2. Verify/fix VPN-IPv4 encoding to use RFC 8950 format (RD=0 prefix)

## Current Status

### Already Implemented ✅

| Component | Status | Location |
|-----------|--------|----------|
| Capability parsing (Code 5) | ✅ | `pkg/bgp/capability/capability.go:563-627` |
| Capability negotiation | ✅ | `pkg/bgp/capability/negotiated.go:117-163, 280-295` |
| EncodingCaps field | ✅ | `pkg/bgp/capability/encoding.go:22-24, 47-55` |
| ZeBGP config schema | ✅ | `pkg/config/bgp.go:207, 223, 498, 558-566` |
| Config parsing | ✅ | `pkg/config/bgp.go:1023-1031, 1682-1717` |
| Capability building | ✅ | `pkg/config/loader.go:231-245` |
| MP_REACH_NLRI IPv6 NH parsing | ✅ | `pkg/bgp/attribute/mpnlri.go:273-295` |
| MP_REACH_NLRI IPv6 NH encoding | ✅ | `pkg/bgp/attribute/mpnlri.go:104-114, 117-150` |
| Functional tests | ✅ | `test/data/encode/extended-nexthop.*` |

### Gaps Identified (All Resolved ✅)

| Gap | Severity | Description | Resolution |
|-----|----------|-------------|------------|
| ExaBGP schema | Medium | `nexthop` capability not in schema | ✅ Added to schema + capability inference from block |
| VPN encoding | Low | Uses RFC 5549 format (16 bytes)? | ✅ Already RFC 8950 compliant (24 bytes in `buildMPReachVPN`) |

## Required Reading

### Architecture Docs
- [x] `docs/architecture/core-design.md` - overall architecture
- [x] `docs/architecture/wire/capabilities.md` - capability handling

### RFC Summaries
- [x] `docs/rfc/rfc8950.md` - Extended Next Hop Encoding (primary)
- [x] `docs/rfc/rfc5549.md` - Obsoleted predecessor (for comparison)
- [x] `docs/rfc/rfc4760.md` - MP-BGP (MP_REACH_NLRI format)

**Key insights:**
- RFC 8950 obsoletes RFC 5549
- VPN-IPv4 with IPv6 NH changed from 16/32 bytes (no RD) to 24/48 bytes (RD=0)
- Capability Code 5, variable length (6 bytes per AFI/SAFI/NH-AFI tuple)

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestMigrateNexthopCapability` | `pkg/exabgp/migrate_test.go` | Capability inferred from `nexthop { }` block | ✅ |
| `TestMigrateNexthopExplicitAndBlock` | `pkg/exabgp/migrate_test.go` | Both explicit cap + block without duplication | ✅ |
| `TestMigrateNexthopBlock` | `pkg/exabgp/migrate_test.go` | ExaBGP `nexthop { ... }` block converts | ✅ |
| `TestMigrateNexthopBlockSAFINormalization` | `pkg/exabgp/migrate_test.go` | SAFI names normalized | ✅ |
| `TestMigrateTemplateWithNexthop` | `pkg/exabgp/migrate_test.go` | Template nexthop blocks converted | ✅ |
| `TestParseMPReachNLRI_VPNWithIPv6NextHop` | `pkg/bgp/attribute/mpnlri_test.go` | VPN with 24-byte IPv6 NH (RFC 4659/8950) | ✅ |

### Boundary Tests

N/A - no new numeric inputs

### Functional Tests

| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| `nexthop-migrate` | `test/data/migrate/nexthop/` | ExaBGP config with nexthop capability + block | ✅ |

## Files to Modify

- `pkg/exabgp/schema.go:62` - Added `nexthop` to capability block
- `pkg/exabgp/migrate.go:227-235` - Added capability inference from `nexthop { }` block
- `pkg/exabgp/migrate_test.go` - Added `TestMigrateNexthopCapability`, `TestMigrateNexthopExplicitAndBlock`, updated `TestMigrateNexthopBlock`

## Files to Create

- `test/data/migrate/nexthop/input.conf` - ExaBGP config with nexthop block
- `test/data/migrate/nexthop/expected.conf` - Expected ZeBGP output with inferred capability

## Implementation Steps

### Phase 1: ExaBGP Schema (migration support)

1. **Add nexthop to schema**
   - Add `config.Field("nexthop", config.Flex())` to capability block in `schema.go`

2. **Write migration test** - Verify ExaBGP `nexthop;` converts correctly
   ```go
   func TestMigrateNexthopCapability(t *testing.T) {
       input := `neighbor 10.0.0.1 {
           local-as 65001;
           peer-as 65002;
           capability {
               nexthop;
           }
       }`
       // Should produce: capability { nexthop enable; }
   }
   ```

3. **Run test** - Verify FAIL (paste output)

4. **Implement** - Schema change should make it work (migrate.go already handles it)

5. **Run test** - Verify PASS (paste output)

6. **Add functional test** - `test/data/migrate/nexthop/`

### Phase 2: VPN RFC 8950 Encoding (if needed)

1. **Verify current behavior**
   - Check if VPN-IPv4 with IPv6 NH encodes as 16 bytes (RFC 5549) or 24 bytes (RFC 8950)
   - Run existing `extended-nexthop` test and inspect wire bytes

2. **If RFC 5549 format detected:**
   - Write test for RFC 8950 format (24 bytes with RD=0)
   - Update `nextHopLen()` and `Pack()` in `mpnlri.go`
   - Verify test passes

3. **If RFC 8950 format already used:**
   - Document in code, no changes needed

### Phase 3: Verification

1. **Run all tests**
   ```bash
   make test && make lint && make functional
   ```

2. **RFC refs** - Add RFC 8950 comments where missing

## RFC Documentation

### Reference Comments
- `// RFC 8950 Section 4` for capability handling
- `// RFC 8950 Section 3` for next-hop encoding

### Constraint Comments

```go
// RFC 8950 Section 3: "VPN-IPv4 NLRI with IPv6 next-hop uses 24 or 48 bytes"
// Format: RD (8 bytes, all zeros) + IPv6 address (16 bytes)
// This replaces RFC 5549's 16/32 byte format without RD prefix.
```

## Implementation Summary

### What Was Implemented
- Added `nexthop` capability to ExaBGP schema (`pkg/exabgp/schema.go:62`)
- Added `nexthop` block to ExaBGP schema at peer level (`pkg/exabgp/schema.go:98`)
- Added `convertNexthopBlock()` and `convertNexthopSyntax()` for syntax conversion
- Added `normalizeSAFI()` for ExaBGP→ZeBGP SAFI name conversion
- Added serialization for nexthop block in `SerializeTree()`
- **Updated `migrateCapability()` to infer nexthop capability from `nexthop { }` block presence** (RFC 8950)
- Unit tests: `TestMigrateNexthopCapability`, `TestMigrateNexthopExplicitAndBlock`, `TestMigrateNexthopBlock`, `TestMigrateNexthopBlockSAFINormalization`, `TestMigrateTemplateWithNexthop`
- Added `TestParseMPReachNLRI_VPNWithIPv6NextHop` for RFC 4659/8950 VPN parsing (table-driven)
- Updated RFC comments in `mpnlri.go` (RFC 5549→8950)
- Fixed buggy case 40→case 48 in `parseVPNNextHops()`
- Functional test `test/data/migrate/nexthop/` with input.conf and expected.conf

### Bugs Found/Fixed
- **Critical (found during review):** Original code had `case 40` but 8+16+8+16=48, not 40. Fixed to `case 48`.
- **Medium:** RFC comments referenced obsolete RFC 5549 format for VPN encoding. Updated to RFC 8950.
- **Capability inference:** ExaBGP supports both explicit `capability { nexthop; }` AND auto-detection from `nexthop { }` block. Updated migration to handle both without duplication.

### ExaBGP Verification
Verified against ExaBGP source (`configuration/neighbor/__init__.py`):
```python
if neighbor.capability.nexthop.is_unset() and nexthop:
    neighbor.capability.nexthop = TriState.TRUE
```
- ExaBGP supports explicit `capability { nexthop true/false; }`
- If unset, auto-detects from `nexthop { }` block presence
- Our implementation correctly handles both cases

### Design Insights
- **VPN-IPv4 encoding is already RFC 8950 compliant.** The `buildMPReachVPN` in `update_build.go:626-632` uses 24 bytes (RD=0 + IPv6) for IPv6 next-hop.
- The generic `MPReachNLRI.Pack()` doesn't handle SAFI-specific encoding, but VPN routes use the specialized `buildMPReachVPN` function which is correct.
- **Parsing handles both formats** for backwards compatibility:
  - RFC 5549: 16/32 bytes (legacy, some implementations still use this)
  - RFC 8950: 24/48 bytes (current standard with RD=0 prefix)
- ExaBGP has TWO nexthop-related configs:
  1. `capability { nexthop; }` - enables the capability
  2. `nexthop { ipv4 unicast ipv6; }` - configures AFI/SAFI tuples

### Deviations from Plan
- Phase 2 (VPN encoding fix) was not needed - investigation showed encoding is already correct.
- Additional work: nexthop block schema + conversion (found during critical review).

## Checklist

### 🧪 TDD
- [x] Tests written
- [x] Tests FAIL: `parse: line 6: unknown field in capability: nexthop`
- [x] Implementation complete
- [x] Tests PASS

### Verification
- [x] `make lint` passes (0 issues)
- [x] `make test` passes
- [x] `make functional` passes

### Documentation
- [x] Required docs read
- [x] RFC summaries read (rfc8950.md)
- [x] RFC references added to code (N/A - no new RFC code)
- [x] RFC constraint comments added (N/A - schema only)

### Completion
- [x] Architecture docs updated with learnings (N/A - no new patterns)
- [x] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] All files committed together
