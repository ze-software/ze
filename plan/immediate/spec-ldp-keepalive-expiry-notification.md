# Spec: ldp-keepalive-expiry-notification

<!-- DESIGN-TIME template: everything that must exist BEFORE code is written.
     The closure half (Implementation Summary, Audit, Goal Validation, Review
     Gate, Pre-Commit Verification, Mistake Log) lives in
     plan/TEMPLATE-CLOSURE.md and is APPENDED by /ze-close at step 1.
     Do not copy it in advance: sections copied 300 lines ahead of their use
     reach closure untouched, the ones created when needed get filled. -->

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-14 |

<!-- Handoff: `verify` splits the work over two sessions -- the implementation session commits and stops at Status `verification`, a later session reviews that commit and closes, on Opus when the model is an Anthropic one. `-` closes in the same session. -->

<!-- Scope drives which optional blocks below apply. Say which one this is, so
     an absent section reads as "inapplicable" rather than "skipped".
     The file's DIRECTORY carries the release bucket: plan/immediate/ for a defect
     an operator meets, plan/pre-release/ for work the release cannot go out
     without, plan/ for everything else (plan/README.md). -->

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Send the fatal LDP Notification for KeepAlive Timer expiry before closing the
transport, whether or not any optional liveness monitor is configured. This
spec receives the existing unconditional AC-8 and the best-effort wire-failure
portion of AC-9 from `plan/spec-liveness-event-tears-down-bgp-peer.md` during
the 2026-09-19 planning reconciliation. It authorises no additional feature.

RFC 5036 Sections 2.5.4, 3.5.1.1 and 3.5.1.2.3 require a fatal Notification
carrying the KeepAlive Timer Expired Status Code: Status Data 0x00000014 with
the E bit set. Section 2.5.6 requires only the close. Section 2.5.4 uses
Shutdown generically for a fatal Notification; this obligation must not be
changed to the distinct Shutdown status code 0x0000000A. Preserve notification
before transport close and the owner's best-effort write decision: use the
existing write deadline, log a failed send once and still close the session.

Current source evidence is `internal/plugins/ldp/session.go`, where
`Session.ReadLoop` returns `errKeepaliveExpiry` on its read timeout, and
`internal/plugins/ldp/register.go`, where `runSession` logs the error and its
deferred `Session.Stop` closes the transport. This is a source-confirmed missing
send, with no runtime verification claimed by this skeleton. Design must read
the current wire encoder, session lifecycle and RFC before implementation.

This spec owns the encoder/send path and its discriminating unit, functional
and independent-peer wire proof, including the no-monitor case and send
failure. The optional liveness event, monitor configuration and BGP teardown
remain in the originating spec and must consume this behaviour without gating
it. The decode-failure half of RFC5036-2.5.3-2 and other LDP Notification causes
remain outside this transferred scope; their obligations are not waived.

### Research status (2026-10-13)

| Check | Result |
|-------|--------|
| Another session on this spec | None. No `session-state-*ldp-keepalive*` file under `tmp/session/*/state/`, and no commit in `git log` names it. The only other references are the transfer rows in `plan/spec-liveness-event-tears-down-bgp-peer.md` |
| Defect still open at HEAD (242f64d2a4) | Yes. `Session.ReadLoop` (`internal/plugins/ldp/session.go`) returns `errKeepaliveExpiry` from its read-timeout branch with no write; `runSession` (`internal/plugins/ldp/register.go`) logs `ldp: session ended` and its deferred `sess.Stop` closes the connection. No Status Code `0x00000014` constant exists in `wire.go`. Last LDP commits (`7c25971594`, `e47919016b`, `8ca6178596`, `0bde29b2db`) touch other paths |
| `rfc/short/rfc5036.md` | RFC5036-2.5.3-2 (MUST) and RFC5036-3.5.1-1 (SHOULD) are `{gap: partly met ...}` and name "KeepAlive expiry" as a remaining cause, saying this spec "owns only the expiry cause" |
| `rfc/audit/rfc5036.json` | 26 audited requirements; neither RFC5036-2.5.3-2 nor RFC5036-3.5.1-1 has a verdict. No tagged test names either id (`grep -rn` over `internal/` finds only the quote backfill list in `internal/le/rfc/quote_backfill_test.go`) |
| `./le rfc check` | Exits 2 with two violations on RFC4456-8-1 (SHIFTED audit verdict) and RFC9514 rows, both other sessions' work. No line names rfc5036 |
| Skeleton wording corrected | The skeleton Task said "Sections 2.5.6 and 3.5.1.2.3 require" the code, and the Task now names Sections 2.5.4, 3.5.1.1 and 3.5.1.2.3 (design review r1, N-8). Section 2.5.6 does not require it: it says the LSR "terminates the LDP session by closing the transport connection" and names no Notification. The obligation comes from Sections 2.5.4, 3.5.1.1 and 3.5.1.2.3, quoted under RFC Text below |

### RFC text (read in `rfc/full/rfc5036.txt`)

| Section | Whole sentence quoted | What it fixes in the design |
|---------|----------------------|-----------------------------|
| 2.5.4 | "Note that a Shutdown message is implemented as a Notification message with a Status TLV indicating a fatal error." | "Shutdown" in the FSM table is any fatal Notification, not Status Code 0x0000000A |
| 2.5.4 (table, OPERATIONAL row) | "Timeout NON EXISTENT Action: Transmit Shutdown msg and close transport connection" (table row, cells joined) | Expiry in OPERATIONAL owes a fatal Notification, then close |
| 2.5.4 (diagram) | The INITIALIZED, OPENREC and OPENSENT edges read "Rx Any other msg or Timeout / Tx NAK msg" | A timeout during initialization also owes a fatal Notification (owner question Q-2 settles which code) |
| 2.5.6 | "If the KeepAlive Timer expires without receipt of an LDP PDU from the peer, the LSR concludes that the transport connection is bad or that the peer has failed, and it terminates the LDP session by closing the transport connection." | Close stays; the Notification precedes it |
| 3.5.1.1 | "If an LSR encounters a condition requiring it to notify its peer with advisory or error information, it sends the peer a Notification message containing a Status TLV that encodes the information and optionally additional TLVs that provide more information about the condition." | The RFC5036-2.5.3-2 row's source |
| 3.5.1.1 | "In this case, after sending the Notification message the LSR SHOULD terminate the LDP session by closing the session TCP connection and discard all state associated with the session, including all label-FEC bindings learned via the session." | Order: Notification first, close second (RFC5036-3.5.1-1) |
| 3.5.1.2.3 | "This is a fatal error signaled by the KeepAlive Timer Expired Status Code." | The code is KeepAlive Timer Expired |
| 3.9 | "The "E" column is the required setting of the Status Code E-bit; the "Status Data" column is the value of the 30-bit Status Data field in the Status Code TLV." Table row: "KeepAlive Timer 1 0x00000014 ... Expired" | Status Code on the wire is 0x80000014 (E set, F clear) |
| 3.9 | "Note that the setting of the Status Code F-bit is at the discretion of the LSR originating the Status TLV." | F clear, as for every status ze sends today |
| 3.4.6 | "If zero, no specific peer message is being identified." and "If zero, the Status TLV does not refer to any specific message type." | Expiry answers no message: Message ID 0 and Message Type 0 |
| 3.5.3 | "The value chosen for KeepAlive Time indicates the maximum number of seconds that may elapse between the receipt of successive PDUs from the LDP peer on the session TCP connection." | Ze's timer is three times this (owner question Q-1, journal row in `plan/journal/paraphrase-becomes-the-authority.md`) |

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress.
     Capture what you learned as -> Decision: / -> Constraint: annotations, which
     survive compaction; track reading progress in the session state file. -->

### Architecture Docs
- [ ] `docs/architecture/ldp/mpls-ldp.md` - the `// Design:` page of `session.go`; records the Notification decisions
  → Decision: every Notification ze sends goes through `Session.sendNotification`, bounded by `notificationWriteTimeout` (5 s) and clearing the deadline after a successful write. A failed fatal Notification is logged and the connection still closes ("Decision: the establishment KeepAlive answers the peer's Initialization" section)
  → Constraint: the page's sentence "Other fatal causes, including KeepAlive expiry and decoding errors outside the initialization type checks, still close silently" becomes false for expiry and MUST be edited in the same phase as the code (`ai/rules/documentation.md`); decoding errors stay named
- [ ] `docs/guide/mpls.md` - operator page for `ldp/keepalive-time`
  → Constraint: it says ze "closes the session when no PDU arrives inside three times that value"; this spec adds that ze first sends the KeepAlive Timer Expired Notification. The three-times wording changes only if owner question Q-1 folds that fix in
- [ ] `docs/architecture/testing/interop.md` - interop suites, scenario naming, discrimination
  → Constraint: a scenario directory is named, no numeric prefix; LDP has no suite of its own. Scenarios live in the BGP lab (`test/interop/scenarios/`, checkers in `internal/le/interoplab/bgp/`, run by `./le test integration interop`)
  → Constraint: no lab scenario has ever brought up an LDP session (design review r1, I-3). `ospf-ldp-sync-frr` configures `mpls ldp` in `frr.conf`, but the shared `test/interop/daemons` sets `ldpd=no`, the scenario ships no `daemons` override, and its `checkers.go` entry waits only for OSPF Full. `prepare.go` mounts a scenario's own `daemons` file in place of the shared one when it exists, so the new scenario ships one with `ldpd=yes` (A-6). Journal row: `plan/journal/test-against-broken-path.md`, 2026-10-14
  → Decision: the precedent for forcing a timer expiry against FRR is `check_special.go` (BFD failover): an nft table in ze's own container drops ze's INBOUND traffic, because the FRR image ships no iptables. The same shape drops FRR's TCP toward ze, so ze's KeepAlive Timer expires while ze's outbound Notification still reaches FRR
  → Constraint: before claiming the interop test validates the change, revert, rebuild the image, see it red, restore, see it green, and record the red

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc5036.md` - rows RFC5036-2.5.3-2, RFC5036-3.5.1-1, Support remaining, and the prose at its implementation notes
  → Constraint: both rows are ordinary `{gap}` rows. `docs/contributing/rfc-conformance-gates.md` "Clause-scoped evidence": "An ordinary `{gap}` still forbids polarity tags", and `{partial}` selects a span of the quote, never a cause. The decode-failure cause stays open, so neither row can carry this spec's tests as tags; their gap text narrows to drop the expiry cause, nothing more
  → Constraint: RFC scope is frozen (owner, 2026-10-08): no new summaries. Whether a new row for the 2.5.4 OPERATIONAL Timeout transition counts as fixing an in-scope summary is owner question Q-3
  → Constraint: `rfc/extraction/rfc5036.json` records section 2.5.4 as walked with unsourced-ids RFC5036-2.5.4-1 and -2 only, and 3.5.1.2.3 as walked with 0 sites

**Key insights:** (minimal context to resume after compaction)
- The encoder and the bounded writer already exist (`encodeNotification`, `sendNotification`); `rejectInit` is the log-once, close-anyway pattern. The change is one new status constant and one call on the timeout branch of `ReadLoop`, plus the ordering guarantee below
- The KeepAlive sender goroutine in `runSession` writes on the same connection with no lock, so today a KeepAlive can follow a fatal Notification before `sess.Stop` closes the socket. The Notification must be the last PDU
- `SendKeepalive`, `SendInit`, `SendLabelMapping` and `SendLabelWithdraw` write with no write deadline; only `sendNotification` arms one. Today no Go lock exists, so `sendNotification`'s `SetWriteDeadline` also unblocks a KeepAlive write that is stuck on a peer that stopped reading. A write lock that spans `conn.Write` would remove that, which is why D-3 fixes the lock order
- No existing test drives `ReadLoop` to expiry, so no existing RFC-tagged test changes for the Notification itself
- The KeepAlive Timer is 3x the negotiated KeepAlive Time, against RFC 5036 Section 3.5.3 (Q-1)

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/plugins/ldp/session.go` (~800L) - Session struct, FSM, ReadLoop, processMessages, rejectInit, sendNotification. Key: `ReadLoop` sets `SetReadDeadline(now + currentHoldTime())` before each header read and maps a `net.Error` timeout to `errKeepaliveExpiry`, in every state (OPENSENT, OPENREC, OPERATIONAL). `holdTime` starts at `3 * keepalive` in `NewSession` and becomes `3 * negotiatedKA` in `handleInit`, under the comment "RFC 5036: hold time is typically 3x keepalive". `sendNotification(status, referMsgID, referMsgType)` sets a 5 s write deadline, encodes into a 64-byte stack buffer, writes, then clears the deadline. `rejectInit` logs a failed send at Warn and returns the cause. `processMessages` already sends fatal `statusShutdown` for an unexpected message in OPENSENT/OPENREC, logging a failure once. `Stop` closes `stopCh`, closes the conn and sets NON EXISTENT, once
- [ ] `internal/plugins/ldp/wire.go` - status constants (`statusFatalBit`, `statusBadProtocolVersion`, `statusUnknownTLV`, `statusShutdown` 0x8000000A, `statusLoopDetected`, `statusSessionRejectedNoHello`, `statusSessionRejectedBadKeepaliveTime`), `encodeNotification` writing Message header + one Status TLV (Message ID, Message Type), `decodeNotification`
- [ ] `internal/plugins/ldp/register.go` - `runSession` (only caller of `ReadLoop`): SendInit, a KeepAlive sender goroutine that waits for `accepted` then writes every `currentKeepalive()/3` via `SendKeepalive` with no write lock, the ReadLoop callbacks, then `log.Info("ldp: session ended", "reason", err)`; deferred `sess.Stop()` and `onDone()`. `expireAdjacencies` stops sessions on Hello adjacency expiry (no Notification; outside this spec)
- [ ] `internal/plugins/ldp/rfc5036_test.go`, `rfc5036_establishment_test.go`, `rfc5036_procedures_test.go`, `rfc5036_openrec_test.go`, `rfc5443_sessionup_test.go`, `keepalive_test.go` - silence probes use windows of 100 to 300 ms against a 60 s KeepAlive; none reaches the 180 s expiry, so none changes. `TestRFC5036InitNonZeroKeepaliveTimeAccepted` (RFC5036-3.5.3-1 positive) asserts `currentHoldTime() == 90s`; `TestConfiguredKeepaliveReachesInitializationMessage` asserts 45 s
- [ ] `internal/plugins/ldp/frr_interop_integration_linux_test.go` - `TestLDPInteropFRR`, QEMU-run FRR ldpd peer in a netns; establishes a session and decodes a Label Mapping
- [ ] `internal/le/interoplab/bgp/check_special.go`, `checkers.go`, `check_extras.go`, `names.go` - BFD failover nft drop precedent; `ospf-ldp-sync-frr` checker; op kinds `opExec`, `opWaitLogContains`, `opWaitContains`
- [ ] `test/ldp/*.ci` - four `.ci` tests; `ldp-session.ci` states a live peer "needs a mock LDP peer", which no harness provides

**Behavior to preserve:** (unless the user explicitly said to change it)
- `ReadLoop` still returns `errKeepaliveExpiry` on expiry, so `runSession` logs `ldp: session ended ... reason=ldp: keepalive timer expired` and the liveness spec can key on that error
- The session closes and the peer's bindings are withdrawn whether or not the Notification write succeeds (owner's best-effort decision)
- The write stays bounded by `notificationWriteTimeout`; no new timer, no new config leaf
- Every other Notification (rejectInit codes, Shutdown NAK, Unknown TLV, Loop Detected) keeps its code, references and log lines
- No `monitor`/liveness configuration gates the send

**Behavior to change:** (only what the user asked for)
- On a `ReadLoop` read timeout, ze sends one Notification, Status Code 0x80000014 (KeepAlive Timer Expired, E set, F clear), Message ID 0, Message Type 0, before the transport closes
- A failed send is logged once at Warn and the session still closes
- No PDU follows that Notification on the connection (the KeepAlive sender cannot write after it)

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Silence on the LDP session TCP connection: no PDU from the peer for the read deadline that `ReadLoop` arms from `currentHoldTime()`
- Format at entry: a `net.Error` with `Timeout() == true` from `io.ReadFull` on the PDU header

### Transformation Path
1. `Session.ReadLoop` read-timeout branch classifies the event as KeepAlive Timer expiry
2. `Session.sendNotification` (bounded write) with the new status constant, references zero, encoded by `encodeNotification` into the existing stack buffer
3. On write failure: one Warn log line naming the status and error
4. `ReadLoop` returns `errKeepaliveExpiry`; `runSession` logs `ldp: session ended`; deferred `Session.Stop` closes the connection; existing peer-down teardown withdraws bindings

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| ze ↔ LDP peer (TCP 646) | One LDP PDU: header + Notification message + Status TLV, RFC 5036 Section 3.5.1 | No (AC-1, AC-6) |
| Session ↔ runSession | Returned error `errKeepaliveExpiry`, unchanged | No (AC-4) |

### Integration Points
- `sendNotification` - reused unchanged for the write
- `statusFatalBit` constant block in `wire.go` - gains KeepAlive Timer Expired beside the existing codes
- KeepAlive sender in `runSession` - must not write after the fatal Notification (AC-5)
- `plan/spec-liveness-event-tears-down-bgp-peer.md` - consumes this behaviour; its AC-8 and AC-9 cite this spec

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes (design) | Write goes through `sendNotification`, the one bounded Notification writer |
| No unintended coupling (components stay isolated) | Yes (design) | Change stays inside `internal/plugins/ldp` |
| No duplicated functionality (extends existing, does not recreate) | Yes (design) | No second encoder; one constant added to the existing block |
| Zero-copy preserved where applicable (refs, not copies) | Yes (design) | Stack buffer `[64]byte` in `sendNotification`; no heap allocation added |
| Registration over hardcoding, outbound | N-A | No command, view, family or handler is added |
| Registration over hardcoding, inbound | N-A | The feature introduces no name any switch, parser, validator or table must learn; searched: status constant block (`wire.go`) is a protocol code table, not a feature registry |

## Risks & Assumptions

<!-- LIVE: written during RESEARCH/DESIGN, statuses updated during implementation.
     Gate answers from /ze-spec (assumption challenge, Failure Mode Analysis)
     land HERE, not only in conversation. -->

### Assumptions
<!-- Every row needs a validation method. `unvalidated` is not a valid final
     status: closure re-checks each one. A broken assumption also gets a
     Mistake Log row and a Deviations entry. -->
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `ReadLoop` is the only place a KeepAlive Timer expiry is detected | `grep` for `errKeepaliveExpiry` finds one return, in `ReadLoop`; `runSession` is the only `ReadLoop` caller (passive role absent) | A second detection site closes silently | Grep at implementation start; a future passive-role session reuses `ReadLoop` | unvalidated |
| A-2 | Concurrent `Write` calls on one TCP `net.Conn` do not interleave bytes | Go `net` documents conn methods as safe for concurrent use; the poll FD write lock serialises each `Write` | A KeepAlive could split the Notification PDU | AC-5 mechanism serialises all session writes, which removes the dependency | unvalidated |
| A-3 | FRR ldpd records a received fatal Notification with its status name ("KeepAlive Timer Expired") where the lab can read it, given `debug mpls ldp messages recv` (or equivalent) in `frr.conf` | Not verified against FRR source in this session | The interop checker cannot tell the code from any other close | Run the scenario once at implementation; fallback: capture the PDU on FRR's interface (packet capture in ze's container on egress) and assert the Status TLV bytes | unvalidated |
| A-4 | Dropping FRR's TCP toward ze in ze's container input hook expires ze's timer while ze's outbound Notification still reaches FRR, and Hellos (UDP) keep the adjacency alive so `expireAdjacencies` does not stop the session first | BFD failover precedent in `check_special.go` drops one direction only | The adjacency expiry path closes without a Notification and the test proves nothing | Scenario run plus forced red under revert (AC-6) | unvalidated |
| A-5 | Zero Message ID and Message Type are right for expiry | RFC 5036 Section 3.4.6 quotes above; expiry answers no peer message | FRR rejects the Status TLV | Interop AC-6 | unvalidated |
| A-6 | FRR ldpd, started by a scenario `daemons` file with `ldpd=yes`, reaches OPERATIONAL with ze in the Docker lab | Nothing yet: no lab scenario has started ldpd (journal row 2026-10-14 in `plan/journal/test-against-broken-path.md`); the QEMU `TestLDPInteropFRR` establishes a session in a netns, which is a different topology. `prepare.go` mounts the scenario's `daemons` over `/etc/frr/daemons` | AC-6 has no session to expire; the interop proof needs a lab repair first, and the owner is told before any other route is chosen | First step of phase 5: run the scenario with only the OPERATIONAL wait in its checker (`vtysh -c "show mpls ldp neighbor"` showing the ze LSR ID in state OPERATIONAL), before any nft step is written | unvalidated |
| A-7 | Arming the write deadline unblocks a `conn.Write` already pending in another goroutine | Go `net.Conn` `SetWriteDeadline` documents that a deadline applies to pending as well as future writes, and `net.Pipe` implements deadlines | The expiry Notification waits on the write lock without bound (AC-4b) | AC-4b over `net.Pipe` with a KeepAlive blocked mid-write | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The KeepAlive sender writes a KeepAlive after the fatal Notification and before `Stop` closes the socket | AC-5 test sees a PDU after the Notification | One session write path refuses writes once a fatal Notification has been sent (Design, decision D-3) |
| R-2 | The peer has stopped reading, so the Notification write blocks | Session close delayed | Existing `notificationWriteTimeout` (5 s) bounds it; AC-4 proves the session still closes |
| R-3 | The Notification write fails (peer reset, socket error) while the read side still times out | Warn log | Logged once, error returned stays `errKeepaliveExpiry` (AC-3); `runSession`'s deferred `Stop` closes the session (wiring test) |
| R-8 | A KeepAlive write blocked on a peer that stopped reading holds the write lock, so the expiry Notification waits on the lock rather than on `notificationWriteTimeout`; `ReadLoop` never returns, `runSession`'s deferred `Stop` never runs, and the KeepAlive sender goroutine leaks (design review r1, I-1) | AC-4b exceeds its bound or hangs | D-3 lock order: the expiry path arms the write deadline BEFORE it takes the write lock, so the deadline fails the pending write and the lock is released inside the same budget |
| R-9 | A short write, meaning a write that returns 0 < n < len(PDU): a KeepAlive write that the armed deadline fails after writing part of its PDU leaves the stream mid-PDU, so the Notification would be read as the tail of a malformed PDU. A write that returns n = 0 is not short and leaves the stream intact; on `net.Pipe` the blocked KeepAlive of AC-4b returns n = 0 with the timeout, so AC-4b exercises the intact-stream branch | AC-4b sees a short KeepAlive write | D-3: a short write by any writer marks the stream broken; the Notification is then not written, the Warn line names the cause, and the session closes |
| R-10 | An advisory Notification's deadline fails a blocked KeepAlive, the KeepAlive sender exits, and a session that stays up sends no further KeepAlives | `TestAdvisoryNotificationBoundedBehindBlockedKeepalive` sees the sender exit | Accepted as a known gap that predates this spec, unchanged from HEAD, where the same deadline already fails the pending KeepAlive. The obligation the dead sender stops meeting is RFC 5036 Section 2.5.6: "an LSR must arrange that its peer receive an LDP PDU from it at least every KeepAlive time period" (lowercase must, so not tagged). It is accepted because the peer's own KeepAlive Timer ends the session, and a peer that stopped reading makes the advisory itself fail at its deadline, which ends `ReadLoop` (D-3) |
| R-4 | The cleared write deadline after a successful fatal send lets a later write block forever | Hang in teardown | After a fatal Notification no further write is allowed (D-3), so nothing runs without a deadline |
| R-5 | A unit test through `runSession` waits for a real expiry (3 s minimum at KeepAlive 1 s today) | Slow test | Drive `ReadLoop` directly with a short `holdTime` for the fast cases; keep one wiring test through `runSession` |
| R-6 | The liveness spec's monitor work lands in the same files at the same time | Conflicting hunks in `session.go`/`register.go` | That spec consumes this one and adds no send path; check `git status` before editing |
| R-7 | The interop scenario passes vacuously because FRR logs a close for any reason | Green under revert | Assert the status name or the Status TLV bytes, and record a red under revert (`./le rfc discriminate-record` does not apply to an untagged scenario; record in the spec's Goal Validation) |

## Blast Radius

<!-- What a wrong landing costs, and how to get out. A reviewer reads this first. -->
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A session that was already ending gains or loses one PDU on the wire. A bug in the ordering guard could block a KeepAlive on a live session, which would drop live LDP sessions |
| How is it reverted? | Single commit revert; no config, no stored state |
| Who else touches this path? | `plan/spec-liveness-event-tears-down-bgp-peer.md` (consumer); `plan/spec-ldp-label-request-path.md` edits `processMessages`; the decode-failure half of RFC5036-2.5.3-2 is unowned |

## Design

### Alternatives

| # | Approach | Gains | Costs |
|---|----------|-------|-------|
| 1 (recommended) | `ReadLoop`'s timeout branch sends the Notification through `sendNotification`, logs a failure once, returns `errKeepaliveExpiry`. Session writes are serialised and refuse after a fatal Notification | The session that detects the expiry also reports it; any future `ReadLoop` caller (passive role) inherits it; mirrors `rejectInit` | Touches the write path every sender uses |
| 2 | `runSession` matches `errKeepaliveExpiry` after `ReadLoop` returns and sends the Notification there | `ReadLoop` stays read-only | Second place that knows Notification policy; a future caller of `ReadLoop` forgets it; the KeepAlive sender is still running at that point |
| 3 | Stop the KeepAlive sender (cancel its context) before sending, instead of a write guard | No change to the write path | Covers only `runSession`'s sender; other fatal paths (Shutdown NAK in OPENREC after `accepted`) keep the race; cancellation does not wait for an in-flight write |

### Decisions

| ID | Decision | Rejected | Rationale |
|----|----------|----------|-----------|
| D-1 | Status Code KeepAlive Timer Expired, 0x80000014, as a named constant in the `wire.go` block | Shutdown 0x8000000A | Section 3.5.1.2.3 names the code; Section 2.5.4 says Shutdown is the generic fatal Notification |
| D-2 | Message ID 0, Message Type 0 | Echo the last received message | Section 3.4.6: zero means no specific peer message; expiry answers none |
| D-3 | All session PDU writes go through one serialised path that refuses any write after a fatal (E-bit) Notification has been written or attempted; the refusal is an error to the caller, not a silent drop. Lock order for EVERY `sendNotification`, fatal and advisory alike (the advisory Unknown TLV from `ignoreUnknownTLV` and Loop Detected from `processMessages`, both on the `ReadLoop` goroutine, included), in this order: (1) arm the write deadline on the conn at now plus `notificationWriteTimeout`, (2) take the write lock, (3) set the fatal flag when the E bit is set, (4) write, (4b) an advisory clears the deadline as `sendNotification` does at HEAD, before release, (5) release. The clear comes before the release so the order is obvious; it would be safe after it too, because the armed deadline is still in the future and every `sendNotification` caller runs on the `ReadLoop` goroutine, so the clear can never undo a fatal arm. Step 1 precedes step 2 because the deadline also fails a write already pending under the lock (A-7): a KeepAlive blocked mid-write returns a timeout error and releases the lock, so the lock wait and the Notification write share one `notificationWriteTimeout` budget. The four non-Notification writers (`SendKeepalive`, `SendInit`, `SendLabelMapping`, `SendLabelWithdraw`) never touch the deadline, so none can clear the armed one; advisory Notifications arm and clear it, but they are sent only from the `ReadLoop` goroutine, the same goroutine as the expiry, so they never run concurrently with it. An advisory's deadline can fail a KeepAlive blocked under the lock, exactly as `sendNotification`'s `SetWriteDeadline` already does at HEAD with no lock: the KeepAlive sender in `runSession` (`register.go`) returns on that send error, and the session goes on without KeepAlives, as at HEAD. That is accepted and not changed here (risk R-10): the advisory itself is then either written, because the peer is draining, and the peer's own hold timer, receiving nothing more from ze, ends the session within its KeepAlive Time; or it fails at its own deadline, which ends `ReadLoop`, because both advisory callers return the send error. A short write (R-9: 0 < n < len) by any writer marks the stream broken: a fatal Notification is then not written, its Warn line names the cause, and the session still closes; an advisory one is refused with an error, which ends `ReadLoop` the same way. After `ReadLoop` returns, `runSession`'s deferred `kaCancel` and `Stop` run (in that order), and the KeepAlive sender exits on its context or on its failed write, so no goroutine outlives the session and none is added | Context cancel of the sender (alternative 3); accept the race; a lock that spans `conn.Write` with the deadline armed inside it (deadlocks behind a blocked KeepAlive, R-8) | The Notification must be the last PDU, and the guard covers every fatal path, not only expiry. The KeepAlive sender already returns on a send error. `ai/rules/goroutine-lifecycle.md`: no new goroutine, and the existing one has a bounded exit |
| D-4 | Expiry in OPENSENT and OPENREC sends the same code (subject to Q-2) | Shutdown during initialization | The 2.5.4 diagram NAKs a Timeout in the initialization states; the timer that expired is the same read deadline |
| D-5 | No new timer, leaf, metric or log level; one Warn line on a failed send | A counter for sent Notifications | Not asked for; the owner's best-effort decision names one log line |

### Triple challenge

| Challenge | Answer |
|-----------|--------|
| Simplicity | One constant, one call in the timeout branch, one write guard. The guard is the only addition beyond the minimum, and it is what makes "Notification before close" true rather than usually true |
| Uniformity | Same shape as `rejectInit` and the Shutdown NAK in `processMessages`; same writer |
| Performance | No heap allocation: `sendNotification` uses a 64-byte stack buffer. The write guard adds one uncontended mutex per PDU write on a control-plane session |

## Wiring Test (MANDATORY -- NOT deferrable)

<!-- BLOCKING: proves the feature is reachable from its intended entry point.
     Without it the feature exists in isolation: unit tests pass, nothing calls it.
     Every row needs a concrete test name. "Deferred"/"TODO"/empty is rejected
     by `internal/le/hookruntime/lifecycle.go`, which is the point: an unedited row fails. -->
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Peer silent on an operational session dialed by `startSessionForAdj` (checked 2026-10-14: `startSessionForAdj` in `register.go` starts `go runSession(...)`; re-confirm the name at implementation) | → | `runSession` → `Session.ReadLoop` timeout branch → `sendNotification` → deferred `Stop` | `TestRunSessionKeepaliveExpiryNotifiesThenCloses` (`internal/plugins/ldp/keepalive_expiry_test.go`) |
| FRR ldpd session, FRR's TCP toward ze dropped | → | the same chain in the running `ze` daemon | interop scenario `ldp-keepalive-expiry-frr` |

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion, stated as
     observable behavior, never as the mechanism used to reach it. -->
| AC ID | Input / Condition | Expected Behavior | Proof |
|-------|-------------------|-------------------|-------|
| AC-1 | An OPERATIONAL session, no liveness monitor configured, the peer sends no PDU for longer than the KeepAlive Timer | The next PDU ze writes is exactly one Notification: PDU version 1, ze's LSR ID and label space, Message Type 0x0001, Status TLV type 0x0300 length 10, Status Code 0x80000014, Message ID 0, Message Type 0. Then the session reaches NON EXISTENT and the peer reads EOF | `TestReadLoopKeepaliveExpirySendsFatalNotification` asserts the decoded fields and the raw octets in hex and that `ReadLoop` returns `errKeepaliveExpiry` (`ReadLoop` never calls `Stop`, so this unit asserts no close); `TestRunSessionKeepaliveExpiryNotifiesThenCloses` asserts the same Notification through `runSession`, then EOF on the peer side and NON EXISTENT, which its deferred `Stop` produces |
| AC-2 | The timer expires in OPENSENT, and separately in OPENREC | The same Notification (code per Q-2); `ReadLoop` returns `errKeepaliveExpiry`; no SessionUp | `TestReadLoopKeepaliveExpiryDuringInitialization` (table over both states, over bare `ReadLoop`; the close is `runSession`'s and is proven by the wiring test) |
| AC-3 | The Notification write fails while reads still time out (a peer that had closed would make the read return EOF, so the timeout branch would never run) | `ReadLoop` returns `errKeepaliveExpiry` (not the write error); exactly one Warn line naming the status and the error | `TestReadLoopKeepaliveExpiryNotificationWriteFails` over a fake conn whose `Read` returns a timeout `net.Error` and whose `Write` returns an error, with a captured logger counting the line |
| AC-4 | The peer is connected but never reads, so the write cannot complete | The attempt ends within `notificationWriteTimeout`; one Warn line; `ReadLoop` returns `errKeepaliveExpiry` | `TestReadLoopKeepaliveExpiryNotificationWriteBounded` (elapsed time bounded, `net.Pipe` peer that never reads; about 5 s, because `notificationWriteTimeout` is a constant) |
| AC-4b | As AC-4, and a KeepAlive from the `runSession` sender is blocked mid-write holding the write lock when the timer expires | The blocked KeepAlive write returns a timeout error; the expiry attempt still ends within `notificationWriteTimeout` of the expiry; one Warn line; `ReadLoop` returns, `runSession`'s deferred `Stop` runs, and the KeepAlive sender goroutine has exited when `runSession` returns | `TestRunSessionKeepaliveExpiryBoundedBehindBlockedKeepalive` (`net.Pipe` peer that reads nothing after the session is OPERATIONAL, KeepAlive 1 s so the sender is mid-write at expiry; the test LIB holds no local bindings, so `SendLabelMapping` in the `onOperational` callback, which carries no deadline, writes nothing and cannot block `ReadLoop`; the blocked write fails only AT the armed deadline, so the test asserts elapsed time from expiry to `ReadLoop` return of `notificationWriteTimeout` plus a fixed slack, and that the Notification write itself fails because the peer reads nothing; `runSession` returns; goroutine count back to its baseline). Red against a design that arms the deadline inside the lock |
| AC-5 | A fatal Notification has been written (expiry, or any other fatal cause) and a KeepAlive, Label Mapping, Label Withdraw or Initialization send is then attempted on the same session | Nothing more reaches the wire; each send returns an error | `TestNoPDUAfterFatalNotification` (calls `SendKeepalive`, `SendLabelMapping`, `SendLabelWithdraw` and `SendInit` after the expiry Notification, asserts each returned error and that the peer reads no further octet) |
| AC-6 | Live FRR ldpd peer, operational session, FRR's TCP toward ze dropped in ze's container | FRR receives a Notification carrying KeepAlive Timer Expired from ze and the FRR neighbor leaves OPERATIONAL; with the send reverted the scenario is red | Interop scenario `ldp-keepalive-expiry-frr`, plus a recorded red under revert with a rebuilt image |
| AC-7 | Every other Notification path (Bad Protocol Version, Session Rejected/No Hello, Session Rejected/Bad KeepAlive Time, Shutdown NAK, Unknown TLV, Loop Detected) | Unchanged codes, references and log lines; advisory Notifications do not block later writes | The existing tagged tests in `rfc5036_test.go`, `rfc5036_procedures_test.go`, `rfc5036_establishment_test.go`, `rfc5036_openrec_test.go` stay green unedited, plus `TestAdvisoryNotificationDoesNotBlockWrites`; an advisory Notification behind a blocked KeepAlive is bounded by its own armed deadline (D-3), proven by `TestAdvisoryNotificationBoundedBehindBlockedKeepalive` |
| AC-8 | Docs | `docs/architecture/ldp/mpls-ldp.md` and `docs/guide/mpls.md` say ze sends KeepAlive Timer Expired before closing; `rfc/short/rfc5036.md` RFC5036-2.5.3-2, RFC5036-3.5.1-1, Support remaining and the implementation prose drop expiry from the silent causes and keep decoding errors | `./le spec citation anchors` clean; grep shows no "KeepAlive expiry ... close silently" sentence left |

## End-to-End User Stories

<!-- One row per user-facing operation the feature enables. ACs verify that
     components work; stories verify the chain is connected. A broken link in a
     path is a spec gap: add the missing component to ACs, Files, and Test Plan
     before proceeding. Delete this section when Scope is tooling or docs. -->
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Operator runs ze with an LDP neighbour that stops sending PDUs; the neighbour's operator sees why ze dropped the session | silence → `ReadLoop` timeout → Notification 0x80000014 → close → peer logs KeepAlive Timer Expired | `ldp-keepalive-expiry-frr` |
| 2 | Operator reads ze's log after the expiry | `runSession` logs `ldp: session ended ... keepalive timer expired`; a failed send adds one Warn line | `TestReadLoopKeepaliveExpiryNotificationWriteFails` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEncodeNotificationKeepaliveTimerExpired` | `internal/plugins/ldp/wire_test.go` | Exact octets of the message for status 0x80000014, references zero, decoded back by `decodeNotification` | |
| `TestReadLoopKeepaliveExpirySendsFatalNotification` | `internal/plugins/ldp/keepalive_expiry_test.go` (new) | AC-1 over `ReadLoop` with a short `holdTime` | |
| `TestReadLoopKeepaliveExpiryDuringInitialization` | same | AC-2 | |
| `TestReadLoopKeepaliveExpiryNotificationWriteFails` | same | AC-3 | |
| `TestReadLoopKeepaliveExpiryNotificationWriteBounded` | same | AC-4 | |
| `TestRunSessionKeepaliveExpiryBoundedBehindBlockedKeepalive` | same | AC-4b: lock order of D-3, sender exit; the test LIB has no local bindings, so `onOperational`'s `SendLabelMapping` cannot block `ReadLoop`; asserts elapsed = `notificationWriteTimeout` plus slack and that the Notification write fails | |
| `TestNoPDUAfterFatalNotification` | same | AC-5 | |
| `TestAdvisoryNotificationDoesNotBlockWrites` | same | AC-7: after Unknown TLV (E clear) a Label Mapping still reaches the wire | |
| `TestAdvisoryNotificationBoundedBehindBlockedKeepalive` | same | AC-7, D-3: an Unknown TLV Notification sent while a KeepAlive is blocked mid-write holding the write lock arms the deadline before the lock, so the KeepAlive fails with a timeout, the advisory attempt ends within `notificationWriteTimeout` plus slack, and the KeepAlive sender has exited (R-10). The test wraps the `net.Pipe` conn so `Write` signals when it is entered, and sends the Unknown TLV only after that signal, so the KeepAlive is known to hold the lock first; without that sync the test would also pass when the advisory took the lock first, so only with it does the test go red against "arm inside the lock" | |
| `TestRunSessionKeepaliveExpiryNotifiesThenCloses` | same | Wiring: AC-1 through the production `runSession`, KeepAlive 1 s on both sides, including the EOF and NON EXISTENT that its deferred `Stop` produces | |

Every new test is written first and observed red against HEAD (no Notification arrives; the peer reads EOF or a KeepAlive), then green.

**RFC-tagged tests that would change, and so need owner approval:**

| Tagged test | Requirement | Changes when | Change |
|-------------|-------------|--------------|--------|
| none | - | Q-1 answered "separate" | The Notification itself changes no tagged test: no existing test reaches the expiry. BUT under "separate" two tagged claims stay false at HEAD (RFC-tagged test review, 2026-10-13, confirmed in the bodies and in `Session.ReadLoop`'s read deadline, which is armed from `currentHoldTime()`): `TestRFC5036InitNonZeroKeepaliveTimeAccepted` keeps asserting the 3x defect, and the RFC5036-2.5.1-2 `enforced` verdict stays false. Both are recorded in `plan/journal/reference-checked-claim-unchecked.md`, and "separate" leaves them there |
| `TestRFC5036InitNonZeroKeepaliveTimeAccepted` (`internal/plugins/ldp/rfc5036_test.go`) | RFC5036-3.5.3-1 positive | Q-1 answered "fold in" | Its `currentHoldTime() == 90s` assertion becomes 30 s, and its prose "the hold time is three times it" is rewritten to the Section 3.5.3 sentence ("The receiving LSR MUST calculate the value of the KeepAlive Timer by using the smaller of its proposed KeepAlive Time and the KeepAlive Time received in the PDU."); the unit changes, so its audit verdict stales and a fresh `./le rfc discriminate-record` is owed. Verdict today: WRONG while Q-1 is "separate", COMPLIANT under "fold in" |
| `TestRFC5036KeepaliveNegotiationAdoptsLower`, `TestRFC5036KeepaliveNegotiationRefusesHigher` (`internal/plugins/ldp/rfc5036_test.go`) | RFC5036-2.5.1-2 positive / negative (`enforced`) | Q-1 answered "fold in" | Each gains an assertion on the timer ze ARMS (`currentHoldTime()`, which sets the read deadline) equal to the negotiated time, beside the existing `currentKeepalive()` assertion. Row 2.5.1-2 is the "MUST calculate the value of the KeepAlive Timer" sentence, and today both units assert only the negotiated time, so they pass while the armed timer is 3x (UNDER-TEST). Under "separate" the bodies stay and the `enforced` verdict must be re-judged down, which is an `rfc/` change outside this spec |

Untagged test that would also change under Q-1 "fold in": `TestConfiguredKeepaliveReachesInitializationMessage` (`keepalive_test.go`, 45 s becomes 15 s).

#### Owner approval commands

Approved by the owner on 2026-10-13 ("approve all RFC-tagged test changes"); the three units change only under Q-1 "fold in", which is still open. The gate reads approvals from the committing session's own file (`tmp/commit-rfc-approved-<session>.md`), and a row is used up by the commit that carries its `RFC-approved:` trailer. So the owner runs these at the prompt of the session that implements and commits phase 4, and runs a unit's command again if a later commit changes that unit a second time. A unit is `<package directory>.<top-level test function>`; all three were checked to exist in `internal/plugins/ldp/rfc5036_test.go` at HEAD `051876c3f4`. The new Notification tests are new units and need none.

3 commands:

```
./le rfc approve unit ldp.TestRFC5036InitNonZeroKeepaliveTimeAccepted reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-ldp-keepalive-expiry-notification, Q-1 fold in: hold time 90 s becomes 30 s, prose rewritten)'
./le rfc approve unit ldp.TestRFC5036KeepaliveNegotiationAdoptsLower reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-ldp-keepalive-expiry-notification, Q-1 fold in: gains an assertion on the armed timer)'
./le rfc approve unit ldp.TestRFC5036KeepaliveNegotiationRefusesHigher reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-ldp-keepalive-expiry-notification, Q-1 fold in: gains an assertion on the armed timer)'
```

New tests carry no `RFC requirement:` tag unless Q-3 adds a row: RFC5036-2.5.3-2 and RFC5036-3.5.1-1 are ordinary `{gap}` rows, which forbid polarity tags, and `{partial}` cannot scope by cause.

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Status Code sent | fixed 0x80000014 | 0x80000014 | N/A | N/A |
| Read deadline | `currentHoldTime()` | N/A: the feature takes no new numeric input; the KeepAlive Time range 1 to 65535 is already covered by `TestSessionKeepaliveBoundaries` | N/A | N/A |

### Functional Tests
<!-- REQUIRED: a unit test proves the algorithm, a .ci proves the user can reach
     the feature. New RPCs/APIs are never covered by unit tests alone.
     Structure: ai/patterns/functional-test.md -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| none (Q-4) | `test/ldp/` | No `.ci` harness can play an LDP peer: `test/ldp/ldp-session.ci` records that a live session "needs a mock LDP peer". The user path is proven by `TestRunSessionKeepaliveExpiryNotifiesThenCloses` over the production `runSession` and by the interop scenario. No RPC, command or config is added | |

### Interop Tests (Scope: protocol)
<!-- REQUIRED when wire-visible behavior changes. See
     ai/rules/interop-and-goal-validation.md, including the vacuity traps: prove
     the test FAILS when the behavior under test is reverted. -->
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ldp-keepalive-expiry-frr` | `test/interop/scenarios/ldp-keepalive-expiry-frr/` (`ze.conf` with `ldp/keepalive-time` small, `frr.conf` with `mpls ldp` and message debug, `daemons` with `ldpd=yes` because the shared `test/interop/daemons` sets `ldpd=no`); checker in `internal/le/interoplab/bgp/` beside the BFD failover checker; run by `./le test integration interop` | FRR ldpd | Session OPERATIONAL; nft table in ze's container drops inbound TCP from FRR (port 646); ze's timer expires; FRR records a received Notification naming KeepAlive Timer Expired (A-3) and its neighbour leaves OPERATIONAL; table removed on exit. Red under a revert of the send with the image rebuilt | |

The name follows `docs/architecture/testing/interop.md` (named, no numeric prefix) and replaces the originating spec's planned `ldp-keepalive-shutdown-notification-frr`, which named the wrong status. The originating spec's interop row is updated to the new name in the same phase.

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files.
     Check each file's // Design: annotation: if the change alters behavior the
     referenced architecture doc describes, list that doc here too. -->
- `internal/plugins/ldp/wire.go` - KeepAlive Timer Expired status constant in the RFC 5036 Section 3.9 block, with the Section 3.5.1.2.3 quote
- `internal/plugins/ldp/session.go` - `ReadLoop` timeout branch sends the Notification (RFC 5036 Section 2.5.4 and 3.5.1.2.3 quotes above it), logs a failed send once, returns `errKeepaliveExpiry`; one serialised write path with a fatal-sent flag used by `SendInit`, `SendKeepalive`, `SendLabelMapping`, `SendLabelWithdraw`, `sendNotification`; doc comments of `sendNotification` and `Stop` updated
- `internal/plugins/ldp/register.go` - only if the KeepAlive sender needs to treat the new refusal error as its normal exit (it already returns on any send error); comment at the call naming RFC 5036 Section 2.5.4
- `internal/plugins/ldp/wire_test.go` - `TestEncodeNotificationKeepaliveTimerExpired`
- `docs/architecture/ldp/mpls-ldp.md` - expiry sends KeepAlive Timer Expired; the write guard; source anchors
- `docs/guide/mpls.md` - `keepalive-time` paragraph: the Notification before close
- `rfc/short/rfc5036.md` - RFC5036-2.5.3-2 and RFC5036-3.5.1-1 `{gap}` reasons, Support remaining, and the implementation prose: expiry removed from the silent causes, decoding errors kept
- `docs/features/rfc-status.md` - regenerated from the summary, never hand-edited
- `plan/spec-liveness-event-tears-down-bgp-peer.md` - interop row renamed to `ldp-keepalive-expiry-frr`
- `internal/le/interoplab/bgp/checkers.go` or `check_special.go`, `names.go` - the scenario's checker and its nft constants
- `docs/architecture/testing/interop.md` - Scenario Inventory row for the new scenario

## Files to Create
- `internal/plugins/ldp/keepalive_expiry_test.go` - the unit and wiring tests above
- `test/interop/scenarios/ldp-keepalive-expiry-frr/ze.conf`, `frr.conf` - scenario inputs
- `test/interop/scenarios/ldp-keepalive-expiry-frr/daemons` - the shared FRR `daemons` file with `ldpd=yes`; `prepare.go` mounts it over `/etc/frr/daemons` in place of the shared one (A-6)
- No `.ci` (Q-4)

### Integration Checklist
<!-- Answer every row Yes / No / N-A. Never leave a bare marker: an unanswered
     row is indistinguishable from a forgotten one. N-A needs a reason. -->
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No config or RPC added |
| YANG validation constraints | N-A | No leaf added |
| YANG custom validators | N-A | No leaf added |
| CLI commands/flags | N-A | None |
| CLI grammar (keyword before value) | N-A | No command |
| Editor autocomplete | N-A | No command |
| Functional test for new RPC/API | N-A | No RPC/API; see Q-4 for the `.ci` question |
| Pipe completeness | N-A | No output |
| Env var registration | N-A | None |
| Doctor check for runtime dependencies | N-A | No new path, socket, port, module or binary; the TCP 646 session already exists |
| Prometheus counters/metrics | No | D-5: no counter asked for |
| BGP family surface (new SAFI / capability / attribute) | N-A | LDP, not BGP |

### Discovery (`ai/rules/repo-maintenance.md` Mechanical Checklist)
| Question | Answer |
|----------|--------|
| Where an agent looks first | `ai/INDEX.md` LDP row and `ai/CODE-TO-DOCS.md` (`session.go` → `docs/architecture/ldp/mpls-ldp.md`, `docs/guide/mpls.md`) |
| What prevents regression | The new unit tests and the interop scenario; the `rfc/short` gap text names the remaining causes |
| What registry prevents drift | Interop catalog: `Discover` errors on a scenario directory with no checker |
| What verification proves it | `./le job run` scoped `go test ./internal/plugins/ldp/...`; `./le test integration interop` with the scenario selector |

### Documentation Update Checklist (BLOCKING)
<!-- Answer every row Yes / No / N-A. A No must be backed by a source-aware
     check, not a guess: at minimum grep docs/ for source anchors pointing at the
     files you changed. Any factual doc change carries a source anchor. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Defect fix in an existing feature; `features/mpls-ldp-rsvp-te.md` gains the interop scenario in its evidence only if its Interop entry lists LDP scenarios (check at implementation) |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | Behaviour inside the ldp plugin only |
| 6 | Has a user guide page? | Yes | `docs/guide/mpls.md` (`keepalive-time` paragraph) |
| 7 | Wire format changed? | Yes | `docs/architecture/ldp/mpls-ldp.md` (one more PDU ze sends); no `docs/architecture/wire/` page covers LDP |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc5036.md` gap text and prose; `docs/features/rfc-status.md` regenerated |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/interop.md` Scenario Inventory |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `docs/architecture/ldp/mpls-ldp.md` (write guard after a fatal Notification) |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | - |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Run `./le spec citation anchors spec plan/immediate/spec-ldp-keepalive-expiry-notification.md` at implementation. Known: `docs/architecture/ldp/mpls-ldp.md` (Design header of `session.go`, anchors on `processMessages`, `rejectInit`, `sendNotification`, `encodeNotification`), `docs/guide/mpls.md` (CODE-TO-DOCS for `session.go`, `register.go`), `docs/DESIGN.md`, `docs/architecture/iface/management.md`, `docs/guide/configuration.md` (CODE-TO-DOCS for `register.go`, unaffected unless `register.go` changes behaviour they describe) |
| 17 | Existing docs show config/CLI/API examples for this area? | No | No example changes |

## Implementation Steps

<!-- Concrete phases of work, not a restatement of the /ze-implement stages
     (those live in the skill). Phase 1 is ALWAYS wiring. Order by dependency:
     schema before resolution, resolution before CLI. Each phase follows TDD
     (write test -> fail -> implement -> pass) and ends with a self-critical
     review; fix what it finds before starting the next phase. -->

0. **Owner answers** -- Q-1 to Q-4 below are answered and recorded in the question table before code
1. **Phase: Wiring (MANDATORY FIRST)** -- write `TestRunSessionKeepaliveExpiryNotifiesThenCloses` over the production `runSession`
   - Tests: `TestRunSessionKeepaliveExpiryNotifiesThenCloses`
   - Files: `internal/plugins/ldp/keepalive_expiry_test.go`
   - Verify: red at HEAD (peer reads EOF, no Notification)
2. **Phase: Status code and send** -- constant, `ReadLoop` timeout branch, log-once on failure
   - Tests: `TestEncodeNotificationKeepaliveTimerExpired`, `TestReadLoopKeepaliveExpirySendsFatalNotification`, `TestReadLoopKeepaliveExpiryDuringInitialization`, `TestReadLoopKeepaliveExpiryNotificationWriteFails`, `TestReadLoopKeepaliveExpiryNotificationWriteBounded`
   - Files: `wire.go`, `session.go`, `wire_test.go`
   - Verify: red, then green; wiring test green; docs `mpls-ldp.md` and `mpls.md` edited in this phase
3. **Phase: Notification is the last PDU** -- serialised write path refusing after a fatal Notification
   - Tests: `TestNoPDUAfterFatalNotification`, `TestRunSessionKeepaliveExpiryBoundedBehindBlockedKeepalive`, `TestAdvisoryNotificationDoesNotBlockWrites`, all existing ldp tests unedited
   - Files: `session.go`, possibly `register.go`
   - Verify: red, then green; `go test -race` scoped to `./internal/plugins/ldp/` under `./le job run`
4. **Phase: Q-1 fold-in (only if the owner chooses it)** -- KeepAlive Timer equals the negotiated KeepAlive Time
   - Tests: the two assertions named in the TDD plan, with a fresh discrimination record for the tagged one
   - Files: `session.go`, `rfc5036_test.go`, `keepalive_test.go`, `docs/guide/mpls.md`; the journal row in `plan/journal/paraphrase-becomes-the-authority.md` updated to fixed
5. **Phase: Interop** -- scenario `ldp-keepalive-expiry-frr` with its own `daemons` (`ldpd=yes`), checker, inventory row
   - First: validate A-6 (ldpd reaches OPERATIONAL with ze) with a checker that only waits for OPERATIONAL; if it does not, stop and report before writing the nft step
   - Verify: green; then revert phase 2, rebuild the image, observe red, restore, observe green, record the red in the spec
6. **Phase: RFC ledger text** -- `rfc/short/rfc5036.md` gap text and prose; regenerate `docs/features/rfc-status.md`; the originating spec's interop row renamed. Gates owed by the main thread: `./le rfc check`, `./le verify worktree`

### Critical Review Checklist

<!-- Feature-SPECIFIC checks. The generic ones in ai/rules/quality.md always
     apply and are not repeated here. A row that would read the same on any spec
     is not worth a row. -->
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | Status Code is 0x80000014, not 0x8000000A; references are zero; the Notification precedes close and nothing follows it |
| Correctness | `ReadLoop` returns `errKeepaliveExpiry` whether or not the send succeeded; exactly one Warn line on failure |
| Naming | Constant named for the RFC status ("KeepAlive Timer Expired"), beside the existing `status*` names |
| Data flow | Notification policy lives in `Session`; `runSession` learns nothing new beyond the sender's exit |
| Rule: rfc-compliance | Quote of Sections 2.5.4 and 3.5.1.2.3 above the send; Section 3.9 above the constant; no tag added to an ordinary `{gap}` row |
| Rule: goroutine-lifecycle | No new goroutine; the KeepAlive sender still exits on its context or on a send error, including when it was blocked mid-write at expiry (AC-4b) |
| Concurrency | D-3 lock order: the write deadline is armed before the write lock is taken; no lock spans a write without a deadline that the fatal path can arm |

### Deliverables Checklist

<!-- Every deliverable with a command that proves it. "Looks done" is not a
     verification method. -->
| Deliverable | Verification method |
|-------------|---------------------|
| KeepAlive Timer Expired constant | `grep -n 0x00000014 internal/plugins/ldp/wire.go` |
| Send on expiry | `go test -run 'KeepaliveExpiry' ./internal/plugins/ldp/` under `./le job run`, output pasted |
| No PDU after fatal | `go test -race -run 'NoPDUAfterFatalNotification|AdvisoryNotification' ./internal/plugins/ldp/` |
| Interop | `./le test integration interop` selecting `ldp-keepalive-expiry-frr`; red-under-revert record |
| Ledger text | `grep -n "KeepAlive expiry" rfc/short/rfc5036.md docs/architecture/ldp/mpls-ldp.md` shows no "close silently" claim for expiry |

### Security Review Checklist

<!-- Feature-specific: untrusted input, injection, resource exhaustion, error
     leakage, authorization that could fail open. -->
| Check | What to look for |
|-------|-----------------|
| Resource exhaustion | A peer that stops reading cannot hold the session open: the write is bounded by `notificationWriteTimeout` (AC-4) |
| Fail open | The write guard must never block a write on a live session: advisory Notifications do not set it (AC-7) |
| Error leakage | The Notification carries no optional TLV and no internal state |
| Input validation | No new input is parsed |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
<!-- LIVE: write immediately when you learn something. Route each lesson to its
     governing surface under ai/rules/planning.md. A problem-class journal row
     is appropriate only when no surface governs the lesson yet; closure alone
     requires no lesson artifact. -->

## Key Design Decisions
<!-- "Chose X over Y because Z." The rejected alternative is the valuable half. -->
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Send from `ReadLoop`'s timeout branch | Send from `runSession` after the error returns | One owner of the detection and its report; mirrors `rejectInit` (Design, alternatives 1 and 2) |
| Serialised writes with a fatal-sent refusal | Cancel the KeepAlive sender first | Covers every fatal path and in-flight writes (alternative 3) |
| KeepAlive Timer Expired in all states | Shutdown in OPENSENT/OPENREC | Same timer; Section 2.5.4 diagram NAKs a Timeout there (Q-2 confirms) |
| Interop in the Docker lab (`test/interop/scenarios/`) | Extend the QEMU `TestLDPInteropFRR` | The lab has a catalog, named scenarios, `./le feature record-run` and an nft drop precedent; the QEMU test has no discovery and its 9p mount failure is open in `plan/journal/silent-fall-through.md` |

## Owner Questions

| ID | Question | Recommendation | Why it needs the owner | Answer |
|----|----------|----------------|------------------------|--------|
| Q-1 | Ze's KeepAlive Timer is 3x the negotiated KeepAlive Time (`holdTime`), against RFC 5036 Section 3.5.3. Fix it here as code related to the problem in hand, or leave it on the journal row? | Fold in as phase 4: it decides when this Notification fires. The negotiated timer is one site, `handleInit`'s `3 * negotiatedKA`, and it changes. `NewSession` also sets `holdTime: 3 * keepalive`, which governs only the wait for the peer's first Initialization before any KeepAlive Time is negotiated (its comment cites RFC 5036 Section 2.5.3); it stays, and the phase-4 implementer confirms that against the Section 2.5.3 text before leaving it | It changes the assertions of three tagged tests, `TestRFC5036InitNonZeroKeepaliveTimeAccepted` (RFC5036-3.5.3-1 positive, 90 s becomes 30 s) and `TestRFC5036KeepaliveNegotiationAdoptsLower` / `RefusesHigher` (RFC5036-2.5.1-2, gain an assertion on the armed timer), and makes ze close a session three times sooner against every peer. Answering "separate" is not neutral: the first test keeps asserting the defect against Section 3.5.3 "The receiving LSR MUST calculate the value of the KeepAlive Timer by using the smaller of its proposed KeepAlive Time and the KeepAlive Time received in the PDU.", and the RFC5036-2.5.1-2 `enforced` verdict stays false (RFC-tagged test review, 2026-10-13) | open (fold in or separate). The tagged-test approval is answered: **owner, 2026-10-13: "approve all RFC-tagged test changes"**. It covers the three tagged units above and applies only if Q-1 is answered "fold in"; under "separate" no tagged test changes. It does not answer Q-1. Commands under "Owner approval commands" in the TDD plan |
| Q-2 | A read timeout in OPENSENT or OPENREC: send KeepAlive Timer Expired (0x80000014) or Shutdown (0x8000000A)? | KeepAlive Timer Expired: it is the same timer, and Section 2.5.4 only says "Tx NAK" | The RFC names no code for an initialization timeout. If Shutdown is chosen instead: the timeout branch picks its code from the session state, so `ReadLoop` reads the state at expiry; AC-2's table expects 0x8000000A for OPENSENT and OPENREC; D-4 and the Key Design Decisions row are rewritten; and an FRR peer timing out in initialization logs Shutdown rather than the timer that actually expired | open |
| Q-3 | RFC5036-2.5.3-2 and RFC5036-3.5.1-1 stay `{gap}` (decode failures remain), so the new tests cannot be tagged. Add a row RFC5036-2.5.4-3 for the OPERATIONAL Timeout transition ("Timeout NON EXISTENT Action: Transmit Shutdown msg and close transport connection") so the behaviour is gated? | Add the row: section 2.5.4 is already walked and in scope, and an ungated fix can regress unseen | The owner froze RFC scope on 2026-10-08: no new enrolment and no new summaries, and in-scope summaries are fixed only. Adding RFC5036-2.5.4-3 grows the gated set of an enrolled summary, so it is allowed only if the owner rules that the transition is "already in scope and handled incorrectly"; this spec does not decide that. What "add" costs, all in this spec's phase 6: (1) `rfc/short/rfc5036.md` gains the row; (2) `rfc/extraction/rfc5036.json` section 2.5.4 gains RFC5036-2.5.4-3 in its `unsourced-ids` (it lists only -1 and -2, and the section has 0 sites); (3) the row text must pass `./le rfc check`'s verbatim row-quote gate, and the quote above is table cells joined, not a sentence of Section 2.5.4, so the row must quote a span the gate accepts or the gate refuses it; (4) the new expiry tests carry `RFC requirement: RFC5036-2.5.4-3`; (5) each tagged unit owes a `./le rfc discriminate-record` in the same commit. If "do not add": the fix stays ungated, the new tests stay untagged, and `rfc/` changes only by the gap text in phase 6 | open |
| Q-4 | No `.ci` can drive an LDP peer. Accept the `runSession` wiring test plus the FRR interop scenario as the end-to-end proof, or build an LDP mock-peer fixture first? | Accept: no RPC, command or config is added, and the interop test drives the real daemon | `ai/rules/testing.md` asks for a `.ci` for user-reachable behaviour. If the mock peer is built first: a new LDP peer fixture in the `.ci` harness (`internal/test/`) that speaks Initialization, KeepAlive and Notification, its own spec or a phase before phase 1, and a `test/ldp/` `.ci` that goes silent and asserts the Notification octets; this spec cannot close until that fixture lands, and `test/ldp/ldp-session.ci`'s "needs a mock LDP peer" note becomes a test | open |

## Design Review Log

| Round | Date | Reviewer | Result | Disposition |
|-------|------|----------|--------|-------------|
| 1 | 2026-10-14 | independent, did not author (HEAD `f9e044ec1b`) | 0 BLOCKER, 4 ISSUE, 4 NIT | applied. I-1: D-3 lock order (deadline armed before the write lock), R-8, R-9, A-7, AC-4b and its test, goroutine exit stated. I-2: AC-3 over a fake conn whose `Write` fails while reads time out; EOF and NON EXISTENT assertions moved to the `runSession` test (AC-1, AC-2, AC-4). I-3: scenario `daemons` with `ldpd=yes` in Files to Create, A-6, phase 5 first step, Required Reading corrected; journal row in `plan/journal/test-against-broken-path.md`. I-4: Q-3 states the freeze conflict and the five costs of "add"; Q-2 and Q-4 state the consequence of the other option. N-5 Q-1 names both 3x sites; N-6 AC-5 calls all four senders; N-7 `startSessionForAdj` checked; N-8 Task wording |
| 2 | 2026-10-14 | independent, did not author | 0 BLOCKER, 1 ISSUE, 3 NIT | applied. #1: D-3 lock order covers every `sendNotification`, advisory included; the deadline sentence names the four Send* writers only; the KeepAlive sender's exit after an advisory's deadline is stated and accepted as at HEAD (R-10); `TestAdvisoryNotificationBoundedBehindBlockedKeepalive` added under AC-7. #2: R-9 defines a short write as 0 < n < len; the AC-4b `net.Pipe` KeepAlive returns n = 0, stream intact. #3: AC-4b asserts `notificationWriteTimeout` plus slack and a failed Notification; the test LIB has no local bindings. #4: none |
| 3 | 2026-10-14 | independent, did not author | 0 BLOCKER, 0 ISSUE, 3 NIT | applied; design converged. #1: D-3 step 4b, an advisory clears its deadline before release. #2: R-10 quotes RFC 5036 Section 2.5.6 ("an LSR must arrange that its peer receive an LDP PDU from it at least every KeepAlive time period") as a known gap that predates this spec, accepted because the peer's own KeepAlive Timer ends the session. #3: `TestAdvisoryNotificationBoundedBehindBlockedKeepalive` wraps the `net.Pipe` conn so `Write` signals on entry and sends the Unknown TLV only after it, so it goes red against "arm inside the lock". Owner questions Q-2 to Q-4 stay open |

## Known Limitations
<!-- Deliberate scope boundaries. Anything here that is actually outstanding work
     is not a limitation: write it as its own spec, in the bucket that item
     belongs to, and name that spec here (ai/rules/planning.md). -->
- Decoding errors outside the initialization type checks still close with no Notification (RFC5036-2.5.3-2, RFC5036-3.5.1-1 remain `{gap}`). No spec owns that half; the transfer in `plan/spec-liveness-event-tears-down-bgp-peer.md` states it is not waived
- `expireAdjacencies` closes a session on Hello adjacency expiry with no Notification. That is not KeepAlive expiry and is outside this spec; it is a further silent cause of the same rows

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test (N-A when Scope is tooling or docs, which delete that section)
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied: lint fixed rather than disabled, the focused check for the changed behavior run once with its OUTPUT PASTED, and any red named with the one-line reason it is scaffolding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script (commit A preserves the spec in history)
