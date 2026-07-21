# Deferrals: improve-3-event-replay

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/deferral-tracking.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-10 | spec-improve-3-event-replay (R-3) | Exact goroutine-interleaving reproduction in event replay (deterministic scheduler / event-queue layer) | Replay asserts outcomes (FSM transitions, RIB effect), not interleavings; exact reproduction needs the analysis doc's event-queue layer, out of scope for this spec | `plan/spec-improve-3-event-replay-deferred-deterministic-scheduler.md` | deferred |

