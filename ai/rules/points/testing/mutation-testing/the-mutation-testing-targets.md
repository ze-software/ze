---
kind: table
level:
stage:
---
| Target | Purpose |
|--------|---------|
| `make ze-mutation-test` | Full run on all non-excluded packages (slow) |
| `make ze-mutation-changed` | Incremental, changed files only (fast) |
| `make ze-mutation-report` | Full run with HTML report to `tmp/mutation-report.html` |
