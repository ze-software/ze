| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-03-21 | - | time | `time.Since(estAt)` bypassed simulated clock | replaced with `clock.Now().Sub(estAt)` |
| 2026-03-21 | - | time | same bypass in operational-commands handlers | replaced all `time.Now()` with clock instance calls |
