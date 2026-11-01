# Network primitives

Technology aggregate. Network primitives other protocols build on. The RFCs whose summary names the primitives aggregate, with the MUST bar and the overview bar of the set and of each RFC.

[All profiles](../) · [All RFCs](../../)

Figures as of the last build, read from [data/rfc/profiles/primitives.json](../../../../data/rfc/profiles/primitives.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze proven 20, Ze correctness gap 31
- **Overview bar**: Ze MUST proven 20, SHOULD not decided 34, Ze correctness gap 31

| Figure | Rows |
|---|---|
| RFCs | 6 |
| Not walked | 0 |
| Mandatory register | not complete |
| MUST rows | 51 |
| MUST rows kept (the MUST bar) | 51 |
| Ze proven | 20 |
| OS verified | 0 |
| OS testing gap | 0 |
| Ze correctness gap | 31 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 34 |
| SHOULD proven | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 34 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

## Tree

Roots first: an RFC sits one level above the highest level of the RFCs it builds on, and the RFCs of one citation cycle share a level. Each RFC carries the same two bars as the set above, from its own data file; an RFC that is not walked shows "not walked" and no bar.

### Level 0

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 792](../../rfc792/) (cycle with rfc791) | Internet Control Message Protocol | Ze proven 6, Ze correctness gap 4 | Ze MUST proven 6, SHOULD not decided 4, Ze correctness gap 4 |
| [RFC 905](../../rfc905/) | ISO Transport Protocol Specification (ISO DP 8073) | Ze proven 8, Ze correctness gap 3 | Ze MUST proven 8, Ze correctness gap 3 |
| [RFC 1071](../../rfc1071/) | Computing the Internet Checksum | Ze proven 6 | Ze MUST proven 6, SHOULD not decided 3 |

### Level 1

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 4787](../../rfc4787/) | Network Address Translation (NAT) Behavioral Requirements for Unicast UDP | Ze correctness gap 13 | SHOULD not decided 16, Ze correctness gap 13 |

### Level 3

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 3022](../../rfc3022/) | Traditional IP Network Address Translator (Traditional NAT) | no rows | no rows |
| [RFC 5382](../../rfc5382/) | NAT Behavioral Requirements for TCP | Ze correctness gap 11 | SHOULD not decided 11, Ze correctness gap 11 |
