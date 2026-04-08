# Graceful Restart

Graceful Restart (RFC 4724) preserves forwarding state across BGP session restarts. When a peer goes down and comes back, routes are held during the restart window instead of being immediately withdrawn, preventing traffic black-holes.
<!-- source: internal/component/bgp/plugins/gr/register.go -- bgp-gr registration, RFCs 4724/9494 -->

## Configuration

```
plugin {
    external gr {
        use bgp-gr
        encoder json
    }
    external rib {
        use bgp-rib
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
<!-- source: internal/component/bgp/plugins/gr/schema/ -- ze-graceful-restart YANG schema -->

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
<!-- source: internal/component/bgp/plugins/gr/ -- GR state machine, retain-routes/purge-stale commands -->

## Plugin Bindings

The GR plugin requires:
- `receive [ state eor ]` -- needs peer up/down events and End-of-RIB markers
- The RIB plugin must also be loaded with `receive [ state ]` and `send [ update ]`

The GR plugin depends on `bgp-rib` (declared in its registration). The engine ensures bgp-rib starts first.
<!-- source: internal/component/bgp/plugins/gr/register.go -- Dependencies: bgp-rib -->

## CLI

```
$ ze cli -c "rib routes received"
# Shows routes with stale flag when applicable
```

## Long-Lived Graceful Restart (RFC 9494)

LLGR extends standard GR with a second, much longer stale period. When the GR restart-time expires without the peer reconnecting, instead of purging all stale routes, LLGR keeps them for up to ~194 days (per-family configurable) with reduced priority.

### Configuration

Add `long-lived-stale-time` under the `graceful-restart` block:

```
capability {
    graceful-restart {
        restart-time 120;
        long-lived-stale-time 3600;    # LLGR period in seconds (0-16777215)
    }
}
```

| Path | Type | Default | Description |
|------|------|---------|-------------|
| `graceful-restart / long-lived-stale-time` | uint32 | -- | Seconds to hold LLGR-stale routes per family (0-16777215, 24-bit) |

LLGR is only active when both peers negotiate it. Ze advertises LLGR capability (code 71) in OPEN when `long-lived-stale-time` is configured. LLGR requires GR capability (code 64) to also be present -- LLGR without GR is ignored per RFC 9494.
<!-- source: internal/component/bgp/plugins/gr/register.go -- CapabilityCodes: 64, 71 -->

### How It Works

1. **GR period expires** -- peer has not reconnected within `restart-time` seconds
2. **LLGR begins** -- for each family with `long-lived-stale-time > 0`:
   - Routes carrying the NO_LLGR community (0xFFFF0007) are deleted
   - LLGR_STALE community (0xFFFF0006) is attached to remaining stale routes
   - Routes are marked as stale level 2 (deprioritized in best-path selection)
   - Per-family LLST timer starts
3. **During LLGR** -- LLGR-stale routes lose to any non-stale route in best-path selection. Between two LLGR-stale routes, normal tiebreaking applies.
4. **LLST timer expires** -- stale routes for that family are purged
5. **Peer reconnects during LLGR** -- standard RFC 4724 procedures apply; families with F-bit=0 or missing from the new OPEN are purged

### Stale Levels

Ze uses a graduated stale level system for route prioritization:

| Level | Meaning | Best-path behavior |
|-------|---------|-------------------|
| 0 | Fresh | Normal selection |
| 1 | GR-stale | Normal selection (not deprioritized) |
| 2+ | LLGR-stale | Loses to any route with level < 2 |

### Special Case: Skip GR

If `restart-time` is 0 but `long-lived-stale-time` is nonzero, the GR period is skipped entirely. On session drop, LLGR begins immediately.

| restart-time | long-lived-stale-time | Behavior |
|-------------|----------------------|----------|
| 0 | nonzero | Skip GR, enter LLGR immediately |
| nonzero | 0 | GR only, no LLGR |
| 0 | 0 | Neither GR nor LLGR |
| nonzero | nonzero | GR then LLGR (serial) |

### Well-Known Communities

| Community | Value | Purpose |
|-----------|-------|---------|
| LLGR_STALE | 0xFFFF0006 | Attached to stale routes during LLGR period |
| NO_LLGR | 0xFFFF0007 | Routes with this community are deleted on LLGR entry |
<!-- source: internal/component/bgp/plugins/gr/register.go -- CommunityLLGRStale, CommunityNoLLGR -->

### CLI

Decode LLGR capability from hex:

```
$ ze plugin bgp-gr --capa 00010180000e10
```

Shows per-family LLST values and F-bit flags.

## Without Graceful Restart

When GR is not configured or the peer does not advertise the GR capability, routes are withdrawn immediately on session down. No stale state, no restart timer.
<!-- source: internal/component/bgp/plugins/gr/ -- GR plugin implementation -->
