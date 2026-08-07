---
kind: table
level:
stage:
---
| Wrong | Right | Why |
|-------|-------|-----|
| `json:"remove-private"` | `json:"remove-private-as"` | Truncated; must match the full config key |
| `json:"policyAttrASPathRemovePrivate"` | `json:"remove-private-as"` | camelCase is not kebab-case |
| No `json` tag on exported field | `json:"kebab-name"` | Go default leaks PascalCase |
