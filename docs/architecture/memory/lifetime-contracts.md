# Buffer Lifetime Contracts

> **Context:** Ze holds route/attribute bytes past a call boundary through four
> separate contracts, each with different copy semantics. This doc gives them
> one shared vocabulary and one shared debug-only enforcement so a violation
> crashes loudly in debug instead of silently returning recycled bytes.
> See `docs/architecture/pool-architecture.md` for the attrpool internals and
> `ai/rules/performance.md` / `ai/rules/performance.md` for the
> zero-copy discipline these contracts implement.

## TL;DR

- On the hot path Ze **borrows** bytes (zero-copy slices) rather than copying them.
- A borrow is valid only until a named **boundary** (a handler return, a dispatch
  completion, a handle release). Reading past the boundary reads recycled memory.
- To use bytes past the boundary a consumer must **retain** them (take a
  reference) or **own** a copy — always *before* the boundary.
- The four contracts differ only in *when* they copy (eager / never / lazy /
  none). That difference is load-bearing and the types stay separate.
- What they share is the **enforcement**: at every recycle/release point, debug
  builds `memguard.Poison` the bytes (or, for attrpool, refuse to reuse the
  slot) so a borrow that outlives its boundary reads an obviously-invalid
  pattern. Release builds keep the exact zero-copy borrow with zero added cost.

## Vocabulary

| Term | Meaning |
|------|---------|
| **Boundary** | The point past which borrowed bytes may be recycled: a callback/handler return, the completion of a synchronous dispatch, or a handle's final `Release`. A borrow is only valid *before* its boundary. |
| **Borrow** | A zero-copy slice into memory owned by another layer, valid only until a named boundary. Reading a borrow after its boundary is a use-after-free. |
| **Retain** | To extend a borrow past its boundary by taking a counted reference (`AddRef`) so the owner keeps the bytes alive. Must happen before the boundary. |
| **Own** | To hold memory whose lifetime you control — a `Snapshot` copy, a lazily-materialized `AddRef` buffer, or a live refcounted `Handle`. Safe to read until you release or drop it. |

Poison is applied **at the boundary** (recycle / release), so any borrow that
survives it reads the poison pattern. `memguard` is the one primitive that
writes and detects that pattern.
<!-- source: internal/core/memguard/poison_debug.go -- Poison/IsPoisonedForTest/Enabled -->

## The four contracts

Each contract lends route bytes across a boundary and recycles them afterwards.
They are kept as separate types because their copy semantics are mutually
exclusive; unifying them would force one layer to pay another's cost.

| Contract | Copy semantics | Boundary | Retain / Own mechanism | Debug enforcement |
|----------|----------------|----------|------------------------|-------------------|
| **A. WireUpdate** | **Eager** copy on retain | fire-and-forget event delivery | `Snapshot()` deep-copies the payload (Own) | receive buffer poisoned at recycle |
| **B. attrpool Handle** | **Never** copies (dedup sharing is the point) | handle `Release` (refcount → 0) | `AddRef`/`Intern` refcount (Retain); `Get` returns a Borrow valid while the Handle is live | slot not reused + dead bytes poisoned |
| **C. ribForwardHandle** | **Lazy** copy on first retain | producing handler return | first `AddRef` materializes an owned copy via `sync.Once` (Own) | owned copy poisoned on final `Release` |
| **D. redistevents batch** | **None** (borrow-only) | synchronous `Emit` return | none — the batch MUST NOT be retained past dispatch | entries poisoned (struct sentinel) at `ReleaseBatch` |

### A. WireUpdate — eager copy
Received-UPDATE `RawBytes` point at a reactor receive buffer reused after the
callback; `IsAsyncSafe` reports this. A structured subscriber that needs the
bytes past fire-and-forget delivery calls `Snapshot()`, which owns a copy.
Enforcement: the receive buffer is poisoned when returned to the pool, so a
retained `RawBytes` borrow reads poison in debug.

**An index of offsets shares the boundary of the bytes it indexes.** The
attribute span index (`attribute.SpanIndex`) holds offsets and lengths, never
bytes, so it carries no lifetime of its own: it is exactly as valid as the base
that owns it and must never be published apart from that base. Because the
offsets are relative to the attribute section rather than to the payload, they
survive a copy of identical bytes unchanged, which is why `Snapshot()` carries
the index across instead of rebuilding it. A caller that produces *different*
bytes gets a new index, never a rebased one.
<!-- source: internal/component/bgp/wireu/wire_update.go -- WireUpdate.Snapshot eager copy on retain -->
<!-- source: internal/core/bgp/attribute/wire.go -- AttributesWire.CarryOver -->
<!-- source: internal/component/bgp/types/rawmessage.go -- RawMessage.IsAsyncSafe borrow-vs-owned boundary -->
<!-- source: internal/component/bgp/reactor/session.go -- ReturnReadBuffer receive-buffer recycle/poison point -->

### B. attrpool Handle — never copies
Attribute bytes are interned once and shared (refcounted) across thousands of
routes; copying on retain would destroy the dedup memory win. `Get` returns a
Borrow valid while the Handle is live. The 32-bit Handle is fully packed
(1 bufferBit + 5 poolIdx + 26 slot), leaving no bit for an always-on generation
tag, so ABA (a stale handle to a reused slot) cannot be detected by the handle
alone. Enforcement: debug builds do **not** reuse freed slots, so a released
slot stays dead and a stale handle trips the existing `ErrSlotDead`; the dead
slot's bytes are poisoned too.
<!-- source: internal/component/bgp/attrpool/handle.go -- Handle packed 1+5+26 bits, no spare generation bit -->
<!-- source: internal/component/bgp/attrpool/pool.go -- intern slotReuseEnabled gate + release dead-slot poison -->
<!-- source: internal/component/bgp/attrpool/validate_release.go -- slotReuseEnabled + dead-slot rejection -->

### C. ribForwardHandle — lazy copy
One UPDATE's wire bytes are offered to zero-or-more RIB `Change` subscribers.
Usually nobody retains, so nothing is copied. A subscriber that wants the bytes
past the handler calls `AddRef`, whose first call materializes an owned copy.
Enforcement: the owned copy is poisoned on final `Release`, so reading `Bytes()`
after release reads poison in debug.
<!-- source: internal/component/bgp/plugins/rib/forward_handle.go -- ribForwardHandle lazy copy + Release poison -->
<!-- source: internal/core/rib/locrib/forward_handle.go -- ForwardHandle/ForwardBytes AddRef/Release/Bytes contract -->

### D. redistevents batch — borrow-only
A pooled `RouteChangeBatch` is emitted synchronously; every subscriber has
returned by the time `Emit` returns, which is the boundary. The batch is a
pure Borrow — it MUST NOT be retained past dispatch, and there is no retain
mechanism. Enforcement: `ReleaseBatch` overwrites each entry's *scalar* fields
(`Action`/`Metric`/`Table`/`OriginAS`) with a recognizable sentinel in debug, so
a retained batch read after release sees an obviously-invalid value instead of a
plausible-looking zero. The `netip` `Prefix`/`NextHop` fields stay zero either
way: a zero `netip` is already `IsValid()==false` (not `0.0.0.0/0`), so it needs
no sentinel, and leaving it zero keeps its `z` pointer nil (GC-safe).

Note: `RouteChangeEntry` embeds `netip.Prefix`/`netip.Addr`, which carry an
internal `z` pointer. Raw byte-poisoning would fabricate a bogus pointer and
crash the GC, so contract D uses a struct sentinel (scalar fields set to a
`0xDEADBEEF`-style marker; the `netip` fields left zero so their pointers stay
nil and GC-safe) instead of `memguard.Poison`. It still gates on
`memguard.Enabled` and follows the same boundary discipline.
<!-- source: internal/core/redistevents/pool.go -- ReleaseBatch entry sentinel poison in debug -->

## Enforcement model

- **One primitive.** `memguard.Poison`/`IsPoisoned` is the single byte-poison
  implementation (contracts A, B, C). Contract D uses a struct sentinel because
  `netip` pointers preclude byte-poison, but under the same `memguard.Enabled`
  gate.
- **Debug-only, zero release cost.** `memguard.Enabled` is a compile-time
  constant: `true` under `//go:build debug`, `false` otherwise. Every poison
  call site is wrapped in `if memguard.Enabled { ... }`, so release builds
  dead-code-eliminate the guard, the poison, and any slice expression built as
  its argument. The attrpool Handle ABI stays 32 bits; `Get` gains no
  comparison; no allocation is added on any release path.
- **Silent → loud.** Each contract's previously-silent failure (recycled bytes,
  another route's bytes, freed bytes, zeroed-but-plausible entries) becomes a
  poison read or an `ErrSlotDead` in debug, caught by a debug-tagged test.
- **What debug does not do.** These are test/chaos-time detectors, not
  production guards. Production ABA enforcement for attrpool would need a 64-bit
  Handle with a generation field; that widening is deliberately deferred on a
  memory-cost basis (millions of route entries).
- **Debug memory ceiling.** Because debug never reuses attrpool slots, the slot
  table (`shard.slots`) is append-only and never shrinks — compaction reclaims
  buffer *bytes*, not slot *entries*. A long debug soak that interns enough
  *distinct* attributes will hit `ErrPoolFull` at `MaxSlotsPerShard`
  (4,194,304 per shard) even under steady-state churn where release would
  otherwise keep the table small, and it does not recover. This is a debug-only
  ceiling; release builds reuse slots and never hit it. Size high-churn debug
  soaks accordingly (or run them against a release build).

Run the enforcement with `CGO_ENABLED=0 go test -tags debug ./...`.
