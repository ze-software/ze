# FTTx edge: customers to the internet over L2TP, local DNS for the LAN

Release profile. The RFCs this use case requires, each declared by the Use cases row of its own summary. RFCs it lists as optional follow in their own table and never enter its figures. Before release, every mandatory requirement of every required RFC must be proven by a recorded break of its test.

[All profiles](../) · [All RFCs](../../)

Figures as of the last build, read from [data/rfc/profiles/fttx-edge.json](../../../../data/rfc/profiles/fttx-edge.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze proven 342, OS testing gap 120, Ze correctness gap 285
- **Overview bar**: Ze MUST proven 342, SHOULD proven 7, SHOULD not decided 328, OS testing gap 120, Ze correctness gap 285

| Figure | Rows |
|---|---|
| RFCs | 18 |
| Not walked | 0 |
| Mandatory register | not complete |
| MUST rows | 747 |
| MUST rows kept (the MUST bar) | 747 |
| Ze proven | 342 |
| OS verified | 0 |
| OS testing gap | 120 |
| Ze correctness gap | 285 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 335 |
| SHOULD proven | 7 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 328 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

## Tree

Roots first: an RFC sits one level above the highest level of the RFCs it builds on, and the RFCs of one citation cycle share a level. Each RFC carries the same two bars as the set above, from its own data file; an RFC that is not walked shows "not walked" and no bar. The last column names the summary whose Use cases row declares the RFC: a normative citation never adds one.

### Level 0

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 1661](../../rfc1661/) | The Point-to-Point Protocol (PPP) | Ze proven 42, OS testing gap 2, Ze correctness gap 32 | Ze MUST proven 42, SHOULD not decided 25, OS testing gap 2, Ze correctness gap 32 | fttx-edge: required by rfc/short/rfc1661.md |

### Level 1

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 1994](../../rfc1994/) | PPP Challenge Handshake Authentication Protocol (CHAP) | Ze proven 8, Ze correctness gap 8 | Ze MUST proven 8, SHOULD proven 2, SHOULD not decided 9, Ze correctness gap 8 | fttx-edge: required by rfc/short/rfc1994.md |
| [RFC 2516](../../rfc2516/) | A Method for Transmitting PPP Over Ethernet (PPPoE) | Ze proven 25, OS testing gap 1, Ze correctness gap 11 | Ze MUST proven 25, SHOULD not decided 3, OS testing gap 1, Ze correctness gap 11 | fttx-edge: required by rfc/short/rfc2516.md |
| [RFC 4787](../../rfc4787/) | Network Address Translation (NAT) Behavioral Requirements for Unicast UDP | Ze correctness gap 13 | SHOULD not decided 16, Ze correctness gap 13 | fttx-edge: required by rfc/short/rfc4787.md |

### Level 2

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 1035](../../rfc1035/) (cycle with rfc1034, rfc1122, rfc1191, rfc1195, rfc1332, rfc1812, rfc2131, rfc2132, rfc4291, rfc4443, rfc4861, rfc5905, rfc8200, rfc8201, rfc9293) | Domain Names - Implementation and Specification | Ze proven 12, Ze correctness gap 15 | Ze MUST proven 12, SHOULD not decided 6, Ze correctness gap 15 | fttx-edge: required by rfc/short/rfc1035.md |
| [RFC 1332](../../rfc1332/) (cycle with rfc1034, rfc1035, rfc1122, rfc1191, rfc1195, rfc1812, rfc2131, rfc2132, rfc4291, rfc4443, rfc4861, rfc5905, rfc8200, rfc8201, rfc9293) | The PPP Internet Protocol Control Protocol (IPCP) | Ze proven 8 | Ze MUST proven 8, SHOULD not decided 6 | fttx-edge: required by rfc/short/rfc1332.md |
| [RFC 2131](../../rfc2131/) (cycle with rfc1034, rfc1035, rfc1122, rfc1191, rfc1195, rfc1332, rfc1812, rfc2132, rfc4291, rfc4443, rfc4861, rfc5905, rfc8200, rfc8201, rfc9293) | Dynamic Host Configuration Protocol | Ze proven 18, OS testing gap 9, Ze correctness gap 44 | Ze MUST proven 18, SHOULD proven 2, SHOULD not decided 36, OS testing gap 9, Ze correctness gap 44 | fttx-edge: required by rfc/short/rfc2131.md |
| [RFC 2132](../../rfc2132/) (cycle with rfc1034, rfc1035, rfc1122, rfc1191, rfc1195, rfc1332, rfc1812, rfc2131, rfc4291, rfc4443, rfc4861, rfc5905, rfc8200, rfc8201, rfc9293) | DHCP Options and BOOTP Vendor Extensions | Ze proven 1, Ze correctness gap 33 | Ze MUST proven 1, SHOULD not decided 34, Ze correctness gap 33 | fttx-edge: required by rfc/short/rfc2132.md |
| [RFC 4861](../../rfc4861/) (cycle with rfc1034, rfc1035, rfc1122, rfc1191, rfc1195, rfc1332, rfc1812, rfc2131, rfc2132, rfc4291, rfc4443, rfc5905, rfc8200, rfc8201, rfc9293) | Neighbor Discovery for IP version 6 (IPv6) | Ze proven 30, OS testing gap 91, Ze correctness gap 17 | Ze MUST proven 30, SHOULD not decided 79, OS testing gap 91, Ze correctness gap 17 | fttx-edge: required by rfc/short/rfc4861.md |

### Level 3

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 1877](../../rfc1877/) | PPP Internet Protocol Control Protocol Extensions for Name Server Addresses | no rows | no rows | fttx-edge: required by rfc/short/rfc1877.md |
| [RFC 2181](../../rfc2181/) | Clarifications to the DNS Specification | Ze proven 12, Ze correctness gap 11 | Ze MUST proven 12, SHOULD proven 1, SHOULD not decided 46, Ze correctness gap 11 | fttx-edge: required by rfc/short/rfc2181.md |
| [RFC 3022](../../rfc3022/) | Traditional IP Network Address Translator (Traditional NAT) | no rows | no rows | fttx-edge: required by rfc/short/rfc3022.md |
| [RFC 4862](../../rfc4862/) (cycle with rfc2464) | IPv6 Stateless Address Autoconfiguration | OS testing gap 6, Ze correctness gap 10 | SHOULD not decided 14, OS testing gap 6, Ze correctness gap 10 | fttx-edge: required by rfc/short/rfc4862.md |
| [RFC 5382](../../rfc5382/) | NAT Behavioral Requirements for TCP | Ze correctness gap 11 | SHOULD not decided 11, Ze correctness gap 11 | fttx-edge: required by rfc/short/rfc5382.md |

### Level 4

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 5072](../../rfc5072/) | IP Version 6 over PPP | Ze proven 9, Ze correctness gap 7 | Ze MUST proven 9, SHOULD not decided 12, Ze correctness gap 7 | fttx-edge: required by rfc/short/rfc5072.md |
| [RFC 9915](../../rfc9915/) | Dynamic Host Configuration Protocol for IPv6 (DHCPv6) | Ze proven 82, OS testing gap 3, Ze correctness gap 41 | Ze MUST proven 82, OS testing gap 3, Ze correctness gap 41 | fttx-edge: required by rfc/short/rfc9915.md |

### Level 5

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 2661](../../rfc2661/) | Layer Two Tunneling Protocol "L2TP" | Ze proven 81, OS testing gap 1, Ze correctness gap 12 | Ze MUST proven 81, SHOULD proven 2, SHOULD not decided 6, OS testing gap 1, Ze correctness gap 12 | fttx-edge: required by rfc/short/rfc2661.md |

### Level 7

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 7084](../../rfc7084/) | Basic Requirements for IPv6 Customer Edge Routers | Ze proven 14, OS testing gap 7, Ze correctness gap 20 | Ze MUST proven 14, SHOULD not decided 25, OS testing gap 7, Ze correctness gap 20 | fttx-edge: required by rfc/short/rfc7084.md |

## Optional (not counted)

RFCs this use case lists as optional, each declared by the Optional use cases row of its own summary. They count in none of the figures above until the owner brings one into scope by moving it to the Use cases row. Their own bars and figures follow.

- **MUST bar**: Ze proven 4, Ze correctness gap 188
- **Overview bar**: Ze MUST proven 4, SHOULD not decided 66, Ze correctness gap 188

| Figure | Rows |
|---|---|
| RFCs | 6 |
| Not walked | 0 |
| Mandatory register | not complete |
| MUST rows | 192 |
| MUST rows kept (the MUST bar) | 192 |
| Ze proven | 4 |
| OS verified | 0 |
| OS testing gap | 0 |
| Ze correctness gap | 188 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 66 |
| SHOULD proven | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 66 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 4638](../../rfc4638/) | Accommodating a Maximum Transit Unit/Maximum Receive Unit (MTU/MRU) Greater Than 1492 in the Point-to-Point Protocol over Ethernet (PPPoE) | Ze correctness gap 6 | SHOULD not decided 3, Ze correctness gap 6 | fttx-edge: optional in rfc/short/rfc4638.md |
| [RFC 5905](../../rfc5905/) (cycle with rfc1034, rfc1035, rfc1122, rfc1191, rfc1195, rfc1332, rfc1812, rfc2131, rfc2132, rfc4291, rfc4443, rfc4861, rfc8200, rfc8201, rfc9293) | Network Time Protocol Version 4: Protocol and Algorithms Specification | Ze correctness gap 8 | Ze correctness gap 8 | fttx-edge: optional in rfc/short/rfc5905.md |
| [RFC 8106](../../rfc8106/) | IPv6 Router Advertisement Options for DNS Configuration | Ze correctness gap 11 | SHOULD not decided 13, Ze correctness gap 11 | fttx-edge: optional in rfc/short/rfc8106.md |
| [RFC 4033](../../rfc4033/) (cycle with rfc4034, rfc4035) | DNS Security Introduction and Requirements | no rows | no rows | fttx-edge: optional in rfc/short/rfc4033.md |
| [RFC 4034](../../rfc4034/) (cycle with rfc4033, rfc4035) | Resource Records for the DNS Security Extensions | Ze correctness gap 59 | Ze correctness gap 59 | fttx-edge: optional in rfc/short/rfc4034.md |
| [RFC 4035](../../rfc4035/) (cycle with rfc4033, rfc4034) | Protocol Modifications for the DNS Security Extensions | Ze proven 4, Ze correctness gap 104 | Ze MUST proven 4, SHOULD not decided 50, Ze correctness gap 104 | fttx-edge: optional in rfc/short/rfc4035.md |
