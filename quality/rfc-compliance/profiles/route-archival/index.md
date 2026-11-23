# Route archival

Technology profile. MRT routing information export. The RFCs grouped under route-archival, showing mandatory requirement coverage and overall coverage for the group and each RFC.

[All profiles](../) · [All RFCs](../../)

Figures as of the last build, read from [data/rfc/profiles/route-archival.json](../../../../data/rfc/profiles/route-archival.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze verified 22
- **Overview bar**: Ze MUST verified 22, Not owed 1, Not assessed 13

| Figure | Rows |
|---|---|
| RFCs | 3 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | complete |
| MUST rows | 23 |
| MUST rows kept (the MUST bar) | 22 |
| Ze verified | 22 |
| OS verified | 0 |
| Tests awaiting verification | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 0 |
| MUST rows not owed | 1 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 1 |
| SHOULD / MAY rows | 13 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 13 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

## Tree

The RFCs are listed with the standards they build on first. An RFC appears one level after the highest level of its dependencies; RFCs that refer back to each other share a level. Each RFC has the same two coverage bars as the group above. An RFC whose requirement list has not been reviewed shows "requirements not reviewed" instead of a bar.

### Level 10

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 6396](../../rfc6396/) | Multi-Threaded Routing Toolkit (MRT) Routing Information Export Format | Ze verified 12 | Ze MUST verified 12, Not owed 1, Not assessed 12 |

### Level 11

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 6397](../../rfc6397/) | Multi-Threaded Routing Toolkit (MRT) Border Gateway Protocol (BGP) Routing Information Export Format with Geo-Location Extensions | Ze verified 4 | Ze MUST verified 4, Not assessed 1 |
| [RFC 8050](../../rfc8050/) | Multi-Threaded Routing Toolkit (MRT) Routing Information Export Format with BGP Additional Path Extensions | Ze verified 6 | Ze MUST verified 6 |
