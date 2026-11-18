# RADIUS and TACACS+

Technology profile. RADIUS and TACACS+ authentication, authorization, and accounting. The RFCs grouped under radius-tacacs, showing mandatory requirement coverage and overall coverage for the group and each RFC.

[All profiles](../) · [All RFCs](../../)

Figures as of the last build, read from [data/rfc/profiles/radius-tacacs.json](../../../../data/rfc/profiles/radius-tacacs.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze verified 103, Tests awaiting verification 49, Exclusion unchecked 2, Not assessed 56, Not built 23
- **Overview bar**: Ze MUST verified 103, SHOULD verified 5, Tests awaiting verification 49, Exclusion unchecked 2, Not assessed 190, Not built 23

| Figure | Rows |
|---|---|
| RFCs | 12 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 233 |
| MUST rows kept (the MUST bar) | 233 |
| Ze verified | 103 |
| OS verified | 0 |
| Tests awaiting verification | 49 |
| OS testing gap | 0 |
| Exclusion unchecked | 2 |
| Not assessed | 56 |
| Not built | 23 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 139 |
| SHOULD verified | 5 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 134 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

## Tree

The RFCs are listed with the standards they build on first. An RFC appears one level after the highest level of its dependencies; RFCs that refer back to each other share a level. Each RFC has the same two coverage bars as the group above. An RFC whose requirement list has not been reviewed shows "requirements not reviewed" instead of a bar.

### Level 1

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 2865](../../rfc2865/) (cycle with rfc2866) | Remote Authentication Dial In User Service (RADIUS) | Ze verified 27, Tests awaiting verification 4 | Ze MUST verified 27, Tests awaiting verification 4, Not assessed 1 |
| [RFC 2866](../../rfc2866/) (cycle with rfc2865) | RADIUS Accounting | Ze verified 7, Tests awaiting verification 8 | Ze MUST verified 7, Tests awaiting verification 8, Not assessed 1 |

### Level 3

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 2548](../../rfc2548/) | Microsoft Vendor-specific RADIUS Attributes | Not assessed 31 | SHOULD verified 2, Not assessed 63 |

### Level 4

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 6911](../../rfc6911/) | RADIUS Attributes for IPv6 Access Networks | Not assessed 3 | Not assessed 12 |

### Level 5

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 4818](../../rfc4818/) | RADIUS Delegated-IPv6-Prefix Attribute | Not assessed 3 | Not assessed 6 |

### Level 6

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 2867](../../rfc2867/) (cycle with rfc2868, rfc2869) | RADIUS Accounting Modifications for Tunnel Protocol Support | Not assessed 1 | Not assessed 17 |
| [RFC 2868](../../rfc2868/) (cycle with rfc2867, rfc2869) | RADIUS Attributes for Tunnel Protocol Support | Not assessed 18 | Not assessed 61 |
| [RFC 2869](../../rfc2869/) (cycle with rfc2867, rfc2868) | RADIUS Extensions | Ze verified 6, Tests awaiting verification 2 | Ze MUST verified 6, Tests awaiting verification 2, Not assessed 3 |

### Level 7

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 3162](../../rfc3162/) | RADIUS and IPv6 | Ze verified 2, Tests awaiting verification 1, Not built 3 | Ze MUST verified 2, Tests awaiting verification 1, Not assessed 7, Not built 3 |

### Level 8

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 3579](../../rfc3579/) (cycle with rfc5176) | RADIUS (Remote Authentication Dial In User Service) Support For Extensible Authentication Protocol (EAP) | Ze verified 17, Not built 14 | Ze MUST verified 17, SHOULD verified 2, Not assessed 17, Not built 14 |
| [RFC 5176](../../rfc5176/) (cycle with rfc3579) | Dynamic Authorization Extensions to Remote Authentication Dial In User Service (RADIUS) | Ze verified 6, Tests awaiting verification 24 | Ze MUST verified 6, SHOULD verified 1, Tests awaiting verification 24 |

### Level 9

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 8907](../../rfc8907/) | The Terminal Access Controller Access-Control System Plus (TACACS+) Protocol | Ze verified 38, Tests awaiting verification 10, Exclusion unchecked 2, Not built 6 | Ze MUST verified 38, Tests awaiting verification 10, Exclusion unchecked 2, Not assessed 2, Not built 6 |
