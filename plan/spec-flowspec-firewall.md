# Spec: FlowSpec-to-Firewall Bridge

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/10 |
| Updated | 2026-05-22 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/core-design.md` - registration pattern, event bus
4. `internal/component/firewall/registry.go` - multi-owner table registry
5. `internal/component/bgp/plugins/rib/events/events.go` - BestChange event shape
6. `internal/component/bgp/plugins/nlri/flowspec/types.go` - FlowSpec component types

## Task

Connect BGP FlowSpec routes to the firewall component so that:
- Received FlowSpec rules (ipv4/flow, ipv6/flow) are translated into nftables firewall rules.
- Withdrawn FlowSpec routes remove the corresponding firewall rules.
- When a BGP session carrying FlowSpec goes down, all rules learned from that peer are removed.
- FlowSpec extended community actions (discard, rate-limit, mark, redirect) map to firewall actions.

The bridge is a new plugin that subscribes to `(bgp-rib, best-change)` events for FlowSpec
families, translates each FlowSpec NLRI + extended communities into `firewall.Table`/`Chain`/`Term`
structures, registers them via `firewall.RegisterTables("flowspec", ...)`, and calls
`firewall.ApplyAll()` to reconcile.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - event bus, registration pattern, plugin lifecycle
  → Decision: plugins register via init() + RunEngine pattern; event bus is typed pub/sub
  → Constraint: in-process subscribers MUST NOT block on I/O; treat payload as read-only
- [ ] `internal/component/firewall/registry.go` - multi-owner RegisterTables/ApplyAll
  → Decision: each owner registers its desired tables; ApplyAll merges all owners
  → Constraint: passing nil tables removes the owner's contribution
  → Constraint: already used by "firewall" and "policy-routes" owners (7 call sites)
- [ ] `internal/component/firewall/model.go` - Table/Chain/Term/Match/Action types
  → Constraint: every Match/Action type must implement marker interface
- [ ] `internal/component/firewall/backend.go` - Backend.Apply reconciles kernel state
  → Constraint: only ze_* tables are touched; non-ze_* tables are never modified

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc8955.md` - FlowSpec IPv4: component types, validation, traffic actions
  → Constraint: components combine with AND logic; one FlowSpec = one firewall rule
- [ ] `rfc/short/rfc8956.md` - FlowSpec IPv6: extends component types (flow-label)
  → Constraint: IPv6 uses "destination-ipv6" (type 1), "source-ipv6" (type 2), "next-header" (type 3)
- [ ] `rfc/short/rfc5575.md` - Original FlowSpec: traffic filtering actions via extended communities
  → Constraint: traffic-rate (0x8006), traffic-action (0x8007), redirect (0x8008), traffic-marking (0x8009)

**Key insights:**
- FlowSpec NLRI = match criteria; extended communities = actions (RFC 8955 Section 7)
- The firewall already has a multi-owner registry; "flowspec" is a new owner alongside "firewall" and "policy-routes"
- The RIB stores FlowSpec in opaque map (non-CIDR); best-change events carry the family
- The firewall model already covers most FlowSpec match types (src/dst prefix, protocol, ports, DSCP, TCP flags, ICMP type/code) but lacks packet-length and fragment matches
- FlowSpec VPN (SAFI 134) is out of scope for v1 (requires VRF-aware firewall tables)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/nlri/flowspec/types.go` - FlowSpec NLRI types (13 component types)
- [ ] `internal/component/bgp/plugins/nlri/flowspec/plugin.go` - NLRI codec plugin, families: ipv4/flow, ipv6/flow, ipv4/flow-vpn, ipv6/flow-vpn
- [ ] `internal/component/bgp/plugins/nlri/flowspec/json.go` - AppendJSON for FlowSpec NLRI
- [ ] `internal/component/bgp/plugins/rib/events/events.go` - BestChangeBatch: protocol, family, changes[]
- [ ] `internal/component/bgp/plugins/rib/rib_bestchange.go` - publishes best-change; peer-down purges routes and emits withdrawals
- [ ] `internal/component/bgp/plugins/rib/storage/familyrib.go` - FlowSpec stored in opaque map keyed by wire bytes
- [ ] `internal/component/firewall/engine.go` - firewall plugin lifecycle; RegisterTables + ApplyAll
- [ ] `internal/component/firewall/registry.go` - multi-owner registry; ApplyAll merges all owners
- [ ] `internal/component/firewall/model.go` - Table/Chain/Term with Match and Action types
- [ ] `internal/component/bgp/route/route_flowspec.go` - FlowSpecRoute/FlowSpecActions parsing
- [ ] `internal/component/bgp/types/types.go` - FlowSpecActions struct (Accept, Discard, RateLimit, Redirect, MarkDSCP)
- [ ] `cmd/ze/bgp/decode_extcomm.go` - extended community wire parsing (traffic-rate, redirect, mark)

**Behavior to preserve:**
- Firewall config-driven rules continue to work unchanged (owner "firewall")
- Policy-route-driven rules continue to work unchanged (owner "policy-routes")
- FlowSpec NLRI codec is unmodified; bridge consumes decoded data
- RIB is not involved: FlowSpec routes bypass the RIB entirely
- ApplyAll merging: flowspec-owned tables coexist with other owners' tables

**Behavior to change:**
- New: FlowSpec routes produce nftables rules in ze_flowspec table(s)
- New: FlowSpec withdrawal removes corresponding terms
- New: BGP session down removes all rules from that peer

## Data Flow (MANDATORY)

### Entry Point
- BGP UPDATE carrying FlowSpec NLRI (AFI 1/2, SAFI 133) with extended communities
- Arrives via wire, decoded by bgp-nlri-flowspec plugin
- Bridge receives UPDATE events directly as a BGP plugin (not via the RIB)

### Transformation Path
1. **BGP update event** - bridge is a BGP plugin subscribed to `update` events for FlowSpec families
2. **Event parsing** - extract FamilyOps (NLRI add/withdraw per family) and ExtendedCommunities from the BGP Event
3. **NLRI decode** - parse FlowSpec NLRI wire bytes to components via `ParseFlowSpec`
4. **Action extraction** - parse extended communities for traffic actions (discard, rate-limit, mark)
5. **Hook selection** - check FlowSpec destination prefix against local interface addresses: if destination is local, use `input` hook; otherwise use `forward` hook
6. **Translation** - map FlowSpec components to `firewall.Match` types, actions to `firewall.Action` types
7. **Term construction** - one FlowSpec rule = one `firewall.Term` in the appropriate chain
8. **Table registration** - `firewall.RegisterTables("flowspec", tables)` + `firewall.ApplyAll()`
9. **Peer down** - bridge subscribes to `(bgp, state)` events; on session close, removes all rules from that peer

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| BGP engine → Bridge | BGP plugin event subscription (update + state events) | [ ] |
| Bridge → Firewall | RegisterTables + ApplyAll; typed []firewall.Table | [ ] |
| NLRI wire → Components | flowspec.ParseFlowSpec; typed *FlowSpec | [ ] |
| Iface → Bridge | EventBus (interface, addr-added/addr-removed); `iface.AddrPayload` | [ ] |

### Integration Points
- BGP plugin subscription for `update` events with FlowSpec families
- BGP plugin subscription for `state` events (peer up/down)
- `eb.Subscribe("interface", "addr-added", ...)` / `"addr-removed"` for local address tracking (`iface.AddrPayload`)
- `firewall.RegisterTables("flowspec", tables)` - register FlowSpec-derived rules
- `firewall.ApplyAll()` - reconcile merged desired state against kernel
- `flowspec.ParseFlowSpec(fam, data)` - decode wire NLRI back to components

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Design

### Plugin Structure

New plugin: `internal/plugins/flowspec-firewall/` (BGP plugin, receives UPDATE events directly).

Registers as a BGP plugin that subscribes to `update` events for FlowSpec families and
`state` events for peer tracking. Also subscribes to `(interface, addr-added/addr-removed)`
on the EventBus to maintain the local address set for hook selection. Maintains an in-memory
map of active FlowSpec rules keyed by (peer, family, NLRI wire bytes). On each event:
- **add/update**: parse NLRI + extended communities from the UPDATE event, determine hook, build Term, upsert in map, re-register + apply
- **withdraw**: remove from map, re-register + apply (empty map = nil tables = owner removed)
- **peer down**: remove all rules for that peer, re-register + apply

The bridge bypasses the RIB entirely. FlowSpec routes are ephemeral filtering instructions,
not routing state. They don't need best-path selection, dedup, or persistence. The bridge
consumes the UPDATE directly and translates to firewall rules.

### FlowSpec-to-Firewall Mapping

**Table**: one `ze_flowspec` table, family `inet` (dual-stack, handles both IPv4 and IPv6 FlowSpec).

**Chains**: two base chains in the same table, both type=filter, priority=-1, policy=accept:
- `flowspec-fwd` with hook=forward: rules for transit traffic (destination is not a local address)
- `flowspec-in` with hook=input: rules for locally-destined traffic (destination matches a local interface address)

Each FlowSpec rule is placed in exactly one chain based on its destination prefix.

### Hook Selection

The bridge maintains a set of local interface addresses by subscribing to
`(interface, addr-added)` and `(interface, addr-removed)` events on the EventBus.
The `iface.AddrPayload` provides Address + PrefixLength + Family for each event.
Same pattern as the `connected` plugin (`internal/plugins/connected/connected.go:164`).

When a FlowSpec rule arrives:
1. Extract the destination prefix from the FlowSpec components (Type 1: Destination Prefix)
2. Check if any local address falls within that destination prefix
3. If yes: place the term in `flowspec-in` (input hook) -- traffic is destined for this box
4. If no: place the term in `flowspec-fwd` (forward hook) -- traffic is transit
5. If no destination prefix in the FlowSpec rule: use `flowspec-fwd` (forward is the safe default; FlowSpec without a destination prefix is typically transit filtering)

When a local address is added or removed, re-evaluate all active rules and move terms
between chains if their hook assignment changed. This handles interface reconfiguration.

**Terms**: one term per active FlowSpec rule, named by a hash of the wire NLRI for uniqueness.

#### Component-to-Match Mapping

| FlowSpec Component | firewall.Match | Notes |
|--------------------|----------------|-------|
| Type 1: Destination Prefix | MatchDestinationAddress | Direct mapping |
| Type 2: Source Prefix | MatchSourceAddress | Direct mapping |
| Type 3: IP Protocol | MatchProtocol | Number-to-name (6="tcp", 17="udp", etc.) |
| Type 4: Port (any) | MatchSourcePort + MatchDestinationPort | Both directions |
| Type 5: Destination Port | MatchDestinationPort | Direct mapping |
| Type 6: Source Port | MatchSourcePort | Direct mapping |
| Type 7: ICMP Type | MatchICMPType | Direct mapping |
| Type 8: ICMP Code | Not directly supported | v2: add MatchICMPCode to firewall model |
| Type 9: TCP Flags | MatchTCPFlags | Direct mapping |
| Type 10: Packet Length | Not in current model | v2: add MatchPacketLength |
| Type 11: DSCP | MatchDSCP | Direct mapping |
| Type 12: Fragment | Not in current model | v2: add MatchFragment |
| Type 13: Flow Label (IPv6) | Not in current model | v2: add MatchFlowLabel |

→ Decision: v1 supports types 1-7, 9, 11 (covers the common cases: prefix, protocol, ports, TCP flags, DSCP, ICMP type). Types 8, 10, 12, 13 cause the rule to be rejected (not installed).

→ Decision: Rules with unsupported components are rejected (not installed). Conservative matching that drops/rate-limits more traffic than the sender intended is dangerous. Log a warning with the peer and NLRI details. This is the safe default per RFC 8955 Section 6 validation: "If a FlowSpec update is received that is not understood, the update MUST be silently ignored."

#### Action Mapping

| FlowSpec Extended Community | firewall.Action | Notes |
|-----------------------------|-----------------|-------|
| traffic-rate (0x8006) rate=0 | Drop | Discard = rate-limit to 0 |
| traffic-rate (0x8006) rate>0 | Limit + Accept | Rate limit in bytes/sec, then accept |
| traffic-action (0x8007) | (flags) | Terminal/sample bits; v2 |
| redirect (0x8008) | (skip) | Requires VRF support; v2 |
| traffic-marking (0x8009) | SetDSCP + Accept | DSCP remarking |
| No action community | Accept | Default: accept (no filtering) |

→ Decision: No action extended community = rule is not installed (just a match with accept is pointless nftables clutter). Only install rules that have at least one filtering/shaping action.

### Peer Tracking

The bridge tracks which rules came from which peer using its own in-memory state:
`map[peerAddr]map[nlriKey]Term`. The rule map is keyed by (peer address, NLRI wire bytes).

On peer down (via `(bgp, state)` event with state=closed), the bridge deletes all entries
for that peer, rebuilds the table, and calls RegisterTables + ApplyAll.

No RIB involvement. The bridge owns its own FlowSpec state.

### Extended Community Access

Since the bridge receives BGP UPDATE events directly (not via the RIB), it has direct
access to extended communities from the `bgp.Event.ExtendedCommunities` field. The
UPDATE event carries both the NLRI (in `FamilyOps`) and the path attributes (including
extended communities) in the same event structure. No separate lookup needed.

The extended communities are string-encoded in the event (e.g., "rate-limit:0",
"redirect:65000:100", "mark:46"). The bridge parses these strings to determine the
firewall action.

### Configuration

The bridge should be configurable:
- **Enable/disable**: `flowspec-firewall` config section (default: disabled, must opt in)
- **Chain priority**: configurable nftables priority for the flowspec chain (default: -1)
- **Maximum rules**: cap on total FlowSpec rules installed (DoS protection)

YANG config under `flowspec-firewall`:
```
flowspec-firewall {
    enabled true;
    chain-hook forward;
    chain-priority -1;
    max-rules 1000;
}
```

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Plugin registration (init) | → | flowspecfw.runEngine | `TestFlowSpecFWPluginRegistered` |
| BGP update event (FlowSpec family) | → | flowspecfw.handleUpdate | `TestFlowSpecFWUpdateWiring` |
| BGP state event (peer down) | → | flowspecfw.handlePeerDown | `TestFlowSpecFWPeerDownWiring` |
| Interface addr-added event | → | flowspecfw.handleAddrAdded | `TestFlowSpecFWAddrWiring` |
| FlowSpec add with transit dest | → | firewall.RegisterTables("flowspec", ...) in flowspec-fwd chain | `TestFlowSpecFWRuleInstalledForward` |
| FlowSpec add with local dest | → | firewall.RegisterTables("flowspec", ...) in flowspec-in chain | `TestFlowSpecFWRuleInstalledInput` |
| FlowSpec withdraw | → | term removed from table | `TestFlowSpecFWRuleWithdrawn` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | BGP peer announces FlowSpec route: destination 10.0.0.0/24 protocol tcp destination-port 80 then discard; 10.0.0.0/24 is not local | nftables rule in ze_flowspec table, `flowspec-fwd` chain (forward hook), matching dst 10.0.0.0/24 proto tcp dport 80 action drop |
| AC-2 | BGP peer withdraws the FlowSpec route from AC-1 | nftables rule is removed from ze_flowspec table |
| AC-3 | BGP session carrying FlowSpec goes down | All FlowSpec rules learned from that peer are removed |
| AC-4 | FlowSpec route with traffic-rate 0 (discard) | firewall.Drop action |
| AC-5 | FlowSpec route with traffic-rate 8000 (rate limit 8000 bytes/sec) | firewall.Limit + firewall.Accept actions |
| AC-6 | FlowSpec route with traffic-marking DSCP 46 | firewall.SetDSCP{46} + firewall.Accept actions |
| AC-7 | FlowSpec route with unsupported component (packet-length, fragment, flow-label) | Rule is NOT installed; warning logged with peer and NLRI details |
| AC-8 | FlowSpec route with no action extended community | Rule is NOT installed (no-op filtering is not useful) |
| AC-9 | IPv6 FlowSpec route: destination-ipv6 2001:db8::/32 protocol tcp then discard | Correct nftables rule in ze_flowspec inet table |
| AC-10 | Max-rules limit reached (configurable, default 1000) | New FlowSpec rules rejected with warning; existing rules preserved |
| AC-11 | FlowSpec bridge disabled in config (default) | No subscription, no table registration, no nftables state |
| AC-12 | Multiple peers announce overlapping FlowSpec rules | Each rule installed as separate term; withdrawal of one does not affect the other |
| AC-13 | FlowSpec route with destination matching a local interface address (e.g., 192.168.1.1/32 where eth0 has 192.168.1.1) | Rule placed in `flowspec-in` chain (input hook), not `flowspec-fwd` |
| AC-14 | FlowSpec route with no destination prefix component | Rule placed in `flowspec-fwd` chain (forward hook, safe default) |
| AC-15 | Local address added/removed while FlowSpec rules are active | Affected rules move between `flowspec-in` and `flowspec-fwd` chains |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestComponentToMatches` | `internal/plugins/flowspec-firewall/translate_test.go` | FlowSpec components map to correct firewall.Match types | |
| `TestActionToFirewall` | `internal/plugins/flowspec-firewall/translate_test.go` | Extended community actions map to firewall.Action types | |
| `TestBuildTerm` | `internal/plugins/flowspec-firewall/translate_test.go` | Full FlowSpec + actions produce correct Term | |
| `TestBuildTable` | `internal/plugins/flowspec-firewall/translate_test.go` | Rule map produces correct Table with two chains | |
| `TestUnsupportedComponentRejected` | `internal/plugins/flowspec-firewall/translate_test.go` | Packet-length/fragment/flow-label rules return error | |
| `TestNoActionSkipped` | `internal/plugins/flowspec-firewall/translate_test.go` | Route without action extended community returns nil Term | |
| `TestRuleMapAddRemove` | `internal/plugins/flowspec-firewall/state_test.go` | Add/remove/clear rule map operations | |
| `TestRuleMapMaxRules` | `internal/plugins/flowspec-firewall/state_test.go` | Max-rules cap enforced | |
| `TestRuleMapPeerDown` | `internal/plugins/flowspec-firewall/state_test.go` | All rules for a peer removed on peer down | |
| `TestTermNaming` | `internal/plugins/flowspec-firewall/translate_test.go` | Term names are deterministic and unique per NLRI | |
| `TestHookSelectionTransit` | `internal/plugins/flowspec-firewall/hook_test.go` | Destination not local -> forward hook | |
| `TestHookSelectionLocal` | `internal/plugins/flowspec-firewall/hook_test.go` | Destination matches local addr -> input hook | |
| `TestHookSelectionNoDestination` | `internal/plugins/flowspec-firewall/hook_test.go` | No destination component -> forward hook | |
| `TestHookReassignmentOnAddrChange` | `internal/plugins/flowspec-firewall/hook_test.go` | Local address add/remove re-evaluates chain placement | |
| `TestLocalAddrTracking` | `internal/plugins/flowspec-firewall/localaddr_test.go` | addr-added/addr-removed events maintain correct set | |
| `TestHandleUpdate` | `internal/plugins/flowspec-firewall/bridge_test.go` | BGP UPDATE events processed correctly (add/withdraw) | |
| `TestHandlePeerDown` | `internal/plugins/flowspec-firewall/bridge_test.go` | Peer state=closed removes all peer rules | |
| `TestParseExtendedCommunities` | `internal/plugins/flowspec-firewall/translate_test.go` | String ext-community parsing (rate-limit:N, mark:N, redirect:A:T) | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| max-rules | 1-100000 | 100000 | 0 | 100001 |
| chain-priority | -400..400 | 400 | -401 | 401 |
| DSCP marking | 0-63 | 63 | N/A | 64 |
| rate-limit bytes/sec | 0-4294967295 | 4294967295 | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-flowspec-fw-add` | `test/plugin/flowspec-fw-add.ci` | FlowSpec route announced, nftables rule appears | |
| `test-flowspec-fw-withdraw` | `test/plugin/flowspec-fw-withdraw.ci` | FlowSpec route withdrawn, nftables rule removed | |
| `test-flowspec-fw-peerdown` | `test/plugin/flowspec-fw-peerdown.ci` | Peer session down, all peer rules removed | |
| `test-flowspec-fw-disabled` | `test/plugin/flowspec-fw-disabled.ci` | Bridge disabled, no rules installed | |

### Future (if deferring any tests)
- VPN FlowSpec (SAFI 134) tests deferred: requires VRF-aware firewall tables (not yet implemented)
- ICMP code matching deferred: firewall model lacks MatchICMPCode
- Packet-length, fragment, flow-label matching deferred: requires new firewall Match types
- Redirect action deferred: requires VRF support
- traffic-action (sample/terminal) deferred: nftables sampling support not yet wired

## Files to Modify
- None (bridge is self-contained; no modifications to existing RIB or BGP code needed)

## Files to Create
- `internal/plugins/flowspec-firewall/register.go` - plugin registration (init + RunEngine ref)
- `internal/plugins/flowspec-firewall/engine.go` - BGP plugin lifecycle, event subscriptions, config
- `internal/plugins/flowspec-firewall/translate.go` - FlowSpec component/action to firewall.Match/Action translation
- `internal/plugins/flowspec-firewall/state.go` - rule map: (peer, nlri-wire-key) -> Term, peer tracking
- `internal/plugins/flowspec-firewall/hook.go` - hook selection logic (input vs forward based on local addrs)
- `internal/plugins/flowspec-firewall/localaddr.go` - local address set from interface events
- `internal/plugins/flowspec-firewall/translate_test.go` - translation unit tests
- `internal/plugins/flowspec-firewall/state_test.go` - state management unit tests
- `internal/plugins/flowspec-firewall/hook_test.go` - hook selection unit tests
- `internal/plugins/flowspec-firewall/localaddr_test.go` - local address tracking tests
- `internal/plugins/flowspec-firewall/bridge_test.go` - integration tests for event handling
- `test/plugin/flowspec-fw-add.ci` - functional test: rule installation
- `test/plugin/flowspec-fw-withdraw.ci` - functional test: rule removal
- `test/plugin/flowspec-fw-peerdown.ci` - functional test: peer-down cleanup
- `test/plugin/flowspec-fw-disabled.ci` - functional test: disabled state

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [x] | `internal/plugins/flowspec-firewall/schema/ze-flowspec-firewall.yang` |
| CLI commands/flags | [x] | `show flowspec firewall rules` via YANG-driven RPC |
| CLI grammar (action before identifier) | [x] | `.claude/rules/cli-grammar.md` |
| Editor autocomplete | [x] | YANG-driven (automatic if YANG updated) |
| Functional test for new RPC/API | [x] | `test/plugin/flowspec-fw-*.ci` |
| Doctor check for runtime dependencies | [x] | Firewall backend must be loaded; `ze doctor` already checks this |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` |
| 2 | Config syntax changed? | [x] | `docs/guide/configuration.md` |
| 3 | CLI command added/changed? | [x] | `docs/guide/command-reference.md` - `show flowspec firewall` |
| 4 | API/RPC added/changed? | [ ] | - |
| 5 | Plugin added/changed? | [x] | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | [x] | `docs/guide/flowspec-firewall.md` |
| 7 | Wire format changed? | [ ] | - |
| 8 | Plugin SDK/protocol changed? | [ ] | - |
| 9 | RFC behavior implemented? | [x] | `rfc/short/rfc8955.md` (annotate traffic filtering actions section) |
| 10 | Test infrastructure changed? | [ ] | - |
| 11 | Affects daemon comparison? | [x] | `docs/comparison.md` - FlowSpec-to-firewall column |
| 12 | Internal architecture changed? | [ ] | - |

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

1. **Phase: Wiring (MANDATORY FIRST)** -- register entry points, write failing wiring tests
   - Tests: `TestFlowSpecFWPluginRegistered`, `TestFlowSpecFWUpdateWiring`, `TestFlowSpecFWPeerDownWiring`, `TestFlowSpecFWAddrWiring`
   - Files: `register.go`, `engine.go` skeleton
   - Verify: plugin registered in `cmd/ze/main_test.go` expected list; wiring tests fail because handlers are stubs

2. **Phase: Local address tracking** -- maintain set of local interface addresses
   - Tests: `TestLocalAddrTracking`
   - Files: `localaddr.go`, `localaddr_test.go`
   - Verify: addr-added/addr-removed events correctly maintain the local address set

3. **Phase: Hook selection** -- determine input vs forward based on destination + local addrs
   - Tests: `TestHookSelectionTransit`, `TestHookSelectionLocal`, `TestHookSelectionNoDestination`, `TestHookReassignmentOnAddrChange`
   - Files: `hook.go`, `hook_test.go`
   - Verify: correct chain assignment for local/transit/absent destinations; reassignment on addr changes

4. **Phase: Translation** -- FlowSpec components to firewall.Match/Action
   - Tests: `TestComponentToMatches`, `TestActionToFirewall`, `TestBuildTerm`, `TestUnsupportedComponentRejected`, `TestNoActionSkipped`, `TestTermNaming`, `TestParseExtendedCommunities`
   - Files: `translate.go`, `translate_test.go`
   - Verify: all component types produce correct Match types; unsupported components rejected; ext-community strings parsed

5. **Phase: State management** -- rule map with add/remove/rebuild, per-peer tracking
   - Tests: `TestRuleMapAddRemove`, `TestRuleMapMaxRules`, `TestRuleMapPeerDown`
   - Files: `state.go`, `state_test.go`
   - Verify: map operations correct; max-rules enforced; peer-down clears all peer rules

6. **Phase: Bridge logic** -- connect BGP update/state events to state and firewall
   - Tests: `TestHandleUpdate`, `TestHandlePeerDown`, `TestFlowSpecFWRuleInstalledForward`, `TestFlowSpecFWRuleInstalledInput`, `TestFlowSpecFWRuleWithdrawn`
   - Files: `engine.go`
   - Verify: full path from BGP event to RegisterTables + ApplyAll; correct chain placement

7. **Phase: YANG config + CLI** -- config schema and show command
   - Files: `schema/ze-flowspec-firewall.yang`, show handler
   - Verify: `ze config validate` accepts flowspec-firewall config

7. **Functional tests** -- Create after feature works
8. **RFC refs** -- Add `// RFC 8955 Section X.Y` comments
9. **Full verification** -- `make ze-verify`
10. **Complete spec** -- Fill audit tables, write learned summary, delete spec

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | FlowSpec component-to-match mapping matches RFC 8955 component definitions |
| Naming | Term names deterministic and collision-free; table name ze_flowspec; chain name flowspec-in |
| Data flow | Event -> parse -> translate -> register -> apply; no shortcut paths |
| CLI grammar | `show flowspec firewall rules` follows action-before-identifier |
| Doctor checks | Firewall backend loaded check covers flowspec-firewall dependency |
| Rule: no-layering | No duplicate translation logic; reuses flowspec.ParseFlowSpec |
| Rule: buffer-first | No hot-path allocations in event handler beyond term construction |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Plugin registered | `grep -rn flowspec-firewall cmd/ze/main_test.go` |
| Translation code | `ls internal/plugins/flowspec-firewall/translate.go` |
| Hook selection code | `ls internal/plugins/flowspec-firewall/hook.go` |
| Local addr tracking | `ls internal/plugins/flowspec-firewall/localaddr.go` |
| State management | `ls internal/plugins/flowspec-firewall/state.go` |
| Engine wired to BGP events | `grep -rn 'Subscribe.*update\|Subscribe.*state\|addr-added' internal/plugins/flowspec-firewall/` |
| RegisterTables called | `grep -rn RegisterTables internal/plugins/flowspec-firewall/` |
| YANG schema | `ls internal/plugins/flowspec-firewall/schema/` |
| Functional tests | `ls test/plugin/flowspec-fw-*.ci` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | FlowSpec NLRI from untrusted peers: malformed components must not crash; ParseFlowSpec already handles this |
| Resource exhaustion | Max-rules cap prevents peer flooding nftables with thousands of rules |
| Privilege | nftables operations require CAP_NET_ADMIN; firewall backend already handles this |
| Rule injection | Term names derived from NLRI hash, not user-controlled strings; no injection vector |
| Stale rules | Session teardown must clean up; verify no leaked rules after peer flap |

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

### Failed Approaches
| Approach | Why abandoned | Replacement |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |

## Design Insights

## RFC Documentation

Add `// RFC 8955 Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, component type mapping, traffic action semantics.

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

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-15 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
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

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] Summary included in commit
