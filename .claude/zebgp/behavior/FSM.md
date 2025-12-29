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

---

## Events

### ManualStart

- **Trigger:** Peer configured and enabled
- **From:** IDLE
- **To:** CONNECT (active) or ACTIVE (passive)

### TCP Connection Established

- **Trigger:** TCP handshake complete
- **From:** CONNECT or ACTIVE
- **To:** OPENSENT (after sending OPEN)

### Receive OPEN

- **Trigger:** Valid OPEN message received
- **From:** OPENSENT
- **To:** OPENCONFIRM (after sending KEEPALIVE)

### Receive KEEPALIVE

- **Trigger:** KEEPALIVE received in OPENCONFIRM
- **From:** OPENCONFIRM
- **To:** ESTABLISHED

### Error Events

- **Trigger:** NOTIFICATION, TCP error, hold timer expired
- **From:** Any
- **To:** IDLE

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

---

## ZeBGP Implementation Notes

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

type FSM struct {
    peer  *Peer
    state State
}

func (f *FSM) Change(newState State) {
    f.state = newState
    if f.peer.neighbor.API.FSM {
        f.peer.reactor.processes.FSM(f.peer.neighbor, f)
    }
}
```

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

---

**Last Updated:** 2025-12-19
