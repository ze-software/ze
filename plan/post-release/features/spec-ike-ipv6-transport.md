# Spec: IKE over IPv6

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Priority | P3 |
| Release reason | IKEv2 runs only over IPv4: an IPv6 peer or an AAAA-only name cannot be reached, and commit accepts both. Post-release feature; the commit refusal half is the defect to settle first. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-09 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Every IKE socket Ze opens is `udp4` (`internal/component/ike/transport/udp.go`
resolves and listens with `udp4`), and the connect cycle resolves the configured
`remote-address` with `udp4` (`engine/cycle_addr.go` `resolveRemoteFn`). An IPv6
literal or a name with only AAAA records is therefore unreachable, and commit
refuses neither. This spec carries a second socket family through the transport
and the listener set (`engine/apply.go`), so a peer configured over IPv6
negotiates, and until it lands, commit refuses an IPv6 `remote-address` or
`local-address` with the fix in the message.

The split came from the closure of spec-home-cpe-4-ike-dynamic-wan, which made a
DNS-named remote work over IPv4 and recorded IPv6 as a known limitation.

## Required Reading

### Architecture Docs
- [ ] <doc> - <why>

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc7296.md`

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ike/transport/udp.go` - resolves and listens with `udp4`
- [ ] `internal/component/ike/engine/cycle_addr.go` - `resolveRemoteFn` resolves `remote-address` with `udp4`

## Existing Mechanisms (MANDATORY)

| Mechanism | Where | How this design extends or replaces it |
|-----------|-------|----------------------------------------|
| <mechanism> | <file> | <extend / replace> |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- <entry>

### Transformation Path
1. <step>

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|

### Integration Points
- <point>

### Architectural Verification
- [ ] <check>

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | <answer> |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| <entry> | → | <code> | <test> |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | <input> | <expected> |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|

## Files to Modify
- <file>

## Files to Create
- <file>

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|

## Implementation Steps

1. <phase>

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|

### Failure Routing
| Failure | Route To |
|---------|----------|

## Design Insights

<insight>

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `unwired-feature.md` | 2026-11-07 | IKE over IPv6, every socket udp4 |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
- <limitation>

## RFC Documentation (Scope: protocol)

<quotes owed>

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] `./le test ipsec` and `./le test integration interop-ipsec` pass for the IPv6 peer

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
