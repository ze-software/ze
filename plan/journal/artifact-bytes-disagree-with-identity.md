| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-26 | spec-terminal-demo-showcase | `runBuild` and Go executable installation | Two successful native builds yielded zeroed executable regions; both files fail their embedded Go content-ID check. A later native build's retained source and staged file matched. | open: cause unestablished. Preserve damaged artifacts and compare source/destination before execution on recurrence; distinguish source, copy and later mutation. No code fix claimed. |
