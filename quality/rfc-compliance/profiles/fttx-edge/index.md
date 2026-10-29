# FTTx edge: customers to the internet over L2TP, local DNS for the LAN

Release profile. The RFCs this use case requires, each declared by the Use cases row of its own summary. RFCs it lists as optional follow in their own table and never enter its figures. Before release, every mandatory requirement of every required RFC must be proven by a recorded break of its test.

[All profiles](../) · [All RFCs](../../)

- **Mandatory**: 45.5% (340 of 747 (45.5%)). MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that a recorded test break proves.
- **Optional**: 2.1% (7 of 335 (2.1%)). MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows, counted the same way.
- **Mandatory ceiling**: 70.8% (529 of 747 (70.8%); excluded: Optional feature not offered (classification can be revisited): 18 of 747 (2.4%); Met by a lower layer: 119 of 747 (15.9%); Not applicable (classification can be revisited): 81 of 747 (10.8%)). Attainable mandatory rows under current classifications, over the unchanged total.
- **Optional ceiling**: 100.0% (335 of 335 (100.0%)). Attainable optional rows under current classifications, over the unchanged total.
- **Not walked**: 0 (of 18 RFCs). RFCs with no valid extraction sign-off: their rows count in neither figure.
- **Mandatory register**: not complete (every row proven, none unwalked). A profile is complete only when it requires at least one RFC and none of them is unwalked.

Mandatory ceiling: 529 of 747 (70.8%); excluded: Optional feature not offered (classification can be revisited): 18 of 747 (2.4%); Met by a lower layer: 119 of 747 (15.9%); Not applicable (classification can be revisited): 81 of 747 (10.8%). Optional ceiling: 335 of 335 (100.0%). Lowest mandatory ceiling: rfc4862 0 of 16 (0.0%). Lowest optional ceiling: rfc1035 6 of 6 (100.0%). Ceilings describe current classifications, not an immutable Linux maximum. Not-applicable and feature-declined classifications can be revisited. Every gap remains work Ze owes.

## Tree

Roots first: an RFC sits one level above the highest level of the RFCs it builds on, and the RFCs of one citation cycle share a level. Each bar is the share of that figure's rows a recorded break proves; the ceiling below it shows attainable rows under current classifications. An RFC that is not walked shows an empty bar and no figure or ceiling. The last column names the summary whose Use cases row declares the RFC: a normative citation never adds one.

### Level 0

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 1661](../../rfc1661/) | The Point-to-Point Protocol (PPP) | 39 of 76 (51.3%); ceiling 70 of 76 (92.1%); excluded: Not applicable (classification can be revisited): 6 of 76 (7.9%) | 0 of 25 (0.0%); ceiling 25 of 25 (100.0%) | fttx-edge: required by rfc/short/rfc1661.md |

### Level 1

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 1994](../../rfc1994/) | PPP Challenge Handshake Authentication Protocol (CHAP) | 8 of 16 (50.0%); ceiling 16 of 16 (100.0%) | 2 of 11 (18.2%); ceiling 11 of 11 (100.0%) | fttx-edge: required by rfc/short/rfc1994.md |
| [RFC 2516](../../rfc2516/) | A Method for Transmitting PPP Over Ethernet (PPPoE) | 25 of 37 (67.6%); ceiling 36 of 37 (97.3%); excluded: Met by a lower layer: 1 of 37 (2.7%) | 0 of 3 (0.0%); ceiling 3 of 3 (100.0%) | fttx-edge: required by rfc/short/rfc2516.md |
| [RFC 4787](../../rfc4787/) | Network Address Translation (NAT) Behavioral Requirements for Unicast UDP | 0 of 13 (0.0%); ceiling 6 of 13 (46.2%); excluded: Met by a lower layer: 7 of 13 (53.8%) | 0 of 16 (0.0%); ceiling 16 of 16 (100.0%) | fttx-edge: required by rfc/short/rfc4787.md |

### Level 2

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 1035](../../rfc1035/) (cycle with rfc1034, rfc1122, rfc1191, rfc1195, rfc1332, rfc1812, rfc2131, rfc2132, rfc4291, rfc4443, rfc4861, rfc5905, rfc8200, rfc8201, rfc9293) | Domain Names - Implementation and Specification | 12 of 27 (44.4%); ceiling 27 of 27 (100.0%) | 0 of 6 (0.0%); ceiling 6 of 6 (100.0%) | fttx-edge: required by rfc/short/rfc1035.md |
| [RFC 1332](../../rfc1332/) (cycle with rfc1034, rfc1035, rfc1122, rfc1191, rfc1195, rfc1812, rfc2131, rfc2132, rfc4291, rfc4443, rfc4861, rfc5905, rfc8200, rfc8201, rfc9293) | The PPP Internet Protocol Control Protocol (IPCP) | 6 of 8 (75.0%); ceiling 7 of 8 (87.5%); excluded: Not applicable (classification can be revisited): 1 of 8 (12.5%) | 0 of 6 (0.0%); ceiling 6 of 6 (100.0%) | fttx-edge: required by rfc/short/rfc1332.md |
| [RFC 2131](../../rfc2131/) (cycle with rfc1034, rfc1035, rfc1122, rfc1191, rfc1195, rfc1332, rfc1812, rfc2132, rfc4291, rfc4443, rfc4861, rfc5905, rfc8200, rfc8201, rfc9293) | Dynamic Host Configuration Protocol | 20 of 71 (28.2%); ceiling 47 of 71 (66.2%); excluded: Not applicable (classification can be revisited): 24 of 71 (33.8%) | 2 of 38 (5.3%); ceiling 38 of 38 (100.0%) | fttx-edge: required by rfc/short/rfc2131.md |
| [RFC 2132](../../rfc2132/) (cycle with rfc1034, rfc1035, rfc1122, rfc1191, rfc1195, rfc1332, rfc1812, rfc2131, rfc4291, rfc4443, rfc4861, rfc5905, rfc8200, rfc8201, rfc9293) | DHCP Options and BOOTP Vendor Extensions | 1 of 34 (2.9%); ceiling 9 of 34 (26.5%); excluded: Not applicable (classification can be revisited): 25 of 34 (73.5%) | 0 of 34 (0.0%); ceiling 34 of 34 (100.0%) | fttx-edge: required by rfc/short/rfc2132.md |
| [RFC 4861](../../rfc4861/) (cycle with rfc1034, rfc1035, rfc1122, rfc1191, rfc1195, rfc1332, rfc1812, rfc2131, rfc2132, rfc4291, rfc4443, rfc5905, rfc8200, rfc8201, rfc9293) | Neighbor Discovery for IP version 6 (IPv6) | 30 of 138 (21.7%); ceiling 43 of 138 (31.2%); excluded: Met by a lower layer: 95 of 138 (68.8%) | 0 of 79 (0.0%); ceiling 79 of 79 (100.0%) | fttx-edge: required by rfc/short/rfc4861.md |

### Level 3

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 1877](../../rfc1877/) | PPP Internet Protocol Control Protocol Extensions for Name Server Addresses | no rows; ceiling no rows | no rows; ceiling no rows | fttx-edge: required by rfc/short/rfc1877.md |
| [RFC 2181](../../rfc2181/) | Clarifications to the DNS Specification | 12 of 23 (52.2%); ceiling 18 of 23 (78.3%); excluded: Optional feature not offered (classification can be revisited): 1 of 23 (4.3%); Not applicable (classification can be revisited): 4 of 23 (17.4%) | 1 of 47 (2.1%); ceiling 47 of 47 (100.0%) | fttx-edge: required by rfc/short/rfc2181.md |
| [RFC 3022](../../rfc3022/) | Traditional IP Network Address Translator (Traditional NAT) | no rows; ceiling no rows | no rows; ceiling no rows | fttx-edge: required by rfc/short/rfc3022.md |
| [RFC 4862](../../rfc4862/) | IPv6 Stateless Address Autoconfiguration | 0 of 16 (0.0%); ceiling 0 of 16 (0.0%); excluded: Not applicable (classification can be revisited): 16 of 16 (100.0%) | 0 of 14 (0.0%); ceiling 14 of 14 (100.0%) | fttx-edge: required by rfc/short/rfc4862.md |
| [RFC 5382](../../rfc5382/) | NAT Behavioral Requirements for TCP | 0 of 11 (0.0%); ceiling 6 of 11 (54.5%); excluded: Met by a lower layer: 5 of 11 (45.5%) | 0 of 11 (0.0%); ceiling 11 of 11 (100.0%) | fttx-edge: required by rfc/short/rfc5382.md |

### Level 4

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 5072](../../rfc5072/) | IP Version 6 over PPP | 9 of 16 (56.2%); ceiling 15 of 16 (93.8%); excluded: Not applicable (classification can be revisited): 1 of 16 (6.2%) | 0 of 12 (0.0%); ceiling 12 of 12 (100.0%) | fttx-edge: required by rfc/short/rfc5072.md |
| [RFC 9915](../../rfc9915/) | Dynamic Host Configuration Protocol for IPv6 (DHCPv6) | 83 of 126 (65.9%); ceiling 102 of 126 (81.0%); excluded: Optional feature not offered (classification can be revisited): 17 of 126 (13.5%); Met by a lower layer: 3 of 126 (2.4%); Not applicable (classification can be revisited): 4 of 126 (3.2%) | no rows; ceiling no rows | fttx-edge: required by rfc/short/rfc9915.md |

### Level 5

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 2661](../../rfc2661/) | Layer Two Tunneling Protocol "L2TP" | 81 of 94 (86.2%); ceiling 93 of 94 (98.9%); excluded: Met by a lower layer: 1 of 94 (1.1%) | 2 of 8 (25.0%); ceiling 8 of 8 (100.0%) | fttx-edge: required by rfc/short/rfc2661.md |

### Level 7

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 7084](../../rfc7084/) | Basic Requirements for IPv6 Customer Edge Routers | 14 of 41 (34.1%); ceiling 34 of 41 (82.9%); excluded: Met by a lower layer: 7 of 41 (17.1%) | 0 of 25 (0.0%); ceiling 25 of 25 (100.0%) | fttx-edge: required by rfc/short/rfc7084.md |

## Optional (not counted)

RFCs this use case lists as optional, each declared by the Optional use cases row of its own summary. They count in none of the figures above until the owner brings one into scope by moving it to the Use cases row. Their own figures: mandatory 4 of 192 (2.1%), optional 0 of 66 (0.0%); ceiling mandatory 101 of 192 (52.6%), optional 66 of 66 (100.0%); mandatory excluded: not-applicable 91; 6 RFCs, 0 not walked.

Mandatory ceiling: 101 of 192 (52.6%); excluded: Not applicable (classification can be revisited): 91 of 192 (47.4%). Optional ceiling: 66 of 66 (100.0%). Lowest mandatory ceiling: rfc4035 17 of 108 (15.7%). Lowest optional ceiling: rfc4035 50 of 50 (100.0%). Ceilings describe current classifications, not an immutable Linux maximum. Not-applicable and feature-declined classifications can be revisited. Every gap remains work Ze owes.

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 4638](../../rfc4638/) | Accommodating a Maximum Transit Unit/Maximum Receive Unit (MTU/MRU) Greater Than 1492 in the Point-to-Point Protocol over Ethernet (PPPoE) | 0 of 6 (0.0%); ceiling 6 of 6 (100.0%) | 0 of 3 (0.0%); ceiling 3 of 3 (100.0%) | fttx-edge: optional in rfc/short/rfc4638.md |
| [RFC 5905](../../rfc5905/) (cycle with rfc1034, rfc1035, rfc1122, rfc1191, rfc1195, rfc1332, rfc1812, rfc2131, rfc2132, rfc4291, rfc4443, rfc4861, rfc8200, rfc8201, rfc9293) | Network Time Protocol Version 4: Protocol and Algorithms Specification | 0 of 8 (0.0%); ceiling 8 of 8 (100.0%) | no rows; ceiling no rows | fttx-edge: optional in rfc/short/rfc5905.md |
| [RFC 8106](../../rfc8106/) | IPv6 Router Advertisement Options for DNS Configuration | 0 of 11 (0.0%); ceiling 11 of 11 (100.0%) | 0 of 13 (0.0%); ceiling 13 of 13 (100.0%) | fttx-edge: optional in rfc/short/rfc8106.md |
| [RFC 4033](../../rfc4033/) (cycle with rfc4034, rfc4035) | DNS Security Introduction and Requirements | no rows; ceiling no rows | no rows; ceiling no rows | fttx-edge: optional in rfc/short/rfc4033.md |
| [RFC 4034](../../rfc4034/) (cycle with rfc4033, rfc4035) | Resource Records for the DNS Security Extensions | 0 of 59 (0.0%); ceiling 59 of 59 (100.0%) | no rows; ceiling no rows | fttx-edge: optional in rfc/short/rfc4034.md |
| [RFC 4035](../../rfc4035/) (cycle with rfc4033, rfc4034) | Protocol Modifications for the DNS Security Extensions | 4 of 108 (3.7%); ceiling 17 of 108 (15.7%); excluded: Not applicable (classification can be revisited): 91 of 108 (84.3%) | 0 of 50 (0.0%); ceiling 50 of 50 (100.0%) | fttx-edge: optional in rfc/short/rfc4035.md |
