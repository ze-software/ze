# Flow telemetry

Technology aggregate. Flow export: NetFlow v9 and IPFIX. The RFCs whose summary names the flow-telemetry aggregate, with the MUST bar and the overview bar of the set and of each RFC.

[All profiles](../) · [All RFCs](../../)

Figures as of the last build, read from [data/rfc/profiles/flow-telemetry.json](../../../../data/rfc/profiles/flow-telemetry.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze proven 17, Ze correctness gap 49
- **Overview bar**: Ze MUST proven 17, Not owed 4, SHOULD not decided 19, Ze correctness gap 49

| Figure | Rows |
|---|---|
| RFCs | 3 |
| Not walked | 0 |
| Mandatory register | not complete |
| MUST rows | 70 |
| MUST rows kept (the MUST bar) | 66 |
| Ze proven | 17 |
| OS verified | 0 |
| OS testing gap | 0 |
| Ze correctness gap | 49 |
| MUST rows not owed | 4 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 4 |
| SHOULD / MAY rows | 19 |
| SHOULD proven | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 19 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

## Tree

Roots first: an RFC sits one level above the highest level of the RFCs it builds on, and the RFCs of one citation cycle share a level. Each RFC carries the same two bars as the set above, from its own data file; an RFC that is not walked shows "not walked" and no bar.

### Level 0

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 3954](../../rfc3954/) | Cisco Systems NetFlow Services Export Version 9 | Ze proven 4, Ze correctness gap 6 | Ze MUST proven 4, Not owed 4, SHOULD not decided 12, Ze correctness gap 6 |

### Level 5

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 7011](../../rfc7011/) (cycle with rfc7012) | Specification of the IP Flow Information Export (IPFIX) Protocol for the Exchange of Flow Information | Ze proven 12, Ze correctness gap 38 | Ze MUST proven 12, SHOULD not decided 7, Ze correctness gap 38 |
| [RFC 7012](../../rfc7012/) (cycle with rfc7011) | Information Model for IP Flow Information Export (IPFIX) | Ze proven 1, Ze correctness gap 5 | Ze MUST proven 1, Ze correctness gap 5 |
