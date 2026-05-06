# Learned: spec-l2tp-8-plugins -- Plugin Infrastructure

## What Was Built

Two-layer plugin registration for L2TP. Handler-level: plugins call
`l2tp.RegisterAuthHandler()` / `l2tp.RegisterPoolHandler()` from `init()`
to inject function-typed callbacks into the L2TP component's handler
registry (package-level vars behind `sync.RWMutex`). Plugin-level:
standard `registry.Register()` with YANG schema, config roots, CLI
handlers, metrics binders, and EventBus subscribers.

Four plugins built on this infrastructure: l2tpauthlocal, l2tpauthradius,
l2tppool, l2tpshaper.

## Key Decisions

- **Drain goroutines per session.** `drainAuth` and `drainPool` read from
  typed channels; PPP session writes requests, drain calls the registered
  handler, sends the response back. Decouples PPP FSM from plugin latency.

- **Panic recovery in drain.** If a handler panics, drain recovers, logs,
  rejects the current request, and continues. Next request succeeds.

- **Nil handler asymmetry.** Nil auth handler accepts all (with WARN); nil
  pool handler rejects all (with ERROR). No pool means no IP, so session
  must fail. Auth-less operation is a valid deployment.

- **Handled sentinel for async.** RADIUS handler calls `respond()` from its
  own goroutine, returns `Handled=true` so drain skips its own response.
  Lets sync and async handlers share one drain path.

- **Last-writer-wins registration.** If multiple auth plugins load, import
  order determines which handler wins. No composition or chaining.

## Patterns Worth Reusing

- Drain goroutine + typed channel pattern isolates FSM from plugin I/O.
- Panic recovery + reject-and-continue keeps sessions alive across handler bugs.
