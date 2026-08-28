# Spec: vrf

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-07-03 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/component/iface/yang/ze-iface-conf.yang` - the stub `vrf` leaf
4. `internal/plugins/iface/netlink/` - link creation / enslavement
5. `ai/rules/planning.md` - skeleton; full DESIGN not yet done

## Task

**Large feature area — skeleton only. Full design not started.**

Ze exposes a per-interface `vrf` config leaf, but it is a **schema-only stub**: no
Go code reads or applies it, and Ze creates no VRF devices and enslaves no
interfaces. So the leaf silently does nothing today, and features that depend on
VRF (VRF-bound services, per-VRF routing tables, WireGuard-in-VRF fwmark rules)
cannot work.

Implement real VRF support:

- Create Linux VRF master devices and enslave member interfaces (`ip link ... master
  vrf`), each VRF bound to its own routing table.
- Apply the per-unit `vrf` leaf so an interface's traffic uses the VRF's table.
- Provide the routing/fwmark rules that make VRF-bound interfaces (including tunnels
  such as WireGuard) forward out of the correct table.
- Tighten the `vrf` leaf validation (it is currently a bare `type string`).

This is foundational and multi-component (iface, routing/sysrib, policyroute). It
must go through the full `/ze-spec` RESEARCH/DESIGN workflow before implementation.
This skeleton tracks the gap; it is NOT ready to implement.

## Required Reading

### Architecture Docs
- [ ] `internal/plugins/iface/netlink/` - how links are created and enslaved (bridge uses `LinkSetMaster`).
  → Constraint: VRF enslavement mirrors bridge-port enslavement but binds a routing table; today `LinkSetMaster` is used only for bridge ports.
- [ ] `internal/plugins/policyroute/` - existing ip-rule / fwmark machinery (independent mark range).
  → Constraint: VRF table selection must coordinate with, not collide with, policyroute's fwmark range.
- [ ] `ai/rules/platform-linux.md` - VRF is Linux-only; QEMU integration tests are mandatory.
  → Constraint: never skip QEMU tests for "needs hardware".

### RFC Summaries (MUST for protocol work)
- [ ] N/A - VRF is a Linux dataplane feature, not a wire protocol. No RFC summary required (kernel `vrf` / l3mdev behaviour instead).

**Key insights:**
- The `vrf` leaf already exists in config; the work is dataplane + routing-table plumbing, plus wiring the leaf to it.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/iface/netlink/bridge_linux.go` - `LinkSetMaster` is used only for bridge ports (bridge_linux.go), never for VRF; there is no `netlink.Vrf` device creation anywhere in the netlink backend.
- [ ] `internal/component/iface/yang/ze-iface-conf.yang` - per-unit `leaf vrf { type string; }` (ze-iface-conf.yang); bare string, no validation, no completion. It is the only `vrf` reference in the iface component/plugin.
- [ ] `internal/plugins/policyroute/rules_linux.go` - the only ip-rule/fwmark machinery, with its own independent mark range; no VRF or table linkage.

**Behavior to preserve:**
- Non-VRF interfaces behave exactly as today.
- policyroute's existing fwmark range and rules are not disturbed.

**Behavior to change:**
- The `vrf` leaf becomes functional: VRF devices are created, members enslaved, tables bound.

## Data Flow (MANDATORY)

### Entry Point
- Config: a top-level `vrf` definition (name → routing table id) plus the existing per-unit `vrf <name>` membership leaf in `ze-iface-conf.yang`.
- Applied via the iface netlink backend at config apply.

### Transformation Path
1. Config parsed: VRF definitions (name, table) and per-interface membership.
2. Netlink creates a VRF master device per VRF, bound to its table id.
3. Member interfaces are enslaved to the VRF master (`LinkSetMaster` to the VRF, analogous to bridge ports).
4. Routing/ip-rules direct traffic on member interfaces to the VRF table; tunnel interfaces (e.g. WireGuard fwmark) resolve to the correct table.
5. VRF-aware services bind within the VRF where configured.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config ↔ iface netlink | VRF defs + membership → device creation/enslavement | [ ] |
| iface ↔ routing/sysrib | per-VRF table id, routes installed into the right table | [ ] |
| iface ↔ policyroute | fwmark/table rules coordinated, no mark-range collision | [ ] |

### Integration Points
- `internal/plugins/iface/netlink/` - VRF device creation + enslavement.
- `internal/component/iface/` - parse VRF defs and membership; validate the `vrf` leaf.
- `internal/plugins/policyroute/` and `internal/component/sysrib/` - table selection / route installation per VRF.

### Architectural Verification
- [ ] No bypassed layers (config via the standard iface path)
- [ ] No unintended coupling (VRF logic in iface/routing, not scattered)
- [ ] No duplicated functionality (reuse enslavement + rule machinery)
- [ ] Registration over hardcoding — VRF handling stays within iface/routing plugins; no per-VRF special-case in a core struct.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The netlink backend can create l3mdev VRF devices and enslave members | `LinkSetMaster` already used for bridges | needs new netlink plumbing | spike with the netlink library | unvalidated |
| A-2 | policyroute's fwmark range can coexist with VRF table selection | policyroute uses a fixed mark range | collision requires coordination | read policyroute marks during RESEARCH | unvalidated |
| A-3 | sysrib can install routes into per-VRF tables | routing table model | routing rework needed | read sysrib during RESEARCH | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Partial VRF (device made, routing not table-aware) leaks traffic to main table | traffic uses wrong table in QEMU test | do not ship until end-to-end table isolation is proven in QEMU |
| R-2 | Interaction with existing services binding to main table | services break when moved to a VRF | phase: interface enslavement first, service binding later |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `set interfaces ... vrf red` + VRF def | → | netlink creates VRF master, enslaves the unit | `test/qemu/vrf-enslave.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | define VRF `red` (table N) and assign an interface | VRF device exists; interface is enslaved to it |
| AC-2 | route learned on a VRF interface | installed into the VRF's table, not main |
| AC-3 | traffic on a VRF interface | uses the VRF table (isolation proven in QEMU) |
| AC-4 | WireGuard interface in a VRF | encrypted traffic routes out the correct table |
| AC-5 | invalid `vrf` leaf value | config verify rejects (leaf validation tightened) |
| AC-6 | no `vrf` on an interface | unchanged, uses main table |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | puts two interfaces in different VRFs | config → VRF devices + enslavement → isolated tables | `test/qemu/vrf-enslave.ci` |
| 2 | runs a service bound to a VRF | service binds within the VRF | `test/qemu/vrf-service.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestVRFLeafValidation` | `internal/component/iface/..._test.go` | `vrf` leaf constraint + completion | |
| `TestVRFEnslavePlan` | `internal/plugins/iface/netlink/vrf_test.go` | membership → enslavement plan | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| VRF table id | 1-4294967295 (design may reserve) | design | 0 (main reserved) | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `vrf-enslave` | `test/qemu/vrf-enslave.ci` | interface enslaved, table isolation proven | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A - dataplane feature; validated by QEMU functional tests, not a peer daemon | - | - | table isolation is a kernel behaviour, tested in QEMU | - |

### Future (if deferring any tests)
- Phasing: enslavement + table isolation first; VRF-bound services and WireGuard-in-VRF fwmark in follow-up sub-specs.

## Files to Modify
- `internal/component/iface/yang/ze-iface-conf.yang` - top-level VRF defs; tighten the `vrf` leaf
- `internal/component/iface/` - parse VRF defs + membership; validation
- `internal/plugins/iface/netlink/` - VRF device creation + enslavement
- `internal/component/sysrib/` / `internal/plugins/policyroute/` - per-VRF table routing

## Files to Create
- `internal/plugins/iface/netlink/vrf_linux.go` - VRF device + enslavement
- `internal/plugins/iface/netlink/vrf_test.go` - unit tests
- `test/qemu/vrf-enslave.ci` - QEMU functional test

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file (skeleton — run `/ze-spec` RESEARCH/DESIGN first) |

### Implementation Phases
1. **RESEARCH/DESIGN (not started)** — full `/ze-spec` workflow: netlink VRF plumbing, table model in sysrib, policyroute coordination, QEMU test design, phasing. Not implementable as-is.

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

## Known Limitations
- Skeleton only: acceptance criteria and tests are provisional placeholders for DESIGN.
- WireGuard-in-VRF fwmark rules depend on this base VRF work; tracked here as AC-4, likely a follow-up sub-spec.

## Implementation Summary
### What Was Implemented
- Nothing yet (skeleton).

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
<!-- Add a new block per re-run. Final run MUST show zero BLOCKER/ISSUE. -->
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Goal Gates (MUST pass)
- [ ] Full `/ze-spec` DESIGN completed and approved before implementation
- [ ] QEMU table-isolation test passes
- [ ] `./le verify current mode full` passes (after implementation)
- [ ] Feature code integrated (`internal/*`)

### Quality Gates (SHOULD pass)
- [ ] Interface/routing docs updated

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior (QEMU)

### Post-wave corrections (2026-07-10)

The planned `internal/plugins/iface/netlink/vrf_linux.go` (Files to Create) now falls inside two gates the followup wave added or widened:

- `ze-platform-vet` (`internal/le/` native action tables; live stage at `internal/le/verify/run.go`, `:141`) vets `./internal/plugins/iface/...` under GOOS=darwin and GOOS=freebsd. The `_linux.go` suffix excludes the file itself from those builds, but any VRF symbol referenced from non-suffixed backend code needs an `_other.go` counterpart (matching the existing `default_other.go`/`backend_other.go` pattern) or the vet fails.
- `ze-iface-resolution-check` (`internal/le/` native action tables; gate source `internal/le/ifaceresolution/ifaceresolution.go`) forbids direct kernel name resolution (`LinkByName`, `net.InterfaceByName`, SIOCGIFINDEX) outside an allowlist. `internal/plugins/iface/netlink/` is allowlisted as "the single kernel owner" (`internal/le/ifaceresolution/ifaceresolution.go`), so VRF device creation and enslavement code placed there is covered by the existing entry. However, any VRF-related resolution added OUTSIDE that tree (e.g. in the sysrib/policyroute table integration this spec plans) must go through the shared resolver (`internal/component/iface/resolve.go` Resolve, `:71` Addresses, `:80` Subscribe) or the gate fails.
