# First-hop redundancy

Technology aggregate. First-hop redundancy. The RFCs whose summary names the fhrp aggregate, with the MUST bar and the overview bar of the set and of each RFC.

[All profiles](../) · [All RFCs](../../)

Figures as of the last build, read from [data/rfc/profiles/fhrp.json](../../../../data/rfc/profiles/fhrp.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze proven 117, OS testing gap 3, Ze correctness gap 46
- **Overview bar**: Ze MUST proven 117, SHOULD proven 4, SHOULD not decided 61, OS testing gap 3, Ze correctness gap 46

| Figure | Rows |
|---|---|
| RFCs | 3 |
| Not walked | 0 |
| Mandatory register | not complete |
| MUST rows | 166 |
| MUST rows kept (the MUST bar) | 166 |
| Ze proven | 117 |
| OS verified | 0 |
| OS testing gap | 3 |
| Ze correctness gap | 46 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 65 |
| SHOULD proven | 4 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 61 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

## Tree

Roots first: an RFC sits one level above the highest level of the RFCs it builds on, and the RFCs of one citation cycle share a level. Each RFC carries the same two bars as the set above, from its own data file; an RFC that is not walked shows "not walked" and no bar.

### Level 6

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 9568](../../rfc9568/) | Virtual Router Redundancy Protocol (VRRP) Version 3 for IPv4 and IPv6 | Ze proven 37, Ze correctness gap 29 | Ze MUST proven 37, SHOULD proven 3, SHOULD not decided 25, Ze correctness gap 29 |

### Level 7

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 3768](../../rfc3768/) | Virtual Router Redundancy Protocol (VRRP) | Ze proven 31, Ze correctness gap 10 | Ze MUST proven 31, SHOULD not decided 11, Ze correctness gap 10 |
| [RFC 5798](../../rfc5798/) | Virtual Router Redundancy Protocol (VRRP) Version 3 for IPv4 and IPv6 | Ze proven 49, OS testing gap 3, Ze correctness gap 7 | Ze MUST proven 49, SHOULD proven 1, SHOULD not decided 25, OS testing gap 3, Ze correctness gap 7 |
