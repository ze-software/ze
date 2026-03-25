# Spec: iface-2 — Interface Management + YANG + CLI

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-iface-1 |
| Phase | - |
| Updated | 2026-03-25 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/spec-iface-0-umbrella.md` — shared topics, payloads, YANG hierarchy, CLI design
3. `internal/plugins/iface/iface.go` — shared types from Phase 1
4. `internal/component/config/` — config pipeline
5. `cmd/ze/` — CLI patterns

## Task

Add interface management capability to the `iface` plugin (from Phase 1): create, delete, configure OS interfaces via netlink. Add YANG configuration schema (`ze-iface-conf`) and CLI commands (`ze interface`). VyOS-aligned hierarchy with type-first grouping.

## Required Reading

### Architecture Docs
- [ ] `plan/spec-iface-0-umbrella.md` — YANG hierarchy, CLI design, OS operations
  → Decision: VyOS-aligned type-first grouping (ethernet, dummy, veth, bridge)
  → Decision: `interface-common` YANG grouping shared across types
  → Constraint: naming convention — interface names prefixed by type
- [ ] `.claude/rules/cli-patterns.md` — CLI dispatch, flags, exit codes
  → Constraint: `cmd/ze/<domain>/main.go` with `func Run(args []string) int`
- [ ] `.claude/rules/config-design.md` — no version numbers, fail on unknown keys
  → Constraint: fail on unknown keys at any level
- [ ] `internal/component/config/` — config pipeline
  → Constraint: File → Tree → ResolveBGPTree() → `map[string]any`

### RFC Summaries (MUST for protocol work)
- [ ] N/A — no BGP protocol work in this phase

**Key insights:**
- VyOS uses type-first grouping: `set interfaces <type> <name>`
- YANG `grouping`/`uses` for shared fields (address, mtu, description, disable, vrf)
- Linux IFNAMSIZ = 16 (15 chars + null) — interface name limit
- sysctl for IPv4/IPv6 per-interface options

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/iface/iface.go` — shared types from Phase 1
- [ ] `internal/plugins/iface/register.go` — plugin registration
- [ ] `internal/component/config/resolve.go` — config resolution pipeline
- [ ] `cmd/ze/bgp/main.go` — CLI pattern reference
- [ ] `internal/component/bgp/schema/ze-bgp-conf.yang` — YANG pattern reference

**Behavior to preserve:**
- Phase 1 monitoring continues to work
- Config pipeline patterns (File → Tree → Resolve)
- CLI patterns (flag.NewFlagSet, exit codes, stderr for errors)

**Behavior to change:**
- No interface management exists — this spec adds it
- No `ze interface` CLI command exists
- No `ze-iface-conf` YANG module exists

## Data Flow (MANDATORY)

### Entry Point
- Config file with `interface` stanza → config pipeline
- CLI: `ze interface create/delete/addr` → command dispatch

### Transformation Path
1. **Config parse** — `ze-iface-conf.yang` validates config tree
2. **Config resolve** — interface stanzas extracted from `map[string]any`
3. **Plugin configure** — 5-stage protocol delivers config to plugin
4. **Netlink execute** — plugin creates/deletes interfaces, assigns addresses
5. **Monitor detects** — Phase 1 monitor publishes Bus events for changes

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Config ↔ Plugin | 5-stage protocol, config delivery | [ ] |
| Plugin ↔ OS | Netlink `RTM_NEWLINK`/`RTM_NEWADDR` | [ ] |
| CLI ↔ Plugin | Command dispatch via registry | [ ] |

### Integration Points
- `internal/component/config/` — config resolution for `interface` stanza
- `internal/component/plugin/registry/` — CLI handler registration
- Phase 1 monitor — detects changes made by management operations

### Architectural Verification
- [ ] No bypassed layers (config → plugin → netlink → monitor → Bus)
- [ ] No unintended coupling (management uses netlink, monitor detects results)
- [ ] No duplicated functionality (builds on Phase 1 plugin)
- [ ] Zero-copy preserved where applicable

## YANG Configuration (from umbrella)

### Hierarchy (VyOS-aligned)

| YANG Path | Node Type | Description |
|-----------|-----------|-------------|
| `interface` | container | Top-level interface container |
| `interface/ethernet` | list (key: `name`) | Physical ethernet (configure only) |
| `interface/dummy` | list (key: `name`) | Dummy/loopback-like interfaces |
| `interface/veth` | list (key: `name`) | Virtual ethernet pairs |
| `interface/bridge` | list (key: `name`) | Bridge interfaces |
| `interface/loopback` | container | Loopback (singleton) |
| `interface/monitor` | container | OS monitoring settings |

### Shared Grouping: `interface-common`

| Field | Type | Default | Constraint |
|-------|------|---------|------------|
| `address` | leaf-list string | — | CIDR format |
| `description` | leaf string | — | max 255 chars |
| `mtu` | leaf uint16 | 1500 | 68-16000 |
| `disable` | leaf empty | — | present = disabled |
| `vrf` | leaf string | — | must reference existing VRF |

### IPv4/IPv6 Options (sysctl)

| YANG Path | Description | sysctl |
|-----------|-------------|--------|
| `ipv4/forwarding` | IPv4 forwarding | `net.ipv4.conf.<iface>.forwarding` |
| `ipv4/arp-filter` | ARP filtering | `net.ipv4.conf.<iface>.arp_filter` |
| `ipv4/arp-accept` | Gratuitous ARP | `net.ipv4.conf.<iface>.arp_accept` |
| `ipv6/autoconf` | SLAAC | `net.ipv6.conf.<iface>.autoconf` |
| `ipv6/accept-ra` | Accept RAs | `net.ipv6.conf.<iface>.accept_ra` |
| `ipv6/forwarding` | IPv6 forwarding | `net.ipv6.conf.<iface>.forwarding` |

Note: when `forwarding=true`, `accept_ra` must be `2` (not `1`) to still accept RAs.

## CLI Design (from umbrella)

| Command | Description |
|---------|-------------|
| `ze interface show` | List all interfaces with state + addresses |
| `ze interface show <name>` | Detail for one interface |
| `ze interface show --json` | JSON output |
| `ze interface create dummy <name>` | Create dummy interface |
| `ze interface create veth <name> <peer>` | Create veth pair |
| `ze interface delete <name>` | Delete Ze-managed interface |
| `ze interface addr add <name> <addr/prefix>` | Add IP address |
| `ze interface addr del <name> <addr/prefix>` | Remove IP address |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Config with `interface` section | → | Plugin creates OS interface | `TestIfacePluginCreatesInterface` |
| CLI `ze interface show` | → | Lists interfaces | `test/plugin/iface-create.ci` |
| YANG validation | → | Invalid config rejected | `test/parse/iface-invalid.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-3 | Config specifies managed interface | Plugin creates interface via netlink `RTM_NEWLINK`, brings it up, assigns configured addresses |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestIfaceCreate` | `internal/plugins/iface/iface_linux_test.go` | Creates dummy interface via netlink | |
| `TestIfaceDelete` | `internal/plugins/iface/iface_linux_test.go` | Deletes Ze-managed interface | |
| `TestIfaceAddrAdd` | `internal/plugins/iface/iface_linux_test.go` | Adds IPv4 and IPv6 addresses | |
| `TestIfaceAddrDel` | `internal/plugins/iface/iface_linux_test.go` | Removes addresses | |
| `TestSysctlAutoconf` | `internal/plugins/iface/sysctl_linux_test.go` | Writes correct sysctl values | |
| `TestSysctlForwardingAcceptRA` | `internal/plugins/iface/sysctl_linux_test.go` | forwarding=true sets accept_ra=2 | |
| `TestCLIInterfaceShow` | `cmd/ze/interface/main_test.go` | CLI output format | |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| MTU | 68-16000 | 16000 | 67 | 16001 |
| VLAN ID | 1-4094 | 4094 | 0 | 4095 |
| Interface name | 1-15 chars | 15 chars | empty | 16 chars |
| Description | 0-255 chars | 255 chars | N/A | 256 chars |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-iface-create` | `test/plugin/iface-create.ci` | Config with interface section creates dummy interface | |
| `test-iface-invalid` | `test/parse/iface-invalid.ci` | Invalid interface config rejected with error | |

### Future (if deferring any tests)
- macOS `_darwin.go` implementation — future spec

## Files to Modify

- `internal/plugins/iface/register.go` — add `ConfigRoots: ["interface"]`, YANG schema
- `cmd/ze/bgp/main.go` — reference for CLI pattern (not modified)

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new module) | [x] | `internal/plugins/iface/schema/ze-iface-conf.yang` |
| CLI commands/flags | [x] | `cmd/ze/interface/main.go` |
| CLI usage/help text | [x] | Same |
| Editor autocomplete | [x] | YANG-driven |
| Functional test | [x] | `test/plugin/iface-create.ci` |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` — interface management |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` — interface stanzas |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` — `ze interface` |
| 4 | API/RPC added/changed? | No | — |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` — iface plugin config |
| 6 | Has a user guide page? | Yes | `docs/guide/interfaces.md` — new |
| 7 | Wire format changed? | No | — |
| 8 | Plugin SDK/protocol changed? | No | — |
| 9 | RFC behavior implemented? | No | — |
| 10 | Test infrastructure changed? | No | — |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` — interface management |
| 12 | Internal architecture changed? | No | — |

## Files to Create

- `internal/plugins/iface/iface_linux.go` — Interface create/delete/addr management
- `internal/plugins/iface/sysctl_linux.go` — sysctl writes for IPv4/IPv6 options
- `internal/plugins/iface/schema/ze-iface-conf.yang` — YANG config schema
- `cmd/ze/interface/main.go` — CLI subcommand dispatch
- `cmd/ze/interface/show.go` — `ze interface show`
- `cmd/ze/interface/create.go` — `ze interface create`
- `cmd/ze/interface/addr.go` — `ze interface addr add/del`
- `internal/plugins/iface/iface_linux_test.go` — Management unit tests
- `internal/plugins/iface/sysctl_linux_test.go` — sysctl unit tests
- `test/plugin/iface-create.ci` — Functional test
- `test/parse/iface-invalid.ci` — Config validation test

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + umbrella |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Implementation phases below |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report |

### Implementation Phases

1. **Phase: YANG schema** — `ze-iface-conf.yang` with VyOS-aligned hierarchy
   - Tests: config parse tests
   - Files: `schema/ze-iface-conf.yang`
   - Verify: YANG validates
2. **Phase: Interface management** — create/delete/addr via netlink
   - Tests: `TestIfaceCreate`, `TestIfaceDelete`, `TestIfaceAddrAdd`, `TestIfaceAddrDel`
   - Files: `iface_linux.go`
   - Verify: tests fail → implement → tests pass
3. **Phase: sysctl** — IPv4/IPv6 per-interface options
   - Tests: `TestSysctlAutoconf`, `TestSysctlForwardingAcceptRA`
   - Files: `sysctl_linux.go`
   - Verify: tests fail → implement → tests pass
4. **Phase: CLI** — `ze interface show/create/delete/addr`
   - Tests: `TestCLIInterfaceShow`
   - Files: `cmd/ze/interface/*.go`
   - Verify: tests fail → implement → tests pass
5. **Functional tests** → `test/plugin/iface-create.ci`, `test/parse/iface-invalid.ci`
6. **Full verification** → `make ze-verify`
7. **Complete spec** → Fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-3 has implementation with file:line |
| Correctness | Netlink operations match umbrella OS operations table |
| Naming | YANG uses kebab-case, CLI follows `rules/cli-patterns.md` |
| Data flow | Config → plugin → netlink → monitor detects → Bus event |
| Rule: config-design | Fail on unknown keys, no version numbers |
| Rule: cli-patterns | flag.NewFlagSet, exit codes, stderr for errors |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| `internal/plugins/iface/iface_linux.go` exists | `ls -la` |
| `internal/plugins/iface/sysctl_linux.go` exists | `ls -la` |
| `internal/plugins/iface/schema/ze-iface-conf.yang` exists | `ls -la` |
| `cmd/ze/interface/main.go` exists | `ls -la` |
| `test/plugin/iface-create.ci` exists | `ls -la` |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | Interface names: 1-15 chars, valid chars only. CIDR addresses validated. |
| Privilege | Interface creation requires `CAP_NET_ADMIN` — document requirement |
| Command injection | No shell commands — all via netlink syscalls |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior → RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural → DESIGN phase |
| Functional test fails | Check AC; if AC wrong → DESIGN; if AC correct → IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
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

## RFC Documentation

N/A — no BGP protocol work in this phase.

## Implementation Summary

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered]

### Documentation Updates
- [Docs updated, or "None"]

### Deviations from Plan
- [Differences from plan and why]

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
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

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
- [ ] AC-3 demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` — no failures)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-iface-2-manage.md`
- [ ] **Summary included in commit** — NEVER commit implementation without the completed summary. One commit = code + tests + summary.
