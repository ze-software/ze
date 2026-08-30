# Deferrals: fixit-migrate-sleeps-infra

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/planning.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-14 | spec-fixit-migrate-sleeps-infra (P0) | Root-cause and fix (or document a constraint + recipe for) the redistribute establishment stall: observer activity (any engine RPC OR even a `wait_for_event` callback read) during BGP establishment prevents a single-peer redistribute session from establishing (`connections-established: 0`); ties to the late-join replay-on-establish path (`redistribute_egress/replay.go` fires ReplayRequest on peer establishment). Config-specific: `redistribute-as112-announce` (different import) unaffected, plain BGP (`nexthop-self.ci`) fine. Blocks converting `bgp-redistribute-*`, `api-raw`, `api-route-refresh` | Engine concurrency investigation, not a test conversion; carved out so the infra spec's other buckets (reject done, bmp/teardown/show-l2tp/rs next) proceed | `spec-fixit-redistribute-establishment-stall` <!-- doc-links: ignore (spec closed on 2026-08-23 by 8f3a80bf9 and deleted from plan/) --> (P0 backoff-floor bug found and fixed in `44ad25d23`; the residual stall stays open there) | done |

