# Tunnels

Technology aggregate. IP-in-IP, GRE, and IPv6 transition tunnels. The RFCs whose summary names the tunnels aggregate, with the MUST bar and the overview bar of the set and of each RFC.

[All profiles](../) · [All RFCs](../../)

Figures as of the last build, read from [data/rfc/profiles/tunnels.json](../../../../data/rfc/profiles/tunnels.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze proven 2, OS testing gap 24, Ze correctness gap 35
- **Overview bar**: Ze MUST proven 2, SHOULD not decided 64, OS testing gap 24, Ze correctness gap 35

| Figure | Rows |
|---|---|
| RFCs | 5 |
| Not walked | 0 |
| Mandatory register | not complete |
| MUST rows | 61 |
| MUST rows kept (the MUST bar) | 61 |
| Ze proven | 2 |
| OS verified | 0 |
| OS testing gap | 24 |
| Ze correctness gap | 35 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 64 |
| SHOULD proven | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 64 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

## Tree

Roots first: an RFC sits one level above the highest level of the RFCs it builds on, and the RFCs of one citation cycle share a level. Each RFC carries the same two bars as the set above, from its own data file; an RFC that is not walked shows "not walked" and no bar.

### Level 3

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 2003](../../rfc2003/) | IP Encapsulation within IP | Ze correctness gap 13 | SHOULD not decided 24, Ze correctness gap 13 |
| [RFC 2784](../../rfc2784/) | Generic Routing Encapsulation (GRE) | Ze correctness gap 11 | SHOULD not decided 2, Ze correctness gap 11 |
| [RFC 4213](../../rfc4213/) | Basic Transition Mechanisms for IPv6 Hosts and Routers | Ze proven 1, OS testing gap 17, Ze correctness gap 5 | Ze MUST proven 1, SHOULD not decided 24, OS testing gap 17, Ze correctness gap 5 |

### Level 5

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 2473](../../rfc2473/) | Generic Packet Tunneling in IPv6 Specification | Ze proven 1, OS testing gap 7, Ze correctness gap 3 | Ze MUST proven 1, SHOULD not decided 10, OS testing gap 7, Ze correctness gap 3 |
| [RFC 2890](../../rfc2890/) | Key and Sequence Number Extensions to GRE | Ze correctness gap 3 | SHOULD not decided 4, Ze correctness gap 3 |
