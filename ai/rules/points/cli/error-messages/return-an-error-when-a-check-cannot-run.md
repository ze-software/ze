---
kind: note
level:
stage:
---
When a check, assertion, validation, or translation cannot be evaluated, return
an error that says so. Never return success, `nil`, or skip. A silent skip is a
false pass and a silent data loss, and it removes the corrective signal entirely.
An audit of the `.ci` runner found four of these at once: an assertion that was
parsed but never read in the decision path, an early `return true` that skipped
the later assertions, an empty pattern that matched everything, and a content
matcher that skipped a family it could not extract. See `ai/rules/protocol.md`.
