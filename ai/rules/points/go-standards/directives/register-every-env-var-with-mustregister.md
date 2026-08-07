---
kind: directive
level:
stage:
---
**Registration required:** Every env var must be registered via package-level `var _ = env.MustRegister(...)`. Calling `env.Get()` with an unregistered key aborts the process.
