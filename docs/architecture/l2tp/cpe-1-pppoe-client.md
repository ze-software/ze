# PPPoE client interface kind

The client half of PPPoE, for the customer-premises case: dial an access
concentrator over a physical Ethernet interface, negotiate LCP, authentication
and IPCP, and present the resulting PPP session as a routable interface with
server-assigned addresses.

<!-- source: internal/component/iface/pppoe_client.go -- PPPoEClient, PPPoEClientConfig, PPPoEDialer, reconcilePPPoEClients -->
<!-- source: internal/component/l2tp/pppoeclient/dialer.go -- Dialer.Dial, waitForPADO, waitForPADS -->
<!-- source: internal/component/l2tp/pppoeclient/session.go -- negotiateSession, negotiateLCP, negotiateIPCP, keepaliveLoop -->
<!-- source: internal/component/l2tp/pppoeclient/auth.go -- client-mode authentication helpers -->

## RFC obligations carried by this code

RFC 2516 Section 5.3 requires the PADR to echo the Relay-Session-Id tag when one
was present in the PADO. It matters only where a relay agent is in the path, and
it is a MUST regardless.

## Decisions

**A separate package, because of an import cycle.** The pppoe package imports
the iface package for the backend lookup, so iface cannot import pppoe. The
`PPPoEDialer` interface is defined in iface and implemented in the pppoeclient
package, which imports both pppoe for the wire format and ppp for the state
machine and the kernel setup. Registration is an `init()` plus a blank import in
the hub.

**One negotiator answers a peer, whichever role ze is in.** `negotiateLCP`
calls `ppp.NegotiatePeerOptions`, the same function the LNS side calls, rather
than acknowledging every Configure-Request it can parse. That is a conformance
obligation and not a tidiness one: RFC 1661 Section 5.3 requires a
Configure-Nak when "every instance of the received Configuration Options is
recognizable, but some values are not acceptable", and Section 6.4 says a
Magic-Number of zero "MUST always be Nak'd, if it is not Rejected outright".
Two negotiators meant ze Nak'd that peer as an LNS and Acked it as a client.
Section 5.4 decides which answer wins when both apply: a reply is a
Configure-Reject or a Configure-Nak, never both.

The client retains its last transmitted Configure-Request and validates
Configure-Ack, Configure-Nak and Configure-Reject against it through
`ppp.ValidateLCPReply`. A stale Identifier or changed Ack/Reject options cannot
advance LCP. A valid reply changes the next request's Identifier, while an
unanswered timeout retransmits the same packet. The client adopts MRU Naks within
the receive bounds and redraws a nonzero Magic-Number after a Magic Nak. Rejected
options remain absent even if a later Nak suggests them again. The final
negotiated Magic-Number is passed to the authentication and keepalive phases.
Duplicate Acks and matching Naks or Rejects received in Ack-Rcvd trigger a fresh
request. A replacement peer request that draws a refusal cancels the peer's
earlier acceptance, so both directions must agree before LCP opens. Echo requests
receive no reply during negotiation.

IPCP retains and correlates its request in the same way. Its Nak values use the
IPCP option parser rather than LCP's option widths. A timeout retransmits the
outstanding request unchanged; a valid Nak changes its Identifier and proposed
address. Unsupported peer options receive a verbatim Configure-Reject, and a
replacement proposal must be accepted before the client publishes its addresses.
Failed or short request and Ack writes abort negotiation. The client requires
usable local and peer IPv4 addresses before connecting the PPP unit.
<!-- source: internal/component/l2tp/pppoeclient/session.go -- negotiateIPCP, sendLCPAck, writeClientPacket -->

**Client-mode PPP drives the FSM directly, it does not extend the PPP driver.**
The existing driver is server oriented: it sends the authentication challenge,
assigns addresses from a pool, and uses external authentication and address
event channels. Client mode reverses all of that. Adding client branches to
every server-side handler would put the L2TP and BNG path at risk, so the client
calls the ppp package's exported pure functions instead.

**One reader goroutine for the whole session lifetime.** `Dial` creates a reader
and passes its frame channel through negotiation to keepalive. Cleanup cancels
blocked data and error delivery even when the four-frame queue is full, closes
the transport to release a blocked read, and joins the reader by draining its
channel until it closes.
The reader and negotiation buffers allow a 1500-octet Information field
plus its two-octet Protocol field. The kernel receive MRU remains 1500;
the outgoing IP MTU is bounded by both the peer's MRU and the configured
PPPoE MTU, including when the peer omits its MRU option.
<!-- source: internal/component/l2tp/pppoeclient/dialer.go -- Dial -->
<!-- source: internal/component/l2tp/pppoeclient/session.go -- startReader, negotiateSession -->

**Session loss stops transport, not unit ownership.** A PADT or keepalive failure
stops PPP and closes `Done` immediately. The unit descriptor remains open until
the caller invokes `Cleanup`, because interface setup may still be using the
returned `UnitNum`. Releasing that descriptor from the asynchronous watcher
would let another subscriber reuse `pppN` before the old caller finishes its MTU,
address, or route changes. Cleanup joins the readers and then releases the unit.
<!-- source: internal/component/l2tp/pppoeclient/dialer.go -- Dial, sessionLink.Close -->
<!-- source: internal/component/iface/pppoe_client.go -- PPPoEClient.runSession -->

**PAP starts with the client's Authenticate-Request.** The client retries on
the same PPP session every three seconds, for at most five requests, and changes
the Identifier on each attempt. Only a well-formed Ack or Nak matching the latest
request completes authentication; stale replies and malformed Message lengths
are discarded. A failed or short write aborts immediately, including on a retry.
<!-- source: internal/component/l2tp/pppoeclient/session.go -- runClientAuth -->

The client accepts PAP or CHAP-MD5 during LCP and Naks another authentication
method with a CHAP-MD5 proposal. For CHAP, a malformed or empty Challenge
produces no Response. Only a result whose Identifier matches a successfully
written Response completes authentication; an unsolicited Success or an old
result is discarded. The result's Message remains advisory.
<!-- source: internal/component/l2tp/pppoeclient/session.go -- negotiateLCP, runClientAuth, buildCHAPResponse -->

**Reconciliation follows the DHCP shape.** Desired against active map diffing, a
config-change check that restarts affected clients, and a shutdown loop that
stops all of them.

<!-- source: internal/component/iface/pppoe_client.go -- pppoeClientConfigChanged, ReconnectDelay -->

**The discovery wait blocks rather than polls.** `Dial` sets `SO_RCVTIMEO` to
~100ms on the discovery socket before `waitForPADO` or `waitForPADS` ever runs,
so their `select`'s `default` arm blocks on that timeout when no frame arrives
rather than returning at once. That is what paces both loops: neither adds a
wait of its own, and neither calls `runtime.Gosched()` to yield between
attempts, because a call that already blocks has nothing to yield from.
`readDiscoveryFrame` (`dialer.go`) is a package variable over
`pppoe.ReadDiscoveryFrame` precisely so a test can swap in a fake without a
real AF_PACKET socket and prove the loop returns promptly on stop and does not
retry far more often than the blocking read allows.

Discovery stays active after PADS. `watchPADT` owns that socket's reads
through negotiation and keepalive, and accepts termination only for the
established interface, peer MAC, local MAC and session ID. A matching PADT
closes the PPP channel and kernel transport before publishing session Done.
`sessionLink` serializes writes against closure, so a queued Echo-Reply or
Terminate-Ack cannot use a released channel descriptor. Cleanup waits for
the discovery reader before closing its descriptor, and outbound PADT is
sent only after PPP has stopped.
<!-- source: internal/component/l2tp/pppoeclient/dialer.go -- watchPADT, sessionLink, sendPADT -->

RFC 2516 Section 7 prohibits ACCM, ACFC and FCS Alternatives. The client
sets `LCPNegPolicy.PPPoE`, so the shared negotiator rejects those options
even when their lengths and values are well formed. PFC is only
NOT RECOMMENDED and retains its existing negotiation behaviour.
<!-- source: internal/component/l2tp/ppp/lcp_options.go -- LCPNegPolicy, negotiatePeerOption -->

## Traps this code exists to avoid

**An empty `default:` in a select is refused by a repository hook.** The
non-blocking read shape has to be restructured into a reader goroutine plus a
channel.

**`strconv.FormatInt` is refused in production code.** MAC formatting uses
`net.HardwareAddr.String()`.

**A YANG leaf gated on Linux is pruned on macOS.** The config walker removes the
whole PPPoE client list on a non-Linux host, so `ze config validate` rejects it
as an unknown path. The functional parse test skips that operating system.

## Review findings worth remembering

Every one of these was found by review before the code shipped. They are the
failure shapes a new interface kind repeats:

| Finding | Root cause |
|---------|-----------|
| No shutdown cleanup for the clients | copied the DHCP reconcile and left out the shutdown loop |
| Channel and unit descriptors leaked in the cleanup closure | the PPPoE server closes them through the PPP driver, and the client has no driver |
| Two reader goroutines on the same channel descriptor | negotiation started one and the keepalive loop started another |
| Echo failure fired at four losses, not three | `>` against `>=` with a post-increment |
| A reload did not detect a changed parameter | the DHCP reconcile detects it and this one did not |
| The source interface was not validated | other interface kinds validate their key name |
| The discovery socket read blocked | it needs a receive timeout to stay responsive to the stop signal |
