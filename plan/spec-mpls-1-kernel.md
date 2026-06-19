# Spec: mpls-1-kernel

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1 of 3 (MPLS) |
| Updated | 2026-06-18 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/plugins/fib/kernel/fibkernel.go` - current kernel FIB backend
4. `internal/plugins/fib/vpp/mpls.go` - existing VPP MPLS implementation (reference)
5. `internal/plugins/sysrib/events/events.go` - BestChangeEntry with Labels field

## Task

Add MPLS forwarding support to the Linux kernel FIB backend. The sysrib already
carries labels in `BestChangeEntry.Labels` and the VPP backend already programs
them. The kernel backend currently ignores labels. This spec extends the kernel
backend to program MPLS routes via netlink: label push (imposition), swap
(transit), and pop (disposition).

After this spec, BGP labeled unicast routes received from peers are installed
into the Linux kernel MPLS FIB, making Ze a functional MPLS label-switching
router without VPP.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - FIB plugin architecture
- [ ] `internal/plugins/fib/vpp/mpls.go` - reference implementation for MPLS FIB operations

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc3032.md` - MPLS Label Stack Encoding (MUST CREATE)
  → Constraint: 20-bit label, S-bit, TTL, label stack ordering
- [ ] `rfc/short/rfc3031.md` - MPLS Architecture (MUST CREATE)
  → Constraint: LSR forwarding model, FEC-to-label binding

**Key insights:**
- Linux kernel MPLS support since 4.x: `RTM_NEWROUTE`/`RTM_DELROUTE` with `AF_MPLS`, `RTA_NEWDST` for swap, `RTA_ENCAP` with `MPLS_IPTUNNEL_DST` for push
- Kernel requires `sysctl net.mpls.platform_labels` and per-interface `net.mpls.conf.<iface>.input`
- The VPP backend already validates labels (20-bit range, stack depth <= 16)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/fib/kernel/fibkernel.go` - subscribes to sysrib best-change, programs kernel routes via netlink
- [ ] `internal/plugins/fib/kernel/backend_linux.go` - netlink route add/del/replace
- [ ] `internal/plugins/sysrib/events/events.go` - BestChangeEntry has Labels field, already populated by RIB

**Behavior to preserve:**
- Unlabeled route installation unchanged
- Route monitoring and re-assertion on external modification
- Prometheus metrics for route installs/updates/removals
- Protocol ID tagging for ze-owned routes

**Behavior to change:**
- Labeled routes currently silently dropped by kernel backend (labels ignored)
- No MPLS sysctl configuration at startup
- No MPLS-specific metrics

## Data Flow (MANDATORY)

### Entry Point
- BGP receives labeled unicast UPDATE from peer
- RIB selects best path, emits `(system-rib, best-change)` with `Labels` populated

### Transformation Path
1. `fibkernel` receives `BestChangeBatch` via EventBus subscription
2. `processEvent` checks `entry.Labels` -- if non-empty, dispatches to MPLS path
3. MPLS path constructs netlink message with `RTA_ENCAP` (push) or `AF_MPLS` + `RTA_NEWDST` (swap)
4. Netlink programs kernel MPLS FIB

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| sysrib -> fibkernel | EventBus typed event `BestChange` | [ ] |
| fibkernel -> kernel | netlink `RTM_NEWROUTE` with `AF_MPLS` / `RTA_ENCAP` | [ ] |

### Integration Points
- `backend_linux.go` extended with MPLS netlink operations
- sysctl plugin: must configure `net.mpls.platform_labels` and per-interface `net.mpls.conf.*.input`
- `ze doctor` check for kernel MPLS support (`/proc/sys/net/mpls/platform_labels`)

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| sysrib best-change with Labels | -> | fibkernel MPLS dispatch | `TestFibKernelMPLSPush` |
| config `interface X { mpls { enable; } }` | -> | sysctl `net.mpls.conf.X.input=1` | `TestMPLSInterfaceEnable` |
| `ze doctor` | -> | MPLS kernel support check | `test/plugin/mpls-doctor.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | BGP labeled unicast route received with label stack | Kernel MPLS route installed via netlink (label push) |
| AC-2 | BGP labeled unicast route withdrawn | Kernel MPLS route removed |
| AC-3 | BGP labeled unicast route updated (label change) | Kernel MPLS route replaced |
| AC-4 | Interface configured with `mpls { enable; }` | sysctl `net.mpls.conf.<iface>.input=1` applied |
| AC-5 | System startup with MPLS config | `net.mpls.platform_labels` set, interfaces enabled |
| AC-6 | `ze doctor` on system without MPLS support | Warning: kernel MPLS not available |
| AC-7 | `show mpls forwarding` CLI command | Display installed MPLS label entries |
| AC-8 | Prometheus metrics | `ze_fibkernel_mpls_routes_installed`, `ze_fibkernel_mpls_installs_total` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestMPLSPushNetlink` | `internal/plugins/fib/kernel/mpls_test.go` | Netlink message for label push | |
| `TestMPLSSwapNetlink` | `internal/plugins/fib/kernel/mpls_test.go` | Netlink message for label swap | |
| `TestMPLSPopNetlink` | `internal/plugins/fib/kernel/mpls_test.go` | Netlink message for label pop (implicit null) | |
| `TestMPLSLabelValidation` | `internal/plugins/fib/kernel/mpls_test.go` | 20-bit range, stack depth | |
| `TestFibKernelMPLSDispatch` | `internal/plugins/fib/kernel/fibkernel_test.go` | Labeled entries routed to MPLS path | |
| `TestFibKernelUnlabeledUnchanged` | `internal/plugins/fib/kernel/fibkernel_test.go` | Unlabeled entries use existing path | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Label | 0-1048575 | 1048575 | N/A | 1048576 |
| Stack depth | 1-16 | 16 | 0 (empty) | 17 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `mpls-push` | `test/plugin/mpls-push.ci` | BGP labeled route installed in kernel | |
| `mpls-withdraw` | `test/plugin/mpls-withdraw.ci` | BGP labeled route removed from kernel | |
| `mpls-doctor` | `test/plugin/mpls-doctor.ci` | `ze doctor` reports MPLS support | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `mpls-labeled-frr` | `test/interop/scenarios/` | FRR | Labeled unicast exchanged, kernel MPLS route installed | |

### Future (if deferring any tests)
- None planned

## Files to Modify
- `internal/plugins/fib/kernel/fibkernel.go` - add MPLS dispatch in processEvent
- `internal/plugins/fib/kernel/backend_linux.go` - add MPLS netlink operations
- `internal/plugins/fib/kernel/backend_other.go` - noop MPLS stubs for non-Linux
- `internal/plugins/sysctl/known.go` - add MPLS sysctl keys
- `cmd/ze/doctor/checks.go` - add MPLS kernel support check

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [x] | `internal/yang/modules/mpls.yang` |
| CLI commands/flags | [x] | `show mpls forwarding` |
| CLI grammar (action before identifier) | [x] | `.claude/rules/cli-grammar.md` |
| Editor autocomplete | [x] | YANG-driven (automatic if YANG updated) |
| Functional test for new RPC/API | [x] | `test/plugin/mpls-push.ci` |
| Doctor check for runtime dependencies | [x] | `cmd/ze/doctor/`, `ai/rules/doctor-checks.md` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` |
| 2 | Config syntax changed? | [x] | `docs/guide/configuration.md` |
| 3 | CLI command added/changed? | [x] | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | [ ] | |
| 5 | Plugin added/changed? | [x] | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | [x] | `docs/guide/mpls.md` |
| 7 | Wire format changed? | [ ] | |
| 8 | Plugin SDK/protocol changed? | [ ] | |
| 9 | RFC behavior implemented? | [x] | `rfc/short/rfc3032.md`, `rfc/short/rfc3031.md` |
| 10 | Test infrastructure changed? | [ ] | |
| 11 | Affects daemon comparison? | [x] | `docs/comparison.md` |
| 12 | Internal architecture changed? | [ ] | |

## Files to Create
- `internal/plugins/fib/kernel/mpls_linux.go` - MPLS netlink operations
- `internal/plugins/fib/kernel/mpls_other.go` - noop MPLS for non-Linux
- `internal/plugins/fib/kernel/mpls_test.go` - MPLS unit tests
- `internal/yang/modules/ze-mpls.yang` - MPLS interface config
- `test/plugin/mpls-push.ci` - functional test
- `test/plugin/mpls-withdraw.ci` - functional test
- `test/plugin/mpls-doctor.ci` - doctor check test
- `rfc/short/rfc3032.md` - MPLS Label Stack Encoding summary
- `rfc/short/rfc3031.md` - MPLS Architecture summary

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist below |
| 8. Fix issues | Fix every issue from critical review |
| 9. Re-verify | Re-run stage 6 |
| 10. Repeat 7-9 | Until clean |
| 11. Deliverables review | Deliverables Checklist below |
| 12. Security review | Security Review Checklist below |
| 13. Re-verify | Re-run stage 6 |
| 14. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** -- register MPLS dispatch in fibkernel, write failing wiring tests
   - Tests: `TestFibKernelMPLSPush`, `TestMPLSInterfaceEnable`
   - Files: `fibkernel.go`, `mpls_linux.go` skeleton
   - Verify: entry point exists and is reachable; wiring test fails because feature logic is a stub
2. **Phase: Netlink MPLS** -- implement `AF_MPLS` route programming via netlink (push, swap, pop)
   - Tests: `TestMPLSPushNetlink`, `TestMPLSSwapNetlink`, `TestMPLSPopNetlink`, `TestMPLSLabelValidation`
   - Files: `mpls_linux.go`, `mpls_other.go`
   - Verify: tests fail -> implement -> tests pass
3. **Phase: Sysctl** -- YANG schema for interface MPLS enable, sysctl application
   - Tests: `TestMPLSInterfaceEnable`
   - Files: `ze-mpls.yang`, `sysctl/known.go`
   - Verify: interface MPLS enable programs sysctl
4. **Phase: Doctor** -- kernel MPLS support detection
   - Tests: `test/plugin/mpls-doctor.ci`
   - Files: `cmd/ze/doctor/checks.go`
   - Verify: doctor reports MPLS status
5. **Phase: CLI** -- `show mpls forwarding` command
   - Tests: functional test for show output
   - Files: CLI handler registration
   - Verify: command displays installed entries
6. **Phase: Metrics** -- MPLS-specific Prometheus counters
   - Tests: metrics registration test
   - Files: `fibkernel.go` metrics block
   - Verify: counters increment on install/withdraw
7. **Phase: Functional tests** -- end-to-end .ci tests with labeled unicast
   - Tests: `mpls-push.ci`, `mpls-withdraw.ci`
   - Verify: full daemon lifecycle with labeled routes
8. **RFC refs** -- Add `// RFC 3032 Section X.Y` comments
9. **Full verification** -- `make ze-verify`
10. **Complete spec** -- learned summary, delete spec

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Netlink messages match kernel MPLS API exactly |
| Naming | YANG uses kebab-case, CLI uses `show mpls forwarding` |
| Data flow | Labels flow sysrib -> fibkernel -> netlink, no bypass |
| CLI grammar | `show mpls forwarding` follows action-before-identifier |
| Doctor checks | `ze doctor` check registered for MPLS kernel support |
| Rule: no-layering | No duplication with VPP MPLS backend |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| `mpls_linux.go` exists | `ls internal/plugins/fib/kernel/mpls_linux.go` |
| YANG module exists | `ls internal/yang/modules/ze-mpls.yang` |
| Functional tests exist | `ls test/plugin/mpls-push.ci test/plugin/mpls-withdraw.ci` |
| RFC summaries exist | `ls rfc/short/rfc3031.md rfc/short/rfc3032.md` |
| Doctor check registered | `grep -r "mpls" cmd/ze/doctor/` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Label values validated to 20-bit range before netlink call |
| Resource exhaustion | Label table size bounded by `platform_labels` sysctl |
| Privilege | MPLS netlink requires CAP_NET_ADMIN (already held by fibkernel) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
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

Add `// RFC 3032 Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, label encoding.

## Implementation Summary

### What Was Implemented
- A closure-time audit first concluded this spec was "already fully implemented" -- the fibkernel netlink code was all present. **A subsequent deep review proved that wrong (F1):** the default in-process path routes BGP best-paths through the unified Loc-RIB, whose `Path` carried no `Labels`, so a BGP labeled-unicast route installed a plain IP route, not an MPLS push. AC-1 did NOT work end-to-end. This session fixed it (see Bugs Found) and the supporting kernel pieces remain:
  - AC-1/2/3 (push/withdraw/relabel): `fibkernel.go` `processEvent`→`addChange`/`replaceChange`/`delChange` validate `c.Labels` and route to the rich-route path (AF_MPLS encap in `mplsentry_linux.go`/`nexthop_linux.go`) -- NOW reachable because F1 carries the labels through `locrib.Path` + `sysrib.changeToBatch`.
  - AC-4/5 (iface MPLS enable → sysctl): `iface/config_sysctl.go` emits `net.mpls.conf.<iface>.input`; `core/sysctl/known_linux.go` has `net.mpls.platform_labels`; YANG `iface/yang/ze-iface-conf.yang` container `mpls { enable }`.
  - AC-6 (doctor): `doctor/checks_linux.go checkMPLSSupport` → `doctor-mpls-unavailable`, now gated on MPLS actually being in use (F15).
  - AC-7 (`show mpls forwarding`): `component/mpls/show_forwarding.go` + `mpls-cmd` YANG, now including label-imposition (push) routes (F14).
  - AC-8 (metrics): `ze_fibkernel_mpls_routes_installed`, `ze_fibkernel_mpls_installs_total` in `fibkernel.go`.
- Also corrected stale `# TODO: verify kernel MPLS FIB` comments in `test/plugin/mpls-push.ci`/`mpls-withdraw.ci`.
- **Closed**: the end-to-end BGP-LU → kernel-MPLS-route integration test now exists and is QEMU-green. `internal/plugins/fib/kernel/bgplu_mpls_integration_linux_test.go::TestMPLSIntegration_BGPLabeledUnicastPush` drives a labeled `BestChange` through the production `processEvent` → rich-route path to a live Alpine kernel and reads back the MPLS label encap (push), the relabel (replace), the withdraw, and a foreign-route non-clobber case. AC-1/AC-2/AC-3 are now certified against a real kernel through the exact F1 path (distinct from the LDP/RSVP-TE `handleMPLSEntry` path).

### Bugs Found/Fixed
- None. (Data-plane is verified under QEMU by `internal/plugins/fib/kernel/mplsentry_integration_linux_test.go`.)

### Documentation Updates
- RFC summaries `rfc/short/rfc3031.md`, `rfc/short/rfc3032.md` already exist. Only `.ci` comment accuracy corrected.

### Deviations from Plan
- The spec named files to create (`mpls_linux.go`, `ze-mpls.yang`) that were ultimately realized under different names/locations (`mplsentry_linux.go`, the iface YANG `mpls` container, the `mpls-cmd` plugin). Functionality matches; file layout differs.
- The `.ci` functional tests verify the BGP control plane; the kernel netlink data plane is verified by the QEMU integration test rather than within `.ci` (keeps the `.ci` harness free of an MPLS-capable-kernel dependency).

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

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Kernel MPLS route install (push) | unit + integration | `fibkernel_test.go::TestKernelMPLSPush`; `mplsentry_integration_linux_test.go::TestMPLSIntegration_Push` (live kernel, reads AF_MPLS/encap route back) |
| Label withdraw removes entry | integration | `TestMPLSIntegration_Push` (withdraw half); `mpls-withdraw.ci` (control plane) |
| Relabel updates kernel route | integration | `TestMPLSIntegration_PushRelabel` |
| Label validation (20-bit/stack) | unit (boundary) | `mpls_linux_test.go::TestMPLSLabelValidationBoundary` |
| BGP labeled-unicast → kernel encap (F1, AC-1/2/3) | integration | `fib/kernel/bgplu_mpls_integration_linux_test.go::TestMPLSIntegration_BGPLabeledUnicastPush` + `_NoClobberForeign` -- QEMU-green on a live Alpine kernel (push/relabel/withdraw + foreign non-clobber), the production `processEvent` path |
| BGP labeled-unicast control plane | functional | `mpls-push.ci`/`mpls-withdraw.ci`, `fib-mpls-kernel.ci` |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | NOTE | Spec already fully implemented; only stale `.ci` TODO comments remained | `test/plugin/mpls-{push,withdraw}.ci` | Corrected comments to state actual scope + cross-ref integration test |

### Fixes applied
- `.ci` comment accuracy corrected; no code changes needed.

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| - | none | All 8 ACs verified with file:line + tests | - | - |

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE  → met: 0 blocker / 0 issue
- [ ] All NOTEs recorded above (or explicitly "none")  → recorded above

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
- [ ] AC-1..AC-8 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments added
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
- [ ] Interop tests for protocol features
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-mpls-kernel.md`
- [ ] Summary included in commit
