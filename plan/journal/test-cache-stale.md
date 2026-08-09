| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-03-21 | - | build | `go test` cache hid compile break in dependent package | ran `go clean -testcache` before verify |
| 2026-03-28 | - | build | same stale cache after modifying exported identifier | touched importing package to force recompile |
