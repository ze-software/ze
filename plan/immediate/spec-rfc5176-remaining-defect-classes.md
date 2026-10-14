# Spec: RFC 5176 residual CoA atomicity and session matching

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Depends | `plan/immediate/spec-lifecycle-invariants.md` (applied-result contract, AC-3/AC-4; gates AC-8 only); `plan/immediate/spec-radius-rfc-defects.md` (D2 replay key and D3 Acct-Session-Id format touch the same file; land order in R-6) |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-14 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`rfc/extraction/rfc5176.json` records sign-off on 2026-09-01. That source-walk
milestone is delivered; it does not prove that every CoA request is applied
atomically or that every subscriber lookup honours the request's full selector.
This spec owns those residual conformance obligations, with the applied-result
contract shared with the lifecycle spec as described below.

The rfc5176 walk of `spec-rfcgate-6` recorded nine defect classes on
2026-08-31: Service-Type never read; attributes not treated as mandatory; only
the first matching session acted on; Proxy-State and State not echoed; a stale
Event-Timestamp NAK'd where Section 6.3 says silently discard; CoA state changes
not atomic; Termination-Action State echo absent.

**Four were verified DEAD at the producer on 2026-09-03**, in
`internal/component/l2tp/plugins/authradius/coa.go`. The row's original path,
`internal/component/radius/coa.go`, is wrong and no file is there.

1. Service-Type IS read: the CoA path tests `pkt.FindAttr(radius.AttrServiceType)` and quotes Section 3.2's Authorize-Only rule above the branch.
2. Attributes ARE treated as mandatory: two allow-listed attribute sets gate what a CoA-Request and a Disconnect-Request may carry.
3. Proxy-State and State ARE echoed: `(*coaListener).sendResponse` copies every attribute whose type is `AttrProxyState` or `AttrState`, quoting Sections 3.1 and 3.3.
4. A stale Event-Timestamp is SILENTLY DISCARDED: the stale arm logs and returns with no answer, quoting Section 6.3, and its comment gives the reason a NAK would be wrong, that it tells a replaying sender the secret is right. An absent Event-Timestamp draws `ErrorCauseInvalidRequest`.

Current ownership, checked against `handleCoA`, `applySubscriberCoA`,
`findSubscriberSession` and `oneSession` in
`internal/component/l2tp/plugins/authradius/coa.go`:

- This spec owns atomic multi-change application. `applySubscriberCoA` emits
  subscriber rate, L2TP rate and CoS changes in sequence and NAKs the first
  failed emit. Earlier changes have no rollback here. Completion owes the
  all-or-nothing result, including downstream application failures.
- `plan/immediate/spec-lifecycle-invariants.md` AC-3/AC-4 owns the applied-result
  contract and PPPoE delivery. A successful emit still does not prove a change
  was applied: `shaper.onSessionRateChange` returns without a result after an
  unknown session or an `applyTC` failure. This spec consumes that contract for
  its atomic transaction; neither spec may count the other's unfinished half
  as complete.
- This spec owns subscriber selector correctness and multi-match refusal.
  `findSubscriberSession` reads only Acct-Session-Id and returns one registry
  lookup before `oneSession` can inspect the L2TP match set. The residual is
  the missing conjunction with other supplied identifiers and the complete
  matching-set decision. One returned record alone proves neither ambiguity
  nor correctness. Preserve the existing 508 response when a request matches
  multiple sessions and Ze does not support a multi-session change.
- Termination-Action is a retained optional-feature question. The signed
  extraction excludes its State obligation as `feature-out-of-scope` because
  RFC 2865 Section 5.29 makes that reauthentication feature optional. Absence
  of that feature is not an unsigned extraction or an unconditional CoA bug.
  The original AC-3 obligation remains conditional on selecting that feature;
  Thomas must resolve that scope question before this spec closes.

`plan/pre-release/spec-rfcgate-6-supported-extraction-signoff.md` is the parent
evidence programme. Its recorded signature must not be repeated as a future
deliverable or used to close these residual behaviours.

### Defect-class status at HEAD (re-checked 2026-10-13 against `3e2770062b`)

The commit cited is the earliest at this path carrying the fixing text (`git log -S`); the code may predate a file move.

| Class (2026-08-31 walk) | At HEAD | Evidence |
|-------------------------|---------|----------|
| Service-Type never read | fixed | `fe51839da0` (2026-09-01); `handlePacket` NAKs every Service-Type in a CoA-Request with 405, Section 3.2 quoted |
| Attributes not treated as mandatory | fixed | `fe51839da0` (allow-lists `coaSupportedAttrs`, `disconnectSupportedAttrs`); `0cc6cb1b22` (2026-09-29) added value refusals: `readCoAChange` 407 for any unusable Filter-Id/VSA beside a usable one, `unsupportedAttrValue` 407 for a wrong fixed length |
| Only the first matching session acted on | fixed on the L2TP path, OPEN on the subscriber path | `fe51839da0`: `oneSession` NAKs 508 for several matches and `findSessions` ANDs Acct-Session-Id, User-Name and NAS-Port. `findSubscriberSession` still reads only Acct-Session-Id and returns one record (S-2) |
| Proxy-State and State not echoed | fixed | `fe51839da0`; `sendResponse` copies both, Sections 3.1 and 3.3 quoted |
| Stale Event-Timestamp NAK'd | fixed | `fe51839da0`; silent discard, Section 6.3 quoted |
| CoA state changes not atomic | partly fixed, OPEN | `0cc6cb1b22`: every refusal decided before the first emit; audit verdict RFC5176-2.3-6 `enforced` on 2026-09-29 judged emit-level atomicity. Open: an unknown CoS profile is ACKed and dropped downstream (T-1); no rollback when a later part fails to APPLY (T-2) |
| Termination-Action State echo absent | excluded, owner decision pending | `rfc/extraction/rfc5176.json` site 3.3:7 `feature-out-of-scope` (RFC 2865 Section 5.29 MAY); Q-6 |

### Defects this spec owns (all verified at the producer on 2026-10-13)

| ID | Producer | RFC text (read in `rfc/full/rfc5176.txt`) | What Ze does wrong |
|----|----------|--------------------------------------------|--------------------|
| S-1 | `coa.go::findSessions` | Section 3: "The combination of NAS and session identification attributes included in a CoA-Request or Disconnect-Request packet MUST match at least one session in order for a Request to be successful; otherwise a Disconnect-NAK or CoA-NAK MUST be sent." Section 3 table: Acct-Session-Id is "The identifier uniquely identifying a session on the NAS." | The Acct-Session-Id is matched on its "tunnelID-sessionID-" prefix only. A late Disconnect-Request carrying the id `acct.go::genSessionID` issued to a session that has ended tears down the current session that reuses the same tunnel and session numbers. Raised as Q-6 of `spec-radius-rfc-defects` (2026-10-13), which keeps the prefix and routes the fix here |
| S-2 | `coa.go::findSubscriberSession`, `handleCoA`, `handleDisconnect` | Section 3, first sentence above; Section 2.3: "If the CoA-Request is unsuccessful for any matching sessions, the NAS MUST send a CoA-NAK in reply, and the requested authorization changes MUST NOT be made for any of the matching sessions." | The registry is searched by `Session.AcctSessionID`, which for L2TP holds `l2tp-<T>-<S>` (`subscriber_bridge.go` lines 72 and 147) and for PPPoE `pppoe` ids (`pppoe/subsystem.go` lines 397, 445): values no RADIUS packet carries. A hit acts with no other identifier checked. A real DAC's id (`T-S-N`) misses the registry and falls to the L2TP-only path, which NAKs every CoS change 506 "no access interface" although the registry session has one, so CoS-by-CoA is unreachable for an L2TP session from a real DAC |
| S-3 | `coaSupportedAttrs`, `disconnectSupportedAttrs`, `findSessions` | Section 3, first sentence above; Section 2.3: "In CoA-Request and Disconnect-Request packets, all attributes MUST be treated as mandatory." | Eleven identification attributes are accepted and never evaluated: Framed-IP-Address, Called-Station-Id, Calling-Station-Id, Acct-Multi-Session-Id, NAS-Port-Id, Chargeable-User-Identity, Framed-Interface-Id, Framed-IPv6-Prefix, NAS-IP-Address, NAS-Identifier, NAS-IPv6-Address. A request whose combination names no session still acts on the session its Acct-Session-Id or User-Name names |
| T-1 | `readCoAChange` / `addFilterID` / `matchVendorCoS`; consumer `internal/plugins/cos/handler.go::onCoSChange` | Section 2.3: "If one or more authorization changes specified in a CoA-Request cannot be carried out, the NAS MUST send a CoA-NAK." Section 2.2: "The NAS responds to a CoA-Request sent by a Dynamic Authorization Client with a CoA-ACK if the NAS is able to successfully change the authorizations for the user session(s), or a CoA-NAK if the CoA- Request is unsuccessful." | A CoS profile name is parsed (`coreCos.ParseFilterID`) and never looked up, so `cos:no-such-profile` is ACKed; `onCoSChange` then logs "CoA profile not found" and returns. `onCoSChange` also stores the new `profileName` before `updateQoS` succeeds |
| T-2 | `applySubscriberCoA` | Section 2.3: "State changes resulting from a CoA-Request MUST be atomic: if the CoA-Request is successful for all matching sessions, the NAS MUST send a CoA-ACK in reply, and all requested authorization changes MUST be made." | Rate and CoS events are fire-and-forget (`events.Event.Emit`, handlers are `func(T)` with no result). When the rate applies and the CoS `updateQoS` fails, the rate stays changed and the DAC has already been ACKed. Needs the applied-result contract of `spec-lifecycle-invariants` AC-3/AC-4 |

### Owner questions (each states the reading the design proceeds under; Q-5 is answered by the owner on 2026-10-13, Q-1 and Q-7 on 2026-10-14 through `spec-radius-rfc-defects` Q-6 and Q-7)

| ID | Question | Design proceeds under | Why it needs the owner |
|----|----------|-----------------------|------------------------|
| Q-1 (S-1 scope) | Take the stale-Acct-Session-Id defect (`spec-radius-rfc-defects` Q-6) into this spec? | **Answered by the owner, 2026-10-14, through `spec-radius-rfc-defects` Q-6: "whatever"; the recommendation is taken: yes.** The stale Acct-Session-Id prefix-match defect is in this spec: AC-1 here; `spec-radius-rfc-defects` keeps D2/D3 and its prefix-preserving R-5, and this spec's exact match does not depend on the id format D3 chooses | Scope move between two specs; it changes which Disconnect tears down which session |
| Q-2 (S-1/S-2 design) | One session table: (B) the subscriber registry is the only table the CoA matcher reads, and `Session.AcctSessionID` is redefined as the Acct-Session-Id Ze put on the wire (written by the accountant when it issues one, empty when none was issued); or (A) an authradius-owned table of issued ids beside the L2TP snapshot, with the registry consulted only for the access interface | (B) | (B) changes the meaning of a registry field and deletes the L2TP-only fallback (`ai/rules/no-layering.md`); (A) leaves a field holding a value no packet carries and two lookups that disagree (S-2) |
| Q-3 (S-3) | An identification attribute Ze holds no fact to compare (Called-Station-Id, Acct-Multi-Session-Id, Chargeable-User-Identity, NAS-IPv6-Address): (i) drop it from the accepted sets, so the request is NAKed 401; (ii) keep it accepted and let it match nothing, NAK 503; (iii) keep ignoring it | (i) | Changes which requests a deployed DAC gets answered. Ze sends none of the four in any Access-Request or Accounting-Request, so a DAC that copies accounting attributes never carries them; (iii) is the current RFC 5176 Section 3 breach |
| Q-4 (T-2) | Atomic multi-change: (alpha) apply in order and, when a later part reports failure, re-apply the prior state of the parts already applied, then NAK 506; (beta) merge rate and CoS into one desired-state `traffic` backend Apply per interface | (alpha), started only after `spec-lifecycle-invariants` AC-4 gives a synchronous applied result | (beta) merges the shaper and CoS plugins' state; (alpha) can itself fail, and AC-8 then reports the breach rather than hiding it |
| Q-5 (tests) | Approve changing the RFC-tagged tests in "Tagged tests that change". Revised 2026-10-13 after the RFC-tagged test review: `TestRFC5176IdentificationAttributesIdentifyOnly` (RFC5176-3.6-1, both units now drop Called-Station-Id), `TestRFC5176DisconnectCarryingOnlyIdentificationIsACKed` (RFC5176-3-1 negative, prose narrows; its claim turns on Q-3), `TestRFC5176RequestNotRoutableCauseNeverSentForAForeignSession` (RFC5176-3.5-8, now a body change: matching NAS-Identifier, since AC-3 is now 403 per Section 3.5 "NAS Identification Mismatch"), the 2.3-6 premise changes, the 2.3-5/2.3-6 re-records, and the fixture-only units. `TestDisconnectReplayReturnsCachedResponse` is also edited by `spec-radius-rfc-defects` Q-3 | **Answered by the owner, 2026-10-13: "approve all RFC-tagged test changes".** It covers every unit in "Tagged tests that change", the behaviour edits and the fixture-only units alike. Q-2 and Q-3 are still open and this answers neither: the fixture-only edits apply under Q-2 (B), the Called-Station-Id edits and the `TestRFC5176DisconnectCarryingOnlyIdentificationIsACKed` claim are written for whichever Q-3 answer is recorded, and `TestRFC5176RequestNotRoutableCauseNeverSentForAForeignSession` takes either form its row offers. Commands under "Owner approval commands" in the TDD plan | `ai/rules/rule-precedence.md`: changing an RFC-tagged test needs the owner |
| Q-6 (AC-9) | Termination-Action (RFC 2865 Section 5.29, optional): select the reauthentication feature, or keep the recorded `feature-out-of-scope` exclusion | keep the exclusion; nothing implemented | Optional-feature scope (`ai/rules/rfc-compliance.md`); the skeleton already requires Thomas to decide before closure |
| Q-7 (interop) | Interop needs FreeRADIUS `radclient` beside a live L2TP session, which no lab has. Where does FreeRADIUS run, shared with `spec-radius-rfc-defects` Q-7 and the planned `radius-coa-disconnect-freeradius` of `spec-rfc-verdict-fix-access`? | **Answered by the owner, 2026-10-14, through `spec-radius-rfc-defects` Q-7: "Make Free radius, its own lab."** FreeRADIUS gets its own lab, `interop-radius-subscriber` (scenarios under `test/interop-radius-subscriber/scenarios/`, plumbing in `internal/le/interoplab/radiussubscriber/`), shared with `spec-radius-rfc-defects`, which defines it under "The `interop-radius-subscriber` lab"; whichever spec lands first builds it. No FreeRADIUS container is added to the L2TP lab, whose RADIUS peer stays `radiusmock`. The two scenarios named here go into that lab | Lab work larger than the producer fixes; one lab for three specs is an ordering decision |
| Q-8 (PPPoE) | PPPoE sessions have no Ze-issued Acct-Session-Id (the only issuer, `acct.go`, subscribes to `l2tpevents` only). Should the matcher select PPPoE sessions by the other identifiers (User-Name, Framed-IP-Address) | yes, the same evaluator runs over PPPoE registry entries; a PPPoE Disconnect still answers 504 and PPPoE CoA delivery stays with `spec-lifecycle-invariants` AC-3 and I-4 | Today a PPPoE session is reachable only by its internal id, which no DAC can know; opening it by User-Name changes which requests act |

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/rfc-conformance-gates.md` - the extraction sign-off, the tagged-test routes and the discrimination record
  → Constraint: every tagged unit whose body or producer changes stales its `rfc/discrimination/rfc5176.json` record; re-record with `./le rfc discriminate-record`, never by hand. A row quote stays a verbatim span of its section.
  → Constraint: the 2026-09-01 extraction sign-off is not re-signed by this spec unless a site's disposition changes; no new requirement rows (RFC scope frozen 2026-10-08), so S-1..S-3 are proven under the existing RFC5176-3.3-1, RFC5176-2.3-5, RFC5176-2.3-7 rows and T-1/T-2 under RFC5176-2.3-5 and RFC5176-2.3-6.
- [ ] `docs/guide/l2tp.md` lines 609-676 - the operator page for the CoA/DM listener
  → Constraint: the check table row "The identification attributes match exactly one session" over-claims today (three of fourteen accepted identification attributes are evaluated); it is corrected in the same phase as S-3, not at closure (`ai/rules/documentation.md`).
  → Constraint: the row "The requested change reached the shaper | NAK 506" describes emit, not application; AC-8 changes it to "was applied".
- [ ] `docs/research/l2tpv2-ze-integration.md` - the `// Design:` document of `coa.go` and `acct.go`
  → Decision: lines 395-396 and 438 set CoA rate changes and DM teardown for live sessions as the integration goal; nothing there fixes how a session is selected, so the selector design here adds no conflict.
- [ ] `docs/architecture/testing/interop.md` - suites, the FreeRADIUS admin-login lab, scenario naming
  → Constraint: scenario directories are named, never numbered; the FreeRADIUS suite (`internal/le/interoplab/radius/`) has no L2TP, and the L2TP lab's RADIUS peer is Ze's own `radiusmock`, so a FreeRADIUS-against-live-session scenario needs a new lab: `interop-radius-subscriber` (Q-7, owner 2026-10-14).
  → Constraint: every checker reads both sides (Ze log and the server's record), and a forced red is recorded by reverting the producer and rebuilding the image.
- [ ] `docs/architecture/l2tp/subscriber-session-model.md` - the registry's page; its source anchor names `LookupByAcctSessionID`
  → Constraint: Q-2 B changes what `Session.AcctSessionID` means and removes `LookupByAcctSessionID`; this page and its anchor change in the phase that changes the field (Documentation row 12).
- [ ] `docs/architecture/l2tp/bng-5-pppoe.md` - the PPPoE subsystem page, the `// Design:` of `pppoe/subsystem.go`
  → Constraint: it says nothing about Acct-Session-Id today (grep 2026-10-13), so the PPPoE change (stop writing the internal id) owes no edit there unless the implementer adds a sentence; Q-8 decides PPPoE selection.
- [ ] `docs/architecture/traffic/cos-dynamic.md` - the CoS-by-CoA design page, `// Design:` of `internal/plugins/cos/handler.go`
  → Constraint: line 55-56 names `extractRate` and `extractCoSProfile`, which `0cc6cb1b22` replaced with `readCoAChange`; the page is already wrong and is corrected in the T-1 phase, together with the profile-lookup refusal.
  → Decision: line 57-59: `AccessInterface` is empty for a pure L2TP LNS, so AC-5 holds only for an L2TP session that carries an access interface; a pure-LNS CoS CoA stays NAK 506.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc5176.md` - the declared requirement ids for this stem
  → Constraint: rows used: RFC5176-3.3-1 (line 169, Section 3 "MUST match at least one session"), RFC5176-2.3-5 (line 176, attributes mandatory, NAK when a change cannot be carried out), RFC5176-2.3-6 (line 177, atomic), RFC5176-2.3-7 (line 178, 508), RFC5176-3.6-1 (line 187, identification only). All five are `enforced` in `rfc/audit/rfc5176.json` at HEAD; a fix that changes their producers owes a re-judge by an agent that did not write the tests.
- [ ] `rfc/full/rfc5176.txt` - Sections 2.2, 2.3, 3, 3.5, read at the source text
  → Constraint: Section 2.3, "State changes resulting from a CoA-Request MUST be atomic: if the CoA-Request is successful for all matching sessions, the NAS MUST send a CoA-ACK in reply, and all requested authorization changes MUST be made."
  → Constraint: Section 3, "The combination of NAS and session identification attributes included in a CoA-Request or Disconnect-Request packet MUST match at least one session in order for a Request to be successful; otherwise a Disconnect-NAK or CoA-NAK MUST be sent." A combination, so every supplied identification attribute narrows the set.
  → Constraint: Section 3, "If all NAS identification attributes match, and more than one session matches all of the session identification attributes, then a CoA-Request or Disconnect- Request MUST apply to all matching sessions." The NAS identification attributes are part of the match, not decoration.
  → Constraint: Section 2.3, "A NAS that does not support dynamic authorization changes applying to multiple sessions MUST send a CoA-NAK or Disconnect-NAK in reply; an Error-Cause Attribute with value 508 (Multiple Session Selection Unsupported) SHOULD be included." Preserved.
  → Constraint: Section 3.5, ""Session Context Not Found" is a fatal error sent if the session context identified in the CoA-Request or Disconnect-Request does not exist on the NAS." 503 is the answer for a stale id and a mismatched combination. A foreign NAS identity is 403: ""NAS Identification Mismatch" is a fatal error sent if one or more NAS identification attributes (see Section 3) do not match the identity of the NAS receiving the Request." (revised 2026-10-13, AC-3).
  → Constraint: Section 3.5, ""Unsupported Attribute" is a fatal error sent if a Request contains an attribute (such as a Vendor-Specific or EAP-Message Attribute) that is not supported." 401 under Q-3 (i).
  → Constraint: Section 3.5, ""Invalid Attribute Value" is a fatal error sent if a CoA-Request or Disconnect-Request contains an attribute with an unsupported value." 407 for an unknown CoS profile (T-1).

**Key insights:** (minimal context to resume after compaction)
- Five of seven 2026-08-31 classes are fixed (`fe51839da0`, `0cc6cb1b22`); open are subscriber-path selection (S-2), unevaluated identification attributes (S-3), the stale-id prefix match (S-1, from `spec-radius-rfc-defects` Q-6), the unknown CoS profile ACK (T-1) and downstream rollback (T-2, gated on lifecycle AC-4).
- The registry's `AcctSessionID` never holds an id RADIUS saw: L2TP stores `l2tp-T-S`, the accountant sends `T-S-N`. That one field is why the subscriber path and the L2TP path disagree.
- Every existing CoA unit test sends `10-20-1` against a fake L2TP service; any exact-match design changes those fixtures, which is the bulk of Q-5.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/l2tp/plugins/authradius/coa.go` - sequential emissions, subscriber Acct-Session-Id lookup before the L2TP match-set check, and the existing State/Proxy-State response echo.
- [ ] `internal/component/l2tp/plugins/shaper/shaper.go` - `onSessionRateChange` has no application-result return to the CoA caller.
- [ ] `rfc/extraction/rfc5176.json` - signed 2026-09-01; its optional Termination-Action exclusion is distinct from the remaining atomicity and selector proofs.

**Behavior to preserve:** (unless the user explicitly said to change it)
- the four classes already verified dead, each with its RFC sentence quoted in the code beside it
- the Section 3.2 decision taken in `fe51839da0`: Ze does not offer Authorize-Only, and every Service-Type in a CoA-Request earns `CodeCoANAK` with `ErrorCauseUnsupportedService`

**Digests (read 2026-10-13):**

| File | Lines | Digest |
|------|-------|--------|
| `internal/component/l2tp/plugins/authradius/coa.go` | 916 | `handlePacket` (107): source allow-list, Request Authenticator, decode, code, Message-Authenticator, `cachedReplay`, Event-Timestamp, Service-Type 405, `unsupportedAttr` 401, `unsupportedAttrValue` 407, dispatch. `handleCoA` (383): `readCoAChange` first, then `findSubscriberSession` hit → `applySubscriberCoA`, else `oneSession` → L2TP-only rate emit (CoS refused 506). `applySubscriberCoA` (468): refusals before first emit, then subscriber rate, L2TP rate, CoS emits, ACK. `handleDisconnect` (526): registry PPPoE hit → 504; else `oneSession` → `TeardownSession`, 504 on failure. `findSubscriberSession` (576): `Registry.LookupByAcctSessionID` only. `oneSession` (599): 503 none, 508 several. `findSessions` (629): L2TP snapshot, AND of Acct-Session-Id prefix, User-Name, NAS-Port (= LocalSID). `readCoAChange` (691): one rate, one CoS profile, 407 otherwise, 401 for no change |
| `internal/component/l2tp/plugins/authradius/acct.go` | ~600 | `genSessionID` (235) `T-S-N`, N from `nextSess`; `onSessionIPAssigned` (259) issues once per session when `accepting` and a client exist, stores `acctSession{acctSessID, callingStationID, nasPortID, username, ...}` in `a.sessions`; `onSessionDown` deletes it; `detach` (558) moves every live record to a retired accountant on reload, so the active accountant no longer knows ids issued before a reload |
| `internal/component/l2tp/subscriber/session.go`, `registry.go` | - | `Session` has ID, AccessType, Username, AccessInterface, MAC, TunnelID, SessionID, IPv4Addr, IPv6InterfaceID, IPv6Prefix, AcctSessionID; no Calling-Station-Id, no NAS-Port-Id. `LookupByAcctSessionID` is a linear scan returning the first hit; its only non-test caller is `coa.go` |
| `internal/component/l2tp/subscriber_bridge.go` | - | `onSessionUp` (57) and the IP-assigned path (135) set `AcctSessionID = l2tpSessionID(T,S)` = `l2tp-T-S` |
| `internal/component/l2tp/pppoe/subsystem.go` | - | lines 397 and 445 set `AcctSessionID` to the internal PPPoE id |
| `internal/component/l2tp/plugins/shaper/shaper.go` | - | `onSessionRateChange` (152): unknown session → warn and return; `applyTC` error → warn and return; no result to the caller. Engine subscribers run synchronously inside `Emit` (`internal/core/events/typed.go` 97-102) |
| `internal/plugins/cos/handler.go` | - | `onCoSChange` (167): empty interface → return; `coreCos.Lookup` miss → warn and return; stores `profileName` before `updateQoS`; `updateQoS` error → warn and return |
| `internal/core/cos/cos.go` | - | `ParseFilterID` (94) strips the `cos:` prefix only; `Lookup` (39) answers whether a profile is registered |
| `rfc/audit/rfc5176.json` (HEAD) | - | RFC5176-2.3-6, -2.3-7, -3.3-1, -3.6-1, -2.3-5 all `enforced`; 2.3-6 note judges emit-level atomicity sufficient because `deliverEvent` fails only for an unregistered event |

**Behavior to change:** (only what the user asked for)
- S-1: an Acct-Session-Id matches only the id Ze issued to that live session, compared whole.
- S-2: one matcher over one session table; the L2TP-only fallback in `handleCoA` and the `oneSession`/`findSessions` pair are replaced, not kept beside it.
- S-3: every accepted identification attribute is evaluated against the session or the NAS; the ones Ze has no fact for are handled per Q-3.
- T-1: an unknown CoS profile is refused 407 before any emit; `onCoSChange` records the profile only after `updateQoS` succeeds.
- T-2: after the applied-result contract lands, a failed later part rolls back the earlier ones and the answer is CoA-NAK.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- a CoA-Request or a Disconnect-Request arriving on the CoA UDP listener
- `coaListener.serve` → `handlePacket(data, *net.UDPAddr)`; the listener exists only when `coa-port` is set and a `server` address resolves (`register.go` line 394)
- the issued id enters at `radiusAcct.onSessionIPAssigned` (`l2tpevents.SessionIPAssigned`) and leaves at `onSessionDown`

### Transformation Path
1. `handlePacket` authenticates, dedups, checks Event-Timestamp, Service-Type and the accepted attribute set (unchanged except Q-3 removes the unevaluable identification attributes from both sets).
2. CoA only: `readCoAChange` reads the change; T-1 adds the CoS profile registry lookup here, before any session is touched.
3. New: one selector evaluates the NAS identification attributes against Ze's own identity (configured `nas-identifier` or `hostNASIdentifier`, configured `source-address`), then filters the registry's sessions by every supplied session identification attribute. Result: zero (503), several (508) or one session.
4. CoA: `applySubscriberCoA` on the one session (refusals first, then emits). Under AC-8, each emit returns the applied result and a failure triggers rollback of the parts already applied.
5. Disconnect: L2TP session → `l2tp.Service.TeardownSession(SessionID)`; PPPoE session → 504 as today (lifecycle I-4 owns PPPoE teardown).
6. `sendResponse` echoes Proxy-State/State and remembers the answer for duplicates (unchanged).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CoA listener ↔ subscriber registry | `subscriber.LookupService().Registry` read (`All` or a new filtered read) | Yes, `coa.go` 576-586 |
| Accountant ↔ subscriber registry | new: the accountant writes the issued Acct-Session-Id onto the registry entry for (T,S) and clears it at session down (Q-2 B) | No, assumption A-2 |
| CoA listener ↔ shaper / CoS plugins | typed events on the bus, engine subscribers run synchronously | Yes, `typed.go` 97-102 |
| CoA listener ↔ L2TP service | `l2tp.LookupService().TeardownSession` | Yes, `coa.go` 551-568 |

### Integration Points
- `internal/component/l2tp/subscriber/` - the registry the selector reads (the skeleton's `internal/component/subscriber/` path does not exist)
- `internal/component/l2tp/` - `Service.TeardownSession`; the snapshot `findSessions` reads today stops being read by the CoA path
- `internal/component/l2tp/plugins/authradius/acct.go` - issuer of the Acct-Session-Id, new writer of the registry field
- `internal/core/cos/` - `Lookup` for T-1
- `internal/plugins/cos/handler.go`, `internal/component/l2tp/plugins/shaper/shaper.go` - consumers whose applied result T-2 needs (contract owned by `spec-lifecycle-invariants`)

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes (design) | the selector reads the registry the L2TP bridge and PPPoE subsystem already populate; teardown still goes through `l2tp.Service` |
| No unintended coupling (components stay isolated) | Yes (design) | authradius already imports `l2tp/subscriber` (`coa.go`); the accountant writes through the registry's public API, no new import direction |
| No duplicated functionality (extends existing, does not recreate) | Yes (design) | replaces `findSubscriberSession` + `oneSession` + `findSessions` with one selector; deletes the L2TP-only branch of `handleCoA` |
| Zero-copy preserved where applicable | N-A | control plane, one request per DAC action |
| Registration over hardcoding, outbound | N-A | no new command, family or handler |
| Registration over hardcoding, inbound | Yes (design) | the evaluated identification attributes are one table in the selector, keyed by attribute type, read by both accepted-set checks so the two cannot drift (a type accepted but not evaluated fails a unit test) |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The signed extraction's mappings remain accurate after the residual fixes | the 2026-09-01 artifact accounts for the source sites but does not prove these behaviours | a signature masks stale evidence | review affected sites, tests and discrimination records against each changed producer | unvalidated |
| A-2 | The registry holds an entry for every live L2TP session before its Acct-Session-Id is issued, so the accountant can write onto it | `subscriber_bridge.go` creates the entry at session up and at IP assignment (line 135-147); the accountant issues at IP assignment | the write finds no entry and the session is unreachable by its id | unit test ordering both events both ways; grep event subscription order | unvalidated |
| A-3 | `subscriber_bridge.go` no longer overwrites `AcctSessionID` once the field means the wire id | lines 72 and 147 assign it today | a later SessionUp erases the issued id | the bridge stops writing the field; a unit test emits SessionUp after IP-assigned | unvalidated |
| A-4 | Nothing else reads `Session.AcctSessionID` or calls `LookupByAcctSessionID` | grep 2026-10-13: only `coa.go` (non-test) | redefining the field breaks a reader | grep at implementation, including `show` commands and web views | unvalidated |
| A-5 | The NAS-Port Ze sends for an L2TP session is its LocalSID | `handler.go` line 299 and `acct.go` line 421 both send `uint32(sessionID)` | NAS-Port evaluation compares the wrong value | read both producers at implementation | unvalidated |
| A-6 | The NAS-Port-Id Ze sent can be recomputed at match time from `resolveNASPortID(format, nasID, T, S)` | `acct.go` 296 computes it from config and (T,S) only | a reload that changes the format makes the old value unrecoverable | store the resolved value on the registry entry with the issued id instead, if the format is reloadable | unvalidated |
| A-7 | `spec-lifecycle-invariants` AC-4 delivers a synchronous applied result to the CoA caller | that spec's AC-3/AC-4 and its boundary paragraph | AC-8 cannot be built; T-2 stays open | read that spec's design at implementation start; AC-8 phase is last | unvalidated |
| A-8 | A real DAC (FreeRADIUS `radclient`, `radiusd` originate) sends only attributes Ze can evaluate when it copies from accounting | Ze's Accounting-Request carries Acct-Session-Id, User-Name, NAS-Port, NAS-Port-Id, Calling-Station-Id, Framed-IP-Address, NAS-IP-Address/NAS-Identifier (`acct.go` 395-445) | Q-3 (i) refuses common requests | interop scenarios send the copied set | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Making the change set atomic needs a rollback the shaper events cannot give | an emit that cannot be undone | AC-8 waits for the applied-result contract (A-7); rollback re-applies the prior rate held in shaper state; a failed rollback is NAKed 506 and logged at error with the RFC section, never ACKed |
| R-2 | Sessions alive across an accounting reload lose their issued id (`detach` moves the records to a retired accountant) | a valid Disconnect after a reload answers 503 | under Q-2 B the id lives on the registry entry, not in the accountant, so a reload does not clear it; unit test reloads between issue and Disconnect |
| R-3 | No accounting configured means no id is issued, so a DAC that addresses by Acct-Session-Id gets 503 | Disconnect with an id answered 503 on a NAS without accounting | correct per Section 3 (the NAS issued no such id); documented in `docs/guide/l2tp.md`; User-Name and NAS-Port still select |
| R-4 | Q-3 (i) refuses a deployed DAC that sends Called-Station-Id or CUI | NAK 401 in an operator log | A-8 interop evidence; the guide lists the evaluated attributes; owner may pick (ii) |
| R-5 | NAS identity check refuses a DAC that sends the NAS-IP-Address it learned from the packet source while Ze has no `source-address` configured | NAK 403 with a correct session id | compare NAS-IP-Address with the configured `source-address` and, when none, with the local address the CoA arrived on; unit test both |
| R-6 | `spec-radius-rfc-defects` edits the same file (replay key, `findSessions` comment, id format) | merge conflict or a test asserting the prefix | land order: radius-rfc-defects first; this spec's exact match is format-agnostic and deletes `findSessions` together with that spec's comment |
| R-7 | Fixture changes across ~55 tagged units stale every rfc5176 discrimination record at once | `./le rfc check` red on stale records | change only the shared fixture helpers; re-record every staled unit with `./le rfc discriminate-record` in the same phase |
| R-8 | Concurrent CoA and session teardown: the selected session goes down between selection and emit | an emit for a gone session; ACK for nothing | T-2's applied result reports it once A-7 lands; before that, `TeardownSession` error already answers 504 for Disconnect |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | a Disconnect tears down the wrong subscriber, a valid CoA is refused, or a change is ACKed and not applied |
| How is it reverted? | single commit revert per phase; no config, wire or storage format changes (the registry field is in-memory) |
| Who else touches this path? | `spec-radius-rfc-defects` (coa.go replay key, acct.go id format), `spec-lifecycle-invariants` (applied result, PPPoE delivery), `spec-rfc-verdict-fix-access` (planned `radius-coa-disconnect-freeradius`), `spec-rfcgate-6-supported-extraction-signoff` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| a Disconnect-Request on UDP `coa-port` carrying an Acct-Session-Id issued to an ended session | → | the new selector replacing `findSessions` | `test/l2tp/radius-coa-stale-acct-session-id.ci` (daemon, live session, real socket) |
| a CoA-Request on UDP `coa-port` carrying the wire Acct-Session-Id of an L2TP session and a CoS profile | → | selector → `applySubscriberCoA` | `TestRFC5176CoSChangeReachesAnL2TPSessionFoundByItsAcctSessionID` (real listener socket via `newCoAListener`) |
| a CoA-Request carrying a rate change and a CoS change where the second fails to apply | → | `applySubscriberCoA` rollback | `TestRFC5176FailedCoSApplicationRestoresTheRate` (gated on A-7) |

## Acceptance Criteria

Skeleton mapping: old AC-1 is AC-8, old AC-2 is AC-2 and AC-6, old AC-3 is AC-9, old AC-4 is AC-10.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Session (T,S) issued id X, went down; a new session reuses (T,S) and is issued id Y. A Disconnect-Request, then a CoA-Request, each carrying X | Disconnect-NAK and CoA-NAK, each with Error-Cause 503; zero `TeardownSession` calls, zero bus events. The same two requests carrying Y: Disconnect-ACK with one teardown of (T,S); CoA-ACK with one rate event |
| AC-2 | A request carrying the current session's Acct-Session-Id plus ONE of User-Name, NAS-Port, Framed-IP-Address, Calling-Station-Id, NAS-Port-Id, Framed-IPv6-Prefix with Framed-Interface-Id, whose value belongs to another session or to none | NAK 503 and no effect, for each attribute in turn and for both request codes. With that attribute's value equal to the session's: ACK and the change made. Each attribute alone (no Acct-Session-Id) also selects its session |
| AC-3 | A request whose NAS-IP-Address or NAS-Identifier is not Ze's (configured `source-address`, else the address the request arrived on; configured `nas-identifier`, else the host name) | NAK 403, no effect. Equal values: the request proceeds to session selection. Revised from 503 on 2026-10-13 (RFC-tagged test review, checked against `rfc/full/rfc5176.txt`): Section 3.5 assigns this case its own cause, ""NAS Identification Mismatch" is a fatal error sent if one or more NAS identification attributes (see Section 3) do not match the identity of the NAS receiving the Request.", while 503 is for ""Session Context Not Found" is a fatal error sent if the session context identified in the CoA-Request or Disconnect-Request does not exist on the NAS." |
| AC-4 | A request carrying an identification attribute Ze holds no fact for (Called-Station-Id, Acct-Multi-Session-Id, Chargeable-User-Identity, NAS-IPv6-Address) | under Q-3 (i): NAK 401, no effect, for both request codes. Every type in either accepted set is either evaluated by the selector or not accepted, checked by a unit test over the two sets |
| AC-5 | A CoA-Request for an L2TP session with an access interface, selected by its wire Acct-Session-Id, asking for a known CoS profile | CoA-ACK and one `l2tp/session-cos-change` event for that interface (today: NAK 506) |
| AC-6 | The supplied identifiers match two sessions (two L2TP sessions with one User-Name; and an L2TP and a PPPoE session with one User-Name) | CoA-NAK / Disconnect-NAK with Error-Cause 508, no session changed (preserved behaviour, now across both access types) |
| AC-7 | A CoA-Request whose CoS profile (`cos:<name>` Filter-Id or a vendor CoS VSA) is not registered in `internal/core/cos` | CoA-NAK with Error-Cause 407, zero bus events, also when a valid rate rides beside it. A registered profile is ACKed. `onCoSChange` stores the profile name only after `updateQoS` succeeds |
| AC-8 | (gated on A-7) A CoA-Request carrying a rate and a CoS profile; the rate applies and the CoS application reports failure | the session's previous rate is re-applied, CoA-NAK with Error-Cause 506, and no requested change remains. When the re-apply itself fails: CoA-NAK 506 and an error log naming the session and RFC 5176 Section 2.3; never a CoA-ACK |
| AC-9 | The retained Termination-Action question (Q-6) is resolved | If Thomas selects the optional reauthentication feature, its Access-Request echoes State as required and carries end-to-end proof. Otherwise the recorded exclusion remains explicit and no implementation or conformance completion is inferred from the absence of attribute 29 |
| AC-10 | The residual behaviours and affected RFC 5176 evidence are checked | Selector and atomicity tests prove both success and refusal, every staled tagged unit carries a fresh discrimination record, RFC5176-3.3-1, -2.3-5, -2.3-6, -2.3-7 and -3.6-1 are re-judged by an agent that did not write the tests, and the extraction stays signed; the 2026-09-01 signature is not counted as new work |
| AC-11 | `docs/guide/l2tp.md` CoA/DM section after each phase | the check table names the evaluated identification attributes, the 401 for unevaluable ones, the 407 for an unknown profile, and "was applied" for the 506 row; the source anchors name the new selector |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | an operator's RADIUS server sends a late Disconnect-Request for a subscriber who already reconnected | UDP 3799 → `handlePacket` → selector (registry, exact Acct-Session-Id) → 503 | `test/l2tp/radius-coa-stale-acct-session-id.ci`; interop `l2tp-radius-coa-stale-session-freeradius` |
| 2 | a DAC changes a subscriber's rate and CoS profile in one request | UDP 3799 → `readCoAChange` (profile looked up) → selector → `applySubscriberCoA` → shaper and CoS consumers | `TestRFC5176CoSChangeReachesAnL2TPSessionFoundByItsAcctSessionID`; interop `l2tp-radius-coa-identification-freeradius` |
| 3 | a DAC addresses a subscriber by User-Name and Framed-IP-Address that disagree | UDP 3799 → selector → 503 | `TestRFC5176EveryIdentificationAttributeNarrowsTheMatch`; interop `l2tp-radius-coa-identification-freeradius` |

## 🧪 TDD Test Plan

### Unit Tests (new; file `internal/component/l2tp/plugins/authradius/rfc5176_session_selector_test.go` unless named)
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC5176StaleAcctSessionIDMatchesNoSession` | selector file | AC-1 negative, RFC5176-3.3-1. Red at HEAD (prefix match tears down) | |
| `TestRFC5176CurrentAcctSessionIDMatchesItsSession` | selector file | AC-1 positive | |
| `TestRFC5176EveryIdentificationAttributeNarrowsTheMatch` | selector file | AC-2, table over the six attributes, both codes, both polarities. Red at HEAD for Framed-IP-Address, Calling-Station-Id, NAS-Port-Id, IPv6 pair | |
| `TestRFC5176ForeignNASIdentityIsAMismatch` | selector file | AC-3, both NAS-IP-Address sources (configured and arrival address) and NAS-Identifier (configured and host name): NAK with Error-Cause 403, never 503 | |
| `TestRFC5176UnevaluableIdentificationAttributeIsUnsupported` | selector file | AC-4 (Q-3 i), RFC5176-2.3-5 | |
| `TestCoAAcceptedIdentificationTypesAreAllEvaluated` | selector file | AC-4 drift guard: every identification type in `coaSupportedAttrs` and `disconnectSupportedAttrs` has an evaluator | |
| `TestRFC5176CoSChangeReachesAnL2TPSessionFoundByItsAcctSessionID` | selector file | AC-5. Red at HEAD (506) | |
| `TestRFC5176MultipleMatchAcrossAccessTypesIsNAKed` | selector file | AC-6, RFC5176-2.3-7 | |
| `TestRFC5176UnknownCoSProfileIsNAKed` | `rfc5176_coa_values_test.go` | AC-7, RFC5176-2.3-5. Red at HEAD (ACK) | |
| `TestCoSChangeRecordsProfileOnlyAfterApply` | `internal/plugins/cos/handler_test.go` | AC-7 consumer half | |
| `TestRFC5176FailedCoSApplicationRestoresTheRate` | `rfc5176_coa_values_test.go` | AC-8 (gated on A-7), RFC5176-2.3-6 | |
| `TestRFC5176FailedRollbackIsNAKedAndLogged` | `rfc5176_coa_values_test.go` | AC-8 rollback failure | |
| `TestAcctIssuedSessionIDIsRecordedOnTheSubscriberSession` | `internal/component/l2tp/plugins/authradius/acct_session_id_registry_test.go` | Q-2 B: written at issue, cleared at down, survives `detach` (R-2), not overwritten by a later SessionUp (A-2, A-3) | |

### Tagged tests that change (each needs the owner's approval, Q-5)

Behaviour or body changes:

| Test | File | Tag (verdict) | Change | Why |
|------|------|---------------|--------|-----|
| `TestRFC5176IdentificationAttributesIdentifyOnly` (both units) | `rfc5176_walk_test.go` | RFC5176-3.6-1 (`enforced`) | positive request drops Called-Station-Id and sets NAS-Identifier to the listener's identity; the NEGATIVE ("bare": User-Name, NAS-Port, Called-Station-Id, expecting 401) also drops Called-Station-Id, so its 401 comes from what the unit claims and not from Called-Station-Id alone | under Q-3 (i) Called-Station-Id is NAKed 401 on its own, so a negative that keeps it passes for the wrong reason (UNDER-TEST, isolation; confirmed in the body); under AC-3 a foreign NAS-Identifier is 403 |
| `TestRFC5176DisconnectCarryingOnlyIdentificationIsACKed` | `rfc5176_section_rows_test.go` | RFC5176-3-1 NEGATIVE (`enforced`; earlier drafts of this table said positive, the tag line says negative) | drops Called-Station-Id; Calling-Station-Id, NAS-Identifier and NAS-IP-Address take the session's and the NAS's values; the tag prose that lists Called-Station-Id among the session identification attributes is rewritten to name only "the identification attributes Ze evaluates" | Depends on Q-3: Called-Station-Id is a Section 3 session identification attribute, and under Q-3 (i) a Disconnect carrying it is NAKed 401 "Unsupported Attribute". That is permitted, because Ze does not support it, but the negative then proves only the attributes Ze evaluates; the owner's Q-3 answer decides this unit's claim |
| `TestRFC5176SubscriberCoAThatCannotBeCarriedOutWholeMakesNoChange`, `TestRFC5176RequestWhoseEveryValueIsSupportedIsACKed` | `rfc5176_coa_values_test.go` | RFC5176-2.3-6, -2.3-5 (`enforced`) | helper `subscriberSession` unchanged; assertions unchanged; producer changes stale the records | re-record only |
| `TestRFC5176CoANAKMakesNoPartialChange` | `rfc5176_coa_branches_test.go` | RFC5176-2.3-6 (`enforced`) | its premise "L2TP-only session, CoS refused 506" becomes "registry session with no access interface" | the L2TP-only branch is deleted (no-layering) |
| `TestRFC5176ChangeThatCannotBeCarriedOutIsNAKed` (both units) | `rfc5176_walk_test.go` | RFC5176-2.3-6 (`enforced`) | same premise change if it relies on the L2TP-only branch; the implementer confirms by reading the body before the first edit | same |
| `TestRFC5176RequestNotRoutableCauseNeverSentForAForeignSession` | `rfc5176_section_rows_test.go` | RFC5176-3.5-8 negative | the request's NAS-Identifier changes from "other-nas" to the listener's identity, so its 503 comes from the unknown user `bob@elsewhere.example` (Session Context Not Found) and the assertion [503] stays; the foreign-NAS case is carried by `TestRFC5176ForeignNASIdentityIsAMismatch` (403). Alternative for the owner: keep "other-nas" and assert [403] | WRONG after the change otherwise (confirmed: the body sends NAS-Identifier "other-nas" and asserts [503]); under AC-3 that request draws 403. Body change, so it needs Q-5 approval |

Fixture-only changes (Q-2 B: the selector reads the registry, so every test that publishes a fake L2TP service and sends `10-20-1` with no registry entry needs one). The edit is confined to the shared helpers (`fakeL2TPService` in `rfc5176_coa_listener_test.go`, `twoAliceSessions`, `twoSessionService`, `newWindowListener`, and the fake built in `rfc5176_coa_test.go` and `disconnect_service_stopped_test.go`), which also seed the registry; the test bodies do not change. Units affected, by file (from a scan of 2026-10-13 for units sending a `T-S[-N]` Acct-Session-Id; most carry an `RFC requirement:` tag, and the implementer re-runs the scan, drops untagged units from the approval list and adds any tagged unit the scan missed before the first edit):

| File | Units |
|------|--------------|
| `rfc5176_coa_branches_test.go` | `TestRFC5176CoAWithAnUnsupportedAttributeValueIsNAKed`, `TestRFC5176DisconnectThatCannotTerminateIsNAKed`, `TestRFC5176DisconnectThatTerminatesIsACKed`, `TestRFC5176MatchedIdentificationSucceeds`, `TestRFC5176NAKForARequestWithoutStateCarriesNone`, `TestRFC5176ResponseAuthenticatorMatchesAnIndependentMD5`, `TestRFC5176UnmatchedIdentificationIsNAKed`, `TestRFC5176CoAMatchingOneSessionIsACKed`, `TestRFC5176CoAMatchingSeveralSessionsIsNAKed` |
| `rfc5176_coa_listener_test.go` | `TestCoAListenerInvalidAuth`, `TestCoAListenerMissingEventTimestamp`, `TestCoAListenerMissingMessageAuthenticatorDroppedWhenRequired`, `TestCoAListenerUnknownSession`, `TestDisconnectListenerUnknownSession`, `TestDisconnectReplayReturnsCachedResponse` |
| `rfc5176_coa_test.go` | `TestCoASourceFilterAnswersAConfiguredClient`, `TestCoASourceFilterDiscardsWhenNoServerResolved`, `TestRFC5176ResponseAuthenticator`, `TestRFC5176NoSessionIdNotActedOn` |
| `rfc5176_coa_values_test.go` | `TestRFC5176CoAWithOneUnsupportedValueBesideSupportedOnesIsNAKed`, `TestRFC5176DisconnectWithAnUnsupportedValueIsNAKed` |
| `rfc5176_message_authenticator_optional_test.go` | `TestRFC5176MessageAuthenticatorAbsentIsAcceptedByDefault`, `TestRFC5176WrongMessageAuthenticatorDiscardedWhenNotRequired` |
| `rfc5176_message_authenticator_test.go` | `TestRFC5176ListenerAcceptsConformantMessageAuthenticator`, `TestRFC5176ListenerDiscardsWrongMessageAuthenticator` |
| `rfc5176_proxy_state_opaque_test.go` | `TestRFC3579SubscriberNASRefusesEAPMessage`, `TestRFC5176ProxyStateContentIsNeverRead`, `TestRFC5176StateAndClassUnmodifiedProxyStateOpaque` |
| `rfc5176_section_rows_test.go` | `TestRFC5176CoAWithoutServiceTypeIsACKed`, `TestRFC5176DisconnectAdmitsTheSection36TableAttributes`, `TestRFC5176DisconnectWithANonIdentificationAttributeIsNAKed`, `TestRFC5176DuplicateAndTimestampWindowsBothEndOutside`, `TestRFC5176DuplicateAndTimestampWindowsBothHoldInside`, `TestRFC5176InvalidEAPPacketCauseNeverSentForAnEAPRequest`, `TestRFC5176SessionNotRemovableNeverInACoANAK`, `TestRFC5176StateIsNeverReadAsAChangeOrAnIdentity`, `TestRFC5176StateNeverChangesTheOutcome`, `TestRFC5176UnsupportedServiceTypeValueIsNAKed`, `TestRFC5176VendorSpecificIsAChangeOnly`, `TestRFC5176VendorSpecificIsNeverAnIdentity` |
| `rfc5176_walk_test.go` | `TestRFC5176DuplicateRequestAnsweredFromCache`, `TestRFC5176ErrorCausePlacement`, `TestRFC5176InvalidCodeDiscarded`, `TestRFC5176LengthFieldGovernsTheOctetsRead`, `TestRFC5176MessageAuthenticatorZeroesBothFields`, `TestRFC5176ProxyStateReturnedUnmodified`, `TestRFC5176ServiceTypeNAKed`, `TestRFC5176StaleEventTimestampDiscarded`, `TestRFC5176StateReturnedUnmodified`, `TestRFC5176UnsupportedAttributeNAKed`, `TestRFC5176SharedSecretChosenBySourceAddress`, `TestRFC5176UntrustedSourceDiscarded`, `TestRFC5176MultipleMatchingSessionsNAKed` |
| `disconnect_service_stopped_test.go` | `TestRFC5176DisconnectLocatedButServiceStoppedIsNotRemovable` |

Four of these (`TestRFC5176DuplicateRequestAnsweredFromCache`, `TestRFC5176DuplicateAndTimestampWindowsBoth*`, `TestDisconnectReplayReturnsCachedResponse`) are also rewritten by `spec-radius-rfc-defects` (its Q-3); whichever spec lands second rebases onto the other's version. Every staled unit is re-recorded with `./le rfc discriminate-record`.

#### Owner approval commands

Approved by the owner on 2026-10-13 (Q-5). The gate reads approvals from the committing session's own file (`tmp/commit-rfc-approved-<session>.md`), and a row is used up by the commit that carries its `RFC-approved:` trailer. So the owner runs these at the prompt of the session that implements and commits the edits, and runs a unit's command again if a later commit changes that unit a second time (the four units shared with `spec-radius-rfc-defects` need it again in whichever spec lands second). A unit is `<package directory>.<top-level test function>`; every unit here is in `internal/component/l2tp/plugins/authradius/`, so the package is `authradius` (a same-named `TestRFC5176MessageAuthenticatorZeroesBothFields` also exists in `radius`, which this spec does not touch). Every name was checked to exist at HEAD `051876c3f4` and to carry an `RFC requirement:` tag. Three listed units are untagged at HEAD and need no approval: `TestCoAListenerMissingEventTimestamp`, `TestDisconnectListenerUnknownSession`, `TestRFC5176DisconnectLocatedButServiceStoppedIsNotRemovable`. The gate compares each tagged function's own text, so a re-record-only unit, or a fixture-only unit whose body stays byte-identical because only the shared helpers change, is not asked for and its row goes unused. The first seven are the behaviour and premise edits; the rest are the fixture-only units. The implementer's re-scan before the first edit may add a tagged unit this list missed; that unit needs its own command in the same form.

58 commands:

```
./le rfc approve unit authradius.TestRFC5176IdentificationAttributesIdentifyOnly reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-rfc5176-remaining-defect-classes, both polarities drop Called-Station-Id, NAS-Identifier set to the listener)'
./le rfc approve unit authradius.TestRFC5176DisconnectCarryingOnlyIdentificationIsACKed reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-rfc5176-remaining-defect-classes, drops Called-Station-Id, prose names only attributes Ze evaluates)'
./le rfc approve unit authradius.TestRFC5176SubscriberCoAThatCannotBeCarriedOutWholeMakesNoChange reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-rfc5176-remaining-defect-classes, producer change, re-record only)'
./le rfc approve unit authradius.TestRFC5176RequestWhoseEveryValueIsSupportedIsACKed reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-rfc5176-remaining-defect-classes, producer change, re-record only)'
./le rfc approve unit authradius.TestRFC5176CoANAKMakesNoPartialChange reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-rfc5176-remaining-defect-classes, premise becomes registry session with no access interface)'
./le rfc approve unit authradius.TestRFC5176ChangeThatCannotBeCarriedOutIsNAKed reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-rfc5176-remaining-defect-classes, premise change if it relies on the L2TP-only branch)'
./le rfc approve unit authradius.TestRFC5176RequestNotRoutableCauseNeverSentForAForeignSession reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-rfc5176-remaining-defect-classes, NAS-Identifier becomes the listener identity, or keep other-nas and assert 403)'
./le rfc approve unit authradius.TestRFC5176CoAWithAnUnsupportedAttributeValueIsNAKed reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-rfc5176-remaining-defect-classes, fixture-only, shared helpers seed the registry)'
./le rfc approve unit authradius.TestRFC5176DisconnectThatCannotTerminateIsNAKed reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-rfc5176-remaining-defect-classes, fixture-only, shared helpers seed the registry)'
./le rfc approve unit authradius.TestRFC5176DisconnectThatTerminatesIsACKed reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-rfc5176-remaining-defect-classes, fixture-only, shared helpers seed the registry)'
./le rfc approve unit authradius.TestRFC5176MatchedIdentificationSucceeds reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-rfc5176-remaining-defect-classes, fixture-only, shared helpers seed the registry)'
./le rfc approve unit authradius.TestRFC5176NAKForARequestWithoutStateCarriesNone reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-rfc5176-remaining-defect-classes, fixture-only, shared helpers seed the registry)'
./le rfc approve unit authradius.TestRFC5176ResponseAuthenticatorMatchesAnIndependentMD5 reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-rfc5176-remaining-defect-classes, fixture-only, shared helpers seed the registry)'
./le rfc approve unit authradius.TestRFC5176UnmatchedIdentificationIsNAKed reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-rfc5176-remaining-defect-classes, fixture-only, shared helpers seed the registry)'
./le rfc approve unit authradius.TestRFC5176CoAMatchingOneSessionIsACKed reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-rfc5176-remaining-defect-classes, fixture-only, shared helpers seed the registry)'
./le rfc approve unit authradius.TestRFC5176CoAMatchingSeveralSessionsIsNAKed reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-rfc5176-remaining-defect-classes, fixture-only, shared helpers seed the registry)'
./le rfc approve unit authradius.TestCoAListenerInvalidAuth reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-rfc5176-remaining-defect-classes, fixture-only, shared helpers seed the registry)'
./le rfc approve unit authradius.TestCoAListenerMissingMessageAuthenticatorDroppedWhenRequired reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-rfc5176-remaining-defect-classes, fixture-only, shared helpers seed the registry)'
./le rfc approve unit authradius.TestCoAListenerUnknownSession reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-rfc5176-remaining-defect-classes, fixture-only, shared helpers seed the registry)'
./le rfc approve unit authradius.TestDisconnectReplayReturnsCachedResponse reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-rfc5176-remaining-defect-classes, fixture-only, shared helpers seed the registry)'
./le rfc approve unit authradius.TestCoASourceFilterAnswersAConfiguredClient reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-rfc5176-remaining-defect-classes, fixture-only, shared helpers seed the registry)'
./le rfc approve unit authradius.TestCoASourceFilterDiscardsWhenNoServerResolved reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-rfc5176-remaining-defect-classes, fixture-only, shared helpers seed the registry)'
./le rfc approve unit authradius.TestRFC5176ResponseAuthenticator reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-rfc5176-remaining-defect-classes, fixture-only, shared helpers seed the registry)'
./le rfc approve unit authradius.TestRFC5176NoSessionIdNotActedOn reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-rfc5176-remaining-defect-classes, fixture-only, shared helpers seed the registry)'
./le rfc approve unit authradius.TestRFC5176CoAWithOneUnsupportedValueBesideSupportedOnesIsNAKed reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-rfc5176-remaining-defect-classes, fixture-only, shared helpers seed the registry)'
./le rfc approve unit authradius.TestRFC5176DisconnectWithAnUnsupportedValueIsNAKed reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-rfc5176-remaining-defect-classes, fixture-only, shared helpers seed the registry)'
./le rfc approve unit authradius.TestRFC5176MessageAuthenticatorAbsentIsAcceptedByDefault reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-rfc5176-remaining-defect-classes, fixture-only, shared helpers seed the registry)'
./le rfc approve unit authradius.TestRFC5176WrongMessageAuthenticatorDiscardedWhenNotRequired reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-rfc5176-remaining-defect-classes, fixture-only, shared helpers seed the registry)'
./le rfc approve unit authradius.TestRFC5176ListenerAcceptsConformantMessageAuthenticator reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-rfc5176-remaining-defect-classes, fixture-only, shared helpers seed the registry)'
./le rfc approve unit authradius.TestRFC5176ListenerDiscardsWrongMessageAuthenticator reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-rfc5176-remaining-defect-classes, fixture-only, shared helpers seed the registry)'
./le rfc approve unit authradius.TestRFC3579SubscriberNASRefusesEAPMessage reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-rfc5176-remaining-defect-classes, fixture-only, shared helpers seed the registry)'
./le rfc approve unit authradius.TestRFC5176ProxyStateContentIsNeverRead reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-rfc5176-remaining-defect-classes, fixture-only, shared helpers seed the registry)'
./le rfc approve unit authradius.TestRFC5176StateAndClassUnmodifiedProxyStateOpaque reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-rfc5176-remaining-defect-classes, fixture-only, shared helpers seed the registry)'
./le rfc approve unit authradius.TestRFC5176CoAWithoutServiceTypeIsACKed reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-rfc5176-remaining-defect-classes, fixture-only, shared helpers seed the registry)'
./le rfc approve unit authradius.TestRFC5176DisconnectAdmitsTheSection36TableAttributes reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-rfc5176-remaining-defect-classes, fixture-only, shared helpers seed the registry)'
./le rfc approve unit authradius.TestRFC5176DisconnectWithANonIdentificationAttributeIsNAKed reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-rfc5176-remaining-defect-classes, fixture-only, shared helpers seed the registry)'
./le rfc approve unit authradius.TestRFC5176DuplicateAndTimestampWindowsBothEndOutside reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-rfc5176-remaining-defect-classes, fixture-only, shared helpers seed the registry)'
./le rfc approve unit authradius.TestRFC5176DuplicateAndTimestampWindowsBothHoldInside reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-rfc5176-remaining-defect-classes, fixture-only, shared helpers seed the registry)'
./le rfc approve unit authradius.TestRFC5176InvalidEAPPacketCauseNeverSentForAnEAPRequest reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-rfc5176-remaining-defect-classes, fixture-only, shared helpers seed the registry)'
./le rfc approve unit authradius.TestRFC5176SessionNotRemovableNeverInACoANAK reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-rfc5176-remaining-defect-classes, fixture-only, shared helpers seed the registry)'
./le rfc approve unit authradius.TestRFC5176StateIsNeverReadAsAChangeOrAnIdentity reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-rfc5176-remaining-defect-classes, fixture-only, shared helpers seed the registry)'
./le rfc approve unit authradius.TestRFC5176StateNeverChangesTheOutcome reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-rfc5176-remaining-defect-classes, fixture-only, shared helpers seed the registry)'
./le rfc approve unit authradius.TestRFC5176UnsupportedServiceTypeValueIsNAKed reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-rfc5176-remaining-defect-classes, fixture-only, shared helpers seed the registry)'
./le rfc approve unit authradius.TestRFC5176VendorSpecificIsAChangeOnly reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-rfc5176-remaining-defect-classes, fixture-only, shared helpers seed the registry)'
./le rfc approve unit authradius.TestRFC5176VendorSpecificIsNeverAnIdentity reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-rfc5176-remaining-defect-classes, fixture-only, shared helpers seed the registry)'
./le rfc approve unit authradius.TestRFC5176DuplicateRequestAnsweredFromCache reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-rfc5176-remaining-defect-classes, fixture-only, shared helpers seed the registry)'
./le rfc approve unit authradius.TestRFC5176ErrorCausePlacement reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-rfc5176-remaining-defect-classes, fixture-only, shared helpers seed the registry)'
./le rfc approve unit authradius.TestRFC5176InvalidCodeDiscarded reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-rfc5176-remaining-defect-classes, fixture-only, shared helpers seed the registry)'
./le rfc approve unit authradius.TestRFC5176LengthFieldGovernsTheOctetsRead reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-rfc5176-remaining-defect-classes, fixture-only, shared helpers seed the registry)'
./le rfc approve unit authradius.TestRFC5176MessageAuthenticatorZeroesBothFields reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-rfc5176-remaining-defect-classes, fixture-only, shared helpers seed the registry)'
./le rfc approve unit authradius.TestRFC5176ProxyStateReturnedUnmodified reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-rfc5176-remaining-defect-classes, fixture-only, shared helpers seed the registry)'
./le rfc approve unit authradius.TestRFC5176ServiceTypeNAKed reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-rfc5176-remaining-defect-classes, fixture-only, shared helpers seed the registry)'
./le rfc approve unit authradius.TestRFC5176StaleEventTimestampDiscarded reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-rfc5176-remaining-defect-classes, fixture-only, shared helpers seed the registry)'
./le rfc approve unit authradius.TestRFC5176StateReturnedUnmodified reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-rfc5176-remaining-defect-classes, fixture-only, shared helpers seed the registry)'
./le rfc approve unit authradius.TestRFC5176UnsupportedAttributeNAKed reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-rfc5176-remaining-defect-classes, fixture-only, shared helpers seed the registry)'
./le rfc approve unit authradius.TestRFC5176SharedSecretChosenBySourceAddress reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-rfc5176-remaining-defect-classes, fixture-only, shared helpers seed the registry)'
./le rfc approve unit authradius.TestRFC5176UntrustedSourceDiscarded reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-rfc5176-remaining-defect-classes, fixture-only, shared helpers seed the registry)'
./le rfc approve unit authradius.TestRFC5176MultipleMatchingSessionsNAKed reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-rfc5176-remaining-defect-classes, fixture-only, shared helpers seed the registry)'
```

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| NAS-Port | 32-bit, compared to LocalSID 1-65535 | 65535 matches LocalSID 65535 | 0 matches no session | 65536 matches no session (no uint16 truncation) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `radius-coa-stale-acct-session-id.ci` | `test/l2tp/` | daemon with `coa-port` and accounting to a fixture server; an L2TP session comes up, the fixture captures the Accounting-Start id, the session drops and a new one reuses the numbers; Disconnect with the old id answers 503, with the new id answers ACK and the session ends. New fixture `l2tp/radius-coa-acct-id` in `internal/test/fixture/tunnel_fixture_l2tp_radius.go` | |
| `radius-coa-identification-mismatch.ci` | `test/l2tp/` | live session; CoA with the right Acct-Session-Id and a wrong User-Name answers 503 and the shaper log shows no update; the right pair answers ACK | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `l2tp-radius-coa-stale-session-freeradius` | `test/interop-radius-subscriber/scenarios/` (the `interop-radius-subscriber` lab, Q-7) | FreeRADIUS `radclient` plus its `detail` accounting log, xl2tpd/pppd client | the server's recorded Acct-Session-Id of a reconnected subscriber's first session draws Disconnect-NAK 503 and the live session stays up (Ze log and pppd both read); the current id draws Disconnect-ACK and pppd sees the session end | |
| `l2tp-radius-coa-identification-freeradius` | `test/interop-radius-subscriber/scenarios/` | FreeRADIUS `radclient` | the attribute set FreeRADIUS copies from accounting (A-8) selects the session and a rate CoA is ACKed with the new rate visible in `tc`; the same request with one mismatched identifier is NAKed 503 and `tc` is unchanged | |

Each scenario records a forced red: revert the selector producer, rebuild the Ze image, observe the scenario fail, restore.

## Files to Modify
- `internal/component/l2tp/plugins/authradius/coa.go` - replace `findSubscriberSession`, `oneSession`, `findSessions` and the L2TP-only branch of `handleCoA` with one selector (NAS identity check, then every session identification attribute against the registry); `handleDisconnect` uses the same selector; Q-3 removes the four unevaluable types from both accepted sets; `addFilterID`/`addVendorSpecific` look the CoS profile up (T-1); `applySubscriberCoA` gains rollback (T-2, last phase). Section 3 and 3.5 quotes above the selector and its NAK branches
- `internal/component/l2tp/plugins/authradius/acct.go` - `onSessionIPAssigned` writes the issued id onto the registry entry for (T,S), `onSessionDown` clears it; `genSessionID` comment no longer names a prefix parser
- `internal/component/l2tp/subscriber/session.go`, `registry.go` - `AcctSessionID` documented as the Acct-Session-Id Ze put on the wire; a setter keyed by (T,S) or ID; `CallingStationID` field (and the resolved NAS-Port-Id per A-6) so the selector reads one table; `LookupByAcctSessionID` replaced by the selector's read
- `internal/component/l2tp/subscriber_bridge.go` - stop writing `AcctSessionID` (lines 72, 147); carry Calling-Station-Id from the L2TP payload
- `internal/component/l2tp/pppoe/subsystem.go` - stop writing the internal id into `AcctSessionID` (lines 397, 445)
- `internal/plugins/cos/handler.go` - `onCoSChange` stores the profile name after `updateQoS` succeeds (T-1); later returns the applied result per A-7 (T-2)
- `internal/component/l2tp/plugins/shaper/shaper.go` - none in this spec's own phases; the applied result is `spec-lifecycle-invariants`' change, consumed by T-2
- `internal/component/l2tp/plugins/authradius/*_test.go` - the helpers and units in "Tagged tests that change", only after Q-5 approval
- `internal/test/fixture/tunnel_fixture_l2tp_radius.go`, `tunnel_fixture_l2tp.go` - fixture `l2tp/radius-coa-acct-id` (CoA/Disconnect with a captured id)
- `internal/le/interoplab/radiussubscriber/` - the `interop-radius-subscriber` lab's checkers for this spec's two scenarios, per Q-7 (the lab is shared with `spec-radius-rfc-defects` and built by whichever spec lands first). The spec that builds it also owes the lab rows of `spec-radius-rfc-defects` Files to Modify: the move of the `/dev/ppp` half of BOTH the L2TP lab (`internal/le/interoplab/l2tp/l2tp.go`) and the PPPoE lab (`internal/le/interoplab/pppoe/pppoe.go`), and of their tests, into one shared file in `internal/le/interoplab/` (that spec's Q-7 Preflight row). This list does not copy those rows
- `docs/guide/l2tp.md` - CoA/DM check table and source anchors (AC-11), in the phase that changes each row
- `rfc/audit/rfc5176.json` - re-judged verdicts (stamped by an independent judge); `rfc/discrimination/rfc5176.json` (tool-written)
- `rfc/short/rfc5176.md`, `rfc/extraction/rfc5176.json` - no new rows (scope frozen); edited only if a judge finds a row's prose names the old behaviour

## Files to Create
- `internal/component/l2tp/plugins/authradius/rfc5176_session_selector_test.go` - AC-1..AC-6 units
- `internal/component/l2tp/plugins/authradius/acct_session_id_registry_test.go` - issued-id lifecycle
- `test/l2tp/radius-coa-stale-acct-session-id.ci`, `test/l2tp/radius-coa-identification-mismatch.ci`
- `test/interop-radius-subscriber/scenarios/l2tp-radius-coa-stale-session-freeradius/`, `test/interop-radius-subscriber/scenarios/l2tp-radius-coa-identification-freeradius/`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | no leaf added; `coa-port`, `nas-identifier`, `source-address` already exist |
| YANG validation constraints | N-A | no leaf |
| YANG custom validators | N-A | no leaf |
| CLI commands/flags | No | none |
| CLI grammar (keyword before value) | N-A | no command |
| Editor autocomplete | N-A | no command |
| Functional test for new RPC/API | Yes | `test/l2tp/radius-coa-stale-acct-session-id.ci`, `test/l2tp/radius-coa-identification-mismatch.ci` (existing endpoint, new behaviour) |
| Pipe completeness | N-A | no command output |
| Env var registration | No | none |
| Doctor check for runtime dependencies | No | no new path, port, socket, module or binary; the CoA port and its doctor check exist (`authradius/doctor.go`) |
| Prometheus counters/metrics | No | none added; `recordCoAChanged` in the CoS plugin keeps counting only applied changes |
| BGP family surface | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | defect fixes in an existing feature; the RADIUS CoA feature entry's evidence gains the interop scenarios if `./le feature` lists CoA (implementer checks) |
| 2 | Config syntax changed? | No | none |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | behaviour of an existing plugin; no registration change |
| 6 | Has a user guide page? | Yes | `docs/guide/l2tp.md` CoA/DM section (AC-11) |
| 7 | Wire format changed? | No | same packets; different answers |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/audit/rfc5176.json` re-judged rows; `docs/features/rfc-status.md` regenerated if any verdict moves |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` if the new fixture is listed there; `docs/architecture/testing/interop.md` for the `interop-radius-subscriber` lab section and suites row, and the two scenarios; if this spec builds the lab, also the L2TP and PPPoE preflight text named by `spec-radius-rfc-defects` Documentation row 10 (the shared `/dev/ppp` probe) |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | Yes | `docs/architecture/l2tp/subscriber-session-model.md`: `AcctSessionID` meaning and the removed `LookupByAcctSessionID` anchor; `docs/architecture/traffic/cos-dynamic.md` lines 55-56 (stale `extractRate`/`extractCoSProfile` names) in the T-1 phase |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | events unchanged until A-7's contract, which `spec-lifecycle-invariants` documents |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | derive with `./le spec citation anchors spec plan/immediate/spec-rfc5176-remaining-defect-classes.md` at implementation; known: `coa.go` → `docs/guide/l2tp.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | No | the CoA section shows config only, unchanged |

### Discovery (`ai/rules/repo-maintenance.md` mechanical checklist)
| Question | Answer |
|----------|--------|
| Where does an agent look first? | `ai/CODE-TO-DOCS.md` row for `coa.go` → `docs/guide/l2tp.md` CoA/DM section |
| What rule prevents regression? | `ai/rules/rfc-compliance.md` (Section 3 quote above the selector), tagged tests with discrimination records |
| What registry prevents drift? | the selector's evaluator table, checked against both accepted sets by `TestCoAAcceptedIdentificationTypesAreAllEvaluated` |
| What verification proves it? | the unit tests above, the two `.ci` tests, the two interop scenarios with forced reds, `./le rfc check` |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- failing wiring tests
   - Tests: `TestRFC5176StaleAcctSessionIDMatchesNoSession`, `TestRFC5176CoSChangeReachesAnL2TPSessionFoundByItsAcctSessionID`, `radius-coa-stale-acct-session-id.ci` with its fixture
   - Files: test files, fixture
   - Verify: each is red at HEAD for the reason named in the Unit Tests table
2. **Phase: issued id on the registry (Q-2 B)** -- accountant writes, bridge and PPPoE stop writing, registry documents the field
   - Tests: `TestAcctIssuedSessionIDIsRecordedOnTheSubscriberSession`
   - Files: `acct.go`, `subscriber/session.go`, `registry.go`, `subscriber_bridge.go`, `pppoe/subsystem.go`
   - Verify: A-2, A-3, A-4 confirmed or broken in the spec
3. **Phase: selector (S-1, S-2, S-3, Q-3)** -- delete the three lookups and the L2TP-only branch, add the selector; owner-approved helper and test edits land here
   - Tests: AC-1..AC-6 units, the approved tagged-test changes, both `.ci` tests
   - Files: `coa.go`, the test helpers, `docs/guide/l2tp.md` rows for identification
   - Verify: every staled unit re-recorded; `./le rfc check` green for rfc5176
4. **Phase: unknown CoS profile (T-1)**
   - Tests: `TestRFC5176UnknownCoSProfileIsNAKed`, `TestCoSChangeRecordsProfileOnlyAfterApply`
   - Files: `coa.go`, `internal/plugins/cos/handler.go`, `docs/guide/l2tp.md` 407 row
5. **Phase: interop** -- the `interop-radius-subscriber` lab (Q-7; built here unless `spec-radius-rfc-defects` built it first), both scenarios, forced reds recorded
6. **Phase: rollback (T-2)** -- only after `spec-lifecycle-invariants` AC-4 lands (A-7)
   - Tests: `TestRFC5176FailedCoSApplicationRestoresTheRate`, `TestRFC5176FailedRollbackIsNAKedAndLogged`
   - Files: `coa.go`, `docs/guide/l2tp.md` 506 row
7. **Phase: evidence** -- independent re-judge of the five rows (AC-10); Q-6 outcome recorded (AC-9)

## RFC Documentation (Scope: protocol)

Every MUST this spec implements carries
`// RFC 5176 Section X.Y: "<quoted requirement>"` directly above the enforcing
code, and every tagged test it adds carries a discrimination record written by
`./le rfc discriminate-record`.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Rule: rfc-compliance | no claim is wider than what the test body checks; Section 3 quote above the selector, Section 3.5 quotes above each NAK branch, Section 2.3 above the rollback |
| Completeness | one selector serves CoA and Disconnect for both access types; no second lookup path remains (`ai/rules/no-layering.md`) |
| Correctness | the Acct-Session-Id comparison is whole-string; NAS-Port compares 32 bits without truncation; an attribute present twice is evaluated as a set the session must satisfy (implementer states the rule in the selector comment) |
| Data flow | the issued id is declared once (registry entry) and read by the selector; the accountant does not keep a second copy for matching |
| Rule: principles (silent wrong value) | an unevaluable attribute is refused, never ignored; an absent registry service answers a NAK, never a match |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Exact, conjunctive session selection (S-1..S-3) | AC-1..AC-6 unit tests, both `.ci` tests, both interop scenarios with forced reds |
| Unknown CoS profile refused (T-1) | AC-7 tests |
| Atomic application (T-2) | AC-8 tests, after A-7 |
| Evidence | fresh discrimination records for every staled unit; independent re-judge of the five rows; extraction sign-off preserved |

### Security Review Checklist
| Check | What to look for |
|-------|------------------|
| Input validation | every identification attribute value is compared, never parsed into a lookup key that could select a different session (the prefix match was this defect) |
| Wrong-target action | no path tears down or changes a session the request's combination does not name |
| Information leak | 503 for a stale id is indistinguishable from "no such session"; a foreign NAS identity draws 403, which tells the sender only that it addressed the wrong NAS, information it supplied itself |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Test fails on behavior mismatch | Re-read the source in Current Behavior |
| A-2/A-3 broken (registry entry missing at issue time) | DESIGN: Q-2 (A) becomes the fallback design; ask the owner |
| A-7 not delivered | AC-8 stays open; the spec is not closed (no parking) |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Exact Acct-Session-Id match against the id Ze issued (S-1) | (C) keep the prefix and refuse only when a different id was issued for (T,S) | (C) still tears down a reconnected session whenever no id was recorded for it (accounting off, or a session alive across a reload), so it is not fully correct; rejected under `ai/rules/simplicity.md` (simplicity may not cut correctness) |
| One session table, the registry (Q-2 B) | (A) authradius issued-id table plus the L2TP snapshot | (B) fixes S-2 at its cause (a field holding a value no packet carries) and reaches PPPoE with the same code; (A) keeps two lookups |
| Unevaluable identification attributes refused 401 (Q-3 i) | 503; ignore | ignoring is the breach; 401 tells the DAC operator which attribute to stop sending |
| Rollback by compensation (Q-4 alpha) | one combined backend Apply | keeps the shaper and CoS plugins separate; the failure of the compensation is surfaced |

## Known Limitations
- A pure L2TP LNS session has no access interface, so a CoS CoA for it stays NAK 506 (`docs/architecture/traffic/cos-dynamic.md` lines 57-59).
- PPPoE Disconnect teardown (504 today) and PPPoE CoA delivery belong to `spec-lifecycle-invariants` (AC-3, I-4).
- With accounting not configured Ze issues no Acct-Session-Id, so a DAC must select by User-Name, NAS-Port or Framed-IP-Address (R-3).
- Termination-Action stays excluded unless Q-6 selects it.

## Design Review Log

| Date | Round | Outcome |
|------|-------|---------|
| 2026-10-14 | owner answers recorded | Q-1 answered through `spec-radius-rfc-defects` Q-6 ("whatever", recommendation taken: the stale-id prefix match stays here as S-1/AC-1); Q-7 answered through `spec-radius-rfc-defects` Q-7 ("Make Free radius, its own lab."): every FreeRADIUS-in-L2TP lab reference now names the shared `interop-radius-subscriber` lab (Required Reading, Interop Tests, Files to Modify, Files to Create, Documentation row 10, Implementation step 5); scenario names unchanged. Other questions unchanged. Review round owed |
| 2026-10-14 | consistency edit from `spec-radius-rfc-defects` review round 2 (its NIT-5) | Files to Modify and Documentation row 10 point at that spec's lab rows: the shared `/dev/ppp` move out of the L2TP and PPPoE labs, and the L2TP and PPPoE preflight doc text, owed by whichever spec builds the lab. No other change |

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

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Interop tests for protocol features (or N-A with a reason)

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] `./le verify worktree` passes
