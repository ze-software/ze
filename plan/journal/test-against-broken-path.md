| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-03-21 | - | tests | old-vs-new comparison test where both sides were broken | rebuilt fixture from real production output |
| 2026-03-21 | - | tests | ExaBGP migration tests used Ze syntax as input, no migration code exercised | captured fixture from live run |
| 2026-03-29 | - | tests | `.ci` test used `cmd=api` syntax the real parser did not accept | fixed fixture to match production input |
