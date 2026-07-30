# Spec: rfcgate-1b -- the RFC 7296 pilot

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/rfcgate-1b-rfc7296-pilot.md` |
| Updated | 2026-07-30 |

Part of the `rfcgate` spec set; the umbrella is `plan/spec-rfcgate-0-umbrella.md`.
This spec is phase 8 of `plan/spec-rfcgate-1-extraction.md`, separated out of it by
owner ruling OR-2 (below) and re-ordered to run AFTER
`plan/spec-rfcgate-4-ledger.md`.

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**The problem.** `rfc/short/rfc7296.md` carries 23 requirement rows, 18 of them
MUST-level (`rfc/short/rfc7296.md:459-481`). RFC 7296 contains 289 MUST-level keyword
lines across 92 numbered sections. A section-by-section walk performed on 2026-07-29
found **214 distinct MUST-level obligations the summary does not capture**. The summary
therefore captures roughly **8%** of RFC 7296's MUST surface, and every gate in
`scripts/dev/rfc_requirements.py` judges only what a summary lists, so `make ze-rfc-check`
has been green over a 92%-blind spot.

**The symptom.** `rfc/enrolled.txt:159` advertises rfc7296 as "15 MET + 1 single-polarity
+ 2 gap", and `docs/features/rfc-status.md:213` discloses exactly two MUST gaps. Both are
accurate about the 18 extracted rows and both are silent about the 214. A reader of either
document would conclude Ze's IKEv2 compliance position is 15/18 with two known holes. The
real position, measured, is that **108 of the 214 unextracted obligations are not
implemented at all**.

**The goal.** Close the gap completely: extract all 214, implement all 108, prove all 214
with tagged tests in both polarities, and sign off `rfc/extraction/rfc7296.json` so the
bound is mechanical rather than asserted.

### The two owner rulings this spec encodes (2026-07-29)

| Ruling | Decision |
|--------|----------|
| OR-1 | **FIX ALL 108 INSIDE THIS SPEC.** Every missing IKEv2 MUST is implemented and proven here. Not deferred, not annotated, not routed onward. Thomas chose this over the narrower option that would have annotated them under his authorisation. |
| OR-2 | **PHASE 8 BECOMES ITS OWN SPEC** (this one), running AFTER `plan/spec-rfcgate-4-ledger.md`, so the rfcgate machinery set is not serialised behind IKEv2 compliance work. `plan/spec-rfcgate-1-extraction.md` becomes machinery-only and closes without phase 8. |

### The constraint OR-1 creates (BLOCKING, governs every row)

**No `{gap}`, `{not-applicable}`, `partial` or `{single-polarity}` annotation may be
written for any of the 214 rows.** `ai/rules/rfc-compliance.md:53` already voids every
prior annotation as authority; Thomas has now ruled explicitly in the opposite direction
for this RFC. There is no annotation budget in this spec, not one row, and nothing here
pre-authorises one.

If a specific obligation proves unimplementable, that is **a STOP-and-ask-Thomas
escalation**, never a self-authorised annotation. The escalation names, in this order:

1. the requirement id,
2. the RFC text verbatim with its `rfc/full/rfc7296.txt:<line>` locator,
3. the producing `file:line` that was read (`ai/rules/no-fabrication.md`),
4. what full implementation plus a tagged pair would cost,
5. the question "which way do you want this fixed" -- never "may I skip it"
   (`ai/rules/no-parking.md`).

The spec stays OPEN across such an escalation. A row is closed by code and proof, or by
Thomas's recorded answer in Deviations naming the date, the id and the decision.

### Why this spec runs after child 4

OR-2 orders it last, which has a mechanical consequence: by the time this spec runs,
`plan/spec-rfcgate-1-extraction.md` has landed, so the extraction bar is already on HEAD.
`rfc/extraction/rfc7296.json` must therefore satisfy `check_extraction_signoff`
**in full** -- every derived site and every section classified, the register derived, no
site left at the `null` disposition a generated skeleton emits
(`rfc/extraction/README.md:54-60`).

rfc7296 is enrolled at HEAD (`rfc/enrolled.txt:159`), so `check_enrolment` does not demand
an artifact for it: grandfathering is implemented as scope, not as an allowlist
(`rfc/extraction/README.md:121-126`). **This spec signs it off anyway.** That was the
pilot's entire purpose: proving the artifact format against the worst-measured input in
the 166-RFC corpus before 165 other stems are asked to use it.

**D7 of `plan/spec-rfcgate-0-umbrella.md:77` -- "FOUND MEANS OWED" -- still binds.** A
CONFIRMED unextracted MUST escapes the grandfather by definition. The walk confirmed 214.
This spec is where that debt is paid.

## Required Reading

### Architecture Docs
- [ ] `rfc/extraction/README.md` - the sign-off artifact's contract: which fields are
  authored and which are derived, the closed exclusion-kind set, the register ladder
  → Constraint: only `disposition`, `reason`, `mapped-to`, `excluded-kind`,
    `skip-kind`, `unsourced-ids`, `register`, `register-reason`, `signed-off` and
    `reviewer` are authored. `sites`, `sections`, `quote`, `register`, `source-path`,
    `source-sha` and every count are DERIVED and cross-checked; editing a derived field to
    turn a red green fails the check naming the field (`rfc/extraction/README.md:45-52`)
  → Constraint: the exclusion kinds are a closed set of five
    (`not-a-requirement`, `binds-another-role`, `duplicate-of`, `cross-document`,
    `advisory-in-context`) and the section skip kinds a closed set of five
    (`rfc/extraction/README.md:98-110`)
  → Decision: an artifact may declare the derived register or a WEAKER one; a stronger
    claim is refused (`rfc/extraction/README.md:62-66`)
- [ ] `plan/spec-rfcgate-0-umbrella.md` - D7 ("found means owed"), the sequencing
  constraint, and R-9 (a newly extracted MUST absorbed by a fresh annotation)
  → Constraint (D7, `:77`): a confirmed unextracted MUST escapes the grandfather
  → Constraint (R-9, `:787`): the cheapest way to keep the gate green over a new row is to
    annotate it, and the gate CANNOT catch that -- a `{gap}` is a legal annotation. It is a
    review obligation, carried in this spec's Critical Review Checklist
- [ ] `plan/spec-rfcgate-1-extraction.md` - the machinery this spec consumes; AC-23
  through AC-26 (`:643-646`) were written for phase 8 and are now this spec's
  → Constraint: this spec must not edit that file. It is machinery-only after OR-2
- [ ] `ai/skills/ze-rfc.md` - the `RFC requirement:` tag format and the one-obligation-per-line rule
  → Constraint: the tag is `// RFC requirement: <id> <positive|negative> -- <what>`
    (`ai/skills/ze-rfc.md:259-267`), and a tagged test's behaviour may not change without
    the user's approval
- [ ] `ai/rules/rfc-compliance.md` - "Ask Thomas Whenever Full Compliance Is On The Table"
  → Constraint (`:53`): every earlier answer pointing away from full compliance is VOID and
    must be re-raised, never cited

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc7296.md` - the summary being re-authored, 23 rows today
  → Constraint: three annotations exist and are VOID as authority --
    `{single-polarity: positive}` on `RFC7296-3.3-1` (`:462`), `{gap}` on `RFC7296-2.9-1`
    (`:467`) and `{gap}` on `RFC7296-1.4-1` (`:473`). Three of the new rows
    (`2.9-2`, `2.9.2-1`, `2.9.2-2`) sit on the same unimplemented TS-narrowing path as the
    `RFC7296-2.9-1` gap
  → Constraint: the Section Index (`:423-424`) swaps §1.3.2 and §1.3.3. The RFC has
    §1.3.2 = "Rekeying IKE SAs" (`rfc/full/rfc7296.txt:847`) and §1.3.3 = "Rekeying Child
    SAs" (`:882`). Existing id `RFC7296-1.3.3-1` therefore cites the wrong section; the
    obligation it names ("The KEi payload MUST be included") is at `rfc/full/rfc7296.txt:857`,
    inside §1.3.2
- [ ] `rfc/full/rfc7296.txt` - the source, 7955 lines
  → Constraint: 289 MUST-level keyword lines, 92 numbered sections, 70 of them carrying at
    least one MUST-level keyword

**Key insights:** (minimal context to resume after compaction)
- The walk report is `tmp/rfcgate-1-rfc7296-walk.md`. **`tmp/` is gitignored**, so it will
  not survive. Everything load-bearing is carried in this spec: the 214 rows with their
  triage class are Appendix A, the verified defect evidence is in Current Behavior, and the
  work-package partition is in Implementation Steps. Do not treat the walk report as a
  live reference; treat this spec as the record.
- Triage over the 214: **63 implemented-and-testable, 25 implemented-untested,
  108 NOT IMPLEMENTED, 18 uncertain.** Disjoint and exhaustive.
- The 214 ids were validated against the 23 existing ids: no collision, no internal
  duplicate, no ordinal colliding with an existing per-section high-water mark. Nothing is
  renumbered, reused or retired.

## Current Behavior (MANDATORY)

**Source files read:** (read before writing this spec; each claim below cites the function
that PRODUCES the behavior, per `ai/rules/no-fabrication.md`)
- [ ] `internal/component/ike/engine/sa.go` - the `SA` struct. `NextMsgID` is a bare
  `uint32` (`:83`); `remoteUDPAddr` resolves `ikeAddr(sa.PeerCfg.RemoteAddress)`, the
  CONFIGURED address, and stores no observed source port (`:166-172`)
- [ ] `internal/component/ike/engine/dpd.go` - `sendDPD` (`:79-117`) builds a
  `wire.Message` with only `Header` set and no `Payloads`, so the encoder emits a bare
  28-byte IKE header with no Encrypted (SK) payload; the comment at `:76-78` states the
  omission. `:90` hardcodes `Flags: wire.FlagInitiator`. `:94` is an unchecked
  `sa.NextMsgID++`
- [ ] `internal/component/ike/engine/fsm.go` - the retransmit path resends
  `sa.LastSentMsg` byte-identically (`:138-142`); `handleSAInitResponse`'s notify switch
  handles only `NotifyNoProposalChosen`, `NotifySignatureHashAlgorithms` and the two
  NAT-detection types (`:384-426`), and falls to the "incomplete IKE_SA_INIT response"
  dead end at `:428-432`
- [ ] `internal/component/ike/engine/inbound.go` - `handleDeletePayload` (`:282-293`)
  switches on `del.ProtocolID` and, for ESP, removes only `ps.supersededChild`;
  `del.SPIs` is never read. Unchecked `sa.NextMsgID++` at `:242` and `:309`
- [ ] `internal/component/ike/engine/established.go` - `sendRaw` (`:253-265`) sends every
  established-path message to `sa.remoteUDPAddr()`
- [ ] `internal/component/ike/wire/payload_notify.go` - the notify constant block
- [ ] `internal/component/ike/wire/payload_cp.go` - the Configuration payload codec
- [ ] `rfc/short/rfc7296.md` - the 23 existing rows and three annotations
- [ ] `rfc/extraction/README.md` - the artifact contract
- [ ] `scripts/dev/rfc_requirements.py` - `check_extraction_signoff` (`:2442`),
  `evaluate_extractions` (`:2423-2439`), `check_extraction_ratchet` (`:2506`)

### Verified defects (each traced to its producing function)

| ID(s) | Finding | Producing evidence |
|---|---|---|
| `RFC7296-2.2-1` | **IS implemented.** Retransmission resends `sa.LastSentMsg` byte-identically, so the Message ID is reused by construction. Needs a tagged pair, not code | `internal/component/ike/engine/fsm.go:138-142` |
| `RFC7296-2.2-2` | **NOT implemented.** `NextMsgID` is a bare `uint32` and every mutation is an unchecked `++`. On rollover the SA silently wraps to 0 and keeps running instead of being closed or rekeyed. No guard exists: `grep -rn "MaxUint32\|0xffffffff\|0xFFFFFFFF" internal/component/ike` returns nothing | field `internal/component/ike/engine/sa.go:83`; producers `dpd.go:94`, `inbound.go:242`, `inbound.go:309`, `rekey.go:86`, `rekey.go:329`, `fsm.go:596`, `fsm.go:634`, `fsm.go:703`, `fsm.go:726`; assignments `responder.go:574`, `fsm.go:482` |
| `RFC7296-1.4-2` | **NOT implemented.** The DPD liveness probe is sent unencrypted. `sendDPD` never sets `Payloads`, so no SK payload is emitted; the code comment admits it. Every other INFORMATIONAL path uses `buildEncryptedMessageEx` | `internal/component/ike/engine/dpd.go:79-117` (comment `:76-78`); contrast `inbound.go:270`, `:237`, `:304` |
| `RFC7296-3.1-12` | **NOT implemented.** `sendDPD` hardcodes `Flags: wire.FlagInitiator`, so a responder-role SA sets the I bit on a message it originates. The correct helper `initiatorFlag(sa)` is used by every other post-establishment sender | `internal/component/ike/engine/dpd.go:90` vs `inbound.go:237`, `:304` |
| `RFC7296-1.2-5`, `-6` | **NOT implemented.** The initiator cannot consume INVALID_KE_PAYLOAD. Ze SENDS it as responder (`responder.go:139`) but the initiator's notify switch does not handle it, so the response (which carries no SA/KE/Nonce) falls to the dead end and the SA dies instead of retrying with the responder's group | `internal/component/ike/engine/fsm.go:384-432`; sender `responder.go:139`; grep for `NotifyInvalidKEPayload` outside tests finds only `payload_notify.go:16` and `responder.go:139` |
| `RFC7296-2.6-3`, `-4`, `-5` | **NOT implemented.** COOKIE has no producer and no consumer. `grep -rn "NotifyCookie" internal/component/ike/` returns exactly one hit, the constant declaration | `internal/component/ike/wire/payload_notify.go:33` |
| `RFC7296-2.5-10`, `2.21.2-1`, `2.21.3-1`, `3.10.1-3` | **NOT implemented.** No error notification is emitted after IKE_SA_INIT. `NotifyUnsupportedCriticalPayload`, `NotifyInvalidSyntax`, `NotifyInvalidIKESPI`, `NotifyInvalidMessageID`, `NotifyTemporaryFailure`, `NotifySetWindowSize`, `NotifyIPCompSupported` and `NotifyUseTransportMode` are all declared constants with **zero** other references anywhere under `internal/component/ike/` | `internal/component/ike/wire/payload_notify.go:9`, `:10`, `:12`, `:13`, `:24`, `:28`, `:30`, `:34` |
| `RFC7296-1.4.1-2` | **NOT implemented.** A peer Delete naming the LIVE Child SA is ignored: `handleDeletePayload` removes only `ps.supersededChild` (the make-before-break leftover) and never reads `del.SPIs`, so the XFRM state stays installed | `internal/component/ike/engine/inbound.go:282-293` |
| `RFC7296-2.11-2`, `-3`, `2.23-4` | **NOT implemented.** Established-path replies ignore the observed source address and port. `sendRaw` resolves the CONFIGURED remote at the IKE port; the observed source port is never stored on the SA | `internal/component/ike/engine/established.go:253-265` -> `internal/component/ike/engine/sa.go:166-172` |
| `RFC7296-2.19-*`, `3.15.1-*`, `4-2`, `-3`, `1.7-1`, `2.20-1` (17 rows) | **NOT implemented.** The Configuration payload is a dead codec. `PayloadCP` is fully implemented and registered in the decoder table, but a repo-wide grep finds no consumer and no producer outside the codec file, the decoder registration, and two tests | codec `internal/component/ike/wire/payload_cp.go`; sole references `internal/component/ike/wire/payload.go:147`, `len_test.go:60`, `payload_cp_test.go` |

**Behavior to preserve:**
- The 23 existing requirement ids in `rfc/short/rfc7296.md` keep their numbers, their text
  and at least the polarities they hold at HEAD. `check_retired_requirements` blocks their
  disappearance and `check_coverage_ratchet` blocks a polarity loss; both must stay green
  across the re-authoring.
- The 31 existing `RFC requirement: RFC7296-*` tagged tests keep passing. They map to the
  23 existing ids and are protected by the `rfc-tagged-test` hook: any behaviour change to
  one needs the user's explicit approval, and `// test-relax:` does NOT satisfy that gate.
- `RFC7296-1.3.3-1` is **not renumbered**, even though it cites the wrong section.
  `check_id_allocation` blocks reuse and `check_retired_requirements` blocks disappearance,
  and two tests already tag it (`internal/component/ike/engine/responder_test.go:265`,
  `internal/component/ike/engine/rfc7296_test.go:123`).
- Every currently passing scenario under `test/ipsec-interop/scenarios/` (11 scenarios,
  strongSwan) and every `.ci` in `test/ipsec/` stays green. Several work packages change
  wire-visible behaviour, so this is a live constraint, not a formality.
- The other 165 enrolled stems stay unsigned and unaccused. A non-empty artifact set
  changes nothing for a stem that has no artifact.

**Behavior to change:**
- 108 IKEv2 MUST obligations move from unimplemented to implemented (Appendix A, class
  `NOT IMPL`; partitioned into 12 work packages in Implementation Steps).
- `rfc/short/rfc7296.md` grows from 23 rows to 237 (232 MUST-level).
- The Section Index entries for §1.3.2 and §1.3.3 are corrected. The id is not moved.
- `rfc/extraction/rfc7296.json` is created and signed off.
- `rfc/enrolled.txt:159` and `docs/features/rfc-status.md:213` are re-authored against the
  real position.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

Two flows meet in this spec: the compliance-gate flow (a requirement row becomes a checked claim) and the protocol flow (an IKEv2 message reaches the code the new rows govern).

### Entry Point
- **Gate flow:** `rfc/full/rfc7296.txt`, the normative source text, read by `derive_inventory` in `scripts/dev/rfc_requirements.py` at check time. Format: RFC plain text, 7955 lines, 289 MUST-level keyword sites.
- **Protocol flow:** UDP datagrams on ports 500 and 4500, received by `UDPTransport.Run` (`internal/component/ike/transport/udp.go:89-119`) into a `transport.MaxMsgSize` (3000-byte) buffer. Format: RFC 7296 wire messages, a 28-byte header followed by a payload chain. Attacker-controlled in full.

### Transformation Path
1. **Source to inventory.** `derive_inventory(stem, gated_count)` scans `rfc/full/rfc7296.txt` for MUST-level keyword sites, groups them by enclosing section, and derives the register (`rfc2119` / `prose` / `manual-walk`).
2. **Inventory to sign-off.** `_evaluate_extraction` compares the derived inventory against `rfc/extraction/rfc7296.json`: forward arithmetic (every derived site is `mapped` to an id or `excluded` with a closed-set kind and a reason) and reverse arithmetic (every gated requirement the summary declares is some site's target or is declared in a section's `unsourced-ids`).
3. **Summary to requirement set.** `parse_checklist_line` parses each `- [ ] [<id>] [<level>] <text> (§<section>)` row of `rfc/short/rfc7296.md` into a `Requirement`, validating that the id's section segment matches its `(§X.Y)` citation.
4. **Tags to coverage.** `make ze-rfc-index` scans `internal/**` for `RFC requirement: <id> <polarity>` comments and writes each test's `file:line` into `ai/RFC-REQUIREMENTS.md`; `make ze-rfc-check` fails on any gated requirement missing a polarity, and on a stale ledger.
5. **Wire bytes to parsed message.** `Message.ReadFrom` (`internal/component/ike/wire/message.go:98-137`) walks the payload chain by Next Payload, rejecting an unknown critical payload with `ErrUnsupportedCrit` (`:122-125`) and demoting an unknown non-critical payload to `PayloadRaw`.
6. **Parsed message to SA state.** `dispatchInbound` (`internal/component/ike/engine/register.go:641-651`) matches the SPI pair to an `SA`, then `classifyInbound` (`internal/component/ike/engine/msgid.go:69-93`) accepts exactly one request at `ExpectedMsgID` and caches exactly one response.
7. **SA state to dataplane.** `child.go` derives KEYMAT and installs XFRM state through `internal/component/ike/dataplane/`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| RFC source text <-> gate | `derive_inventory` reads `rfc/full/rfc7296.txt`; `source-sha` pins sha256[:16] of the normalized text, so a source edit stales the sign-off | No |
| Summary <-> gate | `parse_checklist_line` over `rfc/short/rfc7296.md`; id/section agreement enforced | No |
| Test tree <-> ledger | `RFC requirement:` comment tags scanned into `ai/RFC-REQUIREMENTS.md`; a moved test line stales the ledger and reds both verify modes | No |
| Network <-> IKE engine | UDP datagram to `wire.Message`; attacker-controlled, unauthenticated until IKE_AUTH completes | No |
| IKE engine <-> dataplane | XFRM state install/remove through `internal/component/ike/dataplane/` | No |
| IKE engine <-> config | YANG `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang`; WP-9 and WP-10 add operator-facing surface | No |

### Integration Points
- `scripts/dev/rfc_requirements.py` -- consumed unchanged. This spec authors data the
  machinery already judges; it must not modify the machinery (that is child 1's scope,
  and child 1 is closed by the time this spec runs).
- `internal/component/ike/engine/` -- the FSM, responder, inbound, rekey, DPD and
  established-path senders. Ten of the twelve work packages land here.
- `internal/component/ike/wire/` -- the payload codecs. WP-6, WP-9 and WP-12 extend
  parsing and validation; `payload_cp.go` gains its first production consumer.
- `internal/component/ike/crypto/` -- `NegotiateIKE`/`NegotiateESP`
  (`proposal.go:27-56`) are exact-tuple matchers today; WP-6 replaces that with real
  per-transform validation.
- `test/ipsec-interop/` -- the strongSwan lab, 11 scenarios. Every wire-visible package
  adds a scenario here.
- `test/ipsec/` -- 8 `.ci` functional tests through the daemon.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, handlers register and the core discovers them; no per-feature field, switch case, or factory added to a core/shared package (`ai/rules/plugin-self-containment.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The 214 count holds on a second, independent reading | The walk's mechanical pass mapped all 289 keyword sites to sections and classified each; 43 restatement sites were collapsed and 8 judged non-normative. The row set parses cleanly through `parse_checklist_line` with 0 errors | If it deflates, the extraction is smaller and cheaper; if it INFLATES, rows are missing from Appendix A and the sign-off cannot close. Either way the sign-off's forward arithmetic is the detector | Phase 1 re-walks the source against Appendix A before any row is committed; `check_extraction_signoff` is the mechanical backstop | **confirmed** (2026-07-30, phase 1: 214 rows, 63/25/18/108, 168 MUST + 46 MUST NOT, 0 parse errors, every obligation located inside its cited section) |
| A-2 | The four triage classes are disjoint and exhaustive over the 214 | Re-derived mechanically while writing this spec from the walk's own three explicit id lists (63 / 25 / 18) with `NOT IMPLEMENTED` as the complement: 63+25+18+108 = 214, no overlaps, no id outside the set, levels 168 MUST + 46 MUST NOT = 214 | A misclassified row is worked in the wrong phase; cost, not correctness | Re-derived while authoring this spec; Appendix A carries the per-row class | confirmed |
| A-3 | rfc7296 is grandfathered from the enrolment precondition, so the sign-off is voluntary here | `check_enrolment` demands an artifact only for a stem enrolled since HEAD; grandfathering is scope, not an allowlist (`rfc/extraction/README.md:121-126`). rfc7296 is enrolled at `rfc/enrolled.txt:159` | If wrong, the sign-off becomes a hard precondition rather than the pilot's deliverable -- which changes nothing about doing it | Run `make ze-rfc-check` on an unmodified tree at phase start and confirm rfc7296 is not accused | **confirmed** (2026-07-30: exit 0, and rfc7296 appears nowhere in the output) |
| A-4 | Only dispositions and reasons are authored in the artifact; everything else is derived and cross-checked | `rfc/extraction/README.md:45-52` and the field table at `:78-92` | Hand-typing a derived field to reach green fails the check naming the field and the locator, which is the intended failure | Write the skeleton with `make ze-rfc-extract STEM=rfc7296` (`Makefile:452`) and classify only what it leaves null | **confirmed** (2026-07-30: the skeleton carries every derived field and leaves all 261 sites and 104 sections null) |
| A-5 | All 214 proposed ids are free: no collision with the 23 existing, no internal duplicate, no ordinal below an existing per-section high-water mark | The walk ran the ids through `_validate_id` and reported 0 collisions, 0 internal duplicates, 0 ordinal collisions; per-section high-water marks were respected (§1.2 to 2, §1.4 to 2, §2.1 to 3, §2.4 to 5, §2.6 to 2, §2.7 to 2, §2.8 to 4, §2.9 to 2, §2.23 to 4, §3.3 to 3, §3.3.6 to 2) | An id collision is a hard red from `check_id_allocation` and forces renumbering of the NEW row (never the existing one) | Re-run the parse over the committed summary in phase 1; `check_id_allocation` is the gate | **confirmed** (2026-07-30: 0 collisions, 0 internal duplicates, 0 ordinals at or below a high-water mark, `check_id_allocation` clean, and the union has no ordinal gap) |
| A-6 | rfc7296 derives the `rfc2119` register | The source carries 289 capitalised MUST-level keyword lines, far more than the gated row count, which is the derived condition for `rfc2119` (`rfc/extraction/README.md:67-71`) | A weaker derived register (`prose`) changes which inventory the arithmetic runs over and enlarges the site set | The skeleton emitted by `make ze-rfc-extract STEM=rfc7296` states the derived register; an artifact may declare it or weaker, never stronger | **confirmed** (2026-07-30: `derive_inventory("rfc7296", 18)` returns register `rfc2119`, 261 sites, 104 sections, source-sha `a6f1a101b818977b`) |
| A-7 | The 18 `uncertain` rows resolve into implemented or not-implemented by reading alone, without new infrastructure | The walk names exactly which files were not read: `internal/component/ike/eap/**`, `wire/payload_eap.go`, `wire/payload_id.go`, `crypto/dh.go`, `dataplane/xfrm_linux.go`, and `handleAuthResponse`'s notify handling from `fsm.go:507` | If a row resolves to not-implemented, it joins the 108 and its work package grows. That is a schedule risk, not a scope change: OR-1 already owns it | Phase 4 reads each named file and records the producing `file:line` per row | unvalidated |
| A-8 | The 31 existing tagged tests survive the re-authoring unchanged | They map to the 23 existing ids, none of which is renumbered or re-texted by this spec | A behaviour change to a tagged test needs Thomas's approval; the `rfc-tagged-test` hook blocks it and `// test-relax:` does not satisfy that gate | `make ze-unit-test` over `internal/component/ike/...` at every phase boundary; `check_coverage_ratchet` at every commit | unvalidated |
| A-9 | Every row in the `implemented-and-testable` class can be proven without touching production code | The walk names a producing `file:line` and a target test file for each of the 63 | A row that needs code moves from phase 2 into a work package in phase 3. Cost, not scope | Phase 2 writes the pair; a red that is not the test's own fault reclassifies the row | unvalidated |
| A-10 | The errata for RFC 7296 introduce no obligation absent from `rfc/full/rfc7296.txt` | Not established. The walk explicitly did not check `https://www.rfc-editor.org/errata/rfc7296` (no network access) and flagged it rather than deferring it | A verified erratum that changes normative text means Appendix A is wrong at that row and the source-sha pin is over stale text | Phase 1 fetches the errata page and reconciles it before the first row is committed (`ai/skills/ze-rfc.md` step 4) | **broken** (2026-07-30). No erratum adds an obligation, so the literal claim holds. Two errata still make Appendix A wrong. Verified erratum 6940 corrects text that row `RFC7296-3.10-3` quoted. Held erratum 5056 contradicts how row `RFC7296-1.7-1` reads. Appendix A's preamble records both |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | **The 214 count deflates on a second reading**, and this spec is discovered to be sized against an inflated number. 71 of the 214 rows split a compound RFC sentence to satisfy one-obligation-per-line; a reviewer preferring strict verbatim would merge them back and the count falls | Phase 1's re-walk produces a materially different row set from Appendix A | The count is not the deliverable; the coverage is. A smaller true set is a smaller spec, not a failure. If the merge question is live, it is escalation E-4 to Thomas (quotation fidelity versus id granularity) and his answer is recorded in Deviations before any row is committed |
| R-2 | **Some of the 108 need wire-visible behaviour changes that break the strongSwan lab.** WP-2 (encrypting DPD), WP-6 (real proposal validation), WP-7 (TS narrowing), WP-8 (NAT-T source port) all change what a peer sees | A previously green scenario under `test/ipsec-interop/scenarios/` reds after a work package lands | `make ze-ipsec-interop-test` runs at every work-package boundary, not only at the end. A red there is the package's own defect and is fixed in that package (`ai/rules/no-parking.md`), never absorbed by relaxing the scenario |
| R-3 | **The Message ID exhaustion fix touches replay protection and is security-relevant.** RFC 7296 states at `rfc/full/rfc7296.txt:1437` that Message IDs "are cryptographically protected and provide protection against message replays"; the rollover-to-zero this fixes is precisely a replay window | A change to `classifyInbound` or `NextMsgID` handling that alters which messages are ACCEPTED rather than which are SENT | WP-1 is specified as: close or rekey the SA at exhaustion, and never widen the accept predicate. The Security Review section states what a peer can force. Both polarities of `2.2-2` are tested, and the negative test asserts the SA is torn down rather than that no error is returned |
| R-4 | **A row proves unimplementable and the cheapest exit is an annotation.** `{gap}` is a legal annotation that the gate accepts, so no gate can catch this (umbrella R-9, `plan/spec-rfcgate-0-umbrella.md:787`) | An annotation appears in a diff against `rfc/short/rfc7296.md` | Stated as a BLOCKING constraint in Task. The Critical Review Checklist carries the row explicitly, and the Deliverables Checklist has a grep that must return zero. The only legal exit is the STOP-and-ask escalation, recorded in Deviations with Thomas's answer |
| R-5 | **The spec is large enough that a partial landing leaves `make ze-rfc-check` red**, which blocks every other session's commits (`commit_helper.py` refuses a script over a non-green verify) | A commit adds a checklist row whose tagged tests are not in the same commit | The Landing Strategy section makes this structural: a row and its proof land in the SAME commit, and no commit adds a row it does not prove. `ze-rfc-check` is green at every commit, not only at the end |
| R-6 | **The re-authoring demotes or retires an existing id by accident.** Re-ordering 23 rows into a 237-row file is a bulk edit over ids protected by two ratchets | `check_retired_requirements` or `check_coverage_ratchet` reds on the re-authoring commit | Phase 1 lands the re-authored summary as its own commit with the existing 23 rows byte-identical, so a diff of the existing rows is empty by construction and the ratchets have nothing to catch |
| R-7 | **`RFC7296-1.3.3-1`'s wrong section citation cannot be fixed by moving the id**, and the obvious fix (renumber to `RFC7296-1.3.2-N`) is blocked by two ratchets and would orphan two tagged tests | Any diff that changes the id string on `rfc/short/rfc7296.md:466` | The Section Index is corrected; the id is not. The row's `(§1.3.3)` citation stays, because `parse_checklist_line` validates that the id's section segment MATCHES its citation -- changing one without the other is a parse error. The discrepancy is recorded as a Known Limitation and disclosed in the artifact's section notes |
| R-8 | **WP-9 adds a whole operator-facing feature surface** (remote-access configuration: virtual IP, DNS, netmask attributes) that needs YANG, validation, completion, doctor checks and documentation, not just wire code | The work package's file list starts naming `yang/` and `docs/guide/` | WP-9 is scheduled last among the implementation packages and carries its own Integration Checklist pass. If it turns out to be a spec-sized feature in its own right, that is a scope question for Thomas -- raised as a question, never resolved by dropping the rows |
| R-9 | **The 18 `uncertain` rows resolve badly**: if a majority land in NOT IMPLEMENTED, the implementation set grows from 108 toward 126 | Phase 4's reads | Phase 4 runs BEFORE the implementation packages are sized in detail, so the growth is absorbed into planning rather than discovered mid-flight |
| R-10 | **The source-sha pin stales the sign-off** if `rfc/full/rfc7296.txt` is normalized or re-fetched | `check_extraction_signoff` reports a sha mismatch | Do not touch `rfc/full/rfc7296.txt`. If an erratum forces a source change (A-10), the sign-off is re-derived and `signed-off` is bumped in the same commit |
| R-14 | **The corpus moved under the spec** (found 2026-07-30, phase 1). The spec assumes 166 enrolled stems and zero sign-offs. The tree now holds 168 enrolled stems and four signed artifacts. They are `rfc1035`, `rfc3765`, `rfc4486` and `rfc5301`, and `spec-rfcgate-4-ledger` phase 6 signed all four. Two of them are enrolled. AC-5 and one Deliverables row are already false | `ls rfc/extraction/*.json` returns 4. `ze-rfc-check` reports 168 enrolled and 166 unsigned | At closure the true figures are five artifacts, three signed enrolled stems and 165 enrolled unsigned. AC-5's arithmetic survives by coincidence. Its premise of "exactly one sign-off" does not. Re-author AC-5 and the Deliverables row against the live corpus. The figure moves again each time a sibling spec signs a stem |
| R-12 | **A red-first test in `scripts/dev/rfc_requirements_test.py` reds the gate it tests** (found 2026-07-30, phase 1). `make ze-rfc-check` runs `--selftest` and then `--check` (`Makefile:437-439`). `--selftest` runs that whole file (`rfc_requirements.py:6440`). `python_tests_test.go` also globs `scripts/dev/*_test.py`. No location under `scripts/dev/` holds a red Python test that leaves both gates green. The spec's two phase-1 Verify bullets contradict each other | Any red `TestRealTree` pilot case makes `make ze-rfc-check` exit 2 before `--check` runs | **Needs an owner ruling.** The five cases are written, and phase 1 proved each one red. They are staged at `tmp/rfcgate1b/rfc7296_pilot_wiring.py.staged` with their insertion point, so the shared tree stays green. Either they land in the commit that turns them green, or the pilot needs an incubator for Python gate tests. `ai/rules/testing.md` gives `.ci` tests such an incubator at `test/draft/`. Every later phase meets this same wall |
| R-13 | **An unsigned skeleton cannot live in `rfc/extraction/`** (found 2026-07-30, phase 1). The Landing Strategy calls it "a legal intermediate state that still parses". It does parse, but parsing is not passing. The skeleton produces 385 gate errors, and `ze-rfc-check` exits 2. They are 261 unclassified sites, 104 unclassified sections, an empty `signed-off` and `reviewer`, and 18 reverse-arithmetic errors | `make ze-rfc-check` exits 2 as soon as the skeleton is committed | Generate the skeleton on demand and keep it out of the tree until it is fully classified. It lands once, signed, in the final phase. Run the mid-walk check against the skeleton in place while it stays uncommitted. Remove it again before any commit |
| R-11 | **Interop coverage becomes the bottleneck**: 11 of the 25 `implemented-untested` rows need a second daemon, and several need a driven clock the engine does not expose | Phase 3 stalls on rows whose only proof path is a scenario that does not exist | The test infrastructure (clock injection into `maintainSA`, new strongSwan scenarios) is itself a deliverable of phase 3, listed in Files to Create. Building it is in scope; skipping the rows is not |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | An IKEv2 peer relationship. Ten of the twelve work packages change how Ze parses or emits messages on an unauthenticated or newly authenticated path: a wrong accept predicate lets a peer force state, a wrong reject predicate drops a conforming peer's session. The Configuration-payload package (WP-9) additionally introduces operator-facing config that, if modelled wrong, is a shipped config surface Ze must keep. The compliance-gate half is lower stakes: a wrong sign-off makes `make ze-rfc-check` claim a bound it does not have, which misleads readers of `docs/features/rfc-status.md` without breaking the daemon |
| How is it reverted? | Per work package, and that is the point of the partition: each package is its own commit or short commit run, each leaves `ze-rfc-check` green, and each is revertible without touching its siblings. The summary re-authoring (phase 1) is revertible only in the sense that reverting it also reverts every row's proof -- so it lands first and never moves again. Once a peer has seen new wire behaviour (WP-2 encrypted DPD, WP-8 NAT-T source port), reverting restores the old non-conformance rather than a neutral state |
| Who else touches this path? | `plan/spec-rfcgate-0-umbrella.md` orders the set and forbids two children in flight, so no sibling rfcgate spec should be live alongside this one. Within `internal/component/ike/`, any concurrent IPsec work collides directly: this spec touches the FSM, responder, inbound, rekey, DPD, established-path senders, every wire codec and the proposal negotiator. `rfc/short/rfc7296.md`, `rfc/enrolled.txt` and `docs/features/rfc-status.md` are shared files that other sessions edit for other RFCs |

## Security Review

IKEv2 parses attacker-controlled network input on ports 500 and 4500 before any peer is
authenticated. `UDPTransport.Run` (`internal/component/ike/transport/udp.go:89-119`)
accepts from any source by design (`RFC7296-2.11-1`), so every parser and every state
transition reachable before IKE_AUTH completes is reachable by an unauthenticated
attacker. This section states what a peer can force, what wraps, and what a replayed or
out-of-window message can reach.

### What a peer can force today (pre-existing, and what this spec changes)

| Surface | What a peer can force now | Producing evidence | Which package changes it |
|---|---|---|---|
| Message ID space | Drive `NextMsgID` to wrap. Every mutation is an unchecked `++` on a bare `uint32` with no exhaustion guard, so after 2^32 sends the SA silently restarts its Message ID sequence at 0 while continuing to use the same keys. RFC 7296 calls Message IDs replay protection (`rfc/full/rfc7296.txt:1437`), so a wrapped counter re-opens a window the protocol assumes is closed | `internal/component/ike/engine/sa.go:83`; producers listed in Current Behavior | WP-1 |
| Liveness probe | Read the DPD probe in the clear and correlate SPI pair, Message ID and timing without breaking any cryptography. The probe carries no SK payload | `internal/component/ike/engine/dpd.go:79-117` | WP-2 |
| Child SA teardown | Send a Delete naming the live Child SA and have it silently ignored, so Ze keeps forwarding into an SA the peer has torn down. Conversely, a peer cannot cleanly tear down a specific Child SA at all | `internal/component/ike/engine/inbound.go:282-293` | WP-2 |
| Half-open state | Exhaust responder state without a COOKIE round trip. Ze has no anti-DoS half-open defence beyond the inbound token-bucket rate limiter, because COOKIE has no producer and no consumer | `internal/component/ike/wire/payload_notify.go:33`; limiter `internal/component/ike/engine/register.go:618` | WP-4 |
| Unmatched SPI pair | Send a datagram whose SPI pair matches nothing and have it silently dropped. This is currently SAFE (silence leaks nothing), and WP-3 deliberately makes Ze noisier by adding INVALID_IKE_SPI | `internal/component/ike/engine/register.go:641-651` | WP-3 |

### What each package must not do

| Package | The security-relevant invariant |
|---|---|
| WP-1 (Message ID, window) | Widening the accept predicate is the danger, not the rollover fix. `classifyInbound` (`internal/component/ike/engine/msgid.go:69-93`) accepts exactly one request at `ExpectedMsgID` today. Supporting a window > 1 means accepting a RANGE, and every message in that range is a replay candidate. The window must be bounded by the peer's stated SET_WINDOW_SIZE and by a local cap, an out-of-window message must be dropped without a response (`RFC7296-2.3-5`), and INVALID_MESSAGE_ID must be rate limited (`RFC7296-2.3-6`) or it becomes an amplification primitive. The exhaustion fix itself must CLOSE or REKEY, never reset the counter |
| WP-2 (INFORMATIONAL encryption) | Encrypting the DPD probe must not weaken the response path: `handleDPDResponse` correlates by Message ID (`internal/component/ike/engine/dpd.go:30-32`), which is what rejects a replayed response masking a dead peer. That correlation must survive the change. The I-bit fix is one line and changes nothing security-relevant, but it is wire-visible to a conforming peer |
| WP-3 (error notifications) | Every notification this package ADDS is a new unauthenticated response Ze emits, so each is an amplification and information-disclosure surface. RFC 7296 constrains them precisely and the constraints are the security property: the response MUST NOT be cryptographically protected (`2.21.4-3`), MUST go to the source address and port with the SPIs and Message ID copied (`2.21.4-2`), a peer receiving one MUST NOT respond (`2.21.4-5`) and MUST NOT change SA state (`2.21.4-6`, `-7`). A missing "MUST NOT respond" turns two Ze instances into a packet loop. Rate limiting is mandatory (`RFC7296-2.4-7`, already implemented) |
| WP-4 (COOKIE) | The COOKIE secret and its rotation are the whole security value. A cookie that is not bound to the initiator's address and a rotating local secret is a token an attacker mints. The data is 1 to 64 octets (`RFC7296-2.6-3`); the initiator must echo it unchanged as the FIRST payload with everything else unchanged (`2.6-4`); a mismatched cookie must be IGNORED and the message processed as if absent (`2.6-5`), which is a deliberate fail-open the RFC specifies and must not be "hardened" into a rejection |
| WP-6 (proposal validation) | This package makes Ze REJECT more, which is the safe direction, but each new rejection is reachable pre-authentication and must not panic, allocate unboundedly, or loop on attacker-controlled counts. `Transform.ReadFrom` accepts any attribute set today (`internal/component/ike/wire/payload_sa.go:72-95`); adding duplicate-attribute and encoding checks means iterating attacker-supplied lists |
| WP-7 (TS narrowing) | Narrowing is a security control, not a formality: installing the initiator's TSi/TSr verbatim (`internal/component/ike/engine/responder.go:477-482`) means a peer selects its own traffic selectors and Ze installs XFRM policy for them. This is the highest-value item in the spec from a security standpoint, and `RFC7296-2.9.2-1`/`-2` bound the narrowing so a rekey cannot silently widen scope |
| WP-8 (NAT-T source port) | Replying to the OBSERVED source address and port (`RFC7296-2.11-2`, `-3`) is required for NAT traversal and is also a redirection primitive if the observed address is trusted for an UNAUTHENTICATED message. The observed address must be adopted for the reply only, and an established SA's stored peer address must change only on an authenticated message |
| WP-9 (Configuration payload) | Ze becomes an address-assigning server. `RFC7296-2.19-5` (no CFG_REPLY without a CFG_REQUEST) and `2.19-6` (FAILED_CP_REQUIRED when policy requires CP and the peer omitted it) are authorization checks and must fail closed (`ai/rules/fail-closed-guards.md`). Attribute parsing is attacker-controlled and `3.15.1-4` requires unrecognised attributes to be IGNORED, not to abort |
| WP-10 (certificates, PSK hex) | Accepting up to four certificates means accepting an attacker-supplied chain: the first certificate MUST carry the AUTH key (`RFC7296-1.2-2`) and chain validation must not be weakened to accommodate the new count. Hash-and-URL (`3.6-2`, `3.6-3`) makes Ze fetch an attacker-named http URL, which is an SSRF surface and needs an explicit bound. Hex PSK decoding (`2.15-3`) must not change how an existing ASCII PSK is interpreted |

### Fail-closed obligations introduced here
Per `ai/rules/fail-closed-guards.md`: every new guard added by this spec denies on a miss,
an unmapped input, an empty set, or an error, and a guard that genuinely cannot deny logs
or errors. A zero value must never read as a valid answer -- specifically, a zero Message
ID, a zero SPI, an empty traffic-selector set and an empty attribute list are all
attacker-reachable and none may take a permissive default.

## Landing Strategy

**This spec is large.** It re-authors a summary from 23 rows to 237, writes 214 tagged
test pairs (428 tagged tests), implements 108 protocol obligations across 12 work packages,
builds interop and clock-injection test infrastructure that does not exist, and signs off
an extraction artifact against a 7955-line source. It is not a single-sitting spec and
must not be attempted as one.

**The mechanical constraint that shapes every commit:** a checklist row with no tagged
tests reds `make ze-rfc-check`, and `commit_helper.py create` refuses a script over a
non-green verify. So:

| Rule | Consequence |
|------|-------------|
| A row and its proof land in the SAME commit | No commit ever adds a requirement it does not prove. There is no window in which the gate is red by design |
| No commit adds a row for an unimplemented obligation | For the 108, the implementation, the tagged pair and the checklist row are one commit (or one short commit run ending green) |
| Phase 1 lands the 88 provable rows only | The re-authored summary initially carries the 23 existing plus the 63 `implemented-and-testable` plus the 25 `implemented-untested` rows, each with its pair. The 108 and the 18 arrive with their work packages |
| Each work package is its own landing | 12 packages, each green, each independently revertible |
| `make ze-ipsec-interop-test` runs at every package boundary | A wire-visible change that breaks the strongSwan lab is caught in the package that caused it |

**Consequence for the extraction sign-off:** `rfc/extraction/rfc7296.json` cannot be
completed until every site maps to a committed id, so it is authored incrementally
alongside the rows and signed off (`signed-off`, `reviewer` populated) only in the final
phase. An unsigned skeleton is a legal intermediate state that still parses
(`rfc/extraction/README.md:94-96`), so the artifact can be checked mid-walk to see which
sites remain.

**This spec stays `in-progress` across all of it.** It does not close until every one of
the 214 rows is proven in both polarities and the sign-off validates.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `make ze-rfc-check` on the live tree | -> | `check_extraction_signoff` reading `rfc/extraction/rfc7296.json` against the derived inventory | `TestRealTree.test_rfc7296_signoff_is_valid_and_the_rest_stay_grandfathered` |
| `make ze-rfc-check` on the live tree | -> | `parse_checklist_line` over the 237-row `rfc/short/rfc7296.md`, then gated-coverage evaluation | `TestRealTree.test_rfc7296_every_gated_row_is_proven_in_both_polarities` |
| `make ze-rfc-index` then `make ze-rfc-check` | -> | the `RFC requirement:` tag scan writing each test's `file:line` into `ai/RFC-REQUIREMENTS.md` | `TestRealTree.test_rfc7296_ledger_is_fresh_after_the_pilot` |
| An inbound IKE datagram whose Message ID has wrapped | -> | the exhaustion guard replacing the unchecked `sa.NextMsgID++` (`internal/component/ike/engine/sa.go:83` and its nine producers) | `TestMessageIDExhaustionClosesTheSA` (`internal/component/ike/engine/rfc7296_test.go`) |
| A DPD probe emitted by an established SA | -> | `sendDPD` building through `buildEncryptedMessageEx` with `initiatorFlag(sa)` (`internal/component/ike/engine/dpd.go`) | `test/ipsec/ipsec-dpd-probe-encrypted.ci` |
| An inbound Delete naming the live Child SA | -> | `handleDeletePayload` reading `del.SPIs` (`internal/component/ike/engine/inbound.go:282-293`) | `test/ipsec/ipsec-peer-delete-child.ci` |
| An IKE_SA_INIT response carrying INVALID_KE_PAYLOAD | -> | the initiator retry path in `handleSAInitResponse` (`internal/component/ike/engine/fsm.go:384-432`) | `test/ipsec-interop/scenarios/12-invalid-ke-retry` |
| A peer CFG_REQUEST in the first IKE_AUTH | -> | the first production consumer of `wire.PayloadCP` (`internal/component/ike/wire/payload_cp.go`) | `test/ipsec-interop/scenarios/14-remote-access-cp` |
| `ze doctor --json` on an IPsec config using a hex PSK | -> | the hex PSK decode path (`internal/component/ike/engine/auth.go:232-248`) | `test/parse/ipsec-psk-hex.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `rfc/short/rfc7296.md` after phase 1 | It carries `RFC7296-2.2-1` and `RFC7296-2.2-2`, each quoting its RFC text and citing `(§2.2)`, so the id's section matches its citation as `make ze-rfc-check` already requires. At minimum these two; the other 212 rows are additional, never a substitute |
| AC-2 | `rfc/short/rfc7296.md` at closure | It carries all 214 rows of Appendix A plus the 23 existing rows: 237 rows, 232 of them MUST-level. Every row parses through `parse_checklist_line` with zero errors and every id's section segment matches its `(§X.Y)` citation |
| AC-3 | `rfc/short/rfc7296.md` before and after the re-authoring | Every one of the 23 existing ids is still present and still carries at least the polarities it held at HEAD. `check_retired_requirements` and `check_coverage_ratchet` stay green across the re-authoring, and no id is renumbered or reused |
| AC-4 | Any requirement this spec extracts | It carries a positive AND a negative `RFC requirement:` tagged test. **No annotation is permitted for any row** (OR-1). An annotation present in `rfc/short/rfc7296.md` for any of the 214 fails this AC regardless of any recorded justification other than a Thomas answer captured in Deviations with its date, the requirement id and his exact decision |
| AC-5 | The tree at closure (166 enrolled, exactly one sign-off) | `make ze-rfc-check` exits 0, `rfc/extraction/rfc7296.json` validates with every derived site and every section classified and its register derived, and the other 165 stems remain unsigned and unaccused. A non-empty artifact set changes nothing for a stem that has no artifact |
| AC-6 | The 63 `implemented-and-testable` rows | Each has a positive and a negative tagged test naming the producing `file:line` the walk identified, and none required a production-code change. A row that turned out to need code is recorded as reclassified and moved into its work package |
| AC-7 | The 25 `implemented-untested` rows | Each has a positive and a negative tagged test. The ~11 that need a second daemon have a scenario under `test/ipsec-interop/scenarios/`; the timeout and rate-limit rows have a driven clock the engine exposes to tests |
| AC-8 | The 18 `uncertain` rows | Each has been resolved by reading its producing path, is recorded in this spec with the `file:line` that resolved it, is classified implemented or not-implemented, and carries a tagged pair. No row remains `uncertain` at closure |
| AC-9 | The 108 `NOT IMPLEMENTED` rows | Each is implemented in `internal/component/ike/**` and proven by a tagged pair. Zero remain unimplemented. A row that could not be implemented has a Deviations entry carrying Thomas's answer to the STOP-and-ask escalation, with the date and the requirement id |
| AC-10 | Each of the 12 work packages, at its landing commit | `make ze-verify` is green, `make ze-rfc-check` exits 0, and `make ze-ipsec-interop-test` passes against strongSwan. No commit leaves the RFC gate red |
| AC-11 | An established IKE SA whose `NextMsgID` reaches `math.MaxUint32` | The SA is closed or rekeyed. It does NOT wrap to 0 and continue. The negative test asserts the teardown or rekey happened, not merely that no error was returned |
| AC-12 | A DPD liveness probe emitted by an established SA | It is an Encrypted (SK) INFORMATIONAL message, and its I bit is set only when the SA is the original initiator. Observed on the wire, not inferred from the builder |
| AC-13 | An authenticated Delete naming the live Child SA's SPI | The named Child SA's XFRM state is removed. A Delete naming an unknown SPI does not remove any state |
| AC-14 | An IKE_SA_INIT response carrying INVALID_KE_PAYLOAD with a group the initiator supports | The initiator retries IKE_SA_INIT with that group and re-proposes its full set of acceptable cryptographic suites. It does not mark the SA dead |
| AC-15 | A CREATE_CHILD_SA or IKE_AUTH request whose traffic selectors exceed the configured policy | The responder narrows them to the configured policy, or responds TS_UNACCEPTABLE when the narrowed result is empty. A rekey never yields selectors narrower than the original or wider than the scope in use |
| AC-16 | `rfc/enrolled.txt` and `docs/features/rfc-status.md` at closure | The rfc7296 descriptor and the rfc-status row describe the real position after this spec, with a source anchor to a producing `file:line`. Neither still advertises "2 gap" as the residual, and neither describes a compliance level the tagged tests do not prove |
| AC-17 | `rfc/short/rfc7296.md` Section Index at closure | §1.3.2 reads "Rekeying IKE SAs" and §1.3.3 reads "Rekeying Child SAs", matching `rfc/full/rfc7296.txt:847` and `:882`. `RFC7296-1.3.3-1` keeps its id and its `(§1.3.3)` citation, and the discrepancy is recorded in Known Limitations |
| AC-18 | The whole diff, grepped for annotations | `grep -n "{gap}\|{not-applicable}\|{single-polarity}" rfc/short/rfc7296.md` returns nothing for any of the 214 new rows. The three pre-existing annotations on `RFC7296-3.3-1`, `RFC7296-2.9-1` and `RFC7296-1.4-1` are re-derived: each is either cleared by the work that implements its obligation, or carries Thomas's fresh answer |

### Provenance: the four ACs carried from `plan/spec-rfcgate-1-extraction.md`

These four were written for phase 8 and are now this spec's. They are carried verbatim in
substance; where wording differs, it is because OR-1 removed the annotation escape that the
original AC-25 still allowed.

| Original (`plan/spec-rfcgate-1-extraction.md:643-646`) | Carried here as | Substantive change |
|---|---|---|
| AC-23 (`RFC7296-2.2-1` and `-2.2-2` present, quoting verbatim, citing `(§2.2)`) | AC-1 | None |
| AC-24 (166 enrolled, exactly one sign-off, every site and section classified, register derived, other 165 unaccused) | AC-5 | None |
| AC-25 (every newly extracted requirement carries a positive AND negative tag, OR an annotation authorised by Thomas and recorded in Deviations) | AC-4 | **Narrowed by OR-1.** The annotation branch is removed as a route: no annotation is permitted for any of the 214. The only surviving path to a non-proven row is Thomas's recorded answer to the STOP-and-ask escalation |
| AC-26 (all 23 existing ids present, polarities preserved, ratchets green, nothing renumbered or reused) | AC-3 | None |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Reads `docs/features/rfc-status.md` to decide whether Ze's IKEv2 is fit for their deployment | `rfc/short/rfc7296.md` rows -> `make ze-rfc-index` -> `ai/RFC-REQUIREMENTS.md` -> the rfc-status row | `TestRealTree.test_rfc7296_ledger_is_fresh_after_the_pilot` |
| 2 | Runs an IKEv2 tunnel long enough for the Message ID counter to exhaust | established SA -> `NextMsgID` producer -> exhaustion guard -> SA close or rekey | `TestMessageIDExhaustionClosesTheSA` |
| 3 | Runs Ze as an IKEv2 responder behind a NAT and expects the peer's replies to reach it | inbound datagram -> observed source address/port stored on the SA -> `sendRaw` -> reply to the observed endpoint | `test/ipsec-interop/scenarios/13-natt-source-port` |
| 4 | Configures a remote-access IPsec peer and expects Ze to hand out a virtual IP | operator YANG -> IKE_AUTH CFG_REQUEST -> `wire.PayloadCP` consumer -> `eap.Pool` allocation -> CFG_REPLY | `test/ipsec-interop/scenarios/14-remote-access-cp` |
| 5 | Configures narrower traffic selectors than the peer proposes and expects them enforced | operator policy -> IKE_AUTH TSi/TSr -> `narrowTS` in production -> XFRM policy install | `test/ipsec-interop/scenarios/15-ts-narrowing` |
| 6 | Tears down one Child SA from the peer side and expects Ze to stop forwarding into it | peer Delete -> `handleDeletePayload` reading `del.SPIs` -> `removeChildSA` -> XFRM state removed | `test/ipsec/ipsec-peer-delete-child.ci` |
| 7 | Configures a shared secret as a hex string | YANG `psk` leaf -> config parse -> hex decode -> AUTH computation | `test/parse/ipsec-psk-hex.ci` |

## 🧪 TDD Test Plan

Every test in this plan carries an `RFC requirement: <id> <positive|negative>` tag in the
form `ai/skills/ze-rfc.md:259-267` specifies. The tables below name the target files and
the shape; the per-row assignment is Appendix A, whose `Class` column selects the phase.

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestMessageIDExhaustionClosesTheSA` | `internal/component/ike/engine/rfc7296_test.go` | AC-11, `RFC7296-2.2-2` positive: an SA at `math.MaxUint32` is closed or rekeyed | |
| `TestMessageIDDoesNotWrapToZero` | `internal/component/ike/engine/rfc7296_test.go` | AC-11, `RFC7296-2.2-2` negative: the counter never returns to 0 on a live SA | |
| `TestRetransmitReusesOriginalMessageID` | `internal/component/ike/engine/rfc7296_test.go` | `RFC7296-2.2-1` positive, against `fsm.go:138-142` | |
| `TestRetransmitIsBitwiseIdentical` | `internal/component/ike/engine/rfc7296_test.go` | `RFC7296-2.1-8` positive and `2.2-1` negative: a differing Message ID is not a retransmission | |
| `TestDPDProbeIsEncrypted` | `internal/component/ike/engine/dpd_test.go` | AC-12, `RFC7296-1.4-2` positive: the probe carries an SK payload | |
| `TestDPDProbeIBitFollowsRole` | `internal/component/ike/engine/dpd_test.go` | AC-12, `RFC7296-3.1-12` positive and negative: set for an initiator SA, cleared for a responder SA | |
| `TestDeleteRemovesNamedChildSA` | `internal/component/ike/engine/inbound_test.go` | AC-13, `RFC7296-1.4.1-2` positive and negative | |
| `TestInitiatorRetriesOnInvalidKEPayload` | `internal/component/ike/engine/fsm_test.go` | AC-14, `RFC7296-1.2-5` positive; `1.2-6` positive for the full re-proposal | |
| `TestCookieRoundTrip` | `internal/component/ike/engine/responder_test.go` | `RFC7296-2.6-3`, `-4`, `-5` in both polarities | |
| `TestOutOfSAErrorHandling` | `internal/component/ike/engine/register_test.go` | `RFC7296-2.21.4-1` through `-7`, including the MUST NOT-respond and MUST NOT-change-state halves | |
| `TestTransformAttributeValidation` | `internal/component/ike/wire/payload_sa_test.go` | `RFC7296-3.3.5-1` through `-5`, `3.3-6`, `3.3-7` | |
| `TestProposalNumberingAndSPISize` | `internal/component/ike/wire/payload_sa_test.go` | `RFC7296-3.3-5`, `3.3.1-1`, `-2` | |
| `TestTrafficSelectorNarrowing` | `internal/component/ike/engine/child_test.go` | AC-15, `RFC7296-2.9-2`, `2.9.2-1`, `-2` | |
| `TestConfigurationPayloadExchange` | `internal/component/ike/engine/cp_test.go` | `RFC7296-2.19-*`, `3.15.1-*` | |
| `TestHexPSKDecoding` | `internal/component/ike/engine/auth_test.go` | `RFC7296-2.15-3` positive and negative | |
| `TestRealTree.test_rfc7296_signoff_is_valid_and_the_rest_stay_grandfathered` | `scripts/dev/rfc_requirements_test.py` | AC-5 | |
| `TestRealTree.test_rfc7296_summary_carries_the_section_2_2_requirements` | `scripts/dev/rfc_requirements_test.py` | AC-1 | |
| `TestRealTree.test_rfc7296_every_gated_row_is_proven_in_both_polarities` | `scripts/dev/rfc_requirements_test.py` | AC-2, AC-4: no gated rfc7296 row lacks a polarity, and none carries an annotation | |
| `TestRealTree.test_rfc7296_ids_are_neither_retired_nor_demoted` | `scripts/dev/rfc_requirements_test.py` | AC-3, driving `check_retired_requirements` and `check_coverage_ratchet` | |
| `TestRealTree.test_rfc7296_ledger_is_fresh_after_the_pilot` | `scripts/dev/rfc_requirements_test.py` | Story 1: `ai/RFC-REQUIREMENTS.md` is regenerated in the same commit as any tagged-test move | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| IKE Message ID (`RFC7296-2.2-2`) | 0..4294967295 | 4294967295 | N/A | 4294967296 (the wrap this spec forbids: the SA closes or rekeys at the boundary) |
| COOKIE notification data (`RFC7296-2.6-3`) | 1..64 octets | 64 | 0 | 65 |
| Nonce data (`RFC7296-3.9-1`) | 16..256 octets | 256 | 15 | 257 |
| SET_WINDOW_SIZE data (`RFC7296-2.3-1`) | exactly 4 octets | 4 | 3 | 5 |
| IKE message length (`RFC7296-2-1`) | must handle >= 1280 | 1280 | N/A | N/A (a smaller ceiling fails the MUST) |
| Traffic selector Start/End Port (`RFC7296-3.13.1-1`, `-2`, `-3`) | 0..65535 | 65535 | N/A | 65536; plus the OPAQUE encoding start=65535 end=0 |
| Delete SPI Size (`RFC7296-3.11-2`) | 0 for IKE, 4 for AH/ESP | 4 | 3 | 5 |
| PRF output size (`RFC7296-5-1`) | >= 128 bits | 128 | 127 | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipsec-dpd-probe-encrypted` | `test/ipsec/ipsec-dpd-probe-encrypted.ci` | A running tunnel's liveness probe is an encrypted INFORMATIONAL, observable through the daemon | |
| `ipsec-peer-delete-child` | `test/ipsec/ipsec-peer-delete-child.ci` | The peer deletes one Child SA and `show ipsec sa` stops listing it | |
| `ipsec-msgid-exhaustion` | `test/ipsec/ipsec-msgid-exhaustion.ci` | An SA driven to Message ID exhaustion is torn down or rekeyed rather than wrapping | |
| `ipsec-ts-narrowing` | `test/ipsec/ipsec-ts-narrowing.ci` | A peer proposing wider selectors than policy gets the narrowed set installed | |
| `ipsec-remote-access-cp` | `test/ipsec/ipsec-remote-access-cp.ci` | A remote-access peer receives a virtual IP through CFG_REPLY | |
| `ipsec-psk-hex` | `test/parse/ipsec-psk-hex.ci` | A hex-encoded shared secret parses and authenticates | |
| `ipsec-cookie-challenge` | `test/ipsec/ipsec-cookie-challenge.ci` | Under half-open pressure the responder issues a COOKIE and the exchange completes on retry | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `12-invalid-ke-retry` | `test/ipsec-interop/scenarios/` | strongSwan | Ze as initiator retries IKE_SA_INIT after INVALID_KE_PAYLOAD and establishes (AC-14) | |
| `13-natt-source-port` | `test/ipsec-interop/scenarios/` | strongSwan | Behind a NAT, Ze replies from and to the observed port and the tunnel carries traffic | |
| `14-remote-access-cp` | `test/ipsec-interop/scenarios/` | strongSwan | A strongSwan road-warrior receives a virtual IP from Ze through the Configuration payload | |
| `15-ts-narrowing` | `test/ipsec-interop/scenarios/` | strongSwan | strongSwan proposes wider selectors; Ze narrows them and strongSwan accepts the narrowed set (AC-15) | |
| `16-transport-mode` | `test/ipsec-interop/scenarios/` | strongSwan | USE_TRANSPORT_MODE is negotiated and the Child SA is transport mode | |
| `17-crossing-deletes` | `test/ipsec-interop/scenarios/` | strongSwan | Both ends issue Deletes simultaneously; outgoing SAs go on the request, incoming on the response, and no duplicate Delete is echoed | |
| `18-cookie-challenge` | `test/ipsec-interop/scenarios/` | strongSwan | strongSwan completes an exchange against a Ze responder issuing COOKIE | |
| `19-error-notifications` | `test/ipsec-interop/scenarios/` | strongSwan | strongSwan receives INVALID_SYNTAX / UNSUPPORTED_CRITICAL_PAYLOAD and reacts as the RFC specifies | |
| `20-ipcomp` | `test/ipsec-interop/scenarios/` | strongSwan | IPComp is negotiated, and a non-proposed algorithm is refused | |

**Vacuity check (BLOCKING, `ai/rules/interop-and-goal-validation.md`).** Each scenario
above must be shown to FAIL when its behaviour is reverted. Two traps apply directly here:
a receiver that accepts any conforming form (so a sender-side change leaves the peer's
state identical), and an assertion of ABSENCE (so deleting the mechanism yields the same
absence). Every scenario records which mutation turned it red.

## Files to Modify
- `rfc/short/rfc7296.md` - re-authored from 23 rows to 237. Existing 23 rows byte-identical; Section Index §1.3.2/§1.3.3 corrected
- `rfc/enrolled.txt` - the rfc7296 descriptor at `:159`, re-authored against the real position (AC-16)
- `docs/features/rfc-status.md` - the rfc7296 row at `:213`: Status, Implemented coverage, and the Remaining column that today discloses exactly two MUST gaps (AC-16)
- `ai/RFC-REQUIREMENTS.md` - regenerated by `make ze-rfc-index` in the same commit as any tagged-test addition or move
- `internal/component/ike/engine/sa.go` - the Message ID counter type and the observed peer endpoint (WP-1, WP-8)
- `internal/component/ike/engine/dpd.go` - encrypt the probe, use `initiatorFlag(sa)` (WP-2)
- `internal/component/ike/engine/fsm.go` - INVALID_KE_PAYLOAD and COOKIE consumption, response-to-response guard, Message ID producers (WP-1, WP-4, WP-5)
- `internal/component/ike/engine/inbound.go` - Delete by SPI, Message ID producers (WP-1, WP-2)
- `internal/component/ike/engine/established.go` - reply to the observed endpoint (WP-8)
- `internal/component/ike/engine/responder.go` - error notification emission, COOKIE issuance, TS narrowing, proposal echo rules (WP-3, WP-4, WP-6, WP-7)
- `internal/component/ike/engine/register.go` - out-of-SA handling and INVALID_IKE_SPI (WP-3); the 4500 socket becoming send-capable (WP-8)
- `internal/component/ike/engine/msgid.go` - window handling beyond 1 (WP-1)
- `internal/component/ike/engine/child.go` - `narrowTS` into production; transport-mode Child SAs (WP-7)
- `internal/component/ike/engine/auth.go` - certificate chain, hash-and-URL, hex PSK (WP-10)
- `internal/component/ike/engine/rekey.go` - Message ID producers, TEMPORARY_FAILURE (WP-1)
- `internal/component/ike/wire/payload_sa.go` - transform and attribute validation (WP-6)
- `internal/component/ike/wire/payload_notify.go` - notify payload Protocol ID and SPI rules (WP-12)
- `internal/component/ike/wire/payload_ts.go` - port range encoding (WP-7)
- `internal/component/ike/wire/payload_cert.go` - hash-and-URL encoding (WP-10)
- `internal/component/ike/crypto/proposal.go` - real per-transform negotiation replacing exact-tuple matching (WP-6)
- `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` - remote-access configuration, ID type configurability, certificate count, hex PSK (WP-9, WP-10)
- `scripts/dev/rfc_requirements_test.py` - the `TestRealTree` cases listed in the Unit Tests table

## Files to Create
- `rfc/extraction/rfc7296.json` - the pilot sign-off: every derived site classified `mapped` or `excluded`, every section `walked` or `skipped`, register derived, `signed-off` and `reviewer` populated
- `plan/deferrals/rfcgate-1b-rfc7296-pilot.md` - the shard named in the metadata table
- `internal/component/ike/engine/cp.go` - the Configuration payload producer and consumer, the first production user of `wire.PayloadCP` (WP-9)
- `internal/component/ike/engine/cookie.go` - COOKIE generation, binding and validation (WP-4)
- `internal/component/ike/engine/notify_error.go` - the error-notification emitter, including the unprotected out-of-SA path (WP-3)
- `internal/component/ike/engine/cp_test.go`, `cookie_test.go`, `notify_error_test.go` - the tagged pairs for those packages
- `test/ipsec/ipsec-dpd-probe-encrypted.ci`, `ipsec-peer-delete-child.ci`, `ipsec-msgid-exhaustion.ci`, `ipsec-ts-narrowing.ci`, `ipsec-remote-access-cp.ci`, `ipsec-cookie-challenge.ci` - the functional tests
- `test/parse/ipsec-psk-hex.ci` - hex PSK config parse
- `test/ipsec-interop/scenarios/12-invalid-ke-retry/` through `20-ipcomp/` - the nine interop scenarios, each with `ze.conf`, the strongSwan config and `check.py`
- A driven-clock seam for `maintainSA` so the timeout and rate-limit obligations (`RFC7296-2.4-6`, `-7`, `-9`) are provable without wall-clock sleeps (`ai/rules/fix-dont-record.md`)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang`: remote-access address pool and attributes (WP-9); ID type configurability `RFC7296-3.5-2`/`-3`/`-4`, certificate count `RFC7296-3.6-1`, hex PSK `RFC7296-2.15-3`, IKE suite acceptance controls `RFC7296-3.3.4-1` (WP-10). Read `ai/rules/config-surface.md` and `ai/rules/config-naming.md` before adding a leaf |
| YANG validation constraints | Yes | Every new leaf takes native validation: the address pool is `zt:ip-prefix`, the certificate count a bounded `uint8` range, the PSK encoding an `enumeration`. `ai/patterns/config-option.md` |
| YANG custom validators | Yes | The hex PSK needs `ze:validate` because its validity is an encoding property native constraints cannot express |
| CLI commands/flags | Yes | `show ipsec sa` gains the assigned virtual IP and the negotiated mode (tunnel/transport). Existing verbs only; no new root command |
| CLI grammar (keyword before value) | Yes | Any new selector follows `ai/rules/cli-grammar.md`; the ipsec commands already use typed selectors |
| Editor autocomplete | Yes | Automatic for the new enum and typed leaves; the address pool needs no `CompleteFn` |
| Functional test for new RPC/API | Yes | `test/ipsec/*.ci` per the Functional Tests table |
| Pipe completeness | Yes | The new `show ipsec sa` fields flow through the existing `ApplyPipes` path; no new display mode is added |
| Env var registration | N-A | No leaf lands under `environment/` |
| Doctor check for runtime dependencies | Yes | WP-9's address pool is a config-declared resource, and WP-10's hash-and-URL fetch is an outbound network dependency. Both need an owning-package check plus a `doctor-` code in `internal/core/diagnostic/codes.go` (`ai/rules/doctor-checks.md`) |
| Prometheus counters/metrics | Yes | Message ID exhaustion events, COOKIE challenges issued, and error notifications emitted are all operationally significant and rate-limited; define and register them in the owning package |
| BGP family surface (new SAFI / capability / attribute) | N-A | No BGP surface is touched |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md`: remote-access IPsec (virtual IP assignment) and transport-mode Child SAs are new user-facing capabilities delivered by WP-7 and WP-9 |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` and `docs/architecture/config/syntax.md` for the new ipsec leaves |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` for the new `show ipsec sa` fields |
| 4 | API/RPC added/changed? | No | No RPC is added; the changes are to existing handlers' output shape, covered by row 3 |
| 5 | Plugin added/changed? | No | IKE is a component, not a plugin, and no registration surface changes |
| 6 | Has a user guide page? | Yes | The IPsec guide page gains remote-access and transport-mode sections |
| 7 | Wire format changed? | Yes | `docs/architecture/wire/*.md`: the Configuration payload, COOKIE, and the encrypted INFORMATIONAL probe are wire-visible changes |
| 8 | Plugin SDK/protocol changed? | No | No SDK or IPC surface changes |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc7296.md` (237 rows) and `docs/features/rfc-status.md:213`, both with source anchors to producing `file:line`; reconcile the `rfc/enrolled.txt:159` descriptor (AC-16) |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` for the nine new interop scenarios and the driven-clock seam; `docs/contributing/rfc-implementation-guide.md` for the pilot's method |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md`: remote-access IPsec and transport mode are comparison-relevant capabilities against strongSwan and VyOS |
| 12 | Internal architecture changed? | Yes | The IKE subsystem doc: the window model, the observed-endpoint state on the SA, and the Configuration-payload path |
| 13 | Route metadata keys added/changed? | No | None |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` or the IPsec telemetry doc, for the counters named in the Integration Checklist |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | No registration surface changes |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Grep `docs/` for anchors naming every file in Files to Modify. `docs/features/rfc-status.md:213` is already known to anchor `engine/child.go:351` and `engine/inbound.go:270-276`, both of which this spec changes |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Verify every ipsec config example against the changed YANG and the changed `show` output |

## Implementation Steps

Phases 1 to 3 are sequential. Phase 3's twelve work packages are ordered by dependency and
risk, and each is its own landing (see Landing Strategy).

1. **Phase: Wiring (MANDATORY FIRST)** -- make the gate see the pilot before any row is written
   - Tests: the five `TestRealTree` cases from the Wiring Test table, each written to FAIL
     first: the sign-off artifact does not exist, and the summary carries 23 rows not 237
   - Files: `scripts/dev/rfc_requirements_test.py`; an unsigned skeleton at
     `rfc/extraction/rfc7296.json` from `make ze-rfc-extract STEM=rfc7296` (`Makefile:452`)
   - Also in this phase: fetch and reconcile the RFC 7296 errata (A-10); re-walk the source
     against Appendix A to validate the 214 (A-1, A-5); confirm the derived register (A-6);
     confirm rfc7296 is unaccused on an unmodified tree (A-3)
   - Verify: the five tests fail for the right reason; `make ze-rfc-check` is green on the
     unmodified tree with zero artifacts judged

2. **Phase: Re-author the summary with the 88 provable rows** -- extraction plus the proof that already exists
   - Tests: 88 tagged pairs (176 tagged tests) for the 63 `implemented-and-testable` and
     the 25 `implemented-untested` rows, each naming the producing `file:line` recorded in
     Appendix A's source column
   - Files: `rfc/short/rfc7296.md` (23 existing rows byte-identical, Section Index
     corrected, 88 rows added); the test files named in the Unit Tests table; the
     driven-clock seam and the first interop scenarios for the ~11 rows that need a peer
   - Verify: `check_retired_requirements` and `check_coverage_ratchet` green across the
     re-authoring (AC-3); `make ze-rfc-check` exits 0; AC-1, AC-6, AC-7 demonstrated
   - Self-critical review before phase 3: has any row been added whose pair does not
     actually gate? Mutation-verify a sample by disabling the producing function

3. **Phase: Resolve the 18 `uncertain` rows** -- reading, not guessing
   - Tests: none yet; this phase produces classifications, not code
   - Files: read `internal/component/ike/eap/**` (`peer.go`, `eap.go`, `eap_tls.go`,
     `eap_mschapv2.go`, `mschapv2.go`), `engine/responder_eap.go` in full,
     `wire/payload_eap.go`, `wire/payload_id.go`, `crypto/dh.go`,
     `dataplane/xfrm_linux.go`, and `handleAuthResponse` from `fsm.go:507`
   - Verify: AC-8 -- each of the 18 carries a producing `file:line` and a class. Rows that
     resolve to implemented join phase 2's pattern; rows that resolve to not-implemented
     join their work package below and the package's row count is updated in this spec

4. **Phase: WP-1 -- Message ID lifecycle and the request window** (7 rows: `2.2-2`, `2.3-1`, `2.3-3`, `2.3-5`, `2.3-6`, `2.25-1`, `3.1-10`)
   - Tests: `TestMessageIDExhaustionClosesTheSA`, `TestMessageIDDoesNotWrapToZero`, window
     acceptance boundary tests, TEMPORARY_FAILURE back-off
   - Files: `engine/sa.go`, `engine/msgid.go`, `engine/fsm.go`, `engine/inbound.go`,
     `engine/rekey.go`, `engine/dpd.go`
   - Verify: AC-11; the accept predicate is bounded and the Security Review's WP-1
     invariant holds; `make ze-ipsec-interop-test` green

5. **Phase: WP-2 -- INFORMATIONAL encryption, DPD correctness, Delete by SPI** (4 rows: `1.4-2`, `3.1-12`, `1.4.1-2`, `1.5-1`)
   - Tests: `TestDPDProbeIsEncrypted`, `TestDPDProbeIBitFollowsRole`,
     `TestDeleteRemovesNamedChildSA`; `.ci` tests `ipsec-dpd-probe-encrypted`, `ipsec-peer-delete-child`
   - Files: `engine/dpd.go`, `engine/inbound.go`
   - Verify: AC-12, AC-13; the Message ID correlation in `handleDPDResponse` survives

6. **Phase: WP-5 -- header flags, version negotiation, critical bit** (4 rows: `2.5-5`, `2.5-12`, `3.2-1`, `3.12-1`)
   - Tests: header builder and parser pairs in `wire/header_test.go` and `wire/message_test.go`
   - Files: `engine/fsm.go`, `engine/responder.go`, `wire/payload.go`
   - Verify: low-risk, wire-visible; interop green

7. **Phase: WP-12 -- notify shape, expired SA, INITIAL_CONTACT, NO_ADDITIONAL_SAS** (5 rows: `3.10-1`, `3.10-2`, `2.8-4`, `2.4-5`, `4-1`)
   - Tests: notify codec pairs; an expired SA is not used; the NO_ADDITIONAL_SAS fallback
   - Files: `wire/payload_notify.go`, `engine/established.go`, `engine/rekey.go`
   - Verify: interop green

8. **Phase: WP-3 -- error notification emission and consumption** (13 rows: `2.5-10`, `2.21.2-1..-3`, `2.21.3-1`, `2.21.4-1..-7`, `3.10.1-3`)
   - Tests: `TestOutOfSAErrorHandling` plus the MUST NOT-respond and MUST NOT-change-state
     halves; interop `19-error-notifications`
   - Files: `engine/notify_error.go` (new), `engine/register.go`, `engine/responder.go`,
     `wire/message.go`
   - Verify: every added response is rate limited and none creates a packet loop
     (Security Review, WP-3)

9. **Phase: WP-4 -- IKE_SA_INIT retry: INVALID_KE_PAYLOAD and COOKIE** (6 rows: `1.2-5`, `1.2-6`, `2.6-3`, `2.6-4`, `2.6-5`, `2.6.1-1`)
   - Tests: `TestInitiatorRetriesOnInvalidKEPayload`, `TestCookieRoundTrip`; `.ci`
     `ipsec-cookie-challenge`; interop `12-invalid-ke-retry`, `18-cookie-challenge`
   - Files: `engine/fsm.go`, `engine/cookie.go` (new), `engine/responder.go`
   - Verify: AC-14; the cookie is bound to the initiator address and a rotating secret

10. **Phase: WP-6 -- proposal, transform and attribute validation** (22 rows: `3.3-5`, `-6`, `-7`, `3.3.1-1`, `-2`, `3.3.3-1`, `3.3.4-2`, `-3`, `3.3.5-1..-5`, `3.3.6-2`, `-4`, `-5`, `-6`, `2.18-1`, `3.4-2`, `-3`, `3.14-1`, `5-1`)
    - Tests: `TestTransformAttributeValidation`, `TestProposalNumberingAndSPISize`, and the
      per-transform unacceptability pairs
    - Files: `crypto/proposal.go`, `wire/payload_sa.go`, `engine/responder.go`
    - Verify: the largest package; every new rejection is reachable pre-authentication and
      must not panic or allocate unboundedly on attacker-controlled counts

11. **Phase: WP-8 -- NAT-T source port and established-path addressing** (6 rows: `2.11-2`, `-3`, `2.23-4`, `-5`, `-6`, `2.23-8`)
    - Tests: interop `13-natt-source-port`
    - Files: `engine/sa.go` (observed endpoint), `engine/established.go`,
      `engine/register.go` (the 4500 socket becomes send-capable)
    - Verify: an unauthenticated message never mutates the stored peer endpoint
      (Security Review, WP-8)

12. **Phase: WP-7 -- traffic selectors, transport mode, port ranges** (11 rows: `2.9-2`, `2.9.2-1`, `-2`, `2.23.1-1..-3`, `1.3.1-1`, `-2`, `3.13.1-1..-3`)
    - Tests: `TestTrafficSelectorNarrowing`; `.ci` `ipsec-ts-narrowing`; interop
      `15-ts-narrowing`, `16-transport-mode`
    - Files: `engine/child.go` (`narrowTS` into production), `engine/responder.go`,
      `wire/payload_ts.go`
    - Verify: AC-15; this also clears the pre-existing `{gap}` on `RFC7296-2.9-1` (AC-18)

13. **Phase: WP-11 -- IPComp** (4 rows: `2.22-1..-4`)
    - Tests: interop `20-ipcomp`
    - Files: `engine/child.go`, `wire/payload_notify.go`
    - Verify: a non-proposed algorithm is refused

14. **Phase: WP-10 -- certificates, identities, management interface** (9 rows: `3.6-1`, `-2`, `-3`, `3.5-2`, `-3`, `-4`, `3.3.4-1`, `4-4`, `2.15-3`)
    - Tests: `TestHexPSKDecoding`; `.ci` `ipsec-psk-hex`; certificate chain pairs
    - Files: `engine/auth.go`, `wire/payload_cert.go`, `ipsec/yang/ze-ipsec-conf.yang`
    - Verify: hash-and-URL fetching is bounded (SSRF surface, Security Review WP-10);
      chain validation is not weakened by the higher certificate count

15. **Phase: WP-9 -- Configuration payload and remote access** (17 rows: `2.19-1..-6`, `2.20-1`, `3.15.1-1..-7`, `4-2`, `-3`, `1.7-1`)
    - Tests: `TestConfigurationPayloadExchange`; `.ci` `ipsec-remote-access-cp`; interop
      `14-remote-access-cp`
    - Files: `engine/cp.go` (new), `engine/responder.go`, `ipsec/yang/ze-ipsec-conf.yang`,
      the `eap.Pool` wiring
    - Verify: the authorization checks fail closed (`2.19-5`, `2.19-6`); the full
      Integration Checklist is re-answered for this package's config surface (R-8)

16. **Phase: Sign-off and reconciliation**
    - Tests: the five `TestRealTree` cases now pass
    - Files: `rfc/extraction/rfc7296.json` completed and signed; `rfc/enrolled.txt:159`;
      `docs/features/rfc-status.md:213`; `ai/RFC-REQUIREMENTS.md` regenerated
    - Verify: AC-2, AC-4, AC-5, AC-9, AC-16, AC-17, AC-18; `make ze-rfc-check` exits 0 with
      exactly one sign-off present and 165 unsigned

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every one of the 214 rows has a tagged pair at `file:line`, and every one of the 108 has an implementation at `file:line` |
| Feature completeness | Every user story has a working path: no story ends at a codec with no production consumer, which is exactly the `wire.PayloadCP` failure this spec fixes |
| **No annotation smuggled in (umbrella R-9)** | `grep -n "{gap}\|{not-applicable}\|{single-polarity}\|partial" rfc/short/rfc7296.md` over the diff. The gate CANNOT catch this: a `{gap}` is a legal annotation. This row is the only defence, and OR-1 makes any hit a blocker unless Thomas's answer is recorded in Deviations with its date and the requirement id |
| **Pre-existing annotations re-derived** | The three annotations at HEAD (`RFC7296-3.3-1` `:462`, `RFC7296-2.9-1` `:467`, `RFC7296-1.4-1` `:473`) are VOID as authority (`ai/rules/rfc-compliance.md:53`). Each is either cleared by the work that implements it (WP-7 clears `2.9-1`; WP-2 clears `1.4-1`) or re-raised with Thomas |
| **Id integrity** | No existing id renumbered, reused or retired. `RFC7296-1.3.3-1` keeps its id AND its `(§1.3.3)` citation, because `parse_checklist_line` validates that they agree |
| **Tagged-test integrity** | No existing tagged test's behaviour changed. The `rfc-tagged-test` hook blocks it and `// test-relax:` does not satisfy that gate; only the user can authorise it |
| Correctness (fail-closed) | Every new guard denies on a miss, an unmapped input, an empty set or an error. A zero Message ID, zero SPI, empty TS set and empty attribute list are attacker-reachable and none takes a permissive default (`ai/rules/fail-closed-guards.md`) |
| Correctness (test vacuity) | Each interop scenario was shown to fail when its behaviour was reverted. A receiver that accepts any conforming form, and an assertion of absence, are the two traps that apply here |
| Data flow | The compliance half authors DATA the machinery judges; `scripts/dev/rfc_requirements.py` is not modified by this spec |
| Naming | New YANG leaves follow `ai/rules/config-naming.md`; dimensioned leaves carry a `units` statement and a unit-free name (`ai/rules/yang-structure.md`) |
| Rule: `ai/rules/no-parking.md` | No row is moved to `tmp/`, filed in a deferral, or recorded in `plan/known-failures/` instead of being fixed. The deferral shard exists for genuinely separable work only, and OR-1 means none of the 214 qualifies |
| Rule: `ai/rules/rfc-compliance.md` | Every escalation to Thomas quotes the requirement id, the RFC text verbatim with its locator, the producing `file:line`, the cost, and asks "which way", never "may I skip it" |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| 237 rows in the summary | `grep -c "^- \[ \] \[RFC7296-" rfc/short/rfc7296.md` returns 237 |
| Zero annotations across the 214 | `grep -n "{gap}\|{not-applicable}\|{single-polarity}" rfc/short/rfc7296.md` returns nothing not covered by a recorded Thomas answer |
| Both polarities on every gated row | `make ze-rfc-check` exits 0 |
| 428 tagged tests for the new rows | `grep -rc "RFC requirement: RFC7296" internal/` is at least 459 (31 existing + 428) |
| The sign-off validates | `make ze-rfc-check` names no `rfc/extraction/rfc7296.json` error; `make ze-rfc-extraction-status` shows rfc7296 signed (the target always emits JSON; spelling it `--json` makes GNU make exit 2 with `unrecognized option` before any recipe runs, corrected 2026-07-29) |
| Exactly one sign-off, 165 unsigned | `ls rfc/extraction/*.json` returns one file |
| Ledger fresh | `make ze-rfc-index` produces no diff in `ai/RFC-REQUIREMENTS.md` |
| Interop green | `make ze-ipsec-interop-test` passes, including the nine new scenarios |
| Functional green | `make ze-functional-test` passes, including the seven new `.ci` tests |
| Existing ids intact | `check_retired_requirements` and `check_coverage_ratchet` green |
| Pre-commit gate | `make ze-verify` passes |

### Security Review Checklist
The full analysis is the `## Security Review` section above. These are the mechanical
checks a reviewer runs against the diff.

| Check | What to look for |
|-------|-----------------|
| Input validation | Every new parse path over attacker-controlled bytes bounds its iteration by a length the header declares, not by an attacker-supplied count, and rejects rather than truncating |
| Accept-predicate widening | Any change to `classifyInbound` or the Message ID window is examined for what it now ACCEPTS. A widened window is a widened replay surface, and the SET_WINDOW_SIZE value is peer-supplied |
| Unauthenticated response surface | Every notification added by WP-3 is checked against `RFC7296-2.21.4-3` (not cryptographically protected), `-5` (recipient MUST NOT respond) and `-6`/`-7` (MUST NOT change SA state). A missing MUST NOT-respond is a packet loop between two Ze instances |
| Rate limiting | Every new unauthenticated emission is rate limited (`RFC7296-2.4-7`, `2.3-6`). An unlimited notify is an amplification primitive |
| Endpoint adoption | The observed source address/port (WP-8) is used for the reply only; an established SA's stored peer endpoint changes only on an authenticated message |
| Authorization fails closed | `RFC7296-2.19-5` and `2.19-6` are authorization checks. Neither may fall through to the permissive branch on a miss (`ai/rules/fail-closed-guards.md`) |
| Outbound fetch | Hash-and-URL certificate lookup (`RFC7296-3.6-2`, `-3`) fetches an attacker-named http URL. Bound the size, the timeout and the redirect count, and state whether the scheme allowlist is enforced |
| Secret handling | The COOKIE secret rotates and is never derived from a value the peer supplies. Hex PSK decoding does not change how an existing ASCII PSK is interpreted |
| Resource exhaustion | The COOKIE path exists to bound half-open state; verify it actually reduces per-peer state before authentication rather than adding a second table |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior; if misunderstood -> RESEARCH |
| Lint failure | Fix inline; if architectural -> DESIGN |
| Functional test fails | Check the AC: wrong AC -> DESIGN, correct AC -> IMPLEMENT |
| Interop scenario reds after a work package | The package's own defect. Fix it in that package; never relax the scenario (`ai/rules/no-parking.md`) |
| `make ze-rfc-check` reds on a landing commit | A row landed without its proof. Split the commit so the row and its pair land together |
| A row cannot be implemented | **STOP.** Escalate to Thomas with the id, the RFC text verbatim and its locator, the producing `file:line`, and the cost. Ask which way to fix it. Never annotate, never defer, never drop. The spec stays OPEN |
| The 214 count is wrong | Re-derive from the source, update Appendix A, and record the correction in Deviations. The coverage is the deliverable, not the count |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- **A green compliance gate is bounded by what somebody wrote down.** rfc7296 was 15 MET
  with 2 disclosed gaps and fully green, over a summary capturing 8% of the MUST surface.
  The gate was not lying; it was answering a smaller question than its readers assumed.
  That is exactly the failure `rfc/extraction/README.md:10-15` describes, and it is why the
  sign-off artifact exists.
- **The pilot was chosen well.** rfc7296 is the worst-measured input in the corpus, so if
  the artifact format survives it, the remaining 165 are easier by construction. Choosing
  an easy stem would have proven nothing.
- **A dead codec is a specific, greppable failure shape.** `wire.PayloadCP` is complete,
  registered in the decoder table, round-trip tested, and has no production consumer. It
  reads as implemented from every angle except the one that matters. Seventeen MUST
  obligations depend on it. The same shape appears eight more times in this RFC as notify
  constants with no producer.
- **"Implemented" and "proven" are different claims and the gap is measurable.** 88 of the
  214 obligations are already satisfied by shipped code and none of them was proven; 108
  are not satisfied at all. Without the walk, both sets looked identical from outside: both
  were absent from the summary.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Fix all 108 inside this spec | Annotate them under Thomas's authorisation (the narrower option actually offered); split into 8-12 separate compliance specs as the walk's own cost model suggested | Owner ruling OR-1, 2026-07-29. Thomas chose the complete fix. Splitting would re-create the disclosure problem: an annotated row reads as a decision, and `ai/rules/rfc-compliance.md:53` voids exactly those decisions |
| Run after child 4 rather than inside child 1 | Keep phase 8 inside `plan/spec-rfcgate-1-extraction.md` | Owner ruling OR-2. IKEv2 compliance work is orders of magnitude larger than the machinery it validates; serialising the rfcgate set behind it would strand four machinery specs |
| Land row and proof in the same commit | Land all 237 rows first, then prove them | A row without its pair reds `ze-rfc-check`, and a red gate blocks every session's commits (`commit_helper.py` refuses a script over a non-green verify). Same-commit landing keeps the gate green at every point without any bypass |
| Partition the 108 into 12 subsystem work packages | A flat list of 108 rows worked in id order | Id order interleaves unrelated subsystems and forces the same file to be reopened dozens of times. The packages are verified to partition the 108 exactly: no duplicate, no gap |
| Correct the Section Index but not the id | Renumber `RFC7296-1.3.3-1` to `RFC7296-1.3.2-N` | Blocked three ways: `check_id_allocation` forbids reuse, `check_retired_requirements` forbids disappearance, and two tests already tag the id. `parse_checklist_line` also validates that an id's section segment matches its `(§X.Y)` citation, so the citation must stay `(§1.3.3)` too |
| Carry Appendix A in the spec | Reference `tmp/rfcgate-1-rfc7296-walk.md`; write a separate committed data file | `tmp/` is gitignored, so the walk output would be lost at the first clean. A separate data file was not created because this spec's authoring scope is one file. The rows are data, not code, so `ai/rules/spec-no-code.md` is satisfied |
| Build the driven-clock seam rather than sleep | Wall-clock sleeps in the timeout and rate-limit tests | `ai/rules/fix-dont-record.md`: a test that waits a duration instead of a condition is a broken test, and the load-sensitivity would surface later as a phantom flake |

## Known Limitations
- **`RFC7296-1.3.3-1` continues to cite §1.3.3 while its obligation lives at §1.3.2**
  (`rfc/full/rfc7296.txt:857`, under the heading at `:847`). The id cannot move and the
  citation cannot diverge from the id, so the discrepancy is permanent. It is recorded
  here, in the summary's Section Index note, and in the sign-off artifact's section notes
  so a future reader is not misled. This is a labelling defect, not a compliance one: the
  obligation itself is captured and proven.
- **The sign-off bounds sites the extractor can SEE, not obligations that exist.**
  `rfc/extraction/README.md:26-31` is explicit that recall differs by section and can be
  near zero for normative prose written without RFC 2119 keywords. `unsourced-ids` is how
  the residual is published rather than claimed away. A signed rfc7296 does not mean 100%
  of RFC 7296's obligations are captured; it means every keyword site is accounted for and
  the residual is visible.
- **Errata are checked once, at phase 1.** An erratum published after that lands is not
  detected by any gate here. The `source-sha` pin detects a change to the local source
  text, not a change upstream.
- **71 of the 214 rows split a compound RFC sentence** to satisfy one-obligation-per-line,
  restoring the subject onto the second and third clauses. Every such row carries its
  `(§X.Y)` citation so the untouched text is one lookup away. If Thomas prefers strict
  verbatim, those rows merge back into compound rows and the count falls (R-1, escalation
  E-4 from the walk).

## RFC Documentation (Scope: protocol)

Add `// RFC 7296 Section X.Y: "<quoted requirement>"` above every enforcing code path
introduced by the 12 work packages. MUST document: validation rules, error conditions,
state transitions, timer constraints, message ordering, and every MUST/MUST NOT.

Two documentation obligations are specific to this spec:

1. **Every implementation of one of the 108 carries the requirement id in its comment**,
   not only the section number, so a reader can move between the code, the summary row and
   `ai/RFC-REQUIREMENTS.md` without a search. The tagged test carries the same id in its
   `RFC requirement:` tag.
2. **Wire-format changes get an ASCII diagram with byte offsets** per
   `ai/rules/rfc-compliance.md`: the Configuration payload (WP-9), the COOKIE notification
   (WP-4), and the encrypted INFORMATIONAL probe (WP-2) all change what appears on the
   wire and none is currently documented under `docs/architecture/wire/`.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-18 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes (the pre-commit gate; `ai/rules/git-safety.md`)
- [ ] `make ze-rfc-check` exits 0 with exactly one sign-off present and 165 unsigned
- [ ] `make ze-ipsec-interop-test` passes against strongSwan, including the nine new scenarios
- [ ] All 214 rows carry both polarities; ZERO annotations (OR-1)
- [ ] All 108 previously unimplemented obligations are implemented
- [ ] Feature code integrated (`internal/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features, each shown to fail when its behavior is reverted

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

## Appendix A -- the 214 proposed rows

Carried from the 2026-07-29 section-by-section walk. `tmp/` is gitignored, so this table is
the surviving record; the walk report at `tmp/rfcgate-1-rfc7296-walk.md` must not be
treated as a live reference.

**Provenance and validation.** Every row parses through
`scripts/dev/rfc_requirements.py:parse_checklist_line`, every id passes `_validate_id`, and
every id's section segment matches its `(§X.Y)` citation. Against the 23 existing ids:
zero collisions, zero internal duplicates, zero ordinals colliding with an existing
per-section high-water mark. Levels: 168 MUST and 46 MUST NOT.

**Revalidated 2026-07-30 (phase 1, A-1 and A-5).** A second independent pass re-measured
this table against `rfc/full/rfc7296.txt`. The count holds at 214. The class tally holds at
63 / 25 / 18 / 108, and the level tally at 168 / 46. All 214 rows parse with zero errors.

`check_id_allocation` is clean over the 23 existing ids plus these 214. The source carries
every obligation inside the section its row cites. No run of four or more words fell
outside a cited section.

**Errata reconciled 2026-07-30 (A-10).** RFC 7296 carries nine errata, and two touch this
table. Erratum 6940 is Verified and Technical. It corrects §3.10 from "the field must be
empty" to "the SPI field must be empty". Row `RFC7296-3.10-3` now quotes the corrected
text.

Erratum 5056 is Held for Document Update and Technical. It reports that the word
"proposals" in §1.7 is wrong. Configuration attribute 5 belongs to a Configuration payload,
not to an SA proposal. The published text stands, so row `RFC7296-1.7-1` still quotes it.

WP-9 must implement the reading the verifier recorded. It must ignore that one attribute
alone, and never the whole proposal. The phase-1 findings cover the other seven errata.

**Quote fidelity measured 2026-07-30 (phase 2a).** Every one of the 47 rows lifted in
phase 2a was re-measured against `rfc/full/rfc7296.txt`. The text was flattened first,
because the RFC wraps its lines. 28 rows quote the source exactly. 19 carry a verbatim run
of half the row or more. No row states an obligation the source does not carry.

Two rows put a clarifying noun inside the quoted span, so both are corrected above. §3.14
reads "recipients MUST accept any value", not "any IV value". It reads "MUST accept any
length that results in proper alignment", not "any Pad Length". Each row now carries the
field name as a leading label. The normative words are verbatim. The form follows
`RFC1035-2.3.4-1`.

The other 19 partial rows are condensations, not paraphrases. Some split one compound RFC
sentence into two testable rows. `RFC7296-3.1-7` and `RFC7296-3.1-8` divide one sentence
about the X bits. A reader who diffs a partial row against the RFC will find fewer words,
never different ones.

**Triage corrected 2026-07-30 (phase 2a).** Nine rows carry the class `impl-untested`, but
a plain Go unit test proves each one. The nine are `RFC7296-2.10-3`, `2.13-2`, `2.13-4`,
`2.15-1`, `2.15-2`, `3.12-3`, `3.14-3`, `3.14-4` and `3.14-5`. Their tests use
`establishPSK` and `parseMsg` from `internal/component/ike/engine/responder_test.go`, plus
`decryptAndParse` from `internal/component/ike/engine/inbound.go`. All three already
existed. No new infrastructure was built for them.

The class column stays as the walk wrote it, because it is the walk's record. Read these
nine as `impl-testable`. The count of rows that genuinely need new infrastructure is
therefore 16, not 25.

Phase 2b must not accept `impl-untested` at face value. The walk assigned that class and
did not search for a seam. Each remaining row gets a search for an existing harness first.

**Coverage gap found in phase 2a.** Six rows depend on logic that exists at two sites.
`Message.ReadFrom` parses the outer chain (`internal/component/ike/wire/message.go:117-127`).
`ParsePayloadChain` parses the decrypted inner chain
(`internal/component/ike/wire/chain.go:33-41`). Both reject an unrecognized critical payload
and both demote a non-critical one. The first batch of tagged tests drove only the outer
parser.

The six are `RFC7296-2.5-8`, `2.5-9`, `2.5-11`, `2.5-13`, `3.2-2` and `3.2-3`. After
IKE_SA_INIT almost every IKEv2 payload arrives inside an Encrypted payload, so the inner
parser carries most of the traffic. A row proven on the outer path alone is a claim with half
its surface untested. `ai/rules/integration-completeness.md` names this shape: one
implementation found is not proof there is only one.

Phase 2a adds `internal/component/ike/wire/rfc7296_innerchain_test.go`. It drives
`ParsePayloadChain` directly and carries its own tagged pair for `RFC7296-2.5-8`, `2.5-9`,
`2.5-11` and `3.2-2`. The mutation proved the gap was real. It removed the whole reject at
`chain.go:38-40`. That turned the two new critical tests red and left every outer-path test
green.

`RFC7296-2.5-13` and `3.2-3` keep one proof each. Payload order and the set of understood
types are properties of `decodePayload`, which both parsers call (`message.go:117`,
`chain.go:33`). Neither parser holds an order table, so both accept any order for the same
structural reason.

**Mutation verification (phase 2a).** Ten requirements were verified across four packages.
Each check broke the producing code and confirmed the tagged test went red. The harness
restored every mutation and re-ran the test to prove it went green again. All ten gate. No
test survived its mutation.

| Requirement | Producer broken | Verdict |
|-------------|-----------------|---------|
| `RFC7296-3.9-1`, `2.10-2` | `payload_nonce.go` `NonceMinLen` 16 to 1 | gates |
| `RFC7296-3.9-1` | `payload_nonce.go` `NonceMaxLen` 256 to 4096 | gates |
| `RFC7296-2.5-9` | `message.go` critical reject deleted | gates |
| `RFC7296-2.5-9` | `chain.go` critical reject deleted | gates |
| `RFC7296-3.11-1` | `payload_delete.go` Protocol ID forced to 0 | gates |
| `RFC7296-5-2` | `transform.go` AUTH_NONE registered as "none" | gates |
| `RFC7296-3.1-7` | `header.go` flags octet given a stray X bit | gates |
| `RFC7296-3.1-5` | `header.go` major version forced to 3 | gates |
| `RFC7296-2.5-6` | `payload_ke.go` reserved field set to 0xffff | gates |
| `RFC7296-2.5-7` | `payload.go` critical bit read from the whole octet | gates |

The two `RFC7296-2.5-9` lines are one requirement at its two producers. Breaking one
producer leaves the other producer's test green. That is why the row needs both proofs.

**The two parsers disagree about truncation.** `Message.ReadFrom` returns `ErrTruncated` when
a payload runs past the buffer (`message.go:113-114`). `ParsePayloadChain` breaks out of the
loop and returns the payloads it already holds (`chain.go:29-31`). A truncated inner chain
therefore reports a short payload list rather than an error. A required payload then looks
absent instead of malformed. This is one concrete case of `RFC7296-2.21.2-1`, which is
already `NOT IMPL` and owned by a phase 4-15 work package.

**The `Class` column is the walk's triage, mechanically re-derived while authoring this
spec** from the walk's three explicit id lists with `NOT IMPL` as the complement. The four
classes are disjoint and exhaustive: 63 `impl-testable` + 25 `impl-untested` +
18 `uncertain` + 108 `NOT IMPL` = 214 (assumption A-2, confirmed).

| Class | Rows | Phase that owns it |
|-------|------|--------------------|
| `impl-testable` | 63 | Phase 2 -- behaviour exists and a unit-test seam exists; tagged pair only, no production change |
| `impl-untested` | 25 | Phase 2 -- behaviour exists but proving it needs infrastructure that does not (a second daemon, a driven clock, a config-surface test) |
| `uncertain` | 18 | Phase 3 -- the producing path was not read; resolved by reading, then joins one of the other classes |
| **`NOT IMPL`** | **108** | Phases 4-15 -- the 12 work packages, verified to partition this set exactly |

To lift the rows into `rfc/short/rfc7296.md`, each becomes
`- [ ] [<ID>] [<Level>] <Obligation> (§<section>)`; the section is already carried inside
the Obligation text.

| ID | Level | Obligation | Class |
|----|-------|------------|-------|
| `RFC7296-1-1` | MUST | In all cases, all IKE_SA_INIT exchanges MUST complete before any other exchange type, then all IKE_AUTH exchanges MUST complete, and following that, any number of CREATE_CHILD_SA and INFORMATIONAL exchanges may occur in any order (§1) | impl-testable |
| `RFC7296-1.2-2` | MUST | If any CERT payloads are included, the first certificate provided MUST contain the public key used to verify the AUTH field (§1.2, §3.6) | impl-testable |
| `RFC7296-1.2-3` | MUST | Both parties in the IKE_AUTH exchange MUST verify that all signatures and Message Authentication Codes (MACs) are computed correctly (§1.2) | impl-testable |
| `RFC7296-1.2-4` | MUST | If either side uses a shared secret for authentication, the names in the ID payload MUST correspond to the key used to generate the AUTH payload (§1.2) | impl-testable |
| `RFC7296-1.2-5` | MUST | If the initiator guesses wrong, the responder will respond with a Notify payload of type INVALID_KE_PAYLOAD indicating the selected group. In this case, the initiator MUST retry the IKE_SA_INIT with the corrected Diffie-Hellman group (§1.2) | **NOT IMPL** |
| `RFC7296-1.2-6` | MUST | The initiator MUST again propose its full set of acceptable cryptographic suites because the rejection message was not authenticated (§1.2) | **NOT IMPL** |
| `RFC7296-1.3-1` | MUST | If a CREATE_CHILD_SA exchange includes a KEi payload, at least one of the SA offers MUST include the Diffie-Hellman group of the KEi (§1.3) | impl-untested |
| `RFC7296-1.3-2` | MUST | If the responder selects a proposal using a different Diffie-Hellman group (other than NONE), the responder MUST reject the request and indicate its preferred Diffie-Hellman group in the INVALID_KE_PAYLOAD Notify payload (§1.3, §3.4) | impl-untested |
| `RFC7296-1.3.1-1` | MUST | If the request is accepted, the response MUST also include a notification of type USE_TRANSPORT_MODE (§1.3.1) | **NOT IMPL** |
| `RFC7296-1.3.1-2` | MUST | If the responder declines the request, the Child SA will be established in tunnel mode. If this is unacceptable to the initiator, the initiator MUST delete the SA (§1.3.1) | **NOT IMPL** |
| `RFC7296-1.4-2` | MUST | INFORMATIONAL exchanges MUST ONLY occur after the initial exchanges and are cryptographically protected with the negotiated keys (§1.4) | **NOT IMPL** |
| `RFC7296-1.4-3` | MUST | Control messages that pertain to an IKE SA MUST be sent under that IKE SA. Control messages that pertain to Child SAs MUST be sent under the protection of the IKE SA that generated them (§1.4) | impl-untested |
| `RFC7296-1.4-4` | MUST | The recipient of an INFORMATIONAL exchange request MUST send some response; otherwise, the sender will assume the message was lost in the network and will retransmit it (§1.4, §4) | impl-testable |
| `RFC7296-1.4.1-1` | MUST | When an SA is closed, both members of the pair MUST be closed (that is, deleted). Each endpoint MUST close its incoming SAs and allow the other endpoint to close the other SA in each pair (§1.4.1) | impl-untested |
| `RFC7296-1.4.1-2` | MUST | The recipient MUST close the designated SAs (§1.4.1) | **NOT IMPL** |
| `RFC7296-1.4.1-3` | MUST | If a node receives a delete request for SAs for which it has already issued a delete request, it MUST delete the outgoing SAs while processing the request and the incoming SAs while processing the response (§1.4.1) | impl-untested |
| `RFC7296-1.4.1-4` | MUST NOT | The responses MUST NOT include Delete payloads for the deleted SAs, since that would result in duplicate deletion and could in theory delete the wrong SA (§1.4.1) | impl-untested |
| `RFC7296-1.4.1-5` | MUST NOT | A node MAY refuse to accept incoming data on half-closed connections but MUST NOT unilaterally close them and reuse the SPIs (§1.4.1) | impl-untested |
| `RFC7296-1.5-1` | MUST NOT | This message is not part of an INFORMATIONAL exchange, and the receiving node MUST NOT respond to it because doing so could cause a message loop (§1.5) | **NOT IMPL** |
| `RFC7296-1.7-1` | MUST | Implementations that conform to this document MUST ignore proposals that have configuration attribute type 5, the old value for INTERNAL_ADDRESS_EXPIRY (§1.7) | **NOT IMPL** |
| `RFC7296-1.7-2` | MUST | All pseudorandom functions (PRFs) used with IKEv2 MUST take variable-sized keys (§1.7, §2.13) | impl-testable |
| `RFC7296-2-1` | MUST | All IKEv2 implementations MUST be able to send, receive, and process IKE messages that are up to 1280 octets long (§2) | impl-testable |
| `RFC7296-2.1-3` | MUST | The responder MUST never retransmit a response unless it receives a retransmission of the request (§2.1) | impl-testable |
| `RFC7296-2.1-4` | MUST | In that event, the responder MUST ignore the retransmitted request except insofar as it causes a retransmission of the response (§2.1) | impl-testable |
| `RFC7296-2.1-5` | MUST | The initiator MUST remember each request until it receives the corresponding response. The responder MUST remember each response until it receives a request whose sequence number is larger than or equal to the sequence number in the response plus its window size (§2.1, §2.3) | impl-testable |
| `RFC7296-2.1-6` | MUST | If the responder receives a retransmitted request for which it has already forgotten the response, it MUST ignore the request (and not, for example, attempt constructing a new response) (§2.1) | impl-testable |
| `RFC7296-2.1-7` | MUST | IKE is a reliable protocol: the initiator MUST retransmit a request until it either receives a corresponding response or deems the IKE SA to have failed (§2.1) | impl-testable |
| `RFC7296-2.1-8` | MUST | A retransmission from the initiator MUST be bitwise identical to the original request (§2.1) | impl-testable |
| `RFC7296-2.2-1` | MUST | Retransmission of a message MUST use the same Message ID as the original message (§2.2) | impl-testable |
| `RFC7296-2.2-2` | MUST | In the unlikely event that Message IDs grow too large to fit in 32 bits, the IKE SA MUST be closed or rekeyed (§2.2) | **NOT IMPL** |
| `RFC7296-2.3-1` | MUST | The data associated with a SET_WINDOW_SIZE notification MUST be 4 octets long and contain the big endian representation of the number of messages the sender promises to keep (§2.3) | **NOT IMPL** |
| `RFC7296-2.3-2` | MUST | An IKE endpoint MUST wait for a response to each of its messages before sending a subsequent message unless it has received a SET_WINDOW_SIZE Notify message from its peer (§2.3) | impl-testable |
| `RFC7296-2.3-3` | MUST | An IKE endpoint MUST be prepared to accept and process a request while it has a request outstanding in order to avoid a deadlock in this situation (§2.3) | **NOT IMPL** |
| `RFC7296-2.3-4` | MUST NOT | An IKE endpoint MUST NOT exceed the peer's stated window size for transmitted IKE requests (§2.3) | impl-testable |
| `RFC7296-2.3-5` | MUST NOT | This Notify message MUST NOT be sent in a response; the invalid request MUST NOT be acknowledged (§2.3) | **NOT IMPL** |
| `RFC7296-2.3-6` | MUST | Sending this notification is OPTIONAL, and notifications of this type MUST be rate limited (§2.3) | **NOT IMPL** |
| `RFC7296-2.4-5` | MUST NOT | This notification MUST NOT be sent by an entity that may be replicated (§2.4) | **NOT IMPL** |
| `RFC7296-2.4-6` | MUST | An endpoint MUST conclude that the other endpoint has failed only when repeated attempts to contact it have gone unanswered for a timeout period or when a cryptographically protected INITIAL_CONTACT notification is received on a different IKE SA to the same authenticated identity (§2.4) | impl-untested |
| `RFC7296-2.4-7` | MUST | Implementations MUST limit the rate at which they take actions based on unprotected messages (§2.4) | impl-untested |
| `RFC7296-2.4-8` | MUST | To be a good network citizen, retransmission times MUST increase exponentially to avoid flooding the network and making an existing congestion situation worse (§2.4) | impl-testable |
| `RFC7296-2.4-9` | MUST | If a system creates Child SAs that can fail independently from one another without the associated IKE SA being able to send a delete message, then the system MUST negotiate such Child SAs using separate IKE SAs (§2.4) | impl-untested |
| `RFC7296-2.4-10` | MUST | If an IKE endpoint chooses to delete Child SAs, it MUST send Delete payloads to the other end notifying it of the deletion (§2.4) | impl-testable |
| `RFC7296-2.5-1` | MUST | The minor version number indicates new capabilities, and MUST be ignored by a node with a smaller minor version number, but used for informational purposes by the node with the larger minor version number (§2.5, §3.1) | impl-testable |
| `RFC7296-2.5-2` | MUST | If an endpoint receives a message with a higher major version number, it MUST drop the message (§2.5, §3.1) | impl-testable |
| `RFC7296-2.5-3` | MUST | If an endpoint supports major version n, and major version m, it MUST support all versions between n and m (§2.5) | uncertain |
| `RFC7296-2.5-4` | MUST | If it receives a message with a major version that it supports, it MUST respond with that version number (§2.5) | uncertain |
| `RFC7296-2.5-5` | MUST | If they mistakenly negotiate to version n, then both will notice that the other side can support a higher version number, and they MUST break the connection and reconnect using version n+1 (§2.5) | **NOT IMPL** |
| `RFC7296-2.5-6` | MUST | Also, for forward compatibility, all fields marked RESERVED MUST be set to zero by an implementation running version 2.0 (§2.5, §3.2, §3.3.1, §3.3.2, §3.5, §3.8, §3.13, §3.15, §3.15.1) | impl-testable |
| `RFC7296-2.5-7` | MUST | The content of all fields marked RESERVED MUST be ignored by an implementation running version 2.0 (§2.5, §3.2, §3.3.1, §3.3.2, §3.5, §3.8, §3.13, §3.15, §3.15.1) | impl-testable |
| `RFC7296-2.5-8` | MUST | Payload types that are not defined are reserved for future use; implementations of a version where they are undefined MUST skip over those payloads and ignore their contents (§2.5, §4) | impl-testable |
| `RFC7296-2.5-9` | MUST | If the critical flag is set and the payload type is unrecognized, the message MUST be rejected (§2.5, §4) | impl-testable |
| `RFC7296-2.5-10` | MUST | The response to the IKE request containing that payload MUST include a Notify payload UNSUPPORTED_CRITICAL_PAYLOAD, indicating an unsupported critical payload was included (§2.5) | **NOT IMPL** |
| `RFC7296-2.5-11` | MUST | If the critical flag is not set and the payload type is unsupported, that payload MUST be ignored (§2.5) | impl-testable |
| `RFC7296-2.5-12` | MUST NOT | Payloads sent in IKE response messages MUST NOT have the critical flag set (§2.5) | **NOT IMPL** |
| `RFC7296-2.5-13` | MUST NOT | Implementations MUST NOT reject as invalid a message with those payloads in any other order (§2.5, §1.7) | impl-testable |
| `RFC7296-2.6-2` | MUST | Each endpoint chooses one of the two SPIs and MUST choose them so as to be unique identifiers of an IKE SA (§2.6) | impl-testable |
| `RFC7296-2.6-3` | MUST | The data associated with this notification MUST be between 1 and 64 octets in length (inclusive) (§2.6) | **NOT IMPL** |
| `RFC7296-2.6-4` | MUST | If the IKE_SA_INIT response includes the COOKIE notification, the initiator MUST then retry the IKE_SA_INIT request, and include the COOKIE notification containing the received data as the first payload, and all other payloads unchanged (§2.6) | **NOT IMPL** |
| `RFC7296-2.6-5` | MUST | When one party receives an IKE_SA_INIT request containing a cookie whose contents do not match the value expected, that party MUST ignore the cookie and process the message as if no cookie had been included (§2.6) | **NOT IMPL** |
| `RFC7296-2.6.1-1` | MUST NOT | Implementations SHOULD support this shorter exchange, but MUST NOT fail if other implementations do not support this shorter exchange (§2.6.1) | **NOT IMPL** |
| `RFC7296-2.7-2` | MUST | Each proposal contains one protocol. If a proposal is accepted, the SA response MUST contain the same protocol (§2.7) | impl-testable |
| `RFC7296-2.8-4` | MUST NOT | When the lifetime of a Security Association expires, the Security Association MUST NOT be used (§2.8) | **NOT IMPL** |
| `RFC7296-2.8-5` | MUST | If an SA has expired or is about to expire and rekeying attempts using the mechanisms described here fail, an implementation MUST close the IKE SA and any associated Child SAs and then MAY start new ones (§2.8) | impl-testable |
| `RFC7296-2.8-6` | MUST | After the new equivalent IKE SA is created, the initiator deletes the old IKE SA, and the Delete payload to delete itself MUST be the last request sent over the old IKE SA (§2.8) | impl-testable |
| `RFC7296-2.8-7` | MUST | The responder to a CREATE_CHILD_SA MUST be prepared to accept messages on an SA before sending its response to the creation request, so there is no ambiguity for the initiator (§2.8) | impl-untested |
| `RFC7296-2.8.1-1` | MUST | When there are two SAs eligible to receive packets, a node MUST accept incoming packets through either SA (§2.8.1) | impl-testable |
| `RFC7296-2.8.2-1` | MUST | The new IKE SA containing the lowest nonce SHOULD be deleted by the node that created it, and the other surviving new IKE SA MUST inherit all the Child SAs (§2.8.2) | impl-untested |
| `RFC7296-2.9-2` | MUST | If the responder's policy allows it to accept the first selector of TSi and TSr, then the responder MUST narrow the Traffic Selectors to a subset that includes the initiator's first choices (§2.9) | **NOT IMPL** |
| `RFC7296-2.9.2-1` | MUST NOT | Thus, the new SA MUST NOT have narrower selectors than the original (§2.9.2) | **NOT IMPL** |
| `RFC7296-2.9.2-2` | MUST NOT | The responder MUST NOT narrow down the Traffic Selectors narrower than the scope currently in use (§2.9.2) | **NOT IMPL** |
| `RFC7296-2.10-1` | MUST | Nonces used in IKEv2 MUST be randomly chosen (§2.10) | impl-testable |
| `RFC7296-2.10-2` | MUST | Nonces used in IKEv2 MUST be at least 128 bits in size (§2.10) | impl-testable |
| `RFC7296-2.10-3` | MUST | Nonces used in IKEv2 MUST be at least half the key size of the negotiated pseudorandom function (PRF) (§2.10) | impl-untested |
| `RFC7296-2.11-1` | MUST | An implementation MUST accept incoming requests even if the source port is not 500 or 4500 (§2.11, §2.23) | impl-testable |
| `RFC7296-2.11-2` | MUST | An implementation MUST respond to the address and port from which the request was received (§2.11, §2.23) | **NOT IMPL** |
| `RFC7296-2.11-3` | MUST | It MUST specify the address and port at which the request was received as the source address and port in the response (§2.11) | **NOT IMPL** |
| `RFC7296-2.12-1` | MUST | Achieving perfect forward secrecy requires that when a connection is closed, each endpoint MUST forget not only the keys used by the connection but also any information that could be used to recompute those keys (§2.12) | uncertain |
| `RFC7296-2.13-1` | MUST | For algorithms that accept a variable-length key, a fixed key size MUST be specified as part of the cryptographic transform negotiated (§2.13) | impl-testable |
| `RFC7296-2.13-2` | MUST | For algorithms for which not all values are valid keys, the algorithm by which keys are derived from arbitrary values MUST be specified by the cryptographic transform (§2.13) | impl-untested |
| `RFC7296-2.13-3` | MUST | The preferred key size MUST be used as the length of SK_d, SK_pi, and SK_pr (§2.13, §2.14) | impl-testable |
| `RFC7296-2.13-4` | MUST | Other types of PRFs MUST specify their preferred key size (§2.13) | impl-untested |
| `RFC7296-2.15-1` | MUST | The management interface by which the shared secret is provided MUST accept ASCII strings of at least 64 octets (§2.15) | impl-untested |
| `RFC7296-2.15-2` | MUST NOT | The management interface MUST NOT add a null terminator before using them as shared secrets (§2.15) | impl-untested |
| `RFC7296-2.15-3` | MUST | It MUST also accept a hex encoding of the shared secret (§2.15) | **NOT IMPL** |
| `RFC7296-2.16-1` | MUST | These protocols are typically used to authenticate the initiator to the responder and MUST be used in conjunction with a public-key-signature-based authentication of the responder to the initiator (§2.16, §5) | impl-untested |
| `RFC7296-2.16-2` | MUST | Extensible authentication is implemented in IKE as additional IKE_AUTH exchanges that MUST be completed in order to initialize the IKE SA (§2.16) | impl-untested |
| `RFC7296-2.16-3` | MUST | For EAP methods that create a shared key as a side effect of authentication, that shared key MUST be used by both the initiator and responder to generate AUTH payloads in messages 7 and 8 using the syntax for shared secrets specified in Section 2.15 (§2.16) | uncertain |
| `RFC7296-2.16-4` | MUST NOT | This shared key generated during an IKE exchange MUST NOT be used for any other purpose (§2.16) | impl-untested |
| `RFC7296-2.16-5` | MUST | If EAP methods that do not generate a shared key are used, the AUTH payloads in messages 7 and 8 MUST be generated using SK_pi and SK_pr, respectively (§2.16) | uncertain |
| `RFC7296-2.16-6` | MUST | Once the protocol exchange defined by the chosen EAP authentication method has successfully terminated, the responder MUST send an EAP payload containing the Success message (§2.16) | uncertain |
| `RFC7296-2.16-7` | MUST | Similarly, if the authentication method has failed, the responder MUST send an EAP payload containing the Failure message (§2.16) | uncertain |
| `RFC7296-2.16-8` | MUST | Following such an extended exchange, the EAP AUTH payloads MUST be included in the two messages following the one containing the EAP Success message (§2.16) | uncertain |
| `RFC7296-2.17-1` | MUST | Keying material for each Child SA MUST be taken from the expanded KEYMAT using the following rules: all keys for SAs carrying data from the initiator to the responder are taken before SAs going from the responder to the initiator (§2.17) | impl-testable |
| `RFC7296-2.17-2` | MUST | For ESP and AH, the encryption key (if any) MUST be taken from the first bits and the integrity key (if any) MUST be taken from the remaining bits (§2.17) | impl-testable |
| `RFC7296-2.18-1` | MUST NOT | An initiator MUST NOT propose the value NONE for the Diffie-Hellman transform, and a responder MUST NOT accept such a proposal (§2.18) | **NOT IMPL** |
| `RFC7296-2.18-2` | MUST | The new IKE SA MUST reset its message counters to 0 (§2.18) | impl-testable |
| `RFC7296-2.19-1` | MUST | Since the IKE_AUTH exchange creates an IKE SA and a Child SA, the IRAC MUST request the IRAS-controlled address in the IKE_AUTH exchange (§2.19, §4) | **NOT IMPL** |
| `RFC7296-2.19-2` | MUST | In all cases, the CP payload MUST be inserted before the SA payload (§2.19) | **NOT IMPL** |
| `RFC7296-2.19-3` | MUST | In variations of the protocol where there are multiple IKE_AUTH exchanges, the CP payloads MUST be inserted in the messages containing the SA payloads (§2.19) | **NOT IMPL** |
| `RFC7296-2.19-4` | MUST | CP(CFG_REQUEST) MUST contain at least an INTERNAL_ADDRESS attribute, either IPv4 or IPv6 (§2.19) | **NOT IMPL** |
| `RFC7296-2.19-5` | MUST NOT | The responder MUST NOT send a CFG_REPLY without having first received a CP(CFG_REQUEST) from the initiator (§2.19) | **NOT IMPL** |
| `RFC7296-2.19-6` | MUST | In the case where the IRAS's configuration requires that CP be used for a given identity IDi, but IRAC has failed to send a CP(CFG_REQUEST), IRAS MUST fail the request, and terminate the Child SA creation with a FAILED_CP_REQUIRED error (§2.19) | **NOT IMPL** |
| `RFC7296-2.20-1` | MUST | In that case, it MUST either return an empty string or no CP payload if CP is not supported (§2.20) | **NOT IMPL** |
| `RFC7296-2.21.2-1` | MUST | Request messages that contain an unsupported critical payload, or where the whole message is malformed, MUST only lead to an UNSUPPORTED_CRITICAL_PAYLOAD or INVALID_SYNTAX Notification sent as a response (§2.21.2) | **NOT IMPL** |
| `RFC7296-2.21.2-2` | MUST NOT | The responder may reply with error notifications for the piggybacked exchanges, and the initiator MUST NOT fail the authentication because of this (§2.21.2) | **NOT IMPL** |
| `RFC7296-2.21.2-3` | MUST NOT | Extension documents may define new error notifications with these semantics, but MUST NOT use them unless the peer has been shown to understand them (§2.21.2) | **NOT IMPL** |
| `RFC7296-2.21.3-1` | MUST | After the IKE SA is authenticated, all requests having errors MUST result in a response notifying the other end of the error (§2.21.3) | **NOT IMPL** |
| `RFC7296-2.21.4-1` | MUST NOT | If the message is marked as a response, the node can audit the suspicious event but MUST NOT respond (§2.21.4) | **NOT IMPL** |
| `RFC7296-2.21.4-2` | MUST | If a response is sent, the response MUST be sent to the IP address and port from where it came with the same IKE SPIs and the Message ID copied (§2.21.4) | **NOT IMPL** |
| `RFC7296-2.21.4-3` | MUST NOT | The response MUST NOT be cryptographically protected (§2.21.4) | **NOT IMPL** |
| `RFC7296-2.21.4-4` | MUST | The response MUST contain an INVALID_IKE_SPI Notify payload (§2.21.4) | **NOT IMPL** |
| `RFC7296-2.21.4-5` | MUST NOT | A peer receiving such an unprotected Notify payload MUST NOT respond (§2.21.4) | **NOT IMPL** |
| `RFC7296-2.21.4-6` | MUST NOT | A peer receiving such an unprotected Notify payload MUST NOT change the state of any existing SAs (§2.21.4) | **NOT IMPL** |
| `RFC7296-2.21.4-7` | MUST NOT | The recipient MUST NOT change the state of any SAs as a result, but may wish to audit the event to aid in diagnosing malfunctions (§2.21.4) | **NOT IMPL** |
| `RFC7296-2.22-1` | MUST NOT | These payloads MUST NOT occur in messages that do not contain SA payloads (§2.22) | **NOT IMPL** |
| `RFC7296-2.22-2` | MUST NOT | Implementations of this specification MUST NOT accept an IPComp algorithm that was not proposed (§2.22) | **NOT IMPL** |
| `RFC7296-2.22-3` | MUST NOT | Implementations of this specification MUST NOT accept more than one IPComp algorithm (§2.22) | **NOT IMPL** |
| `RFC7296-2.22-4` | MUST NOT | Implementations of this specification MUST NOT compress using an algorithm other than one proposed and accepted in the setup of the Child SA (§2.22) | **NOT IMPL** |
| `RFC7296-2.23-4` | MUST | An IPsec endpoint that discovers a NAT between it and its correspondent MUST send all subsequent traffic from port 4500 (§2.23) | **NOT IMPL** |
| `RFC7296-2.23-5` | MUST NOT | UDP encapsulation MUST NOT be done on port 500 (§2.23) | **NOT IMPL** |
| `RFC7296-2.23-6` | MUST | If Network Address Translation Traversal (NAT-T) is supported, all devices MUST be able to receive and process both UDP-encapsulated ESP and non-UDP-encapsulated ESP packets at any time (§2.23) | **NOT IMPL** |
| `RFC7296-2.23-7` | MUST | Both the IKE initiator and responder MUST include in their IKE_SA_INIT packets Notify payloads of type NAT_DETECTION_SOURCE_IP and NAT_DETECTION_DESTINATION_IP (§2.23) | impl-untested |
| `RFC7296-2.23-8` | MUST | Implementations MUST process received UDP-encapsulated ESP packets even when no NAT was detected (§2.23) | **NOT IMPL** |
| `RFC7296-2.23.1-1` | MUST | For transport mode, it MUST use exactly one IP address in the TSi and TSr payloads (§2.23.1, §2.23) | **NOT IMPL** |
| `RFC7296-2.23.1-2` | MUST | The TSi entries MUST have exactly one IP address, and that MUST match the source address of the IKE SA (§2.23.1) | **NOT IMPL** |
| `RFC7296-2.23.1-3` | MUST | The TSr entries MUST have exactly one IP address, and that MUST match the destination address of the IKE SA (§2.23.1) | **NOT IMPL** |
| `RFC7296-2.24-1` | MUST | Tunnel encapsulators and decapsulators for all tunnel mode SAs created by IKEv2 MUST support the ECN full-functionality option for tunnels (§2.24) | uncertain |
| `RFC7296-2.24-2` | MUST | Tunnel encapsulators and decapsulators MUST implement the tunnel encapsulation and decapsulation processing specified in IPSECARCH to prevent discarding of ECN congestion indications (§2.24) | uncertain |
| `RFC7296-2.25-1` | MUST NOT | When a peer receives a TEMPORARY_FAILURE notification, it MUST NOT immediately retry the operation; it MUST wait so that the sender may complete whatever operation caused the temporary condition (§2.25) | **NOT IMPL** |
| `RFC7296-3.1-1` | MUST | An Encrypted payload MUST be the last payload in a packet (§3.1, §3.14) | impl-testable |
| `RFC7296-3.1-2` | MUST NOT | An Encrypted payload MUST NOT contain another Encrypted payload (§3.1) | impl-testable |
| `RFC7296-3.1-3` | MUST NOT | Initiator's SPI is a value chosen by the initiator to identify a unique IKE Security Association. This value MUST NOT be zero (§3.1) | impl-testable |
| `RFC7296-3.1-4` | MUST | Responder's SPI is a value chosen by the responder to identify a unique IKE Security Association. This value MUST be zero in the first message of an IKE initial exchange (§3.1) | impl-testable |
| `RFC7296-3.1-5` | MUST | Implementations based on this version of IKE MUST set the major version to 2 (§3.1) | impl-testable |
| `RFC7296-3.1-6` | MUST | Implementations based on this version of IKE MUST set the minor version to 0 (§3.1) | impl-testable |
| `RFC7296-3.1-7` | MUST | X bits MUST be cleared when sending (§3.1) | impl-testable |
| `RFC7296-3.1-8` | MUST | X bits MUST be ignored on receipt (§3.1) | impl-testable |
| `RFC7296-3.1-9` | MUST | The R bit MUST be cleared in all request messages and MUST be set in all responses (§3.1) | impl-testable |
| `RFC7296-3.1-10` | MUST NOT | An IKE endpoint MUST NOT generate a response to a message that is marked as being a response (§3.1) | **NOT IMPL** |
| `RFC7296-3.1-11` | MUST | Implementations of IKEv2 MUST clear the V bit when sending and MUST ignore it in incoming messages (§3.1) | impl-testable |
| `RFC7296-3.1-12` | MUST | The I bit MUST be set in messages sent by the original initiator of the IKE SA and MUST be cleared in messages sent by the original responder (§3.1) | **NOT IMPL** |
| `RFC7296-3.2-1` | MUST | The Critical bit MUST be set to zero for payload types defined in this document (§3.2) | **NOT IMPL** |
| `RFC7296-3.2-2` | MUST | The Critical bit MUST be ignored by the recipient if the recipient understands the payload type code in the Next Payload field of the previous payload (§3.2) | impl-testable |
| `RFC7296-3.2-3` | MUST | All implementations MUST understand all payload types defined in this document (§3.2, §4) | impl-testable |
| `RFC7296-3.3-3` | MUST | An SA payload MAY contain multiple proposals. If there is more than one, they MUST be ordered from most preferred to least preferred (§3.3) | impl-testable |
| `RFC7296-3.3-4` | MUST | When parsing an SA, an implementation MUST check that the total Payload Length is consistent with the payload's internal lengths and counts (§3.3) | impl-testable |
| `RFC7296-3.3-5` | MUST | Each structure MUST have a proposal number one (1) greater than the previous structure. The first Proposal in the initiator's SA payload MUST have a Proposal Num of one (1) (§3.3, §3.3.1) | **NOT IMPL** |
| `RFC7296-3.3-6` | MUST NOT | A transform MUST NOT have multiple attributes of the same type (§3.3) | **NOT IMPL** |
| `RFC7296-3.3-7` | MUST | To propose alternate values for an attribute, an implementation MUST include multiple transforms with the same Transform Type each with a single Attribute (§3.3) | **NOT IMPL** |
| `RFC7296-3.3.1-1` | MUST | When a proposal is accepted, the proposal number in the SA payload MUST match the number on the proposal sent that was accepted (§3.3.1) | **NOT IMPL** |
| `RFC7296-3.3.1-2` | MUST | For an initial IKE SA negotiation, the SPI Size field MUST be zero; the SPI is obtained from the outer header (§3.3.1) | **NOT IMPL** |
| `RFC7296-3.3.3-1` | MUST | A compliant implementation MUST understand all mandatory and optional Transform Types for each protocol it supports (§3.3.3) | **NOT IMPL** |
| `RFC7296-3.3.4-1` | MUST | All implementations of IKEv2 MUST include a management facility that enables a user or system administrator to specify the suites that are acceptable for use with IKE (§3.3.4) | **NOT IMPL** |
| `RFC7296-3.3.4-2` | MUST | Upon receipt of a payload with a set of Transform IDs, the implementation MUST compare the transmitted Transform IDs against those locally configured via the management controls, to verify that the proposed suite is acceptable based on local policy (§3.3.4) | **NOT IMPL** |
| `RFC7296-3.3.4-3` | MUST | The implementation MUST reject SA proposals that are not authorized by these IKE suite controls (§3.3.4) | **NOT IMPL** |
| `RFC7296-3.3.5-1` | MUST NOT | Attributes described as fixed length MUST NOT be encoded using the variable-length encoding unless that length exceeds two bytes (§3.3.5) | **NOT IMPL** |
| `RFC7296-3.3.5-2` | MUST NOT | Variable-length attributes MUST NOT be encoded as fixed-length even if their value can fit into two octets (§3.3.5) | **NOT IMPL** |
| `RFC7296-3.3.5-3` | MUST | The Key Length attribute specifies the key length in bits and MUST use network byte order (§3.3.5) | **NOT IMPL** |
| `RFC7296-3.3.5-4` | MUST NOT | The Key Length attribute MUST NOT be used with transforms that use a fixed-length key (§3.3.5) | **NOT IMPL** |
| `RFC7296-3.3.5-5` | MUST | Some transforms specify that the Key Length attribute MUST be always included, and proposals not containing it MUST be rejected (§3.3.5) | **NOT IMPL** |
| `RFC7296-3.3.6-2` | MUST | Any attributes of a selected transform MUST be returned unmodified (§3.3.6) | **NOT IMPL** |
| `RFC7296-3.3.6-3` | MUST | The initiator of an exchange MUST check that the accepted offer is consistent with one of its proposals, and if not MUST terminate the exchange (§3.3.6) | impl-testable |
| `RFC7296-3.3.6-4` | MUST | If the responder receives a proposal that contains a Transform Type it does not understand, or a proposal that is missing a mandatory Transform Type, it MUST consider this proposal unacceptable (§3.3.6) | **NOT IMPL** |
| `RFC7296-3.3.6-5` | MUST | If the responder receives a transform that it does not understand, or one that contains a Transform Attribute it does not understand, it MUST consider this transform unacceptable (§3.3.6) | **NOT IMPL** |
| `RFC7296-3.3.6-6` | MUST | If one of the proposals offered is for the Diffie-Hellman group of NONE, and the responder selects that Diffie-Hellman group, then it MUST ignore the initiator's KE payload and omit the KE payload from the response (§3.3.6) | **NOT IMPL** |
| `RFC7296-3.4-1` | MUST | The length of the Diffie-Hellman public value for MODP groups MUST be equal to the length of the prime modulus over which the exponentiation was performed, prepending zero bits to the value if necessary (§3.4) | uncertain |
| `RFC7296-3.4-2` | MUST | This Diffie-Hellman Group Num MUST match a Diffie-Hellman group specified in a proposal in the SA payload that is sent in the same message (§3.4) | **NOT IMPL** |
| `RFC7296-3.4-3` | MUST NOT | If none of the proposals in that SA payload specifies a Diffie-Hellman group, the KE payload MUST NOT be present (§3.4) | **NOT IMPL** |
| `RFC7296-3.5-1` | MUST NOT | The ID_FQDN and ID_RFC822_ADDR strings MUST NOT contain any terminators (e.g., NULL, CR, etc.) (§3.5) | uncertain |
| `RFC7296-3.5-2` | MUST | Implementations MUST be configurable to send at least one of ID_IPV4_ADDR, ID_FQDN, ID_RFC822_ADDR, or ID_KEY_ID (§3.5) | **NOT IMPL** |
| `RFC7296-3.5-3` | MUST | Implementations MUST be configurable to accept all of these four types (§3.5) | **NOT IMPL** |
| `RFC7296-3.5-4` | MUST | IPv6-capable implementations MUST additionally be configurable to accept ID_IPV6_ADDR (§3.5) | **NOT IMPL** |
| `RFC7296-3.6-1` | MUST | Implementations MUST be capable of being configured to send and accept up to four X.509 certificates in support of authentication (§3.6) | **NOT IMPL** |
| `RFC7296-3.6-2` | MUST | Implementations MUST be capable of being configured to send and accept the two Hash and URL formats (with HTTP URLs) (§3.6) | **NOT IMPL** |
| `RFC7296-3.6-3` | MUST | Implementations MUST support the http: scheme for hash-and-URL lookup (§3.6, §1.7) | **NOT IMPL** |
| `RFC7296-3.9-1` | MUST | The size of the Nonce Data MUST be between 16 and 256 octets, inclusive (§3.9) | impl-testable |
| `RFC7296-3.9-2` | MUST NOT | Nonce values MUST NOT be reused (§3.9) | impl-testable |
| `RFC7296-3.10-1` | MUST | For notifications concerning Child SAs, the Protocol ID field MUST contain either (2) to indicate AH or (3) to indicate ESP (§3.10) | **NOT IMPL** |
| `RFC7296-3.10-2` | MUST | If the SPI field is empty, the Protocol ID field MUST be sent as zero and MUST be ignored on receipt (§3.10) | **NOT IMPL** |
| `RFC7296-3.10-3` | MUST | For a notification concerning the IKE SA, the SPI Size MUST be zero and the SPI field must be empty (§3.10) | impl-testable |
| `RFC7296-3.10.1-1` | MUST | An implementation receiving a Notify payload with one of these types that it does not recognize in a response MUST assume that the corresponding request has failed entirely (§3.10.1) | uncertain |
| `RFC7296-3.10.1-2` | MUST | Unrecognized error types in a request and status types in a request or response MUST be ignored, and they should be logged (§3.10.1) | uncertain |
| `RFC7296-3.10.1-3` | MUST | To avoid leaking information to someone probing a node, this status MUST be sent in response to any error not covered by one of the other status types (§3.10.1) | **NOT IMPL** |
| `RFC7296-3.11-1` | MUST NOT | Each SPI MUST be for the same protocol. Mixing of protocol identifiers MUST NOT be performed in the Delete payload (§3.11) | impl-testable |
| `RFC7296-3.11-2` | MUST | The SPI Size MUST be zero for IKE (SPI is in message header) or four for AH and ESP (§3.11) | impl-testable |
| `RFC7296-3.12-1` | MUST NOT | A Vendor ID payload MUST NOT change the interpretation of any information defined in this specification, i.e., the critical bit MUST be set to 0 (§3.12) | **NOT IMPL** |
| `RFC7296-3.12-2` | MUST | Unfamiliar Vendor IDs MUST be ignored (§3.12) | impl-testable |
| `RFC7296-3.12-3` | MUST | Writers of documents who wish to extend this protocol MUST define a Vendor ID payload to announce the ability to implement the extension in the document (§3.12) | impl-untested |
| `RFC7296-3.13.1-1` | MUST | For protocols for which port is undefined (including protocol 0), or if all ports are allowed, the Start Port field MUST be zero (§3.13.1) | **NOT IMPL** |
| `RFC7296-3.13.1-2` | MUST | For protocols for which port is undefined (including protocol 0), or if all ports are allowed, the End Port field MUST be 65535 (§3.13.1) | **NOT IMPL** |
| `RFC7296-3.13.1-3` | MUST | Systems that wish to indicate OPAQUE ports, but not ANY ports, MUST set the start port to 65535 and the end port to 0 (§3.13.1) | **NOT IMPL** |
| `RFC7296-3.14-1` | MUST NOT | Peers MUST NOT negotiate transforms for which no such specification exists (§3.14) | **NOT IMPL** |
| `RFC7296-3.14-2` | MUST | Senders MUST select a new unpredictable IV for every message (§3.14) | impl-testable |
| `RFC7296-3.14-3` | MUST | Initialization Vector -- recipients MUST accept any value (§3.14) | impl-untested |
| `RFC7296-3.14-4` | MUST | Padding MAY contain any value chosen by the sender, and MUST have a length that makes the combination of the payloads, the Padding, and the Pad Length to be a multiple of the encryption block size (§3.14) | impl-untested |
| `RFC7296-3.14-5` | MUST | Pad Length -- the recipient MUST accept any length that results in proper alignment (§3.14) | impl-untested |
| `RFC7296-3.14-6` | MUST | The checksum MUST be computed over the encrypted message (§3.14) | impl-testable |
| `RFC7296-3.15.1-1` | MUST | Only one netmask is allowed in the request and response messages, and it MUST be used only with an INTERNAL_IP4_ADDRESS attribute (§3.15.1) | **NOT IMPL** |
| `RFC7296-3.15.1-2` | MUST NOT | Non-empty values for the INTERNAL_IP4_NETMASK attribute in a CFG_REQUEST do not make sense and thus MUST NOT be included (§3.15.1) | **NOT IMPL** |
| `RFC7296-3.15.1-3` | MUST | When used within a Request, the SUPPORTED_ATTRIBUTES attribute MUST be zero-length and specifies a query to the responder to reply back with all of the attributes that it supports (§3.15.1) | **NOT IMPL** |
| `RFC7296-3.15.1-4` | MUST | Unrecognized or unsupported attributes MUST be ignored in both requests and responses (§3.15.1) | **NOT IMPL** |
| `RFC7296-3.15.1-5` | MUST | The responder MUST return a Configuration payload if it accepted any of the configuration data, and the Configuration payload MUST contain the attributes that the responder accepted with zero-length data (§3.15.1) | **NOT IMPL** |
| `RFC7296-3.15.1-6` | MUST NOT | Those attributes that it did not accept MUST NOT be in the CFG_ACK Configuration payload (§3.15.1) | **NOT IMPL** |
| `RFC7296-3.15.1-7` | MUST | If no attributes were accepted, the responder MUST return either an empty CFG_ACK payload or a response message without a CFG_ACK payload (§3.15.1) | **NOT IMPL** |
| `RFC7296-3.16-1` | MUST | In a response message, the Identifier octet MUST be set to match the identifier in the corresponding request (§3.16) | uncertain |
| `RFC7296-3.16-2` | MUST | The Length field MUST be four less than the Payload Length of the encapsulating payload (§3.16) | uncertain |
| `RFC7296-3.16-3` | MUST | For codes other than Request or Response, the EAP message length MUST be four octets and the Type and Type_Data fields MUST NOT be present (§3.16) | uncertain |
| `RFC7296-3.16-4` | MUST | In a Response (2) message, Type MUST either be Nak or match the type of the data requested (§3.16) | uncertain |
| `RFC7296-4-1` | MUST | If the responder rejects the CREATE_CHILD_SA request with a NO_ADDITIONAL_SAS notification, the implementation MUST be capable of instead deleting the old SA and creating a new one (§4) | **NOT IMPL** |
| `RFC7296-4-2` | MUST | If an implementation supports responding to such requests, it MUST parse the CP payload of type CFG_REQUEST in the first message in the IKE_AUTH exchange and recognize a field of type INTERNAL_IP4_ADDRESS or INTERNAL_IP6_ADDRESS (§4) | **NOT IMPL** |
| `RFC7296-4-3` | MUST | If it supports leasing an address of the appropriate type, it MUST return a CP payload of type CFG_REPLY containing an address of the requested type (§4) | **NOT IMPL** |
| `RFC7296-4-4` | MUST | For an implementation to be called conforming to this specification, it MUST be possible to configure it to accept PKIX certificates containing and signed by RSA keys of size 1024 or 2048 bits, and shared secret authentication (§4) | **NOT IMPL** |
| `RFC7296-5-1` | MUST NOT | A PRF whose output is less than 128 bits MUST NOT be used with this protocol (§5) | **NOT IMPL** |
| `RFC7296-5-2` | MUST NOT | Implementations MUST NOT negotiate NONE as the IKE integrity protection algorithm or ENCR_NULL as the IKE encryption algorithm (§5) | impl-testable |

## Phase 1 Results and Four Resolutions (2026-07-30)

Phase 1 revalidated the walk, fetched the errata, and found three structural problems
with the plan. All four resolutions below are mechanical readings of existing rules.
None needed an owner ruling.

### Revalidation verdicts

| Assumption | Verdict | Evidence |
|------------|---------|----------|
| A-1, the 214 count | **confirmed** | A second independent pass re-measured 214 rows. Classes 63/25/18/108. Levels 168 MUST and 46 MUST NOT. Zero parse errors |
| A-5, id freeness | **confirmed** | Zero collisions with the 23 existing ids. Zero internal duplicates. Zero ordinals at or below a per-section high-water mark. `check_id_allocation` clean when driven |
| A-6, derived register | **confirmed** | `derive_inventory("rfc7296", 18)` returns `rfc2119`, 261 sites, 104 sections, `source-sha a6f1a101b818977b` |
| A-4, skeleton parses | **confirmed** | 261 sites and 104 sections, all unclassified. It re-parses through `parse_extraction_artifact` |
| A-10, errata | **broken** | Nine errata exist. Two touch Appendix A |
| A-8, the 31 tags map to 23 ids | **broken** | They map to **16**. `RFC7296-2.9-1` and `-1.4-1` are both `{gap}` and carry no tags. The five SHOULD rows carry none |

### Resolution 1: the errata, and the one that would have caused a wrong implementation

**EID 6940, Verified, Technical, section 3.10.** Row `RFC7296-3.10-3` quoted the
UNCORRECTED text. "the field must be empty" is corrected to "the SPI field must be
empty". Appendix A now carries the corrected text. The id contract pins quoted text by
sha, so an uncorrected quote would have made a misquote into the obligation.

**EID 5056, Held for Doc Update, Technical, section 1.7, is the substantive one.**

Implementing `RFC7296-1.7-1` as literally quoted would ignore an ENTIRE SA proposal:

- The correct behaviour ignores ONE configuration attribute.
- Attribute type 5 is a Configuration payload attribute. It is not a proposal attribute.
- The erratum verifier's words: `only the attribute type should be ignored, not the entire proposal`.

→ Constraint: the row keeps the verbatim text. That is what the id contract pins, and
what a reader compares against the source. WP-9 implements the CORRECTED semantics.

→ Constraint: the row records the erratum beside the quote. A later reader must not
implement the literal reading by accident.

→ Constraint: WP-9 settles whether `RFC7296-1.7-1` duplicates `RFC7296-3.15.1-4`, which
says unrecognized attributes MUST be ignored. Both cover an ignored attribute. When they
are one obligation, the sign-off excludes one site as `duplicate-of` the other.

**EID 9027, Reported, section 2.21.1** says COOKIE is a status, not an error type. No row
cites that section. It bears on how WP-3 and WP-4 frame the cookie exchange.

No erratum requires editing `rfc/full/rfc7296.txt`. The `source-sha` pin is intact.

### Resolution 2, for R-12: a red-first Python test cannot be committed red, and need not be

`make ze-rfc-check` runs `--selftest` then `--check` (`Makefile:437-439`). `--selftest`
runs the whole of `scripts/dev/rfc_requirements_test.py` (`rfc_requirements.py:6440`).
`scripts/dev/python_tests_test.go:43-52` globs that same file into `ze-unit-test`. So a
red test under `scripts/dev/` reds BOTH verify stages, and `commit_helper.py` then
refuses every session's commits.

**This is not a conflict between TDD and the gate.** `ai/rules/tdd.md` requires the test
to FAIL before the implementation exists. It does not require committing it red.

| Step | Where it happens |
|------|------------------|
| Write the test, observe red, record the message | Working tree, uncommitted |
| Implement | Working tree |
| Commit test and implementation together, green | One commit |

That is the rule the Landing Strategy already states for checklist rows. A row and its
proof land in the same commit. The five wiring tests land in the commit that turns them
green. Phase 1's red evidence is the recorded failure message for each.

| Test | Red observed in phase 1 |
|------|-------------------------|
| `test_rfc7296_signoff_is_valid_and_the_rest_stay_grandfathered` | `rfc/extraction/rfc7296.json does not exist` |
| `test_rfc7296_every_gated_row_is_proven_in_both_polarities` | `237 != 23 : the pilot lands 237 rows` |
| `test_rfc7296_ledger_is_fresh_after_the_pilot` | `237 != 23 : the pilot lands 237 rows` |
| `test_rfc7296_summary_carries_the_section_2_2_requirements` | `missing from rfc/short/rfc7296.md` |
| `test_rfc7296_ids_are_neither_retired_nor_demoted` | `237 != 23 : the pilot lands 237 rows` |

→ Constraint: no Python gate-test incubator is built. That would be new infrastructure
for a problem correct commit discipline already solves.

### Resolution 3, for R-13: the unsigned skeleton stays OUT of the tree until signed

An unsigned skeleton parses. It does not pass. It yields 385 gate errors and exit 2.

The 385 break down as:

- 261 unclassified sites.
- 104 unclassified sections.
- An empty `signed-off` and an empty `reviewer`.
- 18 reverse-arithmetic errors, one per gated row at HEAD.

~~The Landing Strategy says the artifact can be checked mid-walk.~~ **Corrected
2026-07-30.** It can be checked mid-walk only while UNCOMMITTED. The skeleton lives in
the working tree during the walk. It is committed once, signed, in the final phase.

### Resolution 4, for R-14: AC-5's closure figures were already false

`spec-rfcgate-4-ledger` signed four artifacts on 2026-07-30 and enrolled two stems. The
corpus is at 168 enrolled, not 166.

| Claim | Was | Corrected |
|-------|-----|-----------|
| AC-5, exactly one sign-off present | 1 | **5 artifacts, of which 3 are signed AND enrolled** |
| Deliverables, one file from `ls rfc/extraction/*.json` | 1 | **5** |
| 165 unsigned | coincidence | **165 enrolled stems unsigned**, which holds only because two of child 4's four are not enrolled |

→ Constraint: a closure check that counts artifacts is wrong by construction, because the
fleet drain adds them continuously. Assert that `rfc7296` is signed and valid. Never
assert that it is the only one.

### One planning datum for the sign-off

**261 sites must be classified against 232 gated ids.** Roughly 29 sites need an
exclusion or a `duplicate-of` mapping. That is the arithmetic the final phase closes.

## Phase 3 -- the 18 uncertain rows resolved (2026-07-30)

Each row was resolved by reading the producing code, never by reading a caller. The tally
below replaces the walk's `uncertain` class for all 18.

| Bucket | Count |
|--------|-------|
| `impl-testable` | 11 |
| `impl-untested` | 0 |
| `NOT IMPL` | 5 |
| Owner ruling | 2 |

Nothing landed in `impl-untested`. Every behaviour that exists is reachable from a
package-visible entry, so phase 3 hands no new infrastructure debt to phase 2b. This agrees
with the phase 2a triage correction recorded in Appendix A.

**The eleven that are provable now.** `RFC7296-2.5-4`, `2.16-3`, `2.16-6`, `2.16-7`,
`2.16-8`, `3.4-1`, `3.10.1-2`, `3.16-1`, `3.16-2`, `3.16-3` and `3.16-4`. Each joins the
phase 2b row set and needs a tagged pair.

**The five that move into phases 4-15.**

| Row | What is absent |
|-----|----------------|
| `RFC7296-2.12-1` | The IKE SA's `SKKeys` are never cleared on close. `SKKeys.Clear` is called from `established.go:161` and `reconcile.go:167` only, and both are rekey paths. |
| `RFC7296-3.5-1` | No terminator check exists. `encodeIKEID` returns `[]byte(id)` unexamined (`auth.go:473`), and the YANG leaf is a bare string. A `local-id` holding NUL or CR reaches the wire. |
| `RFC7296-3.10.1-1` | No code classifies a notify by the 0 to 16383 error range. The only use of 16384 is the `NotifyInitialContact` constant (`payload_notify.go:27`). |
| `RFC7296-2.24-1` | No ECN field exists. `SAParams` carries none (`dataplane.go:81-108`). |
| `RFC7296-2.24-2` | The XFRM and VPP backends set no ECN flag. |

**Owner rulings (Thomas, 2026-07-30).** All three were raised under
`ai/rules/rfc-compliance.md`, which reserves a compliance judgement to the owner.

OR-D: `RFC7296-2.5-3` is discharged by proof, never by annotation. Ze supports the singleton
{2}, and the inbound gate is an equality test on the raw header byte (`register.go:455` and
`register.go:625`). A tagged pair must assert that Ze accepts major version 2 and drops every
other value. The row stays gated, so a future second supported version cannot pass unnoticed.

OR-E: `RFC7296-2.16-5` is discharged by proof, never by annotation. Ze refuses every EAP
method that derives no shared key (`eap.go:141-142`). A tagged pair must assert that refusal,
which keeps the keyless mode the MUST governs unreachable. The row stays gated, so a wider
accepted-method set cannot pass unnoticed.

OR-F: `RFC7296-2.24-1` and `2.24-2` are not classified yet. The Linux XFRM and the VPP IPsec
sources must be read first. VPP is vendored at `third_party/`. The ruling then follows their
code, rather than an inference about a foreign system.
