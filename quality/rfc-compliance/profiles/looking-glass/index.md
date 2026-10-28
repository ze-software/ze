# Looking glass

Release profile. The RFCs this use case requires, each declared by the Use cases row of its own summary. RFCs it lists as optional follow in their own table and never enter its figures. Before release, every mandatory requirement of every required RFC must be proven by a recorded break of its test.

[All profiles](../) · [All RFCs](../../)

Use case: [AS-Path Topology](../../../../use-cases/as-path-topology/)

- **Mandatory**: 53.7% (165 of 307 (53.7%)). MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that a recorded test break proves.
- **Optional**: 5.4% (5 of 93 (5.4%)). MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows, counted the same way.
- **Mandatory ceiling**: 97.1% (298 of 307 (97.1%); excluded: Not applicable (classification can be revisited): 9 of 307 (2.9%)). Attainable mandatory rows under current classifications, over the unchanged total.
- **Optional ceiling**: 100.0% (93 of 93 (100.0%)). Attainable optional rows under current classifications, over the unchanged total.
- **Not walked**: 0 (of 16 RFCs). RFCs with no valid extraction sign-off: their rows count in neither figure.
- **Mandatory register**: not complete (every row proven, none unwalked). A profile is complete only when it requires at least one RFC and none of them is unwalked.

Mandatory ceiling: 298 of 307 (97.1%); excluded: Not applicable (classification can be revisited): 9 of 307 (2.9%). Optional ceiling: 93 of 93 (100.0%). Lowest mandatory ceiling: rfc5492 8 of 9 (88.9%). Lowest optional ceiling: rfc1997 4 of 4 (100.0%). Ceilings describe current classifications, not an immutable Linux maximum. Not-applicable and feature-declined classifications can be revisited. Every gap remains work Ze owes.

## Tree

Roots first: an RFC sits one level above the highest level of the RFCs it builds on, and the RFCs of one citation cycle share a level. Each bar is the share of that figure's rows a recorded break proves; the ceiling below it shows attainable rows under current classifications. An RFC that is not walked shows an empty bar and no figure or ceiling. The last column names the summary whose Use cases row declares the RFC: a normative citation never adds one.

### Level 3

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 4271](../../rfc4271/) | A Border Gateway Protocol 4 (BGP-4) | 83 of 133 (62.4%); ceiling 126 of 133 (94.7%); excluded: Not applicable (classification can be revisited): 7 of 133 (5.3%) | 3 of 34 (8.8%); ceiling 34 of 34 (100.0%) | looking-glass: required by rfc/short/rfc4271.md |

### Level 4

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 1997](../../rfc1997/) | BGP Communities Attribute | 3 of 5 (60.0%); ceiling 5 of 5 (100.0%) | 0 of 4 (0.0%); ceiling 4 of 4 (100.0%) | looking-glass: required by rfc/short/rfc1997.md |
| [RFC 2545](../../rfc2545/) | Use of BGP-4 Multiprotocol Extensions for IPv6 Inter-Domain Routing | 3 of 5 (60.0%); ceiling 5 of 5 (100.0%) | 0 of 2 (0.0%); ceiling 2 of 2 (100.0%) | looking-glass: required by rfc/short/rfc2545.md |
| [RFC 5492](../../rfc5492/) | Capabilities Advertisement with BGP-4 | 2 of 9 (22.2%); ceiling 8 of 9 (88.9%); excluded: Not applicable (classification can be revisited): 1 of 9 (11.1%) | 0 of 7 (0.0%); ceiling 7 of 7 (100.0%) | looking-glass: required by rfc/short/rfc5492.md |
| [RFC 6608](../../rfc6608/) | Subcodes for BGP Finite State Machine Error | 0 of 3 (0.0%); ceiling 3 of 3 (100.0%) | no rows; ceiling no rows | looking-glass: required by rfc/short/rfc6608.md |
| [RFC 9687](../../rfc9687/) | Border Gateway Protocol 4 (BGP-4) Send Hold Timer | 3 of 13 (23.1%); ceiling 13 of 13 (100.0%) | 0 of 7 (0.0%); ceiling 7 of 7 (100.0%) | looking-glass: required by rfc/short/rfc9687.md |

### Level 5

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 4760](../../rfc4760/) | Multiprotocol Extensions for BGP-4 | 5 of 6 (83.3%); ceiling 6 of 6 (100.0%) | 1 of 11 (9.1%); ceiling 11 of 11 (100.0%) | looking-glass: required by rfc/short/rfc4760.md |

### Level 6

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 6793](../../rfc6793/) (cycle with rfc5668) | BGP Support for Four-Octet Autonomous System (AS) Number Space | 20 of 30 (66.7%); ceiling 30 of 30 (100.0%) | 0 of 6 (0.0%); ceiling 6 of 6 (100.0%) | looking-glass: required by rfc/short/rfc6793.md |
| [RFC 7911](../../rfc7911/) | Advertisement of Multiple Paths in BGP | 9 of 9 (100.0%); ceiling 9 of 9 (100.0%) | 0 of 4 (0.0%); ceiling 4 of 4 (100.0%) | looking-glass: required by rfc/short/rfc7911.md |
| [RFC 8522](../../rfc8522/) | Looking Glass Command Set | 0 of 18 (0.0%); ceiling 18 of 18 (100.0%) | 0 of 5 (0.0%); ceiling 5 of 5 (100.0%) | looking-glass: required by rfc/short/rfc8522.md |

### Level 7

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 6286](../../rfc6286/) | Autonomous-System-Wide Unique BGP Identifier for BGP-4 | 0 of 4 (0.0%); ceiling 4 of 4 (100.0%) | 1 of 1 (100.0%); ceiling 1 of 1 (100.0%) | looking-glass: required by rfc/short/rfc6286.md |
| [RFC 6811](../../rfc6811/) | BGP Prefix Origin Validation | 1 of 5 (20.0%); ceiling 5 of 5 (100.0%) | 0 of 4 (0.0%); ceiling 4 of 4 (100.0%) | looking-glass: required by rfc/short/rfc6811.md |
| [RFC 7606](../../rfc7606/) (cycle with rfc4364, rfc4659, rfc4798, rfc6368, rfc8277, rfc8950) | Revised Error Handling for BGP UPDATE Messages | 28 of 53 (52.8%); ceiling 52 of 53 (98.1%); excluded: Not applicable (classification can be revisited): 1 of 53 (1.9%) | 0 of 4 (0.0%); ceiling 4 of 4 (100.0%) | looking-glass: required by rfc/short/rfc7606.md |

### Level 8

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 7607](../../rfc7607/) | Codification of AS 0 Processing | 0 of 5 (0.0%); ceiling 5 of 5 (100.0%) | no rows; ceiling no rows | looking-glass: required by rfc/short/rfc7607.md |
| [RFC 8092](../../rfc8092/) | BGP Large Communities Attribute | 6 of 7 (85.7%); ceiling 7 of 7 (100.0%) | 0 of 4 (0.0%); ceiling 4 of 4 (100.0%) | looking-glass: required by rfc/short/rfc8092.md |
| [RFC 9774](../../rfc9774/) | Deprecation of AS_SET and AS_CONFED_SET in BGP | 2 of 2 (100.0%); ceiling 2 of 2 (100.0%) | no rows; ceiling no rows | looking-glass: required by rfc/short/rfc9774.md |

## Optional (not counted)

RFCs this use case lists as optional, each declared by the Optional use cases row of its own summary. They count in none of the figures above until the owner brings one into scope by moving it to the Use cases row. Their own figures: mandatory 14 of 64 (21.9%), optional 6 of 31 (19.4%); ceiling mandatory 36 of 64 (56.2%), optional 29 of 31 (93.5%); mandatory excluded: not-applicable 28; optional excluded: not-applicable 2; 2 RFCs, 0 not walked.

Mandatory ceiling: 36 of 64 (56.2%); excluded: Not applicable (classification can be revisited): 28 of 64 (43.8%). Optional ceiling: 29 of 31 (93.5%); excluded: Not applicable (classification can be revisited): 2 of 31 (6.5%). Lowest mandatory ceiling: rfc4360 2 of 5 (40.0%). Lowest optional ceiling: rfc8210 25 of 27 (92.6%). Ceilings describe current classifications, not an immutable Linux maximum. Not-applicable and feature-declined classifications can be revisited. Every gap remains work Ze owes.

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 4360](../../rfc4360/) | BGP Extended Communities Attribute | 2 of 5 (40.0%); ceiling 2 of 5 (40.0%); excluded: Not applicable (classification can be revisited): 3 of 5 (60.0%) | 0 of 4 (0.0%); ceiling 4 of 4 (100.0%) | looking-glass: optional in rfc/short/rfc4360.md |
| [RFC 8210](../../rfc8210/) | The Resource Public Key Infrastructure (RPKI) to Router Protocol, Version 1 | 12 of 59 (20.3%); ceiling 34 of 59 (57.6%); excluded: Not applicable (classification can be revisited): 25 of 59 (42.4%) | 6 of 27 (22.2%); ceiling 25 of 27 (92.6%); excluded: Not applicable (classification can be revisited): 2 of 27 (7.4%) | looking-glass: optional in rfc/short/rfc8210.md |
