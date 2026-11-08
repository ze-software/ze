# Spec: iface-ethernet-speed-duplex

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | config |
| Priority | P1 |
| Release reason | A shipped NOS that cannot force speed, duplex or autonegotiation cannot bring up a fibre or SFP link whose far end has autonegotiation off, and an operator cannot pin a copper link that flaps between rates. Ze ships ethernet management, so this is a configuration gap in a shipped capability (owner decision 2026-11-08: speed and duplex before the first release) |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

An operator must be able to set, per ethernet interface, the link speed, the
duplex and autonegotiation. Ze must refuse at verify a combination the NIC
cannot do, instead of accepting it and leaving the link down.

| # | Operator need | Example |
|---|---------------|---------|
| 1 | Force a fibre or SFP port to a fixed BASE-X mode with autonegotiation off, to match a provider handoff that has it off | 1000BASE-X full, autonegotiation off |
| 2 | Keep autonegotiation on but advertise one rate only, which 1000BASE-T and faster copper need (IEEE 802.3 clause 40 makes autonegotiation mandatory for 1000BASE-T) | speed 1000, autonegotiation on |
| 3 | Force 10 or 100 Mbit/s, half or full duplex, for old copper equipment | speed 100, duplex full, autonegotiation off |
| 4 | Return to the NIC default when the config is removed | delete the link settings |
| 5 | See the configured and the negotiated values side by side | `show interface eth0` |

Thomas (2026-11-08): a spec for every interface feature Ze lacks that VyOS has,
all after the first release, except speed and duplex, which he wants before the
first release.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/iface/offload.md` - the existing ethtool path in the iface component
  → Constraint: ethtool ioctls bypass `Backend` deliberately, and take the device the apply's selector resolution produced, never the logical name
- [ ] `docs/architecture/iface/logical-name-resolution.md` - selector binding and unbound entries
  → Constraint: an entry whose selector names no present device is UNBOUND, and every phase skips it. Link settings follow the same rule
- [ ] `docs/architecture/core-design.md` section 14a - the `ze:backend` capability gate
  → Constraint: a backend that cannot apply the setting refuses at commit with the YANG path (exact or reject)

**Key insights:**
- Ze reads speed and duplex today (sysfs) and writes neither.
- VyOS warns and skips an unsupported speed. Ze refuses instead (`ai/rationale/exact-or-reject.md`).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/iface/yang/ze-iface-conf.yang` - `list ethernet` uses `interface-l2` (mac, offload) and `interface-unit`. No speed, duplex or autonegotiation leaf exists in the module
- [ ] `internal/component/iface/backend.go` - `Backend.LinkSpeedDuplex` is read-only, documented as best-effort enrichment. No setter exists
- [ ] `internal/plugins/iface/netlink/show_linux.go` - `LinkSpeedDuplex` reads `/sys/class/net/<dev>/speed` and `duplex`; `parseLinkSpeedDuplex` maps a non-positive or garbage speed to 0. Consumers: flow export and OSPF auto-cost
- [ ] `internal/plugins/iface/vpp/query.go` - `LinkSpeedDuplex` answers `(0, "")`, because a VPP interface has no sysfs entry
- [ ] `internal/component/iface/offload_linux.go` - `applyOffloads` opens one AF_INET socket and makes one `SIOCETHTOOL` call per configured leaf. A failure is only logged as a warning, and the commit succeeds
- [ ] `internal/component/iface/config_apply.go` - the Phase 2 entry loop calls `applyOffloads(device, e.Offload)` on the resolved device, after MTU and MAC
- [ ] `internal/component/iface/config.go` - `ifaceEntry` with `Offload *offloadConfig`, tri-state pointer fields
- [ ] `internal/component/iface/register.go` - `verifyIfaceConfig`, `parseAndVerifyIfaceSections`: backend gate, `validateUniqueMatchMAC`, VPP QoS map checks
- [ ] `internal/component/host/nic_linux.go` - `show host nic` reads speed and duplex from sysfs only when the carrier is up

**Behavior to preserve:**
- An ethernet entry without the new leaves makes no link-settings call: an absent leaf keeps the OS default, as for offloads.
- `LinkSpeedDuplex` keeps its contract for its two consumers.

**Behavior to change:**
- Ethernet entries gain speed, duplex and autonegotiation leaves, applied through `ETHTOOL_SLINKSETTINGS` on the resolved device.
- Verify refuses a mode the bound NIC does not list among its supported link modes, and refuses the leaves on a backend that cannot apply them.
- A link-settings write that fails is an apply error, never only a log line (the offload path's warn-and-continue is not copied).

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Offload ethtool path | `offload_linux.go` `applyOffloads`, `ethtoolIoctl`; `docs/architecture/iface/offload.md` | Sets GRO, GSO, TSO, LRO, SG and hw-tc-offload by `SIOCETHTOOL` on the resolved device; an absent leaf makes no call | Extends: link settings use the same socket pattern and the same per-entry loop, with the tri-state rule. Does not copy its warn-only failure handling |
| Speed and duplex read | `Backend.LinkSpeedDuplex`, netlink `show_linux.go` | Reads the negotiated speed and duplex from sysfs | Extends: `show interface` reports configured beside negotiated, and the read stays the source of the negotiated value |
| Host NIC inventory | `internal/component/host/nic_linux.go`, `ethtool_linux.go` | Reports driver, PCI ids, speed, duplex and rings | Does not apply to config. Reference for the ioctl pattern only |
| Backend capability gate | `ze:backend` and `config.validateBackendFeatures` (`core-design.md` 14a), `validateBackendGate` in `register.go` | Refuses at commit a node the selected backend cannot implement | Extends: the new leaves carry `ze:backend "netlink"` unless A-2 finds a VPP mechanism |
| NIC-capability validation | `plan/release/fixes/spec-config-bound-authority.md` (VPP queue counts read by `ETHTOOL_GCHANNELS`) | Design: validate reads a NIC maximum, refuses above it, warns when it cannot read | Extends the same pattern: verify reads `ETHTOOL_GLINKSETTINGS` supported modes. An unreadable or unbound NIC does not refuse at commit |
| Selector binding | `config_apply.go` `bindDevices`, `deviceFor` | Resolves `os-name` and `mac match` once per apply | Extends: link settings apply on the bound device. An unbound entry is skipped and applied when the device appears |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: link settings on `interface ethernet <name>` (leaf names settled in design; VyOS spells `speed` and `duplex` at interface level).

### Transformation Path
1. YANG parse. Enums bound the values: speed auto, 10, 100, 1000, 2500, 5000, 10000, 25000, 40000, 50000, 100000, 200000, 400000; duplex auto, half, full.
2. `OnConfigVerify`: structural checks (speed and duplex both auto or both fixed; half duplex only at 10 and 100), then, for a bound device, read the supported link modes and refuse a mode not listed.
3. `OnConfigApply` Phase 2 entry loop: read the current settings on the resolved device, and write only when they differ.
4. Removal: autonegotiation on, every supported mode advertised.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config to iface plugin | Stage 2 config JSON, string values | No |
| iface to kernel | `SIOCETHTOOL` ioctl on the resolved device | No |

### Changed-Wire Census

**Search evidence:** no protocol or RPC wire changes. The `show interface` answer gains fields; its census is owed at design.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| `show interface` JSON | writer | the iface show handler (to locate at design) | to read at design | adds fields, keeps existing keys |

### Integration Points
- The Phase 2 entry loop in `config_apply.go`, beside `applyOffloads`.

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
| A-1 | `ETHTOOL_GLINKSETTINGS` reports BASE-X, BASE-KX4, BASE-SR and BASE-T modes as separate bits, so the supported-mode check must match speed plus duplex over every mode family | VyOS T9353 (`41e64e809`): a check that matched only `<speed>baset/<duplex>` refused every SFP BASE-X mode | Fibre ports refused | Unit test over a bitmap holding only 1000baseX/Full and 2500baseX/Full | unvalidated |
| A-2 | The VPP backend has no per-interface API to force speed, duplex or autonegotiation on a DPDK port | `internal/plugins/iface/vpp/query.go` reads no speed at all | VPP could support it, and refusing would be wrong | Read VPP's dpdk plugin and interface API source and quote the deciding function | unvalidated |
| A-3 | A QEMU guest NIC model in Ze's kernel accepts `ETHTOOL_SLINKSETTINGS` | none yet | The functional proof needs another NIC model or a netdevsim route | Run the ioctl in the `./le test qemu` guest | unvalidated |
| A-4 | Some drivers expose no link-mode control (vmxnet3, virtio_net, iavf, ice, i40e, hv_netvsc, veth, ixgbevf, tun) | VyOS `_drivers_without_speed_duplex_flow` in `python/vyos/ethtool.py` | The verify read must treat an empty supported set as "no forced mode possible" | Unit test with an empty bitmap | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Forcing a mode on the management NIC cuts the operator off | Link down after commit | `ze doctor` reports a forced mode with no carrier; commit confirm restores on timeout |
| R-2 | A NIC not bound at verify cannot be checked | `mac match` unbound | Accept, apply when bound, and raise `show warnings` naming the leaf and the supported modes when the NIC cannot do it |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A link forced to a wrong mode stays down, and every session over it drops |
| How is it reverted? | Remove the leaves: removal restores autonegotiation with every supported mode |
| Who else touches this path? | The iface fix specs written 2026-11-08 in `plan/release/fixes/` (offload refusal, late or hot-plugged NIC re-apply) change the same Phase 2 entry loop |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `interface ethernet eth0` with speed 100, duplex full, autonegotiation off, committed | → | link-settings apply in the Phase 2 entry loop | `test/plugin/iface-ethernet-speed-duplex-apply.ci` (`option=needs-linux`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | speed 100, duplex full, autonegotiation off, on a NIC that supports 100baseT/Full | `ETHTOOL_GLINKSETTINGS` reads back speed 100, duplex full, autonegotiation disabled | None |
| AC-2 | speed 1000, autonegotiation on, on a BASE-T NIC | Autonegotiation on; the advertised modes are exactly the 1000 Mbit/s modes the NIC supports | None |
| AC-3 | speed 1000, duplex full, autonegotiation off, NIC lists only 1000baseX/Full (SFP) | Accepted at verify and applied (VyOS T9353 regression) | None |
| AC-4 | speed 2500 on a NIC whose supported modes stop at 1000 | Verify refuses, naming the interface, the leaf, the value, the modes the NIC supports and the fix | Running config unchanged |
| AC-5 | speed 1000, autonegotiation off, on a NIC that lists only 1000baseT modes | Verify refuses: 1000BASE-T requires autonegotiation; the message says to set autonegotiation on | None |
| AC-6 | speed fixed with duplex auto, or speed auto with duplex fixed | Verify refuses, naming both leaves | None |
| AC-7 | duplex half with speed 1000 or more | Refused, naming the leaf | None |
| AC-8 | The same leaves with `backend vpp` | Commit refused with the YANG path and the backend name (if A-2 finds a mechanism, AC-1 to AC-6 hold on VPP instead) | None |
| AC-9 | Link leaves removed from an entry | Autonegotiation on, every supported mode advertised | None |
| AC-10 | Entry bound by `mac match` whose NIC is absent at commit | Commit succeeds. When the NIC appears the settings apply on it. A mode it cannot do raises `show warnings` naming the leaf and the supported modes | No refusal at commit, because no NIC was readable |
| AC-11 | Entry with no link leaves | No `ETHTOOL_SLINKSETTINGS` call | None |
| AC-12 | Re-commit with unchanged settings | No write, so the link does not renegotiate | None |
| AC-13 | `show interface eth0` | Shows configured speed, duplex and autonegotiation, and the negotiated values | None |
| AC-14 | Forced mode, no carrier past the doctor threshold | `ze doctor` warns, naming the forced mode as a likely mismatch and the command that returns it to auto | None |
| AC-15 | Driver reports no link modes (A-4) and a forced mode is set | Verify refuses, saying the driver cannot change link settings | None |
| AC-16 | The kernel refuses the write at apply | The apply fails for that entry with the device, the values and the kernel error; it is never only a log line | None |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Forces an SFP port to 1000 full, autonegotiation off | config, verify, Phase 2 loop, ioctl | `test/plugin/iface-ethernet-speed-duplex-apply.ci` |
| 2 | Asks for a mode the NIC cannot do | config, verify refusal | `test/plugin/iface-ethernet-speed-unsupported.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestLinkModeSupportedAcrossFamilies` | `internal/component/iface/linksettings_linux_test.go` | AC-3, A-1 | |
| `TestLinkSettingsStructuralVerify` | `internal/component/iface/linksettings_test.go` | AC-5, AC-6, AC-7, AC-15 | |
| `TestLinkSettingsNoWriteWhenEqual` | same | AC-11, AC-12 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| speed | enumeration | 400000 | N/A | any value not in the enumeration |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `iface-ethernet-speed-duplex-apply` | `test/plugin/` | AC-1, AC-9, AC-12 in the QEMU guest | |
| `iface-ethernet-speed-unsupported` | `test/plugin/` | AC-4 refusal text | |
| `iface-ethernet-speed-vpp-refused` | `test/vpp/` | AC-8 | |

### Interop Tests (Scope: protocol)
N-A: no protocol peer. Link negotiation is IEEE 802.3 and is proven against the guest NIC model.

## Files to Modify
- `internal/component/iface/yang/ze-iface-conf.yang` - link leaves on `list ethernet`
- `internal/component/iface/config.go` - parse the leaves into `ifaceEntry`
- `internal/component/iface/config_apply.go` - the Phase 2 call on the resolved device
- `internal/component/iface/register.go` - the verify checks
- `docs/architecture/iface/offload.md` (or a new link-settings page), `docs/guide/configuration.md`

## Files to Create
- `internal/component/iface/linksettings_linux.go`, `internal/component/iface/linksettings_other.go`
- `test/plugin/iface-ethernet-speed-duplex-apply.ci`, `test/plugin/iface-ethernet-speed-unsupported.ci`, `test/vpp/iface-ethernet-speed-vpp-refused.ci`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `ze-iface-conf.yang` |
| YANG validation constraints | Yes | enumerations for speed and duplex |
| YANG custom validators | Yes | the supported-mode check needs the NIC |
| CLI commands/flags | No | config only |
| CLI grammar (keyword before value) | N-A | no command |
| Editor autocomplete | Yes | automatic for enumerations |
| Functional test for new RPC/API | N-A | no RPC |
| Pipe completeness | N-A | no command |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | Yes | AC-14 |
| Prometheus counters/metrics | No | `ze_host_nic_link_speed_mbps` exists |
| BGP family surface | N-A | |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `features/<id>.md` |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` |
| 3-17 | Remaining rows | to answer at design | |

## Implementation Steps

1. **Phase: Wiring** - leaves, parse, failing `.ci`.
2. **Phase: Verify** - structural and supported-mode checks.
3. **Phase: Apply and removal** - write, no write when equal, restore.
4. **Phase: Show and doctor**.

## VyOS reference

VyOS checkout `~/Code/github.com/vyos/vyos-1x`, read 2026-11-08 at `611550d23`.

| Ticket / commit | Lesson for Ze |
|-----------------|---------------|
| T9353, `41e64e809` "ethtool: fix BASEX mode(s) support in check_speed_duplex()" | The supported-mode check matched only `<speed>baset/<duplex>`, so every SFP BASE-X mode was refused. AC-3, A-1 |
| `python/vyos/ifconfig/ethernet.py` `set_speed_duplex` | VyOS warns and returns when the driver cannot change speed. Ze refuses at verify (AC-4, AC-15) |
| `python/vyos/ethtool.py` `_drivers_without_speed_duplex_flow` | Drivers with no link-mode control (A-4) |

## Design Insights

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Refuse an unsupported mode at verify | Warn and skip (VyOS) | Exact or reject: an accepted setting that does nothing is a silent wrong value |

## Known Limitations
- Flow control (pause frames) and FEC are separate VyOS options, not in this spec. Each is its own post-release spec when wanted.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)
