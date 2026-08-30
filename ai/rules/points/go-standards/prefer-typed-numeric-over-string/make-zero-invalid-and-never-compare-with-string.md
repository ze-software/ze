---
kind: directive
level: MUST
stage:
---
**A typed enum MUST make its zero value invalid, so an unset field cannot pass for a real one. A hot-path comparison MUST be against the typed constant, never against a string literal.**
