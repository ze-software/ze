# BGP-LS

Technology aggregate. BGP Link-State distribution. The RFCs whose summary names the bgp-ls aggregate, with the MUST bar and the overview bar of the set and of each RFC.

[All profiles](../) · [All RFCs](../../)

Figures as of the last build, read from [data/rfc/profiles/bgp-ls.json](../../../../data/rfc/profiles/bgp-ls.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze proven 83, Ze correctness gap 41
- **Overview bar**: Ze MUST proven 83, SHOULD proven 4, Not owed 1, SHOULD not decided 69, Ze correctness gap 41

| Figure | Rows |
|---|---|
| RFCs | 6 |
| Not walked | 0 |
| Mandatory register | not complete |
| MUST rows | 125 |
| MUST rows kept (the MUST bar) | 124 |
| Ze proven | 83 |
| OS verified | 0 |
| OS testing gap | 0 |
| Ze correctness gap | 41 |
| MUST rows not owed | 1 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 1 |
| SHOULD / MAY rows | 73 |
| SHOULD proven | 4 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 69 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

## Tree

Roots first: an RFC sits one level above the highest level of the RFCs it builds on, and the RFCs of one citation cycle share a level. Each RFC carries the same two bars as the set above, from its own data file; an RFC that is not walked shows "not walked" and no bar.

### Level 12

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 9552](../../rfc9552/) | Distribution of Link-State and Traffic Engineering Information Using BGP | Ze proven 44, Ze correctness gap 14 | Ze MUST proven 44, SHOULD proven 1, Not owed 1, SHOULD not decided 29, Ze correctness gap 14 |

### Level 13

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 7752](../../rfc7752/) | North-Bound Distribution of Link-State and Traffic Engineering (TE) Information Using BGP | Ze proven 8, Ze correctness gap 18 | Ze MUST proven 8, SHOULD not decided 26, Ze correctness gap 18 |
| [RFC 8571](../../rfc8571/) | BGP - Link State (BGP-LS) Advertisement of IGP Traffic Engineering Performance Metric Extensions | no rows | no rows |

### Level 14

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 9085](../../rfc9085/) | Border Gateway Protocol - Link State (BGP-LS) Extensions for Segment Routing | Ze proven 9 | Ze MUST proven 9, SHOULD proven 3, SHOULD not decided 2 |

### Level 15

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 9086](../../rfc9086/) | Border Gateway Protocol - Link State (BGP-LS) Extensions for Segment Routing BGP Egress Peer Engineering | Ze proven 7, Ze correctness gap 4 | Ze MUST proven 7, SHOULD not decided 9, Ze correctness gap 4 |

### Level 16

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 9514](../../rfc9514/) | Border Gateway Protocol - Link State (BGP-LS) Extensions for Segment Routing over IPv6 (SRv6) | Ze proven 15, Ze correctness gap 5 | Ze MUST proven 15, SHOULD not decided 3, Ze correctness gap 5 |
