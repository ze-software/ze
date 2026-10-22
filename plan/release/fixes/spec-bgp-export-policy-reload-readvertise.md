# Spec: bgp-export-policy-reload-readvertise

| Field | Value |
|-------|-------|
| Status | skeleton |
| Priority | P1 |
| Release reason | Committed export policy must reconcile previously advertised routes, not only future updates. |
| Scope | protocol |
| Depends | `plan/release/fixes/spec-plugin-reload-applies-or-refuses.md` (the egress filter plugins gain a live apply there) |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-14 |

Bucket: immediate. An operator who edits an egress rule and reloads sees the
new rule only on routes sent after the reload: every route already advertised
keeps the attributes, or the acceptance, of the old rule until its next UPDATE.

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Created by design review round 1 of `spec-plugin-reload-applies-or-refuses`
(finding I4), 2026-10-14, as the named owner of export re-advertisement. That
spec makes an egress filter or policy edit take effect on reload for
subsequent UPDATEs only: `filter_community` tag, strip and scrub,
`filter_modify`, `filter_remove_private_as`, role `export`, and every egress
filter definition. Nothing re-sends the routes already advertised to a peer,
so the peer holds state the running config no longer produces.

To be designed: on a committed reload that changes an egress rule, re-run the
egress chain over the routes advertised to each affected peer and send the
UPDATE or withdrawal the difference requires (the effect an operator expects
from outbound soft reconfiguration). Import re-evaluation is not this spec's:
it is `plan/post-release/features/spec-bgp-filtered-route-storage.md` AC-3.

## Required Reading

### Architecture Docs
- [ ] `docs/guide/config-reload.md` - what a reload does today
  → Constraint: to be filled at design
- [ ] `plan/release/fixes/spec-plugin-reload-applies-or-refuses.md` "Boundary with stored routes"
  → Decision: that spec applies egress edits to subsequent UPDATEs only and names this spec as the owner of re-advertisement

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4271.md` - UPDATE and Adj-RIB-Out
  → Constraint: to be filled at design

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/filter_community/filter_community.go` - egress tag/strip/scrub; read at design
- [ ] `internal/component/bgp/plugins/filter_modify/filter_modify.go` - egress attribute modifiers; read at design

**Behavior to preserve:**
- to be filled at design

**Behavior to change:**
- Routes already advertised are re-evaluated under a changed egress rule

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- SIGHUP or CLI commit that changes an egress rule

### Transformation Path
1. to be filled at design

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ Plugin | to be filled at design | No |

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
| Who else touches this path? | `spec-plugin-reload-applies-or-refuses`, `spec-bgp-withdraw-only-exported-routes` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| SIGHUP after an egress community tag edit | → | to be filled at design | `test/reload/bgp-filter-export-reload-readvertise.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A committed reload changes an egress rule for a peer with routes already advertised | The peer receives the UPDATEs and withdrawals that make its view match the new rule |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Edits a peer's export community tag and reloads | to be filled at design | `test/reload/bgp-filter-export-reload-readvertise.ci` |

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
| `bgp-filter-export-reload-readvertise` | `test/reload/` | AC-1 | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| to be named at design | `test/interop/scenarios/` | FRR | the peer's received attributes follow the reload | |

## Files to Modify
- to be filled at design

## Files to Create
- `test/reload/bgp-filter-export-reload-readvertise.ci`

## Implementation Steps

1. to be filled at design

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
- to be filled at design
- Test-tooling need (recorded 2026-10-15 from the review of `test/reload/bgp-filter-export-reload-next-update-only.ci`): that test catches a re-send of U1 only when it reaches B before its end barrier U3. ze-peer checks only `reject=` rules after its last expectation, and a `reject=` pattern matches frame bytes, optionally scoped to the announced or withdrawn NLRI (`internal/test/peer/reject.go`, `ParseRejectRule`), with no `attribute=` qualifier, so no rule can refuse "NLRI 10.0.0.0/24 carrying COMMUNITY 65000:2" without also refusing the legitimate U1 at seq=2. When this spec flips that test, its re-advertisement assertion needs either an NLRI-plus-attribute reject qualifier or a reject that arms only after a given expectation; until then a late re-send is not caught.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] Current Behavior and Data Flow sections completed

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] `./le verify worktree` passes

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] **Commit A:** code + tests + docs + edited spec
- [ ] **Commit B:** `remove plan/release/fixes/spec-bgp-export-policy-reload-readvertise.md` only
