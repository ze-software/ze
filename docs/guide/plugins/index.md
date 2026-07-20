# Plugins

Ze uses a plugin architecture for all features beyond core BGP session management. Plugins handle RIB storage, route reflection, graceful restart, RPKI validation, NLRI encoding, and more.
<!-- source: internal/component/plugin/registry/registry.go -- plugin registry; internal/component/plugin/all/ -- blank imports -->

## Which Plugins Do I Need?

| Use case | Plugins | Why |
|----------|---------|-----|
| Announce routes to upstream | `bgp-rib` | Stores routes and sends them to peers |
| Route server (IXP) | `bgp-rib` + `bgp-rs` + `bgp-adj-rib-in` | Forward routes between clients, replay on reconnect |
| With RPKI validation | Add `bgp-rpki` + `bgp-adj-rib-in` | Validate origin AS against ROA cache |
| With merged RPKI events | Add `bgp-rpki-decorator` (+ above) | Receive UPDATE events pre-merged with RPKI state |
| With graceful restart | Add `bgp-gr` | Hold routes across restarts (RFC 4724) |
| Service healthcheck | `bgp-healthcheck` + `bgp-watchdog` | Monitor services, control route announcement via MED or withdraw. [Guide](https://github.com/ze-software/ze/blob/main/docs/guide/healthcheck.md) |
| Monitor only (no RIB) | None | Ze runs without plugins -- peers connect, events fire, no routes stored |
| Interface-aware BGP | `iface` + `bgp-rib` | React to OS interface changes -- start/stop BGP listeners when addresses appear/disappear |
| Static routes | (auto-loaded) | Config-driven static routes with ECMP, weighted load balancing, BFD failover. [Guide](../static-routes/index.md) |
| OSPFv2 edge plugin | (auto-loaded) | `ospf {}` starts the native OSPFv2 edge plugin, validates router-id/area/interface config, opens raw IPv4 sockets for active links, runs the Interface and Neighbor State Machines, and handles LSDB flooding |

NLRI family plugins (bgp-nlri-evpn, bgp-nlri-vpn, etc.) are loaded automatically when you configure the corresponding address family. You don't need to declare them.
<!-- source: internal/component/bgp/plugins/nlri/ -- NLRI plugin registrations with Families field -->

## Config-Driven Loading

BGP itself is a config-driven plugin. If your config has a `bgp { }` section, BGP loads automatically. If it doesn't, ze starts without BGP (useful for interface-only or FIB-only deployments). Native OSPFv2 follows the same pattern: an `ospf { }` section auto-loads the `ospf` edge plugin through `ConfigRoots ["ospf"]`.
<!-- source: internal/component/plugin/server/startup_autoload.go -- getConfigPathPlugins, autoLoadForNewConfigPaths -->
<!-- source: internal/plugins/ospf/register.go -- registerOSPF ConfigRoots -->
<!-- source: internal/plugins/ospf/neighbor/table.go -- Table, Hello, HandleDBDesc -->
<!-- source: internal/plugins/ospf/lsdb/lsdb.go -- LSDB -->

## Loading Plugins

Plugins are declared in the `plugin { }` block. Built-in plugins use `internal`, external processes use `external`:

```
plugin {
    internal rib {
        use bgp-rib
    }
    internal adj-rib-in {
        use bgp-adj-rib-in
    }
    internal gr {
        use bgp-gr
    }
}
```

For external processes (scripts, custom binaries):

```
plugin {
    external collector {
        run ./collector.py
        encoder json
    }
}
```

### Plugin Block Settings

| List | Setting | Description |
|------|---------|-------------|
| `internal` | `use` | Name of a built-in plugin to run in-process |
| `external` | `run` | Command to start an external plugin process |
| `external` | `encoder` | Wire encoding: `json` (default) or `text` |

## Binding Plugins to Peers

Each peer declares which plugins receive its events via `process` blocks. The process name must match the plugin's `internal` or `external` name in the `plugin { }` block:

```
plugin {
    internal rib { ... }       # <-- this name
}
peer transit-a {
    process rib { ... }        # <-- must match
}
```

Plugins receive BGP events through process bindings on each peer:

```
peer transit-a {
    ...
    process rib {
        receive [ state ]
        send [ update ]
    }
    process adj-rib-in {
        receive [ update state ]
    }
}
```

### Event Types

| Event | Description |
|-------|-------------|
| `update` | Route announcements and withdrawals |
| `open` | OPEN message |
| `notification` | NOTIFICATION message |
| `keepalive` | KEEPALIVE message |
| `refresh` | Route refresh request |
| `state` | Peer state changes (up/down) |
| `negotiated` | Capability negotiation results |
| `eor` | End-of-RIB marker |
| `rpki` | RPKI validation results |
| `update-rpki` | Merged UPDATE + RPKI validation (from bgp-rpki-decorator) |

Plugins can register custom event types via the `EventTypes` field in their registration.
These become valid in `receive` config directives and `subscribe-events` RPCs.
<!-- source: internal/component/plugin/registry/registry.go -- Registration.EventTypes -->

### Runtime subscriptions

The Go SDK can subscribe at startup with `SetStartupSubscriptions` or after
startup with `SubscribeEvents`. Each subscription carries its own namespace,
event list, peer selector, format, and envelope preference. An empty namespace
uses the protocol component's default namespace, normally `bgp`.

The event name `"*"` expands at registration time to every event type currently
registered in that namespace. This avoids a wildcard check on every delivered
event. Events registered later require a new subscription.

By default, `OnEvent` receives the original payload string. Call
`SetEnvelope(true)` before startup when one handler needs to distinguish events
from several namespaces or event types. Delivery then wraps the payload:

```json
{
  "namespace": "vpn-ipsec",
  "event": "sa-up",
  "payload": {
    "peer": "branch-a"
  }
}
```

The envelope sits inside the existing event string, so single-event and batch
delivery use the same callback. Subscribers that do not opt in remain
byte-compatible with earlier releases.

<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- SetStartupSubscriptions, SetEnvelope -->
<!-- source: pkg/plugin/rpc/types.go -- SubscribeEventsInput, EventEnvelope -->
<!-- source: internal/component/plugin/server/dispatch.go -- wildcard expansion and enveloped delivery -->

### Directions

```
process my-plugin {
    receive [ update ]        # events FROM the peer
    send [ update ]           # ability to send TO the peer
}
```

## Invocation Modes

| Mode | Config Syntax | Description |
|------|--------------|-------------|
| Internal | `internal rib { use bgp-rib }` | Compiled-in plugin using `net.Pipe` for startup and DirectBridge for hot paths |
| External | `external feed { run "/usr/local/bin/my-plugin" }` | External binary or script using TLS connect-back |

Internal mode (`use pluginname`) runs a compiled-in plugin as a goroutine within the ze process. Startup still uses the same YANG RPC handshake as external plugins, then DirectBridge bypasses socket I/O for supported hot paths. External mode starts a separate process; that process connects back to the plugin hub over TLS and authenticates with its per-plugin token.
<!-- source: internal/component/plugin/server/ -- plugin invocation modes; internal/component/plugin/cli/cli.go -- RunPlugin -->

## Built-In Plugins

List available plugins:

```
ze --plugins
```

### Storage and Policy

| Plugin | Purpose | Typical Binding |
|--------|---------|----------------|
| `bgp-rib` | Route Information Base | `receive [ state ] send [ update ]` |
| `bgp-adj-rib-in` | Adj-RIB-In (raw hex replay, auto-replays on peer-up) | `receive [ update state ]` |
| `bgp-persist` | Route persistence across restarts | `receive [ update state ] send [ update ]` |
| `bgp-rs` | Route server (forward-all) | `receive [ update ] send [ update ]` |
| `bgp-watchdog` | Deferred route announcement | `receive [ update ]` |
<!-- source: internal/component/bgp/plugins/rib/register.go -- bgp-rib registration -->
<!-- source: internal/component/bgp/plugins/adj_rib_in/register.go -- bgp-adj-rib-in registration -->
<!-- source: internal/component/bgp/plugins/persist/register.go -- bgp-persist registration -->
<!-- source: internal/component/bgp/plugins/rs/register.go -- bgp-rs registration -->
<!-- source: internal/component/bgp/plugins/watchdog/register.go -- bgp-watchdog registration -->

### Protocol

| Plugin | Purpose | Typical Binding |
|--------|---------|----------------|
| `bgp-gr` | Graceful Restart (RFC 4724) and Long-Lived GR (RFC 9494) | `receive [ state eor ]` |
| `bgp-rpki` | RPKI origin validation (RFC 6811) | `receive [ update ]` |
| `bgp-rpki-decorator` | Merged UPDATE+RPKI events | `receive [ update rpki ]` |
| `bgp-route-refresh` | Route Refresh (RFC 2918) | `receive [ refresh ]` |
| `bgp-role` | BGP Role (RFC 9234) | -- |
| `bgp-hostname` | FQDN capability | -- |
| `bgp-softver` | Software version capability | -- |
| `bgp-llnh` | Link-local next-hop (RFC 2545) | -- |
| `bgp-bmp` | BMP receiver + sender (RFC 7854) | `receive [ state update ]` |
<!-- source: internal/component/bgp/plugins/gr/register.go -- bgp-gr registration -->
<!-- source: internal/component/bgp/plugins/rpki/register.go -- bgp-rpki registration -->
<!-- source: internal/component/bgp/plugins/rpki_decorator/register.go -- bgp-rpki-decorator registration -->
<!-- source: internal/component/bgp/plugins/route_refresh/register.go -- bgp-route-refresh registration -->
<!-- source: internal/component/bgp/plugins/role/register.go -- bgp-role registration -->
<!-- source: internal/component/bgp/plugins/hostname/register.go -- bgp-hostname registration -->
<!-- source: internal/component/bgp/plugins/softver/register.go -- bgp-softver registration -->
<!-- source: internal/component/bgp/plugins/llnh/register.go -- bgp-llnh registration -->
<!-- source: internal/component/bgp/plugins/bmp/register.go -- bgp-bmp registration -->

### Infrastructure

| Plugin | Description | Process Binding |
|--------|-------------|-----------------|
| `iface` | OS interface orchestration: loads backend, dispatches operations | -- (Bus events, no peer binding) |
| `iface-netlink` | Netlink backend for iface: manage, monitor, bridge, sysctl, mirror | -- (registered as iface backend) |
| `iface-dhcp` | DHCP client: DHCPv4/DHCPv6 lease acquisition and renewal | -- (Bus events) |
| `rib` | System RIB: selects best route across protocols by admin distance | -- (Bus events, no peer binding) |
| `fib-kernel` | FIB kernel: programs OS routes from system RIB via netlink | -- (Bus events, no peer binding) |
| `fib-p4` | FIB P4: programs P4 switch from system RIB via gRPC/P4Runtime (noop backend) | -- (Bus events, no peer binding) |
| `fib-vpp` | FIB VPP: programs VPP FIB from system RIB via GoVPP. MPLS label push (IPRouteAddDel with LabelStack), swap/pop (MplsRouteAddDel), interface enable (SwInterfaceSetMplsEnable). | -- (Bus events, no peer binding) |
| `firewall-vpp` | VPP ACL backend for firewall: translates ze Match/Action types to VPP ACL rules, read-merge-write bindings preserving foreign ACLs | -- (registered as firewall backend) |
| `policy-routes` | Policy-based routing via nftables packet marking and kernel ip rules. Steers traffic to alternate routing tables or next-hops based on L3/L4 match criteria. [Guide](../policy-routing/index.md) | -- (Config-driven, depends on firewall) |
| `sysctl` | Kernel tunable management: three-layer precedence (config > transient > default), restore on stop. Named profiles (dsr, router, hardened, multihomed, proxy) for interface units. User-defined profiles. | -- (Bus events, CLI commands) |
| `dhcpserver` | DHCP server (RFC 2131/2132) with PXE boot support (RFC 4578): pool management, lease tracking, static mappings, PXE option injection (options 43/60/66/67/93) for BIOS/UEFI bootfile selection | -- (Config-driven, UDP listener) |
| `tftpserver` | Read-only TFTP server (RFC 1350): serves bootloader files for PXE provisioning in 512-byte blocks with stop-and-wait ACK, concurrent transfer limiting, path traversal protection | -- (Config-driven, UDP listener on port 69) |
| `imageserver` | HTTP image server for PXE provisioning: serves gokrazy disk images, installer boot files, and pre-provisioned zefs databases with SSH credentials. Own HTTP listener, path traversal protection, Range request support | -- (Config-driven, HTTP listener) |
| `geodns` | GeoDNS server (RFC 1035): DNS answers selected by client source IP. Client IP from EDNS0 client-subnet (RFC 7871) or packet source; CIDR longest-prefix selects a named host-set (A/AAAA/SRV records); synthesizes SOA/NS/glue. `show geodns`, `ze_geodns_*` metrics, UDP+TCP listeners (default 127.0.0.1:5300) | -- (Config-driven, UDP+TCP listeners) |
| `as112` | AS112 anycast DNS node (RFC 7534, RFC 7535): authoritative-only sink for misdirected RFC 1918 / link-local reverse-DNS queries plus the EMPTY.AS112.ARPA DNAME-redirection zone. Four fixed anycast host addresses (never operator-typed) registered via the iface address-ownership registry, `IP_FREEBIND` listeners, optional `allow-from` client-source access list (loopback always permitted). `show as112`, `as112 health` (one-shot query for the healthcheck probe), `ze_as112_*` metrics, UDP+TCP listeners on port 53. Now also a BGP redistribute source: `redistribute { destination bgp { import as112 } }` originates the four covering prefixes into BGP, with `asn` (origin AS, default 112), `community`, and a `watchdog` health-gate under `service { as112 }`. <!-- source: internal/plugins/as112/redistribute.go -- registerAS112Sources --> | -- (Config-driven, UDP+TCP listeners) |
| `traffic-usage` | eBPF TCX per-(port, protocol) and opt-in per-IP byte accounting (IPv4, monitoring only). Pure-Go assembled eBPF (cilium/ebpf asm.Instructions, no C/clang). Prometheus `ze_traffic_usage_*` metrics, `show traffic usage [name <interface>]`. Linux >= 6.6. [Guide](../traffic-usage/index.md) | -- (Config-driven, Bus events) |
<!-- source: internal/plugins/trafficusage/register.go -- traffic-usage registration -->
<!-- source: internal/plugins/policyroute/register.go -- policy-routes registration -->
<!-- source: internal/component/sysctl/register.go -- sysctl registration -->
<!-- source: internal/plugins/dhcpserver/register.go -- dhcpserver registration -->
<!-- source: internal/plugins/tftpserver/register.go -- tftpserver registration -->
<!-- source: internal/plugins/geodns/register.go -- geodns registration -->
<!-- source: internal/plugins/as112/register.go -- as112 registration -->
<!-- source: internal/component/iface/register.go -- iface registration -->
<!-- source: internal/plugins/iface/netlink/register.go -- iface-netlink backend registration -->
<!-- source: internal/plugins/iface/dhcp/register.go -- iface-dhcp registration -->
<!-- source: internal/component/sysrib/register.go -- rib plugin registration -->
<!-- source: internal/plugins/fib/p4/register.go -- fib-p4 registration -->
<!-- source: internal/plugins/fib/kernel/register.go -- fib-kernel registration -->
<!-- source: internal/plugins/firewall/vpp/register.go -- firewall-vpp backend registration -->

### L2TP

| Plugin | Description |
|--------|-------------|
| `l2tp-auth-local` | Static user/password authentication for L2TP PPP sessions (PAP/CHAP-MD5/MS-CHAPv2) |
| `l2tp-auth-radius` | RADIUS authentication (Access-Request), accounting (Start/Stop/Interim-Update), CoA/DM listener, and Access-Accept attribute extraction (Framed-IP-Address, Framed-Pool, Session-Timeout, Idle-Timeout, Filter-Id, Acct-Interim-Interval) |
| `l2tp-pool` | Bitmap-backed IPv4 address pool with default and named pools; Framed-IP-Address bypasses pool (direct RADIUS IP assignment), Framed-Pool selects a named pool |
| `l2tp-shaper` | TC traffic shaping (TBF/HTB) on pppN interfaces with configured default rates, RADIUS CoA rate updates, and initial rate from Filter-Id at session establishment |

These plugins register via the L2TP handler registry (`RegisterAuthHandler`,
`RegisterPoolHandler`) at init time. Only one auth handler is active at a
time (last registered wins). The pool and shaper plugins subscribe to
session lifecycle events via the EventBus.

See [L2TP guide](../l2tp/index.md) for configuration details.

<!-- source: internal/component/l2tp/plugins/authlocal/register.go -->
<!-- source: internal/component/l2tp/plugins/authradius/register.go -->
<!-- source: internal/component/l2tp/plugins/pool/register.go -->
<!-- source: internal/component/l2tp/plugins/shaper/register.go -->

The `iface` plugin defines a `Backend` interface and loads a backend by name (YANG
`backend` leaf, default `netlink`). The `iface-netlink` backend handles all Linux
interface operations. `iface-dhcp` is a separate plugin for DHCP client lifecycle.
BGP reacts to address events by starting/stopping listeners. Uses a JunOS-style
two-layer model: physical interfaces + logical units (VLANs).

Bus topics published:

| Topic | When |
|-------|------|
| `interface/created` | Interface appeared |
| `interface/deleted` | Interface removed |
| `interface/up` | Link state to up |
| `interface/down` | Link state to down |
| `interface/addr/added` | IP assigned |
| `interface/addr/removed` | IP removed |

<!-- source: internal/component/iface/iface.go -- topic constants and payload types -->

The `rib` plugin aggregates best routes from all protocol RIBs and selects
the system-wide best per prefix by administrative distance (lower wins).
Subscribes to `bgp-rib/best-change/` Bus topic prefix, publishes `system-rib/best-change`.
<!-- source: internal/component/sysrib/sysrib.go -- system-rib topic, protocolRoute, admin distance selection -->

The `fib-kernel` plugin programs OS routes from the system RIB into the kernel
via netlink (Linux). Uses a custom rtm_protocol ID (RTPROT_ZE=250) to identify
ze-installed routes. Crash recovery marks existing ze routes as stale at startup
and sweeps them after reconvergence. A kernel route monitor detects external
changes and re-asserts ze routes when overwritten.
<!-- source: internal/plugins/fib/kernel/fibkernel.go -- fibKernel, startupSweep, sweepStale -->
<!-- source: internal/plugins/fib/kernel/monitor_linux.go -- kernel route monitor -->

Bus topics in the FIB pipeline:

| Topic | Publisher | Subscriber | Payload |
|-------|-----------|------------|---------|
| `bgp-rib/best-change/bgp` | `bgp-rib` | `rib` | Batch of per-prefix best-path changes |
| `system-rib/best-change` | `rib` | `fib-kernel`, `fib-p4` | Batch of system-wide best route changes |
| `fib/external-change` | `fib-kernel` | monitoring | External route change on ze-managed prefix |
<!-- source: internal/component/bgp/plugins/rib/rib_bestchange.go -- bestChangeTopic -->
<!-- source: internal/component/sysrib/sysrib.go -- system-rib topic -->
<!-- source: internal/plugins/fib/kernel/monitor.go -- externalChangeTopic -->
<!-- source: internal/plugins/fib/kernel/fibkernel.go -- system-rib/best-change subscription -->

Bus topics in the sysctl pipeline:

| Topic | Publisher | Subscriber | Payload |
|-------|-----------|------------|---------|
| `sysctl/default` | `fib-kernel`, `iface` | `sysctl` | Plugin-required kernel default (key, value, source) |
| `sysctl/set` | CLI | `sysctl` | Transient value from user (key, value) |
| `sysctl/applied` | `sysctl` | any | Notification after kernel write (key, value, source) |
| `sysctl/show-request` | CLI | `sysctl` | Request active keys table (request-id) |
| `sysctl/show-result` | `sysctl` | requester | Active keys JSON (request-id, entries) |
| `sysctl/list-request` | CLI | `sysctl` | Request known keys table (request-id) |
| `sysctl/list-result` | `sysctl` | requester | Known keys JSON (request-id, entries) |
| `sysctl/clear-profile-defaults` | `iface` | `sysctl` | Clear stale profile defaults for an interface before re-emission (interface) |
<!-- source: internal/component/plugin/server/events.go -- NamespaceSysctl, EventSysctl* -->
<!-- source: internal/component/sysctl/register.go -- EventBus subscribe/emit -->

### Route Filters

Plugins can declare named filters at stage 1 for import and/or export filtering.
Each filter specifies which attributes it needs, and the engine sends only those
attributes as text for each UPDATE. Filters respond accept, reject, or modify
(delta-only). See [Route Filters](https://github.com/ze-software/ze/blob/main/docs/guide/redistribution.md) for configuration.

A single plugin can offer multiple named filters. Config references them as
`<plugin>:<filter>` (e.g., `rpki:validate`, `community:scrub`).

| Category | Behavior | Example |
|----------|----------|---------|
| Mandatory | Always on, cannot be overridden | `rfc:otc` |
| Default | On by default, overridable per-peer | `rfc:no-self-as` |
| User | Explicit in `filter {}` config | `rpki:validate` |

Filters can declare `overrides` to remove default filters from the chain
(e.g., `allow-own-as:relaxed` overrides `rfc:no-self-as` for a specific peer).

<!-- source: plan/learned/479-redistribution-filter.md -- redistribution filter design -->

### Cross-Protocol Redistribute (`redistribute-orchestrator`)

`redistribute-orchestrator` is the single subscriber that dispatches non-consumer
protocol route-change events to registered `RedistConsumer` implementations.
Unlike the route filter chain above (which gates intra-BGP traffic),
the orchestrator lets operators redistribute locally-originated routes from
other protocols (L2TP sessions, connected interface prefixes, static routes,
future OSPF / ISIS) into destination protocols (BGP, future OSPF/ISIS).

Config:

```
redistribute {
    destination bgp {
        import connected;
        import static;
        import l2tp { family [ ipv4/unicast ipv6/unicast ]; }
    }
}
```

Each `destination <protocol>` names a registered consumer. Under it,
`import <source>` enables one non-consumer protocol. The import rule's
`source` is the protocol's canonical name registered via
`redistribute.RegisterSource`. Per-source `family` lists narrow which
address families are redistributed; an empty list means "all families".
An import is scoped to its enclosing `destination`: an `import` under
`destination bgp` feeds only BGP, not OSPF/IS-IS.

The orchestrator **auto-loads** when `redistribute {}` appears in the
config. No `plugin { internal redistribute-orchestrator { use redistribute-orchestrator } }`
block is required.

Reactor per-peer NEXT_HOP substitution applies: when the producer leaves
`NextHop` zero, the reactor stamps each peer's local session address as the
NEXT_HOP. Producers that have an explicit address pass it through verbatim.

**Late-join replay:** a route injected by a source into `destination bgp` also
reaches a BGP peer that establishes AFTER the injection. On a peer's down->up
edge the orchestrator emits a `redistevents.ReplayRequest` carrying an opaque
`ReplayID` token; each producer re-emits its current set with the token echoed,
and the orchestrator targets only the newly-established peer. Producers stay
peer-agnostic; the orchestrator holds the `ReplayID -> peer` mapping. Out-of-process
producers re-emit asynchronously, so the mapping is held for a TTL. This closes the
gap for dynamic/inbound peers not present in the reactor map at injection time.

Counters: `ze_bgp_redistribute_events_received`, `_announcements`,
`_withdrawals`, `_filtered_protocol_total`, `_filtered_rule_total`,
`ze_bgp_redistribute_replay_total{source}` (routes replayed to a newly-established peer).

<!-- source: internal/component/bgp/plugins/redistribute_egress/redistribute.go -- consumer plugin -->
<!-- source: internal/component/bgp/plugins/redistribute_egress/replay.go -- late-join replay-on-peer-up -->
<!-- source: internal/core/redistevents/registry.go -- ProtocolID + producer registration -->

### Prefix-List Filter (`bgp-filter-prefix`)

`bgp-filter-prefix` is a built-in filter plugin that matches IPv4 and IPv6
routes against ordered prefix lists defined in `bgp { policy { prefix-list
NAME { ... } } }`. Each list is a sequence of `entry <CIDR>` blocks with
optional `ge` / `le` bounds and an `action` of `accept` or `reject`; the
first matching entry wins and no match is an implicit deny.

Filter chain references use the standard `<plugin>:<filter>` form:
`bgp-filter-prefix:CUSTOMERS`. The shorter form `prefix-list:CUSTOMERS` also
resolves to the same plugin via the filter-type registration, and a bare
`CUSTOMERS` resolves if no other filter plugin claims a filter of that name.

| UPDATE content | Filter action |
|----------------|---------------|
| Single prefix, accepted | `accept` (passes through) |
| Single prefix, denied | `reject` (update dropped) |
| Multi-prefix, all accepted | `accept` (passes through) |
| Multi-prefix, all denied | `reject` (update dropped) |
| Multi-prefix, mixed | `modify` -- rewrites the UPDATE NLRI section to carry only the accepted prefixes, denied prefixes are silently removed. cmd-4 phase 2. |

The mixed case supports only IPv4 unicast legacy NLRI in v1. For
multiprotocol families (MP_REACH_NLRI), the plugin falls back to whole-
update accept when any prefix passes -- implementing per-NLRI rewriting for
MP_REACH requires declaring `raw=true` on the filter registration and
rewriting the attribute value directly.

<!-- source: internal/component/bgp/plugins/filter_prefix/filter_prefix.go -- handleFilterUpdate, cmd-4 phase 1 & 2 -->
<!-- source: internal/component/bgp/plugins/filter_prefix/match.go -- partitionUpdate -->
<!-- source: internal/component/bgp/reactor/filter_delta.go -- extractLegacyNLRIOverride -->

### AS-Path Filter (`bgp-filter-aspath`)

`bgp-filter-aspath` matches the UPDATE's AS-path against ordered regex
entries defined in `bgp { policy { as-path-list NAME { entry REGEX { action
accept|reject; } } } }`. The AS-path is converted to a space-separated
decimal string (e.g., `"65001 65002 65003"`) and each entry's regex is
matched using Go's RE2 engine (linear time, inherently ReDoS-safe). First
match wins; no match is implicit deny.

Chain references: `bgp-filter-aspath:NAME`, `as-path-list:NAME`, or bare
`NAME`. Config authors should use `[0-9]` instead of `\d` in regex strings
because ze's config parser interprets backslash as an escape character.

<!-- source: internal/component/bgp/plugins/filter_aspath/filter_aspath.go -- handleFilterUpdate -->
<!-- source: internal/component/bgp/plugins/filter_aspath/match.go -- evaluateASPath, extractASPathField -->

### Community Match Filter (`bgp-filter-community-match`)

`bgp-filter-community-match` checks for presence of a specific community
value in the route's standard, large, or extended community attributes.
Defined in `bgp { policy { community-match NAME { entry COMMUNITY { type
standard|large|extended; action accept|reject; } } } }`. First match wins;
no match is implicit deny.

Separate from the tag/strip community plugin (`bgp-filter-community`) because
intent differs: this plugin filters (accept/reject), that one modifies
(tag/strip). They can coexist in the same deployment.

Chain references: `bgp-filter-community-match:NAME` or `community-match:NAME`.

Well-known community names (`no-export`, `no-advertise`, `blackhole`, etc.)
work as match values because the filter text format renders them as names.

<!-- source: internal/component/bgp/plugins/filter_community_match/filter_community_match.go -- handleFilterUpdate -->
<!-- source: internal/component/bgp/plugins/filter_community_match/match.go -- evaluateCommunities -->

### Route Attribute Modifier (`bgp-filter-modify`)

`bgp-filter-modify` unconditionally applies declared operations on every route
that reaches it in the filter chain. Three operation types are supported:

**Set** (absolute value): `set { local-preference 200; med 50; origin igp;
next-hop 10.0.0.1; as-path-prepend 3; }`. Only present leaves are applied.

**Increment/Decrement** (relative adjustment): `increment { local-preference 50; }`
or `decrement { med 30; }`. Supported attributes: local-preference, med, aigp.
Increment saturates at uint32 max (4294967295). Decrement floors at 0.
Set and increment/decrement for the same attribute are mutually exclusive.

**Community Add/Remove**: `set { community-add [ 65000:200 ]; community-remove
[ 65000:100 ]; large-community-add [ 65000:100:200 ]; }`. Adds or removes
individual community values (standard, large, extended) without replacing the
entire attribute. The engine maps these to AttrModAdd/AttrModRemove operations.

For conditional modification, compose with match filters earlier in the chain:
`filter import [ prefix-list:CUSTOMERS modify:PREFER-LOCAL ]`.

Chain references: `bgp-filter-modify:NAME` or `modify:NAME`.

<!-- source: internal/component/bgp/plugins/filter_modify/filter_modify.go -- handleFilterUpdate, buildDynamicDelta -->
<!-- source: internal/component/bgp/plugins/filter_modify/modify.go -- buildDelta, buildDynamicDelta -->
<!-- source: internal/component/bgp/reactor/filter_delta.go -- ExtractASPathPrependOps, communityDirectives -->

### AS-Path Length Filter (`bgp-filter-aspath-length`)

`bgp-filter-aspath-length` accepts or rejects routes based on AS_PATH hop count.
Configure named filters with min and/or max bounds:
`bgp { policy { as-path-length REJECT-LONG { max 30; } } }`.
Routes outside the configured range are rejected. At least one of max or min
is required. Path length counts AS_SEQUENCE entries individually and AS_SET as 1,
following RFC 4271 Section 9.1.2.2.

Chain references: `bgp-filter-aspath-length:NAME` or `as-path-length:NAME`.

<!-- source: internal/component/bgp/plugins/filter_aspath_length/filter_aspath_length.go -- handleFilterUpdate -->
<!-- source: internal/component/bgp/plugins/filter_aspath_length/aspath_length.go -- evaluateASPathLength, countASPathHops -->

### Remove Private AS (`bgp-filter-remove-private-as`)

`bgp-filter-remove-private-as` removes RFC 6996 Private Use ASNs from AS path
attributes in an import or export policy chain. Define named actions in
`bgp { policy { remove-private-as NAME { ... } } }` and reference them from a
peer, group, or global filter chain by their unique name.

Default mode strips private ASNs from `AS_PATH` and `AS4_PATH`. The optional
`replace-with peer-as` mode replaces each private ASN with the destination peer
AS on export, or the source peer AS on import.

```
bgp {
    policy {
        remove-private-as STRIP {
        }
        remove-private-as REPLACE {
            replace-with peer-as
        }
    }
    peer transit-a {
        filter {
            export [ STRIP ]
        }
    }
}
```

Filter instance names are globally unique under `bgp policy`. When a name is
unique, reference it directly (e.g. `STRIP`). The prefixed forms
`remove-private-as:STRIP` and `bgp-filter-remove-private-as:STRIP` remain
accepted for disambiguation or advanced use.

The plugin emits policy intent only. The reactor performs the wire rewrite so
AS_SEQUENCE, AS_SET, and confederation segment structure is preserved. On export
to EBGP peers, private-AS removal runs before the normal local-AS prepend.

<!-- source: internal/component/bgp/plugins/filter_remove_private_as/filter_remove_private_as.go -- handleFilterUpdate -->
<!-- source: internal/component/bgp/reactor/filter_delta.go -- ExtractRemovePrivateASOps -->
<!-- source: internal/component/bgp/reactor/reactor_api_forward.go -- export policy before EBGP prepend -->

To test what a filter would do without sending traffic, use `show policy test`:

```
ze show policy test peer upstream1 export filter STRIP update <BGP-UPDATE-HEX>
```

This returns per-filter trace output showing accept/reject/modify decisions and
changed attributes. See the [command reference](../command-reference/index.md) for full syntax.
<!-- source: internal/component/bgp/plugins/cmd/policy/handler.go -- handleShowPolicyTest -->

### NLRI Encoders/Decoders

NLRI plugins register address family support at init time via `family.MustRegister(afi, safi, afiStr, safiStr)`. The four base families (`ipv4/unicast`, `ipv6/unicast`, `ipv4/multicast`, `ipv6/multicast`) live in `internal/core/family/registry.go` itself; everything else is owned by its plugin's `types.go`. Plugins are loaded automatically when the corresponding family is configured.

| Plugin | Families |
|--------|----------|
| `bgp-nlri-vpn` | ipv4/mpls-vpn, ipv6/mpls-vpn |
| `bgp-nlri-evpn` | l2vpn/evpn |
| `bgp-nlri-vpls` | l2vpn/vpls |
| `bgp-nlri-flowspec` | ipv4/flow, ipv6/flow, ipv4/flow-vpn, ipv6/flow-vpn |
| `bgp-nlri-labeled` | ipv4/mpls-label, ipv6/mpls-label |
| `bgp-nlri-mup` | ipv4/mup, ipv6/mup |
| `bgp-nlri-mvpn` | ipv4/mvpn, ipv6/mvpn |
| `bgp-nlri-rtc` | ipv4/rtc |
| `bgp-nlri-ls` | bgp-ls/bgp-ls, bgp-ls/bgp-ls-vpn |
<!-- source: internal/component/bgp/plugins/nlri/vpn/types.go -- VPN family types -->
<!-- source: internal/component/bgp/plugins/nlri/evpn/types.go -- EVPN family types -->
<!-- source: internal/component/bgp/plugins/nlri/vpls/types.go -- VPLS family types -->
<!-- source: internal/component/bgp/plugins/nlri/flowspec/types.go -- FlowSpec family types -->
<!-- source: internal/component/bgp/plugins/nlri/labeled/types.go -- labeled unicast family types -->
<!-- source: internal/component/bgp/plugins/nlri/mup/types.go -- MUP family types -->
<!-- source: internal/component/bgp/plugins/nlri/mvpn/types.go -- MVPN family types -->
<!-- source: internal/component/bgp/plugins/nlri/rtc/types.go -- RTC family types -->
<!-- source: internal/component/bgp/plugins/nlri/ls/types.go -- BGP-LS family types -->
<!-- source: internal/core/family/registry.go -- family registry -->

## Hub Configuration

For external plugins that connect over TLS (non-internal mode), configure the hub:

```
plugin {
    hub {
        server local {
            host 127.0.0.1;
            port 0;                               # auto-assign port
            secret change-this-token-to-at-least-32-chars;  # TLS auth token
        }
    }
}
```
<!-- source: internal/component/hub/yang/ -- hub config YANG schema; pkg/plugin/sdk/ -- TLS auth -->

## Writing External Plugins

External plugins communicate with ze using the same newline-framed YANG RPC protocol as internal plugins: `#<id> <verb> [json]`. External processes connect back to the plugin hub over TLS using the `ZE_PLUGIN_HUB_*` environment variables set by the engine. The Go SDK in `pkg/plugin/sdk` is the reference implementation; the functional-test helper `test/scripts/ze_api.py` shows the Python shape:

```python
from ze_api import API

api = API()
api.declare_done()
api.wait_for_config()
api.capability_done()
api.wait_for_registry()
api.subscribe(['update direction received'])
api.ready()

# Event loop
while True:
    event = api.read_line(timeout=1.0)
    if event:
        # process event JSON
        pass
```

See [plugin-development/protocol.md](https://github.com/ze-software/ze/blob/main/docs/plugin-development/protocol.md) for the full protocol reference.
<!-- source: pkg/plugin/sdk/sdk.go -- NewFromTLSEnv, Run -->
<!-- source: test/scripts/ze_api.py -- API YANG RPC client -->

## Dependencies

Plugins can declare dependencies on other plugins. The engine starts plugins in dependency order and delivers state/EOR events to dependents first.

```
# bgp-gr depends on bgp-rib
# bgp-rpki depends on bgp-adj-rib-in
# bgp-rs optionally uses bgp-adj-rib-in
```

Dependencies are declared in the plugin's registration, not in config. The engine resolves them automatically. Two kinds:

| Kind | Field | Behaviour if missing |
|------|-------|----------------------|
| Hard | `Dependencies` | Startup fails with `ErrMissingDependency`. |
| Optional | `OptionalDependencies` | Silently skipped. Plugin owner handles runtime absence (typically a one-shot WARN + feature disabled). |

`bgp-rs` uses `bgp-adj-rib-in` optionally: when both are loaded, replay-on-peer-up works; when `bgp-adj-rib-in` is absent, forwarding still works and a single WARN log announces that replay is disabled. `bgp-rs` forwards via the typed `Plugin.ForwardCached` / `ReleaseCached` fast path (rs-fastpath-3) instead of the legacy text-RPC `bgp cache forward <id> <sel>` pipeline. See [architecture/api/commands](https://github.com/ze-software/ze/blob/main/docs/architecture/api/commands.md#fast-path-typed-sdk-rs-fastpath-3) for the full SDK surface.
<!-- source: internal/component/plugin/registry/registry.go -- Registration.Dependencies + Registration.OptionalDependencies -->
<!-- source: internal/component/bgp/plugins/rs/server_forward.go -- flushBatch via Plugin.ForwardCached -->

## Debugging Plugins

The plugin debug shell lets you manually interact with the engine using the plugin protocol. This is useful when debugging plugin code -- you can send individual commands and inspect responses.

```
ze bgp plugin cli
```

The debug shell:

1. Asks about handshake parameters (plugin name, families) with defaults -- hit Enter to accept
2. Connects to the daemon via SSH
3. Runs the 5-stage plugin handshake over the SSH channel
4. Enters interactive command mode

Available post-handshake commands:

| Command | Description |
|---------|-------------|
| `dispatch-command <cmd>` | Dispatch an engine command |
| `subscribe-events <events>` | Subscribe to events |
| `unsubscribe-events` | Unsubscribe from events |
| `decode-nlri <family> <hex>` | Decode NLRI from hex |
| `encode-nlri <family> <args>` | Encode NLRI |
| `bye` | Disconnect |

Use `--name <name>` to set a custom plugin name for the session.
<!-- source: internal/component/bgp/cli/cmd_plugin.go -- cmdPluginCLI -->
