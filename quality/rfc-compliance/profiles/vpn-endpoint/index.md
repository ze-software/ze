# IPsec VPN endpoint

Release profile. The RFCs this use case requires, each declared by the Use cases row of its own summary. RFCs it lists as optional follow in their own table and never enter its figures. Before release, every mandatory requirement of every required RFC must have a test that has been shown to fail when the behavior it checks is deliberately broken.

[All profiles](../) · [All RFCs](../../)

Figures as of the last build, read from [data/rfc/profiles/vpn-endpoint.json](../../../../data/rfc/profiles/vpn-endpoint.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze verified 258, Tests awaiting verification 156, OS testing gap 56, Not built 77
- **Overview bar**: Ze MUST verified 258, SHOULD verified 7, Tests awaiting verification 159, OS testing gap 57, Not assessed 156, Not built 79

| Figure | Rows |
|---|---|
| RFCs | 12 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 547 |
| MUST rows kept (the MUST bar) | 547 |
| Ze verified | 258 |
| OS verified | 0 |
| Tests awaiting verification | 156 |
| OS testing gap | 56 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 77 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 169 |
| SHOULD verified | 7 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 1 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 3 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 156 |
| SHOULD not built | 2 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

## Tree

The RFCs are listed with the standards they build on first. An RFC appears one level after the highest level of its dependencies; RFCs that refer back to each other share a level. Each RFC has the same two coverage bars as the group above. An RFC whose requirement list has not been reviewed shows "requirements not reviewed" instead of a bar. The last column names the summary whose Use cases row declares the RFC: a normative citation never adds one.

### Level 1

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 4754](../../rfc4754/) | IKE and IKEv2 Authentication Using the Elliptic Curve Digital Signature Algorithm (ECDSA) | Ze verified 1 | Ze MUST verified 1 | vpn-endpoint: required by rfc/short/rfc4754.md |

### Level 3

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 5280](../../rfc5280/) | Internet X.509 Public Key Infrastructure Certificate and Certificate Revocation List (CRL) Profile | Ze verified 32, OS testing gap 50, Not built 20 | Ze MUST verified 32, OS testing gap 50, Not assessed 108, Not built 20 | vpn-endpoint: required by rfc/short/rfc5280.md |

### Level 4

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 3948](../../rfc3948/) (cycle with rfc3602, rfc4106, rfc4301, rfc4302, rfc4303, rfc4309, rfc5282, rfc7296, rfc8247) | UDP Encapsulation of IPsec ESP Packets | Ze verified 6, Tests awaiting verification 4, Not built 1 | Ze MUST verified 6, SHOULD verified 2, Tests awaiting verification 4, Not assessed 3, Not built 1 | vpn-endpoint: required by rfc/short/rfc3948.md |
| [RFC 4106](../../rfc4106/) (cycle with rfc3602, rfc3948, rfc4301, rfc4302, rfc4303, rfc4309, rfc5282, rfc7296, rfc8247) | The Use of Galois/Counter Mode (GCM) in IPsec Encapsulating Security Payload (ESP) | Ze verified 5, OS testing gap 3 | Ze MUST verified 5, OS testing gap 3, Not assessed 5 | vpn-endpoint: required by rfc/short/rfc4106.md |
| [RFC 4301](../../rfc4301/) (cycle with rfc3602, rfc3948, rfc4106, rfc4302, rfc4303, rfc4309, rfc5282, rfc7296, rfc8247) | Security Architecture for the Internet Protocol | Ze verified 60, Tests awaiting verification 7, OS testing gap 3, Not built 31 | Ze MUST verified 60, Tests awaiting verification 7, OS testing gap 4, Not assessed 1, Not built 32 | vpn-endpoint: required by rfc/short/rfc4301.md |
| [RFC 4303](../../rfc4303/) (cycle with rfc3602, rfc3948, rfc4106, rfc4301, rfc4302, rfc4309, rfc5282, rfc7296, rfc8247) | IP Encapsulating Security Payload (ESP) | Ze verified 7, Tests awaiting verification 2, Not built 14 | Ze MUST verified 7, Tests awaiting verification 3, Not assessed 2, Not built 15 | vpn-endpoint: required by rfc/short/rfc4303.md |
| [RFC 5282](../../rfc5282/) (cycle with rfc3602, rfc3948, rfc4106, rfc4301, rfc4302, rfc4303, rfc4309, rfc7296, rfc8247) | Using Authenticated Encryption Algorithms with the Encrypted Payload of the Internet Key Exchange version 2 (IKEv2) Protocol | Ze verified 19 | Ze MUST verified 19, Not assessed 11 | vpn-endpoint: required by rfc/short/rfc5282.md |
| [RFC 7296](../../rfc7296/) (cycle with rfc3602, rfc3948, rfc4106, rfc4301, rfc4302, rfc4303, rfc4309, rfc5282, rfc8247) | Internet Key Exchange Protocol Version 2 (IKEv2) | Ze verified 91, Tests awaiting verification 143 | Ze MUST verified 91, SHOULD verified 4, Tests awaiting verification 145, Not assessed 3 | vpn-endpoint: required by rfc/short/rfc7296.md |
| [RFC 8247](../../rfc8247/) (cycle with rfc3602, rfc3948, rfc4106, rfc4301, rfc4302, rfc4303, rfc4309, rfc5282, rfc7296) | Algorithm Implementation Requirements and Usage Guidance for the Internet Key Exchange Protocol Version 2 (IKEv2) | Ze verified 17, Not built 3 | Ze MUST verified 17, SHOULD verified 1, Not assessed 5, Not built 3 | vpn-endpoint: required by rfc/short/rfc8247.md |

### Level 5

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 7427](../../rfc7427/) | Signature Authentication in the Internet Key Exchange Version 2 (IKEv2) | Ze verified 4 | Ze MUST verified 4, Not assessed 3 | vpn-endpoint: required by rfc/short/rfc7427.md |
| [RFC 8221](../../rfc8221/) | Cryptographic Algorithm Implementation Requirements and Usage Guidance for Encapsulating Security Payload (ESP) and Authentication Header (AH) | Ze verified 16 | Ze MUST verified 16, Not assessed 3 | vpn-endpoint: required by rfc/short/rfc8221.md |

### Level 10

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 6071](../../rfc6071/) | IP Security (IPsec) and Internet Key Exchange (IKE) Document Roadmap | Not built 8 | Not assessed 12, Not built 8 | vpn-endpoint: required by rfc/short/rfc6071.md |

## Optional (not counted)

RFCs this use case lists as optional, each declared by the Optional use cases row of its own summary. They count in none of the figures above until the owner brings one into scope by moving it to the Use cases row. Their own bars and figures follow.

- **MUST bar**: Ze verified 1, OS testing gap 1, Not assessed 25, Not built 26
- **Overview bar**: Ze MUST verified 1, OS testing gap 1, Not assessed 47, Not built 26

| Figure | Rows |
|---|---|
| RFCs | 4 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 53 |
| MUST rows kept (the MUST bar) | 53 |
| Ze verified | 1 |
| OS verified | 0 |
| Tests awaiting verification | 0 |
| OS testing gap | 1 |
| Exclusion unchecked | 0 |
| Not assessed | 25 |
| Not built | 26 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 22 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 22 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

| RFC | Title | MUST bar | Overview bar | Why it is here |
|---|---|---|---|---|
| [RFC 7383](../../rfc7383/) | Internet Key Exchange Protocol Version 2 (IKEv2) Message Fragmentation | Not assessed 25 | Not assessed 40 | vpn-endpoint: optional in rfc/short/rfc7383.md |
| [RFC 7634](../../rfc7634/) | ChaCha20, Poly1305, and Their Use in the Internet Key Exchange Protocol (IKE) and IPsec | Ze verified 1, OS testing gap 1 | Ze MUST verified 1, OS testing gap 1, Not assessed 3 | vpn-endpoint: optional in rfc/short/rfc7634.md |
| [RFC 8031](../../rfc8031/) | Curve25519 and Curve448 for the Internet Key Exchange Protocol Version 2 (IKEv2) Key Agreement | Not built 3 | Not built 3 | vpn-endpoint: optional in rfc/short/rfc8031.md |
| [RFC 9370](../../rfc9370/) | Multiple Key Exchanges in the Internet Key Exchange Protocol Version 2 (IKEv2) | Not built 23 | Not assessed 4, Not built 23 | vpn-endpoint: optional in rfc/short/rfc9370.md |
