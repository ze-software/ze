# Spec: iface-1 — Interface Monitor Plugin

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-iface-0 |
| Phase | - |
| Updated | 2026-03-25 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/spec-iface-0-umbrella.md` — shared topics, payloads, design insights
3. `pkg/ze/bus.go` — Bus interface
4. `internal/bus/bus.go` — Bus implementation
5. `internal/component/plugin/registry/registry.go` — plugin registration

## Task

Create the interface monitoring plugin (`iface`). Linux-only for now. The plugin opens a netlink multicast socket, receives kernel events for link state and address changes, classifies them into Bus topics, encodes payloads as JSON, and publishes to the Bus. This is the **read-only** foundation — monitoring only, no interface management.

## Required Reading

### Architecture Docs
- [ ] `plan/spec-iface-0-umbrella.md` — Bus topics, payload format, OS-level operations
  → Decision: hierarchical topics under `interface/`, JSON payloads, kebab-case
  → Constraint: payload is `[]byte`, bus never type-asserts
- [ ] `.claude/rules/plugin-design.md` — registration, 5-stage protocol
  → Constraint: `init()` in `register.go`, blank import in `all/all.go`
- [ ] `plan/learned/423-reactor-bus-subscribe.md` — reactor Bus subscription pattern
  → Decision: handlers registered before Start, prefix-based matching

### RFC Summaries (MUST for protocol work)
- [ ] N/A — no BGP protocol work in this phase

**Key insights:**
- Netlink multicast groups: `RTMGRP_LINK`, `RTMGRP_IPV4_IFADDR`, `RTMGRP_IPV6_IFADDR`
- `vishvananda/netlink` provides `LinkSubscribe` and `AddrSubscribe` for async monitoring
- IPv6 DAD must complete before publishing `addr/added` — check `IFA_F_TENTATIVE` flag

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `pkg/ze/bus.go` — Bus interface with Publish/Subscribe/Consumer
- [ ] `internal/bus/bus.go` — Implementation: prefix matching, per-consumer delivery goroutine
- [ ] `internal/component/plugin/registry/registry.go` — `Register()` function, `Registration` struct
- [ ] `internal/component/plugin/all/all.go` — blank imports triggering all `init()`

**Behavior to preserve:**
- Bus content-agnostic — payload is `[]byte`
- Plugin registration via `init()` + blank import
- No existing interface monitoring to preserve

**Behavior to change:**
- No interface events on Bus today — this spec adds them

## Data Flow (MANDATORY)

### Entry Point
- Kernel netlink multicast messages arrive on subscribed socket
- Format: raw netlink messages (`RTM_NEWLINK`, `RTM_NEWADDR`, etc.)

### Transformation Path
1. **Receive** — `vishvananda/netlink` delivers `LinkUpdate` or `AddrUpdate` structs
2. **Classify** — map update type to Bus topic string (see umbrella topic table)
3. **Filter** — skip tentative IPv6 (DAD incomplete), skip loopback unless configured
4. **Encode** — serialize to JSON `[]byte` with kebab-case keys
5. **Publish** — `bus.Publish(topic, payload, metadata)`

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| OS kernel ↔ Plugin | Netlink socket via `vishvananda/netlink` | [ ] |
| Plugin ↔ Bus | `bus.Publish(topic, []byte, metadata)` | [ ] |

### Integration Points
- `internal/component/plugin/registry/` — plugin registers here
- `pkg/ze/bus.go` — `Bus.Publish`, `Bus.Subscribe`
- `internal/component/plugin/all/all.go` — blank import added

### Architectural Verification
- [ ] No bypassed layers (plugin → Bus only)
- [ ] No unintended coupling (no BGP imports)
- [ ] No duplicated functionality (new capability)
- [ ] Zero-copy preserved (payload is `[]byte`)

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Plugin registered via `init()` | → | Registry contains `"iface"` plugin | `TestIfacePluginRegistered` |
| Netlink addr event | → | Monitor publishes to Bus | `TestIfaceMonitorPublishesAddrAdded` |
| Plugin lifecycle start/stop | → | Netlink socket opened/closed | `test/plugin/iface-monitor.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Interface plugin starts on Linux | Opens netlink socket, subscribes to `RTMGRP_LINK` + `RTMGRP_IPV4_IFADDR` + `RTMGRP_IPV6_IFADDR`, begins monitoring |
| AC-2 | External IP added to OS interface | Plugin publishes `interface/addr/added` to Bus within 1 second |
| AC-13 | Interface plugin stops | Closes netlink socket cleanly |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBusTopicCreation` | `internal/plugins/iface/iface_test.go` | Plugin creates correct Bus topics on start | |
| `TestNetlinkEventToTopic` | `internal/plugins/iface/monitor_linux_test.go` | Maps netlink message types to correct Bus topics | |
| `TestPayloadFormat` | `internal/plugins/iface/iface_test.go` | JSON payload matches spec (kebab-case, correct fields) | |
| `TestIfacePluginRegistered` | `internal/plugins/iface/iface_test.go` | Plugin found in registry after init() | |
| `TestMonitorStartStop` | `internal/plugins/iface/monitor_linux_test.go` | Monitor goroutine starts and stops cleanly | |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Interface name | 1-15 chars (IFNAMSIZ-1) | 15 chars | empty | 16 chars |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-iface-monitor` | `test/plugin/iface-monitor.ci` | Interface plugin starts, monitors, stops cleanly | |

### Future (if deferring any tests)
- Netlink throughput benchmark — not needed for correctness
- Rapid interface flapping test — defer to chaos framework

## Files to Modify

- `internal/component/plugin/all/all.go` — blank import for `iface` plugin (auto-generated by `make generate`)
- `go.mod` — add `github.com/vishvananda/netlink`

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [ ] | N/A — monitoring only, no config |
| CLI commands/flags | [ ] | N/A — monitoring only |
| Editor autocomplete | [ ] | N/A |
| Functional test for new RPC/API | [x] | `test/plugin/iface-monitor.ci` |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | — (monitoring is internal, user-facing comes in Phase 2) |
| 2 | Config syntax changed? | No | — |
| 3 | CLI command added/changed? | No | — |
| 4 | API/RPC added/changed? | No | — |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` — iface plugin |
| 6 | Has a user guide page? | No | — (deferred to Phase 2) |
| 7 | Wire format changed? | No | — |
| 8 | Plugin SDK/protocol changed? | No | — |
| 9 | RFC behavior implemented? | No | — |
| 10 | Test infrastructure changed? | No | — |
| 11 | Affects daemon comparison? | No | — (not user-visible yet) |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` — interface plugin |

## Files to Create

- `internal/plugins/iface/iface.go` — Shared types, Bus topic constants, payload encoding
- `internal/plugins/iface/register.go` — Plugin registration (`init()` → `registry.Register()`)
- `internal/plugins/iface/monitor_linux.go` — Netlink multicast monitor goroutine
- `internal/plugins/iface/iface_test.go` — Unit tests for topics, payload format, registration
- `internal/plugins/iface/monitor_linux_test.go` — Unit tests for netlink event mapping
- `test/plugin/iface-monitor.ci` — Functional test

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

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Plugin skeleton** — registration, shared types, topic constants
   - Tests: `TestIfacePluginRegistered`, `TestBusTopicCreation`
   - Files: `iface.go`, `register.go`
   - Verify: tests fail → implement → tests pass
2. **Phase: Payload encoding** — JSON encoding for all event types
   - Tests: `TestPayloadFormat`
   - Files: `iface.go`
   - Verify: tests fail → implement → tests pass
3. **Phase: Netlink monitor** — subscribe to multicast, classify events, publish
   - Tests: `TestNetlinkEventToTopic`, `TestMonitorStartStop`
   - Files: `monitor_linux.go`
   - Verify: tests fail → implement → tests pass
4. **Functional test** — `test/plugin/iface-monitor.ci`
5. **Full verification** → `make ze-verify`
6. **Complete spec** → Fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1, AC-2, AC-13 all have implementation with file:line |
| Correctness | JSON keys are kebab-case, IPv6 DAD filtering works |
| Naming | Topic strings match umbrella table exactly |
| Data flow | Netlink → classify → encode → Bus.Publish, no shortcuts |
| Rule: goroutine-lifecycle | Monitor is long-lived worker, not per-event goroutine |
| Rule: plugin-design | Registration via `init()`, no direct imports from other plugins |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| `internal/plugins/iface/iface.go` exists | `ls -la internal/plugins/iface/iface.go` |
| `internal/plugins/iface/register.go` exists | `ls -la internal/plugins/iface/register.go` |
| `internal/plugins/iface/monitor_linux.go` exists | `ls -la internal/plugins/iface/monitor_linux.go` |
| Plugin in registry | `grep -r '"iface"' internal/plugins/iface/register.go` |
| `test/plugin/iface-monitor.ci` exists | `ls -la test/plugin/iface-monitor.ci` |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | Netlink messages may be malformed — validate before encoding |
| Resource exhaustion | Monitor goroutine must not leak on rapid events — bounded channel |
| Privilege | Netlink multicast requires no special privilege for monitoring |

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
- [ ] AC-1, AC-2, AC-13 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`)
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
- [ ] Write learned summary to `plan/learned/NNN-iface-1-monitor.md`
- [ ] **Summary included in commit** — NEVER commit implementation without the completed summary. One commit = code + tests + summary.
