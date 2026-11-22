# Handover 57: capability work, the last follow-ups

Written 2026-11-22 on the server (`claude`), main checkout. It follows handover
56, whose six parts landed in d68986a34b, 113f630a83, f1393f1583, 045d38c59c,
099dd14016 and 79486c2894, then 83cc6e092d (the runner's `lacks-caps` option,
the BGP privileged-port refusal test promoted) and 295c4701f3 (the SDK restart
advice and two doctor-code descriptions follow ze.service membership).

## Start here

| # | Work | State | First action |
|---|------|-------|--------------|
| 1 | Doc pages of 83cc6e092d and 295c4701f3 | Written, uncommitted in the working tree: `docs/architecture/testing/ci-format.md`, `docs/architecture/testing/qemu-integration.md`, `docs/functional-tests.md` (the `lacks-caps` option); `docs/guide/plugins.md`, `docs/guide/config-reload.md`, `docs/architecture/config/transaction-protocol.md`, `docs/architecture/api/process-protocol.md` (the restart route per membership, `RunningConfigLostRestart()`). A Bash hook refuses every command naming a `docs/` path while `rfc/short/rfc5492.md` and `rfc/short/rfc7606.md` (another session's) do not parse, so they could not be committed | Once those summaries parse: check each page's diff holds only these changes, run `./le doc check all`, commit them through `./le commit create` |
| 2 | `test/reload/exabgp-bridge-run-reload-refused.ci` is red | Its restart-text steps pass. Its peer-exchange step fails: after the refused reload, the exabgp bridge's mux pipe closes ("mux conn read error: io: read/write on closed pipe") and the helper's `send bgp ... nlri add 1.1.0.0/24` is lost. Cause not established; the commit that promoted it only changed advice text, and another session had uncommitted edits in `internal/component/plugin/server/` in the tree the run built from | Run the test against a build of HEAD, in the main checkout, once no session holds plugin-server edits: red there means a defect in the reload refusal path (fix it); green means it was the other session's change |
| 3 | Two stale restart fixes found by the last agent | `internal/component/ike/engine/doctor_ike_socket_bypass.go` `ikeSocketBypassFix` ends "then restart it: ze signal restart", and the CodeMeta of `doctor-ipsec-ike-socket-bypass` says the same: an unsupervised ze stays stopped after that command. `docs/architecture/doctor-and-health-checks.md` (the plugin-server restart paragraph) still says "systemctl restart ze under systemd" | Route the fix through `privilege.RestartRemedy()` and reword the CodeMeta and the page; test the fix per membership with `privilege.SetServiceMembershipForTest` |
| 4 | Lint of every package the capability work touched | Never run: `./le go lint run scope` refused each time while other sessions edited its dependencies | `./le go lint run scope "./internal/core/privilege/... ./internal/core/network/... ./internal/component/doctor/... ./internal/plugins/trafficusage/... ./internal/component/sysctl/... ./internal/component/ike/... ./internal/component/bgp/reactor/... ./internal/component/l2tp/... ./internal/test/runner/... ./pkg/plugin/sdk/..."` when the tree is quiet; fix what it reports |

## Open for Thomas (unchanged from handover 56)

- Whether `internal/component/iface/register.go` `permissionRemediation` moves to
  `privilege.CapabilityRemedy` (changes `apply_errors_test.go`, which pins "setcap").
- `internal/test/fixture/daemon.log` holds the old warning text, with no reader.

## Defects met, outside this work

- `./le arch tier check` scans `bin/builds/.building`, the temporary source copy a
  running `le` build makes, and reports its files as tier failures.
- `TestCIAcceptOnlyLint` is red on `test/plugin/config-sha-crypt-password-at-boot.ci`
  (another session's committed file).
