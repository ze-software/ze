# Use Cases

Use these pages when you want Ze to play a concrete role in a network.

Each example brings together the Ze configuration, the neighboring router configuration, and the lab checks for that setup. The feature guides describe individual settings; these pages explain how to combine them for a network role.

To see how complete each use case is, the [completion by use case](../quality/rfc-compliance/profiles/) page counts the RFC requirements verified by its tests.

## Examples

| Example | Ze role | Network side | Evidence |
| --- | --- | --- | --- |
| [AS112 anycast DNS inside a network](as112/) | Authoritative AS112 DNS sink plus BGP origin for the AS112 covering prefixes | VyOS, Junos, Cisco IOS XR, BIRD, and FRR receive the routes from Ze | Existing AS112 interop scenarios cover the DNS server, BGP redistribution, origin AS, communities, and the covering-prefix guard |
| [ExaBGP migration](exabgp-migration/) | Ze as a replacement engine for an existing ExaBGP deployment | Existing ExaBGP config and process scripts are converted or bridged into Ze | The migration command, compatibility bridge, and staged workflow are linked to implementation sources |
| [BGP performance testing with Ze](bgp-performance/) | Ze provides the sender, receiver, timing, and JSON report for a route-server performance test | FRR, BIRD, Ze, or another DUT receives and exports generated BGP routes | Existing Ze performance tooling supplies the report; the page links to AMS-IX, LINX, and bgperf prior art |
| [Route server at an IXP](route-server/) | RFC 7947 route server with member policy, ADD-PATH, and RPKI validation | IXP members establish one eBGP session and receive eligible member routes | Configuration checks, replay, Adj-RIB-Out inspection, and restart behavior |
| [Transit edge with RPKI](transit-edge-rpki/) | Dual-transit Internet edge with origin validation and deterministic preference | Two transit providers advertise full or partial tables | RPKI state changes, failover, prefix-limit, memory, and convergence checks |
| [FlowSpec injection](flowspec-injection/) | Authorized source or relay for FlowSpec rules | Mitigation automation and enforcing BGP peers | Commit, propagation, enforcement, expiry, and withdrawal checks |
| [Chaos-tested BGP peering](chaos-tested-peering/) | Peer configuration tested against deterministic failures | A controlled peer or route-server scenario | Recorded seed, replay, property deadlines, shrinking, and recovery checks |
| [AS-path topology](as-path-topology/) | Looking Glass graph for one prefix and its visible paths | Authenticated workbench or public Looking Glass clients | Peer, prefix, route, and graph inspection before and after changes |
| [Home and SMB FTTP CPE](fttp-cpe/) | PPPoE WAN client, DHCPv6-PD + RA, DHCPv4 server, LAN DNS forwarding, nftables MSS/NAT | ISP access concentrator (`accel-ppp`) and LAN hosts | Topology interop against `accel-ppp` plus PPPoE, DHCP, and DNS `.ci` tests |
| [Site-to-site IPsec and WireGuard VPN](site-to-site-vpn/) | Route-based IKEv2 IPsec on Linux XFRM VTI interfaces or kernel WireGuard tunnels | strongSwan 6.1.0 responder or WireGuard peer | strongSwan interop scenarios, QEMU kernel cipher proofs, and reload `.ci` tests |
| [Campus and IGP edge](igp-edge/) | OSPFv2 + single-hop BFD or dual-stack IS-IS with VRRPv3 and uplink tracking | FRR IGP peers and keepalived VRRPv3 standby router | FRR OSPF/IS-IS/BFD interop and keepalived VRRPv3 Docker and QEMU labs |
| [Fleet configuration management](fleet-management/) | Central TLS hub enrolling managed devices, pushing live configs, and resolving divergence | Managed Ze devices booting from hub or local cache | 32 managed-fleet `.ci` tests covering enrollment, freeze, live push, adopt, and revert |
| [Immutable appliance A/B lifecycle](appliance-lifecycle/) | Gokrazy read-only appliance with `--testboot` A/B OTA, `commit confirmed`, and ZeFS backups | Operator workstation running `./ze-host` over HTTPS and SSH | QEMU `appliance-ota` and `appliance-config` labs plus commit-confirmed and backup `.ci` tests |

## What belongs here

Each use case includes:

- A topology with clear addresses and ASNs.
- A full Ze config for the role.
- The neighboring router or daemon config.
- The verification command or lab scenario that tests the important behavior.

Reference-only material stays in [Documentation](../docs/). Raw interop evidence stays in [Labs](../labs/).
