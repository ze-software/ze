# Route collector

Release profile. The RFCs this use case requires, each declared by the Use cases row of its own summary. RFCs it lists as optional follow in their own table and never enter its figures. Before release, every mandatory requirement of every required RFC must have a test that has been shown to fail when the behavior it checks is deliberately broken.

[All profiles](../) · [All RFCs](../../)

Figures as of the last build, read from [data/rfc/profiles/route-collector.json](../../../../data/rfc/profiles/route-collector.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze verified 350, Not built 9
- **Overview bar**: Ze MUST verified 350, SHOULD verified 10, Not owed 3, Not assessed 89, Not built 9

| Figure | Rows |
|---|---|
| RFCs | 18 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 362 |
| MUST rows kept (the MUST bar) | 359 |
| Ze verified | 350 |
| OS verified | 0 |
| Tests awaiting verification | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 9 |
| MUST rows not owed | 3 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 3 |
| SHOULD / MAY rows | 99 |
| SHOULD verified | 10 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 89 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

## Tree

The RFCs are listed with the standards they build on first. An RFC appears one level after the highest level of its dependencies; RFCs that refer back to each other share a level. Each RFC has the same two coverage bars as the group above. An RFC whose requirement list has not been reviewed shows "requirements not reviewed" instead of a bar. The last column names the summary whose Use cases row declares the RFC: a normative citation never adds one.

### Level 3

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 4271](../../rfc4271/) | A Border Gateway Protocol 4 (BGP-4) | Ze verified 126, Not built 7 | Ze MUST verified 126, SHOULD verified 4, Not assessed 30, Not built 7 | route-collector: required by rfc/short/rfc4271.md |

### Level 4

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 2545](../../rfc2545/) | Use of BGP-4 Multiprotocol Extensions for IPv6 Inter-Domain Routing | Ze verified 3, Not built 2 | Ze MUST verified 3, Not assessed 2, Not built 2 | route-collector: required by rfc/short/rfc2545.md |
| [RFC 5492](../../rfc5492/) | Capabilities Advertisement with BGP-4 | Ze verified 8 | Ze MUST verified 8, Not owed 1, Not assessed 7 | route-collector: required by rfc/short/rfc5492.md |
| [RFC 6608](../../rfc6608/) | Subcodes for BGP Finite State Machine Error | Ze verified 3 | Ze MUST verified 3 | route-collector: required by rfc/short/rfc6608.md |
| [RFC 9687](../../rfc9687/) | Border Gateway Protocol 4 (BGP-4) Send Hold Timer | Ze verified 13 | Ze MUST verified 13, Not assessed 7 | route-collector: required by rfc/short/rfc9687.md |

### Level 5

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 4760](../../rfc4760/) | Multiprotocol Extensions for BGP-4 | Ze verified 6 | Ze MUST verified 6, SHOULD verified 1, Not assessed 10 | route-collector: required by rfc/short/rfc4760.md |

### Level 6

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 6793](../../rfc6793/) (cycle with rfc5668) | BGP Support for Four-Octet Autonomous System (AS) Number Space | Ze verified 30 | Ze MUST verified 30, Not assessed 6 | route-collector: required by rfc/short/rfc6793.md |
| [RFC 7911](../../rfc7911/) | Advertisement of Multiple Paths in BGP | Ze verified 9 | Ze MUST verified 9, Not assessed 4 | route-collector: required by rfc/short/rfc7911.md |

### Level 7

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 6286](../../rfc6286/) | Autonomous-System-Wide Unique BGP Identifier for BGP-4 | Ze verified 4 | Ze MUST verified 4, SHOULD verified 1 | route-collector: required by rfc/short/rfc6286.md |
| [RFC 7606](../../rfc7606/) (cycle with rfc4364, rfc4659, rfc4798, rfc6368, rfc8277, rfc8950) | Revised Error Handling for BGP UPDATE Messages | Ze verified 52 | Ze MUST verified 52, Not owed 1, Not assessed 4 | route-collector: required by rfc/short/rfc7606.md |

### Level 8

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 7607](../../rfc7607/) | Codification of AS 0 Processing | Ze verified 5 | Ze MUST verified 5 | route-collector: required by rfc/short/rfc7607.md |
| [RFC 7854](../../rfc7854/) | BGP Monitoring Protocol (BMP) | Ze verified 39 | Ze MUST verified 39, SHOULD verified 4, Not assessed 5 | route-collector: required by rfc/short/rfc7854.md |
| [RFC 9774](../../rfc9774/) | Deprecation of AS_SET and AS_CONFED_SET in BGP | Ze verified 3 | Ze MUST verified 3 | route-collector: required by rfc/short/rfc9774.md |

### Level 9

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 8671](../../rfc8671/) | Support for Adj-RIB-Out in the BGP Monitoring Protocol (BMP) | Ze verified 11 | Ze MUST verified 11, Not assessed 1 | route-collector: required by rfc/short/rfc8671.md |
| [RFC 9069](../../rfc9069/) | Support for Local RIB in the BGP Monitoring Protocol (BMP) | Ze verified 16 | Ze MUST verified 16 | route-collector: required by rfc/short/rfc9069.md |

### Level 10

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 6396](../../rfc6396/) | Multi-Threaded Routing Toolkit (MRT) Routing Information Export Format | Ze verified 12 | Ze MUST verified 12, Not owed 1, Not assessed 12 | route-collector: required by rfc/short/rfc6396.md |

### Level 11

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 6397](../../rfc6397/) | Multi-Threaded Routing Toolkit (MRT) Border Gateway Protocol (BGP) Routing Information Export Format with Geo-Location Extensions | Ze verified 4 | Ze MUST verified 4, Not assessed 1 | route-collector: required by rfc/short/rfc6397.md |
| [RFC 8050](../../rfc8050/) | Multi-Threaded Routing Toolkit (MRT) Routing Information Export Format with BGP Additional Path Extensions | Ze verified 6 | Ze MUST verified 6 | route-collector: required by rfc/short/rfc8050.md |

## Optional (not counted)

RFCs this use case lists as optional, each declared by the Optional use cases row of its own summary. They count in none of the figures above until the owner brings one into scope by moving it to the Use cases row. Their own bars and figures follow.

- **MUST bar**: Ze verified 25, Not built 1
- **Overview bar**: Ze MUST verified 25, SHOULD verified 3, Not owed 1, Not assessed 9, Not built 1

| Figure | Rows |
|---|---|
| RFCs | 3 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 27 |
| MUST rows kept (the MUST bar) | 26 |
| Ze verified | 25 |
| OS verified | 0 |
| Tests awaiting verification | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 1 |
| MUST rows not owed | 1 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 1 |
| SHOULD / MAY rows | 12 |
| SHOULD verified | 3 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 9 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 9072](../../rfc9072/) | Extended Optional Parameters Length for BGP OPEN Message | Ze verified 9 | Ze MUST verified 9, SHOULD verified 1, Not assessed 2 | route-collector: optional in rfc/short/rfc9072.md |
| [RFC 2918](../../rfc2918/) | Route Refresh Capability for BGP-4 | Ze verified 6 | Ze MUST verified 6, SHOULD verified 1, Not assessed 4 | route-collector: optional in rfc/short/rfc2918.md |
| [RFC 8654](../../rfc8654/) | Extended Message Support for BGP | Ze verified 10, Not built 1 | Ze MUST verified 10, SHOULD verified 1, Not owed 1, Not assessed 3, Not built 1 | route-collector: optional in rfc/short/rfc8654.md |
