---
kind: directive
level: MUST
stage:
---
- **A removal-compliance test MUST exist and MUST run in verification:** build, or analyse the command and schema registries, with a plugin's provider import removed, and assert that no command, schema node, help string or handler reference to that plugin survives in any generic or central package. Adding this invariant means adding the gate that enforces it (`ai/rules/repo-maintenance.md`).
