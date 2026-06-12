# Spec: rs-env-promote

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | - |
| Updated | 2026-06-12 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/component/bgp/plugins/rs/server.go`

## Task

Promote `ze.rs.chan.size` to a YANG config leaf under the RS plugin's
config tree. This is an operator-facing setting (route server worker
queue depth) that should be visible in `show configuration`.

Per `ai/rules/config-surface.md`: operator-tuning settings belong in YANG.
Per `ai/rules/plugin-self-containment.md`: RS plugin owns its own YANG.

### Env var to promote

| Env var | Default | Purpose | Suggested YANG leaf |
|---------|---------|---------|---------------------|
| `ze.rs.chan.size` | 4096 | Per-source-peer worker channel capacity | `rs/worker-queue-size` |

## Required Reading

### Architecture Docs
- [ ] `ai/rules/config-surface.md` - YANG vs env var decision
- [ ] `ai/rules/config-naming.md` - naming conventions
- [ ] `ai/rules/plugin-self-containment.md` - plugin owns its own YANG

**Key insights:** (to fill during research)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/rs/server.go:38,193` - registration and usage
- [ ] `internal/component/bgp/plugins/rs/worker.go:33` - chanSize in worker config

**Behavior to preserve:**
- Default 4096 unchanged
- Env var continues to work as override

**Behavior to change:**
- New YANG leaf in RS plugin config
- Config resolution feeds value into worker pool init

## Data Flow (MANDATORY)

### Entry Point
- Config: RS plugin YANG leaf for worker queue size

### Transformation Path
1. YANG validates
2. Config tree stores value
3. RS server reads config, falls back to env var, then default

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config -> RS plugin | via plugin config | [ ] |

### Integration Points
- `internal/component/bgp/plugins/rs/server.go` - reads env var at init

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality
- [ ] Zero-copy preserved where applicable

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | RS plugin has its own YANG module or can add one | Plugin architecture | Need to check | grep yang rs/ | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | RS plugin may not have existing YANG | Missing yang/ dir | Create new YANG module per plugin pattern |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| RS config leaf for queue size | -> | `workerPoolConfig.chanSize` | TBD |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | RS worker queue size set in config | Worker pool uses configured value |
| AC-2 | No config set, no env var | Default 4096 used |
| AC-3 | YANG validation rejects invalid values | Out-of-range rejected |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRSConfigFromYANG` | `internal/component/bgp/plugins/rs/server_test.go` | Config populated from tree | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| worker-queue-size | 1-1000000 | 1000000 | 0 | 1000001 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-rs-config` | `test/decode/` | RS config with queue size parses | |

### Interop Tests (MANDATORY for protocol features)
N/A - internal tuning.

## Files to Modify
- RS plugin YANG (new or existing)
- `internal/component/bgp/plugins/rs/server.go` - read config tree

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | [x] | RS plugin YANG. Read `ai/rules/config-surface.md` (YANG vs env var) and `ai/rules/config-naming.md` (naming) |
| YANG validation constraints | [x] | range constraint |
| YANG custom validators | [ ] | N/A |
| CLI commands/flags | [ ] | N/A |
| CLI grammar (action before identifier) | [ ] | N/A |
| Editor autocomplete | [ ] | automatic |
| Functional test for new RPC/API | [x] | config parse test |
| Pipe completeness | [ ] | N/A |
| Env var registration | [ ] | Existing env var stays. Read `ai/rules/config-surface.md` before adding env-only settings |
| Doctor check for runtime dependencies | [ ] | N/A |
| Prometheus counters/metrics | [ ] | N/A |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | docs if RS config guide exists |
| 2-17 | TBD during research | [ ] | |

## Files to Create
- RS plugin YANG module (if none exists)
- Functional test

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2-14 | Standard /implement flow |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring** -- YANG leaf + config struct
2. **Phase: Config resolution** -- Wire YANG value into worker pool
3. **Functional tests**
4. **Full verification**

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation |
| Correctness | Precedence: env > YANG > default |
| YANG validation | Leaf has range constraint |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| YANG leaf in RS plugin | grep worker-queue-size |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | YANG range constraint |

### Failure Routing
| Failure | Route To |
|---------|----------|
| 3 fix attempts fail | STOP. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |

### Failed Approaches
| Approach | Why abandoned | Replacement |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
- Single env var promotion

## RFC Documentation
N/A.

## Implementation Summary

### What Was Implemented
- [to fill]

### Bugs Found/Fixed
- [to fill]

### Documentation Updates
- [to fill]

### Deviations from Plan
- [to fill]

## Implementation Audit
### Requirements from Task
| Requirement | Status | Location | Notes |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |

### Tests from TDD Plan
| Test | Status | Location | Notes |

### Files from Plan
| File | Status | Notes |

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|

## Review Gate
### Run 1 (initial)
| # | Severity | Finding | Location | Action |

### Fixes applied

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification
### Files Exist (ls)
| File | Exists | Evidence |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |

### Assumptions Resolved
| ID | Final Status | Evidence |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-3 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered
- [ ] Critical Review passes
- [ ] Risks & Assumptions resolved

### Quality Gates (SHOULD pass)
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
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests N/A

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/spec-rs-env-promote.md`
