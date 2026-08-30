---
kind: directive
level: MUST
stage:
---
**Every spec MUST carry a metadata table immediately under its `# Spec:` title, and a status transition MUST happen at the BEGINNING of the phase it names, never at its end.** A spec left in `design` while it is being implemented is lying about its state. `hookValidateSpec` (`internal/le/hookruntime/lifecycle.go`) validates the table on every write, `writeSpecStatus` (`internal/le/hookruntime/writeedit.go`) refuses a source edit while the claimed spec is not `in-progress`, and `docs/contributing/spec-workflow.md` carries the status vocabulary and the event-to-status table.
