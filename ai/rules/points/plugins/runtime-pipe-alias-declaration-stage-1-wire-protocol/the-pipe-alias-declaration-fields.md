---
kind: table
level:
stage:
---
| Field | Type | Purpose |
|-------|------|---------|
| `pipes[].command` | string | Command path the alias sits on. MUST be one of this plugin's own declared commands |
| `pipes[].name` | string | The word an operator types after the pipe character (kebab-case, 1-64 chars) |
| `pipes[].description` | string | The line completion and `command help` show beside the name |
| `pipes[].expansion` | string | The operator chain the name stands for, as an operator would type it |
