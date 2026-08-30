---
kind: directive
level: MUST
stage:
---
**When the design touches one of these areas, its row MUST be read too.**

| Area | Read | Prevents |
|------|------|----------|
| Plugin startup timing | `internal/component/plugin/server/startup.go` (`TopologicalTiers`, `runPluginPhase`) | Hand-waving instead of tier ordering |
| Wire encoding | `ai/rules/performance.md` | Allocations in encoding |
| Env vars | `ai/rules/go-standards.md`, `ai/rules/config.md`, `internal/core/env/` | `os.Getenv`, a missing `MustRegister`, env-only where YANG config belongs, wrong naming convention |
| JSON format | `ai/rules/cli.md` | Wrong key casing |
| Testing | `ai/rules/testing.md` and `ai/patterns/functional-test.md` | Missing `.ci` tests, wrong structure |
| Daemon lifecycle | `OnStarted` and `OnAllPluginsReady` in a similar plugin | Wrong callback, missing cleanup |
