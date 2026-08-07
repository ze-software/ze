---
kind: note
level:
stage:
rationale: plan/learned/839-ci-runner-false-pass-audit.md
---
When a check, assertion, validation, or translation cannot be evaluated, return
an error that says so. Never return success, `nil`, or skip. A silent skip is a
false pass and a silent data loss, and it removes the corrective signal entirely.
See `plan/learned/839-ci-runner-false-pass-audit.md` and
`ai/rules/protocol.md`.
