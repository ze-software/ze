# DNS

Technology profile. DNS, DNSSEC, encrypted DNS transports, and AS112. The RFCs grouped under dns, showing mandatory requirement coverage and overall coverage for the group and each RFC.

[All profiles](../) · [All RFCs](../../)

Use case: [AS112 anycast DNS inside a network](../../../../use-cases/as112/)

Figures as of the last build, read from [data/rfc/profiles/dns.json](../../../../data/rfc/profiles/dns.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze proven 46, Proof pending 44, Not assessed 61, Not built 157
- **Overview bar**: Ze MUST proven 46, SHOULD proven 1, Not owed 3, Proof pending 44, Not assessed 286, Not built 157

| Figure | Rows |
|---|---|
| RFCs | 12 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 311 |
| MUST rows kept (the MUST bar) | 308 |
| Ze proven | 46 |
| OS verified | 0 |
| Proof pending | 44 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 61 |
| Not built | 157 |
| MUST rows not owed | 3 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 3 |
| SHOULD / MAY rows | 226 |
| SHOULD proven | 1 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD proof pending | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 225 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze proven means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Proof pending counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

## Tree

The RFCs are listed with the standards they build on first. An RFC appears one level after the highest level of its dependencies; RFCs that refer back to each other share a level. Each RFC has the same two coverage bars as the group above. An RFC whose requirement list has not been reviewed shows "requirements not reviewed" instead of a bar.

### Level 2

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 1035](../../rfc1035/) (cycle with rfc1034, rfc1122, rfc1123, rfc1191, rfc1195, rfc1332, rfc1350, rfc1812, rfc2131, rfc2132, rfc4291, rfc4443, rfc4861, rfc5905, rfc8200, rfc8201, rfc9293) | Domain Names - Implementation and Specification | Ze proven 12, Proof pending 13, Not assessed 2 | Ze MUST proven 12, Proof pending 13, Not assessed 8 |

### Level 3

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 2181](../../rfc2181/) | Clarifications to the DNS Specification | Ze proven 12, Proof pending 6, Not built 5 | Ze MUST proven 12, SHOULD proven 1, Proof pending 6, Not assessed 46, Not built 5 |

### Level 4

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 2782](../../rfc2782/) | A DNS RR for specifying the location of services (DNS SRV) | Not built 5 | Not owed 2, Not assessed 7, Not built 5 |

### Level 5

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 4033](../../rfc4033/) (cycle with rfc4034, rfc4035) | DNS Security Introduction and Requirements | no rows | no rows |
| [RFC 4034](../../rfc4034/) (cycle with rfc4033, rfc4035) | Resource Records for the DNS Security Extensions | Not assessed 59 | Not assessed 59 |
| [RFC 4035](../../rfc4035/) (cycle with rfc4033, rfc4034) | Protocol Modifications for the DNS Security Extensions | Ze proven 4, Proof pending 10, Not built 94 | Ze MUST proven 4, Proof pending 10, Not assessed 50, Not built 94 |
| [RFC 8484](../../rfc8484/) | DNS Queries over HTTPS (DoH) | Ze proven 1, Proof pending 8, Not built 6 | Ze MUST proven 1, Not owed 1, Proof pending 8, Not assessed 11, Not built 6 |

### Level 6

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 7534](../../rfc7534/) (cycle with rfc7535) | AS112 Nameserver Operations | Ze proven 2, Proof pending 1 | Ze MUST proven 2, Proof pending 1, Not assessed 13 |
| [RFC 7535](../../rfc7535/) (cycle with rfc7534) | AS112 Redirection Using DNAME | Not built 1 | Not assessed 6, Not built 1 |
| [RFC 7871](../../rfc7871/) | Client Subnet in DNS Queries | Proof pending 2, Not built 34 | Proof pending 2, Not assessed 52, Not built 34 |

### Level 7

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 5625](../../rfc5625/) | DNS Proxy Implementation Guidelines | Ze proven 12 | Ze MUST proven 12, Not assessed 15 |

### Level 8

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 7858](../../rfc7858/) | Specification for DNS over Transport Layer Security (TLS) | Ze proven 3, Proof pending 4, Not built 12 | Ze MUST proven 3, Proof pending 4, Not assessed 19, Not built 12 |
