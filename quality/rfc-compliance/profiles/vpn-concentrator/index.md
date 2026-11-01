# IPsec VPN concentrator for remote-access clients

Use-case profile. The RFCs this use case requires, each declared by the Use cases row of its own summary. RFCs it lists as optional follow in their own table and never enter its figures.

[All profiles](../) · [All RFCs](../../)

Figures as of the last build, read from [data/rfc/profiles/vpn-concentrator.json](../../../../data/rfc/profiles/vpn-concentrator.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze proven 357, OS testing gap 57, Ze correctness gap 306
- **Overview bar**: Ze MUST proven 357, SHOULD proven 13, SHOULD not decided 210, OS testing gap 57, Ze correctness gap 306

| Figure | Rows |
|---|---|
| RFCs | 17 |
| Not walked | 0 |
| Mandatory register | not complete |
| MUST rows | 720 |
| MUST rows kept (the MUST bar) | 720 |
| Ze proven | 357 |
| OS verified | 0 |
| OS testing gap | 57 |
| Ze correctness gap | 306 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 223 |
| SHOULD proven | 13 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 210 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

## Tree

Roots first: an RFC sits one level above the highest level of the RFCs it builds on, and the RFCs of one citation cycle share a level. Each RFC carries the same two bars as the set above, from its own data file; an RFC that is not walked shows "not walked" and no bar. The last column names the summary whose Use cases row declares the RFC: a normative citation never adds one.

### Level 0

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 4739](../../rfc4739/) | Multiple Authentication Exchanges in the Internet Key Exchange (IKEv2) Protocol | Ze proven 2 | Ze MUST proven 2 | vpn-concentrator: required by rfc/short/rfc4739.md |

### Level 1

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 4754](../../rfc4754/) | IKE and IKEv2 Authentication Using the Elliptic Curve Digital Signature Algorithm (ECDSA) | Ze proven 1 | Ze MUST proven 1 | vpn-concentrator: required by rfc/short/rfc4754.md |

### Level 2

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 3748](../../rfc3748/) | Extensible Authentication Protocol (EAP) | Ze proven 42, Ze correctness gap 17 | Ze MUST proven 42, SHOULD not decided 4, Ze correctness gap 17 | vpn-concentrator: required by rfc/short/rfc3748.md |

### Level 3

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 5280](../../rfc5280/) | Internet X.509 Public Key Infrastructure Certificate and Certificate Revocation List (CRL) Profile | Ze proven 26, OS testing gap 50, Ze correctness gap 26 | Ze MUST proven 26, SHOULD not decided 108, OS testing gap 50, Ze correctness gap 26 | vpn-concentrator: required by rfc/short/rfc5280.md |

### Level 4

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 3948](../../rfc3948/) (cycle with rfc3602, rfc4106, rfc4301, rfc4302, rfc4303, rfc4309, rfc5282, rfc7296, rfc8247) | UDP Encapsulation of IPsec ESP Packets | Ze proven 6, Ze correctness gap 5 | Ze MUST proven 6, SHOULD proven 2, SHOULD not decided 3, Ze correctness gap 5 | vpn-concentrator: required by rfc/short/rfc3948.md |
| [RFC 4106](../../rfc4106/) (cycle with rfc3602, rfc3948, rfc4301, rfc4302, rfc4303, rfc4309, rfc5282, rfc7296, rfc8247) | The Use of Galois/Counter Mode (GCM) in IPsec Encapsulating Security Payload (ESP) | Ze proven 5, OS testing gap 3 | Ze MUST proven 5, SHOULD not decided 5, OS testing gap 3 | vpn-concentrator: required by rfc/short/rfc4106.md |
| [RFC 4301](../../rfc4301/) (cycle with rfc3602, rfc3948, rfc4106, rfc4302, rfc4303, rfc4309, rfc5282, rfc7296, rfc8247) | Security Architecture for the Internet Protocol | Ze proven 52, OS testing gap 2, Ze correctness gap 47 | Ze MUST proven 52, SHOULD proven 1, SHOULD not decided 2, OS testing gap 2, Ze correctness gap 47 | vpn-concentrator: required by rfc/short/rfc4301.md |
| [RFC 4303](../../rfc4303/) (cycle with rfc3602, rfc3948, rfc4106, rfc4301, rfc4302, rfc4309, rfc5282, rfc7296, rfc8247) | IP Encapsulating Security Payload (ESP) | Ze proven 7, Ze correctness gap 16 | Ze MUST proven 7, SHOULD not decided 4, Ze correctness gap 16 | vpn-concentrator: required by rfc/short/rfc4303.md |
| [RFC 5282](../../rfc5282/) (cycle with rfc3602, rfc3948, rfc4106, rfc4301, rfc4302, rfc4303, rfc4309, rfc7296, rfc8247) | Using Authenticated Encryption Algorithms with the Encrypted Payload of the Internet Key Exchange version 2 (IKEv2) Protocol | Ze proven 19 | Ze MUST proven 19, SHOULD not decided 11 | vpn-concentrator: required by rfc/short/rfc5282.md |
| [RFC 7296](../../rfc7296/) (cycle with rfc3602, rfc3948, rfc4106, rfc4301, rfc4302, rfc4303, rfc4309, rfc5282, rfc8247) | Internet Key Exchange Protocol Version 2 (IKEv2) | Ze proven 84, Ze correctness gap 150 | Ze MUST proven 84, SHOULD proven 4, SHOULD not decided 5, Ze correctness gap 150 | vpn-concentrator: required by rfc/short/rfc7296.md |
| [RFC 8247](../../rfc8247/) (cycle with rfc3602, rfc3948, rfc4106, rfc4301, rfc4302, rfc4303, rfc4309, rfc5282, rfc7296) | Algorithm Implementation Requirements and Usage Guidance for the Internet Key Exchange Protocol Version 2 (IKEv2) | Ze proven 17, Ze correctness gap 3 | Ze MUST proven 17, SHOULD not decided 5, Ze correctness gap 3 | vpn-concentrator: required by rfc/short/rfc8247.md |

### Level 5

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 4555](../../rfc4555/) | IKEv2 Mobility and Multihoming Protocol (MOBIKE) | Ze proven 3, Ze correctness gap 11 | Ze MUST proven 3, SHOULD proven 2, SHOULD not decided 10, Ze correctness gap 11 | vpn-concentrator: required by rfc/short/rfc4555.md |
| [RFC 5216](../../rfc5216/) | The EAP-TLS Authentication Protocol | Ze proven 30, OS testing gap 2, Ze correctness gap 15 | Ze MUST proven 30, SHOULD proven 2, SHOULD not decided 5, OS testing gap 2, Ze correctness gap 15 | vpn-concentrator: required by rfc/short/rfc5216.md |
| [RFC 5998](../../rfc5998/) | An Extension for EAP-Only Authentication in IKEv2 | Ze proven 1, Ze correctness gap 5 | Ze MUST proven 1, SHOULD not decided 1, Ze correctness gap 5 | vpn-concentrator: required by rfc/short/rfc5998.md |
| [RFC 7427](../../rfc7427/) | Signature Authentication in the Internet Key Exchange Version 2 (IKEv2) | Ze proven 4 | Ze MUST proven 4, SHOULD not decided 3 | vpn-concentrator: required by rfc/short/rfc7427.md |
| [RFC 8221](../../rfc8221/) | Cryptographic Algorithm Implementation Requirements and Usage Guidance for Encapsulating Security Payload (ESP) and Authentication Header (AH) | Ze proven 16 | Ze MUST proven 16, SHOULD not decided 3 | vpn-concentrator: required by rfc/short/rfc8221.md |

### Level 6

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 9190](../../rfc9190/) | EAP-TLS 1.3: Using the Extensible Authentication Protocol with TLS 1.3 | Ze proven 42, Ze correctness gap 11 | Ze MUST proven 42, SHOULD proven 2, SHOULD not decided 41, Ze correctness gap 11 | vpn-concentrator: required by rfc/short/rfc9190.md |

## Optional (not counted)

RFCs this use case lists as optional, each declared by the Optional use cases row of its own summary. They count in none of the figures above until the owner brings one into scope by moving it to the Use cases row. Their own bars and figures follow.

- **MUST bar**: Ze proven 1, OS testing gap 1, Ze correctness gap 107
- **Overview bar**: Ze MUST proven 1, SHOULD not decided 75, OS testing gap 1, Ze correctness gap 107

| Figure | Rows |
|---|---|
| RFCs | 6 |
| Not walked | 0 |
| Mandatory register | not complete |
| MUST rows | 109 |
| MUST rows kept (the MUST bar) | 109 |
| Ze proven | 1 |
| OS verified | 0 |
| OS testing gap | 1 |
| Ze correctness gap | 107 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 75 |
| SHOULD proven | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 75 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 5685](../../rfc5685/) | Redirect Mechanism for the Internet Key Exchange Protocol Version 2 (IKEv2) | Ze correctness gap 26 | SHOULD not decided 9, Ze correctness gap 26 | vpn-concentrator: optional in rfc/short/rfc5685.md |
| [RFC 7383](../../rfc7383/) | Internet Key Exchange Protocol Version 2 (IKEv2) Message Fragmentation | Ze correctness gap 25 | SHOULD not decided 15, Ze correctness gap 25 | vpn-concentrator: optional in rfc/short/rfc7383.md |
| [RFC 7634](../../rfc7634/) | ChaCha20, Poly1305, and Their Use in the Internet Key Exchange Protocol (IKE) and IPsec | Ze proven 1, OS testing gap 1 | Ze MUST proven 1, SHOULD not decided 3, OS testing gap 1 | vpn-concentrator: optional in rfc/short/rfc7634.md |
| [RFC 8031](../../rfc8031/) | Curve25519 and Curve448 for the Internet Key Exchange Protocol Version 2 (IKEv2) Key Agreement | Ze correctness gap 3 | Ze correctness gap 3 | vpn-concentrator: optional in rfc/short/rfc8031.md |
| [RFC 9329](../../rfc9329/) | TCP Encapsulation of Internet Key Exchange Protocol (IKE) and IPsec Packets | Ze correctness gap 30 | SHOULD not decided 44, Ze correctness gap 30 | vpn-concentrator: optional in rfc/short/rfc9329.md |
| [RFC 9370](../../rfc9370/) | Multiple Key Exchanges in the Internet Key Exchange Protocol Version 2 (IKEv2) | Ze correctness gap 23 | SHOULD not decided 4, Ze correctness gap 23 | vpn-concentrator: optional in rfc/short/rfc9370.md |
