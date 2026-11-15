# Flow telemetry

Technology profile. Flow export: NetFlow v9 and IPFIX. The RFCs grouped under flow-telemetry, showing mandatory requirement coverage and overall coverage for the group and each RFC.

[All profiles](../) · [All RFCs](../../)

Figures as of the last build, read from [data/rfc/profiles/flow-telemetry.json](../../../../data/rfc/profiles/flow-telemetry.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze proven 17, Proof pending 6, Not built 43
- **Overview bar**: Ze MUST proven 17, Not owed 4, Proof pending 6, Not assessed 19, Not built 43

| Figure | Rows |
|---|---|
| RFCs | 3 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 70 |
| MUST rows kept (the MUST bar) | 66 |
| Ze proven | 17 |
| OS verified | 0 |
| Proof pending | 6 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 43 |
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
| SHOULD proof pending | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 19 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze proven means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Proof pending counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

## Tree

The RFCs are listed with the standards they build on first. An RFC appears one level after the highest level of its dependencies; RFCs that refer back to each other share a level. Each RFC has the same two coverage bars as the group above. An RFC whose requirement list has not been reviewed shows "requirements not reviewed" instead of a bar.

### Level 0

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 3954](../../rfc3954/) | Cisco Systems NetFlow Services Export Version 9 | Ze proven 4, Not built 6 | Ze MUST proven 4, Not owed 4, Not assessed 12, Not built 6 |

### Level 5

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 7011](../../rfc7011/) (cycle with rfc7012) | Specification of the IP Flow Information Export (IPFIX) Protocol for the Exchange of Flow Information | Ze proven 12, Proof pending 6, Not built 32 | Ze MUST proven 12, Proof pending 6, Not assessed 7, Not built 32 |
| [RFC 7012](../../rfc7012/) (cycle with rfc7011) | Information Model for IP Flow Information Export (IPFIX) | Ze proven 1, Not built 5 | Ze MUST proven 1, Not built 5 |
