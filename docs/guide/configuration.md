# Configuration

Ze uses a JUNOS-like hierarchical configuration format.

## Syntax Rules

| Element | Syntax | Example |
|---------|--------|---------|
| Blocks | `name { ... }` | `bgp { ... }` |
| Values | `key value` or `key value;` | `hold-time 180` |
| Comments | `#` to end of line | `# this is a comment` |
| Lists | `[ item1 item2 ]` | `receive [ update state ]` |
| Strings | Unquoted or `"double quoted"` | `run "ze plugin bgp-rib"` |
| Terminators | Optional semicolons (`;`) | Both `hold-time 180` and `hold-time 180;` work |
| Inline blocks | `name { key value; key value; }` | `remote { ip 10.0.0.1; as 65001; }` |

Indentation is not significant. Unknown keys are rejected with a suggestion for the closest valid key.
<!-- source: internal/component/bgp/schema/ze-bgp-conf.yang -- BGP config YANG schema; internal/component/bgp/config/resolve.go -- ResolveBGPTree -->

## File Format

```
# Global BGP settings
bgp {
    router-id 1.2.3.4;
    local-as 65000;

    # Peer group with shared defaults
    group upstream {
        hold-time 180;
        connection active;

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
            hold-time 90;    # overrides group value
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
| Group | Peers in group | `group upstream { hold-time 180; }` |
| Peer | Single peer | `peer transit-a { hold-time 90; }` |

Containers (like `capability`, `family`) are deep-merged across levels. Leaf values (like `hold-time`) override.
<!-- source: internal/component/bgp/config/resolve.go -- ResolveBGPTree, inheritance merging -->

## Peer Settings

Peers are keyed by name (`peer <name> { }`) where the name must start with a letter.

| Setting | Description | Required |
|---------|-------------|----------|
| `remote { ip; as; }` | Peer IP and AS number | Yes |
| `local { ip; as; }` | Local bind address and AS | Yes (ip can be `auto`) |
| `router-id` | BGP router ID | Yes (or inherited) |
| `description` | Human-readable description | No |
| `hold-time` | Hold timer seconds (0 or 3-65535) | No (default: 180) |
| `connection` | Connect mode: `active`, `passive`, `both` | No (default: both) |
| `port` | TCP port | No (default: 179) |
| `md5-password` | TCP MD5 authentication | No |
| `ttl-security` | Minimum TTL for incoming packets | No |
| `outgoing-ttl` | TTL for outgoing packets | No |
| `group-updates` | Enable/disable UPDATE grouping | No (default: enable) |
<!-- source: internal/component/bgp/config/peers.go -- PeersFromTree; internal/component/bgp/schema/ze-bgp-conf.yang -- peer settings -->

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
<!-- source: internal/component/bgp/reactor/session_prefix.go -- prefix limit enforcement; internal/component/bgp/schema/ze-bgp-conf.yang -- prefix config -->

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
