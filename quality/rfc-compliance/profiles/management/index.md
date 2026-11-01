# Management

Technology aggregate. Configuration modeling and management protocols. The RFCs whose summary names the management aggregate, with the MUST bar and the overview bar of the set and of each RFC.

[All profiles](../) · [All RFCs](../../)

Figures as of the last build, read from [data/rfc/profiles/management.json](../../../../data/rfc/profiles/management.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze proven 37, Ze correctness gap 62
- **Overview bar**: Ze MUST proven 37, Ze correctness gap 62

| Figure | Rows |
|---|---|
| RFCs | 2 |
| Not walked | 0 |
| Mandatory register | not complete |
| MUST rows | 99 |
| MUST rows kept (the MUST bar) | 99 |
| Ze proven | 37 |
| OS verified | 0 |
| OS testing gap | 0 |
| Ze correctness gap | 62 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 0 |
| SHOULD proven | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 0 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

## Tree

Roots first: an RFC sits one level above the highest level of the RFCs it builds on, and the RFCs of one citation cycle share a level. Each RFC carries the same two bars as the set above, from its own data file; an RFC that is not walked shows "not walked" and no bar.

### Level 0

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 7950](../../rfc7950/) | The YANG 1.1 Data Modeling Language | Ze proven 37, Ze correctness gap 54 | Ze MUST proven 37, Ze correctness gap 54 |

### Level 2

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 5905](../../rfc5905/) (cycle with rfc1034, rfc1035, rfc1122, rfc1191, rfc1195, rfc1332, rfc1812, rfc2131, rfc2132, rfc4291, rfc4443, rfc4861, rfc8200, rfc8201, rfc9293) | Network Time Protocol Version 4: Protocol and Algorithms Specification | Ze correctness gap 8 | Ze correctness gap 8 |
