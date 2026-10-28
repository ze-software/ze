# BGP session operations

Technology aggregate. BGP session operations: graceful restart, shutdown, route reflection, route servers, and AS migration. The RFCs whose summary names the bgp-session-ops aggregate, with their mandatory and optional coverage.

[All profiles](../) · [All RFCs](../../)

Use case: [Chaos-Tested BGP Peering](../../../../use-cases/chaos-tested-peering/)

- **Mandatory**: 50.0% (38 of 76 (50.0%)). MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that a recorded test break proves.
- **Optional**: 3.3% (4 of 122 (3.3%)). MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows, counted the same way.
- **Mandatory ceiling**: 96.1% (73 of 76 (96.1%); excluded: Not applicable (classification can be revisited): 3 of 76 (3.9%)). Attainable mandatory rows under current classifications, over the unchanged total.
- **Optional ceiling**: 100.0% (122 of 122 (100.0%)). Attainable optional rows under current classifications, over the unchanged total.
- **Not walked**: 3 (of 13 RFCs). RFCs with no valid extraction sign-off: their rows count in neither figure.
- **Mandatory register**: not complete (every row proven, none unwalked). A profile is complete only when it requires at least one RFC and none of them is unwalked.

Mandatory ceiling: 73 of 76 (96.1%); excluded: Not applicable (classification can be revisited): 3 of 76 (3.9%). Optional ceiling: 122 of 122 (100.0%). Lowest mandatory ceiling: rfc9494 23 of 25 (92.0%). Lowest optional ceiling: rfc4456 5 of 5 (100.0%). Ceilings describe current classifications, not an immutable Linux maximum. Not-applicable and feature-declined classifications can be revisited. Every gap remains work Ze owes.

## Tree

Roots first: an RFC sits one level above the highest level of the RFCs it builds on, and the RFCs of one citation cycle share a level. Each bar is the share of that figure's rows a recorded break proves; the ceiling below it shows attainable rows under current classifications. An RFC that is not walked shows an empty bar and no figure or ceiling.

### Level 4

| RFC | Title | Mandatory | Optional |
|---|---|---|---|
| [RFC 4456](../../rfc4456/) | BGP Route Reflection: An Alternative to Full Mesh Internal BGP (IBGP) | 1 of 4 (25.0%); ceiling 4 of 4 (100.0%) | 0 of 5 (0.0%); ceiling 5 of 5 (100.0%) |
| [RFC 5065](../../rfc5065/) | Autonomous System Confederations for BGP | not walked | not walked |

### Level 5

| RFC | Title | Mandatory | Optional |
|---|---|---|---|
| [RFC 7705](../../rfc7705/) | Autonomous System Migration Mechanisms and Their Effects on the BGP AS_PATH Attribute | 9 of 9 (100.0%); ceiling 9 of 9 (100.0%) | 0 of 8 (0.0%); ceiling 8 of 8 (100.0%) |
| [RFC 8326](../../rfc8326/) | Graceful BGP Session Shutdown | not walked | not walked |

### Level 6

| RFC | Title | Mandatory | Optional |
|---|---|---|---|
| [RFC 4724](../../rfc4724/) | Graceful Restart Mechanism for BGP | 14 of 26 (53.8%); ceiling 25 of 26 (96.2%); excluded: Not applicable (classification can be revisited): 1 of 26 (3.8%) | 1 of 5 (20.0%); ceiling 5 of 5 (100.0%) |
| [RFC 7947](../../rfc7947/) | Internet Exchange BGP Route Server | 1 of 2 (50.0%); ceiling 2 of 2 (100.0%) | 2 of 5 (40.0%); ceiling 5 of 5 (100.0%) |

### Level 7

| RFC | Title | Mandatory | Optional |
|---|---|---|---|
| [RFC 6996](../../rfc6996/) | Autonomous System (AS) Reservation for Private Use | 1 of 1 (100.0%); ceiling 1 of 1 (100.0%) | 0 of 2 (0.0%); ceiling 2 of 2 (100.0%) |
| [RFC 7948](../../rfc7948/) | Internet Exchange BGP Route Server Operations | no rows; ceiling no rows | 0 of 11 (0.0%); ceiling 11 of 11 (100.0%) |
| [RFC 8203](../../rfc8203/) | BGP Administrative Shutdown Communication | 3 of 5 (60.0%); ceiling 5 of 5 (100.0%) | 0 of 4 (0.0%); ceiling 4 of 4 (100.0%) |
| [RFC 9003](../../rfc9003/) | Extended BGP Administrative Shutdown Communication | 1 of 4 (25.0%); ceiling 4 of 4 (100.0%) | 0 of 7 (0.0%); ceiling 7 of 7 (100.0%) |

### Level 8

| RFC | Title | Mandatory | Optional |
|---|---|---|---|
| [RFC 7454](../../rfc7454/) | BGP Operations and Security | no rows; ceiling no rows | 0 of 64 (0.0%); ceiling 64 of 64 (100.0%) |
| [RFC 8538](../../rfc8538/) | Notification Message Support for BGP Graceful Restart | not walked | not walked |

### Level 9

| RFC | Title | Mandatory | Optional |
|---|---|---|---|
| [RFC 9494](../../rfc9494/) | Long-Lived Graceful Restart for BGP | 8 of 25 (32.0%); ceiling 23 of 25 (92.0%); excluded: Not applicable (classification can be revisited): 2 of 25 (8.0%) | 1 of 11 (9.1%); ceiling 11 of 11 (100.0%) |
