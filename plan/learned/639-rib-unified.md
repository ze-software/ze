# 639 -- rib-unified

## Context

Before this work ze had two RIBs: the BGP-shaped one in
`internal/component/bgp/plugins/rib/` (Adj-RIB-In per peer + best-among-
BGP) and `internal/plugins/sysrib/` (kernel facing). Cross-protocol
arbitration happened in the kernel. The goal was to bring arbitration
into ze: one store holds candidates from every source, runs cross-
source best-path, and feeds the kernel FIB through existing event-bus
plumbing. The 2026-04-19 design conversation captured the shape; three
phases landed in subsequent commits. A fourth phase ("RS / RR fast
path on the unified RIB with a skip-when-no-modify shortcut") was
cancelled on 2026-04-20 once the production-side flow was re-read end
to end -- the framing was wrong about how RS forwards.

## Decisions

- **Unified Loc-RIB lives in `internal/core/rib/locrib/`**, not in any
  one component. Chose `core/` over BGP-internal because both BGP
  (candidate source + reader) and sysrib (candidate source + FIB
  consumer) need it; neither owns it.
- **Generic store extracted to `internal/core/rib/store/`** with no
  BGP imports. `Store[T]` is BART-backed; `nlrikey.go` carries
  `NLRIToPrefix` / `PrefixToNLRI` keyed on `family.Family`. Map
  fallback (`-tags maprib`) preserved for benchmarking parity.
- **ADD-PATH collapsed into the value layer.** `Store` no longer
  bifurcates between trie and `map[NLRIKey]T`; BART is the only prefix
  index. Per-path-id semantics live in BGP storage's `pathSet`
  (`multi *store.Store[pathSet]`); locrib's `PathGroup.Paths` is keyed
  by `(Source, Instance)`. Behaviour gain: ADD-PATH sessions get LPM
  and iteration; behaviour cost: callers needing per-path-id put a
  path-id -> T map in the value layer.
- **Cross-source best path via admin-distance table.** `Path.AdminDistance
  uint8` orders before metric in `selectBest`; defaults follow Cisco /
  Juniper. YANG override is additive future work, not a blocker.
- **Two Insert methods, not variadic options** for the BGP -> locrib
  mirror site -- `Insert` stays for non-BGP callers; `InsertForward`
  threads the optional `ForwardHandle`. Detail in 637.
- **Cancelled Decision 3 / Phase 4 "unified-with-skip RS / RR fast
  path".** The framing assumed RS / RR could be driven from
  `locrib.OnChange` if the Change event carried `ContextID` + a buffer
  ref. Re-reading the production path proved it wrong on three
  counts. (a) RS in ze is forward-all
  (`rs/server_forward.go:34-56`), not a per-peer best-path computer;
  there is no per-peer Change to subscribe to. (b) Per-peer egress
  (egress filters, RFC 4456 RR injection, next-hop, AS-override, EBGP
  prepend) lives in `reactor_api_forward.go:380-540` and is keyed off
  the per-received-UPDATE trigger, not per-best-change. (c) The
  receive-path trigger is also where the inbound filter pipeline
  runs (`reactor_notify.go:302-353` in-process ingress filters with
  copy-on-modify; `reactor_notify.go:357+` external-plugin import
  policy chain). Retiring it would re-home the filter pipeline for no
  benefit. Two triggers (StructuredEvent for forwarders, OnChange for
  state trackers) coexist by design.
- **`Change.Forward` is state-tracker infra, not forwarder infra.**
  The handle that landed in 637 is for sysrib byte-mirror, route
  archive, RR cluster-list extractor -- consumers downstream of the
  rib plugin that want the post-filter wire bytes without going back
  to StructuredEvent. RS / RR will never use it.
- **Sharding deferred to its own spec** (`plan/design-rib-shard.md`).
  Behaviour change to the Loc-RIB manager, not a file move; lands
  after the reorganization compiles and tests pass.
- **`FamilyIndex` interface deferred (YAGNI).** Today every supported
  family is prefix-shaped, so BART-keyed-on-`netip.Prefix` covers them
  all. Introduce the abstraction when the first non-prefix family
  (flow, EVPN, MVPN, MUP, RTC, bgp-ls) actually lands -- not before.

## Consequences

- BGP's `rib_bestchange.go` no longer emits the final cross-protocol
  best directly; it publishes a BGP-sourced `Candidate` into locrib,
  which arbitrates across sources. Sysrib registers as a candidate
  source for kernel-learned routes and subscribes to locrib best-path
  changes for FIB programming.
- `internal/core/rib/locrib/` is the one place that owns cross-protocol
  best-path arbitration. Future protocol sources (OSPF, static)
  register the same way -- no plumbing needed beyond a `Candidate`
  with an `AdminDistance`.
- The receive-path / `StructuredEvent` -> per-peer egress flow in
  `reactor_api_forward.go` is preserved untouched. Anyone proposing to
  consolidate triggers should re-read the cancellation reasoning above
  before opening the discussion -- the prior session lost hours to it.
- Sharding (`plan/design-rib-shard.md`) is the only remaining "active"
  follow-up. The locrib `OnChange` contract (synchronous under the RIB
  write lock; subscribers must be cheap, defer heavy work to a
  goroutine) is the constraint sharding must keep.
- Non-prefix SAFIs and the YANG admin-distance override are future
  work. Each has a natural landing point (a per-family spec; a YANG
  augment over the admin-distance defaults) and neither blocks
  anything that exists today.

## Gotchas

- **The "two triggers per UPDATE" is by design, not a bug to be
  consolidated.** Forwarders fire per received UPDATE (including
  duplicates that don't change best); state trackers fire per best-
  change. They cannot collapse unless locrib stores every received
  path per peer, which is not what a Loc-RIB is.
- **Banned phrases that signal the wrong model is creeping back in:**
  "retire the receive-path trigger", "single trigger", "per-peer
  best-path Change events", "drive forwarding from locrib".
- **`OnChange` subscribers must stay non-blocking.** Handlers run
  synchronously under the locrib write lock. AddRef / Bytes copy on
  the handle is cheap and lock-safe (sync.Once + atomic); anything
  else must be deferred to the subscriber's own goroutine.
- **ADD-PATH families now key the BGP value layer, not the store.**
  Code that previously walked the `multi` map needs to walk
  `pathSet` inside the trie value. Single-path families are
  unaffected.
- **The cancelled rows in deferrals.md (2026-04-20, design-rib-rs-
  fastpath sources) carry the load-bearing wrong-model citations.**
  If a future session re-opens "consolidate the triggers", they are
  the file:line evidence to show first.

## Files

**Created (Phase 1-3):**
- `internal/core/rib/store/store_bart.go` (moved from
  `bgp/plugins/rib/storage/`, ADD-PATH branch removed)
- `internal/core/rib/store/store_map.go` (moved, single-path shape)
- `internal/core/rib/store/nlrikey.go` (moved)
- `internal/core/rib/locrib/candidate.go` (Candidate, Path,
  AdminDistance)
- `internal/core/rib/locrib/entry.go` (PathGroup, Entry, selectBest)
- `internal/core/rib/locrib/manager.go` (Manager, single-writer,
  shard-ready)
- `internal/core/rib/locrib/change.go` + `default.go` +
  `forward_handle.go` (Change stream + process-wide instance + handle
  contract; `forward_handle.go` body in 637)

**Modified:**
- `internal/component/bgp/plugins/rib/storage/familyrib*.go` (use
  `core/rib/store`; ADD-PATH moves to `pathSet` value layer)
- `internal/component/bgp/plugins/rib/storage/pathset.go` (new value-
  layer ADD-PATH wrapper)
- `internal/component/bgp/plugins/rib/rib_bestchange.go` (publish
  BGP-sourced Candidate into locrib; mirror site for `InsertForward`)
- `internal/component/bgp/nlri/nlrisplit/*.go` (per-family NLRI
  splitting consumers of the moved keys)
- `internal/plugins/sysrib/` (registers as candidate source +
  subscribes to best-path changes for FIB programming)

**Deferred (own specs / docs):**
- `plan/design-rib-shard.md` -- N-shard worker model
- Future per-family specs -- non-prefix SAFI `FamilyIndex` interface
- Future YANG augment -- operator override of admin-distance defaults
