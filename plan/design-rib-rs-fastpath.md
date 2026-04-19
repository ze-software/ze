# Plan: RS/RR unified-with-skip fast path

Working design document. Precedes a formal spec.

## Why

Route-server (RS) and parts of route-reflector (RR) mode forward an
incoming UPDATE to many egress peers byte-for-byte. Today
`internal/component/bgp/reactor/reactor_api_forward.go` (~51K lines,
the largest file in reactor by design) preserves that zero-copy
forward: one wire buffer received from peer A is shared across every
egress peer B...N that needs no attribute rewrite. `ContextID`
identifies "this buffer, unchanged"; the forward-pool refcounts it
and releases once the last peer has sent.

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

## Shape

Two pieces:

1. **Change events carry a forwarding handle.** `locrib.Change` gains
   optional `ContextID` and `Buffer` fields. The BGP producer populates
   them on Insert; non-BGP producers (static, kernel) leave them zero.
2. **Per-peer output worker checks "would my filter modify anything"
   before rebuilding.** If the answer is no, it writes the original
   buffer to the TCP socket. Otherwise it rebuilds from Loc-RIB state
   through the export chain.

Step 1 is additive to Phase 3c. Step 2 is where the real work lives;
it touches `reactor/forward_build.go`, `reactor/filter_chain.go`, and
the per-peer output path.

## Extending locrib.Change

```go
type Change struct {
    Family   family.Family
    Prefix   netip.Prefix
    Kind     ChangeKind
    Best     Path

    // Optional forwarding handle. Non-zero when the producer can hand
    // the consumer a shared wire buffer it may send unmodified. Zero
    // when the consumer MUST rebuild from Best.
    ContextID uint64
    Buffer    *reactor.SharedBuffer  // refcounted, released by consumer
}
```

Cross-package types in `locrib` feel off -- `locrib` is core, reactor
is a component. Cleanest resolution: put `SharedBuffer` (or just the
refcount interface) in `internal/core/rib/locrib/buffer.go` as an
opaque type; reactor implements it. Alternative: locrib exposes a
`ForwardHandle` interface with `AddRef()` / `Release()`; reactor's
buffer type satisfies it.

## Per-peer decision: "will my filter modify this?"

`reactor/filter_chain.go` already tracks whether a filter "modifies"
per-peer (next-hop rewrite, AS-path prepend, attribute strip). Today
`forward_build.go` uses that signal to pick between "forward
unchanged" and "rebuild". The same signal answers our question:

- Filter is a no-op AND peer negotiated the same capabilities as the
  source peer AND no attribute rewrite is configured -> forward the
  original buffer.
- Otherwise -> rebuild from Loc-RIB entry's Path list through export
  chain.

Migration: today the path-through decision happens in forward_build
against the receive-buffer. After this change it happens against the
Loc-RIB change event. Both take the same inputs; the rebuild trigger
just moves up the stack.

## Ordering vs. buffer refcount

The forward pool's refcount is released when the last peer has sent.
Today the forward path holds the only references. After the change,
the Loc-RIB change event holds an additional reference for as long as
the event is in flight. OnChange handlers run synchronously, so the
extra reference is bounded to the dispatch duration.

If a subscriber (sysrib) needs to retain the buffer past dispatch, it
calls AddRef before returning and Release later. Contract matches
existing forward-pool semantics.

## What this is NOT

- **Not** route-server being implemented. RS/RR already exist.
- **Not** a replacement for forward_build / forward_pool. Those stay;
  they become the rebuild-producer side of the per-peer decision.
- **Not** about packet receive / parse. Wire decode is upstream of
  this change and unaffected.

## Migration steps

1. **Add `ForwardHandle` interface to locrib.** Zero value is no-op;
   producers that don't have one leave the fields zero.
2. **BGP populates Change.ForwardHandle on the insert path.** In
   `rib_bestchange.go`, when calling `r.locRIB.Insert`, pass the
   incoming buffer + ContextID via the extended Change. Buffer ref
   is held for the duration of Insert (which dispatches OnChange
   synchronously).
3. **Extend OnChange dispatch to retain the ref.** While a handler
   runs, the buffer is guaranteed alive; when the last handler
   returns, BGP's Insert releases.
4. **Output worker pulls the Change instead of the in-memory RouteEntry.**
   Per-peer output for RS/RR consumes Change.ForwardHandle when
   available; falls back to rebuild from Path when nil or when the
   filter modifies.
5. **Delete the direct-from-forward-pool output path.** The RS/RR
   output is now driven from Loc-RIB change events, not directly from
   the forward pool. Single code path.

Step 5 is the behavior change that satisfies `rules/no-layering.md`:
no dual-path "RS/RR bypass vs. RIB path".

## Files touched

- `internal/core/rib/locrib/buffer.go` -- new; `ForwardHandle`
  interface.
- `internal/core/rib/locrib/change.go` -- Change struct gains two
  fields.
- `internal/core/rib/locrib/manager.go` -- dispatch path passes the
  ref through.
- `internal/component/bgp/plugins/rib/rib_bestchange.go` -- populate
  on Insert; release on Change.Remove.
- `internal/component/bgp/reactor/forward_build.go` -- decide skip vs.
  rebuild using the Change handle plus the per-peer filter flag.
- `internal/component/bgp/reactor/reactor_api_forward.go` -- wire the
  output worker to consume locrib Changes instead of forward-pool
  events.
- `test/plugin/bgp-rs-*.ci` -- extend RS functional tests to assert
  byte-identical forwarding when the filter is a no-op (existing
  tests already assert delivery; add ContextID equality).

## Benchmarks

Gate the commit on:

- `BenchmarkRSForwardNoOp`: 1M prefixes, 20 egress peers, no export
  filter. Must be within 10 percent of the pre-change baseline
  (captured today from `hotpath_bench_test.go`).
- `BenchmarkRSForwardWithPrepend`: same shape with AS-path prepend
  configured on half the peers. Measures the mixed-mode cost.

Either miss -> the design needs rework before the commit lands.

## Deferred

- RR-specific cluster-list / originator-id rewrites still require a
  rebuild. The skip path applies only when the filter is a true no-op.
  Documenting this as an explicit AC in the spec.
- MP-BGP families with Label-Stack / RD rewrite need the same
  "filter modifies?" check extended to the rewrite layer. Add per
  family as needed; the infrastructure is the same.
- Per-peer Adj-RIB-Out materialisation (design doc Phase 3 agreed
  "compute at send time, don't store"): this spec is the mechanism
  that makes that affordable. `show bgp neighbor X advertised-routes`
  runs the export chain on demand; the skip path keeps normal
  forwarding fast.

## Open questions

1. Does BGP-only `bestPrev` state become redundant once RS/RR drives
   from locrib Change events? Not for BGP replay (rib_bestchange's
   replay path iterates bestPrev directly). The two coexist;
   bestPrev is BGP-internal authoritative state for BGP consumers.
2. Change-event ordering across peers: the forward pool guarantees
   that a given buffer's send completion is observed before the
   buffer is released. OnChange is synchronous. Confirm with a
   race-coverage run (`make ze-race-reactor`) when the spec is
   implemented.
3. Do non-BGP Change producers ever have a forward handle? Not today
   (kernel routes are produced without a wire buffer). Leave the
   fields zero-valued; revisit if a protocol producer gains a
   forward-shaped payload.
