# Spec: bmp-sflow-export-rfc-defects

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Depends | the uncommitted RFC 7854 Section 3.3 change in `internal/component/bgp/plugins/bmp/bmp_events.go` (Peer Down only for a peer reported up, `peerDownCause.reported`), which this design builds on and which lands first |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-13 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Two defects in what Ze reports to a monitoring collector, found by the strict
re-read of `spec-rfc-requirement-quote-hand-backfill`. A BMP
station is told Ze closed a session the peer dropped, and an sFlow collector's
sample pool stops advancing once it saturates. Both were read at the producer
in HEAD on 2026-09-27.

## Defects

| ID | Source | Verbatim quote | Producer | What Ze does wrong | Audit verdict / record |
|----|--------|----------------|----------|--------------------|------------------------|
| D1 | RFC 7854 Section 4.9 | "Reason 4: The remote system closed the session without a notification message.  This includes any unexpected termination of the transport session, so in some cases both the local and remote systems might consider this to apply." | `internal/component/bgp/plugins/bmp/bmp_events.go::peerDownFor` | Every close without a NOTIFICATION is reported as reason 2 (local close, FSM event 0); reason 4 is never sent, including for a TCP reset or a connection the peer dropped | no row names it |
| D2 | sFlow v5 (`rfc/full/sflow-v5.txt`, Section 5, `flow_sample`), row SFLOW-V5-x-11. NO NORMATIVE SENTENCE: a Ze behaviour defect measured against the field definition | the definition: "unsigned int sample_pool;      /* Total number of packets that could have been sampled (i.e. packets skipped by sampling process + total number of samples) */", and the note on `sequence_number`: "If the agent resets the sample_pool then it must also reset the sequence_number." | `internal/plugins/flowexport/sflow/flow_adapter.go::EncodeFlowSample` | The pool is estimated as sequence times rate and saturates at 2^32-1. Correction (RFC-tagged test review, 2026-10-13): sFlow v5 Section 5's conventions do speak to it, "Unknown counter. Use the maximum counter value to indicate that the counter is not available." and "An available counter may temporarily have the max value just before it rolls to zero. This is permitted.", so a saturated pool announces "not available" and rolling to zero is what the text expects. The defect is that a saturated value stops being the count the definition names, and the lowercase note treats the pool as a cumulative counter reset only together with the sequence number, which saturation is not. Once saturated, a collector's per-interval delta reads zero | weak; lead, not checked against a collector. Row SFLOW-V5-x-11 itself quotes a field description tagged MUST: a row correction, listed with the other sFlow corrections in `plan/pre-release/spec-rfc-verdict-test-fix-pass.md` |

## What a fix must prove

| Obligation | Detail |
|------------|--------|
| Failing test first | D1: a peer-down after a TCP loss yields reason 4. D2: a pool past 2^32 wraps |
| Both polarities | D1: a local close without NOTIFICATION stays reason 2, a NOTIFICATION stays reason 1 or 3. D2: below the limit the value is unchanged |
| Discrimination | `./le rfc discriminate-record` for every tagged unit |
| Interop | D1 against a BMP collector (the existing BMP interop scenario); D2 against sflowtool or host-sflow, reading the collector's delta |

## Owner decisions

| Row | Question |
|-----|----------|
| D2 | Wrap versus saturate is NOT silent (RFC-tagged test review, 2026-10-13, checked against `rfc/full/sflow-v5.txt:1887-1892`): sFlow v5 Section 5 says "Unknown counter. Use the maximum counter value to indicate that the counter is not available." and "An available counter may temporarily have the max value just before it rolls to zero. This is permitted." Saturating at 2^32-1 therefore announces "not available", and rolling to zero is what the text expects, so the wrap is the text's answer. What stays the owner's call is the estimate-versus-count question: sample_pool is defined as "Total number of packets that could have been sampled (i.e. packets skipped by sampling process + total number of samples)", and the encoder writes an estimate (sequence times rate, or the accumulated rate). Whether the pool should count packets actually seen is Ze's choice, and the tag prose states it as Ze's choice, not as sFlow text |

### Open owner questions (design, 2026-10-13)

Each row carries the reading the design proceeds under until the owner answers.

| ID | Question | Options | Design proceeds under |
|----|----------|---------|-----------------------|
| Q-1 | D1 approach: where the close cause is decided | (A) the session records a typed close cause, first writer wins, at every local close site and on a received NOTIFICATION; the down event carries it; BMP keeps its NOTIFICATION cache for the reason 1 and 3 PDU. (B) the reactor carries the closing NOTIFICATION PDU itself on the down event, built or received, delivered or not, and BMP's `notifyCache` is deleted | (A). Smaller, keeps the grant contract (a binding without `notification` never sees a NOTIFICATION). (B) is the RFC-literal answer for Q-2 and Q-3 and is the one to pick if the owner wants reason 1 for a NOTIFICATION whose write failed |
| Q-2 | A remote NOTIFICATION ended the session but the BMP binding does not receive `notification`, so no PDU is cached. Reason 3 without the PDU is malformed (Section 4.9: "Following the Reason is a BGP PDU containing the BGP NOTIFICATION message as received from the peer"), and reason 4 says "without a notification message", which is false. The same gap hits a NOTIFICATION ze sent and delivered: with no `notification` grant nothing is cached, and reason 2 ("No notification message was sent") would be false | (a) reason 4 plus a WARN naming the missing grant; (b) config refusal: a binding that feeds `bgp-bmp` state without `notification` fails the commit; (c) take approach B of Q-1 | (b), a commit-time refusal, because every other answer either lies to the collector or is approach B. The refusal text names the binding and the missing grant |
| Q-3 | A local NOTIFICATION ze decided to send but whose write failed (TCP already gone). RFC 7854 Section 4.9 (`rfc/full/rfc7854.txt:863-865`): "Reason 1: The local system closed the session.  Following the Reason is a BGP PDU containing a BGP NOTIFICATION message that would have been sent to the peer." Reason 2: "The local system closed the session.  No notification message was sent." "Would have been sent" reads as covering a NOTIFICATION ze built and could not deliver, so reason 2 for this case is a reading, not the text. Three RFC-tagged units depend on the answer: `TestHandleSenderStatePeerDown` (RFC7854-x-10) and `TestRFC7854PeerDownCarriesTheDataItsReasonRequires` (RFC7854-4.9-1) both drive ManualStop with no cached NOTIFICATION, which in production is exactly this failed-write case, and AC-6 | (a) reason 2 with the FSM event that started the close (ManualStop 00 02, AutomaticStop 00 08, HoldTimer_Expires 00 0A, OpenCollisionDump 00 17, UpdateMsgErr 00 1C); (b) reason 1 with the PDU, which needs approach B of Q-1 | NO RECOMMENDATION (owner, 2026-10-13: not answered). Research: GoBGP, BIRD and rustbgpd send reason 1 with the PDU when the write fails, which is option (b). The draft is written under (a) only so AC-5, AC-6 and the Q-7 edits have a concrete shape; under (b) AC-6 becomes reason 1 with the built PDU, Q-1 must take approach B, and the two tagged units above assert reason 1 with that PDU instead of 00 02 |
| Q-4 | The draft-ietf-idr-bgp-bfd-strict-mode events 30 to 35 are not in RFC 4271 Section 8.1, which Section 4.9 points at ("see Section 8.1 of [RFC4271]"). A strict BFD Down whose Cease write failed is reason 2 with which event code | (a) 00 1F (Event 31, the number the draft's Section 13.3 registers); (b) 00 00 ("no relevant Event code is defined") | (b), 00 00, because the RFC names Section 8.1 and the draft is not an RFC. One table row flips it |
| Q-5 | Closes ze makes with no NOTIFICATION and no FSM stop event of its own: the GR-aware congestion teardown (`forward_pool_congestion.go` closes TCP and fires Event 18 to simulate a transport failure), context cancellation, `StopForRestart` | (a) reason 2 with 00 00; (b) reason 4, since Event 18 is what the FSM processed | (a). The local system closed the session, and Event 18's definition ("the local system has received a TCP connection failure notice", RFC 4271 Section 8.1.4) is not what caused it |
| Q-6 | The textual state reason (`connection lost` / `session closed`, `peer_run.go`) reaches JSON and text consumers. Every exit from Established goes to Idle, so `session closed` is never produced on that path, and `connection lost` names every cause | (a) leave the text untouched, this spec adds typed fields beside it; (b) derive the text from the new close cause | (a). Changing a JSON value external consumers read is a separate decision; the dead `session closed` arm gets a journal row |
| Q-7 | Approval to edit the RFC-tagged tests listed in the TDD plan: `TestHandleSenderStatePeerDown` (RFC7854-x-10, body tightens and tag prose narrows to what the body drives), `TestBMPSenderPeerDown` (RFC7854-x-10, no body change, prose stays narrow), `TestRFC7854PeerDownCarriesTheDataItsReasonRequires` (RFC7854-4.9-1 positive), `TestRFC7854PeerDownOmitsDataWhereTheReasonHasNone` (RFC7854-4.9-1 negative, gains reason 4 arm and prose naming reason 4), `TestSFlowFlowSamplePoolTracksTotal` (SFLOW-V5-x-11, wrap plus prose rewrite). The first and third bodies depend on Q-3, so Q-7 is answered after Q-3 | approve / refuse | **Answered by the owner, 2026-10-13: "approve all RFC-tagged test changes".** The approval covers the five edits listed in the TDD plan. Q-3 is still open, so for `TestHandleSenderStatePeerDown` and `TestRFC7854PeerDownCarriesTheDataItsReasonRequires` it covers the edit for whichever Q-3 answer is recorded ((a) reason 2 with 00 02, or (b) reason 1 with the built PDU); this approval does not answer Q-3. The commands are under "Owner approval commands" in the TDD plan |
| Q-8 | D2 interop: no sFlow interop scenario exists, sampling in the lab needs a tc `sample` action and psample in a container, and the wrap at 2^32 cannot be crossed with real traffic (rate is bounded 1..1000000 by `ze-flowexport-conf.yang`, so the pool reaches 2^32 only after about 4.3e9 packets) | (a) create `sflow-sample-pool-sflowtool` proving sflowtool decodes the pool and its per-sample delta equals the rate, wrap proven in unit only; (b) no interop for D2, unit plus decoded datagram only | (a), because `ai/rules/interop-and-goal-validation.md` says a missing scenario is created, and the wrap is a pure encoder property the unit test reaches with rate MaxUint32 |

## Required Reading

### Architecture Docs
- [ ] `docs/guide/bmp.md` - the BMP sender's documented Peer Down behaviour (line 162, line 186: "with that peer's identity and mapped reason code")
  → Constraint: the page says only "mapped reason code"; the reason-by-cause table this spec produces goes on this page in the same change
  → Constraint: line 186 already carries the Section 3.3 rule (Peer Down only for a peer whose Peer Up was sent), written by the in-flight change this spec depends on
- [ ] `docs/architecture/behavior/peer-lifecycle.md` - "On transition out of StateEstablished", step 1 "Determine a textual reason"
  → Constraint: step 1 and step 3 describe the reason that reaches `announceClosed`; the typed close cause this spec adds is a new step and the page changes with it
- [ ] `docs/architecture/behavior/fsm.md` (line 484) - quotes the `connection lost` arm
  → Constraint: stays true under Q-6 (a); re-read when the callback changes
- [ ] `docs/architecture/testing/interop.md` - pmacct `pmbmpd` row (line 72): "pmacct decodes the per-peer header, the Peer Up Information TLVs and the Peer Down reason itself"
  → Decision: the D1 interop reads pmacct's `reason_type`, the field `bmp-locrib-pmacct` already asserts (`pmacctPeerDownReasonSix` in `internal/le/interoplab/bgp/names.go`)
  → Constraint: a scenario directory is named, never numbered; a scenario carrying `pmbmpd.conf` starts pmacct instead of Ze's own collector (line 775)

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc7854.md`, `rfc/short/sflow-v5.md`
  → Constraint: row RFC7854-x-10 (line 119) carries `{single-polarity: positive; Peer Down is always written with a reason byte ...}`. Once both polarities of the cause mapping are tested that annotation is untrue; correcting the row text is a row edit, owned by the verdict-fix spec (see Doc checklist row 9)
  → Constraint: RFC7854-4.9-1 (line 122) is one row for reasons 1, 2 and 3; its tagged units must prove the reason 2 event code, not only its length
- [ ] `rfc/short/rfc4271.md` - Section 8.1 event numbering (the FSM event code Section 4.9 reason 2 carries)
  → Constraint: Ze's `fsm.Event` is an iota (`EventManualStart` is 0, `internal/component/bgp/fsm/state.go`), so the wire value is never the Go value; an RFC number per event must be declared once beside `eventNames`
- [ ] `rfc/short/rfc9069.md` - Section 5.3 reason 6 on the Loc-RIB Peer Down
  → Constraint: untouched by this spec; the Loc-RIB path (`bmp_locrib.go`, `BMPPlugin.Stop`) is not routed through `peerDownFor`

### RFC text read (verbatim, `rfc/full/`)

| Source | Section | Sentence |
|--------|---------|----------|
| RFC 7854 | 4.9 | "Reason indicates why the session was closed." |
| RFC 7854 | 4.9 | "Reason 1: The local system closed the session.  Following the Reason is a BGP PDU containing a BGP NOTIFICATION message that would have been sent to the peer." |
| RFC 7854 | 4.9 | "Reason 2: The local system closed the session.  No notification message was sent.  Following the reason code is a 2-byte field containing the code corresponding to the Finite State Machine (FSM) Event that caused the system to close the session (see Section 8.1 of [RFC4271]).  Two bytes both set to 0 are used to indicate that no relevant Event code is defined." |
| RFC 7854 | 4.9 | "Reason 3: The remote system closed the session with a notification message.  Following the Reason is a BGP PDU containing the BGP NOTIFICATION message as received from the peer." |
| RFC 7854 | 4.9 | "Reason 4: The remote system closed the session without a notification message.  This includes any unexpected termination of the transport session, so in some cases both the local and remote systems might consider this to apply." |
| RFC 7854 | 4.9 | "Reason 5: Information for this peer will no longer be sent to the monitoring station for configuration reasons." |
| RFC 7854 | 4.9 (figure) | "Data (present if Reason = 1, 2 or 3)" |
| RFC 7854 | 3.3 | "If any BGP peers for which Peer Up messages were sent transition out of the Established state, corresponding Peer Down messages are sent." |
| RFC 7854 | 10.8 | "Type = 4: Remote system closed, no data" |
| RFC 4271 | 8.1.2 | Event 2 ManualStop: "Local system administrator manually stops the peer connection." |
| RFC 4271 | 8.1.2 | Event 8 AutomaticStop: "Local system automatically stops the BGP connection." |
| RFC 4271 | 8.1.4 | Event 18 TcpConnectionFails: "Event indicating that the local system has received a TCP connection failure notice." |
| RFC 9069 | 5.3 | "The Peer Down notification MUST use reason code 6." |
| sFlow v5 | 5, `flow_sample` | "unsigned int sample_pool;      /* Total number of packets that could have been sampled (i.e. packets skipped by sampling process + total number of samples) */" |
| sFlow v5 | 5, `flow_sample` | on `sequence_number`: "Note: If the agent resets the sample_pool then it must also reset the sequence_number." |

**Key insights:** (minimal context to resume after compaction)
- Every Established exit in `fsm.handleEstablished` goes to Idle, so `peer_run.go` always hands `announceClosed` the text `connection lost`; the text carries no cause.
- The only cause signal the BMP plugin has today is its own NOTIFICATION cache, filled from `notification` events the binding may or may not grant.
- Ze fires FSM Event 18 for its own local closes too (GR-aware congestion teardown, context cancel, write retirement), and the read goroutine's Event 18 can reach the FSM before the teardown's own ManualStop when the NOTIFICATION write fails. The event the FSM processed is therefore not the cause; the cause must be recorded where the close is decided, first writer wins, before the connection is closed.
- `Session.setCloseReason` (`session_connection.go`) is already first-writer-wins and already called before `closeConn` at every local site; the cause record rides beside it.
- `plugin.PeerInfo.InitialReplay` is the precedent for a per-event value carried on PeerInfo and copied into `rpc.StructuredEvent` by `getStructuredStateEvent` (`internal/component/bgp/server/events.go`).
- sFlow: `sample_pool` is `min(seq*rate, MaxUint32)`; rate is 1..1000000 by YANG, so the wrap is a unit-test property.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/bmp/bmp_events.go` - `peerDownCause`, `fsmEventNone`, `clearPeerState`, `peerDownFor`, `handleSenderState`, `cacheNotificationPDU` (read with the uncommitted Section 3.3 diff applied)
  → Constraint: `peerDownFor(notify, reason string)` decides: cached sent NOTIFICATION gives 1, cached received gives 3, `rpc.ReasonPeerRemoved` gives 5, everything else gives 2 with the constant `fsmEventNone` (00 00). Reason 4 (`PeerDownRemoteNoData`, `msg.go`) is declared and never produced
  → Constraint: the in-flight change adds `peerDownCause.reported` (true only when `peerUps` held the peer) and the down arm of `handleSenderState` returns early when it is false. Keep it: the cause is decided only for a reported peer
  → Constraint: `clearPeerState` reads and deletes `notifyCache` in one lock; the reason must still be chosen from that same read (the comment explains why the two jobs are one function)
- [ ] `internal/component/bgp/plugins/bmp/bmp.go` - subscriptions (line 500): state, update, open, notification, keepalive, refresh subscribed unconditionally
  → Constraint: delivery is still narrowed per peer by the binding's receive grant (`ProcessBinding.Receive`, `peer_settings.go`); `notification` is its own grant, so a binding can feed state and open without NOTIFICATIONs
- [ ] `pkg/plugin/rpc/bridge.go` - `StructuredEvent` (line 1029), `PutStructuredEvent` (line 1077)
  → Constraint: state events carry `State` and `Reason string` only. Every new field is zeroed in `PutStructuredEvent` (pooled struct)
- [ ] `pkg/plugin/rpc/enums.go` - `SessionState`, `ReasonPeerRemoved`, `ReasonAttached`
  → Decision: the close-origin enum lives here beside `SessionState`, same style (uint8, zero is Unspecified)
- [ ] `internal/component/bgp/server/events.go` - `getStructuredStateEvent` (line 106), `deliverPeerState`
  → Constraint: structured consumers get the StructuredEvent; text/JSON consumers get `format.AppendStateChange`, unchanged under Q-6 (a)
- [ ] `internal/component/bgp/reactor/reactor_api.go` - `apiStateObserver.OnPeerClosed` (line 101) builds `plugin.PeerInfo` and calls `OnPeerStateChange(..., SessionStateDown, reason)`
  → Decision: the close cause is set on that PeerInfo, the `InitialReplay` precedent
- [ ] `internal/component/bgp/reactor/peer_run.go` - FSM callback (line 465), from-Established arm (line 543): reason `session closed`, `connection lost` when `to == StateIdle`; `announceClosed(reactor, session, reason)`
  → Constraint: the callback is `func(from, to fsm.State)`; it has the session, so it can read the session's recorded cause without changing the FSM callback signature
- [ ] `internal/component/bgp/fsm/fsm.go` - `handleEstablished` (line 1138): ManualStop, AutomaticStop, OpenCollisionDump, HoldTimerExpires, NotifMsg/NotifMsgVerErr, UpdateMsgErr, BGPHeaderErr, TCPConnectionFails, BfdDown all `change(StateIdle)`
  → Constraint: every Established exit lands in Idle, so the `session closed` text arm is dead on this path (Q-6)
- [ ] `internal/component/bgp/fsm/state.go` - `Event` iota enum (line 94), `eventNames` (line 319)
  → Decision: the RFC 4271 number joins the name in one per-event table, so a new event cannot get a name without a number
- [ ] `internal/component/bgp/reactor/session_connection.go` - `CloseWithNotification` (OpenCollisionDump, line 440), `Teardown` (ManualStop), `teardownAutomatic` (AutomaticStop), `teardown` (line 513: seal, `setCloseReason(ErrTeardown)`, send Cease, `closeConn`, FSM stop event), `setCloseReason` (line 609, CompareAndSwap from nil)
- [ ] `internal/component/bgp/reactor/session.go` - hold timer expiry (line 684 `setCloseReason(ErrHoldTimerExpired)` before the NOTIFICATION), cancel goroutine (line 1160 `setCloseReason(reason)` then `closeConn`), `Stop` (ManualStop with no NOTIFICATION, line 1032; no production caller found by grep, tests only)
- [ ] `internal/component/bgp/reactor/session_read.go` - `handleConnectionClose` (Event 18 on EOF or reset), policy teardown (line 421 `ErrPolicyTeardown`, NOTIFICATION, Event 28 UpdateMsgErr, then close)
- [ ] `internal/component/bgp/reactor/session_write.go` - `sendNotificationWithin` (the sent-NOTIFICATION structured event exists only after a successful write; `onNotifSent` fires with `delivered=false` otherwise), `retireWrite` (line 808, `setCloseReason(err)` with the I/O error)
- [ ] `internal/component/bgp/reactor/session_bfd_strict.go` - `bfdTeardown`: Established BfdDown sends Cease BFD Down
- [ ] `internal/component/bgp/reactor/forward_pool_congestion.go` - GR-aware branch: `ErrCongestionTeardown`, `closeConn`, Event 18, no NOTIFICATION; non-GR branch `teardownAutomatic` with Cease Out of Resources
- [ ] `internal/plugins/flowexport/sflow/flow_adapter.go` - `EncodeFlowSample`: per-ifIndex `seqNums`, pool `min(uint64(seq)*rate, MaxUint32)`
  → Constraint: the encoder holds per-source state in a map keyed by ifIndex; a per-source pool accumulator fits the same map shape
- [ ] `internal/plugins/flowexport/sflow/sflow_v5_flow_adapter_test.go` - `TestSFlowFlowSamplePoolTracksTotal`, tagged SFLOW-V5-x-11, subtest "saturates rather than wraps"

**Close paths out of Established, as the code stands (2026-10-13):**

| Close path | Producer | NOTIFICATION on the wire | Event the FSM processes | Close reason recorded | BMP reason today | RFC 7854 4.9 reason owed |
|------------|----------|--------------------------|-------------------------|-----------------------|------------------|--------------------------|
| Operator teardown | `Session.Teardown` | Cease, if the write succeeds | ManualStop (2), or 18 if the read loop wins | `ErrTeardown` | 1 (cache) | 1; if the write failed, 2 with 00 02 (Q-3) |
| Automatic stop (non-strict BFD, congestion non-GR) | `teardownAutomatic` | Cease | AutomaticStop (8) | `ErrTeardown` | 1 | 1; write failed: 2 with 00 08 |
| Hold timer expiry | `session.go` hold arm | Hold Timer Expired | HoldTimer_Expires (10) | `ErrHoldTimerExpired` | 1 | 1; write failed: 2 with 00 0A |
| Collision dump | `CloseWithNotification` | Cease | OpenCollisionDump (23) | `ErrCollisionDump` | 1 | 1; write failed: 2 with 00 17 |
| Policy teardown | `session_read.go` line 421 | the filter's code | UpdateMsgErr (28) | `ErrPolicyTeardown` | 1 | 1; write failed: 2 with 00 1C |
| Strict BFD Down | `bfdTeardown` | Cease BFD Down | BfdDown (draft 31) | `ErrTeardown` via teardown | 1 | 1; write failed: 2 with Q-4 |
| Peer sends NOTIFICATION | `handleNotification` | received | NotifMsg (25) / NotifMsgVerErr (24) | none | 3 if the binding grants `notification`, else 2 | 3 (Q-2 when no PDU) |
| Peer closes TCP or resets | `handleConnectionClose` | none | TcpConnectionFails (18) | none | 2 with 00 00 | 4 |
| Our write fails (peer gone) | `retireWrite` | none | 18 via the read loop | the I/O error | 2 with 00 00 | 4 |
| GR-aware congestion drop | `congestionTeardownPeer` | none | 18 | `ErrCongestionTeardown` | 2 with 00 00 | 2 with 00 00 (Q-5) |
| Context cancel, reactor stop, `StopForRestart` | `Run` cancel goroutine | none | 18 via the read loop | `ctx.Err()` or the errChan value | 2 with 00 00 | 2 with 00 00 (Q-5) |
| Peer removed from config | `removePeer` (`reactor_peers.go` line 287) | Cease Peer De-configured | n/a, direct event | n/a | 5 | 5 |

**Behavior to change:** D1, the reason column (2 with 00 00 for a remote close, 2 with 00 00 where the event is known) becomes the owed column. D2, the saturation in `EncodeFlowSample`.

**Behavior to preserve:**

| Behaviour | Where |
|-----------|-------|
| No Peer Down for a peer never reported up (Section 3.3) | `peerDownCause.reported`, `handleSenderState` down arm |
| Reason 1 and 3 carry the exact NOTIFICATION PDU from the cache; the cache dies with the session | `cacheNotificationPDU`, `clearPeerState`, `TestPeerDownDropsTheNotificationWithTheSession` |
| Reason 5 for `rpc.ReasonPeerRemoved`, with no Data | `peerDownFor`, `TestRFC7854PeerDownOmitsDataWhereTheReasonHasNone` |
| Reason 6 on the Loc-RIB Peer Down | `bmp_locrib.go`, `bmp-locrib-pmacct` |
| The textual `Reason` string on state events and JSON | `peer_run.go`, `format.AppendStateChange` (Q-6) |
| sFlow `sample_pool` grows by the rate per exported sample below 2^32 | `TestSFlowFlowSamplePoolTracksTotal` "grows by rate per sample" |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- D1: a BGP session leaving Established, by any row of the close-path table above
- D2: a packet sampled by the kernel and handed to `FlowEncoder.EncodeFlowSample`

### Transformation Path (D1, proposed)
1. The site that decides a close records the session's close cause before it closes the connection: origin `local` plus the RFC 4271 event it is about to process (ManualStop, AutomaticStop, HoldTimer_Expires, OpenCollisionDump, UpdateMsgErr, BfdDown), or origin `local` with no event (congestion GR drop, context cancel, stop). `handleNotification` records origin `remote` with the NOTIFICATION event (24 or 25) before it fires that event. First writer wins, like `setCloseReason`.
2. A session whose connection ends with no cause recorded (read EOF or reset, write I/O failure) is origin `remote`, no NOTIFICATION: the transport ended under it.
3. The from-Established arm of the FSM callback in `peer_run.go` reads the session's cause and passes it to `announceClosed`, which hands it through `notifyPeerClosed` to `apiStateObserver.OnPeerClosed`, which sets it on `plugin.PeerInfo`.
4. `getStructuredStateEvent` copies it onto two new `rpc.StructuredEvent` fields: the close origin (enum, zero Unspecified) and the RFC 4271 Section 8.1 event number (uint16, zero meaning none). `PutStructuredEvent` zeroes both.
5. `clearPeerState` hands `peerDownFor` the cached NOTIFICATION and the event. The decision order: cached NOTIFICATION sent gives 1 with the PDU; cached received gives 3 with the PDU; `ReasonPeerRemoved` gives 5; origin local gives 2 with the event number as two octets, network order (00 00 when none); origin remote gives 4 with no Data; origin Unspecified on a non-removed down is a producer defect: WARN naming the peer, and reason 2 with 00 00.
6. `handleSenderState` writes the Peer Down only when `reported` holds (unchanged).

### Transformation Path (D2, proposed)
1. `FlowEncoder` keeps a per-ifIndex pool beside `seqNums`, created and reset with it.
2. Each sample adds its own rate to that pool in 32-bit unsigned arithmetic, so the value wraps modulo 2^32, and writes it as `sample_pool`. (If the owner picks sequence times rate instead, the product is taken modulo 2^32.)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Session ↔ Peer (reactor) | session close cause read by the FSM callback in `peer_run.go` | Yes, callback has the session (`peer_run.go` line 465) |
| Reactor ↔ server event dispatch | `plugin.PeerInfo` field, set in `apiStateObserver.OnPeerClosed` | Yes, `InitialReplay` travels this way |
| Engine ↔ BMP plugin | `rpc.StructuredEvent` new typed fields, DirectBridge only | Yes, `getStructuredStateEvent` |
| Engine ↔ text/JSON plugins | none: the JSON state event keeps `reason` only (Q-6 a) | n/a |

### Integration Points
- `Session.setCloseReason` and its callers, which the cause record rides beside
- `fsm.Event`'s per-event table, which gains the RFC 4271 number
- `rpc.StructuredEvent` and `PutStructuredEvent`
- `peerDownFor`, `clearPeerState`

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | the cause takes the existing state-event path, PeerInfo to StructuredEvent |
| No unintended coupling (components stay isolated) | Yes | BMP reads typed rpc fields; it imports nothing from the reactor or fsm |
| No duplicated functionality (extends existing, does not recreate) | Yes | the record sits beside `setCloseReason`; the NOTIFICATION cache stays the PDU source (approach A) |
| Zero-copy preserved where applicable (refs, not copies) | Yes | two scalar fields; the Peer Down Data for reason 2 is two octets written into the message buffer |
| Registration over hardcoding, outbound | Yes | no new command, family, or handler |
| Registration over hardcoding, inbound | Yes | the RFC number is declared in the one per-event table in `fsm/state.go` that `String()` already reads; no switch elsewhere learns event names. Searched: `eventNames`, `bfdEventFor` (`peer_bfd.go`), `notificationEvent` (`session_handlers.go`) |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|------------|---------------------------------|----------|--------------|--------|
| A-1 | Every local close site calls `setCloseReason` before `closeConn`, so a cause recorded beside it is visible before the read loop's Event 18 reaches the FSM callback | `teardown`, `CloseWithNotification`, hold expiry, policy teardown, congestion GR branch, cancel goroutine all read in this research | A local close reports reason 4 | a unit test per site asserting the recorded cause, plus a race test closing via teardown while the read loop runs | unvalidated |
| A-2 | When a NOTIFICATION write fails, the read goroutine's Event 18 can reach the FSM before the teardown's own stop event | `teardown` order: write, `closeConn`, then `logFSMEvent(stopEvent)`; `retireWrite` closes the conn | if it never races, the FSM callback's event would also do, but the recorded cause is still correct | reading; the design does not depend on which event wins | unvalidated |
| A-3 | `apiStateObserver.OnPeerClosed` is the only producer of the down event the BMP plugin turns into reason 1 to 4 | `session_handlers.go` line 113 comment: "notifyPeerClosed is the sole producer of the SessionStateDown"; detach/removal produce `ReasonPeerRemoved` | a second producer would arrive with origin Unspecified | grep for `OnPeerStateChange(` and `OnPeerUnbound(` callers, and the WARN arm's test | unvalidated |
| A-4 | The BMP sender only ever runs in-process (DirectBridge), so the new fields need no JSON form | `OnStructuredEvent` is fed by DirectBridge (`bridge.go` doc); `register.go` has a CLIHandler, so an external run is possible but receives no structured events today | an external BMP sender would never see the cause | check whether `bgp-bmp` can be configured external; if it can, journal row | unvalidated |
| A-5 | pmacct prints `reason_type` 1, 3 and 4 the way it prints 6 | `pmacctPeerDownReasonSix` in `names.go` is `"reason_type": 6` | the interop needles change | the first scenario run | unvalidated |
| A-6 | FRR's bgpd can be killed inside the lab container (`opExec`) so the TCP session ends without a NOTIFICATION | `check_rfc.go` line 73 runs `vtysh` through `Lab.Exec`; `opExec` exists (`checkers.go` isis scenarios) | reason 4 cannot be provoked in interop; use an `iptables` drop plus hold expiry on FRR's side instead | first scenario run | unvalidated |
| A-7 | The `.ci` harness can make its emulated peer close TCP without a NOTIFICATION | not checked | the functional test falls back to the interop scenario as the user-path proof | read `internal/test/` peer emulator options in the implement phase | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|-----------------------|
| R-1 | A local close site is missed, so it reports reason 4 (remote) | the per-site unit table has a row with no recorded cause | the cause table test walks every `setCloseReason` caller found by grep; a new caller without a cause fails it |
| R-2 | The pooled `StructuredEvent` leaks a cause from one event to the next | a reason 2 event code on an up event or on a peer-removed down | `PutStructuredEvent` zeroes both fields; a test gets, sets, puts, gets again and asserts zero |
| R-3 | The two downs for a removed peer (`removePeer`'s `removed` and the FSM's own down) arrive in either order | a removed peer reported as reason 1 or 4 instead of 5 | unchanged by this spec: the first down clears `peerUps`, the second finds `reported` false and sends nothing. Pinned by the existing reason 5 test |
| R-4 | The RFC 4271 number table drifts from the event enum | a new event added with a name and no number | the number lives in the same table entry as the name; a test asserts every declared event has both |
| R-5 | Origin Unspecified is reached in production and silently reads as reason 2 | WARN `bmp: peer down with no close cause` in a log | WARN is the "say so" the principles require; a test pins it; a functional run greps for its absence |
| R-6 | Q-2 refusal breaks an existing config that feeds BMP state without `notification` | `ze config validate` on the test configs and `test/parse/bmp-config.ci` | the refusal is an owner decision (Q-2); if refused, the WARN fallback of option (a) is the alternative |
| R-7 | D2 wrap changes what an existing collector dashboard sees above 2^32 | none in tests; operators with saturated pools see the counter restart | the wrap is the owner's call (D2 decision); the doc page states it |
| R-8 | The in-flight Section 3.3 change is reworked before landing | `git diff` of `bmp_events.go` differs from what this spec read | this spec depends on it (metadata `Depends`); re-read at implement start |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A BMP collector misattributes why a session closed (local vs remote), or misdecodes reason 2 Data; no BGP session behaviour changes. D2: an sFlow collector's pool delta |
| How is it reverted? | single commit revert per defect |
| Who else touches this path? | the in-flight BMP Section 3.3 change (`bmp_events.go`, its tests); `spec-rfc-verdict-fix-services` (SFLOW-V5-x-11 row text); `spec-rfc-verdict-fix-bgp` (moved the BMP verdicts here) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| peer drops TCP (FRR bgpd killed) | → | `handleConnectionClose` → cause remote → `peerDownFor` reason 4 | interop `bmp-peerdown-reason-pmacct`, needle `"reason_type": 4` |
| peer sends NOTIFICATION (FRR `neighbor shutdown`) | → | `handleNotification` → cache → reason 3 | interop `bmp-peerdown-reason-pmacct`, needle `"reason_type": 3` |
| operator tears the peer down (`ze` peer teardown command) | → | `Session.Teardown` → cache → reason 1 | interop `bmp-peerdown-reason-pmacct`, needle `"reason_type": 1` |
| session close inside the reactor | → | cause recorded → `PeerInfo` → `StructuredEvent` | `TestPeerClosedCarriesTheSessionCloseCause` (reactor) and `TestStructuredStateEventCarriesTheCloseCause` (server) |
| down state event at the BMP plugin | → | `handleStructuredEvent` → `clearPeerState` → `peerDownFor` → wire | `TestRFC7854PeerDownReasonFollowsTheCloseCause` (bmp, reads the decoded Peer Down off the collector pipe) |
| sampled packet | → | `EncodeFlowSample` → on-wire `sample_pool` | `TestSFlowFlowSamplePoolTracksTotal` (decoded datagram) and interop `sflow-sample-pool-sflowtool` (Q-8) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | monitored peer closes TCP with no NOTIFICATION | Peer Down reason 4 |
| AC-2 | sample pool crossing 2^32 | wraps modulo 2^32 |
| AC-3 | the `weak` verdict of SFLOW-V5-x-11, after this spec's D2 producer fix | the verdict reaches `enforced`: a tagged test proves the quoted sentence, and an agent that did not write that test re-judges it with `./le rfc audit-stamp ... mode rejudge`. The row correction stays with `plan/pre-release/spec-rfc-verdict-fix-services.md`, whose "Blocked by" table moved the verdict here (parent P-3, 2026-09-28) |
| AC-4 | the `weak` verdict of RFC7854-x-10 ("Reason indicates why the session was closed", Section 4.9), after this spec's D1 producer fix | the verdict reaches `enforced`: tagged tests tie each observed close cause to its reason in `peerDownFor` (a TCP loss or a peer close without NOTIFICATION gives reason 4, a local close without NOTIFICATION reason 2, a sent or received NOTIFICATION reason 1 or 3, a deconfigured peer reason 5), both polarities, re-judged by an agent that did not write them with `./le rfc audit-stamp ... mode rejudge`. `plan/pre-release/spec-rfc-verdict-fix-bgp.md` moved the verdict here (parent P-3, 2026-09-29) |
| AC-5 | the `weak` verdict of RFC7854-4.9-1 (Section 4.9, reason 2: "Following the reason code is a 2-byte field containing the code corresponding to the Finite State Machine (FSM) Event that caused the system to close the session"), after this spec's D1 producer fix | a local close without NOTIFICATION reaches the collector as reason 2 whose 2-octet data is the RFC 4271 Section 8.1 number of the FSM event that closed the session (ManualStop gives 00 02, AutomaticStop 00 08), and 00 00 only where no relevant event is defined. Today `peerDownFor` writes the constant `fsmEventNone` for every reason 2, because `rpc.StructuredEvent` carries only a Reason string, so the event must cross the reactor and the plugin boundary. Tagged tests cover both polarities and are re-judged by an agent that did not write them with `./le rfc audit-stamp ... mode rejudge`. `plan/pre-release/spec-rfc-verdict-fix-bgp.md` moved the verdict here (parent ruling R13, 2026-09-29) |
| AC-6 | each local close path, NOTIFICATION write refused (test conn that fails writes): Teardown, teardownAutomatic, hold expiry, collision dump, policy teardown | the collector reads reason 2 with Data 00 02, 00 08, 00 0A, 00 17, 00 1C respectively (Q-3 a); never reason 1 with no PDU, never reason 4 |
| AC-7 | the same paths with the write accepted | reason 1 carrying the exact NOTIFICATION PDU sent (unchanged) |
| AC-8 | peer closes TCP (EOF) or resets, no NOTIFICATION, no local cause recorded | reason 4, message ends after the reason byte (no Data); also for a write that fails with an I/O error |
| AC-9 | GR-aware congestion drop, context cancel, reactor stop | reason 2 with 00 00 (Q-5 a), not reason 4 |
| AC-10 | peer sends a NOTIFICATION, binding grants `notification` | reason 3 carrying the received PDU (unchanged) |
| AC-11 | Q-2 (b): a binding that delivers peer state to `bgp-bmp` without `notification` | commit refused; the message names the peer, the binding and the missing grant (`ai/rules/user-facing-errors.md`) |
| AC-12 | a down state event with origin Unspecified and a reason other than `removed` | WARN log naming the peer and that no close cause arrived; reason 2 with 00 00 |
| AC-13 | peer removed from config | reason 5, no Data (unchanged) |
| AC-14 | a peer never reported up, any close cause | no Peer Down (the Section 3.3 change, unchanged) |
| AC-15 | a recycled `StructuredEvent` after `PutStructuredEvent` | close origin Unspecified, event 0 |
| AC-16 | every `fsm.Event` constant | has a name and an RFC 4271 Section 8.1 number (or the draft number / none per Q-4) in the one table; ManualStop 2, AutomaticStop 8, HoldTimerExpires 10, TCPConnectionFails 18, OpenCollisionDump 23, NotifMsgVerErr 24, NotifMsg 25, UpdateMsgErr 28 |
| AC-17 | sFlow, rate 1024, three samples | `sample_pool` 1024, 2048, 3072 (unchanged) |
| AC-18 | sFlow, pool crossing 2^32 (rate MaxUint32, two samples) | second `sample_pool` is (2 x (2^32 - 1)) mod 2^32 = 2^32 - 2, not MaxUint32; with per-source accumulation the per-sample delta equals the rate modulo 2^32 |
| AC-19 | sFlow, two sources (ifIndex 7 and 9) | each source's pool advances independently and resets only with its own sequence number |

## End-to-End User Stories

| Story | Path | Proof |
|-------|------|-------|
| An operator watching pmacct sees a peer that crashed reported as "remote system closed, no data" | FRR bgpd killed → ze read EOF → Peer Down reason 4 → pmacct | `bmp-peerdown-reason-pmacct` |
| The same operator sees an administrative shutdown on the peer as reason 3 with the Cease, and ze's own teardown as reason 1 | FRR `neighbor shutdown` / ze teardown command | `bmp-peerdown-reason-pmacct` |
| An operator whose ze dropped a peer locally with no NOTIFICATION reads reason 2 with the FSM event (or 00 00) | unit, the paths are not provokable from a peer | AC-6, AC-9 units |
| An sFlow collector computing per-interval pool deltas keeps a positive delta across 2^32 | encoder unit, decoded datagram | AC-18 |

## 🧪 TDD Test Plan

Failing first: every new or changed assertion below is run red against the current producer and the red is pasted in the session state before the producer changes.

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEventRFCNumbers` (new) | `internal/component/bgp/fsm/state_test.go` | AC-16, every declared event has name and number | |
| `TestSessionCloseCauseLocalSites` (new, table) | `internal/component/bgp/reactor/session_close_cause_test.go` | AC-6, AC-9, A-1: each local site records origin local and its event before `closeConn` | |
| `TestSessionCloseCauseRemote` (new) | same file | AC-8: EOF, reset, write I/O failure record nothing local, read as remote; a received NOTIFICATION records remote with 24/25 | |
| `TestSessionCloseCauseFirstWriterWins` (new) | same file | a teardown racing the read loop's Event 18 keeps the teardown's cause | |
| `TestPeerClosedCarriesTheSessionCloseCause` (new, wiring) | `internal/component/bgp/reactor/reactor_api_test.go` | PeerInfo on the down event carries the cause | |
| `TestStructuredStateEventCarriesTheCloseCause` (new, wiring) | `internal/component/bgp/server/events_test.go` | `getStructuredStateEvent` copies it | |
| `TestPutStructuredEventClearsTheCloseCause` (new) | `pkg/plugin/rpc/bridge_test.go` | AC-15 | |
| `TestRFC7854PeerDownReasonFollowsTheCloseCause` (new, tag RFC7854-x-10 both polarities) | `internal/component/bgp/plugins/bmp/rfc7854_peerdown_cause_test.go` | AC-1, AC-4, AC-8, AC-9, AC-10, AC-13: cause in, decoded Peer Down reason out; negative: a remote close is never reason 2, a local close never reason 4 | |
| `TestRFC7854PeerDownReasonTwoCarriesTheEventThatClosed` (new, tag RFC7854-4.9-1 positive) | same file | AC-5, AC-6: 00 02, 00 08, 00 0A, 00 17, 00 1C, and 00 00 for no event | |
| `TestPeerDownWithNoCloseCauseSaysSo` (new) | same file | AC-12 WARN | |
| `TestBMPBindingWithoutNotificationIsRefused` (new, if Q-2 b) | the binding validator's package (implement phase locates it) | AC-11 | |
| `TestPeerDownReasonMapping` (untagged, CHANGED) | `event_test.go` | row "no notification, connection lost" becomes origin remote, reason 4, Data empty; rows gain the cause argument | |
| `TestPeerDownDropsTheNotificationWithTheSession` (untagged, CHANGED) | `rfc7854_peerdown_data_test.go` | second down carries origin local, ManualStop: reason 2 with 00 02 (was 00 00) | |
| `TestSFlowSamplePoolPerSource` (new, untagged) | `sflow_v5_flow_adapter_test.go` | AC-19 | |

### RFC-tagged tests this spec changes (owner approval required, Q-7)

| Unit | Tag | Change | Why |
|------|-----|--------|-----|
| `internal/component/bgp/plugins/bmp/event_test.go::TestHandleSenderStatePeerDown` | RFC7854-x-10 positive | the down event gains origin local and event ManualStop; the assertion tightens from "2 octets of Data" to Data equal 00 02 (under Q-3 (a); under Q-3 (b) the scenario carries the built Cease and asserts reason 1 with that PDU). The tag prose is NARROWED to what the body drives: today it also claims "a teardown ze closed by NOTIFICATION is reason 1 carrying that PDU", which this body never drives (the untagged sibling does); that clause leaves this tag and is carried by `TestRFC7854PeerDownReasonFollowsTheCloseCause` | today it drives Reason `notification` with no cause and asserts only the Data length, which cannot fail on a wrong event. Review verdict OVER-CLAIM plus NEEDS-OWNER (Q-3), confirmed against the tag comment above `TestHandleSenderStatePeerDown` |
| `internal/component/bgp/plugins/bmp/rfc7854_sender_test.go::TestBMPSenderPeerDown` | RFC7854-x-10 positive | no body change; the tag stays with its narrow claim ("WritePeerDown emits the reason byte"), and the rejudge reads it beside the new cause test | it proves the writer round-trips the reason byte, which is part of x-10, not the cause: it is no evidence of "why the session was closed", so the x-10 weight sits on `TestRFC7854PeerDownReasonFollowsTheCloseCause` |
| `internal/component/bgp/plugins/bmp/rfc7854_peerdown_data_test.go::TestRFC7854PeerDownCarriesTheDataItsReasonRequires` | RFC7854-4.9-1 positive | subtest "reason 2 carries the two-byte event code" drives a local close with ManualStop instead of `connection lost` and asserts 00 02 (RFC 4271 Section 8.1.2 Event 2); subtests 1 and 3 set the cause their scenario implies (local / remote). Depends on Q-3: in production a ManualStop with no cached PDU arises only when the Cease write failed, which Reason 1's "would have been sent" arguably covers; under Q-3 (b) the reason 2 subtest drives a close that builds no NOTIFICATION at all (a Q-5 cause) | the audit note: the subtest "drives 'connection lost', an unexpected transport termination that Section 4.9 assigns to reason 4" |
| `internal/component/bgp/plugins/bmp/rfc7854_peerdown_data_test.go::TestRFC7854PeerDownOmitsDataWhereTheReasonHasNone` | RFC7854-4.9-1 negative | gains a reason 4 arm asserting no Data after the reason byte; the tag prose names reason 4 and cites the figure and Section 10.8 "Type = 4: Remote system closed, no data", because the 4.9-1 row text itself quotes reasons 1-3 only | the figure's "Data (present if Reason = 1, 2 or 3)" binds reason 4 too |
| `internal/plugins/flowexport/sflow/sflow_v5_flow_adapter_test.go::TestSFlowFlowSamplePoolTracksTotal` | SFLOW-V5-x-11 positive | subtest "saturates rather than wraps" inverts to "wraps modulo 2^32" (AC-18). The tag prose is rewritten: it drops "tracks the total packets seen" (an over-claim before and after, because the encoder writes an estimate, not Section 5's "Total number of packets that could have been sampled"), states the estimate as Ze's D2 choice and not as sFlow text, and cites sFlow v5 Section 5 for the wrap: "Use the maximum counter value to indicate that the counter is not available." and "An available counter may temporarily have the max value just before it rolls to zero. This is permitted." | the prose today claims saturation, which the text reads as "not available". Unchanged siblings tagged SFLOW-V5-x-28 (in `sflow_v5_test.go` and `sflow_v5_sequence_test.go`) assert pool == k*rate at constant rate and stay true under per-source accumulation; the rewritten prose must not call a wrap a "reset" in the x-28 sense |

Every changed tag owes a fresh `./le rfc discriminate-record` and a rejudge by an agent that did not write it (`./le rfc audit-stamp ... mode rejudge`), per AC-3, AC-4, AC-5.

#### Owner approval commands

Approved by the owner on 2026-10-13 (Q-7). The gate reads approvals from the committing session's own file (`tmp/commit-rfc-approved-<session>.md`), and a row is used up by the commit that carries its `RFC-approved:` trailer. So the owner runs these at the prompt of the session that implements and commits the edits, and runs a unit's command again if a later commit changes that unit a second time. A unit is `<package directory>.<top-level test function>`: the gate names a subtest by its enclosing function, and a moved file by its new directory. Every name was checked to exist at HEAD `051876c3f4`. `TestBMPSenderPeerDown` is planned with no change, so its row is used only if its unit text changes after all. Q-3 is still open: the first and third commands cover the edit for whichever Q-3 answer is recorded.

5 commands:

```
./le rfc approve unit bmp.TestHandleSenderStatePeerDown reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-bmp-sflow-export-rfc-defects, down event gains the close cause, Data assertion tightens, tag prose narrowed)'
./le rfc approve unit bmp.TestBMPSenderPeerDown reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-bmp-sflow-export-rfc-defects, tag kept narrow, run only if the unit text changes)'
./le rfc approve unit bmp.TestRFC7854PeerDownCarriesTheDataItsReasonRequires reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-bmp-sflow-export-rfc-defects, reason 2 subtest drives a local ManualStop close, subtests set their cause)'
./le rfc approve unit bmp.TestRFC7854PeerDownOmitsDataWhereTheReasonHasNone reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-bmp-sflow-export-rfc-defects, gains a reason 4 arm, tag prose names reason 4)'
./le rfc approve unit sflow.TestSFlowFlowSamplePoolTracksTotal reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-bmp-sflow-export-rfc-defects, saturate subtest inverts to wrap modulo 2^32, tag prose rewritten)'
```

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Reason 2 event code (uint16) | 0..65535, used 0..35 | 35 (draft BfdStrictConfigChanged, if Q-4 a) | n/a | n/a, unsigned |
| `sample_pool` (uint32) | wraps | 2^32 - 1, next sample wraps to (2^32 - 1 + rate) mod 2^32 | n/a | n/a |
| sFlow rate (YANG) | 1..1000000 | 1000000 | 0 (existing refusal) | 1000001 (existing refusal) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bmp-sender-peer-down-reason.ci` (new, if A-7 holds) | `test/plugin/` | emulated peer closes TCP without NOTIFICATION; the test BMP collector decodes reason 4 | |
| `bmp-config.ci` (extended, if Q-2 b) | `test/parse/` | a binding feeding BMP without `notification` is refused with the named message | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bmp-peerdown-reason-pmacct` (new) | `test/interop/scenarios/bmp-peerdown-reason-pmacct/` (frr.conf, pmbmpd.conf, ze.conf) + checks in `internal/le/interoplab/bgp/checkers.go` | FRR (BGP peer), pmacct `pmbmpd` (collector) | three teardowns in sequence, each waiting for the session to re-establish: ze teardown → `"reason_type": 1`; FRR `neighbor shutdown` → `"reason_type": 3`; FRR bgpd killed → `"reason_type": 4`, and `"reason_type": 2` absent with proof needles | |
| `sflow-sample-pool-sflowtool` (new, Q-8 a) | `test/interop/scenarios/sflow-sample-pool-sflowtool/` | sflowtool | sflowtool decodes ze's `sample_pool`, and the delta between consecutive samples of one source equals the configured rate | |

Discrimination for the interop (`ai/rules/interop-and-goal-validation.md`): revert `peerDownFor` to today's arm, rebuild the ze image, confirm the reason 4 row goes red (pmacct prints 2), restore, confirm green, record.

## Files to Modify

| File | Change |
|------|--------|
| `internal/component/bgp/fsm/state.go` | the per-event table carries the RFC 4271 number beside the name; an accessor returns it |
| `internal/component/bgp/reactor/session.go` | the session's close-cause record (typed, first writer wins); hold-expiry and cancel sites record it |
| `internal/component/bgp/reactor/session_connection.go` | `teardown` records local plus its stop event; `CloseWithNotification` records OpenCollisionDump |
| `internal/component/bgp/reactor/session_read.go` | policy teardown records UpdateMsgErr |
| `internal/component/bgp/reactor/session_handlers.go` | `handleNotification` records remote plus 24/25 before the FSM event |
| `internal/component/bgp/reactor/forward_pool_congestion.go` | GR-aware branch records local with no event |
| `internal/component/bgp/reactor/peer_run.go` | from-Established arm reads the cause and passes it to `announceClosed` |
| `internal/component/bgp/reactor/peer_derived_bindings.go` | `announceClosed` carries it |
| `internal/component/bgp/reactor/reactor_notify.go`, `reactor_api.go` | `notifyPeerClosed` / `OnPeerClosed` carry it to PeerInfo; the `mrt` observer (`internal/plugins/mrt/register.go`) follows the signature |
| `internal/component/plugin/types_bgp.go` | `PeerInfo` gains the cause |
| `pkg/plugin/rpc/enums.go`, `pkg/plugin/rpc/bridge.go` | close-origin enum; two `StructuredEvent` fields; `PutStructuredEvent` zeroes them |
| `internal/component/bgp/server/events.go` | `getStructuredStateEvent` copies them |
| `internal/component/bgp/plugins/bmp/bmp_events.go` | `peerDownFor` takes the cause; `fsmEventNone` and its comment replaced by the event's two octets (stale comment removed, `ai/rules/stale-comments.md`); RFC 7854 Section 4.9 quotes above each arm |
| the binding validator (if Q-2 b) | refusal of a BMP-feeding binding without `notification` |
| `internal/plugins/flowexport/sflow/flow_adapter.go` | per-source pool, wrapping; the saturation comment replaced |
| `internal/le/interoplab/bgp/checkers.go`, `names.go` | the new BMP scenario's ops and needles; the sFlow scenario's |
| `docs/guide/bmp.md`, `docs/architecture/behavior/peer-lifecycle.md`, `docs/architecture/testing/interop.md`, `docs/architecture/api/process-protocol.md` | see Documentation checklist |
| `rfc/audit/rfc7854.json`, `rfc/audit/sflow-v5.json`, `rfc/discrimination/*.json` | written by the `./le rfc` tools only, never by hand |

## Files to Create

| File | Purpose |
|------|---------|
| `internal/component/bgp/reactor/session_close_cause_test.go` | the per-site cause table |
| `internal/component/bgp/plugins/bmp/rfc7854_peerdown_cause_test.go` | the tagged cause-to-reason tests |
| `test/interop/scenarios/bmp-peerdown-reason-pmacct/` | BMP interop |
| `test/interop/scenarios/sflow-sample-pool-sflowtool/` | sFlow interop (Q-8 a) |
| `test/plugin/bmp-sender-peer-down-reason.ci` | if A-7 holds |

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | no config surface; Q-2 (b) is a validator, not a leaf |
| YANG validation constraints | No | none added |
| YANG custom validators | Yes if Q-2 b | the binding validator refusing BMP state without `notification` |
| CLI commands/flags | No | none |
| CLI grammar (keyword before value) | N-A | no command |
| Editor autocomplete | N-A | no YANG leaf |
| Functional test for new RPC/API | N-A | no RPC; functional test listed above for behaviour |
| Pipe completeness | N-A | no CLI output |
| Env var registration | No | none |
| Doctor check for runtime dependencies | No | no new runtime dependency |
| Prometheus counters/metrics | No | none; `show bmp` already reports the peer-down reason byte (`state.go` `peerDown`) |
| BGP family surface (new SAFI / capability / attribute) | N-A | no family |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | correctness of an existing feature |
| 2 | Config syntax changed? | No | (Q-2 b adds a refusal, documented in row 6) |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | `docs/guide/bmp.md`: a reason-by-cause table replacing "mapped reason code"; the Q-2 refusal if taken. sFlow: the flowexport guide page states the pool wraps |
| 7 | Wire format changed? | No | the BMP and sFlow formats are unchanged; values change |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/architecture/api/process-protocol.md`: the two new `StructuredEvent` fields for state events |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | the verdicts move via `./le rfc audit-stamp`; RFC7854-x-10's `{single-polarity: positive; ...}` annotation becomes untrue and is corrected (row text owned by the verdict-fix spec; flag it there) |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/interop.md`: the two new scenarios in its scenario list |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/behavior/peer-lifecycle.md`: the close cause step on leaving Established |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | run `./le spec citation anchors spec plan/immediate/spec-bmp-sflow-export-rfc-defects.md` at implement start and name each |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/bmp.md` testing walkthrough (line 431, line 437: "Close FRR's BGP session and check that Peer Down carries ...") gains the expected reason |

### Discovery (`ai/rules/repo-maintenance.md`)
| Question | Answer |
|----------|--------|
| Where does an agent look first | `ai/CODE-TO-DOCS.md` maps `bmp_events.go` to `docs/guide/bmp.md`; the reason table lives there |
| What rule prevents regression | RFC7854-x-10 and RFC7854-4.9-1 tagged units with discrimination records, re-checked by `./le rfc check` |
| What registry prevents drift | the fsm per-event table (name and number in one entry) |
| What verification proves it | the unit tests above and `bmp-peerdown-reason-pmacct` |

## Implementation Steps

1. **Phase: Wiring** -- the typed cause from session to StructuredEvent
   - Tests: `TestEventRFCNumbers`, `TestPeerClosedCarriesTheSessionCloseCause`, `TestStructuredStateEventCarriesTheCloseCause`, `TestPutStructuredEventClearsTheCloseCause`
   - Files: `fsm/state.go`, `rpc/enums.go`, `rpc/bridge.go`, `types_bgp.go`, `server/events.go`, `peer_run.go`, `peer_derived_bindings.go`, `reactor_notify.go`, `reactor_api.go`
   - Verify: red first, then the cause reaches the event
2. **Phase: Recording the cause** -- every close site
   - Tests: `TestSessionCloseCauseLocalSites`, `TestSessionCloseCauseRemote`, `TestSessionCloseCauseFirstWriterWins`
   - Files: `session.go`, `session_connection.go`, `session_read.go`, `session_handlers.go`, `forward_pool_congestion.go`
3. **Phase: BMP reason** -- after Q-7 approval for the tagged edits
   - Tests: the new tagged pair, `TestPeerDownWithNoCloseCauseSaysSo`, the changed untagged and tagged units
   - Files: `bmp_events.go`, `docs/guide/bmp.md`, `peer-lifecycle.md`, `process-protocol.md`
4. **Phase: Q-2 refusal** (if b) -- validator, `bmp-config.ci`
5. **Phase: sFlow D2** -- per-source wrapping pool; `TestSFlowFlowSamplePoolTracksTotal` changed (Q-7), `TestSFlowSamplePoolPerSource`
6. **Phase: Interop** -- `bmp-peerdown-reason-pmacct`, `sflow-sample-pool-sflowtool`, with their red-then-green discrimination
7. **Phase: verdicts** -- `./le rfc discriminate-record` per changed tag, rejudge by a different agent, `rfc/audit` updated by the tool

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | no local close reaches reason 4; no remote close reaches reason 2; reason 2 Data is the RFC number, never the Go iota |
| Naming | the enum names the origin (local / remote), not the BMP reason; BMP alone maps to reasons |
| Data flow | the reactor decides the cause; only BMP decides the reason byte |
| Rule: `ai/rules/principles.md` | origin Unspecified is never silently read as local: WARN plus test |
| Rule: `ai/rules/rfc-compliance.md` | each `peerDownFor` arm and each recording site added carries its RFC quote; callers carry the section |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| reason 4 on the wire | pmacct `"reason_type": 4` |
| reason 2 event code | unit decode of Data |
| sFlow wrap | decoded datagram unit |

### Security Review Checklist
| Check | What to look for |
|-------|------------------|
| Input validation | none new: the cause is internal; the NOTIFICATION PDU path is unchanged |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the close-path table source; if misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |

## Design Insights

- The event the FSM processed is a poor witness of the cause: ze fires Event 18 for its own closes, and the read loop's Event 18 races the teardown's stop event. The witness is the site that decided to close, which already calls `setCloseReason` first.
- BMP should stay the only place a reason byte is chosen; the reactor reports facts (origin, event), not BMP vocabulary.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Record the cause at the close site, first writer wins (approach A) | FSM callback carries the processed event; reactor carries the NOTIFICATION PDU on the down event (approach B, Q-1) | the processed event is wrong for local closes (Event 18) and races; B changes the grant contract and deletes the cache the in-flight change just reworked |
| Carry it on `PeerInfo` then `StructuredEvent` | a new parameter on every `OnPeerStateChange` signature | `InitialReplay` precedent; one field add per struct instead of five signature changes |
| RFC number in the fsm per-event table | a switch in the BMP plugin mapping event names to numbers | one declaration; BMP imports nothing from fsm |
| sFlow per-source accumulated pool | sequence times rate modulo 2^32 | correct when the rate changes for a source, and resets with the sequence number as the sFlow note requires; owner decides (D2) |

## Known Limitations

- The textual state reason stays `connection lost` for every Established exit (Q-6 a); `session closed` is dead on that path. One journal row.
- A BMP sender run as an external process receives no structured events and so no cause (A-4).

## RFC Documentation (Scope: protocol)

Add `// RFC 7854 Section 4.9: "<quoted reason text>"` above each `peerDownFor` arm, `// RFC 4271 Section 8.1.x` beside each RFC number in the event table, and the sFlow v5 `sample_pool` and `sequence_number` note above the pool arithmetic.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints

### Goal Gates (MUST pass)
- [ ] AC-1..AC-19 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated, not library-only
- [ ] Integration and Documentation checklists answered with evidence
- [ ] Architectural Verification table filled
- [ ] Every A-N confirmed or broken, none `unvalidated`

### Goal Validation
| Goal (Task) | Evidence owed |
|-------------|---------------|
| D1: a BMP station is told who closed the session | `bmp-peerdown-reason-pmacct` passing, with the recorded red when `peerDownFor` is reverted |
| D1: reason 2 carries the FSM event | `TestRFC7854PeerDownReasonTwoCarriesTheEventThatClosed` decode, discrimination record |
| D2: the pool keeps advancing past 2^32 | AC-18 decoded datagram; `sflow-sample-pool-sflowtool` for the decode by a third party |

### TDD
- [ ] Tests written
- [ ] Tests FAIL (before the fix)
- [ ] Tests PASS (after the fix)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
- [ ] **Commit A** and **Commit B** per `ai/rules/planning.md`

### Verification
- [ ] `./le verify worktree`
