# Spec: bridge-disable-learning

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-07-04 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/component/iface/yang/ze-iface-conf.yang` - bridge `member` schema
4. `internal/plugins/iface/netlink/bridge_linux.go` - bridge netlink backend
5. `internal/component/iface/config.go` - `bridgeEntry` / `Members`

## Task

Ze cannot disable MAC address learning on an individual bridge member port. There
is no config leaf and no netlink code that toggles per-port learning (the Linux
`IFLA_BRPORT_LEARNING` attribute / `/sys/class/net/<ifname>/brport/learning`).
Disabling learning on a port is needed for setups that program forwarding entries
externally (EVPN, controller-driven L2), where dynamic learning must be off.

Add a per-bridge-member `disable-learning` option that sets the port's learning flag
off. This requires first giving bridge members a per-member config container, since
today `member` is a flat leaf-list with nowhere to attach a per-member option.

## Required Reading

### Architecture Docs
- [ ] `docs/features/interfaces.md` - bridge interface management (referenced by `bridge_linux.go`).
  → Constraint: bridge port state is set via netlink/sysfs on the port, keyed by the member interface name.
- [ ] `ai/rules/config.md` and `ai/rules/config.md` - the new per-member leaf.
  → Constraint: restructuring `member` from a leaf-list to a per-member container is a schema change; preserve existing member semantics.

**Key insights:**
- The blocker is schema shape: a flat `leaf-list member` has no per-member node. The learning leaf needs a per-member container (tagNode) first.
- Setting learning is a single netlink attribute (`IFLA_BRPORT_LEARNING`) or a sysfs write, parallel to how STP is set today.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/iface/netlink/bridge_linux.go` - backend implements only `BridgeAddPort` (bridge_linux.go, `LinkSetMaster`), `BridgeDelPort` (:46, `LinkSetNoMaster`), and `BridgeSetSTP` (:60, sysfs `bridge/stp_state`). No per-port learning attribute is set anywhere.
- [ ] `internal/component/iface/yang/ze-iface-conf.yang` - bridge exposes `name`, `stp` (boolean), and `member` as a bare `leaf-list` of interface-name strings (ze-iface-conf.yang); there is no per-member container, so no place for a `learning` leaf.
- [ ] `internal/component/iface/config.go` - `bridgeEntry` / `Members` model the flat membership (config.go).

**Behavior to preserve:**
- Existing bridge membership (add/remove ports) and STP behaviour unchanged.
- Default learning stays ON (kernel default) when the option is absent.
- Member interface-name semantics preserved after the schema change.

**Behavior to change:**
- `member` gains a per-member container carrying a `disable-learning` option; the backend sets the port learning flag accordingly.

## Data Flow (MANDATORY)

### Entry Point
- Config: per-member `disable-learning` leaf under a new per-member container in the bridge schema (`ze-iface-conf.yang`).

### Transformation Path
1. Bridge `member` schema changes from a flat leaf-list to a per-member container keyed by interface name, carrying `disable-learning`.
2. `bridgeEntry`/`Members` parse the per-member option into the config model.
3. On apply, after the port is enslaved (`BridgeAddPort`), the backend sets the port learning flag: off when `disable-learning` is set, on (default) otherwise.
4. The flag is written via netlink `IFLA_BRPORT_LEARNING` (preferred) or sysfs `brport/learning`, parallel to `BridgeSetSTP`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config ↔ iface model | per-member container → `Members` with learning flag | [ ] |
| Model ↔ netlink backend | new `BridgeSetPortLearning` sets the port flag | [ ] |
| Backend ↔ kernel | `IFLA_BRPORT_LEARNING` / sysfs brport/learning | [ ] |

### Integration Points
- `ze-iface-conf.yang` bridge block - restructure `member`, add `disable-learning`.
- `internal/component/iface/config.go` - parse the per-member option.
- `internal/plugins/iface/netlink/bridge_linux.go` - new `BridgeSetPortLearning` alongside `BridgeSetSTP`.

### Architectural Verification
- [ ] No bypassed layers (learning set through the netlink backend, like STP)
- [ ] No unintended coupling (per-port state keyed by member name)
- [ ] No duplicated functionality (mirror the STP setter pattern)
- [ ] Registration over hardcoding — bridge member handling stays in the iface component/backend; no central change.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The netlink library exposes `IFLA_BRPORT_LEARNING` (else sysfs fallback) | `vishvananda/netlink` used for bridges | use sysfs `brport/learning` like STP | check the netlink lib API during audit | unvalidated |
| A-2 | Restructuring `member` to a container preserves existing configs via migration | schema change | existing configs break | write a config migration + test | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Schema change breaks existing flat-list bridge configs | commit fails on old config | migration converts leaf-list to container; functional test with an old config |
| R-2 | Learning flag set before enslavement has no effect | flag not applied | order: enslave, then set learning (as STP is ordered) |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `set interfaces bridge br0 member eth1 disable-learning` | → | `BridgeSetPortLearning(eth1, off)` after enslave | `test/qemu/bridge-disable-learning.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | member with `disable-learning` | port `brport/learning` is 0 |
| AC-2 | member without the option | port learning stays 1 (default) |
| AC-3 | toggle `disable-learning` off again | port learning returns to 1 |
| AC-4 | existing flat-list bridge config (pre-migration) | migrates cleanly, membership preserved |
| AC-5 | non-member interface | option not applicable / rejected |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | disables learning on a controller-managed bridge port | config → per-member model → `BridgeSetPortLearning` | `test/qemu/bridge-disable-learning.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBridgeMemberLearningParse` | `internal/component/iface/config_test.go` | per-member `disable-learning` parsed | |
| `TestBridgeSetPortLearning` | `internal/plugins/iface/netlink/bridge_linux_test.go` | sysfs/netlink learning flag written (sysfs root override) | |
| `TestBridgeMemberMigration` | `internal/component/iface/..._test.go` | flat leaf-list migrates to container | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| disable-learning | boolean (valueless) | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bridge-disable-learning` | `test/qemu/bridge-disable-learning.ci` | member learning toggled, verified via sysfs | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A - kernel bridge feature; validated by QEMU functional test | - | - | learning flag is a kernel behaviour | - |

### Future (if deferring any tests)
- None planned.

## Files to Modify
- `internal/component/iface/yang/ze-iface-conf.yang` - restructure `member` to a per-member container; add `disable-learning`
- `internal/component/iface/config.go` - parse per-member option into `Members`
- `internal/plugins/iface/netlink/bridge_linux.go` - add `BridgeSetPortLearning`
- config migration - convert existing flat `member` leaf-list to the container form

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (changed config) | [ ] yes | `ze-iface-conf.yang` bridge member container; `ai/rules/config.md` |
| Config migration | [ ] yes | leaf-list → container migration + test |
| Functional test for new behaviour | [ ] yes | `test/qemu/bridge-disable-learning.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes | `docs/features.md` |
| 2 | Config syntax changed? | [ ] yes | `docs/guide/configuration.md`, `docs/features/interfaces.md` |

## Files to Create
- `test/qemu/bridge-disable-learning.ci` - QEMU functional test
- (unit tests extend existing test files)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |

### Implementation Phases
1. **Phase: Wiring (MANDATORY FIRST)** — restructure `member` to a per-member container + migration; parse (unused) `disable-learning`; failing `test/qemu/bridge-disable-learning.ci`.
2. **Phase: Backend setter** — add `BridgeSetPortLearning` (sysfs root override for tests), call it after enslavement.
   - Tests: `TestBridgeSetPortLearning`, `TestBridgeMemberLearningParse`
3. **Phase: Migration** — convert existing flat configs.
   - Tests: `TestBridgeMemberMigration`
4. **Functional test (QEMU)**
5. **Full verification** → `make ze-verify`
6. **Complete spec** → audit, learned summary, two-commit closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N implemented with file:line |
| Correctness | learning set after enslave; default preserved; migration lossless |
| YANG validation | per-member container keyed correctly; `disable-learning` valueless |
| Registration over hardcoding | change confined to iface component/backend |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| learning setter | `go test ./internal/plugins/iface/netlink -run Learning` |
| migration | `go test ./internal/component/iface -run Migration` |
| QEMU | `test/qemu/bridge-disable-learning.ci` verifies sysfs value |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | member interface name validated (as add-port already does) |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

## Design Insights
<!-- LIVE -->

## Implementation Summary
### What Was Implemented
- (fill during implementation)

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
- [ ] AC-1..AC-5 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and passing test
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated (`internal/*`)
- [ ] Documentation Update Checklist answered

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (boolean)
- [ ] Functional tests for end-to-end behavior (QEMU)

### Post-wave corrections (2026-07-10)

Core evidence re-verified current after the followup-vpp-iface wave; no design change, but the member restructure now lands in a file set with NEW guard tests:

- Verified-current evidence: `bridgeEntry` at `internal/component/iface/config.go` with `Members` at :178 (the spec's :174-178 range still holds); YANG `list bridge` at `internal/component/iface/yang/ze-iface-conf.yang` with `leaf-list member` at :608 (the spec's :586-614 range still holds). The bridge list remains `ze:backend "netlink"` (ze-iface-conf.yang), untouched by the wave's VPP gate widening, so this spec stays netlink-only.
- NEW constraints from the wave the implementation must satisfy:
  - The iface YANG schema snapshot test widened during the wave; restructuring `member` from a leaf-list to a per-member container changes the schema and must update/pass `internal/component/iface/schema_test.go`.
  - The VPP backend gate test `internal/component/iface/backend_gate_vpp_test.go` asserts which iface features are permitted per backend; the new per-member `disable-learning` leaf must be classified there (netlink-only) so a vpp-backend config is rejected cleanly rather than silently ignored.
  - The wave added migration tests in `internal/component/iface/migrate_linux_test.go`; the planned flat-leaf-list-to-container migration (A-2/R-1) must coexist with those and follow the same test pattern.
