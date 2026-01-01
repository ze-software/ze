# Issue: ORIGINATOR_ID/CLUSTER_LIST Missing in MVPN/FlowSpec/MUP Routes

**Priority:** Low
**RFC:** 4456 (BGP Route Reflection)
**Created:** 2026-01-01

## Problem

The following route types do **not** support ORIGINATOR_ID and CLUSTER_LIST attributes:

| Route Type | Params Struct | Has OriginatorID | Has ClusterList |
|------------|---------------|------------------|-----------------|
| MVPN (SAFI 5) | `MVPNParams` | ❌ | ❌ |
| FlowSpec (SAFI 133/134) | `FlowSpecParams` | ❌ | ❌ |
| MUP (SAFI 85) | `MUPParams` | ❌ | ❌ |

These route types cannot be used with route reflector configurations because the RFC 4456 attributes are not passed through.

## Route Types That Already Have Support

| Route Type | Params Struct | Has OriginatorID | Has ClusterList |
|------------|---------------|------------------|-----------------|
| Unicast | `UnicastParams` | ✅ | ✅ |
| VPN (SAFI 128) | `VPNParams` | ✅ | ✅ |
| Labeled Unicast (SAFI 4) | `LabeledUnicastParams` | ✅ | ✅ |
| VPLS (AFI 25, SAFI 65) | `VPLSParams` | ✅ | ✅ |

## Impact

- Route reflector deployments using MVPN, FlowSpec, or MUP will silently drop these attributes
- Loop detection (CLUSTER_LIST) will not work for these route types
- ORIGINATOR_ID for client identification will be lost

## Risk Assessment

**Low priority** because:
1. Route reflector configurations with these SAFIs are rare
2. MVPN/FlowSpec/MUP are advanced features with limited deployment
3. No existing tests or functional requirements for this combination

## Fix Required

For each affected Params struct, add:

```go
// ORIGINATOR_ID (RFC 4456) - 0 means not set.
OriginatorID uint32

// CLUSTER_LIST (RFC 4456) - cluster IDs traversed.
ClusterList []uint32
```

Then add encoding in each Build* method:

```go
// 9. ORIGINATOR_ID (RFC 4456)
if p.OriginatorID != 0 {
    origIP := netip.AddrFrom4([4]byte{
        byte(p.OriginatorID >> 24), byte(p.OriginatorID >> 16),
        byte(p.OriginatorID >> 8), byte(p.OriginatorID),
    })
    attrs = append(attrs, attribute.OriginatorID(origIP))
}

// 10. CLUSTER_LIST (RFC 4456)
if len(p.ClusterList) > 0 {
    cl := make(attribute.ClusterList, len(p.ClusterList))
    copy(cl, p.ClusterList)
    attrs = append(attrs, cl)
}
```

## Files to Modify

- `pkg/bgp/message/update_build.go`:
  - Add fields to `MVPNParams`, `FlowSpecParams`, `MUPParams`
  - Add encoding in `BuildMVPN`, `BuildFlowSpec`, `BuildMUP`

## Testing

Add tests similar to `TestBuildUnicast_EncodesReflectorAttrs` for each affected builder.

## Related

- Fixed in same PR: Unicast, VPN, LabeledUnicast, VPLS (already have support)
- Spec: `plan/spec-peer-encoding-extraction.md`
