# BGP Finite State Machine

A walk-through of Ze's BGP FSM implementation, based on RFC 4271 Section 8.
For per-state runbooks and detailed wiring, see
[architecture/behavior/fsm.md](architecture/behavior/fsm.md).

## States

Ze implements the six states defined in RFC 4271. Each peer runs its own FSM in a
dedicated goroutine.
<!-- source: internal/component/bgp/fsm/state.go -- State, StateIdle..StateEstablished -->

| State | Value | Description |
|-------|-------|-------------|
| IDLE | 0x01 | Initial state, no connection |
| ACTIVE | 0x02 | Listening for incoming connection |
| CONNECT | 0x04 | Attempting outgoing connection |
| OPENSENT | 0x08 | OPEN sent, waiting for peer OPEN |
| OPENCONFIRM | 0x10 | OPEN received, waiting for KEEPALIVE |
| ESTABLISHED | 0x20 | Session established, exchanging routes |

## State Transitions

```
                ┌──────────────────────────────────────────┐
                │                                          │
                v                                          │
         ┌───────────┐                                     │
┌───────>│   IDLE    │<─────────────────────────────┐      │
│        └─────┬─────┘                              │      │
│              │                                    │      │
│              │ ManualStart                        │      │
│              v                                    │      │
│   ┌─────────────────────┐                         │      │
│   │                     │                         │      │
│   │  ┌────────┐   ┌─────┴────┐                    │      │
│   │  │CONNECT │   │ ACTIVE   │                    │      │
│   │  └───┬────┘   └────┬─────┘                    │      │
│   │      │              │                         │      │
│   │      │ TCP ok       │ TCP ok                  │      │
│   │      └──────┬───────┘                         │      │
│   │             │                                 │      │
│   │             v                                 │      │
│   │      ┌───────────┐                            │      │
│   │      │ OPENSENT  │─────── Error ──────────────┘      │
│   │      └─────┬─────┘                                   │
│   │            │                                         │
│   │            │ Receive OPEN                            │
│   │            v                                         │
│   │     ┌────────────┐                                   │
│   │     │OPENCONFIRM │─────── Error ─────────────────────┘
│   │     └──────┬─────┘
│   │            │
│   │            │ Receive KEEPALIVE
│   │            v
│   │     ┌────────────┐
│   └────>│ESTABLISHED │
│         └──────┬─────┘
│                │ Error / Notification
└────────────────┘
```
<!-- source: internal/component/bgp/fsm/fsm.go -- handleIdle, handleConnect, handleActive, handleOpenSent, handleOpenConfirm, handleEstablished -->

## Valid Transitions

| To State | Valid From |
|----------|-----------|
| IDLE | Any state (error/shutdown) |
| ACTIVE | IDLE, ACTIVE, OPENSENT |
| CONNECT | IDLE, CONNECT, ACTIVE |
| OPENSENT | CONNECT only |
| OPENCONFIRM | OPENSENT, OPENCONFIRM |
| ESTABLISHED | OPENCONFIRM, ESTABLISHED |

## Events

**ManualStart:** Peer configured and enabled. From IDLE, transitions to CONNECT
(active mode) or ACTIVE (passive mode).
<!-- source: internal/component/bgp/fsm/fsm.go -- handleIdle, EventManualStart -->

**TCP Connection Established:** TCP handshake completes. From CONNECT or ACTIVE,
sends OPEN message and transitions to OPENSENT.
<!-- source: internal/component/bgp/fsm/fsm.go -- handleConnect, handleActive, EventTCPConnectionConfirmed -->

**Receive OPEN:** Valid OPEN message received. From OPENSENT, validates
capabilities, sends KEEPALIVE, transitions to OPENCONFIRM.
<!-- source: internal/component/bgp/fsm/fsm.go -- handleOpenSent, EventBGPOpen -->

**Receive KEEPALIVE:** From OPENCONFIRM, transitions to ESTABLISHED. The session
is now exchanging routes.
<!-- source: internal/component/bgp/fsm/fsm.go -- handleOpenConfirm, EventKeepaliveMsg -->

**Error Events:** NOTIFICATION received, TCP error, or hold timer expired. From
any state, transitions to IDLE.
<!-- source: internal/component/bgp/fsm/state.go -- EventHoldTimerExpires, EventTCPConnectionFails, EventNotifMsg -->

## Timers

| Timer | Default | Purpose |
|-------|---------|---------|
| Connect Retry | 120s | Delay between connection attempts |
| Hold | Negotiated (min of local and peer, 0 disables) | Detect dead peer. Reset on KEEPALIVE/UPDATE received. |
| Keepalive | Hold / 3 | Send periodic KEEPALIVE messages |
| Open Wait | 60s | Timeout waiting for OPEN in OPENSENT |
| Send Hold | max(8min, 2x hold) | Detect inability to send (RFC 9687). Not configurable. |
<!-- source: internal/component/bgp/fsm/state.go -- EventHoldTimerExpires, EventKeepaliveTimerExpires, EventConnectRetryTimerExpires -->
<!-- source: internal/component/bgp/reactor/session_write.go -- sendHoldTimerExpired, Send Hold Timer (RFC 9687) -->

## Capability Negotiation

During the OPEN exchange, each side declares its capabilities. Ze negotiates:
ASN4, ADD-PATH (per-family send/receive/both), Extended Message, Extended Next
Hop (per-family NH mapping), Graceful Restart, Long-Lived GR, Route Refresh,
Enhanced Route Refresh, BGP Role, Hostname, Software Version, and Link-Local
Next Hop.

The negotiated capabilities are hashed into a `ContextID` (uint16). Peers with
the same ContextID share the same encoding rules, enabling zero-copy forwarding.
<!-- source: internal/core/bgp/capability/capability.go -- capability codes and parsing -->
<!-- source: internal/core/bgp/context/registry.go -- ContextID hashing -->

## Collision Detection

When both peers initiate a connection simultaneously (RFC 4271 Section 6.8):

1. Compare BGP Identifiers (router-id)
2. Higher ID keeps its outgoing connection
3. Lower ID's connection is dropped

If the existing session is in OpenConfirm, Ze reads the OPEN from the pending
connection to compare BGP IDs before deciding which connection to close.
<!-- source: internal/component/bgp/reactor/reactor_connection.go -- handlePendingCollision, acceptOrReject -->

## Connection Modes

Each peer has a `connection` setting:

| Mode | Behavior |
|------|----------|
| `active` | Dial out only |
| `passive` | Accept inbound only (RFC 4271 S8.1.1 PassiveTcpEstablishment) |
| `both` (default) | Both dial and accept. Collision detection resolves races. |
<!-- source: internal/component/bgp/reactor/peer_settings.go -- ConnectionMode -->

## Ze Implementation

Each peer's FSM runs as a goroutine with a `switch` on `fsm.state`. The FSM
notifies the reactor on ESTABLISHED transitions (triggering initial route sends)
and on session close (triggering peer-down events to plugins).
<!-- source: internal/component/bgp/reactor/peer.go -- Peer, StartWithContext -->
<!-- source: internal/component/bgp/reactor/peer_run.go -- run, runOnce, SetCallback closure -->
<!-- source: internal/component/bgp/reactor/reactor_notify.go -- notifyPeerEstablished, notifyPeerClosed -->

TCP sockets are tuned for BGP: `TCP_NODELAY` (messages are application-framed),
`DSCP CS6` (RFC 4271 S5.1 IP precedence for network control), and half-close on
shutdown to ensure the remote peer reads pending NOTIFICATIONs.
<!-- source: internal/component/bgp/reactor/session_connection.go -- connectionEstablished, closeConn -->

Hold timer expiry: Ze grants no reprieve. Every expiry runs the action list of
RFC 4271 Section 8.2.2, Event 10. Ze sends NOTIFICATION code 4 (Hold Timer
Expired) and stops the session. A CPU-congested Ze drops the session.
<!-- source: internal/component/bgp/reactor/session.go -- OnHoldTimerExpires callback -->

## Per-State Runbooks

For detailed per-state documentation (entry wiring, events handled, timers,
wire side effects, RFC deviations, tests):

| State | Runbook |
|-------|---------|
| Idle | [architecture/behavior/fsm-idle.md](architecture/behavior/fsm-idle.md) |
| Connect | [architecture/behavior/fsm-connect.md](architecture/behavior/fsm-connect.md) |
| Active | [architecture/behavior/fsm-active.md](architecture/behavior/fsm-active.md) |
| OpenSent | [architecture/behavior/fsm-open-sent.md](architecture/behavior/fsm-open-sent.md) |
| OpenConfirm | [architecture/behavior/fsm-open-confirm.md](architecture/behavior/fsm-open-confirm.md) |
| Established | [architecture/behavior/fsm-established.md](architecture/behavior/fsm-established.md) |
| Peer lifecycle | [architecture/behavior/peer-lifecycle.md](architecture/behavior/peer-lifecycle.md) |

## RFC References

- **RFC 4271** Section 8: BGP FSM specification. See [rfc/short/rfc4271.md](../rfc/short/rfc4271.md).
- **RFC 9687**: Send Hold Timer. Ze implements this as `max(8min, 2x hold-time)`.
- **RFC 5082**: TTL Security / GTSM for session protection.
- **RFC 2385**: TCP MD5 authentication.

## Further Reading

| Topic | Document |
|-------|----------|
| Architecture overview | [architecture.md](architecture.md) |
| Full design document | [DESIGN.md](DESIGN.md) |
| Wire format: messages | [architecture/wire/messages.md](architecture/wire/messages.md) |
| Wire format: capabilities | [architecture/wire/capabilities.md](architecture/wire/capabilities.md) |
