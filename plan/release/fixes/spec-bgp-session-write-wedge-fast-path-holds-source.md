# Spec: a stalled route-server destination holds the source peer's read loop

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Priority | P2 |
| Release reason | Shipped lifecycle defect: on the route-server fast path one client that stops reading holds the read goroutine of the peer whose routes are being forwarded for up to the write budget, so a source with a short hold time can lose its own session |
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
bounded the fast path's write and recorded what the bound does not remove
(its Known Limitations).

The defect, read at the producer on 2026-11-14 and not run:

- The route-server fast path, `tryDirectWriteNoFlush`
  (`internal/component/bgp/reactor/forward_rs_direct.go`), writes a forwarded
  UPDATE to the destination's session on the SOURCE peer's read goroutine. It
  takes the destination's write path by `TryLock`, arms the write budget
  (`Session.armFastPathBudget`, `ze.fwd.write.deadline`, 30 s by default) and
  writes synchronously. The destination's write buffer is 16 KiB, so a batch
  that overflows it writes to the socket inside that call.
- A destination that stops reading with its connection open blocks that write
  until the budget passes. The destination is then retired and the source's
  loop goes on, which is what the source spec's AC-6 asked for.
- For as long as the write is blocked the source's read goroutine reads
  nothing: no KEEPALIVE and no UPDATE from the source is processed. A source
  whose negotiated hold time is shorter than the budget can lose its own
  session to its hold timer, because of another peer's stall. A standing
  budget is renewed only after a sixteenth of it has passed
  (`fastPathBudgetShare`), so the block lasts at least fifteen sixteenths of
  the budget (28.1 s of 30 s).

Goal: a destination that stops reading never costs the source its session.
The source's read goroutine does not wait on a destination's socket for longer
than a bound that is short against any legal hold time, or does not wait on it
at all.

Constraints already known:

- `plan/release/validation/spec-perf-rs-fast-path-regression-and-pass-labels.md`
  measures this path and states that the path stays where it is: its Existing
  Mechanisms row for the fast path sets moving the work elsewhere aside. This
  spec's design is presented to Thomas against that statement, and a figure
  that spec records is taken again after any change here.
- `docs/architecture/bgp/structural-forwarding.md`: the fast path MUST NOT
  wait for a destination's write path, which is why it uses `TryLock`; a
  contended destination is handed to its forward worker
  (`Peer.handOffForwardFlush`, `docs/architecture/forward-congestion-pool.md`).
  The hand-off exists for a held lock, not for a full socket.
- Any change keeps the fast path's allocation count
  (`TestFastPathBudgetAddsNoAllocation`) and the per-source FIFO order of
  forwarded UPDATEs.
- A time figure for this path comes only from Thomas's Mac
  (`ai/META-DECISIONS.md`).

## Required Reading

- `ai/doors/system.md`, `ai/doors/bgp-engine.md`
- `docs/architecture/bgp/structural-forwarding.md`, `docs/architecture/forward-congestion-pool.md`
- `docs/architecture/core-design.md`, "Write Deadline"
- `ai/digests/bgp-reactor.md`

## Current Behavior (MANDATORY)

- [ ] `internal/component/bgp/reactor/forward_rs_direct.go` -> `tryDirectWriteNoFlush`: `TryLock`, `armFastPathBudget`, a synchronous write on the source's read goroutine
- [ ] `internal/component/bgp/reactor/session_write_mutex.go` -> `armFastPathBudget`, `releaseFastPathWrites`: the standing budget and its renewal

## Existing Mechanisms (MANDATORY)

To be completed at design. Known today:

| Mechanism | Where | This spec |
|-----------|-------|-----------|
| Fast-path write budget | `session_write_mutex.go` `armFastPathBudget` | Bounds the block at the write budget; this spec shortens the block or removes it |
| Flush hand-off to the forward worker | `peer.go` `handOffForwardFlush` | Covers a contended write path, not a blocked socket write; a candidate to extend |
| Forward pool worker | `forward_pool.go` `fwdBatchHandler` | Already writes off the source's goroutine, under the same budget |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
A received UPDATE on a route-server source peer whose destination has stopped reading.

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

- AC-1: route server, one destination stops reading while a source whose hold time is shorter than the write budget keeps sending: the source's session stays Established, and the stalled destination is retired.

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
| `slow-reader-stalls-a-shared-goroutine.md` | 2026-11-14 | fast path `tryDirectWriteNoFlush` blocks the source's read goroutine on a stalled destination |

## Checklist

- [ ] Tests written
- [ ] Tests FAIL before the change
- [ ] Tests PASS after the change
- [ ] The targeted tests pass, run in the main checkout by the main thread: `./le test unit pkg path ./internal/component/bgp/reactor/` and the functional test the design names under `./le test bgp plugin`
