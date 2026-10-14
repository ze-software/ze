| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-03-21 | - | time | `time.Since(estAt)` bypassed simulated clock | fixed: replaced with `clock.Now().Sub(estAt)` |
| 2026-03-21 | - | time | same bypass in operational-commands handlers | fixed: replaced all `time.Now()` with clock instance calls |
