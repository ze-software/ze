# Goroutine Lifecycle Rationale

Why: `.claude/rules/goroutine-lifecycle.md`

## Why Per-Event Goroutines Are Forbidden

At 1000 UPDATEs/sec with 3 plugins: 3000 goroutines/sec created and destroyed.
Each: ~4KB stack alloc, scheduler overhead, GC pressure from closures, no backpressure.

## Channel Backpressure

Channels self-regulate: consumer slow → channel fills → sender blocks/errors.
Unbounded goroutine creation: no backpressure → memory exhaustion.

## Existing Implementations

| Component | Channel | Worker | Location |
|-----------|---------|--------|----------|
| Plugin process | `eventChan` | `deliveryLoop()` | `internal/component/plugin/process/process.go` |
| Peer session | `deliverChan` | delivery goroutine | `internal/component/bgp/reactor/peer.go` |
