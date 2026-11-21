# Drafts and non-RFC standards

Technology profile. Drafts and non-RFC standards tracked near RFC work. The RFCs grouped under drafts, showing mandatory requirement coverage and overall coverage for the group and each RFC.

[All profiles](../) · [All RFCs](../../)

Figures as of the last build, read from [data/rfc/profiles/drafts.json](../../../../data/rfc/profiles/drafts.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze verified 76, Tests awaiting verification 3, OS testing gap 1, Not built 42
- **Overview bar**: Ze MUST verified 76, SHOULD verified 6, Tests awaiting verification 3, OS testing gap 1, Not assessed 55, Not built 42

| Figure | Rows |
|---|---|
| RFCs | 8 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 122 |
| MUST rows kept (the MUST bar) | 122 |
| Ze verified | 76 |
| OS verified | 0 |
| Tests awaiting verification | 3 |
| OS testing gap | 1 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 42 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 61 |
| SHOULD verified | 6 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 55 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

## Tree

The RFCs are listed with the standards they build on first. An RFC appears one level after the highest level of its dependencies; RFCs that refer back to each other share a level. Each RFC has the same two coverage bars as the group above. An RFC whose requirement list has not been reviewed shows "requirements not reviewed" instead of a bar.

### Level 3

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [SFLOW-V5](../../sflow-v5/) | sFlow: A Method for Monitoring Traffic in Switched and Routed Networks | Ze verified 21, OS testing gap 1, Not built 9 | Ze MUST verified 21, SHOULD verified 3, OS testing gap 1, Not assessed 8, Not built 9 |

### Level 5

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [DRAFT-WALTON-BGP-HOSTNAME-CAPABILITY](../../draft-walton-bgp-hostname-capability/) | Hostname Capability for BGP | no rows | Not assessed 1 |

### Level 6

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY](../../draft-abraitis-bgp-version-capability/) | Software Version Capability for BGP | Ze verified 11 | Ze MUST verified 11, Not assessed 4 |

### Level 7

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT](../../draft-abraitis-idr-addpath-paths-limit/) | Scalability Considerations for ADD-PATH with PATHS-LIMIT | Ze verified 1, Tests awaiting verification 3 | Ze MUST verified 1, SHOULD verified 3, Tests awaiting verification 3 |

### Level 8

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [DRAFT-IETF-BESS-MUP-SAFI](../../draft-ietf-bess-mup-safi/) | BGP Extensions for the Mobile User Plane (MUP) SAFI | Ze verified 17, Not built 30 | Ze MUST verified 17, Not assessed 20, Not built 30 |

### Level 9

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY](../../draft-ietf-idr-linklocal-capability/) | Link-Local Next Hop Capability for BGP | Ze verified 13, Not built 2 | Ze MUST verified 13, Not assessed 12, Not built 2 |
| [DRAFT-IETF-SIDROPS-ASPA-VERIFICATION](../../draft-ietf-sidrops-aspa-verification/) | BGP AS_PATH Verification Based on Autonomous System Provider Authorization (ASPA) Objects | Ze verified 9, Not built 1 | Ze MUST verified 9, Not assessed 10, Not built 1 |

### Level 10

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE](../../draft-ietf-idr-bgp-bfd-strict-mode/) | BGP BFD Strict-Mode | Ze verified 4 | Ze MUST verified 4 |
