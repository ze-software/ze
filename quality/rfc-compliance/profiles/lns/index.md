# L2TP network server

Release profile. The RFCs this use case requires, each declared by the Use cases row of its own summary. RFCs it lists as optional follow in their own table and never enter its figures. Before release, every mandatory requirement of every required RFC must be proven by a recorded break of its test.

[All profiles](../) · [All RFCs](../../)

Figures as of the last build, read from [data/rfc/profiles/lns.json](../../../../data/rfc/profiles/lns.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze proven 263, OS testing gap 94, Ze correctness gap 101
- **Overview bar**: Ze MUST proven 263, SHOULD proven 7, SHOULD not decided 154, OS testing gap 94, Ze correctness gap 101

| Figure | Rows |
|---|---|
| RFCs | 14 |
| Not walked | 0 |
| Mandatory register | not complete |
| MUST rows | 458 |
| MUST rows kept (the MUST bar) | 458 |
| Ze proven | 263 |
| OS verified | 0 |
| OS testing gap | 94 |
| Ze correctness gap | 101 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 161 |
| SHOULD proven | 7 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 154 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

## Tree

Roots first: an RFC sits one level above the highest level of the RFCs it builds on, and the RFCs of one citation cycle share a level. Each RFC carries the same two bars as the set above, from its own data file; an RFC that is not walked shows "not walked" and no bar. The last column names the summary whose Use cases row declares the RFC: a normative citation never adds one.

### Level 0

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 1661](../../rfc1661/) | The Point-to-Point Protocol (PPP) | Ze proven 42, OS testing gap 2, Ze correctness gap 32 | Ze MUST proven 42, SHOULD not decided 25, OS testing gap 2, Ze correctness gap 32 | lns: required by rfc/short/rfc1661.md |
| [RFC 2865](../../rfc2865/) (cycle with rfc2866) | Remote Authentication Dial In User Service (RADIUS) | Ze proven 27, Ze correctness gap 4 | Ze MUST proven 27, SHOULD not decided 1, Ze correctness gap 4 | lns: required by rfc/short/rfc2865.md |
| [RFC 2866](../../rfc2866/) (cycle with rfc2865) | RADIUS Accounting | Ze proven 8, Ze correctness gap 7 | Ze MUST proven 8, SHOULD not decided 1, Ze correctness gap 7 | lns: required by rfc/short/rfc2866.md |

### Level 1

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 1994](../../rfc1994/) | PPP Challenge Handshake Authentication Protocol (CHAP) | Ze proven 8, Ze correctness gap 8 | Ze MUST proven 8, SHOULD proven 2, SHOULD not decided 9, Ze correctness gap 8 | lns: required by rfc/short/rfc1994.md |

### Level 2

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 1332](../../rfc1332/) (cycle with rfc1034, rfc1035, rfc1122, rfc1191, rfc1195, rfc1812, rfc2131, rfc2132, rfc4291, rfc4443, rfc4861, rfc5905, rfc8200, rfc8201, rfc9293) | The PPP Internet Protocol Control Protocol (IPCP) | Ze proven 8 | Ze MUST proven 8, SHOULD not decided 6 | lns: required by rfc/short/rfc1332.md |
| [RFC 1334](../../rfc1334/) | PPP Authentication Protocols | Ze proven 13 | Ze MUST proven 13, SHOULD not decided 3 | lns: required by rfc/short/rfc1334.md |
| [RFC 4861](../../rfc4861/) (cycle with rfc1034, rfc1035, rfc1122, rfc1191, rfc1195, rfc1332, rfc1812, rfc2131, rfc2132, rfc4291, rfc4443, rfc5905, rfc8200, rfc8201, rfc9293) | Neighbor Discovery for IP version 6 (IPv6) | Ze proven 30, OS testing gap 91, Ze correctness gap 17 | Ze MUST proven 30, SHOULD not decided 79, OS testing gap 91, Ze correctness gap 17 | lns: required by rfc/short/rfc4861.md |

### Level 3

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 1877](../../rfc1877/) | PPP Internet Protocol Control Protocol Extensions for Name Server Addresses | no rows | no rows | lns: required by rfc/short/rfc1877.md |
| [RFC 2759](../../rfc2759/) | Microsoft PPP CHAP Extensions, Version 2 | Ze proven 4, Ze correctness gap 3 | Ze MUST proven 4, SHOULD proven 2, SHOULD not decided 2, Ze correctness gap 3 | lns: required by rfc/short/rfc2759.md |

### Level 4

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 5072](../../rfc5072/) | IP Version 6 over PPP | Ze proven 9, Ze correctness gap 7 | Ze MUST proven 9, SHOULD not decided 12, Ze correctness gap 7 | lns: required by rfc/short/rfc5072.md |

### Level 5

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 2661](../../rfc2661/) | Layer Two Tunneling Protocol "L2TP" | Ze proven 81, OS testing gap 1, Ze correctness gap 12 | Ze MUST proven 81, SHOULD proven 2, SHOULD not decided 6, OS testing gap 1, Ze correctness gap 12 | lns: required by rfc/short/rfc2661.md |

### Level 6

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 2869](../../rfc2869/) (cycle with rfc2867, rfc2868) | RADIUS Extensions | Ze proven 4, Ze correctness gap 4 | Ze MUST proven 4, SHOULD not decided 3, Ze correctness gap 4 | lns: required by rfc/short/rfc2869.md |

### Level 7

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 3162](../../rfc3162/) | RADIUS and IPv6 | Ze proven 3, Ze correctness gap 3 | Ze MUST proven 3, SHOULD not decided 7, Ze correctness gap 3 | lns: required by rfc/short/rfc3162.md |

### Level 8

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 5176](../../rfc5176/) (cycle with rfc3579) | Dynamic Authorization Extensions to Remote Authentication Dial In User Service (RADIUS) | Ze proven 26, Ze correctness gap 4 | Ze MUST proven 26, SHOULD proven 1, Ze correctness gap 4 | lns: required by rfc/short/rfc5176.md |

## Optional (not counted)

RFCs this use case lists as optional, each declared by the Optional use cases row of its own summary. They count in none of the figures above until the owner brings one into scope by moving it to the Use cases row. Their own bars and figures follow.

- **MUST bar**: Ze proven 17, Ze correctness gap 69
- **Overview bar**: Ze MUST proven 17, SHOULD proven 2, SHOULD not decided 114, Ze correctness gap 69

| Figure | Rows |
|---|---|
| RFCs | 7 |
| Not walked | 0 |
| Mandatory register | not complete |
| MUST rows | 86 |
| MUST rows kept (the MUST bar) | 86 |
| Ze proven | 17 |
| OS verified | 0 |
| OS testing gap | 0 |
| Ze correctness gap | 69 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 116 |
| SHOULD proven | 2 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 114 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 1570](../../rfc1570/) | PPP LCP Extensions | Ze correctness gap 22 | SHOULD not decided 16, Ze correctness gap 22 | lns: optional in rfc/short/rfc1570.md |
| [RFC 6911](../../rfc6911/) | RADIUS Attributes for IPv6 Access Networks | Ze correctness gap 3 | SHOULD not decided 9, Ze correctness gap 3 | lns: optional in rfc/short/rfc6911.md |
| [RFC 4818](../../rfc4818/) | RADIUS Delegated-IPv6-Prefix Attribute | Ze correctness gap 3 | SHOULD not decided 3, Ze correctness gap 3 | lns: optional in rfc/short/rfc4818.md |
| [RFC 2867](../../rfc2867/) (cycle with rfc2868, rfc2869) | RADIUS Accounting Modifications for Tunnel Protocol Support | Ze correctness gap 1 | SHOULD not decided 16, Ze correctness gap 1 | lns: optional in rfc/short/rfc2867.md |
| [RFC 2868](../../rfc2868/) (cycle with rfc2867, rfc2869) | RADIUS Attributes for Tunnel Protocol Support | Ze correctness gap 18 | SHOULD not decided 43, Ze correctness gap 18 | lns: optional in rfc/short/rfc2868.md |
| [RFC 3145](../../rfc3145/) | L2TP Disconnect Cause Information | Ze correctness gap 8 | SHOULD not decided 10, Ze correctness gap 8 | lns: optional in rfc/short/rfc3145.md |
| [RFC 3579](../../rfc3579/) (cycle with rfc5176) | RADIUS (Remote Authentication Dial In User Service) Support For Extensible Authentication Protocol (EAP) | Ze proven 17, Ze correctness gap 14 | Ze MUST proven 17, SHOULD proven 2, SHOULD not decided 17, Ze correctness gap 14 | lns: optional in rfc/short/rfc3579.md |
