# DDoS detection and mitigation by sending FlowSpec upstream

Use-case profile. The RFCs this use case requires, each declared by the Use cases row of its own summary. RFCs it lists as optional follow in their own table and never enter its figures.

[All profiles](../) · [All RFCs](../../)

Use case: [FlowSpec Injection](../../../../use-cases/flowspec-injection/)

- **Mandatory**: 52.2% (179 of 343 (52.2%)). MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that a recorded test break proves.
- **Optional**: 12.1% (15 of 124 (12.1%)). MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows, counted the same way.
- **Not walked**: 0 (of 13 RFCs). RFCs with no valid extraction sign-off: their rows count in neither figure.
- **Mandatory register**: not complete (every row proven, none unwalked). A profile is complete only when it requires at least one RFC and none of them is unwalked.

## Tree

Roots first: an RFC sits one level above the highest level of the RFCs it builds on, and the RFCs of one citation cycle share a level. Each bar is the share of that figure's rows a recorded break proves; an RFC that is not walked shows an empty bar and no figure. The last column names the summary whose Use cases row declares the RFC: a normative citation never adds one.

### Level 0

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 3954](../../rfc3954/) | Cisco Systems NetFlow Services Export Version 9 | 4 of 14 (28.6%) | 0 of 12 (0.0%) | ddos-flowspec: required by rfc/short/rfc3954.md |

### Level 1

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 5635](../../rfc5635/) | Remote Triggered Black Hole Filtering with Unicast Reverse Path Forwarding (uRPF) | no rows | 0 of 1 (0.0%) | ddos-flowspec: required by rfc/short/rfc5635.md |

### Level 3

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 4271](../../rfc4271/) | A Border Gateway Protocol 4 (BGP-4) | 80 of 133 (60.2%) | 3 of 34 (8.8%) | ddos-flowspec: required by rfc/short/rfc4271.md |
| [SFLOW-V5](../../sflow-v5/) | sFlow: A Method for Monitoring Traffic in Switched and Routed Networks | 22 of 32 (68.8%) | 3 of 11 (27.3%) | ddos-flowspec: required by rfc/short/sflow-v5.md |

### Level 5

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 4360](../../rfc4360/) | BGP Extended Communities Attribute | 2 of 5 (40.0%) | 0 of 4 (0.0%) | ddos-flowspec: required by rfc/short/rfc4360.md |
| [RFC 4760](../../rfc4760/) | Multiprotocol Extensions for BGP-4 | 5 of 6 (83.3%) | 1 of 11 (9.1%) | ddos-flowspec: required by rfc/short/rfc4760.md |
| [RFC 7011](../../rfc7011/) (cycle with rfc7012) | Specification of the IP Flow Information Export (IPFIX) Protocol for the Exchange of Flow Information | 12 of 50 (24.0%) | 0 of 7 (0.0%) | ddos-flowspec: required by rfc/short/rfc7011.md |
| [RFC 7012](../../rfc7012/) (cycle with rfc7011) | Information Model for IP Flow Information Export (IPFIX) | 1 of 6 (16.7%) | no rows | ddos-flowspec: required by rfc/short/rfc7012.md |

### Level 7

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 7606](../../rfc7606/) (cycle with rfc4364, rfc4659, rfc4798, rfc6368, rfc8277, rfc8950) | Revised Error Handling for BGP UPDATE Messages | 28 of 53 (52.8%) | 0 of 4 (0.0%) | ddos-flowspec: required by rfc/short/rfc7606.md |

### Level 8

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 7999](../../rfc7999/) | BLACKHOLE Community | 3 of 4 (75.0%) | 2 of 12 (16.7%) | ddos-flowspec: required by rfc/short/rfc7999.md |
| [RFC 8955](../../rfc8955/) | Dissemination of Flow Specification Rules | 13 of 29 (44.8%) | 6 of 20 (30.0%) | ddos-flowspec: required by rfc/short/rfc8955.md |

### Level 9

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 8956](../../rfc8956/) | Dissemination of Flow Specification Rules for IPv6 | 7 of 9 (77.8%) | 0 of 5 (0.0%) | ddos-flowspec: required by rfc/short/rfc8956.md |
| [RFC 9117](../../rfc9117/) | Revised Validation Procedure for BGP Flow Specifications | 2 of 2 (100.0%) | 0 of 3 (0.0%) | ddos-flowspec: required by rfc/short/rfc9117.md |

## Optional (not counted)

RFCs this use case lists as optional, each declared by the Optional use cases row of its own summary. They count in none of the figures above until the owner brings one into scope by moving it to the Use cases row. Their own figures: mandatory no rows, optional no rows; 1 RFCs, 0 not walked.

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 3882](../../rfc3882/) | Configuring BGP to Block Denial-of-Service Attacks | no rows | no rows | ddos-flowspec: optional in rfc/short/rfc3882.md |
