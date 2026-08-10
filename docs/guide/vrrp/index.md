# VRRP

VRRP gives a LAN a gateway address that survives the loss of the router holding
it. Two or more routers share a virtual IP; one is Active (Master) and answers
for it, the others wait. If the Active router dies, a Backup takes the address
over, and the hosts on the segment never change their default gateway.

Ze implements [RFC 9568](https://www.rfc-editor.org/rfc/rfc9568.html) (VRRPv3,
IPv4 and IPv6) by default, and [RFC 3768](https://www.rfc-editor.org/rfc/rfc3768.html)
(VRRPv2, IPv4 only) as a per-group opt-in for talking to older equipment.

> **Status: Experimental.** The implementation is complete, its wire format is
> asserted against golden frames under QEMU, and it interoperates with keepalived
> 2.3.1 across election, node-death failover, and graceful-stop scenarios
> (including virtual-MAC ownership of the virtual IP), but it has not yet been
> proven in production. See
> [RFC status](../../features/rfc-status/index.md#first-hop-redundancy).

## Configuration

A VRRP group lives under the interface unit's address family, next to the
addresses it backs:

```
interface {
    backend netlink;
    ethernet eth0 {
        unit 0 {
            ipv4 {
                address [ 192.0.2.251/24 ];
                vrrp {
                    group uplink {
                        vrid 10;
                        virtual-address [ 192.0.2.1 ];
                        priority 200;
                    }
                }
            }
        }
    }
}
```

`group uplink` is a name you choose, for your own readability: it names the
group in `show vrrp` output, in log lines, and in the `group` metric label. It
carries no protocol meaning. The protocol identity is `vrid`, which is
mandatory, must be 1..255, and must match on every router in the group.

The two routers in a group differ only in `priority`. Give the router you want
to win the higher number.

| Leaf | Default | Meaning |
|------|---------|---------|
| `vrid` | none (mandatory) | Virtual Router ID, 1..255. The same value on every router in the group. |
| `virtual-address` | none (at least one) | The address(es) the group backs. A leaf-list: `[ 192.0.2.1 192.0.2.2 ]`. |
| `priority` | 100 | 1..254. Highest wins. 255 is reserved for the address owner and is assigned by ze, never by you. |
| `preempt` | true | Whether a higher-priority router takes mastership back when it returns. |
| `preempt-delay-seconds` | 0 | 0..3600. How long a returning higher-priority router waits before preempting. Useful when a router's uplinks converge more slowly than it boots, so it does not take the gateway back before it can forward. |
| `accept-mode` | false | VRRPv3 only. Records whether a non-owner Active should accept packets addressed to the virtual IP. **Not enforced on the dataplane yet: see the limitation below.** |
| `advertise-interval-milliseconds` | 1000 | How often the Active router advertises. |
| `version` | 3 | `2` opts this group into VRRPv2. |

### Choosing a version

VRRPv3 is the default and is what you want unless something else on the segment
cannot speak it. Set `version 2` per group for legacy peers:

```
vrrp {
    group legacy {
        vrid 20;
        version 2;
        virtual-address [ 192.0.2.5 ];
        advertise-interval-milliseconds 2000;
    }
}
```

VRRPv2 is IPv4-only and has no `accept-mode`; ze rejects a config that combines
them rather than ignoring the leaf. The two versions also encode the advertise
interval differently, which constrains what you can ask for:

| Version | Interval encoding | Valid `advertise-interval-milliseconds` |
|---------|-------------------|------------------------------------------|
| 3 | centiseconds | any multiple of 10 |
| 2 | whole seconds | any multiple of 1000 |

Ze refuses an interval it cannot put on the wire instead of silently rounding
it, because a rounded interval is a timing mismatch you would only discover as a
flapping election.

RFC 3768's authentication types are deliberately not implemented: RFC 9568
Section 9 removed them, as they protect against no real attack.

### IPv6

An IPv6 group's first virtual address must be the link-local one, because
RFC 9568 Section 5.2.9 makes the first address the advert's source identity:

```
ipv6 {
    vrrp {
        group uplink-v6 {
            vrid 10;
            virtual-address [ fe80::1 2001:db8::1 ];
            priority 200;
        }
    }
}
```

IPv4 and IPv6 groups are independent state machines even when they share a
`vrid`, so a group of each on one unit is normal and correct.

IPv6 needs none of the ARP-flux sysctls IPv4 needs: Neighbor Solicitation for
the virtual IP targets its solicited-node multicast group, which only the
virtual-MAC macvlan joins (the parent does not hold the virtual IP), so the
parent never competes to answer. Ze does disable Duplicate Address Detection on
that macvlan (`accept_dad=0`): a virtual IP lives on one router at a time, so DAD
would only add a tentative window during which the address is unreachable right
after a promotion. IPv6 interoperability is verified against keepalived under
QEMU (election, node-death failover, and virtual-MAC resolution of the virtual
IP).

### The address owner

If a group's virtual address equals an address configured on the unit, that
router owns the address and ze assigns it priority 255 automatically
(RFC 9568 Section 5.2.4). The owner always wins. Do not set `priority 255`
yourself; ze rejects it, because the owner is derived from the addresses, and a
hand-set 255 on a non-owner would claim an authority the router does not have.

## The virtual MAC

Each group gets its own macvlan device carrying the RFC virtual MAC:

| Family | Virtual MAC |
|--------|-------------|
| IPv4 | `00:00:5e:00:01:{vrid}` |
| IPv6 | `00:00:5e:00:02:{vrid}` |

This is why failover works without the hosts noticing. The virtual MAC moves
with the virtual IP, so a host's ARP or neighbour entry for the gateway stays
valid: the address it points at simply arrives from a different router. Ze
additionally sends gratuitous ARP (IPv4) or unsolicited Neighbor Advertisements
(IPv6) on promotion so switches relearn the port immediately.

The macvlan is created and destroyed with the group. It belongs to the plugin,
not to your config, and you do not declare it.

A host on the segment that resolves the virtual IP gets the **virtual MAC**, not
the router's real MAC, so failover is transparent at L2: the address moves to a
different router without the host's ARP or neighbour entry changing. Achieving
this on Linux, where the macvlan's parent holds a real address in the same
subnet, requires the macvlan to be the sole ARP responder for the virtual IP.
Ze arranges that automatically when it creates the macvlan (you configure
nothing): the device is created in macvlan *private* mode, the virtual IP is
installed with the parent's subnet prefix, and a small set of `arp_ignore`,
`arp_filter`, and `rp_filter` sysctls stop the parent answering for the virtual
IP while letting the macvlan answer with the virtual MAC. This mirrors what
keepalived's `use_vmac` does, and ze restores the sysctls it changed when the
last group on an interface goes away. This behaviour is verified against
keepalived under QEMU (`ze-test`'s VRRP interop lab).

A few consequences of this mechanism are worth knowing:

- **Ze owns the parent's `arp_ignore`, `arp_filter`, and `rp_filter` while a
  group is active.** It re-asserts them on every config apply, so if you also set
  those knobs on the same interface unit, ze's values win for as long as VRRP
  runs there. Do not rely on a conflicting manual setting on a VRRP interface.
- **`net.ipv4.conf.all.rp_filter` is set to 0** (host-wide) while any IPv4 group
  is active, because a per-device `rp_filter` cannot go below the `all` value.
  Ze restores the previous value when the last group is removed, but a hard kill
  (SIGKILL, power loss) skips that cleanup and leaves it at 0. This matches
  keepalived, which never restores it.
- **The address owner is a special case.** When the virtual IP equals a real
  address on the unit, that address already lives on the parent, so it keeps
  answering with the parent's real MAC (ze installs it as a host route on the
  macvlan to avoid a duplicate subnet route). Virtual-MAC ownership does not
  apply to the owner, which never fails the address over anyway.
- **First-resolution race (IPv4 only).** The very first host to ARP for the
  virtual IP right as a router becomes Active can, for one resolution, cache the
  parent's real MAC before the macvlan takes over; it converges to the virtual
  MAC on its next resolution. keepalived's `use_vmac` behaves identically. IPv6
  has no such race (Neighbor Solicitation is not a broadcast).

## Operating

| Command | Shows |
|---------|-------|
| `show vrrp` | Every group: state, priority, vrid, virtual addresses. |
| `show vrrp interface name eth0` | Only the groups on one interface. |
| `show vrrp statistics` | Per-group counters, including rejected packets and why. |
| `clear vrrp statistics` | Resets those counters. |

State changes are logged as `vrrp: state change` with `from`, `to`, and a
`reason`, so a failover leaves a record of what triggered it.

### Demo: Keep the gateway reachable while Ze stops

Inspect the active and live VRRP state, stop the higher-priority Ze router, and prove keepalived takes the same reachable VIP.

[Play the WebM recording](../../../assets/demos/vrrp-failover.webm?v=50fbcdb61e) · [View the poster](../../../assets/demos/vrrp-failover.png?v=a4b250811a) · [Plain-text transcript](../../../assets/demos/vrrp-failover.txt?v=0b9dc45f28)

Recorded with Ze 26.07.18 in a Linux namespace lab using VHS 0.11.0. Duration: 2 minutes 54 seconds.

```console
An operator needs to stop the active router without changing the default gateway on every host.

$ ze config show demos/terminal/vrrp-failover/ze.conf interface ethernet eth0 unit 0 ipv4 vrrp group gateway
The daemon configuration shows VRID 10, VIP 192.0.2.1, priority 200, and the advertisement interval.

$ grep -E 'interface|virtual_router_id|priority|192.0.2.1' keepalived.conf
The peer is configured as BACKUP on the same interface, VRID, and VIP with a lower priority of 100.

$ ze cli -c "show vrrp"
The complete live state shows Ze is master.

$ ip -n vrrp-ze -o addr show | grep 192.0.2.1
The kernel shows the VIP on Ze's RFC virtual-MAC interface.

$ cat failover-proof.sh
$ bash -x failover-proof.sh
The recording displays and traces the exact commands that kill Ze, remove its namespace, inspect the VIP on keepalived, and send two probes.

The final kernel output shows 192.0.2.1 on keepalived's `vrrp.10` interface, and both probes succeed after failover.
```


For Prometheus:

| Metric | Meaning |
|--------|---------|
| `ze_vrrp_state{device,group,vrid,family}` | Current state of each group. |
| `ze_vrrp_transitions_total` | State transitions, for spotting a flapping group. |

## Requirements and limits

**`accept-mode` is not enforced on the dataplane.** RFC 9568 Section 6.4.3 says a
non-owner Active with `Accept_Mode` false must not accept packets addressed to
the virtual IP, other than the adverts themselves. Ze records the leaf, validates
it (rejecting it under `version 2`), and reports it in `show vrrp`, but does not
yet install the filtering: the virtual address is configured on the group's
macvlan while the router is Active, so the kernel answers ping and other traffic
for it whichever way you set the leaf. In practice this means an Active ze router
answers on the virtual IP, which is what `accept-mode true` asks for and what
most deployments want. If you are relying on `accept-mode false` to make the
virtual IP unreachable except as a forwarding next hop, it will not do that
today.

**No priority tracking.** A group's `priority` is fixed at the value you
configure. Ze does not decrement it in response to an uplink going down, a route
disappearing, or a health check failing, so it will not on its own hand the
gateway to a Backup when an upstream path fails while the VRRP interface itself
stays up. The interface, route, and script tracking that Junos, Nokia, and VyOS
use to drive that priority-decrement failover is not implemented in this
release; failover is driven only by VRRP's own triggers: loss of the Active
router's adverts, carrier loss on the VRRP interface, or a graceful stop.

**Netlink backend only.** VRRP needs macvlan devices and raw sockets, which the
VPP backend does not provide. A VPP-backed config carrying a VRRP group is
rejected at validation rather than started in a degraded state.

**Raw socket privileges.** Ze needs `CAP_NET_RAW` and `CAP_NET_ADMIN` to send
adverts (IP protocol 112) and to manage the macvlan.

**Multicast must reach the segment.** Adverts go to `224.0.0.18` (IPv4) or
`ff02::12` (IPv6) with TTL/hop-limit 255, and RFC 9568 requires receivers to
discard anything arriving with a lower TTL. A device that rewrites the TTL or
filters link-local multicast will break the election.

## Diagnostics

`ze doctor` runs the same verifier a commit runs, so the two can never disagree.

| Code | Meaning |
|------|---------|
| `doctor-vrrp-config-invalid` | A cross-leaf rule was broken (for example `accept-mode` with `version 2`). |
| `doctor-vrrp-backend-unusable` | The tree configures VRRP on a backend that cannot run it. |
| `doctor-vrrp-raw-socket` | The raw socket VRRP needs cannot be opened, usually missing `CAP_NET_RAW`. |

Run `ze explain <code>` for the remediation.

<!-- source: internal/plugins/vrrp/yang/ze-vrrp-conf.yang -- config surface -->
<!-- source: internal/plugins/vrrp/groups.go -- cross-leaf verifier and its messages -->
<!-- source: internal/plugins/vrrp/cmd_show.go -- show/clear commands -->
<!-- source: internal/plugins/vrrp/doctor.go -- diagnostic codes -->
<!-- source: internal/plugins/vrrp/telemetry.go -- metrics and state-change event -->
<!-- source: internal/plugins/vrrp/engine.go -- state-change log line -->
<!-- source: internal/component/iface/macvlan.go -- per-group macvlan virtual MAC (private mode) -->
<!-- source: internal/plugins/vrrp/dataplane_linux.go -- ARP-ownership sysctls that make the virtual MAC answer for the VIP -->

<!-- test: test/vrrp/vrrp-config.ci -- every valid shape shown above validates -->
<!-- test: test/vrrp/vrrp-config-invalid.ci -- every rejection rule described above -->
<!-- test: test/vrrp/vrrp-doctor.ci -- the diagnostic codes are explainable -->
