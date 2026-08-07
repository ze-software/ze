---
kind: note
level:
stage:
---
If the loop body genuinely needs per-file logic that a single command cannot
express, batch with `xargs` or `find -exec +` instead of per-file forks:
