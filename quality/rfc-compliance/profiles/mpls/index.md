# MPLS

Technology profile. MPLS, LDP, RSVP-TE, and fast reroute. The RFCs grouped under mpls, showing mandatory requirement coverage and overall coverage for the group and each RFC.

[All profiles](../) · [All RFCs](../../)

Figures as of the last build, read from [data/rfc/profiles/mpls.json](../../../../data/rfc/profiles/mpls.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze verified 127, Tests awaiting verification 9, OS testing gap 2, Not built 181
- **Overview bar**: Ze MUST verified 127, SHOULD verified 6, Tests awaiting verification 10, OS testing gap 2, Not assessed 49, Not built 184

| Figure | Rows |
|---|---|
| RFCs | 10 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 319 |
| MUST rows kept (the MUST bar) | 319 |
| Ze verified | 127 |
| OS verified | 0 |
| Tests awaiting verification | 9 |
| OS testing gap | 2 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 181 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 59 |
| SHOULD verified | 6 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 1 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 49 |
| SHOULD not built | 3 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

## Tree

The RFCs are listed with the standards they build on first. An RFC appears one level after the highest level of its dependencies; RFCs that refer back to each other share a level. Each RFC has the same two coverage bars as the group above. An RFC whose requirement list has not been reviewed shows "requirements not reviewed" instead of a bar.

### Level 0

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 2205](../../rfc2205/) (cycle with rfc2210, rfc2211, rfc2215) | Resource ReSerVation Protocol (RSVP) -- Version 1 Functional Specification | Ze verified 31, Not built 44 | Ze MUST verified 31, Not assessed 5, Not built 44 |

### Level 1

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 5036](../../rfc5036/) | LDP Specification | Ze verified 29, Tests awaiting verification 3, Not built 59 | Ze MUST verified 29, Tests awaiting verification 3, Not assessed 3, Not built 60 |

### Level 2

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 5561](../../rfc5561/) | LDP Capabilities | Ze verified 1, Not built 16 | Ze MUST verified 1, Not assessed 1, Not built 16 |

### Level 3

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 3031](../../rfc3031/) (cycle with rfc3032) | Multiprotocol Label Switching Architecture | Ze verified 2, Not built 8 | Ze MUST verified 2, Not assessed 2, Not built 8 |
| [RFC 3032](../../rfc3032/) (cycle with rfc3031) | MPLS Label Stack Encoding | Ze verified 3, Not built 15 | Ze MUST verified 3, Not assessed 5, Not built 16 |

### Level 4

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 3209](../../rfc3209/) | RSVP-TE: Extensions to RSVP for LSP Tunnels | Ze verified 23, Tests awaiting verification 2, OS testing gap 2, Not built 32 | Ze MUST verified 23, Tests awaiting verification 3, OS testing gap 2, Not assessed 2, Not built 32 |

### Level 5

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 4090](../../rfc4090/) | Fast Reroute Extensions to RSVP-TE for LSP Tunnels | Ze verified 29, Tests awaiting verification 3, Not built 1 | Ze MUST verified 29, SHOULD verified 6, Tests awaiting verification 3, Not assessed 1, Not built 2 |
| [RFC 5443](../../rfc5443/) | LDP IGP Synchronization | Ze verified 5, Not built 3 | Ze MUST verified 5, Not assessed 6, Not built 3 |

### Level 6

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 6138](../../rfc6138/) | LDP IGP Synchronization for Broadcast Networks | Ze verified 2 | Ze MUST verified 2 |

### Level 10

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 5286](../../rfc5286/) | Basic Specification for IP Fast Reroute: Loop-Free Alternates | Ze verified 2, Tests awaiting verification 1, Not built 3 | Ze MUST verified 2, Tests awaiting verification 1, Not assessed 24, Not built 3 |
