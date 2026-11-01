# RPKI

Technology aggregate. RPKI, origin validation, and RPKI to router. The RFCs whose summary names the rpki aggregate, with the MUST bar and the overview bar of the set and of each RFC.

[All profiles](../) · [All RFCs](../../)

Use case: [Transit Edge with RPKI](../../../../use-cases/transit-edge-rpki/)

Figures as of the last build, read from [data/rfc/profiles/rpki.json](../../../../data/rfc/profiles/rpki.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze proven 13, Ze correctness gap 124
- **Overview bar**: Ze MUST proven 13, SHOULD proven 6, Not owed 5, SHOULD not decided 74, Ze correctness gap 124

| Figure | Rows |
|---|---|
| RFCs | 9 |
| Not walked | 1 |
| Mandatory register | not complete |
| MUST rows | 142 |
| MUST rows kept (the MUST bar) | 137 |
| Ze proven | 13 |
| OS verified | 0 |
| OS testing gap | 0 |
| Ze correctness gap | 124 |
| MUST rows not owed | 5 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 5 |
| SHOULD / MAY rows | 80 |
| SHOULD proven | 6 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 74 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

## Tree

Roots first: an RFC sits one level above the highest level of the RFCs it builds on, and the RFCs of one citation cycle share a level. Each RFC carries the same two bars as the set above, from its own data file; an RFC that is not walked shows "not walked" and no bar.

### Level 3

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 9582](../../rfc9582/) | A Profile for Route Origin Authorizations (ROAs) | Ze correctness gap 16 | SHOULD not decided 6, Ze correctness gap 16 |

### Level 4

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 6482](../../rfc6482/) | A Profile for Route Origin Authorizations (ROAs) | Ze correctness gap 9 | SHOULD not decided 1, Ze correctness gap 9 |

### Level 7

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 6811](../../rfc6811/) | BGP Prefix Origin Validation | Ze proven 1, Ze correctness gap 4 | Ze MUST proven 1, SHOULD not decided 4, Ze correctness gap 4 |

### Level 8

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 6810](../../rfc6810/) | The Resource Public Key Infrastructure (RPKI) to Router Protocol | Ze correctness gap 38 | Not owed 2, SHOULD not decided 28, Ze correctness gap 38 |
| [RFC 8097](../../rfc8097/) | BGP Prefix Origin Validation State Extended Community | Ze correctness gap 5 | SHOULD not decided 5, Ze correctness gap 5 |
| [RFC 8893](../../rfc8893/) | Resource Public Key Infrastructure (RPKI) Origin Validation for BGP Export | Ze correctness gap 4 | SHOULD not decided 2, Ze correctness gap 4 |
| [RFC 9319](../../rfc9319/) | The Use of maxLength in the Resource Public Key Infrastructure (RPKI) | Ze correctness gap 4 | SHOULD not decided 7, Ze correctness gap 4 |

### Level 9

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 8210](../../rfc8210/) | The Resource Public Key Infrastructure (RPKI) to Router Protocol, Version 1 | Ze proven 12, Ze correctness gap 44 | Ze MUST proven 12, SHOULD proven 6, Not owed 3, SHOULD not decided 21, Ze correctness gap 44 |

### Level 10

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [DRAFT-IETF-SIDROPS-8210BIS](../../draft-ietf-sidrops-8210bis/) | The Resource Public Key Infrastructure (RPKI) to Router Protocol, Version 2 | not walked | not walked |
