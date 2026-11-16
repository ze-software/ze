# L2TP network server

Release profile. The RFCs this use case requires, each declared by the Use cases row of its own summary. RFCs it lists as optional follow in their own table and never enter its figures. Before release, every mandatory requirement of every required RFC must have a test that has been shown to fail when the behavior it checks is deliberately broken.

[All profiles](../) · [All RFCs](../../)

Figures as of the last build, read from [data/rfc/profiles/lns.json](../../../../data/rfc/profiles/lns.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze verified 269, Tests awaiting verification 55, OS testing gap 94, Not built 41
- **Overview bar**: Ze MUST verified 269, SHOULD verified 9, Tests awaiting verification 56, OS testing gap 94, Not assessed 151, Not built 41

| Figure | Rows |
|---|---|
| RFCs | 14 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 459 |
| MUST rows kept (the MUST bar) | 459 |
| Ze verified | 269 |
| OS verified | 0 |
| Tests awaiting verification | 55 |
| OS testing gap | 94 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 41 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 161 |
| SHOULD verified | 9 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 1 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 151 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

## Tree

The RFCs are listed with the standards they build on first. An RFC appears one level after the highest level of its dependencies; RFCs that refer back to each other share a level. Each RFC has the same two coverage bars as the group above. An RFC whose requirement list has not been reviewed shows "requirements not reviewed" instead of a bar. The last column names the summary whose Use cases row declares the RFC: a normative citation never adds one.

### Level 0

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 1661](../../rfc1661/) | The Point-to-Point Protocol (PPP) | Ze verified 46, Tests awaiting verification 15, OS testing gap 2, Not built 13 | Ze MUST verified 46, SHOULD verified 2, Tests awaiting verification 16, OS testing gap 2, Not assessed 22, Not built 13 | lns: required by rfc/short/rfc1661.md |

### Level 1

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 1994](../../rfc1994/) | PPP Challenge Handshake Authentication Protocol (CHAP) | Ze verified 6, Tests awaiting verification 8, Not built 2 | Ze MUST verified 6, SHOULD verified 2, Tests awaiting verification 8, Not assessed 9, Not built 2 | lns: required by rfc/short/rfc1994.md |
| [RFC 2865](../../rfc2865/) (cycle with rfc2866) | Remote Authentication Dial In User Service (RADIUS) | Ze verified 27, Tests awaiting verification 4 | Ze MUST verified 27, Tests awaiting verification 4, Not assessed 1 | lns: required by rfc/short/rfc2865.md |
| [RFC 2866](../../rfc2866/) (cycle with rfc2865) | RADIUS Accounting | Ze verified 8, Tests awaiting verification 7 | Ze MUST verified 8, Tests awaiting verification 7, Not assessed 1 | lns: required by rfc/short/rfc2866.md |

### Level 2

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 1332](../../rfc1332/) (cycle with rfc1034, rfc1035, rfc1122, rfc1123, rfc1191, rfc1195, rfc1350, rfc1812, rfc2131, rfc2132, rfc4291, rfc4443, rfc4861, rfc5905, rfc8200, rfc8201, rfc9293) | The PPP Internet Protocol Control Protocol (IPCP) | Ze verified 5, Tests awaiting verification 3 | Ze MUST verified 5, Tests awaiting verification 3, Not assessed 6 | lns: required by rfc/short/rfc1332.md |
| [RFC 1334](../../rfc1334/) | PPP Authentication Protocols | Ze verified 10, Tests awaiting verification 3 | Ze MUST verified 10, Tests awaiting verification 3, Not assessed 3 | lns: required by rfc/short/rfc1334.md |
| [RFC 4861](../../rfc4861/) (cycle with rfc1034, rfc1035, rfc1122, rfc1123, rfc1191, rfc1195, rfc1332, rfc1350, rfc1812, rfc2131, rfc2132, rfc4291, rfc4443, rfc5905, rfc8200, rfc8201, rfc9293) | Neighbor Discovery for IP version 6 (IPv6) | Ze verified 30, OS testing gap 91, Not built 17 | Ze MUST verified 30, OS testing gap 91, Not assessed 79, Not built 17 | lns: required by rfc/short/rfc4861.md |

### Level 3

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 1877](../../rfc1877/) | PPP Internet Protocol Control Protocol Extensions for Name Server Addresses | no rows | no rows | lns: required by rfc/short/rfc1877.md |
| [RFC 2759](../../rfc2759/) | Microsoft PPP CHAP Extensions, Version 2 | Ze verified 4, Tests awaiting verification 3 | Ze MUST verified 4, SHOULD verified 2, Tests awaiting verification 3, Not assessed 2 | lns: required by rfc/short/rfc2759.md |

### Level 4

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 5072](../../rfc5072/) | IP Version 6 over PPP | Ze verified 9, Tests awaiting verification 3, Not built 4 | Ze MUST verified 9, Tests awaiting verification 3, Not assessed 12, Not built 4 | lns: required by rfc/short/rfc5072.md |

### Level 5

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 2661](../../rfc2661/) | Layer Two Tunneling Protocol "L2TP" | Ze verified 89, Tests awaiting verification 3, OS testing gap 1, Not built 2 | Ze MUST verified 89, SHOULD verified 2, Tests awaiting verification 3, OS testing gap 1, Not assessed 6, Not built 2 | lns: required by rfc/short/rfc2661.md |

### Level 6

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 2869](../../rfc2869/) (cycle with rfc2867, rfc2868) | RADIUS Extensions | Ze verified 6, Tests awaiting verification 2 | Ze MUST verified 6, Tests awaiting verification 2, Not assessed 3 | lns: required by rfc/short/rfc2869.md |

### Level 7

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 3162](../../rfc3162/) | RADIUS and IPv6 | Ze verified 3, Not built 3 | Ze MUST verified 3, Not assessed 7, Not built 3 | lns: required by rfc/short/rfc3162.md |

### Level 8

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 5176](../../rfc5176/) (cycle with rfc3579) | Dynamic Authorization Extensions to Remote Authentication Dial In User Service (RADIUS) | Ze verified 26, Tests awaiting verification 4 | Ze MUST verified 26, SHOULD verified 1, Tests awaiting verification 4 | lns: required by rfc/short/rfc5176.md |

## Optional (not counted)

RFCs this use case lists as optional, each declared by the Optional use cases row of its own summary. They count in none of the figures above until the owner brings one into scope by moving it to the Use cases row. Their own bars and figures follow.

- **MUST bar**: Ze verified 17, Not assessed 86, Not built 14
- **Overview bar**: Ze MUST verified 17, SHOULD verified 4, Not assessed 232, Not built 14

| Figure | Rows |
|---|---|
| RFCs | 8 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 117 |
| MUST rows kept (the MUST bar) | 117 |
| Ze verified | 17 |
| OS verified | 0 |
| Tests awaiting verification | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 86 |
| Not built | 14 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 150 |
| SHOULD verified | 4 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 146 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 1570](../../rfc1570/) (cycle with rfc1662) | PPP LCP Extensions | Not assessed 22 | Not assessed 38 | lns: optional in rfc/short/rfc1570.md |
| [RFC 2548](../../rfc2548/) | Microsoft Vendor-specific RADIUS Attributes | Not assessed 31 | SHOULD verified 2, Not assessed 63 | lns: optional in rfc/short/rfc2548.md |
| [RFC 6911](../../rfc6911/) | RADIUS Attributes for IPv6 Access Networks | Not assessed 3 | Not assessed 12 | lns: optional in rfc/short/rfc6911.md |
| [RFC 4818](../../rfc4818/) | RADIUS Delegated-IPv6-Prefix Attribute | Not assessed 3 | Not assessed 6 | lns: optional in rfc/short/rfc4818.md |
| [RFC 2867](../../rfc2867/) (cycle with rfc2868, rfc2869) | RADIUS Accounting Modifications for Tunnel Protocol Support | Not assessed 1 | Not assessed 17 | lns: optional in rfc/short/rfc2867.md |
| [RFC 2868](../../rfc2868/) (cycle with rfc2867, rfc2869) | RADIUS Attributes for Tunnel Protocol Support | Not assessed 18 | Not assessed 61 | lns: optional in rfc/short/rfc2868.md |
| [RFC 3145](../../rfc3145/) | L2TP Disconnect Cause Information | Not assessed 8 | Not assessed 18 | lns: optional in rfc/short/rfc3145.md |
| [RFC 3579](../../rfc3579/) (cycle with rfc5176) | RADIUS (Remote Authentication Dial In User Service) Support For Extensible Authentication Protocol (EAP) | Ze verified 17, Not built 14 | Ze MUST verified 17, SHOULD verified 2, Not assessed 17, Not built 14 | lns: optional in rfc/short/rfc3579.md |
