---
kind: table
level:
stage:
---
| Axis | Mechanical test |
|------|-----------------|
| **A. Is it a config-driven engine?** | does it call `sdk.NewWithConn(`? |
| **B. Does a feature depend on it?** | does any `.go` file under `internal/component/` or `internal/plugins/` (excluding its own subtree, the generated composition root, `cmd/ze` dispatch, `internal/core`, `internal/chaos`, `internal/test`, and `_test.go`) import it? |
