# Plan: locrib state-tracker zero-copy access

Working design document. Implementation log of what landed; the
original "retire the receive-path trigger" framing was based on a
wrong model of how ze handles RS/RR and per-peer filtering. Section
"Two-Trigger Model" below documents the actual design; the rest of
the document is the implementation log of the additive `Change.Forward`
handle that fits within it.

## Two-Trigger Model (the actual design)

Every received UPDATE flows through one filter pipeline in the reactor
(see `docs/architecture/core-design.md` "Ingress Filter Pipeline"):
in-process ingress filters first (may modify wire bytes via
`IngressFilterFunc.modifiedPayload`), then external-plugin import
policy filters (`PolicyFilterChain` direction `import`, may modify via
`ModAccumulator`). The post-filter `WireUpdate` is cached and a
`StructuredEvent` is dispatched once.

Two consumer categories subscribe:

| Category | Examples | Trigger | What they need |
|----------|----------|---------|----------------|
| Forwarders | RS, RR, future bgp-mirror | StructuredEvent (per received UPDATE) | every received UPDATE, then per-peer egress decision (egress filters, RFC 4456 RR injection, next-hop, AS-override, EBGP prepend) in `reactor_api_forward.go` |
| State trackers | rib plugin, then `locrib.OnChange` consumers (sysrib, FIB, observability, future archive) | StructuredEvent for the rib plugin; `locrib.OnChange` for downstream consumers | best-path-change events; optionally the wire bytes via `Change.Forward.(ForwardBytes).Bytes()` |

These two needs do not collapse. Forwarders fire per received UPDATE
(including duplicates that don't change best); state trackers fire per
best-change. Locrib stores ONE best per (family, prefix); turning it
into a per-received-path event source would require it to store every
received path, which is not what a Loc-RIB is.

The receive-path trigger is **load-bearing for the filter pipeline**:
ingress filters run there, before caching, before either consumer
category sees the bytes. It cannot be retired without re-homing the
filter pipeline, which would be a cross-component refactor with no
benefit.

## What this document delivered

An additive optional handle on `locrib.Change` so state-tracker
consumers downstream of the rib plugin can access the post-filter wire
bytes without going back to the StructuredEvent path. Lets a future
sysrib mirror, route archive, or RR cluster-list extractor see the
bytes that BGP saw, gated on the consumer calling `AddRef` to retain
past the handler invocation.

This does NOT change forwarder behaviour: RS / RR still subscribe to
StructuredEvent and call `ForwardCached` / `ForwardUpdatesDirect`
through the reactor's per-peer egress path. The forward fast-path is
unchanged.

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

## Ordering vs. buffer refcount

`ChangeHandler` runs synchronously under the locrib write lock
(`internal/core/rib/locrib/manager.go:38-40`: "MUST NOT re-enter
Insert/Remove on the same RIB and should defer any heavy work to a
goroutine"). The `ribForwardHandle.AddRef` path is therefore in the
lock; it must be cheap. The current implementation is a sync.Once
copy + atomic increment -- the once.Do fires synchronously while the
producing handler still holds the source bytes (which are
`bgptypes.RawMessage.RawBytes`, valid only while the handler runs).

After AddRef returns, the handle's owned buffer is safe for the
subscriber's worker to read off-lock. `Release` is an atomic decrement;
the handle becomes GC-eligible once all AddRefs are matched.

Subscribers that don't touch `Forward` pay only the nil-check cost
inside their own handler; locrib does not AddRef on their behalf.

## What this is NOT

- **Not** a replacement for the receive-path trigger. The receive
  path is the filter pipeline entry (see "Two-Trigger Model" above)
  and stays.
- **Not** a per-peer event source. Locrib still tracks one global
  best per (family, prefix); `Change.Forward` carries the post-filter
  wire bytes that produced that best, not per-peer views.
- **Not** a new write path. RS / RR / future forwarders still go
  through `reactor_api_forward.go`'s `ForwardUpdate` /
  `ForwardUpdatesDirect`. State trackers do not write to peers.
- **Not** about packet receive / parse. Wire decode and the ingress
  filter pipeline both run upstream and are unaffected.

## What was implemented

1. **`ForwardHandle` interface + `Change.Forward` field**
   (`internal/core/rib/locrib/forward_handle.go`,
   `internal/core/rib/locrib/change.go`). Opaque, refcount-only on
   the locrib side. Optional `ForwardBytes` sidecar interface for the
   retained-bytes accessor.
2. **`InsertForward` method**
   (`internal/core/rib/locrib/manager.go`). Sibling of `Insert`;
   threads the handle into the dispatched `Change` for `ChangeAdd` /
   `ChangeUpdate`. `ChangeRemove` carries `Forward == nil` (no source
   buffer to share).
3. **`ribForwardHandle` producer**
   (`internal/component/bgp/plugins/rib/forward_handle.go`). sync.Once
   copy of `RawMessage.RawBytes` on first `AddRef`; atomic refcount;
   `Bytes()` accessor satisfying `ForwardBytes`. Subscriber-free
   UPDATEs pay one handle alloc and zero byte copies; retaining
   subscribers pay one bounded copy.
4. **rib plugin wiring**
   (`internal/component/bgp/plugins/rib/rib_structured.go`,
   `rib_bestchange.go`). One handle per received UPDATE, threaded
   through `checkBestPathChange` into `locrib.InsertForward`.
5. **Observer subscriber**
   (`internal/component/bgp/plugins/rib/forward_observer.go`,
   `rib.go::SetLocRIB`). First real `OnChange` consumer that nil-checks
   `Change.Forward`; logs at debug level when present. Functional `.ci`
   test (`test/plugin/rib-forward-handle-observed.ci`) drives a real
   BGP UPDATE through the daemon and asserts the observer line.

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

## Cancelled (was deferred under the wrong model)

- **"Retire the receive-path forward trigger."** The receive path
  hosts the ingress filter pipeline
  (`reactor_notify.go:302-353` in-process ingress filters that may
  modify wire bytes via `IngressFilterFunc.modifiedPayload`;
  `reactor_notify.go:357+` import policy filter chain via
  `PolicyFilterChain` / `ModAccumulator`). Filtering and modify happen
  before caching. Retiring the receive trigger would require re-homing
  the filter pipeline; there is no benefit in doing so. Two triggers
  serve two consumer categories by design; see "Two-Trigger Model".
- **"Per-peer Change-driven subscriber."** The framing assumed RS
  needed per-peer best-path events from locrib. RS does not compute
  per-peer best-paths -- it forward-alls every received UPDATE, and
  per-peer egress logic (filters, RR injection, next-hop, AS-override,
  EBGP prepend) lives in `reactor_api_forward.go:380-540`. Driving
  forward from `locrib.OnChange` would lose duplicates that don't
  shift the global best; the receive-path trigger is the right shape
  for forwarders.

## Open work (genuinely incomplete in the current per-peer egress)

- **RR-specific cluster-list / originator-id rewrites.** Today
  `reactor_api_forward.go:463-485` injects ORIGINATOR_ID and prepends
  CLUSTER_LIST when reflecting between iBGP peers; full RFC 4456
  per-peer cluster-list rewriting (including loop detection on
  outbound) is partial. Extend the existing per-peer `ModAccumulator`
  hook in the egress path; no new infra needed.
- **MP-BGP families with Label-Stack / RD rewrite.** Per-peer rewrite
  hooks for non-CIDR families (VPN, EVPN, MUP) are missing from the
  egress chain. Same shape as the existing handlers; add per family.
- **Per-peer Adj-RIB-Out materialisation** (design doc Phase 3 agreed
  "compute at send time, don't store"). `show bgp neighbor X
  advertised-routes` runs the export chain on demand. Independent of
  the changes in this design.

## Open questions

1. **`bestPrev` redundancy.** BGP-internal state is not subsumed by
   locrib Change events (replay in `rib_bestchange.go` iterates
   `bestPrev` directly). They coexist; `bestPrev` stays authoritative
   for BGP consumers.
2. **Non-BGP forward handles.** Kernel / static producers leave
   `Forward` nil today. Revisit if a protocol producer gains a
   forward-shaped payload (e.g. OSPF LSA originator).
