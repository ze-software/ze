# BGP monitoring

Technology aggregate. BGP Monitoring Protocol (BMP). The RFCs whose summary names the bgp-monitoring aggregate, with the MUST bar and the overview bar of the set and of each RFC.

[All profiles](../) · [All RFCs](../../)

Figures as of the last build, read from [data/rfc/profiles/bgp-monitoring.json](../../../../data/rfc/profiles/bgp-monitoring.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze proven 36, Ze correctness gap 48
- **Overview bar**: Ze MUST proven 36, SHOULD proven 3, SHOULD not decided 12, Ze correctness gap 48

| Figure | Rows |
|---|---|
| RFCs | 4 |
| Not walked | 0 |
| Mandatory register | not complete |
| MUST rows | 84 |
| MUST rows kept (the MUST bar) | 84 |
| Ze proven | 36 |
| OS verified | 0 |
| OS testing gap | 0 |
| Ze correctness gap | 48 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 15 |
| SHOULD proven | 3 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 12 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

## Tree

Roots first: an RFC sits one level above the highest level of the RFCs it builds on, and the RFCs of one citation cycle share a level. Each RFC carries the same two bars as the set above, from its own data file; an RFC that is not walked shows "not walked" and no bar.

### Level 6

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 8522](../../rfc8522/) | Looking Glass Command Set | Ze correctness gap 18 | SHOULD not decided 5, Ze correctness gap 18 |

### Level 8

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 7854](../../rfc7854/) | BGP Monitoring Protocol (BMP) | Ze proven 25, Ze correctness gap 14 | Ze MUST proven 25, SHOULD proven 3, SHOULD not decided 6, Ze correctness gap 14 |

### Level 9

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 8671](../../rfc8671/) | Support for Adj-RIB-Out in the BGP Monitoring Protocol (BMP) | Ze proven 2, Ze correctness gap 9 | Ze MUST proven 2, SHOULD not decided 1, Ze correctness gap 9 |
| [RFC 9069](../../rfc9069/) | Support for Local RIB in the BGP Monitoring Protocol (BMP) | Ze proven 9, Ze correctness gap 7 | Ze MUST proven 9, Ze correctness gap 7 |
