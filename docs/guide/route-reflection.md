# Route Reflection

Ze can operate as a route server (RFC 7947) or route reflector, forwarding received routes to other peers. The `bgp-rs` plugin handles route forwarding with zero-copy wire optimization when peers share the same encoding context.
<!-- source: internal/component/bgp/plugins/rs/register.go -- bgp-rs registration, RFC 7947 -->

## Configuration

```
plugin {
    external rs {
        run "ze.bgp-rs"
    }
    external adj-rib-in {
        run "ze.bgp-adj-rib-in"
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

### Forwarding and Congestion

Each destination peer has a dedicated forwarding worker (long-lived goroutine with a buffered channel). When a destination peer is slower than the update rate:

1. The channel buffer absorbs short bursts (default capacity: 64 items)
2. If the channel is full, items go into a per-worker overflow buffer
3. The worker fires a congestion event (visible in logs and Prometheus metrics)
4. When the peer catches up and the channel drains below 25%, congestion clears

Overflow is bounded by a global token pool (default: 100,000 items, configurable via `ze.fwd.pool.size`). When the pool is exhausted, items fall back to unbounded append and a warning is logged. Routes are never dropped -- missing a route update is worse than using extra memory. Prometheus metrics expose per-destination overflow depth (`ze_bgp_overflow_items`), per-source overflow ratio (`ze_bgp_overflow_ratio`), and global pool utilization (`ze_bgp_pool_used_ratio`).
<!-- source: internal/component/bgp/reactor/forward_pool.go -- per-destination forward workers, overflow pool -->
<!-- source: internal/component/bgp/reactor/reactor_metrics.go -- overflow Prometheus metrics -->

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
<!-- source: internal/component/bgp/plugins/rs/ -- RunRouteServer; internal/component/bgp/plugins/adj_rib_in/ -- adj-rib-in for replay -->

## Cache Commands

Routes in transit can be managed via cache commands:

| Command | Description |
|---------|-------------|
| `cache list` | List cached messages |
| `cache forward <id> <peer>` | Forward cached message to peer |
| `cache release <id>` | Release message from cache |

## Without Route Reflection

When `bgp-rs` is not loaded, received routes are not forwarded to other peers. The `bgp-rib` plugin stores routes and performs best-path selection, but does not re-advertise them. To forward routes between peers, load the `bgp-rs` plugin.
<!-- source: internal/component/bgp/plugins/rib/register.go -- bgp-rib registration -->
