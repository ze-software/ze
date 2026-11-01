# IS-IS

Technology aggregate. IS-IS. The RFCs whose summary names the isis aggregate, with the MUST bar and the overview bar of the set and of each RFC.

[All profiles](../) · [All RFCs](../../)

Figures as of the last build, read from [data/rfc/profiles/isis.json](../../../../data/rfc/profiles/isis.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze proven 64, Ze correctness gap 61
- **Overview bar**: Ze MUST proven 64, SHOULD proven 1, SHOULD not decided 35, Ze correctness gap 61

| Figure | Rows |
|---|---|
| RFCs | 11 |
| Not walked | 1 |
| Mandatory register | not complete |
| MUST rows | 125 |
| MUST rows kept (the MUST bar) | 125 |
| Ze proven | 64 |
| OS verified | 0 |
| OS testing gap | 0 |
| Ze correctness gap | 61 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 36 |
| SHOULD proven | 1 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 35 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

## Tree

Roots first: an RFC sits one level above the highest level of the RFCs it builds on, and the RFCs of one citation cycle share a level. Each RFC carries the same two bars as the set above, from its own data file; an RFC that is not walked shows "not walked" and no bar.

### Level 0

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 5301](../../rfc5301/) | Dynamic Hostname Exchange Mechanism for IS-IS | Ze correctness gap 7 | SHOULD not decided 8, Ze correctness gap 7 |
| [RFC 5304](../../rfc5304/) | IS-IS Cryptographic Authentication | Ze correctness gap 9 | SHOULD not decided 5, Ze correctness gap 9 |
| [RFC 5305](../../rfc5305/) | IS-IS Extensions for Traffic Engineering | Ze proven 8, Ze correctness gap 6 | Ze MUST proven 8, SHOULD not decided 8, Ze correctness gap 6 |

### Level 2

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 1195](../../rfc1195/) (cycle with rfc1034, rfc1035, rfc1122, rfc1191, rfc1332, rfc1812, rfc2131, rfc2132, rfc4291, rfc4443, rfc4861, rfc5905, rfc8200, rfc8201, rfc9293) | Use of OSI IS-IS for Routing in TCP/IP and Dual Environments | Ze proven 25, Ze correctness gap 16 | Ze MUST proven 25, SHOULD not decided 1, Ze correctness gap 16 |

### Level 3

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 2966](../../rfc2966/) | Domain-wide Prefix Distribution with Two-Level IS-IS | Ze proven 3, Ze correctness gap 3 | Ze MUST proven 3, SHOULD not decided 2, Ze correctness gap 3 |
| [RFC 3786](../../rfc3786/) | Extending the Number of Intermediate System to Intermediate System (IS-IS) Link State PDU (LSP) Fragments Beyond the 256 Limit | Ze proven 1, Ze correctness gap 8 | Ze MUST proven 1, SHOULD not decided 4, Ze correctness gap 8 |
| [RFC 3787](../../rfc3787/) | Recommendations for Interoperable IP Networks using Intermediate System to Intermediate System (IS-IS) | Ze proven 5 | Ze MUST proven 5, SHOULD not decided 3 |
| [RFC 5303](../../rfc5303/) | Three-Way Handshake for IS-IS Point-to-Point Adjacencies | Ze proven 8, Ze correctness gap 9 | Ze MUST proven 8, SHOULD not decided 1, Ze correctness gap 9 |
| [RFC 5308](../../rfc5308/) | Routing IPv6 with IS-IS | Ze proven 8 | Ze MUST proven 8, SHOULD not decided 1 |
| [RFC 5310](../../rfc5310/) | IS-IS Generic Cryptographic Authentication | Ze proven 6, Ze correctness gap 3 | Ze MUST proven 6, SHOULD proven 1, SHOULD not decided 2, Ze correctness gap 3 |

### No level: the corpus holds no text of the RFC

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 5120](../../rfc5120/) | M-ISIS: Multi Topology (MT) Routing in Intermediate System to Intermediate Systems (IS-ISs) | not walked | not walked |
