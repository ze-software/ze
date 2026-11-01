# DNS

Technology aggregate. DNS, DNSSEC, encrypted DNS transports, and AS112. The RFCs whose summary names the dns aggregate, with the MUST bar and the overview bar of the set and of each RFC.

[All profiles](../) · [All RFCs](../../)

Use case: [AS112 anycast DNS inside a network](../../../../use-cases/as112/)

Figures as of the last build, read from [data/rfc/profiles/dns.json](../../../../data/rfc/profiles/dns.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze proven 34, Ze correctness gap 262
- **Overview bar**: Ze MUST proven 34, SHOULD proven 1, Not owed 3, SHOULD not decided 210, Ze correctness gap 262

| Figure | Rows |
|---|---|
| RFCs | 11 |
| Not walked | 0 |
| Mandatory register | not complete |
| MUST rows | 299 |
| MUST rows kept (the MUST bar) | 296 |
| Ze proven | 34 |
| OS verified | 0 |
| OS testing gap | 0 |
| Ze correctness gap | 262 |
| MUST rows not owed | 3 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 3 |
| SHOULD / MAY rows | 211 |
| SHOULD proven | 1 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 210 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

## Tree

Roots first: an RFC sits one level above the highest level of the RFCs it builds on, and the RFCs of one citation cycle share a level. Each RFC carries the same two bars as the set above, from its own data file; an RFC that is not walked shows "not walked" and no bar.

### Level 2

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 1035](../../rfc1035/) (cycle with rfc1034, rfc1122, rfc1191, rfc1195, rfc1332, rfc1812, rfc2131, rfc2132, rfc4291, rfc4443, rfc4861, rfc5905, rfc8200, rfc8201, rfc9293) | Domain Names - Implementation and Specification | Ze proven 12, Ze correctness gap 15 | Ze MUST proven 12, SHOULD not decided 6, Ze correctness gap 15 |

### Level 3

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 2181](../../rfc2181/) | Clarifications to the DNS Specification | Ze proven 12, Ze correctness gap 11 | Ze MUST proven 12, SHOULD proven 1, SHOULD not decided 46, Ze correctness gap 11 |

### Level 4

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 2782](../../rfc2782/) | A DNS RR for specifying the location of services (DNS SRV) | Ze correctness gap 5 | Not owed 2, SHOULD not decided 7, Ze correctness gap 5 |

### Level 5

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 4033](../../rfc4033/) (cycle with rfc4034, rfc4035) | DNS Security Introduction and Requirements | no rows | no rows |
| [RFC 4034](../../rfc4034/) (cycle with rfc4033, rfc4035) | Resource Records for the DNS Security Extensions | Ze correctness gap 59 | Ze correctness gap 59 |
| [RFC 4035](../../rfc4035/) (cycle with rfc4033, rfc4034) | Protocol Modifications for the DNS Security Extensions | Ze proven 4, Ze correctness gap 104 | Ze MUST proven 4, SHOULD not decided 50, Ze correctness gap 104 |
| [RFC 8484](../../rfc8484/) | DNS Queries over HTTPS (DoH) | Ze proven 1, Ze correctness gap 14 | Ze MUST proven 1, Not owed 1, SHOULD not decided 11, Ze correctness gap 14 |

### Level 6

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 7534](../../rfc7534/) (cycle with rfc7535) | AS112 Nameserver Operations | Ze proven 2, Ze correctness gap 1 | Ze MUST proven 2, SHOULD not decided 13, Ze correctness gap 1 |
| [RFC 7535](../../rfc7535/) (cycle with rfc7534) | AS112 Redirection Using DNAME | Ze correctness gap 1 | SHOULD not decided 6, Ze correctness gap 1 |
| [RFC 7871](../../rfc7871/) | Client Subnet in DNS Queries | Ze correctness gap 36 | SHOULD not decided 52, Ze correctness gap 36 |

### Level 7

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 7858](../../rfc7858/) | Specification for DNS over Transport Layer Security (TLS) | Ze proven 3, Ze correctness gap 16 | Ze MUST proven 3, SHOULD not decided 19, Ze correctness gap 16 |
