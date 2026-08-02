# Deferrals -- spec-wire-edit-4-api-origin

Source: `plan/spec-wire-edit-4-api-origin.md`. Format: `ai/rules/deferral-tracking.md`.

The encoder convergence this spec exists for is complete: one writer, both announce
rails byte-identical, `attribute.Builder.WriteTo` retired. Two rows from the spec's
own Integration and Interop checklists were NOT reached in the implementation
session. Neither blocks the goal. Both were re-homed on 2026-08-02, when this spec closed
and was removed from `plan/`.

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-01 | spec-wire-edit-4-api-origin | The `NN-wire-edit-api-origin` interop scenario: a real BIRD peer accepts an API-originated route and installs the attributes in the expected order | Not reached in the implementation session. The property it would prove is covered by unit tests over both rails and by `test/plugin/wire-edit-api-origin-order.ci`, which pins the exact wire bytes through the daemon; a live peer is stronger evidence and is still owed | `plan/spec-wire-edit-4-api-origin-deferred-bird-interop.md` (re-homed 2026-08-02; the original destination `spec-wire-edit-4-api-origin.md` was removed at closure) | deferred |
| 2026-08-01 | spec-wire-edit-4-api-origin | The `bgp_announce_dropped_oversize_total` counter, so an AC-5 oversize drop is observable as a metric rather than only as a log line | Not reached in the implementation session. The drop itself is implemented and tested (`TestAnnounceOversizeDropsWithNamedLog` asserts both rails refuse and name the route), so the fail-closed behavior is proven; only the metric surface is missing | `plan/spec-wire-edit-4-api-origin-deferred-oversize-metric.md` (re-homed 2026-08-02; the original destination `spec-wire-edit-4-api-origin.md` was removed at closure) | deferred |
