# Deferrals: fixit-ping-monitor-cadence

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/deferral-tracking.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-16 | spec-fixit-ping-monitor-cadence (Known Limitations) | `show ping` has NO inter-probe pacing: `doPingCtx` (`internal/component/ping/cmd/ping.go:274-296`) sends the next probe as soon as the previous resolves, blocking in `ReadFrom` until a reply or the `start.Add(timeout)` deadline. Against a black-holed target `show ping <dest> count 100 timeout 30s` therefore takes ~50 minutes with no output until it finishes. Real ping paces its batch; ze's does not | Shares the serial send/receive structure with `streamPing`, which the cadence spec fixes, but it is NOT the same bug: `show ping` has no `interval` argument to violate and its worst case is bounded (`count * timeout`) and operator-initiated. Folding it into the cadence spec would widen a concurrency rewrite across both engines at once. Found while writing that spec; recorded rather than fixed silently. **Moved to its own spec 2026-07-16** at the user's direction | `plan/spec-finish-ci-coverage.md` (design) | done |

