# Configuration

Ze uses a JUNOS-like hierarchical configuration format.

## Syntax Rules

| Element | Syntax | Example |
|---------|--------|---------|
| Blocks | `name { ... }` | `bgp { ... }` |
| Values | `key value` or `key value;` | `router-id 1.2.3.4` |
| Comments | `#` to end of line | `# this is a comment` |
| Lists | `[ item1 item2 ]` | `receive [ update state ]` |
| Strings | Unquoted or `"double quoted"` | `run "/usr/bin/my-plugin"` |
| Terminators | Optional semicolons (`;`) | Both `router-id 1.2.3.4` and `router-id 1.2.3.4;` work |
| Inline blocks | `name { key value; key value; }` | `remote { ip 10.0.0.1; as 65001; }` |

Indentation is not significant. Unknown keys are rejected with a suggestion for the closest valid key.
<!-- source: internal/component/bgp/schema/ze-bgp-conf.yang -- BGP config YANG schema; internal/component/bgp/config/resolve.go -- ResolveBGPTree -->

## File Format

```
# Global BGP settings
bgp {
    router-id 1.2.3.4;
    local { as 65000; }

    # Peer group with shared defaults
    group upstream {
        timer {
            receive-hold-time 180;
        }
        local { connect false; }   # passive: don't initiate outbound

        capability {
            asn4;
            route-refresh;
        }

        family {
            ipv4/unicast;
            ipv6/unicast;
        }

        peer transit-a {
            remote { ip 10.0.0.1; as 65001; }
        }

        peer transit-b {
            remote { ip 10.0.0.2; as 65002; }
            timer {
                receive-hold-time 90;    # overrides group value
            }
        }
    }

    # Standalone peer (no group)
    peer internal {
        remote { ip 192.168.1.1; as 65000; }
        local { ip 192.168.1.2; as 65000; }
        router-id 192.168.1.2

        family {
            ipv4/unicast
        }
    }
}
```

## Inheritance

Configuration uses 3-level inheritance: BGP globals, group defaults, peer overrides.

| Level | Scope | Example |
|-------|-------|---------|
| BGP | All peers | `bgp { router-id 1.2.3.4; }` |
| Group | Peers in group | `group upstream { timer { receive-hold-time 180; } }` |
| Peer | Single peer | `peer transit-a { timer { receive-hold-time 90; } }` |

Containers (like `capability`, `family`, `timer`) are deep-merged across levels. Leaf values (like `receive-hold-time` inside `timer`) override.
<!-- source: internal/component/bgp/config/resolve.go -- ResolveBGPTree, inheritance merging -->

## Peer Settings

Peers are keyed by name (`peer <name> { }`) where the name must start with a letter.

| Setting | Description | Required |
|---------|-------------|----------|
| `remote { ip; as; }` | Peer IP and AS number | Yes |
| `local { ip; as; }` | Local bind address and AS | Yes (ip can be `auto`) |
| `router-id` | BGP router ID | Yes (or inherited) |
| `description` | Human-readable description | No |
| `timer { }` | Timer container: `receive-hold-time` (seconds, 0 or 3-65535, default 90), `send-hold-time` (seconds, 0 or 480-65535, default 0), `connect-retry` (seconds, default 120) | No |
| `local { connect }` | Initiate outbound TCP connections: `true` or `false` (default: true) | No |
| `remote { accept }` | Accept inbound TCP connections: `true` or `false` (default: true) | No |
| `port` | TCP port | No (default: 179) |
| `md5-password` | TCP MD5 authentication | No |
| `ttl-security` | Minimum TTL for incoming packets | No |
| `outgoing-ttl` | TTL for outgoing packets | No |
| `group-updates` | Enable/disable UPDATE grouping | No (default: enable) |
<!-- source: internal/component/bgp/config/peers.go -- PeersFromTree; internal/component/bgp/schema/ze-bgp-conf.yang -- peer settings, container timer -->

## Capabilities

Configured under `capability { }` at any inheritance level.

| Capability | Config | Values |
|------------|--------|--------|
| 4-byte ASN | `asn4` | presence (enabled by default) |
| Route Refresh | `route-refresh` | presence |
| Extended Message | `extended-message` | presence |
| Graceful Restart | `graceful-restart { restart-time 120; }` | See [Graceful Restart guide](graceful-restart.md) |
| ADD-PATH | `add-path send/receive` | See [ADD-PATH guide](add-path.md) |
| Extended Next Hop | `nexthop { ipv4/unicast ipv6; }` | Per-family NH mapping |
| BGP Role | `role provider` | provider, customer, rs, rs-client, peer |
<!-- source: internal/component/bgp/schema/ze-bgp-conf.yang -- capability definitions -->

## Address Families

Configured under `family { }`. Each family requires a `prefix { maximum N; }` block.

```
family {
    ipv4/unicast { prefix { maximum 1000000; } }
    ipv6/unicast { prefix { maximum 200000; } }
    ipv4/mpls-vpn { prefix { maximum 500; } }
    l2vpn/evpn { prefix { maximum 10000; } }
}
```

Use `ze --plugins` to see available families from registered plugins.
<!-- source: internal/component/bgp/plugins/nlri/ -- NLRI plugin Families registration -->

### Prefix Limits

Every family must have a prefix maximum. Ze refuses to start without one.

```
family {
    ipv4/unicast {
        prefix {
            maximum 1000000
            warning 900000
        }
    }
}
```

Warning defaults to 90% of maximum when not set. Peer-level settings control enforcement behavior:

```
peer transit-a {
    prefix {
        teardown false
        idle-timeout 30
    }
    family {
        ipv4/unicast { prefix { maximum 1000000; } }
    }
}
```

| Setting | Default | Description |
|---------|---------|-------------|
| `teardown` | `true` | Send NOTIFICATION and close on exceed. `false` = warn only, drop excess NLRIs. |
| `idle-timeout` | `0` | Seconds before reconnect after prefix teardown. 0 = no reconnect. |

### PeeringDB Prefix Data

Ze can query PeeringDB to suggest prefix maximums. Configure the PeeringDB API URL and margin in the system block:

```
system {
    peeringdb {
        url "https://www.peeringdb.com"
        margin 10
    }
}
```

| Setting | Default | Description |
|---------|---------|-------------|
| `url` | `https://www.peeringdb.com` | PeeringDB-compatible API base URL. |
| `margin` | `10` | Percentage added above PeeringDB prefix count (0-100). |

Run `ze update bgp peer * prefix` to query PeeringDB and update prefix maximums. Review changes with `ze config diff`, then apply with `ze config commit`.
<!-- source: internal/component/bgp/reactor/session_prefix.go -- prefix limit enforcement; internal/component/bgp/schema/ze-bgp-conf.yang -- prefix config -->
<!-- source: internal/component/config/system/schema/ze-system-conf.yang -- peeringdb config -->

## Process Bindings

Plugins are bound to peers via `process` blocks. Each process block names a plugin and configures what events it receives.

```
peer transit-a {
    ...
    process adj-rib-in {
        receive [ update state ]
    }
    process rpki {
        receive [ update ]
    }
}
```

Base event types for `receive`: `update`, `open`, `notification`, `keepalive`, `refresh`, `state`, `negotiated`, `eor`, `rpki`. Plugins may register additional types (e.g., `update-rpki` from bgp-rpki-decorator). Validated at config parse time against registered event types.
<!-- source: internal/component/bgp/event.go -- event type definitions; internal/component/plugin/registry/registry.go -- EventTypes registration -->

## Redistribution Filters (planned)

Route filtering is configured via `redistribution` blocks at bgp, group, or peer level.
Values are `<plugin>:<filter>` references to named filters declared by plugins.

```
bgp {
    redistribution {
        import [ rpki:validate ]
    }
    group customers {
        redistribution {
            import [ community:scrub ]
        }
        peer customer-a {
            remote { ip 10.0.0.1; as 65001; }
            redistribution {
                export [ aspath:prepend ]
            }
        }
    }
}
```

Chains are cumulative: bgp-level filters run first, then group, then peer.
Mandatory filters (e.g., `rfc:otc`) always run before user filters and cannot
be removed. Default filters (e.g., `rfc:no-self-as`) run unless overridden by
a filter that declares `overrides`.

See [Redistribution Guide](redistribution.md) for details.
<!-- source: plan/spec-redistribution-filter.md -- redistribution filter config design -->

### Prefix-List Filter

Named prefix-list filters live under `bgp { policy { prefix-list NAME { ... } } }`
and are invoked via peer filter chains. Each list is an ordered sequence
of match entries, first-match-wins, with an implicit deny on no match.

```
bgp {
    policy {
        prefix-list CUSTOMERS {
            entry 10.0.0.0/8 {
                ge 16
                le 24
                action accept
            }
            entry 192.168.0.0/16 {
                ge 24
                le 24
                action reject
            }
        }
    }

    peer customer-a {
        connection { remote { ip 10.0.0.1; as 65001; } }
        filter {
            import [ bgp-filter-prefix:CUSTOMERS ]
        }
    }
}
```

Each `entry` has:

| Field | Type | Purpose |
|-------|------|---------|
| `ge` | uint | Minimum prefix length (inclusive). Defaults to the entry's own CIDR length. |
| `le` | uint | Maximum prefix length (inclusive). Defaults to 32 (IPv4) / 128 (IPv6). |
| `action` | enum | `accept` or `reject`. |

A route matches an entry when its address is contained in the entry's CIDR
AND its prefix length satisfies `ge <= length <= le`. Entries are evaluated
in the order they appear; the first match determines the action. A route
that matches no entry is implicitly rejected.

**Multi-prefix UPDATEs.** When an UPDATE carries several prefixes and the
prefix-list accepts some but rejects others, the filter returns `modify`:
the UPDATE is rewritten to carry only the accepted prefixes. The rejected
ones are silently removed. In v1 this applies to the legacy IPv4 unicast
NLRI only; IPv6 and labeled families still use whole-update accept/reject
semantics.

Chain reference forms accepted by the engine:

| Form | Example |
|------|---------|
| `<plugin>:<filter>` | `bgp-filter-prefix:CUSTOMERS` |
| `<filter-type>:<filter>` | `prefix-list:CUSTOMERS` |
| Plain filter name | `CUSTOMERS` (when no other plugin claims a filter by that name) |

<!-- source: internal/component/bgp/plugins/filter_prefix/schema/ze-filter-prefix.yang -- prefix-list YANG container -->
<!-- source: internal/component/bgp/plugins/filter_prefix/config.go -- parsePrefixLists -->
<!-- source: internal/component/bgp/plugins/filter_prefix/filter_prefix.go -- handleFilterUpdate, per-prefix partition modify path -->

### AS-Path Filter

Named AS-path regex filters live under `bgp { policy { as-path-list NAME { ... } } }`.
Each list is an ordered sequence of regex entries with accept/reject action.
The AS-path is converted to a space-separated decimal string and matched
using Go RE2 (linear time, inherently ReDoS-safe). Use `[0-9]` instead of
`\d` in regex strings (ze's config parser interprets backslash as escape).

```
bgp {
    policy {
        as-path-list ALLOW-PEER {
            entry "^65001" { action accept }
        }
        as-path-list TRANSIT {
            entry "^[0-9]+( [0-9]+)+" { action accept }
        }
    }

    peer customer-a {
        filter { import [ as-path-list:ALLOW-PEER ] }
    }
}
```

<!-- source: internal/component/bgp/plugins/filter_aspath/schema/ze-filter-aspath.yang -- as-path-list YANG container -->
<!-- source: internal/component/bgp/plugins/filter_aspath/config.go -- parseAsPathLists -->

### Community Match Filter

Named community-match filters live under `bgp { policy { community-match NAME { ... } } }`.
Each entry checks for presence of a specific community value in the route's
standard, large, or extended community attributes.

```
bgp {
    policy {
        community-match BLOCK-NO-EXPORT {
            entry no-export { type standard; action reject }
        }
        community-match ALLOW-CUSTOMER {
            entry 65001:100 { type standard; action accept }
            entry 65001:100:200 { type large; action accept }
        }
    }

    peer transit-a {
        filter { import [ community-match:BLOCK-NO-EXPORT ] }
    }
}
```

<!-- source: internal/component/bgp/plugins/filter_community_match/schema/ze-filter-community-match.yang -- community-match YANG container -->
<!-- source: internal/component/bgp/plugins/filter_community_match/config.go -- parseCommunityLists -->

### Route Attribute Modifier

Named modifier definitions live under `bgp { policy { modify NAME { set { ... } } } }`.
Only present leaves are applied; undeclared attributes pass through unchanged.
For conditional modification, compose with match filters earlier in the chain.

```
bgp {
    policy {
        modify PREFER-LOCAL {
            set {
                local-preference 200
            }
        }
        modify PREPEND-3 {
            set {
                as-path-prepend 3
            }
        }
    }

    peer transit-a {
        filter {
            import [ prefix-list:CUSTOMERS modify:PREFER-LOCAL ]
            export [ modify:PREPEND-3 ]
        }
    }
}
```

| Leaf | Type | Range | Purpose |
|------|------|-------|---------|
| `local-preference` | uint32 | 0-4294967295 | Set LOCAL_PREF |
| `med` | uint32 | 0-4294967295 | Set MED |
| `origin` | enum | igp, egp, incomplete | Set ORIGIN |
| `next-hop` | IP address | IPv4 | Set NEXT_HOP |
| `as-path-prepend` | uint8 | 1-32 | Prepend local AS N times |

<!-- source: internal/component/bgp/plugins/filter_modify/schema/ze-filter-modify.yang -- modify YANG container -->
<!-- source: internal/component/bgp/plugins/filter_modify/config.go -- parseModifyDefs -->

## Static Routes

Routes can be configured directly per peer:

```
peer transit-a {
    ...
    update {
        attribute {
            origin igp
            next-hop 10.0.0.2
            local-preference 100
            community [ no-export ]
        }
        nlri {
            ipv4/unicast add 10.0.0.0/24
            ipv4/unicast add 10.0.1.0/24
        }
    }
}
```

<!-- source: internal/component/bgp/schema/ze-bgp-conf.yang -- static route config, update/attribute/nlri blocks -->

## Interface Configuration

Ze manages network interfaces with a descriptive-name model. Each interface type is a
YANG list keyed by a user-chosen name. The MAC address serves as the binding between the
config entry and the physical (or virtual) hardware.

<!-- source: internal/component/iface/schema/ze-iface-conf.yang -- interface container, list definitions -->

### Interface Types

| Type | Description | MAC required |
|------|-------------|--------------|
| `ethernet` | Physical ethernet interface | Yes |
| `veth` | Virtual ethernet pair | Yes |
| `bridge` | Bridge interface | Yes |
| `dummy` | Virtual dummy interface | No |
| `loopback` | Loopback (container, no key) | No |

<!-- source: internal/component/iface/schema/ze-iface-conf.yang -- ethernet, veth, bridge, dummy, loopback definitions -->

### MAC Address Binding

For ethernet, veth, and bridge interfaces, `mac-address` is required and must be unique
within each type. The MAC ties the named config entry to the actual hardware. Names are
descriptive labels chosen by the operator; rename the config entry freely without losing
the physical binding.

<!-- source: internal/component/iface/schema/ze-iface-conf.yang -- unique "mac-address", ze:required "mac-address" -->

### Discovery During Init

Running `ze init` discovers OS interfaces via netlink (Linux) or stdlib (other platforms)
and writes initial config to `ze.conf`. Each discovered interface gets an entry named
after its OS name, with `mac-address` populated and an `os-name` hidden field preserving
the original OS name. Loopback appears as an empty `loopback { }` container.

<!-- source: cmd/ze/init/main.go -- generateInterfaceConfig -->
<!-- source: internal/component/iface/discover.go -- DiscoverInterfaces -->

### Example

```
interface {
    ethernet uplink {
        mac-address 00:1a:2b:3c:4d:5e;
    }
    ethernet mgmt {
        mac-address 00:1a:2b:3c:4d:5f;
        mtu 1500;
    }
    bridge fabric {
        mac-address 00:1a:2b:3c:4d:60;
        stp true;
    }
    dummy blackhole {
    }
    loopback {
    }
}
```

The MAC address validator provides format checking and live autocomplete from currently
discovered OS interfaces when editing config interactively.

<!-- source: internal/component/config/validators.go -- MACAddressValidator -->

### Route Priority

The `route-priority` leaf on a unit sets the Linux route metric for default routes
on that interface. Lower values are preferred by the kernel. When a link goes down,
the metric is increased by 1024 to deprioritize the interface, allowing traffic to
shift to an alternative uplink. When the link comes back up, the original metric is
restored.

For IPv4, ze installs the default route with the configured metric when a DHCP lease
provides a gateway. For IPv6, ze suppresses the kernel's automatic RA default route
(`accept_ra_defrtr=0`) and installs `::/0` routes with the configured metric when
NDP neighbor events indicate a router (NTF_ROUTER flag). Multiple routers on the
same link are each installed with the same metric. On clean shutdown or config
removal, `accept_ra_defrtr` is restored to 1.

<!-- source: internal/component/iface/schema/ze-iface-conf.yang -- route-priority leaf -->
<!-- source: internal/component/iface/register.go -- handleLinkDown, handleLinkUp, handleLinkDownIPv6, handleLinkUpIPv6 -->
<!-- source: internal/component/iface/register.go -- suppressAcceptRaDefrtr, handleRouterDiscovered -->

```
interface {
    ethernet uplink {
        mac-address 00:1a:2b:3c:4d:5e;
        unit 0 {
            route-priority 1;
            dhcp {
                enabled true;
            }
        }
    }
    ethernet backup {
        mac-address 00:1a:2b:3c:4d:5f;
        unit 0 {
            route-priority 5;
            dhcp {
                enabled true;
            }
        }
    }
}
```

With this config, uplink (metric 1) is preferred over backup (metric 5). If uplink
goes down, its metric becomes 1025 (1 + 1024), so backup (metric 5) takes over. When
uplink recovers, its metric returns to 1 and traffic shifts back. The default value
is 0 (kernel default), which preserves existing behavior when not configured.

## Authentication Users

Local SSH login users are declared under `system.authentication.user`. The
list key is the username; each entry has a one-way bcrypt-hashed password
and an optional list of authorization profile names.

```
system {
    authentication {
        user alice {
            plaintext-password "secret"     # write-only; hashed on commit
            profile [ admin ]
        }
        user bob {
            password "$2a$10$..."           # canonical form, paste from `ze passwd`
            profile [ read-only ]
        }
    }
}
```

| Leaf | Stored on disk | Notes |
|------|---------------|-------|
| `password` | bcrypt hash (`$2a$10$...`) | Canonical, displayed in `show config`. Hand-editing a literal plaintext here triggers a `ze config validate` warning. |
| `plaintext-password` | never persisted | Junos-style write-only input. The commit hook bcrypt-hashes the value into `password` and removes this leaf. |

Both leaves are mutually exchangeable inputs; only `password` survives a
commit. For end-to-end usage (login, hashing, multi-user setup) see
[authentication.md](authentication.md).
<!-- source: internal/component/ssh/schema/ze-ssh-conf.yang -- system.authentication.user -->
<!-- source: internal/component/config/password_hash.go -- ApplyPasswordHashing -->

## Sysctl Configuration

Kernel tunables are managed by the `sysctl` plugin via a generic key/value list:

```
sysctl {
    setting net.ipv4.conf.all.forwarding {
        value 1
    }
    setting net.core.somaxconn {
        value 4096
    }
}
```

Keys use kernel-native names (e.g., the `/proc/sys/` path with `/` replaced by `.`
on Linux, or MIB names on Darwin). Known keys get type and range validation on
commit; unknown keys are validated by attempting the write.

The sysctl plugin uses three-layer precedence: config values (above) are authoritative
and override both transient values (`sysctl set` from CLI) and plugin defaults
(e.g., fib-kernel declaring forwarding=1). See the
[command reference](command-reference.md#sysctl-kernel-tunables) for CLI usage.
<!-- source: internal/plugins/sysctl/sysctl.go -- parseSysctlConfig, applyConfig -->
<!-- source: internal/plugins/sysctl/schema/ze-sysctl-conf.yang -- sysctl container -->

### Sysctl Profiles

Named profiles group co-dependent kernel tunables applied per interface unit.
Five built-in profiles cover common network operator use cases:

| Profile | Purpose | Keys set |
|---------|---------|----------|
| `dsr` | Direct Server Return ARP tuning | arp_announce=2, arp_ignore=1 |
| `router` | Enable IPv4/IPv6 forwarding | forwarding=1 (both) |
| `hardened` | Anti-spoofing | rp_filter=1, log_martians=1, arp_filter=1 |
| `multihomed` | Prevent ARP flux | arp_filter=1 |
| `proxy` | Proxy ARP | proxy_arp=1, arp_accept=1 |

Apply profiles to an interface unit:

```
interface {
    ethernet eth0 {
        unit 0 {
            sysctl-profile [ dsr hardened ]
        }
    }
}
```

Profiles emit as defaults: explicit `sysctl { setting ... }` config overrides them.
Multiple profiles per unit are composable; last wins on key overlap.

User-defined profiles are declared in the `sysctl` config block:

```
sysctl {
    profile my-edge {
        setting net.ipv4.conf.<iface>.forwarding {
            value 1
        }
        setting net.ipv4.conf.<iface>.rp_filter {
            value 2
        }
    }
}
```

The `<iface>` placeholder is substituted with the actual interface name at apply time.
<!-- source: internal/core/sysctl/profiles.go -- ProfileDef, builtinProfiles, ResolveProfileSettings -->
<!-- source: internal/component/iface/config.go -- applySysctlProfiles -->

## Environment Block

Global settings outside BGP:

```
environment {
    tcp {
        attempts 3;        # connection retry attempts
    }
    log {
        level warn;        # default log level
        bgp.routes debug;  # per-subsystem override
    }
}
```

<!-- source: internal/component/config/environment.go -- environment block parsing; internal/core/slogutil/slogutil.go -- log level config -->

### Named Listeners

Every service that accepts inbound connections models its listen endpoints
as a named YANG list:

```
environment {
    web {
        enabled true;
        server primary {
            ip 0.0.0.0;
            port 3443;
        }
        server admin {
            ip 127.0.0.1;
            port 13443;
        }
    }
}
```

Each `server <name> { ... }` block becomes a bound TCP listener on the same
service. The same pattern applies to `environment.ssh`, `environment.mcp`,
`environment.looking-glass`, `telemetry.prometheus`, `environment.api-server.rest`,
`environment.api-server.grpc`, and `plugin.hub`.

<!-- source: internal/component/config/yang/modules/ze-types.yang — grouping listener -->
<!-- source: internal/component/config/yang/modules/ze-extensions.yang — extension listener -->
<!-- source: internal/component/config/loader_extract.go — ExtractWebConfig, ExtractLGConfig, ExtractMCPConfig, ExtractAPIConfig -->

Binder lifetime rules are uniform across every service:

| Rule | Detail |
|------|--------|
| Minimum | At least one entry is required when a service is enabled. An enabled block with no `server {}` uses the YANG `refine` default (e.g. `0.0.0.0:3443` for web). |
| Bind order | Entries are bound in config declaration order for live trees; in `map[string]any` roundtrips (after `ToMap()`) the ordering falls back to alphabetical key order. |
| Failure mode | Bind is all-or-nothing: if any listener fails to bind, the already-bound listeners are closed and the service fails to start. Partial binding is never accepted. |
| Shutdown | Every listener closes when the service is stopped; there is no "primary plus extras" asymmetry. |
| Insecure web | `insecure true` forces every entry's ip to `127.0.0.1` with a warning logged per rewrite. |
| MCP | MCP is localhost-only. Non-loopback entries are rewritten to `127.0.0.1` with a warning. |

<!-- source: internal/component/web/server.go — WebServer.ListenAndServe multi-listener bind-and-rollback -->
<!-- source: internal/component/lg/server.go — LGServer.ListenAndServe multi-listener bind-and-rollback -->
<!-- source: internal/core/metrics/server.go — metrics.Server.Start multi-listener bind-and-rollback -->
<!-- source: internal/component/api/rest/server.go — RESTServer.ListenAndServe multi-listener bind-and-rollback -->
<!-- source: internal/component/api/grpc/server.go — GRPCServer.Serve multi-listener bind-and-rollback -->
<!-- source: cmd/ze/hub/mcp.go — startMCPServer multi-listener bind-and-rollback -->

#### Port Conflict Detection

`ze config validate` runs `CollectListeners` over every enabled service and
rejects the config if two listeners would bind overlapping `ip:port` pairs.
Wildcard addresses (`0.0.0.0`, `::`) conflict with any address in the same
family; cross-family entries (`0.0.0.0` vs `::1`) never conflict. The check
covers `web`, `ssh`, `mcp`, `looking-glass`, `prometheus`, `plugin.hub`,
`api-server.rest`, and `api-server.grpc`.

<!-- source: internal/component/config/listener.go — knownListenerServices, CollectListeners, ValidateListenerConflicts -->

#### Environment Variable Override

Each service exposes a `ze.<svc>.listen` variable that accepts one or more
comma-separated `ip:port` pairs. IPv6 entries use bracket notation:

| Variable | Example | Notes |
|----------|---------|-------|
| `ze.web.listen` | `0.0.0.0:3443,[::1]:3443` | Overrides web `server` list entirely |
| `ze.looking-glass.listen` | `0.0.0.0:8443` | Overrides LG `server` list |
| `ze.mcp.listen` | `127.0.0.1:8080,127.0.0.1:18080` | MCP enforces 127.0.0.1 on every entry |
| `ze.api-server.rest.listen` | `0.0.0.0:8081` | Overrides REST `server` list |
| `ze.api-server.grpc.listen` | `0.0.0.0:50051` | Overrides gRPC `server` list |

Env vars replace the config-file list when set; partial merging is not
supported so the precedence is predictable. Precedence per service is:
env var > CLI flag > config file > YANG default.

<!-- source: internal/component/config/environment.go — ParseCompoundListen -->
<!-- source: cmd/ze/hub/main.go — runYANGConfig env/CLI/config resolution for webAddrs, lgAddrs, mcpAddrs -->

### DNS Resolver

Configure a shared DNS resolver for all Ze components:

```
environment {
    dns {
        server 8.8.8.8:53;    # upstream DNS server (empty = system default)
        timeout 5;             # query timeout in seconds (1-60)
        cache-size 10000;      # max cached entries (0 = disable cache)
        cache-ttl 86400;       # max cache TTL in seconds (0 = use response TTL only)
    }
}
```

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `server` | string | `""` | DNS server address with port. Empty uses system `/etc/resolv.conf`. |
| `timeout` | uint16 | `5` | Query timeout in seconds. Range: 1-60. |
| `cache-size` | uint32 | `10000` | Maximum cached entries. 0 disables caching entirely. |
| `cache-ttl` | uint32 | `86400` | Maximum cache entry TTL in seconds. 0 means use only the response TTL. |

The cache respects DNS response TTLs: entries expire at `min(response TTL, cache-ttl)`.
Records with TTL=0 from the server are not cached (per RFC 1035).

### Environment Variable Override

| Variable | Type | Description |
|----------|------|-------------|
| `ze.dns.server` | string | DNS server address (e.g., `8.8.8.8:53`) |
| `ze.dns.timeout` | int | Query timeout in seconds (1-60) |
| `ze.dns.cache-size` | int | Max cached entries (0 = disabled) |
| `ze.dns.cache-ttl` | int | Max cache TTL in seconds (0 = response TTL only) |

Precedence: env var > config file > system default (`/etc/resolv.conf`).

<!-- source: internal/component/dns/schema/ze-dns-conf.yang -- DNS YANG schema -->
<!-- source: internal/component/dns/resolver.go -- NewResolver, Resolve -->
<!-- source: cmd/ze/hub/main.go -- ze.dns.server env registration -->

### Reactor Settings

Configure reactor behavior under `environment { reactor { } }`:

```
environment {
    reactor {
        update-groups true;   # cross-peer UPDATE grouping (default: true)
    }
}
```

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `update-groups` | boolean | `true` | Enable cross-peer UPDATE grouping. When enabled, peers with identical encoding contexts share a single UPDATE build. |

When `update-groups` is true (the default), the reactor groups established peers by their outbound encoding context (ContextID + policy). UPDATEs are built once per group and the wire bytes are fanned out to all group members. This reduces CPU usage proportionally to group size -- for a route server with 100 peers sharing the same capabilities, UPDATE building work is reduced by approximately 100x.

Disable update groups when:

- **ExaBGP compatibility:** migrated configs need per-peer UPDATE behavior matching ExaBGP's model. The `ze exabgp migrate` command automatically injects `update-groups false` into migrated configs.
- **Debugging:** isolating per-peer UPDATE building to diagnose encoding issues.

When disabled (or when all peers have unique encoding contexts), behavior is identical to per-peer building with negligible overhead.

#### Environment Variable Override

| Variable | Type | Description |
|----------|------|-------------|
| `ze.bgp.reactor.update-groups` | bool | Cross-peer UPDATE grouping (default: true) |

<!-- source: internal/component/bgp/schema/ze-bgp-conf.yang -- reactor container, leaf update-groups -->
<!-- source: internal/component/config/environment.go -- ze.bgp.reactor.update-groups registration -->
<!-- source: internal/component/bgp/reactor/update_group.go -- NewUpdateGroupIndexFromEnv -->
<!-- source: internal/exabgp/migration/migrate.go -- injectUpdateGroupsDisabled -->

### L2TP Tunnels

L2TPv2 (RFC 2661) tunnel support uses two config blocks. Protocol settings
live under the root-level `l2tp {}` block. Listener endpoints follow the
standard named-server pattern under `environment { l2tp { } }`.

```
l2tp {
    enabled true;
    shared-secret "my-tunnel-secret";
    hello-interval 60;
    max-tunnels 1000;
    max-sessions 100;
}

environment {
    l2tp {
        server main {
            ip 0.0.0.0;
            port 1701;
        }
    }
}
```
<!-- source: internal/component/l2tp/schema/ze-l2tp-conf.yang -- L2TP YANG schema -->

Protocol settings (root `l2tp {}`):

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `enabled` | boolean | `true` | Presence of `l2tp {}` implies enabled. Use `enabled false` to disable. Use `enabled true` as a filler when no other settings are needed. |
| `shared-secret` | string | (unset) | CHAP-MD5 challenge/response secret (RFC 2661 S4.2). If unset and a peer sends a Challenge AVP, the tunnel is rejected with StopCCN Result Code 4. |
| `hello-interval` | uint16 | (unset) | Seconds of peer silence before sending HELLO (1-3600). RFC 2661 recommends 60. |
| `max-tunnels` | uint16 | 0 (unbounded) | Maximum concurrent tunnels. New SCCRQs beyond the limit receive StopCCN Result Code 2. |
| `max-sessions` | uint16 | 0 (unbounded) | Maximum concurrent sessions per tunnel. New ICRQs/OCRQs beyond the limit receive CDN Result Code 4 (no resources). |

Listener endpoints (`environment { l2tp { } }`):

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `server <name>` | list | `0.0.0.0:1701` | Named UDP listen endpoints, same pattern as other services. |

<!-- source: internal/component/l2tp/config.go -- ExtractParameters -->
<!-- source: internal/component/l2tp/subsystem.go -- L2TPSubsystem lifecycle -->

## Hub Configuration

The plugin hub provides TLS transport for plugin communication and fleet management.
Named `server` blocks declare listeners; hub-level `client` blocks declare outbound
connections to a remote hub (managed mode).

```
plugin {
    hub {
        server local {
            ip 127.0.0.1;
            port 1790;
            secret "local-secret-minimum-32-characters";
        }
        server central {
            ip 0.0.0.0;
            port 1791;
            secret "central-secret-min-32-characters!";
            client edge-01 { secret "per-client-token-min-32-chars!!!"; }
        }
        client edge-01 {
            host 10.0.0.1;
            port 1791;
            secret "per-client-token-min-32-chars!!!";
        }
    }
}
```

Every ze instance has at least one `server` block (for local plugins and SSH).
Secrets must be at least 32 characters. See [Fleet Configuration](fleet-config.md) for details.

<!-- source: internal/component/plugin/schema/ze-plugin-conf.yang -- hub YANG schema -->
<!-- source: internal/component/bgp/config/plugins.go -- ExtractHubConfig -->

## Validation

Validate a configuration file without starting ze:

```
ze config validate myconfig.conf
```

Unknown keys are rejected with a suggestion for the closest valid key.

## Healthcheck

Service healthcheck probes monitor availability and control BGP route announcement via watchdog groups:

```
bgp {
    healthcheck {
        probe dns {
            command "dig @127.0.0.1 example.com +short"
            group hc-dns
            interval 5
            rise 3
            fall 3
            withdraw-on-down false
            up-metric 100
            down-metric 1000
        }
    }
}
```

Each probe runs a shell command periodically. When the service is UP, routes in the watchdog group are announced with `up-metric` as MED. When DOWN, routes are either withdrawn (`withdraw-on-down true`) or re-announced with `down-metric` as MED (default). See [Healthcheck Guide](healthcheck.md) for full configuration reference.
<!-- source: internal/component/bgp/plugins/healthcheck/schema/ze-healthcheck-conf.yang -- YANG schema -->

## ExaBGP Migration

Ze auto-detects and migrates ExaBGP configuration files:

```
ze config migrate exabgp.conf > ze.conf
```
<!-- source: cmd/ze/config/cmd_migrate.go -- cmdMigrate; internal/exabgp/migration/ -- ExaBGP config conversion -->
