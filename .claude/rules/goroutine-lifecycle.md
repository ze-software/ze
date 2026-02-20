# Goroutine Lifecycle

**BLOCKING:** All goroutines MUST be long-lived workers. Never create per-event goroutines in hot paths.

## Why This Rule Exists

Creating a goroutine per event (e.g., per BGP UPDATE, per plugin delivery) causes:
- Stack allocation (~4KB per goroutine) on every event
- Scheduler overhead for creation and teardown
- GC pressure from closure captures
- No backpressure — unbounded goroutine count under load

At 1000 UPDATEs/sec with 3 plugins, per-event goroutines create and destroy 3000 goroutines/sec. Long-lived goroutines eliminate this entirely.

## The Rule

| Pattern | Status |
|---------|--------|
| Long-lived goroutine reading from channel | Required |
| Goroutine per lifecycle (process, session, peer) | Acceptable |
| Goroutine per event in a hot path | Forbidden |
| `go func()` inside a `for range` loop on events | Forbidden |

## What "Long-Lived" Means

A goroutine is long-lived if its lifetime is tied to a **component** (process, peer, session), not to an **event** (message received, state change).

| Goroutine Lifetime | Example | OK? |
|--------------------|---------|-----|
| Process lifetime | Delivery goroutine per plugin process | Yes |
| Peer session lifetime | Per-peer delivery goroutine in reactor | Yes |
| Single event | `go func() { sendEvent() }()` per UPDATE | No |
| Single RPC call | `go func() { sendRPC() }()` per plugin | No |

## Required Pattern: Channel + Worker

```
Component Start:
    create channel
    start worker goroutine (reads from channel)

Hot Path (per event):
    enqueue work to channel (no goroutine creation)

Component Stop:
    close channel
    worker drains remaining items and exits
```

## Backpressure

Channels provide natural backpressure. When a consumer is slow:
- Channel fills up
- Sender blocks (or returns error)
- System self-regulates

This is better than unbounded goroutine creation, which has no backpressure and can exhaust memory.

## Existing Implementations

| Component | Channel | Worker | Location |
|-----------|---------|--------|----------|
| Plugin process | `eventChan` | `deliveryLoop()` | `internal/plugin/process.go` |
| Peer session | `deliverChan` | delivery goroutine in `runOnce()` | `internal/plugins/bgp/reactor/peer.go` |

## When go func() IS Acceptable

| Context | Why it's OK |
|---------|------------|
| Component startup (one-time) | `go p.monitor()`, `go p.relayStderr()` — tied to process lifetime |
| Test helpers | `go mockPluginResponder(...)` — not production code |
| `ProcessManager.Stop()` wait | One-time parallel wait, not a hot path |
| `Process.Wait()` bridge | Adapts blocking Wait to select — one goroutine, one-time |

## Enforcement

Before writing `go func()`:
1. Is this inside a loop that processes events? → Use channel + worker
2. Is this called on every message/event? → Use channel + worker
3. Is this a one-time lifecycle operation? → OK to use `go func()`
