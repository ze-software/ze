# Spec: ipsec-esp-offer-requires-esn

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Priority | P1 |
| Release reason | Ze as IKEv2 responder accepts an ESP offer that lacks a mandatory transform type, so shipped negotiation breaks RFC 7296 Section 3.3.3. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-10 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Found while closing `spec-ipsec-esp-selection-refusal` (the initiator half of
the same rule). As responder, `espProposalMatches`
(`internal/component/ike/engine/responder.go`) collects every Transform Type 5
value of an offered ESP proposal and refuses the proposal only when ESN values
are present and none is value 0 (`len(esns) > 0 && !slices.Contains(esns,
espESNNotExtended)`). A proposal that carries no ESN transform at all skips the
guard and is accepted, and `espProposalWire` then answers it with an ESN
transform the peer never offered.

RFC 7296 Section 3.3.3 lists ENCR and ESN as the mandatory transform types for
ESP. Refuse such an offer with NO_PROPOSAL_CHOSEN through the existing
`selectResponderESP` refusal path, quoting the RFC text above the check, and
convert the tests that build ESP offers without an ESN transform. A test change
that could lower what a test proves goes to the `ze-test-judge` skill.

The 2026-08-29 `silent-fall-through` row described this half already, and
`plan/release/fixes/spec-ike-eap-one-off-defects.md` lists that row under
"Rows found already fixed". Its evidence covers the unoffered-value half only:
the missing-type half was still open, and this spec owns it.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/access-security.md` - the IKE engine is gated `ze_ike`; `ike/crypto` stays ungated
  → Constraint: the mandatory-type set is read from the crypto package's Section 3.3.3 table, never restated in the engine
- [ ] `docs/architecture/ike/ipsec-6-ikev2-crypto.md` - how the two readers of a peer's offer drop unacceptable transforms
  → Constraint: the new refusal sits after the drop, so a type whose every transform was dropped is already refused
- [ ] `docs/architecture/ike/ipsec-14-responder.md` - `selectResponderESP` negotiates one proposal and narrows the ESP group
  → Constraint: the refusal is one more non-match inside that selection, so the one-proposal answer is unchanged
- [ ] `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` - a refused Child SA in IKE_AUTH carries NO_PROPOSAL_CHOSEN and the IKE SA stays up childless
  → Constraint: AC-1's refusal follows that existing route; nothing new is installed
### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc7296.md` - `RFC7296-3.3.6-4` (a proposal missing a mandatory type is unacceptable), `RFC7296-2.7-1`
  → Constraint: "other proposals in the same SA payload are processed as usual", so the refusal is per proposal, not per payload

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ike/engine/responder.go` - `espProposalMatches` accepts an ESP proposal with no ESN transform; `selectResponderESP` and `matchOfferedESP` call it
- [ ] `internal/component/ike/engine/create_child.go` - `selectNewChildESP` reaches it through `matchOfferedESP`
- [ ] `internal/component/ike/engine/rekey.go` - the Child SA rekey responder reaches it through `matchOfferedESP`
- [ ] `internal/component/ike/crypto/proposal.go` - `protocolTransformTypes` holds the Section 3.3.3 table; `espProposalComplete` leaves the ESN check to the wire readers

**Behavior to preserve:**
- An offer that includes ESN value 0 is accepted and answered with ESN 0; an offer of ESN 1 alone is refused
- A refused proposal leaves the peer's later proposals on offer

**Behavior to change:**
- An ESP offer with no Transform Type 5 is refused with NO_PROPOSAL_CHOSEN

## Existing Mechanisms (MANDATORY)

| Mechanism | Where | What it does today | This spec |
|-----------|-------|--------------------|-----------|
| Section 3.3.3 transform-type table | `crypto/proposal.go` `protocolTransformTypes`, `transformTypeMandatory`, `TransformTypeUnderstoodESP` | Declares which types each protocol takes and which are required; the engine reads only the "understood" half | Extends: `MandatoryTransformTypesESP` exports the required half as a bitmask, so the engine reads the fact rather than naming ESN |
| Per-type bitmask in the ESP reader | `engine/responder.go` `espProposalMatches` (`offeredTypes`, `usableTypes`) | Refuses a proposal whose type lost every transform | Extends: the same mask is compared with the mandatory set |
| ESP refusal path | `selectResponderESP`, `selectNewChildESP`, child rekey in `rekey.go`, all through `matchOfferedESP`; `notifyForRefusal` | A proposal that does not match is skipped; none left answers `crypto.ErrNoProposalChosen`, mapped to NO_PROPOSAL_CHOSEN | Reused unchanged: the new refusal is one more "does not match" |
| Initiator-side ESN check | `engine/initiator.go` `acceptedESPESNConsistent` | Refuses an answer that selects no ESN or a value Ze never offered (fix 0093baab76) | Does not apply: that is the initiator half, already fixed |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An IKE_AUTH or CREATE_CHILD_SA request carrying an SA payload with an ESP proposal

### Transformation Path
1. The request handler decodes SAi2 (IKE_AUTH) or SA (CREATE_CHILD_SA) into `wire.PayloadSA`
2. `selectResponderESP` / `selectNewChildESP` / the child rekey responder walk the configured proposals and call `matchOfferedESP`
3. `matchOfferedESP` calls `espProposalMatches` per offered ESP proposal; the proposal is refused when a mandatory type is absent
4. With no proposal left, `crypto.ErrNoProposalChosen` reaches `notifyForRefusal`, which answers NO_PROPOSAL_CHOSEN

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Peer to IKE engine | the decoded SA payload, read by `espProposalMatches` | Yes, `TestRFC7296ResponderRefusesESPOfferMissingESN` |
| IKE engine to crypto | `crypto.MandatoryTransformTypesESP`, a pure read of the table | Yes, same test (the refusal comes from that mask) |

### Integration Points
- `crypto.MandatoryTransformTypesESP` - new exported reader of `protocolTransformTypes`

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | the refusal is inside the one reader all three responder selections share |
| No central enumeration | Yes | the mandatory set comes from the Section 3.3.3 table, not a list in the engine |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| IKE_AUTH SAi2 that omits ESN | → | `selectResponderESP` → `espProposalMatches` | `TestRFC7296ResponderRefusesESPOfferMissingESN/ike-auth-refuses` |
| CREATE_CHILD_SA SA that omits ESN | → | `selectNewChildESP` → `espProposalMatches` | `TestRFC7296ResponderRefusesESPOfferMissingESN/create-child-refuses` |
| CREATE_CHILD_SA Child SA rekey that omits ESN | → | `respondChildRekey` → `matchOfferedESP` → `espProposalMatches` | `TestRFC7296ResponderRefusesESPOfferMissingESN/child-rekey-refuses` |
| Two daemons negotiating IKE_AUTH and a Child SA rekey | → | the same reader over Ze's own offer, which carries ESN 0 | `test/ipsec/ipsec-sa-installed.ci`, `test/ipsec/ipsec-child-rekey.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | An ESP proposal offered to Ze carries no Transform Type 5 | The proposal is unacceptable; with no other acceptable proposal Ze answers NO_PROPOSAL_CHOSEN and installs no Child SA |
| AC-2 | The same SA payload carries a second proposal with ESN 0 | The second proposal is selected and its Proposal Num echoed |
| AC-3 | Ze against Ze, and Ze against strongSwan | Tunnels still establish and rekey, because both offer ESN |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC7296ResponderRefusesESPOfferMissingESN` | `internal/component/ike/engine/rfc7296_esp_mandatory_type_test.go` | AC-1, AC-2 on IKE_AUTH and CREATE_CHILD_SA; AC-1 on a Child SA rekey | pass (full-tag `go test -race`, main thread, 2026-11-10) |
| `TestEsnResponderAnswersOnlyAValueTheOfferCarried` | `internal/component/ike/engine/rfc7296_esn_test.go` | ESN value behavior preserved | unchanged |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipsec-sa-installed` | `test/ipsec/ipsec-sa-installed.ci` | AC-3: two daemons establish a Child SA through the responder's selection | pass (main thread, 2026-11-10) |
| `ipsec-child-rekey` | `test/ipsec/ipsec-child-rekey.ci` | AC-3: a Child SA rekey passes the same reader | pass (main thread, 2026-11-10) |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `psk-site-to-site` | `test/interop-ipsec/scenarios/psk-site-to-site` | strongSwan | AC-3: an ordinary offer still establishes | pass (main thread, 2026-11-10, after an orphaned lab container was cleaned) |
| `esn-both-offered` | `test/interop-ipsec/scenarios/esn-both-offered` | strongSwan | ESN 0 and 1 offered, Ze selects 0 | pass (main thread, 2026-11-10) |

## Files to Modify
- `internal/component/ike/engine/responder.go` - `espProposalMatches`
- `internal/component/ike/crypto/proposal.go` - `MandatoryTransformTypesESP`, `espProposalComplete` comment
- `internal/component/ike/engine/esp_proposal_alternatives_test.go` - fixtures carry ESN 0 (`espESNTransform`)
- `internal/component/ike/engine/rfc7296_aead_mix_test.go` - fixtures carry ESN 0
- `docs/architecture/ike/ipsec-6-ikev2-crypto.md` - the mandatory-type refusal
- `plan/journal/silent-fall-through.md` - the 2026-08-29 and 2026-11-09 rows

## Files to Create
- `internal/component/ike/engine/rfc7296_esp_mandatory_type_test.go`

## Implementation Steps

1. Export the mandatory half of `protocolTransformTypes` for ESP
2. Refuse in `espProposalMatches` when the usable types miss a mandatory one; keep the `len(esns) > 0` conjunct on the ESN value check, so the mask alone owns the presence of ESN (Sections 3.3.3, 3.3.6) and the value check owns its value (Section 3.3.2). Deleting only the mask then turns the missing-ESN subtests red
3. Add an ESN 0 transform to every hand-built ESP offer the responder reader judges, positive and negative alike, so each still fails or passes for its own reason
4. Add the tagged test over both responder selections; record it and re-record the records whose producer changed

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `silent-fall-through.md` | 2026-11-09 | responder `espProposalMatches` accepts an ESP offer with no ESN transform |

## Key Design Decisions

- The mandatory set is read from `protocolTransformTypes` rather than written as a test of `esns`: the Section 3.3.3 table is declared once, and the same check covers ENCR. A missing ENCR is also refused by the encryption comparison that follows, so the mask is the sole refusal only for a missing ESN, and that is the case its tests prove.
- Negative fixtures gain ESN too: without it they would go on refusing, but for the missing type, and stop proving their own refusal.
- strongSwan's and libreswan's source is not in this checkout, so their offer was not read; the interop scenarios are the evidence that strongSwan offers ESN.

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] `./le test functional ipsec` and the scoped `internal/component/ike/engine` package tests green

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it

## Deliverables Checklist

| Deliverable | Verification method | Evidence (closure, 2026-11-10) |
|-------------|---------------------|--------------------------------|
| Mandatory-type mask exported from the Section 3.3.3 table | read `crypto/proposal.go` | `MandatoryTransformTypesESP` iterates `protocolTransformTypes[protoESP]` and sets a bit per `transformTypeRequired` (ENCR, ESN) |
| Responder refuses an ESP offer with no ESN | read `engine/responder.go` | `espProposalMatches`: `usableTypes&mandatory != mandatory` returns false, after the `offeredTypes != usableTypes` drop; the ESN value check keeps `len(esns) > 0` |
| Tagged test over all three responder paths | read `rfc7296_esp_mandatory_type_test.go` | `TestRFC7296ResponderRefusesESPOfferMissingESN`: ike-auth, create-child and child-rekey refusals, both next-proposal subtests, ESN 0 rekey control |
| Fixtures carry ESN so each refusal stays its own | `git show affa3d8684` | `espESNTransform()` added to every hand-built offer in the three changed test files; judge approvals in affa3d8684 and da3ff616f6 |
| Proof records and audit verdicts current | `./le rfc check` | audits for RFC4301-4.2-1, RFC4303-1-2, RFC4303-3.2-1, RFC5282-8-1, RFC7296-3.3-1, RFC7296-3.3.6-5 re-judged and stamped; the RFC7296-3.3.6-5 positive record, stale after the fixture edit, was re-recorded in 83faa3dabe; the records and audits landed in b4e4f7d67d |

## Security Review Checklist

| Concern | Finding |
|---------|---------|
| Peer-controlled input reaching a panic or an out-of-range shift | `1 << t.Type` runs only after `TransformTypeUnderstoodESP` accepted the type (ENCR 1, INTEG 3, D-H 4, ESN 5), so the shift stays below 32; no panic path added |
| Accepting what the peer never offered | the defect closed here: Ze no longer answers ESN to an offer without it |
| Denial of service | the mask is a map walk of four entries per offered proposal; no allocation, no loop over peer data added |
| Downgrade | the refusal only narrows what is accepted; no cipher, key length or integrity rule relaxed |

## Documentation Update Checklist

| Category | Yes/No | Evidence |
|----------|--------|----------|
| Architecture design | Yes | `docs/architecture/ike/ipsec-6-ikev2-crypto.md`: the mandatory-type refusal paragraph (affa3d8684) and the presence-versus-value split (da3ff616f6), anchored to `MandatoryTransformTypesESP`, `protocolTransformTypes`, `espProposalMatches`, `selectResponderESP` |
| RFC compliance | Yes | `RFC7296-3.3.6-4` tagged by the new test; `./le rfc update` regenerates `docs/features/rfc-status.md` (never hand-edited) |
| Feature list, user guide, config syntax, CLI, API, plugin SDK, wire format, comparison | No | no config leaf, command, API or wire field changed; the refusal uses the existing NO_PROPOSAL_CHOSEN path |
| News | Yes | `website/changes/fragments/ipsec-esp-offer-requires-esn.md` |

---

## Implementation Summary

### What Was Implemented
- `crypto.MandatoryTransformTypesESP` (crypto/proposal.go) exports the mandatory half of the RFC 7296 Section 3.3.3 table as a bitmask
- `espProposalMatches` (engine/responder.go) refuses an ESP proposal whose usable types miss a mandatory one, so `selectResponderESP`, `selectNewChildESP` and `respondChildRekey` (through `matchOfferedESP`) go to the peer's next proposal and answer NO_PROPOSAL_CHOSEN when none is left
- `TestRFC7296ResponderRefusesESPOfferMissingESN` tags `RFC7296-3.3.6-4` both polarities

### Bugs Found/Fixed
- The responder accepted an ESP offer with no Transform Type 5 and answered it with an ESN transform the peer never offered (`silent-fall-through.md`, 2026-11-09 row); covered by `TestRFC7296ResponderRefusesESPOfferMissingESN`

### Documentation Updates
- `docs/architecture/ike/ipsec-6-ikev2-crypto.md` (affa3d8684, da3ff616f6), source anchors `crypto/proposal.go -- MandatoryTransformTypesESP, protocolTransformTypes` and `engine/responder.go -- espProposalMatches, selectResponderESP`
- `./le doc check all`: not run in this closure; commit A carries no `docs/` change

### Deviations from Plan
- None in the code. Implementation step 2 was first landed without the `len(esns) > 0` conjunct (affa3d8684) and restored in da3ff616f6 after review round 1

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | affa3d8684 dropped the `len(esns) > 0` conjunct, so a missing ESN was refused twice and deleting the mask left every test green | the claim that the mask refuses was unproven until the conjunct came back | review round 1 ISSUE | fixed in da3ff616f6; red proof re-run by the main thread |
| approach | the test fixture edits staled the RFC7296-3.3.6-5 positive revert record (`appendIKECombinations` break, unit-changed) and it was not re-recorded with the others | every record whose tagged unit changed is owed a re-record, not only those whose producer is `espProposalMatches` | `./le rfc check` at closure | re-recorded by the main thread (revert of `initiator.go::appendIKECombinations`, unit red under the revert), committed as 83faa3dabe |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Refuse an ESP offer with no ESN through the existing refusal path | Done | `engine/responder.go` `espProposalMatches` | mask check after the dropped-type check |
| Quote the RFC text above the check | Done | same, RFC 7296 Section 3.3.6 quoted whole | Section 3.3.3 named |
| Convert tests that build ESP offers without ESN | Done | `esp_proposal_alternatives_test.go`, `rfc7296_aead_mix_test.go`, `rfc7296_unknown_transform_test.go` | judge-approved |
| Weakening-class test changes go to `ze-test-judge` | Done | `RFC-approved:` lines in affa3d8684 (3) and da3ff616f6 (1) | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestRFC7296ResponderRefusesESPOfferMissingESN/ike-auth-refuses`, `/create-child-refuses`, `/child-rekey-refuses` | NO_PROPOSAL_CHOSEN mapping and no replacement Child SA asserted |
| AC-2 | Done | `/ike-auth-takes-the-next-proposal`, `/create-child-takes-the-next-proposal` | Proposal Num 2 asserted |
| AC-3 | Done | Ze against Ze: `test/ipsec/ipsec-sa-installed.ci`, `test/ipsec/ipsec-child-rekey.ci` PASS; Ze against strongSwan: `psk-site-to-site`, `esn-both-offered` PASS (main thread, 2026-11-10) | the first interop attempt failed to start on a Docker address-pool overlap (infrastructure); it passed after an orphaned lab container was cleaned |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestRFC7296ResponderRefusesESPOfferMissingESN` | Done | `internal/component/ike/engine/rfc7296_esp_mandatory_type_test.go` | green; mask-only break turns 5 subtests red |
| `TestEsnResponderAnswersOnlyAValueTheOfferCarried` | Done | `internal/component/ike/engine/rfc7296_esn_test.go` | unchanged, green in the package run |
| `ipsec-sa-installed`, `ipsec-child-rekey` | Done | `test/ipsec/` | PASS, main thread 2026-11-10 |
| interop `psk-site-to-site`, `esn-both-offered` | Done | `test/interop-ipsec/scenarios/` | PASS, main thread 2026-11-10 (`IPSEC_INTEROP_SCENARIO=<name>`) |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/ike/engine/responder.go` | Done | affa3d8684, da3ff616f6 |
| `internal/component/ike/crypto/proposal.go` | Done | affa3d8684 |
| `internal/component/ike/engine/esp_proposal_alternatives_test.go` | Done | affa3d8684 |
| `internal/component/ike/engine/rfc7296_aead_mix_test.go` | Done | affa3d8684 |
| `docs/architecture/ike/ipsec-6-ikev2-crypto.md` | Done | both commits |
| `plan/journal/silent-fall-through.md` | Done | affa3d8684 |
| `internal/component/ike/engine/rfc7296_esp_mandatory_type_test.go` | Done | created in affa3d8684 |

### Audit Summary
- **Total items:** 18
- **Done:** 18
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 0

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A peer's ESP offer without ESN is refused with NO_PROPOSAL_CHOSEN on every responder path | unit over the product entry points, with recorded red | `TestRFC7296ResponderRefusesESPOfferMissingESN` green under full tags; deleting only the `MandatoryTransformTypesESP` check turns the three refusal and both next-proposal subtests red while `child-rekey-accepts-esn-0` stays green (main thread, 2026-11-10); discrimination record `rfc/discrimination/rfc7296/RFC7296-3.3.6-4.json` |
| Other proposals in the same SA payload are still processed | unit | the two takes-the-next-proposal subtests assert Proposal Num 2 is selected |
| Real tunnels still establish and rekey | functional and interop | `test/ipsec/ipsec-sa-installed.ci` and `test/ipsec/ipsec-child-rekey.ci` PASS (two Ze daemons, IKE_AUTH and a Child SA rekey through the responder selection); interop against strongSwan `psk-site-to-site` PASS (an ordinary offer still establishes) and `esn-both-offered` PASS (ESN 0 and 1 offered, Ze selects 0); main thread, 2026-11-10 |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| Extend the "Rows found already fixed" evidence of the 2026-08-29 `silent-fall-through` row to name the missing-type refusal (`MandatoryTransformTypesESP`, da3ff616f6) — review round 1 NOTE 1 | that spec (status design) has no `## Existing Mechanisms` section, so the spec-edit hook refuses any edit until its design is brought up to the template | `plan/release/fixes/spec-ike-eap-one-off-defects.md` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | recorded by `./le spec review record` at closure (path in the closure report) |
| `./le spec review check` | clean at record time |
| Rounds | 2 |
| Reviewer lenses used | round 1 (independent reviewer, main thread): logic, wiring, RFC quoting, test isolation; round 2 (closure agent, no authorship): da3ff616f6 and the whole affa3d8684..da3ff616f6 diff for logic, peer-reachable panics, style, stale comments |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | the mask was not the sole refusal of a missing ESN: without the `len(esns) > 0` conjunct the ESN value check also refused, so deleting the mask left every test green | `engine/responder.go` `espProposalMatches` | da3ff616f6 restores the conjunct; red proof shows the mask alone refuses |

### Run 2
- 0 BLOCKER, 0 ISSUE. NOTEs: `MandatoryTransformTypesESP` walks its four-entry map on every proposal rather than caching (no allocation, below a hot-path concern); review round 1 NOTEs 2-5 were fixed in da3ff616f6, NOTE 1 is the Work Not Done row
- Style pass over `responder.go`, `proposal.go`, `initiator.go` and the test files: no peer-reachable `panic()`, the shift is bounded by the understood-type check, RFC quotes byte-exact (Section 3.3.6 and 3.3.2 sentences checked against `rfc/full/rfc7296.txt`), comments on `espProposalComplete` and `espESNNotExtended` updated with the behaviour

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/ike/engine/rfc7296_esp_mandatory_type_test.go` | Yes | read in full at closure |
| `test/ipsec/ipsec-sa-installed.ci`, `test/ipsec/ipsec-child-rekey.ci` | Yes | `ls` at closure |
| `test/interop-ipsec/scenarios/psk-site-to-site`, `esn-both-offered` | Yes | `ls`: `swanctl.conf`, `ze.conf` each |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | refused on three paths | test body read: `errors.Is(err, crypto.ErrNoProposalChosen)` on `selectResponderESP`, `selectNewChildESP`, `respondChildRekey`; `notifyForRefusal` maps to `NotifyNoProposalChosen`; main-thread run green |
| AC-2 | next proposal taken | `sa.ChildProposalNum != 2` and `number != 2` assertions; green |
| AC-3 | tunnels establish | `ipsec-sa-installed`, `ipsec-child-rekey`, interop `psk-site-to-site`, `esn-both-offered` all PASS (main thread, 2026-11-10) |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| IKE_AUTH SAi2 / CREATE_CHILD_SA / rekey without ESN | unit over `selectResponderESP`, `selectNewChildESP`, `respondChildRekey` | Yes, product entry points, not the matcher alone |
| Two daemons, IKE_AUTH and Child SA rekey | `test/ipsec/ipsec-sa-installed.ci` (12 ESP lines), `test/ipsec/ipsec-child-rekey.ci` (16) | Yes: both files drive ESP and PASS (main thread, 2026-11-10) |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| (no A-N declared) | confirmed | the one design assumption, that Ze's own offer carries ESN 0, holds: `wireESPOffer` and `espESNNotExtended` (`engine/initiator.go`), and the `child-rekey-accepts-esn-0` control |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `ipsec-6-ikev2-crypto.md`: mask owns presence, value check owns value | `espProposalMatches`: mask at the type check, `len(esns) > 0 && !slices.Contains(esns, espESNNotExtended)` after | Yes |
| RFC ledger | `./le rfc update` run at closure after the audit stamps | Yes |
