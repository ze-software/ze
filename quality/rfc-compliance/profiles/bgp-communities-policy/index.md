# BGP communities and policy

Technology profile. BGP communities, route leak prevention, and policy signalling. The RFCs grouped under bgp-communities-policy, showing mandatory requirement coverage and overall coverage for the group and each RFC.

[All profiles](../) · [All RFCs](../../)

Figures as of the last build, read from [data/rfc/profiles/bgp-communities-policy.json](../../../../data/rfc/profiles/bgp-communities-policy.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze proven 28, Proof pending 12, Not built 9
- **Overview bar**: Ze MUST proven 28, SHOULD proven 4, Proof pending 12, Not assessed 33, Not built 9

| Figure | Rows |
|---|---|
| RFCs | 11 |
| Requirements not reviewed | 1 |
| Mandatory requirements complete | not complete |
| MUST rows | 49 |
| MUST rows kept (the MUST bar) | 49 |
| Ze proven | 28 |
| OS verified | 0 |
| Proof pending | 12 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 9 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 37 |
| SHOULD proven | 4 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD proof pending | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 33 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze proven means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Proof pending counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

## Tree

The RFCs are listed with the standards they build on first. An RFC appears one level after the highest level of its dependencies; RFCs that refer back to each other share a level. Each RFC has the same two coverage bars as the group above. An RFC whose requirement list has not been reviewed shows "requirements not reviewed" instead of a bar.

### Level 0

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 3882](../../rfc3882/) | Configuring BGP to Block Denial-of-Service Attacks | no rows | no rows |

### Level 1

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 5635](../../rfc5635/) | Remote Triggered Black Hole Filtering with Unicast Reverse Path Forwarding (uRPF) | no rows | Not assessed 1 |

### Level 4

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 1997](../../rfc1997/) | BGP Communities Attribute | Ze proven 5 | Ze MUST proven 5, Not assessed 4 |

### Level 5

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 3765](../../rfc3765/) | NOPEER Community for Border Gateway Protocol (BGP) Route Scope Control | no rows | Not assessed 2 |
| [RFC 4360](../../rfc4360/) | BGP Extended Communities Attribute | Ze proven 2, Not built 3 | Ze MUST proven 2, Not assessed 4, Not built 3 |

### Level 6

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 5701](../../rfc5701/) | IPv6 Address Specific BGP Extended Community Attribute | Proof pending 3, Not built 1 | Proof pending 3, Not assessed 2, Not built 1 |

### Level 8

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 7611](../../rfc7611/) | BGP ACCEPT_OWN Community Attribute | Ze proven 2, Proof pending 1, Not built 2 | Ze MUST proven 2, Proof pending 1, Not assessed 3, Not built 2 |
| [RFC 7999](../../rfc7999/) | BLACKHOLE Community | Ze proven 4 | Ze MUST proven 4, SHOULD proven 3, Not assessed 9 |
| [RFC 8092](../../rfc8092/) | BGP Large Communities Attribute | Ze proven 7 | Ze MUST proven 7, Not assessed 4 |
| [RFC 9234](../../rfc9234/) | Route Leak Prevention and Detection Using Roles in UPDATE and OPEN Messages | Ze proven 8, Proof pending 8, Not built 3 | Ze MUST proven 8, SHOULD proven 1, Proof pending 8, Not assessed 4, Not built 3 |

### Level 9

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 8195](../../rfc8195/) | Use of BGP Large Communities | requirements not reviewed | requirements not reviewed |
