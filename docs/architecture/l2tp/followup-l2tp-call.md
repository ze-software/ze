# The L2TP initiator half: dialing tunnels and placing calls

Ze's L2TPv2 subsystem could only answer, in the network-server role. The
initiator half was a set of stubs: no request encoders, three dead receive
stubs, states that were never entered, and no way to dial a remote. Ze now dials
a tunnel, places a network-server-side outgoing call through an RPC, and relays
a PPPoE subscriber into an access-concentrator incoming call.

<!-- source: internal/component/l2tp/outgoing_call.go -- PlaceOutgoingCall, OutgoingCallResult -->
<!-- source: internal/component/l2tp/cmd/outgoing_call.go -- handleOutgoingCall -->
<!-- source: internal/component/l2tp/relay_sink.go -- relaySink.Relay, relayTargetLocked -->
<!-- source: internal/core/callsink/callsink.go -- Sink, Register, Unregister, Lookup -->
<!-- source: internal/component/l2tp/bridge_linux.go -- bridgeChannels, unbridgeChannel, maybeBridgePPPoE -->

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
sends that PADS. A relayed subscriber then gets its PADS and its unused PPP unit
is closed, whether or not the box has a PPP driver, so a LAC with no interface
backend still relays. Local termination and the realm relay both run LCP on
the PPP driver. With no driver, a subscriber no Service-Name binding relays
gets a PADS with AC-System-Error and SESSION_ID 0 (RFC 2516 Section 5.4), and
a WARN names the missing interface backend and the
`set interface backend netlink` that fixes it.

<!-- source: internal/component/l2tp/pppoe/server.go -- handlePADR, relayToL2TP, sendPADS -->

**A realm relay crosses the same sink, after local LCP.** `Sink.RelayRealm`
takes a request the PPPoE auth drain makes once the subscriber's
authentication arrived, before any credential check. The sink places the
incoming call with the proxy AVPs; the kernel worker asks the PPPoE side to
quiesce the subscriber's channel reader, bridges, then reports the result. The
returned `RealmCall.Cancel` is the PPPoE side's one report that the subscriber
left: before the bridge it abandons the call, after it the sink tears the L2TP
session down with CDN. A call to a remote whose tunnel is established or still
connecting rides that tunnel, so one remote's subscribers share one tunnel.

<!-- source: internal/component/l2tp/relay_sink.go -- relaySink.RelayRealm, realmCall.Cancel, realmCall.markBridged -->
<!-- source: internal/component/l2tp/pppoe/realm_relay.go -- realmRelay.decide, realmHandoff.run -->
<!-- source: internal/component/l2tp/reactor_dial.go -- handleDial, connectingTunnelToLocked, placePendingCallLocked -->

**A call is an operational event, not config.** The operator surface is the
remote dial-target list plus a `request l2tp outgoing-call` RPC. A config-driven
auto-dial was rejected because it turns an event into state.

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
