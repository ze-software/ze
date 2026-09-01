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
| `source-address` | - | Source IP address for outbound BMP connections |
| `route-monitoring-policy` | all | `pre-policy` (Adj-RIB-In), `post-policy` (Adj-RIB-Out, RFC 8671), or `all` |
| `route-mirroring` | false | Stream verbatim copies of every BGP message as Route Mirroring (RFC 7854 Section 4.7) |
| `loc-rib` | false | Stream local RIB best-path changes as Loc-RIB Route Monitoring (RFC 9069, Peer Type 3) |
| `statistics-timeout` | 0 | Seconds between statistics reports. Read by nothing today: the sender has no statistics timer |

The sender reconnects automatically with exponential backoff (30s to 720s)
per RFC 7854 recommendations.

## CLI Commands

| Command | Description |
|---------|-------------|
| `ze show bmp sessions` | Show active BMP receiver sessions (router address, sysName, uptime) |
| `ze show bmp peers` | Show monitored BGP peers (AS, BGP ID, up/down status, and the address families their Peer Up OPEN advertised) |
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
| Statistics Report (1) | Stores per-peer counters | Encoder only, with the O flag cleared (RFC 8671 Section 6.2); no timer sends one yet |
| Route Mirroring (6) | Logs raw BGP PDUs | Wraps every BGP PDU when `route-mirroring` is on |

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
- With `route-mirroring true`, wraps every BGP message (OPEN, UPDATE, NOTIFICATION,
  KEEPALIVE, ROUTE-REFRESH, both directions) as Route Mirroring (RFC 7854 Section 4.7).
  The O flag follows the direction, as it does for Route Monitoring
- Clears the O flag on a Statistics Report, which RFC 8671 Section 6.2 requires
  because the report belongs to neither RIB
- Route-monitoring-policy controls which direction(s) are streamed
- With `loc-rib true`, streams local RIB best-path changes as Loc-RIB Route
  Monitoring (RFC 9069, Peer Type 3): one Loc-RIB Peer Up per RIB instance
  carrying a fabricated BGP OPEN in both the sent and the received field, the
  router's own 4-octet ASN as Peer AS and the local router-id as Peer BGP ID, a
  full-table dump, and a Loc-RIB Peer Down with reason code 6 (RFC 9069
  Section 5.3) on shutdown and on the commit that turns `loc-rib` off. RFC 9069
  Section 5.2 requires that OPEN and its capabilities: "This is a fabricated BGP
  OPEN message. Capabilities MUST include the 4-octet ASN and all necessary
  capabilities to represent the Loc-RIB Route Monitoring messages." Ze
  advertises the 4-octet ASN capability and one address-family capability per
  family the dump delivers, and nothing else
- Names that Loc-RIB `global` in a VRF/Table Name Information TLV (type 3) on
  the Peer Up, and repeats the TLV after reason code 6 on the Peer Down. RFC
  9069 Section 5.2.1: "The default value of "global" MUST be used for the
  default Loc-RIB instance with a zero-filled distinguisher", and Section 5.3:
  "The VRF/Table Name informational TLV MUST be included if it was in the Peer
  Up." Ze runs one Loc-RIB, the default instance, and its distinguisher is zero
- Timestamps a Loc-RIB message with the time its routes entered the Loc-RIB, and
  with ZERO where that time is unknown. RFC 9069 Section 5.1: "If zero, the time
  is unavailable." An incremental best change is delivered on the goroutine that
  installed it, so it carries a real time; the initial full-table dump, the Peer
  Up, the Peer Down and the End-of-RIB marker each carry zero rather than a
  wall-clock read that would date every replayed route to the collector's
  connection

<!-- source: internal/component/bgp/plugins/bmp/bmp_locrib.go -- locRIBPeerHeader, fabricateLocRIBOpen, ensureLocRIBPeerUp, sendLocRIBPeerDown -->

#### A Config Change Bounces the Peers, Not the Session

RFC 8671 Section 7.2 says a change that alters the behavior of an existing BMP
session MUST bounce that session with a Peer Down and Peer Up sequence. Ze
bounces the peers inside the session and leaves the session itself up: each
established BGP peer gets a Peer Down with reason 5 (configuration reasons)
followed by a Peer Up, so the collector re-learns that peer under the new
configuration. The TCP connection is not closed and no Termination is sent, so
the collector keeps everything the change did not touch.

Ze acts on a change, not on a commit. Four leaves decide what a collector
session carries, and only a move in one of them bounces the peers:

| Leaf | Why it alters the session |
|------|---------------------------|
| `route-monitoring-policy` | decides which direction is streamed |
| `route-mirroring` | decides whether verbatim BGP messages are streamed |
| `loc-rib` | decides whether the Loc-RIB feed is streamed |
| `statistics-timeout` | configures the Statistics Report interval |

A commit that changes anything else under `bgp`, a new neighbor for example,
leaves every collector session untouched. So does a commit that changes nothing
under `bgp`.

The collector list is separate, because changing it changes which sessions exist
rather than what one of them carries. A collector you remove, or point at another
address, is sent a Termination and its TCP connection is closed: that session
ends rather than continues. A collector you add gets a new session, with
Initiation and a Peer Up for every established peer. A collector you leave alone
keeps its session, even when you edit another collector beside it.

One consequence for an operator: a behavior change costs one Peer Down and one
Peer Up per peer on each collector, rather than a full re-dump of the table.

<!-- source: internal/component/bgp/plugins/bmp/sender_config.go -- applySenderConfig, behaviorOf, syncSenders -->
<!-- source: internal/component/bgp/plugins/bmp/bmp_events.go -- bounceMonitoredPeers -->
<!-- source: internal/component/bgp/plugins/bmp/yang/ze-bmp-conf.yang -- the four sender behavior leaves -->

Ze sends no periodic Statistics Report yet, so `statistics-timeout` bounces the
peers but changes nothing else (`plan/journal/unwired-feature.md`, 2026-08-31).

#### Every Connection Is a Fresh BMP Session

A BMP session carries no state across TCP connections, so a collector that
connects (or reconnects after a drop) is told everything again, in this order:

1. Initiation
2. Peer Up for every BGP peer that is currently established
3. With `loc-rib true`: the Loc-RIB Peer Up, a full fresh table dump, and an
   End-of-RIB marker for every family the dump OWES (RFC 4724 Section 2 form),
   which is IPv4 unicast and IPv6 unicast. A family the dump carried no route for
   gets its marker too, so a table with IPv6 populated and IPv4 empty closes
   both. RFC 4724 Section 4 requires the marker "including the case when there is
   no update to send" for an address family, and RFC 7854 Section 5 imports that
   definition for the BMP dump. A Loc-RIB with no best paths at all therefore
   still gets both markers, so a collector can tell an empty table from a dump
   still in flight.

Each dump carries a correlation token, and the RIB echoes it back on every batch
that dump produces. Ze closes a family only for a batch whose token matches, so
two collectors that connect together each get a complete dump of their own, and
neither is told that a dump it never requested has finished. A replay that
another subsystem asks for (sysrib emits one on the same handle) still reaches
every collector as Route Monitoring, because those routes are real, but it closes
no family.

<!-- source: internal/component/bgp/plugins/bmp/bmp_locrib.go -- emitReplayRequest, handleBestChange, dumpFamilies -->

Reconnection is not immediate: after a connection ends, ze waits out its
reconnect interval before redialing, so a flapping collector cannot drive a
dump loop.

<!-- source: internal/component/bgp/plugins/bmp/bmp_events.go -- primeSender -->
<!-- source: internal/component/bgp/plugins/bmp/bmp_locrib.go -- requestLocRIBDump, sendLocRIBEndOfRIB, closeDumpFamilies -->
<!-- source: internal/component/bgp/plugins/bmp/sender.go -- run, onConnected -->

#### A Slow Collector Cannot Stall BGP

Nothing that produces a BMP message writes to the collector socket. Each
message is copied into that collector's transmit queue and written by the
session's own goroutine, so an unresponsive collector costs the BGP RIB a
memory copy rather than a blocking write.

The queue is bounded in bytes (256 MiB per collector, sized to absorb a full
Loc-RIB dump). The bound is not what catches a collector that stops reading
outright: that one is caught in seconds by the drain's per-write deadline,
long before 256 MiB accumulates. The bound bites for the other shape -- a
collector that keeps reading, but steadily slower than ze produces -- where
every individual write succeeds and it is the backlog that grows. In either
case ze logs `bmp: collector connection stalled, resetting session` and resets
the session with a plain TCP close -- no Termination message, because the
session is being abandoned rather than shut down. The collector's next connection gets
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
<!-- source: internal/component/bgp/plugins/bmp/bmp_events.go -- handleSenderMirror, peerHeaderFromEvent -->


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
