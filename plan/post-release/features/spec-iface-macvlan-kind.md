# Spec: iface-macvlan-kind

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | config |
| Priority | P3 |
| Release reason | Adds an operator-configured interface kind absent from the supported release (VyOS `pseudo-ethernet`). Plugin-owned macvlans (VRRP) stay shipped and required |
| RFC gaps | - |
| Depends | spec-iface-bridge-vlan-filtering.md (only for AC-6, the VLAN-aware bridge case) |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

An operator must be able to configure a macvlan interface (VyOS calls it
`pseudo-ethernet`) on a parent ethernet, VLAN unit or bridge: a second MAC on
the same wire, with its own addresses. The main use is an anycast gateway: the
same gateway MAC and address on several VLANs of an EVPN-style bridge, so a
host keeps its gateway when it moves. VyOS broke this case (T9253): two
anycast gateways with one MAC on different bridge VLANs failed to commit with
"bridge fdb add ... File exists", and at boot only one survived.

| # | Operator need |
|---|---------------|
| 1 | A macvlan on eth0 with its own MAC, address and mode (bridge, private, vepa, passthru) |
| 2 | Anycast gateway: macvlans with one shared MAC on several VLAN units of one bridge |
| 3 | Removal restores the parent exactly, including any bridge FDB entry Ze added |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/iface/logical-name-resolution.md` - "The plugin-facing registries take the resolved device too"
  → Constraint: `MacvlanSpec.Parent` is an OS device name; a parent named in config goes through `ResolveDevice`, never straight to `LinkByName`
  → Constraint: a macvlan wears its own MAC, not the parent's; the parent's `mac match` must stay unambiguous with macvlans on it

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/iface/macvlan.go` - `MacvlanSpec{Name, Parent, MAC, Mode, Alias}`, modes bridge and private only, `validate` refuses a zero or multicast MAC; used by plugin-owned devices
- [ ] `internal/component/iface/backend.go` - `CreateMacvlanDevice(spec)`: the netlink backend creates with alias `ze:owned:<owner>` and the MAC atomically; non-netlink backends refuse
- [ ] `internal/component/iface/yang/ze-iface-conf.yang` - no operator macvlan or pseudo-ethernet kind
- [ ] `internal/component/iface/config.go` - interface kinds parsed into typed entries (`ifaceEntry`, `vethEntry`, `bridgeEntry`)

**Behavior to preserve:**
- VRRP's owned macvlans and their reconcile.

**Behavior to change:**
- An operator-configured macvlan kind, built on the same backend call, with the bridge FDB handling the anycast case needs.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Owned macvlan devices | `internal/component/iface/macvlan.go` `MacvlanSpec`; `internal/component/iface/device_owner.go` | A plugin (VRRP) registers a macvlan; the reconcile pass creates, marks and deletes it | Extends: the config kind reuses `MacvlanSpec` and `CreateMacvlanDevice`, gaining the vepa and passthru modes; whether config entries go through the owned-device registry or the config apply is a design decision, never a second create path |
| Selector binding and `ResolveDevice` | logical-name-resolution page | Translates a configured parent to its device | Extends: the parent is resolved the same way |
| Bridge kind | `bridgeEntry`, `BridgeAddPort` | Non-VLAN-aware bridges | Extends: the FDB local entry for a macvlan above a bridge |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: a new interface list for macvlans (name settled at design) with `source-interface`, `mode`, `mac address`, units and addresses.

### Transformation Path
1. Verify: source interface exists in config; MAC unicast; two macvlans with one MAC on one source interface refused.
2. Apply: resolve the source device; create the macvlan; when the source sits on a bridge, replace a local FDB entry for the MAC on the bridge, bound to the VLAN when the bridge is VLAN-aware.
3. Removal: delete the macvlan; delete the FDB entry only when no other configured macvlan still needs that MAC and VLAN.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| iface to kernel | rtnetlink macvlan link; RTM_NEWNEIGH AF_BRIDGE with NTF_SELF, NUD_PERMANENT, NLM_F_REPLACE | No |

### Changed-Wire Census

**Search evidence:** owed at design.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| [format] | [role] | [source site] | [finding] | [disposition] |

### Integration Points
- The owned-device reconcile, the bridge apply.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound | No | |
| Registration over hardcoding, inbound | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A macvlan above a bridge (or a bridge VLAN device) needs a local FDB entry for its MAC, or the bridge does not deliver frames for that MAC upward | VyOS installs `bridge fdb add <mac> dev <bridge> self local` (T9253) | No FDB step needed | Integration test with and without the entry | unvalidated |
| A-2 | On a VLAN-aware bridge, one MAC on several VLANs needs one FDB entry per VLAN; on a non-VLAN-aware bridge they share one entry | VyOS T9253 `e3643b21a` | Different keying | Integration test after `spec-iface-bridge-vlan-filtering.md` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A failure on one macvlan at boot aborts the rest of the config (VyOS T9253) | Only one anycast gateway configured | Replace semantics on the FDB entry; per-entry errors never abort sibling entries |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | The gateway for every host on the affected VLANs |
| How is it reverted? | Remove the macvlans |
| Who else touches this path? | VRRP owned macvlans, the bridge apply, selectors |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A macvlan on eth0 committed | → | config macvlan create through `CreateMacvlanDevice` | `test/plugin/iface-macvlan-apply.ci` (`option=needs-linux`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | macvlan on eth0, mode bridge, MAC and address set | Device exists with that MAC, mode and address; a host on eth0's segment reaches the address | None |
| AC-2 | Each mode: bridge, private, vepa, passthru | The kernel reports the configured mode | None |
| AC-3 | Source interface bound by `mac match` | The macvlan's lower device is the resolved device | None |
| AC-4 | Two macvlans with one MAC on one source interface | Verify refuses naming both (VyOS T9253) | None |
| AC-5 | Anycast gateway: macvlans with one MAC on VLAN units 10 and 20 of non-VLAN-aware bridge br0 | Both commit; one local FDB entry; re-commit does not fail with "exists" | None |
| AC-6 | Same on a VLAN-aware bridge | One local FDB entry per VLAN, each bound to its VLAN, installed with replace (VyOS T9253) | Waits on `spec-iface-bridge-vlan-filtering.md` |
| AC-7 | One of the anycast macvlans removed | Its FDB entry stays while another macvlan needs it, and goes with the last one | None |
| AC-8 | A boot where one macvlan cannot be created | The others and the rest of the config still apply; the failure names the entry | None |
| AC-9 | `backend vpp` | Commit refused with the YANG path (VPP has no macvlan) | None |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures an anycast gateway on two VLANs | config, create, FDB | `test/plugin/iface-macvlan-anycast.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestMacvlanSharedMACRules` | `internal/component/iface/config_macvlan_test.go` | AC-4, AC-7 | |

### Boundary Tests (numeric inputs)
N-A: no numeric input.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `iface-macvlan-apply` | `test/plugin/` | AC-1, AC-2, AC-3 | |
| `iface-macvlan-anycast` | `test/plugin/` | AC-5, AC-7, AC-8 | |

## Files to Modify
- `internal/component/iface/yang/ze-iface-conf.yang` - the kind
- `internal/component/iface/macvlan.go` - vepa and passthru modes
- `internal/component/iface/config.go` - parse
- `internal/component/iface/backend.go` - a bridge FDB local entry call

## Files to Create
- `test/plugin/iface-macvlan-apply.ci`, `test/plugin/iface-macvlan-anycast.ci`

## Implementation Steps

1. **Phase: Wiring** - kind, parse, failing `.ci`.
2. **Phase: Create** - reuse `CreateMacvlanDevice`, modes, selector parent.
3. **Phase: Anycast** - FDB replace, refcounted removal, per-entry failure isolation.

Proof: `./le test unit plugins`, `./le test functional plugin` for the named `.ci` files, `./le test qemu all`.

## VyOS reference

VyOS checkout `~/Code/github.com/vyos/vyos-1x`, read 2026-11-08 at `611550d23`.

| Ticket / commit | Lesson for Ze |
|-----------------|---------------|
| T9253, `e3643b21a` "pseudo-ethernet: fix anycast-gateway with shared MAC on bridge VLANs" | The local FDB entry was installed without a VLAN, so every interface competed for one entry; a failing boot commit left one interface and dropped the rest of the config. Now bound to the VLAN on a VLAN-aware bridge, installed with replace, and two anycast gateways with one MAC on one source interface are refused. AC-4 to AC-8 |

## Design Insights

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
- Macvtap is not in this spec.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason): N-A, no protocol peer
