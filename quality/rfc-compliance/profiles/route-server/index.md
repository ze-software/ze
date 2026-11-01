# IXP route server

Release profile. The RFCs this use case requires, each declared by the Use cases row of its own summary. RFCs it lists as optional follow in their own table and never enter its figures. Before release, every mandatory requirement of every required RFC must be proven by a recorded break of its test.

[All profiles](../) · [All RFCs](../../)

Use case: [Route Server at an IXP](../../../../use-cases/route-server/)

Figures as of the last build, read from [data/rfc/profiles/route-server.json](../../../../data/rfc/profiles/route-server.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze proven 203, Ze correctness gap 191
- **Overview bar**: Ze MUST proven 203, SHOULD proven 18, Not owed 3, SHOULD not decided 203, Ze correctness gap 191

| Figure | Rows |
|---|---|
| RFCs | 24 |
| Not walked | 0 |
| Mandatory register | not complete |
| MUST rows | 397 |
| MUST rows kept (the MUST bar) | 394 |
| Ze proven | 203 |
| OS verified | 0 |
| OS testing gap | 0 |
| Ze correctness gap | 191 |
| MUST rows not owed | 3 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 3 |
| SHOULD / MAY rows | 221 |
| SHOULD proven | 18 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 203 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

## Tree

Roots first: an RFC sits one level above the highest level of the RFCs it builds on, and the RFCs of one citation cycle share a level. Each RFC carries the same two bars as the set above, from its own data file; an RFC that is not walked shows "not walked" and no bar. The last column names the summary whose Use cases row declares the RFC: a normative citation never adds one.

### Level 3

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 4271](../../rfc4271/) | A Border Gateway Protocol 4 (BGP-4) | Ze proven 83, Ze correctness gap 50 | Ze MUST proven 83, SHOULD proven 3, SHOULD not decided 31, Ze correctness gap 50 | route-server: required by rfc/short/rfc4271.md |
| [RFC 9582](../../rfc9582/) | A Profile for Route Origin Authorizations (ROAs) | Ze correctness gap 16 | SHOULD not decided 6, Ze correctness gap 16 | route-server: required by rfc/short/rfc9582.md |

### Level 4

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 1997](../../rfc1997/) | BGP Communities Attribute | Ze proven 5 | Ze MUST proven 5, SHOULD not decided 4 | route-server: required by rfc/short/rfc1997.md |
| [RFC 2545](../../rfc2545/) | Use of BGP-4 Multiprotocol Extensions for IPv6 Inter-Domain Routing | Ze proven 3, Ze correctness gap 2 | Ze MUST proven 3, SHOULD not decided 2, Ze correctness gap 2 | route-server: required by rfc/short/rfc2545.md |
| [RFC 5492](../../rfc5492/) | Capabilities Advertisement with BGP-4 | Ze proven 8, Ze correctness gap 1 | Ze MUST proven 8, SHOULD not decided 7, Ze correctness gap 1 | route-server: required by rfc/short/rfc5492.md |
| [RFC 6608](../../rfc6608/) | Subcodes for BGP Finite State Machine Error | Ze correctness gap 3 | Ze correctness gap 3 | route-server: required by rfc/short/rfc6608.md |
| [RFC 8212](../../rfc8212/) | Default External BGP (EBGP) Route Propagation Behavior without Policies | Ze proven 2 | Ze MUST proven 2, SHOULD not decided 1 | route-server: required by rfc/short/rfc8212.md |
| [RFC 9687](../../rfc9687/) | Border Gateway Protocol 4 (BGP-4) Send Hold Timer | Ze proven 3, Ze correctness gap 10 | Ze MUST proven 3, SHOULD not decided 7, Ze correctness gap 10 | route-server: required by rfc/short/rfc9687.md |

### Level 5

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 4760](../../rfc4760/) | Multiprotocol Extensions for BGP-4 | Ze proven 6 | Ze MUST proven 6, SHOULD proven 1, SHOULD not decided 10 | route-server: required by rfc/short/rfc4760.md |

### Level 6

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 6793](../../rfc6793/) (cycle with rfc5668) | BGP Support for Four-Octet Autonomous System (AS) Number Space | Ze proven 20, Ze correctness gap 10 | Ze MUST proven 20, SHOULD not decided 6, Ze correctness gap 10 | route-server: required by rfc/short/rfc6793.md |
| [RFC 7911](../../rfc7911/) | Advertisement of Multiple Paths in BGP | Ze proven 9 | Ze MUST proven 9, SHOULD not decided 4 | route-server: required by rfc/short/rfc7911.md |
| [RFC 7947](../../rfc7947/) | Internet Exchange BGP Route Server | Ze proven 2, Ze correctness gap 1 | Ze MUST proven 2, SHOULD proven 3, SHOULD not decided 2, Ze correctness gap 1 | route-server: required by rfc/short/rfc7947.md |

### Level 7

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 6286](../../rfc6286/) | Autonomous-System-Wide Unique BGP Identifier for BGP-4 | Ze correctness gap 4 | SHOULD proven 1, Ze correctness gap 4 | route-server: required by rfc/short/rfc6286.md |
| [RFC 6811](../../rfc6811/) | BGP Prefix Origin Validation | Ze proven 1, Ze correctness gap 4 | Ze MUST proven 1, SHOULD not decided 4, Ze correctness gap 4 | route-server: required by rfc/short/rfc6811.md |
| [RFC 7606](../../rfc7606/) (cycle with rfc4364, rfc4659, rfc4798, rfc6368, rfc8277, rfc8950) | Revised Error Handling for BGP UPDATE Messages | Ze proven 28, Ze correctness gap 25 | Ze MUST proven 28, SHOULD not decided 4, Ze correctness gap 25 | route-server: required by rfc/short/rfc7606.md |
| [RFC 7948](../../rfc7948/) | Internet Exchange BGP Route Server Operations | no rows | SHOULD not decided 11 | route-server: required by rfc/short/rfc7948.md |

### Level 8

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 7454](../../rfc7454/) | BGP Operations and Security | no rows | SHOULD not decided 64 | route-server: required by rfc/short/rfc7454.md |
| [RFC 7607](../../rfc7607/) | Codification of AS 0 Processing | Ze correctness gap 5 | Ze correctness gap 5 | route-server: required by rfc/short/rfc7607.md |
| [RFC 7999](../../rfc7999/) | BLACKHOLE Community | Ze proven 4 | Ze MUST proven 4, SHOULD proven 3, SHOULD not decided 9 | route-server: required by rfc/short/rfc7999.md |
| [RFC 8092](../../rfc8092/) | BGP Large Communities Attribute | Ze proven 7 | Ze MUST proven 7, SHOULD not decided 4 | route-server: required by rfc/short/rfc8092.md |
| [RFC 8893](../../rfc8893/) | Resource Public Key Infrastructure (RPKI) Origin Validation for BGP Export | Ze correctness gap 4 | SHOULD not decided 2, Ze correctness gap 4 | route-server: required by rfc/short/rfc8893.md |
| [RFC 9234](../../rfc9234/) | Route Leak Prevention and Detection Using Roles in UPDATE and OPEN Messages | Ze proven 8, Ze correctness gap 11 | Ze MUST proven 8, SHOULD proven 1, SHOULD not decided 4, Ze correctness gap 11 | route-server: required by rfc/short/rfc9234.md |
| [RFC 9774](../../rfc9774/) | Deprecation of AS_SET and AS_CONFED_SET in BGP | Ze proven 2, Ze correctness gap 1 | Ze MUST proven 2, Ze correctness gap 1 | route-server: required by rfc/short/rfc9774.md |

### Level 9

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 8210](../../rfc8210/) | The Resource Public Key Infrastructure (RPKI) to Router Protocol, Version 1 | Ze proven 12, Ze correctness gap 44 | Ze MUST proven 12, SHOULD proven 6, Not owed 3, SHOULD not decided 21, Ze correctness gap 44 | route-server: required by rfc/short/rfc8210.md |

## Optional (not counted)

RFCs this use case lists as optional, each declared by the Optional use cases row of its own summary. They count in none of the figures above until the owner brings one into scope by moving it to the Use cases row. Their own bars and figures follow.

- **MUST bar**: Ze proven 57, Ze correctness gap 39
- **Overview bar**: Ze MUST proven 57, SHOULD proven 5, Not owed 1, SHOULD not decided 52, Ze correctness gap 39

| Figure | Rows |
|---|---|
| RFCs | 12 |
| Not walked | 0 |
| Mandatory register | not complete |
| MUST rows | 97 |
| MUST rows kept (the MUST bar) | 96 |
| Ze proven | 57 |
| OS verified | 0 |
| OS testing gap | 0 |
| Ze correctness gap | 39 |
| MUST rows not owed | 1 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 1 |
| SHOULD / MAY rows | 57 |
| SHOULD proven | 5 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 52 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 2385](../../rfc2385/) | Protection of BGP Sessions via the TCP MD5 Signature Option | Ze proven 8 | Ze MUST proven 8, SHOULD not decided 1 | route-server: optional in rfc/short/rfc2385.md |
| [RFC 9072](../../rfc9072/) | Extended Optional Parameters Length for BGP OPEN Message | Ze proven 5, Ze correctness gap 4 | Ze MUST proven 5, SHOULD proven 1, SHOULD not decided 2, Ze correctness gap 4 | route-server: optional in rfc/short/rfc9072.md |
| [RFC 4360](../../rfc4360/) | BGP Extended Communities Attribute | Ze proven 2, Ze correctness gap 3 | Ze MUST proven 2, SHOULD not decided 4, Ze correctness gap 3 | route-server: optional in rfc/short/rfc4360.md |
| [RFC 5082](../../rfc5082/) | The Generalized TTL Security Mechanism (GTSM) | Ze proven 4 | Ze MUST proven 4, SHOULD proven 1, SHOULD not decided 5 | route-server: optional in rfc/short/rfc5082.md |
| [RFC 2918](../../rfc2918/) | Route Refresh Capability for BGP-4 | Ze proven 3, Ze correctness gap 3 | Ze MUST proven 3, SHOULD proven 1, SHOULD not decided 4, Ze correctness gap 3 | route-server: optional in rfc/short/rfc2918.md |
| [RFC 4486](../../rfc4486/) | Subcodes for BGP Cease NOTIFICATION Message | Ze correctness gap 1 | SHOULD not decided 10, Ze correctness gap 1 | route-server: optional in rfc/short/rfc4486.md |
| [RFC 4724](../../rfc4724/) | Graceful Restart Mechanism for BGP | Ze proven 14, Ze correctness gap 12 | Ze MUST proven 14, SHOULD proven 1, SHOULD not decided 4, Ze correctness gap 12 | route-server: optional in rfc/short/rfc4724.md |
| [RFC 7313](../../rfc7313/) | Enhanced Route Refresh Capability for BGP-4 | Ze proven 5, Ze correctness gap 6 | Ze MUST proven 5, SHOULD not decided 6, Ze correctness gap 6 | route-server: optional in rfc/short/rfc7313.md |
| [RFC 8203](../../rfc8203/) | BGP Administrative Shutdown Communication | Ze proven 3, Ze correctness gap 2 | Ze MUST proven 3, SHOULD not decided 4, Ze correctness gap 2 | route-server: optional in rfc/short/rfc8203.md |
| [RFC 8950](../../rfc8950/) (cycle with rfc4364, rfc4659, rfc4798, rfc6368, rfc7606, rfc8277) | Advertising IPv4 Network Layer Reachability Information (NLRI) with an IPv6 Next Hop | Ze proven 3, Ze correctness gap 3 | Ze MUST proven 3, SHOULD not decided 2, Ze correctness gap 3 | route-server: optional in rfc/short/rfc8950.md |
| [RFC 9003](../../rfc9003/) | Extended BGP Administrative Shutdown Communication | Ze proven 1, Ze correctness gap 3 | Ze MUST proven 1, SHOULD not decided 7, Ze correctness gap 3 | route-server: optional in rfc/short/rfc9003.md |
| [RFC 8654](../../rfc8654/) | Extended Message Support for BGP | Ze proven 9, Ze correctness gap 2 | Ze MUST proven 9, SHOULD proven 1, Not owed 1, SHOULD not decided 3, Ze correctness gap 2 | route-server: optional in rfc/short/rfc8654.md |
