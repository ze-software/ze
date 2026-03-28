# Configuration

Ze uses a JUNOS-like hierarchical configuration format.

## Syntax Rules

| Element | Syntax | Example |
|---------|--------|---------|
| Blocks | `name { ... }` | `bgp { ... }` |
| Values | `key value` or `key value;` | `router-id 1.2.3.4` |
| Comments | `#` to end of line | `# this is a comment` |
| Lists | `[ item1 item2 ]` | `receive [ update state ]` |
| Strings | Unquoted or `"double quoted"` | `run "ze plugin bgp-rib"` |
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
    ipv4/vpn { prefix { maximum 500; } }
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

## Hub Configuration

The plugin hub provides TLS transport for plugin communication and fleet management.
Named `server` blocks declare listeners; hub-level `client` blocks declare outbound
connections to a remote hub (managed mode).

```
plugin {
    hub {
        server local {
            host 127.0.0.1;
            port 1790;
            secret "local-secret-minimum-32-characters";
        }
        server central {
            host 0.0.0.0;
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

## ExaBGP Migration

Ze auto-detects and migrates ExaBGP configuration files:

```
ze config migrate exabgp.conf > ze.conf
```
<!-- source: cmd/ze/config/cmd_migrate.go -- cmdMigrate; internal/exabgp/migration/ -- ExaBGP config conversion -->
