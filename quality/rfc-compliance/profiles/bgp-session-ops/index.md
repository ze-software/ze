# BGP session operations

Technology profile. BGP session operations: graceful restart, shutdown, route reflection, route servers, and AS migration. The RFCs grouped under bgp-session-ops, showing mandatory requirement coverage and overall coverage for the group and each RFC.

[All profiles](../) · [All RFCs](../../)

Use case: [Chaos-Tested BGP Peering](../../../../use-cases/chaos-tested-peering/)

Figures as of the last build, read from [data/rfc/profiles/bgp-session-ops.json](../../../../data/rfc/profiles/bgp-session-ops.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze verified 38, Tests awaiting verification 22, Not built 17
- **Overview bar**: Ze MUST verified 38, SHOULD verified 6, Tests awaiting verification 26, Not assessed 112, Not built 17

| Figure | Rows |
|---|---|
| RFCs | 13 |
| Requirements not reviewed | 3 |
| Mandatory requirements complete | not complete |
| MUST rows | 77 |
| MUST rows kept (the MUST bar) | 77 |
| Ze verified | 38 |
| OS verified | 0 |
| Tests awaiting verification | 22 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 17 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 122 |
| SHOULD verified | 6 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 4 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 112 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

## Tree

The RFCs are listed with the standards they build on first. An RFC appears one level after the highest level of its dependencies; RFCs that refer back to each other share a level. Each RFC has the same two coverage bars as the group above. An RFC whose requirement list has not been reviewed shows "requirements not reviewed" instead of a bar.

### Level 4

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 4456](../../rfc4456/) | BGP Route Reflection: An Alternative to Full Mesh Internal BGP (IBGP) | Tests awaiting verification 4 | Tests awaiting verification 7, Not assessed 2 |
| [RFC 5065](../../rfc5065/) | Autonomous System Confederations for BGP | requirements not reviewed | requirements not reviewed |

### Level 5

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 7705](../../rfc7705/) | Autonomous System Migration Mechanisms and Their Effects on the BGP AS_PATH Attribute | Ze verified 9 | Ze MUST verified 9, Not assessed 8 |
| [RFC 8326](../../rfc8326/) | Graceful BGP Session Shutdown | requirements not reviewed | requirements not reviewed |

### Level 6

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 4724](../../rfc4724/) | Graceful Restart Mechanism for BGP | Ze verified 14, Tests awaiting verification 3, Not built 9 | Ze MUST verified 14, SHOULD verified 1, Tests awaiting verification 3, Not assessed 4, Not built 9 |
| [RFC 7947](../../rfc7947/) | Internet Exchange BGP Route Server | Ze verified 2, Not built 1 | Ze MUST verified 2, SHOULD verified 4, Tests awaiting verification 1, Not built 1 |

### Level 7

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 6996](../../rfc6996/) | Autonomous System (AS) Reservation for Private Use | Ze verified 1 | Ze MUST verified 1, Not assessed 2 |
| [RFC 7948](../../rfc7948/) | Internet Exchange BGP Route Server Operations | no rows | Not assessed 11 |
| [RFC 8203](../../rfc8203/) | BGP Administrative Shutdown Communication | Ze verified 3, Tests awaiting verification 2 | Ze MUST verified 3, Tests awaiting verification 2, Not assessed 4 |
| [RFC 9003](../../rfc9003/) | Extended BGP Administrative Shutdown Communication | Ze verified 1, Tests awaiting verification 3 | Ze MUST verified 1, Tests awaiting verification 3, Not assessed 7 |

### Level 8

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 7454](../../rfc7454/) | BGP Operations and Security | no rows | Not assessed 64 |
| [RFC 8538](../../rfc8538/) | Notification Message Support for BGP Graceful Restart | requirements not reviewed | requirements not reviewed |

### Level 9

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 9494](../../rfc9494/) | Long-Lived Graceful Restart for BGP | Ze verified 8, Tests awaiting verification 10, Not built 7 | Ze MUST verified 8, SHOULD verified 1, Tests awaiting verification 10, Not assessed 10, Not built 7 |
