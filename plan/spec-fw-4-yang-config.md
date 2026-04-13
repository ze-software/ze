# Spec: fw-4-yang-config — Firewall and Traffic Control YANG Configuration

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-fw-1-data-model |
| Phase | - |
| Updated | 2026-04-13 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/spec-fw-0-umbrella.md` — config syntax decisions (5b, 5c)
3. `plan/spec-fw-1-data-model.md` — data model types to parse into
4. `internal/component/iface/schema/` — existing YANG module pattern

## Task

Define YANG modules `ze-firewall-conf` and `ze-traffic-control-conf`. Implement config parsers
that convert YANG tree to data model types (fw-1). Register env vars for config leaves.

Config syntax is hybrid Junos/ze: named terms with from/then blocks, readable names,
nftables structural concepts (table/chain/hook/priority/policy/set/flowtable).

Keywords use readable names: `destination port` not `dport`, `connection state` not `ct state`.
Match keywords go in `from { }`, action/modifier keywords go in `then { }`.

Table names in config are bare (e.g., `wan`). The component prepends `ze_` before passing to backend.

## Required Reading

### Architecture Docs
- [ ] `rules/config-design.md` — config design rules
  → Constraint: fail on unknown keys, no version numbers, env var for every YANG environment leaf
  → Constraint: YANG grouping for shared structure, augment only for cross-component
- [ ] `.claude/patterns/config-option.md` — config option pattern
  → Constraint: YANG leaf + env.MustRegister for environment leaves
- [ ] `internal/component/iface/schema/` — existing YANG module example
  → Constraint: follow naming convention: ze-<component>-conf.yang
- [ ] `internal/component/iface/config.go` — existing config parser pattern
  → Constraint: OnConfigVerify validates, OnConfigReload applies
- [ ] `plan/spec-fw-0-umbrella.md` — config syntax decisions
  → Decision: nft-native with readable names
  → Decision: table name in config is bare, ze_ prefix added by component

**Key insights:**
- YANG modules follow `ze-<component>-conf` naming
- Config parser reads YANG tree, builds data model structs, calls backend Apply
- OnConfigVerify for validation (parse + check), OnConfigReload for apply
- env vars registered for any leaves under `environment/`
- Readable keyword mapping table defines config-to-expression translation

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/iface/config.go` — config parser for interface component
  → Constraint: pattern: parse tree → build struct → validate → apply via backend
- [ ] `internal/component/iface/schema/ze-iface-conf.yang` — existing YANG module
  → Constraint: uses ze-types groupings, ze-extensions for listeners
- [ ] `internal/component/config/yang/register.go` — YANG module registration
  → Constraint: yang.RegisterModule(name, content) in init()

**Behavior to preserve:**
- No existing firewall or traffic YANG modules. Greenfield.
- Existing config parsing infrastructure unchanged.

**Behavior to change:**
- Add two new YANG modules
- Add two new config parsers
- Register env vars for environment leaves

## Data Flow (MANDATORY)

### Entry Point
- Config file (or `ze config edit`) containing `firewall { ... }` or `traffic-control { ... }`

### Transformation Path
1. Config system parses file into YANG tree
2. Firewall component's OnConfigVerify receives tree, parses `firewall` subtree
3. Parser walks table/chain/term structure, translating from-block keywords to Match types and then-block keywords to Action types
4. Parser builds `[]Table` (firewall) or `map[string]InterfaceQoS` (traffic)
5. On reload, component calls backend.Apply with built structs

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config system → Component | OnConfigVerify/OnConfigReload callbacks with YANG tree | [ ] |
| Component → Backend | Apply method with data model structs | [ ] |

### Integration Points
- `internal/component/config/` — provides YANG tree to component callbacks
- `internal/component/firewall/model.go` (fw-1) — Table, Expression types to construct
- `internal/component/traffic/model.go` (fw-1) — InterfaceQoS types to construct
- `internal/core/env/` — env var registration

### Architectural Verification
- [ ] No bypassed layers (config → parser → model → backend)
- [ ] No unintended coupling
- [ ] No duplicated functionality
- [ ] Zero-copy not applicable

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| firewall config file | → | config parser → []Table → Apply | `test/firewall/001-boot-apply.ci` |
| traffic-control config file | → | config parser → map[string]InterfaceQoS → Apply | `test/traffic/001-boot-apply.ci` |
| invalid firewall config | → | OnConfigVerify returns error | `test/parse/firewall-invalid-001.ci` |
| invalid traffic config | → | OnConfigVerify returns error | `test/parse/traffic-invalid-001.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config term with `from { destination port 80; }` | Parsed to Term with MatchDestinationPort(80) in Matches |
| AC-2 | Config term with `from { source address 10.0.0.0/8; }` | Parsed to Term with MatchSourceAddress(10.0.0.0/8) in Matches |
| AC-3 | Config term with `from { connection state established,related; }` | Parsed to Term with MatchConnState(established,related) in Matches |
| AC-4 | Config term with `then { limit rate 10/second; }` | Parsed to Term with Limit(10, second) in Actions |
| AC-5 | Config term with `then { log prefix "DROP: "; }` | Parsed to Term with Log(prefix="DROP: ") in Actions |
| AC-6 | Config term with `then { mark set 0x10; }` | Parsed to Term with SetMark(0x10) in Actions |
| AC-7 | Config terms with `then { accept; }`, `then { drop; }`, `then { reject; }` | Parsed to Accept, Drop, Reject in Actions |
| AC-8 | Config terms with `then { snat ... }`, `then { dnat ... }`, `then { masquerade; }` | Parsed to SNAT, DNAT, Masquerade in Actions |
| AC-9 | Config term with `then { flow offload @flowtable-name; }` | Parsed to FlowOffload referencing named flowtable |
| AC-10 | Config term with `from { source address @blocked; }` | Parsed to MatchInSet(blocked, source-addr) in Matches |
| AC-11 | Config table name `wan` | Component produces Table with Name `ze_wan` |
| AC-12 | Config with HTB qdisc, classes, mark match | Parsed to InterfaceQoS with correct structure |
| AC-13 | Unknown keyword in from block | Config rejected with clear error |
| AC-14 | Action keyword in from block | Config rejected: "accept not valid in from block" |
| AC-15 | Match keyword in then block | Config rejected: "source address not valid in then block" |
| AC-16 | Missing required field (base chain without type) | Config rejected with clear error |
| AC-17 | Term without name | Config rejected with clear error |
| AC-18 | YANG module registered | `yang.Modules()` includes ze-firewall-conf and ze-traffic-control-conf |

## Readable Keyword Mapping

Config uses named terms with `from { }` (matches) and `then { }` (actions/modifiers).
Keywords map to abstract Match/Action types from fw-1, not to nftables expressions.

### from block keywords (Match types)

| Config keyword | Abstract type | Details |
|----------------|--------------|---------|
| `source address` | MatchSourceAddress | IPv4 or IPv6 prefix |
| `destination address` | MatchDestinationAddress | IPv4 or IPv6 prefix |
| `source port` | MatchSourcePort | Single or range (5060-5061) |
| `destination port` | MatchDestinationPort | Single or range |
| `protocol` | MatchProtocol | tcp, udp, icmp, sctp, etc. |
| `input interface` | MatchInputInterface | By name |
| `output interface` | MatchOutputInterface | By name |
| `connection state` | MatchConnState | established, related, new, invalid (comma-separated) |
| `connection mark` | MatchConnMark | uint32 value + optional mask |
| `mark` | MatchMark | uint32 value + optional mask |
| `dscp` | MatchDSCP | ef, af41, cs6, etc. |
| `connection bytes` | MatchConnBytes | over/under + byte count |
| `connection limit` | MatchConnLimit | Count + flags |
| `fib` | MatchFib | Result + flags |
| `socket` | MatchSocket | Key + level |
| `routing` | MatchRt | Key |
| `extension header` | MatchExtHdr | Type + field + offset |
| `source address @set-name` | MatchInSet | SetName + field=source-addr |
| `destination address @set-name` | MatchInSet | SetName + field=dest-addr |

### then block keywords (Action types)

| Config keyword | Abstract type | Details |
|----------------|--------------|---------|
| `accept` | Accept | Terminal verdict |
| `drop` | Drop | Terminal verdict |
| `reject` | Reject | Optional type/code (e.g., `with icmp admin-prohibited`) |
| `return` | Return | Return to caller chain |
| `jump` | Jump | Target chain name |
| `goto` | Goto | Target chain name |
| `snat` | SNAT | Address + optional port |
| `dnat` | DNAT | Address + optional port |
| `masquerade` | Masquerade | Optional port range |
| `redirect` | Redirect | Optional port |
| `notrack` | Notrack | Bypass conntrack |
| `flow offload` | FlowOffload | @flowtable-name |
| `tproxy` | TProxy | Address + port |
| `queue` | Queue | Queue number + flags |
| `duplicate` | Duplicate | Address + device |
| `synproxy` | Synproxy | MSS + wscale + flags |

### then block keywords (Modifier types)

| Config keyword | Abstract type | Details |
|----------------|--------------|---------|
| `mark set` | SetMark | uint32 value + optional mask |
| `connection mark set` | SetConnMark | uint32 value + optional mask |
| `dscp set` | SetDSCP | ef, af41, cs6, etc. |
| `counter` | Counter | Optional name for named counters |
| `log` | Log | Optional prefix, level, group, snaplen |
| `quota` | Quota | Bytes + flags |
| `limit rate` | Limit | N/unit (per second, minute, hour, day), optional burst |
| `secmark` | SecMark | Name |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseFirewallTable` | `internal/component/firewall/config_test.go` | YANG tree → Table with chains |
| `TestParseBaseChain` | `internal/component/firewall/config_test.go` | Base chain with hook/priority/policy |
| `TestParseRuleExpressions` | `internal/component/firewall/config_test.go` | Each readable keyword → correct Expression |
| `TestParseNamedSet` | `internal/component/firewall/config_test.go` | Set definition with type and flags |
| `TestParseFlowtable` | `internal/component/firewall/config_test.go` | Flowtable with devices |
| `TestTableNamePrefix` | `internal/component/firewall/config_test.go` | Config name "wan" → ze_wan |
| `TestParseInvalidConfig` | `internal/component/firewall/config_test.go` | Unknown keywords, missing fields → error |
| `TestParseTrafficHTB` | `internal/component/traffic/config_test.go` | HTB qdisc with classes and filters |
| `TestParseTrafficHFSC` | `internal/component/traffic/config_test.go` | HFSC qdisc with service curves |
| `TestParseTrafficInvalid` | `internal/component/traffic/config_test.go` | Missing interface, invalid rate → error |
| `TestYANGModuleRegistered` | `internal/component/firewall/schema/register_test.go` | Module appears in yang.Modules() |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Port | 1-65535 | 65535 | 0 | 65536 |
| Rate limit value | 1+ | 1 | 0 | N/A |
| Priority | int32 | max | N/A | N/A |
| HTB rate | 1+ bps | 1 | 0 | N/A |
| Mark value | 0-0xFFFFFFFF | 0xFFFFFFFF | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Boot with firewall config | `test/firewall/001-boot-apply.ci` | Config parsed, tables created | |
| Boot with traffic config | `test/traffic/001-boot-apply.ci` | Config parsed, qdiscs created | |
| Invalid firewall config | `test/parse/firewall-invalid-001.ci` | Config rejected with error message | |
| Invalid traffic config | `test/parse/traffic-invalid-001.ci` | Config rejected with error message | |
| All expression keywords | `test/firewall/005-all-expressions.ci` | Every readable keyword parsed correctly | |

### Future (if deferring any tests)
- None

## Files to Modify

No existing files modified.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | Yes | New modules (this spec) |
| CLI commands | No | Spec-fw-5 |
| Functional test | Yes | `test/firewall/*.ci`, `test/traffic/*.ci`, `test/parse/*.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` — firewall and traffic config |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` — add firewall and traffic-control sections |
| 3 | CLI command added/changed? | No | Spec-fw-5 |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | Yes | `docs/guide/firewall.md`, `docs/guide/traffic-control.md` |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` |
| 12 | Internal architecture changed? | No | - |

## Files to Create

- `internal/component/firewall/schema/ze-firewall-conf.yang` — YANG module
- `internal/component/firewall/schema/register.go` — yang.RegisterModule in init()
- `internal/component/firewall/schema/embed.go` — go:embed
- `internal/component/firewall/config.go` — config parser: YANG tree → []Table
- `internal/component/firewall/config_test.go` — parser unit tests
- `internal/component/firewall/register.go` — component registration, OnConfigVerify/OnConfigReload
- `internal/component/traffic/schema/ze-traffic-control-conf.yang` — YANG module
- `internal/component/traffic/schema/register.go` — yang.RegisterModule in init()
- `internal/component/traffic/schema/embed.go` — go:embed
- `internal/component/traffic/config.go` — config parser: YANG tree → map[string]InterfaceQoS
- `internal/component/traffic/config_test.go` — parser unit tests
- `internal/component/traffic/register.go` — component registration
- `test/parse/firewall-invalid-001.ci` — invalid config test
- `test/parse/traffic-invalid-001.ci` — invalid config test

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + fw-0 + fw-1 |
| 2. Audit | Files to Create |
| 3. Implement (TDD) | Phases below |
| 4-12 | Standard flow |

### Implementation Phases

1. **Phase: YANG modules** — write ze-firewall-conf.yang, ze-traffic-control-conf.yang
   - Tests: TestYANGModuleRegistered
   - Files: schema/*.yang, schema/register.go, schema/embed.go
   - Verify: module loads without error

2. **Phase: Firewall config parser** — YANG tree → []Table
   - Tests: TestParseFirewallTable, TestParseBaseChain, TestParseRuleExpressions, TestParseNamedSet
   - Files: firewall/config.go
   - Verify: tests fail → implement → tests pass

3. **Phase: Traffic config parser** — YANG tree → map[string]InterfaceQoS
   - Tests: TestParseTrafficHTB, TestParseTrafficHFSC, TestParseTrafficInvalid
   - Files: traffic/config.go
   - Verify: tests fail → implement → tests pass

4. **Phase: Component registration** — OnConfigVerify, OnConfigReload, env vars
   - Tests: functional .ci tests
   - Files: firewall/register.go, traffic/register.go
   - Verify: tests fail → implement → tests pass

5. **Full verification** → `make ze-verify`

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every readable keyword in mapping table has parser code |
| Correctness | ze_ prefix prepended to table names |
| Naming | YANG keywords match readable keyword table exactly |
| Data flow | Config → parser → model structs → backend Apply |
| Rule: config-design | Unknown keys rejected, not silently ignored |
| Rule: explicit > implicit | No default values silently applied without logging |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| YANG modules load | `grep "ze-firewall-conf" internal/component/firewall/schema/` |
| Config parser handles all keywords | Test coverage of readable keyword table |
| ze_ prefix applied | TestTableNamePrefix passes |
| Invalid config rejected | test/parse/*.ci pass |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Input validation | All config values validated: IP addresses, ports, rates, interface names |
| Unknown keys | Rejected at parse time, not silently ignored |
| Table name injection | Table names sanitized (alphanumeric + underscore only after ze_ prefix) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| YANG parse error | Check YANG syntax against ze-types.yang |
| Config parse wrong expression | Re-check readable keyword mapping table |
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
- [ ] AC-1..AC-18 all demonstrated
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
- [ ] Write learned summary to `plan/learned/NNN-fw-4-yang-config.md`
- [ ] Summary included in commit
