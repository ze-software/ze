| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-07-30 | - | config | `enabled != true` early return skipped security settings parsing | split into `extractXBlock` plus two exported callers |
| 2026-08-03 | - | config | same pattern on looking-glass TLS flag | same split: parse everything, gate nothing |
