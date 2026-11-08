# Spec: firewall-zones

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | config |
| Priority | P3 |
| Release reason | Zone-based firewall policy is a capability VyOS offers and Ze lacks; the shipped table, chain and term model is complete without it, so it is post-release feature work. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

An operator wants to write policy as "traffic from LAN to WAN may do this,
WAN to DMZ may do that, and anything to the router itself is limited", naming
groups of interfaces (zones) instead of writing one chain per interface pair by
hand. A zone holds interfaces (or a VRF), a default action for traffic between
zones with no policy, and per source zone a named rule set. A special local zone
stands for the router itself. Ze compiles zones into its existing tables,
chains, terms and jumps; it adds no second firewall engine.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/forwarding.md` - nft table ownership, interface names through the shared resolver
- [ ] `docs/architecture/iface/logical-name-resolution.md` - how a configured interface name becomes a device
- [ ] `docs/guide/firewall.md` - tables, chains, terms, `jump`, `goto`

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/firewall/model.go` - `Table`, `Chain`, `Jump`, `Goto`, `MatchInputInterface`, `MatchOutputInterface`; no zone type.
- [ ] `internal/component/firewall/yang/ze-firewall-conf.yang` - `table`, `chain`, `term`, `set`, `flowtable`; no `zone`.
- [ ] `internal/component/firewall/config.go` `ParseFirewallConfig` - parses tables only.
- [ ] `internal/component/firewall/validate.go` `validateTerm` - checks jump and goto targets against the chain names of the table.
- [ ] `internal/plugins/firewall/nft/lower_linux.go` `lowerMatch` - interface name matches with prefix wildcard; no verdict map.

**Behavior to preserve:** every operator-written table.

**Behavior to change:** add zones that generate Ze-owned tables.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Table, chain, term model | `model.go` `Table`, `Chain`, `Jump` | The whole firewall | Extends: zones compile into one generated `ze_` table of base chains (forward, input, output) dispatching by interface to per zone-pair chains |
| Jump target validation | `validate.go` `validateTerm` | Refuses a jump to an unknown chain | Extends: every generated jump passes the same check before apply (AC-4) |
| Table registry | `firewall.RegisterTables`, `ApplyAll` | Producers register owned tables | Extends: the zone compiler registers its table like policy routing does |
| Interface resolver | `docs/architecture/iface/logical-name-resolution.md` | Config name to device | Extends: zone members resolve through it |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: `firewall zone <name>` with members, default action, and `from <zone> firewall <rule-set>` entries; a `local-zone` flag.

### Transformation Path
1. Parse zones, verify membership and references.
2. Compile to forward, input and output base chains that dispatch on input and output interface to one chain per zone pair, falling back to the destination zone's default action.
3. Register the generated table; `ApplyAll` lowers it.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config → firewall component | YANG tree | No |
| Firewall component → nft backend | `Backend.Apply` | No |

### Integration Points
- `ParseFirewallConfig`, `ValidateTables`, `RegisterTables`

## Wiring Test (MANDATORY -- NOT deferrable)
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `firewall zone` config | → | zone compiler and generated table | `test/firewall/zone-basic.ci` (proposed) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Zones LAN and WAN, policy LAN to WAN accept, WAN default drop | LAN to WAN traffic passes, WAN to LAN new traffic is dropped and counted, established replies pass | - |
| AC-2 | Local zone with a policy from WAN | Traffic to the router from WAN interfaces follows that policy at the input hook; traffic the router sends follows the output side | - |
| AC-3 | A zone whose member is a VRF, combined with a local zone | Local traffic arriving in the VRF reaches the right zone policy | - |
| AC-4 | Any zone combination, including a VRF-member zone with a default firewall inside a local zone | Every generated jump names an existing chain; the generated table passes `ValidateTables` before apply (VyOS `1e49a0330`: a VRF zone in a local zone rendered a jump with an empty target and the commit failed) | - |
| AC-5 | An interface in two zones, a policy naming an unknown zone, or a zone with no members | Refused at verify naming both | - |
| AC-6 | Intra-zone traffic | Follows the zone's intra-zone setting (accept by default, or a named rule set) | - |
| AC-7 | Zones together with operator tables | Both apply; their priority order is documented and tested | - |
| AC-8 | VPP backend | Refused at verify (VPP refuses non-base chains) unless an exact mapping is designed | - |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCompileZones` | `internal/component/firewall/` | AC-1 to AC-4 generated chains | |
| `TestZoneVerify` | `internal/component/firewall/` | AC-5 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `zone-basic` | `test/firewall/zone-basic.ci` | AC-1, AC-2 | |

## Implementation Steps

1. **Phase: Wiring** -- YANG, parse, failing `.ci`
2. **Phase: Compiler** -- AC-1 to AC-6
3. **Phase: Coexistence and VRF** -- AC-3, AC-7

## Files to Modify
- `internal/component/firewall/config.go`, `validate.go`
- `internal/component/firewall/yang/ze-firewall-conf.yang`
- `docs/guide/firewall.md`

## Files to Create
- `test/firewall/zone-basic.ci`

## VyOS reference

T9549 (merge `3a0f8ad43`, PR #5614): `1e49a0330` "Fix default-firewall jump for
VRF zone in local zone". AC-4 carries it.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] Targeted tests pass: `./le test unit plugins`, `./le test firewall`, `./le test integration firewall`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
