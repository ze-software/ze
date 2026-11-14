# A wait outlives the process whose output it waits for

The waiter polls for a file or a line that one process writes, and it does not
watch that process. When the process exits without writing it, the wait runs to
its deadline anyway, so the exit is reported late, as a timeout, or not at all.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-11-01 | filter-wire-0-umbrella (walked into) | `waitReady` (`internal/test/runner/runner_exec_util.go`) | A foreground ze that exits at startup (exit 1, refused chain) never writes daemon.ready; waitReady polls for it for 5s times the parallel headroom and never looks at the process, so `filter-wire-name-pattern` and `filter-wire-external-fields-refused` each take 15s for a ze that exited in under 1s | open: end the wait when the foreground process exits (select on its exit beside the file poll), and prove it with a runner .ci whose ze exits at once |
| 2026-11-13 | single-owner-lifecycle-5-session-end-locals | `(*Process).startExternal` (`internal/component/plugin/process/process.go`) | A forked plugin whose `run` exits at once is not seen to end: `waiter.Wait` watches only its 30 s timeout on `p.ctx` and the acceptor, never `reaped`, which `monitorCmd` closes at the exit, so the test daemon answered nothing over SSH for 30 s and at stop logged `TLS connect-back: context canceled` | open: end the wait when the child is reaped, and name its exit in the error |
| 2026-11-14 | eor-treats-route-processes-as-peers (walked into) | replay fence, `Peer.lowerReplayFence` | Only an owner's report or the next session lowers the fence; no timer does. An owner that stops or is reloaded away before it reports, a failed report that is only logged (rr `signalSessionReady`, rib `signalReplayReady`, rs `peerAction`), or rr's stale-generation return parks the peer's live forwards for the session, and quiesce on it times out naming the process | open: lower the fence when its owner leaves or its report fails, one test per path |
