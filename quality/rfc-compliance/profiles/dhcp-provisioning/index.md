# DHCP and provisioning

Technology profile. DHCP, address autoconfiguration, and TFTP provisioning. The RFCs grouped under dhcp-provisioning, showing mandatory requirement coverage and overall coverage for the group and each RFC.

[All profiles](../) · [All RFCs](../../)

Figures as of the last build, read from [data/rfc/profiles/dhcp-provisioning.json](../../../../data/rfc/profiles/dhcp-provisioning.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze proven 190, Proof pending 20, OS testing gap 116, Not built 147
- **Overview bar**: Ze MUST proven 190, SHOULD proven 10, Not owed 1, Proof pending 20, OS testing gap 116, Not assessed 209, Not built 148

| Figure | Rows |
|---|---|
| RFCs | 13 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 474 |
| MUST rows kept (the MUST bar) | 473 |
| Ze proven | 190 |
| OS verified | 0 |
| Proof pending | 20 |
| OS testing gap | 116 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 147 |
| MUST rows not owed | 1 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 1 |
| SHOULD / MAY rows | 220 |
| SHOULD proven | 10 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD proof pending | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 209 |
| SHOULD not built | 1 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze proven means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Proof pending counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

## Tree

The RFCs are listed with the standards they build on first. An RFC appears one level after the highest level of its dependencies; RFCs that refer back to each other share a level. Each RFC has the same two coverage bars as the group above. An RFC whose requirement list has not been reviewed shows "requirements not reviewed" instead of a bar.

### Level 2

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 1350](../../rfc1350/) (cycle with rfc1034, rfc1035, rfc1122, rfc1123, rfc1191, rfc1195, rfc1332, rfc1812, rfc2131, rfc2132, rfc4291, rfc4443, rfc4861, rfc5905, rfc8200, rfc8201, rfc9293) | The TFTP Protocol (Revision 2) | Ze proven 8, Not built 3 | Ze MUST proven 8, SHOULD proven 2, Not assessed 2, Not built 3 |
| [RFC 2131](../../rfc2131/) (cycle with rfc1034, rfc1035, rfc1122, rfc1123, rfc1191, rfc1195, rfc1332, rfc1350, rfc1812, rfc2132, rfc4291, rfc4443, rfc4861, rfc5905, rfc8200, rfc8201, rfc9293) | Dynamic Host Configuration Protocol | Ze proven 21, Proof pending 13, OS testing gap 9, Not built 28 | Ze MUST proven 21, SHOULD proven 3, Proof pending 13, OS testing gap 9, Not assessed 35, Not built 28 |
| [RFC 2132](../../rfc2132/) (cycle with rfc1034, rfc1035, rfc1122, rfc1123, rfc1191, rfc1195, rfc1332, rfc1350, rfc1812, rfc2131, rfc4291, rfc4443, rfc4861, rfc5905, rfc8200, rfc8201, rfc9293) | DHCP Options and BOOTP Vendor Extensions | Ze proven 1, Proof pending 7, Not built 26 | Ze MUST proven 1, Proof pending 7, Not assessed 34, Not built 26 |
| [RFC 4861](../../rfc4861/) (cycle with rfc1034, rfc1035, rfc1122, rfc1123, rfc1191, rfc1195, rfc1332, rfc1350, rfc1812, rfc2131, rfc2132, rfc4291, rfc4443, rfc5905, rfc8200, rfc8201, rfc9293) | Neighbor Discovery for IP version 6 (IPv6) | Ze proven 30, OS testing gap 91, Not built 17 | Ze MUST proven 30, OS testing gap 91, Not assessed 79, Not built 17 |

### Level 3

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 2347](../../rfc2347/) (cycle with rfc2348, rfc2349) | TFTP Option Extension | Ze proven 3 | Ze MUST proven 3, SHOULD proven 1, Not assessed 1 |
| [RFC 2348](../../rfc2348/) (cycle with rfc2347, rfc2349) | TFTP Blocksize Option | Ze proven 5 | Ze MUST proven 5 |
| [RFC 2349](../../rfc2349/) (cycle with rfc2347, rfc2348) | TFTP Timeout Interval and Transfer Size Options | Ze proven 3, Not built 1 | Ze MUST proven 3, SHOULD proven 1, Not assessed 1, Not built 1 |
| [RFC 4578](../../rfc4578/) | Dynamic Host Configuration Protocol (DHCP) Options for the Intel Preboot eXecution Environment (PXE) | Ze proven 1, Not built 3 | Ze MUST proven 1, Not owed 1, Not assessed 3, Not built 3 |
| [RFC 4862](../../rfc4862/) (cycle with rfc2464) | IPv6 Stateless Address Autoconfiguration | OS testing gap 6, Not built 10 | OS testing gap 6, Not assessed 14, Not built 10 |

### Level 4

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 7440](../../rfc7440/) | TFTP Windowsize Option | Ze proven 9 | Ze MUST proven 9, SHOULD proven 1, Not assessed 5 |
| [RFC 8106](../../rfc8106/) | IPv6 Router Advertisement Options for DNS Configuration | Ze proven 5, Not built 6 | Ze MUST proven 5, SHOULD proven 2, Not assessed 11, Not built 6 |
| [RFC 9915](../../rfc9915/) | Dynamic Host Configuration Protocol for IPv6 (DHCPv6) | Ze proven 85, OS testing gap 3, Not built 38 | Ze MUST proven 85, OS testing gap 3, Not built 38 |

### Level 7

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 7084](../../rfc7084/) | Basic Requirements for IPv6 Customer Edge Routers | Ze proven 19, OS testing gap 7, Not built 15 | Ze MUST proven 19, OS testing gap 7, Not assessed 24, Not built 16 |
