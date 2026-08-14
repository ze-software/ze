# Deferrals: fixit-mirror-clsact-ownership

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/planning.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-14 | spec-fixit-mirror-clsact-ownership (R-2) | Reconcile kernel tc state against the configuration at startup, so a mirror removed from the config file while ze was down is torn down on the next boot | This fix makes teardown work from the previous config, which `applyConfig` already holds. A restart applies with a nil previous config, so nothing tells ze the mirror existed. Every interface delta in `applyConfig` has that shape, not the mirror alone, so the answer is a reconciliation pass over kernel state rather than a branch in one apply path. Building it inside this fix would widen a two-defect change into a design | unassigned: needs its own spec, and Thomas has not commissioned one | deferred |
