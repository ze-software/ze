| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-07-30 | - | build | `make ze-verify` reported build failures across unrelated packages | ran `go clean -cache` and `docker builder prune -f` |
