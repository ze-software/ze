# Spec: ipsec-esp-offer-requires-esn

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Priority | P1 |
| Release reason | Ze as IKEv2 responder accepts an ESP offer that lacks a mandatory transform type, so shipped negotiation breaks RFC 7296 Section 3.3.3. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-09 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Found while closing `spec-ipsec-esp-selection-refusal` (the initiator half of
the same rule). As responder, `espProposalMatches`
(`internal/component/ike/engine/responder.go`) collects every Transform Type 5
value of an offered ESP proposal and refuses the proposal only when ESN values
are present and none is value 0 (`len(esns) > 0 && !slices.Contains(esns,
espESNNotExtended)`). A proposal that carries no ESN transform at all skips the
guard and is accepted.

RFC 7296 Section 3.3.3 lists ENCR and ESN as the mandatory transform types for
ESP. To be designed: refuse such an offer with NO_PROPOSAL_CHOSEN through the
existing `selectResponderESP` refusal path, quoting the RFC text above the
check, and convert the tests that build ESP offers without an ESN transform.
A test change that could lower what a test proves goes to the `ze-test-judge`
skill.

The 2026-08-29 `silent-fall-through` row described this half already, and
`plan/release/fixes/spec-ike-eap-one-off-defects.md` lists that row under
"Rows found already fixed". Its evidence covers the unoffered-value half only:
the missing-type half is still open, and this spec owns it.

## Required Reading

### Architecture Docs
- [ ] `ai/digests/ipsec-ike.md` - how an IKE SA and its Child SA are negotiated
  → Constraint: to be filled at design

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc7296.md` - Sections 2.7, 3.3.3 and 3.3.6
  → Constraint: to be filled at design

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ike/engine/responder.go` - `espProposalMatches` accepts an ESP proposal with no ESN transform

**Behavior to preserve:**
- An offer that includes ESN value 0 is accepted and answered with ESN 0; an offer of ESN 1 alone is refused

**Behavior to change:**
- An ESP offer with no Transform Type 5 is refused with NO_PROPOSAL_CHOSEN

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An IKE_AUTH or CREATE_CHILD_SA request carrying an SA payload with an ESP proposal

### Transformation Path
1. to be filled at design

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Peer to IKE engine | to be filled at design | No |

### Integration Points
- to be filled at design

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | No | to be filled at design |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| IKE_AUTH request whose SAi2 omits ESN | → | `espProposalMatches` | to be named at design |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | An ESP proposal offered to Ze carries no Transform Type 5 | The proposal is unacceptable; with no other acceptable proposal Ze answers NO_PROPOSAL_CHOSEN and installs no Child SA |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| to be filled at design | - | AC-1 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| to be named at design | `test/` | AC-1 | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| to be named at design | - | strongSwan | offers with ESN still establish | |

## Files to Modify
- `internal/component/ike/engine/responder.go` - `espProposalMatches`

## Implementation Steps

1. to be filled at design

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `silent-fall-through.md` | 2026-11-09 | responder `espProposalMatches` accepts an ESP offer with no ESN transform |

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] `./le test functional ipsec` and the scoped `internal/component/ike/engine` package tests green

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
