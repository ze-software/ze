# 1124 -- vrrp-first-hop-redundancy

First-hop redundancy for ze interfaces: RFC 9568 (VRRPv3, default) and RFC 3768
(VRRPv2, opt-in for keepalived interop), IPv4 and IPv6, with RFC-strict
virtual-MAC failover from day one (a per-group macvlan carrying
00:00:5e:00:01:{vrid} / 00:00:5e:00:02:{vrid}). Delivered as a 6-spec umbrella
(vrrp-0..6, vrrp-7/VPP skeleton). Config lives under the interface unit
(`interface <type> <name> unit <u> ipv4|ipv6 vrrp group <name> { vrid ...; ... }`),
Junos/Nokia style. Proven against keepalived 2.3.1 in a QEMU netns lab.

## Context

ze had no gateway-redundancy protocol. VRRP is a wire protocol, so both the RFC
correctness and the keepalived interop had to be right, and the two reference
implementations studied for architecture shape (holo-vrrp, uvrrpd) independently
get v3 interval-adoption, skew arithmetic, and checksum details wrong. The FSM
and codec were therefore written from `rfc/short/` tables only, with every
verified reference bug turned into a negative test. The whole thing is an edge
plugin (`internal/plugins/vrrp/`) plus a generic macvlan mechanism grafted onto
the iface component; nothing in core or in iface learns the string "vrrp".

## Architecture (layer -> package)

- Packet codec (vrrp-1): `internal/plugins/vrrp/packet` -- pure Go, no sockets;
  value-type `Advertisement`, `WriteTo(buf,off) int` + `FillChecksum`,
  IHL-aware `StripIPv4Header`, a 13-row ordered receive-validation ladder, typed
  `Err*` -> `Reason(err)` mapping (the `ze_vrrp_packet_errors_total{reason}`
  labels), zero-alloc happy-path decode, fuzz target. BFD's packet package was
  the in-repo shape model.
- FSM (vrrp-2): `internal/plugins/vrrp/fsm` -- pure, single-threaded,
  actions-as-values (11-action closed set). Emits ordered action slices; the
  engine is the sole executor and the sole owner of `clock.Timer`s. Injected
  `clock.Clock`; `sim.FakeClock` in tests. Timer generations guard stale fires.
- macvlan + dataplane (vrrp-3 + `dataplane_linux.go`, cross-ref learned 1122):
  iface gains a generic owned-macvlan registry (`RegisterOwnedMacvlan`,
  `MacvlanSpec`, `device_owner.go`, `macvlan.go`) parallel to the address-owner
  registry; the netlink backend creates the device; vrrp applies the private-mode
  ARP-ownership sysctl recipe. The full recipe and its debugging live in 1122.
- Transport (vrrp-4): `internal/plugins/vrrp/transport` -- per-instance raw
  proto-112 sockets (rx bound to PARENT with multicast join, tx bound to the
  MACVLAN so frames egress with the virtual MAC), GTSM TTL/hop-limit 255, plus
  the two net-new LAN announcers (gratuitous ARP over AF_PACKET, unsolicited NA
  over raw ICMPv6) that actually move traffic on promotion. Owns
  `doctor-vrrp-raw-socket` and five `ze_vrrp_*` wire-counter series.
- Plugin (vrrp-5): `internal/plugins/vrrp/` -- registration, YANG augments,
  extract-only config walk + cross-leaf verifier, instance manager, show/clear,
  engine-owned state telemetry, events, `doctor-vrrp-config-invalid`.
- Umbrella (vrrp-0) carries the cross-child decisions; vrrp-6 is the keepalived
  interop; vrrp-7 (native VPP) is a skeleton (VPP-backed trees reject at verify).

## Decisions

- v3/IPv4 checksum: TRANSMIT the RFC 5798 pseudo-header form, NOT the RFC 9568
  message-only form. RFC 9568 5.2.8 says message-only for IPv4, but keepalived
  2.3.1 computes and REQUIRES the pseudo-header and rejects message-only as
  "Invalid VRRPv3 checksum"; interop with the installed base outranks the RFC
  clarification. RX dual-accepts: pseudo-header is canonical, message-only is
  accepted and FLAGGED (`MsgOnlyChecksum`, counted `checksum-rfc9568-message-only`)
  so a strict-RFC-9568 peer is operator-visible. The sum selection is isolated in
  one function (`checksum.go`). This flip-flopped twice during development; a live
  keepalived capture settled it.
- Virtual-MAC ARP ownership over macvlan PRIVATE mode (not bridge) + VIP installed
  at the parent's SUBNET prefix (not /32), so the macvlan owns the connected route
  and answers ARP/ND for the VIP. Chosen over bridge mode (parent wins the ARP-flux
  race) and over arp_ignore-alone (VIP becomes unreachable). Full recipe: 1122.
- VIP address-owner strings are PER-INSTANCE (`vrrp:<macvlan-device>`), not a
  single shared "vrrp" owner. `UnregisterOwnedAddresses(owner)` drops the owner
  from ALL interfaces and is the sole `staleIfaces` populator (the only kernel-prune
  path for a device with no YANG desired state), so a shared owner could not drop
  one instance's VIPs without dropping every instance's.
- Metric labels are `{device,group,vrid,family}`, NOT `{interface,vrid,family}`.
  A logical interface is not unique per virtual router: eth0 and eth0.100 can each
  host vrid 10 in one family and would collapse onto one series. `device` is the
  unit's OS device; kernel-facing state hangs off it, never the bare parent.
- YANG group `list` is keyed by operator NAME with `vrid` a mandatory leaf, over
  keying by vrid. Matches ze's `list peer { key "name" }` convention and lets a
  vrid be renumbered without the tree seeing a new object. Cost: a new rule that
  two groups on one unit+family MUST NOT share a vrid (the keyed shape got this
  for free); enforced by the plugin verifier.
- Owner auto-detection FORCES effective values (priority 255, accept-mode true)
  rather than rejecting an operator-set priority on an owner group. RFC 9568 5.2.4
  mandates 255 for owners; forcing keeps a group stanza portable between the owner
  and backup boxes. Surfaced honestly as priority vs effective-priority in show.
- Internal unit is MILLISECONDS everywhere; wire conversion (v3 centiseconds, v2
  whole seconds) happens ONLY in the codec. Kills the s/cs/ms confusion class
  (holo's 100x bug). But COMPUTED durations (skew, master-down) are `time.Duration`
  (ns), because valid v3 skews are sub-millisecond (prio 254 / 10 ms = 78.125 us)
  and integer-ms fields would reintroduce the truncate-to-zero bug; multiply before
  divide, divide last.
- FSM is actions-as-values and pure; the engine executes. Per-instance FSM
  goroutine + manager keyed (parent, unit, family, vrid) over one multiplexed loop
  (matches the injected-clock/per-instance-timer contract). Config-bounded instance
  count.
- Macvlan created at instance CREATE, deleted at instance DELETE; VIP install is
  the ONLY kernel action on the Master transition (plus the GARP/NA burst). Keeps
  failover latency to one address-registry call, per umbrella R-4.
- VPP-backed trees reject at verify (fail closed) until native VPP VRRP lands:
  belt-and-braces of a plugin verify check (reads `interface.backend`) PLUS a
  `ze:backend "netlink"` annotation on the vrrp containers enforced by iface's
  generic gate. Schema-level enforcement survives even if the plugin check regresses.
- No v2 authentication (auth type 0 only, deprecated by RFC 9568); no sync-groups,
  no unicast peers, no priority tracking in this umbrella.

## Consequences

- vrrp shares the `interface` config root with the iface plugin and augments its
  tree (8 augments: ethernet/veth/bridge/dummy x ipv4/ipv6). Auto-loads whenever
  interfaces are configured; with zero groups the engine is fully idle (no sockets,
  no devices), proven by `vrrp-idle.ci`.
- The owned-macvlan registry is generic iface infrastructure and stays if vrrp is
  deleted; orphan cleanup is driven by a kernel-side IFLA_IFALIAS marker
  (`ze:owned:<owner>`), so it works after a crash with no in-memory history. Deleting
  a marked macvlan removes its addresses with it.
- Cross-leaf config rules (version-vs-interval, v3 10ms granularity, accept-mode
  under v2, ipv6 first-VIP link-local, duplicate VIP, duplicate vrid, backend gate)
  live in the plugin's InProcessConfigVerifier + OnConfigVerify (they need sibling
  context), NOT per-leaf `ze:validate`. `ze config validate` works offline.
- Two metric ownership halves: the transport owns the five wire-counter series
  (`ze_vrrp_adverts_*`, `ze_vrrp_packet_errors_total`, ...) plus per-instance
  CounterSnapshot/ResetCounters for show/clear; the engine owns `ze_vrrp_state` +
  `ze_vrrp_transitions_total` (state series live next to the transition that
  produces them).

## Gotchas

- The FIRST IPv6 advert sources from the macvlan's transient auto (EUI-64)
  link-local, not `fe80::1`, and this CANNOT be fixed by FSM action ordering
  because `iface.RegisterOwnedAddresses` is asynchronous. Full ground-truth analysis,
  the proven-ineffective reorder attempt, and why it is genuinely cosmetic are in
  learned 1122 -- do not re-derive or re-attempt.
- accept-mode dataplane enforcement, priority-decrement tracking (interface/route/
  health), native VPP dataplane, and RFC 3768 v2-vs-keepalived interop are all
  DEFERRED, not done. See `plan/deferrals.md` (rows for spec-vrrp-6 / spec-vrrp-7).
  accept-mode false is not enforced: VIPs are ordinary kernel addresses on the
  macvlan so the Active answers VIP traffic regardless; the leaf drives FSM/owner
  semantics only. Interop scenarios needing pingable VIPs set accept-mode true.
- Transport metrics were silently dead: `ConfigureMetrics` installed the engine
  registry but never forwarded it to the shared transport, so the five `ze_vrrp_*`
  series sat on the no-op registry. Fixed by `sharedTransport.SetMetrics(reg)` in
  the plugin's `ConfigureMetrics`. A registered-but-never-incremented counter and a
  registered-but-never-forwarded registry are both invisible failures -- assert the
  production wiring, not just the unit-level registration.
- Each instance opens its OWN parent-bound rx socket, so one advert on the wire is
  delivered once PER instance on that parent. Routing rx by VRID would hand every
  copy to one instance (duplicate FSM events, N-times-inflated counters). Fixed by
  stamping `transport.RxItem.Key` at the per-instance rx sink and routing on it.
- accept-mode (and other bool leaves) can arrive as the STRING "true" from the
  config tree; a `.(bool)` assertion silently drops it, so a `version 2` +
  accept-mode config validated clean. Use a helper that accepts both shapes and
  rejects anything else.
- macvlan create does NOT persist IFLA_IFALIAS: the vendored netlink lib serializes
  the alias in RTM_NEWLINK but the kernel ignores it at create (MAC and mode DO
  stick). Fallback is LinkAdd + LinkSetAlias + LinkSetUp with rollback LinkDel, and
  the reconcile pass adopts-and-re-marks an unmarked macvlan that holds a registered
  name.
- Parent admin-down does NOT propagate to the macvlan's oper-state: the kernel sets
  an M-DOWN flag but leaves the macvlan UP/LOWER_UP, immediately and after linkwatch
  ticks. There is no eventual propagation. Consequence: vrrp readiness keys on the
  PARENT's state (`parentReady`, via `iface.Subscribe`) and treats the macvlan as
  existence-only; the macvlan's oper-state is never a liveness signal.
- Two QEMU-found kernel/netns traps in the transport: (1) sending a v6 advert from a
  still-TENTATIVE macvlan link-local (DAD window) fails sendmsg with EINVAL --
  resolve through the iface seam and skip tentative/non-link-local addresses,
  count a residual EINVAL as {reason=no-link-local} with natural retry; (2) netlink
  queries bind to the CALLING thread's netns, so resolving the link-local lazily on
  the announcer goroutine made the device invisible in a netns test -- warm the
  source cache on the engine's goroutine, let the worker do only socket I/O. Also:
  `/proc/net/igmp` is the thread-GROUP-leader's netns; read `/proc/thread-self/...`
  to observe a setns'd thread's multicast joins.
- `go test -update` on the plugin-name golden without the feature build tags
  silently deletes other plugins' methods from the snapshot; regenerate with the
  Makefile's `GO_TEST_TAGS`.
- `docs/features/interfaces.md` "Gateway Redundancy | VRRP / keepalived |
  missing" was never flipped to implemented (residual doc gap, recorded in the
  umbrella and spec-5).

## Interop evidence

Proven against keepalived 2.3.1 via `scripts/evidence/effective-vrrp-keepalived.py`
(`make ze-qemu-vrrp-keepalived-test`): v3 IPv4 election, node-death failover
(prio-0 + timeout), preempt, and virtual-MAC ownership (observer resolves the VIP
to 00:00:5e:00:01:{vrid}). v3 IPv6 (adverts both ways, election, node-death
failover, unsolicited NA burst, VIP -> 00:00:5e:00:02:{vrid}) was proven manually
in the QEMU IPv6 lab and captured in learned 1122; automated QS-5 (v2) and QS-6
(IPv6) scenarios are PENDING on disk. Wire format is golden-byte tested against RFC
diagrams and cross-checked against live keepalived captures; functional `.ci` under
QEMU covers instance-up (macvlan + virtual MAC + VIP install + teardown), show, and
doctor.

## Files

- `internal/plugins/vrrp/` -- register.go, groups.go, engine.go, instance.go,
  cmd_show.go, telemetry.go, doctor.go, vrrp.go, dataplane_linux.go
- `internal/plugins/vrrp/packet/` -- packet.go, checksum.go, validate.go, fuzz_test.go
- `internal/plugins/vrrp/fsm/` -- fsm.go, actions.go, events.go, timers.go, doc.go
- `internal/plugins/vrrp/transport/` -- transport.go, backend_linux.go,
  backend_other.go, garp*.go, na*.go, announce.go, counters.go, metrics.go, doctor*.go
- `internal/plugins/vrrp/yang/` -- ze-vrrp-conf.yang (augments), ze-vrrp-cmd.yang
- `internal/component/iface/` -- macvlan.go, device_owner.go (generic owned-macvlan
  registry), device pass in config_apply.go, Alias on InterfaceInfo
- `internal/plugins/iface/netlink/` -- macvlan_linux.go, doctor_linux.go
- `internal/core/diagnostic/codes.go` -- doctor-vrrp-raw-socket, doctor-vrrp-config-invalid,
  doctor-vrrp-backend-unusable, doctor-iface-macvlan CodeMeta rows
- `scripts/evidence/effective-vrrp-keepalived.py`, `test/vrrp/*.ci`, `docs/guide/vrrp.md`
