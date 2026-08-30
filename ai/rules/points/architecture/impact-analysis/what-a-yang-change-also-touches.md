---
kind: directive
level: MUST
stage:
---
**A change to a `*.yang` file MUST also update everything its row names.**

| What changed | Also update |
|---|---|
| New leaf or container | The config parser that reads the tree (grep `GetContainer` and `GetChild` for the path) |
| New leaf or container | The validator, when validation rules apply |
| New leaf or container | CLI completion, when the command references the schema |
| Renamed path | `./le yang migration path-refactor` handles slash paths, set commands, brace blocks, and GetContainer chains |
| New `environment/` leaf | `env.MustRegister()` in the component's config loader |
| New `ze:listener` | Conflict detection through `FindListenerConflict` |
| New `ze:command` | The RPC handler, then `./le doc check verify` |
