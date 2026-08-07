---
kind: table
level:
stage:
---
| Situation | Action |
|-----------|--------|
| Splitting a file | Hub gets `// Detail:` to leaves, leaves get `// Overview:` to hub |
| Tightly coupled new file | Add reference + matching back-reference |
| Touching file with stale refs | Fix (remove deleted, add missing, fix direction) |
