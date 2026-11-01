# OSPF

Technology aggregate. OSPFv2 and OSPFv3. The RFCs whose summary names the ospf aggregate, with the MUST bar and the overview bar of the set and of each RFC.

[All profiles](../) · [All RFCs](../../)

Figures as of the last build, read from [data/rfc/profiles/ospf.json](../../../../data/rfc/profiles/ospf.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze proven 152, OS testing gap 7, Ze correctness gap 200
- **Overview bar**: Ze MUST proven 152, SHOULD proven 9, Not owed 1, SHOULD not decided 213, OS testing gap 7, Ze correctness gap 200

| Figure | Rows |
|---|---|
| RFCs | 21 |
| Not walked | 3 |
| Mandatory register | not complete |
| MUST rows | 360 |
| MUST rows kept (the MUST bar) | 359 |
| Ze proven | 152 |
| OS verified | 0 |
| OS testing gap | 7 |
| Ze correctness gap | 200 |
| MUST rows not owed | 1 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 1 |
| SHOULD / MAY rows | 222 |
| SHOULD proven | 9 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 213 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

## Tree

Roots first: an RFC sits one level above the highest level of the RFCs it builds on, and the RFCs of one citation cycle share a level. Each RFC carries the same two bars as the set above, from its own data file; an RFC that is not walked shows "not walked" and no bar.

### Level 4

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 2328](../../rfc2328/) (cycle with rfc3101, rfc5250) | OSPF Version 2 | Ze proven 58, Ze correctness gap 9 | Ze MUST proven 58, SHOULD proven 2, SHOULD not decided 13, Ze correctness gap 9 |
| [RFC 3101](../../rfc3101/) (cycle with rfc2328, rfc5250) | The OSPF Not-So-Stubby Area (NSSA) Option | Ze proven 15, Ze correctness gap 13 | Ze MUST proven 15, SHOULD proven 2, SHOULD not decided 5, Ze correctness gap 13 |
| [RFC 5250](../../rfc5250/) (cycle with rfc2328, rfc3101) | The OSPF Opaque LSA Option | Ze proven 10, Ze correctness gap 5 | Ze MUST proven 10, SHOULD not decided 2, Ze correctness gap 5 |

### Level 5

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 3623](../../rfc3623/) | Graceful OSPF Restart | Ze proven 8, Ze correctness gap 5 | Ze MUST proven 8, SHOULD not decided 13, Ze correctness gap 5 |
| [RFC 3630](../../rfc3630/) | Traffic Engineering (TE) Extensions to OSPF Version 2 | Ze correctness gap 5 | SHOULD not decided 5, Ze correctness gap 5 |
| [RFC 5709](../../rfc5709/) | OSPFv2 HMAC-SHA Cryptographic Authentication | Ze proven 5, Ze correctness gap 10 | Ze MUST proven 5, SHOULD proven 2, SHOULD not decided 5, Ze correctness gap 10 |

### Level 6

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 7474](../../rfc7474/) | Security Extensions for OSPFv2 when Using Manual Key Management | Ze proven 7, Ze correctness gap 5 | Ze MUST proven 7, SHOULD proven 1, SHOULD not decided 3, Ze correctness gap 5 |
| [RFC 7684](../../rfc7684/) | OSPFv2 Prefix/Link Attribute Advertisement | Ze proven 4, Ze correctness gap 5 | Ze MUST proven 4, SHOULD not decided 11, Ze correctness gap 5 |

### Level 9

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 4552](../../rfc4552/) (cycle with rfc5340) | Authentication/Confidentiality for OSPFv3 | Ze proven 6, OS testing gap 6, Ze correctness gap 15 | Ze MUST proven 6, SHOULD not decided 11, OS testing gap 6, Ze correctness gap 15 |
| [RFC 5340](../../rfc5340/) (cycle with rfc4552) | OSPF for IPv6 | Ze proven 6, OS testing gap 1, Ze correctness gap 16 | Ze MUST proven 6, SHOULD not decided 23, OS testing gap 1, Ze correctness gap 16 |

### Level 10

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 5187](../../rfc5187/) | OSPFv3 Graceful Restart | Ze proven 4 | Ze MUST proven 4 |
| [RFC 5392](../../rfc5392/) | OSPF Extensions in Support of Inter-Autonomous System (AS) MPLS and GMPLS Traffic Engineering | Ze proven 5, Ze correctness gap 9 | Ze MUST proven 5, SHOULD not decided 18, Ze correctness gap 9 |
| [RFC 5838](../../rfc5838/) | Support of Address Families in OSPFv3 | Ze proven 4, Ze correctness gap 12 | Ze MUST proven 4, SHOULD not decided 11, Ze correctness gap 12 |
| [RFC 6549](../../rfc6549/) | OSPFv2 Multi-Instance Extensions | Ze correctness gap 1 | SHOULD not decided 3, Ze correctness gap 1 |
| [RFC 6987](../../rfc6987/) | OSPF Stub Router Advertisement | not walked | not walked |
| [RFC 7166](../../rfc7166/) | Supporting Authentication Trailer for OSPFv3 | Ze correctness gap 17 | SHOULD not decided 17, Ze correctness gap 17 |
| [RFC 7770](../../rfc7770/) | Extensions to OSPF for Advertising Optional Router Capabilities | Ze proven 1, Ze correctness gap 9 | Ze MUST proven 1, SHOULD proven 1, Not owed 1, SHOULD not decided 13, Ze correctness gap 9 |

### Level 11

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 8362](../../rfc8362/) | OSPFv3 Link State Advertisement (LSA) Extensibility | not walked | not walked |
| [RFC 8665](../../rfc8665/) | OSPF Extensions for Segment Routing | Ze proven 13, Ze correctness gap 38 | Ze MUST proven 13, SHOULD proven 1, SHOULD not decided 34, Ze correctness gap 38 |
| [RFC 9129](../../rfc9129/) | YANG Data Model for the OSPF Protocol | not walked | not walked |

### Level 12

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 8666](../../rfc8666/) | OSPFv3 Extensions for Segment Routing | Ze proven 6, Ze correctness gap 26 | Ze MUST proven 6, SHOULD not decided 26, Ze correctness gap 26 |

## Optional (not counted)

RFCs of this aggregate listed as optional, each declared by the Aggregate tier row of its own summary. They count in none of the figures above until the owner brings one into scope by deleting that row. Their own bars and figures follow.

- **MUST bar**: Ze correctness gap 50
- **Overview bar**: SHOULD proven 1, SHOULD not decided 19, Ze correctness gap 50

| Figure | Rows |
|---|---|
| RFCs | 2 |
| Not walked | 0 |
| Mandatory register | not complete |
| MUST rows | 50 |
| MUST rows kept (the MUST bar) | 50 |
| Ze proven | 0 |
| OS verified | 0 |
| OS testing gap | 0 |
| Ze correctness gap | 50 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 20 |
| SHOULD proven | 1 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 19 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 4576](../../rfc4576/) (cycle with rfc4577) | Using a Link State Advertisement (LSA) Options Bit to Prevent Looping in BGP/MPLS IP Virtual Private Networks (VPNs) | Ze correctness gap 4 | Ze correctness gap 4 |
| [RFC 4577](../../rfc4577/) (cycle with rfc4576) | OSPF as the Provider/Customer Edge Protocol for BGP/MPLS IP Virtual Private Networks (VPNs) | Ze correctness gap 46 | SHOULD proven 1, SHOULD not decided 19, Ze correctness gap 46 |
