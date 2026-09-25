# BMP (BGP Monitoring Protocol)

<!-- source: internal/component/bgp/plugins/bmp/bmp.go -- BMPPlugin -->
<!-- source: internal/component/bgp/plugins/bmp/yang/ze-bmp-conf.yang -- YANG config -->

Ze implements RFC 7854 BMP in both directions: as a **receiver** (accepting
feeds from routers) and as a **sender** (streaming state to collectors).

Both halves follow a reload. A commit that adds, moves or disables a receiver
listener rebinds it, and one that changes the sender bounces the collector
sessions, so neither reports success over a change it did not make. A Peer Down
carries the Data its reason code requires: RFC 7854 Section 4.9 draws the field
as present for reasons 1, 2 and 3, so a session ze closed by NOTIFICATION
carries that PDU and one closed without carries the two-octet FSM event code.

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

BMP uses unauthenticated, unencrypted TCP. Listener addresses and firewall rules
should restrict access to the monitoring network. RFC 7854 Section 11 recommends
IPsec tunnel protection where transport security is needed; BMP configuration
does not itself configure IPsec.

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

Load the `bgp-bmp` internal plugin and attach it to each monitored peer. The
sender configuration selects collectors; the attachment supplies the OPEN,
session-state and UPDATE events from which the sender builds its feed:

```
plugin {
    internal bgp-bmp {
        use bgp-bmp;
    }
}
bgp {
    peer monitored-peer {
        attach process bgp-bmp {
            receive [ state update open notification keepalive refresh ]
        }
    }
}
```

The peer still needs its normal connection and session configuration. Attach
BMP before establishing that BGP session, because replay depends on observing
the OPEN exchange and subsequent updates. Adding the attachment to an already
established peer requires that BGP peer to reconnect before it can be monitored.

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
| `statistics-timeout` | 0 | Seconds between Statistics Reports (RFC 7854 Section 4.8). 0 sends none |

The sender reconnects automatically with exponential backoff (30s to 720s)
per RFC 7854 recommendations.

## CLI Commands

| Command | Description |
|---------|-------------|
| `ze show bmp sessions` | Show receiver sessions, identity, uptime and ordered `initiation-strings` |
| `ze show bmp peers` | Show monitored peers, OPEN address families and ordered information strings |
| `ze show bmp collectors` | Show sender collector connection status |

## Protocol Details

### Message Types

Ze handles all 7 BMP message types defined in RFC 7854:

| Type | Receiver | Sender |
|------|----------|--------|
| Initiation (4) | Requires sysName/sysDescr and preserves information string order | Sends system identity on connect |
| Termination (5) | Requires a two-byte Reason and closes the session | Sends administrative Reason 0 before disconnect |
| Peer Up (3) | Tracks monitored peer | Sends on BGP Established |
| Peer Down (2) | Marks peer down | Sends on BGP session close |
| Route Monitoring (0) | Decodes inner BGP UPDATE | Wraps received UPDATEs |
| Statistics Report (1) | Validates framing and logs peer/count metadata | Sends one per established peer every `statistics-timeout` seconds, with the O flag cleared (RFC 8671 Section 6.2) |
| Route Mirroring (6) | Validates TLVs and logs peer/count metadata | Wraps every BGP PDU when `route-mirroring` is on |

### Receiver Behavior

- Validates BMP version 3; rejects other versions
- Malformed BMP headers, enclosing lengths or required body fields close that session
- Malformed inner BGP messages are logged; session stays open
- Session count capped at `max-sessions`
- 30-second read deadline ensures clean shutdown
- Requires both Initiation identity TLVs, including when their values are empty
- Requires a two-byte Termination Reason
- Requires the BGP Message TLV to be last in Route Mirroring and present with Information code 0 (Errored PDU)
- Requires a nonempty Statistics Report whose count matches its TLVs; unknown stat types and unexpected stat values are ignored
- Reports Information strings in received order, including duplicates
- Keeps separate Loc-RIB peer reports for each distinguisher and BGP ID, so an old identity's Peer Down cannot mark its replacement down

### Sender Behavior

- Sends Initiation with the system identity on each connection
- Sends Peer Up with the cached sent and received OPEN messages for every established BGP peer, followed by its current route snapshot and per-family End-of-RIB
- Reports the actual established TCP local and remote ports in Peer Up, including the active opener's ephemeral port
- Sends Peer Down with the established peer's identity and mapped reason code on session close, even after BGP has cleared the negotiated router ID
- Wraps received BGP UPDATEs as Route Monitoring (pre-policy, Adj-RIB-In)
- Wraps sent BGP UPDATEs as Route Monitoring with O+L flags (post-policy, Adj-RIB-Out, RFC 8671)
- With `route-mirroring true`, wraps every BGP message (OPEN, UPDATE, NOTIFICATION,
  KEEPALIVE, ROUTE-REFRESH, both directions) as Route Mirroring (RFC 7854 Section 4.7).
  The O flag follows the direction, as it does for Route Monitoring
- With `statistics-timeout` set to a nonzero number of seconds, sends one
  Statistics Report per established BGP peer, to every collector, at that
  interval. Each report carries RFC 7854 Section 4.8 Stat Type 13, "Number of
  duplicate update messages received", as a 4-byte counter. That is the one
  statistic ze measures: repeated received UPDATE bodies count as duplicates
  while the peer's route state is unchanged. An attribute change followed by a
  return to an earlier value is a new update. Sent bodies do not increment the
  counter. Peer Down and a configuration bounce reset it
- Clears the O flag on a Statistics Report, which RFC 8671 Section 6.2 requires
  because the report belongs to neither RIB
- Route-monitoring-policy controls which direction(s) are streamed
- Preserves a route's recorded receipt timestamp on replay; unavailable timestamps and snapshot completion markers carry zero
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
- Reads that ASN and that router-id from `bgp session asn local` and `bgp
  router-id`, and from nowhere else. Both leaves are required, so a configured
  ze always knows its own identity, and it knows it before any BGP peer comes
  up. That is the case RFC 9069 Section 1.1 exists to serve, a Loc-RIB monitored
  where there are "no preexisting BGP peers": a router that read its identity
  off an established session would have none to read. Ze sends no Loc-RIB
  message at all while the identity is unknown, and logs why, rather than
  sending a Peer AS of 0 from router 0.0.0.0
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

#### System Identity

`sysName` uses `system.host`, or the operating-system hostname when that leaf is
absent. `system.domain` qualifies an unqualified, nonempty name. An explicitly
empty name remains empty, as permitted by the MIB-II definition. `sysDescr`
contains the Ze version and the operating system name, release and hardware
architecture returned by `uname`. These are Ze's MIB-II identity values; Ze has
no separate SNMP provider.

Identity values must fit MIB-II's 255-byte ASCII strings. A source or validation
error prevents the Initiation from being sent and is logged. A system identity
configuration change takes effect on the next collector connection; it does not
interrupt existing sessions.

<!-- source: internal/component/bgp/plugins/bmp/system_identity.go -- readSystemIdentity -->

#### A Config Change Bounces the Peers, Not the Session

RFC 8671 Section 7.2 says a change that alters the behavior of an existing BMP
session MUST bounce that session with a Peer Down and Peer Up sequence. Ze
bounces the peers inside the session and leaves the session itself up: each
established BGP peer gets a Peer Down with reason 5 (configuration reasons)
followed by a Peer Up and a current route snapshot with per-family End-of-RIB.
The collector therefore relearns the peer under the new configuration. The TCP
connection stays open and no Termination is sent.

Ze acts on a change, not on a commit. Four leaves decide what a collector
session carries, and only a move in one of them bounces the peers:

| Leaf | Why it alters the session |
|------|---------------------------|
| `route-monitoring-policy` | decides which direction is streamed |
| `route-mirroring` | decides whether verbatim BGP messages are streamed |
| `loc-rib` | decides whether the Loc-RIB feed is streamed |
| `statistics-timeout` | decides whether a periodic Statistics Report is sent, and how often |

A commit that changes anything else under `bgp`, a new neighbor for example,
leaves every collector session untouched. So does a commit that changes nothing
under `bgp`.

The router's own identity is the one exception, and it bounces the Loc-RIB
emulated peer alone. A commit that moves `bgp router-id` or `bgp session asn
local` while `loc-rib` is on sends that peer a Peer Down with reason 6 under the
OLD identity, then a Peer Up under the new one. RFC 9069 Section 6.1.1 has the
collector identify the Loc-RIB "by the peer header distinguisher and BGP ID", so
a new BGP ID is a new peer to it and Section 6.1.3 owes it the bounce. The
monitored BGP peers are not bounced: their per-peer headers carry their own
addresses and AS numbers, which the router's identity does not appear in.

The collector list is separate, because changing it changes which sessions exist
rather than what one of them carries. A collector you remove, or point at another
address, is sent a Termination and its TCP connection is closed: that session
ends rather than continues. A collector you add gets a new session, with
Initiation and a Peer Up for every established peer. A collector you leave alone
keeps its session, even when you edit another collector beside it.

A behavior change therefore replays each monitored peer's selected Adj-RIB
snapshot after its Peer Down and Peer Up. A collector endpoint change starts a
new TCP session and replays the state selected for that session.

<!-- source: internal/component/bgp/plugins/bmp/sender_config.go -- applySenderConfig, behaviorOf, syncSenders -->
<!-- source: internal/component/bgp/plugins/bmp/bmp_events.go -- bounceMonitoredPeers -->
<!-- source: internal/component/bgp/plugins/bmp/yang/ze-bmp-conf.yang -- the four sender behavior leaves -->

A move in `statistics-timeout` also replaces the report timer: exactly one
report stream is live per configuration, so a collector never reads two streams
at two intervals. Setting it to 0 stops the reports and leaves the session up.

<!-- source: internal/component/bgp/plugins/bmp/statistics.go -- setStatisticsTimeout, statisticsLoop, sendStatisticsReports -->

The RFC 9069 Loc-RIB emulated peer gets no Statistics Report. RFC 9069
Section 5.6 names the two stat types relevant to a Loc-RIB, 8 and 10, and both
count routes in the Loc-RIB itself, which the BMP plugin does not hold.

#### Every Connection Is a Fresh BMP Session

A BMP session carries no state across TCP connections, so a collector that
connects (or reconnects after a drop) is told everything again, in this order:

1. Initiation
2. Peer Up for every BGP peer that is currently established
3. The current Adj-RIB routes selected by `route-monitoring-policy`, followed by
   End-of-RIB for each peer, selected direction and address family. Withdrawn
   routes and superseded attributes are absent. Empty negotiated families get
   their completion marker too.
4. With `loc-rib true`: the Loc-RIB Peer Up, a full fresh table dump, and an
   End-of-RIB marker for every family the dump OWES (RFC 4724 Section 2 form),
   which is IPv4 unicast and IPv6 unicast. A family the dump carried no route for
   gets its marker too, so a table with IPv6 populated and IPv4 empty closes
   both. RFC 4724 Section 4 requires the marker "including the case when there is
   no update to send" for an address family, and RFC 7854 Section 5 imports that
   definition for the BMP dump. A Loc-RIB with no best paths at all therefore
   still gets both markers, so a collector can tell an empty table from a dump
   still in flight.

The sender maintains the current per-NLRI state even while collectors are
disconnected. ADD-PATH identifiers distinguish paths, and family-specific keys
match labelled withdrawals to their advertisements. Snapshot enqueueing and
live peer events share one ordering lock, so an old snapshot cannot follow a
new withdrawal on the same collector session.

The shared snapshot store allows up to two million paths and 512 MiB of retained
wire data and prefix keys. A storage or parsing failure closes collector sessions
without sending a misleading End-of-RIB. The affected BGP peer must reconnect to
rebuild its complete state before that peer can be replayed.

<!-- source: internal/component/bgp/plugins/bmp/bmp_replay.go -- cacheAdjUpdate, replayPeerLocked -->

Each Loc-RIB dump carries a correlation token, and the RIB echoes it on every batch
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

Stopping a collector session sends a Termination message with the required
Reason TLV, code 0 (administrative closure), and then closes the TCP connection.
If a socket write is already in flight to a
collector that is not reading, ze gives it one second and then closes anyway
rather than delaying the shutdown of the other collectors.

<!-- source: internal/component/bgp/plugins/bmp/sender.go -- stop, terminateAndClose -->
<!-- source: internal/component/bgp/plugins/bmp/bmp_events.go -- handleSenderMirror, peerHeaderFromEvent -->

### Checking the Live Feed

The existing independent-collector scenario runs FRR as the BGP source and
pmacct `pmbmpd` as the BMP collector:

```
INTEROP_SCENARIO=bmp-statistics-pmacct ./le test integration interop
```

Its automated assertions cover periodic Statistics Reports decoded by pmacct.
They do not establish reconnect replay. For that check, the same
`test/interop/scenarios/bmp-statistics-pmacct/` configuration provides a starting
point for a separately managed lab, with `pmbmpd -f <collector-config>` recording
its decoded messages in `/var/log/pmacct/bmp.log`:

1. Set `route-monitoring-policy pre-policy`, establish FRR's BGP session and
   advertise `10.45.0.0/24` while the collector is stopped. Start `pmbmpd` and
   wait for Ze's reconnect interval. The new
   connection must begin with Initiation and Peer Up, then carry that prefix
   before the IPv4 End-of-RIB. An empty UPDATE alone is insufficient evidence.
2. Stop only the collector. Withdraw `10.45.0.0/24` in FRR and advertise
   `10.46.0.0/24`, then restart the collector. Its new snapshot must contain
   `10.46.0.0/24` and omit the withdrawn prefix. A packet capture decoded as BMP
   can confirm the End-of-RIB ordering if the collector omits completion markers
   from its message log.
3. Commit a change from `pre-policy` to `all` while the connection stays up.
   Observe Peer Down reason 5, Peer Up and a fresh snapshot with a completion
   marker for each selected direction. The received stream has O clear and the
   sent stream has O+L set. A Termination would indicate an incorrect transport
   restart.
   Compare Peer Up's local and remote ports with the established BGP socket or
   its packet capture; the active opener's port must not be replaced with 179.
4. Close FRR's BGP session and check that Peer Down carries the same Peer AS,
   address and BGP ID as its Peer Up. Reconnect the collector while BGP remains
   down; neither that peer nor its routes may reappear.
5. Re-establish BGP, change `system.host`, and reconnect the collector. The new
   Initiation must carry the changed name while `sysDescr` identifies the running
   Ze build and kernel. Removing the collector from configuration must send
   Termination with the two-byte administrative Reason 0 before TCP closes.

For dual-stack or ADD-PATH peers, repeat with an empty negotiated family and
with two path identifiers for one prefix. Each selected family must complete,
and withdrawing one path must leave only the other in the reconnect snapshot.

<!-- source: test/interop/scenarios/bmp-statistics-pmacct/ze.conf -- peer attachment and collector -->
<!-- source: test/interop/scenarios/bmp-statistics-pmacct/pmbmpd.conf -- independent message log -->
<!-- source: internal/le/interoplab/bgp/check_extras.go -- scenarioStatisticsPMACCT -->


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

- **Loc-RIB Route Monitoring** (RFC 9069) omits communities and LOCAL_PREF:
  the best-change feed it is built from does not carry them, and RFC 9069
  forbids a RIB back-door for the full attribute set.
