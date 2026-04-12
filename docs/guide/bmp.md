# BMP (BGP Monitoring Protocol)

<!-- source: internal/component/bgp/plugins/bmp/bmp.go -- BMPPlugin -->
<!-- source: internal/component/bgp/plugins/bmp/schema/ze-bmp-conf.yang -- YANG config -->

Ze implements RFC 7854 BMP in both directions: as a **receiver** (accepting
feeds from routers) and as a **sender** (streaming state to collectors).

## Configuration

BMP is configured under the `bgp { bmp { ... } }` block.

### Receiver

The receiver listens for TCP connections from BMP-enabled routers.

```
bgp {
    bmp {
        receiver {
            server default {
                ip 0.0.0.0;
                port 11019;
            }
            max-sessions 100;
        }
    }
}
```

<!-- source: internal/component/bgp/plugins/bmp/schema/ze-bmp-conf.yang -- receiver container -->

| Field | Default | Description |
|-------|---------|-------------|
| `server` | - | Named listener endpoints (key: name) |
| `ip` | 0.0.0.0 | Listen IP address |
| `port` | 11019 | Listen TCP port (IANA assigned for BMP) |
| `max-sessions` | 100 | Maximum concurrent BMP sessions (1-1000) |

Port conflicts with other ze listeners are detected at config commit time
via the YANG `ze:listener` extension.

### Sender

The sender connects to one or more external BMP collectors and streams
ze's own BGP peer state changes and route updates.

```
bgp {
    bmp {
        sender {
            collector monitoring-station {
                address 10.0.0.100;
                port 11019;
            }
            route-monitoring-policy pre-policy;
            statistics-timeout 0;
        }
    }
}
```

<!-- source: internal/component/bgp/plugins/bmp/schema/ze-bmp-conf.yang -- sender container -->

| Field | Default | Description |
|-------|---------|-------------|
| `collector` | - | Named collector endpoints (key: name) |
| `address` | (required) | Collector IP address |
| `port` | 11019 | Collector TCP port |
| `route-monitoring-policy` | pre-policy | `pre-policy` or `all` |
| `statistics-timeout` | 0 | Seconds between statistics reports (0 = disabled) |

The sender reconnects automatically with exponential backoff (30s to 720s)
per RFC 7854 recommendations.

## CLI Commands

| Command | Description |
|---------|-------------|
| `ze bmp sessions` | Show active BMP receiver sessions (router address, sysName, uptime) |
| `ze bmp peers` | Show monitored BGP peers (AS, BGP ID, up/down status) |
| `ze bmp collectors` | Show sender collector connection status |

## Protocol Details

### Message Types

Ze handles all 7 BMP message types defined in RFC 7854:

| Type | Receiver | Sender |
|------|----------|--------|
| Initiation (4) | Parses sysName/sysDescr | Sends ze identity on connect |
| Termination (5) | Closes session cleanly | Sends before disconnect |
| Peer Up (3) | Tracks monitored peer | Sends on BGP Established |
| Peer Down (2) | Marks peer down | Sends on BGP session close |
| Route Monitoring (0) | Decodes inner BGP UPDATE | Wraps received UPDATEs |
| Statistics Report (1) | Stores per-peer counters | Periodic (if configured) |
| Route Mirroring (6) | Logs raw BGP PDUs | Not implemented (follow-up) |

### Receiver Behavior

- Validates BMP version 3; rejects other versions
- Malformed BMP header closes the session; other sessions unaffected
- Malformed inner BGP messages are logged; session stays open
- Session count capped at `max-sessions`
- 30-second read deadline ensures clean shutdown

### Sender Behavior

- Sends Initiation with sysName="ze" on each connection
- Sends Peer Up for each BGP peer reaching Established state
- Sends Peer Down with mapped reason code on session close
- Wraps received BGP UPDATEs as Route Monitoring messages
- Sends Termination before graceful disconnect

## Limitations

- **Sender OPEN messages are synthetic:** the plugin event system does not
  carry raw BGP OPEN PDUs. Peer Up messages contain minimal OPENs built from
  AS metadata. Capabilities are not reflected. This can be improved when the
  event schema is extended.
- **No per-NLRI ribout dedup:** all received UPDATEs are forwarded to
  collectors as-is. Per-NLRI dedup requires parsing NLRIs from the raw
  UPDATE, which is a follow-up task.
- **Adj-RIB-Out** (RFC 8671) and **Loc-RIB** (RFC 9069) monitoring are not
  yet implemented.
- **Route Mirroring** encoding on the sender side is not implemented.
