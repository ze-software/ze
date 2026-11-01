# BGP core

Technology aggregate. BGP base protocol, capabilities, attributes, and error handling. The RFCs whose summary names the bgp-core aggregate, with the MUST bar and the overview bar of the set and of each RFC.

[All profiles](../) · [All RFCs](../../)

Use case: [BGP performance testing with Ze](../../../../use-cases/bgp-performance/)

Use case: [ExaBGP Migration](../../../../use-cases/exabgp-migration/)

Figures as of the last build, read from [data/rfc/profiles/bgp-core.json](../../../../data/rfc/profiles/bgp-core.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze proven 205, Ze correctness gap 142
- **Overview bar**: Ze MUST proven 205, SHOULD proven 8, Not owed 1, SHOULD not decided 106, Ze correctness gap 142

| Figure | Rows |
|---|---|
| RFCs | 21 |
| Not walked | 0 |
| Mandatory register | not complete |
| MUST rows | 348 |
| MUST rows kept (the MUST bar) | 347 |
| Ze proven | 205 |
| OS verified | 0 |
| OS testing gap | 0 |
| Ze correctness gap | 142 |
| MUST rows not owed | 1 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 1 |
| SHOULD / MAY rows | 114 |
| SHOULD proven | 8 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 106 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

## Tree

Roots first: an RFC sits one level above the highest level of the RFCs it builds on, and the RFCs of one citation cycle share a level. Each RFC carries the same two bars as the set above, from its own data file; an RFC that is not walked shows "not walked" and no bar.

### Level 3

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 4271](../../rfc4271/) | A Border Gateway Protocol 4 (BGP-4) | Ze proven 83, Ze correctness gap 50 | Ze MUST proven 83, SHOULD proven 3, SHOULD not decided 31, Ze correctness gap 50 |

### Level 4

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 2545](../../rfc2545/) | Use of BGP-4 Multiprotocol Extensions for IPv6 Inter-Domain Routing | Ze proven 3, Ze correctness gap 2 | Ze MUST proven 3, SHOULD not decided 2, Ze correctness gap 2 |
| [RFC 5492](../../rfc5492/) | Capabilities Advertisement with BGP-4 | Ze proven 8, Ze correctness gap 1 | Ze MUST proven 8, SHOULD not decided 7, Ze correctness gap 1 |
| [RFC 6608](../../rfc6608/) | Subcodes for BGP Finite State Machine Error | Ze correctness gap 3 | Ze correctness gap 3 |
| [RFC 8212](../../rfc8212/) | Default External BGP (EBGP) Route Propagation Behavior without Policies | Ze proven 2 | Ze MUST proven 2, SHOULD not decided 1 |
| [RFC 9072](../../rfc9072/) | Extended Optional Parameters Length for BGP OPEN Message | Ze proven 5, Ze correctness gap 4 | Ze MUST proven 5, SHOULD proven 1, SHOULD not decided 2, Ze correctness gap 4 |
| [RFC 9687](../../rfc9687/) | Border Gateway Protocol 4 (BGP-4) Send Hold Timer | Ze proven 3, Ze correctness gap 10 | Ze MUST proven 3, SHOULD not decided 7, Ze correctness gap 10 |

### Level 5

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 4760](../../rfc4760/) | Multiprotocol Extensions for BGP-4 | Ze proven 6 | Ze MUST proven 6, SHOULD proven 1, SHOULD not decided 10 |

### Level 6

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 2918](../../rfc2918/) | Route Refresh Capability for BGP-4 | Ze proven 3, Ze correctness gap 3 | Ze MUST proven 3, SHOULD proven 1, SHOULD not decided 4, Ze correctness gap 3 |
| [RFC 4486](../../rfc4486/) | Subcodes for BGP Cease NOTIFICATION Message | Ze correctness gap 1 | SHOULD not decided 10, Ze correctness gap 1 |
| [RFC 6793](../../rfc6793/) (cycle with rfc5668) | BGP Support for Four-Octet Autonomous System (AS) Number Space | Ze proven 20, Ze correctness gap 10 | Ze MUST proven 20, SHOULD not decided 6, Ze correctness gap 10 |
| [RFC 7311](../../rfc7311/) | The Accumulated IGP Metric Attribute for BGP | Ze proven 13, Ze correctness gap 9 | Ze MUST proven 13, SHOULD not decided 4, Ze correctness gap 9 |
| [RFC 7911](../../rfc7911/) | Advertisement of Multiple Paths in BGP | Ze proven 9 | Ze MUST proven 9, SHOULD not decided 4 |

### Level 7

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 6286](../../rfc6286/) | Autonomous-System-Wide Unique BGP Identifier for BGP-4 | Ze correctness gap 4 | SHOULD proven 1, Ze correctness gap 4 |
| [RFC 7313](../../rfc7313/) | Enhanced Route Refresh Capability for BGP-4 | Ze proven 5, Ze correctness gap 6 | Ze MUST proven 5, SHOULD not decided 6, Ze correctness gap 6 |
| [RFC 7606](../../rfc7606/) (cycle with rfc4364, rfc4659, rfc4798, rfc6368, rfc8277, rfc8950) | Revised Error Handling for BGP UPDATE Messages | Ze proven 28, Ze correctness gap 25 | Ze MUST proven 28, SHOULD not decided 4, Ze correctness gap 25 |
| [RFC 8950](../../rfc8950/) (cycle with rfc4364, rfc4659, rfc4798, rfc6368, rfc7606, rfc8277) | Advertising IPv4 Network Layer Reachability Information (NLRI) with an IPv6 Next Hop | Ze proven 3, Ze correctness gap 3 | Ze MUST proven 3, SHOULD not decided 2, Ze correctness gap 3 |

### Level 8

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 5549](../../rfc5549/) | Advertising IPv4 Network Layer Reachability Information with an IPv6 Next Hop | Ze proven 3, Ze correctness gap 3 | Ze MUST proven 3, SHOULD not decided 3, Ze correctness gap 3 |
| [RFC 7607](../../rfc7607/) | Codification of AS 0 Processing | Ze correctness gap 5 | Ze correctness gap 5 |
| [RFC 8654](../../rfc8654/) | Extended Message Support for BGP | Ze proven 9, Ze correctness gap 2 | Ze MUST proven 9, SHOULD proven 1, Not owed 1, SHOULD not decided 3, Ze correctness gap 2 |
| [RFC 9774](../../rfc9774/) | Deprecation of AS_SET and AS_CONFED_SET in BGP | Ze proven 2, Ze correctness gap 1 | Ze MUST proven 2, Ze correctness gap 1 |
