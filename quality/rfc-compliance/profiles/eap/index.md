# EAP

Technology aggregate. EAP methods and their TLS and CHAP building blocks. The RFCs whose summary names the eap aggregate, with the MUST bar and the overview bar of the set and of each RFC.

[All profiles](../) · [All RFCs](../../)

Figures as of the last build, read from [data/rfc/profiles/eap.json](../../../../data/rfc/profiles/eap.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze proven 119, OS testing gap 2, Ze correctness gap 51
- **Overview bar**: Ze MUST proven 119, SHOULD proven 6, SHOULD not decided 53, OS testing gap 2, Ze correctness gap 51

| Figure | Rows |
|---|---|
| RFCs | 6 |
| Not walked | 1 |
| Mandatory register | not complete |
| MUST rows | 172 |
| MUST rows kept (the MUST bar) | 172 |
| Ze proven | 119 |
| OS verified | 0 |
| OS testing gap | 2 |
| Ze correctness gap | 51 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 59 |
| SHOULD proven | 6 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 53 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

## Tree

Roots first: an RFC sits one level above the highest level of the RFCs it builds on, and the RFCs of one citation cycle share a level. Each RFC carries the same two bars as the set above, from its own data file; an RFC that is not walked shows "not walked" and no bar.

### Level 2

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 3748](../../rfc3748/) | Extensible Authentication Protocol (EAP) | Ze proven 42, Ze correctness gap 17 | Ze MUST proven 42, SHOULD not decided 4, Ze correctness gap 17 |

### Level 3

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 2759](../../rfc2759/) | Microsoft PPP CHAP Extensions, Version 2 | Ze proven 4, Ze correctness gap 3 | Ze MUST proven 4, SHOULD proven 2, SHOULD not decided 2, Ze correctness gap 3 |

### Level 5

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 5216](../../rfc5216/) | The EAP-TLS Authentication Protocol | Ze proven 30, OS testing gap 2, Ze correctness gap 15 | Ze MUST proven 30, SHOULD proven 2, SHOULD not decided 5, OS testing gap 2, Ze correctness gap 15 |
| [RFC 5998](../../rfc5998/) | An Extension for EAP-Only Authentication in IKEv2 | Ze proven 1, Ze correctness gap 5 | Ze MUST proven 1, SHOULD not decided 1, Ze correctness gap 5 |
| [RFC 7627](../../rfc7627/) | Transport Layer Security (TLS) Session Hash and Extended Master Secret Extension | not walked | not walked |

### Level 6

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 9190](../../rfc9190/) | EAP-TLS 1.3: Using the Extensible Authentication Protocol with TLS 1.3 | Ze proven 42, Ze correctness gap 11 | Ze MUST proven 42, SHOULD proven 2, SHOULD not decided 41, Ze correctness gap 11 |
