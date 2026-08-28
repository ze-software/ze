---
kind: note
level:
stage:
---
- **One producer answers the change set: `runSelector` (`internal/le/changed/scope.go`). `internal/le/changed/changed.go` holds no selection logic and only dispatches between the two routes to it.**
- **A verify run selects ONCE, before the first stage, writes the answer into that run's own artifact directory, and names that file to every stage in `ZE_VERIFY_SCOPE_PACKAGES`.** Two runs of one checkout therefore never share a scope, and no two stages of one run scope to different trees.
- **A direct `./le verify-lint run` or `./le test-unit` outside a verify run has no published answer, so it selects its own** (2.4 to 2.9s). Both routes reach the same producer.
- **The import graph is built with `ze_core` and every tag in `feature-gates.txt`, so a `//go:build ze_<feature>` importer is selected.** One file under `internal/component/ssh` selects `./cmd/ze`, `./cmd/ze/hub` and `./internal/component/ssh`, and the feature answer is `ze_ssh` alone.
- **The reverse walk stops at two levels of importers.** `./le changed scope drop-log FILE` records which packages that bound dropped.
- **Ask before assuming:** `./le changed scope print both` shows the current change set's package and feature-tag scope.
