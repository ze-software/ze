# DHCP and provisioning

Technology aggregate. DHCP, address autoconfiguration, and TFTP provisioning. The RFCs whose summary names the dhcp-provisioning aggregate, with their mandatory and optional coverage.

[All profiles](../) · [All RFCs](../../)

- **Mandatory**: 33.6% (159 of 473 (33.6%)). MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that a recorded test break proves.
- **Optional**: 0.9% (2 of 220 (0.9%)). MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows, counted the same way.
- **Mandatory ceiling**: 55.6% (263 of 473 (55.6%); excluded: Optional feature not offered (classification can be revisited): 17 of 473 (3.6%); Met by a lower layer: 105 of 473 (22.2%); Not applicable (classification can be revisited): 88 of 473 (18.6%)). Attainable mandatory rows under current classifications, over the unchanged total.
- **Optional ceiling**: 100.0% (220 of 220 (100.0%)). Attainable optional rows under current classifications, over the unchanged total.
- **Not walked**: 0 (of 13 RFCs). RFCs with no valid extraction sign-off: their rows count in neither figure.
- **Mandatory register**: not complete (every row proven, none unwalked). A profile is complete only when it requires at least one RFC and none of them is unwalked.

Mandatory ceiling: 263 of 473 (55.6%); excluded: Optional feature not offered (classification can be revisited): 17 of 473 (3.6%); Met by a lower layer: 105 of 473 (22.2%); Not applicable (classification can be revisited): 88 of 473 (18.6%). Optional ceiling: 220 of 220 (100.0%). Lowest mandatory ceiling: rfc4862 0 of 16 (0.0%). Lowest optional ceiling: rfc1350 4 of 4 (100.0%). Ceilings describe current classifications, not an immutable Linux maximum. Not-applicable and feature-declined classifications can be revisited. Every gap remains work Ze owes.

## Tree

Roots first: an RFC sits one level above the highest level of the RFCs it builds on, and the RFCs of one citation cycle share a level. Each bar is the share of that figure's rows a recorded break proves; the ceiling below it shows attainable rows under current classifications. An RFC that is not walked shows an empty bar and no figure or ceiling.

### Level 0

| RFC | Title | Mandatory | Optional |
|---|---|---|---|
| [RFC 1350](../../rfc1350/) | The TFTP Protocol (Revision 2) | 6 of 10 (60.0%); ceiling 8 of 10 (80.0%); excluded: Not applicable (classification can be revisited): 2 of 10 (20.0%) | 0 of 4 (0.0%); ceiling 4 of 4 (100.0%) |

### Level 1

| RFC | Title | Mandatory | Optional |
|---|---|---|---|
| [RFC 2347](../../rfc2347/) (cycle with rfc2348, rfc2349) | TFTP Option Extension | 2 of 3 (66.7%); ceiling 2 of 3 (66.7%); excluded: Not applicable (classification can be revisited): 1 of 3 (33.3%) | 0 of 2 (0.0%); ceiling 2 of 2 (100.0%) |
| [RFC 2348](../../rfc2348/) (cycle with rfc2347, rfc2349) | TFTP Blocksize Option | 3 of 5 (60.0%); ceiling 4 of 5 (80.0%); excluded: Not applicable (classification can be revisited): 1 of 5 (20.0%) | no rows; ceiling no rows |
| [RFC 2349](../../rfc2349/) (cycle with rfc2347, rfc2348) | TFTP Timeout Interval and Transfer Size Options | 0 of 4 (0.0%); ceiling 1 of 4 (25.0%); excluded: Not applicable (classification can be revisited): 3 of 4 (75.0%) | 0 of 2 (0.0%); ceiling 2 of 2 (100.0%) |

### Level 2

| RFC | Title | Mandatory | Optional |
|---|---|---|---|
| [RFC 2131](../../rfc2131/) (cycle with rfc1034, rfc1035, rfc1122, rfc1191, rfc1195, rfc1332, rfc1812, rfc2132, rfc4291, rfc4443, rfc4861, rfc5905, rfc8200, rfc8201, rfc9293) | Dynamic Host Configuration Protocol | 18 of 71 (25.4%); ceiling 47 of 71 (66.2%); excluded: Not applicable (classification can be revisited): 24 of 71 (33.8%) | 2 of 38 (5.3%); ceiling 38 of 38 (100.0%) |
| [RFC 2132](../../rfc2132/) (cycle with rfc1034, rfc1035, rfc1122, rfc1191, rfc1195, rfc1332, rfc1812, rfc2131, rfc4291, rfc4443, rfc4861, rfc5905, rfc8200, rfc8201, rfc9293) | DHCP Options and BOOTP Vendor Extensions | 1 of 34 (2.9%); ceiling 9 of 34 (26.5%); excluded: Not applicable (classification can be revisited): 25 of 34 (73.5%) | 0 of 34 (0.0%); ceiling 34 of 34 (100.0%) |
| [RFC 4861](../../rfc4861/) (cycle with rfc1034, rfc1035, rfc1122, rfc1191, rfc1195, rfc1332, rfc1812, rfc2131, rfc2132, rfc4291, rfc4443, rfc5905, rfc8200, rfc8201, rfc9293) | Neighbor Discovery for IP version 6 (IPv6) | 30 of 138 (21.7%); ceiling 43 of 138 (31.2%); excluded: Met by a lower layer: 95 of 138 (68.8%) | 0 of 79 (0.0%); ceiling 79 of 79 (100.0%) |
| [RFC 7440](../../rfc7440/) | TFTP Windowsize Option | 1 of 9 (11.1%); ceiling 1 of 9 (11.1%); excluded: Not applicable (classification can be revisited): 8 of 9 (88.9%) | 0 of 6 (0.0%); ceiling 6 of 6 (100.0%) |

### Level 3

| RFC | Title | Mandatory | Optional |
|---|---|---|---|
| [RFC 4578](../../rfc4578/) | Dynamic Host Configuration Protocol (DHCP) Options for the Intel Preboot eXecution Environment (PXE) | 1 of 5 (20.0%); ceiling 1 of 5 (20.0%); excluded: Not applicable (classification can be revisited): 4 of 5 (80.0%) | 0 of 3 (0.0%); ceiling 3 of 3 (100.0%) |
| [RFC 4862](../../rfc4862/) | IPv6 Stateless Address Autoconfiguration | 0 of 16 (0.0%); ceiling 0 of 16 (0.0%); excluded: Not applicable (classification can be revisited): 16 of 16 (100.0%) | 0 of 14 (0.0%); ceiling 14 of 14 (100.0%) |

### Level 4

| RFC | Title | Mandatory | Optional |
|---|---|---|---|
| [RFC 8106](../../rfc8106/) | IPv6 Router Advertisement Options for DNS Configuration | 0 of 11 (0.0%); ceiling 11 of 11 (100.0%) | 0 of 13 (0.0%); ceiling 13 of 13 (100.0%) |
| [RFC 9915](../../rfc9915/) | Dynamic Host Configuration Protocol for IPv6 (DHCPv6) | 83 of 126 (65.9%); ceiling 102 of 126 (81.0%); excluded: Optional feature not offered (classification can be revisited): 17 of 126 (13.5%); Met by a lower layer: 3 of 126 (2.4%); Not applicable (classification can be revisited): 4 of 126 (3.2%) | no rows; ceiling no rows |

### Level 7

| RFC | Title | Mandatory | Optional |
|---|---|---|---|
| [RFC 7084](../../rfc7084/) | Basic Requirements for IPv6 Customer Edge Routers | 14 of 41 (34.1%); ceiling 34 of 41 (82.9%); excluded: Met by a lower layer: 7 of 41 (17.1%) | 0 of 25 (0.0%); ceiling 25 of 25 (100.0%) |
