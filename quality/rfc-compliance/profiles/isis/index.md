# IS-IS

Technology profile. IS-IS. The RFCs grouped under isis, showing mandatory requirement coverage and overall coverage for the group and each RFC.

[All profiles](../) · [All RFCs](../../)

Figures as of the last build, read from [data/rfc/profiles/isis.json](../../../../data/rfc/profiles/isis.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze proven 64, Proof pending 34, Exclusion unchecked 7, Not built 20
- **Overview bar**: Ze MUST proven 64, SHOULD proven 1, Proof pending 34, Exclusion unchecked 7, Not assessed 35, Not built 20

| Figure | Rows |
|---|---|
| RFCs | 11 |
| Requirements not reviewed | 1 |
| Mandatory requirements complete | not complete |
| MUST rows | 125 |
| MUST rows kept (the MUST bar) | 125 |
| Ze proven | 64 |
| OS verified | 0 |
| Proof pending | 34 |
| OS testing gap | 0 |
| Exclusion unchecked | 7 |
| Not assessed | 0 |
| Not built | 20 |
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
| SHOULD proof pending | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 35 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze proven means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Proof pending counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

## Tree

The RFCs are listed with the standards they build on first. An RFC appears one level after the highest level of its dependencies; RFCs that refer back to each other share a level. Each RFC has the same two coverage bars as the group above. An RFC whose requirement list has not been reviewed shows "requirements not reviewed" instead of a bar.

### Level 0

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 5301](../../rfc5301/) | Dynamic Hostname Exchange Mechanism for IS-IS | Proof pending 7 | Proof pending 7, Not assessed 8 |
| [RFC 5304](../../rfc5304/) | IS-IS Cryptographic Authentication | Proof pending 8, Not built 1 | Proof pending 8, Not assessed 5, Not built 1 |
| [RFC 5305](../../rfc5305/) | IS-IS Extensions for Traffic Engineering | Ze proven 8, Proof pending 1, Not built 5 | Ze MUST proven 8, Proof pending 1, Not assessed 8, Not built 5 |

### Level 2

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 1195](../../rfc1195/) (cycle with rfc1034, rfc1035, rfc1122, rfc1123, rfc1191, rfc1332, rfc1350, rfc1812, rfc2131, rfc2132, rfc4291, rfc4443, rfc4861, rfc5905, rfc8200, rfc8201, rfc9293) | Use of OSI IS-IS for Routing in TCP/IP and Dual Environments | Ze proven 25, Proof pending 9, Not built 7 | Ze MUST proven 25, Proof pending 9, Not assessed 1, Not built 7 |

### Level 3

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 2966](../../rfc2966/) | Domain-wide Prefix Distribution with Two-Level IS-IS | Ze proven 3, Proof pending 3 | Ze MUST proven 3, Proof pending 3, Not assessed 2 |
| [RFC 3786](../../rfc3786/) | Extending the Number of Intermediate System to Intermediate System (IS-IS) Link State PDU (LSP) Fragments Beyond the 256 Limit | Ze proven 1, Proof pending 1, Exclusion unchecked 7 | Ze MUST proven 1, Proof pending 1, Exclusion unchecked 7, Not assessed 4 |
| [RFC 3787](../../rfc3787/) | Recommendations for Interoperable IP Networks using Intermediate System to Intermediate System (IS-IS) | Ze proven 5 | Ze MUST proven 5, Not assessed 3 |
| [RFC 5303](../../rfc5303/) | Three-Way Handshake for IS-IS Point-to-Point Adjacencies | Ze proven 8, Proof pending 3, Not built 6 | Ze MUST proven 8, Proof pending 3, Not assessed 1, Not built 6 |
| [RFC 5308](../../rfc5308/) | Routing IPv6 with IS-IS | Ze proven 8 | Ze MUST proven 8, Not assessed 1 |
| [RFC 5310](../../rfc5310/) | IS-IS Generic Cryptographic Authentication | Ze proven 6, Proof pending 2, Not built 1 | Ze MUST proven 6, SHOULD proven 1, Proof pending 2, Not assessed 2, Not built 1 |

### No level: the repository has no copy of this RFC's text

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 5120](../../rfc5120/) | M-ISIS: Multi Topology (MT) Routing in Intermediate System to Intermediate Systems (IS-ISs) | requirements not reviewed | requirements not reviewed |
