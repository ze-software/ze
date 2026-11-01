# BGP transport security

Technology aggregate. BGP session transport security. The RFCs whose summary names the bgp-transport-security aggregate, with the MUST bar and the overview bar of the set and of each RFC.

[All profiles](../) · [All RFCs](../../)

Figures as of the last build, read from [data/rfc/profiles/bgp-transport-security.json](../../../../data/rfc/profiles/bgp-transport-security.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze proven 12
- **Overview bar**: Ze MUST proven 12, SHOULD proven 1, SHOULD not decided 6

| Figure | Rows |
|---|---|
| RFCs | 3 |
| Not walked | 1 |
| Mandatory register | not complete |
| MUST rows | 12 |
| MUST rows kept (the MUST bar) | 12 |
| Ze proven | 12 |
| OS verified | 0 |
| OS testing gap | 0 |
| Ze correctness gap | 0 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 7 |
| SHOULD proven | 1 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 6 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

## Tree

Roots first: an RFC sits one level above the highest level of the RFCs it builds on, and the RFCs of one citation cycle share a level. Each RFC carries the same two bars as the set above, from its own data file; an RFC that is not walked shows "not walked" and no bar.

### Level 0

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 2385](../../rfc2385/) | Protection of BGP Sessions via the TCP MD5 Signature Option | Ze proven 8 | Ze MUST proven 8, SHOULD not decided 1 |

### Level 5

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 5082](../../rfc5082/) | The Generalized TTL Security Mechanism (GTSM) | Ze proven 4 | Ze MUST proven 4, SHOULD proven 1, SHOULD not decided 5 |

### No level: the corpus holds no text of the RFC

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 5925](../../rfc5925/) | The TCP Authentication Option | not walked | not walked |
