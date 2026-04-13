# Spec: fw-1-data-model — Firewall and Traffic Control Data Model

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-fw-0-umbrella |
| Phase | - |
| Updated | 2026-04-13 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/spec-fw-0-umbrella.md` — design decisions
3. `internal/component/iface/backend.go` — Backend interface pattern
4. `internal/component/iface/tunnel.go` — TunnelSpec/TunnelKind enum pattern

## Task

Define the Go data model for firewall tables/chains/sets/flowtables/terms with abstract
match/action types, and for traffic control qdiscs/classes/filters. These types are used
by every other spec in the fw set. Pure Go, no external dependencies.

The data model uses abstract firewall concepts (MatchSourceAddress, Accept, SetMark),
not nftables-native types (Payload, Cmp, Immediate). The nft backend lowers abstract
types to nftables register operations internally. The VPP backend maps them directly
to ACL rules/policers.

## Required Reading

### Architecture Docs
- [ ] `internal/component/iface/backend.go` — Backend interface with RegisterBackend
  → Constraint: same registration pattern for firewall and traffic backends
- [ ] `internal/component/iface/tunnel.go` — TunnelSpec struct and TunnelKind enum
  → Decision: use typed enums (not raw strings) for families, hooks, chain types, verdicts
- [ ] `rules/design-principles.md` — design principles
  → Constraint: no identity wrappers, no premature abstraction, explicit > implicit
- [ ] `rules/go-standards.md` — Go standards
  → Constraint: golangci-lint must pass, error wrapping with context

**Key insights:**
- Match interface + Action interface with concrete types per firewall concept (not nftables expression kind)
- Term = Name + []Match + []Action (Junos-style named terms with from/then split)
- Typed enums for families, hooks, chain types, policies
- Backend interfaces: Apply + ListTables + GetCounters (firewall), Apply + ListQdiscs (traffic)
- Backend registration follows iface pattern: RegisterBackend/LoadBackend/GetBackend
- Component registration follows iface pattern: registry.Register() plugin in init()

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/iface/backend.go` — Backend interface, RegisterBackend, LoadBackend, GetBackend
  → Constraint: firewall and traffic backends follow this exact pattern
- [ ] `internal/component/iface/tunnel.go` — TunnelKind uint8 enum, TunnelSpec struct
  → Constraint: use same enum pattern for TableFamily, ChainHook, ChainType, etc.
- [ ] `internal/component/iface/iface.go` — InterfaceInfo, InterfaceStats structs
  → Constraint: flat structs with exported fields, no methods unless transformation needed

**Behavior to preserve:**
- No existing firewall or traffic data model. Greenfield.

**Behavior to change:**
- Add firewall data model types in `internal/component/firewall/`
- Add traffic control data model types in `internal/component/traffic/`
- Add backend interfaces in both components

## Data Flow (MANDATORY)

### Entry Point
- Config parser (spec-fw-4) creates data model structs from YANG tree
- Structs passed to backend Apply method

### Transformation Path
1. YANG tree parsed into config structs (spec-fw-4)
2. Config structs converted to data model types (Table, Chain, Rule with Expressions)
3. Data model passed to backend Apply (spec-fw-2/fw-3)
4. Backend translates to kernel API calls

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config parser → Data model | Direct struct construction | [ ] |
| Data model → Backend | Apply method argument | [ ] |

### Integration Points
- `internal/component/firewall/config.go` (spec-fw-4) — constructs Tables from YANG
- `internal/plugins/firewallnft/` (spec-fw-2) — consumes Tables, translates to google/nftables
- `internal/component/traffic/config.go` (spec-fw-4) — constructs InterfaceQoS from YANG
- `internal/plugins/trafficnetlink/` (spec-fw-3) — consumes InterfaceQoS, translates to vishvananda/netlink

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling (firewall and traffic models independent)
- [ ] No duplicated functionality
- [ ] Zero-copy preserved where applicable

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Config with firewall section | → | firewall model construction + backend Apply | `test/firewall/001-boot-apply.ci` |
| Config with traffic-control section | → | traffic model construction + backend Apply | `test/traffic/001-boot-apply.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Construct Table with all fields | Table, Chain, Set, Flowtable, Term structs hold all firewall concepts |
| AC-2 | Construct Term with matches and actions | Term has Name, Matches []Match, Actions []Action |
| AC-3 | Every abstract match type | Has a concrete Match implementation (18 types) |
| AC-4 | Every abstract action/modifier type | Has a concrete Action implementation (24 types: 16 action + 8 modifier) |
| AC-5 | Construct InterfaceQoS with HTB | Qdisc, classes, filters all representable |
| AC-6 | Backend interface registered | RegisterBackend/LoadBackend/GetBackend work for both firewall and traffic |
| AC-7 | Backend read methods | ListTables, GetCounters (firewall), ListQdiscs (traffic) return data |
| AC-8 | Table name validation | Names must be non-empty, valid identifiers |
| AC-9 | Chain with hook | Base chain has type, hook, priority, policy; regular chain has none |
| AC-10 | Set with elements | Named set with type, flags, optional elements |
| AC-11 | Flowtable definition | Flowtable with hook, priority, devices list |
| AC-12 | Term name validation | Names must be non-empty, valid identifiers |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestTableConstruction` | `internal/component/firewall/model_test.go` | Table/Chain/Term/Set/Flowtable creation | |
| `TestMatchTypes` | `internal/component/firewall/model_test.go` | Every Match concrete type implements interface (18 types) | |
| `TestActionTypes` | `internal/component/firewall/model_test.go` | Every Action concrete type implements interface (24 types) | |
| `TestTermConstruction` | `internal/component/firewall/model_test.go` | Term with Name, Matches, Actions | |
| `TestTermNameValidation` | `internal/component/firewall/model_test.go` | Empty name rejected | |
| `TestTableValidation` | `internal/component/firewall/model_test.go` | Empty name rejected, valid family required | |
| `TestChainHookValidation` | `internal/component/firewall/model_test.go` | Base chain requires type+hook+priority, regular chain rejects them | |
| `TestSetConstruction` | `internal/component/firewall/model_test.go` | Named set with type, flags, elements | |
| `TestFlowtableConstruction` | `internal/component/firewall/model_test.go` | Flowtable with hook, priority, devices | |
| `TestBackendRegistration` | `internal/component/firewall/backend_test.go` | RegisterBackend, LoadBackend, GetBackend lifecycle | |
| `TestBackendReadMethods` | `internal/component/firewall/backend_test.go` | ListTables, GetCounters on mock backend | |
| `TestInterfaceQoSConstruction` | `internal/component/traffic/model_test.go` | InterfaceQoS with HTB, classes, filters | |
| `TestTrafficBackendRegistration` | `internal/component/traffic/backend_test.go` | RegisterBackend, LoadBackend, GetBackend lifecycle | |
| `TestTrafficBackendReadMethods` | `internal/component/traffic/backend_test.go` | ListQdiscs on mock backend | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Chain priority | int32 | 2147483647 | N/A (negative valid) | N/A |
| TableFamily enum | 0-5 | 5 (netdev) | N/A | 6 |
| Port in expression | 1-65535 | 65535 | 0 | 65536 |
| Rate limit | 1+ | 1 | 0 | N/A |
| HTB rate bps | 1+ | 1 | 0 | N/A |
| Mark value | 0-0xFFFFFFFF | 0xFFFFFFFF | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Model used in boot | `test/firewall/001-boot-apply.ci` | Config parsed into model, Apply called, kernel state correct | |
| Traffic model used | `test/traffic/001-boot-apply.ci` | Config parsed into traffic model, Apply called, tc state correct | |

### Future (if deferring any tests)
- None. All tests implemented with this spec.

## Files to Modify

No existing files modified. All new files.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | Spec-fw-4 handles YANG |
| CLI commands | No | Spec-fw-5 handles CLI |
| Editor autocomplete | No | YANG-driven |
| Functional test | Yes | `test/firewall/001-boot-apply.ci`, `test/traffic/001-boot-apply.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Internal data model, not user-facing |
| 2 | Config syntax changed? | No | Spec-fw-4 |
| 3 | CLI command added/changed? | No | Spec-fw-5 |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | Backend interface only, plugins in fw-2/fw-3 |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` — add firewall/traffic data model |

## Files to Create

- `internal/component/firewall/model.go` — Table, Chain, Term, Set, Flowtable, Match/Action types, enums
- `internal/component/firewall/model_test.go` — unit tests for all types
- `internal/component/firewall/backend.go` — Backend interface, RegisterBackend, LoadBackend, GetBackend
- `internal/component/firewall/backend_test.go` — backend registration tests
- `internal/component/traffic/model.go` — InterfaceQoS, Qdisc types, TrafficClass, TrafficFilter
- `internal/component/traffic/model_test.go` — unit tests
- `internal/component/traffic/backend.go` — Backend interface, RegisterBackend, LoadBackend, GetBackend
- `internal/component/traffic/backend_test.go` — backend registration tests

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Create |
| 3. Implement (TDD) | Phases below |
| 4. Full verification | `make ze-lint && make ze-unit-test` |
| 5. Critical review | Checklist below |
| 6-8. Fix/re-verify | Iterate |
| 9. Deliverables | Checklist below |
| 10. Security | Checklist below |
| 12. Present | Executive Summary |

### Implementation Phases

1. **Phase: Firewall enums and base types** — TableFamily, ChainHook, ChainType, Policy, VerdictCode
   - Tests: TestTableFamily, TestChainHook enum validation
   - Files: `internal/component/firewall/model.go`
   - Verify: tests fail → implement → tests pass

2. **Phase: Expression interface and all concrete types** — Expression interface, one type per google/nftables/expr.*
   - Tests: TestExpressionTypes (verify interface compliance)
   - Files: `internal/component/firewall/model.go`
   - Verify: tests fail → implement → tests pass

3. **Phase: Table/Chain/Rule/Set/Flowtable structs** — composite types
   - Tests: TestTableConstruction, TestChainHookValidation, TestSetConstruction, TestFlowtableConstruction
   - Files: `internal/component/firewall/model.go`
   - Verify: tests fail → implement → tests pass

4. **Phase: Firewall backend interface** — RegisterBackend/LoadBackend/GetBackend
   - Tests: TestBackendRegistration
   - Files: `internal/component/firewall/backend.go`
   - Verify: tests fail → implement → tests pass

5. **Phase: Traffic control types** — InterfaceQoS, Qdisc types, TrafficClass, TrafficFilter
   - Tests: TestInterfaceQoSConstruction
   - Files: `internal/component/traffic/model.go`
   - Verify: tests fail → implement → tests pass

6. **Phase: Traffic backend interface** — RegisterBackend/LoadBackend/GetBackend
   - Tests: TestTrafficBackendRegistration
   - Files: `internal/component/traffic/backend.go`
   - Verify: tests fail → implement → tests pass

7. **Full verification** → `make ze-verify`

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Enums cover all families, hooks, chain types. 42 abstract types present. |
| Naming | Types use Go conventions (exported, CamelCase), config keywords use readable names |
| Data flow | Data model types are pure Go, no external library types leak |
| Rule: no-layering | No wrapper types around google/nftables types. Abstract types are ze-native. |
| Rule: explicit > implicit | Every Match/Action type has explicit fields, no interface{} or map[string]any |
| Data model abstraction | Types model firewall concepts (MatchSourceAddress), not nftables operations (Payload+Cmp) |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| firewall model.go exists | `ls internal/component/firewall/model.go` |
| firewall backend.go exists | `ls internal/component/firewall/backend.go` |
| traffic model.go exists | `ls internal/component/traffic/model.go` |
| traffic backend.go exists | `ls internal/component/traffic/backend.go` |
| All Match types exist | `grep "func.*matchMarker" internal/component/firewall/model.go` count >= 18 |
| All Action types exist | `grep "func.*actionMarker" internal/component/firewall/model.go` count >= 24 |
| Backend registration compiles | `go build ./internal/component/firewall/ ./internal/component/traffic/` |
| Tests pass | `go test ./internal/component/firewall/... ./internal/component/traffic/...` |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Input validation | Table names, chain names validated as valid identifiers |
| No unsafe | No unsafe.Pointer usage in data model |
| Enum bounds | All enum types validated on construction |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Lint failure | Fix inline |
| 3 fix attempts fail | STOP. Report all 3. Ask user. |

## Abstract Type Inventory

Types model firewall concepts, not nftables kernel operations. The nft backend lowers
these to nftables register operations internally. The VPP backend maps directly to ACL/policer fields.

### Match Types (from block, 18 types)

| Type | Config keyword | Key fields |
|------|---------------|------------|
| MatchSourceAddress | `source address` | Prefix |
| MatchDestinationAddress | `destination address` | Prefix |
| MatchSourcePort | `source port` | Port, PortEnd (0 = single) |
| MatchDestinationPort | `destination port` | Port, PortEnd |
| MatchProtocol | `protocol` | Protocol (tcp/udp/icmp/sctp/...) |
| MatchInputInterface | `input interface` | Name |
| MatchOutputInterface | `output interface` | Name |
| MatchConnState | `connection state` | States (bitmask: new/established/related/invalid) |
| MatchConnMark | `connection mark` | Value, Mask |
| MatchMark | `mark` | Value, Mask |
| MatchDSCP | `dscp` | Value (ef/af41/cs6/...) |
| MatchConnBytes | `connection bytes` | Over/Under, Bytes |
| MatchConnLimit | `connection limit` | Count, Flags |
| MatchFib | `fib` | Result, Flags |
| MatchSocket | `socket` | Key, Level |
| MatchRt | `routing` | Key |
| MatchExtHdr | `extension header` | Type, Field, Offset |
| MatchInSet | `@set-name` | SetName, MatchField (source-addr/dest-addr/source-port/dest-port) |

### Action Types (then block, 16 types)

| Type | Config keyword | Key fields |
|------|---------------|------------|
| Accept | `accept` | (none) |
| Drop | `drop` | (none) |
| Reject | `reject` | Type, Code |
| Jump | `jump` | Target chain name |
| Goto | `goto` | Target chain name |
| Return | `return` | (none) |
| SNAT | `snat` | Address, Port, PortEnd, Flags |
| DNAT | `dnat` | Address, Port, PortEnd, Flags |
| Masquerade | `masquerade` | Port, PortEnd, Flags |
| Redirect | `redirect` | Port, Flags |
| Queue | `queue` | Num, Total, Flags |
| Notrack | `notrack` | (none) |
| TProxy | `tproxy` | Address, Port |
| Duplicate | `duplicate` | Address, Device |
| FlowOffload | `flow offload` | FlowtableName |
| Synproxy | `synproxy` | MSS, Wscale, Flags |

### Modifier Types (then block, 8 types)

| Type | Config keyword | Key fields |
|------|---------------|------------|
| SetMark | `mark set` | Value, Mask |
| SetConnMark | `connection mark set` | Value, Mask |
| SetDSCP | `dscp set` | Value |
| Counter | `counter` | Name (optional, for named counters) |
| Log | `log` | Prefix, Level, Group, Snaplen |
| Limit | `limit rate` | Rate, Unit, Over (bool), Burst |
| Quota | `quota` | Bytes, Flags |
| SecMark | `secmark` | Name |

### Not in model (nftables implementation details, handled by nft backend lowering)

Payload, Cmp, Range, Bitwise, Immediate, Byteorder, Meta, Numgen, Hash, Dynset, Objref.
These are nftables register-machine operations generated by the nft backend when lowering
abstract types to kernel expressions.

### Deferred

Hash and Numgen (load-balancing expressions). Can be added as MatchHash/MatchNumgen when needed.

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
### Bugs Found/Fixed
### Documentation Updates
### Deviations from Plan

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
- [ ] Write learned summary to `plan/learned/NNN-fw-1-data-model.md`
- [ ] Summary included in commit
