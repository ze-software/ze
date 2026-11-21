# BGP FlowSpec

Technology profile. BGP Flow Specification. The RFCs grouped under bgp-flowspec, showing mandatory requirement coverage and overall coverage for the group and each RFC.

[All profiles](../) · [All RFCs](../../)

Figures as of the last build, read from [data/rfc/profiles/bgp-flowspec.json](../../../../data/rfc/profiles/bgp-flowspec.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze verified 29, Tests awaiting verification 16, Not built 3
- **Overview bar**: Ze MUST verified 29, SHOULD verified 6, Not owed 1, Tests awaiting verification 16, Not assessed 26, Not built 3

| Figure | Rows |
|---|---|
| RFCs | 4 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 49 |
| MUST rows kept (the MUST bar) | 48 |
| Ze verified | 29 |
| OS verified | 0 |
| Tests awaiting verification | 16 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 3 |
| MUST rows not owed | 1 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 1 |
| SHOULD / MAY rows | 32 |
| SHOULD verified | 6 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 26 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

## Tree

The RFCs are listed with the standards they build on first. An RFC appears one level after the highest level of its dependencies; RFCs that refer back to each other share a level. Each RFC has the same two coverage bars as the group above. An RFC whose requirement list has not been reviewed shows "requirements not reviewed" instead of a bar.

### Level 8

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 8955](../../rfc8955/) | Dissemination of Flow Specification Rules | Ze verified 14, Tests awaiting verification 12, Not built 2 | Ze MUST verified 14, SHOULD verified 6, Not owed 1, Tests awaiting verification 12, Not assessed 14, Not built 2 |

### Level 9

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 5575](../../rfc5575/) | Dissemination of Flow Specification Rules | Ze verified 4, Tests awaiting verification 4, Not built 1 | Ze MUST verified 4, Tests awaiting verification 4, Not assessed 4, Not built 1 |
| [RFC 8956](../../rfc8956/) | Dissemination of Flow Specification Rules for IPv6 | Ze verified 9 | Ze MUST verified 9, Not assessed 5 |
| [RFC 9117](../../rfc9117/) | Revised Validation Procedure for BGP Flow Specifications | Ze verified 2 | Ze MUST verified 2, Not assessed 3 |
