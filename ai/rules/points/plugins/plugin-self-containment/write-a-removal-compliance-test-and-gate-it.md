---
kind: note
level:
stage:
---
A removal-compliance test must exist and run in verification: build (or analyse
the command/schema registries) with a plugin's provider import removed and
assert no command, schema node, help string, or handler reference to that
plugin survives in any generic or central package. See
`ai/rules/repo-maintenance.md`: adding this invariant means adding the gate
that enforces it.
