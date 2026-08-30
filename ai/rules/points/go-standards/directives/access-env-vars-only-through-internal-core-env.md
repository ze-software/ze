---
kind: directive
level: MUST
stage:
---
**Every Ze environment variable MUST be reached through `internal/core/env` and MUST be registered with a package-level `var _ = env.MustRegister(...)`; `env.Get()` on an unregistered key aborts the process.** `os.Getenv` and `os.Setenv` MAY be used only for a genuine system variable such as `PATH`, `HOME` or `NO_COLOR`, a test that calls `os.Setenv` directly MUST call `env.ResetCache()`, and `ai/rules/config.md` decides whether the setting belongs in YANG instead.
