---
kind: fence
level:
stage:
---
```
[ ] Classified as YANG config or env-only using the decision table above
[ ] If YANG: leaf defined with type, range, default, description
[ ] If YANG: description mentions env var override if one exists
[ ] If env-only: env.MustRegister() with clear description
[ ] If env-only: document WHY it is not in YANG (debug, bootstrap, safety cap)
[ ] If promoting: old env var preserved, precedence documented
```
