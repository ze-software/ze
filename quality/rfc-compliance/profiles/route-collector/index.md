# Route collector

Release profile. The RFCs this use case requires, each declared by the Use cases row of its own summary. RFCs it lists as optional follow in their own table and never enter its figures. Before release, every mandatory requirement of every required RFC must be proven by a recorded break of its test.

[All profiles](../) · [All RFCs](../../)

Figures as of the last build, read from [data/rfc/profiles/route-collector.json](../../../../data/rfc/profiles/route-collector.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze proven 209, Ze correctness gap 153
- **Overview bar**: Ze MUST proven 209, SHOULD proven 8, SHOULD not decided 91, Ze correctness gap 153

| Figure | Rows |
|---|---|
| RFCs | 18 |
| Not walked | 0 |
| Mandatory register | not complete |
| MUST rows | 362 |
| MUST rows kept (the MUST bar) | 362 |
| Ze proven | 209 |
| OS verified | 0 |
| OS testing gap | 0 |
| Ze correctness gap | 153 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 99 |
| SHOULD proven | 8 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 91 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

## Tree

Roots first: an RFC sits one level above the highest level of the RFCs it builds on, and the RFCs of one citation cycle share a level. Each RFC carries the same two bars as the set above, from its own data file; an RFC that is not walked shows "not walked" and no bar. The last column names the summary whose Use cases row declares the RFC: a normative citation never adds one.

### Level 3

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 4271](../../rfc4271/) | A Border Gateway Protocol 4 (BGP-4) | Ze proven 83, Ze correctness gap 50 | Ze MUST proven 83, SHOULD proven 3, SHOULD not decided 31, Ze correctness gap 50 | route-collector: required by rfc/short/rfc4271.md |

### Level 4

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 2545](../../rfc2545/) | Use of BGP-4 Multiprotocol Extensions for IPv6 Inter-Domain Routing | Ze proven 3, Ze correctness gap 2 | Ze MUST proven 3, SHOULD not decided 2, Ze correctness gap 2 | route-collector: required by rfc/short/rfc2545.md |
| [RFC 5492](../../rfc5492/) | Capabilities Advertisement with BGP-4 | Ze proven 8, Ze correctness gap 1 | Ze MUST proven 8, SHOULD not decided 7, Ze correctness gap 1 | route-collector: required by rfc/short/rfc5492.md |
| [RFC 6608](../../rfc6608/) | Subcodes for BGP Finite State Machine Error | Ze correctness gap 3 | Ze correctness gap 3 | route-collector: required by rfc/short/rfc6608.md |
| [RFC 9687](../../rfc9687/) | Border Gateway Protocol 4 (BGP-4) Send Hold Timer | Ze proven 3, Ze correctness gap 10 | Ze MUST proven 3, SHOULD not decided 7, Ze correctness gap 10 | route-collector: required by rfc/short/rfc9687.md |

### Level 5

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 4760](../../rfc4760/) | Multiprotocol Extensions for BGP-4 | Ze proven 6 | Ze MUST proven 6, SHOULD proven 1, SHOULD not decided 10 | route-collector: required by rfc/short/rfc4760.md |

### Level 6

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 6793](../../rfc6793/) (cycle with rfc5668) | BGP Support for Four-Octet Autonomous System (AS) Number Space | Ze proven 20, Ze correctness gap 10 | Ze MUST proven 20, SHOULD not decided 6, Ze correctness gap 10 | route-collector: required by rfc/short/rfc6793.md |
| [RFC 7911](../../rfc7911/) | Advertisement of Multiple Paths in BGP | Ze proven 9 | Ze MUST proven 9, SHOULD not decided 4 | route-collector: required by rfc/short/rfc7911.md |

### Level 7

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 6286](../../rfc6286/) | Autonomous-System-Wide Unique BGP Identifier for BGP-4 | Ze correctness gap 4 | SHOULD proven 1, Ze correctness gap 4 | route-collector: required by rfc/short/rfc6286.md |
| [RFC 7606](../../rfc7606/) (cycle with rfc4364, rfc4659, rfc4798, rfc6368, rfc8277, rfc8950) | Revised Error Handling for BGP UPDATE Messages | Ze proven 28, Ze correctness gap 25 | Ze MUST proven 28, SHOULD not decided 4, Ze correctness gap 25 | route-collector: required by rfc/short/rfc7606.md |

### Level 8

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 7607](../../rfc7607/) | Codification of AS 0 Processing | Ze correctness gap 5 | Ze correctness gap 5 | route-collector: required by rfc/short/rfc7607.md |
| [RFC 7854](../../rfc7854/) | BGP Monitoring Protocol (BMP) | Ze proven 25, Ze correctness gap 14 | Ze MUST proven 25, SHOULD proven 3, SHOULD not decided 6, Ze correctness gap 14 | route-collector: required by rfc/short/rfc7854.md |
| [RFC 9774](../../rfc9774/) | Deprecation of AS_SET and AS_CONFED_SET in BGP | Ze proven 2, Ze correctness gap 1 | Ze MUST proven 2, Ze correctness gap 1 | route-collector: required by rfc/short/rfc9774.md |

### Level 9

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 8671](../../rfc8671/) | Support for Adj-RIB-Out in the BGP Monitoring Protocol (BMP) | Ze proven 2, Ze correctness gap 9 | Ze MUST proven 2, SHOULD not decided 1, Ze correctness gap 9 | route-collector: required by rfc/short/rfc8671.md |
| [RFC 9069](../../rfc9069/) | Support for Local RIB in the BGP Monitoring Protocol (BMP) | Ze proven 9, Ze correctness gap 7 | Ze MUST proven 9, Ze correctness gap 7 | route-collector: required by rfc/short/rfc9069.md |

### Level 10

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 6396](../../rfc6396/) | Multi-Threaded Routing Toolkit (MRT) Routing Information Export Format | Ze proven 6, Ze correctness gap 7 | Ze MUST proven 6, SHOULD not decided 12, Ze correctness gap 7 | route-collector: required by rfc/short/rfc6396.md |

### Level 11

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 6397](../../rfc6397/) | Multi-Threaded Routing Toolkit (MRT) Border Gateway Protocol (BGP) Routing Information Export Format with Geo-Location Extensions | Ze correctness gap 4 | SHOULD not decided 1, Ze correctness gap 4 | route-collector: required by rfc/short/rfc6397.md |
| [RFC 8050](../../rfc8050/) | Multi-Threaded Routing Toolkit (MRT) Routing Information Export Format with BGP Additional Path Extensions | Ze proven 5, Ze correctness gap 1 | Ze MUST proven 5, Ze correctness gap 1 | route-collector: required by rfc/short/rfc8050.md |

## Optional (not counted)

RFCs this use case lists as optional, each declared by the Optional use cases row of its own summary. They count in none of the figures above until the owner brings one into scope by moving it to the Use cases row. Their own bars and figures follow.

- **MUST bar**: Ze proven 17, Ze correctness gap 9
- **Overview bar**: Ze MUST proven 17, SHOULD proven 3, Not owed 1, SHOULD not decided 9, Ze correctness gap 9

| Figure | Rows |
|---|---|
| RFCs | 3 |
| Not walked | 0 |
| Mandatory register | not complete |
| MUST rows | 27 |
| MUST rows kept (the MUST bar) | 26 |
| Ze proven | 17 |
| OS verified | 0 |
| OS testing gap | 0 |
| Ze correctness gap | 9 |
| MUST rows not owed | 1 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 1 |
| SHOULD / MAY rows | 12 |
| SHOULD proven | 3 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 9 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 9072](../../rfc9072/) | Extended Optional Parameters Length for BGP OPEN Message | Ze proven 5, Ze correctness gap 4 | Ze MUST proven 5, SHOULD proven 1, SHOULD not decided 2, Ze correctness gap 4 | route-collector: optional in rfc/short/rfc9072.md |
| [RFC 2918](../../rfc2918/) | Route Refresh Capability for BGP-4 | Ze proven 3, Ze correctness gap 3 | Ze MUST proven 3, SHOULD proven 1, SHOULD not decided 4, Ze correctness gap 3 | route-collector: optional in rfc/short/rfc2918.md |
| [RFC 8654](../../rfc8654/) | Extended Message Support for BGP | Ze proven 9, Ze correctness gap 2 | Ze MUST proven 9, SHOULD proven 1, Not owed 1, SHOULD not decided 3, Ze correctness gap 2 | route-collector: optional in rfc/short/rfc8654.md |
