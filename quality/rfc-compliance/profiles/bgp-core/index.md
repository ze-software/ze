# BGP core

Technology aggregate. BGP base protocol, capabilities, attributes, and error handling. The RFCs whose summary names the bgp-core aggregate, with their mandatory and optional coverage.

[All profiles](../) · [All RFCs](../../)

Use case: [BGP performance testing with Ze](../../../../use-cases/bgp-performance/)

Use case: [ExaBGP Migration](../../../../use-cases/exabgp-migration/)

- **Mandatory**: 55.6% (193 of 347 (55.6%)). MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that a recorded test break proves.
- **Optional**: 7.0% (8 of 114 (7.0%)). MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows, counted the same way.
- **Mandatory ceiling**: 96.5% (335 of 347 (96.5%); excluded: Optional feature not offered (classification can be revisited): 1 of 347 (0.3%); Not applicable (classification can be revisited): 11 of 347 (3.2%)). Attainable mandatory rows under current classifications, over the unchanged total.
- **Optional ceiling**: 100.0% (114 of 114 (100.0%)). Attainable optional rows under current classifications, over the unchanged total.
- **Not walked**: 0 (of 21 RFCs). RFCs with no valid extraction sign-off: their rows count in neither figure.
- **Mandatory register**: not complete (every row proven, none unwalked). A profile is complete only when it requires at least one RFC and none of them is unwalked.

Mandatory ceiling: 335 of 347 (96.5%); excluded: Optional feature not offered (classification can be revisited): 1 of 347 (0.3%); Not applicable (classification can be revisited): 11 of 347 (3.2%). Optional ceiling: 114 of 114 (100.0%). Lowest mandatory ceiling: rfc8654 10 of 12 (83.3%). Lowest optional ceiling: rfc2545 2 of 2 (100.0%). Ceilings describe current classifications, not an immutable Linux maximum. Not-applicable and feature-declined classifications can be revisited. Every gap remains work Ze owes.

## Tree

Roots first: an RFC sits one level above the highest level of the RFCs it builds on, and the RFCs of one citation cycle share a level. Each bar is the share of that figure's rows a recorded break proves; the ceiling below it shows attainable rows under current classifications. An RFC that is not walked shows an empty bar and no figure or ceiling.

### Level 3

| RFC | Title | Mandatory | Optional |
|---|---|---|---|
| [RFC 4271](../../rfc4271/) | A Border Gateway Protocol 4 (BGP-4) | 80 of 133 (60.2%); ceiling 126 of 133 (94.7%); excluded: Not applicable (classification can be revisited): 7 of 133 (5.3%) | 3 of 34 (8.8%); ceiling 34 of 34 (100.0%) |

### Level 4

| RFC | Title | Mandatory | Optional |
|---|---|---|---|
| [RFC 2545](../../rfc2545/) | Use of BGP-4 Multiprotocol Extensions for IPv6 Inter-Domain Routing | 3 of 5 (60.0%); ceiling 5 of 5 (100.0%) | 0 of 2 (0.0%); ceiling 2 of 2 (100.0%) |
| [RFC 5492](../../rfc5492/) | Capabilities Advertisement with BGP-4 | 2 of 9 (22.2%); ceiling 8 of 9 (88.9%); excluded: Not applicable (classification can be revisited): 1 of 9 (11.1%) | 0 of 7 (0.0%); ceiling 7 of 7 (100.0%) |
| [RFC 6608](../../rfc6608/) | Subcodes for BGP Finite State Machine Error | 0 of 3 (0.0%); ceiling 3 of 3 (100.0%) | no rows; ceiling no rows |
| [RFC 8212](../../rfc8212/) | Default External BGP (EBGP) Route Propagation Behavior without Policies | 0 of 2 (0.0%); ceiling 2 of 2 (100.0%) | 0 of 1 (0.0%); ceiling 1 of 1 (100.0%) |
| [RFC 9072](../../rfc9072/) | Extended Optional Parameters Length for BGP OPEN Message | 5 of 9 (55.6%); ceiling 9 of 9 (100.0%) | 1 of 3 (33.3%); ceiling 3 of 3 (100.0%) |
| [RFC 9687](../../rfc9687/) | Border Gateway Protocol 4 (BGP-4) Send Hold Timer | 3 of 13 (23.1%); ceiling 13 of 13 (100.0%) | 0 of 7 (0.0%); ceiling 7 of 7 (100.0%) |

### Level 5

| RFC | Title | Mandatory | Optional |
|---|---|---|---|
| [RFC 4760](../../rfc4760/) | Multiprotocol Extensions for BGP-4 | 5 of 6 (83.3%); ceiling 6 of 6 (100.0%) | 1 of 11 (9.1%); ceiling 11 of 11 (100.0%) |

### Level 6

| RFC | Title | Mandatory | Optional |
|---|---|---|---|
| [RFC 2918](../../rfc2918/) | Route Refresh Capability for BGP-4 | 3 of 6 (50.0%); ceiling 6 of 6 (100.0%) | 1 of 5 (20.0%); ceiling 5 of 5 (100.0%) |
| [RFC 4486](../../rfc4486/) | Subcodes for BGP Cease NOTIFICATION Message | 0 of 1 (0.0%); ceiling 1 of 1 (100.0%) | 0 of 10 (0.0%); ceiling 10 of 10 (100.0%) |
| [RFC 6793](../../rfc6793/) (cycle with rfc5668) | BGP Support for Four-Octet Autonomous System (AS) Number Space | 20 of 30 (66.7%); ceiling 30 of 30 (100.0%) | 0 of 6 (0.0%); ceiling 6 of 6 (100.0%) |
| [RFC 7311](../../rfc7311/) | The Accumulated IGP Metric Attribute for BGP | 13 of 22 (59.1%); ceiling 22 of 22 (100.0%) | 0 of 4 (0.0%); ceiling 4 of 4 (100.0%) |
| [RFC 7911](../../rfc7911/) | Advertisement of Multiple Paths in BGP | 9 of 9 (100.0%); ceiling 9 of 9 (100.0%) | 0 of 4 (0.0%); ceiling 4 of 4 (100.0%) |

### Level 7

| RFC | Title | Mandatory | Optional |
|---|---|---|---|
| [RFC 6286](../../rfc6286/) | Autonomous-System-Wide Unique BGP Identifier for BGP-4 | 0 of 4 (0.0%); ceiling 4 of 4 (100.0%) | 1 of 1 (100.0%); ceiling 1 of 1 (100.0%) |
| [RFC 7313](../../rfc7313/) | Enhanced Route Refresh Capability for BGP-4 | 5 of 11 (45.5%); ceiling 10 of 11 (90.9%); excluded: Optional feature not offered (classification can be revisited): 1 of 11 (9.1%) | 0 of 6 (0.0%); ceiling 6 of 6 (100.0%) |
| [RFC 7606](../../rfc7606/) (cycle with rfc4364, rfc4659, rfc4798, rfc6368, rfc8277, rfc8950) | Revised Error Handling for BGP UPDATE Messages | 28 of 53 (52.8%); ceiling 52 of 53 (98.1%); excluded: Not applicable (classification can be revisited): 1 of 53 (1.9%) | 0 of 4 (0.0%); ceiling 4 of 4 (100.0%) |
| [RFC 8950](../../rfc8950/) (cycle with rfc4364, rfc4659, rfc4798, rfc6368, rfc7606, rfc8277) | Advertising IPv4 Network Layer Reachability Information (NLRI) with an IPv6 Next Hop | 3 of 6 (50.0%); ceiling 6 of 6 (100.0%) | 0 of 2 (0.0%); ceiling 2 of 2 (100.0%) |

### Level 8

| RFC | Title | Mandatory | Optional |
|---|---|---|---|
| [RFC 5549](../../rfc5549/) | Advertising IPv4 Network Layer Reachability Information with an IPv6 Next Hop | 3 of 6 (50.0%); ceiling 6 of 6 (100.0%) | 0 of 3 (0.0%); ceiling 3 of 3 (100.0%) |
| [RFC 7607](../../rfc7607/) | Codification of AS 0 Processing | 0 of 5 (0.0%); ceiling 5 of 5 (100.0%) | no rows; ceiling no rows |
| [RFC 8654](../../rfc8654/) | Extended Message Support for BGP | 9 of 12 (75.0%); ceiling 10 of 12 (83.3%); excluded: Not applicable (classification can be revisited): 2 of 12 (16.7%) | 1 of 4 (25.0%); ceiling 4 of 4 (100.0%) |
| [RFC 9774](../../rfc9774/) | Deprecation of AS_SET and AS_CONFED_SET in BGP | 2 of 2 (100.0%); ceiling 2 of 2 (100.0%) | no rows; ceiling no rows |
