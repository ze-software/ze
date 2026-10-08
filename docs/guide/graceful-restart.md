# Graceful Restart

Graceful Restart (RFC 4724) preserves forwarding state across BGP session restarts. When a peer goes down and comes back, routes are held during the restart window instead of being immediately withdrawn, preventing traffic black-holes.
<!-- source: internal/component/bgp/plugins/gr/register.go -- bgp-gr registration, RFCs 4724/9494 -->

## Configuration

```
plugin {
    internal gr {
        use bgp-gr
    }
    internal rib {
        use bgp-rib
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

        attach process gr {
            receive [ open state eor ]
            send [ update ]
        }
        attach process rib {
            receive [ update state refresh ]
            send [ update ]
        }
    }
}
```

### Config Reference

| Path | Type | Default | Description |
|------|------|---------|-------------|
| `graceful-restart / restart-time` | uint16 | 120 | Seconds to hold stale routes during restart (0-4095) |
| `graceful-restart / family / name` | leaf-list | -- | Address families the capability names. The container carries presence: absent, Ze names every family the peer carries; present and empty, Ze names none; present and filled, Ze names those |
<!-- source: internal/component/bgp/plugins/gr/yang/ -- ze-graceful-restart YANG schema -->

## How It Works

### What Ze Advertises

The code-64 capability Ze sends carries the Restart Time, then one
`<AFI, SAFI, Flags>` tuple for each address family the peer carries. A peer
configured for `ipv4/unicast` and `ipv6/unicast` with the default restart time
receives `0078 00010100 00020100`. The family list is the peer's own `family`
configuration, which the code-71 (LLGR) capability lists too. RFC 9494 Section
4.2 reads a family that code 64 omits as a Restart Time of zero.

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

The `graceful-restart family` container narrows that list, and it governs code
64 alone:

```
capability {
    graceful-restart {
        family {
            name ipv6/unicast
        }
    }
}
```

That peer receives `0078 00020100`, whatever else its `family` configuration
carries. Writing the container with no `name` in it makes Ze name no address
family at all, which RFC 4724 Section 3 reads as a speaker that runs the
Receiving Speaker procedures and preserves nothing of its own. Writing no
container is the default, and the default names every family the peer carries.
The container only narrows: a `name` the RIB plugin cannot store is left out
with the same warning, never added.

A `name` the peer's own `family` list does not carry is refused, because RFC
4724 Section 3 scopes a tuple to routes "advertised with the same AFI and SAFI"
and the session carries none of them. Ze names the family in the error. The
refusal runs at `config commit`, so the commit fails and the running router is
untouched; a configuration that reaches startup carrying one stops the daemon
rather than leaving it running without Graceful Restart.

The families the peer carries include the ones it inherits from its group, and
exclude any entry written `mode disable`. Both are what the session really
negotiates.

Narrowing code 64 is how an operator asks for LLGR with no conventional GR
phase. RFC 9494 Section 4.1: "the conventional GR phase can be skipped by
omitting all AFIs/SAFIs from the GR Capability, advertising a Restart Time of
zero, or both". So the container never touches the code-71 list.

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

1. **Peer goes down** -- GR plugin sends `retain-routes` with the received GR families plus exchanged LLGR families whose received LLST is nonzero
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

### Keep the bgp-gr engine in the daemon process

Load the plugin with `internal <name> { use bgp-gr; }`, as every example on this
page does. `run "ze plugin bgp-gr"` starts the plugin engine in a child process,
and that arrangement changes what the daemon sends to every peer.

The RFC 9494 egress filter runs in the daemon. The peer capability state it reads
is written by the plugin engine, so with `run` that state stays empty for the life
of the daemon process. The filter then treats every neighbor as a neighbor from
which the LLGR Capability was never received: it withdraws stale routes from
external peers (Section 4.3) and attaches NO_EXPORT with LOCAL_PREF 0 to the
routes it sends to internal peers (Section 4.6). Peers that negotiated LLGR with
the child engine get that same treatment.

`ze doctor` reports the arrangement as `doctor-bgp-gr-out-of-process`. Run
`ze explain doctor-bgp-gr-out-of-process` for the full text.
<!-- source: internal/component/bgp/plugins/gr/doctor.go -- checkGRInProcess, doctor-bgp-gr-out-of-process -->
<!-- source: internal/component/bgp/plugins/gr/gr_egress.go -- LLGREgressFilter reads egressState -->

The route-server DOWN handler also uses an in-process query owned by `bgp-gr`.
After draining that source's forwarding worker it asks which families are still
retained, and withdraws only the others. The query reads GR's existing family
state; it does not keep a second route inventory. No owner means no retention.
A child-process `bgp-gr` cannot publish that owner into the daemon, so this query
does **not** establish external-plugin LLGR support. Use internal `bgp-gr`,
`bgp-rib`, and `bgp-rs` for the coordinated lifecycle described here.
<!-- source: internal/component/bgp/retention/retention.go -- Publish, Family -->
<!-- source: internal/component/bgp/plugins/rs/server_handlers.go -- handleStateDown -->

## CLI

```
 $ ze cli -c "show bgp rib received"
# Shows routes with stale flag when applicable
```

## Long-Lived Graceful Restart (RFC 9494)

LLGR extends standard GR with a second, much longer stale period. When the GR restart-time expires without the peer reconnecting, instead of purging all stale routes, LLGR keeps them for up to ~194 days with reduced priority. The stale time a peer advertises can differ per family; the one Ze advertises is a single value for every family of the session.

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
| `graceful-restart / long-lived-stale-time` | uint32 | -- | LLST Ze advertises for every family of this peer (0-16777215, 24-bit); received peer values govern Ze's helper timers |

LLGR is off unless you configure it. Ze advertises the LLGR capability (code 71) in its OPEN only when `long-lived-stale-time` is configured for the peer, and then for every family the peer's session carries.

This is one peer-level leaf, not a per-family configuration list. The
`graceful-restart family` container selects code-64 tuples only; it does not
select LLGR families or assign different LLST values to them. Received code-71
tuples can still carry different times, and Ze tracks those timers by family.
<!-- source: internal/component/bgp/plugins/gr/yang/ze-graceful-restart.yang -- graceful-restart-config -->

LLGR is only active for a family when both OPENs of the session list it in their LLGR capability. A peer that advertises LLGR for a family Ze did not advertise it for gets the base GR treatment for that family: its routes are kept for the Restart Time and no longer (RFC 9494 Section 5 requires configuration per AFI/SAFI before the procedures run). LLGR also requires the GR capability (code 64) in the same OPEN: LLGR without GR is ignored, as RFC 9494 Section 4.5 requires.
<!-- source: internal/component/bgp/plugins/gr/gr_llgr_exchange.go -- exchangedLLGRLocked -->
<!-- source: internal/component/bgp/plugins/gr/gr_llgr.go -- extractLLGRCapabilities -->
<!-- source: internal/component/bgp/plugins/gr/gr.go -- forgetGRCapability -->
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

The LLST expiry uses the time in the **remote peer's received OPEN**, not the
`long-lived-stale-time` Ze advertises from its local configuration. Different
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
<!-- source: internal/component/bgp/plugins/rib/rib_sent_lifecycle.go -- reconcileSentSourceLocked, attachSentCommunity -->

State and End-of-RIB events run in reverse implementation-dependency order:
`bgp-gr` precedes `bgp-rib`, including when either is loaded under an operator
alias such as `gr` or `rib`. This lets GR request retention before the RIB
processes peer-down. An external program does not inherit a compiled-in
plugin's dependency edges merely by sharing its name.
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

### Readvertising a Stale Route

RFC 9494 Section 4.3 governs a stale route that ze forwards on to a neighbor from which the LLGR Capability was not received. The stale level on the ROUTE decides, not the restart state, so a route staled by `request bgp rib mark-stale` takes the same treatment as one staled by a peer restart.

| Destination | Action |
|-------------|--------|
| LLGR was received from it | Advertise unchanged |
| Internal, LLGR not received | Advertise with NO_EXPORT and LOCAL_PREF 0 (Section 4.6) |
| External, LLGR not received | Withdraw the route (Section 4.3) |

The filter fails closed. When it cannot read the peer capability state, it answers "LLGR was not received" for every destination. It raises one warning per process and then withdraws or depreferences. An unread state used to accept, which put a long-lived stale route into a neighbor that never agreed to hold one.
<!-- source: internal/component/bgp/plugins/gr/gr_egress.go -- LLGREgressFilter, egressState -->

The one arrangement that leaves the state permanently unread is `run "ze plugin bgp-gr"`. See [Keep the bgp-gr engine in the daemon process](#keep-the-bgp-gr-engine-in-the-daemon-process).

### Special Case: Skip GR

If `restart-time` is 0 but `long-lived-stale-time` is nonzero, the GR period is skipped entirely. On session drop, LLGR begins immediately, after the routes are retained and marked stale: the LLGR steps above run last, so the routes they mark LLGR-stale are the retained ones. When both times are 0 the peer's routes are released at once, as in base BGP.
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
