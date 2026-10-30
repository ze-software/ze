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

`conf.all.rp_filter=0` is required, because the macvlan cannot reach 0 while
`all` is 1 ([LL-IPV4-CONF-ALL-WINS](../linux-limitations.md#ll-ipv4-conf-all-wins)). This is the
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
| `ze_vrrp_backup` | inet | prerouting | host-type unicast received by the Backup's PRIVATE macvlan |

A private macvlan receives a unicast frame only when the destination MAC is its
own. The discard combines the exact input interface with `MatchUnicast`, lowered
to nft `meta pkttype host`. Broadcast and multicast, including VRRP and IPv6 ND,
do not match. Both advertisement receive and transmit sockets bind to the
macvlan, while the engine retains the parent's identity for group lookup.
The worker sets the entry when it starts; a demotion sets it before addresses
are removed; a promotion withdraws it after installation; shutdown withdraws it.
`TestVRRPBackupDoesNotForwardVirtualMACFrames` sends a transit datagram to the
Virtual Router MAC: with no filter the kernel forwards it, with the filter it does
not, and with the filter withdrawn and the address installed it is forwarded.

Listening on the parent alone loses advertisements from a real VRRP peer.
Linux's `macvlan_handle_frame` looks up the source MAC of a multicast frame.
When it matches a local PRIVATE macvlan, the kernel delivers the frame to that
macvlan and consumes the parent's copy. VRRP peers share the same virtual source
MAC, so this is the normal election path, not reflected local traffic.
`macvlan_broadcast_one` preserves its multicast packet type. For unicast, the
same handler selects the macvlan by destination MAC and sets `PACKET_HOST`.
The nft `meta pkttype` getter reads that classification unchanged for these
packets.

This explains the tracked-priority split brain: keepalived received Ze's
priority-50 advertisements and became Active at 100, but the old parent-bound Ze
socket never received the winning advertisements. Joining on the macvlan and
narrowing the Backup discard are one fix: otherwise the filter would block the
election immediately after demotion.

Sources: Linux
[`macvlan_handle_frame` and `macvlan_broadcast_one`](https://github.com/torvalds/linux/blob/master/drivers/net/macvlan.c),
[`nft_meta_get_eval`](https://github.com/torvalds/linux/blob/master/net/netfilter/nft_meta.c),
and keepalived's
[`open_vrrp_read_socket`](https://github.com/acassen/keepalived/blob/v2.4.3/keepalived/vrrp/vrrp.c),
which binds to the instance's VMAC device when `use_vmac` selects it.

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

### IPv6 needs no parent ARP recipe, and does need DAD and ARP off and the router role on its macvlan

<!-- source: internal/plugins/vrrp/dataplane_linux.go -- vmacV6Sysctls: accept_dad, arp_ignore and forwarding on the IPv6 macvlan -->

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

The macvlan also gets `net.ipv6.conf.<vmac>.forwarding=1`. The kernel answers a
Neighbor Solicitation for a virtual address, and it sets the Router flag of that
solicited Neighbor Advertisement from the receiving device's own forwarding
knob, which RFC 9568 Section 6.4.3 requires set. Per-device IPv6 forwarding
moves no packet (`conf/all/forwarding` decides that); it makes the device a
router on the link, so it sends no Router Solicitation and ignores Router
Advertisements. A write to `conf/all/forwarding` is copied onto every device, so
the engine re-asserts the IPv6 knobs on every config apply as it does the IPv4
recipe. `TestVRRPSolicitedNACarriesRouterFlag` captures the R flag set with host
forwarding off, and clear once the macvlan's knob is cleared.

### The Active macvlan joins its addresses' solicited-node groups before it holds them

<!-- source: internal/plugins/vrrp/solicitednode_linux.go -- joinSolicitedNode, leaveSolicitedNode, syncReceiveFilter, linkIsPeer -->
<!-- source: internal/plugins/vrrp/instance.go -- doInstallVIPs, doRemoveVIPs, reconfigure -->
<!-- source: internal/plugins/vrrp/fsm/fsm.go -- masterConfigUpdated -->

RFC 9568 Section 6.4.3 requires the Active router to be a member of the
Solicited-Node multicast group of each IPv6 virtual address. The kernel joins
that group itself only when the address is added, and the address is added
late. A promotion registers the addresses with `iface.RegisterOwnedAddresses`,
which records them and triggers a reconcile; the iface reconcile installs them
later, on its own worker. Even then the kernel joins the group from the DAD work
the add queues (`addrconf_dad_begin`, `net/ipv6/addrconf.c`), and the join's
receive-filter update is deferred again: for a device with
`ndo_change_rx_flags`, which a macvlan has, `__dev_set_rx_mode` queues it to the
netdev work queue (`net/core/dev_addr_lists.c`). Until that work runs,
`macvlan_handle_frame` finds the group in neither port filter and drops a
host's Neighbor Solicitation before the macvlan sees it. These kernel
statements were read in the Linux 7.2.9 source, the kernel.org stable tag
`v7.2.9`, at the paths named. The VLAN step below was observed, not read:
`TestSolicitedNodeJoinReachesTheBottomOfAVLANStack` finds the group on every
device of a two-VLAN stack when the join returns.

So `doInstallVIPs` joins the groups before it registers the addresses. For each
IPv6 virtual address it adds `33:33:ff` followed by the address's low 24 bits
to the macvlan with `PACKET_ADD_MEMBERSHIP` (`PACKET_MR_MULTICAST`) on a packet
socket of protocol 0, which receives nothing. The membership is link-layer
only: it sends no MLD report and creates no IPv6 state. `PACKET_ADD_MEMBERSHIP`
adds the group to the macvlan's list but leaves the filter update queued, like
the kernel's own join, so Ze then sends one `RTM_NEWLINK` that changes nothing
to the macvlan and to each device under it. `do_setlink` ends in
`netif_rx_mode_sync` (`net/core/rtnetlink.c`), which runs a queued update
inline. The macvlan's update is `macvlan_set_mac_lists`, which recomputes the
macvlan's and its port's filters and syncs the groups onto the parent, whose
own update is queued in turn; a VLAN parent's update (`vlan_dev_set_rx_mode`)
syncs them one device further down. So the walk follows each device's link
(`IFLA_LINK`) to the bottom of the stack. It ends after a device that reports
no link, and after a device whose link is its peer rather than a device under
it: a veth or a netkit device reports its peer's index as its link
(`veth_get_iflink`, `drivers/net/veth.c`; `netkit_get_iflink`,
`drivers/net/netkit.c`), and nothing this end's groups reach is past it. That
holds in whichever namespace the peer is, so a macvlan on a container's `eth0`,
a veth whose peer is in the host namespace, is synced without error. It also
ends after a device it already synced. A device under the stack that is in
another namespace is reported as an error: a request from this namespace cannot
reach it, so its filter catches up only when the kernel's queued update runs.
The kernel stacks at most eight devices (`MAX_NEST_DEV`,
`include/linux/netdevice.h`, enforced in `__netdev_upper_dev_link`), so the walk
syncs up to eight and reports a ninth as an error. When the promotion returns,
every filter down the stack admits the groups. A bond's members are not on the
`IFLA_LINK` chain, so their filters follow the bond's on the kernel's schedule.

The socket is the membership's whole lifetime: closing it drops every group it
holds. `doRemoveVIPs` closes it after the addresses are deregistered, on
demotion and on shutdown, so a Backup holds no membership of its own. Its
macvlan still receives a group while the kernel holds the address, until the
reconcile removes it. A join that fails is logged with its cause and the
commands that show the device stack and its groups, and does not stop the
addresses being registered.

A commit that changes the virtual addresses of an Active router reaches
`doInstallVIPs` again (`masterConfigUpdated`, `fsm/fsm.go`), and the join
replaces the membership: it opens the new socket before it closes the old one,
so a group both address sets hold never leaves the filter. A replace whose new
socket cannot be opened, or whose device cannot be found, is logged and keeps
the old socket, so the old address set's groups stay joined until the next
leave closes it.

One case is not covered. The membership is held by interface index, and the
kernel drops it when the device goes away (`packet_dev_mclist_delete` on
`NETDEV_UNREGISTER`, `net/packet/af_packet.c`). If the macvlan is deleted and
created again while the router stays Active, nothing joins on the new device:
the instance's link watch covers the parent and the tracked interfaces, not the
macvlan, so no event reaches `doInstallVIPs`. The kernel still joins each
group itself once the reconcile adds the address to the new device, so the
loss is the same early window this section exists to close, not a lasting one.

`TestVRRPActiveJoinsSolicitedNodeBeforeItsAddressIsInstalled` drives the
promotion through the FSM and the real address-owner registry, which installs
nothing, as in production when the promotion returns. It requires the group in
the macvlan's multicast list and synced onto the parent's before the address
exists. It then swaps one virtual address through `reconfigure`, the commit
path, while a second goroutine samples the multicast list: the new group
appears, the old one goes, and the group of `fe80::1`, held by both sets, is
never missing. With the old membership closed before the new one opens, that
check failed 20 of 20 runs. After a demotion the group is absent again. A
second promotion then plays the
reconcile with the product's netlink backend: once the address is usable, the
host sends one solicitation with no retry, and the answer must carry the
virtual MAC. On Linux 7.2.9, without the join the test failed 20 of 20 runs,
and without the `RTM_NEWLINK` it failed 20 of 20 at the parent check; with
both it passed 40 of 40. The same deferred update cost
`TestVRRPNonOwnerMasterAnswersNDWithVirtualMACOnly` its answer in 8 of 25 runs
before its readiness helper gained the same no-op `RTM_NEWLINK`: that test adds
the address itself and waits for the kernel's own join to reach the filter.

`TestSolicitedNodeJoinReachesTheBottomOfAVLANStack` stacks the product's
macvlan on a VLAN on a VLAN on a veth, joins, and requires the group in the
list of every device down to the veth when the join returns. Without the stop
at the veth's peer and the stop at a synced device it failed 5 of 5 runs, the
walk running between the veth and its peer past its bound. Limiting the walk to
the macvlan and its parent did not turn it red in 20 runs: the kernel's queued
update of the lower VLAN ran before the test read the list, so that read cannot
tell the walk's depth from the kernel's own catch-up. The walk itself is proven
over a fake device chain, through the lookup and no-op functions
`syncReceiveFilter` takes, by the `TestReceiveFilterWalk` unit tests: three
devices get three no-ops, top down; a veth ends the walk whether its peer is
here or in another namespace; a lower device in another namespace is an error;
eight devices are walked whole and a ninth is an error. Each went red under the
break of the step it covers.

### The macvlan derives no link-local from the virtual MAC

<!-- source: internal/plugins/iface/netlink/macvlan_linux.go -- setMacvlanAddrGenNone -->

iface creates every owned macvlan with `addr_gen_mode` none, set before the
device comes up, because the kernel adds the default Modified EUI-64 link-local
on link-up ([LL-IPV6-LINKLOCAL-FROM-MAC](../linux-limitations.md#ll-ipv6-linklocal-from-mac))
and a later mode change does not remove it. RFC 9568 Section 7.4
forbids building an interface identifier from the Virtual Router MAC. The only
link-local the macvlan ever holds is the configured virtual link-local, which
`InstallVIPs` puts there on promotion. Two consequences follow. A Backup's
macvlan holds no IPv6 address at all, so it sends no MLD report or Router
Solicitation with the virtual MAC as its source. And Linux sources an ICMPv6
redirect from a link-local of the device the triggering packet arrived on,
which is now only the virtual link-local, the address the host used as its
first hop (RFC 9568 Section 8.2.1). `TestVRRPMacvlanDerivesNoLinkLocalFromVirtualMAC`
and `TestVRRPIPv6RedirectSourcedFromVirtualLinkLocal` assert both, each against
a control macvlan created with the kernel's default mode.

`random` (stable privacy with a random secret) was the alternative. It would
keep a router-unique link-local on the macvlan, but Linux keeps the oldest
link-local first when it picks a redirect source, so the redirect would still
not come from the virtual link-local.

### The first IPv6 advert after a promotion is skipped. Do not reorder the FSM

<!-- source: internal/plugins/vrrp/transport/backend_linux.go -- macvlanLinkLocal source resolution -->
<!-- source: internal/component/iface/address_owner.go -- RegisterOwnedAddresses reconcile trigger -->

The macvlan holds no link-local until `InstallVIPs` puts `fe80::1` there (see
above), so at steady state it holds `fe80::1` and the global VIP only, and
`macvlanLinkLocal` returns `fe80::1`.

The first advert runs before that. `InstallVIPs` calls
`iface.RegisterOwnedAddresses`, which is asynchronous: it records the address and
fires a reconcile trigger, and the kernel apply happens on a later reconcile pass.
The `SendAdvert` in the same dispatcher loop therefore resolves before `fe80::1`
exists, finds no link-local, and is counted `{reason=no-link-local}`; the next
advert, one interval later, carries `fe80::1`.

Reordering `promoteToMaster` to install the VIPs before the advert was tried and
proved ineffective against the keepalived IPv6 lab: ordering cannot win an
asynchronous race. A real fix gates the first advert on `fe80::1` being present,
which delays the mastership claim by about one advert interval, or makes the
address-owner registry apply synchronously.

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
the parent's real source address. The test runs under `linux` and re-executes
in private user/network namespaces against the running kernel; it exercises
the product's netlink backend, `vipCIDRs`, and the apply/reassert sysctl path.

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
