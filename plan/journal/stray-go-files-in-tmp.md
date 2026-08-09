| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-04-11 | - | build | subagents saved fetched Go source into `tmp/`, `go test ./...` compiled it | renamed files to `.txt` |
| 2026-04-17 | - | build | same stray `.go` files from a different subagent | saved as `.txt` and excluded `tmp/` path |
