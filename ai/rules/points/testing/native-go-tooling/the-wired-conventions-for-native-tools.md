---
kind: table
level:
stage:
---
| Tool kind | Convention | Runs because |
|-----------|------------|--------------|
| Repository action | Put callable behavior and `*_test.go` beside it under `internal/le/<area>/`; register the command in `register.go` | `./le <area> <verb>` reaches the same Go function the package test calls |
| Functional fixture | Put the compiled driver and its tests under `internal/test/fixture`; register the driver with `fixture.Register` | `ze-test fixture <name>` reaches the registry, and the owning `./le functional <suite>` action exercises the `.ci` caller |
