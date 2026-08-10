# Spec: cgnat

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-06-10 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/core-design.md` - core design, firewall backend abstraction
4. `docs/architecture/firewall/fw-6-firewall-vpp.md` - VPP firewall backend decisions
5. `internal/plugins/firewall/vpp/nat_linux.go` - current NAT44 VPP path
6. `internal/plugins/firewall/vpp/backend_linux.go` - VPP backend, govpp ops
7. `internal/component/firewall/model.go` - abstract firewall data model
8. `internal/component/firewall/config.go` - config parsing (SNAT, DNAT, masquerade)
9. `internal/component/firewall/backend.go` - Backend interface, registry

## Task

Add Carrier-Grade NAT (CGNAT) support to Ze. CGNAT maps many inside (subscriber) addresses
onto a small pool of outside (public) addresses using deterministic or dynamic port-block
allocation. This is ISP infrastructure for IPv4 address conservation (RFC 6888, RFC 7857).

The feature must provide:
1. CGNAT configuration under the firewall component (inside prefix, outside pool, ports-per-user, exclusions)
2. VPP backend implementation using the already-vendored nat44_ed binapi (address ranges, interface features, session limits, identity mappings for exclusions)
3. nftables backend implementation using nft NAT chains with address/port-range mapping
4. NAT session visibility: CLI commands and web UI to show active NAT users, sessions, and port allocations
5. NAT session logging: integration with the flowexport component for IPFIX/NetFlow NAT event export (RFC 8158 NAT logging)

**Motivation:** The VyOS forum post (VPP CGNAT port allocation log) highlights a real ISP
operational need: knowing which subscriber held which public address+port at a given time,
for compliance and troubleshooting. Ze already has the NAT44-ED VPP binapi vendored and
basic SNAT/DNAT/masquerade wired. CGNAT extends this to carrier scale with allocation tracking.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - firewall data model, backend abstraction
  -> Decision: firewall uses abstract Match/Action types lowered by per-backend code
  -> Constraint: Backend interface is Apply(desired []Table), ListTables, GetCounters, Close
- [ ] `docs/architecture/firewall/fw-6-firewall-vpp.md` - VPP firewall backend design history
  -> Decision: ACL-only scope at launch; NAT44, classify, policer added later via same ops seam
  -> Constraint: fakeOps-based tests mandatory for all VPP backends

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc6888.md` - Common requirements for CGNAT (RFC 6888)
  -> Constraint: REQ-1 through REQ-14 define CGNAT behavior: endpoint-independent mapping, port parity, hairpinning, fragment handling, logging
- [ ] `rfc/short/rfc7857.md` - Updates to CGNAT requirements (RFC 7857)
  -> Constraint: updates to REQ-5 (EIM not mandated), REQ-8 (logging format)
- [ ] `rfc/short/rfc4787.md` - NAT behavioral requirements for UDP (RFC 4787)
  -> Constraint: address-dependent mapping, port preservation, binding timeout
- [ ] `rfc/short/rfc8158.md` - IPFIX information elements for logging NAT events
  -> Constraint: defines IE 225 (natEvent), IE 226-230 for session create/delete/quota events

**Key insights:**
- Ze already vendors govpp nat44_ed binapi with full user/session dump APIs (Nat44UserDump, Nat44UserSessionV3Dump) that are not wired
- The firewall model already has SNAT (with address range), DNAT, Masquerade, and identity mapping concepts
- The VPP ops seam (vppOps interface) is the extension point for new nat44 calls
- flowexport component already has conntrack, IPFIX, NetFlow9, sFlow encoders and exporter infrastructure
- No det44 (deterministic NAT) binapi is vendored; nat44_ed with address pools is the available VPP path

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/firewall/vpp/nat_linux.go` - applyNATChains: masquerade (all-interfaces output), SNAT (address range + inside feature), DNAT (static mapping with tag), orphan cleanup
- [ ] `internal/plugins/firewall/vpp/ops.go` - vppOps interface: nat44Enable, nat44AddDelAddressRange, nat44AddDelStaticMapping, nat44AddDelOutputInterface, nat44AddDelInterfaceFeature, nat44StaticMappingDump
- [ ] `internal/plugins/firewall/vpp/backend_linux.go` - govppOps adapter: enables NAT44-ED with endpoint-dependent flag, uses Nat44AddDelStaticMappingV2, Nat44AddDelAddressRange, Nat44InterfaceAddDelFeature, Nat44EdAddDelOutputInterface
- [ ] `internal/component/firewall/model.go` - SNAT{Address, AddressEnd, Port, PortEnd, Flags}, DNAT{same}, Masquerade{Port, PortEnd, Flags}
- [ ] `internal/component/firewall/config.go` - parseNATSpec (addr, addr range, port, port range), parseMasquerade, parseThenBlock handles snat/dnat/masquerade/redirect/exclude keywords
- [ ] `internal/component/firewall/backend.go` - Backend interface (Apply, ListTables, GetCounters, Close), Verifier, registry
- [ ] `internal/component/flowexport/` - conntrack worker, IPFIX/NF9/sFlow encoders, exporter with collectors, sampling, enrichment

**Behavior to preserve:**
- Existing SNAT, DNAT, masquerade, redirect firewall actions continue to work unchanged
- VPP ops seam pattern: ops interface with fakeOps for testing
- Firewall backend registry and verifier pattern
- flowexport collector/exporter architecture
- YANG config schema for existing firewall tables/chains/terms
- CLI `show firewall` output format

**Behavior to change:**
- Add new `cgnat` config section under `firewall` (or as a new component)
- Add new vppOps methods for user dump, session dump, session limits, identity mapping
- Add new CLI commands for CGNAT status/session visibility
- Add NAT event logging integration with flowexport

## Data Flow (MANDATORY)

### Entry Point
- Config: YANG `firewall { cgnat { ... } }` parsed during OnConfigVerify/OnConfigure
- CLI: `show nat cgnat users`, `show nat cgnat sessions <ip>`, `show nat cgnat allocation`
- Logging: periodic or event-driven NAT session export via flowexport component

### Transformation Path
1. YANG config parsed into CGNAT model types (inside prefixes, outside pool, ports-per-user, exclusions, inside/outside interfaces)
2. Verifier checks backend compatibility and parameter validity (pool size vs subscriber count vs ports-per-user)
3. Backend Apply: VPP path calls nat44 enable, address range, interface features, session limits, identity mappings for exclusions; nft path creates NAT chains with address-mapped SNAT rules
4. Session dump: CLI/web handler calls VPP Nat44UserDump + Nat44UserSessionV3Dump (VPP) or reads conntrack (nft) to enumerate active translations
5. Logging: NAT session create/delete events exported as IPFIX records using RFC 8158 information elements

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config -> CGNAT model | YANG parse -> Go structs | [ ] |
| CGNAT model -> VPP | vppOps interface calls | [ ] |
| CGNAT model -> nftables | nft NAT chain lowering | [ ] |
| VPP session table -> CLI | Nat44UserSessionV3Dump via govpp | [ ] |
| NAT events -> flowexport | event feed into IPFIX encoder | [ ] |

### Integration Points
- `firewall.Backend.Apply` - extended to handle CGNAT desired state alongside existing tables
- `vppOps` interface - new methods: nat44UserDump, nat44UserSessionV3Dump, nat44SetSessionLimit, nat44AddDelIdentityMapping
- `flowexport` component - NAT event source alongside existing conntrack source
- CLI command registry - new `show nat cgnat` subtree
- Web UI - new CGNAT status page (SSE-capable for live session view)

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| YANG cgnat config | -> | CGNAT model parse + verify | TestCGNATConfigParse |
| firewall Apply with CGNAT | -> | VPP nat44 address range + interface + session limit | TestCGNATApplyVPP |
| firewall Apply with CGNAT | -> | nft NAT chain generation | TestCGNATApplyNFT |
| CLI `show nat cgnat users` | -> | VPP Nat44UserDump via ops | TestCGNATShowUsers |
| CLI `show nat cgnat sessions` | -> | VPP Nat44UserSessionV3Dump via ops | TestCGNATShowSessions |
| NAT event feed | -> | flowexport IPFIX NAT event records | TestCGNATFlowExport |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | YANG config with cgnat section (inside prefix 100.64.0.0/10, outside pool 203.0.113.0-203.0.113.15, ports-per-user 1024) | Config parses and verifies successfully; pool arithmetic validated (pool size * ports-per-user >= subscriber capacity) |
| AC-2 | CGNAT config applied with VPP backend | VPP nat44 enabled, address range programmed, inside/outside interfaces marked, session limit set matching ports-per-user |
| AC-3 | CGNAT config with exclusion list (e.g., 100.64.0.1/32 bypass NAT) | Excluded addresses programmed as identity mappings (VPP) or return rules (nft), traffic bypasses NAT |
| AC-4 | CLI `show nat cgnat users` | Lists all inside addresses with active sessions, session count, static session count |
| AC-5 | CLI `show nat cgnat sessions <inside-ip>` | Lists per-session detail: inside ip:port, outside ip:port, protocol, external host, bytes/packets, last heard, timed-out flag |
| AC-6 | CGNAT config applied with nft backend | nftables NAT chain created with SNAT rules mapping inside prefix to outside pool |
| AC-7 | NAT event logging enabled in config | NAT session create/delete events exported as IPFIX records with RFC 8158 information elements (natEvent, timestamps, addresses, ports) |
| AC-8 | Pool exhaustion: more subscribers than port capacity | Config verify rejects with clear error message showing arithmetic (N subscribers * M ports > pool capacity) |
| AC-9 | Show command when no CGNAT configured | Clean "CGNAT not configured" message, not an error |
| AC-10 | CGNAT config with VPP backend, port-block allocation logging enabled | Periodic dump of user-to-port-block mapping written to syslog/audit and available via CLI |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestCGNATConfigParse | `internal/component/firewall/cgnat_config_test.go` | YANG cgnat section parsing into CGNAT model | |
| TestCGNATConfigVerify | `internal/component/firewall/cgnat_verify_test.go` | Pool arithmetic validation, exclusion parsing, boundary checks | |
| TestCGNATApplyVPP | `internal/plugins/firewall/vpp/cgnat_test.go` | fakeOps verifies correct nat44 API call sequence | |
| TestCGNATShowUsers | `internal/plugins/firewall/vpp/cgnat_show_test.go` | fakeOps returns user list, formatted for CLI | |
| TestCGNATShowSessions | `internal/plugins/firewall/vpp/cgnat_show_test.go` | fakeOps returns session list for a user, formatted for CLI | |
| TestCGNATExclusion | `internal/plugins/firewall/vpp/cgnat_test.go` | Identity mapping programmed for excluded addresses | |
| TestCGNATApplyNFT | `internal/plugins/firewall/nft/cgnat_test.go` | Correct nft NAT chain generated | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| ports-per-user | 64-65536 | 65536 | 63 | 65537 |
| outside pool size | 1-65536 addresses | 65536 | 0 | N/A (prefix bounds) |
| session-limit | 0 (unlimited) to 2^32-1 | 4294967295 | N/A | 4294967296 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| cgnat-vpp-basic | `test/plugin/cgnat-vpp-basic.ci` | Configure CGNAT with VPP backend, verify NAT users dump works | |
| cgnat-show-users | `test/plugin/cgnat-show-users.ci` | show nat cgnat users returns formatted table | |
| cgnat-show-sessions | `test/plugin/cgnat-show-sessions.ci` | show nat cgnat sessions <ip> returns per-session detail | |
| cgnat-exclusion | `test/plugin/cgnat-exclusion.ci` | Excluded address bypasses NAT | |
| cgnat-nft-basic | `test/plugin/cgnat-nft-basic.ci` | Configure CGNAT with nft backend, verify NAT chain created | |

### Interop Tests (MANDATORY for protocol features)
N/A. CGNAT is a local dataplane feature, not a wire protocol. The IPFIX export is covered by existing flowexport interop infrastructure.

### Future (if deferring any tests)
- Real VPP integration tests with traffic deferred until VPP CI infrastructure is built (consistent with fw-6 decision)
- IPFIX NAT event interop with external collector deferred to flowexport spec

## Files to Modify
- `internal/component/firewall/engine.go` - wire CGNAT config parsing into OnConfigVerify/OnConfigure
- `internal/plugins/firewall/vpp/ops.go` - add nat44UserDump, nat44UserSessionV3Dump, nat44SetSessionLimit, nat44AddDelIdentityMapping to vppOps
- `internal/plugins/firewall/vpp/backend_linux.go` - govppOps adapters for new nat44 methods
- `internal/component/firewall/yang/` - YANG schema additions for cgnat section

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | [x] | `internal/component/firewall/yang/` - cgnat container with inside-prefix, outside-pool, ports-per-user, exclusion, session-limit, logging leaves |
| YANG validation constraints | [x] | ports-per-user: range 64..65536; pool addresses: ipv4-prefix; session-limit: uint32 |
| YANG custom validators | [x] | Pool arithmetic cross-validation (pool capacity vs subscriber count) |
| CLI commands/flags | [x] | `show nat cgnat users`, `show nat cgnat sessions <ip>`, `show nat cgnat allocation` |
| CLI grammar (action before identifier) | [x] | `show nat cgnat` follows existing `show firewall` pattern |
| Editor autocomplete | [x] | YANG enum/type for cgnat leaves |
| Functional test for new RPC/API | [x] | `test/plugin/cgnat-*.ci` |
| Pipe completeness | [x] | show nat cgnat output routed through ApplyPipes |
| Env var registration | [ ] | N/A unless environment/ leaves added |
| Doctor check for runtime dependencies | [x] | VPP running check (already exists), nft availability (already exists) |
| Prometheus counters/metrics | [x] | `ze_cgnat_active_users`, `ze_cgnat_active_sessions`, `ze_cgnat_pool_utilization` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` - CGNAT row |
| 2 | Config syntax changed? | [x] | `docs/guide/configuration.md` - cgnat section |
| 3 | CLI command added/changed? | [x] | `docs/guide/command-reference.md` - show nat cgnat |
| 4 | API/RPC added/changed? | [x] | `docs/architecture/api/commands.md` - cgnat show RPCs |
| 5 | Plugin added/changed? | [ ] | N/A - part of firewall component |
| 6 | Has a user guide page? | [x] | `docs/guide/cgnat.md` - new |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [x] | `rfc/short/rfc6888.md`, `rfc/short/rfc7857.md`, `rfc/short/rfc8158.md` |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [x] | `docs/comparison.md` - CGNAT capability |
| 12 | Internal architecture changed? | [x] | `docs/architecture/core-design.md` - CGNAT subsystem |
| 13 | Route metadata keys added/changed? | [ ] | N/A |
| 14 | Prometheus counters added/changed? | [x] | Metrics doc for cgnat counters |
| 15 | Registered plugin, event type, send type, command, capability, or runtime inventory changed? | [x] | CLI command registration |
| 16 | Any changed source file is referenced by existing doc source anchors? | [ ] | Check after implementation |
| 17 | Existing docs show config/CLI/API examples for this area? | [ ] | Check after implementation |

## Files to Create
- `internal/component/firewall/cgnat_model.go` - CGNAT data model
- `internal/component/firewall/cgnat_config.go` - CGNAT config parsing
- `internal/component/firewall/cgnat_verify.go` - CGNAT config verification
- `internal/component/firewall/cgnat_config_test.go` - config parsing tests
- `internal/component/firewall/cgnat_verify_test.go` - verification tests
- `internal/component/firewall/cmd/show_cgnat.go` - CLI show formatting
- `internal/component/firewall/cmd/show_cgnat_test.go` - show formatting tests
- `internal/plugins/firewall/vpp/cgnat_linux.go` - VPP CGNAT apply
- `internal/plugins/firewall/vpp/cgnat_show_linux.go` - VPP user/session dump
- `internal/plugins/firewall/vpp/cgnat_test.go` - VPP CGNAT tests (fakeOps)
- `internal/plugins/firewall/vpp/cgnat_show_test.go` - VPP show tests
- `test/plugin/cgnat-vpp-basic.ci` - functional test
- `test/plugin/cgnat-show-users.ci` - functional test
- `test/plugin/cgnat-show-sessions.ci` - functional test
- `test/plugin/cgnat-exclusion.ci` - functional test
- `test/plugin/cgnat-nft-basic.ci` - functional test
- `docs/guide/cgnat.md` - user guide

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
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

1. **Phase: CGNAT Data Model + Config Parsing** - Define CGNAT config types and parser
   - Tests: TestCGNATConfigParse, TestCGNATConfigVerify
   - Files: cgnat_model.go, cgnat_config.go, cgnat_verify.go, YANG schema
   - Verify: config parses into model types; pool arithmetic validated; boundary tests pass

2. **Phase: VPP Backend CGNAT Apply** - Wire CGNAT model into VPP nat44 calls
   - Tests: TestCGNATApplyVPP, TestCGNATExclusion
   - Files: vpp/ops.go (new methods), vpp/backend_linux.go (govpp adapters), vpp/cgnat_linux.go
   - Verify: fakeOps records correct nat44 call sequence; exclusions become identity mappings

3. **Phase: VPP CGNAT Show (Session Visibility)** - Implement user/session dump via VPP APIs
   - Tests: TestCGNATShowUsers, TestCGNATShowSessions
   - Files: vpp/cgnat_show_linux.go, firewall/cmd/show_cgnat.go
   - Verify: fakeOps returns user/session data; CLI formatting matches expected output

4. **Phase: nft Backend CGNAT** - Generate nft NAT chains for CGNAT config
   - Tests: TestCGNATApplyNFT
   - Files: nft/cgnat_linux.go (or equivalent)
   - Verify: correct nft NAT chain structure generated for inside-prefix to outside-pool mapping

5. **Phase: NAT Event Logging (flowexport integration)** - Export NAT session events as IPFIX
   - Tests: TestCGNATFlowExport
   - Files: flowexport NAT event source, RFC 8158 information elements
   - Verify: NAT create/delete events produce correct IPFIX records

6. **Phase: CLI Wiring + Functional Tests** - Wire show commands, write .ci tests
   - Tests: cgnat-*.ci functional tests
   - Files: CLI registration, .ci test files
   - Verify: end-to-end show commands work in test harness

7. **Phase: Metrics + Documentation** - Prometheus counters, docs
   - Tests: metric registration tests
   - Files: metrics.go additions, docs/guide/cgnat.md, feature docs
   - Verify: metrics exported, docs complete

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1 through AC-10 has implementation with file:line |
| Correctness | Pool arithmetic overflow-safe (uint64); session dump handles VPP unavailable gracefully |
| Naming | YANG leaves use kebab-case (inside-prefix, outside-pool, ports-per-user); Go types use CamelCase |
| Data flow | Config -> model -> backend Apply; show -> ops dump -> format; no direct VPP calls outside ops seam |
| CLI grammar | `show nat cgnat` follows action-before-identifier per cli.md |
| Doctor checks | VPP running check covers CGNAT (existing); no new runtime deps beyond VPP/nft |
| YANG validation | Every numeric leaf has range constraint; pool addresses validated as IPv4 |
| Prometheus counters | cgnat_active_users, cgnat_active_sessions, cgnat_pool_utilization registered |
| Rule: no-layering | No duplicate NAT config path; CGNAT extends existing firewall NAT, does not replace it |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| CGNAT config parses | `go test ./internal/component/firewall/ -run CGNAT` |
| VPP CGNAT apply works | `go test ./internal/plugins/firewall/vpp/ -run CGNAT` |
| VPP session dump works | `go test ./internal/plugins/firewall/vpp/ -run CGNATShow` |
| CLI show nat cgnat | `grep -r 'show.*nat.*cgnat' test/plugin/` |
| Functional tests exist | `ls test/plugin/cgnat-*.ci` |
| YANG schema present | `grep -r 'cgnat' internal/component/firewall/yang/` |
| Docs written | `ls docs/guide/cgnat.md` |
| Metrics registered | `grep -r 'cgnat' internal/component/firewall/metrics.go` or new metrics file |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Inside prefix must be valid IPv4; outside pool addresses must be valid IPv4; ports-per-user must be power-of-2 or within range; session limit within uint32 |
| Resource exhaustion | Session dump on large NAT table could be expensive; implement pagination or limits on CLI dump; VPP channel timeout on large dumps |
| Information disclosure | NAT session table contains subscriber IP mappings; restrict show commands to admin role (existing authz framework) |
| Pool arithmetic overflow | Use uint64 for intermediate calculations; verify pool capacity fits before programming |
| Exclusion bypass | Verify excluded addresses actually bypass NAT; malformed exclusion must not open a hole |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior, then RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural, go to DESIGN phase |
| Functional test fails | Check AC; if AC wrong, go to DESIGN; if AC correct, go to IMPLEMENT |
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

## Core Insight

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Use nat44_ed (endpoint-dependent) not det44 (deterministic) | det44 gives fixed port-block mapping but binapi not vendored; nat44_ed already vendored with full session dump | nat44_ed is available today, provides session visibility via Nat44UserSessionV3Dump, and VPP's ED mode is the actively maintained NAT path |
| CGNAT as extension of firewall component, not separate component | Separate component would have its own lifecycle | NAT is already in the firewall component; CGNAT is "bigger NAT" and reuses the same backend seam and ops interface |
| Session visibility via VPP API dump, not syslog parsing | syslog parsing is fragile and lossy | govpp Nat44UserDump/Nat44UserSessionV3Dump gives structured, complete data directly from VPP's session table |
| NAT logging via flowexport IPFIX, not custom syslog | Syslog is human-readable but not machine-parseable at scale | RFC 8158 defines standard IPFIX IEs for NAT events; flowexport already has IPFIX infrastructure |

## Known Limitations
- No deterministic (det44) port-block allocation: nat44_ed uses dynamic allocation; an operator cannot pre-compute which subscriber gets which port range. Session dump provides after-the-fact visibility instead.
- IPv4 only: CGNAT is an IPv4 address conservation mechanism. NAT64 is a separate feature.
- No per-subscriber QoS integration: port-block limits are enforced by VPP session limits, not by subscriber-aware traffic shaping.
- nft backend CGNAT will have limited session visibility compared to VPP (relies on conntrack, not structured dump).

## RFC Documentation

Add `// RFC 6888 REQ-N: "<quoted requirement>"` above enforcing code.
MUST document: REQ-1 (EIM), REQ-2 (port preservation), REQ-3 (port parity), REQ-8 (logging), REQ-9 (hairpinning), REQ-13 (session limit).

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

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| CGNAT config parses and applies | functional test | cgnat-vpp-basic.ci |
| Session visibility via CLI | functional test | cgnat-show-users.ci, cgnat-show-sessions.ci |
| NAT event logging | unit test | TestCGNATFlowExport |
| Pool arithmetic validation | unit test | TestCGNATConfigVerify |
| Exclusion bypass | functional test | cgnat-exclusion.ci |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- [pending]

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

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments added
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
- [ ] Interop tests for protocol features (or N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Add the closure row to `plan/journal/<class>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-cgnat.md`
