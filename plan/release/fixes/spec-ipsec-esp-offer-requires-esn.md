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
| `TestRFC7296ResponderRefusesESPOfferMissingESN` | `internal/component/ike/engine/rfc7296_esp_mandatory_type_test.go` | AC-1, AC-2 on IKE_AUTH and CREATE_CHILD_SA; AC-1 on a Child SA rekey | written, not run |
| `TestEsnResponderAnswersOnlyAValueTheOfferCarried` | `internal/component/ike/engine/rfc7296_esn_test.go` | ESN value behavior preserved | unchanged |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipsec-sa-installed` | `test/ipsec/ipsec-sa-installed.ci` | AC-3: two daemons establish a Child SA through the responder's selection | to run |
| `ipsec-child-rekey` | `test/ipsec/ipsec-child-rekey.ci` | AC-3: a Child SA rekey passes the same reader | to run |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `psk-site-to-site` | `test/interop-ipsec/scenarios/psk-site-to-site` | strongSwan | AC-3: an ordinary offer still establishes | to run |
| `esn-both-offered` | `test/interop-ipsec/scenarios/esn-both-offered` | strongSwan | ESN 0 and 1 offered, Ze selects 0 | to run |

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
