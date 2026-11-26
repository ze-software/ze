# Spec: plugin-runner-inertness

| Field | Value |
|-------|-------|
| Status | design |
| Priority | P0 |
| Release reason | Plugin startup failures can mutate host state, leak kernel handles or goroutines, or clobber live process globals; re-audited across all 98 registered product runners at HEAD to fix every remaining pre-Stage-1 side effect and enforce inertness going forward. |
| Scope | plugin |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Design tier | strongest tier |
| Implementation tier | development tier |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

A plugin runner does side-effecting work BEFORE `(*sdk.Plugin).Run` sends Stage 1 (`ze-plugin-engine:declare-registration`), or registers unconditional `defer` teardown that clears process-wide state this run never acquired. When Stage 1 fails (connection drop, protocol version mismatch, rejected declaration, schema mismatch, or startup tier abort), the runner returns 1 having already mutated host state, opened kernel sockets, launched goroutines, subscribed on the EventBus, allocated process-wide singletons, or clobbered live process globals.

### Dated Historical Audit (2026-09-08)

Measured across 92 registered product runners on 2026-09-08, 19 reached outside their own locals before `p.Run` sent Stage 1:

| Finding | Runners | What ran before Stage 1 (2026-09-08) |
|---------|---------|--------------------------------------|
| Mutates the HOST | `flowspec-firewall`, `ike` | nftables syscalls through `ApplyAll` under `LegacySweepPending`; four node-wide XFRM policies through `installIKEBypass`, whose defer beside a live IKE engine removes that daemon's too |
| Mutates the calling PROCESS | `trafficusage` | `rlimit.RemoveMemlock()`, a `setrlimit`, before its own `!p.IsInternal()` gate |
| Opens a kernel handle | `fib/kernel`, `fib/p4`, `sysctl` | `newBackend()`, `netlink.NewHandle` inside it, closed after the abort's `return 1` |
| Allocates the process-wide default Loc-RIB | `connected`, `rib`, `static`, `sysrib` | `locrib.Default()` |
| Starts a goroutine | `iface` (four), `rpki`, `adj_rib_in` | workers started before the declaration; `iface`'s stops are straight-line code after `p.Run`, so its `return 1` skips all six |
| Leaves a process global pointing at a dead plugin | `adj_rib_in`, `rib`, `redistribute_egress`, `sysrib`, `isis`, `ospf`, `iface`, `kernel`, `connected` | exactly ONE is a defer (`rib`'s `activeManager.Store(nil)`, which does not unwind its route-injector registration); `ospf` also registers an opaque type whose second registration returns `ErrOpaqueTypeRegistered` and is only logged |
| Constructs a manager and subscribes | `as112` | `newAS112Producer`, `newServerManager`, `subscribeReplay` |

**Provenance:** the delivery recorded under `spec-plugin-query-mode` on 2026-09-09. Query mode does not depend on this: it never enters a runner body, which is what makes its inertness a property rather than a promise (`plan/learned/013-inertness-is-unreachability.md`). `plan/learned/007-declaration-on-the-registration.md` carries the same audit and is the answer to any proposal to run engines for introspection.

### First-release disposition, 2026-09-19

This is startup-failure safety work, independent of optional declaration-query capability. Source inspection on 2026-09-19 confirmed reachable abort-path defects and required complete HEAD remeasurement during design before selecting repairs.

### Complete HEAD Re-Audit (2026-11-26)

All **98** product `RunEngine` registrations across `internal/component/` and `internal/plugins/` were re-read at their entry points on 2026-11-26:

- **66 runners are already inert at HEAD** (including the 3 inline no-op stubs `bgp-capa`, `loop`, and `crashes`; `bgp-nlri-srpolicy`, now wired cleanly to `sdk.NewWithConn` + `p.Run`; and 4 runners resolved since 2026-09-08: `flowspec-firewall` moved `clearStaleRules` and `FlowSpecChanged.Subscribe` into `OnConfigure` guarded by `initialized`, `sysrib` moved client open and start into `ConfigTx`/`OnStarted` with no `locrib.Default()`, `fib-p4` uses a pure `&noopBackend{}`, and `kernel` already executes `registerKernelSources()` in `init()` under `sync.Once` while `obs.run` starts in `OnStarted`. `redistribute_egress` was removed and replaced by `internal/plugins/redistribute` plus `internal/component/bgp/redistribute`).
- **32 runners still violate pre-Stage-1 inertness at HEAD** (14 surviving from the 2026-09-08 audit — with `ike`, `traffic-usage`, and `sysctl` partially changed — plus 18 additional runners discovered across the full 98-runner scan):

| # | Plugin | Entry Point | Exact Pre-`p.Run` / Stage-1-Abort Side Effects at HEAD (2026-11-26) | Target Lifecycle Placement |
|---|--------|-------------|-------------------------------------------------------------------|----------------------------|
| 1 | `bfd` | `internal/component/bfd/bfd.go` `runBFDPlugin` | `issuequeue.Start(p)` starts report worker goroutine (L859); unconditional `defer` calls `api.SetService(nil)` and `api.SetRawCaptureProvider(nil)` (L868-869), clobbering any live service on Stage 1 abort. | Start `issuequeue` once in `OnConfigure` (guarded); clear `api.SetService`/`SetRawCaptureProvider` in `defer` only if `OnStarted` published them. |
| 2 | `bgp-adj-rib-in` | `internal/component/bgp/plugins/adj_rib_in/rib.go` `runAdjRIBInPlugin` | `ribevents.RegisterGateDispatch(...)` mutates global (L447); `r.startTimeoutScanner(stopCh)` starts timeout-scanner goroutine (L512); `rpc.RegisterBatchValidator(...)` mutates process global with no unwind (L520). | Register gate dispatch, batch validator, and start timeout scanner once in `OnConfigure` (or `OnStarted` for scanner), with guarded `defer` cleanup including `rpc.RegisterBatchValidator(nil)` only when registered by this run. |
| 3 | `bgp-epe` | `internal/component/bgp/plugins/epe/epe_plugin.go` `runEPEProducer` | `linkstateevents.Request.Subscribe(bus, ...)` (L36) and `mplsfib.ReplayRequest.Subscribe(bus, ...)` (L46) subscribe on shared EventBus before `p.Run`. | Subscribe once in `OnConfigure` (guarded against redelivery), with nil-checked `defer` unsubscribes. |
| 4 | `bgp-ls-export` | `internal/component/bgp/plugins/ls_export/export_plugin.go` `runTopologyExporter` | Subscribes on shared EventBus for every source in `linkstateevents.Sources()` (L49-57) before `p.Run`. | Subscribe once in `OnConfigure` (guarded against redelivery), with `defer` unsubscribing registered handles. |
| 5 | `bgp-rib` | `internal/component/bgp/plugins/rib/rib.go` `runRIBPlugin` | `newSelectedDispatch` starts `selectedReporter` goroutine (L812 -> `rib_selected_diagnostic.go:58`); `activeManager.Store(r)` + unconditional `defer Store(nil)` (L827-828); `ribevents.RegisterFlowSpecLookup` / `RegisterFlowSpecRedirectLookup` / `PublishRecovery` / `RegisterGateDispatch` mutate globals with unconditional clears (L829-841); `locrib.Default()` allocates/reads singleton (L845); `rpc.RegisterRouteInjector` mutates global with no unwind (L850); `SetEventBus(&sdkEventBus{...})` when external (L902); `r.sourceLeases.start(ctx)` (L931) and `r.startRecoveryRemovals(...)` (L935) start goroutines before `p.Run`. | Keep `newSelectedDispatch` goroutine-free until `OnConfigure`/`OnStarted` (start reporter and lease/recovery workers once after Stage 1); move `locrib.Default()`, global stores/registrations, and external `SetEventBus` into one-time `OnConfigure` initialization; guard every `defer` unwind (CAS/ownership flag + `rpc.RegisterRouteInjector(nil)`). |
| 6 | `bgp-rpki` | `internal/component/bgp/plugins/rpki/rpki.go` `runRPKIPlugin` | `workerWg.Go(rp.validationWorker)` starts validation goroutine (L348); unconditional `defer` calls `ribpolicy.SetOriginValidator(nil)` (L354). | Start `validationWorker` once in `OnConfigure` (or `OnStarted`); clear `ribpolicy.SetOriginValidator(nil)` in `defer` only if this run installed a validator. |
| 7 | `bgp-rpki-decorator` | `internal/component/bgp/plugins/rpki_decorator/decorator.go` `runDecorator` | `sdk.NewUnion(...)` (L52) starts `sweepLoop` goroutine (`pkg/plugin/sdk/union.go:84`) before `p.Run`. | Construct `sdk.NewUnion` once in `OnStarted` (guarded, nil-checked `defer u.Stop()`). |
| 8 | `bgp-rr` | `internal/component/bgp/plugins/rr/rr.go` `runRouteReflector` -> `serveRouteReflector` | `ribevents.RegisterGateDispatch(rr.dispatchCommand)` (L307) mutates process global before `p.Run`. | Register gate dispatch once in `OnConfigure` (or `OnStarted`) with guarded `defer` release. |
| 9 | `bgp-rs` | `internal/component/bgp/plugins/rs/server.go` `RunRouteServer` | `issuequeue.Start(p)` starts report worker goroutine (L369); `ribevents.RegisterGateDispatch(rs.dispatchCommand)` mutates process global (L430). | Start `issuequeue` and register gate dispatch once in `OnConfigure` (guarded), with guarded `defer` stop/release. |
| 10 | `interface` | `internal/component/iface/register.go` `runEngine` | `globalPPPoEClients.CompareAndSwap` (L401); starts 6 goroutines (`vppReconcile` L448, `registryReconcile` L471, `runUnnumberedWorker` L498, `runDelegationWorker` L517, `runRDNSSWorker` L521, `linkQueue.start` L527); mutates 3 globals (`setAddressOwnerReconcileTrigger` L491, `setRALearned` L511, `setDeviceOwnerReconcileTrigger` L526); subscribes via `SubscribeCollectNotify` (L555); stores `globalTracker.Store(tracker)` and starts `tracker.Start()` goroutine (L989-990); ALL stops and unsubscribes are straight-line code after `if err := p.Run(...); err != nil { return 1 }` (L1009-1058), leaking everything on abort. | Move worker startup, global trigger/tracker stores, `globalPPPoEClients` init, and `SubscribeCollectNotify` into one-time `OnConfigure` init; move all teardown into a guarded `defer` (using `CompareAndSwap` for `globalTracker`) so both error and clean exits tear down only what started. |
| 11 | `ike` | `internal/component/ike/engine/register.go` `runEngine` | `dataplane.Load` (L484) + unconditional `defer dataplane.CloseBackend()` (L526); mutates 4 globals (`activeTablePtr.Store` L503, `setActivePeers` L510, `dataplane.RegisterESPFormReaderStopped` L535 + unconditional `defer(nil)`, `reEstablishFn.Store` L560); `startAddressWorker` starts goroutine and subscribes to bus (L546); metrics ticker goroutine (L562-575). *(2026-09-08 `installIKEBypass` XFRM policies already removed.)* | Move `dataplane.Load`, global stores, `RegisterESPFormReaderStopped`, `startAddressWorker`, and metrics ticker into one-time `OnConfigure` init; guard `defer` teardown so Stage 1 abort closes no backend and clears no global. |
| 12 | `l2tp-auth-radius` | `internal/component/l2tp/plugins/authradius/register.go` `runPlugin` | Subscribes 4 EventBus handlers (`acctInstance.subscribeEventBus` L158, `subscribeCoAHealth` L159, `txevents.EventCommitted` L163, `txevents.EventAccepted` L176); unconditional `defer` calls `retireAuthClient()`, `closeCoAListener()`, `report.ClearSource(Name)`, `retireAccounting()` (L198-215) on Stage 1 abort. | Subscribe once in `OnConfigure` (guarded); gate global cleanup in `defer` behind whether this run initialized/configured. |
| 13 | `sysctl` | `internal/component/sysctl/register.go` `runSysctlPlugin` | Subscribes 7 EventBus handlers (L166-280) before `p.Run`, with unsubscribes in straight-line code AFTER `if err != nil { return 1 }` (L447-455), leaking all 7 subscriptions pointing at the dead store on abort. | Subscribe once in `OnConfigure` (guarded); move unsubscribes and `s.restoreAll()` into a guarded `defer` so abort leaks zero subscriptions and restores only if configured. |
| 14 | `traffic` | `internal/component/traffic/register.go` `runEngine` | Unconditional `defer CloseBackend()` (L189-195) closes and restores the process-wide traffic backend on Stage 1 abort even when this run never configured or loaded a backend. | Track whether this run entered `OnConfigure`/loaded a backend and call `CloseBackend()` in `defer` only when true. |
| 15 | `anomaly-observe` | `internal/plugins/anomaly/observe/register.go` `runEngine` | Unconditional `defer teardown()` calls `activeStore.Store(nil)` (L146-148) on Stage 1 abort even though this run never installed a store. | Track the store installed by this run and clear `activeStore` via `CompareAndSwap(mine, nil)` only when non-nil. |
| 16 | `as112` | `internal/plugins/as112/register.go` `runAS112Plugin` | `prod.subscribeReplay()` (L274) subscribes on shared EventBus before `p.Run`. | Subscribe once in `OnConfigure` (or first `inst.install` / `OnStarted`), with nil-checked `defer` unsubscribe and guarded `prod.withdraw()`. |
| 17 | `connected` | `internal/plugins/connected/connected.go` `runConnectedPlugin` | `locrib.Default()` allocates/reads process-wide Loc-RIB (L363) before `p.Run` (plus redundant `registerConnectedSources()` L356 already called in `init()`). | Resolve `locrib.Default()` / `routeinstall.New` and wire `obs.setLocRIB` once in `OnConfigure` (or `OnStarted`), removing the redundant `registerConnectedSources()` call; guard `defer obs.withdrawLocalPaths()`. |
| 18 | `container` | `internal/plugins/container/container.go` `runContainerEngine` | `registeredSiblings.handEngine(engine)` mutates process global (L112); `sup.sweep()` mutates HOST by sweeping/removing leftover containers before `p.Run` (L117). | Move `registeredSiblings.handEngine(engine)` and `sup.sweep()` into one-time `OnConfigure` init (before `installer.install`); guard `defer sup.killRunning()` and `sup.close()` so Stage 1 abort touches nothing. |
| 19 | `cos` | `internal/plugins/cos/register.go` `runPlugin` | Unconditional `defer` stops package-global `dynamicHandler` (L162-165) on Stage 1 abort even though this run never configured CoS. | Track whether this run configured CoS / installed `dynamicHandler` and stop it in `defer` only when owned by this run. |
| 20 | `ddos-flowspec` | `internal/plugins/ddos/flowspec/register.go` `runEngine` | `startMaxDurationWorker(ctx, ...)` starts background worker goroutine (L864); unconditional `defer` calls `activeResponder.Store(nil)` (L664). | Start `startMaxDurationWorker` once in `OnStarted` (or `OnConfigure`); clear `activeResponder` via `CompareAndSwap(resp, nil)` only when this run stored it. |
| 21 | `ddos-local` | `internal/plugins/ddos/local/register.go` `runEngine` | `startMaxDurationWorker(ctx, ...)` starts background worker goroutine (L511); unconditional `defer` calls `activeResponder.Store(nil)` (L374). | Start `startMaxDurationWorker` once in `OnStarted` (or `OnConfigure`); clear `activeResponder` via `CompareAndSwap(resp, nil)` only when this run stored it. |
| 22 | `ddos-observe` | `internal/plugins/ddos/observe/register.go` `runEngine` | Unconditional `defer activeStore.Store(nil)` (L125) clears any live store on Stage 1 abort even though this run never installed one. | Track the store installed by this run and clear `activeStore` via `CompareAndSwap(mine, nil)` only when non-nil. |
| 23 | `dnsresolver` | `internal/plugins/dnsresolver/register.go` `runResolverPlugin` | `res.start()` starts background worker goroutine (L134); `activeResolver.Store(res)` mutates global + unconditional `defer activeResolver.Store(nil)` (L136-137). | Call `res.start()` and `activeResolver.Store(res)` once in `OnConfigure` (guarded); unwind via `CompareAndSwap(res, nil)` and `res.stop()` in `defer` only if started. |
| 24 | `fib-kernel` | `internal/plugins/fib/kernel/register.go` `runFIBKernelPlugin` | `newBackend()` (L261 -> `backend_linux.go:66-76`) opens a Linux netlink socket (`netlink.NewHandle`) AND installs IPv4/IPv6 MPLS guard routes (`ensureMPLSGuard`) in the kernel before `p.Run`. | Open `newBackend()` and construct `f := newFIBKernel(backend)` in `OnStarted` (nil-safe `show fib kernel` before start); close `backend` in `defer` only when non-nil. |
| 25 | `flow-export` | `internal/plugins/flowexport/register.go` `runEngine` | `iface.SubscribeCollectNotify(notifyFromRateTracker)` subscribes before `p.Run` (L182); unconditional `defer` calls `activeExporter.Swap(nil)` and stops any live exporter (L185-187). | Subscribe once in `OnConfigure` (guarded); unsubscribe and CAS-clear only this run's exporter in `defer`. |
| 26 | `isis` | `internal/plugins/isis/register.go` `runISISEngine` + `runISISPlugin` | `setRouteInstallClient(p)` mutates global with no unwind (L300); `newEngine` -> `initSPF()` calls `locrib.Default()` (`spf_wiring.go:43`); `issuequeue.Start(p)` starts goroutine and `issueQueue.Store(issues)` mutates global with no unwind (L319-320). | Defer `setRouteInstallClient`, `issuequeue.Start`/`issueQueue.Store`, and `newEngine` (or SPF/locrib binding) until one-time `OnConfigure` init; unwind `routeInstallPtr` and `issueQueue` via `CompareAndSwap` in `defer` only when stored by this run. |
| 27 | `ldp` | `internal/plugins/ldp/register.go` `runLDPEngine` | `issuequeue.Start(p)` starts report worker goroutine (L273) and `issueQueue.Store(issues)` mutates process global with no unwind (L274). | Start `issuequeue` and store `issueQueue` once in `OnStarted` (or `OnConfigure`); stop and CAS-clear `issueQueue` in `defer` only when started. |
| 28 | `ospf` | `internal/plugins/ospf/register.go` `runOSPFEngine` | `issuequeue.Start(p)` starts goroutine + `issueQueue.Store(issues)` global with no unwind (L425-426); `setRouteInstallClient(p)` global with no unwind (L432); `wireV4Engine()` calls `initSPF()` -> `locrib.Default()` (`spf_wiring.go:40`), registers 5 opaque/SR consumers into process globals (L444-472), and registers metrics; `newInstanceManager` sets `ze_ospf_instances` gauge to 1 (L500); `newV6EngineSet` registers `ze_ospf_af_instances` metric (L509). | Move `issuequeue.Start`/`issueQueue.Store`, `setRouteInstallClient`, `wireV4Engine()`, `newInstanceManager`, and `newV6EngineSet` into one-time `OnConfigure` init; stop and CAS-clear `issueQueue` and `routeInstallPtr` in `defer` only when initialized by this run. |
| 29 | `redistribute` | `internal/plugins/redistribute/register.go` `runPlugin` | `consumerHeld.beginRun()` clears package-global held-routes map (L107); `issuequeue.Start(p)` starts report worker goroutine (L110); `setReplayCoordinator(coord)` mutates process global with no unwind (L126). | Move `consumerHeld.beginRun()`, `issuequeue.Start`, and `setReplayCoordinator` into one-time `OnConfigure` init (keeping `constructFactoryConsumers(p)` pure callback registration before `p.Run`); stop `issues` and CAS-clear `replayCoordPtr` in `defer` only when initialized. |
| 30 | `static` | `internal/plugins/static/register.go` `runStaticPlugin` | `issuequeue.Start(p)` starts goroutine (L453); `locrib.Default()` allocates/reads singleton (L460); `activeRouteManager.Store(rm)` + unconditional `defer Store(nil)` (L469-470); `redistevents.ReplayRequestEvent.Subscribe` subscribes on EventBus (L476). | Move `issuequeue.Start`, `locrib.Default()` / `routeinstall.New`, `activeRouteManager.Store(rm)`, and replay subscription into one-time `OnConfigure` init; move `rm.shutdown()`/`rm.closeBackend()` and guarded CAS-clear + stop/unsubscribe into `defer`. |
| 31 | `traffic-usage` | `internal/plugins/trafficusage/register.go` `runEngine` | `activeMonitor.Store(mon)` mutates global (L85); `iface.SubscribeCollectNotify(mon.onSnapshot)` subscribes (L90); `mon.Start()` starts poller goroutine (L94); Stage 1 error branch unconditionally calls `activeMonitor.Store(nil)` (L175). *(2026-09-08 `rlimit.RemoveMemlock()` already moved to `Available()` during reconcile.)* | Move `activeMonitor.Store(mon)`, `iface.SubscribeCollectNotify`, and `mon.Start()` into one-time `OnConfigure` init; unwind via guarded `defer` using `CompareAndSwap(mon, nil)`. |
| 32 | `vrrp` | `internal/plugins/vrrp/register.go` `runVRRPEngine` | `liveEngine.Store(eng)` mutates process global (L192, with `CompareAndSwap` defer L193) before `p.Run`. | Move `liveEngine.Store(eng)` into one-time `OnConfigure` init with guarded `CompareAndSwap(eng, nil)` in `defer`. |

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress. -->

### Architecture Docs
- [ ] `docs/architecture/api/process-protocol.md` - the five startup stages, `OnConfigure`, `OnStarted`, and what a Stage 1 abort does to a plugin
  → Decision: Keep the 5-stage protocol and wire format completely unchanged; move pre-Stage-1 side effects into existing `OnConfigure` / `OnStarted` callbacks.
  → Constraint: `OnConfigure` can be redelivered on failed live removal (`pkg/plugin/sdk/sdk_callbacks.go`), so one-time runtime initialization inside `OnConfigure` MUST be idempotent (guarded by a per-run initialized flag).
- [ ] `docs/architecture/plugin/plugin-system.md` - registration, discovery, startup phases, and the process boundary
  → Decision: No new `pkg/plugin/sdk` callback or `registry.Registration` field is added; existing `OnConfigure` and `OnStarted` callbacks already enforce Stage 1 completion before side effects run.
  → Constraint: Every plugin remains compatible with both internal (`DirectBridge`) and external (TLS subprocess) transport modes; existing `!p.IsInternal()` refusals remain unchanged.
- [ ] `ai/rules/plugins.md` - declarations as pure values and inertness requirements
  → Constraint: Every side effect MUST sit behind Stage 1 completion (never executed before `(*sdk.Plugin).Run` completes Stage 1), and a Stage 1 abort MUST leave host, process, goroutines, subscriptions, and globals untouched.
- [ ] `ai/rules/goroutine-lifecycle.md` - long-lived workers and lifecycle joins
  → Constraint: No worker goroutine may be launched before `p.Run` completes Stage 1; every worker started after Stage 1 MUST have a `defer` stop/join path that runs on both error and clean returns.
- [ ] `ai/patterns/plugin.md` - structural template for runners and callbacks
  → Constraint: Runners follow the standard `sdk.NewWithConn` + callback registration + `p.Run` structure, matching `flowspec-firewall` and `sysrib`.

### RFC Summaries (Scope: plugin)
N-A. No wire protocol and no RFC obligation.

**Key insights:**
- All 98 registered product runners were re-audited at HEAD on 2026-11-26; 66 are clean and 32 still perform pre-`p.Run` side effects or unconditional global-clearing defers.
- `flowspec-firewall` (`internal/plugins/flowspec-firewall/engine.go:127-155`) and `sysrib` (`internal/component/sysrib/register.go:171-190`) already demonstrate the exact Ze mechanism: pure callback registration before `p.Run`, idempotent one-time initialization in `OnConfigure` (or `OnStarted`), and ownership-guarded `defer` cleanup.
- Two severe straight-line-cleanup leaks (`iface` and `sysctl`) skip all goroutine stops and EventBus unsubscribes whenever `p.Run` returns an error.
- Eleven runners have unconditional `defer` global-clearing calls (`Store(nil)`, `Swap(nil)`, `SetService(nil)`, `SetOriginValidator(nil)`, `CloseBackend()`, `dynamicHandler.stop()`, etc.) that clobber a live instance's state when a second runner aborts at Stage 1.

## Current Behavior (MANDATORY)

**Source files read:** (2026-11-26, all 98 product `RunEngine` registrations scanned and all 32 violating runners + helpers inspected directly)
- [ ] `pkg/plugin/sdk/sdk.go` - `(*Plugin).Run` executes Stage 1 (`declare-registration`) first, returning `*StartupError` immediately on failure without invoking `OnConfigure` or `OnStarted`.
- [ ] `pkg/plugin/sdk/sdk_callbacks.go` - documents `OnConfigure` (Stage 2, redeliverable on failed live removal) and `OnStarted` (post-Stage-5, runs once before event loop and joined by `runtime-ready` before dependent tiers start).
- [ ] `pkg/plugin/sdk/union.go` - `sdk.NewUnion` spawns `go u.sweepLoop()` immediately at construction.
- [ ] `pkg/plugin/rpc/bridge.go` - `RegisterRouteInjector` and `RegisterBatchValidator` store typed function values into `atomic.Value`; passing `nil` clears them safely without changing `pkg/plugin/rpc`.
- [ ] `internal/component/plugin/issuequeue/queue.go` - `issuequeue.Start` spawns `go q.run()` immediately at construction.
- [ ] `internal/plugins/flowspec-firewall/engine.go` and `internal/component/sysrib/register.go` - already-fixed reference runners showing idempotent `OnConfigure`/`OnStarted` initialization.
- [ ] All 32 violating runner files listed in `## Files to Modify` (verified line-by-line at HEAD on 2026-11-26).

**Behavior to preserve:**
- The 5-stage startup protocol, wire framing, and stage order are untouched. No new SDK callback or `pkg/` API change is introduced.
- Every plugin that completes startup behaves identically to today on boot, runtime events, commands, reload (`verify`/`apply`/`rollback`/`commit`), doctor checks, and clean shutdown.
- Pure pre-Stage-1 mode/precondition refusals stay intact: inline stubs (`bgp-capa`, `loop`, `crashes`) return 0 without calling `p.Run`; internal-only runners (`as112`, `dhcpserver`, `flow-export`, `iface-dhcp`, `l2tp-auth-radius`, `l2tp-pool`, `traffic-usage`, `vrrp`) return 1 at `!p.IsInternal()` before `p.Run` with zero side effects; bus-dependent runners (`bgp-epe`, `bgp-ls-export`, `flowspec-firewall`) return 1 when `bus == nil` before `p.Run` with zero side effects.

**Behavior to change:**
- All 32 violating runners perform ZERO host mutations, ZERO kernel socket/handle opens, ZERO goroutine launches, ZERO EventBus or rate-tracker subscriptions, ZERO `locrib.Default()` calls, ZERO process-global mutations, and ZERO unconditional teardown of unowned globals before `(*sdk.Plugin).Run` succeeds past Stage 1.
- Every cleanup path (goroutine stop/join, unsubscribe, backend close, global CAS-clear) runs in an ownership-guarded `defer`, never straight-line code skipped on `p.Run` error.
- Whole-tree static AST and runtime Stage-1-abort guards in `internal/component/plugin/all/runner_inertness_test.go` fail CI if any existing or future runner introduces a pre-`p.Run` side effect.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| 5-stage SDK runner lifecycle (`OnConfigure`, `OnStarted`) | `pkg/plugin/sdk/sdk.go` (`(*Plugin).Run`), `pkg/plugin/sdk/sdk_callbacks.go` (`OnConfigure`, `OnStarted`) | Executes Stage 1 (`declare-registration`) first and returns `*StartupError` on failure before invoking Stage 2 `OnConfigure` or post-Stage-5 `OnStarted` (which is joined by `runtime-ready` before dependent tiers start). | Extends usage across all 32 violating runners: moves every pre-`p.Run` side effect into `OnConfigure` (guarded against redelivery) or `OnStarted`, adding no new SDK callback. |
| Idempotent `OnConfigure` initialization pattern | `internal/plugins/flowspec-firewall/engine.go` (`runEngine`, L103-155) | Uses a local `initialized` boolean inside `OnConfigure` so initial host sweep and EventBus subscription run once after Stage 1, survive failed-removal configure redelivery, and never run on Stage 1 abort. | Extends: adopted as the standard pattern for every runner whose side effects are needed during Stage 2 configuration. |
| `sdk.ConfigTx` boot/reload transaction helper | `pkg/plugin/sdk/config_tx.go` (`NewConfigTx`, `Register`), used by `internal/component/sysrib/register.go` (`runRouteViewPlugin`) | Registers verify/configure/apply/rollback callbacks before `p.Run` without running the installer until Stage 2 `OnConfigure`. | Extends: runners already using `ConfigTx` (`as112`, `container`, `dnsresolver`, `isis`, `ospf`, `bgp-rib`, `cos`) keep their `ConfigTx` and gate side effects behind Stage 2/5. |
| Composition-root conformance test suite | `internal/component/plugin/all/all_test.go` (`TestEveryDeclaredShapeIsOneStage1Accepts`, `TestNoRegistrationDependsOnAFIBWriter`) | Walks `registry.All()` over every compiled-in plugin in one test binary to enforce whole-tree registration and lifecycle invariants. | Extends: adds `internal/component/plugin/all/runner_inertness_test.go` with both an AST guard over all registered product `RunEngine` bodies and a Stage-1-abort runtime sweep over `registry.All()`. |
| Atomic pointer CAS & typed global clear helpers | `pkg/plugin/rpc/bridge.go` (`RegisterRouteInjector`, `RegisterBatchValidator`), `internal/plugins/vrrp/register.go` (`liveEngine.CompareAndSwap`) | Allows safe ownership-checked clearing of process-wide singletons so an aborting or stopping instance never clobbers another live instance. | Extends: applied to every global store/registration across the 32 runners so `defer` unwinds only what this run actually published. |
| Query-mode declaration isolation | `pkg/plugin/sdk/sdk_query.go` (`RunOrDeclare`), `internal/component/plugin/cli/main.go` (`answerQuery`) | Keeps external and built-in declaration queries unreachable from live runner code (`plan/learned/013-inertness-is-unreachability.md`). | Does not apply: query mode already never enters compiled-in `RunEngine` bodies; this spec fixes live startup Stage-1 abort safety. |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | A plugin can run in another process | the door every change reads | "A plugin queried in declare mode (`ze.plugin.mode=declare`, spelled `ZE_PLUGIN_MODE` in the environment) starts nothing (`ai/rules/plugins.md`)." | agrees | Every compiled-in runner now keeps everything before `p.Run` Stage 1 pure and side-effect-free, aligning live startup Stage 1 abort safety with declare-mode purity without changing `pkg/`. |
| `rfc/short/rfc5880.md` | Meta | `// Design:` page of `internal/component/bfd/bfd.go` | - | silent | RFC 5880 summary covers BFD wire format, state machine, and timers; it says nothing about Go runner initialization order before Stage 1. |
| `docs/research/bfd-implementation-guide.md` | Executive Summary | `// Design:` page of `internal/component/bfd/bfd.go` | - | silent | Research guide covers BFD protocol design and package layout; it says nothing about `issuequeue.Start` placement or `defer` guards before `p.Run`. |
| `docs/architecture/bfd.md` | Design | anchored to `internal/component/bfd/bfd.go` | - | silent | Architecture doc covers BFD engine loops, sessions, and transport; it says nothing about pre-Stage-1 runner initialization. |
| `docs/architecture/iface/logical-name-resolution.md` | Where the resolver lives, and what it returns | anchored to `bfd`, `iface`, `vrrp` | - | silent | Covers logical-to-OS interface name resolution; says nothing about Stage 1 runner inertness. |
| `docs/architecture/ospf/bfd-client.md` | Decisions | anchored to `bfd`, `ospf` | - | silent | Covers OSPF-to-BFD session subscription semantics; says nothing about pre-Stage-1 runner setup. |
| `ai/doors/routing-protocols.md` | Boundaries | the door covering `bfd`, `isis`, `ldp`, `ospf` | "In a subprocess `locrib.Default()` is nil, so the installer needs its `RouteSink`, which ships each batch over the `route-install` RPC" | agrees | `isis` and `ospf` still resolve `locrib.Default()` and fall back to the `route-install` RPC client when forked; they merely defer doing so until `OnConfigure` after Stage 1 succeeds. |
| `docs/architecture/plugin/rib-storage-design.md` | Source-DOWN replacement and sent ordering | `// Design:` page of `adj_rib_in`, `rib`, `rpki`; anchored to `rr`, `rs` | "The plugin's run function starts it before events flow and stops it after `Run` drains delivery" | agrees | Starting `bgp-rib`'s recovery-removal dispatcher in guarded `OnConfigure`/`OnStarted` still starts it before the event loop receives events and stops it after `Run` drains delivery. |
| `docs/architecture/api/commands.md` | Overview | anchored to `adj_rib_in`, `rpki`, `rs`, `ospf` | - | silent | Covers command verb taxonomy, shapes, and pipe semantics; says nothing about pre-Stage-1 runner startup ordering. |
| `docs/architecture/api/process-protocol.md` | 5-Stage Startup Protocol | anchored to `adj_rib_in`, `rib`, `rr`, `rs`, `ike`, `dnsresolver` | "Ze uses a synchronized 5-stage startup protocol with barriers between stages. Within each dependency tier, all plugins must complete each stage before any can proceed to the next." | agrees | Preserves all five stages, stage barriers, Stage 1 callback derivations (`OnCurrentSelection`, `OnOriginIntake`), and `runtime-ready` joins; moves only runner side effects behind Stage 1. |
| `ai/doors/bgp-plugins.md` | Where it sits | the door covering the 8 BGP plugin files | "A plugin runs internal (a goroutine) or external (a forked subprocess over TLS RPC), and the SDK hides which" | agrees | Every BGP plugin fix preserves both internal `DirectBridge` and external TLS subprocess behavior while eliminating pre-Stage-1 side effects. |
| `docs/architecture/wire/nlri-bgpls.md` | Wire Format Overview | `// Design:` page of `epe`, `ls_export` | - | silent | Covers BGP-LS NLRI wire layout and EPE/exporter encodings; says nothing about when EventBus subscriptions attach relative to Stage 1. |
| `docs/architecture/behavior/fsm-established.md` | TL;DR | anchored to `internal/component/bgp/plugins/rib/rib.go` | - | silent | Covers BGP Established-state FSM transitions; says nothing about `bgp-rib` runner startup before `p.Run`. |
| `docs/architecture/decisions/001-pull-model-metrics.md` | Current Ze State | anchored to `internal/component/bgp/plugins/rib/rib.go` | "`ConfigureMetrics func(reg any)` remains on `plugin.Registration`. The internal plugin runner reads the global metrics registry and calls the callback before `RunEngine`" | agrees | `ConfigureMetrics` wiring on `registry.Registration` is untouched; only runner-body metric registration inside `ospf`'s `RunEngine` moves behind Stage 1. |
| `docs/architecture/doctor-and-health-checks.md` | The runtime phase: checks only the daemon runs | anchored to `rib`, `rs`, `authradius` | - | silent | Covers doctor check phases and `OnDoctorCheck` contract; all `OnDoctorCheck` callbacks remain registered before `p.Run` and nil-safe before `OnStarted`. |
| `docs/architecture/rib-transition.md` | Executive Summary | anchored to `internal/component/bgp/plugins/rib/rib.go` | - | silent | Covers moving RIB storage out of the engine into plugins; says nothing about Stage 1 abort inertness. |
| `docs/architecture/rib/forward-handle.md` | The decisions | anchored to `internal/component/bgp/plugins/rib/rib.go` | - | silent | Covers zero-copy `ForwardHandle` tracking in Loc-RIB; says nothing about runner initialization before `p.Run`. |
| `docs/architecture/route-selection.md` | Overview | anchored to `internal/component/bgp/plugins/rib/rib.go` | - | silent | Covers RFC 4271 best-path selection rules; says nothing about Stage 1 runner lifecycle. |
| `docs/architecture/edge-cases/as4.md` | Overview | anchored to `internal/component/bgp/plugins/rpki/rpki.go` | - | silent | Covers 4-byte ASN wire reconstruction; says nothing about `bgp-rpki` worker launch ordering. |
| `docs/architecture/plugin/plugin-system.md` | Startup phases and callbacks | anchored to `rpki`, `rs`, `iface` | "A plugin's `OnStarted` fires after its ready acknowledgment within Stage 5, before plugins of a later phase are loaded. A declared initializer must return successfully and answer runtime-ready before its tier completes." | agrees | Relies directly on `OnConfigure` (Stage 2) and `OnStarted` (Stage 5, joined by `runtime-ready`) to initialize resources safely after Stage 1 completes. |
| `docs/architecture/core-design.md` | 6. Plugin API Communication | `// Design:` page of `rr`, `rs`, `traffic`, `connected`; anchored to `iface`, `sysctl`, `fib/kernel`, `ospf`, `redistribute` | "Ze engine communicates with plugins via newline-framed YANG RPC over a single bidirectional connection. Internal plugins start on `net.Pipe()` and switch supported hot paths to DirectBridge after Stage 5." | agrees | Transport, framing, and post-Stage-5 DirectBridge switch are preserved unchanged while ensuring pre-Stage-1 runner bodies stay inert. |
| `docs/architecture/api/architecture.md` | TL;DR (Read This First) | anchored to `internal/component/bgp/plugins/rr/rr.go` | - | silent | Covers engine-vs-plugin split and bus participant wiring; says nothing about pre-Stage-1 runner initialization. |
| `docs/architecture/api/text-parser.md` | Current Parser | anchored to `rr`, `rs` | - | silent | Covers text UPDATE syntax parsing; says nothing about runner lifecycle before `p.Run`. |
| `docs/architecture/plugin/plugin-relationships.md` | 4. Dependency Graph | anchored to `rr`, `iface` | - | silent | Covers plugin dependency tiers and config ownership; tier ordering is unchanged by moving runner side effects into `OnConfigure`/`OnStarted`. |
| `docs/architecture/wire/nlri.md` | Overview | anchored to `internal/component/bgp/plugins/rr/rr.go` | - | silent | Covers NLRI wire encoding/decoding; says nothing about plugin runner startup. |
| `docs/architecture/buffer-architecture.md` | Executive Summary | anchored to `internal/component/bgp/plugins/rs/server.go` | - | silent | Covers buffer-first pooled wire encoding; says nothing about plugin runner startup. |
| `docs/features/interfaces.md` | Architecture | `// Design:` page of `internal/component/iface/register.go` | "iface component (register.go) -- OnConfigure() loads backend, starts monitor" | agrees | Moving `iface`'s workers, triggers, collect subscription, and rate tracker into guarded `OnConfigure` matches the documented design that `OnConfigure` activates the interface runtime. |
| `docs/architecture/config/apply-ordering.md` | The design | anchored to `internal/component/iface/register.go` | - | silent | Covers config operation DAG ordering during reload; says nothing about pre-Stage-1 runner startup. |
| `docs/architecture/config/transaction-protocol.md` | 1. Design Principles | anchored to `iface`, `authradius` | - | silent | Covers verify/apply/rollback transaction phases during reload; says nothing about pre-Stage-1 runner initialization. |
| `docs/architecture/iface/management.md` | Two layers, not one flat list | anchored to `iface`, `ldp` | - | silent | Covers logical vs OS interface representation and sysctl ownership; says nothing about Stage 1 abort safety. |
| `docs/architecture/l2tp/cpe-1-pppoe-client.md` | Decisions | anchored to `internal/component/iface/register.go` | - | silent | Covers PPPoE client lifecycle and cleanup diagnostics; deferring `globalPPPoEClients` init into `OnConfigure` preserves restart persistence while keeping Stage 1 abort inert. |
| `docs/architecture/traffic/cos-plugin.md` | Decisions | anchored to `iface`, `cos` | - | silent | Covers named VLAN QoS profiles; says nothing about guarding `dynamicHandler.stop()` on Stage 1 abort. |
| `ai/doors/forwarding.md` | Boundaries | the door covering `iface`, `sysctl`, `traffic`, `fib/kernel` | "A plugin raises or clears issues through SDK `RaiseIssue` or `ClearIssue`, whose `report-issue` RPC reaches the daemon's store" | agrees | Preserves SDK issue reporting and backend/FIB ownership boundaries while deferring kernel netlink handles, subscriptions, and backend teardown behind Stage 1. |
| `docs/architecture/ike/ipsec-7-ikev2-engine.md` | Decisions | `// Design:` page of `internal/component/ike/engine/register.go` | "`OnConfigure` applies the configuration and starts the IKE and NAT-T receive loops, then stages the peer configuration for `OnAllPluginsReady`." | agrees | Deferring `dataplane.Load`, global pointers, address worker, and metrics ticker into one-time `OnConfigure` init preserves socket-before-peers startup while making Stage 1 abort inert. |
| `docs/architecture/ike/ipsec-10-cli-diag.md` | Decisions | anchored to `internal/component/ike/engine/register.go` | - | silent | Covers IPsec show/clear CLI and diagnostics; says nothing about pre-Stage-1 `runEngine` setup. |
| `docs/architecture/ike/ipsec-13-rekey-wire.md` | Decisions | anchored to `internal/component/ike/engine/register.go` | - | silent | Covers CREATE_CHILD_SA rekey wire exchange; says nothing about runner startup before `p.Run`. |
| `docs/architecture/ike/ipsec-14-responder.md` | Decisions | anchored to `internal/component/ike/engine/register.go` | - | silent | Covers IKE responder exchange handling; says nothing about runner startup before `p.Run`. |
| `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` | Decisions | anchored to `internal/component/ike/engine/register.go` | - | silent | Covers Child SA and XFRM dataplane abstraction; says nothing about loading the dataplane before Stage 1. |
| `ai/doors/access-security.md` | Where it sits | the door covering `ike/engine`, `l2tp/plugins/authradius` | - | silent | Covers AAA, PKI, IKE, and L2TP component boundaries and compile tags; says nothing about pre-Stage-1 runner initialization order. |
| `docs/research/l2tpv2-ze-integration.md` | 1. Architectural Placement | `// Design:` page of `internal/component/l2tp/plugins/authradius/register.go` | - | silent | Research design covers L2TPv2 subsystem placement and subscriber hooks; says nothing about EventBus subscription timing relative to Stage 1. |
| `docs/architecture/l2tp/bng-1-radius-attributes.md` | Decisions | anchored to `internal/component/l2tp/plugins/authradius/register.go` | - | silent | Covers RADIUS subscriber attributes and accounting; says nothing about pre-Stage-1 runner startup. |
| `docs/architecture/traffic/fw-7b-backend-hardening.md` | The context comes from the plugin's own signal handler | anchored to `internal/component/traffic/register.go` | "The traffic component's `runEngine` synthesises the context from its own plugin lifetime with `sdk.SignalContext()`, rather than waiting for a new SDK surface." | agrees | Keeps `sdk.SignalContext()` in `traffic/register.go` and only guards `defer CloseBackend()` so Stage 1 abort never closes an unowned backend. |
| `docs/architecture/anomaly/anomaly-3-observe.md` | Reconfigure order is fixed | `// Design:` page of `internal/plugins/anomaly/observe/register.go` | "`teardown` unsubscribes first and stops the sweep worker second, and only then does `apply` build the replacement store." | agrees | Preserves the exact unsubscribe-then-stop-then-replace order while guarding `activeStore` clearing with CAS so Stage 1 abort never clears another instance's store. |
| `ai/doors/system-plugins.md` | Boundaries | the door covering the 14 system plugin files | "Hand the declaration and the start function to `sdk.RunOrDeclare` (`pkg/plugin/sdk` `RunOrDeclare`) as separate arguments (`ai/rules/plugins.md`)." | agrees | Preserves external declare-mode isolation and extends pre-Stage-1 inertness to all compiled-in system plugin `RunEngine` runners. |
| `docs/architecture/dns/as112.md` | The on-box carve-out covers the anycast address | `// Design:` page of `internal/plugins/as112/register.go` | - | silent | Covers AS112 DNS responses, zone matching, and anycast address ownership; says nothing about subscribing to replay events before Stage 1. |
| `docs/architecture/container/runtime.md` | The plugin | `// Design:` page of `internal/plugins/container/container.go` and modified by this spec | "Before the engine is asked to start the plugin, the sweep removes what a previous run left. Once the engine connection closed, whatever still runs is killed at once" | departs | Moves `sup.sweep()` from before `p.Run` into one-time guarded `OnConfigure` (after Stage 1 succeeds, still before any container starts in `OnStarted`) and guards `killRunning` so a Stage 1 abort never sweeps or kills host containers while preserving the never-adopt rule on live starts. `docs/architecture/container/runtime.md` is updated in `## Files to Modify`. |
| `docs/architecture/container/image-store.md` | Where it is today | anchored to `internal/plugins/container/container.go` | - | silent | Covers container image storage layout and commands; says nothing about pre-Stage-1 runner sweep timing. |
| `docs/architecture/traffic/cos-dynamic.md` | Decisions | anchored to `internal/plugins/cos/register.go` | - | silent | Covers RADIUS-driven dynamic CoS behavior; says nothing about stopping `dynamicHandler` on Stage 1 abort. |
| `docs/architecture/ddos/cp-survival-5-detect-0-umbrella.md` | Decisions | `// Design:` page of `internal/plugins/ddos/flowspec/register.go` | - | silent | Covers DDoS detection stages and responder architecture; says nothing about starting `startMaxDurationWorker` before `p.Run`. |
| `docs/architecture/firewall/table-ownership-and-shutdown-flush.md` | Rule 1: the backend loads on demand | anchored to `internal/plugins/ddos/local/register.go` | "`ApplyAll` now loads the OS-default backend when a producer registers tables and no firewall section loaded one." | agrees | Keeping `ddos-local` inert before Stage 1 ensures no background worker or teardown touches firewall tables or `activeResponder` on Stage 1 abort. |
| `docs/architecture/dns/server-harness.md` | Why a core package and not a shared plugin | `// Design:` page of `internal/plugins/dnsresolver/register.go` | - | silent | Covers the shared core DNS server harness; says nothing about calling `res.start()` or `activeResolver.Store` before Stage 1. |
| `docs/architecture/dns/lan-resolver.md` | Runtime state | anchored to `internal/plugins/dnsresolver/register.go` | - | silent | Covers recursive LAN resolver cache, coalescing, and rate limits; says nothing about pre-Stage-1 worker launch. |
| `docs/architecture/flowexport/flow-export-0-umbrella.md` | In-process component with the SDK protocol | anchored to `internal/plugins/flowexport/register.go` | - | silent | Covers flow-export cross-cutting design and internal-mode requirement; `!p.IsInternal()` refusal stays intact while `SubscribeCollectNotify` moves behind Stage 1. |
| `docs/architecture/flowexport/flow-export-1-counter-export.md` | IPFIX counter records use the Total IEs, not the Delta IEs | anchored to `internal/plugins/flowexport/register.go` | - | silent | Covers IPFIX counter record encoding and polling intervals; says nothing about pre-Stage-1 rate-tracker subscription. |
| `docs/architecture/flowexport/flow-export-2-flow-records.md` | Workers are platform-independent | anchored to `internal/plugins/flowexport/register.go` | - | silent | Covers flow record encoders and worker teardown; says nothing about pre-Stage-1 runner initialization. |
| `docs/architecture/isis/isis-4-component-config.md` | Decision: one transaction owns runtime config and the export chain | `// Design:` page of `internal/plugins/isis/register.go` | "`sdk.ConfigTx` retains the parsed IS-IS config and its compiled export chain as one value. Startup installs that value without opening circuits; `OnStarted` opens them after the SDK completes startup." | agrees | Deferring `newEngine`, `setRouteInstallClient`, and `issuequeue.Start` into one-time `OnConfigure` init preserves `ConfigTx` ownership and keeps circuit opening in `OnStarted`. |
| `docs/architecture/isis/isis-11-redistribution.md` | Decision: source at init, consumer at start | anchored to `internal/plugins/isis/register.go` | "The config source registers from `init()`, because `ze config validate` links in every component but does **not** start the engine, so `import isis` must validate without a running engine. The consumer needs the engine handle, so it registers at start." | agrees | Source registration stays in `init()` and consumer registration stays in `OnStarted`; no pre-Stage-1 engine state is mutated. |
| `docs/architecture/isis/isis-13-cli-diag-interop.md` | Decision: show and clear are dispatcher proxies | anchored to `internal/plugins/isis/register.go` | - | silent | Covers IS-IS CLI proxy commands, diagnostics, and metrics; says nothing about pre-Stage-1 runner initialization. |
| `docs/architecture/overview.md` | 2. System Architecture | anchored to `isis`, `ospf` | - | silent | High-level system architecture overview; says nothing about Stage 1 abort runner inertness. |
| `docs/architecture/ldp/mpls-ldp.md` | Decision: reconcile discovery, do not restart the engine | `// Design:` page of `internal/plugins/ldp/register.go` | - | silent | Covers LDP label allocation, discovery reconciliation, and session timers; says nothing about launching `issuequeue.Start` before Stage 1. |
| `docs/architecture/ospf/ospf-4-component-config.md` | Decisions | `// Design:` page of `internal/plugins/ospf/register.go` | "`runOSPFEngine` starts the plugin report queue (`issuequeue.Start`) over its `sdk.Plugin` before it builds an engine, and stops it after every engine has shut down" | agrees | Moving both `issuequeue.Start` and `wireV4Engine()` into guarded `OnConfigure` keeps `issuequeue.Start` before engine creation and stops it after shutdown while keeping Stage 1 abort inert. |
| `docs/architecture/command-ownership.md` | Three Directories, Three Roles | anchored to `internal/plugins/ospf/register.go` | - | silent | Covers command placement and removal test; says nothing about pre-Stage-1 runner initialization. |
| `docs/architecture/config/syntax.md` | Overview | anchored to `internal/plugins/ospf/register.go` | - | silent | Covers operator configuration syntax; says nothing about plugin runner startup ordering. |
| `docs/architecture/ospf/ospf-ext-16-ipsec-auth.md` | Decisions | anchored to `internal/plugins/ospf/register.go` | - | silent | Covers OSPFv3 IPsec authentication; says nothing about pre-Stage-1 runner initialization. |
| `docs/architecture/static-routes.md` | Startup ordering and emit rules | `// Design:` page of `internal/plugins/static/register.go` | "Startup configuration validates and retains the initial route set without installing it. `OnStarted` binds the available BFD service; `OnAllPluginsReady` then applies that set, including named-table routes." | agrees | Deferring `issuequeue.Start`, `locrib.Default()`, `activeRouteManager.Store`, and replay subscription into guarded `OnConfigure` preserves the exact `OnConfigure` -> `OnStarted` -> `OnAllPluginsReady` startup sequence. |
| `docs/architecture/forked-route-install.md` | A sink per installer, not a remote Loc-RIB | anchored to `internal/plugins/static/register.go` | - | silent | Covers remote route-install RPC when `locrib.Default()` is nil in a subprocess; deferring the check to `OnConfigure` preserves both internal and forked paths. |
| `docs/architecture/traffic/traffic-usage.md` | Decisions | anchored to `internal/plugins/trafficusage/register.go` | - | silent | Covers eBPF TCX attachment and doctor reporting; deferring `activeMonitor`, `SubscribeCollectNotify`, and `mon.Start()` to `OnConfigure` preserves lazy eBPF probing and `!p.IsInternal()` refusal. |
| `docs/architecture/vrrp/vrrp-first-hop-redundancy.md` | Decisions | `// Design:` page of `internal/plugins/vrrp/register.go` | - | silent | Covers VRRP protocol state machine, RFC compliance, and internal-mode guard; says nothing about storing `liveEngine` before Stage 1. |
| `docs/architecture/vrrp/vrrp-macvlan-vmac-dataplane.md` | The problem | anchored to `internal/plugins/vrrp/register.go` | - | silent | Covers macvlan VMAC kernel dataplane behavior; says nothing about pre-Stage-1 `liveEngine` publication. |
| `ai/doors/plugin-infra.md` | Boundaries | the door covering `internal/component/plugin/all/runner_inertness_test.go` | "A new callback is one `On*` method in `sdk_callbacks.go`. The event loops do not change" | agrees | Adds zero new SDK methods or event-loop changes; enforces Stage 1 inertness purely with existing `OnConfigure`/`OnStarted` hooks and composition-root tests. |
| `ai/doors/le-tooling.md` | Boundaries | the door covering `test/plugin/runner-stage1-abort.ci` | "Run one package through `./le test unit pkg path <dir>/ [run <regex>] [vet]`, which takes the tags from the toolchain." | agrees | Scoped verification uses `./le test unit pkg` with feature tags and `.ci` functional tests under `test/plugin/`. |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-08-21 | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | This spec modifies no commands, command paths, or pipe operations. |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-20 | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | This spec changes no command pipe overrides or answer shapes. |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-23 | "A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does" | agrees | This spec changes no pipe operations or command execution behavior. |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-23 | "Every engine operation names the message it carries or `none` with a reason, and a plugin identity is authorized by a profile compiled from its filter" | agrees | This spec adds no engine operations and preserves all declared `Sends`/`Receives` and compiled bus authorization profiles unchanged. |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-25 | "Engine-to-plugin event delivery never drops events on queue overflow; endpoints expect every subscribed network event losslessly" | agrees | This spec leaves event delivery queuing and lossless delivery untouched; subscriptions still activate before runtime events flow. |
| `docs/architecture/plugin/plugin-system.md` | Design decisions | decision of 2026-11-23 | "With no daemon running, a command that parses a config starts each declared external program in query mode, never its run loop, and validates against its declaration" | agrees | Query-mode declaration isolation is preserved untouched; this spec complements it by making compiled-in live runners inert on Stage 1 abort too. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | This spec does not alter data-plane validation or host-side compatibility rules. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | This spec does not alter mismatch verdicts (and supports the 2026-11-26 supersession below by ensuring unstarted/aborted plugins leave zero side effects). |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | This spec does not touch firewall YANG annotations or VPP firewall verification. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | This spec does not change any data-plane selector names or registrations. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | This spec does not modify BGP egress next-hop rewriting or export filter chains. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | This spec does not modify policy modify rules or export chain references. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | This spec does not modify `policychain.Chain` or built-in BGP filter steps. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | This spec does not modify export chain evaluation or wireOverride provenance. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | This spec does not modify IPv4/IPv6 next-hop validation or policy chains. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | This spec does not modify forwarded vs locally originated next-hop handling. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | This spec does not modify filter insertions or the implied-filter seam. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | "Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code. Examples are a web certificate name that does not resolve, a looking-glass `server` entry with no certificate to present, and a looking-glass address list that does not parse. Where a service has several endpoints, only the endpoint that cannot be served stays down and the others serve. A reload whose config cannot be served stays refused as a whole, and the running services keep what they serve. In Thomas's words (voice-transcribed): "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; asked about a reload, "reload stay refused"; asked "Should only that endpoint stay down while the others serve", "yes". The reason is that a daemon that does not start brings up no networking, so the operator cannot reach the box to correct it." | agrees | Making every plugin runner inert before Stage 1 directly supports keeping the rest of `ze` clean and unaffected when one plugin fails to start, with zero leaked host mutations, goroutines, or clobbered globals. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | "The rule of 2026-11-23 above, that a configuration which holds a feature that programs only the Linux kernel while the interfaces run on VPP "is refused at daemon start", is superseded for the start: ze starts, that feature stays off, and `ze doctor` reports the mismatch with its fix. A commit, a reload and `ze config validate` still refuse such a configuration, by the same verdict as before. Thomas was asked whether the rule that ze always starts replaces the refusal at start, "(a) yes, ze starts, that feature stays off and ze doctor reports the mismatch, while commit and reload still refuse it; (b) no", and answered "2a" (voice-transcribed). The reason is the one of the row above: a daemon that does not start brings up no networking, so the operator cannot reach the box to correct the mismatch." | agrees | Keeping plugin runners strictly inert before Stage 1 guarantees a feature that fails or is held off at startup cannot mutate kernel state or poison the running daemon. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "When `\| implied` modifies an existing config line (such as inserting automatic members into a `filters [ ... ];` list), it keeps the line intact and places an inline `# implied: ...` comment on the same line after the statement naming what was added and why, rather than placing a comment only above the line or splitting the list across multiple lines. In Thomas's words (2026-11-23): "No, it should be on the same line and say what was added"." | agrees | This spec does not modify `\| implied` formatting. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "Everything on the bus is a participant named by its configuration path. A participant registers the messages it sends and receives, events and commands alike, and with nothing written that registration is what runs" | agrees | All participant `Sends`/`Receives` registrations are preserved unchanged. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A `send > to` list written on a participant replaces the destinations its messages have by registration; it never adds to them. Present and empty means nobody" | agrees | This spec does not change bus `send > to` filter compilation or delivery. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A `receive > from` list limits who may address a participant. Absent means everybody; present and empty means nobody" | agrees | This spec does not change bus `receive > from` filter compilation or delivery. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A send and a receive allow-list of message names (`send > allow`, `receive > allow`) exist on every participant: absent means everything declared, empty means nothing. The bus enforces them when it receives the message" | agrees | This spec does not change bus allow-list enforcement. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "What is written on the participant and what is written in the `bus` section compile into one filter per participant. A disagreement between the two places, or between an allow-list and a destination on one participant, is refused at commit naming both lines" | agrees | This spec does not change bus filter compilation or commit validation. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "The bus does not arbitrate between participants: no exclusive-role rule, no commit refusal and no silent winner. A plugin that must not duplicate another limits itself" | agrees | This spec does not change bus arbitration; CAS-guarded teardown also ensures a second aborting runner never clobbers another participant's globals. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "One display shows every registered default beside what the operator wrote, the defaults a written list replaced, and the line that produced each effective entry" | agrees | This spec does not modify bus display output. |
| `docs/features/interfaces.md` | Design decisions | decision of 2026-11-22 | "`ze init` claims every discovered device by default, including the ones another program owns. `ze init --confirm` asks about each discovered device and writes only the ones the operator confirms" | agrees | This spec does not modify `ze init` device discovery or confirmation behavior. |
| `docs/features/interfaces.md` | Design decisions | decision of 2026-11-22 | "`ze init --confirm` takes only an explicit answer: `yes`, `y`, `no` or `n`. Anything else, an empty line included, is not accepted, and the same device is asked again" | agrees | This spec does not modify `ze init --confirm` prompt handling. |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `registry.Registration.RunEngine` (`func(conn net.Conn) int`), invoked by `internal/component/plugin/process` (`startInternal` over a `net.Pipe` wrapped in `rpc.BridgedConn`, or `internal/test/cli/cmd_plugin_external.go` over TLS in external mode).

### Transformation Path
1. Runner entry: builds `p := sdk.NewWithConn(name, conn)` with `defer p.Close()`, checks pure mode/precondition guards (`!p.IsInternal()` or `bus == nil`), allocates only private in-memory structs/channels, registers `p.On*` / `sdk.NewConfigTx` callbacks, and registers ownership-guarded `defer` cleanups (all no-ops when uninitialized).
2. `(*sdk.Plugin).Run(ctx, &sdk.Registration{...})` sends Stage 1 (`ze-plugin-engine:declare-registration`) as its first engine RPC.
3. **Stage 1 abort branch:** engine rejects Stage 1 or closes `conn`. `p.Run` returns `*sdk.StartupError` (stage 1) without invoking `OnConfigure` or `OnStarted`. Every `defer` sees uninitialized state and performs zero mutations; runner returns 1 with host, kernel, goroutine count, EventBus subscriptions, Loc-RIB singleton, and process globals byte-for-byte unchanged.
4. **Normal startup branch:** Stage 1 succeeds; Stage 2 invokes `OnConfigure` (performing one-time guarded initialization for resources required during config application); Stage 3 declares capabilities; Stage 4 delivers registry; Stage 5 signals `ready`, activates `DirectBridge`, starts metrics, and invokes `OnStarted` (performing one-time runtime initialization for workers/backends not needed during Stage 2, joined by `runtime-ready` before dependent tiers start).
5. **Shutdown / post-Stage-1 failure branch:** `p.Run` drains callbacks and returns; ownership-guarded `defer` cleanups stop and join every worker started by this run, unsubscribe every subscription, close opened backends, and CAS-clear every global owned by this run.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Runner body -> Host state | `container` (`sup.sweep()`), `fib-kernel` (`netlink.NewHandle` + `ensureMPLSGuard`), `ike` (`dataplane.Load`/`CloseBackend`), `traffic` (`CloseBackend`) deferred behind Stage 1 | [ ] |
| Runner body -> Process state | Goroutines (`issuequeue.Start`, `sdk.NewUnion`, workers/scanners/tickers), EventBus and `iface.SubscribeCollectNotify` subscriptions, `locrib.Default()`, and package-global pointers/registries deferred behind Stage 1 and CAS-guarded on teardown | [ ] |
| Engine <-> Plugin (5-stage RPC) | Unchanged `#<id> <verb> [<json>]\n` over `net.Conn` / `DirectBridge` | [ ] |

### Changed-Wire Census

No wire format changes. This spec modifies zero files under `pkg/plugin/rpc/`, `pkg/plugin/sdk/`, or `internal/component/plugin/server/`. The Stage 1 `ze-plugin-engine:declare-registration` request, Stage 2 `configure`, Stage 3 `declare-capabilities`, Stage 4 `share-registry`, and Stage 5 `ready`/`runtime-ready` wire frames and JSON schemas are preserved verbatim.

**Search evidence:** Searched `pkg/plugin/rpc/types.go`, `pkg/plugin/sdk/sdk.go`, and `internal/component/plugin/server/startup.go` for `MethodDeclareRegistration`, `DeclareRegistrationInput`, and `registrationFromRPC`. Positive control confirmed writer `pkg/plugin/sdk/sdk.go:637` (`(*Plugin).Run` calling `p.callEngineWithResult(ctx, rpc.MethodDeclareRegistration, &reg)`) and reader `internal/component/plugin/server/startup.go:899,1400` (`(*engineStartupSink).onRegistration` and `registrationFromRPC`); neither writer nor reader is modified by this spec.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Plugin 5-stage startup RPC (`ze-plugin-engine:declare-registration`, Stage 1..5) | writer | `pkg/plugin/sdk/sdk.go` (`(*Plugin).Run`, L637) | Calls `p.callEngineWithResult(ctx, rpc.MethodDeclareRegistration, &reg)` before Stage 2 `configure` and Stage 5 `OnStarted` | Preserve unchanged; no field, method name, or stage order changes |
| Plugin 5-stage startup RPC (`ze-plugin-engine:declare-registration`, Stage 1..5) | reader | `internal/component/plugin/server/startup.go` (`(*engineStartupSink).onRegistration` L899, `registrationFromRPC` L1400) | Validates and converts `rpc.DeclareRegistrationInput` into `plugin.PluginRegistration` | Preserve unchanged; Stage 1 refusal path already returns an RPC error that terminates `(*Plugin).Run` before `OnConfigure`/`OnStarted` |

### Integration Points
- `(*sdk.Plugin).OnConfigure` and `(*sdk.Plugin).OnStarted` (`pkg/plugin/sdk/sdk_callbacks.go`) - existing lifecycle hooks that execute only after Stage 1 succeeds.
- `internal/component/plugin/all/runner_inertness_test.go` - composition-root static AST guard and runtime Stage-1-abort verification harness covering every compiled-in product runner.
- The 32 runner packages listed in `## Files to Modify`.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Every runner initializes resources inside existing `OnConfigure` / `OnStarted` callbacks driven by `(*sdk.Plugin).Run`, never bypassing Stage 1. |
| No unintended coupling (components stay isolated) | Yes | Each of the 32 runner fixes edits only its own package; no cross-plugin imports are added. The whole-tree guard lives in `internal/component/plugin/all/` (the existing composition root). |
| No duplicated functionality (extends existing, does not recreate) | Yes | Reuses `OnConfigure`, `OnStarted`, `sdk.ConfigTx`, `atomic.Pointer.CompareAndSwap`, and `rpc.Register*(nil)` instead of adding a new SDK hook or runner wrapper. |
| Zero-copy preserved where applicable (refs, not copies) | Yes | `DirectBridge` structured event and wire forwarding paths (`bgp-rib`, `bgp-adj-rib-in`, `bgp-rs`, `bgp-rr`) are untouched. |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | Zero new fields on `registry.Registration` or `sdk.Registration`; zero changes to `internal/component/plugin/server` or `pkg/plugin/sdk`. |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes | No new plugin, command, or `le` verb is introduced. `internal/component/plugin/all/runner_inertness_test.go` derives its runtime population from `registry.All()` and its AST scan roots from `pluginimports.PluginSearchRoots()`, with zero hardcoded plugin name lists. |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The HEAD runner inventory is complete and accurate | Exhaustive 2026-11-26 scan of all 98 product `RunEngine` registrations across `internal/component/` and `internal/plugins/` | A violating runner could escape repair | `TestAllRunnersInertBeforeStage1` (AST + runtime Stage-1 abort sweep over `registry.All()`) | validated |
| A-2 | Every one of the 32 runners can defer its pre-`p.Run` side effects into `OnConfigure` or `OnStarted` without altering normal-startup ordering or dependent-tier readiness | `pkg/plugin/sdk/sdk.go:651,749` runs `OnConfigure` at Stage 2 and `OnStarted` after Stage 5 before `runtime-ready` unblocks dependent tiers; `flowspec-firewall` and `sysrib` already follow this pattern | A runner whose consumer reads a global during Stage 2 before its `OnStarted` would need initialization in `OnConfigure` rather than `OnStarted` | Per-runner normal-start tests plus existing functional suites (`plugin`, `reload`, `bgp`, `ospf`, `isis`, `ldp`, `bfd`, `ipsec`, `l2tp`, `static`, `firewall`, `traffic`) | unvalidated |
| A-3 | `OnConfigure` can be redelivered after a failed live removal (`sdk_callbacks.go:137`), so one-time initialization in `OnConfigure` must be guarded | `pkg/plugin/sdk/sdk_callbacks.go:137-141` and `internal/plugins/flowspec-firewall/engine.go:128-132` | Redelivered `OnConfigure` would double-subscribe, start duplicate workers, or leak handles | Per-runner unit tests verifying Stage-2 redelivery idempotence for every runner initializing in `OnConfigure` | unvalidated |
| A-4 | `bgp-rib` `newSelectedDispatch` and `isis`/`ospf` `newEngine` can defer goroutine launch and `locrib.Default()` lookup without breaking existing unit tests that call those constructors directly | Direct inspection of `rib_selected_dispatch.go:135`, `isis/spf_wiring.go:42`, `ospf/spf_wiring.go:20` | Direct unit tests expecting `newEngine` to have initialized SPF would fail unless `runISISEngine`/`runOSPFEngine` constructs `engine` inside `OnConfigure` (leaving `newEngine` signature intact) | Constructing `eng` inside `OnConfigure` (for `isis`/`ospf`) and deferring `newSelectedDispatch` to `OnConfigure` (for `bgp-rib`) keeps internal constructors unchanged for existing unit tests | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | 32 runner packages in one giant diff is too large to review safely | Diff spans 32+ packages | Implement in 6 logical phases (Wiring guard first, then 5 grouped phases by subsystem/defect class), each verified with its scoped package tests before moving on |
| R-2 | Moving initialization into `OnConfigure`/`OnStarted` breaks a subtle startup dependency (e.g. a plugin reading a global before Stage 5) | Scoped package test or `.ci` startup test fails with nil handle/service/dispatcher | Place resources needed during Stage 2 in idempotent `OnConfigure` (which finishes before Stage 2 barrier), and resources needed only at runtime in `OnStarted` (which finishes before `runtime-ready` unblocks dependent tiers) |
| R-3 | Unconditional `defer` cleanup beside a pre-existing live instance clobbers the live instance's global on Stage 1 abort | Sentinel-global Stage-1 abort test sees the pre-populated sentinel cleared or replaced | Every global writer uses an ownership flag or `atomic.Pointer.CompareAndSwap(mine, nil)` so teardown clears ONLY what this run installed |
| R-4 | A runner's `OnExecuteCommand` or `OnDoctorCheck` is invoked before `OnStarted` completes | Nil-pointer panic in command/doctor handler during early startup | Every handler reading lazily initialized state checks for nil and returns a clean not-started status or empty finding |
| R-5 | AST static guard produces false positives on pure local struct constructors or misses a new side-effect helper | Legitimate clean runner fails AST guard, or a seeded regression passes | Match concrete side-effect patterns (`go` statements, `.Go(`, `issuequeue.Start`, `sdk.NewUnion`, `.Subscribe(`, `SubscribeCollectNotify`, `locrib.Default`, `netlink.NewHandle`, `dataplane.Load`, `.sweep(`, global `.Store(`/`.Swap(`/unconditional `defer` clears) AND pair the AST guard with a real Stage-1-abort runtime execution test over every registered runner |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A plugin whose initialization moved too late could miss initial config/events or fail startup; an unguarded `defer` could leave a worker running on shutdown or fail to unwind on clean exit. No wire protocol or persisted config format is affected. |
| How is it reverted? | Single commit (or per-phase commit) revert; zero config, schema, or persistent state migration needed. |
| Who else touches this path? | Any spec editing `runEngine`/`run*Plugin` in the 32 listed packages; the composition-root test in `internal/component/plugin/all/`. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Any registered product `RunEngine` driven to a Stage 1 rejection over `net.Pipe` / `rpc.BridgedConn` | → | Runner's pre-`p.Run` path exits 1 without entering `OnConfigure`/`OnStarted` and without mutating host, goroutines, Loc-RIB, subscriptions, or globals | `TestAllRunnersInertOnStage1Abort` (`internal/component/plugin/all/runner_inertness_test.go`) |
| Static AST walk over every product `RunEngine` registration discovered via `pluginimports.PluginSearchRoots()` | → | AST verifier checking statements before `p.Run` and unconditional global-clearing `defer`s | `TestNoRunnerHasPreStage1SideEffectsAST` (`internal/component/plugin/all/runner_inertness_test.go`) |
| Live `ze` daemon booting a plugin whose Stage 1 declaration is refused | → | Daemon aborts startup cleanly with zero leaked host rules/handles/workers, and a valid config still boots normally | `test/plugin/runner-stage1-abort.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Each of the 32 audited runners is driven to a failed Stage 1 (`ze-plugin-engine:declare-registration` returns an RPC error or `conn` closes before Stage 1 reply), in every mode that reaches `p.Run` (both bridged internal and external pipe where applicable), with pre-populated sentinel values in any process global the runner touches | The runner returns exit code 1, closes its SDK connection, leaves `runtime.NumGoroutine()` at baseline, leaves `locrib.Default()` unallocated when initially nil, leaves EventBus and rate-tracker subscriber counts unchanged, leaves every pre-populated sentinel global untouched, opens no kernel netlink socket, loads no IPsec/traffic dataplane, and sweeps no host containers | None: zero side effects may survive a Stage 1 abort. Pure local stack/heap allocations garbage-collected on return and `slog` debug/error log lines are permitted because they do not mutate host or process state. |
| AC-2 | Each of the 32 audited runners is started normally through all 5 stages (including Stage 2 redelivery where initialization lives in `OnConfigure`) and then cleanly shut down | The runner completes startup, initializes its workers/subscriptions/backends/globals once, answers its commands/events/config-apply identically to before this change, and tears down its own resources cleanly on shutdown. All 8 `!p.IsInternal()` refusals, 3 `bus == nil` refusals, and 3 inline no-op stubs retain their exact pre-Stage-1 exit behavior | Existing intentional mode refusals (`!p.IsInternal()` returning 1 before `p.Run`, `bus == nil` returning 1 before `p.Run`, and inline no-op stubs `bgp-capa`, `loop`, `crashes` returning 0) remain permitted because they never reach Stage 1 and perform zero side effects. |
| AC-3 | `TestNoRunnerHasPreStage1SideEffectsAST` and `TestAllRunnersInertOnStage1Abort` run over the full composition root (`internal/component/plugin/all`) | Both tests pass across all 98 registered product runners, and injecting a pre-`p.Run` goroutine launch, `locrib.Default()`, EventBus subscription, or unconditional global-clear `defer` into any runner makes the guard fail naming the offending plugin and symbol | Pure `sdk.NewWithConn`, `defer p.Close()`, `sdk.SignalContext()`, pure `!p.IsInternal()` / `bus == nil` checks, pure local struct/channel allocation, and `p.On*` / `sdk.NewConfigTx` callback registrations remain permitted before `p.Run` because they mutate no external or shared state. |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Starts `ze` with a misconfigured or incompatible plugin whose Stage 1 declaration is refused | `plugin/server` Stage 1 check rejects declaration -> `(*sdk.Plugin).Run` returns `*StartupError` -> runner exits 1 -> daemon exits cleanly without leaked host state, netlink handles, containers swept, or orphaned workers | `test/plugin/runner-stage1-abort.ci`, `TestAllRunnersInertOnStage1Abort` |
| 2 | Starts `ze` normally with BGP, IGP, forwarding, firewall, IKE, L2TP, DNS, DDoS, and container plugins enabled | `(*sdk.Plugin).Run` completes Stage 1 -> `OnConfigure` / `OnStarted` perform one-time initialization -> dependent tiers start after `runtime-ready` -> daemon serves traffic and commands identically | Per-package normal-start unit tests and `test/plugin/runner-stage1-abort.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestNoRunnerHasPreStage1SideEffectsAST` | `internal/component/plugin/all/runner_inertness_test.go` | AC-3 (static AST guard over all 98 product runners + seeded-violation red proof) | |
| `TestAllRunnersInertOnStage1Abort` | `internal/component/plugin/all/runner_inertness_test.go` | AC-1, AC-3 (runtime Stage-1 abort sweep over `registry.All()` checking goroutine baseline, `locrib.Default()`, and clean exit) | |
| `TestRunContainerEngineInertOnStage1Abort` | `internal/plugins/container/inertness_test.go` | AC-1, AC-2 (no `sup.sweep()` host call and no `registeredSiblings.handEngine` on Stage 1 abort) | |
| `TestRunFIBKernelInertOnStage1Abort` | `internal/plugins/fib/kernel/inertness_test.go` | AC-1, AC-2 (no netlink handle or MPLS guard install before Stage 1) | |
| `TestRunIKEEngineInertOnStage1Abort` | `internal/component/ike/engine/inertness_test.go` | AC-1, AC-2 (no `dataplane.Load`/`CloseBackend`, no global mutation, no address/metrics goroutines on Stage 1 abort) | |
| `TestRunTrafficEngineInertOnStage1Abort` | `internal/component/traffic/inertness_test.go` | AC-1, AC-2 (sentinel active backend preserved on Stage 1 abort) | |
| `TestRunIfaceEngineInertOnStage1Abort` | `internal/component/iface/inertness_test.go` | AC-1, AC-2 (zero leaked goroutines, zero `SubscribeCollectNotify` leaks, sentinel `globalTracker` & triggers preserved on Stage 1 abort) | |
| `TestRunSysctlPluginInertOnStage1Abort` | `internal/component/sysctl/inertness_test.go` | AC-1, AC-2 (zero leaked EventBus subscriptions on Stage 1 abort) | |
| `TestRunBFDPluginInertOnStage1Abort` | `internal/component/bfd/inertness_test.go` | AC-1, AC-2 (no `issuequeue` goroutine, sentinel `api.Service` preserved on Stage 1 abort) | |
| `TestRunAdjRIBInInertOnStage1Abort` | `internal/component/bgp/plugins/adj_rib_in/inertness_test.go` | AC-1, AC-2 (no scanner goroutine, no gate dispatch or `rpc.RegisterBatchValidator` mutation on Stage 1 abort) | |
| `TestRunBGPEPEInertOnStage1Abort` | `internal/component/bgp/plugins/epe/inertness_test.go` | AC-1, AC-2 (zero EventBus subscriptions on Stage 1 abort) | |
| `TestRunBGPLSExportInertOnStage1Abort` | `internal/component/bgp/plugins/ls_export/inertness_test.go` | AC-1, AC-2 (zero EventBus subscriptions on Stage 1 abort) | |
| `TestRunBGPRIBInertOnStage1Abort` | `internal/component/bgp/plugins/rib/inertness_test.go` | AC-1, AC-2 (no `selectedReporter`/lease/recovery goroutines, no `locrib.Default()`, sentinel `activeManager`/`RouteInjector`/lookups preserved on Stage 1 abort) | |
| `TestRunBGPRPKIInertOnStage1Abort` | `internal/component/bgp/plugins/rpki/inertness_test.go` | AC-1, AC-2 (no `validationWorker` goroutine, sentinel `OriginValidator` preserved on Stage 1 abort) | |
| `TestRunBGPRPKIDecoratorInertOnStage1Abort` | `internal/component/bgp/plugins/rpki_decorator/inertness_test.go` | AC-1, AC-2 (no `sdk.NewUnion` sweep goroutine on Stage 1 abort) | |
| `TestRunBGPRRInertOnStage1Abort` | `internal/component/bgp/plugins/rr/inertness_test.go` | AC-1, AC-2 (no `ribevents.RegisterGateDispatch` mutation on Stage 1 abort) | |
| `TestRunBGPRSInertOnStage1Abort` | `internal/component/bgp/plugins/rs/inertness_test.go` | AC-1, AC-2 (no `issuequeue` goroutine, no `RegisterGateDispatch` mutation on Stage 1 abort) | |
| `TestRunL2TPAuthRadiusInertOnStage1Abort` | `internal/component/l2tp/plugins/authradius/inertness_test.go` | AC-1, AC-2 (zero EventBus subscriptions, no auth/CoA/report teardown on Stage 1 abort; `!p.IsInternal()` refusal preserved) | |
| `TestRunISISInertOnStage1Abort` | `internal/plugins/isis/inertness_test.go` | AC-1, AC-2 (no `issuequeue` goroutine, no `locrib.Default()`, sentinel `routeInstallPtr` and `issueQueue` preserved on Stage 1 abort) | |
| `TestRunLDPInertOnStage1Abort` | `internal/plugins/ldp/inertness_test.go` | AC-1, AC-2 (no `issuequeue` goroutine, sentinel `issueQueue` preserved on Stage 1 abort) | |
| `TestRunOSPFInertOnStage1Abort` | `internal/plugins/ospf/inertness_test.go` | AC-1, AC-2 (no `issuequeue` goroutine, no `locrib.Default()`, no opaque/SR registration or gauge write, sentinel globals preserved on Stage 1 abort) | |
| `TestRunAS112InertOnStage1Abort` | `internal/plugins/as112/inertness_test.go` | AC-1, AC-2 (zero replay EventBus subscriptions on Stage 1 abort; `!p.IsInternal()` refusal preserved) | |
| `TestRunConnectedInertOnStage1Abort` | `internal/plugins/connected/inertness_test.go` | AC-1, AC-2 (no `locrib.Default()` allocation on Stage 1 abort) | |
| `TestRunStaticInertOnStage1Abort` | `internal/plugins/static/inertness_test.go` | AC-1, AC-2 (no `issuequeue`, no `locrib.Default()`, no replay subscription, sentinel `activeRouteManager` preserved on Stage 1 abort) | |
| `TestRunRedistributeInertOnStage1Abort` | `internal/plugins/redistribute/inertness_test.go` | AC-1, AC-2 (no `consumerHeld.beginRun()`, no `issuequeue`, sentinel `replayCoordPtr` preserved on Stage 1 abort) | |
| `TestRunDNSResolverInertOnStage1Abort` | `internal/plugins/dnsresolver/inertness_test.go` | AC-1, AC-2 (no `res.start()` goroutine, sentinel `activeResolver` preserved on Stage 1 abort) | |
| `TestRunFlowExportInertOnStage1Abort` | `internal/plugins/flowexport/inertness_test.go` | AC-1, AC-2 (no `SubscribeCollectNotify`, sentinel `activeExporter` preserved on Stage 1 abort; `!p.IsInternal()` refusal preserved) | |
| `TestRunTrafficUsageInertOnStage1Abort` | `internal/plugins/trafficusage/inertness_test.go` | AC-1, AC-2 (no `SubscribeCollectNotify`, no `mon.Start()` goroutine, sentinel `activeMonitor` preserved on Stage 1 abort; `!p.IsInternal()` refusal preserved) | |
| `TestRunVRRPInertOnStage1Abort` | `internal/plugins/vrrp/inertness_test.go` | AC-1, AC-2 (sentinel `liveEngine` preserved on Stage 1 abort; `!p.IsInternal()` refusal preserved) | |
| `TestRunCoSInertOnStage1Abort` | `internal/plugins/cos/inertness_test.go` | AC-1, AC-2 (sentinel `dynamicHandler` untouched on Stage 1 abort) | |
| `TestRunAnomalyObserveInertOnStage1Abort` | `internal/plugins/anomaly/observe/inertness_test.go` | AC-1, AC-2 (sentinel `activeStore` preserved on Stage 1 abort) | |
| `TestRunDDoSObserveInertOnStage1Abort` | `internal/plugins/ddos/observe/inertness_test.go` | AC-1, AC-2 (sentinel `activeStore` preserved on Stage 1 abort) | |
| `TestRunDDoSFlowspecInertOnStage1Abort` | `internal/plugins/ddos/flowspec/inertness_test.go` | AC-1, AC-2 (no `startMaxDurationWorker` goroutine, sentinel `activeResponder` preserved on Stage 1 abort) | |
| `TestRunDDoSLocalInertOnStage1Abort` | `internal/plugins/ddos/local/inertness_test.go` | AC-1, AC-2 (no `startMaxDurationWorker` goroutine, sentinel `activeResponder` preserved on Stage 1 abort) | |

### Boundary Tests (numeric inputs)
N-A. This spec introduces no new numeric config, CLI, or wire inputs.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `runner-stage1-abort` | `test/plugin/runner-stage1-abort.ci` | Starts `ze` with plugins whose Stage 1 handshake fails (external refusal / tier abort) and verifies clean exit without stranded runtime state, followed by normal startup of the affected plugins | |

### Interop Tests (Scope: plugin)
N-A. No wire-visible protocol behavior changes.

## Files to Modify
- `internal/component/bfd/bfd.go` - defer `issuequeue.Start` into `OnConfigure` and guard `api.SetService`/`SetRawCaptureProvider` teardown
- `internal/component/bgp/plugins/adj_rib_in/rib.go` - defer `RegisterGateDispatch`, `startTimeoutScanner`, and `RegisterBatchValidator` behind Stage 1 with guarded unwind
- `internal/component/bgp/plugins/epe/epe_plugin.go` - defer EventBus subscriptions into guarded `OnConfigure`
- `internal/component/bgp/plugins/ls_export/export_plugin.go` - defer linkstate EventBus subscriptions into guarded `OnConfigure`
- `internal/component/bgp/plugins/rib/rib.go` - defer `newSelectedDispatch`, `locrib.Default()`, global stores/registrations, `SetEventBus`, and lease/recovery workers behind Stage 1 with CAS/ownership-guarded unwind
- `internal/component/bgp/plugins/rpki/rpki.go` - defer `validationWorker` launch behind Stage 1 and guard `SetOriginValidator(nil)` teardown
- `internal/component/bgp/plugins/rpki_decorator/decorator.go` - defer `sdk.NewUnion` into `OnStarted` with nil-checked `defer u.Stop()`
- `internal/component/bgp/plugins/rr/rr.go` - defer `RegisterGateDispatch` in `serveRouteReflector` behind Stage 1 with guarded release
- `internal/component/bgp/plugins/rs/server.go` - defer `issuequeue.Start` and `RegisterGateDispatch` behind Stage 1 with guarded teardown
- `internal/component/iface/register.go` - defer 6 workers, `globalPPPoEClients` CAS, 3 global triggers, `SubscribeCollectNotify`, and `globalTracker`/`tracker.Start` into guarded `OnConfigure`, and move all straight-line teardown into guarded `defer`
- `internal/component/ike/engine/register.go` - defer `dataplane.Load`, 4 global stores/registrations, `startAddressWorker`, and metrics ticker into guarded `OnConfigure`, and guard `defer` teardown
- `internal/component/l2tp/plugins/authradius/register.go` - defer 4 EventBus subscriptions into guarded `OnConfigure` and guard global teardown in `defer`
- `internal/component/sysctl/register.go` - defer 7 EventBus subscriptions into guarded `OnConfigure` and move straight-line unsubscribes/restore into guarded `defer`
- `internal/component/traffic/register.go` - guard `defer CloseBackend()` so Stage 1 abort never closes an unowned backend
- `internal/plugins/anomaly/observe/register.go` - guard `activeStore` clear with CAS for this run's store
- `internal/plugins/as112/register.go` - defer `prod.subscribeReplay()` behind Stage 1 and guard `prod.withdraw()`
- `internal/plugins/connected/connected.go` - defer `locrib.Default()` / `routeinstall.New` behind Stage 1, remove redundant `registerConnectedSources()`, guard `withdrawLocalPaths()`
- `internal/plugins/container/container.go` - defer `registeredSiblings.handEngine` and `sup.sweep()` into guarded `OnConfigure` and guard `defer` teardown
- `internal/plugins/cos/register.go` - guard `defer` stop of package-global `dynamicHandler`
- `internal/plugins/ddos/flowspec/register.go` - defer `startMaxDurationWorker` behind Stage 1 and CAS-guard `activeResponder` clear
- `internal/plugins/ddos/local/register.go` - defer `startMaxDurationWorker` behind Stage 1 and CAS-guard `activeResponder` clear
- `internal/plugins/ddos/observe/register.go` - CAS-guard `activeStore` clear in `defer`
- `internal/plugins/dnsresolver/register.go` - defer `res.start()` and `activeResolver.Store` into guarded `OnConfigure` with CAS-guarded `defer`
- `internal/plugins/fib/kernel/register.go` - defer `newBackend()` (netlink socket + MPLS guard) into `OnStarted` with nil-checked `defer` close
- `internal/plugins/flowexport/register.go` - defer `SubscribeCollectNotify` into guarded `OnConfigure` and CAS-guard `activeExporter` teardown
- `internal/plugins/isis/register.go` - defer `setRouteInstallClient`, `newEngine` (`locrib.Default()`), and `issuequeue.Start`/`issueQueue.Store` into guarded `OnConfigure` with CAS-guarded `defer`
- `internal/plugins/ldp/register.go` - defer `issuequeue.Start`/`issueQueue.Store` behind Stage 1 with CAS-guarded `defer`
- `internal/plugins/ospf/register.go` - defer `issuequeue.Start`/`issueQueue.Store`, `setRouteInstallClient`, `wireV4Engine()`, `newInstanceManager`, and `newV6EngineSet` into guarded `OnConfigure` with CAS-guarded `defer`
- `internal/plugins/redistribute/register.go` - defer `consumerHeld.beginRun()`, `issuequeue.Start`, and `setReplayCoordinator` into guarded `OnConfigure` with CAS-guarded `defer`
- `internal/plugins/static/register.go` - defer `issuequeue.Start`, `locrib.Default()`, `activeRouteManager.Store`, and replay subscription into guarded `OnConfigure`, and move `rm.shutdown()`/`closeBackend()` into guarded `defer`
- `internal/plugins/trafficusage/register.go` - defer `activeMonitor.Store`, `SubscribeCollectNotify`, and `mon.Start()` into guarded `OnConfigure` with CAS-guarded `defer`
- `internal/plugins/vrrp/register.go` - defer `liveEngine.Store(eng)` into guarded `OnConfigure` with CAS-guarded `defer`
- `docs/architecture/container/runtime.md` - update `The plugin` and `The sweep of what a previous run left` to state that leftover container sweeping runs during Stage 2 `OnConfigure` (after Stage 1 succeeds and before any container starts in `OnStarted`), never before Stage 1, and that `killRunning` runs only after initialization

## Files to Create
- `internal/component/plugin/all/runner_inertness_test.go` - composition-root AST guard (`TestNoRunnerHasPreStage1SideEffectsAST`) and Stage-1-abort runtime sweep (`TestAllRunnersInertOnStage1Abort`)
- `internal/component/bfd/inertness_test.go` - Stage 1 abort and normal-start inertness proof for `bfd`
- `internal/component/bgp/plugins/adj_rib_in/inertness_test.go` - Stage 1 abort inertness proof for `bgp-adj-rib-in`
- `internal/component/bgp/plugins/epe/inertness_test.go` - Stage 1 abort inertness proof for `bgp-epe`
- `internal/component/bgp/plugins/ls_export/inertness_test.go` - Stage 1 abort inertness proof for `bgp-ls-export`
- `internal/component/bgp/plugins/rib/inertness_test.go` - Stage 1 abort inertness proof for `bgp-rib`
- `internal/component/bgp/plugins/rpki/inertness_test.go` - Stage 1 abort inertness proof for `bgp-rpki`
- `internal/component/bgp/plugins/rpki_decorator/inertness_test.go` - Stage 1 abort inertness proof for `bgp-rpki-decorator`
- `internal/component/bgp/plugins/rr/inertness_test.go` - Stage 1 abort inertness proof for `bgp-rr`
- `internal/component/bgp/plugins/rs/inertness_test.go` - Stage 1 abort inertness proof for `bgp-rs`
- `internal/component/iface/inertness_test.go` - Stage 1 abort inertness proof for `interface`
- `internal/component/ike/engine/inertness_test.go` - Stage 1 abort inertness proof for `ike`
- `internal/component/l2tp/plugins/authradius/inertness_test.go` - Stage 1 abort inertness proof for `l2tp-auth-radius`
- `internal/component/sysctl/inertness_test.go` - Stage 1 abort inertness proof for `sysctl`
- `internal/component/traffic/inertness_test.go` - Stage 1 abort inertness proof for `traffic`
- `internal/plugins/anomaly/observe/inertness_test.go` - Stage 1 abort inertness proof for `anomaly-observe`
- `internal/plugins/as112/inertness_test.go` - Stage 1 abort inertness proof for `as112`
- `internal/plugins/connected/inertness_test.go` - Stage 1 abort inertness proof for `connected`
- `internal/plugins/container/inertness_test.go` - Stage 1 abort inertness proof for `container`
- `internal/plugins/cos/inertness_test.go` - Stage 1 abort inertness proof for `cos`
- `internal/plugins/ddos/flowspec/inertness_test.go` - Stage 1 abort inertness proof for `ddos-flowspec`
- `internal/plugins/ddos/local/inertness_test.go` - Stage 1 abort inertness proof for `ddos-local`
- `internal/plugins/ddos/observe/inertness_test.go` - Stage 1 abort inertness proof for `ddos-observe`
- `internal/plugins/dnsresolver/inertness_test.go` - Stage 1 abort inertness proof for `dnsresolver`
- `internal/plugins/fib/kernel/inertness_test.go` - Stage 1 abort inertness proof for `fib-kernel`
- `internal/plugins/flowexport/inertness_test.go` - Stage 1 abort inertness proof for `flow-export`
- `internal/plugins/isis/inertness_test.go` - Stage 1 abort inertness proof for `isis`
- `internal/plugins/ldp/inertness_test.go` - Stage 1 abort inertness proof for `ldp`
- `internal/plugins/ospf/inertness_test.go` - Stage 1 abort inertness proof for `ospf`
- `internal/plugins/redistribute/inertness_test.go` - Stage 1 abort inertness proof for `redistribute`
- `internal/plugins/static/inertness_test.go` - Stage 1 abort inertness proof for `static`
- `internal/plugins/trafficusage/inertness_test.go` - Stage 1 abort inertness proof for `traffic-usage`
- `internal/plugins/vrrp/inertness_test.go` - Stage 1 abort inertness proof for `vrrp`
- `test/plugin/runner-stage1-abort.ci` - functional test verifying Stage 1 abort leaves no stranded host/daemon state

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | Lifecycle-ordering fix only; no config leaves or RPCs added or changed |
| YANG validation constraints | N-A | No YANG modules modified |
| YANG custom validators | N-A | No YANG validators added or changed |
| CLI commands/flags | N-A | No CLI commands or flags added or changed |
| CLI grammar (keyword before value) | N-A | No CLI commands added or changed |
| Editor autocomplete | N-A | No schema or command changes |
| Functional test for new RPC/API | Yes | `test/plugin/runner-stage1-abort.ci` exercises Stage 1 abort and normal startup end-to-end |
| Pipe completeness | N-A | No command output surfaces added or changed |
| Env var registration | N-A | No env vars added or changed |
| Doctor check for runtime dependencies | N-A | No new runtime dependencies; existing `OnDoctorCheck` handlers remain nil-safe before `OnStarted` |
| Doctor code per problem class | N-A | No new problem classes or refusal paths added |
| Prometheus counters/metrics | N-A | Existing metrics unchanged; OSPF gauge initialization moves behind Stage 1 so aborted starts do not publish misleading counts |
| BGP family surface (new SAFI / capability / attribute) | N-A | No BGP families, capabilities, or attributes changed |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Internal startup-failure safety fix; no user-facing feature change |
| 2 | Config syntax changed? | No | No config syntax changes |
| 3 | CLI command added/changed? | No | No CLI command changes |
| 4 | API/RPC added/changed? | No | No API or RPC changes |
| 5 | Plugin added/changed? | No | No plugin registrations, names, or operator surfaces changed |
| 6 | Has a user guide page? | No | Operator-visible behavior on healthy startup is unchanged |
| 7 | Wire format changed? | No | 5-stage RPC wire protocol unchanged (verified in Changed-Wire Census) |
| 8 | Plugin SDK/protocol changed? | No | `pkg/plugin/sdk` and `pkg/plugin/rpc` are not modified |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No protocol wire or RFC state-machine changes |
| 10 | Test infrastructure changed? | No | Uses standard Go `testing` package and existing `.ci` runner |
| 11 | Affects daemon comparison? | No | No feature comparison changes |
| 12 | Internal architecture changed? | Yes | `docs/architecture/container/runtime.md` (leftover sweep moves from pre-Stage-1 to guarded `OnConfigure`, still before any container starts in `OnStarted`) |
| 13 | Route metadata keys added/changed? | No | No route metadata changes |
| 14 | Prometheus counters added/changed? | No | No metric names or labels added or removed |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | Registration declarations are untouched |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived via `./le spec citation anchors`: `docs/architecture/container/runtime.md` is modified (`## Files to Modify`). The remaining 17 declared `// Design:` pages are unaffected because normal post-Stage-1 behavior, wire formats, and config/API contracts are unchanged: `docs/architecture/anomaly/anomaly-3-observe.md` (reconfigure order preserved), `docs/architecture/core-design.md` (RPC/DirectBridge contract unchanged), `docs/architecture/ddos/cp-survival-5-detect-0-umbrella.md` (detection/mitigation design unchanged), `docs/architecture/dns/as112.md` (DNS sink/anycast semantics unchanged), `docs/architecture/dns/server-harness.md` (shared DNS harness unchanged), `docs/architecture/ike/ipsec-7-ikev2-engine.md` (`OnConfigure`-before-`OnAllPluginsReady` startup preserved), `docs/architecture/isis/isis-4-component-config.md` (`ConfigTx`/`OnStarted` contract preserved), `docs/architecture/ldp/mpls-ldp.md` (LDP discovery/session design unchanged), `docs/architecture/ospf/ospf-4-component-config.md` (`issuequeue.Start`-before-engine relative order preserved), `docs/architecture/plugin/rib-storage-design.md` (recovery-removal before events flow preserved), `docs/architecture/static-routes.md` (`OnConfigure`/`OnStarted`/`OnAllPluginsReady` sequence preserved), `docs/architecture/vrrp/vrrp-first-hop-redundancy.md` (VRRP protocol and internal-mode guard preserved), `docs/architecture/wire/nlri-bgpls.md` (BGP-LS wire format unchanged), `docs/features/interfaces.md` (`OnConfigure` activation matches doc), `docs/research/bfd-implementation-guide.md` (BFD protocol guide unchanged), `docs/research/l2tpv2-ze-integration.md` (L2TP architecture unchanged), `rfc/short/rfc5880.md` (RFC 5880 wire/state summary unchanged). |
| 17 | Existing docs show config/CLI/API examples for this area? | No | No config, CLI, or API syntax changed |

## Design presentation

### What changes

Every compiled-in Ze plugin runner becomes strictly inert before Stage 1 (`ze-plugin-engine:declare-registration`) completes. Across all 98 registered product runners at HEAD, 32 still mutate host state (sweeping containers, opening netlink sockets and installing MPLS guards, loading IPsec dataplanes), launch background goroutines, subscribe on the EventBus or rate tracker, allocate `locrib.Default()`, or write/clobber process globals before `(*sdk.Plugin).Run` reaches Stage 1. Moving each side effect into the runner's existing `OnConfigure` (guarded against Stage 2 redelivery) or `OnStarted` callback and guarding every teardown `defer` by ownership fixes all 32 runners without adding any SDK API or wire change, and a whole-tree AST plus Stage-1-abort test suite in `internal/component/plugin/all/runner_inertness_test.go` prevents regressions.

### Before and after

1. **Startup Stage 1 abort on `container` or `fib-kernel`:**
   - Before: invoking `runContainerEngine` runs `sup.sweep()` on the host before Stage 1 is even sent; invoking `runFIBKernelPlugin` opens a netlink socket and runs `ensureMPLSGuard` for IPv4/IPv6 before Stage 1 is sent.
   - After: Stage 1 failure returns 1 before `OnConfigure`/`OnStarted` runs; zero containers are swept and zero netlink handles or kernel routes are touched.
2. **Startup Stage 1 abort on `interface` or `sysctl`:**
   - Before: `iface.runEngine` launches 7 goroutines (6 workers + rate tracker), writes 4 globals, and subscribes to collect notifications before `p.Run`, then returns 1 on Stage 1 error before its straight-line stops — leaking all 7 goroutines and globals. `sysctl` subscribes 7 EventBus handlers before `p.Run` and returns 1 before its straight-line unsubscribes — leaking all 7 handlers.
   - After: zero goroutines launch and zero subscriptions register before Stage 1 succeeds; all teardown lives in ownership-guarded `defer`s so post-Stage-1 errors also clean up completely.
3. **Startup Stage 1 abort beside a live instance (`bfd`, `bgp-rpki`, `traffic`, `ike`, `ddos-*`, `anomaly-observe`, `cos`, `flow-export`, `traffic-usage`, `dnsresolver`, `bgp-rib`, `static`):**
   - Before: unconditional `defer` cleanup (`api.SetService(nil)`, `SetOriginValidator(nil)`, `CloseBackend()`, `activeStore.Store(nil)`, `activeResponder.Store(nil)`, `dynamicHandler.stop()`, etc.) runs on Stage 1 abort and tears down live process-wide state this run never installed.
   - After: every global or backend teardown is gated on ownership (`CompareAndSwap(mine, nil)` or per-run ownership flag) and never runs when Stage 1 aborted.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Add a new `sdk.RunWithInit(ctx, reg, initFn)` callback to `pkg/plugin/sdk` | Violates `ai/META-DECISIONS.md` ("start from the mechanisms Ze already has") and expands the published `pkg/plugin/sdk` contract unnecessarily when `OnConfigure` (Stage 2) and `OnStarted` (Stage 5) already enforce Stage 1 completion and are already used by `flowspec-firewall` and `sysrib`. |
| Add a new `./le plugin inertness check` CLI command | Adds a new `le` verb surface (triggering `le help`/completion obligations) when a composition-root test (`internal/component/plugin/all/runner_inertness_test.go`, alongside `TestEveryDeclaredShapeIsOneStage1Accepts`) already runs in unit/CI verification over every compiled-in plugin with zero new CLI surface. |
| Only fix the 14 surviving runners from the 2026-09-08 audit and leave the 18 newly audited runners | Violates "done means done" and leaves host-mutating `container` (`sup.sweep()`), `issuequeue.Start` goroutine leaks (`bfd`, `bgp-rs`, `ldp`, `redistribute`), and unconditional global-clobbering defers (`traffic`, `cos`, `ddos-*`, `anomaly-observe`, `l2tp-auth-radius`) unfixed in P0 release code. |

### Documents departed from

- `docs/architecture/container/runtime.md`, section `The plugin` (and subsection `The sweep of what a previous run left`):
  - Current binding statement: "Before the engine is asked to start the plugin, the sweep removes what a previous run left. Once the engine connection closed, whatever still runs is killed at once"
  - What the page says after this work: On first Stage 2 `OnConfigure` (after Stage 1 succeeds and before any container starts in `OnStarted`), the sweep removes what a previous run left, preserving the owner's ruling that containers are never adopted while guaranteeing a Stage 1 abort neither sweeps nor kills host containers; once an initialized plugin's engine connection closes, whatever still runs is killed at once.

### Choices for Thomas

none

## Implementation Steps

1. **Phase 1: Wiring & Whole-Tree Guard (MANDATORY FIRST)** -- create `internal/component/plugin/all/runner_inertness_test.go` with `TestNoRunnerHasPreStage1SideEffectsAST` and `TestAllRunnersInertOnStage1Abort`, proving RED across the 32 violating runners before touching any runner.
   - Tests: `TestNoRunnerHasPreStage1SideEffectsAST`, `TestAllRunnersInertOnStage1Abort`
   - Files: `internal/component/plugin/all/runner_inertness_test.go`
   - Verify: tests run and fail specifically on the 32 violating runners.
2. **Phase 2: Host-Mutating & Forwarding/Kernel Runners (`container`, `fib-kernel`, `ike`, `traffic`, `interface`, `sysctl`)** -- move `sup.sweep()`, `newBackend()`, `dataplane.Load`, workers, subscriptions, and global writes behind Stage 1; move straight-line cleanup in `interface` and `sysctl` into ownership-guarded `defer`s; guard `traffic` `CloseBackend()`.
   - Tests: `inertness_test.go` in each of the 6 packages
   - Files: `internal/plugins/container/container.go`, `internal/plugins/fib/kernel/register.go`, `internal/component/ike/engine/register.go`, `internal/component/traffic/register.go`, `internal/component/iface/register.go`, `internal/component/sysctl/register.go`
   - Verify: per-package Stage-1 abort tests fail before edit, pass after edit, and normal-start tests remain green.
3. **Phase 3: BGP Subsystem Plugins (`bgp-adj-rib-in`, `bgp-epe`, `bgp-ls-export`, `bgp-rib`, `bgp-rpki`, `bgp-rpki-decorator`, `bgp-rr`, `bgp-rs`)** -- move `issuequeue.Start`, `sdk.NewUnion`, `newSelectedDispatch`, `locrib.Default()`, workers, EventBus subscriptions, and global registrations (`RegisterGateDispatch`, `RegisterBatchValidator`, `RegisterRouteInjector`, `activeManager`, FlowSpec lookups) behind Stage 1 with ownership/CAS-guarded teardown.
   - Tests: `inertness_test.go` in each of the 8 packages
   - Files: the 8 BGP plugin files listed in `## Files to Modify`
   - Verify: per-package Stage-1 abort tests (under `-tags ze_bgp`) fail before edit and pass after edit.
4. **Phase 4: Routing Protocol & L2TP Plugins (`bfd`, `isis`, `ldp`, `ospf`, `l2tp-auth-radius`)** -- move `issuequeue.Start`, `setRouteInstallClient`, `newEngine`/`wireV4Engine` (`locrib.Default()`, opaque/SR consumers, gauges), and L2TP RADIUS EventBus subscriptions behind Stage 1; guard `bfd` `api.SetService`/`SetRawCaptureProvider`, `l2tp-auth-radius` retire/close calls, and `issueQueue`/`routeInstallPtr` with CAS/ownership flags.
   - Tests: `inertness_test.go` in each of the 5 packages
   - Files: `internal/component/bfd/bfd.go`, `internal/plugins/isis/register.go`, `internal/plugins/ldp/register.go`, `internal/plugins/ospf/register.go`, `internal/component/l2tp/plugins/authradius/register.go`
   - Verify: per-package Stage-1 abort tests pass under their respective build tags (`ze_bfd`, `ze_isis`, `ze_ldp`, `ze_ospf`, `ze_l2tp`).
5. **Phase 5: System, DNS, Redistribution, Monitoring & DDoS Plugins (`as112`, `connected`, `static`, `redistribute`, `dnsresolver`, `flow-export`, `traffic-usage`, `vrrp`, `cos`, `anomaly-observe`, `ddos-observe`, `ddos-flowspec`, `ddos-local`)** -- move `issuequeue.Start`, `locrib.Default()`, `res.start()`, `mon.Start()`, `startMaxDurationWorker`, `consumerHeld.beginRun()`, EventBus/`SubscribeCollectNotify` subscriptions, and global stores behind Stage 1; CAS/ownership-guard every teardown `defer`.
   - Tests: `inertness_test.go` in each of the 13 packages
   - Files: the 13 system plugin files listed in `## Files to Modify`
   - Verify: per-package Stage-1 abort tests pass, and `internal/component/plugin/all/runner_inertness_test.go` turns completely GREEN across all 98 runners.
6. **Phase 6: End-to-End Functional Verification** -- add `test/plugin/runner-stage1-abort.ci` proving real daemon Stage-1 refusal safety and normal startup.
   - Tests: `test/plugin/runner-stage1-abort.ci`
   - Files: `test/plugin/runner-stage1-abort.ci`
   - Verify: functional test fails on unfixed runner and passes once all phases land.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every one of the 32 audited runners has its pre-`p.Run` side effects moved behind Stage 1 and has a Stage-1-abort unit test |
| Idempotent Stage 2 redelivery | Every runner initializing in `OnConfigure` guards one-time setup against failed-removal configure redelivery (no duplicate goroutines or subscriptions) |
| Ownership-guarded teardown | Every global clear (`Store(nil)`, `Swap(nil)`, `SetService(nil)`, `SetOriginValidator(nil)`, `CloseBackend()`, `Register*(nil)`) is gated by CAS or an ownership flag set only after Stage 1 |
| No straight-line teardown leaks | Every goroutine stop/join, unsubscribe, and backend close runs via `defer` (never skipped by `if err := p.Run(...); err != nil { return 1 }`) |
| Nil-safe early handlers | `OnExecuteCommand` and `OnDoctorCheck` handlers tolerate nil lazy state if invoked before `OnStarted` completes |
| Mode refusals preserved | All 8 `!p.IsInternal()` refusals, 3 `bus == nil` refusals, and 3 inline no-op stubs retain exact pre-Stage-1 exit codes |
| Whole-tree guard non-vacuity | `TestNoRunnerHasPreStage1SideEffectsAST` and `TestAllRunnersInertOnStage1Abort` cover all 98 runners and fail when a pre-`p.Run` side effect is seeded |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| All 32 runners inert before Stage 1 | Scoped `go test -race` with feature tags across all 32 packages |
| Whole-tree AST + Stage-1-abort guard | `./le test unit pkg path ./internal/component/plugin/all/` (with full feature tags) |
| End-to-end functional proof | `./le test functional plugin` including `test/plugin/runner-stage1-abort.ci` |
| Plugin boundary and declaration checks | `./le plugin boundary check`, `./le plugin declarations check`, `./le arch tier check` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Unauthenticated / failed Stage 1 host isolation | A plugin failing Stage 1 MUST NOT execute host-level syscalls (netlink route/MPLS guard writes, container sweeps, XFRM/IPsec dataplane load/close, tc qdisc restore) |
| Denial-of-service on live instance | A second/respawned runner aborting at Stage 1 MUST NOT clobber a live instance's globals (`activeManager`, `activeStore`, `activeResponder`, `api.Service`, `OriginValidator`, `dynamicHandler`, active traffic/IKE backend) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on startup ordering mismatch | Check whether initialization belongs in `OnConfigure` (Stage 2) vs `OnStarted` (Stage 5) |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Use existing `OnConfigure` (guarded against redelivery) and `OnStarted` callbacks instead of adding a new SDK hook | Adding `sdk.RunWithInit` or a new `pkg/plugin/sdk` lifecycle callback | `(*sdk.Plugin).Run` already guarantees neither `OnConfigure` nor `OnStarted` runs when Stage 1 fails; `flowspec-firewall` and `sysrib` already prove this pattern in production without widening the published `pkg/` API (`ai/META-DECISIONS.md`). |
| Guard every process-global and backend teardown with CAS or a per-run ownership flag | Unconditional `defer Store(nil)` / `defer CloseBackend()` | Unconditional defers executed on Stage 1 abort clobber live state owned by another instance or test fixture. |
| Enforce inertness via a composition-root AST + runtime Stage-1-abort test in `internal/component/plugin/all/runner_inertness_test.go` | Adding a new `./le plugin inertness check` subcommand | Matches existing composition-root guards (`TestEveryDeclaredShapeIsOneStage1Accepts`, `TestNoRegistrationDependsOnAFIBWriter`), runs in normal `go test`, requires zero hardcoded plugin lists, and adds no new `le` CLI surface. |

## Known Limitations
- External third-party binaries compiled outside this repository remain governed by `sdk.RunOrDeclare` (`pkg/plugin/sdk/sdk_query.go`) and `./le plugin conformance check run`; Go AST inspection in `internal/component/plugin/all/runner_inertness_test.go` applies only to compiled-in product `RunEngine` registrations in this repository (`plan/learned/013-inertness-is-unreachability.md`).
- `sdk.SignalContext()` installs OS signal notification for the plugin context before `p.Run` and cancels it via `defer cancel()` on return; keeping it local to `RunEngine` is harmless for normal daemon/subprocess execution and cancels cleanly on Stage 1 abort.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-3 all demonstrated across all 32 runners and the full 98-runner composition root
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches. No agent runs any `./le verify` command: GitHub CI verifies the pushed commit, and the full verification is Thomas's release check. A red CI reports gets a fix commit
- [ ] Feature code integrated (`internal/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (N-A: no numeric inputs added)
- [ ] Functional `.ci` tests for end-user behavior
- [ ] Interop tests for protocol features (N-A: Scope is plugin, no wire change)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/fixes/spec-plugin-runner-inertness.md` only, in the same `./le commit create` script

## Review Gate
