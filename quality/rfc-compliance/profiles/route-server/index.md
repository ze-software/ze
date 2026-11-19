# IXP route server

Release profile. The RFCs this use case requires, each declared by the Use cases row of its own summary. RFCs it lists as optional follow in their own table and never enter its figures. Before release, every mandatory requirement of every required RFC must have a test that has been shown to fail when the behavior it checks is deliberately broken.

[All profiles](../) · [All RFCs](../../)

Use case: [Route Server at an IXP](../../../../use-cases/route-server/)

Figures as of the last build, read from [data/rfc/profiles/route-server.json](../../../../data/rfc/profiles/route-server.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze verified 251, Tests awaiting verification 74, Not assessed 16, Not built 53
- **Overview bar**: Ze MUST verified 251, SHOULD verified 16, Not owed 3, Tests awaiting verification 79, Not assessed 214, Not built 55

| Figure | Rows |
|---|---|
| RFCs | 24 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 397 |
| MUST rows kept (the MUST bar) | 394 |
| Ze verified | 251 |
| OS verified | 0 |
| Tests awaiting verification | 74 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 16 |
| Not built | 53 |
| MUST rows not owed | 3 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 3 |
| SHOULD / MAY rows | 221 |
| SHOULD verified | 16 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 5 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 198 |
| SHOULD not built | 2 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

## Tree

The RFCs are listed with the standards they build on first. An RFC appears one level after the highest level of its dependencies; RFCs that refer back to each other share a level. Each RFC has the same two coverage bars as the group above. An RFC whose requirement list has not been reviewed shows "requirements not reviewed" instead of a bar. The last column names the summary whose Use cases row declares the RFC: a normative citation never adds one.

### Level 3

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 4271](../../rfc4271/) | A Border Gateway Protocol 4 (BGP-4) | Ze verified 96, Tests awaiting verification 24, Not built 13 | Ze MUST verified 96, SHOULD verified 2, Tests awaiting verification 26, Not assessed 30, Not built 13 | route-server: required by rfc/short/rfc4271.md |
| [RFC 9582](../../rfc9582/) | A Profile for Route Origin Authorizations (ROAs) | Not assessed 16 | Not assessed 22 | route-server: required by rfc/short/rfc9582.md |

### Level 4

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 1997](../../rfc1997/) | BGP Communities Attribute | Ze verified 5 | Ze MUST verified 5, Not assessed 4 | route-server: required by rfc/short/rfc1997.md |
| [RFC 2545](../../rfc2545/) | Use of BGP-4 Multiprotocol Extensions for IPv6 Inter-Domain Routing | Ze verified 3, Not built 2 | Ze MUST verified 3, Not assessed 2, Not built 2 | route-server: required by rfc/short/rfc2545.md |
| [RFC 5492](../../rfc5492/) | Capabilities Advertisement with BGP-4 | Ze verified 8, Not built 1 | Ze MUST verified 8, Not assessed 7, Not built 1 | route-server: required by rfc/short/rfc5492.md |
| [RFC 6608](../../rfc6608/) | Subcodes for BGP Finite State Machine Error | Ze verified 3 | Ze MUST verified 3 | route-server: required by rfc/short/rfc6608.md |
| [RFC 8212](../../rfc8212/) | Default External BGP (EBGP) Route Propagation Behavior without Policies | Ze verified 2 | Ze MUST verified 2, Not assessed 1 | route-server: required by rfc/short/rfc8212.md |
| [RFC 9687](../../rfc9687/) | Border Gateway Protocol 4 (BGP-4) Send Hold Timer | Ze verified 13 | Ze MUST verified 13, Not assessed 7 | route-server: required by rfc/short/rfc9687.md |

### Level 5

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 4760](../../rfc4760/) | Multiprotocol Extensions for BGP-4 | Ze verified 6 | Ze MUST verified 6, SHOULD verified 1, Not assessed 10 | route-server: required by rfc/short/rfc4760.md |

### Level 6

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 6793](../../rfc6793/) (cycle with rfc5668) | BGP Support for Four-Octet Autonomous System (AS) Number Space | Ze verified 30 | Ze MUST verified 30, Not assessed 6 | route-server: required by rfc/short/rfc6793.md |
| [RFC 7911](../../rfc7911/) | Advertisement of Multiple Paths in BGP | Ze verified 7, Tests awaiting verification 2 | Ze MUST verified 7, Tests awaiting verification 2, Not assessed 4 | route-server: required by rfc/short/rfc7911.md |
| [RFC 7947](../../rfc7947/) | Internet Exchange BGP Route Server | Ze verified 2, Not built 1 | Ze MUST verified 2, SHOULD verified 2, Tests awaiting verification 3, Not built 1 | route-server: required by rfc/short/rfc7947.md |

### Level 7

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 6286](../../rfc6286/) | Autonomous-System-Wide Unique BGP Identifier for BGP-4 | Ze verified 4 | Ze MUST verified 4, SHOULD verified 1 | route-server: required by rfc/short/rfc6286.md |
| [RFC 6811](../../rfc6811/) | BGP Prefix Origin Validation | Ze verified 3, Tests awaiting verification 2 | Ze MUST verified 3, Tests awaiting verification 2, Not assessed 4 | route-server: required by rfc/short/rfc6811.md |
| [RFC 7606](../../rfc7606/) (cycle with rfc4364, rfc4659, rfc4798, rfc6368, rfc8277, rfc8950) | Revised Error Handling for BGP UPDATE Messages | Ze verified 33, Tests awaiting verification 19, Not built 1 | Ze MUST verified 33, Tests awaiting verification 19, Not assessed 4, Not built 1 | route-server: required by rfc/short/rfc7606.md |
| [RFC 7948](../../rfc7948/) | Internet Exchange BGP Route Server Operations | no rows | Not assessed 11 | route-server: required by rfc/short/rfc7948.md |

### Level 8

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 7454](../../rfc7454/) | BGP Operations and Security | no rows | Not assessed 64 | route-server: required by rfc/short/rfc7454.md |
| [RFC 7607](../../rfc7607/) | Codification of AS 0 Processing | Ze verified 5 | Ze MUST verified 5 | route-server: required by rfc/short/rfc7607.md |
| [RFC 7999](../../rfc7999/) | BLACKHOLE Community | Ze verified 2, Tests awaiting verification 2 | Ze MUST verified 2, SHOULD verified 3, Tests awaiting verification 2, Not assessed 9 | route-server: required by rfc/short/rfc7999.md |
| [RFC 8092](../../rfc8092/) | BGP Large Communities Attribute | Ze verified 7 | Ze MUST verified 7, Not assessed 4 | route-server: required by rfc/short/rfc8092.md |
| [RFC 8893](../../rfc8893/) | Resource Public Key Infrastructure (RPKI) Origin Validation for BGP Export | Not built 4 | Not assessed 2, Not built 4 | route-server: required by rfc/short/rfc8893.md |
| [RFC 9234](../../rfc9234/) | Route Leak Prevention and Detection Using Roles in UPDATE and OPEN Messages | Ze verified 8, Tests awaiting verification 8, Not built 3 | Ze MUST verified 8, SHOULD verified 1, Tests awaiting verification 8, Not assessed 4, Not built 3 | route-server: required by rfc/short/rfc9234.md |
| [RFC 9774](../../rfc9774/) | Deprecation of AS_SET and AS_CONFED_SET in BGP | Ze verified 2, Not built 1 | Ze MUST verified 2, Not built 1 | route-server: required by rfc/short/rfc9774.md |

### Level 9

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 8210](../../rfc8210/) | The Resource Public Key Infrastructure (RPKI) to Router Protocol, Version 1 | Ze verified 12, Tests awaiting verification 17, Not built 27 | Ze MUST verified 12, SHOULD verified 6, Not owed 3, Tests awaiting verification 17, Not assessed 19, Not built 29 | route-server: required by rfc/short/rfc8210.md |

## Optional (not counted)

RFCs this use case lists as optional, each declared by the Optional use cases row of its own summary. They count in none of the figures above until the owner brings one into scope by moving it to the Use cases row. Their own bars and figures follow.

- **MUST bar**: Ze verified 52, Tests awaiting verification 21, Not built 23
- **Overview bar**: Ze MUST verified 52, SHOULD verified 5, Not owed 1, Tests awaiting verification 22, Not assessed 51, Not built 23

| Figure | Rows |
|---|---|
| RFCs | 12 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 97 |
| MUST rows kept (the MUST bar) | 96 |
| Ze verified | 52 |
| OS verified | 0 |
| Tests awaiting verification | 21 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 23 |
| MUST rows not owed | 1 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 1 |
| SHOULD / MAY rows | 57 |
| SHOULD verified | 5 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 1 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 51 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 2385](../../rfc2385/) | Protection of BGP Sessions via the TCP MD5 Signature Option | Ze verified 8 | Ze MUST verified 8, Not assessed 1 | route-server: optional in rfc/short/rfc2385.md |
| [RFC 9072](../../rfc9072/) | Extended Optional Parameters Length for BGP OPEN Message | Ze verified 5, Not built 4 | Ze MUST verified 5, SHOULD verified 1, Not assessed 2, Not built 4 | route-server: optional in rfc/short/rfc9072.md |
| [RFC 4360](../../rfc4360/) | BGP Extended Communities Attribute | Ze verified 2, Not built 3 | Ze MUST verified 2, Not assessed 4, Not built 3 | route-server: optional in rfc/short/rfc4360.md |
| [RFC 5082](../../rfc5082/) | The Generalized TTL Security Mechanism (GTSM) | Ze verified 4 | Ze MUST verified 4, SHOULD verified 1, Not assessed 5 | route-server: optional in rfc/short/rfc5082.md |
| [RFC 2918](../../rfc2918/) | Route Refresh Capability for BGP-4 | Ze verified 3, Tests awaiting verification 3 | Ze MUST verified 3, SHOULD verified 1, Tests awaiting verification 3, Not assessed 4 | route-server: optional in rfc/short/rfc2918.md |
| [RFC 4486](../../rfc4486/) | Subcodes for BGP Cease NOTIFICATION Message | Tests awaiting verification 1 | Tests awaiting verification 2, Not assessed 9 | route-server: optional in rfc/short/rfc4486.md |
| [RFC 4724](../../rfc4724/) | Graceful Restart Mechanism for BGP | Ze verified 12, Tests awaiting verification 5, Not built 9 | Ze MUST verified 12, SHOULD verified 1, Tests awaiting verification 5, Not assessed 4, Not built 9 | route-server: optional in rfc/short/rfc4724.md |
| [RFC 8203](../../rfc8203/) | BGP Administrative Shutdown Communication | Ze verified 3, Tests awaiting verification 2 | Ze MUST verified 3, Tests awaiting verification 2, Not assessed 4 | route-server: optional in rfc/short/rfc8203.md |
| [RFC 8950](../../rfc8950/) (cycle with rfc4364, rfc4659, rfc4798, rfc6368, rfc7606, rfc8277) | Advertising IPv4 Network Layer Reachability Information (NLRI) with an IPv6 Next Hop | Ze verified 2, Tests awaiting verification 4 | Ze MUST verified 2, Tests awaiting verification 4, Not assessed 2 | route-server: optional in rfc/short/rfc8950.md |
| [RFC 9003](../../rfc9003/) | Extended BGP Administrative Shutdown Communication | Ze verified 1, Tests awaiting verification 3 | Ze MUST verified 1, Tests awaiting verification 3, Not assessed 7 | route-server: optional in rfc/short/rfc9003.md |
| [RFC 7313](../../rfc7313/) | Enhanced Route Refresh Capability for BGP-4 | Ze verified 4, Tests awaiting verification 2, Not built 5 | Ze MUST verified 4, Tests awaiting verification 2, Not assessed 6, Not built 5 | route-server: optional in rfc/short/rfc7313.md |
| [RFC 8654](../../rfc8654/) | Extended Message Support for BGP | Ze verified 8, Tests awaiting verification 1, Not built 2 | Ze MUST verified 8, SHOULD verified 1, Not owed 1, Tests awaiting verification 1, Not assessed 3, Not built 2 | route-server: optional in rfc/short/rfc8654.md |
