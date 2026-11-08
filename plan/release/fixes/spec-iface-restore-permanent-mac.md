# Spec: iface-restore-permanent-mac

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | plugin |
| Priority | P1 |
| Release reason | Removing `mac address` leaves the configured MAC on the NIC until reboot: accepted config whose removal has no effect is incorrect shipped behavior |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

When the operator deletes `interface ethernet <name> mac address`, or deletes the
whole ethernet entry, the NIC keeps the MAC that Ze set. It keeps it until the box
reboots. The goal: when the leaf or the entry goes away, the device gets back the
address it came with.

Found by comparing recent VyOS fixes against Ze. VyOS fixed the same class in
T3871, commit 2b6a744cc in `~/Code/github.com/vyos/vyos-1x` ("ethernet: T3871:
restore the address the card came with, not the one on it").

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/iface/logical-name-resolution.md` - the apply path binds each ethernet entry to a kernel device once per apply, and the prune step resolves the PREVIOUS config with its own selectors
  → Constraint: the restore acts on the device the previous config was bound to, never on the logical name

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/iface/config_apply.go` `applyConfig` Phase 2 - calls `SetMACAddress` through `applyBackendStep` only when `e.MACAddress` is set. Its undo puts back the MAC read before the change. No phase acts when `MACAddress` is empty or the entry is gone
- [ ] `internal/component/iface/iface.go` `InterfaceInfo.PermanentMAC` - the backend already reads IFLA_PERM_ADDRESS. The resolver uses it for `mac match`, and `show interface` prints it
- [ ] `internal/component/iface/backend.go` `Backend.SetMACAddress` - the one write path, netlink and VPP

**Behavior to preserve:**
- A set `mac address` is applied, and a failed commit puts the previous MAC back (`applyBackendStep` journal)

**Behavior to change:**
- Removing the leaf, or the entry, restores the device's own address

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Permanent address read | `internal/component/iface/iface.go` `InterfaceInfo.PermanentMAC` | Carries IFLA_PERM_ADDRESS for every link the backend lists | extends: it is the value to restore |
| Per-step rollback journal | `internal/component/iface/config_apply.go` `applyBackendStep` | Records an undo for each backend write in a commit | extends: the restore is one more journaled step, so a failed commit undoes it |
| Previous-config binding | `internal/component/iface/config_apply.go` `bindDevices` over the previous config | Names the devices the replaced config was bound to | extends: names the device whose leaf or entry was removed |
| MAC write | `internal/component/iface/backend.go` `Backend.SetMACAddress` | Sets a link's MAC on netlink and VPP | extends: no new backend call |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Ethernet entry bound to a NIC with a permanent address sets `mac address`, commit, then the leaf is deleted and committed | The NIC carries its IFLA_PERM_ADDRESS again | none: the leaf is gone, so no configured address remains |
| AC-2 | Same as AC-1, but the whole ethernet entry is deleted | The NIC carries its IFLA_PERM_ADDRESS again | none |
| AC-3 | The device has no permanent address (IFLA_PERM_ADDRESS empty, as on a veth) | The device gets back the address it had before Ze first changed it. The design phase decides where Ze records that address, and the record survives a daemon restart | none |
| AC-4 | The leaf is removed in a commit that fails at a later step | The rollback puts the configured MAC back | the configured MAC, because the commit did not happen |
| AC-5 | `mac address` changes from A to B | The device carries B | B, the new config |

### Tests that prove each AC

| AC | Test | Entry point |
|----|------|-------------|
| AC-1, AC-2 | `TestIntegrationMACRestoredToPermanent` in a new `internal/component/iface/config_apply_mac_integration_linux_test.go`, run in Ze's kernel by `./le test qemu all` on the guest's virtio NIC, which carries a permanent address | `applyAndPublish` with the config before and after |
| AC-3 | new `test/plugin/iface-mac-address-removed-restores.ci`, `option=needs-linux`, a veth bound as ethernet: commit, delete the leaf, commit, read the link MAC | config commit through the daemon |
| AC-4 | `TestApplyMACRestoreRolledBack` (unit, fake backend, fails a later step) | `applyConfig` |
| AC-5 | existing `test/plugin/iface-mac-match-address-apply.ci` keeps passing | config commit |

## Data Flow (MANDATORY)

### Entry Point
- A config commit that removes `mac address` or an ethernet entry, delivered to the iface plugin's apply

### Transformation Path
1. `applyAndPublish` binds the new and the previous config to kernel devices
2. The apply finds each previous entry whose MAC leaf is gone, and writes the device's own address as one journaled step

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| iface component to backend | `Backend.SetMACAddress` | No |

### Integration Points
- `applyBackendStep`, `bindDevices`, `InterfaceInfo.PermanentMAC`

## Risks & Assumptions

| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | The VPP backend reports a permanent address | not read yet | AC-1 needs a VPP-side source | design phase reads `internal/plugins/iface/vpp` | unvalidated |
| A-2 | A virtio NIC in the QEMU guest carries IFLA_PERM_ADDRESS | Linux sets perm_addr for a driver-assigned address | AC-1 needs another device | design phase lists the guest's links | unvalidated |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| config commit deleting `mac address` | → | `applyConfig` restore step | `test/plugin/iface-mac-address-removed-restores.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestApplyMACRestoreRolledBack` | `internal/component/iface/config_apply_test.go` | AC-4 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `iface-mac-address-removed-restores` | `test/plugin/` | AC-3 | |

## Files to Modify
- `internal/component/iface/config_apply.go` - the restore step for a removed leaf or entry

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema | No | no leaf changes |
| Doctor check | No | no new runtime dependency |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 6 | Has a user guide page? | Yes | the interface guide's `mac address` text says what removal does |

## Implementation Steps
1. **Phase: Wiring** -- the failing `.ci` and integration test
2. **Phase: Restore step** -- in `applyConfig`, journaled

## Checklist

### Goal Gates (MUST pass)
- [ ] `./le test integration iface` and `./le test qemu all` for AC-1 and AC-2; `./le test functional plugin` for the `.ci`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `removed-setting-keeps-its-kernel-value.md` | 2026-11-08 | `applyConfig` Phase 2 never restores the permanent MAC |
