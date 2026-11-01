# Looking glass

Release profile. The RFCs this use case requires, each declared by the Use cases row of its own summary. RFCs it lists as optional follow in their own table and never enter its figures. Before release, every mandatory requirement of every required RFC must be proven by a recorded break of its test.

[All profiles](../) · [All RFCs](../../)

Use case: [AS-Path Topology](../../../../use-cases/as-path-topology/)

Figures as of the last build, read from [data/rfc/profiles/looking-glass.json](../../../../data/rfc/profiles/looking-glass.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze proven 175, Ze correctness gap 133
- **Overview bar**: Ze MUST proven 175, SHOULD proven 5, SHOULD not decided 88, Ze correctness gap 133

| Figure | Rows |
|---|---|
| RFCs | 16 |
| Not walked | 0 |
| Mandatory register | not complete |
| MUST rows | 308 |
| MUST rows kept (the MUST bar) | 308 |
| Ze proven | 175 |
| OS verified | 0 |
| OS testing gap | 0 |
| Ze correctness gap | 133 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 93 |
| SHOULD proven | 5 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 88 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

## Tree

Roots first: an RFC sits one level above the highest level of the RFCs it builds on, and the RFCs of one citation cycle share a level. Each RFC carries the same two bars as the set above, from its own data file; an RFC that is not walked shows "not walked" and no bar. The last column names the summary whose Use cases row declares the RFC: a normative citation never adds one.

### Level 3

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 4271](../../rfc4271/) | A Border Gateway Protocol 4 (BGP-4) | Ze proven 83, Ze correctness gap 50 | Ze MUST proven 83, SHOULD proven 3, SHOULD not decided 31, Ze correctness gap 50 | looking-glass: required by rfc/short/rfc4271.md |

### Level 4

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 1997](../../rfc1997/) | BGP Communities Attribute | Ze proven 5 | Ze MUST proven 5, SHOULD not decided 4 | looking-glass: required by rfc/short/rfc1997.md |
| [RFC 2545](../../rfc2545/) | Use of BGP-4 Multiprotocol Extensions for IPv6 Inter-Domain Routing | Ze proven 3, Ze correctness gap 2 | Ze MUST proven 3, SHOULD not decided 2, Ze correctness gap 2 | looking-glass: required by rfc/short/rfc2545.md |
| [RFC 5492](../../rfc5492/) | Capabilities Advertisement with BGP-4 | Ze proven 8, Ze correctness gap 1 | Ze MUST proven 8, SHOULD not decided 7, Ze correctness gap 1 | looking-glass: required by rfc/short/rfc5492.md |
| [RFC 6608](../../rfc6608/) | Subcodes for BGP Finite State Machine Error | Ze correctness gap 3 | Ze correctness gap 3 | looking-glass: required by rfc/short/rfc6608.md |
| [RFC 9687](../../rfc9687/) | Border Gateway Protocol 4 (BGP-4) Send Hold Timer | Ze proven 3, Ze correctness gap 10 | Ze MUST proven 3, SHOULD not decided 7, Ze correctness gap 10 | looking-glass: required by rfc/short/rfc9687.md |

### Level 5

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 4760](../../rfc4760/) | Multiprotocol Extensions for BGP-4 | Ze proven 6 | Ze MUST proven 6, SHOULD proven 1, SHOULD not decided 10 | looking-glass: required by rfc/short/rfc4760.md |

### Level 6

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 6793](../../rfc6793/) (cycle with rfc5668) | BGP Support for Four-Octet Autonomous System (AS) Number Space | Ze proven 20, Ze correctness gap 10 | Ze MUST proven 20, SHOULD not decided 6, Ze correctness gap 10 | looking-glass: required by rfc/short/rfc6793.md |
| [RFC 7911](../../rfc7911/) | Advertisement of Multiple Paths in BGP | Ze proven 9 | Ze MUST proven 9, SHOULD not decided 4 | looking-glass: required by rfc/short/rfc7911.md |
| [RFC 8522](../../rfc8522/) | Looking Glass Command Set | Ze correctness gap 18 | SHOULD not decided 5, Ze correctness gap 18 | looking-glass: required by rfc/short/rfc8522.md |

### Level 7

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 6286](../../rfc6286/) | Autonomous-System-Wide Unique BGP Identifier for BGP-4 | Ze correctness gap 4 | SHOULD proven 1, Ze correctness gap 4 | looking-glass: required by rfc/short/rfc6286.md |
| [RFC 6811](../../rfc6811/) | BGP Prefix Origin Validation | Ze proven 1, Ze correctness gap 4 | Ze MUST proven 1, SHOULD not decided 4, Ze correctness gap 4 | looking-glass: required by rfc/short/rfc6811.md |
| [RFC 7606](../../rfc7606/) (cycle with rfc4364, rfc4659, rfc4798, rfc6368, rfc8277, rfc8950) | Revised Error Handling for BGP UPDATE Messages | Ze proven 28, Ze correctness gap 25 | Ze MUST proven 28, SHOULD not decided 4, Ze correctness gap 25 | looking-glass: required by rfc/short/rfc7606.md |

### Level 8

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 7607](../../rfc7607/) | Codification of AS 0 Processing | Ze correctness gap 5 | Ze correctness gap 5 | looking-glass: required by rfc/short/rfc7607.md |
| [RFC 8092](../../rfc8092/) | BGP Large Communities Attribute | Ze proven 7 | Ze MUST proven 7, SHOULD not decided 4 | looking-glass: required by rfc/short/rfc8092.md |
| [RFC 9774](../../rfc9774/) | Deprecation of AS_SET and AS_CONFED_SET in BGP | Ze proven 2, Ze correctness gap 1 | Ze MUST proven 2, Ze correctness gap 1 | looking-glass: required by rfc/short/rfc9774.md |

## Optional (not counted)

RFCs this use case lists as optional, each declared by the Optional use cases row of its own summary. They count in none of the figures above until the owner brings one into scope by moving it to the Use cases row. Their own bars and figures follow.

- **MUST bar**: Ze proven 14, Ze correctness gap 47
- **Overview bar**: Ze MUST proven 14, SHOULD proven 6, Not owed 3, SHOULD not decided 25, Ze correctness gap 47

| Figure | Rows |
|---|---|
| RFCs | 2 |
| Not walked | 0 |
| Mandatory register | not complete |
| MUST rows | 64 |
| MUST rows kept (the MUST bar) | 61 |
| Ze proven | 14 |
| OS verified | 0 |
| OS testing gap | 0 |
| Ze correctness gap | 47 |
| MUST rows not owed | 3 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 3 |
| SHOULD / MAY rows | 31 |
| SHOULD proven | 6 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 25 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 4360](../../rfc4360/) | BGP Extended Communities Attribute | Ze proven 2, Ze correctness gap 3 | Ze MUST proven 2, SHOULD not decided 4, Ze correctness gap 3 | looking-glass: optional in rfc/short/rfc4360.md |
| [RFC 8210](../../rfc8210/) | The Resource Public Key Infrastructure (RPKI) to Router Protocol, Version 1 | Ze proven 12, Ze correctness gap 44 | Ze MUST proven 12, SHOULD proven 6, Not owed 3, SHOULD not decided 21, Ze correctness gap 44 | looking-glass: optional in rfc/short/rfc8210.md |
