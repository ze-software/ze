# UPDATE Message Packing Strategy

## Historical Context

Original BGP (RFC 1771/4271) was designed for parsing efficiency:

```
+------------------+
| Withdrawn Routes |  ← NLRI being removed
+------------------+
| Path Attributes  |  ← Apply to ALL announced NLRI
+------------------+
| NLRI             |  ← Prefixes being announced
+------------------+
```

This was elegant:
- Withdraw needs no extra data
- Attributes are a single blob, cacheable, shareable
- NLRI follows - attributes apply to all

## What Broke

Multi-protocol extensions (RFC 4760) embedded NLRI inside attributes:
- `MP_REACH_NLRI` (type 14) - contains next-hop + announced NLRI
- `MP_UNREACH_NLRI` (type 15) - contains withdrawn NLRI

<!-- source: internal/component/bgp/attribute/attribute.go -- AttrMPReachNLRI, AttrMPUnreachNLRI constants -->

Later RFCs added more attributes that may relate to specific NLRI.

Result:
- Attribute blob now contains NLRI-specific data
- Cannot cache/share attribute section between updates
- Parsing must scan entire attribute section to find MP_REACH/MP_UNREACH
- RFC type-code ordering (Appendix F.3) scatters related data

## Ze Packing Strategy

### Goal: Restore Parsing Efficiency

Pack attributes in this order (within the RFC attribute section):

```
+---------------------------+
| Traditional Withdrawn     |  ← RFC 4271 withdraw section (IPv4 unicast)
+---------------------------+
| Regular Attributes        |  ← ORIGIN, AS_PATH, NEXT_HOP, MED, etc.
|  (type codes 1-13, 16+    |    Excludes MP_REACH (14), MP_UNREACH (15)
|   except 14, 15)          |    CACHEABLE - same for route groups
+---------------------------+
| MP_REACH_NLRI (14)        |  ← Announces (shifted to end)
+---------------------------+
| MP_UNREACH_NLRI (15)      |  ← Withdrawals (shifted to end)
+---------------------------+
| Traditional NLRI          |  ← RFC 4271 NLRI section (IPv4 unicast)
+---------------------------+
```

<!-- source: internal/component/bgp/message/update_build.go -- UPDATE attribute ordering -->

### Why This Order

1. **MP_UNREACH first**: Withdrawals logically precede announcements
2. **Regular attributes second**: Cacheable blob, shared between updates with same path
3. **MP_REACH last**: Announcements follow the attributes that describe them
4. **Preserves RFC compliance**: Just reorders within attribute section (allowed)

### Benefits

- Attribute caching: Routes with same path share serialized attribute prefix
- Faster parsing: Regular attrs at known offset, MP attrs at end
- Zero-copy potential: Can splice cached attrs + fresh MP_REACH

### Implementation Note

This is an internal optimization, NOT a protocol change. Receivers parse per RFC.
Senders MAY order attributes however they want (RFC 4271 Appendix F.3 is SHOULD, not MUST).

## Non-Goal

This is NOT proposing an RFC change. Just documenting Ze's internal strategy
for efficient UPDATE construction while remaining fully RFC-compliant.

<!-- source: internal/component/bgp/message/update_build.go -- update building with attribute ordering -->
<!-- source: internal/component/bgp/wire/update_sections.go -- UpdateSections parsing -->
