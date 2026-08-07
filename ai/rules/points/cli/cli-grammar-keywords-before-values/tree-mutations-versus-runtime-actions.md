---
kind: directive
level:
stage:
---
- Interface addresses and units are config-tree mutations. They belong under
  engine `set` / `delete`, not operational commands like `interface addr del`
  or `interface unit remove`.
- Runtime actions that are not tree mutation, such as `clear counters`,
  `teardown`, or `pause`, may have operational command verbs.
