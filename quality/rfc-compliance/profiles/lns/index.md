# L2TP network server

Release profile. The RFCs this use case requires, each declared by the Use cases row of its own summary. RFCs it lists as optional follow in their own table and never enter its figures. Before release, every mandatory requirement of every required RFC must be proven by a recorded break of its test.

[All profiles](../) · [All RFCs](../../)

- **Mandatory**: 55.2% (253 of 458 (55.2%)). MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that a recorded test break proves.
- **Optional**: 4.3% (7 of 161 (4.3%)). MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows, counted the same way.
- **Mandatory ceiling**: 77.3% (354 of 458 (77.3%); excluded: Met by a lower layer: 96 of 458 (21.0%); Not applicable (classification can be revisited): 8 of 458 (1.7%)). Attainable mandatory rows under current classifications, over the unchanged total.
- **Optional ceiling**: 100.0% (161 of 161 (100.0%)). Attainable optional rows under current classifications, over the unchanged total.
- **Not walked**: 0 (of 14 RFCs). RFCs with no valid extraction sign-off: their rows count in neither figure.
- **Mandatory register**: not complete (every row proven, none unwalked). A profile is complete only when it requires at least one RFC and none of them is unwalked.

Mandatory ceiling: 354 of 458 (77.3%); excluded: Met by a lower layer: 96 of 458 (21.0%); Not applicable (classification can be revisited): 8 of 458 (1.7%). Optional ceiling: 161 of 161 (100.0%). Lowest mandatory ceiling: rfc4861 43 of 138 (31.2%). Lowest optional ceiling: rfc1332 6 of 6 (100.0%). Ceilings describe current classifications, not an immutable Linux maximum. Not-applicable and feature-declined classifications can be revisited. Every gap remains work Ze owes.

## Tree

Roots first: an RFC sits one level above the highest level of the RFCs it builds on, and the RFCs of one citation cycle share a level. Each bar is the share of that figure's rows a recorded break proves; the ceiling below it shows attainable rows under current classifications. An RFC that is not walked shows an empty bar and no figure or ceiling. The last column names the summary whose Use cases row declares the RFC: a normative citation never adds one.

### Level 0

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 1661](../../rfc1661/) | The Point-to-Point Protocol (PPP) | 39 of 76 (51.3%); ceiling 70 of 76 (92.1%); excluded: Not applicable (classification can be revisited): 6 of 76 (7.9%) | 0 of 25 (0.0%); ceiling 25 of 25 (100.0%) | lns: required by rfc/short/rfc1661.md |
| [RFC 2865](../../rfc2865/) (cycle with rfc2866) | Remote Authentication Dial In User Service (RADIUS) | 24 of 31 (77.4%); ceiling 31 of 31 (100.0%) | 0 of 1 (0.0%); ceiling 1 of 1 (100.0%) | lns: required by rfc/short/rfc2865.md |
| [RFC 2866](../../rfc2866/) (cycle with rfc2865) | RADIUS Accounting | 8 of 15 (53.3%); ceiling 15 of 15 (100.0%) | 0 of 1 (0.0%); ceiling 1 of 1 (100.0%) | lns: required by rfc/short/rfc2866.md |

### Level 1

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 1994](../../rfc1994/) | PPP Challenge Handshake Authentication Protocol (CHAP) | 8 of 16 (50.0%); ceiling 16 of 16 (100.0%) | 2 of 11 (18.2%); ceiling 11 of 11 (100.0%) | lns: required by rfc/short/rfc1994.md |

### Level 2

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 1332](../../rfc1332/) (cycle with rfc1034, rfc1035, rfc1122, rfc1191, rfc1195, rfc1812, rfc2131, rfc2132, rfc4291, rfc4443, rfc4861, rfc5905, rfc8200, rfc8201, rfc9293) | The PPP Internet Protocol Control Protocol (IPCP) | 6 of 8 (75.0%); ceiling 7 of 8 (87.5%); excluded: Not applicable (classification can be revisited): 1 of 8 (12.5%) | 0 of 6 (0.0%); ceiling 6 of 6 (100.0%) | lns: required by rfc/short/rfc1332.md |
| [RFC 1334](../../rfc1334/) | PPP Authentication Protocols | 11 of 13 (84.6%); ceiling 13 of 13 (100.0%) | 0 of 3 (0.0%); ceiling 3 of 3 (100.0%) | lns: required by rfc/short/rfc1334.md |
| [RFC 2759](../../rfc2759/) | Microsoft PPP CHAP Extensions, Version 2 | 4 of 7 (57.1%); ceiling 7 of 7 (100.0%) | 2 of 4 (50.0%); ceiling 4 of 4 (100.0%) | lns: required by rfc/short/rfc2759.md |
| [RFC 4861](../../rfc4861/) (cycle with rfc1034, rfc1035, rfc1122, rfc1191, rfc1195, rfc1332, rfc1812, rfc2131, rfc2132, rfc4291, rfc4443, rfc5905, rfc8200, rfc8201, rfc9293) | Neighbor Discovery for IP version 6 (IPv6) | 30 of 138 (21.7%); ceiling 43 of 138 (31.2%); excluded: Met by a lower layer: 95 of 138 (68.8%) | 0 of 79 (0.0%); ceiling 79 of 79 (100.0%) | lns: required by rfc/short/rfc4861.md |

### Level 3

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 1877](../../rfc1877/) | PPP Internet Protocol Control Protocol Extensions for Name Server Addresses | no rows; ceiling no rows | no rows; ceiling no rows | lns: required by rfc/short/rfc1877.md |

### Level 4

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 5072](../../rfc5072/) | IP Version 6 over PPP | 9 of 16 (56.2%); ceiling 15 of 16 (93.8%); excluded: Not applicable (classification can be revisited): 1 of 16 (6.2%) | 0 of 12 (0.0%); ceiling 12 of 12 (100.0%) | lns: required by rfc/short/rfc5072.md |

### Level 5

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 2661](../../rfc2661/) | Layer Two Tunneling Protocol "L2TP" | 81 of 94 (86.2%); ceiling 93 of 94 (98.9%); excluded: Met by a lower layer: 1 of 94 (1.1%) | 2 of 8 (25.0%); ceiling 8 of 8 (100.0%) | lns: required by rfc/short/rfc2661.md |

### Level 6

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 2869](../../rfc2869/) (cycle with rfc2867, rfc2868) | RADIUS Extensions | 4 of 8 (50.0%); ceiling 8 of 8 (100.0%) | 0 of 3 (0.0%); ceiling 3 of 3 (100.0%) | lns: required by rfc/short/rfc2869.md |

### Level 7

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 3162](../../rfc3162/) | RADIUS and IPv6 | 3 of 6 (50.0%); ceiling 6 of 6 (100.0%) | 0 of 7 (0.0%); ceiling 7 of 7 (100.0%) | lns: required by rfc/short/rfc3162.md |

### Level 8

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 5176](../../rfc5176/) (cycle with rfc3579) | Dynamic Authorization Extensions to Remote Authentication Dial In User Service (RADIUS) | 26 of 30 (86.7%); ceiling 30 of 30 (100.0%) | 1 of 1 (100.0%); ceiling 1 of 1 (100.0%) | lns: required by rfc/short/rfc5176.md |

## Optional (not counted)

RFCs this use case lists as optional, each declared by the Optional use cases row of its own summary. They count in none of the figures above until the owner brings one into scope by moving it to the Use cases row. Their own figures: mandatory 17 of 86 (19.8%), optional 2 of 116 (1.7%); ceiling mandatory 86 of 86 (100.0%), optional 116 of 116 (100.0%); 7 RFCs, 0 not walked.

Mandatory ceiling: 86 of 86 (100.0%). Optional ceiling: 116 of 116 (100.0%). Every RFC shown with measured rows can reach 100% under current classifications. Lowest mandatory ceiling: rfc1570 22 of 22 (100.0%). Lowest optional ceiling: rfc1570 16 of 16 (100.0%). Ceilings describe current classifications, not an immutable Linux maximum. Not-applicable and feature-declined classifications can be revisited. Every gap remains work Ze owes.

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 1570](../../rfc1570/) | PPP LCP Extensions | 0 of 22 (0.0%); ceiling 22 of 22 (100.0%) | 0 of 16 (0.0%); ceiling 16 of 16 (100.0%) | lns: optional in rfc/short/rfc1570.md |
| [RFC 6911](../../rfc6911/) | RADIUS Attributes for IPv6 Access Networks | 0 of 3 (0.0%); ceiling 3 of 3 (100.0%) | 0 of 9 (0.0%); ceiling 9 of 9 (100.0%) | lns: optional in rfc/short/rfc6911.md |
| [RFC 4818](../../rfc4818/) | RADIUS Delegated-IPv6-Prefix Attribute | 0 of 3 (0.0%); ceiling 3 of 3 (100.0%) | 0 of 3 (0.0%); ceiling 3 of 3 (100.0%) | lns: optional in rfc/short/rfc4818.md |
| [RFC 2867](../../rfc2867/) (cycle with rfc2868, rfc2869) | RADIUS Accounting Modifications for Tunnel Protocol Support | 0 of 1 (0.0%); ceiling 1 of 1 (100.0%) | 0 of 16 (0.0%); ceiling 16 of 16 (100.0%) | lns: optional in rfc/short/rfc2867.md |
| [RFC 2868](../../rfc2868/) (cycle with rfc2867, rfc2869) | RADIUS Attributes for Tunnel Protocol Support | 0 of 18 (0.0%); ceiling 18 of 18 (100.0%) | 0 of 43 (0.0%); ceiling 43 of 43 (100.0%) | lns: optional in rfc/short/rfc2868.md |
| [RFC 3145](../../rfc3145/) | L2TP Disconnect Cause Information | 0 of 8 (0.0%); ceiling 8 of 8 (100.0%) | 0 of 10 (0.0%); ceiling 10 of 10 (100.0%) | lns: optional in rfc/short/rfc3145.md |
| [RFC 3579](../../rfc3579/) (cycle with rfc5176) | RADIUS (Remote Authentication Dial In User Service) Support For Extensible Authentication Protocol (EAP) | 17 of 31 (54.8%); ceiling 31 of 31 (100.0%) | 2 of 19 (10.5%); ceiling 19 of 19 (100.0%) | lns: optional in rfc/short/rfc3579.md |
