# Spec: RFC 5176 residual CoA atomicity and session matching

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Depends | `plan/immediate/spec-lifecycle-invariants.md` (applied-result contract, AC-3/AC-4; gates AC-8 only); `spec-radius-rfc-defects` (closed 2026-10-20; its D2 replay key and D3 Acct-Session-Id format touched the same files, R-6) |
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
| Termination-Action State echo absent | excluded, owner kept the exclusion 2026-10-14 | `rfc/extraction/rfc5176.json` site 3.3:7 `feature-out-of-scope` (RFC 2865 Section 5.29 MAY); Q-6 |

### Defects this spec owns (all verified at the producer on 2026-10-13)

| ID | Producer | RFC text (read in `rfc/full/rfc5176.txt`) | What Ze does wrong |
|----|----------|--------------------------------------------|--------------------|
| S-1 | `coa.go::findSessions` | Section 3: "The combination of NAS and session identification attributes included in a CoA-Request or Disconnect-Request packet MUST match at least one session in order for a Request to be successful; otherwise a Disconnect-NAK or CoA-NAK MUST be sent." Section 3 table: Acct-Session-Id is "The identifier uniquely identifying a session on the NAS." | The Acct-Session-Id is matched on its "tunnelID-sessionID-" prefix only. A late Disconnect-Request carrying the id `acct.go::genSessionID` issued to a session that has ended tears down the current session that reuses the same tunnel and session numbers. Raised as Q-6 of `spec-radius-rfc-defects` (2026-10-13), which keeps the prefix and routes the fix here |
| S-2 | `coa.go::findSubscriberSession`, `handleCoA`, `handleDisconnect` | Section 3, first sentence above; Section 2.3: "If the CoA-Request is unsuccessful for any matching sessions, the NAS MUST send a CoA-NAK in reply, and the requested authorization changes MUST NOT be made for any of the matching sessions." | The registry is searched by `Session.AcctSessionID`, which for L2TP holds `l2tp-<T>-<S>` (`subscriber_bridge.go::onSessionUp` and `onSessionIPAssigned`) and for PPPoE `pppoe` ids (two assignments in `pppoe/subsystem.go`, grep `AcctSessionID`): values no RADIUS packet carries. A hit acts with no other identifier checked. A real DAC's id (`T-S-N`) misses the registry and falls to the L2TP-only path, which NAKs every CoS change 506 "no access interface" although the registry session has one, so CoS-by-CoA is unreachable for an L2TP session from a real DAC |
| S-3 | `coaSupportedAttrs`, `disconnectSupportedAttrs`, `findSessions` | Section 3, first sentence above; Section 2.3: "In CoA-Request and Disconnect-Request packets, all attributes MUST be treated as mandatory." | Eleven identification attributes are accepted and never evaluated: Framed-IP-Address, Called-Station-Id, Calling-Station-Id, Acct-Multi-Session-Id, NAS-Port-Id, Chargeable-User-Identity, Framed-Interface-Id, Framed-IPv6-Prefix, NAS-IP-Address, NAS-Identifier, NAS-IPv6-Address. A request whose combination names no session still acts on the session its Acct-Session-Id or User-Name names |
| T-1 | `readCoAChange` / `addFilterID` / `matchVendorCoS`; consumer `internal/plugins/cos/handler.go::onCoSChange` | Section 2.3: "If one or more authorization changes specified in a CoA-Request cannot be carried out, the NAS MUST send a CoA-NAK." Section 2.2: "The NAS responds to a CoA-Request sent by a Dynamic Authorization Client with a CoA-ACK if the NAS is able to successfully change the authorizations for the user session(s), or a CoA-NAK if the CoA- Request is unsuccessful." | A CoS profile name is parsed (`coreCos.ParseFilterID`) and never looked up, so `cos:no-such-profile` is ACKed; `onCoSChange` then logs "CoA profile not found" and returns. `onCoSChange` also stores the new `profileName` before `updateQoS` succeeds |
| T-2 | `applySubscriberCoA` | Section 2.3: "State changes resulting from a CoA-Request MUST be atomic: if the CoA-Request is successful for all matching sessions, the NAS MUST send a CoA-ACK in reply, and all requested authorization changes MUST be made." | Rate and CoS events are fire-and-forget (`events.Event.Emit`, handlers are `func(T)` with no result). When the rate applies and the CoS `updateQoS` fails, the rate stays changed and the DAC has already been ACKed. Needs the applied-result contract of `spec-lifecycle-invariants` AC-3/AC-4 |

### Owner questions (each states the reading the design proceeds under; Q-5 is answered by the owner on 2026-10-13, Q-1 and Q-7 on 2026-10-14 through `spec-radius-rfc-defects` Q-6 and Q-7, and Q-2, Q-3, Q-4, Q-6 and Q-8 on 2026-10-14, "Do what you think is right", which accepts every recommendation; no question is open)

| ID | Question | Design proceeds under | Why it needs the owner |
|----|----------|-----------------------|------------------------|
| Q-1 (S-1 scope) | Take the stale-Acct-Session-Id defect (`spec-radius-rfc-defects` Q-6) into this spec? | **Answered by the owner, 2026-10-14, through `spec-radius-rfc-defects` Q-6: "whatever"; the recommendation is taken: yes.** The stale Acct-Session-Id prefix-match defect is in this spec: AC-1 here; `spec-radius-rfc-defects` keeps D2/D3 and its prefix-preserving R-5, and this spec's exact match does not depend on the id format D3 chooses | Scope move between two specs; it changes which Disconnect tears down which session |
| Q-2 (S-1/S-2 design) | One session table: (B) the subscriber registry is the only table the CoA matcher reads, and `Session.AcctSessionID` is redefined as the Acct-Session-Id Ze put on the wire (written by the accountant when it issues one, empty when none was issued); or (A) an authradius-owned table of issued ids beside the L2TP snapshot, with the registry consulted only for the access interface | **Owner, 2026-10-14: accept (recommendation): (B).** The subscriber registry is the only table the matcher reads; `Session.AcctSessionID` becomes the Acct-Session-Id Ze put on the wire | (B) changes the meaning of a registry field and deletes the L2TP-only fallback (`ai/rules/no-layering.md`). It also changes what an operator sees: `show subscriber` prints the field as `acct-session-id` (`subscriber/cmd/subscriber.go`, printed only when non-empty), so an L2TP session shows the wire id (`T-S-N`) instead of `l2tp-T-S`, and the key is absent when accounting is off (no id issued) and for every PPPoE session (PPPoE stops writing it). (A) leaves a field holding a value no packet carries and two lookups that disagree (S-2), and leaves `show subscriber` as it is |
| Q-3 (S-3) | An identification attribute Ze holds no fact to compare (Called-Station-Id, Acct-Multi-Session-Id, Chargeable-User-Identity, NAS-IPv6-Address): (i) drop it from the accepted sets, so the request is NAKed 401; (ii) keep it accepted and let it match nothing, NAK 503 for the three session attributes and NAK 403 for NAS-IPv6-Address, which is a NAS identification attribute (Section 3.5 "NAS Identification Mismatch", AC-3); (iii) keep ignoring it | **Owner, 2026-10-14: accept (recommendation): (i).** The four types leave both accepted sets; a request carrying one is NAKed 401 | Changes which requests a deployed DAC gets answered. Ze sends none of the four in any Access-Request or Accounting-Request (`nasidentity.go::appendNASIdentity` emits NAS-IP-Address and NAS-Identifier only, and `source-address` is IPv4-only in `config.go`), so a DAC that copies accounting attributes never carries them. Per option: (i) the DAC gets 401 naming an attribute it can stop sending, and the request acts on nothing; (ii) the DAC gets "no such session" (or "wrong NAS") for a session or NAS that exists, a cause that sends its operator looking in the wrong place; (iii) is the current RFC 5176 Section 3 breach: the request acts on a session the whole combination may not name |
| Q-4 (T-2) | Atomic multi-change: (alpha) apply in order and, when a later part reports failure, re-apply the prior state of the parts already applied, then NAK 506; (beta) merge rate and CoS into one desired-state `traffic` backend Apply per interface | **Owner, 2026-10-14: accept (recommendation): (alpha)**, apply in order, restore the prior state on failure, NAK 506; started only after `spec-lifecycle-invariants` AC-4 gives a synchronous applied result | (beta) merges the shaper and CoS plugins' state; (alpha) can itself fail, and AC-8 then reports the breach rather than hiding it |
| Q-5 (tests) | Approve changing the RFC-tagged tests in "Tagged tests that change". Revised 2026-10-13 after the RFC-tagged test review: `TestRFC5176IdentificationAttributesIdentifyOnly` (RFC5176-3.6-1, both units now drop Called-Station-Id), `TestRFC5176DisconnectCarryingOnlyIdentificationIsACKed` (RFC5176-3-1 negative, prose narrows; its claim turns on Q-3), `TestRFC5176RequestNotRoutableCauseNeverSentForAForeignSession` (RFC5176-3.5-8, now a body change: matching NAS-Identifier, since AC-3 is now 403 per Section 3.5 "NAS Identification Mismatch"), the 2.3-6 premise changes, the 2.3-5/2.3-6 re-records, and the fixture-only units. `TestDisconnectReplayReturnsCachedResponse` is also edited by `spec-radius-rfc-defects` Q-3 | **Answered by the owner, 2026-10-13: "approve all RFC-tagged test changes".** It covers every unit in "Tagged tests that change", the behaviour edits and the fixture-only units alike. Q-2 (B) and Q-3 (i), answered 2026-10-14, fix the remaining forms: the fixture-only edits apply under Q-2 (B), the Called-Station-Id edits and the `TestRFC5176DisconnectCarryingOnlyIdentificationIsACKed` claim are written for Q-3 (i), and `TestRFC5176RequestNotRoutableCauseNeverSentForAForeignSession` takes the matching-NAS-Identifier form its row names. Commands under "Owner approval commands" in the TDD plan | `ai/rules/rule-precedence.md`: changing an RFC-tagged test needs the owner |
| Q-6 (AC-9) | Termination-Action (RFC 2865 Section 5.29, optional): select the reauthentication feature, or keep the recorded `feature-out-of-scope` exclusion | **Owner, 2026-10-14: accept (recommendation): keep the exclusion; nothing implemented** | Optional-feature scope (`ai/rules/rfc-compliance.md`); the skeleton already requires Thomas to decide before closure. Per option: "keep" adds no code and leaves site 3.3:7 `feature-out-of-scope`; "select" adds a new reauthentication Access-Request path (Termination-Action read from the Access-Accept, an Access-Request sent when the session's service terminates instead of ending it, State echoed), its own tests and an interop scenario against FreeRADIUS, and re-signs the extraction site |
| Q-7 (interop) | Interop needs FreeRADIUS `radclient` beside a live L2TP session, which no lab has. Where does FreeRADIUS run, shared with `spec-radius-rfc-defects` Q-7 and the planned `radius-coa-disconnect-freeradius` of `spec-rfc-verdict-fix-access`? | **Answered by the owner, 2026-10-14, through `spec-radius-rfc-defects` Q-7: "Make Free radius, its own lab."** FreeRADIUS gets its own lab, `interop-radius-subscriber` (scenarios under `test/interop-radius-subscriber/scenarios/`, plumbing in `internal/le/interoplab/radiussubscriber/`), shared with `spec-radius-rfc-defects`, which defines it under "The `interop-radius-subscriber` lab"; whichever spec lands first builds it. No FreeRADIUS container is added to the L2TP lab, whose RADIUS peer stays `radiusmock`. The two scenarios named here go into that lab | Lab work larger than the producer fixes; one lab for three specs is an ordering decision |
| Q-8 (PPPoE) | PPPoE sessions have no Ze-issued Acct-Session-Id (the only issuer, `acct.go`, subscribes to `l2tpevents` only). Should the matcher select PPPoE sessions by the other identifiers (User-Name, Framed-IP-Address) | **Owner, 2026-10-14: accept (recommendation): yes**, the same evaluator runs over PPPoE registry entries; a PPPoE Disconnect still answers 504 and PPPoE CoA delivery stays with `spec-lifecycle-invariants` AC-3 and I-4. Reconciled with Q-2 (B): the 503-for-every-PPPoE-request outcome belonged to "no" only. Under (B) PPPoE stops writing `AcctSessionID`, so an Acct-Session-Id never selects a PPPoE session, but PPPoE sessions ARE in the registry already (`pppoe/subsystem.go::subscriberSession` inserts the entry and sets `Username`; the IP-up path sets `IPv4Addr`), so User-Name and Framed-IP-Address select them. AC-12 states it; the TDD row it owes is listed in the Design Review Log row of 2026-10-14 (owner answers) | Today a PPPoE session is reachable only by its internal id, which no DAC can know; opening it by User-Name changes which requests act. Per option: "yes" makes a PPPoE session selectable by User-Name or Framed-IP-Address (a Disconnect then answers 504, a CoA follows lifecycle AC-3), and a User-Name shared by an L2TP and a PPPoE session answers 508 (AC-6). "no" with Q-2 (B) leaves no PPPoE session selectable at all, because PPPoE stops writing `AcctSessionID` and no other identifier is evaluated for it: every request naming a PPPoE session answers 503 |

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/rfc-conformance-gates.md` - the extraction sign-off, the tagged-test routes and the discrimination record
  → Constraint: every tagged unit whose body or producer changes stales its `rfc/discrimination/rfc5176.json` record; re-record with `./le rfc discriminate-record`, never by hand. A row quote stays a verbatim span of its section.
  → Constraint: the 2026-09-01 extraction sign-off is not re-signed by this spec unless a site's disposition changes; no new requirement rows (RFC scope frozen 2026-10-08), so S-1..S-3 are proven under the existing RFC5176-3.3-1, RFC5176-2.3-5, RFC5176-2.3-7 rows and T-1/T-2 under RFC5176-2.3-5 and RFC5176-2.3-6.
- [ ] `docs/guide/l2tp.md` CoA/DM listener section (the check table and the identification prose below it) - the operator page for the CoA/DM listener
  → Constraint: since `88d784795c` (2026-10-13) the page already states the current behaviour: the row reads "The identification attributes Ze evaluates match exactly one session (see below)" and the prose lists the three evaluated and eleven ignored attributes and the prefix match. Each phase that changes a behaviour rewrites that text in the same phase, not at closure (`ai/rules/documentation.md`).
  → Constraint: the row "The requested change reached the shaper | NAK 506" describes emit, not application; AC-8 changes it to "was applied".
- [ ] `docs/research/l2tpv2-ze-integration.md` - the `// Design:` document of `coa.go` and `acct.go`
  → Decision: its CoA and DM paragraphs set CoA rate changes and DM teardown for live sessions as the integration goal; nothing there fixes how a session is selected, so the selector design here adds no conflict.
- [ ] `docs/architecture/testing/interop.md` - suites, the FreeRADIUS admin-login lab, scenario naming
  → Constraint: scenario directories are named, never numbered; the FreeRADIUS suite (`internal/le/interoplab/radius/`) has no L2TP, and the L2TP lab's RADIUS peer is Ze's own `radiusmock`, so a FreeRADIUS-against-live-session scenario needs a new lab: `interop-radius-subscriber` (Q-7, owner 2026-10-14).
  → Constraint: every checker reads both sides (Ze log and the server's record), and a forced red is recorded by reverting the producer and rebuilding the image.
- [ ] `docs/architecture/l2tp/subscriber-session-model.md` - the registry's page; its source anchor names `LookupByAcctSessionID`
  → Constraint: Q-2 B changes what `Session.AcctSessionID` means and removes `LookupByAcctSessionID`; this page and its anchor change in the phase that changes the field (Documentation row 12).
- [ ] `docs/architecture/l2tp/bng-5-pppoe.md` - the PPPoE subsystem page, the `// Design:` of `pppoe/subsystem.go`
  → Constraint: it says nothing about Acct-Session-Id today (grep 2026-10-13), so the PPPoE change (stop writing the internal id) owes no edit there unless the implementer adds a sentence; Q-8 (yes, owner 2026-10-14) makes a PPPoE session selectable by User-Name or Framed-IP-Address, so the page owes a sentence saying so.
- [ ] `docs/architecture/traffic/cos-dynamic.md` - the CoS-by-CoA design page, `// Design:` of `internal/plugins/cos/handler.go`
  → Constraint: since `88d784795c` (2026-10-13) its Consequences section names `readCoAChange`, `setRate` and `setCoSProfile` and no doc names `extractRate`, so the page is current; the T-1 phase adds only the profile-lookup refusal (407) to that paragraph.
  → Decision: its Consequences paragraph on `AccessInterface`: `AccessInterface` is empty for a pure L2TP LNS, so AC-5 holds only for an L2TP session that carries an access interface; a pure-LNS CoS CoA stays NAK 506.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc5176.md` - the declared requirement ids for this stem
  → Constraint: rows used: RFC5176-3.3-1 (Section 3 "MUST match at least one session"), RFC5176-2.3-5 (attributes mandatory, NAK when a change cannot be carried out), RFC5176-2.3-6 (atomic), RFC5176-2.3-7 (508), RFC5176-3.6-1 (identification only), RFC5176-3-1 (a Disconnect-Request carrying other attributes is NAKed) and RFC5176-3.5-8 (a NAS never sends "Request Not Routable"); the last two hold in their `tests` maps `TestRFC5176DisconnectCarryingOnlyIdentificationIsACKed` and `TestRFC5176RequestNotRoutableCauseNeverSentForAForeignSession`, whose bodies change. All seven are `enforced` in `rfc/audit/rfc5176.json` at HEAD; a fix that changes their producers owes a re-judge by an agent that did not write the tests.
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
| `internal/component/l2tp/plugins/authradius/coa.go` | 916 | `handlePacket`: source allow-list, Request Authenticator, decode, code, Message-Authenticator, `cachedReplay`, Event-Timestamp, Service-Type 405, `unsupportedAttr` 401, `unsupportedAttrValue` 407, dispatch. `handleCoA`: `readCoAChange` first, then `findSubscriberSession` hit → `applySubscriberCoA`, else `oneSession` → L2TP-only rate emit (CoS refused 506). `applySubscriberCoA`: refusals before first emit, then subscriber rate, L2TP rate, CoS emits, ACK. `handleDisconnect`: registry PPPoE hit → 504; else `oneSession` → `TeardownSession`, 504 on failure. `findSubscriberSession`: `Registry.LookupByAcctSessionID` only. `oneSession`: 503 none, 508 several. `findSessions`: L2TP snapshot, AND of Acct-Session-Id prefix, User-Name, NAS-Port (= LocalSID). `coaListenerConfig` carries port, secrets, bus, allowed sources and the Message-Authenticator flag, no NAS identity; `newCoAListener` binds `udp4` on the wildcard address and `serve` reads with `ReadFromUDP`, so the destination address of a request is not known. `readCoAChange`: one rate, one CoS profile, 407 otherwise, 401 for no change |
| `internal/component/l2tp/plugins/authradius/acct.go` | ~600 | `genSessionID` `T-S-N`, N from `nextSess`; `onSessionIPAssigned` issues once per session when `accepting` and a client exist, stores `acctSession{acctSessID, callingStationID, nasPortID, username, ...}` in `a.sessions`; `onSessionDown` deletes it; `detach` moves every live record to a retired accountant on reload, so the active accountant no longer knows ids issued before a reload |
| `internal/component/l2tp/subscriber/session.go`, `registry.go` | - | `Session` has ID, AccessType, Username, AccessInterface, MAC, TunnelID, SessionID, IPv4Addr, IPv6InterfaceID, IPv6Prefix, AcctSessionID; no Calling-Station-Id, no NAS-Port-Id. `LookupByAcctSessionID` is a linear scan returning the first hit; its only non-test caller is `coa.go` |
| `internal/component/l2tp/subscriber_bridge.go` | - | `onSessionUp` and `onSessionIPAssigned` set `AcctSessionID = l2tpSessionID(T,S)` = `l2tp-T-S`; `onSessionUp`, `onAuthResult` and `onSessionIPAssigned` each read a whole copy with `Registry.Get`, change it, and write it back with `Registry.Add` (which replaces the whole stored copy), so a field another writer set between the two calls is overwritten; `onSessionUp` reads `wasActive` from that copy and emits it as the SessionUp payload |
| `internal/component/l2tp/subscriber/cmd/subscriber.go` | - | the `show subscriber` handler prints `Session.AcctSessionID` under the key `acct-session-id` when it is non-empty |
| `internal/component/l2tp/plugins/authradius/nasidentity.go` | - | `appendNASIdentity(attrs, nasID, sourceAddr)`: NAS-IP-Address when `source-address` is set (IPv4 only, `config.go` refuses IPv6); NAS-Identifier when `nas-identifier` is set; the host name (`hostNASIdentifier`) as NAS-Identifier only when neither is set. Never NAS-IPv6-Address. Called by `handler.go::buildAuthAttrs` and `acct.go::buildAcctPacket` |
| `internal/component/l2tp/pppoe/subsystem.go` | - | `subscriberSession` and `onSessionUp` set `AcctSessionID` to the internal PPPoE id |
| `internal/component/l2tp/plugins/shaper/shaper.go` | - | `onSessionRateChange`: unknown session → warn and return; `applyTC` error → warn and return; no result to the caller. Engine subscribers run synchronously inside `Emit` (`internal/core/events/typed.go::Event.Emit`) |
| `internal/plugins/cos/handler.go` | - | `onCoSChange`: empty interface → return; `coreCos.Lookup` miss → warn and return; stores `profileName` before `updateQoS`; `updateQoS` error → warn and return |
| `internal/core/cos/cos.go` | - | `ParseFilterID` strips the `cos:` prefix only; `Lookup` answers whether a profile is registered |
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
- `coaListener.serve` → `handlePacket(data, *net.UDPAddr)`; the listener exists only when `coa-port` is set and a `server` address resolves (`register.go::startCoAListener`), and `activateRadiusConfig` closes and restarts it on every applied configuration
- the issued id enters at `radiusAcct.onSessionIPAssigned` (`l2tpevents.SessionIPAssigned`) and leaves at `onSessionDown`

### Transformation Path
1. `handlePacket` authenticates, dedups, checks Event-Timestamp, Service-Type and the accepted attribute set (unchanged except Q-3 removes the unevaluable identification attributes from both sets).
2. CoA only: `readCoAChange` reads the change; T-1 adds the CoS profile registry lookup here, before any session is touched.
3. New: one selector evaluates the NAS identification attributes against the identity Ze itself sends the server: the NAS-IP-Address and NAS-Identifier that `appendNASIdentity` emits for the configured `nas-identifier` and `source-address` (AC-3). `startCoAListener` passes those two configured values into `coaListenerConfig`; the selector derives the identity from `appendNASIdentity`, so the identity Ze sends and the identity it compares are declared once. The selector runs inside `handleCoA` and `handleDisconnect`, so `handlePacket` and `serve` do not change. It then filters the registry's sessions by every supplied session identification attribute. Result: zero (503), several (508) or one session.
4. CoA: `applySubscriberCoA` on the one session (refusals first, then emits). Under AC-8, each emit returns the applied result and a failure triggers rollback of the parts already applied.
5. Disconnect: L2TP session → `l2tp.Service.TeardownSession(SessionID)`; PPPoE session → 504 as today (lifecycle I-4 owns PPPoE teardown).
6. `sendResponse` echoes Proxy-State/State and remembers the answer for duplicates (unchanged).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CoA listener ↔ subscriber registry | `subscriber.LookupService().Registry` read (`All` or a new filtered read) | Yes, `coa.go::findSubscriberSession` |
| Accountant ↔ subscriber registry | new: the accountant writes the issued Acct-Session-Id onto the registry entry for (T,S) and clears it at session down (Q-2 B), through the atomic registry upsert (R-9) | No, assumption A-2 |
| CoA listener ↔ RADIUS configuration | new: `startCoAListener` copies `nas-identifier` and `source-address` from `radiusConfig` into `coaListenerConfig` | Yes, `register.go::startCoAListener`, `activateRadiusConfig` |
| CoA listener ↔ shaper / CoS plugins | typed events on the bus, engine subscribers run synchronously | Yes, `typed.go::Event.Emit` |
| CoA listener ↔ L2TP service | `l2tp.LookupService().TeardownSession` | Yes, `coa.go::handleDisconnect` |

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
| A-2 | The registry holds an entry for every live L2TP session before its Acct-Session-Id is issued, so the accountant can write onto it | `subscriber_bridge.go` creates the entry at session up (`onSessionUp`) and at IP assignment (`onSessionIPAssigned`); the accountant issues at IP assignment | the write finds no entry and the session is unreachable by its id | unit test ordering both events both ways; grep event subscription order | unvalidated |
| A-3 | `subscriber_bridge.go` no longer overwrites `AcctSessionID` once the field means the wire id | `onSessionUp` and `onSessionIPAssigned` assign it today | a later SessionUp erases the issued id | the bridge stops writing the field; a unit test emits SessionUp after IP-assigned | unvalidated |
| A-4 | The only readers of `Session.AcctSessionID` are `coa.go` (matching) and `subscriber/cmd/subscriber.go` (the `show subscriber` key `acct-session-id`), and `LookupByAcctSessionID` has no non-test caller but `coa.go` | grep 2026-10-14; the 2026-10-13 grep missed `subscriber.go` (design review r1). `subscriber.go` stays a reader and changes what it prints (Q-2 consequences) | redefining the field breaks a reader | grep at implementation, including web views | unvalidated |
| A-5 | The NAS-Port Ze sends for an L2TP session is its LocalSID | `handler.go::buildAuthAttrs` and `acct.go::buildAcctPacket` both send `uint32(sessionID)` | NAS-Port evaluation compares the wrong value | read both producers at implementation | unvalidated |
| A-6 | The NAS-Port-Id Ze sent can be recomputed at match time from `resolveNASPortID(format, nasID, T, S)` | `acct.go::onSessionIPAssigned` computes it from config and (T,S) only | a reload that changes the format makes the old value unrecoverable | store the resolved value on the registry entry with the issued id instead, if the format is reloadable | unvalidated |
| A-7 | `spec-lifecycle-invariants` AC-4 delivers a synchronous applied result to the CoA caller | that spec's AC-3/AC-4 and its boundary paragraph | AC-8 cannot be built; T-2 stays open | read that spec's design at implementation start; AC-8 phase is last | unvalidated |
| A-8 | A real DAC (FreeRADIUS `radclient`, `radiusd` originate) sends only attributes Ze can evaluate when it copies from accounting | Ze's Accounting-Request carries Acct-Session-Id, User-Name, NAS-Port, NAS-Port-Id, Calling-Station-Id, Framed-IP-Address, NAS-IP-Address/NAS-Identifier (`acct.go::buildAcctPacket`, `nasidentity.go::appendNASIdentity`) | Q-3 (i) refuses common requests | interop scenarios send the copied set | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Making the change set atomic needs a rollback the shaper events cannot give | an emit that cannot be undone | AC-8 waits for the applied-result contract (A-7); rollback re-applies the prior rate held in shaper state; a failed rollback is NAKed 506 and logged at error with the RFC section, never ACKed |
| R-2 | Sessions alive across an accounting reload lose their issued id (`detach` moves the records to a retired accountant) | a valid Disconnect after a reload answers 503 | under Q-2 B the id lives on the registry entry, not in the accountant, so a reload does not clear it; unit test reloads between issue and Disconnect |
| R-3 | No accounting configured means no id is issued, so a DAC that addresses by Acct-Session-Id gets 503 | Disconnect with an id answered 503 on a NAS without accounting | correct per Section 3 (the NAS issued no such id); documented in `docs/guide/l2tp.md`; User-Name and NAS-Port still select |
| R-4 | Q-3 (i) refuses a deployed DAC that sends Called-Station-Id or CUI | NAK 401 in an operator log | A-8 interop evidence; the guide lists the evaluated attributes; the owner chose (i) on 2026-10-14 |
| R-5 | NAS identity check refuses a DAC that sends a NAS-IP-Address it learned from the packet source while Ze has no `source-address` configured, or a NAS-Identifier while Ze sends none (`source-address` set, `nas-identifier` unset) | NAK 403 with a correct session id | Ze compares only against the identity it sends (AC-3); a DAC copying Ze's accounting attributes carries exactly that identity (A-8). The guide names the fix: set `source-address` or `nas-identifier` to the value the DAC sends. The arrival address is not used: the listener binds the wildcard and reads with `ReadFromUDP`, so the destination is unknown, and a PKTINFO path would be machinery for an identity Ze never announced |
| R-6 | `spec-radius-rfc-defects` edits the same functions: `coa.go::findSessions` (its comment; this spec deletes the function), `acct.go::genSessionID` (that spec rewrites the body; this spec edits the comment), `coa.go::handlePacket` (that spec's D2 replay key; this spec does not edit `handlePacket`, AC-3) | merge conflict or a test asserting the prefix | land order: radius-rfc-defects first, recorded in both specs. The "tunnelID-sessionID-" prefix contract that spec preserves (its R-5, AC-3c, `TestRFC2866AcctSessionIDKeepsTunnelSessionPrefix`, "Behavior to preserve") ends with S-1 here, because the exact match reads no prefix. Before the first edit the implementer reads that unit: a body that calls `findSessions` changes here and, if tagged, needs its own approval command |
| R-9 | The bridge's `onSessionUp`, `onAuthResult` and `onSessionIPAssigned` each read a whole copy (`Registry.Get`), change it and write it back (`Registry.Add`, which replaces the stored copy); when the accountant writes the issued id between the two calls, the bridge's write erases it. A-3 stops `onSessionUp` writing `AcctSessionID` and does not close this window | a session issued an id answers 503 to its own id | the registry gains one atomic upsert (under the write lock, when its insert flag is set, it inserts an entry carrying only the session id when none exists, and with the flag clear it changes nothing on a missing id; it applies the change to the stored entry, and reports whether the entry existed before); the accountant writes the id through it, and all three bridge paths move onto it with no separate `Get` or `Add`, so no writer replaces a field it did not change and two first writers cannot overwrite each other (`Registry.Add`, which replaces the whole stored copy, is no longer called by the bridge). Session-up reads `wasActive` inside the upsert from the stored entry BEFORE its change, false for an entry the upsert inserted (after the change, `State = StateActive` makes it always true and `subscriber.RecordSessionUp` would never fire, where HEAD reads it from the pre-change copy), and takes its SessionUp payload from the entry as the upsert left it. The SessionUp emit and `RecordSessionUp` run after the upsert returns, never inside the callback: engine subscribers fire synchronously from `deliverEvent` (`internal/component/plugin/server/dispatch.go`, which calls `dispatchEngineEvent` in `engine_event.go`) and `Registry.mu` is a non-reentrant `sync.RWMutex`. Unit tests: `TestBridgeUpdateKeepsAcctSessionID` (bridge, both orders, `RecordSessionUp` once) and `TestRegistryUpsertKeepsFieldsItDidNotChange` (the upsert alone). The upsert takes an insert flag: every bridge path and the accountant's id write at session start pass insert=true; the accountant's clear at session down passes insert=false, so after the bridge's `onSessionDown` has called `Registry.Remove` the clear finds no entry, changes nothing and recreates nothing (main-thread decision, round 4). A third unit case asserts a clear after Remove leaves the registry without the session. Out of order delivery is pre-existing and not fixed here: `reactor_kernel.go::handleSessionIPAssigned` emits SessionIPAssigned after releasing `tunnelsMu`, while `teardown.go::teardownSessionByID` emits SessionDown from the timeout goroutines (`session_timeout.go`) and from `subsystem_snapshot.go::TeardownSession` (CLI, CoA Disconnect), so SessionDown can be delivered first. At HEAD the bridge's `onSessionIPAssigned` then misses on `Get` and `Add`s a ghost entry, and `acct.go::onSessionDown` finds no record and returns, so the later `onSessionIPAssigned` stores one and sends Acct-Start with no Stop. The insert=true writes do not widen that race, because the ghost entry exists at HEAD. This spec's goal (no issued id lost to a read-modify-write) does not depend on the ordering, so the race is recorded in `plan/journal/late-write-lands-on-the-successor.md` (2026-10-14) and not fixed (main-thread decision, round 5) |
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
| a rate CoA-Request, then a Disconnect-Request, on UDP `coa-port` carrying the live session's captured Acct-Session-Id `T-S-N` with a wrong last field (`T-S-<other>`), an id Ze never issued | → | the new selector replacing `findSessions` | `test/l2tp/radius-coa-stale-acct-session-id.ci` (daemon, live session, real socket) |
| a CoA-Request on UDP `coa-port` carrying the wire Acct-Session-Id of an L2TP session and a CoS profile | → | selector → `applySubscriberCoA` | `TestRFC5176CoSChangeReachesAnL2TPSessionFoundByItsAcctSessionID` (real listener socket via `newCoAListener`) |
| a CoA-Request carrying a rate change and a CoS change where the second fails to apply | → | `applySubscriberCoA` rollback | `TestRFC5176FailedCoSApplicationRestoresTheRate` (gated on A-7) |

## Acceptance Criteria

Skeleton mapping: old AC-1 is AC-8, old AC-2 is AC-2 and AC-6, old AC-3 is AC-9, old AC-4 is AC-10.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | (a) unit, fake services: session (T,S) issued id X, went down; a new session reuses (T,S) and is issued id Y. (b) daemon and interop: one live session issued `T-S-N`; the request carries `T-S-<other>`, the same tunnel and session numbers with a last field Ze never issued. The real entry point cannot make a new session reuse (T,S) (`reactor.go::allocateLocalTID` counts, `session.go::allocateSessionID` is random), and an id with another (T,S) already draws 503 at HEAD, so (b) is the form that is red at HEAD. A CoA-Request carrying a rate, then a Disconnect-Request, each carrying X (a) or `T-S-<other>` (b). The CoA goes first in both halves: a Disconnect that tears the session down leaves a later CoA nothing to find | CoA-NAK and Disconnect-NAK, each with Error-Cause 503; zero `TeardownSession` calls, zero bus events, and the session survives. At HEAD `findSubscriberSession` misses (the registry holds `l2tp-T-S`) and `oneSession` / `findSessions` hits on the `T-S-` prefix, so HEAD ACKs the CoA with one rate event, then ACKs the Disconnect and tears the session down. The same two requests, in the same order, carrying the issued id (Y, or `T-S-N`): CoA-ACK with one rate event, then Disconnect-ACK with one teardown of (T,S) |
| AC-2 | A request carrying the current session's Acct-Session-Id plus ONE of User-Name, NAS-Port, Calling-Station-Id, NAS-Port-Id (both request codes), or Framed-IP-Address, Framed-IPv6-Prefix with Framed-Interface-Id (CoA-Request only), whose value belongs to another session or to none | NAK 503 and no effect, for each attribute in turn and for each request code it is listed under. With that attribute's value equal to the session's: ACK and the change made. Each attribute alone (no Acct-Session-Id) also selects its session. A Disconnect-Request carrying Framed-IP-Address, Framed-IPv6-Prefix or Framed-Interface-Id answers Disconnect-NAK 401 whatever its value, as today (`disconnectSupportedAttrs` leaves them out): the RFC 5176 Section 3.6 Disconnect table gives each of types 8, 96 and 97 the Request entry "0", and its legend reads "0    This attribute MUST NOT be present in packet." |
| AC-3 | A request whose NAS-IP-Address or NAS-Identifier is not the identity Ze sends the server, derived from `appendNASIdentity`: NAS-IP-Address equals the configured `source-address`; NAS-Identifier equals the configured `nas-identifier`, or the host name when neither leaf is set. A NAS identification attribute of a kind Ze sends none of (NAS-IP-Address with no `source-address`; NAS-Identifier with `source-address` set and `nas-identifier` unset) cannot match Ze's identity and is the same mismatch. NAS-IPv6-Address, which Ze never sends, is AC-4 (Q-3) | NAK 403, no effect. Equal values: the request proceeds to session selection. A request carrying no NAS identification attribute proceeds too: Section 3 says they "SHOULD be present", and "the NAS-Identifier Attribute MAY be present". Revised from 503 on 2026-10-13 (RFC-tagged test review, checked against `rfc/full/rfc5176.txt`): Section 3.5 assigns this case its own cause, ""NAS Identification Mismatch" is a fatal error sent if one or more NAS identification attributes (see Section 3) do not match the identity of the NAS receiving the Request.", while 503 is for ""Session Context Not Found" is a fatal error sent if the session context identified in the CoA-Request or Disconnect-Request does not exist on the NAS." |
| AC-4 | A request carrying an identification attribute Ze holds no fact for (Called-Station-Id, Acct-Multi-Session-Id, Chargeable-User-Identity, NAS-IPv6-Address) | NAK 401, no effect, for both request codes (Q-3 (i), owner 2026-10-14). Every type in either accepted set is either evaluated by the selector or not accepted, checked by a unit test over the two sets |
| AC-5 | A CoA-Request for an L2TP session with an access interface, selected by its wire Acct-Session-Id, asking for a known CoS profile | CoA-ACK and one `l2tp/session-cos-change` event for that interface (today: NAK 506) |
| AC-6 | The supplied identifiers match two sessions: two L2TP sessions with one User-Name; and an L2TP and a PPPoE session with one User-Name (Q-8 yes, owner 2026-10-14) | CoA-NAK / Disconnect-NAK with Error-Cause 508, no session changed. The two-L2TP case is preserved behaviour; the L2TP-plus-PPPoE case is new (at HEAD no PPPoE session can be matched by User-Name) |
| AC-7 | A CoA-Request whose CoS profile (`cos:<name>` Filter-Id or a vendor CoS VSA) is not registered in `internal/core/cos` | CoA-NAK with Error-Cause 407, zero bus events, also when a valid rate rides beside it. A registered profile is ACKed. `onCoSChange` stores the profile name only after `updateQoS` succeeds |
| AC-8 | (gated on A-7) A CoA-Request carrying a rate and a CoS profile; the rate applies and the CoS application reports failure | the session's previous rate is re-applied, CoA-NAK with Error-Cause 506, and no requested change remains. When the re-apply itself fails: CoA-NAK 506 and an error log naming the session and RFC 5176 Section 2.3; never a CoA-ACK |
| AC-9 | Termination-Action (Q-6: owner kept the exclusion, 2026-10-14) | The recorded `feature-out-of-scope` exclusion of site 3.3:7 remains explicit; no reauthentication code is added and no implementation or conformance completion is inferred from the absence of attribute 29 |
| AC-10 | The residual behaviours and affected RFC 5176 evidence are checked | Selector and atomicity tests prove both success and refusal, every staled tagged unit carries a fresh discrimination record, RFC5176-3.3-1, -2.3-5, -2.3-6, -2.3-7, -3.6-1, -3-1 and -3.5-8 are re-judged by an agent that did not write the tests, and the extraction stays signed; the 2026-09-01 signature is not counted as new work |
| AC-11 | `docs/guide/l2tp.md` CoA/DM section after each phase | the behaviour text changes with the phase that changes it: the identification prose and check table name every evaluated attribute, the whole-id Acct-Session-Id match, the 403 for a NAS identity other than the one Ze sends (with the `source-address` / `nas-identifier` fix), and the 401 for unevaluable ones; the 407 row covers an unknown CoS profile; the 506 row reads "was applied" (T-2); the source anchors name the new selector. `docs/architecture/traffic/cos-dynamic.md` gains the 407 for an unknown profile. Neither page needs a correction of today's text, which `88d784795c` already made |
| AC-12 | (Q-8 yes, owner 2026-10-14) A request naming one live PPPoE session by User-Name or by Framed-IP-Address (CoA-Request only for Framed-IP-Address, per AC-2), and no Acct-Session-Id | The selector finds the PPPoE registry entry. A Disconnect-Request answers Disconnect-NAK 504 as today (teardown stays with `spec-lifecycle-invariants` I-4); a CoA-Request follows `spec-lifecycle-invariants` AC-3. A request carrying an Acct-Session-Id never selects a PPPoE session, because under Q-2 (B) none is issued for one: 503 |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | an operator's RADIUS server sends a Disconnect-Request carrying an Acct-Session-Id Ze did not issue to the live session (a late request for an earlier session, or a wrong id) | UDP 3799 → `handlePacket` → selector (registry, exact Acct-Session-Id) → 503 | `test/l2tp/radius-coa-stale-acct-session-id.ci`; interop `l2tp-radius-coa-stale-session-freeradius` |
| 2 | a DAC changes a subscriber's rate and CoS profile in one request | UDP 3799 → `readCoAChange` (profile looked up) → selector → `applySubscriberCoA` → shaper and CoS consumers | `TestRFC5176CoSChangeReachesAnL2TPSessionFoundByItsAcctSessionID`; interop `l2tp-radius-coa-identification-freeradius` |
| 3 | a DAC addresses a subscriber by User-Name and Framed-IP-Address that disagree | UDP 3799 → selector → 503 | `TestRFC5176EveryIdentificationAttributeNarrowsTheMatch`; interop `l2tp-radius-coa-identification-freeradius` |

## 🧪 TDD Test Plan

### Unit Tests (new; file `internal/component/l2tp/plugins/authradius/rfc5176_session_selector_test.go` unless named)
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC5176StaleAcctSessionIDMatchesNoSession` | selector file | AC-1 negative, RFC5176-3.3-1, both forms: the reused-(T,S) id X and the wrong-suffix `T-S-<other>`. Red at HEAD (prefix match tears down) | |
| `TestRFC5176CurrentAcctSessionIDMatchesItsSession` | selector file | AC-1 positive | |
| `TestRFC5176EveryIdentificationAttributeNarrowsTheMatch` | selector file | AC-2, table over the six attributes, both polarities: four attributes under both codes, Framed-IP-Address and the IPv6 pair under CoA-Request only, plus the Disconnect-Request rows for types 8, 96 and 97 asserting 401. Red at HEAD for Framed-IP-Address, Calling-Station-Id, NAS-Port-Id, IPv6 pair | |
| `TestRFC5176ForeignNASIdentityIsAMismatch` | selector file | AC-3, over the configurations `appendNASIdentity` distinguishes (`source-address` only, `nas-identifier` only, both, neither): a foreign NAS-IP-Address or NAS-Identifier, and one of a kind Ze sends none of, draw NAK with Error-Cause 403, never 503; the values Ze sends proceed; no NAS identification attribute proceeds | |
| `TestRFC5176UnevaluableIdentificationAttributeIsUnsupported` | selector file | AC-4 (Q-3 i), RFC5176-2.3-5 | |
| `TestCoAAcceptedIdentificationTypesAreAllEvaluated` | selector file | AC-4 drift guard: every identification type in `coaSupportedAttrs` and `disconnectSupportedAttrs` has an evaluator | |
| `TestRFC5176CoSChangeReachesAnL2TPSessionFoundByItsAcctSessionID` | selector file | AC-5. Red at HEAD (506) | |
| `TestRFC5176MultipleMatchAcrossAccessTypesIsNAKed` | selector file | AC-6, RFC5176-2.3-7 | |
| `TestRFC5176UnknownCoSProfileIsNAKed` | `rfc5176_coa_values_test.go` | AC-7, RFC5176-2.3-5. Red at HEAD (ACK) | |
| `TestCoSChangeRecordsProfileOnlyAfterApply` | `internal/plugins/cos/handler_test.go` | AC-7 consumer half | |
| `TestRFC5176FailedCoSApplicationRestoresTheRate` | `rfc5176_coa_values_test.go` | AC-8 (gated on A-7), RFC5176-2.3-6 | |
| `TestRFC5176FailedRollbackIsNAKedAndLogged` | `rfc5176_coa_values_test.go` | AC-8 rollback failure | |
| `TestAcctIssuedSessionIDIsRecordedOnTheSubscriberSession` | `internal/component/l2tp/plugins/authradius/acct_session_id_registry_test.go` | Q-2 B: written at issue, cleared at down, survives `detach` (R-2), not overwritten by a later SessionUp (A-2, A-3) | |
| `TestBridgeUpdateKeepsAcctSessionID` | `internal/component/l2tp/subscriber_bridge_test.go` (package `l2tp`) | R-9: each of the three bridge paths (`onSessionUp`, `onAuthResult`, `onSessionIPAssigned`) interleaved with the accountant's write of the issued id, in both orders (bridge first, accountant first), keeps the id and the bridge's own fields; `subscriber.RecordSessionUp` fires exactly once, on the first up, whether that up inserted the entry or found one | |
| `TestRegistryUpsertKeepsFieldsItDidNotChange` | `internal/component/l2tp/subscriber/registry_test.go` (package `subscriber`) | R-9: the atomic upsert changes only the fields its change sets and leaves every other stored field; on a missing id with insert=true it inserts the entry and reports it did not exist; on an existing id it reports it existed; with insert=false a clear after `Remove` changes nothing and leaves the registry without the session | |

### Tagged tests that change (each needs the owner's approval, Q-5)

Behaviour or body changes:

| Test | File | Tag (verdict) | Change | Why |
|------|------|---------------|--------|-----|
| `TestRFC5176IdentificationAttributesIdentifyOnly` (both units) | `rfc5176_walk_test.go` | RFC5176-3.6-1 (`enforced`) | positive request drops Called-Station-Id and sets NAS-Identifier to the listener's identity; the NEGATIVE ("bare": User-Name, NAS-Port, Called-Station-Id, expecting 401) also drops Called-Station-Id, so its 401 comes from what the unit claims and not from Called-Station-Id alone | under Q-3 (i) Called-Station-Id is NAKed 401 on its own, so a negative that keeps it passes for the wrong reason (UNDER-TEST, isolation; confirmed in the body); under AC-3 a foreign NAS-Identifier is 403 |
| `TestRFC5176DisconnectCarryingOnlyIdentificationIsACKed` | `rfc5176_section_rows_test.go` | RFC5176-3-1 NEGATIVE (`enforced`; earlier drafts of this table said positive, the tag line says negative) | drops Called-Station-Id; Calling-Station-Id, NAS-Identifier and NAS-IP-Address take the session's and the NAS's values; the tag prose that lists Called-Station-Id among the session identification attributes is rewritten to name only "the identification attributes Ze evaluates" | Depends on Q-3: Called-Station-Id is a Section 3 session identification attribute, and under Q-3 (i) a Disconnect carrying it is NAKed 401 "Unsupported Attribute". That is permitted, because Ze does not support it, but the negative then proves only the attributes Ze evaluates; with Q-3 (i) answered (owner 2026-10-14) the claim names only the attributes Ze evaluates |
| `TestRFC5176SubscriberCoAThatCannotBeCarriedOutWholeMakesNoChange`, `TestRFC5176RequestWhoseEveryValueIsSupportedIsACKed` | `rfc5176_coa_values_test.go` | RFC5176-2.3-6, -2.3-5 (`enforced`) | helper `subscriberSession` unchanged; assertions unchanged; producer changes stale the records | re-record only |
| `TestRFC5176CoANAKMakesNoPartialChange` | `rfc5176_coa_branches_test.go` | RFC5176-2.3-6 (`enforced`) | its premise "L2TP-only session, CoS refused 506" becomes "registry session with no access interface" | the L2TP-only branch is deleted (no-layering) |
| `TestRFC5176ChangeThatCannotBeCarriedOutIsNAKed` (both units) | `rfc5176_walk_test.go` | RFC5176-2.3-6 (`enforced`) | same premise change if it relies on the L2TP-only branch; the implementer confirms by reading the body before the first edit | same |
| `TestRFC5176RequestNotRoutableCauseNeverSentForAForeignSession` | `rfc5176_section_rows_test.go` | RFC5176-3.5-8 negative | the request's NAS-Identifier changes from "other-nas" to the listener's identity, so its 503 comes from the unknown user `bob@elsewhere.example` (Session Context Not Found) and the assertion [503] stays; the foreign-NAS case is carried by `TestRFC5176ForeignNASIdentityIsAMismatch` (403) | WRONG after the change otherwise (confirmed: the body sends NAS-Identifier "other-nas" and asserts [503]); under AC-3 that request draws 403. Body change, so it needs Q-5 approval |

Fixture-only changes (Q-2 B: the selector reads the registry, so every test that publishes a fake L2TP service and sends `10-20-1` with no registry entry needs one). The edit is confined to the shared helpers (`fakeL2TPService` in `rfc5176_coa_listener_test.go`, `twoAliceSessions` in `rfc5176_coa_branches_test.go`, `twoSessionService` and `newWindowListener` in `rfc5176_section_rows_test.go`, `oneSessionService` and `coaTestListener` in `rfc5176_walk_test.go`, and the fake built in `rfc5176_coa_test.go` and `disconnect_service_stopped_test.go`), which also seed the registry; the test bodies do not change. Units affected, by file (from a scan of 2026-10-13 for units sending a `T-S[-N]` Acct-Session-Id; most carry an `RFC requirement:` tag, and the implementer re-runs the scan, drops untagged units from the approval list and adds any tagged unit the scan missed before the first edit):

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

58 commands. Each is `./le rfc approve unit authradius.<Unit> reason '<Reason>'`, where `<Reason>` is `Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-rfc5176-remaining-defect-classes, <Detail>)` with the row's Detail. The owner's quote stays verbatim in every reason.

| # | Unit | Detail |
|---|------|--------|
| 1 | `TestRFC5176IdentificationAttributesIdentifyOnly` | both polarities drop Called-Station-Id, NAS-Identifier set to the listener |
| 2 | `TestRFC5176DisconnectCarryingOnlyIdentificationIsACKed` | drops Called-Station-Id, prose names only attributes Ze evaluates |
| 3 | `TestRFC5176SubscriberCoAThatCannotBeCarriedOutWholeMakesNoChange` | producer change, re-record only |
| 4 | `TestRFC5176RequestWhoseEveryValueIsSupportedIsACKed` | producer change, re-record only |
| 5 | `TestRFC5176CoANAKMakesNoPartialChange` | premise becomes registry session with no access interface |
| 6 | `TestRFC5176ChangeThatCannotBeCarriedOutIsNAKed` | premise change if it relies on the L2TP-only branch |
| 7 | `TestRFC5176RequestNotRoutableCauseNeverSentForAForeignSession` | NAS-Identifier becomes the listener identity, or keep other-nas and assert 403 |
| 8 | `TestRFC5176CoAWithAnUnsupportedAttributeValueIsNAKed` | fixture-only, shared helpers seed the registry |
| 9 | `TestRFC5176DisconnectThatCannotTerminateIsNAKed` | fixture-only, shared helpers seed the registry |
| 10 | `TestRFC5176DisconnectThatTerminatesIsACKed` | fixture-only, shared helpers seed the registry |
| 11 | `TestRFC5176MatchedIdentificationSucceeds` | fixture-only, shared helpers seed the registry |
| 12 | `TestRFC5176NAKForARequestWithoutStateCarriesNone` | fixture-only, shared helpers seed the registry |
| 13 | `TestRFC5176ResponseAuthenticatorMatchesAnIndependentMD5` | fixture-only, shared helpers seed the registry |
| 14 | `TestRFC5176UnmatchedIdentificationIsNAKed` | fixture-only, shared helpers seed the registry |
| 15 | `TestRFC5176CoAMatchingOneSessionIsACKed` | fixture-only, shared helpers seed the registry |
| 16 | `TestRFC5176CoAMatchingSeveralSessionsIsNAKed` | fixture-only, shared helpers seed the registry |
| 17 | `TestCoAListenerInvalidAuth` | fixture-only, shared helpers seed the registry |
| 18 | `TestCoAListenerMissingMessageAuthenticatorDroppedWhenRequired` | fixture-only, shared helpers seed the registry |
| 19 | `TestCoAListenerUnknownSession` | fixture-only, shared helpers seed the registry |
| 20 | `TestDisconnectReplayReturnsCachedResponse` | fixture-only, shared helpers seed the registry |
| 21 | `TestCoASourceFilterAnswersAConfiguredClient` | fixture-only, shared helpers seed the registry |
| 22 | `TestCoASourceFilterDiscardsWhenNoServerResolved` | fixture-only, shared helpers seed the registry |
| 23 | `TestRFC5176ResponseAuthenticator` | fixture-only, shared helpers seed the registry |
| 24 | `TestRFC5176NoSessionIdNotActedOn` | fixture-only, shared helpers seed the registry |
| 25 | `TestRFC5176CoAWithOneUnsupportedValueBesideSupportedOnesIsNAKed` | fixture-only, shared helpers seed the registry |
| 26 | `TestRFC5176DisconnectWithAnUnsupportedValueIsNAKed` | fixture-only, shared helpers seed the registry |
| 27 | `TestRFC5176MessageAuthenticatorAbsentIsAcceptedByDefault` | fixture-only, shared helpers seed the registry |
| 28 | `TestRFC5176WrongMessageAuthenticatorDiscardedWhenNotRequired` | fixture-only, shared helpers seed the registry |
| 29 | `TestRFC5176ListenerAcceptsConformantMessageAuthenticator` | fixture-only, shared helpers seed the registry |
| 30 | `TestRFC5176ListenerDiscardsWrongMessageAuthenticator` | fixture-only, shared helpers seed the registry |
| 31 | `TestRFC3579SubscriberNASRefusesEAPMessage` | fixture-only, shared helpers seed the registry |
| 32 | `TestRFC5176ProxyStateContentIsNeverRead` | fixture-only, shared helpers seed the registry |
| 33 | `TestRFC5176StateAndClassUnmodifiedProxyStateOpaque` | fixture-only, shared helpers seed the registry |
| 34 | `TestRFC5176CoAWithoutServiceTypeIsACKed` | fixture-only, shared helpers seed the registry |
| 35 | `TestRFC5176DisconnectAdmitsTheSection36TableAttributes` | fixture-only, shared helpers seed the registry |
| 36 | `TestRFC5176DisconnectWithANonIdentificationAttributeIsNAKed` | fixture-only, shared helpers seed the registry |
| 37 | `TestRFC5176DuplicateAndTimestampWindowsBothEndOutside` | fixture-only, shared helpers seed the registry |
| 38 | `TestRFC5176DuplicateAndTimestampWindowsBothHoldInside` | fixture-only, shared helpers seed the registry |
| 39 | `TestRFC5176InvalidEAPPacketCauseNeverSentForAnEAPRequest` | fixture-only, shared helpers seed the registry |
| 40 | `TestRFC5176SessionNotRemovableNeverInACoANAK` | fixture-only, shared helpers seed the registry |
| 41 | `TestRFC5176StateIsNeverReadAsAChangeOrAnIdentity` | fixture-only, shared helpers seed the registry |
| 42 | `TestRFC5176StateNeverChangesTheOutcome` | fixture-only, shared helpers seed the registry |
| 43 | `TestRFC5176UnsupportedServiceTypeValueIsNAKed` | fixture-only, shared helpers seed the registry |
| 44 | `TestRFC5176VendorSpecificIsAChangeOnly` | fixture-only, shared helpers seed the registry |
| 45 | `TestRFC5176VendorSpecificIsNeverAnIdentity` | fixture-only, shared helpers seed the registry |
| 46 | `TestRFC5176DuplicateRequestAnsweredFromCache` | fixture-only, shared helpers seed the registry |
| 47 | `TestRFC5176ErrorCausePlacement` | fixture-only, shared helpers seed the registry |
| 48 | `TestRFC5176InvalidCodeDiscarded` | fixture-only, shared helpers seed the registry |
| 49 | `TestRFC5176LengthFieldGovernsTheOctetsRead` | fixture-only, shared helpers seed the registry |
| 50 | `TestRFC5176MessageAuthenticatorZeroesBothFields` | fixture-only, shared helpers seed the registry |
| 51 | `TestRFC5176ProxyStateReturnedUnmodified` | fixture-only, shared helpers seed the registry |
| 52 | `TestRFC5176ServiceTypeNAKed` | fixture-only, shared helpers seed the registry |
| 53 | `TestRFC5176StaleEventTimestampDiscarded` | fixture-only, shared helpers seed the registry |
| 54 | `TestRFC5176StateReturnedUnmodified` | fixture-only, shared helpers seed the registry |
| 55 | `TestRFC5176UnsupportedAttributeNAKed` | fixture-only, shared helpers seed the registry |
| 56 | `TestRFC5176SharedSecretChosenBySourceAddress` | fixture-only, shared helpers seed the registry |
| 57 | `TestRFC5176UntrustedSourceDiscarded` | fixture-only, shared helpers seed the registry |
| 58 | `TestRFC5176MultipleMatchingSessionsNAKed` | fixture-only, shared helpers seed the registry |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| NAS-Port | 32-bit, compared to LocalSID 1-65535 | 65535 matches LocalSID 65535 | 0 matches no session | 65536 matches no session (no uint16 truncation) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `radius-coa-stale-acct-session-id.ci` | `test/l2tp/` | daemon with `coa-port` and accounting to a fixture server; an L2TP session comes up and the fixture captures the Accounting-Start id `T-S-N`; a rate CoA, then a Disconnect, each with `T-S-<other>` (same tunnel and session numbers, a last field Ze never issued): both answer 503, no rate change is applied and the session stays up; then a rate CoA with `T-S-N` answers CoA-ACK with one rate change, and a Disconnect with `T-S-N` answers ACK and the session ends. Red at HEAD: the prefix match ACKs the `T-S-<other>` CoA with a rate change, then ACKs the `T-S-<other>` Disconnect and tears the session down (AC-1 b). New fixture `l2tp/radius-coa-acct-id` in `internal/test/fixture/tunnel_fixture_l2tp_radius.go` | |
| `radius-coa-identification-mismatch.ci` | `test/l2tp/` | live session; CoA with the right Acct-Session-Id and a wrong Calling-Station-Id answers 503 and the shaper log shows no update; the right pair answers ACK. Calling-Station-Id, not User-Name: HEAD already evaluates User-Name and answers 503, so only an attribute HEAD ignores makes the test red at HEAD | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `l2tp-radius-coa-stale-session-freeradius` | `test/interop-radius-subscriber/scenarios/` (the `interop-radius-subscriber` lab, Q-7) | FreeRADIUS `radclient` plus its `detail` accounting log, xl2tpd/pppd client | `radclient` sends a rate CoA, then a Disconnect, each with the Acct-Session-Id from the server's `detail` record of the live session with its last field changed (`T-S-<other>`): CoA-NAK 503 with `tc` showing the rate unchanged, then Disconnect-NAK 503 and the session stays up (Ze log and pppd both read); the same two with the recorded id itself draw CoA-ACK with `tc` showing the new rate, then Disconnect-ACK, and pppd sees the session end | |
| `l2tp-radius-coa-identification-freeradius` | `test/interop-radius-subscriber/scenarios/` | FreeRADIUS `radclient` | the attribute set FreeRADIUS copies from accounting (A-8) selects the session and a rate CoA is ACKed with the new rate visible in `tc`; the same request with one mismatched identifier is NAKed 503 and `tc` is unchanged | |

Each scenario records a forced red: revert the selector producer, rebuild the Ze image, observe the scenario fail, restore. For `l2tp-radius-coa-stale-session-freeradius` the red is the prefix match at HEAD answering the `T-S-<other>` CoA with an ACK and `tc` showing the new rate, then answering the `T-S-<other>` Disconnect with an ACK and pppd seeing the session end; a stale id with another (T,S) is not used, because HEAD already answers it 503 and the scenario could not go red. For `l2tp-radius-coa-identification-freeradius` the red is HEAD ACKing the request whose one mismatched identifier is one HEAD does not evaluate (Framed-IP-Address or Calling-Station-Id) and `tc` showing the new rate.

## Files to Modify
- `internal/component/l2tp/plugins/authradius/coa.go` - replace `findSubscriberSession`, `oneSession`, `findSessions` and the L2TP-only branch of `handleCoA` with one selector (NAS identity check, then every session identification attribute against the registry); `handleDisconnect` uses the same selector; Q-3 removes the four unevaluable types from both accepted sets; `addFilterID`/`addVendorSpecific` look the CoS profile up (T-1); `applySubscriberCoA` gains rollback (T-2, last phase). Section 3 and 3.5 quotes above the selector and its NAK branches
- `internal/component/l2tp/plugins/authradius/register.go` - `startCoAListener` copies `cfg.NASIdentifier` and `cfg.SourceAddress` into `coaListenerConfig` (AC-3); `coaListenerConfig` in `coa.go` gains the two fields. `handlePacket` and `serve` are not edited, so this spec does not touch the function `spec-radius-rfc-defects` D2 edits
- `internal/component/l2tp/plugins/authradius/acct.go` - `onSessionIPAssigned` writes the issued id onto the registry entry for (T,S), `onSessionDown` clears it, both through the atomic registry upsert (R-9: the id write passes insert=true, the clear passes insert=false, so it never recreates an entry `Remove` deleted); `genSessionID` comment no longer names a prefix parser
- `internal/component/l2tp/subscriber/session.go`, `registry.go` - `AcctSessionID` documented as the Acct-Session-Id Ze put on the wire; one atomic upsert that, under the write lock, inserts the entry when missing only when its insert flag is set (a missing id with the flag clear changes nothing), applies a change to the stored entry and reports whether it existed before (R-9); `CallingStationID` field (and the resolved NAS-Port-Id per A-6) so the selector reads one table; `LookupByAcctSessionID` replaced by the selector's read
- `internal/component/l2tp/subscriber_bridge.go` - stop writing `AcctSessionID` (`onSessionUp`, `onSessionIPAssigned`); `onSessionUp`, `onAuthResult` and `onSessionIPAssigned` write the entry through the atomic upsert instead of `Get` then `Add`, with no other registry write (R-9); carry Calling-Station-Id from the L2TP payload
- `internal/component/l2tp/subscriber/cmd/subscriber.go` - the field comment beside the `acct-session-id` key states that the value is the Acct-Session-Id Ze sent the RADIUS server, absent when none was issued (Q-2 consequences); the code keeps printing the field. `subscriber_test.go` sets no `AcctSessionID` (grep 2026-10-14) and gains one case asserting the key carries the wire id; no YANG names the key (grep of the `yang/` directory, 2026-10-14)
- `internal/component/l2tp/pppoe/subsystem.go` - stop writing the internal id into `AcctSessionID` (`subscriberSession`, `onSessionUp`)
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
| 3 | CLI command added/changed? | Yes (output value) | no command or key is added, but under Q-2 (B) `show subscriber`'s `acct-session-id` changes value (`l2tp-T-S` to the wire id) and is absent without accounting and for PPPoE; `docs/architecture/l2tp/subscriber-session-model.md` (the page of `subscriber/cmd/subscriber.go` in `ai/CODE-TO-DOCS.md`) says what the key holds, in the phase that changes the field (row 12) |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | behaviour of an existing plugin; no registration change |
| 6 | Has a user guide page? | Yes | `docs/guide/l2tp.md` CoA/DM section (AC-11) |
| 7 | Wire format changed? | No | same packets; different answers |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/audit/rfc5176.json` re-judged rows; `docs/features/rfc-status.md` regenerated if any verdict moves |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` if the new fixture is listed there; `docs/architecture/testing/interop.md` for the `interop-radius-subscriber` lab section and suites row, and the two scenarios; if this spec builds the lab, also the L2TP and PPPoE preflight text named by `spec-radius-rfc-defects` Documentation row 10 (the shared `/dev/ppp` probe) |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | Yes | `docs/architecture/l2tp/subscriber-session-model.md`: `AcctSessionID` meaning, the atomic registry upsert (R-9), the `show subscriber` `acct-session-id` value, and the removed `LookupByAcctSessionID` anchor; `docs/architecture/traffic/cos-dynamic.md` gains the 407 for an unknown profile in the T-1 phase (its names are already current, `88d784795c`) |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | events unchanged until A-7's contract, which `spec-lifecycle-invariants` documents |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | derive with `./le spec citation anchors spec plan/immediate/spec-rfc5176-remaining-defect-classes.md` at implementation; known: `coa.go` → `docs/guide/l2tp.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | No | the CoA section shows config only, unchanged; no page shows `show subscriber` output with `acct-session-id` (grep of `docs/` for `acct-session-id`, 2026-10-14, no hit) |

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
   - Tests: `TestAcctIssuedSessionIDIsRecordedOnTheSubscriberSession`, `TestBridgeUpdateKeepsAcctSessionID`, `TestRegistryUpsertKeepsFieldsItDidNotChange`
   - Files: `acct.go`, `subscriber/session.go`, `registry.go`, `subscriber_bridge.go`, `subscriber/cmd/subscriber.go` and its test, `pppoe/subsystem.go`, `docs/architecture/l2tp/subscriber-session-model.md`
   - Verify: A-2, A-3, A-4 confirmed or broken in the spec
3. **Phase: selector (S-1, S-2, S-3, Q-3)** -- delete the three lookups and the L2TP-only branch, add the selector; owner-approved helper and test edits land here
   - Tests: AC-1..AC-6 units, the approved tagged-test changes, both `.ci` tests
   - Files: `coa.go`, `register.go` (`startCoAListener` passes the NAS identity), the test helpers, `docs/guide/l2tp.md` identification prose and rows
   - Verify: every staled unit re-recorded; `./le rfc check` green for rfc5176
4. **Phase: unknown CoS profile (T-1)**
   - Tests: `TestRFC5176UnknownCoSProfileIsNAKed`, `TestCoSChangeRecordsProfileOnlyAfterApply`
   - Files: `coa.go`, `internal/plugins/cos/handler.go`, `docs/guide/l2tp.md` 407 row, `docs/architecture/traffic/cos-dynamic.md` 407 sentence
5. **Phase: interop** -- the `interop-radius-subscriber` lab (Q-7; built here unless `spec-radius-rfc-defects` built it first), both scenarios, forced reds recorded
6. **Phase: rollback (T-2)** -- only after `spec-lifecycle-invariants` AC-4 lands (A-7)
   - Tests: `TestRFC5176FailedCoSApplicationRestoresTheRate`, `TestRFC5176FailedRollbackIsNAKedAndLogged`
   - Files: `coa.go`, `docs/guide/l2tp.md` 506 row
7. **Phase: evidence** -- independent re-judge of the seven rows RFC5176-3.3-1, -2.3-5, -2.3-6, -2.3-7, -3.6-1, -3-1 and -3.5-8 (AC-10); Q-6 outcome recorded (AC-9)

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
| Evidence | fresh discrimination records for every staled unit; independent re-judge of the seven rows (AC-10); extraction sign-off preserved |

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
| A-2/A-3 broken (registry entry missing at issue time) | DESIGN: the owner's Q-2 (B) rests on A-2/A-3, so a broken assumption reopens Q-2 for a design round and the owner |
| A-7 not delivered | AC-8 stays open; the spec is not closed (no parking) |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Exact Acct-Session-Id match against the id Ze issued (S-1) | (C) keep the prefix and refuse only when a different id was issued for (T,S) | (C) still tears down a reconnected session whenever no id was recorded for it (accounting off, or a session alive across a reload), so it is not fully correct; rejected under `ai/rules/simplicity.md` (simplicity may not cut correctness) |
| One session table, the registry (Q-2 B) | (A) authradius issued-id table plus the L2TP snapshot | (B) fixes S-2 at its cause (a field holding a value no packet carries) and reaches PPPoE with the same code; (A) keeps two lookups |
| Unevaluable identification attributes refused 401 (Q-3 i) | 503; ignore | ignoring is the breach; 401 tells the DAC operator which attribute to stop sending |
| NAS identity is what Ze sends the server, derived from `appendNASIdentity` (AC-3) | the local address the request arrived on (`ReadMsgUDP` plus `IP_PKTINFO` in `serve` and a new `handlePacket` parameter) | Ze announces its identity in every Access-Request and Accounting-Request, and a DAC copies it from there (A-8); comparing that is one declaration and no socket change. The arrival address needs PKTINFO machinery, edits `handlePacket` beside `spec-radius-rfc-defects` D2, and names an identity Ze never announced |
| Rollback by compensation (Q-4 alpha) | one combined backend Apply | keeps the shaper and CoS plugins separate; the failure of the compensation is surfaced |

## Known Limitations
- A pure L2TP LNS session has no access interface, so a CoS CoA for it stays NAK 506 (`docs/architecture/traffic/cos-dynamic.md`, Consequences, the `AccessInterface` paragraph).
- A DAC that names the NAS by an identity Ze does not send (an arrival address, NAS-IPv6-Address) is refused 403 or 401; the operator sets `source-address` or `nas-identifier` to match (AC-3, R-5).
- PPPoE Disconnect teardown (504 today) and PPPoE CoA delivery belong to `spec-lifecycle-invariants` (AC-3, I-4).
- With accounting not configured Ze issues no Acct-Session-Id, so a DAC must select by User-Name, NAS-Port or Framed-IP-Address (R-3).
- Termination-Action stays excluded (Q-6, owner 2026-10-14).

## Design Review Log

| Date | Round | Outcome |
|------|-------|---------|
| 2026-10-14 | owner answers recorded | Q-1 answered through `spec-radius-rfc-defects` Q-6 ("whatever", recommendation taken: the stale-id prefix match stays here as S-1/AC-1); Q-7 answered through `spec-radius-rfc-defects` Q-7 ("Make Free radius, its own lab."): every FreeRADIUS-in-L2TP lab reference now names the shared `interop-radius-subscriber` lab (Required Reading, Interop Tests, Files to Modify, Files to Create, Documentation row 10, Implementation step 5); scenario names unchanged. Other questions unchanged. Review round owed |
| 2026-10-14 | consistency edit from `spec-radius-rfc-defects` review round 2 (its NIT-5) | Files to Modify and Documentation row 10 point at that spec's lab rows: the shared `/dev/ppp` move out of the L2TP and PPPoE labs, and the L2TP and PPPoE preflight doc text, owed by whichever spec builds the lab. No other change |
| 2026-10-14 | design review round 1 (0 BLOCKER, 6 ISSUE, 6 NIT), applied | ISSUE-1: AC-1 splits into (a) reused (T,S) for the fake-service units and (b) the live session's id with a wrong last field (`T-S-<other>`) for the `.ci` and `l2tp-radius-coa-stale-session-freeradius`, red at HEAD because the prefix match ACKs it; forced red restated (allocators verified: `reactor.go::allocateLocalTID` counts, `session.go::allocateSessionID` is random). ISSUE-2: Framed-IP-Address and the IPv6 pair are CoA-only in AC-2; a Disconnect carrying them stays 401, Section 3.6 legend quoted. ISSUE-3: the NAS identity is what `appendNASIdentity` sends, passed through `startCoAListener` / `coaListenerConfig`; no arrival address, no PKTINFO, `handlePacket` untouched; a NAS identification attribute of a kind Ze sends none of is 403; Key Design Decisions row added; R-5 rewritten. ISSUE-4: `subscriber/cmd/subscriber.go` in Files and Digests, A-4 corrected, Doc rows 3 and 17 revisited, Q-2 consequences name the `show subscriber` change. ISSUE-5: the stale "over-claims" and `extractRate` constraints dropped (`88d784795c` verified), AC-11 and Doc row 12 owe only behaviour edits and the 407/506 text. ISSUE-6: RFC5176-3-1 and -3.5-8 added to AC-10, step 7, Required Reading and Deliverables. NIT-7: AC-6's L2TP+PPPoE case conditional on Q-8. NIT-8: per-option consequences for Q-3, Q-6, Q-8. NIT-9: atomic registry update, R-9 and its unit. NIT-10: `oneSessionService`, `coaTestListener` named. NIT-11: radius-first order in R-6 and in `spec-radius-rfc-defects` (Dependencies note). NIT-12: line numbers replaced by symbols; the approval block is a table. Also found while applying: `radius-coa-identification-mismatch.ci` used a wrong User-Name, which HEAD already answers 503, so it now uses Calling-Station-Id; the identification interop red names an attribute HEAD ignores. Round 2 owed, narrow, over these edits |
| 2026-10-14 | design review round 2 (0 BLOCKER, 2 ISSUE, 0 NIT), applied | ISSUE-1: AC-1 (b) sends the rate CoA before the Disconnect in both halves, because at HEAD the Disconnect tears the session down (`findSubscriberSession` misses on `l2tp-T-S`, `oneSession` / `findSessions` hits on the `T-S-` prefix, `TeardownSession`, ACK) and a later CoA finds nothing; negative `T-S-<other>`: HEAD ACKs the CoA with a rate event, then ACKs the Disconnect and tears down, the fix answers 503 to both and the session survives; positive `T-S-N`: CoA-ACK with one rate event, then Disconnect-ACK and teardown. Wiring row, `radius-coa-stale-acct-session-id.ci`, `l2tp-radius-coa-stale-session-freeradius` and its forced red follow the same order (verified in `coa.go::handleCoA`, `handleDisconnect`). ISSUE-2: `subscriber_bridge.go::onSessionUp` (verified: `Get`, edit, `Add`, and `Registry.Add` replaces the whole stored copy) moves onto the atomic registry update, `Add` only when no entry exists; Digest, R-9, the `subscriber_bridge.go` Files row and `TestRegistryUpdateKeepsFieldsItDidNotChange` cover all three paths. Round 3 owed, narrow, over these edits |
| 2026-10-14 | design review round 3 (0 BLOCKER, 1 ISSUE, 1 NIT; `tmp/session/2026-10-12-0c3648c8-b9d7-45d8-b3f0-2473bceefb25/scratch/review-rfc5176-design-r3.md`), applied by the main thread | ISSUE-1: R-9 reads `wasActive` inside the update before the change, so `RecordSessionUp` still fires once on the first up (unit assertion added). NIT-2: the SessionUp emit and `RecordSessionUp` run after the update returns (synchronous subscribers, non-reentrant `Registry.mu`). Round 4 owed, narrow, over the R-9 row only |
| 2026-10-14 | design review round 4 (0 BLOCKER, 1 ISSUE, 2 NIT; `tmp/session/2026-10-12-0c3648c8-b9d7-45d8-b3f0-2473bceefb25/scratch/review-rfc5176-design-r4.md`), applied | ISSUE-1: the test row splits: `TestBridgeUpdateKeepsAcctSessionID` in `subscriber_bridge_test.go` (package `l2tp`) carries the three bridge paths interleaved with the accountant's write, both orders, and `RecordSessionUp` exactly once on the first up; `TestRegistryUpsertKeepsFieldsItDidNotChange` in `subscriber/registry_test.go` (package `subscriber`) covers the upsert alone (both files verified). NIT-1: synchronous delivery cited at `plugin/server/dispatch.go::deliverEvent` and `engine_event.go::dispatchEngineEvent`. NIT-2 (main-thread decision): the atomic update becomes an upsert that inserts when missing under the same write lock; the fallback that called `Add` only when no entry existed is deleted from R-9 and from the `subscriber_bridge.go` Files row (registry API verified: `registry.go` holds `Add`, `Remove`, `Get`, `All`, `Count`, `ByAccessType` and `LookupByAcctSessionID` under one `sync.RWMutex`, so the upsert is new). Found while applying: the accountant's clear at session down must not insert after the bridge's `Remove`; R-9 records this as open. Round 5 owed, narrow, over R-9 and the two test rows |
| 2026-10-14 | design review round 5 (0 BLOCKER, 2 ISSUE, 0 NIT; `tmp/session/2026-10-12-0c3648c8-b9d7-45d8-b3f0-2473bceefb25/scratch/review-rfc5176-design-r5.md`), applied | ISSUE-1 (main-thread decision): the SessionIPAssigned-after-SessionDown ordering race is pre-existing (verified: `reactor_kernel.go::handleSessionIPAssigned` emits after releasing `tunnelsMu`; `teardownSessionByID` is called from `session_timeout.go` and `subsystem_snapshot.go::TeardownSession`; the bridge's `onSessionIPAssigned` `Get`-miss then `Add`; `acct.go::onSessionDown` returns on a missing record). R-9 states the upsert does not widen it and that it is recorded, not fixed; journal row in `plan/journal/late-write-lands-on-the-successor.md`. ISSUE-2: the `acct.go` and `registry.go` Files rows and `TestRegistryUpsertKeepsFieldsItDidNotChange` now carry the insert flag (clear uses insert=false, decided), and the test row carries the clear-after-Remove case. Round 6 owed, narrow, over R-9 and those three rows |
| 2026-10-14 | design review round 6, narrow (0 BLOCKER, 0 ISSUE, 0 NIT; `tmp/session/2026-10-12-0c3648c8-b9d7-45d8-b3f0-2473bceefb25/scratch/review-rfc5176-design-r6.md`) | R-9, the Files rows and both test rows agree; "not widened" holds at the producers. Design converged; Q-2, Q-3, Q-4, Q-6 and Q-8 stay with the owner |
| 2026-10-14 | owner answers recorded | Owner, 2026-10-14: "Do what you think is right", accepting every recommendation: Q-2 (B), Q-3 (i), Q-4 (alpha) after `spec-lifecycle-invariants` AC-4, Q-6 keep the exclusion, Q-8 yes. Edited: question header and rows, Q-5 note, the 3.3:7 table row, Required Reading (`bng-5-pppoe.md` now owes a sentence), R-4, AC-4, AC-6 and AC-9 lose their conditionals, AC-12 added (PPPoE selection by User-Name or Framed-IP-Address, from Q-8's own row), the two tagged-test rows lose the Q-3 conditional and the 403 alternative, failure routing for A-2/A-3, Known Limitations. Q-8 reconciled with Q-2 (B): the "no PPPoE session selectable" outcome was the "no" branch; PPPoE entries exist in the registry with `Username` and `IPv4Addr`. Owed by the next design round (narrow): (1) a TDD row for AC-12, a unit over a PPPoE registry entry selected by User-Name and by Framed-IP-Address asserting Disconnect 504 and an Acct-Session-Id-only request 503; (2) what AC-12's CoA half answers if this spec lands before `spec-lifecycle-invariants` AC-3; (3) whether the PPPoE `Username` set only on first insert (`subscriberSession` sets it when `!found`) is always populated before an Access-Accept makes the session reachable, verified at the producer; (4) Documentation row for `docs/architecture/l2tp/bng-5-pppoe.md` and the `docs/guide/l2tp.md` CoA/DM text (AC-11) naming PPPoE selection |

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
