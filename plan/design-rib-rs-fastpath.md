# Plan: RS/RR unified-with-skip fast path

Working design document. Precedes a formal spec.

## Why

Route-server (RS) and parts of route-reflector (RR) mode forward an
incoming UPDATE to many egress peers byte-for-byte. Today
`internal/component/bgp/reactor/reactor_api_forward.go` (~1.3K lines,
~51KB; the largest file in reactor by design) preserves that
zero-copy forward: one wire buffer received from peer A is shared
across every egress peer B...N that needs no attribute rewrite.
`bgpctx.ContextID` identifies "this buffer, unchanged"; the
forward-pool refcounts it and releases once the last peer has sent.

Phase 3b now mirrors BGP best-path into the cross-protocol Loc-RIB.
If the RS/RR forward path also has to round-trip through the RIB on
every UPDATE -- parse attributes, Insert into Loc-RIB, emit a change
event, rebuild the outbound UPDATE from Loc-RIB state for each egress
peer -- we lose the zero-copy property. A full-feed RS with 20 peers
pays ~20 rebuilds per prefix that would otherwise have been a single
byte copy.

Goal: keep the RIB authoritative (Loc-RIB sees the route, sysrib /
FIB / observability work correctly) without regressing the RS/RR hot
path. Per-peer rebuild is skipped when the export filter for that
peer is a no-op -- same behaviour the forward-pool already gives us
today.

## Scope

**In scope (this design):** the additive piece that makes zero-copy
forwarding possible for Loc-RIB Change subscribers. Adds an optional
forward handle to `locrib.Change`; BGP populates it on Insert; any
subscriber that wants to forward without rebuild can take a ref and
hand the buffer to the send path.

**Out of scope (explicitly deferred):**

- **RS/RR per-peer best-path.** RS mode computes per-peer best-paths
  (each client sees a different view via per-peer import filters);
  `locrib` tracks one global best. Driving RS forwarding from global
  Change events would drop updates where the global best didn't move
  but a per-peer view did. RS/RR forwarding therefore stays on the
  existing receive-path trigger. A separate design for per-peer
  best-change events is prerequisite to moving RS/RR off that path.
- **Retiring the old forward trigger.** Until RS/RR has a correct
  Change-driven path, the receive-path forward trigger stays. The
  "single trigger" end-state from earlier drafts of this design is
  blocked on the per-peer piece.

## Shape

Two pieces:

1. **Change events carry an optional forward handle.** `locrib.Change`
   gains a `Forward ForwardHandle` field (opaque interface satisfied
   by the reactor's buffer type). BGP producers populate it on Insert;
   non-BGP producers (static, kernel) leave it nil. Subscribers that
   don't need it ignore it; no ref / alloc cost for non-users.
2. **Subscribers that want to forward do the per-peer decision
   off-lock.** `ChangeHandler` runs under the RIB write lock
   (`locrib/manager.go:38-40`: "MUST NOT re-enter Insert/Remove on the
   same RIB and should defer any heavy work to a goroutine"). The
   reactor's subscriber takes a ref on the handle, enqueues a work
   item on the existing reactor worker channel, and returns. The
   worker later performs the per-peer "filter modifies?" check; no-op
   peers get the original buffer, modifying peers get a rebuild. Both
   feed the existing forward-pool send path.

Step 1 is additive to Phase 3c. Step 2 is where the real work lives;
it touches `reactor/forward_build.go`, `reactor/filter_chain.go`, and
the Change-event subscriber that feeds the per-peer send path.
`ChangeRemove` has no handle (Remove means no valid best); the worker
produces a WITHDRAW from `Change.Prefix` + family alone.

## Extending locrib.Change

Interface approach (the one we're going with). `locrib` is core;
reactor is a component that imports core, so a concrete
`*reactor.SharedBuffer` field in locrib would be a cycle. Instead
locrib defines an opaque interface; the reactor buffer type satisfies
it. Non-BGP producers leave the field nil.

Field name: `Forward ForwardHandle`. The interface exposes the two
methods the subscriber needs (refcount + "was this buffer ever
unchanged for this destination" probe via context-id). Producer does
not need to hold a ref on the RIB's hot path; the handle is populated
from a buffer already refcounted by the forward pool for the duration
of the Insert call.

## Per-peer decision: "will my filter modify this?"

`reactor/filter_chain.go` already tracks whether a filter "modifies"
per-peer (next-hop rewrite, AS-path prepend, attribute strip). Today
`forward_build.go` uses that signal to pick between "forward
unchanged" and "rebuild". The same signal answers our question:

- Filter is a no-op AND peer negotiated the same capabilities as the
  source peer AND no attribute rewrite is configured -> pass the
  original `Change.Buffer` to the per-peer send path.
- Otherwise -> rebuild from Loc-RIB entry's Path list through export
  chain, then pass the rebuilt buffer to the same send path.

The send path (`forward_pool` refcount + per-peer TCP writer) is
agnostic to which branch produced the buffer. It keeps doing what it
does today.

Migration: today the path-through decision happens in forward_build
against the receive-buffer. After this change it happens against the
Loc-RIB change event. Both take the same inputs; the rebuild trigger
just moves up the stack. The writer does not move.

## Ordering vs. buffer refcount

Today the forward path holds the only references; the pool releases
when the last peer has sent.

After this change the Loc-RIB Insert call holds a ref for the
duration of dispatch. `ChangeHandler` runs under the write lock, so
dispatch is bounded by the time it takes every subscriber to
`AddRef()` + enqueue + return. None of the subscribers do TCP I/O
under the lock (explicitly forbidden by
`locrib/manager.go:38-40`). Subscribers that want to retain the
buffer past dispatch hold their own ref; subscribers that don't
touch `Forward` pay no cost.

When Insert returns, BGP releases its own ref. The buffer stays
alive until the last AddRef-ing subscriber releases. Contract
matches existing forward-pool semantics.

## What this is NOT

- **Not** route-server being implemented. RS/RR already exist.
- **Not** a replacement for `forward_build` or `forward_pool`. Both
  stay. `forward_build` is the rebuild producer on the modify branch;
  `forward_pool` is the refcount + per-peer TCP writer on both
  branches.
- **Not** a new write path. The TCP writer is untouched. Only the
  decision site and the buffer source move.
- **Not** about packet receive / parse. Wire decode is upstream of
  this change and unaffected.

## Migration steps

Phase ordering matches the scope split above. Steps 1-3 are this
design; steps 4-5 are the deferred per-peer piece.

1. **Add `ForwardHandle` interface + Change field.** New file
   `internal/core/rib/locrib/forward_handle.go` holds the interface.
   `change.go` grows a `Forward ForwardHandle` field. Zero value is
   nil; non-BGP producers leave it nil. Unit test confirms nil-handle
   dispatch works.
2. **Reactor buffer satisfies the interface.** `forward_pool`'s
   shared buffer type gets the `AddRef() / Release()` and
   `SourceContextID() bgpctx.ContextID` methods. Existing refcount
   logic is reused; no new allocation path.
3. **BGP populates Change.Forward on Insert.** Wired via
   `ribForwardHandle` (`internal/component/bgp/plugins/rib/forward_handle.go`),
   created once per received UPDATE in `handleReceivedStructured`
   and threaded through `checkBestPathChange` into
   `locrib.InsertForward`. Implementation is sync.Once copy-on-AddRef
   with a `Bytes()` accessor exposed via the optional
   `locrib.ForwardBytes` interface. Subscriber-free UPDATEs pay one
   handle struct alloc and zero byte copies; retaining subscribers
   pay one bounded copy.

### Deferred (per-peer design prerequisite)

4. **Change-subscriber → worker-channel hand-off.** A future
   subscriber in the reactor takes a ref, enqueues on the existing
   worker channel, and returns. The worker does the per-peer
   decision off-lock.
5. **Consolidate triggers.** Once RS/RR can be driven from per-peer
   Change events, the receive-path trigger can be retired. Until
   then, RS/RR stays on its existing path and this design coexists
   with it for non-RS/RR subscribers (sysrib, future single-view
   forwarding).

Steps 4-5 are intentionally out of scope for this spec. Landing them
prematurely would break RS correctness (per-peer filter views not
expressed in global Change events).

## Files touched (this design)

- `internal/core/rib/locrib/forward_handle.go` -- new; `ForwardHandle`
  interface with `AddRef()`, `Release()`, `SourceContextID()`.
- `internal/core/rib/locrib/change.go` -- `Change` gains
  `Forward ForwardHandle`.
- `internal/core/rib/locrib/manager.go` -- `Insert` gains an overload
  that threads the handle into the dispatched Change; existing
  `Insert(fam, prefix, Path)` stays for non-BGP producers.
- `internal/core/rib/locrib/locrib_test.go` -- nil-handle
  dispatch case; AddRef/Release counts on a handle-carrying Insert.
- `internal/component/bgp/reactor/forward_pool.go` -- shared buffer
  type grows the three interface methods (no behaviour change,
  existing refcount is reused).
- `internal/component/bgp/plugins/rib/rib_bestchange.go` -- pass the
  handle to `locRIB.InsertForward` on the best-change path.
- `internal/component/bgp/plugins/rib/rib_structured.go` -- create
  one `ribForwardHandle` per received UPDATE at the top of
  `handleReceivedStructured`; thread it to every
  `checkBestPathChange` call for that UPDATE.
- `internal/component/bgp/plugins/rib/forward_handle.go` -- new;
  `ribForwardHandle` refcount (retained-bytes extension deferred to
  first real subscriber).

No change to `reactor_api_forward.go` / `forward_build.go` /
`filter_chain.go` in this design -- the receive-path trigger stays
as-is. The per-peer subscriber that consumes the handle is the
deferred step.

The reactor's forward-pool buffer model (single-owner-release via
`peerBufIdx` + `releaseItem`) is NOT touched: `ribForwardHandle`
operates on the wire bytes exposed via `RawMessage.RawBytes` rather
than on reactor-owned buffers. A future zero-copy wiring that hands
out reactor-refcounted buffers can replace `ribForwardHandle`
without changing the locrib interface.

## Benchmarks

This design is additive; no reactor hot path changes. Gate on the
three `BenchmarkLocribInsert*` benchmarks in `locrib_test.go`.

| Benchmark | ns/op | B/op | allocs/op |
|-----------|-------|------|-----------|
| `BenchmarkLocribInsert` | 194.3 | 32 | 1 |
| `BenchmarkLocribInsertForwardNil` | 148.5 | 32 | 1 |
| `BenchmarkLocribInsertForwardHandle` | 147.8 | 32 | 1 |

Apple M4 Max, Go dev build. The Forward variants are within noise of
the baseline (identical alloc shape; ns/op delta is GC scheduling
between runs, not the extra interface argument). Design budget "3
percent of baseline" is comfortably met.

The RS/RR hot-path benchmarks (`BenchmarkRSForwardNoOp`,
`BenchmarkRSForwardWithPrepend`) are deferred together with the
per-peer subscriber.

## Deferred

- **Per-peer Change-driven subscriber + trigger retirement.**
  Prerequisite: per-peer best-path Change events (see Scope).
- **RR-specific cluster-list / originator-id rewrites** still require
  a rebuild. The skip path applies only when the filter is a true
  no-op.
- **MP-BGP families with Label-Stack / RD rewrite** need the same
  "filter modifies?" check extended to the rewrite layer. Add per
  family as needed; the infrastructure is the same.
- **Per-peer Adj-RIB-Out materialisation** (design doc Phase 3 agreed
  "compute at send time, don't store"): this handle is the mechanism
  that makes that affordable. `show bgp neighbor X advertised-routes`
  runs the export chain on demand; the skip path keeps normal
  forwarding fast.

## Open questions

1. **`bestPrev` redundancy.** BGP-internal state is not subsumed by
   locrib Change events (replay in `rib_bestchange.go` iterates
   `bestPrev` directly). They coexist; `bestPrev` stays authoritative
   for BGP consumers.
2. **Race coverage.** Handle lifetime crosses the locrib write-lock
   boundary. `make ze-race-reactor` must pass with the new subscriber
   test that AddRef-s inside the handler and Release-s later on a
   goroutine.
3. **Non-BGP forward handles.** Kernel / static producers leave
   `Forward` nil today. Revisit if a protocol producer gains a
   forward-shaped payload (e.g. OSPF LSA originator).
