---
kind: directive
level: MUST NOT
stage:
---
**Go struct tags:** `json:"kebab-name"` or `json:"kebab-name,omitempty"`. The tag
is the contract. Go field names are PascalCase (Go convention), JSON tags are
kebab-case (Ze convention). MUST NOT let Go's default JSON marshaling (which uses the
Go field name) leak into output.
