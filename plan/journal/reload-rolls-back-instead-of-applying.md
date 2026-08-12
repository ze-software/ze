# Reload rolls back instead of applying

A SIGHUP reload runs its rollback path. The change the operator asked for is not
applied. The log reports the rollback and not the refusal that caused it, so the
test that asserts the applied behavior fails on a missing line. A missing line
says what did not happen. It does not say why.

The rollback was the second-order symptom. The daemon was stopping while the
reload was still running. `Server.Stop` closed every plugin connection
(`internal/component/plugin/process/manager.go`, `ProcessManager.Stop`), and the
in-flight transaction read each closed connection as a crashed plugin
(`internal/component/plugin/server/config_tx_bridge.go`, the `conn == nil` arm
of `subscribePhase`). The daemon stopped because the test runner stopped it, not
because the reload was wrong.

The first-order failure IS logged, at WARN, by `reloadConfig`
(`internal/component/plugin/server/reload.go`): `config reload: transaction
failed`, with the plugin that failed and the phase. It is also printed as
`reload error: ...` by `handleSIGHUPReload` (`cmd/ze/hub/main_reload.go`). The
line was not missing from the daemon. It was missing from the test report, which
prints 30 lines of daemon stderr (`internal/test/runner/report.go`,
`truncateOutput`) and cut it off.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-12 | - | config.transaction | `test/reload/reload-import-policy-no-bounce.ci` fails deterministically, on a suite run and on two single runs. The reload logs `plugin broken during rollback, restarting` for bgp-healthcheck, bgp-hostname and bgp-filter-community, and never logs `peer settings swapped in place` (`internal/component/bgp/reactor/reactor_api.go`, `reconcilePeersJournaled`). Not caused by the plugin-only-boot work: the only Go changes in that tree were the cmd/ze config routing and `internal/component/config/probe.go`, none on the BGP reload path | fixed 2026-08-12. The reload was never wrong: the test killed the daemon in the middle of it. `ze-peer` sleeps a fixed 500ms after it sends the signal (`internal/test/peer/peer.go`, `pauseForSignal`) and then the runner stops the daemon. A reload of this config takes longer than 500ms. This test is the only reload test that can wait for nothing on the wire after the signal, because proving the absence of a bounce means there is no next connection to wait for; its siblings block on the reconnect the restart branch makes. The test now fences on `await=stderr:contains=sighup reload complete`, which is the phrase `cmd/ze/hub/main_reload.go` prints for exactly this purpose. No assertion changed. Proof: 15 green runs after the fence, and one red run with `hotSwappableSettings` mutated to drop `ImportFilters`, which made the daemon log `peer restart required changed=ImportFilters` |

## Shutdown under a running transaction

Fixed 2026-08-12, same class, second defect. Shutdown did not wait for an
in-flight config transaction. `Server.Stop` closed the plugin connections
under a running reload. The bridge then read each closed connection as a
crashed plugin (`internal/component/plugin/server/config_tx_bridge.go`, the
`conn == nil` arm of `subscribePhase`). The orchestrator elected a rollback,
and `handleRollbackAck`
(`internal/component/config/transaction/orchestrator.go`) restarted plugins
the next line was about to kill.

The daemon still exited and no operator state was lost. The cost was a burst
of WARN lines saying a plugin had crashed when nothing had. That is what the
log tells an operator who reads it after a real incident.

The fix names the reason rather than letting the transaction infer one.
`Server.Stop` now cancels the transaction with `transaction.ErrShutdown` as
the context CAUSE. It then waits for the transaction to unwind, bounded by
`txShutdownGrace` (3s), before `cleanup` closes any connection
(`internal/component/plugin/server/reload.go`, `stopTransaction`).

`Execute` skips the abort and the rollback for that cause alone. An ordinary
cancellation still rolls back, a reload's own 30-second deadline included:
that daemon keeps running, so it must be told to undo a half-applied change.
