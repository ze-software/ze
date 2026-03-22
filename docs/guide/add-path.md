# ADD-PATH

ADD-PATH (RFC 7911) allows multiple paths per prefix by including a Path Identifier with each NLRI. This is useful for route servers that need to forward all available paths, not just the best one.
<!-- source: internal/component/bgp/schema/ze-bgp-conf.yang -- add-path capability config -->

## Configuration

### Global

Enable ADD-PATH for all families:

```
capability {
    add-path send/receive;
}
```

### Per-Family

Override the global setting for specific families:

```
add-path {
    ipv4/unicast send;
    ipv6/unicast send/receive;
    ipv4/mpls-vpn receive
}
```

### Full Example

```
bgp {
    peer transit-a {
        remote { ip 10.0.0.1; as 65001; }
        local { ip 10.0.0.2; as 65000; }
        router-id 10.0.0.2

        capability {
            add-path send/receive
        }

        family {
            ipv4/unicast
            ipv6/unicast
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
| `disable` | Do not negotiate ADD-PATH for this family |
<!-- source: internal/component/bgp/schema/ze-bgp-conf.yang -- add-path direction/mode config -->

Example with mode:

```
add-path {
    ipv4/unicast send require;
    ipv6/unicast send/receive enable;
}
```

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
