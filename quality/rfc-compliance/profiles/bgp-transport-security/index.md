# BGP transport security

Technology profile. BGP session transport security. The RFCs grouped under bgp-transport-security, showing mandatory requirement coverage and overall coverage for the group and each RFC.

[All profiles](../) · [All RFCs](../../)

Figures as of the last build, read from [data/rfc/profiles/bgp-transport-security.json](../../../../data/rfc/profiles/bgp-transport-security.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze proven 12
- **Overview bar**: Ze MUST proven 12, SHOULD proven 1, Not assessed 6

| Figure | Rows |
|---|---|
| RFCs | 3 |
| Requirements not reviewed | 1 |
| Mandatory requirements complete | not complete |
| MUST rows | 12 |
| MUST rows kept (the MUST bar) | 12 |
| Ze proven | 12 |
| OS verified | 0 |
| Proof pending | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 0 |
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
| SHOULD proof pending | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 6 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze proven means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Proof pending counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

## Tree

The RFCs are listed with the standards they build on first. An RFC appears one level after the highest level of its dependencies; RFCs that refer back to each other share a level. Each RFC has the same two coverage bars as the group above. An RFC whose requirement list has not been reviewed shows "requirements not reviewed" instead of a bar.

### Level 0

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 2385](../../rfc2385/) | Protection of BGP Sessions via the TCP MD5 Signature Option | Ze proven 8 | Ze MUST proven 8, Not assessed 1 |

### Level 5

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 5082](../../rfc5082/) | The Generalized TTL Security Mechanism (GTSM) | Ze proven 4 | Ze MUST proven 4, SHOULD proven 1, Not assessed 5 |

### No level: the repository has no copy of this RFC's text

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 5925](../../rfc5925/) | The TCP Authentication Option | requirements not reviewed | requirements not reviewed |
