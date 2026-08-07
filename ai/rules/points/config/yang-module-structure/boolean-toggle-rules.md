---
kind: table
level:
stage:
---
| Rule | Detail |
|------|--------|
| Positive assertion, one word | `enabled`, not `enable`, `disable`, or `disabled` (see YANG Leaves, positive boolean). |
| Standard admin-state words are the only exception | `shutdown` (BFD, RFC 5880 §6.8.16) and interface `disable` (kernel admin-down) are allowed because they are the canonical protocol/kernel terms, but type them as `boolean` with `default false`, never `type empty`. |
| No boolean-as-enum | Do not model a two-value on/off as `enumeration { enum enable; enum disable; }`. If config inheritance genuinely needs a distinct unset state, justify that tri-state in the module: it is an exception, not the default. |
| Bare flag | For "this section is on when present", use a `presence` container. Do not use a `type empty` leaf. |
