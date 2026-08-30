---
kind: directive
level: MUST
stage:
---
**The corrective action MUST be carried on a machine-facing surface (doctor, startup, config apply and verify, readiness, plugin load).** An internal error wrapped upward MUST carry the first two legs and a wrapped cause (`%w`), and SHOULD carry the corrective action whenever a clear next step exists. A deep internal error MUST NOT invent one.
