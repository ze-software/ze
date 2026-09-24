# Route Reflection

Ze can operate as a route server (RFC 7947) or route reflector, forwarding received routes to other peers. The `bgp-rs` plugin handles route forwarding with zero-copy wire optimization when peers share the same encoding context.
<!-- source: internal/component/bgp/plugins/rs/register.go -- bgp-rs registration, RFC 7947 -->

## Configuration

```
plugin {
    internal rs {
        use bgp-rs
    }
    internal adj-rib-in {
        use bgp-adj-rib-in
    }
}

bgp {
    peer client-a {
        remote { ip 10.0.0.1; as 65001; }
        local { ip 10.0.0.254; as 65000; }
        router-id 10.0.0.254;

        family { ipv4/unicast; }

        attach process rs {
            receive [ update-received state open-received refresh ]
            send [ update ]
        }
        attach process adj-rib-in {
            receive [ update-received state ]
        }
    }

    peer client-b {
        remote { ip 10.0.0.2; as 65002; }
        local { ip 10.0.0.254; as 65000; }
        router-id 10.0.0.254;

        family { ipv4/unicast; }

        attach process rs {
            receive [ update-received state open-received refresh ]
            send [ update ]
        }
        attach process adj-rib-in {
            receive [ update-received state ]
        }
    }
}
```

### A Route Server Without Per-Peer Entries

An IXP route server can accept its members from a dynamic peer group instead of a `peer` block for each one. The group opens its own listening socket, so a configuration that names no static peer still accepts members.

```
bgp {
    policy {
        reject-asn NO-TRANSIT {
            indirect [ 174 701 3356 ]
        }
    }
    group ix {
        connection {
            remote {
                ip dynamic
                connect false
                range 198.51.100.0/24
                max-peers 500
            }
            local {
                ip 198.51.100.1
                accept true
            }
        }
        session {
            asn { local 64500 }
            router-id 198.51.100.1
            family { ipv4/unicast { prefix { maximum 200000 } } }
        }
        role { import rs }
        filter {
            import [ NO-TRANSIT ]
            export [ NO-TRANSIT ]
        }

        attach process rs {
            receive [ update-received state open-received refresh ]
            send [ update ]
        }
        attach process adj-rib-in {
            receive [ update-received state ]
        }
    }
}
```

The `role { import rs }` leaf makes both `filter` chains mandatory: a declared RFC 9234 role obliges each bound chain to name a filter that can refuse a path through a transit provider (RFC 7454 Section 9), and `rs` binds both. To run a session without the check, name the filter with the `inactive:` member prefix. The chain records the decision, and the filter never executes:

```
filter {
    import [ inactive:NO-TRANSIT ]
    export [ inactive:NO-TRANSIT ]
}
```

The prefix goes on the member, inside the brackets. A separate `inactive: import NO-TRANSIT` statement deactivates a member the chain already carries; where the chain carries none, it deactivates the whole leaf, which leaves no chain and Ze refuses the config. See [config deactivation](config-deactivate.md) for both forms.

A member inherits the group's whole settings, its `attach process` blocks and its per-peer plugin config included. A static peer whose address falls inside the range keeps its own session and its own settings. See [BGP peering](bgp-peering.md) for the group leaves and the reload rules.

## How It Works

### Forward-All Model

The route server forwards eligible received routes to other peers without choosing a best path. Receive validation and each destination's export policy still apply.

When receive validation is enabled, pending and rejected paths are withheld on both the cached forwarding path and the reactor fast path. An UPDATE containing paths with different verdicts forwards only its eligible NLRIs. A later validation change withdraws an advertised path that has become ineligible; recovery replays the retained route with its received attributes, without waiting for another UPDATE. ADD-PATH withdrawals keep the identifier used for the advertisement.
<!-- source: internal/component/bgp/plugins/rs/server_validation.go -- validationChanged, processValidation -->
<!-- source: internal/component/bgp/reactor/forward_validation.go -- forwardUpdateCore, forwardValidationWire -->
<!-- source: internal/component/bgp/reactor/forward_rs.go -- reactorForwardRS validation fallback -->

The route reflector also reconciles retained unicast paths on validation
changes. It coalesces notifications before requesting replay, so a receive-store
callback never waits for an RPC into that same store. Replayed routes pass the
reactor's ordinary reflection rules and egress policy.
<!-- source: internal/component/bgp/plugins/rr/validation.go -- startValidation, replayValidation -->

### Zero-Copy Forwarding

When two peers negotiate identical capabilities (same ADD-PATH mode, same ASN format, same extended message support), they share the same encoding context. Routes between peers with matching contexts are forwarded as raw wire bytes without re-encoding -- no parse, no rebuild, no allocation.

### Forwarding and Congestion

Each destination peer has a dedicated forwarding worker (long-lived goroutine with a buffered channel). When a destination peer is slower than the update rate:

1. The channel buffer absorbs short bursts (default capacity: 64 items)
2. If the channel is full, items go into a per-worker overflow buffer
3. The worker fires a congestion event (visible in logs and Prometheus metrics)
4. When the peer catches up and the channel drains below 25%, congestion clears

Overflow uses a two-tier pool: per-peer pools (64 slots) absorb steady-state traffic, and a shared MixedBufMux overflow pool (auto-sized from peer prefix maximums, overridable via `ze.fwd.pool.size` byte budget) bounds overflow memory. When the overflow pool is exhausted, items proceed without a pool handle and a warning is logged. Routes are never dropped -- missing a route update is worse than using extra memory. Prometheus metrics expose per-destination overflow depth (`ze_bgp_overflow_items`), per-source overflow ratio (`ze_bgp_overflow_ratio`), and global pool utilization (`ze_bgp_pool_used_ratio`).
<!-- source: internal/component/bgp/reactor/forward_pool.go -- per-destination forward workers, overflow pool -->
<!-- source: internal/component/bgp/reactor/reactor_metrics.go -- overflow Prometheus metrics -->

### Convergent Replay

When a peer reconnects, the route server replays eligible stored routes from other peers:

1. Full snapshot replay of non-FlowSpec routes from adj-rib-in
2. Delta loop catches those routes received during replay
3. Authorized FlowSpec paths replay from the mandatory selecting RIB
4. End-of-RIB sent when replay completes

FlowSpec has one snapshot source: ordinary adj-rib-in store replay excludes
those families. Both the route server and reflector replay current authorized rules
from `bgp-rib` before End-of-RIB even when optional-store replay fails. The
route server preserves its received-generation cut, and both roles check the
destination's reconnect generation. Reconstructed routes still pass ordinary
authorization, reflection and export policy.
<!-- source: internal/component/bgp/plugins/rs/server_handlers.go -- replayForPeer -->

The route server splits the UPDATEs for a peer that comes up at a message-id
cut. The replay delivers every UPDATE at or below the cut, and the live forward
delivers every UPDATE above it. The two rails reach the peer through different
processes, so the engine orders them for that peer alone. While a process that
reports the peer's initial update (the route server, the route reflector) has not
yet reported it done, the engine holds the peer's live forwards and validation
changes in the peer's forward queue. The replayed routes pass. The report comes
after the replay's End-of-RIB, and the held changes then go out in the order they
arrived. A live withdrawal can then never arrive before a replayed announcement
of the same prefix, and the first route the source announced arrives first.

Other peers are not held. A live UPDATE goes to every other destination at
once, whatever replay is running. The held changes wait in the peer's forward
overflow queue, so the congestion controls of that queue apply to them: overflow
denial, then teardown of the peer, never a silent drop (see
[the replay fence](../architecture/forward-congestion-pool.md#the-replay-fence)).

A peer-down withdrawal leaves the route server by selector, on the announce
rail. For a peer whose replay is still running, the engine puts it in the same
forward queue as the held live changes, so the replay passes it and it reaches
the peer after the replay's End-of-RIB. A replay read before the receive store
processed the peer-down can still carry the route, and the withdrawal that
follows it removes it.
<!-- source: internal/component/bgp/reactor/peer.go -- forwardOrderHold, initialUpdateOwed, withdrawBehindForwards -->
<!-- source: internal/component/bgp/plugins/rs/server_handlers.go -- handleStateUp, endReplay -->
<!-- source: internal/component/bgp/plugins/rs/server_forward.go -- flushBatch -->
<!-- source: internal/component/bgp/plugins/rs/server_validation.go -- replayFlowSpecs -->
<!-- source: internal/component/bgp/plugins/rr/rr.go -- replayForPeer -->
<!-- source: internal/component/bgp/plugins/rr/validation.go -- replayFlowSpecs -->

If no other plugin owns replay for a particular peer, `bgp-adj-rib-in` obtains
the same authorized FlowSpec snapshot from `bgp-rib` before reporting that its
initial update is ready. A delegated replay owner suppresses this self-replay;
if no owner both receives state events and has permission to send UPDATEs for
the peer, the engine's unheld-role marker restores it. The bounded relay uses
the reactor's existing destination session, family, source-generation and
export checks.
<!-- source: internal/component/bgp/plugins/adj_rib_in/rib.go -- handleStructuredState, handleState -->
<!-- source: internal/component/bgp/plugins/adj_rib_in/rib_replay_path.go -- replayFlowSpecs -->

The route server and reflector advertise the same `bgp-peer-up-replay` role
at startup. Adj-RIB-In consumes that claim during configure, before peers
start; the engine retracts it per peer when no owner both receives that peer's
state event and has `send [ update ]` permission. An observer or a plugin with
only `send [ raw ]` does not suppress Adj-RIB-In's self-replay.
If a reflector joins later or restarts, its post-startup callback uses the
existing `claim-replay` command to notify a receive store that is already
running. The startup declaration still provides ordering for the first peer.
<!-- source: internal/component/bgp/plugins/rr/register.go -- init -->
<!-- source: internal/component/bgp/plugins/adj_rib_in/rib_claims.go -- applyStartupClaims, replayDrivenElsewhere -->
<!-- source: internal/component/bgp/plugins/rr/rr.go -- runRouteReflector OnAllPluginsReady -->
<!-- source: internal/component/bgp/server/events.go -- onPeerStateChange -->

## Plugin Bindings

The `bgp-rs` plugin requires:
- `receive [ update-received state open-received refresh ]` -- the UPDATEs the peer
  sends, its session state, the capabilities in its OPEN, and route refresh
- `send [ update ]` -- sends forwarded routes back to the peer

The received direction is not a preference. A route server that also receives the
UPDATEs ze sends deadlocks: forwarding one raises the sent event that the forward
is waiting on.
<!-- source: internal/component/bgp/plugins/rs/server.go -- SetStartupSubscriptions and the deadlock rationale -->

The `bgp-adj-rib-in` plugin stores received routes for replay on peer reconnect.
<!-- source: internal/component/bgp/plugins/rs/ -- RunRouteServer; internal/component/bgp/plugins/adj_rib_in/ -- adj-rib-in for replay -->

## Cache Commands

Routes in transit can be managed via cache commands:

| Command | Description |
|---------|-------------|
| `show cache` | List cached messages |
| `request cache retain <id>` | Prevent cache eviction |
| `request cache release <id>` | Release a cached message |
| `request cache expire <id>` | Remove a cached message immediately |
| `send bgp <peer> cached <id>` | Forward a cached message to a peer |
<!-- source: internal/component/bgp/plugins/cmd/cache/yang/ze-cli-cache-cmd.yang -- module ze-cli-cache-cmd -->

## Without Route Reflection

When `bgp-rs` is not loaded, received routes are not forwarded to other peers. The `bgp-rib` plugin stores routes and performs best-path selection, but does not re-advertise them. To forward routes between peers, load the `bgp-rs` plugin.
<!-- source: internal/component/bgp/plugins/rib/register.go -- bgp-rib registration -->
