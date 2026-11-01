# BGP communities and policy

Technology aggregate. BGP communities, route leak prevention, and policy signalling. The RFCs whose summary names the bgp-communities-policy aggregate, with the MUST bar and the overview bar of the set and of each RFC.

[All profiles](../) · [All RFCs](../../)

Figures as of the last build, read from [data/rfc/profiles/bgp-communities-policy.json](../../../../data/rfc/profiles/bgp-communities-policy.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze proven 28, Ze correctness gap 21
- **Overview bar**: Ze MUST proven 28, SHOULD proven 4, SHOULD not decided 33, Ze correctness gap 21

| Figure | Rows |
|---|---|
| RFCs | 11 |
| Not walked | 1 |
| Mandatory register | not complete |
| MUST rows | 49 |
| MUST rows kept (the MUST bar) | 49 |
| Ze proven | 28 |
| OS verified | 0 |
| OS testing gap | 0 |
| Ze correctness gap | 21 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 37 |
| SHOULD proven | 4 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 33 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

## Tree

Roots first: an RFC sits one level above the highest level of the RFCs it builds on, and the RFCs of one citation cycle share a level. Each RFC carries the same two bars as the set above, from its own data file; an RFC that is not walked shows "not walked" and no bar.

### Level 0

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 3882](../../rfc3882/) | Configuring BGP to Block Denial-of-Service Attacks | no rows | no rows |

### Level 1

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 5635](../../rfc5635/) | Remote Triggered Black Hole Filtering with Unicast Reverse Path Forwarding (uRPF) | no rows | SHOULD not decided 1 |

### Level 4

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 1997](../../rfc1997/) | BGP Communities Attribute | Ze proven 5 | Ze MUST proven 5, SHOULD not decided 4 |

### Level 5

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 3765](../../rfc3765/) | NOPEER Community for Border Gateway Protocol (BGP) Route Scope Control | no rows | SHOULD not decided 2 |
| [RFC 4360](../../rfc4360/) | BGP Extended Communities Attribute | Ze proven 2, Ze correctness gap 3 | Ze MUST proven 2, SHOULD not decided 4, Ze correctness gap 3 |

### Level 6

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 5701](../../rfc5701/) | IPv6 Address Specific BGP Extended Community Attribute | Ze correctness gap 4 | SHOULD not decided 2, Ze correctness gap 4 |

### Level 8

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 7611](../../rfc7611/) | BGP ACCEPT_OWN Community Attribute | Ze proven 2, Ze correctness gap 3 | Ze MUST proven 2, SHOULD not decided 3, Ze correctness gap 3 |
| [RFC 7999](../../rfc7999/) | BLACKHOLE Community | Ze proven 4 | Ze MUST proven 4, SHOULD proven 3, SHOULD not decided 9 |
| [RFC 8092](../../rfc8092/) | BGP Large Communities Attribute | Ze proven 7 | Ze MUST proven 7, SHOULD not decided 4 |
| [RFC 9234](../../rfc9234/) | Route Leak Prevention and Detection Using Roles in UPDATE and OPEN Messages | Ze proven 8, Ze correctness gap 11 | Ze MUST proven 8, SHOULD proven 1, SHOULD not decided 4, Ze correctness gap 11 |

### Level 9

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 8195](../../rfc8195/) | Use of BGP Large Communities | not walked | not walked |
