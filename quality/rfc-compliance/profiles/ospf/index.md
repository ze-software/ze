# OSPF

Technology profile. OSPFv2 and OSPFv3. The RFCs grouped under ospf, showing mandatory requirement coverage and overall coverage for the group and each RFC.

[All profiles](../) · [All RFCs](../../)

Figures as of the last build, read from [data/rfc/profiles/ospf.json](../../../../data/rfc/profiles/ospf.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze verified 150, Tests awaiting verification 121, OS testing gap 7, Not built 81
- **Overview bar**: Ze MUST verified 150, SHOULD verified 9, Not owed 1, Tests awaiting verification 122, OS testing gap 7, Not assessed 210, Not built 83

| Figure | Rows |
|---|---|
| RFCs | 21 |
| Requirements not reviewed | 3 |
| Mandatory requirements complete | not complete |
| MUST rows | 360 |
| MUST rows kept (the MUST bar) | 359 |
| Ze verified | 150 |
| OS verified | 0 |
| Tests awaiting verification | 121 |
| OS testing gap | 7 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 81 |
| MUST rows not owed | 1 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 1 |
| SHOULD / MAY rows | 222 |
| SHOULD verified | 9 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 1 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 210 |
| SHOULD not built | 2 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

## Tree

The RFCs are listed with the standards they build on first. An RFC appears one level after the highest level of its dependencies; RFCs that refer back to each other share a level. Each RFC has the same two coverage bars as the group above. An RFC whose requirement list has not been reviewed shows "requirements not reviewed" instead of a bar.

### Level 4

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 2328](../../rfc2328/) (cycle with rfc3101, rfc5250) | OSPF Version 2 | Ze verified 58, Tests awaiting verification 9 | Ze MUST verified 58, SHOULD verified 2, Tests awaiting verification 9, Not assessed 13 |
| [RFC 3101](../../rfc3101/) (cycle with rfc2328, rfc5250) | The OSPF Not-So-Stubby Area (NSSA) Option | Ze verified 15, Tests awaiting verification 12, Not built 1 | Ze MUST verified 15, SHOULD verified 2, Tests awaiting verification 13, Not assessed 4, Not built 1 |
| [RFC 5250](../../rfc5250/) (cycle with rfc2328, rfc3101) | The OSPF Opaque LSA Option | Ze verified 10, Tests awaiting verification 5 | Ze MUST verified 10, Tests awaiting verification 5, Not assessed 2 |

### Level 5

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 3623](../../rfc3623/) | Graceful OSPF Restart | Ze verified 8, Tests awaiting verification 1, Not built 4 | Ze MUST verified 8, Tests awaiting verification 1, Not assessed 13, Not built 4 |
| [RFC 3630](../../rfc3630/) | Traffic Engineering (TE) Extensions to OSPF Version 2 | Tests awaiting verification 1, Not built 4 | Tests awaiting verification 1, Not assessed 5, Not built 4 |
| [RFC 5709](../../rfc5709/) | OSPFv2 HMAC-SHA Cryptographic Authentication | Ze verified 5, Tests awaiting verification 10 | Ze MUST verified 5, SHOULD verified 2, Tests awaiting verification 10, Not assessed 5 |

### Level 6

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 7474](../../rfc7474/) | Security Extensions for OSPFv2 when Using Manual Key Management | Ze verified 7, Tests awaiting verification 4, Not built 1 | Ze MUST verified 7, SHOULD verified 1, Tests awaiting verification 4, Not assessed 3, Not built 1 |
| [RFC 7684](../../rfc7684/) | OSPFv2 Prefix/Link Attribute Advertisement | Ze verified 4, Tests awaiting verification 2, Not built 3 | Ze MUST verified 4, Tests awaiting verification 2, Not assessed 11, Not built 3 |

### Level 9

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 4552](../../rfc4552/) (cycle with rfc5340) | Authentication/Confidentiality for OSPFv3 | Ze verified 4, Tests awaiting verification 11, OS testing gap 6, Not built 6 | Ze MUST verified 4, Tests awaiting verification 11, OS testing gap 6, Not assessed 11, Not built 6 |
| [RFC 5340](../../rfc5340/) (cycle with rfc4552) | OSPF for IPv6 | Ze verified 6, Tests awaiting verification 11, OS testing gap 1, Not built 5 | Ze MUST verified 6, Tests awaiting verification 11, OS testing gap 1, Not assessed 23, Not built 5 |

### Level 10

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 5187](../../rfc5187/) | OSPFv3 Graceful Restart | Ze verified 4 | Ze MUST verified 4 |
| [RFC 5392](../../rfc5392/) | OSPF Extensions in Support of Inter-Autonomous System (AS) MPLS and GMPLS Traffic Engineering | Ze verified 5, Tests awaiting verification 4, Not built 5 | Ze MUST verified 5, Tests awaiting verification 4, Not assessed 18, Not built 5 |
| [RFC 5838](../../rfc5838/) | Support of Address Families in OSPFv3 | Ze verified 4, Tests awaiting verification 3, Not built 9 | Ze MUST verified 4, Tests awaiting verification 3, Not assessed 11, Not built 9 |
| [RFC 6549](../../rfc6549/) | OSPFv2 Multi-Instance Extensions | Tests awaiting verification 1 | Tests awaiting verification 1, Not assessed 3 |
| [RFC 6987](../../rfc6987/) | OSPF Stub Router Advertisement | requirements not reviewed | requirements not reviewed |
| [RFC 7166](../../rfc7166/) | Supporting Authentication Trailer for OSPFv3 | Not built 17 | Not assessed 15, Not built 19 |
| [RFC 7770](../../rfc7770/) | Extensions to OSPF for Advertising Optional Router Capabilities | Ze verified 1, Tests awaiting verification 3, Not built 6 | Ze MUST verified 1, SHOULD verified 1, Not owed 1, Tests awaiting verification 3, Not assessed 13, Not built 6 |

### Level 11

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 8362](../../rfc8362/) | OSPFv3 Link State Advertisement (LSA) Extensibility | requirements not reviewed | requirements not reviewed |
| [RFC 8665](../../rfc8665/) | OSPF Extensions for Segment Routing | Ze verified 13, Tests awaiting verification 24, Not built 14 | Ze MUST verified 13, SHOULD verified 1, Tests awaiting verification 24, Not assessed 34, Not built 14 |
| [RFC 9129](../../rfc9129/) | YANG Data Model for the OSPF Protocol | requirements not reviewed | requirements not reviewed |

### Level 12

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 8666](../../rfc8666/) | OSPFv3 Extensions for Segment Routing | Ze verified 6, Tests awaiting verification 20, Not built 6 | Ze MUST verified 6, Tests awaiting verification 20, Not assessed 26, Not built 6 |

## Optional (not counted)

RFCs of this aggregate listed as optional, each declared by the Aggregate tier row of its own summary. They count in none of the figures above until the owner brings one into scope by deleting that row. Their own bars and figures follow.

- **MUST bar**: Tests awaiting verification 1, Not built 49
- **Overview bar**: SHOULD verified 1, Tests awaiting verification 2, Not built 67

| Figure | Rows |
|---|---|
| RFCs | 2 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 50 |
| MUST rows kept (the MUST bar) | 50 |
| Ze verified | 0 |
| OS verified | 0 |
| Tests awaiting verification | 1 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 49 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 20 |
| SHOULD verified | 1 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 1 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 0 |
| SHOULD not built | 18 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 4576](../../rfc4576/) (cycle with rfc4577) | Using a Link State Advertisement (LSA) Options Bit to Prevent Looping in BGP/MPLS IP Virtual Private Networks (VPNs) | Not built 4 | Not built 4 |
| [RFC 4577](../../rfc4577/) (cycle with rfc4576) | OSPF as the Provider/Customer Edge Protocol for BGP/MPLS IP Virtual Private Networks (VPNs) | Tests awaiting verification 1, Not built 45 | SHOULD verified 1, Tests awaiting verification 2, Not built 63 |
