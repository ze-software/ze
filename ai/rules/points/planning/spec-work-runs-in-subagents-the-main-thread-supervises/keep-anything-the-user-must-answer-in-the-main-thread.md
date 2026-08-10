---
kind: directive
level: MUST
stage:
---
**Anything the user MUST answer MUST stay in the main thread.** A subagent cannot hold a dialogue with the user, so `/ze-spec` and `/ze-design` question gates, scope reductions, and RFC-compliance escalations (`ai/rules/rfc-compliance.md`) are raised by the main thread and MUST NOT be delegated away.
