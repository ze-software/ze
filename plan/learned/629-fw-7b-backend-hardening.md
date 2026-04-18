# 629 -- fw-7b-backend-hardening

## Context

The fw-7 VPP traffic-backend review landed seven rounds of fixes with no
Apply-path unit tests. Only pure-function translation was covered by
`translate_test.go`; the create/update/undo/reconcile/orphan branches
added during pass 7 were reachable only through `.ci` functional tests
against a running VPP. Separately, `traffic.Backend.Apply` synthesised
its own `context.WithTimeout(context.Background(), 5s)` inside the VPP
backend, which meant a daemon SIGTERM mid-reload blocked for the full 5
seconds when VPP was unreachable. The goal was to plumb a real context
through `Backend.Apply` and introduce a narrow test seam so every future
change to the VPP Apply path could be exercised without a VPP daemon.

## Decisions

- `Backend.Apply` takes `context.Context` as the first parameter across
  the interface and both backends. The netlink backend accepts but
  ignores the ctx (vishvananda/netlink has no ctx-aware syscalls;
  documented). VPP honors it for `conn.WaitConnected`. Chose "first
  param" over an optional setter because the optional form leaves
  ctx-less call sites valid forever.
- The traffic component's `runEngine` synthesises the ctx from its own
  plugin lifetime rather than waiting on a new SDK surface.
  `signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)`
  gives subprocess plugins real cancellation on daemon shutdown without
  needing an SDK change; internal-mode plugins already unwind via the
  SDK pipe close, so this is a belt-and-braces safety net for them.
  Chose component synthesis over adding ctx to `OnConfigApply` now
  because the SDK change would ripple through every plugin and delay
  the concrete win (VPP WaitConnected cancellable end-to-end).
- `vppOps` interface (4 methods: `dumpInterfaces`, `policerAddDel`,
  `policerDel`, `policerOutput`) stays unexported. `govppOps{ch
  api.Channel}` is the stateless production adapter. `fakeOps` records
  calls and scripts failures by policer name or by Nth-addDel count.
  Chose a narrow interface over mocking the whole GoVPP `api.Channel`
  (which has 8 methods spread across Channel / RequestCtx /
  MultiRequestCtx) so the test seam costs four trivial methods to
  maintain.
- `applyWithOps(ops, desired)` split out of `Apply` so tests inject
  `fakeOps` directly without touching the connector or channel
  lifecycle. `Apply` keeps the ctx/lock/connector preamble; `applyWithOps`
  is what tests call (under `b.mu` via an `applyWithOpsLocked` helper).
- `fakeOps.failOnNthAddDel` counter handles deterministic 2-interface
  partial-failure tests despite Go map iteration being unordered. Chose
  count-based scripting over forcing a sorted iteration order in
  production.
- Inlined the four `sendPolicerAddDel` / `sendPolicerDel` /
  `sendPolicerOutput` / `dumpInterfaceIndex` helpers into `govppOps`
  methods. Chose inlining over keeping them package-level because after
  the refactor they had no other callers, and the inlined versions
  are the same length.
- Added `//go:build linux` to `ops.go`. Required once the `//nolint:unused`
  directive came off; `vppOps` is only consumed by `backend_linux.go` so
  darwin builds stay clean without the directive.

## Consequences

- Future backends with IPC surfaces (netconf, gNMI, any out-of-process
  RPC) can copy the `vppOps` / `govppOps` / `fakeOps` pattern and get
  full branch coverage of their Apply path in unit tests. The pattern
  is documented in `docs/functional-tests.md` section 6.
- `Backend.Apply(ctx, ...)` is now the interface contract for every
  backend registered via `traffic.RegisterBackend`. Adding a new backend
  requires implementing the ctx-aware signature; the netlink backend's
  doc block is the template for "accept but cannot honor" cases.
- `pkg/plugin/sdk/signal.go` exposes `sdk.SignalContext()` as THE way
  for any plugin to get a ctx that cancels on SIGINT/SIGTERM. 41 plugin
  runEngines across `internal/plugins/*` and `internal/component/*` are
  wired to it. Centralising the signal set means a future SIGHUP (live
  reload) lands in one place and every plugin picks it up.
- Subprocess (fork-mode) plugins now unblock on SIGTERM instead of
  being killed by the default Go signal disposition. Internal
  (goroutine-mode) plugins are unaffected beyond a belt-and-braces
  safety net -- ze's own main handler still drives shutdown via pipe
  close.
- Eight new unit tests run in <10ms each (including the warn-path
  coverage for `reconcileRemovals` added in the /ze-review pass),
  making the VPP Apply path safe to refactor further without spinning
  up `test/traffic/011`, `012` each time.

## Gotchas

- The `//nolint:unused` directive on `vppOps` (left over from when the
  interface was scaffolded as an empty consumer) was NOT the right
  solution once the interface shipped. A lint-clean approach needed a
  build tag (`//go:build linux`) so both GOOS targets treat the file
  consistently.
- The pre-existing `logger()` in `trafficvpp.go` was flagged as
  "unused" on darwin (used only in `backend_linux.go`). Moved to
  a new `logger_linux.go` with `//go:build linux` rather than tagging
  the whole `trafficvpp.go` file -- that file carries the package
  doc comment, which must stay visible on all GOOS.
- `make ze-verify-fast` was blocked during this session by an unrelated
  compile error in `internal/component/plugin/coordinator.go` from the
  parallel `spec-rs-fastpath-3-passthrough` session (interface
  `ReactorLifecycle` extended with `ForwardUpdatesDirect` but
  `*Coordinator` not updated). Logged to `plan/known-failures.md`;
  targeted `go test -race` and `golangci-lint run` on the traffic
  packages were used for verification instead.
- Map-iteration order nondeterminism in `applyAll` meant the naive
  "two-iface, one fails" partial-failure test was flaky. The
  `failOnNthAddDel` counter and count-based assertions (rather than
  index-based) make the test deterministic regardless of iteration
  order.
- `signal.NotifyContext` only helps when the plugin runs as its own
  process (subprocess mode). In internal goroutine mode, ze's own
  signal handler catches SIGTERM first and tears down the plugin via
  pipe close; the plugin's own signal handler still fires but is
  redundant. Both mechanisms are safe to co-exist.
- `TestApplyContextCancelMidWait` was tightened in the /ze-review pass
  to assert Apply returns within 500ms of cancellation. Without that
  bound, the test could have passed on a slow CI by way of the natural
  5s WaitConnected timeout -- same assertion (`errors.Is(err,
  context.Canceled)`), different code path, invalid verification. Rule
  for similar async tests: always bound the latency if the mechanism
  being verified has a natural fallback timeout.

## Files

Traffic + SDK signal helper:
- `internal/component/traffic/backend.go` — `Backend.Apply(ctx, desired)` interface.
- `internal/component/traffic/register.go` — `runCtx` synthesis via `sdk.SignalContext` + three Apply call sites + p.Run.
- `internal/plugins/traffic/netlink/backend_linux.go` — ctx-aware Apply, documented noop.
- `internal/plugins/traffic/vpp/backend_linux.go` — ctx-aware Apply, `applyWithOps`, `govppOps` adapter, `applyAll` / `applyInterface` / `reconcileRemovals` take `vppOps`.
- `internal/plugins/traffic/vpp/ops.go` — `//nolint:unused` removed, `//go:build linux` added.
- `internal/plugins/traffic/vpp/apply_test.go` (new) — `fakeOps` + 8 tests covering AC-3..AC-10 + warn-path.
- `internal/plugins/traffic/vpp/trafficvpp.go` — trimmed to keep just package doc + loggerPtr.
- `internal/plugins/traffic/vpp/logger_linux.go` (new) — `logger()` helper tagged linux.
- `internal/component/traffic/backend_test.go` — `fakeBackend.Apply` signature match.
- `pkg/plugin/sdk/signal.go` (new) — `sdk.SignalContext()` helper.

41 plugin runEngines rewired to `sdk.SignalContext()`:
- `internal/plugins/{bfd,sysctl,ntp,sysrib,fib/{p4,vpp,kernel},iface/dhcp}/*.go`
- `internal/component/{iface,vpp}/register.go`
- `internal/component/bgp/plugin/register.go`
- `internal/component/bgp/plugins/{rib,filter_prefix,filter_community,role,softver,filter_modify,aigp,gr,filter_aspath,healthcheck,route_refresh,rr,filter_community_match,watchdog,rpki,hostname,llnh,rpki_decorator,bmp,persist,adj_rib_in}/*.go`
- `internal/component/bgp/plugins/nlri/{mup,vpn,evpn,labeled,rtc,vpls,ls,flowspec,mvpn}/*.go`

Docs + bookkeeping:
- `docs/architecture/core-design.md` — traffic backend table + ctx/vppOps paragraph.
- `docs/functional-tests.md` — new "Backend Apply-Path Unit Tests" section.
- `plan/known-failures.md` — pre-existing Coordinator compile error logged.
