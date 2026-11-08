# Spec: iface-static-neighbor-entries

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | config |
| Priority | P3 |
| Release reason | Adds a capability absent from the supported release: operator-configured static ARP and IPv6 neighbor entries. The shipped neighbor table readback stays required |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

An operator must be able to pin an IPv4 address to a MAC (static ARP) and an
IPv6 address to a MAC (static neighbor) on an interface, for a device that
does not answer ARP or ND, for a load balancer's virtual MAC, or to stop ARP
spoofing on a point link. Deleting the entry from the config must delete it
from the kernel: VyOS shipped a release where it did not (T9268).

| # | Operator need |
|---|---------------|
| 1 | Static IPv4 entry on an interface: 192.0.2.10 at 00:01:02:03:04:0a |
| 2 | Static IPv6 entry: 2001:db8::10 at the same MAC |
| 3 | Remove one entry, or all, and see them gone from the kernel |
| 4 | `show interface neighbors` shows them as permanent and as configured by Ze |
| 5 | An entry on a unit (VLAN) or a selector-bound interface lands on the right device |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/iface/logical-name-resolution.md` - the interface name in config is logical
  → Constraint: the entry is written on the resolved device; an unbound interface defers the entry
- [ ] `ai/doors/forwarding.md`, "Who depends on it from outside" and Traps
  → Constraint: Ze owns only what it marked; a reconciler that deletes neighbor entries another owner wrote (the kernel, the DHCP server's reply entries, VRRP) repeats the "reconciler claims state another owner installed" defect

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/iface/backend.go` - `ListNeighbors(family)` and `WatchNeighbors` only; no add or delete
- [ ] `internal/plugins/iface/netlink/neighbor_linux.go` - `ListNeighbors` dumps RTM_GETNEIGH, maps NUD flags to names (`permanent`, `noarp`, `reachable` ...); nothing writes
- [ ] `internal/plugins/iface/vpp/neighbor.go` - `ListNeighbors` by `ip_neighbor_dump`, STATIC shown as `permanent`; `WatchNeighbors` refuses with `errNoNeighborWatch`
- [ ] `internal/component/iface/yang/ze-iface-conf.yang` - unit `ipv4` holds arp-filter, arp-accept, proxy-arp, arp-announce, arp-ignore sysctls; no neighbor entry list

**Behavior to preserve:**
- Neighbor readback on both backends; every neighbor entry Ze did not configure.

**Behavior to change:**
- Configured static entries are installed as permanent, tagged as Ze's, reconciled on every apply, and deleted when removed from the config.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Neighbor readback | `Backend.ListNeighbors`, netlink `neighbor_linux.go`, VPP `neighbor.go` | Reads the table for show and consumers | Extends: add and delete beside it on the same backend interface |
| DHCP server reply neighbor write | `internal/plugins/dhcpserver` (logical-name-resolution page, "The DHCP server binds the same way") | Writes a neighbor entry for a reply on the answering device | Does not apply as the owner; its entries must survive this spec's reconcile |
| Route ownership by protocol | `internal/core/rtproto`, forwarding door | Ze owns routes by `rtm_protocol` | Extends the idea: a static entry carries a Ze owner tag (NDA_PROTOCOL, A-1), and reconcile touches only tagged entries |
| Selector binding | `config_apply.go` `deviceFor` | Resolves the device once per apply | Extends: entries land on the resolved device or unit device |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: a neighbor list on the unit's `ipv4` and `ipv6` containers (or a top-level list keyed by interface and address, as VyOS `protocols static arp interface <if> address <ip> mac <mac>`; settled at design).

### Transformation Path
1. Verify: address family matches the container; MAC unicast and non-zero; address not one of the interface's own.
2. Apply: for each configured entry, replace (create or update) a permanent entry tagged as Ze's on the resolved device.
3. Reconcile: list Ze-tagged entries; delete those no longer configured. Entries without the tag are never touched.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| iface to kernel | RTM_NEWNEIGH with NLM_F_REPLACE, RTM_DELNEIGH | No |
| iface to VPP | `ip_neighbor_add_del` with the STATIC flag | No |

### Changed-Wire Census

**Search evidence:** owed at design (`show interface neighbors` gains an owner field).

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| [format] | [role] | [source site] | [finding] | [disposition] |

### Integration Points
- The apply journal (rollback re-installs or removes), the neighbor show command.

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
| A-1 | The Ze kernel keeps NDA_PROTOCOL on a neighbor entry and returns it on dump, so ownership is readable from the kernel after a restart | Linux 5.x added NDA_PROTOCOL | Ownership needs a persisted list in the state store instead | Integration test: add with protocol, dump, compare | unvalidated |
| A-2 | The vendored netlink `Neigh` carries a protocol field, or Ze encodes the attribute itself | to check in `vendor/github.com/vishvananda/netlink` | Own encoder, or a module patch kept as a diff | Read the vendored type | unvalidated |
| A-3 | VPP STATIC neighbors cannot carry an owner tag | `vpp/neighbor.go` maps only STATIC and NO_FIB_ENTRY | VPP reconcile must use a persisted list | Read the VPP API | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A reconcile deletes an operator's or another plugin's permanent entry | Entry vanishes after a commit | Only tagged entries are deleted (AC-5) |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Traffic to the pinned host, or to a host whose entry was wrongly deleted |
| How is it reverted? | Remove the entries; reconcile deletes only Ze's |
| Who else touches this path? | DHCP server reply entries, VRRP, IPv6 RA and ND consumers of `WatchNeighbors` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A static IPv4 entry on eth0 committed | → | neighbor apply in the iface apply | `test/plugin/iface-static-neighbor-apply.ci` (`option=needs-linux`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | IPv4 entry 192.0.2.10 at 00:01:02:03:04:0a on eth0 | The kernel holds a PERMANENT entry with that address, MAC and device | None |
| AC-2 | IPv6 entry on eth0 | The kernel holds a PERMANENT IPv6 entry | None |
| AC-3 | The whole neighbor config deleted | Both PERMANENT entries are gone from the kernel (VyOS T9268 regression) | None |
| AC-4 | One entry's MAC changed | The kernel entry is replaced in place, never duplicated | None |
| AC-5 | A PERMANENT entry Ze did not configure (written by `ip neigh` or another plugin) on the same device | Survives every apply and every removal | It is not Ze's |
| AC-6 | Entry on a `mac match`-bound interface or a VLAN unit | Lands on the resolved device or unit device | None |
| AC-7 | Interface unbound at commit | Commit succeeds; the entry is installed when the device appears | Deferred, as every unbound setting |
| AC-8 | The address is one of the interface's own addresses, or the MAC is multicast or zero | Verify refuses naming the leaf | None |
| AC-9 | Daemon restart after an entry was removed from the config while Ze was down | The stale Ze entry is deleted at the first apply | None |
| AC-10 | `show interface neighbors` | Configured entries show as permanent and owned by Ze | None |
| AC-11 | `backend vpp` | Entries installed as VPP STATIC neighbors and removed on delete, or the commit refuses with the YANG path | None |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Pins a host and later removes it | config, apply, reconcile | `test/plugin/iface-static-neighbor-apply.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestStaticNeighborReconcileTouchesOnlyOwned` | `internal/component/iface/config_neighbor_test.go` | AC-3, AC-5, AC-9 | |

### Boundary Tests (numeric inputs)
N-A: no numeric input.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `iface-static-neighbor-apply` | `test/plugin/` | AC-1, AC-2, AC-3, AC-5 | |

## Files to Modify
- `internal/component/iface/yang/ze-iface-conf.yang` - the neighbor list
- `internal/component/iface/backend.go` - add and delete calls
- `internal/plugins/iface/netlink/neighbor_linux.go`, `internal/plugins/iface/vpp/neighbor.go` - backend writes
- `internal/component/iface/config.go`, `internal/component/iface/config_apply.go`

## Files to Create
- `test/plugin/iface-static-neighbor-apply.ci`

## Implementation Steps

1. **Phase: Wiring** - YANG, parse, failing `.ci`.
2. **Phase: Netlink** - tagged replace, owned-only reconcile.
3. **Phase: VPP** - STATIC add and delete, or refusal.

Proof: `./le test unit plugins`, `./le test functional plugin` for the named `.ci` file, `./le test qemu all`.

## VyOS reference

VyOS checkout `~/Code/github.com/vyos/vyos-1x`, read 2026-11-08 at `611550d23`.

| Ticket / commit | Lesson for Ze |
|-----------------|---------------|
| T9268, `30ac19fd3` "static_arp: Fix deletion of static ARP entries" | Deleting the `protocols static arp` node left the PERMANENT entry in the kernel; the smoketest now asserts it gone after deleting the whole node. AC-3 |

## Design Insights

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
- Proxy NDP entries (VyOS `neighbor-proxy`) are not in this spec.

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
