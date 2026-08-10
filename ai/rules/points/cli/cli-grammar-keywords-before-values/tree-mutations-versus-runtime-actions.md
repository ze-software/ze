---
kind: directive
level: MUST
stage:
---
- Interface addresses and units are config-tree mutations. They MUST belong under
  engine `set` / `delete`, not operational commands like `interface addr del`
  or `interface unit remove`.
- Runtime actions that are not tree mutation, such as `clear counters`,
  `teardown`, or `pause`, MAY have operational command verbs.
