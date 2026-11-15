# Network primitives

Technology profile. Network primitives other protocols build on. The RFCs grouped under primitives, showing mandatory requirement coverage and overall coverage for the group and each RFC.

[All profiles](../) · [All RFCs](../../)

Figures as of the last build, read from [data/rfc/profiles/primitives.json](../../../../data/rfc/profiles/primitives.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze proven 20, Proof pending 4, Not built 27
- **Overview bar**: Ze MUST proven 20, SHOULD proven 2, Proof pending 4, Not assessed 32, Not built 27

| Figure | Rows |
|---|---|
| RFCs | 6 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 51 |
| MUST rows kept (the MUST bar) | 51 |
| Ze proven | 20 |
| OS verified | 0 |
| Proof pending | 4 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 27 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 34 |
| SHOULD proven | 2 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD proof pending | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 32 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze proven means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Proof pending counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

## Tree

The RFCs are listed with the standards they build on first. An RFC appears one level after the highest level of its dependencies; RFCs that refer back to each other share a level. Each RFC has the same two coverage bars as the group above. An RFC whose requirement list has not been reviewed shows "requirements not reviewed" instead of a bar.

### Level 0

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 792](../../rfc792/) (cycle with rfc791) | Internet Control Message Protocol | Ze proven 6, Proof pending 4 | Ze MUST proven 6, Proof pending 4, Not assessed 4 |
| [RFC 905](../../rfc905/) | ISO Transport Protocol Specification (ISO DP 8073) | Ze proven 8, Not built 3 | Ze MUST proven 8, Not built 3 |
| [RFC 1071](../../rfc1071/) | Computing the Internet Checksum | Ze proven 6 | Ze MUST proven 6, Not assessed 3 |

### Level 1

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 4787](../../rfc4787/) | Network Address Translation (NAT) Behavioral Requirements for Unicast UDP | Not built 13 | SHOULD proven 2, Not assessed 14, Not built 13 |

### Level 3

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 3022](../../rfc3022/) | Traditional IP Network Address Translator (Traditional NAT) | no rows | no rows |
| [RFC 5382](../../rfc5382/) | NAT Behavioral Requirements for TCP | Not built 11 | Not assessed 11, Not built 11 |
