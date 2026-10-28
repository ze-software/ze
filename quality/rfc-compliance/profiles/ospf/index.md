# OSPF

Technology aggregate. OSPFv2 and OSPFv3. The RFCs whose summary names the ospf aggregate, with their mandatory and optional coverage.

[All profiles](../) · [All RFCs](../../)

- **Mandatory**: 36.9% (151 of 409 (36.9%)). MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that a recorded test break proves.
- **Optional**: 4.1% (10 of 242 (4.1%)). MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows, counted the same way.
- **Mandatory ceiling**: 80.7% (330 of 409 (80.7%); excluded: Optional feature not offered (classification can be revisited): 1 of 409 (0.2%); Met by a lower layer: 6 of 409 (1.5%); Not applicable (classification can be revisited): 72 of 409 (17.6%)). Attainable mandatory rows under current classifications, over the unchanged total.
- **Optional ceiling**: 92.6% (224 of 242 (92.6%); excluded: Not applicable (classification can be revisited): 18 of 242 (7.4%)). Attainable optional rows under current classifications, over the unchanged total.
- **Not walked**: 3 (of 23 RFCs). RFCs with no valid extraction sign-off: their rows count in neither figure.
- **Mandatory register**: not complete (every row proven, none unwalked). A profile is complete only when it requires at least one RFC and none of them is unwalked.

Mandatory ceiling: 330 of 409 (80.7%); excluded: Optional feature not offered (classification can be revisited): 1 of 409 (0.2%); Met by a lower layer: 6 of 409 (1.5%); Not applicable (classification can be revisited): 72 of 409 (17.6%). Optional ceiling: 224 of 242 (92.6%); excluded: Not applicable (classification can be revisited): 18 of 242 (7.4%). Lowest mandatory ceiling: rfc4576 0 of 4 (0.0%). Lowest optional ceiling: rfc4577 2 of 20 (10.0%). Ceilings describe current classifications, not an immutable Linux maximum. Not-applicable and feature-declined classifications can be revisited. Every gap remains work Ze owes.

## Tree

Roots first: an RFC sits one level above the highest level of the RFCs it builds on, and the RFCs of one citation cycle share a level. Each bar is the share of that figure's rows a recorded break proves; the ceiling below it shows attainable rows under current classifications. An RFC that is not walked shows an empty bar and no figure or ceiling.

### Level 4

| RFC | Title | Mandatory | Optional |
|---|---|---|---|
| [RFC 2328](../../rfc2328/) (cycle with rfc3101, rfc5250) | OSPF Version 2 | 57 of 67 (85.1%); ceiling 67 of 67 (100.0%) | 2 of 15 (13.3%); ceiling 15 of 15 (100.0%) |
| [RFC 3101](../../rfc3101/) (cycle with rfc2328, rfc5250) | The OSPF Not-So-Stubby Area (NSSA) Option | 15 of 28 (53.6%); ceiling 27 of 28 (96.4%); excluded: Optional feature not offered (classification can be revisited): 1 of 28 (3.6%) | 2 of 7 (28.6%); ceiling 7 of 7 (100.0%) |
| [RFC 5250](../../rfc5250/) (cycle with rfc2328, rfc3101) | The OSPF Opaque LSA Option | 10 of 15 (66.7%); ceiling 15 of 15 (100.0%) | 0 of 2 (0.0%); ceiling 2 of 2 (100.0%) |

### Level 5

| RFC | Title | Mandatory | Optional |
|---|---|---|---|
| [RFC 3623](../../rfc3623/) | Graceful OSPF Restart | 8 of 13 (61.5%); ceiling 11 of 13 (84.6%); excluded: Not applicable (classification can be revisited): 2 of 13 (15.4%) | 0 of 13 (0.0%); ceiling 13 of 13 (100.0%) |
| [RFC 3630](../../rfc3630/) | Traffic Engineering (TE) Extensions to OSPF Version 2 | 0 of 5 (0.0%); ceiling 1 of 5 (20.0%); excluded: Not applicable (classification can be revisited): 4 of 5 (80.0%) | 0 of 5 (0.0%); ceiling 5 of 5 (100.0%) |
| [RFC 5709](../../rfc5709/) | OSPFv2 HMAC-SHA Cryptographic Authentication | 5 of 15 (33.3%); ceiling 15 of 15 (100.0%) | 2 of 7 (28.6%); ceiling 7 of 7 (100.0%) |

### Level 6

| RFC | Title | Mandatory | Optional |
|---|---|---|---|
| [RFC 7474](../../rfc7474/) | Security Extensions for OSPFv2 when Using Manual Key Management | 7 of 12 (58.3%); ceiling 12 of 12 (100.0%) | 1 of 4 (25.0%); ceiling 4 of 4 (100.0%) |
| [RFC 7684](../../rfc7684/) | OSPFv2 Prefix/Link Attribute Advertisement | 4 of 9 (44.4%); ceiling 6 of 9 (66.7%); excluded: Not applicable (classification can be revisited): 3 of 9 (33.3%) | 0 of 11 (0.0%); ceiling 11 of 11 (100.0%) |

### Level 8

| RFC | Title | Mandatory | Optional |
|---|---|---|---|
| [RFC 4576](../../rfc4576/) (cycle with rfc4577) | Using a Link State Advertisement (LSA) Options Bit to Prevent Looping in BGP/MPLS IP Virtual Private Networks (VPNs) | 0 of 4 (0.0%); ceiling 0 of 4 (0.0%); excluded: Not applicable (classification can be revisited): 4 of 4 (100.0%) | no rows; ceiling no rows |
| [RFC 4577](../../rfc4577/) (cycle with rfc4576) | OSPF as the Provider/Customer Edge Protocol for BGP/MPLS IP Virtual Private Networks (VPNs) | 0 of 45 (0.0%); ceiling 1 of 45 (2.2%); excluded: Not applicable (classification can be revisited): 44 of 45 (97.8%) | 1 of 20 (5.0%); ceiling 2 of 20 (10.0%); excluded: Not applicable (classification can be revisited): 18 of 20 (90.0%) |

### Level 9

| RFC | Title | Mandatory | Optional |
|---|---|---|---|
| [RFC 4552](../../rfc4552/) (cycle with rfc5340) | Authentication/Confidentiality for OSPFv3 | 6 of 27 (22.2%); ceiling 20 of 27 (74.1%); excluded: Met by a lower layer: 6 of 27 (22.2%); Not applicable (classification can be revisited): 1 of 27 (3.7%) | 0 of 11 (0.0%); ceiling 11 of 11 (100.0%) |
| [RFC 5340](../../rfc5340/) (cycle with rfc4552) | OSPF for IPv6 | 6 of 23 (26.1%); ceiling 22 of 23 (95.7%); excluded: Not applicable (classification can be revisited): 1 of 23 (4.3%) | 0 of 23 (0.0%); ceiling 23 of 23 (100.0%) |

### Level 10

| RFC | Title | Mandatory | Optional |
|---|---|---|---|
| [RFC 5187](../../rfc5187/) | OSPFv3 Graceful Restart | 4 of 4 (100.0%); ceiling 4 of 4 (100.0%) | no rows; ceiling no rows |
| [RFC 5392](../../rfc5392/) | OSPF Extensions in Support of Inter-Autonomous System (AS) MPLS and GMPLS Traffic Engineering | 5 of 14 (35.7%); ceiling 14 of 14 (100.0%) | 0 of 18 (0.0%); ceiling 18 of 18 (100.0%) |
| [RFC 5838](../../rfc5838/) | Support of Address Families in OSPFv3 | 4 of 16 (25.0%); ceiling 14 of 16 (87.5%); excluded: Not applicable (classification can be revisited): 2 of 16 (12.5%) | 0 of 11 (0.0%); ceiling 11 of 11 (100.0%) |
| [RFC 6549](../../rfc6549/) | OSPFv2 Multi-Instance Extensions | 0 of 1 (0.0%); ceiling 1 of 1 (100.0%) | 0 of 3 (0.0%); ceiling 3 of 3 (100.0%) |
| [RFC 6987](../../rfc6987/) | OSPF Stub Router Advertisement | not walked | not walked |
| [RFC 7166](../../rfc7166/) | Supporting Authentication Trailer for OSPFv3 | 0 of 17 (0.0%); ceiling 17 of 17 (100.0%) | 0 of 17 (0.0%); ceiling 17 of 17 (100.0%) |
| [RFC 7770](../../rfc7770/) | Extensions to OSPF for Advertising Optional Router Capabilities | 1 of 11 (9.1%); ceiling 4 of 11 (36.4%); excluded: Not applicable (classification can be revisited): 7 of 11 (63.6%) | 1 of 14 (7.1%); ceiling 14 of 14 (100.0%) |

### Level 11

| RFC | Title | Mandatory | Optional |
|---|---|---|---|
| [RFC 8362](../../rfc8362/) | OSPFv3 Link State Advertisement (LSA) Extensibility | not walked | not walked |
| [RFC 8665](../../rfc8665/) | OSPF Extensions for Segment Routing | 13 of 51 (25.5%); ceiling 51 of 51 (100.0%) | 1 of 35 (2.9%); ceiling 35 of 35 (100.0%) |
| [RFC 9129](../../rfc9129/) | YANG Data Model for the OSPF Protocol | not walked | not walked |

### Level 12

| RFC | Title | Mandatory | Optional |
|---|---|---|---|
| [RFC 8666](../../rfc8666/) | OSPFv3 Extensions for Segment Routing | 6 of 32 (18.8%); ceiling 28 of 32 (87.5%); excluded: Not applicable (classification can be revisited): 4 of 32 (12.5%) | 0 of 26 (0.0%); ceiling 26 of 26 (100.0%) |
