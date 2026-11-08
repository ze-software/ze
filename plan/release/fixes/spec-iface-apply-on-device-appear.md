# Spec: iface-apply-on-device-appear

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | plugin |
| Priority | P1 |
| Release reason | A NIC that enumerates after the commit stays unconfigured until the next commit: a late probe, a USB NIC or a replaced card leaves the box without its addresses and MTU |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

An ethernet entry whose device is absent at commit is UNBOUND, and every apply
phase skips it (`docs/architecture/iface/logical-name-resolution.md`). That is
correct at commit time. The defect is that nothing applies the entry when the
device appears later: a driver that probes late, a USB NIC, or a replaced card
whose MAC the `mac match` selector names. The device stays without its
addresses, MTU, MAC, admin state, units and offloads until an unrelated commit
runs the apply again. A mirror whose target appears later is never installed
either.

The goal: when a device that an entry of the active config binds to appears,
Ze applies that entry, the same way a commit would, without a commit.

Found by comparing recent VyOS fixes against Ze. VyOS fixed the same class in
T3871 (c95aae771, "name hot-plugged interfaces") and T6393 (99271db4a,
5359ab6f4 "mirror and redirect into a target created later"), in
`~/Code/github.com/vyos/vyos-1x`.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/iface/logical-name-resolution.md` - "Cache invalidation rides the monitor events": a `created` or `up` event wakes every `mac/match` name that does not know its device, and the event-bus handler must not make a backend call
  → Constraint: the handler only signals. A worker does the backend I/O, as the registry reconcile already does
  → Constraint: the apply binds against one interface listing, so a re-apply re-reads it

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/iface/register.go` `EventCreated` subscriber and `subscribeLinkEvents` callback - signal `registryReconcileCh` only when the new device is a registered macvlan parent (`isRegisteredMacvlanParent`). Any other device appearing triggers nothing
- [ ] `internal/component/iface/resolve.go` `resolver.onLinkEvent` - drops the resolver cache and notifies subscribers. It applies no config
- [ ] `internal/component/iface/config_apply.go` `reconcileOnRegistryChange` - runs `reconcileOnReady` (address reconcile, mirror reconcile, prune). It never runs Phase 1 and 2 (create, MTU, MAC, admin state, units, offload)

**Behavior to preserve:**
- An absent device does not fail a commit
- The event-bus handler makes no backend call

**Behavior to change:**
- A device appearing that an active entry binds to (by name, `os-name` or `mac match`) gets that entry's whole config, and a mirror whose target appears is installed

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Created-link signal | `internal/component/iface/register.go` `EventCreated` subscriber | Signals the registry reconcile for a registered macvlan parent | extends: signals a re-apply for any device the active config binds |
| Background reconcile worker | `internal/component/iface/register.go` `registryReconcileCh` worker, `internal/component/iface/config_apply.go` `reconcileOnRegistryChange` | Runs address, mirror and prune reconcile off the event goroutine, serialized by `reconcileMu` | extends: the same worker pattern runs the Phase 1 and 2 apply for the entries that bind to the new device |
| Resolver wake for `mac match` names | `internal/component/iface/resolve.go` `logicalsForLocked` | Names the logical entries a created device may answer | extends: gives the set of entries to re-apply |
| VPP-ready deferred reconcile | `internal/component/iface/config_apply.go` `reconcileOnReady`, vpp connect handler | Runs deferred work once VPP connects | does not apply: VPP readiness, not device arrival |
| Absent mirror destination retry | `plan/release/fixes/spec-network-services-one-off-defects.md`, AC-22 | Designed: an absent destination is reported and retried | extends: this spec supplies the trigger AC-22's retry needs. AC-22 keeps the no-abort half |

## Data Flow (MANDATORY)

### Entry Point
- An RTM_NEWLINK for a device, delivered as `(interface, created)` on the EventBus

### Transformation Path
1. The subscriber decides from the active config whether any entry may bind the device, and signals the worker
2. The worker lists interfaces once, binds the active config, and applies Phase 1 and 2 and the reconcile for the newly bound entries, under `reconcileMu`

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| netlink monitor to iface | EventBus `created` event | No |

### Integration Points
- `registryReconcileCh` worker, `reconcileMu`, `bindDevices`, `applyConfig` Phase 1 and 2

## Risks & Assumptions

| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | Phase 1 and 2 can run for a subset of entries without a commit transaction | not read yet | the re-apply needs its own journal and rollback scope | design reads `applyConfig` whole | unvalidated |
| R-1 | A device that flaps re-applies on every create | | | the design bounds it to entries whose binding changed | unvalidated |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| veth created after commit | → | re-apply worker | `test/plugin/iface-device-appears-after-commit.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Commit an ethernet entry with `mac match`, an address and an MTU while no device carries that MAC; then create a veth with that MAC | Within a bounded wait the veth carries the configured address and MTU and is admin up, with no commit | none |
| AC-2 | Same with `os-name` naming the absent device | Same | none |
| AC-3 | A unit mirrors toward a declared target that is absent at commit; the target is created later | The tc mirred filter is installed toward it, with no commit | none |
| AC-4 | A device appears that no entry binds | Ze changes nothing on it | the device's own state, because Ze does not own it |
| AC-5 | The device disappears and appears again | The config is applied again | none |

### Tests that prove each AC

| AC | Test | Entry point |
|----|------|-------------|
| AC-1, AC-5 | new `test/plugin/iface-device-appears-after-commit.ci`, `option=needs-linux`: commit with the NIC absent, add a veth with the matched MAC, assert the address and MTU appear, delete and re-add it, assert again | daemon config commit, then a kernel link create |
| AC-2 | the same `.ci`, a second entry bound by `os-name` | same |
| AC-3 | new `test/plugin/iface-mirror-target-appears-later.ci`, `option=needs-linux`, reads the tc filter | same |
| AC-4 | `TestIntegrationUnboundDeviceUntouched` in `internal/component/iface/config_apply_resolve_integration_linux_test.go`, run by `./le test integration iface` | the worker |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCreatedEventSignalsReapplyForBoundEntry` | `internal/component/iface/register_test.go` | the subscriber signals for a bound device and not for an unbound one | |

## Files to Modify
- `internal/component/iface/register.go` - the `created` subscriber and the worker
- `internal/component/iface/config_apply.go` - a re-apply for the newly bound entries
- `docs/architecture/iface/logical-name-resolution.md` - the UNBOUND paragraph says the entry is applied when its device appears

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema | No | no config change |
| Doctor check | No | no new runtime dependency |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 12 | Internal architecture changed? | Yes | `docs/architecture/iface/logical-name-resolution.md` |

## Implementation Steps
1. **Phase: Wiring** -- the failing `.ci` for AC-1
2. **Phase: Signal and worker** -- AC-1, AC-2, AC-4, AC-5
3. **Phase: Mirror** -- AC-3

## Checklist

### Goal Gates (MUST pass)
- [ ] `./le test functional plugin` for the new `.ci` files in the QEMU guest; `./le test integration iface`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `listener-never-binds-a-resource-that-appears-later.md` | 2026-11-08 | iface apply never runs for a device that appears after commit |
