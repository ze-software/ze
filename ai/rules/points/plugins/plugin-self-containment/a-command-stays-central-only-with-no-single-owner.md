---
kind: directive
level: MUST
stage:
---
- **A command MUST stay in a central verb package only when it has no single removable owner:** it aggregates a cross-plugin registry, it reads a generic core system, or it is process-global (`show warnings`, `show health`, `subscribe`). Everything that reads one plugin's or one component's state MUST be placed with that owner, whatever the `ze-<ns>:` label on its WireMethod says. Worked examples are in `docs/architecture/command-ownership.md`, "Finding the Owner".
