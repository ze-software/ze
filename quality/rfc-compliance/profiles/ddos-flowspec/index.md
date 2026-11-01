# DDoS detection and mitigation by sending FlowSpec upstream

Use-case profile. The RFCs this use case requires, each declared by the Use cases row of its own summary. RFCs it lists as optional follow in their own table and never enter its figures.

[All profiles](../) · [All RFCs](../../)

Use case: [FlowSpec Injection](../../../../use-cases/flowspec-injection/)

Figures as of the last build, read from [data/rfc/profiles/ddos-flowspec.json](../../../../data/rfc/profiles/ddos-flowspec.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze proven 184, OS testing gap 1, Ze correctness gap 153
- **Overview bar**: Ze MUST proven 184, SHOULD proven 16, Not owed 5, SHOULD not decided 108, OS testing gap 1, Ze correctness gap 153

| Figure | Rows |
|---|---|
| RFCs | 13 |
| Not walked | 0 |
| Mandatory register | not complete |
| MUST rows | 343 |
| MUST rows kept (the MUST bar) | 338 |
| Ze proven | 184 |
| OS verified | 0 |
| OS testing gap | 1 |
| Ze correctness gap | 153 |
| MUST rows not owed | 5 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 5 |
| SHOULD / MAY rows | 124 |
| SHOULD proven | 16 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 108 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

## Tree

Roots first: an RFC sits one level above the highest level of the RFCs it builds on, and the RFCs of one citation cycle share a level. Each RFC carries the same two bars as the set above, from its own data file; an RFC that is not walked shows "not walked" and no bar. The last column names the summary whose Use cases row declares the RFC: a normative citation never adds one.

### Level 0

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 3954](../../rfc3954/) | Cisco Systems NetFlow Services Export Version 9 | Ze proven 4, Ze correctness gap 6 | Ze MUST proven 4, Not owed 4, SHOULD not decided 12, Ze correctness gap 6 | ddos-flowspec: required by rfc/short/rfc3954.md |

### Level 1

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 5635](../../rfc5635/) | Remote Triggered Black Hole Filtering with Unicast Reverse Path Forwarding (uRPF) | no rows | SHOULD not decided 1 | ddos-flowspec: required by rfc/short/rfc5635.md |

### Level 3

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 4271](../../rfc4271/) | A Border Gateway Protocol 4 (BGP-4) | Ze proven 83, Ze correctness gap 50 | Ze MUST proven 83, SHOULD proven 3, SHOULD not decided 31, Ze correctness gap 50 | ddos-flowspec: required by rfc/short/rfc4271.md |
| [SFLOW-V5](../../sflow-v5/) | sFlow: A Method for Monitoring Traffic in Switched and Routed Networks | Ze proven 22, OS testing gap 1, Ze correctness gap 9 | Ze MUST proven 22, SHOULD proven 3, SHOULD not decided 8, OS testing gap 1, Ze correctness gap 9 | ddos-flowspec: required by rfc/short/sflow-v5.md |

### Level 5

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 4360](../../rfc4360/) | BGP Extended Communities Attribute | Ze proven 2, Ze correctness gap 3 | Ze MUST proven 2, SHOULD not decided 4, Ze correctness gap 3 | ddos-flowspec: required by rfc/short/rfc4360.md |
| [RFC 4760](../../rfc4760/) | Multiprotocol Extensions for BGP-4 | Ze proven 6 | Ze MUST proven 6, SHOULD proven 1, SHOULD not decided 10 | ddos-flowspec: required by rfc/short/rfc4760.md |
| [RFC 7011](../../rfc7011/) (cycle with rfc7012) | Specification of the IP Flow Information Export (IPFIX) Protocol for the Exchange of Flow Information | Ze proven 12, Ze correctness gap 38 | Ze MUST proven 12, SHOULD not decided 7, Ze correctness gap 38 | ddos-flowspec: required by rfc/short/rfc7011.md |
| [RFC 7012](../../rfc7012/) (cycle with rfc7011) | Information Model for IP Flow Information Export (IPFIX) | Ze proven 1, Ze correctness gap 5 | Ze MUST proven 1, Ze correctness gap 5 | ddos-flowspec: required by rfc/short/rfc7012.md |

### Level 7

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 7606](../../rfc7606/) (cycle with rfc4364, rfc4659, rfc4798, rfc6368, rfc8277, rfc8950) | Revised Error Handling for BGP UPDATE Messages | Ze proven 28, Ze correctness gap 25 | Ze MUST proven 28, SHOULD not decided 4, Ze correctness gap 25 | ddos-flowspec: required by rfc/short/rfc7606.md |

### Level 8

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 7999](../../rfc7999/) | BLACKHOLE Community | Ze proven 4 | Ze MUST proven 4, SHOULD proven 3, SHOULD not decided 9 | ddos-flowspec: required by rfc/short/rfc7999.md |
| [RFC 8955](../../rfc8955/) | Dissemination of Flow Specification Rules | Ze proven 13, Ze correctness gap 15 | Ze MUST proven 13, SHOULD proven 6, Not owed 1, SHOULD not decided 14, Ze correctness gap 15 | ddos-flowspec: required by rfc/short/rfc8955.md |

### Level 9

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 8956](../../rfc8956/) | Dissemination of Flow Specification Rules for IPv6 | Ze proven 7, Ze correctness gap 2 | Ze MUST proven 7, SHOULD not decided 5, Ze correctness gap 2 | ddos-flowspec: required by rfc/short/rfc8956.md |
| [RFC 9117](../../rfc9117/) | Revised Validation Procedure for BGP Flow Specifications | Ze proven 2 | Ze MUST proven 2, SHOULD not decided 3 | ddos-flowspec: required by rfc/short/rfc9117.md |

## Optional (not counted)

RFCs this use case lists as optional, each declared by the Optional use cases row of its own summary. They count in none of the figures above until the owner brings one into scope by moving it to the Use cases row. Their own bars and figures follow.

- **MUST bar**: no rows
- **Overview bar**: no rows

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Not walked | 0 |
| Mandatory register | complete |
| MUST rows | 0 |
| MUST rows kept (the MUST bar) | 0 |
| Ze proven | 0 |
| OS verified | 0 |
| OS testing gap | 0 |
| Ze correctness gap | 0 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 0 |
| SHOULD proven | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 0 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 3882](../../rfc3882/) | Configuring BGP to Block Denial-of-Service Attacks | no rows | no rows | ddos-flowspec: optional in rfc/short/rfc3882.md |
