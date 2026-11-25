# Graceful Restart

Graceful Restart (RFC 4724) preserves forwarding state across BGP session restarts. When a peer goes down and comes back, routes are held during the restart window instead of being immediately withdrawn, preventing traffic black-holes.
<!-- source: internal/component/bgp/plugins/gr/register.go -- bgp-gr registration, RFCs 4724/9494 -->

## Configuration

```
bus {
    endpoint gr  { internal bgp-gr; }
    endpoint rib { internal bgp-rib; }
}

bgp {
    peer upstream1 {
        remote { ip 10.0.0.1; as 65001; }
        ...
        capability {
            graceful-restart {
                short-lived {
                    restart-time 120;
                }
            }
        }
    }
}
```

### Config Reference

| Path | Type | Default | Description |
|------|------|---------|-------------|
| `graceful-restart / short-lived / restart-time` | uint16 | 120 | Seconds to hold stale routes during conventional GR (0-4095); omitting `short-lived` sends code 64 with Restart Time 0 and no family tuples |
| `graceful-restart / short-lived / family` | list (`name`) | -- | Address families code 64 names when `short-lived` is present. Absent: every family the peer carries; present: only the listed families |
| `graceful-restart / long-lived / family / stale-time` | uint32 | -- | Per-family Long-Lived Stale Time in seconds (0-16777215, mandatory per entry) advertised in code 71 |
<!-- source: internal/component/bgp/plugins/gr/yang/ -- ze-graceful-restart YANG schema -->

## How It Works

### What Ze Advertises

The code-64 capability Ze sends carries the Restart Time, then one
`<AFI, SAFI, Flags>` tuple for each address family the peer carries. A peer
configured for `ipv4/unicast` and `ipv6/unicast` with the default restart time
receives `0078 00010100 00020100`. The family list defaults to the peer's own
`family` configuration (narrowed when `short-lived / family` is written);
code 71 (LLGR) is configured separately under `long-lived / family`. RFC 9494
Section 4.2 reads a family that code 64 omits as a Restart Time of zero.

A tuple tells the peer that Ze can preserve that family across a restart, and
what Ze keeps is the RIB plugin's copy of the routes, which it sends again once
the session is back. RFC 4724 Section 4: "A BGP speaker MAY advertise the
Graceful Restart Capability for an address family to its peer if it has the
ability to preserve its forwarding state for the address family when BGP
restarts." So code 64 lists a family only when the RIB plugin can store it,
which is when an NLRI splitter frames that family. `bgp-gr` always loads
`bgp-rib`, and every family compiled into Ze has a splitter, so the check
removes nothing from a stock build. A family an external plugin registers at
runtime with no splitter is left out, and Ze logs a warning naming the peer and
the family. When no family is left, Ze still sends the capability, with no
tuple.

The `graceful-restart short-lived family` list narrows code 64 alone:

```
capability {
    graceful-restart {
        short-lived {
            restart-time 120;
            family {
                ipv6/unicast
            }
        }
    }
}
```

That peer receives `0078 00020100`, whatever else its `family` configuration
carries. Omitting `short-lived` altogether (writing `graceful-restart { }` or
configuring only `long-lived { ... }`) makes Ze send code 64 with Restart Time
0 and no address family tuple (`0000`), which RFC 4724 Section 3 reads as a
speaker that runs the Receiving Speaker procedures and preserves nothing of its
own, and RFC 9494 Section 4.1 uses to skip the conventional GR phase. Writing
`short-lived` with no `family` list is the default for conventional GR, naming
every family the peer carries. The list only narrows: a family the RIB plugin
cannot store is left out with the same warning, never added.

A family under `short-lived family` or `long-lived family` that the peer's own
`family` list does not carry is refused, because RFC 4724 Section 3 and RFC
9494 Section 3.1 scope a tuple to routes "advertised with the same AFI and
SAFI" and the session carries none of them. Ze names the family in the error.
The refusal runs at offline `ze config validate`, at `config commit`, and at
startup rather than leaving the daemon running without Graceful Restart.

The families the peer carries include the ones it inherits from its group, and
exclude any entry written `mode disable`. Both are what the session really
negotiates.

Omitting `short-lived` while configuring `long-lived` is how an operator asks
for LLGR with no conventional GR phase. RFC 9494 Section 4.1: "the conventional
GR phase can be skipped by omitting all AFIs/SAFIs from the GR Capability,
advertising a Restart Time of zero, or both". `short-lived family` and
`long-lived family` govern code 64 and code 71 independently.

### The Two Bits Ze Sets at Connection Time

The payload above is built when the configuration is loaded, so it carries what
the operator configured and nothing about a restart. Two bits say what happened
to Ze, and both are written on the way to the OPEN.

The Restart State bit says Ze has restarted. `ze signal restart` writes the
restart marker, and the reactor sets the bit while that marker is live. Outside
the window a new connection gets 0, which is a cold start.

The Forwarding State bit of an address family says the routes of that family
were still being forwarded while Ze was down. Ze sets it inside the same window,
and only when the forwarding plane kept its routes. The kernel FIB does keep
them by default: `fib { kernel { } }` leaves every route it installed in place
as Ze stops, marks them on the way back up and removes only the ones that do not
return (`flush-on-stop`, `sweep-delay`). Writing `flush-on-stop true`, or
configuring no FIB at all, means nothing was preserved and the bit stays 0.

The bit decides what the peer does the moment Ze comes back. RFC 4724 Section
4.2: if it "is not set in the newly received Graceful Restart Capability ...
then the Receiving Speaker MUST immediately remove all the stale routes from the
peer that it is retaining for that address family". So a peer holds Ze's routes
while the session is down either way, and a clear bit makes it drop them at
re-establishment rather than waiting for Ze to re-advertise.
<!-- source: internal/component/bgp/plugins/gr/gr_capability.go -- parseGRCapValue, extractGRCapabilities -->
<!-- source: internal/component/bgp/reactor/peer_gr_flags.go -- restartFlagsFor -->
<!-- source: internal/component/bgp/grmarker/grmarker.go -- SetRBit, SetFBit -->
<!-- source: internal/plugins/fib/kernel/register.go -- preservesForwardingState -->

### Normal Session

1. Peer session establishes, GR capability negotiated in OPEN
2. Routes received and installed in RIB normally

### Peer Restarts

1. **Peer goes down** -- unless the session ended by a NOTIFICATION, sent or received. RFC 4724 Section 4 says "the normal BGP procedures MUST be followed when the TCP session terminates due to the sending or receiving of a BGP NOTIFICATION message", so that down (reason `notification`) retains nothing. Ze never advertises the RFC 8538 N bit, so no NOTIFICATION is graceful. A session whose received UPDATEs Ze could not publish to its plugins retains nothing either: Ze ends it with a Cease (Out of Resources), and when that Cease did not reach the peer, or the session had already ended, the down carries reason `adj-rib-in-invalid`, because the routes it holds are known to be incomplete or already withdrawn. For any other down (`tcp-failure`, `local-close`) the GR plugin sends `retain-routes` with the received GR families plus exchanged LLGR families whose received LLST is nonzero
2. **RIB marks routes as stale** -- routes kept in forwarding but flagged
3. **Restart timer starts** -- countdown from `restart-time` seconds
4. **Peer reconnects** -- new session established, fresh routes received. A family whose Forwarding State bit is clear in the new OPEN, a family the new capability omits, and every family when the new OPEN carries no Graceful Restart Capability at all, are purged with `purge-stale` at once (RFC 4724 Section 4.2)
5. **Fresh routes replace stale** -- each new route implicitly clears its stale flag
6. **End-of-RIB received** -- GR plugin sends `purge-stale` to RIB
7. **Remaining stale routes removed** -- any route not refreshed is withdrawn

The RIB retains only those families: a negotiated family with neither a GR
period nor an enabled LLGR period is removed from the received inventory as
well as withdrawn from its recorded destinations. GR supplies the explicit
`on-down` argument so RIB applies the family list before its DOWN event but
leaves nonretained-family wire withdrawals to the forwarding DOWN owner (RS);
no second retained-route inventory is kept. The operator form
`request bgp rib retain-routes <selector>` remains peer-wide. Supplying
`[family ...]` restricts it to those families and withdraws the others directly,
including when the source session is still established.

The GR plugin uses the internal `request bgp rib source` commands, always
with `owner <original-owner> connection <original-connection>` before the
operation's arguments. `retain-routes` takes `on-down families <afi/safi>...`;
`mark-stale` takes `restart-time <seconds> [stale-level <level>] [family <afi/safi>]`;
`purge-stale` takes an optional `family`; and `release-routes` takes nothing
further. `attach-community` and `delete-with-community` take both `family` and
`community <eight-hex-digits>`. These commands match received source references
and sent provenance, so an old connection's cleanup cannot touch a new physical
connection or a different configured Peer at the same address.

Retention uses the existing active GR state and original source leases. Family
cleanup can empty that cycle before LLGR's next operation; its active state
remains until explicit release, safety expiry or replacement by a new current
cycle. A replacement keeps, as earlier connections of the new cycle, the
connections of the old one that still own a route. This happens when a peer in
its LLGR period reconnects and drops again before End-of-RIB. The
`retain-routes` answer lists them under `earlier-connections`, as decimal
strings, and the list is empty for a first restart. An earlier connection
stays addressable by its own `owner` and `connection` until it is released,
even after a family purge has removed its last route. There is no
completed-source ledger. Cleanup joins captured selected
output and original lease releases before answering; a refused join reports the
unfinished work. The public peer-wide commands below retain their operator
scope.
<!-- source: internal/component/bgp/plugins/rib/rib_source_commands.go -- ribSourceCommands, retainPhysicalSourceCommand, markPhysicalSourceCommand -->
<!-- source: internal/component/bgp/plugins/rib/rib_source_mutation.go -- removePhysicalSource, expirePhysicalSource, retainPhysicalStateLocked, earlierRetainedLocked -->
<!-- source: internal/component/bgp/plugins/rib/rib_source_commands.go -- retainedSourceAnswer -->

Likewise, `request bgp rib mark-stale <peer> <restart-time> [level [family]]`
remains peer-wide when the family is omitted. GR supplies the particular
family when raising routes to LLGR level 2, so entering LLGR for one family
does not raise another family's level.

### Restart Timer Expiry

Without LLGR, Ze releases the peer's retained routes when the received
`restart-time` expires. Expiry runs only after the session-down retention and
stale marking finish. That processing time counts toward the restart deadline.
A zero Restart Time therefore releases routes before the DOWN handler returns,
rather than leaving an observable GR retention window.
<!-- source: internal/component/bgp/plugins/gr/gr_state.go -- onSessionDownDeferred, startRestartTimer, handleTimerExpired -->
<!-- source: internal/component/bgp/plugins/gr/gr.go -- handleStructuredState, handleStateEvent, onTimerExpired, releaseRoutes -->

### Fail-Safe

For a nonzero Restart Time, the RIB also starts a safety timer at
`restart-time + 5s`. This margin belongs to the safety timer, not the GR
plugin's restart deadline. A zero Restart Time arms no RIB safety timer:
the GR plugin releases routes immediately or enters the negotiated LLGR period.
<!-- source: internal/component/bgp/plugins/rib/rib_commands.go -- markStaleCommand, grTimerMargin -->
<!-- source: internal/component/bgp/plugins/gr/gr_state.go -- onSessionDownDeferred, startRestartTimer -->

## Plugin Bindings

The GR plugin requires:
- `receive [ open state eor ]` -- needs both received and sent OPENs, up/down events, and End-of-RIB markers. LLGR compares the peer's declaration with the families in Ze's own sent OPEN; receiving only `open-received` cannot establish that both speakers enabled the procedure.
- The RIB plugin must also be loaded with `receive [ update state refresh ]` and `send [ update ]`

The GR plugin depends on `bgp-rib` (declared in its registration). The engine ensures bgp-rib starts first.
<!-- source: internal/component/bgp/plugins/gr/register.go -- Dependencies: bgp-rib -->

GR and LLGR capability values in normalized JSON OPEN events contain only the
payload bytes, encoded as lowercase hex. The capability code and length are
not part of `value`. JSON and structured OPEN events give the GR plugin the
same received restart times and native address families. Sent OPEN events
identify the same locally advertised LLGR families on both paths.
For example, GR value `000300010180` means three seconds and
`ipv4/unicast`, with forwarding preserved.
<!-- source: internal/component/bgp/format/decode.go -- formatCapability -->
<!-- source: internal/component/bgp/plugins/gr/gr.go -- handleOpenEvent, extractGRCaps -->
<!-- source: internal/component/bgp/plugins/gr/gr_llgr_exchange.go -- handleSentOpenEvent, addSentLLGRFamilies -->

### Running bgp-gr in a child process

The examples on this page load the plugin in-daemon with `bus { endpoint gr { internal bgp-gr; } }`.
Adding `process separate;` runs the same engine in a child process over TLS
connect-back, and it behaves the same way. The RFC 9494 procedures run in the
plugin, beside the peer capability state it records, as members of the peers'
filter lists (see "Long-Lived Stale Routes in the Filter Lists" below), and
the families it retains for a departed peer reach `bgp-rr` and `bgp-rs` as the
`bgp-gr/retention` event (described below). Both work over the plugin
connection, so neither needs the daemon's memory.
<!-- source: internal/component/bgp/plugins/gr/gr.go -- runGRPlugin -->
<!-- source: test/plugin/llgr-transition-forked.ci -- the LLGR lifecycle with bgp-gr forked -->
<!-- source: test/plugin/llgr-readvertise-forked.ci -- the stale readvertise with bgp-gr forked -->
<!-- source: test/plugin/llgr-import-no-llgr-forked.ci -- import NO_LLGR with bgp-gr forked -->
<!-- source: internal/component/bgp/plugins/gr/gr_filter.go -- evaluateGRFilter, grInsertions -->

The route server and reflector decide which families to retain from `bgp-gr`'s
bus verdict for that DOWN, not an in-process query. After draining that source's
forwarding worker, the route server withdraws only the other families. The
reflector makes the same decision before handing the source's routes to the
engine's departure recovery. Without that filter a reflector would withdraw
every retained route from its other peers during the restart window. Both
handlers consume the verdict synchronously while they
handle the DOWN, so a reconnect or a Restart Time expiry that follows cannot
change which families that DOWN withdraws. Each handler then queues the routes
left over on its plugin's departure workers and returns without waiting.
Up to eight workers send departures to the engine, taking sources in the order
they went down, so a source whose departure the engine is slow to complete
holds back only its own worker. One source's departures are sent one at a
time, and a source that goes down again before its waiting departure was sent
is merged into it. At shutdown each plugin cancels its outstanding departure
requests, then stops the workers once they have handled every queued
departure; a plugin that never queued a departure has no workers to stop.

`bgp-gr` tells both owners its decision on the bus. On every physical DOWN it
handles, it emits a `bgp-gr/retention` event before its DOWN handler returns.
The event carries the original peer owner, physical connection, address and
retained families (`<afi>/<safi>`), with an empty list after a NOTIFICATION close
or when the peer has no GR capability. Owner-terminal peer removal closes the
forwarding lifetime directly and does not need a physical retention verdict. The engine delivers a DOWN to one plugin at a time, in reverse
dependency order, and waits for each plugin's handler to return, so `bgp-gr`'s
event is queued to both owners before their DOWN. Each process receives its
events in the order they were queued. `bgp-rs` is an optional dependency of
`bgp-gr`, and `bgp-rr` is named in its `StartAfter` (order only, since a GR
configuration does not load the reflector). Each owner subscribes at startup,
but records verdicts only within state lifetimes it owns: every received UP,
including an `attached` UP on reload, opens a lifetime and clears any earlier
verdict. Each physical DOWN consumes only that connection's verdict and closes
its lifetime, including empty departures and detach. Peer removal closes the
actual owner's lifetime through `End`, including when no current Session exists. A global verdict for a peer whose UP this
owner never received, or whose DOWN it already handled, creates no record.
Storage is therefore bounded by active owned state lifetimes, even when dynamic
addresses churn outside this forwarder's attachments. Recreating an address
opens a fresh lifetime; a DOWN without a new verdict retains nothing. An owned
verdict naming an unknown family is refused and logged. When the Restart
Time expires without a reconnect, the retained
routes are deleted and the reflector's other peers are sent their withdrawal,
under the Path Identifier each was sent. The verdict comes from GR's existing
family state; no second route inventory is kept. The event crosses a process
boundary, so `bgp-gr`, `bgp-rs` and `bgp-rr` may each run in a child process
(`run "ze plugin bgp-rr"`), in any mix. An edge another plugin declares on
`bgp-rr` or `bgp-rs` reaches it forked as well, under the implementation its
`run` line names, so `bgp-gr`'s verdict still precedes their DOWN. Use internal
`bgp-rib` for the coordinated lifecycle described here.
<!-- source: internal/component/bgp/retention/retention.go -- Event, Verdict, Ledger -->
<!-- source: internal/component/bgp/plugins/gr/gr.go -- emitRetention -->
<!-- source: internal/component/plugin/process/delivery.go -- deliverMixedBatch -->
<!-- source: internal/component/bgp/plugins/rr/rr.go -- runRouteReflector, stopDepartures -->
<!-- source: internal/component/bgp/server/events.go -- sortByReverseDependencyTier -->
<!-- source: internal/component/bgp/plugins/rs/server.go -- RunRouteServer -->
<!-- source: internal/component/bgp/plugins/rs/server_handlers.go -- handleStateDown, enqueueDeparture, stopDepartures -->
<!-- source: internal/component/bgp/plugins/rr/rr.go -- handleStateDown, enqueueDeparture -->
<!-- source: internal/core/syncutil/coalescer.go -- Coalescer, Enqueue, Stop -->
<!-- source: internal/component/bgp/plugins/gr/register.go -- init (OptionalDependencies bgp-rs, StartAfter bgp-rr) -->
<!-- source: test/plugin/addpath-departure-rr-gr-retained.ci -- a retained family is not withdrawn by the reflector -->
<!-- source: test/plugin/addpath-departure-rr-gr-retained-forked.ci -- the same with bgp-rr and bgp-gr forked -->
<!-- source: test/plugin/addpath-departure-rr-gr-notification-forked.ci -- a DOWN after a NOTIFICATION retains nothing, forked -->
<!-- source: test/plugin/addpath-departure-rr-gr-expiry.ci -- at Restart Time expiry the reflected paths are withdrawn under ze's identifiers -->


## CLI

```
 $ ze cli -c "show bgp rib | received"
# Shows routes with stale flag when applicable
```

## Long-Lived Graceful Restart (RFC 9494)

LLGR extends standard GR with a second, much longer stale period. When the GR restart-time expires without the peer reconnecting, instead of purging all stale routes, LLGR keeps them for up to ~194 days with reduced priority. Both the stale time Ze advertises and the stale time a peer advertises are configured and tracked per address family.

### Configuration

Add `long-lived` with per-family stale times under the `graceful-restart` block:

```
capability {
    graceful-restart {
        short-lived {
            restart-time 120;
        }
        long-lived {
            family {
                ipv4/unicast 3600    # LLGR period in seconds (0-16777215)
                ipv6/unicast 86400
            }
        }
    }
}
```

| Path | Type | Default | Description |
|------|------|---------|-------------|
| `graceful-restart / long-lived / family / stale-time` | uint32 | -- | Per-family LLST Ze advertises in code 71 (0-16777215, 24-bit, mandatory per entry); received peer values govern Ze's helper timers |

LLGR is off unless you configure it per address family. Ze advertises the LLGR capability (code 71) in its OPEN only for families listed under `graceful-restart long-lived family`, each with its own `stale-time` (RFC 9494 Section 5). Nesting `long-lived` inside `graceful-restart` ensures code 71 is never advertised without code 64 (RFC 9494 Section 4.1).

The `short-lived family` and `long-lived family` lists are independent: omitting `short-lived` skips the conventional GR phase while still sending code 64 with Restart Time 0 and no tuples, and naming a family only in `short-lived` keeps LLGR off for it. Received code-71 tuples are tracked per family and honored only for families Ze also declared in its sent OPEN.
<!-- source: internal/component/bgp/plugins/gr/yang/ze-graceful-restart.yang -- graceful-restart-config -->

LLGR is only active for a family when both OPENs of the session list it in their LLGR capability. A peer that advertises LLGR for a family Ze did not advertise it for gets the base GR treatment for that family: its routes are kept for the Restart Time and no longer (RFC 9494 Section 5 requires configuration per AFI/SAFI before the procedures run). LLGR also requires the GR capability (code 64) in the same OPEN: LLGR without GR is ignored, as RFC 9494 Section 4.5 requires.
<!-- source: internal/component/bgp/plugins/gr/gr_llgr_exchange.go -- exchangedLLGRLocked -->
<!-- source: internal/component/bgp/plugins/gr/gr_llgr.go -- extractLLGRCapabilities -->
<!-- source: internal/component/bgp/plugins/gr/gr.go -- forgetGRCapability -->
<!-- source: internal/component/bgp/plugins/gr/register.go -- CapabilityCodes: 64, 71 -->

### How It Works

1. **GR period expires** -- peer has not reconnected within `restart-time` seconds
2. **LLGR begins** -- for each family with a nonzero received Long-Lived Stale Time (`stale-time > 0`):
   - Routes carrying the NO_LLGR community (0xFFFF0007) are deleted
   - LLGR_STALE community (0xFFFF0006) is attached to remaining stale routes
   - Routes are marked as stale level 2 (deprioritized in best-path selection)
   - Per-family LLST timer starts
3. **During LLGR** -- LLGR-stale routes lose to any non-stale route in best-path selection. Between two LLGR-stale routes, normal tiebreaking applies.
4. **LLST timer expires** -- stale routes for that family are purged
5. **Peer reconnects during LLGR** -- standard RFC 4724 procedures apply; families with F-bit=0 or missing from the new OPEN are purged. A family whose LLGR period has begun must also pass the new LLGR Capability: if that capability clears the family's F bit, omits the family, or is absent, the family's stale routes are purged at once, whatever the GR Capability says (RFC 9494 Section 4.2). A family still in its conventional GR period is judged by the GR Capability alone. A configuration with a Restart Time of zero therefore keeps its stale routes across a reconnect only when the new GR Capability also lists the family with its F bit set
6. **Peer drops again before End-of-RIB** -- a family whose LLST timer is running keeps that timer, unchanged, and keeps its stale routes (RFC 9494 Section 4.2: "in the case of consecutive restarts, the previously marked stale routes MUST NOT be deleted before the timer for the Long-Lived Stale Time expires."). The routes the second session delivered for that family are retained too: they are raised to stale level 2 and get LLGR_STALE at once, and those carrying NO_LLGR are deleted. They start no Restart Time period of their own. When the timer expires, or a later session sends End-of-RIB for the family, the stale routes of every connection are deleted. A family still in its conventional GR period follows RFC 4724 instead: its previously stale routes are deleted at the second drop and the routes of the second session start a new Restart Time
<!-- source: internal/component/bgp/plugins/gr/gr_state.go -- onSessionReestablished, reconnectKeepsFamily, onSessionDownPlan -->
<!-- source: internal/component/bgp/plugins/gr/gr_consecutive.go -- restalePreservedLLGR, dispatchRetained, recordEarlierRetained -->

The LLST expiry uses the time in the **remote peer's received OPEN**, not the
per-family `stale-time` Ze advertises from its local configuration. Different
peers can therefore expire at different times even when their local
configuration is identical. Retention means routes in the received RIB:
replaying a locally configured static route after reconnect does not demonstrate
that a received route survived either restart period.

EOR and expiry withdraw only the stale received paths they remove. A refreshed
path is not withdrawn merely because its new UPDATE has reached the received RIB
before route-server forwarding or sent-event delivery catches up. Received
ADD-PATH identifiers, including zero, distinguish paths even when one UPDATE
originally advertised them together. Cleanup still carries the old destination
advertisement's receipt, so it cannot remove a newer advertisement from the same
source or another source. Received-state deletion alone is not proof of delivery
to the destination; the final wire checks remain separate.
<!-- source: internal/component/bgp/plugins/rib/rib_commands.go -- purgeStaleCommand, autoExpireStale -->
<!-- source: internal/component/bgp/plugins/rib/rib_sent_lifecycle.go -- withdrawRemovedSentLocked, sentReceivedOwner -->

Each family's LLST deadline is the original GR deadline plus that family's
received Long-Lived Stale Time. Time spent dispatching the DOWN commands does
not extend either period. After retention and stale marking finish, Ze removes
families whose deadlines already elapsed. Other families enter LLGR only for
the time that remains. This also applies when Restart Time is zero.
<!-- source: internal/component/bgp/plugins/gr/gr_state.go -- onSessionDownDeferred, enterLLGRLocked -->

The timer fixture `llgr-peer-stale-time-drives-timer.ci` gives the source a
3-second LLST and a control peer 60 seconds, against Ze's local 3600. Its
receiver acknowledges the control's initial route and LLGR_STALE advertisement
before the source advertises. This order matters: each LLGR entry calls
`clear bgp rib out` for the destination's whole family, not just the routes
from the peer entering LLGR. The source's later transition must therefore
deliver its two retained routes and the already-stale control route, each
with LLGR_STALE in the same UPDATE as that route. Only the receiver grants
route-server and RIB output; the strict source and control peers receive only
the observer's release markers. Destination acknowledgments fence the real
TCP closes, the stale phase, and both short-LLST expiry withdrawals. The
control remains in the received RIB and is forbidden from being withdrawn
on the receiver's wire.
<!-- source: internal/test/fixture/plugin_fixture_llgr_lifecycle.go -- llgrLifecycle -->
<!-- source: internal/component/bgp/plugins/gr/gr.go -- wireStateCallbacks -->
<!-- source: internal/component/bgp/plugins/rib/rib_commands.go -- outboundResend -->

A peer selector given to bgp-rib by name matches the configured name the
session's state events carried (`selectsPeer`), since bgp-rib keys sessions by
address. A session whose events named it nothing is selected by no name.
<!-- source: internal/component/bgp/plugins/rib/rib_peer_name.go -- selectsPeer -->

The received-route EOR scenario uses a different release prefix for each phase.
The initial readiness and DOWN-release markers replay when the source
reconnects; neither releases its EOR. Only after the observer sees the refreshed
route without LLGR_STALE and the unrefreshed route still stale does it send
the distinct EOR-release marker. The receiver then acknowledges the
unrefreshed route's withdrawal before the scenario can finish. Its ACK routes
are not reflected to the strict source: that source grants RIB marker replay,
but no route-server or Adj-RIB-In replay output.
<!-- source: internal/test/fixture/plugin_fixture_llgr_lifecycle.go -- llgrLifecycle eor scenario -->

The NO_LLGR decision uses the imported route, so a `modify` import policy that
adds `65535:7` also excludes that route from LLGR retention. A route without the
community remains eligible for the LLGR period.

A fresh UPDATE replaces the locally decorated attributes as well as clearing the
stale level. In particular, re-sending the original route without LLGR_STALE
removes the community Ze attached during retention; an original-wire cache hit
must not leave that local decoration on the refreshed route.

The RIB also replaces its owned sent-attribute reference before readvertising
LLGR_STALE. It retains unrelated attributes, including unknown transitive ones,
and does not edit shared pool bytes. NO_LLGR deletion, End-of-RIB purge, and
expiry remove only advertisements owned by that received source route and send
withdrawals to their recorded destinations. This applies to native NLRIs as
well as CIDR routes: received labeled routes use stripped prefix keys, whereas
sent labeled routes keep their wire labels. Received generation and route
identity establish ownership; inbound and outbound ADD-PATH identifiers are
not assumed equal. Replay feedback cannot recreate deleted ownership or replace
the current locally decorated attributes.
<!-- source: internal/component/bgp/plugins/rib/rib_sent_lifecycle.go -- withdrawRemovedSentLocked, attachSentSourceCommunityLocked, attachSentCommunity -->

State and End-of-RIB events run in reverse implementation-dependency order:
`bgp-gr` precedes `bgp-rib`, including when either is loaded under an operator
alias such as `gr` or `rib`. This lets GR request retention before the RIB
processes peer-down. An external program does not inherit a compiled-in
plugin's dependency edges merely by sharing its name: it is ordered by the
edges its own Stage 1 declaration carries. `bgp-gr` declares the same edges
at Stage 1 as in its registration, so the order a forked `bgp-gr` would take
a DOWN in is the internal one: before `bgp-rr`, `bgp-rs` and `bgp-rib`, with
its retention verdict queued ahead of their DOWN. The same order-only edges
place GR ahead of `bgp-adj-rib-in` and `bgp-rpki` when either is loaded; enabling
GR does not load optional receive validation.

A retained route keeps its original received validation outcome and cache
revalidation inputs through the original GR family lifetime. Retention does not
accept a pending or Invalid path. Cache and policy changes can still make that
same retained path ineligible or eligible before GR ends. Reconnection never
relabels those bytes as the replacement connection.

For an End-of-RIB purge, expiry or NO_LLGR deletion, GR first closes the exact
original owner, physical connection and family scope in the received validation
stores. It joins the captured consumers and their declared deferred replay work,
then runs the existing selecting-RIB source cleanup and its final-output join.
The order matters because one source receipt covers a whole original session;
a retained sibling can still hold it after another path is removed. A failed
application or replay join is reported as unfinished work and cannot authorize
the final purge's success. NO_LLGR matches the effective accepted received
attributes, including a community added by the import policy.
<!-- source: internal/component/bgp/plugins/gr/validation_cleanup.go -- prepareValidationCleanup -->
<!-- source: internal/component/bgp/plugins/adj_rib_in/retained_validation.go -- handleRetainedValidationEvent, cleanupValidationConnectionLocked -->
<!-- source: internal/component/plugin/server/emit_event_applied.go -- emitEventApplied -->
<!-- source: internal/component/bgp/plugins/gr/register.go -- grDependencies, grOptionalDependencies, grStartAfter -->
<!-- source: internal/component/bgp/server/events.go -- sortByReverseDependencyTier -->
<!-- source: internal/component/bgp/plugins/gr/gr.go -- handleStructuredState -->
<!-- source: internal/component/bgp/plugins/rib/rib.go -- commandDecls declares the LLGR community mutation commands -->

### Stale Levels

Ze uses a graduated stale level system for route prioritization:

| Level | Meaning | Best-path behavior |
|-------|---------|-------------------|
| 0 | Fresh | Normal selection |
| 1 | GR-stale | Normal selection (not deprioritized) |
| 2+ | LLGR-stale | Loses to any route with level < 2 |

A route a neighbor sends carrying LLGR_STALE is least preferred too, when Ze
sent that neighbor the LLGR Capability (RFC 9494 Section 4.3 binds "A BGP
speaker that has advertised the Long-Lived Graceful Restart Capability to a
neighbor"). From any other neighbor the route competes normally. The GR
plugin decides this as the UPDATE arrives, before the operator's import chain runs, and
best path treats the route as level 2. That mark is separate from the stale
level above: `mark-stale` and `purge-stale` act on the stale level alone, so
such a route is retained at a TCP loss and survives an EOR like a fresh route.
A refresh without the community makes it a normal route again. When the route
is advertised, the LLGR egress decision applies to it as to a route Ze marked
itself: withdrawn toward an EBGP neighbor without LLGR, sent with NO_EXPORT and
LOCAL_PREF 0 toward an IBGP one. That holds on every path that advertises it,
listed under "Readvertising a Stale Route" below. The `show bgp rib` stale
level still shows only the retention mark.
<!-- source: internal/component/bgp/plugins/gr/gr_ingress.go -- LLGRIngressFilter, llgrStaleFromSource -->

### Long-Lived Stale Routes in the Filter Lists

Both LLGR decisions are members of the peers' filter lists, which `bgp-gr`
inserts itself. On every peer whose `graceful-restart long-lived family` is
configured, the list that makes Ze advertise the LLGR Capability,
`bgp-gr:llgr-ingress` runs on import, ahead of every member the operator names
in the import chain, and marks a received LLGR_STALE route as described above.
While any peer configures `graceful-restart long-lived family`,
`bgp-gr:llgr-egress` runs on every peer's export list, ahead of the operator's
members: RFC 9494 Section 4.3 binds the routes of the neighbor Ze advertised
the capability to, and they can leave toward any peer. It
declares `readvertise`, so the stale readvertise rail runs it too, and
`judges-final`, so it runs again over a body the export chain rewrote: that is what
keeps LLGR_STALE, its NO_EXPORT and its LOCAL_PREF 0 on a route whatever a member
behind it did, unless that member also declares `judges-final`, whose answer then
comes after it. A lone member on the stale rail has nothing behind it and runs once. Both members are required:
a chain that names, moves or deactivates one is refused at commit with the
RFC 9494 Section 4.3 reason. The decisions read the route's stale level, the
destination's original runtime `peer-owner` and actual `ibgp` classification,
and for a route rebuilt from storage the source session's
`source-llgr-advertised` fact. A destination's LLGR capability belongs to its
original owner and physical connection: another peer at the same address,
or a replacement connection of that owner, cannot borrow an old OPEN capability. Missing
owner information means no capability was recorded. Missing internal-session
classification rejects a governed route; it does not infer one from ASNs.
The ordinary fresh-route fast path needs neither fact.
<!-- source: internal/component/bgp/plugins/gr/gr_filter.go -- grFilterDecls, grInsertions, evaluateLLGRIngress, evaluateLLGREgress -->
<!-- source: internal/component/bgp/plugins/rib/rib_structured.go -- receivedLLGRStaleFromMeta -->
<!-- source: internal/component/bgp/plugins/rib/rib_commands.go -- extractCandidate -->

### Readvertising a Stale Route

RFC 9494 Section 4.3 governs a route at the LLGR stale level (2), whether Ze marked it or it arrived with LLGR_STALE, that ze forwards on to a neighbor from which the LLGR Capability was not received. The stale level on the ROUTE decides, not the restart state, so a route raised to level 2 by `request bgp rib mark-stale` takes the same treatment as one raised by a peer's LLGR period.

| Destination | Action |
|-------------|--------|
| LLGR was received from it | Advertise unchanged |
| Internal, LLGR not received | Advertise with NO_EXPORT and LOCAL_PREF 0 (Section 4.6) |
| External, LLGR not received | Withdraw the route (Section 4.3) |

Internal means the session's reactor-supplied `ibgp` fact, including a session
configured for RFC 7705 AS migration. A migration session can be internal while
its peer AS differs from the effective local AS.

Automatic stale re-advertisement uses `clear bgp rib out-excluding-owner owner
<original-owner> [family <afi/safi>]` to omit the exact original peer whose
restart triggered it. The owner is a nonzero unsigned 64-bit runtime identity;
it narrows replay and grants no send permission. The ordinary `clear bgp rib
out` command keeps its selector grammar, including peers literally named
`owner` or `exclude-owner`.

The replay captures every remaining destination's current UP receipt. Ordinary
destinations use the selected-route worker and its final End receipt; auxiliary
destinations replay immutable SENT attributes with bound Begin, route and End
requests. Replacing a destination refuses its remaining old-session output.
The command reports refreshed and refused destination counts and returns an
error if any captured operation remains unresolved. It sends no initial
readiness report for this same-session replay.
<!-- source: internal/component/bgp/plugins/rib/rib_resend_excluding_owner.go -- capture, resend, resendExcludedSent -->

Adding NO_EXPORT to a received COMMUNITY preserves its Partial bit, along with
the retained community values. The community rebuild uses an Extended Length
header for both complete and partial attributes.
<!-- source: internal/component/bgp/plugins/filter_community/handler.go -- emitCommunity -->

The filter fails closed. When it cannot read the peer capability state, it answers "LLGR was not received" for every destination. It raises one warning per process and then withdraws or depreferences. An unread state used to accept, which put a long-lived stale route into a neighbor that never agreed to hold one.
<!-- source: internal/component/bgp/plugins/gr/gr_egress.go -- LLGREgressFilter, egressState -->

Only a route at the LLGR stale level takes this treatment: one Ze marked LLGR
stale, or one received with LLGR_STALE as described above. A route in its
conventional GR period (stale level 1) carries no LLGR_STALE, so it is
advertised like any other route when the RIB replays it to a re-established
neighbor or resends it on a route refresh.

Every path that advertises a route reaches the same decision, and each carries
the received LLGR_STALE fact its own way:

| Path | How the fact arrives |
|------|----------------------|
| Live forward and route server forward | The GR plugin's ingress step marks the route as it arrives |
| RIB replay to a re-established neighbor, `clear bgp rib out`, route refresh | The RIB reads the source route the copy was sent from, so a mark-stale on the copy does not hide it |
| `bgp-adj-rib-in` replay and relay recovery | The rebuilt route is marked `rebuilt`, and the GR plugin re-runs its ingress test on the source session and the stored COMMUNITIES |
| AIGP metric re-advertisement | The route keeps the mark it was given on arrival |
<!-- source: internal/component/bgp/plugins/gr/gr_egress.go -- advertisedStaleLevel -->
<!-- source: internal/component/bgp/plugins/rib/sent_owner.go -- sentSourceState, replayStaleLevel -->
<!-- source: internal/component/bgp/plugins/rib/storage/routeentry.go -- AdvertisedStaleLevel -->
<!-- source: internal/component/bgp/reactor/reactor_api_relay.go -- buildRelayUpdate, resolveRelaySource -->

The state lives in the plugin's own process, beside the filter that reads it, so `run "ze plugin bgp-gr"` behaves the same way. See [Running bgp-gr in a child process](#running-bgp-gr-in-a-child-process).

### Original source ownership

Automatic GR actions use the original engine peer owner and physical
connection carried by the event. The address is for display. Two peers at the
same address have separate capabilities and timer cycles. A reconnect also has
a new physical `connection-id`, distinct from the destination replay receipt.
The `request bgp rib source` commands carry `owner N connection C` to target the
original received routes. These identifiers narrow existing rights and grant
no additional permission. A successor's EOR removes the original connection's
retained stale routes; a delayed original release does not erase fresh routes.

After a consecutive restart during the LLGR period, one retained cycle holds
routes of more than one connection: the stale routes an earlier connection
delivered and the later one did not refresh, and the routes the later one
delivered. The RIB names the earlier connections in its `retain-routes`
answer. GR then sends each purge and each release once per connection, the
earlier ones first, and the `validation-cleanup` event with it, so every store
keyed by connection is told about each one. At the second drop the NO_LLGR
deletion and the LLGR_STALE attachment also go to each connection, so every
earlier route carries the community whichever connection delivered it. Stale
marking goes to the latest
connection only: the earlier routes were marked when their own connection
went down. If the answer names no list, GR keeps
addressing every connection the cycle already held and logs a warning.
<!-- source: internal/component/bgp/plugins/gr/gr_consecutive.go -- retainedConnections, dispatchRetained, recordEarlierRetained, earlierConnections -->

The `bgp-gr/retention` event carries `peer-owner` as an unsigned integer and
`connection-id` as a decimal unsigned integer string. The forwarding owner's
ledger admits one connection per owner. A verdict or DOWN for an old connection
cannot fill or consume the successor's lifetime. Both identifiers preserve the
full 64-bit value, including values above 2^53.

A `removed` state event (`rpc.ReasonPeerRemoved`) ends the actual peer owner, including every old
retained connection. It can carry connection zero when no current Session
exists. GR handles this terminal owner separately, before physical-source
validation. The forwarding ledger's `End` closes that owner's lifetime; zero
is never accepted as a wildcard for a transport DOWN or a lifecycle command.
<!-- source: pkg/plugin/rpc/enums.go -- ReasonPeerRemoved -->
<!-- source: internal/component/bgp/plugins/gr/gr_source.go -- newGRSource, grJSONSource -->
<!-- source: internal/component/bgp/plugins/gr/gr.go -- dispatchSource, emitRetention -->
<!-- source: internal/component/bgp/retention/retention.go -- Verdict, Ledger -->

An LLGR callback checks that its captured state still owns the family's timer.
An LLGR entry already being applied when the peer reconnects runs to its end:
the NO_LLGR deletion, the LLGR_STALE attachment, stale level 2 and the
readvertisement all still go to the original connection, and the LLST timer is
neither restarted nor updated (RFC 9494 Section 4.2). Routes the new connection
sends are stored under that connection, so the entry never marks them. An entry
stops only when its family's LLST timer is gone (End-of-RIB, expiry, a purge on
reconnect) or a later session-down changes the cycle's source, which completes
it. A reconnect still cancels deferred entry completions. An LLGR entry that
has begun a blocking RIB RPC rechecks its timer before it sends the next
mutation.
<!-- source: internal/component/bgp/plugins/gr/gr_state.go -- handleLLSTExpired, llgrFamilyActive -->

### Special Case: Skip GR

If `restart-time` is 0 (or `short-lived` is omitted) while `stale-time` is nonzero, the GR period is skipped entirely. On session drop, LLGR begins immediately, after the routes are retained and marked stale: the LLGR steps above run last, so the routes they mark LLGR-stale are the retained ones. When both times are 0 the peer's routes are released at once, as in base BGP.
<!-- source: internal/component/bgp/plugins/gr/gr_state.go -- onSessionDownDeferred -->
<!-- source: internal/component/bgp/plugins/gr/gr.go -- handleStructuredState, handleStateEvent -->

A family can also skip GR when the received code-64 capability omits it.
RFC 9494 Section 4.2: "If the Graceful Restart Capability that was received
does not list all AFIs/SAFIs supported by the session, then the GR Restart Time
shall be deemed zero for those AFIs/SAFIs that are not listed." If both OPENs
declare LLGR for that family and its received LLST is nonzero, Ze retains it
and enters LLGR immediately after the DOWN retention commands finish.

This is a per-family decision. An omitted family's LLST runs from the original
DOWN instant, while a family listed in GR still gets its conventional GR
period followed by its own LLST. The later GR expiry does not reset an LLST
timer already running, and the first family's expiry does not release another
family still in GR. Conventional expiry is armed after retention and stale
marking, before the immediate family's readvertisement callbacks run, so a
blocked readvertisement cannot extend its sibling's GR or LLST period.
A family omitted from GR with zero or unexchanged LLST is not retained.
<!-- source: internal/component/bgp/plugins/gr/gr_state.go -- onSessionDownDeferred, enterLLGRLocked, handleLLSTExpired -->
<!-- source: internal/component/bgp/plugins/gr/gr.go -- retainPeerFamilies -->

Before 2026-10-02 the immediate LLGR entry ran first, and the session-down purge that follows it deleted every route it had just marked, so a peer advertising `restart-time 0` lost all its routes on a TCP failure.

| `restart-time` | `stale-time` (LLST) | Behavior |
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


### Automatic source lifecycle arguments

Automatic GR requests use `request bgp rib source <operation> owner <id>
connection <id>`. The connection remains the original physical source even
after reconnect. Retention appends `on-down families <afi/safi> ...`; the
family list comes last. Stale marking appends `restart-time <seconds>` and,
for LLGR, `stale-level 2 family <afi/safi>`. Family purges append
`family <afi/safi>`, and community operations append `family <afi/safi>
community <eight hex digits>`. Release has no operation arguments.

The GR plugin sends these arguments through the internal command dispatcher.
The operator commands outside `source` use peer selectors. The captured owner
and connection limit which routes cleanup can affect and grant no permission
to receive, select, or send a route.
<!-- source: internal/component/bgp/plugins/gr/gr.go -- dispatchSource, retainPeerFamilies, wireStateCallbacks -->
<!-- source: internal/component/bgp/plugins/rib/rib_source_commands.go -- parseRIBPhysicalSource, retainPhysicalSourceCommand, markPhysicalSourceCommand, parsePhysicalSourceCommunity -->


The passive validation owners also consume original source removal.
`removed` (`rpc.ReasonPeerRemoved`) ends the logical owner's entire retained population, whereas a
physical DOWN still names one exact connection. RPKI checks queued validation
against the existing active lifetime or its current original route tracker and
joins already dispatched decisions before acknowledging cleanup. Adj-RIB-In
accepts retained-route revalidation without permitting an absent source to
recreate early decisions. `retention.Ledger.Active` reads only an admitted
current lifetime and creates no record.
<!-- source: pkg/plugin/rpc/enums.go -- ReasonPeerRemoved -->
<!-- source: internal/component/bgp/retention/retention.go -- Ledger, Active -->

## Design decisions

| Decision | It forbids | Source | Date |
|----------|------------|--------|------|
| Graceful Restart and Long-Lived Graceful Restart use symmetric `short-lived` and `long-lived` containers inside `graceful-restart`, both carrying keyed `list family` (`restart-time` inside `short-lived`; mandatory `stale-time` per family inside `long-lived`), with legacy `long-lived-stale-time`, top-level `restart-time`, and `family { name [...] }` removed | Putting `long-lived` outside `graceful-restart` (which would allow code 71 without code 64), using different `family` syntax between short-lived and long-lived, adding a `helper-only` flag when omitting `short-lived` already sends `0000`, or keeping a legacy fallback for `long-lived-stale-time` | 3c2a055fd9, plan/records/rfc-verdict-test-fix-pass-rulings.md | 2026-11-22 |
