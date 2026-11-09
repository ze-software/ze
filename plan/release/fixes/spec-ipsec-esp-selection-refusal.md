# Spec: ipsec-esp-selection-refusal

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Priority | P1 |
| Release reason | Existing non-ESN negotiation must refuse a malformed accepted ESP selection before installing a Child SA. |
| Depends | Existing native IKEv2 engine (the original spec-ipsec-0-umbrella dependency's engine prerequisite is recorded as satisfied); no dependency on new ESN support |
| Phase | - |
| Updated | 2026-11-09 |

## Task

Own the existing-capability portion of AC-6b split from
`plan/post-release/features/spec-ipsec-12-esn.md`. Ze offers No-ESN today.
The original's final amendment records that `acceptedESPESNConsistent` in
`internal/component/ike/engine/initiator.go` rejects duplicate and unoffered
values but accepts zero ESN transforms (`esns <= 1`). Missing-transform
refusal and its evidence are still owed. Repair that receive-path boundary,
not the optional 64-bit counter feature.

## Ownership and preserved decisions

| Original obligation | Owner |
|---------------------|-------|
| AC-6b for the current No-ESN offer, including missing-transform refusal; existing-selection part of Data Flow step 3, R-3, transform-value boundary and negotiation-phase work | This spec |
| `TestInitiatorParseAgreedESN` refusal cases for the current offer, plus proof that refusal occurs before Child SA installation | This spec |
| AC-1..AC-6, AC-7/7b, AC-8/8b, AC-9..AC-11; AC-6b coverage of new required/optional offers; all new config, negotiation modes, rekey and XFRM/VPP ESN plumbing and associated proof/docs | `plan/post-release/features/spec-ipsec-12-esn.md` |

The feature sibling retains the full original narrative, tests and dated
amendments as context. Its August 29 AC-7b evidence proves honest refusal of
unsupported extended-only offers, not missing-transform refusal. Preserve the
existing `esn-extended-only-refused` and `esn-both-offered` interop evidence and
`RFC7296-2.7-1` evidence ratchet. No feature or conformance verdict changes in
this planning split. `ready` is inherited, not a new review or readiness claim.

## Constraints and dependencies

- RFC 7296 Section 2.7 requires exactly one selected transform of each offered
  type; Section 3.3.3 lists ESN as mandatory for ESP. Do not substitute a default
  No-ESN value or warning for malformed-selection refusal.
- The existing engine is the prerequisite. New ESN configuration, XFRM/VPP ESN
  flags and VPP integration are not dependencies of this repair.
- Keep default offers as exactly one ESN value 0 and leave IKE proposals free
  of ESN. Do not lift unsupported ESN refusal before the feature and dataplane
  can honor it. Do not change wire encoding, add config leaves or claim ESN support.
- Read the sibling's Required Reading, Current Behavior and final amendment
  before implementation; earlier handler/line references are dated history.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|---------------------------------------------------------------------|
| Accepted-offer ESN check | `internal/component/ike/engine/initiator.go` `acceptedESPESNConsistent` | Refused an unoffered or duplicated ESN value, accepted none | Extends: the same function now also refuses a missing ESN transform |
| Accepted-offer verification and its callers | `initiator.go` `verifyAcceptedOffer`, called from `fsm.go` (IKE_AUTH response), `create_child.go` and `rekey.go` | Return `errAcceptedOfferESN` before any Child SA SPI is adopted or installed | Does not change: the callers already refuse before install |
| The ESP offer's single ESN value | `initiator.go` `espESNNotExtended`, `wireESPOffer` | Every ESP proposal offers exactly one ESN transform, value 0 | Does not change: it is why exactly one is the only valid answer |
| Daemon-path SA establishment proof | `test/ipsec/ipsec-sa-installed.ci` | Two ze daemons negotiate IKEv2 over loopback, the initiator reading the responder's real SAr2 | Reused as the daemon-path proof that the stricter check still accepts ze's own conformant answer |

## Acceptance Criteria

| ID | Condition | Expected behavior |
|----|-----------|-------------------|
| AC-6b | SAr2 answering the current non-ESN offer carries zero or multiple ESN transforms, or an unknown/unoffered value | Refuse the exchange before Child SA installation; no default No-ESN value substitutes for the malformed selection |

## Implementation and proof obligations

1. Repair the existing accepted-ESP selection validation and its caller boundary
   in `internal/component/ike/engine/initiator.go` and the current IKE_AUTH
   response path. Do not implement optional ESN support here.
2. Carry the existing-offer portion of `TestInitiatorParseAgreedESN` from the
   source TDD plan: missing, duplicate, unknown and unoffered selections must
   refuse before install; one explicitly offered value 0 remains accepted.
3. Preserve existing default non-ESN behavior and the dated unit/interop
   evidence above. Any documentation touched must distinguish repair of the
   current receive path from absent ESN capability. Tests and product edits
   belong to implementation, not this reorganization.

## Files to Modify

- `internal/component/ike/engine/initiator.go` - accepted ESP selection consistency for the current No-ESN offer.
- `internal/component/ike/engine/fsm.go` - the IKE_AUTH response boundary that must refuse a malformed selection before Child SA installation.

Ownership is limited to the existing-offer AC-6b repair and its proof; new ESN
configuration, negotiated state and dataplane plumbing remain with the feature
sibling. This list claims no implementation.

## Remaining state

Closed 2026-11-09. `acceptedESPESNConsistent` now requires exactly one ESN
transform, so a missing selection is refused like a duplicate or unoffered one.
The sibling `plan/post-release/features/spec-ipsec-12-esn.md` owns adapting this
same boundary when new ESN offers are introduced.

## Deliverables Checklist

Added at closure: the split spec carried none.

| Deliverable | Verification method | Evidence |
|-------------|---------------------|----------|
| Missing-transform refusal | Read `acceptedESPESNConsistent` (`internal/component/ike/engine/initiator.go`) | Returns `esns == 1`; a non-zero ID returns false inside the loop |
| `TestInitiatorParseAgreedESN` with missing, duplicated, unassigned, unoffered and offered cases | Read `internal/component/ike/engine/rfc7296_esn_test.go` | Five cases, each run at `verifyAcceptedOffer` and at `handleAuthResponse` |
| Refusal before Child SA install | Read callers of `verifyAcceptedOffer` | `fsm.go` (IKE_AUTH response) returns with `StateDead` before the SPI is adopted; `rekey.go` and `create_child.go` return the error before install |
| Discrimination record | `rfc/discrimination/rfc7296/RFC7296-3.3.6-3.json` | Negative and positive mutant records, producer `acceptedESPESNConsistent` |
| Daemon path still establishes | `test/ipsec/ipsec-sa-installed.ci` | Owed main-thread run: `./le test functional ipsec` selecting `ipsec-sa-installed` |

## Security Review Checklist

Added at closure.

| Concern | Finding |
|---------|---------|
| Peer-driven panic or unbounded work | None: a loop over the already-parsed, bounded transform list returning a bool |
| Downgrade to a default | Removed: an SAr2 with no ESN transform is no longer read as No-ESN |
| Legitimate peer refused | None: `wireESPOffer` puts exactly one ESN transform (value 0) in every ESP proposal, so a conformant answer carries exactly one |
| Information leakage | The WARN log names the peer and the fixed error text, no key material |

## Documentation Update Checklist

Added at closure.

| Category | Update? | Evidence |
|----------|---------|----------|
| Feature list (`features/<id>.md`) | No | No feature's scope moves: Ze still offers No-ESN only |
| User guide, config syntax, CLI, API | No | No config leaf, command or output changed |
| Architecture / wire docs | No | `grep -rn 'acceptedESPESNConsistent\|errAcceptedOfferESN' docs/ ai/` finds nothing; the ESN lines in `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` describe XFRM state, not offer checking |
| RFC compliance | Yes | `RFC7296-3.3.6-3` gains the tagged unit `TestInitiatorParseAgreedESN` and its record; `./le rfc update` regenerated nothing new |

---

## Implementation Summary

### What Was Implemented
- `acceptedESPESNConsistent` (`internal/component/ike/engine/initiator.go`) answers
  `esns == 1` in place of `esns <= 1`, quoting RFC 7296 Section 2.7;
  `errAcceptedOfferESN` text and the doc comments name the missing case.
- `TestInitiatorParseAgreedESN` (`rfc7296_esn_test.go`), tagged
  `RFC7296-3.3.6-3` negative and positive.
- `negAuthResponseRewritten` (`rfc7296_negotiation_test.go`): `negAuthResponse`
  generalized to any SAr2 rewrite; its assertions moved unchanged.

### Bugs Found/Fixed
- An SAr2 with no Transform Type 5 was accepted as No-ESN. Covered by
  `TestInitiatorParseAgreedESN/missing/*`.

### Documentation Updates
- None in `docs/`: grep evidence in the Documentation Update Checklist.
- Closure fixed the stale comment above `espESNNotExtended` (`initiator.go`):
  it called `acceptedESPESNConsistent` "below" and cited a dead spec path;
  it now says "above" and cites `plan/post-release/features/spec-ipsec-12-esn.md`.

### Deviations from Plan
- `fsm.go` is listed in Files to Modify and was not changed: its IKE_AUTH
  response handler already called `verifyAcceptedOffer` and returned with
  `StateDead` before adopting the Child SA SPI, so the repair of the check alone
  moved the refusal ahead of installation.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The spec reached closure without Deliverables, Security Review and Documentation Update checklists | A planning split still owes them | `/ze-close` precondition | Added at closure from the evidence |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Refuse a missing ESN transform in the accepted ESP selection | Done | `initiator.go` `acceptedESPESNConsistent` | |
| Keep duplicate and unoffered refusal | Done | same function | Cases `duplicated`, `unassigned`, `unoffered-extended` |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-6b | Done | `TestInitiatorParseAgreedESN` | Refused cases leave `StateDead` and `ChildOutboundSPI == 0` |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestInitiatorParseAgreedESN` | Done | `internal/component/ike/engine/rfc7296_esn_test.go` | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/ike/engine/initiator.go` | Done | |
| `internal/component/ike/engine/fsm.go` | Changed | No edit needed; see Deviations |

### Audit Summary
- **Total items:** 6
- **Done:** 5
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (fsm.go, recorded in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A malformed accepted ESP selection, including a missing ESN transform, is refused before a Child SA is installed | Unit test over a real handshake, with recorded breaks | `TestInitiatorParseAgreedESN`: a real Ze responder builds the SAr2, the test rewrites and reseals it, and `handleAuthResponse` leaves refused answers dead with no Child SA SPI. Break `esns == 1` to `esns <= 1` turned `missing/verify` and `missing/ike-auth-response` red; break to `esns == 2` turned `the-value-we-offered` and `duplicated` red (`rfc/discrimination/rfc7296/RFC7296-3.3.6-3.json`). Package run of `./internal/component/ike/engine/` with full feature tags: ok (202s) |
| Default non-ESN behavior preserved | Unit test, functional, interop | `the-value-we-offered` establishes the SA; daemon path `test/ipsec/ipsec-sa-installed.ci` and strongSwan interop `psk-site-to-site` (`IPSEC_INTEROP_SCENARIO=psk-site-to-site ./le test integration interop-ipsec`) are the main thread's runs |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| Responder refusal of an ESP offer with no ESN transform | Responder half, separate boundary (`espProposalMatches`) | `plan/release/fixes/spec-ipsec-esp-offer-requires-esn.md` |
| New ESN offers, config and dataplane | Optional feature, post-release | `plan/post-release/features/spec-ipsec-12-esn.md` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/ipsec-esp-selection-refusal-f90e7822-ecbf-462e-ac37-c494b4c2c988.md` (7 files, verdict clean) |
| `./le spec review check` | clean |
| Rounds | 2 |
| Reviewer lenses used | Round 1: independent ze-read reviewer (logic, wiring, RFC quotes, install ordering, test discrimination). Round 2: closure agent (stale comments, citations, weakening, RFC proof freshness, style) |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | New tagged unit carried no discrimination record | `rfc/discrimination/rfc7296/RFC7296-3.3.6-3.json` | Two `./le rfc record` runs, negative and positive mutant |

### Notes
- Round 2 fixed a stale comment above `espESNNotExtended` ("below", and a dead spec path). Style pass: no peer-reachable panic, no new switch, no dropped error.
- `./le test weakened check` flagged `negAuthResponse` (4 assertions moved out); an independent ze-test-judge pass approved it under (a), recorded with `./le test weakened approve`.
- Two audit verdicts shifted by the test file growth (`RFC7296-2.7-1`, `RFC4303-2.2.1-2`) were resealed with `./le rfc reseal`; nothing re-judged.
- `plan/journal/silent-fall-through.md` 2026-08-29 row says a missing-ESN refusal would redden the `verifyAcceptedOffer` tests: not true for the initiator. The file carries other sessions' hunks and the row's owner is `spec-ipsec-esp-offer-requires-esn`, so it is left for that spec.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `rfc/discrimination/rfc7296/RFC7296-3.3.6-3.json` | Yes | untracked file, two records |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-6b | Missing, duplicate, unknown, unoffered refused before install | `initiator.go` `return esns == 1`; package test ok (202s); `./le rfc check` names no record problem for RFC7296-3.3.6-3 |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| IKE_AUTH response `handleAuthResponse` (`fsm.go`) | `test/ipsec/ipsec-sa-installed.ci` (accept path, two daemons); the refusal needs a forged responder, so `TestInitiatorParseAgreedESN/*/ike-auth-response` drives the real handler over a real handshake | Unit: yes. `.ci`: main-thread run |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| Every Ze ESP offer carries exactly one ESN transform | confirmed | Test asserts `esnIDsOf(buildWireESPProposals(...)[0])` equals `[espESNNotExtended]` |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| No page describes the offer check | grep in Documentation Update Checklist | Yes |
