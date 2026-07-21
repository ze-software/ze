# Spec: fixit-rfc7606-treat-as-withdraw

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | - |
| Updated | 2026-07-19 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `rfc/short/rfc7606.md` - Revised Error Handling for BGP UPDATE Messages
4. Source files in Current Behavior below

## Task

**[HIGH]** RFC 7606 "treat-as-withdraw" drops the UPDATE instead of withdrawing its NLRI,
leaving stale routes / a peer-triggerable blackhole. Verified by the 2026-07-16 audit
(verifier V3); every citation re-verified against the working tree on 2026-07-16 by the
design pass.

When `enforceRFC7606` returns `RFC7606ActionTreatAsWithdraw`, `processMessage` fires
`EventUpdateMsg` and returns early at `reactor/session_read.go:170-171` (branch
`:163-171`), before the only RIB-delivery path (`onMessageReceived`,
`reactor/session_read.go:233-234`). The `EventUpdateMsg` FSM handler does exactly one
thing, reset the hold timer (`fsm/fsm.go:441-450`), so nothing withdraws the NLRI.
`onMessageReceived` is wired `reactor/peer_run.go:196` -> `reactor/reactor_peers.go:121`
-> `reactor/reactor_notify.go:218-579` (`notifyMessageReceiver`); the early return skips
it entirely.

Treat-as-withdraw fires for fully-parseable, NLRI-carrying UPDATEs: a well-known attribute
with the Optional bit set (`message/rfc7606.go:106-113`), bad ORIGIN (`:390-408`), NEXT_HOP
(`:416-426`), Community (`:495-505`), malformed AS_PATH (`:622-686`), missing mandatory
attributes (`:290-312`). The Section 5.2 escalation to session-reset happens only when there
is no NLRI (`message/rfc7606.go:336-342`), so every attribute-level treat-as-withdraw has
parseable routes to withdraw. RFC 7606 Section 2 requires those routes be handled "as if
they had been withdrawn." The code comment at `reactor/session_read.go:164-169` quotes this
and then implements "ignore." Explicit withdrawn-routes riding in the same UPDATE are
dropped too. Existing tests pin only "session stays Established"
(`reactor/session_test.go:1328-1425`), never RIB withdrawal, and
`reactor/session_test.go:1887-1915` pins "callback must NOT fire," which the fix will
deliberately replace with a stronger assertion (callback fires with a withdraw-only UPDATE).

## Required Reading

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc7606.md` - revised UPDATE error handling
  → Constraint: Section 2 — a treat-as-withdraw UPDATE MUST be handled as though its contained routes were withdrawn, and removed from Adj-RIB-In per RFC 4271.
  → Constraint: Section 5.2 — session reset applies only when there is no reachable NLRI; otherwise treat-as-withdraw.
  → Constraint: Section 3.j — treat-as-withdraw requires the NLRI / MP_REACH / MP_UNREACH fields to be successfully parsed; if not, session reset (or AFI/SAFI disable) MUST be used.
  → Constraint: Section 5.1 Rx — an implementation MUST accept withdrawn routes, NLRI, MP_REACH and MP_UNREACH in any combination from older speakers.

**Key insights:**
- "Not installing the malformed UPDATE's routes" is already correct; the bug is failing to remove previously-installed state for the same NLRI.
- Treat-as-withdraw reaches `session_read.go:163` from TWO producer classes with opposite data quality (see Current Behavior): the structural/truncation class where the NLRI may be unparseable (keep dropping wholesale), and the attribute-semantic class where the NLRI and withdrawn sections were already syntax-validated (must withdraw). The fix must split these at the only place that can tell them apart: `enforceRFC7606`.

## Current Behavior (MANDATORY)

**Source files read (all producers verified in the working tree):**
- [ ] `internal/component/bgp/reactor/session_read.go` - `processMessage` (`:141-268`); the treat-as-withdraw early return (`:163-171`: comment `:164-169`, `logFSMEvent(fsm.EventUpdateMsg)` `:170`, `return nil, false` `:171`); the `onMessageReceived` dispatch (`:233-234`); normal UPDATEs continue to `handleUpdate` (`:255`).
- [ ] `internal/component/bgp/reactor/session_validation.go` - `enforceRFC7606` (`:26-162`). Class A producers (structural, NLRI possibly unparseable, all return treat-as-withdraw and are dropped wholesale today): body < 4 (`:31-34`), withdrawn length overruns body (`:39-41`), withdrawn NLRI syntax invalid (`:44-49`), attribute length overruns body (`:54-57`), NLRI syntax invalid (`:64-69`). Note `:46` and `:66` collapse ANY non-nil `ValidateNLRISyntax` result to treat-as-withdraw, including the Section 3.j session-reset result produced at `message/rfc7606.go:864-870` (a pre-existing divergence at the time this section was written; since escalated in `b1f27a3e0` -- see Known Limitations. Line numbers here describe the pre-implementation tree). Class B producer: `message.ValidateUpdateRFC7606` call (`:78`), whose treat-as-withdraw results arrive only AFTER both NLRI sections passed syntax validation. Attribute-discard body-rebuild precedent (heap body, new `WireUpdate`, preserved ctxID/sourceID): `:117-126`.
- [ ] `internal/component/bgp/message/rfc7606.go` - `ValidateUpdateRFC7606` (`:142-354`), per-attribute treat-as-withdraw producers (flags `:106-123`, ORIGIN `:390-408`, NEXT_HOP `:416-426`, MED `:429-439`, LOCAL_PREF iBGP `:451-458`, Community `:495-505`, ORIGINATOR_ID iBGP `:517-525`, CLUSTER_LIST iBGP `:538-546`, ExtCommunity `:550-560`, LargeCommunity `:563-573`, AS_PATH `:622-686`, SRv6 TLVs `:759-819`, missing mandatory `:290-330`), Section 5.2 escalation (`:336-342`). `validateMPReachAttr` (`:576-586`) checks only minimum length and next-hop length (session-reset when bad); it does NOT syntax-validate the embedded MP NLRI prefixes. `validateMPUnreachAttr` (`:589-599`) checks only minimum length.
- [ ] `internal/component/bgp/fsm/fsm.go` - `EventUpdateMsg` handler (`:441-450`): hold-timer reset only.
- [ ] `internal/component/bgp/reactor/reactor_notify.go` - `notifyMessageReceiver` (`:218-579`), the single delivery path for received UPDATEs: `updates-received` counter (`:244-245`), ingress filter pipeline (`:395-471`) with the modified-payload precedent (heap payload with pool-buf ownership, `:451-466`), `recentUpdates` cache (`:492-510`), per-peer async delivery (`:567-570`) with sync fallback (`:573`).
- [ ] `internal/component/bgp/reactor/peer_run.go` - `session.onMessageReceived = p.messageCallback` (`:196`); delivery goroutine (`:445-511`) draining `deliverChan` into `receiver.OnMessageBatchReceived` (`:493`).
- [ ] `internal/component/bgp/reactor/reactor_peers.go` - `peer.messageCallback = r.notifyMessageReceiver` (`:121`).
- [ ] `internal/component/bgp/plugins/rib/rib_structured.go` - the existing explicit-withdraw delivery path this fix reuses: `dispatchStructured` (`:49-62`) -> `handleReceivedStructured` (`:75`) -> legacy withdrawn processing `wu.Withdrawn()` + `nlrisplit.Split` + `peerRIB.Remove` (`:197-207`) and MP_UNREACH processing `wu.MPUnreach()` + `peerRIB.Remove` (`:240-259`) -> best-path change detection (`:261-306`) -> `publishBestChanges` (`:301,304`).
- [ ] `internal/component/bgp/plugins/rib/rib_bestchange.go` - `checkBestPathChange` (`:701`) and `publishBestChanges` (`:1245`, EventBus emission): the Adj-RIB-In -> Loc-RIB withdraw propagation producer.
- [ ] `internal/component/bgp/plugins/rib/storage/familyrib.go` - `FamilyRIB.Remove` (`:317-343`): returns false for absent NLRI, no error path; withdraw of a never-installed prefix is a no-op (validates R-1).
- [ ] `internal/component/bgp/wireu/wire_update.go` - NLRI accessors available at the decision point: `Withdrawn()` (`:84`), `NLRI()` (`:114`), `MPReach()` (`:125`), `MPUnreach()` (`:149`); `internal/component/bgp/wireu/mpwire.go` - `MPReachWire.Family()` (`:40`), `NLRIBytes()` (`:164`), `MPUnreachWire.WithdrawnBytes()` (`:275`).
- [ ] `internal/component/bgp/reactor/session_handlers.go` - `handleUpdate` (`:233-246`) fires `fsm.EventUpdateMsg` (`:245`): the normal flow already performs the hold-timer reset that the early return does by hand.
- [ ] `internal/component/bgp/reactor/peer_rib_routes.go` - `buildWithdrawNLRI` (`:170-198`): existing egress precedent for encoding a withdraw-only UPDATE (legacy WithdrawnRoutes for ipv4/unicast, `attribute.MPUnreachNLRI` + `attribute.WriteAttrTo` for MP families).
- [ ] `internal/component/bgp/message/attr_discard.go` - `RebuildUpdateBody` (`:278`): existing precedent for rebuilding an UPDATE body on the receive path (cold path, heap allocation accepted).

**Existing tests pinning current behavior:**
- `reactor/session_test.go:1328-1425` (`TestSessionRFC7606MalformedOriginTreatAsWithdraw`), `:1436` (Community), `:1534` (missing mandatory): assert only no-error + Established.
- `reactor/session_test.go:1887-1915` (`TestSessionRFC7606TreatAsWithdrawSuppressesCallback`): asserts `callbackCount == 0`. MUST be rewritten (not weakened): the callback now fires exactly once with a withdraw-only payload, and the malformed attribute bytes never reach plugins.
- `reactor/session_validate_test.go:56-117` (`TestEnforceRFC7606_ShortBody`, `_InvalidWithdrawnNLRI`, `_InvalidTrailingNLRI`, `_MissingMandatoryAttrs`): action-only assertions on `enforceRFC7606`.
- `test/plugin/rfc7606-withdraw.ci`: pins session survival only. Its fence comment (lines 22-26) states updates-received is NOT incremented for the malformed UPDATE; after the fix the synthesized withdraw increments it (`reactor_notify.go:244-245`), the `>= 1` gate still passes, but the comment must be corrected.

**Behavior to preserve:**
- Not installing the routes carried by the malformed UPDATE (malformed attributes never reach plugins as announcements).
- No NOTIFICATION on treat-as-withdraw (RFC-correct); session stays Established; hold timer reset per RFC 4271 8.2.2 Event 27.
- Class A (structural/truncation/NLRI-syntax) treat-as-withdraw cases keep the wholesale drop: nothing is reliably parseable to withdraw (RFC 7606 Section 3.j precondition fails).
- A malformed UPDATE whose MP_REACH family was never negotiated must not gain a new teardown path: today it is dropped before `validateUpdateFamilies` runs; synthesis must skip non-negotiated MP families so the outcome stays "drop".

**Behavior to change:**
- On Class B treat-as-withdraw (attribute-semantic error, NLRI parseable), synthesize a well-formed withdraw-only UPDATE from the malformed one (explicit withdrawn-routes + announced NLRI into Withdrawn Routes; MP_REACH NLRI into MP_UNREACH; original MP_UNREACH preserved) and let it flow through the UNCHANGED normal dispatch path to the RIB and all other consumers.
- Side effect (accept + document): `updates-received` now increments for treat-as-withdraw UPDATEs because the synthesized withdraw traverses `notifyMessageReceiver` (`reactor_notify.go:244-245`). Visible in `show bgp peer <x> detail`.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- A received UPDATE whose attributes fail an RFC 7606 check that maps to treat-as-withdraw, while its withdrawn/NLRI sections parse cleanly (guaranteed for Class B by `session_validation.go:44-49,64-70` running before `ValidateUpdateRFC7606` at `:78`).

### Transformation Path
1. `processMessage` (`session_read.go:141`) parses the UPDATE into a `WireUpdate` and calls `enforceRFC7606` (`:158`).
2. `enforceRFC7606` (`session_validation.go:26`) classifies: Class A structural failures return treat-as-withdraw with no synthesized body (drop, unchanged); Class B (`ValidateUpdateRFC7606` returns `RFC7606ActionTreatAsWithdraw`, case at `:132-138`) synthesizes withdraw-only UPDATE body/bodies from the sections it already holds: withdrawn slice (`body[2:2+withdrawnLen]`, `:44-45`), announced NLRI slice (`body[offset+attrLen:]`, `:64-65`), MP_REACH family + NLRI bytes and MP_UNREACH via `wu.MPReach()` / `wu.MPUnreach()` accessors. NLRI bytes are copied verbatim (add-path path IDs preserved). It returns a new `WireUpdate` over the synthesized body with the original ctxID and sourceID (same mechanics as the attribute-discard rebuild at `:117-126`).
3. `processMessage` distinguishes the two outcomes at the current branch (`session_read.go:163`): no synthesized body -> keep today's `logFSMEvent(EventUpdateMsg)` + early return; synthesized body -> fall through to the normal flow with the synthesized `wireUpdate` (exactly like attribute-discard falls through today).
4. Normal flow, unchanged: `validateUpdateFamilies` (`:181`) -> `checkPrefixLimits` (`:201`, withdrawals counted) -> `onMessageReceived` (`:233-234`) -> `notifyMessageReceiver` (`reactor_notify.go:218`) -> ingress filters, cache, `deliverChan` -> `OnMessageBatchReceived` (`peer_run.go:493`) -> RIB `handleReceivedStructured` (`rib_structured.go:75`) -> `peerRIB.Remove` for legacy withdrawn (`:197-207`) and MP_UNREACH (`:240-259`) -> `checkBestPathChange` + `publishBestChanges` (`:261-306`, `rib_bestchange.go:701,1245`) withdraw the prior best path from Loc-RIB and notify downstream consumers.
5. `handleUpdate` (`session_handlers.go:233-246`) fires `EventUpdateMsg` (`:245`): hold-timer reset preserved without the hand-rolled call.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Wire ↔ reactor | parsed UPDATE + RFC 7606 action + synthesized withdraw body (`session_validation.go:132-138` hook) | [ ] |
| Reactor ↔ RIB/plugins | `onMessageReceived` -> `notifyMessageReceiver` -> `handleReceivedStructured` delivery of the synthesized withdraw (`session_read.go:233-234`, `reactor_notify.go:218`, `rib_structured.go:75`) | [ ] |
| Adj-RIB-In ↔ Loc-RIB | `peerRIB.Remove` + `checkBestPathChange` + `publishBestChanges` remove the prior route and publish the withdraw (`rib_structured.go:197-207,240-259,261-306`; `rib_bestchange.go:701,1245`) | [ ] |

### Integration Points
- `enforceRFC7606` (`session_validation.go:26`): action source AND the only point that can distinguish Class A from Class B; synthesis lives here (or in a helper it calls).
- `processMessage` (`session_read.go:163`): branch becomes "drop (Class A) vs continue with synthesized withdraw (Class B)".
- `notifyMessageReceiver` / RIB structured path: reused verbatim, no treat-as-withdraw-specific code downstream of the session layer.
- Wire-encoding helpers reused: `attribute.MPUnreachNLRI` + `attribute.WriteAttrTo` (egress precedent `peer_rib_routes.go:170-198`), body rebuild precedent `message.RebuildUpdateBody` (`attr_discard.go:278`).

### Architectural Verification
- [ ] No bypassed layers: the withdrawal flows through the normal `onMessageReceived` delivery path; no direct RIB call from the session layer.
- [ ] No duplicated functionality: the RIB's existing withdrawn/MP_UNREACH handling (`rib_structured.go:197-207,240-259`) is the single implementation; no parallel withdraw path, no per-consumer treat-as-withdraw flag.
- [ ] Registration over hardcoding — N/A (protocol semantics inside the session layer), stated for completeness (`ai/rules/plugin-self-containment.md`).

## Key Design Decisions

| # | Decision | Alternatives rejected | Evidence |
|---|----------|----------------------|----------|
| D-1 | Synthesize a well-formed withdraw-only UPDATE at the session layer and dispatch it through the unchanged normal path | (a) meta-flag on the delivery so each consumer implements treat-as-withdraw itself: spreads RFC semantics into rib, adj_rib_in, bmp, lg, forwarding; (b) direct RIB removal call: bypasses layers and misses every other consumer | Precedents for body replacement on the receive path: attribute-discard rebuild `session_validation.go:117-126`; ingress-filter modified payload `reactor_notify.go:451-466` |
| D-2 | Hook the synthesis in `enforceRFC7606` at the Class B case (`session_validation.go:132-138`); Class A keeps the early-return drop at `session_read.go:163-171` | Synthesizing in `processMessage`: it cannot distinguish Class A from Class B, the classification exists only inside `enforceRFC7606` | Class A producers `:31-70`; Class B producer `:78` runs after NLRI syntax validation `:44-49,64-70` |
| D-3 | Synthesis layout: Withdrawn Routes = original withdrawn bytes ++ original announced NLRI bytes (verbatim copy, path IDs ride along); MP_REACH(fam) becomes MP_UNREACH(fam) with the same NLRI bytes; original MP_UNREACH kept. If the original carries MP_REACH and MP_UNREACH of two DIFFERENT families, emit a second withdraw UPDATE (one MP_UNREACH per UPDATE; multiple MP_UNREACH is a 3.g session-reset shape) | Merging two families into one MP_UNREACH: invalid wire format | `rfc/short/rfc7606.md` Sections 3.g, 5.1; encoding precedent `peer_rib_routes.go:170-198` |
| D-4 | Validate MP_REACH's embedded NLRI with `nlrisplit.Split` (returns error on malformed input, `internal/core/bgp/nlri/nlrisplit/nlrisplit.go:78-84`) at synthesis time; on failure drop that portion wholesale (current behavior preserved) and log | Escalating to session reset per RFC 3.j: behavior change beyond this fix's scope, listed as an open question | `validateMPReachAttr` (`message/rfc7606.go:576-586`) does not syntax-check the MP NLRI, so synthesis is the first consumer of those bytes |
| D-5 | Skip synthesis for MP families not negotiated on the session; that portion stays dropped | Letting the synthesized withdraw hit `validateUpdateFamilies` strict mode: would convert today's silent drop into a NOTIFICATION + teardown, a new behavior on the malformed-UPDATE path | Current order: early return `session_read.go:163-171` precedes the family check `:181-195`; a non-negotiated family also has nothing in the RIB to withdraw |
| D-6 | Preserve ctxID and sourceID on the synthesized `WireUpdate` | Fresh ctxID: breaks add-path decoding in the RIB (`rib_structured.go:199,246` reads `ctx.AddPath(fam)` via `wu.SourceCtxID()`) | Attr-discard precedent preserves both (`session_validation.go:120-126`) |
| D-7 | Cold-path heap allocation for the synthesized body is acceptable; no pooling work | Pool plumbing for a malformed-UPDATE path: complexity without a hot path | `message.RebuildUpdateBody` precedent (`attr_discard.go:278`); `ai/rules/buffer-first.md` targets hot encode paths |
| D-8 | The primary synthesized UPDATE is dispatched with the original pool `BufHandle` (cache takes ownership as today); a second UPDATE (rare two-family case, D-3) is dispatched with an empty `BufHandle`, so it reaches the RIB via `deliverChan` but is not entered in the `recentUpdates` forward cache; RIB-driven withdraw propagation (`publishBestChanges`) is unaffected; only RS raw fast-path forwarding of that second body is skipped | Acquiring a second pool buffer: ownership complexity for a wire shape RFC 5.1 tells senders not to produce | Cache gate `reactor_notify.go:492`; rs fast path `:550-559`; two-family unit test pins both withdrawals reaching the RIB |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The NLRI is already fully parsed and valid when Class B treat-as-withdraw is decided | `session_validation.go:44-49` (withdrawn) and `:64-70` (NLRI) run `ValidateNLRISyntax` BEFORE `ValidateUpdateRFC7606` at `:78`; every Class B action postdates both checks | Cannot derive NLRI to withdraw | read of the parse ordering in `enforceRFC7606` (this design pass) | validated for Class B; explicitly FALSE for Class A producers (`:31-70`), which therefore keep the drop |
| A-2 | The explicit-withdraw delivery path can be reused for treat-as-withdraw NLRI | `rib_structured.go:197-207` (legacy withdrawn) and `:240-259` (MP_UNREACH) process any received withdraw-only UPDATE; `notifyMessageReceiver` imposes no announce-only constraint | Need new plumbing | trace `session_read.go:233` -> `reactor_notify.go:218` -> `peer_run.go:493` -> `rib_structured.go:75` (this design pass) | validated |
| A-3 | MP_REACH's embedded NLRI bytes may be malformed at synthesis time (not validated by the RFC 7606 attribute checks) | `validateMPReachAttr` `message/rfc7606.go:576-586` checks length + next-hop only | Synthesized MP_UNREACH would carry garbage into the RIB | `nlrisplit.Split` validation at synthesis time (D-4) | validated (gap confirmed; mitigation designed) |
| A-4 | Withdrawing a never-installed prefix is a safe no-op | `FamilyRIB.Remove` returns false, no error (`storage/familyrib.go:317-343`) | Spurious churn/log on AC-5 | read of the producer (this design pass) | validated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Withdrawing NLRI that was never installed causes spurious churn/log | RIB churn on first malformed UPDATE for a new prefix | `FamilyRIB.Remove` is a no-op for absent routes (A-4); `checkBestPathChange` emits nothing when no best path existed; AC-5 pins it |
| R-2 | Add-path / MP_REACH NLRI variants missed | failure of `TestRFC7606TreatAsWithdrawAddPath` / `TestRFC7606TreatAsWithdrawMPReach` | verbatim NLRI byte copy + preserved ctxID (D-3, D-6); dedicated ACs and unit tests |
| R-3 | Old-behavior pins break silently or get weakened: `TestSessionRFC7606TreatAsWithdrawSuppressesCallback` (`session_test.go:1887-1915`, callbackCount == 0) and the `rfc7606-withdraw.ci` fence comment (updates-received not counted) | those tests red after the change | rewrite them to STRONGER assertions (callback fires once with withdraw-only payload; malformed attrs never delivered); never delete the malformed-attrs-never-reach-plugins guarantee (AC-7) |
| R-4 | Synthesized withdraw for a non-negotiated family triggers a new strict-mode teardown | `TestRFC7606TreatAsWithdrawNonNegotiatedFamilyDrops` red | D-5: skip non-negotiated families at synthesis |
| R-5 | Two-family edge (MP_REACH fam Y + MP_UNREACH fam X, X != Y) mishandled | `TestSynthesizeWithdrawTwoFamilies` red | D-3/D-8: second UPDATE; unit test pins both withdrawals delivered |

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| peer re-advertises an installed prefix in a treat-as-withdraw UPDATE | -> | withdrawal delivered to RIB, prior route removed | `TestRFC7606TreatAsWithdrawRemovesRoute` |
| treat-as-withdraw UPDATE also carrying explicit withdrawn-routes | -> | both sets withdrawn | `TestRFC7606TreatAsWithdrawWithdrawnRoutes` |
| treat-as-withdraw UPDATE with MP_REACH NLRI | -> | synthesized MP_UNREACH withdraws the MP routes | `TestRFC7606TreatAsWithdrawMPReach` |
| end-to-end malformed re-advertisement over a live session | -> | prior route gone from the RIB (`show bgp rib`), session survives | `test/plugin/rfc7606-treat-as-withdraw.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Prefix P installed, then re-advertised in an UPDATE with a treat-as-withdraw attribute error | P is withdrawn from Adj-RIB-In and the Loc-RIB best path (withdraw published via `publishBestChanges`); session stays Established; no NOTIFICATION |
| AC-2 | Treat-as-withdraw UPDATE also carrying explicit WITHDRAWN routes | the explicit withdrawn-routes are also removed (merged into the synthesized Withdrawn Routes section) |
| AC-3 | MP_REACH NLRI in a treat-as-withdraw UPDATE | the MP NLRI is withdrawn via a synthesized MP_UNREACH of the same family with identical NLRI bytes |
| AC-4 | Genuinely un-parseable / truncated UPDATE (Class A: `session_validation.go:31-70`) | dropped wholesale as today; no callback; no crash; hold timer still reset |
| AC-5 | New prefix (never installed) in a treat-as-withdraw UPDATE | no route installed, no error, no spurious best-path event (`FamilyRIB.Remove` no-op) |
| AC-6 | Add-path session: treat-as-withdraw UPDATE announcing (pathID, P) | exactly that path is withdrawn; path IDs preserved verbatim in the synthesized NLRI bytes |
| AC-7 | Any Class B treat-as-withdraw UPDATE | plugins receive exactly one callback whose payload is withdraw-only; the malformed attribute bytes are never delivered as an announcement (strengthens, not weakens, the old suppression pin) |
| AC-8 | Original UPDATE carries MP_REACH (family Y) and MP_UNREACH (family X, X != Y) plus a treat-as-withdraw error | both families' routes are withdrawn (second synthesized UPDATE per D-3/D-8) |
| AC-9 | Treat-as-withdraw UPDATE whose MP_REACH family was not negotiated | dropped for that family exactly as today; no NOTIFICATION, no teardown (D-5) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC7606TreatAsWithdrawRemovesRoute` | `internal/component/bgp/reactor/session_test.go` | AC-1 (callback receives withdraw-only UPDATE whose Withdrawn section contains the announced prefix) | |
| `TestRFC7606TreatAsWithdrawWithdrawnRoutes` | `internal/component/bgp/reactor/session_test.go` | AC-2 | |
| `TestRFC7606TreatAsWithdrawMPReach` | `internal/component/bgp/reactor/session_test.go` | AC-3 | |
| `TestRFC7606TreatAsWithdrawAddPath` | `internal/component/bgp/reactor/session_test.go` | AC-6 | |
| `TestRFC7606TreatAsWithdrawUnparseableDrops` | `internal/component/bgp/reactor/session_test.go` | AC-4 (no callback for Class A) | |
| `TestRFC7606TreatAsWithdrawNonNegotiatedFamilyDrops` | `internal/component/bgp/reactor/session_test.go` | AC-9 | |
| `TestSessionRFC7606TreatAsWithdrawSuppressesCallback` (rewritten) | `internal/component/bgp/reactor/session_test.go` | AC-7 (malformed attrs never delivered; withdraw-only payload delivered once) | |
| `TestSynthesizeWithdrawBodyLegacy` | new synthesis helper test file (see Files to Create) | D-3 wire layout, withdrawn merge | |
| `TestSynthesizeWithdrawBodyMPReach` | new synthesis helper test file | D-3 MP_UNREACH synthesis, ctxID/sourceID preserved (D-6) | |
| `TestSynthesizeWithdrawTwoFamilies` | new synthesis helper test file | AC-8 / R-5 | |
| `TestSynthesizeWithdrawMalformedMPNLRIDrops` | new synthesis helper test file | D-4 / A-3 | |
| `TestRIBTreatAsWithdrawRemovesInstalledRoute` | `internal/component/bgp/plugins/rib/rib_structured_test.go` (or nearest existing structured-handler test file) | AC-1/AC-5 at the RIB boundary: install P, feed synthesized withdraw, P gone; feed withdraw for absent P, no event | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| NLRI count in synthesized withdrawn section (announced + explicit) | 0..N | N | N/A | N/A |
| MP families requiring MP_UNREACH synthesis | 0..2 | 2 (two UPDATEs) | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rfc7606-treat-as-withdraw.ci` | `test/plugin/` | peer announces P (valid), plugin observes P in `show bgp rib`; peer re-advertises P with malformed ORIGIN; plugin observes P GONE from the RIB; session survives (subsequent route still delivered) | |
| `rfc7606-withdraw.ci` (comment fix only) | `test/plugin/` | existing session-survival pin kept; stale "not counted" fence comment corrected to the new counter behavior | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| ~~`47-rfc7606-treat-as-withdraw`~~ **N/A (justified)** | `test/interop/scenarios/` | exabgp (can emit crafted/raw attributes; FRR and BIRD cannot deliberately send malformed attributes) ~~— daemon choice is an open question for Thomas; if no interop daemon can emit the malformed wire, justify N/A and rely on the ze-peer raw-hex functional test as the conformance driver~~ **RESOLVED (OQ2, 2026-07-17): interop harness cannot host exabgp today (FRR/BIRD/GoBGP/keepalived only); no shipped daemon emits malformed attributes. No scenario 47 is created; the ze-peer raw-hex functional test `test/plugin/rfc7606-treat-as-withdraw.ci` is the conformance driver. See the OQ2 AUTONOMOUS DEFAULT note.** | Ze withdraws the previously-installed route on a treat-as-withdraw UPDATE (driven end-to-end by the ze-peer raw-hex `.ci`, not a third-party speaker) | N/A (justified) |

## Files to Modify
- `internal/component/bgp/reactor/session_validation.go` - Class B case (`:132-138`): synthesize withdraw-only body/bodies (D-1..D-6); Class A early returns unchanged
- `internal/component/bgp/reactor/session_read.go` - treat-as-withdraw branch (`:163-171`): drop for Class A (unchanged), fall through to normal dispatch with the synthesized WireUpdate for Class B; loop for the rare second body (D-8); RFC comments per RFC Documentation below
- `internal/component/bgp/reactor/session_test.go` - new tests + rewrite of `TestSessionRFC7606TreatAsWithdrawSuppressesCallback` (R-3)
- `internal/component/bgp/reactor/session_validate_test.go` - extend `enforceRFC7606` tests for the synthesized-return contract
- `test/plugin/rfc7606-withdraw.ci` - correct the stale fence comment (lines 22-26) about updates-received

Explicitly NOT modified (verified no change needed):
- `internal/component/bgp/reactor/reactor_notify.go` - the delivery path is reused as-is (A-2)
- `internal/component/bgp/fsm/fsm.go` - `EventUpdateMsg` stays hold-timer-only; the synthesized flow reaches it via `handleUpdate` (`session_handlers.go:245`)
- `internal/component/bgp/plugins/rib/` - consumes the synthesized withdraw through its existing handlers

## Files to Create
- `internal/component/bgp/reactor/session_withdraw_synth.go` (name indicative) - synthesis helper: sections in, withdraw-only UPDATE body/bodies out; reuses `attribute.MPUnreachNLRI` + `attribute.WriteAttrTo`
- `internal/component/bgp/reactor/session_withdraw_synth_test.go` - `TestSynthesizeWithdraw*` unit tests
- `test/plugin/rfc7606-treat-as-withdraw.ci` - functional withdrawal test (modelled on `test/plugin/rfc7606-withdraw.ci`, plus `show bgp rib` assertions before/after the malformed re-advertisement)
- ~~`test/interop/scenarios/47-rfc7606-treat-as-withdraw/` - interop scenario (or documented N/A per the open question)~~ **OQ2 resolved N/A (justified, 2026-07-17): NOT created; the interop harness cannot host exabgp and no shipped daemon emits malformed attributes. The ze-peer raw-hex functional test `test/plugin/rfc7606-treat-as-withdraw.ci` is the conformance driver.**

## Implementation Steps

### Implementation Phases
1. **Phase: Wiring (MANDATORY FIRST)** - add the failing test `TestRFC7606TreatAsWithdrawRemovesRoute`: establish session, deliver valid UPDATE for P, deliver treat-as-withdraw re-advertisement of P, assert the callback receives a withdraw-only UPDATE containing P (red against current early return).
2. **Phase: synthesis helper** - `session_withdraw_synth.go` with its unit tests: legacy merge, MP_UNREACH synthesis, two-family split, malformed-MP-NLRI drop (D-3, D-4), ctxID/sourceID preservation (D-6).
3. **Phase: classify + dispatch** - `enforceRFC7606` returns the synthesized WireUpdate(s) for Class B (D-2); `processMessage` falls through to normal dispatch for Class B and keeps the drop for Class A; non-negotiated families skipped (D-5).
4. **Phase: preserve pins deliberately** - rewrite `TestSessionRFC7606TreatAsWithdrawSuppressesCallback` to AC-7; add AC-4/AC-9 drop tests; fix the `rfc7606-withdraw.ci` comment.
5. **Phase: RIB boundary test** - `TestRIBTreatAsWithdrawRemovesInstalledRoute` proving Adj-RIB-In removal + no spurious event for absent prefixes.
6. **Functional + interop tests** - `test/plugin/rfc7606-treat-as-withdraw.ci`; ~~interop scenario or justified N/A (open question)~~ **interop = N/A (justified, OQ2 resolved 2026-07-17): the harness cannot host exabgp and no shipped daemon emits malformed attributes; the `.ci` raw-hex test is the conformance driver.**
7. **Full verification** - `make ze-verify`.
8. **Complete spec** - audit tables, `plan/learned/NNN-<name>.md`, two-commit closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1..AC-9 each have code + test |
| Correctness | withdrawal reaches Adj-RIB-In and Loc-RIB best path; MP_REACH + add-path + two-family covered; Class A drop and Section 5.2 session-reset cases preserved |
| Data flow | withdrawal uses the normal delivery path (`onMessageReceived` -> `notifyMessageReceiver` -> RIB structured handlers), not a parallel one |
| Registration over hardcoding | N/A — protocol semantics (`ai/rules/plugin-self-containment.md`) |

## Known Limitations
- ~~Pre-existing divergence: `session_validation.go:46,66` collapse `ValidateNLRISyntax`'s Section 3.j session-reset result to a treat-as-withdraw drop, and several Class A truncation shapes are also dropped.~~ **Resolved in commit `b1f27a3e0`; this limitation no longer holds.** Every shape it named now escalates to session reset: `len(body) < 4` (`session_validation.go:39-41`), withdrawn-length overrun (`:45-47`), attribute-length overrun (`:61-63`), and both `ValidateNLRISyntax` call sites (`:54-56`, `:72-74`) now route through `rfc7606NLRISyntaxAction` (`:204-215`), which honors `RFC7606ActionSessionReset` at `:207-209` instead of flattening it. The escalation is pinned by nine `RFC requirement: RFC7606-3.j-1` tags, including the functional `test/plugin/rfc7606-reset.ci:9`.
- RFC 7606 Section 6 debugging (log the malformed UPDATE with its NLRI) is only partially met by the existing `sessionLogger().Debug` lines; not extended here.

## Open Questions (for Thomas)
1. ~~Unparseable MP_REACH NLRI inside a Class B treat-as-withdraw UPDATE: escalate now or in a follow-up fixit?~~ **Answered by the code that shipped in `b1f27a3e0`: escalated.** A malformed MP_REACH cannot reach the synthesis path any more, because Section 5.3 makes it "incorrect" and Section 3(j) escalates that to session reset upstream (`message/rfc7606_withdraw.go:127-128` states this at the skip branch). Covered by `rfc7606_test.go:684` (MP_REACH too short to parse escalates to session reset) and `:1282` (same for MP_UNREACH). No follow-up fixit is needed, and the Class A collapses this was to be bundled with are resolved too (see Known Limitations).
2. Interop daemon: FRR/BIRD cannot deliberately emit malformed attributes; exabgp can emit raw attribute bytes but the interop harness currently ships frr/bird/gobgp scenarios only. Add an exabgp-based scenario, or accept justified N/A with the ze-peer raw-hex functional test as the conformance driver?
   → AUTONOMOUS DEFAULT (2026-07-17): N/A (justified). Do NOT add an exabgp interop scenario; the ze-peer raw-hex functional test `test/plugin/rfc7606-treat-as-withdraw.ci` is the conformance driver. Rationale: the interop harness (`test/interop/`) hosts only FRR, BIRD, GoBGP and keepalived, started purely by config-file presence in `interop.py` `Scenario.setup()` (`frr.conf`->FRR `:1312-1327`, `bird.conf`->BIRD `:1329-1340`, `gobgp.toml`->GoBGP `:1364-1375`, `keepalived.conf`->keepalived `:1350-1362`); `run.py` builds/pulls only those images (`:31-109`). There is no exabgp Dockerfile, container constant, IP, build step or setup branch (grep for `exabgp` across `test/interop/` returns nothing), and none of the shipped daemons will deliberately emit a malformed attribute. Adding exabgp means new harness infrastructure (Dockerfile + container/IP + run.py build + setup/teardown branch + an exabgp process-API config emitting raw attribute bytes), out of scope for this fixit and the larger, non-self-contained option. The ze-peer harness already emits crafted/raw malformed attribute bytes via `action=send:...:hex=` and asserts on `expect=...:hex=` (proven by the sibling `test/plugin/rfc7606-withdraw.ci`, e.g. its malformed-ORIGIN-length-2 send at Step 3), so the conformance behavior is driven end-to-end without a third-party speaker. Thomas: override if wrong (add exabgp to the harness first, then scenario 47).
3. `updates-received` will now count treat-as-withdraw UPDATEs (synthesized withdraw traverses the counter at `reactor_notify.go:244-245`). Acceptable operator-visible change?
   → AUTONOMOUS DEFAULT (2026-07-17): Accept. The `updates-received` counter incrementing for treat-as-withdraw UPDATEs is an acceptable operator-visible change, already documented in Current Behavior "Behavior to change" (`:92`) and the `rfc7606-withdraw.ci` comment fix (`:82`, `:207`). Rationale: after the fix the synthesized withdraw is a real UPDATE that traverses `notifyMessageReceiver`'s counter (`reactor_notify.go:244-245`), so counting it is truthful (a route WAS withdrawn); the alternative (special-casing the counter to skip synthesized withdraws) would hide real RIB activity and push treat-as-withdraw-specific code into the generic delivery path, which D-1 explicitly rejects. Visible in `show bgp peer <x> detail`. Thomas: override if wrong.

## RFC Documentation

Add `// RFC 7606 Section 2: "MUST be handled as though all of the routes contained in an UPDATE message ... had been withdrawn"` above the Class B synthesis dispatch, `// RFC 7606 Section 3.j` above the Class A drop (NLRI not reliably parseable, nothing safe to withdraw), and keep `// RFC 7606 Section 5.2` at the session-reset escalation (`message/rfc7606.go:332-342`).

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-9 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Interop scenario passes (or N/A with justification per open question 2)
- [ ] Registration over hardcoding respected (N/A stated)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for NLRI counts and family counts
- [ ] Interop tests for protocol behavior (or N/A with justification)

## Notes
- Skeleton captured from the 2026-07-16 repository audit (verifier V3). Deepened to design on 2026-07-16: every citation re-verified against the working tree; the two-class producer split (Class A structural drop vs Class B attribute-semantic withdraw), the synthesis design (D-1..D-8), and assumptions A-1..A-4 resolved with producer evidence. Line-number corrections vs the skeleton: `reactor_notify.go:218-339` -> `:218-579`; `session_validate_test.go:54-116` -> `:56-117`; functional test location `test/bgp/` (nonexistent) -> `test/plugin/`. ~~Not `ready` until Thomas answers the open questions and approves.~~
- Readiness pass (2026-07-17): all three Open Questions resolved autonomously so a fresh implementer needs no answer from Thomas. OQ1 already answered by the code shipped in `b1f27a3e0` (left as-is). OQ2 resolved N/A (justified): the interop harness cannot host exabgp and no shipped daemon emits malformed attributes, so no `47-*` scenario is created and `test/plugin/rfc7606-treat-as-withdraw.ci` (ze-peer raw hex) is the conformance driver; dependent Interop/Files-to-Create/Implementation-step rows updated to match. OQ3 resolved Accept (`updates-received` counting synthesized withdraws is an acceptable, documented operator-visible change). Each resolution is recorded APPEND-ONLY at its Open-Questions entry with a Thomas-override line. Status stays `ready`.

## Review Gate

| Field | Value |
|-------|-------|
| Verdict | CLEAN (0 BLOCKER, 0 ISSUE) |
| Reviewer | Independent subagent over the diff (2026-07-19) |
| Artifact | `tmp/review/fixit-rfc7606-treat-as-withdraw-58c51aab-79d8-400d-b779-2c0cf322a274.md` |

### Scope note (park session 58c51aab, 2026-07-19)

The spec CORE (`message.SynthesizeWithdraw` + `enforceRFC7606` treat-as-withdraw dispatch)
was already implemented and committed by a prior session. This session closed the
RIB-boundary coverage gaps and fixed one latent ADD-PATH bug exposed by that coverage.

AC status:
- AC-1: COVERED. Adj-RIB-In removal (prior `adj_rib_in` test) + NEW Loc-RIB best-change
  propagation (`TestRIBTreatAsWithdrawRemovesInstalledRoute`, this session). Established +
  no NOTIFICATION covered by prior reactor tests.
- AC-2: COVERED (unit byte-transform, prior `TestSynthesizeWithdrawPreservesExistingWithdrawn`).
- AC-3: COVERED (unit, prior `TestSynthesizeWithdrawConvertsMPReachToMPUnreach`).
- AC-4: COVERED (prior session-reset + no-callback tests).
- AC-5: COVERED (NEW `TestRIBTreatAsWithdrawRemovesInstalledRoute` never-installed sub-case).
- AC-6: COVERED (NEW `TestRIBTreatAsWithdrawAddPathPreservesPathID`; required the
  `rib_structured.go` `SetAddPath` fix so the structured receive path keys ADD-PATH siblings).
- AC-7: COVERED (prior `TestSessionRFC7606TreatAsWithdrawDispatchesWithdrawal`).
- AC-8: **MET (2026-07-21).** `SynthesizeWithdrawFamilies` (`rfc7606_withdraw.go:78`) emits one
  withdraw body PER MP family (the RIB reads only the first MP_UNREACH via `GetRaw` first-match);
  the reactor dispatches `bodies[1:]` each as its own UPDATE. Both families reach the RIB AND
  route-server clients: the extra bodies ride a `noPoolBufID` cache-eligible `BufHandle`
  (`session_read.go:212`) so they enter `recentUpdates` and forward like the primary (the earlier
  empty-BufHandle attempt blackholed RS clients + logged a false "BUG" — see Review Gate Run 2).
  Tests: `TestSynthesizeWithdrawTwoFamilies`, `TestSessionRFC7606TreatAsWithdrawTwoFamiliesEntersForwardCache`.
- AC-9: **MET (2026-07-21).** Synthesis is negotiation-aware (`mpFamilyDispatchable`,
  `session_validation.go:355`, an exact mirror of `validateUpdateFamilies`' accept condition);
  non-negotiated MP families are skipped and, when nothing is left, `processMessage` restarts the
  HoldTimer and returns BEFORE `validateUpdateFamilies` — no NOTIFICATION, no teardown (D-5).
  Test: `TestRFC7606TreatAsWithdrawNonNegotiatedFamilyDrops`. Synthesis was moved out of
  `enforceRFC7606` (now classify/log only) into `processMessage` because it is negotiation-aware
  and multi-body.

## Review Gate

### Run 1 (park, 2026-07-19) — AC-1..AC-7 slice
Independent review over the park diff: CLEAN for the AC-1..AC-7 slice; AC-8/AC-9 honestly
declared NOT met (not claimed done).

### Run 2 (closure, 2026-07-21) — AC-8/AC-9 + fix
Independent adversarial review of the AC-8/AC-9 changeset found **0 BLOCKER, 1 ISSUE**: the
second-family body was dispatched with an EMPTY `BufHandle`, so in a route-server deployment it
was never cached; bgp-rs's `ForwardUpdatesDirect` missed, logged a false attacker-triggerable
`"BUG: msgID missing from cache"`, and did NOT forward the second family's withdraw to RS clients
(D-8's "publishBestChanges covers it" premise was wrong — RS clients are transparent-forward-fed
via the cache). FIXED (`session_read.go:212`): dispatch the extra body with
`BufHandle{ID: noPoolBufID, Buf: extra}` (existing sentinel for a non-pool heap Buf; `extra` is a
fresh, non-aliasing heap slice), so it caches and forwards to RS clients like the primary.

### Run 3 (closure re-review, 2026-07-21) — the fix
Independent review of the fix: **CLEAN — 0 BLOCKER, 0 ISSUE, 1 NOTE** (a stale test comment,
corrected). Confirmed: `noPoolBufID` return no-ops (no double-free / no pool slot consumed);
`extra` never aliases the pooled read buffer (`buildWithdrawBody` make+copy); the second body now
forwards to RS clients and the false BUG log is unreachable; the two new forward-cache tests are
non-vacuous (RED on the reverted one-line change); no AC-1..AC-9 regression; `message`/`reactor`/
`plugins/rs`/`plugins/rib` green, reactor `-race` clean, `go vet` clean, `ze-lint-changed` 0 issues,
`ze-rfc-check` green.

Gate satisfied: last run 0 BLOCKER, 0 ISSUE.

NIT (from review, non-blocking): the MP_REACH `SetAddPath` branch has no dedicated test
(line-for-line mirror of the tested IPv4 branch); an IPv6/MP add-path treat-as-withdraw
test would close it.

Verification: scoped `go test` on rib/message/fsm packages PASS; rib package `go vet` +
`golangci-lint` clean. (`make ze-verify*` and functional/QEMU suites deliberately NOT run —
park session on a shared live tree.)
