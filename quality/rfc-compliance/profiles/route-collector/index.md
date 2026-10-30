# Route collector

Release profile. The RFCs this use case requires, each declared by the Use cases row of its own summary. RFCs it lists as optional follow in their own table and never enter its figures. Before release, every mandatory requirement of every required RFC must be proven by a recorded break of its test.

[All profiles](../) · [All RFCs](../../)

- **Mandatory**: 54.7% (198 of 362 (54.7%)). MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that a recorded test break proves.
- **Optional**: 8.1% (8 of 99 (8.1%)). MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows, counted the same way.
- **Mandatory ceiling**: 100.0% (362 of 362 (100.0%)). Attainable mandatory rows under current classifications, over the unchanged total.
- **Optional ceiling**: 100.0% (99 of 99 (100.0%)). Attainable optional rows under current classifications, over the unchanged total.
- **Not walked**: 0 (of 18 RFCs). RFCs with no valid extraction sign-off: their rows count in neither figure.
- **Mandatory register**: not complete (every row proven, none unwalked). A profile is complete only when it requires at least one RFC and none of them is unwalked.

Mandatory ceiling: 362 of 362 (100.0%). Optional ceiling: 99 of 99 (100.0%). Every RFC shown with measured rows can reach 100% under current classifications. Lowest mandatory ceiling: rfc2545 5 of 5 (100.0%). Lowest optional ceiling: rfc2545 2 of 2 (100.0%). Ceilings describe current classifications, not an immutable Linux maximum. Not-applicable and feature-declined classifications can be revisited. Every gap remains work Ze owes.

## Tree

Roots first: an RFC sits one level above the highest level of the RFCs it builds on, and the RFCs of one citation cycle share a level. Each bar is the share of that figure's rows a recorded break proves; the ceiling below it shows attainable rows under current classifications. An RFC that is not walked shows an empty bar and no figure or ceiling. The last column names the summary whose Use cases row declares the RFC: a normative citation never adds one.

### Level 3

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 4271](../../rfc4271/) | A Border Gateway Protocol 4 (BGP-4) | 78 of 133 (58.6%); ceiling 133 of 133 (100.0%) | 3 of 34 (8.8%); ceiling 34 of 34 (100.0%) | route-collector: required by rfc/short/rfc4271.md |

### Level 4

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 2545](../../rfc2545/) | Use of BGP-4 Multiprotocol Extensions for IPv6 Inter-Domain Routing | 5 of 5 (100.0%); ceiling 5 of 5 (100.0%) | 0 of 2 (0.0%); ceiling 2 of 2 (100.0%) | route-collector: required by rfc/short/rfc2545.md |
| [RFC 5492](../../rfc5492/) | Capabilities Advertisement with BGP-4 | 2 of 9 (22.2%); ceiling 9 of 9 (100.0%) | 0 of 7 (0.0%); ceiling 7 of 7 (100.0%) | route-collector: required by rfc/short/rfc5492.md |
| [RFC 6608](../../rfc6608/) | Subcodes for BGP Finite State Machine Error | 0 of 3 (0.0%); ceiling 3 of 3 (100.0%) | no rows; ceiling no rows | route-collector: required by rfc/short/rfc6608.md |
| [RFC 9687](../../rfc9687/) | Border Gateway Protocol 4 (BGP-4) Send Hold Timer | 3 of 13 (23.1%); ceiling 13 of 13 (100.0%) | 0 of 7 (0.0%); ceiling 7 of 7 (100.0%) | route-collector: required by rfc/short/rfc9687.md |

### Level 5

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 4760](../../rfc4760/) | Multiprotocol Extensions for BGP-4 | 6 of 6 (100.0%); ceiling 6 of 6 (100.0%) | 1 of 11 (9.1%); ceiling 11 of 11 (100.0%) | route-collector: required by rfc/short/rfc4760.md |

### Level 6

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 6793](../../rfc6793/) (cycle with rfc5668) | BGP Support for Four-Octet Autonomous System (AS) Number Space | 20 of 30 (66.7%); ceiling 30 of 30 (100.0%) | 0 of 6 (0.0%); ceiling 6 of 6 (100.0%) | route-collector: required by rfc/short/rfc6793.md |
| [RFC 7911](../../rfc7911/) | Advertisement of Multiple Paths in BGP | 9 of 9 (100.0%); ceiling 9 of 9 (100.0%) | 0 of 4 (0.0%); ceiling 4 of 4 (100.0%) | route-collector: required by rfc/short/rfc7911.md |

### Level 7

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 6286](../../rfc6286/) | Autonomous-System-Wide Unique BGP Identifier for BGP-4 | 0 of 4 (0.0%); ceiling 4 of 4 (100.0%) | 1 of 1 (100.0%); ceiling 1 of 1 (100.0%) | route-collector: required by rfc/short/rfc6286.md |
| [RFC 7606](../../rfc7606/) (cycle with rfc4364, rfc4659, rfc4798, rfc6368, rfc8277, rfc8950) | Revised Error Handling for BGP UPDATE Messages | 26 of 53 (49.1%); ceiling 53 of 53 (100.0%) | 0 of 4 (0.0%); ceiling 4 of 4 (100.0%) | route-collector: required by rfc/short/rfc7606.md |

### Level 8

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 7607](../../rfc7607/) | Codification of AS 0 Processing | 0 of 5 (0.0%); ceiling 5 of 5 (100.0%) | no rows; ceiling no rows | route-collector: required by rfc/short/rfc7607.md |
| [RFC 7854](../../rfc7854/) | BGP Monitoring Protocol (BMP) | 25 of 39 (64.1%); ceiling 39 of 39 (100.0%) | 3 of 9 (33.3%); ceiling 9 of 9 (100.0%) | route-collector: required by rfc/short/rfc7854.md |
| [RFC 9774](../../rfc9774/) | Deprecation of AS_SET and AS_CONFED_SET in BGP | 2 of 3 (66.7%); ceiling 3 of 3 (100.0%) | no rows; ceiling no rows | route-collector: required by rfc/short/rfc9774.md |

### Level 9

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 8671](../../rfc8671/) | Support for Adj-RIB-Out in the BGP Monitoring Protocol (BMP) | 2 of 11 (18.2%); ceiling 11 of 11 (100.0%) | 0 of 1 (0.0%); ceiling 1 of 1 (100.0%) | route-collector: required by rfc/short/rfc8671.md |
| [RFC 9069](../../rfc9069/) | Support for Local RIB in the BGP Monitoring Protocol (BMP) | 9 of 16 (56.2%); ceiling 16 of 16 (100.0%) | no rows; ceiling no rows | route-collector: required by rfc/short/rfc9069.md |

### Level 10

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 6396](../../rfc6396/) | Multi-Threaded Routing Toolkit (MRT) Routing Information Export Format | 6 of 13 (46.2%); ceiling 13 of 13 (100.0%) | 0 of 12 (0.0%); ceiling 12 of 12 (100.0%) | route-collector: required by rfc/short/rfc6396.md |

### Level 11

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 6397](../../rfc6397/) | Multi-Threaded Routing Toolkit (MRT) Border Gateway Protocol (BGP) Routing Information Export Format with Geo-Location Extensions | 0 of 4 (0.0%); ceiling 4 of 4 (100.0%) | 0 of 1 (0.0%); ceiling 1 of 1 (100.0%) | route-collector: required by rfc/short/rfc6397.md |
| [RFC 8050](../../rfc8050/) | Multi-Threaded Routing Toolkit (MRT) Routing Information Export Format with BGP Additional Path Extensions | 5 of 6 (83.3%); ceiling 6 of 6 (100.0%) | no rows; ceiling no rows | route-collector: required by rfc/short/rfc8050.md |

## Optional (not counted)

RFCs this use case lists as optional, each declared by the Optional use cases row of its own summary. They count in none of the figures above until the owner brings one into scope by moving it to the Use cases row. Their own figures: mandatory 17 of 27 (63.0%), optional 3 of 12 (25.0%); ceiling mandatory 26 of 27 (96.3%), optional 12 of 12 (100.0%); mandatory excluded: not-applicable 1; 3 RFCs, 0 not walked.

Mandatory ceiling: 26 of 27 (96.3%); excluded: Not applicable (classification can be revisited): 1 of 27 (3.7%). Optional ceiling: 12 of 12 (100.0%). Lowest mandatory ceiling: rfc8654 11 of 12 (91.7%). Lowest optional ceiling: rfc2918 5 of 5 (100.0%). Ceilings describe current classifications, not an immutable Linux maximum. Not-applicable and feature-declined classifications can be revisited. Every gap remains work Ze owes.

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 9072](../../rfc9072/) | Extended Optional Parameters Length for BGP OPEN Message | 5 of 9 (55.6%); ceiling 9 of 9 (100.0%) | 1 of 3 (33.3%); ceiling 3 of 3 (100.0%) | route-collector: optional in rfc/short/rfc9072.md |
| [RFC 2918](../../rfc2918/) | Route Refresh Capability for BGP-4 | 3 of 6 (50.0%); ceiling 6 of 6 (100.0%) | 1 of 5 (20.0%); ceiling 5 of 5 (100.0%) | route-collector: optional in rfc/short/rfc2918.md |
| [RFC 8654](../../rfc8654/) | Extended Message Support for BGP | 9 of 12 (75.0%); ceiling 11 of 12 (91.7%); excluded: Not applicable (classification can be revisited): 1 of 12 (8.3%) | 1 of 4 (25.0%); ceiling 4 of 4 (100.0%) | route-collector: optional in rfc/short/rfc8654.md |
