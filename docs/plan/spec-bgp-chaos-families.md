# Spec: bgp-chaos-families (Phase 4 of 5) — SKELETON

**Master design:** `docs/plan/spec-bgp-chaos.md`
**Previous spec:** `spec-bgp-chaos-chaos.md`
**Next spec:** `spec-bgp-chaos-reporting.md`

**Status:** Skeleton — to be fleshed out after Phase 3 completes.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `docs/plan/spec-bgp-chaos.md` - master design (family table, NLRI generation strategies)
3. Phase 1-3 done specs - learnings and actual APIs
4. `docs/architecture/wire/nlri.md` - NLRI encoding per family
5. `.claude/rules/planning.md` - workflow rules

## Task

Extend `ze-bgp-chaos` from ipv4/unicast-only to full multi-family support: ipv6/unicast, VPN, EVPN, and FlowSpec.

**Scope:**
- Route generators for each family (deterministic from seed)
- Per-peer family assignment (not all peers support all families)
- Capability/OPEN negotiation with correct multiprotocol capabilities per peer
- Family-aware validation (peer only expects routes for negotiated families)
- `--families` include filter and `--exclude-families` exclude filter
- Config generation with correct family blocks per peer

## Required Reading

### Architecture Docs
- [ ] `docs/plan/spec-bgp-chaos.md` - master design (family table, NLRI generation)
  → Decision: 7 families supported with distinct generation strategies
  → Constraint: Each peer gets unique prefix block partitioned by seed
- [ ] `docs/architecture/wire/nlri.md` - NLRI encoding per family
  → Constraint: VPN uses RD+prefix, EVPN uses Type-2 MAC/IP, FlowSpec uses match rules
- [ ] `docs/architecture/wire/attributes.md` - family-specific attributes
  → Constraint: MP_REACH_NLRI required for all non-ipv4-unicast families

### Source Code
- [ ] Phase 1-3 implementation files (paths TBD)
- [ ] `internal/plugins/bgp/nlri/` - NLRI types and builders
- [ ] `internal/plugins/bgp-vpn/` - VPN NLRI construction
- [ ] `internal/plugins/bgp-evpn/` - EVPN NLRI construction
- [ ] `internal/plugins/bgp-flowspec/` - FlowSpec NLRI construction

**Key insights:**
- Chaos works per-session, not per-family — all 10 event types target a peer connection, not a specific address family
- No family-specific chaos events needed — disconnects, withdrawals, etc. affect the entire peer session
- Chaos executor lives in `peer/simulator.go` (not a separate executor file) — `executeChaos()` is called from the KEEPALIVE select loop
- Partial/full withdrawal currently uses `BuildWithdrawal()` for IPv4/unicast only — families phase must extend this
- Reconnect storm and connection collision create new TCP connections via `net.Dialer.DialContext` — multi-family OPENs must include correct capabilities
- The `chaos/` package is pure scheduling logic (action types + weighted selection); execution lives in `peer/`

## Current Behavior (MANDATORY)

**Source files read:** (to be re-read after Phase 3 completes)
- [ ] `cmd/ze-bgp-chaos/scenario/routes.go` — IPv4 route generation
- [ ] `cmd/ze-bgp-chaos/scenario/generator.go` — seed-based PeerProfile generation
- [ ] `cmd/ze-bgp-chaos/scenario/config.go` — Ze config file generation
- [ ] `cmd/ze-bgp-chaos/peer/sender.go` — route UPDATE building and sending
- [ ] `cmd/ze-bgp-chaos/peer/receiver.go` — incoming UPDATE parsing
- [ ] `cmd/ze-bgp-chaos/validation/model.go` — expected state model
- [ ] `cmd/ze-bgp-chaos/peer/simulator.go` — chaos event execution in `executeChaos()` + helpers

**Behavior to preserve:**
- IPv4/unicast route generation from Phase 1
- All chaos and validation from Phase 2-3

**Behavior to change:**
- Extend route generation to 6 additional families
- Extend validation model with family awareness

## Data Flow (MANDATORY)

### Entry Point
- Scenario generator assigns family subsets to each peer (seed-based)
- Route generator creates routes per family per peer

### Transformation Path
1. Profile generation includes family assignment (weighted selection)
2. Route generator dispatches to family-specific generators
3. UPDATE builder uses correct encoding per family (inline NLRI vs MP_REACH)
4. Receiver parses MP_REACH/MP_UNREACH for non-v4 families
5. Validation model keys on family + prefix (not just prefix)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Tool ↔ Ze Engine | TCP BGP wire bytes (multi-family UPDATEs) | [ ] |

### Integration Points
- Ze's NLRI builder packages (via registry or direct import)
- Phase 1 UpdateBuilder (for unicast) + lower-level APIs for VPN/EVPN/FlowSpec

### Architectural Verification
- [ ] Each family's NLRI correctly encoded
- [ ] MP_REACH_NLRI used for non-ipv4-unicast
- [ ] EOR sent per family

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Peer with ipv6/unicast | Sends IPv6 UPDATEs via MP_REACH_NLRI |
| AC-2 | Peer with ipv4/vpn | Sends VPN UPDATEs with RD + prefix |
| AC-3 | Peer with l2vpn/evpn | Sends EVPN Type-2 MAC/IP routes |
| AC-4 | Peer with ipv4/flow | Sends FlowSpec rules |
| AC-5 | Peer A (v4+v6), Peer B (v4 only) | B receives only v4 routes from A |
| AC-6 | `--families ipv4/unicast,ipv6/unicast` | Only unicast families |
| AC-7 | `--exclude-families l2vpn/evpn` | EVPN excluded |
| AC-8 | Multi-family peer | EOR sent per family |
| AC-9 | Validation across families | Each family validated independently |
| AC-10 | Chaos disconnect of multi-family peer | Withdrawals for all families |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRouteGenIPv6Unique` | `scenario/routes_ipv6_test.go` | IPv6 /48 prefixes unique per peer | |
| `TestRouteGenIPv6Deterministic` | `scenario/routes_ipv6_test.go` | Same seed → same routes | |
| `TestRouteGenVPNv4` | `scenario/routes_vpn_test.go` | VPN routes have correct RD | |
| `TestRouteGenVPNv6` | `scenario/routes_vpn_test.go` | VPN IPv6 routes correct | |
| `TestRouteGenEVPN` | `scenario/routes_evpn_test.go` | EVPN Type-2 with unique MACs | |
| `TestRouteGenFlowSpecV4` | `scenario/routes_flowspec_test.go` | Valid FlowSpec rules | |
| `TestRouteGenFlowSpecV6` | `scenario/routes_flowspec_test.go` | Valid IPv6 FlowSpec rules | |
| `TestFamilyAssignment` | `scenario/generator_test.go` | Per-peer family sets from seed | |
| `TestFamilyFilterInclude` | `scenario/generator_test.go` | --families limits families | |
| `TestFamilyFilterExclude` | `scenario/generator_test.go` | --exclude-families removes families | |

_Additional tests to be identified after Phase 3._

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| EVPN MAC bytes | 6 bytes | valid MAC | N/A | N/A |
| IPv6 prefix len | 0-128 | 128 | N/A | 129 |
| VPN RD | 8 bytes | valid RD | N/A | N/A |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `chaos-multi-family` | `test/chaos/multi-family.ci` | 3 peers with different families, verify filtering | |
| `chaos-ipv6` | `test/chaos/ipv6.ci` | IPv6 unicast propagation test | |

## Files to Create

- `cmd/ze-bgp-chaos/scenario/routes_ipv6.go`
- `cmd/ze-bgp-chaos/scenario/routes_ipv6_test.go`
- `cmd/ze-bgp-chaos/scenario/routes_vpn.go`
- `cmd/ze-bgp-chaos/scenario/routes_vpn_test.go`
- `cmd/ze-bgp-chaos/scenario/routes_evpn.go`
- `cmd/ze-bgp-chaos/scenario/routes_evpn_test.go`
- `cmd/ze-bgp-chaos/scenario/routes_flowspec.go`
- `cmd/ze-bgp-chaos/scenario/routes_flowspec_test.go`

## Files to Modify

- `cmd/ze-bgp-chaos/scenario/generator.go` - family assignment logic
- `cmd/ze-bgp-chaos/scenario/config.go` - family blocks in Ze config
- `cmd/ze-bgp-chaos/peer/sender.go` - multi-family UPDATE building
- `cmd/ze-bgp-chaos/peer/receiver.go` - multi-family UPDATE parsing
- `cmd/ze-bgp-chaos/validation/model.go` - family-aware expected state
- `cmd/ze-bgp-chaos/main.go` - --families / --exclude-families processing

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | N/A |
| Makefile | No | Already added in Phase 1 |

## Implementation Steps

1. **Read Phase 1-3 learnings** - understand route generation and validation APIs
   → Review: How are routes structured? How does validation key on routes?

2. **Write IPv6 route generation tests**
   → Run: Tests FAIL

3. **Implement IPv6 route generator**
   → Run: Tests PASS

4. **Write VPN route generation tests**
   → Run: Tests FAIL

5. **Implement VPN route generator**
   → Run: Tests PASS

6. **Write EVPN route generation tests**
   → Run: Tests FAIL

7. **Implement EVPN route generator**
   → Run: Tests PASS

8. **Write FlowSpec route generation tests**
   → Run: Tests FAIL

9. **Implement FlowSpec route generator**
   → Run: Tests PASS

10. **Implement family assignment** in scenario generator

11. **Extend sender** for multi-family UPDATEs

12. **Extend receiver** for MP_REACH/MP_UNREACH parsing

13. **Extend validation** with family-aware model

14. **Implement `--families` / `--exclude-families`**

15. **Verify** - `make lint && make test`

16. **Update follow-on specs** (Spec Propagation Task)

## Spec Propagation Task

**MANDATORY at end of this phase:**

Before marking this spec complete, update the following spec:

1. **`spec-bgp-chaos-reporting.md`** — Update with:
   - Per-family stats available
   - Family-specific validation results
   - Any family-specific chaos interactions discovered

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

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| IPv6/unicast route generation | | | |
| VPN route generation | | | |
| EVPN route generation | | | |
| FlowSpec route generation | | | |
| Per-peer family assignment | | | |
| Family-aware validation | | | |
| --families filter | | | |
| --exclude-families filter | | | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | | | |
| AC-2 | | | |
| AC-3 | | | |
| AC-4 | | | |
| AC-5 | | | |
| AC-6 | | | |
| AC-7 | | | |
| AC-8 | | | |
| AC-9 | | | |
| AC-10 | | | |

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

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 demonstrated
- [ ] Tests pass (`make test`)
- [ ] No regressions (`make functional`)

### Quality Gates (SHOULD pass)
- [ ] `make lint` passes
- [ ] Follow-on spec updated (Spec Propagation Task)
- [ ] Implementation Audit completed

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Implementation complete
- [ ] Tests PASS
- [ ] Boundary tests for numeric inputs

### Completion
- [ ] Spec Propagation Task completed
- [ ] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-bgp-chaos-families.md`
