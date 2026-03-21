# Spec: rpki-6-container-test

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | spec-rpki-5-wiring |
| Phase | - |
| Updated | 2026-03-18 |

## Task

Container-based integration test using real RPKI infrastructure. Run stayrtr (https://github.com/bgp/stayrtr) as an RTR cache server with real-world RPKI data, connect ze with RPKI config, and validate against known prefixes.

**Parent spec:** `spec-rpki-0-umbrella.md`

**Known test cases:**
- `82.212.0.0/16` -- no RPKI validation (expected: NotFound)
- Need to identify prefixes with Valid and Invalid states from live RPKI data

**Key components:**
- Container test infrastructure (Docker/Podman)
- stayrtr as RTR cache server with real ROA data
- ze connecting to stayrtr via RTR protocol
- Validation of real-world prefixes against live RPKI state
- Make target for container tests (separate from unit/functional)

## Required Reading

- [ ] `https://github.com/bgp/stayrtr` -- stayrtr RTR server
- [ ] `docs/architecture/testing/ci-format.md` -- existing test infrastructure

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] To be completed during design phase

**Behavior to preserve:**
- All existing unit and functional tests
- Existing RPKI plugin behavior from spec-rpki-5-wiring

**Behavior to change:**
- Add container test infrastructure
- Add make target for container-based RPKI integration tests

## Data Flow (MANDATORY)

### Entry Point
- Container running stayrtr with real RPKI data on TCP port
- ze connecting via RTR protocol to stayrtr

### Transformation Path
1. stayrtr fetches ROA data from RPKI repositories
2. ze connects to stayrtr, receives VRPs via RTR
3. Test harness validates known prefixes against expected states

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| stayrtr -> ze | TCP RTR protocol | [ ] |
| ze RPKI plugin -> ROA cache | VRP population | [ ] |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| stayrtr container with real data | -> | ze RTR session + validation | container test TBD |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | stayrtr running with real RPKI data, ze connects | RTR session established, VRPs populated |
| AC-2 | Prefix 82.212.0.0/16 validated | NotFound (no ROA coverage) |
| AC-3 | Known RPKI-valid prefix validated | Valid state |
| AC-4 | Known RPKI-invalid prefix validated | Invalid state |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates |
|------|------|-----------|
| TBD during design | TBD | TBD |

### Functional Tests

| Test | File | Validates |
|------|------|-----------|
| Container RPKI integration | TBD | End-to-end with real RPKI data |

## Files to Create

- TBD during design phase

## Files to Modify

- TBD during design phase

## Implementation Steps

1. Design container test infrastructure
2. Write Dockerfile / compose for stayrtr
3. Write test harness
4. Add make target

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

## Implementation Summary

### What Was Implemented
- [pending]

### Bugs Found/Fixed
- [pending]

### Documentation Updates
- [pending]

### Deviations from Plan
- [pending]
