# Spec: iface-nic-slot-selector

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | config |
| Priority | P3 |
| Release reason | Adds a selector absent from the supported release: binding an ethernet entry to a PCI slot. `os-name` and `mac match` remain the shipped selectors and their obligations stay required |
| RFC gaps | - |
| Depends | `plan/release/fixes/spec-iface-apply-on-device-appear.md` (a NIC that appears after boot gets its entry applied) |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

When a NIC fails and is replaced, the new card has another MAC. An entry bound
by `mac match` then binds to nothing, and the box is unreachable on that port
until someone edits the config at the console. An operator must be able to
bind an ethernet entry to the PCI slot the card sits in, so a replacement card
in the same slot takes the same config. And where an entry is bound by
`mac match`, `ze doctor` must say when that entry is unbound while an
unclaimed NIC sits in the slot the matched card used to occupy: that is a
replaced card, and the doctor names the edit that rebinds it.

| # | Operator need |
|---|---------------|
| 1 | Bind `interface ethernet uplink` to PCI address `0000:03:00.0` (bus path), so any card in that slot is `uplink` |
| 2 | Keep `mac match` for operators who want the binding to follow the card |
| 3 | After a card swap with `mac match`, `ze doctor` reports the unbound entry, the slot, the new card's MAC and the `mac match` edit that adopts it |
| 4 | When cards moved between slots, or two known cards swapped, the report says so rather than looking like nothing changed |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/iface/logical-name-resolution.md` - the shared resolver, selectors, unbound entries, the doctor's sysfs route
  → Constraint: an entry whose selector names no present device is UNBOUND; no phase falls back to the logical name
  → Constraint: the doctor runs where no backend runs and reads selectors from the config tree through `SelectedNetDevice`; a new selector joins that helper
  → Constraint: a selector several devices answer is refused by the apply (error), one no device answers is deferred (warning)
- [ ] `docs/guide/vpp.md` - `vpp dpdk interface <pci-address>`: the VPP backend already names NICs by PCI address

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/iface/yang/ze-iface-conf.yang` - `container mac` holds `address` and `match`; `match` binds by IFLA_PERM_ADDRESS, applies to ethernet only, and waits when absent; `os-name` aliases by name. No PCI or bus selector
- [ ] `internal/component/iface/config.go` - `ifaceEntry.OSName`, `ifaceEntry.MatchMAC`
- [ ] `internal/component/iface/config_apply.go` - `deviceFor`; the Phase 2 loop keys work by the bound device
- [ ] `internal/component/iface/doctor_linux.go` - `checkEthernetInterfaces` with codes `iface-missing`, `iface-down`, `CodeDoctorIfaceSelectorUnmatched` (warning), `CodeDoctorIfaceSelectorAmbiguous` (error); skips the VPP backend
- [ ] `internal/component/host/nic_linux.go` - `show host nic` reads `device/vendor`, `device/device` and decides a physical NIC by the presence of `/sys/class/net/<n>/device`

**Behavior to preserve:**
- `os-name` and `mac match` semantics, precedence (mac match wins over os-name) and doctor verdicts.

**Behavior to change:**
- A PCI slot selector on ethernet entries, resolved by the shared resolver, the apply and the doctor.
- A doctor verdict for "mac match unbound, unclaimed NIC in the slot its card last used", which needs a record of the last slot each matched card was bound in.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Shared resolver and selectors | `internal/component/iface/resolve.go`, `config_apply.go` `bindDevices`; logical-name-resolution page | Binds a logical name by `os-name` or `mac match`, defers when absent, refuses ambiguity | Extends with a third selector. No second resolver |
| Doctor selector check | `doctor_linux.go` `checkEthernetInterfaces`, `doctor_selector.go` `SelectedNetDevice` | Reports unmatched and ambiguous selectors from the config tree and sysfs | Extends: PCI selector verdicts, and the replaced-card verdict |
| Host NIC inventory | `internal/component/host/nic_linux.go` | Reads PCI vendor and device ids, decides physical NICs | Extends or shares: the PCI address read (`device` symlink target) belongs beside it |
| VPP DPDK interface by PCI address | `docs/guide/vpp.md`, `vpp dpdk interface <pci-address>` | VPP already binds NICs by slot | Does not apply to the netlink backend; the design states how a PCI selector on a VPP-backed entry relates to it (likely refused as redundant, or mapped) |
| Runtime state store | `statestore`, `pkg/zefs/keys.go` (system door) | Persists daemon runtime state | Extends: the "card last seen in slot" record is runtime state and goes there, never a raw file |
| Late NIC re-apply | the fix spec named in Depends | A NIC that appears after boot gets its entry applied | Depends on it: a replacement card hot-plugged at runtime must bind without a restart |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: a slot selector on `interface ethernet <name>` (name settled at design, for example `pci match <domain:bus:device.function>`).
- Doctor: `ze doctor`, served by the daemon on every surface.

### Transformation Path
1. Verify: one entry carries at most one hardware selector (mac match or pci match), or the design states their combination; format check of the PCI address.
2. Resolve: list net devices whose `device` symlink resolves to that PCI function; one answer binds, none defers, several (multi-port function) refuses.
3. Apply: each bind of a `mac match` entry records card permanent MAC plus PCI address in the state store.
4. Doctor: for each unbound `mac match` entry, look up the recorded slot; if a physical NIC with no entry bound to it sits there, report it as a replaced card with the fix.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| iface to sysfs | read-only `/sys/class/net/<dev>/device` link | No |
| iface to state store | zefs key registered for the slot record | No |

### Changed-Wire Census

**Search evidence:** owed at design (doctor JSON codes, `show interface` selector field).

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| [format] | [role] | [source site] | [finding] | [disposition] |

### Integration Points
- `bindDevices`, `SelectedNetDevice`, the resolver cache invalidation on `created` events.

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
| A-1 | A PCI function with several ports (some multi-port NICs) exposes several net devices on one PCI address; the selector then needs a port index (`dev_port`) | Linux sysfs `dev_port` | Ambiguous binding | QEMU guest with a multi-queue or multi-function NIC | unvalidated |
| A-2 | A device with no permanent address (some modems, VFs) gets a random MAC per probe and must never be recorded as a card identity | VyOS T3871 `015663a31` | The doctor reports a replaced card every boot | Unit test with a device reporting no IFLA_PERM_ADDRESS | unvalidated |
| A-3 | Ze's kernel exposes no `/sys/bus/pci/slots` (no PCI hotplug drivers), so the selector is the PCI bus address, not a physical slot number | `plan/release/delivery/spec-docker-hosts-run-the-ze-kernel.md` notes `CONFIG_HOTPLUG_PCI_*` unset | Slot numbers would be preferable where present | Read the Ze kernel config | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A firmware or BIOS update renumbers PCI buses, so a slot selector binds the wrong port | Address moves to another cable | Doctor reports a slot record that disagrees with the card found there (AC-6) |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A configured address lands on another cable, which can leak traffic between networks |
| How is it reverted? | Remove the selector; mac match and os-name are unchanged |
| Who else touches this path? | Every resolver consumer (VRRP, DHCP server, BFD, PPPoE), the late NIC re-apply fix |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `interface ethernet uplink` with a PCI selector naming the guest NIC's address | → | resolver bind by PCI address in the apply | `test/plugin/iface-pci-selector-apply.ci` (`option=needs-linux`, QEMU guest NIC) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | PCI selector naming the address of the guest NIC | The entry's address and MTU land on that NIC, whatever its kernel name and MAC | None |
| AC-2 | The card in that slot replaced by one with another MAC (QEMU device swap) | The same entry binds to the new card with no config edit, and a WARN log plus `show warnings` say the card in the slot changed (VyOS `f5c844e8a`: warn at swap time, not only at boot) | None |
| AC-3 | PCI selector naming an empty slot | Commit succeeds, the entry is unbound, doctor warns `selector unmatched` naming the address | Deferred binding, as for mac match |
| AC-4 | `mac match` entry unbound, and an unclaimed physical NIC sits at the PCI address that card was last bound at | `ze doctor` reports a replaced card: entry name, slot, old MAC, new MAC, and the exact `set interface ethernet <name> mac match <new>` fix (or the PCI selector) | None |
| AC-5 | Same, but the NIC in that slot is bound to another entry | No replaced-card report: the NIC is claimed | None |
| AC-6 | Two known cards swapped slots (both MACs on record, each now in the other's slot) | Doctor reports the swap as a change, not as an ordinary match (VyOS `9f75f7bf6`) | Binding still follows the selector each entry uses |
| AC-7 | A device with no permanent address | Never recorded as a card identity; no replaced-card report for it (VyOS `015663a31`) | None |
| AC-8 | A card that is a bridge or bond member | Still recorded and discoverable in its slot (VyOS `ccac4ac56`) | None |
| AC-9 | Both mac match and the PCI selector on one entry | Behavior fixed at design: refused naming both, or both required to agree; never a silent precedence | None |
| AC-10 | PCI selector on a VPP-backed entry | Defined at design against `vpp dpdk interface`; never silently ignored | None |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Binds by slot and swaps a failed card | config, resolver, apply, swap event | `test/plugin/iface-pci-selector-apply.ci`, `iface-pci-selector-swap.ci` |
| 2 | Runs `ze doctor` after swapping a mac-matched card | doctor check over config and slot record | `test/plugin/iface-doctor-replaced-card.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPCISelectorParse` | `internal/component/iface/config_selector_test.go` | AC-9, address format | |
| `TestReplacedCardVerdict` | `internal/component/iface/doctor_linux_test.go` | AC-4, AC-5, AC-6, AC-7 over a sysfs fixture | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| PCI function | 0..7 | 7 | N/A | 8 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `iface-pci-selector-apply` | `test/plugin/` | AC-1, AC-3 | |
| `iface-pci-selector-swap` | `test/plugin/` | AC-2 | |
| `iface-doctor-replaced-card` | `test/plugin/` | AC-4 | |

## Files to Modify
- `internal/component/iface/yang/ze-iface-conf.yang` - the selector leaf
- `internal/component/iface/config.go`, `internal/component/iface/config_apply.go` - parse, bind
- `internal/component/iface/doctor_linux.go` - the new verdicts
- `docs/architecture/iface/logical-name-resolution.md` - the third selector and the slot record

## Files to Create
- `test/plugin/iface-pci-selector-apply.ci`, `test/plugin/iface-pci-selector-swap.ci`, `test/plugin/iface-doctor-replaced-card.ci`

## Implementation Steps

1. **Phase: Wiring** - leaf, parse, failing `.ci`.
2. **Phase: Resolver** - bind by PCI address in apply, resolver and `SelectedNetDevice`.
3. **Phase: Slot record** - state store key, written on each mac-match bind.
4. **Phase: Doctor** - replaced-card and swapped-card verdicts.

Proof: `./le test unit plugins`, `./le test functional plugin` for the named `.ci` files, `./le test qemu all`.

## VyOS reference

VyOS checkout `~/Code/github.com/vyos/vyos-1x`, read 2026-11-08 at `611550d23`. VyOS keeps an interface-name store keyed by slot and card (`python/vyos/ifconfig/ifname_store.py`, `src/system/vyos-net-name-resolve.py`).

| Ticket / commit | Lesson for Ze |
|-----------------|---------------|
| T3871, `035d0260f` "name a card by the card, not only by the slot it sits in" | A card moved to another slot keeps its name; only an unknown card in a slot on record is a replacement. AC-4, AC-5 |
| T3871, `9f75f7bf6` "report a name which changed hands between known cards" | Cards rearranged among themselves looked like nothing moved. AC-6 |
| T3871, `f5c844e8a` "warn about a replaced card when it is swapped, not only at boot" | A runtime swap was never reported. AC-2 |
| T3871, `015663a31` "stop reporting a modem as replaced hardware every boot" | A device with no permanent MAC gets a random one per probe. AC-7, A-2 |
| T3871, `ccac4ac56` "keep bridge and bond members discoverable" | Enslaved cards were skipped. AC-8 |

## Design Insights

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
- USB NICs have no PCI address; a USB path selector is not in this spec.

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
