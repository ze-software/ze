# Spec: fw-5-cli — Firewall and Traffic Control CLI Commands

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-fw-2-firewall-nft, spec-fw-3-traffic-netlink, spec-fw-4-yang-config |
| Phase | - |
| Updated | 2026-04-13 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/spec-fw-0-umbrella.md` — design decisions
3. `internal/component/iface/cmd/` — existing CLI command pattern
4. `.claude/patterns/cli-command.md` — CLI command pattern

## Task

Implement CLI commands for firewall and traffic control visibility. All commands read-only
and offline-capable (query kernel directly, no daemon required). All mutations through config.

Commands:
- `ze firewall show` — list all ze_* tables with chains, rules, sets
- `ze firewall show table <name>` — single table detail
- `ze firewall show counters` — counter values across all ze_* tables
- `ze firewall monitor` — subscribe to nftables trace events (online, requires daemon)
- `ze traffic-control show` — show qdiscs, classes, filters per configured interface
- `ze traffic-control show interface <name>` — single interface detail

## Required Reading

### Architecture Docs
- [ ] `.claude/patterns/cli-command.md` — CLI command pattern
  → Constraint: YANG schema required for CLI tree, dispatch via WireMethod
- [ ] `internal/component/iface/cmd/` — existing CLI commands for iface
  → Constraint: offline commands use local backend, online use daemon dispatch
- [ ] `internal/component/cli/model.go` — command dispatch model
  → Constraint: commands registered via constants for compiler-checked names

**Key insights:**
- Offline commands query kernel directly via backend (firewallnft, trafficnetlink)
- Online commands (monitor) go through daemon via SSH dispatch
- YANG defines the CLI tree, commands auto-complete from schema
- Output format: structured tables for show, streaming for monitor

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/iface/cmd/` — show interface, show stats commands
  → Constraint: pattern: parse args, call backend, format output
- [ ] `internal/component/iface/dispatch.go` — iface command dispatch
  → Constraint: dispatch table maps command prefixes to handlers

**Behavior to preserve:**
- No existing firewall or traffic CLI commands. Greenfield.

**Behavior to change:**
- Add `ze firewall` and `ze traffic-control` command trees

## Data Flow (MANDATORY)

### Entry Point
- User types `ze firewall show` in terminal (offline)
- User types `ze cli firewall show` via SSH (online, same handler)
- User types `ze firewall monitor` (online only)

### Transformation Path
1. CLI parser matches command to handler via YANG tree
2. Handler loads backend (firewallnft or trafficnetlink)
3. For show: backend queries kernel state, handler formats output
4. For monitor: backend subscribes to nftables events, handler streams output

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI → Handler | Command dispatch via YANG path | [ ] |
| Handler → Backend | Direct backend method calls (offline) or daemon RPC (online) | [ ] |
| Backend → Kernel | nftables ListTables/ListChains/etc or tc QdiscList/ClassList | [ ] |

### Integration Points
- `internal/component/firewall/backend.go` — Backend interface with ListTables, GetCounters (from fw-1, decision 13)
- `internal/plugins/firewallnft/read_linux.go` — ListTables, GetCounters implementations (from fw-2)
- `internal/plugins/trafficnetlink/` — ListQdiscs implementation (from fw-3)
- `internal/component/cli/` — command dispatch infrastructure

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality
- [ ] Zero-copy not applicable

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze firewall show` command | → | firewall show handler → backend query → formatted output | `test/firewall/004-cli-show.ci` |
| `ze traffic-control show` command | → | traffic show handler → backend query → formatted output | `test/traffic/002-cli-show.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze firewall show` with active tables | Output lists all ze_* tables, chains, rules |
| AC-2 | `ze firewall show table wan` | Output shows single table detail with rules and sets |
| AC-3 | `ze firewall show counters` | Output shows packet/byte counters per rule |
| AC-4 | `ze firewall show` with no ze_* tables | Output indicates no firewall tables configured |
| AC-5 | `ze firewall monitor` | Streams nftables trace events until interrupted |
| AC-6 | `ze traffic-control show` with active qdiscs | Output shows qdiscs, classes, filters per interface |
| AC-7 | `ze traffic-control show interface eth0` | Output shows single interface tc detail |
| AC-8 | `ze traffic-control show` with no tc config | Output indicates no traffic control configured |
| AC-9 | Commands available offline | `ze firewall show` works without running daemon |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestFormatFirewallTable` | `internal/component/firewall/cmd/show_test.go` | Table/chain/rule formatting |
| `TestFormatCounters` | `internal/component/firewall/cmd/show_test.go` | Counter output formatting |
| `TestFormatTrafficQdisc` | `internal/component/traffic/cmd/show_test.go` | Qdisc/class/filter formatting |
| `TestShowEmptyFirewall` | `internal/component/firewall/cmd/show_test.go` | "No tables" message |
| `TestShowEmptyTraffic` | `internal/component/traffic/cmd/show_test.go` | "No traffic control" message |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Counter packets | 0-uint64 max | uint64 max | N/A | N/A |
| Counter bytes | 0-uint64 max | uint64 max | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Firewall show | `test/firewall/004-cli-show.ci` | Config with firewall, `ze firewall show` outputs tables | |
| Traffic show | `test/traffic/002-cli-show.ci` | Config with tc, `ze traffic-control show` outputs qdiscs | |
| Firewall show empty | `test/firewall/007-cli-show-empty.ci` | No firewall config, show outputs empty message | |

### Future (if deferring any tests)
- Monitor command test deferred: requires interactive terminal simulation

## Files to Modify

No existing files modified.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | Yes | Add command tree to ze-firewall-conf.yang, ze-traffic-control-conf.yang |
| CLI commands | Yes | This spec |
| Functional test | Yes | `test/firewall/*.ci`, `test/traffic/*.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Already in fw-0 |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` — add firewall/traffic-control commands |
| 4-12 | Other categories | No | - |

## Files to Create

- `internal/component/firewall/cmd/show.go` — firewall show handlers
- `internal/component/firewall/cmd/show_test.go` — show formatting tests
- `internal/component/firewall/cmd/monitor.go` — firewall monitor handler
- `internal/component/traffic/cmd/show.go` — traffic-control show handlers
- `internal/component/traffic/cmd/show_test.go` — show formatting tests
- `test/firewall/004-cli-show.ci` — functional test
- `test/firewall/007-cli-show-empty.ci` — functional test
- `test/traffic/002-cli-show.ci` — functional test

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Create |
| 3. Implement (TDD) | Phases below |
| 4-12 | Standard flow |

### Implementation Phases

1. **Phase: Firewall show commands** — show, show table, show counters
   - Tests: TestFormatFirewallTable, TestFormatCounters, TestShowEmptyFirewall
   - Files: firewall/cmd/show.go
   - Verify: tests fail → implement → tests pass

2. **Phase: Traffic show commands** — show, show interface
   - Tests: TestFormatTrafficQdisc, TestShowEmptyTraffic
   - Files: traffic/cmd/show.go
   - Verify: tests fail → implement → tests pass

3. **Phase: Firewall monitor** — nftables trace event streaming
   - Files: firewall/cmd/monitor.go
   - Verify: manual test (requires interactive terminal)

4. **Phase: Functional tests** — .ci tests
   - Files: test/firewall/*.ci, test/traffic/*.ci
   - Verify: all pass

5. **Full verification** → `make ze-verify`

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All show commands return useful output |
| Correctness | Only ze_* tables shown (not lachesis tables) |
| Naming | Command names match `ze firewall show`, `ze traffic-control show` |
| Data flow | CLI → handler → backend query → formatted output |
| Offline | show commands work without daemon |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| firewall show handler | `grep "show" internal/component/firewall/cmd/show.go` |
| traffic show handler | `grep "show" internal/component/traffic/cmd/show.go` |
| Functional tests pass | test/firewall/004-cli-show.ci, test/traffic/002-cli-show.ci |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Output sanitization | Table/chain/rule names not used for command injection in formatted output |
| No secrets in output | Counter values, rule expressions only |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in phase |
| Output format wrong | Re-check formatting logic |
| 3 fix attempts fail | STOP. Report. Ask user. |

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
### Bugs Found/Fixed
### Documentation Updates
### Deviations from Plan

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
- [ ] AC-1..AC-9 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-test` passes
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

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-fw-5-cli.md`
- [ ] Summary included in commit
