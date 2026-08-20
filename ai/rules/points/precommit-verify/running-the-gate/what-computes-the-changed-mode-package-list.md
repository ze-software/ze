---
kind: note
level:
stage:
---
- **One producer answers the change set: `runSelector` (`scripts/checks/verify_scope_selector.go`). `scripts/dev/changed-pkgs.sh` holds no selection logic and only dispatches between the two routes to it.**
- **A verify run selects ONCE, before the first stage, writes the answer into that run's own artifact directory, and names that file to every stage in `ZE_VERIFY_SCOPE_PACKAGES`.** Two runs of one checkout therefore never share a scope, and no two stages of one run scope to different trees.
- **A direct `make ze-lint-changed` or `make ze-unit-test-changed` outside a verify run has no published answer, so it selects its own** (2.4 to 2.9s). Both routes reach the same producer.
- **The import graph is built with `ze_core` and every tag in `feature-gates.txt`, so a `//go:build ze_<feature>` importer is selected.** One file under `internal/component/ssh` selects `./cmd/ze`, `./cmd/ze/hub` and `./internal/component/ssh`, and the feature answer is `ze_ssh` alone.
- **The reverse walk stops at two levels of importers, and the selector states on stderr how many packages that bound dropped.** `make ze-verify-scope-selector ARGS=--drop-log=FILE` records which ones.
- **Ask the selector what your own change set answers before you assume the scoped run covered it: `make ze-verify-scope-selector ARGS="--print=both"`.** On a tree several sessions have dirtied the answer is often still wide, and the narrow answer belongs to a change set that touches one feature.
