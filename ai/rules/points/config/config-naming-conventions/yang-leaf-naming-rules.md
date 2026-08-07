---
kind: table
level:
stage:
---
| Rule | Example | Anti-pattern |
|------|---------|-------------|
| kebab-case, no abbreviations | `forward-queue-size` | `fwd-chan-size` |
| Noun or noun-phrase | `read-buffer-size` | `read-buf-sz` |
| Dimensioned value: state the unit via a YANG `units` statement, keep the name unit-free (see Units) | `teardown-grace` + `units seconds;` | `teardown-grace-seconds` (unit in the name), `teardown-grace` with no `units` |
| No `ze-` prefix (implicit in the tree) | `cache-ttl` | `ze-cache-ttl` |
| Boolean: positive assertion | `update-groups` | `no-update-groups`, `disable-update-groups` |
