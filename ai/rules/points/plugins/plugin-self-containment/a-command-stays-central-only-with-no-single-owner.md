---
kind: note
level:
stage:
---
A command is **generic (stays central)** only when it has no single removable
owner: it aggregates a cross-plugin registry, reads a generic core system, or is
process-global (`show warnings`, `show health`, `subscribe`). Everything that
reads one plugin's or component's state belongs to that owner, regardless of the
`ze-<ns>:` label on its WireMethod.
