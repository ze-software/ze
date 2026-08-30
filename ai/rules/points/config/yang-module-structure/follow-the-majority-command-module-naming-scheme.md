---
kind: directive
level: MUST NOT
stage:
---
**A NEW command module for an operational verb SHOULD take the majority `ze-cli-<verb>-cmd` form with a paired `-api`, and it MUST NOT invent a fourth scheme.** Converging the existing names is a rename tracked separately, and it is described in `docs/architecture/config/yang-config-design.md`. Command ownership and grammar rules are `ai/rules/cli.md` and `ai/rules/plugins.md`.
