# Route archival

Technology aggregate. MRT routing information export. The RFCs whose summary names the route-archival aggregate, with the MUST bar and the overview bar of the set and of each RFC.

[All profiles](../) · [All RFCs](../../)

Figures as of the last build, read from [data/rfc/profiles/route-archival.json](../../../../data/rfc/profiles/route-archival.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze proven 11, Ze correctness gap 12
- **Overview bar**: Ze MUST proven 11, SHOULD not decided 13, Ze correctness gap 12

| Figure | Rows |
|---|---|
| RFCs | 3 |
| Not walked | 0 |
| Mandatory register | not complete |
| MUST rows | 23 |
| MUST rows kept (the MUST bar) | 23 |
| Ze proven | 11 |
| OS verified | 0 |
| OS testing gap | 0 |
| Ze correctness gap | 12 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 13 |
| SHOULD proven | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 13 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

## Tree

Roots first: an RFC sits one level above the highest level of the RFCs it builds on, and the RFCs of one citation cycle share a level. Each RFC carries the same two bars as the set above, from its own data file; an RFC that is not walked shows "not walked" and no bar.

### Level 10

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 6396](../../rfc6396/) | Multi-Threaded Routing Toolkit (MRT) Routing Information Export Format | Ze proven 6, Ze correctness gap 7 | Ze MUST proven 6, SHOULD not decided 12, Ze correctness gap 7 |

### Level 11

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 6397](../../rfc6397/) | Multi-Threaded Routing Toolkit (MRT) Border Gateway Protocol (BGP) Routing Information Export Format with Geo-Location Extensions | Ze correctness gap 4 | SHOULD not decided 1, Ze correctness gap 4 |
| [RFC 8050](../../rfc8050/) | Multi-Threaded Routing Toolkit (MRT) Routing Information Export Format with BGP Additional Path Extensions | Ze proven 5, Ze correctness gap 1 | Ze MUST proven 5, Ze correctness gap 1 |
