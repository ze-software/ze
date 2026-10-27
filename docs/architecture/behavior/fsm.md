# BGP Finite State Machine

## TL;DR (Read This First)

| Concept | Description |
|---------|-------------|
| **States** | IDLE→ACTIVE→CONNECT→OPENSENT→OPENCONFIRM→ESTABLISHED |
| **Timers** | Connect retry (120s), optional DelayOpen (0..240s), Hold (negotiated), Keepalive (hold/3) |
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
| OPENSENT | CONNECT or ACTIVE after immediate OPEN or DelayOpen expiry |
| OPENCONFIRM | OPENSENT, or CONNECT/ACTIVE after OPEN received during DelayOpen |
| ESTABLISHED | OPENCONFIRM, ESTABLISHED |
<!-- source: internal/component/bgp/fsm/fsm.go -- handleIdle, handleConnect, handleActive, handleOpenSent, handleOpenConfirm, handleEstablished -->

---

## Events

### ManualStart

- **Trigger:** Peer configured and enabled
- **From:** IDLE
- **To:** CONNECT (active) or ACTIVE (passive)
<!-- source: internal/component/bgp/fsm/fsm.go -- handleIdle, EventManualStart -->

If ManualStart (Event 1), AutomaticStart (Event 3) or
AutomaticStart_with_DampPeerOscillations (Event 6) is delivered in OpenSent, OpenConfirm or Established, the FSM
ignores it under RFC 4271 section 8.2.2. It returns no error, changes no
state or ConnectRetryCounter, and invokes no state-change callback.
Normal startup constructs a fresh Idle FSM; this rule describes the
handler contract, not a claim that ordinary startup delivers starts in
these later states.
<!-- source: internal/component/bgp/fsm/fsm.go -- handleOpenSent, handleOpenConfirm, handleEstablished -->

### TCP Connection Established

- **Trigger:** TCP handshake complete
- **From:** CONNECT or ACTIVE
- **To:** OPENSENT after sending OPEN when delay is zero; otherwise remain in
  CONNECT or ACTIVE with DelayOpenTimer running and no OPEN sent
<!-- source: internal/component/bgp/fsm/fsm.go -- handleConnect, handleActive, EventTCPConnectionConfirmed -->

### Receive OPEN

- **Trigger:** Valid OPEN message received
- **From:** OPENSENT, or CONNECT/ACTIVE while DelayOpenTimer runs
- **To:** OPENCONFIRM (OPEN then KEEPALIVE for the delayed path)
<!-- source: internal/component/bgp/fsm/fsm.go -- handleOpenSent -->
<!-- source: internal/component/bgp/fsm/delay_open.go -- handleDelayOpen -->

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

RFC 4271 Section 8.1.2 and Section 8.1.5 mark these four optional. Ze
implements them because the ConnectRetryCounter below cannot be correct
without them: each carries a counter clause that differs from the mandatory
event it would otherwise be folded into.

Every reconnect cycle after the operator's first start is automatic, and the
reconnect loop chooses between Event 3 and Event 6 from one fact: whether the
cycle that just ended reached Established. The FSM state callback in
`runOnce` sets `Peer.cycleReachedEstablished` beside `PeerStateEstablished`,
and the next `runOnce` swaps it back to false before its start event, whatever
error ended the previous cycle. That is Ze's DampPeerOscillations attribute:
FALSE after a session that ran (RFC 4271 Section 8.1.2 says of Event 3 "If the
DampPeerOscillations is supported, it SHOULD be set to FALSE when this event
occurs"), TRUE after a failed attempt, when the reconnect loop's exponential
backoff is the damping. No config leaf sets it, because the answer is a fact
about the last cycle and not a preference.

| Event | Ze producer | Why it is not the mandatory event beside it |
|-------|-------------|---------------------------------------------|
| Event 3, `AutomaticStart` | `Session.startAutomatic`, from a reconnect cycle whose previous cycle reached Established | Event 3 runs Event 1's Idle action list, so it sets the ConnectRetryCounter to zero too: a session that reached Established ended the run of failures the counter recorded. It differs from Event 1 in who started it, which is the system |
| Event 6, `AutomaticStart_with_DampPeerOscillations` | `Session.startDamped`, from a reconnect cycle whose previous cycle never reached Established | Event 1 (ManualStart) sets the ConnectRetryCounter to zero. Ze fires a start event per cycle because each cycle builds a new FSM, so Event 1 there would zero the counter on every retry |
| Event 8, `AutomaticStop` | `Session.teardownAutomatic`, from a BFD session going down and from a forward-pool out-of-resources drop | Event 2 (ManualStop) zeroes the counter; Event 8 increments it. The operator did not ask for this stop |
| Event 23, `OpenCollisionDump` | `Session.CloseWithNotification`, whose only caller is RFC 4271 Section 6.8 collision resolution | Same difference as Event 8: a connection lost to a collision is an attempt that failed |

<!-- source: internal/component/bgp/fsm/state.go -- EventAutomaticStart, EventAutomaticStartWithDampPeerOscillations, EventAutomaticStop, EventOpenCollisionDump -->
<!-- source: internal/component/bgp/reactor/peer_run.go -- runOnce -->

The `plugin.PeerInfo` that `runOnce` builds for the session's wire observers carries the peer's AS as `rpc.RemoteAS`: a dynamic peer's settings hold 0 until its OPEN is negotiated, and `rpc.RemoteASOf` turns that 0 into the not-yet-known state, so an observer of the OPEN exchange sees `remote as unknown` (text) or no `remote.as` (JSON), never AS 0. See [peer lifecycle](peer-lifecycle.md).
<!-- source: internal/component/bgp/reactor/session_connection.go -- teardownAutomatic, CloseWithNotification -->

### The six BFD strict-mode events

draft-ietf-idr-bgp-bfd-strict-mode Section 4 adds six events, registered as FSM
events 30 to 35 by its Section 13.3. Each is Optional in the draft's own words
and each is implemented, because ze runs the strict-mode procedures of Section 8.

| Event | Ze name | Ze producer | What it does |
|-------|---------|-------------|--------------|
| 30 `BfdAdminDown` | `EventBfdAdminDown` | `Peer.runBFDSubscriber`; `Peer.endVanishedBFDSession` when the BFD engine stops and closes the subscription; `Session.raiseBFDStrictConfigChanged` when the peer's BFD block is absent or disabled | Releases a pending sub-state; ignored everywhere else. AdminDown says nothing about the data path (RFC 5882 Section 3.2) |
| 31 `BfdDown` | `EventBfdDown` | `Peer.runBFDSubscriber` | Closes the session with Cease / BFD Down in Established always, and in OpenSent or OpenConfirm when strict mode is negotiated. Ignored in Connect and Active |
| 32 `BfdUp` | `EventBfdUp` | `Peer.runBFDSubscriber` | Releases a pending sub-state: the withheld KEEPALIVE goes out and the session advances |
| 33 `BfdDisabled` | `EventBfdDisabled` | No production producer; the handler is exercised directly by tests. Disabling BFD through `Session.raiseBFDStrictConfigChanged` produces event 30, as Section 4 requires | Same release as event 30 |
| 34 `BfdHoldTimerExpires` | `EventBfdHoldTimerExpires` | the BfdHoldTimer below | Closes the session with Cease / BFD Down and increments the ConnectRetryCounter |
| 35 `BfdStrictConfigChanged` | `EventBfdStrictConfigChanged` | `Session.raiseBFDStrictConfigChanged`, from the reload path | Drops every pre-Established state to Idle, with Cease / Other Configuration Change from OpenSent and OpenConfirm. Ignored in Established |

The ConnectRetryCounter directions are the draft's and they differ by state.
Event 31 ZEROES the counter in OpenSent and OpenConfirm (Sections 8.5.2 and
8.6.2) and INCREMENTS it in Established (Section 8.7.2). Event 34 increments
(Sections 8.3.3, 8.4.3, 8.5.3) and event 35 zeroes.

<!-- source: internal/component/bgp/fsm/state.go -- EventBfdAdminDown, EventBfdDown, EventBfdUp, EventBfdDisabled, EventBfdHoldTimerExpires, EventBfdStrictConfigChanged -->
<!-- source: internal/component/bgp/reactor/peer_bfd.go -- runBFDSubscriber, bfdEventFor, endVanishedBFDSession -->
<!-- source: internal/component/bgp/reactor/session_bfd_strict.go -- handleBFDEvent, bfdTeardown, raiseBFDStrictConfigChanged -->

### The four BFD strict-mode sub-states

draft-ietf-idr-bgp-bfd-strict-mode Section 8.1 names four sub-states. Ze uses
the two DelayOpen sub-states when an OPEN arrives before the configured delay
expires and negotiated strict mode is waiting for BFD. DelayOpen zero preserves
the OpenSent path.

| Sub-state | Entered when | Left by |
|-----------|--------------|---------|
| `OpenSentBfdUpPending` | the peer's OPEN arrives, strict mode is negotiated and the BFD session is neither Up nor AdminDown. The KEEPALIVE is withheld (Section 8.5.5) | events 30, 32 or 33: KEEPALIVE sent, state becomes OpenConfirm |
| `OpenSentConfirmedBfdUpPending` | the peer's KEEPALIVE arrives while the wait is on. The remote BFD session can come Up first (Section 8.5.6) | events 30, 32 or 33: KEEPALIVE sent, state becomes Established |
| `ConnectDelayOpenBfdUpPending` | Event 20 in Connect sends OPEN, but BFD is neither Up nor AdminDown; KEEPALIVE is withheld (Section 8.3.5) | events 30, 32 or 33: KEEPALIVE sent, state becomes OpenConfirm |
| `ActiveDelayOpenBfdUpPending` | the corresponding accepted-connection Event 20 in Active (Section 8.4.5) | events 30, 32 or 33: KEEPALIVE sent, state becomes OpenConfirm |

A sub-state is cleared by any transition out of its parent state in `FSM.change`,
so no handler can leave one behind. `Peer.bfdSubState` reads `FSM.BfdSubState` and
`show bgp peer list` and `show bgp peer detail` render it as `bfd-sub-state`, which is the
visibility Section 11 asks for. The key is written only while a session is
waiting, so a peer that is not carries none.

`FSM.EnterDelayOpenBfdUpPending` atomically consumes the running DelayOpen
timer for strict Event 20; it refuses every other state, a stopped delay, or
an already pending OPEN. The original `EnterBfdUpPending` remains OpenSent-only.
The ordinary negotiated HoldTimer bounds a nonzero-hold wait; zero hold uses
BfdHoldTimer. A second OPEN sends FSM Error and increments the retry counter.
BfdHold expiry sends Cease/BFD Down and increments it; strict configuration
change closes Connect/Active silently and resets it. All three retire the
connection and leave Idle. Ordinary HoldTimer expiry also counts the failed
attempt and retires the session through Event 10.

Pre-established received KEEPALIVEs share `openMu` with the BFD release. A
peer can answer immediately after the withheld KEEPALIVE reaches TCP, before
the sending goroutine has finished its write. That reply waits until the
release has applied OpenConfirm, rather than reaching Connect/Active's Event26
catchall. The wire KEEPALIVE still precedes the state transition. Established
KEEPALIVEs keep their existing fast path without this additional lock.
<!-- source: internal/component/bgp/reactor/session_handlers.go -- handleKeepalive, fsmMessageEvent -->
<!-- source: internal/component/bgp/reactor/bfd_delay_open_release_race_test.go -- TestSessionBFDStrictDelayOpenImmediatePeerKeepalive -->

<!-- source: internal/component/bgp/fsm/delay_open.go -- EnterDelayOpenBfdUpPending, releaseDelayOpenBfdPending -->
<!-- source: internal/component/bgp/reactor/session_bfd_strict.go -- advanceAfterOpen, handleBFDEvent -->
<!-- source: internal/component/bgp/reactor/bfd_delay_open_test.go -- TestSessionBFDStrictDelayOpenRelease, TestSessionBFDStrictDelayOpenBoundedWait, TestPeerBFDStrictDelayOpenRelease -->

<!-- source: internal/component/bgp/reactor/peer_bfd.go -- bfdSubState -->
<!-- source: internal/component/bgp/plugins/cmd/peer/peer.go -- handleBgpPeerDetail, fieldBFDSubState -->

`show bgp peer detail` and `show bgp peer list` leave the `remote-as` field out of a peer's row while a dynamic peer's AS is not yet negotiated, as the JSON events omit `remote.as`, rather than showing AS 0. An AS selector matches no such peer.
<!-- source: internal/component/bgp/plugins/cmd/peer/peer.go -- setRemoteAS, handleBgpPeerList, filterPeersBySelectorValue -->

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

The first cycle uses `Session.Start`; later cycles use `startDamped`.
Before publishing each Session, `runOnce` checks the peer's stopping flag
under the publication lock. Collision winners wait in a peer-owned inbound
slot until the losing cycle has cleared its negotiated state and initial-sync
facts; their OPEN is processed by the next Session, not the sealed loser.

<!-- source: internal/component/bgp/fsm/connect_retry_counter.go -- ConnectRetryCounter -->
<!-- source: internal/component/bgp/reactor/peer_run.go -- runOnce -->

### When it moves

| Direction | Events | Note |
|-----------|--------|------|
| set to zero | Event 1 (ManualStart) and Event 3 (AutomaticStart) in Idle; Event 2 (ManualStop) in every other state | The operator ends a retry history, and so does a session that reached Established, through the Event 3 that starts the cycle after it |
| increment by 1 | Events 8, 10, 18, 21, 22, 23, 24, 25, 28, and each state's "any other event" arm | Which events apply is per state, not global |
| unchanged | Event 6 (damped restart), Events 26 and 27 (KEEPALIVE, UPDATE), Event 19 in OpenSent, Event 11, and every event in Idle | The success path and the ignores |

Three of those are per-state rather than uniform, and the FSM handlers say so
in each arm:

- Event 24 (NotifMsgVerErr) increments in Established, and in Connect/Active
  only when DelayOpenTimer is not running. It does NOT increment while that
  timer runs, or in OpenSent/OpenConfirm, whose action lists have no counter line.
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

### DelayOpen Timer

- **Configuration:** `timer { delay-open 5; }`, inherited at BGP, group, or peer
  level. The YANG range is 0..240 seconds; zero is the default and sends OPEN
  immediately.
- **TCP success (Events 16/17):** stop ConnectRetryTimer, arm DelayOpenTimer,
  retain the socket in Connect or Active, and write no OPEN.
- **Peer OPEN (Event 20):** validate and negotiate against the prepared local
  proposal and send OPEN. Without a strict BFD wait, enter OpenConfirm and send
  KEEPALIVE; negotiated hold zero leaves both ordinary timers off. With a strict
  wait, retain the corresponding Connect/Active pending sub-state and withhold
  KEEPALIVE until BFD releases it, bounded by HoldTimer or BfdHoldTimer.
- **Expiry (Event 12):** send OPEN, start the OPEN-wait HoldTimer, enter OpenSent.
- **Failure and collision:** Connect Event 18 clears the delay and moves to
  Active without incrementing; Active Event 18 clears it and moves to Idle with
  an increment. An inbound OPEN may retire an OPEN-less held socket with
  Event 23 and no NOTIFICATION. Two OPEN-less sockets are not compared by ID.
- **Ownership:** the Session serializes expiry, OPEN receipt, and OPEN-less
  collision retirement. A delayed OPEN write failure and the reader it wakes
  share one Event 18 owner; transport closure must not erase the running-delay
  flag before that event. Peer cleanup joins initial OPEN work and stops timers
  even on failures before `Session.Run`; no callback can rearm after final cleanup.

<!-- source: internal/component/bgp/fsm/timer_delay_open.go -- StartDelayOpenTimer, StopDelayOpenTimer -->
<!-- source: internal/component/bgp/fsm/delay_open.go -- handleDelayOpen -->
<!-- source: internal/component/bgp/reactor/session_delay_open.go -- delayOpenExpired, closeHeldForCollision, stopSessionTimers -->
<!-- source: internal/component/bgp/reactor/session_read.go -- handleConnectionCloseLocked -->

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
- **Default:** 120 seconds (`ze.bgp.openwait`)
- **Behavior:** Fire in OPENSENT triggers disconnect

### BFD Hold-Down Timer

- **Purpose:** Delay the release until the BFD session has been Up for the configured interval (draft-ietf-idr-bgp-bfd-strict-mode Section 10)
- **Default:** 0, meaning no hold-down, set by the peer's `connection bfd { hold-down }` leaf in milliseconds
- **Behavior:** It is armed on the step into ESTABLISHED, which is where Section 10 puts the gate, and the expiry re-enters the transition it held. That is one of two places, never both for one session. From `OpenSentConfirmedBfdUpPending`, where the peer's KEEPALIVE already arrived, a BFD Up arms it instead of establishing. From `OpenSentBfdUpPending` a BFD Up does NOT arm it: that release goes to OpenConfirm, and the interval is served when the peer's KEEPALIVE arrives there (`Session.handleKeepalive`). `EventBfdAdminDown` and `EventBfdDisabled` skip it, because neither says the session has been Up for any time at all
- **Measured from:** when the BFD session entered Up, not when the BGP session noticed. A strict peer's BFD session outlives every BGP connection (Section 7), so a link that has been Up for a minute owes nothing on the next attempt, and only the part of the interval still outstanding is armed

### BFD Hold Timer

- **Purpose:** Bound the strict-mode wait for BFD when nothing else does
- **Default:** 30 seconds (draft-ietf-idr-bgp-bfd-strict-mode Section 3, attribute 18), set by the peer's `connection bfd { hold-time }` leaf
- **Behavior:** Started only when the negotiated BGP hold time is zero. A nonzero HoldTimer bounds the OpenSent or Connect/Active DelayOpen strict wait instead; Event 10 ends it. `Session.advanceAfterOpen` and `FSM.EnterDelayOpenBfdUpPending` select the timer at entry. BfdHoldTimer expiry closes the session with Cease / BFD Down. It stops on Idle and when the wait ends.
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

The FSM callback in `peer_run.go` publishes negotiated capabilities on entry
to OpenConfirm, before the answering KEEPALIVE, with identity from the current
negotiation. Established transitions separately notify lifecycle observers.
A callback runs on whichever goroutine drains the FSM's pending queue: the
read goroutine, a teardown caller, the hold timer, the BFD subscriber or a
forward-pool worker. So the leave arm waits on nothing, and the down is
announced later, by `runOnce` itself:

```go
session.fsm.SetCallback(func(from, to fsm.State, event fsm.Event) {
    if to == fsm.StateOpenConfirm {
        if reactor != nil {
            p.announceNegotiated(reactor, session, session.Negotiated())
        }
    } else if to == fsm.StateEstablished {
        // ... set negotiated capabilities ...
        if reactor != nil {
            p.announceEstablished(reactor, session) // marks the up phase around notifyPeerEstablished
        }
        p.startInitialRoutes()
    } else if from == fsm.StateEstablished {
        // leaveSession: release BFD without waiting, record what the close
        // is derived from (event, NOTIFICATION, close cause), mark the
        // session closing and forget it in the reactor (announceClosed),
        // clear capabilities, setState(Connecting), build the history row,
        // then close the leave signal.
        p.leaveSession(session, leave, from, to, event)
    }
})
defer p.sessionDown(session, publication, deliverChan, deliveryDone, leave)
return session.Run(p.ctx)
```

`sessionDown` runs on every exit from `runOnce`, a panic on the read goroutine
included:

| Step | What it does |
|------|--------------|
| 1 | When the FSM still reads Established (nothing else ended the session), fire `EventTCPConnectionFails`, so the leave arm runs |
| 2 | Close the delivery channel and wait for the delivery worker, which publishes every buffered UPDATE first |
| 3 | When the FSM's sticky entered-Established bit is set, wait on the leave signal, then on the BFD subscriber the arm released |
| 4 | Derive the close once (`sessionCloseFor`, `peer_close.go`) from the leave record and from whether the receive publication failed, which is final only now; raise session-dropped when no NOTIFICATION was exchanged, announce the down (`announceDown`, `notifyPeerClosed`), log "session closed" and append the leave-Established history row with the same reason |

So a plugin hears every UPDATE of a session before its down, and the down
follows `PeerStateConnecting`. A delivery worker that cannot publish a batch
(a recovered receiver panic, or no receiver) tears its session down with Cease
/ Out of Resources (`failReceivePublication`), without waiting on itself.

Before any handshake can publish negotiation, `runOnce` defers its publication
end, including for a retained collision-winner OPEN. The session owns admission
and completion of that publication. Ending seals admission, rejects queued late
callbacks and waits for an admitted publication to finish, without holding
reactor or peer locks across delivery. This also orders a BFD-subscriber
publication against a read-loop failure.

If the session published negotiation but never entered Established,
`endNegotiatedSession` marks that publication closing and sends only the
dispatcher's state-down event. It does not run Established lifecycle observers,
route cleanup, BFD release or session-dropped reporting. A failure before
publication emits no down. This ends the ExaBGP bridge's negotiated block and
attribute context before the next connection's OPEN; both caches live until
state down, not merely until the socket closes. Established teardown still
follows the table above, with exactly one down after all buffered UPDATEs.

Every callback receives the event whose handling made its change: `Event`
records it before dispatching, and `change` queues it with the transition, so a
callback drained on another goroutine still reads its own cause. `change` sets
the entered-Established bit, under the same lock as the state write, before it
queues the enter transition's callback.

<!-- source: internal/component/bgp/reactor/peer_run.go -- SetCallback on fsm -->
<!-- source: internal/component/bgp/reactor/peer_close.go -- sessionCloseFor -->
<!-- source: internal/component/bgp/fsm/fsm.go -- FSM.Event, FSM.change -->
<!-- source: internal/component/bgp/reactor/reactor_notify.go -- notifyPeerEstablished, notifyPeerClosed -->
<!-- source: internal/component/bgp/reactor/peer_derived_bindings.go -- Peer.announceNegotiated, Peer.announceEstablished, Peer.announceClosed, Peer.announceDown -->
<!-- source: internal/component/bgp/reactor/peer_session_end.go -- Peer.leaveSession, Peer.sessionDown, failReceivePublication -->
<!-- source: internal/component/bgp/fsm/fsm.go -- FSM.EnteredEstablished -->

### Peer lifecycle observers

Reactor maintains a list of observers notified on state changes. The interface
and its registration are both internal to the reactor package:

```go
type peerLifecycleObserver interface {
    OnPeerEstablished(peer *Peer)
    OnPeerClosed(peer *Peer, closed sessionClose)
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
