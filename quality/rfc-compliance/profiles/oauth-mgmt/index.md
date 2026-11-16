# OAuth for management

Technology profile. OAuth 2.0 for management access. The RFCs grouped under oauth-mgmt, showing mandatory requirement coverage and overall coverage for the group and each RFC.

[All profiles](../) · [All RFCs](../../)

Figures as of the last build, read from [data/rfc/profiles/oauth-mgmt.json](../../../../data/rfc/profiles/oauth-mgmt.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze verified 18, Tests awaiting verification 11, Exclusion unchecked 19, Not built 11
- **Overview bar**: Ze MUST verified 18, SHOULD verified 2, Tests awaiting verification 11, Exclusion unchecked 21, Not assessed 8, Not built 12

| Figure | Rows |
|---|---|
| RFCs | 3 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 59 |
| MUST rows kept (the MUST bar) | 59 |
| Ze verified | 18 |
| OS verified | 0 |
| Tests awaiting verification | 11 |
| OS testing gap | 0 |
| Exclusion unchecked | 19 |
| Not assessed | 0 |
| Not built | 11 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 13 |
| SHOULD verified | 2 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 2 |
| SHOULD not assessed | 8 |
| SHOULD not built | 1 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

## Tree

The RFCs are listed with the standards they build on first. An RFC appears one level after the highest level of its dependencies; RFCs that refer back to each other share a level. Each RFC has the same two coverage bars as the group above. An RFC whose requirement list has not been reviewed shows "requirements not reviewed" instead of a bar.

### Level 6

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 8707](../../rfc8707/) | Resource Indicators for OAuth 2.0 | Not built 2 | Exclusion unchecked 2, Not assessed 2, Not built 3 |

### Level 9

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 8414](../../rfc8414/) | OAuth 2.0 Authorization Server Metadata | Ze verified 9, Tests awaiting verification 7, Exclusion unchecked 12, Not built 3 | Ze MUST verified 9, Tests awaiting verification 7, Exclusion unchecked 12, Not assessed 2, Not built 3 |

### Level 11

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 9728](../../rfc9728/) | OAuth 2.0 Protected Resource Metadata | Ze verified 9, Tests awaiting verification 4, Exclusion unchecked 7, Not built 6 | Ze MUST verified 9, SHOULD verified 2, Tests awaiting verification 4, Exclusion unchecked 7, Not assessed 4, Not built 6 |
