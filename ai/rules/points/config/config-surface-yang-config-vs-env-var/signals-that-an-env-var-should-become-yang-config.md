---
kind: directive
level: SHOULD
stage:
---
**These signals indicate an env var SHOULD become YANG config:**
- It appears in runbooks or deployment documentation
- Multiple operators have asked about it or been told to set it
- It controls behavior visible in `show` commands or logs
- Changing it is part of normal scaling or tuning workflows
- It was added as env-only for expedience during implementation
