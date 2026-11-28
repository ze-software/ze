# Spec: single-owner-reload-reconcile-2-iface-reconcile

| Field | Value |
|-------|-------|
| Status | design |
| Scope | config |
| Priority | P1 |
| Release reason | Interface reconciliation acts on stale state in three paths: a registry change reconciles against an `activeCfg` that VPP refused, the 1 Hz carrier resync repairs against a snapshot older than the link worker's last applied transition, and DHCPv6 DAD mistakes an undelivered down-and-up flap for a duplicate address (false RFC 9915 Decline). |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Parent umbrella: `plan/release/fixes/spec-single-owner-reload-reconcile.md` (Family S3, part 2 of 3).

## Task

**Root cause.** Three interface reconcile paths judge live state against stale or unordered snapshots instead of event-ordered state owned by the apply/link path:

1. `reconcileOnRegistryChange` (`internal/component/iface/config_apply.go`) loads `activeCfg` and runs `reconcileOnReady` without asking `applyIsOwed()`. While VPP refused the last apply and left it owed, `activeCfg` holds a config the dataplane never received, so every address-registry event targets devices VPP never created and logs a warning per address.
2. `resyncCarrierState` (`internal/component/iface/link_queue.go`) compares acted-on route metric state against a carrier snapshot the rate tracker took on its 1 Hz tick (`SubscribeCollectNotify` in `register.go`). A carrier transition processed by the link worker after the snapshot was taken looks like a contradiction, causing a transient route metric move and a false increment of `ze_iface_carrier_resyncs_total`.
3. `goneWhileUp` (`internal/plugins/iface/dhcp/dhcp_v6_attach_linux.go`) judges an address missing during DAD by calling `iface.ReadAddressState` a second time and draining `w.links`. A down-and-up transition between the two reads whose netlink monitor events have not yet reached `w.links` leaves both reads `up` with nothing to drain, so a unique address deleted by the kernel on link-down is falsely Declined under RFC 9915.

In addition, AC-5 of the umbrella requires confirming that learned addresses (DHCP, SLAAC, IPv6 link-local, and VPP-shaped zero-lifetime registered leases) are never removed by a commit reconcile. Kernel DHCP/SLAAC/link-local preservation is already landed at HEAD (`learnedAddrSet`, `RegisterLeasedAddresses`, `TestIntegrationReconcileKeepsLearnedAddresses`); this spec adds explicit unit coverage for VPP-shaped zero-lifetime leased addresses and link-locals (`ValidLifetime == 0`, since VPP's `ip_address_details` carries no kernel-style lifetime per `docs/features/interfaces.md` note `[2]`).

**Goal.**

1. Guard `reconcileOnRegistryChange` with `applyIsOwed()`: while an apply is owed, skip the registry reconcile and touch no address until `applyOwedConfig` succeeds (which then reconciles the full desired state including `ownedAddresses()`).
2. Order carrier resync snapshots against link-worker transitions: capture a monotonic carrier sequence before the rate tracker's `ListInterfaces()` dump and attach it to `linkEventResync`; in `linkEventQueue`, record each interface's last applied carrier sequence and skip any interface in `resyncCarrierState` whose carrier was updated at or after the snapshot sequence (or still has a pending carrier event in the queue).
3. Surface Linux `IFLA_CARRIER_CHANGES` (`unix.IFLA_CARRIER_CHANGES`) from `counterSource.consume` (`internal/plugins/iface/netlink/counter_linux.go`) onto `iface.InterfaceInfo` and `iface.AddressState`, and have `v6Worker` (`internal/plugins/iface/dhcp/dhcp_v6_attach_linux.go`) record `CarrierChanges` at address install and compare it on DAD reads (`goneDuplicates` / `goneWhileUp`). Any increase in `CarrierChanges` proves an undelivered link flap occurred, routing to `noteMove()` instead of Declining.
4. Add unit coverage proving VPP-shaped `AddrInfo` (zero `ValidLifetime`) spares registered leased addresses (`RegisterLeasedAddresses`) and kernel IPv6 link-locals across commits.

**Why it matters for shipping.** A VPP router with an owed apply stops spamming address errors against absent devices; `ze_iface_carrier_resyncs_total` stops false-firing during link flaps; and a DHCPv6 client on a flapping WAN link stops falsely Declining its own unique lease under RFC 9915.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - platform contracts, boundaries
- [ ] `ai/doors/forwarding.md` - iface ownership, VPP backend limits, traps
- [ ] `ai/doors/le-tooling.md` - Linux integration test conventions (`//go:build linux`, `userns.Enter`)
- [ ] `docs/features/interfaces.md` - learned addresses, VPP address lifetimes note `[2]`, `counter-generation`, owed apply
- [ ] `docs/architecture/iface/management.md` - carrier event queue and resync design

## Current Behavior (MANDATORY)

Verified at the producers on 2026-11-26.

**Source files read:**
- [ ] `internal/component/iface/config_apply.go` - `reconcileOnRegistryChange` (lines 2507-2527) calls `reconcileOnReady(cfg, b)` without checking `applyIsOwed()`. `learnedAddrSet` (lines 199-239) and `reconcileOnReadyWithJournal` (lines 1822-1836) already spare `ValidLifetime > 0`, `leasedAddressSet()`, and unconfigured `fe80::`.
- [ ] `internal/component/iface/link_queue.go` - `pushResync` (line 200) and `resyncCarrierState` (lines 367-400) compare `value.carrier` against acted-on metric state with no causal sequence against carrier events handled by the worker.
- [ ] `internal/component/iface/register.go` - `SubscribeCollectNotify` (line 555) passes `carrierFromInterfaces(ifs)` to `linkQueue.pushResync`.
- [ ] `internal/component/iface/iface.go` & `resolve.go` - `InterfaceInfo` and `AddressState` do not carry `CarrierChanges` / `HasCarrierChanges`.
- [ ] `internal/plugins/iface/netlink/counter_linux.go` & `show_linux.go` - `counterSource.consume` already parses raw `RTM_GETLINK` attributes (`bridgeSTPFromAttrs`), and `unix.IFLA_CARRIER_CHANGES` (`0x23`) is available in `golang.org/x/sys/unix`.
- [ ] `internal/plugins/iface/dhcp/dhcp_v6_attach_linux.go` - `goneWhileUp` (lines 503-522) checks `later.State` and `w.drainLinks()`, missing an undelivered down-and-up transition between reads.
- [ ] `internal/plugins/iface/vpp/query.go` - `fillAddresses` / `addrInfoOf` (lines 260-301) reads `ip.IPAddressDetails`, which carries no lifetime field; VPP has no kernel-style address lifetimes (`docs/features/interfaces.md` note `[2]`).

**Behavior to preserve:**
- `reconcileOnVPPConnected` / `reconcileOnVPPReady` still runs `applyOwedConfig` when VPP connects, and that pass reconciles `ownedAddresses()` automatically.
- `resyncCarrierState` still repairs genuine missed events or failed `AddRoute` calls (where no newer carrier event superseded the snapshot) and still increments `ze_iface_carrier_resyncs_total` once per genuine repair.
- `goneWhileUp` still Declines genuine DAD failures where `CarrierChanges` did not advance and no link move occurred.
- On backends that do not report `CarrierChanges` (`HasCarrierChanges == false`), `goneWhileUp` preserves its existing two-read fallback.

**Behavior to change:**
- `reconcileOnRegistryChange` returns early when `applyIsOwed()` is true.
- `linkEventQueue` tracks per-interface carrier sequence numbers and `resyncCarrierState` skips interfaces whose carrier state was applied or queued at or after the snapshot's sequence.
- `ifacenetlink` parses `IFLA_CARRIER_CHANGES` into `InterfaceInfo.CarrierChanges` + `HasCarrierChanges`, `ReadAddressState` copies them into `AddressState`, and `v6Worker` compares `CarrierChanges` across install and DAD reads to detect undelivered down+up transitions.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| iface owed-apply record | `internal/component/iface/config_apply.go` `applyIsOwed`, `applyOwedConfig`, `owedApply` | Defers an apply refused by a not-ready VPP backend and replays it on VPP connect; read by `register.go` but not `reconcileOnRegistryChange` | Extends: `reconcileOnRegistryChange` checks `applyIsOwed()` before calling `reconcileOnReady` |
| Learned-address and leased-address sparing | `internal/component/iface/config_apply.go` `learnedAddrSet`, `addrLearned`, `reconcileOnReadyWithJournal`; `internal/component/iface/address_owner.go` `RegisterLeasedAddresses`, `leasedAddressSet` | Spares finite-lifetime addresses (SLAAC, kernel DHCP), `RegisterLeasedAddresses` entries (finite or infinite, backend-agnostic), and unconfigured IPv6 link-locals; proven on kernel by `TestIntegrationReconcileKeepsLearnedAddresses` | Extends: adds unit proof for VPP-shaped zero-lifetime `AddrInfo` with `RegisterLeasedAddresses` and `fe80::` (VPP carries no kernel-style lifetime in `ip_address_details`) |
| Carrier event queue and 1 Hz resync | `internal/component/iface/link_queue.go` `linkEventQueue`, `pushCarrier`, `pushResync`, `resyncCarrierState`; `internal/component/iface/rate.go` `collect`, `SubscribeCollectNotify` | Coalesces carrier/router events per key and runs a 1 Hz resync from the rate tracker's interface dump | Extends: stamps each resync snapshot with a monotonic sequence taken before `ListInterfaces()` and skips any interface whose worker-applied carrier sequence is at or above that stamp |
| Raw `RTM_GETLINK` attribute reader | `internal/plugins/iface/netlink/counter_linux.go` `counterSource.consume`, `bridgeSTPFromAttrs`; `show_linux.go` `ListInterfaces`, `GetInterface` | Reads `RTM_GETLINK` attributes directly from the kernel socket alongside `LinkDeserialize` | Extends: extracts `unix.IFLA_CARRIER_CHANGES` (`uint32`) from the same `RTM_GETLINK` message with zero extra syscalls |
| DHCPv6 DAD address state read | `internal/component/iface/resolve.go` `AddressState`, `ReadAddressState`; `internal/plugins/iface/dhcp/dhcp_v6_attach_linux.go` `v6Worker.goneDuplicates`, `goneWhileUp` | Reads addresses and link state twice and drains `w.links` before Declining a gone address | Extends: carries `CarrierChanges` + `HasCarrierChanges` on `AddressState` and treats an advanced counter between install and DAD read (or between the two reads) as a link move rather than a duplicate |

## Data Flow (MANDATORY)

### Entry Point
- `RegisterOwnedAddresses` / `UnregisterOwnedAddresses` triggering `reconcileOnRegistryChange`.
- Rate tracker `collect()` tick triggering `SubscribeCollectNotify` -> `linkQueue.pushResync`.
- DHCPv6 `v6Worker` DAD poll calling `iface.ReadAddressState` -> `goneDuplicates` -> `goneWhileUp`.

### Transformation Path
1. **Owed-apply guard:** `reconcileOnRegistryChange` checks `if applyIsOwed() { return }` before loading/reconciling `activeCfg`. When VPP later connects, `applyOwedConfig` applies the deferred config and reconciles the live `ownedAddresses()` snapshot in one pass.
2. **Event-ordered carrier resync:** `linkEventQueue` owns a monotonic `carrierSeq atomic.Uint64` incremented inside `pushCarrier` (and recorded on the queue entry). Before `rate.go` calls `b.ListInterfaces()`, the rate tracker reads the current carrier-epoch stamp (via a callback or `linkEventQueue.snapshotEpoch()`). When the worker drains `linkEventCarrier`, it records `lastAppliedSeq[ifaceName] = entry.seq`. When `resyncCarrierState` runs for a `linkEventResync` stamped with `snapSeq`, it skips any interface where `lastAppliedSeq[ifaceName] > snapSeq` or where a newer `linkEventCarrier` for `ifaceName` is currently pending in `q.pending`. Only a genuine contradiction whose link state predates the snapshot is repaired and counted in `ze_iface_carrier_resyncs_total`.
3. **Link change counter on DHCPv6 DAD:** In `counterSource.consume`, a helper `carrierChangesFromAttrs(message.Data[unix.SizeofIfInfomsg:])` parses `unix.IFLA_CARRIER_CHANGES` (`uint32`, 4-byte native-endian payload) into `counterLink.carrierChanges` and `counterLink.hasCarrierChanges`. `show_linux.go` copies them onto `InterfaceInfo.CarrierChanges` / `HasCarrierChanges`, and `ReadAddressState` copies them onto `AddressState`. When `v6Worker` installs addresses, it records `w.installCarrierChanges` / `w.hasInstallCarrierChanges` from `ReadAddressState`. In `goneDuplicates` / `goneWhileUp`, if `HasCarrierChanges` is true and either `read.CarrierChanges != w.installCarrierChanges` (when recorded) or `later.CarrierChanges != read.CarrierChanges`, `v6Worker` calls `w.noteMove()` and returns `nil` (no Decline).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| `iface` component <-> `ifacenetlink` backend | `InterfaceInfo.CarrierChanges` and `HasCarrierChanges` Go struct fields (not serialized on wire unless JSON-tagged `omitempty`, internal backend read) | Yes, same pattern as `BridgeSTP` and `CounterGeneration` |
| `iface` component <-> `ifacedhcp` plugin | `iface.AddressState.CarrierChanges` and `HasCarrierChanges` (in-process; `ifacedhcp` already requires internal mode per `internal/plugins/iface/dhcp/register.go`) | Yes |

### Changed-Wire Census

No network protocol wire format, RPC message, EventBus payload, YANG leaf, or persisted state format changes in this spec.

**Search evidence:** `InterfaceInfo.CarrierChanges` / `HasCarrierChanges` and `AddressState.CarrierChanges` / `HasCarrierChanges` are in-process Go fields carrying kernel `IFLA_CARRIER_CHANGES` from `ifacenetlink` to `ifacedhcp` (`json:"-"` on `InterfaceInfo`, matching `AddressesRead` and `DADReported`). `reconcileOnRegistryChange` and `linkEventQueue` are internal to `internal/component/iface`.

### Integration Points
- `internal/component/iface/config_apply.go`: `reconcileOnRegistryChange` guard.
- `internal/component/iface/link_queue.go`, `rate.go`, `register.go`: monotonic carrier snapshot sequence.
- `internal/component/iface/iface.go`, `resolve.go`: `CarrierChanges` / `HasCarrierChanges` fields.
- `internal/plugins/iface/netlink/counter_linux.go`, `show_linux.go`: `IFLA_CARRIER_CHANGES` attribute parse.
- `internal/plugins/iface/dhcp/dhcp_v6_attach_linux.go`: `CarrierChanges` check in `v6Worker`.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | `IFLA_CARRIER_CHANGES` rides the existing `counterSource` `RTM_GETLINK` path and `ReadAddressState` |
| No unintended coupling | Yes | `ifacedhcp` reads `iface.AddressState`, never importing `ifacenetlink` |
| No duplicated functionality | Yes | Reuses `applyIsOwed()`, `counterSource.consume`, and `RegisterLeasedAddresses` |
| Zero-copy preserved where applicable | Yes | Single `uint32` attribute read during existing `RTM_GETLINK` parse; zero extra syscalls |
| Registration over hardcoding, outbound | Yes | No backend name switches; `HasCarrierChanges` flag lets any backend opt in cleanly |
| Registration over hardcoding, inbound | Yes | No interface-name or backend hardcoding |

## Wiring Test (MANDATORY -- NOT deferrable)
| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| `RegisterOwnedAddresses` while `applyIsOwed()` is true | -> | `reconcileOnRegistryChange` early return | `TestRegistryReconcileWaitsForOwedApply` (`internal/component/iface/config_apply_deferred_test.go`) |
| `SubscribeCollectNotify` racing `pushCarrier` on `linkEventQueue` | -> | `resyncCarrierState` snapshot-sequence guard | `TestCarrierResyncSkipsInterfaceUpdatedAfterSnapshot` (`internal/component/iface/link_queue_test.go`) |
| `v6Worker` DAD poll after undelivered down+up (`IFLA_CARRIER_CHANGES` incremented) | -> | `goneDuplicates` / `goneWhileUp` carrier-change check | `TestDHCPv6UndeliveredDownUpNotDeclined` (`internal/plugins/iface/dhcp/rfc9915_dhcp_v6_attach_linux_test.go`) |

## Acceptance Criteria
| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | VPP refuses an apply (leaving `applyIsOwed() == true`), then an owned-address registry event triggers `reconcileOnRegistryChange` | `reconcileOnRegistryChange` returns immediately leaving the owed-apply record intact and issuing zero backend calls; once VPP connects and `applyOwedConfig` succeeds, that pass applies the deferred config together with the current `ownedAddresses()` snapshot | Registry-triggered reconciles when `applyIsOwed() == false` remain permitted to run `reconcileOnReady` immediately because the dataplane already holds `activeCfg` |
| AC-2 | DHCP, SLAAC, IPv6 link-local, and VPP-shaped zero-lifetime registered leased addresses (`ValidLifetime == 0` via `RegisterLeasedAddresses`) exist on an enabled unit and a commit runs | The commit keeps every DHCP, SLAAC, unconfigured IPv6 link-local, and registered leased address (including zero-lifetime VPP-shaped leases) on the unit while removing any dropped configured address | Learned addresses (finite-lifetime DHCP/SLAAC), unconfigured `fe80::` link-locals, and `RegisterLeasedAddresses` entries remain permitted on the unit because Ze owns only configured addresses; dropped configured addresses have no permitted survivors |
| AC-3 | A carrier event is queued or applied by `linkEventQueue` after the rate tracker began its `ListInterfaces()` snapshot | `resyncCarrierState` skips that interface, keeps the worker-applied route metric and `ze_iface_carrier_resyncs_total` unchanged, and repairs and increments `ze_iface_carrier_resyncs_total` once for any genuine missed carrier transition predating the snapshot | Genuine carrier contradictions whose last worker-applied carrier sequence predates the snapshot sequence (with no newer pending carrier event) remain permitted for resync repair so missed netlink events and failed `AddRoute` calls self-heal |
| AC-4 | On Linux, an interface goes down and back up between `v6Worker` address installation (or first `ReadAddressState`) and the second `ReadAddressState` with netlink monitor events undelivered | `InterfaceInfo.CarrierChanges` and `AddressState.CarrierChanges` increase across the reads with `HasCarrierChanges == true`, and `v6Worker` calls `noteMove()` to treat the missing address as a link move rather than sending a DHCPv6 Decline (RFC 9915) | Backends that do not report `IFLA_CARRIER_CHANGES` (`HasCarrierChanges == false`) remain permitted to use the existing two-read `goneWhileUp` fallback because they expose no carrier-change counter |
| AC-5 | On a genuine duplicate address where `IFLA_CARRIER_CHANGES` is unchanged and the link stays up | `v6Worker` sends the RFC 9915 DHCPv6 Decline for the duplicate address | None for genuine DAD failures on a stable link; every confirmed duplicate address must be Declined under RFC 9915 |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRegistryReconcileWaitsForOwedApply` | `internal/component/iface/config_apply_deferred_test.go` | AC-1 | |
| `TestLearnedAddrSetVPPZeroLifetimeRegisteredLease` | `internal/component/iface/reconcile_learned_test.go` | AC-2 | |
| `TestCarrierResyncSkipsInterfaceUpdatedAfterSnapshot` | `internal/component/iface/link_queue_test.go` | AC-3 | |
| `TestCarrierChangesFromAttrs` | `internal/plugins/iface/netlink/counter_linux_test.go` | AC-4 | |
| `TestDHCPv6UndeliveredDownUpNotDeclined` | `internal/plugins/iface/dhcp/rfc9915_dhcp_v6_attach_linux_test.go` | AC-4, AC-5 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `IFLA_CARRIER_CHANGES` (`uint32`) | `0..4294967295` (equality comparison `!=` handles uint32 wrap) | `0xFFFFFFFF` -> wraps to `0`, still `!=` | N/A (unsigned) | N/A (truncated attribute < 4 bytes ignored with `HasCarrierChanges = false`) |

### Functional / Integration Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `TestIntegrationCarrierChangesReadOnLinkFlap` (`//go:build linux`, `userns.Enter`) | `internal/plugins/iface/netlink/counter_integration_linux_test.go` | Real Linux veth down+up increments `InterfaceInfo.CarrierChanges` read through `GetInterface` / `ReadAddressState` (AC-4) | |
| `iface-link-flap-during-commit` (existing QEMU `.ci`) | `test/plugin/iface-link-flap-during-commit.ci` | Proves `ze_iface_carrier_resyncs_total` stays 0 across 6 commit-overlapping flap rounds (AC-3) | |

### Interop Tests (Scope: config)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A | - | - | Host kernel / local reconciler correctness only; no wire change to DHCPv6 message encoding | |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- add failing tests `TestRegistryReconcileWaitsForOwedApply`, `TestCarrierResyncSkipsInterfaceUpdatedAfterSnapshot`, `TestLearnedAddrSetVPPZeroLifetimeRegisteredLease`, and `TestDHCPv6UndeliveredDownUpNotDeclined`.
2. **Phase: Owed-apply guard and VPP learned-address proof** -- add `if applyIsOwed() { return }` in `reconcileOnRegistryChange` and verify AC-1 and AC-2.
3. **Phase: Event-ordered carrier resync** -- add monotonic sequence stamping on `linkEventQueue` and `rate.go` `collect()`, and skip interfaces updated at/after the snapshot sequence in `resyncCarrierState` (AC-3).
4. **Phase: `IFLA_CARRIER_CHANGES` and DHCPv6 `goneWhileUp`** -- parse `unix.IFLA_CARRIER_CHANGES` in `counter_linux.go`, expose `CarrierChanges`/`HasCarrierChanges` on `InterfaceInfo` and `AddressState`, and check it in `v6Worker` (`dhcp_v6_attach_linux.go`) with Linux `userns.Enter` integration test (AC-4, AC-5).
5. **Phase: Docs** -- update `docs/architecture/iface/management.md` ("Carrier events are queued per interface, never dropped") and `docs/features/interfaces.md` (`ReadAddressState` / DAD flap evidence) in the same work.

## Files to Modify
- `internal/component/iface/config_apply.go`
- `internal/component/iface/config_apply_deferred_test.go`
- `internal/component/iface/reconcile_learned_test.go`
- `internal/component/iface/link_queue.go`
- `internal/component/iface/link_queue_test.go`
- `internal/component/iface/rate.go`
- `internal/component/iface/register.go`
- `internal/component/iface/iface.go`
- `internal/component/iface/resolve.go`
- `internal/plugins/iface/netlink/counter_linux.go`
- `internal/plugins/iface/netlink/counter_linux_test.go`
- `internal/plugins/iface/netlink/counter_integration_linux_test.go`
- `internal/plugins/iface/netlink/show_linux.go`
- `internal/plugins/iface/dhcp/dhcp_v6_attach_linux.go`
- `internal/plugins/iface/dhcp/rfc9915_dhcp_v6_attach_linux_test.go`
- `docs/architecture/iface/management.md`
- `docs/features/interfaces.md`

## Files to Create
- None.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| reconciler-repairs-against-a-stale-snapshot | 2026-08-23 | `resyncCarrierState` judges carrier from a 1 Hz sample |
| reconciler-repairs-against-a-stale-snapshot | 2026-10-17 | `reconcileOnRegistryChange` ignores `applyIsOwed` |
| reconciler-repairs-against-a-stale-snapshot | 2026-10-19 | `goneWhileUp` misses an undelivered down and up |

## Work Not Done

Sibling children of `plan/release/fixes/spec-single-owner-reload-reconcile.md`:
- `plan/release/fixes/spec-single-owner-reload-reconcile-1-tx-conformance.md` (reload-wide pre-autoload verification and registered conformance suite)
- `plan/release/fixes/spec-single-owner-reload-reconcile-3-firewall-nft-ownership.md` (nft `NFTA_TABLE_USERDATA` ownership marker and restart reclaim)
- `plan/release/fixes/spec-single-owner-reload-reconcile-4-web-form-atomic-save.md` (atomic web form save)
- `plan/release/fixes/spec-single-owner-reload-reconcile-5-isis-identity-purge.md` (IS-IS old-SystemID LSP purge on `SystemID` change)

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | One schema, one command registry, every interface | the door every change reads | "A config change is a transaction of stream events (verify, apply, rollback) that each plugin answers for its own roots (`docs/architecture/config/transaction-protocol.md`)." | agrees | `reconcileOnRegistryChange` defers owned-address reconcile while an owed config apply has not yet reached the backend, and carrier/DHCPv6 reconcile stays aligned with kernel state. |
| `docs/features/interfaces.md` | Key Design Decisions | the `// Design:` page of `config_apply.go`, `reconcile_learned_test.go`, `rate.go`, `register.go`, `iface.go`, `counter_linux.go`, `show_linux.go`, `dhcp_v6_attach_linux.go`; a page this spec modifies | "The config owns its addresses, not every address on the unit." | agrees | Extends unit proofs to cover VPP-shaped zero-lifetime registered leases and adds `IFLA_CARRIER_CHANGES` to `InterfaceInfo` / `AddressState` so DHCPv6 avoids false Declines on missed carrier flaps. |
| `docs/architecture/appliance/kernel-profiles.md` | Go owns the guarantee | anchored to `internal/component/iface/config_apply.go` | — | silent | Covers appliance kernel module profiles; this spec requires no new kernel modules (`IFLA_CARRIER_CHANGES` is core Linux rtnetlink since 3.17). |
| `docs/architecture/core-design.md` | 14. Interface Management | anchored to `config_apply.go`, `register.go`, `iface.go` | "The `iface` component (`internal/component/iface/`) manages OS network interfaces through a pluggable backend architecture." | agrees | `IFLA_CARRIER_CHANGES` flows through the existing `Backend` / `InterfaceInfo` / `AddressState` abstractions (`CarrierChangesReported` false on backends that do not report it). |
| `docs/architecture/iface/logical-name-resolution.md` | Where the resolver lives, and what it returns | the `// Design:` page of `resolve.go`; anchored to `config_apply.go`, `register.go`, `iface.go`, `resolve.go`, `show_linux.go` | "`Addresses` and `ReadAddressState` read the addresses too (`GetInterface`) and report that failure." | agrees | `ReadAddressState` copies `CarrierChanges` and `CarrierChangesReported` from the existing `GetInterface` snapshot without adding a new syscall or bypassing the resolver. |
| `docs/architecture/iface/offload.md` | Absence means "keep the OS default" | anchored to `internal/component/iface/config_apply.go` | — | silent | Covers NIC offload settings; untouched by this spec. |
| `docs/architecture/plugin/plugin-system.md` | Registration fields | anchored to `config_apply.go`, `register.go` | — | silent | Covers plugin registration and seal rules; untouched by this spec. |
| `docs/architecture/traffic/cos-plugin.md` | Decisions | anchored to `config_apply.go`, `register.go` | — | silent | Covers Class of Service plugin; untouched by this spec. |
| `ai/doors/forwarding.md` | Traps | the door covering `internal/component/iface/*`, `internal/plugins/iface/netlink/*`, `internal/plugins/iface/dhcp/*` | "**A reconcile ran against a config the data plane never received** while a VPP apply was still owed (`plan/journal/reconciler-repairs-against-a-stale-snapshot.md`)." | agrees | This spec directly closes that exact trap (plus the carrier resync and DHCPv6 carrier-change traps under the same journal class). |
| `docs/architecture/iface/management.md` | Carrier events are queued per interface, never dropped | the `// Design:` page of `link_queue.go`, `link_queue_test.go`; anchored to `register.go`, `iface.go`, `resolve.go`; a page this spec modifies | "Two things read live carrier state now. The worker applies the queued final state, and a resync compares the recorded metric state against the interface list the rate tracker already dumps every second, moving a route only where the two definitely contradict each other." | agrees | Adds a monotonic carrier-event sequence to `linkEventQueue` so `resyncCarrierState` skips interfaces updated at or after the 1 Hz dump started, and documents `CarrierChanges` / `CarrierChangesReported` alongside `DADReported`. |
| `docs/architecture/ddos/ddos-detect-enhancements.md` | The bandwidth trigger is a second baseline instance | anchored to `internal/component/iface/rate.go` | — | silent | Covers DDoS bandwidth baseline detection; untouched by stamping a snapshot sequence before `ListInterfaces()`. |
| `docs/architecture/decisions/001-pull-model-metrics.md` | Decision | anchored to `internal/component/iface/rate.go` | — | silent | Covers pull vs push metrics model; untouched by this spec. |
| `docs/architecture/flowexport/flow-export-0-umbrella.md` | Single collection, many consumers | anchored to `internal/component/iface/rate.go` | — | silent | Covers IPFIX/sFlow export umbrella; untouched by this spec. |
| `docs/architecture/config/apply-ordering.md` | The requirement | anchored to `internal/component/iface/register.go` | — | silent | Covers cross-plugin commit apply ordering; untouched by this spec. |
| `docs/architecture/config/transaction-protocol.md` | 1. Design Principles | anchored to `internal/component/iface/register.go` | — | silent | Covers config transaction protocol; untouched by this spec. |
| `docs/architecture/l2tp/cpe-1-pppoe-client.md` | Decisions | anchored to `internal/component/iface/register.go` | — | silent | Covers PPPoE client; untouched by this spec. |
| `docs/architecture/plugin/plugin-relationships.md` | 1. System Plugins | anchored to `internal/component/iface/register.go` | — | silent | Covers plugin dependency and topic maps; untouched by this spec. |
| `docs/architecture/iface/vlan-qos-map.md` | Decision: two maps, because the kernel has two | anchored to `iface.go`, `show_linux.go` | — | silent | Covers VLAN egress/ingress QoS maps; untouched by this spec. |
| `docs/architecture/flowexport/flow-export-1-counter-export.md` | IPFIX counter records use the Total IEs, not the Delta IEs | anchored to `counter_linux.go`, `show_linux.go` | — | silent | Covers IPFIX/sFlow counter export; parsing `IFLA_CARRIER_CHANGES` alongside existing netlink attributes does not affect counter export IEs. |
| `docs/architecture/ospf/ospf-4-component-config.md` | Decisions | anchored to `internal/plugins/iface/netlink/show_linux.go` | — | silent | Covers OSPF configuration; untouched by this spec. |
| `docs/architecture/traffic/cos-dynamic.md` | Decisions | anchored to `internal/plugins/iface/netlink/show_linux.go` | — | silent | Covers dynamic CoS policies; untouched by this spec. |
| `docs/features/interfaces.md` | Design decisions | decision of 2026-11-22 | "`ze init` claims every discovered device by default, including the ones another program owns. `ze init --confirm` asks about each discovered device and writes only the ones the operator confirms" | agrees | `ze init` device claiming is untouched by this spec. |
| `docs/features/interfaces.md` | Design decisions | decision of 2026-11-22 | "`ze init --confirm` takes only an explicit answer: `yes`, `y`, `no` or `n`. Anything else, an empty line included, is not accepted, and the same device is asked again" | agrees | `ze init --confirm` prompting is untouched by this spec. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | Kernel/VPP backend mismatch rules are untouched by this spec. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | Superseded for daemon start by the 2026-11-26 row below; this spec adds no startup refusal. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | VPP firewall verification is untouched by this spec. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | Data-plane selector naming (`kernel` / `vpp`) is preserved untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | BGP export next-hop rewriting is untouched by this spec. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | BGP export next-hop policy authoring is untouched by this spec. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | Built-in BGP export chain compilation is untouched by this spec. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | BGP export chain next-hop folding is untouched by this spec. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | BGP explicit next-hop support is untouched by this spec. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | BGP originated vs forwarded next-hop policy is untouched by this spec. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | Implied filter seam is untouched by this spec. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | "Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code. Examples are a web certificate name that does not resolve, a looking-glass `server` entry with no certificate to present, and a looking-glass address list that does not parse. Where a service has several endpoints, only the endpoint that cannot be served stays down and the others serve. A reload whose config cannot be served stays refused as a whole, and the running services keep what they serve. In Thomas's words (voice-transcribed): "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; asked about a reload, "reload stay refused"; asked "Should only that endpoint stay down while the others serve", "yes". The reason is that a daemon that does not start brings up no networking, so the operator cannot reach the box to correct it." | agrees | No startup refusal is added; deferring owned-address reconcile while VPP owes an apply keeps startup and VPP reconnect non-fatal. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | "The rule of 2026-11-23 above, that a configuration which holds a feature that programs only the Linux kernel while the interfaces run on VPP "is refused at daemon start", is superseded for the start: ze starts, that feature stays off, and `ze doctor` reports the mismatch with its fix. A commit, a reload and `ze config validate` still refuse such a configuration, by the same verdict as before. Thomas was asked whether the rule that ze always starts replaces the refusal at start, "(a) yes, ze starts, that feature stays off and ze doctor reports the mismatch, while commit and reload still refuse it; (b) no", and answered "2a" (voice-transcribed). The reason is the one of the row above: a daemon that does not start brings up no networking, so the operator cannot reach the box to correct the mismatch." | agrees | Startup dataplane mismatch handling is untouched by this spec. |
| `docs/architecture/plugin/plugin-system.md` | Design decisions | decision of 2026-11-23 | "With no daemon running, a command that parses a config starts each declared external program in query mode, never its run loop, and validates against its declaration" | agrees | Offline external plugin query-mode validation is untouched by this spec. |

## Design presentation

### What changes
- `reconcileOnRegistryChange` skips while `applyIsOwed()` is true so it never reconciles against an `activeCfg` VPP never received.
- `linkEventQueue` stamps each 1 Hz resync snapshot with a monotonic carrier sequence read before `ListInterfaces()`, and `resyncCarrierState` skips interfaces whose carrier state the worker updated at or after that sequence.
- `ifacenetlink` reads Linux `IFLA_CARRIER_CHANGES` on the existing `RTM_GETLINK` dump, exposes it on `InterfaceInfo` and `AddressState`, and DHCPv6 `v6Worker` treats an increased counter across install/DAD reads as a link move instead of a duplicate address.
- `TestLearnedAddrSetVPPZeroLifetimeRegisteredLease` proves VPP-shaped zero-lifetime leased addresses registered with `RegisterLeasedAddresses` and kernel IPv6 link-locals survive commit reconciliation.

### Before and after
- **Before:** An owned-address event while VPP owed an apply reconciled against absent VPP devices; a carrier event arriving during the 1 Hz rate dump caused a false route metric repair and incremented `ze_iface_carrier_resyncs_total`; an undelivered down-and-up between two `ReadAddressState` calls caused DHCPv6 to send a false RFC 9915 Decline.
- **After:** Registry reconcile waits until `applyOwedConfig` succeeds; `resyncCarrierState` only repairs genuine contradictions predating the last worker carrier event; `v6Worker` detects undelivered down-and-up flaps via `IFLA_CARRIER_CHANGES` and re-installs via `noteMove()` instead of Declining.

### Alternatives set aside
- **Reconciling against `owedApply.previous` in `reconcileOnRegistryChange`:** Rejected because the dataplane is not yet converged on the committed config and may not even have a connected VPP channel; skipping until `applyOwedConfig` succeeds matches AC-1 and guarantees `ownedAddresses()` is applied in the exact pass that brings the dataplane current.
- **Requiring two consecutive 1 Hz resync ticks to confirm a contradiction:** Rejected in favor of monotonic snapshot sequencing, which is deterministic, adds no 1-second delay to genuine repairs, and eliminates the race structurally rather than widening a polling window.
- **Reading `/sys/class/net/<dev>/carrier_changes` in `ifacedhcp`:** Rejected because `counterSource` already reads the exact `RTM_GETLINK` message from netlink (zero extra syscalls, works inside read-only `/sys` namespaces, and stays behind the `iface.Backend` abstraction).

### Documents departed from
None.

### Choices for Thomas
none

## Review Gate

- [ ] `/ze-review` passed on diff
- [ ] `/ze-review-spec` confirmed all acceptance criteria and files

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Producer evidence re-read for every row
- [ ] No code snippets

### Goal Gates (MUST pass)
- [ ] AC-1 to AC-5 each proven by a named test
- [ ] Wiring Test table complete
- [ ] Targeted tests pass in the main checkout: `./le test unit pkg path ./internal/component/iface/`, `./le test unit pkg path ./internal/plugins/iface/netlink/`, `./le test unit pkg path ./internal/plugins/iface/dhcp/`, and `./le test integration iface` / QEMU run for `iface-link-flap-during-commit.ci`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Linux integration test using `//go:build linux` + `userns.Enter`

### Closure
- [ ] Owned journal rows marked `fixed:` with commit and test evidence
