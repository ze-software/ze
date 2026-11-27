# Event History and FSM Transition Rings

An operator asks "what happened in the last N minutes?" and gets an answer with
no debug logging and no external tooling. Three rings supply it.

| Ring | Records | Command |
|------|---------|---------|
| global event ring | every event that reaches `deliverEvent()` | `show event recent [count N] [namespace X] [\| operation <id>]`, `show event namespaces` |
| per-peer BGP FSM | peer state transitions | `show bgp peer <sel> history` |
| per-tunnel and per-session L2TP FSM | tunnel and session state transitions | `show l2tp tunnel history <tid>`, `show l2tp session history <sid>` |

<!-- source: internal/component/plugin/server/event_ring.go -- EventRing, Append, EventRecord, AppendWithMeta, appendWithOp, SnapshotWithOp -->
<!-- source: internal/component/plugin/server/dispatch.go -- deliverEvent, deliverEventWithContext, deliverEventCaptured -->
<!-- source: internal/component/plugin/server/engine_event.go -- EmitEngineEventContext -->
<!-- source: internal/component/plugin/server/engine_event_gateway.go -- newConfigEventGatewayWithContext, ConfigEventGateway.EmitConfigEvent -->
<!-- source: internal/component/bgp/reactor/peer_history.go -- fsmHistory, FSMTransition -->
<!-- source: internal/component/l2tp/fsm_history.go -- L2TP tunnel and session history -->

## The decisions

**The global ring taps `deliverEvent()`, the single dispatch point for every
event.** One append there captures everything, with no per-namespace
subscription to keep in sync. `deliverEventCaptured` records `EventRecord` via
`appendWithOp`, filling `Source` (`stampedSender`, emitter name, or `"engine"`),
`Subject` (`peerAddress`), `Direction`, and optional `OpID` (`opid.From(ctx)`)
from values already in scope.

**Config reloads propagate their `operation-id` through `ConfigEventGateway`.**
`Server.EmitEngineEventContext` routes through `deliverEventWithContext` without
changing `pkg/ze.EventBus`. During a reload transaction, `reload_tx.go` binds
the reload context into `newConfigEventGatewayWithContext(s, ctx)` so every
`config` namespace event (`verify`, `apply`, `rollback`, `complete`) carries the
reload's `operation-id` on `EventRecord.OpID`. Operators and AI agents filter
the ring by operation via `show event recent | operation <id>` (over MCP:
`pipe: [{"name": "operation", "args": ["<id>"]}]`), backed by
`EventRing.SnapshotWithOp`.

**Fixed-size circular buffer, overwrite-oldest, non-blocking append.** The emit
path allocates nothing. Capacity is fixed at compile time per ring type, and the
oldest entries are overwritten in silence.

**BGP history records `PeerState`, not the internal `fsm.State`.** `PeerState`
is the state an operator observes, and it carries fewer values than the FSM's
own. The transition is appended where the peer's state actually changes.
<!-- source: internal/component/bgp/reactor/peer_run.go -- the FSMTransition append site -->

**L2TP reuses the same ring.** The tunnel and the session each own one, and a
transition is recorded where the state actually changes.

Nobody querying costs nothing. The ring rotates.

## Constraints

**The global ring appends BEFORE subscriber dispatch**, so an entry is visible
even when every subscriber rejected the event.

**BGP peer history belongs to the `Peer` struct instance.** Remove a peer and
add it back and the history starts fresh, because the old struct is collected.

**L2TP rings hold 16 entries per tunnel and per session.** A tunnel handshake
produces 3 to 5 transitions, so a ring covers several lifecycle iterations.
