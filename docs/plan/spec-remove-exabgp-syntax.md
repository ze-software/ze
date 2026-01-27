# Spec: remove-exabgp-syntax (Umbrella)

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/compatibility.md` - NO ExaBGP in engine
3. Child specs (below)

## Task

Remove ExaBGP syntax from engine. Per `.claude/rules/compatibility.md`:
- **Engine code:** No ExaBGP format awareness, no compatibility shims
- **Config migration:** One-time conversion, not runtime compatibility

This is an **umbrella spec** that coordinates two child specs.

## Required Reading

### Rules
- [ ] `.claude/rules/compatibility.md` - compatibility policy

### Architecture Docs
- [ ] `docs/architecture/api/update-syntax.md` - API syntax reference

## Child Specs (in order)

| Spec | Description | Status |
|------|-------------|--------|
| `done/180-native-update-syntax.md` | Add native `update { attribute { } nlri { } }` config syntax | ✅ |
| `spec-remove-exabgp-announce.md` | Remove ExaBGP syntax, convert tests to native | TODO |

## Design Decision: RESOLVED

### API Syntax (unchanged)
```
peer * update text origin set igp nhop set 10.0.0.1 nlri ipv4/unicast add 1.0.0.0/24
```

### Config Syntax (NEW - replaces announce/static)
```
update {
    attribute {
        origin igp;
        next-hop 10.0.0.1;
        community [ 65000:1 65000:2 ];
    }
    nlri {
        ipv4/unicast 1.0.0.0/24 2.0.0.0/24;
    }
}
```

### Ze Family Reference
| Family | Config Syntax |
|--------|---------------|
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

Pattern: `family [modifiers] nlri-data;` (no `add` keyword in config - always announce)

## ExaBGP Syntax to Remove (Phase 2)

| Syntax | Location | Description |
|--------|----------|-------------|
| `announce { }` | YANG, bgp.go | ExaBGP route announcement block |
| `static { }` | YANG, bgp.go | Old ExaBGP syntax |
| `flow { }` | YANG, bgp.go | ExaBGP FlowSpec syntax (not ipv4/flowspec) |
| `l2vpn { }` | YANG, bgp.go | ExaBGP L2VPN syntax |
| `withdraw { }` | YANG, bgp.go | ExaBGP withdraw syntax |
| `operational { }` | YANG, bgp.go | ExaBGP-specific |
| `neighbor-changes` | YANG, bgp.go | ExaBGP API notification |

## 🧪 TDD Test Plan

**Umbrella spec - tests defined in child specs:**
- `spec-native-update-syntax.md` - tests for new syntax parsing
- `spec-remove-exabgp-announce.md` - tests for removal/conversion

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| See child specs | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| See child specs | - | - | - |

## Files to Modify

**Umbrella spec - files listed in child specs:**
- `spec-native-update-syntax.md` - YANG, bgp.go additions
- `spec-remove-exabgp-announce.md` - YANG, bgp.go removals, test updates

## Implementation Steps

1. **Create child spec 1** - `spec-native-update-syntax.md`
2. **Implement Phase 1** - Add native `update { }` syntax
3. **Create child spec 2** - `spec-remove-exabgp-announce.md`
4. **Implement Phase 2** - Remove ExaBGP syntax, convert tests
5. **Verify all** - `make lint && make test && make functional`

## Checklist

### 🧪 TDD
- [ ] Tests written (in child specs)
- [ ] Tests FAIL (in child specs)
- [ ] Implementation complete
- [ ] Tests PASS (in child specs)

### Verification
- [ ] Phase 1: Native `update { }` syntax implemented
- [ ] Phase 2: ExaBGP syntax removed, tests converted
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make functional` passes
