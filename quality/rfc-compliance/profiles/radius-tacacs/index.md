# RADIUS and TACACS+

Technology aggregate. RADIUS and TACACS+ authentication, authorization, and accounting. The RFCs whose summary names the radius-tacacs aggregate, with the MUST bar and the overview bar of the set and of each RFC.

[All profiles](../) · [All RFCs](../../)

Figures as of the last build, read from [data/rfc/profiles/radius-tacacs.json](../../../../data/rfc/profiles/radius-tacacs.json) and [data/rfc/index.json](../../../../data/rfc/index.json).

## Figures

- **MUST bar**: Ze proven 123, Ze correctness gap 79
- **Overview bar**: Ze MUST proven 123, SHOULD proven 3, SHOULD not decided 102, Ze correctness gap 79

| Figure | Rows |
|---|---|
| RFCs | 11 |
| Not walked | 0 |
| Mandatory register | not complete |
| MUST rows | 202 |
| MUST rows kept (the MUST bar) | 202 |
| Ze proven | 123 |
| OS verified | 0 |
| OS testing gap | 0 |
| Ze correctness gap | 79 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 105 |
| SHOULD proven | 3 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 102 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

## Tree

Roots first: an RFC sits one level above the highest level of the RFCs it builds on, and the RFCs of one citation cycle share a level. Each RFC carries the same two bars as the set above, from its own data file; an RFC that is not walked shows "not walked" and no bar.

### Level 0

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 2865](../../rfc2865/) (cycle with rfc2866) | Remote Authentication Dial In User Service (RADIUS) | Ze proven 27, Ze correctness gap 4 | Ze MUST proven 27, SHOULD not decided 1, Ze correctness gap 4 |
| [RFC 2866](../../rfc2866/) (cycle with rfc2865) | RADIUS Accounting | Ze proven 8, Ze correctness gap 7 | Ze MUST proven 8, SHOULD not decided 1, Ze correctness gap 7 |

### Level 4

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 6911](../../rfc6911/) | RADIUS Attributes for IPv6 Access Networks | Ze correctness gap 3 | SHOULD not decided 9, Ze correctness gap 3 |

### Level 5

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 4818](../../rfc4818/) | RADIUS Delegated-IPv6-Prefix Attribute | Ze correctness gap 3 | SHOULD not decided 3, Ze correctness gap 3 |

### Level 6

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 2867](../../rfc2867/) (cycle with rfc2868, rfc2869) | RADIUS Accounting Modifications for Tunnel Protocol Support | Ze correctness gap 1 | SHOULD not decided 16, Ze correctness gap 1 |
| [RFC 2868](../../rfc2868/) (cycle with rfc2867, rfc2869) | RADIUS Attributes for Tunnel Protocol Support | Ze correctness gap 18 | SHOULD not decided 43, Ze correctness gap 18 |
| [RFC 2869](../../rfc2869/) (cycle with rfc2867, rfc2868) | RADIUS Extensions | Ze proven 4, Ze correctness gap 4 | Ze MUST proven 4, SHOULD not decided 3, Ze correctness gap 4 |

### Level 7

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 3162](../../rfc3162/) | RADIUS and IPv6 | Ze proven 3, Ze correctness gap 3 | Ze MUST proven 3, SHOULD not decided 7, Ze correctness gap 3 |

### Level 8

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 3579](../../rfc3579/) (cycle with rfc5176) | RADIUS (Remote Authentication Dial In User Service) Support For Extensible Authentication Protocol (EAP) | Ze proven 17, Ze correctness gap 14 | Ze MUST proven 17, SHOULD proven 2, SHOULD not decided 17, Ze correctness gap 14 |
| [RFC 5176](../../rfc5176/) (cycle with rfc3579) | Dynamic Authorization Extensions to Remote Authentication Dial In User Service (RADIUS) | Ze proven 26, Ze correctness gap 4 | Ze MUST proven 26, SHOULD proven 1, Ze correctness gap 4 |

### Level 9

| RFC | Title | MUST bar | Overview bar |
|---|---|---|---|
| [RFC 8907](../../rfc8907/) | The Terminal Access Controller Access-Control System Plus (TACACS+) Protocol | Ze proven 38, Ze correctness gap 18 | Ze MUST proven 38, SHOULD not decided 2, Ze correctness gap 18 |
