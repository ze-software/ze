# Spec: bgp-conditional-advertisement

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | plugin |
| Priority | P3 |
| Release reason | Adds a capability absent from the supported release: advertise a set of routes to a peer only while another route exists (or does not exist) in the BGP table. No RFC governs it. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

## Task

BGP conditional advertisement lets an operator say: "advertise these routes to
this peer only while route X is (or is not) present in my BGP table". The
classic use is multihoming: announce a backup prefix to the secondary upstream
only when the default route from the primary upstream disappears.

FRR calls it `advertise-map <map> exist-map <map>` or `non-exist-map <map>`
per neighbor and address family; Cisco and Juniper have the same feature.
Ze has no way to make an export decision depend on the presence of another
route.

### VyOS reference

VyOS `protocols bgp neighbor <n> address-family <af> conditionally-advertise
advertise-map <rm> exist-map|non-exist-map <rm>` and the global
`parameters conditional-advertisement timer <s>`. T9450 (vyos-1x `c46b3c3f0`)
stopped re-rendering the default timer. FRR scans on a timer, 60 seconds by
default: `bgp_conditional_adv_timer` and `bgp_conditional_adv_routes`
(`bgpd/bgp_conditional_adv.c`). BIRD has no named feature, so FRR is the
interop reference.

## Required Reading

- [ ] `docs/guide/bgp-policy.md` - the policy model, filter outcomes, "Conditional defaults and stale readvertisement"
- [ ] `docs/architecture/core-design.md`, "Policy Filter Chain" - per-peer chains, declared filter insertions
- [ ] `docs/architecture/plugin/plugin-system.md` - filter declarations and the `filter-update` RPC
- [ ] `ai/digests/rib.md` - how `bgp-rib` stores best paths and emits `(bgp-rib, best-change)`

## Current Behavior

- [ ] `docs/guide/bgp-policy.md` - the only conditional export is `default-originate-filter`, which withholds the originated default route when a named filter rejects it. No export member can read the presence of a different prefix.
- [ ] `internal/component/bgp/plugins/watchdog/watchdog.go` - the watchdog plugin announces and withdraws configured route pools on command, through "update text" commands, with no condition on the RIB.

## Existing Mechanisms

| Mechanism | Where | What it does today | This spec |
|-----------|-------|--------------------|-----------|
| Per-peer export chain and `filter-update` RPC | BGP reactor; `pkg/plugin/rpc` `FilterUpdateInput` | Runs each egress route through the peer's chain on both transports | Extends: the advertise selector is an export member whose verdict also depends on the condition state |
| Declared filter insertions | `rpc.FilterInsertion`, `CheckFilterInsertionPaths` | A plugin inserts the member a feature's config implies | Reused: the feature's plugin inserts its member on the peers whose config names it (`ai/META-DECISIONS.md`, every route-processing step is a filter) |
| Best-path change events | `(bgp-rib, best-change)` | Notify subscribers of Loc-RIB changes | Reused: the condition re-evaluates on events rather than on a 60-second scan |
| Re-export of already-sent routes | stale readvertisement (`bgp-policy.md`), `spec-bgp-refresh-reapplies-export-policy` | Re-runs export members that opt into `readvertise` | Extends: a condition flip announces or withdraws the selected routes to that peer; the design names the path and either inherits that spec's known defects or waits for it |
| `default-originate-filter` | reactor `defaultOriginateFilterAccepts` | Conditional origination of the default only | Does not apply: one prefix, origination only |
| Prefix and community match types | `filter_prefix`, `filter_community_match` | Match conditions a chain reaches | Reused for the advertise, exist and non-exist selectors |

## Design questions owed

1. **Where the condition is evaluated.** A plugin that subscribes to
   best-change and holds the condition per (peer, family). It must work forked
   (`ai/doors/system.md`): no in-process call into `bgp-rib`.
2. **What "exists" means.** FRR matches the exist-map against BGP best paths,
   attributes included. State whether Ze reads the bgp-rib best path or the
   system RIB, and why.
3. **Damping.** Event-driven evaluation can flap advertisements as fast as the
   condition route flaps. Decide whether a hold-down leaf exists, and reuse
   FRR's timer name only if the meaning is the same.

## Data Flow

### Entry Point
Per-peer, per-family config naming an advertise selector and one exist or non-exist selector.

### Transformation Path
1. The plugin evaluates the condition at start and on each best-change.
2. Its inserted export member rejects advertise-selected routes while the condition is false.
3. A flip triggers announcement or withdrawal of those routes toward the peer.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin to reactor | `filter-update` verdicts; a re-export request through a command or RPC | No |
| bgp-rib to plugin | `(bgp-rib, best-change)` events | No |

### Integration Points
- Filter insertion declaration; best-change subscription; the re-export trigger.

## Wiring Test

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Peer config with advertise and non-exist selectors | → | inserted export member and condition tracker | [functional `.ci` named at design] |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Non-exist condition on 0.0.0.0/0 from upstream A; advertise selector 192.0.2.0/24 toward upstream B; A's default present | B does not receive 192.0.2.0/24 |
| AC-2 | A withdraws its default | B receives 192.0.2.0/24 within the bound the design sets, with no session reset |
| AC-3 | A re-announces its default | 192.0.2.0/24 is withdrawn from B |
| AC-4 | Exist condition instead | The inverse of AC-1 to AC-3 |
| AC-5 | Routes outside the advertise selector | Unaffected by the condition in every state |
| AC-6 | The plugin runs forked | AC-1 to AC-3 hold |
| AC-7 | A show command | Prints each condition, its state and the time of the last flip |
| AC-8 | A reload changes a selector | The condition is re-evaluated and the peer converges without a reset |
| AC-9 | Interop: Ze advertises to FRR | FRR's table follows AC-1 to AC-3; proven red with the condition check reverted |

## Risks & Assumptions

| ID | Risk | Mitigation |
|----|------|------------|
| R-1 | The re-export path re-applies non-idempotent export members (known defect in `spec-bgp-refresh-reapplies-export-policy`) | Depend on that fix, or use a path that runs the whole chain once |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| [named at design] | the new plugin's package | The condition state machine for exist and non-exist | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| [named at design] | `test/plugin/` | The backup prefix appears when the primary default disappears | |

## Files to Modify

- A new plugin under `internal/component/bgp/plugins/`, with its YANG augment of the peer
- `docs/guide/bgp-policy.md`, `docs/guide/plugins.md`

## Implementation Steps

1. Answer the design questions; name the plugin and the selector syntax.
2. Tests first, then the condition tracker, the inserted member and the re-export trigger.
3. FRR interop scenario.

### Integration Checklist
- [ ] `./le plugin declarations check` and `./le plugin boundary check` pass

### Documentation Update Checklist
- [ ] `docs/guide/bgp-policy.md` gains a conditional advertisement section

## Checklist

- [ ] Tests written
- [ ] Tests FAIL before the change
- [ ] Tests PASS after the change
- [ ] `./le test functional plugin`, `./le test integration interop` with the new scenario
