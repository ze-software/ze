---
kind: note
level:
stage:
---
Every exported struct field that reaches JSON output **must** have a `json:"kebab-name"` tag. Keys are lowercase kebab-case, matching the YANG leaf or config tree key. Full rules and attribute table: `ai/rules/cli.md`.
