# PPP access

Technology profile. PPP, PPPoE, and L2TP subscriber access. The RFCs grouped under ppp-access, showing mandatory requirement coverage and overall coverage for the group and each RFC.

[All profiles](../) · [All RFCs](../../)

Figures as of the last build, read from [data/rfc/profiles/ppp-access.json](../../../../data/rfc/profiles/ppp-access.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze proven 190, Proof pending 42, OS testing gap 4, Not assessed 30, Not built 31
- **Overview bar**: Ze MUST proven 190, SHOULD proven 6, Proof pending 43, OS testing gap 4, Not assessed 120, Not built 31

| Figure | Rows |
|---|---|
| RFCs | 11 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 297 |
| MUST rows kept (the MUST bar) | 297 |
| Ze proven | 190 |
| OS verified | 0 |
| Proof pending | 42 |
| OS testing gap | 4 |
| Exclusion unchecked | 0 |
| Not assessed | 30 |
| Not built | 31 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 97 |
| SHOULD proven | 6 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD proof pending | 1 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 90 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze proven means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Proof pending counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

## Tree

The RFCs are listed with the standards they build on first. An RFC appears one level after the highest level of its dependencies; RFCs that refer back to each other share a level. Each RFC has the same two coverage bars as the group above. An RFC whose requirement list has not been reviewed shows "requirements not reviewed" instead of a bar.

### Level 0

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 1661](../../rfc1661/) | The Point-to-Point Protocol (PPP) | Ze proven 47, Proof pending 14, OS testing gap 2, Not built 13 | Ze MUST proven 47, SHOULD proven 2, Proof pending 15, OS testing gap 2, Not assessed 22, Not built 13 |

### Level 1

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 1570](../../rfc1570/) (cycle with rfc1662) | PPP LCP Extensions | Not assessed 22 | Not assessed 38 |
| [RFC 1994](../../rfc1994/) | PPP Challenge Handshake Authentication Protocol (CHAP) | Ze proven 6, Proof pending 8, Not built 2 | Ze MUST proven 6, SHOULD proven 2, Proof pending 8, Not assessed 9, Not built 2 |
| [RFC 2516](../../rfc2516/) | A Method for Transmitting PPP Over Ethernet (PPPoE) | Ze proven 24, Proof pending 8, OS testing gap 1, Not built 4 | Ze MUST proven 24, Proof pending 8, OS testing gap 1, Not assessed 3, Not built 4 |

### Level 2

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 1332](../../rfc1332/) (cycle with rfc1034, rfc1035, rfc1122, rfc1123, rfc1191, rfc1195, rfc1350, rfc1812, rfc2131, rfc2132, rfc4291, rfc4443, rfc4861, rfc5905, rfc8200, rfc8201, rfc9293) | The PPP Internet Protocol Control Protocol (IPCP) | Ze proven 5, Proof pending 3 | Ze MUST proven 5, Proof pending 3, Not assessed 6 |
| [RFC 1334](../../rfc1334/) | PPP Authentication Protocols | Ze proven 10, Proof pending 3 | Ze MUST proven 10, Proof pending 3, Not assessed 3 |
| [RFC 4638](../../rfc4638/) | Accommodating a Maximum Transit Unit/Maximum Receive Unit (MTU/MRU) Greater Than 1492 in the Point-to-Point Protocol over Ethernet (PPPoE) | Not built 6 | Not assessed 3, Not built 6 |

### Level 3

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 1877](../../rfc1877/) | PPP Internet Protocol Control Protocol Extensions for Name Server Addresses | no rows | no rows |

### Level 4

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 5072](../../rfc5072/) | IP Version 6 over PPP | Ze proven 9, Proof pending 3, Not built 4 | Ze MUST proven 9, Proof pending 3, Not assessed 12, Not built 4 |

### Level 5

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 2661](../../rfc2661/) | Layer Two Tunneling Protocol "L2TP" | Ze proven 89, Proof pending 3, OS testing gap 1, Not built 2 | Ze MUST proven 89, SHOULD proven 2, Proof pending 3, OS testing gap 1, Not assessed 6, Not built 2 |

### Level 6

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 3145](../../rfc3145/) | L2TP Disconnect Cause Information | Not assessed 8 | Not assessed 18 |
