# Handover 58: the red native PPP and CPE tests in the netlink backend

Written 2026-11-22 on the server (`claude`), main checkout, by the session
that cleared the netlink lint findings (acbcffd369). Nothing here is pushed.
Thomas's order: fix the PPP settlement-history defect, then make the one-line
CPE fixture fix and run the CPE tests verbosely to see what is behind them; the
kernel option waits for his next kernel rebuild.

The owning spec is `plan/post-release/features/spec-pppoe-vpp-dataplane.md`
(in-progress, phase 2/7, paused since 2026-11-19, no live claim). Claim it
before editing: `./le spec claim spec-pppoe-vpp-dataplane.md`. Its rows
"Provider history defect" (line 33), "Native validation still owed" (line 32)
and "Current netlink checks" (line 64) describe this work.

The causes below were found by reading on 2026-11-22; nothing was run to
confirm them except where the table says so.

## Start here

| # | Work | State | First action |
|---|------|-------|--------------|
| 1 | PPP settlement history lost on close (3 tests: `TestNativePPPModelSettlementHistorySurvivesPhysicalClose`, untagged; `TestNativePPPProviderSettledACKHistorySurvivesClose` and `...NACKHistorySurvivesClose`, integration tag) | Cause read and confirmed at the line. `(*nativePPPWriter).closeOriginal` (`internal/plugins/iface/netlink/native_ppp_linux.go`, `w.closeErr = w.socketCloseErr`) replaces the stored settlement history with the socket close result, nil on a clean close, where `settleRetiredUnitLocked` joins. Second half: `(*nativePPPTransaction).receive` (`native_ppp_transport_linux.go`, end) clears `writer.pending` and returns the joined wait causes or NACK but never stores them in `writer.closeErr`, so a later Settle or Close returns nil. 39848b88c6 says only a delayed settlement should record history ("Immediate NACK behavior stays") | Join instead of overwrite in `closeOriginal`; record the delayed settlement's causes in `receive`. Then `go test -race -run '^TestNativePPPModelSettlementHistorySurvivesPhysicalClose$' ./internal/plugins/iface/netlink/` (red today) and `go test -race -tags integration -run '^TestNativePPPProviderSettled(ACK|NACK)HistorySurvivesClose$' ./internal/plugins/iface/netlink/ -args -ppp-provider-required=true`, both through `./le job run` |
| 2 | CPE fault tests time out (7 tests in `native_cpe_fault_integration_linux_test.go`) | Likely cause, not observed: `newCPENativeFault` (`native_cpe_fault_fixture_integration_linux_test.go`) never calls `iface.LoadBackend("netlink")`, confirmed absent by grep. `Dialer.dialPrepared` resolves the source interface first (`pppoe.ResolveInterface` -> `iface.Resolve` -> `GetLink` -> `backendOrErr`), which fails with no backend, so the bind the test waits 30 s for never starts and cleanup reports "lost custody: <nil>". Sibling tests in `internal/component/l2tp/pppoeclient/*_integration_linux_test.go` do load it. The terminal-cleanup assertions also depend on item 1 | First prove the cause: run `go test -race -tags integration -v -run '^TestNativeCPEPartialBindTerminalCleanup$' ./internal/plugins/iface/netlink/` with the first dial error printed; expect "no backend loaded". Then add the backend load to the fixture, mirroring the pppoeclient tests, and run the seven. These tests have never run on Linux; expect further reds, each investigated, not assumed |
| 3 | `TestNativeCPERegistryRestartRetainsFailedCloseOriginal` | Undecided. Its backend is loaded through config, and its sibling `...RegistryReadmitsOnlyAfterOriginalPhysicalCleanup` passes; the difference is the `cpeSocketClose` fault (`pppcpefault.DenyOriginalClose`, seccomp with TSYNC) | Run it alone with `-v` and read which wait fails and the first dial error |
| 4 | `TestNativeAccessLateCreateKeepsOriginalOutcomeAndForwardingSurvivor`, `TestNativeAccessLateEchoAllowsOnlyOriginalAbsenceRetirement` | Undecided; `nativeVLANTransport.request`/`resume` and the worker read consistent with both tests, and their failing lines were not captured | Run them alone with `-v` and read the failing assertions |
| 5 | `TestVLANQoSDSCPFullChain`, `TestVLANQoSEgressPCPOnWire` (`nft: exit status 1`) | Cause confirmed: this VM's kernel has `# CONFIG_NF_TABLES_NETDEV is not set` and `gokrazy/kernel/kernel.config` lacks it; the tests load `table netdev`. Owned by journal row `plan/journal/unwired-feature.md` (2026-10-28) and `plan/release/validation/spec-kernel-capability-gate.md` (blocked) | Thomas's call: add the option to `kernel.config` and `kernel.require` with the next kernel rebuild, not now, because the rebuild reboots a VM other sessions use |

## Evidence of today's state

All 17 integration failures and the one unit failure were seen in one run on
2026-11-22 after acbcffd369, whose edits in these files are lint-only and were
read to be behaviour-neutral; the unit failure was already red that morning,
before any edit. A before/after run with those edits set aside was refused by
the permission system.
