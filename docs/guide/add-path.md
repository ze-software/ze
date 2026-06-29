# ADD-PATH and PATHS-LIMIT

ADD-PATH (RFC 7911) allows multiple paths per prefix by including a Path Identifier with each NLRI. This is useful for route servers that need to forward all available paths, not just the best one.

PATHS-LIMIT (draft-abraitis-idr-addpath-paths-limit) lets a receiver advertise the maximum number of paths it wants per prefix per family, preventing uncontrolled path proliferation.
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- add-path capability config -->

## Configuration

All ADD-PATH and PATHS-LIMIT config lives under `session > capability > add-path`.

### Default Direction and Limit

Enable ADD-PATH for all negotiated families, with an optional default path count limit:

```
capability {
    add-path {
        direction send/receive;
        limit 10;
    }
}
```

The `limit` on the container is inherited by all families. Per-family entries can override it.

### Per-Family Override

Override direction, set path count limits, or set negotiation mode per family:

```
capability {
    add-path {
        direction send;
        family {
            ipv4/unicast {
                direction send/receive;
                limit 10;
            }
            ipv6/unicast {
                direction receive;
                mode require;
            }
        }
    }
}
```

### Full Example

```
bgp {
    peer transit-a {
        connection {
            remote { ip 10.0.0.1; }
            local { ip 10.0.0.2; }
        }
        session {
            asn { local 65000; remote 65001; }
            capability {
                add-path {
                    direction send/receive;
                    family {
                        ipv4/unicast { limit 10; }
                    }
                }
            }
            family {
                ipv4/unicast
                ipv6/unicast
            }
        }
    }
}
```

## Direction and Mode

| Direction | Meaning |
|-----------|---------|
| `send` | Advertise multiple paths to peer |
| `receive` | Accept multiple paths from peer |
| `send/receive` | Both directions |

| Mode | Meaning |
|------|---------|
| `enable` (default) | Negotiate ADD-PATH if peer supports it |
| `require` | Reject peer if it does not support ADD-PATH |
| `refuse` | Reject peer if it advertises ADD-PATH |
| `disable` | Do not negotiate ADD-PATH for this family |
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- add-path direction/mode config -->

## PATHS-LIMIT

The `limit` leaf on a per-family entry advertises a PATHS-LIMIT capability (code 76) for that family. The value (1-65535) is the maximum number of paths per prefix the receiver wants.

| Setting | Effect |
|---------|--------|
| No `limit` leaf | No path count limit for that family |
| `limit 10` | Peer will send at most 10 paths per prefix |
| `limit 1` | Effectively single-path behavior |
<!-- source: internal/core/bgp/capability/capability.go -- PathsLimit struct -->

## How It Works

When ADD-PATH is negotiated, each NLRI is prefixed with a 4-byte Path Identifier. This allows the same prefix (e.g., `10.0.0.0/24`) to appear multiple times with different path IDs, each carrying different attributes.

### Wire Format

Without ADD-PATH:
```
[prefix-length][prefix-bytes]
```

With ADD-PATH:
```
[4-byte path-id][prefix-length][prefix-bytes]
```

### Encoding Context

Peers that negotiate the same ADD-PATH modes share an encoding context (`ContextID`). The route server can forward wire bytes unchanged between peers with matching contexts, avoiding re-encoding.
<!-- source: internal/component/bgp/reactor/session.go -- ContextID; internal/component/bgp/reactor/reactor_api_forward.go -- zero-copy forwarding -->

### Route Withdrawal

To withdraw a specific path, the withdrawal NLRI includes the same path ID used in the announcement. Withdrawing without a path ID removes all paths for that prefix.

## Interaction with Route Reflection

ADD-PATH is particularly useful with the route server plugin (`bgp-rs`). Without ADD-PATH, the route server can only forward one path per prefix to each peer. With ADD-PATH, all received paths are forwarded, allowing downstream routers to make their own best-path decisions.
<!-- source: internal/component/bgp/plugins/rs/ -- route server ADD-PATH forwarding -->
