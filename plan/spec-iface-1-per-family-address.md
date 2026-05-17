# Spec: Per-Family Address Configuration

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 6/7 |
| Updated | 2026-05-17 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/iface/config.go` - unitEntry.Addresses []string, parseUnit
4. `internal/component/iface/schema/ze-iface-conf.yang` - `leaf-list address` at unit level
5. `internal/component/iface/schema/ze-iface-conf.yang` - `container ipv4` / `container ipv6`

## Task

Currently interface addresses are a flat list at the unit level mixing IPv4 and IPv6:

```
interface eth0 {
    unit 0 {
        address [ 10.0.0.1/24 fd00::1/64 ];
    }
}
```

This is inconsistent with the per-family `ipv4 {}` / `ipv6 {}` containers that already exist
for protocol knobs (forwarding, rp-filter, etc). It also makes it harder to express
family-specific semantics (e.g., IPv4 secondary vs primary, IPv6 eui-64 auto-derivation).

Move addresses into the per-family containers:

```
interface eth0 {
    unit 0 {
        ipv4 {
            address [ 10.0.0.1/24 10.0.0.2/24 ];
            forwarding true;
            rpf-check strict;
        }
        ipv6 {
            address [ fd00::1/64 ];
            forwarding true;
        }
    }
}
```

The old flat `address [ ... ]` syntax is removed. No backward compatibility.

## Vendor Reference

| Vendor | Address placement |
|--------|-------------------|
| Junos | `family inet { address 10.0.0.1/24; }` / `family inet6 { address fd00::1/64; }` |
| Arista | `ip address 10.0.0.1/24` / `ipv6 address fd00::1/64` (flat, but family-prefixed) |
| IOS-XR | `ipv4 address 10.0.0.1 255.255.255.0` / `ipv6 address fd00::1/64` |
| VyOS | `set interfaces ethernet eth0 address 10.0.0.1/24` (flat, auto-detected) |
| Nokia | `interface "x" { ipv4 { primary { address ... } } ipv6 { address ... } }` |

Ze follows the Junos/Nokia model: addresses live inside the family container.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - interface component design

### RFC Summaries (MUST for protocol work)
- Not applicable (address assignment is operational, not protocol-level)

**Key insights:**
- `unitEntry.Addresses []string` holds mixed v4+v6 as text CIDRs
- Addresses are applied via netlink (`AddrAdd`/`AddrDel`) in the iface plugin
- The connected-routes plugin (`internal/plugins/connected/`) subscribes to address events
- DHCP and DHCPv6 also assign addresses; they bypass static config but coexist
- The `address` leaf-list uses `ze:syntax "bracket"` for `[ ... ]` list notation

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/iface/config.go` - `unitEntry.Addresses []string` (flat list)
  -> Constraint: `parseStringList(um, "address")` at line 707
- [ ] `internal/component/iface/schema/ze-iface-conf.yang` - `leaf-list address` under unit grouping
  -> Constraint: type is string with pattern `[0-9a-fA-F:./]+`; accepts both v4 and v6
- [ ] `internal/plugins/connected/` - watches address add/remove events for redistribute

**Behavior to preserve:**
- Addresses applied via netlink (AddrAdd/AddrDel) unchanged
- Connected-routes plugin sees same address events regardless of config location
- DHCP/DHCPv6 address assignment unaffected (they don't use static config)
- Multiple addresses per family supported (leaf-list)

**Behavior to change:**
- Move `leaf-list address` from unit level into `container ipv4` and `container ipv6`
- `unitEntry.Addresses []string` split into `ipv4Sysctl.Addresses []string` and `ipv6Sysctl.Addresses []string` (or separate fields on unitEntry keyed by family)
- Old flat `address [ ... ]` accepted at parse time: v4 CIDRs -> ipv4.address, v6 CIDRs -> ipv6.address
- Config migration from ExaBGP format must also target new structure

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- Config: `interface eth0 { unit 0 { ipv4 { address [ 10.0.0.1/24 ]; } } }`
- Legacy config: `interface eth0 { unit 0 { address [ 10.0.0.1/24 fd00::1/64 ]; } }`

### Transformation Path
1. YANG validation: address strings validated by pattern
2. Config parse: `parseIPv4Sysctl` reads `address` leaf-list -> `Addresses` field
3. Reconciliation: compare desired addresses vs current OS state
4. Apply: netlink AddrAdd/AddrDel for differences
5. Event emission: connected-routes plugin notified of changes

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config -> iface plugin | YANG-validated config tree | [ ] |
| iface plugin -> kernel | netlink AddrAdd/AddrDel | [ ] |
| iface plugin -> event bus | address add/remove events | [ ] |
| event bus -> connected plugin | redistribute route changes | [ ] |

### Integration Points
- Connected-routes plugin: subscribes to address events (must still work)
- DHCP plugin: assigns addresses independently (must not conflict with static)
- Config migration (ExaBGP): must produce new family-scoped format
- `show interface`: must display addresses grouped by family

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling (address source doesn't change, only config location)
- [ ] No duplicated functionality
- [ ] Zero-copy preserved where applicable

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config `ipv4 { address [ 10.0.0.1/24 ]; }` | -> | parser -> apply -> netlink | `TestAddress_IPv4InFamily` |
| Config `ipv6 { address [ fd00::1/64 ]; }` | -> | parser -> apply -> netlink | `TestAddress_IPv6InFamily` |
| Legacy `address [ 10.0.0.1/24 fd00::1/64 ]` | -> | migration parser -> split | `TestAddress_LegacyMigration` |
| `show interface eth0` | -> | displays per-family addresses | `TestShowInterface_FamilyAddresses` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config `ipv4 { address [ 10.0.0.1/24 ]; }` | Address applied to interface via netlink |
| AC-2 | Config `ipv6 { address [ fd00::1/64 ]; }` | Address applied to interface via netlink |
| AC-3 | Both families configured | Both addresses applied |
| AC-4 | Flat `address [ ... ]` at unit level | Ignored (YANG leaf removed, not supported) |
| AC-5 | IPv4 address in `ipv6 {}` container | Config rejected at verify (wrong family) |
| AC-6 | IPv6 address in `ipv4 {}` container | Config rejected at verify (wrong family) |
| AC-7 | Config reload adds address | New address applied without removing existing |
| AC-8 | Config reload removes address | Old address removed |
| AC-9 | Connected-routes plugin receives events | Same behavior as before for redistribute |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseAddress_IPv4InFamily` | `internal/component/iface/config_test.go` | Parses v4 address from ipv4 container | |
| `TestParseAddress_IPv6InFamily` | `internal/component/iface/config_test.go` | Parses v6 address from ipv6 container | |
| `TestParseAddress_LegacyFlat` | `internal/component/iface/config_test.go` | Old flat list split by family | |
| `TestParseAddress_WrongFamily` | `internal/component/iface/config_test.go` | v4 in ipv6 container rejected | |
| `TestParseAddress_Multiple` | `internal/component/iface/config_test.go` | Multiple addresses per family | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| IPv4 prefix | /0 - /32 | 10.0.0.1/32 | N/A | /33 rejected by netip |
| IPv6 prefix | /0 - /128 | fd00::1/128 | N/A | /129 rejected by netip |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-iface-family-address` | `test/plugin/iface-family-address.ci` | Configure per-family addresses, verify applied | |

### Future (if deferring any tests)
- None planned

## Files to Modify
- `internal/component/iface/config.go` - move Addresses into per-family structs; update parser
- `internal/component/iface/schema/ze-iface-conf.yang` - move `leaf-list address` into ipv4/ipv6 containers
- `internal/component/iface/config_test.go` - new tests

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | Yes | `ze-iface-conf.yang` |
| Config migration | Yes | Accept old `address` at unit level |
| Connected-routes plugin | Verify | Must still receive same events |
| show interface | Yes | Display addresses grouped by family |
| ExaBGP migration | Yes | `internal/exabgp/migration/` |

## Files to Create
- None (all changes to existing files)

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Restructure, not new feature |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` - per-family address |
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

## Implementation Steps

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: YANG restructure** -- move `leaf-list address` into ipv4/ipv6 containers
   - Tests: `TestParseAddress_IPv4InFamily`, `TestParseAddress_IPv6InFamily`
   - Files: `ze-iface-conf.yang`, `config.go`

2. **Phase: Family validation** -- reject wrong-family addresses
   - Tests: `TestParseAddress_WrongFamily`
   - Files: `config.go`

3. **Phase: Legacy migration** -- accept old flat `address` list, split by family
   - Tests: `TestParseAddress_LegacyFlat`
   - Files: `config.go`

4. **Phase: Apply path** -- ensure netlink apply works with new struct layout
   - Tests: functional test
   - Files: apply logic in iface plugin

5. **Phase: Functional test** -- .ci test
   - Files: `test/plugin/iface-family-address.ci`

6. **Phase: Full verification** -- `make ze-verify`

7. **Phase: Complete spec** -- audit, learned summary, commit

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | v4 addresses only in ipv4 container, v6 only in ipv6 |
| Naming | YANG leaf-list stays `address` (same name, new location) |
| Data flow | Parse -> struct -> reconcile -> netlink unchanged |
| Rule: config-design | Old syntax accepted with warning |
| Rule: wiring-completeness | Addresses parsed AND applied AND events emitted |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| YANG `address` in ipv4 container | `grep "address" ze-iface-conf.yang` in ipv4 section |
| YANG `address` in ipv6 container | Same, in ipv6 section |
| Legacy migration path | `grep "address" config.go` at unit level still parsed |
| Family validation | Test for wrong-family rejection passes |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | CIDR pattern validated by YANG + netip.ParsePrefix in Go |
| Privilege | Address assignment requires CAP_NET_ADMIN; Ze already holds it |

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
- [ ] AC-1..AC-9 all demonstrated
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
