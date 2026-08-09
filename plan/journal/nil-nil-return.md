| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-03-21 | - | error-handling | function returned `(nil, nil)` on error path | replaced with `(nil, err)` |
| 2026-03-21 | - | error-handling | `sync.Once` cached `(nil, err)` result, second call lost the error | rewrote to explicit state plus mutex |
