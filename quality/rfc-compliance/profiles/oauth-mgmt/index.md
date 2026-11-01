# OAuth for management

Technology aggregate. OAuth 2.0 for management access. The RFCs whose summary names the oauth-mgmt aggregate, with the MUST bar and the overview bar of the set and of each RFC.

[All profiles](../) · [All RFCs](../../)

Figures as of the last build, read from [data/rfc/profiles/oauth-mgmt.json](../../../../data/rfc/profiles/oauth-mgmt.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze proven 18, Ze correctness gap 41
- **Overview bar**: Ze MUST proven 18, SHOULD proven 2, SHOULD not decided 11, Ze correctness gap 41

| Figure | Rows |
|---|---|
| RFCs | 3 |
| Not walked | 0 |
| Mandatory register | not complete |
| MUST rows | 59 |
| MUST rows kept (the MUST bar) | 59 |
| Ze proven | 18 |
| OS verified | 0 |
| OS testing gap | 0 |
| Ze correctness gap | 41 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 13 |
| SHOULD proven | 2 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 11 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

## Tree

Roots first: an RFC sits one level above the highest level of the RFCs it builds on, and the RFCs of one citation cycle share a level. Each RFC carries the same two bars as the set above, from its own data file; an RFC that is not walked shows "not walked" and no bar.

### Level 0

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 8707](../../rfc8707/) | Resource Indicators for OAuth 2.0 | Ze correctness gap 2 | SHOULD not decided 5, Ze correctness gap 2 |

### Level 7

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 8414](../../rfc8414/) | OAuth 2.0 Authorization Server Metadata | Ze proven 9, Ze correctness gap 22 | Ze MUST proven 9, SHOULD not decided 2, Ze correctness gap 22 |

### Level 8

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 9728](../../rfc9728/) | OAuth 2.0 Protected Resource Metadata | Ze proven 9, Ze correctness gap 17 | Ze MUST proven 9, SHOULD proven 2, SHOULD not decided 4, Ze correctness gap 17 |
