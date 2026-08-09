| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-03-21 | - | tests | test A called `Reset()` on package-level registry, test B failed | added Snapshot/Restore pair with t.Cleanup |
| 2026-04-06 | - | tests | Snapshot/Restore/Reset in registry missed new global | included every new global in snapshot |
