# Spec: ipsec-esp-selection-refusal

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | protocol |
| Priority | P1 |
| Release reason | Existing non-ESN negotiation must refuse a malformed accepted ESP selection before installing a Child SA. |
| Depends | Existing native IKEv2 engine (the original spec-ipsec-0-umbrella dependency's engine prerequisite is recorded as satisfied); no dependency on new ESN support |
| Phase | - |
| Updated | 2026-10-22 |

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

The missing-transform refusal and proof are open. Existing duplicate/unoffered
refusal is retained evidence, not justification to close AC-6b. The sibling
owns adapting this same boundary when new ESN offers are introduced. No build,
test, protocol audit, implementation or closure is claimed by this split.
