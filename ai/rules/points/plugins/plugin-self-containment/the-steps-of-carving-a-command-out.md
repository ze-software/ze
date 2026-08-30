---
kind: directive
level: MUST
stage:
---
- **A carve MUST put the handler and its `pluginserver.RegisterRPCs` in the owner package, and the command YANG in `<owner>/yang/` as a standalone module that re-declares the path from the root.** The command YANG MUST NOT be nested under `<owner>/cmd/yang/`. The step-by-step, including the generator refresh a new `yang/` package needs, is `docs/architecture/command-ownership.md`, "Carving a Command Into Its Owner".
