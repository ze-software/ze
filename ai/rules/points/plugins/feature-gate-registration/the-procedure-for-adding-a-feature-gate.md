---
kind: directive
level: MUST
stage:
---
- **A new gate MUST be finished with `./le repository generate`, which emits `all_ze_<x>.go` and regenerates every derived tag list, and then `./le verify worktree`.** A feature-only helper MUST live INSIDE a gated file, or a no-feature build flags it U1000-unused. The six-step procedure is `docs/architecture/plugin/feature-gates.md`, "Procedure: add a feature gate".
