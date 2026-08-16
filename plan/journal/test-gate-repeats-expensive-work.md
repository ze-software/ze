| Date | Spec | Surface | Symptom | Fix |
|---|---|---|---|---|
| 2026-08-16 | weakened-followups | RFC requirement Python tests | `rfc_requirements_test.py` exceeded the 180-second package-test limit because `go_func_scopes` parsed the same 495 file contents 10,278 times. | Cache immutable spans by exact content in a bounded LRU. The full script now runs 792 tests in 36.750 seconds. |
| 2026-08-16 | staticcheck-feature-tag-matrix | agent verification | The agent reran `ze-precommit-verify-changed` for one known edited test and caused 600+ unrelated tests to run. | Exact package and test reruns now precede one final aggregate pass. |
