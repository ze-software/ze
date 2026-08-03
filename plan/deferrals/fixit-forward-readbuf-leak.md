# Deferrals: fixit-forward-readbuf-leak

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/planning.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-21 | spec-fixit-forward-readbuf-leak (adjacent observation) | Outgoing peer-pool MOD buffer leak on the body-build failure path: `forwardUpdateCore` acquires a mod buffer (`reactor_api_forward.go`) then `continue`s at :627-628 when `buildFwdBody` fails, dropping the item without `Return`; same shape in `forward_rs.go`/:436-437. Distinct from the read-pool leak this spec fixed (different pool, failure path only) | Found while verifying the read-pool fix; explicitly OUT of this spec's scope ("flag for triage, do not fix here without approval"); recorded in the spec Notes which vanish on closure, so homed here + in learned 1234 | plan/spec-fixit-forward-readbuf-leak-deferred-pool-release-paths.md (re-homed 2026-08-03; re-confirm the citation first, the fan-out dedup rewrote that function) | deferred |
| 2026-07-21 | spec-fixit-forward-readbuf-leak (adjacent observation) | Pool-stopped `DispatchOverflow` calls `done()` but not `releaseItem` (`forward_pool.go`,:617-623) -- shutdown-window-only leak of item resources | Found while verifying the read-pool fix; OUT of this spec's scope (forward_pool.go untouched per D-1); shutdown-window only. Recorded in learned 1234 | plan/spec-fixit-forward-readbuf-leak-deferred-pool-release-paths.md (re-homed 2026-08-03; verified still live) | deferred |

