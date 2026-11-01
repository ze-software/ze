# PPP access

Technology aggregate. PPP, PPPoE, and L2TP subscriber access. The RFCs whose summary names the ppp-access aggregate, with the MUST bar and the overview bar of the set and of each RFC.

[All profiles](../) · [All RFCs](../../)

Figures as of the last build, read from [data/rfc/profiles/ppp-access.json](../../../../data/rfc/profiles/ppp-access.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze proven 186, OS testing gap 4, Ze correctness gap 106
- **Overview bar**: Ze MUST proven 186, SHOULD proven 4, SHOULD not decided 93, OS testing gap 4, Ze correctness gap 106

| Figure | Rows |
|---|---|
| RFCs | 11 |
| Not walked | 0 |
| Mandatory register | not complete |
| MUST rows | 296 |
| MUST rows kept (the MUST bar) | 296 |
| Ze proven | 186 |
| OS verified | 0 |
| OS testing gap | 4 |
| Ze correctness gap | 106 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 97 |
| SHOULD proven | 4 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 93 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

## Tree

Roots first: an RFC sits one level above the highest level of the RFCs it builds on, and the RFCs of one citation cycle share a level. Each RFC carries the same two bars as the set above, from its own data file; an RFC that is not walked shows "not walked" and no bar.

### Level 0

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 1661](../../rfc1661/) | The Point-to-Point Protocol (PPP) | Ze proven 42, OS testing gap 2, Ze correctness gap 32 | Ze MUST proven 42, SHOULD not decided 25, OS testing gap 2, Ze correctness gap 32 |

### Level 1

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 1570](../../rfc1570/) | PPP LCP Extensions | Ze correctness gap 22 | SHOULD not decided 16, Ze correctness gap 22 |
| [RFC 1994](../../rfc1994/) | PPP Challenge Handshake Authentication Protocol (CHAP) | Ze proven 8, Ze correctness gap 8 | Ze MUST proven 8, SHOULD proven 2, SHOULD not decided 9, Ze correctness gap 8 |
| [RFC 2516](../../rfc2516/) | A Method for Transmitting PPP Over Ethernet (PPPoE) | Ze proven 25, OS testing gap 1, Ze correctness gap 11 | Ze MUST proven 25, SHOULD not decided 3, OS testing gap 1, Ze correctness gap 11 |

### Level 2

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 1332](../../rfc1332/) (cycle with rfc1034, rfc1035, rfc1122, rfc1191, rfc1195, rfc1812, rfc2131, rfc2132, rfc4291, rfc4443, rfc4861, rfc5905, rfc8200, rfc8201, rfc9293) | The PPP Internet Protocol Control Protocol (IPCP) | Ze proven 8 | Ze MUST proven 8, SHOULD not decided 6 |
| [RFC 1334](../../rfc1334/) | PPP Authentication Protocols | Ze proven 13 | Ze MUST proven 13, SHOULD not decided 3 |
| [RFC 4638](../../rfc4638/) | Accommodating a Maximum Transit Unit/Maximum Receive Unit (MTU/MRU) Greater Than 1492 in the Point-to-Point Protocol over Ethernet (PPPoE) | Ze correctness gap 6 | SHOULD not decided 3, Ze correctness gap 6 |

### Level 3

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 1877](../../rfc1877/) | PPP Internet Protocol Control Protocol Extensions for Name Server Addresses | no rows | no rows |

### Level 4

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 5072](../../rfc5072/) | IP Version 6 over PPP | Ze proven 9, Ze correctness gap 7 | Ze MUST proven 9, SHOULD not decided 12, Ze correctness gap 7 |

### Level 5

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 2661](../../rfc2661/) | Layer Two Tunneling Protocol "L2TP" | Ze proven 81, OS testing gap 1, Ze correctness gap 12 | Ze MUST proven 81, SHOULD proven 2, SHOULD not decided 6, OS testing gap 1, Ze correctness gap 12 |

### Level 6

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 3145](../../rfc3145/) | L2TP Disconnect Cause Information | Ze correctness gap 8 | SHOULD not decided 10, Ze correctness gap 8 |
