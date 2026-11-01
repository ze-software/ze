# DHCP and provisioning

Technology aggregate. DHCP, address autoconfiguration, and TFTP provisioning. The RFCs whose summary names the dhcp-provisioning aggregate, with the MUST bar and the overview bar of the set and of each RFC.

[All profiles](../) · [All RFCs](../../)

Figures as of the last build, read from [data/rfc/profiles/dhcp-provisioning.json](../../../../data/rfc/profiles/dhcp-provisioning.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze proven 174, OS testing gap 116, Ze correctness gap 182
- **Overview bar**: Ze MUST proven 174, SHOULD proven 7, Not owed 1, SHOULD not decided 213, OS testing gap 116, Ze correctness gap 182

| Figure | Rows |
|---|---|
| RFCs | 13 |
| Not walked | 0 |
| Mandatory register | not complete |
| MUST rows | 473 |
| MUST rows kept (the MUST bar) | 472 |
| Ze proven | 174 |
| OS verified | 0 |
| OS testing gap | 116 |
| Ze correctness gap | 182 |
| MUST rows not owed | 1 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 1 |
| SHOULD / MAY rows | 220 |
| SHOULD proven | 7 |
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

### Level 0

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 1350](../../rfc1350/) | The TFTP Protocol (Revision 2) | Ze proven 8, Ze correctness gap 2 | Ze MUST proven 8, SHOULD proven 2, SHOULD not decided 2, Ze correctness gap 2 |

### Level 1

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 2347](../../rfc2347/) (cycle with rfc2348, rfc2349) | TFTP Option Extension | Ze proven 3 | Ze MUST proven 3, SHOULD proven 1, SHOULD not decided 1 |
| [RFC 2348](../../rfc2348/) (cycle with rfc2347, rfc2349) | TFTP Blocksize Option | Ze proven 5 | Ze MUST proven 5 |
| [RFC 2349](../../rfc2349/) (cycle with rfc2347, rfc2348) | TFTP Timeout Interval and Transfer Size Options | Ze proven 3, Ze correctness gap 1 | Ze MUST proven 3, SHOULD proven 1, SHOULD not decided 1, Ze correctness gap 1 |

### Level 2

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 2131](../../rfc2131/) (cycle with rfc1034, rfc1035, rfc1122, rfc1191, rfc1195, rfc1332, rfc1812, rfc2132, rfc4291, rfc4443, rfc4861, rfc5905, rfc8200, rfc8201, rfc9293) | Dynamic Host Configuration Protocol | Ze proven 18, OS testing gap 9, Ze correctness gap 44 | Ze MUST proven 18, SHOULD proven 2, SHOULD not decided 36, OS testing gap 9, Ze correctness gap 44 |
| [RFC 2132](../../rfc2132/) (cycle with rfc1034, rfc1035, rfc1122, rfc1191, rfc1195, rfc1332, rfc1812, rfc2131, rfc4291, rfc4443, rfc4861, rfc5905, rfc8200, rfc8201, rfc9293) | DHCP Options and BOOTP Vendor Extensions | Ze proven 1, Ze correctness gap 33 | Ze MUST proven 1, SHOULD not decided 34, Ze correctness gap 33 |
| [RFC 4861](../../rfc4861/) (cycle with rfc1034, rfc1035, rfc1122, rfc1191, rfc1195, rfc1332, rfc1812, rfc2131, rfc2132, rfc4291, rfc4443, rfc5905, rfc8200, rfc8201, rfc9293) | Neighbor Discovery for IP version 6 (IPv6) | Ze proven 30, OS testing gap 91, Ze correctness gap 17 | Ze MUST proven 30, SHOULD not decided 79, OS testing gap 91, Ze correctness gap 17 |
| [RFC 7440](../../rfc7440/) | TFTP Windowsize Option | Ze proven 9 | Ze MUST proven 9, SHOULD proven 1, SHOULD not decided 5 |

### Level 3

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 4578](../../rfc4578/) | Dynamic Host Configuration Protocol (DHCP) Options for the Intel Preboot eXecution Environment (PXE) | Ze proven 1, Ze correctness gap 3 | Ze MUST proven 1, Not owed 1, SHOULD not decided 3, Ze correctness gap 3 |
| [RFC 4862](../../rfc4862/) (cycle with rfc2464) | IPv6 Stateless Address Autoconfiguration | OS testing gap 6, Ze correctness gap 10 | SHOULD not decided 14, OS testing gap 6, Ze correctness gap 10 |

### Level 4

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 8106](../../rfc8106/) | IPv6 Router Advertisement Options for DNS Configuration | Ze correctness gap 11 | SHOULD not decided 13, Ze correctness gap 11 |
| [RFC 9915](../../rfc9915/) | Dynamic Host Configuration Protocol for IPv6 (DHCPv6) | Ze proven 82, OS testing gap 3, Ze correctness gap 41 | Ze MUST proven 82, OS testing gap 3, Ze correctness gap 41 |

### Level 7

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 7084](../../rfc7084/) | Basic Requirements for IPv6 Customer Edge Routers | Ze proven 14, OS testing gap 7, Ze correctness gap 20 | Ze MUST proven 14, SHOULD not decided 25, OS testing gap 7, Ze correctness gap 20 |
