# Mutation Score History

`history.ndjson` accumulates one line per package per gomu run:

```json
{"ts":"2026-06-10T12:00:00Z","sha":"3df025ae3","package":"internal/core/bufpool","mutants":120,"killed":96,"score":80.0}
```

Append it with `./le mutation record-history report <path>` after each native
gomu run.
Commit the file with the run that produced it so mutation coverage has a
trend and a reviewable baseline (the gomu incremental cache
`.gomu_history.json` is gitignored and holds no kill/survive results).

Fix surviving mutants with `/ze-mutation-fix`.
