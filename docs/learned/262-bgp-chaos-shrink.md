# 262 — BGP Chaos Shrink

## Objective

When a property violation is found, automatically minimize the reproduction to the smallest sequence of events that still triggers the same failure, making bugs debuggable rather than buried in multi-thousand-event traces.

## Decisions

- Shrink engine is **pure**: no TCP, no Ze instance — replays events through PropertyEngine only. Each iteration feeds a candidate event list to the replay engine and checks whether the same property still fails.
- Binary search (O(log n)) followed by single-step elimination (O(n)) — binary search rapidly reduces 10k events to hundreds; single-step removes individual unnecessary events.
- Causal cascade: removing an Established event also removes all route events for that peer whose precondition was Established. Without this, the shrunk log contains impossible sequences (route announced before session established).
- `--auto-shrink` deferred — collecting events in-memory during a live run requires buffering the entire run; `--shrink <file>` from a saved log covers the primary use case.
- Reverse iteration in single-step elimination — iterating end-to-start keeps indices stable after removal without index adjustment.

## Patterns

- `ParseLog` (NDJSON → `[]peer.Event`) is a separate function in `shrink/parse.go` — clean separation from the shrink algorithm itself, and reusable for other log-reading scenarios.
- Binary search stops when neither half alone reproduces the failure — does not try splitting further (failure requires events from both halves, so the next phase handles it).

## Gotchas

- None.

## Files

- `cmd/ze-bgp-chaos/shrink/shrink.go` — Config, Result, Run, binarySearch, singleStepEliminate
- `cmd/ze-bgp-chaos/shrink/causal.go` — RemoveWithDependents (cascade deletion)
- `cmd/ze-bgp-chaos/shrink/parse.go` — ParseLog (NDJSON → []peer.Event)
