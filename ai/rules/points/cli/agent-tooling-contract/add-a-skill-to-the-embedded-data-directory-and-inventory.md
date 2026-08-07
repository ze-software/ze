---
kind: directive
level:
stage:
---
1. New skills go in `internal/plugins/skills/data/<name>.md` with frontmatter.
2. The skill inventory in `internal/plugins/skills/main.go` must list every embedded file.
3. `ze skills list` must show all bundled skills without a static list elsewhere.
