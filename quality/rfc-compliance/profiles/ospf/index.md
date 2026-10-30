# OSPF

Technology aggregate. OSPFv2 and OSPFv3. The RFCs whose summary names the ospf aggregate, with their mandatory and optional coverage.

[All profiles](../) · [All RFCs](../../)

- **Mandatory**: 41.7% (150 of 360 (41.7%)). MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that a recorded test break proves.
- **Optional**: 3.6% (8 of 222 (3.6%)). MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows, counted the same way.
- **Mandatory ceiling**: 97.8% (352 of 360 (97.8%); excluded: Met by a lower layer: 7 of 360 (1.9%); Not applicable (classification can be revisited): 1 of 360 (0.3%)). Attainable mandatory rows under current classifications, over the unchanged total.
- **Optional ceiling**: 100.0% (222 of 222 (100.0%)). Attainable optional rows under current classifications, over the unchanged total.
- **Not walked**: 3 (of 21 RFCs). RFCs with no valid extraction sign-off: their rows count in neither figure.
- **Mandatory register**: not complete (every row proven, none unwalked). A profile is complete only when it requires at least one RFC and none of them is unwalked.

Mandatory ceiling: 352 of 360 (97.8%); excluded: Met by a lower layer: 7 of 360 (1.9%); Not applicable (classification can be revisited): 1 of 360 (0.3%). Optional ceiling: 222 of 222 (100.0%). Lowest mandatory ceiling: rfc4552 21 of 27 (77.8%). Lowest optional ceiling: rfc2328 15 of 15 (100.0%). Ceilings describe current classifications, not an immutable Linux maximum. Not-applicable and feature-declined classifications can be revisited. Every gap remains work Ze owes.

## Tree

Roots first: an RFC sits one level above the highest level of the RFCs it builds on, and the RFCs of one citation cycle share a level. Each bar is the share of that figure's rows a recorded break proves; the ceiling below it shows attainable rows under current classifications. An RFC that is not walked shows an empty bar and no figure or ceiling.

### Level 4

| RFC | Title | Mandatory | Optional |
|---|---|---|---|
| [RFC 2328](../../rfc2328/) (cycle with rfc3101, rfc5250) | OSPF Version 2 | 57 of 67 (85.1%); ceiling 67 of 67 (100.0%) | 2 of 15 (13.3%); ceiling 15 of 15 (100.0%) |
| [RFC 3101](../../rfc3101/) (cycle with rfc2328, rfc5250) | The OSPF Not-So-Stubby Area (NSSA) Option | 14 of 28 (50.0%); ceiling 28 of 28 (100.0%) | 1 of 7 (14.3%); ceiling 7 of 7 (100.0%) |
| [RFC 5250](../../rfc5250/) (cycle with rfc2328, rfc3101) | The OSPF Opaque LSA Option | 10 of 15 (66.7%); ceiling 15 of 15 (100.0%) | 0 of 2 (0.0%); ceiling 2 of 2 (100.0%) |

### Level 5

| RFC | Title | Mandatory | Optional |
|---|---|---|---|
| [RFC 3623](../../rfc3623/) | Graceful OSPF Restart | 8 of 13 (61.5%); ceiling 13 of 13 (100.0%) | 0 of 13 (0.0%); ceiling 13 of 13 (100.0%) |
| [RFC 3630](../../rfc3630/) | Traffic Engineering (TE) Extensions to OSPF Version 2 | 0 of 5 (0.0%); ceiling 5 of 5 (100.0%) | 0 of 5 (0.0%); ceiling 5 of 5 (100.0%) |
| [RFC 5709](../../rfc5709/) | OSPFv2 HMAC-SHA Cryptographic Authentication | 5 of 15 (33.3%); ceiling 15 of 15 (100.0%) | 2 of 7 (28.6%); ceiling 7 of 7 (100.0%) |

### Level 6

| RFC | Title | Mandatory | Optional |
|---|---|---|---|
| [RFC 7474](../../rfc7474/) | Security Extensions for OSPFv2 when Using Manual Key Management | 7 of 12 (58.3%); ceiling 12 of 12 (100.0%) | 1 of 4 (25.0%); ceiling 4 of 4 (100.0%) |
| [RFC 7684](../../rfc7684/) | OSPFv2 Prefix/Link Attribute Advertisement | 4 of 9 (44.4%); ceiling 9 of 9 (100.0%) | 0 of 11 (0.0%); ceiling 11 of 11 (100.0%) |

### Level 9

| RFC | Title | Mandatory | Optional |
|---|---|---|---|
| [RFC 4552](../../rfc4552/) (cycle with rfc5340) | Authentication/Confidentiality for OSPFv3 | 6 of 27 (22.2%); ceiling 21 of 27 (77.8%); excluded: Met by a lower layer: 6 of 27 (22.2%) | 0 of 11 (0.0%); ceiling 11 of 11 (100.0%) |
| [RFC 5340](../../rfc5340/) (cycle with rfc4552) | OSPF for IPv6 | 6 of 23 (26.1%); ceiling 22 of 23 (95.7%); excluded: Met by a lower layer: 1 of 23 (4.3%) | 0 of 23 (0.0%); ceiling 23 of 23 (100.0%) |

### Level 10

| RFC | Title | Mandatory | Optional |
|---|---|---|---|
| [RFC 5187](../../rfc5187/) | OSPFv3 Graceful Restart | 4 of 4 (100.0%); ceiling 4 of 4 (100.0%) | no rows; ceiling no rows |
| [RFC 5392](../../rfc5392/) | OSPF Extensions in Support of Inter-Autonomous System (AS) MPLS and GMPLS Traffic Engineering | 5 of 14 (35.7%); ceiling 14 of 14 (100.0%) | 0 of 18 (0.0%); ceiling 18 of 18 (100.0%) |
| [RFC 5838](../../rfc5838/) | Support of Address Families in OSPFv3 | 4 of 16 (25.0%); ceiling 16 of 16 (100.0%) | 0 of 11 (0.0%); ceiling 11 of 11 (100.0%) |
| [RFC 6549](../../rfc6549/) | OSPFv2 Multi-Instance Extensions | 0 of 1 (0.0%); ceiling 1 of 1 (100.0%) | 0 of 3 (0.0%); ceiling 3 of 3 (100.0%) |
| [RFC 6987](../../rfc6987/) | OSPF Stub Router Advertisement | not walked | not walked |
| [RFC 7166](../../rfc7166/) | Supporting Authentication Trailer for OSPFv3 | 0 of 17 (0.0%); ceiling 17 of 17 (100.0%) | 0 of 17 (0.0%); ceiling 17 of 17 (100.0%) |
| [RFC 7770](../../rfc7770/) | Extensions to OSPF for Advertising Optional Router Capabilities | 1 of 11 (9.1%); ceiling 10 of 11 (90.9%); excluded: Not applicable (classification can be revisited): 1 of 11 (9.1%) | 1 of 14 (7.1%); ceiling 14 of 14 (100.0%) |

### Level 11

| RFC | Title | Mandatory | Optional |
|---|---|---|---|
| [RFC 8362](../../rfc8362/) | OSPFv3 Link State Advertisement (LSA) Extensibility | not walked | not walked |
| [RFC 8665](../../rfc8665/) | OSPF Extensions for Segment Routing | 13 of 51 (25.5%); ceiling 51 of 51 (100.0%) | 1 of 35 (2.9%); ceiling 35 of 35 (100.0%) |
| [RFC 9129](../../rfc9129/) | YANG Data Model for the OSPF Protocol | not walked | not walked |

### Level 12

| RFC | Title | Mandatory | Optional |
|---|---|---|---|
| [RFC 8666](../../rfc8666/) | OSPFv3 Extensions for Segment Routing | 6 of 32 (18.8%); ceiling 32 of 32 (100.0%) | 0 of 26 (0.0%); ceiling 26 of 26 (100.0%) |

## Optional (not counted)

RFCs of this aggregate listed as optional, each declared by the Aggregate tier row of its own summary. They count in none of the figures above until the owner brings one into scope by deleting that row. Their own figures: mandatory 0 of 50 (0.0%), optional 1 of 20 (5.0%); ceiling mandatory 50 of 50 (100.0%), optional 20 of 20 (100.0%); 2 RFCs, 0 not walked.

Mandatory ceiling: 50 of 50 (100.0%). Optional ceiling: 20 of 20 (100.0%). Every RFC shown with measured rows can reach 100% under current classifications. Lowest mandatory ceiling: rfc4576 4 of 4 (100.0%). Lowest optional ceiling: rfc4577 20 of 20 (100.0%). Ceilings describe current classifications, not an immutable Linux maximum. Not-applicable and feature-declined classifications can be revisited. Every gap remains work Ze owes.

| RFC | Title | Mandatory | Optional |
|---|---|---|---|
| [RFC 4576](../../rfc4576/) (cycle with rfc4577) | Using a Link State Advertisement (LSA) Options Bit to Prevent Looping in BGP/MPLS IP Virtual Private Networks (VPNs) | 0 of 4 (0.0%); ceiling 4 of 4 (100.0%) | no rows; ceiling no rows |
| [RFC 4577](../../rfc4577/) (cycle with rfc4576) | OSPF as the Provider/Customer Edge Protocol for BGP/MPLS IP Virtual Private Networks (VPNs) | 0 of 46 (0.0%); ceiling 46 of 46 (100.0%) | 1 of 20 (5.0%); ceiling 20 of 20 (100.0%) |
