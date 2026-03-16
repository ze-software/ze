# 272 — Session Read Pipeline

## Objective

Eliminate three performance bottlenecks in the BGP session read path: 100ms deadline polling, synchronous plugin event delivery blocking the read goroutine, and sequential fan-out to multiple plugins.

## Decisions

- Close-on-cancel (cancel goroutine closes `net.Conn`) instead of `SetReadDeadline` polling — standard Go idiom, instantly unblocks `ReadFull`, works with `net.Pipe()` and mock connections where `SetReadDeadline` is a no-op.
- Per-peer bounded delivery channel (capacity 256) + delivery goroutine, not a reactor-wide shared channel — per-peer isolation ensures peer A's backlog cannot block peer B's read goroutine.
- `cache.Add()` stays on the read goroutine BEFORE enqueue; `Activate()` happens on the delivery goroutine AFTER all deliveries — preserves the fast-forward ordering contract.
- Only UPDATE messages use async delivery; OPEN/KEEPALIVE/NOTIFICATION remain synchronous — they are infrequent and FSM-critical.
- Pre-format optimization: group plugins by format mode, encode once per distinct format before fan-out — eliminates N redundant JSON encodings for N same-format plugins.
- Delivery channel lives in `Peer` (not `Session`) because the channel is a Reactor-level concern — `notifyMessageReceiver` in reactor enqueues to it.

## Patterns

- `WaitGroup` + goroutines per plugin for parallel fan-out; atomic counter for consumer count passed to `Activate`.
- Three independently shippable phases: close-on-cancel → parallel fan-out → async pipeline. Each committable and verifiable alone.
- Channel capacity 256 chosen over spec's initial 64 — 64 caps throughput at ~6.4K UPDATEs/sec with 10ms delivery spike; 256 sustains ~256K UPDATEs/sec before backpressure.

## Gotchas

- Cross-peer circular deadlock risk: A reads UPDATE → delivers to RR → RR forwards to B → B's TCP write blocks because B's recv buffer full because B's read goroutine blocked on its delivery channel → channel blocked because delivery goroutine blocked on forwarding to A. Per-peer channels + async delivery breaks this cycle.
- `TestDeliveryBackpressure` and `TestCrossPeerIsolation` initially hung in pre-implementation state — blocked because test goroutine was the receiver; fixed by restructuring: backpressure test uses undrained channel, cross-peer test moves peer-A sends to a goroutine.
- Phase 3 tests landed in `reactor_test.go`, not `session_test.go` as planned — because `notifyMessageReceiver` and `Peer` live in reactor, not session.

## Files

- `internal/component/bgp/reactor/delivery.go` — `deliveryItem` struct + `deliveryChannelCapacity` (created)
- `internal/component/bgp/reactor/peer.go` — `deliverChan` field + lifecycle in `runOnce`
- `internal/component/bgp/reactor/reactor.go` — async enqueue in `notifyMessageReceiver`
- `internal/component/bgp/reactor/session.go`, `listener.go` — close-on-cancel
- `internal/component/bgp/server/events.go` — parallel fan-out + pre-format map
- `internal/component/bgp/server/events_test.go` — 3 Phase 2 tests (created)
- `internal/component/bgp/reactor/reactor_test.go` — 7 Phase 3 tests added
