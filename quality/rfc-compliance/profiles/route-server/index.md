# IXP route server

Release profile. The RFCs this use case requires, each declared by the Use cases row of its own summary. RFCs it lists as optional follow in their own table and never enter its figures. Before release, every mandatory requirement of every required RFC must be proven by a recorded break of its test.

[All profiles](../) · [All RFCs](../../)

Use case: [Route Server at an IXP](../../../../use-cases/route-server/)

- **Mandatory**: 47.8% (189 of 395 (47.8%)). MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that a recorded test break proves.
- **Optional**: 8.6% (19 of 221 (8.6%)). MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows, counted the same way.
- **Not walked**: 0 (of 24 RFCs). RFCs with no valid extraction sign-off: their rows count in neither figure.
- **Mandatory register**: not complete (every row proven, none unwalked). A profile is complete only when it requires at least one RFC and none of them is unwalked.

## Tree

Roots first: an RFC sits one level above the highest level of the RFCs it builds on, and the RFCs of one citation cycle share a level. Each bar is the share of that figure's rows a recorded break proves; an RFC that is not walked shows an empty bar and no figure. The last column names the summary whose Use cases row declares the RFC: a normative citation never adds one.

### Level 3

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 4271](../../rfc4271/) | A Border Gateway Protocol 4 (BGP-4) | 83 of 133 (62.4%) | 3 of 34 (8.8%) | route-server: required by rfc/short/rfc4271.md |
| [RFC 9582](../../rfc9582/) | A Profile for Route Origin Authorizations (ROAs) | 0 of 16 (0.0%) | 0 of 6 (0.0%) | route-server: required by rfc/short/rfc9582.md |

### Level 4

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 1997](../../rfc1997/) | BGP Communities Attribute | 3 of 5 (60.0%) | 0 of 4 (0.0%) | route-server: required by rfc/short/rfc1997.md |
| [RFC 2545](../../rfc2545/) | Use of BGP-4 Multiprotocol Extensions for IPv6 Inter-Domain Routing | 3 of 5 (60.0%) | 0 of 2 (0.0%) | route-server: required by rfc/short/rfc2545.md |
| [RFC 5492](../../rfc5492/) | Capabilities Advertisement with BGP-4 | 2 of 9 (22.2%) | 0 of 7 (0.0%) | route-server: required by rfc/short/rfc5492.md |
| [RFC 6608](../../rfc6608/) | Subcodes for BGP Finite State Machine Error | 0 of 3 (0.0%) | no rows | route-server: required by rfc/short/rfc6608.md |
| [RFC 8212](../../rfc8212/) | Default External BGP (EBGP) Route Propagation Behavior without Policies | 0 of 2 (0.0%) | 0 of 1 (0.0%) | route-server: required by rfc/short/rfc8212.md |
| [RFC 9687](../../rfc9687/) | Border Gateway Protocol 4 (BGP-4) Send Hold Timer | 3 of 13 (23.1%) | 0 of 7 (0.0%) | route-server: required by rfc/short/rfc9687.md |

### Level 5

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 4760](../../rfc4760/) | Multiprotocol Extensions for BGP-4 | 5 of 6 (83.3%) | 1 of 11 (9.1%) | route-server: required by rfc/short/rfc4760.md |

### Level 6

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 6793](../../rfc6793/) (cycle with rfc5668) | BGP Support for Four-Octet Autonomous System (AS) Number Space | 20 of 30 (66.7%) | 0 of 6 (0.0%) | route-server: required by rfc/short/rfc6793.md |
| [RFC 7911](../../rfc7911/) | Advertisement of Multiple Paths in BGP | 9 of 9 (100.0%) | 0 of 4 (0.0%) | route-server: required by rfc/short/rfc7911.md |
| [RFC 7947](../../rfc7947/) | Internet Exchange BGP Route Server | 1 of 2 (50.0%) | 4 of 5 (80.0%) | route-server: required by rfc/short/rfc7947.md |

### Level 7

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 6286](../../rfc6286/) | Autonomous-System-Wide Unique BGP Identifier for BGP-4 | 0 of 4 (0.0%) | 1 of 1 (100.0%) | route-server: required by rfc/short/rfc6286.md |
| [RFC 6811](../../rfc6811/) | BGP Prefix Origin Validation | 1 of 5 (20.0%) | 0 of 4 (0.0%) | route-server: required by rfc/short/rfc6811.md |
| [RFC 7606](../../rfc7606/) (cycle with rfc4364, rfc4659, rfc4798, rfc6368, rfc8277, rfc8950) | Revised Error Handling for BGP UPDATE Messages | 28 of 53 (52.8%) | 0 of 4 (0.0%) | route-server: required by rfc/short/rfc7606.md |
| [RFC 7948](../../rfc7948/) | Internet Exchange BGP Route Server Operations | no rows | 0 of 11 (0.0%) | route-server: required by rfc/short/rfc7948.md |

### Level 8

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 7454](../../rfc7454/) | BGP Operations and Security | no rows | 0 of 64 (0.0%) | route-server: required by rfc/short/rfc7454.md |
| [RFC 7607](../../rfc7607/) | Codification of AS 0 Processing | 0 of 5 (0.0%) | no rows | route-server: required by rfc/short/rfc7607.md |
| [RFC 7999](../../rfc7999/) | BLACKHOLE Community | 3 of 4 (75.0%) | 3 of 12 (25.0%) | route-server: required by rfc/short/rfc7999.md |
| [RFC 8092](../../rfc8092/) | BGP Large Communities Attribute | 6 of 7 (85.7%) | 0 of 4 (0.0%) | route-server: required by rfc/short/rfc8092.md |
| [RFC 8893](../../rfc8893/) | Resource Public Key Infrastructure (RPKI) Origin Validation for BGP Export | 0 of 4 (0.0%) | 0 of 2 (0.0%) | route-server: required by rfc/short/rfc8893.md |
| [RFC 9234](../../rfc9234/) | Route Leak Prevention and Detection Using Roles in UPDATE and OPEN Messages | 8 of 19 (42.1%) | 1 of 5 (20.0%) | route-server: required by rfc/short/rfc9234.md |
| [RFC 9774](../../rfc9774/) | Deprecation of AS_SET and AS_CONFED_SET in BGP | 2 of 2 (100.0%) | no rows | route-server: required by rfc/short/rfc9774.md |

### Level 9

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 8210](../../rfc8210/) | The Resource Public Key Infrastructure (RPKI) to Router Protocol, Version 1 | 12 of 59 (20.3%) | 6 of 27 (22.2%) | route-server: required by rfc/short/rfc8210.md |

## Optional (not counted)

RFCs this use case lists as optional, each declared by the Optional use cases row of its own summary. They count in none of the figures above until the owner brings one into scope by moving it to the Use cases row. Their own figures: mandatory 57 of 97 (58.8%), optional 5 of 57 (8.8%); 12 RFCs, 0 not walked.

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 2385](../../rfc2385/) | Protection of BGP Sessions via the TCP MD5 Signature Option | 8 of 8 (100.0%) | 0 of 1 (0.0%) | route-server: optional in rfc/short/rfc2385.md |
| [RFC 9072](../../rfc9072/) | Extended Optional Parameters Length for BGP OPEN Message | 5 of 9 (55.6%) | 1 of 3 (33.3%) | route-server: optional in rfc/short/rfc9072.md |
| [RFC 4360](../../rfc4360/) | BGP Extended Communities Attribute | 2 of 5 (40.0%) | 0 of 4 (0.0%) | route-server: optional in rfc/short/rfc4360.md |
| [RFC 5082](../../rfc5082/) | The Generalized TTL Security Mechanism (GTSM) | 4 of 4 (100.0%) | 1 of 6 (16.7%) | route-server: optional in rfc/short/rfc5082.md |
| [RFC 2918](../../rfc2918/) | Route Refresh Capability for BGP-4 | 3 of 6 (50.0%) | 1 of 5 (20.0%) | route-server: optional in rfc/short/rfc2918.md |
| [RFC 4486](../../rfc4486/) | Subcodes for BGP Cease NOTIFICATION Message | 0 of 1 (0.0%) | 0 of 10 (0.0%) | route-server: optional in rfc/short/rfc4486.md |
| [RFC 4724](../../rfc4724/) | Graceful Restart Mechanism for BGP | 14 of 26 (53.8%) | 1 of 5 (20.0%) | route-server: optional in rfc/short/rfc4724.md |
| [RFC 7313](../../rfc7313/) | Enhanced Route Refresh Capability for BGP-4 | 5 of 11 (45.5%) | 0 of 6 (0.0%) | route-server: optional in rfc/short/rfc7313.md |
| [RFC 8203](../../rfc8203/) | BGP Administrative Shutdown Communication | 3 of 5 (60.0%) | 0 of 4 (0.0%) | route-server: optional in rfc/short/rfc8203.md |
| [RFC 8950](../../rfc8950/) (cycle with rfc4364, rfc4659, rfc4798, rfc6368, rfc7606, rfc8277) | Advertising IPv4 Network Layer Reachability Information (NLRI) with an IPv6 Next Hop | 3 of 6 (50.0%) | 0 of 2 (0.0%) | route-server: optional in rfc/short/rfc8950.md |
| [RFC 9003](../../rfc9003/) | Extended BGP Administrative Shutdown Communication | 1 of 4 (25.0%) | 0 of 7 (0.0%) | route-server: optional in rfc/short/rfc9003.md |
| [RFC 8654](../../rfc8654/) | Extended Message Support for BGP | 9 of 12 (75.0%) | 1 of 4 (25.0%) | route-server: optional in rfc/short/rfc8654.md |
