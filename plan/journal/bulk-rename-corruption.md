| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-03-21 | - | refactor | bulk sed renamed map-literal string keys alongside variable names | reverted and edited file-by-file |
| 2026-04-07 | - | refactor | family rename hit slog kv pairs and GetContainer arguments | reverted and renamed selectively |
| 2026-08-10 | fixit-unexport-package-private-symbols | refactor | `gopls rename` refuses a rename that breaks a reference. It runs untagged on the host GOOS. A reference under `//go:build ze_core` is invisible to it, so the refusal never fires. Unexporting `AvailableInternalPlugins` broke `cmd/ze`. Its only caller is `cmd/ze/main_test.go`, under `ze_core`. A per-package `go vet` misses it too, because it never compiles the other package | whole-tree `go vet` over all four build views after every package is processed, then rename each caught symbol back |
