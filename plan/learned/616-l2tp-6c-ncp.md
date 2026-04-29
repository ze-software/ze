# 616 -- l2tp-6c NCP layer (IPCP + IPv6CP)

## Context

Spec-l2tp-6c closes out the PPP control-plane stack that spec-l2tp-6a (LCP)
and spec-l2tp-6b (auth) started: the NCPs that actually hand a subscriber
an IP address. IPCP (RFC 1332) negotiates IPv4 plus DNS hints (RFC 1877),
IPv6CP (RFC 5072) negotiates the 64-bit interface identifier that lets the
kernel auto-derive a link-local `fe80::<iid>/64`. Before this spec the
PPP session emitted EventSessionUp as soon as auth succeeded, leaving the
transport in a lie: the session was "up" but had no IP on pppN. Post-6c,
EventSessionUp is gated on every enabled NCP reaching Opened, and
EventSessionIPAssigned carries the values the L2TP subsystem needs to
inject the subscriber route into the redistribute path in Phase 7.

## Decisions

- **Reuse the RFC 1661 FSM verbatim for NCPs** over a generic parameterized
  type or a wrapper struct. RFC 1661 §2 explicitly says the FSM is shared
  across every NCP. ze calls `LCPDoTransition` directly from `ncp.go` and
  keeps the `LCP*` type prefix. The rename `lcp_fsm.go` -> `ppp_fsm.go`
  with an updated doc block makes the sharing explicit without three
  type-rename churn.
- **LNS-role deviation from AC-2** (initial CONFREQ carries assigned
  local, not `0.0.0.0`). Spec text reflected LAC-client behavior;
  ze is always the LNS in the umbrella scope. Pragmatic choice agreed
  in the spec handoff: emit EventIPRequest BEFORE the first CONFREQ
  so the handler picks the address, then ship the CONFREQ with the
  assigned value. Documented on `runNCPPhase` and in the Deviations
  section of the spec.
- **Separate IPEventsOut channel** mirroring the AuthEventsOut split
  introduced in 6b, rather than multiplexing `EventIPRequest` on
  the lifecycle `EventsOut`. An operator plugin subscribes to the one
  concern it implements (l2tp-pool reads IP events, l2tp-auth reads
  auth events); nothing is forced to type-switch on messages it does
  not handle.
- **Buffered(2) `ipRespCh`, one slot per family**. The NCP coordinator
  sends EventIPRequest serially (IPCP then IPv6CP) and waits for each
  response, so two slots cover the normal case without ever blocking
  `IPResponse`. A duplicate response returns `ErrIPResponsePending`
  rather than blocking or silently overwriting.
- **`DisableIPCP` / `DisableIPv6CP` inverted booleans**. Zero value =
  enabled, matching the YANG defaults (`enable-ipcp = true`). Spec
  reads the three env vars (`ze.l2tp.ncp.enable-ipcp`,
  `ze.l2tp.ncp.enable-ipv6cp`, `ze.l2tp.ncp.ip-timeout`) in
  `reactor.go` at StartSession build time; full YANG leaf wiring is
  spec-l2tp-7-subsystem's job.
- **Parallel scripted peer for tests** over sequential per-family
  helpers. A first attempt with `completeIPCP` followed by
  `completeIPv6CP` deadlocked because the driver sends both initial
  CRs back-to-back on a single goroutine and `net.Pipe` is
  synchronous. The `runParallelNCPPeer` helper reads frames and
  dispatches by proto in one loop, which also mirrors how a real peer
  would behave.

## Consequences

- Every new L2TP session now negotiates IPv4 AND IPv6 on the PPP layer
  by default; operators who want IPv4-only can set
  `ze.l2tp.ncp.enable-ipv6cp=false` (and vice versa).
- `EventSessionIPAssigned` lands on the lifecycle channel on each NCP
  Opened; the L2TP subsystem in Phase 7 will consume it to publish
  subscriber routes into the redistribute path.
- `iface.Backend.AddAddressP2P`, `AddRoute`, `RemoveAddress`,
  `RemoveRoute` now have a live caller from PPP. The netlink backend
  (`internal/plugins/ifacenetlink/manage_linux.go`) uses rtnetlink's
  `Addr.Peer` field; non-Linux backends return `not supported`.
- On session teardown (`StopSession` or natural exit), `teardownNCPResources`
  calls `RemoveRoute` and `RemoveAddress` best-effort. Errors are logged
  at debug level; they do not block teardown.
- The PPP package's main `handleFrame` now dispatches to three
  protocol families (LCP, IPCP, IPv6CP). This unlocks AC-20
  renegotiation paths naturally, even without a direct test.

## Gotchas

- `sendNCPConfigureRequest` uses `nextNCPIdentifier` with a single per-
  family counter. The counter wraps at 256; at realistic retransmit
  cadences this never causes a collision, but aggressive reneg loops
  could.
- `onNCPOpened` calls `SetAdminUp` for IPv4 even though `afterLCPOpen`
  already called it. The second call is idempotent on Linux netlink
  (`IFF_UP` is already set). Do not rely on this elsewhere.
- IPCP `Configure-Reject` of Primary-DNS / Secondary-DNS is handled by
  clearing the corresponding `netip.Addr` in per-session state and
  omitting the option from the next CONFREQ. IPCP Reject of IP-Address
  is fatal (AC-16). IPv6CP Reject of Interface-Identifier is fatal
  (the only option).
- Existing 6a/6b tests that asserted `EventSessionUp` timing had to be
  updated with `DisableIPCP: true, DisableIPv6CP: true`; otherwise
  the NCP phase parks on EventIPRequest that no fixture responds to
  and they time out. If you write a new test at the Driver level and
  don't care about NCPs, set both flags.
- `make ze-verify` requires `flock` from `util-linux`. macOS dev
  hosts need `brew install util-linux` + a `flock` wrapper, or use the
  direct `go test -race ./...` invocation. Pre-existing
  `TestVPPManagerRunOnce_External*` failures on Darwin are unrelated
  (sun_path length: macOS 104, Linux 108); logged in
  `plan/known-failures.md`.

## Files

- `internal/component/ppp/ncp.go` -- NCP coordinator, per-family FSM driver, backend programming
- `internal/component/ppp/ipcp.go` -- IPCP option codec (handoff-written, lint-unused symbols pruned)
- `internal/component/ppp/ipv6cp.go` -- IPv6CP option codec (handoff-written, lint-unused symbols pruned)
- `internal/component/ppp/ip_events.go` -- IPEvent sealed sum, AddressFamily, ipResponseMsg
- `internal/component/ppp/events.go` -- EventSessionIPAssigned added
- `internal/component/ppp/session.go` -- NCP state fields, ipRespCh, ipEventsOut, Disable*CP, IPTimeout
- `internal/component/ppp/session_run.go` -- runNCPPhase invoked in afterLCPOpen; handleFrame dispatches by proto; teardownNCPResources on exit
- `internal/component/ppp/manager.go` -- IPEventsOut accessor, IPResponse method, ErrIPResponsePending
- `internal/component/ppp/start_session.go` -- DisableIPCP, DisableIPv6CP, IPTimeout fields
- `internal/component/ppp/ncp_test.go` -- 13 integration tests (FSM + wiring + net.Pipe)
- `internal/component/ppp/ncp_helpers_test.go` -- test fixtures, scripted parallel peer
- `internal/component/ppp/ipcp_test.go` -- 6 codec + wiring tests
- `internal/component/ppp/ipv6cp_test.go` -- 6 codec + wiring tests
- `internal/component/l2tp/config.go` -- 3 env var registrations (ze.l2tp.ncp.*)
- `internal/component/l2tp/reactor.go` -- env vars plumbed through StartSession
- `rfc/short/rfc1877.md` -- new RFC summary
- `internal/component/ppp/ppp_fsm.go` -- renamed from lcp_fsm.go in the checkpoint commit; doc updated
