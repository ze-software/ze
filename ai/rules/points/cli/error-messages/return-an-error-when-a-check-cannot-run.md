---
kind: directive
level: MUST
stage:
---
**A check, assertion, validation or translation that cannot be evaluated MUST return an error saying so, and MUST NOT return success, `nil`, or skip.** A silent skip is a false pass and a silent data loss, and it removes the corrective signal entirely. See `ai/rules/protocol.md`.
