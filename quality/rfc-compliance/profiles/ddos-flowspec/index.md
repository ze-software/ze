# DDoS detection and mitigation by sending FlowSpec upstream

Use-case profile. The RFCs this use case requires, each declared by the Use cases row of its own summary. RFCs it lists as optional follow in their own table and never enter its figures.

[All profiles](../) · [All RFCs](../../)

Use case: [FlowSpec Injection](../../../../use-cases/flowspec-injection/)

Figures as of the last build, read from [data/rfc/profiles/ddos-flowspec.json](../../../../data/rfc/profiles/ddos-flowspec.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze proven 199, Proof pending 67, OS testing gap 1, Not built 73
- **Overview bar**: Ze MUST proven 199, SHOULD proven 16, Not owed 5, Proof pending 68, OS testing gap 1, Not assessed 107, Not built 73

| Figure | Rows |
|---|---|
| RFCs | 14 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 345 |
| MUST rows kept (the MUST bar) | 340 |
| Ze proven | 199 |
| OS verified | 0 |
| Proof pending | 67 |
| OS testing gap | 1 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 73 |
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
| SHOULD proof pending | 1 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 107 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze proven means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Proof pending counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

## Tree

The RFCs are listed with the standards they build on first. An RFC appears one level after the highest level of its dependencies; RFCs that refer back to each other share a level. Each RFC has the same two coverage bars as the group above. An RFC whose requirement list has not been reviewed shows "requirements not reviewed" instead of a bar. The last column names the summary whose Use cases row declares the RFC: a normative citation never adds one.

### Level 0

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 3954](../../rfc3954/) | Cisco Systems NetFlow Services Export Version 9 | Ze proven 4, Not built 6 | Ze MUST proven 4, Not owed 4, Not assessed 12, Not built 6 | ddos-flowspec: required by rfc/short/rfc3954.md |

### Level 1

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 5635](../../rfc5635/) | Remote Triggered Black Hole Filtering with Unicast Reverse Path Forwarding (uRPF) | no rows | Not assessed 1 | ddos-flowspec: required by rfc/short/rfc5635.md |

### Level 3

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 4271](../../rfc4271/) | A Border Gateway Protocol 4 (BGP-4) | Ze proven 91, Proof pending 27, Not built 15 | Ze MUST proven 91, SHOULD proven 3, Proof pending 28, Not assessed 30, Not built 15 | ddos-flowspec: required by rfc/short/rfc4271.md |
| [SFLOW-V5](../../sflow-v5/) | sFlow: A Method for Monitoring Traffic in Switched and Routed Networks | Ze proven 21, OS testing gap 1, Not built 9 | Ze MUST proven 21, SHOULD proven 3, OS testing gap 1, Not assessed 8, Not built 9 | ddos-flowspec: required by rfc/short/sflow-v5.md |

### Level 4

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 6608](../../rfc6608/) | Subcodes for BGP Finite State Machine Error | Ze proven 3 | Ze MUST proven 3 | ddos-flowspec: required by rfc/short/rfc6608.md |

### Level 5

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 4360](../../rfc4360/) | BGP Extended Communities Attribute | Ze proven 2, Not built 3 | Ze MUST proven 2, Not assessed 4, Not built 3 | ddos-flowspec: required by rfc/short/rfc4360.md |
| [RFC 4760](../../rfc4760/) | Multiprotocol Extensions for BGP-4 | Ze proven 5, Proof pending 1 | Ze MUST proven 5, SHOULD proven 1, Proof pending 1, Not assessed 10 | ddos-flowspec: required by rfc/short/rfc4760.md |
| [RFC 7011](../../rfc7011/) (cycle with rfc7012) | Specification of the IP Flow Information Export (IPFIX) Protocol for the Exchange of Flow Information | Ze proven 12, Proof pending 6, Not built 32 | Ze MUST proven 12, Proof pending 6, Not assessed 7, Not built 32 | ddos-flowspec: required by rfc/short/rfc7011.md |
| [RFC 7012](../../rfc7012/) (cycle with rfc7011) | Information Model for IP Flow Information Export (IPFIX) | Ze proven 1, Not built 5 | Ze MUST proven 1, Not built 5 | ddos-flowspec: required by rfc/short/rfc7012.md |

### Level 7

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 7606](../../rfc7606/) (cycle with rfc4364, rfc4659, rfc4798, rfc6368, rfc8277, rfc8950) | Revised Error Handling for BGP UPDATE Messages | Ze proven 33, Proof pending 19, Not built 1 | Ze MUST proven 33, Proof pending 19, Not assessed 4, Not built 1 | ddos-flowspec: required by rfc/short/rfc7606.md |

### Level 8

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 7999](../../rfc7999/) | BLACKHOLE Community | Ze proven 4 | Ze MUST proven 4, SHOULD proven 3, Not assessed 9 | ddos-flowspec: required by rfc/short/rfc7999.md |
| [RFC 8955](../../rfc8955/) | Dissemination of Flow Specification Rules | Ze proven 14, Proof pending 12, Not built 2 | Ze MUST proven 14, SHOULD proven 6, Not owed 1, Proof pending 12, Not assessed 14, Not built 2 | ddos-flowspec: required by rfc/short/rfc8955.md |

### Level 9

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 8956](../../rfc8956/) | Dissemination of Flow Specification Rules for IPv6 | Ze proven 7, Proof pending 2 | Ze MUST proven 7, Proof pending 2, Not assessed 5 | ddos-flowspec: required by rfc/short/rfc8956.md |
| [RFC 9117](../../rfc9117/) | Revised Validation Procedure for BGP Flow Specifications | Ze proven 2 | Ze MUST proven 2, Not assessed 3 | ddos-flowspec: required by rfc/short/rfc9117.md |

## Optional (not counted)

RFCs this use case lists as optional, each declared by the Optional use cases row of its own summary. They count in none of the figures above until the owner brings one into scope by moving it to the Use cases row. Their own bars and figures follow.

- **MUST bar**: no rows
- **Overview bar**: no rows

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | complete |
| MUST rows | 0 |
| MUST rows kept (the MUST bar) | 0 |
| Ze proven | 0 |
| OS verified | 0 |
| Proof pending | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 0 |
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
| SHOULD proof pending | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 0 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze proven means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Proof pending counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 3882](../../rfc3882/) | Configuring BGP to Block Denial-of-Service Attacks | no rows | no rows | ddos-flowspec: optional in rfc/short/rfc3882.md |
