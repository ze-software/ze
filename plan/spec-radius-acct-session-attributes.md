# Spec: radius-acct-session-attributes

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | 3/7 (Event-Timestamp, Calling-Station-Id and the cause are done; the client and Acct-Delay-Time are not) |
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
| `radius-acct-wire` | `test/l2tp/radius-acct-wire.ci` | extended to assert all four across Start, Interim and Stop | |

### Interop Tests (Scope: protocol)
| Scenario | Peer implementation | Asserts | Status |
|----------|--------------------|---------|--------|
| `04-radius-acct-attrs` (`test/interop-l2tp/scenarios/`) | xl2tpd LAC plus the lab's RADIUS mock | A real server records the attributes, and the Stop record's cause survives a round trip into the server's own store | **CHANGED AND UNRUN.** See below |

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

**Why the scenario could not pass, verified at the producer.** `buildAuthAttrs`
(`internal/component/l2tp/plugins/authradius/handler.go`) returns `nil, false`
for `ppp.AuthMethodNone`, so no Access-Request leaves ze, and
`checkRadiusAttributes` waits for one. Since `6bc7b6063b` (2026-08-31) the
scenario's `auth-method none` therefore made it unpassable. `297b790446`
changed `ze.conf` to `auth-method chap-md5`, and nothing else in the directory.

**The change is REASONED, not verified.** It rests on xl2tpd sending no RFC 2661
proxy LCP AVPs, which nobody read out of xl2tpd's source or a capture. The ze
half IS verified: `EvaluateProxyLCP` (`internal/component/l2tp/ppp/proxy.go:69`)
returns `errProxyLCPMissing` when any of the three AVPs is empty, so `isProxy`
is false and `session_run.go` negotiates LCP directly, where the `auth-method`
leaf selects the method. If xl2tpd DOES send them, ze short-circuits,
`authMethodFromAuthProto` (`internal/component/l2tp/ppp/auth.go:97`) reads
Auth-Protocol out of the LAC's Last-Sent CONFREQ, the leaf is inert exactly as
it was in `test/l2tp/radius-acct-wire.ci`, and the repair is fixture-side as in
`bab29e430`.

**How to settle it in one grep.** Ze logs `ppp: proxy LCP short-circuit` with
`auth-proto` and `auth-method` (`session_run.go:154`), and ONLY when `isProxy`
is true. Absent line means the leaf is load-bearing and the edit is right.
Present line means read `auth-proto`: 0 maps to `AuthMethodNone` whatever the
config says, and so does CHAP with an empty algorithm byte (`auth.go:102`).

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
| Four attributes on the wire | `test/l2tp/radius-acct-wire.ci` | |
| Stop-only cause | `TestTerminateCauseOnStopOnly` | |
| Honest delay on retransmit | `TestAcctDelayTimeUpdatesOnRetransmit` | |
| Access path unaffected | `TestAccessRequestRetransmitIsByteIdentical` | |
| Interop proof | `radius-acct-attrs` scenario | |
| Docs updated | the two page diffs | |

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
