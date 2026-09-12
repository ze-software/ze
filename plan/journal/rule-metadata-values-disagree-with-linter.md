| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-16 | - | rule format | The documented `level` values omitted `SHOULD NOT`, which the policy and linter accept | added `SHOULD NOT` to the canonical field table |
| 2026-08-30 | - | rule corpus | `principles/directives/done-means-a-user-reaches-it` declared `level: MUST` while its body states only MUST NOT, so `./le rules lint` was red at HEAD | set `level: MUST NOT` to match the body |
| 2026-09-12 | - | rule corpus | `testing/rfc-tagged-tests-blocking/reindex-after-moving-a-tagged-test` declares `level: MUST` while every obligation in its body is MUST NOT, so `./le rules lint` is red at HEAD | not fixed; third row in this class, so the deliberate pass over the journal owns it |
