# BGP-LS

Technology profile. BGP Link-State distribution. The RFCs grouped under bgp-ls, showing mandatory requirement coverage and overall coverage for the group and each RFC.

[All profiles](../) · [All RFCs](../../)

Figures as of the last build, read from [data/rfc/profiles/bgp-ls.json](../../../../data/rfc/profiles/bgp-ls.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze verified 92, Tests awaiting verification 8, Not built 20
- **Overview bar**: Ze MUST verified 92, SHOULD verified 9, Not owed 5, Tests awaiting verification 8, Not assessed 62, Not built 22

| Figure | Rows |
|---|---|
| RFCs | 6 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 125 |
| MUST rows kept (the MUST bar) | 120 |
| Ze verified | 92 |
| OS verified | 0 |
| Tests awaiting verification | 8 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 20 |
| MUST rows not owed | 5 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 5 |
| SHOULD / MAY rows | 73 |
| SHOULD verified | 9 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 62 |
| SHOULD not built | 2 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

## Tree

The RFCs are listed with the standards they build on first. An RFC appears one level after the highest level of its dependencies; RFCs that refer back to each other share a level. Each RFC has the same two coverage bars as the group above. An RFC whose requirement list has not been reviewed shows "requirements not reviewed" instead of a bar.

### Level 12

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 9552](../../rfc9552/) | Distribution of Link-State and Traffic Engineering Information Using BGP | Ze verified 44, Tests awaiting verification 7, Not built 5 | Ze MUST verified 44, SHOULD verified 1, Not owed 3, Tests awaiting verification 7, Not assessed 28, Not built 6 |

### Level 13

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 7752](../../rfc7752/) | North-Bound Distribution of Link-State and Traffic Engineering (TE) Information Using BGP | Ze verified 17, Tests awaiting verification 1, Not built 6 | Ze MUST verified 17, SHOULD verified 5, Not owed 2, Tests awaiting verification 1, Not assessed 20, Not built 7 |
| [RFC 8571](../../rfc8571/) | BGP - Link State (BGP-LS) Advertisement of IGP Traffic Engineering Performance Metric Extensions | no rows | no rows |

### Level 14

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 9085](../../rfc9085/) | Border Gateway Protocol - Link State (BGP-LS) Extensions for Segment Routing | Ze verified 9 | Ze MUST verified 9, SHOULD verified 3, Not assessed 2 |

### Level 15

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 9086](../../rfc9086/) | Border Gateway Protocol - Link State (BGP-LS) Extensions for Segment Routing BGP Egress Peer Engineering | Ze verified 7, Not built 4 | Ze MUST verified 7, Not assessed 9, Not built 4 |

### Level 16

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 9514](../../rfc9514/) | Border Gateway Protocol - Link State (BGP-LS) Extensions for Segment Routing over IPv6 (SRv6) | Ze verified 15, Not built 5 | Ze MUST verified 15, Not assessed 3, Not built 5 |
