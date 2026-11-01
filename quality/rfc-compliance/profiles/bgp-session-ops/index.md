# BGP session operations

Technology aggregate. BGP session operations: graceful restart, shutdown, route reflection, route servers, and AS migration. The RFCs whose summary names the bgp-session-ops aggregate, with the MUST bar and the overview bar of the set and of each RFC.

[All profiles](../) · [All RFCs](../../)

Use case: [Chaos-Tested BGP Peering](../../../../use-cases/chaos-tested-peering/)

Figures as of the last build, read from [data/rfc/profiles/bgp-session-ops.json](../../../../data/rfc/profiles/bgp-session-ops.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze proven 39, Ze correctness gap 38
- **Overview bar**: Ze MUST proven 39, SHOULD proven 5, SHOULD not decided 117, Ze correctness gap 38

| Figure | Rows |
|---|---|
| RFCs | 13 |
| Not walked | 3 |
| Mandatory register | not complete |
| MUST rows | 77 |
| MUST rows kept (the MUST bar) | 77 |
| Ze proven | 39 |
| OS verified | 0 |
| OS testing gap | 0 |
| Ze correctness gap | 38 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 122 |
| SHOULD proven | 5 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 117 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

## Tree

Roots first: an RFC sits one level above the highest level of the RFCs it builds on, and the RFCs of one citation cycle share a level. Each RFC carries the same two bars as the set above, from its own data file; an RFC that is not walked shows "not walked" and no bar.

### Level 4

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 4456](../../rfc4456/) | BGP Route Reflection: An Alternative to Full Mesh Internal BGP (IBGP) | Ze proven 1, Ze correctness gap 3 | Ze MUST proven 1, SHOULD not decided 5, Ze correctness gap 3 |
| [RFC 5065](../../rfc5065/) | Autonomous System Confederations for BGP | not walked | not walked |

### Level 5

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 7705](../../rfc7705/) | Autonomous System Migration Mechanisms and Their Effects on the BGP AS_PATH Attribute | Ze proven 9 | Ze MUST proven 9, SHOULD not decided 8 |
| [RFC 8326](../../rfc8326/) | Graceful BGP Session Shutdown | not walked | not walked |

### Level 6

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 4724](../../rfc4724/) | Graceful Restart Mechanism for BGP | Ze proven 14, Ze correctness gap 12 | Ze MUST proven 14, SHOULD proven 1, SHOULD not decided 4, Ze correctness gap 12 |
| [RFC 7947](../../rfc7947/) | Internet Exchange BGP Route Server | Ze proven 2, Ze correctness gap 1 | Ze MUST proven 2, SHOULD proven 3, SHOULD not decided 2, Ze correctness gap 1 |

### Level 7

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 6996](../../rfc6996/) | Autonomous System (AS) Reservation for Private Use | Ze proven 1 | Ze MUST proven 1, SHOULD not decided 2 |
| [RFC 7948](../../rfc7948/) | Internet Exchange BGP Route Server Operations | no rows | SHOULD not decided 11 |
| [RFC 8203](../../rfc8203/) | BGP Administrative Shutdown Communication | Ze proven 3, Ze correctness gap 2 | Ze MUST proven 3, SHOULD not decided 4, Ze correctness gap 2 |
| [RFC 9003](../../rfc9003/) | Extended BGP Administrative Shutdown Communication | Ze proven 1, Ze correctness gap 3 | Ze MUST proven 1, SHOULD not decided 7, Ze correctness gap 3 |

### Level 8

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 7454](../../rfc7454/) | BGP Operations and Security | no rows | SHOULD not decided 64 |
| [RFC 8538](../../rfc8538/) | Notification Message Support for BGP Graceful Restart | not walked | not walked |

### Level 9

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 9494](../../rfc9494/) | Long-Lived Graceful Restart for BGP | Ze proven 8, Ze correctness gap 17 | Ze MUST proven 8, SHOULD proven 1, SHOULD not decided 10, Ze correctness gap 17 |
