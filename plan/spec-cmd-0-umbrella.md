# Spec: cmd-0 -- Vendor Parity Commands (Umbrella)

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 4/9 |
| Updated | 2026-04-10 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. Child specs: `spec-cmd-1-*` through `spec-cmd-9-*`
4. `internal/component/bgp/schema/ze-bgp-conf.yang` -- peer-fields grouping
5. `internal/component/bgp/plugins/filter_community/` -- existing filter plugin pattern
6. `internal/component/bgp/reactor/filter/` -- existing loop-detection filter
7. `internal/component/bgp/plugins/rib/rib_pipeline.go` -- RIB pipeline pattern

## Task

Gap analysis comparing Ze's CLI commands against Junos, Arista EOS, Cisco IOS-XR, and VyOS
identified missing commands needed for production BGP deployments. This umbrella organizes
the work into child specs by component boundary.

### Methodology

Vendor commands were audited across all four NOS platforms. Gaps were filtered against Ze's
existing capabilities (some apparent gaps, like per-neighbor route views, already exist via
`rib routes received peer <sel>` with pipeline filters). The remaining gaps were designed
following Ze's patterns: YANG-modeled config, named filter plugins, pipeline-composable
operational commands.

### Design Decisions

| Decision | Detail |
|----------|--------|
| Config knobs in peer-fields | RR, next-hop, send-community, default-originate, local-as modifiers, as-override -- all YANG leaves in the existing `peer-fields` grouping, inheritable at bgp/group/peer levels |
| Filter plugins, not route-maps | Prefix-list, AS-path, community-match, route-modify are separate `ze:filter` plugins under `bgp/policy`. Composable in filter chains: `filter import prefix-list:X as-path-list:Y modify:Z`. Each does one thing. |
| next-hop as single union leaf | `self`, `unchanged`, `auto`, or explicit IP. One leaf, not three booleans. |
| send-community default all | Matches Junos (send everything unless restricted). Leaf-list of types to send. |
| Multipath in RIB plugin | `maximum-paths` + `relax-as-path` extend the existing best-path selection. Global config, not per-peer. |
| Ping/traceroute under resolve | Active probes alongside DNS/Cymru/PeeringDB/IRR -- all network resolution tools. |
| show policy for introspection | Ze's introspection philosophy: everything inspectable at runtime. Policy dry-run is unique to Ze. |

### Existing Capabilities (NOT gaps)

These were initially flagged as gaps but already exist:

| Feature | Ze command |
|---------|-----------|
| Per-neighbor received routes | `rib routes received peer <sel>` |
| Per-neighbor advertised routes | `rib routes sent peer <sel>` |
| Filter by prefix | `rib routes cidr <prefix>` |
| Filter by community | `rib routes community <value>` |
| Filter by AS-path pattern | `rib routes path <pattern>` |
| Filter by family | `rib routes family <afi/safi>` |
| Clear soft inbound | `peer <sel> clear soft` (sends ROUTE-REFRESH) |
| Clear Adj-RIB-Out | `rib clear out <sel>` |
| AS-path loop detection | `bgp policy loop-detection` filter |
| Allow-own-AS | `loop-detection allow-own-as N` |

### Scope

**In scope:** All child specs below.

**Out of scope (future work):**

| Feature | Reason | Destination |
|---------|--------|-------------|
| BFD | OS/kernel feature, not BGP daemon. Platform-specific. | spec-bfd (future) |
| Aggregate-address | Operators use static `update` blocks today. Lower priority. | spec-aggregate (future) |
| Route dampening | Falling out of industry favor. | spec-dampening (future) |
| Confederation | Rare -- RR covers 99% of use cases. | spec-confederation (future) |
| BMP | Monitoring protocol, not blocking. | spec-bmp (future) |

### Child Specs

| Phase | Spec | Component | Scope | Depends |
|-------|------|-----------|-------|---------|
| 1 | `spec-cmd-1-rr-nexthop.md` | BGP session config | Route-reflector-client, cluster-id, next-hop control (self/unchanged/auto/IP) | - |
| 2 | `spec-cmd-2-session-policy.md` | BGP session config | Send-community control, default-originate, local-as modifiers, as-override | - |
| 3 | `spec-cmd-3-multipath.md` | BGP config + RIB plugin | maximum-paths, relax-as-path for ECMP | - |
| 4 | `spec-cmd-4-prefix-filter.md` | Filter plugin | `bgp-filter-prefix`: named prefix-lists under bgp/policy | policy framework |
| 5 | `spec-cmd-5-aspath-filter.md` | Filter plugin | `bgp-filter-aspath`: named AS-path regex lists under bgp/policy | policy framework |
| 6 | `spec-cmd-6-community-match.md` | Filter plugin | Extend `bgp-filter-community` with match-and-act | policy framework |
| 7 | `spec-cmd-7-route-modify.md` | Filter plugin | `bgp-filter-modify`: set local-preference, MED, origin, next-hop, AS-prepend | policy framework, spec-apply-mods |
| 8 | `spec-cmd-8-policy-show.md` | Operational commands | `show policy list/detail/chain/test` -- introspection and dry-run | filters exist |
| 9 | `spec-cmd-9-ops.md` | Operational commands | Best-path reason, ping/traceroute, interface counters/brief, show uptime | - |

### Execution Order

Phases 1-3 have no dependencies and can be implemented in any order or in parallel.

Phases 4-7 depend on the policy framework (filter plugin infrastructure). The framework
already exists (loop-detection and community filters use it), but these add new filter types.
Phase 7 also depends on spec-apply-mods for wire-level attribute rewriting.

Phase 8 depends on at least one filter type existing (phases 4-6).

Phase 9 has no dependencies and can be implemented at any time.

### Vendor Parity After Completion

| Feature | Junos | EOS | IOS-XR | VyOS | Ze (after) |
|---------|-------|-----|--------|------|------------|
| Route reflection | Y | Y | Y | Y | Y |
| Next-hop control | Y | Y | Y | Y | Y |
| Prefix-list filtering | Y | Y | Y | Y | Y |
| AS-path filtering | Y | Y | Y | Y | Y |
| Community matching | Y | Y | Y | Y | Y |
| Route attribute modification | Y | Y | Y | Y | Y |
| Send-community control | Y | Y | Y | Y | Y |
| Default-originate | Y | Y | Y | Y | Y |
| Multipath/ECMP | Y | Y | Y | Y | Y |
| Local-AS modifiers | Y | Y | Y | Y | Y |
| AS-override | Y | Y | Y | Y | Y |
| Policy introspection | partial | partial | Y | partial | Y |
| Policy dry-run testing | -- | -- | partial | -- | Y (unique) |
| Ping/traceroute | Y | Y | Y | Y | Y |
| Interface counters | Y | Y | Y | Y | Y |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` -- overall architecture, reactor, plugin model
  -> Constraint: plugins register via init() + register.go; reactor is the central event loop
- [ ] `.claude/patterns/config-option.md` -- how to add config leaves
  -> Constraint: YANG leaf + resolver + reactor wiring + .ci test
- [ ] `.claude/patterns/plugin.md` -- how to create a filter plugin
  -> Constraint: filter plugins augment bgp/policy, use ze:filter extension

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4271.md` -- BGP-4 base: UPDATE processing, next-hop, AS-path
  -> Constraint: next-hop rewriting rules for iBGP/eBGP (Section 5.1.3)
- [ ] `rfc/short/rfc4456.md` -- Route Reflection
  -> Constraint: RR client/non-client, ORIGINATOR_ID, CLUSTER_LIST, cluster-id
- [ ] `rfc/short/rfc7911.md` -- ADD-PATH (multipath advertisement)
  -> Constraint: multiple paths per prefix, path-id

**Key insights:**
- Route reflection changes UPDATE forwarding rules (client-to-client, non-client-to-client)
- Next-hop self must be applied during UPDATE building, not at receive time
- Filter plugins already have a working pattern (loop-detection, community)
- The RIB pipeline is the right extension point for best-path reason and multipath

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/schema/ze-bgp-conf.yang` -- peer-fields grouping with session/connection/behavior/timer/filter containers
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go` -- UPDATE forwarding with filter chain execution
- [ ] `internal/component/bgp/reactor/filter_chain.go` -- filter chain dispatch to plugins
- [ ] `internal/component/bgp/plugins/filter_community/` -- existing filter plugin (tag/strip pattern)
- [ ] `internal/component/bgp/reactor/filter/` -- in-process loop-detection filter
- [ ] `internal/component/bgp/plugins/rib/rib_pipeline.go` -- RIB show pipeline (source/filter/terminal)
- [ ] `internal/component/bgp/plugins/rib/bestpath.go` -- best-path selection logic

**Behavior to preserve:**
- Existing peer-fields YANG structure and inheritance (bgp > group > peer)
- Existing filter chain dispatch (in-process filters first, then plugin filters)
- Existing RIB pipeline composability (scope + filters + terminal)
- Existing community tag/strip functionality unchanged
- Existing loop-detection filter unchanged
- All existing config files parse and work identically

**Behavior to change:**
- New YANG leaves added to peer-fields for RR, next-hop, send-community, default-originate, local-as modifiers, as-override
- New filter plugin types registered under bgp/policy
- RIB pipeline extended with best-path reason terminal
- New operational commands for policy introspection, ping/traceroute, interface counters

## Data Flow (MANDATORY)

### Entry Point
- Config file: YANG leaves parsed during config load, resolved through `ResolveBGPTree()`
- CLI: `set bgp peer X session route-reflector-client` in editor, `show policy list` as operational command
- Filter chain: UPDATE wire bytes passed through filter plugins during ingress/egress processing

### Transformation Path
1. Config parse: YANG leaves extracted from config tree by `ResolveBGPTree()`
2. Peer creation: config values wired into `PeerFilterInfo` and reactor peer state
3. UPDATE receive: wire bytes pass through ingress filter chain (loop-detection, prefix-list, as-path, community-match)
4. UPDATE forward: reactor applies next-hop rewriting, send-community filtering, then egress filter chain (modify, community tag/strip)
5. RIB storage: accepted routes stored in Adj-RIB-In, best-path computed (with multipath if configured)
6. Operational query: `rib routes`/`show policy`/`resolve ping` dispatched to handlers

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Config -> Reactor | `PeersFromTree()` extracts YANG values into peer config structs | [ ] |
| Reactor -> Filter Plugin | JSON filter-update RPC over MuxConn with wire bytes | [ ] |
| RIB -> CLI | Pipeline iterator yields routes matching filters | [ ] |
| CLI -> OS | ping/traceroute exec OS commands | [ ] |

### Integration Points
- `PeersFromTree()` in `internal/component/bgp/config/peers.go` -- extracts config into peer structs
- `reactor_api_forward.go` -- UPDATE forwarding applies next-hop rewriting and filter chains
- `filter_chain.go` -- dispatches to plugin filter RPCs
- `rib_pipeline.go` -- composable route query pipeline

### Architectural Verification
- [ ] No bypassed layers (config flows through YANG -> resolver -> reactor)
- [ ] No unintended coupling (filter plugins are independent, composable)
- [ ] No duplicated functionality (extends existing filter chain and pipeline patterns)
- [ ] Zero-copy preserved (filter plugins receive wire bytes, not parsed structs)

## Wiring Test (MANDATORY -- NOT deferrable)

Umbrella delegates to child specs. Each child has its own wiring tests.

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config with `session route-reflector-client` | -> | Reactor marks peer as RR client, forwards accordingly | `test/plugin/rr-basic.ci` (spec-cmd-1) |
| Config with `session next-hop self` | -> | Reactor rewrites next-hop on egress | `test/plugin/nexthop-self.ci` (spec-cmd-1) |
| Config with `policy prefix-list` + filter chain | -> | Prefix filter plugin rejects non-matching prefixes | `test/plugin/prefix-filter.ci` (spec-cmd-4) |
| `show policy list` CLI command | -> | Policy introspection handler returns registered filters | `test/plugin/policy-show.ci` (spec-cmd-8) |
| `resolve ping 127.0.0.1` | -> | Ping handler executes ICMP probe | `test/plugin/resolve-ping.ci` (spec-cmd-9) |

## Acceptance Criteria

Umbrella ACs are high-level. Child specs define detailed per-feature ACs.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | iBGP peer with `route-reflector-client` | Routes from RR clients forwarded to other RR clients and non-clients per RFC 4456 |
| AC-2 | eBGP peer with `next-hop self` | All UPDATEs sent to this peer have next-hop rewritten to local address |
| AC-3 | Peer with `community send standard large` | Only standard and large communities included in outbound UPDATEs |
| AC-4 | Peer with `default-originate` per family | Default route (0.0.0.0/0 or ::/0) originated to peer for that family |
| AC-5 | `multipath maximum-paths 4` | RIB selects up to 4 equal-cost paths per prefix |
| AC-6 | Prefix-list filter in import chain | UPDATEs with non-matching prefixes rejected |
| AC-7 | AS-path filter in import chain | UPDATEs with non-matching AS-paths rejected |
| AC-8 | Community-match filter in import chain | UPDATEs with matching community rejected or accepted per filter action |
| AC-9 | Modify filter in export chain | Outbound UPDATEs have attributes modified per filter config |
| AC-10 | `show policy list` | All registered filter types and instances listed |
| AC-11 | `show policy test peer X import prefix 10.0.0.0/24` | Dry-run result shows accept/reject and which filter decided |
| AC-12 | `resolve ping 10.0.0.1` | ICMP probe executed, RTT and status returned |
| AC-13 | `show interface eth0 counters` | RX/TX packets, bytes, errors, drops shown |
| AC-14 | `rib best 10.0.0.0/24 reason` | Best-path decision steps shown with winner/loser reasoning |

## 🧪 TDD Test Plan

### Unit Tests

Umbrella delegates to child specs. Summary of test areas:

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestReflectorForwarding` | spec-cmd-1 | RR client-to-client and client-to-non-client forwarding | |
| `TestNextHopRewrite` | spec-cmd-1 | next-hop self/unchanged/auto/IP rewriting on egress | |
| `TestSendCommunityFilter` | spec-cmd-2 | community type filtering on outbound UPDATEs | |
| `TestDefaultOriginate` | spec-cmd-2 | default route generation and conditional origination | |
| `TestMultipathSelection` | spec-cmd-3 | N best-paths selected when maximum-paths > 1 | |
| `TestPrefixListMatch` | spec-cmd-4 | prefix matching with ge/le/exact | |
| `TestAsPathRegexMatch` | spec-cmd-5 | AS-path regex filter accept/reject | |
| `TestCommunityMatchAction` | spec-cmd-6 | community match with accept/reject action | |
| `TestRouteModify` | spec-cmd-7 | attribute modification in filter chain | |
| `TestPolicyIntrospection` | spec-cmd-8 | show policy list/detail/chain/test | |
| `TestBestPathReason` | spec-cmd-9 | decision step reporting in rib best | |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rr-basic.ci` | `test/plugin/rr-basic.ci` | Config with RR client, verify route forwarding | |
| `nexthop-self.ci` | `test/plugin/nexthop-self.ci` | Config with next-hop self, verify wire output | |
| `prefix-filter.ci` | `test/plugin/prefix-filter.ci` | Config with prefix-list filter, verify rejection | |
| `policy-show.ci` | `test/plugin/policy-show.ci` | Show policy list returns registered filters | |
| `resolve-ping.ci` | `test/plugin/resolve-ping.ci` | Ping loopback returns success | |

## Files to Modify

Umbrella delegates to child specs. Key files across all phases:

- `internal/component/bgp/schema/ze-bgp-conf.yang` -- YANG leaf additions (phases 1-2)
- `internal/component/bgp/config/peers.go` -- config resolution for new leaves (phases 1-2)
- `internal/component/bgp/reactor/reactor_api_forward.go` -- next-hop rewriting, send-community (phases 1-2)
- `internal/component/bgp/plugins/rib/bestpath.go` -- multipath selection (phase 3)
- `internal/component/bgp/plugins/filter_prefix/` -- new plugin (phase 4)
- `internal/component/bgp/plugins/filter_aspath/` -- new plugin (phase 5)
- `internal/component/bgp/plugins/filter_community/` -- extend with match-and-act (phase 6)
- `internal/component/bgp/plugins/filter_modify/` -- new plugin (phase 7)
- `internal/component/cmd/show/` -- policy introspection commands (phase 8)
- `internal/component/resolve/cmd/` -- ping/traceroute (phase 9)
- `internal/component/cmd/show/` -- interface counters, uptime (phase 9)

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This umbrella + active child spec |
| 2. Audit | Child spec's Files to Modify and TDD Plan |
| 3. Implement (TDD) | Child spec's implementation phases |
| 4. Full verification | `make ze-verify` |
| 5. Critical review | Child spec's critical review checklist |
| 6-12. | Per child spec |

### Implementation Phases

Each child spec is one phase. Phases 1-3 and 9 are independent. Phases 4-8 have dependencies.

1. **Phase: RR + Next-Hop** (spec-cmd-1) -- YANG leaves, reactor forwarding rules, next-hop rewriting
2. **Phase: Session Policy** (spec-cmd-2) -- send-community, default-originate, local-as, as-override
3. **Phase: Multipath** (spec-cmd-3) -- maximum-paths, relax-as-path in RIB
4. **Phase: Prefix Filter** (spec-cmd-4) -- bgp-filter-prefix plugin
5. **Phase: AS-Path Filter** (spec-cmd-5) -- bgp-filter-aspath plugin
6. **Phase: Community Match** (spec-cmd-6) -- extend community plugin
7. **Phase: Route Modify** (spec-cmd-7) -- bgp-filter-modify plugin
8. **Phase: Policy Show** (spec-cmd-8) -- policy introspection commands
9. **Phase: Operational** (spec-cmd-9) -- best-path reason, ping/traceroute, interface counters, uptime

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every child spec's ACs demonstrated before closing that child |
| Vendor parity | Cross-check each feature against Junos/EOS/IOS-XR/VyOS behavior |
| Config inheritance | New YANG leaves inherit correctly at bgp > group > peer levels |
| Filter composability | New filters work in chains with existing filters |
| Wire correctness | Next-hop rewriting, community filtering produce correct BGP wire bytes |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| All child specs have learned summaries | `ls plan/learned/*cmd*` |
| All .ci functional tests pass | `make ze-functional-test` |
| docs/guide/command-reference.md updated | `grep 'route-reflector\|next-hop self\|prefix-list' docs/guide/command-reference.md` |
| docs/comparison.md updated | `grep 'route reflection\|prefix filter' docs/comparison.md` |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | Prefix-list entries: validate CIDR, ge/le ranges. AS-path regex: limit complexity to prevent ReDoS. |
| Resource exhaustion | Limit prefix-list size, AS-path regex count. Ping/traceroute: rate-limit, timeout. |
| Privilege | Ping/traceroute may need raw socket or setuid. Validate source address is local. |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
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

- `rib routes` already covers per-neighbor route views and filtering -- many vendor "gaps" are not gaps
- Ze's composable filter chain (prefix-list:X as-path-list:Y modify:Z) is more explicit than monolithic route-maps
- Ze's `show policy test` dry-run is unique -- no vendor has built-in hypothetical route testing
- Route reflection and next-hop rewriting interact: RR clients often need next-hop-unchanged

## Implementation Summary (2026-04-10 session)

### Child Spec Status

| Spec | Phase | Done | Remaining |
|------|-------|------|-----------|
| cmd-1 RR + Next-Hop | 3/4 | YANG, config, RR forwarding, ORIGINATOR_ID/CLUSTER_LIST handlers, IPv4 next-hop, cluster-id sync, peer detail | IPv6 next-hop (MP_REACH type 14) |
| cmd-2 Session Policy | 3/4 | YANG, config, send-community stripping, AS-override, default-originate (unconditional) | Local-AS OPEN modifiers, default-originate-filter conditional |
| cmd-3 Multipath | 1/3 | YANG schema + parse test | Config delivery to RIB, N-way best-path algorithm (needs design) |
| cmd-4 Prefix Filter | 0/- | Skeleton spec | Full implementation (depends on policy framework) |
| cmd-5 AS-Path Filter | 0/- | Skeleton spec | Full implementation (depends on policy framework) |
| cmd-6 Community Match | 0/- | Skeleton spec | Full implementation (depends on policy framework) |
| cmd-7 Route Modify | 0/- | Skeleton spec | Full implementation (depends on policy framework + spec-apply-mods) |
| cmd-8 Policy Show | 0/- | Skeleton spec | Full implementation (depends on filters existing) |
| cmd-9 Ops | 2/3 | show uptime, show interface brief/counters, resolve ping/traceroute | rib best reason terminal (needs design) |

### Commits (4 total, 2026-04-10)
1. `feat(bgp): add route reflection, next-hop control, session policy, multipath, and operational commands` -- YANG + config + RR forwarding + next-hop + show uptime/interface (16 files)
2. `fix(config): sync session/cluster-id with loop-detection/cluster-id` -- cluster-id sync (2 files)
3. `feat(bgp): wire send-community control, AS-override, and default-originate` -- wire behavior (5 files)
4. `feat(ops): add resolve ping/traceroute, show uptime fix, interface brief YANG` -- operational (5 files)

### What Needs Design Before Implementation

| Item | Spec | Design question |
|------|------|----------------|
| RIB N-way best-path | cmd-3 | Storage model for N paths per prefix, consumer API for FIB ECMP, relax-as-path comparison semantics |
| IPv6 next-hop rewriting | cmd-1 | Type-14 MP_REACH handler vs separate rewrite path |
| rib best reason terminal | cmd-9 | Instrumentation approach: re-run with trace vs callback |

### What Can Be Implemented Directly

| Item | Spec | Key file | Effort |
|------|------|----------|--------|
| Local-AS OPEN modifiers | cmd-2 | `session_open.go` | Medium |
| Default-originate-filter | cmd-2 | `peer_initial_sync.go` | Small |
| Multipath config delivery | cmd-3 | RIB plugin Stage 2 | Medium |

### Bugs Found/Fixed
- ORIGINATOR_ID used source IP instead of BGP Identifier (fixed)
- show uptime nil CommandContext panic (fixed)

### Documentation Updates
- `docs/guide/command-reference.md` not yet updated for any new commands

### Deviations from Plan
- Filter plugins (cmd-4 through cmd-8) not started -- depend on policy framework
- IPv6 next-hop and multipath algorithm require design work not in original skeleton specs

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
- [ ] AC-1..AC-14 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

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
- [ ] Write learned summary to `plan/learned/NNN-cmd-0-umbrella.md`
- [ ] Summary included in commit
