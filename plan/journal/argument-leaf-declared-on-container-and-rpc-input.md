# A command's argument leaf is declared on its -cmd container and again on its rpc input

The command tree, completion, `ze help ai --json` and the web admin form read a
command's arguments from the leaves of its `ze:command` container in the `-cmd`
module (`extractArgDefs`, `internal/component/config/yang/command.go`). MCP and
the API schema read them from the input of the rpc in the `-api` module
(`buildParamMeta`, `cmd/ze/hub/command_meta.go`). Where both exist, one
argument is declared twice, with two descriptions and two mandatory flags, and
nothing compares them.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-15 | - | command YANG modules | Measured while fixing `system command help` (`plan/journal/command-takes-an-untyped-positional-value.md`): 25 commands declare the same leaf on the `-cmd` container and on the rpc input, `system command help` and `plugin command help` (`name`) and their `complete` siblings (`partial`) now among them. Only 70 of 405 command containers name an rpc that exists in a `-api` module, so the rpc input cannot be the one declaration, and the container leaf cannot reach MCP through `buildParamMeta`. The two copies drift with nothing to arbitrate them (`ai/rules/principles.md`) | not fixed. One declaration needs one reader: either `buildParamMeta` derives its parameters from the command tree node, or the command tree derives its `ArgDefs` from the rpc input where the rpc exists. Either is a change to a shared producer with `.ci` obligations over every command it reaches |
