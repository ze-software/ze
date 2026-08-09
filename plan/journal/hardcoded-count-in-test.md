| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-03-21 | - | tests | `assert len(rpcs) == 14` broke on every feature addition | replaced literal with registry query |
| 2026-03-21 | - | tests | two sessions added features, both broke the same count assertion | used `>= min_expected` with documented floor |
