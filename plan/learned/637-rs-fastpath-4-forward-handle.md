# 637 -- rs-fastpath-4-forward-handle

## Context

Phase 3b mirrors BGP best-path into the cross-protocol Loc-RIB. After
that phase, any subscriber that wants to forward a received UPDATE
without rebuilding attributes has no way to get at the producer's wire
buffer via the locrib Change stream. The design-rib-rs-fastpath.md
goal (preserve zero-copy forwarding for RS/RR) needs a handle that
flows with the Change. This session landed the infrastructure for that
handle end-to-end (locrib API + BGP producer + observability
subscriber + functional test) without touching the reactor's
single-owner-release buffer model or the deferred RS/RR per-peer
design.

## Decisions

- **Opaque `locrib.ForwardHandle` interface** with `AddRef()` /
  `Release()` and an optional `locrib.ForwardBytes` sidecar for the
  retained-bytes accessor. Chose an optional-interface pattern over
  adding `Bytes()` to the main interface so a future zero-copy
  reactor-backed handle can satisfy `ForwardHandle` without forcing
  `Bytes()` on every implementation.
- **Interface in `locrib`, not `reactor`.** `locrib` is core; reactor
  imports core. A concrete `*reactor.SharedBuffer` field on
  `Change.Forward` would be a cycle. Subscribers type-assert to the
  optional `ForwardBytes` from locrib alone (no rib import needed).
- **Two Insert methods, not variadic options.** `Insert` stays
  unchanged for every non-BGP caller; new `InsertForward` accepts the
  handle. Shared private `insert()` does the dispatch. Avoided
  `opts ...Option` churn across ~50 existing call sites.
- **`ChangeHandler` dispatch stays synchronous under the RIB write
  lock.** Drafts considered enqueuing onto a worker channel so the
  subscriber could do TCP writes; rejected because of how big that
  makes the diff AND because the current locrib contract ("cheap,
  non-blocking handlers only; offload heavy work to a goroutine")
  already tells subscribers what to do. AddRef must be cheap; anything
  heavy goes to the subscriber's own worker.
- **Chose copy-on-first-AddRef over eager copy or no-copy.** Eager
  would pay an allocation per UPDATE regardless of subscriber
  presence. No-copy would let subscribers hit use-after-free when
  `RawBytes` is reused. Copy-on-first-AddRef via `sync.Once` pays the
  copy only when a subscriber retains, at most once per handle.
- **Scope narrowed.** The original design aimed at retiring the
  receive-path forward trigger entirely; blocked on per-peer best-path
  events for RS mode (each RS peer has its own filtered best). Only
  the additive producer + observer landed. Retiring the old trigger
  remains deferred with an explicit entry in `plan/deferrals.md`.
- **First consumer is observability, not a byte-consumer.** No real
  subscriber wants the wire bytes today (sysrib is a FIB consumer, not
  a BGP forwarder). The `observeForwardHandles` subscriber nil-checks
  only and debug-logs; the AddRef + Bytes path is exercised only in
  unit tests until an RS/RR or archival subscriber lands.

## Consequences

- `locrib.Change` grows one pointer-sized field. Insert callers that
  don't need a handle keep calling `Insert` verbatim; benchmarks show
  the `InsertForward(..., nil)` path is within noise of `Insert`.
- Every BGP UPDATE that triggers a best-path change now allocates one
  `ribForwardHandle` struct (unused fields outside debug / future
  consumer). When an operator flips `bgp.rib=debug`, the observer
  writes one log line per Change under the RIB write lock; the
  `Enabled()` gate keeps the warn/info path zero-cost.
- The interface is stable enough for a future zero-copy reactor
  producer to replace `ribForwardHandle` without touching consumer
  code. The AddRef/Release contract already matches the forward-pool's
  refcount semantics.
- RS/RR per-peer best-change events remain blocked. Retiring the
  receive-path forward trigger cannot land until per-peer Change
  events exist in locrib or an equivalent.

## Gotchas

- **`ChangeKind.String()` was documented "never used on the hot
  path."** The observer's debug log breaks that invariant when enabled.
  Fixed by gating on `logger().Enabled(ctx, slog.LevelDebug)` before
  any `.String()` call; warn/info still observe the original
  invariant. The comment on `ChangeKind.String()` was softened to
  reflect the opt-in debug path.
- **Typed-nil interface pitfall.** Passing `(*ribForwardHandle)(nil)`
  into `InsertForward` wraps a non-nil interface around a nil concrete
  pointer; subscribers doing the standard `if c.Forward != nil` guard
  then panic on method dispatch. Documented in both
  `ForwardHandle` and `InsertForward` godocs; callers must pass
  untyped nil (or use `Insert`).
- **`RawBytes` is unsafe after the producing handler returns** --
  `bgptypes.RawMessage.IsAsyncSafe()` returns false for received
  UPDATEs. The sync.Once copy inside `AddRef` runs synchronously from
  the handler (under the RIB write lock), so the copy completes while
  `source` is still valid. Any future refactor that moves `AddRef`
  off-handler MUST carry the source-retention guarantee forward.
- **`.ci` tests that rely on timing under parallel-load flake.** The
  first functional test drafted a 1.5s hardcoded sleep before
  dispatching `daemon shutdown`; that matches an existing
  known-failures category. Replaced with polling on `bgp rib show
  best` for the expected prefix, which is race-free.
- **`ChangeUpdate` synthesized by `Remove` carries `Forward == nil`**
  (fallback to next-best). `PathGroup` paths don't retain per-path
  buffers, so there's no handle to pass on that edge. Documented on
  the `Change.Forward` field.

## Files

**New:**
- `internal/core/rib/locrib/forward_handle.go` (ForwardHandle +
  ForwardBytes interfaces)
- `internal/component/bgp/plugins/rib/forward_handle.go`
  (`ribForwardHandle` with sync.Once copy-on-AddRef + Bytes accessor)
- `internal/component/bgp/plugins/rib/forward_observer.go`
  (`observeForwardHandles` subscriber)
- `test/plugin/rib-forward-handle-observed.ci` (end-to-end functional
  test: peer sends UPDATE -> observer logs the Change)

**Modified:**
- `internal/core/rib/locrib/change.go` (Change.Forward field;
  ChangeHandler godoc; ChangeKind.String invariant softened)
- `internal/core/rib/locrib/manager.go` (InsertForward method; shared
  private insert helper)
- `internal/component/bgp/plugins/rib/rib.go` (SetLocRIB idempotence +
  observer lifecycle)
- `internal/component/bgp/plugins/rib/rib_bestchange.go`
  (checkBestPathChange forward param; InsertForward at the mirror
  site)
- `internal/component/bgp/plugins/rib/rib_structured.go` (create one
  handle per received UPDATE; thread through)
- `plan/design-rib-rs-fastpath.md` (scope narrowed, decisions
  captured)
- `plan/deferrals.md` (producer wiring closed; first-real-consumer
  opened)

**Tests:**
- `internal/core/rib/locrib/locrib_test.go` (5 new: nil-handle
  dispatch, handle propagation, nil-handle equivalence, subscriber
  AddRef contract, Remove-carries-no-handle, ForwardBytes optional
  cast; plus 3 benchmarks)
- `internal/component/bgp/plugins/rib/rib_bestchange_test.go` (6 new:
  handle propagation end-to-end, empty-bytes short-circuit, refcount
  lifecycle, lazy copy + non-aliasing, concurrent AddRef, observer
  logs-on-non-nil and quiet-when-debug-off)
