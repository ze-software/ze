# The L2TP initiator half: dialing tunnels and placing calls

Ze's L2TPv2 subsystem could only answer, in the network-server role. The
initiator half was a set of stubs: no request encoders, three dead receive
stubs, states that were never entered, and no way to dial a remote. Ze now dials
a tunnel, places a network-server-side outgoing call through an RPC, and relays
a PPPoE subscriber into an access-concentrator incoming call.

<!-- source: internal/component/l2tp/outgoing_call.go -- PlaceOutgoingCall, OutgoingCallResult, reactorForRemoteLocked -->
<!-- source: internal/component/l2tp/cmd/outgoing_call.go -- handleOutgoingCall -->
<!-- source: internal/component/l2tp/relay_sink.go -- relaySink.Relay, relaySink.RelayRealm, relayTargetLocked -->
<!-- source: internal/core/callsink/callsink.go -- Sink, Register, Unregister, Lookup -->
<!-- source: internal/component/l2tp/bridge_linux.go -- maybeBridgePPPoE, maybeBridgePPPoEWith -->

## RFC obligations carried by this code

RFC 2661 defines L2TPv2, including the control-connection establishment
exchange, the incoming and outgoing call exchanges, and the reliable control
channel with its Ns and Nr sequence numbers. The reference summary is
`rfc/short/rfc2661.md`.

## Decisions

**The relay boundary is a registered sink in a leaf package.** PPPoE hands a
call to `callsink`, and L2TP registers the implementation. Neither component
imports the other.

**A Service-Name relay needs no PPP driver.** `handlePADR` sets up the session
socket and the PPP channel, builds the PADS, and asks `Sink.Relay` before it
sends that PADS. A relayed subscriber then gets its PADS. The accepted call
carries an optional preparation owner to the kernel worker, which releases
the unused local PPP channel and unit immediately before attaching its own
channel. Until that point, PPPoE retains the local lease and owns cleanup if
the subscriber leaves. Failed preparation refuses kernel setup before any
attachment. A caller with no unused lease supplies no preparation; its ordinary
PPP setup still runs. This handler bypasses the local PPP driver. Normal
subsystem startup requires a registered session backend and prepared access
interfaces before discovery starts. Local termination and the realm relay both run LCP on
the PPP driver. With no driver, a subscriber no Service-Name binding relays
gets a PADS with AC-System-Error and SESSION_ID 0 (RFC 2516 Section 5.4), and
a WARN names the missing interface backend and the
`set interface backend kernel` that fixes it.

<!-- source: internal/component/l2tp/pppoe/server.go -- handlePADR, relayToL2TP, sendPADS -->
<!-- source: internal/component/l2tp/relay_preparation.go -- prepareAndSetupPPP -->
<!-- source: internal/component/l2tp/kernel_linux.go -- pppSetupReal -->

**A native bridge owns both fresh original channels until cleanup settles.**
Both relay requests carry the original transport owner through `callsink`.
Native relay requires its original-file retention capability. The admitted
session reserves one exact lifetime token, then lends the SAME PPPoX file for
bounded local channel capture. No backend or network wait runs inside that
borrow. Providers without the capability refuse native relay; VPP does not
claim a Linux channel bridge through a fabricated descriptor.

Service-Name preparation releases the unused local PPP lease. Realm preparation
joins its reader, releases its original local PPP owner through the existing
native seal path, and requires the exact attempt's finished, released, error-free
receipt. Only then does the worker attach fresh channels and pass both to
`NewOriginalChannelBridge`. The constructor consumes both new owners on every
outcome. Channels already consumed by `NewSession` are never replayed.

The worker retains the exact bridge through publication, cancellation, failure,
retry and stop. `Close` first unbridges the retained native pair; uncertain
unbridge retains BOTH files and the transport token for retry. After successful
unbridge it closes both original claims. A terminal file-close error remains
recorded while independent transport cleanup still runs. Transport departure
refuses without physical close while its exact token is retained. Releasing
the final exact token fulfills any already-requested physical close, even when
the original departure caller never retries. Its result is retained by both
the admitted transport and the worker cleanup receipt. Numeric labels appear only in diagnostics and
private fault models, never as bridge or cleanup authority.

<!-- source: internal/core/callsink/callsink.go -- RelayChannel, Request, RealmRequest -->
<!-- source: internal/component/l2tp/pppoe/dataplane/session.go -- openedSession.WithOriginalFile, openedSession.CloseTransport -->
<!-- source: internal/component/l2tp/pppoe/dataplane/relay_file.go -- openedSession.RetainOriginalFile, relayFileLease.Close -->
<!-- source: internal/component/l2tp/ppp/kernel/original_bridge_linux.go -- NewOriginalChannelBridge, OriginalChannelBridge.Close, OriginalChannelBridge.ClosePending -->
<!-- source: internal/component/l2tp/bridge_linux.go -- maybeBridgePPPoE -->
<!-- source: internal/component/l2tp/kernel_ppp_owners_linux.go -- bridgeOriginalKernelPPP, closeOriginalKernelPPP -->

**A realm relay crosses the same sink, after local LCP.** `Sink.RelayRealm`
takes a request the PPPoE auth drain makes once the subscriber's
authentication arrived, before any credential check. The sink places the
incoming call with the proxy AVPs; the kernel worker asks the PPPoE side to
quiesce and release the local PPP owner, captures fresh original peers, bridges,
then reports the result. If bridging fails after local release, access teardown
ends the subscriber because the old PPP owner can no longer answer authentication. The
returned `RelayCall.Cancel` is the PPPoE side's one report that the subscriber
left: before the bridge it abandons the call, after it the sink tears the L2TP
session down with CDN. A call to a remote whose tunnel is established or still
connecting rides that tunnel, so one remote's subscribers share one tunnel.

<!-- source: internal/component/l2tp/relay_sink.go -- relaySink.RelayRealm, relayCall.Cancel, relayCall.markBridged -->
<!-- source: internal/component/l2tp/pppoe/realm_relay.go -- realmRelay.decide, realmHandoff.run -->
<!-- source: internal/component/l2tp/reactor_dial.go -- handleDial, connectingTunnelToLocked, placePendingCallLocked -->

**Both relay kinds own the call until it ends.** Each accepted request returns
one `RelayCall` token. Subscriber departure, access removal and subsystem stop
cancel that token outside access-table locks. A remote CDN, tunnel failure or
kernel setup refusal reports `Ended` once for either kind. Service-Name end
notifications wait in the admitted session table until PADS publication, then
wake the existing PPPoE event consumer; no extra per-call worker is created.
The table drains before its access generation is released at stop.
<!-- source: internal/component/l2tp/pppoe/session.go -- SessionTable.markSessionFor -->
<!-- source: internal/component/l2tp/pppoe/service_relay.go -- serviceBridgePreparation.Ended, SessionTable.takeServiceRelayEnds, Subsystem.drainServiceRelayEnds -->

Cancellation and every queued worker refusal or teardown carry the exact call
owner. The reactor and worker compare that owner while holding their own map
locks, so an old callback cannot delete a subscriber that reused the numeric
tunnel/session pair. Only realm calls carry proxy metadata or run Quiesce and
Bridged callbacks; a Service-Name call never publishes a fabricated realm
snapshot.

<!-- source: internal/component/l2tp/relay_sink.go -- relayCall.Cancel, relayCall.end -->
<!-- source: internal/component/l2tp/teardown.go -- teardownRelaySession -->
<!-- source: internal/component/l2tp/kernel_linux.go -- kernelWorker.teardownSession, kernelWorker.reportError -->

**A call is an operational event, not config.** The operator surface is the
remote dial-target list plus a `request l2tp outgoing-call` RPC. A config-driven
auto-dial was rejected because it turns an event into state.

**Dials select a listener that can reach the remote.** With multiple
`environment l2tp server` blocks (for example a loopback listener alongside a
routable listener, or IPv4 and IPv6 listeners), `PlaceOutgoingCall`,
`relaySink.Relay`, and `relaySink.RelayRealm` select the reactor whose bound
listener matches the remote's IP address family (`udp4` vs `IPV6_V6ONLY`
`udp6`) and loopback compatibility via `reactorForRemoteLocked`. A loopback
remote prefers a same-family loopback listener and falls back to a same-family
non-loopback listener when none is configured; an off-host remote requires a
same-family non-loopback listener. When no listener can reach the remote, the
dial refuses with an operator error naming the remote and listeners. Config load
(`validateDialSources`) and reload (`reloadDialSources`) apply the same
reachability predicate so configuration validation matches runtime selection.

<!-- source: internal/component/l2tp/outgoing_call.go -- reactorForRemoteLocked, unreachableRemoteErrorLocked -->
<!-- source: internal/component/l2tp/reactor_dial.go -- sameIPFamily, listenerCanReach, ErrNoReachingListener -->
<!-- source: internal/component/l2tp/config.go -- validateDialSources, checkDialSources -->
<!-- source: internal/component/l2tp/subsystem_reload.go -- reloadDialSources -->

**The functional test drives the RPC over the token-guarded REST API.** The
no-authentication REST surface is read-only, so the test config sets an API
token and the test peer sends a bearer header. That is the only way a functional
test can invoke a mutating `request` verb. SSH would need keys, which the
functional tests do not carry.

**The test peer both triggers the dial and answers on the wire.** A background
thread posts the blocking RPC while the main thread answers on a fixed port.
That is the answer to "Ze is the initiator, so who makes it dial".

**Real-peer interop dials xl2tpd and establishes the control connection.** The
alternative, Ze as an access concentrator with PPP up, needs a PPPoE subscriber
harness and the kernel bridge under `CAP_NET_ADMIN`. The scenario ships its own
runner and runs unprivileged.

## Consequences worth knowing

- Ze interoperates as an L2TP access concentrator and initiator against a
  third-party network server at the tunnel level, proven against xl2tpd 1.3.18.
- The token-guarded REST execute endpoint is the reusable way for a functional
  test to drive any mutating in-process RPC against a running daemon.
- A scenario directory that ships its own runner is supported. The interop
  runner delegates to it and skips the Ze-as-network-server preflight and image
  build. Use it for any inverted-topology scenario.
- The access-concentrator dataplane, which bridges the PPPoE channel to the
  kernel L2TP channel, stays gated on `CAP_NET_ADMIN` and runs under QEMU.

<!-- source: test/interop-l2tp/scenarios -- interop scenarios, including the Ze-dials-xl2tpd case -->

## Traps this code exists to avoid

**xl2tpd cannot answer an outgoing-call request.** Its control handler has no
case for that message type, so a received type 7 hits the default unimplemented
branch and it closes the tunnel. An outgoing call against xl2tpd therefore fails
BY DESIGN once the tunnel is up. The interop proof is the established control
connection, not the call. Only accel-ppp in access-concentrator mode answers,
through an undocumented source-only option, so the wire proof for the
network-server outgoing call stays at the functional tier.

**The reliable sequence numbers must be mirrored for the initiator case.** The
peer sends its reply with `ns=0,nr=1`; after Ze's connected message at `ns=1`
and its outgoing-call request at `ns=2` the peer replies `ns=1,nr=3` and then
`ns=2,nr=3`. Advancing the peer's receive number only on the next expected send
number keeps a test peer simple and correct.

**Skipping the kernel probe establishes the session at the control level only.**
With the probe skipped there is no kernel worker, so the kernel setup event
collection returns early. That is why the outgoing-call functional test
completes without `CAP_NET_ADMIN`.

**A dotted environment variable name needs the `env` builtin.** Bash rejects a
dot in a bare assignment prefix and exits 127.

## Operator documentation

`docs/guide/l2tp.md` carries the remote dial-target list and the outgoing-call
RPC.
