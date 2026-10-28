# Route collector

Release profile. The RFCs this use case requires, each declared by the Use cases row of its own summary. RFCs it lists as optional follow in their own table and never enter its figures. Before release, every mandatory requirement of every required RFC must be proven by a recorded break of its test.

[All profiles](../) · [All RFCs](../../)

- **Mandatory**: 56.1% (202 of 360 (56.1%)). MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that a recorded test break proves.
- **Optional**: 8.1% (8 of 99 (8.1%)). MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows, counted the same way.
- **Not walked**: 0 (of 18 RFCs). RFCs with no valid extraction sign-off: their rows count in neither figure.
- **Mandatory register**: not complete (every row proven, none unwalked). A profile is complete only when it requires at least one RFC and none of them is unwalked.

## Tree

Roots first: an RFC sits one level above the highest level of the RFCs it builds on, and the RFCs of one citation cycle share a level. Each bar is the share of that figure's rows a recorded break proves; an RFC that is not walked shows an empty bar and no figure. The last column names the summary whose Use cases row declares the RFC: a normative citation never adds one.

### Level 3

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 4271](../../rfc4271/) | A Border Gateway Protocol 4 (BGP-4) | 83 of 133 (62.4%) | 3 of 34 (8.8%) | route-collector: required by rfc/short/rfc4271.md |

### Level 4

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 2545](../../rfc2545/) | Use of BGP-4 Multiprotocol Extensions for IPv6 Inter-Domain Routing | 3 of 5 (60.0%) | 0 of 2 (0.0%) | route-collector: required by rfc/short/rfc2545.md |
| [RFC 5492](../../rfc5492/) | Capabilities Advertisement with BGP-4 | 2 of 9 (22.2%) | 0 of 7 (0.0%) | route-collector: required by rfc/short/rfc5492.md |
| [RFC 6608](../../rfc6608/) | Subcodes for BGP Finite State Machine Error | 0 of 3 (0.0%) | no rows | route-collector: required by rfc/short/rfc6608.md |
| [RFC 9687](../../rfc9687/) | Border Gateway Protocol 4 (BGP-4) Send Hold Timer | 3 of 13 (23.1%) | 0 of 7 (0.0%) | route-collector: required by rfc/short/rfc9687.md |

### Level 5

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 4760](../../rfc4760/) | Multiprotocol Extensions for BGP-4 | 5 of 6 (83.3%) | 1 of 11 (9.1%) | route-collector: required by rfc/short/rfc4760.md |

### Level 6

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 6793](../../rfc6793/) (cycle with rfc5668) | BGP Support for Four-Octet Autonomous System (AS) Number Space | 20 of 30 (66.7%) | 0 of 6 (0.0%) | route-collector: required by rfc/short/rfc6793.md |
| [RFC 7911](../../rfc7911/) | Advertisement of Multiple Paths in BGP | 9 of 9 (100.0%) | 0 of 4 (0.0%) | route-collector: required by rfc/short/rfc7911.md |

### Level 7

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 6286](../../rfc6286/) | Autonomous-System-Wide Unique BGP Identifier for BGP-4 | 0 of 4 (0.0%) | 1 of 1 (100.0%) | route-collector: required by rfc/short/rfc6286.md |
| [RFC 7606](../../rfc7606/) (cycle with rfc4364, rfc4659, rfc4798, rfc6368, rfc8277, rfc8950) | Revised Error Handling for BGP UPDATE Messages | 28 of 53 (52.8%) | 0 of 4 (0.0%) | route-collector: required by rfc/short/rfc7606.md |

### Level 8

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 7607](../../rfc7607/) | Codification of AS 0 Processing | 0 of 5 (0.0%) | no rows | route-collector: required by rfc/short/rfc7607.md |
| [RFC 7854](../../rfc7854/) | BGP Monitoring Protocol (BMP) | 25 of 39 (64.1%) | 3 of 9 (33.3%) | route-collector: required by rfc/short/rfc7854.md |
| [RFC 9774](../../rfc9774/) | Deprecation of AS_SET and AS_CONFED_SET in BGP | 2 of 2 (100.0%) | no rows | route-collector: required by rfc/short/rfc9774.md |

### Level 9

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 8671](../../rfc8671/) | Support for Adj-RIB-Out in the BGP Monitoring Protocol (BMP) | 2 of 10 (20.0%) | 0 of 1 (0.0%) | route-collector: required by rfc/short/rfc8671.md |
| [RFC 9069](../../rfc9069/) | Support for Local RIB in the BGP Monitoring Protocol (BMP) | 9 of 16 (56.2%) | no rows | route-collector: required by rfc/short/rfc9069.md |

### Level 10

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 6396](../../rfc6396/) | Multi-Threaded Routing Toolkit (MRT) Routing Information Export Format | 6 of 13 (46.2%) | 0 of 12 (0.0%) | route-collector: required by rfc/short/rfc6396.md |

### Level 11

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 6397](../../rfc6397/) | Multi-Threaded Routing Toolkit (MRT) Border Gateway Protocol (BGP) Routing Information Export Format with Geo-Location Extensions | 0 of 4 (0.0%) | 0 of 1 (0.0%) | route-collector: required by rfc/short/rfc6397.md |
| [RFC 8050](../../rfc8050/) | Multi-Threaded Routing Toolkit (MRT) Routing Information Export Format with BGP Additional Path Extensions | 5 of 6 (83.3%) | no rows | route-collector: required by rfc/short/rfc8050.md |

## Optional (not counted)

RFCs this use case lists as optional, each declared by the Optional use cases row of its own summary. They count in none of the figures above until the owner brings one into scope by moving it to the Use cases row. Their own figures: mandatory 17 of 27 (63.0%), optional 3 of 12 (25.0%); 3 RFCs, 0 not walked.

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 9072](../../rfc9072/) | Extended Optional Parameters Length for BGP OPEN Message | 5 of 9 (55.6%) | 1 of 3 (33.3%) | route-collector: optional in rfc/short/rfc9072.md |
| [RFC 2918](../../rfc2918/) | Route Refresh Capability for BGP-4 | 3 of 6 (50.0%) | 1 of 5 (20.0%) | route-collector: optional in rfc/short/rfc2918.md |
| [RFC 8654](../../rfc8654/) | Extended Message Support for BGP | 9 of 12 (75.0%) | 1 of 4 (25.0%) | route-collector: optional in rfc/short/rfc8654.md |
