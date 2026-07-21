# Deferrals: migrate-plugin-sleeps

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/deferral-tracking.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-14 | spec-migrate-plugin-sleeps (bgp-redistribute group) | Convert the 5 `bgp-redistribute-*.ci` sleeps (announce/burst/explicit-nhop/filtered-out/nexthop-self) to deterministic waits | Original deferral reason (redistribute bypasses updates-sent counter) DISPROVEN by the infra-spec audit: `reactor_notify.go:265-268` DOES count them. Real blocker is the establishment stall below | `plan/spec-fixit-migrate-sleeps-infra.md` (moved; see P0 row below) | done |
| 2026-07-14 | spec-migrate-plugin-sleeps (DEFER/KEEP buckets) | Convert the remaining ~91 `test/plugin` intentional sleeps + the ~155 non-plugin (`firewall`/`l2tp`/`traffic`/`static`/`policy`/`vpp`) sleeps | Moved into the follow-on infra spec, which now owns the per-bucket work (reject bucket converting, others in progress) | `plan/spec-fixit-migrate-sleeps-infra.md` | done |

