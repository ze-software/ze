# Interface Management

Ze manages Linux network interfaces via pure netlink (no iproute2 shell-outs).
JunOS-style two-layer model: physical interfaces with logical units.

<!-- source: internal/component/iface/schema/ze-iface-conf.yang — interface container -->
<!-- source: internal/component/iface/register.go — registration -->

## Capability Table

| Category | Feature | Status | Priority |
|---|---|---|---|
| **Interface Types** | Ethernet (physical) | have | |
| | Dummy (virtual) | have | |
| | Veth pairs | have | |
| | Bridge (basic) | have | |
| | VLAN 802.1Q | have | |
| | Loopback | have | |
| | Bonding / LACP | missing | high |
| | VXLAN | missing | medium |
| | GRE / IPIP / SIT tunnels | missing | medium |
| | IP6GRE / ERSPAN / GRETAP | missing | lower |
| | WireGuard | missing | lower |
| | MACsec | missing | lower |
| | MACVLAN | missing | lower |
| | Geneve | missing | lower |
| | PPPoE | missing | lower |
| | L2TPv3 | missing | lower |
| | WiFi | missing | lower |
| | VTI (IPSec tunnel) | missing | lower |
| | QinQ (802.1ad) | missing | lower |
| **Logical Model** | Two-layer physical + unit | have | |
| | Unit 0 implicit | have | |
| | VLAN units create subinterfaces | have | |
| **Lifecycle** | Create (dummy, veth, bridge, VLAN) | have | |
| | Delete | have | |
| | Auto-up on create | have | |
| | Admin state control (explicit up/down) | missing | high |
| | Interface rename | missing | lower |
| **Address Management** | Add/remove IPv4/IPv6 CIDR | have | |
| | DAD-aware monitoring | have | |
| | MAC address set/get | missing | high |
| | Gratuitous ARP on add | missing | medium |
| | Neighbor table (ARP/NDP) | missing | lower |
| **DHCP** | DHCPv4 (RFC 2131, full DORA) | have | |
| | DHCPv6 (SARR, IA_NA, IA_PD) | have | |
| | Concurrent v4+v6 | have | |
| | Direct netlink install | have | |
| | Bus events (acquired/renewed/expired) | have | |
| | DHCPv6 proper Renew (not re-solicit) | missing | medium |
| | DHCP relay | missing | lower |
| | DHCP server | missing | lower |
| **Monitoring** | Netlink multicast (link + addr) | have | |
| | Virtual iface state detection | have | |
| | 8 bus topics, JSON payloads | have | |
| | Interface statistics/counters | missing | high |
| | Persistent counter tracking | missing | medium |
| **Per-Interface Tuning** | IPv4 forwarding | have | |
| | ARP filter / ARP accept | have | |
| | IPv6 autoconf (SLAAC) | have | |
| | IPv6 accept-ra (boolean) | have | |
| | IPv6 forwarding | have | |
| | Accept-RA levels (0/1/2/3) | missing | high |
| | Proxy ARP | missing | medium |
| | ARP announce / ARP ignore | missing | medium |
| | RPF / source validation | missing | medium |
| | TCP MSS clamping (v4+v6) | missing | medium |
| | Directed broadcast | missing | lower |
| **IPv6 Extended** | EUI-64 address generation | missing | medium |
| | DAD configuration (messages, accept) | missing | medium |
| | Custom interface identifiers | missing | lower |
| **Traffic Mirroring** | Ingress/egress via tc mirred | have | |
| | Idempotent setup/cleanup | have | |
| | Traffic redirect (vs mirror) | missing | lower |
| **Traffic Control** | QoS / shaping | missing | lower |
| | Policing | missing | lower |
| | Queuing disciplines | missing | lower |
| **Migration** | Make-before-break 4-phase | have | |
| | BGP readiness signaling | have | |
| | Per-phase rollback | have | |
| **BGP Integration** | Reactor subscribes to addr events | have | |
| | Listener start/stop on addr change | have | |
| | bgp/listener/ready publish | have | |
| **Bridge Features** | Create and bring up | have | |
| | STP | missing | medium |
| | VLAN filtering | missing | medium |
| | Add/remove member ports | missing | medium |
| | Multicast snooping | missing | lower |
| | Port isolation | missing | lower |
| | Ageing/forward delay/hello/max age | missing | lower |
| **Bonding** | Mode selection | missing | high |
| | Hash policy | missing | high |
| | LACP rate | missing | high |
| | MII monitoring | missing | high |
| | Min active links | missing | high |
| | Member management | missing | high |
| **VRF** | YANG leaf exists | partial | high |
| | Route table isolation | missing | high |
| | Per-VRF address assignment | missing | high |
| | VRF-aware DHCP | missing | medium |
| **Gateway Redundancy** | VRRP / keepalived | missing | medium |
| | Virtual MAC | missing | medium |
| | State monitoring/failover | missing | medium |
| **Physical Layer** | Speed / duplex / autoneg | missing | medium |
| | Hardware offload (GRO/GSO/TSO/LRO) | missing | medium |
| | Ring buffer sizing | missing | lower |
| | RPS / RFS | missing | lower |
| | ethtool integration | missing | medium |
| **Security** | 802.1X / EAPoL | missing | lower |
| | Storm control | missing | lower |
| **Configuration** | YANG model (all types + units) | have | |
| | Input validation (name, VLAN, MTU) | have | |
| **Platform** | Linux (pure netlink) | have | |
| | macOS / Darwin | missing | lower |
| | FreeBSD / OpenBSD | missing | lower |
| **Quality** | Context-wrapped errors | have | |
| | Panic recovery | have | |
| | 14 test files (unit + integration) | have | |

<!-- source: internal/component/iface/manage_linux.go — CreateDummy, CreateVeth, CreateBridge, CreateVLAN, DeleteInterface, AddAddress, RemoveAddress, SetMTU -->
<!-- source: internal/component/iface/monitor_linux.go — Monitor, netlink multicast subscription -->
<!-- source: internal/component/iface/dhcp_v4_linux.go — DHCPv4 worker -->
<!-- source: internal/component/iface/dhcp_v6_linux.go — DHCPv6 worker -->
<!-- source: internal/component/iface/migrate_linux.go — MigrateInterface 4-phase protocol -->
<!-- source: internal/component/iface/mirror_linux.go — traffic mirroring via tc -->
<!-- source: internal/component/iface/sysctl_linux.go — per-interface sysctl writes -->
<!-- source: internal/component/iface/slaac_linux.go — SLAAC control -->
<!-- source: internal/component/iface/manage_other.go — non-Linux stubs -->
<!-- source: internal/component/iface/iface.go — bus topics, payload types -->
<!-- source: internal/component/bgp/reactor/reactor_iface.go — BGP integration -->

## Architecture

```
Config (YANG)
  |
  v
Engine (register.go) -- OnConfigure() creates Monitor + applies config
  |
  v
Management (manage_linux.go) -- netlink: create, delete, address, MTU, sysctl
  |
  v
Monitor (monitor_linux.go) -- netlink multicast: link + address events
  |
  v
Bus topics -- interface/{created,deleted,up,down,addr/added,addr/removed,dhcp/*}
  |
  v
Subscribers -- BGP reactor (starts/stops listeners on address changes)
```

## Key Design Decisions

- **Pure netlink, no shell-outs.** All kernel interaction through `github.com/vishvananda/netlink`.
- **Event-driven.** Monitor publishes to bus; consumers subscribe and react. No polling.
- **DAD-aware.** IPv6 addresses with `IFA_F_TENTATIVE` flag are held until DAD completes.
- **Make-before-break.** Migration adds new address, waits for BGP readiness, then removes old.
  Prevents session loss during address moves.
- **Virtual interface state.** Dummy/bridge/veth report `OperUnknown` not `OperUp`;
  monitor checks `IFF_UP` flag as fallback.
- **Idempotent cleanup.** Delete and mirror removal succeed even if already gone.

## Bus Topics

| Topic | Trigger | Payload |
|---|---|---|
| `interface/created` | First RTM_NEWLINK for an index | name, type, index, mtu, managed |
| `interface/deleted` | RTM_DELLINK | name, type, index, mtu, managed |
| `interface/up` | OperUp or OperUnknown+IFF_UP | name, index |
| `interface/down` | Other oper states | name, index |
| `interface/addr/added` | RTM_NEWADDR (DAD complete) | name, unit, index, address, prefix-length, family, managed |
| `interface/addr/removed` | RTM_DELADDR | name, unit, index, address, prefix-length, family, managed |
| `interface/dhcp/lease-acquired` | DHCPv4 ACK | name, unit, address, prefix-length, router, dns, lease-time |
| `interface/dhcp/lease-renewed` | Renewal success | name, unit, address, prefix-length, router, dns, lease-time |
| `interface/dhcp/lease-expired` | Lease timeout | name, unit, address, prefix-length, router, dns, lease-time |

<!-- source: internal/component/iface/iface.go — Topic* constants, *Payload structs -->
