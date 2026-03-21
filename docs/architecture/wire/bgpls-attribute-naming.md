# BGP-LS Attribute TLV Naming Convention

**Status:** Draft -- pending agreement before implementation.

This document defines the naming convention for BGP-LS attribute type 29 TLV fields.
Names are used as JSON keys in decode output AND as text keywords in the API command syntax.
Input = output symmetry: what you type is what you see.

## Naming Rules

| Rule | Example |
|------|---------|
| kebab-case always | `igp-metric`, not `igp_metric` or `igpMetric` |
| Plural key = JSON array value | `srlgs` (array of uint32), `local-router-ids` (array of strings) |
| Singular key = scalar or single object | `igp-metric` (int), `prefix-sid` (object) |
| Fixed-size array = singular | `unreserved-bandwidth` (always 8 floats, not variable) |
| `-sid`/`-sids` suffix on all SID TLVs | `adj-sids` (list), `prefix-sid` (single), `peer-node-sid` (single) |
| `max-` not `maximum-` | `max-link-bandwidth`, not `maximum-link-bandwidth` |
| Drop redundant qualifiers | `admin-group` not `admin-group-mask`, `sr-prefix-flags` not `sr-prefix-attribute-flags` |
| Shorter when unambiguous | `link-delay` not `unidirectional-link-delay` |

## Node Attribute TLVs (RFC 7752 Section 3.3.1)

| TLV | RFC Name | Ze Name | ExaBGP Name | GoBGP JSON | Type | Notes |
|-----|----------|---------|-------------|------------|------|-------|
| 1024 | Node Flag Bits | `node-flags` | N/A | `node_flags` | object | {O,T,E,B,R,V,RSV} |
| 1025 | Opaque Node Attribute | `opaque-node-attr` | N/A | `node_opaque_attribute` | string | hex |
| 1026 | Node Name | `node-name` | N/A | `node_name` | string | UTF-8 |
| 1027 | IS-IS Area Identifier | `area-id` | N/A | `isis_area_id` | string | hex |
| 1028 | IPv4 Local Router-ID | `local-router-ids` | N/A | `node_local_router_id_ipv4` | []string | plural: can have v4+v6 |
| 1029 | IPv6 Local Router-ID | `local-router-ids` | N/A | `node_local_router_id_ipv6` | []string | merged with 1028 |

## Node SR Attribute TLVs (RFC 9085)

| TLV | RFC Name | Ze Name | ExaBGP Name | GoBGP JSON | Type | Notes |
|-----|----------|---------|-------------|------------|------|-------|
| 1034 | SR Capabilities | `sr-capabilities` | N/A | `sr_capabilities` | object | {flags, ranges[]} |
| 1035 | SR Algorithm | `sr-algorithms` | N/A | `sr_algorithms` | []int | plural: list of IDs |
| 1036 | SR Local Block | `sr-local-block` | N/A | `sr_local_block` | object | {ranges[]} |

## Link Attribute TLVs (RFC 7752 Section 3.3.2)

| TLV | RFC Name | Ze Name | ExaBGP Name | GoBGP JSON | Type | Notes |
|-----|----------|---------|-------------|------------|------|-------|
| 1030 | IPv4 Remote Router-ID | `remote-router-ids` | N/A | `remote_router_id_ipv4` | []string | plural: can have v4+v6 |
| 1031 | IPv6 Remote Router-ID | `remote-router-ids` | N/A | `remote_router_id_ipv6` | []string | merged with 1030 |
| 1088 | Admin Group | `admin-group` | N/A | `admin_group` | int | uint32 bitmask |
| 1089 | Max Link Bandwidth | `max-link-bandwidth` | N/A | `max_link_bw` | float | IEEE float32, bytes/sec |
| 1090 | Max Reservable Bandwidth | `max-reservable-bandwidth` | N/A | `max_reservable_link_bw` | float | IEEE float32, bytes/sec |
| 1091 | Unreserved Bandwidth | `unreserved-bandwidth` | N/A | `unreserved_bw` | []float | fixed 8 (singular: not variable) |
| 1092 | TE Default Metric | `te-metric` | N/A | `te_default_metric` | int | uint32 |
| 1095 | IGP Metric | `igp-metric` | N/A | `igp_metric` | int | 1-3 bytes |
| 1096 | SRLG | `srlgs` | N/A | `srlgs` | []int | plural: list of uint32 |
| 1097 | Opaque Link Attribute | `opaque-link-attr` | N/A | `link_opaque_attribute` | string | hex |
| 1098 | Link Name | `link-name` | N/A | `link_name` | string | UTF-8 |

## Link SR Attribute TLVs (RFC 9085)

| TLV | RFC Name | Ze Name | ExaBGP Name | GoBGP JSON | Type | Notes |
|-----|----------|---------|-------------|------------|------|-------|
| 1099 | Adjacency SID | `adj-sids` | N/A | `adjacency_sid` | []object | plural: multiple instances |

## Link EPE Attribute TLVs (RFC 9086)

| TLV | RFC Name | Ze Name | ExaBGP Name | GoBGP JSON | Type | Notes |
|-----|----------|---------|-------------|------------|------|-------|
| 1101 | Peer Node SID | `peer-node-sid` | N/A | `peer_node_sid` | object | singular: one per link |
| 1102 | Peer Adjacency SID | `peer-adj-sid` | N/A | `peer_adjacency_sid` | object | singular |
| 1103 | Peer Set SID | `peer-set-sid` | N/A | `peer_set_sid` | object | singular |

## Link SRv6 Attribute TLVs (RFC 9514)

| TLV | RFC Name | Ze Name | ExaBGP Name | GoBGP JSON | Type | Notes |
|-----|----------|---------|-------------|------------|------|-------|
| 1106 | SRv6 End.X SID | `srv6-endx-sids` | N/A | `srv6_end_x_sid` | []object | plural: multiple SIDs |
| 1107 | IS-IS SRv6 LAN End.X SID | `srv6-lan-endx-isis` | N/A | N/A | []object | plural implied by context |
| 1108 | OSPFv3 SRv6 LAN End.X SID | `srv6-lan-endx-ospf` | N/A | N/A | []object | plural implied by context |

## Link Delay Metric TLVs (RFC 8571)

| TLV | RFC Name | Ze Name | ExaBGP Name | GoBGP JSON | Type | Notes |
|-----|----------|---------|-------------|------------|------|-------|
| 1114 | Unidirectional Link Delay | `link-delay` | N/A | `unidirectional_link_delay` | object | {anomalous, delay} |
| 1115 | Min/Max Link Delay | `link-delay-minmax` | N/A | `min_max_unidirectional_link_delay` | object | {anomalous, min-delay, max-delay} |
| 1116 | Delay Variation | `delay-variation` | N/A | `unidirectional_delay_variation` | int | microseconds |

## Prefix Attribute TLVs (RFC 7752 Section 3.3.3)

| TLV | RFC Name | Ze Name | ExaBGP Name | GoBGP JSON | Type | Notes |
|-----|----------|---------|-------------|------------|------|-------|
| 1152 | IGP Flags | `igp-flags` | N/A | `igp_flags` | object | {D,N,L,P,RSV} |
| 1155 | Prefix Metric | `prefix-metric` | N/A | N/A (TODO) | int | uint32 |
| 1157 | Opaque Prefix Attribute | `opaque-prefix-attr` | N/A | `prefix_opaque_attribute` | string | hex |

## Prefix SR Attribute TLVs (RFC 9085)

| TLV | RFC Name | Ze Name | ExaBGP Name | GoBGP JSON | Type | Notes |
|-----|----------|---------|-------------|------------|------|-------|
| 1158 | Prefix SID | `prefix-sid` | N/A | `sr_prefix_sid` | object | {flags, algorithm, sid} |
| 1161 | SID/Label | `sid-label` | N/A | N/A (sub-TLV) | int | 20-bit label or 32-bit index |
| 1170 | SR Prefix Attribute Flags | `sr-prefix-flags` | N/A | N/A (TODO) | object | {X,R,N,RSV} |
| 1171 | Source Router ID | `source-router-id` | N/A | `source_router_id` | string | IPv4 or IPv6 |

## SRv6 Attribute TLVs (RFC 9514)

| TLV | RFC Name | Ze Name | ExaBGP Name | GoBGP JSON | Type | Notes |
|-----|----------|---------|-------------|------------|------|-------|
| 1250 | SRv6 Endpoint Behavior | `srv6-endpoint-behavior` | N/A | `srv6_endpoint_behavior` | object | {behavior, flags, algorithm} |
| 1251 | SRv6 BGP Peer Node SID | `srv6-bgp-peer-node-sid` | N/A | `srv6_bgp_peer_node_sid` | object | {flags, weight, peer-as, peer-bgp-id} |
| 1252 | SRv6 SID Structure | `srv6-sid-structure` | N/A | `srv6_sid_structure` | object | {loc-block-len, loc-node-len, func-len, arg-len} |

## Unknown TLVs

| Pattern | Ze Name | Type | Notes |
|---------|---------|------|-------|
| Unregistered TLV code | `generic-lsid-<code>` | []string | hex value in array (forward compatibility) |

## ExaBGP Migration Notes

ExaBGP did not decode BGP-LS attribute TLVs (type 29). It passed them through as opaque hex bytes.
All ExaBGP Name columns are N/A because there is no ExaBGP naming to migrate from.

ExaBGP DID decode BGP-LS NLRI descriptors (TLVs 256-265, 512-515). Those names are documented
in `docs/architecture/wire/nlri-bgpls.md` and are not part of this naming convention (they are
NLRI fields, not attribute TLV fields).

## Renames from Current Implementation

The following renames are needed to align with this convention:

| Current Ze Name | New Ze Name | Reason |
|-----------------|-------------|--------|
| `sr-adj` | `adj-sids` | Clearer + plural (list of SID entries) |
| `srlg` | `srlgs` | Plural (list of values) |
| `sr-algorithm` | `sr-algorithms` | Plural (list of IDs) |
| `admin-group-mask` | `admin-group` | Drop redundant `-mask` |
| `sr-prefix-attribute-flags` | `sr-prefix-flags` | Shorter |
| `maximum-link-bandwidth` | `max-link-bandwidth` | `max-` not `maximum-` |
| `maximum-reservable-link-bandwidth` | `max-reservable-bandwidth` | `max-` + shorter |
| `srv6-endx` | `srv6-endx-sids` | Plural + `-sids` suffix |
| `unidirectional-link-delay` | `link-delay` | Shorter, unambiguous |
| `min-max-link-delay` | `link-delay-minmax` | Groups with `link-delay` prefix |

## Text API Syntax (future)

When the text API is implemented, the keyword will match the JSON key:

```
peer * update text bgp-ls igp-metric set 10 node-name set "router1" adj-sids add ... nlri bgp-ls/bgp-ls add ...
```

For list attributes: `add` appends, `set` replaces, `del` removes.
For scalar attributes: `set` replaces, `del` removes.

**Last Updated:** 2026-03-21
