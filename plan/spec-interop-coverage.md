# Spec: interop-coverage

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-03-21 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/testing/interop.md` - interop test architecture
4. `test/interop/interop.py` - daemon helper classes
5. Existing scenario configs for pattern reference (07, 10, 13)

## Task

Expand the live interop test suite to cover gaps documented in `docs/architecture/testing/interop.md` lines 208-214. The goal is to add scenarios that exercise address families and features currently validated only by ExaBGP wire compat (or not at all) against live BGP daemon peers.

### Scope

**In scope (new interop scenarios):**

| # | Scenario | Daemons | Gap Addressed |
|---|----------|---------|---------------|
| 20 | evpn-frr | Ze, FRR | EVPN sessions with live peers |
| 21 | vpn-frr | Ze, FRR | VPN (L3VPN) sessions with live peers |
| 22 | flowspec-frr | Ze, FRR | FlowSpec sessions with live peers |
| 23 | ipv6-ebgp-bird | Ze, BIRD | IPv6 sessions with BIRD |
| 24 | ipv6-ebgp-gobgp | Ze, GoBGP | IPv6 sessions with GoBGP |
| 25 | multihop-ebgp-frr | Ze, FRR | Multi-hop eBGP |

**Out of scope (blocked on unfinished implementation):**

| Feature | Blocker | Destination |
|---------|---------|-------------|
| Long-Lived Graceful Restart | LLGR not implemented; `spec-llgr-0-umbrella.md` (status: design) | Add interop scenario to LLGR child spec when implementation is done |
| BFD | No BFD protocol implementation exists in Ze | Requires its own spec for BFD protocol support first |

### Rationale for Scenario Selection

EVPN, VPN, and FlowSpec encoding is already validated byte-for-byte via the ExaBGP compat suite. However, a live session test proves that capability negotiation, MP_REACH/MP_UNREACH framing, and route exchange work end-to-end with a real peer -- something wire encoding tests cannot validate.

IPv6 is tested with FRR (scenario 10) but not with BIRD or GoBGP. Each daemon has its own MP_REACH implementation and capability negotiation quirks.

Multi-hop eBGP exercises the `outgoing-ttl` config leaf and proves Ze can establish sessions across routed hops (simulated via Docker networking with TTL > 1).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/interop.md` - interop test framework, scenario inventory, daemon helpers
  -> Constraint: each scenario is a directory with ze.conf (required), peer config, check.py (required)
  -> Constraint: daemon starts conditionally based on which config files exist (frr.conf, bird.conf, gobgp.toml)
  -> Decision: containers use 172.30.0.0/24 network; Ze=.2, FRR=.3, BIRD=.4, GoBGP=.5
- [ ] `docs/architecture/testing/ci-format.md` - functional test format (not used for interop, but related context)

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc7432.md` - EVPN (BGP MPLS-Based Ethernet VPN)
  -> Constraint: EVPN uses AFI 25 / SAFI 70, Type-2 MAC/IP Advertisement route for basic testing
- [ ] `rfc/short/rfc4364.md` - L3VPN (BGP/MPLS IP VPNs)
  -> Constraint: VPN uses AFI 1 / SAFI 128 (ipv4/vpn), Route Distinguisher + label in NLRI
- [ ] `rfc/short/rfc8955.md` - FlowSpec (Dissemination of Flow Specification Rules)
  -> Constraint: FlowSpec uses AFI 1 / SAFI 133 (ipv4/flow), match criteria + actions in NLRI
- [ ] `rfc/short/rfc4271.md` - BGP-4 base (Section 5.1.3: EBGP multihop TTL handling)
  -> Constraint: eBGP default TTL=1; multi-hop requires explicit TTL configuration

**Key insights:**
- EVPN/VPN/FlowSpec encoding already passes ExaBGP wire compat (38 tests); live interop proves session negotiation + route exchange
- Multi-hop eBGP config exists (YANG `outgoing-ttl` leaf, `peersettings.go` applies it)
- BIRD and GoBGP already have container infrastructure (scenarios 02, 06, 18, 19); just need IPv6 configs
- FRR supports EVPN, VPN, FlowSpec natively; BIRD and GoBGP have partial support

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `test/interop/interop.py` - orchestrator helpers (FRR, BIRD, GoBGP, Ze classes)
- [ ] `test/interop/run.py` - scenario runner, image builds, pass/fail reporting
- [ ] `test/interop/scenarios/10-ipv6-ebgp-frr/` - IPv6 pattern (ze.conf, frr.conf, announce-v6.py, check.py)
- [ ] `test/interop/scenarios/07-routes-to-frr/` - route announcement pattern (process plugin + check)
- [ ] `internal/component/bgp/schema/ze-bgp-conf.yang` - ttl-security, outgoing-ttl, incoming-ttl leaves exist
- [ ] `internal/component/bgp/reactor/peersettings.go` - TTL settings applied to peer connections

**Behavior to preserve:**
- Existing 19 scenarios continue passing unchanged
- Docker network layout (172.30.0.0/24) and container naming convention
- Daemon helper API (FRR.wait_session, check_route, etc.)
- Plugin announcement pattern (ze_api.py with ready/flush/wait_for_shutdown)

**Behavior to change:**
- None -- this is purely additive (new scenario directories)

## Data Flow (MANDATORY)

### Entry Point
- Ze config file with address family declarations (e.g., `family { l2vpn/evpn; }`)
- Process plugin script announcing routes via `ze_api.flush()` using text command syntax

### Transformation Path
1. Ze reads config, negotiates capabilities with peer (MP_REACH for the target family)
2. Process plugin sends announcement command via IPC
3. Ze builds UPDATE with MP_REACH_NLRI for the negotiated family
4. Peer daemon (FRR/BIRD/GoBGP) receives UPDATE, installs routes in its RIB
5. check.py queries peer daemon CLI to verify routes arrived

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Ze config -> BGP session | YANG config parsed, peer settings applied, session established | [ ] |
| Plugin -> Ze engine | Text command via IPC (ze_api.flush) | [ ] |
| Ze -> Peer daemon | BGP UPDATE over TCP with MP_REACH_NLRI | [ ] |
| check.py -> Peer CLI | Docker exec + vtysh/birdc/gobgp CLI | [ ] |

### Integration Points
- `bgp-nlri-evpn` plugin (registered via init, provides encode/decode for l2vpn/evpn family)
- `bgp-nlri-vpn` plugin (provides encode/decode for ipv4/vpn, ipv6/vpn)
- `bgp-nlri-flowspec` plugin (provides encode/decode for ipv4/flow, ipv6/flow)
- `peersettings.go` OutgoingTTL field (applied to TCP connection for multi-hop)
- `test/scripts/ze_api.py` (plugin announcement library)

### Architectural Verification
- [ ] No bypassed layers -- routes flow through normal plugin -> reactor -> wire path
- [ ] No unintended coupling -- new scenarios are self-contained directories
- [ ] No duplicated functionality -- reuses existing interop.py helpers
- [ ] Zero-copy preserved -- no changes to engine code

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| ze.conf with `l2vpn/evpn` family | -> | bgp-nlri-evpn encode/decode + MP_REACH framing | `test/interop/scenarios/20-evpn-frr/check.py` |
| ze.conf with `ipv4/vpn` family | -> | bgp-nlri-vpn encode/decode + MP_REACH framing | `test/interop/scenarios/21-vpn-frr/check.py` |
| ze.conf with `ipv4/flow` family | -> | bgp-nlri-flowspec encode/decode + MP_REACH framing | `test/interop/scenarios/22-flowspec-frr/check.py` |
| ze.conf with `ipv6/unicast` + BIRD peer | -> | builtin IPv6 encode + MP_REACH with BIRD | `test/interop/scenarios/23-ipv6-ebgp-bird/check.py` |
| ze.conf with `ipv6/unicast` + GoBGP peer | -> | builtin IPv6 encode + MP_REACH with GoBGP | `test/interop/scenarios/24-ipv6-ebgp-gobgp/check.py` |
| ze.conf with `outgoing-ttl 2` | -> | peersettings TTL application | `test/interop/scenarios/25-multihop-ebgp-frr/check.py` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Ze configured with `l2vpn/evpn` family, FRR peer configured for EVPN | Session establishes, Ze announces EVPN Type-2 route, FRR receives it in `show bgp l2vpn evpn` |
| AC-2 | Ze configured with `ipv4/vpn` family, FRR peer configured for VPNv4 | Session establishes, Ze announces VPN route with RD+label, FRR receives it in `show bgp ipv4 vpn` |
| AC-3 | Ze configured with `ipv4/flow` family, FRR peer configured for FlowSpec | Session establishes, Ze announces FlowSpec rule, FRR receives it in `show bgp ipv4 flowspec` |
| AC-4 | Ze configured with `ipv6/unicast` family, BIRD peer | Session establishes, Ze announces IPv6 routes, BIRD receives them |
| AC-5 | Ze configured with `ipv6/unicast` family, GoBGP peer | Session establishes, Ze announces IPv6 routes, GoBGP receives them |
| AC-6 | Ze configured with `outgoing-ttl 2`, FRR peer 2+ hops away | eBGP session establishes despite non-adjacent peers (TTL > 1) |
| AC-7 | All 6 new scenarios added | `make ze-interop-test` runs all 25 scenarios without failure |
| AC-8 | Interop doc updated | `docs/architecture/testing/interop.md` scenario inventory includes scenarios 20-25, "not yet covered" list updated |

## TDD Test Plan

### Unit Tests

No new unit tests needed -- this spec adds interop scenarios (Python check scripts), not Go code. The existing NLRI plugins and peersettings already have unit test coverage.

### Boundary Tests (MANDATORY for numeric inputs)

Not applicable -- no new numeric inputs in Ze code.

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `20-evpn-frr` | `test/interop/scenarios/20-evpn-frr/check.py` | EVPN session with FRR, Ze announces Type-2 route | |
| `21-vpn-frr` | `test/interop/scenarios/21-vpn-frr/check.py` | VPN session with FRR, Ze announces VPNv4 route | |
| `22-flowspec-frr` | `test/interop/scenarios/22-flowspec-frr/check.py` | FlowSpec session with FRR, Ze announces flow rule | |
| `23-ipv6-ebgp-bird` | `test/interop/scenarios/23-ipv6-ebgp-bird/check.py` | IPv6 session with BIRD, Ze announces IPv6 routes | |
| `24-ipv6-ebgp-gobgp` | `test/interop/scenarios/24-ipv6-ebgp-gobgp/check.py` | IPv6 session with GoBGP, Ze announces IPv6 routes | |
| `25-multihop-ebgp-frr` | `test/interop/scenarios/25-multihop-ebgp-frr/check.py` | Multi-hop eBGP with FRR using outgoing-ttl | |

### Future (if deferring any tests)
- LLGR interop test: deferred to `spec-llgr-0-umbrella.md` (LLGR not yet implemented)
- BFD interop test: deferred until BFD protocol support spec is created

## Files to Modify

- `docs/architecture/testing/interop.md` - update scenario inventory table (add 20-25), update "not yet covered" section

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | - |
| RPC count in architecture docs | No | - |
| CLI commands/flags | No | - |
| CLI usage/help text | No | - |
| API commands doc | No | - |
| Plugin SDK docs | No | - |
| Editor autocomplete | No | - |
| Functional test for new RPC/API | No | - |

## Files to Create

### Scenario 20: EVPN with FRR
- `test/interop/scenarios/20-evpn-frr/ze.conf` - Ze config with l2vpn/evpn family + process plugin
- `test/interop/scenarios/20-evpn-frr/frr.conf` - FRR config with EVPN address family
- `test/interop/scenarios/20-evpn-frr/announce-evpn.py` - plugin announcing EVPN Type-2 route
- `test/interop/scenarios/20-evpn-frr/check.py` - verify FRR received EVPN route

### Scenario 21: VPN with FRR
- `test/interop/scenarios/21-vpn-frr/ze.conf` - Ze config with ipv4/vpn family + process plugin
- `test/interop/scenarios/21-vpn-frr/frr.conf` - FRR config with VPNv4 address family
- `test/interop/scenarios/21-vpn-frr/announce-vpn.py` - plugin announcing VPNv4 route with RD + RT
- `test/interop/scenarios/21-vpn-frr/check.py` - verify FRR received VPN route

### Scenario 22: FlowSpec with FRR
- `test/interop/scenarios/22-flowspec-frr/ze.conf` - Ze config with ipv4/flow family + process plugin
- `test/interop/scenarios/22-flowspec-frr/frr.conf` - FRR config with FlowSpec address family
- `test/interop/scenarios/22-flowspec-frr/announce-flowspec.py` - plugin announcing FlowSpec rule
- `test/interop/scenarios/22-flowspec-frr/check.py` - verify FRR received FlowSpec rule

### Scenario 23: IPv6 with BIRD
- `test/interop/scenarios/23-ipv6-ebgp-bird/ze.conf` - Ze config with ipv6/unicast family + process plugin
- `test/interop/scenarios/23-ipv6-ebgp-bird/bird.conf` - BIRD config with IPv6 unicast
- `test/interop/scenarios/23-ipv6-ebgp-bird/announce-v6.py` - plugin announcing IPv6 routes
- `test/interop/scenarios/23-ipv6-ebgp-bird/check.py` - verify BIRD received IPv6 routes

### Scenario 24: IPv6 with GoBGP
- `test/interop/scenarios/24-ipv6-ebgp-gobgp/ze.conf` - Ze config with ipv6/unicast family + process plugin
- `test/interop/scenarios/24-ipv6-ebgp-gobgp/gobgp.toml` - GoBGP config with IPv6 unicast
- `test/interop/scenarios/24-ipv6-ebgp-gobgp/announce-v6.py` - plugin announcing IPv6 routes
- `test/interop/scenarios/24-ipv6-ebgp-gobgp/check.py` - verify GoBGP received IPv6 routes

### Scenario 25: Multi-hop eBGP with FRR
- `test/interop/scenarios/25-multihop-ebgp-frr/ze.conf` - Ze config with outgoing-ttl 2
- `test/interop/scenarios/25-multihop-ebgp-frr/frr.conf` - FRR config with ebgp-multihop 2
- `test/interop/scenarios/25-multihop-ebgp-frr/announce-routes.py` - plugin announcing routes
- `test/interop/scenarios/25-multihop-ebgp-frr/check.py` - verify session establishes and routes arrive

### Documentation Update Checklist (BLOCKING)
<!-- Every row MUST be answered Yes/No during the Completion Checklist (planning.md step 1). -->
<!-- Every Yes MUST name the file and what to add/change. -->
<!-- See planning.md "Documentation Update Checklist" for the full table with examples. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | `docs/features.md` |
| 2 | Config syntax changed? | [ ] | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` |
| 3 | CLI command added/changed? | [ ] | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | [ ] | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | [ ] | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | [ ] | `docs/guide/<topic>.md` |
| 7 | Wire format changed? | [ ] | `docs/architecture/wire/*.md` |
| 8 | Plugin SDK/protocol changed? | [ ] | `.claude/rules/plugin-design.md`, `docs/architecture/api/process-protocol.md` |
| 9 | RFC behavior implemented? | [ ] | `rfc/short/rfcNNNN.md` |
| 10 | Test infrastructure changed? | [ ] | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | [ ] | `docs/comparison.md` |
| 12 | Internal architecture changed? | [ ] | `docs/architecture/core-design.md` or subsystem doc |

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Create -- check what exists |
| 3. Implement (TDD) | Implementation phases below |
| 4. Full verification | `make ze-interop-test` (requires Docker) |
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

1. **Phase: IPv6 with BIRD and GoBGP** -- simplest scenarios, extend existing patterns
   - Create scenarios 23 and 24 by adapting scenario 10 (ipv6-ebgp-frr) configs for BIRD and GoBGP
   - Reuse the existing announce-v6.py plugin script pattern
   - Verify: `make ze-interop-test INTEROP_SCENARIO=23-ipv6-ebgp-bird` and `24-ipv6-ebgp-gobgp`

2. **Phase: EVPN with FRR** -- new address family, requires EVPN-specific announce script and FRR EVPN config
   - Create scenario 20 with l2vpn/evpn family, EVPN Type-2 MAC/IP route announcement
   - FRR needs `address-family l2vpn evpn` with appropriate EVPN configuration
   - Ze process plugin announces via text command with appropriate EVPN NLRI format
   - Verify: `make ze-interop-test INTEROP_SCENARIO=20-evpn-frr`

3. **Phase: VPN with FRR** -- VPNv4 family with Route Distinguisher and Route Target
   - Create scenario 21 with ipv4/vpn family, VPN route with RD and RT extended community
   - FRR needs `address-family ipv4 vpn` configuration
   - Verify: `make ze-interop-test INTEROP_SCENARIO=21-vpn-frr`

4. **Phase: FlowSpec with FRR** -- FlowSpec family with match/action rules
   - Create scenario 22 with ipv4/flow family, FlowSpec rule announcement
   - FRR needs `address-family ipv4 flowspec` configuration
   - Verify: `make ze-interop-test INTEROP_SCENARIO=22-flowspec-frr`

5. **Phase: Multi-hop eBGP with FRR** -- TTL configuration for non-adjacent peers
   - Create scenario 25 with `outgoing-ttl 2` in Ze config and `ebgp-multihop 2` in FRR
   - May need Docker network tweaks to simulate non-adjacent peers (TTL check enforcement)
   - Verify: `make ze-interop-test INTEROP_SCENARIO=25-multihop-ebgp-frr`

6. **Phase: Documentation update** -- update interop.md scenario inventory
   - Add scenarios 20-25 to the inventory table
   - Update "not yet covered" section (remove items now covered, keep LLGR and BFD)
   - Verify: doc review

7. **Full verification** -- `make ze-interop-test` runs all 25 scenarios

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All 6 scenarios created with ze.conf + peer config + announce script + check.py |
| Correctness | Each check.py verifies the specific address family routes arrived at the peer daemon |
| Naming | Scenario directories follow NN-description convention, numbered 20-25 |
| Data flow | Routes flow through normal plugin -> reactor -> wire -> peer path |
| Config syntax | Ze configs use correct YANG family names (l2vpn/evpn, ipv4/vpn, ipv4/flow) |
| FRR config | FRR address-family blocks match the tested family |
| BIRD config | BIRD protocol block uses correct IPv6 channel |
| GoBGP config | GoBGP TOML uses correct AFI/SAFI for IPv6 |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| Scenario 20 directory with 4 files | `ls test/interop/scenarios/20-evpn-frr/` |
| Scenario 21 directory with 4 files | `ls test/interop/scenarios/21-vpn-frr/` |
| Scenario 22 directory with 4 files | `ls test/interop/scenarios/22-flowspec-frr/` |
| Scenario 23 directory with 4 files | `ls test/interop/scenarios/23-ipv6-ebgp-bird/` |
| Scenario 24 directory with 4 files | `ls test/interop/scenarios/24-ipv6-ebgp-gobgp/` |
| Scenario 25 directory with 4 files | `ls test/interop/scenarios/25-multihop-ebgp-frr/` |
| interop.md updated | grep "20-evpn-frr" docs/architecture/testing/interop.md |
| "Not yet covered" updated | grep "LLGR" and "BFD" still present, EVPN/VPN/FlowSpec/IPv6/multihop removed |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | check.py scripts validate route content, not just presence |
| Container isolation | Scenarios use standard Docker network, no host networking |
| Credential exposure | No secrets in config files (MD5 scenario 17 already has pattern) |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Session does not establish | Check capability negotiation -- peer may not support the address family |
| Routes not received by peer | Check MP_REACH_NLRI encoding -- compare with ExaBGP compat test output |
| FRR rejects route | Check NLRI format matches RFC expectations (RD, label, EVPN type) |
| BIRD/GoBGP IPv6 fails | Check MP_REACH next-hop encoding (link-local vs global) |
| Multi-hop session fails | Check TTL is actually applied -- verify with tcpdump in container |
| Docker network issues | Check container IPs, network creation, port conflicts |
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

Not applicable -- no new Go code, only test scenarios. RFC compliance is validated by the scenarios themselves.

## Implementation Summary

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered -- add test for each]

### Documentation Updates
- [Docs updated, or "None"]

### Deviations from Plan
- [Differences from original plan and why]

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
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

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
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-interop-test` passes all 25 scenarios
- [ ] Feature code integrated (test scenarios only -- no engine changes)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated (interop.md)
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
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
- [ ] Critical Review passes -- all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary. One commit = code + tests + summary.
