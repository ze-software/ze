# BMP (BGP Monitoring Protocol)

<!-- source: internal/component/bgp/plugins/bmp/bmp.go -- BMPPlugin -->
<!-- source: internal/component/bgp/plugins/bmp/yang/ze-bmp-conf.yang -- YANG config -->

Ze implements RFC 7854 BMP in both directions: as a **receiver** (accepting
feeds from routers) and as a **sender** (streaming state to collectors).

## Configuration

BMP receiver is configured under `environment { bmp { ... } }` (like SSH,
web, looking glass). Sender config lives under `bgp { bmp { ... } }`.

### Receiver

The receiver listens for TCP connections from BMP-enabled routers.
Configured under `environment` to follow ze's service listener pattern.

```
environment {
    bmp {
        enabled true;
        server default {
            ip 0.0.0.0;
            port 11019;
        }
        max-sessions 100;
    }
}
```

<!-- source: internal/component/bgp/plugins/bmp/yang/ze-bmp-conf.yang -- environment container -->

| Field | Default | Description |
|-------|---------|-------------|
| `enabled` | false | Enable BMP receiver |
| `server` | - | Named listener endpoints (key: name) |
| `ip` | 0.0.0.0 | Listen IP address |
| `port` | 11019 | Listen TCP port (IANA assigned for BMP) |
| `max-sessions` | 100 | Maximum concurrent BMP sessions (1-1000) |
| `route-action` | monitor | `monitor` (BMP RIB for visibility) or `redistribute` (future: also enter best-path) |

Multiple listeners are supported (same pattern as SSH/web):

```
environment {
    bmp {
        enabled true;
        server ipv4 {
            ip 0.0.0.0;
            port 11019;
        }
        server ipv6 {
            ip "::";
            port 11019;
        }
    }
}
```

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
            loc-rib true;
            statistics-timeout 0;
        }
    }
}
```

<!-- source: internal/component/bgp/plugins/bmp/yang/ze-bmp-conf.yang -- sender container -->

| Field | Default | Description |
|-------|---------|-------------|
| `collector` | - | Named collector endpoints (key: name) |
| `address` | (required) | Collector IP address |
| `port` | 11019 | Collector TCP port |
| `route-monitoring-policy` | all | `pre-policy` (Adj-RIB-In), `post-policy` (Adj-RIB-Out, RFC 8671), or `all` |
| `loc-rib` | false | Stream local RIB best-path changes as Loc-RIB Route Monitoring (RFC 9069, Peer Type 3) |
| `statistics-timeout` | 0 | Seconds between statistics reports (0 = disabled) |

The sender reconnects automatically with exponential backoff (30s to 720s)
per RFC 7854 recommendations.

## CLI Commands

| Command | Description |
|---------|-------------|
| `ze show bmp sessions` | Show active BMP receiver sessions (router address, sysName, uptime) |
| `ze show bmp peers` | Show monitored BGP peers (AS, BGP ID, up/down status) |
| `ze show bmp collectors` | Show sender collector connection status |

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
- Wraps received BGP UPDATEs as Route Monitoring (pre-policy, Adj-RIB-In)
- Wraps sent BGP UPDATEs as Route Monitoring with O+L flags (post-policy, Adj-RIB-Out, RFC 8671)
- Route-monitoring-policy controls which direction(s) are streamed
- With `loc-rib true`, streams local RIB best-path changes as Loc-RIB Route
  Monitoring (RFC 9069, Peer Type 3): one Loc-RIB Peer Up per RIB instance with
  zero-length OPENs and the local router-id as Peer BGP ID, a full-table dump,
  and a Loc-RIB Peer Down on shutdown

#### Every Connection Is a Fresh BMP Session

A BMP session carries no state across TCP connections, so a collector that
connects (or reconnects after a drop) is told everything again, in this order:

1. Initiation
2. Peer Up for every BGP peer that is currently established
3. With `loc-rib true`: the Loc-RIB Peer Up, a full fresh table dump, and an
   End-of-RIB marker per family closing that dump (RFC 4724 Section 2 form)

Reconnection is not immediate: after a connection ends, ze waits out its
reconnect interval before redialing, so a flapping collector cannot drive a
dump loop.

<!-- source: internal/component/bgp/plugins/bmp/bmp.go -- resyncSender -->
<!-- source: internal/component/bgp/plugins/bmp/bmp_locrib.go -- requestLocRIBDump, sendLocRIBEndOfRIB -->
<!-- source: internal/component/bgp/plugins/bmp/sender.go -- run, onConnected -->

#### A Slow Collector Cannot Stall BGP

Nothing that produces a BMP message writes to the collector socket. Each
message is copied into that collector's transmit queue and written by the
session's own goroutine, so an unresponsive collector costs the BGP RIB a
memory copy rather than a blocking write.

The queue is bounded in bytes (256 MiB per collector, sized to absorb a full
Loc-RIB dump). If a collector stops reading for long enough to fill it, ze
logs `bmp: collector connection stalled, resetting session` and resets the
session with a plain TCP close -- no Termination message, because the session
is being abandoned rather than shut down. The collector's next connection gets
a complete fresh session as described above. Messages are never silently
dropped: either they are delivered, or the session that owed them is reset.

<!-- source: internal/component/bgp/plugins/bmp/txqueue.go -- txQueueLimitBytes, txQueue.push -->
<!-- source: internal/component/bgp/plugins/bmp/sender_drain.go -- enqueueLocked, drainLoop -->

<!-- source: internal/component/bgp/plugins/bmp/bmp_locrib.go -- RFC 9069 Loc-RIB monitoring -->
<!-- source: internal/component/bgp/plugins/bmp/header.go -- PeerTypeLocRIB (Peer Type 3) -->
<!-- source: rfc/short/rfc9069.md -- RFC 9069 requirement summary -->

#### Shutdown

Stopping a collector session sends a Termination message (RFC 7854 Section 4.5)
and then closes the TCP connection. If a socket write is already in flight to a
collector that is not reading, ze gives it one second and then closes anyway
rather than delaying the shutdown of the other collectors.

<!-- source: internal/component/bgp/plugins/bmp/sender.go -- stop, terminateAndClose -->


## Looking Glass Integration

When the BMP receiver is enabled, monitored routes are stored in the BMP RIB
(a separate protocol namespace). These routes are visible through `show bmp rib`
and looking glass endpoints but never enter BGP best-path selection or the FIB.

The `route-action` leaf controls future behavior:
- `monitor` (default): store in BMP RIB for visibility only
- `redistribute`: store in BMP RIB AND redistribute into BGP best-path (not yet implemented)

### CLI

| Command | Description |
|---------|-------------|
| `ze show bmp rib` | Show all BMP-monitored routes |

BMP routes are separate from BGP routes: `ze bgp rib show` excludes
BMP-monitored routes, and `ze show bmp rib` excludes real BGP routes.

### API Endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /api/looking-glass/protocols/bmp` | List BMP-monitored peers |
| `GET /api/looking-glass/routes/bmp/{name}` | Routes from a specific BMP peer |

The `{name}` parameter is the composite peer key in `<router>:<peer-address>`
format (e.g., `10.0.0.1:12345:192.168.1.1`).

Responses follow the birdwatcher format for compatibility with Alice-LG and
other looking glass frontends.

### Route Lifecycle

- **Injection:** Route Monitoring messages inject BGP UPDATE routes under
  `bmpProtocolID` with composite keys `<router>:<peer-address>`.
- **Peer Down:** All routes for the monitored peer are withdrawn.
- **Session disconnect:** All routes for all peers of that router are withdrawn.
- **Best-path isolation:** BMP routes are stored under a separate ProtocolID.
  The best-path algorithm only iterates BGP peers, so BMP routes are
  automatically excluded with zero filter code.

## Limitations

- **Sender OPEN messages are synthetic:** the plugin event system does not
  carry raw BGP OPEN PDUs. Peer Up messages contain minimal OPENs built from
  AS metadata. Capabilities are not reflected. This can be improved when the
  event schema is extended.
- **No per-NLRI ribout dedup:** all UPDATEs are forwarded to collectors
  as-is. Per-NLRI dedup requires parsing NLRIs from the raw UPDATE,
  which is a follow-up task.
- **Loc-RIB Route Monitoring** (RFC 9069) omits communities and LOCAL_PREF:
  the best-change feed it is built from does not carry them, and RFC 9069
  forbids a RIB back-door for the full attribute set.
- **Route Mirroring** encoding on the sender side is not implemented.
