---
kind: directive
level: MUST
stage:
---
**Escalation ladder:** direct test -> file/feature test -> single package -> component group -> whole suite or `ze-verify`. If any rung fails, MUST fix from that evidence and rerun the failed rung or a narrower failing test, not a wider suite.
