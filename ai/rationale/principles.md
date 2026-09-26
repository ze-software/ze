# Principles rationale

Rule: `ai/rules/principles.md`.

## History moved from rule points (2026-09-26)

- `ai/rules/points/principles/directives/a-wrong-value-must-not-look-like-a-right-one.md`: "This is the single largest source of defects this repository has recorded: a type assertion that fails and disables a feature with no log line, a cross-boundary call that no-ops when the plugin runs external, a search whose zero hits are read as absence, a test whose passing assertion would also pass against a stub." The examples stay in the point as the defect's shapes.
