# locrib ForwardHandle: zero-copy access for state trackers

State trackers downstream of the rib plugin (sysrib, FIB, observability, a
future archive) need the post-filter wire bytes of the UPDATE that produced a
best-path change. Without a handle they would re-enter the `StructuredEvent`
path or re-parse.

<!-- source: internal/core/rib/locrib/forward_handle.go -- the ForwardHandle contract -->
<!-- source: internal/component/bgp/plugins/rib/forward_handle.go -- the producer side -->

## The decisions

**`ForwardHandle` is an interface in locrib, not a concrete reactor type.**
locrib is core; the reactor buffer type satisfies the interface. The reactor
imports core and not the reverse, so an interface avoids the cycle. A non-BGP
producer leaves `Forward == nil`.

**Refcounting copies once, through `sync.Once`.** The first `AddRef` copies
`RawMessage.RawBytes` under the locrib write lock, which is cheap and bounded.
Later `AddRef` calls are atomic increments. A subscriber that never calls
`AddRef` pays a nil check.

**`InsertForward` is a sibling of `Insert`, not a replacement.** `Insert` stays
for non-BGP producers. `InsertForward` threads the handle into the dispatched
Change for `ChangeAdd` and `ChangeUpdate` only. `ChangeRemove` carries
`Forward == nil`.

**The two-trigger model stays.** The receive-path trigger fires per received
UPDATE for forwarders. `OnChange` fires per best change for state trackers.
The full reasoning is in `docs/architecture/rib/unified-locrib.md`.

**`Change.Forward` is state-tracker infrastructure.** The route server and the
route reflector will never use it.

## Constraints

**The handle is populated from a buffer the forward pool already refcounts for
the duration of the Insert call.** The RIB hot path takes no extra reference.

**The handle operates on `RawMessage.RawBytes`, not on reactor-owned buffers.**
A future zero-copy wiring can replace it without changing the locrib interface.

**The observer subscriber is a debug tool.** A production consumer implements
its own `OnChange` handler with matching `AddRef` and `Release` calls.
<!-- source: internal/component/bgp/plugins/rib/forward_observer.go -- debug subscriber -->
<!-- source: internal/component/bgp/plugins/rib/forward_tracker.go -- first production consumer -->

## Measured

`BenchmarkLocribInsert` in its baseline, `ForwardNil` and `ForwardHandle`
variants all sit within noise at about 148 ns/op, 32 B/op, 1 alloc/op.
