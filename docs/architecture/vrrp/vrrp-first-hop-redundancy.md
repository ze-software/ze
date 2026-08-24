# VRRP: First-Hop Redundancy

First-hop redundancy for ze interfaces. RFC 9568 (VRRPv3) is the default and
RFC 3768 (VRRPv2) is opt-in for keepalived interop. IPv4 and IPv6 are both
supported, with virtual-MAC failover: a per-group macvlan carrying
`00:00:5e:00:01:{vrid}` for IPv4 and `00:00:5e:00:02:{vrid}` for IPv6.

Config lives under the interface unit, in Junos and Nokia style:
`interface <type> <name> unit <u> ipv4|ipv6 vrrp group <name> { vrid ...; ... }`.

The dataplane recipe that makes the virtual MAC own the VIP is a separate page:
[VRRP virtual-MAC dataplane](vrrp-macvlan-vmac-dataplane.md).

## Why the codec was written from the RFC tables alone

VRRP is a wire protocol, so RFC correctness and keepalived interop both had to be
right. Two reference implementations were read for architecture shape, holo-vrrp
and uvrrpd. Each gets v3 interval adoption, skew arithmetic, or checksum details
wrong, and they get them wrong independently. The FSM and the codec were
therefore written from the `rfc/short/` tables only, and every verified reference
bug became a negative test.

## Layers

<!-- source: internal/plugins/vrrp/vrrp.go -- plugin package layout -->

| Layer | Package | Content |
|-------|---------|---------|
| Packet codec | `internal/plugins/vrrp/packet` | value-type `Advertisement`, `WriteTo(buf, off) int` plus `FillChecksum`, IHL-aware `StripIPv4Header`, a 13-row ordered receive-validation ladder, typed `Err*` mapped by `Reason(err)` to the `ze_vrrp_packet_errors_total{reason}` labels, zero-allocation decode on the happy path, a fuzz target. No sockets. The BFD packet package was the in-repo shape model |
| FSM | `internal/plugins/vrrp/fsm` | pure, single-threaded, actions-as-values over a closed set of 11 actions. It emits ordered action slices. Injected `clock.Clock`, `sim.FakeClock` in tests. Timer generations reject a stale fire |
| Dataplane | `internal/plugins/vrrp/dataplane_linux.go` | the macvlan ARP-ownership recipe. iface holds a generic owned-macvlan registry (`RegisterOwnedMacvlan`, `MacvlanSpec`) parallel to its address-owner registry, and the netlink backend creates the device |
| Transport | `internal/plugins/vrrp/transport` | per-instance raw proto-112 sockets, plus the gratuitous ARP and unsolicited NA announcers that move traffic on promotion. Owns `doctor-vrrp-raw-socket` and five `ze_vrrp_*` wire-counter series |
| Plugin | `internal/plugins/vrrp` | registration, YANG augments, extract-only config walk with a cross-leaf verifier, instance manager, show and clear, engine-owned state telemetry, events, `doctor-vrrp-config-invalid` |

The engine is the sole executor of FSM actions and the sole owner of
`clock.Timer`s. Nothing in core and nothing in iface holds the string "vrrp".

## Decisions

### The v3 IPv4 checksum transmits the RFC 5798 pseudo-header form

<!-- source: internal/plugins/vrrp/packet/checksum.go -- FillChecksum, pseudoSumV4Legacy -->

RFC 9568 Section 5.2.8, for IPv4: "the checksum computation only includes the
VRRP message starting with the Version field and ending with the end of the last
IPvX address".

keepalived 2.3.1 computes and requires the RFC 5798 pseudo-header form, and
rejects the message-only form with "Invalid VRRPv3 checksum". Ze transmits the
pseudo-header form, because interop with the installed base outranks the RFC
clarification here.

Receive dual-accepts. The pseudo-header form is canonical. The message-only form
is accepted and flagged as `MsgOnlyChecksum`, counted under
`checksum-rfc9568-message-only`, so a strict RFC 9568 peer is visible to the
operator. The selection sits in one function in `checksum.go`. The decision
flip-flopped twice during development and a live keepalived capture settled it.

IPv6 includes the RFC 8200 Section 8.1 pseudo-header. VRRPv2 uses no
pseudo-header, per RFC 3768 Section 5.3.7.

### GTSM on receive

<!-- source: internal/plugins/vrrp/transport/backend_linux.go -- TTL and hop-limit checks -->

Adverts are sent and checked with TTL or hop limit 255. The receive socket is
bound to the PARENT and joins the multicast group. The transmit socket is bound
to the MACVLAN, so frames leave with the virtual MAC.

### Owner auto-detection forces the effective values

<!-- source: internal/plugins/vrrp/groups.go -- owner detection, effective priority -->

RFC 9568 Section 5.2.4 mandates priority 255 for the address owner. Ze forces
priority 255 and `accept-mode true` when it detects an owner, rather than
rejecting an operator-set priority. A group stanza is then portable between the
owner box and the backup box. Show output reports priority and effective priority
separately, so the forcing is visible.

### VIP address-owner strings are per-instance

<!-- source: internal/plugins/vrrp/instance.go -- owner string per macvlan device -->
<!-- source: internal/component/iface/address_owner.go -- UnregisterOwnedAddresses, staleIfaces -->

The owner string is `vrrp:<macvlan-device>`, not a single shared `vrrp` owner.
`UnregisterOwnedAddresses(owner)` drops the owner from EVERY interface, and it is
the only populator of `staleIfaces`, which is the only kernel-prune path for a
device with no YANG desired state. A shared owner could therefore not drop one
instance's VIPs without dropping every instance's.

### Metric labels are `{device,group,vrid,family}`

<!-- source: internal/plugins/vrrp/telemetry.go -- metric label set -->

A logical interface is not unique per virtual router. `eth0` and `eth0.100` can
each host vrid 10 in one family, and `{interface,vrid,family}` collapses them
onto one series. `device` is the unit's OS device. Kernel-facing state hangs off
the device, never off the bare parent.

### The virtual router lives on the RESOLVED device

<!-- source: internal/plugins/vrrp/groups.go -- parentDevice, unitDeviceName, unitVLANID -->
<!-- source: internal/plugins/vrrp/engine.go -- apply, the binding step -->

An operator pins an interface to a NIC with `mac/match`, or aliases it with
`os-name`, and the virtual router belongs on the device that selector answers.
RFC 9568 Section 7.3 puts the virtual MAC on the interface the virtual router
protects, so building it on a device that merely shares the interface's name is
not a degraded VRRP: the protected address fails over to a router that cannot
carry it.

Binding happens in `engine.apply`, once per group per apply, through
`iface.ResolveDevice`. That is the same answer the by-name dispatch ops take, so
VRRP is a consumer of the shared resolution rather than a second route to it.
The macvlan parent, the per-device sysctls, the transport's parent and the
readiness probe all read `GroupSpec.ParentDevice` afterwards, so one resolution
feeds every kernel-facing value.

Extraction stays pure and leaves `ParentDevice` empty: `ze config validate` and
`ze doctor` judge the configuration and must not need the selected NIC to be
present. `GroupSpec` therefore carries the unit's `VLANID` as the config fact it
is, and the VLAN suffix is composed onto the resolved device at binding, because
both backends name a VLAN netdev after the parent they are handed.

A binding outcome MAY create a virtual router and MAY move one. It MUST NOT
destroy one: only the config removing a group tears its instance down.
`ResolveDevice` refuses on any resolution failure once the name carries a
selector, and "the backend is not loaded" and "the interface listing failed" are
among them, so an error does not mean the device is gone. Acting on it as though
it did converts one transient netlink read into a permanent outage, because the
resolver cache is dropped on every iface apply and `apply` runs only on a config
event. The device actually disappearing is handled by `parentReady` and
`watchParent`, which stand the router down and start it again with no commit and
no macvlan churn.

So a selector that answers no device, or more than one, keeps a group that is
NOT running out of the desired set, and no macvlan, socket or sysctl of it
reaches any device. A group that IS running keeps the device it is bound to, and
the rest of the operator's edit still lands on it.

A group whose device CHANGES between applies is rebuilt rather than reconfigured
in place, because its macvlan, its sockets and its sysctls all hang off the old
device and reconfigure touches none of them. The rebuild BUILDS the replacement
before it releases the predecessor: `create` splits into `build`, which makes
everything and stores nothing, and `start`. That ordering is load-bearing rather
than tidy. `reconcileOwnedDevices` fails fast on the first owned-device error in
a pass, so one unrelated device times this group's `waitDevicePresent` out, and
releasing first would let that destroy a working virtual router -- the same
permanent outage the rule above forbids, arriving through the one path that
still calls `teardown` for a binding reason. The replacement's macvlan is named
from the NEW parent's ifindex, so it cannot collide with the one still in place.

One move is the exception, and it releases first. A transport instance is keyed
`{parent name, vrid, family}` while the macvlan is named from the parent's
ifindex, so a netdev REPLACED under a name that did not change -- a card that
re-enumerates, a driver that reloads, a VLAN device an iface apply recreates --
moves the macvlan and leaves the transport key where it was. A replacement built
first would open over the running instance's key, and the transport overwrites
that entry without shutting the displaced sockets down, so the teardown that
follows closes the replacement instead. The group would then hold sockets that
are shut while every surface reported it running. Releasing first destroys
nothing that still works there: the netdev the running instance is bound to no
longer wears that name.

<!-- source: internal/plugins/vrrp/engine.go -- apply, build, start, teardown -->

### The YANG group list is keyed by operator name

<!-- source: internal/plugins/vrrp/register.go -- config verifier, duplicate vrid rule -->

The list key is the operator's name and `vrid` is a mandatory leaf. This matches
ze's `list peer { key "name" }` convention and lets a vrid be renumbered without
the tree seeing a new object. The cost is one rule that a keyed-by-vrid shape got
for free: two groups on one unit and family MUST NOT share a vrid. The plugin
verifier enforces it.

### Milliseconds internally, wire units only in the codec

<!-- source: internal/plugins/vrrp/fsm/timers.go -- skew and master-down arithmetic -->

Every internal interval is milliseconds. The conversion to v3 centiseconds and v2
whole seconds happens only in the codec. That kills the seconds/centiseconds/
milliseconds confusion class, which is the source of holo-vrrp's 100x bug.

Computed durations are `time.Duration` nanoseconds, not integer milliseconds. A
valid v3 skew is sub-millisecond: priority 254 at a 10 ms interval gives
78.125 us, and an integer-millisecond field truncates it to zero. Multiply before
you divide, and divide last.

### One FSM goroutine per instance

<!-- source: internal/plugins/vrrp/engine.go -- per-instance dispatcher -->

The manager keys instances by (parent, unit, family, vrid). One multiplexed loop
was rejected: the injected-clock and per-instance-timer contract wants one
goroutine per instance. The instance count is bounded by config.

### The macvlan lives for the instance, not for mastership

<!-- source: internal/plugins/vrrp/instance.go -- create and delete of the macvlan -->

The macvlan is created at instance CREATE and deleted at instance DELETE. VIP
install is the only kernel action on the transition to Master, together with the
gratuitous ARP and unsolicited NA burst. Failover latency is therefore one
address-registry call.

### VPP-backed trees fail closed

<!-- source: internal/plugins/vrrp/register.go -- backend gate on interface.backend -->

Until a native VPP VRRP dataplane exists, a VPP-backed tree is rejected at verify.
Two mechanisms enforce it: a plugin verify check that reads `interface.backend`,
and a `ze:backend "netlink"` annotation on the vrrp containers that iface's
generic gate enforces. The schema-level gate survives a regression in the plugin
check.

### Not implemented in this design

No VRRPv2 authentication: auth type 0 only, since RFC 9568 deprecates the others.
No sync groups, no unicast peers, no priority tracking.

## Consequences

<!-- source: internal/plugins/vrrp/register.go -- YANG augments and auto-load -->

- vrrp shares the `interface` config root with the iface plugin and augments its
  tree with 8 augments: ethernet, veth, bridge and dummy, each for ipv4 and ipv6.
  It auto-loads whenever interfaces are configured. With zero groups the engine is
  idle: no sockets and no devices.
- The owned-macvlan registry is generic iface infrastructure and survives the
  deletion of vrrp. Orphan cleanup is driven by a kernel-side `IFLA_IFALIAS`
  marker (`ze:owned:<owner>`), so it works after a crash with no in-memory
  history. Deleting a marked macvlan removes its addresses with it.
- Cross-leaf config rules need sibling context, so they live in the plugin's
  `InProcessConfigVerifier` and `OnConfigVerify`, not in per-leaf `ze:validate`.
  The rules are version-versus-interval, v3 10 ms granularity, accept-mode under
  v2, IPv6 first-VIP link-local, duplicate VIP, duplicate vrid, and the backend
  gate. `ze config validate` therefore works offline.
- Metric ownership is split. The transport owns the five wire-counter series and
  the per-instance `CounterSnapshot` and `ResetCounters` used by show and clear.
  The engine owns `ze_vrrp_state` and `ze_vrrp_transitions_total`, because a state
  series belongs next to the transition that produces it.

## Traps

<!-- source: internal/plugins/vrrp/transport/transport.go -- RxItem.Key per-instance routing -->
<!-- source: internal/plugins/vrrp/instance.go -- parentReady -->

- `accept-mode false` is not enforced in the dataplane. VIPs are ordinary kernel
  addresses on the macvlan, so the Active router answers VIP traffic whatever the
  leaf says. The leaf drives FSM and owner semantics only. An interop scenario
  that needs a pingable VIP sets `accept-mode true`.
- Transport metrics were once silently dead: `ConfigureMetrics` installed the
  engine registry and never forwarded it to the shared transport, so the five
  `ze_vrrp_*` series sat on a no-op registry. `sharedTransport.SetMetrics(reg)`
  in the plugin's `ConfigureMetrics` fixes it. A registered counter that nothing
  increments, and a registered registry that nothing forwards, are both invisible
  failures. Assert the production wiring, not the unit-level registration.
- Each instance opens its own parent-bound receive socket, so one advert on the
  wire is delivered once per instance on that parent. Routing receive by VRID
  hands every copy to one instance, which gives duplicate FSM events and counters
  inflated by the instance count. The per-instance receive sink stamps
  `transport.RxItem.Key` and routing keys on that.
- A bool leaf can arrive from the config tree as the STRING "true". A `.(bool)`
  assertion drops it silently, which let a `version 2` plus accept-mode config
  validate clean. Use a helper that accepts both shapes and rejects anything else.
- macvlan create does not persist `IFLA_IFALIAS`. The vendored netlink library
  serializes the alias in `RTM_NEWLINK` and the kernel ignores it at create, while
  the MAC and the mode do stick. The fallback is LinkAdd, LinkSetAlias, LinkSetUp,
  with LinkDel as rollback. The reconcile pass adopts and re-marks an unmarked
  macvlan that holds a registered name.
- Parent admin-down does not propagate to the macvlan's oper-state. The kernel
  sets an M-DOWN flag and leaves the macvlan UP and LOWER_UP, immediately and
  after linkwatch ticks. There is no eventual propagation. VRRP readiness
  therefore keys on the PARENT's state, through `iface.Subscribe`, and treats the
  macvlan as existence-only. The macvlan's oper-state is never a liveness signal.
- Sending a v6 advert from a still-tentative macvlan link-local, inside the DAD
  window, fails `sendmsg` with EINVAL. Resolve the source through the iface seam
  and skip tentative and non-link-local addresses. A residual EINVAL is counted as
  `{reason=no-link-local}` and retries naturally.
- A netlink query binds to the CALLING thread's netns. Resolving the link-local
  lazily on the announcer goroutine made the device invisible in a netns test.
  Warm the source cache on the engine's goroutine and leave the worker socket I/O
  only. For the same reason, `/proc/net/igmp` reports the thread-group leader's
  netns: read `/proc/thread-self/net/igmp` to observe a `setns`ed thread's
  multicast joins.
- `go test -update` on the plugin-name golden without the feature build tags
  deletes other plugins' methods from the snapshot. Regenerate through the
  Makefile, which sets `GO_TEST_TAGS`.

## Interop evidence

<!-- source: scripts/evidence/effective-vrrp-keepalived.py -- keepalived interop driver -->

`make ze-qemu-vrrp-keepalived-test` runs against keepalived 2.3.1 in a QEMU netns
lab and covers v3 IPv4 election, node-death failover by priority 0 and by
timeout, preempt, and virtual-MAC ownership, where the observer resolves the VIP
to `00:00:5e:00:01:{vrid}`.

v3 IPv6 was proven in the same lab by hand: adverts in both directions, election,
node-death failover, the unsolicited NA burst, and the VIP resolving to
`00:00:5e:00:02:{vrid}`.

Wire format is golden-byte tested against the RFC diagrams and cross-checked
against live keepalived captures. Functional `.ci` coverage under QEMU covers
instance-up (macvlan, virtual MAC, VIP install, teardown), show, and doctor.
