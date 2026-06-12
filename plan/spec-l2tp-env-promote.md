# Spec: l2tp-env-promote

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/4 |
| Updated | 2026-06-12 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/component/l2tp/yang/ze-l2tp-conf.yang`
4. `internal/component/l2tp/config.go`

## Task

Promote 5 L2TP env vars to YANG config leaves under the L2TP config tree.
These are operator-facing settings (auth timeouts, NCP toggles) that should
be visible in `show configuration`, validated, and part of commit/rollback.

Per `ai/rules/config-surface.md`: operator-tuning settings belong in YANG.

### Env vars to promote

| Env var | Default | Purpose | Suggested YANG leaf |
|---------|---------|---------|---------------------|
| `ze.l2tp.auth.timeout` | 30s | PPP auth-phase timeout | `l2tp/authentication/timeout` |
| `ze.l2tp.auth.reauth-interval` | 0s | Periodic re-auth interval | `l2tp/authentication/reauth-interval` |
| `ze.l2tp.ncp.enable-ipcp` | true | Enable IPv4 NCP | `l2tp/ncp/enable-ipcp` |
| `ze.l2tp.ncp.enable-ipv6cp` | true | Enable IPv6 NCP | `l2tp/ncp/enable-ipv6cp` |
| `ze.l2tp.ncp.ip-timeout` | 30s | NCP negotiation timeout | `l2tp/ncp/timeout` |

### Env vars staying env-only

| Env var | Reason |
|---------|--------|
| `ze.l2tp.skip-kernel-probe` | Test-only (Private) |
| `ze.l2tp.metrics.poll-interval` | Observability plumbing |

## Required Reading

### Architecture Docs
- [ ] `ai/rules/config-surface.md` - YANG vs env var decision
- [ ] `ai/rules/config-naming.md` - naming conventions
- [ ] `ai/patterns/config-option.md` - structural template

**Key insights:** (to fill during research)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/l2tp/yang/ze-l2tp-conf.yang` - existing L2TP YANG
- [ ] `internal/component/l2tp/config.go` - env var reads
- [ ] `internal/component/config/environment.go:127-128` - registrations

**Behavior to preserve:**
- All existing defaults unchanged
- Env vars continue to work as override

**Behavior to change:**
- New YANG leaves under L2TP config tree
- Config resolution feeds values into L2TP subsystem

## Data Flow (MANDATORY)

### Entry Point
- Config: `protocols l2tp authentication timeout 30s`

### Transformation Path
1. YANG validates
2. Config tree stores value
3. L2TP config loader reads tree, falls back to env var, then default

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config -> L2TP | via L2TP config struct | [ ] |

### Integration Points
- `internal/component/l2tp/config.go` - reads env vars at init

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality
- [ ] Zero-copy preserved where applicable

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | L2TP config struct exists and carries settings to subsystem | config.go | Need alternate path | grep | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Env var + YANG precedence confusion | Unexpected value | Eliminated: env vars removed, YANG is sole config surface |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `l2tp { authentication { timeout 60 } }` | -> | L2TP auth timeout = 60s | TestConfig_AuthTimeoutOverride |
| `l2tp { ncp { enable-ipcp false } }` | -> | IPCP disabled | TestConfig_NCPDisableIPCP |
| `l2tp { authentication { reauth-interval 300 } }` | -> | Reauth = 300s | TestConfig_ReauthIntervalOverride |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | L2TP auth timeout set in config | Auth subsystem uses configured value |
| AC-2 | NCP toggles set in config | IPCP/IPv6CP enabled/disabled per config |
| AC-3 | No config set, no env var | Hardcoded defaults used |
| AC-4 | YANG validation rejects invalid values | Out-of-range rejected |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestL2TPConfigFromYANG` | `internal/component/l2tp/config_test.go` | Config populated from tree | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| auth timeout | 1s-3600s | 3600s | 0s | 3601s |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-l2tp-config` | `test/decode/` | L2TP config with auth/NCP leaves parses | |

### Interop Tests (MANDATORY for protocol features)
N/A - internal config, not wire protocol.

## Files to Modify
- `internal/component/l2tp/yang/ze-l2tp-conf.yang` - add leaves
- `internal/component/l2tp/config.go` - read config tree

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | [x] | `internal/component/l2tp/yang/ze-l2tp-conf.yang`. Read `ai/rules/config-surface.md` (YANG vs env var) and `ai/rules/config-naming.md` (naming) |
| YANG validation constraints | [x] | range constraints on each leaf |
| YANG custom validators | [ ] | N/A |
| CLI commands/flags | [ ] | N/A |
| CLI grammar (action before identifier) | [ ] | N/A |
| Editor autocomplete | [ ] | automatic |
| Functional test for new RPC/API | [x] | config parse test |
| Pipe completeness | [ ] | N/A |
| Env var registration | [ ] | Existing env vars stay. Read `ai/rules/config-surface.md` before adding env-only settings |
| Doctor check for runtime dependencies | [ ] | N/A |
| Prometheus counters/metrics | [ ] | N/A |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | docs if L2TP config guide exists |
| 2-17 | TBD during research | [ ] | |

## Files to Create
- Functional test for config parsing

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2-14 | Standard /implement flow |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring** -- YANG leaves + config struct
2. **Phase: Config resolution** -- Wire YANG values into L2TP subsystem
3. **Functional tests**
4. **Full verification**

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation |
| Correctness | Precedence: env > YANG > default |
| YANG validation | Every leaf has range constraint |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| YANG leaves in ze-l2tp-conf.yang | grep authentication ze-l2tp-conf.yang |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | YANG range constraints |

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
- Scope limited to the 5 env vars listed above

## RFC Documentation
N/A - internal config.

## Implementation Summary

### What Was Implemented
- YANG `authentication` container with `timeout` (uint16, 1..3600, default 30) and `reauth-interval` (uint32, 0|5..86400, default 0)
- YANG `ncp` container with `enable-ipcp` (boolean, default true), `enable-ipv6cp` (boolean, default true), `timeout` (uint16, 1..3600, default 30)
- Parameters struct: AuthTimeout, ReauthInterval, EnableIPCP, EnableIPv6CP, NCPTimeout
- ReactorParams: same five fields, wired from subsystem.go
- ExtractParameters reads config tree containers
- reactor_kernel.go uses r.params instead of env.Get for all 5 settings
- subsystem.go uses s.params.ReauthInterval instead of env.Get
- Removed clampReauthInterval (YANG range replaces runtime clamping)
- 14 new unit tests (defaults, overrides, boundaries, gap rejection)
- 4 converted reactor tests (auth timeout, NCP toggles, NCP timeout, reauth interval)
- 1 new functional test (test/parse/l2tp-auth-ncp.ci)

### Bugs Found/Fixed
- None

### Documentation Updates
- YANG descriptions are self-documenting for CLI completion

### Deviations from Plan
- Env vars removed entirely (no backward compatibility) per user instruction, rather than preserved as overrides
- clampReauthInterval deleted; YANG range "0 | 5..86400" enforces the safety floor at parse time

## Implementation Audit
### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Promote 5 env vars to YANG | Done | ze-l2tp-conf.yang | authentication + ncp containers |
| Config resolution feeds L2TP | Done | config.go:188-221 | ExtractParameters reads containers |
| Defaults unchanged | Done | config.go:125-136 | Same 30s/0s/true/true/30s defaults |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | TestConfig_AuthTimeoutOverride | 60s config -> 60s Parameters |
| AC-2 | Done | TestConfig_NCPDisableIPCP, TestConfig_NCPDisableIPv6CP | false/true toggle works |
| AC-3 | Done | TestConfig_AuthTimeoutDefault, TestConfig_NCPDefaults | hardcoded defaults used |
| AC-4 | Done | TestConfig_AuthTimeoutZeroRejected, TestConfig_ReauthIntervalGapRejected, TestConfig_NCPTimeoutZeroRejected | YANG range rejects |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Config from YANG | Done | config_test.go | 14 new tests |
| Boundary tests | Done | config_test.go | auth timeout, reauth interval, NCP timeout |
| Functional test | Done | test/parse/l2tp-auth-ncp.ci | Config validates |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| ze-l2tp-conf.yang | Modified | +2 containers, +5 leaves |
| config.go | Modified | Parameters + ExtractParameters + constants |
| reactor.go | Modified | ReactorParams + removed clampReauthInterval |
| reactor_kernel.go | Modified | Uses r.params instead of env.Get |
| subsystem.go | Modified | Wires new ReactorParams fields |
| environment.go | Modified | Removed 2 auth env var registrations |
| config_test.go | Modified | +14 tests |
| reactor_ppp_linux_test.go | Modified | Converted 4 tests from env to params |
| reactor_kernel_linux_test.go | Modified | Updated newUnstartedReactor defaults |
| test/parse/l2tp-auth-ncp.ci | Created | Functional test |

### Audit Summary
- **Total items:** 18 (3 requirements + 4 ACs + 3 test categories + 8 files)
- **Done:** 18
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (env vars removed instead of preserved as override)

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Settings visible in show configuration | YANG leaf | 5 leaves in ze-l2tp-conf.yang authentication + ncp containers |
| Settings validated by YANG | Unit test | TestConfig_AuthTimeoutZeroRejected, TestConfig_ReauthIntervalGapRejected, TestConfig_NCPTimeoutZeroRejected |
| Settings part of commit/rollback | YANG structure | Leaves under l2tp{} container, parsed by config system |
| Config resolution feeds L2TP subsystem | Unit test | TestConfig_AuthTimeoutOverride (60s), TestL2TPReactorAuthTimeoutFromParams (45s) |
| Defaults unchanged | Unit test | TestConfig_AuthTimeoutDefault (30s), TestConfig_NCPDefaults (true/true/30s) |

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
- [ ] AC-1..AC-4 all demonstrated
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
- [ ] **Commit B:** `git rm plan/spec-l2tp-env-promote.md`
