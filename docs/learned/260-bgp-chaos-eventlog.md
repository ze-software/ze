# 260 — BGP Chaos Event Log

## Objective

Extend the NDJSON event log with a header line, global sequence numbers, and relative timestamps, then add a replay engine and diff tool so recorded runs can be replayed against the validation model without a live Ze instance.

## Decisions

- Sequence numbers and time-offset-ms are assigned by the JSONLog writer, not carried on the `peer.Event` struct — the Event struct stays clean; enrichment happens at the serialization boundary.
- `time-offset-ms` instead of absolute wall-clock timestamps — enables deterministic log comparison; same seed produces same relative offsets regardless of start time.
- EventProcessor switch logic (~25 lines) is inlined in `replay/replay.go` rather than imported — EventProcessor is in `package main` and cannot be imported; the validation primitives (Model, Tracker, Convergence, Check) are fully reused.
- Diff ignores `time-offset-ms` when comparing events — comparing event-type/peer-index/prefix makes diffs deterministic across runs with timing variation.

## Patterns

- `writeErr()` helper: gosec requires checking `fmt.Fprintf` return, but on error-reporting paths there is nothing useful to do — a named helper that checks but discards the error is cleaner than `_, _ =`.
- Log format is self-describing: first NDJSON line is always a header with `record-type: "header"` containing version, seed, and run parameters.

## Gotchas

- `peer/event.go` did NOT need modification — spec anticipated adding seq/timestamp fields to Event, but keeping them out of Event and adding them only in the writer was the cleaner approach.
- `validation/model.go` did NOT need modification — the validation model was already decoupled from TCP; replay worked without changes.

## Files

- `cmd/ze-bgp-chaos/report/jsonlog.go` — Extended format with header + seq + time-offset-ms
- `cmd/ze-bgp-chaos/replay/replay.go` — Replay engine: `Run(r, w) int`
- `cmd/ze-bgp-chaos/replay/diff.go` — Log diff: `Diff(r1, r2, w) int`
