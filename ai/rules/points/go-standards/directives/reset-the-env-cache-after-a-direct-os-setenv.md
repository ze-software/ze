---
kind: directive
level: MUST
stage:
---
**Cache:** Built once from `os.Environ()` on first `Get()`. `Set*()` updates both cache and os env. Tests that use `os.Setenv` directly MUST call `env.ResetCache()`.
