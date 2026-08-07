---
kind: table
level:
stage:
---
| Anti-pattern | Why it fails the removal test |
|--------------|-------------------------------|
| Plugin command spelling in generic dispatch (`internal/component/plugin/server`) | Deleting the plugin leaves dead BGP/iface knowledge in shared code |
| A plugin's subtree in a central verb schema (e.g. `show bgp ...` in `internal/component/cmd/show/yang/ze-cli-show-cmd.yang`) | Deleting the plugin leaves a `show bgp` branch with no handler |
| Plugin handlers registered from a central verb package (`internal/component/cmd/show`, `internal/component/cmd/delete`, ...) | Deleting the plugin leaves the central package referencing gone symbols |
| Help / usage / inventory strings that hardcode a plugin's commands in a generic package | Deleting the plugin leaves help advertising commands that no longer exist |
| The CLI helper (`cmd/ze/internal/cmdutil`) special-casing a plugin's selectors | Selector handling is generic; per-plugin knowledge belongs to the owner |
