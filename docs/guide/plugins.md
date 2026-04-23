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
| Service healthcheck | `bgp-healthcheck` + `bgp-watchdog` | Monitor services, control route announcement via MED or withdraw. [Guide](healthcheck.md) |
| Monitor only (no RIB) | None | Ze runs without plugins -- peers connect, events fire, no routes stored |
| Interface-aware BGP | `iface` + `bgp-rib` | React to OS interface changes -- start/stop BGP listeners when addresses appear/disappear |
| Static routes | (auto-loaded) | Config-driven static routes with ECMP, weighted load balancing, BFD failover. [Guide](static-routes.md) |

NLRI family plugins (bgp-nlri-evpn, bgp-nlri-vpn, etc.) are loaded automatically when you configure the corresponding address family. You don't need to declare them.
<!-- source: internal/component/bgp/plugins/nlri/ -- NLRI plugin registrations with Families field -->

## Config-Driven Loading

BGP itself is a config-driven plugin. If your config has a `bgp { }` section, BGP loads automatically. If it doesn't, ze starts without BGP (useful for interface-only or FIB-only deployments). You can add or remove `bgp { }` at runtime via config reload (SIGHUP) and BGP will start or stop dynamically. The same mechanism works for `interface { }` and other top-level config sections.
<!-- source: internal/component/plugin/server/startup_autoload.go -- getConfigPathPlugins, autoLoadForNewConfigPaths -->

## Loading Plugins

Plugins are declared in the `plugin { }` block:

```
plugin {
    external rib {
        use bgp-rib
        encoder json
    }
    external adj-rib-in {
        use bgp-adj-rib-in
        encoder json
    }
    external gr {
        use bgp-gr
        encoder json
    }
}
```

### Plugin Block Settings

| Setting | Description |
|---------|-------------|
| `run` | Command to start the plugin |
| `encoder` | Wire encoding: `json` (default) or `text` |

## Binding Plugins to Peers

Each peer declares which plugins receive its events via `process` blocks. The process name must match the plugin's `external` name in the `plugin { }` block:

```
plugin {
    external rib { ... }       # <-- this name
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
| Internal | `use bgp-rib` | Goroutine + net.Pipe (best performance) |
| External | `run "/usr/local/bin/my-plugin"` | External binary or script |

Internal mode (`use pluginname`) runs the plugin as a goroutine within the ze process, using direct function calls instead of IPC. This is the fastest mode but requires the plugin to be compiled into ze.
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
<!-- source: internal/component/bgp/plugins/rib/register.go; internal/component/bgp/plugins/adj_rib_in/register.go; internal/component/bgp/plugins/persist/register.go; internal/component/bgp/plugins/rs/register.go; internal/component/bgp/plugins/watchdog/register.go -->

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
<!-- source: internal/component/bgp/plugins/gr/register.go; internal/component/bgp/plugins/rpki/register.go; internal/component/bgp/plugins/rpki_decorator/register.go; internal/component/bgp/plugins/route_refresh/register.go; internal/component/bgp/plugins/role/register.go; internal/component/bgp/plugins/hostname/register.go; internal/component/bgp/plugins/softver/register.go; internal/component/bgp/plugins/llnh/register.go; internal/component/bgp/plugins/bmp/register.go -->

### Infrastructure

| Plugin | Description | Process Binding |
|--------|-------------|-----------------|
| `iface` | OS interface orchestration: loads backend, dispatches operations | -- (Bus events, no peer binding) |
| `iface-netlink` | Netlink backend for iface: manage, monitor, bridge, sysctl, mirror | -- (registered as iface backend) |
| `iface-dhcp` | DHCP client: DHCPv4/DHCPv6 lease acquisition and renewal | -- (Bus events) |
| `rib` | System RIB: selects best route across protocols by admin distance | -- (Bus events, no peer binding) |
| `fib-kernel` | FIB kernel: programs OS routes from system RIB via netlink | -- (Bus events, no peer binding) |
| `fib-p4` | FIB P4: programs P4 switch from system RIB via gRPC/P4Runtime (noop backend) | -- (Bus events, no peer binding) |
| `sysctl` | Kernel tunable management: three-layer precedence (config > transient > default), restore on stop. Named profiles (dsr, router, hardened, multihomed, proxy) for interface units. User-defined profiles. | -- (Bus events, CLI commands) |
<!-- source: internal/plugins/sysctl/register.go -- sysctl registration -->
<!-- source: internal/component/iface/register.go -- iface registration -->
<!-- source: internal/plugins/iface/netlink/register.go -- iface-netlink backend registration -->
<!-- source: internal/plugins/iface/dhcp/register.go -- iface-dhcp registration -->
<!-- source: internal/plugins/sysrib/register.go -- rib plugin registration -->
<!-- source: internal/plugins/fib/p4/register.go -- fib-p4 registration -->
<!-- source: internal/plugins/fib/kernel/register.go -- fib-kernel registration -->

### L2TP

| Plugin | Description |
|--------|-------------|
| `l2tp-auth-local` | Static user/password authentication for L2TP PPP sessions (PAP/CHAP-MD5/MS-CHAPv2) |
| `l2tp-auth-radius` | RADIUS authentication (Access-Request), accounting (Start/Stop/Interim-Update), and CoA/DM listener |
| `l2tp-pool` | Bitmap-backed IPv4 address pool with RADIUS-directed pool selection (Framed-Pool attribute) |
| `l2tp-shaper` | TC traffic shaping (TBF/HTB) on pppN interfaces with RADIUS CoA rate updates |

These plugins register via the L2TP handler registry (`RegisterAuthHandler`,
`RegisterPoolHandler`) at init time. Only one auth handler is active at a
time (last registered wins). The pool and shaper plugins subscribe to
session lifecycle events via the EventBus.

See [L2TP guide](l2tp.md) for configuration details.

<!-- source: internal/plugins/l2tpauthlocal/register.go -->
<!-- source: internal/plugins/l2tpauthradius/register.go -->
<!-- source: internal/plugins/l2tppool/register.go -->
<!-- source: internal/plugins/l2tpshaper/register.go -->

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
<!-- source: internal/plugins/sysrib/sysrib.go -- system-rib topic, protocolRoute, admin distance selection -->

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
<!-- source: internal/plugins/sysrib/sysrib.go -- system-rib topic -->
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
<!-- source: internal/component/plugin/events.go -- NamespaceSysctl, EventSysctl* -->
<!-- source: internal/plugins/sysctl/register.go -- EventBus subscribe/emit -->

### Redistribution Filters (planned)

Plugins can declare named filters at stage 1 for import and/or export filtering.
Each filter specifies which attributes it needs, and the engine sends only those
attributes as text for each UPDATE. Filters respond accept, reject, or modify
(delta-only). See [Redistribution Guide](redistribution.md) for configuration.

A single plugin can offer multiple named filters. Config references them as
`<plugin>:<filter>` (e.g., `rpki:validate`, `community:scrub`).

| Category | Behavior | Example |
|----------|----------|---------|
| Mandatory | Always on, cannot be overridden | `rfc:otc` |
| Default | On by default, overridable per-peer | `rfc:no-self-as` |
| User | Explicit in `redistribution {}` config | `rpki:validate` |

Filters can declare `overrides` to remove default filters from the chain
(e.g., `allow-own-as:relaxed` overrides `rfc:no-self-as` for a specific peer).

<!-- source: plan/spec-redistribution-filter.md -- redistribution filter design -->

### Cross-Protocol Redistribute (`bgp-redistribute-egress`)

`bgp-redistribute-egress` is the single subscriber that turns non-BGP protocol
route-change events into BGP UPDATE announcements. Unlike the redistribution
filter chain above (which gates intra-BGP traffic), `bgp-redistribute-egress` lets
operators advertise locally-originated routes from other protocols (L2TP
sessions today, future connected / static / OSPF / ISIS) to BGP peers.

Config:

```
redistribute {
    import l2tp { family [ ipv4/unicast ipv6/unicast ]; }
    import fakeredist;          # all families
}
```

Each `import <source>` enables one non-BGP protocol. The import rule's
`source` is the protocol's canonical name registered via
`redistribute.RegisterSource`. Per-source `family` lists narrow which
address families are advertised; an empty list means "all families".

The egress consumer **auto-loads** when `redistribute {}` appears in the
config. No `plugin { external bgp-redistribute-egress { use bgp-redistribute-egress } }`
block is required. Add an explicit block only if you need to override
plugin defaults (encoder, respawn policy, etc).

Reactor per-peer NEXT_HOP substitution applies: when the producer leaves
`NextHop` zero, the reactor stamps each peer's local session address as the
NEXT_HOP. Producers that have an explicit address pass it through verbatim.

Counters: `ze_bgp_redistribute_events_received`, `_announcements`,
`_withdrawals`, `_filtered_protocol_total`, `_filtered_rule_total`.

<!-- source: internal/component/bgp/plugins/redistribute/redistribute.go -- consumer plugin -->
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

`bgp-filter-modify` unconditionally sets declared attributes on every route
that reaches it in the filter chain. Defined in `bgp { policy { modify NAME {
set { local-preference 200; med 50; origin igp; next-hop 10.0.0.1;
as-path-prepend 3; } } } }`. Only present leaves are applied; undeclared
attributes pass through unchanged.

For conditional modification, compose with match filters earlier in the chain:
`filter import [ prefix-list:CUSTOMERS modify:PREFER-LOCAL ]`.

Chain references: `bgp-filter-modify:NAME` or `modify:NAME`.

The plugin returns `action=modify` with a pre-built text delta. The engine
handles wire-level rewriting via `textDeltaToModOps` and
`buildModifiedPayload`. AS-path prepend uses a dedicated `AttrModPrepend`
handler that inserts N copies of the peer's local AS before the existing path.

<!-- source: internal/component/bgp/plugins/filter_modify/filter_modify.go -- handleFilterUpdate -->
<!-- source: internal/component/bgp/reactor/filter_delta.go -- ExtractASPathPrependOps -->
<!-- source: internal/component/bgp/reactor/filter_delta_handlers.go -- aspathHandler -->

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
<!-- source: internal/component/bgp/plugins/nlri/vpn/types.go; internal/component/bgp/plugins/nlri/evpn/types.go; internal/component/bgp/plugins/nlri/vpls/types.go; internal/component/bgp/plugins/nlri/flowspec/types.go; internal/component/bgp/plugins/nlri/labeled/types.go; internal/component/bgp/plugins/nlri/mup/types.go; internal/component/bgp/plugins/nlri/mvpn/types.go; internal/component/bgp/plugins/nlri/rtc/types.go; internal/component/bgp/plugins/nlri/ls/types.go; internal/core/family/registry.go -->

## Hub Configuration

For external plugins that connect over TLS (non-internal mode), configure the hub:

```
plugin {
    hub {
        server local {
            host 127.0.0.1;
            port 0;                               # auto-assign port
            secret my-shared-secret-key-here;     # TLS auth token
        }
    }
}
```
<!-- source: internal/component/hub/schema/ -- hub config YANG schema; pkg/plugin/sdk/ -- TLS auth -->

## Writing External Plugins

External plugins communicate with ze over a JSON-RPC protocol. Ze provides a Python SDK:

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

See [plugin-development/](../plugin-development/) for the full protocol reference.

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

`bgp-rs` uses `bgp-adj-rib-in` optionally: when both are loaded, replay-on-peer-up works; when `bgp-adj-rib-in` is absent, forwarding still works and a single WARN log announces that replay is disabled. `bgp-rs` forwards via the typed `Plugin.ForwardCached` / `ReleaseCached` fast path (rs-fastpath-3) instead of the legacy text-RPC `bgp cache <id> forward <sel>` pipeline. See [architecture/api/commands](../architecture/api/commands.md#fast-path-typed-sdk-rs-fastpath-3) for the full SDK surface.
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
<!-- source: cmd/ze/bgp/cmd_plugin.go -- cmdPluginCLI -->
