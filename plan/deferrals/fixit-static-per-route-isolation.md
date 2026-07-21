# Deferrals: fixit-static-per-route-isolation

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/deferral-tracking.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-19 | plan/learned/650-static-routes.md | per-route error isolation for static routes (log-and-skip a bad route vs whole-section fail) | changes the failure contract for all static routes + interacts with OnConfigApply journal/rollback; scoped to its own spec | plan/spec-fixit-static-per-route-isolation.md | deferred |

