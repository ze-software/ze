# Spec: iface-route-priority -- Configurable Route Priority per Interface

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-04-13 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/iface/schema/ze-iface-conf.yang` - interface YANG schema
4. `internal/component/iface/config.go` - interface config parsing
5. `internal/component/iface/register.go` - link-state failover logic

## Task

Add a YANG leaf for configurable route priority (metric) per interface unit.
Currently, link-state failover hardcodes metric 0 (normal) and 1024
(deprioritized). This spec adds a `route-priority` leaf so operators can
express preferences like "prefer eth0 over wlan0 even when both are up"
(e.g., eth0 metric 1, wlan0 metric 5).

The deprioritized metric on link-down should be relative to the configured
base metric (configured + 1024, not a flat 1024).

### Context

Link-state failover was implemented in spec-gokrazy-4. It toggles between
metric 0 and 1024 on carrier change. Without a configurable base metric,
multi-uplink setups cannot express interface preference when all links are up.
gokrazy uses eth=1, wlan=5, down=1024 as its metric scheme.

### Scope

**In scope:**

| Area | Description |
|------|-------------|
| YANG leaf | `route-priority` under interface unit (integer, default 0) |
| Config parsing | Parse leaf into unitEntry |
| DHCP route metric | DHCP client receives configured metric, installs routes with it |
| dhcpEntry/dhcpParams | Track base metric for failover and reload change detection |
| Failover update | Deprioritized metric = configured + 1024 |
| Reload metric change | When route-priority changes on reload, old-metric route removed, new-metric route installed |

**Out of scope:**

| Area | Reason |
|------|--------|
| WiFi management | No wlan support in ze |
| Per-route metrics (non-default) | Different feature, per-route config |
| Policy routing / multiple tables | Separate concern |
| IPv6 default route metric | DHCPv6 does not install default routes in current implementation |
| ~~Static route metric~~ | No static route config exists in the iface YANG schema |

## Required Reading

### Architecture Docs
- [ ] `docs/features/interfaces.md` - interface management features
  -> Constraint: route priority is a per-unit setting, not per-interface
- [ ] `internal/component/iface/register.go` - link-state failover logic
  -> Constraint: deprioritized metric currently hardcoded as 1024

### RFC Summaries (MUST for protocol work)
- N/A - no protocol work

**Key insights:**
- Link-state failover exists but uses hardcoded metrics
- AddRoute already accepts a metric parameter
- Route priority is per-unit because DHCP is per-unit

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/iface/config.go` - parses unit config, no route-priority field
- [ ] `internal/component/iface/register.go` - handleLinkDown/handleLinkUp use deprioritizedMetric=1024
- [ ] `internal/component/iface/backend.go` - AddRoute accepts metric int
- [ ] `internal/component/iface/schema/ze-iface-conf.yang` - no route-priority leaf

**Behavior to preserve:**
- Link-state failover continues to work
- Default behavior unchanged when route-priority not configured (metric 0)

**Behavior to change:**
- unitEntry gains RoutePriority field from YANG
- dhcpParams gains RoutePriority field (triggers DHCP client restart on metric change)
- dhcpEntry gains baseMetric field (used by failover handlers)
- DHCP client receives configured metric via factory, installs routes with it
- DHCP client removes routes with configured metric on lease expiry
- Link-down deprioritization uses configured + 1024 instead of flat 1024
- Link-up restoration uses configured metric instead of 0

**Pre-requisite completed:**
- RemoveRoute now accepts metric parameter (Backend interface, all implementations, all callers)

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- Config file with `interface { ethernet { eth0 { unit 0 { route-priority 5 } } } }`

### Transformation Path
1. YANG parser extracts `route-priority` leaf value
2. `parseUnits` stores value in unitEntry.RoutePriority
3. `reconcileDHCP` copies RoutePriority into dhcpParams (change detection) and dhcpEntry.baseMetric
4. DHCP client factory receives metric, client calls AddRoute with configured metric
5. On link-down, RemoveRoute(baseMetric) then AddRoute(baseMetric + 1024)
6. On link-up, RemoveRoute(baseMetric + 1024) then AddRoute(baseMetric)
7. On DHCP lease expiry, RemoveRoute with configured metric
8. On reload with changed route-priority, reconcileDHCP detects change via dhcpParams, restarts client

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config -> iface plugin | unitEntry.RoutePriority field | [ ] |
| iface plugin -> dhcpParams | RoutePriority field in reconcileDHCP | [ ] |
| iface plugin -> dhcpEntry | baseMetric field for failover | [ ] |
| iface plugin -> DHCP client | metric parameter via factory function | [ ] |
| DHCP client -> Backend | metric parameter in AddRoute/RemoveRoute | [ ] |

### Integration Points
- `unitEntry` struct in `config.go` - new RoutePriority field
- `dhcpParams` struct in `register.go` - new RoutePriority field for change detection
- `dhcpEntry` struct in `register.go` - new baseMetric field for failover
- `dhcpClientFactory` signature in `register.go` - new metric parameter
- `DHCPClient` in `ifacedhcp/` - stores and uses configured metric
- `reconcileDHCP` in `register.go` - copies metric into params and entry
- `handleLinkDown`/`handleLinkUp` in `register.go` - uses entry.baseMetric + 1024

### Architectural Verification
- [ ] No bypassed layers (config flows through standard parsing)
- [ ] No unintended coupling (metric is a simple integer passed through)
- [ ] No duplicated functionality (extends existing AddRoute metric parameter)
- [ ] Zero-copy preserved where applicable (N/A)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config with `route-priority 5` | -> | DHCP route installed with metric 5 | To be designed |
| Link down with route-priority 5 | -> | Route metric changes to 1029 | To be designed |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config with `route-priority 5` on eth0 | DHCP default route installed with metric 5 |
| AC-2 | No route-priority configured (default) | Metric 0, same as current behavior |
| AC-3 | Link down with route-priority 5 | Route deprioritized to metric 1029 (5 + 1024) |
| AC-4 | Link up with route-priority 5 | Route restored to metric 5 |
| AC-5 | Invalid route-priority (negative or > max) | Config rejected at parse time |
| AC-6 | Reload changes route-priority from 5 to 10 | Old metric-5 route removed, new metric-10 route installed |
| AC-7 | Two interfaces with different route-priority | Each uses its own configured metric independently |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseUnitRoutePriority` | `internal/component/iface/config_test.go` | route-priority parsed into unitEntry | |
| `TestParseUnitRoutePriorityDefault` | `internal/component/iface/config_test.go` | default is 0 when not configured | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| route-priority | 0-4294966271 | 4294966271 | N/A (0 is valid) | 4294966272 |

Note: upper bound is `2^32 - 1 - 1024` (max Linux metric minus deprioritized offset).
This ensures `configured + 1024` never overflows a uint32. YANG type is uint32 with
a range constraint.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `route-priority-config` | `test/parse/route-priority.ci` | Config with route-priority parses | |

### Future (if deferring any tests)
- Integration test with real netlink route metrics (requires Linux)

## Files to Modify

- `internal/component/iface/schema/ze-iface-conf.yang` - add route-priority leaf under unit
- `internal/component/iface/config.go` - parse route-priority into unitEntry
- `internal/component/iface/register.go` - dhcpParams/dhcpEntry gain metric fields, failover uses them, factory passes metric
- `internal/plugins/ifacedhcp/dhcp_v4_linux.go` - store configured metric, use in AddRoute/RemoveRoute calls

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [x] | `ze-iface-conf.yang` |
| CLI commands/flags | [ ] | N/A |
| Editor autocomplete | [x] | YANG-driven (automatic) |
| Functional test | [x] | `test/parse/route-priority.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features/interfaces.md` |
| 2 | Config syntax changed? | [x] | `docs/guide/configuration.md` |
| 3 | CLI command added/changed? | [ ] | N/A |
| 4 | API/RPC added/changed? | [ ] | N/A |
| 5 | Plugin added/changed? | [ ] | N/A |
| 6 | Has a user guide page? | [x] | `docs/features/interfaces.md` |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [ ] | N/A |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [ ] | N/A |
| 12 | Internal architecture changed? | [ ] | N/A |

## Files to Create

- `test/parse/route-priority.ci` - functional test for config parsing

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Implementation phases below |
| 4. Full verification | `make ze-verify` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: YANG + config parsing** -- add route-priority leaf, parse into unitEntry
   - Tests: TestParseUnitRoutePriority, TestParseUnitRoutePriorityDefault
   - Files: ze-iface-conf.yang, config.go
   - Verify: tests fail -> implement -> tests pass

2. **Phase: Wiring** -- use configured metric in DHCP routes and failover
   - Tests: existing link-state tests updated for configurable metric
   - Files: register.go
   - Verify: tests fail -> implement -> tests pass

3. **Functional tests** -- config parse test
4. **Full verification** -- `make ze-verify`

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N implemented |
| Correctness | Deprioritized metric = configured + 1024, not flat 1024 |
| Naming | YANG leaf uses kebab-case `route-priority` |
| Data flow | Config -> unitEntry -> AddRoute metric parameter |
| Rule: no-layering | No duplicate metric path |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| YANG leaf exists | grep route-priority ze-iface-conf.yang |
| Config parsing works | TestParseUnitRoutePriority passes |
| Default unchanged | TestParseUnitRoutePriorityDefault passes |
| Functional test | test/parse/route-priority.ci passes |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | route-priority range enforced by YANG (0-1023) |
| Overflow | configured + 1024 cannot overflow int |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

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

(to be filled during implementation)

## RFC Documentation

N/A

## Implementation Summary

### What Was Implemented
- (to be filled)

### Bugs Found/Fixed
- (to be filled)

### Documentation Updates
- (to be filled)

### Deviations from Plan
- (to be filled)

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

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-verify` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

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

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-iface-route-priority.md`
- [ ] Summary included in commit
