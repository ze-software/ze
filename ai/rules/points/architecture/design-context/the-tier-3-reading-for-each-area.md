---
kind: table
level:
stage:
---
| Area | Read | Prevents |
|------|------|----------|
| Plugin startup timing | `internal/component/plugin/server/startup.go` (`TopologicalTiers`, `runPluginPhase`) | Hand-waving instead of tier ordering |
| Wire encoding | `ai/rules/performance.md` | Allocations in encoding |
| Env vars | `ai/rules/go-standards.md` + `ai/rules/config.md` + `ai/rules/config.md` + `internal/core/env/` | `os.Getenv`, missing `MustRegister`, env-only when should be YANG config, wrong naming convention |
| JSON format | `ai/rules/cli.md` | Wrong key casing |
| Testing | `ai/rules/testing.md` + `ai/patterns/functional-test.md` | Missing .ci tests, wrong structure |
| Daemon lifecycle | `OnStarted`/`OnAllPluginsReady` in a similar plugin | Wrong callback, missing cleanup |
