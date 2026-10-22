# A policy change never re-evaluates what the old policy admitted

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-08 | static-route-tag-reaches-no-consumer | redistribute `SetGlobal` vs orchestrator `handleBatch` | A reload that narrows or removes a `redistribute` import rule leaves the routes the old rule admitted at their destination: `SetGlobal` only stores the evaluator and nothing re-judges held routes. Producers read; not reproduced | open: not fixed; owner decides whether a rule change retracts at once. `heldRoutes` (`held.go`) is the input a fix needs |
| 2026-10-22 | - | GoBGP `pkg/config/config.go` `UpdateConfig`, `internal/pkg/table/destination.go` `Update.GetChanges` | Four-peer wire probe: changing export reject to accept by SIGHUP advertises the retained best to the secondary-route client, but not the ordinary client. Reload runs soft reset IN only; unchanged best paths suppress ordinary export. A client ROUTE-REFRESH supplies the missing route. | open: third-party checkout not patched; reconcile export policy changes with soft reset OUT, including previously sent routes |
