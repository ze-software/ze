# Route Reflection

Ze can operate as a route server (RFC 7947) or route reflector, forwarding received routes to other peers. The `bgp-rs` plugin handles route forwarding with zero-copy wire optimization when peers share the same encoding context.

## Configuration

```
plugin {
    external rs {
        run "ze plugin bgp-rs"
        encoder json
    }
    external adj-rib-in {
        run "ze plugin bgp-adj-rib-in"
        encoder json
    }
}

bgp {
    peer client-a {
        remote { ip 10.0.0.1; as 65001; }
        local { ip 10.0.0.254; as 65000; }
        router-id 10.0.0.254;

        family { ipv4/unicast; }

        process rs {
            receive [ update ]
            send [ update ]
        }
        process adj-rib-in {
            receive [ update state ]
        }
    }

    peer client-b {
        remote { ip 10.0.0.2; as 65002; }
        local { ip 10.0.0.254; as 65000; }
        router-id 10.0.0.254;

        family { ipv4/unicast; }

        process rs {
            receive [ update ]
            send [ update ]
        }
        process adj-rib-in {
            receive [ update state ]
        }
    }
}
```

## How It Works

### Forward-All Model

The route server forwards all received routes to all other peers. There is no best-path selection -- every route from every peer is forwarded to every other peer. This is the RFC 7947 route server model used at Internet Exchange Points.

### Zero-Copy Forwarding

When two peers negotiate identical capabilities (same ADD-PATH mode, same ASN format, same extended message support), they share the same encoding context. Routes between peers with matching contexts are forwarded as raw wire bytes without re-encoding -- no parse, no rebuild, no allocation.

### Backpressure

Each destination peer has a dedicated forwarding worker (long-lived goroutine + channel). If a peer falls behind, the worker applies backpressure by pausing the source peer's TCP read. This prevents unbounded memory growth without dropping routes.

### Convergent Replay

When a peer reconnects, the route server replays all stored routes from other peers:

1. Full snapshot replay from adj-rib-in
2. Delta loop catches routes received during replay
3. End-of-RIB sent when caught up

## Plugin Bindings

The `bgp-rs` plugin requires:
- `receive [ update ]` -- receives route updates from the peer
- `send [ update ]` -- sends forwarded routes back to the peer

The `bgp-adj-rib-in` plugin stores received routes for replay on peer reconnect.

## Cache Commands

Routes in transit can be managed via cache commands:

| Command | Description |
|---------|-------------|
| `cache list` | List cached messages |
| `cache forward <id> <peer>` | Forward cached message to peer |
| `cache release <id>` | Release message from cache |

## Without Route Reflection

When `bgp-rs` is not loaded, ze operates as a standard BGP speaker. Routes are not forwarded between peers.
