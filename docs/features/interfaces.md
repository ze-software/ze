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
| | GRE / GRETAP / IPIP / SIT tunnels | have | |
| | IP6GRE / IP6GRETAP / IP6TNL / IPIP6 tunnels | have | |
| | ERSPAN | missing | lower |
| | WireGuard (declarative peers, `$9$`-encoded keys) | have | |
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
| **Lifecycle** | Create (dummy, veth, bridge, VLAN, tunnel) | have | |
| | Delete | have | |
| | Auto-up on create | have | |
| | Admin state control (explicit up/down) | have | |
| | Interface rename | missing | lower |
| **Address Management** | Add/remove IPv4/IPv6 CIDR | have | |
| | DAD-aware monitoring | have | |
| | MAC address set/get | have | |
| | Gratuitous ARP on add | missing | medium |
| | Neighbor table (ARP/NDP) | missing | lower |
| **DHCP** | DHCPv4 (RFC 2131, full DORA) | have | |
| | DHCPv6 (SARR, IA_NA, IA_PD) | have | |
| | Concurrent v4+v6 | have | |
| | Direct netlink install | have | |
| | Bus events (acquired/renewed/expired) | have | |
| | Config-driven (`dhcp { enabled true }`) | have | |
| | Default route from DHCP Router option | have | |
| | DNS from DHCP to `/tmp/resolv.conf` | have | |
| | Hostname in DHCPv4 (option 12) | have | |
| | Client-ID in DHCPv4 (option 61) | have | |
| | NTP servers from DHCP (option 42) | have | |
| | DHCPv6 proper Renew (not re-solicit) | missing | medium |
| | DHCP relay | missing | lower |
| | DHCP server | missing | lower |
| **Monitoring** | Netlink multicast (link + addr) | have | |
| | Virtual iface state detection | have | |
| | 9 bus topics, JSON payloads | have | |
| | Interface statistics/counters | have | |
| | Persistent counter tracking | missing | medium |
| **Per-Interface Tuning** | IPv4 forwarding | have | |
| | ARP filter / ARP accept | have | |
| | IPv6 autoconf (SLAAC) | have | |
| | IPv6 accept-ra (0/1/2) | have | |
| | IPv6 forwarding | have | |
| | Proxy ARP | have | |
| | ARP announce / ARP ignore | have | |
| | RPF / source validation | have | |
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
| **Migration** | Make-before-break 5-phase | have | |
| | BGP readiness signaling | have | |
| | Per-phase rollback | have | |
| **BGP Integration** | Reactor subscribes to addr events | have | |
| | Listener start/stop on addr change | have | |
| | bgp/listener/ready publish | have | |
| **Bridge Features** | Create and bring up | have | |
| | STP | have | |
| | VLAN filtering | missing | medium |
| | Add/remove member ports | have | |
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
| **Platform** | Pluggable backend interface | have | |
| | Linux netlink backend (default) | have | |
| | YANG `backend` leaf (config-driven selection) | have | |
| | macOS / Darwin | missing | lower |
| | FreeBSD / OpenBSD | missing | lower |
| | systemd-networkd | missing | lower |
| **Quality** | Context-wrapped errors | have | |
| | Panic recovery | have | |
| | 14 test files (unit + integration) | have | |

<!-- source: internal/component/iface/backend.go — Backend interface, RegisterBackend, LoadBackend -->
<!-- source: internal/component/iface/dispatch.go — package-level functions delegating to backend -->
<!-- source: internal/component/iface/iface.go — bus topics, payload types, InterfaceStats -->
<!-- source: internal/component/iface/migrate_linux.go — MigrateInterface 5-phase protocol -->
<!-- source: internal/plugins/ifacenetlink/manage_linux.go — CreateDummy, CreateVeth, CreateBridge, etc. -->
<!-- source: internal/plugins/ifacenetlink/tunnel_linux.go — CreateTunnel for 8 tunnel kinds via Gretun/Gretap/Iptun/Sittun/Ip6tnl -->
<!-- source: internal/plugins/ifacenetlink/wireguard_linux.go — CreateWireguardDevice (netlink), ConfigureWireguardDevice and GetWireguardDevice (wgctrl) -->
<!-- source: internal/plugins/ifacenetlink/monitor_linux.go — netlink multicast subscription -->
<!-- source: internal/plugins/ifacenetlink/bridge_linux.go — bridge ports, STP via sysfs -->
<!-- source: internal/plugins/ifacenetlink/sysctl_linux.go — per-interface sysctl writes -->
<!-- source: internal/plugins/ifacenetlink/mirror_linux.go — traffic mirroring via tc -->
<!-- source: internal/plugins/ifacedhcp/dhcp_v4_linux.go — DHCPv4 worker -->
<!-- source: internal/plugins/ifacedhcp/dhcp_v6_linux.go — DHCPv6 worker -->
<!-- source: internal/component/bgp/reactor/reactor_iface.go — BGP integration -->

## Architecture

```
Config (YANG: ze-iface-conf.yang, "backend" leaf selects backend)
  |
  v
iface component (register.go) -- OnConfigure() loads backend, starts monitor
  |
  v
Backend interface (backend.go) -- 33 methods: lifecycle, address, sysctl, mirror, monitor
  |
  v
+------------------+--------------------+
|                  |                    |
netlink backend    DHCP plugin          (future: networkd, FreeBSD)
(ifacenetlink/)    (ifacedhcp/)
|                  |
netlink calls      lease negotiation
|                  |
v                  v
Bus topics -- interface/{created,deleted,up,down,addr/*,dhcp/*}
  |
  v
Subscribers -- BGP reactor (starts/stops listeners on address changes)
```

## Interface Discovery

During `ze init`, Ze discovers OS network interfaces and generates initial configuration
entries. The `DiscoverInterfaces` function enumerates interfaces via `ListInterfaces`
(netlink on Linux, stdlib on other platforms) and classifies each by Ze type: ethernet,
bridge, veth, dummy, or loopback. On Linux, the netlink `device` type maps to ethernet
(except `lo`, which maps to loopback). Results are sorted by type then name.

<!-- source: internal/component/iface/discover.go -- DiscoverInterfaces, infoToZeType -->

The generated config uses descriptive names as YANG list keys (the OS interface name at
discovery time). The MAC address serves as the physical binding between configuration and
hardware. For ethernet, veth, and bridge interfaces, `mac-address` is required
(`ze:required`) and must be unique within each list. This means the user can freely rename
the config entry to a descriptive name while the MAC address maintains the link to the
physical device.

<!-- source: internal/component/iface/schema/ze-iface-conf.yang -- unique, ze:required on ethernet/veth/bridge lists -->

Each discovered interface also records an `os-name` hidden leaf that preserves the original
OS interface name. This field is auto-populated during discovery and remains available for
debugging and internal binding after the user renames the config entry.

<!-- source: internal/component/iface/schema/ze-iface-conf.yang -- os-name hidden leaf in interface-common grouping -->

A MAC address validator (`ze:validate "mac-address"`) provides format checking (colon-separated
hex octets) and live OS autocomplete. The `CompleteFn` calls `DiscoverInterfaces` on each
tab press, returning MAC addresses from currently active OS interfaces.

<!-- source: internal/component/config/validators.go -- MACAddressValidator, CompleteFn -->
<!-- source: internal/component/config/validators_register.go -- mac-address registration -->

## Key Design Decisions

- **Descriptive names as keys, MAC as binding.** Interface names in config are user-chosen
  descriptive labels. The MAC address ties the config entry to physical hardware. This
  separates the logical identity (name) from the physical identity (MAC).
- **Pluggable backends.** The iface component defines a `Backend` interface (33 methods).
  OS-specific operations live in backend plugins (`ifacenetlink` for Linux). The YANG
  `backend` leaf selects the backend (default: `netlink`). DHCP is a separate plugin
  (`ifacedhcp`) that uses the backend for address operations.
- **Pure netlink, no shell-outs.** The netlink backend uses `github.com/vishvananda/netlink`.
- **Event-driven.** Monitor publishes to bus; consumers subscribe and react. No polling.
- **DAD-aware.** IPv6 addresses with `IFA_F_TENTATIVE` flag are held until DAD completes.
- **Make-before-break.** Migration adds new address, waits for BGP readiness, then removes old.
  Prevents session loss during address moves.
- **Virtual interface state.** Dummy/bridge/veth report `OperUnknown` not `OperUp`;
  monitor checks `IFF_UP` flag as fallback.
- **Tunnel encapsulation as YANG choice/case.** The `tunnel` list at the iface level
  is one container with an `encapsulation` choice that branches into one case per
  Linux netlink kind (gre, gretap, ip6gre, ip6gretap, ipip, sit, ip6tnl, ipip6).
  Per-case leaf sets are constrained by the schema: `key` only appears in GRE-family
  cases, `hoplimit`/`tclass`/`encaplimit` only in v6-underlay cases. Local and
  remote endpoints use the same `local { ip ... } remote { ip ... }` shape as the
  BGP peer connection block, with `local { interface ... }` as an alternative when
  the source should be taken from another interface.
- **Idempotent cleanup.** Delete and mirror removal succeed even if already gone.

## Tunnel Configuration

```
interface {
    tunnel gre0 {
        encapsulation {
            gre {
                local  { ip 192.0.2.1; }
                remote { ip 198.51.100.1; }
                key 42
            }
        }
        unit 0 {
            address 10.0.0.1/30
        }
    }

    tunnel sixin4 {
        encapsulation {
            sit {
                local  { ip 192.0.2.1; }
                remote { ip 198.51.100.1; }
            }
        }
        unit 0 {
            address 2001:db8::1/64
        }
    }

    tunnel v6ov6 {
        encapsulation {
            ip6tnl {
                local  { ip 2001:db8::1; }
                remote { ip 2001:db8::2; }
                hoplimit 64
                encaplimit 4
            }
        }
    }
}
```

The eight supported encapsulation kinds map to Linux netlink kinds:

| Kind | Linux netlink | Underlay | Layer | Notes |
|------|--------------|----------|-------|-------|
| `gre` | gre | IPv4 | L3 | RFC 2784, RFC 2890 key |
| `gretap` | gretap | IPv4 | L2 (bridgeable) | Ethernet over GRE |
| `ip6gre` | ip6gre | IPv6 | L3 | hoplimit/tclass per RFC 2473 |
| `ip6gretap` | ip6gretap | IPv6 | L2 (bridgeable) | |
| `ipip` | ipip | IPv4 | L3 | RFC 2003, no key |
| `sit` | sit | IPv4 | L3 | 6in4 per RFC 4213 |
| `ip6tnl` | ip6tnl | IPv6 | L3 | RFC 2473 with Proto=IPV6 |
| `ipip6` | ip6tnl | IPv6 | L3 | RFC 2473 with Proto=IPIP |

`ipip6` shares the kernel `ip6tnl` netdev with a different inner protocol byte (4 vs 41).
Both surface as distinct YANG cases so the schema and config are unambiguous.

L2 tunnel kinds (`gretap`, `ip6gretap`) support an optional `mac-address` leaf inside
the case container. L3 kinds do not carry a MAC address (the kernel does not assign one).

ERSPAN, GRE keepalives, VRF underlay/overlay leaves, and `ignore-df` on gretap are
out of scope for v1; see `plan/deferrals.md`.

<!-- source: internal/component/iface/schema/ze-iface-conf.yang -- list tunnel, choice kind, tunnel-v4-endpoints / tunnel-v6-endpoints groupings -->
<!-- source: internal/component/iface/tunnel.go -- TunnelKind enum, TunnelSpec struct -->
<!-- source: internal/plugins/ifacenetlink/tunnel_linux.go -- CreateTunnel switch and per-kind builders -->

## Tunnel Reload Behaviour

On config reload (SIGHUP or transaction commit), `applyTunnels` compares each
tunnel's spec against the previously applied config, indexed by name. Tunnels
whose spec is unchanged are left alone; MTU, MAC, and addresses still reconcile
through later phases, so non-spec changes still take effect. Tunnels whose spec
changed (encapsulation kind, `local`, `remote`, `key`, `ttl`, `hoplimit`, and
the rest of the per-case leaves) are deleted and recreated, because Linux does
not support in-place modification of most tunnel kinds. The recreate briefly
drops traffic on the changed tunnel only; unrelated tunnels are not disturbed.

<!-- source: internal/component/iface/config.go -- applyTunnels, indexTunnelSpecs -->

## Tunnel Validation Scope

`ze config validate` runs YANG schema validation plus BGP- and hub-specific
checks. It does not invoke plugin `OnConfigVerify` callbacks, so iface
parser-level rejections (missing `encapsulation` case, both `local { ip }` and
`local { interface }` set in the same block, `key` on kinds that do not
support it) only fire when the daemon loads or reloads the config. A config
file that passes `ze config validate` can still be rejected by the running
daemon at reload time.

<!-- source: cmd/ze/config/cmd_validate.go -- runValidation (YANG + BGP + hub only, no plugin OnConfigVerify) -->
<!-- source: internal/component/iface/config.go -- parseTunnelEntry (iface tunnel validation runs at OnConfigVerify) -->

## WireGuard Configuration

WireGuard interfaces are a top-level `wireguard` list under `interface`,
alongside `ethernet`, `tunnel`, and the other iface kinds. Each entry carries
interface-level parameters plus a nested `peer` list; unit-level addresses
ride the same `interface-unit` grouping used everywhere else.

```
interface {
    wireguard wg0 {
        listen-port 51820
        fwmark 0
        private-key "$9$ABCabc..."        # $9$-encoded; see below
        peer site2 {
            public-key "YYYY..."           # base64, plaintext
            preshared-key "$9$DEF..."      # optional, also $9$-encoded
            endpoint {
                ip 198.51.100.2
                port 51820
            }
            allowed-ips [ 10.0.0.2/32 192.168.10.0/24 ]
            persistent-keepalive 25
        }
        unit 0 {
            address 10.0.0.1/24
        }
    }
}
```

<!-- source: internal/component/iface/schema/ze-iface-conf.yang -- list wireguard, ze:sensitive on private-key and peer preshared-key, ze:listener on the list entry -->
<!-- source: internal/component/iface/wireguard.go -- WireguardSpec / WireguardPeerSpec types -->
<!-- source: internal/component/iface/config.go -- parseWireguardEntry, applyWireguards, wireguardSpecEqual -->

### Key material and `$9$` encoding

`private-key` and peer `preshared-key` are marked `ze:sensitive` in YANG.
The config parser auto-decodes `$9$`-prefixed values on load
(`internal/component/config/parser.go:127`); `ze config show` / `ze config
dump` always re-encodes them on output (`cmd/ze/config/cmd_dump.go:132`),
so the plaintext base64 form never reaches the config file on disk. Public
keys are public and stored plaintext.

**`$9$` is JunOS-compatible obfuscation, not encryption.** Anyone with
read access to the config file (or the zefs blob, depending on storage
backend) can trivially recover the plaintext key via `secret.Decode`. The
protection is on the filesystem layer: `chmod 600 /etc/ze/ze.conf` (or the
equivalent on the `.zefs` blob). This is the same posture ze uses for BGP
MD5 passwords, SSH secrets, MCP tokens, and API tokens.

<!-- source: internal/component/config/secret/secret.go -- Encode, Decode, IsEncoded ($9$ implementation) -->

### Reconciliation

On reload, `applyWireguards` compares the new spec to the previously
applied spec via `wireguardSpecEqual`. Unchanged entries are a no-op; the
kernel is not touched and peer handshake state is preserved. Changed
entries trigger a single `ConfigureWireguardDevice` call with
`wgtypes.Config{ReplacePeers: true}` -- the kernel matches unchanged peers
by public-key and preserves their handshake state, so "apply entire spec
on every change" is functionally equivalent to a per-peer diff at a tiny
fraction of the code. New wireguard entries get a `CreateWireguardDevice`
(netlink) before the Configure call. Wireguard list entries removed from
config are deleted by Phase 4 reconciliation, same as tunnels.

Peer names in config (`peer site2 { ... }`) are operator-chosen labels.
The kernel tracks peers only by public key, so the label can change
freely without affecting the handshake. `ze init` emits discovered peers
with synthetic names (`peer0`, `peer1`, ...) which operators typically
rename via `ze config edit`.

<!-- source: internal/component/iface/config.go -- wireguardSpecEqual, wireguardPeerEqual -->
<!-- source: internal/component/iface/backend.go -- CreateWireguardDevice, ConfigureWireguardDevice, GetWireguardDevice interface methods -->

### Port conflict detection

`listen-port` participates in the same conflict-detection mechanism as
TCP services (web, ssh, mcp, etc.) with one Phase-5 extension:
`ListenerEndpoint` gained a `Protocol` field so wireguard's UDP ports
never clash with a TCP service on the same port. Two wireguards with
the same `listen-port` are rejected at reload time.

<!-- source: internal/component/config/listener.go -- ListenerEndpoint.Protocol, collectWireguardListeners, conflicts -->

### Dependencies

WireGuard peer and key configuration uses
[`golang.zx2c4.com/wireguard/wgctrl`](https://github.com/WireGuard/wgctrl-go),
the reference Go client maintained by the WireGuard authors (Donenfeld,
Layher). It is vendored under `vendor/golang.zx2c4.com/wireguard/wgctrl`
along with its transitive dependencies `github.com/mdlayher/genetlink`
and `github.com/mdlayher/netlink`. WireGuard has no RFC; reference
material is the original whitepaper
(https://www.wireguard.com/papers/wireguard.pdf), the Linux kernel
genetlink spec
(https://www.kernel.org/doc/html/latest/userspace-api/netlink/specs/wireguard.html),
and `wg(8)`.

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
