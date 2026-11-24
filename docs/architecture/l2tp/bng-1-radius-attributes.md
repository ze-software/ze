# RADIUS subscriber-profile attributes

A broadband network gateway provisions subscribers from RADIUS. Ze's auth
handler once rejected any Access-Accept that carried a profile attribute, which
made it unusable in that role. Each attribute now reaches its consumer:
Framed-IP-Address, Framed-Pool, Framed-MTU, Session-Timeout, Idle-Timeout,
Filter-Id and Acct-Interim-Interval.

<!-- source: internal/component/l2tp/plugins/authradius/extract.go -- extractAuthMetadata, extractFramedRoutes, isValidSubscriberIP -->
<!-- source: internal/component/l2tp/plugins/authradius/extract_mtu.go -- extractFramedMTU -->
<!-- source: internal/component/l2tp/session_metadata.go -- AuthMetadata, StoreSessionMetadata, LoadSessionMetadata, ClearSessionMetadata -->
<!-- source: internal/component/l2tp/session_timeout.go -- startSessionTimeouts, runSessionTimeout, runIdleTimeout, cancelSessionTimeouts -->
<!-- source: internal/component/traffic/filterid_rate.go -- ParseFilterIDRate -->

## Decisions

**A `sync.Map` metadata store, not a wider auth result.** The pool, the reactor,
the shaper and the accounting path all need the same data. Extending the auth
respond callback signature would have changed a contract that four independent
consumers depend on.

L2TP profile readers use the original supplied subscriber ID. The actor caches
it before publication and carries it on lifecycle events. Pool allocation,
timeouts, shaping, accounting and CoS use that ID for their metadata lookup;
they do not rebuild it from the tunnel and session numbers. PPPoE keeps its
existing supplied identity.
<!-- source: internal/component/l2tp/session.go -- SubscriberID -->
<!-- source: internal/component/l2tp/session_metadata.go -- StoreSessionMetadata, LoadSessionMetadata, ClearSessionMetadata -->
<!-- source: internal/component/l2tp/session_timeout.go -- startSessionTimeouts -->
<!-- source: internal/component/l2tp/plugins/shaper/shaper.go -- onSessionUp -->
<!-- source: internal/component/l2tp/plugins/authradius/acct.go -- onSessionIPAssigned -->
<!-- source: internal/component/l2tp/plugins/pool/register.go -- handle, handlePrefix -->
<!-- source: internal/plugins/cos/handler.go -- onSessionUp -->

**Framed-IP-Address carries two special values alongside a static assignment.**
RFC 2865 Section 5.8 gives `0xFFFFFFFE` (`255.255.255.254`) the meaning "the NAS
should select an address for the user" and `0xFFFFFFFF` (`255.255.255.255`) the
meaning "the NAS Should allow the user to select an address (e.g. Negotiated)".
`extractAuthMetadata` leaves `AuthMetadata.FramedIP` unset for both so RFC 2866
Section 4.1 accounting never reports a sentinel as the subscriber's address, and
sets `AuthMetadata.FramedIPNegotiated` for `0xFFFFFFFF` alone. The pool handler
allocates from the pool for `0xFFFFFFFE` or an absent attribute, pins the
address for a unicast `FramedIP`, and for `FramedIPNegotiated` skips pool
allocation while supplying the selected pool's gateway, DNS, and
`AdmitPeerIPv4`/`ReleasePeerIPv4` callbacks with `PeerNegotiated: true`. IPCP
then accepts a peer-proposed unicast IPv4 address only when it is distinct from
the local gateway, falls outside every configured IPv4 pool (default and named),
and is not already held by any other live session (pool-allocated, static
RADIUS, or peer-negotiated). Admitted peer-negotiated addresses are recorded
exclusively in `sessionAddrs` (`fromPool: false`), carried safely across pool
reloads, and released on IPCP This-Layer-Down, LCP reset, or session teardown.
Unusable, in-pool, or already-held proposals draw a Configure-Reject because Ze
holds no pool address to Nak with; an omitted IP-Address option draws a
Configure-Nak carrying `0.0.0.0` per RFC 1332 Section 3.3. The negotiated
address is what programs `pppN` and reaches `EventSessionIPAssigned` and RADIUS
accounting.

<!-- source: internal/component/l2tp/plugins/authradius/extract.go -- extractAuthMetadata, framedIPUserSelects, framedIPNASSelects -->
<!-- source: internal/component/l2tp/session_metadata.go -- AuthMetadata -->
<!-- source: internal/component/l2tp/plugins/pool/register.go -- handle -->
<!-- source: internal/component/l2tp/plugins/pool/radius.go -- peerNegotiatedAnswer, admitPeerAddress, releasePeerAddress, addressInAnyPoolLocked -->
<!-- source: internal/component/l2tp/ppp/ncp.go -- requestIPCPAddresses, evalIPCPRequest, buildNakOrReject, copyRejectedIPCPOptions, isValidNegotiatedPeerIPv4 -->

**Framed-MTU lowers the interface MTU, never the LCP MRU.** RFC 2865 Section
5.12 gives the MTU "to be configured for the user, when it is not negotiated by
some other means (such as PPP)". In PPP, LCP opens before authentication, so
the Access-Accept arrives after the link is agreed: renegotiating LCP would
restart authentication. The value is staged on the authentication request with
the framed routes, and the session configures `pppN` with the smaller of the
LCP transmit limit and the Framed-MTU. A larger value cannot raise the
interface past what the peer agreed to receive. RFC 2865 allows 64 to 67, but
IPv4 needs 68 (RFC 791 Section 3.2), so those are raised to 68, the floor LCP
already Naks a smaller peer MRU to. accel-ppp, the implementation operators
usually migrate from, does not read Framed-MTU at all: its Access-Accept
attribute handler (`rad_proc_attrs`, `radius/radius.c`) has no case for it, and
its `pppN` MTU comes only from its `[ppp]` settings and LCP. FRR and BIRD
terminate no PPP and hold no RADIUS client, so they have no behaviour to follow.
<!-- source: internal/component/l2tp/ppp/request.go -- StageFramedMTU -->
<!-- source: internal/component/l2tp/ppp/framed_mtu.go -- interfaceMTU -->

**A goroutine per session timer, not a central timer wheel.** The expected
session count is in the low thousands. A shared wheel costs complexity that the
count does not justify.

**Idle detection reads the Linux sysfs interface statistics.** The receive-byte
counter under the interface's statistics directory needs no socket and is
readable from any goroutine. Netlink statistics were rejected for that reason.
The non-Linux build returns 0, so the timer fires unconditionally there.

<!-- source: internal/component/l2tp/iface_stats_linux.go -- interface byte counters from sysfs -->
<!-- source: internal/component/l2tp/iface_stats_other.go -- non-Linux stub -->

**The accounting interval is clamped to 60 to 3600 seconds.** A misconfigured
RADIUS server would otherwise drive an accounting storm.

**A configured `acct-interval` beats the Access-Accept, and the leaf carries no
YANG default.** RFC 2869 Section 2.1 puts the NAS in charge: "Note that a locally
configured value on the NAS MUST override the value found in an Access-Accept."
That rule needs the absent case to survive the parse. The parser wrote 300
seconds into every config that named no leaf, so every deployment looked locally
configured. That default moved into `acctIntervalDefault`, which applies only
after the Access-Accept is silent too.

The leaf lost its own `default 300` in the same change, because a schema states
what the code does. Measured on 2026-08-31, the YANG default was inert on this
path: `ParseTreeWithYANG` and `ToPluginMap` do not materialize a leaf nobody
wrote.

<!-- source: internal/component/l2tp/plugins/authradius/acct.go -- acctInterval, acctIntervalDefault -->
<!-- source: internal/component/l2tp/plugins/authradius/yang/ze-l2tp-auth-radius-conf.yang -- acct-interval -->
<!-- source: rfc/short/rfc2869.md -- RFC2869-2.1-2 -->

**A local value is not clamped twice.** The clamp above applies to the
Access-Accept value alone. The `acct-interval` leaf carries its own YANG range,
which is the same 60 to 3600 seconds. A configured value arrives inside the
range, or the commit fails.

**Accounting has its own destination port and its own client.** RFC 2866
Section 3 sends the accounting packet where "the UDP Destination Port field
indicates 1813 (decimal)", and a standard server listens for accounting there
alone: FreeRADIUS 3.2.7 refused ze's Accounting-Request on 1812 with "Invalid
packet code 4 sent to authentication port". Each `server` entry therefore
carries `accounting-port`, default 1813, beside `port`. The parser builds two
lists from the entries, in the same failover order, and the accountant gets a
`radius.Client` built on the accounting list. A `radius.Client` sends every
packet to its configured destination, and the plugin builds the separate
lists. The transport requires an Accounting-Response for that exchange and
keeps waiting after a reply of another kind. The parser writes the
1813 default itself, because the YANG default is inert on this path (see
`acct-interval` above).

<!-- source: internal/component/l2tp/plugins/authradius/config.go -- parseConfigFromTree, serverPort, accountingPortDefault -->
<!-- source: internal/component/l2tp/plugins/authradius/register.go -- activateRadiusConfig, retireAuthClient -->

**One task per session sends its Start, its Interims and its Stop.** The Stop
was once sent from the session-down callback, which waited up to 10 seconds
for the server. That callback runs on the publisher of the Down, which is the
L2TP reactor when a peer sends a CDN, so one silent accounting server stopped
the control traffic of every tunnel. The callback now records the end of
service (cause, session time, Event-Timestamp instant) and hands the Stop to
the session's accounting task, which awaits the network's terminal counters
from the original PPP unit, within 10 seconds, before it sends the Stop. One sender per session also
keeps the Stop behind an unacknowledged Start or Interim. Each record is
bounded at 10 seconds.

**A session keeps the endpoint it started under.** The client, the NAS identity
and the server address are pinned to the session at its Start, so every record
of one session reaches the server that holds its Start. A reload installs a new
endpoint for new sessions, and the replaced client closes when its last
session has sent its Stop. Shutdown and the removal of the RADIUS
configuration send an Admin Reboot Stop for every live session and wait for
every owed Stop, those already handed over included, within one shared
10-second bound. At the bound the waiting clients close, and each abandoned
Stop is logged with its Acct-Session-Id.

**A record no server acknowledged is a doctor finding, not only a log line.**
The runtime check `l2tp-radius-accounting` reports
`doctor-radius-accounting-unacknowledged` per session, named by its
Acct-Session-Id, and a later acknowledged record of the session clears it.

**Records are keyed by the session's Birth plus its wire labels.** A tunnel
reuses session IDs, so two live sessions can share a tunnel and session ID; the
Birth keeps their records apart, and a Down for one cannot end the other's.

<!-- source: internal/component/l2tp/plugins/authradius/acct.go -- runAccounting, onSessionDown, acctSession.end, acctEndpoint, setClient, stopDetached, sessionKey -->
<!-- source: internal/component/l2tp/plugins/authradius/acct_health.go -- recordAccountingOutcome, acctHealthDeclaration -->

**The subscriber's resolvers are read from the Access-Accept, and the pool
decides whether to offer them.** `extractMSDNSServers` reads
MS-Primary-DNS-Server and MS-Secondary-DNS-Server (RFC 2548 Sections 2.7.6
and 2.7.7: vendor 311, types 28 and 29, a four-octet address) into
`AuthMetadata.DNSPrimary` and `DNSSecondary`. The first well-formed attribute
of each type wins, and one whose value is not four octets names no server.
The pool's `radius` source reads the two fields; its `dns-source` list,
`radius` first by default, decides what IPCP carries
(`docs/features/dns-resolver.md`). IPv6 subscriber DNS (RFC 6911 attribute
169) is not read yet.

<!-- source: internal/component/l2tp/plugins/authradius/extract_dns.go -- extractMSDNSServers -->
<!-- source: internal/component/l2tp/session_metadata.go -- AuthMetadata -->

## What every Accounting-Request carries

Ze emits the session attributes below on every subscriber Accounting-Request.
The owner ruled them unconditional by DEFAULT on 2026-09-03. An operator who
does not want one of them holds it back with `attributes exclude`, which the
next section describes; nothing turns one on, because each is already sent.

| Attribute | Type | Records | Value |
|-----------|------|---------|-------|
| Event-Timestamp | 55 | Start, Interim, Stop | Four octets, seconds since 1970-01-01 00:00 UTC (RFC 2869 Section 5.3). On the Stop, the instant service ended, even when the Stop is sent later |
| Calling-Station-Id | 31 | Start, Interim, Stop | The L2TP Calling Number AVP, or the subscriber MAC on the PPPoE relay path (RFC 2865 Section 5.31). Omitted when neither side named one |
| Acct-Terminate-Cause | 49 | Stop ONLY | Why the session ended, as an RFC 2866 Section 5.10 integer |
| Acct-Delay-Time | 41 | Start, Interim, Stop | How many seconds the client has been trying to send this record (RFC 2866 Section 5.2). Zero on the first attempt, updated on every retransmission |

<!-- source: internal/component/l2tp/plugins/authradius/acct.go -- buildAcctPacket, acctSession.eventTime -->

**Acct-Delay-Time makes the accounting retransmit re-encode.** RFC 2866
Section 4.1: "if Acct-Delay-Time is included in the attributes of an
Accounting-Request then the Acct-Delay-Time value will be updated when the
packet is retransmitted, changing the content of the Attributes field and
requiring a new Identifier and Request Authenticator." So the client stamps the
delay at send time, and each retry takes a fresh Identifier and recomputes the
authenticator, which Section 3 derives from the attributes. The Access-Request
path is untouched and still replays its first buffer byte for byte, because RFC
2865 Section 2.5 asks for a new Identifier only when the attributes change and
none of an Access-Request's do.

The delay is measured from the first send attempt rather than from the session
event, which is what Section 5.2 defines: "how many seconds the client has been
trying to send this record". A constant zero becomes incorrect once a retry is
at least one second after the first attempt.

<!-- source: internal/component/radius/client.go -- Client.exchange, encodeRequest, setAcctDelayTime -->

**Acct-Terminate-Cause is on the Stop record and nowhere else.** RFC 2866
Section 5.10: "This attribute indicates how the session was terminated, and can
only be present in Accounting-Request records where the Acct-Status-Type is set
to Stop." Juniper's attribute table lists it on Interim as well. The RFC
outranks a vendor's table, so ze sends it on Stop alone.

### The operator can hold six of them back

The owner's 2026-09-03 ruling settled the DEFAULT, not the absence of a switch.
Junos exposes `exclude` under `[edit access profile <name> radius attributes]`,
and ze copies that shape at its own profile-shaped scope, `l2tp auth radius`.
An `attributes exclude` container names an attribute and the record types it is
held back from, and an attribute nobody names is sent.

Six attributes are excludable: Calling-Station-Id (31), Event-Timestamp (55),
Acct-Delay-Time (41), Acct-Terminate-Cause (49), NAS-Port-Id (87) and
Framed-IP-Address (8). Three attributes are NOT, and the schema does not name
them: Acct-Status-Type (40) and Acct-Session-Id (44), which RFC 2866
Section 5.13 counts as "1  Exactly one instance of this attribute MUST be
present", and the NAS identity pair its Note 1 governs.

**The enum is curated, and that is the decision.** Junos 18.1R1 added a numeric
form, `standard-attribute <number>`, whose unsupported configurations "have no
effect". A number therefore accepts a line that suppresses a mandatory attribute
and answers nothing either way. A closed enum refuses the word at configuration
load, which is where the operator can still fix it.

**Each attribute's legal record types are in the SCHEMA, not in a runtime
check.** Junos does the same: its `accounting-terminate-cause` leaf accepts only
`accounting-off`. Acct-Terminate-Cause is Stop-only in ze under RFC 2866
Section 5.10, so its `packet-type` leaf-list enumerates `accounting-stop` alone
and the configuration load refuses anything else. No packet builder ever asks
whether an exclusion was permitted.

**The filter runs once per builder, on the finished list.** `buildAcctPacket`
and `buildAccessRequestAttrs` each call `attributeExclusions.filter` after the
list is assembled, rather than guarding each append. A condition per append
grows with the attribute list and lets an attribute added later miss the feature
with nothing to say so.

**Acct-Delay-Time is the exception, because ze does not append it.** RFC 2866
Section 5.2 counts the seconds the CLIENT has been trying to send, so
`radius.Client.Exchange` writes the attribute and rewrites it on each
retransmission. Its exclusion therefore travels on the packet, as
`radius.Packet.OmitAcctDelayTime`, and a record that carries the field is
replayed byte for byte the way an Access-Request is: RFC 2866 Section 4.1 asks
for a new Identifier only "if Acct-Delay-Time is included in the attributes".

<!-- source: internal/component/l2tp/plugins/authradius/exclude.go -- attributeExclusions, filter, parseAttributeExclusions -->
<!-- source: internal/component/l2tp/plugins/authradius/yang/ze-l2tp-auth-radius-conf.yang -- attributes container -->

### What each teardown path reports

The cause is a typed value on the session-down event, never text. Each teardown
site names the cause that is TRUE of its own path.

| Teardown path | Cause | RFC 2866 Section 5.10 value |
|---------------|-------|------------------------------|
| LCP reaching Closed or Stopped without an authentication-policy refusal or a no-NCP bound expiry, including the Stopped a peer's Terminate-Request leads to | User Request | 1 |
| The LCP Close Ze drives when a session that came up holds no NCP Opened for one no-NCP bound (`ncp timeout` on L2TP, 30 seconds on PPPoE), RFC 1661 Section 3.7 "the expiration of an idle-period timer", reaching Closed | Idle Timeout | 4 |
| LCP echo probes unanswered past the limit | Lost Carrier | 2 |
| A CDN whose Result Code is 1, "Call disconnected due to loss of carrier" | Lost Carrier | 2 |
| The dead-peer keepalive timeout and the exhausted retransmit budget, which are both the peer no longer answering | Lost Carrier | 2 |
| A StopCCN the peer sent, which takes every session on the tunnel with it | Lost Service | 3 |
| Idle-Timeout expired (RFC 2865 Section 5.28) | Idle Timeout | 4 |
| Session-Timeout expired (RFC 2865 Section 5.27) | Session Timeout | 5 |
| Operator `clear l2tp session`, `clear l2tp tunnel id <tid>`, `clear l2tp tunnel all`, and RADIUS Disconnect-Request (RFC 5176) | Admin Reset | 6 |
| A CDN whose Result Code is 3, "Call disconnected for administrative reasons" | Admin Reset | 6 |
| The accounting plugin stopping, or its RADIUS configuration removed, which ends service on the NAS | Admin Reboot | 7 |
| Any failure the NAS detected: negotiation timeout, auth rejection, NCP refusal, an ioctl error, a channel fd that closed or failed to read, a refused StartSession, a message ze refused during tunnel or session setup, a kernel setup failure | NAS Error | 9 |
| A CDN carrying any other Result Code, and a CDN ze could not parse | NAS Error | 9 |
| The PPP driver stopping a running session on purpose, including an authentication-policy refusal before LCP Opened | NAS Request | 10 |

These are session-down event causes. The pre-Opened policy-refusal row does
not imply an Accounting-Stop record before authentication. The two Idle
Timeout rows are different timers: the RADIUS Idle-Timeout attribute, and the
no-NCP bound, which ends a session that carries no network protocol. The same
bound expiring before the session came up is a negotiation timeout, NAS Error.
<!-- source: internal/component/l2tp/ppp/session_run.go -- applyTransition -->
<!-- source: internal/component/l2tp/ppp/ncp_bound.go -- closeIdleLink -->

**A peer's LCP Terminate-Request reaches that row since 2026-09-20.** RFC 1661
puts Opened plus RTR into Stopping with the Restart counter zeroed, and ze's
FSM table says exactly that. The counter and its one-shot Restart timer now
exist: `applyTransition` performs irc and zrc, and the timer's expiry raises
TO-, which runs tlf and carries the session to Stopped. There tlf is what RFC
1661 Section 4.4 says it is: "the lower layer is no longer needed for the
link." The session emits its one down event at that moment, with User Request,
so the L2TP reactor sends the CDN (or the PPPoE subsystem the PADT) and the
Stop record carries User Request. The session goroutine does not end yet:
until the lower layer's teardown closes the channel, it still answers a new
Configure-Request, because RFC 1661 Section 4.3 says of RTR: "The
implementation MUST be prepared to receive a new Configure-Request without
network administrator intervention." A renegotiation that reaches Opened in
that window ends the session instead of starting authentication over a
transport being torn down, and no path reports a second down event, so the
wait in Stopped is bounded by the teardown ze itself started. The echo timer
no longer decides a peer-driven teardown, so the cause it used to report, Lost
Carrier, is gone from that path.

<!-- source: internal/component/l2tp/ppp/session_run.go -- applyTransition, handleRestartTimeout, sessionEndedAt, signalLayerFinished, sendEvent -->

**A path ze cannot attribute reports NAS Error, never a guess.** RFC 2866
Section 5.10 defines eighteen causes; ze names the nine above and no others,
because a cause it cannot source would land in an operator's billing store as a
fact. A cause of zero reaching the encoder is reported as NAS Error for the
same reason.

**Two of the twelve CDN Result Codes translate, and the rest report NAS
Error.** RFC 2661 Section 4.4.2 defines the CDN codes, and only its 1 and its 3
state something RFC 2866 Section 5.10 also states. Every other code describes a
call the LAC could not place: "Invalid destination", "Call failed due to
detection of a busy signal", "Call was not established within time allotted by
LAC". Code 2 defers its reason to an Error Code AVP whose values are protocol
and message-format errors. None of those is a Section 5.10 cause, so ze reports
the general value rather than picking the nearest-looking specific one.

**A peer StopCCN reports Lost Service whatever Result Code it carried.** RFC
2866 Section 5.10 defines Lost Service as "Service can no longer be provided;
for example, user's connection to a host was interrupted", and that is a
statement about ze rather than about the peer's intent: the tunnel those
sessions were carried on is gone. Reading the peer's StopCCN Result Code into
the subscriber's billing record would claim to know why the far end went away.

**The event fires on both teardown halves now.** A locally initiated teardown
used to remove the session from the tunnel map and emit nothing, so the PPP
event that followed found no session and returned. RADIUS then sent no
Accounting-Stop for a session-timeout, an idle-timeout, an operator clear or a
CoA-Disconnect, and the pool and the shaper never heard either. Exactly one of
the two paths emits, because both look the session up under `tunnelsMu` before
they emit.

**One removal point queues the event, so no teardown path can be silent.**
`removeSession` is the only place a session leaves a tunnel, and it performs
every obligation the removal carries: it cancels the two timers, drops the
session metadata, queues the kernel teardown when the session held kernel
resources, fails a call still waiting on an RPC, and queues the session-down
with the username and the cause read off the session struct it is about to
drop. `clearSessions` runs it for every session on a tunnel that ended. The
reactor drains both lists with `drainPendingTeardowns` while it still holds
`tunnelsMu`, and the drain takes the tunnel's order ticket for the batch. The
reactor delivers the batch after it unlocks, in that ticket's turn: a reader
runs on the delivering goroutine, and a reader run under the lock would hold
every state change of the reactor behind it. A reader must not cause a
publication for the same tunnel from its handler, because that publication
would wait behind the ticket its caller holds
(`subscriber-session-model.md`, "The order of one L2TP session's
notifications").

The username is read at removal rather than after, because it lives only on the
session struct the removal drops: a read taken later returns an empty string,
which reaches RADIUS as a Stop record with no User-Name.

This replaced three rounds of per-site collection. The PPP-driven teardown
emitted and the operator session teardowns did not; the operator session
teardowns were fixed and the operator tunnel teardown was not; the operator
paths were all fixed and the peer-driven ones were not. Each round left the
route observer working, which is why each round looked correct
(`plan/journal/guard-added-to-one-half-of-a-pair.md`, 2026-09-04).

<!-- source: internal/component/l2tp/events/terminate_cause.go -- TerminateCause -->
<!-- source: internal/component/l2tp/plugins/authradius/acct.go -- acctRetirement.stopDetached -->
<!-- source: internal/component/l2tp/session.go -- removeSession, clearSessions, drainPendingTeardowns -->
<!-- source: internal/component/l2tp/session_fsm.go -- terminateCauseForCDN, handleCDN -->
<!-- source: internal/component/l2tp/teardown.go -- teardownTunnelByID, teardownSessionOnTunnel, teardownCapturedSession, deliverSessionDowns -->
<!-- source: internal/component/l2tp/ppp/session_run.go -- fail, the LCP and echo emitters -->

**Calling-Station-Id crosses three boundaries, and it used to cross none.**
`parseICRQ` read attribute 22 off the wire and dropped it. The value now lands
on the L2TP session, rides the session-ip-assigned event, and is stored on the
accounting session, which repeats it in every record of that session.

An absent value sends NO attribute. RFC 2865 Section 5: "Text of length zero
(0) MUST NOT be sent; omit the entire attribute instead." `AppendTextAttr` is
the one place that rule is written, so every text attribute obeys it.

<!-- source: internal/component/l2tp/plugins/authradius/acct.go -- buildAcctPacket, acctNow, acctSession -->
<!-- source: internal/component/l2tp/session_fsm.go -- handleICRQ, parseICRQ -->
<!-- source: internal/component/l2tp/events/events.go -- SessionIPAssignedPayload -->

**Framed-IP-Netmask is not applied to the PPP interface.** PPP is point to
point, so the netmask only matters for delegated-prefix routing. That belongs
with the IPv6 pool work, not here.

## Native accounting wire proof

`test/l2tp/radius-acct-wire.ci` uses real kernel PPPoL2TP, CHAP and IPCP.
The peer compares the RADIUS server's decoded Accounting-Start address with
the address it negotiated, `10.99.7.10`. It keeps answering LCP echo probes
while waiting for the configured 60-second Interim-Update. Only after checking
that record does it deliberately stop answering, so the Stop must report Lost
Carrier. Every record's timestamp, calling number and delay are checked, and
Start and Interim must carry no termination cause.

The fixture's SCCRQ exchange releases its socket deadlines before returning a
successful reply. Those deadlines bound the handshake, not the subscriber
session. Leaving its 250-millisecond write deadline installed lets fast CHAP and
IPCP exchanges succeed but prevents subsequent echo replies from reaching ze,
tearing the call down before the accounting checks reach Interim.

<!-- source: internal/test/fixture/tunnel_fixture_l2tp.go -- tunnelL2TPExchange -->
<!-- source: internal/test/fixture/tunnel_fixture_l2tp_ppp.go -- tunnelL2TPRadiusAccountingPeer, checkRadius, handleAccountingPacket -->

## Consequences worth knowing

- `AuthMetadata` is the canonical carrier for RADIUS profile data. A new
  attribute such as Delegated-IPv6-Prefix or Class extends that struct.
- Named pools are available to any feature that needs pool selection, not only
  to Framed-Pool.
- The pool handler's from-pool flag names a current bitmap reservation.
  A RADIUS address inside a configured range reserves that pool and returns
  its slot at teardown. An external assignment consumes no bitmap slot.
<!-- source: internal/component/l2tp/plugins/pool/radius.go -- reserveRADIUSAddressLocked -->
<!-- source: internal/component/l2tp/plugins/pool/reload.go -- releaseAddressLocked -->

## Traps this code exists to avoid

**The metadata key is the canonical subscriber ID.** An L2TP tunnel/session
pair and a PPPoE interface/session pair can be numerically equal. The existing
subscriber ID keeps those transports separate. Transports supply it through
PPP authentication and address requests; metadata readers and teardown use
the same ID. Missing identities are refused and reported through doctor.
<!-- source: internal/component/l2tp/session_metadata.go -- StoreSessionMetadata, LoadSessionMetadata, ClearSessionMetadata -->
<!-- source: internal/component/l2tp/subscriber/id_required.go -- RequireID -->

The RADIUS provider also checks identity when called directly, before its
no-auth shortcut. A missing ID receives a synchronous refusal without invoking
the response callback. A no-auth request with a valid opaque ID remains
accepted synchronously.
<!-- source: internal/component/l2tp/plugins/authradius/handler.go -- handle -->
<!-- source: internal/component/l2tp/subscriber/session.go -- PPPKey -->

**The two timeout entry points have opposite locking rules.** Cancelling must
happen while the tunnel lock is held, inside teardown. Starting must NOT hold
that lock, because the new goroutine can immediately call teardown by session
id, which deadlocks.

**Metadata must be cleared on BOTH teardown paths.** Session teardown and tunnel
teardown each reach the store. Missing either one leaks map entries.

**Filter-Id rate accepts two spellings.** RADIUS servers vary, so both
`rate:20mbit/5mbit` and the bare `20mbit/5mbit` parse. The prefix is optional.

**One function reads them, for every caller.** `traffic.ParseFilterIDRate` is
that function. The shaper reads the Access-Accept Filter-Id through it and the
CoA listener reads the CoA-Request Filter-Id through it, so a value the NAS
accepts at login is a value it accepts in a change of authorization. The CoA
listener open-coded a `ParseRateBps` call over the whole string until
2026-09-06, which accepted `rate:10mbit` and answered CoA-NAK to
`rate:20mbit/5mbit`.

<!-- source: internal/component/traffic/filterid_rate.go -- ParseFilterIDRate -->
