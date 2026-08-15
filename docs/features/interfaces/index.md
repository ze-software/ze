# Interface Management

Ze manages Linux network interfaces via pure netlink (no iproute2 shell-outs).
JunOS-style two-layer model: physical interfaces with named logical units.

<!-- source: internal/component/iface/yang/ze-iface-conf.yang — interface container -->
<!-- source: internal/component/iface/register.go — registration -->

## Capability Table

| Category | Feature | Status | Priority |
|---|---|---|---|
| **Interface Types** | Ethernet (physical) | have | |
| | Dummy (virtual) | have | |
| | Veth pairs | have | |
| | Bridge (basic) | have | |
| | VLAN 802.1Q | have | |
| | VLAN 802.1p QoS maps (ingress/egress PCP-priority) | have | |
| | Class-of-service named profiles (802.1p, interface-level inheritance) | have | |
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
| | XFRM (route-based IPsec, if_id) | have | |
| | VTI (legacy IPsec tunnel) | missing | lower |
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
| | Route priority per unit (`route-priority`) | have | |
| | Link-down route deprioritization (metric + 1024) | have | |
| | IPv6 default route from RA with configurable metric | have | |
| | Routes carry `proto 253` (`ze-iface`), so a teardown removes only its own | have | <!-- source: internal/plugins/iface/netlink/manage_linux.go -- AddRoute stamps rtm_protocol, RemoveRoute matches on it --> |
| | Learned default routes rank at metric 254, clear of a static route's metric | have | <!-- source: internal/component/iface/config.go -- defaultLearnedRouteMetric --> |
| | DNS from DHCP to `/tmp/resolv.conf` | have | |
| | Hostname in DHCPv4 (option 12) | have | |
| | Client-ID in DHCPv4 (option 61) | have | |
| | NTP servers from DHCP (option 42) | have | |
| | DHCPv6 proper Renew (not re-solicit) | missing | medium |
| | DHCP relay | missing | lower |
| | DHCP server | missing | lower |
| **Monitoring** | Netlink multicast (link + addr + neigh) | have | |
| | Virtual iface state detection | have | |
| | 11 bus topics, JSON payloads | have | |
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
| **Router Advertisement** | RA sender per unit (RFC 4861) | have | |
| | Prefix Information options for SLAAC | have | |
| | RDNSS resolver option (RFC 8106) | have | |
| | Router Solicitation answers with rate limit | have | |
| | Route Information option (RFC 4191) | missing | lower |
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
| **Gateway Redundancy** | VRRP / keepalived | have | |
| | Virtual MAC | have | |
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
| | VPP backend (ifacevpp, via GoVPP) | have | ResetCounters via sw_interface_clear_stats; ListKernelRoutes via ip_route_v2_dump; RouteLookup via IPRouteLookupV2 (VPP FIB is authoritative); ListNeighbors via ip_neighbor_dump; VXLAN/GRE/IPIP/LCP/stats-socket/mirror/STP pending third-party vendoring |
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
<!-- source: internal/plugins/iface/netlink/manage_linux.go — CreateDummy, CreateVeth, CreateBridge, etc. -->
<!-- source: internal/plugins/iface/netlink/tunnel_linux.go — CreateTunnel for 8 tunnel kinds via Gretun/Gretap/Iptun/Sittun/Ip6tnl -->
<!-- source: internal/plugins/iface/netlink/wireguard_linux.go — CreateWireguardDevice (netlink), ConfigureWireguardDevice and GetWireguardDevice (wgctrl) -->
<!-- source: internal/plugins/iface/netlink/monitor_linux.go — netlink multicast subscription -->
<!-- source: internal/plugins/iface/netlink/bridge_linux.go — bridge ports, STP via sysfs -->
<!-- source: internal/component/sysctl/backend_linux.go -- per-interface sysctl writes -->
<!-- source: internal/plugins/iface/netlink/mirror_linux.go — traffic mirroring via tc -->
<!-- source: internal/plugins/iface/vpp/ifacevpp.go — VPP Backend implementation (lazy GoVPP channel) -->
<!-- source: internal/plugins/iface/vpp/query.go — ListInterfaces/GetInterface/Get/SetMACAddress via SwInterfaceDump and SwInterfaceSetMacAddress -->
<!-- source: internal/plugins/iface/vpp/monitor.go — StartMonitor via WantInterfaceEvents + SubscribeNotification -->
<!-- source: internal/plugins/iface/vpp/naming.go — ze short name <-> VPP SwIfIndex bidirectional map -->
<!-- source: internal/plugins/iface/vpp/fib.go — RouteLookup via IPRouteLookupV2; ListKernelRoutes via ip_route_v2_dump over both v4 and v6 tables -->
<!-- source: internal/plugins/iface/dhcp/dhcp_v4_linux.go — DHCPv4 worker -->
<!-- source: internal/plugins/iface/dhcp/dhcp_v6_linux.go — DHCPv6 worker -->
<!-- source: internal/component/bgp/reactor/reactor_iface.go — BGP integration -->

## Architecture

```
Config (YANG: ze-iface-conf.yang, "backend" leaf selects backend)
  |
  v
iface component (register.go) -- OnConfigure() loads backend, starts monitor
  |
  v
Backend interface (backend.go) -- 34 methods: lifecycle, address, sysctl, mirror, monitor
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
hardware. For ethernet, veth, and bridge interfaces, the MAC address (`mac { address }`) is
optional and, when set, must be unique within each list. Omit it to keep the
hardware-assigned MAC, or set it to override the address and pin the named config entry to
a specific physical device.

<!-- source: internal/component/iface/yang/ze-iface-conf.yang -- unique on ethernet/veth/bridge lists -->

Each discovered interface also records an `os-name` hidden leaf that preserves the original
OS interface name. This field is auto-populated during discovery and remains available for
debugging and internal binding after the user renames the config entry.

<!-- source: internal/component/iface/yang/ze-iface-conf.yang -- os-name hidden leaf in interface-common grouping -->

Operator-facing interface operations resolve the logical name to its kernel
device through the shared resolver before touching the kernel, so a configured
name that differs from the OS device (via the `os-name` or `mac/match` selector)
is honored uniformly: the `iface` CLI ops (set MTU, add/remove address, admin
up/down, bridge, mirror, ...), the DHCP client socket binding, and the
routing/protocol consumers all act on the bound kernel device. The dispatch
layer performs this translation for the by-name backend ops, leaving
`GetInterface`/`ListInterfaces` raw because the resolver is built on them. A
checks gate (`make ze-iface-resolution-check`) keeps new consumers from
resolving the kernel directly instead of through the resolver.

<!-- source: internal/component/iface/dispatch.go -- resolveOS translation in the by-name dispatch ops -->
<!-- source: internal/component/iface/resolve.go -- Resolve / Addresses / Subscribe logical-name resolver -->
<!-- source: scripts/checks/iface_resolution.go -- no-direct-resolution guard -->

A MAC address validator (`ze:validate "mac-address"`) provides format checking (colon-separated
hex octets) and live OS autocomplete. The `CompleteFn` calls `DiscoverInterfaces` on each
tab press, returning MAC addresses from currently active OS interfaces.

<!-- source: internal/component/config/validators.go -- MACAddressValidator, CompleteFn -->
<!-- source: internal/component/config/validators_register.go -- mac-address registration -->

## Key Design Decisions

- **Descriptive names as keys, MAC as binding.** Interface names in config are user-chosen
  descriptive labels. The MAC address ties the config entry to physical hardware. This
  separates the logical identity (name) from the physical identity (MAC).
- **Pluggable backends.** The iface component defines a `Backend` interface (34 methods).
  OS-specific operations live in backend plugins (`ifacenetlink` for Linux). The YANG
  `backend` leaf selects the backend (default: `netlink`). DHCP is a separate plugin
  (`ifacedhcp`) that uses the backend for address operations.
- **Pure netlink, no shell-outs.** The netlink backend uses `github.com/vishvananda/netlink`.
- **Event-driven.** Monitor publishes to bus; consumers subscribe and react. No polling.
- **Per-family addresses.** Addresses live inside `ipv4 { address [...]; }` and
  `ipv6 { address [...]; }` containers within each unit, following the Junos/Nokia model.
- **DAD-aware.** IPv6 addresses with `IFA_F_TENTATIVE` flag are held until DAD completes.
- **Make-before-break.** Migration adds new address, waits for BGP readiness, then removes old.
  Prevents session loss during address moves.
- **Same-subnet renumbers keep working.** Because the new address is added first, a renumber
  inside one subnet (`10.0.0.1/24` -> `10.0.0.2/24`) briefly leaves the new address as a Linux
  IPv4 *secondary* of the old one, and Linux deletes every secondary of a subnet along with its
  primary. Before such a removal the netlink backend enables
  `net.ipv4.conf.<device>.promote_secondaries` so the kernel promotes a secondary instead of
  flushing the subnet, logging `enabled promote_secondaries` when it changes the knob. If the
  knob cannot be set the removal fails, naming the addresses that would have been destroyed,
  rather than silently emptying the interface. The knob is left enabled: restoring it would
  re-arm the same hazard on the next removal. IPv6 has no primary/secondary distinction and is
  untouched, as is the VPP backend, which deletes exactly the requested address.
<!-- source: internal/plugins/iface/netlink/addr_primary.go -- ensureDeleteIsolated, flushedByDelete -->
<!-- source: internal/plugins/iface/netlink/manage_linux.go -- RemoveAddress delegates to removeAddressGuarded -->
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
- **The mirror shares its qdisc, so it owns only its filters.** Mirroring attaches a
  tc mirred filter at priority 1 to the qdisc at handle `ffff:` on the source
  interface. That qdisc is a shared attachment point: flow-export sampling attaches
  its own filter at priority 100 to the same object, and both hooks, ingress and
  egress, hang off it. So mirror setup accepts a qdisc another subsystem created
  (`EEXIST` is not an error), and mirror teardown deletes its own filters and leaves
  the qdisc standing, empty. A teardown cannot know who created a shared qdisc, and
  the set of filters on it can change between the moment it is read and the moment
  it is acted on. An empty qdisc carries no filter and classifies no packet, and the
  next mirror or sampling setup adopts it. Only the rollback of a failed setup
  deletes a qdisc, because it created that qdisc moments earlier and knows so. Ze always creates clsact, never the older ingress qdisc, because clsact
  carries both hooks and can therefore serve a mirror with a different destination
  per direction. `tc qdisc show` on an interface that once mirrored reports a clsact
  qdisc with no filter on it, which is expected.
<!-- source: internal/plugins/iface/netlink/mirror_linux.go -- RemoveMirror, removeMirrorFilters, undoMirrorSetup, ensureClsactQdisc -->
- **A mirror the config drops is torn down.** `applyConfig` compares the mirrors the
  new config asks for against the mirrors the previous config installed, and removes
  every one that was dropped or changed before it installs the new set. A changed
  destination is a remove followed by an install, because tc filters are additive:
  installing the new destination would otherwise leave the old one duplicating
  traffic. A daemon restart starts from no previous config, so a mirror deleted from
  the config file while ze was down is not reconciled away.
<!-- source: internal/component/iface/config_mirror.go -- indexMirrorSpecs, removeStaleMirrors, applyMirror -->

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
        unit default {
            ipv4 {
                address [ 10.0.0.1/30 ]
            }
        }
    }

    tunnel sixin4 {
        encapsulation {
            sit {
                local  { ip 192.0.2.1; }
                remote { ip 198.51.100.1; }
            }
        }
        unit default {
            ipv6 {
                address [ 2001:db8::1/64 ]
            }
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

L2 tunnel kinds (`gretap`, `ip6gretap`) support an optional `mac` container (with an
`address` leaf) inside the case container. L3 kinds do not carry a MAC address (the
kernel does not assign one).

ERSPAN, GRE keepalives, VRF underlay/overlay leaves, and `ignore-df` on gretap are
out of scope for v1; see `plan/deferrals/`.

<!-- source: internal/component/iface/yang/ze-iface-conf.yang -- list tunnel, choice kind, tunnel-v4-endpoints / tunnel-v6-endpoints groupings -->
<!-- source: internal/component/iface/tunnel.go -- TunnelKind enum, TunnelSpec struct -->
<!-- source: internal/plugins/iface/netlink/tunnel_linux.go -- CreateTunnel switch and per-kind builders -->

## Tunnel Reload Behaviour

On config reload (SIGHUP or transaction commit), `applyConfig` compares each
tunnel's spec against the previously applied config, which `indexTunnelSpecs`
indexes by name. Tunnels
whose spec is unchanged are left alone; MTU, MAC, and addresses still reconcile
through later phases, so non-spec changes still take effect. Tunnels whose spec
changed (encapsulation kind, `local`, `remote`, `key`, `ttl`, `hoplimit`, and
the rest of the per-case leaves) are deleted and recreated, because Linux does
not support in-place modification of most tunnel kinds. The recreate briefly
drops traffic on the changed tunnel only; unrelated tunnels are not disturbed.

A third case applies when the previously applied config is not available. The
comparison above reads the config this process applied before, and a netdev
outlives the process that made it. A tunnel therefore has no previous spec at
every plugin start, at every daemon start, and in the second apply of a reload
that starts the interface plugin, so the apply tries to create a link that is
already there. What holds the name decides the result:

| What holds the name | Result |
|---------------------|--------|
| Nothing | The create runs. A failure here is the kernel's own answer, and it fails the apply |
| A tunnel of the configured kind | Ze keeps it and logs a WARN. The link is not deleted and not rebuilt, so traffic crossing it is not interrupted. Its encapsulation parameters are not compared: the read-back carries the link type and no encapsulation field |
| Any other device: a dummy, a bridge, a physical NIC, or a tunnel of a different kind | The apply fails. Ze does not delete a device it did not create, and it does not push this tunnel's MTU, admin state and addresses onto a device that is not the tunnel |

<!-- source: internal/component/iface/config_apply.go — applyConfig, the tunnel create step -->
<!-- source: test/plugin/iface-tunnel-restart.ci — ze runs twice over one tunnel config; the kernel ifindex is compared across the restart -->

The last row is also what an operator meets after editing `encapsulation` while
the daemon is down. There is no previous spec to compare against, so the edit
does not reach the delete-and-recreate path. Ze refuses the config instead of
running the old encapsulation under the new addresses. Delete the tunnel, or
give the new encapsulation a new interface name.

<!-- source: internal/component/iface/config_apply.go -- applyConfig, indexTunnelSpecs -->
<!-- source: internal/component/iface/tunnel.go -- kernelLinkTypes, the link type each kind reports -->

## Tunnel Validation Scope

`ze config validate`, API pre-save validation, and CLI commit validation run
YANG schema checks plus registered side-effect-free in-process plugin verifiers.
They do not call live external plugin `OnConfigVerify` callbacks because those
callbacks are runtime transaction participants. Live external plugin verification
runs when the daemon loads, reloads, or commits config; failed API commits roll
the saved file back to the previous content before returning the reload error.

Interface validation that has an in-process verifier, such as tunnel case
consistency and backend feature gates, is visible in static validation. Any
third-party external plugin that only implements a live `OnConfigVerify`
callback is verified at daemon transaction time, not by `ze config validate`.

<!-- source: internal/component/config/cli/cmd_validate.go -- runValidation generic in-process plugin verifier loop -->
<!-- source: internal/component/config/plugin_verify.go -- VerifyPluginConfig uses InProcessConfigVerifier only -->
<!-- source: internal/component/iface/config.go -- parseTunnelEntry used by iface in-process verifier and OnConfigVerify -->
<!-- source: internal/component/api/config_session.go -- failed reload restores previous content -->

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
        unit default {
            ipv4 {
                address [ 10.0.0.1/24 ]
            }
        }
    }
}
```

<!-- source: internal/component/iface/yang/ze-iface-conf.yang -- list wireguard, ze:sensitive on private-key and peer preshared-key, ze:listener on the list entry -->
<!-- source: internal/component/iface/wireguard.go -- WireguardSpec / WireguardPeerSpec types -->
<!-- source: internal/component/iface/config.go -- parseWireguardEntry, parseWireguardPeer -->
<!-- source: internal/component/iface/config_apply.go -- applyConfig, indexWireguardSpecs, wireguardSpecEqual -->

### Key material and `$9$` encoding

`private-key` and peer `preshared-key` are marked `ze:sensitive` in YANG.
The config parser auto-decodes `$9$`-prefixed values on load
(`internal/component/config/parser.go`); `ze config show` / `ze config
dump` always re-encodes them on output (`internal/component/config/cli/cmd_dump.go`),
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

On reload, `applyConfig` compares the new spec to the previously
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

<!-- source: internal/component/iface/config_apply.go -- wireguardSpecEqual, wireguardPeerEqual -->
<!-- source: internal/component/iface/backend.go -- CreateWireguardDevice, ConfigureWireguardDevice, GetWireguardDevice interface methods -->

### Port conflict detection

`listen-port` participates in the same conflict-detection mechanism as
TCP services (web, ssh, mcp, etc.) with one Phase-5 extension:
`ListenerEndpoint` gained a `Protocol` field so wireguard's UDP ports
never clash with a TCP service on the same port. Two wireguards with
the same `listen-port` are rejected at reload time.

<!-- source: internal/component/config/listener.go -- ListenerEndpoint.Protocol, buildListenerService, CollectListeners, FindListenerConflict -->

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

## IPv6 Router Advertisements

Ze sends IPv6 Router Advertisements on a LAN unit, which is the job radvd does on
other systems. Hosts on the link build addresses by stateless address
autoconfiguration (SLAAC), learn a default router, and learn DNS resolvers. The
`iface-ra` plugin owns the socket, the timers, and the answers to Router
Solicitations (RFC 4861).

The `router-advertisement` container sits inside the per-unit `ipv6` container.
It carries `ze:os "linux"` and `ze:backend "netlink"`. The schema walker prunes
it on another platform, and a tree with `backend vpp` rejects it at config verify
with `feature not supported by backend "vpp"`.

<!-- source: internal/component/iface/yang/ze-iface-conf.yang -- container router-advertisement -->
<!-- source: internal/component/config/backend_gate.go -- ze:backend commit-time feature gate -->
<!-- source: internal/plugins/iface/ra/register.go -- iface-ra registration -->

### Send and accept are separate

An advertising interface tells hosts to send it their off-link traffic. Set
`ipv6 forwarding true` on that unit, or the kernel drops that traffic and every
host on the link loses connectivity. Leave `accept-ra` at 0 unless the same
interface also learns from another router.

Config verify cannot catch this, because forwarding can arrive from a sysctl
profile after verify runs. Ze reports the state instead. The
`doctor-iface-ra-forwarding` check emits one warning for each advertising
interface whose `net.ipv6.conf.<device>.forwarding` is 0.

<!-- source: internal/plugins/iface/ra/doctor.go -- checkRAForwarding, doctor-iface-ra-forwarding -->

### Container leaves

| Leaf | Type | Range | Unit | Default | Meaning |
|------|------|-------|------|---------|---------|
| `enabled` | boolean | | | `false` | Send advertisements on this unit. RFC 4861 Section 6.2.1 requires the default `false`, so a node never becomes a router by accident |
| `maximum-interval` | uint16 | 4..1800 | seconds | 600 | Longest wait between unsolicited advertisements (MaxRtrAdvInterval) |
| `minimum-interval` | uint16 | 3..1350 | seconds | 200 | Shortest wait between unsolicited advertisements (MinRtrAdvInterval) |
| `router-lifetime` | uint16 | 0..9000 | seconds | 1800 | How long hosts keep Ze in their default router list (AdvDefaultLifetime) |
| `hop-limit` | uint8 | 0..255 | | 64 | Value hosts put in the Hop Limit field of their outgoing packets. 0 states no value |
| `managed` | boolean | | | `false` | The M flag: hosts get their addresses from DHCPv6 |
| `other-config` | boolean | | | `false` | The O flag: hosts get other configuration, such as DNS, from DHCPv6 |
| `reachable-time` | uint32 | 0..3600000 | milliseconds | 0 | How long a host treats a neighbor as reachable after a confirmation. 0 states no value |
| `retransmit-timer` | uint32 | | milliseconds | 0 | Time between retransmitted Neighbor Solicitations on this link. 0 states no value |

Ze picks each unsolicited interval at random between `minimum-interval` and
`maximum-interval`, which keeps two routers on one link from synchronizing
(RFC 4861 Section 6.2.4). The first three advertisements after a start wait
16 seconds at most, so a new router is found quickly.

<!-- source: internal/plugins/iface/ra/ifacera.go -- unsolicitedInterval, maxInitialAdvertisements -->

### Prefix list

Each `prefix` entry becomes one Prefix Information option (RFC 4861
Section 4.6.2). The list key is the prefix in CIDR notation. SLAAC needs a
64-bit prefix (RFC 4862 Section 5.5.3).

| Leaf | Type | Unit | Default | Meaning |
|------|------|------|---------|---------|
| `on-link` | boolean | | `true` | The L flag: hosts treat addresses in this prefix as on-link |
| `autonomous` | boolean | | `true` | The A flag: hosts build addresses from this prefix by SLAAC |
| `valid-lifetime` | uint32 | seconds | 2592000 | How long the prefix stays valid. 30 days. 4294967295 never expires |
| `preferred-lifetime` | uint32 | seconds | 604800 | How long addresses from the prefix stay preferred. 7 days |

<!-- source: internal/component/iface/config_ra.go -- raParsePrefixEntry, raDefaultValidLifetime, raDefaultPreferredLifetime -->

### Resolvers (RDNSS)

The `rdnss` container points a link at a resolver without a DHCPv6 server
(RFC 8106 Section 5.1). `server` is a leaf-list of up to 8 IPv6 addresses, and
all of them share one `lifetime`.

`lifetime` has no default, and the two zero-like cases stay apart because of
that.

| You write | Ze advertises | Hosts do |
|-----------|---------------|----------|
| No `lifetime` leaf | 3 x `maximum-interval` | Use the resolvers, and refresh them on each advertisement |
| `lifetime 0` | 0 | Stop using these resolvers |
| `lifetime 4294967295` | 4294967295 | Use the resolvers, and never expire them |

<!-- source: internal/component/iface/config_ra.go -- raUnitConfig.RDNSSLifetime, EffectiveRDNSSLifetime -->

### Validation

The YANG ranges bound each leaf on its own. Config verify applies the cross-leaf
rules of RFC 4861 that no single range can express, and every one of them
rejects the commit.

| Rule | Rejected input | Message |
|------|----------------|---------|
| `minimum-interval` is at most 0.75 x `maximum-interval` | `minimum-interval 500` with `maximum-interval 600` | `above three quarters of maximum-interval` |
| `router-lifetime` is 0, or at least `maximum-interval` | `router-lifetime 300` with `maximum-interval 600` | `below maximum-interval, and not 0` |
| `preferred-lifetime` is at most `valid-lifetime` | `valid-lifetime 3600` alone | `preferred-lifetime 604800 is above valid-lifetime 3600` |
| The prefix carries no host bits | `prefix 2001:db8:1::1/64` | `host bits set below the prefix length, write 2001:db8:1::/64` |
| The prefix is not link-local | `prefix fe80::/64` | `the link-local prefix is not advertised` |

Two of these rows are traps worth reading twice.

`router-lifetime 0` is valid input, and it means that Ze advertises its prefixes
and its resolvers while it is not a default router (RFC 4861 Section 4.2). The
"at least `maximum-interval`" rule applies to every other value.

`preferred-lifetime` defaults to 604800, so lowering only `valid-lifetime` below
that number is rejected. Set both leaves together.

Ze rejects a prefix with host bits rather than masking them, because a masked
prefix advertises something the operator did not write.

<!-- source: internal/component/iface/config_ra.go -- raValidate, raParsePrefixEntry -->

### Example

A LAN unit that advertises one prefix, two resolvers, and itself as the default
router:

```
interface {
    backend netlink;
    ethernet eth0 {
        unit 0 {
            ipv6 {
                address [ 2001:db8:1::1/64 ];
                forwarding true;
                router-advertisement {
                    enabled true;
                    maximum-interval 900;
                    minimum-interval 300;
                    router-lifetime 1800;
                    hop-limit 64;
                    prefix 2001:db8:1::/64 {
                        on-link true;
                        autonomous true;
                        valid-lifetime 86400;
                        preferred-lifetime 43200;
                    }
                    rdnss {
                        server [ 2001:db8:1::53 2001:db8:1::54 ];
                        lifetime 3600;
                    }
                }
            }
        }
    }
}
```

<!-- source: test/parse/iface-router-advertisement.ci -- accepted router-advertisement unit -->

### Send loop

One goroutine owns the socket and every timer of one sender. It joins `ff02::2`
to receive Router Solicitations, and it sends to `ff02::1`. Every advertisement
leaves with Hop Limit 255, which RFC 4861 Section 6.1.2 makes a receiver check.

A solicitation draws an answer after a random wait of 500 milliseconds at most.
Consecutive multicast advertisements stay 3 seconds apart, so a flood of
solicitations cannot become a flood of advertisements (RFC 4861 Section 6.2.6).
A sender that stops sends up to three advertisements with a Router Lifetime of
0. Each host then drops Ze from its default router list at once.

Nothing leaves a link that is down. The next link-up event restarts the initial
burst.

<!-- source: internal/plugins/iface/ra/sender_linux.go -- openRASocket, run, sendFinal -->
<!-- source: internal/plugins/iface/ra/ifacera.go -- solicitedDelay, solicitedSendTime, minDelayBetweenRAs -->

### Counters

| Metric | Type | Labels | Counts |
|--------|------|--------|--------|
| `ze_iface_ra_sent_total` | CounterVec | `interface` | Every advertisement put on the wire, unsolicited and solicited together |
| `ze_iface_ra_solicited_total` | CounterVec | `interface` | The advertisements that answered a Router Solicitation |

A solicited advertisement increments both counters, so `ze_iface_ra_sent_total`
stays the total.

<!-- source: internal/plugins/iface/ra/ifacera.go -- SetMetricsRegistry, incSent, incSolicited -->

## Plugin-Owned Devices (Macvlan)

Beyond the plugin-owned **address** registry (a plugin declares addresses ze
should keep on an interface), iface offers a plugin-owned **device** registry
for bridge-mode macvlan devices. A same-process plugin asks iface to maintain a
macvlan carrying a caller-chosen unicast MAC on a parent interface; iface
creates it, keeps it reconciled, and deletes it on release or after a crash. The
one consumer today is VRRP (one macvlan per group carrying the RFC virtual MAC),
but the mechanism is generic and carries zero VRRP knowledge.

The API is a small value struct plus three calls (mirroring the address
registry):

- `MacvlanSpec{Name, Parent, MAC}` -- `Parent` is the parent's OS device name;
  mode is always bridge (no field).
- `ComposeOwnedDeviceName(prefix, parentIfindex, id)` -- builds a deterministic,
  collision-free name `<prefix>-<parentIfindex>-<id>` that fits the 15-char
  IFNAMSIZ budget, rejecting (never truncating) an over-budget candidate.
- `RegisterOwnedMacvlan(owner, spec)`, `UnregisterOwnedMacvlan(owner, name)`,
  `UnregisterOwnedMacvlans(owner)`.

**Ownership marker.** At create, the device's kernel link alias (IFLA_IFALIAS)
is set to `ze:owned:<owner>`, atomically with the MAC in one RTM_NEWLINK. The
reconcile pass reads ownership back from the kernel rather than remembering
history, so orphan cleanup works across restarts and crashes -- the one
structural difference from the address registry (which must track its own stale
interfaces because an address carries no owner).

**Reconcile ordering.** The device pass runs inside the same reconcile as
addresses, BEFORE the address loops: registered devices are created (or
re-asserted on MAC/parent/MTU drift by delete + recreate) first, so a VIP
registered on the device name via the address registry lands on an existing
device in the same pass. Then the orphan scan deletes any macvlan carrying the
`ze:owned:` alias that has no registration; deletion requires BOTH kind macvlan
AND the alias, so an operator's own macvlan is never touched. Macvlans are not
in the Phase 4 prune set (`zeManageable`) -- they are alias-guarded, not
YANG-managed. A registered device whose parent is absent is retried when the
parent appears (monitor `interface/created` / `interface/up` re-triggers the
pass). Non-netlink backends (VPP, non-Linux) reject `CreateMacvlanDevice`
fail-closed (exact-or-reject).

A `ze doctor` check (`doctor-iface-macvlan`) probes kernel macvlan capability
(create + delete a throwaway device pair), and the gauge
`ze_iface_owned_devices{owner}` tracks the registry.

<!-- source: internal/component/iface/macvlan.go -- MacvlanSpec, ComposeOwnedDeviceName -->
<!-- source: internal/component/iface/device_owner.go -- RegisterOwnedMacvlan, alias marker, gauge -->
<!-- source: internal/component/iface/config_apply.go -- reconcileOwnedDevices (device pass) -->
<!-- source: internal/plugins/iface/netlink/macvlan_linux.go -- CreateMacvlanDevice -->
<!-- source: internal/plugins/iface/netlink/doctor.go -- doctor-iface-macvlan capability check -->

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

## Compound Commands (Auto-Ensure Parent)

When a command creates a sub-resource (VLAN unit, address) on an interface
that may not exist yet, the compound form auto-creates the parent:

| Command | Behavior |
|---------|----------|
| `create interface dummy name <name> unit <vid>` | Creates dummy `<name>` if missing, then creates VLAN `<name>.<vid>` |
| `create interface dummy name <name> address <prefix>` | Creates dummy `<name>` if missing, then adds address |
| `create interface bridge name <name> unit <vid>` | Creates bridge `<name>` if missing, then creates VLAN |
| `create interface bridge name <name> address <prefix>` | Creates bridge `<name>` if missing, then adds address |
| `create interface <name> unit <vid>` | Direct form: parent must already exist |
| `create interface <name> address <prefix>` | Direct form: interface must already exist |

The type keyword (`dummy`, `bridge`) is required when the parent does not
exist, because the system needs to know what kind of interface to create.
If the parent already exists with a different type, the command fails
(e.g., `create interface dummy name br0 unit 100` rejects if `br0` is a bridge).

Rollback: if the sub-resource creation fails after the parent was
auto-created, the parent is deleted. Pre-existing parents are never
deleted on failure.

The mechanism is driven by the `ze:ensure-exists` YANG extension on the
type containers (`dummy`, `bridge`). The dispatch system builds an ensure
chain at registration time and wraps the leaf handler automatically.

Telling those two rollback cases apart requires the creation handler to
report whether it created the resource or found it already present. That
report is mandatory: if a handler on an ensure-exists path returns no
`created` flag, the command is refused with `ensure-exists contract
violation` naming the handler, and the sub-resource step never runs.
Ze fails closed here rather than guessing, because each guess corrupts a
different case: assuming "created" would delete an interface the operator
already owned, and assuming "not created" would leave a half-built
interface behind.

`veth` has no compound form. Creating a veth creates a *pair*, so it needs
a peer name, and the ensure chain invokes parent handlers with no arguments
(it only knows the parent's name selector). There is nowhere in
`create interface veth name <name> unit <vid>` to put the peer, so veth is
deliberately absent from the table above; create the pair first, then use
the direct form on either end.

<!-- source: internal/component/iface/yang/ze-iface-cmd.yang — ze:ensure-exists on dummy, bridge; veth has none -->
<!-- source: internal/component/plugin/server/ensure.go — wrapWithEnsureChain, buildEnsureChain, wasCreated, ErrEnsureContract -->
<!-- source: internal/component/iface/cmd/manage.go — idempotent handleCreateTyped; handleCreateVeth requires a peer arg -->

## Backend Implementations

The `Backend` interface declares 34 methods. Three implementations ship in
tree; coverage varies per platform and dataplane. The table below lists
each method against the backend and whether it is wired or returns an
error. `err` means the method is implemented as a stub that rejects every
call with a descriptive error. `real` means the method drives the
underlying mechanism. Cells with a footnote carry a caveat.

| Category | Method | netlink (Linux) | VPP | stub (non-Linux) |
|---|---|---|---|---|
| **Lifecycle** | `CreateDummy` | real | real (CreateLoopback) | err |
| | `CreateVeth` | real | err (VPP uses memif/TAP) | err |
| | `CreateBridge` | real | real (BridgeDomainAddDelV2) | err |
| | `CreateVLAN` | real | real (CreateVlanSubif) | err |
| | `CreateTunnel` | real [1] | err (pending GoVPP tunnel API) | err |
| | `CreateWireguardDevice` | real (rtnetlink) | err (requires VPP wg plugin) | err |
| | `ConfigureWireguardDevice` | real (wgctrl) | err (requires VPP wg plugin) | err |
| | `GetWireguardDevice` | real (wgctrl) | err (requires VPP wg plugin) | err |
| | `CreateXFRM` | real (rtnetlink) | err (XFRM is Linux netlink only) | err |
| | `GetXFRMInfo` | real (rtnetlink+xfrm) | err (XFRM is Linux netlink only) | err |
| | `DeleteInterface` | real | real (DeleteLoopback/DeleteSubif) | err |
| **Address** | `AddAddress` | real | real (SwInterfaceAddDelAddress) | err |
| | `RemoveAddress` | real | real (SwInterfaceAddDelAddress) | err |
| | `ReplaceAddressWithLifetime` | real | partial [2] | err |
| | `AddAddressP2P` | real | err (PPP NCP not supported yet) | err |
| **Route** | `AddRoute` | real | err (use fib-vpp plugin) | err |
| | `RemoveRoute` | real | err (use fib-vpp plugin) | err |
| | `ListRoutes` | real | err (use fib-vpp plugin) | err |
| | `RouteLookup` | real (netlink RouteGet) | real (IPRouteLookupV2 LPM) | err |
| | `ListKernelRoutes` | real | err [3] | err |
| **Link state** | `SetAdminUp` | real | real (SwInterfaceSetFlags) | err |
| | `SetAdminDown` | real | real (SwInterfaceSetFlags) | err |
| **Properties** | `SetMTU` | real | real (SwInterfaceSetMtu) | err |
| | `SetMACAddress` | real | real (SwInterfaceSetMacAddress) | err |
| | `GetMACAddress` | real | real (via SwInterfaceDump) | err |
| | `GetStats` | real | err (pending GoVPP stats API) | err |
| | `ResetCounters` | real [4] | err (pending sw_interface_clear_stats) | err |
| **Query** | `ListInterfaces` | real | real (SwInterfaceDump) | err |
| | `GetInterface` | real | real (SwInterfaceDump) | err |
| | `ListNeighbors` | real | err (pending ip_neighbor_dump) | err |
| **Bridge** | `BridgeAddPort` | real | real (SwInterfaceSetL2Bridge) | err |
| | `BridgeDelPort` | real | real (SwInterfaceSetL2Bridge) | err |
| | `BridgeSetSTP` | real (sysfs) | err (VPP STP varies by version) | err |
| **Mirror** | `SetupMirror` | real (tc mirred) | err (pending SpanEnableDisableL2) | err |
| | `RemoveMirror` | real | err (pending SpanEnableDisableL2) | err |
| **Monitor** | `StartMonitor` | real (netlink multicast) | real (WantInterfaceEvents) | err |
| | `StopMonitor` | real | real | no-op |
| | `Close` | real | real | no-op |

[1] `CreateTunnel` rejects an unknown kind with `unsupported tunnel kind
<k>`. Valid kinds are gre, gretap, ip6gre, ip6gretap, ipip, sit, ip6tnl,
ipip6.

[2] VPP has no kernel-style address lifetimes. The VPP backend ignores
the `validLft`/`preferredLft` arguments and installs the address without
an expiry, matching the exact-or-reject rule only when the operator does
not actually need DHCP lease-aware behaviour on VPP. DHCP runs against
the netlink backend today.

[3] On VPP the kernel FIB is not authoritative; the VPP backend rejects
`ListKernelRoutes` rather than return misleading data. A VPP FIB dump via
`ip_route_v2_dump` is the correct replacement and is not yet wired.

[4] Linux netlink has no generic counter-reset syscall. The netlink
backend returns `iface.ErrCountersNotResettable` and the dispatch layer
falls back to a per-interface baseline-delta model: the current values
become a baseline and `GetStats` / `ListInterfaces` / `GetInterface`
subtract the baseline before returning, so the operator sees
"since last clear" values.

### Netlink (Linux, default)

Pure netlink via `vishvananda/netlink`, with WireGuard peer/key
operations via `golang.zx2c4.com/wireguard/wgctrl`. Every method is
implemented; the only caveats are `CreateTunnel` rejecting unknown kinds
and `ResetCounters` using baseline-delta (both noted above). No
iproute2 shell-outs. Non-Linux builds of this package compile into the
stub backend described below.

<!-- source: internal/plugins/iface/netlink/manage_linux.go -- CreateDummy/CreateVeth/CreateBridge/CreateVLAN/Delete/AddAddress/RemoveAddress/ReplaceAddressWithLifetime/AddAddressP2P/SetAdminUp/SetAdminDown/SetMTU -->
<!-- source: internal/plugins/iface/netlink/tunnel_linux.go -- CreateTunnel switch over 8 kinds -->
<!-- source: internal/plugins/iface/netlink/wireguard_linux.go -- CreateWireguardDevice/ConfigureWireguardDevice/GetWireguardDevice -->
<!-- source: internal/plugins/iface/netlink/bridge_linux.go -- BridgeAddPort/BridgeDelPort/BridgeSetSTP -->
<!-- source: internal/plugins/iface/netlink/mirror_linux.go -- SetupMirror/RemoveMirror via tc -->
<!-- source: internal/plugins/iface/netlink/route_linux.go -- AddRoute/RemoveRoute/ListRoutes/ListKernelRoutes -->
<!-- source: internal/plugins/iface/netlink/neighbor_linux.go -- ListNeighbors, ResetCounters returning ErrCountersNotResettable -->
<!-- source: internal/plugins/iface/netlink/show_linux.go -- ListInterfaces/GetInterface/GetStats -->
<!-- source: internal/plugins/iface/netlink/monitor_linux.go -- StartMonitor/StopMonitor -->
<!-- source: internal/component/iface/counters.go -- ResetCounters baseline-delta fallback -->

### VPP (opt-in, via GoVPP)

Selected by the `backend vpp` leaf. 17 methods drive the VPP binary API
through GoVPP; 16 return `errNotSupported` with a string naming the
missing GoVPP call or the plugin gap. The channel to VPP is acquired
lazily on first method call; before the first successful acquire, every
method returns `iface.ErrBackendNotReady` so the reconciliation phase can
retry.

Use the VPP backend when VPP owns the dataplane. Use netlink elsewhere.
Routes, stats, counter reset, neighbour table, mirrors, and STP are the
main gaps against feature parity.

<!-- source: internal/plugins/iface/vpp/ifacevpp.go -- full Backend implementation, errNotSupported reasons per method -->
<!-- source: internal/plugins/iface/vpp/query.go -- ListInterfaces/GetInterface/GetMACAddress/SetMACAddress via SwInterfaceDump, SwInterfaceSetMacAddress -->
<!-- source: internal/plugins/iface/vpp/monitor.go -- StartMonitor/StopMonitor via WantInterfaceEvents + SubscribeNotification -->
<!-- source: internal/plugins/iface/vpp/naming.go -- ze short name <-> VPP SwIfIndex map -->

### Stub (non-Linux)

On darwin, macOS, BSD, and Windows the netlink backend package compiles
to a stub whose constructor succeeds but whose every method returns
`"interface management not supported on <GOOS>"`. `StopMonitor` and
`Close` are no-ops. The stub exists so the rest of the daemon can load
and the binary remains testable on developer machines; real interface
management requires Linux.

The stub never installs itself as the default silently. A macOS daemon
that actually tries `ze interface show` or any config-driven
reconciliation sees the explicit error and rejects under
exact-or-reject.

<!-- source: internal/plugins/iface/netlink/backend_other.go -- stubBackend, unsupported() returning "not supported on <GOOS>" -->
