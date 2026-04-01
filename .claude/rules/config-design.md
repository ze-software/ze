# Config Design

Rationale: `.claude/rationale/config-design.md`
Structural template: `.claude/patterns/config-option.md`

- No version numbers in config. Design for machine-transformable migration.
- Fail on unknown keys at any level. No silent ignore. Suggest closest valid key.
- Every YANG `environment/<name>` leaf MUST have a matching `ze.<name>.<leaf>` env var registered via `env.MustRegister()`. Env vars are part of the config interface, not follow-up work.
