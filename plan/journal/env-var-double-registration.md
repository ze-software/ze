| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-03-29 | - | config | changing env var default in one file had no effect at runtime | reconciled to one registration site |
| 2026-04-18 | - | config | duplicate `ze.config.dir` in two files | documented the duplication with cross-reference comment |
| 2026-09-04 | spec-local-ca | plugin transport | `ze.plugin.hub.host/port/token` and `ze.plugin.ca.pem` are registered in `pkg/plugin/sdk` and read by `internal/component/plugin/cli.connFromEnv` and `internal/core/rib/locrib.Default`, neither of which imports the SDK; a binary that links a reader without it aborts in `env.mustBeRegistered` with `os.Exit(2)` | not fixed: the shipped binaries link both, so only a narrow test binary reaches it, and the one test that does blank-imports the SDK |
