# Family-Specific Keyword Validation

**Status:** ✅ COMPLETE (2023-12-23)

## Problem

Current implementation accepts all keywords for all families. Invalid combinations are silently ignored:
```
announce ipv4/unicast 10.0.0.0/24 rd 100:100 next-hop 1.2.3.4
```
→ `rd` silently ignored (should error: "rd not valid for ipv4/unicast")

## Goal

- Family-specific keyword validation (whitelist per family)
- Shared attribute parsing code (DRY)
- Clear error messages for invalid keywords

## Implementation Summary

### Files Created/Modified

| File | Changes |
|------|---------|
| `pkg/api/route_keywords.go` | `UnicastKeywords`, `MPLSKeywords`, `VPNKeywords` |
| `pkg/api/route.go` | `parseRouteAttributes()`, `parseLabeledUnicastAttributes()`, `parseL3VPNAttributes()`, `parseCommonAttribute()` |
| `pkg/api/route_parse_test.go` | Keyword validation tests |
| `pkg/api/handler_test.go` | Handler keyword rejection tests (40+ tests) |

### Keyword Sets Implemented

```go
// UnicastKeywords (IPv4/IPv6 unicast)
next-hop, origin, med, local-preference, as-path, community, large-community, split

// MPLSKeywords (SAFI 4 labeled-unicast)
UnicastKeywords + label

// VPNKeywords (SAFI 128 L3VPN)
UnicastKeywords + rd, rt, label (no split)
```

### Architecture

```
parseCommonAttribute()     ← shared: origin, med, local-pref, as-path, community, large-community
        ↑
parseRouteAttributes()     ← unicast (validates against UnicastKeywords)
parseLabeledUnicastAttributes() ← MPLS (validates against MPLSKeywords)
parseL3VPNAttributes()     ← VPN (validates against VPNKeywords)
```

## Success Criteria

1. ✅ `announce ipv4/unicast ... rd ...` returns error
2. ✅ `announce ipv6/unicast ... rd ...` returns error
3. ✅ Unknown keywords return error (not silently ignored)
4. ✅ Valid unicast keywords still work
5. ✅ All existing tests pass
6. ✅ L3VPN keywords (rd, rt, label) work for mpls-vpn
7. ✅ MPLS keywords (label, split) work for nlri-mpls

## Out of Scope (see route-families.md)

- FlowSpec keyword validation
- VPLS keyword validation
- L2VPN/EVPN keyword validation
