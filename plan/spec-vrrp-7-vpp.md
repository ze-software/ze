# Spec: vrrp-7 -- VRRP on the VPP Dataplane (Skeleton)

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | spec-vrrp-5-plugin (closed with vrrp-0..5, commit fbc99f1d4, learned 1124) |
| Phase | - |
| Updated | 2026-07-14 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/spec-vrrp-0-umbrella.md` -- decisions and the interim VPP rejection contract
3. `plan/spec-vrrp-5-plugin.md` -- where the verify-time rejection is implemented
4. `rfc/short/rfc9568.md`, `rfc/short/rfc3768.md`

## Task

Bring VRRP to VPP-backed interfaces. The kernel/netlink implementation
(spec-vrrp-1..6) uses macvlan devices and raw sockets, neither of which exists
on the VPP dataplane. VPP ships its own native VRRP plugin; the expected design
is to translate ze vrrp group config into VPP VRRP binapi calls, mirroring how
other ze features drive VPP (translate + apply + verify pattern).

Until this spec is designed and implemented, the contract set by the umbrella
holds: **vrrp config on a VPP-backed interface is REJECTED at verify time with
an actionable error** (fail closed, per the exact-or-reject principle in
`ai/rules/architecture.md`). That rejection is implemented and tested by
spec-vrrp-5 (`test/vrrp/vrrp-vpp-reject.ci`). This skeleton exists so the
deferred VPP work has a concrete destination (`ai/rules/planning.md`
"No deferral without a destination").

Open research questions for the design phase (all currently unverified):
- VPP VRRP binapi surface (vr add/del, flags for preempt/accept, unicast peers,
  virtual MAC handling) and its version/feature coverage vs RFC 9568
- Whether ze's vendored VPP binapi bindings already include the vrrp service
- Parity mapping: which ze YANG leaves translate exactly, which cannot
  (exact-or-reject decides per leaf)
- Show/statistics: reading VPP VRRP state back into `show vrrp`
- Test infra: VPP VRRP in the existing fakeOps pattern (`ai/rules/testing.md`
  "VPP Backend Testing Is Mandatory")

## Required Reading

### Architecture Docs
- [ ] `ai/rules/testing.md` (VPP Backend Testing Is Mandatory) - fakeOps seam requirement
  → Constraint: Apply/Undo via scripted fakeOps tests; translate/verify as pure-function tests; no "needs a real VPP daemon" deferrals
- [ ] `ai/rules/architecture.md` (Exact or reject) - parity contract
  → Constraint: any ze vrrp leaf the VPP plugin cannot apply exactly fails verify with a clear error, never silent approximation
- [ ] `plan/spec-vrrp-0-umbrella.md` - decisions this child inherits
  → Decision: v3 default + v2 opt-in, ipv4/ipv6 containers, virtual MAC mandatory -- VPP native VRRP must honor all four or reject

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc9568.md` - conformance target for the VPP-side behavior audit
  → Constraint: VPP's own VRRP implementation must be audited against the compliance checklist before ze exposes it as equivalent
- [ ] `rfc/short/rfc3768.md` - v2 opt-in parity question
  → Constraint: if VPP VRRP lacks v2, `version 2` on a VPP interface is a verify-time rejection

**Key insights:**
- The netlink design (macvlan + raw sockets) is intentionally NOT portable to
  VPP; this child is a translate-to-native-VPP effort, not a port.

## Current Behavior (MANDATORY)

**Source files read:** (verified this session; VPP binapi surface deliberately not yet researched)
- [ ] `internal/component/firewall/protocol.go` - `"vrrp": 112` :14 -- firewall knows the protocol number only
- [ ] `internal/plugins/firewall/vpp/translate.go` - `"vrrp": 112` :26 -- VPP firewall translation knows the number only
- [ ] `internal/component/iface/address_owner.go` - RegisterOwnedAddresses :80 -- the kernel-path VIP mechanism this child replaces on VPP

**Behavior to preserve:**
- The spec-vrrp-5 verify-time rejection stays in place until this child ships;
  removing it is the last step of this child's implementation
- Kernel-path VRRP (children 1-6) behavior unchanged by this child
- Existing VPP features (firewall, fib, traffic) untouched

**Behavior to change:**
- None until design: this is a skeleton. At implementation, vrrp on VPP-backed
  interfaces moves from verify-rejection to native VPP VRRP.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Same YANG config as spec-vrrp-5 (`interface ... unit ... ipv4|ipv6 vrrp group <vrid>`); the dataplane backend of the parent interface selects the path

### Transformation Path
1. Config commit -> vrrp plugin GroupSpec extraction (spec-vrrp-5, unchanged)
2. Backend dispatch: parent is VPP-backed -> VPP VRRP translator (this child) instead of macvlan/raw-socket instance
3. Translator -> VPP VRRP binapi calls (vr add/config) through the fakeOps-seamed apply pipeline
4. VPP runs the protocol; state polled/streamed back for `show vrrp` and metrics

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| vrrp plugin <-> VPP backend | translate/apply seam (fakeOps pattern) | [ ] |
| ze <-> VPP | binapi (govpp) session | [ ] |
| VPP VRRP <-> show/metrics | state read-back mapping | [ ] |

### Integration Points
- spec-vrrp-5 GroupSpec model (shared config source)
- VPP apply pipeline precedents under `internal/plugins/iface/vpp/` and `internal/plugins/traffic/vpp/` (fakeOps reference `internal/plugins/traffic/vpp/apply_test.go` per `ai/rules/testing.md`)

### Architectural Verification
- [ ] No bypassed layers (translator lives in the VPP backend tier, plugin stays dataplane-agnostic)
- [ ] No unintended coupling (kernel path keeps zero VPP knowledge)
- [ ] No duplicated functionality (one GroupSpec source of truth)
- [ ] Zero-copy preserved where applicable (control-plane only; N/A beyond binapi buffers)
- [ ] Registration over hardcoding -- backend selected through the existing backend registry, no vrrp switch in shared code

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | VPP's native VRRP plugin covers RFC 9568 v3 for IPv4+IPv6 including virtual MAC | VPP upstream ships a vrrp plugin (unverified -- not yet researched in this repo's vendored binapi) | Scope shrinks to what VPP supports; unsupported leaves stay verify-rejected | Design phase: read vendored binapi / VPP docs; conformance audit vs rfc/short checklist | unvalidated |
| A-2 | ze's govpp bindings can be extended to the vrrp service without vendoring churn | other VPP features bind services under `internal/plugins/*/vpp/` | binapi regeneration becomes a prerequisite task in this spec | Design phase: locate binapi generation path and check for vrrp service | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | VPP VRRP semantics diverge from ze's kernel-path behavior (timers, accept-mode), splitting operator experience per dataplane | Conformance audit table shows divergent rows | Exact-or-reject per leaf; document divergences in docs/guide/vrrp.md |
| R-2 | Skeleton rots: nobody picks this up and the verify-rejection quietly becomes permanent | spec-status shows this skeleton unclaimed while vrrp ships | Umbrella Known Limitations names this spec; `./le spec-status` keeps it visible |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| vrrp group on VPP-backed interface (interim contract) | → | spec-vrrp-5 verify rejection | `test/vrrp/vrrp-vpp-reject.ci` (owned by spec-vrrp-5) |
| vrrp group on VPP-backed interface (this child, at implementation) | → | VPP VRRP translator apply | `test/vrrp/vrrp-vpp-native.ci` |

## Acceptance Criteria

Skeleton-level; the design phase expands these.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | vrrp group committed on a VPP-backed interface | VPP VRRP virtual router created via binapi; state visible in `show vrrp` |
| AC-2 | ze leaf with no exact VPP equivalent | verify fails naming leaf + interface (exact-or-reject) |
| AC-3 | Failover between a VPP-backed ze and a kernel-path peer | interop scenario passes (same wire protocol) |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Commits vrrp group on VPP interface | config -> GroupSpec -> VPP translator -> binapi -> VPP VRRP active | `test/vrrp/vrrp-vpp-native.ci` |
| 2 | Runs `show vrrp` on a VPP-backed group | VPP state read-back -> show payload | `test/vrrp/vrrp-vpp-show.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestVPPVRRPTranslate` | `internal/plugins/iface/vpp/vrrp_translate_test.go` (indicative) | GroupSpec -> binapi request mapping, per-leaf reject list | |
| `TestVPPVRRPApplyUndo` | fakeOps apply tests | create/update/delete/partial-failure undo | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| vrid | 1-255 | 255 | 0 | 256 |
| advertise-interval-milliseconds | VPP-supported range (design phase) | tbd at design | tbd at design | tbd at design |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `vrrp-vpp-native.ci` | `test/vrrp/` | group active on VPP dataplane | |
| `vrrp-vpp-show.ci` | `test/vrrp/` | show vrrp reflects VPP state | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| VPP-ze vs keepalived failover | `test/interop/scenarios/` or QEMU lab extension | keepalived | wire parity of the VPP path | |

### Future (if deferring any tests)
- None beyond the skeleton status itself; the whole child is future work with this spec as its destination.

## Files to Modify
- `internal/plugins/iface/vpp/` - VRRP translator + apply (design phase fixes exact files)
- `internal/plugins/vrrp/` - backend dispatch: remove the interim verify-rejection when native support lands
- `docs/guide/vrrp.md` - VPP section

## Files to Create
- `internal/plugins/iface/vpp/vrrp_*.go` - translator/apply/read-back (indicative)
- `test/vrrp/vrrp-vpp-native.ci`, `test/vrrp/vrrp-vpp-show.ci`

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | N/A | reuses spec-vrrp-5 schema unchanged |
| YANG validation constraints | Yes | per-leaf VPP-parity validator additions (design phase) |
| YANG custom validators | Yes | VPP-parity reject list (design phase) |
| CLI commands/flags | N/A | `show vrrp` reused |
| CLI grammar (action before identifier) | N/A | no new grammar |
| Editor autocomplete | N/A | unchanged |
| Functional test for new RPC/API | Yes | `test/vrrp/vrrp-vpp-native.ci` |
| Pipe completeness | N/A | reuses spec-vrrp-5 handlers |
| Env var registration | N/A | none |
| Doctor check for runtime dependencies | Yes | VPP VRRP plugin availability probe (design phase; `ai/rules/repo-maintenance.md`) |
| Prometheus counters/metrics | Yes | VPP-path state mapped into ze_vrrp_* series (design phase) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/guide/vrrp.md` VPP section; `docs/guide/vpp.md` |
| 2 | Config syntax changed? | No | same schema |
| 3 | CLI command added/changed? | No | same commands |
| 4 | API/RPC added/changed? | No | same wire methods |
| 5 | Plugin added/changed? | Yes | `docs/plugin-overview.md` if backend registration changes |
| 6 | Has a user guide page? | Yes | `docs/guide/vrrp.md` |
| 7 | Wire format changed? | No | protocol unchanged |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `docs/features/rfc-status.md` VPP-path coverage note |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` if a VPP vrrp lab target is added |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` VPP column nuance |
| 12 | Internal architecture changed? | No | backend-internal |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | Yes | metrics doc if labels change |
| 15 | Registered plugin/event/command inventory changed? | No | none expected |
| 16 | Changed source files referenced by doc source anchors? | Yes | grep at implementation |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | verify guide examples still hold on VPP |

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file after its design phase completes |
| 2. Audit | Files to Modify/Create |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Phases below |
| 5-14 | Per template; this child follows the umbrella's stage mapping |

### Implementation Phases

1. **Phase: Wiring (MANDATORY FIRST)** -- backend dispatch seam + failing `vrrp-vpp-native.ci`
   - Tests: `test/vrrp/vrrp-vpp-native.ci`
   - Files: `internal/plugins/iface/vpp/vrrp_*.go` skeleton
   - Verify: VPP path reachable, rejection removed behind a failing test
2. **Phase: Translate + verify** -- GroupSpec -> binapi mapping, per-leaf parity/reject
   - Tests: `TestVPPVRRPTranslate`
   - Files: translator
   - Verify: table-driven mapping tests pass
3. **Phase: Apply/read-back** -- fakeOps apply pipeline + show/metrics read-back
   - Tests: `TestVPPVRRPApplyUndo`, `vrrp-vpp-show.ci`
   - Files: apply + read-back
   - Verify: tests fail -> implement -> pass
4. **Functional tests** -> full .ci coverage
5. **RFC refs** -> conformance audit rows
6. **Full verification** -> `./le verify current mode full`
7. **Complete spec** -> audit + learned summary + two-commit closure

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC implemented with file:line |
| Correctness | VPP behavior audited against rfc/short/rfc9568.md checklist |
| Registration over hardcoding | Backend dispatch via existing registry, no vrrp switch in shared code |
| Rule: exact-or-reject | Every non-exact leaf rejected with actionable error |
| Rule: VPP testing mandatory | fakeOps apply/translate/verify/register tests all present |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| VPP VRRP active from ze config | `vrrp-vpp-native.ci` pass |
| Parity reject list | translate test table covers every YANG leaf |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | binapi requests built only from YANG-validated values |
| Failure handling | binapi errors surface as commit failures, never partial silent state |

### Failure Routing
| Failure | Route To |
|---------|----------|
| binapi mismatch | Design phase re-research |
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

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Native VPP VRRP over porting macvlan design | userspace VRRP over VPP host interfaces | VPP has a first-class VRRP plugin; punting to kernel defeats the dataplane |

## Known Limitations
- Entire child is future work; interim contract is verify-time rejection (spec-vrrp-5).

## RFC Documentation

At implementation: conformance audit table mapping VPP VRRP behavior to the
rfc/short/rfc9568.md compliance checklist; `// RFC 9568` comments on any ze-side
enforcement (e.g. parity validators).

## Implementation Summary

### What Was Implemented
- (skeleton -- nothing yet)

### Bugs Found/Fixed
- (skeleton)

### Documentation Updates
- (skeleton)

### Deviations from Plan
- (skeleton)

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
| VRRP on VPP dataplane | functional + interop test | (fill at implementation) |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- (fill during /ze-review)

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

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

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-3 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `./le verify current mode full` passes
- [ ] Feature code integrated (`internal/*`)
- [ ] Documentation Update Checklist answered with source evidence

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete

### Design
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features
- [ ] Goal Validation table filled

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Learned summary written
- [ ] Two-commit closure per `ai/rules/planning.md`
