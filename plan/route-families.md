# Route Family Keyword Validation Plan

## Current State

✅ **Implemented:**
- IPv4 unicast: `UnicastKeywords` validation
- IPv6 unicast: `UnicastKeywords` validation
- L3VPN (IPv4/IPv6 VPN): `VPNKeywords` validation with RD, RT, label parsing

❌ **Not Implemented:**
- MPLS (Labeled Unicast)
- FlowSpec
- VPLS
- L2VPN/EVPN

## BGP Address Families

| AFI | SAFI | Family | Handler | Status |
|-----|------|--------|---------|--------|
| 1 | 1 | IPv4 Unicast | `handleAnnounceIPv4` | ✅ Validated |
| 2 | 1 | IPv6 Unicast | `handleAnnounceIPv6` | ✅ Validated |
| 1 | 4 | IPv4 MPLS | - | ❌ Not implemented |
| 2 | 4 | IPv6 MPLS | - | ❌ Not implemented |
| 1 | 128 | IPv4 VPN | `handleAnnounceIPv4` | ✅ Validated |
| 2 | 128 | IPv6 VPN | `handleAnnounceIPv6` | ✅ Validated |
| 1 | 133 | IPv4 FlowSpec | `handleAnnounceFlow` | ⚠️ No validation |
| 2 | 133 | IPv6 FlowSpec | `handleAnnounceFlow` | ⚠️ No validation |
| 25 | 65 | VPLS | `handleAnnounceVPLS` | ⚠️ No validation |
| 25 | 70 | EVPN | `handleAnnounceL2VPN` | ⚠️ No validation |

## Keyword Sets by Family

### UnicastKeywords (done)
```go
var UnicastKeywords = KeywordSet{
    "next-hop", "origin", "med", "local-preference",
    "as-path", "community", "large-community", "split",
}
```

### MPLSKeywords (to implement)
```go
var MPLSKeywords = UnicastKeywords + {
    "label", // MPLS label stack
}
```

### VPNKeywords (defined, not used)
```go
var VPNKeywords = UnicastKeywords + {
    "rd",    // Route Distinguisher
    "rt",    // Route Target (extended community)
    "label", // MPLS label
}
```

### FlowSpecKeywords (to implement)
```go
var FlowSpecKeywords = KeywordSet{
    "rd", "next-hop",
    // Match criteria
    "source", "destination", "protocol", "port",
    "source-port", "destination-port",
    "icmp-type", "icmp-code", "tcp-flags",
    "packet-length", "dscp", "fragment",
    // Actions
    "rate-limit", "redirect", "mark", "action",
    "extended-community",
}
```

### VPLSKeywords (to implement)
```go
var VPLSKeywords = KeywordSet{
    "rd", "rt", "ve-id", "ve-block-offset", "ve-block-size",
    "label-block-offset", "label-block-size", "next-hop",
    "extended-community",
}
```

### L2VPNKeywords (to implement)
```go
var L2VPNKeywords = KeywordSet{
    "rd", "rt", "next-hop", "label", "esi", "ethernet-tag",
    "mac", "ip", "extended-community",
}
```

## Implementation Plan

### Phase 1: MPLS (Labeled Unicast)
1. Add `handleAnnounceMPLS` handler for IPv4/IPv6 MPLS
2. Define `MPLSKeywords` (unicast + label)
3. Parse MPLS label stack
4. Add tests

### Phase 2: L3VPN Support ✅ DONE
1. ~~Add `handleAnnounceVPN` handler for IPv4/IPv6 VPN~~ ✅
2. ~~Use existing `VPNKeywords`~~ ✅
3. ~~Parse RD, RT, label~~ ✅
4. ~~Add tests~~ ✅
5. ~~RD format validation (RFC 4364 Type 0/1/2)~~ ✅
6. ~~Label range validation (20-bit, 0-1048575)~~ ✅
7. ~~Label stack support `[label1 label2]`~~ ✅
8. ~~Label=0 (Explicit Null) accepted~~ ✅

### Phase 3: FlowSpec Validation
1. Define `FlowSpecKeywords`
2. Update `handleAnnounceFlow` to validate keywords
3. Add tests for invalid keywords

### Phase 4: VPLS Validation
1. Define `VPLSKeywords`
2. Update `handleAnnounceVPLS` to validate keywords
3. Add tests

### Phase 5: L2VPN/EVPN Validation
1. Define `L2VPNKeywords`
2. Update `handleAnnounceL2VPN` to validate keywords
3. Add tests

## Priority

1. **High:** MPLS - labeled unicast, foundation for VPN
2. **High:** L3VPN - most common after unicast
3. **Medium:** FlowSpec - used for DDoS mitigation
4. **Low:** VPLS, L2VPN/EVPN - specialized use cases

## Dependencies

- Check ExaBGP for keyword compatibility
- Review RFC 8277 (MPLS Labels), RFC 4364 (L3VPN), RFC 5575 (FlowSpec), RFC 4761 (VPLS), RFC 7432 (EVPN)
