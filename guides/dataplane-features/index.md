# Features by Data Plane: Linux Kernel and VPP

Ze can forward packets through the Linux kernel or through VPP. Most of Ze is
control plane and is the same on both. The features that write forwarding state
are different: each one has a kernel implementation, a VPP implementation, or
both. This page lists them, so that you can see what a VPP deployment keeps and
what it loses before you select it.

For how Ze starts and supervises VPP, see the [VPP guide](../vpp/index.md).

## How the data plane is selected

No single switch selects VPP. Four parts of the configuration each select
their own backend.

| Configuration | Values | Default | What it selects |
|---------------|--------|---------|-----------------|
| `interface backend` | `netlink`, `vpp` | `netlink` | Who creates and configures interfaces. PPPoE subscriber sessions and the automatic route writer follow this leaf. |
| `firewall backend` | `nft`, `vpp` | `nft` | Who carries the firewall and NAT rules: nftables, or VPP ACL and NAT44. |
| `traffic control backend` | `tc`, `vpp` | `tc` | Who shapes traffic: Linux queueing disciplines, or VPP policers. |
| `fib kernel`, `fib vpp` | a block for each writer | none | Who writes routes. Without a `fib` block, Ze loads the writer for the data plane that `interface backend` names. |

<!-- source: internal/component/iface/yang/ze-iface-conf.yang -- leaf backend -->
<!-- source: internal/component/firewall/yang/ze-firewall-conf.yang -- leaf backend -->
<!-- source: internal/component/traffic/yang/ze-traffic-control-conf.yang -- leaf backend -->
<!-- source: internal/component/plugin/server/startup_autoload.go -- appendDataPlaneWriter -->

The `vpp` value of each leaf exists only in a build that has the `ze_vpp` tag.
A build without the tag refuses the configuration and names the tag.
<!-- source: internal/component/iface/register.go -- ConfigNeeds for interface backend vpp -->
<!-- source: internal/component/firewall/register.go -- ConfigNeeds for firewall backend vpp -->
<!-- source: internal/component/traffic/register.go -- ConfigNeeds for traffic control backend vpp -->

Ze checks each selection against the features under it. Ze does not compare the
selections with each other. For example, a configuration with
`interface backend vpp` and the default `firewall backend nft` commits. The
section [Features that only program the kernel](#features-that-only-program-the-kernel)
says what that combination does.

## Summary

"Refused" means that the commit, the daemon start and `ze config validate` all
refuse the configuration and name the path. "Not built" in the VPP column means
that the feature has a kernel implementation only. Ze accepts its configuration
and programs the kernel, whichever interface backend is selected.

| Feature | Linux kernel | VPP | Detail |
|---------|--------------|-----|--------|
| Ethernet, dummy, loopback, VLAN units | Yes | Yes | [Interfaces](#interfaces) |
| Bridge, veth | Yes | Refused | [Interfaces](#interfaces) |
| GRE, GRETAP, IPIP, VXLAN tunnels | Yes | Yes, without the outer TTL leaf | [Interfaces](#interfaces) |
| IP6GRE, IP6GRETAP, SIT, IP6TNL, IPIP6 tunnels | Yes | Refused | [Interfaces](#interfaces) |
| WireGuard | Yes | Yes, with the VPP WireGuard plugin | [Interfaces](#interfaces) |
| XFRM interface, PPPoE client | Yes | Refused | [Interfaces](#interfaces) |
| Port mirroring | Yes | Yes | [Interfaces](#interfaces) |
| DHCP client, SLAAC, router advertisements, reverse path check | Yes | Refused | [Interfaces](#interfaces) |
| VRRP | Yes | Refused | [Interfaces](#interfaces) |
| Container networks | Yes | Refused | [Interfaces](#interfaces) |
| VRF on an interface unit | Not built | Not built | [Interfaces](#interfaces) |
| IPv4 and IPv6 route install | Yes | Yes | [Routes and MPLS](#routes-and-mpls) |
| MPLS push, swap and pop | Yes | Yes, without the path MTU | [Routes and MPLS](#routes-and-mpls) |
| SRv6 service routes | Yes | Yes, one Service SID for each policy | [Routes and MPLS](#routes-and-mpls) |
| Stateless packet filter | Yes | Yes, a subset | [Firewall and NAT](#firewall-and-nat) |
| Connection tracking matches and helpers | Yes | Refused | [Firewall and NAT](#firewall-and-nat) |
| NAT | Yes | IPv4 only, a subset | [Firewall and NAT](#firewall-and-nat) |
| Traffic shaping | Yes | HTB and TBF as policers | [Traffic control](#traffic-control) |
| Ingress policer | Yes | Refused | [Traffic control](#traffic-control) |
| PPPoE subscriber access | Yes | Yes, in a `ze_vpp` build | [Subscriber access](#subscriber-access) |
| L2TP | Yes | Not built | [Features that only program the kernel](#features-that-only-program-the-kernel) |
| IPsec | Yes | Not built | [Features that only program the kernel](#features-that-only-program-the-kernel) |
| Flow export packet sampling | Yes | Not built | [Features that only program the kernel](#features-that-only-program-the-kernel) |
| Policy routing | Yes | Not built | [Features that only program the kernel](#features-that-only-program-the-kernel) |
| Kernel tunables (`sysctl`) | Yes | Yes, applied to the Linux kernel | [Features that only program the kernel](#features-that-only-program-the-kernel) |

## Interfaces

`interface backend` selects the backend. The schema marks each node that only
some backends implement, and the commit refuses a marked node that the selected
backend does not implement.

| Feature | `netlink` | `vpp` | Limits |
|---------|-----------|-------|--------|
| `ethernet`, `dummy`, `loopback`, VLAN units, addresses, MTU, MAC address | Yes | Yes | On VPP a dummy interface is a VPP loopback. |
| `bridge` | Yes | Refused | |
| `veth` | Yes | Refused | |
| `tunnel` with `gre`, `gretap`, `ipip` | Yes | Yes | The `ttl` leaf is refused on VPP: the VPP GRE and IPIP calls that Ze uses carry no hop limit. |
| `tunnel` with `vxlan` | Yes | Yes | |
| `tunnel` with `ip6gre`, `ip6gretap`, `sit`, `ip6tnl`, `ipip6` | Yes | Refused | |
| `wireguard` | Yes | Yes | VPP needs `vpp plugins wireguard true`. The commit does not check that leaf. `ze doctor` reports `doctor-vpp-wireguard` when it is off. |
| `xfrm` | Yes | Refused | |
| `pppoe-client` | Yes | Refused | |
| `mirror` on a unit | Yes | Yes | The kernel mirrors with tc. VPP mirrors with SPAN, as a port mirror of the device. |
| `mpls enable` on a unit | Yes | Yes | |
| `dhcp`, `dhcpv6`, `delegated-prefix` on a unit | Yes | Refused | |
| `autoconf`, `accept-ra` on a unit | Yes | Refused | |
| `router-advertisement` on a unit | Yes | Refused | |
| `rpf-check` on a unit | Yes | Refused | |
| `vrrp` on a unit | Yes | Refused | VRRP needs macvlan devices and raw sockets. Native VPP VRRP is not built. |
| `vrf` on a unit | Not built | Not built | Ze refuses the leaf on both backends. |
| Container `network` | Yes | Refused | A network with `masquerade`, and a container that publishes a port, also need `firewall backend nft`. |

<!-- source: internal/component/iface/yang/ze-iface-conf.yang -- every ze:backend statement, and ze:validate "unimplemented-vrf" on leaf vrf -->
<!-- source: internal/plugins/vrrp/yang/ze-vrrp-conf.yang -- container vrrp, ze:backend "netlink" -->
<!-- source: internal/plugins/vrrp/groups.go -- validateGroups -->
<!-- source: internal/plugins/container/yang/ze-container-conf.yang -- list network, ze:backend "netlink" -->
<!-- source: internal/plugins/container/network/backend.go -- checkBackends -->
<!-- source: internal/plugins/iface/vpp/doctor.go -- doctor-vpp-wireguard -->
<!-- source: internal/plugins/iface/vpp/mplsinput.go -- ReplaceMPLSInput -->

A refusal names the path, the selected backend and the backends that implement
the node:

```
<path>: feature not supported by backend "vpp" (supported: netlink)
```

<!-- source: internal/component/config/backend_gate.go -- formatBackendError -->

On VPP, BGP still uses Linux sockets. The VPP Linux Control Plane plugin shows
each VPP interface to Linux as a TAP device, and BGP binds on that device. The
[VPP guide](../vpp/index.md#how-the-two-halves-fit-together) describes the namespace
limit of that arrangement.

## Routes and MPLS

One plugin writes routes for each data plane: `fib-kernel` for the kernel and
`fib-vpp` for VPP. Ze refuses a second writer for the same data plane. `fib p4`
is in the schema and reaches no data plane today.

| Feature | Kernel (`fib-kernel`) | VPP (`fib-vpp`) | Limits |
|---------|-----------------------|-----------------|--------|
| IPv4 and IPv6 routes from the system RIB | Yes | Yes | VPP routes go to the table that `fib vpp table-id` names. |
| Automatic writer for static, OSPF, IS-IS, LDP, RSVP-TE and BGP EPE routes | Yes | Yes | Ze loads the writer for the selected interface backend. A build without that writer refuses to start. |
| MPLS label push on an IP route | Yes | Yes | The kernel must have MPLS support. Ze refuses to start when it does not. |
| MPLS transit swap and pop | Yes | Yes | VPP uses MPLS table 0. VPP has no MTU for each route, so Ze does not install the path MTU of an entry and logs one warning. |
| MPLS push into a private table (RSVP-TE facility backup) | Yes | Refused when the entry arrives | VPP has no packet mark context. |
| SRv6 service routes | Yes | Yes | VPP: one encapsulation policy for each Service SID. Ze allocates no egress Service SID and installs no remote endpoint behavior. |
| `show mpls forwarding` | Yes | Yes | The command reads the data plane that `interface backend` selects. |

<!-- source: internal/plugins/fib/kernel/register.go -- ProgramsFIB, DataPlane "netlink" -->
<!-- source: internal/plugins/fib/vpp/register.go -- ProgramsFIB, DataPlane "vpp" -->
<!-- source: internal/plugins/fib/p4/register.go -- ProgramsFIB with no DataPlane -->
<!-- source: internal/component/plugin/registry/dataplane_writer.go -- DataPlaneWriter -->
<!-- source: internal/plugins/fib/vpp/mplsentry.go -- handleMPLSEntry, warnPathMTULocked -->
<!-- source: internal/plugins/fib/vpp/srv6.go -- acquirePolicy, addSRv6Steer -->
<!-- source: internal/plugins/fib/kernel/kernelcap_linux.go -- kernel capability enrolment -->
<!-- source: internal/core/mplsfib/forwarding.go -- reader registry by data plane name -->

`fib vpp` needs `interface backend vpp`. `fib-vpp` resolves each next-hop
interface to a VPP interface index, and that lookup refuses while another
interface backend is active. The commit does not check this pair, so the
refusal comes when a route is installed.
<!-- source: internal/component/iface/resolve.go -- ResolveVPPIndex -->
<!-- source: internal/plugins/fib/vpp/backend.go -- next-hop interface resolution -->

## Firewall and NAT

`firewall backend` selects the backend. The `vpp` backend translates filter
chains to VPP ACL rules and NAT chains to VPP NAT44. It refuses at commit every
rule that it cannot translate exactly, and the refusal names the table, the
chain and the term.

| Feature | `nft` | `vpp` | Limits on VPP |
|---------|-------|-------|---------------|
| Match on source or destination address, protocol, ICMP type, ICMPv6 type, TCP flags | Yes | Yes | |
| Match on source or destination port | Yes | Yes | One port range in a term. |
| `accept`, `drop` | Yes | Yes | |
| Rate limit in a term | Yes | Yes | IPv4 addresses and exact ports only. |
| `connection-state`, `connection-mark` | Yes | Refused | |
| `notrack`, `flow-offload`, `flowtable` | Yes | Refused | |
| `mark-set`, `connection-mark-set`, mark match | Yes | Refused | |
| `tcp-mss-clamp-to-pmtu`, TCP MSS set, DSCP match, DSCP set | Yes | Refused | |
| `reject`, `redirect`, `counter`, `log` | Yes | Refused | VPP ACL denies without a response and has no named counter for a rule. |
| `jump`, `goto`, `return`, chains that are not base chains | Yes | Refused | A VPP ACL is a flat list of rules. |
| Named sets | Yes | Refused | |
| Match on input or output interface | Yes | Refused | A VPP ACL is bound to an interface. |
| Match on `destination-local`, link-layer unicast, IPsec transform | Yes | Refused | |
| Conntrack helpers (`system conntrack module`) | Yes | Refused | |
| `snat`, `masquerade` | Yes | IPv4 only | No destination or protocol match. A source address match is accepted and has no effect: VPP NAT44 applies to all traffic on the inside interface. No port mapping and no flags on `masquerade`. |
| `dnat` | Yes | IPv4 only | Needs a protocol match and a destination port match. |
| The `ruleset`, `group` and `conntrack` show commands | Yes | Refused | |

<!-- source: internal/component/firewall/yang/ze-firewall-conf.yang -- every ze:backend "nft" statement -->
<!-- source: internal/component/firewall/yang/ze-firewall-cmd.yang -- ruleset, group, conntrack, ze:backend "nft" -->
<!-- source: internal/plugins/firewall/vpp/verify.go -- Verify, verifyNATChain, verifyNATSourceMatches, verifyNATDestMatches, verifyClassifyConstraints -->

Control-plane policing, policy routing, the local DDoS responder and the
FlowSpec firewall bridge add their own tables to the same backend. The `vpp`
backend checks those tables against the same limits when Ze applies them.
<!-- source: internal/component/firewall/backend.go -- RunVerifier, called from ApplyAll -->
<!-- source: internal/component/firewall/yang/ze-firewall-conf.yang -- leaf flush-on-shutdown, table producers share one backend -->

## Traffic control

`traffic control backend` selects the backend. The `vpp` backend has no
queueing scheduler. It builds each class as a VPP policer, and it refuses at
commit what a policer cannot do.

| Feature | `tc` | `vpp` | Limits on VPP |
|---------|------|-------|---------------|
| `htb`, `tbf` | Yes | Yes | At least one class. With more than one class, each class needs a `protocol` or `dscp` filter. |
| `hfsc`, `fq`, `fq_codel`, `sfq`, `netem`, `prio` | Yes | Refused | |
| Filter on `protocol`, `dscp` | Yes | Yes | One value steers to one class. |
| Filter on `mark` | Yes | Refused | The Linux packet mark has no VPP equivalent. |
| Root `rate` of a qdisc | Yes | Refused | The class policers carry the rates. |
| Ingress policer | Yes | Refused | The L2TP shaper uses it for the upload rate of a subscriber. |

<!-- source: internal/component/traffic/yang/ze-traffic-control-conf.yang -- qdisc-type, filter type, leaf rate with ze:backend "tc" -->
<!-- source: internal/plugins/traffic/vpp/verify.go -- Verify, verifyQdiscType, errIngressPolicerNotSupportedByBackend, errFilterMarkNotSupportedByBackend -->

## Subscriber access

| Feature | Kernel | VPP | Limits |
|---------|--------|-----|--------|
| PPPoE access interfaces | Yes | Yes, in a `ze_vpp` build | Sessions follow `interface backend`. On VPP the configuration must enable `vpp` and `vpp lcp`, and Ze loads the VPP PPPoE plugin. The commit refuses a backend that has no session provider in the build. |
| L2TP | Yes | Not built | L2TP sessions use the kernel L2TP and PPP drivers. |
| PPPoE client | Yes | Refused | |

<!-- source: internal/component/l2tp/pppoe/backend_gate.go -- validateBackendGate -->
<!-- source: internal/component/l2tp/plugins/pppoe-vpp/startup.go -- registerStartup, startupRequired -->
<!-- source: internal/component/l2tp/register_kernelcap_linux.go -- L2TP kernel capabilities -->

## Features that only program the kernel

These features have one implementation, and it writes Linux kernel state. They
have no backend leaf, and the commit does not look at `interface backend` for
them.

| Feature | What Ze programs |
|---------|------------------|
| IPsec | Kernel XFRM states and policies. The configuration has no way to select another IPsec data plane. |
| L2TP | Kernel L2TP tunnels and PPP interfaces. |
| Flow export packet sampling | A tc sample action on each listed interface, read through the kernel psample channel. |
| Policy routing | `ip rule` entries, and firewall tables on the selected firewall backend. |
| `firewall backend nft` | nftables tables in the kernel. |
| `traffic control backend tc` | Queueing disciplines on kernel interfaces. |
| `sysctl` | Kernel tunables. They are applied to the Linux kernel whatever the data plane, so a setting about kernel behaviour works the same on a VPP box. A key about packet forwarding, such as `rp_filter`, acts only on the packets that reach Linux, not on the packets VPP forwards. |

<!-- source: internal/component/ike/engine/testport.go -- ikeDataplaneName -->
<!-- source: internal/component/l2tp/register_kernelcap_linux.go -- L2TP kernel capabilities -->
<!-- source: internal/plugins/flowexport/yang/ze-flowexport-conf.yang -- container sampling -->
<!-- source: internal/component/sysctl/sysctl.go -- kernel tunables -->

With `interface backend vpp`, VPP owns the network cards and forwards transit
packets itself. Linux sees a VPP interface only as the TAP device that the VPP
Linux Control Plane plugin creates. A feature in the table above that handles
packets then acts on the packets that reach Linux through that TAP device. It
does not act on the packets that VPP forwards. Ze accepts such a configuration
today and does not warn about it. `sysctl` is the exception for settings about
kernel behaviour: those apply to the Linux kernel as they do on any data plane.

To filter or shape the traffic that VPP forwards, select `firewall backend vpp`
and `traffic control backend vpp`. IPsec, L2TP, flow export sampling and policy
routing have no VPP implementation.

BFD and the DHCP server have no code that is specific to a data plane.
