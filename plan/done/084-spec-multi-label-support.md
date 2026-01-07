# Spec: multi-label-support

## Task
Add RFC 8277 multi-label stack support. Currently ZeBGP parsing handles multiple labels but building/config only supports single label. This creates an asymmetry where received routes with label stacks are parsed correctly, but locally originated routes cannot use multiple labels.

## Required Reading
- [ ] `.claude/zebgp/wire/NLRI.md` - NLRI wire format for labeled routes
- [ ] `.claude/zebgp/UPDATE_BUILDING.md` - *Params struct design pattern
- [ ] `rfc/rfc8277.txt` - RFC 8277 Multiple Labels Capability

**Key insights:**
- RFC 8277 Section 2: Label stack is "a sequence of MPLS labels organized as a contiguous part of a label stack"
- RFC 8277 Section 2.1: Without Multiple Labels Capability → single label MUST be used
- RFC 8277 Section 2.1: With capability negotiated by both peers → MAY use multiple labels
- IPVPN already uses `labels []uint32` for parsing (pkg/bgp/nlri/ipvpn.go:261)
- VPNParams, LabeledUnicastParams use `Label uint32` (single)
- StaticRoute uses `Label uint32` (single)

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates |
|------|------|-----------|
| `TestStaticRoute_MultiLabel` | `pkg/reactor/peersettings_test.go` | StaticRoute with multiple labels, IsLabeledUnicast() |
| `TestBuildVPNNLRIBytes_MultiLabel` | `pkg/bgp/message/update_build_test.go` | Wire format: totalBits, label encoding |
| `TestBuildLabeledUnicastNLRIBytes_MultiLabel` | `pkg/bgp/message/update_build_test.go` | Wire format: totalBits, label encoding |
| `TestBuildVPN_MultiLabel` | `pkg/bgp/message/update_build_test.go` | Full BuildVPN with label stack |
| `TestBuildLabeledUnicast_MultiLabel` | `pkg/bgp/message/update_build_test.go` | Full BuildLabeledUnicast with label stack |
| `TestConfigLoader_MultiLabel` | `pkg/config/loader_test.go` | Config parsing `labels [100 200]` |
| `TestConfigLoader_LabelBackwardCompat` | `pkg/config/loader_test.go` | Old `label 100` syntax still works |
| `TestConfigLoader_VPNRequiresLabel` | `pkg/config/loader_test.go` | VPN route without label rejected |

### Functional Tests
| Test | Location | Scenario |
|------|----------|----------|
| N/A | - | Existing functional tests cover single-label; multi-label is config-only |

## Files to Modify
- `pkg/reactor/peersettings.go` - Change `Label uint32` → `Labels []uint32`, update IsLabeledUnicast()
- `pkg/bgp/message/update_build.go` - Change VPNParams.Label, LabeledUnicastParams.Label to Labels []uint32; update buildVPNNLRIBytes, buildLabeledUnicastNLRIBytes to use EncodeLabelStack
- `pkg/reactor/peer.go` - Update toStaticRouteVPNParams, toStaticRouteLabeledUnicastParams
- `pkg/config/loader.go` - Support `labels: [100, 200]` array syntax
- `pkg/config/bgp.go` - Update StaticRouteConfig.Label to Labels, add schema field
- `pkg/config/routeattr.go` - Update ParsedRouteAttributes.Label to Labels []MPLSLabel

## Implementation Steps
1. **Write tests** - Wire format tests for multi-label NLRI encoding
2. **Run tests** - Verify FAIL (paste output)
3. **Change StaticRoute** - `Label uint32` → `Labels []uint32`, update IsLabeledUnicast()
4. **Change *Params structs** - Update VPNParams.Label, LabeledUnicastParams.Label to Labels []uint32
5. **Update wire encoding** - buildVPNNLRIBytes, buildLabeledUnicastNLRIBytes: use EncodeLabelStack, fix totalBits
6. **Update conversion functions** - toStaticRouteVPNParams, toStaticRouteLabeledUnicastParams
7. **Update config** - Schema for `labels` field, loader to parse array, backward compat with `label`
8. **Update routeattr.go** - ParsedRouteAttributes.Label → Labels
9. **Run tests** - Verify PASS (paste output)
10. **Verify all** - `make lint && make test && make functional`
11. **RFC refs** - Add RFC 8277 comments

## Design Decisions

### Backward Compatibility
Config will support both forms:
```yaml
# Old (single label) - still works
route 10.0.0.0/8 rd 100:100 label 1000 next-hop self;

# New (multiple labels)
route 10.0.0.0/8 rd 100:100 labels [1000 2000] next-hop self;
```

### Helper Methods
```go
// IsLabeledUnicast detection needs update
func (r StaticRoute) IsLabeledUnicast() bool {
    return len(r.Labels) > 0 && r.RD == ""
}

// Single-label helper for common case
func (r StaticRoute) SingleLabel() uint32 {
    if len(r.Labels) > 0 {
        return r.Labels[0]
    }
    return 0
}
```

### Wire Encoding
Current `buildVPNNLRIBytes` and `buildLabeledUnicastNLRIBytes` manually encode a single label:
```go
// Current (hardcoded single label with BOS=1):
labelBytes := []byte{byte(label >> 12), byte(label >> 4), byte(label<<4) | 0x01}
```

Change to use existing `nlri.EncodeLabelStack()` which handles multiple labels correctly:
```go
// New (use existing function):
labelBytes := nlri.EncodeLabelStack(p.Labels)
```

**Critical:** Update totalBits calculation:
```go
// Current: totalBits := 24 + 64 + prefixBits (VPN, one label)
// New:     totalBits := len(labels)*24 + 64 + prefixBits (VPN, N labels)

// Current: totalBits := 24 + prefixBits (Labeled, one label)
// New:     totalBits := len(labels)*24 + prefixBits (Labeled, N labels)
```

### Capability Negotiation (Out of Scope)
RFC 8277 Section 2.1 specifies Multiple Labels Capability negotiation. This spec implements data structure support only; capability negotiation is deferred:
- Config-originated routes are operator responsibility
- Operators must ensure peer supports multi-label before configuring
- Capability enforcement can be added in a future spec

### Withdrawal Semantics
RFC 8277 withdrawal uses special label 0x800000. Multi-label withdrawals:
- Use single withdrawal label (not the original stack)
- Existing withdrawal code handles this correctly

### Validation
**Labeled-unicast self-validates:** `IsLabeledUnicast()` returns false when `len(Labels) == 0`, so empty labels → treated as plain unicast. No explicit validation needed.

**VPN requires validation:** `IsVPN()` checks `RD != ""` only, not labels. A VPN route with empty labels would produce invalid wire format.

Validate in config loader (fail early):
```go
// pkg/config/loader.go - during static route parsing
if hasRD && len(labels) == 0 {
    return fmt.Errorf("VPN route %s requires at least one label", prefix)
}
```

### API Impact
If API text handler (`pkg/api/`) creates routes with labels:
1. Update to support `labels` array syntax
2. Apply same validation: VPN routes require ≥1 label

Check for StaticRoute usage in API code during implementation.

### Breaking Change Note
Existing configs with VPN routes that omit `label` (implicitly using label 0) will now error. Fix: add explicit `label 0` to such routes.

## RFC Documentation
- Add `// RFC 8277 Section 2` comments to Labels field
- Add `// RFC 8277 Section 2.3` comments for multi-label encoding

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
- [ ] RFC references added
- [ ] `.claude/zebgp/wire/NLRI.md` updated if needed

### Completion
- [ ] Spec moved to `plan/done/NNN-multi-label-support.md`
