---
kind: directive
level: MUST
stage:
---
- **Do:** you MUST use `statestore.Put(key, data)` / `statestore.Get(key)` (package `internal/core/statestore`), keyed by a registered `pkg/zefs` key (`meta/<subsystem>/<name>` in `pkg/zefs/keys.go`).
- **Don't:** you MUST NOT use `os.WriteFile` / `os.Create` / `os.OpenFile(..., O_CREATE...)` / `os.Rename` a state blob into a path under the config/state dir.
