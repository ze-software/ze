# BGP segment routing and tunnels

Technology aggregate. BGP segment routing, SRv6, and tunnel encapsulation. The RFCs whose summary names the bgp-sr-tunnel aggregate, with the MUST bar and the overview bar of the set and of each RFC.

[All profiles](../) · [All RFCs](../../)

Figures as of the last build, read from [data/rfc/profiles/bgp-sr-tunnel.json](../../../../data/rfc/profiles/bgp-sr-tunnel.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze proven 89, Ze correctness gap 154
- **Overview bar**: Ze MUST proven 89, SHOULD proven 3, Not owed 1, SHOULD not decided 133, Ze correctness gap 154

| Figure | Rows |
|---|---|
| RFCs | 5 |
| Not walked | 0 |
| Mandatory register | not complete |
| MUST rows | 244 |
| MUST rows kept (the MUST bar) | 243 |
| Ze proven | 89 |
| OS verified | 0 |
| OS testing gap | 0 |
| Ze correctness gap | 154 |
| MUST rows not owed | 1 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 1 |
| SHOULD / MAY rows | 136 |
| SHOULD proven | 3 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 133 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

## Tree

Roots first: an RFC sits one level above the highest level of the RFCs it builds on, and the RFCs of one citation cycle share a level. Each RFC carries the same two bars as the set above, from its own data file; an RFC that is not walked shows "not walked" and no bar.

### Level 8

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 8669](../../rfc8669/) | Segment Routing Prefix Segment Identifier Extensions for BGP | Ze proven 13, Ze correctness gap 11 | Ze MUST proven 13, SHOULD proven 3, SHOULD not decided 18, Ze correctness gap 11 |

### Level 9

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 9012](../../rfc9012/) | The BGP Tunnel Encapsulation Attribute | Ze proven 25, Ze correctness gap 50 | Ze MUST proven 25, Not owed 1, SHOULD not decided 22, Ze correctness gap 50 |

### Level 11

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 9252](../../rfc9252/) | BGP Overlay Services Based on Segment Routing over IPv6 (SRv6) | Ze proven 14, Ze correctness gap 11 | Ze MUST proven 14, SHOULD not decided 5, Ze correctness gap 11 |

### Level 13

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 9256](../../rfc9256/) | Segment Routing Policy Architecture | Ze correctness gap 22 | SHOULD not decided 47, Ze correctness gap 22 |

### Level 14

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 9830](../../rfc9830/) | BGP Extensions for the Advertisement of Segment Routing (SR) Policies | Ze proven 37, Ze correctness gap 60 | Ze MUST proven 37, SHOULD not decided 41, Ze correctness gap 60 |
