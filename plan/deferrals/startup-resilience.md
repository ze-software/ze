# Deferrals: startup-resilience

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/deferral-tracking.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-10 | spec-startup-resilience (AC-6 / Future) | authradius CoA end-to-end `.ci` + the missing `coa-port` YANG leaf: `coa-port` is parsed (`authradius/config.go:93-100`) but has no YANG leaf, so the CoA listener branch (`register.go:200-210`) is unreachable via production config (parser rejects unknown fields, `config/parser.go:372-380`). The apply-path DNS lookup on that branch is already BOUNDED by this spec (FIX 2: shared 750ms < 1s ApplyBudget) and unit-tested (`TestServerIPs*`); only the leaf + end-to-end `.ci` remain | Cannot exercise CoA end-to-end today (leaf not settable); adding the leaf is a distinct config-surface change, out of this reachability-only spec | `plan/spec-finish-l2tp.md` (work item added) | deferred |
| 2026-07-10 | spec-startup-resilience (Known Limitations) | Re-apply idempotency (osvbng's sibling theme) | Explicitly scoped out of the startup-resilience spec; findings route to a new spec when picked up | `plan/spec-startup-deferred-reapply-idempotency.md` | deferred |

