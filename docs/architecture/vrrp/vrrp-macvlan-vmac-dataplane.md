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

### A Backup needs a filter, because the macvlan forwards without an address

<!-- source: internal/plugins/vrrp/backupfilter.go -- backupFilterTables -->
<!-- source: internal/plugins/vrrp/instance.go -- run, doInstallVIPs, doRemoveVIPs -->

The macvlan exists in every state, so its MAC and the IPv6 link-local address
derived from it stay stable across failovers. Only the virtual addresses wait for
promotion. A frame sent to the Virtual Router MAC that reaches a Backup, for
example by unknown-unicast flooding after a failover, is received by the macvlan.
With forwarding enabled the kernel forwards it, with or without an address on the
macvlan. RFC 3768, RFC 5798 and RFC 9568 Section 6.4.2 say a Backup MUST discard
it.

While an instance is not Active, the firewall table registry carries one table
under the owner `vrrp-backup`:

| Table | Family | Hook | Drops |
|-------|--------|------|-------|
| `ze_vrrp_backup` | inet | prerouting | every packet the Backup's macvlan receives |

A private macvlan receives a unicast frame only when the destination MAC is its
own. A broadcast or multicast frame also reaches the parent, which handles it,
and the advertisement socket listens on the parent. The worker sets the entry when
it starts, a demotion sets it again before the addresses are removed, a promotion
withdraws it after they are installed, and the worker withdraws it when it stops.
`TestVRRPBackupDoesNotForwardVirtualMACFrames` sends a transit datagram to the
Virtual Router MAC: with no filter the kernel forwards it, with the filter it does
not, and with the filter withdrawn and the address installed it is forwarded.

### The address owner needs a filter, not a sysctl

<!-- source: internal/plugins/vrrp/register.go -- vipMaskBits -->
<!-- source: internal/plugins/vrrp/ownerfilter.go -- ownerFilterTables -->

When the VIP equals a real address of the parent, the router is the address owner.
The address is local to the parent, so `arp_ignore` cannot muzzle it: the parent
answers ARP and Neighbor Solicitations for it with its physical MAC. Ze installs
the VIP on the macvlan too, as a host route (/32 or /128), never at the subnet
prefix, or the box gains a duplicate connected route. `vipMaskBits` makes that
choice. The macvlan then answers with the virtual MAC, and the parent's answer
competes with it.

RFC 3768 Section 8.2 and RFC 9568 Sections 8.1.2 and 8.2.2 forbid the physical-MAC
answer, so Ze drops it on its way out. While the macvlan holds an owned VIP, the
firewall table registry carries two tables under the owner `vrrp-owner`:

| Table | Family | Hook | Drops |
|-------|--------|------|-------|
| `ze_vrrp_owner_arp` | arp | output | an ARP reply leaving the parent with the VIP as sender protocol address |
| `ze_vrrp_owner_nd` | ip6 | output | a Neighbor Advertisement leaving the parent with the VIP as target |

The macvlan's own answers leave through the macvlan, so no rule names them. ARP
requests and Neighbor Solicitations the router sends from the parent are left
alone: they are its own resolution traffic. The tables are published before the
VIP is installed and withdrawn after it is removed, so once the macvlan gives the
VIP up the parent answers for its own address again.

The arp table needs `CONFIG_NF_TABLES_ARP`, which `gokrazy/kernel/kernel.config`
builds in. The arp family numbers its hooks in its own space (output is 1, not
the inet 3), which `lowerHook` in the nft backend handles.

A /128 owner VIP on the macvlan and the same address on the parent both pass
DAD. The macvlan runs none (`accept_dad=0`), and a parent re-running DAD does not
see the macvlan's copy, because a private macvlan receives only frames arriving
from the wire. `TestVRRPOwnerAnswersWithVirtualMACOnly` asserts both addresses
leave DAD usable, then resolves the VIP from a peer and reads the MAC in each ARP
reply and in each Neighbor Advertisement's Target Link-Layer Address option. Its
control phase, with no filter, captures the physical MAC; with the filter, only
the virtual MAC arrives.

### The cold-start race is inherent

The first resolution after a neighbour-cache flush can cache the parent's real
MAC once. Every resolution after it returns the virtual MAC. keepalived's
`use_vmac` has the identical race. Do not try to remove it. An interop assertion
must flush and re-resolve to observe the steady state, and must not read the
cache once.

### IPv6 needs no parent ARP recipe, and does need DAD and ARP off on its macvlan

<!-- source: internal/plugins/vrrp/dataplane_linux.go -- accept_dad and arp_ignore on the IPv6 macvlan -->

Neighbour Discovery resolves the VIP to the virtual MAC natively. A Neighbour
Solicitation targets the VIP's solicited-node multicast group, which only the
macvlan joins, so the parent never competes and there is no cold race.

IPv6 does need `net.ipv6.conf.<vmac>.accept_dad=0`. A VRRP VIP lives on one router
at a time, so Duplicate Address Detection has nothing to detect. Leaving DAD on
makes the VIP tentative, and therefore unreachable, for about one second after
every promotion.

The IPv6 group's macvlan also gets `net.ipv4.conf.<vmac>.arp_ignore=8`, which
answers no ARP request at all. Linux answers ARP for any local IPv4 address on
any interface, so without it the IPv6 macvlan answered who-has for the parent's
IPv4 addresses and for an IPv4 group's VIP with the IPv6 virtual MAC, and a LAN
host then followed the IPv6 group's mastership for an IPv4 address. QEMU showed
it: `TestVRRPOwnerAnswersWithVirtualMACOnly` saw `00:00:5e:00:02:0a` answer an
ARP request for an IPv4 owner address.

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

IPv6 needs no knob. `net/ipv6/ndisc.c::ndisc_send_redirect` always takes the
redirect's source from a link-local address of the device the packet arrived
on, so a packet sent to a group's virtual MAC draws a redirect from a link-local
address of that group's macvlan, and a packet sent to the physical MAC draws one
from the parent's. That is the attribution RFC 9568 Section 8.2.1 asks for ("it
has to determine to which Virtual Router the packet was sent"). Linux sends an
ICMPv6 redirect only when the route forwards the packet back out the device it
arrived on (`net/ipv6/ip6_output.c::ip6_forward`), so a packet taken in on a
macvlan and routed out the parent draws no redirect at all, and none is ever
attributed to the wrong router. The test routes each probe out its ingress
device to meet that condition. Which of the
macvlan's link-local addresses Linux picks (the configured virtual link-local or
the one it derives from the virtual MAC) is the kernel's choice, and no test
pins it. `TestVRRPIPv6RedirectFollowsVirtualMAC` runs two IPv6 groups on one
parent and asserts that each redirect's source is held by the macvlan the packet
was sent to and by no other device.

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
