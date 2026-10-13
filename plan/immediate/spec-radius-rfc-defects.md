# Spec: radius-rfc-defects

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-13 |

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

| Row | Question |
|-----|----------|
| D3 | Which restart-unique scheme makes the Acct-Session-Id unique across restarts: a boot counter kept in storage, or a time component? |

### Owner questions raised by the 2026-10-13 design (each states the reading the design proceeds under)

| ID | Question | Design proceeds under | Why it needs the owner |
|----|----------|-----------------------|------------------------|
| Q-1 (D3, replaces the row above) | Restart-unique scheme: (a) a random per-process boot tag from `crypto/rand`, (b) a boot counter persisted in storage, (c) a time component | (a), a 64-bit random boot tag, hex, minted once in `newRADIUSAcct` | Format of an operator-visible billing key. (c) is rejected in design: a gokrazy appliance with no RTC can start two boots at the same pre-NTP wall time. (b) adds a storage write on every start and a new failure mode. (a) is probabilistic (two boots collide with probability 2^-64) |
| Q-2 (D2) | Duplicate key: (A) source IP, source port, Code, Identifier only, the RFC's literal triple; (B) the same plus the Request Authenticator as a tie-breaker, so a same-triple request with a different Authenticator is processed as new and replaces the cache entry | (B) | Changes when a CoA is applied. (A) answers a genuinely new request with a stale cached answer when a busy DAC wraps its 8-bit Identifier on one port within the 5-minute window. (B) is what RFC 5080 Section 2.2.2 prescribes for the Access-Request analog (quoted under Required Reading). RFC 5176 Section 2.3 forbids a legitimate retransmission from changing its Authenticator, so (B) still answers every legitimate retransmission from the cache |
| Q-3 (D2) | Approve changing four RFC-tagged tests whose duplicate copies come from a fresh socket each, so they would stop exercising the cache once the port is in the key (list under TDD Test Plan, "Tagged tests that change") | open: not approved; the design shows the edit the owner would approve, and no tagged test is touched until the owner records the approval | `ai/rules/rule-precedence.md`: changing an RFC-tagged test needs the owner |
| Q-4 (D1) | ARAP-Password: (i) add type 70 to the exclusive credential set by a literal, unexported value in `client.go`, keeping `dict.go` free of any ARAP constant; (ii) refuse any ARAP-family attribute (70-74, 84) in an Access-Request outright, as RFC 2869 Section 1.1 forbids Ze implementing them | (i) | (ii) is stricter than RFC 3579 and makes "ARAP-Password alone encodes" false, which the skeleton's "each single credential type encodes" obligation assumed. (i) recognises the type to refuse a pair, which is not implementing the ARAP service |
| Q-5 (D1) | Also extend the RFC2869-5.19-1 tests (same sentence, verdict `enforced`, `rfc2869_credential_types_test.go`) with the ARAP pairs, or only correct their header comment "ARAP-Password is not driven" | correct the comment only; the ARAP proof lives in the RFC3579-3.3-2 tests | Changing an `enforced` verdict's tagged test needs the owner; the comment becomes false the moment D1 lands (`ai/rules/stale-comments.md`) |
| Q-6 (D3, scope) | `coa.go::findSessions` matches a CoA's Acct-Session-Id by the "tunnelID-sessionID-" prefix and ignores the rest, so a Disconnect-Request carrying a previous session's (or previous boot's) id tears down whichever current session reuses that tunnel and session number. Fix here (exact match against the accountant's recorded id) or route to `plan/immediate/spec-rfc5176-remaining-defect-classes.md` | out of scope here; this spec keeps the prefix so the match is unchanged, and the implementer writes a journal row if the owner routes it elsewhere | Scope change. It is code related to the id format this spec changes, so `ai/rules/rule-precedence.md` would allow fixing it on the spot, but it changes which CoA tears down which session |
| Q-7 (interop) | D2 and D3 interop need FreeRADIUS beside a live L2TP session, which no lab has: the L2TP lab's RADIUS peer is `internal/le/interoplab/l2tp/radiusmock/` (Ze's own Go program) and the FreeRADIUS suite has no L2TP. Add a FreeRADIUS container (radclient + accounting `detail` log) to the L2TP lab, or share the planned `radius-coa-disconnect-freeradius` scenario of `spec-rfc-verdict-fix-access` | add to the L2TP lab, two new named scenarios (Interop Tests table) | Lab work larger than the three producer fixes; the owner may prefer one shared FreeRADIUS-in-L2TP lab change for both specs |

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
- [ ] RFC 5080 Section 2.2.2 (NOT enrolled, RFC scope frozen 2026-10-08; read from the session scratch copy fetched from rfc-editor.org, cited as design support only, never as a requirement)
  → Decision: RFC 5080 Section 2.2.2: "Cache entries MUST also be purged if the server receives a valid Access-Request packet that matches a cached Access-Request packet in source address, source port, RADIUS Identifier, and receiving socket, but where the Request Authenticator field is different from the one in the cached packet." This is the shape of Q-2 option (B).
- [ ] `rfc/short/rfc2866.md` line 134 (RFC2866-5.5-1), `rfc/full/rfc2866.txt` Section 5.5
  → Constraint: RFC 2866 Section 5.5: "This attribute is a unique Accounting ID to make it easy to match start and stop records in a log file." The row is tagged [MUST] although the sentence carries no keyword; the same section's example reserves digits for reboots: "the first two digits increment on each reboot (wrapping every 256 reboots)".
  → Constraint: RFC 2866 Section 5.5: "The start and stop records for a given session MUST have the same Acct-Session-Id." The new id is minted once per session at `onSessionIPAssigned` and stored in `acctSession.acctSessID`; that must not change.

**Key insights:**
- All three verdicts are `weak` at HEAD and the gate is green: the defect is in what the producers do, not in tag resolution.
- D2's port fix breaks four tagged tests by construction, because every test helper (`sendRawCoAPacket`, `coaRawExchange`) dials a fresh UDP socket per send; their "duplicate" copies only matched because the port was ignored.
- D3's id is parsed back by `coa.go::findSessions` ("tunnelID-sessionID-" prefix); the new format keeps that prefix.
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
| A-3 | The only consumer of the Acct-Session-Id format is `findSessions` (prefix) | grep of `acctSessID` / `AttrAcctSessionID` in `authradius`; `.ci` files and docs assert no format | a changed format breaks a consumer | grep at implementation; `test/l2tp/radius-acct-wire.ci` and `radius-coa-listener.ci` still pass | unvalidated |
| A-4 | FreeRADIUS `radclient` can set the request Identifier (`-i`) and send from distinct processes (distinct source ports) | FreeRADIUS v3 `radclient` usage, not yet read from the pinned image | the interop duplicate/non-duplicate halves need a raw sender beside radclient | `radclient -h` inside the pinned image | unvalidated |
| A-5 | Every test that sends the same CoA wire twice is among the four listed under "Tagged tests that change" | grep of `replay\|duplicate` in `authradius/*_test.go` | an unlisted test goes red or turns vacuous | full `authradius` package test after the key change, and a grep for tests calling a send helper twice with one wire | unvalidated |
| A-6 | `crypto/rand.Read` cannot fail on Go 1.27 | go.mod `go 1.27.2`; since Go 1.24 `crypto/rand.Read` does not return an error | a failure would mint a constant tag | Go release notes at implementation | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Adding the port makes the four fresh-socket tests either red (Inside, FromCache, Replay) or vacuous (EndOutside: it would pass because the port differs, not because the window expired) | the tests' teardown counts | rewrite their sends to reuse one socket (Q-3); re-record discrimination for each |
| R-2 | (B) leaves a cache entry keyed on the triple; a new request with a different Authenticator must REPLACE it, or a later retransmission of the first request would be answered with the second's response | a test: request X, request Y same triple, retransmit X | key on the triple, store the Authenticator in the entry, compare on lookup, overwrite on `rememberReplay` |
| R-3 | Two DACs behind one NAT with the same Identifier are now distinct only by port; a NAT that rewrites ports between retransmissions defeats detection | none in tests | inherent in RFC 5176 Section 2.3's triple; documented in `docs/guide/l2tp.md` |
| R-4 | A longer Acct-Session-Id overflows a server column | FreeRADIUS `radacct.acctsessionid` is varchar(64) in its stock schema | 16 hex digits of boot tag keeps the id under 40 characters for 5-digit tunnel and session numbers and a 10-digit counter |
| R-5 | The changed id format confuses `findSessions` | `TestDisconnect*` and CoA match tests | keep "T-S-" first; boot tag and counter follow |
| R-6 | A retired accountant (`detach`) mints no id, but a future change could construct a second `radiusAcct` in one process with a new tag | none | the tag is per accountant; two accountants in one process each have a unique tag, so uniqueness still holds |
| R-7 | The interop lab change (FreeRADIUS in the L2TP lab) is larger than the code fix and needs the PPPoL2TP kernel module | lab preflight refusal | Q-7; the scenario names are fixed here so the lab work can be shared |

## Blast Radius
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | D1: an Access-Request refused (none Ze builds today). D2: a CoA applied twice or answered from a stale cache. D3: billing records that do not join, or a CoA that matches no session |
| How is it reverted? | single commit revert per defect; no config or storage migration under Q-1 (a) |
| Who else touches this path? | `spec-rfc5176-remaining-defect-classes` (coa.go matching and atomicity), `spec-lifecycle-invariants` (acct.go D-6, RFC2866-5.5-1), `spec-rfc-verdict-fix-access` (verdicts, planned `radius-coa-disconnect-freeradius`) |

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
| AC-1d | `dict.go` after the fix | declares no `Attr*` constant valued 70; `TestRFC2869DictionaryDeclaresNoAttributeForAnUnofferedService` still passes unchanged |
| AC-2a | a Disconnect-Request sent twice, byte-identical, from ONE client socket (same IP, port, Identifier, Authenticator) | both answered with identical octets; the session is torn down once (teardowns == 1) |
| AC-2b | the same wire sent from two different client sockets (same IP and Identifier, different source port) | not a duplicate: the second is processed afresh (teardowns == 2 against a service whose session survives teardown, or its own fresh answer) |
| AC-2c | under Q-2 (B): request X, then request Y from the same socket with the same Identifier but a different Authenticator, then X's wire again | Y is processed afresh (not answered with X's cached response); X's retransmission after Y is processed afresh too, because Y replaced the entry |
| AC-2d | `TestRFC5176DuplicateAndTimestampWindowsBothHoldInside` / `EndOutside` rewritten to send from one socket | inside the window the duplicate is answered from the cache; past it, processed again; the negative goes red if `pruneReplayLocked` is disabled (not merely because the port differs) |
| AC-3a | two accountants constructed in turn (a simulated process restart), each starting a session with the same tunnel and session ids | the two Accounting-Start records carry different Acct-Session-Id values |
| AC-3b | one accountant, one session, Start, Interim-Update and Stop | all three carry the same Acct-Session-Id, which begins "<tunnel>-<session>-" |
| AC-3c | a Disconnect-Request carrying a current session's Acct-Session-Id | `findSessions` still matches it (existing CoA tests stay green with ids from the new format) |
| AC-5 | every tagged unit this spec adds or whose claim it changes | carries a discrimination record from `./le rfc discriminate-record`, observed red against a revert of its producer |

## End-to-End User Stories
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | an operator's RADIUS server retransmits a Disconnect-Request after a lost ACK | UDP 3799 → `serve` → `handlePacket` → `cachedReplay` → `sendRawResponse` | `TestRFC5176DuplicateIsSameSourcePortAndIdentifier`; interop `l2tp-radius-coa-duplicate-freeradius` |
| 2 | two DACs behind one NAT address send Disconnect-Requests with the same Identifier | UDP 3799 → `handlePacket` → miss in `cachedReplay` → `handleDisconnect` | `TestRFC5176DifferentSourcePortIsNotADuplicate`; interop `l2tp-radius-coa-duplicate-freeradius` |
| 3 | the operator restarts ze; subscribers reconnect on the same tunnel and session numbers; billing joins Start and Stop records in the server log | `SessionIPAssigned` → `onSessionIPAssigned` → `genSessionID` → `buildAcctPacket` → server log | `TestRFC2866AcctSessionIDUniqueAcrossRestart`; interop `l2tp-radius-acct-restart-freeradius` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestAccessRequestRefusesTwoCredentialTypes` (extended: all six pairs, ARAP via literal 70) | `internal/component/radius/authenticator_eap_test.go` | AC-1a; RFC3579-3.3-2 negative. Red at HEAD for the three ARAP pairs and passes for UP+CHAP (already refused) | |
| `TestRFC3579EachCredentialTypeAloneEncodes` (new) | `internal/component/radius/authenticator_eap_test.go` | AC-1b; RFC3579-3.3-2 positive. Green at HEAD (control for the negative) | |
| `TestRFC3579ARAPBesideEAPIsRefusedAtExchange` (new) | `internal/component/radius/rfc2869_credential_types_test.go` or a new `rfc3579_credential_types_test.go` | AC-1c; RFC3579-3.3-2 negative at the public entry point. Red at HEAD | |
| `TestRFC5176DuplicateIsSameSourcePortAndIdentifier` (new) | `internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go` | AC-2a; RFC5176-2.3-2 positive over one socket | |
| `TestRFC5176DifferentSourcePortIsNotADuplicate` (new) | same file | AC-2b; RFC5176-2.3-2 negative. Red at HEAD (HEAD answers the second port from the cache) | |
| `TestRFC5176SameTripleNewAuthenticatorIsProcessedAfresh` (new) | same file | AC-2c (Q-2 B), including the replace-then-retransmit order of R-2 | |
| `TestRFC2866AcctSessionIDUniqueAcrossRestart` (new) | `internal/component/l2tp/plugins/authradius/rfc2866_accounting_test.go` | AC-3a; RFC2866-5.5-1 positive. Red at HEAD (both ids "1-2-1") | |
| `TestRFC2866AcctSessionIDKeepsTunnelSessionPrefix` (new) | same file | AC-3b, AC-3c; preserves the `findSessions` contract | |
| `TestReplayKeyCarriesSourcePort` (new, untagged) | `internal/component/l2tp/plugins/authradius/coa_test.go` or beside the walk tests | the key differs for two `*net.UDPAddr` differing only in port | |

### Tagged tests that change (owner approval needed, Q-3 and Q-5)
| Test | File | Requirement (verdict) | Change | Why |
|------|------|-----------------------|--------|-----|
| `TestRFC5176DuplicateRequestAnsweredFromCache` | `authradius/rfc5176_walk_test.go` | RFC5176-2.3-2 (`weak`) | the two duplicate copies go out on one socket; comment's "same source" made precise | today the copies come from two sockets; with the port in the key the second is no longer a duplicate and teardowns becomes 2 |
| `TestRFC5176DuplicateAndTimestampWindowsBothHoldInside` | `authradius/rfc5176_section_rows_test.go` | RFC5176-6.3-2 (`enforced`) | `coaRawExchange` pair replaced by two sends on one socket | goes red otherwise (second copy processed) |
| `TestRFC5176DuplicateAndTimestampWindowsBothEndOutside` | `authradius/rfc5176_section_rows_test.go` | RFC5176-6.3-2 (`enforced`) | same | stays green but VACUOUS otherwise: the second copy would be processed because the port differs, not because the window expired |
| `TestDisconnectReplayReturnsCachedResponse` | `authradius/rfc5176_coa_listener_test.go` | RFC5176-3.3-1 (`enforced`) | the replay goes out on the first socket | goes red otherwise |
| `rfc2869_credential_types_test.go` header comment only (Q-5 reading) | `radius/rfc2869_credential_types_test.go` | RFC2869-5.19-1 (`enforced`) | "ARAP-Password is not driven" corrected to name where the ARAP pairs are proven | becomes false when D1 lands; no assertion change under the Q-5 reading |
| `encodeRequest` caller comment (code, not a test) | `radius/client.go` | - | "ARAP-Password is not named" replaced | stale once 70 is in the set |

`TestRFC2866AcctSessionIDUnique` and `TestRFC2866AcctSessionIDNoCollisionOnReusedKey` do not change: both stay true with the boot tag.
Every changed tagged unit stales its `rfc/discrimination/<stem>.json` record (rfc5176: 5 units, rfc3579: 1 unit, per a grep on 2026-10-13); each is re-recorded by `./le rfc discriminate-record`.

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
| `l2tp-radius-coa-duplicate-freeradius` (new, named, no numeric prefix) | `test/interop-l2tp/scenarios/` | FreeRADIUS `radclient` as DAC, xl2tpd + pppd as LAC (live session) | (1) two `radclient` processes send a Disconnect-Request with the same Identifier from distinct ports: each is processed (Ze log shows two dispatches); (2) a lost first Disconnect-ACK (dropped by an `iptables` rule in the radclient container) makes `radclient` retransmit on its own socket: Ze answers from the cache and tears the session down once (Ze log, pppd log). Forced red: revert `replayKey`, rebuild ze, (1) fails | |
| `l2tp-radius-acct-restart-freeradius` (new, named) | `test/interop-l2tp/scenarios/` | FreeRADIUS accounting with the `detail` module; xl2tpd + pppd | the session's Start before a ze restart and the reconnected session's Start after it carry different Acct-Session-Id values in FreeRADIUS's own detail log, and each Stop carries its Start's id. Forced red: revert `genSessionID`, rebuild ze, the two ids are equal | |
| D1 | N-A | - | no wire-visible change: Ze builds no ARAP-Password, and the refusal happens before the socket; the existing `radius-admin-*-freeradius` scenarios must stay green (no credential combination changed) | |

### Goal validation
| Goal | Evidence owed |
|------|---------------|
| D1: no Access-Request leaves Ze with two credential types | AC-1a/1c tests, with a discrimination record observed red against a revert of the type-70 entry |
| D2: the DAS detects duplicates on source IP, source port and Identifier | `l2tp-radius-coa-duplicate-freeradius` passing with its forced red, plus AC-2a/2b |
| D3: Acct-Session-Ids do not repeat across a restart in the server's log | `l2tp-radius-acct-restart-freeradius` passing with its forced red, plus AC-3a |
| AC-4 | each of the three verdicts stamped `enforced` by an independent `mode rejudge` |

## Files to Modify

- `internal/component/radius/client.go` - add ARAP-Password (70, unexported literal) to `oneCredentialType`; replace the stale caller comment; RFC 3579 Section 3.3 quote stays above the check
- `internal/component/l2tp/plugins/authradius/coa.go` - `coaReplayKey` gains the source port (and under Q-2 B, the Authenticator moves from the key to the entry); `cachedReplay`, `rememberReplay` and `replayKey` take `*net.UDPAddr`; the callers are `handlePacket` (`cachedReplay(from.IP, pkt)`) and `sendResponse` line 828 (`rememberReplay(to.IP, req, wire)`), which pass the whole address; the `findSessions` comment updated to the new format
- `internal/component/l2tp/plugins/authradius/acct.go` - `radiusAcct` gains a boot tag minted in `newRADIUSAcct`; `genSessionID` writes "tunnel-session-tag-counter"; RFC 2866 Section 5.5 quote above it
- the tests listed under "Unit Tests" and "Tagged tests that change"
- `rfc/discrimination/rfc3579.json`, `rfc/discrimination/rfc5176.json`, `rfc/discrimination/rfc2866.json` (re-recorded by the tool, never by hand); `rfc/audit/rfc3579.json`, `rfc5176.json`, `rfc2866.json` (stamped by an independent judge)
- `docs/guide/l2tp.md` - one sentence each on the duplicate key and on restart-unique Acct-Session-Id
- `docs/architecture/testing/interop.md` - the two new L2TP scenarios and the FreeRADIUS peer in the L2TP lab
- `internal/le/interoplab/l2tp/` - FreeRADIUS container beside `radiusmock` (scope per Q-7)

## Files to Create
- `test/interop-l2tp/scenarios/l2tp-radius-coa-duplicate-freeradius/`
- `test/interop-l2tp/scenarios/l2tp-radius-acct-restart-freeradius/`

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
| Doctor check for runtime dependencies | N-A | Q-1 (a) adds no file, socket or storage; Q-1 (b) would add a storage dependency and a doctor check |
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
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/interop.md` (FreeRADIUS in the L2TP lab) |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` cites acct.go for features, not the id format; confirm with the anchors command |
| 12 | Internal architecture changed? | No | - |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | - |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `./le spec citation anchors spec plan/immediate/spec-radius-rfc-defects.md` at implementation; known from `ai/CODE-TO-DOCS.md`: acct.go → `bng-1-radius-attributes.md`, `subscriber-session-model.md`, `comparison.md`, `guide/l2tp.md`; coa.go → `guide/l2tp.md`; client.go → `bng-1-radius-attributes.md`; `// Design:` of all three is `docs/research/l2tpv2-ze-integration.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | No | no example shows an Acct-Session-Id value |

## Implementation Steps

1. **Phase: Wiring** -- write the failing wiring tests: `TestRFC5176DifferentSourcePortIsNotADuplicate`, `TestRFC3579ARAPBesideEAPIsRefusedAtExchange`, `TestRFC2866AcctSessionIDUniqueAcrossRestart`; confirm each is red at HEAD for the reason named in the TDD table.
2. **Phase: D1** -- extend `TestAccessRequestRefusesTwoCredentialTypes` to six pairs and add the single-type positive; add 70 to `oneCredentialType`; fix the two stale comments; `TestRFC2869Dictionary*` unchanged and green.
3. **Phase: D2** -- with Q-3 approved, rewrite the four tagged tests to send from one socket (they then pass at HEAD, which is correct: HEAD detects same-port duplicates); add the AC-2 tests; change the key; confirm EndOutside goes red with pruning disabled.
4. **Phase: D3** -- boot tag per Q-1; add the AC-3 tests; confirm CoA tests stay green.
5. **Phase: Discrimination and verdicts** -- `./le rfc discriminate-record` for every new or changed tagged unit; an independent agent stamps the three verdicts with `mode rejudge`.
6. **Phase: Interop** -- per Q-7; each scenario's forced red recorded.
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
| Input validation | the replay map is filled only by authenticated packets from allowed sources; the port adds at most 65535 keys per source within the window, bounded by `pruneReplayLocked` |
| Resource exhaustion | an allowed DAC cycling ports grows the map; same bound as today's Authenticator-keyed map |
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
| D3: random boot tag (Q-1 a) | storage boot counter (b); start time (c) | (c) is unsafe on an RTC-less appliance; (b) adds storage I/O and a doctor check; (a) needs neither |

## Known Limitations
- `findSessions` prefix matching across sessions that reuse tunnel and session numbers (Q-6): not fixed here unless the owner scopes it in.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code: RFC 3579 Section 3.3 Note 1 above `oneCredentialType`'s set, RFC 5176 Section 2.3 above `replayKey`, RFC 2866 Section 5.5 above `genSessionID`.

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (before the fix)
- [ ] Tests PASS (after the fix)

### Verification
- [ ] `./le verify worktree`
