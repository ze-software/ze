# Drafts and non-RFC standards

Technology aggregate. Drafts and non-RFC standards tracked near RFC work. The RFCs whose summary names the drafts aggregate, with the MUST bar and the overview bar of the set and of each RFC.

[All profiles](../) · [All RFCs](../../)

Figures as of the last build, read from [data/rfc/profiles/drafts.json](../../../../data/rfc/profiles/drafts.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze proven 74, OS testing gap 1, Ze correctness gap 46
- **Overview bar**: Ze MUST proven 74, SHOULD proven 6, SHOULD not decided 55, OS testing gap 1, Ze correctness gap 46

| Figure | Rows |
|---|---|
| RFCs | 8 |
| Not walked | 0 |
| Mandatory register | not complete |
| MUST rows | 121 |
| MUST rows kept (the MUST bar) | 121 |
| Ze proven | 74 |
| OS verified | 0 |
| OS testing gap | 1 |
| Ze correctness gap | 46 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 61 |
| SHOULD proven | 6 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 55 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

## Tree

Roots first: an RFC sits one level above the highest level of the RFCs it builds on, and the RFCs of one citation cycle share a level. Each RFC carries the same two bars as the set above, from its own data file; an RFC that is not walked shows "not walked" and no bar.

### Level 3

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [SFLOW-V5](../../sflow-v5/) | sFlow: A Method for Monitoring Traffic in Switched and Routed Networks | Ze proven 22, OS testing gap 1, Ze correctness gap 9 | Ze MUST proven 22, SHOULD proven 3, SHOULD not decided 8, OS testing gap 1, Ze correctness gap 9 |

### Level 5

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [DRAFT-WALTON-BGP-HOSTNAME-CAPABILITY](../../draft-walton-bgp-hostname-capability/) | Hostname Capability for BGP | no rows | SHOULD not decided 1 |

### Level 6

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY](../../draft-abraitis-bgp-version-capability/) | Software Version Capability for BGP | Ze proven 11 | Ze MUST proven 11, SHOULD not decided 4 |

### Level 7

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT](../../draft-abraitis-idr-addpath-paths-limit/) | Scalability Considerations for ADD-PATH with PATHS-LIMIT | Ze proven 1, Ze correctness gap 3 | Ze MUST proven 1, SHOULD proven 3, Ze correctness gap 3 |

### Level 8

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [DRAFT-IETF-BESS-MUP-SAFI](../../draft-ietf-bess-mup-safi/) | BGP Extensions for the Mobile User Plane (MUP) SAFI | Ze proven 17, Ze correctness gap 30 | Ze MUST proven 17, SHOULD not decided 20, Ze correctness gap 30 |

### Level 9

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY](../../draft-ietf-idr-linklocal-capability/) | Link-Local Next Hop Capability for BGP | Ze proven 13, Ze correctness gap 2 | Ze MUST proven 13, SHOULD not decided 12, Ze correctness gap 2 |
| [DRAFT-IETF-SIDROPS-ASPA-VERIFICATION](../../draft-ietf-sidrops-aspa-verification/) | BGP AS_PATH Verification Based on Autonomous System Provider Authorization (ASPA) Objects | Ze proven 6, Ze correctness gap 2 | Ze MUST proven 6, SHOULD not decided 10, Ze correctness gap 2 |

### Level 10

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE](../../draft-ietf-idr-bgp-bfd-strict-mode/) | BGP BFD Strict-Mode | Ze proven 4 | Ze MUST proven 4 |
