# Known Test Failures

Pre-existing failures that need fixing. Each entry includes failure output and root-cause hypothesis.
Sessions should attempt to fix entries here before logging new ones.

Remove entries once fixed.

## make ze-command-list (compilation failure)

**File:** `scripts/inventory/commands.go:94-95` (formerly `scripts/command_inventory.go`)
**Failure:** `go run` fails with `rpc.Help undefined (type server.RPCRegistration has no field or method Help)` and `rpc.ReadOnly undefined`.
**Root cause:** The script was written when `internal/component/plugin/server/handler.go:RPCRegistration` had `Help string` and `ReadOnly bool` fields. Those fields were removed in a later commit (the current struct has only WireMethod, Handler, RequiresSelector, PluginCommand). The script's `commands.go:90-97` and `commands.go:111-117` still reference the removed fields.
**Fix:** Delete the `Help` and `ReadOnly` references from both struct literal sites and from the `CommandInfo` output struct, OR re-add the fields to RPCRegistration if the help/readonly metadata is still wanted. Either is small (~10 lines).
**Confirmed pre-existing:** broken before the scripts/command_inventory.go -> scripts/inventory/commands.go rename in commit 3 of the scripts rationalisation. The rename touched only the path, not the file content.

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

## TestInProcessSpeed (race) -- could not reproduce 2026-04-07

**File:** `internal/chaos/inprocess/runner_test.go:218`
**Failure (as logged previously):** DATA RACE between `bufio.(*Reader).Read` in `session_read.go:104` (goroutine running `readAndProcessMessage`) and `runtime.slicecopy` in another goroutine accessing the same `bufio.Reader` buffer.
**Reproduction attempts (2026-04-07):** Could not reproduce in 40+ runs across `go test -race -count=20 -run TestInProcessSpeed`, `-count=5 -parallel=8` of the full inprocess package, and `-count=2` of the combined `inprocess` + `bgp/reactor` packages. Static review of `session_read.go:60`/`:104` shows only one production caller of `s.bufReader` (the `Run` loop at `session.go:620`) and the bufReader is replaced under `s.mu.Lock()` in `connectionEstablished`. Only suspicious access path is the unsynchronized read of `s.bufReader` at `session_read.go:60` racing the write at `session_connection.go:255` -- but that would be a pointer race, not a slice copy race. Leaving the entry in case the failure resurfaces under different load; if it does not appear in the next 2 weeks of CI, delete the entry as stale.

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
**Friction notes for follow-up:**
- The `slogutil.RelayLevel` default of WARN silently swallowed a process-killing panic. Plain text lines parse as `LevelInfo`. Plugin panic stack traces should always reach the engine logs regardless of relay level. Suggest: detect a "panic:" prefix in `relayStderrFrom` and force ERROR level for the panic block.
- Three SDK constructors (`NewWithConn`, `NewWithIO`, `NewFromTLSEnv`) duplicate the `initCallbackDefaults` call. Suggest: a single private constructor that all three delegate to.

### Verification (2026-04-07)

| Suite | Before bridge fix | After all 8 fixes |
|-------|------|------|
| `bin/ze-test bgp encode --all` | 0/96 | **48/48** |
| `bin/ze-test bgp plugin --all` | ~0/218 | **218/218** |
| `bin/ze-test bgp reload --all` | 0/19 | **19/19** |

Plugin/process/server/SDK unit tests: `go test -race ./internal/component/plugin/... ./pkg/plugin/sdk/...` -> all green.
