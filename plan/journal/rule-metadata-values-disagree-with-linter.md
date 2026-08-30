| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-16 | - | rule format | The documented `level` values omitted `SHOULD NOT`, which the policy and linter accept | added `SHOULD NOT` to the canonical field table |
| 2026-08-30 | - | rule corpus | `principles/directives/done-means-a-user-reaches-it` declared `level: MUST` while its body states only MUST NOT, so `./le rules lint` was red at HEAD | set `level: MUST NOT` to match the body |
