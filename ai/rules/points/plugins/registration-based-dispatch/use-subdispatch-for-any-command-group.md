---
kind: directive
level: MUST
stage:
---
- **A command group that has sub-actions MUST be built with `subdispatch.New(name, summary)`, registering each sub-action with its handler and description.** The dispatcher then owns help, the unknown-command error and the suggestion. The template is `ai/patterns/plugin.md`, "Sub-Dispatcher Registration".
