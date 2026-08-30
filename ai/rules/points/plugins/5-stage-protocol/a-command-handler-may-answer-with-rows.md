---
kind: directive
level: MUST NOT
stage:
---
- **A command handler MAY answer with a `plugin.Records` rather than a built value, and `Records.Rows` MUST NOT be stored.** It is walked once, before the handler's call returns.
<!-- source: pkg/plugin/records.go -- Records, Records.WriteAnswer -->
