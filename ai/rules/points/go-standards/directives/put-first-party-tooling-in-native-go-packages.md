---
kind: note
level:
stage:
---
First-party tooling MUST live in a native Go package under `internal/le/` and register its `./le` action. The root `le` and `ze` POSIX launchers are intentional entry points; new Python or shell helpers are prohibited.
