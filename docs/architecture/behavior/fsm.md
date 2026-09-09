# BGP Finite State Machine

## TL;DR (Read This First)

| Concept | Description |
|---------|-------------|
| **States** | IDLE→ACTIVE→CONNECT→OPENSENT→OPENCONFIRM→ESTABLISHED |
| **Timers** | Connect retry (120s), Hold (negotiated), Keepalive (hold/3) |
| **Collision** | Higher router-id wins when both peers initiate |
| **Pattern** | Goroutine-per-peer, switch on `fsm.state` |
| **Key Types** | `State`, `FSM`, `Peer.Run()` |

**When to read full doc:** Connection lifecycle, timer logic, collision detection.
<!-- source: internal/component/bgp/fsm/state.go -- State, StateIdle..StateEstablished -->
<!-- source: internal/component/bgp/fsm/fsm.go -- FSM, Event() -->

## Per-state runbooks

For "what happens in state X" questions, go straight to the per-state
runbook. Each page indexes every file that participates when the FSM is in
that state: entry wiring, events handled, timers running, wire side
effects, RFC deviations, and tests.

| State | Runbook |
|-------|---------|
| Idle | [fsm-idle.md](fsm-idle.md) |
| Connect | [fsm-connect.md](fsm-connect.md) |
| Active | [fsm-active.md](fsm-active.md) |
| OpenSent | [fsm-open-sent.md](fsm-open-sent.md) |
| OpenConfirm | [fsm-open-confirm.md](fsm-open-confirm.md) |
| Established | [fsm-established.md](fsm-established.md) |

For the outer peer run loop that sits around the FSM (reconnect
backoff, panic recovery, collision resolution, inbound connection
buffering, and the replacement for RFC 4271's `ConnectRetryTimer`), see
[peer-lifecycle.md](peer-lifecycle.md).

---

**Source:** ExaBGP `bgp/fsm.py`, `reactor/peer/`
**Reference:** RFC 4271 Section 8

---

## States

| State | Value | Description |
|-------|-------|-------------|
| IDLE | 0x01 | Initial state, no connection |
| ACTIVE | 0x02 | Listening for incoming connection |
| CONNECT | 0x04 | Attempting outgoing connection |
| OPENSENT | 0x08 | OPEN sent, waiting for peer OPEN |
| OPENCONFIRM | 0x10 | OPEN received, waiting for KEEPALIVE |
| ESTABLISHED | 0x20 | Session established, exchanging routes |
<!-- source: internal/component/bgp/fsm/state.go -- StateIdle=0x01, StateActive=0x02, StateConnect=0x04, StateOpenSent=0x08, StateOpenConfirm=0x10, StateEstablished=0x20 -->

---

## State Transitions

```
                    ┌─────────────────────────────────────────────┐
                    │                                             │
                    ▼                                             │
            ┌───────────┐                                         │
   ┌───────▶│   IDLE    │◀──────────────────────────────────┐     │
   │        └─────┬─────┘                                   │     │
   │              │                                         │     │
   │              │ ManualStart                             │     │
   │              ▼                                         │     │
   │  ┌───────────────────────┐                            │     │
   │  │                       │                            │     │
   │  │  ┌─────────┐    ┌─────┴─────┐                      │     │
   │  │  │ CONNECT │    │  ACTIVE   │                      │     │
   │  │  └────┬────┘    └─────┬─────┘                      │     │
   │  │       │               │                            │     │
   │  │       │ TCP Connected │ TCP Connected              │     │
   │  │       │               │                            │     │
   │  │       └───────┬───────┘                            │     │
   │  │               │                                    │     │
   │  │               ▼                                    │     │
   │  │        ┌───────────┐                               │     │
   │  │        │ OPENSENT  │──────── Error ────────────────┘     │
   │  │        └─────┬─────┘                                     │
   │  │              │                                           │
   │  │              │ Receive OPEN                              │
   │  │              ▼                                           │
   │  │       ┌────────────┐                                     │
   │  │       │OPENCONFIRM │──────── Error ──────────────────────┘
   │  │       └──────┬─────┘
   │  │              │
   │  │              │ Receive KEEPALIVE
   │  │              ▼
   │  │       ┌────────────┐
   │  └──────▶│ESTABLISHED │
   │          └──────┬─────┘
   │                 │
   │                 │ Error / Notification
   └─────────────────┘
```

---

## Transition Table

From `fsm.py`:

```python
transition = {
    IDLE:        [IDLE, ACTIVE, CONNECT, OPENSENT, OPENCONFIRM, ESTABLISHED],
    ACTIVE:      [IDLE, ACTIVE, OPENSENT],
    CONNECT:     [IDLE, CONNECT, ACTIVE],
    OPENSENT:    [CONNECT],
    OPENCONFIRM: [OPENSENT, OPENCONFIRM],
    ESTABLISHED: [OPENCONFIRM, ESTABLISHED],
}
```

| To State | Valid From States |
|----------|-------------------|
| IDLE | Any state (error/shutdown) |
| ACTIVE | IDLE, ACTIVE, OPENSENT |
| CONNECT | IDLE, CONNECT, ACTIVE |
| OPENSENT | CONNECT only |
| OPENCONFIRM | OPENSENT, OPENCONFIRM |
| ESTABLISHED | OPENCONFIRM, ESTABLISHED |
<!-- source: internal/component/bgp/fsm/fsm.go -- handleIdle, handleConnect, handleActive, handleOpenSent, handleOpenConfirm, handleEstablished -->

---

## Events

### ManualStart

- **Trigger:** Peer configured and enabled
- **From:** IDLE
- **To:** CONNECT (active) or ACTIVE (passive)
<!-- source: internal/component/bgp/fsm/fsm.go -- handleIdle, EventManualStart -->

### TCP Connection Established

- **Trigger:** TCP handshake complete
- **From:** CONNECT or ACTIVE
- **To:** OPENSENT (after sending OPEN)
<!-- source: internal/component/bgp/fsm/fsm.go -- handleConnect, handleActive, EventTCPConnectionConfirmed -->

### Receive OPEN

- **Trigger:** Valid OPEN message received
- **From:** OPENSENT
- **To:** OPENCONFIRM (after sending KEEPALIVE)
<!-- source: internal/component/bgp/fsm/fsm.go -- handleOpenSent, EventBGPOpen -->

### Receive KEEPALIVE

- **Trigger:** KEEPALIVE received in OPENCONFIRM
- **From:** OPENCONFIRM
- **To:** ESTABLISHED
<!-- source: internal/component/bgp/fsm/fsm.go -- handleOpenConfirm, EventKeepaliveMsg -->

### Error Events

- **Trigger:** NOTIFICATION, TCP error, hold timer expired
- **From:** Any
- **To:** IDLE
<!-- source: internal/component/bgp/fsm/state.go -- EventHoldTimerExpires, EventTCPConnectionFails, EventNotifMsg -->

### Optional events ze implements

RFC 4271 Section 8.1.2 and Section 8.1.5 mark these three optional. Ze
implements them because the ConnectRetryCounter below cannot be correct
without them: each carries a counter clause that differs from the mandatory
event it would otherwise be folded into.

| Event | Ze producer | Why it is not the mandatory event beside it |
|-------|-------------|---------------------------------------------|
| Event 6, `AutomaticStart_with_DampPeerOscillations` | `Session.StartDamped`, from every reconnect cycle after the first | Event 1 (ManualStart) sets the ConnectRetryCounter to zero. Ze fires a start event per cycle because each cycle builds a new FSM, so Event 1 there would zero the counter on every retry |
| Event 8, `AutomaticStop` | `Session.TeardownAutomatic`, from a BFD session going down and from a forward-pool out-of-resources drop | Event 2 (ManualStop) zeroes the counter; Event 8 increments it. The operator did not ask for this stop |
| Event 23, `OpenCollisionDump` | `Session.CloseWithNotification`, whose only caller is RFC 4271 Section 6.8 collision resolution | Same difference as Event 8: a connection lost to a collision is an attempt that failed |

<!-- source: internal/component/bgp/fsm/state.go -- EventAutomaticStartWithDampPeerOscillations, EventAutomaticStop, EventOpenCollisionDump -->
<!-- source: internal/component/bgp/reactor/session_connection.go -- TeardownAutomatic, CloseWithNotification -->

### The six BFD strict-mode events

draft-ietf-idr-bgp-bfd-strict-mode Section 4 adds six events, registered as FSM
events 30 to 35 by its Section 13.3. Each is Optional in the draft's own words
and each is implemented, because ze runs the strict-mode procedures of Section 8.

| Event | Ze name | Ze producer | What it does |
|-------|---------|-------------|--------------|
| 30 `BfdAdminDown` | `EventBfdAdminDown` | `Peer.runBFDSubscriber` | Releases a pending sub-state; ignored everywhere else. AdminDown says nothing about the data path (RFC 5882 Section 3.2) |
| 31 `BfdDown` | `EventBfdDown` | `Peer.runBFDSubscriber` | Closes the session with Cease / BFD Down in Established always, and in OpenSent or OpenConfirm when strict mode is negotiated. Ignored in Connect and Active |
| 32 `BfdUp` | `EventBfdUp` | `Peer.runBFDSubscriber` | Releases a pending sub-state: the withheld KEEPALIVE goes out and the session advances |
| 33 `BfdDisabled` | `EventBfdDisabled` | `Session.raiseBFDStrictConfigChanged`, when BFD was turned off | Same release as event 30 |
| 34 `BfdHoldTimerExpires` | `EventBfdHoldTimerExpires` | the BfdHoldTimer below | Closes the session with Cease / BFD Down and increments the ConnectRetryCounter |
| 35 `BfdStrictConfigChanged` | `EventBfdStrictConfigChanged` | `Session.raiseBFDStrictConfigChanged`, from the reload path | Drops every pre-Established state to Idle, with Cease / Other Configuration Change from OpenSent and OpenConfirm. Ignored in Established |

The ConnectRetryCounter directions are the draft's and they differ by state.
Event 31 ZEROES the counter in OpenSent and OpenConfirm (Sections 8.5.2 and
8.6.2) and INCREMENTS it in Established (Section 8.7.2). Event 34 increments
(Sections 8.3.3, 8.4.3, 8.5.3) and event 35 zeroes.

<!-- source: internal/component/bgp/fsm/state.go -- EventBfdAdminDown, EventBfdDown, EventBfdUp, EventBfdDisabled, EventBfdHoldTimerExpires, EventBfdStrictConfigChanged -->
<!-- source: internal/component/bgp/reactor/peer_bfd.go -- runBFDSubscriber, bfdEventFor -->
<!-- source: internal/component/bgp/reactor/session_bfd_strict.go -- handleBFDEvent, bfdTeardown -->

### The two BFD strict-mode sub-states

draft-ietf-idr-bgp-bfd-strict-mode Section 8.1 names four sub-states. Ze declares
the two that sit inside OpenSent. The other two,
`ConnectDelayOpenBfdUpPending` and `ActiveDelayOpenBfdUpPending`, are entered
only by Event 20, an OPEN received while the DelayOpenTimer runs, and ze
implements no DelayOpenTimer (permitted by RFC 4271 Section 8.2.1.3).

| Sub-state | Entered when | Left by |
|-----------|--------------|---------|
| `OpenSentBfdUpPending` | the peer's OPEN arrives, strict mode is negotiated and the BFD session is neither Up nor AdminDown. The KEEPALIVE is withheld (Section 8.5.5) | events 30, 32 or 33: KEEPALIVE sent, state becomes OpenConfirm |
| `OpenSentConfirmedBfdUpPending` | the peer's KEEPALIVE arrives while the wait is on. The remote BFD session can come Up first (Section 8.5.6) | events 30, 32 or 33: KEEPALIVE sent, state becomes Established |

A sub-state is cleared by any transition out of OpenSent, in `FSM.change`, so no
handler can leave one behind. `Peer.bfdSubState` reads `FSM.BfdSubState` and
`show bgp peer list` and `show bgp peer detail` render it as `bfd-sub-state`, which is the
visibility Section 11 asks for. The key is written only while a session is
waiting, so a peer that is not carries none.

<!-- source: internal/component/bgp/reactor/peer_bfd.go -- bfdSubState -->
<!-- source: internal/component/bgp/plugins/cmd/peer/peer.go -- handleBgpPeerDetail, fieldBFDSubState -->

<!-- source: internal/component/bgp/fsm/state.go -- BfdSubState, SubStateOpenSentBfdUpPending, SubStateOpenSentConfirmedBfdUpPending -->
<!-- source: internal/component/bgp/fsm/fsm.go -- EnterBfdUpPending, handleOpenSent -->

---

## ConnectRetryCounter

RFC 4271 Section 8.1.1 makes the ConnectRetryCounter a MANDATORY session
attribute and defines it as "the number of times a BGP peer has tried to
establish a peer session". Section 8.2.2 says exactly when it moves: a small
set of events set it to zero, and every teardown the RFC counts as a failed
attempt increments it by one.

### Where it lives, and why not in the FSM

The RFC keeps one FSM per peer for the life of that peer. Ze does not:
`Peer.runOnce` builds a new `Session`, and therefore a new `FSM`, on every
connection cycle, because the peer-level reconnect loop replaces the RFC's
ConnectRetryTimer. A counter held in FSM state would reset on every retry and
could never count one.

So the counter is its own type, `fsm.ConnectRetryCounter`, the `Peer` owns the
value, and `runOnce` hands the same pointer to each cycle's FSM through
`FSM.SetConnectRetryCounter`. The FSM handlers own every mutation; nothing
else writes it.

<!-- source: internal/component/bgp/fsm/connect_retry_counter.go -- ConnectRetryCounter -->
<!-- source: internal/component/bgp/reactor/peer_run.go -- runOnce -->

### When it moves

| Direction | Events | Note |
|-----------|--------|------|
| set to zero | Event 1 (ManualStart) in Idle; Event 2 (ManualStop) in every other state | Only the operator ends a retry history |
| increment by 1 | Events 8, 10, 18, 21, 22, 23, 24, 25, 28, and each state's "any other event" arm | Which events apply is per state, not global |
| unchanged | Event 6 (damped restart), Events 26 and 27 (KEEPALIVE, UPDATE), Event 19 in OpenSent, Event 11, and every event in Idle | The success path and the ignores |

Three of those are per-state rather than uniform, and the FSM handlers say so
in each arm:

- Event 24 (NotifMsgVerErr) increments in Connect, Active and Established, and
  does NOT in OpenSent or OpenConfirm, where its action list has no counter
  line.
- Event 18 (TcpConnectionFails) increments in Active, OpenConfirm and
  Established, and does NOT in Connect or OpenSent.
- Event 10 (HoldTimer_Expires) applies only where the timer can be armed:
  OpenSent, OpenConfirm and Established.

<!-- source: internal/component/bgp/fsm/fsm.go -- handleIdle, handleConnect, handleActive, handleOpenSent, handleOpenConfirm, handleEstablished -->

### Reading it

`show bgp peer <address> detail` carries it as `connect-retry-counter`, and
Prometheus publishes it as the `ze_bgp_connect_retry_counter` gauge
(`docs/guide/monitoring.md`). It is a gauge rather than a counter because the
zeroing clauses make the value go down, which a Prometheus counter may not do.

<!-- source: internal/component/bgp/plugins/cmd/peer/peer.go -- handleBgpPeerDetail -->
<!-- source: internal/component/bgp/reactor/reactor_metrics.go -- updatePeriodicMetrics -->

---

## Timers

### Connect Retry Timer

- **Purpose:** Delay between connection attempts
- **Default:** 120 seconds
- **Behavior:** Start on IDLE, fire triggers reconnect

### Hold Timer

- **Purpose:** Detect dead peer
- **Negotiated:** Min of local and peer hold time (0 disables)
- **Behavior:** Reset on KEEPALIVE/UPDATE received

### Keepalive Timer

- **Purpose:** Send periodic KEEPALIVEs
- **Value:** Hold Timer / 3
- **Behavior:** Fire sends KEEPALIVE

### Open Wait Timer

- **Purpose:** Timeout waiting for OPEN
- **Default:** 60 seconds (`exabgp.bgp.openwait`)
- **Behavior:** Fire in OPENSENT triggers disconnect

### BFD Hold-Down Timer

- **Purpose:** Delay the release until the BFD session has been Up for the configured interval (draft-ietf-idr-bgp-bfd-strict-mode Section 10)
- **Default:** 0, meaning no hold-down, set by the peer's `connection bfd { hold-down }` leaf in milliseconds
- **Behavior:** A BFD Up arms it instead of releasing the pending sub-state; the expiry re-enters the release. `EventBfdAdminDown` and `EventBfdDisabled` skip it, because neither says the session has been Up for any time at all

### BFD Hold Timer

- **Purpose:** Bound the strict-mode wait for BFD when nothing else does
- **Default:** 30 seconds (draft-ietf-idr-bgp-bfd-strict-mode Section 3, attribute 18), set by the peer's `connection bfd { hold-time }` leaf
- **Behavior:** Started only when the NEGOTIATED BGP hold time is zero, because a non-zero one already bounds the wait: `Session.advanceAfterOpen` re-arms the ordinary HoldTimer to the negotiated value on the way into the wait, and RFC 4271 Section 8.2.2 Event 10 in OpenSent then ends it. Its expiry closes the session with Cease / BFD Down. Stopped on any transition to Idle and when the wait ends
<!-- source: internal/component/bgp/fsm/state.go -- EventHoldTimerExpires, EventKeepaliveTimerExpires, EventConnectRetryTimerExpires -->

---

## ExaBGP Implementation

### FSM Class

```python
class FSM:
    class STATE(IntEnum):
        IDLE = 0x01
        ACTIVE = 0x02
        CONNECT = 0x04
        OPENSENT = 0x08
        OPENCONFIRM = 0x10
        ESTABLISHED = 0x20

    def __init__(self, peer: Peer, state: STATE) -> None:
        self.peer = peer
        self.state = state

    def change(self, state: STATE) -> FSM:
        self.state = state
        # Notify API if configured
        if self.peer.neighbor.api and self.peer.neighbor.api['fsm']:
            self.peer.reactor.processes.fsm(self.peer.neighbor, self)
        return self

    def __eq__(self, other) -> bool:
        return self.state == other
```

### API Notification

FSM changes can be reported via API:

```json
{
  "exabgp": "6.0.0",
  "type": "fsm",
  "neighbor": {
    "address": { ... },
    "state": "ESTABLISHED"
  }
}
```

---

## Peer Loop (Simplified)

```python
async def peer_loop(self):
    while True:
        if self.fsm == FSM.IDLE:
            await self.connect()  # -> CONNECT or ACTIVE

        elif self.fsm == FSM.CONNECT:
            await self.tcp_connect()
            if connected:
                await self.send_open()
                self.fsm.change(FSM.OPENSENT)

        elif self.fsm == FSM.OPENSENT:
            msg = await self.receive()
            if msg.type == OPEN:
                self.process_open(msg)
                await self.send_keepalive()
                self.fsm.change(FSM.OPENCONFIRM)

        elif self.fsm == FSM.OPENCONFIRM:
            msg = await self.receive()
            if msg.type == KEEPALIVE:
                self.fsm.change(FSM.ESTABLISHED)

        elif self.fsm == FSM.ESTABLISHED:
            msg = await self.receive()
            self.process_message(msg)
            # Stay in ESTABLISHED until error
```

---

## Collision Detection

When both peers initiate connection:

1. Compare BGP Identifiers (router-id)
2. Higher ID wins
3. Loser's connection is dropped

```python
def check_collision(self, remote_id):
    if remote_id < self.local_id:
        # We win, drop incoming connection
        return False
    else:
        # They win, drop our outgoing connection
        self.close_outgoing()
        return True
```
<!-- source: internal/component/bgp/reactor/reactor_connection.go -- handleConnection, collision detection -->

---

## Ze Implementation Notes

### FSM Type

```go
type State int

const (
    StateIdle        State = 0x01
    StateActive      State = 0x02
    StateConnect     State = 0x04
    StateOpenSent    State = 0x08
    StateOpenConfirm State = 0x10
    StateEstablished State = 0x20
)
```
<!-- source: internal/component/bgp/fsm/state.go -- State, StateIdle..StateEstablished -->

### Reactor Notification

The FSM callback in `peer.go` notifies the reactor on Established transitions:

```go
session.fsm.SetCallback(func(from, to fsm.State) {
    if to == fsm.StateEstablished {
        // ... set negotiated capabilities ...
        if reactor != nil {
            reactor.notifyPeerEstablished(p)
        }
        go p.sendInitialRoutes()
    } else if from == fsm.StateEstablished {
        reason := "session closed"
        if to == fsm.StateIdle {
            reason = "connection lost"
        }
        if reactor != nil {
            reactor.notifyPeerClosed(p, reason)
        }
        // ... clear capabilities ...
    }
})
```
<!-- source: internal/component/bgp/reactor/peer.go -- SetCallback on fsm -->
<!-- source: internal/component/bgp/reactor/reactor_notify.go -- notifyPeerEstablished, notifyPeerClosed -->

### Peer lifecycle observers

Reactor maintains a list of observers notified on state changes. The interface
and its registration are both internal to the reactor package:

```go
type peerLifecycleObserver interface {
    OnPeerEstablished(peer *Peer)
    OnPeerClosed(peer *Peer, reason string)
}

// Register an in-package observer
reactor.addPeerObserver(observer)
```

A plugin outside the package registers through `AddPeerLifecycleCallback`, which
wraps a `registry.PeerLifecycleCallback` in `callbackAdapter`.

The `apiStateObserver` is registered by `startAPIServer`, emitting state messages to external processes.
<!-- source: internal/component/bgp/reactor/reactor.go -- peerLifecycleObserver -->
<!-- source: internal/component/bgp/reactor/reactor_notify.go -- addPeerObserver, AddPeerLifecycleCallback, callbackAdapter -->
<!-- source: internal/component/bgp/reactor/reactor_api.go -- apiStateObserver -->

**See:** `docs/architecture/api/architecture.md` for full details.

### State Machine Goroutine

```go
func (p *Peer) Run(ctx context.Context) error {
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
        }

        switch p.fsm.state {
        case StateIdle:
            p.connect()
        case StateConnect:
            p.tcpConnect()
        case StateOpenSent:
            p.waitForOpen()
        case StateOpenConfirm:
            p.waitForKeepalive()
        case StateEstablished:
            p.processMessages()
        }
    }
}
```
<!-- source: internal/component/bgp/reactor/peer.go -- PeerState, PeerStateStopped..PeerStateEstablished -->
<!-- source: internal/component/bgp/fsm/fsm.go -- FSM.Event(), handleIdle..handleEstablished -->

---

**Last Updated:** 2026-01-03
