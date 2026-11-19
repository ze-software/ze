# A supplied configuration path outranks the configured store folder

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-11-19 | fleet-6-config-freeze (walked into, not caused) | `internal/core/resolve/resolve.go` `StoreDir` | A bare config name typed outside the store folder (`ze config edit client-edge-99.conf`) answers "no configuration store" even with `ze.config.dir` set, because any supplied path, a bare name included, outranks the environment | open: decide whether a bare name with no file beside it resolves to the configured store; 35 callers and the loose-file `ze start <name>` form depend on the rule |
