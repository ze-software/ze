# Token bucket refill ignores the rate

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-11-13 | spec-deployment-user-journeys | `dataplane.espFormLimiter.allow` | Source finding, not reproduced: after a full second, refill restores the burst rather than accruing the configured rate. The body predates this deployment work. | open: add a full-second boundary count test and accrue elapsed rate capped at the burst. |
