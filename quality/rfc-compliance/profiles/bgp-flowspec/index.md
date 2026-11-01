# BGP FlowSpec

Technology aggregate. BGP Flow Specification. The RFCs whose summary names the bgp-flowspec aggregate, with the MUST bar and the overview bar of the set and of each RFC.

[All profiles](../) · [All RFCs](../../)

Figures as of the last build, read from [data/rfc/profiles/bgp-flowspec.json](../../../../data/rfc/profiles/bgp-flowspec.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze proven 26, Ze correctness gap 22
- **Overview bar**: Ze MUST proven 26, SHOULD proven 6, Not owed 1, SHOULD not decided 26, Ze correctness gap 22

| Figure | Rows |
|---|---|
| RFCs | 4 |
| Not walked | 0 |
| Mandatory register | not complete |
| MUST rows | 49 |
| MUST rows kept (the MUST bar) | 48 |
| Ze proven | 26 |
| OS verified | 0 |
| OS testing gap | 0 |
| Ze correctness gap | 22 |
| MUST rows not owed | 1 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 1 |
| SHOULD / MAY rows | 32 |
| SHOULD proven | 6 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 26 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

## Tree

Roots first: an RFC sits one level above the highest level of the RFCs it builds on, and the RFCs of one citation cycle share a level. Each RFC carries the same two bars as the set above, from its own data file; an RFC that is not walked shows "not walked" and no bar.

### Level 8

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 8955](../../rfc8955/) | Dissemination of Flow Specification Rules | Ze proven 13, Ze correctness gap 15 | Ze MUST proven 13, SHOULD proven 6, Not owed 1, SHOULD not decided 14, Ze correctness gap 15 |

### Level 9

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 5575](../../rfc5575/) | Dissemination of Flow Specification Rules | Ze proven 4, Ze correctness gap 5 | Ze MUST proven 4, SHOULD not decided 4, Ze correctness gap 5 |
| [RFC 8956](../../rfc8956/) | Dissemination of Flow Specification Rules for IPv6 | Ze proven 7, Ze correctness gap 2 | Ze MUST proven 7, SHOULD not decided 5, Ze correctness gap 2 |
| [RFC 9117](../../rfc9117/) | Revised Validation Procedure for BGP Flow Specifications | Ze proven 2 | Ze MUST proven 2, SHOULD not decided 3 |
