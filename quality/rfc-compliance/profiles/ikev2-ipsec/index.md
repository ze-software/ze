# IKEv2 and IPsec

Technology aggregate. IPsec, IKEv2, and kernel security associations. The RFCs whose summary names the ikev2-ipsec aggregate, with their mandatory and optional coverage.

[All profiles](../) · [All RFCs](../../)

- **Mandatory**: 34.1% (240 of 703 (34.1%)). MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that a recorded test break proves.
- **Optional**: 3.4% (9 of 265 (3.4%)). MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows, counted the same way.
- **Mandatory ceiling**: 84.4% (593 of 703 (84.4%); excluded: Optional feature not offered (classification can be revisited): 2 of 703 (0.3%); Met by a lower layer: 81 of 703 (11.5%); Not applicable (classification can be revisited): 27 of 703 (3.8%)). Attainable mandatory rows under current classifications, over the unchanged total.
- **Optional ceiling**: 100.0% (265 of 265 (100.0%)). Attainable optional rows under current classifications, over the unchanged total.
- **Not walked**: 0 (of 21 RFCs). RFCs with no valid extraction sign-off: their rows count in neither figure.
- **Mandatory register**: not complete (every row proven, none unwalked). A profile is complete only when it requires at least one RFC and none of them is unwalked.

Mandatory ceiling: 593 of 703 (84.4%); excluded: Optional feature not offered (classification can be revisited): 2 of 703 (0.3%); Met by a lower layer: 81 of 703 (11.5%); Not applicable (classification can be revisited): 27 of 703 (3.8%). Optional ceiling: 265 of 265 (100.0%). Lowest mandatory ceiling: rfc6071 0 of 8 (0.0%). Lowest optional ceiling: rfc3948 5 of 5 (100.0%). Ceilings describe current classifications, not an immutable Linux maximum. Not-applicable and feature-declined classifications can be revisited. Every gap remains work Ze owes.

## Tree

Roots first: an RFC sits one level above the highest level of the RFCs it builds on, and the RFCs of one citation cycle share a level. Each bar is the share of that figure's rows a recorded break proves; the ceiling below it shows attainable rows under current classifications. An RFC that is not walked shows an empty bar and no figure or ceiling.

### Level 0

| RFC | Title | Mandatory | Optional |
|---|---|---|---|
| [RFC 4739](../../rfc4739/) | Multiple Authentication Exchanges in the Internet Key Exchange (IKEv2) Protocol | 0 of 2 (0.0%); ceiling 2 of 2 (100.0%) | no rows; ceiling no rows |
| [RFC 5685](../../rfc5685/) | Redirect Mechanism for the Internet Key Exchange Protocol Version 2 (IKEv2) | 0 of 26 (0.0%); ceiling 26 of 26 (100.0%) | 0 of 9 (0.0%); ceiling 9 of 9 (100.0%) |

### Level 1

| RFC | Title | Mandatory | Optional |
|---|---|---|---|
| [RFC 4754](../../rfc4754/) | IKE and IKEv2 Authentication Using the Elliptic Curve Digital Signature Algorithm (ECDSA) | 0 of 1 (0.0%); ceiling 1 of 1 (100.0%) | no rows; ceiling no rows |

### Level 3

| RFC | Title | Mandatory | Optional |
|---|---|---|---|
| [RFC 5280](../../rfc5280/) | Internet X.509 Public Key Infrastructure Certificate and Certificate Revocation List (CRL) Profile | 26 of 102 (25.5%); ceiling 49 of 102 (48.0%); excluded: Met by a lower layer: 53 of 102 (52.0%) | 0 of 108 (0.0%); ceiling 108 of 108 (100.0%) |

### Level 4

| RFC | Title | Mandatory | Optional |
|---|---|---|---|
| [RFC 3948](../../rfc3948/) (cycle with rfc3602, rfc4106, rfc4301, rfc4302, rfc4303, rfc4309, rfc5282, rfc7296, rfc8247) | UDP Encapsulation of IPsec ESP Packets | 5 of 11 (45.5%); ceiling 10 of 11 (90.9%); excluded: Not applicable (classification can be revisited): 1 of 11 (9.1%) | 2 of 5 (40.0%); ceiling 5 of 5 (100.0%) |
| [RFC 4106](../../rfc4106/) (cycle with rfc3602, rfc3948, rfc4301, rfc4302, rfc4303, rfc4309, rfc5282, rfc7296, rfc8247) | The Use of Galois/Counter Mode (GCM) in IPsec Encapsulating Security Payload (ESP) | 5 of 8 (62.5%); ceiling 5 of 8 (62.5%); excluded: Met by a lower layer: 3 of 8 (37.5%) | 0 of 5 (0.0%); ceiling 5 of 5 (100.0%) |
| [RFC 4301](../../rfc4301/) (cycle with rfc3602, rfc3948, rfc4106, rfc4302, rfc4303, rfc4309, rfc5282, rfc7296, rfc8247) | Security Architecture for the Internet Protocol | 50 of 100 (50.0%); ceiling 92 of 100 (92.0%); excluded: Optional feature not offered (classification can be revisited): 1 of 100 (1.0%); Met by a lower layer: 4 of 100 (4.0%); Not applicable (classification can be revisited): 3 of 100 (3.0%) | 1 of 3 (33.3%); ceiling 3 of 3 (100.0%) |
| [RFC 4302](../../rfc4302/) (cycle with rfc3602, rfc3948, rfc4106, rfc4301, rfc4303, rfc4309, rfc5282, rfc7296, rfc8247) | IP Authentication Header | 8 of 32 (25.0%); ceiling 11 of 32 (34.4%); excluded: Optional feature not offered (classification can be revisited): 1 of 32 (3.1%); Met by a lower layer: 20 of 32 (62.5%) | 0 of 10 (0.0%); ceiling 10 of 10 (100.0%) |
| [RFC 4303](../../rfc4303/) (cycle with rfc3602, rfc3948, rfc4106, rfc4301, rfc4302, rfc4309, rfc5282, rfc7296, rfc8247) | IP Encapsulating Security Payload (ESP) | 7 of 23 (30.4%); ceiling 9 of 23 (39.1%); excluded: Not applicable (classification can be revisited): 14 of 23 (60.9%) | 0 of 4 (0.0%); ceiling 4 of 4 (100.0%) |
| [RFC 5282](../../rfc5282/) (cycle with rfc3602, rfc3948, rfc4106, rfc4301, rfc4302, rfc4303, rfc4309, rfc7296, rfc8247) | Using Authenticated Encryption Algorithms with the Encrypted Payload of the Internet Key Exchange version 2 (IKEv2) Protocol | 19 of 19 (100.0%); ceiling 19 of 19 (100.0%) | 0 of 11 (0.0%); ceiling 11 of 11 (100.0%) |
| [RFC 7296](../../rfc7296/) (cycle with rfc3602, rfc3948, rfc4106, rfc4301, rfc4302, rfc4303, rfc4309, rfc5282, rfc8247) | Internet Key Exchange Protocol Version 2 (IKEv2) | 82 of 234 (35.0%); ceiling 234 of 234 (100.0%) | 4 of 9 (44.4%); ceiling 9 of 9 (100.0%) |
| [RFC 8247](../../rfc8247/) (cycle with rfc3602, rfc3948, rfc4106, rfc4301, rfc4302, rfc4303, rfc4309, rfc5282, rfc7296) | Algorithm Implementation Requirements and Usage Guidance for the Internet Key Exchange Protocol Version 2 (IKEv2) | 16 of 20 (80.0%); ceiling 20 of 20 (100.0%) | 0 of 5 (0.0%); ceiling 5 of 5 (100.0%) |

### Level 5

| RFC | Title | Mandatory | Optional |
|---|---|---|---|
| [RFC 4555](../../rfc4555/) | IKEv2 Mobility and Multihoming Protocol (MOBIKE) | 3 of 14 (21.4%); ceiling 14 of 14 (100.0%) | 2 of 12 (16.7%); ceiling 12 of 12 (100.0%) |
| [RFC 7383](../../rfc7383/) | Internet Key Exchange Protocol Version 2 (IKEv2) Message Fragmentation | 0 of 25 (0.0%); ceiling 25 of 25 (100.0%) | 0 of 15 (0.0%); ceiling 15 of 15 (100.0%) |
| [RFC 7427](../../rfc7427/) | Signature Authentication in the Internet Key Exchange Version 2 (IKEv2) | 2 of 4 (50.0%); ceiling 3 of 4 (75.0%); excluded: Not applicable (classification can be revisited): 1 of 4 (25.0%) | 0 of 3 (0.0%); ceiling 3 of 3 (100.0%) |
| [RFC 7634](../../rfc7634/) | ChaCha20, Poly1305, and Their Use in the Internet Key Exchange Protocol (IKE) and IPsec | 1 of 2 (50.0%); ceiling 1 of 2 (50.0%); excluded: Met by a lower layer: 1 of 2 (50.0%) | 0 of 3 (0.0%); ceiling 3 of 3 (100.0%) |
| [RFC 8031](../../rfc8031/) | Curve25519 and Curve448 for the Internet Key Exchange Protocol Version 2 (IKEv2) Key Agreement | 0 of 3 (0.0%); ceiling 3 of 3 (100.0%) | no rows; ceiling no rows |
| [RFC 8221](../../rfc8221/) | Cryptographic Algorithm Implementation Requirements and Usage Guidance for Encapsulating Security Payload (ESP) and Authentication Header (AH) | 16 of 16 (100.0%); ceiling 16 of 16 (100.0%) | 0 of 3 (0.0%); ceiling 3 of 3 (100.0%) |
| [RFC 9329](../../rfc9329/) | TCP Encapsulation of Internet Key Exchange Protocol (IKE) and IPsec Packets | 0 of 30 (0.0%); ceiling 30 of 30 (100.0%) | 0 of 44 (0.0%); ceiling 44 of 44 (100.0%) |
| [RFC 9370](../../rfc9370/) | Multiple Key Exchanges in the Internet Key Exchange Protocol Version 2 (IKEv2) | 0 of 23 (0.0%); ceiling 23 of 23 (100.0%) | 0 of 4 (0.0%); ceiling 4 of 4 (100.0%) |

### Level 10

| RFC | Title | Mandatory | Optional |
|---|---|---|---|
| [RFC 6071](../../rfc6071/) | IP Security (IPsec) and Internet Key Exchange (IKE) Document Roadmap | 0 of 8 (0.0%); ceiling 0 of 8 (0.0%); excluded: Not applicable (classification can be revisited): 8 of 8 (100.0%) | 0 of 12 (0.0%); ceiling 12 of 12 (100.0%) |
