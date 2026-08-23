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
<!-- source: internal/plugins/traffic/vpp/timeout_linux.go -- newGovppOps, the constructor that binds the reply deadline -->
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

`newGovppOps` is the one place that builds the production adapter, and it
installs the reply deadline on the channel before it returns. That placement is
part of the seam contract, not an implementation detail of one call site. GoVPP
leaves a new channel on `core.DefaultReplyTimeout`, which is 0, and
`receiveReplyInternal` reads a value at or below zero as `maxInt64`.
`Channel.ReceiveReply` takes no context either, so the `ctx` that `Apply`
receives cannot end the wait, and `Apply` holds `b.mu` across the whole call. A
VPP that accepts a request and never answers would therefore stop the backend
accepting any further apply for the life of the process.

The compiler does not keep that one place the only one: `govppOps` is
unexported, so a bare `&govppOps{ch: ch}` stays legal anywhere in the package.
`TestGovppOpsIsBuiltOnlyByItsConstructor` (`ops_construction_test.go`) is what
does. It parses the package's own sources and fails on a `govppOps` built
anywhere but inside the constructor, and fails again on finding none at all, so
it cannot pass by matching nothing. The scan is build-tag blind, so it runs on
every GOOS rather than only where the backend compiles.

The guard is a ratchet against the regression that occurred, not a proof that an
unbounded facade cannot be built. It sees the three forms that name the type
directly: a composite literal, `new(govppOps)`, and a `var` declaration with no
initializer. It does not see one built as part of another value, such as the
elided inner literal in `[]govppOps{{ch: ch}}`, because that needs the type of
an expression and therefore `go/types` rather than a parse. The function's own
comment carries that list.

Binding in the constructor rather than at connect time is what makes the value
deterministic. The channel comes from a pool on the one `Connection` every
plugin shares, and `(*Channel).Reset` drains the buffers while leaving
`replyTimeout` alone, so a pooled channel arrives carrying whatever its previous
owner set. The constructor is the only code that runs once per use of a channel.

The deadline defaults to 10s and clamps to 1s..60s, read from
`ze.traffic.vpp.reply-timeout`. Zero is refused rather than honored, because
zero is GoVPP's spelling of "no deadline". The firewall VPP backend carries the
same shape under `ze.firewall.vpp.reply-timeout`, and `core-design.md` publishes
those numbers under "Firewall reconcile concurrency" for the two FIREWALL
backends. This backend is not in that table: it matches the numbers on purpose,
so an operator meets one pair of bounds across every ze dataplane, and states
them locally rather than importing `firewall.MaxBackendDeadline`. That constant
exists so three firewall things agree, the nft clamp, the vpp clamp and the last
finite bucket of the apply-latency histogram. Traffic has none of the three, so
the import would buy coupling and no agreement.

The bound is per ROUND TRIP, not per apply, and the difference is what an
operator needs to know. One `Apply` issues a sequence of requests: the interface
dump, the policer dump, then a policer add and an output bind for each class,
plus the classify table, session and bind for each interface that steers. Each
one gets its own deadline. On failure the undo list issues more. Against a VPP
that accepts every request and answers none, `b.mu` is therefore held for
roughly the request count times the deadline, which is minutes for a
many-interface configuration rather than the 10 seconds the default reads like.
`Channel.ReceiveReply` still takes no context, so the caller's `ctx` cannot cut
the apply short either. What the deadline buys is termination: the apply ends,
reports an error and releases the lock, instead of never ending at all.

No timeout sentinel crosses the traffic `Backend` boundary. The firewall
equivalent exists to drive a counter and a rollback skip, and traffic has
neither consumer, so a reply-deadline failure surfaces as the wrapped GoVPP
error like any other VPP failure.

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
