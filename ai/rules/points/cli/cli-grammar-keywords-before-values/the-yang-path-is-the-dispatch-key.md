---
kind: directive
level: MUST
stage:
---
The daemon dispatcher registers each built-in handler under its **YANG path**, not
its wire method: `LoadBuiltins` does `d.RegisterWithOptions(wireToPath[wireMethod], ...)`
(`internal/component/plugin/server/command.go`). Moving a `ze:command` container in
the YANG tree therefore changes the command's dispatch key, and the move MUST be
treated as a rename. Relocating `command list` to `show command list` deletes the old
`command list` key entirely.
