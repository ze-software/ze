# Backend Apply-Path Hardening: Context and the Ops Seam

Two problems in the [VPP traffic backend](fw-7-traffic-vpp.md). Its Apply path
had no unit tests: only pure translation was covered, and the create, update,
undo, reconcile and orphan branches were reachable only through a `.ci` test
against a running VPP. And `Apply` synthesised its own
`context.WithTimeout(context.Background(), 5s)`, so a daemon SIGTERM during a
reload blocked for the full five seconds when VPP was unreachable.

## `Backend.Apply` takes a context

<!-- source: internal/component/traffic/backend.go -- Backend.Apply -->
<!-- source: internal/plugins/traffic/netlink/backend_linux.go -- ctx-aware Apply, documented no-op -->

The context is the FIRST parameter, across the interface and both backends. An
optional setter would leave every context-less call site valid forever.

The netlink backend accepts the context and cannot honor it, because
`vishvananda/netlink` has no context-aware syscall. Its doc block is the template
for an "accept but cannot honor" case. The VPP backend honors it in
`WaitConnected`.

## The context comes from the plugin's own signal handler

<!-- source: pkg/plugin/sdk/signal.go -- SignalContext -->
<!-- source: internal/component/traffic/register.go -- runCtx synthesis -->

The traffic component's `runEngine` synthesises the context from its own plugin
lifetime with `sdk.SignalContext()`, rather than waiting for a new SDK surface.
Adding a context to `OnConfigApply` would ripple through every plugin and delay
the concrete win.

`sdk.SignalContext()` is now THE way for any plugin to get a context that cancels
on SIGINT and SIGTERM, and it is wired into 41 plugin `runEngine` functions.
Centralising the signal set means a future SIGHUP lands in one place.

It only changes behavior for a subprocess-mode plugin, which now unblocks on
SIGTERM instead of dying under the default Go signal disposition. An
internal-mode plugin already unwinds through the SDK pipe close, and this is a
second safety net for it.

## The `vppOps` seam

<!-- source: internal/plugins/traffic/vpp/ops.go -- vppOps interface -->
<!-- source: internal/plugins/traffic/vpp/ops_linux.go -- govppOps production adapter -->
<!-- source: internal/plugins/traffic/vpp/apply_test.go -- fakeOps -->

`vppOps` is a narrow unexported interface. `govppOps` is the stateless production
adapter over a GoVPP channel. `fakeOps` records calls and scripts failures by
policer name or by an Nth-addDel counter.

Mocking the whole GoVPP `api.Channel` means eight methods spread over Channel,
RequestCtx and MultiRequestCtx. The narrow interface costs a handful of trivial
methods.

`applyWithOps(ops, desired)` is split out of `Apply` so a test injects `fakeOps`
with no connector and no channel lifecycle. `Apply` keeps the context, lock and
connector preamble.

`fakeOps.failOnNthAddDel` makes a two-interface partial-failure test
deterministic. Go map iteration is unordered, and count-based scripting is
cheaper than forcing a sorted iteration order in production code.

Any future backend with an IPC surface (netconf, gNMI, an out-of-process RPC) can
copy the `vppOps`, `govppOps`, `fakeOps` shape and get full branch coverage of
its Apply path in unit tests.

## Build-tag placement

- `ops.go` carries `//go:build linux`. Once `//nolint:unused` came off, the build
  tag is what keeps both GOOS targets consistent: `vppOps` is consumed only by
  `backend_linux.go`.
- `logger()` moved to `logger_linux.go` rather than tagging `trafficvpp.go`,
  because that file carries the package doc comment, which must stay visible on
  every GOOS.

## A test rule this produced

**Bound the latency when the mechanism under test has a natural fallback
timeout.** `TestApplyContextCancelMidWait` asserts that `Apply` returns within
500 ms of cancellation. Without the bound, the test passes on a slow machine
through the natural 5-second `WaitConnected` timeout: the same assertion
(`errors.Is(err, context.Canceled)`), a different code path, and an invalid
verification.
