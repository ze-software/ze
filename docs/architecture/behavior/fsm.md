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

### PeerLifecycleObserver

Reactor maintains a list of observers notified on state changes:

```go
type PeerLifecycleObserver interface {
    OnPeerEstablished(peer *Peer)
    OnPeerClosed(peer *Peer, reason string)
}

// Register observer
reactor.AddPeerObserver(observer)
```

The `apiStateObserver` is registered automatically when API server starts, emitting state messages to external processes.
<!-- source: internal/component/bgp/reactor/reactor_notify.go -- PeerLifecycleObserver, AddPeerObserver -->

**See:** `docs/architecture/api/ARCHITECTURE.md` for full details.

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
