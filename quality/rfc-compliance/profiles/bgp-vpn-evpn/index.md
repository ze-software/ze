# BGP VPN and EVPN

Technology profile. BGP/MPLS VPNs, labeled routes, VPLS, and EVPN. The RFCs grouped under bgp-vpn-evpn, showing mandatory requirement coverage and overall coverage for the group and each RFC.

[All profiles](../) · [All RFCs](../../)

Figures as of the last build, read from [data/rfc/profiles/bgp-vpn-evpn.json](../../../../data/rfc/profiles/bgp-vpn-evpn.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze proven 31, Proof pending 6, Not built 165
- **Overview bar**: Ze MUST proven 31, SHOULD proven 1, Proof pending 6, Not assessed 53, Not built 165

| Figure | Rows |
|---|---|
| RFCs | 9 |
| Requirements not reviewed | 2 |
| Mandatory requirements complete | not complete |
| MUST rows | 202 |
| MUST rows kept (the MUST bar) | 202 |
| Ze proven | 31 |
| OS verified | 0 |
| Proof pending | 6 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 165 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 54 |
| SHOULD proven | 1 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD proof pending | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 53 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze proven means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Proof pending counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

## Tree

The RFCs are listed with the standards they build on first. An RFC appears one level after the highest level of its dependencies; RFCs that refer back to each other share a level. Each RFC has the same two coverage bars as the group above. An RFC whose requirement list has not been reviewed shows "requirements not reviewed" instead of a bar.

### Level 7

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 4364](../../rfc4364/) (cycle with rfc4659, rfc4798, rfc6368, rfc7606, rfc8277, rfc8950) | BGP/MPLS IP Virtual Private Networks (VPNs) | Not built 8 | Not assessed 5, Not built 8 |
| [RFC 4659](../../rfc4659/) (cycle with rfc4364, rfc4798, rfc6368, rfc7606, rfc8277, rfc8950) | BGP-MPLS IP Virtual Private Network (VPN) Extension for IPv6 VPN | Ze proven 4, Proof pending 1, Not built 13 | Ze MUST proven 4, Proof pending 1, Not assessed 6, Not built 13 |
| [RFC 8277](../../rfc8277/) (cycle with rfc4364, rfc4659, rfc4798, rfc6368, rfc7606, rfc8950) | Using BGP to Bind MPLS Labels to Address Prefixes | Ze proven 11, Not built 24 | Ze MUST proven 11, SHOULD proven 1, Not assessed 6, Not built 24 |

### Level 8

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 4684](../../rfc4684/) | Constrained Route Distribution for Border Gateway Protocol/MultiProtocol Label Switching (BGP/MPLS) Internet Protocol (IP) Virtual Private Networks (VPNs) | Ze proven 2, Not built 4 | Ze MUST proven 2, Not assessed 4, Not built 4 |
| [RFC 4761](../../rfc4761/) | Virtual Private LAN Service (VPLS) Using BGP for Auto-Discovery and Signaling | Not built 18 | Not assessed 9, Not built 18 |
| [RFC 6514](../../rfc6514/) | BGP Encodings and Procedures for Multicast in MPLS/BGP IP VPNs | requirements not reviewed | requirements not reviewed |

### Level 9

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 7432](../../rfc7432/) | BGP MPLS-Based Ethernet VPN | Ze proven 11, Proof pending 2, Not built 90 | Ze MUST proven 11, Proof pending 2, Not assessed 17, Not built 90 |

### Level 10

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 9136](../../rfc9136/) | IP Prefix Advertisement in Ethernet VPN (EVPN) | Ze proven 3, Proof pending 3, Not built 8 | Ze MUST proven 3, Proof pending 3, Not assessed 6, Not built 8 |

### No level: the repository has no copy of this RFC's text

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 4762](../../rfc4762/) | Virtual Private LAN Service (VPLS) Using Label Distribution Protocol (LDP) Signaling | requirements not reviewed | requirements not reviewed |
