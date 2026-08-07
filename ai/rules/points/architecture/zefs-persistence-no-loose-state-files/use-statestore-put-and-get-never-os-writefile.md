---
kind: directive
level:
stage:
---
- **Do:** `statestore.Put(key, data)` / `statestore.Get(key)` (package `internal/core/statestore`), keyed by a registered `pkg/zefs` key (`meta/<subsystem>/<name>` in `pkg/zefs/keys.go`).
- **Don't:** `os.WriteFile` / `os.Create` / `os.OpenFile(..., O_CREATE...)` / `os.Rename` a state blob into a path under the config/state dir.
