---
kind: directive
level: MUST
stage:
---
1. New skills MUST go in `internal/plugins/skills/data/<name>.md` with frontmatter.
2. The skill inventory in `internal/plugins/skills/main.go` MUST list every embedded file.
3. `ze skills list` MUST show all bundled skills without a static list elsewhere.
