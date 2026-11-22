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
| 2 | `test/reload/exabgp-bridge-run-reload-refused.ci` was red | Done in b70b9ebac4. The cause was the test's ordering, not the refused reload: the stage script ended before the forked helper's route reached the bridge, and the fixture's shutdown closed the pipe first. Both bridge reload tests now wait for the peer's `updates-sent` to reach 2. Stress run: the unfenced test red on invocation 1, each fenced test clean over 80; both pass in place | None |
| 3 | Stale restart fixes | Done: IPsec in be542dc106; CoPP, fib-vpp, the host trend monitor, gNMI TLS, MCP TLS and GTSM in 09853ff552; the gNMI Set token fix and the command reference's plugin-restart sentences in 12feff8d38. Each goes through `privilege.RestartRemedy()` with a test per membership, and no operator message in `internal/`, `cmd/` or `pkg/` names `ze signal restart` any more | None |
| 4 | Lint of every package the capability work touched | Run for all but two packages, one small scope at a time, and it found nothing in a line this work wrote. Ran: privilege, network, diagnostic, ike, trafficusage, sysctl, sdk, test/runner, plugin/server, l2tp, bgp/reactor. Their findings belong to other work: the `os.Exit` in `register.go` pattern, goimports, misspell and style in other sessions' SDK, plugin-server and reactor tests, unused kernel PPP functions in l2tp mid-rewrite. Not run: `./internal/component/iface/...` and `./internal/component/doctor/...`, refused five times each while other sessions edited files they depend on | `./le go lint run scope "./internal/component/iface/..."` then `./le go lint run scope "./internal/component/doctor/..."` when the tree is quiet; fix what they report in this work's lines (`joinApplyErrors` in iface, `checks_listener.go` and `checks_daemon_capabilities.go` in doctor) |

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
| 5 | Scoped lint of the fixes | Done on 2026-11-22, folder by folder, every folder of the ten packages on Linux. The four findings this work introduced are fixed in 3465713ec0; every remaining finding traces by blame to an older commit. Clean: `internal/plugins/init`, `internal/component/cli/sshclient`, `internal/plugins/host`, `internal/component/command/grammar`, `internal/component/command/registry`, `internal/component/iface/yang`, `internal/component/iface/cli`, and on Linux `internal/component/iface/cmd`. Older findings only: `internal/component/command`, `internal/component/command/commandtest`, `internal/component/cli/client`, `internal/component/iface`, `internal/plugins/iface/netlink`, `internal/plugins/crashes`, `internal/plugins/iface/vpp`, `internal/core/diagnostic`. The macOS pass of the iface packages does not compile: aecb6907c5 made untagged files (`internal/component/iface/doctor_codes.go`, `internal/component/config/system/register_doctor_codes.go`) use constants declared only in `_linux.go` files | None for this work. The macOS break belongs to aecb6907c5's author; macOS-only defects wait until after the first release |
| 6 | Red phase of `test/ui/init-start-cli-answers.ci` | Not walked: the test passed in the QEMU guest with the fix, and was never run with the fix reverted | Revert the populate step of `internal/plugins/init/main.go`, run `./le test qemu ci test test/ui/init-start-cli-answers.ci`, expect "connection refused" at the `ze cli` step, restore |
| 8 | The parse and ui suites for `cli-host-*-show` and `cli-crashes-*` | Not run after 65dfcaa062, which adds a stderr note after a local answer; they assert stdout and exit only | Run those tests by pattern |
| 9 | Two stale texts | The header comment of `test/plugin/cli-refused-login-not-offline.ci` says `runBGP` prints the refusal (now `endFailedAsk`); `docs/contributing/ze-go-style.md` names `./le go style check path <file>`, which `./le` does not have | Correct both |

Answered by Thomas: `ze init` keeps claiming every discovered device by
default, the ones another program owns included, and `ze init --confirm` asks
about each device and writes only the confirmed ones. `--confirm` is
implemented in this work; the ruling is the `## Design decisions` row of
`docs/features/interfaces.md`.
