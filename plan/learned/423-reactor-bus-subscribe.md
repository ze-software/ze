# 423 -- Reactor Bus Subscription

## Context

The BGP reactor could publish Bus events (peer-up/down, updates) but could not receive them. This blocked `spec-iface-bus`, which needs the reactor to react to `interface/addr/added` and `interface/addr/removed` events. The fix makes the reactor a bidirectional Bus participant by implementing `ze.Consumer` and adding a handler registration API.

## Decisions

- **No extra goroutine** — the Bus already creates a per-consumer delivery worker goroutine. `Deliver()` runs inside it and calls handlers synchronously. This avoids double-buffering, chosen over a reactor-internal channel+worker.
- **Handler map frozen at Start** — `OnBusEvent()` returns error after `StartWithContext`. This avoids synchronization on every `Deliver()` call, chosen over runtime-mutable handlers with mutex.
- **Prefix-based handlers** — matches Bus subscription model. A handler for `"interface/"` receives all `interface/*` events.
- **Handler signature `func(ze.Event)`** — no error return, consistent with fire-and-forget Bus design. Handlers log errors internally.
- **Subscribe per unique prefix** — multiple handlers for the same prefix share one Bus subscription. `Deliver()` dispatches to all matching handlers.

## Consequences

- Reactor can now subscribe to cross-component events (interface, config, future subsystems)
- `spec-iface-bus` can register `OnBusEvent("interface/", handler)` before reactor starts
- Handler registration is compile-time only — no runtime plugin can add Bus handlers to the reactor
- `Deliver()` must never hold `reactor.mu` (deadlock risk with `publishBusNotification`)

## Gotchas

- `subscribeBus()` is a no-op when no handlers are registered — the `.ci` functional test can only validate AC-6 (clean lifecycle) until `spec-iface-bus` provides actual handlers
- The wiring tests use Go unit tests with real `bus.NewBus()`, not `.ci` tests, because there's no external API to publish Bus events or register handlers
- Bus creates one worker per unique Consumer pointer — multiple subscriptions from the same reactor share one delivery goroutine

## Files

- `internal/component/bgp/reactor/reactor_bus.go` — Consumer impl, OnBusEvent, subscribeBus/unsubscribeBus
- `internal/component/bgp/reactor/reactor_bus_test.go` — 6 unit tests (AC-1 through AC-7)
- `internal/component/bgp/reactor/reactor.go` — busHandlers/busSubs fields, wiring in StartWithContext/monitor
- `test/plugin/reactor-bus-subscribe.ci` — production lifecycle functional test
