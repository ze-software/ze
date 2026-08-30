---
kind: directive
level: MUST NOT
stage:
---
**Every Ze environment variable access MUST use `internal/core/env`. `os.Getenv` and `os.Setenv` MUST NOT be used for a Ze-specific variable.** The accessors and the registration flags are in `docs/contributing/go-conventions.md`. `os.Getenv` stays correct for a genuine system variable such as `PATH`.
