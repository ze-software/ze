---
kind: directive
level: MUST
stage:
---
You MUST run these scoped gates instead:

- `make ze-lint-changed`
- the touched packages' `go test` (or `make ze-precommit-verify-changed`)
- `make ze-doc-verify` / `make ze-repository-check` when those surfaces changed
- a QEMU run for any linux-only runtime code touched
