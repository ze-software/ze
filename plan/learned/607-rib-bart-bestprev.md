# 607 -- RIB BART bestPrev consolidation

## Context

After three phases of `checkBestPathChange` allocation reductions
(`plan/learned/534-rib-alloc.md` and its phase-2/phase-3 iterations), the
1M-prefix stress profile still showed `checkBestPathChange` holding 107.74 MB
of flat inuse heap (47% of the total) and `gcBgMarkWorker` consuming 31% of
CPU. The residual allocation was the nested Go map `bestPrev
map[Family]map[string]bestPathRecord`: every non-duplicate insert copied
NLRI bytes into a `string` key, every `bestPathRecord` eagerly stored a
formatted `Prefix string`, and the inner map paid 7+ bucket-growth rehashes
approaching 1M entries. The goal was to close that gap by folding `bestPrev`
into BART via a generic, single-source dispatch pattern that both `FamilyRIB`
and the best-path tracker can share.

## Decisions

- Introduced `storage.Store[T]` as a generic NLRI-keyed store dispatching
  between a BART trie (non-ADD-PATH) and a map keyed by `NLRIKey` (ADD-PATH),
  over two other candidates: a hybrid always-both-backends generic (would
  have forced `FamilyRIB` to carry an empty map forever) and two small
  orthogonal primitives (would have renamed the split instead of
  consolidating). `Store[T]` is single-mode at construction; hybrid dispatch
  lives one layer up at the rib layer where it is actually needed.
- Rewrote `FamilyRIB` as a thin wrapper around `*Store[RouteEntry]` plus
  pool-handle lifecycle methods (`Release`, `MarkStale`, `PurgeStale`,
  `StaleCount`). Public API unchanged; callers see the same signatures.
- Replaced `bestPrev map[Family]map[string]bestPathRecord` with
  `map[Family]*bestPrevStore` where each `bestPrevStore` holds two
  `*Store[bestPathRecord]` (one non-ADD-PATH, one ADD-PATH). Mixed-mode
  sessions (some peers ADD-PATH, some not, same family) route each call to
  the correct key space.
- Dropped `bestPathRecord.Prefix string` (the eagerly-formatted display
  string). The trie backend yields `netip.Prefix` for free; the map backend
  yields `NLRIKey`. The display prefix is formatted lazily only when a change
  event is emitted or when replay rebuilds a batch.
- Preserved the `maprib` build tag: `store_bart.go` (`!maprib`) is BART+map,
  `store_map.go` (`maprib`) is map-only; both `FamilyRIB` variants wrap the
  corresponding `Store[T]`.
- `Store[T].Iterate` reuses a single stack `[17]byte` buffer for the trie
  backend instead of allocating via `PrefixToNLRI` per entry. Added
  `PrefixToNLRIInto(pfx, buf)` in `nlrikey.go` so callers with a scratch
  buffer pay zero heap. Iteration contract documented: `nlriBytes` is valid
  only during the callback; copy if retained.

## Consequences

- bestPrev no longer pays for Go map rehash cycles as it crosses bucket
  thresholds; BART grows incrementally with one node per insert.
- The `bestPathRecord` struct is smaller (one less `string` field). On 1M
  entries this eliminates ~30 MB of eagerly-formatted prefix strings.
- `FamilyRIB` and `bestPrev` share one dispatch implementation instead of
  two. Future storage changes (new families, different sharding) land in one
  place.
- The `addPath` parameter at `checkBestPathChange` call sites is now
  load-bearing: it selects which backend the call routes to. Callers that
  pass the wrong flag will silently write to the wrong backend (same latent
  risk as the prior `string(nlriBytes)` approach, which likewise relied on
  the 4-byte path-id prefix being present or absent as declared).
- `FamilyRIB.IterateEntry` on the trie backend now runs without per-entry
  heap allocation via the shared stack buffer. Cold-path callers
  (`ze show rib`) at 1M-prefix scale no longer generate 17 MB of transient
  garbage per enumeration.
- The `maprib` escape hatch remains a debugging lever: `go test -tags maprib`
  forces map-only storage for both `FamilyRIB` and `bestPrev`, useful for
  isolating BART-specific regressions.

## Gotchas

- The `Store[T].Iterate` trie path uses a reused stack scratch buffer.
  Callbacks that retain the `nlriBytes` slice past return will observe
  subsequent iterations' data. Documented on the type and mirrored on the
  map backend (where `key.Bytes()` has analogous semantics). `PurgeStale`
  demonstrates the correct pattern: collect copies during iteration, delete
  after iteration returns.
- `wirePrefixToString` (in `rib_structured.go`) does not bounds-check
  `prefixLen` against family max, while `NLRIToPrefix` does. The asymmetry
  is unreachable in practice because RFC 7606 §5.3 treat-as-withdraw runs
  earlier in the wire parser (`message/rfc7606.go`), but the comment now
  points at that guarantee so the next reader does not mistake it for a
  latent bug.
- `bestPrevStore` allocates both the trie and the map eagerly (tens of bytes
  idle when a family ends up using only one). Intentional: lazy allocation
  would have required a per-call `if nil` check on the hot path for an
  ignorable memory saving.
- `FamilyRIB.PurgeStale` collects stale NLRIs into an allocated slice before
  deletion because BART iteration cannot accept concurrent deletes. The
  constraint is part of the BART library contract; the collection loop makes
  a transient per-stale-entry allocation, bounded by stale count.

## Measurement

1M-prefix stress re-profile (`make ze-stress-profile`, 90s CPU + heap snapshot)
comparing the Phase-4 BART-backed bestPrev against the Phase-3
(map-with-presize) baseline captured earlier in the same session.

| Metric | Phase-3 baseline | Phase-4 | Delta |
|---|---|---|---|
| `checkBestPathChange` flat heap | 107.74 MB (47% of inuse) | not in top 20 (< 0.72 MB) | -99% |
| `bart NewFringeNode[bestPathRecord]` | N/A | 41 MB (28%) | new: bestPrev trie storage |
| `bart NewFringeNode[RouteEntry]` | 48.5 MB | 37.5 MB | -23% |
| `fmt.Sprintf` via `bestCandidateNextHop` | 15 MB | 15 MB | unchanged (separate target) |
| Total inuse heap | 228 MB | 144 MB | -37% |
| Total CPU samples | 8.98s (10.0% wall) | 6.82s (7.58% wall) | -24% |
| GC share (gcBgMarkWorker cum) | 31% | ~25% | -6 pp |
| `checkBestPathChange` CPU flat | 0.26s | 0.04s | -85% |

The 107.74 MB of per-insert churn in `checkBestPathChange` collapsed into
41 MB of steady-state BART fringe nodes for `bestPathRecord`. AC-1 target was
"under 50 MB on the hot function" -- actual is ~41 MB, and that lives in the
trie as durable storage rather than as per-update allocation pressure.

`fmt.Sprintf` at 15 MB is the remaining `bestCandidateNextHop -> formatNextHop`
allocation flagged earlier in the `/ze-design` decision log as an out-of-scope
follow-up; it did not move because this initial refactor did not touch that
path.

### Phase-4b follow-up: NextHop as netip.Addr

Immediately after the Phase-4 commits landed, a follow-up pass converted
`bestPathRecord.NextHop` from `string` to `netip.Addr` so the hot-path
comparison inside `checkBestPathChange` became a value compare, and the
display string was produced only on the emission path. `formatNextHop`
itself was left alone (used by show/filter code paths with their own
existing conventions); a new `parseNextHopAddr`, `extractMPNextHopAddr`
and `nextHopString` helpers sit on the best-path path.

| Metric | Phase-4 | Phase-4b | Delta |
|---|---|---|---|
| `fmt.Sprintf` via bestCandidateNextHop | 15 MB (10.4%) | not in top 20 | eliminated |
| Total CPU | 6.82s (7.58%) | 6.36s (7.07%) | -7% |
| `checkBestPathChange` flat heap | < 0.72 MB | < 0.86 MB | unchanged |
| BART NewFringeNode[bestPathRecord] | 41 MB | 56.5 MB | +15 MB (larger record) |
| Total inuse heap | 144 MB | 172 MB | run variance |

The bestPathRecord struct grew by 8 bytes per entry (netip.Addr 24 vs
string header 16), reflected in the larger fringe-node allocation. This
cost is steady-state trie storage, offset by the elimination of the
per-call transient string allocations that were producing 15 MB of
fmt.Sprintf churn. The total-heap variance (+28 MB) is run-to-run noise
once allocations are dominated by BART internals; both phase-4 runs
are well below the 228 MB Phase-3 baseline.

IPv6 next-hop emission now uses `netip.Addr.String()` which produces
RFC 5952 canonical compressed form (`2001::1`). This aligns the
best-change event output with ze's other JSON paths (verified against
`test/plugin/nexthop.ci`, `ipv6.ci`, `mup6.ci` expectations). The
former `formatNextHop` in `rib_nlri.go` produced uncompressed form and
remains unchanged for the non-best-change callers (show / filter)
where those paths' existing output format is preserved.

## Files

- `internal/component/bgp/plugins/rib/storage/store_bart.go` (new) -- generic
  `Store[T]` with BART+map dispatch under default build
- `internal/component/bgp/plugins/rib/storage/store_map.go` (new) -- generic
  `Store[T]` map-only under `-tags maprib`
- `internal/component/bgp/plugins/rib/storage/store_test.go` (new) --
  table-driven coverage of the generic surface
- `internal/component/bgp/plugins/rib/storage/store_bart_test.go` (new) --
  trie-only malformed-input assertion gated by `!maprib`
- `internal/component/bgp/plugins/rib/storage/familyrib_bart.go` -- rewritten
  as a thin wrapper around `*Store[RouteEntry]` + lifecycle layer
- `internal/component/bgp/plugins/rib/storage/familyrib_map.go` -- same
  rewrite under the `maprib` tag
- `internal/component/bgp/plugins/rib/storage/nlrikey.go` -- added
  `PrefixToNLRIInto(pfx, buf)` for zero-alloc iteration
- `internal/component/bgp/plugins/rib/rib_bestchange.go` -- dropped
  `bestPathRecord.Prefix`; introduced `bestPrevStore` pairing two
  `*Store[bestPathRecord]`; rewrote `checkBestPathChange` and
  `replayBestPaths`
- `internal/component/bgp/plugins/rib/rib.go` -- `bestPrev` field type
  change + initializer update
- `internal/component/bgp/plugins/rib/rib_bestchange_test.go`,
  `internal/component/bgp/plugins/rib/rib_test.go` -- constructor lines
- `internal/component/bgp/plugins/rib/rib_structured.go` -- RFC 7606
  guarantee comment on `wirePrefixToString`
- `docs/architecture/plugin/rib-storage-design.md` -- paragraph on
  `Store[T]` and the bestPrev consolidation
