---
kind: directive
level: MUST
stage:
---
**Registration required:** every env var MUST be registered via package-level `var _ = env.MustRegister(...)`. Calling `env.Get()` with an unregistered key aborts the process.
