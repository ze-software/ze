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
| Monitor only (no RIB) | None | Ze runs without plugins -- peers connect, events fire, no routes stored |

NLRI family plugins (bgp-nlri-evpn, bgp-nlri-vpn, etc.) are loaded automatically when you configure the corresponding address family. You don't need to declare them.
<!-- source: internal/component/bgp/plugins/nlri/ -- NLRI plugin registrations with Families field -->

## Loading Plugins

Plugins are declared in the `plugin { }` block:

```
plugin {
    external rib {
        run "ze plugin bgp-rib"
        encoder json
    }
    external adj-rib-in {
        run "ze plugin bgp-adj-rib-in"
        encoder json
    }
    external gr {
        run "ze plugin bgp-gr"
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
| Fork | `run "ze plugin bgp-rib"` | Subprocess, TLS connect-back (default) |
| Internal | `run "ze.bgp-rib"` | Goroutine + net.Pipe (best performance) |
| Path | `run "/usr/local/bin/my-plugin"` | External binary |

Internal mode (`ze.pluginname`) runs the plugin as a goroutine within the ze process, using direct function calls instead of IPC. This is the fastest mode but requires the plugin to be compiled into ze.
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
<!-- source: internal/component/bgp/plugins/gr/register.go; internal/component/bgp/plugins/rpki/register.go; internal/component/bgp/plugins/rpki_decorator/register.go; internal/component/bgp/plugins/route_refresh/register.go; internal/component/bgp/plugins/role/register.go; internal/component/bgp/plugins/hostname/register.go; internal/component/bgp/plugins/softver/register.go; internal/component/bgp/plugins/llnh/register.go -->

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

### NLRI Encoders/Decoders

NLRI plugins register address family support. They are loaded automatically when the corresponding family is configured.

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
<!-- source: internal/component/bgp/plugins/nlri/vpn/register.go; internal/component/bgp/plugins/nlri/evpn/register.go; internal/component/bgp/plugins/nlri/vpls/register.go; internal/component/bgp/plugins/nlri/flowspec/register.go; internal/component/bgp/plugins/nlri/labeled/register.go; internal/component/bgp/plugins/nlri/mup/register.go; internal/component/bgp/plugins/nlri/mvpn/register.go; internal/component/bgp/plugins/nlri/rtc/register.go; internal/component/bgp/plugins/nlri/ls/register.go -->

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
# bgp-rs depends on bgp-adj-rib-in
```

Dependencies are declared in the plugin's registration, not in config. The engine resolves them automatically.
<!-- source: internal/component/plugin/registry/registry.go -- Registration.Dependencies -->
