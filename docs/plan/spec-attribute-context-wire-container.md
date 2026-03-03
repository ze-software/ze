# Spec: Route Reflection via API — Overview

**Status:** Phase 1 (PackWithContext) complete. Phase 2 split into sub-specs.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file — overview only, implementation in sub-specs
2. Sub-specs: `spec-attributes-wire.md`, `spec-route-id-forwarding.md`, `spec-api-attribute-filter.md`, `spec-rfc9234-role.md`

## Task

Overview spec for route reflection via the API (not internally). Phase 1 (PackWithContext) is complete. Phase 2 is split into four self-contained sub-specs. This document is an index — see sub-specs for implementation details.

## Architecture

ZeBGP implements route reflection through the API, not internally:

| Step | Component | Action |
|------|-----------|--------|
| 1 | Peer A → Engine | Receive UPDATE |
| 2 | Engine | Store (wire + route-id + role) |
| 3 | Engine → API | Output route event |
| 4 | External process | Decides forwarding |
| 5 | API → Engine | Command: `peer !<source> forward route-id 123` |
| 6 | Engine → Peer B,C | Zero-copy forward via route-id lookup |

## Self-Contained Specs

| Spec | Description | Dependencies |
|------|-------------|--------------|
| `spec-attributes-wire.md` | AttributesWire type — wire-canonical storage with lazy parsing | None |
| `spec-route-id-forwarding.md` | Route ID, forward command, `!<ip>` selector | spec-attributes-wire |
| `spec-api-attribute-filter.md` | `attributes <list>` config for partial parsing | spec-attributes-wire |
| `spec-rfc9234-role.md` | RFC 9234 Role capability for API policy | None |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/encoding-context.md` — PackContext, ContextID, zero-copy
  → Constraint: Zero-copy forwarding requires same encoding context
- [ ] `docs/architecture/update-building.md` — wire format patterns
  → Decision: Wire-canonical storage is foundation

### RFC Summaries
- [ ] `rfc/short/rfc4271.md` — BGP-4 UPDATE structure
- [ ] `rfc/short/rfc9234.md` — Role capability
  → Constraint: Role values and OTC attribute rules

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] See individual sub-specs for source file analysis

**Behavior to preserve:**
- See individual sub-specs

**Behavior to change:**
- See individual sub-specs

## Data Flow (MANDATORY)

### Entry Point
- Wire bytes from BGP peer (UPDATE message)

### Transformation Path
1. Wire parsing → WireUpdate with lazy attribute iterators
2. Store wire bytes + route-id (new) + source peer role (new)
3. API output with route-id and optional attribute filtering
4. External process decides forwarding via API commands
5. `forward route-id N` → zero-copy forward from wire cache

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Wire → Storage | AttributesWire (lazy parsed, wire-canonical) | [ ] |
| Storage → API | JSON with route-id, optional attribute filter | [ ] |
| API → Engine | Text command: `peer !<ip> forward route-id N` | [ ] |

### Integration Points
- AttributesWire replaces current attribute storage in UPDATE cache
- Route-id adds lookup key to cached UPDATEs
- Role tagging integrates with bgp-role plugin

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality
- [ ] Zero-copy preserved where applicable

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|

## Key Concepts

### Wire-Canonical Storage
Routes store wire bytes as canonical form. Parsing is lazy (on demand). Methods: `Get(code)` for single attribute, `GetMultiple(codes)` for subset, `Packed()` for zero-copy forward.

### Route ID
Each received route gets unique ID for API reference. Forwarding by ID avoids re-serialization.

### RFC 9234 Role
Routes tagged with source peer's role for policy. Policy without attribute parsing: customer routes can go anywhere, provider/peer routes to customers only.

### Attribute Filtering
API config limits which attributes are parsed/output. Reduces overhead for API consumers that only need subset.

## Implementation Order

| Step | Spec | Dependencies |
|------|------|--------------|
| 1 | `spec-attributes-wire.md` | Foundation — lazy parsing type |
| 2a | `spec-route-id-forwarding.md` | Uses AttributesWire.PackFor |
| 2b | `spec-api-attribute-filter.md` | Uses AttributesWire.GetMultiple |
| 3 | `spec-rfc9234-role.md` | Independent, can be done anytime |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|

## Summary Table

| Feature | Spec | Status |
|---------|------|--------|
| AttributesWire type | spec-attributes-wire.md | Ready |
| Route ID field | spec-route-id-forwarding.md | Ready |
| `forward route-id` command | spec-route-id-forwarding.md | Ready |
| `!<ip>` selector | spec-route-id-forwarding.md | Ready |
| `attributes <list>` config | spec-api-attribute-filter.md | Ready |
| RFC 9234 Role capability | spec-rfc9234-role.md | Ready |
| OTC attribute | spec-rfc9234-role.md | Ready |
| `peer [role X]` selector | spec-rfc9234-role.md | Ready |

## Files to Modify

See individual sub-specs for detailed file lists.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| See individual sub-specs | | |

## Implementation Steps

See individual sub-specs for implementation steps. This overview tracks ordering only.

1. Implement `spec-attributes-wire.md` — foundation
2. Implement `spec-route-id-forwarding.md` and `spec-api-attribute-filter.md` — parallel
3. Implement `spec-rfc9234-role.md` — independent

### Failure Routing

| Failure | Route To |
|---------|----------|

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Design Insights

- Wire-canonical storage avoids re-serialization for forwarding — zero-copy preserved
- Lazy parsing means API overhead is proportional to attributes requested, not total
- Route-id decouples forwarding from attribute parsing — forward by reference
- Role tagging enables policy without parsing — cheaper than community-based

## Implementation Summary

### What Was Implemented
- Phase 1: PackWithContext — complete and merged
- Phase 2: Pending (split into 4 sub-specs)

### Documentation Updates
- (pending)

### Deviations from Plan
- (pending)

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Checklist

### Goal Gates (MUST pass)
- [ ] AC defined and demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-unit-test` passes
- [ ] `make ze-functional-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] `make ze-lint` passes
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `docs/learned/`
- [ ] Summary included in commit
