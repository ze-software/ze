# RADIUS subscriber-profile attributes

A broadband network gateway provisions subscribers from RADIUS. Ze's auth
handler once rejected any Access-Accept that carried a profile attribute, which
made it unusable in that role. Each attribute now reaches its consumer:
Framed-IP-Address, Framed-Pool, Session-Timeout, Idle-Timeout, Filter-Id and
Acct-Interim-Interval.

<!-- source: internal/component/l2tp/plugins/authradius/extract.go -- extractAuthMetadata, extractFramedRoutes, isValidSubscriberIP -->
<!-- source: internal/component/l2tp/session_metadata.go -- AuthMetadata, StoreSessionMetadata, LoadSessionMetadata, ClearSessionMetadata -->
<!-- source: internal/component/l2tp/session_timeout.go -- startSessionTimeouts, runSessionTimeout, runIdleTimeout, cancelSessionTimeouts -->
<!-- source: internal/component/l2tp/plugins/shaper/filter_rate.go -- parseFilterRate -->

## Decisions

**A `sync.Map` metadata store, not a wider auth result.** The pool, the reactor,
the shaper and the accounting path all need the same data. Extending the auth
respond callback signature would have changed a contract that four independent
consumers depend on.

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

## What every Accounting-Request carries

Ze emits the session attributes below on every subscriber Accounting-Request.
The owner ruled them unconditional by DEFAULT on 2026-09-03. An operator who
does not want one of them holds it back with `attributes exclude`, which the
next section describes; nothing turns one on, because each is already sent.

| Attribute | Type | Records | Value |
|-----------|------|---------|-------|
| Event-Timestamp | 55 | Start, Interim, Stop | Four octets, seconds since 1970-01-01 00:00 UTC (RFC 2869 Section 5.3) |
| Calling-Station-Id | 31 | Start, Interim, Stop | The L2TP Calling Number AVP, or the subscriber MAC on the PPPoE relay path (RFC 2865 Section 5.31). Omitted when neither side named one |
| Acct-Terminate-Cause | 49 | Stop ONLY | Why the session ended, as an RFC 2866 Section 5.10 integer |
| Acct-Delay-Time | 41 | Start, Interim, Stop | How many seconds the client has been trying to send this record (RFC 2866 Section 5.2). Zero on the first attempt, updated on every retransmission |

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
trying to send this record". A constant zero is not a shortcut around this. It
reports a number that is true once and false on every attempt after it.

<!-- source: internal/component/radius/client.go -- Exchange, encodeRequest, setAcctDelayTime -->

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
| LCP reaching Closed or Stopped | User Request | 1 |
| LCP echo probes unanswered past the limit | Lost Carrier | 2 |
| A CDN whose Result Code is 1, "Call disconnected due to loss of carrier" | Lost Carrier | 2 |
| The dead-peer keepalive timeout and the exhausted retransmit budget, which are both the peer no longer answering | Lost Carrier | 2 |
| A StopCCN the peer sent, which takes every session on the tunnel with it | Lost Service | 3 |
| Idle-Timeout expired (RFC 2865 Section 5.28) | Idle Timeout | 4 |
| Session-Timeout expired (RFC 2865 Section 5.27) | Session Timeout | 5 |
| Operator `clear l2tp session`, `clear l2tp tunnel id <tid>`, `clear l2tp tunnel all`, and RADIUS Disconnect-Request (RFC 5176) | Admin Reset | 6 |
| A CDN whose Result Code is 3, "Call disconnected for administrative reasons" | Admin Reset | 6 |
| The accounting plugin stopping, which ends service on the NAS | Admin Reboot | 7 |
| Any failure the NAS detected: negotiation timeout, auth rejection, NCP refusal, an ioctl error, a channel fd that closed or failed to read, a refused StartSession, a message ze refused during tunnel or session setup, a kernel setup failure | NAS Error | 9 |
| A CDN carrying any other Result Code, and a CDN ze could not parse | NAS Error | 9 |
| The PPP driver stopping a running session on purpose | NAS Request | 10 |

**A peer's LCP Terminate-Request does not reach that row today.** RFC 1661
puts Opened plus RTR into Stopping, with the restart counter zeroed, and ze's
FSM table says exactly that. The restart counter and its timer are not
implemented, so nothing moves the session on to Stopped, and the session ends
later by the echo timer as Lost Carrier instead
(`plan/journal/silent-fall-through.md`, 2026-09-04). The row above is the state
that produces the cause, not the peer message that should produce the state.

<!-- source: internal/component/l2tp/ppp/session_run.go -- performAction, the LCPActIRC and LCPActZRC no-op -->

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
`tunnelsMu` and emits after it unlocks, because a subscriber runs on that
goroutine and can call back into the reactor.

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
<!-- source: internal/component/l2tp/session.go -- removeSession, clearSessions, drainPendingTeardowns -->
<!-- source: internal/component/l2tp/session_fsm.go -- terminateCauseForCDN, handleCDN -->
<!-- source: internal/component/l2tp/teardown.go -- teardownTunnelByID, teardownSessionByID, teardownSessionOnTunnel, emitSessionDowns -->
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

## Consequences worth knowing

- `AuthMetadata` is the canonical carrier for RADIUS profile data. A new
  attribute such as Delegated-IPv6-Prefix or Class extends that struct.
- Named pools are available to any feature that needs pool selection, not only
  to Framed-Pool.
- The pool handler's from-pool flag is load-bearing at teardown. An address
  assigned by RADIUS must not be released back into the pool.

## Traps this code exists to avoid

**The metadata key is the tunnel id AND the session id.** Session ids are per
tunnel, so a plain session id collides across tunnels.

**The two timeout entry points have opposite locking rules.** Cancelling must
happen while the tunnel lock is held, inside teardown. Starting must NOT hold
that lock, because the new goroutine can immediately call teardown by session
id, which deadlocks.

**Metadata must be cleared on BOTH teardown paths.** Session teardown and tunnel
teardown each reach the store. Missing either one leaks map entries.

**Filter-Id rate accepts two spellings.** RADIUS servers vary, so both
`rate:20mbit/5mbit` and the bare `20mbit/5mbit` parse. The prefix is optional.
