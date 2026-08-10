| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-03-21 | - | tests | old-vs-new comparison test where both sides were broken | rebuilt fixture from real production output |
| 2026-03-21 | - | tests | ExaBGP migration tests used Ze syntax as input, no migration code exercised | captured fixture from live run |
| 2026-03-29 | - | tests | `.ci` test used `cmd=api` syntax the real parser did not accept | fixed fixture to match production input |
| 2026-08-10 | - | tests | `test/plugin/bgp-gtsm-reject.ci` (index 78) times out at 5.0s with zero received messages, standalone and in the suite. Reproduced on a pristine `git archive` of committed HEAD, so it predates the working tree. It carries `expect=exit:code=1` and no `tmpfs=` block, so it cannot reach the `runOrchestrated` teardown branch changed the same day | not fixed, found while clearing the ze-lint fault |
