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

## Shutdown before the reload reported

Fixed 2026-08-12, same class, third defect. The wait above starts at the
transaction, and a reload refused before it reaches one never gets there. The
config parser, the value validators, the PKI load and the provider snapshot all
decide inside `runReload` (`cmd/ze/hub/main_reload.go`), which returns to
`handleSIGHUPReload` to be printed as `reload error: ...`. Nothing waited for
that goroutine: `runYANGConfig` closed `reloadCh` and went straight on to
shutdown, so the process could exit with the verdict unprinted.

An operator who sent SIGHUP and then SIGTERM saw `received SIGHUP, reloading
config...` as the daemon's last word, and never learned whether the config was
applied or refused.

`awaitReloadWorker` now waits for the worker to drain and return, bounded by
`reloadShutdownGrace` (3s, matching `txShutdownGrace`), and says so on stderr
when a reload outlasts the grace.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-12 | - | hub SIGHUP shutdown | `test/reload/config-reload-invalid-validator.ci` failed under suite load: the daemon printed `received SIGHUP, reloading config...` and then the Cease notification of its own shutdown, with no `reload: parse config: config validation failed` in between, so the refusal the test asserts was never on stderr. `ze-peer` sends SIGTERM a fixed 500ms after SIGHUP (`pauseForSignal`, `internal/test/peer/peer.go`), which is normally far more than the refusal needs | fixed: `awaitReloadWorker`. No assertion changed and the `.ci` is untouched. The `await=stderr` fence that fixed the sibling test does not apply here: that test's SIGTERM comes from the runner, this one's comes from the peer, so a runner-side fence cannot delay it. The runner's await arm also REPLACES the `fgProc.Wait()` that `expect=exit:code=0` reads, so adding one here would have made this test's liveness half assert nothing (`runner_exec.go`, the switch at the `awaitStderrSW != nil` arm). No `.ci` pairs the two today. Proof: with `pauseForSignal` cut to 1ms the unfixed daemon fails 9 times in 44 runs and the fixed one is 40/40; at the real 500ms, the retired `ze-functional-reload-test` (current: `./le functional reload`) is 40/40, three serial suite passes are 120/120, and five passes at `-p 20` keep this test at 5/5. Red when `refuseInvalidCustomSections` is made to accept the invalid plugin name: 0/3 |
| 2026-08-25 | - | traffic plugin reload apply | `test/traffic/traffic-reload-apply.ci` fails about one run in eight with `config apply partial failure: plugin traffic apply failed: traffic-control reload: context canceled`, so the reload transaction rolls back and `traffic-control config reloaded` never reaches stderr. Reproduced at HEAD on the unconverted file, 1 failure in 8 isolated runs, so it predates the conversion of that driver's blind holds to `wait_for_daemon_ready` | not fixed: found while doing that conversion, which does not touch the reload path. The converted file was 8/8 in the same loop |
