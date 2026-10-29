# Spec: bgp-validation-change-nlri-keyed-consumers

| Field | Value |
|-------|-------|
| Status | skeleton |
| Priority | P1 |
| Release reason | A validation or eligibility change to a route of an NLRI-keyed family (VPN, labeled unicast) never re-runs selection in any consumer, so a route that became ineligible keeps being selected and advertised |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-29 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Found while fixing bgp-adj-rib-in's validation-change keys for the RFC 8212
spec (`spec-bgp-rfc8212-ebgp-default-reject.md`, Phase 3). bgp-adj-rib-in now
emits a `ValidationChange` key that carries the native NLRI for a route of a
family keyed by its NLRI, and `validationRouteKey` resolves it to the stored
route. Every consumer still drops such a key:

- bgp-rib `rib_validation.go::validationChanged` skips a key whose `Prefix` is
  invalid.
- bgp-rr `validation.go::replayValidation` skips it unless the family is
  FlowSpec.
- bgp-rs `server_validation.go` calls `replay-path` with `Prefix.String()`.
- bgp-adj-rib-in `rib_replay_path.go` refuses every family that is not a simple
  prefix.

To be designed: each consumer resolves an NLRI-keyed key through the producer's
key form and re-runs selection for that route, and `replay-path` accepts the
native NLRI form. The JSON ingest path that keys VPN on its prefix string is
`plan/journal/declared-format-contradicts-payload.md`'s open row and is not this
spec's.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/plugin/rib-storage-design.md` - ValidationChange section, key forms
  → Constraint: to be filled at design

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4364.md` - VPN-IPv4 route identity
  → Constraint: to be filled at design

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/adj_rib_in/rib_eligibility.go` - producer of NLRI-keyed keys; read at design
- [ ] `internal/component/bgp/plugins/rib/rib_validation.go` - consumer; read at design

**Behavior to preserve:**
- Simple-prefix keys and their consumers behave as today

**Behavior to change:**
- A validation or eligibility change to an NLRI-keyed route re-runs selection in every consumer

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A validation verdict or RFC 8212 source-policy change for a VPN or labeled route in bgp-adj-rib-in

### Transformation Path
1. to be filled at design

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin ↔ Plugin | ValidationChange event, `replay-path` command | No |

### Integration Points
- to be filled at design

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | No | to be filled at design |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | to be filled at design | - | - | - | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | to be filled at design | - | - |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | to be filled at design |
| How is it reverted? | to be filled at design |
| Who else touches this path? | `spec-bgp-rfc8212-ebgp-default-reject` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Validation change of a received VPN route | → | to be filled at design | to be named at design |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A received VPN route becomes ineligible | Every consumer re-runs selection for it and its downstream advertisement follows |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Receives a VPN route whose validation state changes | to be filled at design | to be named at design |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| to be filled at design | - | AC-1 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| to be named at design | `test/` | AC-1 | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| to be named at design | `test/interop/scenarios/` | FRR | to be filled at design | |

## Files to Modify
- to be filled at design

## Files to Create
- to be filled at design

## Implementation Steps

1. to be filled at design

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
- to be filled at design

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] Current Behavior and Data Flow sections completed

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] `./le verify changed` passes on the closing commit

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] **Commit A:** code + tests + docs + edited spec
- [ ] **Commit B:** `remove plan/release/fixes/spec-bgp-validation-change-nlri-keyed-consumers.md` only

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `parser-refuses-a-form-its-producer-emits.md` | 2026-10-29 | Validation-change consumers drop NLRI-keyed keys |
