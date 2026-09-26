# UPDATE Message Packing Strategy

## Historical Context

BGP-4 (RFC 4271) was designed for parsing efficiency:

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

<!-- source: internal/core/bgp/attribute/attribute.go -- AttrMPReachNLRI, AttrMPUnreachNLRI constants -->

Later RFCs added more attributes that may relate to specific NLRI.

Result:
- Attribute blob now contains NLRI-specific data
- Cannot cache/share attribute section between updates
- Parsing must scan entire attribute section to find MP_REACH/MP_UNREACH
- RFC type-code ordering (Appendix F.3) scatters related data

## Ze Packing Strategy

### Goal: Restore Parsing Efficiency

Pack attributes in the order `attribute.OrderAttributes` decides (within the
RFC attribute section):

```
+---------------------------+
| Traditional Withdrawn     |  ← RFC 4271 withdraw section (IPv4 unicast)
+---------------------------+
| MP_UNREACH_NLRI (15)      |  ← Withdrawals, first
+---------------------------+
| Every other attribute     |  ← ORIGIN, AS_PATH, NEXT_HOP, MED, ...
|  by type code             |    MP_REACH_NLRI (14) included, between
|                           |    CLUSTER_LIST (10) and EXT. COMMUNITIES (16)
+---------------------------+
| Traditional NLRI          |  ← RFC 4271 NLRI section (IPv4 unicast)
+---------------------------+
```

<!-- source: internal/component/bgp/message/update_build.go -- UPDATE attribute ordering -->
<!-- source: internal/core/bgp/attribute/origin.go -- OrderAttributes -->

### Why This Order

1. **MP_UNREACH first**: Withdrawals logically precede announcements
2. **Everything else by type code**: one order for every build path, so a static route and the same route sent through `update text` produce the same bytes
3. **Not RFC 7606 §5.1**: §5.1 requires the MP attribute to be the very first path attribute. MP_REACH_NLRI sorts by type code instead, so an announcement with ORIGIN or AS_PATH does not meet it. This is the `{gap}` on RFC7606-5.1-1, demonstrated by `TestRFC7606Section51MPAttributeEncodedFirst`

### Benefits

- Attribute caching: Routes with same path share serialized attribute prefix
- Faster parsing: Regular attrs at known offset, MP attrs at end
- Zero-copy potential: Can splice cached attrs + fresh MP_REACH

### Implementation Note

This is an internal optimization, NOT a protocol change. Receivers parse per RFC.
Senders MAY order attributes however they want (RFC 4271 Appendix F.3 is SHOULD, not MUST).

## One Writer for Every Origin

An UPDATE Ze originates through the API reaches the wire by one of two rails.
`Peer.ShouldQueue` picks: a route injected while the destination peer still drains
its initial sync is queued and drained through `buildRIBRouteUpdate`, and the same
route injected after establishment is built by `buildBatchAnnounceUpdate`. Nothing
in the route selects the rail. Scheduling does.

Both rails now describe the announce the same way: the caller's attribute block is
a BASE, and everything the rail adds (the mandatory attributes, the authoritative
NEXT_HOP or MP_REACH_NLRI, an iBGP LOCAL_PREF, the RFC 6793 AS4_PATH) is an edit
over it. `announceAttrs` hands both to `attrEmitter`, which is the same
plan-size-write walk `buildModifiedPayload` runs for a FORWARDED UPDATE. Ascending
type-code order (RFC 4271 Section 5), the header size class, and the exact output
size are therefore properties of that one writer.

<!-- source: internal/component/bgp/reactor/announce_build.go -- announceAttrs, the shared announce writer -->
<!-- source: internal/component/bgp/reactor/forward_build.go -- attrEmitter, the plan-size-write walk both origins run -->

Three writers preceded it, and the cost was a class of timing-dependent defect
rather than a style problem. `attribute.Builder.WriteTo` emitted a fixed order
coded into a function body, the established rail merge-inserted into a byte block
with `findAttrInsertPosition`, and the queued rail interleaved range passes around
an `attrWriter`. One route could reach the wire as two different byte strings
depending on which rail won the race. `attribute.Builder` is now an intent
collector: `AppendAttributes` is its single ordering statement, and both consumers
read it.

<!-- source: internal/core/bgp/attribute/builder.go -- AppendAttributes, the builder's only ordering statement -->

## Non-Goal

This is NOT proposing an RFC change. Just documenting Ze's internal strategy
for efficient UPDATE construction while remaining fully RFC-compliant.

<!-- source: internal/component/bgp/message/update_build.go -- update building with attribute ordering -->
<!-- source: internal/core/bgp/wire/update_sections.go -- UpdateSections parsing -->
