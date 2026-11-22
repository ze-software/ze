# BGP monitoring

Technology profile. BGP Monitoring Protocol (BMP). The RFCs grouped under bgp-monitoring, showing mandatory requirement coverage and overall coverage for the group and each RFC.

[All profiles](../) · [All RFCs](../../)

Figures as of the last build, read from [data/rfc/profiles/bgp-monitoring.json](../../../../data/rfc/profiles/bgp-monitoring.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze verified 64, Not built 20
- **Overview bar**: Ze MUST verified 64, SHOULD verified 3, Tests awaiting verification 1, Not assessed 11, Not built 20

| Figure | Rows |
|---|---|
| RFCs | 4 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 84 |
| MUST rows kept (the MUST bar) | 84 |
| Ze verified | 64 |
| OS verified | 0 |
| Tests awaiting verification | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 20 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 15 |
| SHOULD verified | 3 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 1 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 11 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

## Tree

The RFCs are listed with the standards they build on first. An RFC appears one level after the highest level of its dependencies; RFCs that refer back to each other share a level. Each RFC has the same two coverage bars as the group above. An RFC whose requirement list has not been reviewed shows "requirements not reviewed" instead of a bar.

### Level 6

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 8522](../../rfc8522/) | Looking Glass Command Set | Not built 18 | Not assessed 5, Not built 18 |

### Level 8

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 7854](../../rfc7854/) | BGP Monitoring Protocol (BMP) | Ze verified 39 | Ze MUST verified 39, SHOULD verified 3, Tests awaiting verification 1, Not assessed 5 |

### Level 9

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 8671](../../rfc8671/) | Support for Adj-RIB-Out in the BGP Monitoring Protocol (BMP) | Ze verified 9, Not built 2 | Ze MUST verified 9, Not assessed 1, Not built 2 |
| [RFC 9069](../../rfc9069/) | Support for Local RIB in the BGP Monitoring Protocol (BMP) | Ze verified 16 | Ze MUST verified 16 |
