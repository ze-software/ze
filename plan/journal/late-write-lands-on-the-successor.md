# Late write lands on the successor

A goroutine scoped to one session, one connection, or one generation outlives it,
because nothing joins it and its bounded waits are longer than the gap before the
successor starts. Its last write then lands on shared state the successor already
owns. The teardown that cleared the state is correct, and the entry guard that
stopped a second goroutine starting is correct. What is missing is any way for
the LAST write to know which generation it belongs to.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-31 | fixit-peer-pending-sync-settles-too-early | `sendInitialRoutes` (`internal/component/bgp/reactor/peer_initial_sync.go`) and the teardown clear in `runOnce` (`peer_run.go`) | Nothing joins the per-session `sendInitialRoutes` goroutine, and `runOnce`'s own comment says the teardown can return while it is still inside its bounded waits. `waitForAPISync` is bounded at 2s, so a session that tears down and re-establishes inside that window has its successor's `initialSyncEOROwed` cleared by the predecessor's goroutine at the end of the marker loop. `pendingSync` then reports the new session settled while its marker is still owed, and `AnnounceEOR` stops deferring to it. The identical shape already existed on `sendingInitialRoutes`, which the pre-split code cleared at the same point, so the flag split copied a hazard rather than adding one. Not reproduced: it needs a teardown and a re-establishment inside one 2s wait | recorded, not fixed. It does not block the spec that found it, and the repair is a session generation the two flags do not carry today: the goroutine would compare the generation it started under against the peer's current one before every store. The `CompareAndSwap(1, 2)` at the top of `sendInitialRoutes` guards concurrent ENTRY and cannot guard a late EXIT |
