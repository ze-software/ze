# Unified RIB — Design Notes

Working design document (not a spec). Captures the conversation between
Thomas and Claude on 2026-04-19 about reshaping the ze RIB. Reference
this when implementing; update it when a decision changes.

## Why

Today ze has two separate RIBs:

- `internal/component/bgp/plugins/rib/` — BGP routes: Adj-RIB-In per
  peer, best-path selection among BGP sources, show commands.
- `internal/plugins/sysrib/` — the kernel-facing side (reads and
  writes the OS routing table).

There is no single Loc-RIB that arbitrates across protocols. BGP picks
its best, sysrib pushes it to the kernel, and the kernel arbitrates
with static / connected / OSPF. The goal is to bring that arbitration
into ze so one store holds routes from every source, with best-path
selection across sources, and peers / kernel sync all speak to it
through the existing event bus.

## Decisions

Agreed in the 2026-04-19 conversation.

| # | Decision | Notes |
|---|---|---|
| 1 | One unified Loc-RIB store across protocols. No opaque blob per source. BGP candidates use the existing `attrpool` (per-attribute dedup, refcounted). Non-BGP candidates carry small typed fields inline (metric, tag, distance, flags). | Avoids the zebra-style `(prefix -> nexthops + protocol blob)` pattern. Keeps attribute dedup on the BGP hot path. |
| 2 | Adj-RIB-In stored per peer (pre-policy). Loc-RIB is the merged store. Adj-RIB-Out is computed at send time from Loc-RIB through the per-peer export chain — not stored. | Matches RFC 4271 three-tier model. `show bgp neighbor X advertised-routes` re-runs the export chain at query time. BIRD-equivalent behavior. |
| 3 | ~~RS / RR fast path preserved. RIB is on the path, but the per-peer output worker skips the rebuild when the export filter is a no-op. The change event carries the `ContextID` and the pooled buffer reference.~~ | **Cancelled (2026-04-20, commit c645540e2).** Wrong model. RS does not go through locrib for forwarding -- it is a forward-all dispatcher (`rs/server_forward.go:34-56`); per-peer egress (filters, RFC 4456 RR injection, next-hop, AS-override, EBGP prepend) lives in `reactor_api_forward.go:380-540` and is driven by the receive-path trigger. The receive path is also load-bearing for the inbound filter pipeline (`reactor_notify.go:302-353` ingress filters with `IngressFilterFunc.modifiedPayload`; `reactor_notify.go:357+` import policy filter chain). Two triggers (StructuredEvent for forwarders, `locrib.OnChange` for state trackers) coexist by design. The "change event carries `ContextID` + pooled buffer reference" half landed as `Change.Forward` (`internal/core/rib/locrib/forward_handle.go`), but only as state-tracker infra (sysrib mirror, route archive), NOT as a forwarder mechanism. See `plan/design-rib-rs-fastpath.md` "Two-Trigger Model" and `docs/architecture/core-design.md` "Ingress Filter Pipeline". |
| 4 | N parallel RIB workers, sharded by prefix hash. Each shard owns a disjoint slice of the prefix space; operations on a given prefix always land on the same shard. | Today's RIB is single-writer under one `sync.RWMutex` (`rib.go:246`). Sharding is additive, not a move — see deferred. |
| 5 | BART is the prefix index for every case. Path-id moves from the store key into the value layer (a per-prefix candidate list). Kills the ADD-PATH map fallback (`store_bart.go:37-43`). | BART is already vendored (`github.com/gaissmai/bart`) and is the default (`familyrib_bart.go`, build tag `!maprib`). |
| 6 | Each peer already runs its own goroutine. Confirmed at `peer.go:673` (`go p.run()`). Processing (filter chain, RIB apply) stays on shared workers so one full-feed peer does not starve customer peers. | Not a change — factual answer to the question raised. |

## Deferred

Carry into later specs. Not part of the initial reorganization.

| Topic | Why deferred |
|---|---|
| N-shard worker model | Behavior change to the Loc-RIB manager, not a file move. Land after the reorganization compiles and tests pass. Has its own design doc: `plan/design-rib-shard.md`. |
| ~~Unified-with-skip RS / RR path~~ | ~~Requires the change event to carry `ContextID` + pooled buffer ref, and a "will my filter modify?" decision per peer. Touches `reactor_api_forward.go` + filter chain. Separate spec.~~ **Cancelled (2026-04-20).** Same wrong model as Decision 3. The "will my filter modify?" decision per peer already exists in `reactor_api_forward.go:380-540` and runs at forward time, driven by the receive-path trigger. There is nothing to move. See cancelled rows in `plan/deferrals.md`. |
| Non-prefix SAFIs (flow, EVPN, MVPN, MUP, RTC, bgp-ls) | BART keys on `netip.Prefix`. These need family-specific indexes behind a common `FamilyIndex` interface. One spec per family as the need arises. |

## Current state

Files confirmed to exist and their roles.

### BGP RIB plugin — `internal/component/bgp/plugins/rib/`

The BGP-shaped RIB today.

- `rib.go` (32K) — `RIBManager`, single `sync.RWMutex` protecting per-family maps.
- `bestpath.go` / `bestpath_test.go` — RFC 4271 §9.1.2 tiebreakers. BGP-only.
- `rib_bestchange.go` — best-path change events published via `EventBus`.
- `rib_commands.go` (34K), `rib_pipeline.go`, `rib_pipeline_best.go`,
  `rib_structured.go`, `rib_attr_format.go`, `rib_nlri.go` — show
  commands and query plumbing.
- `compaction.go` — pool compaction scheduler.
- `register.go` — plugin registration.
- `events/`, `schema/` — event types and YANG schema.
- `pool/` — per-attribute pools (BGP-specific, keeps the attribute
  dedup machinery).
- `storage/`:
  - `familyrib.go` — `FamilyRIB` shared helpers (`entriesEqual`, `ToWireBytes`).
  - `familyrib_bart.go` — default backend (build tag `!maprib`). Wraps `Store[RouteEntry]`.
  - `familyrib_map.go` — map fallback (build tag `maprib`).
  - `store_bart.go` — generic `Store[T]`. **Has the ADD-PATH branch that decision 5 collapses.**
  - `store_map.go` — map-only fallback.
  - `nlrikey.go` — `NLRIKey`, `NLRIToPrefix`, `PrefixToNLRI`.
  - `routeentry.go` — BGP `RouteEntry` (pool handles for each attribute).
  - `peerrib.go` — per-peer Adj-RIB-In bookkeeping.
  - `attrparse.go` — BGP attribute parsing into pool handles.

### Non-BGP RIB — `internal/plugins/sysrib/`

- `sysrib.go` (14K) — kernel RIB abstraction (reads from kernel, writes to kernel).
- `register.go`, `events/`, `schema/` — standard plugin plumbing.

### Reactor and hot path — `internal/component/bgp/reactor/`

Stays put. Noted for context because the RIB redesign must not
regress any of this.

- `peer.go` + `peer_*.go` — peer goroutine (one per peer, `Start` at `peer.go:658`).
- `reactor_api_forward.go` (51K) — the RS / RR zero-copy forward path.
- `forward_pool*.go` — refcounted buffer pool, ring-backed.
- `filter_chain.go`, `filter_delta*.go`, `forward_build.go` — export filter plumbing.
- `reactor_api.go` + `reactor_api_batch.go` — plugin-facing API.

### Core primitives — `internal/core/`

Existing homes for cross-component primitives.

- `family/` — `family.Family` (AFI + SAFI wrapper).
- `env/`, `metrics/`, `slogutil/`, `redistevents/`.

## Target shape

After the three phases below.

### New: `internal/core/rib/`

```
internal/core/rib/
  store/                     # generic NLRI → T store, BART-backed
    store_bart.go            # moved from bgp/plugins/rib/storage/
    store_map.go             # moved from bgp/plugins/rib/storage/
    nlrikey.go               # moved from bgp/plugins/rib/storage/
  locrib/                    # the unified Loc-RIB (new, thin)
    candidate.go             # Candidate interface
    entry.go                 # Entry { prefix, []Candidate, best }
    manager.go               # Manager, initially single-writer, shard-ready later
```

Rationale for `core/`: the generic `Store[T]` and the Loc-RIB are
cross-component. BGP feeds it; sysrib feeds it and reads it. Neither
one owns the Loc-RIB, so it cannot live under either.

### BGP RIB becomes "BGP candidate source"

`internal/component/bgp/plugins/rib/` stays. Its conceptual role narrows:

- Holds Adj-RIB-In per peer (as today).
- Computes the best-among-BGP-candidates for each prefix (bestpath.go).
- Publishes the BGP winner into Loc-RIB (new: one small hook from
  `rib_bestchange.go`).
- Owns BGP-specific show commands, which now query Loc-RIB + BGP's
  adj-rib-in for operator-facing data.

Files stay where they are. The manager edit is small.

### Sysrib becomes "kernel candidate source + FIB consumer"

`internal/plugins/sysrib/` stays. Conceptual role:

- Publishes kernel-learned routes (static, connected, kernel-manual)
  into Loc-RIB as non-BGP candidates.
- Subscribes to Loc-RIB best-path changes and programs the kernel FIB.

Files stay where they are. Wiring changes.

## Phases

Each phase compiles and tests pass before the next starts. If a phase
breaks something, it rolls back cleanly because the moves are
mechanical.

### Phase 1 — extract the generic store (moves only) -- DONE

Three files with no BGP imports move from `bgp/plugins/rib/storage/`
to `internal/core/rib/store/`:

| From | To |
|---|---|
| `internal/component/bgp/plugins/rib/storage/store_bart.go` | `internal/core/rib/store/store_bart.go` |
| `internal/component/bgp/plugins/rib/storage/store_map.go` | `internal/core/rib/store/store_map.go` |
| `internal/component/bgp/plugins/rib/storage/nlrikey.go` | `internal/core/rib/store/nlrikey.go` |

Package rename from `storage` to `ribstore` (or `store`, pick one).
Update imports in `familyrib_bart.go` / `familyrib_map.go` / any test
files. `NLRIToPrefix` / `PrefixToNLRI` take a `family.Family`; no
circular import risk because `core/family` is leaf-level.

Zero behavior change. `make ze-verify-fast` must pass.

### Phase 2 — collapse the ADD-PATH branch -- DONE

`internal/core/rib/store/store_bart.go:30-33` is now `Store{fam, trie}`
with no ADD-PATH bifurcation. Path-id moved into BGP's value layer as
`pathSet` (`internal/component/bgp/plugins/rib/storage/familyrib.go:46`
declares `multi *store.Store[pathSet]`). `store_bart.go:17-22` documents
the contract: callers needing per-path-id semantics put a path-id -> T
map in the value layer, keeping `Store` itself generic and non-branching.

One-file surgical edit to `store_bart.go` (after the Phase 1 move).
The `trie *bart.Table[T]` OR `routes map[NLRIKey]T` bifurcation
disappears. BART is the only prefix index. Path-id moves up into the
value layer — specifically, `FamilyRIB` now holds a per-prefix
candidate list inside `RouteEntry` (or a sibling wrapper) keyed by
path-id.

Map fallback (`store_map.go`, `-tags maprib`) keeps the same single-
path shape for benchmarking parity.

Touches: `store/store_bart.go`, `store/store_map.go`,
`bgp/plugins/rib/storage/familyrib.go`, `familyrib_bart.go`,
`familyrib_map.go`, possibly `routeentry.go` if the candidate list
lives there.

Behavior change: ADD-PATH sessions now get a real trie (LPM,
iteration, no hash collisions). Test with `bgp plugin` cases that
exercise ADD-PATH.

### Phase 3 — add `locrib` (new, thin) -- DONE

New package `internal/core/rib/locrib/`:

- `candidate.go` — `Candidate` interface with methods for source
  identity, comparison hooks, and the minimum state needed for
  cross-source best-path.
- `entry.go` — `Entry { prefix, []Candidate, best }`.
- `manager.go` — `Manager` backed by one `*store.Store[*Entry]` per
  family. Single-writer initially; sharding is Phase 4.

BGP plugin edits:

- `bgp/plugins/rib/rib_bestchange.go` — when BGP best changes, publish
  a BGP-sourced `Candidate` into Loc-RIB instead of directly emitting
  the final best-path event.

Sysrib edits:

- Register as a candidate source publishing kernel-learned routes.
- Subscribe to Loc-RIB best-path changes for FIB programming.

No files move in Phase 3. All changes are additions to existing
packages plus the new `locrib/` directory.

### Phase 4 and beyond (deferred)

- Shard the Loc-RIB manager by prefix hash. Requires a shard table,
  per-shard locks, and a fan-out iterator for cross-shard queries. See
  `plan/design-rib-shard.md`.
- ~~Unified-with-skip RS / RR fast path: carry `ContextID` + buffer ref
  on change events; output worker decides to skip rebuild.~~
  **Cancelled (2026-04-20).** Wrong model -- see Decision 3 and the
  cancelled deferred row above. The infrastructure that would have
  served it (`Change.Forward` carrying a buffer handle) shipped as
  state-tracker infra in `c645540e2`; it does not drive forwarders.
- Non-prefix SAFIs: `FamilyIndex` interface behind which BART sits for
  prefix-shaped families and specialized indexes sit for flow / EVPN /
  MVPN / MUP / RTC / bgp-ls.

## Rules this touches

Flagged so reviewers know what to verify.

- `rules/design-principles.md` — encapsulation onion, buffer-first,
  lazy-over-eager. The unified Loc-RIB must not introduce copies on
  the RS / RR path (enforced by Phase 4, not regressed in Phase 3).
- `rules/enum-over-string.md` — `ProtocolID` on `Candidate` must be a
  typed numeric identity, not a string.
- `rules/exact-or-reject.md` — cross-protocol best-path rules that
  cannot be applied exactly must reject at config verify.
- `rules/buffer-first.md` — any new encoding helpers on the Loc-RIB
  path write into pooled buffers.

## Open questions

1. ~~Where does per-path-id live on the entry?~~ **Resolved.** Option (a)
   at both layers: locrib's `PathGroup.Paths []Path` keyed by
   `(Source, Instance)` (`internal/core/rib/locrib/entry.go:14-22`);
   BGP storage's `pathSet` per prefix in
   `multi *store.Store[pathSet]` for ADD-PATH families.
2. ~~Best-path across sources: admin-distance table or configurable?~~
   **Resolved as admin-distance table.** `Path.AdminDistance uint8`
   (`internal/core/rib/locrib/candidate.go:43`); `selectBest`
   (`entry.go:74-96`) orders by AdminDistance then Metric. Cisco /
   Juniper defaults documented on the field. YANG override remains
   future work but is additive.
3. Non-prefix SAFIs: land the `FamilyIndex` interface in Phase 3 (even
   unused for prefix families), or introduce it only when the first
   non-prefix family is added? Leaning toward the second — YAGNI.
   Still open; current code stays YAGNI.
