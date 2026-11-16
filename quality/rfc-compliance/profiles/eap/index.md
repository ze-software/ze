# EAP

Technology profile. EAP methods and their TLS and CHAP building blocks. The RFCs grouped under eap, showing mandatory requirement coverage and overall coverage for the group and each RFC.

[All profiles](../) · [All RFCs](../../)

Figures as of the last build, read from [data/rfc/profiles/eap.json](../../../../data/rfc/profiles/eap.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze verified 119, Tests awaiting verification 22, OS testing gap 2, Not assessed 3, Not built 26
- **Overview bar**: Ze MUST verified 119, SHOULD verified 6, Tests awaiting verification 22, OS testing gap 2, Not assessed 55, Not built 27

| Figure | Rows |
|---|---|
| RFCs | 6 |
| Requirements not reviewed | 1 |
| Mandatory requirements complete | not complete |
| MUST rows | 172 |
| MUST rows kept (the MUST bar) | 172 |
| Ze verified | 119 |
| OS verified | 0 |
| Tests awaiting verification | 22 |
| OS testing gap | 2 |
| Exclusion unchecked | 0 |
| Not assessed | 3 |
| Not built | 26 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 59 |
| SHOULD verified | 6 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 52 |
| SHOULD not built | 1 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

## Tree

The RFCs are listed with the standards they build on first. An RFC appears one level after the highest level of its dependencies; RFCs that refer back to each other share a level. Each RFC has the same two coverage bars as the group above. An RFC whose requirement list has not been reviewed shows "requirements not reviewed" instead of a bar.

### Level 2

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 3748](../../rfc3748/) | Extensible Authentication Protocol (EAP) | Ze verified 42, Tests awaiting verification 11, Not built 6 | Ze MUST verified 42, Tests awaiting verification 11, Not assessed 4, Not built 6 |

### Level 3

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 2759](../../rfc2759/) | Microsoft PPP CHAP Extensions, Version 2 | Ze verified 4, Tests awaiting verification 3 | Ze MUST verified 4, SHOULD verified 2, Tests awaiting verification 3, Not assessed 2 |

### Level 5

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 5216](../../rfc5216/) | The EAP-TLS Authentication Protocol | Ze verified 30, Tests awaiting verification 7, OS testing gap 2, Not built 8 | Ze MUST verified 30, SHOULD verified 2, Tests awaiting verification 7, OS testing gap 2, Not assessed 4, Not built 9 |
| [RFC 5998](../../rfc5998/) | An Extension for EAP-Only Authentication in IKEv2 | Ze verified 1, Not built 5 | Ze MUST verified 1, Not assessed 1, Not built 5 |
| [RFC 7627](../../rfc7627/) | Transport Layer Security (TLS) Session Hash and Extended Master Secret Extension | requirements not reviewed | requirements not reviewed |

### Level 6

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 9190](../../rfc9190/) | EAP-TLS 1.3: Using the Extensible Authentication Protocol with TLS 1.3 | Ze verified 42, Tests awaiting verification 1, Not assessed 3, Not built 7 | Ze MUST verified 42, SHOULD verified 2, Tests awaiting verification 1, Not assessed 44, Not built 7 |
