---
kind: directive
level: MUST
stage:
---
You MUST run these scoped gates instead:

- `./le verify lint run`
- the touched packages' `go test` (or `./le verify worktree`)
- `./le doc check verify` / `./le repository check` when those surfaces changed
- a QEMU run for any linux-only runtime code touched
