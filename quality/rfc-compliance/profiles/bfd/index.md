# BFD

Technology aggregate. Bidirectional Forwarding Detection. The RFCs whose summary names the bfd aggregate, with the MUST bar and the overview bar of the set and of each RFC.

[All profiles](../) · [All RFCs](../../)

Figures as of the last build, read from [data/rfc/profiles/bfd.json](../../../../data/rfc/profiles/bfd.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze proven 88, Ze correctness gap 55
- **Overview bar**: Ze MUST proven 88, SHOULD proven 12, SHOULD not decided 57, Ze correctness gap 55

| Figure | Rows |
|---|---|
| RFCs | 5 |
| Not walked | 0 |
| Mandatory register | not complete |
| MUST rows | 143 |
| MUST rows kept (the MUST bar) | 143 |
| Ze proven | 88 |
| OS verified | 0 |
| OS testing gap | 0 |
| Ze correctness gap | 55 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 69 |
| SHOULD proven | 12 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 57 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

## Tree

Roots first: an RFC sits one level above the highest level of the RFCs it builds on, and the RFCs of one citation cycle share a level. Each RFC carries the same two bars as the set above, from its own data file; an RFC that is not walked shows "not walked" and no bar.

### Level 6

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 5880](../../rfc5880/) | Bidirectional Forwarding Detection (BFD) | Ze proven 72, Ze correctness gap 39 | Ze MUST proven 72, SHOULD proven 6, SHOULD not decided 20, Ze correctness gap 39 |

### Level 7

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 5881](../../rfc5881/) (cycle with rfc5882, rfc5883) | Bidirectional Forwarding Detection (BFD) for IPv4 and IPv6 (Single Hop) | Ze proven 13, Ze correctness gap 10 | Ze MUST proven 13, SHOULD not decided 10, Ze correctness gap 10 |
| [RFC 5882](../../rfc5882/) (cycle with rfc5881, rfc5883) | Generic Application of Bidirectional Forwarding Detection (BFD) | Ze correctness gap 3 | SHOULD proven 5, SHOULD not decided 24, Ze correctness gap 3 |
| [RFC 5883](../../rfc5883/) (cycle with rfc5881, rfc5882) | Bidirectional Forwarding Detection (BFD) for Multihop Paths | Ze proven 3, Ze correctness gap 3 | Ze MUST proven 3, SHOULD not decided 1, Ze correctness gap 3 |

### Level 9

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 9384](../../rfc9384/) | A BGP Cease NOTIFICATION Subcode for Bidirectional Forwarding Detection (BFD) | no rows | SHOULD proven 1, SHOULD not decided 2 |
