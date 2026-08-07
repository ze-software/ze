---
kind: note
level: MUST NOT
stage:
---
`scripts/dev/dep_audit.py --check` enforces the engine-placement rule, the non-engine category manifest, the **core import-direction rule** (`internal/core/` MUST NOT import `internal/component/` or `internal/plugins/`; grandfathered pairs live in the shrink-only `scripts/dev/core_import_baseline.txt` with a fix route each, and new pairs and stale rows both fail), the disable-ability rule, and golangci build-tag drift. It runs in `make ze-verify` (target `ze-tier-check`). It:
