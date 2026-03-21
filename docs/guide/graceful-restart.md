# Graceful Restart

Graceful Restart (RFC 4724) preserves forwarding state across BGP session restarts. When a peer goes down and comes back, routes are held during the restart window instead of being immediately withdrawn, preventing traffic black-holes.

## Configuration

```
plugin {
    external gr {
        run "ze plugin bgp-gr"
        encoder json
    }
    external rib {
        run "ze plugin bgp-rib"
        encoder json
    }
}

bgp {
    peer upstream1 {
        remote { ip 10.0.0.1; as 65001; }
        ...
        capability {
            graceful-restart {
                restart-time 120;
            }
        }

        process gr {
            receive [ state eor ]
        }
        process rib {
            receive [ state ]
            send [ update ]
        }
    }
}
```

### Config Reference

| Path | Type | Default | Description |
|------|------|---------|-------------|
| `graceful-restart / restart-time` | uint16 | 120 | Seconds to hold stale routes during restart (0-4095) |
| `graceful-restart / mode` | enum | -- | `require`: reject peers without GR capability |
| `graceful-restart / disable` | presence | -- | Disable GR for this peer |

## How It Works

### Normal Session

1. Peer session establishes, GR capability negotiated in OPEN
2. Routes received and installed in RIB normally

### Peer Restarts

1. **Peer goes down** -- GR plugin sends `retain-routes` to RIB
2. **RIB marks routes as stale** -- routes kept in forwarding but flagged
3. **Restart timer starts** -- countdown from `restart-time` seconds
4. **Peer reconnects** -- new session established, fresh routes received
5. **Fresh routes replace stale** -- each new route implicitly clears its stale flag
6. **End-of-RIB received** -- GR plugin sends `purge-stale` to RIB
7. **Remaining stale routes removed** -- any route not refreshed is withdrawn

### Restart Timer Expiry

If the peer does not reconnect within `restart-time` seconds, all stale routes are purged. A safety margin of 5 seconds is added to account for processing delays.

### Fail-Safe

If the GR plugin crashes or fails to issue `purge-stale`, the RIB automatically expires stale routes after `restart-time + 5s`.

## Plugin Bindings

The GR plugin requires:
- `receive [ state eor ]` -- needs peer up/down events and End-of-RIB markers
- The RIB plugin must also be loaded with `receive [ state ]` and `send [ update ]`

The GR plugin depends on `bgp-rib` (declared in its registration). The engine ensures bgp-rib starts first.

## CLI

```
$ ze cli --run "rib routes received"
# Shows routes with stale flag when applicable
```

## Without Graceful Restart

When GR is not configured or the peer does not advertise the GR capability, routes are withdrawn immediately on session down. No stale state, no restart timer.
