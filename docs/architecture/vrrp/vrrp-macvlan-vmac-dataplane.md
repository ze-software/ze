# VRRP: Virtual-MAC Dataplane over a macvlan

How a macvlan becomes the sole L2 owner of an IP whose parent interface holds a
real address in the same subnet. The mechanism is generic: any virtual-MAC or
floating-IP feature can use it. VRRP is its first user.

## The problem

A virtual IP on a bridge-mode macvlan carrying a virtual MAC does not resolve to
that virtual MAC. When the macvlan's PARENT holds a real address in the same
subnet, the Linux kernel answers ARP for the VIP from the parent, with the
parent's real MAC. The macvlan receives the broadcast who-has frame, and the
kernel still picks the parent as the responder.

`arp_ignore=1` on the parent alone does not fix it. It silences the parent and
nothing else answers, so the VIP becomes unreachable. That state is worse than
the ARP-flux it replaces.

## The recipe

<!-- source: internal/plugins/vrrp/dataplane_linux.go -- macvlan sysctl recipe, apply and restore -->

The engine applies this at macvlan create and restores shared settings at teardown.
The ARP settings follow keepalived's `use_vmac`; Ze also selects ICMP error sources
from the inbound virtual-MAC device.

Setup fails if a required sysctl cannot be read or written. Before changing
shared knobs, Ze saves their original values. A failed setup acquires no group
reference and attempts to restore the values it changed. If restoration also
fails, the snapshot retains exactly those failed keys, independently of the
successful-group reference counts.

A later setup retries outstanding restoration before recording new originals;
it cannot save a leftover recipe value as the original. Teardown also retries
saved values for parents with no remaining groups and, after the last IPv4
group, the namespace-wide values. Failed restores remain saved for another
attempt. A failed setup on a second parent does not restore global settings
still owned by an active group on the first parent.

- macvlan in PRIVATE mode, not bridge mode.
- Install the VIP with the parent's SUBNET prefix, for example /24, not /32. The
  macvlan then owns the connected route for the subnet.
- `conf.<parent>.arp_ignore=1`, `arp_filter=1`, `rp_filter=1`
- `conf.<macvlan>.arp_ignore=1`, `rp_filter=0`
- `conf.all.rp_filter=0`
- `net.ipv4.icmp_errors_use_inbound_ifaddr=1`

`conf.all.rp_filter=0` is required. The effective `rp_filter` is
`max(all, iface)`, so the macvlan cannot reach 0 while `all` is 1. This is the
ingredient that arp_ignore-only attempts miss.

Each ingredient was isolated in QEMU and proven necessary. `disable_ipv6=1` on
the virtual-MAC device is not needed and was dropped, although keepalived sets it.

## Constraints the code does not state

### The address owner is the exception

<!-- source: internal/plugins/vrrp/register.go -- vipMaskBits -->

When the VIP equals a real address of the parent, the router is the address owner.
The address is local to the parent, so `arp_ignore` cannot muzzle it and
virtual-MAC ownership is unreachable. Install such a VIP as a host route (/32),
never at the subnet prefix, or the box gains a duplicate connected route.
`vipMaskBits` makes that choice.

### The cold-start race is inherent

The first resolution after a neighbour-cache flush can cache the parent's real
MAC once. Every resolution after it returns the virtual MAC. keepalived's
`use_vmac` has the identical race. Do not try to remove it. An interop assertion
must flush and re-resolve to observe the steady state, and must not read the
cache once.

### IPv6 needs no ARP recipe, and does need DAD off

<!-- source: internal/plugins/vrrp/dataplane_linux.go -- accept_dad on the macvlan -->

Neighbour Discovery resolves the VIP to the virtual MAC natively. A Neighbour
Solicitation targets the VIP's solicited-node multicast group, which only the
macvlan joins, so the parent never competes and there is no cold race.

IPv6 does need `net.ipv6.conf.<vmac>.accept_dad=0`. A VRRP VIP lives on one router
at a time, so Duplicate Address Detection has nothing to detect. Leaving DAD on
makes the VIP tentative, and therefore unreachable, for about one second after
every promotion.

### The first IPv6 advert sources from the auto link-local. Do not reorder the FSM

<!-- source: internal/plugins/vrrp/transport/backend_linux.go -- macvlanLinkLocal source resolution -->
<!-- source: internal/component/iface/address_owner.go -- RegisterOwnedAddresses reconcile trigger -->

The kernel gives the macvlan a transient EUI-64 link-local at creation, for
example `fe80::200:5eff:fe00:20a`. Installing the VIPs replaces it, so at steady
state the macvlan holds `fe80::1` and the global VIP only. `macvlanLinkLocal`
returns the first non-tentative link-local, so after VIP install it returns
`fe80::1`.

The first advert runs before that. `InstallVIPs` calls
`iface.RegisterOwnedAddresses`, which is asynchronous: it records the address and
fires a reconcile trigger, and the kernel apply happens on a later reconcile pass.
The `SendAdvert` in the same dispatcher loop therefore resolves before `fe80::1`
exists and picks the auto link-local.

Reordering `promoteToMaster` to install the VIPs before the advert was tried and
proved ineffective against the keepalived IPv6 lab: ordering cannot win an
asynchronous race. A real fix gates the first advert on `fe80::1` being present,
which delays the mastership claim by about one advert interval, or makes the
address-owner registry apply synchronously. Both cost more than the symptom. The
auto link-local is a valid RFC 9568 link-local source, keepalived accepts it, and
election, failover and the dataplane are unaffected.

### The iface component overwrites the recipe

<!-- source: internal/plugins/vrrp/dataplane_linux.go -- reassertDataplaneSysctls -->

iface emits `arp_ignore`, `arp_filter` and `rp_filter` from unit config on every
apply, which clobbers the recipe. The engine re-asserts the recipe on every config
apply and logs any write failure.

### ICMP redirects identify the virtual router

Linux demultiplexes a received virtual destination MAC to that group's macvlan.
With `icmp_errors_use_inbound_ifaddr=1`, `net/ipv4/icmp.c::__icmp_send`
selects the source address from that inbound device. A non-owner Master's
redirect therefore names that group's VIP even when the reverse route to the
host uses the parent or another virtual router. This implements RFC 3768
Section 8.1, retained by RFC 9568 Section 8.1.1.

The knob applies to IPv4 ICMP errors throughout the network namespace. Ze saves
it with `all.rp_filter` on the first IPv4 group, reasserts both on config apply,
and restores the saved values after the last group. It leaves `send_redirects`
unchanged. Linux's normal redirect eligibility and rate limits still apply.

`TestVRRPRedirectSourceFollowsVirtualMAC` injects packets through two private
macvlans with distinct VIPs and virtual MACs on one parent, and captures the
forwarded packets and redirects on the peer. Its physical-MAC control expects
the parent's real source address. The test runs under `integration && linux`
against the runtime kernel; it exercises the product's netlink backend,
`vipCIDRs`, and the apply/reassert sysctl path.

### Namespace-wide settings are not restored on SIGKILL

An abrupt termination leaves both `all.rp_filter=0` and
`icmp_errors_use_inbound_ifaddr=1` in the network namespace. Normal teardown
restores their saved values. keepalived has the same abrupt-termination
property; crash-safe cleanup is not implemented.

## How it was found

Reasoning from ARP semantics went in circles. Three steps settled it, and they
are the method to repeat on the next kernel-behavior question:

1. Freeze test. `kill -STOP` on a working keepalived, then re-arp. The VIP still
   answered with the virtual MAC, which proves the KERNEL answers and no
   userspace ARP responder is involved. A pure-kernel recipe therefore exists.
2. Exhaustive state diff. Dump every `/proc/sys/net/ipv4` knob and every route for
   a working keepalived netns and for a failing hand-built one, then diff. The
   delta is the recipe. Do not eyeball a subset.
3. Isolate each ingredient by turning it off and re-testing, in the production
   topology. A bridge and a direct veth gave different race outcomes.
