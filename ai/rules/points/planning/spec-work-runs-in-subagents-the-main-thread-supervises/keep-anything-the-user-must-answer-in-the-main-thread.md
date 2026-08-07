---
kind: directive
level:
stage:
---
**Anything the user must answer stays in the main thread.** A subagent cannot hold a dialogue with the user, so `/ze-spec` and `/ze-design` question gates, scope reductions, and RFC-compliance escalations (`ai/rules/rfc-compliance.md`) are raised by the main thread, never delegated away.
