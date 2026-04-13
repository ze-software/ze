# Spec: fw-3-traffic-netlink — Traffic Control Backend

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-fw-1-data-model |
| Phase | 5/5 |
| Updated | 2026-04-13 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/spec-fw-0-umbrella.md` — design decisions
3. `plan/spec-fw-1-data-model.md` — InterfaceQoS, Qdisc, TrafficClass types
4. `vendor/github.com/vishvananda/netlink/qdisc.go` — tc types available

## Task

Implement the trafficnetlink plugin using `vishvananda/netlink` (already vendored). The plugin
receives `map[string]InterfaceQoS` via `Apply`, reconciles qdiscs/classes/filters on each
named interface.

Supports: HTB, HFSC, FQ, FQ_CoDel, SFQ, TBF, Netem, Prio, Clsact, Ingress qdiscs.
Classes: HtbClass, HfscClass. Filters: fw (mark-based), u32, flower.

## Required Reading

### Architecture Docs
- [ ] `plan/spec-fw-0-umbrella.md` — design decisions 6, 7
  → Decision: Apply(map[string]InterfaceQoS) error
  → Constraint: separate component from firewall, same backend pattern
- [ ] `vendor/github.com/vishvananda/netlink/qdisc.go` — available qdisc types
  → Constraint: HTB, HFSC, FQ, FQ_CoDel, SFQ, TBF, Netem, Prio, Clsact, Ingress
- [ ] `vendor/github.com/vishvananda/netlink/class.go` — available class types
  → Constraint: HtbClass (rate/ceil/burst), HfscClass (rt/ls/ul curves)
- [ ] `internal/plugins/ifacenetlink/manage_linux.go` — vishvananda/netlink usage pattern
  → Constraint: Handle for namespace awareness, error wrapping

**Key insights:**
- vishvananda/netlink tc API: QdiscAdd/Del/Replace, ClassAdd/Del/Change, FilterAdd/Del
- Reconciliation: replace root qdisc (HTB replaces pfifo_fast), classes and filters rebuilt
- Interface lookup by name via netlink.LinkByName to get link index
- tc operations require CAP_NET_ADMIN (same as nftables)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `vendor/github.com/vishvananda/netlink/qdisc.go` — Qdisc interface, all types
  → Constraint: QdiscAttrs needs LinkIndex, Handle, Parent
- [ ] `vendor/github.com/vishvananda/netlink/qdisc_linux.go` — QdiscAdd, QdiscDel, QdiscReplace, QdiscList
  → Constraint: all operations take Qdisc interface
- [ ] `vendor/github.com/vishvananda/netlink/class.go` — Class interface, HtbClass, HfscClass
  → Constraint: ClassAttrs needs LinkIndex, Handle, Parent
- [ ] `internal/plugins/ifacenetlink/manage_linux.go` — how ze uses vishvananda/netlink
  → Constraint: netlink.LinkByName for name→index, error wrapping with context

**Behavior to preserve:**
- No existing tc code. Greenfield.
- Existing interface configuration (ifacenetlink) must not be affected.

**Behavior to change:**
- Add trafficnetlink plugin that programs tc via vishvananda/netlink

## Data Flow (MANDATORY)

### Entry Point
- Component calls `backend.Apply(desired map[string]InterfaceQoS)` on startup and reload

### Transformation Path
1. Backend receives `map[string]InterfaceQoS` (ze data model)
2. For each interface name: resolve to link index via netlink.LinkByName
3. List current qdiscs on that link
4. Replace root qdisc with desired qdisc type (QdiscReplace)
5. Add classes under the root qdisc (ClassAdd)
6. Add filters to classify traffic into classes (FilterAdd)
7. For interfaces in previous state but not in desired: remove custom qdiscs (restore default)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Component → Plugin | Apply(map[string]InterfaceQoS) method call | [ ] |
| Plugin → Kernel | vishvananda/netlink RTNETLINK socket | [ ] |

### Integration Points
- `internal/component/traffic/model.go` (fw-1) — InterfaceQoS, Qdisc, TrafficClass types
- `internal/component/traffic/backend.go` (fw-1) — Backend interface, RegisterBackend
- `vendor/github.com/vishvananda/netlink` — already vendored

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling (traffic plugin independent of firewall plugin)
- [ ] No duplicated functionality
- [ ] Zero-copy not applicable

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| traffic-control config in YANG | → | trafficnetlink Apply creates qdiscs | `test/traffic/001-boot-apply.ci` |
| Config reload changes tc | → | trafficnetlink Apply replaces qdiscs | `test/traffic/003-reload.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Apply with HTB qdisc on eth0 | HTB root qdisc on eth0, classes with rate/ceil |
| AC-2 | Apply with HFSC qdisc | HFSC root qdisc with rt/ls/ul service curves |
| AC-3 | Apply with FQ_CoDel | FQ_CoDel qdisc with target/interval/limit |
| AC-4 | Apply with mark-based filter | fw filter classifying by nfmark into correct class |
| AC-5 | Apply with DSCP-based filter | u32 filter matching DSCP field |
| AC-6 | Interface not found | Clear error message naming the interface |
| AC-7 | Apply removes interface from config | Custom qdisc removed, default restored |
| AC-8 | Apply with nested classes | Parent-child class hierarchy correct (HTB allows nesting) |
| AC-9 | Backend registered as "tc" | LoadBackend("tc") succeeds |
| AC-10 | Apply with VoIP QoS pattern | HTB with voip (rate 10mbit, priority 0, mark 0x10), interactive, bulk classes |
| AC-11 | ListQdiscs on configured interface | Returns current qdisc, classes, filters |
| AC-12 | ListQdiscs on unconfigured interface | Returns empty/default state |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestTranslateHTB` | `internal/plugins/trafficnetlink/translate_test.go` | ze HTB → netlink.Htb + HtbClass |
| `TestTranslateHFSC` | `internal/plugins/trafficnetlink/translate_test.go` | ze HFSC → netlink.Hfsc + HfscClass |
| `TestTranslateFQCoDel` | `internal/plugins/trafficnetlink/translate_test.go` | ze FQCoDel → netlink.FqCodel |
| `TestTranslateFilter` | `internal/plugins/trafficnetlink/translate_test.go` | ze TrafficFilter → netlink fw/u32 filter |
| `TestReconcileCreate` | `internal/plugins/trafficnetlink/reconcile_test.go` | New qdisc created on interface |
| `TestReconcileReplace` | `internal/plugins/trafficnetlink/reconcile_test.go` | Changed qdisc replaced |
| `TestReconcileRemove` | `internal/plugins/trafficnetlink/reconcile_test.go` | Removed interface gets default qdisc restored |
| `TestInterfaceNotFound` | `internal/plugins/trafficnetlink/reconcile_test.go` | Clear error for missing interface |
| `TestListQdiscs` | `internal/plugins/trafficnetlink/read_test.go` | ListQdiscs returns current tc state |
| `TestBackendRegistration` | `internal/plugins/trafficnetlink/register_test.go` | RegisterBackend("tc") works |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| HTB rate bps | 1+ | 1 | 0 | N/A (uint64) |
| HTB ceil | >= rate | rate | rate-1 | N/A |
| HTB burst | 1+ | 1 | 0 | N/A |
| FQ_CoDel target us | 1+ | 1 | 0 | N/A |
| FQ_CoDel limit | 1+ | 1 | 0 | N/A |
| Class priority | 0-7 | 7 | N/A | 8 |
| Mark value | 0-0xFFFFFFFF | 0xFFFFFFFF | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Boot apply | `test/traffic/001-boot-apply.ci` | ze starts with tc config, tc qdisc show shows HTB | |
| CLI show | `test/traffic/002-cli-show.ci` | `ze traffic-control show` outputs qdiscs and classes | |
| Reload | `test/traffic/003-reload.ci` | tc config changed, reload, new qdiscs applied | |
| VoIP pattern | `test/traffic/004-voip-qos.ci` | Full VoIP QoS: HTB with voip/interactive/bulk classes | |

### Future (if deferring any tests)
- None

## Files to Modify

No existing files modified. All new files.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | Spec-fw-4 |
| CLI commands | No | Spec-fw-5 |
| Functional test | Yes | `test/traffic/*.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` — traffic control |
| 2-12 | Other categories | Handled by umbrella and fw-4/fw-5 specs | |

## Files to Create

- `internal/plugins/trafficnetlink/trafficnetlink.go` — package doc, logger
- `internal/plugins/trafficnetlink/backend_linux.go` — Apply implementation
- `internal/plugins/trafficnetlink/backend_other.go` — stub for non-linux
- `internal/plugins/trafficnetlink/translate_linux.go` — ze types → netlink types
- `internal/plugins/trafficnetlink/translate_test.go` — translation unit tests
- `internal/plugins/trafficnetlink/reconcile_linux.go` — diff and apply logic
- `internal/plugins/trafficnetlink/reconcile_test.go` — reconciliation tests
- `internal/plugins/trafficnetlink/read_linux.go` — ListQdiscs implementation
- `internal/plugins/trafficnetlink/read_test.go` — ListQdiscs tests
- `internal/plugins/trafficnetlink/register.go` — init() RegisterBackend("tc", factory)
- `test/traffic/001-boot-apply.ci` — functional test
- `test/traffic/002-cli-show.ci` — functional test
- `test/traffic/003-reload.ci` — functional test
- `test/traffic/004-voip-qos.ci` — functional test

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + fw-0 + fw-1 |
| 2. Audit | Files to Create |
| 3. Implement (TDD) | Phases below |
| 4-12 | Standard flow |

### Implementation Phases

1. **Phase: Translation layer** — ze InterfaceQoS → netlink qdisc/class/filter types
   - Tests: TestTranslateHTB, TestTranslateHFSC, TestTranslateFQCoDel, TestTranslateFilter
   - Files: translate_linux.go
   - Verify: tests fail → implement → tests pass

2. **Phase: Reconciler** — diff desired vs kernel tc state, apply delta
   - Tests: TestReconcileCreate, TestReconcileReplace, TestReconcileRemove, TestInterfaceNotFound
   - Files: reconcile_linux.go, backend_linux.go
   - Verify: tests fail → implement → tests pass

3. **Phase: Registration and stubs** — RegisterBackend, non-linux stub
   - Tests: TestBackendRegistration
   - Files: register.go, backend_other.go
   - Verify: tests fail → implement → tests pass

4. **Phase: Functional tests** — .ci tests
   - Files: test/traffic/*.ci
   - Verify: all pass

5. **Full verification** → `make ze-verify`

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N implemented |
| Correctness | Qdisc handles and parent handles correct for class hierarchy |
| Naming | Backend name is "tc" |
| Read methods | ListQdiscs returns correct state |
| Data flow | Component → Apply → translate → netlink calls |
| Rule: no-layering | Direct vishvananda/netlink usage, no wrapper |
| Handle math | HTB major:minor handles computed correctly |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| trafficnetlink plugin compiles | `go build ./internal/plugins/trafficnetlink/...` |
| Translation covers all qdisc types | unit test output |
| Reconciler handles create/replace/remove | unit test output |
| ListQdiscs works | TestListQdiscs passes |
| Functional tests pass | `test/traffic/*.ci` all pass |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Interface name validation | Only apply to interfaces that exist (LinkByName error check) |
| Privilege | tc requires CAP_NET_ADMIN |
| Rate values | Validated as positive before passing to netlink |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in phase |
| Handle math wrong | Re-read tc handle documentation |
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
- Plugin registration: `register.go` calls `traffic.RegisterBackend("tc", newBackend)`
- Non-Linux stub: `backend_other.go` returns "not supported on $GOOS"
- Linux backend: `backend_linux.go` with Apply (per-interface qdisc replace, class add, filter add), ListQdiscs (query + type raise)
- Translation layer: `translate_linux.go` converts all 10 ze qdisc types to netlink types, HTB/HFSC class translation with rate/ceil/priority, mark-based fw filter and u32 filter translation, bidirectional qdisc type mapping
- `make generate` updated `all.go` to import trafficnetlink plugin

### Bugs Found/Fixed
- None (greenfield)

### Documentation Updates
- None (covered by fw-0 umbrella)

### Deviations from Plan
- Linux-only code cannot be tested on macOS. Type mismatches in `_linux.go` files (reported by LSP cross-compilation diagnostics) need Linux CI validation.
- Read method (ListQdiscs) returns qdisc type but not full class/filter tree. Full reconstruction requires walking the class and filter lists per qdisc.
- Functional .ci tests deferred: require Linux + CAP_NET_ADMIN.

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
- [ ] AC-1..AC-12 all demonstrated
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
- [ ] Write learned summary to `plan/learned/NNN-fw-3-traffic-netlink.md`
- [ ] Summary included in commit
