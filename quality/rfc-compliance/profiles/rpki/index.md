# RPKI

Technology profile. RPKI, origin validation, and RPKI to router. The RFCs grouped under rpki, showing mandatory requirement coverage and overall coverage for the group and each RFC.

[All profiles](../) · [All RFCs](../../)

Use case: [Transit Edge with RPKI](../../../../use-cases/transit-edge-rpki/)

Figures as of the last build, read from [data/rfc/profiles/rpki.json](../../../../data/rfc/profiles/rpki.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze verified 17, Tests awaiting verification 17, Not assessed 16, Not built 87
- **Overview bar**: Ze MUST verified 17, SHOULD verified 6, Not owed 5, Tests awaiting verification 17, Not assessed 88, Not built 89

| Figure | Rows |
|---|---|
| RFCs | 9 |
| Requirements not reviewed | 1 |
| Mandatory requirements complete | not complete |
| MUST rows | 142 |
| MUST rows kept (the MUST bar) | 137 |
| Ze verified | 17 |
| OS verified | 0 |
| Tests awaiting verification | 17 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 16 |
| Not built | 87 |
| MUST rows not owed | 5 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 5 |
| SHOULD / MAY rows | 80 |
| SHOULD verified | 6 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 72 |
| SHOULD not built | 2 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

## Tree

The RFCs are listed with the standards they build on first. An RFC appears one level after the highest level of its dependencies; RFCs that refer back to each other share a level. Each RFC has the same two coverage bars as the group above. An RFC whose requirement list has not been reviewed shows "requirements not reviewed" instead of a bar.

### Level 3

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 9582](../../rfc9582/) | A Profile for Route Origin Authorizations (ROAs) | Not assessed 16 | Not assessed 22 |

### Level 4

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 6482](../../rfc6482/) | A Profile for Route Origin Authorizations (ROAs) | Not built 9 | Not assessed 1, Not built 9 |

### Level 7

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 6811](../../rfc6811/) | BGP Prefix Origin Validation | Ze verified 5 | Ze MUST verified 5, Not assessed 4 |

### Level 8

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 6810](../../rfc6810/) | The Resource Public Key Infrastructure (RPKI) to Router Protocol | Not built 38 | Not owed 2, Not assessed 28, Not built 38 |
| [RFC 8097](../../rfc8097/) | BGP Prefix Origin Validation State Extended Community | Not built 5 | Not assessed 5, Not built 5 |
| [RFC 8893](../../rfc8893/) | Resource Public Key Infrastructure (RPKI) Origin Validation for BGP Export | Not built 4 | Not assessed 2, Not built 4 |
| [RFC 9319](../../rfc9319/) | The Use of maxLength in the Resource Public Key Infrastructure (RPKI) | Not built 4 | Not assessed 7, Not built 4 |

### Level 9

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 8210](../../rfc8210/) | The Resource Public Key Infrastructure (RPKI) to Router Protocol, Version 1 | Ze verified 12, Tests awaiting verification 17, Not built 27 | Ze MUST verified 12, SHOULD verified 6, Not owed 3, Tests awaiting verification 17, Not assessed 19, Not built 29 |

### Level 10

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [DRAFT-IETF-SIDROPS-8210BIS](../../draft-ietf-sidrops-8210bis/) | The Resource Public Key Infrastructure (RPKI) to Router Protocol, Version 2 | requirements not reviewed | requirements not reviewed |
