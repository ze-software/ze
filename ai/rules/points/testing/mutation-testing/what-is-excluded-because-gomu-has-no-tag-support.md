---
kind: note
level:
stage:
---
gomu has no `--tags` support. Files with custom build tags (`ze_test`,
`ze_chaos`, `ze_perf`, `ze_analyze`) and `cmd/ze/` are excluded via
`.gomuignore`. Reports go to `tmp/` (gitignored).
