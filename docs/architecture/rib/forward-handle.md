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
producer leaves `Forward == nil`; a BGP control-plane re-election with no source
UPDATE also leaves it nil.

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

A received UPDATE inserts its routes into Adj-RIB-In before publishing their
best-path changes. The AIGP reselection worker can elect one of those inserted
routes first, with no source UPDATE handle. The receive handler's later mirror
of the identical path is a Loc-RIB no-op, not a second byte-carrying event.
Consumers therefore observe `Change.Best` for the selected path and treat
`Change.Forward` as optional even for BGP. Tests needing a causal recovery marker
use a changed path field, such as the MED carried as `Best.Metric`, and check the
wire attributes separately in storage and on recipient TCP.
<!-- source: internal/component/bgp/plugins/rib/rib_structured.go -- handleReceivedStructured -->
<!-- source: internal/component/bgp/plugins/rib/rib_aigp.go -- reselectAIGPRoutes -->
<!-- source: internal/component/bgp/plugins/rib/rib_bestchange.go -- checkRouteBestChange -->
<!-- source: internal/core/rib/locrib/manager.go -- insert -->

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

SDK `Run` fences and drains admitted events before `runRIBPlugin` detaches the
mirror with `SetLocRIB(nil)`. Each mirror operation loads one atomically
published Loc-RIB pointer. `SetLocRIB` replaces publication and subscriber
ownership under `peerMu`, but unsubscribes and stops the old tracker after
unlocking, so callbacks can reenter manager operations.

Tracker `Stop` fences callback admission before joining its worker and releasing
queued handles. Unsubscription alone cannot revoke callbacks retained in an
earlier Loc-RIB subscriber snapshot. A concurrent `SetLocRIB(nil)` is not itself
a delivery join: an operation that already loaded the pointer may finish.
<!-- source: internal/component/bgp/plugins/rib/rib.go -- runRIBPlugin, SetLocRIB -->
<!-- source: internal/component/bgp/plugins/rib/forward_tracker.go -- onChange, Stop -->
<!-- source: pkg/plugin/sdk/sdk.go -- Run -->

The shutdown fixture holds the admitted DOWN at its removal hook. Before DOWN
starts, it pauses the real AIGP reselection worker at candidate extraction, then
queues DOWN's peer-state writer before releasing that worker. Otherwise a scan
that already copied the prefix can remove it before DOWN reaches the hook,
making the fixture fail even though the route was withdrawn. This scheduling
barrier leaves the SDK drain and all shutdown assertions in place.
<!-- source: internal/component/bgp/plugins/rib/rib_shutdown_test.go -- TestRIBShutdownDrainsStructuredPeerDown -->

Concurrent mirror publication also checks the surviving lifecycle contract:
reattachment publishes the requested path, a later detached withdrawal leaves
that shared RIB untouched, and reattachment permits its removal.
<!-- source: internal/component/bgp/plugins/rib/rib_shutdown_test.go -- TestLocRIBPublicationConcurrentMirrors -->

## Measured

`BenchmarkLocribInsert` in its baseline, `ForwardNil` and `ForwardHandle`
variants all sit within noise at about 148 ns/op, 32 B/op, 1 alloc/op.
