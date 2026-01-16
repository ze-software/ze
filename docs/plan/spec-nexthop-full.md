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

### Gaps Identified

| Gap | Severity | Description |
|-----|----------|-------------|
| ExaBGP schema | Medium | `nexthop` capability not in schema, migration fails |
| VPN encoding | Low | Uses RFC 5549 format (16 bytes), RFC 8950 requires 24 bytes with RD=0 |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - overall architecture
- [ ] `docs/architecture/wire/capabilities.md` - capability handling

### RFC Summaries
- [ ] `docs/rfc/rfc8950.md` - Extended Next Hop Encoding (primary)
- [ ] `docs/rfc/rfc5549.md` - Obsoleted predecessor (for comparison)
- [ ] `docs/rfc/rfc4760.md` - MP-BGP (MP_REACH_NLRI format)

**Key insights:**
- RFC 8950 obsoletes RFC 5549
- VPN-IPv4 with IPv6 NH changed from 16/32 bytes (no RD) to 24/48 bytes (RD=0)
- Capability Code 5, variable length (6 bytes per AFI/SAFI/NH-AFI tuple)

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestMigrateNexthopCapability` | `pkg/exabgp/migrate_test.go` | ExaBGP `nexthop;` converts to ZeBGP | |
| `TestVPNNextHopRFC8950Encoding` | `pkg/bgp/attribute/mpnlri_test.go` | VPN-IPv4 with IPv6 NH uses 24-byte format | |

### Boundary Tests

N/A - no new numeric inputs

### Functional Tests

| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| `nexthop-migrate` | `test/data/migrate/nexthop/` | ExaBGP config with nexthop capability | |

## Files to Modify

- `pkg/exabgp/schema.go` - Add `nexthop` to capability block
- `pkg/exabgp/migrate.go` - Handle `nexthop` in capability migration (already in enableFields)
- `pkg/bgp/attribute/mpnlri.go` - Update VPN encoding to RFC 8950 format (if needed)

## Files to Create

- `test/data/migrate/nexthop/input.conf` - ExaBGP config with nexthop capability
- `test/data/migrate/nexthop/expected.conf` - Expected ZeBGP output

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

<!-- Fill after implementation -->

### What Was Implemented
-

### Bugs Found/Fixed
-

### Design Insights
-

### Deviations from Plan
-

## Checklist

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)

### Verification
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make functional` passes

### Documentation
- [ ] Required docs read
- [ ] RFC summaries read
- [ ] RFC references added to code
- [ ] RFC constraint comments added

### Completion
- [ ] Architecture docs updated with learnings
- [ ] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] All files committed together
