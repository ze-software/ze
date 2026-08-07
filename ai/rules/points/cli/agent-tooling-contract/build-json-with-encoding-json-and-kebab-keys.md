---
kind: directive
level:
stage:
---
1. Use `encoding/json`, never string concatenation.
2. Use lower kebab-case keys (see "Field Naming: kebab-case" above).
3. Include `schema-version` in top-level envelopes via `diagnostic.NewValidateResult` or `diagnostic.NewFixPlan`.
4. Emit no ANSI escape sequences when stdout is not a terminal.
