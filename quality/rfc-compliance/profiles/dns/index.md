# DNS

Technology aggregate. DNS, DNSSEC, encrypted DNS transports, and AS112. The RFCs whose summary names the dns aggregate, with their mandatory and optional coverage.

[All profiles](../) · [All RFCs](../../)

Use case: [AS112 anycast DNS inside a network](../../../../use-cases/as112/)

- **Mandatory**: 11.4% (34 of 299 (11.4%)). MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that a recorded test break proves.
- **Optional**: 0.5% (1 of 211 (0.5%)). MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows, counted the same way.
- **Mandatory ceiling**: 49.8% (149 of 299 (49.8%); excluded: Optional feature not offered (classification can be revisited): 1 of 299 (0.3%); Not applicable (classification can be revisited): 149 of 299 (49.8%)). Attainable mandatory rows under current classifications, over the unchanged total.
- **Optional ceiling**: 100.0% (211 of 211 (100.0%)). Attainable optional rows under current classifications, over the unchanged total.
- **Not walked**: 0 (of 11 RFCs). RFCs with no valid extraction sign-off: their rows count in neither figure.
- **Mandatory register**: not complete (every row proven, none unwalked). A profile is complete only when it requires at least one RFC and none of them is unwalked.

Mandatory ceiling: 149 of 299 (49.8%); excluded: Optional feature not offered (classification can be revisited): 1 of 299 (0.3%); Not applicable (classification can be revisited): 149 of 299 (49.8%). Optional ceiling: 211 of 211 (100.0%). Lowest mandatory ceiling: rfc2782 0 of 7 (0.0%). Lowest optional ceiling: rfc1035 6 of 6 (100.0%). Ceilings describe current classifications, not an immutable Linux maximum. Not-applicable and feature-declined classifications can be revisited. Every gap remains work Ze owes.

## Tree

Roots first: an RFC sits one level above the highest level of the RFCs it builds on, and the RFCs of one citation cycle share a level. Each bar is the share of that figure's rows a recorded break proves; the ceiling below it shows attainable rows under current classifications. An RFC that is not walked shows an empty bar and no figure or ceiling.

### Level 2

| RFC | Title | Mandatory | Optional |
|---|---|---|---|
| [RFC 1035](../../rfc1035/) (cycle with rfc1034, rfc1122, rfc1191, rfc1195, rfc1332, rfc1812, rfc2131, rfc2132, rfc4291, rfc4443, rfc4861, rfc5905, rfc8200, rfc8201, rfc9293) | Domain Names - Implementation and Specification | 12 of 27 (44.4%); ceiling 27 of 27 (100.0%) | 0 of 6 (0.0%); ceiling 6 of 6 (100.0%) |

### Level 3

| RFC | Title | Mandatory | Optional |
|---|---|---|---|
| [RFC 2181](../../rfc2181/) | Clarifications to the DNS Specification | 12 of 23 (52.2%); ceiling 18 of 23 (78.3%); excluded: Optional feature not offered (classification can be revisited): 1 of 23 (4.3%); Not applicable (classification can be revisited): 4 of 23 (17.4%) | 1 of 47 (2.1%); ceiling 47 of 47 (100.0%) |

### Level 4

| RFC | Title | Mandatory | Optional |
|---|---|---|---|
| [RFC 2782](../../rfc2782/) | A DNS RR for specifying the location of services (DNS SRV) | 0 of 7 (0.0%); ceiling 0 of 7 (0.0%); excluded: Not applicable (classification can be revisited): 7 of 7 (100.0%) | 0 of 7 (0.0%); ceiling 7 of 7 (100.0%) |

### Level 5

| RFC | Title | Mandatory | Optional |
|---|---|---|---|
| [RFC 4033](../../rfc4033/) (cycle with rfc4034, rfc4035) | DNS Security Introduction and Requirements | no rows; ceiling no rows | no rows; ceiling no rows |
| [RFC 4034](../../rfc4034/) (cycle with rfc4033, rfc4035) | Resource Records for the DNS Security Extensions | 0 of 59 (0.0%); ceiling 59 of 59 (100.0%) | no rows; ceiling no rows |
| [RFC 4035](../../rfc4035/) (cycle with rfc4033, rfc4034) | Protocol Modifications for the DNS Security Extensions | 4 of 108 (3.7%); ceiling 17 of 108 (15.7%); excluded: Not applicable (classification can be revisited): 91 of 108 (84.3%) | 0 of 50 (0.0%); ceiling 50 of 50 (100.0%) |
| [RFC 8484](../../rfc8484/) | DNS Queries over HTTPS (DoH) | 1 of 16 (6.2%); ceiling 10 of 16 (62.5%); excluded: Not applicable (classification can be revisited): 6 of 16 (37.5%) | 0 of 11 (0.0%); ceiling 11 of 11 (100.0%) |

### Level 6

| RFC | Title | Mandatory | Optional |
|---|---|---|---|
| [RFC 7534](../../rfc7534/) (cycle with rfc7535) | AS112 Nameserver Operations | 2 of 3 (66.7%); ceiling 3 of 3 (100.0%) | 0 of 13 (0.0%); ceiling 13 of 13 (100.0%) |
| [RFC 7535](../../rfc7535/) (cycle with rfc7534) | AS112 Redirection Using DNAME | 0 of 1 (0.0%); ceiling 0 of 1 (0.0%); excluded: Not applicable (classification can be revisited): 1 of 1 (100.0%) | 0 of 6 (0.0%); ceiling 6 of 6 (100.0%) |
| [RFC 7871](../../rfc7871/) | Client Subnet in DNS Queries | 0 of 36 (0.0%); ceiling 7 of 36 (19.4%); excluded: Not applicable (classification can be revisited): 29 of 36 (80.6%) | 0 of 52 (0.0%); ceiling 52 of 52 (100.0%) |

### Level 7

| RFC | Title | Mandatory | Optional |
|---|---|---|---|
| [RFC 7858](../../rfc7858/) | Specification for DNS over Transport Layer Security (TLS) | 3 of 19 (15.8%); ceiling 8 of 19 (42.1%); excluded: Not applicable (classification can be revisited): 11 of 19 (57.9%) | 0 of 19 (0.0%); ceiling 19 of 19 (100.0%) |
