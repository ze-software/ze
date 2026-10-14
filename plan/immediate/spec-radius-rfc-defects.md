# Spec: radius-rfc-defects

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-14 |

**Dependencies:** `plan/immediate/spec-rfc5176-remaining-defect-classes.md` lands after this spec (its R-6); its S-1 deletes `coa.go::findSessions`, so the "tunnelID-sessionID-" prefix contract this spec preserves (R-5, AC-3c, `TestRFC2866AcctSessionIDKeepsTunnelSessionPrefix`) ends there.

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Three defects in Ze's RADIUS client and its L2TP dynamic-authorization and
accounting use, found by the strict re-read of
`spec-rfc-requirement-quote-hand-backfill`. Each was read at
its producer in HEAD on 2026-09-27.

## Defects

| ID | RFC section | Verbatim quote | Producer | What Ze does wrong | Audit verdict / record |
|----|-------------|----------------|----------|--------------------|------------------------|
| D1 | RFC 3579 Section 3.3 [Note 1] (RFC3579-3.3-2) | "An Access-Request that contains either a User-Password or CHAP-Password or ARAP-Password or one or more EAP-Message attributes MUST NOT contain more than one type of those four attributes." | `internal/component/radius/client.go::oneCredentialType` | The credential set holds three of the four types; ARAP-Password (70) is missing, so EAP-Message beside ARAP-Password encodes. Ze builds no ARAP today, so only a caller-supplied attribute reaches it | weak |
| D2 | RFC 5176 Section 2.3 (RFC5176-2.3-2) | "A Dynamic Authorization Server implementing this specification MUST be capable of detecting a duplicate request if it has the same source IP address, source UDP port, and Identifier within a short span of time." | `internal/component/l2tp/plugins/authradius/coa.go::replayKey` | The key is source IP, Code, Identifier and Request Authenticator: the source UDP port is ignored, so two clients behind one address collide, and the Authenticator makes a same-port same-Identifier retransmission with a changed Authenticator look new | weak; lead |
| D3 | RFC 2866 Section 5.5 | "This attribute is a unique Accounting ID to make it easy to match start and stop records in a log file." | `internal/component/l2tp/plugins/authradius/acct.go::genSessionID` | The id is tunnel, session and a process-local counter, so after a restart ids repeat in the server's log | suspected; the sentence is descriptive, not a MUST |

## What a fix must prove

| Obligation | Detail |
|------------|--------|
| Failing test first | one per row, red against HEAD |
| Both polarities | D1: each single credential type encodes, each pair refuses. D2: same IP, port and Identifier is a duplicate; a different port is not. D3: ids differ across a simulated restart |
| Discrimination | `./le rfc discriminate-record` for every tagged unit |
| Interop | D2 against FreeRADIUS `radclient` sending CoA from two source ports; D3 against a FreeRADIUS accounting log |

## Re-verification at HEAD `3e2770062b` (2026-10-13)

All three defects are still open. No commit since 2026-09-27 touched the three
producers' defective lines: `git log` on the three files shows only
`af69577952`, `4e55b63ea6`, `8c68fcf514`, `189db75308`, `0cc6cb1b22`, none of
which changes `oneCredentialType`, `replayKey` or `genSessionID`.

| ID | Requirement | Audit verdict at HEAD (`rfc/audit/<stem>.json`) | Producer at HEAD | `./le rfc check` |
|----|-------------|-----------------------------------------------|------------------|------------------|
| D1 | RFC3579-3.3-2 | `weak`; note names the ARAP-Password omission and the untested User-Password beside CHAP-Password pair | `client.go::oneCredentialType` map holds types 2, 3, 79 only; its caller comment in `encodeRequest` states "ARAP-Password is not named" | OK (gate passes; the gate does not judge verdicts) |
| D2 | RFC5176-2.3-2 | `weak`; note: the test's two copies differ in source port, and `replayKey` omits the port | `coa.go::replayKey` keys on source IP string, Code, Identifier, Authenticator; `cachedReplay(from.IP, pkt)` is passed the IP only | OK |
| D3 | RFC2866-5.5-1 | `weak`; note: counter restarts at 1 on every start, no unit covers a restart | `acct.go::genSessionID` is tunnel-session-counter; `nextSess` is an `atomic.Uint32` in `radiusAcct`, built once per process by `register.go` (`acctInstance = newRADIUSAcct()`) | OK |

`plan/pre-release/spec-rfc-verdict-fix-access.md` rows 146-148 and AC-C1/AC-C4
point these three ids at this spec. RFC2866-5.5-1 is also cited by
`plan/immediate/spec-lifecycle-invariants.md` (its D-6 constraint: "Uniqueness
is not identity"); this spec owns the restart half only.

## Owner decisions

| Row | Question | Owner answer |
|-----|----------|--------------|
| D3 | Which restart-unique scheme makes the Acct-Session-Id unique across restarts? | **Owner, 2026-10-13: "c"** (a time-based boot tag in Acct-Session-Id). Design under Q-1 below |
| D1 | Recognise ARAP-Password in the credential exclusivity check? | **Owner, 2026-10-13: "i"** (Q-4) |
| D2 | Duplicate key | **Owner, 2026-10-14: "B"** (Q-2) |

### Owner questions raised by the 2026-10-13 design (each states the reading the design proceeds under)

| ID | Question | Design proceeds under | Why it needs the owner |
|----|----------|-----------------------|------------------------|
| Q-1 (D3, replaces the row above) | Restart-unique scheme for the Acct-Session-Id | **Answered by the owner, 2026-10-13: "c" (time-based boot tag in Acct-Session-Id).** Design (2026-10-14, revised after review the same day): the boot tag is a time part AND a random part, minted once per accountant at its FIRST `genSessionID` call (once-only, guarded so concurrent first calls mint one tag), never at construction: 8 hex digits of the Unix seconds read from `acctNow` at that moment, then 16 hex digits (64 bits) from `crypto/rand`. The id becomes "tunnel-session-tag-counter". Why not at construction: the only construction is the package-level `register.go` line 34 (`acctInstance = newRADIUSAcct()`), which runs at Go package init, before any plugin starts and so before the NTP plugin's Phase 1 (`ntp.go::syncWorker.run` → `persist.go::loadTime` → `clock_linux.go::setClock`, the only `Settimeofday` caller), so a construction-time read on a box with no RTC sees the kernel's boot clock, frozen near one value on every boot. The first `genSessionID` runs at the first session's IP assignment, after ze's plugins have started, so on a box running ze's NTP it reads in practice at least the clock Phase 1 restored, or the synced clock; no code orders the two, so this is an assumption, not a guarantee (A-7). **Honest limit:** on a box with no RTC whose first session is assigned before NTP answers, the time part is the restored saved time, which can repeat across boots; uniqueness then rests on the random half alone, and the time part orders boots only once the clock is real. **Attribution:** the owner chose "c" (time-based boot tag) over the recommended "a" after reading that "c" alone repeats on a box with no RTC (commit `4c568af69b`); the 64-bit random half is the designer's addition to the owner's "c", and the report to the owner says so | Format of an operator-visible billing key. Why time plus random and not "wait for a synced clock", read at the producers on 2026-10-14: (1) ze learns that the clock is synced only inside the NTP plugin (`internal/plugins/ntp/ntp.go::syncWorker`, `synced atomic.Bool`, and the `clock-synced` event of `internal/plugins/ntp/events/events.go::EventClockSynced`, emitted once after the first successful sync), and only when ze's own NTP is configured. With NTP unconfigured, or the clock set by another daemon, no signal ever arrives, so waiting needs a timeout and a second scheme behind it (`ai/rules/no-layering.md`), and it holds every Accounting-Start until NTP answers. (2) Before NTP answers, the clock of an appliance with no RTC is the time `syncWorker.run` restored from the store at boot (Phase 1, `persist.go::loadTime`), so two boots that both mint their tag before NTP answers and before the next `saveTime` read the same wall time: time alone repeats a tag. The random part makes a repeat need equal seconds AND equal 64 random bits. Tests for both under the TDD plan |
| Q-2 (D2) | Duplicate key | **Answered by the owner, 2026-10-14: "B"**, after "I do not see the benefit of B / issue with A can you explain". Duplicate key = source IP, source port, Identifier, plus the Request Authenticator: a same-triple request with a different Authenticator is processed as new and replaces the cache entry. RFC 5080 Section 2.2.2 quoted verbatim under Required Reading | Changes when a CoA is applied. Without the Authenticator, a busy DAC that wraps its 8-bit Identifier on one port within the 5-minute window gets a genuinely new request answered with a stale cached response. RFC 5176 Section 2.3 forbids a legitimate retransmission from changing its Authenticator, so (B) still answers every legitimate retransmission from the cache. The Code leaves the key: the Request Authenticator hashes the Code (RFC 2866 Section 3: "Code + Identifier + Length + 16 zero octets + request attributes + shared secret", which RFC 5176 Section 2.3 adopts: "The Request Authenticator is calculated the same way as for an Accounting-Request, specified in [RFC2866]."), so a CoA-Request and a Disconnect-Request sharing a source port and Identifier differ in Authenticator and are processed as distinct requests |
| Q-3 (D1, D2) | Approve changing five RFC-tagged tests (list under TDD Test Plan, "Tagged tests that change"): the four D2 units whose duplicate copies come from a fresh socket each, so they would stop exercising the cache once the port is in the key (`TestRFC5176DuplicateRequestAnsweredFromCache` RFC5176-2.3-2, `TestRFC5176DuplicateAndTimestampWindowsBothHoldInside` and `...EndOutside` RFC5176-6.3-2, `TestDisconnectReplayReturnsCachedResponse` RFC5176-3.3-1), plus the D1 unit `TestAccessRequestRefusesTwoCredentialTypes` (RFC3579-3.3-2 negative), whose body grows from two pairs to six. The fifth was missing from this list until the 2026-10-13 RFC-tagged test review: a tagged unit whose body changes needs the same approval | **Answered by the owner, 2026-10-13: "approve all RFC-tagged test changes".** The approval covers every unit listed under "Tagged tests that change" (the five above, and the two RFC 2869 units Q-5 "fix and extend" adds); Q-2 is answered separately ("B", 2026-10-14), and the prose of `TestRFC5176DuplicateRequestAnsweredFromCache` names its consequence. The commands are under "Owner approval commands" in the TDD plan | `ai/rules/rule-precedence.md`: changing an RFC-tagged test needs the owner |
| Q-4 (D1) | ARAP-Password in the credential exclusivity check | **Answered by the owner, 2026-10-13: "i" (recognise ARAP-Password type 70 in credential exclusivity).** Type 70 joins the exclusive credential set of `client.go::oneCredentialType` by a literal, unexported value in `client.go`; `dict.go` stays free of any ARAP constant | (i) recognises the type to refuse a pair, which is not implementing the ARAP service that RFC 2869 Section 1.1 forbids; ARAP-Password alone still encodes (AC-1b) |
| Q-5 (D1) | The RFC2869-5.19-1 tests (same four-type sentence, verdict `enforced`, `rfc2869_credential_types_test.go`) | **Answered by the owner, 2026-10-13: "fix and extend"** (fix the wrong comment AND extend the existing RFC 2869 tagged credential tests with ARAP cases; tagged-test changes approved 2026-10-13). The header comment "ARAP-Password is not driven" is corrected, `TestRFC2869EAPBesideAnotherCredentialIsRefused` gains ARAP-Password (literal 70) beside EAP-Message, and `TestRFC2869EAPOnlyAccessRequestIsSent` gains the ARAP-Password absence assertion. The new ARAP tests carry the RFC3579-3.3-2 tag only, since the RFC2869-5.19-1 carriers now drive ARAP themselves | RFC 2869 Section 5.19 Note 1 is the same four-type sentence as RFC 3579 Section 3.3 Note 1; at HEAD the tests drive EAP beside User-Password and CHAP-Password only. The approval commands are under "Owner approval commands" |
| Q-6 (D3, scope) | `coa.go::findSessions` matches a CoA's Acct-Session-Id by the "tunnelID-sessionID-" prefix and ignores the rest, so a Disconnect-Request carrying a previous session's (or previous boot's) id tears down whichever current session reuses that tunnel and session number | **Answered by the owner, 2026-10-14: "whatever"**; the recommendation is taken: the prefix-match fix belongs to `plan/immediate/spec-rfc5176-remaining-defect-classes.md` (its S-1 and AC-1; its Q-1 records the move, answered by this question). This spec keeps the "tunnelID-sessionID-" prefix (R-5) and the Acct-Session-Id format work; that spec's exact match is format-agnostic | It changes which CoA tears down which session, so it lives with the CoA matching work |
| Q-7 (interop) | D2 and D3 interop need FreeRADIUS in front of Ze's CoA listener and accountant | **Answered by the owner, 2026-10-14: "Make Free radius, its own lab."** Follow-up, owner 2026-10-14: "yes" to the xl2tpd + pppd LAC inside that lab (`NET_ADMIN`, `/dev/ppp` preflight), since Ze accounts and disconnects only kernel L2TP sessions. Design under "Interop Tests": a FreeRADIUS lab of its own, `interop-radius-subscriber`, separate from the L2TP lab and from the admin-login suite, shared with `plan/immediate/spec-rfc5176-remaining-defect-classes.md` (its Q-7) | Lab work larger than the three producer fixes; one lab serves both specs |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/interop.md` - suites, "The FreeRADIUS admin-login suite"
  → Constraint: the FreeRADIUS suite (`internal/le/interoplab/radius/`, `./le test integration interop-radius`) runs admin login only, with no L2TP; the L2TP lab's RADIUS peer is `internal/le/interoplab/l2tp/radiusmock/`, "ze's own Go program", so no existing scenario puts a RADIUS implementation Ze did not write in front of CoA or accounting.
  → Constraint: every checker reads BOTH sides (Ze's log and the server's own record), and an interop claim needs a forced red with the change reverted and the image rebuilt.
- [ ] `docs/guide/l2tp.md` - CoA/DM section (lines 600-660) and accounting section (lines 414, 540)
  → Constraint: the page states no Acct-Session-Id format and no duplicate-detection key today; D2 adds one sentence on duplicate detection (source address, source port, Identifier, Authenticator; 5-minute window), D3 adds one on restart-unique ids.
- [ ] `docs/architecture/l2tp/bng-1-radius-attributes.md` - mapped to `acct.go` and `client.go` in `ai/CODE-TO-DOCS.md`
  → Constraint: line 107 names Acct-Session-Id (44) as always present; it states no format, so D3 needs no edit there unless the implementer adds one.

### RFC Summaries and full texts (Scope: protocol)
- [ ] `rfc/short/rfc3579.md` line 217 (RFC3579-3.3-2), `rfc/full/rfc3579.txt` Section 3.3 Note 1
  → Constraint: RFC 3579 Section 3.3 [Note 1]: "An Access-Request that contains either a User-Password or CHAP-Password or ARAP-Password or one or more EAP-Message attributes MUST NOT contain more than one type of those four attributes." Four types, six pairs; the guard covers three types and the tests cover two pairs.
- [ ] `rfc/short/rfc2869.md` line 146 (RFC2869-5.19-1, the same sentence, verdict `enforced`), `rfc/full/rfc2869.txt` Section 1.1
  → Constraint: RFC 2869 Section 1.1: "A NAS that does not implement a given service MUST NOT implement the RADIUS attributes for that service. For example, a NAS that is unable to offer ARAP service MUST NOT implement the RADIUS attributes for ARAP." `rfc2869_unoffered_service_attributes_test.go` parses `dict.go` and refuses any `Attr*` constant valued 70-74 or 84.
  → Decision: D1 recognises type 70 in `client.go` by an unexported literal, never by a `dict.go` constant (Q-4).
- [ ] `rfc/short/rfc5176.md` line 173 (RFC5176-2.3-2), `rfc/full/rfc5176.txt` Section 2.3, "Identifier"
  → Constraint: RFC 5176 Section 2.3: "A Dynamic Authorization Server implementing this specification MUST be capable of detecting a duplicate request if it has the same source IP address, source UDP port, and Identifier within a short span of time."
  → Constraint: RFC 5176 Section 2.3: "If the Dynamic Authorization Client is retransmitting a Disconnect-Request or CoA-Request to the same Dynamic Authorization Server as before, and the attributes haven't changed, the same Request Authenticator, Identifier, and source port MUST be used. If any attributes have changed, a new Authenticator and Identifier MUST be used." So a legitimate retransmission carries the same Authenticator, and keeping the Authenticator in the key loses no legitimate duplicate.
- [ ] `rfc/full/rfc5080.txt` Section 2.2.2 (fetched from rfc-editor.org on 2026-10-14 for reading; NOT enrolled, no `rfc/short/` summary, RFC scope frozen 2026-10-08; cited as design support only, never as a requirement)
  → Decision (Q-2, owner "B"): RFC 5080 Section 2.2.2: "Cache entries MUST also be purged if the server receives a valid Access-Request packet that matches a cached Access-Request packet in source address, source port, RADIUS Identifier, and receiving socket, but where the Request Authenticator field is different from the one in the cached packet." The CoA key follows that shape. "Receiving socket" is implicit: each `coaListener` owns one socket and its own replay map.
  → Constraint: RFC 5080 Section 2.2.2: "Packets with invalid Message-Authenticators MUST NOT affect the cache in any way." Ze already satisfies it by order: `handlePacket` runs `cachedReplay` only after the Request Authenticator and Message-Authenticator checks.
- [ ] `rfc/short/rfc2866.md` line 134 (RFC2866-5.5-1), `rfc/full/rfc2866.txt` Section 5.5
  → Constraint: RFC 2866 Section 5.5: "This attribute is a unique Accounting ID to make it easy to match start and stop records in a log file." The row is tagged [MUST] although the sentence carries no keyword; the same section's example reserves digits for reboots: "the first two digits increment on each reboot (wrapping every 256 reboots)".
  → Constraint: RFC 2866 Section 5.5: "The start and stop records for a given session MUST have the same Acct-Session-Id." The new id is minted once per session at `onSessionIPAssigned` and stored in `acctSession.acctSessID`; that must not change.

**Key insights:**
- All three verdicts are `weak` at HEAD and the gate is green: the defect is in what the producers do, not in tag resolution.
- D2's port fix breaks four tagged tests by construction, because every test helper (`sendRawCoAPacket`, `coaRawExchange`) dials a fresh UDP socket per send; their "duplicate" copies only matched because the port was ignored.
- D3's id is parsed back by `coa.go::findSessions` ("tunnelID-sessionID-" prefix); the new format keeps that prefix. The stale-id prefix match itself is S-1 of `spec-rfc5176-remaining-defect-classes` (Q-6).
- Ze accounts and disconnects L2TP sessions only, verified 2026-10-14: `acct.go::subscribeEventBus` subscribes `l2tpevents.SessionIPAssigned` and `SessionDown` only; the one L2TP emitter of `SessionIPAssigned` is `internal/component/l2tp/reactor_kernel.go` (line 367), after a kernel session has an address; `findSessions` reads `l2tp.LookupService()`, which `service_locator.go::PublishService` fills only from the Subsystem's Start. An interop lab for D2 and D3 therefore needs a live kernel L2TP session (Q-7).
- Ze's clock-sync knowledge, verified 2026-10-14: only `internal/plugins/ntp` knows (`syncWorker.synced`, event `clock-synced`), only when ze's NTP is configured, and its Phase 1 restores the last saved time before any sync (`ntp.go::syncWorker.run`, `persist.go::loadTime`, `clock_linux.go::setClock`, the only `Settimeofday` caller). `acctInstance = newRADIUSAcct()` (`register.go` line 34) runs at package init, before every plugin, so the tag is minted at the first `genSessionID`, not at construction. Q-1 rests on these facts.
- D1 has no wire-visible change for any request Ze builds (no builder emits ARAP-Password); it guards `Client.Exchange`, an exported entry point.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/radius/client.go` - `encodeRequest` (lines ~270-325) refuses EAP-Message/Message-Authenticator in an Accounting-Request, then calls `oneCredentialType` for an Access-Request before encoding; `oneCredentialType` (line 337) walks `pkt.Attrs` with a name map {2 User-Password, 3 CHAP-Password, 79 EAP-Message} and errors "radius: an Access-Request carries X beside Y; RFC 3579 Section 3.3 Note 1 permits one credential type".
  → Constraint: the error text "one credential type" is asserted by `TestAccessRequestRefusesTwoCredentialTypes`; keep it.
- [ ] `internal/component/radius/rfc2869_unoffered_service_attributes_test.go` - parses `dict.go` for `Attr*` integer constants; refuses ARAP values 70-74, 84.
  → Constraint: no `Attr*` constant valued 70 may appear in `dict.go`.
- [ ] `internal/component/radius/authenticator_eap_test.go::TestAccessRequestRefusesTwoCredentialTypes` - RFC3579-3.3-2 negative over `encodeRequest`: EAP beside User-Password, EAP beside CHAP-Password; control: EAP alone encodes.
- [ ] `internal/component/radius/rfc2869_credential_types_test.go` - RFC2869-5.19-1 pair over `Client.Exchange`; header says "ARAP-Password is not driven".
- [ ] `internal/component/l2tp/plugins/authradius/coa.go` - `serve` reads with `ReadFromUDP` (the `*net.UDPAddr` has the port); `handlePacket` checks source allow-list, Request Authenticator, decode, code, Message-Authenticator, THEN `cachedReplay(from.IP, pkt)` and on a hit `sendRawResponse(from, cached)`; then Event-Timestamp, Service-Type, attribute checks, dispatch. `coaReplayKey{source string, code, identifier, authenticator}`; `coaReplayWindow = 5 * time.Minute`; `rememberReplay` stores a copy of the response; `pruneReplayLocked` deletes entries older than the window.
  → Constraint: the duplicate check stays after both authenticator checks, so an unauthenticated packet never touches the cache.
  → Constraint: `coaReplayWindow` stays 5 minutes: `TestRFC5176DuplicateAndTimestampWindowsBoth*` (RFC5176-6.3-2, `enforced`) pin the dedup window to the Event-Timestamp window.
- [ ] `internal/component/l2tp/plugins/authradius/coa.go::findSessions` (line ~629) - matches Acct-Session-Id by `HasPrefix(id, "<LocalTID>-<LocalSID>-")`.
- [ ] `internal/component/l2tp/plugins/authradius/acct.go` - `radiusAcct.nextSess atomic.Uint32`; `genSessionID(tunnelID, sessionID)` returns "T-S-N" with N = `nextSess.Add(1)`; called once per session in `onSessionIPAssigned` (line 280), which first returns early if the session is already tracked; `detach` builds a retired accountant without copying `nextSess`.
- [ ] `internal/component/l2tp/plugins/authradius/register.go` line 34 - `acctInstance = newRADIUSAcct()`: one accountant per process, so the counter resets only on a process restart.
- [ ] `internal/component/l2tp/plugins/authradius/rfc2866_accounting_test.go` - `TestRFC2866AcctSessionIDUnique` (1600 concurrent ids distinct, one instance), `TestRFC2866AcctSessionIDNoCollisionOnReusedKey` (same key twice differs, one instance).
- [ ] `internal/component/l2tp/plugins/authradius/rfc2866_request_contents_test.go` lines 186-243 - use `genSessionID` output as an opaque value; no format assertion.

**Behavior to preserve:**
- The "tunnelID-sessionID-" prefix of the Acct-Session-Id (parsed by `findSessions`).
- One Acct-Session-Id per session for Start, Interim and Stop.
- The error text "one credential type" and refusal before any datagram is sent.
- The 5-minute duplicate window, cached-response replay of the same octets, and the order of checks in `handlePacket`.

**Behavior to change:** the three rows (D1 to D3); nothing else.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- an Access-Request built by Ze; a CoA or Disconnect-Request received on UDP 3799; an L2TP session start

### Transformation Path
1. request build and validation in the RADIUS client
2. DAS duplicate detection before acting on a CoA
3. accounting id generation at session start

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| L2TP plugin ↔ RADIUS component | client API | No |
| UDP socket ↔ CoA listener | `ReadFromUDP` gives `*net.UDPAddr` (IP and port) to `handlePacket` | Yes, `coa.go::serve` |
| L2TP event bus ↔ accountant | `SessionIPAssigned` event → `onSessionIPAssigned` → `genSessionID` | Yes, `acct.go::subscribeEventBus` |
| Ze ↔ RADIUS server log | Acct-Session-Id text in every Accounting-Request | No (no interop run with a real server) |

### Integration Points
- the authradius plugin's UDP listener, which has the source port: `cachedReplay` and `rememberReplay` take the `*net.UDPAddr` instead of its IP
- `findSessions` parses the id prefix the accountant writes
- `Client.Exchange` and `SendToServers`, the only routes to `encodeRequest`

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | each fix is inside its producer; no new path |
| No unintended coupling | Yes | no new import; the boot tag lives in the accountant |
| No duplicated functionality | Yes | extends `oneCredentialType`, `replayKey`, `genSessionID` |
| Zero-copy preserved where applicable | Yes | the replay key gains a port field; `genSessionID` stays one `textbuf.Buffer` build per session (session start, not per packet) |
| Registration over hardcoding, outbound | N-A | no command, family or handler added |
| Registration over hardcoding, inbound | Yes | the credential set is the RFC's closed list of four types, not a feature registry; searched `client.go` and `dict.go` |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | No Ze builder emits ARAP-Password (70), so D1 changes no request Ze sends | grep of `radius.Attr{Type:` in `authradius/*.go`, `radius/*.go`; `dict.go` declares no ARAP constant | D1 would refuse a live login | grep at implementation, plus the full `internal/component/radius` and `authradius` package tests | unvalidated |
| A-2 | A legitimate CoA retransmission keeps its Request Authenticator | RFC 5176 Section 2.3, quoted under Required Reading | (B) would miss duplicates from a client that re-signs | the RFC text; interop retransmission by `radclient` | unvalidated |
| A-3 | The only consumer of the Acct-Session-Id format is `findSessions` (prefix) | grep of `acctSessID` / `AttrAcctSessionID` in `authradius`; `.ci` files and docs assert no format. `coa.go::findSubscriberSession` also reads an Acct-Session-Id, but it compares against the subscriber registry's `AcctSessionID` (`l2tp-T-S` from `subscriber_bridge.go` line 72, or the PPPoE id), which never holds the accountant's id, so it consumes no part of this format; that mismatch is S-2 of `spec-rfc5176-remaining-defect-classes` | a changed format breaks a consumer | grep at implementation; `test/l2tp/radius-acct-wire.ci` and `radius-coa-listener.ci` still pass | unvalidated |
| A-4 | FreeRADIUS `radclient` can set the request Identifier (`-i`) and send from distinct processes (distinct source ports) | FreeRADIUS v3 `radclient` usage, not yet read from the pinned image | the interop duplicate/non-duplicate halves need a raw sender beside radclient | `radclient -h` inside the pinned image | unvalidated |
| A-5 | Every test that sends the same CoA wire twice is among the four listed under "Tagged tests that change" | grep of `replay\|duplicate` in `authradius/*_test.go` | an unlisted test goes red or turns vacuous | full `authradius` package test after the key change, and a grep for tests calling a send helper twice with one wire | unvalidated |
| A-6 | `crypto/rand.Read` cannot fail on Go 1.27 | go.mod `go 1.27.2`; since Go 1.24 `crypto/rand.Read` does not return an error | a failure would mint a constant tag | Go release notes at implementation | unvalidated |
| A-7 | The first `genSessionID` runs after the NTP plugin's Phase 1 has restored the saved clock, on a box that runs ze's NTP | the first id is minted at the first `SessionIPAssigned` (`acct.go::onSessionIPAssigned`), which needs a tunnel, a session and PPP address assignment after plugin start; no code orders it after `ntp.go::syncWorker.run` Phase 1 | the time part reads the kernel boot clock for that one boot; uniqueness still holds through the random half (AC-3d) | read the plugin start order at implementation; the claim the spec makes stays "at least the restored clock in practice", never a guarantee | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Adding the port makes the four fresh-socket tests either red (Inside, FromCache, Replay) or vacuous (EndOutside: it would pass because the port differs, not because the window expired) | the tests' teardown counts | rewrite their sends to reuse one socket (Q-3); re-record discrimination for each |
| R-2 | (B) leaves a cache entry keyed on the triple; a new request with a different Authenticator must REPLACE it, or a later retransmission of the first request would be answered with the second's response | a test: request X, request Y same triple, retransmit X | key on the triple, store the Authenticator in the entry, compare on lookup, overwrite on `rememberReplay`. Replacement happens only when the new request earns a response (`rememberReplay` is called from `sendResponse` only); a request silently discarded after the cache check leaves the old entry, which is intended (AC-2f) |
| R-3 | Two DACs behind one NAT with the same Identifier are now distinct only by port; a NAT that rewrites ports between retransmissions defeats detection | none in tests | inherent in RFC 5176 Section 2.3's triple; documented in `docs/guide/l2tp.md` |
| R-4 | A longer Acct-Session-Id overflows a server column | FreeRADIUS `radacct.acctsessionid` is varchar(64) in its stock schema | the 24-hex-digit boot tag (8 time, 16 random) keeps the id at 47 characters at most: 5-digit tunnel and session numbers, three dashes, the tag and a 10-digit counter |
| R-5 | The changed id format confuses `findSessions` | `TestDisconnect*` and CoA match tests | keep "T-S-" first; boot tag and counter follow |
| R-6 | A retired accountant (`detach`) mints no id, but a future change could construct a second `radiusAcct` in one process with a new tag | none | the tag is per accountant; two accountants in one process each have a unique tag, so uniqueness still holds |
| R-7 | The new `interop-radius-subscriber` lab is larger than the code fix and needs the PPPoL2TP kernel module and `/dev/ppp` for its session source | the shared Docker kernel check refusal (`l2tp-ppp` kernelcap) or the shared `/dev/ppp` preflight refusal | Q-7: one lab shared with `spec-rfc5176-remaining-defect-classes`; the scenario names are fixed here; the kernel check is the shared one, and the `/dev/ppp` half of BOTH the L2TP and the PPPoE preflight moves into `interoplab` (Q-7 Preflight row), so after the move no probe is declared twice |
| R-8 | The restored pre-NTP clock (or, if A-7 fails, the kernel boot clock) repeats across two boots, so the time part of the tag repeats | AC-3d test with a fixed `acctNow` | the 64-bit random part still differs; a repeat needs equal seconds and equal random bits |

## Blast Radius
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | D1: an Access-Request refused (none Ze builds today). D2: a CoA applied twice or answered from a stale cache. D3: billing records that do not join, or a CoA that matches no session |
| How is it reverted? | single commit revert per defect; the Q-1 boot tag needs no config or storage migration |
| Who else touches this path? | `spec-rfc5176-remaining-defect-classes` (coa.go matching and atomicity, the stale-id prefix match moved there by Q-6, and the shared `interop-radius-subscriber` lab of Q-7), `spec-lifecycle-invariants` (acct.go D-6, RFC2866-5.5-1), `spec-rfc-verdict-fix-access` (verdicts, planned `radius-coa-disconnect-freeradius`) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| CoA on UDP 3799 | → | `replayKey` | `TestRFC5176DuplicateIsSameSourcePortAndIdentifier` (new, sends over a real socket to `newCoAListener`) |
| `Client.Exchange` with an Access-Request | → | `oneCredentialType` | `TestAccessRequestRefusesTwoCredentialTypes` (extended) via `encodeRequest`, and `TestRFC3579ARAPBesideEAPIsRefusedAtExchange` (new, through `Client.Exchange` to a capture server) |
| `SessionIPAssigned` event on the bus | → | `genSessionID` | `TestRFC2866AcctSessionIDUniqueAcrossRestart` (new, two accountants, each fed a `SessionIPAssigned` for the same tunnel and session, Start records captured) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Access-Request with EAP-Message and ARAP-Password | refused |
| AC-2 | CoA retransmitted from the same IP and port with the same Identifier | answered once |
| AC-3 | accounting after a restart | no repeated Acct-Session-Id |
| AC-4 | the `weak` verdicts of RFC3579-3.3-2, RFC5176-2.3-2 and RFC2866-5.5-1 (the D3 row), after this spec's producer fix | each verdict reaches `enforced`: a tagged test proves the quoted sentence, and an agent that did not write that test re-judges it with `./le rfc audit-stamp ... mode rejudge`. Moved here from "Blocked by" in `plan/pre-release/spec-rfc-verdict-fix-access.md` (parent P-3, 2026-09-28) |

### Acceptance criteria refined by the 2026-10-13 design (AC-1 to AC-4 above stand; these make them testable)

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1a | an Access-Request through `encodeRequest` carrying each of the six pairs of {User-Password, CHAP-Password, ARAP-Password (type 70), EAP-Message} | each refused with an error containing "one credential type" and naming both attribute names; no octets encoded |
| AC-1b | an Access-Request carrying exactly one of the four types (EAP-Message with its Message-Authenticator; ARAP-Password alone) | encodes (n > 0, no error) under Q-4 (i) |
| AC-1c | EAP-Message plus ARAP-Password through `Client.Exchange` to a capture server | `Exchange` returns an error and the server reads zero datagrams |
| AC-1e | (Q-5) `TestRFC2869EAPBesideAnotherCredentialIsRefused` with ARAP-Password (literal 70) beside EAP-Message through `Client.Exchange`; `TestRFC2869EAPOnlyAccessRequestIsSent` | the ARAP pair is refused with zero datagrams on the wire; the EAP-only datagram carries no attribute of type 70 |
| AC-1d | `dict.go` after the fix | declares no `Attr*` constant valued 70; `TestRFC2869DictionaryDeclaresNoAttributeForAnUnofferedService` still passes unchanged |
| AC-2a | a Disconnect-Request sent twice, byte-identical, from ONE client socket (same IP, port, Identifier, Authenticator) | both answered with identical octets; the session is torn down once (teardowns == 1) |
| AC-2b | the same wire sent from two different client sockets (same IP and Identifier, different source port) | not a duplicate: the second is processed afresh (teardowns == 2 against a service whose session survives teardown, or its own fresh answer) |
| AC-2c | under Q-2 (B): request X, then request Y from the same socket with the same Identifier but a different Authenticator, then X's wire again | Y is processed afresh (not answered with X's cached response); X's retransmission after Y is processed afresh too, because Y's response replaced the entry. The Identifier-wrap case (X with Identifier 0, 255 other Identifiers, then Z with Identifier 0 and a new Authenticator) is this same triple-and-new-Authenticator case, because under the triple key the 255 intermediate requests touch other entries, so it has no unit of its own (review NIT-8, 2026-10-14) |
| AC-2f | (when the entry is replaced) request X answered, then Y on the same triple with a new Authenticator that is silently discarded after the cache check (a stale Event-Timestamp, `coa.go::handlePacket`, RFC 5176 Section 6.3), then X's wire again | X's retransmission is answered from X's cached response: replacement happens in `rememberReplay`, which only `sendResponse` calls, so a request that earns no response leaves the old entry. Intended, on Ze's reading: RFC 5080 Section 2.2.2 purges on a "valid" request with a different Authenticator and does not define "valid"; Y passed both authenticator checks, and Ze reads valid as accepted for processing. Y is not accepted, because RFC 5176 Section 6.3 says "If the Event-Timestamp Attribute is not current, then the packet MUST be silently discarded.", so Y earns no response and X's cached answer is still the one X earned |
| AC-2d | `TestRFC5176DuplicateAndTimestampWindowsBothHoldInside` / `EndOutside` rewritten to send from one socket | inside the window the duplicate is answered from the cache; past it, processed again; the negative goes red if `pruneReplayLocked` is disabled (not merely because the port differs) |
| AC-3a | two accountants constructed in turn (a simulated process restart), each starting a session with the same tunnel and session ids | the two Accounting-Start records carry different Acct-Session-Id values |
| AC-3b | one accountant, one session, Start, Interim-Update and Stop | all three carry the same Acct-Session-Id, which begins "<tunnel>-<session>-" |
| AC-3c | a Disconnect-Request carrying a current session's Acct-Session-Id | `findSessions` still matches it (existing CoA tests stay green with ids from the new format) |
| AC-3d | (Q-1, clock repeats: a box with no RTC whose first session is assigned before NTP answers) two accountants with `acctNow` fixed to the SAME second at each one's first `genSessionID`, each starting a session with the same tunnel and session ids | the two Acct-Session-Id values differ; their 8 time digits are equal and their 16 random digits differ |
| AC-3e | (Q-1, clock advances) two accountants with the random source replaced by one constant byte sequence and `acctNow` set to two different seconds | the two Acct-Session-Id values differ; each tag's first 8 hex digits are the `acctNow` Unix seconds read at that accountant's first `genSessionID` |
| AC-3f | (Q-1, mint time) one accountant constructed while `acctNow` reads second S1, then `acctNow` moved to S2 before its first session starts; a second session after `acctNow` moves to S3 | both sessions' tags open with S2 in hex: the tag is read at the first `genSessionID`, never at construction, and never again. Two breaks, each observed red: mint the tag in `newRADIUSAcct` (both tags open with S1), and mint it again on each `genSessionID` call (the second tag opens with S3) |
| AC-5 | every tagged unit this spec adds or whose claim it changes | carries a discrimination record from `./le rfc discriminate-record`, observed red against a revert of its producer |

## End-to-End User Stories
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | an operator's RADIUS server retransmits a Disconnect-Request after a lost ACK | UDP 3799 → `serve` → `handlePacket` → `cachedReplay` → `sendRawResponse` | `TestRFC5176DuplicateIsSameSourcePortAndIdentifier`; interop `radius-coa-duplicate-freeradius` |
| 2 | two DACs behind one NAT address send Disconnect-Requests with the same Identifier | UDP 3799 → `handlePacket` → miss in `cachedReplay` → `handleDisconnect` | `TestRFC5176DifferentSourcePortIsNotADuplicate`; interop `radius-coa-duplicate-freeradius` |
| 3 | the operator restarts ze; subscribers reconnect on the same tunnel and session numbers; billing joins Start and Stop records in the server log | `SessionIPAssigned` → `onSessionIPAssigned` → `genSessionID` → `buildAcctPacket` → server log | `TestRFC2866AcctSessionIDUniqueAcrossRestart`; interop `radius-acct-restart-freeradius` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestAccessRequestRefusesTwoCredentialTypes` (extended: all six pairs, ARAP via literal 70) | `internal/component/radius/authenticator_eap_test.go` | AC-1a; RFC3579-3.3-2 negative. Red at HEAD for the three ARAP pairs and passes for UP+CHAP (already refused) | |
| `TestRFC3579EachCredentialTypeAloneEncodes` (new) | `internal/component/radius/authenticator_eap_test.go` | AC-1b; RFC3579-3.3-2 positive. Green at HEAD (control for the negative) | |
| `TestRFC3579ARAPBesideEAPIsRefusedAtExchange` (new) | a new `internal/component/radius/rfc3579_credential_types_test.go` | AC-1c; RFC3579-3.3-2 negative at the public entry point. Red at HEAD | |
| `TestRFC5176DuplicateIsSameSourcePortAndIdentifier` (new) | `internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go` | AC-2a; RFC5176-2.3-2 positive over one socket | |
| `TestRFC5176DifferentSourcePortIsNotADuplicate` (new) | same file | AC-2b; RFC5176-2.3-2 negative. Red at HEAD (HEAD answers the second port from the cache) | |
| `TestRFC5176SameTripleNewAuthenticatorIsProcessedAfresh` (new) | same file | AC-2c (Q-2 B), including the replace-then-retransmit order of R-2, and AC-2f (a Y discarded for a stale Event-Timestamp leaves X's entry, so X's retransmission is answered from the cache) | |
| `TestRFC2866AcctSessionIDUniqueAcrossRestart` (new) | `internal/component/l2tp/plugins/authradius/rfc2866_accounting_test.go` | AC-3a; RFC2866-5.5-1 positive. Red at HEAD (both ids "1-2-1") | |
| `TestRFC2866AcctSessionIDKeepsTunnelSessionPrefix` (new) | same file | AC-3b, AC-3c; preserves the `findSessions` contract | |
| `TestRFC2866AcctSessionIDUniqueWhenClockRepeats` (new) | same file | AC-3d; RFC2866-5.5-1 positive for a pre-NTP boot: `acctNow` fixed to one second for both accountants. Red at HEAD (both ids "1-2-1"); discrimination break: drop the random part | |
| `TestRFC2866AcctSessionIDTagCarriesBootTime` (new) | same file | AC-3e and AC-3f: random source replaced by constant bytes, two `acctNow` seconds; ids differ and each tag opens with the seconds read at its first `genSessionID`; a clock moved between construction and the first session, and again before the second, leaves both tags at the first-session second. Untagged (the time part orders boots; uniqueness is AC-3d's claim); its red is observed against a build with the time part removed, and its AC-3f half against two builds: one that mints the tag in `newRADIUSAcct`, and one that mints it again on each `genSessionID` call | |
| `TestReplayKeyCarriesSourcePort` (new, untagged) | `internal/component/l2tp/plugins/authradius/coa_test.go` or beside the walk tests | the key differs for two `*net.UDPAddr` differing only in port | |

### Tagged tests that change (owner approval needed, Q-3 and Q-5)
| Test | File | Requirement (verdict) | Change | Why |
|------|------|-----------------------|--------|-----|
| `TestRFC5176DuplicateRequestAnsweredFromCache` | `authradius/rfc5176_walk_test.go` | RFC5176-2.3-2 (`weak`) | the two duplicate copies go out on one socket, and the third "fresh" request (Identifier 7, today a new `sendRawCoAPacket` socket) goes out on that SAME socket, so the negative differs from the duplicate in the Identifier only; comment's "same source" made precise, and the prose says that under Q-2 (B) the Request Authenticator differs too, which is unavoidable because it hashes the Identifier | today the copies come from two sockets; with the port in the key the second is no longer a duplicate and teardowns becomes 2. A third request on a new socket would differ in port AND Identifier and no longer isolate the Identifier (review verdict UNDER-TEST unless revised) |
| `TestRFC5176DuplicateAndTimestampWindowsBothHoldInside` | `authradius/rfc5176_section_rows_test.go` | RFC5176-6.3-2 (`enforced`) | `coaRawExchange` pair replaced by two sends on one socket | goes red otherwise (second copy processed) |
| `TestRFC5176DuplicateAndTimestampWindowsBothEndOutside` | `authradius/rfc5176_section_rows_test.go` | RFC5176-6.3-2 (`enforced`) | same | stays green but VACUOUS otherwise: the second copy would be processed because the port differs, not because the window expired |
| `TestDisconnectReplayReturnsCachedResponse` | `authradius/rfc5176_coa_listener_test.go` | RFC5176-3.3-1 (`enforced`) | the replay goes out on the first socket | goes red otherwise. The tag claims only "a valid Acct-Session-Id acts on the matching session" and the replay half is untagged, so the claim is unchanged. `spec-rfc5176-remaining-defect-classes` also edits this unit's fixture: the two edits are coordinated, whichever spec lands second rebases onto the other |
| `TestAccessRequestRefusesTwoCredentialTypes` | `radius/authenticator_eap_test.go` | RFC3579-3.3-2 negative | extended to all six pairs of the four types, ARAP via literal 70 (AC-1a) | at HEAD its tag quotes the whole four-type sentence while the body tests two pairs (OVER-CLAIM at HEAD); the extension makes the claim true |
| `TestRFC2869EAPBesideAnotherCredentialIsRefused` | `radius/rfc2869_credential_types_test.go` | RFC2869-5.19-1 negative (`enforced`) | (Q-5 "fix and extend") the map of other credentials gains "ARAP-Password", an attribute of type 70 named by the same unexported literal `client.go` uses; the tag prose names ARAP-Password beside EAP-Message (AC-1e) | at HEAD the tag quotes the four-type sentence while the body drives two of the three EAP pairs; red at HEAD for the ARAP pair |
| `TestRFC2869EAPOnlyAccessRequestIsSent` | `radius/rfc2869_credential_types_test.go` | RFC2869-5.19-1 positive (`enforced`) | (Q-5) asserts the sent datagram carries no attribute of type 70; the tag prose names "none of User-Password, CHAP-Password or ARAP-Password" (AC-1e) | the positive names three of the four types at HEAD |
| `rfc2869_credential_types_test.go` header comment | `radius/rfc2869_credential_types_test.go` | - (outside every unit) | (Q-5) "ARAP-Password is not driven" replaced by: ARAP-Password is recognised by literal 70 and driven beside EAP-Message here, while `dict.go` still declares no ARAP attribute | false once D1 lands (`ai/rules/stale-comments.md`) |
| `encodeRequest` caller comment (code, not a test) | `radius/client.go` | - | "ARAP-Password is not named" replaced | stale once 70 is in the set |

`TestRFC2866AcctSessionIDUnique` and `TestRFC2866AcctSessionIDNoCollisionOnReusedKey` do not change: both stay true with the boot tag.
Every changed tagged unit stales its `rfc/discrimination/<stem>.json` record (rfc5176: 4 units holding 5 records, `rfc/discrimination/rfc5176.json` lines 51, 62, 425, 755, 766 on 2026-10-14; rfc3579: 1 unit, per a grep on 2026-10-13; rfc2869: the 2 Q-5 units, whose records sit at `rfc/discrimination/rfc2869.json` lines 73 and 84 on 2026-10-14); each is re-recorded by `./le rfc discriminate-record`.

Discrimination records owed (`./le rfc discriminate-record`, AC-5), one per unit:

| Stem | Changed units (re-record) | New tagged units (first record) |
|------|---------------------------|---------------------------------|
| rfc3579 | `TestAccessRequestRefusesTwoCredentialTypes` | `TestRFC3579EachCredentialTypeAloneEncodes`, `TestRFC3579ARAPBesideEAPIsRefusedAtExchange` |
| rfc2869 | `TestRFC2869EAPBesideAnotherCredentialIsRefused`, `TestRFC2869EAPOnlyAccessRequestIsSent` | - |
| rfc5176 | `TestRFC5176DuplicateRequestAnsweredFromCache`, `TestRFC5176DuplicateAndTimestampWindowsBothHoldInside`, `TestRFC5176DuplicateAndTimestampWindowsBothEndOutside`, `TestDisconnectReplayReturnsCachedResponse` (five records: `TestRFC5176DuplicateRequestAnsweredFromCache` holds two, grep of `rfc/discrimination/rfc5176.json` on 2026-10-14) | `TestRFC5176DuplicateIsSameSourcePortAndIdentifier`, `TestRFC5176DifferentSourcePortIsNotADuplicate`, `TestRFC5176SameTripleNewAuthenticatorIsProcessedAfresh` |
| rfc2866 | - | `TestRFC2866AcctSessionIDUniqueAcrossRestart`, `TestRFC2866AcctSessionIDUniqueWhenClockRepeats`, `TestRFC2866AcctSessionIDKeepsTunnelSessionPrefix` if tagged |

#### Owner approval commands

Approved by the owner on 2026-10-13 (Q-3, "approve all RFC-tagged test changes"; Q-5 "fix and extend" names the two RFC 2869 units under that approval). The gate reads approvals from the committing session's own file (`tmp/commit-rfc-approved-<session>.md`), and a row is used up by the commit that carries its `RFC-approved:` trailer. So the owner runs these at the prompt of the session that implements and commits the edits, and runs a unit's command again if a later commit changes that unit a second time. A unit is `<package directory>.<top-level test function>`. Every name was checked to exist at HEAD `051876c3f4`. The two RFC 2869 unit names were checked to exist on 2026-10-14 (`rfc2869_credential_types_test.go` lines 56 and 71). The `rfc2869_credential_types_test.go` header comment (Q-5) sits above the file's first function, outside every tagged unit, so the gate does not ask for it and it has no command; the new ARAP tests are new units and need none either. If `spec-rfc5176-remaining-defect-classes` lands first, the four shared `authradius` units are changed again here and need their command run again in this session.

7 commands:

```
./le rfc approve unit radius.TestRFC2869EAPBesideAnotherCredentialIsRefused reason 'Thomas, 2026-10-13: "fix and extend" and "approve all RFC-tagged test changes" (spec-radius-rfc-defects Q-5, ARAP-Password beside EAP-Message added)'
./le rfc approve unit radius.TestRFC2869EAPOnlyAccessRequestIsSent reason 'Thomas, 2026-10-13: "fix and extend" and "approve all RFC-tagged test changes" (spec-radius-rfc-defects Q-5, ARAP-Password absence asserted)'
./le rfc approve unit authradius.TestRFC5176DuplicateRequestAnsweredFromCache reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-radius-rfc-defects, duplicates and the fresh request go out on one socket)'
./le rfc approve unit authradius.TestRFC5176DuplicateAndTimestampWindowsBothHoldInside reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-radius-rfc-defects, both sends on one socket)'
./le rfc approve unit authradius.TestRFC5176DuplicateAndTimestampWindowsBothEndOutside reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-radius-rfc-defects, both sends on one socket)'
./le rfc approve unit authradius.TestDisconnectReplayReturnsCachedResponse reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-radius-rfc-defects, replay goes out on the first socket)'
./le rfc approve unit radius.TestAccessRequestRefusesTwoCredentialTypes reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-radius-rfc-defects, extended to all six credential-type pairs, ARAP via 70)'
```

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Identifier in the replay key | 0-255 | 255 and 0 distinct keys | N-A | N-A |
| Source port in the replay key | 1-65535 | two ports differing by one are distinct keys | N-A | N-A |
| `nextSess` counter | uint32 | 2^32-1; the counter then wraps to 0. Uniqueness after 2^32 sessions within one boot is not claimed by this spec (unchanged from HEAD) | N-A | N-A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `radius-coa-listener` (unchanged, must stay green) | `test/l2tp/radius-coa-listener.ci` | config `coa-port` to a bound socket answering a Disconnect-Request | |
| `radius-acct-wire` (unchanged, must stay green) | `test/l2tp/radius-acct-wire.ci` | Start/Interim/Stop on the wire; uses the id opaquely | |
| no new `.ci` | - | duplicate vs fresh processing is not observable on the wire without a live session (both answers are the same NAK 503 in `radius-coa-listener.ci`), and a daemon restart within one `.ci` is not supported; the real entry points (UDP socket, event bus, `Client.Exchange`) are driven by the unit tests above and the interop scenarios below | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `radius-coa-duplicate-freeradius` (new, named, no numeric prefix) | `test/interop-radius-subscriber/scenarios/` | FreeRADIUS `radclient` as DAC, xl2tpd + pppd as the lab's own LAC (live session) | (1) two `radclient` processes send a Disconnect-Request with the same Identifier from distinct ports: each is processed (Ze log shows two dispatches); (2) a lost first Disconnect-ACK (dropped by an `iptables` rule in the radclient container) makes `radclient` retransmit on its own socket: Ze answers from the cache and tears the session down once (Ze log, pppd log). Precondition of half (1): HEAD keys on the Request Authenticator, so HEAD treats the second copy as a duplicate only if both packets are byte-identical apart from the source port. The scenario therefore writes ONE request file (one Event-Timestamp value, written once at scenario start, inside the 5-minute window; one attribute list) and both `radclient` processes send that file with the same Identifier and secret, so their Request Authenticators are equal (RFC 2866 Section 3 makes it a function of the packet and the secret); the checker confirms the two received Authenticators are equal before judging. Forced red: revert `replayKey`, rebuild ze, (1) fails (the second port answered from the cache, one dispatch) | |
| `radius-acct-restart-freeradius` (new, named) | `test/interop-radius-subscriber/scenarios/` | FreeRADIUS accounting with the `detail` module; xl2tpd + pppd as the lab's own LAC | in FreeRADIUS's own detail log, the session's Start before a ze restart and the reconnected session's Start after it each carry an Acct-Session-Id of the form "tunnel-session-tag-counter" whose third field is the 24-hex-digit boot tag, the two tags differ, and each Stop carries its Start's id. What it proves: the boot tag reaches a real server's record and changes across a real daemon restart. It does not prove the same-(T,S) collision, because ze as LNS allocates the session id at random (`session.go::allocateSessionID`) and the tunnel id from a counter (`reactor.go::allocateLocalTID`), so a reconnected session rarely repeats (T,S); that case is the unit tests' (AC-3a, AC-3d). Forced red: revert `genSessionID`, rebuild ze, the scenario fails on every run, because HEAD's "T-S-N" ids carry no boot-tag field | |
| D1 | N-A | - | no wire-visible change: Ze builds no ARAP-Password, and the refusal happens before the socket; the existing `radius-admin-*-freeradius` scenarios must stay green (no credential combination changed) | |

#### The `interop-radius-subscriber` lab (Q-7, owner 2026-10-14: "Make Free radius, its own lab.")

| Field | Value |
|-------|-------|
| Name | `interop-radius-subscriber`: scenarios under `test/interop-radius-subscriber/scenarios/`, plumbing in a new `internal/le/interoplab/radiussubscriber/`, run by `./le test integration interop-radius-subscriber`. Lab and scenario names carry no numeric prefix (`docs/architecture/testing/interop.md`) |
| Not the L2TP lab | no FreeRADIUS container is added to `internal/le/interoplab/l2tp/`, whose RADIUS peer stays `radiusmock` |
| Not the admin-login lab | the existing FreeRADIUS suite (`internal/le/interoplab/radius/`, `test/interop-radius/`) runs unprivileged: `radius_test.go::TestSuiteNeedsNoKernelModule` holds that it mounts no `/lib/modules` tree, adds no capability, runs nothing `--privileged` and gates on readiness. This lab cannot keep that promise, because ze's kernel L2TP needs `NET_ADMIN` and the `/dev/ppp` device, as the L2TP lab grants its ze and LAC containers (`interoplab/l2tp/l2tp.go`, `pppDeviceArguments` and the `NET_ADMIN` capability) |
| Shared with | `plan/immediate/spec-rfc5176-remaining-defect-classes.md` (its Q-7 and its stale-id scenario), and the planned `radius-coa-disconnect-freeradius` of `plan/pre-release/spec-rfc-verdict-fix-access.md`. This spec names the lab and its two scenarios; the other specs add theirs to the same directory |
| Containers | ze (daemon cross-compiled through the lab's `LabBinaries`, as `interoplab/radius.LabBinaries` does), FreeRADIUS at the pinned `docker.io/freeradius/freeradius-server:3.2.7` already used by `interoplab/radius` (accounting `detail` module enabled, `radclient` for CoA and Disconnect), and an xl2tpd + pppd LAC |
| Session source | a live kernel L2TP session from the lab's own LAC. Verified at the producers on 2026-10-14 that nothing else can stand in: `acct.go::subscribeEventBus` accounts only `l2tpevents.SessionIPAssigned` / `SessionDown`; the only L2TP emitter of `SessionIPAssigned` is `reactor_kernel.go` (line 367), after the kernel session has an address; `coa.go::findSessions` matches against `l2tp.LookupService()`, published only by the Subsystem's Start (`service_locator.go::PublishService`). `coa.go::findSubscriberSession`, consulted first by `handleCoA` and by `handleDisconnect` for PPPoE, matches the subscriber registry's `AcctSessionID`, which holds `l2tp-T-S` for L2TP (`subscriber_bridge.go` line 72) or the PPPoE internal id (`pppoe/subsystem.go` line 445), never the accountant's id; PPPoE emits no `l2tpevents.SessionIPAssigned`, so the accountant never accounts it, and a PPPoE Disconnect answers 504. So a PPPoE session cannot stand in either. The registry mismatch is S-2 of `spec-rfc5176-remaining-defect-classes` (A-3) |
| Preflight | none of its own for the kernel: `Suite.Run` runs the shared Docker host kernel check (`interoplab/kernelcheck.go::DockerKernel`, through `Suite.StagedZe`) for every lab, and it refuses a host lacking an enrolled ze kernel capability, which includes `l2tp` (CONFIG_L2TP) and `l2tp-ppp` (CONFIG_PPPOL2TP) from `internal/component/l2tp/kernelcap_linux.go`. The one part that check does not cover is the `/dev/ppp` character device. Two labs hold that half today, each its own copy: the L2TP lab (`interoplab/l2tp/l2tp.go`: `pppDevice`, `pppDeviceArguments`, the DEV_PPP line of `preflightContainer`, the DEV_PPP check of `preflightRefusal`, the `/dev/ppp` hint `preflight` adds when the container does not run, and `parsePreflight`) and the PPPoE lab (`interoplab/pppoe/pppoe.go`: `pppDevice`, `deviceFlag`, `pppDeviceArguments`, the DEV_PPP line of `preflightContainer`, the DEV_PPP check and the inline KEY=value parse of `validatePreflightOutput`, and the `/dev/ppp` stderr refusal of `preflightRunRefusal`). BOTH copies move into one file in package `interoplab`, which holds: (1) the device grant arguments (`--device /dev/ppp`); (2) the DEV_PPP probe fragment, one shell line that prints `DEV_PPP=ok` or `DEV_PPP=missing`; (3) the parser of the probe's KEY=value lines; (4) the DEV_PPP check, which names "/dev/ppp (the PPP device, module ppp_generic)" when the value is not ok; (5) the run refusal, which takes the lab name and the `RunOneShot` error and always wraps that error with `%w`, so the operation, exit code, stderr and cause that `commandError` carries survive for every Docker failure (L2TP's behaviour today; PPPoE's `preflightRunRefusal` keeps only rc and stderr and drops the cause, and gains it), and adds the `sudo modprobe ppp_generic` line, preceded by PPPoE's explanation that the Docker host has no PPP device and lab containers never load kernel modules, only when the stderr names `/dev/ppp`. The timeout refusal (PPPoE's `context.DeadlineExceeded` arm) is not part of the move and stays in the PPPoE lab; L2TP has none. The shared file's test carries an L2TP case for a non-`/dev/ppp` failure: Docker's words and the cause kept, no PPP hint. Each lab still runs ONE preflight container: its `preflightContainer` puts the shared fragment first in its single `sh -c` command and adds its own probe lines (L2TP_PPP and IP_L2TP for L2TP, PPPOE for PPPoE); its refusal parses with the shared parser, takes the DEV_PPP entry from the shared check, and adds its own entries and its own module list; its run-failure path uses the shared run refusal. Each lab keeps its skip-probe refusal. This lab includes the same fragment in its own one container. After the move no lab declares the `/dev/ppp` probe, grant or check, so the fact is declared once |
| Checkers | both sides, per `docs/architecture/testing/interop.md`: ze's log (dispatches, teardowns, Acct-Session-Id values) and FreeRADIUS's own record (the `detail` file, `radclient` output) |
| Rejected | a lab with no L2TP, where a test fixture drives ze's RADIUS client: the fixture would have to publish a stub `l2tp.Service`, emit `SessionIPAssigned` on its own bus and construct the accountant and CoA listener, whose constructors (`newRADIUSAcct`, `newCoAListener`, `activateRadiusConfig`) are unexported in package `authradius`. A lab binary would need a new exported seam in production code, and it would skip the daemon's config path and the kernel emit path, so it proves less than the real entry point |

### Goal validation
| Goal | Evidence owed |
|------|---------------|
| D1: no Access-Request leaves Ze with two credential types | AC-1a/1c tests, with a discrimination record observed red against a revert of the type-70 entry |
| D2: the DAS detects duplicates on source IP, source port and Identifier | `radius-coa-duplicate-freeradius` passing with its forced red, plus AC-2a/2b/2c |
| D3: Acct-Session-Ids do not repeat across a restart in the server's log | `radius-acct-restart-freeradius` passing (boot tags present and different across the restart in FreeRADIUS's detail log) with its forced red, plus AC-3a/3d for the same-(T,S) case the scenario cannot force |
| AC-4 | each of the three verdicts stamped `enforced` by an independent `mode rejudge` |

## Files to Modify

- `internal/component/radius/client.go` - add ARAP-Password (70, unexported literal) to `oneCredentialType`; replace the stale caller comment; RFC 3579 Section 3.3 quote stays above the check
- `internal/component/l2tp/plugins/authradius/coa.go` - (Q-2 B) `coaReplayKey` becomes source IP, source port and Identifier (the Code leaves it, the Authenticator moves from the key to the entry); a lookup whose Authenticator differs from the entry's is a miss, and `rememberReplay` overwrites the entry; RFC 5176 Section 2.3 quote above `replayKey`; `cachedReplay`, `rememberReplay` and `replayKey` take `*net.UDPAddr`; the callers are `handlePacket` (`cachedReplay(from.IP, pkt)`) and `sendResponse` line 828 (`rememberReplay(to.IP, req, wire)`), which pass the whole address; the `findSessions` comment updated to the new format
- `internal/component/l2tp/plugins/authradius/acct.go` - (Q-1 "c") `radiusAcct` gains a boot tag minted once, at its first `genSessionID` call (once-only and safe under concurrent first calls), never in `newRADIUSAcct`, which runs at package init: 8 hex digits of the Unix seconds from `acctNow`, then 16 hex digits from `crypto/rand` read through a package variable a test replaces (the same seam shape as `acctNow`); `genSessionID` writes "tunnel-session-tag-counter"; RFC 2866 Section 5.5 quote above it
- the tests listed under "Unit Tests" and "Tagged tests that change"
- `rfc/discrimination/rfc3579.json`, `rfc/discrimination/rfc2869.json`, `rfc/discrimination/rfc5176.json`, `rfc/discrimination/rfc2866.json` (re-recorded by the tool, never by hand); `rfc/audit/rfc3579.json`, `rfc5176.json`, `rfc2866.json` (stamped by an independent judge)
- `docs/guide/l2tp.md` - one sentence each on the duplicate key and on restart-unique Acct-Session-Id
- `docs/architecture/testing/interop.md` - the suites table gains the `interop-radius-subscriber` row, and a section on that lab (Q-7) beside "The FreeRADIUS admin-login suite"
- `internal/le/interoplab/l2tp/l2tp.go` - its `/dev/ppp` half (`pppDevice`, `pppDeviceArguments`, the DEV_PPP probe line and check, the `/dev/ppp` run hint, `parsePreflight`) moves to package `interoplab` (Q-7 Preflight row); its one preflight container puts the shared fragment first and keeps its L2TP_PPP and IP_L2TP probes, its refusal and the skip-probe refusal
- `internal/le/interoplab/pppoe/pppoe.go` - its `/dev/ppp` half (`pppDevice`, `deviceFlag`, `pppDeviceArguments`, the DEV_PPP probe line, the KEY=value parse and DEV_PPP check of `validatePreflightOutput`, the `/dev/ppp` stderr refusal of `preflightRunRefusal`) moves to the same shared file; its one preflight container puts the shared fragment first and keeps its PPPOE probe and the skip-probe refusal
- `internal/le/interoplab/l2tp/privilege_test.go`, `internal/le/interoplab/pppoe/privilege_test.go`, `internal/le/interoplab/pppoe/pppoe_test.go` - their DEV_PPP cases move to the shared file's test; each keeps the assertions on its own lab's container grants and probes
- `internal/le/interoplab/` - a new file holding the shared `/dev/ppp` grant, probe fragment, parser, check and run refusal moved from both labs, with its test

## Files to Create
- `internal/le/interoplab/radiussubscriber/` - the lab of Q-7 (suite calling the shared `/dev/ppp` preflight, checkers, catalog registration)
- `test/interop-radius-subscriber/` - lab files (FreeRADIUS `clients.conf`, `detail` module, xl2tpd and pppd config)
- `test/interop-radius-subscriber/scenarios/radius-coa-duplicate-freeradius/`
- `test/interop-radius-subscriber/scenarios/radius-acct-restart-freeradius/`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | no config added |
| YANG validation constraints | N-A | no leaf added |
| YANG custom validators | N-A | no leaf added |
| CLI commands/flags | N-A | none |
| CLI grammar (keyword before value) | N-A | none |
| Editor autocomplete | N-A | none |
| Functional test for new RPC/API | N-A | no RPC; see Functional Tests for why no new `.ci` |
| Pipe completeness | N-A | no output |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | N-A | the Q-1 boot tag adds no file, socket or storage, and waits on no clock |
| Prometheus counters/metrics | No | a duplicate counter would be useful but is not asked for |
| BGP family surface | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | defect fixes; the L2TP feature's evidence gains interop, which `./le feature report` reads from the scenarios |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | behavior inside `authradius` only |
| 6 | Has a user guide page? | Yes | `docs/guide/l2tp.md` (CoA/DM and accounting sections) |
| 7 | Wire format changed? | No | the Acct-Session-Id is opaque text; its content changes, not its encoding |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc2866.md` enrolment-reason text naming genSessionID's restart property; verdicts in `rfc/audit/`; `docs/features/rfc-status.md` regenerates |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/interop.md` (the `interop-radius-subscriber` lab section and suites row, and the L2TP and PPPoE preflight text, today "the preflight probes with the same grants" and "its preflight probes the same way", naming the shared `/dev/ppp` probe both labs include) |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` cites acct.go for features, not the id format; confirm with the anchors command |
| 12 | Internal architecture changed? | No | - |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | - |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `./le spec citation anchors spec plan/immediate/spec-radius-rfc-defects.md` at implementation; known from `ai/CODE-TO-DOCS.md`: acct.go → `bng-1-radius-attributes.md`, `subscriber-session-model.md`, `comparison.md`, `guide/l2tp.md`; coa.go → `guide/l2tp.md`; client.go → `bng-1-radius-attributes.md`; `// Design:` of all three is `docs/research/l2tpv2-ze-integration.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | No | no example shows an Acct-Session-Id value |

## Implementation Steps

1. **Phase: Wiring** -- write the failing wiring tests: `TestRFC5176DifferentSourcePortIsNotADuplicate`, `TestRFC3579ARAPBesideEAPIsRefusedAtExchange`, `TestRFC2866AcctSessionIDUniqueAcrossRestart`; confirm each is red at HEAD for the reason named in the TDD table.
2. **Phase: D1** -- extend `TestAccessRequestRefusesTwoCredentialTypes` to six pairs and add the single-type positive; extend the two RFC2869-5.19-1 units with ARAP (Q-5, AC-1e); add 70 to `oneCredentialType`; fix the two stale comments (`client.go` caller comment, `rfc2869_credential_types_test.go` header); `TestRFC2869Dictionary*` unchanged and green.
3. **Phase: D2** -- with Q-3 approved, rewrite the four tagged tests to send from one socket (they then pass at HEAD, which is correct: HEAD detects same-port duplicates); add the AC-2 tests; change the key; confirm EndOutside goes red with pruning disabled.
4. **Phase: D3** -- boot tag per Q-1 (seconds read at the first `genSessionID` plus 64 random bits); add the AC-3 tests, AC-3d, AC-3e and AC-3f included; confirm CoA tests stay green.
5. **Phase: Discrimination and verdicts** -- `./le rfc discriminate-record` for every new or changed tagged unit; an independent agent stamps the three verdicts with `mode rejudge`.
6. **Phase: Interop** -- build the `interop-radius-subscriber` lab (Q-7) unless `spec-rfc5176-remaining-defect-classes` built it first; add the two scenarios; each scenario's forced red recorded.
7. **Phase: Docs** -- `docs/guide/l2tp.md`, `interop.md`, `rfc/short/rfc2866.md` text, in the same phase as each code change.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1a..AC-5 each at a file:line |
| Correctness | the cache lookup still happens after both authenticator checks; R-2's replace order holds |
| Correctness | no `Attr*` constant valued 70 in `dict.go` |
| Data flow | `findSessions` prefix unchanged; one id per session |
| Rule: rfc-compliance | new implementing code carries the RFC quote above it; callers name the section |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| three producer fixes | the three red-first tests green under `./le job run` scoped package tests |
| verdicts enforced | `jq` of the three ids in `rfc/audit/*.json` |
| discrimination | `./le rfc check` OK with the new tags |
| interop | the two scenario logs with their forced reds |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | the replay map is filled only by authenticated packets from allowed sources, and only by those that earn a response |
| Resource exhaustion | under (B) the key is (source IP, source port, Identifier), so one source holds at most 65535 x 256 entries within the window. Today's key carries the Authenticator and has no per-source size bound, so (B) tightens it. The only other bound is the 5-minute prune, and `pruneReplayLocked` is an O(n) scan of the whole map run on every lookup and every store (`cachedReplay`, `rememberReplay`); that cost is pre-existing and unchanged |
| Error leakage | the D1 error names attribute names only, never values |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| D1: literal 70 in `client.go` | an ARAP constant in `dict.go` (forbidden by RFC 2869 Section 1.1 and its test); refusing all ARAP attributes (Q-4 ii) | minimal change that makes the four-type sentence true without implementing ARAP |
| D2: triple key, Authenticator as tie-breaker (Q-2 B) | literal triple only (Q-2 A) | (A) answers a new request after an Identifier wrap with a stale response; RFC 5176's own retransmission rule makes (B) lose no legitimate duplicate; RFC 5080 Section 2.2.2 prescribes (B)'s shape for the Access-Request analog |
| D3: time-based boot tag, seconds read at the first `genSessionID` plus 64 random bits (Q-1, owner "c", 2026-10-13; the random half is the designer's addition) | wait for a synced clock before the first id; mint at construction | ze knows sync only through its NTP plugin when configured, so waiting needs a timeout and a second scheme, and holds every Accounting-Start; construction runs at package init, before NTP's Phase 1 restores the clock; the restored pre-NTP clock can still repeat across boots, which the random part absorbs |
| D2 key without Code (Q-2 B) | keep Code in the key | the Request Authenticator hashes the Code (RFC 2866 Section 3), so Code adds nothing once the Authenticator is compared |
| Q-7: own FreeRADIUS lab with its own LAC | FreeRADIUS in the L2TP lab (owner chose against it); a no-L2TP fixture | see "The `interop-radius-subscriber` lab" |

## Known Limitations
- `findSessions` prefix matching across sessions that reuse tunnel and session numbers: owned by `plan/immediate/spec-rfc5176-remaining-defect-classes.md` (S-1, AC-1), moved there by Q-6 (owner, 2026-10-14). This spec keeps the prefix.

## Design Review Log

| Date | Round | Outcome |
|------|-------|---------|
| 2026-10-14 | owner answers applied | Q-1 "c", Q-2 "B", Q-4 "i", Q-5 "fix and extend", Q-6 "whatever" (routed to `spec-rfc5176-remaining-defect-classes`), Q-7 "Make Free radius, its own lab." recorded with their consequences; owner answers applied 2026-10-14; review round owed |
| 2026-10-14 | review of the owner-answer edits (0 BLOCKER, 4 ISSUE, 8 NIT), applied | ISSUE-1: boot tag minted at the first `genSessionID`, not at package init (AC-3f, A-7), honest limit on RTC-less boxes stated, random half attributed to the designer (also NIT-5). ISSUE-2: `radius-acct-restart-freeradius` now asserts a boot-tag field in both ids and different tags, red at HEAD on every run; the same-(T,S) case stays with AC-3a/3d. ISSUE-3: Documentation row 10 names the own lab. ISSUE-4: kernel support rests on the shared Docker kernel check (`l2tp`, `l2tp-ppp` kernelcaps); the `/dev/ppp` probe moves into `interoplab`, shared by both labs. NIT-6 resource bound restated; NIT-7 AC-2f (a discarded request leaves the old entry, intended); NIT-8 AC-2e and `TestRFC5176WrappedIdentifierIsProcessedAfresh` dropped (reduces to AC-2c); NIT-9 byte-identical `radclient` precondition; NIT-10 record counts and `rfc2869.json` in Files; NIT-11 admin-login lab reason reworded; NIT-12 `findSubscriberSession` and sister S-2 cited in A-3 and the session-source row. Sister spec Q-1/Q-7 updated there. Review round owed |
| 2026-10-14 | review round 2, narrow, of the round-1 edits (0 BLOCKER, 1 ISSUE, 4 NIT), applied | ISSUE-1: the PPPoE lab holds its own copy of the `/dev/ppp` half, so BOTH copies (L2TP and PPPoE) move into one shared file in package `interoplab`; `pppoe/pppoe.go` and the two labs' tests added to Files to Modify, the PPPoE preflight text to Documentation row 10, and the "declared once" claim (Preflight row, R-7) now rests on both moves. NIT-2: the shape is named: a shared probe fragment, parser, check and run refusal that each lab's single preflight container includes, so each lab still runs one container (read at `l2tp.go` and `pppoe.go` `preflightContainer`, `validatePreflightOutput`, `preflightRunRefusal`). NIT-3: AC-2f says Ze reads "valid" as accepted for processing and quotes RFC 5176 Section 6.3. NIT-4: AC-3f and its test row name both breaks. NIT-5: sister spec's lab Files point at this spec's lab rows. Round 3 owed, narrow, over these edits |
| 2026-10-14 | review round 3, narrow, of the round-2 edits (0 BLOCKER, 1 ISSUE, 0 NIT), applied | ISSUE-1: the shared run refusal was defined only by its `/dev/ppp` arm, which in PPPoE's shape drops `commandError`'s cause and chain for every other Docker failure. Now defined fully: it always wraps the `RunOneShot` error with `%w` and adds the `ppp_generic` hint only when stderr names `/dev/ppp`; the timeout arm stays in the PPPoE lab; the shared test carries a non-`/dev/ppp` L2TP case. Round 4 owed, narrow, over this one edit |
| 2026-10-14 | review round 4, narrow, of the round-3 edit (0 BLOCKER, 0 ISSUE, 1 NIT) | NIT applied: item (5) says the `ppp_generic` hint keeps PPPoE's explanation. Design converged; owner questions still open as listed |
| 2026-10-14 | cross-spec dependency note, no design change | from `spec-rfc5176-remaining-defect-classes` design review r1 (NIT-11): the Dependencies line under the metadata records that spec landing after this one and the prefix contract ending with its S-1 |

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code: RFC 3579 Section 3.3 Note 1 above `oneCredentialType`'s set, RFC 5176 Section 2.3 above `replayKey`, RFC 2866 Section 5.5 above `genSessionID`.

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (before the fix)
- [ ] Tests PASS (after the fix)

### Verification
- [ ] `./le verify worktree`
