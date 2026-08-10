| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-10 | session-bin-directory | build | mutation-testing a build recipe's path guard executed the unguarded path FOR REAL: the seed step reached the shared `bin/ze` and ran `ze init` against the operator's own `etc/ze` | the operator's database survived only because the recipe carries no `--force`, which is what calls `moveAsideDB`. The command a guard protects must be non-destructive on its own, because the proof that the guard works runs that command without it |
