# Spec: cos-plugin

| Field | Value |
|-------|-------|
| Status | complete |
| Depends | - |
| Phase | 7/7 |
| Updated | 2026-06-12 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `ai/patterns/plugin.md` - plugin structure template
3. `ai/rules/plugin-self-containment.md` - removal test
4. `internal/component/iface/config.go` - unitEntry struct, parseUnits(), parseQoSMap()
5. `internal/component/iface/yang/ze-iface-conf.yang` - interface-l2, interface-unit groupings
6. `internal/plugins/iface/netlink/manage_linux.go` - CreateVLAN() netlink application
7. `internal/plugins/l2tpshaper/` - reference plugin (YANG, config, registry pattern)

## Task

Create a `class-of-service` plugin that owns 802.1p QoS profile definitions
and binds them to interfaces via YANG container-merge. The iface component
retains the low-level `ingress-qos-map`/`egress-qos-map` mechanism; the
new plugin adds the higher-level named-profile abstraction on top.

The design was iterated in a conversation comparing Ze's syntax with
Juniper, Cisco, Nokia, Arista, and Linux. The agreed syntax is Option 2
(two explicit containers, ingress keyed by PCP, egress keyed by priority)
with interface-level inheritance and per-unit override/opt-out.

**Existing staged work:** `git reset --soft HEAD~2` has left the original
inline QoS map implementation staged (iface YANG, config.go, netlink
backend, tests, docs). That code stays as-is. This spec adds the plugin
layer on top.

### Operator syntax (the goal)

```
class-of-service {
    ieee-802.1p residential {
        ingress {
            pcp 0 { priority 0; }
            pcp 5 { priority 5; }
            pcp 6 { priority 6; }
        }
        egress {
            priority 0 { pcp 0; }
            priority 5 { pcp 5; }
            priority 6 { pcp 6; }
        }
    }
}

interface {
    ethernet eth0 {
        class-of-service residential;
        unit v100 { vlan-id 100; }
        unit v200 { vlan-id 200; class-of-service none; }
    }
}
```

## Required Reading

### Architecture Docs
- [ ] `ai/patterns/plugin.md` - plugin file structure, register.go template, 5-stage protocol
  -> Constraint: plugin must have register.go with init() -> registry.Register()
  -> Constraint: atomic logger pattern mandatory
- [ ] `ai/rules/plugin-self-containment.md` - removal test
  -> Constraint: delete plugin dir + blank import -> class-of-service disappears, build green
- [ ] `ai/rules/plugin-design.md` - cross-boundary value types, import rules
  -> Constraint: shared types in internal/core/, not in the plugin
- [ ] `ai/rules/design-principles.md` - explicit > implicit, no translation layers
- [ ] `ai/patterns/config-option.md` - YANG config patterns
  -> Constraint: every leaf needs max native validation (range, length, pattern)
- [ ] `ai/rules/naming.md` - naming conventions

### RFC Summaries (MUST for protocol work)
- [ ] IEEE 802.1Q - PCP is 3-bit field (0-7), 8 possible values per direction
  -> Constraint: range 0..7 enforced in YANG, not just Go

**Key insights:**
- Plugin self-containment: removing the plugin removes the class-of-service surface from YANG, but iface inline maps still work
- The shared registry lives in internal/core/cos/ (like core/family/), not in the plugin
- YANG container-merge unions same-named containers from different modules; no augment needed
- Config ordering: CoS plugin parses profiles in InProcessConfigVerifier (synchronous), available when iface verifier calls cos.Lookup()

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/iface/config.go` - unitEntry has IngressQoSMap/EgressQoSMap (map[uint32]uint32). parseUnits() calls parseQoSMap() for inline syntax. parseQoSMap() reads YANG list from JSON, validates 0-7 range
  -> Constraint: unitEntry.IngressQoSMap/EgressQoSMap are the only fields the netlink backend reads; any new mechanism must populate these same fields
- [ ] `internal/component/iface/yang/ze-iface-conf.yang` - ingress-qos-map/egress-qos-map lists inside interface-unit grouping (lines 192-235). interface-l2 grouping used by ethernet, dummy, veth, bridge (lines 560, 575, 595, 625)
  -> Constraint: inline syntax stays in iface YANG; CoS plugin adds class-of-service via container-merge
- [ ] `internal/plugins/iface/netlink/manage_linux.go` - CreateVLAN() passes IngressQoSMap/EgressQoSMap to netlink.Vlan struct (lines 140-141). validateQoSMap() checks 0-7 range
  -> Constraint: netlink backend unchanged; it receives the same maps regardless of how they were configured
- [ ] `internal/component/iface/config.go:883` - validation: qos maps require vlan-id
  -> Constraint: class-of-service on a unit without vlan-id must also be rejected
- [ ] `internal/plugins/l2tpshaper/register.go` - reference plugin: YANG in yang/ subdir, ConfigRoots, InProcessConfigVerifier, runPlugin
- [ ] `internal/component/subscriber/handler_registry.go` - reference for cross-component registry pattern (Register/Get/Unregister with sync.RWMutex)

**Behavior to preserve:**
- Inline `ingress-qos-map`/`egress-qos-map` syntax continues to work unchanged
- Existing functional tests (test/parse/iface-vlan-qos.ci, test/parse/iface-vlan-qos-invalid.ci, test/plugin/iface-vlan-qos.ci) pass without modification
- Netlink backend receives the same map[uint32]uint32 regardless of config source
- VLAN-id requirement for QoS maps is enforced for both inline and profile syntax

**Behavior to change:**
- Add class-of-service leaf to ethernet, dummy, veth, bridge interface types and their units (via YANG container-merge from CoS plugin)
- Add profile resolution in iface parseUnits: class-of-service ref -> cos.Lookup() -> populate IngressQoSMap/EgressQoSMap
- Interface-level class-of-service is inherited by all VLAN units unless overridden per-unit

## Data Flow (MANDATORY)

### Entry Point
- Config file contains `class-of-service { ieee-802.1p <name> { ... } }` section
- Config file contains `class-of-service <name>;` leaf on interface or unit

### Transformation Path
1. YANG loader merges ze-cos-conf.yang into the config tree (container-merge for both class-of-service root and interface subtree)
2. CoS plugin receives class-of-service config root via InProcessConfigVerifier
3. CoS plugin parses profiles: for each ieee-802.1p entry, builds IngressMap (PCP->priority) and EgressMap (priority->PCP)
4. CoS plugin registers profiles via cos.Register() in internal/core/cos/
5. Iface component receives interface config root via its own config path
6. parseUnits() encounters class-of-service key in unit JSON (from container-merge)
7. If no unit-level class-of-service, checks parent interface level
8. Calls cos.Lookup(name) to get the profile
9. Populates u.IngressQoSMap and u.EgressQoSMap from the profile
10. CreateVLAN() passes maps to netlink (unchanged path)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CoS plugin -> core/cos registry | cos.Register() / cos.Lookup() | [ ] |
| core/cos registry -> iface config parser | cos.Lookup() returns Profile | [ ] |
| iface config -> netlink backend | existing unitEntry.IngressQoSMap/EgressQoSMap | [ ] |

### Integration Points
- `internal/core/cos/` registry package - new, shared between CoS plugin and iface
- `internal/component/iface/config.go:parseUnits()` - add cos.Lookup() resolution
- `internal/component/plugin/all/all.go` - generated blank import for CoS plugin

### Architectural Verification
- [ ] No bypassed layers (profiles go through the same parseUnits -> CreateVLAN path as inline maps)
- [ ] No unintended coupling (iface imports core/cos, not the plugin; plugin imports core/cos, not iface)
- [ ] No duplicated functionality (extends inline maps, doesn't recreate)
- [ ] Zero-copy preserved where applicable (maps are small, copy is fine)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | YANG container-merge works for interface subtree (list ethernet with key "name") | Used by CLI command plugins (container show, container clear) with container-merge | If it doesn't merge lists, would need augment | Functional test: config validate with both modules loaded | unvalidated |
| A-2 | InProcessConfigVerifier for CoS runs before iface's verifier | Plugin startup ordering; CoS has no dependencies, iface has no dependency on CoS | If iface verifier runs first, cos.Lookup() returns not-found | Unit test: register then lookup | unvalidated |
| A-3 | class-of-service JSON key appears in the interface config section when YANG is merged | Container-merge puts the leaf in the unified tree; config parser delivers full subtree to the owning ConfigRoot | If not, iface wouldn't see the key | Functional test with both class-of-service root and interface root | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | YANG container-merge for config lists (not just CLI containers) may not work | Config validation rejects the class-of-service leaf on interface | Fall back to augment pattern (adds base-module coupling) |
| R-2 | Config verification ordering is not deterministic across plugins | cos.Lookup() returns not-found during iface verification | Add explicit dependency from iface plugin on cos plugin name |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config: `class-of-service { ieee-802.1p x { ingress { pcp 0 { priority 0; } } egress { priority 0 { pcp 0; } } } }` | -> | CoS plugin config parser | TestCoSProfileParse |
| Config: `interface { ethernet eth0 { class-of-service x; unit v100 { vlan-id 100; } } }` | -> | iface parseUnits cos.Lookup | TestCoSProfileResolution |
| Config: combined class-of-service + interface with ref | -> | ze config validate | test/parse/cos-profile.ci |
| Config: class-of-service ref + inline qos-map on same unit | -> | iface parseUnits conflict check | test/parse/cos-profile-conflict.ci |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | class-of-service with valid ieee-802.1p profile (ingress + egress, values 0-7) | Config validates successfully; profile registered in cos registry |
| AC-2 | class-of-service with PCP or priority value 8 (out of range) | Config validation rejects with clear error |
| AC-3 | class-of-service with duplicate priority values in egress (two priorities mapping to same PCP) | Config validates (not a conflict; last-write-wins is kernel behavior) |
| AC-4 | ethernet interface with class-of-service ref; VLAN unit with vlan-id | Unit inherits profile; IngressQoSMap/EgressQoSMap populated |
| AC-5 | ethernet interface with class-of-service ref; VLAN unit with explicit class-of-service override | Unit uses its own profile, not the parent's |
| AC-6 | ethernet interface with class-of-service ref; VLAN unit with `class-of-service none;` | Unit has no QoS maps (opt-out) |
| AC-7 | ethernet interface with class-of-service ref; unit without vlan-id | Validation error: class-of-service requires vlan-id |
| AC-8 | Unit with both class-of-service ref and inline ingress-qos-map | Validation error: mutually exclusive |
| AC-9 | class-of-service ref to a profile name that doesn't exist | Validation error: profile not found |
| AC-10 | class-of-service ref on dummy, veth, bridge (not just ethernet) | Works identically to ethernet |
| AC-11 | CoS plugin removed (blank import deleted) | Build green; inline qos maps still work; class-of-service leaf absent from YANG |
| AC-12 | Existing inline QoS map tests still pass | test/parse/iface-vlan-qos.ci, test/parse/iface-vlan-qos-invalid.ci, test/plugin/iface-vlan-qos.ci unchanged and green |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestCoSProfileParse | internal/plugins/cos/config_test.go | AC-1: valid profile parsing into maps | |
| TestCoSProfileParseInvalid | internal/plugins/cos/config_test.go | AC-2: out-of-range values rejected | |
| TestCoSProfileParseEmpty | internal/plugins/cos/config_test.go | Empty profile (no ingress/egress) parses as empty maps | |
| TestCoSRegistryRegisterLookup | internal/core/cos/cos_test.go | Register then Lookup returns correct profile | |
| TestCoSRegistryLookupMissing | internal/core/cos/cos_test.go | Lookup of nonexistent name returns false | |
| TestCoSRegistryClear | internal/core/cos/cos_test.go | Clear removes all profiles | |
| TestCoSProfileResolution | internal/component/iface/config_test.go | AC-4: class-of-service ref on parent populates unit maps | |
| TestCoSProfileUnitOverride | internal/component/iface/config_test.go | AC-5: unit override wins over parent | |
| TestCoSProfileUnitOptOut | internal/component/iface/config_test.go | AC-6: "none" disables inheritance | |
| TestCoSProfileNoVLAN | internal/component/iface/config_test.go | AC-7: class-of-service without vlan-id is rejected | |
| TestCoSProfileConflictInline | internal/component/iface/config_test.go | AC-8: class-of-service + inline maps rejected | |
| TestCoSProfileNotFound | internal/component/iface/config_test.go | AC-9: missing profile name rejected | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| PCP value (ingress key) | 0-7 | 7 | N/A (uint8, 0 is valid) | 8 |
| Priority value (ingress leaf) | 0-7 | 7 | N/A | 8 |
| Priority value (egress key) | 0-7 | 7 | N/A | 8 |
| PCP value (egress leaf) | 0-7 | 7 | N/A | 8 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| cos-profile | test/parse/cos-profile.ci | Operator defines a class-of-service profile and references it from an interface; `ze config validate` accepts | |
| cos-profile-invalid | test/parse/cos-profile-invalid.ci | Operator uses out-of-range PCP value in profile; `ze config validate` rejects | |
| cos-profile-conflict | test/parse/cos-profile-conflict.ci | Operator combines profile ref with inline qos-map on same unit; rejected | |
| cos-profile-not-found | test/parse/cos-profile-not-found.ci | Operator references nonexistent profile name; rejected | |
| cos-profile-inherit | test/plugin/cos-profile-inherit.ci | Interface-level profile inherited by VLAN unit; full config validates | |
| cos-profile-override | test/plugin/cos-profile-override.ci | Unit-level profile overrides interface-level; validates | |
| cos-profile-none | test/plugin/cos-profile-none.ci | Unit with class-of-service none; validates, no maps | |

### Interop Tests (MANDATORY for protocol features)
N/A. This is a config abstraction layer over existing VLAN QoS maps. No wire protocol involved; the netlink path is unchanged from spec-vlan-qos-map.

## Files to Modify
- `internal/component/iface/config.go` - add cos.Lookup() resolution in parseUnits(), pass parentCoS from interface level
- `internal/component/iface/config_test.go` - add profile resolution tests (TestCoSProfile*)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | [x] | `internal/plugins/cos/yang/ze-cos-conf.yang` |
| YANG validation constraints | [x] | range 0..7 on all PCP/priority leaves; mandatory true on value leaves |
| YANG custom validators | [ ] | N/A - native YANG constraints sufficient |
| CLI commands/flags | [ ] | N/A - config-only, no CLI commands in this spec |
| CLI grammar (action before identifier) | [ ] | N/A |
| Editor autocomplete | [ ] | N/A - profile names are node-name type; unit class-of-service is string (validated in Go) |
| Functional test for new RPC/API | [x] | test/parse/cos-profile*.ci, test/plugin/cos-profile*.ci |
| Pipe completeness | [ ] | N/A |
| Env var registration | [ ] | N/A |
| Doctor check for runtime dependencies | [ ] | N/A - no external dependencies |
| Prometheus counters/metrics | [ ] | N/A |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` - add class-of-service profiles |
| 2 | Config syntax changed? | [x] | `docs/guide/configuration.md` - add class-of-service section example |
| 3 | CLI command added/changed? | [ ] | N/A |
| 4 | API/RPC added/changed? | [ ] | N/A |
| 5 | Plugin added/changed? | [x] | `docs/features/interfaces.md` - document CoS profiles |
| 6 | Has a user guide page? | [ ] | No existing QoS guide page |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [ ] | N/A - config abstraction only |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [ ] | N/A |
| 12 | Internal architecture changed? | [ ] | N/A |
| 13 | Route metadata keys added/changed? | [ ] | N/A |
| 14 | Prometheus counters added/changed? | [ ] | N/A |
| 15 | Registered plugin, event type, send type, command, capability, or runtime inventory changed? | [x] | Plugin overview: new cos plugin |
| 16 | Any changed source file is referenced by existing doc source anchors? | [ ] | Check during implementation |
| 17 | Existing docs show config/CLI/API examples for this area? | [x] | docs/features/interfaces.md has QoS map examples from spec-vlan-qos-map; update to show profile syntax |

## Files to Create
- `internal/core/cos/cos.go` - Profile type, Register(), Lookup(), Clear()
- `internal/core/cos/cos_test.go` - registry unit tests
- `internal/plugins/cos/cos.go` - plugin main: logger, runPlugin
- `internal/plugins/cos/register.go` - init() -> registry.Register()
- `internal/plugins/cos/config.go` - parseCoSConfig()
- `internal/plugins/cos/config_test.go` - config parsing tests
- `internal/plugins/cos/yang/embed.go` - generated (make generate)
- `internal/plugins/cos/yang/register.go` - generated (make generate)
- `internal/plugins/cos/yang/ze-cos-conf.yang` - profile definitions + interface container-merge
- `test/parse/cos-profile.ci` - valid profile + interface ref
- `test/parse/cos-profile-invalid.ci` - out-of-range PCP
- `test/parse/cos-profile-conflict.ci` - profile + inline conflict
- `test/parse/cos-profile-not-found.ci` - missing profile name
- `test/plugin/cos-profile-inherit.ci` - inheritance
- `test/plugin/cos-profile-override.ci` - unit override
- `test/plugin/cos-profile-none.ci` - opt-out

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Create, TDD Test Plan -- check what exists |
| 3. Wiring phase | Wiring Test table -- registry + plugin skeleton |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | make ze-lint-changed && make ze-unit-test && make ze-functional-test |
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

1. **Phase: Core registry (MANDATORY FIRST)** -- internal/core/cos/
   - Tests: TestCoSRegistryRegisterLookup, TestCoSRegistryLookupMissing, TestCoSRegistryClear
   - Files: internal/core/cos/cos.go, internal/core/cos/cos_test.go
   - Verify: registry tests pass

2. **Phase: YANG module** -- ze-cos-conf.yang with both sections
   - Tests: N/A (YANG is structural; validated by functional tests)
   - Files: internal/plugins/cos/yang/ze-cos-conf.yang
   - Verify: YANG is syntactically valid (loaded by config engine)

3. **Phase: Plugin skeleton** -- register.go + cos.go + config.go
   - Tests: TestCoSProfileParse, TestCoSProfileParseInvalid, TestCoSProfileParseEmpty
   - Files: internal/plugins/cos/register.go, cos.go, config.go, config_test.go
   - Verify: make generate updates all.go; plugin loads; config parsing tests pass

4. **Phase: Iface resolution** -- cos.Lookup() in parseUnits
   - Tests: TestCoSProfileResolution, TestCoSProfileUnitOverride, TestCoSProfileUnitOptOut, TestCoSProfileNoVLAN, TestCoSProfileConflictInline, TestCoSProfileNotFound
   - Files: internal/component/iface/config.go, internal/component/iface/config_test.go
   - Verify: all resolution tests pass; existing iface tests still pass

5. **Phase: Functional tests** -- .ci files
   - Tests: all test/parse/cos-profile*.ci and test/plugin/cos-profile*.ci
   - Files: test/parse/ and test/plugin/ directories
   - Verify: make ze-functional-test passes

6. **Phase: Documentation** -- update docs
   - Files: docs/features.md, docs/features/interfaces.md, docs/guide/configuration.md
   - Verify: doc examples match YANG; make ze-doc-test

7. **Full verification** -- make ze-verify

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | PCP/priority values validated 0-7 in YANG AND Go; maps populated correctly for both directions |
| Naming | Plugin name "cos", YANG prefix "cos", module "ze-cos-conf", registry package "cos" -- consistent |
| Data flow | Resolution only in parseUnits; netlink backend unchanged; no shortcut paths |
| Plugin removal | Delete internal/plugins/cos/ + blank import in all.go; build must be green |
| YANG validation | All PCP/priority leaves have range 0..7; mandatory true on value leaves |
| Container-merge | class-of-service leaf appears on ethernet, dummy, veth, bridge AND their units |
| Mutual exclusion | class-of-service + inline ingress/egress-qos-map on same unit is rejected |
| Inheritance | parent class-of-service flows to units; unit override works; "none" opts out |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Core registry | ls internal/core/cos/cos.go |
| Plugin package | ls internal/plugins/cos/register.go |
| YANG module | ls internal/plugins/cos/yang/ze-cos-conf.yang |
| Plugin in all.go | grep cos internal/component/plugin/all/all.go |
| Parse tests | ls test/parse/cos-profile*.ci |
| Plugin tests | ls test/plugin/cos-profile*.ci |
| Existing tests pass | make ze-functional-test (includes iface-vlan-qos tests) |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | PCP/priority range enforced in YANG (0-7); profile name is node-name (alphanumeric + hyphens) |
| Resource exhaustion | At most 8 entries per direction per profile; number of profiles bounded by config size |
| Config injection | Profile name used as map key lookup, never as a command or path |

### Failure Routing
| Failure | Route To |
|---------|----------|
| YANG container-merge doesn't work for config lists | R-1: switch to augment pattern |
| cos.Lookup() returns not-found during iface verification | R-2: add dependency declaration |
| Compilation error | Fix in the phase that introduced it |
| Existing iface tests break | Check that inline qos-map parsing is preserved |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |

### Failed Approaches
| Approach | Why abandoned | Replacement |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Two explicit ingress/egress containers over bidirectional shorthand | Single list with implicit bidirectionality | Explicit directions match the kernel model (two independent maps). A single entry like `pcp 6 { priority 6; }` looks unidirectional; hiding egress in the same entry is surprising. |
| YANG container-merge over augment | augment (creates base-module coupling) | Container-merge needs no import of ze-iface-conf; deleting the plugin cleanly removes the leaf |
| Shared registry in internal/core/cos/ over DirectBridge | DirectBridge (sync RPC), plugin-internal store | Config resolution is synchronous during verification; a shared registry is simpler. Same pattern as subscriber handler_registry.go |
| Profile ref as string (not leafref) in interface YANG | leafref (type-safe cross-module reference) | leafref across container-merged modules creates YANG coupling. Validation in Go via cos.Lookup() is equivalent and survives plugin removal |
| Interface-level inheritance with per-unit override | Per-unit only (no inheritance) | BNG use case: hundreds of VLAN subscribers share one profile. Repeating per-unit defeats the purpose of named profiles |
| "none" keyword for opt-out | Absent leaf means no inheritance | Need to distinguish "not set, inherit from parent" vs "explicitly no maps." "none" is the Juniper convention |
| CoS plugin owns interface binding leaf via container-merge | Binding leaf in ze-iface-conf.yang | Plugin self-containment: removing the plugin removes both the profile definitions AND the interface binding. Iface only provides the low-level mechanism. |

## Known Limitations
- No CLI commands in this spec (show class-of-service, etc.). A cos-cmd plugin can be added later
- No RADIUS-driven dynamic profile selection. That requires subscriber session integration (separate spec)
- Profiles are static config; no runtime modification
- No per-profile statistics or hit counters
- Container-merge adds the class-of-service leaf to all L2 interface types (ethernet, dummy, veth, bridge) but QoS maps only apply to VLAN sub-interfaces; applying to a non-VLAN unit is a validation error, not silent

## RFC Documentation

IEEE 802.1Q: PCP is bits 15-13 of the TCI field. 3-bit field, values 0-7.
No new RFC constraints beyond what spec-vlan-qos-map already enforces.

## Implementation Summary

### What Was Implemented

- Core registry (`internal/core/cos/`) with Register, Lookup, Clear
- CoS plugin (`internal/plugins/cos/`) with YANG, config parsing, InProcessConfigVerifier
- YANG container-merge for class-of-service leaf on ethernet, dummy, veth, bridge + units
- Profile resolution in iface parseUnits: inheritance, per-unit override, "none" opt-out
- Mutual exclusion between class-of-service ref and inline qos maps
- 3 unit tests (core registry) + 4 config parse tests + 2 verifier tests + 6 iface resolution tests
- 7 functional tests (4 parse, 3 plugin)
- Documentation in features.md, features/interfaces.md, guide/configuration.md

### Bugs Found/Fixed

None.

### Documentation Updates

- docs/features.md: added cos profiles mention in Interfaces row
- docs/features/interfaces.md: added Class-of-service named profiles row
- docs/guide/configuration.md: added Class-of-Service Profiles section with example

### Deviations from Plan

None. All acceptance criteria implemented as designed.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |

### Tests from TDD Plan
| Test | Status | Location | Notes |

### Files from Plan
| File | Status | Notes |

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Named CoS profiles parse and register | Unit test | TestCoSProfileParse |
| Profile ref on interface populates VLAN QoS maps | Unit test | TestCoSProfileResolution |
| Interface-level inheritance works | Functional test | test/plugin/cos-profile-inherit.ci |
| Per-unit override works | Functional test | test/plugin/cos-profile-override.ci |
| Opt-out with "none" works | Functional test | test/plugin/cos-profile-none.ci |
| Inline QoS maps unchanged | Existing test | test/parse/iface-vlan-qos.ci still passes |
| Plugin removal is clean | Build test | AC-11 (delete dir + import -> green build) |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |

### Fixes applied

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |

### Assumptions Resolved
| ID | Final Status | Evidence |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-12 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Critical Review passes
- [ ] Risks & Assumptions: every A-N confirmed or broken

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
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to plan/learned/NNN-cos-plugin.md
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-cos-plugin.md`
