# BGP VPN and EVPN

Technology aggregate. BGP/MPLS VPNs, labeled routes, VPLS, and EVPN. The RFCs whose summary names the bgp-vpn-evpn aggregate, with the MUST bar and the overview bar of the set and of each RFC.

[All profiles](../) · [All RFCs](../../)

Figures as of the last build, read from [data/rfc/profiles/bgp-vpn-evpn.json](../../../../data/rfc/profiles/bgp-vpn-evpn.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze proven 31, Ze correctness gap 171
- **Overview bar**: Ze MUST proven 31, SHOULD proven 1, SHOULD not decided 53, Ze correctness gap 171

| Figure | Rows |
|---|---|
| RFCs | 9 |
| Not walked | 2 |
| Mandatory register | not complete |
| MUST rows | 202 |
| MUST rows kept (the MUST bar) | 202 |
| Ze proven | 31 |
| OS verified | 0 |
| OS testing gap | 0 |
| Ze correctness gap | 171 |
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
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 53 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

## Tree

Roots first: an RFC sits one level above the highest level of the RFCs it builds on, and the RFCs of one citation cycle share a level. Each RFC carries the same two bars as the set above, from its own data file; an RFC that is not walked shows "not walked" and no bar.

### Level 7

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 4364](../../rfc4364/) (cycle with rfc4659, rfc4798, rfc6368, rfc7606, rfc8277, rfc8950) | BGP/MPLS IP Virtual Private Networks (VPNs) | Ze correctness gap 8 | SHOULD not decided 5, Ze correctness gap 8 |
| [RFC 4659](../../rfc4659/) (cycle with rfc4364, rfc4798, rfc6368, rfc7606, rfc8277, rfc8950) | BGP-MPLS IP Virtual Private Network (VPN) Extension for IPv6 VPN | Ze proven 4, Ze correctness gap 14 | Ze MUST proven 4, SHOULD not decided 6, Ze correctness gap 14 |
| [RFC 8277](../../rfc8277/) (cycle with rfc4364, rfc4659, rfc4798, rfc6368, rfc7606, rfc8950) | Using BGP to Bind MPLS Labels to Address Prefixes | Ze proven 11, Ze correctness gap 24 | Ze MUST proven 11, SHOULD proven 1, SHOULD not decided 6, Ze correctness gap 24 |

### Level 8

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 4684](../../rfc4684/) | Constrained Route Distribution for Border Gateway Protocol/MultiProtocol Label Switching (BGP/MPLS) Internet Protocol (IP) Virtual Private Networks (VPNs) | Ze proven 2, Ze correctness gap 4 | Ze MUST proven 2, SHOULD not decided 4, Ze correctness gap 4 |
| [RFC 4761](../../rfc4761/) | Virtual Private LAN Service (VPLS) Using BGP for Auto-Discovery and Signaling | Ze correctness gap 18 | SHOULD not decided 9, Ze correctness gap 18 |
| [RFC 6514](../../rfc6514/) | BGP Encodings and Procedures for Multicast in MPLS/BGP IP VPNs | not walked | not walked |

### Level 9

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 7432](../../rfc7432/) | BGP MPLS-Based Ethernet VPN | Ze proven 11, Ze correctness gap 92 | Ze MUST proven 11, SHOULD not decided 17, Ze correctness gap 92 |

### Level 10

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 9136](../../rfc9136/) | IP Prefix Advertisement in Ethernet VPN (EVPN) | Ze proven 3, Ze correctness gap 11 | Ze MUST proven 3, SHOULD not decided 6, Ze correctness gap 11 |

### No level: the corpus holds no text of the RFC

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 4762](../../rfc4762/) | Virtual Private LAN Service (VPLS) Using Label Distribution Protocol (LDP) Signaling | not walked | not walked |
