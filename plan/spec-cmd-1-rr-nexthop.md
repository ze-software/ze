# Spec: cmd-1 -- Route Reflection and Next-Hop Control

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 4/4 |
| Updated | 2026-04-14 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/spec-cmd-0-umbrella.md` -- umbrella context
3. `internal/component/bgp/schema/ze-bgp-conf.yang` -- peer-fields grouping
4. `internal/component/bgp/reactor/reactor_api_forward.go` -- UPDATE forwarding
5. `rfc/short/rfc4456.md` -- Route Reflection

## Task

Add route-reflector-client, cluster-id, and next-hop control (self/unchanged/auto/IP) to
Ze's BGP peer configuration. These are the two most critical missing features for production
iBGP deployments. Every vendor (Junos, EOS, IOS-XR, VyOS) has them.

**Config syntax (editor):**

| Command | Purpose |
|---------|---------|
| `set bgp group IBGP session route-reflector-client` | Mark peers in group as RR clients |
| `set bgp group IBGP session cluster-id 1.1.1.1` | Override cluster-id (default: router-id) |
| `set bgp peer 10.0.0.1 session next-hop self` | Rewrite next-hop to local address |
| `set bgp peer 10.0.0.1 session next-hop unchanged` | Never rewrite next-hop |
| `set bgp peer 10.0.0.1 session next-hop 192.168.1.1` | Set explicit next-hop IP |
| `set bgp peer 10.0.0.1 session next-hop auto` | RFC default: rewrite for eBGP, preserve for iBGP |

**YANG location:** `session` container in `peer-fields` grouping.

| Leaf | Type | Default | RFC |
|------|------|---------|-----|
| `route-reflector-client` | boolean | false | RFC 4456 |
| `cluster-id` | ipv4-address | (router-id) | RFC 4456 Section 7 |
| `next-hop` | union: ip-address, enum {self, unchanged, auto} | auto | RFC 4271 Section 5.1.3 |

**Route reflection rules (RFC 4456):**
- Route from client -> forward to all clients and non-clients (add ORIGINATOR_ID if missing, prepend own cluster-id to CLUSTER_LIST)
- Route from non-client -> forward to clients only
- Route from eBGP -> forward to all clients and non-clients
- Drop if own cluster-id found in CLUSTER_LIST (loop)
- Drop if own router-id is ORIGINATOR_ID

**Next-hop rewriting rules:**
- `auto`: rewrite to local address for eBGP, preserve for iBGP (RFC 4271 default)
- `self`: always rewrite to local address (common for iBGP with RR)
- `unchanged`: never rewrite (route-server, third-party next-hop)
- explicit IP: set next-hop to specified address

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` -- reactor event loop, UPDATE forwarding
  -> Constraint: forwarding decisions are in reactor, not plugins
- [ ] `.claude/patterns/config-option.md` -- YANG leaf + resolver + reactor wiring
  -> Constraint: follow the pattern for adding config knobs

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4456.md` -- Route Reflection
  -> Constraint: ORIGINATOR_ID and CLUSTER_LIST attribute handling rules
- [ ] `rfc/short/rfc4271.md` -- BGP-4 base
  -> Constraint: next-hop rewriting rules (Section 5.1.3)

**Key insights:**
- Route reflection changes the forwarding decision matrix (who gets what)
- ORIGINATOR_ID and CLUSTER_LIST must be added/checked during forwarding
- Next-hop rewriting happens at egress time (per-destination-peer decision)
- cluster-id defaults to router-id if not configured

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/schema/ze-bgp-conf.yang` -- peer-fields grouping, session container
- [ ] `internal/component/bgp/config/peers.go` -- PeersFromTree() extracts peer config
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go` -- forwardUpdate(), dispatchToMatchingPeers()
- [ ] `internal/component/bgp/reactor/forward_build.go` -- buildModifiedPayload()
- [ ] `internal/component/bgp/reactor/peer.go` -- Peer struct, session state

**Behavior to preserve:**
- Existing eBGP forwarding rules (AS-path loop, next-hop rewrite)
- Existing filter chain execution order (in-process filters, then plugin filters)
- Wire-level UPDATE building via buffer pools
- All existing config files parse and work identically (no RR = no behavior change)

**Behavior to change:**
- New YANG leaves in session container: route-reflector-client, cluster-id, next-hop
- Forwarding decision matrix includes RR client/non-client distinction
- Next-hop rewriting controlled by per-peer config instead of implicit iBGP/eBGP rule
- ORIGINATOR_ID and CLUSTER_LIST attributes added during RR forwarding

## Data Flow (MANDATORY)

### Entry Point
- Config: `session { route-reflector-client; cluster-id 1.1.1.1; next-hop self; }` parsed from YANG
- Wire: UPDATE received from peer, forwarding decision made per destination peer

### Transformation Path
1. Config parse: YANG leaves extracted by `ResolveBGPTree()`
2. Peer creation: `PeersFromTree()` sets RR client flag, cluster-id, next-hop mode on peer struct
3. UPDATE receive: wire bytes parsed, attributes available via iterators
4. Forwarding decision: check source peer type (client/non-client/eBGP) vs destination peer type
5. Attribute modification: add ORIGINATOR_ID if missing, prepend cluster-id to CLUSTER_LIST (RR)
6. Next-hop rewrite: apply per-destination-peer next-hop rule (self/unchanged/auto/IP)
7. Wire encoding: modified attributes written via buffer-first pattern

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Config -> Reactor | PeersFromTree() extracts RR/next-hop config into peer struct | [ ] |
| Reactor -> Wire | buildModifiedPayload() applies attribute modifications | [ ] |

### Integration Points
- `PeersFromTree()` -- add extraction for new YANG leaves
- `dispatchToMatchingPeers()` -- add RR forwarding rules
- `buildModifiedPayload()` -- add ORIGINATOR_ID, CLUSTER_LIST, next-hop rewriting

### Architectural Verification
- [ ] No bypassed layers (config -> resolver -> reactor -> wire)
- [ ] No unintended coupling (RR logic in reactor, not in RIB or plugins)
- [ ] No duplicated functionality (next-hop rewriting extends existing path)
- [ ] Zero-copy preserved (ORIGINATOR_ID/CLUSTER_LIST added via buffer-first encoding)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config with `session route-reflector-client` | -> | Reactor forwards client route to other clients | `test/plugin/rr-basic.ci` |
| Config with `session next-hop self` | -> | Outbound UPDATE has next-hop rewritten | `test/plugin/nexthop-self.ci` |
| Config with `session next-hop unchanged` | -> | Outbound UPDATE preserves original next-hop | `test/plugin/nexthop-unchanged.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Route from RR client peer | Forwarded to all other clients and all non-clients with ORIGINATOR_ID and CLUSTER_LIST |
| AC-2 | Route from non-client iBGP peer | Forwarded to clients only (not to other non-clients) |
| AC-3 | Route from eBGP peer | Forwarded to all clients and all non-clients |
| AC-4 | Route with own cluster-id in CLUSTER_LIST | Dropped (loop detection) |
| AC-5 | Route with own router-id as ORIGINATOR_ID | Dropped (loop detection) |
| AC-6 | No cluster-id configured | Router-id used as cluster-id |
| AC-7 | `next-hop self` configured | All outbound UPDATEs to this peer have next-hop = local address |
| AC-8 | `next-hop unchanged` configured | All outbound UPDATEs to this peer preserve original next-hop |
| AC-9 | `next-hop auto` (default) with eBGP peer | Next-hop rewritten to local address |
| AC-10 | `next-hop auto` (default) with iBGP peer | Next-hop preserved (unchanged) |
| AC-11 | `next-hop 192.168.1.1` configured | All outbound UPDATEs to this peer have next-hop = 192.168.1.1 |
| AC-12 | No RR config (existing deployments) | Behavior identical to current Ze (no RR, default next-hop rules) |
| AC-13 | `peer detail` for RR client | Shows route-reflector-client: true and cluster-id |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRRForwardClientToClient` | `reactor_forward_test.go` | Client route forwarded to other clients | |
| `TestRRForwardClientToNonClient` | `reactor_forward_test.go` | Client route forwarded to non-clients | |
| `TestRRForwardNonClientToClient` | `reactor_forward_test.go` | Non-client route forwarded to clients only | |
| `TestRRForwardNonClientToNonClient` | `reactor_forward_test.go` | Non-client route NOT forwarded to other non-clients | |
| `TestRRClusterListLoop` | `reactor_forward_test.go` | Route with own cluster-id dropped | |
| `TestRROriginatorIDLoop` | `reactor_forward_test.go` | Route with own router-id as originator-id dropped | |
| `TestNextHopSelf` | `reactor_forward_test.go` | Next-hop rewritten to local address | |
| `TestNextHopUnchanged` | `reactor_forward_test.go` | Next-hop preserved | |
| `TestNextHopAutoEBGP` | `reactor_forward_test.go` | eBGP next-hop rewritten | |
| `TestNextHopAutoIBGP` | `reactor_forward_test.go` | iBGP next-hop preserved | |
| `TestNextHopExplicitIP` | `reactor_forward_test.go` | Next-hop set to explicit IP | |
| `TestClusterIDDefault` | `config_test.go` | cluster-id defaults to router-id | |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| cluster-id | valid IPv4 | 255.255.255.255 | N/A (string) | N/A |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rr-basic` | `test/plugin/rr-basic.ci` | iBGP RR: client route forwarded to second client | |
| `nexthop-self` | `test/plugin/nexthop-self.ci` | UPDATE sent to peer has next-hop rewritten | |
| `nexthop-unchanged` | `test/plugin/nexthop-unchanged.ci` | UPDATE sent to peer preserves original next-hop | |

## Files to Modify

- `internal/component/bgp/schema/ze-bgp-conf.yang` -- add route-reflector-client, cluster-id, next-hop leaves
- `internal/component/bgp/config/peers.go` -- extract new leaves in PeersFromTree()
- `internal/component/bgp/reactor/peer.go` -- add RR client flag, cluster-id, next-hop mode to Peer struct
- `internal/component/bgp/reactor/reactor_api_forward.go` -- RR forwarding rules, next-hop rewriting
- `internal/component/bgp/reactor/forward_build.go` -- ORIGINATOR_ID/CLUSTER_LIST attribute building

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new leaves) | [x] | `internal/component/bgp/schema/ze-bgp-conf.yang` |
| CLI commands/flags | [ ] | YANG-driven (automatic) |
| Editor autocomplete | [ ] | YANG-driven (automatic) |
| Functional test for new feature | [x] | `test/plugin/rr-basic.ci`, `test/plugin/nexthop-self.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` -- add route reflection |
| 2 | Config syntax changed? | [x] | `docs/guide/configuration.md` -- RR and next-hop config examples |
| 3 | CLI command added/changed? | [ ] | N/A (config only) |
| 4 | API/RPC added/changed? | [ ] | N/A |
| 5 | Plugin added/changed? | [ ] | N/A |
| 6 | Has a user guide page? | [x] | `docs/guide/route-reflection.md` -- update with config syntax |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [x] | `rfc/short/rfc4456.md` |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [x] | `docs/comparison.md` -- route reflection now supported |
| 12 | Internal architecture changed? | [ ] | N/A |

## Files to Create

- `test/plugin/rr-basic.ci` -- RR client forwarding functional test
- `test/plugin/nexthop-self.ci` -- next-hop self functional test
- `test/plugin/nexthop-unchanged.ci` -- next-hop unchanged functional test

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + umbrella |
| 2. Audit | Files to Modify, TDD Plan |
| 3. Implement (TDD) | Phases below |
| 4. Full verification | `make ze-verify` |
| 5-12. | Standard flow |

### Implementation Phases

1. **Phase: YANG + Config** -- Add leaves to ze-bgp-conf.yang, extract in PeersFromTree()
   - Tests: `TestClusterIDDefault`
   - Files: ze-bgp-conf.yang, peers.go
2. **Phase: RR Forwarding** -- Implement RFC 4456 forwarding rules in reactor
   - Tests: `TestRRForward*`, `TestRRClusterListLoop`, `TestRROriginatorIDLoop`
   - Files: reactor_api_forward.go, forward_build.go
3. **Phase: Next-Hop Control** -- Implement next-hop rewriting per-peer
   - Tests: `TestNextHop*`
   - Files: reactor_api_forward.go, forward_build.go
4. **Functional tests** -- .ci tests proving end-to-end behavior
5. **Full verification** -- `make ze-verify`

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All 13 ACs demonstrated |
| RFC compliance | ORIGINATOR_ID added only once, CLUSTER_LIST prepended not replaced |
| Backward compat | No RR config = identical behavior to current Ze |
| Wire correctness | ORIGINATOR_ID (type 9), CLUSTER_LIST (type 10) encoded correctly |
| Inheritance | RR client flag inherits from group to peer |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| YANG leaves in ze-bgp-conf.yang | `grep route-reflector-client internal/component/bgp/schema/ze-bgp-conf.yang` |
| RR forwarding in reactor | `grep -r route-reflector internal/component/bgp/reactor/` |
| .ci functional tests | `ls test/plugin/rr-basic.ci test/plugin/nexthop-self.ci` |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | cluster-id must be valid IPv4; next-hop IP must be valid |
| Loop prevention | CLUSTER_LIST and ORIGINATOR_ID checks must not be bypassable |

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

## RFC Documentation

Add `// RFC 4456 Section N: "<requirement>"` above RR forwarding and loop detection code.
Add `// RFC 4271 Section 5.1.3: "<requirement>"` above next-hop rewriting code.

## Implementation Summary

### What Was Implemented

**Phase 1 (YANG + Config) -- DONE:**
- YANG leaves: `session/route-reflector-client` (boolean), `session/cluster-id` (ipv4-address), `session/next-hop` (union: self/unchanged/auto/IP)
- Config extraction in `parsePeerFromTree()` for all three fields
- `PeerSettings`: `RouteReflectorClient`, `ClusterID`, `NextHopMode` (4 constants), `NextHopAddress`, `EffectiveClusterID()`
- 16 unit test subtests in `config_test.go`
- Parse test: `test/parse/rr-nexthop-config.ci`

**Phase 2 (RR Forwarding) -- DONE:**
- Forwarding rules in `ForwardUpdate`: client->all, non-client->clients-only, eBGP->all
- `remoteRouterID` atomic on Peer (set in `validateOpen`, cleared in `clearEncodingContexts`)
- ORIGINATOR_ID handler (`originatorIDHandler`): set-if-absent using source peer's BGP Identifier
- CLUSTER_LIST handler (`clusterListHandler`): prepend own cluster-id
- Handlers registered in `attrModHandlersWithDefaults()`
- 6 handler test subtests

**Phase 3 (Next-Hop Control) -- DONE (IPv4 + IPv6):**
- `applyNextHopMod()`: per-destination-peer NEXT_HOP (type 3) modification via ModAccumulator
- IPv4 safety: `Is4()` check before `As4()` to prevent zero-value on IPv6 addresses
- Auto/self/unchanged/explicit modes all functional
- IPv6: `mpReachNextHopHandler()` in `filter_delta_handlers.go` rewrites MP_REACH_NLRI (type 14) NH field
- `applyNextHopMod` emits MP_REACH ops for IPv6 addresses

**Phase 3b (Cluster-ID Sync) -- DONE:**
- `PeersFromConfigTree` syncs `session/cluster-id` and `loop-detection/cluster-id`
- Whichever is set propagates to the other (both directions)
- 2 unit test subtests

**Phase 4 (peer detail output) -- DONE:**
- `HandleBgpPeerDetail` shows `route-reflector-client`, `cluster-id`, `next-hop` mode
- `PeerInfo` struct extended with new fields
- `Peers()` API populates new fields from PeerSettings

### What Remains

~~IPv6 next-hop rewriting (MP_REACH_NLRI type 14)~~ -- DONE. `mpReachNextHopHandler()` implements approach 1 (type-14 AttrModHandler). Found already implemented during 2026-04-14 audit.

| Item | Effort | Design needed |
|------|--------|---------------|
| Wire-level forwarding .ci tests (rr-basic, nexthop-self, nexthop-unchanged) | Medium | No -- blocked by bgp-rr replay timing in single-ze-peer multi-IP pattern. Config acceptance + RIB storage tests exist. |

### Bugs Found/Fixed
- ORIGINATOR_ID initially used source peer IP instead of BGP Identifier (review finding 1 -- fixed)
- Redundant EffectiveClusterID fallback removed (review finding 2 -- fixed)
- IPv6 As4() panic prevented with Is4() guard (review finding 5 -- fixed)
- Unrelated export filter text delta change reverted (review finding 10 -- fixed)

### Documentation Updates
- `docs/guide/command-reference.md` not yet updated (deferred to spec completion)

### Deviations from Plan
- ~~IPv6 next-hop rewriting deferred to separate phase~~ -- found already implemented (2026-04-14 audit)
- Wire-level forwarding .ci tests use config-acceptance + RIB-storage pattern instead of two-peer hex verification (bgp-rr replay timing blocker)

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
- [ ] AC-1..AC-13 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-cmd-1-rr-nexthop.md`
- [ ] Summary included in commit
