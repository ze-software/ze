# Spec: native-update-syntax

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. Parent: `docs/plan/spec-remove-exabgp-syntax.md` - design decisions
3. `docs/architecture/api/update-syntax.md` - API syntax reference
4. `internal/plugin/bgp/message/family.go` - Ze family names

## Task

Add native `update { attribute { } nlri { } }` config syntax for route announcements.

This replaces ExaBGP's `announce { }` / `static { }` blocks with Ze-native syntax that mirrors the API.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/update-syntax.md` - API syntax (config mirrors this)
- [ ] `docs/architecture/core-design.md` - UPDATE message structure

### Source Files
- [ ] `internal/plugin/bgp/message/family.go` - Ze family names
- [ ] `internal/config/bgp.go` - config schema (peerFields)
- [ ] `internal/plugin/bgp/schema/ze-bgp.yang` - YANG schema
- [ ] `internal/plugin/update_text.go` - API text parser (reuse for config)

**Key insights:**
- Config syntax mirrors API but without `add` keyword (config = always announce)
- NLRI parsing can reuse `internal/plugin/update_text.go` logic
- Attribute block uses simple `key value;` format

## Config Syntax

### Basic Structure
```
update {
    attribute {
        origin igp;
        next-hop 10.0.0.1;
        med 100;
        local-preference 200;
        as-path [ 65001 65002 ];
        community [ 65000:1 65000:2 ];
        large-community [ 65000:0:1 ];
        extended-community [ target:65000:100 ];
    }
    nlri {
        ipv4/unicast 1.0.0.0/24 2.0.0.0/24;
        ipv6/unicast 2001:db8::/32;
    }
}
```

### Family-Specific NLRI

| Family | Syntax |
|--------|--------|
| `ipv4/unicast` | `ipv4/unicast 1.0.0.0/24;` |
| `ipv6/unicast` | `ipv6/unicast 2001:db8::/32;` |
| `ipv4/mpls` | `ipv4/mpls label 1000 10.0.0.0/24;` |
| `ipv6/mpls` | `ipv6/mpls label 2000 2001:db8::/32;` |
| `ipv4/mpls-vpn` | `ipv4/mpls-vpn rd 65000:100 label 1000 10.0.0.0/24;` |
| `ipv6/mpls-vpn` | `ipv6/mpls-vpn rd 65000:100 label 2000 2001:db8::/32;` |
| `ipv4/flowspec` | `ipv4/flowspec destination 10.0.0.0/24 protocol tcp;` |
| `ipv6/flowspec` | `ipv6/flowspec destination 2001:db8::/32 protocol tcp;` |
| `l2vpn/evpn` | `l2vpn/evpn mac-ip rd 1:1 mac 00:11:22:33:44:55 label 100;` |
| `l2vpn/vpls` | `l2vpn/vpls rd 1:1 ve-id 1 ve-block-offset 0 ve-block-size 10 label-base 1000;` |

### Multiple Updates
```
peer 10.0.0.1 {
    update {
        attribute { origin igp; next-hop 10.0.0.1; }
        nlri { ipv4/unicast 1.0.0.0/24; }
    }
    update {
        attribute { origin egp; next-hop 10.0.0.2; }
        nlri { ipv4/unicast 2.0.0.0/24; }
    }
}
```

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseUpdateBlock_Basic` | `internal/config/bgp_test.go` | Basic update block parsing | ✅ |
| `TestParseUpdateBlock_Attributes` | `internal/config/bgp_test.go` | All attribute types | ✅ |
| `TestParseUpdateBlock_MultiplePrefixes` | `internal/config/bgp_test.go` | Multiple prefixes per family | ✅ |
| `TestParseUpdateBlock_NextHopSelf` | `internal/config/bgp_test.go` | next-hop self handling | ✅ |
| `TestParseUpdateBlock_MissingNLRI` | `internal/config/bgp_test.go` | Error on missing nlri | ✅ |
| `TestParseUpdateBlock_InvalidFamily` | `internal/config/bgp_test.go` | Error on invalid family | ✅ |
| `TestParseUpdateBlock_Multiple` | `internal/config/bgp_test.go` | Multiple update blocks | ✅ |
| `TestParseUpdateBlock_InvalidMED` | `internal/config/bgp_test.go` | Error on invalid MED | ✅ |
| `TestParseUpdateBlock_IPv6` | `internal/config/bgp_test.go` | IPv6 prefix parsing | ✅ |
| `TestParseUpdateBlock_MEDBoundary` | `internal/config/bgp_test.go` | MED boundary (0, max uint32) | ✅ |
| `TestParseUpdateBlock_LocalPrefBoundary` | `internal/config/bgp_test.go` | local-pref boundary (0, max uint32) | ✅ |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `native-update-v4` | `test/encode/native-update-v4.ci` | IPv4 unicast via native syntax | Deferred* |
| `native-update-v6` | `test/encode/native-update-v6.ci` | IPv6 unicast via native syntax | Deferred* |
| `native-update-vpn` | `test/encode/native-update-vpn.ci` | VPN family via native syntax | Deferred* |

*Deferred: Config parsing produces `StaticRouteConfig` which feeds into existing route announcement pipeline. That pipeline is already tested via `simple-v4.ci`, `simple-v6.ci`. Native syntax only changes config parsing (covered by unit tests).

## Files to Modify

- `internal/plugin/bgp/schema/ze-bgp.yang` - Add update container to peer-fields
- `internal/config/bgp.go` - Add update field to peerFields(), add parsing logic
- `internal/config/bgp_test.go` - Add unit tests

## Files to Create

- `test/encode/native-update-v4.ci` - Functional test for IPv4
- `test/encode/native-update-v6.ci` - Functional test for IPv6

## Implementation Steps

1. **Write unit tests** - Tests for update block parsing
   → **Review:** Edge cases covered?

2. **Run tests** - Verify FAIL (paste output)
   → **Review:** Failing for right reason?

3. **Update YANG** - Add update container to peer-fields grouping
   → **Review:** Matches design?

4. **Update bgp.go schema** - Add update field to peerFields()
   → **Review:** Consistent with existing patterns?

5. **Implement parsing** - Parse update blocks into StaticRouteConfig
   → **Review:** Reuses existing NLRI parsing?

6. **Run tests** - Verify PASS (paste output)
   → **Review:** All tests pass?

7. **Add functional tests** - End-to-end tests
   → **Review:** Cover user scenarios?

8. **Verify all** - `make lint && make test && make functional` (paste output)
   → **Review:** Clean?

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| No `add` keyword | Config = always announce, unlike API which has add/del |
| Reuse NLRI parsing | API parser already handles all families |
| `attribute` block | Separates attrs from NLRI cleanly |
| Multiple `update` blocks | Different routes can have different attributes |

## Checklist

### 🧪 TDD
- [x] Tests written (11 unit tests)
- [x] Tests FAIL (verified before implementation)
- [x] Implementation complete
- [x] Tests PASS

### Verification
- [x] `make lint` passes (0 issues)
- [x] `make test` passes
- [x] `make functional` passes

### Documentation
- [x] Required docs read
- [x] Config syntax documented in this spec (mirrors API syntax)

### Future Work
- [x] Add missing attributes (path-information, labels, etc.)
- [x] Refactor to eliminate code duplication with parseRouteConfig
- [ ] Support inline VPN syntax in nlri block (e.g., `ipv4/mpls-vpn rd 1:1 label 100 10.0.0.0/24`)

## Refactoring: Shared Attribute Parsing

Extract `applyAttributesFromTree(*Tree, *StaticRouteConfig) error` to eliminate duplication between:
- `extractRoutesFromUpdateBlock()` - new update syntax
- `parseRouteConfig()` - existing announce/static syntax

## Known Limitations

### Attributes Now Fully Supported

All attributes from `announce { }` / `static { }` are now supported in `update { }`:

| Attribute | Description | Status |
|-----------|-------------|--------|
| `path-information` | ADD-PATH path-id | ✅ |
| `labels` | Multi-label stack (RFC 8277) | ✅ |
| `attribute` | Generic hex attributes | ✅ |
| `bgp-prefix-sid` | Prefix-SID (label index) | ✅ |
| `bgp-prefix-sid-srv6` | SRv6 Prefix-SID | ✅ |
| `split` | Prefix splitting | ✅ |
| `watchdog` | Route watchdog | ✅ |
| `withdraw` | Withdraw flag | ✅ |
| `name` | Route name | ❌ (not needed - update blocks are anonymous) |

### VPN Family Syntax Not Supported

The inline VPN syntax is not yet implemented:
```
nlri {
    ipv4/mpls-vpn rd 65000:100 label 1000 10.0.0.0/24;  # NOT YET
}
```

Workaround: Use `rd` and `label` in attribute block:
```
update {
    attribute {
        rd 65000:100;
        label 1000;
        next-hop 10.0.0.1;
    }
    nlri {
        ipv4/mpls-vpn 10.0.0.0/24;
    }
}
```

### Code Duplication (FIXED)

Refactored: Shared `applyAttributesFromTree(*Tree, *StaticRouteConfig)` helper now used by both:
- `extractRoutesFromUpdateBlock()` - update syntax
- `parseRouteConfig()` - announce/static syntax

## Implementation Summary

### What Was Implemented

- YANG schema: Added `list update` with `attribute` and `nlri` containers
- Go parsing: Added `extractRoutesFromUpdateBlock()` function
- Refactored: `applyAttributesFromTree()` shared between old/new syntax
- Tests: 11 unit tests covering basic usage, attributes, IPv6, boundaries, errors

### Files Modified

| File | Change |
|------|--------|
| `internal/plugin/bgp/schema/ze-bgp.yang` | +29 lines: `list update` definition |
| `internal/config/bgp.go` | +130 lines: parsing logic, `configSelf` constant |
| `internal/config/bgp_test.go` | +380 lines: 11 test functions (IPv6, boundary tests added) |

### Verification

```
make test && make lint && make functional
✅ All tests pass
✅ 0 lint issues
✅ All functional tests pass
```
