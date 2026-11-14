# Spec: a BGP writer parked on a plugin keeps its peer from reconnecting

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Priority | P2 |
| Release reason | Shipped lifecycle defect: a plugin that stops taking the events Ze delivers to it holds a BGP session's write path, and the peer of that session does not reconnect until the plugin drains or stops |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-14 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Created at the closure of `spec-bgp-session-write-wedge` (2026-11-14), which
found this defect as its broken assumption A-3, did not cause it, and did not
fix it. That spec bounded every socket write and made every close interrupt a
writer blocked on the socket. A writer blocked on something else than the
socket is what remains.

The defect, as that spec's main thread read it at the producers on 2026-11-14.
It was not run, and the design of this spec reads each producer again:

- A session writer delivers the "sent" event to plugins while it holds the
  session's write path (`writeMu`): `writeUpdateBody` and `writeMessageHeld`
  (`internal/component/bgp/reactor/session_write.go`) call `onMessageReceived`,
  which reaches `Reactor.notifyMessageReceiver`
  (`internal/component/bgp/reactor/reactor_notify.go`, "Sent messages:
  synchronous delivery, no async channel") and from there the plugin's
  `Process.Deliver` (`internal/component/plugin/process/delivery.go`).
- `Process.Deliver` is a channel send whose only escape is the process's own
  context. A plugin that does not drain its event queue parks the writer
  there, and a socket close does not wake it.
- The parked writer also holds the adj-RIB-Out table mutex. That table is the
  Peer's, not the session's (`peer_run.go` `runOnce`: `session.adjOut =
  &p.adjOut`), so every session of the peer shares it.
- The close routine does not depend on the writer: the socket closes and
  `Session.Run` returns (`TestSessionEndsWhileWriterParkedOnPlugin`). But the
  session-end defer of `runOnce` joins the recovery worker (`joinRecovery`) and
  the initial-sync goroutine (`endInitialSync`), which wait on the locks the
  parked writer holds. So the peer does not reconnect until the plugin drains
  or stops, and the parked goroutine and the old session's locks stay until
  then.

What an operator sees today: one ERROR line and one event under
`doctor-bgp-write-path-held`, whose text names a plugin that does not take
events as the second cause, and a peer that stays down.

Goal: a plugin that stops taking events cannot keep a BGP peer from
reconnecting. The delivery wait either leaves the write path or gets a bound,
and the session-end path of the Peer never waits on a writer parked on a
plugin.

Second item, same surface (the held-path report): the record that a close
reported a held write path, `Session.writePathHeldReported`
(`internal/component/bgp/reactor/session.go`), is kept on the Session and never
cleared, while the fact belongs to one connection. `reportUnsentNotification`
(`session_connection.go`) and `leaveCause` (`peer_close.go`) read it. On a
Session used for a second connection an earlier report would silence a later
missed-NOTIFICATION line and one `session-dropped`. It was reviewed as
unreachable past Established today, because a Peer runs one Session per run
cycle. The record belongs on the connection's claim (`socketClaim`).

Constraints already known:

- The delivery seam itself, `Process.Deliver` and its 64-slot event queue, is
  the subject of `plan/post-release/engineering/spec-review-bus-async-fanout.md`:
  its problem 1 is this same blocking send. Thomas postponed that spec on
  2026-11-14, with no design chosen, so the send stays as it is for the first
  release. The bound this spec needs is therefore its own to design, on the
  reactor's side: what is held across the delivery, and the session-end joins.
- `TestSessionDownFollowsBufferedDeliveries` pins that a peer's down follows
  its buffered deliveries, a contract with plugins.

## Required Reading

- `ai/doors/system.md`, `ai/doors/bgp-engine.md`, `ai/doors/plugin-infra.md`
- `docs/architecture/core-design.md`, "Write Deadline" and the close sequence
- `docs/architecture/api/process-protocol.md`, event delivery
- `ai/digests/bgp-reactor.md`, `ai/digests/plugin-transport.md`
- `plan/learned/DESIGN-HISTORY.md`, BGP engine invariants (the session lock order)

## Current Behavior (MANDATORY)

- [ ] `internal/component/bgp/reactor/session_write.go` -> `writeUpdateBody`, `writeMessageHeld`: the sent event is delivered under `writeMu`
- [ ] `internal/component/bgp/reactor/reactor_notify.go` -> `notifyMessageReceiver`: a sent message is delivered synchronously
- [ ] `internal/component/plugin/process/delivery.go` -> `Process.Deliver`: a channel send with no timeout
- [ ] `internal/component/bgp/reactor/peer_run.go` -> `runOnce`: the session-end defer joins `joinRecovery` and `endInitialSync`
- [ ] `internal/component/bgp/reactor/session_connection.go` -> `reportWritePathHeld`, `reportUnsentNotification`

## Existing Mechanisms (MANDATORY)

To be completed at design. Known today:

| Mechanism | Where | This spec |
|-----------|-------|-----------|
| Bounded close routine | `session_connection.go` `closeConnWithin` | Does not apply to a writer parked on a plugin: it closes the socket and returns, and the writer stays parked |
| Held-path report | `session_connection.go` `reportWritePathHeld`, code `doctor-bgp-write-path-held` | Extends: the record of the report moves to the connection's claim |
| Plugin event delivery | `plugin/process/delivery.go` `Process.Deliver` | Does not apply: the send stays unchanged for the first release (`spec-review-bus-async-fanout` is postponed), so this spec bounds what the reactor holds across it |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
A session write whose sent event goes to a plugin that takes no events, followed by the end of that session.

### Transformation Path
To be written at design.

### Boundaries Crossed
To be written at design.

### Integration Points
To be written at design.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| To be designed | → | To be designed | To be designed |

## Acceptance Criteria

- AC-1: with a plugin that takes no events and a peer whose session ends, the peer reconnects within a stated bound, without the plugin draining or stopping.
- AC-2: a held-path report on one connection of a Session does not silence the missed-NOTIFICATION line or the `session-dropped` event of a later connection of that Session.

## Risks & Assumptions

To be written at design.

## 🧪 TDD Test Plan

### Unit Tests

| Test | Asserts |
|------|---------|
| To be designed | To be designed |

## Files to Modify

To be decided at design.

## Implementation Steps

To be decided at design.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `lock-held-across-a-join.md` | 2026-11-14 | sent-event delivery to plugins under `writeMu` and the adj-RIB-Out mutex |
| `per-request-state-on-a-shared-object.md` | 2026-11-14 | `Session.writePathHeldReported` kept per Session, never cleared |

## Checklist

- [ ] Tests written
- [ ] Tests FAIL before the change
- [ ] Tests PASS after the change
- [ ] The targeted tests pass, run in the main checkout by the main thread: `./le test unit pkg path ./internal/component/bgp/reactor/` and the functional test the design names under `./le test bgp plugin`
