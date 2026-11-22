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

## Three defects fixed on 2026-11-22, and what they leave

Fixed and committed, unpushed: 65dfcaa062 (an offline fallback learns why the
daemon did not answer; AC-32 of the host trend alerts spec), 5eb1b39c6e (the
interface apply writes a link setting only when the device differs; new doctor
code `doctor-iface-state-unread`), 6c12d53958 (`ze init`'s default profile
writes the SSH listener the CLI connects to).

| # | Left | State | First action |
|---|------|-------|--------------|
| 5 | Scoped lint of the three fixes | Never ran: refused while another session edited `internal/component/bgp/plugins/gr` | `./le go lint run scope "./internal/plugins/init/... ./internal/component/command/... ./internal/component/cli/client/... ./internal/component/cli/sshclient/... ./internal/plugins/host/... ./internal/plugins/crashes/... ./internal/component/iface/... ./internal/plugins/iface/netlink/... ./internal/plugins/iface/vpp/... ./internal/core/diagnostic/..."` when the tree is quiet; fix what it reports |
| 6 | Red phase of `test/ui/init-start-cli-answers.ci` | Not walked: the test passed in the QEMU guest with the fix, and was never run with the fix reverted | Revert the populate step of `internal/plugins/init/main.go`, run `./le test qemu ci test test/ui/init-start-cli-answers.ci`, expect "connection refused" at the `ze cli` step, restore |
| 7 | WireGuard keys and peers are still written on the first apply after start | Open: the write needs privilege even when the device already holds them; comparing needs a read of the device's keys and peers | Read the device's keys and peers and skip `ConfigureWireguardDevice` when they match, as 5eb1b39c6e does for link settings |
| 8 | The parse and ui suites for `cli-host-*-show` and `cli-crashes-*` | Not run after 65dfcaa062, which adds a stderr note after a local answer; they assert stdout and exit only | Run those tests by pattern |
| 9 | Two stale texts | The header comment of `test/plugin/cli-refused-login-not-offline.ci` says `runBGP` prints the refusal (now `endFailedAsk`); `docs/contributing/ze-go-style.md` names `./le go style check path <file>`, which `./le` does not have | Correct both |

Open for Thomas: `ze init` writes every discovered bridge, veth, dummy,
WireGuard and xfrm device into the config, with nothing that excludes a device
another program owns (docker0, `br-*`, virbr0, container veths). A root ze
then brings each up, manages it, and creates a missing one. Keeping all needs
no change. Skipping foreign devices needs an ownership signal discovery does
not read today (the `ze:owned:` alias, the master index, or a kind rule such as
ethernet and loopback only, which `EmitBootstrapConfig` already applies), in
`ze init` alone or in `iface.EmitConfig`, which also changes
`ze interface scan --config`.
