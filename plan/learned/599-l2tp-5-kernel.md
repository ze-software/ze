# 599 -- L2TP Linux Kernel Integration

## Context

Phases 1-4 built the L2TP userspace stack (wire format, reliable delivery,
tunnel and session state machines) but sessions that reached Established had
no kernel counterpart -- no l2tp_ppp kernel tunnel, no PPPoL2TP socket, no
pppN interface. Phase 5 wires the Linux kernel L2TP module so established
sessions program kernel state that the data plane can forward. This is the
prerequisite for Phase 6 (PPP engine) to obtain the `/dev/ppp` fds for
LCP/auth/IPCP negotiation, and ultimately for subscriber IP traffic to flow
through pppN.

## Decisions

- Kernel worker is a single long-lived goroutine per reactor processing
  events from a buffered channel, over a goroutine per session. Matches
  ze's `goroutine-lifecycle` rule (no per-event goroutines in hot paths).
- `kernelOps` struct carries injectable function fields for every syscall,
  over an interface abstraction. Three call sites is the threshold for
  abstracting; ze has one production `newKernelOps` and one fake in tests.
- FSM-to-reactor signalling uses a boolean flag (`kernelSetupNeeded`) on
  the session plus a slice of pending teardowns on the tunnel, over a
  callback. The reactor already walks tunnels after `Process()` returns
  so this adds no new traversal.
- `newSubsystemKernelWorker` returns nil on genl resolve failure, over
  failing `Start()`. Userspace control still functions; only the kernel
  data plane is disabled. Operators on systems without the l2tp module can
  still accept SCCRQ and observe protocol behavior.
- Module probe is injectable via a package-level `probeKernelModulesFn`
  (exported through `export_test.go` as `SetProbeKernelModulesForTest`),
  over environment-variable gating. Zero production cost, zero contract
  exposure, tests can run without root.
- Tie-breaker losers' pending kernel teardowns are propagated up through
  `discardTunnelLocked` -> `resolveTieBreakerLocked` -> `locateTunnelLocked`
  -> `handle`, which enqueues them after releasing `tunnelsMu`. Over
  enqueueing under the lock (risks deadlock if the worker blocks on state
  the reactor owns) or silently dropping them (kernel state leak).
- Subsystem `Stop` and `unwindLocked` stop order: timers -> reactors ->
  workers -> listeners. Reactors stop before workers so no new
  `kernelSetupEvent` can be enqueued after `TeardownAll` runs, satisfying
  AC-14 ("all kernel resources torn down on Stop()") without a
  `stopped`-check inside `setupSession`.
- `SetKernelWorker` panics on second call (tracked via dedicated
  `kernelWorkerSet bool`) over silently overwriting. The field pair is
  read by the reactor goroutine; a torn write would race.
- `.ci` functional tests for kernel integration are explicitly out of
  scope: they require root + kernel modules + a real L2TP peer. Coverage
  is via Go unit tests with mock `kernelOps`. Phase 7 (subsystem wiring)
  adds end-to-end `.ci` coverage of the full daemon lifecycle.

## Consequences

- Phase 6 (PPP engine) consumes `pppSessionFDs` stored in
  `w.sessions[sessionKey]`: the channel fd drives LCP, the unit fd drives
  IPCP. Worker code does not block on PPP negotiation; Phase 6 adds a
  separate PPP worker pool.
- Phase 7 adds the operator-facing config note about `l2tp_ppp` /
  `pppol2tp` kernel module dependency (deferred).
- Subsystem `Stop` now reliably cleans up kernel state before returning.
  Operators restarting ze do not leak kernel tunnels or pppN interfaces.
- A tunnel that loses a tie-breaker race and held established sessions
  now correctly releases its kernel resources. Previously those sessions
  were removed from ze's view but not from the kernel.

## Gotchas

- `SetKernelWorker` must be called BEFORE `reactor.Start()`. The reactor
  goroutine reads `r.kernelErrCh` via select; a write after Start races
  against the run loop. The original implementation violated this contract
  in tests (caught by `-race`).
- Initial Phase 4 wiring left `s.kernelWorkers` as an empty slice that
  `unwindLocked` / `Stop` iterated without effect -- the slice was never
  populated. The lint pass flagged all kernel production code as unused
  until `newSubsystemKernelWorker` was called from `Start`.
- `probeKernelModules` must use a fresh `context.WithTimeout` per
  `modprobe` invocation. A single shared 10s deadline lets a hung first
  call starve the fallback.
- `unix.Close` on rollback paths must be annotated `//nolint:errcheck`
  with a reason; the banned `_ = unix.Close(fd)` pattern is rejected by
  the `block-ignored-errors` hook.
- Sockaddr binary construction via `unsafe.Pointer` and ioctl wrappers
  need `//nolint:gosec` because the kernel interface has no
  Go-friendly accessor. Rationale documented inline.
- Dev machines without l2tp kernel modules need `SetProbeKernelModulesForTest`
  to bypass `modprobe` in subsystem tests; CI machines likewise.
- Tie-breaker propagation required changing four function signatures
  (`discardTunnelLocked`, `resolveTieBreakerLocked`, `locateTunnelLocked`,
  `handle`). Adding a reactor-level "pending discards" slice would have
  been less invasive but would have required an explicit drain step.

## Files

- `internal/component/l2tp/genl_linux.go` -- L2TP Generic Netlink constants, tunnel/session create/delete, attribute marshallers
- `internal/component/l2tp/genl_linux_test.go` -- attribute encoding, LNS mode, sequencing, padding, boundary IDs (6 tests)
- `internal/component/l2tp/pppox_linux.go` -- PPPoL2TP socket, sockaddr layout, `/dev/ppp` ioctls, channel/unit setup
- `internal/component/l2tp/pppox_linux_test.go` -- sockaddr layout, IPv6 rejection, htons (3 tests)
- `internal/component/l2tp/kernel_linux.go` -- kernelOps, kernelWorker, setupSession, teardownSession, probeKernelModules, newSubsystemKernelWorker
- `internal/component/l2tp/kernel_linux_test.go` -- worker lifecycle, idempotent tunnel, teardown order, partial-failure rollback, TeardownAll (10 tests)
- `internal/component/l2tp/kernel_other.go` -- non-Linux stubs
- `internal/component/l2tp/kernel_event.go` -- event types
- `internal/component/l2tp/reactor.go` -- kernel event collection, handleKernelError, SetKernelWorker, tie-breaker teardown propagation
- `internal/component/l2tp/reactor_kernel_test.go` -- reactor kernel-event wiring (6 tests)
- `internal/component/l2tp/subsystem.go` -- probeKernelModulesFn, worker construction, Stop ordering
- `internal/component/l2tp/subsystem_test.go` -- probe-override in Start tests
- `internal/component/l2tp/listener.go` -- SocketFD()
- `internal/component/l2tp/session.go`, `session_fsm.go`, `tunnel.go` -- kernelSetupNeeded flag, pendingKernelTeardowns
- `internal/component/l2tp/export_test.go` -- SetProbeKernelModulesForTest
