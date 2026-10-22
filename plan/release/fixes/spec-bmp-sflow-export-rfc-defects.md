# Spec: bmp-sflow-export-rfc-defects

| Field | Value |
|-------|-------|
| Status | design |
| Priority | P1 |
| Release reason | Monitoring exports must report the actual session-down cause and advancing sample pool. |
| Scope | protocol |
| Depends | the RFC 7854 Section 3.3 change (Peer Down only for a peer reported up, `peerDownCause.reported`), committed as dce906c381 "fix(bmp): send Peer Down only for a peer reported up"; and, for AC-1 and AC-9 on the `Session.Run` exits that fire no FSM event, `plan/release/fixes/spec-bgp-rs-replacement-on-withdrawal.md` Phase 2b (the `runOnce` down defer fires the leave when `Run` returns with the FSM still Established). This spec records the cause on those exits and does not duplicate that fix |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-14 |

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
| D2 | sFlow v5 (`rfc/full/sflow-v5.txt`, Section 5, `flow_sample`), row SFLOW-V5-x-11. NO NORMATIVE SENTENCE: a Ze behaviour defect measured against the field definition | the definition: "unsigned int sample_pool;      /* Total number of packets that could have been sampled (i.e. packets skipped by sampling process + total number of samples) */", and the note on `sequence_number`: "If the agent resets the sample_pool then it must also reset the sequence_number." | `internal/plugins/flowexport/sflow/flow_adapter.go::EncodeFlowSample` | The pool is estimated as sequence times rate and saturates at 2^32-1. Correction (RFC-tagged test review, 2026-10-13): sFlow v5 Section 5's conventions do speak to it, "Unknown counter. Use the maximum counter value to indicate that the counter is not available." and "An available counter may temporarily have the max value just before it rolls to zero. This is permitted.", so a saturated pool announces "not available" and rolling to zero is what the text expects. The defect is that a saturated value stops being the count the definition names, and the lowercase note treats the pool as a cumulative counter reset only together with the sequence number, which saturation is not. Once saturated, a collector's per-interval delta reads zero | weak; lead, not checked against a collector. Row SFLOW-V5-x-11 itself quotes a field description tagged MUST: a row correction, listed with the other sFlow corrections in `plan/release/validation/spec-rfc-verdict-test-fix-pass.md` |

## What a fix must prove

| Obligation | Detail |
|------------|--------|
| Failing test first | D1: a peer-down after a TCP loss yields reason 4. D2: a pool past 2^32 wraps |
| Both polarities | D1: a local close without NOTIFICATION stays reason 2, a NOTIFICATION stays reason 1 or 3. D2: below the limit the value is unchanged |
| Discrimination | `./le rfc discriminate-record` for every tagged unit |
| Interop | D1 against a BMP collector (the existing BMP interop scenario); D2 against sflowtool, by extending the existing carrier with the sibling `TestFlowExportSFlowToolSamplePoolInterop` beside `TestFlowExportSFlowToolInterop` (Q-8, review r2 #6), reading the collector's decoded `samplePool` per sample, the test's delta over them, and the wrap |

## Owner decisions

| Row | Question |
|-----|----------|
| D2 | Wrap versus saturate is NOT silent (RFC-tagged test review, 2026-10-13, checked against `rfc/full/sflow-v5.txt:1887-1892`): sFlow v5 Section 5 says "Unknown counter. Use the maximum counter value to indicate that the counter is not available." and "An available counter may temporarily have the max value just before it rolls to zero. This is permitted." Saturating at 2^32-1 therefore announces "not available", and rolling to zero is what the text expects, so the wrap is the text's answer. What stays the owner's call is the estimate-versus-count question: sample_pool is defined as "Total number of packets that could have been sampled (i.e. packets skipped by sampling process + total number of samples)", and the encoder writes an estimate (sequence times rate, or the accumulated rate). Whether the pool should count packets actually seen is Ze's choice, and the tag prose states it as Ze's choice, not as sFlow text |

| OD-1 | Grant contract (owner, 2026-10-13, Q-1 "b": "state subscribers see the closing NOTIFICATION without the notification grant"). A process granted `state` receives, on the down event of a session that a NOTIFICATION closed, that NOTIFICATION's PDU and whether ze built it or received it, whether or not the binding also grants `notification`. This holds for every `state` subscriber, whatever its transport: a structured (DirectBridge) subscriber reads the new `rpc.StructuredEvent` fields, and a JSON or text subscriber reads the same facts on the down state event (`format.AppendStateChange`, `appendStateChangeJSON` in `format/text_json.go`, `appendStateChangeText` in `format/text_human.go`), added as optional members (in the text form before `reason`, so a down event's multi-word reason stays last); see "State event wire shape" under Data Flow. The change is additive: a member appears only on a down event whose close record has the fact, so every existing parser keeps working. The `notification` grant keeps its own meaning: it delivers the NOTIFICATION message events (BMP Route Mirroring, RFC 7854 Section 4.7, `bmp_events.go` mirror arm, still reads them). Pages that state the grant and change with the code, never before it (a page describing behaviour HEAD lacks is a false claim): `docs/config-reference.md` "Process Bindings" (`state` row "Peer state changes (up/down)", `notification` row), `docs/architecture/api/process-protocol.md` (state event fields, the paragraph "For state events, `StructuredEvent.State` and `StructuredEvent.Reason` carry the data"), `docs/architecture/api/text-format.md` (the `<state-event>` BNF), `docs/architecture/api/ipc_protocol.md` ("State events" paragraph, line 204), `docs/guide/bmp.md` (attach example and Peer Down text). See Documentation checklist rows 6 and 8 |
| OD-2 | Where the PDU is kept until the down event is published, and its cost (design under Q-1 "b"). One close record per `Session`, held behind an atomic pointer and set once, first writer wins, the `closeReason` shape (`Session.closeReason` in `session.go`, `Session.setCloseReason` in `session_connection.go`). The record holds the origin (local or remote) and, where a NOTIFICATION closed the session, a copy of its body. It holds no FSM event (Q-9 "iii", owner 2026-10-14): reason 2 always writes 00 00. It is written before the write or the FSM event that can publish the down event, at these producers: `Session.sendNotificationWithin` (every NOTIFICATION ze builds, before `writeMessageWithin`, so a failed write still leaves it); `Session.handleNotification` (`session_handlers.go`; the received body, copied out of the pooled read buffer before `UnpackNotification`, because the payload must be a self-contained value); `Session.handleConnectionClose` (remote, no PDU, before Event 18); `Session.retireWrite` (remote, no PDU, before the conn is closed) for a write the transport refused, with the local retirements classified at their site first (below); the `Session.Run` exits that return with no FSM event (below); the Q-5 sites (local, no PDU); and the nil-connection branches of the local closers (local, no PDU). Local retirements that reach `retireWrite`: `finishRawWrite` retires for `errRawOwnership` ("a local safety policy", its comment), `writeUpdateBody`'s deferred `retireWrite(result)` retires after a partial write that ze refused, and the forward flush past ze's own write deadline (`session_write.go`, "forward flush failed; destination session retired") retires the destination. Each records local, no PDU, before it calls `retireWrite`, so first writer wins keeps it and the collector reads reason 2 with 00 00, never reason 4. `Run` exits with no FSM event (`session_read.go` and `session_coalesce.go`): a `read body` error and a header read error that is neither EOF nor a reset record remote, no PDU, before returning (RFC 7854 Section 4.9 reason 4: "This includes any unexpected termination of the transport session"); when a local closer closed the conn first (the cancel goroutine's `closeConn`, whose 100 ms drain deadline can hand the blocked reader `i/o timeout`), that closer recorded first and the remote record loses. Publishing the down event for those exits is `spec-bgp-rs-replacement-on-withdrawal` Phase 2b (metadata Depends); this spec owns only the record. The nil-connection branches: `Session.teardown` (its `stopEvent`: ManualStop from `Teardown`, AutomaticStop from `teardownAutomatic`, BfdDown from `bfdTeardown`), the hold-expiry arm in `session.go` (HoldTimerExpires), `CloseWithNotification` (OpenCollisionDump) and `sendHoldTimerExpired` in `session_write.go` (HoldTimerExpires) each read `s.conn` and skip the NOTIFICATION when it is nil, and each still fires its FSM event after that (read at the producers 2026-10-14), so the event can publish the down event; there each records local, no PDU, before the event, so that down event never arrives Unspecified (AC-12). Because every closer that records does so before `closeConn`, that branch's record lands only when the conn was closed by a closer that records nothing. Corrected 2026-10-14 (review r2 #1): with this spec's own recorders in place, the one such production window is a race. A context cancel while the session is paused makes `waitForResume` (`session_flow.go`) return `ctx.Err()` on `<-ctx.Done()`, and `Run`'s deferred `closeConn` (`session.go`) can run before the cancel goroutine, woken by the same `ctx.Done()`, writes its local record; a `bfdTeardown` or a hold expiry landing in that gap reads a nil conn. Every other Established exit of `Run` records before its deferred `closeConn` runs (`handleConnectionClose`, the `read body` and non-EOF header exits, the `logNotifyErr` exits through `sendNotificationWithin`, which records before `writeMessageWithin` returns `ErrNotConnected` for a nil conn, and the nil-conn loop branch returning a recorded `closeReason`). The cancel goroutine's panic-recovery `closeConn` is not a production shape: nothing before its record can panic in practice, so the recovery arm runs only after the record is written. The asymmetry between nil-conn local closes is intended: `teardown`, `CloseWithNotification`, the hold-expiry arm and `sendHoldTimerExpired` skip the build and give reason 2 with 00 00, while the `logNotifyErr` sites (policy, prefix-limit and family teardowns, `fsmMessageEvent`, header errors) pass the nil conn into `sendNotificationWithin`, record the PDU they built, and give reason 1 (Q-3 "b"). Read by the from-Established arm of the FSM callback (`peer_run.go`). `fsm.change` queues callbacks in `f.pending`, and the arm can run on whichever goroutine drains them (`fsm/fsm.go`), so the arm does not run "inside" the event; it is still race-free because the record is stored before the event is enqueued under `f.mu` and never changes after, so the drainer's load sees it. Cost: one pointer word per Session while it runs; at close one allocation, the record plus the body, at most 4077 octets (`message.MaxMsgLen` 4096 minus the 19-octet header) or 65516 with RFC 8654 Extended Message (`message.ExtMsgLen`). A teardown Cease always carries `BuildCeaseData(subcode, msg)` with the subcode name as the default message (`Session.teardown`), so a typical record is a few dozen octets, not 2. A new `Session` is built per connection attempt (`runOnce`, `Peer.newSession` in `peer_bfd.go`), so the record is freed with its Session; cold path, at most one per session close. The payload crossing to plugins is a self-contained value (`ai/rules/plugins.md`): immutable bytes owned by the record, referenced by the event and cleared by `PutStructuredEvent`. BMP copies the body into its Peer Down message buffer and keeps no reference, which the StructuredEvent contract (no retention) already requires |

### Open owner questions (design, 2026-10-13)

Each row carries the reading the design proceeds under until the owner answers. Q-1 to Q-7 are answered (2026-10-13); Q-8 was re-asked on a corrected premise and answered 2026-10-14; Q-9 was answered "a" on 2026-10-14, re-asked the same day on a corrected premise (review r2 #1), and answered "iii", which supersedes "a".

| ID | Question | Options | Design proceeds under |
|----|----------|---------|-----------------------|
| Q-1 | D1 approach: where the close cause is decided | (b) the reactor carries the closing NOTIFICATION PDU itself on the down event, built or received, delivered or not, and BMP's `notifyCache` is deleted | **Answered by the owner, 2026-10-13: "b"** (given after the owner asked "Pro and Cons."). Consequences: OD-1 (the grant contract) and OD-2 (where the PDU is kept until the down event is published, and its memory cost) under "Owner decisions"; Data Flow D1, the ACs and the TDD plan are rewritten for it. BMP's `notifyCache`, `notifyPDU` and `cacheNotificationPDU` are deleted, never kept as a fallback (`ai/rules/no-layering.md`) |
| Q-2 | A remote NOTIFICATION ended the session but the BMP binding does not receive `notification`, so no PDU is cached. Reason 3 without the PDU is malformed (Section 4.9: "Following the Reason is a BGP PDU containing the BGP NOTIFICATION message as received from the peer"), and reason 4 says "without a notification message", which is false. The same gap hits a NOTIFICATION ze sent and delivered: with no `notification` grant nothing is cached, and reason 2 ("No notification message was sent") would be false | answered by Q-1 | **Answered 2026-10-13 by the owner's Q-1 answer "b"; the owner did not object to that reading.** The down event carries the closing NOTIFICATION whatever the binding grants (OD-1), so reason 1 and reason 3 always have their PDU and no grant check is needed. No commit refusal: its validator, its AC, its functional test and its risk are deleted from this spec |
| Q-3 | A local NOTIFICATION ze decided to send but whose write failed (TCP already gone). RFC 7854 Section 4.9 (`rfc/full/rfc7854.txt:863-865`): "Reason 1: The local system closed the session.  Following the Reason is a BGP PDU containing a BGP NOTIFICATION message that would have been sent to the peer." Reason 2: "The local system closed the session.  No notification message was sent." "Would have been sent" reads as covering a NOTIFICATION ze built and could not deliver, so reason 2 for this case is a reading, not the text. Three RFC-tagged units depend on the answer: `TestHandleSenderStatePeerDown` (RFC7854-x-10) and `TestRFC7854PeerDownCarriesTheDataItsReasonRequires` (RFC7854-4.9-1) both drive ManualStop with no cached NOTIFICATION, which in production is exactly this failed-write case, and AC-6 | (b) reason 1 with the PDU ze built, which Q-1 "b" carries | **Answered by the owner, 2026-10-13: "b"** (reason 1 with the built NOTIFICATION PDU when the write failed). The same reading GoBGP, BIRD and rustbgpd take. AC-6 is reason 1 with the built PDU; `TestHandleSenderStatePeerDown` (RFC7854-x-10) and `TestRFC7854PeerDownCarriesTheDataItsReasonRequires` (RFC7854-4.9-1) assert reason 1 carrying that PDU (approved under Q-7) |
| Q-4 | The draft-ietf-idr-bgp-bfd-strict-mode events 30 to 35 are not in RFC 4271 Section 8.1, which Section 4.9 points at ("see Section 8.1 of [RFC4271]"). A strict BFD Down whose Cease write failed is reason 2 with which event code | (a) 00 1F | **Answered by the owner, 2026-10-13: "a"** (event code 00 1F for a strict-BFD close with no NOTIFICATION). **Superseded by Q-9 "iii" (owner, 2026-10-14):** no event field is carried, so 00 1F is never sent. A strict-BFD close with no NOTIFICATION, reachable only in the paused-cancel race (OD-2), is reason 2 with 00 00 |
| Q-5 | Closes ze makes with no NOTIFICATION and no FSM stop event of its own: the GR-aware congestion teardown (`forward_pool_congestion.go` closes TCP and fires Event 18 to simulate a transport failure), context cancellation, `StopForRestart` | (a) reason 2 with 00 00 | **Answered by the owner, 2026-10-13: "a"** (reason 2, event 00 00). The local system closed the session, and Event 18's definition ("the local system has received a TCP connection failure notice", RFC 4271 Section 8.1.4) is not what caused it. Each of these sites records a local close with no PDU before it closes the connection (Data Flow D1). Review 2026-10-14 (#6): `Reactor.Stop` is NOT one of them. `Stop` calls `stop(true)`, which runs `notifyPeersShutdown`, then `Peer.shutdownNotify`, then `Session.Teardown(NotifyCeaseAdminShutdown, "")`, so it builds the Cease through `sendNotificationWithin` and is reason 1 with that PDU (Q-3 "b"). Only `StopForRestart` (`stop(false)`, which notifies nobody) and a plain context cancel reach the cancel goroutine with no NOTIFICATION; the local retirements of OD-2 (`finishRawWrite`, a refused partial write, the forward flush past ze's own deadline) are the same class and also 00 00 |
| Q-6 | The textual state reason (`connection lost` / `session closed`, `peer_run.go`) reaches JSON and text consumers. Every exit from Established goes to Idle, so `session closed` is never produced on that path, and `connection lost` names every cause | (a) leave the text untouched, this spec adds typed fields beside it | **Answered by the owner, 2026-10-13: "a"** (keep the textual reason, add typed fields beside it). The dead `session closed` arm has its journal row, 2026-10-13, in `plan/journal/constant-reported-as-measured-state.md` |
| Q-7 | Approval to edit the RFC-tagged tests listed in the TDD plan: `TestHandleSenderStatePeerDown` (RFC7854-x-10, body tightens and tag prose narrows to what the body drives), `TestBMPSenderPeerDown` (RFC7854-x-10, no body change, prose stays narrow), `TestRFC7854PeerDownCarriesTheDataItsReasonRequires` (RFC7854-4.9-1 positive), `TestRFC7854PeerDownOmitsDataWhereTheReasonHasNone` (RFC7854-4.9-1 negative, gains reason 4 arm and prose naming reason 4), `TestSFlowFlowSamplePoolTracksTotal` (SFLOW-V5-x-11, wrap plus prose rewrite). The first and third bodies depend on Q-3, so Q-7 is answered after Q-3 | approve / refuse | **Answered by the owner, 2026-10-13: "approve all RFC-tagged test changes".** The approval covers the five edits listed in the TDD plan. Q-3 is answered "b" (2026-10-13), so for `TestHandleSenderStatePeerDown` and `TestRFC7854PeerDownCarriesTheDataItsReasonRequires` it covers the reason 1 with the built PDU edit, which the owner approved on 2026-10-13 with the Q-3 answer. The commands are under "Owner approval commands" in the TDD plan |
| Q-8 | D2 interop. Corrected premise (review 2026-10-14, #9): an sflowtool carrier exists. `TestFlowExportSFlowToolInterop` (`internal/plugins/flowexport/interop_integration_linux_test.go`, build tags `integration && linux`) runs sflowtool v6.11, pinned in `test/interop-flowexport/Dockerfile.collectors`, drives the public encoder through `Sender` over UDP (no tc, psample or privileges), and already asserts `"samplePool": "64"` for one sample at `Rate: 64`. It is documented in `docs/architecture/flowexport/flow-export-1-counter-export.md`, "Independent collector tests". Because the test sets `FlowSample.Rate` directly (the YANG bound 1..1000000 binds config only), sflowtool can observe the wrap past 2^32 | (a) a new scenario `sflow-sample-pool-sflowtool` (the 2026-10-13 answer); (b) extend the existing carrier with the per-source delta and the wrap | **Owner re-asked 2026-10-14 on corrected premise; answered by the owner, 2026-10-14: "Extend the existing test".** This supersedes the 2026-10-13 answer "a", which rested on the false premise that no sflowtool carrier existed. The carrier gains the sample-pool and wrap assertions as a sibling test in the same file, `TestFlowExportSFlowToolSamplePoolInterop` (review r2 #6): `assertExportSFlowDatagrams` requires exactly two datagrams, refuses a duplicate sample type, and pins `sampleSequenceNo` "1" and one `sourceId` for every sample, so more flow samples on two sources would trip all four, and the existing test keeps its assertions unchanged (Interop Tests). No scenario directory, no `test/interop/scenarios/` entry, no BGP lab sidecar, no new suite. The wrap at 2^32 is decoded by a third party, sflowtool (AC-26), as well as proven by the unit (AC-18). sflowtool reads `sample_pool` into a 32-bit unsigned and prints it with `%u` (`sflowtool.c` `readFlowSample`, read in an older session's copy, not confirmed to be pin `c953cfe8`), so it prints the decoded value modulo 2^32 and computes no delta: any delta is the test's own arithmetic over consecutive `samplePool` strings |
| Q-9 | Raised 2026-10-13 from the producer, after Q-1 "b", Q-3 "b", Q-4 "a" and Q-5 "a". Every RFC 4271 event a local close fires from Established (ManualStop, AutomaticStop, HoldTimer_Expires, OpenCollisionDump, UpdateMsgErr, BGPHeaderErr, BfdDown) is preceded by a NOTIFICATION ze builds through `Session.sendNotificationWithin` (`session_write.go`), so under Q-3 "b" each is reason 1. The only local closes that build none are the Q-5 closes, which are 00 00. `Session.Stop` (ManualStop with no NOTIFICATION) has no production caller. A strict-BFD close builds none only when `Session.teardown` reads a nil connection, which happens only after another closer has run `closeConn`. Corrected 2026-10-14 (review #3): the 2026-10-13 text said "that closer records first", which is false at HEAD, where `Run`'s `defer s.closeConn()` records nothing; and `sendHoldTimerExpired` (`session_write.go`) also fires HoldTimerExpires with no NOTIFICATION when it reads a nil conn. Corrected again 2026-10-14 (review r2 #1): this spec's own recorders close every other Established exit of `Run` before its deferred `closeConn` (OD-2), and the cancel goroutine's panic-recovery `closeConn` is not a production shape (it runs only after the record is written). The one remaining window is a race: a context cancel while the session is paused, where `waitForResume` (`session_flow.go`) returns `ctx.Err()` on `<-ctx.Done()` and `Run`'s deferred `closeConn` runs before the cancel goroutine, woken by the same `ctx.Done()`, writes its local record; under "a", a `bfdTeardown` or a hold expiry in that gap would have recorded reason 2 with its event. The 2026-10-14 answer "a" rested on "reachable after an unrecorded closer", which the spec's own recording sites make true only for that race | (a) add an RFC 4271 number to each entry of the per-event table (`eventNames` in `fsm/state.go` holds names only today) and an event field on the down event, so a local close with no NOTIFICATION reports its event; (b) carry no event: reason 2 always writes 00 00, "Two bytes both set to 0 are used to indicate that no relevant Event code is defined" (RFC 7854 Section 4.9). Re-asked options: (i) deliberate keep with the race-only writer; (ii) record in `waitForResume`'s ctx arm too, leaving the event field test-only; (iii) drop the table, reason 2 always 00 00 | **Answered by the owner, 2026-10-14: "iii"** (re-asked on the corrected premise that the event-number table's writer is reachable only through the ctx-cancel-while-paused race): drop the event-number table and the event field; reason 2 always writes 00 00. **This supersedes the 2026-10-14 answer "a"**, which rested on "reachable after an unrecorded closer". Consequences: `eventNames` gains no number and no accessor; the close record, `PeerInfo` and `rpc.StructuredEvent` carry no event; the down state event carries no `close-event`; Q-4's 00 1F is never sent. The nil-connection branches still record local, no PDU, because each still fires its FSM event and the down event must not arrive Unspecified (OD-2); in the race they give reason 2 with 00 00, the Q-5 class |

## Required Reading

### Architecture Docs
- [ ] `docs/guide/bmp.md` - the BMP sender's documented Peer Down behaviour (line 162, line 186: "with that peer's identity and mapped reason code")
  → Constraint: the page says only "mapped reason code"; the reason-by-cause table this spec produces goes on this page in the same change
  → Constraint: line 186 already carries the Section 3.3 rule (Peer Down only for a peer whose Peer Up was sent), written by the Section 3.3 change this spec depends on (dce906c381)
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
  → Decision: Q-9 "iii" (owner, 2026-10-14): reason 2 always writes 00 00, so no RFC number is declared for any `fsm.Event`; the iota value never reaches the wire
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
| draft-ietf-idr-bgp-bfd-strict-mode-19 (`rfc/drafts/`, not enrolled) | 8.7.2 | "In response to the BfdDown event (Event 31), the local system: sends a NOTIFICATION message with the error code Cease (6) and error subcode BFD Down (10)," |
| RFC 9069 | 5.3 | "The Peer Down notification MUST use reason code 6." |
| sFlow v5 | 5, `flow_sample` | "unsigned int sample_pool;      /* Total number of packets that could have been sampled (i.e. packets skipped by sampling process + total number of samples) */" |
| sFlow v5 | 5, `flow_sample` | on `sequence_number`: "Note: If the agent resets the sample_pool then it must also reset the sequence_number." |

**Key insights:** (minimal context to resume after compaction)
- Every Established exit in `fsm.handleEstablished` goes to Idle, so `peer_run.go` always hands `announceClosed` the text `connection lost`; the text carries no cause.
- The only cause signal the BMP plugin has today is its own NOTIFICATION cache, filled from `notification` events the binding may or may not grant.
- Ze fires FSM Event 18 for its own local closes too (GR-aware congestion teardown, context cancel, write retirement), and the read goroutine's Event 18 can reach the FSM before the teardown's own ManualStop when the NOTIFICATION write fails. The event the FSM processed is therefore not the cause; the cause must be recorded where the close is decided, first writer wins, before the connection is closed.
- `Session.setCloseReason` (`session_connection.go`) is already first-writer-wins and already called before `closeConn` at every local site; the close record (OD-2) takes the same shape.
- Every NOTIFICATION a session builds passes through `Session.sendNotificationWithin` (`session_write.go`; `logNotifyErr` in `session.go` and the hold-expiry arm call it), so one recording site covers every built NOTIFICATION, delivered or not (Q-1 "b", Q-3 "b").
- `handleConnectionClose` (`session_read.go`) fires Event 18 BEFORE `closeConn`, and the FSM callback publishes the down event inside that event, so a remote record must be written before the event fires.
- Under Q-3 "b" every local close whose conn is still published builds a NOTIFICATION and is reason 1. A local closer's nil-connection branch is reached only after an unrecorded `closeConn`, and in production that is only the paused-cancel race (`waitForResume` returns `ctx.Err()`, `Run`'s deferred `closeConn` runs before the cancel goroutine records; OD-2). On that premise the owner answered Q-9 "iii" (2026-10-14): no event is carried, and every reason 2 is 00 00.
- Some `Session.Run` exits fire no FSM event (`read body`, a header read error that is neither EOF nor a reset), so no down event is published for them at HEAD; `spec-bgp-rs-replacement-on-withdrawal` Phase 2b publishes it, and this spec records their cause (remote).
- `docs/config-reference.md` "Process Bindings" states the receive grants; OD-1 changes what `state` carries on a down event.
- `plugin.PeerInfo.InitialReplay` is the precedent for a per-event value carried on PeerInfo and copied into `rpc.StructuredEvent` by `getStructuredStateEvent` (`internal/component/bgp/server/events.go`).
- sFlow: `sample_pool` is `min(seq*rate, MaxUint32)`; rate is 1..1000000 by YANG for config, but `TestFlowExportSFlowToolInterop` sets `FlowSample.Rate` directly, so sflowtool can observe the wrap (Q-8, 2026-10-14).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/bmp/bmp_events.go` - `peerDownCause`, `fsmEventNone`, `clearPeerState`, `peerDownFor`, `handleSenderState`, `cacheNotificationPDU` (the Section 3.3 change is committed, dce906c381)
  → Constraint: `peerDownFor(notify, reason string)` decides: cached sent NOTIFICATION gives 1, cached received gives 3, `rpc.ReasonPeerRemoved` gives 5, everything else gives 2 with the constant `fsmEventNone` (00 00). Reason 4 (`PeerDownRemoteNoData`, `msg.go`) is declared and never produced
  → Constraint: the Section 3.3 change (dce906c381) adds `peerDownCause.reported` (true only when `peerUps` held the peer) and the down arm of `handleSenderState` returns early when it is false. Keep it: the cause is decided only for a reported peer
  → Constraint: `clearPeerState` reads and deletes `notifyCache` in one lock today (the comment explains why the two jobs are one function). Under Q-1 "b" the cache and that read are deleted; the reason is chosen from the down event alone, and the comment is rewritten with it (`ai/rules/stale-comments.md`)
  → Constraint: `cacheNotificationPDU` keeps "The last NOTIFICATION wins"; the reactor record is first writer wins. Two NOTIFICATIONs exist only when they crossed on the wire, and the first one is the one that closed the session
- [ ] `internal/component/bgp/plugins/bmp/bmp.go` - subscriptions (line 500): state, update, open, notification, keepalive, refresh subscribed unconditionally
  → Constraint: delivery is still narrowed per peer by the binding's receive grant (`ProcessBinding.Receive`, `peer_settings.go`); `notification` is its own grant, so a binding can feed state and open without NOTIFICATIONs
  → Decision: OD-1, the down event carries the closing NOTIFICATION whatever the grant. The `notification` subscription stays: Route Mirroring reads it (`handleStructuredEvent` mirror arm, `bmp_events.go`); only the cache arm goes
- [ ] `internal/component/bgp/reactor/session_bfd_strict.go` - `handleBFDEvent`, `bfdTeardown`
  → Constraint: from Established only BfdDown tears down (BfdHoldTimerExpires has no Established clause, BfdStrictConfigChanged is ignored there); it goes through `Session.teardown`, which builds the Cease BFD Down whenever the connection is still published (Q-4, Q-9)
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
  → Decision: Q-9 "iii" (owner, 2026-10-14, superseding "a"): untouched; `eventNames` keeps names only and gains no RFC number
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
| Operator teardown | `Session.Teardown` | Cease, if the write succeeds | ManualStop (2), or 18 if the read loop wins | `ErrTeardown` | 1 (cache) | 1 with the built PDU, delivered or not (Q-3 "b") |
| Automatic stop (non-strict BFD, congestion non-GR) | `teardownAutomatic` | Cease | AutomaticStop (8) | `ErrTeardown` | 1 | 1 with the built PDU (Q-3 "b") |
| Hold timer expiry | `session.go` hold arm | Hold Timer Expired | HoldTimer_Expires (10) | `ErrHoldTimerExpired` | 1 | 1 with the built PDU (Q-3 "b") |
| Collision dump | `CloseWithNotification` | Cease | OpenCollisionDump (23) | `ErrCollisionDump` | 1 | 1 with the built PDU (Q-3 "b") |
| Policy teardown | `session_read.go` line 421 | the filter's code | UpdateMsgErr (28) | `ErrPolicyTeardown` | 1 | 1 with the built PDU (Q-3 "b") |
| Strict BFD Down | `bfdTeardown` | Cease BFD Down | BfdDown (draft 31) | `ErrTeardown` via teardown | 1 | 1 with the built PDU (Q-3 "b"); when the conn was already emptied by a closer that recorded nothing (only the paused-cancel race), 2 with 00 00 (Q-9 "iii", superseding Q-4 "a") |
| Any local closer reading a nil conn (`teardown`, hold-expiry arm, `CloseWithNotification`, `sendHoldTimerExpired`) after an unrecorded `closeConn` (in production only `Run`'s deferred close in the paused-cancel race: `waitForResume` returns `ctx.Err()` before the cancel goroutine records) | the closer's nil-connection branch | none | the closer's own event | the closer's error | 2 with 00 00 | 2 with 00 00 (Q-9 "iii"); the branch records local, no PDU, so the down event its FSM event publishes is never Unspecified |
| Peer sends NOTIFICATION | `handleNotification` | received | NotifMsg (25) / NotifMsgVerErr (24) | none | 3 if the binding grants `notification`, else 2 | 3 with the received PDU, whatever the grant (Q-1 "b", OD-1) |
| Peer closes TCP or resets | `handleConnectionClose` | none | TcpConnectionFails (18) | none | 2 with 00 00 | 4 |
| Our write fails (peer gone) | `retireWrite` | none | 18 via the read loop | the I/O error | 2 with 00 00 | 4 |
| Ze retires its own writer (raw diagnostic bytes, a refused partial UPDATE write, a forward flush past ze's write deadline) | `finishRawWrite` (`errRawOwnership`), `writeUpdateBody` deferred `retireWrite(result)`, forward flush in `session_write.go` | none | 18 via the read loop | `errRawOwnership` or the refusal | 2 with 00 00 | 2 with 00 00 (Q-5 class; review #5) |
| Read loop ends on a `read body` error, or a header read error that is neither EOF nor a reset | `readAndProcessMessage` (`session_read.go`), `readAndProcessCoalesced` (`session_coalesce.go`) | none | none: `Run` returns with the FSM still Established, so no down event is published at HEAD | none | none (no Peer Down) | 4; publication is `spec-bgp-rs-replacement-on-withdrawal` Phase 2b, the record is this spec |
| GR-aware congestion drop | `congestionTeardownPeer` | none | 18 | `ErrCongestionTeardown` | 2 with 00 00 | 2 with 00 00 (Q-5) |
| Reactor stop | `Reactor.Stop` → `stop(true)` → `notifyPeersShutdown` → `Peer.shutdownNotify` → `Session.Teardown(NotifyCeaseAdminShutdown)` | Cease Administrative Shutdown, if the write succeeds | ManualStop (2), or 18 if the read loop wins | `ErrTeardown` | 1 (cache) | 1 with the built PDU (Q-3 "b"; review #6) |
| Context cancel, `StopForRestart` | `Run` cancel goroutine (`stop(false)` notifies nobody) | none | 18 via the read loop | `ctx.Err()` or the errChan value | 2 with 00 00 | 2 with 00 00 (Q-5) |
| Peer removed from config | `removePeer` (`reactor_peers.go` line 287) | Cease Peer De-configured | n/a, direct event | n/a | 5 | 5 |

**Behavior to change:** D1, the reason column (2 with 00 00 for a remote close) becomes the owed column. D2, the saturation in `EncodeFlowSample`.

**Behavior to preserve:**

| Behaviour | Where |
|-----------|-------|
| No Peer Down for a peer never reported up (Section 3.3) | `peerDownCause.reported`, `handleSenderState` down arm |
| Reason 1 and 3 carry the exact NOTIFICATION PDU, and a PDU never outlives its session. Today from the BMP cache; under Q-1 "b" from the down event, and the per-Session record dies with its Session (OD-2) | today `cacheNotificationPDU`, `clearPeerState`, `TestPeerDownDropsTheNotificationWithTheSession`; after, the reactor record and `TestSessionCloseRecordDiesWithTheSession` |
| Reason 5 for `rpc.ReasonPeerRemoved`, with no Data | `peerDownFor`, `TestRFC7854PeerDownOmitsDataWhereTheReasonHasNone` |
| Reason 6 on the Loc-RIB Peer Down | `bmp_locrib.go`, `bmp-locrib-pmacct` |
| The textual `Reason` string on state events and JSON | `peer_run.go`, `format.AppendStateChange` (Q-6) |
| sFlow `sample_pool` grows by the rate per exported sample below 2^32 | `TestSFlowFlowSamplePoolTracksTotal` "grows by rate per sample" |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- D1: a BGP session leaving Established, by any row of the close-path table above
- D2: a packet sampled by the kernel and handed to `FlowEncoder.EncodeFlowSample`

### Transformation Path (D1, Q-1 "b")
1. The session's close record (OD-2) is written once, first writer wins, before the I/O or the FSM event that can publish the down event. `Session.sendNotificationWithin` records origin `local` with a copy of the NOTIFICATION body it built, before `writeMessageWithin`, so a refused write keeps it (Q-3 "b"). `Session.handleNotification` records origin `remote` with a copy of the received body, before `UnpackNotification` and before `fsmMessageEvent`. The copy is owed because the body lives in the pooled read buffer, not because unpack can fail: every received NOTIFICATION has passed `hdr.ValidateLengthWithMax` (`MinNotificationLen` 21, `message/header.go`) and `UnpackNotification` refuses only a body under 2 octets. A version-error NOTIFICATION (`notificationEvent`, OPEN Message Error / Unsupported Version Number, Event 24) is recorded the same way.
2. Closes with no NOTIFICATION record their origin with no PDU, before they close or fire. `handleConnectionClose` (read EOF or reset) and `retireWrite` (write I/O failure) record `remote`, before Event 18 and before the connection is closed. The `Run` exits with no FSM event (`read body`, a non-EOF non-reset header error, in `readAndProcessMessage` and `readAndProcessCoalesced`) record `remote` before returning. The Q-5 sites record `local`: the GR-aware branch of `congestionTeardownPeer`, the cancel goroutine in `Session.Run` (context cancel, `StopForRestart`; `Reactor.Stop` is reason 1, step 1), and the local retirements (`finishRawWrite`, the refused partial write in `writeUpdateBody`, the forward flush past ze's deadline), each before it calls `retireWrite`. The nil-connection branches of `Session.teardown`, the hold-expiry arm, `CloseWithNotification` and `sendHoldTimerExpired` record `local` with no PDU before the FSM event the closer still fires (Q-9 "iii": no event is recorded); first writer wins, so this lands only after a `closeConn` that recorded nothing, which in production is only the paused-cancel race (OD-2).
3. The from-Established arm of the FSM callback in `peer_run.go` loads the record and passes it to `announceClosed`, which hands it through `notifyPeerClosed` to `apiStateObserver.OnPeerClosed`, which sets it on `plugin.PeerInfo`.
4. `getStructuredStateEvent` copies it onto two new `rpc.StructuredEvent` fields: the close origin (enum beside `SessionState`, zero Unspecified) and the NOTIFICATION body (empty when none). No event field (Q-9 "iii"). Whether the PDU was built or received follows from the origin. `PutStructuredEvent` zeroes both. `format.AppendStateChange` writes the same two facts for JSON and text subscribers (OD-1; "State event wire shape" below).
5. `clearPeerState` no longer reads a cache. `peerDownFor` decides from the down event: `ReasonPeerRemoved` gives 5; origin local with a PDU gives 1 carrying it; origin remote with a PDU gives 3 carrying it; origin local with no PDU gives 2 with the 2-octet Data 00 00 (Q-5 "a", Q-9 "iii"); origin remote with no PDU gives 4 with no Data; origin Unspecified on a non-removed down is a producer defect: WARN naming the peer, and reason 2 with 00 00. The 19-octet header is synthesized by `bgpPDU` as today.
6. `handleSenderState` writes the Peer Down only when `reported` holds (unchanged).

### State event wire shape (OD-1, JSON and text subscribers)

The down state event gains two optional members, written by `appendStateChangeJSON` and `appendStateChangeText` from `PeerInfo`, in the order `close-origin`, `close-notification`, both BEFORE `reason` (review r2 #2). There is no `close-event` member: under Q-9 "iii" the record carries no event, so it would have no value to carry. In the text form that order is load-bearing: `appendStateChangeText` writes the reason as is, and on every Established exit it is two words, `connection lost` (`peer_run.go`), so a reader finds the end of the reason only at end of line. A down event never carries `initial-replay` (`apiStateObserver.OnPeerClosed` sets none), so on a down line `reason` is the last member. `unheld-roles` is a JSON member only (`format.AppendStateChange`), never in the text form. In JSON the order carries no meaning (`bgp.ParseEvent` skips unknown keys without a `/`), and the members are written in the same order. Each is absent when its fact is absent, so an up event, a `removed` down and every existing parser see what they see today. Keys are kebab-case; the NOTIFICATION members reuse the names the NOTIFICATION event already uses (`code`, `subcode`, `data` as hex of the Data field, `format/json.go` `Notification`).

| Member (JSON) | Text keyword | Type | Present when | Value |
|---------------|--------------|------|--------------|-------|
| `close-origin` | `close-origin` | string, `local` or `remote` | the close record exists (origin not Unspecified) | who closed the session |
| `close-notification` | `close-notification code <n> subcode <n> data <hex>`; with no Data field the text member ends after the subcode value, `close-notification code <n> subcode <n>`, because `reason` follows it and a bare `data` (what `AppendNotification` in `format/text.go` writes at end of line) would take `reason` as its value (review r2 #7) | object with `code`, `subcode`, `data` | a NOTIFICATION closed the session, built (origin local) or received (origin remote) | that NOTIFICATION; JSON `data` is the empty string when it has no Data field |

Output sample, JSON, a ze teardown (one line on the wire):
`{"type":"bgp","bgp":{"message":{"type":"state"},"peer":{...},"state":"down","close-origin":"local","close-notification":{"code":6,"subcode":2,"data":"..."},"reason":"connection lost"}}`

Output sample, text, a strict BfdDown in the paused-cancel race (no NOTIFICATION built):
`peer 192.0.2.1 remote as 65001 state down close-origin local reason connection lost`

Output sample, text, a peer Cease with no Data field:
`peer 192.0.2.1 remote as 65001 state down close-origin remote close-notification code 6 subcode 2 reason connection lost`

The text `reason` value is written as is and stays untouched (Q-6 "a"). The three keyword scanners in the tree, `rs` `parseTextState` (`server_text.go`), `rr` (`rr.go`) and `redistribute_egress` `parseStateEvent` (`register.go`), survive the new members; each already keeps only the first word of a down reason, `connection`, which is the pre-existing BNF/producer mismatch with its journal row (2026-10-14, `plan/journal/declared-format-contradicts-payload.md`). In `rs`, the value `remote` of `close-origin remote` meets the header arm `tokenRemote`, which swallows the next token while probing for `as`. On a remote close with no NOTIFICATION (`close-origin remote reason connection lost`) that token is the `reason` keyword itself, so rs's down `Event.Reason` reads empty. Harmless today, because rs reads `Reason` only to match `attached` (`server_handlers.go`), which a down event never carries; it is still a value that collides with a keyword. `docs/architecture/api/text-format.md` `<state-event>` BNF gains `[<close-origin>] [<close-notification>]` before `[<reason>]`, and states that a down event's `<reason>` runs to end of line (Documentation row 8).

### Transformation Path (D2, proposed)
1. `FlowEncoder` keeps a per-ifIndex pool beside `seqNums`, created and reset with it.
2. Each sample adds its own rate to that pool in 32-bit unsigned arithmetic, so the value wraps modulo 2^32, and writes it as `sample_pool`. (If the owner picks sequence times rate instead, the product is taken modulo 2^32.)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Session ↔ Peer (reactor) | session close cause read by the FSM callback in `peer_run.go` | Yes, callback has the session (`peer_run.go` line 465) |
| Reactor ↔ server event dispatch | `plugin.PeerInfo` field, set in `apiStateObserver.OnPeerClosed` | Yes, `InitialReplay` travels this way |
| Engine ↔ BMP plugin | `rpc.StructuredEvent` new typed fields (DirectBridge) | Yes, `getStructuredStateEvent` |
| Engine ↔ text/JSON plugins | the down state event gains `close-origin`, `close-notification` before `reason` (OD-1, review r2 #2); `reason` itself is unchanged (Q-6 "a") and stays last on a down line | Yes, `format.AppendStateChange` reads `PeerInfo`, the `InitialReplay` precedent (`initial-replay` member) |

### Integration Points
- `Session.setCloseReason`, whose shape the close record takes, and `Session.sendNotificationWithin`, `handleNotification`, `handleConnectionClose`, `retireWrite` and its local callers, the `Run` exits with no FSM event, the Q-5 sites, and the nil-connection branches of the local closers, which write it
- `rpc.StructuredEvent` and `PutStructuredEvent`; `format.AppendStateChange` (JSON and text)
- `peerDownFor`, `clearPeerState`; `notifyCache`, `notifyPDU`, `cacheNotificationPDU` and the cache arm of `handleStructuredEvent` (deleted)

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | the cause takes the existing state-event path, PeerInfo to StructuredEvent |
| No unintended coupling (components stay isolated) | Yes | BMP reads typed rpc fields; it imports nothing from the reactor or fsm |
| No duplicated functionality (extends existing, does not recreate) | Yes | the record takes the `setCloseReason` shape; the BMP NOTIFICATION cache is deleted, so the reactor record is the one PDU source (Q-1 "b", no layering) |
| Zero-copy preserved where applicable (refs, not copies) | Yes, one copy owed | the received body is copied once out of the pooled read buffer (the buffer is reused, and the payload must be a self-contained value, `ai/rules/plugins.md`); the built body is copied once at build; both cold, at most one per close. The Peer Down Data for reason 2 is two octets written into the message buffer |
| Registration over hardcoding, outbound | Yes | no new command, family, or handler |
| Registration over hardcoding, inbound | Yes | no event-to-number mapping exists anywhere (Q-9 "iii"); no switch learns event names |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|------------|---------------------------------|----------|--------------|--------|
| A-1 | Every NOTIFICATION a session builds goes through `sendNotificationWithin`, and every close with none passes a recording site before `closeConn` or Event 18, so the record is complete before the FSM callback publishes the down event | `grep` 2026-10-13: every reactor NOTIFICATION send is `logNotifyErr` (which calls `sendNotification`) or the hold-expiry `sendNotificationWithin`; `reactor_connection.go` builds one for a connection that never became a session; `session_prefix.go` builds one handed to `logNotifyErr`. The no-NOTIFICATION closers read: `handleConnectionClose`, `retireWrite` and its local callers (`finishRawWrite`, `writeUpdateBody`, the forward flush), the `Run` exits with no FSM event (`read body`, non-EOF non-reset header error), congestion GR branch, the `Run` cancel goroutine, and the nil-connection branches of `teardown`, the hold-expiry arm, `CloseWithNotification` and `sendHoldTimerExpired` (review 2026-10-14 #3, #4, #5). One closer records nothing by design, `Run`'s `defer s.closeConn()`, and every Established exit records before it except the paused-cancel race (review r2 #1); the cancel goroutine's panic-recovery `closeConn` runs only after its record | a local close reports reason 4, or a remote one reason 2 | a unit test per recording site, plus a race test closing via teardown while the read loop runs; R-1's table test over every `closeConn` and `retireWrite` caller and every `Run` exit | unvalidated |
| A-2 | When a NOTIFICATION write fails, the read goroutine's Event 18 can reach the FSM before the teardown's own stop event | `teardown` order: write, `closeConn`, then `logFSMEvent(stopEvent)`; `retireWrite` closes the conn | none: the PDU is recorded before the write, so whichever event leaves Established finds it | reading; the design does not depend on which event wins | unvalidated |
| A-3 | `apiStateObserver.OnPeerClosed` is the only producer of the down event the BMP plugin turns into reason 1 to 4 | `session_handlers.go` line 113 comment: "notifyPeerClosed is the sole producer of the SessionStateDown"; detach/removal produce `ReasonPeerRemoved` | a second producer would arrive with origin Unspecified | grep for `OnPeerStateChange(` and `OnPeerUnbound(` callers, and the WARN arm's test | unvalidated |
| A-4 | The BMP sender only ever runs in-process (DirectBridge), so it reads the cause from the StructuredEvent fields; any other `state` subscriber reads it from the JSON or text members (OD-1) | `OnStructuredEvent` is fed by DirectBridge (`bridge.go` doc); `register.go` has a CLIHandler, so an external run is possible but receives no structured events today | an external BMP sender would never see the cause, though the JSON members now carry it | check whether `bgp-bmp` can be configured external; if it can, journal row | unvalidated |
| A-5 | pmacct prints `reason_type` 1, 3 and 4 the way it prints 6 | `pmacctPeerDownReasonSix` in `names.go` is `"reason_type": 6` | the interop needles change | the first scenario run | unvalidated |
| A-6 | FRR's bgpd can be killed inside the lab container (`opExec`) so the TCP session ends without a NOTIFICATION | `check_rfc.go` line 73 runs `vtysh` through `Lab.Exec`; `opExec` exists (`checkers.go` isis scenarios) | reason 4 cannot be provoked in interop; use an `iptables` drop plus hold expiry on FRR's side instead | first scenario run | unvalidated |
| A-7 | The `.ci` harness can make its emulated peer close TCP without a NOTIFICATION | not checked | the functional test falls back to the interop scenario as the user-path proof | read `internal/test/` peer emulator options in the implement phase | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|-----------------------|
| R-1 | A local close with no NOTIFICATION is missed and an unrecorded close later meets `handleConnectionClose`, so it reports reason 4 (remote); or a `Run` exit with no FSM event leaves no record, so the Phase 2b down event arrives Unspecified | the per-site unit table has a row with no record | the table test walks every `closeConn` and `retireWrite` caller and every `Run` exit found by grep (including `read body` and the non-EOF header error in both read loops); a new one with neither a NOTIFICATION nor a record fails it, except the one named unrecorded closer, `Run`'s deferred `closeConn` (A-1) |
| R-2 | The pooled `StructuredEvent` leaks a cause from one event to the next | a close origin or PDU on an up event or on a peer-removed down | `PutStructuredEvent` zeroes both fields; a test gets, sets, puts, gets again and asserts zero |
| R-3 | The two downs for a removed peer (`removePeer`'s `removed` and the FSM's own down) arrive in either order | a removed peer reported as reason 1 or 4 instead of 5 | unchanged by this spec: the first down clears `peerUps`, the second finds `reported` false and sends nothing. Pinned by the existing reason 5 test |
| R-4 | Withdrawn 2026-10-14: the RFC 4271 number table it guarded is dropped (Q-9 "iii") | n/a | n/a |
| R-5 | Origin Unspecified is reached in production and silently reads as reason 2 | WARN `bmp: peer down with no close cause` in a log | WARN is the "say so" the principles require; a test pins it; a functional run greps for its absence |
| R-6 | A recording site writes after the I/O or the event it precedes, so a down event is published before its record | a reason 4 for a local teardown under `-race`, or a reason 2 for a received NOTIFICATION | each recording-site test asserts the record is set when the write is refused, and when the FSM callback runs |
| R-7 | D2 wrap changes what an existing collector dashboard sees above 2^32 | none in tests; operators with saturated pools see the counter restart | the wrap is the owner's call (D2 decision); the doc page states it |
| R-8 | The Section 3.3 change (committed as dce906c381) is changed again before this spec is implemented | `git log` of `bmp_events.go` shows a later commit | re-read `peerDownCause.reported` and the down arm at implement start |
| R-9 | Memory held by the record (OD-2) | a Session retained past its close | the record is freed with its Session; at most 4077 octets, or 65516 with RFC 8654 Extended Message, one per closed Session; `PutStructuredEvent` clears the event's reference |
| R-10 | `spec-bgp-rs-replacement-on-withdrawal` Phase 2b has not landed when this spec is implemented | a peer killed mid-UPDATE in `bmp-peerdown-reason-pmacct` yields no Peer Down at all | the AC-1 interop and AC-23 depend on it (metadata Depends); the record written here is tested at the session without it, and the Peer Down for those exits is asserted once Phase 2b lands |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A BMP collector misattributes why a session closed (local vs remote), or misdecodes reason 2 Data; no BGP session behaviour changes. D2: an sFlow collector's pool delta |
| How is it reverted? | single commit revert per defect |
| Who else touches this path? | the BMP Section 3.3 change (dce906c381, `bmp_events.go`, its tests); `spec-bgp-rs-replacement-on-withdrawal` Phase 2b (`runOnce` down defer, `peer_run.go`); JSON/text state-event consumers, which see only added members (OD-1); `spec-rfc-verdict-fix-services` (SFLOW-V5-x-11 row text); `spec-rfc-verdict-fix-bgp` (it moved the BMP verdicts here and touches nothing now) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| peer drops TCP (FRR bgpd killed) | → | `handleConnectionClose` → cause remote → `peerDownFor` reason 4 | interop `bmp-peerdown-reason-pmacct`, needle `"reason_type": 4` |
| peer sends NOTIFICATION (FRR `neighbor shutdown`) | → | `handleNotification` → close record → down event → reason 3 | interop `bmp-peerdown-reason-pmacct`, needle `"reason_type": 3` |
| operator tears the peer down (`ze` peer teardown command) | → | `Session.Teardown` → `sendNotificationWithin` close record → down event → reason 1 | interop `bmp-peerdown-reason-pmacct`, needle `"reason_type": 1` |
| session close inside the reactor | → | cause recorded → `PeerInfo` → `StructuredEvent` | `TestPeerClosedCarriesTheSessionCloseCause` (reactor) and `TestStructuredStateEventCarriesTheCloseCause` (server) |
| down state event to a JSON or text `state` subscriber | → | `PeerInfo` → `format.AppendStateChange` → `close-origin`, `close-notification` | `TestStateChangeCarriesTheCloseCause` (format) and the `.ci` below, which reads the JSON member from a process granted `state` only |
| strict BfdDown after an unrecorded `closeConn` (the paused-cancel race) | → | `bfdTeardown` → `Session.teardown` nil-connection branch → record local, no PDU → down event → reason 2 with 00 00 (Q-9 "iii") | `TestTeardownOnClosedConnRecordsALocalClose` (reactor) then `TestRFC7854PeerDownReasonTwoCarriesItsEventCode` (bmp) |
| down state event at the BMP plugin | → | `handleStructuredEvent` → `clearPeerState` → `peerDownFor` → wire | `TestRFC7854PeerDownReasonFollowsTheCloseCause` (bmp, reads the decoded Peer Down off the collector pipe) |
| sampled packet | → | `EncodeFlowSample` → on-wire `sample_pool` | `TestSFlowFlowSamplePoolTracksTotal` (decoded datagram) and the extended sflowtool carrier's sibling `TestFlowExportSFlowToolSamplePoolInterop` (Q-8, 2026-10-14) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | monitored peer closes TCP with no NOTIFICATION | Peer Down reason 4. A peer killed mid-UPDATE can end the read loop on `read body` instead of EOF (AC-23); that exit's Peer Down exists only once `spec-bgp-rs-replacement-on-withdrawal` Phase 2b lands (metadata Depends, R-10) |
| AC-2 | sample pool crossing 2^32 | wraps modulo 2^32 |
| AC-3 | the `weak` verdict of SFLOW-V5-x-11, after this spec's D2 producer fix | the verdict reaches `enforced`: a tagged test proves the quoted sentence, and an agent that did not write that test re-judges it with `./le rfc audit-stamp ... mode rejudge`. The row correction stays with `plan/release/validation/spec-rfc-verdict-fix-services.md`, whose "Blocked by" table moved the verdict here (parent P-3, 2026-09-28) |
| AC-4 | the `weak` verdict of RFC7854-x-10 ("Reason indicates why the session was closed", Section 4.9), after this spec's D1 producer fix | the verdict reaches `enforced`: tagged tests tie each observed close cause to its reason in `peerDownFor` (a TCP loss or a peer close without NOTIFICATION gives reason 4, a local close without NOTIFICATION reason 2, a sent or received NOTIFICATION reason 1 or 3, a deconfigured peer reason 5), both polarities, re-judged by an agent that did not write them with `./le rfc audit-stamp ... mode rejudge`. The BGP child spec-rfc-verdict-fix-bgp moved the verdict here (parent P-3, 2026-09-29) |
| AC-5 | the `weak` verdict of RFC7854-4.9-1 (Section 4.9, reason 2: "Following the reason code is a 2-byte field containing the code corresponding to the Finite State Machine (FSM) Event that caused the system to close the session"), after this spec's D1 producer fix | a local close without NOTIFICATION reaches the collector as reason 2 with its 2-octet Data field. Under Q-3 "b" every local close that fires an RFC 4271 event builds a NOTIFICATION and is reason 1, so the reachable reason 2 closes are the Q-5 ones and the nil-connection branches of AC-24, and every one writes Data 00 00, "Two bytes both set to 0 are used to indicate that no relevant Event code is defined" (Q-5 "a", Q-9 "iii"). Today `peerDownFor` writes 00 00 for every reason 2, remote closes included, which is the defect. Tagged tests cover both polarities: a Q-5 close is reason 2 with 00 00, and a remote close with no NOTIFICATION is never reason 2 and are re-judged by an agent that did not write them with `./le rfc audit-stamp ... mode rejudge`. The BGP child spec-rfc-verdict-fix-bgp moved the verdict here (parent ruling R13, 2026-09-29) |
| AC-6 | each local close path, NOTIFICATION write refused (test conn that fails writes): Teardown, teardownAutomatic, hold expiry, collision dump, policy teardown | the collector reads reason 1 carrying the NOTIFICATION PDU ze built and could not deliver (Q-3 "b"): Cease Administrative Shutdown or the configured subcode, Cease from `teardownAutomatic`, Hold Timer Expired, Cease Connection Collision Resolution, the filter's code; also strict BFD Down (Cease BFD Down, subcode 10). Never reason 1 with no PDU, never reason 2, never reason 4 |
| AC-7 | the same paths with the write accepted | reason 1 carrying the exact NOTIFICATION PDU sent, now taken from the down event (Q-1 "b") |
| AC-8 | peer closes TCP (EOF) or resets, no NOTIFICATION, no local cause recorded | reason 4, message ends after the reason byte (no Data); also for a write that fails with an I/O error |
| AC-9 | GR-aware congestion drop, context cancel, `StopForRestart`, and ze's own writer retirements (`finishRawWrite` for `errRawOwnership`, a refused partial write in `writeUpdateBody`, a forward flush past ze's write deadline) | reason 2 with 00 00 (Q-5 "a"), not reason 4: each records local before it closes or calls `retireWrite`, and a later remote record loses (first writer wins) |
| AC-10 | peer sends a NOTIFICATION, binding grants `notification` | reason 3 carrying the received PDU (unchanged on the wire, now taken from the down event) |
| AC-11 | peer sends a NOTIFICATION, or ze builds one, and the binding grants `state` but not `notification` (OD-1) | reason 3 (received) or reason 1 (built) carrying the PDU, exactly as with the grant; the commit is accepted (no refusal, Q-2) |
| AC-12 | a down state event with origin Unspecified and a reason other than `removed` | WARN log naming the peer and that no close cause arrived; reason 2 with 00 00 |
| AC-13 | peer removed from config | reason 5, no Data (unchanged) |
| AC-14 | a peer never reported up, any close cause | no Peer Down (the Section 3.3 change, unchanged) |
| AC-15 | a recycled `StructuredEvent` after `PutStructuredEvent` | close origin Unspecified, no PDU |
| AC-16 | withdrawn 2026-10-14 (Q-9 "iii"): the event-number table it asserted is dropped | none; the ID stays so the AC numbering holds |
| AC-17 | sFlow, rate 1024, three samples | `sample_pool` 1024, 2048, 3072 (unchanged) |
| AC-18 | sFlow, pool crossing 2^32 (rate 2^31 = 2147483648, three samples on one source; review r2 #5) | `sample_pool` 2147483648, 0, 2147483648: it rolls to zero and never writes MaxUint32, the "not available" sentinel; saturation would give 2147483648, 4294967295, 4294967295. The per-sample delta modulo 2^32 equals the rate. At a constant rate this cannot tell per-source accumulation from sequence times rate modulo 2^32, both correct; AC-19 and the rate change in `TestSFlowSamplePoolPerSource` tell them apart |
| AC-19 | sFlow, two sources (ifIndex 7 and 9) | each source's pool advances independently and resets only with its own sequence number |
| AC-20 | the peer sends a version-error NOTIFICATION (OPEN Message Error, Unsupported Version Number; `notificationEvent` gives Event 24) while Established | reason 3 carrying that NOTIFICATION's PDU as received |
| AC-21 | the BMP plugin after the change | holds no NOTIFICATION cache (`notifyCache`, `notifyPDU`, `cacheNotificationPDU` gone); Route Mirroring of NOTIFICATIONs unchanged; a down event with a PDU followed, on the next session, by a down event with none carries no PDU on the second (the BMP half of the guarantee `TestPeerDownDropsTheNotificationWithTheSession` held) |
| AC-22 | `Reactor.Stop` (`stop(true)`, `notifyPeersShutdown`, `Peer.shutdownNotify`, `Session.Teardown(NotifyCeaseAdminShutdown)`) | reason 1 carrying the Cease Administrative Shutdown ze built, delivered or not (Q-3 "b"); never reason 2 |
| AC-23 | the read loop ends on a `read body` error, or on a header read error that is neither EOF nor a reset, with no local closer recorded (`readAndProcessMessage`, `readAndProcessCoalesced`) | the session's record is origin remote, no PDU; once `spec-bgp-rs-replacement-on-withdrawal` Phase 2b publishes the down event for that exit, the collector reads reason 4. When the cancel goroutine closed the conn first (the reader got `i/o timeout` from the 100 ms drain), the record stays the cancel goroutine's local one and the collector reads reason 2 with 00 00 |
| AC-24 (Q-9 "iii", 2026-10-14) | a local closer reads a nil conn after a `closeConn` that recorded nothing, which in production is only `Run`'s deferred close in the paused-cancel race (`waitForResume` returns `ctx.Err()` before the cancel goroutine records; OD-2): strict BfdDown through `bfdTeardown`, `Teardown`, `teardownAutomatic`, hold expiry, `CloseWithNotification`, `sendHoldTimerExpired` | the record is origin local, no PDU, written before the FSM event the closer fires, so the down event is never Unspecified; the collector reads reason 2 with Data 00 00, never the AC-12 WARN. When any recorded closer ran first, its record stays (first writer wins) |
| AC-25 | a process bound with `receive [ state ]` and no `notification`, JSON encoding, then text encoding | its down state event carries `close-origin`, and `close-notification` with `code`, `subcode`, `data` for a NOTIFICATION close; an up event and a `removed` down carry neither (OD-1). In the text form both come before `reason`, and `reason connection lost` ends the line |
| AC-26 | the sflowtool carrier, sibling test `TestFlowExportSFlowToolSamplePoolInterop` in `interop_integration_linux_test.go`: one source, several samples at a fixed rate, then, on a second source (its own ifIndex), three samples at `Rate` 2^31 crossing 2^32 (Q-8, 2026-10-14; review r2 #5, #6) | the decoded `samplePool` values sflowtool prints are, on the first source, k times the rate for sample k, and on the second 2147483648, 0, 2147483648, never 4294967295. sflowtool prints the 32-bit value as decoded and computes no delta (review r2 #4); the delta is the test's arithmetic over those strings |

## End-to-End User Stories

| Story | Path | Proof |
|-------|------|-------|
| An operator watching pmacct sees a peer that crashed reported as "remote system closed, no data" | FRR bgpd killed → ze read EOF → Peer Down reason 4 → pmacct | `bmp-peerdown-reason-pmacct` |
| The same operator sees an administrative shutdown on the peer as reason 3 with the Cease, and ze's own teardown as reason 1 | FRR `neighbor shutdown` / ze teardown command | `bmp-peerdown-reason-pmacct` |
| An operator whose ze dropped a peer locally with no NOTIFICATION (congestion GR drop, stop) reads reason 2 with 00 00; one whose Cease could not be delivered reads reason 1 with that Cease | unit, the paths are not provokable from a peer | AC-6, AC-9, AC-22, AC-24 units |
| An sFlow collector computing per-interval pool deltas modulo 2^32 keeps the delta equal to the packets represented across 2^32, where a saturated pool would give zero | encoder → `Sender` → UDP → sflowtool | AC-18 unit, AC-26 `TestFlowExportSFlowToolSamplePoolInterop` |
| A JSON plugin granted only `state` learns which NOTIFICATION closed the session | ze teardown → down state event → `close-notification` | AC-25 `.ci` |

## 🧪 TDD Test Plan

Failing first: every new or changed assertion below is run red against the current producer and the red is pasted in the session state before the producer changes.

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSessionCloseRecordLocalSites` (new, table) | `internal/component/bgp/reactor/session_close_cause_test.go` | AC-6, AC-7, AC-9, AC-22, A-1: each local close path records origin local with the body it built, before the write, and keeps it when the write is refused; `Reactor.Stop` reaches the Cease Administrative Shutdown record; each Q-5 site and each local retirement (`finishRawWrite` `errRawOwnership`, the refused partial write in `writeUpdateBody`, the forward flush past ze's deadline) records local with no PDU before `closeConn` or `retireWrite` | |
| `TestSessionCloseRecordRemote` (new) | same file | AC-8, AC-20, AC-23: EOF, reset and write I/O failure record remote with no PDU before Event 18; a `read body` error and a non-EOF non-reset header error, in both read loops, record remote with no PDU before `Run` returns; a received NOTIFICATION, a version-error one included, records remote with its body; the copy survives reuse of the read buffer | |
| `TestSessionCloseRecordFirstWriterWins` (new) | same file | a teardown racing the read loop's Event 18 keeps the teardown's PDU; a strict-BFD teardown after a recorded closer ran `closeConn` does not overwrite that closer's record; the cancel goroutine's local record survives the reader's later `i/o timeout` exit (AC-23) | |
| `TestTeardownOnClosedConnRecordsALocalClose` (new, table) | same file | AC-24, Q-9 "iii": the test calls `closeConn` directly to stand in for the one production window, `Run`'s deferred close in the paused-cancel race (OD-2); it does not drive that race. After that unrecorded `closeConn`, `bfdTeardown` (strict BfdDown), `Teardown`, `teardownAutomatic`, the hold-expiry arm, `CloseWithNotification` and `sendHoldTimerExpired` each leave origin local, no PDU, before their FSM event; the down event built by the from-Established arm carries origin local, never Unspecified | |
| `TestSessionCloseRecordDiesWithTheSession` (new) | same file | a new Session starts with no record, so a PDU never reaches the next session's down event (the guarantee `TestPeerDownDropsTheNotificationWithTheSession` held for the BMP cache) | |
| `TestPeerClosedCarriesTheSessionCloseCause` (new, wiring) | `internal/component/bgp/reactor/reactor_api_test.go` | PeerInfo on the down event carries the cause | |
| `TestStructuredStateEventCarriesTheCloseCause` (new, wiring) | `internal/component/bgp/server/events_test.go` | `getStructuredStateEvent` copies it | |
| `TestPutStructuredEventClearsTheCloseCause` (new) | `pkg/plugin/rpc/bridge_test.go` | AC-15 | |
| `TestRFC7854PeerDownReasonFollowsTheCloseCause` (new, tag RFC7854-x-10 both polarities) | `internal/component/bgp/plugins/bmp/rfc7854_peerdown_cause_test.go` | AC-1, AC-4, AC-8, AC-9, AC-10, AC-13: cause in, decoded Peer Down reason out; negative: a remote close is never reason 2, a local close never reason 4 | |
| `TestRFC7854PeerDownReasonTwoCarriesItsEventCode` (new, tag RFC7854-4.9-1 positive) | same file | AC-5, AC-24: a Q-5 local close is reason 2 with Data 00 00, and so is the strict-BfdDown down event `TestTeardownOnClosedConnRecordsALocalClose` produces (Q-9 "iii") | |
| `TestStateChangeCarriesTheCloseCause` (new) | `internal/component/bgp/format/text_test.go` | AC-25: JSON and text down events carry `close-origin`, `close-notification` exactly when the PeerInfo has them; up and `removed` events carry none; in the text form the members precede `reason`, which ends the line, and a NOTIFICATION with no Data field writes no `data` keyword | |
| `TestPeerDownWithNoCloseCauseSaysSo` (new) | same file | AC-12 WARN | |
| `TestPeerDownCarriesTheNotificationWithoutTheGrant` (new, wiring) | `internal/component/bgp/server/events_test.go` | AC-11, OD-1: a binding granted `state` without `notification` receives the down event carrying the PDU | |
| `TestBMPHoldsNoNotificationCache` (new) | `internal/component/bgp/plugins/bmp/rfc7854_peerdown_cause_test.go` | AC-21: a `notification` event leaves the Peer Down unchanged; reason 1 and 3 come from the down event alone; a down event with a PDU, then on the next session a down event with none, gives a Peer Down whose second message carries no PDU | |
| `TestPeerDownReasonMapping` (untagged, CHANGED) | `event_test.go` | row "no notification, connection lost" becomes origin remote, reason 4, Data empty; rows gain the origin and PDU arguments | |
| `TestHandleSenderStatePeerDownCarriesTheNotificationZeSent` (untagged, CHANGED) | `event_test.go` | it calls `bp.cacheNotificationPDU`, which this spec deletes; it now drives the down event with origin local and the PDU ze built, and asserts reason 1 carrying exactly that PDU | |
| `TestPeerDownDropsTheNotificationWithTheSession` (untagged, DELETED with the cache it tests) | `rfc7854_peerdown_data_test.go` | its subject, the BMP cache, is deleted (Q-1 "b"); the reactor half of its guarantee moves to `TestSessionCloseRecordDiesWithTheSession` (a new Session per `runOnce`, `Peer.newSession`), the BMP half (a PDU down, then a no-PDU down, carries no PDU) to `TestBMPHoldsNoNotificationCache` | |
| `TestSFlowSamplePoolPerSource` (new, untagged) | `sflow_v5_flow_adapter_test.go` | AC-19 | |

### RFC-tagged tests this spec changes (owner approval required, Q-7)

| Unit | Tag | Change | Why |
|------|-----|--------|-----|
| `internal/component/bgp/plugins/bmp/event_test.go::TestHandleSenderStatePeerDown` | RFC7854-x-10 positive | the down event gains origin local and the Cease ze built for the ManualStop (Q-3 "b"); the assertion tightens from "2 octets of Data" to reason 1 whose Data is exactly that PDU. The tag prose is NARROWED to what the body drives: today it also claims "a teardown ze closed by NOTIFICATION is reason 1 carrying that PDU", which this body never drives (the untagged sibling does); that clause leaves this tag and is carried by `TestRFC7854PeerDownReasonFollowsTheCloseCause` | today it drives Reason `notification` with no cause and asserts only the Data length, which cannot fail on a wrong event. Review verdict OVER-CLAIM plus NEEDS-OWNER (Q-3), confirmed against the tag comment above `TestHandleSenderStatePeerDown` |
| `internal/component/bgp/plugins/bmp/rfc7854_sender_test.go::TestBMPSenderPeerDown` | RFC7854-x-10 positive | no body change; the tag stays with its narrow claim ("WritePeerDown emits the reason byte"), and the rejudge reads it beside the new cause test | it proves the writer round-trips the reason byte, which is part of x-10, not the cause: it is no evidence of "why the session was closed", so the x-10 weight sits on `TestRFC7854PeerDownReasonFollowsTheCloseCause` |
| `internal/component/bgp/plugins/bmp/rfc7854_peerdown_data_test.go::TestRFC7854PeerDownCarriesTheDataItsReasonRequires` | RFC7854-4.9-1 positive | Q-3 "b": the ManualStop case it drives today (no PDU delivered) is reason 1 carrying the Cease ze built, which a subtest asserts. Its reason 2 subtest drives a close that builds no NOTIFICATION at all (a Q-5 local close) instead of `connection lost`, and asserts the 2-octet Data 00 00; subtests 1 and 3 carry the PDU on the down event with origin local / remote | the audit note: the subtest "drives 'connection lost', an unexpected transport termination that Section 4.9 assigns to reason 4" |
| `internal/component/bgp/plugins/bmp/rfc7854_peerdown_data_test.go::TestRFC7854PeerDownOmitsDataWhereTheReasonHasNone` | RFC7854-4.9-1 negative | gains a reason 4 arm asserting no Data after the reason byte; the tag prose names reason 4 and cites the figure and Section 10.8 "Type = 4: Remote system closed, no data", because the 4.9-1 row text itself quotes reasons 1-3 only | the figure's "Data (present if Reason = 1, 2 or 3)" binds reason 4 too |
| `internal/plugins/flowexport/sflow/sflow_v5_flow_adapter_test.go::TestSFlowFlowSamplePoolTracksTotal` | SFLOW-V5-x-11 positive | subtest "saturates rather than wraps" inverts to "wraps modulo 2^32" (AC-18). The tag prose is rewritten: it drops "tracks the total packets seen" (an over-claim before and after, because the encoder writes an estimate, not Section 5's "Total number of packets that could have been sampled"), states the estimate as Ze's D2 choice and not as sFlow text, and cites sFlow v5 Section 5 for the wrap: "Use the maximum counter value to indicate that the counter is not available." and "An available counter may temporarily have the max value just before it rolls to zero. This is permitted." | the prose today claims saturation, which the text reads as "not available". Unchanged siblings tagged SFLOW-V5-x-28 (in `sflow_v5_test.go` and `sflow_v5_sequence_test.go`) assert pool == k*rate at constant rate and stay true under per-source accumulation; the rewritten prose must not call a wrap a "reset" in the x-28 sense |

Every changed tag owes a fresh `./le rfc discriminate-record` and a rejudge by an agent that did not write it (`./le rfc audit-stamp ... mode rejudge`), per AC-3, AC-4, AC-5.

#### Owner approval commands

Approved by the owner on 2026-10-13 (Q-7). The gate reads approvals from the committing session's own file (`tmp/commit-rfc-approved-<session>.md`), and a row is used up by the commit that carries its `RFC-approved:` trailer. So the owner runs these at the prompt of the session that implements and commits the edits, and runs a unit's command again if a later commit changes that unit a second time. A unit is `<package directory>.<top-level test function>`: the gate names a subtest by its enclosing function, and a moved file by its new directory. Every name was checked to exist at HEAD `051876c3f4`. `TestBMPSenderPeerDown` is planned with no change, so its row is used only if its unit text changes after all. Q-3 is answered "b" (2026-10-13): the first and third commands cover the reason 1 with the built PDU edit.

5 commands:

```
./le rfc approve unit bmp.TestHandleSenderStatePeerDown reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-bmp-sflow-export-rfc-defects, down event carries the built Cease, asserts reason 1 with that PDU, tag prose narrowed)'
./le rfc approve unit bmp.TestBMPSenderPeerDown reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-bmp-sflow-export-rfc-defects, tag kept narrow, run only if the unit text changes)'
./le rfc approve unit bmp.TestRFC7854PeerDownCarriesTheDataItsReasonRequires reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-bmp-sflow-export-rfc-defects, ManualStop with a failed write is reason 1 with the built PDU, reason 2 subtest drives a close with no NOTIFICATION, subtests carry the PDU on the down event)'
./le rfc approve unit bmp.TestRFC7854PeerDownOmitsDataWhereTheReasonHasNone reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-bmp-sflow-export-rfc-defects, gains a reason 4 arm, tag prose names reason 4)'
./le rfc approve unit sflow.TestSFlowFlowSamplePoolTracksTotal reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-bmp-sflow-export-rfc-defects, saturate subtest inverts to wrap modulo 2^32, tag prose rewritten)'
```

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| NOTIFICATION body on the close record | 2..4077 octets, 2..65516 with RFC 8654 Extended Message | 4077 / 65516 | empty means no PDU (origin decides reason 2 or 4) | bounded by the message length the reader already refuses |
| `sample_pool` (uint32) | wraps | 2^32 - 1, next sample wraps to (2^32 - 1 + rate) mod 2^32; the tests cross the limit at rate 2^31 (2147483648, then 0) so they never write the 2^32 - 1 "not available" sentinel | n/a | n/a |
| sFlow rate (YANG) | 1..1000000 | 1000000 | 0 (existing refusal) | 1000001 (existing refusal) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bmp-sender-peer-down-reason.ci` (new, if A-7 holds) | `test/plugin/` | emulated peer closes TCP without NOTIFICATION; the test BMP collector decodes reason 4 | |
| `state-down-carries-close-notification.ci` (new) | `test/plugin/` | AC-25: a process bound `receive [ state ]` without `notification`, JSON encoding; ze tears the peer down; the process's down state event carries `close-origin` `local` and `close-notification` with code 6 and the teardown subcode | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bmp-peerdown-reason-pmacct` (new) | `test/interop/scenarios/bmp-peerdown-reason-pmacct/` (frr.conf, pmbmpd.conf, ze.conf) + checks in `internal/le/interoplab/bgp/checkers.go` | FRR (BGP peer), pmacct `pmbmpd` (collector) | three teardowns in sequence, each waiting for the session to re-establish: ze teardown → `"reason_type": 1`; FRR `neighbor shutdown` → `"reason_type": 3`; FRR bgpd killed → `"reason_type": 4`, and `"reason_type": 2` absent with proof needles | |
| `TestFlowExportSFlowToolSamplePoolInterop` (NEW sibling in the extended carrier, owner Q-8 2026-10-14 "Extend the existing test"; review r2 #6) | `internal/plugins/flowexport/interop_integration_linux_test.go` (tags `integration && linux`), beside `TestFlowExportSFlowToolInterop`, whose assertions stay unchanged; sflowtool v6.11 pinned in `test/interop-flowexport/Dockerfile.collectors`; no scenario directory | sflowtool | AC-26: sflowtool decodes ze's `samplePool` for several samples of one source, the test's delta between consecutive values equals the rate; on a second source at `Rate` 2^31 (set directly on `FlowSample`, the YANG bound 1..1000000 is config only) it prints 2147483648, 0, 2147483648. Today the carrier asserts only `"samplePool": "64"` for one sample. Run route: the test skips without `-flowexport-peer-dir` (`exportInteropTool`), so the evidence and its red come from the collector image's default command, whose `-test.run` pattern in `Dockerfile.collectors` selects only the NFDump and SFlowTool tests today and widens to select the sibling (`flow-export-1-counter-export.md`, "Independent collector tests") | |

Discrimination for the interop (`ai/rules/interop-and-goal-validation.md`): revert `peerDownFor` to today's arm, rebuild the ze image, confirm the reason 4 row goes red (pmacct prints 2), restore, confirm green, record. For the sflowtool carrier: restore the saturating arm in `EncodeFlowSample`, rebuild the collector image so the test binary carries it, confirm `TestFlowExportSFlowToolSamplePoolInterop` goes red through the image's default command (sflowtool prints 2147483648, 4294967295, 4294967295 on the second source), restore, rebuild, confirm green, record.

## Files to Modify

| File | Change |
|------|--------|
| `internal/component/bgp/reactor/session.go` | the session's close record (OD-2: origin, optional body, atomic pointer, first writer wins); the `Run` cancel goroutine records local, no PDU (Q-5); the hold-expiry arm's nil-conn branch records local, no PDU |
| `internal/component/bgp/reactor/session_write.go` | `sendNotificationWithin` records local with the built body before the write; `retireWrite` records remote with no PDU before it closes the conn; `finishRawWrite`, `writeUpdateBody`'s deferred retirement and the forward flush record local, no PDU, before `retireWrite`; `sendHoldTimerExpired`'s nil-conn branch records local, no PDU |
| `internal/component/bgp/reactor/session_read.go`, `session_coalesce.go` | `handleConnectionClose` records remote with no PDU before Event 18; the `read body` and non-EOF non-reset header error exits of `readAndProcessMessage` and `readAndProcessCoalesced` record remote with no PDU before returning |
| `internal/component/bgp/reactor/session_connection.go` | the nil-conn branches of `teardown` and `CloseWithNotification` record local, no PDU, before the FSM event |
| `internal/component/bgp/reactor/session_handlers.go` | `handleNotification` records remote with a copy of the received body before `UnpackNotification` |
| `internal/component/bgp/reactor/forward_pool_congestion.go` | GR-aware branch records local with no PDU (Q-5) |
| `internal/component/bgp/reactor/peer_run.go` | from-Established arm reads the cause and passes it to `announceClosed` |
| `internal/component/bgp/reactor/peer_derived_bindings.go` | `announceClosed` carries it |
| `internal/component/bgp/reactor/reactor.go`, `reactor_notify.go`, `reactor_api.go` | `peerLifecycleObserver.OnPeerClosed` (`reactor.go`), `notifyPeerClosed` and `apiStateObserver.OnPeerClosed` carry the record to PeerInfo, with `callbackAdapter` and the test observers; `registry.PeerLifecycleCallback` and the `mrt` observer are unchanged |
| `internal/component/bgp/format/text_json.go`, `text_human.go` | `appendStateChangeJSON` and `appendStateChangeText` write `close-origin`, `close-notification` from PeerInfo, before `reason` (OD-1, review r2 #2); the `Format:` comment above `appendStateChangeText` names them |
| `internal/plugins/flowexport/interop_integration_linux_test.go` | sibling `TestFlowExportSFlowToolSamplePoolInterop` beside `TestFlowExportSFlowToolInterop` (unchanged): the per-sample pool delta and the wrap on a second source (Q-8, review r2 #6) |
| `test/interop-flowexport/Dockerfile.collectors` | the default command's `-test.run` pattern also selects `TestFlowExportSFlowToolSamplePoolInterop` |
| `internal/component/plugin/types_bgp.go` | `PeerInfo` gains the cause |
| `pkg/plugin/rpc/enums.go`, `pkg/plugin/rpc/bridge.go` | close-origin enum; two `StructuredEvent` fields (origin, NOTIFICATION body); `PutStructuredEvent` zeroes them |
| `internal/component/bgp/server/events.go` | `getStructuredStateEvent` copies them |
| `internal/component/bgp/plugins/bmp/bmp_events.go` | `peerDownFor` decides from the down event's origin and PDU; `cacheNotificationPDU` and the cache arm of `handleStructuredEvent` deleted (the mirror arm stays); `clearPeerState` comment rewritten; `fsmEventNone`'s comment rewritten to the Q-5 reason (stale comment removed, `ai/rules/stale-comments.md`); RFC 7854 Section 4.9 quotes above each arm |
| `internal/component/bgp/plugins/bmp/bmp.go` | `notifyCache` field, its comment and its `make` deleted; `notifyPDU` type deleted |
| `internal/plugins/flowexport/sflow/flow_adapter.go` | per-source pool, wrapping; the saturation comment replaced |
| `internal/le/interoplab/bgp/checkers.go`, `names.go` | the new BMP scenario's ops and needles |
| `docs/guide/bmp.md`, `docs/architecture/behavior/peer-lifecycle.md`, `docs/architecture/testing/interop.md`, `docs/architecture/api/process-protocol.md`, `docs/architecture/api/text-format.md`, `docs/architecture/api/ipc_protocol.md`, `docs/architecture/flowexport/flow-export-1-counter-export.md`, `docs/config-reference.md` | see Documentation checklist |
| `rfc/audit/rfc7854.json`, `rfc/audit/sflow-v5.json`, `rfc/discrimination/*.json` | written by the `./le rfc` tools only, never by hand |

## Files to Create

| File | Purpose |
|------|---------|
| `internal/component/bgp/reactor/session_close_cause_test.go` | the per-site cause table |
| `internal/component/bgp/plugins/bmp/rfc7854_peerdown_cause_test.go` | the tagged cause-to-reason tests |
| `test/interop/scenarios/bmp-peerdown-reason-pmacct/` | BMP interop |
| `test/plugin/bmp-sender-peer-down-reason.ci` | if A-7 holds |
| `test/plugin/state-down-carries-close-notification.ci` | AC-25, OD-1 for a JSON `state` subscriber |

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | no config surface |
| YANG validation constraints | No | none added |
| YANG custom validators | No | Q-2 answered by Q-1 "b": no refusal |
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
| 2 | Config syntax changed? | No syntax; Yes meaning | `docs/config-reference.md` "Process Bindings": the `state` row states that a down event carries the closing NOTIFICATION, built or received, and the `notification` row that the grant delivers NOTIFICATION message events, not the close cause (OD-1). Edited in the same change as the producer, never before it |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | `docs/guide/bmp.md`: a reason-by-cause table replacing "mapped reason code", stating that reason 1 and 3 need no `notification` grant (OD-1) and that a NOTIFICATION ze built but could not deliver is reason 1 (Q-3 "b"); the attach example keeps `notification` for Route Mirroring. sFlow: the flowexport guide page states the pool wraps |
| 7 | Wire format changed? | No | the BMP and sFlow formats are unchanged; values change |
| 8 | Plugin SDK/protocol changed? | Yes, additive | `docs/architecture/api/process-protocol.md`: the paragraph "For state events, `StructuredEvent.State` and `StructuredEvent.Reason` carry the data" names the two new fields (close origin, NOTIFICATION body), and the grant contract of OD-1: every `state` subscriber, structured, JSON or text, sees the closing NOTIFICATION without the `notification` grant. `docs/architecture/api/text-format.md`: the `<state-event>` BNF gains the two optional members before `[<reason>]`, and says a down event's `<reason>` runs to end of line (the producer writes `connection lost`, two words), which also repairs the existing `<reason> ::= "reason" <token>` claim; the State row of its event table gains the members in the same order. `docs/architecture/api/ipc_protocol.md` line 204 ("Down events include `\"reason\": \"...\"` field"): the down event also carries `close-origin`, `close-notification` when known |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | the verdicts move via `./le rfc audit-stamp`; RFC7854-x-10's `{single-polarity: positive; ...}` annotation becomes untrue and is corrected (row text owned by the verdict-fix spec; flag it there) |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/interop.md`: `bmp-peerdown-reason-pmacct` in its scenario list, and the sflowtool carrier, which its suite table does not list today (a doc gap the review found, 2026-10-14), as a row pointing at `TestFlowExportSFlowToolInterop` and its sibling `TestFlowExportSFlowToolSamplePoolInterop`; `docs/architecture/flowexport/flow-export-1-counter-export.md` "Independent collector tests": the sibling's pool delta and wrap assertions, its `source:` anchor, and the default command now selecting three tests |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/behavior/peer-lifecycle.md`: the close cause step on leaving Established |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | run `./le spec citation anchors spec plan/release/fixes/spec-bmp-sflow-export-rfc-defects.md` at implement start and name each |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/bmp.md` testing walkthrough (line 431, line 437: "Close FRR's BGP session and check that Peer Down carries ...") gains the expected reason |

### Discovery (`ai/rules/repo-maintenance.md`)
| Question | Answer |
|----------|--------|
| Where does an agent look first | `ai/CODE-TO-DOCS.md` maps `bmp_events.go` to `docs/guide/bmp.md`; the reason table lives there |
| What rule prevents regression | RFC7854-x-10 and RFC7854-4.9-1 tagged units with discrimination records, re-checked by `./le rfc check` |
| What registry prevents drift | none needed: no event-to-number mapping exists (Q-9 "iii"); the close record is the one source of the cause |
| What verification proves it | the unit tests above and `bmp-peerdown-reason-pmacct` |

## Implementation Steps

1. **Phase: Wiring** -- the typed cause from session to StructuredEvent
   - Tests: `TestPeerClosedCarriesTheSessionCloseCause`, `TestStructuredStateEventCarriesTheCloseCause`, `TestPutStructuredEventClearsTheCloseCause`, `TestStateChangeCarriesTheCloseCause`, `TestPeerDownCarriesTheNotificationWithoutTheGrant`
   - Files: `rpc/enums.go`, `rpc/bridge.go`, `types_bgp.go`, `server/events.go`, `format/text_json.go`, `format/text_human.go`, `peer_run.go`, `peer_derived_bindings.go`, `reactor.go`, `reactor_notify.go`, `reactor_api.go`; docs `process-protocol.md`, `text-format.md`, `ipc_protocol.md`, `config-reference.md` in the same change
   - Verify: red first, then the cause reaches the event
2. **Phase: Recording the close** -- the producers of OD-2
   - Tests: `TestSessionCloseRecordLocalSites`, `TestSessionCloseRecordRemote`, `TestSessionCloseRecordFirstWriterWins`, `TestTeardownOnClosedConnRecordsALocalClose`, `TestSessionCloseRecordDiesWithTheSession`
   - Files: `session.go`, `session_write.go`, `session_read.go`, `session_coalesce.go`, `session_connection.go`, `session_handlers.go`, `forward_pool_congestion.go`
3. **Phase: BMP reason** -- after Q-7 approval for the tagged edits
   - Tests: the new tagged pair, `TestPeerDownWithNoCloseCauseSaysSo`, the changed untagged and tagged units
   - Files: `bmp_events.go`, `docs/guide/bmp.md`, `peer-lifecycle.md`, `process-protocol.md`
4. **Phase: delete the BMP NOTIFICATION cache** -- `notifyCache`, `notifyPDU`, `cacheNotificationPDU` removed before `peerDownFor` reads the down event (no layering); `TestBMPHoldsNoNotificationCache`
5. **Phase: sFlow D2** -- per-source wrapping pool; `TestSFlowFlowSamplePoolTracksTotal` changed (Q-7), `TestSFlowSamplePoolPerSource`
6. **Phase: Interop** -- `bmp-peerdown-reason-pmacct` and `TestFlowExportSFlowToolSamplePoolInterop` (the carrier's sibling, run through the collector image's default command), with their red-then-green discrimination; the `state-down-carries-close-notification.ci` functional test
7. **Phase: verdicts** -- `./le rfc discriminate-record` per changed tag, rejudge by a different agent, `rfc/audit` updated by the tool

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | no local close reaches reason 4; no remote close reaches reason 2; reason 2 Data is always 00 00 (Q-9 "iii") |
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
- BMP should stay the only place a reason byte is chosen; the reactor reports facts (origin, PDU), not BMP vocabulary.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The reactor carries the closing NOTIFICATION PDU on the down event, built or received, delivered or not, in a per-Session close record written first-writer-wins before the I/O or event (Q-1 "b", owner 2026-10-13) | FSM callback carries the processed event (wrong for local closes, which fire Event 18, and racy) | the owner's answer; one recording site covers every built NOTIFICATION; reason 1 for a failed write (Q-3 "b") needs it; the grant contract changes by owner decision (OD-1) and the BMP cache is deleted |
| Carry it on `PeerInfo` then `StructuredEvent` | a new parameter on every `OnPeerStateChange` signature | `InitialReplay` precedent; the record reaches `PeerInfo` through the one reactor-internal close path (`peerLifecycleObserver.OnPeerClosed` in `reactor.go`, `callbackAdapter`, `apiStateObserver`, test observers), and every `OnPeerStateChange` consumer, `registry.PeerLifecycleCallback` and `mrt` stay unchanged |
| No event number anywhere; reason 2 always writes 00 00 (owner, 2026-10-14, Q-9: "iii", superseding "a") | (i) keep the table with a writer reached only by the paused-cancel race; (ii) record in `waitForResume`'s ctx arm too, leaving the field test-only | the owner's answer, on the corrected premise that the only production writer was that race. The nil-connection branches keep a local, no-PDU record, because each still fires its FSM event and the down event must carry an origin |
| Every `state` subscriber gets the close cause, JSON and text included (OD-1) | narrow OD-1 to structured (DirectBridge) subscribers | the owner's words, "state subscribers see the closing NOTIFICATION without the notification grant", carry no transport limit; the members are additive and optional, so no parser breaks |
| D2 interop extends `TestFlowExportSFlowToolInterop` (owner, 2026-10-14, Q-8: "Extend the existing test") | a new scenario `sflow-sample-pool-sflowtool` (the superseded 2026-10-13 answer) | the carrier exists with sflowtool pinned; it sets `FlowSample.Rate` directly, so it can also cross 2^32, which a lab scenario could not |
| sFlow per-source accumulated pool | sequence times rate modulo 2^32 | correct when the rate changes for a source, and resets with the sequence number as the sFlow note requires; owner decides (D2) |

## Known Limitations

- The textual state reason stays `connection lost` for every Established exit (Q-6 a); `session closed` is dead on that path. Journal row 2026-10-13 in `plan/journal/constant-reported-as-measured-state.md`.
- The Peer Down for a `Run` exit with no FSM event (`read body`, a non-EOF non-reset header error) depends on `spec-bgp-rs-replacement-on-withdrawal` Phase 2b; until it lands such a drop publishes no down event at all, and this spec only records its cause.
- A BMP sender run as an external process receives no structured events (A-4); the JSON members of OD-1 carry the cause, but the BMP sender does not read JSON.

## Design Review Log

| Date | Round | Outcome |
|------|-------|---------|
| 2026-10-13 | owner answers | owner answers applied 2026-10-13; review round owed (Q-1 PDU on down event, Q-8 scenario). Q-1 "b", Q-2 by Q-1, Q-3 "b", Q-4 "a", Q-5 "a", Q-6 "a", Q-8 "a" recorded; OD-1 and OD-2 added; Q-9 raised from the producer (no reachable reason 2 with a non-zero event). The round owes: OD-2's four producers and their order against `-race`; the deletion of `TestPeerDownDropsTheNotificationWithTheSession` and the move of its guarantee; the sflowtool sidecar and suite placement |
| 2026-10-14 | review of the owner-answer edits (0 BLOCKER, 10 ISSUE, 9 NIT; `tmp/session/2026-10-12-0c3648c8-b9d7-45d8-b3f0-2473bceefb25/scratch/review-bmp-owner-answers.md`) | Applied. #1 Q-9 "a" recorded, every (b)-conditional removed; #2 the event field has a production writer, the local closers' nil-connection branches (OD-2, KDD, Known Limitations, AC-24, `TestTeardownOnClosedConnRecordsItsStopEvent`); #3 Q-9 evidence corrected (`Run`'s deferred close and the cancel goroutine's panic recovery record nothing; `sendHoldTimerExpired`); #4 the `Run` exits with no FSM event record remote here, and their down event is `spec-bgp-rs-replacement-on-withdrawal` Phase 2b, named in Depends (AC-1, AC-23, R-10), not duplicated; #5 local retirements classified at the site (AC-9); #6 `Reactor.Stop` is reason 1, split from `StopForRestart` (AC-22); #7 AC-20 rewritten as a version-error NOTIFICATION, unreachable unpack rationale dropped; #8 OD-1 kept for every `state` subscriber, JSON and text members designed ("State event wire shape", AC-25, doc row 8); #9 Q-8 re-asked on the corrected premise, owner answered 2026-10-14 "Extend the existing test" (AC-26); #10 `TestHandleSenderStatePeerDownCarriesTheNotificationZeSent` CHANGED row; NITs 11-19 applied (OD-2 drainer wording, `handleNotification` in `session_handlers.go`, Cease record size, `reactor.go` observer and no mrt change, Depends and R-8 committed as dce906c381, Q-9 (a) "add", BMP copies the body, the BMP half of the moved guarantee, AC and R order). Next round owed, narrow: the Q-9 reachable writer (nil-connection branches against first writer wins, and that no recorded closer makes them unreachable again); the JSON and text members (shape, a multi-word text `reason` before `close-origin`, doc rows); the Q-8 carrier extension (second source, wrap assertion, discrimination) |
| 2026-10-14 | review round 2, narrow (0 BLOCKER, 2 ISSUE, 7 NIT; `tmp/session/2026-10-12-0c3648c8-b9d7-45d8-b3f0-2473bceefb25/scratch/review-bmp-r2.md`) | Applied; **Q-9 pending owner**. #1 the Q-9 writer's production window corrected at the producers to the paused-cancel race (`waitForResume` returns `ctx.Err()`, `Run`'s deferred `closeConn` runs before the cancel goroutine records), "panic recovery" dropped as a production shape (Q-9 row, OD-2, KDD, Known Limitations, AC-24, the `TestTeardownOnClosedConnRecordsItsStopEvent` row, A-1, R-1, Q-4, close-path table, Wiring row); Q-9 re-asked with options (i) keep with the race-only writer, (ii) record in `waitForResume`'s ctx arm, (iii) drop the table, not chosen. #2 text members written before `reason`, so a down event's reason ends the line; doc row 8 BNF says a down `<reason>` runs to end of line; journal row 2026-10-14 in `plan/journal/declared-format-contradicts-payload.md` for the pre-existing BNF/producer mismatch. NITs: #3 Q-8 cites AC-26; #4 sflowtool decodes, the delta is the test's; #5 the wrap tests use rate 2^31 over three samples (AC-18, AC-26, boundary, discrimination); #6 sibling `TestFlowExportSFlowToolSamplePoolInterop` and its run route through the collector image's default command (`Dockerfile.collectors` pattern widened); #7 no-Data text form omits `data`; #8 member placement reworded (no text `unheld-roles`, no down `initial-replay`); #9 OD-2 names the nil-conn asymmetry as intended. Next round owed after the Q-9 answer |
| 2026-10-14 | owner Q-9 "iii" | owner Q-9 iii applied 2026-10-14; narrow review owed. "iii" supersedes "a": the event-number table, the record's event, the `StructuredEvent` event field and the `close-event` member are dropped (no value left to carry); reason 2 always writes 00 00; Q-4 marked superseded (00 1F never sent) and its draft Event 31 / Section 13.3 citations removed; AC-16 and R-4 withdrawn, the boundary row and `TestEventRFCNumbers` deleted, the keep's KDD row replaced and the Known Limitations "deliberate keep" line deleted. Kept, after reading `teardown`, `CloseWithNotification`, `sendHoldTimerExpired` and the hold-expiry arm at the producers: each nil-conn branch still fires its FSM event, so it still records local, no PDU, or the down event arrives Unspecified (AC-12 WARN). So AC-24 is rewritten to that record and reason 2 with 00 00, and the test is renamed `TestTeardownOnClosedConnRecordsALocalClose`. The narrow review checks: that every nil-conn recorder is local with no PDU, AC-5/AC-24/AC-25 against the two-member wire shape, and that no `close-event`, event number or 00 1F remains |

## RFC Documentation (Scope: protocol)

Add `// RFC 7854 Section 4.9: "<quoted reason text>"` above each `peerDownFor` arm (the reason 1 quote "that would have been sent to the peer" above the arm a refused write reaches), the same quote above the recording in `sendNotificationWithin` and the reason 3 quote above the recording in `handleNotification`, the reason 2 quote ("Two bytes both set to 0 are used to indicate that no relevant Event code is defined") above the `peerDownFor` arm that writes 00 00 (Q-9 "iii"), the reason 2 quote ("The local system closed the session.  No notification message was sent.") above each nil-connection and Q-5 recording, the reason 4 quote ("This includes any unexpected termination of the transport session") above the `Run`-exit remote recordings, and the sFlow v5 `sample_pool` and `sequence_number` note above the pool arithmetic.

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
- [ ] AC-1..AC-26 all demonstrated (AC-23's Peer Down once `spec-bgp-rs-replacement-on-withdrawal` Phase 2b has landed)
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
| D1: reason 2 carries the FSM event field (always 00 00, Q-9 iii) | `TestRFC7854PeerDownReasonTwoCarriesItsEventCode` decode, discrimination record |
| D2: the pool keeps advancing past 2^32 | AC-18 decoded datagram; AC-26, `TestFlowExportSFlowToolSamplePoolInterop` in the extended carrier, for the decode and the wrap by a third party, with its recorded red |
| OD-1: every `state` subscriber sees the closing NOTIFICATION | `state-down-carries-close-notification.ci` (JSON, `state` grant only) and `TestPeerDownCarriesTheNotificationWithoutTheGrant` |

### TDD
- [ ] Tests written
- [ ] Tests FAIL (before the fix)
- [ ] Tests PASS (after the fix)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
- [ ] **Commit A** and **Commit B** per `ai/rules/planning.md`

### Verification
- [ ] `./le verify worktree`
