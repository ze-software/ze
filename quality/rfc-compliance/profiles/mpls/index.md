# MPLS

Technology aggregate. MPLS, LDP, RSVP-TE, and fast reroute. The RFCs whose summary names the mpls aggregate, with the MUST bar and the overview bar of the set and of each RFC.

[All profiles](../) · [All RFCs](../../)

Figures as of the last build, read from [data/rfc/profiles/mpls.json](../../../../data/rfc/profiles/mpls.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze proven 125, OS testing gap 7, Ze correctness gap 187
- **Overview bar**: Ze MUST proven 125, SHOULD proven 6, SHOULD not decided 53, OS testing gap 7, Ze correctness gap 187

| Figure | Rows |
|---|---|
| RFCs | 10 |
| Not walked | 0 |
| Mandatory register | not complete |
| MUST rows | 319 |
| MUST rows kept (the MUST bar) | 319 |
| Ze proven | 125 |
| OS verified | 0 |
| OS testing gap | 7 |
| Ze correctness gap | 187 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 59 |
| SHOULD proven | 6 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 53 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

## Tree

Roots first: an RFC sits one level above the highest level of the RFCs it builds on, and the RFCs of one citation cycle share a level. Each RFC carries the same two bars as the set above, from its own data file; an RFC that is not walked shows "not walked" and no bar.

### Level 0

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 2205](../../rfc2205/) (cycle with rfc2210, rfc2211, rfc2215) | Resource ReSerVation Protocol (RSVP) -- Version 1 Functional Specification | Ze proven 31, Ze correctness gap 44 | Ze MUST proven 31, SHOULD not decided 5, Ze correctness gap 44 |

### Level 1

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 5036](../../rfc5036/) | LDP Specification | Ze proven 29, Ze correctness gap 62 | Ze MUST proven 29, SHOULD not decided 4, Ze correctness gap 62 |

### Level 2

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 5561](../../rfc5561/) | LDP Capabilities | Ze proven 1, Ze correctness gap 16 | Ze MUST proven 1, SHOULD not decided 1, Ze correctness gap 16 |

### Level 3

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 3031](../../rfc3031/) (cycle with rfc3032) | Multiprotocol Label Switching Architecture | Ze proven 2, Ze correctness gap 8 | Ze MUST proven 2, SHOULD not decided 2, Ze correctness gap 8 |
| [RFC 3032](../../rfc3032/) (cycle with rfc3031) | MPLS Label Stack Encoding | Ze proven 1, OS testing gap 5, Ze correctness gap 12 | Ze MUST proven 1, SHOULD not decided 6, OS testing gap 5, Ze correctness gap 12 |

### Level 4

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 3209](../../rfc3209/) | RSVP-TE: Extensions to RSVP for LSP Tunnels | Ze proven 23, OS testing gap 2, Ze correctness gap 34 | Ze MUST proven 23, SHOULD not decided 3, OS testing gap 2, Ze correctness gap 34 |

### Level 5

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 4090](../../rfc4090/) | Fast Reroute Extensions to RSVP-TE for LSP Tunnels | Ze proven 29, Ze correctness gap 4 | Ze MUST proven 29, SHOULD proven 6, SHOULD not decided 2, Ze correctness gap 4 |
| [RFC 5443](../../rfc5443/) | LDP IGP Synchronization | Ze proven 5, Ze correctness gap 3 | Ze MUST proven 5, SHOULD not decided 6, Ze correctness gap 3 |

### Level 6

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 6138](../../rfc6138/) | LDP IGP Synchronization for Broadcast Networks | Ze proven 2 | Ze MUST proven 2 |

### Level 10

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 5286](../../rfc5286/) | Basic Specification for IP Fast Reroute: Loop-Free Alternates | Ze proven 2, Ze correctness gap 4 | Ze MUST proven 2, SHOULD not decided 24, Ze correctness gap 4 |
