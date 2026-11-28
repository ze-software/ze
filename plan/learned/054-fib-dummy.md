# 054: FIB dummy backend (`spec-fib-dummy`) follow-up and verification record

Spec: `spec-fib-dummy` (closed 2026-11-25 in `3f3e418525` and `4cbf5dbf47`; follow-up verification completed 2026-11-28 on `claude` Linux `x86_64`).

## Closure corrections

### 1. Goal Validation row corrected (`Block added and removed by reload`)

The closure record in `3f3e418525` claimed that the remove path of `"Block added and removed by reload with exact forwarding plugins running"` was covered by `-race` unit tests and `"run in CI"`. Both claims were corrected after running the two guest tests directly inside Ze's QEMU guest (`kernel 7.3.0-ze`, `sha256 ec17f2ec70f1be9dac775af52ef836f3eb0d5b3a6d5cf6dff0ba239f1f212964`, docker-host image `cf22e23c3e55c10d` built with `./le setup docker host build arch amd64`):

| Goal (from Task) | Evidence Type | Concrete Evidence |
|---|---|---|
| Block added and removed by reload with exact forwarding plugins running | Functional `.ci` (host + QEMU guest) + `-race` unit tests | **Add & replace paths (host):** `test/reload/fib-dummy-reload-add.ci` PASS (`32.6s`, stops engine-added `fib-kernel`, refuses `request plugin restart fib-kernel`, surfaces LDP refusal), `test/reload/fib-internal-endpoint-replaced-by-dummy.ci` PASS (`63.5s`, stops bus endpoint `k`, unrelated reload churns nothing), `test/reload/fib-dummy-beside-writer-reload-refused.ci` PASS (`5.9s`). **Remove paths (QEMU guest, `caps=net-admin`):** `./le test qemu ci test test/reload/fib-dummy-reload-remove.ci` PASS (`17.8s 1/1 PASS 76 fib-dummy-reload-remove`: removing `fib { dummy { } }` by SIGHUP starts `fib-kernel`, stops `fib-dummy`, installs `198.18.206.0/24` into Linux, replays LDP's refused label into `ip -f mpls route show`, and clears `doctor-fib-mpls-no-writer`); `./le test qemu ci test test/reload/static-removed-stops-engine-added-writer.ci` PASS (`10.8s 1/1 PASS 152 static-removed-stops-engine-added-writer`: removing `static` withdraws `198.18.205.0/24` from Linux and stops engine-added `fib-kernel`). |

### 2. False `"run in CI"` claim corrected

Neither `test/reload/fib-dummy-reload-remove.ci` nor `test/reload/static-removed-stops-engine-added-writer.ci` ran in any of the last three `verify` workflow runs on `main` (`46054367870`, `46014310208`, `45970508314`):

- Run `46054367870` (`0a2661a511`): all 6 shards failed during `./le` bootstrap before running any stage (`internal/component/plugin/server/command_timeout.go:172: d.handlerTimeoutMu undefined`, introduced by `ebe5ba49ae`).
- Runs `46014310208` (`7d7fc77bab`) and `45970508314` (`7ae25509b6`): shards 1–2 timed out / cancelled at 3h; shards 3–6 failed on unrelated lint, `fs-persistence`, `rfc check`, `repo check tree`, `doc wiring`, and `doc check all` stages. Only `internal/plugins/fib/dummy` unit tests (`ok 1.2s`) ran; the `reload` functional suite never executed in CI.
- **CI findings belonging to `spec-fib-dummy` (`8026918d27`):** shard 1 (`./le go lint run`) reported 5 findings in `internal/test/fixture/misc_fixture_vpp.go` (`errorlint` `%v` on `runErr`, `gosec` G101 on `passHash`, `goconst` on `ip_route_add_del_v2`, `XDG_DATA_HOME`, and `testpass`), all fixed in `ec1a316838` alongside the VPP fixture strengthening. Shard 6 (`./le doc check all`) also noted `../gh-pages/reference/command-equivalents/show-fib-dummy{,-counters}/` missing from the separate `gh-pages` site checkout (shared with `show fleet *` and `request system trace snapshot` because `gh-pages` has not been rebuilt).

## Recorded red runs for all 14 functional `.ci` tests

The 12 host `.ci` tests were walked with `./le test redwalk check` (Go `-overlay` leaving the shared tree untouched). The 2 `caps=net-admin` reload `.ci` tests were walked inside Ze's QEMU guest with `./le test qemu ci test <path>` against a temporary `// MUTATION-APPLIED` edit restored from a pristine copy immediately after each run.

During this walk, `test/vpp/fib-dummy-vpp-backend.ci` stayed green when `ProgramsFIB: true` was reverted because `vppFIBDummyDriver` (`internal/test/fixture/misc_fixture_vpp.go`) configured only `bgp-rib` and `connected` (neither declares `NeedsDataPlane`), so `fib-vpp` was never auto-loaded even without `fib-dummy`. Adding a `static` route (`10.21.0.0/24`) to `vppFIBDummyDriver` made the test exercise `fib-vpp` auto-load suppression and fail under the break.

| # | `.ci` test | Reverted behavior | Clean PASS | Broken FAIL | Observed failure line |
|---|---|---|---|---|---|
| 1 | `test/plugin/fib-dummy.ci` | `dummyFIB.applyBatch` returns early (`internal/plugins/fib/dummy/fibdummy.go`) | `21.7s 1/1 PASS 556` | `51.5s 1/1 FAIL 556` | `cmd seq=6 (le test fixture daemon/await-answer-all "show fib dummy" 192.0.2.0/24 198.51.100.0/24 203.0.113.0/24): expected exit code 0, got 1` |
| 2 | `test/plugin/fib-dummy-forked.ci` | `dummyFIB.handleBestChangeJSON` returns `nil` (`internal/plugins/fib/dummy/fibdummy.go`) | `47.1s 1/1 PASS 554` | `53.2s 1/1 FAIL 554` | `cmd seq=6 (le test fixture daemon/await-answer-all "show fib dummy" 192.0.2.0/24 198.51.100.0/24 203.0.113.0/24): expected exit code 0, got 1` |
| 3 | `test/static/static-fib-dummy-holds-route.ci` | `ProgramsFIB: false, NoDataPlaneChoice: false` (`internal/plugins/fib/dummy/register.go`) | `12.2s 1/1 PASS 4` | `59.6s 1/1 FAIL 4` | `cmd seq=8 (ze cli -c "show fib kernel"): expected exit code 1, got 0` |
| 4 | `test/plugin/fib-dummy-labels-refused.ci` | `dummyFIB.handleMPLSEntry` acknowledges `nil` (`internal/plugins/fib/dummy/labels.go`) | `30.3s 1/1 PASS 555` | `69.9s 1/1 FAIL 555` | `cmd seq=3 (le test fixture daemon/await-answer-all "show doctor | json" doctor-fib-mpls-no-writer "fib dummy programs no label"): expected exit code 0, got 1` |
| 5 | `test/plugin/fib-dummy-doctor.ci` | `checkFIBDummyHoldsRoutes` returns `nil` (`internal/plugins/fib/dummy/doctor.go`) | `2.0s 1/1 PASS 553` | `2.0s 1/1 FAIL 553` | `cmd seq=1 (ze doctor --json config dummy-with-static.conf): output does not contain "\"code\": \"doctor-fib-dummy-holds-routes\""` |
| 6 | `test/plugin/perf-route-path-dut.ci` | Skip `sysribevents.BestChange.Subscribe` in `OnStarted` (`internal/plugins/fib/dummy/register.go`) | `41.1s 1/1 PASS 928` | `68.0s 1/1 FAIL 928` | `cmd seq=6 (le test fixture daemon/await-answer-all "show fib dummy" 192.0.2.0/24 198.51.100.0/24): expected exit code 0, got 1` |
| 7 | `test/parse/fib-dummy-beside-cli-plugin-refused.ci` | Drop `--plugin` fallback in `resolveFIBPluginUse` (`internal/component/config/dataplane_verdict.go`) | `433ms 1/1 PASS 181` | `463ms 1/1 FAIL 181` | `✗ fib-dummy-beside-cli-plugin-refused: seq 1: expected exit code 1, got 0` |
| 8 | `test/parse/fib-dummy-beside-writer-refused.ci` | `evaluateNoPlaneFIBChoice` returns `nil` (`internal/component/config/dataplane_verdict.go`) | `1.8s 1/1 PASS 182` | `429ms 1/1 FAIL 182` | `✗ fib-dummy-beside-writer-refused: seq 1: expected exit code 1, got 0` |
| 9 | `test/reload/fib-dummy-beside-writer-reload-refused.ci` | `evaluateNoPlaneFIBChoice` returns `nil` (`internal/component/config/dataplane_verdict.go`) | `5.9s 1/1 PASS 74` | `3.6s 1/1 FAIL 74` | `cmd seq=4 (le test fixture plugin/bus-sighup-reload-refused config2.conf ze-bgp.conf reload2.failed): expected exit code 0, got 1` (`ZE-OBSERVER-FAIL: SIGHUP reload of config2.conf ended with last-outcome="applied" (generation=1), want failed`) |
| 10 | `test/reload/fib-dummy-reload-add.ci` | Skip stopping non-chosen `runningFIB` in `judgeReloadWriters` Row 2 (`internal/component/plugin/server/reload_dataplane_writer.go`) | `32.6s 1/1 PASS 75` | `20.1s 1/1 FAIL 75` | `cmd seq=8 (ze cli -c "show fib kernel"): expected exit code 1, got 0` |
| 11 | `test/reload/fib-internal-endpoint-replaced-by-dummy.ci` | `effectiveReloadSelections` ignores `candidateSelection` (`internal/component/plugin/server/reload_dataplane_writer.go`) | `63.5s 1/1 PASS 77` | `31.3s 1/1 FAIL 77` | `cmd seq=7 (ze cli -c "show system subsystem list | json"): output matches forbidden regex "\"name\": ?\"k\"" on stdout` |
| 12 | `test/vpp/fib-dummy-vpp-backend.ci` | `ProgramsFIB: false, NoDataPlaneChoice: false` (`internal/plugins/fib/dummy/register.go`), after strengthening `vppFIBDummyDriver` with `static` | `6.5s 1/1 PASS 1` | `9.2s 1/1 FAIL 1` | `ZE-OBSERVER-FAIL: fib-vpp must not run beside fib-dummy` |
| 13 | `test/reload/fib-dummy-reload-remove.ci` (QEMU) | `judgeReloadWriters` Row 3 returns before `ConfigDataPlane` (`internal/component/plugin/server/reload_dataplane_writer.go`) | `17.8s 1/1 PASS 76` | `56.5s 1/1 FAIL 76` | `cmd seq=8 (le test fixture daemon/await-answer-all "show fib kernel" 198.18.206.0/24): expected exit code 0, got 1` |
| 14 | `test/reload/static-removed-stops-engine-added-writer.ci` (QEMU) | `judgeReloadWriters` Row 4 returns before stopping `runningFIB` (`internal/component/plugin/server/reload_dataplane_writer.go`) | `10.8s 1/1 PASS 152` | `12.3s 1/1 FAIL 152` | `cmd seq=9 (ze cli -c "show fib kernel"): expected exit code 1, got 0` |

## Lessons

- **A cold kernel or docker-host cache is a build step, not a reason to skip guest tests.** Building the cache once (`./le setup docker host build arch amd64`) unblocked both `caps=net-admin` reload tests directly inside Ze's QEMU guest.
- **Never cite CI without inspecting an actual passing run.** All three recent `verify` runs on `main` failed on unrelated errors before the `reload` suite ever ran.
- **Every new `.ci` test needs a forced red run (`./le test redwalk check` or QEMU mutation).** Running the red walk caught a real vacuity bug in `test/vpp/fib-dummy-vpp-backend.ci`: without a `NeedsDataPlane` producer (`static`), `fib-vpp` was never auto-loaded even with `ProgramsFIB: false`.
- **Edit docs in the phase that changes the behavior and trim handovers at closure.** Leaving `docs/DESIGN.md`, `doctor-and-health-checks.md`, stale anchors, and the handover file until closure let wrong state linger after the spec closed.
