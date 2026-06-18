# Spec: firewall-irr-iface

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | spec-firewall-irr (closed) |
| Phase | - |
| Updated | 2026-06-18 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/learned/913-firewall-irr.md` - predecessor decisions
4. `internal/component/firewall/plugins/irr/` - existing plugin code

## Task

Add per-interface AS-SET binding for ISP customer-facing source validation.
Each interface can be associated with an AS-SET; the firewall-irr plugin
generates source-address interval sets per interface and applies them as
ingress filters, rejecting traffic with source addresses not covered by the
customer's registered prefixes.

### Use case

An ISP connects customer routers on dedicated interfaces (e.g., `eth1` for
customer A with AS-SET AS-CUSTOMER-A). The ISP wants to automatically reject
spoofed traffic: if a packet arrives on `eth1` with a source address not in
AS-CUSTOMER-A's registered prefixes, it should be dropped.

### Design intent

Extend the existing firewall-irr plugin (spec-firewall-irr) with:
1. A YANG container under `firewall/irr/interface` that binds an AS-SET to an
   interface name
2. The plugin generates per-interface ingress filter chains with
   input-interface + source-address interval set matches
3. Reuses the shared PrefixStore for cache management
4. Same fail-closed semantics: commit rejects if any bound AS-SET has no cache

## Required Reading

### Architecture Docs
- [ ] `plan/learned/913-firewall-irr.md` - predecessor decisions and gotchas
- [ ] `internal/component/firewall/plugins/irr/` - existing plugin architecture
- [ ] `internal/component/iface/` - interface configuration model

### RFC Summaries (MUST for protocol work)

N/A - no protocol work.

**Key insights:**
- (to be filled during design)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/firewall/plugins/irr/irr.go` - existing plugin lifecycle
- [ ] `internal/component/firewall/plugins/irr/config.go` - config extraction
- [ ] `internal/component/firewall/plugins/irr/sets.go` - set generation

**Behavior to preserve:**
- All existing firewall-irr features (update/show commands, term-level source-asn/as-set matching)
- Shared PrefixStore cache semantics

**Behavior to change:**
- Add per-interface AS-SET binding config
- Generate ingress filter chains per interface

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Config commit with `firewall { irr { interface eth1 { source-as-set AS-FOO; } } }`

### Transformation Path
1. (to be filled during design)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config -> Plugin | SDK OnConfigure/OnConfigVerify | [ ] |
| Plugin -> Kernel | RegisterTables + ApplyAll | [ ] |

### Integration Points
- Existing firewall-irr plugin infrastructure
- Shared PrefixStore

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Per-interface ingress chains can coexist with firewall engine chains | registry merge via mergeSameNameTables | Would need separate table | verify with test | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Large number of interfaces with unique AS-SETs could produce many chains/sets | Memory usage, apply time | Cap interface count or warn |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `firewall { irr { interface eth1 { ... } } }` config | -> | plugin parses interface bindings | `TestParseInterfaceBinding` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `firewall { irr { interface eth1 { source-as-set AS-FOO; } } }` with cached data | Ingress chain on eth1 drops traffic with source not in AS-FOO prefixes |
| AC-2 | Same config without cached data | Commit rejects with actionable error |
| AC-3 | Multiple interfaces with different AS-SETs | Each interface gets independent ingress filter |
| AC-4 | Interface binding removed from config | Ingress filter chain removed on next apply |
| AC-5 | Plugin directory removed | Feature completely gone, interface config leaves absent |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures interface binding | Config -> plugin verify -> plugin apply -> kernel chain | `test-firewall-irr-iface.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseInterfaceBinding` | `irr/config_test.go` | Parsing interface AS-SET bindings from config | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-firewall-irr-iface` | `test/plugin/firewall-irr-iface.ci` | Interface binding creates ingress filter | |

### Interop Tests (MANDATORY for protocol features)

N/A - no wire protocol changes.

### Future (if deferring any tests)
- None

## Files to Modify
- `internal/component/firewall/plugins/irr/config.go` - parse interface bindings
- `internal/component/firewall/plugins/irr/irr.go` - verify and apply interface bindings
- `internal/component/firewall/plugins/irr/sets.go` - build per-interface chains
- `internal/component/firewall/plugins/irr/yang/ze-firewall-irr.yang` - YANG augment for interface container

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | Yes | `yang/ze-firewall-irr.yang` |
| CLI commands/flags | No | Existing show/update commands suffice |
| Functional test | Yes | `test/plugin/firewall-irr-iface.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md`, `docs/guide/firewall.md` |
| 2 | Config syntax changed? | Yes | `docs/guide/firewall.md` |
| 6 | Has a user guide page? | Yes | `docs/guide/firewall.md` (IRR section) |

## Files to Create
- (to be determined during design)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** -- YANG augment, config parsing skeleton
2. **Phase: Config** -- parse interface bindings from config JSON
3. **Phase: Chains** -- generate per-interface ingress filter chains
4. **Phase: Verify** -- reject commit if bound AS-SET has no cache
5. **Phase: Apply** -- register chains, apply
6. **Functional tests** -- .ci tests

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation |
| Plugin self-containment | Removing plugin removes all interface binding features |
| Chain naming | Per-interface chains have deterministic, non-colliding names |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| YANG interface container | grep yang file |
| Config parsing | unit test |
| Functional test | .ci file exists |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Interface name validation | Interface names validated against kernel limits |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
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

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
- Spec 1 (term-level IRR matching) must be closed before this work starts
