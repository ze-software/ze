# IPsec VPN endpoint

Release profile. The RFCs this use case requires, each declared by the Use cases row of its own summary. RFCs it lists as optional follow in their own table and never enter its figures. Before release, every mandatory requirement of every required RFC must be proven by a recorded break of its test.

[All profiles](../) · [All RFCs](../../)

- **Mandatory**: 41.8% (228 of 545 (41.8%)). MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that a recorded test break proves.
- **Optional**: 4.2% (7 of 168 (4.2%)). MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows, counted the same way.
- **Mandatory ceiling**: 83.9% (457 of 545 (83.9%); excluded: Optional feature not offered (classification can be revisited): 1 of 545 (0.2%); Met by a lower layer: 60 of 545 (11.0%); Not applicable (classification can be revisited): 27 of 545 (5.0%)). Attainable mandatory rows under current classifications, over the unchanged total.
- **Optional ceiling**: 100.0% (168 of 168 (100.0%)). Attainable optional rows under current classifications, over the unchanged total.
- **Not walked**: 0 (of 11 RFCs). RFCs with no valid extraction sign-off: their rows count in neither figure.
- **Mandatory register**: not complete (every row proven, none unwalked). A profile is complete only when it requires at least one RFC and none of them is unwalked.

Mandatory ceiling: 457 of 545 (83.9%); excluded: Optional feature not offered (classification can be revisited): 1 of 545 (0.2%); Met by a lower layer: 60 of 545 (11.0%); Not applicable (classification can be revisited): 27 of 545 (5.0%). Optional ceiling: 168 of 168 (100.0%). Lowest mandatory ceiling: rfc6071 0 of 8 (0.0%). Lowest optional ceiling: rfc3948 5 of 5 (100.0%). Ceilings describe current classifications, not an immutable Linux maximum. Not-applicable and feature-declined classifications can be revisited. Every gap remains work Ze owes.

## Tree

Roots first: an RFC sits one level above the highest level of the RFCs it builds on, and the RFCs of one citation cycle share a level. Each bar is the share of that figure's rows a recorded break proves; the ceiling below it shows attainable rows under current classifications. An RFC that is not walked shows an empty bar and no figure or ceiling. The last column names the summary whose Use cases row declares the RFC: a normative citation never adds one.

### Level 3

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 5280](../../rfc5280/) | Internet X.509 Public Key Infrastructure Certificate and Certificate Revocation List (CRL) Profile | 26 of 102 (25.5%); ceiling 49 of 102 (48.0%); excluded: Met by a lower layer: 53 of 102 (52.0%) | 0 of 108 (0.0%); ceiling 108 of 108 (100.0%) | vpn-endpoint: required by rfc/short/rfc5280.md |

### Level 4

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 3948](../../rfc3948/) (cycle with rfc3602, rfc4106, rfc4301, rfc4302, rfc4303, rfc4309, rfc5282, rfc7296, rfc8247) | UDP Encapsulation of IPsec ESP Packets | 5 of 11 (45.5%); ceiling 10 of 11 (90.9%); excluded: Not applicable (classification can be revisited): 1 of 11 (9.1%) | 2 of 5 (40.0%); ceiling 5 of 5 (100.0%) | vpn-endpoint: required by rfc/short/rfc3948.md |
| [RFC 4106](../../rfc4106/) (cycle with rfc3602, rfc3948, rfc4301, rfc4302, rfc4303, rfc4309, rfc5282, rfc7296, rfc8247) | The Use of Galois/Counter Mode (GCM) in IPsec Encapsulating Security Payload (ESP) | 5 of 8 (62.5%); ceiling 5 of 8 (62.5%); excluded: Met by a lower layer: 3 of 8 (37.5%) | 0 of 5 (0.0%); ceiling 5 of 5 (100.0%) | vpn-endpoint: required by rfc/short/rfc4106.md |
| [RFC 4301](../../rfc4301/) (cycle with rfc3602, rfc3948, rfc4106, rfc4302, rfc4303, rfc4309, rfc5282, rfc7296, rfc8247) | Security Architecture for the Internet Protocol | 50 of 100 (50.0%); ceiling 92 of 100 (92.0%); excluded: Optional feature not offered (classification can be revisited): 1 of 100 (1.0%); Met by a lower layer: 4 of 100 (4.0%); Not applicable (classification can be revisited): 3 of 100 (3.0%) | 1 of 3 (33.3%); ceiling 3 of 3 (100.0%) | vpn-endpoint: required by rfc/short/rfc4301.md |
| [RFC 4303](../../rfc4303/) (cycle with rfc3602, rfc3948, rfc4106, rfc4301, rfc4302, rfc4309, rfc5282, rfc7296, rfc8247) | IP Encapsulating Security Payload (ESP) | 7 of 23 (30.4%); ceiling 9 of 23 (39.1%); excluded: Not applicable (classification can be revisited): 14 of 23 (60.9%) | 0 of 4 (0.0%); ceiling 4 of 4 (100.0%) | vpn-endpoint: required by rfc/short/rfc4303.md |
| [RFC 5282](../../rfc5282/) (cycle with rfc3602, rfc3948, rfc4106, rfc4301, rfc4302, rfc4303, rfc4309, rfc7296, rfc8247) | Using Authenticated Encryption Algorithms with the Encrypted Payload of the Internet Key Exchange version 2 (IKEv2) Protocol | 19 of 19 (100.0%); ceiling 19 of 19 (100.0%) | 0 of 11 (0.0%); ceiling 11 of 11 (100.0%) | vpn-endpoint: required by rfc/short/rfc5282.md |
| [RFC 7296](../../rfc7296/) (cycle with rfc3602, rfc3948, rfc4106, rfc4301, rfc4302, rfc4303, rfc4309, rfc5282, rfc8247) | Internet Key Exchange Protocol Version 2 (IKEv2) | 82 of 234 (35.0%); ceiling 234 of 234 (100.0%) | 4 of 9 (44.4%); ceiling 9 of 9 (100.0%) | vpn-endpoint: required by rfc/short/rfc7296.md |
| [RFC 8247](../../rfc8247/) (cycle with rfc3602, rfc3948, rfc4106, rfc4301, rfc4302, rfc4303, rfc4309, rfc5282, rfc7296) | Algorithm Implementation Requirements and Usage Guidance for the Internet Key Exchange Protocol Version 2 (IKEv2) | 16 of 20 (80.0%); ceiling 20 of 20 (100.0%) | 0 of 5 (0.0%); ceiling 5 of 5 (100.0%) | vpn-endpoint: required by rfc/short/rfc8247.md |

### Level 5

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 7427](../../rfc7427/) | Signature Authentication in the Internet Key Exchange Version 2 (IKEv2) | 2 of 4 (50.0%); ceiling 3 of 4 (75.0%); excluded: Not applicable (classification can be revisited): 1 of 4 (25.0%) | 0 of 3 (0.0%); ceiling 3 of 3 (100.0%) | vpn-endpoint: required by rfc/short/rfc7427.md |
| [RFC 8221](../../rfc8221/) | Cryptographic Algorithm Implementation Requirements and Usage Guidance for Encapsulating Security Payload (ESP) and Authentication Header (AH) | 16 of 16 (100.0%); ceiling 16 of 16 (100.0%) | 0 of 3 (0.0%); ceiling 3 of 3 (100.0%) | vpn-endpoint: required by rfc/short/rfc8221.md |

### Level 10

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 6071](../../rfc6071/) | IP Security (IPsec) and Internet Key Exchange (IKE) Document Roadmap | 0 of 8 (0.0%); ceiling 0 of 8 (0.0%); excluded: Not applicable (classification can be revisited): 8 of 8 (100.0%) | 0 of 12 (0.0%); ceiling 12 of 12 (100.0%) | vpn-endpoint: required by rfc/short/rfc6071.md |

## Optional (not counted)

RFCs this use case lists as optional, each declared by the Optional use cases row of its own summary. They count in none of the figures above until the owner brings one into scope by moving it to the Use cases row. Their own figures: mandatory 1 of 54 (1.9%), optional 0 of 22 (0.0%); ceiling mandatory 53 of 54 (98.1%), optional 22 of 22 (100.0%); mandatory excluded: lower-layer 1; 5 RFCs, 0 not walked.

Mandatory ceiling: 53 of 54 (98.1%); excluded: Met by a lower layer: 1 of 54 (1.9%). Optional ceiling: 22 of 22 (100.0%). Lowest mandatory ceiling: rfc7634 1 of 2 (50.0%). Lowest optional ceiling: rfc7383 15 of 15 (100.0%). Ceilings describe current classifications, not an immutable Linux maximum. Not-applicable and feature-declined classifications can be revisited. Every gap remains work Ze owes.

| RFC | Title | Mandatory | Optional | Why it is here |
|---|---|---|---|---|
| [RFC 4754](../../rfc4754/) | IKE and IKEv2 Authentication Using the Elliptic Curve Digital Signature Algorithm (ECDSA) | 0 of 1 (0.0%); ceiling 1 of 1 (100.0%) | no rows; ceiling no rows | vpn-endpoint: optional in rfc/short/rfc4754.md |
| [RFC 7383](../../rfc7383/) | Internet Key Exchange Protocol Version 2 (IKEv2) Message Fragmentation | 0 of 25 (0.0%); ceiling 25 of 25 (100.0%) | 0 of 15 (0.0%); ceiling 15 of 15 (100.0%) | vpn-endpoint: optional in rfc/short/rfc7383.md |
| [RFC 7634](../../rfc7634/) | ChaCha20, Poly1305, and Their Use in the Internet Key Exchange Protocol (IKE) and IPsec | 1 of 2 (50.0%); ceiling 1 of 2 (50.0%); excluded: Met by a lower layer: 1 of 2 (50.0%) | 0 of 3 (0.0%); ceiling 3 of 3 (100.0%) | vpn-endpoint: optional in rfc/short/rfc7634.md |
| [RFC 8031](../../rfc8031/) | Curve25519 and Curve448 for the Internet Key Exchange Protocol Version 2 (IKEv2) Key Agreement | 0 of 3 (0.0%); ceiling 3 of 3 (100.0%) | no rows; ceiling no rows | vpn-endpoint: optional in rfc/short/rfc8031.md |
| [RFC 9370](../../rfc9370/) | Multiple Key Exchanges in the Internet Key Exchange Protocol Version 2 (IKEv2) | 0 of 23 (0.0%); ceiling 23 of 23 (100.0%) | 0 of 4 (0.0%); ceiling 4 of 4 (100.0%) | vpn-endpoint: optional in rfc/short/rfc9370.md |
