# A policy change never re-evaluates what the old policy admitted

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-08 | static-route-tag-reaches-no-consumer | redistribute `SetGlobal` vs orchestrator `handleBatch` | A reload that narrows or removes a `redistribute` import rule leaves the routes the old rule admitted at their destination: `SetGlobal` only stores the evaluator and nothing re-judges held routes. Producers read; not reproduced | not fixed; owner decides whether a rule change retracts at once. `heldRoutes` (`held.go`) is the input a fix needs |
