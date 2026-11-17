# BGP segment routing and tunnels

Technology profile. BGP segment routing, SRv6, and tunnel encapsulation. The RFCs grouped under bgp-sr-tunnel, showing mandatory requirement coverage and overall coverage for the group and each RFC.

[All profiles](../) · [All RFCs](../../)

Figures as of the last build, read from [data/rfc/profiles/bgp-sr-tunnel.json](../../../../data/rfc/profiles/bgp-sr-tunnel.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze verified 89, Tests awaiting verification 48, Not built 106
- **Overview bar**: Ze MUST verified 89, SHOULD verified 3, Not owed 1, Tests awaiting verification 49, Not assessed 131, Not built 107

| Figure | Rows |
|---|---|
| RFCs | 5 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 244 |
| MUST rows kept (the MUST bar) | 243 |
| Ze verified | 89 |
| OS verified | 0 |
| Tests awaiting verification | 48 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 106 |
| MUST rows not owed | 1 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 1 |
| SHOULD / MAY rows | 136 |
| SHOULD verified | 3 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 1 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 131 |
| SHOULD not built | 1 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

## Tree

The RFCs are listed with the standards they build on first. An RFC appears one level after the highest level of its dependencies; RFCs that refer back to each other share a level. Each RFC has the same two coverage bars as the group above. An RFC whose requirement list has not been reviewed shows "requirements not reviewed" instead of a bar.

### Level 8

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 8669](../../rfc8669/) | Segment Routing Prefix Segment Identifier Extensions for BGP | Ze verified 13, Tests awaiting verification 4, Not built 7 | Ze MUST verified 13, SHOULD verified 3, Tests awaiting verification 4, Not assessed 18, Not built 7 |

### Level 9

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 9012](../../rfc9012/) | The BGP Tunnel Encapsulation Attribute | Ze verified 25, Not built 50 | Ze MUST verified 25, Not owed 1, Tests awaiting verification 1, Not assessed 21, Not built 50 |

### Level 11

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 9252](../../rfc9252/) | BGP Overlay Services Based on Segment Routing over IPv6 (SRv6) | Ze verified 14, Tests awaiting verification 5, Not built 6 | Ze MUST verified 14, Tests awaiting verification 5, Not assessed 5, Not built 6 |

### Level 13

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 9256](../../rfc9256/) | Segment Routing Policy Architecture | Tests awaiting verification 3, Not built 19 | Tests awaiting verification 3, Not assessed 46, Not built 20 |

### Level 14

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 9830](../../rfc9830/) | BGP Extensions for the Advertisement of Segment Routing (SR) Policies | Ze verified 37, Tests awaiting verification 36, Not built 24 | Ze MUST verified 37, Tests awaiting verification 36, Not assessed 41, Not built 24 |
