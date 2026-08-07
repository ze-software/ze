---
kind: table
level: MUST
stage:
---
| Rule | Detail |
|------|--------|
| Indentation | 4 spaces per level. No tabs, no 2-space modules. |
| Compact leaf | A leaf whose body is only `type` (+ optional `default` and/or `description`) MAY be one line: `leaf med { type uint32; description "..."; }`. |
| Expanded leaf | A leaf with nested constraints (`pattern`, `range`, `enumeration`, `must`, multiple sub-statements) MUST be expanded, one statement per line. |
| List key | quoted: `key "name";`. Prefer `name` for the operator-assigned key. |
