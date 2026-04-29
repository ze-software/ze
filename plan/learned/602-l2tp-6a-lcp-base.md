# 602 -- L2TP Phase 6a (PPP + LCP base)

## Context

Phases 1-5 delivered the L2TP control plane and the kernel data-plane handshake: the subsystem binds UDP, the reactor runs the control FSM, and the Linux kernel worker programs `l2tp_ppp` tunnels/sessions and hands back a `pppSessionFDs` triple (pppox, chan, unit). Up through Phase 5 the successful `setupSession` path was *silent* -- the worker stored fds in its own map, nobody downstream was notified, and LCP was never attempted. Phase 6a builds the standalone PPP package, runs LCP to the Opened state on every new session, and wires the notification edge from kernel worker -> reactor -> ppp.Driver.

The deliverable is a single-binary `ze` that can (on Linux with `iface/netlink`) terminate an L2TPv2 session from an accel-ppp LAC through LCP to "ready for auth" without gaining a dependency on pppd, rp-pppoe, or any external daemon.

## Decisions

- **Package at `internal/component/ppp/`, not `internal/component/l2tp/ppp/`.** PPP is transport-agnostic; PPPoE will reuse the same Driver with a different fd source. Nesting under l2tp would have required a rename-and-move once PPPoE lands. Cost: one extra peer package in `internal/component/`. Benefit: zero churn on spec-6b/6c and the eventual PPPoE spec.
- **Per-session goroutine via Go runtime poller, NOT a reactor + worker pool.** The L2TP reactor is single-threaded because there is ONE UDP socket; PPP has N chan fds, one per session, so the coupling reason is gone. Go's netpoll makes blocking reads cheap even with thousands of fds. Chose this over mirroring the L2TP reactor design that the integration doc recommended.
- **Channel API for Driver (`SessionsIn`, `EventsOut`), not method callbacks.** The L2TP reactor already has a `select` loop; adding one more channel arm is cheaper than adding a callback-on-teardown path that would have to acquire `tunnelsMu` from an unrelated goroutine. Chose channels over callbacks because the reactor's lock-ordering invariants are already set.
- **New `kernelSetupSucceeded` event symmetric with existing `kernelSetupFailed`.** Phase 5 already had the failure half; the success half just never got added. Chose "add the symmetric event" over "inject a callback into pppSetup" because the former follows the existing channel-between-worker-and-reactor pattern and keeps the worker free of PPP-layer dependencies.
- **Reactor uses `pppDriverIface` (an interface), not `*ppp.Driver` directly.** Tests need to substitute a fake without standing up an iface backend. Chose the interface over "expose a `ppp.NewFakeDriver` helper in production code" because the interface is tiny (two methods) and the fake lives in `reactor_ppp_test.go` where it belongs.
- **Added `ppp.NewProductionDriver(logger, authHook, backend)` helper.** `DriverConfig.Ops` is `pppOps` (unexported); l2tp cannot construct it. Chose the helper over "export pppOps" because the ioctl surface is a package-private detail and the helper keeps the contract narrow.
- **Subsystem gracefully skips PPP when `iface.GetBackend()` returns nil.** Test paths and non-Linux don't have an iface backend. Chose "log warn and continue" over "hard-fail Start" because the L2TP userspace control plane is still useful in that configuration and the existing subsystem tests pre-date iface-backend wiring.

## Consequences

- **PPP is a reusable component.** Whoever writes the PPPoE spec can call `ppp.NewProductionDriver` directly; the fd acquisition is the only piece they need to implement. No refactor.
- **L2TP reactor now dispatches on four event classes:** rx packets, timer ticks, kernel errors, kernel successes, PPP events. The `run` select got two new arms. Each new arm grabs `tunnelsMu` only for the brief map/session lookup, matching the existing lock discipline.
- **`kernelSetupEvent` grew past 128 bytes.** Added a gocritic `rangeValCopy` fix in `enqueueKernelEvents` (index instead of range-copy). Anything that copies the event in a hot loop from now on needs the same treatment.
- **Stub auth hook is deliberately loud.** It logs a WARN on every call so a partial deploy (6a shipped but 6b not yet) is visible in production logs. `/ze-review` for 6b will delete the stub.
- **`teardownSession` picked up a `//nolint:unparam` comment.** All current callers pass `cdnResultGeneralError`; the parameter stays because RFC 2661 §4.4.1 defines distinct codes that will plug in via later specs (admin shutdown, busy, no-bearer). Documented in the function comment; a later spec that actually uses a different code should remove the nolint.
- **New deferrals recorded (not closed by 6a):** `.ci` functional test of LCP-Opened + MTU against accel-ppp peer, and LCP Restart timer + IRC/ZRC retransmit for hostile-peer robustness. Both go to spec-l2tp-7-subsystem. Already in `plan/deferrals.md`.

## Gotchas

- **`DriverConfig.Ops` is unexported.** Without `NewProductionDriver`, every external caller of `ppp.NewDriver` is blocked at compile time. The helper is non-optional for any transport that isn't in the ppp package.
- **The reactor's `pppEventsOut` channel is mirrored from `pppDriver.EventsOut()` on `SetPPPDriver`.** Writing `case ev := <-r.pppDriver.EventsOut()` inline in the run-loop select would re-evaluate the receiver expression on every iteration; caching it once matches the project's zero-alloc hot-path style.
- **`successCh` is nil-tolerant in the kernel worker.** Tests that only exercise teardown/failure paths pass `nil`, and `reportSuccess` short-circuits. Production MUST pass a real channel; missing that wiring silently drops every successful session with no runtime error.
- **Start/Stop order matters and is not what one would guess.** Start: reactor -> PPP driver -> kernel worker -> timer. Stop: timer -> reactor -> PPP driver -> kernel worker -> listener. Reactor before driver on Start so the driver has a live SessionsIn consumer; driver before worker on Stop so session goroutines exit before the worker drains fds.
- **Running `make ze-verify` after growing a struct past 128 bytes** is the only way to catch `rangeValCopy`. The linter does not flag growing types retroactively; only iterators over the newly large type get complained about.
- **Subsystem tests pass BECAUSE iface backend is nil in tests.** That is not a bug, it is the designed fallback. If a future spec wires a fake iface backend globally for tests, the PPP driver will start construction during `TestSubsystem_StartEnabledWithListener` and the test will need the same `SetProbeKernelModulesForTest`-style neutralization it already uses for kernel modules.

## Files

New:
- `internal/component/ppp/{doc,manager,session,session_run,start_session,events,auth_hook}.go`
- `internal/component/ppp/{frame,frame_linux,frame_other,lcp,lcp_fsm,lcp_options,echo,proxy,ops,mtu_linux,mtu_other}.go`
- `internal/component/ppp/*_test.go` (12 test files, including `helpers_test.go` and `export_test.go`)
- `internal/component/l2tp/reactor_ppp_test.go`

Modified:
- `internal/component/l2tp/kernel_event.go` -- proxy LCP fields on kernelSetupEvent; new kernelSetupSucceeded
- `internal/component/l2tp/kernel_linux.go` -- successCh field + reportSuccess; signature update
- `internal/component/l2tp/kernel_other.go` -- non-Linux signature parity; linter compile-time references
- `internal/component/l2tp/reactor.go` -- kernelSuccessCh arm, pppEventsOut arm, handleKernelSuccess, handlePPPEvent, SetPPPDriver, pppDriverIface
- `internal/component/l2tp/subsystem.go` -- ppp.NewProductionDriver wiring; reverse-order Stop
- `internal/component/l2tp/session_fsm.go` -- nolint + doc on teardownSession's resultCode
- `internal/component/l2tp/kernel_linux_test.go` -- newTestWorkerWithSuccess helper; TestKernelWorkerEmitsSucceeded; TestKernelWorkerSucceededCoexistsWithNilChannel
- `internal/component/l2tp/reactor_kernel_test.go` -- all newKernelWorker / SetKernelWorker callsites updated for 3-channel shape
- `internal/component/ppp/manager.go` -- NewProductionDriver helper
- `internal/component/ppp/session_run.go` -- RFC 1661 §5.7 comment at sendCodeReject, RFC 2661 §18 comment at proxy short-circuit
