---
kind: directive
level: MUST
stage:
---
1. MUST use `encoding/json`; MUST NOT use string concatenation.
2. MUST use lower kebab-case keys (see "Field Naming: kebab-case" above).
3. MUST include `schema-version` in top-level envelopes via `diagnostic.NewValidateResult` or `diagnostic.NewFixPlan`.
4. MUST NOT emit ANSI escape sequences when stdout is not a terminal.
