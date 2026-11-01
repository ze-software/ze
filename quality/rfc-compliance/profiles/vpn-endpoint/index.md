# IPsec VPN endpoint

Release profile. The RFCs this use case requires, each declared by the Use cases row of its own summary. RFCs it lists as optional follow in their own table and never enter its figures. Before release, every mandatory requirement of every required RFC must be proven by a recorded break of its test.

[All profiles](../) · [All RFCs](../../)

Figures as of the last build, read from [data/rfc/profiles/vpn-endpoint.json](../../../../data/rfc/profiles/vpn-endpoint.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze proven 237, OS testing gap 55, Ze correctness gap 255
- **Overview bar**: Ze MUST proven 237, SHOULD proven 7, SHOULD not decided 161, OS testing gap 55, Ze correctness gap 255

| Figure | Rows |
|---|---|
| RFCs | 12 |
| Not walked | 0 |
| Mandatory register | not complete |
| MUST rows | 547 |
| MUST rows kept (the MUST bar) | 547 |
| Ze proven | 237 |
| OS verified | 0 |
| OS testing gap | 55 |
| Ze correctness gap | 255 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 168 |
| SHOULD proven | 7 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 161 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

## Tree

Roots first: an RFC sits one level above the highest level of the RFCs it builds on, and the RFCs of one citation cycle share a level. Each RFC carries the same two bars as the set above, from its own data file; an RFC that is not walked shows "not walked" and no bar. The last column names the summary whose Use cases row declares the RFC: a normative citation never adds one.

### Level 1

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 4754](../../rfc4754/) | IKE and IKEv2 Authentication Using the Elliptic Curve Digital Signature Algorithm (ECDSA) | Ze proven 1 | Ze MUST proven 1 | vpn-endpoint: required by rfc/short/rfc4754.md |

### Level 3

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 5280](../../rfc5280/) | Internet X.509 Public Key Infrastructure Certificate and Certificate Revocation List (CRL) Profile | Ze proven 26, OS testing gap 50, Ze correctness gap 26 | Ze MUST proven 26, SHOULD not decided 108, OS testing gap 50, Ze correctness gap 26 | vpn-endpoint: required by rfc/short/rfc5280.md |

### Level 4

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 3948](../../rfc3948/) (cycle with rfc3602, rfc4106, rfc4301, rfc4302, rfc4303, rfc4309, rfc5282, rfc7296, rfc8247) | UDP Encapsulation of IPsec ESP Packets | Ze proven 6, Ze correctness gap 5 | Ze MUST proven 6, SHOULD proven 2, SHOULD not decided 3, Ze correctness gap 5 | vpn-endpoint: required by rfc/short/rfc3948.md |
| [RFC 4106](../../rfc4106/) (cycle with rfc3602, rfc3948, rfc4301, rfc4302, rfc4303, rfc4309, rfc5282, rfc7296, rfc8247) | The Use of Galois/Counter Mode (GCM) in IPsec Encapsulating Security Payload (ESP) | Ze proven 5, OS testing gap 3 | Ze MUST proven 5, SHOULD not decided 5, OS testing gap 3 | vpn-endpoint: required by rfc/short/rfc4106.md |
| [RFC 4301](../../rfc4301/) (cycle with rfc3602, rfc3948, rfc4106, rfc4302, rfc4303, rfc4309, rfc5282, rfc7296, rfc8247) | Security Architecture for the Internet Protocol | Ze proven 52, OS testing gap 2, Ze correctness gap 47 | Ze MUST proven 52, SHOULD proven 1, SHOULD not decided 2, OS testing gap 2, Ze correctness gap 47 | vpn-endpoint: required by rfc/short/rfc4301.md |
| [RFC 4303](../../rfc4303/) (cycle with rfc3602, rfc3948, rfc4106, rfc4301, rfc4302, rfc4309, rfc5282, rfc7296, rfc8247) | IP Encapsulating Security Payload (ESP) | Ze proven 7, Ze correctness gap 16 | Ze MUST proven 7, SHOULD not decided 4, Ze correctness gap 16 | vpn-endpoint: required by rfc/short/rfc4303.md |
| [RFC 5282](../../rfc5282/) (cycle with rfc3602, rfc3948, rfc4106, rfc4301, rfc4302, rfc4303, rfc4309, rfc7296, rfc8247) | Using Authenticated Encryption Algorithms with the Encrypted Payload of the Internet Key Exchange version 2 (IKEv2) Protocol | Ze proven 19 | Ze MUST proven 19, SHOULD not decided 11 | vpn-endpoint: required by rfc/short/rfc5282.md |
| [RFC 7296](../../rfc7296/) (cycle with rfc3602, rfc3948, rfc4106, rfc4301, rfc4302, rfc4303, rfc4309, rfc5282, rfc8247) | Internet Key Exchange Protocol Version 2 (IKEv2) | Ze proven 84, Ze correctness gap 150 | Ze MUST proven 84, SHOULD proven 4, SHOULD not decided 5, Ze correctness gap 150 | vpn-endpoint: required by rfc/short/rfc7296.md |
| [RFC 8247](../../rfc8247/) (cycle with rfc3602, rfc3948, rfc4106, rfc4301, rfc4302, rfc4303, rfc4309, rfc5282, rfc7296) | Algorithm Implementation Requirements and Usage Guidance for the Internet Key Exchange Protocol Version 2 (IKEv2) | Ze proven 17, Ze correctness gap 3 | Ze MUST proven 17, SHOULD not decided 5, Ze correctness gap 3 | vpn-endpoint: required by rfc/short/rfc8247.md |

### Level 5

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 7427](../../rfc7427/) | Signature Authentication in the Internet Key Exchange Version 2 (IKEv2) | Ze proven 4 | Ze MUST proven 4, SHOULD not decided 3 | vpn-endpoint: required by rfc/short/rfc7427.md |
| [RFC 8221](../../rfc8221/) | Cryptographic Algorithm Implementation Requirements and Usage Guidance for Encapsulating Security Payload (ESP) and Authentication Header (AH) | Ze proven 16 | Ze MUST proven 16, SHOULD not decided 3 | vpn-endpoint: required by rfc/short/rfc8221.md |

### Level 10

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 6071](../../rfc6071/) | IP Security (IPsec) and Internet Key Exchange (IKE) Document Roadmap | Ze correctness gap 8 | SHOULD not decided 12, Ze correctness gap 8 | vpn-endpoint: required by rfc/short/rfc6071.md |

## Optional (not counted)

RFCs this use case lists as optional, each declared by the Optional use cases row of its own summary. They count in none of the figures above until the owner brings one into scope by moving it to the Use cases row. Their own bars and figures follow.

- **MUST bar**: Ze proven 1, OS testing gap 1, Ze correctness gap 51
- **Overview bar**: Ze MUST proven 1, SHOULD not decided 22, OS testing gap 1, Ze correctness gap 51

| Figure | Rows |
|---|---|
| RFCs | 4 |
| Not walked | 0 |
| Mandatory register | not complete |
| MUST rows | 53 |
| MUST rows kept (the MUST bar) | 53 |
| Ze proven | 1 |
| OS verified | 0 |
| OS testing gap | 1 |
| Ze correctness gap | 51 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 22 |
| SHOULD proven | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 22 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 7383](../../rfc7383/) | Internet Key Exchange Protocol Version 2 (IKEv2) Message Fragmentation | Ze correctness gap 25 | SHOULD not decided 15, Ze correctness gap 25 | vpn-endpoint: optional in rfc/short/rfc7383.md |
| [RFC 7634](../../rfc7634/) | ChaCha20, Poly1305, and Their Use in the Internet Key Exchange Protocol (IKE) and IPsec | Ze proven 1, OS testing gap 1 | Ze MUST proven 1, SHOULD not decided 3, OS testing gap 1 | vpn-endpoint: optional in rfc/short/rfc7634.md |
| [RFC 8031](../../rfc8031/) | Curve25519 and Curve448 for the Internet Key Exchange Protocol Version 2 (IKEv2) Key Agreement | Ze correctness gap 3 | Ze correctness gap 3 | vpn-endpoint: optional in rfc/short/rfc8031.md |
| [RFC 9370](../../rfc9370/) | Multiple Key Exchanges in the Internet Key Exchange Protocol Version 2 (IKEv2) | Ze correctness gap 23 | SHOULD not decided 4, Ze correctness gap 23 | vpn-endpoint: optional in rfc/short/rfc9370.md |
