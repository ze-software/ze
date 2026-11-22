# Handover 57: capability work, the last follow-ups

Written 2026-11-22 on the server (`claude`), main checkout. It follows handover
56, whose six parts landed in d68986a34b, 113f630a83, f1393f1583, 045d38c59c,
099dd14016 and 79486c2894, then 83cc6e092d (the runner's `lacks-caps` option,
the BGP privileged-port refusal test promoted) and 295c4701f3 (the SDK restart
advice and two doctor-code descriptions follow ze.service membership).

## Start here

| # | Work | State | First action |
|---|------|-------|--------------|
| 1 | Doc pages of 83cc6e092d and 295c4701f3 | Done: landed in 997b9a8f7a (the `lacks-caps` option: `ci-format.md`, `qemu-integration.md`, `functional-tests.md`) and 6803c60c8a (the restart route per membership: `plugins.md`, `config-reload.md`, `transaction-protocol.md`, `process-protocol.md`) | None |
| 2 | `test/reload/exabgp-bridge-run-reload-refused.ci` was red | Done in b70b9ebac4. The cause was the test's ordering, not the refused reload: the stage script ended before the forked helper's route reached the bridge, and the fixture's shutdown closed the pipe first. Both bridge reload tests now wait for the peer's `updates-sent` to reach 2. Stress run: the unfenced test red on invocation 1, each fenced test clean over 80 | Run `./le test bgp reload --pattern exabgp-bridge` once in place: it was not run after promotion because the daemon did not build under another session's `internal/component/mcp/tools.go` edit |
| 3 | Stale restart fixes | Done for IPsec in be542dc106: every IKE fix that named `ze signal restart` takes `privilege.RestartRemedy()`. The same advice remains in `internal/plugins/copp/doctor.go`, `internal/plugins/fib/vpp/doctor_label.go` and `doctor_route.go`, `internal/plugins/host-cmd/cmd/show_trend.go`, `internal/component/host/trend/register.go`, `cmd/ze/hub/main_system.go`, `cmd/ze/hub/service_mcp.go`, `internal/component/gnmi/tls.go`, `internal/component/gtsm/doctor.go` and the CodeMeta of the gNMI TLS and GTSM codes | Route each through `privilege.RestartRemedy()` the same way, with a test per membership |
| 4 | Lint of every package the capability work touched | Half run. `./internal/core/privilege/... ./internal/core/network/... ./internal/core/diagnostic/... ./internal/component/ike/... ./internal/plugins/trafficusage/... ./internal/component/sysctl/... ./pkg/plugin/sdk/...` ran: 14 findings, none in a line this work wrote (the `os.Exit` in `register.go` pattern, goimports and nilnil in the SDK files of a27a951239, style findings in older sysctl and traffic-usage tests), left to the lint-backlog set. Not run, refused while other sessions edit their inputs: doctor, bgp/reactor, l2tp, test/runner, iface, plugin/server | `./le go lint run scope "./internal/component/doctor/... ./internal/component/bgp/reactor/... ./internal/component/l2tp/... ./internal/test/runner/... ./internal/component/iface/... ./internal/component/plugin/server/..."` when the tree is quiet; fix what it reports in this work's lines |

## Decided by Thomas

- `permissionRemediation` moved to `privilege.CapabilityRemedy`: 6672adaa67.
- `internal/test/fixture/daemon.log` removed: 6a59cae727.

## Defects met, outside this work

Both were small and are fixed in the working tree, awaiting their test runs and
commit by the main thread (no journal row: neither is open).

- `./le arch tier check` scanned `bin/builds/.building`, the temporary source copy a
  running `le` build makes, and reported its files as tier failures. Fixed:
  `collectSourceEdges` (`internal/le/arch/tier/tier.go`) skips the build store
  `toolchain.BuildsDir` answers; proof `TestTheEdgeScanSkipsTheLauncherBuildStore`.
- `TestCIAcceptOnlyLint` was red on `test/plugin/config-sha-crypt-password-at-boot.ci`:
  its one `expect=exit:code=0` delegates every check to the compiled fixture
  `pluginSHACryptPasswordAtBoot`, which exits non-zero on any failed check. Fixed:
  the file carries the `# accept-only:` annotation naming that fixture.
