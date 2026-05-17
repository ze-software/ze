# Spec: Named Interface Units

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | spec-iface-1-per-family-address |
| Phase | - |
| Updated | 2026-05-17 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/iface/config.go` - unitEntry struct, parseUnits
4. `internal/component/iface/schema/ze-iface-conf.yang` - `list unit` definition

## Task

Currently interface units are keyed by a numeric ID:

```
interface eth0 {
    unit 0 {
        ipv4 { address [ 10.0.0.1/24 ]; }
    }
    unit 100 {
        vlan-id 100
        ipv4 { address [ 10.0.100.1/24 ]; }
    }
}
```

The numeric ID has no operational meaning beyond being a list key. The VLAN ID is
already a separate `vlan-id` leaf. In practice, operators want to label units by
their purpose: the service using the interface, a supplier name, a deployment role.

Change the unit key from a numeric ID to a freeform name:

```
interface eth0 {
    unit default {
        ipv4 { address [ 10.0.0.1/24 ]; }
    }
    unit firewall-3 {
        vlan-id 100
        ipv4 { address [ 10.0.100.1/24 ]; }
    }
    unit supplier-acme {
        vlan-id 200
        ipv6 { address [ 2001:db8::1/64 ]; }
    }
}
```

The name is informational documentation embedded in the config. It appears in
`show interface`, logs, and events, making it clear which service or purpose
each sub-interface serves without consulting external docs.

## Vendor Reference

| Vendor | Unit naming |
|--------|-------------|
| Junos | Numeric unit number (0-16385), tied to subinterface index |
| Arista | No explicit unit; sub-interfaces are `Ethernet1.100` (VLAN-based) |
| VyOS | No explicit unit; VLANs are `eth0 vif 100` (numeric) |
| Nokia | Named sub-interfaces: `subinterface "mgmt"`, `subinterface "to-spine-1"` |
| Ze (proposed) | Named units, following Nokia model |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - interface component design
- [ ] `docs/features/interfaces.md` - current unit model

### RFC Summaries (MUST for protocol work)
- Not applicable (unit naming is operational, not protocol-level)

**Key insights:**
- `unitEntry.ID int` is the current key, parsed from the YANG list key
- `unitEntry.VLANID int` is already separate (used for VLAN subinterface creation)
- `dhcpUnitKey{ifaceName, unit int}` references unit by numeric ID
- OS subinterface name is `ifaceName.vlanID` (e.g., `eth0.100`), not `ifaceName.unitID`
- The unit ID is not used in netlink operations; only the VLAN ID matters for subinterfaces

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/iface/config.go` - `unitEntry{ID int, ...}`, `parseUnits` uses `strconv.Atoi(idStr)`
- [ ] `internal/component/iface/schema/ze-iface-conf.yang` - `list unit { key "id"; leaf id { type uint32 } }`
- [ ] `internal/component/iface/register.go` - `dhcpUnitKey{ifaceName, unit int}`
- [ ] `internal/component/iface/config_apply.go` - iterates units, uses VLANID not ID for OS name
- [ ] `internal/component/iface/config_sysctl.go` - applies sysctls per unit

**Behavior to preserve:**
- VLAN subinterface creation uses `vlan-id`, not unit key
- OS interface naming: `ifaceName.vlanID` unchanged
- Address application, sysctl, mirror, DHCP all keyed by OS interface name
- Config reload detects changes by comparing unit lists

**Behavior to change:**
- YANG: `list unit { key "name"; leaf name { type string } }` instead of numeric ID
- Go: `unitEntry.ID int` becomes `unitEntry.Label string`
- Parser: `parseUnits` uses string key directly instead of `strconv.Atoi`
- DHCP unit key: change from `unit int` to `unit string`
- All log messages, error messages, events that reference unit ID

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- Config: `interface eth0 { unit firewall-3 { vlan-id 100; ... } }`

### Transformation Path
1. YANG validation: unit name validated by pattern
2. Config parse: `parseUnits` reads string key, stores as `Label`
3. Reconciliation: compare by OS name (unchanged, still `ifaceName.vlanID`)
4. Apply: netlink operations use OS name, not unit label

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config -> iface plugin | YANG-validated config tree | [ ] |
| iface plugin -> kernel | netlink (uses OS name, not label) | [ ] |
| iface plugin -> event bus | events include label for debugging | [ ] |
| iface plugin -> DHCP plugin | unit key updated to string | [ ] |

### Integration Points
- DHCP plugin: `dhcpUnitKey` struct changes from `unit int` to `unit string`
- Config migration (ExaBGP): must map numeric units to named equivalents
- `show interface`: display unit label instead of numeric ID
- CLI editor: unit completion shows labels
- Web UI: interface detail page shows labels

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality
- [ ] Zero-copy preserved where applicable

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config `unit firewall-3 { vlan-id 100; ... }` | -> | parser -> apply | `TestUnit_NamedKey` |
| Config `unit default { ... }` (no VLAN) | -> | parser -> apply | `TestUnit_DefaultNoVLAN` |
| Legacy numeric `unit 0 { ... }` | -> | migration parser | `TestUnit_LegacyNumeric` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config `unit firewall-3 { vlan-id 100; }` | Unit parsed with label "firewall-3", VLAN 100 |
| AC-2 | Config `unit default { }` | Unit parsed, no VLAN, works as base unit |
| AC-3 | Multiple named units | All parsed, OS names use vlan-id for subinterfaces |
| AC-4 | Duplicate unit names | Config rejected at verify |
| AC-5 | Unit name with special chars | Validated against pattern (lowercase, digits, hyphens) |
| AC-6 | `show interface` output | Displays unit label, not numeric ID |
| AC-7 | DHCP on named unit | DHCP client works with string-keyed unit |
| AC-8 | Legacy numeric unit names | Accepted for migration (numeric string is a valid name) |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseUnit_NamedKey` | `internal/component/iface/config_test.go` | Named unit parsed correctly | |
| `TestParseUnit_DefaultNoVLAN` | `internal/component/iface/config_test.go` | Base unit without VLAN | |
| `TestParseUnit_MultipleNamed` | `internal/component/iface/config_test.go` | Multiple units with labels | |
| `TestParseUnit_InvalidName` | `internal/component/iface/config_test.go` | Bad characters rejected | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Unit name length | 1..64 | 64 chars | 0 (empty) | 65 chars |
| VLAN ID | 1..4094 | 4094 | 0 | 4095 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-iface-named-units` | `test/plugin/iface-named-units.ci` | Configure named units, verify applied | |

### Future (if deferring any tests)
- None planned

## Files to Modify
- `internal/component/iface/config.go` - unitEntry.ID -> unitEntry.Label, parseUnits
- `internal/component/iface/schema/ze-iface-conf.yang` - list unit key change
- `internal/component/iface/register.go` - dhcpUnitKey
- `internal/component/iface/config_test.go` - update tests

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | Yes | `ze-iface-conf.yang` |
| Config migration | Yes | Accept old numeric keys |
| DHCP plugin | Yes | `dhcpUnitKey` struct |
| show interface | Yes | Display label |
| Web UI | Verify | Interface detail page |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Restructure, not new feature |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md`, `docs/features/interfaces.md` |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | No | Config reshuffling only |

## Files to Create
- None (all changes to existing files)

## Implementation Steps

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: YANG restructure** -- change unit key from `id` to `name`
   - Tests: `TestParseUnit_NamedKey`, `TestParseUnit_DefaultNoVLAN`
   - Files: `ze-iface-conf.yang`, `config.go`

2. **Phase: Name validation** -- enforce naming pattern
   - Tests: `TestParseUnit_InvalidName`
   - Files: `config.go`

3. **Phase: DHCP key update** -- change dhcpUnitKey to string
   - Tests: existing DHCP tests adapted
   - Files: `register.go`

4. **Phase: Legacy migration** -- accept numeric unit keys as valid names
   - Tests: `TestParseUnit_LegacyNumeric`
   - Files: `config.go`

5. **Phase: Show interface** -- display label
   - Tests: functional test
   - Files: `cmd/show/show.go`

6. **Phase: Full verification** -- `make ze-verify`

7. **Phase: Complete spec** -- audit, learned summary, commit

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Unit label is the YANG key, VLAN ID is separate |
| Naming | YANG key changes from `id` to `name`, Go field from `ID` to `Label` |
| Data flow | Parse -> struct -> reconcile -> netlink unchanged (OS name still uses VLAN ID) |
| Rule: config-design | Old numeric keys accepted as valid names |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| YANG `name` key on unit list | `grep "key" ze-iface-conf.yang` in unit section |
| Go unitEntry.Label field | `grep "Label" config.go` |
| DHCP key updated | `grep "dhcpUnitKey" register.go` |
| Legacy numeric accepted | Test passes |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Unit name pattern restricts to safe characters |
| No injection | Unit name never used in shell commands or SQL |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline |
| Functional test fails | Check AC |
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

## Implementation Summary

### What Was Implemented
- To be filled

### Bugs Found/Fixed
- To be filled

### Documentation Updates
- To be filled

### Deviations from Plan
- To be filled

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

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
-

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
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] Summary included in commit
