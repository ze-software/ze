---
kind: note
level: MUST
stage:
---
All Ze environment variable access MUST use `env.Get()` / `env.Set()` or typed helpers. Never use `os.Getenv()` or `os.Setenv()` for Ze-specific vars.
