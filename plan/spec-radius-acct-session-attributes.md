# Spec: radius-acct-session-attributes

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | 7/7 (every phase implemented; the interop scenario is changed and UNRUN, see its row) |
| Deferral shard | - |
| Handoff | - |
| Updated | 2026-09-04 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file.
2. `internal/component/l2tp/plugins/authradius/acct.go` -- `(*radiusAcct).buildAcctPacket`, `acctSession`, `(*radiusAcct).onSessionIPAssigned`.
3. `internal/component/radius/client.go` -- `(*Client).Exchange`, the retransmit loop this spec changes.
4. `internal/component/l2tp/session_fsm.go` -- `callInfo.callingNumber`, where AVP 22 lands.
5. `internal/component/l2tp/events/events.go` -- `SessionDownPayload`, which carries no cause.

## Task

Ze's subscriber Accounting-Requests omit four attributes that every comparable
BNG sends. All four exist in `internal/component/radius/dict.go` and are emitted
nowhere: Calling-Station-Id (31), Event-Timestamp (55), Acct-Delay-Time (41) and
Acct-Terminate-Cause (49).

**Owner ruling, 2026-09-03: copy Juniper and emit all four. No config knob.**
An earlier ruling the same day made each a config option; this replaces it. Every
one of the four is an RFC 2119 MAY for a NAS, so nothing forces the choice and
`ai/rules/rfc-compliance.md` reserves it for the owner. He made it.

**One place ze does NOT copy Juniper.** Juniper's attribute table lists
Acct-Terminate-Cause on Interim as well as Stop. RFC 2866 Section 5.10: "This
attribute indicates how the session was terminated, and can only be present in
Accounting-Request records where the Acct-Status-Type is set to Stop." The RFC
outranks a vendor's behavior (`ai/rules/rfc-compliance.md`), so ze sends it on
Stop only. Nothing else in the ruling is narrowed.

### What each attribute costs, read at the producer

| Attribute | Where its value already exists | What is missing |
|-----------|-------------------------------|-----------------|
| Event-Timestamp (55) | the clock | nothing but the append. This one is a two-line change |
| Calling-Station-Id (31) | `callInfo.callingNumber` (`internal/component/l2tp/session_fsm.go`), set from AVP 22 and, on the relay path, from `req.SubscriberMAC` (`relay_sink.go`) | the value never crosses into the accounting plugin. `ppp.EventAuthRequest` does not carry it and `acctSession` has no field for it |
| Acct-Terminate-Cause (49) | nowhere | `SessionDownPayload` (`internal/component/l2tp/events/events.go`) carries TunnelID, SessionID and Username. `ppp.EventSessionDown.Reason` is free text whose own comment says the transport MUST NOT parse it for control flow. A typed cause is new |
| Acct-Delay-Time (41) | nowhere | the client cannot honestly send it today. See below |

### Why Acct-Delay-Time is a client change, not an attribute

RFC 2866 Section 4.1: "Note that if Acct-Delay-Time is included in the attributes
of an Accounting-Request then the Acct-Delay-Time value will be updated when the
packet is retransmitted, changing the content of the Attributes field and
requiring a new Identifier and Request Authenticator."

`(*Client).Exchange` (`internal/component/radius/client.go`) encodes once and
re-sends `buf[:n]` byte-identical on every retry, under a comment citing RFC 2865
Section 2.5, "retransmit uses same ID and authenticator". That is correct for an
Access-Request and wrong the moment attribute 41 is present.

RFC 2866 Section 3 makes the recomputation unavoidable: the Accounting-Request
Authenticator is "a one-way MD5 hash calculated over a stream of octets
consisting of the Code + Identifier + Length + 16 zero octets + request
attributes + shared secret". It is derived from the attributes, so changing the
delay changes it by construction.

**A constant zero is not a shortcut around this.** osvbng ships one, and it
reports a delay that is never true. accel-ppp is the reference shape: it rewrites
the value in `rad_acct_before_send` on every send and increments the Identifier
with it.

## Required Reading

### Architecture Docs
- [x] `docs/architecture/l2tp/bng-1-radius-attributes.md` -- the page that
  documents which attributes ze's subscriber accounting carries.
  → Constraint: it is SILENT on all four, so it gains a row for each in this same
  work (`ai/rules/documentation.md`), not in a follow-up.
- [ ] `docs/guide/l2tp.md` -- the operator-facing attribute list.
- [ ] `docs/research/l2tpv2-ze-integration.md` -- declared as the design document
  by `internal/component/radius/config.go` and by the L2TP sources this spec edits.
  → Constraint: it states that accounting failures MUST NOT tear down sessions
  (RFC 2866), which AC-11 holds. It says nothing about these four attributes.
- [ ] `docs/research/l2tpv2-implementation-guide.md` -- declared as the design
  document by the L2TP session and PPP sources this spec edits.
  → Constraint: its section numbering is its OWN and has no relation to RFC
  2661's (`plan/journal/reference-checked-claim-unchecked.md`, 2026-08-10), so a
  section number read here is never written into a code comment as an RFC
  citation.
- [x] `ai/rules/config.md` -- read to confirm the decision, not to add a leaf.
  → Decision: no config surface. The owner ruled all four unconditional, so there
  is no leaf, no default, and no enum. `ai/rules/simplicity.md` agrees: an option
  nobody asked for is machinery to cut.

### RFC Summaries (Scope: protocol)
- [x] RFC 2866 Section 3 -- the Accounting-Request Authenticator, quoted above.
  → Constraint: the Authenticator is content-derived, so any re-encode owes a
  fresh one.
- [x] RFC 2866 Section 4.1 -- the retransmission sentence, quoted above.
  → Constraint: a new Identifier AND a new Request Authenticator per retransmit.
- [x] RFC 2866 Section 5.2 -- Acct-Delay-Time: "how many seconds the client has
  been trying to send this record".
  → Constraint: the value is measured from the first send attempt, not from the
  session event.
- [x] RFC 2866 Section 5.10 -- Acct-Terminate-Cause, Stop only. The section
  enumerates EIGHTEEN values, not eleven: 1..11 as listed here plus Port Unneeded
  12, Port Preempted 13, Port Suspended 14, Service Unavailable 15, Callback 16,
  User Error 17 and Host Request 18. Read at `rfc/full/rfc2866.txt` on 2026-09-04;
  the earlier count in this spec was wrong.
  → Constraint: ze's cause type maps onto these integers and adds none. It names
  only the eight ze can honestly produce (1, 2, 4, 5, 6, 7, 9, 10), because a
  constant with no non-test caller is unreachable code.
- [x] RFC 2865 Section 5.31 -- Calling-Station-Id, the phone number or identifier
  the call came from.
- [x] RFC 2869 Section 5.19 -- Event-Timestamp, 0-1 in an Accounting-Request.
- [x] RFC 2869 Section 5.3 is where Event-Timestamp is defined, NOT 5.19 or 5.18
  as this spec first said. Read at `rfc/full/rfc2869.txt` on 2026-09-04: Type 55,
  Length 6, and "The Value field is four octets encoding an unsigned integer with
  the number of seconds since January 1, 1970 00:00 UTC."
  → Constraint: `AttrUint32` writes exactly that, so the encoder is one append.

## Current Behavior (MANDATORY)

**Source files read:**
- [x] `internal/component/l2tp/plugins/authradius/acct.go` -- `acctSession` holds
  `tunnelID`, `sessionID`, `username`, `peerAddr`, `acctSessID`, `nasPortID`,
  `pppInterface`, `startTime` and `cancel`. No calling-station value and no
  terminate cause.
- [x] `internal/component/l2tp/session_fsm.go` -- `callInfo.callingNumber` is
  assigned from the received AVP.
- [x] `internal/component/l2tp/relay_sink.go` -- the relay path sets
  `callingNumber` from `req.SubscriberMAC`, so the field carries a MAC there and
  a calling number on the L2TP path. Both are legitimate Calling-Station-Id
  values and the attribute is text.
- [x] `internal/component/l2tp/events/events.go` -- `SessionDownPayload` is
  `{TunnelID, SessionID, Username}`.
- [x] `internal/component/radius/client.go` -- `(*Client).Exchange` builds one
  buffer and writes `buf[:n]` inside the retry loop.

**Behavior to preserve:** every attribute the Accounting-Request carries today;
the Access-Request path, whose retransmission MUST keep the same Identifier and
Authenticator per RFC 2865 Section 2.5; the rule that an accounting failure never
tears down a session (RFC 2866, enforced in `acct.go`).

**Behavior to change:** four attributes added, and the client's accounting
retransmission path re-encodes instead of replaying a buffer.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
A subscriber PPP session starting, updating and ending. Accounting-Request
Start, Interim and Stop are built by `(*radiusAcct).buildAcctPacket`.

### Transformation Path
1. At call setup, the calling-station value travels from `callInfo.callingNumber`
   into the auth-request event payload and is stored on `acctSession`.
2. At teardown, a typed cause travels on `SessionDownPayload` into the accounting
   plugin.
3. `buildAcctPacket` appends Calling-Station-Id and Event-Timestamp to every
   record, and Acct-Terminate-Cause to Stop records only.
4. The client stamps Acct-Delay-Time at send time and re-encodes on every
   retransmission, with a fresh Identifier and a fresh Request Authenticator.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| L2TP session → PPP auth event | the calling-station value on `ppp.EventAuthRequest` | [ ] |
| PPP auth event → accounting session | a field on `acctSession` | [ ] |
| Session teardown → accounting | a typed cause on `SessionDownPayload` | [ ] |
| Accounting plugin → RADIUS client | the client owns the delay, because only it knows when it first tried | [ ] |

### Integration Points
- `internal/component/l2tp/session_fsm.go`, `relay_sink.go` -- the source value.
- `internal/component/l2tp/ppp/auth_events.go` -- the auth event payload.
- `internal/component/l2tp/events/events.go` -- the cause on `SessionDownPayload`.
- `internal/component/l2tp/plugins/authradius/acct.go` -- the three appends.
- `internal/component/radius/client.go` -- the re-encoding retransmit path.

### Architectural Verification
The delay belongs to the client and to nothing else: the accounting plugin does
not know when the first transmission happened, and a value computed at build time
would be a constant by the time it is retransmitted, which is the osvbng defect.
The cause is a typed value, so an emitter cannot invent one and a reader cannot
parse free text.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The calling-station value exists at call setup on both the L2TP and the relay path | `callInfo.callingNumber` is assigned in `session_fsm.go` and in `relay_sink.go` | Calling-Station-Id has no honest source on one path | read at the producer | confirmed |
| A-2 | No consumer of `SessionDownPayload` breaks when a field is added | the pool plugin is its named consumer | The event needs versioning | AC-12 | confirmed. Five consumers read it and every one constructs by field name: `internal/plugins/cos/handler.go`, `internal/component/l2tp/observer.go`, `internal/component/l2tp/plugins/shaper/shaper.go`, `internal/component/l2tp/plugins/authradius/acct.go` and `internal/component/l2tp/subscriber_bridge.go`. The pool plugin subscribes to `subevents.SessionDown`, which the bridge re-emits, so it never sees this struct. All five packages green |
| A-3 | Every teardown site can name a cause without guessing | the emitters listed in `plan/spec-finish-l2tp.md` | Some sites need a cause ze cannot source, which is a `NAS Error` rather than a fabrication | AC-6 | broken, and the repair landed. The spec's list of emitters was incomplete in the other direction: the session-timeout, idle-timeout, operator-clear and CoA-Disconnect paths emitted NO session-down event at all, because `teardownSessionByID` and `teardownSessionOnTunnel` removed the session from the tunnel map and the later PPP event then found nothing. `emitSessionDown` fixes those four. The TUNNEL teardown paths still have it (`plan/journal/guard-added-to-one-half-of-a-pair.md`, 2026-09-04). The `fail()` and channel-fd sites cannot be attributed and report NAS Error, which is what this row anticipated |
| A-4 | Re-encoding per retransmission does not break the Access-Request path | the two paths differ in which RFC governs the retransmit | An Access-Request retransmit changes its Identifier and violates RFC 2865 Section 2.5 | AC-9 | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The re-encode is applied to Access-Requests too | an Access-Request retransmit with a new Identifier | AC-9 asserts the Access-Request path is byte-identical across retries, which is the existing RFC 2865 Section 2.5 behavior |
| R-2 | A cause is invented where none is known | a teardown reporting User Request when ze does not know | RFC 2866 Section 5.10 has NAS Error and NAS Request for exactly this; a wrong cause is worse than a general one |
| R-3 | The delay is measured from the wrong instant | a delay that is zero on every retransmission | RFC 2866 Section 5.2 measures from when the client began trying to send this record |

## Blast Radius

Three components. `internal/component/l2tp` gains a field on two payloads.
`internal/component/l2tp/plugins/authradius` gains three appends and two stored
values. `internal/component/radius` gains a re-encoding retransmit path used by
accounting alone. The admin RADIUS path shares that client, so AC-9 exists to
prove it is unaffected.

## Wiring Test (MANDATORY -- NOT deferrable)
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| a subscriber session starting and ending | → | the Accounting-Request bytes on the wire | `test/l2tp/radius-acct-wire.ci`, extended |

The wiring test reads the wire, so an attribute that is built and never sent
fails it.

## Acceptance Criteria
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Any Accounting-Request | Carries Event-Timestamp (55), four octets, seconds since the epoch |
| AC-2 | Any Accounting-Request for a session whose calling-station value is known | Carries Calling-Station-Id (31) with that text |
| AC-3 | A session with no calling-station value | The attribute is omitted, not sent empty. RFC 2865 Section 5 forbids sending text of length zero |
| AC-4 | An Accounting-Request with Acct-Status-Type Stop | Carries Acct-Terminate-Cause (49) |
| AC-5 | An Accounting-Request with Acct-Status-Type Start or Interim | Carries NO Acct-Terminate-Cause, per RFC 2866 Section 5.10 |
| AC-6 | Each teardown path: LCP terminate, idle timeout, session timeout, admin clear, CoA-Disconnect, NAS shutdown | Each reports the RFC 2866 Section 5.10 value that is true of it, and a path ze cannot attribute reports NAS Error rather than a guess |
| AC-7 | The first transmission of an Accounting-Request | Carries Acct-Delay-Time (41) reflecting the seconds since the client began trying to send this record |
| AC-8 | A retransmission of an Accounting-Request | Carries an UPDATED Acct-Delay-Time, a new Identifier, and a new Request Authenticator computed per RFC 2866 Section 3 |
| AC-9 | A retransmission of an Access-Request | Byte-identical to the first, with the same Identifier and Authenticator, per RFC 2865 Section 2.5. The accounting change does not reach this path |
| AC-10 | Accounting-Request Authenticator on any send | MD5 over Code + Identifier + Length + 16 zero octets + attributes + secret, asserted against a vector computed independently of the producer |
| AC-11 | An accounting server that never answers | The session is unaffected; accounting failure never tears down a session |
| AC-12 | The existing consumers of `SessionDownPayload` | Still work with the added cause field |
| AC-13 | No config | No YANG leaf, no env var, and no way to disable any of the four |

## End-to-End User Stories
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Brings up a PPP subscriber and reads the Start record on the server | session up → Accounting Start | `test/l2tp/radius-acct-wire.ci` |
| 2 | Disconnects the subscriber and reads the Stop record's cause | teardown → typed cause → Accounting Stop | `test/l2tp/radius-acct-wire.ci` |
| 3 | Runs with a briefly unreachable accounting server and reads the delay | retransmit → updated delay | functional test with a paused server |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestAcctPacketCarriesEventTimestamp` | `internal/component/l2tp/plugins/authradius/acct_test.go` | AC-1 | green |
| `TestAcctPacketCarriesCallingStationId` | same | AC-2 | green, with `TestAcctSessionCallingStationID` for the boundary and `TestHandleICRQStoresCallingNumber` (`internal/component/l2tp/session_fsm_test.go`) for the source |
| `TestAcctPacketOmitsEmptyCallingStationId` | same | AC-3 | green |
| `TestTerminateCauseOnStopOnly` | same | AC-4, AC-5 | green |
| `TestTerminateCausePerTeardownPath` | same | AC-6 | green, with `TestAcctStopCarriesEventCause` for the boundary and `TestTeardownSessionByIDEmitsCause` / `TestTeardownSessionOnTunnelEmitsAdminReset` (`internal/component/l2tp/teardown_cause_test.go`) for the emitters |
| `TestAcctDelayTimeUpdatesOnRetransmit` | `internal/component/radius/client_test.go` | AC-7, AC-8 | |
| `TestAccessRequestRetransmitIsByteIdentical` | same | AC-9 | |
| `TestAccountingRequestAuthenticatorMatchesRFC2866` | `internal/component/radius/packet_test.go` | AC-10 | |

### Boundary Tests (numeric inputs)
| Input | Boundary | Expected |
|-------|----------|----------|
| calling-station value of length 0 | empty | attribute omitted |
| Event-Timestamp | 4 octets | attribute Length 6 |
| Acct-Terminate-Cause | the eight values ze names | no value outside 1..11 is ever sent; `TestTerminateCausePerTeardownPath` asserts the range |
| Acct-Delay-Time on first send | typically 0 | present, and 0 is honest here where a constant 0 on a retransmit is not |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `radius-acct-wire` | `test/l2tp/radius-acct-wire.ci` | extended to assert all four across Start, Interim and Stop | extended; see the run note below |

### Interop Tests (Scope: protocol)
| Scenario | Peer implementation | Asserts | Status |
|----------|--------------------|---------|--------|
| `04-radius-acct-attrs` (`test/interop-l2tp/scenarios/`) | xl2tpd LAC plus the lab's RADIUS mock | A real server records the attributes, and the Stop record's cause survives a round trip into the server's own store | **CHANGED AND UNRUN.** See below |

**What the functional test now asserts, and how it produces three records.**
The peer's ICRQ carries the RFC 2661 Section 4.4.3 Calling Number AVP 22, so
Calling-Station-Id has a source. `checkRecordAttributes`
(`internal/test/fixture/tunnel_fixture_l2tp_ppp.go`) then reads what the RADIUS
mock DECODED for each of the three records and asserts Event-Timestamp inside
the run's own window, Calling-Station-Id equal to that Calling Number,
Acct-Delay-Time present and numeric, and Acct-Terminate-Cause ABSENT on the
Start and on the Interim-Update and equal to 2 on the Stop. The absence is
AC-5, and it is the assertion an append in the wrong place fails.

The Interim-Update costs 60 seconds of wall clock, and that is the leaf rather
than slack: `acct-interval` has a YANG range of 60..3600 seconds, and
`acctInterval` returns a configured value unclamped, so 60 is the earliest an
interim can legally be asked for. The Stop is driven by the peer going silent:
`cqm-enabled true` drops the PPP echo interval to 1 second, the peer stops
answering, and `session_run.go` reports Lost Carrier after three misses. The
whole test takes about 70 seconds and its runner deadline is 240.

**The `.ci` change is written and UNCOMMITTED, and the owner has to release
it.** `test/l2tp/radius-acct-wire.ci` carries an `RFC requirement:` tag, so
`./le commit create` refuses the change without a row in `test/rfc-changed.md`,
and that row is the OWNER's decision which an author may not write for
themselves (the file's own "Who writes the row"). The gate is right about what
it saw: `test/rfc-changed.md` lists "An assertion ADDED to a tagged test" as a
change needing approval, "because stronger is still different". The tag's own
claim, RFC2866-4.1-1 positive over Framed-IP-Address, is untouched. The row the
owner would approve reads: `radius-acct-wire`, assertions added for the four
session attributes across Start, Interim and Stop, with no change to the
Framed-IP-Address claim the tag makes.

**It is also UNRUN, and three attempts say why.** The test is `needs-linux`, so
it runs in the QEMU guest (`./le qemu run kernel <vmlinuz> command '...'`). Two
guest runs failed to BUILD the tree, each on a different package a concurrent
session was mid-edit in: `internal/component/bgp/config/peers.go:112` at 09:35
and `internal/component/radius/authenticator_eap.go:102` at 10:15. A third
attempt, cross-compiling the binaries on the host instead, failed on an
incomplete `vendor/` and a missing build-cache entry while the disk sat at 99%.
None of the three is evidence about this change. The kernel has `CONFIG_PPPOL2TP=y`
(`gokrazy/kernel/runtime.config`), so the guest route is sound when the tree is
quiet.

**Two teardown paths were rejected for that Stop, and each is a defect the
journal now records.** A peer CDN removes the session in `handleCDN`
(`internal/component/l2tp/session_fsm.go:331`) through `removeSession`
(`session.go:243`), and neither emits `(l2tp, session-down)`, so `handlePPPEvent`
finds no session and returns: a subscriber hanging up produces NO Accounting-Stop
(`plan/journal/guard-added-to-one-half-of-a-pair.md`, 2026-09-04). A peer LCP
Terminate-Request moves LCP to Stopping, and `performAction`
(`ppp/session_run.go:1007`) treats ZRC and IRC as no-ops with no restart timer,
so the session never reaches Stopped and never ends
(`plan/journal/silent-fall-through.md`, 2026-09-04). The echo path was chosen
because it is the one that emits.

**`04-radius-acct-attrs` is changed and has not been run.** This host is darwin,
`l2tp_ppp`/`pppol2tp` is absent, and `modulesAvailable` (`internal/le/interoplab/l2tp/l2tp.go:410`)
makes the lab fail closed rather than pass on nothing. There is no QEMU route
either: `internal/le/qemu/` carries guest labs for PPPoE/accel and
VRRP/keepalived only, so `ai/rules/platform-linux.md` has no lever. What it will
prove when CI runs it on a Linux host: that a real RADIUS server decodes
Event-Timestamp, Calling-Station-Id and the Stop record's Acct-Terminate-Cause
off ze's wire. The checker (`checkRadiusAttributes`,
`internal/le/interoplab/l2tp/checkers.go`) does not yet assert the three new
attributes; extending it is phase 6.

**What the interop checker now asserts.** `checkRadiusAttributes`
(`internal/le/interoplab/l2tp/checkers.go`) keeps its NAS-Port-Id and
Framed-IP-Address assertions and adds `checkAccountingAttributes` over the
Start: Event-Timestamp within an hour of the lab clock, Acct-Delay-Time
present, Acct-Terminate-Cause ABSENT, and Calling-Station-Id ABSENT. That last
one is AC-3 against a real peer, because xl2tpd sends no Calling Number AVP and
RFC 2865 Section 5 forbids sending the text attribute empty. The checker then
POSTs `clear l2tp session all` to ze's REST surface, which this scenario's
`ze.conf` now opens on 127.0.0.1:17012, and asserts the Stop record reports
Admin Reset (6). That teardown was chosen because `TeardownAllSessions` reaches
`teardownSessionOnTunnel`, which is the half phase 3 repaired and the one that
emits.

**Why the scenario could not pass, verified at the producer.** `buildAuthAttrs`
(`internal/component/l2tp/plugins/authradius/handler.go`) returns `nil, false`
for `ppp.AuthMethodNone`, so no Access-Request leaves ze, and
`checkRadiusAttributes` waits for one. Since `6bc7b6063b` (2026-08-31) the
scenario's `auth-method none` therefore made it unpassable. `297b790446`
changed `ze.conf` to `auth-method chap-md5`, and nothing else in the directory.

**The change is now MEASURED, and it is right.** It rested on xl2tpd sending no
RFC 2661 Section 4.4.5 proxy LCP AVPs, which nobody had read. Read on
2026-09-04, in xl2tpd's own source at tags v1.3.18 and v1.3.19 and at master:
`avpsend.c` declares 26 `add_*_avp` builders and not one of them writes AVP 26,
27 or 28, and the ICRQ and ICCN branches of `control.c` send Message Type,
Assigned Call ID, Call Serial Number, Bearer Type, Random Vector, TX and RX
Connect Speed and Framing Type alone. The ICCN branch carries the comment "We
don't need any kind of proxy PPP stuff". Alpine 3.21 packages one of those
released tags (`test/interop-l2tp/Dockerfile.lac`).

So ze takes no short-circuit: `EvaluateProxyLCP`
(`internal/component/l2tp/ppp/proxy.go:69`) returns `errProxyLCPMissing`, ze
negotiates LCP directly, and the `auth-method chap-md5` leaf IS load-bearing in
this scenario. The `297b790446` edit stands.

**What would overturn it:** an xl2tpd build carrying a proxy-LCP patch, or a
different LAC image. The one-command check on a Linux host stays valid: ze logs
`ppp: proxy LCP short-circuit` with `auth-proto` (`session_run.go:154`) ONLY
when `isProxy` is true, so an absent line confirms the reading. A present line
means reading `auth-proto`, where 0 maps to `AuthMethodNone` whatever the config
says, and so does CHAP with an empty algorithm byte (`auth.go:102`).

**The log-level caveat does NOT apply to this scenario, checked at the
producers.** The line is at Info, and `ze.log.l2tp=debug` is added only for
`03-ze-lac-xl2tpd-lns` (`l2tp.go:168`). But `zePeer` sets `ZE_LOG_L2TP=debug`
on every ze peer (`l2tp.go:209`), and `env.normalize`
(`internal/core/env/env.go:42`) lowercases and turns `.` into `_`, so
`ZE_LOG_L2TP` and `ze.log.l2tp` are one cache key. The ppp session logger is the
l2tp subsystem logger with a `component=ppp` attribute
(`internal/component/l2tp/subsystem.go:309`), so it runs at that same level. An
absent line in THIS scenario is therefore evidence.

## Files to Modify
- `internal/component/l2tp/session_fsm.go` -- carry the calling-station value out.
- `internal/component/l2tp/ppp/auth_events.go` -- the auth event payload.
- `internal/component/l2tp/events/events.go` -- the cause on `SessionDownPayload`.
- `internal/component/l2tp/plugins/authradius/acct.go` -- the appends and the stored values.
- `internal/component/radius/client.go` -- the re-encoding retransmit path.
- `internal/component/radius/packet.go` -- the Accounting-Request Authenticator, if not already separable.
- `docs/architecture/l2tp/bng-1-radius-attributes.md`, `docs/guide/l2tp.md`.

## Files to Create
- `internal/component/l2tp/events/terminate_cause.go` -- the typed cause, or its nearest existing home.

### Integration Checklist
- [ ] Every new exported symbol has a non-test caller.
- [ ] The cause type's values map onto RFC 2866 Section 5.10 and add none.
- [ ] No config surface was added.

### Documentation Update Checklist (BLOCKING)
- [ ] `docs/architecture/l2tp/bng-1-radius-attributes.md` -- a row per attribute,
      and the Acct-Terminate-Cause mapping table.
- [ ] `docs/guide/l2tp.md` -- the operator-facing list.
- [ ] `docs/comparison.md` -- if it compares attribute coverage.

## Implementation Steps

### Implementation Phases
1. **Phase: Event-Timestamp.** The cheapest, and it proves the append path.
2. **Phase: Calling-Station-Id.** The value crosses two boundaries; the omission
   case is a test, not an afterthought.
3. **Phase: The cause type.** Define it, set it at every teardown emitter, carry
   it, then append it on Stop only.
4. **Phase: The client.** Separate the accounting retransmit from the Access
   retransmit, re-encode with a fresh Identifier and Authenticator, and prove the
   Access path unchanged FIRST so the regression cannot hide.
5. **Phase: Acct-Delay-Time.** Stamped by the client at send time.
6. **Phase: Functional and interop.**
7. **Phase: Docs.**

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Stop only | Acct-Terminate-Cause appears on no Start and no Interim |
| No invented cause | Every emitter's value is true of its path, or is NAS Error |
| Access path untouched | Its retransmit is still byte-identical |
| Authenticator | Recomputed per RFC 2866 Section 3 on every accounting send |
| No knob | No YANG leaf and no env var was added |
| Empty text | No zero-length attribute is ever sent |

### Deliverables Checklist
| Deliverable | Verification method | Status |
|-------------|--------------------|--------|
| Four attributes on the wire | `test/l2tp/radius-acct-wire.ci` | extended over Start, Interim and Stop |
| Stop-only cause | `TestTerminateCauseOnStopOnly` | green (phase 3) |
| Honest delay on retransmit | `TestAcctDelayTimeUpdatesOnRetransmit` | green (phase 5) |
| Access path unaffected | `TestAccessRequestRetransmitIsByteIdentical` | green (phase 4) |
| Interop proof | `radius-acct-attrs` scenario | CHANGED AND UNRUN on this host |
| Docs updated | `docs/architecture/l2tp/bng-1-radius-attributes.md`, `docs/guide/l2tp.md`, `docs/labs/l2tp-interop.md` | done |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Secret handling | The shared secret never reaches a log line or an error string |
| Identifier reuse | A fresh Identifier per accounting retransmission, and no collision with an outstanding request |
| Unbounded work | Re-encoding per retry adds one encode per retry and no allocation growth across them |
| Injection | The calling-station value is peer-supplied text and reaches an attribute; it is length-bounded and never interpreted |

### Failure Routing
| Failure | Route |
|---------|-------|
| No calling-station value | Attribute omitted; the record is still sent |
| No attributable cause | NAS Error, never a guess |
| Accounting server unreachable | Retransmit with an updated delay; the session is never torn down |

## Design Insights

Three of the four are plumbing. The fourth is a statement about the client: ze
cannot honestly say how long it has been trying to send a record while it replays
a buffer it built once. That is why osvbng's hardcoded zero is the wrong shape
rather than a small inaccuracy, and why this attribute is the one that earns real
work.

## Key Design Decisions

| Decision | Why | What it forecloses |
|----------|-----|--------------------|
| No config surface | The owner ruled all four unconditional | An operator cannot suppress one for a server that dislikes it |
| Acct-Terminate-Cause on Stop only | RFC 2866 Section 5.10 says so, and it outranks Juniper's table | Parity with Juniper on that one row |
| The client owns the delay | Only it knows when it first tried to send | Computing it in the accounting plugin, which would freeze at build time |
| A typed cause | A free-text reason cannot be mapped without parsing text the comment forbids parsing | A cause a plugin could invent |

## Known Limitations

- The cause for a teardown ze cannot attribute is NAS Error. That is honest and
  it is less specific than a BNG with deeper teardown instrumentation.

## RFC Documentation (Scope: protocol)
- RFC 2866 Sections 3, 4.1, 5.2 and 5.10; RFC 2865 Sections 5 and 5.31; RFC 2869
  Section 5.19. All three are enrolled. The requirement ids this work adds carry
  tagged tests and a discrimination record in the same change.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [x] Every producer named above was read, not inferred.
- [x] The RFC sentences were read in `rfc/full/rfc2866.txt`, not in a summary.
- [x] The vendor comparison was read from source where the source is public.
- [x] The owner ruled on the MAY.

### Goal Gates (MUST pass)
- [ ] AC-1..AC-13 demonstrated
- [ ] Access-Request retransmit proven unchanged BEFORE the accounting change lands
- [ ] Interop scenario passes
- [ ] `./le verify worktree` passes

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Closure
- [ ] The row in `plan/spec-finish-l2tp.md` resolved and repointed
- [ ] Citations repointed

## Implementation Summary

### What Was Implemented
- Event-Timestamp (55) on every Accounting-Request (`f38bb0e2da`), appended by
  `buildAcctPacket` (`internal/component/l2tp/plugins/authradius/acct.go`).
- Calling-Station-Id (31) carried from `callInfo.callingNumber` through the auth
  event onto `acctSession`, and omitted rather than sent empty (`547910de0d`).
- A typed `TerminateCause` (`internal/component/l2tp/events/terminate_cause.go`)
  set at every teardown emitter, carried on `SessionDownPayload`, and appended as
  Acct-Terminate-Cause (49) on the Stop record alone (`1262ea2cae`).
- The accounting retransmit re-encodes: `setAcctDelayTime` stamps Acct-Delay-Time
  (41) before the first send and again before each retry, and `encodeRequest`
  takes a fresh Identifier and recomputes the RFC 2866 Section 3 authenticator.
  The Access-Request path still replays its first buffer (`5674137ff`).
- `checkAccountingAttributes` in the interop checker
  (`internal/le/interoplab/l2tp/checkers.go`), swept into `ad6a5bd7f0` by another
  session's concurrent commit (`plan/journal/concurrent-session-corruption.md`).
- At closure: `internal/component/radius/acct_delay_time_retransmit_test.go`, the
  AC-7/AC-8 test the TDD table named and nobody wrote.

### Bugs Found/Fixed
- AC-7 and AC-8 had no test asserting the delay VALUE moves. The only retransmit
  test watched the Identifier, so it would pass against a client that moved the
  Identifier and left a constant zero delay, which is the osvbng shape this spec
  exists to avoid. Fixed by `TestAcctDelayTimeUpdatesOnRetransmit`, whose red
  phase was forced and observed. Row in `plan/journal/stale-spec-claims-done.md`.
- `rfc/short/rfc2866.md` pinned the pre-change behavior: RFC2866-3-3 read
  "retransmitted request MUST use the same Identifier", dropping the RFC's own
  condition "where the contents are identical", and its Enrolment reason named a
  renamed test and a producer line range that no longer exists. Fixed and
  regenerated. Row in `plan/journal/claim-outlives-the-evidence-it-cites.md`.

### Documentation Updates
- `docs/architecture/l2tp/bng-1-radius-attributes.md`: the attribute table, the
  teardown-cause mapping and the boundary narrative landed in `f38bb0e2da`,
  `547910de0d` and `1262ea2cae`.
- `docs/guide/l2tp.md`: the operator-facing list landed in `1262ea2cae` and
  `ad6a5bd7f0`. `docs/labs/l2tp-interop.md` landed in `ad6a5bd7f0`.
- `rfc/short/rfc2866.md` Meta and RFC2866-3-3, then `./le rfc index-update`, which
  regenerated `rfc/enrolled.txt`, `rfc/requirements/rfc2866.md` and
  `docs/features/rfc-status.md`.
- NOT LANDED, and the reason is not this spec's: the Acct-Delay-Time row and the
  "makes the accounting retransmit re-encode" block in
  `docs/architecture/l2tp/bng-1-radius-attributes.md` are written in the working
  tree and sit in the same file as `spec-radius-attribute-exclusion`'s
  uncommitted `attributes exclude` hunks. A file stages whole, so landing mine
  lands theirs under this message. `ai/rules/never-destroy-work.md` and
  `ai/rules/principles.md` both say leave it, and that spec's own Phase line
  already says its doc hunks wait on this page.
- `./le doc check verify` FAILS at HEAD on the CLI command catalog and the HTML
  command index, from another session's in-flight command-help work. No failure
  names a RADIUS or L2TP accounting surface, `bng-1-radius-attributes.md`,
  `rfc/short/rfc2866.md` or `docs/features/rfc-status.md`.

### Deviations from Plan
- The AC-7/AC-8 test lives in
  `internal/component/radius/acct_delay_time_retransmit_test.go`, not in
  `client_test.go` as the TDD table said. A new file keeps the commit gate away
  from the tagged functions `client_test.go` already carries.
- `TestAccountingRequestAuthenticatorMatchesRFC2866` was not written under that
  name. `TestRFC2866AccountingRequestAuthFormula` already asserts AC-10 against an
  independent MD5 reference, so a second test of the same formula was cut.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-4 was carried as `unvalidated` to closure | The Access-Request path is provably untouched: `Exchange` re-encodes only under `attempt > 0 && pkt.Code == CodeAccountingReq`, and `encodeRequest` returns `pkt.Authenticator` unchanged for every other code | read at the producer during closure, `internal/component/radius/client.go` | A-4 marked confirmed, with `TestAccessRequestRetransmitIsByteIdentical` as its evidence |
| approach | The Deliverables Checklist read "green (phase 5)" for `TestAcctDelayTimeUpdatesOnRetransmit`, and its TDD Status cell was empty | The test did not exist. AC-7 and AC-8 rested on an Identifier assertion that a constant-zero delay would still pass | closure step 1 grepped for each named test | Test written, red phase forced, journal row in `stale-spec-claims-done.md` |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Emit all four attributes, no config knob | Done | `authradius/acct.go` `buildAcctPacket`, `radius/client.go` `setAcctDelayTime` | The owner's 2026-09-03 ruling. `spec-radius-attribute-exclusion` later added an opt-OUT, which does not change this default |
| Acct-Terminate-Cause on Stop only | Done | `authradius/acct.go` `buildAcctPacket` | RFC 2866 Section 5.10, against Juniper's table |
| The client owns the delay and re-encodes per retry | Done | `radius/client.go` `Exchange`, `encodeRequest`, `setAcctDelayTime` | RFC 2866 Sections 3, 4.1 and 5.2 |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestAcctPacketCarriesEventTimestamp` | |
| AC-2 | Done | `TestAcctPacketCarriesCallingStationId`, `TestAcctSessionCallingStationID`, `TestHandleICRQStoresCallingNumber` | |
| AC-3 | Done | `TestAcctPacketOmitsEmptyCallingStationId` | |
| AC-4, AC-5 | Done | `TestTerminateCauseOnStopOnly` | |
| AC-6 | Done | `TestTerminateCausePerTeardownPath`, `TestTeardownSessionByIDEmitsCause`, `TestTeardownSessionOnTunnelEmitsAdminReset` | |
| AC-7, AC-8 | Done | `TestAcctDelayTimeUpdatesOnRetransmit`, `TestRFC2866AccountingRetransmitTakesANewIdentifier` | The first was written at closure; see the Mistake Log |
| AC-9 | Done | `TestAccessRequestRetransmitIsByteIdentical` | |
| AC-10 | Done | `TestRFC2866AccountingRequestAuthFormula`, and the independent MD5 in `TestAcctDelayTimeUpdatesOnRetransmit` over the retransmitted datagram | |
| AC-11 | Done | `TestRFC2866AcctFailureKeepsSession`, `TestRFC2866SessionTeardownIndependentOfAccounting` | Pre-existing and unchanged |
| AC-12 | Done | A-2's five consumers, all green | |
| AC-13 | Done | No leaf, no default and no env var added by this spec | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestAcctPacketCarriesEventTimestamp` | Done | `authradius/acct_test.go` | |
| `TestAcctPacketCarriesCallingStationId` | Done | same | |
| `TestAcctPacketOmitsEmptyCallingStationId` | Done | same | |
| `TestTerminateCauseOnStopOnly` | Done | same | |
| `TestTerminateCausePerTeardownPath` | Done | same | |
| `TestAcctDelayTimeUpdatesOnRetransmit` | Done | `radius/acct_delay_time_retransmit_test.go` | Different file; written at closure |
| `TestAccessRequestRetransmitIsByteIdentical` | Done | `radius/client_test.go` | |
| `TestAccountingRequestAuthenticatorMatchesRFC2866` | Changed | `radius/rfc2866_accounting_test.go` `TestRFC2866AccountingRequestAuthFormula` | Same assertion under the existing name; see Deviations |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/l2tp/session_fsm.go` | Done | |
| `internal/component/l2tp/ppp/auth_events.go` | Changed | The value travels on `ppp/events.go` and `session.go` instead |
| `internal/component/l2tp/events/events.go` | Done | |
| `internal/component/l2tp/events/terminate_cause.go` | Done | Created |
| `internal/component/l2tp/plugins/authradius/acct.go` | Done | |
| `internal/component/radius/client.go` | Done | |
| `internal/component/radius/packet.go` | Changed | `AccountingRequestAuth` was already separable, so no edit was needed |
| `docs/architecture/l2tp/bng-1-radius-attributes.md`, `docs/guide/l2tp.md` | Partial | See Documentation Updates: the Acct-Delay-Time hunk is written and blocked behind another spec's hunks in the same file |

### Audit Summary
- **Total items:** 25
- **Done:** 21
- **Partial:** 1 (the Acct-Delay-Time doc hunk, blocked by a shared file, written and named)
- **Skipped:** 0
- **Changed:** 3 (recorded in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Ze's Accounting-Requests carry the four attributes every comparable BNG sends | functional | `test/l2tp/radius-acct-wire.ci` asserts all four off the wire across Start, Interim-Update and Stop, reading what the RADIUS mock DECODED rather than what ze built |
| Acct-Terminate-Cause appears on Stop and nowhere else (RFC 2866 Section 5.10) | functional | The same `.ci` asserts ABSENCE on the Start and on the Interim-Update and `Acct-Terminate-Cause 2` on the Stop, which an append one line too early fails |
| The delay ze reports is honest on a retransmission, unlike osvbng's constant zero | unit, with a forced red | `TestAcctDelayTimeUpdatesOnRetransmit` reads both datagrams: delay 0 on the first, at least 1 after a 1.1s retry, a distinct Identifier, and a Request Authenticator equal to an independent MD5 of the second datagram. Setting `seconds = uint32(elapsed / time.Second)` to a constant zero fails it with "the retransmission reported Acct-Delay-Time 0 after a 1.1s wait" |
| The Access-Request path is unaffected | unit | `TestAccessRequestRetransmitIsByteIdentical` compares the captured datagrams byte for byte, and was written BEFORE the divergence existed |
| A real RADIUS server decodes what ze sends | interop | **NOT DEMONSTRATED.** `04-radius-acct-attrs` is CHANGED AND UNRUN. See Deferrals Resolved |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| `plan/deferrals/radius-subscriber-attributes.md`, row 2: Calling-Station-Id, Event-Timestamp, Acct-Delay-Time and Acct-Terminate-Cause present in the dictionary but not emitted | done | Built by this spec. The row now names `buildAcctPacket` and `setAcctDelayTime` and points at `docs/architecture/l2tp/bng-1-radius-attributes.md` |
| `plan/deferrals/radius-subscriber-attributes.md`, row 1: interim-update scheduling as a timer wheel | deferred | Untouched by this spec, homed at `plan/future/spec-radius-acct-timewheel.md`. The shard stays for it, so it is not removed |
| The interop scenario `04-radius-acct-attrs` | deferred | CHANGED AND UNRUN, and not runnable here. This host is darwin with no `l2tp_ppp`, colima's kernel carries no module either, and `internal/le/qemu/` holds guest labs for PPPoE and VRRP alone, so `ai/rules/platform-linux.md` has no lever. `modulesAvailable` (`internal/le/interoplab/l2tp/l2tp.go`) fails the lab closed rather than passing on nothing, which is correct and is the reason there is no result to report. CI on a Linux host is what runs it |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/radius-acct-session-attributes-f89390ec-889f-4a7a-8172-1e2cfd108a12.md` (29 files, verdict=clean) |
| `./le spec session review check` | OK, hashes match |
| Rounds | 2. Round 1 found the two ISSUEs below; round 2 read the fixes and found nothing |
| Reviewer lenses used | wiring + functional coverage, logic + guard audit + removed-behavior, RFC conformance + documentation drift, and the `ze-go-style` pass over every changed Go file |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | AC-7 and AC-8 had no test asserting the Acct-Delay-Time VALUE updates on a retransmission, and the Deliverables Checklist claimed one green that was never written | `internal/component/radius/client.go` `setAcctDelayTime`, and the spec's own tables | `internal/component/radius/acct_delay_time_retransmit_test.go`, with `wireAttrValue` extracted from `wireCarriesAttr` so one packet walk serves both. Red phase forced and observed |
| 2 | ISSUE | The RFC ledger pinned the behavior this spec correctly changed: RFC2866-3-3 dropped the RFC's "where the contents are identical" condition, and its Enrolment reason named a renamed test and a dead producer line range | `rfc/short/rfc2866.md` | Requirement row and Enrolment reason rewritten against `rfc/full/rfc2866.txt` Section 4.1, then `./le rfc index-update` |

NOTEs, recorded and not blocking: the Acct-Delay-Time doc hunk cannot be staged
alone (Documentation Updates), and `04-radius-acct-attrs` is unrun on this host
(Deferrals Resolved).

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/l2tp/events/terminate_cause.go` | yes | `gopls symbols` lists `TerminateCause` and its constants |
| `internal/component/radius/acct_delay_time_retransmit_test.go` | yes | created this session, and `go test` ran it |
| `test/l2tp/radius-acct-wire.ci` | yes | `git diff` shows the four added `expect=` lines |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-7, AC-8 | The delay is stamped on the first send and updated on the retransmission, with a new Identifier and a new authenticator | `--- PASS: TestAcctDelayTimeUpdatesOnRetransmit (1.10s)`, and `--- FAIL ... reported Acct-Delay-Time 0 after a 1.1s wait` with the producer broken |
| AC-9 | The Access-Request retransmit is byte-identical | `--- PASS: TestAccessRequestRetransmitIsByteIdentical (0.20s)` |
| AC-10 | The accounting authenticator is the RFC 2866 Section 3 MD5 | `--- PASS: TestRFC2866AccountingRequestAuthFormula (0.00s)` |
| AC-4, AC-5 | Acct-Terminate-Cause on Stop only | `grep -n 'func TestTerminateCauseOnStopOnly' internal/component/l2tp/plugins/authradius/acct_test.go` returns line 563 |
| AC-13 | No config surface was added by this spec | The `attributes exclude` container in the authradius YANG belongs to `spec-radius-attribute-exclusion`, whose own Task section says so |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| A subscriber PPP session starting, running an acct-interval, and losing carrier | `test/l2tp/radius-acct-wire.ci` | Read, not inferred: the fixture peer sends the ICRQ Calling Number AVP 22, then `checkRecordAttributes` reads the RADIUS mock's decode of all three records. `./le repository check` reports no unwired export in `internal/component/l2tp` or `internal/component/radius` |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `callInfo.callingNumber` is assigned in `session_fsm.go` and in `relay_sink.go` |
| A-2 | confirmed | Five consumers, each constructing by field name, and all five packages green |
| A-3 | broken, repaired | `emitSessionDown` now covers the four teardown paths that emitted nothing; `plan/journal/guard-added-to-one-half-of-a-pair.md` |
| A-4 | confirmed | `Exchange` re-encodes only under `attempt > 0 && pkt.Code == CodeAccountingReq`, and `encodeRequest` returns `pkt.Authenticator` unchanged for every other code. `TestAccessRequestRetransmitIsByteIdentical` is the assertion |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| The teardown-cause mapping table in `bng-1-radius-attributes.md` | `internal/component/l2tp/events/terminate_cause.go`, read at closure: every constant's doc comment quotes its RFC 2866 Section 5.10 sentence, and `session_run.go` emits `TerminateCauseLostCarrier` on the echo path | yes |
| `rfc/short/rfc2866.md` RFC2866-3-3 | `rfc/full/rfc2866.txt` Section 4.1, quoted in the row | yes |
| `docs/features/rfc-status.md` RFC 2866 row | Generated by `./le rfc index-update` from the Meta table above | yes |
| The Acct-Delay-Time row and re-encode block in `bng-1-radius-attributes.md` | Written against `radius/client.go` `Exchange`, `encodeRequest` and `setAcctDelayTime`; NOT staged, because the file also holds another spec's hunks | written, not landed |

## Core Insight

Three of the four attributes were plumbing, and the fourth was a statement about
the client. What nearly escaped is that the same split applies to the TESTS: an
Identifier assertion is plumbing and a delay assertion is the statement, and only
one of them can tell osvbng's constant zero from an honest value. This record's
Deliverables row said the statement was proven. Its TDD Status cell, left empty,
said it was not. The empty cell was right.
