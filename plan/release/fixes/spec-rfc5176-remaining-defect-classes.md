# Spec: RFC 5176 residual CoA atomicity and session matching

| Field | Value |
|-------|-------|
| Status | design |
| Priority | P0 |
| Release reason | CoA and Disconnect session selection must unify on the subscriber registry, evaluate all supported NAS and session identification attributes, and reject unknown CoS profiles with Error-Cause 407 before touching any session. |
| Scope | protocol |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`rfc/extraction/rfc5176.json` records sign-off on 2026-09-01. That source-walk
milestone is delivered; it does not prove that every CoA or Disconnect lookup
honours the request's full NAS and session identification selector across a
single subscriber table or rejects unknown CoS profiles with the RFC-specified
attribute-value cause. This spec owns the remaining RFC 5176 conformance
obligations at HEAD.

`plan/release/validation/spec-rfcgate-6-supported-extraction-signoff.md` is the
parent evidence programme. Its 2026-09-01 signature is not counted as new work
or used to close these residual behaviours.

### Already landed at HEAD (rebased 2026-11-26)

Several defect classes from the 2026-08-31 walk and from the 2026-10-14 draft of
this spec have already landed at HEAD and are preserved rather than rebuilt:

| Former item | Status at HEAD | Source evidence |
|-------------|----------------|-----------------|
| Service-Type never read | landed (`fe51839da0`) | `coa.go::answerRequest` NAKs every Service-Type in a CoA-Request with 405, quoting RFC 5176 Section 3.2 |
| Proxy-State and State not echoed | landed (`fe51839da0`) | `coa.go::sendResponse` copies every `AttrProxyState` and `AttrState` verbatim, quoting Sections 3.1 and 3.3 |
| Stale Event-Timestamp NAK'd | landed (`fe51839da0`) | `coa.go::answerRequest` silently discards stale timestamps, quoting Section 6.3, and NAKs absent timestamps with 404 |
| S-1 / AC-1 exact Acct-Session-Id match (unit) | landed | `coa.go::findSessions` compares the complete `Acct-Session-Id` from `acctInstance.currentSessionIDs()` keyed by `subscriberid.L2TPBirth` rather than a `T-S-` prefix; proven by `rfc5176_account_identity_test.go` and `coa_retired_accounting_test.go`. Functional `.ci` and FreeRADIUS interop proof for `T-S-<other>` remain owed here |
| `findSubscriberSession` selector conjunction | landed (3 attributes) | `coa.go::findSubscriberSession` runs `sessionSelectorsMatch` over `Acct-Session-Id`, `User-Name` and `NAS-Port`; proven by `TestCoARegisteredSubscriberAllSelectorsMustMatch` and `TestDisconnectRegisteredSubscriberAllSelectorsMustMatch` (`rfc5176_owner_apply_test.go`) |
| AC-4 refusal of the 4 unevaluable identification attributes | landed | `coaSupportedAttrs` and `disconnectSupportedAttrs` exclude `Called-Station-Id`, `Acct-Multi-Session-Id`, `Chargeable-User-Identity` and `NAS-IPv6-Address` (NAK 401); proven by `TestDynamicAuthorizationRefusesUnimplementedIdentification` (`rfc5176_identification_test.go`). This spec preserves those 4 refusals, restores and evaluates the 7 evaluable attributes, and adds the evaluator drift-guard test |
| T-2 / AC-8 atomic multi-change application and rollback | landed | `coa_transaction.go::confirmCoA` runs two-phase `prepare` / `apply` / `commit` / reverse `rollback` across `request l2tp shaper rate` and `request class-of-service session` via `cl.cfg.Dispatch`, NAKs 506 and raises `doctor-radius-coa-apply` (`coa_health.go`) on failure; proven by `rfc5176_atomic_owners_test.go` and `rfc5176_owner_apply_test.go`. `internal/plugins/cos/shared_access.go` and `session_transaction.go` store `profileName` only after `access.write` succeeds |
| `interop-radius-subscriber` FreeRADIUS lab | landed | `internal/le/interoplab/radiussubscriber/` and `test/interop-radius-subscriber/scenarios/` already exist with 7 scenarios including `radius-coa-disconnect-freeradius` and `radius-coa-duplicate-freeradius` |
| Event ordering race (2026-10-14 journal row) | landed | `plan/journal/late-write-lands-on-the-successor.md` (2026-10-14 row) fixed by `sessionNoticeTicket`, proven by `l2tp/session_event_order_test.go` |
| Termination-Action (AC-9 / Q-6) | excluded (owner 2026-10-14) | `rfc/extraction/rfc5176.json` site 3.3:7 stays `feature-out-of-scope` (RFC 2865 Section 5.29 MAY); nothing implemented |

### Residual defects this spec owns at HEAD (verified 2026-11-26)

| ID | Producer | RFC text (read in `rfc/full/rfc5176.txt`) | What Ze still does wrong at HEAD |
|----|----------|--------------------------------------------|----------------------------------|
| S-2 | `coa.go::findSubscriberSession`, `oneSession`, `findSessions`, `handleCoA`, `handleDisconnect`; `subscriber_bridge.go`; `pppoe/subsystem.go`; `acct.go` | Section 3: "The combination of NAS and session identification attributes included in a CoA-Request or Disconnect-Request packet MUST match at least one session in order for a Request to be successful; otherwise a Disconnect-NAK or CoA-NAK MUST be sent." Section 2.3: "A NAS that does not support dynamic authorization changes applying to multiple sessions MUST send a CoA-NAK or Disconnect-NAK in reply; an Error-Cause Attribute with value 508 (Multiple Session Selection Unsupported) SHOULD be included." | CoA/DM still splits selection across two tables: `findSubscriberSession` (which requires `AttrAcctSessionID` and calls `Registry.LookupByAcctSessionID`) followed by an L2TP-only fallback (`oneSession` / `findSessions` reading `l2tp.Snapshot()` and `acctInstance.currentSessionIDs()`). Meanwhile `subscriber_bridge.go` and `pppoe/subsystem.go` write the internal canonical `Session.ID` into `Session.AcctSessionID` using full-copy `Get`/`Add`, never the wire `Acct-Session-Id` (`T-S-bootTag-counter`). Consequently: (1) a real DAC's wire `Acct-Session-Id` always misses the registry and falls into the L2TP-only fallback, which NAKs every CoS change 506 ("no access interface") even when the registry entry carries an `AccessInterface` (AC-5); (2) an L2TP and a PPPoE session sharing a `User-Name` never reach 508 because the fallback sees L2TP only (AC-6); (3) a PPPoE session cannot be selected by `User-Name` or `Framed-IP-Address` because `findSubscriberSession` requires `Acct-Session-Id` (AC-12); (4) an accounting reload (`acct.go::detach`) drops live sessions from `currentSessionIDs()` (R-2) |
| S-3 | `coa.go::coaSupportedAttrs`, `disconnectSupportedAttrs`, `unsupportedAttrValue`, `sessionSelectorsMatch`, `coaListenerConfig`; `register.go::startCoAListener` | Section 3, first sentence above, and: "If all NAS identification attributes match, and more than one session matches all of the session identification attributes, then a CoA-Request or Disconnect-Request MUST apply to all matching sessions." Section 3.5: "\"NAS Identification Mismatch\" is a fatal error sent if one or more NAS identification attributes (see Section 3) do not match the identity of the NAS receiving the Request." Section 3.6 Disconnect table: types 8, 96, 97 have entry "0" ("This attribute MUST NOT be present in packet.") | At HEAD all 7 evaluable identification attributes Ze already produces or stores — `Framed-IP-Address`, `Calling-Station-Id`, `NAS-Port-Id`, `Framed-Interface-Id`, `Framed-IPv6-Prefix`, `NAS-IP-Address` and `NAS-Identifier` — are refused with NAK 401 (`TestDynamicAuthorizationRefusesUnimplementedIdentification`), and `coaListenerConfig` carries no NAS identity. A real DAC copying the identification set from Ze's own Accounting-Request is refused 401 instead of evaluated (AC-2, AC-3) |
| T-1 | `coa.go::setCoSProfile` | Section 2.3: "If one or more authorization changes specified in a CoA-Request cannot be carried out, the NAS MUST send a CoA-NAK." Section 3.5: "\"Invalid Attribute Value\" is a fatal error sent if a CoA-Request or Disconnect-Request contains an attribute with an unsupported value." | `setCoSProfile` parses `cos:<name>` (or vendor CoS VSA) without calling `coreCos.Lookup(name)`. An unknown profile passes `readCoAChange`, enters `handleCoA` / `confirmCoA`, and draws CoA-NAK 506 (`ErrorCauseResourcesUnavailable`) plus a false `doctor-radius-coa-apply` backend-health finding instead of CoA-NAK 407 (`ErrorCauseInvalidAttributeValue`) before session lookup (AC-7) |

### Owner decisions (all answered 2026-10-13 and 2026-10-14; zero open questions)

| ID | Question | Owner decision | Status at HEAD |
|----|----------|----------------|----------------|
| Q-1 (S-1 scope) | Take the stale-Acct-Session-Id defect (`spec-radius-rfc-defects` Q-6) into this spec? | **Answered 2026-10-14 ("whatever"; recommendation taken: yes).** Exact Acct-Session-Id match belongs here | Unit exact match landed; unified registry storage (S-2), `.ci` and interop proofs for `T-S-<other>` complete it here |
| Q-2 (S-2 design) | One session table: (B) `subscriber.Registry` is the only table the CoA/DM matcher reads and `Session.AcctSessionID` is redefined as the wire `Acct-Session-Id` written by `acct.go` (empty when none was issued); or (A) an authradius-owned table beside the L2TP snapshot | **Answered 2026-10-14 ("Do what you think is right"; recommendation taken: (B)).** Unified matcher over `subscriber.Registry`; `Session.AcctSessionID` holds the wire ID; `show subscriber` prints the wire ID under `acct-session-id` (omitted when none was issued) | Open; implemented by this spec |
| Q-3 (S-3 unevaluable attributes) | For identification attributes Ze holds no fact to compare (`Called-Station-Id`, `Acct-Multi-Session-Id`, `Chargeable-User-Identity`, `NAS-IPv6-Address`): (i) keep them out of both accepted sets (NAK 401); (ii) accept and NAK 503/403; (iii) ignore | **Answered 2026-10-14: (i).** The 4 types stay excluded from both accepted sets and draw NAK 401 | Exclusion landed; this spec keeps those 4 excluded, evaluates the 7 supported ones (AC-2, AC-3), and adds the drift guard |
| Q-4 (T-2 rollback) | Atomic multi-change: (alpha) compensate already-applied parts and NAK 506; (beta) merge shaper and CoS into one backend Apply | **Answered 2026-10-14: (alpha).** | Landed via `coa_transaction.go::confirmCoA` |
| Q-5 (RFC-tagged tests) | Changing RFC-tagged tests | **Answered 2026-10-13 ("approve all RFC-tagged test changes"); governance superseded by `ai/META-DECISIONS.md` ("Test changes").** Independent agent running `ze-test-judge` judges every changed RFC-tagged test and records `RFC-approved:` approvals without manual owner terminal commands | Active governance under `ze-test-judge` |
| Q-6 (AC-9 Termination-Action) | Select optional Termination-Action (RFC 2865 Section 5.29) or keep `feature-out-of-scope` | **Answered 2026-10-14: keep the exclusion; nothing implemented** | Closed; preserved |
| Q-7 (interop lab) | Where FreeRADIUS runs beside a live L2TP session | **Answered 2026-10-14 ("Make Free radius, its own lab."): `interop-radius-subscriber`** | Lab landed at HEAD; this spec adds its scenarios to the existing lab |
| Q-8 (AC-12 PPPoE selection) | Select PPPoE sessions in the unified matcher by `User-Name` and `Framed-IP-Address`? | **Answered 2026-10-14: yes.** Same evaluator runs over PPPoE registry entries; PPPoE Disconnect answers 504; PPPoE CoA answers 506 until `spec-lifecycle-invariants` AC-3 wires PPPoE owners | Open; implemented by this spec |

### Coordination with sibling specs

- `plan/release/fixes/spec-l2tp-proxied-pap-interop-and-identity.md` (Status: ready, D-3): separates authenticated `Username` from the LAC's unverified `ProxyAuthenName` claim, and requires RADIUS accounting `User-Name` and CoA `User-Name` matching to use the authenticated name, else the LAC claim. When this spec moves CoA selection onto `subscriber.Registry`, whichever spec lands second ensures `subscriber.Session` carries the LAC claim (when unauthenticated) and the selector's `User-Name` evaluator compares against the same effective accounted name (authenticated `Username`, else LAC claim).
- `plan/release/fixes/spec-pppoe-subscribers-produce-no-accounting-or-telemetry.md` (Status: design): connects RADIUS accounting to PPPoE sessions. Before that spec lands, PPPoE sessions issue no wire `Acct-Session-Id`, so `Session.AcctSessionID` is empty for PPPoE and an `Acct-Session-Id`-only request answers 503 for PPPoE (AC-12). This spec moves `pppoe/subsystem.go` onto atomic `Registry.Upsert` and stops it from writing internal IDs into `AcctSessionID`; once PPPoE accounting lands and writes its wire `Acct-Session-Id` through the same `Registry.Upsert`, the unified selector automatically matches PPPoE sessions by wire `Acct-Session-Id` with no changes to `coa.go`.
- `plan/release/fixes/spec-lifecycle-invariants.md` (Status: ready): AC-4's applied-result contract has already landed (`confirmCoA`), so this spec has no remaining dependency blocker. PPPoE Disconnect teardown (I-4) and PPPoE CoA owner wiring (AC-3) stay in `spec-lifecycle-invariants`; until they land, a matched PPPoE Disconnect answers 504 and a matched PPPoE CoA fails `confirmCoA` prepare and answers 506 (never a false ACK).

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/rfc-conformance-gates.md` - extraction sign-off, tagged-test routes, discrimination records
  → Constraint: every tagged unit whose body or producer changes stales its `rfc/discrimination/rfc5176.json` record; re-record with `./le rfc record`, never by hand. Every changed RFC-tagged unit is judged by an independent `ze-test-judge` agent (`ai/META-DECISIONS.md`).
  → Constraint: the 2026-09-01 extraction sign-off is not re-signed unless a site's disposition changes; no new requirement rows are added, so S-2, S-3 and T-1 are proven under existing rows RFC5176-3.3-1, RFC5176-2.3-5, RFC5176-2.3-6, RFC5176-2.3-7, RFC5176-3.6-1, RFC5176-3-1 and RFC5176-3.5-8.
- [ ] `docs/guide/l2tp.md` CoA/DM listener section (lines 1244-1365) - operator page for the CoA/DM listener
  → Constraint: at HEAD the page documents the 3-attribute matcher, 401 for `NAS-Identifier` / `Framed-IP-Address`, and the L2TP-only fallback using `currentSessionIDs`. Each phase that changes a behaviour updates that section and its source anchors in the same phase (`ai/rules/documentation.md`, AC-11).
- [ ] `docs/research/l2tpv2-ze-integration.md` - `// Design:` page of `coa.go`, `register.go` and `acct.go`
  → Decision: Section 6.1 assigns RADIUS CoA and DM handling for live sessions to `l2tp-auth-radius`, and Section 6.3 documents `confirmCoA` over `request l2tp shaper rate`. Unifying selection on `subscriber.Registry` preserves plugin placement and `confirmCoA`.
- [ ] `docs/architecture/testing/interop.md` - `The FreeRADIUS subscriber suite` (lines 4156-4205) and `Design decisions` (2026-11-23 scoped kernel capabilities)
  → Constraint: scenario directories are named without numeric prefixes; the existing `interop-radius-subscriber` suite hosts the new scenarios and inherits its declared `Suite.KernelCapabilities`; each scenario checks both Ze and FreeRADIUS sides and records a forced red by reverting the producer and rebuilding the image.
- [ ] `docs/architecture/l2tp/subscriber-session-model.md` - `// Design:` page of `subscriber/session.go`, `registry.go`, `subscriber_bridge.go` and `subscriber/cmd/subscriber.go`
  → Constraint: departs from lines 176-184 ("A selection by subscriber attribute has one walk and one capture, shared by the RADIUS CoA and Disconnect listener and the operator's `clear l2tp session user` and `clear l2tp session address`"): under Q-2 (B) CoA/DM walks `subscriber.Registry` instead of `l2tp.MatchSessions`, while still using `CaptureMatchedSession` (`L2TPBirth`) before acting on an L2TP match. Updates the page, removes the `LookupByAcctSessionID` anchor, documents wire `AcctSessionID`, `CallingStationID`, `NASPortID`, atomic `Upsert` and `show subscriber` output in the same work.
- [ ] `docs/architecture/l2tp/bng-5-pppoe.md` - `// Design:` page of `pppoe/subsystem.go` (including 2026-11-23 `Design decisions` on VPP API loss)
  → Constraint: PPPoE keeps its canonical `Session.ID`; stopping PPPoE from writing `Session.ID` into `AcctSessionID` and adding CoA/DM selection over the registry by `User-Name` / `Framed-IP-Address` does not touch VPP lifecycle or pidfd adoption; add a sentence noting PPPoE CoA/DM selection.
- [ ] `docs/architecture/traffic/cos-dynamic.md` - anchored to `coa.go` (`readCoAChange`, `setRate`, `setCoSProfile`)
  → Decision: `AccessInterface` is empty for a pure L2TP LNS, so AC-5 holds only for an L2TP session carrying an access interface; a pure-LNS CoS CoA stays NAK 506. Extend the Consequences paragraph with `setCoSProfile`'s `coreCos.Lookup` check returning 407 for an unknown profile.
- [ ] `docs/architecture/l2tp/bng-1-radius-attributes.md` - anchored to `register.go`, `acct.go`, `subscriber/session.go`, `tunnel_fixture_l2tp.go`
  → Constraint: `Calling-Station-Id` already rides `SessionIPAssignedPayload` into `acct.go`; this spec carries the same value onto `subscriber.Session` so CoA evaluates the exact value accounting sends.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc5176.md` - declared requirement IDs for RFC 5176
  → Constraint: rows used: RFC5176-3.3-1 (Section 3 "MUST match at least one session"), RFC5176-2.3-5 (attributes mandatory, NAK when a change cannot be carried out), RFC5176-2.3-6 (atomic), RFC5176-2.3-7 (508), RFC5176-3.6-1 (identification only), RFC5176-3-1 (Disconnect carries identification and Section 3.6 table attributes only), RFC5176-3.5-8 (a NAS never sends 502 "Request Not Routable"). Every row whose producer changes owes an independent re-judge.
- [ ] `rfc/full/rfc5176.txt` - Sections 2.2, 2.3, 3, 3.5, 3.6
  → Constraint: Section 3: "The combination of NAS and session identification attributes included in a CoA-Request or Disconnect-Request packet MUST match at least one session in order for a Request to be successful; otherwise a Disconnect-NAK or CoA-NAK MUST be sent."
  → Constraint: Section 3: "If all NAS identification attributes match, and more than one session matches all of the session identification attributes, then a CoA-Request or Disconnect-Request MUST apply to all matching sessions." Section 2.3 allows NAK 508 when multi-session changes are unsupported.
  → Constraint: Section 3.5: 403 (`ErrorCauseNASIdentificationMismatch`) when one or more NAS identification attributes do not match this NAS; 503 (`ErrorCauseSessionNotFound`) when session identification matches no session; 401 (`ErrorCauseUnsupportedAttribute`) for unsupported attributes; 407 (`ErrorCauseInvalidAttributeValue`) for invalid attribute values including an unregistered CoS profile.
  → Constraint: Section 3.6 Disconnect table gives types 8 (`Framed-IP-Address`), 96 (`Framed-Interface-Id`) and 97 (`Framed-IPv6-Prefix`) the Request entry "0" ("This attribute MUST NOT be present in packet"), so Disconnect continues to NAK them with 401.

**Key insights:**
- S-1 (unit exact match), AC-4 (401 on unevaluable attributes), `sessionSelectorsMatch` (3 attributes), T-2 (`confirmCoA` two-phase rollback), `cos` storing profile only after write, and the `interop-radius-subscriber` lab are ALREADY landed at HEAD.
- What remains open is: (1) unifying CoA/DM onto `subscriber.Registry` with wire `AcctSessionID` / `CallingStationID` / `NASPortID` written atomically via `Registry.Upsert`, deleting the L2TP-only fallback and `currentSessionIDs` (S-2, AC-5, AC-6, AC-12); (2) evaluating the 7 additional supported NAS and session identification attributes with 403 for foreign NAS identity and 401 preserved for the 4 unevaluable types plus Disconnect types 8/96/97 (S-3, AC-2, AC-3, AC-4); (3) calling `coreCos.Lookup` in `setCoSProfile` so unknown profiles draw 407 before session lookup and without raising `doctor-radius-coa-apply` (T-1, AC-7); and (4) functional `.ci` and FreeRADIUS interop proofs plus re-recorded/re-judged RFC 5176 evidence (AC-1b, AC-10, AC-11).

## Current Behavior (MANDATORY)

**Source files read (re-verified at HEAD 2026-11-26):**
- [ ] `internal/component/l2tp/plugins/authradius/coa.go` (1060 lines) - `admitRequest` / `answerRequest` authenticate, deduplicate by `(source IP, source port, Identifier)` + Authenticator, check Event-Timestamp, Service-Type 405, `unsupportedAttr` 401 (only `User-Name`, `NAS-Port`, `Acct-Session-Id` accepted for identification), `unsupportedAttrValue` 407 (`fixedLengthAttrs` has 4 types). `handleCoA` calls `readCoAChange` (`setCoSProfile` does NOT call `coreCos.Lookup`), tries `findSubscriberSession` (requires `AttrAcctSessionID`, calls `Registry.LookupByAcctSessionID` + `sessionSelectorsMatch`, captures `L2TPBirth`, calls `applySubscriberCoA` -> `confirmCoA`), then falls back to `oneSession` / `findSessions` (L2TP snapshot + `acctInstance.currentSessionIDs()`), which refuses every CoS change with 506. `handleDisconnect` checks `findSubscriberSession` for PPPoE -> 504, else `oneSession` -> `captureCoASession` -> `original.Teardown`. `coaListenerConfig` carries no `NASIdentifier` or `SourceAddress`.
- [ ] `internal/component/l2tp/plugins/authradius/coa_transaction.go` (120 lines) - `confirmCoA` already prepares, applies, commits and rolls back `request l2tp shaper rate` and `request class-of-service session` synchronously via `cl.cfg.Dispatch` and records `doctor-radius-coa-apply` via `recordCoAHealth`.
- [ ] `internal/component/l2tp/plugins/authradius/register.go` (570 lines) - `startCoAListener` constructs `coaListenerConfig` without `cfg.NASIdentifier` or `cfg.SourceAddress`.
- [ ] `internal/component/l2tp/plugins/authradius/acct.go` (1160 lines) - `genSessionID` mints `tunnel-session-bootTag-counter`; `onSessionIPAssigned` stores `acctSession` (including `acctSessID`, `callingStationID`, `nasPortID`) in `a.sessions`; `currentSessionIDs()` snapshots `a.sessions` by `L2TPBirth` for `findSessions`; `detach()` moves sessions to `acctRetirement` so detached sessions disappear from `currentSessionIDs()`. Never writes to `subscriber.Registry`.
- [ ] `internal/component/l2tp/plugins/authradius/nasidentity.go` (60 lines) - `appendNASIdentity(attrs, nasID, sourceAddr)` emits `NAS-IP-Address` when IPv4 `sourceAddr` is set, `NAS-Identifier` when `nasID` is set, and `hostNASIdentifier()` as `NAS-Identifier` only when neither is set; never `NAS-IPv6-Address`.
- [ ] `internal/component/l2tp/subscriber/session.go` (86 lines), `registry.go` (102 lines) - `Session` holds `L2TPBirth`, `ID`, `AccessType`, `State`, `MAC`, `Username`, `AccessInterface`, `PPPoESID`, `AccessIfIndex`, `TunnelID`, `SessionID`, `PppInterface`, `IPv4Addr`, `IPv6InterfaceID`, `IPv6Prefix`, `AcctSessionID`, but no `CallingStationID` or `NASPortID`. `Registry` holds `Add`, `Remove`, `Get`, `All`, `Count`, `ByAccessType`, `LookupByAcctSessionID` (no atomic `Upsert`).
- [ ] `internal/component/l2tp/subscriber_bridge.go` (240 lines) - `onSessionUp` (line 89) and `onSessionIPAssigned` (line 181) set `sess.AcctSessionID = sess.ID` (internal canonical ID) and use full-copy `Get` then `Add`; `onAuthResult` and `onSessionIPReleased` also use `Get` then `Add`; `CallingStationID` from `SessionIPAssignedPayload` is not copied.
- [ ] `internal/component/l2tp/subscriber/cmd/subscriber.go` (187 lines) - `sessionFull` prints `s.AcctSessionID` under JSON key `acct-session-id` when non-empty.
- [ ] `internal/component/l2tp/pppoe/subsystem.go` (725 lines), `drain.go` (152 lines) - `subscriberSession` (line 547) and `onSessionUp` (line 600) write internal `id` into `AcctSessionID` and use `Get`/`Add`. Verified for narrow review item (3): `startPPPoEAuthDrain` stores `pendingAuthInfo{username}` before calling `AuthResponse`, `subscriberSession` populates `Username` on first insert, `onSessionIPAssigned` unconditionally sets `sess.Username = e.Username`, and `withdrawSubscriber` removes the registry entry on LCP-down, so `Username` is always populated before a PPPoE session is reachable in `subscriber.Registry`.
- [ ] `internal/plugins/cos/handler.go` (354 lines), `shared_access.go` (438 lines), `session_transaction.go` (500 lines) - both `applyLegacySharedProfile` and `applySessionChange` already call `access.write` before `keepSharedSession` stores `profileName` in `sessionStore`, and `prepareSessionChange` validates `coreCos.Lookup`. No code change is needed in `internal/plugins/cos/`.

**Behavior to preserve:**
- All already-landed RFC 5176 protections: source allow-list discard, Request Authenticator and Message-Authenticator checks, replay cache keyed by `(source IP, source port, Identifier)` + Authenticator, Event-Timestamp discard/404, Service-Type 405, Proxy-State and State opaque echo, and `confirmCoA` two-phase prepare/apply/commit/rollback with `doctor-radius-coa-apply` reporting.
- Exact `L2TPBirth` capture (`captureCoASession` / `l2tp.CaptureMatchedSession`) before CoA confirmation or Disconnect teardown on any matched L2TP session, so a replacement reusing `(tunnelID, sessionID)` can never be touched.
- Refusal (NAK 401) for the 4 unevaluable identification attributes (`Called-Station-Id`, `Acct-Multi-Session-Id`, `Chargeable-User-Identity`, `NAS-IPv6-Address`) on both request codes, and for `Framed-IP-Address`, `Framed-Interface-Id` and `Framed-IPv6-Prefix` on Disconnect-Request (RFC 5176 Section 3.6).
- Pure L2TP LNS CoS CoA (empty `AccessInterface`) staying NAK 506, and PPPoE Disconnect staying NAK 504 (until `spec-lifecycle-invariants` I-4 wires PPPoE teardown).

**Behavior to change:**
- S-2: Replace the split `findSubscriberSession` + `oneSession` / `findSessions` + `currentSessionIDs()` paths with one unified selector over `subscriber.Registry`; redefine `Session.AcctSessionID` as the wire `Acct-Session-Id` written by `acct.go` through a new atomic `Registry.Upsert`, add `CallingStationID` and `NASPortID` to `Session`, move `subscriber_bridge.go` and `pppoe/subsystem.go` onto `Registry.Upsert`, and stop both from writing internal IDs into `AcctSessionID`.
- S-3: Pass `NASIdentifier` and `SourceAddress` from `register.go::startCoAListener` into `coaListenerConfig`; accept and evaluate `NAS-IP-Address` and `NAS-Identifier` against the identity derived from `appendNASIdentity` (NAK 403 on mismatch), and accept and conjunctively evaluate `Calling-Station-Id` and `NAS-Port-Id` (both request codes) plus `Framed-IP-Address`, `Framed-Interface-Id` and `Framed-IPv6-Prefix` (CoA-Request only; Disconnect keeps 401), with wire-format length checks in `unsupportedAttrValue` (407).
- T-1: Validate `coreCos.Lookup(name)` in `coa.go::setCoSProfile` so an unknown CoS profile returns CoA-NAK 407 inside `readCoAChange` before session lookup, with zero bus events and no `doctor-radius-coa-apply` finding.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|------------------------------------------------------------------|
| CoA/DM packet admission, replay cache and attribute guards | `internal/component/l2tp/plugins/authradius/coa.go` (`admitRequest`, `answerRequest`, `coaSupportedAttrs`, `disconnectSupportedAttrs`, `fixedLengthAttrs`, `unsupportedAttr`, `unsupportedAttrValue`, `readCoAChange`, `setCoSProfile`) | Authenticates packets, deduplicates, checks Event-Timestamp and Service-Type, enforces allow-listed attributes (401) and fixed lengths (407), and parses rate/CoS changes without checking `coreCos.Lookup` | Extends: adds the 7 evaluable identification attribute types to the allow-lists (with CoA-only scoping for types 8, 96, 97) and length/format checks in `unsupportedAttrValue`, derives accepted identification types from one evaluator table, and checks `coreCos.Lookup` in `setCoSProfile` |
| Split CoA/DM session matchers and L2TP-only fallback | `coa.go` (`findSubscriberSession`, `sessionSelectorsMatch`, `oneSession`, `findSessions`), `acct.go` (`currentSessionIDs`), `subscriber/registry.go` (`LookupByAcctSessionID`) | Tries exact `LookupByAcctSessionID` + 3-selector conjunction first, then falls back to L2TP snapshot `MatchSessions` + `currentSessionIDs()`, blocking CoS on wire-ID L2TP lookups and hiding PPPoE from `User-Name` / multi-match checks | Replaces: deletes `findSubscriberSession`, `oneSession`, `findSessions`, `currentSessionIDs()` and `LookupByAcctSessionID`, replacing them with one selector over `subscriber.Registry` that checks NAS identity (403) and all supplied session identification attributes (503/508), then captures `L2TPBirth` via existing `captureCoASession` |
| Synchronous two-phase CoA transaction and CoA doctor health | `coa_transaction.go` (`confirmCoA`), `coa_health.go` (`recordCoAHealth`), `shaper/rate_transaction.go`, `internal/plugins/cos/session_transaction.go` | Prepares, applies, commits and compensates rate and CoS owners synchronously via `cl.cfg.Dispatch` before ACK; refuses unowned sessions (including PPPoE today) with 506 | Extends unchanged: every matched CoA (including L2TP CoS found by wire ID) flows through `applySubscriberCoA` -> `confirmCoA`; pre-validating `coreCos.Lookup` in `setCoSProfile` stops unknown profiles with 407 before `confirmCoA` runs |
| Subscriber session registry and transport writers | `internal/component/l2tp/subscriber/session.go`, `registry.go`, `internal/component/l2tp/subscriber_bridge.go`, `internal/component/l2tp/pppoe/subsystem.go`, `internal/component/l2tp/plugins/authradius/acct.go` | `Registry` provides `Get`/`Add`/`Remove`/`All`; bridge and PPPoE overwrite `AcctSessionID` with internal `Session.ID` via full-copy `Get`/`Add`; `acct.go` keeps wire `AcctSessionID` and `NASPortID` only in `a.sessions` | Extends: adds `CallingStationID`, `NASPortID` and atomic `Registry.Upsert` (with `insert` flag); `acct.go` writes/clears wire `AcctSessionID`/`NASPortID` via `Upsert`; bridge and PPPoE move onto `Upsert` and stop writing internal IDs into `AcctSessionID` |
| RADIUS NAS identity builder | `internal/component/l2tp/plugins/authradius/nasidentity.go` (`appendNASIdentity`, `hostNASIdentifier`) | Emits the single authoritative `NAS-IP-Address` and/or `NAS-Identifier` for `Access-Request` and `Accounting-Request` from `(nasID, sourceAddr)` | Extends: `startCoAListener` passes `cfg.NASIdentifier` and `cfg.SourceAddress` into `coaListenerConfig`, and the CoA/DM selector derives its expected NAS identity from `appendNASIdentity` so sent and matched identity are declared once |
| CoS profile registry | `internal/core/cos/cos.go` (`ParseFilterID`, `Lookup`) | `ParseFilterID` strips `cos:`; `Lookup` checks whether a named profile is configured | Extends: `coa.go::setCoSProfile` calls `coreCos.Lookup` to return 407 for unregistered profiles before session lookup |
| FreeRADIUS subscriber interop lab | `internal/le/interoplab/radiussubscriber/` (`radiussubscriber.go`, `checkers.go`, `check_coa_disconnect.go`) and `test/interop-radius-subscriber/scenarios/` | Runs FreeRADIUS 3.2.10 + `radclient` + `xl2tpd`/`pppd` LAC with kernel shaping and detail-log assertions | Extends: adds scenarios `l2tp-radius-coa-stale-session-freeradius` and `l2tp-radius-coa-identification-freeradius` into the existing suite |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | Ze is a platform, and other people's code runs against it | the door every change reads | "The RFC, then ExaBGP API compatibility, then ExaBGP's behavior, in that order" | agrees | Aligns CoA/DM NAS identity (403), conjunctive session identification (503/508), Disconnect attribute restrictions (401), and invalid CoS profile refusal (407) with RFC 5176 Sections 2.3, 3, 3.5 and 3.6 while preserving the command-dispatched `confirmCoA` process boundary |
| `ai/doors/access-security.md` | Who depends on it from outside | the door covering `coa.go`, `register.go`, `acct.go`, `subscriber/session.go`, `registry.go`, `subscriber_bridge.go`, `subscriber/cmd/subscriber.go`, `pppoe/subsystem.go`, and `authradius/*_test.go` | "CoA and Disconnect requests Ze answers" | agrees | Unifies CoA/DM selection over the shared `l2tp/subscriber` registry for both L2TP and PPPoE so real RFC 5176 dynamic authorization clients can address sessions using the exact attributes Ze sends in accounting |
| `ai/doors/le-tooling.md` | Boundaries | the door covering `internal/test/fixture/tunnel_fixture_l2tp_radius.go`, `tunnel_fixture_l2tp.go`, `internal/le/interoplab/radiussubscriber/`, and `test/l2tp/*.ci` | "A compiled Go fixture returns an error when an assertion fails" | agrees | Adds compiled fixture `l2tp/radius-coa-acct-id` returning explicit errors on assertion failure, two functional `.ci` tests, and two named scenarios in the existing `interop-radius-subscriber` lab with forced-red proof |
| `docs/research/l2tpv2-ze-integration.md` | 6.1 l2tp-auth (authentication and authorization) | the `// Design:` page of `coa.go`, `register.go`, `acct.go` | "Handle RADIUS CoA (Change of Authorization) for live sessions." | agrees | Keeps all CoA/DM selection, NAS identity comparison, and accounting wire-ID publication inside `l2tp-auth-radius` without adding RADIUS policy to the L2TP subsystem |
| `docs/architecture/traffic/cos-dynamic.md` | Consequences | anchored to `coa.go`; modified by this spec | "The CoA handler accepts a rate and a CoS Filter-Id in one request." | agrees | Extends `setCoSProfile` to validate profile existence via `coreCos.Lookup` and return NAK 407 before session lookup, preserves pure-LNS empty `AccessInterface` returning 506, and updates the Consequences paragraph in the same phase |
| `docs/architecture/config/transaction-protocol.md` | Phase 3: Rollback (conditional) | anchored to `register.go` | - | silent | This spec only copies `cfg.NASIdentifier` and `cfg.SourceAddress` into `coaListenerConfig` inside `startCoAListener` and does not modify `runPlugin`'s config transaction, deferred accounting acceptance, or rollback handling |
| `docs/architecture/doctor-and-health-checks.md` | Recent events: the report bus in show doctor | anchored to `register.go` | - | silent | This spec adds no doctor check or doctor code and does not change `register.go::init`; validating unknown CoS profiles in `setCoSProfile` prevents false `doctor-radius-coa-apply` backend-health findings on invalid attribute values |
| `docs/architecture/l2tp/bng-1-radius-attributes.md` | Decisions | anchored to `register.go`, `acct.go`, `subscriber/session.go`, `tunnel_fixture_l2tp.go` | "Pool allocation, timeouts, shaping, accounting and CoS use that ID for their metadata lookup; they do not rebuild it from the tunnel and session numbers." | agrees | Preserves canonical subscriber ID metadata lookup and `Calling-Station-Id` propagation on `SessionIPAssignedPayload` (carrying it onto `subscriber.Session.CallingStationID` so CoA evaluates the exact value accounting sends), `traffic.ParseFilterIDRate`, and `acctSession` Birth/endpoint ownership |
| `docs/architecture/l2tp/subscriber-session-model.md` | Packed L2TP identity value | the `// Design:` page of `subscriber/session.go`, `registry.go`, `subscriber_bridge.go`, `subscriber/cmd/subscriber.go`; anchored to `acct.go` and `pppoe/subsystem.go`; modified by this spec | "A selection by subscriber attribute has one walk and one capture, shared by the RADIUS CoA and Disconnect listener and the operator's `clear l2tp session user` and `clear l2tp session address`." | departs | Under owner-approved Q-2 (B) (2026-10-14), CoA/DM stops walking `l2tp.MatchSessions` (deleting `findSessions` and the L2TP-only fallback) and walks `subscriber.Registry` instead so one selector serves both L2TP and PPPoE; matched L2TP sessions still capture their original `L2TPBirth` through `CaptureMatchedSession`, while `clear l2tp session user/address` keeps `MatchSessions`. This spec updates `subscriber-session-model.md` in the same work |
| `docs/architecture/l2tp/bng-5-pppoe.md` | Decisions | the `// Design:` page of `pppoe/subsystem.go`; modified by this spec | "PADR computes the canonical subscriber ID once from the server's complete interface index and PPPoE session ID." | agrees | Preserves canonical `Session.ID` computed at PADR untouched; stops `pppoe/subsystem.go` from copying it into `AcctSessionID`, moves registry writes onto atomic `Upsert`, and documents PPPoE CoA/DM selection by `User-Name` and `Framed-IP-Address` |
| `docs/architecture/l2tp/bng-5-pppoe.md` | Design decisions | decision of 2026-11-23 on `bng-5-pppoe.md` | "D-1, a VPP API loss with the process alive, is resolved by the VPP lifecycle rules: a live VPP that Ze cannot adopt is killed and restarted, so its exit proof always arrives and retires the retained owners" | agrees | This spec edits only registry `AcctSessionID` writes and atomic `Upsert` calls in `pppoe/subsystem.go` and does not touch VPP API loss handling, process retirement, or retained VPP owners |
| `docs/architecture/l2tp/bng-5-pppoe.md` | Design decisions | decision of 2026-11-23 on `bng-5-pppoe.md` | "When Ze adopts the VPP, retained owners are read back by exact identity over the new connection of the same pidfd-proven process, and cleaned there (option C)" | agrees | This spec does not touch VPP connection adoption, pidfd proof, or Option C readback cleanup in `pppoe/dataplane/vpp/` |
| `docs/architecture/l2tp/bng-5-pppoe.md` | Design decisions | decision of 2026-11-23 on `bng-5-pppoe.md` | "Adoption happens only inside the single VPP loss timer: a reconnect to the same pidfd-proven process whose full readback matches (option C) keeps the sessions. A declared loss is never adopted: Ze kills the process through the retained pidfd, waits for its exit proof, restarts and reconfigures VPP as at boot, and PPPoE tears its sessions down per OD-2 and lets subscribers redial. The provider holds the process lease, which gives the identity proof, the kill and the exit proof. This narrows the row "When Ze adopts the VPP" to the single timer, and the D-1 row's "cannot adopt" to every declared loss" | agrees | This spec touches no VPP loss timer, process lease, or VPP restart path; PPPoE teardown and session redial semantics remain untouched |
| `docs/architecture/l2tp/bng-5-pppoe.md` | Design decisions | decision of 2026-11-23 on `bng-5-pppoe.md` | "D-1 (a VPP API loss with the process alive) resolves on the VPP lifecycle's two recovery forms. Form (1), inside the reconnect-and-verify limit that follows the failed alive test: the same pidfd-proven process answering with the data Ze configured is enough to adopt it, and the PPPoE sessions are kept. The retained owners are read back by exact identity over the new connection of that process and cleaned there (option C), as part of the background reconciliation; a mismatch it finds is repaired, never treated as a loss. Form (2), a forced restart: Ze kills the process through the retained pidfd, waits for its exit proof, restarts and reconfigures VPP as at boot, and PPPoE tears its sessions down per OD-2 and lets subscribers redial. The provider holds the process lease, which gives the identity proof, the kill and the exit proof. This supersedes the row "Adoption happens only inside the single VPP loss timer" (the single timer, adoption on a full readback match, and "a declared loss is never adopted"), and narrows the D-1 row and the row "When Ze adopts the VPP" to these two forms" | agrees | This spec touches neither Form (1) reconnect-and-verify adoption nor Form (2) forced VPP restart; its changes in `pppoe/subsystem.go` are limited to `subscriber.Registry` `Upsert` and leaving `AcctSessionID` empty until accounting issues a wire ID |
| `docs/architecture/testing/interop.md` | The FreeRADIUS subscriber suite | modified by this spec | "FreeRADIUS writes every accounting record it accepts to its `ze_detail` file, and its `radclient` is the Dynamic Authorization Client." | agrees | Adds the two new RFC 5176 scenarios directly into `test/interop-radius-subscriber/scenarios/` and `internal/le/interoplab/radiussubscriber/`, using the suite's `ze_detail` accounting log and `radclient` DAC |
| `docs/architecture/testing/interop.md` | Design decisions | decision of 2026-11-23 on `interop.md` | "Each Docker interop suite and scenario checks only the kernel capabilities it actually requires (`Suite.KernelCapabilities` unioned with selected `ScenarioPlan.KernelCapabilities`); suites whose selected scenarios require none run without unrelated kernel features (such as SRv6 or IPsec)" | agrees | The two new scenarios join the existing `interop-radius-subscriber` suite and use its declared `Suite.KernelCapabilities` without adding unrelated kernel capability probes or bypassing the fail-closed check |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A CoA-Request or Disconnect-Request UDP datagram arriving on `coa-port` (`coaListener.serve` -> `serveRequests` -> `admitRequest` -> `answerRequest`).
- The listener starts only when `coa-port` is configured and at least one RADIUS `server` resolves (`register.go::startCoAListener`), and `activateRadiusConfig` restarts it on every applied configuration.
- The wire `Acct-Session-Id` enters at `radiusAcct.onSessionIPAssigned` (`l2tpevents.SessionIPAssigned`) and is cleared at `radiusAcct.onSessionDown` (`l2tpevents.SessionDown`).

### Transformation Path
1. `admitRequest` and `answerRequest` authenticate, deduplicate, and check Event-Timestamp, Service-Type (405), `unsupportedAttr` (401), and `unsupportedAttrValue` (407). Both accepted attribute maps derive their identification attribute entries from one evaluator table (all 10 evaluated identification types for CoA; the 7 non-IP-framing identification types for Disconnect, excluding types 8, 96 and 97 per RFC 5176 Section 3.6; the 4 unevaluable types excluded from both). `unsupportedAttrValue` validates fixed lengths (`AttrNASPort: 4`, `AttrNASIPAddress: 4`, `AttrFramedIPAddress: 4`, `AttrFramedInterfaceID: 8`, `AttrServiceType: 4`, `AttrAcctTerminateCause: 4`, `AttrEventTimestamp: 4`) and RFC 3162 `AttrFramedIPv6Prefix` encoding (length 2..18 octets, prefix length 0..128, enough prefix octets for the prefix length).
2. CoA only: `readCoAChange` parses Filter-Id and Vendor-Specific attributes; `setCoSProfile` validates `coreCos.Lookup(name)` and returns 407 immediately if the profile does not exist, before session lookup and without calling `confirmCoA` or raising `doctor-radius-coa-apply`.
3. Unified selector (`selectSubscriberSession` inside `handleCoA` and `handleDisconnect`):
   - First evaluates every `NAS-IP-Address` and `NAS-Identifier` attribute in the request against the NAS identity derived from `appendNASIdentity(nil, cl.cfg.NASIdentifier, cl.cfg.SourceAddress)`. Any mismatch — including a NAS attribute kind Ze does not emit — immediately answers NAK with Error-Cause 403 (`ErrorCauseNASIdentificationMismatch`). A request carrying no NAS identification attribute proceeds.
   - Next checks that the request carries at least one session identification attribute; if none is present, answers NAK 503.
   - Walks `subscriber.LookupService().Registry.All()` and keeps every session satisfying EVERY occurrence of EVERY supplied session identification attribute: `Acct-Session-Id` (exact non-empty wire `Session.AcctSessionID`), `User-Name` (exact `Session.Username`, coordinated with `spec-l2tp-proxied-pap-interop-and-identity.md` D-3), `NAS-Port` (32-bit exact equality with `uint32(sessionID)` from `session.PPPKey()`, no uint16 truncation), `Calling-Station-Id` (exact `Session.CallingStationID`, or formatted MAC fallback for PPPoE if populated), `NAS-Port-Id` (exact `Session.NASPortID`), and on CoA-Request `Framed-IP-Address` (`Session.IPv4Addr`), `Framed-Interface-Id` (`Session.IPv6InterfaceID` non-zero), and `Framed-IPv6-Prefix` (`Session.IPv6Prefix` valid and equal).
   - Zero matches -> NAK 503 (`ErrorCauseSessionNotFound`); more than one match -> NAK 508 (`ErrorCauseMultiSessionUnsupported`); exactly one match -> proceed.
4. CoA on the single matched session: if `AccessType == subscriber.AccessL2TP`, capture the original row via `captureCoASession` (`L2TPBirth`) and verify `snapshot.SubscriberID == subSess.ID` (503 if stale), refresh `PppInterface`, then call `applySubscriberCoA` (`confirmCoA` two-phase apply/rollback -> observer emits -> CoA-ACK). If `AccessType == subscriber.AccessPPPoE`, call `applySubscriberCoA` (which refuses via `confirmCoA` with CoA-NAK 506 until `spec-lifecycle-invariants` AC-3 wires PPPoE owners).
5. Disconnect on the single matched session: if `AccessType == subscriber.AccessPPPoE`, answer Disconnect-NAK 504 (until `spec-lifecycle-invariants` I-4 wires PPPoE teardown). If `AccessType == subscriber.AccessL2TP`, capture via `captureCoASession` (`L2TPBirth`) and call `original.Teardown` (504 on failure or stopped service, Disconnect-ACK on confirmed teardown).
6. `sendResponse` echoes `Proxy-State`/`State` verbatim, computes Response Authenticator, sends UDP reply, and stores in replay cache (unchanged).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CoA listener <-> subscriber registry | `subscriber.LookupService().Registry.All()` read | Yes, `coa.go::findSubscriberSession` |
| Accountant <-> subscriber registry | `acct.go::onSessionIPAssigned` writes wire `AcctSessionID`, `CallingStationID`, `NASPortID` via `Registry.Upsert(..., true, ...)`; `onSessionDown` clears via `Registry.Upsert(..., false, ...)` | No, assumption A-2 |
| L2TP bridge & PPPoE <-> subscriber registry | Atomic `Registry.Upsert` replacing full-copy `Get` + `Add` | Yes, `subscriber_bridge.go`, `pppoe/subsystem.go` |
| CoA listener <-> RADIUS config | `startCoAListener` copies `cfg.NASIdentifier` and `cfg.SourceAddress` into `coaListenerConfig` | Yes, `register.go::startCoAListener` |
| CoA listener <-> shaper / CoS owners | Synchronous `confirmCoA` command dispatch (`request l2tp shaper rate`, `request class-of-service session`) + notification emits | Yes, `coa_transaction.go` |
| CoA listener <-> L2TP service | `captureCoASession` (`l2tp.CaptureMatchedSession`) -> `Snapshot` / `Teardown` | Yes, `coa.go::handleCoA`, `handleDisconnect` |

### Changed-Wire Census

**Search evidence:** scope `internal/component/l2tp/`, `internal/test/`, `internal/le/interoplab/radiussubscriber/`, `test/`. Terms: `coaSupportedAttrs`, `disconnectSupportedAttrs`, `AttrNASIPAddress`, `AttrNASIdentifier`, `AttrFramedIPAddress`, `AttrCallingStationID`, `AttrNASPortID`, `AttrFramedInterfaceID`, `AttrFramedIPv6Prefix`, `ErrorCauseNASIdentificationMismatch`, `setCoSProfile`, `acct-session-id`. Positive controls: `coa.go::unsupportedAttr` / `readCoAChange` / `sendResponse` found as the wire request reader and reply writer; `acct.go::buildAcctPacket` and `nasidentity.go::appendNASIdentity` found as the Accounting-Request writers whose attribute values a DAC echoes into CoA/Disconnect; `subscriber/cmd/subscriber.go::sessionFull` found as the CLI JSON writer of `acct-session-id`.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| RFC 5176 CoA-Request / Disconnect-Request & ACK/NAK replies | reader (requests) & writer (ACK/NAK) | `internal/component/l2tp/plugins/authradius/coa.go` (`answerRequest`, `unsupportedAttr`, `unsupportedAttrValue`, `readCoAChange`, `setCoSProfile`, `handleCoA`, `handleDisconnect`, `sendResponse`) | `coa.go` decodes incoming UDP `coa-port` requests via `radius.Decode` and encodes replies via `sendResponse` | Change: accept and evaluate 7 additional identification attributes (403 on NAS mismatch, 503/508 on session mismatch), keep 4 unevaluable types and Disconnect types 8/96/97 at 401, return 407 on unknown CoS profile in `setCoSProfile`, unify lookup on `subscriber.Registry` |
| RFC 5176 CoA-Request / Disconnect-Request & ACK/NAK replies | writer (requests) & reader (replies) | External Dynamic Authorization Client (operator RADIUS server / FreeRADIUS `radclient` in `internal/le/interoplab/radiussubscriber/check_coa_disconnect.go` and new interop checkers, plus `internal/test/fixture/tunnel_fixture_l2tp_radius.go`) | `check_coa_disconnect.go::dynamicRequest` runs `radclient` against `coaTarget` and parses `Received CoA-ACK/NAK` and `Error-Cause` | Preserve existing `radius-coa-disconnect-freeradius` and `radius-coa-duplicate-freeradius` (both send `Acct-Session-Id` + `Event-Timestamp` + `Message-Authenticator`, no foreign NAS identity); add new interop checkers and `.ci` fixture covering `T-S-<other>` and full accounting-copied identification sets |
| RFC 2866 Accounting-Request attributes | writer | `internal/component/l2tp/plugins/authradius/acct.go` (`buildAcctPacket`, `genSessionID`) and `nasidentity.go` (`appendNASIdentity`) | Emits `Acct-Session-Id`, `User-Name`, `NAS-Port`, `NAS-Port-Id`, `Calling-Station-Id`, `Framed-IP-Address`, `NAS-IP-Address` / `NAS-Identifier` | Preserve wire encoding byte-for-byte; `acct.go` additionally persists the emitted `AcctSessionID`, `CallingStationID` and `NASPortID` onto `subscriber.Registry` via atomic `Upsert` |
| RFC 2866 Accounting-Request attributes | reader | External RADIUS accounting server (FreeRADIUS `ze_detail` in `test/interop-radius-subscriber/mods-ze-detail`) | FreeRADIUS writes received accounting records to `ze_detail`, from which a DAC or interop checker copies session identifiers | Preserve: wire accounting format is unchanged; CoA now accepts and evaluates the exact attribute set FreeRADIUS records |
| `show subscriber id <id> detail` JSON (`acct-session-id` key) | writer | `internal/component/l2tp/subscriber/cmd/subscriber.go` (`sessionFull`) | Line 130-132 writes `m["acct-session-id"] = s.AcctSessionID` when non-empty | Change value semantics (Q-2 B): outputs the wire `Acct-Session-Id` (`T-S-bootTag-counter`) instead of internal `Session.ID`, and omits the key when no accounting ID was issued (including PPPoE until PPPoE accounting lands) |
| `show subscriber id <id> detail` JSON (`acct-session-id` key) | reader | Operator CLI / API / MCP clients and `internal/component/l2tp/subscriber/cmd/subscriber_test.go` | `grep` of `docs/` and `internal/` shows only `subscriber/cmd/subscriber.go` writes the key and no existing test or doc example asserts the old `l2tp-...` value | Update `subscriber_test.go` to assert the wire ID value and update `docs/architecture/l2tp/subscriber-session-model.md` |

### Integration Points
- `internal/component/l2tp/subscriber/` - unified `Registry` with atomic `Upsert`, `CallingStationID`, `NASPortID`, and wire `AcctSessionID`.
- `internal/component/l2tp/plugins/authradius/coa.go` & `register.go` - unified selector, `coaListenerConfig` NAS identity fields, `setCoSProfile` `coreCos.Lookup` check.
- `internal/component/l2tp/plugins/authradius/acct.go` - writes/clears wire `AcctSessionID` and `NASPortID` on `subscriber.Registry`; removes `currentSessionIDs()`.
- `internal/component/l2tp/subscriber_bridge.go` & `internal/component/l2tp/pppoe/subsystem.go` - atomic `Registry.Upsert` without clobbering `AcctSessionID`.
- `internal/core/cos/` - `Lookup` called by `setCoSProfile` (T-1).

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes (design) | Selector reads `subscriber.Registry`, which L2TP bridge, PPPoE subsystem and `acct.go` populate through atomic `Upsert`; L2TP execution still captures `L2TPBirth` via `captureCoASession` and confirms changes via `confirmCoA` |
| No unintended coupling (components stay isolated) | Yes (design) | `authradius` already imports `l2tp/subscriber` and `internal/core/cos`; `acct.go` writes via `subscriber.Registry`'s public `Upsert` API; no new import direction |
| No duplicated functionality (extends existing, does not recreate) | Yes (design) | Deletes `findSubscriberSession`, `oneSession`, `findSessions`, `currentSessionIDs()`, `LookupByAcctSessionID` and the L2TP-only fallback branch; derives expected NAS identity from `appendNASIdentity` |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Control plane, one request per DAC action |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | N-A | No new command, view, family or plugin handler |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes (design) | Searched `coa.go` for identification attribute lists (`coaSupportedAttrs`, `disconnectSupportedAttrs`, `fixedLengthAttrs`, `sessionSelectorsMatch`): all identification attribute membership derives from one evaluator descriptor table in `coa.go`, guarded by `TestCoAAcceptedIdentificationTypesAreAllEvaluated`, so accepted and evaluated sets cannot drift |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The 2026-09-01 signed extraction mappings remain accurate after the residual fixes | `rfc/extraction/rfc5176.json` signed 2026-09-01 | Signature masks stale evidence | Review affected sites, re-record discrimination records, and independently re-judge all 7 rows | unvalidated |
| A-2 | The registry holds an entry for every live L2TP session (or `Upsert(..., true, ...)` creates it safely) before `acct.go` writes the issued wire `Acct-Session-Id`, and `subscriber_bridge.go` preserves it | `subscriber_bridge.go` creates/updates at `onSessionUp` and `onSessionIPAssigned`; `acct.go` issues at `onSessionIPAssigned`; atomic `Upsert` makes either event order lossless | Issued wire ID is lost or invisible to CoA | `TestAcctIssuedSessionIDIsRecordedOnTheSubscriberSession` and `TestBridgeUpdateKeepsAcctSessionID` testing both event orders | unvalidated |
| A-3 | `subscriber_bridge.go` and `pppoe/subsystem.go` no longer overwrite `AcctSessionID` once moved onto `Registry.Upsert` | Verified at HEAD lines 89, 181 of `subscriber_bridge.go` and lines 547, 600 of `pppoe/subsystem.go` | A later `SessionUp` or `SessionIPAssigned` erases the wire ID | Unit tests emitting `SessionUp` after `acct.go` writes `AcctSessionID` | unvalidated |
| A-4 | The only readers of `Session.AcctSessionID` are `coa.go` and `subscriber/cmd/subscriber.go`, and `LookupByAcctSessionID` has no non-test caller other than `coa.go` | Verified by repo grep 2026-11-26 (`internal/component/l2tp/`, `internal/plugins/cos/`) | Removing `LookupByAcctSessionID` or redefining `AcctSessionID` breaks another caller | Re-grep entire `internal/` and `cmd/` at implementation | unvalidated |
| A-5 | `NAS-Port` emitted for L2TP is `uint32(sess.sessionID)` and for PPPoE is `uint32(sess.PPPoESID)` via `Session.PPPKey()` | Verified in `acct.go:858`, `coa.go:664`, `subscriber/session.go:81` | `NAS-Port` evaluation compares the wrong value | Unit tests for L2TP and PPPoE `NAS-Port` plus boundary tests at 0, 65535, 65536 | unvalidated |
| A-6 | Storing the resolved `NASPortID` on `subscriber.Session` when `acct.go` resolves it preserves exact match across a `nas-port-id-format` reload | `acct.go:75-79` already pins `sess.nasPortID` at accounting start for the same reason | Recomputing from current config would mismatch after a format reload | `TestAcctIssuedSessionIDIsRecordedOnTheSubscriberSession` asserts `NASPortID` survives a config format change | unvalidated |
| A-7 | `coa_transaction.go::confirmCoA` refuses a matched PPPoE CoA with 506 (never false ACK) while `spec-lifecycle-invariants` AC-3 is still open | Verified at HEAD in `coa_transaction.go:77-92` and `cos/session_transaction.go:211-214` (`ParseL2TPBirth` fails for PPPoE ID; shaper has no confirmed PPPoE session state) | A matched PPPoE CoA could send a false CoA-ACK | `TestRFC5176PPPoESessionSelectedByUserNameOrFramedIP` asserts CoA-NAK 506 before lifecycle AC-3 | unvalidated |
| A-8 | A real FreeRADIUS DAC copying attributes from Ze's Accounting-Request sends only attributes Ze evaluates | Verified in `acct.go::buildAcctPacket` and `nasidentity.go::appendNASIdentity` | Q-3 (i) refuses real accounting-copied requests | `l2tp-radius-coa-identification-freeradius` interop scenario | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Re-enabling 7 identification attributes in `coaSupportedAttrs` / `disconnectSupportedAttrs` requires updating `TestDynamicAuthorizationRefusesUnimplementedIdentification` and `TestRFC5176RequestNotRoutableCauseNeverSentForAForeignSession` (both RFC-tagged) | Edit hook refuses RFC-tagged test edit without `ze-test-judge` approval | Submit exact before/after rationale to independent `ze-test-judge` agent (`ai/META-DECISIONS.md`) and re-record via `./le rfc record` |
| R-2 | Sessions alive across an accounting reload/detach lose their issued ID if CoA reads `acctInstance.currentSessionIDs()` | Valid CoA/Disconnect after accounting reload returns 503 | Storing wire `AcctSessionID` and `NASPortID` on `subscriber.Session` in `subscriber.Registry` decouples live CoA lookup from `acct.go::detach()`; covered by `TestAcctIssuedSessionIDIsRecordedOnTheSubscriberSession` |
| R-3 | With accounting disabled, Ze issues no `Acct-Session-Id`, so an `Acct-Session-Id`-only request returns 503 | Disconnect with guessed ID returns 503 when accounting is off | Correct per RFC 5176 Section 3; documented in `docs/guide/l2tp.md`; `User-Name`, `NAS-Port`, `Framed-IP-Address`, `Calling-Station-Id` still select |
| R-4 | Q-3 (i) refuses a DAC that sends `Called-Station-Id`, `Acct-Multi-Session-Id`, `Chargeable-User-Identity`, or `NAS-IPv6-Address` with 401 | NAK 401 in operator log | Owner decided (i) on 2026-10-14; `docs/guide/l2tp.md` lists evaluated vs unsupported attributes |
| R-5 | NAS identity check refuses a DAC sending an arrival IP as `NAS-IP-Address` when `source-address` is unset, or a `NAS-Identifier` when `source-address` is set and `nas-identifier` is unset | NAK 403 on a request with valid session ID | Derived from `appendNASIdentity` so sent and checked NAS identity match; `docs/guide/l2tp.md` names the fix (`set l2tp auth radius source-address` or `nas-identifier`) |
| R-6 | Coordination with `spec-l2tp-proxied-pap-interop-and-identity.md` (D-3) on `coa.go` `User-Name` matching | Merge conflict or no-auth LAC-claim `User-Name` mismatch | Keep `User-Name` matching in a single helper on `subscriber.Session` so D-3 only extends that one predicate with the LAC claim field |
| R-7 | Fixture changes across shared CoA test helpers (`oneSessionService`, `twoSessionService`, `twoAliceSessions`, `coaTestListener`, `coaIncarnationFixture`) stale existing `rfc5176` discrimination records | `./le rfc check` reports stale `unit-sha` / `producer-sha` | Keep edits confined to shared helpers + explicitly listed behavior tests; run `ze-test-judge` for any tagged test body change and `./le rfc record` in the same phase |
| R-8 | Concurrent CoA and session teardown between registry selection and apply/teardown | Action targets a gone or replaced session | Preserve `captureCoASession` (`L2TPBirth`) on L2TP matches before `applySubscriberCoA` and `Teardown`, returning 503 on stale CoA and 504 on unremovable Disconnect |
| R-9 | Concurrent read-modify-write between `subscriber_bridge.go` / `pppoe/subsystem.go` (`Get` + `Add`) and `acct.go` writing wire `AcctSessionID` erases the issued ID | Live session returns 503 to its own wire `Acct-Session-Id` | Add atomic `Registry.Upsert(id string, insert bool, fn func(s *Session, existed bool)) bool` under `Registry.mu` write lock. All bridge paths (`onSessionUp`, `onAuthResult`, `onSessionIPAssigned`, `onSessionIPReleased`), `pppoe/subsystem.go` updates, and `acct.go` (`insert=true` on issue, `insert=false` on clear at `onSessionDown` so it never resurrects a `Remove`d entry) move onto `Upsert`. `onSessionUp` reads `wasActive` BEFORE mutating `State = StateActive` inside `Upsert` and emits `subevents.SessionUp` + `RecordSessionUp` AFTER `Upsert` returns (non-reentrant `Registry.mu`, synchronous engine subscribers). Note: the 2026-10-14 out-of-order `SessionIPAssigned`/`SessionDown` race is already fixed by `sessionNoticeTicket` (`plan/journal/late-write-lands-on-the-successor.md`) |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A valid CoA/Disconnect from a real FreeRADIUS DAC is refused (401/403/503/506), a CoS CoA for an L2TP session remains unreachable, an unknown CoS profile raises false backend-health warnings (506 instead of 407), or an un-narrowed selector acts on the wrong subscriber |
| How is it reverted? | Single commit revert per phase; no config schema, wire accounting format, or persistent storage change (`subscriber.Registry` is in-memory) |
| Who else touches this path? | `spec-l2tp-proxied-pap-interop-and-identity.md` (ready, `coa.go` User-Name matching for LAC claim), `spec-pppoe-subscribers-produce-no-accounting-or-telemetry.md` (design, `acct.go` PPPoE accounting), `spec-lifecycle-invariants.md` (ready, PPPoE Disconnect I-4 and PPPoE CoA owners AC-3) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A rate CoA-Request, then a Disconnect-Request, on UDP `coa-port` carrying a live session's captured `Acct-Session-Id` with a wrong suffix (`T-S-<other>`) | → | Unified `subscriber.Registry` selector in `coa.go` | `test/l2tp/radius-coa-stale-acct-session-id.ci` (daemon, live session, real UDP socket) |
| A CoA-Request on UDP `coa-port` carrying the wire `Acct-Session-Id` of an L2TP session with an `AccessInterface` and a registered CoS profile | → | Unified selector -> `captureCoASession` -> `applySubscriberCoA` -> `confirmCoA` | `TestRFC5176CoSChangeReachesAnL2TPSessionFoundByItsAcctSessionID` (real UDP listener socket) |
| A CoA-Request on UDP `coa-port` carrying an unregistered CoS profile (`cos:no-such-profile`) beside a valid rate | → | `readCoAChange` -> `setCoSProfile` -> `coreCos.Lookup` -> NAK 407 | `TestRFC5176UnknownCoSProfileIsNAKed` (real UDP listener socket) |
| A CoA-Request on UDP `coa-port` carrying `Acct-Session-Id` plus a mismatched `Calling-Station-Id` | → | Unified selector conjunctive attribute evaluation -> NAK 503 | `test/l2tp/radius-coa-identification-mismatch.ci` (daemon, live session, real UDP socket) |

## Acceptance Criteria

Note on retired ACs already landed at HEAD:
- Former AC-1 (a) (unit-level exact `Acct-Session-Id` match against reused `(T,S)`): already landed and proven by `TestCoAOldAccountingIdentityCannotSelectNewIncarnation` and `TestDisconnectOldAccountingIdentityCannotTerminateNewIncarnation`. AC-1 below preserves that invariant over the unified `subscriber.Registry` and completes the live daemon `.ci` and FreeRADIUS interop proof for `T-S-<other>`.
- Former AC-8 (T-2 atomic multi-change rollback on downstream apply failure): already landed via `coa_transaction.go::confirmCoA` and proven by `TestCoAMixedOwnerFailureRestoresBothControls` and `TestCoAMixedOwnersApplyBeforeACK` (`rfc5176_atomic_owners_test.go`). Preserved unchanged across the unified selector refactor.

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Live L2TP session issued wire `Acct-Session-Id` `T-S-bootTag-N` (recorded on `subscriber.Registry` via atomic `Upsert`). A rate CoA-Request, then a Disconnect-Request (CoA first so a teardown does not mask it), each carrying `T-S-<other>` (same tunnel and session numbers, wrong suffix) or an old incarnation's ID | Both requests receive CoA-NAK 503 and Disconnect-NAK 503 with zero teardowns and zero rate changes, and the session survives. Repeating both requests with the exact issued wire ID `T-S-bootTag-N` receives CoA-ACK (new rate installed) followed by Disconnect-ACK (session torn down). Survives an accounting config reload (`detach`) | None: RFC 5176 Section 3 forbids matching any session other than the exact issued context |
| AC-2 | A request carrying the session's exact wire `Acct-Session-Id` plus ONE mismatched value of `User-Name`, `NAS-Port`, `Calling-Station-Id`, `NAS-Port-Id` (both CoA and Disconnect), or `Framed-IP-Address`, `Framed-Interface-Id`, `Framed-IPv6-Prefix` (CoA-Request only); and each attribute alone or matching | Every mismatched combination answers NAK 503 with zero state change. Every matching value (combined or alone) selects the session and answers ACK with the change applied. A Disconnect-Request carrying `Framed-IP-Address`, `Framed-Interface-Id` or `Framed-IPv6-Prefix` answers Disconnect-NAK 401 regardless of value (RFC 5176 Section 3.6 "0 MUST NOT be present"). Malformed fixed lengths (`NAS-Port` != 4, `NAS-IP-Address` != 4, `Framed-IP-Address` != 4, `Framed-Interface-Id` != 8) or invalid `Framed-IPv6-Prefix` encoding answer NAK 407 | None: every supported identification attribute is mandatory and conjunctive (RFC 5176 Sections 2.3 and 3) |
| AC-3 | A CoA-Request or Disconnect-Request carrying `NAS-IP-Address` and/or `NAS-Identifier` against all four `appendNASIdentity` configurations (`source-address` only, `nas-identifier` only, both, neither) | A foreign `NAS-IP-Address` or `NAS-Identifier`, or a NAS attribute kind Ze does not emit under that configuration, answers NAK 403 (`ErrorCauseNASIdentificationMismatch`) with zero state change (never 503). Values matching what `appendNASIdentity` emits proceed to session selection; a request carrying no NAS identification attribute also proceeds | A request carrying no NAS identification attribute is permitted to proceed because RFC 5176 Section 3 makes NAS-IP-Address SHOULD and NAS-Identifier MAY |
| AC-4 | A CoA-Request or Disconnect-Request carrying any of the 4 unevaluable identification attributes (`Called-Station-Id`, `Acct-Multi-Session-Id`, `Chargeable-User-Identity`, `NAS-IPv6-Address`) beside valid matching session selectors | Answers NAK 401 (`ErrorCauseUnsupportedAttribute`) with zero state change for both request codes (Q-3 (i)). A drift-guard test (`TestCoAAcceptedIdentificationTypesAreAllEvaluated`) verifies every identification type in `coaSupportedAttrs` and `disconnectSupportedAttrs` has an evaluator in the selector | Only the 4 attributes Ze holds no fact for remain 401 per owner ruling Q-3 (i) |
| AC-5 | A CoA-Request for an L2TP session that has an `AccessInterface`, addressed by its wire `Acct-Session-Id` (`T-S-bootTag-N`), requesting a registered CoS profile (alone or beside a rate change) | Selected directly from `subscriber.Registry`, confirmed through `confirmCoA`, answered with CoA-ACK, and the requested VLAN QoS maps are installed on `AccessInterface` | A pure L2TP LNS session with an empty `AccessInterface` still answers CoA-NAK 506 because there is no local access VLAN (`docs/architecture/traffic/cos-dynamic.md`) |
| AC-6 | Supplied identification attributes match more than one live session in `subscriber.Registry`: (a) two L2TP sessions sharing `User-Name`; (b) one L2TP session and one PPPoE session sharing `User-Name` | Both cases answer CoA-NAK 508 / Disconnect-NAK 508 (`ErrorCauseMultiSessionUnsupported`) with zero changes and zero teardowns | None: Ze does not support multi-session dynamic authorization changes (RFC 5176 Section 2.3) |
| AC-7 | A CoA-Request whose CoS profile (`cos:<name>` Filter-Id or vendor CoS VSA) is not registered in `internal/core/cos`, alone or beside a valid rate Filter-Id | Answered with CoA-NAK 407 (`ErrorCauseInvalidAttributeValue`) before session lookup, with zero backend calls, zero bus events, and no `doctor-radius-coa-apply` runtime finding. A registered profile continues to proceed to session selection and `confirmCoA` | Registered profiles whose backend apply fails still return 506 via `confirmCoA`, because 506 is the RFC 5176 cause for a backend resource failure rather than an invalid attribute value |
| AC-9 | Termination-Action (RFC 2865 Section 5.29, Q-6 owner decision 2026-10-14) | `rfc/extraction/rfc5176.json` site 3.3:7 remains explicitly `feature-out-of-scope`; no reauthentication path is added and no completion is inferred from absence of attribute 29 | Explicit optional-feature exclusion retained by owner decision Q-6 |
| AC-10 | RFC 5176 conformance evidence after all edits | Selector and value tests prove positive and negative polarities, every modified RFC-tagged test is approved by independent `ze-test-judge`, every staled discrimination record is refreshed with `./le rfc record`, rows RFC5176-3.3-1, -2.3-5, -2.3-6, -2.3-7, -3.6-1, -3-1 and -3.5-8 are independently re-judged, and `./le rfc check` passes for `rfc5176` | None: no stale record or unjudged row is permitted |
| AC-11 | Operator and architecture documentation (`docs/guide/l2tp.md`, `docs/architecture/l2tp/subscriber-session-model.md`, `docs/architecture/l2tp/bng-5-pppoe.md`, `docs/architecture/traffic/cos-dynamic.md`, `docs/architecture/testing/interop.md`) | Updated in the same phase as each behaviour change: `docs/guide/l2tp.md` documents the 10 evaluated identification attributes, 4 unsupported (401), NAS identity mismatch (403 with `source-address` / `nas-identifier` fix), unified registry selection (including PPPoE by `User-Name` / `Framed-IP-Address`), unknown CoS profile 407, and updated source anchors; `subscriber-session-model.md` updates wire `AcctSessionID`, atomic `Upsert`, `show subscriber`, and the CoA/DM walk departure; `bng-5-pppoe.md` documents PPPoE CoA/DM selection; `cos-dynamic.md` documents 407 in `setCoSProfile`; `interop.md` documents the new scenarios | None: docs stay synchronized with producing code |
| AC-12 | A live PPPoE session in `subscriber.Registry` addressed by `User-Name` or (on CoA-Request only) `Framed-IP-Address`, and no `Acct-Session-Id` | The unified selector finds the PPPoE entry. A Disconnect-Request answers Disconnect-NAK 504 (until `spec-lifecycle-invariants` I-4 wires PPPoE teardown); a CoA-Request enters `applySubscriberCoA` -> `confirmCoA` and answers CoA-NAK 506 (never a false CoA-ACK) until `spec-lifecycle-invariants` AC-3 wires PPPoE owners. A Disconnect-Request carrying `Framed-IP-Address` answers 401 (AC-2). Before `spec-pppoe-subscribers-produce-no-accounting-or-telemetry.md` issues wire `Acct-Session-Id` for PPPoE, an `Acct-Session-Id`-only request answers 503; once that spec writes wire `Acct-Session-Id` through `Registry.Upsert`, the same selector matches PPPoE by wire `Acct-Session-Id` too | PPPoE Disconnect 504 and PPPoE CoA 506 remain until `spec-lifecycle-invariants` I-4 / AC-3 land; PPPoE `Acct-Session-Id` 503 remains until `spec-pppoe-subscribers-produce-no-accounting-or-telemetry.md` lands |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | An operator's RADIUS server sends a rate CoA and a Disconnect carrying an `Acct-Session-Id` with a wrong suffix (`T-S-<other>`), then repeats with the real issued ID `T-S-bootTag-N` | UDP `coa-port` -> `answerRequest` -> unified `subscriber.Registry` selector -> 503 on wrong suffix (session & rate untouched), ACK + apply/teardown on real ID | `test/l2tp/radius-coa-stale-acct-session-id.ci`; interop `l2tp-radius-coa-stale-session-freeradius` |
| 2 | A DAC changes an L2TP subscriber's rate and CoS profile using the wire `Acct-Session-Id` from FreeRADIUS accounting | UDP `coa-port` -> `readCoAChange` (`coreCos.Lookup` validates profile) -> unified selector -> `captureCoASession` -> `applySubscriberCoA` -> `confirmCoA` -> CoA-ACK | `TestRFC5176CoSChangeReachesAnL2TPSessionFoundByItsAcctSessionID`; `TestRFC5176UnknownCoSProfileIsNAKed` |
| 3 | A DAC addresses a subscriber using the full attribute set FreeRADIUS copied from accounting (`Acct-Session-Id`, `User-Name`, `NAS-Port`, `NAS-Port-Id`, `Calling-Station-Id`, `Framed-IP-Address`, `NAS-IP-Address`/`NAS-Identifier`), and retries with one mismatched identifier | UDP `coa-port` -> NAS identity check (403 on mismatch) -> unified selector -> ACK on matching set, 503 on mismatched identifier | `TestRFC5176EveryIdentificationAttributeNarrowsTheMatch`; `test/l2tp/radius-coa-identification-mismatch.ci`; interop `l2tp-radius-coa-identification-freeradius` |

## 🧪 TDD Test Plan

### Unit Tests (new; file `internal/component/l2tp/plugins/authradius/rfc5176_session_selector_test.go` unless named)

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC5176StaleAcctSessionIDMatchesNoSession` | selector file | AC-1 negative over unified `subscriber.Registry`, both reused-(T,S) ID and wrong suffix `T-S-<other>`, plus survival across accounting `detach` (R-2) | |
| `TestRFC5176CurrentAcctSessionIDMatchesItsSession` | selector file | AC-1 positive over unified `subscriber.Registry` | |
| `TestRFC5176EveryIdentificationAttributeNarrowsTheMatch` | selector file | AC-2 table over `User-Name`, `NAS-Port`, `Calling-Station-Id`, `NAS-Port-Id` (both codes), `Framed-IP-Address`, `Framed-Interface-Id`, `Framed-IPv6-Prefix` (CoA only), both match/mismatch polarities, plus Disconnect-Request 401 for types 8, 96, 97 and 407 for malformed fixed/IPv6-prefix lengths | |
| `TestRFC5176ForeignNASIdentityIsAMismatch` | selector file | AC-3 over all four `appendNASIdentity` configurations (`source-address` only, `nas-identifier` only, both, neither): foreign or unemitted NAS attribute -> NAK 403 (never 503); matching -> proceeds; absent -> proceeds | |
| `TestRFC5176UnevaluableIdentificationAttributeIsUnsupported` | selector file | AC-4 (Q-3 i), RFC5176-2.3-5: `Called-Station-Id`, `Acct-Multi-Session-Id`, `Chargeable-User-Identity`, `NAS-IPv6-Address` -> NAK 401 on both codes | |
| `TestCoAAcceptedIdentificationTypesAreAllEvaluated` | selector file | AC-4 drift guard: every identification type in `coaSupportedAttrs` and `disconnectSupportedAttrs` has an evaluator in the selector table | |
| `TestRFC5176CoSChangeReachesAnL2TPSessionFoundByItsAcctSessionID` | selector file | AC-5: wire `Acct-Session-Id` selects L2TP session with `AccessInterface` and installs CoS maps via `confirmCoA` -> CoA-ACK (red at HEAD: 506) | |
| `TestRFC5176MultipleMatchAcrossAccessTypesIsNAKed` | selector file | AC-6, RFC5176-2.3-7: two L2TP sessions and L2TP + PPPoE session sharing `User-Name` -> NAK 508 | |
| `TestRFC5176PPPoESessionSelectedByUserNameOrFramedIP` | selector file | AC-12: PPPoE registry entry selected by `User-Name` (CoA and Disconnect) and by `Framed-IP-Address` (CoA only; Disconnect -> 401); asserts Disconnect -> 504, CoA -> 506 (never false ACK before `spec-lifecycle-invariants` AC-3), and `Acct-Session-Id`-only request -> 503 | |
| `TestRFC5176UnknownCoSProfileIsNAKed` | `rfc5176_coa_values_test.go` | AC-7, RFC5176-2.3-5: unregistered `cos:no-such-profile` (Filter-Id and vendor CoS VSA, alone and beside valid rate) -> CoA-NAK 407, zero backend/bus calls, zero `doctor-radius-coa-apply` finding (red at HEAD: 506) | |
| `TestAcctIssuedSessionIDIsRecordedOnTheSubscriberSession` | `internal/component/l2tp/plugins/authradius/acct_session_id_registry_test.go` | Q-2 B: wire `AcctSessionID`, `CallingStationID`, `NASPortID` written at `onSessionIPAssigned`, cleared at `onSessionDown` with `insert=false`, survive `detach` (R-2) and config `nas-port-id-format` reload (A-6), not overwritten by later `SessionUp` (A-2, A-3) | |
| `TestBridgeUpdateKeepsAcctSessionID` | `internal/component/l2tp/subscriber_bridge_test.go` | R-9: `onSessionUp`, `onAuthResult`, `onSessionIPAssigned`, `onSessionIPReleased` interleaved with accountant's wire ID write in both orders preserve both sides' fields; `subscriber.RecordSessionUp` fires once on first up | |
| `TestRegistryUpsertKeepsFieldsItDidNotChange` | `internal/component/l2tp/subscriber/registry_test.go` | R-9: atomic `Upsert` modifies only mutated fields; `insert=true` inserts when missing and reports `existed=false`; existing reports `existed=true`; `insert=false` after `Remove` does not recreate the entry | |
| `TestSubscriberDetailShowsWireAcctSessionID` | `internal/component/l2tp/subscriber/cmd/subscriber_test.go` | Q-2 B: `show subscriber id <id> detail` emits wire `acct-session-id` when non-empty and omits key when empty | |

### Tagged tests that change (governed by independent `ze-test-judge`, `ai/META-DECISIONS.md`)

Per `ai/META-DECISIONS.md` ("Test changes"), no manual owner terminal commands are used: every RFC-tagged test whose body changes is reviewed and approved by an independent agent running `ze-test-judge` (producing `RFC-approved:` commit trailers), and every staled discrimination record is refreshed by `./le rfc record`.

Behaviour or body changes at HEAD:

| Test | File | Tag (verdict) | Change | Why |
|------|------|---------------|--------|-----|
| `TestDynamicAuthorizationRefusesUnimplementedIdentification` | `rfc5176_identification_test.go` | RFC5176-2.3-5 negative (`enforced`) | Narrow the 401 table from all 11 types to the 4 unevaluable types (`Called-Station-Id`, `Acct-Multi-Session-Id`, `Chargeable-User-Identity`, `NAS-IPv6-Address`) on both codes, plus the 3 CoA-only types (`Framed-IP-Address`, `Framed-Interface-Id`, `Framed-IPv6-Prefix`) on Disconnect-Request | The 7 remaining types are now implemented and evaluated by AC-2 and AC-3 (tested positively and negatively in `TestRFC5176EveryIdentificationAttributeNarrowsTheMatch` and `TestRFC5176ForeignNASIdentityIsAMismatch`) |
| `TestRFC5176RequestNotRoutableCauseNeverSentForAForeignSession` | `rfc5176_section_rows_test.go` | RFC5176-3.5-8 negative (`enforced`) | Currently sends `NAS-Identifier: "other-nas"` and expects `[401]`; update expectation to `[403]` (NAS Identification Mismatch, still never 502) or set matching NAS-Identifier to assert `[503]` | Under AC-3, `NAS-Identifier` is now evaluated and a foreign NAS identity answers 403 per RFC 5176 Section 3.5 |
| `TestRFC5176DisconnectCarryingOnlyIdentificationIsACKed` | `rfc5176_section_rows_test.go` | RFC5176-3-1 negative (`enforced`) | Extend the request to include the newly evaluated Disconnect identification attributes (`Calling-Station-Id`, `NAS-Port-Id`, listener `NAS-Identifier`) and update claim wording | Proves every evaluated Disconnect identification attribute is admitted without triggering a 401/403/503 NAK |
| `TestCoALiveAccountingOwnerIdentitySurvivesRegistryMiss` | `rfc5176_account_identity_test.go` | RFC5176-3.3-1 positive (`enforced`) | Rebase onto Q-2 (B): `acct.go` writes the wire `Acct-Session-Id` onto `subscriber.Registry` (there is no longer a split registry-miss fallback), proving a live accountant-issued ID reaches `subscriber.Registry` and selects the session | Replaces the deleted L2TP-only fallback assumption with the unified registry contract |
| `TestRFC5176CoANAKMakesNoPartialChange` | `rfc5176_coa_branches_test.go` | RFC5176-2.3-6 positive (`enforced`) | Seed the session in `subscriber.Registry` with empty `AccessInterface` (and ensure `"gold"` is registered in `coreCos`) so the 506 still comes from missing `AccessInterface` rather than unregistered profile 407 or missing session 503 | Preserves exact premise (rate + CoS on session without `AccessInterface` -> 506 with no partial rate change) after L2TP-only fallback removal and `coreCos.Lookup` validation |

Fixture-helper updates (shared helpers only; test bodies stay unchanged unless listed above):
- `oneSessionService`, `coaTestListener` (`rfc5176_walk_test.go`), `twoAliceSessions` (`rfc5176_coa_branches_test.go`), `twoSessionService` / `newWindowListener` (`rfc5176_section_rows_test.go`), `fakeL2TPService` (`rfc5176_coa_listener_test.go`), `coaIncarnationFixture` (`rfc5176_account_identity_test.go`), `newCoACollisionFixture` (`coa_captured_collision_test.go`), and the inline fakes in `rfc5176_coa_test.go` and `disconnect_service_stopped_test.go` seed matching `subscriber.Session` entries (with wire `AcctSessionID`, `L2TPBirth`, `Username`, `TunnelID`, `SessionID`) into `subscriber.DefaultRegistry` (cleaned up via `t.Cleanup`).
- Because `coa.go` (the producer) changes, `./le rfc record` refreshes every staled discrimination record in `rfc/discrimination/rfc5176.json`.

### Boundary Tests (numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `NAS-Port` | 32-bit unsigned, compared to `LocalSID` / `PPPoESID` 1..65535 | 65535 matches SID 65535 | 0 matches no session | 65536 matches no session (no uint16 truncation) |
| `Framed-IPv6-Prefix` prefix length | 0..128 bits (RFC 3162 Section 2.3), total attr length 2..18 octets | 128 bits (18 octets) | len < 2 -> NAK 407 | prefix-len > 128 or len > 18 or prefix octets < ceil(prefix-len/8) -> NAK 407 |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `radius-coa-stale-acct-session-id.ci` | `test/l2tp/` | Live L2TP session with `coa-port` and accounting to new fixture `l2tp/radius-coa-acct-id` capturing Accounting-Start ID `T-S-bootTag-N`: rate CoA then Disconnect with `T-S-<other>` both answer 503 with session surviving and rate unchanged; then rate CoA and Disconnect with `T-S-bootTag-N` answer CoA-ACK (rate updated) and Disconnect-ACK (session ended) | |
| `radius-coa-identification-mismatch.ci` | `test/l2tp/` | Live L2TP session with `Calling-Station-Id`; CoA with right `Acct-Session-Id` + wrong `Calling-Station-Id` answers 503 with shaper rate unchanged (red at HEAD: 401); CoA with matching `Acct-Session-Id` + `Calling-Station-Id` answers CoA-ACK and applies rate | |

### Interop Tests (Scope: protocol)

| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `l2tp-radius-coa-stale-session-freeradius` | `test/interop-radius-subscriber/scenarios/` (existing `interop-radius-subscriber` lab) | FreeRADIUS 3.2.10 (`radclient` + `ze_detail`), `xl2tpd`/`pppd` LAC | `radclient` reads `Acct-Session-Id` from `ze_detail`, sends rate CoA then Disconnect with mutated suffix `T-S-<other>`: CoA-NAK 503 (kernel `tc` rate unchanged) and Disconnect-NAK 503 (session survives, ping works); then repeats with real ID: CoA-ACK (kernel `tc` updated) and Disconnect-ACK (Accounting-Stop recorded, `pppd` ends) | |
| `l2tp-radius-coa-identification-freeradius` | `test/interop-radius-subscriber/scenarios/` | FreeRADIUS 3.2.10 (`radclient` + `ze_detail`), `xl2tpd`/`pppd` LAC | `radclient` sends the full attribute set FreeRADIUS recorded in `ze_detail` (`Acct-Session-Id`, `User-Name`, `NAS-Port`, `NAS-Port-Id`, `Calling-Station-Id`, `Framed-IP-Address`, `NAS-IP-Address`/`NAS-Identifier`) -> CoA-ACK and kernel `tc` updated (red at HEAD: 401); same request with mismatched `Framed-IP-Address` or `Calling-Station-Id` -> CoA-NAK 503 and `tc` unchanged; same request with foreign `NAS-Identifier` -> CoA-NAK 403 and `tc` unchanged | |

Each scenario records a forced red: revert the selector producer, rebuild the Ze container image, observe the scenario fail for the named reason, restore, and observe green.

## Files to Modify
- `internal/component/l2tp/plugins/authradius/coa.go` - add `NASIdentifier` and `SourceAddress` to `coaListenerConfig`; unify `coaSupportedAttrs`/`disconnectSupportedAttrs` identification entries from one evaluator table (10 for CoA, 7 for Disconnect, 4 unevaluable excluded from both); add fixed-length and `Framed-IPv6-Prefix` checks in `unsupportedAttrValue`; call `coreCos.Lookup` in `setCoSProfile` (407); replace `findSubscriberSession`, `oneSession`, `findSessions` and the L2TP-only fallback with one selector over `subscriber.Registry` (NAS identity check via `appendNASIdentity` -> 403, then conjunctive session identification -> 503/508/1, preserving `captureCoASession` on L2TP matches); add RFC 5176 Sections 3, 3.5 and 3.6 quotes above enforcing code
- `internal/component/l2tp/plugins/authradius/register.go` - `startCoAListener` copies `cfg.NASIdentifier` and `cfg.SourceAddress` into `coaListenerConfig` (AC-3)
- `internal/component/l2tp/plugins/authradius/acct.go` - `onSessionIPAssigned` writes wire `AcctSessionID`, `CallingStationID`, and resolved `NASPortID` via atomic `Registry.Upsert(..., true, ...)`; `onSessionDown` clears `AcctSessionID` and `NASPortID` via `Registry.Upsert(..., false, ...)`; delete `currentSessionIDs()`; update `genSessionID` comment
- `internal/component/l2tp/subscriber/session.go`, `internal/component/l2tp/subscriber/registry.go` - redefine `Session.AcctSessionID` as the wire `Acct-Session-Id`; add `CallingStationID` and `NASPortID` fields; add atomic `Registry.Upsert(id string, insert bool, fn func(*Session, bool)) bool`; delete `LookupByAcctSessionID`
- `internal/component/l2tp/subscriber_bridge.go` - stop writing `Session.ID` into `AcctSessionID`; move `onSessionUp`, `onAuthResult`, `onSessionIPAssigned`, and `onSessionIPReleased` onto atomic `Registry.Upsert` (reading `wasActive` before mutating and emitting/recording after unlock); copy `p.CallingStationID` in `onSessionIPAssigned`
- `internal/component/l2tp/subscriber/cmd/subscriber.go` - update comment beside `acct-session-id` stating it holds the wire `Acct-Session-Id` (absent when none was issued); add test in `subscriber_test.go`
- `internal/component/l2tp/pppoe/subsystem.go` - stop writing internal `id` into `AcctSessionID` (`subscriberSession`, `onSessionUp`); move registry writes onto atomic `Registry.Upsert`
- `internal/component/l2tp/plugins/authradius/*_test.go` - update shared test helpers to seed `subscriber.Registry` and update the 5 tagged units in "Tagged tests that change" with independent `ze-test-judge` approval
- `internal/test/fixture/tunnel_fixture_l2tp_radius.go`, `internal/test/fixture/tunnel_fixture_l2tp.go` - add fixture `l2tp/radius-coa-acct-id`
- `internal/le/interoplab/radiussubscriber/radiussubscriber.go` - register checkers for `l2tp-radius-coa-stale-session-freeradius` and `l2tp-radius-coa-identification-freeradius` in the existing suite
- `docs/guide/l2tp.md` - update CoA/DM check table, identification prose (10 evaluated, 4 refused 401, NAS identity 403, PPPoE selection, unknown CoS profile 407), and source anchors (AC-11)
- `docs/architecture/l2tp/subscriber-session-model.md` - update CoA/DM selection paragraph (departure from `MatchSessions` walk), wire `AcctSessionID`, `CallingStationID`, `NASPortID`, atomic `Upsert`, `show subscriber`, and remove `LookupByAcctSessionID` anchor
- `docs/architecture/l2tp/bng-5-pppoe.md` - document PPPoE CoA/DM selection by `User-Name` and `Framed-IP-Address`
- `docs/architecture/traffic/cos-dynamic.md` - add `setCoSProfile` `coreCos.Lookup` 407 refusal to Consequences
- `docs/architecture/testing/interop.md` - add the two new scenarios to the `interop-radius-subscriber` table
- `rfc/audit/rfc5176.json` - independently re-judged verdicts for the 7 rows; `rfc/discrimination/rfc5176.json` (tool-written via `./le rfc record`)

## Files to Create
- `internal/component/l2tp/plugins/authradius/rfc5176_session_selector_test.go` - AC-1..AC-6 and AC-12 unit tests
- `internal/component/l2tp/plugins/authradius/acct_session_id_registry_test.go` - wire `AcctSessionID` / `NASPortID` registry lifecycle tests
- `internal/le/interoplab/radiussubscriber/check_coa_selection.go` - checkers for the two new FreeRADIUS subscriber interop scenarios
- `test/l2tp/radius-coa-stale-acct-session-id.ci`, `test/l2tp/radius-coa-identification-mismatch.ci`
- `test/interop-radius-subscriber/scenarios/l2tp-radius-coa-stale-session-freeradius/`, `test/interop-radius-subscriber/scenarios/l2tp-radius-coa-identification-freeradius/`

### Integration Checklist

| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No config or RPC leaf added; `coa-port`, `nas-identifier`, `source-address` already exist |
| YANG validation constraints | N-A | No new leaf |
| YANG custom validators | N-A | No new leaf |
| CLI commands/flags | No | No new command or flag |
| CLI grammar (keyword before value) | N-A | No new command |
| Editor autocomplete | N-A | No new command |
| Functional test for new RPC/API | Yes | `test/l2tp/radius-coa-stale-acct-session-id.ci`, `test/l2tp/radius-coa-identification-mismatch.ci` |
| Pipe completeness | N-A | No new command output |
| Env var registration | No | None |
| Doctor check for runtime dependencies | No | No new path, socket, port, module or binary; CoA doctor checks already exist (`authradius/doctor.go`, `coa_health.go`) |
| Doctor code per problem class | No | No new problem class; pre-validating unknown CoS profiles as 407 prevents false `doctor-radius-coa-apply` findings on invalid attribute values |
| Prometheus counters/metrics | No | None added |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not BGP |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Defect fixes in existing RFC 5176 CoA/DM feature |
| 2 | Config syntax changed? | No | None |
| 3 | CLI command added/changed? | Yes (output value) | `show subscriber id <id> detail`'s `acct-session-id` changes from internal `Session.ID` to wire `Acct-Session-Id` (omitted when none was issued); documented in `docs/architecture/l2tp/subscriber-session-model.md` |
| 4 | API/RPC added/changed? | No | None |
| 5 | Plugin added/changed? | No | Existing plugin behavior only; no registration change |
| 6 | Has a user guide page? | Yes | `docs/guide/l2tp.md` CoA/DM section (AC-11) |
| 7 | Wire format changed? | No | Standard RFC 5176 packets; corrected Error-Cause and attribute evaluation |
| 8 | Plugin SDK/protocol changed? | No | None |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/audit/rfc5176.json`, `rfc/discrimination/rfc5176.json`; regenerate `docs/features/rfc-status.md` if any verdict moves |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/interop.md` (`The FreeRADIUS subscriber suite` table); `docs/functional-tests.md` if fixtures are listed there |
| 11 | Affects daemon comparison? | No | None |
| 12 | Internal architecture changed? | Yes | `docs/architecture/l2tp/subscriber-session-model.md` (CoA/DM unified registry walk, wire `AcctSessionID`, atomic `Upsert`, `CallingStationID`, `NASPortID`, removed `LookupByAcctSessionID`), `docs/architecture/l2tp/bng-5-pppoe.md` (PPPoE CoA/DM selection), `docs/architecture/traffic/cos-dynamic.md` (407 on unknown profile in `setCoSProfile`) |
| 13 | Route metadata keys added/changed? | No | None |
| 14 | Prometheus counters added/changed? | No | None |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | None |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived via `./le spec design documents`: `docs/research/l2tpv2-ze-integration.md`, `docs/architecture/traffic/cos-dynamic.md`, `docs/architecture/l2tp/subscriber-session-model.md`, `docs/architecture/l2tp/bng-5-pppoe.md`, `docs/architecture/l2tp/bng-1-radius-attributes.md` (unaffected), `docs/architecture/config/transaction-protocol.md` (unaffected), `docs/architecture/doctor-and-health-checks.md` (unaffected), `docs/guide/l2tp.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | No | CoA config examples unchanged; no doc shows `show subscriber` with `acct-session-id` (grep verified) |

### Discovery (`ai/rules/repo-maintenance.md` mechanical checklist)
| Question | Answer |
|----------|--------|
| Where does an agent look first? | `ai/CODE-TO-DOCS.md` row for `coa.go` -> `docs/guide/l2tp.md` CoA/DM section |
| What rule prevents regression? | `ai/rules/rfc-compliance.md` (RFC 5176 Section 3/3.5/3.6 quotes above the selector), tagged tests with fresh discrimination records |
| What registry prevents drift? | The single identification-attribute evaluator table in `coa.go`, checked against both accepted sets by `TestCoAAcceptedIdentificationTypesAreAllEvaluated` |
| What verification proves it? | Scoped unit tests, the two `.ci` tests, the two FreeRADIUS interop scenarios with forced reds, and `./le rfc check` |

## Design presentation

### What changes

RADIUS CoA and Disconnect requests move from a split two-table lookup (`findSubscriberSession` plus an L2TP-only `findSessions` fallback) to one unified selector over `subscriber.Registry` serving both L2TP and PPPoE sessions. `Session.AcctSessionID` is redefined as the wire `Acct-Session-Id` that `acct.go` actually sends to RADIUS (written and cleared through a new atomic `Registry.Upsert` so bridge and PPPoE updates never overwrite it), and `subscriber.Session` also stores `CallingStationID` and `NASPortID`. The listener now accepts and evaluates all 7 additional identification attributes Ze produces (`NAS-IP-Address` and `NAS-Identifier` compared against `appendNASIdentity`, answering 403 on mismatch; `Calling-Station-Id` and `NAS-Port-Id` on both codes; `Framed-IP-Address`, `Framed-Interface-Id` and `Framed-IPv6-Prefix` on CoA only, keeping Disconnect at 401), keeps the 4 unevaluable identification attributes at 401, and checks `coreCos.Lookup` in `setCoSProfile` so an unknown CoS profile is refused with CoA-NAK 407 before session lookup and without raising a backend-health finding.

### Before and after

1. **CoA with a real FreeRADIUS accounting attribute set (`Framed-IP-Address`, `Calling-Station-Id`, `NAS-Port-Id`, `NAS-Identifier`)**:
   - Before: `radclient` copies the accounting attributes from `ze_detail` and sends a rate CoA. Ze refuses with `CoA-NAK` `Error-Cause = 401 (Unsupported-Attribute)` because those 7 identification attributes are absent from `coaSupportedAttrs`.
   - After: Ze verifies `NAS-Identifier` / `NAS-IP-Address` against `appendNASIdentity` (answering `Error-Cause = 403` on mismatch), matches all session identification attributes conjunctively in `subscriber.Registry` (answering `503` on mismatch or `508` on multiple matches), confirms the change through `confirmCoA`, and answers `CoA-ACK`.
2. **CoA changing CoS profile on an L2TP session with an `AccessInterface`, addressed by wire `Acct-Session-Id`**:
   - Before: `subscriber.Registry` stores internal `Session.ID` in `AcctSessionID`, so the wire `Acct-Session-Id` misses `findSubscriberSession`, falls into the L2TP-only fallback, and answers `CoA-NAK` `Error-Cause = 506` ("no access interface").
   - After: `acct.go` records the wire `Acct-Session-Id` on the registry entry via `Registry.Upsert`; the unified selector finds the entry with its `AccessInterface`, captures `L2TPBirth`, confirms VLAN QoS maps through `confirmCoA`, and answers `CoA-ACK`.
3. **CoA naming an unregistered CoS profile (`Filter-Id = "cos:no-such-profile"`)**:
   - Before: `setCoSProfile` accepts the name, `confirmCoA` fails in CoS prepare, and Ze answers `CoA-NAK` `Error-Cause = 506 (Resources-Unavailable)` while raising a `doctor-radius-coa-apply` runtime doctor warning.
   - After: `setCoSProfile` calls `coreCos.Lookup`, rejects the unknown profile inside `readCoAChange` with `CoA-NAK` `Error-Cause = 407 (Invalid-Attribute-Value)`, touches no session or backend, and raises no doctor warning.
4. **`show subscriber id <id> detail` output for `acct-session-id` and PPPoE selection**:
   - Before: `acct-session-id` prints the internal canonical `Session.ID` (never sent on the wire), and a PPPoE session cannot be selected by `User-Name` or `Framed-IP-Address` (answers 503) or trigger 508 alongside an L2TP session with the same `User-Name`.
   - After: `acct-session-id` prints the wire `Acct-Session-Id` (`T-S-bootTag-counter`) when accounting issued one and is omitted otherwise; PPPoE sessions are matched by `User-Name` or `Framed-IP-Address` (CoA only), returning 504 on Disconnect, 506 on CoA until `spec-lifecycle-invariants` AC-3 wires PPPoE owners, and 508 when sharing a `User-Name` with an L2TP session.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Keep an authradius-owned issued-ID table beside `l2tp.Snapshot()` (Q-2 option A) | Preserves two disagreeing session tables, leaves `Session.AcctSessionID` holding an internal ID no RADIUS packet carries, keeps L2TP CoS unreachable by wire ID, and misses cross-access-type 508; owner chose Q-2 (B) on 2026-10-14 |
| Compare `NAS-IP-Address` against the UDP arrival destination address (`IP_PKTINFO`) | Requires socket-level `ReadMsgUDP` + `IP_PKTINFO` plumbing and matches an address Ze never announced in Access-Request or Accounting-Request; deriving expected identity from `appendNASIdentity` declares NAS identity once |
| Accept the 4 unevaluable identification attributes (`Called-Station-Id`, `Acct-Multi-Session-Id`, `Chargeable-User-Identity`, `NAS-IPv6-Address`) and answer 503/403 (Q-3 option ii) | Tells the DAC operator "session not found" for a live session instead of 401 naming the unsupported attribute to stop sending; owner chose Q-3 (i) on 2026-10-14 |
| Keep `setCoSProfile` trusting any `cos:` name and rely on `confirmCoA` prepare refusal | Answers 506 (`Resources-Unavailable`) instead of RFC 5176 Section 3.5's 407 (`Invalid-Attribute-Value`) and pollutes `show doctor` with a `doctor-radius-coa-apply` backend-health finding on an operator typo |
| Keep `Get` + `Add` in `subscriber_bridge.go` / `pppoe/subsystem.go` without atomic `Upsert` | Concurrent writes from `acct.go` and the bridge race and erase the issued wire `AcctSessionID` (R-9) |

### Documents departed from

- `docs/architecture/l2tp/subscriber-session-model.md`, section `Packed L2TP identity value`:
  - Existing statement: "A selection by subscriber attribute has one walk and one capture, shared by the RADIUS CoA and Disconnect listener and the operator's `clear l2tp session user` and `clear l2tp session address`."
  - Owner ruling: Thomas approved Q-2 (B) on 2026-10-14 ("Do what you think is right", accepting the recommendation to make `subscriber.Registry` the sole table the CoA/DM matcher reads and delete the L2TP-only fallback).
  - What the page says after this work: `clear l2tp session user` and `clear l2tp session address` keep walking `l2tp.MatchSessions`, while the RADIUS CoA and Disconnect listener walks `subscriber.Registry` across both L2TP and PPPoE and still calls `CaptureMatchedSession` (`L2TPBirth`) before acting on any matched L2TP session.

### Choices for Thomas

none

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- failing wiring tests at HEAD
   - Tests: `TestRFC5176CoSChangeReachesAnL2TPSessionFoundByItsAcctSessionID`, `TestRFC5176UnknownCoSProfileIsNAKed`, `TestRFC5176EveryIdentificationAttributeNarrowsTheMatch`, `TestRFC5176ForeignNASIdentityIsAMismatch`, `TestRFC5176PPPoESessionSelectedByUserNameOrFramedIP`, `radius-coa-stale-acct-session-id.ci` and `radius-coa-identification-mismatch.ci` with fixture `l2tp/radius-coa-acct-id`
   - Files: test files and fixture
   - Verify: each new test fails at HEAD for the exact open defect named in this spec
2. **Phase: Wire `AcctSessionID` and atomic `Upsert` on `subscriber.Registry` (Q-2 B, R-9)**
   - Tests: `TestRegistryUpsertKeepsFieldsItDidNotChange`, `TestBridgeUpdateKeepsAcctSessionID`, `TestAcctIssuedSessionIDIsRecordedOnTheSubscriberSession`, `TestSubscriberDetailShowsWireAcctSessionID`
   - Files: `subscriber/session.go`, `subscriber/registry.go`, `subscriber_bridge.go`, `pppoe/subsystem.go`, `acct.go`, `subscriber/cmd/subscriber.go`, `docs/architecture/l2tp/subscriber-session-model.md`, `docs/architecture/l2tp/bng-5-pppoe.md`
   - Verify: A-2, A-3, A-4, A-6 validated; bridge and PPPoE never clobber wire `AcctSessionID`; `detach()` preserves wire ID on live registry sessions
3. **Phase: Unified selector, 7 additional identification attributes, NAS identity 403, and unknown CoS profile 407 (S-2, S-3, T-1, AC-1..AC-7, AC-12)**
   - Tests: AC-1..AC-7 and AC-12 units, the 5 `ze-test-judge`-approved tagged-test edits, both `.ci` tests
   - Files: `coa.go`, `register.go`, `acct.go` (delete `currentSessionIDs()`), `subscriber/registry.go` (delete `LookupByAcctSessionID`), `authradius/*_test.go`, `docs/guide/l2tp.md`, `docs/architecture/traffic/cos-dynamic.md`
   - Verify: `TestCoAAcceptedIdentificationTypesAreAllEvaluated` green; every staled discrimination record refreshed with `./le rfc record`; `./le rfc check` green for `rfc5176`
4. **Phase: FreeRADIUS subscriber interop scenarios**
   - Tests: `l2tp-radius-coa-stale-session-freeradius`, `l2tp-radius-coa-identification-freeradius` in existing `interop-radius-subscriber` lab
   - Files: `internal/le/interoplab/radiussubscriber/radiussubscriber.go`, `check_coa_selection.go`, scenario directories under `test/interop-radius-subscriber/scenarios/`, `docs/architecture/testing/interop.md`
   - Verify: forced red observed by reverting selector producer and rebuilding image, then restored green
5. **Phase: Evidence and independent re-judge (AC-9, AC-10)**
   - Files: `rfc/audit/rfc5176.json`, `rfc/discrimination/rfc5176.json`
   - Verify: independent re-judge of RFC5176-3.3-1, -2.3-5, -2.3-6, -2.3-7, -3.6-1, -3-1 and -3.5-8; site 3.3:7 remains `feature-out-of-scope`; `./le rfc check` green

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-12 has an implementation at file:symbol and passing tests |
| Feature completeness | Every user story has a working path from UDP `coa-port` through unified selection to backend apply/teardown |
| Correctness | Exact whole-string `Acct-Session-Id` comparison; 32-bit `NAS-Port` without uint16 truncation; repeated identification attributes evaluated conjunctively; `Framed-IPv6-Prefix` validated per RFC 3162 Section 2.3; `wasActive` read before mutation in `Upsert` |
| Data flow | Wire `AcctSessionID`, `CallingStationID`, `NASPortID` live once on `subscriber.Registry`; `currentSessionIDs()` and `LookupByAcctSessionID` are deleted; no L2TP-only fallback remains (`ai/rules/no-layering.md`) |
| Operator errors and doctor codes | Unknown CoS profile returns 407 before `confirmCoA` and raises no `doctor-radius-coa-apply`; genuine backend apply/rollback failures continue to return 506 and raise `doctor-radius-coa-apply` |
| Rule: rfc-compliance | RFC 5176 Sections 3, 3.5 and 3.6 quoted above the selector and NAK branches; `ze-test-judge` approval for every modified RFC-tagged test; `./le rfc record` for every staled record |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Unified registry selector, atomic `Upsert`, wire `AcctSessionID`, 7 new identification attributes, NAS 403, CoS 407 | `./le test unit pkg path ./internal/component/l2tp/subscriber/` + `./le test unit pkg path ./internal/component/l2tp/` + `./le test unit pkg path ./internal/component/l2tp/plugins/authradius/` |
| Live daemon functional proof | `./le test functional l2tp` (`radius-coa-stale-acct-session-id.ci`, `radius-coa-identification-mismatch.ci`, `radius-coa-listener.ci`) |
| FreeRADIUS interop proof with forced reds | `./le test integration interop-radius-subscriber` (`l2tp-radius-coa-stale-session-freeradius`, `l2tp-radius-coa-identification-freeradius`, `radius-coa-disconnect-freeradius`) |
| RFC 5176 conformance evidence | `./le rfc check` green for `rfc5176`, fresh discrimination records, independent re-judge of the 7 rows |
| Documentation & source anchors | `./le doc check all` |

### Security Review Checklist

| Check | What to look for |
|-------|------------------|
| Input validation | Every identification attribute is compared as exact data, never parsed into a tunnel/session lookup key; malformed fixed lengths and invalid `Framed-IPv6-Prefix` lengths draw 407 before matching |
| Wrong-target action | No request acts on a session unless every NAS and session identification attribute matches and exactly one session matches; L2TP execution still binds `L2TPBirth` via `captureCoASession` |
| Information leak | 503 for a stale or mismatched session ID is indistinguishable from "no such session"; 403 confirms only a mismatch against the NAS identity the sender itself supplied; unauthenticated packets continue to be silently discarded |
| No false ACK on unowned PPPoE | A matched PPPoE CoA fails `confirmCoA` with 506 (never ACK) until `spec-lifecycle-invariants` AC-3 wires PPPoE owners |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Test fails on behavior mismatch | Re-read source in Current Behavior |
| A-2/A-3 broken (registry entry missing or clobbered) | DESIGN: inspect `Registry.Upsert` callers and event subscription order |
| `ze-test-judge` refuses a tagged-test edit | Fix the producing code or test assertion rather than weakening what the test proves |
| 3 fix attempts failed | STOP. Report all 3 approaches to the user |

## Design Insights

- Rebase audit (2026-11-26): S-1 unit exact match, T-2 (`confirmCoA`), `cos/handler.go` store-after-write, AC-4 401 exclusion, 3-attribute `sessionSelectorsMatch`, `sessionNoticeTicket`, and the `interop-radius-subscriber` lab all landed between 2026-10-14 and 2026-11-24. Narrowing this spec to the actual residual defects removes stale dependencies (`spec-lifecycle-invariants` AC-4 and `spec-radius-rfc-defects`) and keeps edits strictly focused on S-2, S-3, T-1 (407), AC-12, and live `.ci`/interop proof.
- Verified at producer (2026-11-26): PPPoE `Username` is stored in `pendingAuth` before `AuthResponse` (`pppoe/drain.go:51-56`), populated on first insert in `subscriberSession`, unconditionally updated in `onSessionIPAssigned` (`pppoe/subsystem.go:640`), and cleared via `Remove` on LCP-down (`pppoe/subsystem.go:711`), resolving the 2026-10-14 narrow review check.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|

None: the 2026-10-14 row in `plan/journal/late-write-lands-on-the-successor.md` was fixed by `sessionNoticeTicket`, and the 2026-11-11 row in `plan/journal/rollback-forgets-partial-apply.md` was fixed by `confirmCoA`.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One session table: `subscriber.Registry` with wire `AcctSessionID` (Q-2 B, owner 2026-10-14) | Authradius-owned issued-ID map (`currentSessionIDs()`) beside `l2tp.Snapshot()` | Fixes S-2 at its root cause (a field holding an internal ID no RADIUS packet carries), makes L2TP CoS reachable by wire ID (AC-5), enables cross-access-type 508 (AC-6) and PPPoE selection (AC-12), and survives accounting reload (R-2) |
| Atomic `Registry.Upsert` with `insert` flag (R-9) | `Get` + mutate + `Add` | Prevents `subscriber_bridge.go` and `pppoe/subsystem.go` from overwriting wire `AcctSessionID` / `NASPortID` written by `acct.go`, while `insert=false` on clear prevents resurrecting a removed session |
| Unevaluable identification attributes stay refused 401 (Q-3 i, owner 2026-10-14) | Accept and NAK 503/403; ignore | Ignoring breaches RFC 5176 Section 3; 401 tells the DAC operator which unsupported attribute to stop sending |
| Expected NAS identity derived from `appendNASIdentity` (AC-3) | Read packet destination IP via `IP_PKTINFO` | Declares NAS identity once across Access-Request, Accounting-Request and CoA/DM with no socket plumbing change |
| Validate `coreCos.Lookup` inside `coa.go::setCoSProfile` (AC-7) | Rely on `confirmCoA` CoS prepare refusal | Returns RFC 5176 Section 3.5's 407 (`Invalid-Attribute-Value`) before session lookup instead of 506 (`Resources-Unavailable`) and avoids raising false `doctor-radius-coa-apply` warnings |

## Known Limitations
- A pure L2TP LNS session has no local `AccessInterface`, so a CoS CoA for it stays NAK 506 (`docs/architecture/traffic/cos-dynamic.md`).
- A DAC that identifies the NAS by an arrival IP or attribute kind Ze does not emit via `appendNASIdentity` receives NAK 403 (or 401 for `NAS-IPv6-Address`); the operator sets `source-address` or `nas-identifier` to match (AC-3, R-5).
- PPPoE Disconnect teardown (504 today) and PPPoE CoA backend owners (506 today via `confirmCoA`) belong to `plan/release/fixes/spec-lifecycle-invariants.md` (I-4, AC-3).
- PPPoE RADIUS accounting and wire `Acct-Session-Id` issuance belong to `plan/release/fixes/spec-pppoe-subscribers-produce-no-accounting-or-telemetry.md`; once it writes wire `AcctSessionID` via `Registry.Upsert`, PPPoE sessions become selectable by `Acct-Session-Id` automatically.
- With accounting unconfigured, Ze issues no `Acct-Session-Id`, so a DAC must select by `User-Name`, `NAS-Port`, `Calling-Station-Id`, or `Framed-IP-Address` (R-3).
- Termination-Action stays excluded (Q-6, owner 2026-10-14).

## RFC Documentation (Scope: protocol)

Every MUST/MUST NOT this spec enforces carries `// RFC 5176 Section X.Y: "<quoted requirement>"` directly above the enforcing code, and every tagged test added or modified carries a fresh discrimination record written by `./le rfc record`.

| RFC / section | Default configuration and required behavior | Explicit options / extensions to preserve | Tests of default behavior | Tests preserving options / extensions |
|---------------|---------------------------------------------|-------------------------------------------|---------------------------|---------------------------------------|
| RFC 5176 Section 3 & Section 3.5 (NAS & session identification) | With `coa-port` enabled, evaluate every supplied NAS identification attribute against `appendNASIdentity` (403 on mismatch) and every supplied session identification attribute conjunctively across `subscriber.Registry` (503 none, 508 several, 1 match) | Explicit `nas-identifier`, `source-address`, and `nas-port-id-format` leaves determine the exact NAS identity and `NAS-Port-Id` matched | `TestRFC5176StaleAcctSessionIDMatchesNoSession`, `TestRFC5176EveryIdentificationAttributeNarrowsTheMatch`, `TestRFC5176ForeignNASIdentityIsAMismatch`, `TestRFC5176MultipleMatchAcrossAccessTypesIsNAKed` | `TestRFC5176ForeignNASIdentityIsAMismatch` (all 4 `source-address` / `nas-identifier` combinations), `TestAcctIssuedSessionIDIsRecordedOnTheSubscriberSession` (`nas-port-id-format` reload) |
| RFC 5176 Section 2.3 & Section 3.6 (mandatory attributes, Disconnect restrictions, invalid values) | Treat all attributes as mandatory: 401 for the 4 unevaluable identification attributes and for Disconnect types 8, 96, 97; 407 for malformed lengths or unregistered CoS profiles; preserve `confirmCoA` atomic apply/rollback | `coa-require-message-authenticator` opt-in leaf and explicit `Filter-Id` (`rate:` / `cos:`) & vendor VSA encodings preserved | `TestRFC5176UnevaluableIdentificationAttributeIsUnsupported`, `TestCoAAcceptedIdentificationTypesAreAllEvaluated`, `TestRFC5176UnknownCoSProfileIsNAKed`, `TestRFC5176CoSChangeReachesAnL2TPSessionFoundByItsAcctSessionID` | `TestRFC5176MessageAuthenticatorAbsentIsAcceptedByDefault`, `TestCoAListenerMissingMessageAuthenticatorDroppedWhenRequired`, `TestCoAMixedOwnersApplyBeforeACK`, `TestCoAMixedOwnerFailureRestoresBothControls` |

## Design Review Log

| Date | Round | Outcome |
|------|-------|---------|
| 2026-10-14 | owner answers & rounds 1-6 | Q-1..Q-8 answered by owner ("Do what you think is right"); rounds 1-6 converged on atomic `Registry.Upsert`, `appendNASIdentity` 403, CoA-only IPv4/IPv6 framing selectors, and PPPoE selection; 4 narrow follow-ups noted for AC-12 |
| 2026-11-26 | rebase to HEAD & design-gate alignment | Retired already-landed S-1 unit match, T-2 (`confirmCoA`), `cos/handler.go` store-after-write, AC-4 exclusion, 3-attribute `sessionSelectorsMatch`, `sessionNoticeTicket`, and `interop-radius-subscriber` lab into `Already landed at HEAD` and `Existing Mechanisms`; resolved all 4 narrow 2026-10-14 items (added `TestRFC5176PPPoESessionSelectedByUserNameOrFramedIP`, confirmed PPPoE CoA answers 506 via `confirmCoA` before lifecycle AC-3, verified PPPoE `Username` population at producer, added `bng-5-pppoe.md` and `l2tp.md` docs); replaced obsolete 58-command Q-5 owner table with `ze-test-judge`; documented coordination with `spec-l2tp-proxied-pap-interop-and-identity.md` and `spec-pppoe-subscribers-produce-no-accounting-or-telemetry.md`; removed banned `./le verify` line; added `## Design documents`, `### Changed-Wire Census`, `## Design presentation`, `## Owned journal rows`, and 4-column `## Acceptance Criteria` |

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
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-12 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches, plus `./le test integration interop-radius-subscriber` and `./le rfc check`. No agent runs any `./le verify` command: GitHub CI verifies the pushed commit, and the full verification is Thomas's release check
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/fixes/spec-rfc5176-remaining-defect-classes.md` only, in the same `./le commit create` script
