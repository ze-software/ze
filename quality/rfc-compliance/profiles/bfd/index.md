# BFD

Technology profile. Bidirectional Forwarding Detection. The RFCs grouped under bfd, showing mandatory requirement coverage and overall coverage for the group and each RFC.

[All profiles](../) · [All RFCs](../../)

Figures as of the last build, read from [data/rfc/profiles/bfd.json](../../../../data/rfc/profiles/bfd.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze proven 88, Proof pending 32, Not built 23
- **Overview bar**: Ze MUST proven 88, SHOULD proven 12, Proof pending 32, Not assessed 57, Not built 23

| Figure | Rows |
|---|---|
| RFCs | 5 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 143 |
| MUST rows kept (the MUST bar) | 143 |
| Ze proven | 88 |
| OS verified | 0 |
| Proof pending | 32 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 23 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 69 |
| SHOULD proven | 12 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD proof pending | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 57 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze proven means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Proof pending counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

## Tree

The RFCs are listed with the standards they build on first. An RFC appears one level after the highest level of its dependencies; RFCs that refer back to each other share a level. Each RFC has the same two coverage bars as the group above. An RFC whose requirement list has not been reviewed shows "requirements not reviewed" instead of a bar.

### Level 6

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 5880](../../rfc5880/) | Bidirectional Forwarding Detection (BFD) | Ze proven 72, Proof pending 27, Not built 12 | Ze MUST proven 72, SHOULD proven 6, Proof pending 27, Not assessed 20, Not built 12 |

### Level 7

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 5881](../../rfc5881/) (cycle with rfc5882, rfc5883) | Bidirectional Forwarding Detection (BFD) for IPv4 and IPv6 (Single Hop) | Ze proven 13, Proof pending 2, Not built 8 | Ze MUST proven 13, Proof pending 2, Not assessed 10, Not built 8 |
| [RFC 5882](../../rfc5882/) (cycle with rfc5881, rfc5883) | Generic Application of Bidirectional Forwarding Detection (BFD) | Not built 3 | SHOULD proven 5, Not assessed 24, Not built 3 |
| [RFC 5883](../../rfc5883/) (cycle with rfc5881, rfc5882) | Bidirectional Forwarding Detection (BFD) for Multihop Paths | Ze proven 3, Proof pending 3 | Ze MUST proven 3, Proof pending 3, Not assessed 1 |

### Level 9

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 9384](../../rfc9384/) | A BGP Cease NOTIFICATION Subcode for Bidirectional Forwarding Detection (BFD) | no rows | SHOULD proven 1, Not assessed 2 |
