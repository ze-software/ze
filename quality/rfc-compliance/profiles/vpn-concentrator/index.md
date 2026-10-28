# IPsec VPN concentrator for remote-access clients

Use-case profile. The RFCs this use case requires, each declared by the Use cases row of its own summary. RFCs it lists as optional follow in their own table and never enter its figures.

[All profiles](../) · [All RFCs](../../)

- **Mandatory**: 48.3% (346 of 716 (48.3%)). MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that a recorded test break proves.
- **Optional**: 5.8% (13 of 223 (5.8%)). MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows, counted the same way.
- **Mandatory ceiling**: 87.0% (623 of 716 (87.0%); excluded: Optional feature not offered (classification can be revisited): 6 of 716 (0.8%); Met by a lower layer: 62 of 716 (8.7%); Not applicable (classification can be revisited): 25 of 716 (3.5%)). Attainable mandatory rows under current classifications, over the unchanged total.
- **Optional ceiling**: 100.0% (223 of 223 (100.0%)). Attainable optional rows under current classifications, over the unchanged total.
- **Not walked**: 0 (of 15 RFCs). RFCs with no valid extraction sign-off: their rows count in neither figure.
- **Mandatory register**: not complete (every row proven, none unwalked). A profile is complete only when it requires at least one RFC and none of them is unwalked.

Mandatory ceiling: 623 of 716 (87.0%); excluded: Optional feature not offered (classification can be revisited): 6 of 716 (0.8%); Met by a lower layer: 62 of 716 (8.7%); Not applicable (classification can be revisited): 25 of 716 (3.5%). Optional ceiling: 223 of 223 (100.0%). Lowest mandatory ceiling: rfc4303 9 of 23 (39.1%). Lowest optional ceiling: rfc3748 4 of 4 (100.0%). Ceilings describe current classifications, not an immutable Linux maximum. Not-applicable and feature-declined classifications can be revisited. Every gap remains work Ze owes.

## Tree

Roots first: an RFC sits one level above the highest level of the RFCs it builds on, and the RFCs of one citation cycle share a level. Each bar is the share of that figure's rows a recorded break proves; the ceiling below it shows attainable rows under current classifications. An RFC that is not walked shows an empty bar and no figure or ceiling. The last column names the summary whose Use cases row declares the RFC: a normative citation never adds one.

### Level 2

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 3748](../../rfc3748/) | Extensible Authentication Protocol (EAP) | 42 of 59 (71.2%); ceiling 53 of 59 (89.8%); excluded: Not applicable (classification can be revisited): 6 of 59 (10.2%) | 0 of 4 (0.0%); ceiling 4 of 4 (100.0%) | vpn-concentrator: required by rfc/short/rfc3748.md |

### Level 3

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 5280](../../rfc5280/) | Internet X.509 Public Key Infrastructure Certificate and Certificate Revocation List (CRL) Profile | 26 of 102 (25.5%); ceiling 49 of 102 (48.0%); excluded: Met by a lower layer: 53 of 102 (52.0%) | 0 of 108 (0.0%); ceiling 108 of 108 (100.0%) | vpn-concentrator: required by rfc/short/rfc5280.md |

### Level 4

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 3948](../../rfc3948/) (cycle with rfc3602, rfc4106, rfc4301, rfc4302, rfc4303, rfc4309, rfc5282, rfc7296, rfc8247) | UDP Encapsulation of IPsec ESP Packets | 5 of 11 (45.5%); ceiling 10 of 11 (90.9%); excluded: Not applicable (classification can be revisited): 1 of 11 (9.1%) | 2 of 5 (40.0%); ceiling 5 of 5 (100.0%) | vpn-concentrator: required by rfc/short/rfc3948.md |
| [RFC 4106](../../rfc4106/) (cycle with rfc3602, rfc3948, rfc4301, rfc4302, rfc4303, rfc4309, rfc5282, rfc7296, rfc8247) | The Use of Galois/Counter Mode (GCM) in IPsec Encapsulating Security Payload (ESP) | 5 of 8 (62.5%); ceiling 5 of 8 (62.5%); excluded: Met by a lower layer: 3 of 8 (37.5%) | 0 of 5 (0.0%); ceiling 5 of 5 (100.0%) | vpn-concentrator: required by rfc/short/rfc4106.md |
| [RFC 4301](../../rfc4301/) (cycle with rfc3602, rfc3948, rfc4106, rfc4302, rfc4303, rfc4309, rfc5282, rfc7296, rfc8247) | Security Architecture for the Internet Protocol | 50 of 100 (50.0%); ceiling 92 of 100 (92.0%); excluded: Optional feature not offered (classification can be revisited): 1 of 100 (1.0%); Met by a lower layer: 4 of 100 (4.0%); Not applicable (classification can be revisited): 3 of 100 (3.0%) | 1 of 3 (33.3%); ceiling 3 of 3 (100.0%) | vpn-concentrator: required by rfc/short/rfc4301.md |
| [RFC 4303](../../rfc4303/) (cycle with rfc3602, rfc3948, rfc4106, rfc4301, rfc4302, rfc4309, rfc5282, rfc7296, rfc8247) | IP Encapsulating Security Payload (ESP) | 7 of 23 (30.4%); ceiling 9 of 23 (39.1%); excluded: Not applicable (classification can be revisited): 14 of 23 (60.9%) | 0 of 4 (0.0%); ceiling 4 of 4 (100.0%) | vpn-concentrator: required by rfc/short/rfc4303.md |
| [RFC 5282](../../rfc5282/) (cycle with rfc3602, rfc3948, rfc4106, rfc4301, rfc4302, rfc4303, rfc4309, rfc7296, rfc8247) | Using Authenticated Encryption Algorithms with the Encrypted Payload of the Internet Key Exchange version 2 (IKEv2) Protocol | 19 of 19 (100.0%); ceiling 19 of 19 (100.0%) | 0 of 11 (0.0%); ceiling 11 of 11 (100.0%) | vpn-concentrator: required by rfc/short/rfc5282.md |
| [RFC 7296](../../rfc7296/) (cycle with rfc3602, rfc3948, rfc4106, rfc4301, rfc4302, rfc4303, rfc4309, rfc5282, rfc8247) | Internet Key Exchange Protocol Version 2 (IKEv2) | 82 of 234 (35.0%); ceiling 234 of 234 (100.0%) | 4 of 9 (44.4%); ceiling 9 of 9 (100.0%) | vpn-concentrator: required by rfc/short/rfc7296.md |
| [RFC 8247](../../rfc8247/) (cycle with rfc3602, rfc3948, rfc4106, rfc4301, rfc4302, rfc4303, rfc4309, rfc5282, rfc7296) | Algorithm Implementation Requirements and Usage Guidance for the Internet Key Exchange Protocol Version 2 (IKEv2) | 16 of 20 (80.0%); ceiling 20 of 20 (100.0%) | 0 of 5 (0.0%); ceiling 5 of 5 (100.0%) | vpn-concentrator: required by rfc/short/rfc8247.md |

### Level 5

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 4555](../../rfc4555/) | IKEv2 Mobility and Multihoming Protocol (MOBIKE) | 3 of 14 (21.4%); ceiling 14 of 14 (100.0%) | 2 of 12 (16.7%); ceiling 12 of 12 (100.0%) | vpn-concentrator: required by rfc/short/rfc4555.md |
| [RFC 5216](../../rfc5216/) | The EAP-TLS Authentication Protocol | 30 of 47 (63.8%); ceiling 45 of 47 (95.7%); excluded: Met by a lower layer: 2 of 47 (4.3%) | 2 of 7 (28.6%); ceiling 7 of 7 (100.0%) | vpn-concentrator: required by rfc/short/rfc5216.md |
| [RFC 5998](../../rfc5998/) | An Extension for EAP-Only Authentication in IKEv2 | 1 of 6 (16.7%); ceiling 6 of 6 (100.0%) | 0 of 1 (0.0%); ceiling 1 of 1 (100.0%) | vpn-concentrator: required by rfc/short/rfc5998.md |
| [RFC 7427](../../rfc7427/) | Signature Authentication in the Internet Key Exchange Version 2 (IKEv2) | 2 of 4 (50.0%); ceiling 3 of 4 (75.0%); excluded: Not applicable (classification can be revisited): 1 of 4 (25.0%) | 0 of 3 (0.0%); ceiling 3 of 3 (100.0%) | vpn-concentrator: required by rfc/short/rfc7427.md |
| [RFC 8221](../../rfc8221/) | Cryptographic Algorithm Implementation Requirements and Usage Guidance for Encapsulating Security Payload (ESP) and Authentication Header (AH) | 16 of 16 (100.0%); ceiling 16 of 16 (100.0%) | 0 of 3 (0.0%); ceiling 3 of 3 (100.0%) | vpn-concentrator: required by rfc/short/rfc8221.md |

### Level 6

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 9190](../../rfc9190/) | EAP-TLS 1.3: Using the Extensible Authentication Protocol with TLS 1.3 | 42 of 53 (79.2%); ceiling 48 of 53 (90.6%); excluded: Optional feature not offered (classification can be revisited): 5 of 53 (9.4%) | 2 of 43 (4.7%); ceiling 43 of 43 (100.0%) | vpn-concentrator: required by rfc/short/rfc9190.md |

## Optional (not counted)

RFCs this use case lists as optional, each declared by the Optional use cases row of its own summary. They count in none of the figures above until the owner brings one into scope by moving it to the Use cases row. Their own figures: mandatory 1 of 112 (0.9%), optional 0 of 75 (0.0%); ceiling mandatory 111 of 112 (99.1%), optional 75 of 75 (100.0%); mandatory excluded: lower-layer 1; 8 RFCs, 0 not walked.

Mandatory ceiling: 111 of 112 (99.1%); excluded: Met by a lower layer: 1 of 112 (0.9%). Optional ceiling: 75 of 75 (100.0%). Lowest mandatory ceiling: rfc7634 1 of 2 (50.0%). Lowest optional ceiling: rfc5685 9 of 9 (100.0%). Ceilings describe current classifications, not an immutable Linux maximum. Not-applicable and feature-declined classifications can be revisited. Every gap remains work Ze owes.

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 4739](../../rfc4739/) | Multiple Authentication Exchanges in the Internet Key Exchange (IKEv2) Protocol | 0 of 2 (0.0%); ceiling 2 of 2 (100.0%) | no rows; ceiling no rows | vpn-concentrator: optional in rfc/short/rfc4739.md |
| [RFC 5685](../../rfc5685/) | Redirect Mechanism for the Internet Key Exchange Protocol Version 2 (IKEv2) | 0 of 26 (0.0%); ceiling 26 of 26 (100.0%) | 0 of 9 (0.0%); ceiling 9 of 9 (100.0%) | vpn-concentrator: optional in rfc/short/rfc5685.md |
| [RFC 4754](../../rfc4754/) | IKE and IKEv2 Authentication Using the Elliptic Curve Digital Signature Algorithm (ECDSA) | 0 of 1 (0.0%); ceiling 1 of 1 (100.0%) | no rows; ceiling no rows | vpn-concentrator: optional in rfc/short/rfc4754.md |
| [RFC 7383](../../rfc7383/) | Internet Key Exchange Protocol Version 2 (IKEv2) Message Fragmentation | 0 of 25 (0.0%); ceiling 25 of 25 (100.0%) | 0 of 15 (0.0%); ceiling 15 of 15 (100.0%) | vpn-concentrator: optional in rfc/short/rfc7383.md |
| [RFC 7634](../../rfc7634/) | ChaCha20, Poly1305, and Their Use in the Internet Key Exchange Protocol (IKE) and IPsec | 1 of 2 (50.0%); ceiling 1 of 2 (50.0%); excluded: Met by a lower layer: 1 of 2 (50.0%) | 0 of 3 (0.0%); ceiling 3 of 3 (100.0%) | vpn-concentrator: optional in rfc/short/rfc7634.md |
| [RFC 8031](../../rfc8031/) | Curve25519 and Curve448 for the Internet Key Exchange Protocol Version 2 (IKEv2) Key Agreement | 0 of 3 (0.0%); ceiling 3 of 3 (100.0%) | no rows; ceiling no rows | vpn-concentrator: optional in rfc/short/rfc8031.md |
| [RFC 9329](../../rfc9329/) | TCP Encapsulation of Internet Key Exchange Protocol (IKE) and IPsec Packets | 0 of 30 (0.0%); ceiling 30 of 30 (100.0%) | 0 of 44 (0.0%); ceiling 44 of 44 (100.0%) | vpn-concentrator: optional in rfc/short/rfc9329.md |
| [RFC 9370](../../rfc9370/) | Multiple Key Exchanges in the Internet Key Exchange Protocol Version 2 (IKEv2) | 0 of 23 (0.0%); ceiling 23 of 23 (100.0%) | 0 of 4 (0.0%); ceiling 4 of 4 (100.0%) | vpn-concentrator: optional in rfc/short/rfc9370.md |
