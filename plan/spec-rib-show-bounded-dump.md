# Spec: bounded `show bgp rib` and `show bgp rib best` route dump

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-pipe-first-last (for `\| first N` row limiting) |
| Phase | 1/6 |
| Updated | 2026-05-30 |

> Origin: derived from `../bird3-optimisations-report.md` items #14 (CLI memory pressure:
> page buffers, per-item allocation bounding) and #13 (avoid pathological export-side work).
> The BIRD principle: any long iterator / dump command should bound temporary allocation
> lifetime per item and not materialise the whole table before emitting.
>
> Row limiting (`| first N` / `| last N`) is handled by `spec-pipe-first-last.md` as
> generic pipe operators with server-side promotion. This spec focuses on the three
> RIB-internal improvements that reduce the cost of any dump regardless of whether a
> limit is applied: lazy Adj-RIB-Out source, pooled formatting, and lock-scope reduction.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/rules/pipe-completeness.md` - every output command supports all pipe operators
4. `internal/component/bgp/plugins/rib/rib_pipeline.go` (showPipeline, sources, jsonTerminal)
5. `internal/component/bgp/plugins/rib/rib_attr_format.go` (per-route attribute formatting)
6. `internal/component/bgp/plugins/rib/rib_pipeline_best.go` (best-path pipeline, lock scope)
7. `internal/core/textbuf/textbuf.go` (pooled zero-alloc builder, already available)

## Task

`show bgp rib` materialises the entire RIB before emitting and allocates per route and
per attribute, with the RIB read lock held for the whole encode. For a route-server-scale
table (hundreds of thousands of routes per peer) this means: the full Adj-RIB-Out is
reconstructed into `[]RouteItem` up front, every row is turned into a fresh `map[string]any`
with freshly-allocated AS_PATH / community / extended-community strings, the whole result
is accumulated into a nested map, and only then is it passed to a single `json.Marshal`,
all while `peerMu.RLock` is held, which blocks UPDATE processing that needs `peerMu.Lock`.

Goal: reduce the cost of the dump by (a) making the Adj-RIB-Out source lazy (per-peer
buffering) instead of materialising the whole table at construction, (b) reusing a pooled
`textbuf.Buffer` for per-route string formatting where applicable (community strings,
extended community hex), and (c) reducing the `peerMu.RLock` hold time so a large dump
does not stall the UPDATE path. The lock-scope reduction applies to both `showPipeline`
and `bestPipeline`, which share the same `peerMu.RLock(); defer peerMu.RUnlock()` pattern
across the full build + drain. The JSON output shape and all pipe operators are preserved
exactly. No default row limit is imposed; bounding output count is done via `| first N`
(see `spec-pipe-first-last.md`).

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
- [ ] `ai/rules/pipe-completeness.md` - output commands must support all pipe operators
  -> Constraint: the existing `count`, `prefix-summary`, `path`, `family`, `prefix`, `community`, `match` stages MUST keep working unchanged.
- [ ] `ai/rules/no-sprintf-alloc.md` - append-based, pooled formatting on hot paths
  -> Constraint: per-route string formatting should use `textbuf` (pooled) where it applies (community/ext-community strings), not fresh allocations.
- [ ] `ai/rules/json-format.md` - kebab-case JSON keys matching YANG/config names
  -> Constraint: output keys (`adj-rib-in`, `adj-rib-out`, `next-hop`, `path-id`, ...) MUST stay kebab-case and unchanged.
- [ ] `ai/rules/derive-not-hardcode.md` - return structured data, not pre-formatted strings
  -> Constraint: terminals own formatting; sources/filters pass structured `RouteItem`s.

**Key insights:**
- The pipeline is already iterator-based (`PipelineIterator.Next`), so streaming is a natural fit; the non-streaming parts are `newOutboundSource` (eager materialisation) and `jsonTerminal.drain` (full accumulation before marshal).
- `textbuf` already provides a pooled, zero-alloc builder (`Get`/`Release`); it is simply not used on this path today. However, the hot allocations are typed slices (`[]uint32` for AS path, `[]string` for communities) and `map[string]any` for `attrWithFlags`, which `textbuf` cannot replace. The textbuf win is limited to community `String()` and extended community `Hex()` formatting.

## Current Behavior (MANDATORY)

**Source files read:** (read BEFORE writing this spec)
- [ ] `internal/component/bgp/plugins/rib/rib_pipeline.go` - `showPipeline` (983) takes `peerMu.RLock` and holds it (defer Unlock) for the ENTIRE pipeline build + drain. Sources: `inboundSource` (73) buffers one peer's family at a time via `IterateSorted` (lazy-ish); `outboundSource`/`newOutboundSource` (196/204) walks ALL peers/families/keys and calls `reconstructRoute` per route into `items []RouteItem` at construction (full materialisation); `combinedSource` (242) chains both. `jsonTerminal.drain` (877) pulls every item, calls `serializeRouteItem` (947) per route, accumulates into `map[string]*peerRoutes`, then `json.Marshal`s the whole structure once.
  -> Constraint: `peerMu.RLock` held across the full encode blocks writers (`peerMu.Lock` in `handleSent`/`handleReceived`/`handleState`) for the dump's whole duration.
  -> Constraint: `outboundSource` materialises before `Next()` is ever called; memory peaks at full-table size regardless of any downstream `count`/first.
- [ ] `internal/component/bgp/plugins/rib/rib_pipeline_best.go` - `bestPipeline` (156) takes `peerMu.RLock` and holds it (defer Unlock) for the ENTIRE best-path pipeline build + drain. `newBestSource` (41) walks ALL peers/families under the lock, gathers candidates, selects best per prefix, and builds full `items []RouteItem` at construction. `bestJSONTerminal.drain` (413) accumulates all items before `json.Marshal`. Same lock-starvation pattern as `showPipeline`.
  -> Constraint: `bestSource` needs cross-peer data for best-path selection, so lazy-source pattern does not apply here. Lock-scope reduction does.
  -> Constraint: `bestReasonTerminal` needs `candidatesByKey` populated by `newBestSource` under the lock; snapshot must include stashed candidates.
- [ ] `internal/component/bgp/plugins/rib/rib_attr_format.go` - `enrichRouteMapFromEntry` (19) / `enrichRouteMapFromRoute` (69) decode from pools and allocate per attribute: `formatASPath` (140) builds a `[]uint32`, `formatCommunities` (176) builds `[]string` via `c.String()` per community, `textbuf.Hex` per extended community (101), `attrWithFlags` (108) allocates a `map[string]any` per attribute.
  -> Constraint: output field names and value shapes here define the preserved JSON contract.
  -> Decision: textbuf reuse scoped to community/ext-community string formatting only; the `[]uint32`, `[]string`, and `map[string]any` allocations are typed data structures that a byte buffer cannot replace.
- [ ] `internal/core/textbuf/textbuf.go` - pooled `Buffer` (`Get`/`Release`, 128-byte inline) exists and is unused on the show path.
  -> Decision: reuse it for community string formatting; do not invent a new buffer pool.

**Behavior to preserve:**
- JSON output shape: top-level `adj-rib-in` / `adj-rib-out` maps keyed by peer; per-route maps with `family`, `prefix`, `next-hop`, `path-id`, AS_PATH, communities, etc. (all kebab-case keys, same value encodings).
- All pipe operators and terminals: `count`, `prefix-summary`, `path`, `family`, `prefix`, `community`, `match`, `graph`, default JSON.
- Scope semantics: `advertised` / `received` / (combined).
- Filter correctness (e.g. `match`, `community` matching against pool data).

**Behavior to change:**
- Make Adj-RIB-Out source lazy (per-peer buffering like `inboundSource`).
- Reuse pooled `textbuf.Buffer` for community/ext-community string formatting.
- Reduce `peerMu.RLock` hold time in `showPipeline` by snapshotting per-peer state under the lock and performing decode/format outside the lock.
- Reduce `peerMu.RLock` hold time in `bestPipeline` using the same snapshot pattern. `newBestSource` builds candidates and selects winners under the lock; decode/format in terminals runs outside.

## Data Flow (MANDATORY)

### Entry Point
- CLI / RPC: `show bgp rib [| scope] [| filters] [| terminal]` -> `FoldFilters` promotes command-specific filters to server args -> handler in `rib_commands.go` -> `showPipeline(selector, args)` in `rib_pipeline.go:901`.

### Transformation Path
1. `parsePipelineArgs` -> scope + filter/terminal stages (unchanged).
2. Source: `inboundSource` / `outboundSource` / `combinedSource` produce `RouteItem`s. **Changed:** Adj-RIB-Out source becomes lazy (per-peer buffering) like the inbound source.
3. Filter stages (`path`/`family`/`prefix`/`community`/`match`) wrap the source iterator (unchanged).
4. Terminal (`json` default, or `count`/`prefix-summary`/`graph`) drains (unchanged, but per-route formatting uses pooled textbuf where applicable).
5. **Changed:** `peerMu.RLock` held only long enough to snapshot per-peer keys/handles; heavy decode/format runs outside the lock.
6. `bestPipeline` follows the same pattern: `newBestSource` gathers candidates and selects winners under the lock; `bestJSONTerminal`/`bestReasonTerminal` drain outside.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI/RPC -> plugin | command text -> `showPipeline` | [ ] |
| pool <-> formatting | `pool.*.Get` by handle, `reconstructRoute` | [ ] |
| plugin -> CLI | JSON string (pipe-processed) | [ ] |

### Integration Points
- `textbuf.Get/Release` - per-route formatting buffer reuse for community strings.
- `outboundSource` - restructured to lazy per-peer iteration.
- `showPipeline` - lock scope reduced.
- `bestPipeline` - lock scope reduced (same pattern).

### Architectural Verification
- [ ] No bypassed layers (still goes through the pipeline iterator model)
- [ ] No unintended coupling (sources/filters stay structured; terminals own formatting)
- [ ] No duplicated functionality (reuse `textbuf`, reuse the lazy per-peer pattern from `inboundSource`)
- [ ] Zero-copy preserved where applicable (pool reads by handle; no needless `Route` clones)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `show bgp rib \| advertised` on a multi-peer table | -> | `showPipeline` + lazy `outboundSource` | `TestShowOutboundSourceLazy` -- no full materialisation at construction |
| `show bgp rib` with concurrent UPDATE processing | -> | reduced `peerMu.RLock` scope in `showPipeline` | `TestShowPipesUnchanged` -- results unchanged with reduced lock |
| `show bgp rib best` with concurrent UPDATE processing | -> | reduced `peerMu.RLock` scope in `bestPipeline` | `TestBestPipelineLockReduced` -- results unchanged with reduced lock |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `show bgp rib \| advertised` on a large table | Peak memory bounded to ~one peer's family worth of `RouteItem`s, not the whole table (source is lazy) |
| AC-2 | Any dump | Per-route community/ext-community string formatting reuses a pooled `textbuf.Buffer`; allocs/route drop vs baseline for those paths (benchmark) |
| AC-3 | All existing pipes (`count`, `prefix-summary`, `path`, `family`, `prefix`, `community`, `match`, `graph`) | Continue to produce identical results to baseline |
| AC-4 | Large dump concurrent with active UPDATE processing | `peerMu.RLock` is not held for the full decode/format in `showPipeline`; UPDATE writers are not starved for the dump's duration |
| AC-5 | JSON output for any dump | Byte-equivalent to current behavior (no contract change) |
| AC-6 | Snapshot semantics documented | Per-peer snapshot consistency; mid-dump mutations visible at per-peer granularity (not a regression, no whole-table atomicity exists today) |
| AC-7 | `show bgp rib best` concurrent with active UPDATE processing | `peerMu.RLock` in `bestPipeline` is not held for the full drain; candidate gathering and best-path selection run under the lock, terminal formatting runs outside |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestShowOutboundSourceLazy` | `internal/component/bgp/plugins/rib/rib_pipeline_test.go` | AC-1 no full materialisation at construction | |
| `TestShowJSONContractUnchanged` | `internal/component/bgp/plugins/rib/rib_pipeline_test.go` | AC-5 byte-equivalent output | |
| `TestShowPipesUnchanged` | `internal/component/bgp/plugins/rib/rib_pipeline_test.go` | AC-3 all stages equal baseline | |
| `TestBestPipelineLockReduced` | `internal/component/bgp/plugins/rib/rib_pipeline_best_test.go` | AC-7 best pipeline results unchanged with reduced lock | |

### Boundary Tests (MANDATORY for numeric inputs)
<!-- No new numeric inputs in this spec; row limiting is in spec-pipe-first-last. -->
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-rib-show-pipes` | `test/plugin/*.ci` | Each pipe operator still returns expected rows after lazy source + lock reduction | |

### Interop Tests (MANDATORY for protocol features)
<!-- N/A: this is a CLI/observability command, not wire-protocol behavior. No peer daemon involved. -->
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A -- CLI dump, not wire protocol | - | - | - | - |

### Performance Tests
| Test | Location | Validates | Status |
|------|----------|-----------|--------|
| `BenchmarkShowLargeTable` | `internal/component/bgp/plugins/rib/rib_pipeline_test.go` | AC-1/AC-2 allocs/op and peak bytes drop | |

### Future (if deferring any tests)
- None planned.

## Files to Modify
- `internal/component/bgp/plugins/rib/rib_pipeline.go` - lazy `outboundSource` (per-peer buffering like `inboundSource`); reduce `peerMu.RLock` scope in `showPipeline`.
- `internal/component/bgp/plugins/rib/rib_pipeline_best.go` - reduce `peerMu.RLock` scope in `bestPipeline` (same snapshot pattern as `showPipeline`).
- `internal/component/bgp/plugins/rib/rib_attr_format.go` - community/ext-community string formatting reuses a pooled `textbuf.Buffer`.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | [ ] No | - |
| CLI commands/flags | [ ] No | no new commands or flags (row limiting is `\| first N` from spec-pipe-first-last) |
| CLI grammar (action before identifier) | [ ] N/A | no grammar changes |
| Editor autocomplete | [ ] No | - |
| Functional test for new RPC/API | [ ] Yes | `test/plugin/test-rib-show-pipes.ci` |
| Pipe completeness | [ ] Yes | preserve all existing operators |
| Doctor check for runtime dependencies | [ ] No | - |
| Prometheus counters/metrics | [ ] No | - |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 3 | CLI command added/changed? | [ ] No | no user-facing command change |
| 12 | Internal architecture changed? | [ ] Yes | rib show pipeline doc: lazy source, lock reduction |
| 16 | Changed source referenced by doc anchors? | [ ] Check | grep `docs/` for `rib_pipeline.go`/`rib_attr_format.go` anchors |

## Files to Create
- `test/plugin/test-rib-show-pipes.ci`

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify; confirm JSON contract baseline |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Phases below |
| 5. /ze-review gate | Review Gate section |

### Implementation Phases
1. **Phase: Wiring (MANDATORY FIRST)** -- capture baseline JSON output for all pipe operators; `TestShowJSONContractUnchanged` + `TestShowPipesUnchanged` pass against current code.
   - Files: test files
   - Verify: green baseline.
2. **Phase: Lazy outbound source** -- convert `outboundSource` to per-peer lazy buffering (mirror `inboundSource`).
   - Tests: `TestShowOutboundSourceLazy`, re-run `TestShowJSONContractUnchanged`
   - Verify: no construction-time full walk; output unchanged.
3. **Phase: Pooled formatting** -- route community/ext-community string formatting through a reused `textbuf.Buffer`.
   - Tests: `BenchmarkShowLargeTable`
   - Verify: allocs/route drop for community formatting paths.
4. **Phase: Lock-scope reduction (`showPipeline`)** -- snapshot per-peer keys/handles under `peerMu.RLock`, perform decode/format outside the lock.
   - Tests: `TestShowPipesUnchanged` (re-verify results unchanged)
   - Verify: writers not starved; results unchanged.
   - Design note: lock-reduction mechanism differs by direction. For outbound, `reconstructRoute` reads from `ribOut` and `ribOutSourcePeer` (both need `peerMu`), so the snapshot must capture enough state to decode without the lock. For inbound, pool reads (`pool.ASPath.Get`, etc.) happen in the terminal via `serializeRouteItem`, not in the source, so they are naturally outside the lock if the source captures `RouteEntry` values under the lock.
5. **Phase: Lock-scope reduction (`bestPipeline`)** -- same snapshot pattern applied to `bestPipeline`. `newBestSource` gathers candidates and selects winners under the lock (it needs cross-peer data), then the lock is released before terminal drain.
   - Tests: `TestBestPipelineLockReduced`
   - Verify: `show bgp rib best` output unchanged; terminal formatting runs outside the lock.
   - Design note: `bestSource` cannot be made lazy (needs all candidates across peers for best-path selection). The lock boundary moves to after `newBestSource` returns and filter stages are applied, but before terminal drain. `bestReasonTerminal` reads from `candidatesByKey` (populated by `newBestSource` under the lock), so the stash is part of the snapshot.
6. **Functional** -- `test-rib-show-pipes.ci`.
7. **Full verification** -- `make ze-verify`.
8. **Complete spec** -- audit, learned summary, two-commit closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Each AC has code + test at file:line |
| Correctness | Output byte-equivalent; lazy source yields same items in same order |
| Naming | JSON keys stay kebab-case; no contract drift |
| Data flow | Sources/filters structured; terminals format; pool reads by handle |
| Lock correctness | Snapshot semantics documented; no torn reads within a peer; applies to both `showPipeline` and `bestPipeline` |
| Rule: pipe-completeness | every operator still supported |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Lazy outbound | `go test -run TestShowOutboundSourceLazy` |
| JSON contract intact | `go test -run TestShowJSONContractUnchanged` |
| All pipes unchanged | `go test -run TestShowPipesUnchanged` |
| Perf delta | `go test -bench BenchmarkShowLargeTable` before/after numbers in spec |
| Best pipeline lock reduced | `go test -run TestBestPipelineLockReduced` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Lock starvation | Reduced `peerMu.RLock` scope in both `showPipeline` and `bestPipeline` must not introduce inconsistent reads within a peer (document snapshot semantics). For `bestPipeline`, `candidatesByKey` stash must be fully populated before lock release. |
| Resource exhaustion | Without `\| first N`, a full-table dump still materialises all routes through the terminal; lazy source bounds peak source-side memory but terminal accumulation is unchanged. Document this as the expected trade-off. |

### Failure Routing
| Failure | Route To |
|---------|----------|
| JSON contract drifts | Re-read `rib_attr_format.go`; fix terminal formatting |
| Pipe operator regresses | Re-check filter wrapping order in `showPipeline` |
| Reduced lock scope yields torn reads (`showPipeline` or `bestPipeline`) | DESIGN: snapshot handles under lock, decode after; or revert to held-lock if snapshot semantics are insufficient |
| 3 fix attempts fail | STOP. Report. Ask user. |

## Checklist

### Goal Gates
- [ ] AC-1..AC-7 each implemented with code + test at file:line
- [ ] Tests written (unit `rib_pipeline_test.go`, functional `test-rib-show-pipes.ci`, `BenchmarkShowLargeTable`)
- [ ] Tests PASS (baseline tests green before and after; benchmark shows improvement)
- [ ] JSON contract byte-equivalent (AC-5)
- [ ] All pipe operators preserved (AC-3)

### Quality Gates
- [ ] `make ze-test` green
- [ ] `make ze-lint-changed` clean
- [ ] `make ze-verify` green
- [ ] `/ze-review` Review Gate: 0 BLOCKER, 0 ISSUE
- [ ] Benchmark deltas (allocs/op, peak bytes) recorded in Implementation Summary

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| The kernel-FIB dump (`route_linux.go`) was the BIRD #14 target | That path is bounded by kernel table size and already has a `limit`; the large-table hot path is `show bgp rib` (Adj-RIB-Out/In) | tracing `showPipeline` + `newOutboundSource` materialisation | Spec retargeted to the RIB show pipeline |
| Row limiting belongs in this spec | Row limiting is a generic pipe operator (`\| first N` / `\| last N`) that applies to all commands, not just RIB show | review feedback from user | Extracted to `spec-pipe-first-last.md`; this spec depends on it |
| AC-4 (textbuf reuse) would significantly reduce allocs | The hot allocations are `[]uint32`, `[]string`, and `map[string]any` (typed data structures); textbuf only helps with community/ext-community `String()` formatting | reading `rib_attr_format.go` | AC scoped to community/ext-community strings only |
| Lock-scope reduction only applies to `showPipeline` | `bestPipeline` has the identical `RLock(); defer RUnlock()` across full build + drain pattern | comparing `rib_pipeline.go:984` with `rib_pipeline_best.go:157` | Added AC-7 for `bestPipeline` lock reduction |
| Lazy source could generalize to `bestSource` | `bestSource` needs cross-peer aggregation (all candidates for a prefix) before selecting a winner; per-peer laziness doesn't apply | reading `newBestSource` cross-peer walk | Documented as design decision: `bestSource` stays eager |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Design Insights
- The pipeline is already iterator-based, so the fix is mostly "stop pre-materialising and accumulating," not a rewrite. The lock-hold-during-encode issue is the subtler, higher-value part (UPDATE latency under a big dump).
- Lock-reduction mechanism differs by direction: outbound needs to snapshot route data (since `reconstructRoute` reads `ribOut` maps under the lock), while inbound naturally separates source iteration (under lock) from pool reads (in the terminal, can run without lock).

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Lazy Adj-RIB-Out source (per-peer buffering) | Keep eager materialisation | Eager walk peaks at full-table memory regardless of downstream filtering |
| Reduce `peerMu.RLock` scope via per-peer key snapshot (`showPipeline` + `bestPipeline`) | Hold RLock for whole encode (simplest) | Holding the read lock across a 100k-route encode starves UPDATE writers; snapshot keeps reads consistent per peer while freeing the lock between peers. Both pipelines share the same pattern. |
| `bestSource` stays eager (not lazy) | Make best-path source lazy per-peer | Best-path selection needs all candidates across all peers for a given prefix before selecting a winner; cross-peer aggregation is fundamental and cannot be lazified per-peer |
| Reuse `textbuf` pool for community strings only | New per-route buffer pool / reuse for all formatting | `textbuf` already exists; hot allocs are typed slices/maps that a byte buffer cannot replace |
| No default row limit | Default limit of N | Row limiting is `\| first N` (user-explicit); no silent truncation that changes behavior |

## Known Limitations
- Reduced lock scope means a very large dump may reflect routes added/removed mid-dump at per-peer granularity (snapshot-per-peer, not whole-table atomic). This matches typical `show` semantics and is documented, not a regression of any guaranteed atomicity (none exists today either, since the lock is read-only and concurrent reads already interleave with prior writes).
- Terminal accumulation (`jsonTerminal.drain` building the full `map[string]*peerRoutes` before `json.Marshal`) is unchanged by this spec. Streaming marshal is a separate optimization. The lazy source bounds source-side peak memory; the terminal still accumulates.

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | | file:line | |

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
