# 784: locrib ForwardHandle zero-copy access

Spec: design-rib-rs-fastpath

## Context

State-tracker consumers downstream of the rib plugin (sysrib, FIB, observability,
future archive) needed access to post-filter wire bytes from the UPDATE that produced
a best-path change. Without it they would need to re-enter the StructuredEvent path
or re-parse. The original framing ("retire the receive-path trigger") was wrong: the
receive path is load-bearing for the ingress filter pipeline and serves forwarders
(RS, RR), a fundamentally different consumer category.

## Decisions

**Two-Trigger Model stays.** Receive-path trigger fires per received UPDATE for
forwarders (RS, RR); locrib `OnChange` fires per best-change for state trackers.
They serve different consumer categories and do not collapse.

**Interface, not concrete type.** `ForwardHandle` is an interface in locrib (core);
the reactor buffer type satisfies it. Avoids import cycle (reactor imports core,
not the reverse). Non-BGP producers leave `Forward == nil`.

**Refcount via sync.Once copy.** `ribForwardHandle.AddRef` triggers a one-time copy
of `RawMessage.RawBytes` under the locrib write lock (cheap, bounded). Subsequent
AddRefs are atomic increments. Subscribers that never call AddRef pay only a nil
check.

**InsertForward sibling method.** Existing `Insert(fam, prefix, Path)` stays for
non-BGP producers. `InsertForward` threads the handle into the dispatched Change
for `ChangeAdd`/`ChangeUpdate` only; `ChangeRemove` carries `Forward == nil`.

**Cancelled: retire receive-path, per-peer Change-driven subscriber.** Both were
based on incorrect assumptions about how RS/RR work. RS forward-alls every received
UPDATE through per-peer egress logic; locrib only tracks one global best.

## Gotchas

- `ForwardHandle` is populated from a buffer already refcounted by the forward pool
  for the duration of the Insert call. No extra ref needed on the RIB hot path.

- `ribForwardHandle` operates on `RawMessage.RawBytes`, not reactor-owned buffers.
  A future zero-copy wiring can replace it without changing the locrib interface.

- The observer subscriber (`forward_observer.go`) is a debug tool; production
  consumers should implement their own `OnChange` handler with proper AddRef/Release.

## Test coverage

- Unit: `locrib_test.go` nil-handle dispatch, AddRef/Release count assertions.
- Benchmarks: `BenchmarkLocribInsert*` (baseline, ForwardNil, ForwardHandle) all
  within noise (~148 ns/op, 32 B/op, 1 alloc/op).
- Functional: `test/plugin/rib-forward-handle-observed.ci` drives a real BGP UPDATE
  through the daemon, asserts observer log line.

## Files

None recorded.
