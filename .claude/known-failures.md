# Known Test Failures

Pre-existing failures that need fixing. Each entry includes failure output and root-cause hypothesis.
Sessions should attempt to fix entries here before logging new ones.

Remove entries once fixed.

## TestSSEMultiLineData / TestSSEServeHTTP (timeout) -- FIXED 2026-04-07

**File:** `internal/chaos/web/sse_test.go`
**Symptom:** Test occasionally hung indefinitely in `resp.Body.Read(buf)` waiting for SSE data that was never delivered.
**Root cause:** The test ordering was: issue HTTP GET, then `broker.Broadcast(...)`, then `resp.Body.Read(...)`. `http.DefaultClient.Do` returns as soon as the response headers are flushed (`flusher.Flush()` at `sse.go:123`), BEFORE the server-side `ServeHTTP` calls `b.Subscribe()` at `sse.go:125`. If the test's `Broadcast` ran in that gap, the event was sent to zero clients (Broadcast iterates `b.clients` which was still empty), and the test reader blocked forever waiting for data that was never queued.
**Fix:** Added a `waitForClient(t, broker)` helper that polls `broker.ClientCount()` until at least one client is registered (1s timeout). Both `TestSSEMultiLineData` and `TestSSEServeHTTP` call it after `Do` and before `Broadcast`.
**Verification:** `go test -race -count=20 ./internal/chaos/web/` -> all green.

## TestBackpressureNoResumeAbove10Percent (flaky) -- FIXED 2026-04-07

**File:** `internal/component/bgp/plugins/rs/worker_test.go`
**Frequency before fix:** ~1 in 3 runs.
**Root cause:** The test dispatched 11 items into a cap-10 channel and assumed items 2-11 would all queue before the worker dequeued any -- so `depth >= cap` would be observed by `checkBackpressure` and `inBackpressure` would be set. Then when items drained below 10%, the low-water `LoadAndDelete(key)` would find the flag and fire the callback. But the worker goroutine could dequeue items in flight: if it ran fast enough, the channel never reached capacity, `inBackpressure` was never set, and the final `LoadAndDelete` returned `wasBP=false` -- so `onLowWater` was never called even when depth reached 0. The test then failed at the `require.Eventually` polling for `lwCalls >= 1`.
**Fix:** Added a `gate1` channel that parks the handler on item 1 until the dispatch loop has put all 11 items in queue. This guarantees the channel reaches capacity (item 11 spills to overflow), `checkBackpressure` fires, and `inBackpressure` is set. After `BackpressureDetected(key)` is asserted, `gate1` is closed and the original `gate6` flow continues. Two gates are independent so the deferred close handles both safely.
**Verification:** `go test -race -count=20 -run TestBackpressureNoResumeAbove10Percent ./internal/component/bgp/plugins/rs/` -> 20/20 pass; `go test -race -count=3 ./internal/component/bgp/plugins/rs/` -> green.

## TestFwdPool_Barrier_WithOverflow (flake under parallel load) -- logged 2026-04-11

**File:** `internal/component/bgp/reactor/forward_pool_barrier_test.go:170`
**Symptom:** Test fails with `"bufmux: double return detected"` error log and `--- FAIL: TestFwdPool_Barrier_WithOverflow (0.05s)`. A buffer in the `bgp.reactor.forward` BufMux pool is being released twice by the forward-pool worker path.
**Reproduction conditions:**
- FAILS: `go test -race -count=5 ./internal/chaos/inprocess/... ./internal/component/bgp/reactor/...` (combined, two test binaries racing for CPU).
- PASSES: `go test -race -count=10 ./internal/component/bgp/reactor/...` alone (30.99s, 0 failures).
- PASSES: `go test -race -count=10 -run TestFwdPool_Barrier_WithOverflow ./internal/component/bgp/reactor/...` (isolated, 0 failures).
- The failure only surfaces under multi-package parallel test-binary load, likely because the inprocess binary starves the reactor binary of CPU/scheduler slots and exposes a timing window in the forward pool worker's release path.
**Not caused by the 2026-04-11 `s.bufReader` race fix.** That fix is in `session_read.go` and `session.go`; this failure is in `forward_pool.go` and the BufMux double-return guard. Verified orthogonal.
**Hypothesis:** The test dispatches 2 items via `TryDispatch` (fills chanSize=2 channel) and 2 via `DispatchOverflow`, with a blocked handler. When the handler is released, 4 items are processed. The race is likely in how the worker drains the channel + overflow — possibly a drain that reads a slot twice, or an overflow-to-channel transfer that releases the buffer both on channel-pop and overflow-pop. The error originates from `BufMux.ReleaseBlock` detecting a second return for the same block ID.
**Needs:** Read `internal/component/bgp/reactor/forward_pool.go` worker drain loop and trace where each `fwdItem`'s BufHandle is released. Likely a 30-60 min investigation. Parking here until someone picks it up.
**Suggested first check:** `grep -n "returnBlock\|ReleaseBlock\|buf\.Release" internal/component/bgp/reactor/forward_pool*.go` — every release site needs a "released-once" invariant under the worker drain.

**UPDATE 2026-04-11 (sibling failure surfaced by `make ze-race-reactor`):**
The new `ze-race-reactor` Makefile target (`go test -race -count=20
./internal/component/bgp/reactor/...`) immediately surfaced a second
forward-pool failure on its first run:

```
--- FAIL: TestFwdPool_SupersedingDifferentKeys (0.00s)
    forward_pool_supersede_test.go:135:
        Error: Not equal: expected 2, actual 1
ERROR msg="bufmux: double return detected" subsystem=bgp.reactor.forward idx=0 blockID=0
```

Same root error class (`bufmux: double return detected` from
`forward_pool.go`), different test, fires on a single-package run rather
than only under multi-package contention. Strongly suggests the worker
drain / release path has a second double-release window beyond the one
hypothesised above. Adding to this entry rather than creating a new one
because the symptom and likely root cause overlap.

**Action:** when someone picks this up, fix both
`TestFwdPool_Barrier_WithOverflow` and `TestFwdPool_SupersedingDifferentKeys`
in the same investigation. They almost certainly share root cause.

## TestInProcessSpeed (race) -- FIXED 2026-04-11

**File:** `internal/chaos/inprocess/runner_test.go:218`
**Original failure:** DATA RACE between `bufio.(*Reader).Read` in `session_read.go:104` (goroutine running `readAndProcessMessage`) and `runtime.slicecopy` in another goroutine accessing the same `bufio.Reader` buffer.
**Reproduction attempts (2026-04-07):** Could not reproduce in 40+ runs across `go test -race -count=20 -run TestInProcessSpeed`, `-count=5 -parallel=8` of the full inprocess package, and `-count=2` of the combined `inprocess` + `bgp/reactor` packages. The race was dormant under current test scenarios but the underlying unsynchronized access still existed.
**Re-reproduction attempts (2026-04-11):** Another 190+ runs (`-count=20` sequential, `-count=5 -parallel=8` full package, `-count=2` combined, `-count=50 -cpu=1,4,8` GOMAXPROCS sweep). Zero hits again — the race was not being TRIGGERED by current test scheduling, but the unsynchronized field access was a real race by the Go memory model regardless.
**Root cause:** `s.bufReader` (`*bufio.Reader`) was WRITTEN at `session_connection.go:255` inside `connectionEstablished` under `s.mu.Lock()`, but READ at `session_read.go:60` and `:104` inside `readAndProcessMessage` with NO lock. The Run loop at `session.go:620` captured `s.conn` under `s.mu.RLock()` and passed it as a parameter, but `readAndProcessMessage` then dereferenced `s.bufReader` directly from the field. The locked write vs unlocked read was a textbook Go data race on the field, even when the specific scheduling in `TestInProcessSpeed` (no reconnects) kept the values consistent. The detector's "runtime.slicecopy" stack trace was the race surfacing inside `bufio.Reader.Read`'s internal `fill()` path, which calls `copy()` to slide unread bytes — this runs under the Reader pointer that was just unlockedly loaded from the field.
**Fix:** Mirror the existing `conn` discipline. Capture both `s.conn` AND `s.bufReader` under the same `s.mu.RLock()` critical section in the Run loop and in `ReadAndProcess`, then pass `bufReader` through `readAndProcessMessage` as a parameter. The function now takes `(conn net.Conn, bufReader *bufio.Reader)` and uses the parameter instead of the field. Three call sites updated: Run loop (`session.go:620`), `ReadAndProcess` wrapper (`session_read.go:26`), and the direct test caller (`session_read_test.go:99`).
**Sibling fix (s.bufWriter):** Review flagged the same race pattern on `s.bufWriter`. It was written by `connectionEstablished` under `s.mu.Lock()` but read by senders (session_write.go, ~11 call sites) while holding `s.writeMu`. The two locks did not serialize the access. Resolved by nesting `s.writeMu.Lock()` inside `s.mu.Lock()` in `connectionEstablished` (lock ordering `s.mu → s.writeMu` already established at `closeConn:382-386`). The writeMu section covers only the field assignment, released before `sendOpen` to avoid re-entrancy deadlock (sendOpen re-acquires writeMu). Invariant comment added at `session_connection.go:251` stating that conn, bufReader and bufWriter MUST be assigned in a single critical section so readers capturing them under `s.mu.RLock()` always see a consistent triple.
**Verification:** `go test -race -count=5 ./internal/component/bgp/reactor/...` → ok (16.5s). `go test -race -count=10 ./internal/component/bgp/reactor/...` → ok (30.99s). `make ze-verify` full tree → ok.

## Mass functional test failures: encode + plugin + reload (~200 tests) -- FIXED 2026-04-07

**Suites:** `make ze-encode-test`, `make ze-plugin-test`, `make ze-reload-test`
**Files:** all of `test/encode/*.ci`, most of `test/plugin/*.ci`, all of `test/reload/*.ci`

The original mass-failure cluster (200+ timing-out tests) decomposed into five
independent regressions, all introduced between commits `58564e0a` and
`7c5f8d89`. Each is fixed below.

### 1. Bridge-mode mux read loop (~200 tests, all suites)

**Symptom:** ze daemon exited within ~70ms after `"bgp peers started"`, with every plugin logging `"rpc runtime: read failed" error="mux conn closed"`.
**Root cause:** Commit `58564e0a` (refactor: generic callback registry) made the SDK close its end of the mux after bridge transport negotiation. The server-side dispatcher `handleSingleProcessCommandsRPC` in `internal/component/plugin/server/dispatch.go` was not updated -- it kept reading the mux, saw the planned close as `ErrMuxConnClosed`, returned via `defer cleanupProcess`, and decremented `s.wg`. ALL internal plugins switching to bridge simultaneously dropped `s.wg` to zero, `apiServer.Wait()` returned, `waitLoop` exited via `doneCh`, and the daemon shut down before peers could connect.
**Fix:** Added `HasBridge()` accessor to `internal/component/plugin/ipc/rpc.go:PluginConn`. In `handleSingleProcessCommandsRPC`, when `conn.HasBridge()` is true, skip the mux read loop and block on `s.ctx.Done()` so the WaitGroup entry persists until shutdown. Plugin->engine RPCs continue to flow via `DirectBridge` (wired by `wireBridgeDispatch`).

### 2. Duplicate SIGHUP signal handler (6 reload tests)

**Symptom:** After Fix 1, six reload tests still failed: `3 reload-add-peer`, `4 reload-add-route`, `7 reload-rapid-sighup`, `A reload-restart-peer`, `G test-tx-protocol-exclusion`, `I test-tx-protocol-sighup`. ze stderr showed `"config reload failed: config reload already in progress"` or `"transaction in progress, queuing SIGHUP..."` repeating indefinitely.
**Root cause:** Two `signal.Notify` calls registered for SIGHUP -- the hub (`cmd/ze/hub/main.go:384`) and the BGP reactor's own `SignalHandler` (`internal/component/bgp/reactor/signal.go:93`). Go's `signal.Notify` fans out to every channel. Both handlers raced for the new `txLock.tryAcquire` (introduced in commit `559b3e9b`). Before the txLock the handlers used `sync.Mutex` which serialized them; after, the loser returned `ErrReloadInProgress`. The reactor's duplicate handler is a legacy from before the hub owned signals.
**Fix:** Wrap `r.startSignalHandler()` at `internal/component/bgp/reactor/reactor.go:849` in `if !r.externalServer`. When BGP runs as a config-driven plugin under the hub (`externalServer == true`), the hub owns all OS signal handling.

### 3. BGP plugin only the last "bgp"-affected plugin gets apply (all 6 reload tests above)

**Symptom:** Even after Fix 2, the reload completed without invoking the BGP plugin's `OnConfigApply` -- the daemon logged `"config reload: apply phase plugins=11"` but never `"bgp config applied via transaction"`.
**Root cause:** The "apply BGP last" reorder in `internal/component/plugin/server/reload.go` identified the BGP plugin via `slices.Contains(WantsConfigRoots, "bgp")`. ALL eleven BGP-related plugins (`bgp`, `bgp-rib`, `bgp-watchdog`, `bgp-gr`, `bgp-rpki`, `bgp-hostname`, `bgp-route-refresh`, `bgp-filter-community`, `bgp-llnh`, `bgp-role`, `bgp-softver`, `bgp-healthcheck`) declare `WantsConfig=["bgp"]`, so all eleven were deferred and only the LAST one in iteration order was applied.
**Fix:** Identify the BGP daemon by name (`ap.proc.Name() == "bgp"`) instead of by config root membership. The other ten still apply in the first pass.

### 4. BGP plugin verify unwrap (all 6 reload tests above)

**Symptom:** With Fix 3 in place, `OnConfigVerify` was called but `pendingTree` ended up empty, so `ReconcilePeersWithJournal` reconciled to zero peers and no peer was added/restarted on reload.
**Root cause:** `internal/component/bgp/plugin/register.go:OnConfigVerify` did `tree["bgp"]` after unmarshaling `s.Data`, but `s.Data` is already the BGP subtree (produced by `ExtractConfigSubtree(configTree, "bgp")` in the server) -- not wrapped in another `"bgp"` key. The double unwrap returned nil, defaulted to an empty map, and persisted as `pendingTree`.
**Fix:** Use the unmarshaled value directly as `bgpTree` (no `tree["bgp"]` extraction).

### 5. Persist plugin "bgp cache" prefix (1 reload test: `1 persist-across-restart`)

**Symptom:** The persist plugin logged `updateRoute failed ... "bgp cache N retain" ... rpc error: unknown command`.
**Root cause:** Commit `c59b2ff5` (refactor: remove "bgp " prefix from dispatch chain) removed the `bgp` prefix from the dispatch chain, but the persist plugin's seven hardcoded `"bgp cache %d retain|release|forward"` strings in `internal/component/bgp/plugins/persist/server.go` were not updated.
**Fix:** Drop the `bgp ` prefix from all seven `fmt.Sprintf` calls.

### 6. Per-phase ProcessManager replacement (`1 persist-across-restart`)

**Symptom:** With Fix 5, the persist plugin's cache calls dispatched correctly but the test still failed because the SIGHUP-triggered reload reported `"config reload: no affected plugins, updating config"` -- as if no plugins wanted the BGP root, despite eleven being loaded.
**Root cause:** `internal/component/plugin/manager/manager.go:spawnProcesses` created a NEW `process.ProcessManager` for every `SpawnMore` call (one per startup phase: explicit plugins, auto-load config-paths, auto-load families, etc.) and stored them in a slice. `GetProcessManager()` returned only the LAST one. Tests with both an external `plugin {}` block AND auto-loaded BGP plugins ended up with the BGP plugins in an earlier procManager that was no longer reachable -- `pm.AllProcesses()` returned only the persist plugin from the last spawn batch.
**Fix:** Use a single shared `process.ProcessManager` across all phases. Added `StartMore(configs)` to `internal/component/plugin/process/manager.go` that appends new configs and starts them under the existing context. `manager.spawnProcesses` calls `StartWithContext` on the first spawn and `StartMore` on subsequent ones, so `pm.processes` accumulates every plugin from every phase.

### 7. MVPN config parser used legacy family name (1 encode test: `Q mvpn`)

**Symptom:** `bin/ze-test bgp encode Q` failed with `bgp: create reactor: build peers: peer peer1 routes: update block: invalid prefix shared-join: netip.ParsePrefix("shared-join"): no '/'`. The parser fell through to the standard CIDR-prefix branch and tried to parse the MVPN route-type token as an IP prefix.
**Root cause:** `internal/component/bgp/config/bgp_routes.go` had a switch on family name to dispatch MVPN/flowspec/VPLS/MUP NLRI lines to their specialized parsers. The MVPN case used the legacy names `"ipv4/mcast-vpn"` / `"ipv6/mcast-vpn"`, but the family registry (`internal/component/bgp/plugins/nlri/mvpn/register.go:Families`) registers the canonical names `"ipv4/mvpn"` / `"ipv6/mvpn"`. Configs that wrote `ipv4/mvpn add shared-join ...` never reached `parseMVPNNLRILine`.
**Fix:** Update the case in `bgp_routes.go:171` to `"ipv4/mvpn"`, `"ipv6/mvpn"`. Also update the doc comment in `bgp_routes_mvpn.go:14-15` to use the canonical name in its example.

### 9. ExaBGP migration emitted stale family names (2 exabgp-compat tests: M, T)

**Symptom:** `make ze-verify` reached `ze-exabgp-test` and timed out on `M conf-mvpn` and `T conf-prefix-sid`. The migrated configs failed to load with `unknown address family "ipv4/nlri-mpls"` (T) and `update block: invalid family: ipv4/mpls` (T after first fix), or `update block: invalid prefix shared-join` (M, after Fix 7 was applied to bgp_routes.go).
**Root cause:** The migration tool (`internal/exabgp/migration/`) emits Ze config from ExaBGP source. Three places used pre-rename SAFI strings that no longer match Ze's family registry:
- `migrate_family.go:convertFamilySyntax` mapped `ipv4 nlri-mpls` -> `ipv4/nlri-mpls` (should be `mpls-label`) and had no mapping for `ipv4 mcast-vpn` / `ipv6 mcast-vpn` so the fallback produced `ipv4/mcast-vpn` (should be `mvpn`).
- `migrate_routes.go:convertFlexToUpdate` built the family name as `afi + "/" + safi` directly from the ExaBGP SAFI token (`mcast-vpn`), bypassing any rename.
- `migrate_routes.go:detectRouteFamily` returned `ipv4/mpls` / `ipv6/mpls` for label-without-RD routes (should be `mpls-label`).
**Fix:**
- `convertFamilySyntax` table updated: `nlri-mpls` -> `mpls-label`, added `mcast-vpn` -> `mvpn` for both AFIs.
- Added `canonicalSAFI(safi)` helper in `migrate_family.go` mapping ExaBGP SAFI tokens to Ze canonical SAFIs (`mcast-vpn` -> `mvpn`, `nlri-mpls`/`labeled-unicast` -> `mpls-label`, `flowspec` -> `flow`). `convertFlexToUpdate` now calls it before building `fam`.
- `detectRouteFamily` returns `mpls-label` (not `mpls`) for both AFIs.
- `TestConvertFlexToUpdate/mvpn_ipv4` updated to assert the new canonical family name.
**Verification:** `bin/ze-test bgp encode --all` -> 48/48; `uv run ./test/exabgp-compat/bin/functional encoding` -> 37/37; `make ze-verify` -> 0; `go test -race ./internal/exabgp/...` -> green.

### 8. SDK NewFromTLSEnv missing initCallbackDefaults (1 plugin test: `70 exabgp-bridge-sdk`)

**Symptom:** External TLS-connecting plugins (e.g., the ExaBGP bridge in SDK mode) panicked at startup with `panic: assignment to entry in nil map` in `sdk.(*Plugin).OnEvent` at `pkg/plugin/sdk/sdk_callbacks.go:60`. The engine logged `"rpc startup: read registration failed" error="mux conn closed"` because the plugin process died before sending Stage 1 registration.
**Root cause:** Commit `58564e0a` (refactor: generic callback registry) introduced `Plugin.callbacks map[string]callbackHandler` and added `p.initCallbackDefaults()` to `NewWithConn` and `NewWithIO` to allocate it. `NewFromTLSEnv` was missed -- it returned `&Plugin{name, engineConn, engineMux}` literally with `callbacks == nil`. The first `OnEvent`/`OnConfigVerify`/`OnConfigApply`/etc. call panicked. Internal plugins worked because they go through `NewWithConn`. External plugins via `NewFromTLSEnv` didn't.
**Fix:** Call `p.initCallbackDefaults()` in `NewFromTLSEnv` before returning. The bug was masked by the test runner discarding plugin stderr at the default `ze.log.relay=warn` level (panic stack traces parse as `LevelInfo`, below the WARN floor).
**Friction notes (resolved 2026-04-11):**
- ~~The `slogutil.RelayLevel` default of WARN silently swallowed a process-killing panic.~~ Fixed by extracting `classifyStderrLine` in `internal/component/plugin/process/process.go`, which forces ERROR level for lines starting with `panic:` or `fatal error:` and for the goroutine stack that follows. Covered by `TestClassifyStderrLine*` in `stderr_relay_test.go`.
- ~~Three SDK constructors duplicate the `initCallbackDefaults` call.~~ Fixed by adding an unexported `newPlugin(name, rc)` helper in `pkg/plugin/sdk/sdk.go` that all three public constructors delegate to. A new constructor cannot forget to initialize the callbacks map.

### Verification (2026-04-07)

| Suite | Before bridge fix | After all 8 fixes |
|-------|------|------|
| `bin/ze-test bgp encode --all` | 0/96 | **48/48** |
| `bin/ze-test bgp plugin --all` | ~0/218 | **218/218** |
| `bin/ze-test bgp reload --all` | 0/19 | **19/19** |

Plugin/process/server/SDK unit tests: `go test -race ./internal/component/plugin/... ./pkg/plugin/sdk/...` -> all green.

## Full-suite flaky plugin+encode tests (test isolation) -- logged 2026-04-11

**Files / test IDs observed:**
- `test/encode/ebgp.ci` (Test 9, encode category)
- `test/plugin/attributes.ci` (Test Q, plugin category) -- timeout, 0/5 messages received
- `test/plugin/mup4.ci` (Test 121) and `test/plugin/mup6.ci` (Test 122) -- message mismatch
- `test/plugin/rpki-decorator-autoload.ci` (Test 196) -- timeout waiting for second message
- `test/plugin/check.ci` (Test V) -- message mismatch, received UPDATE with default-route payload when the test expected an empty body

**Symptom:** Each test passes in isolation (`bin/ze-test bgp plugin <id> -v` or `bin/ze-test bgp encode ebgp -v` -> exit 0). Each also passes when the plugin suite is re-run right after a failure (`bin/ze-test bgp plugin --all` -> 230/230). They fail only when the whole sequence `make ze-verify` runs all categories back-to-back, and the failing set changes run to run.

**Reproduction conditions:**
- FAILS: `make ze-verify` clean run (encode + plugin + reload + exabgp + editor + etc., all sequential). Flake hits 1-5 tests each time.
- PASSES: `bin/ze-test bgp plugin --all` alone (47s, 230/230).
- PASSES: `bin/ze-test bgp encode --all` alone.
- PASSES: individual test reruns.

**Not caused by dest-0 (plugin-startup-dispatcher-barrier).** dest-0 only touches
`OnAllPluginsReady` dispatch for `bgp-rpki`. None of these failing tests load
bgp-rpki. The specific tests that fail change between runs, and the plugin
suite re-run immediately after a failure hits 230/230 with no code changes.

**Root-cause hypothesis:** resource contention across test binaries when
`make ze-functional-test` runs its child categories sequentially without
sufficient isolation. Candidates:
- Ephemeral port reuse race between `encode` and `plugin` suites.
- Loopback alias `127.0.0.2` setup failing (seen in WARN logs:
  `ensureLoopbackAlias: ioctl SIOCAIFADDR 127.0.0.2 on lo0: operation not permitted`).
  On macOS dev machines this is expected -- tests fall back to ephemeral
  ports, but that may expose ordering assumptions.
- Prior test's BGP peer still holding the listening socket when the next test
  binds. The ebgp/check failures (UPDATE arrives with unexpected default-route
  body) look like a prior peer session bleeding into the new session.

**Proposed next step:** investigate `ze-test` runner for cross-category port
reservation or a barrier between categories.

**Interim workaround:** re-run `make ze-verify` or the specific category.
Single-category runs (`bin/ze-test bgp plugin --all`) are stable and safe as
a gate when the full suite flakes.

## Observer-exit antipattern in plugin .ci tests -- logged 2026-04-11

**Files (16):** `test/plugin/community-cumulative.ci`,
`community-priority.ci`, `community-strip.ci`, `community-tag.ci`,
`forward-overflow-two-tier.ci`, `forward-two-tier-under-load.ci`,
`rib-best-selection.ci`, `rib-graph.ci`, `rib-graph-best.ci`,
`rib-graph-filtered.ci`, `role-otc-egress-filter.ci`,
`role-otc-egress-stamp.ci`, `role-otc-export-unknown.ci`,
`role-otc-ingress-reject.ci`, `role-otc-unicast-scope.ci`,
`show-errors-received.ci`.

**Symptom:** Each test embeds a Python observer plugin that calls
`dispatch(api, 'daemon shutdown') ; api.wait_for_shutdown() ; sys.exit(1)`
on assertion failure. The runner only checks ze's exit code, and ze has
already exited 0 from the clean shutdown by the time `sys.exit(1)` runs.
The tests pass even when the assertion fails. None of these tests use
`expect=stderr:pattern=`, `reject=stderr:pattern=`, or `runtime_fail()`,
so there is no other failure path.

**How they were found:** the cmd-4 fix (`1fc98747`) flagged the same
pattern in three earlier `prefix-filter-*.ci` tests it had just shipped,
fixed those, and noted the antipattern was likely repeated elsewhere.
Sweep on 2026-04-11 found the 16 files above using `grep` for
`tmpfs=*.run` python blocks with `sys.exit(1)` and no
`runtime_fail`/`expect=stderr`/`reject=stderr`.

**Why it matters:** these are not flakes -- they are silent
false-positives. A test that says it covers AC-N may be running zero
assertions. The community-strip test, for example, claims to verify
egress community stripping but only ever asserts ze exited cleanly.

**Migration recipe (per file):**
1. Identify the production code path that should fire (engine log line,
   not observer log line). Run the test once with `ze.log.<subsystem>=info`
   and check the actual stderr to find the line.
2. Replace `sys.exit(1)` paths in the Python observer with
   `from ze_api import runtime_fail; runtime_fail('reason')`. This is the
   safe minimum -- the runner will see the `ZE-OBSERVER-FAIL` sentinel.
3. **Better:** drop the assertion logic from the observer entirely and add
   `expect=stderr:pattern=<production log line>` plus
   `reject=stderr:pattern=<wrong outcome>` at the bottom of the `.ci` file.
   This is what `prefix-filter-accept.ci` does (lines 132-134) and what
   the cmd-4 fix recommended. It tests the production code path, not the
   observer.
4. Verify by deliberately breaking the production code path and confirming
   the test FAILS. A test that still passes after the production logic is
   broken is still wrong.

**Hook:** `block-observer-sys-exit.sh` is wired in `settings.json`
(Write|Edit|MultiEdit on `.ci` files) and will warn (exit 1) on any new
file with this pattern. The 16 files above will trigger the hook on Edit
until migrated.

**Rule:** `.claude/rules/testing.md` "Observer-Exit Antipattern".
**Reference fix:** `1fc98747` (cmd-4 prefix filter), three test files
migrated from observer-exit to `expect=stderr:pattern=` assertions.
