# 015 — FSM Active Design

## Objective

Investigate whether the FSM should be made "active" (handling I/O and timers directly) to reduce reactor bloat — concluded that no change was needed.

## Decisions

- Closed without action. The original critique was based on incorrect understanding of the codebase.

## Patterns

- FSM package (`internal/bgp/fsm/`) already contains both state transitions (`fsm.go`) and timer management (`timer.go`) — the critique that "FSM doesn't handle timers" was false.
- Timer wiring in `session.go` is ~20 lines of callback setup, not a source of bloat.
- Reactor bloat (identified in `peer.go`, ~1350 lines of encoding logic) is caused by encoding logic, not timer management — addressed separately in `peer-encoding-extraction.md`.

## Gotchas

- Finding *a* handler is not the same as understanding *why* the code is structured that way. The FSM/Reactor boundary follows the ExaBGP pattern intentionally: FSM = pure state transitions + timers, Reactor = orchestration + I/O.
- The critique conflated two separate issues (FSM design vs. reactor bloat) that have different root causes and different fixes.

## Files

No files modified — investigation only.
