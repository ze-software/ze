---
kind: table
level:
stage:
---
| What changed | Also update |
|---|---|
| New leaf/container | Config parser that reads the tree (grep `GetContainer`, `GetChild` for the path) |
| New leaf/container | Validator if validation rules apply |
| New leaf/container | CLI completion if the command references the schema |
| Renamed path | `scripts/dev/yang_move.py` handles slash paths, set commands, brace blocks, GetContainer chains |
| New `environment/` leaf | `env.MustRegister()` in the component's config loader |
| New `ze:listener` | Conflict detection via `FindListenerConflict` |
| New `ze:command` | RPC handler + `make ze-doc-test` |
