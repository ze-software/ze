---
kind: directive
level: MUST
stage:
---
**Every exported struct field that reaches JSON output MUST carry a `json:"kebab-name"` tag.** Keys are lowercase kebab-case, matching the YANG leaf or the config tree key. The full rules and the attribute table are in `ai/rules/cli.md`.
