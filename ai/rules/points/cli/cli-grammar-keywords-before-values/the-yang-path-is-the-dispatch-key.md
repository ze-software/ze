---
kind: note
level:
stage:
---
The daemon dispatcher registers each built-in handler under its **YANG path**, not
its wire method: `LoadBuiltins` does `d.RegisterWithOptions(wireToPath[wireMethod], ...)`
(`internal/component/plugin/server/command.go`). So **moving a `ze:command` container
in the YANG tree changes the command's dispatch key.** Relocating `command list` to
`show command list` deletes the old `command list` key entirely.
