---
kind: directive
level:
stage:
---
1. **Grep for programmatic senders of the bare path:** `.ci` tests, `DispatchCommand`
   / `DispatchCommandArgs` calls, `printf ... | ze ... plugin cli`, `SendCommand`. Update
   every one in the same change. (In-tree these are greppable; external plugins/scripts
   are not, so a hard cut with no deprecation carries that residual risk.)
2. The **programmatic plugin SDK is not affected**: it uses structured RPC wire methods
   (`ze-plugin-callback:*`, `pkg/plugin/sdk/sdk_dispatch.go`), not command-path strings.
   Only the interactive plugin CLI and `dispatch-command` carry command-path strings, and
   in-tree those are already verb-first (e.g. `bmp.go` sends `show bgp rib protocol`).
3. Some noun-first forms are a deliberate **namespace**, not un-migrated legacy. Keep
   them: `plugin encoding/format/ack` group plugin-session directives under `plugin`, and
   `set plugin` would collide with the config-tree `plugin` node (see Engine-Owned Tree
   Mutation). `command list/help/complete` (engine introspection) migrated cleanly to
   `show command ...`; `plugin ...` stayed.
