---
kind: table
level:
stage:
---
| Step | Action | Command |
|------|--------|---------|
| 1 | Make the change in ONE file | Edit a single `.ci` or `.go` file |
| 2 | Run just that behavior | Focused compiled-fixture Go test or `./le job run label unit-pkg command go test ... RUN=TestName` |
| 3 | Investigate if it fails | Read output, understand the format, fix |
| 4 | Only then apply to remaining files | Repeat the pattern that worked |
