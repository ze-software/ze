# ADD-PATH and PATHS-LIMIT

ADD-PATH (RFC 7911) allows multiple paths per prefix by including a Path Identifier with each NLRI. Route servers use it to forward all available paths rather than one best path.

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
| `limit 1` | Effectively single-path behaviour |
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

### Ze Generates Its Own Path Identifiers

RFC 7911 Section 2 requires a speaker that re-advertises a route to generate its own Path Identifier, and to assign it so that (prefix, identifier) uniquely names a path advertised to a neighbor. Both forward rails do that instead of relaying the identifier the source chose.

Each route-server client picks its identifiers alone. Two clients that both pick `1` for one prefix would reach a third client as one (prefix, identifier) pair, and RFC 7911 Section 5 makes the receiver treat the second as a replacement for the first, so one path is lost. A source that negotiated no ADD-PATH sends every path under identifier `0`, so without regeneration every path from every ordinary client reached an ADD-PATH client as (prefix, 0).

| Property | Behavior |
|----------|----------|
| Key | The path at ingress: the source that sent it and the identifier that source used. Never the message, never the attributes |
| Withdrawal | A withdrawn route carries no path attributes, so the ingress key is the only key that can be recomputed when the path leaves. The withdrawal names the identifier Ze advertised |
| Re-announcement | A source that re-announces one path with changed attributes keeps the identifier it already has, so the destination sees a replacement rather than a duplicate |
| Release | Identifiers are released at peer removal, not at session down. A reconnecting peer re-announces under the identifiers its destinations already hold |
| Zero | Minted and accepted like any other value, which RFC 7911 Section 3 requires |

Regeneration runs whenever either side of the forward frames identifiers. A session where neither side negotiated ADD-PATH keeps its zero-copy forward.
<!-- source: internal/component/bgp/reactor/forward_path_id.go -- fwdPathIDs, fwdRegenerateRawPathIDs -->
<!-- source: internal/component/bgp/reactor/forward_body.go -- fwdReencodeNLRIs -->

## Interaction with Route Reflection

ADD-PATH fits the route server plugin (`bgp-rs`). Without ADD-PATH, the route server can only forward one path per prefix to each peer. With ADD-PATH, it forwards all received paths, and downstream routers make their own best-path decisions.
<!-- source: internal/component/bgp/plugins/rs/ -- route server ADD-PATH forwarding -->
