---
kind: table
level:
stage:
---
| Rule | Canonical | Anti-pattern |
|------|-----------|--------------|
| Boolean default is unquoted | `default false;` | `default "false";` |
| enum `value N` only for wire numbers | assign `value` when the number is protocol-significant (AFI/SAFI/ORIGIN); otherwise omit | assigning arbitrary values to cosmetic enums |
