# Spec: vrrp-deferred-accept-mode-dataplane

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 7/7 |
| Updated | 2026-09-08 |

## Post-Compaction Recovery

**Re-read these after context compaction:** this spec file;
`.claude/rules/planning.md`; `docs/architecture/vrrp/vrrp-first-hop-redundancy.md`
(the closed VRRP work and its Known Limitations);
`internal/plugins/vrrp/instance.go`, `groups.go`, `fsm/fsm.go`.

## Task

Deferral holder. Provenance: `spec-vrrp-6-interop` (Known Limitations), recorded
2026-07-15 in `plan/deferrals.md` row 70. The named destination
(`plan/spec-vrrp-0-umbrella.md`) was closed and removed, so this file is the
work's home. The surviving `plan/spec-vrrp-7-vpp.md` covers the VPP dataplane
only and does not own this topic.

Two pieces of deferred work, verified against the producing code on 2026-07-16:

## OWNER RULING 2026-08-05: implement the accept-mode filtering

**Thomas ruled that ze IMPLEMENTS the dataplane filtering.** The two cheaper
answers put beside it were not taken: rejecting `accept-mode false` on a
non-owner at config validation, and leaving the disclosed `{gap}` as it stands.

This closes an open RFC MUST NOT violation rather than adding a feature.
`RFC9568-6.4.3-7` says "Active: never accept packets addressed to the Virtual
Router IPvX address(es) when neither owner nor Accept_Mode True", and the ledger
carries it as a `{gap}`. Under the 2026-07-27 directive that classification was
void and had to be re-raised rather than cited; this is the answer.

**Re-verified at the producers 2026-08-05**, so implementation starts from
measurement:

- The FSM emits `InstallVIPs{VIPs: i.cfg.VIPs}` unconditionally at all three
  promotion sites (`internal/plugins/vrrp/fsm/fsm.go`).
- `doInstallVIPs` registers the whole VIP set through the iface address-owner
  registry with no differentiation (`internal/plugins/vrrp/instance.go`).
- `AcceptMode` reaches config parsing, the version-2 rejection rule, and the show
  snapshot. Nothing else. `fsm/events.go` states it in the struct: "stored for
  the state snapshot only".

So a non-owner Active with `accept-mode false` answers traffic on the virtual
address today, whatever the operator configured.

### What the ruling commits ze to

| Piece | Where |
|-------|-------|
| Per-VIP filtering installed on promotion, removed on demotion | The `InstallVIPs` payload must carry the decision, and `doInstallVIPs` must act on it. Today neither sees `AcceptMode` |
| The Section 6.1 owner exemption | `EffectiveAcceptMode` (`internal/plugins/vrrp/groups.go`) already folds ownership in, so the decision input exists |
| The R014 carve-out: never drop IPv6 NS/NA even with Accept_Mode false | ICMPv6 types 135 and 136 must survive the filter. This is `RFC9568-6.1-1` |
| A tagged test per requirement row | `RFC9568-6.4.3-7` and `RFC9568-6.1-1` |
| A QEMU integration test | Mandatory for linux-only code, never skipped for "needs hardware" (`ai/rules/platform-linux.md`) |
| The YANG description stops disclaiming the gap | `internal/plugins/vrrp/yang/ze-vrrp-conf.yang` |

### The ledger consequence, easy to miss

`RFC9568-6.1-1` is currently `{not-applicable}` and its reason says why: the
prohibition "carves an exception out of Accept_Mode packet filtering, and ze
installs no such filter at all". **Once the filter exists that reason expires.**
The requirement becomes live and needs its own tagged test, so re-classifying it
is part of this spec's closure rather than a later discovery.

### Scope note

Item 2 below, priority-decrement tracking, is untouched by this ruling. It has
its own YANG surface and its own tests, and it stayed open while item 1 landed.
Its design is "Item 2 design, decided 2026-09-08" further down this section, and
it stays in this spec (the paragraph after item 2 says why).

1. **accept-mode is not enforced on the dataplane.** The leaf is parsed
   (`groups.go`), cross-leaf validated (rejected under version 2,
   `groups.go`), carried into the FSM config (`instance.go`) and
   reported in the `show vrrp` payload (`instance.go` into the
   `accept-mode` JSON field, `vrrp.go`). Nothing consumes it beyond the
   snapshot: `fsm/events.go` says so in the config struct itself, and the
   FSM emits `InstallVIPs{VIPs: i.cfg.VIPs}` (`fsm/fsm.go`, `:359`, `:378`)
   without consulting `cfg.AcceptMode`. The executor's `doInstallVIPs`
   (`instance.go`) then registers the full VIP set through the iface
   address-owner registry unconditionally. That is the function that would have
   to differentiate, and it does not. Result: an Active non-owner holds the VIPs
   as ordinary kernel addresses on the macvlan and answers traffic addressed to
   them whichever way the leaf is set, violating RFC 9568 R030/R031
   (`rfc/short/rfc9568.md`). Work: install real filtering, keeping the
   owner exemption (Section 6.1) and the IPv6 NS/NA carve-out (R014,
   `rfc/short/rfc9568.md`), and stop the YANG description
   (`yang/ze-vrrp-conf.yang`) from having to disclaim the gap.

2. **Priority-decrement tracking is not implemented.** No interface, route or
   health tracking object exists: the vrrp YANG has no tracking leaf
   (`yang/ze-vrrp-conf.yang` group list stops at `accept-mode`), and the only
   priority producer is `GroupSpec.EffectivePriority` (`groups.go`),
   which returns 255 for an owner and the configured constant otherwise, with no
   decrement input. Junos, Nokia and VyOS offer it; ze does not.

Design decided 2026-09-08: the two halves stay in ONE spec. Item 1 landed in
commit b21f6f2048. Item 2 needs the same Required Reading, the same Current
Behavior and the same test directory, so a second spec would restate all three
to carry one feature. The tracking design below reads nothing the accept-mode
filter installs, so the halves are implemented and reviewed independently inside
this file.

### Item 2 design, decided 2026-09-08

**The tracked object is an interface's operational state, and nothing else this
pass.** `iface.Resolve` returns a device's state and `iface.Subscribe` delivers
its link events for any name, and this plugin already reads both for the parent
device (`parentReady` and `watchParent`, `register.go`). A tracked ROUTE needs a
watch keyed on a prefix, and a tracked HEALTH CHECK needs a script runner with
its own timers, its own output contract and its own security surface. Ze has
neither, so both are named in Known Limitations rather than half-built here.

**The config surface.** Both groupings of `ze-vrrp-conf.yang` take the same
container, node for node. The names follow Junos, so an operator's fingers carry
over. Every node carries a one-line `description` and a `ze:help` beside it
(`ai/rules/config.md`); the texts are written at implementation and their
content is stated in the last column here.

| Node | Kind | Type and constraints | What its texts must say |
|------|------|----------------------|-------------------------|
| `track` | container under `group` | -- | Interfaces whose loss lowers the priority this group advertises. The help states that the decrements of every interface that is down are summed, that the sum comes off the configured priority, that the result never falls below 1 (RFC 9568 Section 5.2.4), that Ze advertises the new priority at once, and that tracking is refused on the address-owner group |
| `interface` | list under `track`, `key "name"` | `max-elements 16` | One tracked interface and the priority its loss costs. The help states that Ze reads operational state ONLY, so an address of this group's family is not required, and that two interfaces down cost the sum of their decrements |
| `name` | leaf, the list key | `zt:node-name`, `ze:validate "vrrp-track-interface"` | The interface to watch. The help states that the name is an interface name from the interface tree or a kernel device name, that Ze resolves it through the resolver `show interface` uses, and that a name resolving to no device counts as DOWN and is logged |
| `priority-decrement` | leaf under `interface` | `uint8`, `range "1..254"`, `mandatory true` | The priority subtracted while this interface is down. The help states that there is no default because a default would pick the operator's failover policy, and points at RFC 9568 Section 8.3.2 for how far apart two priorities belong |

What an operator writes:

```
vrrp {
    group uplink {
        vrid 10;
        virtual-address [ 192.0.2.1 ];
        priority 200;
        track {
            interface eth1 {
                priority-decrement 150;
            }
        }
    }
}
```

| Decision | Answer | Why |
|----------|--------|-----|
| A `track` container, not a bare leaf-list | `track { interface <name> { priority-decrement <n>; } }` | A per-interface decrement is what Junos, Nokia and VyOS offer, and one decrement for a whole list cannot say "the uplink costs 150, the peer link costs 20". The container also holds a future `route` list with no rename. |
| `priority-decrement` is `mandatory true` | no default | A default makes Ze choose the operator's failover policy. `vrid` in the same list is already mandatory, so the shape is not new to this tree. |
| `name` is `zt:node-name` with a suggestion, not a leafref | completion offers the configured interfaces and refuses nothing | `osDeviceFor` (`internal/component/iface/resolve.go`) falls back to the kernel device name when no os-name selector overrides it, so a tracked device the `interface` tree does not carry still resolves. A leafref would refuse it and would need a union over the four interface kinds. `RegisterSuggestion` is the plugin's route to completion (`ai/patterns/config-option.md` step 5b). |
| `max-elements 16`, repeated in the verifier | mirrors `maxVIPs` | `validateGroup` already re-checks the VIP maximum for a producer that skipped schema validation, and the tracked list gets the same treatment. |
| Tracking on the address owner is REFUSED at verify | `validateGroup` error naming the group | RFC 9568 Section 5.2.4: "The priority value for the VRRP Router that owns the IPvX address associated with the Virtual Router MUST be 255 (decimal)." A decrement can never take effect there, and Ze rejects a leaf it cannot honor exactly (`ai/rules/architecture.md`). `ze doctor` reports it through the same verifier (`diagnoseSections`, `doctor.go`). |

**Where the decrement enters.** `GroupSpec.EffectivePriority` (`groups.go`)
gains one argument, the summed decrement, and keeps its `uint8` result.

| Aspect | Answer |
|--------|--------|
| Argument | The summed decrement of the tracked interfaces that are down |
| Argument type | `uint16`, because 16 entries of 254 do not fit a `uint8` |
| First branch | The address owner, returning 255 unchanged, so tracking can never lower an owner (RFC 9568 Section 5.2.4) |
| Second branch | The configured priority less the decrement |
| Floor | 1, never 0. Section 5.2.4 keeps a Backup Router in 1-254, and 0 says the Active Router stopped participating |
| Callers | `fsmConfig` (`instance.go`) and the tests. Nothing else reads it |

`fsmConfig` (`instance.go`) is the only non-test caller, and every stage after
it exists already:

1. `iface.Subscribe` wakes the instance worker on a link change for any watched device.
2. `evaluateTracking` (new, beside `evaluateReadiness`) re-reads every tracked interface through `deps.linkUp` and rebuilds the set that is down. It dispatches only when that set CHANGED, so churn on an unrelated device sends no advertisement.
3. The dispatch is `fsm.ConfigUpdated{Config: in.fsmConfig()}`, whose `Priority` is `EffectivePriority(decrement)`.
4. `masterConfigUpdated` (`fsm/fsm.go`) sends an advertisement from the new priority at once and re-arms the advert timer. `backupConfigUpdated` re-arms the master-down timer, whose skew is priority-derived. The FSM takes no new event and no new action.
5. `doSendAdvert` re-encodes on every send and caches nothing (`instance.go`), so the decremented value is on the wire in the next advertisement.

A decremented Active does not resign. It keeps advertising at the lower
priority, and a Backup with a higher priority preempts it through the ordinary
Section 6.4.3 path. Ze invents no priority-0 shortcut.

**Failing closed.** `deps.linkUp` returns `(bool, error)`. A name that resolves
to no device counts as DOWN, and the instance logs the resolver error at Warn:
an uplink Ze cannot find is not carrying traffic. Reading an unresolvable name
as up is a zero value that looks like an answer (`ai/rules/principles.md`).

**Watching more than one device.** `watchParent` becomes
`watchLinks(devices []string)`, delivering one coarse wake-up for a change on
any of them. The parent and the tracked set are watched for the same reason, and
a tracked interface CAN be the parent, so one merged watch removes a second
select arm and a double subscription. `reconfigure` compares the wanted device
set with the watched one and signals the worker on `rewatch` when they differ,
so an interface tracked by the commit in hand is watched from that commit.

**Observability.** `show vrrp` already reports `priority` (configured) beside
`effective-priority` (running), so the decrement is visible with no new
plumbing. One field is added, `tracked-down`, holding the tracked interfaces
that are down now, so an operator who reads a lowered priority sees which
interface caused it.

## Required Reading

### Architecture Docs
- [ ] `docs/guide/vrrp.md` - the operator-facing statement of the current limitation
  → Constraint: whatever ships here must retire the documented caveat, not add a second one
- [ ] `ai/rules/architecture.md` - exact or reject
  → Constraint: a leaf ze cannot enforce exactly must fail verify, never approximate silently
- [ ] `docs/architecture/vrrp/vrrp-macvlan-vmac-dataplane.md` - the macvlan/vmac recipe the filter must not break
  → Constraint: the ARP/ND sysctl recipe makes the macvlan the sole responder; a filter that drops ARP or ND breaks virtual-MAC ownership
- [ ] `ai/patterns/config-option.md` - the structural template for the tracking leaves
  → Constraint: every added node carries a one-line `description` (96 characters, 25 words) AND a `ze:help` beside it; a plugin reaches completion through `RegisterSuggestion` in a `register*.go` file, never through `RegisterValidators`

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc9568.md` - the conformance target
  → Constraint: R030/R031 (§6.4.3, `rfc/short/rfc9568.md`): Active accepts packets to the virtual addresses only if owner or Accept_Mode is True, otherwise MUST NOT accept them
  → Constraint: R014 (§6.1/§6.4.3, `rfc/short/rfc9568.md`): IPv6 NS/NA are never dropped, even with Accept_Mode False
- [ ] `rfc/short/rfc3768.md` - v2 has no Accept_Mode concept
  → Constraint: the v2 rejection at `groups.go` stays; this spec changes v3 behavior only

**Key insights:**
- The FSM is not the gap. The FSM already carries the flag; the dataplane never reads it.
- Accept_Mode False is the RFC default (`rfc/short/rfc9568.md`), so today's default configuration is the non-conforming one.

## Current Behavior (MANDATORY)

**Source files read:** (verified 2026-07-16)
- [ ] `internal/plugins/vrrp/groups.go` - parses `accept-mode` (:411-416), rejects it under v2 (:531-532), computes `EffectiveAcceptMode` as owner-or-configured (:148-154) and `EffectivePriority` with no decrement input (:141-146)
- [ ] `internal/plugins/vrrp/instance.go` - projects the flag into the FSM config (:420), installs the full VIP set unconditionally (:369-374), reports the flag in the show snapshot (:530)
- [ ] `internal/plugins/vrrp/fsm/fsm.go` - emits `InstallVIPs` with `cfg.VIPs` (:146, :359, :378), never reading `cfg.AcceptMode`; snapshot carries it (:398, :429)
- [ ] `internal/plugins/vrrp/fsm/events.go` - config field comment states the flag is stored for the snapshot only (:45-47)
- [ ] `internal/plugins/vrrp/vrrp.go` - `instanceView.AcceptMode` is the `accept-mode` JSON field of `show vrrp` (:95)
- [ ] `internal/plugins/vrrp/dataplane_linux.go` - the sysctl recipe that gives the macvlan ARP/ND ownership (:120-183); no packet filtering of any kind
- [ ] `internal/plugins/vrrp/yang/ze-vrrp-conf.yang` - `accept-mode` leaf, boolean, default false, description discloses "not dataplane-enforced this pass" (:121-129)

**Source files read for item 2:** (verified 2026-09-08)
- [ ] `internal/plugins/vrrp/groups.go` - `GroupSpec` carries config only and no runtime state (:97-146); `EffectivePriority` returns 255 for an owner and the configured constant otherwise, with no decrement input (:147-156); `applyGroupLeaves` overlays each leaf onto a spec pre-loaded with defaults (:401-471); `validateGroup` holds the cross-leaf rules, including the VIP maximum and the 1..254 priority range (:538-583)
- [ ] `internal/plugins/vrrp/instance.go` - `engineDeps` is the seam every side effect crosses (:85-119); the worker loop watches the parent and re-decides readiness from scratch on every wake-up (:189-250); `fsmConfig` is the ONLY non-test caller of `EffectivePriority` (:451-463); `reconfigure` dispatches `fsm.ConfigUpdated` under `mu` (:482-490); `snapshot` publishes `Priority` (configured) beside `EffectivePriority` (running) (:553-578)
- [ ] `internal/plugins/vrrp/register.go` - `parentReady` reads oper-state plus a family address through `iface.Resolve` and `iface.Addresses` (:447-486); `watchParent` turns `iface.Subscribe` link events into coarse wake-ups (:489-517); `liveDeps` wires both (:426-444)
- [ ] `internal/plugins/vrrp/fsm/fsm.go` - `masterConfigUpdated` re-sends the advertisement from the new priority and re-arms the advert timer (:348-372); `backupConfigUpdated` re-arms the master-down timer, whose skew is priority-derived (:247-258); `promoteToMaster` and `masterAdvert` read `i.cfg.Priority` for every advertisement (:298-332, :374-395)
- [ ] `internal/component/iface/resolve.go` - `Resolve`, `Addresses` and `Subscribe` take a LOGICAL name and fall back to the kernel device name when no os-name selector overrides it (`osDeviceFor`, :179-204); `Subscribe` fires for a device that does not exist yet (:126-142)
- [ ] `internal/plugins/vrrp/doctor.go` - the doctor check re-runs `extractGroupSpecs` plus `validateGroups`, so a new cross-leaf rule reaches `ze doctor` with no second implementation (:44-94)
- [ ] `internal/plugins/vrrp/vrrp.go` - `instanceView` is the `show vrrp` payload and already carries `priority` and `effective-priority` (:77-101)
- [ ] `rfc/full/rfc9568.txt` Section 5.2.4 - "The priority value for the VRRP Router that owns the IPvX address associated with the Virtual Router MUST be 255 (decimal)." and "VRRP Routers backing up a Virtual Router MUST use priority values between 1-254 (decimal)." and "The priority value zero (0) has special meaning, indicating that the current Active Router has stopped participating in VRRP."

**Behavior to preserve:**
- Owner semantics: an address owner accepts regardless of the leaf (`EffectiveAcceptMode`, `groups.go`) and advertises priority 255 (`EffectivePriority`, `groups.go`)
- The v2 plus accept-mode config rejection and its message (`groups.go`, `test/vrrp/vrrp-config-invalid.ci`)
- The doctor check that reports the same cross-leaf violation (`internal/plugins/vrrp/doctor.go`, `test/vrrp/vrrp-doctor-fires.ci`)
- The `accept-mode` field in the `show vrrp` payload (`vrrp.go`)
- The virtual-MAC ARP/ND recipe and its refcounted save/restore (`dataplane_linux.go`)
- Election, timers and failover behavior: this spec touches what an Active router accepts, never who wins

**Behavior to change:**
- With Accept_Mode False and a non-owner Active, packets addressed to a virtual address are no longer accepted (except IPv6 NS/NA)
- Retire the YANG description disclaimer and the `docs/guide/vrrp.md` caveat once enforced
- Priority tracking: a `track` container under the group, and a summed decrement passed into `EffectivePriority` so the advertised priority follows a tracked interface's operational state
- `watchParent` becomes `watchLinks` and covers the parent plus every tracked interface; `reconfigure` re-subscribes when the device set changes
- `show vrrp` gains one field, `tracked-down`
- The `doctor-vrrp-config-invalid` description gains the owner-with-tracking case, because it enumerates what the config can be wrong about

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: `interface ... unit ... ipv4|ipv6 vrrp group <name> accept-mode <bool>`, parsed at `groups.go` into `GroupSpec.AcceptMode`
- Runtime: the FSM's transition into Active emits `InstallVIPs` (`fsm/fsm.go`, `:359`, `:378`), executed at `instance.go`
- Wire: unicast or multicast frames arriving at the macvlan addressed to a virtual address (the traffic this spec must gate)
- Config (item 2): `interface ... unit ... ipv4|ipv6 vrrp group <name> track interface <name> priority-decrement <1..254>`, parsed at `groups.go` into `GroupSpec.TrackedInterfaces`
- Runtime (item 2): a link change on a tracked interface, delivered by `iface.Subscribe` (`internal/component/iface/resolve.go`) through the instance's watch

### Transformation Path
1. Config tree to `GroupSpec.AcceptMode` (`groups.go`), then cross-leaf verify rejects the v2 combination (`groups.go`)
2. `GroupSpec.EffectiveAcceptMode` folds in ownership (`groups.go`) and `instance.fsmConfig` projects it onto `fsm.Config` (`instance.go`)
3. FSM reaches Active and emits `InstallVIPs`; `instance.doInstallVIPs` registers the VIP CIDRs with the iface address-owner registry (`instance.go`), which reconciles them onto the macvlan
4. Today the chain ends there: the kernel answers for the VIP as for any local address. The missing stage is a per-instance acceptance filter installed and torn down alongside the VIPs, keyed on the effective accept-mode
5. `show vrrp` reads the flag back out of the FSM snapshot (`instance.go`, `vrrp.go`)
6. Tracking config to `GroupSpec.TrackedInterfaces` (`applyGroupLeaves`, `groups.go`), sorted by name so one config always extracts to one spec; `validateGroup` refuses the list on an address-owner group and refuses a zero decrement
7. A link event wakes the worker; `evaluateTracking` (`instance.go`) re-reads every tracked interface through `deps.linkUp` and rebuilds the set that is down, treating a name that does not resolve as down
8. A CHANGED set dispatches `fsm.ConfigUpdated{Config: in.fsmConfig()}`, whose `Priority` is `EffectivePriority(decrement)`; `masterConfigUpdated` (`fsm/fsm.go`) sends an advertisement at the new priority at once and `backupConfigUpdated` re-arms the priority-derived master-down timer
9. `doSendAdvert` re-encodes from the action's priority on every send (`instance.go`), so the decremented value is on the wire in the next advertisement; `snapshot` reports it as `effective-priority` beside the new `tracked-down` list

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree ↔ vrrp plugin | `GroupSpec` extraction and cross-leaf verify | [ ] |
| vrrp plugin ↔ FSM | `fsm.Config` projection (`instance.go`) | [ ] |
| vrrp plugin ↔ iface address owner | `RegisterOwnedAddresses` via `deps.installVIPs` | [ ] |
| vrrp plugin ↔ kernel filtering | to be decided at design (nftables via the firewall component, socket filter, or per-device sysctl) | [ ] |
| vrrp plugin ↔ iface resolver (tracked interface) | `deps.linkUp` over `iface.Resolve`, `deps.watchLinks` over `iface.Subscribe` | [ ] |
| vrrp plugin ↔ FSM (tracked change) | `fsm.ConfigUpdated` carrying the decremented `Priority`; no new event type | [ ] |

### Integration Points
- `instance.doInstallVIPs` / `doRemoveVIPs` (`instance.go`): the filter's install and teardown must share their lifetime, or a demoted Backup keeps a stale rule
- `dataplane_linux.go` sysctl recipe: the filter must sit above ARP/ND resolution so the virtual MAC still answers
- `internal/component/firewall/`: the existing rule-installation surface, if the design chooses nftables rather than a socket-level filter
- `GroupSpec.EffectivePriority` (`groups.go`): the single point the tracking decrement feeds, and `fsmConfig` (`instance.go`) is its only non-test caller
- `instance.run` and `instance.reconfigure` (`instance.go`): the watch covers the parent AND every tracked interface, so a changed tracked set has to re-subscribe (`rewatch`)
- `diagnoseSections` (`doctor.go`): re-runs `validateGroups`, so the owner-with-tracking rejection reaches `ze doctor` with no second implementation

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Filtering can be installed without breaking the virtual-MAC ARP/ND recipe | `dataplane_linux.go` operates on sysctls only, orthogonal to a packet filter | The recipe and the filter must be co-designed; QEMU proof needed early | QEMU lab: VIP unreachable with accept-mode false, ARP still answered from the virtual MAC | confirmed 2026-08-29 -- in a QEMU Linux guest the virtual address stays installed on the virtual-MAC macvlan while ping to it gets 100% loss, and reverting to accept-mode true restores the reply. The filter is at the input hook and the recipe is sysctls, so neither reads the other |
| A-2 | The existing firewall component can express a per-device destination-address drop | `internal/component/firewall/` installs rules today (surface not yet read for this spec) | A vrrp-owned filter path is needed instead | Design phase: read the firewall install path | confirmed 2026-08-29, and RESHAPED: the seam is `firewall.RegisterTables` plus `firewall.ApplyAll` (`internal/component/firewall/registry.go`), the same table registry copp, ddos-local and flowspec-firewall use. The rule is scoped to the ADDRESS rather than the device, which is what RFC 9568 Section 6.4.3 says: no ingress interface qualifies the prohibition |
| A-4 | An interface's operational state is the only tracked object this spec can deliver end to end | `iface.Resolve` returns `Binding.State` and `iface.Subscribe` delivers link events for any name (`internal/component/iface/resolve.go`), and `register.go` already reads both. No per-prefix route watch and no script runner exist | route and health tracking would need machinery this spec would have to build first | Read at the producer | confirmed 2026-09-08 |
| A-5 | A priority change reaches the wire with no FSM change | `masterConfigUpdated` re-sends the advertisement from the new config and re-arms the timer (`fsm/fsm.go`); `doSendAdvert` re-encodes on every send and caches nothing (`instance.go`) | the FSM would need a tracking event of its own | Unit test over the instance with a recording `sendAdvert` | confirmed 2026-09-08 |
| A-6 | A tracked interface name resolves whether or not the `interface` tree carries it | `osDeviceFor` falls back to the name itself when no os-name selector overrides it (`internal/component/iface/resolve.go`) | the leaf would have to be a leafref into the interface tree, and a bare kernel device could not be tracked | Unit test over the resolver seam plus the functional test's veth name | confirmed 2026-09-08 |
| A-3 | Interop scenarios that ping the VIP set accept-mode true and so keep passing | `internal/le/qemu/vrrp_keepalived_linux.go` sets accept-mode true for QS-1 | Enforcing the flag reds the interop lab | Run the keepalived lab after enforcement | confirmed 2026-08-29 -- `vrrpZeConfig` (`internal/le/qemu/vrrp_keepalived_linux.go`) writes `accept-mode true`, so every lab scenario takes the accepting branch and installs no filter at all |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Enforcement makes the VIP unpingable and looks like a regression to operators | Support reports "ping to VIP stopped working after upgrade" | Release note plus the RFC 9568 §6.1 ping guidance (`rfc/short/rfc9568.md`) |
| R-2 | Dropping IPv6 NS/NA with the filter breaks ND (violates R014) | IPv6 failover leaves stale neighbor entries | Explicit NS/NA carve-out with a dedicated test |
| R-3 | Tracking grows into a large config surface (objects, groups, weights) and stalls the accept-mode fix | Design phase expands past the accept-mode work | Closed 2026-09-08: accept-mode landed first (b21f6f2048), and the tracking surface is one container with two leaves |
| R-4 | A tracked name that resolves to no device counts as down, so a typo lowers the priority for good | `show vrrp` reports an `effective-priority` under the configured one, with the name in `tracked-down` | Deliberate: an uplink Ze cannot find is not carrying traffic. `linkUp` returns the resolver error and the instance logs it at Warn, so the cause is in the log rather than inferred |
| R-5 | A link event on an unrelated device dispatches `ConfigUpdated`, and `masterConfigUpdated` sends an advertisement every time | Advertisement counters climb with link churn | `evaluateTracking` dispatches only when the down set CHANGES; a unit test drives an event that changes nothing and asserts no advertisement |
| R-6 | A tracked interface added by a commit is never watched, because the subscription was made at startup | Tracking works after a restart and not after a commit | `reconfigure` compares the wanted device set with the watched one and signals `rewatch`; a unit test adds a tracked interface to a running instance and drives its link event |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `accept-mode false` on a non-owner Active | → | acceptance filter installed alongside the VIPs | `test/vrrp/vrrp-accept-mode.ci` |
| Active demotes to Backup | → | filter removed with `doRemoveVIPs` | `test/vrrp/vrrp-accept-mode.ci` |
| `track interface <name> priority-decrement <n>` and that interface goes down | → | `evaluateTracking` sums the decrement, `fsmConfig` lowers the priority, the next advertisement carries it | `test/vrrp/vrrp-track.ci` |
| The tracked interface comes back up | → | the decrement is withdrawn and the advertisement carries the configured priority again | `test/vrrp/vrrp-track.ci` |
| `track` configured on the address-owner group | → | `validateGroup` refuses the commit and `ze doctor` reports the same rule | `test/vrrp/vrrp-config-invalid.ci`, `test/vrrp/vrrp-doctor-fires.ci` |

## Acceptance Criteria

AC-1 to AC-5 cover item 1 (landed in b21f6f2048). AC-6 to AC-13 cover item 2,
and each names the observable a test reads: the priority byte on the wire, the
`show vrrp` payload, or the refusal message.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Non-owner Active, accept-mode false, ping to VIP | No reply (RFC 9568 R031) |
| AC-2 | Non-owner Active, accept-mode true, ping to VIP | Reply (RFC 9568 R030) |
| AC-3 | Address owner, accept-mode false | Accepts anyway (RFC 9568 §6.1); `show vrrp` reports effective accept-mode true |
| AC-4 | IPv6 non-owner Active, accept-mode false, NS to VIP | NA is still sent (RFC 9568 R014) |
| AC-5 | Active demotes to Backup | Filter and VIPs disappear together; no stale rule |
| AC-6 | Non-owner group, priority 200, `track interface eth1 priority-decrement 150`, eth1 goes down | The next advertisement carries priority 50, and `show vrrp` reports `effective-priority` 50 with `eth1` in `tracked-down` |
| AC-7 | eth1 comes back up | The next advertisement carries priority 200 again and `tracked-down` is empty |
| AC-8 | Two tracked interfaces down, decrements 50 and 30, priority 200 | The advertised priority is 120: the decrements of the interfaces that are down are summed |
| AC-9 | Summed decrement at or above the configured priority | The advertised priority is 1, never 0 (RFC 9568 Section 5.2.4: a Backup uses 1-254, and 0 says the Active Router stopped participating) |
| AC-10 | Address owner group carrying a `track` entry | The commit is refused, naming the group and the owner's fixed 255 (RFC 9568 Section 5.2.4); `ze doctor` reports it under `doctor-vrrp-config-invalid` |
| AC-11 | A tracked name that resolves to no device | The decrement is applied (fail closed) and the resolver error is logged at Warn |
| AC-12 | A link event that leaves every tracked interface in the state it was already in | No `ConfigUpdated`, so an Active sends no extra advertisement |
| AC-13 | A commit adds a tracked interface to a running group | That interface is watched from the commit, and its next link change decrements the priority |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestAcceptFilterTableDropsEveryAddressItIsGiven`, `TestAcceptFilterAcceptsNeighborDiscoveryBeforeAnyDrop`, `TestAcceptFilterTermNamesAreValidFirewallNames`, `TestAcceptFilterDeduplicatesAndSortsAddresses` | `internal/plugins/vrrp/acceptfilter_test.go` | the suppressed address set maps to the intended firewall table: one host-scoped drop per address, at the input hook, with the ICMPv6 135/136 carve-out ahead of every drop | done |
| `TestActiveNonOwnerWithAcceptModeFalseSuppressesLocalDelivery`, `TestActiveNonOwnerWithAcceptModeTrueAcceptsLocalDelivery`, `TestActiveAddressOwnerAcceptsWhateverAcceptModeSays`, `TestActiveV2RouterAcceptsOnlyWhenItOwnsTheAddress` | `internal/plugins/vrrp/acceptfilter_test.go` | all three directions of RFC 9568 Section 6.4.3, driven through a real promotion and read off the dataplane calls rather than the config | done |
| `TestAcceptFilterInstalledBeforeTheAddressAndWithdrawnAfterIt`, `TestAcceptModeChangeOnARunningActiveReachesTheDataplane`, `TestAcceptFilterShareTheTableAndWithdrawIndependently`, `TestAcceptFilterDoesNotReachTheFirewallWhenNothingChanges` | `internal/plugins/vrrp/acceptfilter_test.go` | install before the address, withdraw after it, accept-mode flipped on a running Active, two groups sharing one table, and no reconcile when nothing changed | done |
| `TestEffectivePriorityWithTracking` | `internal/plugins/vrrp/groups_test.go` | `EffectivePriority(decrement)` subtracts the summed decrement, floors at 1, keeps 254 reachable, and still returns 255 for an owner whatever the decrement says | done |
| `TestTrackedInterfacesExtracted` | `internal/plugins/vrrp/groups_test.go` | the `track interface` list reaches `GroupSpec.TrackedInterfaces` with each entry's decrement, sorted by name, and an absent container yields an empty list rather than an error | done |
| `TestTrackedInterfaceRejectsAnUnusableDecrement`, `TestTrackedInterfaceRequiresADecrement` | `internal/plugins/vrrp/groups_test.go` | the boundary rows: decrement 0 and 255 are refused, 17 entries exceed the maximum naming it, and a missing `priority-decrement` is a hard error rather than a decrement of 0 | done |
| `TestTrackOnOwnerGroupIsRejected` | `internal/plugins/vrrp/groups_test.go` | `validateGroup` refuses tracking on the address-owner group, names the group and says the owner advertises 255; the same tracking on a non-owner group is accepted | done |
| `TestTrackedInterfaceDownDecrementsTheAdvertisedPriority`, `TestTrackedInterfaceUpRestoresThePriority` | `internal/plugins/vrrp/instance_test.go` | a fake `linkUp` reporting a tracked interface down makes the running instance dispatch `ConfigUpdated`, and the recording `sendAdvert` carries the decremented priority; the reverse restores it. The first also drives a SECOND interface down and reads 120, which is AC-8 | done |
| `TestUnresolvableTrackedInterfaceCountsAsDown` | `internal/plugins/vrrp/instance_test.go` | `linkUp` returning an error applies the decrement, so the failure to resolve never reads as "up" | done |
| `TestTrackingDoesNotAdvertiseWhenNothingChanged` | `internal/plugins/vrrp/instance_test.go` | a wake-up that leaves every tracked state as it was dispatches nothing, so an Active sends no extra advertisement | done |
| `TestReconfigureWatchesANewlyTrackedInterface` | `internal/plugins/vrrp/instance_test.go` | a config change that adds a tracked interface re-subscribes the watch, and one that changes no device set does not | done |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `track interface <name> priority-decrement` | 1-254 | 254 | 0 | 255 |
| effective priority after decrement | 1-254 (non-owner) | 254 | 0 | 255 |
| tracked interfaces in one group | 0-16 (an absent `track` container means no tracking) | 16 | n/a | 17 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `vrrp-accept-mode.ci` | `test/vrrp/` | accept-mode true and false change what the Active answers; kernel rules read back, live UDP probe to the virtual address, teardown leaves no rule | written; `needs-linux:caps=net-admin`, so it runs in the QEMU nightly and skips on darwin |
| `vrrp-track.ci` | `test/vrrp/` | a tracked veth goes down and the group advertises the decremented priority, then the configured one again when it returns; the advertisement is captured on the parent's veth peer and its priority byte read | PASS in the QEMU guest on 2026-09-08, and RED under the reverted decrement (see "QEMU evidence, item 2"); skips on darwin. `needs-linux:caps=net-admin` plus `exclusive:group=iface-owned-macvlan`, like `vrrp-instance-up.ci`; the capture follows `openCapture` / `captureMatch` (`internal/plugins/vrrp/transport/transport_integration_linux_test.go`). `show vrrp` is NOT read back there: reaching it from a `.ci` needs the external-observer dance `vrrp-show.ci` documents, and the payload's `effective-priority` and `tracked-down` are asserted in `TestTrackedInterfaceDownDecrementsTheAdvertisedPriority` instead |
| `vrrp-config-invalid.ci` (extended) | `test/vrrp/` | `track` on the address-owner group is refused at commit with the owner-255 message, and a zero `priority-decrement` is refused | written, PASS on darwin |
| `vrrp-doctor-fires.ci` (extended) | `test/vrrp/` | the same tree makes `ze doctor` report `doctor-vrrp-config-invalid` | written, PASS on darwin |

### TDD Evidence, item 2 (2026-09-08)

Every test below was written before the code it covers and watched fail.

**RED, the unit tests, before any of the tracking code existed**
(`./le job run label vrrp-unit command go test ./internal/plugins/vrrp/`):

```
internal/plugins/vrrp/groups_test.go:1598:34: too many arguments in call to g.EffectivePriority
	have (uint16)
	want ()
internal/plugins/vrrp/groups_test.go:1622:12: undefined: TrackedInterface
internal/plugins/vrrp/groups_test.go:1627:21: specs[0].TrackedInterfaces undefined (type GroupSpec has no field or method TrackedInterfaces)
internal/plugins/vrrp/groups_test.go:1664:33: undefined: maxTrackedInterfaces
internal/plugins/vrrp/instance_test.go:849:7: deps.linkUp undefined (type engineDeps has no field or method linkUp)
internal/plugins/vrrp/instance_test.go:873:5: in.evaluateTracking undefined (type *instance has no field or method evaluateTracking)
internal/plugins/vrrp/instance_test.go:878:71: v.TrackedDown undefined (type instanceView has no field or method TrackedDown)
FAIL	github.com/ze-software/ze/internal/plugins/vrrp [build failed]
```

**RED, the behavior, with the types in place and the decrement not yet reaching
the FSM.** This is the interesting one: `trackDown` was already correct and the
priority on the wire was not, which is exactly the defect the feature exists to
prevent.

```
--- FAIL: TestTrackedInterfaceDownDecrementsTheAdvertisedPriority (0.00s)
    instance_test.go:860: advertised priority = 200 after eth1 went down, want 150 (200 less its decrement of 50)
    instance_test.go:863: show vrrp reports effective-priority 200 and tracked-down [eth1], want 150 and [eth1]
    instance_test.go:871: advertised priority = 200 with both tracked interfaces down, want 120 (200 less 50 and 30)
--- FAIL: TestUnresolvableTrackedInterfaceCountsAsDown (0.00s)
    instance_test.go:919: advertised priority = 200 with an unresolvable tracked name, want 170 (200 less its decrement of 30)
--- FAIL: TestTrackingDoesNotAdvertiseWhenNothingChanged (0.00s)
    instance_test.go:949: adverts = 1 after one real change, want 2: a wake-up that changes nothing must not advertise
FAIL	github.com/ze-software/ze/internal/plugins/vrrp	0.868s
```

**GREEN**

```
ok  	github.com/ze-software/ze/internal/plugins/vrrp	0.940s
```

**GREEN, the functional suite** (`./le functional vrrp`, darwin):

```
1.5s     4/11  PASS  6  vrrp-doctor
1.7s     6/11  PASS  5  vrrp-doctor-quiet
2.1s     7/11  PASS  7  vrrp-idle
2.7s     5/11  PASS  4  vrrp-doctor-fires
3.3s     2/11  PASS  3  vrrp-config
4.6s     3/11  PASS  2  vrrp-config-invalid
pass  6/6  100.0%  4.6s  skip 5 [1, 8, 9, 10, 11]
```

`vrrp-track` is skip 11 and `vrrp-accept-mode` is skip 1: both carry
`needs-linux`, so they run in the QEMU nightly.

**RED walk over the two extended `.ci` files**, per
`ai/rules/interop-and-goal-validation.md`: the `validateTracking` call was
removed from `validateGroup`, the suite rebuilt, and both rows observed red.

```
3.3s     5/11  FAIL  4  vrrp-doctor-fires
5.7s     3/11  FAIL  2  vrrp-config-invalid
TEST FAILURE: 2 vrrp-config-invalid
cmd seq=12 (ze config validate -): expected exit code 1, got 0
TEST FAILURE: 4 vrrp-doctor-fires
cmd seq=3 (ze doctor --json vrrp-track-owner.conf): expected exit code 1, got 0
```

The rule was restored and both went green again (`pass 6/6 100.0%`).

### QEMU evidence, item 2 (2026-09-08)

Both Linux-only artifacts have now RUN, in the QEMU Alpine guest on ze's
runtime kernel (`./le qemu run kernel tmp/kernel/build/vmlinuz`). The guest
took cross-compiled binaries through `ZE_TEST_NO_BUILD`, `ZE_BIN` and
`ZE_EVIDENCE_ZE_BINARY`, because this shared checkout does not compile: several
other sessions hold it mid-edit, and `pkg/plugin/rpc/types.go` in the working
tree drops the `ConfigOperationType` constants its own consumers still name.
The binaries were built from a `go build -overlay` that presents HEAD for every
other session's uncommitted Go and the working tree for this spec's own files.

**The first run of `vrrp-track.ci` found a defect in its own fixture.** The
capture read the VRRP header at offset 1 and reported 12 for every
advertisement, which is this group's `vrid`, not its priority:

```
observer reported runtime failure: ZE-OBSERVER-FAIL: with zetrk1 up: the advertised priority stayed at 12 within 20s, want 200
```

RFC 9568 Section 5.2 lays the header out as
`|Version| Type  | Virtual Rtr ID|   Priority    |IPvX Addr Count|`, so Priority
is the third octet. `vrrpTrackPriorityByte` is now 2
(`internal/test/fixture/vrrp_track_linux.go`). Left at 1 the test would have
read the vrid on every leg and passed for any vrid that happened to equal the
expected priority.

**GREEN, `vrrp-track.ci` in the guest:**

```
═══ vrrp ══════════════════════════════════════════════════════════════════════
10.4s    1/1  PASS  11  vrrp-track
pass  1/1  100.0%  10.4s
    4 ✓ expect exit-code
    5 ✓ expect stderr-contains
    6 ✓ expect stdout-contains
    7 ✓ expect stdout-contains
    8 ✓ expect stdout-contains
QEMU VM: PASS
```

**RED, `vrrp-track.ci` with the tracking decrement reverted.** The revert is
`EffectivePriority` returning `g.Priority` in place of
`uint8(uint16(g.Priority) - decrement)`, so the advertised priority ignores the
tracked interface. The guest daemon was rebuilt under that revert and the test
driven against it:

```
observer reported runtime failure: ZE-OBSERVER-FAIL: with zetrk1 down: the advertised priority stayed at 200 within 20s, want 50
vrrp: tracked interface state changed, advertising a new priority ... tracked-down=[zetrk1] priority=200
22.8s    1/1  FAIL  11  vrrp-track
fail  0/1  0.0%
```

That second line is what the test exists to catch: the tracking machinery still
detects the link and dispatches, and the wire carries 200 anyway. The restored
daemon is the byte-identical binary that produced the GREEN above.

**GREEN, the interop scenario** (`./le qemu vrrp-keepalived-test scenarios
tracked-uplink-hands-the-vip-to-keepalived`, keepalived 2.3.1 in the guest):

```
=== tracked-uplink-hands-the-vip-to-keepalived: tracked veth down: ze drops to prio 50 and keepalived prio 100 takes the VIP (AC-6, AC-7) ===
  tracking: ze holds the VIP at prio 200 while zvt1501 is up
  tracking: zvt1501 down, ze advertised prio 50 and keepalived (prio 100) took the VIP
  tracking: zvt1501 up, ze advertised prio 200 again and keepalived returned to BACKUP
PASS: tracked-uplink-hands-the-vip-to-keepalived
OK: ze VRRP interoperates with keepalived across 1 scenario(s)
```

keepalived's own log carries the election it made on ze's decremented priority:
`(lab) Master received advert from 192.0.2.251 with higher priority 200, ours 100`
on the way in, and its notify script marks MASTER while ze advertises 50.

**RED, the interop scenario under the same revert:**

```
FAIL: tracked-uplink-hands-the-vip-to-keepalived: after the tracked link failed: ze's advertised priority stayed at 200, want 50
  tracking: ze holds the VIP at prio 200 while zvt1503 is up
QEMU VM: FAIL (exit code 1)
```

The first leg still passes under the revert, which is the point of asserting the
undecremented baseline first: 200 is proven to be a value the test can also see
when it is wrong.

**Discrimination records written** (`./le rfc discriminate-record`, route
`revert`, producer `internal/plugins/vrrp/groups.go::EffectivePriority`):

| Requirement | Polarity | Unit |
|-------------|----------|------|
| `RFC9568-5.2.4-1` | positive | `TestEffectivePriorityWithTracking` |
| `RFC9568-5.2.4-2` | positive | `TestEffectivePriorityWithTracking` |
| `RFC9568-5.2.4-1` | positive, negative | `TestOwnerAutoDetection` |
| `RFC3768-5.3.4-1` | positive, negative | `TestOwnerAutoDetection` |
| `RFC5798-5.2.4-1` | positive, negative | `TestOwnerAutoDetection` |

Each one was observed red under the break, for example:

```
break: body of EffectivePriority replaced by panic("BUG: ./le rfc discriminate-record disabled this producer to observe the red")
--- FAIL: TestEffectivePriorityWithTracking (0.00s)
    --- FAIL: TestEffectivePriorityWithTracking/no_tracked_interface_is_down (0.00s)
panic: BUG: ./le rfc discriminate-record disabled this producer to observe the red
```

`rfc/discrimination/rfc9568.json` did not exist before this change: RFC 9568
carried tagged tests and no recorded red at all.

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| accept-mode false vs keepalived | `internal/le/qemu/vrrp_keepalived_linux.go` lab | keepalived | VIP unreachable on the ze Active while election and virtual-MAC ownership are unaffected | |
| `tracked-uplink-hands-the-vip-to-keepalived` | `internal/le/qemu/vrrp_keepalived_linux.go` lab | keepalived 2.3.1 | ze Active at priority 200 with `priority-decrement 150` on a tracked veth; the veth goes down, ze advertises 50, and keepalived at 100 takes the VIP. Another implementation ACTS on the decremented priority, which no ze-only test can show. The lab's existing MASTER-transition detection (its notify script and state marker) is the assertion | PASS in the QEMU guest against keepalived 2.3.1, 2026-09-08, and RED under the reverted decrement. Both outputs are in "QEMU evidence, item 2" above |

### Future (if deferring any tests)
- None. Item 1's tests landed with it in b21f6f2048, and every item 2 row above is written in the same change as the code it covers.
- The new interop scenario is NAMED, per `ai/rules/interop-and-goal-validation.md`. The lab's three existing scenarios are `QS-1`, `QS-2` and `QS-3` (`vrrpScenarioNames`, `internal/le/qemu/guestlabs.go`), which the same rule bans; renaming them is not this spec's work, and the new scenario does not copy the pattern.

## Files to Modify
- `internal/plugins/vrrp/instance.go` - install and remove the filter with the VIPs (`doInstallVIPs` / `doRemoveVIPs`); item 2 adds `trackDown`, `evaluateTracking`, the `rewatch` arm in `run`, the `linkUp` and `watchLinks` deps, the decrement in `fsmConfig`, and `tracked-down` in `snapshot`
- `internal/plugins/vrrp/groups.go` - `TrackedInterface` type and `GroupSpec.TrackedInterfaces`, extraction in `applyGroupLeaves`, the owner and zero-decrement rules in `validateGroup`, the `maxTrackedInterfaces` constant, and the decrement argument on `EffectivePriority`
- `internal/plugins/vrrp/register.go` - `watchParent` becomes `watchLinks`, `linkUp` joins it in `liveDeps`, and `RegisterSuggestion("vrrp-track-interface", ...)` offers the configured interface names for completion
- `internal/plugins/vrrp/vrrp.go` - `tracked-down` in `instanceView`
- `internal/plugins/vrrp/doctor.go` - the `doctor-vrrp-config-invalid` description enumerates what the config can be wrong about, so it gains the owner-with-tracking case
- `internal/plugins/vrrp/yang/ze-vrrp-conf.yang` - drop the accept-mode disclaimer; add the `track` container to BOTH the IPv4 and the IPv6 grouping
- `internal/plugins/vrrp/fsm/events.go` - retire the "snapshot only" comment once the flag drives behavior
- `internal/plugins/vrrp/groups_test.go`, `internal/plugins/vrrp/instance_test.go` - the unit rows above; the `EffectivePriority` call sites in the RFC-tagged `TestOwnerAutoDetection` change with the signature, so `./le rfc discriminate-record` runs for the affected stems
- `internal/le/qemu/vrrp_keepalived_linux.go` - the `tracked-uplink-hands-the-vip-to-keepalived` scenario
- `docs/architecture/testing/qemu-integration.md` - declared as the design of `internal/le/qemu/vrrp_keepalived_linux.go`: the new keepalived scenario joins the page's scenario list
- `internal/test/fixture/routing_fixture_linux.go` - register `vrrp/vrrp-track-setup` and `vrrp/vrrp-track-driver`
- `internal/test/fixture/vrrp_track_linux.go` (new) - the setup builds a veth parent plus a tracked veth; the driver flaps the tracked veth and reads the advertised priority off the parent's peer
- `test/vrrp/vrrp-track.ci` (new), `test/vrrp/vrrp-config-invalid.ci`, `test/vrrp/vrrp-doctor-fires.ci`
- `docs/guide/vrrp.md` - retire the limitation; document the ping consequence; retire "No priority tracking" and add the `track` rows plus a worked example
- `docs/features.md` - the VRRP row still says accept-mode "is not enforced on the dataplane this pass", which commit b21f6f2048 made false; the same row gains tracking
- `docs/architecture/vrrp/vrrp-first-hop-redundancy.md` - the decrement path and the watch that feeds it
- `docs/features/rfc-status.md` - RFC 9568 R014/R030/R031 rows

### Integration Checklist

| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/plugins/vrrp/yang/ze-vrrp-conf.yang`: the `track` container in both groupings |
| YANG validation constraints | Yes | `range "1..254"` and `mandatory true` on `priority-decrement`, `max-elements 16` on the list, `zt:node-name` on the name |
| YANG custom validators | Yes, as a SUGGESTION only | `ze:validate "vrrp-track-interface"` declared through `RegisterSuggestion` in `register.go`, which refuses nothing and only offers the configured interface names (`ai/patterns/config-option.md` step 5b). The cross-leaf rules stay in `validateGroup`, where the sibling context is |
| CLI commands/flags | No | No command is added. `show vrrp` gains one payload field |
| CLI grammar (keyword before value) | Yes | `track interface <name> priority-decrement <n>` keeps keyword before value at every level |
| Editor autocomplete | Yes | The suggestion above feeds the name; `priority-decrement` completes from its YANG range |
| Functional test for new RPC/API | Yes | `test/vrrp/vrrp-track.ci` |
| Pipe completeness | N-A | The `show vrrp` payload is unchanged in shape, so it keeps the pipe handling it has |
| Env var registration | N-A | No leaf under `environment/` |
| Doctor check for runtime dependencies | Yes, through the existing check | `diagnoseSections` re-runs `validateGroups`, so the owner-with-tracking rule reaches `ze doctor`. Its code description is updated; no new code and no new check |
| Prometheus counters/metrics | No | `ze_vrrp_state` is unchanged, and the priority is read from `show vrrp`. A tracked-state gauge is not added, because nothing needs to alert on it that the state gauge does not already carry |
| BGP family surface | N-A | Not BGP |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` VRRP row. **DONE in commit fed8cb495.** The row now states the input-hook drop per virtual address, the Section 6.1 ICMPv6 135/136 carve-out that keeps Neighbor Discovery answering, and the `track` leaf. The other session's Per-protocol FIB row rode along and the commit body discloses it as a forward reference, which is what `ai/rules/git-safety.md` (2026-09-07) requires of an additive foreign hunk. `<!-- source: internal/plugins/vrrp/acceptfilter.go -- RFC 9568 Section 6.4.3 accept-mode filter -->` is the anchor added with it |
| 2 | Config syntax changed? | Yes, in the VRRP page only | `docs/guide/vrrp.md`. `docs/guide/configuration.md` carries no vrrp block (checked 2026-09-08) |
| 3 | CLI command added/changed? | No | No command is added; `docs/guide/command-reference.md` does not enumerate the `show vrrp` fields (checked 2026-09-08) |
| 4 | API/RPC added/changed? | No | The `show vrrp` RPC is unchanged; one field joins its payload |
| 5 | Plugin added/changed? | No | The plugin's registration surface is unchanged |
| 6 | Has a user guide page? | Yes | `docs/guide/vrrp.md`: the leaf table, a worked example, and the "No priority tracking" paragraph retired |
| 7 | Wire format changed? | No | The advertisement carries a different priority VALUE, in the field that already carries it |
| 8 | Plugin SDK/protocol changed? | No | No SDK surface is touched |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc9568.md` rows `RFC9568-5.2.4-1` and `RFC9568-5.2.4-2` gain the decrement path as a proving site, with the discrimination records the change owes |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`: the `vrrp-track` fixtures |
| 11 | Affects daemon comparison? | No, checked 2026-09-08 | `docs/comparison.md` names VRRP nowhere, so it carries no claim this change made false. Adding a VRRP section would be new work rather than a repair |
| 12 | Internal architecture changed? | Yes | `docs/architecture/vrrp/vrrp-first-hop-redundancy.md`: the decrement path, the merged watch, and the fail-closed rule |
| 13 | Route metadata keys added/changed? | N-A | No route metadata |
| 14 | Prometheus counters added/changed? | No | No series is added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | Nothing new registers |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes, re-run after the code landed | `./le spec citation anchors spec plan/immediate/spec-vrrp-deferred-accept-mode-dataplane.md` reports one DECLARED design document, `docs/architecture/testing/qemu-integration.md` (from `internal/le/qemu/vrrp_keepalived_linux.go`), now named under Files to Modify. It also notes `docs/architecture/iface/logical-name-resolution.md`, which mentions `groups.go`: tracking CONSUMES the resolution that page documents and changes none of it, so the implementation adds VRRP tracking to the page's consumer list if it keeps one, and names it unaffected otherwise. Re-run 2026-09-08 after the code landed: `./le spec citation anchors` exits 0, and `./le docs-to-code index-check` names four undeclared anchors, none of them in a page or a file this change touched. `docs/architecture/iface/logical-name-resolution.md` gained the tracked-interface consumer paragraph and a `linkUp, watchLinks` anchor |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/vrrp.md` examples are checked against the YANG after the container lands |

## Implementation Steps

Stage mapping follows `plan/TEMPLATE.md` unchanged.

### Implementation Phases

Phases 1 to 3 landed in commit b21f6f2048. Phase 4 is the work outstanding.

1. **Phase: Wiring (MANDATORY FIRST)** -- filter seam at the install/remove path plus a failing `vrrp-accept-mode.ci`
2. **Phase: Filter rules** -- effective accept-mode to rules, owner and NS/NA carve-outs
3. **Phase: Lifecycle** -- install, remove, reconfigure, restart-safety
4. **Phase: Tracking** -- in this order, each step red first:
   1. YANG `track` container in both groupings, `RegisterSuggestion` for the name, and a failing `test/vrrp/vrrp-track.ci`
   2. `TrackedInterface` and extraction, plus the `validateGroup` rules (owner refused, zero decrement refused, maximum re-checked) and their `.ci` rows
   3. `EffectivePriority(decrement)` with the owner branch first and the floor at 1; update the RFC-tagged call sites and record the discrimination the change owes (`./le rfc discriminate-record`)
   4. `linkUp` and `watchLinks` in `engineDeps` and `liveDeps`, `evaluateTracking`, the `rewatch` arm in `run`, and the decrement in `fsmConfig`
   5. `tracked-down` in `snapshot` and `instanceView`
5. **Functional and interop tests** -- `.ci` coverage plus the keepalived lab re-run
6. **Full verification** -- `./le verify current mode full`
7. **Complete spec** -- audit, learned summary, two-commit closure

### Failure Routing
| Failure | Route To |
|---------|----------|
| Filter breaks ARP/ND ownership | Back to design: A-1 broken, co-design with the sysctl recipe |
| Interop lab reds | Check A-3; scenario config, not the feature, may need the update |
| A tracked interface's link change never reaches the instance | R-6: the watch was made before the tracked set changed. Check the `rewatch` path and the device set comparison in `reconfigure` |
| Advertisements climb with link churn | R-5: `evaluateTracking` is dispatching when the down set did not change |
| A tracked name resolves on the host and not in the QEMU guest | The fixture's veth name, not the feature. Read the setup fixture before touching `linkUp` |
| 3 fix attempts fail | STOP. Report all 3. Ask user. |

## Known Limitations
- Design done 2026-09-08 for both halves. The filter mechanism is the firewall table registry (item 1, landed in b21f6f2048); the tracking surface is the `track` container above; and the halves stay in one spec.
- **Route tracking and health-check tracking are out of scope, and stay unimplemented.** A tracked route needs a watch keyed on a prefix, and a health check needs a script runner with its own timers, output contract and security surface. Ze has neither, so this spec tracks an interface's operational state, which `iface.Resolve` and `iface.Subscribe` already answer. `docs/guide/vrrp.md` says which of the three Ze offers, and does not claim the other two.
- Tracking is refused on the address-owner group rather than accepted and ignored, because an owner advertises 255 (RFC 9568 Section 5.2.4). A shared configuration template that carries `track` and lands on the router that owns the virtual address is refused at commit on that router alone.
- A tracked interface's state is its operational state. A tracked interface that is up but blackholing traffic still counts as up.
- Linux only in scope. The VPP dataplane path belongs to `plan/spec-vrrp-7-vpp.md`, whose R-1 already names accept-mode as a divergence risk.

## RFC Documentation

At implementation: `// RFC 9568 Section 6.4.3` comments on the filter decision
and `// RFC 9568 Section 6.1` on the owner and NS/NA carve-outs; update the
R014/R030/R031 rows in the `rfc/short/rfc9568.md` checklist.

Item 2 is governed by Section 5.2.4, which the module already tags. Tracking
changes the priority a Backup Router advertises, so both requirements below are
enforced on the decrement path and both take a tagged test there:

| Requirement | Section text | Where the decrement path enforces it |
|-------------|--------------|--------------------------------------|
| `RFC9568-5.2.4-1` | "The priority value for the VRRP Router that owns the IPvX address associated with the Virtual Router MUST be 255 (decimal)." | The owner branch of `EffectivePriority` runs before any subtraction, and `validateGroup` refuses `track` on an owner group |
| `RFC9568-5.2.4-2` | "VRRP Routers backing up a Virtual Router MUST use priority values between 1-254 (decimal)." | The floor at 1, plus the YANG range on `priority-decrement` |

`RFC3768-5.3.4-x` and `RFC5798-5.2.4-x` carry the same two obligations, and the
existing tests tag all three families. Tracking is version-independent, so the
boundary test drives a version-3 group and a version-2 group and tags what each
row demonstrates, never more. `TestOwnerAutoDetection` already carries the
`RFC9568-5.2.4-1` pair and calls `EffectivePriority`, so the signature change
touches a tagged unit: `./le rfc discriminate-record` runs for those stems in the
same change (`ai/rules/rfc-compliance.md`).

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by the independent
     /ze-review gate. Round 1 covers commit 0dd2e20da (item 2, interface
     tracking); item 1 (b21f6f2048) is in scope only where tracking touches it. -->

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/vrrp-deferred-accept-mode-dataplane-0a21e591-d035-4f7c-8d1b-231ab023a478.md` (19 files, verdict=clean) |
| `./le spec session review check` | `review_gate: OK (3 code files, clean, hashes match ...)` |
| Rounds | 3. Round 1 found the BLOCKER and four ISSUEs, round 2 found one ISSUE (the staled RFC 5798 discrimination records), round 3 was clean |
| Reviewer lenses used | Round 1: an independent reviewer over commit 0dd2e20da. Rounds 2 and 3: the closure context, with wiring and reachability (every symbol this spec added read to a non-test caller) and evidence integrity (does each test's assertion carry the claim written above it) |

### Run 1 (initial, 2026-09-08, independent reviewer, commit 0dd2e20da)

Counts: 1 BLOCKER, 4 ISSUE, 4 NOTE.

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | BLOCKER | `docs/features.md` VRRP row still publishes "`accept-mode` ... is not enforced on the dataplane this pass, so the virtual address answers traffic while the router is Active regardless of the leaf (RFC 9568 Section 6.4.3 filtering is not installed)". `acceptFilterTables` and `setAcceptFilter` install a `ze_vrrp` input-hook table with one host-scoped Drop per suppressed address, so the page has been false since b21f6f2048, and it is the page that publishes Ze's RFC 9568 conformance to a reader outside the repo. The row also does not carry tracking. The spec's own Documentation Update Checklist row 1 records it as NOT DONE, on the ground that another session held an uncommitted hunk in that file; `ai/rules/git-safety.md` (2026-09-07) makes landing the presumption for a foreign hunk that is additive prose, and rung 2 of `ai/rules/rule-precedence.md` puts an outward-facing false conformance claim above commit tidiness | `docs/features.md` VRRP row; producer `internal/plugins/vrrp/acceptfilter.go` `acceptFilterTables` | CLEARED in commit fed8cb495 -- see Fixes applied |
| 2 | ISSUE | The keepalived lab's recovery leg cannot fail. `assertZeAdvertPriority` scans `zeAdverts()`, which is `parseVRRPAdverts(l.captureLines.snapshot())`, the CUMULATIVE capture since `startCapture`. In `runTrackedUplink` the third call asks for `vrrpZePriority` (200), and the capture already holds the 200-priority advertisements from before the tracked veth went down, so it matches at once whether or not Ze withdrew the decrement. The spec's RED walk never exposed it, because the reverted decrement failed at leg 2 and never reached leg 3. The recovery is still proven, but by the `waitKAState(ctx, "BACKUP")` that follows, not by the priority assertion whose failure message claims to report it. Fix: record `len(l.zeAdverts())` before each flap and require a match at or past that index | `internal/le/qemu/vrrp_keepalived_linux.go` `assertZeAdvertPriority`, `runTrackedUplink` | CLEARED -- see Fixes applied |
| 3 | ISSUE | `trackedInterfaces` returns `nil, nil` when `track`, or its `interface` child, is not a `map[string]any`. The same function hard-errors on a malformed ENTRY and on a missing `priority-decrement`, both justified in comments by "a producer that skipped schema validation". From that same producer a malformed container yields NO tracking, silently: the group keeps advertising its configured priority for ever, the failover the operator wrote never happens, and nothing is logged. That is the zero that reads as an answer (`ai/rules/principles.md`). Fix: keep `nil, nil` only for an ABSENT `track` key, and error when the key is present in a shape the extractor does not understand | `internal/plugins/vrrp/groups.go` `trackedInterfaces` | CLEARED -- see Fixes applied |
| 4 | ISSUE | The Consequences bullet this commit rewrote states the filter order backwards: "installs a drop for the virtual addresses in the `ze_vrrp` firewall table, ahead of the ICMPv6 135/136 carve-out RFC 9568 Section 6.1 requires". `acceptFilterTables` appends the `nd-neighbor-solicit` and `nd-neighbor-advert` Accept terms FIRST and the per-address Drop terms after, and its own comment says why: a packet takes the verdict of the first rule it matches. The page now describes the order that would violate R014 | `docs/architecture/vrrp/vrrp-first-hop-redundancy.md` Consequences; producer `internal/plugins/vrrp/acceptfilter.go` `acceptFilterTables` | CLEARED -- see Fixes applied |
| 5 | ISSUE | RFC claim wider than the assertion. `TestEffectivePriorityWithTracking` tags `RFC9568-5.2.4-2 positive` with "a backing-up router's advertised priority stays within 1-254 ... and it never reaches 0 or 255". No case in the table produces or excludes 255, and the producer does not prevent it: `GroupSpec{Priority: 255, IsOwner: false}.EffectivePriority(0)` returns 255. What keeps a non-owner off 255 is `validateGroup`'s 1..254 priority range, in another function. `rfc/requirements/rfc9568.md` now lists this test as a positive proving site for the requirement, so the over-claim is on the public ledger (`ai/rules/rfc-compliance.md`, "The claim MUST state what the test body checks, and MUST NOT state more"). Fix: drop "or 255" from the claim, or add the case and the assertion that earns it | `internal/plugins/vrrp/groups_test.go` `TestEffectivePriorityWithTracking`; `rfc/requirements/rfc9568.md` `RFC9568-5.2.4-2` | CLEARED -- see Fixes applied |
| 6 | NOTE | `evaluateTrackingLocked` logs at Info "vrrp: tracked interface state changed, advertising a new priority" and then returns without dispatching when `!in.started`. Nothing is advertised on that path, and the operator reading the log is told otherwise | `internal/plugins/vrrp/instance.go` `evaluateTrackingLocked` | open |
| 7 | NOTE | `watchDevices` comments that "a tracked interface that IS the parent is subscribed once". The dedup compares `spec.ParentDevice`, which `engine.apply` fills with the RESOLVED kernel device (`internal/plugins/vrrp/engine.go`), against `TrackedInterface.Name`, which is unresolved. Tracking the parent by a logical name that an os-name selector maps elsewhere subscribes twice. Harmless, because the wake-up is coarse and re-decided from state; the comment states more than the code delivers | `internal/plugins/vrrp/instance.go` `watchDevices` | open |
| 8 | NOTE | `trackedDownLocked` calls `in.deps.linkUp` with no nil guard, while `parentReady`, `watchLinks` and `refreshAddresses` are each guarded in the same file. Unreachable today: `liveDeps` is the only production construction of `engineDeps`. A second producer that omits `linkUp` panics on the first group carrying `track` | `internal/plugins/vrrp/instance.go` `trackedDownLocked` | open |
| 9 | NOTE | `test/vrrp/vrrp-config-invalid.ci` seq=13 asserts a zero `priority-decrement` is refused with `expect=stdout:contains=priority-decrement`, which the YANG `range "1..254"` message also satisfies. The spec's own RED walk (removing `validateTracking` from `validateGroup`) reported only seq=12 red, so seq=13 is proven by the schema and does not drive `validateTracking`'s range branch from the entry point. That branch is covered at `validateGroups` by `TestTrackedInterfaceRejectsAnUnusableDecrement` | `test/vrrp/vrrp-config-invalid.ci` seq=13 | open |

### Checked and clean

Stated explicitly, because the absence of a finding is a result:

- **Decrement arithmetic.** No defect. `EffectivePriority` runs the owner branch
  before any subtraction, and its floor test `uint16(g.Priority) <= decrement +
  backupPriorityMin` returns 1 exactly where `Priority - decrement` would be 1
  or less. No overflow: the summed decrement is bounded by 16 x 254 = 4064,
  inside `uint16`.
- **Order independence.** Several tracked interfaces going down and returning in
  any order reach the same answer. `trackedDownLocked` rebuilds the down set
  from `spec.TrackedInterfaces`, which `trackedInterfaces` sorts by name, so
  `slices.Equal(down, in.trackDown)` is order-stable, and
  `trackDecrementLocked` sums over the CURRENT spec, so a name left in
  `trackDown` by a superseded config contributes nothing.
- **The unresolvable device is written as a decision.** `linkUp` returns
  `(bool, error)` rather than folding the resolver failure into false, and
  `trackedDownLocked` appends the name to the down set and logs at Warn. It
  cannot be confused with a device nobody configured, because the loop runs over
  the configured entries only.
- **Goroutine lifecycle.** No leak and no double start. `watch()` is called only
  from `run`'s own goroutine, cancels the previous subscription before
  re-subscribing, and `run` defers the final cancel. `forwardLinkEvents` is one
  long-lived goroutine for each watched device and returns on `done` or on its
  subscription closing; cancel closes `done`, cancels each subscription, then
  waits. `rewatch` is buffered by one with a default arm, so `reconfigure` never
  blocks and two signals collapse into one re-subscription. No path closes
  `done` twice.
- **Refusal reachability.** `validateGroup` to `validateTracking` is reached from
  every entry point that accepts config: `verifyVRRPConfigSections`
  (`internal/plugins/vrrp/register.go`, `ze config validate` and commit),
  `parseAndVerify` (same file, the apply path) and `diagnoseSections`
  (`internal/plugins/vrrp/doctor.go`). Not only the one the test drives.
- **No test deleted or weakened.** The `TestOwnerAutoDetection` edit is arity
  only: four call sites read `EffectivePriority(0)`, both expected values (255
  and 100) are unchanged, and `test/rfc-changed/ff31c770.md` carries the owner
  approval.
- **The `.ci` RED is caused by the behavior under test.**
  `vrrpTrackAdvertPriority` reads the IPv4 IHL and the protocol byte before
  indexing, and takes the priority at VRRP header offset 2, which RFC 9568
  Section 5.2 puts after Version/Type and the Virtual Router ID.
  `vrrpTrackWaitPriority` consumes from a live AF_PACKET socket, so it cannot
  match a stale frame from an earlier leg, which is the defect finding 2 records
  in the keepalived lab.

### Fixes applied

Round 1 fixes, 2026-09-08. All five are cleared.

- **Finding 1 (the BLOCKER, a false RFC 9568 conformance claim).** The
  `docs/features.md` VRRP row said accept-mode "is not enforced on the dataplane
  this pass", which b21f6f2048 made false on the page that publishes Ze's
  conformance outward. Corrected in commit fed8cb495: the row now states the
  host-scoped drop each virtual address takes at the input hook, the Section 6.1
  carve-out that keeps IPv6 Neighbor Solicitation and Advertisement flowing, and
  the `track` leaf 0dd2e20da added. The producer is `acceptFilterTables`
  (`internal/plugins/vrrp/acceptfilter.go`), which the row's new
  `<!-- source: -->` anchor names. The other session's uncommitted Per-protocol
  FIB row rode along and the commit body discloses it as a forward reference,
  which is the presumption `ai/rules/git-safety.md` (2026-09-07) sets for an
  additive foreign hunk, and rung 2 of `ai/rules/rule-precedence.md` puts an
  outward-facing false conformance claim above commit tidiness.

- **Finding 3 (fail closed).** `trackedInterfaces`
  (`internal/plugins/vrrp/groups.go`) now returns `nil, nil` only for an ABSENT
  `track` key and for a container carrying no `interface` child. A `track` value
  that is not a configuration node, and an `interface` child that is not one,
  each return an error naming what the node holds. The comment states which
  shape is an answer and which is a refusal. RED first:
  `TestTrackedInterfacesRejectAMalformedContainer`
  (`internal/plugins/vrrp/groups_test.go`) failed on both malformed shapes
  ("a track container the extractor cannot read must be refused") before the
  producer changed, and `./le job run label unit-vrrp command go test
  ./internal/plugins/vrrp/... -count=1` is green after it.
- **Finding 2 (the recovery leg could not fail).** `assertZeAdvertPriority`
  (`internal/le/qemu/vrrp_keepalived_linux.go`) takes a `from` index and scans
  only `adverts[from:]`; `runTrackedUplink` records `advertsBefore =
  len(l.zeAdverts())` immediately before each of the three actions it then
  asserts on. Proven by a deliberate break rather than by reading the code: with
  `trackedDownLocked` (`internal/plugins/vrrp/instance.go`) keeping a name in
  the down set for ever, so the decrement is never withdrawn, the guest run
  FAILED on the recovery leg alone -- `FAIL:
  tracked-uplink-hands-the-vip-to-keepalived: after the tracked link returned:
  no ze-sourced advert reached the observer after this leg began, so priority
  200 was never observed`, with legs 1 and 2 still reporting their detail lines.
  The break was reverted, the guest daemon rebuilt from the same overlay, and
  the same command PASSED across all three legs. Both runs:
  `./le qemu run kernel tmp/kernel/build/vmlinuz packages "keepalived tcpdump
  iproute2 libcap iputils" command "... ./le qemu vrrp-keepalived-test scenarios
  tracked-uplink-hands-the-vip-to-keepalived"`, keepalived 2.3.1 in the guest.
- **Finding 4 (filter order stated backwards).** The Consequences bullet in
  `docs/architecture/vrrp/vrrp-first-hop-redundancy.md` now says the ICMPv6
  135/136 carve-out is installed FIRST and the per-address drops after it,
  because a packet takes the verdict of the first term it matches. That is the
  order `acceptFilterTables` (`internal/plugins/vrrp/acceptfilter.go`) appends
  its terms in, and its own comment gives the same reason.
- **Finding 5 (claim wider than the assertion).** The `RFC9568-5.2.4-2 positive`
  tag on `TestEffectivePriorityWithTracking` no longer says "or 255": it claims
  the floor at 1 only, which is what the table asserts. Nothing left the ledger:
  `TestBoundaryPriority` proves both polarities of `RFC9568-5.2.4-2` from the
  config entry point, 255 included, and the tracking test's doc comment now
  names it. The reworded claim staled its discrimination record, so
  `./le rfc discriminate-record id RFC9568-5.2.4-2 polarity positive unit
  internal/plugins/vrrp/groups_test.go::TestEffectivePriorityWithTracking route
  revert producer internal/plugins/vrrp/groups.go::EffectivePriority` observed
  the red again and rewrote `rfc/discrimination/rfc9568.json`.

### Run 2 (closure pass, 2026-09-08, independent closure context)

Scope, written before the run (`ai/rules/planning.md`): the round 1 fixes and
their sibling call sites, which is commit 3048d488a
(`internal/plugins/vrrp/groups.go` `trackedInterfaces`,
`internal/le/qemu/vrrp_keepalived_linux.go` `assertZeAdvertPriority` and
`runTrackedUplink`, `docs/architecture/vrrp/vrrp-first-hop-redundancy.md`
Consequences, the `RFC9568-5.2.4-2` claim on
`internal/plugins/vrrp/groups_test.go` `TestEffectivePriorityWithTracking`) plus
commit fed8cb495 (`docs/features.md`), and the eight always-in-scope classes
over the whole change. Lenses: wiring and reachability (every symbol this spec
added, read to a non-test caller), and evidence integrity (does each test's
assertion carry the claim written above it).

Counts: 0 BLOCKER, 1 ISSUE, 0 NOTE.

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 10 | ISSUE | Six RFC 5798 discrimination records were staled by this spec's own change and were not re-recorded with it. `validateGroup` gained the `validateTracking` call in 0dd2e20da, which moved its fingerprint, and the six records whose `producer` is `validateGroup` still carried `producer-sha` `fe11239c0b3b84bf`. `./le rfc check` named all six: `RFC5798-5.2.4-2` at `TestBoundaryPriority`, `RFC5798-5.2.9-1` at `TestValidateIPv6LinkLocal` and `RFC5798-5.2.9-2` at `TestValidateVIPFamilyMatchesGroupFamily`, each in both polarities. So three RFC 5798 MUSTs had no red anybody has observed over the code that is there now. Attribution is exact: the function body is byte-identical from 57812eae56 (which recorded the six) through `0dd2e20da^`, and differs at HEAD. `ai/rules/rfc-compliance.md` names the tag and the tagged unit as the two things whose change owes a record; the PRODUCER moving under an unchanged tag and an unchanged unit is the third way, and it is the one no sentence in the rule reaches | `rfc/discrimination/rfc5798.json`; producer `internal/plugins/vrrp/groups.go` `validateGroup` | CLEARED -- see Fixes applied, round 2 |

### Fixes applied, round 2

- **Finding 10 (staled discrimination records).** All six were re-recorded with
  `./le rfc discriminate-record ... route revert producer
  internal/plugins/vrrp/groups.go::validateGroup`, which applies the break,
  observes the red and refuses to write a record it did not observe. Each run
  reported `observed red`, for example
  `rfc/discrimination/rfc5798.json: recorded RFC5798-5.2.4-2 negative at
  internal/plugins/vrrp/groups_test.go::TestBoundaryPriority by revert`. No test
  and no product code changed: the six records now pin the fingerprints the
  proofs were actually taken against.

### Run 3 (re-review of the round 2 fix, 2026-09-08)

Scope: `rfc/discrimination/rfc5798.json`, the only file round 2 changed, plus
the always-in-scope classes.

Counts: 0 BLOCKER, 0 ISSUE, 0 NOTE.

`./le rfc check` names no record in `rfc/discrimination/rfc5798.json`, where it
named six before the fix. The file is tool-written and no hand edit was made to
it, so there is no claim in it that a run did not produce. The three RFC 5798
rows the check still reports (`RFC5798-5.1.2.3-1`, `RFC5798-7.4-1`,
`RFC5798-8.4.2-1`, each "has no test and no annotation") name the IPv6 hop
limit, RFC 7217 interface identifiers and the Section 8.4.2 interop mode. None
is reachable from accept-mode filtering or from priority tracking, and all three
predate this spec.

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

Run 3 is the clean run: 0 BLOCKER, 0 ISSUE. The four NOTEs of round 1 (findings
6 to 9) stay recorded and open, which is what a NOTE is for; none of them can
produce a wrong result, and each names the code that would have to change if it
ever did.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-13 all demonstrated
- [ ] Wiring Test table complete, every row a concrete test
- [ ] `/ze-review` gate clean (0 BLOCKER, 0 ISSUE)
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/plugins/vrrp/`)
- [ ] Documentation Update Checklist answered with source evidence

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Interop tests for protocol features
- [ ] Goal Validation table filled

### Completion (BLOCKING -- before ANY commit)
- [ ] Implementation Summary and Audit filled
- [ ] Learned summary written
- [ ] Two-commit closure per `ai/rules/planning.md`

## Work Inherited From a Deferral Row

<!-- The deferral directory was deleted on 2026-09-05. A row that named this spec as
     its destination is reproduced here, so the item and the reasoning behind it
     survive the directory. Each row is outstanding work this spec owns. -->

### From `vrrp-6-interop.md`, 2026-07-15

Deferred by spec-vrrp-6-interop (Known Limitations).

`accept-mode` is not enforced on the dataplane: the leaf is parsed, validated (rejected under v2) and reported by `show vrrp`, but no RFC 9568 6.4.3 filtering is installed, so an Active non-owner answers traffic to the virtual IP whichever way the leaf is set. Also not implemented: priority-decrement tracking (interface/route/health) that Junos/Nokia/VyOS offer

Both halves of that row are now done, and their limits are stated in Known
Limitations: interface tracking is implemented, route and health-check tracking
are not.

## Implementation Summary

### What Was Implemented

Two pieces, in two commits, each with its own tests.

**Item 1, the Accept_Mode dataplane filter (commit b21f6f2048).**
`AcceptMode` reached config parsing, the version-2 rejection and the `show vrrp`
snapshot and nothing else, so an Active non-owner answered traffic on the
virtual address whichever way the operator set the leaf.
`internal/plugins/vrrp/acceptfilter.go` is the new consumer.
`setAcceptFilter` records one instance's Section 6.4.3 decision,
`acceptFilterTables` turns the suppressed address set into one `ze_vrrp` inet
table with a base chain at the input hook, and `acceptFilterPublish` hands it to
`firewall.RegisterTables` plus `firewall.ApplyAll`, the same table registry
copp, ddos-local and flowspec-firewall use. VRRP gained no nftables code and no
Linux-only file. The two ICMPv6 Accept terms (types 135 and 136) are appended
BEFORE the per-address Drop terms, because a packet takes the verdict of the
first rule it matches, which is what makes the Section 6.1 carve-out work.
`doInstallVIPs` (`internal/plugins/vrrp/instance.go`) installs the filter before
the address and `doRemoveVIPs` withdraws it after, so neither end leaves a
window. The address itself stays installed: the same section requires the Active
router to answer ARP and Neighbor Solicitations, and on Linux both follow from
the address being present.

**Item 2, interface tracking (commit 0dd2e20da).** A group takes
`track { interface <name> { priority-decrement <1..254>; } }` in both the IPv4
and the IPv6 grouping of `internal/plugins/vrrp/yang/ze-vrrp-conf.yang`.
`trackedInterfaces` (`internal/plugins/vrrp/groups.go`) extracts the list sorted
by name, `validateTracking` refuses it on the address-owner group and re-checks
the range and the 16-entry maximum, and `GroupSpec.EffectivePriority(decrement)`
runs the owner branch before any subtraction and floors at 1.
`evaluateTracking` (`internal/plugins/vrrp/instance.go`) re-reads every tracked
interface through `deps.linkUp`, rebuilds the down set, and dispatches
`fsm.ConfigUpdated` only when that set CHANGED. `watchLinks`
(`internal/plugins/vrrp/register.go`) merges the parent and every tracked device
into one subscription, and `reconfigure` signals `rewatch` when the device set
differs, so an interface a commit starts tracking is watched from that commit.
`show vrrp` gained `tracked-down`.

### Bugs Found/Fixed

- The `vrrp-track` fixture read the VRRP header at offset 1, which is the vrid,
  not the priority. It reported 12 for every advertisement and would have passed
  for any vrid equal to the expected priority. RFC 9568 Section 5.2 puts Priority
  at offset 2; `vrrpTrackPriorityByte` is now 2
  (`internal/test/fixture/vrrp_track_linux.go`), and the test then found the
  real defect it was written for.
- `trackedInterfaces` returned `nil, nil` for a `track` container it could not
  read, so a malformed shape produced a group that tracks nothing, advertises
  its full priority for ever and logs nothing. Now covered by
  `TestTrackedInterfacesRejectAMalformedContainer`
  (`internal/plugins/vrrp/groups_test.go`).
- The keepalived lab's recovery leg could not fail: `assertZeAdvertPriority`
  scanned the cumulative capture, and the recovery asks for 200, which the
  capture already held. Each leg now records a baseline and reads only what
  arrived after its own action.
- Six RFC 5798 discrimination records were staled by `validateGroup` gaining the
  `validateTracking` call and were not re-recorded with it. Found at closure,
  fixed there; the journal row is in
  `plan/journal/claim-outlives-the-evidence-it-cites.md`.

### Documentation Updates

- `docs/features.md` VRRP row (commit fed8cb495): the enforced filter, the
  Section 6.1 carve-out, and `track`. Anchor
  `<!-- source: internal/plugins/vrrp/acceptfilter.go -- RFC 9568 Section 6.4.3 accept-mode filter -->`.
- `docs/guide/vrrp.md`: the `accept-mode` and `track` leaf rows, the "Tracking an
  interface" section with a worked example and its four rules, and the
  Limitations paragraph rewritten to say Ze tracks an interface and not a route
  or a script.
- `docs/architecture/vrrp/vrrp-first-hop-redundancy.md`: "Priority tracking reads
  one state, and dispatches only on a change", plus the Consequences bullet that
  now states the ICMPv6 carve-out is installed FIRST.
- `docs/architecture/iface/logical-name-resolution.md`: the tracked interface as
  the second name VRRP resolves, and the fail-closed reading of a resolver error.
- `docs/architecture/testing/qemu-integration.md`: the
  `tracked-uplink-hands-the-vip-to-keepalived` scenario row, with a
  `<!-- source: internal/le/qemu/vrrp_keepalived_linux.go -- runTrackedUplink -->`
  anchor.
- `docs/functional-tests.md`: the `vrrp-track` fixtures and what the two kernel
  proofs read.
- `internal/plugins/vrrp/yang/ze-vrrp-conf.yang`: the `accept-mode` disclaimer is
  gone from both groupings and the description states the enforcement.
- `internal/plugins/vrrp/fsm/events.go`: the "stored for the state snapshot only"
  comment on `Config.AcceptMode` is replaced by the path it now drives.
- `rfc/short/rfc9568.md`: the `RFC9568-6.1-1` row lost its `{not-applicable}`,
  which said Ze installed no filter at all, and `RFC9568-6.4.3-6` and
  `RFC9568-6.4.3-7` are live.

`./le doc check verify` is RED on this checkout and no row of it is this spec's.
465 commands fail identically against `../gh-pages/reference/command-equivalents/`
(a generated site surface in a sibling worktree), and the two other failures are
`docs/DESIGN.md` missing the `firewall-domain` plugin and the wiki command
catalog. This spec added no command.

### Deviations from Plan

- `docs/features/rfc-status.md` is named under Files to Modify and was NOT hand
  edited, deliberately: it is generated by `./le rfc index-update` and a hand
  edit to it is destroyed at the next run. The RFC 9568 rows are authored in
  `rfc/short/rfc9568.md`, which is where the change landed.
- The Data Flow section left the filter mechanism open ("nftables via the
  firewall component, socket filter, or per-device sysctl"). Design picked the
  firewall table registry, and A-2 records the reshape: the rule is scoped to
  the ADDRESS rather than the device, because Section 6.4.3 names no ingress
  interface.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The Documentation Update Checklist recorded row 1 as "NOT DONE, and deliberately", on the ground that another live session held an uncommitted hunk in `docs/features.md` | `ai/rules/git-safety.md` (2026-09-07) makes landing the presumption for an additive foreign hunk, and rung 2 of `ai/rules/rule-precedence.md` puts an outward-facing false conformance claim above commit tidiness. The page had been publishing "accept-mode is not enforced on the dataplane" since b21f6f2048 | The independent review of 0dd2e20da raised it as the round 1 BLOCKER | Fixed in commit fed8cb495, which carries the foreign hunk and discloses it in the body as a forward reference |
| escalation | The change re-recorded the discrimination records whose PRODUCER was `EffectivePriority` and missed the six whose producer was `validateGroup` | A discrimination record pins the producer's fingerprint as well as the unit's and the claim's, so an additive edit to a function that some record names as its producer stales that record with no test red anywhere | `./le rfc check` at closure | Fixed at closure, and the class row is in `plan/journal/claim-outlives-the-evidence-it-cites.md`. The check to run before a commit is `grep -rl <function> rfc/discrimination/` |

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Per-VIP filtering installed on promotion, removed on demotion | Done | `doInstallVIPs`, `doRemoveVIPs` (`internal/plugins/vrrp/instance.go`); `setAcceptFilter`, `clearAcceptFilter` (`internal/plugins/vrrp/acceptfilter.go`) | Filter before the address on install, after it on withdrawal |
| The Section 6.1 owner exemption | Done | `GroupSpec.EffectiveAcceptMode` (`internal/plugins/vrrp/groups.go`), read by `doInstallVIPs` | `TestActiveAddressOwnerAcceptsWhateverAcceptModeSays` |
| The R014 carve-out: never drop IPv6 NS/NA | Done | `acceptFilterTables` (`internal/plugins/vrrp/acceptfilter.go`), the two Accept terms appended before every Drop | `TestAcceptFilterAcceptsNeighborDiscoveryBeforeAnyDrop`, and `ND-CARVE-OUT-BEFORE-DROP` reads the order out of the live kernel |
| A tagged test per requirement row | Done | `rfc/requirements/rfc9568.md` rows `RFC9568-6.1-1`, `RFC9568-6.4.3-6`, `RFC9568-6.4.3-7`, `RFC9568-5.2.4-1`, `RFC9568-5.2.4-2` | Both polarities on each |
| A QEMU integration test | Done | `test/vrrp/vrrp-accept-mode.ci`, `test/vrrp/vrrp-track.ci` | Both `needs-linux:caps=net-admin` |
| The YANG description stops disclaiming the gap | Done | `internal/plugins/vrrp/yang/ze-vrrp-conf.yang`, both `accept-mode` leaves | |
| `RFC9568-6.1-1` re-classified once the filter exists | Done | the `RFC9568-6.1-1` row of `rfc/short/rfc9568.md` | The `{not-applicable}` reason ("ze installs no such filter at all") is gone and the requirement is proven in both polarities |
| Priority-decrement tracking | Done | `TrackedInterface`, `validateTracking`, `EffectivePriority` (`internal/plugins/vrrp/groups.go`); `evaluateTracking`, `trackedDownLocked`, `trackDecrementLocked` (`internal/plugins/vrrp/instance.go`); `linkUp`, `watchLinks`, `trackableDevices` (`internal/plugins/vrrp/register.go`) | Interface state only; route and health tracking are in Known Limitations |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestActiveNonOwnerWithAcceptModeFalseSuppressesLocalDelivery` (`internal/plugins/vrrp/acceptfilter_test.go`); `test/vrrp/vrrp-accept-mode.ci` `VIP-NOT-ACCEPTED accept-mode-false` | The `.ci` proof is a UDP datagram to the virtual address on a live kernel, so it reads local delivery rather than the rule |
| AC-2 | Done | `TestActiveNonOwnerWithAcceptModeTrueAcceptsLocalDelivery`; `test/vrrp/vrrp-accept-mode.ci` `VIP-ACCEPTED accept-mode-true` | The `.ci` flips back to false afterwards, so acceptance is proven to FOLLOW the leaf rather than to be switched on once |
| AC-3 | Done | `TestActiveAddressOwnerAcceptsWhateverAcceptModeSays` | Asserts both halves: one filter call with `accept` true, and `in.snapshot().AcceptMode` true |
| AC-4 | Done | `TestAcceptFilterAcceptsNeighborDiscoveryBeforeAnyDrop`; `test/vrrp/vrrp-accept-mode.ci` `ND-CARVE-OUT-BEFORE-DROP` | The observable is the RULE ORDER, read back out of the live kernel ruleset by `vrrpAcceptRuleOrder`, not an NS/NA exchange. That is the whole of what the filter can break: the NA itself is the kernel's and follows from the address being installed, which `ACTIVE-WITH-VIP` proves in the same run |
| AC-5 | Done | `TestAcceptFilterInstalledBeforeTheAddressAndWithdrawnAfterIt`, `TestAcceptFilterShareTheTableAndWithdrawIndependently`; `test/vrrp/vrrp-accept-mode.ci` `TEARDOWN-COMPLETE` | |
| AC-6 | Done | `TestTrackedInterfaceDownDecrementsTheAdvertisedPriority` (`internal/plugins/vrrp/instance_test.go`); `test/vrrp/vrrp-track.ci` `ADVERT-PRIORITY 50 tracked-interface-down` | The `.ci` reads the priority byte off an AF_PACKET capture, which is what a neighbor elects on |
| AC-7 | Done | `TestTrackedInterfaceUpRestoresThePriority`; `test/vrrp/vrrp-track.ci` `ADVERT-PRIORITY 200 tracked-interface-restored` | |
| AC-8 | Done | `TestTrackedInterfaceDownDecrementsTheAdvertisedPriority`, second leg: both tracked interfaces down reads 120 | |
| AC-9 | Done | `TestEffectivePriorityWithTracking` (`internal/plugins/vrrp/groups_test.go`), cases "the decrement equals the priority" and "the decrement passes the priority", both wanting 1 | Tagged `RFC9568-5.2.4-2 positive` |
| AC-10 | Done | `TestTrackOnOwnerGroupIsRejected`; `test/vrrp/vrrp-config-invalid.ci` seq=12; `test/vrrp/vrrp-doctor-fires.ci` | `diagnoseSections` (`internal/plugins/vrrp/doctor.go`) re-runs `validateGroups`, so one rule serves both |
| AC-11 | Done, with one half unasserted | `TestUnresolvableTrackedInterfaceCountsAsDown` | The test asserts the decrement is applied and the name appears in `tracked-down`. The Warn line itself is not asserted: `trackedDownLocked` (`internal/plugins/vrrp/instance.go`) emits it and no test reads the log. The behavior AC-11 exists to protect is the fail-closed decrement, and that is asserted |
| AC-12 | Done | `TestTrackingDoesNotAdvertiseWhenNothingChanged` | Two unchanged wake-ups before and two after the one real change, so the count is pinned on both sides |
| AC-13 | Done | `TestReconfigureWatchesANewlyTrackedInterface` | Also asserts the negative: a commit that changes no device set does not re-subscribe |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| The four `acceptfilter_test.go` table rows | Done | `internal/plugins/vrrp/acceptfilter_test.go` | 13 test functions in the file |
| `TestEffectivePriorityWithTracking` | Done | `internal/plugins/vrrp/groups_test.go` | Claim narrowed in round 1 fix 5 |
| `TestTrackedInterfacesExtracted` | Done | `internal/plugins/vrrp/groups_test.go` | |
| `TestTrackedInterfacesRejectAMalformedContainer` | Done, ADDED at round 1 | `internal/plugins/vrrp/groups_test.go` | Not in the plan; it covers the fail-closed fix |
| `TestTrackedInterfaceRejectsAnUnusableDecrement`, `TestTrackedInterfaceRequiresADecrement` | Done | `internal/plugins/vrrp/groups_test.go` | |
| `TestTrackOnOwnerGroupIsRejected` | Done | `internal/plugins/vrrp/groups_test.go` | |
| `TestTrackedInterfaceDownDecrementsTheAdvertisedPriority`, `TestTrackedInterfaceUpRestoresThePriority` | Done | `internal/plugins/vrrp/instance_test.go` | |
| `TestUnresolvableTrackedInterfaceCountsAsDown` | Done | `internal/plugins/vrrp/instance_test.go` | |
| `TestTrackingDoesNotAdvertiseWhenNothingChanged` | Done | `internal/plugins/vrrp/instance_test.go` | |
| `TestReconfigureWatchesANewlyTrackedInterface` | Done | `internal/plugins/vrrp/instance_test.go` | |
| `vrrp-accept-mode.ci`, `vrrp-track.ci`, `vrrp-config-invalid.ci`, `vrrp-doctor-fires.ci` | Done | `test/vrrp/` | The first two skip on darwin and run in the QEMU nightly |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| Every file under Files to Modify | Done | Verified present and carrying the named change, except the one row below |
| `docs/features/rfc-status.md` | Changed | Generated by `./le rfc index-update`; the edit landed in `rfc/short/rfc9568.md`, which is the authored source. Recorded under Deviations |

### Audit Summary
- **Total items:** 13 acceptance criteria, 8 task requirements, 11 test rows
- **Done:** all of them
- **Partial:** none
- **Skipped:** none
- **Changed:** 1 (`docs/features/rfc-status.md`, recorded in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Close the open RFC 9568 `RFC9568-6.4.3-7` MUST NOT violation: an Active non-owner with Accept_Mode False must not accept packets addressed to the virtual addresses | functional, against a live kernel | `test/vrrp/vrrp-accept-mode.ci` in the QEMU guest: `ACTIVE-WITH-VIP` (the address IS installed on the virtual-MAC macvlan), then `VIP-NOT-ACCEPTED accept-mode-false`, `VIP-ACCEPTED accept-mode-true`, `VIP-NOT-ACCEPTED accept-mode-false-again`. The probe is a UDP datagram to the virtual address, so what is read is local delivery and not the rule text. The `RFC9568-6.4.3-7` row of `rfc/short/rfc9568.md` carries no `{gap}` |
| Keep the R014 carve-out: IPv6 NS and NA are never dropped with Accept_Mode False | functional, kernel rule order | `ND-CARVE-OUT-BEFORE-DROP` in the same run reads the ruleset back and requires both ICMPv6 accepts to precede the first drop (`vrrpAcceptRuleOrder`, `internal/test/fixture/vrrp_accept_mode_linux.go`). `RFC9568-6.1-1` lost the `{not-applicable}` that said no filter existed |
| Do not break the virtual-MAC ARP/ND recipe | functional | A-1, confirmed 2026-08-29: the address stays on the virtual-MAC macvlan while ping to it is 100% loss, and `accept-mode true` restores the reply. `ACTIVE-WITH-VIP` is that assertion inside the `.ci` |
| A tracked interface going down lowers the priority Ze advertises, so a router that loses its uplink hands the virtual address over | interop, against keepalived 2.3.1 | `tracked-uplink-hands-the-vip-to-keepalived` (`./le qemu vrrp-keepalived-test`), PASS 2026-09-08: ze holds the VIP at 200, the tracked veth goes down, ze advertises 50 and keepalived at 100 takes the VIP, the veth returns and keepalived goes back to BACKUP. keepalived's own log carries the election it made on Ze's priority. RED under the reverted decrement: `ze's advertised priority stayed at 200, want 50` |
| The decrement reaches the WIRE, not only the configuration | functional, wire capture | `test/vrrp/vrrp-track.ci` in the QEMU guest reads the priority byte off an AF_PACKET capture on the parent's veth peer: `ADVERT-PRIORITY 200 tracked-interface-up`, then `50 tracked-interface-down`, then `200 tracked-interface-restored`. RED under the reverted decrement, with the plugin logging the tracked interface as down in the same run |
| A backing-up router never advertises 0 or 255 | unit, boundary table, with a recorded red | `TestEffectivePriorityWithTracking` for the floor at 1, `TestBoundaryPriority` for the 1..254 range from the config entry point in both polarities. Discrimination records in `rfc/discrimination/rfc9568.json` and `rfc/discrimination/rfc5798.json`, each observed red under a break of its producer |
| Link churn does not turn into advertisements | unit | `TestTrackingDoesNotAdvertiseWhenNothingChanged`: two unchanged wake-ups before and two after one real change, and the advertisement count moves by exactly one |
| An operator can find out what happened | functional | `show vrrp` reports `effective-priority` beside the configured `priority` and lists `tracked-down`, asserted in `TestTrackedInterfaceDownDecrementsTheAdvertisedPriority`. `docs/guide/vrrp.md` "Tracking an interface" documents both |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| Route tracking and health-check tracking | A tracked route needs a watch keyed on a prefix and a health check needs a script runner with its own timers, output contract and security surface. Ze has neither, so building one was out of this spec's scope from the design on 2026-09-08. Recorded in Known Limitations and in `docs/guide/vrrp.md`, which says which of the three Ze offers and does not claim the other two | None. This is a scope boundary the owner set at design time, not an item this spec started and left. It needs an owner decision before it becomes a spec |
| The VPP dataplane path for accept-mode | Linux only was in scope; VPP has its own backend | `plan/spec-vrrp-7-vpp.md`, whose R-1 already names accept-mode as a divergence risk |
| Renaming the keepalived lab's `QS-1`, `QS-2` and `QS-3` scenarios | `ai/rules/interop-and-goal-validation.md` bans a numeric prefix on a scenario directory. The three predate this spec, the new scenario is NAMED, and renaming the other three touches `vrrpScenarioNames` (`internal/le/qemu/guestlabs.go`) and every caller | None. Named here so the next VRRP spec sees it; it is a rename of test-lab identifiers with no product effect |
| `vrrpKeepalivedConfig`'s unused `priority` parameter | `unparam` reports it on the GOOS=linux integration lint. Deleting it reaches `VRRPParityConfigs` (`internal/le/qemu/guest_parity_linux.go`), a third file outside this work package | Row already written: `plan/journal/parameter-no-caller-ever-fills.md`, 2026-09-08 |
| The firewall drift detector does not report a desired table the kernel no longer has | `AuditTables` (`internal/component/firewall/audit.go`) takes `continue` on a missing table, so a `ze_vrrp` removed by an external `nft flush ruleset` leaves `checkFirewallHealth` reporting healthy. The accept-mode work does not depend on it: a failed apply errors loudly at the moment it fails, and this is about a table removed after a successful one. The verdict it should raise is a decision affecting every table owner | Row already written: `plan/journal/silent-fall-through.md`, 2026-08-29 |

## Pre-Commit Verification

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|
| `internal/plugins/vrrp/acceptfilter.go` | Yes | `ls internal/plugins/vrrp/` -> `acceptfilter.go` 11K, `acceptfilter_test.go` 22K |
| `internal/test/fixture/vrrp_track_linux.go` | Yes | `ls internal/test/fixture/ \| grep vrrp` -> `vrrp_accept_mode_linux.go` 14K, `vrrp_track_linux.go` 9.8K |
| `test/vrrp/vrrp-track.ci` | Yes | `ls test/vrrp/` -> `vrrp-track.ci` 3.3K, `vrrp-accept-mode.ci` 4.1K, `vrrp-config-invalid.ci` 9.4K, `vrrp-doctor-fires.ci` 3.3K |

### AC Verified (grep/test)

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1..AC-5 | The filter follows the effective accept-mode, in all three directions, with the ND carve-out first and a lifetime tied to the addresses | `./le job run label vrrp-close-unit command go test ./internal/plugins/vrrp/... -count=1` -> `ok github.com/ze-software/ze/internal/plugins/vrrp 0.900s` (and fsm, packet, transport, yang), which runs all 13 functions in `acceptfilter_test.go` |
| AC-6..AC-9 | The decrement sums, floors at 1 and leaves the owner at 255 | Same run: `TestEffectivePriorityWithTracking` and `TestTrackedInterfaceDownDecrementsTheAdvertisedPriority` are in the green package |
| AC-10 | Tracking on the owner is refused at commit and by `ze doctor` | `grep -n "track cannot be combined with an address-owner group" internal/plugins/vrrp/groups.go` -> `validateTracking`; the same string is the `.ci` assertion at `test/vrrp/vrrp-config-invalid.ci` seq=12 |
| AC-11..AC-13 | Fail closed, no advertisement without a change, re-subscribe on a commit | Same green run: `TestUnresolvableTrackedInterfaceCountsAsDown`, `TestTrackingDoesNotAdvertiseWhenNothingChanged`, `TestReconfigureWatchesANewlyTrackedInterface` |

### Wiring Verified (end-to-end)

| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `accept-mode false` on a non-owner Active -> filter installed with the VIPs | `test/vrrp/vrrp-accept-mode.ci` | Yes. Read the file: the config block sets `accept-mode false` on a group backing 198.51.100.1 while the parent holds .251, and the driver probes with a UDP datagram |
| Active demotes to Backup -> filter removed | `test/vrrp/vrrp-accept-mode.ci` | Yes, `expect=stdout:contains=TEARDOWN-COMPLETE` |
| `track interface <name> priority-decrement <n>` and that interface goes down | `test/vrrp/vrrp-track.ci` | Yes. The config carries `track { interface zetrk1 { priority-decrement 150; } }`, and the driver reads the priority byte off the wire |
| The tracked interface comes back up | `test/vrrp/vrrp-track.ci` | Yes, `ADVERT-PRIORITY 200 tracked-interface-restored` |
| `track` on the address-owner group | `test/vrrp/vrrp-config-invalid.ci` seq=12, `test/vrrp/vrrp-doctor-fires.ci` | Yes. Both went RED when `validateTracking` was removed from `validateGroup` and green when it was restored |

### Assumptions Resolved

| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | QEMU guest 2026-08-29: the virtual address stays on the virtual-MAC macvlan while ping to it is 100% loss, and `accept-mode true` restores the reply |
| A-2 | confirmed, reshaped | The seam is `firewall.RegisterTables` plus `firewall.ApplyAll` (`internal/component/firewall/registry.go`). The rule is ADDRESS-scoped, not device-scoped, because Section 6.4.3 names no ingress interface |
| A-3 | confirmed | `vrrpZeConfig` (`internal/le/qemu/vrrp_keepalived_linux.go`) writes `accept-mode true`, so every lab scenario takes the accepting branch and installs no filter |
| A-4 | confirmed | `iface.Resolve` and `iface.Subscribe` answer for any name; no per-prefix route watch and no script runner exist |
| A-5 | confirmed | `masterConfigUpdated` (`internal/plugins/vrrp/fsm/fsm.go`) re-sends from the new config; `TestTrackedInterfaceDownDecrementsTheAdvertisedPriority` reads the decremented priority off the recording `sendAdvert`, and the FSM took no new event |
| A-6 | confirmed | `linkUp` (`internal/plugins/vrrp/register.go`) resolves through `iface.Resolve`, which falls back to the kernel device name; `test/vrrp/vrrp-track.ci` tracks the bare veth `zetrk1`, which the interface tree does not carry |

### Documentation Verified

| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/features.md` VRRP row states the enforced filter and `track` | `grep -n -i vrrp docs/features.md` reads back the installed drop, the Section 6.1 carve-out and the `track` sentence; producer `acceptFilterTables` (`internal/plugins/vrrp/acceptfilter.go`) | Yes |
| `docs/guide/vrrp.md` documents the `track` syntax against the YANG | The page's worked example is `track { interface eth1 { priority-decrement 150; } }`; the YANG carries `container track` with a `list interface` keyed on `name` and a mandatory `priority-decrement` ranged 1..254, in BOTH the `vrrp-group-ipv4` and `vrrp-group-ipv6` groupings of `internal/plugins/vrrp/yang/ze-vrrp-conf.yang` | Yes |
| `docs/architecture/vrrp/vrrp-first-hop-redundancy.md` states the carve-out order | The Consequences bullet says the carve-out is installed FIRST and the drops after; `acceptFilterTables` appends the two Accept terms before the loop over addresses | Yes |
| Row 3, no CLI change | `show vrrp` gained one payload field and no command; `docs/guide/command-reference.md` does not enumerate the `show vrrp` fields | Yes |
| Row 11, no comparison change | `grep -i vrrp docs/comparison.md` returns nothing, so the page carries no claim this change made false | Yes |
| Row 16, source anchors | `./le spec citation anchors spec plan/immediate/spec-vrrp-deferred-accept-mode-dataplane.md` exits 0 with no output | Yes |
| `./le doc check verify` | RED, and no row of it is this spec's: 465 commands fail identically against the generated `../gh-pages/reference/command-equivalents/` surface, plus `docs/DESIGN.md` missing the `firewall-domain` plugin and the wiki command catalog | Yes, attributed |

## Core Insight

A discrimination record pins three fingerprints, and only two of them are named
in the rule that asks for one. `ai/rules/rfc-compliance.md` says a tag you ADD
and a tagged unit whose behavior or claim you CHANGE owe a fresh record. The
third is the PRODUCER, and it moves without touching a test, a claim, or a tag.
Adding one call to `validateGroup` here left three RFC 5798 MUSTs with a red
nobody had observed over the code that was there, and no gate but
`./le rfc check` could say so. The question to ask before a commit is not "did I
change a tagged test" but "what did I change that a record names as its
producer", and `grep -rl <function> rfc/discrimination/` answers it in one
command.
