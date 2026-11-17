# BGP core

Technology profile. BGP base protocol, capabilities, attributes, and error handling. The RFCs grouped under bgp-core, showing mandatory requirement coverage and overall coverage for the group and each RFC.

[All profiles](../) · [All RFCs](../../)

Use case: [BGP performance testing with Ze](../../../../use-cases/bgp-performance/)

Use case: [ExaBGP Migration](../../../../use-cases/exabgp-migration/)

Figures as of the last build, read from [data/rfc/profiles/bgp-core.json](../../../../data/rfc/profiles/bgp-core.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze verified 240, Tests awaiting verification 76, Not built 31
- **Overview bar**: Ze MUST verified 240, SHOULD verified 8, Not owed 1, Tests awaiting verification 78, Not assessed 104, Not built 31

| Figure | Rows |
|---|---|
| RFCs | 21 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 348 |
| MUST rows kept (the MUST bar) | 347 |
| Ze verified | 240 |
| OS verified | 0 |
| Tests awaiting verification | 76 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 31 |
| MUST rows not owed | 1 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 1 |
| SHOULD / MAY rows | 114 |
| SHOULD verified | 8 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 2 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 104 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

## Tree

The RFCs are listed with the standards they build on first. An RFC appears one level after the highest level of its dependencies; RFCs that refer back to each other share a level. Each RFC has the same two coverage bars as the group above. An RFC whose requirement list has not been reviewed shows "requirements not reviewed" instead of a bar.

### Level 3

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 4271](../../rfc4271/) | A Border Gateway Protocol 4 (BGP-4) | Ze verified 87, Tests awaiting verification 31, Not built 15 | Ze MUST verified 87, SHOULD verified 3, Tests awaiting verification 32, Not assessed 30, Not built 15 |

### Level 4

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 2545](../../rfc2545/) | Use of BGP-4 Multiprotocol Extensions for IPv6 Inter-Domain Routing | Ze verified 3, Not built 2 | Ze MUST verified 3, Not assessed 2, Not built 2 |
| [RFC 5492](../../rfc5492/) | Capabilities Advertisement with BGP-4 | Ze verified 8, Not built 1 | Ze MUST verified 8, Not assessed 7, Not built 1 |
| [RFC 6608](../../rfc6608/) | Subcodes for BGP Finite State Machine Error | Ze verified 3 | Ze MUST verified 3 |
| [RFC 8212](../../rfc8212/) | Default External BGP (EBGP) Route Propagation Behavior without Policies | Ze verified 2 | Ze MUST verified 2, Not assessed 1 |
| [RFC 9072](../../rfc9072/) | Extended Optional Parameters Length for BGP OPEN Message | Ze verified 5, Not built 4 | Ze MUST verified 5, SHOULD verified 1, Not assessed 2, Not built 4 |
| [RFC 9687](../../rfc9687/) | Border Gateway Protocol 4 (BGP-4) Send Hold Timer | Ze verified 13 | Ze MUST verified 13, Not assessed 7 |

### Level 5

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 4760](../../rfc4760/) | Multiprotocol Extensions for BGP-4 | Ze verified 5, Tests awaiting verification 1 | Ze MUST verified 5, SHOULD verified 1, Tests awaiting verification 1, Not assessed 10 |

### Level 6

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 2918](../../rfc2918/) | Route Refresh Capability for BGP-4 | Ze verified 4, Tests awaiting verification 2 | Ze MUST verified 4, SHOULD verified 1, Tests awaiting verification 2, Not assessed 4 |
| [RFC 4486](../../rfc4486/) | Subcodes for BGP Cease NOTIFICATION Message | Tests awaiting verification 1 | Tests awaiting verification 2, Not assessed 9 |
| [RFC 6793](../../rfc6793/) (cycle with rfc5668) | BGP Support for Four-Octet Autonomous System (AS) Number Space | Ze verified 30 | Ze MUST verified 30, Not assessed 6 |
| [RFC 7311](../../rfc7311/) | The Accumulated IGP Metric Attribute for BGP | Ze verified 9, Tests awaiting verification 13 | Ze MUST verified 9, Tests awaiting verification 13, Not assessed 4 |
| [RFC 7911](../../rfc7911/) | Advertisement of Multiple Paths in BGP | Ze verified 9 | Ze MUST verified 9, Not assessed 4 |

### Level 7

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 6286](../../rfc6286/) | Autonomous-System-Wide Unique BGP Identifier for BGP-4 | Ze verified 4 | Ze MUST verified 4, SHOULD verified 1 |
| [RFC 7606](../../rfc7606/) (cycle with rfc4364, rfc4659, rfc4798, rfc6368, rfc8277, rfc8950) | Revised Error Handling for BGP UPDATE Messages | Ze verified 33, Tests awaiting verification 19, Not built 1 | Ze MUST verified 33, Tests awaiting verification 19, Not assessed 4, Not built 1 |
| [RFC 8950](../../rfc8950/) (cycle with rfc4364, rfc4659, rfc4798, rfc6368, rfc7606, rfc8277) | Advertising IPv4 Network Layer Reachability Information (NLRI) with an IPv6 Next Hop | Ze verified 2, Tests awaiting verification 4 | Ze MUST verified 2, Tests awaiting verification 4, Not assessed 2 |

### Level 8

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 5549](../../rfc5549/) | Advertising IPv4 Network Layer Reachability Information with an IPv6 Next Hop | Ze verified 3, Tests awaiting verification 3 | Ze MUST verified 3, Tests awaiting verification 3, Not assessed 3 |
| [RFC 7313](../../rfc7313/) | Enhanced Route Refresh Capability for BGP-4 | Ze verified 5, Tests awaiting verification 1, Not built 5 | Ze MUST verified 5, Tests awaiting verification 1, Not assessed 6, Not built 5 |
| [RFC 7607](../../rfc7607/) | Codification of AS 0 Processing | Ze verified 5 | Ze MUST verified 5 |
| [RFC 8654](../../rfc8654/) | Extended Message Support for BGP | Ze verified 8, Tests awaiting verification 1, Not built 2 | Ze MUST verified 8, SHOULD verified 1, Not owed 1, Tests awaiting verification 1, Not assessed 3, Not built 2 |
| [RFC 9774](../../rfc9774/) | Deprecation of AS_SET and AS_CONFED_SET in BGP | Ze verified 2, Not built 1 | Ze MUST verified 2, Not built 1 |
