---
kind: directive
level: MUST NOT
stage:
---
- **A verb whose subcommands belong to several owners MUST NOT declare its root container inside any one plugin.** Deleting that plugin would delete the whole verb. The root MUST live in a central, plugin-free `internal/component/cmd/<verb>` that holds NO handler, and each owner container-merges only its own subtree onto it.
