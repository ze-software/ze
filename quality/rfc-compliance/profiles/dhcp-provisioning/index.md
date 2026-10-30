# DHCP and provisioning

Technology aggregate. DHCP, address autoconfiguration, and TFTP provisioning. The RFCs whose summary names the dhcp-provisioning aggregate, with their mandatory and optional coverage.

[All profiles](../) · [All RFCs](../../)

- **Mandatory**: 35.5% (168 of 473 (35.5%)). MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that a recorded test break proves.
- **Optional**: 2.7% (6 of 220 (2.7%)). MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows, counted the same way.
- **Mandatory ceiling**: 72.1% (341 of 473 (72.1%); excluded: Optional feature not offered (classification can be revisited): 15 of 473 (3.2%); Met by a lower layer: 116 of 473 (24.5%); Not applicable (classification can be revisited): 1 of 473 (0.2%)). Attainable mandatory rows under current classifications, over the unchanged total.
- **Optional ceiling**: 100.0% (220 of 220 (100.0%)). Attainable optional rows under current classifications, over the unchanged total.
- **Not walked**: 0 (of 13 RFCs). RFCs with no valid extraction sign-off: their rows count in neither figure.
- **Mandatory register**: not complete (every row proven, none unwalked). A profile is complete only when it requires at least one RFC and none of them is unwalked.

Mandatory ceiling: 341 of 473 (72.1%); excluded: Optional feature not offered (classification can be revisited): 15 of 473 (3.2%); Met by a lower layer: 116 of 473 (24.5%); Not applicable (classification can be revisited): 1 of 473 (0.2%). Optional ceiling: 220 of 220 (100.0%). Lowest mandatory ceiling: rfc4861 47 of 138 (34.1%). Lowest optional ceiling: rfc1350 4 of 4 (100.0%). Ceilings describe current classifications, not an immutable Linux maximum. Not-applicable and feature-declined classifications can be revisited. Every gap remains work Ze owes.

## Tree

Roots first: an RFC sits one level above the highest level of the RFCs it builds on, and the RFCs of one citation cycle share a level. Each bar is the share of that figure's rows a recorded break proves; the ceiling below it shows attainable rows under current classifications. An RFC that is not walked shows an empty bar and no figure or ceiling.

### Level 0

| RFC | Title | Mandatory | Optional |
|---|---|---|---|
| [RFC 1350](../../rfc1350/) | The TFTP Protocol (Revision 2) | 8 of 10 (80.0%); ceiling 10 of 10 (100.0%) | 2 of 4 (50.0%); ceiling 4 of 4 (100.0%) |

### Level 1

| RFC | Title | Mandatory | Optional |
|---|---|---|---|
| [RFC 2347](../../rfc2347/) (cycle with rfc2348, rfc2349) | TFTP Option Extension | 3 of 3 (100.0%); ceiling 3 of 3 (100.0%) | 1 of 2 (50.0%); ceiling 2 of 2 (100.0%) |
| [RFC 2348](../../rfc2348/) (cycle with rfc2347, rfc2349) | TFTP Blocksize Option | 5 of 5 (100.0%); ceiling 5 of 5 (100.0%) | no rows; ceiling no rows |
| [RFC 2349](../../rfc2349/) (cycle with rfc2347, rfc2348) | TFTP Timeout Interval and Transfer Size Options | 1 of 4 (25.0%); ceiling 4 of 4 (100.0%) | 1 of 2 (50.0%); ceiling 2 of 2 (100.0%) |

### Level 2

| RFC | Title | Mandatory | Optional |
|---|---|---|---|
| [RFC 2131](../../rfc2131/) (cycle with rfc1034, rfc1035, rfc1122, rfc1191, rfc1195, rfc1332, rfc1812, rfc2132, rfc4291, rfc4443, rfc4861, rfc5905, rfc8200, rfc8201, rfc9293) | Dynamic Host Configuration Protocol | 18 of 71 (25.4%); ceiling 62 of 71 (87.3%); excluded: Met by a lower layer: 9 of 71 (12.7%) | 1 of 38 (2.6%); ceiling 38 of 38 (100.0%) |
| [RFC 2132](../../rfc2132/) (cycle with rfc1034, rfc1035, rfc1122, rfc1191, rfc1195, rfc1332, rfc1812, rfc2131, rfc4291, rfc4443, rfc4861, rfc5905, rfc8200, rfc8201, rfc9293) | DHCP Options and BOOTP Vendor Extensions | 1 of 34 (2.9%); ceiling 34 of 34 (100.0%) | 0 of 34 (0.0%); ceiling 34 of 34 (100.0%) |
| [RFC 4861](../../rfc4861/) (cycle with rfc1034, rfc1035, rfc1122, rfc1191, rfc1195, rfc1332, rfc1812, rfc2131, rfc2132, rfc4291, rfc4443, rfc5905, rfc8200, rfc8201, rfc9293) | Neighbor Discovery for IP version 6 (IPv6) | 30 of 138 (21.7%); ceiling 47 of 138 (34.1%); excluded: Met by a lower layer: 91 of 138 (65.9%) | 0 of 79 (0.0%); ceiling 79 of 79 (100.0%) |
| [RFC 7440](../../rfc7440/) | TFTP Windowsize Option | 5 of 9 (55.6%); ceiling 9 of 9 (100.0%) | 1 of 6 (16.7%); ceiling 6 of 6 (100.0%) |

### Level 3

| RFC | Title | Mandatory | Optional |
|---|---|---|---|
| [RFC 4578](../../rfc4578/) | Dynamic Host Configuration Protocol (DHCP) Options for the Intel Preboot eXecution Environment (PXE) | 1 of 5 (20.0%); ceiling 4 of 5 (80.0%); excluded: Not applicable (classification can be revisited): 1 of 5 (20.0%) | 0 of 3 (0.0%); ceiling 3 of 3 (100.0%) |
| [RFC 4862](../../rfc4862/) (cycle with rfc2464) | IPv6 Stateless Address Autoconfiguration | 0 of 16 (0.0%); ceiling 10 of 16 (62.5%); excluded: Met by a lower layer: 6 of 16 (37.5%) | 0 of 14 (0.0%); ceiling 14 of 14 (100.0%) |

### Level 4

| RFC | Title | Mandatory | Optional |
|---|---|---|---|
| [RFC 8106](../../rfc8106/) | IPv6 Router Advertisement Options for DNS Configuration | 0 of 11 (0.0%); ceiling 11 of 11 (100.0%) | 0 of 13 (0.0%); ceiling 13 of 13 (100.0%) |
| [RFC 9915](../../rfc9915/) | Dynamic Host Configuration Protocol for IPv6 (DHCPv6) | 82 of 126 (65.1%); ceiling 108 of 126 (85.7%); excluded: Optional feature not offered (classification can be revisited): 15 of 126 (11.9%); Met by a lower layer: 3 of 126 (2.4%) | no rows; ceiling no rows |

### Level 7

| RFC | Title | Mandatory | Optional |
|---|---|---|---|
| [RFC 7084](../../rfc7084/) | Basic Requirements for IPv6 Customer Edge Routers | 14 of 41 (34.1%); ceiling 34 of 41 (82.9%); excluded: Met by a lower layer: 7 of 41 (17.1%) | 0 of 25 (0.0%); ceiling 25 of 25 (100.0%) |
