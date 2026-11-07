# PPPoE client interface kind

The client half of PPPoE, for the customer-premises case: dial an access
concentrator over a physical Ethernet interface, negotiate LCP, authentication
and IPCP, and present the resulting PPP session as a routable interface with
server-assigned addresses.

<!-- source: internal/component/iface/pppoe_client.go -- PPPoEClient, PPPoEClientConfig, PPPoEDialer, reconcilePPPoEClients -->
<!-- source: internal/component/l2tp/pppoeclient/dialer.go -- Dialer.Dial, waitForPADO, waitForPADS -->
<!-- source: internal/component/l2tp/pppoeclient/session.go -- negotiateSession, negotiateLCP, negotiateIPCP -->
<!-- source: internal/component/l2tp/pppoeclient/network_phase.go -- keepaliveLoop -->
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

The request carries IP-Address, Primary-DNS-Address (129) and
Secondary-DNS-Address (131), each 0.0.0.0 and in that order (RFC 1877
Sections 1.1 and 1.3), written by `ppp.WriteIPCPOptions` into the outstanding
request buffer. A Nak's DNS value is adopted only when it is a unicast address,
so a Nak of 0.0.0.0 records nothing and the request keeps asking until the IPCP
timeout. A Configure-Reject of a DNS option removes it from every later request
of that negotiation (RFC 1661 Section 5.4), so an access concentrator with no
DNS to offer (accel-ppp without `[dns]` Rejects both) still brings the session
up; a Reject of IP-Address still ends it. On Ack the request's DNS values are
the learned servers, carried in `iface.PPPoESession` and in every change record
after a renegotiation.

`runSession` hands the learned servers, primary first, to the process
resolv.conf writer (`system.Resolvers`) as a lease keyed by the client's
configured name (`iface.PPPoEResolverSource`), replaces the lease when a
renegotiation changes only the servers, and withdraws it when the session
ends. Leases order by the rank their producer registered
(`system.RegisterResolverOrigin`): DHCPv4 100, DHCPv6 200, PPP 300, so PPP
servers list after DHCP ones, and a static `system name-server` still replaces
them all. `show interface pppoe-client` answers each client's state and its
`dns-servers` from the iface plugin's process (`iface.PPPoEClientCommand`).
Whether the LAN `dns-resolver` should also forward to learned servers is an
open owner decision: today it forwards only to its configured servers.
<!-- source: internal/component/l2tp/pppoeclient/session.go -- ipcpInitialRequest, adoptNakDNS, learnedDNS, sendIPCPRequest -->
<!-- source: internal/component/iface/pppoe_client.go -- runSession, leaseDNS, pppoeDNSServers, PPPoEResolverSource -->
<!-- source: internal/component/iface/pppoe_client_command.go -- PPPoEClientCommand, pppoeClientAnswer -->

**Client-mode PPP drives the FSM directly, it does not extend the PPP driver.**
The existing driver is server oriented: it sends the authentication challenge,
assigns addresses from a pool, and uses external authentication and address
event channels. Client mode reverses all of that. Adding client branches to
every server-side handler would put the L2TP and BNG path at risk, so the client
calls the ppp package's exported pure functions instead.

**One receive worker for the whole session lifetime.** `Dial` creates the
channel wrapper, sets both raw descriptors nonblocking, and starts `frameReader`.
Each request reads the channel first, then the connected unit if the channel
has no packet. The consumer dispatches that packet before requesting another.
Once IPCP is Opened, negotiation connects the channel to the unit
(`PPPIOCCONNECT`) and enables unit reads under the lifecycle lock.
Linux then queues IPCP on the unit, not the channel (`ppp_input` and
`ppp_receive_nonmp_frame`, `drivers/net/ppp/ppp_generic.c`).
<!-- source: internal/component/l2tp/pppoeclient/receive.go -- frameReader, next, run -->
<!-- source: internal/component/l2tp/pppoeclient/receive_linux.go -- readPacket, prepareClientDescriptors -->
<!-- source: internal/component/l2tp/pppoeclient/session.go -- negotiateSession -->

In the network phase, the consumer answers
an IPCP packet with an unknown Code with a Code-Reject whose Identifier goes on
from the last one IPCP negotiation sent, answers an IPCP Terminate-Request with
a Terminate-Ack and ends the session, and ends the session on any other IPCP
packet that takes IPCP out of Opened independently of LCP. Only an LCP
Configure-Request starts in-session renegotiation. Cleanup stops transport,
joins the phase worker, joins the receive worker, and closes the unit last.
Cancellation releases request and delivery waits. Raw reads and each bounded
poll hold the lifecycle lock, so Close cannot recycle a descriptor during a syscall.
Initial negotiation failures have no phase worker to join.
<!-- source: internal/component/l2tp/pppoeclient/dialer.go -- Dial, cleanupConnected -->
<!-- source: internal/component/l2tp/pppoeclient/network_phase.go -- keepaliveLoop, answerOpenedIPCP -->

The lifecycle lock remains held during each 100ms poll. Independently started
writes, IPv4 mode changes and Close can wait for that poll to finish.
This is the code-derived poll contribution, not a measured operation-latency bound:
scheduling and lock contention can add delay. An idle outstanding read requests
about ten timeout wakeups per second. Packet-triggered replies precede the next
read request and do not incur this idle-poll wait.
<!-- source: internal/component/l2tp/pppoeclient/receive_linux.go -- readPacket -->
<!-- source: internal/component/l2tp/pppoeclient/receive.go -- next, received, run -->

Initial negotiation requires a non-nil `sessionLink` before its first LCP write.
An unsupported transport returns an error instead of omitting lifecycle locking
or unit-read activation. Nil interfaces and typed-nil links also return errors.
The same checked link protects connection and unit-read activation.
<!-- source: internal/component/l2tp/pppoeclient/session.go -- negotiateSession -->

An unexpected network-phase receive failure logs a warning with the configured
client name, the cause, and the automatic reconnect action before the supervisor
closes the transport. A closed session stop signal suppresses this warning.
EOF alone does not mean intentional shutdown: a zero-byte raw read also produces
EOF, so an active session reports it, including UnexpectedEOF.
<!-- source: internal/component/l2tp/pppoeclient/network_phase.go -- keepaliveLoop, warnReceiveFailure -->
<!-- source: internal/component/l2tp/pppoeclient/receive.go -- run -->
<!-- source: internal/component/l2tp/pppoeclient/dialer.go -- superviseNetworkPhase, Close -->

The reader and negotiation buffers allow a 1500-octet Information field
plus its two-octet Protocol field. The kernel receive MRU remains 1500;
the outgoing IP MTU is bounded by both the peer's MRU and the configured
PPPoE MTU, including when the peer omits its MRU option.
`Dial` takes the session MTU from `pppoeSessionMTU`: 1492 when none is
configured, and a refusal for anything outside 68..1492 before discovery
starts. The iface config refuses the same range, and the check is repeated
here because RFC 2516 Section 7 forbids negotiating an MRU above 1492, and
the session MTU is the MRU the client proposes.
<!-- source: internal/component/l2tp/pppoeclient/dialer.go -- Dial, pppoeSessionMTU -->
<!-- source: internal/component/l2tp/pppoeclient/receive.go -- run -->
<!-- source: internal/component/l2tp/pppoeclient/session.go -- negotiateSession -->

**The session device carries the configured name.** `Dial` passes the
`pppoe-client` list key to `ppp.DevPPPSetup`, which creates the unit with an
rtnetlink `RTM_NEWLINK` of kind `ppp` carrying `IFLA_PPP_DEV_FD` on the opened
`/dev/ppp` file. The kernel keeps the name the request carries
(`ppp_nl_newlink` in `drivers/net/ppp/ppp_generic.c`), and `PPPIOCGUNIT` reads
back the unit number that `PPPIOCCONNECT` needs. The client never calls
`PPPIOCNEWUNIT`, so it never creates a `pppN` device. Every session and every
reconnect gets `pppoe0` for `pppoe-client pppoe0`, whatever unit number the
kernel picks. `runSession` and `show interface` use that name. A static route,
a firewall match or a masquerade rule that names `pppoe0` therefore reaches the
session. The creation retries `EBUSY` and `EEXIST` a bounded number of times:
the kernel answers `EBUSY` when `ppp_mutex` is contended, and `EEXIST` while
the previous session's device is still being released. A name another program
holds fails with an error that names the device and the fix. The access
concentrator and the LNS still use `PPPIOCNEWUNIT`, because a subscriber
session has no configured name.
<!-- source: internal/component/l2tp/ppp/devppp_linux.go -- DevPPPSetup, createPPPLink, sendPPPLinkCreate -->
<!-- source: internal/component/l2tp/pppoeclient/dialer.go -- Dial -->

**Session loss stops transport, not unit ownership.** A PADT or keepalive failure
stops PPP and closes `Done` immediately. The unit descriptor remains open until
the caller invokes `Cleanup`, because interface setup may still be using the
device. If the asynchronous watcher released that descriptor, the device would
disappear while the old caller was still applying its MTU, address or route
changes. Cleanup joins the readers and then releases the unit, which removes
the device. The next session can then create the same name.
<!-- source: internal/component/l2tp/pppoeclient/dialer.go -- Dial, sessionLink.Close -->
<!-- source: internal/component/iface/pppoe_client.go -- PPPoEClient.runSession -->

Before any phase ends, `requestLCPReentry` validates the option boundaries.
Truncated options are silently discarded in authentication, IPCP and keepalive,
without resetting the current exchange or ending the session. Contained options
that require a Nak or Reject still enter renegotiation.
<!-- source: internal/component/l2tp/pppoeclient/session.go -- requestLCPReentry, runClientAuth, negotiateIPCP -->
<!-- source: internal/component/l2tp/pppoeclient/network_phase.go -- keepaliveLoop -->

An admitted LCP Configure-Request during authentication or initial IPCP negotiation
returns the pending request to `negotiatePhases`. Its iterative phase loop
restarts LCP with a fresh local Magic-Number. It sends its own Configure-Request
before judging the pending request through the existing option negotiator.
Authentication and IPCP start afresh with the new LCP result. LCP, PAP and IPCP
retain their Identifier sequences for the session lifetime, including retries and
restarts. An immediately preceding epoch's reply cannot complete the new exchange.
The IPCP Code-Reject sequence also belongs to the session. Negotiation and
network handling advance the same counter, even when negotiation is interrupted.
Publishing a negotiated result does not replace that counter.
One attempt permits ten LCP epochs, each with the existing phase deadlines.
Repeated reentry without completed IPCP exhausts that attempt and ends the session.
Completed, separated network-phase restarts each receive a new attempt budget.
<!-- source: internal/component/l2tp/pppoeclient/phases.go -- negotiatePhases -->
<!-- source: internal/component/l2tp/pppoeclient/session.go -- negotiateSession, negotiateIPCP -->
<!-- source: internal/component/l2tp/pppoeclient/connected_session.go -- newConnectedSession, restart -->
<!-- source: internal/component/l2tp/pppoeclient/network_phase.go -- keepaliveLoop -->

The phase functions consume the same ordered reader. LCP and authentication
discard premature IPCP before requesting another packet. Initial negotiation
reads only the channel because the unit is not connected yet.
<!-- source: internal/component/l2tp/pppoeclient/session.go -- negotiateLCP, runClientAuth, negotiateIPCP -->

There is no userspace read-ahead queue. One pending request prevents timer or
stop selection from authorizing a second read. Terminal delivery uses the same
cancellation-aware handoff. The descriptor API supplies no arrival timestamp,
so channel-first consumption does not claim a global packet arrival sequence.
<!-- source: internal/component/l2tp/pppoeclient/receive.go -- next, received, run -->
<!-- source: internal/component/l2tp/pppoeclient/receive_linux.go -- readPacket -->

The preceding independent-reader implementation discarded eager IPCP while
authentication Success waited in the channel queue. The deterministic test
reproduced this for CHAP, PAP and no-auth, and the real-kernel test missed the
first request's Ack. With the ordered reader, the deterministic cases, first-Ack
test and descriptor-order test passed with race instrumentation on 2026-11-03.
The kernel evidence is from `7.2.9-ze`. Target kernel 7.3 and final external
review remain open.
<!-- source: internal/component/l2tp/pppoeclient/eager_ipcp_test.go -- TestClientEagerIPCPReaders -->
<!-- source: internal/component/l2tp/pppoeclient/eager_ipcp_integration_linux_test.go -- TestDialAcknowledgesFirstEagerIPCP -->
<!-- source: internal/component/l2tp/pppoeclient/receive_order_integration_linux_test.go -- TestOrderedDescriptorDispatch -->

Network-phase reentry uses a constructed connected owner with the existing
reader, retained credentials and the session's LCP, PAP and IPCP Identifier sequences.
It sets IPv4 mode to DROP before sending any restart packet. The shared phase
owner reruns LCP, authentication and IPCP, then sets PASS after IPCP reaches Opened.
The same result constructor admits initial and restarted negotiation before connection
or PASS. It refuses an unusable IPv4 address or a negotiated MTU outside
68..configured MTU, also bounded by the peer MRU.
Before admission, the shared LCP option negotiator Naks a peer MRU less than 68
with 68. A corrected request can complete on the same session. The client sets
`LCPNegPolicy.MinMRU` from its existing IPv4 minimum. Other callers retain the
shared negotiator's 64-octet local policy floor, which RFC 1661 does not specify.
<!-- source: internal/component/l2tp/pppoeclient/session.go -- clientLCPPolicy -->
<!-- source: internal/component/l2tp/ppp/lcp_options.go -- negotiatePeerOption -->
Keepalive replaces its complete negotiated result and resets echo timing and failure
counts. The Echo Identifier sequence continues across restarts.
Failure ends the transport, while Cleanup retains sole ownership of unit release.
<!-- source: internal/component/l2tp/pppoeclient/network_phase.go -- keepaliveLoop -->
<!-- source: internal/component/l2tp/pppoeclient/connected_session.go -- newConnectedSession, restart -->

The connected worker does not mutate Dial's initial result. Dial snapshots the
returned addresses and MTU before starting that worker. Cleanup uses the stable
descriptor and reader references from initial negotiation. IPv4 mode changes
share the transport lifecycle lock and refuse operations after transport closure.
After every completed restart, the worker sends the complete interface result,
including when its values did not change. The one-slot channel replaces an unread
result with the newest result and never blocks on interface operations.
<!-- source: internal/component/l2tp/pppoeclient/dialer.go -- Dial, setIPv4Mode -->
<!-- source: internal/component/l2tp/pppoeclient/connected_session.go -- restart -->

The interface worker consumes each result on the existing device. It changes
only the MTU, address pair or default gateway that differs from its successful
application state. New addresses and routes precede removal of their old values.
MTU and address errors end the session through Cleanup and reconnect backoff.
A failed route install logs the destination, gateway and device, keeps PPP up,
and retries on the next complete result. Failed installs do not claim ownership.
A failed old-route cleanup retries without blocking installation of the current
desired gateway. Netlink replaces the fixed default-route prefix and metric slot,
so the worker retains one cleanup request rather than a backlog of replaced gateways.
An unchanged record makes no backend call when every field is already applied.
It can retry a previously failed route install or removal.
<!-- source: internal/component/iface/pppoe_client.go -- runSession, warnRouteApply, warnRouteRemoval -->

`ze_pppoe_client_renegotiations_total` counts network-phase LCP restarts by
configured client `name`, including attempts that fail. The registration hook
binds when telemetry becomes available, including after the client starts.
<!-- source: internal/component/l2tp/pppoeclient/metrics.go -- bindClientMetrics, countRenegotiation -->
<!-- source: internal/component/l2tp/pppoeclient/register.go -- init -->

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

Authentication does not end at the first Success. In the network phase the
access concentrator may challenge again at any time (RFC 1994 Section 4.1), so
when LCP negotiated CHAP, the client answers every Challenge with a fresh
Response built the same way. The network phase starts with IPCP negotiation,
so `negotiateIPCP` answers a Challenge too, and the same `networkPhaseCHAP`
state then passes to `keepaliveLoop`, which answers for the rest of the
session. A Success for that Response is logged, a result for another
Identifier is discarded, and a Failure for it ends the session, or ends IPCP
negotiation with an error: the authenticator SHOULD terminate the link after a
Failure (Section 4.2), and the client stops using it rather than wait.
<!-- source: internal/component/l2tp/pppoeclient/network_phase.go -- keepaliveLoop, networkPhaseCHAP.handle -->
<!-- source: internal/component/l2tp/pppoeclient/session.go -- negotiateLCP, runClientAuth, buildCHAPResponse, negotiateIPCP -->

**Reconciliation follows the DHCP shape.** Desired against active map diffing, a
config-change check that restarts affected clients, and a shutdown loop that
stops all of them. `pppoeClientConfigChanged` compares every
`PPPoEClientConfig` field except `Timing`, so a new field restarts the client
on change with no edit to the check. A change to `Timing` alone keeps the
running session: the reconcile hands it to `setTiming`, which stores it under
`c.mu`. `runSession` snapshots the config under the same lock before each
`Dial`, and the next redial wait reads the stored timing.

**Keepalive and reconnect timing come from the config.** The parse applies the
YANG defaults of the `keepalive` and `reconnect` containers (an interval of
10 s, 3 failures, and a redial wait of 1 to 60 s), then validates the result.
`Dial` validates the same `PPPoEClientTiming` again, and `newConnectedSession`
refuses a zero interval or a zero failure count, so no session can start with
a ticker of zero or a check that never fires. `keepaliveLoop` ends the session
on the Nth unanswered Echo-Request in a row, N being the failure count, and
any Echo-Reply resets the count. After a session ends or a dial fails, the
wait before the next discovery starts at `delay-min` and doubles on each
further failure up to `delay-max`.

<!-- source: internal/component/iface/pppoe_client.go -- pppoeClientConfigChanged, setTiming, run -->
<!-- source: internal/component/iface/pppoe_client_timing.go -- PPPoEClientTiming.Validate, reconnectDelay -->
<!-- source: internal/component/l2tp/pppoeclient/network_phase.go -- keepaliveLoop -->

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
NOT RECOMMENDED and retains its existing negotiation behavior.
<!-- source: internal/component/l2tp/ppp/lcp_options.go -- LCPNegPolicy, negotiatePeerOption -->

## Scripted kernel tests

`internal/test/pppoeac` supplies the AF_PACKET peer for client integration tests.
`OpenVeth` creates a pair inside the test's user namespace. `Accept` completes
discovery, and the synchronous `LCP`, `CHAP` and `IPCP` steps let a test pause
between phases or request a restart. `SendControl` and `ReadControl` give the
script control over Echo packets. `SendPacket` and `ReadPacket` share the session
encoder and reader for IPv4 probes. The caller owns cancellation and joins its
script before socket cleanup. The older unit-ownership and unit-descriptor tests
use the same transport with `ServeNoAuth`.
<!-- source: internal/test/pppoeac/script_linux.go -- OpenVeth, Accept, LCP, CHAP, IPCP, ReadControl -->
<!-- source: internal/test/pppoeac/peer_linux.go -- ServeNoAuth, SendControl -->

`ReadExpectedControl` and `ReadExpectedPacket` answer complete Echo requests in
the caller's sole read loop, without extending its context deadline. Both
Magic-Numbers come from observed LCP requests and exact successful Acks. A wrong
client Magic-Number returns an error before any reply. Omitted Magic requires
zero. New requests replace the observed epoch, including revised proposals after
Nak or Reject. Before both exchanges complete, Echo requests receive no reply.
Other packets, including malformed Echo, termination and reentry, reach the caller.
Raw reads remain available for explicit wire assertions.
<!-- source: internal/test/pppoeac/expect_linux.go -- ReadExpectedControl, ReadExpectedPacket -->
<!-- source: internal/test/pppoeac/echo_state_linux.go -- observe, replyMagic -->

The installed-state test loads operator configuration through `LoadConfig`.
The plugin coordinator carries its tree to the production server, which selects
the interface plugin from its registered config root and delivers its config.
The registered runner starts the PPPoE reconciler, dialer and netlink backend.
Both scenarios require exactly one parsed PPPoE entry and one active client,
read from the first startup delivery through an integration-only observer.
The test reads the kernel's
local address, peer, default gateway, route metric, route protocol and MTU.
It checks both authenticated epochs on the same PPP device. The route observer
recognizes the explicit zero prefix returned by the netlink decoder.
<!-- source: internal/component/iface/pppoe_config_integration_linux_test.go -- startPPPoEConfig -->
<!-- source: internal/component/iface/pppoe_startup_observation_integration_linux.go -- observePPPoEStartup -->
<!-- source: internal/component/iface/pppoe_kernel_integration_linux_test.go -- TestPPPoEConfigRenegotiatesKernelState, pppoeKernelState -->

The packet test sends UDP through the kernel in both directions before and after
renegotiation. It withholds the LCP, PAP and IPCP replies to measure discard in
each phase, then checks resumed traffic and the changed installed state.
Both tests exercise registered startup config delivery inside their isolated
test processes; they do not launch the full daemon CLI.
<!-- source: internal/component/iface/pppoe_packets_integration_linux_test.go -- TestDialRenegotiatesLCPInNetworkPhase, packetProbe -->

On 2026-11-03, both scenarios passed with race instrumentation on `7.2.9-ze`.
Omitting interface sections in `Server.deliverConfigRPC` made both fail with
`parsed 0 PPPoE clients, want one`. Duplicating the parsed entry before
reconciliation made both fail with `parsed 2 PPPoE clients, want one`.
Omitting startup reconciliation made both fail with
`reconciler started 0 clients, want one`. Both passed after all three temporary
changes were removed.
<!-- source: internal/component/plugin/server/startup.go -- deliverConfigRPC -->
<!-- source: internal/component/iface/register.go -- runEngine -->
<!-- source: internal/component/iface/pppoe_config_integration_linux_test.go -- startPPPoEConfig -->

The scoped command used `CGO_ENABLED=1 ./le job run label child12-production-entry
command go test -race -count=1 -timeout 90s -tags "$tags" -v
-run '^(TestPPPoEConfigRenegotiatesKernelState|TestDialRenegotiatesLCPInNetworkPhase)$'
./internal/component/iface`. Here, `tags` contains `ze_core integration` and
the feature tags declared in `feature-gates.txt`. Each mutation run used the
same selector and flags. This proves startup config delivery and the retained
cardinality checks, not an independent mutation of each packet assertion.
<!-- source: internal/component/iface/pppoe_kernel_integration_linux_test.go -- TestPPPoEConfigRenegotiatesKernelState -->
<!-- source: internal/component/iface/pppoe_packets_integration_linux_test.go -- TestDialRenegotiatesLCPInNetworkPhase -->

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
