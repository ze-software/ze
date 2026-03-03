# 324 — Architecture Phase 2: Bus Implementation

## Objective

Build the standalone `internal/bus/` implementation satisfying `ze.Bus`. Hierarchical topics, prefix-based subscription matching, metadata filtering, per-consumer delivery goroutines with batch drain.

## Decisions

- Server integration (wiring Bus into existing dispatch path) deferred to Phase 5 — current dispatch formats events per-subscriber after matching, and integrating requires solving format negotiation alongside BGPHooks elimination simultaneously. Bus built standalone first.
- Constructor named `NewBus()` not `New()` — hook `check-existing-patterns.sh` blocks `func New()` as it exists in 6+ other internal packages (false positive; reported as friction)
- Per-consumer worker goroutine with buffered channel (capacity 64) and drain-batch pattern with reusable `[]ze.Event` slice — follows `process_delivery.go` deliveryLoop pattern
- `strings.HasPrefix(topic, prefix)` for matching — exact match is included (prefix == topic)
- Metadata filtering: all filter key-value pairs must exist in event metadata; nil/empty filter matches all

## Patterns

- Standalone component built first, integration deferred to final phase — avoids scope explosion
- `bus.Stop()` closes all worker channels and waits for goroutines — same shutdown pattern as reactor workers
- Import constraint enforced at test time via `go list -f '{{.Imports}}'` — only `pkg/ze/` + stdlib

## Gotchas

- `check-existing-patterns.sh` hook false-positive blocked idiomatic `New()` constructor name — required `NewBus()` as workaround. Reported as friction.

## Files

- `internal/bus/bus.go` — implementation (228 lines)
- `internal/bus/bus_test.go` — 14 tests including concurrent publish, batch delivery, race detector clean
