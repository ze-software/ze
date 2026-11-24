# FTTx edge: customers to the internet over L2TP, local DNS for the LAN

Release profile. The RFCs this use case requires, each declared by the Use cases row of its own summary. RFCs it lists as optional follow in their own table and never enter its figures. Before release, every mandatory requirement of every required RFC must have a test that has been shown to fail when the behavior it checks is deliberately broken.

[All profiles](../) · [All RFCs](../../)

Figures as of the last build, read from [data/rfc/profiles/fttx-edge.json](../../../../data/rfc/profiles/fttx-edge.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze verified 422, Tests awaiting verification 48, OS testing gap 120, Not assessed 1, Not built 184
- **Overview bar**: Ze MUST verified 422, SHOULD verified 14, Not owed 2, Tests awaiting verification 49, OS testing gap 120, Not assessed 339, Not built 187

| Figure | Rows |
|---|---|
| RFCs | 20 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 777 |
| MUST rows kept (the MUST bar) | 775 |
| Ze verified | 422 |
| OS verified | 0 |
| Tests awaiting verification | 48 |
| OS testing gap | 120 |
| Exclusion unchecked | 0 |
| Not assessed | 1 |
| Not built | 184 |
| MUST rows not owed | 2 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 2 |
| SHOULD / MAY rows | 356 |
| SHOULD verified | 14 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 1 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 338 |
| SHOULD not built | 3 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

## Tree

The RFCs are listed with the standards they build on first. An RFC appears one level after the highest level of its dependencies; RFCs that refer back to each other share a level. Each RFC has the same two coverage bars as the group above. An RFC whose requirement list has not been reviewed shows "requirements not reviewed" instead of a bar. The last column names the summary whose Use cases row declares the RFC: a normative citation never adds one.

### Level 0

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 1661](../../rfc1661/) | The Point-to-Point Protocol (PPP) | Ze verified 50, Tests awaiting verification 13, OS testing gap 2, Not built 11 | Ze MUST verified 50, SHOULD verified 3, Tests awaiting verification 14, OS testing gap 2, Not assessed 21, Not built 11 | fttx-edge: required by rfc/short/rfc1661.md |

### Level 1

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 1994](../../rfc1994/) | PPP Challenge Handshake Authentication Protocol (CHAP) | Ze verified 11, Tests awaiting verification 3, Not built 2 | Ze MUST verified 11, SHOULD verified 2, Tests awaiting verification 3, Not assessed 9, Not built 2 | fttx-edge: required by rfc/short/rfc1994.md |
| [RFC 2516](../../rfc2516/) | A Method for Transmitting PPP Over Ethernet (PPPoE) | Ze verified 25, Tests awaiting verification 7, OS testing gap 1, Not built 4 | Ze MUST verified 25, Tests awaiting verification 7, OS testing gap 1, Not assessed 3, Not built 4 | fttx-edge: required by rfc/short/rfc2516.md |
| [RFC 4787](../../rfc4787/) | Network Address Translation (NAT) Behavioral Requirements for Unicast UDP | Not built 13 | SHOULD verified 2, Not assessed 14, Not built 13 | fttx-edge: required by rfc/short/rfc4787.md |

### Level 2

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 1035](../../rfc1035/) (cycle with rfc1034, rfc1122, rfc1123, rfc1191, rfc1195, rfc1332, rfc1350, rfc1812, rfc2131, rfc2132, rfc4291, rfc4443, rfc4861, rfc5905, rfc8200, rfc8201, rfc9293) | Domain Names - Implementation and Specification | Ze verified 30, Not assessed 1 | Ze MUST verified 30, Not owed 1, Not assessed 7 | fttx-edge: required by rfc/short/rfc1035.md |
| [RFC 1332](../../rfc1332/) (cycle with rfc1034, rfc1035, rfc1122, rfc1123, rfc1191, rfc1195, rfc1350, rfc1812, rfc2131, rfc2132, rfc4291, rfc4443, rfc4861, rfc5905, rfc8200, rfc8201, rfc9293) | The PPP Internet Protocol Control Protocol (IPCP) | Ze verified 8 | Ze MUST verified 8, Not assessed 6 | fttx-edge: required by rfc/short/rfc1332.md |
| [RFC 2131](../../rfc2131/) (cycle with rfc1034, rfc1035, rfc1122, rfc1123, rfc1191, rfc1195, rfc1332, rfc1350, rfc1812, rfc2132, rfc4291, rfc4443, rfc4861, rfc5905, rfc8200, rfc8201, rfc9293) | Dynamic Host Configuration Protocol | Ze verified 21, Tests awaiting verification 13, OS testing gap 9, Not built 28 | Ze MUST verified 21, SHOULD verified 3, Tests awaiting verification 13, OS testing gap 9, Not assessed 35, Not built 28 | fttx-edge: required by rfc/short/rfc2131.md |
| [RFC 2132](../../rfc2132/) (cycle with rfc1034, rfc1035, rfc1122, rfc1123, rfc1191, rfc1195, rfc1332, rfc1350, rfc1812, rfc2131, rfc4291, rfc4443, rfc4861, rfc5905, rfc8200, rfc8201, rfc9293) | DHCP Options and BOOTP Vendor Extensions | Ze verified 1, Tests awaiting verification 7, Not built 26 | Ze MUST verified 1, Tests awaiting verification 7, Not assessed 34, Not built 26 | fttx-edge: required by rfc/short/rfc2132.md |
| [RFC 4861](../../rfc4861/) (cycle with rfc1034, rfc1035, rfc1122, rfc1123, rfc1191, rfc1195, rfc1332, rfc1350, rfc1812, rfc2131, rfc2132, rfc4291, rfc4443, rfc5905, rfc8200, rfc8201, rfc9293) | Neighbor Discovery for IP version 6 (IPv6) | Ze verified 30, OS testing gap 91, Not built 17 | Ze MUST verified 30, OS testing gap 91, Not assessed 79, Not built 17 | fttx-edge: required by rfc/short/rfc4861.md |

### Level 3

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 1877](../../rfc1877/) | PPP Internet Protocol Control Protocol Extensions for Name Server Addresses | no rows | no rows | fttx-edge: required by rfc/short/rfc1877.md |
| [RFC 2181](../../rfc2181/) | Clarifications to the DNS Specification | Ze verified 21, Not built 1 | Ze MUST verified 21, SHOULD verified 1, Not owed 1, Not assessed 46, Not built 1 | fttx-edge: required by rfc/short/rfc2181.md |
| [RFC 3022](../../rfc3022/) | Traditional IP Network Address Translator (Traditional NAT) | no rows | no rows | fttx-edge: required by rfc/short/rfc3022.md |
| [RFC 4862](../../rfc4862/) (cycle with rfc2464) | IPv6 Stateless Address Autoconfiguration | OS testing gap 6, Not built 10 | OS testing gap 6, Not assessed 14, Not built 10 | fttx-edge: required by rfc/short/rfc4862.md |
| [RFC 5382](../../rfc5382/) | NAT Behavioral Requirements for TCP | Not built 11 | Not assessed 11, Not built 11 | fttx-edge: required by rfc/short/rfc5382.md |

### Level 4

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 2308](../../rfc2308/) | Negative Caching of DNS Queries (DNS NCACHE) | Ze verified 8, Not built 4 | Ze MUST verified 8, Not assessed 4, Not built 6 | fttx-edge: required by rfc/short/rfc2308.md |
| [RFC 5072](../../rfc5072/) | IP Version 6 over PPP | Ze verified 12, Tests awaiting verification 2, Not built 2 | Ze MUST verified 12, Tests awaiting verification 2, Not assessed 12, Not built 2 | fttx-edge: required by rfc/short/rfc5072.md |
| [RFC 9915](../../rfc9915/) | Dynamic Host Configuration Protocol for IPv6 (DHCPv6) | Ze verified 85, OS testing gap 3, Not built 38 | Ze MUST verified 85, OS testing gap 3, Not built 38 | fttx-edge: required by rfc/short/rfc9915.md |

### Level 5

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 2661](../../rfc2661/) | Layer Two Tunneling Protocol "L2TP" | Ze verified 89, Tests awaiting verification 3, OS testing gap 1, Not built 2 | Ze MUST verified 89, SHOULD verified 3, Tests awaiting verification 3, OS testing gap 1, Not assessed 5, Not built 2 | fttx-edge: required by rfc/short/rfc2661.md |

### Level 7

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 5625](../../rfc5625/) | DNS Proxy Implementation Guidelines | Ze verified 12 | Ze MUST verified 12, Not assessed 15 | fttx-edge: required by rfc/short/rfc5625.md |
| [RFC 7084](../../rfc7084/) | Basic Requirements for IPv6 Customer Edge Routers | Ze verified 19, OS testing gap 7, Not built 15 | Ze MUST verified 19, OS testing gap 7, Not assessed 24, Not built 16 | fttx-edge: required by rfc/short/rfc7084.md |

## Optional (not counted)

RFCs this use case lists as optional, each declared by the Optional use cases row of its own summary. They count in none of the figures above until the owner brings one into scope by moving it to the Use cases row. Their own bars and figures follow.

- **MUST bar**: Ze verified 9, Tests awaiting verification 10, Not assessed 59, Not built 87
- **Overview bar**: Ze MUST verified 9, SHOULD verified 2, Not owed 27, Tests awaiting verification 10, Not assessed 123, Not built 87

| Figure | Rows |
|---|---|
| RFCs | 6 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 192 |
| MUST rows kept (the MUST bar) | 165 |
| Ze verified | 9 |
| OS verified | 0 |
| Tests awaiting verification | 10 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 59 |
| Not built | 87 |
| MUST rows not owed | 27 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 27 |
| SHOULD / MAY rows | 66 |
| SHOULD verified | 2 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 64 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 4638](../../rfc4638/) | Accommodating a Maximum Transit Unit/Maximum Receive Unit (MTU/MRU) Greater Than 1492 in the Point-to-Point Protocol over Ethernet (PPPoE) | Not built 6 | Not assessed 3, Not built 6 | fttx-edge: optional in rfc/short/rfc4638.md |
| [RFC 5905](../../rfc5905/) (cycle with rfc1034, rfc1035, rfc1122, rfc1123, rfc1191, rfc1195, rfc1332, rfc1350, rfc1812, rfc2131, rfc2132, rfc4291, rfc4443, rfc4861, rfc8200, rfc8201, rfc9293) | Network Time Protocol Version 4: Protocol and Algorithms Specification | Not built 8 | Not built 8 | fttx-edge: optional in rfc/short/rfc5905.md |
| [RFC 8106](../../rfc8106/) | IPv6 Router Advertisement Options for DNS Configuration | Ze verified 5, Not built 6 | Ze MUST verified 5, SHOULD verified 2, Not assessed 11, Not built 6 | fttx-edge: optional in rfc/short/rfc8106.md |
| [RFC 4033](../../rfc4033/) (cycle with rfc4034, rfc4035) | DNS Security Introduction and Requirements | no rows | no rows | fttx-edge: optional in rfc/short/rfc4033.md |
| [RFC 4034](../../rfc4034/) (cycle with rfc4033, rfc4035) | Resource Records for the DNS Security Extensions | Not assessed 59 | Not assessed 59 | fttx-edge: optional in rfc/short/rfc4034.md |
| [RFC 4035](../../rfc4035/) (cycle with rfc4033, rfc4034) | Protocol Modifications for the DNS Security Extensions | Ze verified 4, Tests awaiting verification 10, Not built 67 | Ze MUST verified 4, Not owed 27, Tests awaiting verification 10, Not assessed 50, Not built 67 | fttx-edge: optional in rfc/short/rfc4035.md |
