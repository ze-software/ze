# Spec: listener-3-conflict

| Field | Value |
|-------|-------|
| Status | done |
| Depends | spec-listener-1-yang |
| Phase | - |
| Updated | 2026-04-01 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/config/listener.go` - CollectListeners, ValidateListenerConflicts (new)
4. `internal/component/bgp/config/loader_create.go` - wiring point for conflict validation

## Task

Implement port conflict detection at config parse time. CollectListeners walks the resolved config tree for ze:listener containers, collecting all ip:port endpoints. ValidateListenerConflicts checks for overlapping endpoints with 0.0.0.0/:: wildcard awareness. Wire into config validation pipeline.

**Parent spec:** `spec-listener-0-umbrella.md`
**Sibling specs:** `spec-listener-1-yang` (prerequisite), `spec-listener-2-env`, `spec-listener-5-log`, `spec-listener-4-migrate`

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/syntax.md` - config parsing, YANG-driven schema
  -> Constraint: YANG extensions drive parser behavior; ze:listener containers identified by extension metadata
- [ ] `docs/architecture/config/environment.md` - environment configuration
  -> Constraint: environment block extraction happens before conflict check; conflict check needs fully resolved tree

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4271.md` - BGP port 179, connection establishment
  -> Constraint: BGP peer local endpoint uses ip union type with `auto` enum; `auto` entries must be skipped in conflict detection

**Key insights:**
- ze:listener extension marks YANG containers as network listener endpoints (defined in spec-listener-1-yang)
- Config tree is fully resolved (peers expanded from groups, defaults applied) before conflict check runs
- BGP peer `connection > local > ip` uses a union type with `auto` enum value meaning OS chooses; these cannot conflict
- Plugin hub server entries are a YANG list, each with ip + port children
- IPv4 wildcard is 0.0.0.0; IPv6 wildcard is ::; both cover all addresses in their respective family

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/config/loader_create.go` - CreateReactorFromTree: resolves config, creates reactor; no conflict check exists
- [ ] `internal/component/config/environment.go` - Environment struct with listener-related sections (web, mcp, lg, ssh, telemetry, plugin hub)
- [ ] `internal/component/bgp/schema/ze-bgp-conf.yang` - BGP peer `connection > local { ip (union with auto), port }`
- [ ] `internal/component/plugin/schema/ze-plugin-conf.yang` - `plugin > hub > server (list) { host, port }`

**Behavior to preserve:**
- CreateReactorFromTree pipeline: file parse, tree resolution, peers expansion, reactor creation
- All existing config validation (unknown keys, type checks, YANG constraints)
- BGP peer local ip `auto` meaning: OS chooses bind address

**Behavior to change:**
- New CollectListeners function walks resolved config tree for ze:listener containers, returns list of (service-name, ip, port) tuples
- New ValidateListenerConflicts checks collected endpoints: exact duplicates, 0.0.0.0 vs specific IPv4 on same port, :: vs specific IPv6 on same port
- Wired into CreateReactorFromTree after config resolution (peers expanded, defaults applied)
- Config parse error with both conflicting service names in the message
- BGP peer `connection > local` endpoints participate (they have ze:listener)
- Plugin hub server list entries participate (each has ze:listener)

## Data Flow (MANDATORY)

### Entry Point
- Config file parsed into tree (YANG-driven)
- Tree resolved: peers expanded from groups, defaults applied

### Transformation Path
1. Config file parsed into tree via YANG schema
2. Tree resolved (ResolveBGPTree): peer groups expanded, defaults applied
3. NEW: CollectListeners walks tree, finds all containers marked with ze:listener extension, extracts service-name + ip + port for each
4. NEW: ValidateListenerConflicts receives collected endpoints, checks for overlapping ip:port bindings with wildcard awareness
5. If conflict found: return error before reactor starts, naming both conflicting services and the endpoint
6. If no conflict: continue to reactor creation

### Listener Sources

All services use named lists (`list server { key name; }`) after spec-listener-1-yang. Each list entry contributes one endpoint to conflict detection. Services with `enabled false` are skipped.

| Source | Tree path | IP type | Notes |
|--------|-----------|---------|-------|
| Web server | `environment > web > server > *` | zt:ip-address | Default 0.0.0.0:3443 (when enabled, empty list) |
| Looking Glass | `environment > looking-glass > server > *` | zt:ip-address | Default 0.0.0.0:8443 |
| MCP server | `environment > mcp > server > *` | zt:ip-address | Default 127.0.0.1 |
| Telemetry | `telemetry > prometheus > server > *` | zt:ip-address | Default 0.0.0.0:9273 |
| SSH server | `environment > ssh > server > *` | zt:ip-address | List entries |
| BGP peer local | `bgp > neighbor > * > connection > local` | union (ip-address or auto) | Skip when value is `auto` |
| Plugin hub server | `plugin > hub > server > *` | zt:ip-address | List entries |

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree -> Listener collection | Walk tree nodes, check ze:listener extension metadata | [ ] |
| Listener collection -> Validation | Pass slice of ListenerEndpoint to conflict checker | [ ] |
| Validation -> Config pipeline | Return error to CreateReactorFromTree caller | [ ] |

### Integration Points
- `loader_create.go` CreateReactorFromTree - insert conflict check after tree resolution, before reactor creation
- YANG tree walker - use existing tree traversal to find ze:listener containers
- Config error reporting - reuse existing config error format with service names in message

### Architectural Verification
- [ ] No bypassed layers (conflict check uses same parsed config tree, runs in config pipeline)
- [ ] No unintended coupling (listener collection reads YANG metadata, not component internals)
- [ ] No duplicated functionality (no existing conflict detection to extend)
- [ ] Zero-copy preserved where applicable (N/A -- config parsing, not wire encoding)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config with two services on same port | -> | ValidateListenerConflicts | `test/parse/listener-conflict-same-port.ci` |
| Config with 0.0.0.0 vs specific IP conflict | -> | ValidateListenerConflicts | `test/parse/listener-conflict-wildcard.ci` |
| Config with no conflicts | -> | CollectListeners + ValidateListenerConflicts | `test/parse/listener-no-conflict.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config with web ip 0.0.0.0 port 8443 and LG ip 0.0.0.0 port 8443 | Parse error naming both services and the conflicting endpoint |
| AC-2 | Config with web ip 0.0.0.0 port 8443 and LG ip 127.0.0.1 port 8443 | Parse error: wildcard 0.0.0.0 conflicts with 127.0.0.1 on port 8443 |
| AC-3 | Config with web port 3443 and LG port 8443 | No conflict, loads successfully |
| AC-4 | Config with BGP peer local ip 10.0.0.1 port 179 and web ip 10.0.0.1 port 179 | Conflict detected between BGP peer and web |
| AC-5 | Config with BGP peer local ip 10.0.0.1 port 179 and web ip 10.0.0.2 port 179 | No conflict (different specific IPs) |
| AC-6 | IPv6 :: vs specific IPv6 on same port | Conflict detected |
| AC-7 | Plugin hub server entry conflicts with web | Conflict detected |
| AC-8 | BGP peer with local ip auto | Skipped in conflict check (auto = OS chooses) |
| AC-9 | Config with no listeners configured | No error |

## Wildcard Logic

| Left IP | Right IP | Same Port | Conflict? | Reason |
|---------|----------|-----------|-----------|--------|
| 0.0.0.0 | 0.0.0.0 | yes | yes | Exact duplicate |
| 0.0.0.0 | 10.0.0.1 | yes | yes | IPv4 wildcard covers all IPv4 |
| 10.0.0.1 | 10.0.0.1 | yes | yes | Exact duplicate |
| 10.0.0.1 | 10.0.0.2 | yes | no | Different specific IPs |
| :: | ::1 | yes | yes | IPv6 wildcard covers all IPv6 |
| :: | :: | yes | yes | Exact duplicate |
| 0.0.0.0 | ::1 | yes | no | Different address families |
| :: | 10.0.0.1 | yes | no | Different address families |
| 10.0.0.1 | 10.0.0.2 | no | no | Different ports |

## ListenerEndpoint Type

| Field | Type | Description |
|-------|------|-------------|
| Service | string | Human-readable service name for error messages (e.g., "web", "looking-glass", "bgp peer 10.0.0.1") |
| IP | net.IP | Parsed IP address; nil or unspecified means wildcard |
| Port | uint16 | Listening port number |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestValidateListenerConflicts_SamePort` | `internal/component/config/listener_test.go` | AC-1: exact duplicate ip:port detected | |
| `TestValidateListenerConflicts_WildcardIPv4` | `internal/component/config/listener_test.go` | AC-2: 0.0.0.0 conflicts with specific IPv4 on same port | |
| `TestValidateListenerConflicts_WildcardIPv6` | `internal/component/config/listener_test.go` | AC-6: :: conflicts with specific IPv6 on same port | |
| `TestValidateListenerConflicts_NoConflict` | `internal/component/config/listener_test.go` | AC-3, AC-5: different ports or different specific IPs pass | |
| `TestValidateListenerConflicts_BGPPeer` | `internal/component/config/listener_test.go` | AC-4: BGP peer local endpoint participates in conflict check | |
| `TestValidateListenerConflicts_PluginHub` | `internal/component/config/listener_test.go` | AC-7: plugin hub server entry participates in conflict check | |
| `TestValidateListenerConflicts_AutoSkipped` | `internal/component/config/listener_test.go` | AC-8: BGP peer with ip auto excluded from conflict check | |
| `TestValidateListenerConflicts_NoListeners` | `internal/component/config/listener_test.go` | AC-9: empty endpoint list produces no error | |
| `TestCollectListeners` | `internal/component/config/listener_test.go` | Collects endpoints from all ze:listener containers in tree | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| port (zt:port) | 1-65535 | 65535 | 0 | 65536 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `listener-conflict-same-port` | `test/parse/listener-conflict-same-port.ci` | Config with two services on same ip:port rejected at parse time | |
| `listener-conflict-wildcard` | `test/parse/listener-conflict-wildcard.ci` | Config with wildcard vs specific IP conflict rejected at parse time | |
| `listener-no-conflict` | `test/parse/listener-no-conflict.ci` | Config with non-overlapping listeners loads successfully | |

### Future (if deferring any tests)
- Property test: random listener sets, verify conflict detection is symmetric and transitive

## Files to Create
- `internal/component/config/listener.go` - ListenerEndpoint type, CollectListeners, ValidateListenerConflicts
- `internal/component/config/listener_test.go` - unit tests for conflict detection and listener collection
- `test/parse/listener-conflict-same-port.ci` - functional test: same ip:port rejected
- `test/parse/listener-conflict-wildcard.ci` - functional test: wildcard conflict rejected
- `test/parse/listener-no-conflict.ci` - functional test: non-overlapping listeners accepted

## Files to Modify
- `internal/component/bgp/config/loader_create.go` - wire ValidateListenerConflicts call after config resolution, before reactor creation

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A (reads existing ze:listener extension from spec-listener-1-yang) |
| CLI commands/flags | No | N/A (validation is automatic at config parse time) |
| Editor autocomplete | No | N/A |
| Functional test for new RPC/API | No | N/A |
| Functional test for config validation | Yes | `test/parse/listener-conflict-*.ci`, `test/parse/listener-no-conflict.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` -- port conflict detection at config parse time |
| 2 | Config syntax changed? | No | N/A (no syntax change; validation of existing syntax) |
| 3 | CLI command added/changed? | No | N/A |
| 4 | API/RPC added/changed? | No | N/A |
| 5 | Plugin added/changed? | No | N/A |
| 6 | Has a user guide page? | No | N/A |
| 7 | Wire format changed? | No | N/A |
| 8 | Plugin SDK/protocol changed? | No | N/A |
| 9 | RFC behavior implemented? | No | N/A |
| 10 | Test infrastructure changed? | No | N/A |
| 11 | Affects daemon comparison? | No | N/A |
| 12 | Internal architecture changed? | Yes | `docs/architecture/config/environment.md` -- listener conflict detection in config validation pipeline |

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan -- check what exists |
| 3. Implement (TDD) | Implementation phases below (write-test-fail-implement-pass per phase) |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report per `rules/planning.md` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: ListenerEndpoint type and CollectListeners** -- Define the ListenerEndpoint type and implement CollectListeners to walk the resolved config tree for ze:listener containers
   - Tests: TestCollectListeners
   - Files: internal/component/config/listener.go, internal/component/config/listener_test.go
   - Verify: tests fail -> implement -> tests pass

2. **Phase: ValidateListenerConflicts with wildcard logic** -- Implement the conflict detection algorithm with wildcard awareness for both IPv4 and IPv6
   - Tests: TestValidateListenerConflicts_SamePort, TestValidateListenerConflicts_WildcardIPv4, TestValidateListenerConflicts_WildcardIPv6, TestValidateListenerConflicts_NoConflict, TestValidateListenerConflicts_BGPPeer, TestValidateListenerConflicts_PluginHub, TestValidateListenerConflicts_AutoSkipped, TestValidateListenerConflicts_NoListeners
   - Files: internal/component/config/listener.go, internal/component/config/listener_test.go
   - Verify: tests fail -> implement -> tests pass

3. **Phase: Wire into loader_create.go** -- Call ValidateListenerConflicts in CreateReactorFromTree after config resolution, before reactor creation
   - Tests: integration via functional tests
   - Files: internal/component/bgp/config/loader_create.go
   - Verify: manual verification that conflict check is called in the pipeline

4. **Phase: Functional tests** -- Create .ci tests that exercise the full config parse pipeline with conflicting and non-conflicting listener configs
   - Tests: listener-conflict-same-port.ci, listener-conflict-wildcard.ci, listener-no-conflict.ci
   - Files: test/parse/listener-conflict-same-port.ci, test/parse/listener-conflict-wildcard.ci, test/parse/listener-no-conflict.ci
   - Verify: functional tests pass

5. **Phase: Full verification** -- `make ze-verify` (lint + all ze tests except fuzz)

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1 through AC-9 has implementation with file:line |
| Correctness | Wildcard logic handles both IPv4 (0.0.0.0) and IPv6 (::) correctly; cross-family (0.0.0.0 vs ::1) does NOT conflict |
| Error messages | Conflict error names both services and the conflicting endpoint (ip:port) |
| Data flow | Conflict check runs after full config resolution (peers expanded, defaults applied) and before reactor creation |
| Auto skip | BGP peer with ip auto is excluded from conflict check, not treated as wildcard |
| Rule: no-layering | No fallback to runtime bind-time conflict detection |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| ListenerEndpoint type in listener.go | `grep "ListenerEndpoint" internal/component/config/listener.go` |
| CollectListeners function | `grep "func CollectListeners" internal/component/config/listener.go` |
| ValidateListenerConflicts function | `grep "func ValidateListenerConflicts" internal/component/config/listener.go` |
| Wired into loader_create.go | `grep "ValidateListenerConflicts\|CollectListeners" internal/component/bgp/config/loader_create.go` |
| Functional test: same port conflict | `ls test/parse/listener-conflict-same-port.ci` |
| Functional test: wildcard conflict | `ls test/parse/listener-conflict-wildcard.ci` |
| Functional test: no conflict | `ls test/parse/listener-no-conflict.ci` |
| 9 unit tests pass | `go test -run TestValidateListenerConflicts -v ./internal/component/config/` |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Error messages | Do not leak sensitive config (passwords, secrets) in conflict error text; only service names and ip:port |
| Wildcard warning | Document that 0.0.0.0 binds all interfaces (security implication for MCP which defaults to 127.0.0.1) |
| Input validation | IP addresses and ports come from YANG-validated config tree; no additional parsing of untrusted input needed |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH if misunderstood |
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

N/A -- no new RFC behavior. Port conflict detection is a config validation feature, not a protocol feature.

## Implementation Summary

### What Was Implemented
- [pending]

### Bugs Found/Fixed
- [pending]

### Documentation Updates
- [pending]

### Deviations from Plan
- [pending]

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
- [ ] AC-1..AC-9 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-verify` passes
- [ ] Feature code integrated (listener.go wired into loader_create.go)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass -- defer with user approval)
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

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-listener-3-conflict.md`
- [ ] Summary included in commit
