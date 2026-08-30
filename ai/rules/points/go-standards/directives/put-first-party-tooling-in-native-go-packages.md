---
kind: directive
level: MUST
stage:
---
**First-party tooling MUST live in a native Go package under `internal/le/` and MUST register its `./le` action. A new Python or shell helper MUST NOT be added.** The root `le` and `ze` POSIX launchers are the intentional entry points.
