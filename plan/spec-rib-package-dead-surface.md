# Spec: the rest of the dead surface in the bgp rib package

| Field | Value |
|-------|-------|
| Status | done |
| Scope | plugin |
| Depends | - |
| Phase | 1/1 |
| Handoff | - |
| Updated | 2026-09-07 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`RouteStore` and the `internal/component/bgp/store` package were removed because <!-- doc-links: ignore (this sentence exists to name what was DELETED, in 1ba6055ca; the path cannot resolve and repointing it at live code would destroy the sentence's meaning) -->
the Loc-RIB moved out of the engine into the `bgp-rib` plugin and pool handles
replaced the mutex-based typed stores. That deletion took the smallest coherent
unit. The same package still carries a larger dead surface.

Six non-test files outside the package import it, and between them they reach
five symbols: `Route`, `NewRouteWithASPath`, `RouteJSON`, `NewCommitService` and
`CommitOptions`. Everything else in the package is reached only from its own
tests.

The cost is not only the dead code. `NewRouteWithASPath`, the one live
constructor, calls `refCount.Store(1)` on every route it builds, and the three
methods that read that counter are dead. Three more fields, `wireBytes`,
`nlriWireBytes` and `sourceCtxID`, are written by nothing at all now that the
wire-cache constructors are dead. `Route` measures 160 bytes and carries 54
bytes of field that no live code reads.

Goal: delete the dead surface and the fields it existed to serve, so `Route`
costs what it uses.

## Required Reading

### Architecture Docs
- [ ] `plan/learned/DESIGN-HISTORY.md` - records that best-path lives in the bgp-rib plugin and the engine has no Loc-RIB
  → Decision: the engine holds no RIB by design, so an engine-side Adj-RIB-In or Adj-RIB-Out has no future consumer to wait for
  → Constraint: per-attribute mutex stores were replaced by pool handles; a replacement mechanism already exists and is in production
- [ ] `docs/architecture/pool-architecture.md` - carries the per-route memory model and its projections
  → Decision: it already states "Engine OutgoingRIB is excluded: it has no production callers (test-only)", and a previous projection was corrected for counting it at 478 B/route/peer
  → Constraint: the byte figures on that page are derived from struct sizes, so shrinking `Route` makes them wrong in the same commit that shrinks it
- [ ] `docs/architecture/update-building.md` - documents the UPDATE build path
  → Constraint: it carries a source anchor naming `CanForwardDirect`, so deleting that method reddens `./le docs-to-code index-check` unless the anchor and its prose move in the same commit

**Key insights:** (minimal context to resume after compaction)
- Production reaches exactly five symbols from this package; everything else is test-only
- `OutgoingRIB` was unwired 2026-01-03, `IncomingRIB` 2026-03-01, in different commits
- Named commits are implemented by `CommitManager` in `internal/component/bgp/transaction/`, and `SendRoutes` documents that it bypasses the `OutgoingRIB` transaction
- `Route` is 160 bytes measured; the surviving five fields total 96

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/bgp/rib/route.go` - `Route` and its constructors, accessors, wire-cache cluster and refcount trio
- [ ] `internal/component/bgp/rib/incoming.go` - Adj-RIB-In, no production caller
- [ ] `internal/component/bgp/rib/outgoing.go` - Adj-RIB-Out and its transaction surface, no production caller
- [ ] `internal/component/bgp/rib/commit.go` - `CommitService`, production-reached, stays
- [ ] `internal/component/bgp/reactor/reactor_api_batch.go` - `SendRoutes` builds a `CommitService` directly and documents bypassing the `OutgoingRIB` transaction
- [ ] `internal/component/bgp/reactor/reactor_api.go` - `RIBInRoutes` returns nil unconditionally

**Behavior to preserve:**
- Every symbol production reaches: `Route` with `NLRI`, `NextHop`, `Attributes`, `ASPath`, `Index`; `NewRouteWithASPath`; `RouteJSON` and its `MarshalJSON`; `NewCommitService` and `CommitOptions`
- `GroupByAttributesTwoLevel`, reached from `commit.go`

**Behavior to change:**
- None that production can observe. Every symbol removed has no non-test caller.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- No new entry point. The affected path is route construction: a plugin or the API adapter builds a `Route` through `NewRouteWithASPath`.

### Transformation Path
1. `SendRoutes` or `peer_rib_routes.go` builds a `Route` from NLRI, next hop, attributes and AS-PATH
2. `NewRouteWithASPath` allocates it and today also stores 1 into a reference counter nothing reads
3. `Index` derives the route key lazily into `indexCache`
4. `CommitService` groups routes and hands them to the UPDATE build path

### Boundaries Crossed
| Boundary | From | To |
|----------|------|-----|
| None crossed | the change is internal to one package | its own callers, whose call sites do not change |
| Documentation | `Route` struct size | the byte model in `pool-architecture.md` |

### Integration Points
- `internal/component/bgp/reactor` builds and reads `Route`
- `internal/component/bgp/transaction/commit_manager.go` holds `*rib.Route`
- `internal/component/bgp/types/reactor.go` names `rib.Route` and `rib.RouteJSON` in the reactor API interface

## Risks & Assumptions

### Assumptions
| # | Assumption | Basis | If wrong | Validation |
|---|-----------|-------|----------|------------|
| A-1 | No consumer outside this module reaches these symbols | the package is under `internal/`, so the Go toolchain forbids it | nothing, the language prevents it | the import path itself |
| A-2 | `RouteJSON` must stay although `RIBInRoutes` returns nil | it is named in the `bgptypes.ReactorAPI` interface, so removing it changes an API surface | scope widens past the rib package | read the interface in `internal/component/bgp/types/reactor.go` |
| A-3 | Pruning `sizeof_test.go` weakens no assertion | the file contains zero `t.Error`, `t.Fatal` or `t.Fail` calls; it reports through `t.Logf` | the change would be a test weakening needing a different justification | grep the file for assertion calls |
| A-4 | The four fields are unread by surviving code | `commit.go`, `grouping.go` and `peer_rib_routes.go` name none of them | the deletion breaks the build immediately | the compiler |

### Risks
| # | Risk | Early signal | Mitigation |
|---|------|--------------|------------|
| R-1 | Deleting `CanForwardDirect` reddens `./le docs-to-code index-check` | the gate names `docs/architecture/update-building.md` | move the anchor and its prose in the same commit |
| R-2 | The per-route byte figures in `pool-architecture.md` become wrong the moment `Route` shrinks | the doc disagrees with the sizeof reporter | re-run the reporter and update the figures in the same commit |
| R-3 | Removing the reference counter removes a mechanism a future RIB rewire might want | none at build time | `attrpool.Handle` already refcounts and is what production uses; a future design would build on that, not on this |
| R-4 | The commit needs a large `test/weakened.md` population and that file is shared and currently accumulating | `commit_helper.py` refuses rows it cannot pair | write the rows against this commit's own removals only, and check the file is not carrying another session's rows first |
| R-5 | Another session edits the rib package concurrently in the shared checkout | a conflict at commit time | commit promptly once green |

## Acceptance Criteria

| # | Assertion | How it is proven |
|---|-----------|------------------|
| AC-1 | `internal/component/bgp/rib/incoming.go` and `outgoing.go` do not exist, and no symbol they declared is referenced anywhere in the tree | the build, plus a word-boundary search for `IncomingRIB` and `OutgoingRIB` returning only history |
| AC-2 | `Route` declares exactly five fields: `nlri`, `nextHop`, `attributes`, `asPath`, `indexCache` | reading the struct, and `TestStructSizes` reporting the five component sizes |
| AC-3 | `Route` measures 96 bytes, down from the 160 bytes measured on 2026-08-18 | `TestStructSizes` output recorded before and after |
| AC-4 | `NewRouteWithASPath` performs no reference-count write | reading the constructor; no `refCount` identifier survives in the package |
| AC-5 | All four flavors of the committed tree compile | `./le repository tracked-build check` |
| AC-6 | No documentation cites a deleted symbol or a stale byte figure | `./le docs-to-code index-check` and `./le doc check links` both green, and the `pool-architecture.md` figures equal the AC-3 measurement |
| AC-7 | Every production symbol the package exported before the change is still reachable | `NewRouteWithASPath`, `Route` accessors, `RouteJSON`, `NewCommitService` and `CommitOptions` still compile from their six importing files |

## Wiring Test

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `SendRoutes` building a route for a named commit | → | `NewRouteWithASPath` | `TestStructSizes` records the post-change size, and the tracked-build check proves every flavor still compiles |
| A plugin reading routes through the reactor API | → | `RouteJSON` | existing reactor API tests stay green |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates |
|------|------|-----------|
| `TestStructSizes` | `internal/component/bgp/rib/sizeof_test.go` | `Route` measures 96 bytes after the change, down from 160 |
| `TestHeapBytesPerRoute` | `internal/component/bgp/rib/sizeof_test.go` | the per-route heap figure quoted by `pool-architecture.md` |
| existing `route_test.go` cases | `internal/component/bgp/rib/route_test.go` | the surviving accessors and `Index` behave unchanged |

### Functional Tests

No user-facing behavior changes: every symbol removed has no production caller,
so nothing the daemon does can differ. The evidence that the removal is safe is
`./le repository tracked-build check` green across all four flavors and the
existing suites unchanged, not a new test.

## Files to Modify

- `internal/component/bgp/rib/route.go` - remove three constructors, the wire-cache cluster, the refcount trio, `Route.JSON`, both iterators, four struct fields and the `refCount.Store(1)` in `NewRouteWithASPath`
- `internal/component/bgp/rib/incoming.go` - delete
- `internal/component/bgp/rib/outgoing.go` - delete
- `internal/component/bgp/rib/rib_test.go` - delete, every test in it covers the two deleted types
- `internal/component/bgp/rib/outgoing_test.go` - delete
- `internal/component/bgp/rib/transaction_test.go` - delete
- `internal/component/bgp/rib/sizeof_test.go` - drop the field lines and the `OutgoingRIB` case, keep the `Route` reporter
- `internal/component/bgp/rib/route_test.go` - prune cases covering deleted methods
- `docs/architecture/update-building.md` - move the source anchor off `CanForwardDirect` and correct the prose
- `docs/architecture/pool-architecture.md` - update the per-route byte figures to the measured size
- `docs/architecture/rib-transition.md` - correct the diagram that shows the engine holding these types
- `test/weakened.md` - one row per removed test

## Implementation Steps

1. Record the current measurement, `Route` at 160 bytes, before touching anything
2. Delete `incoming.go`, `outgoing.go` and their three test files
3. Remove the dead surface from `route.go`, then the four fields, then the `refCount.Store(1)`
4. Prune `sizeof_test.go` and `route_test.go` to the surviving surface
5. Re-run the sizeof reporter and record the new size
6. Update the three architecture docs, anchors first
7. Run `./le changed scope`, `go test -race ./...` for the package, `./le docs-to-code index-check` and `./le doc check links`
8. Write `test/weakened.md` rows against this commit's removals only
9. Commit, then `./le repository tracked-build check`

## Checklist

- [ ] Tests written
- [ ] Tests FAIL before implementation
- [ ] Tests PASS after implementation
- [ ] `./le verify worktree` green

### Integration Checklist
- [ ] YANG schema and validation - N/A, no config surface changes
- [ ] CLI grammar and completion - N/A, no command changes
- [ ] Functional test - N/A, no observable behavior changes; the tracked-build check is the evidence
- [ ] Env var - N/A, none added
- [ ] Doctor check and diagnostic code - N/A, no runtime dependency added
- [ ] Prometheus counters - N/A, none added or removed
- [ ] BGP family surface - N/A, no family, capability or attribute changes

### Documentation Update Checklist
- [ ] `docs/features.md` - N/A, no feature added or removed
- [ ] Command, API and plugin docs - N/A, no surface changes
- [ ] `docs/architecture/update-building.md` - Yes, the `CanForwardDirect` source anchor moves
- [ ] `docs/architecture/pool-architecture.md` - Yes, the per-route byte figures change
- [ ] `docs/architecture/rib-transition.md` - Yes, its diagram shows types this spec deletes
- [ ] `docs/architecture/route-types.md` - Yes, it described the wire-byte cluster the engine route no longer holds
- [ ] `docs/architecture/buffer-architecture.md` - Yes, its "RIB Storage" section showed the deleted fields and iterators
- [ ] `docs/architecture/encoding-context.md` - Yes, its "Route Wire Cache" section showed the deleted constructors and `CanForwardDirect`
- [ ] `docs/architecture/api/architecture.md` - Yes, its file table named `internal/component/bgp/rib/outgoing.go`
- [ ] Source anchors elsewhere - checked: `guide/route-injection.md` names `Route` or `commit.go` and survives

## Implementation Summary

### What Was Implemented
- `incoming.go` (162 lines) and `outgoing.go` (562 lines) deleted, with `rib_test.go`, `outgoing_test.go`, `transaction_test.go` and `route_iter_test.go`, the four test files whose every case drove the two deleted types or the deleted iterators.
- `route.go` reduced to `Route` with five fields, one constructor, four accessors, `Index`, and `RouteJSON` with its `MarshalJSON`. The wire-byte cluster (`wireBytes`, `nlriWireBytes`, `sourceCtxID`), the reference counter and the two iterators are gone, and `NewRouteWithASPath` no longer writes a count nothing reads.
- `NewRouteWithASPath(n, nextHop, attrs, nil)` is the single constructor. Seven test files take the rename from `NewRoute`, with no changed expectation.
- `sizeof_test.go` and `route_test.go` pruned to the surviving surface. `TestStructSizes` reports 96 bytes over the five component sizes.
- Seven architecture pages rewritten to describe what the engine holds.

### Bugs Found/Fixed
- The rib commit rail carried `attrSize` and `attrSizeWithContext`, an open-coded copy of `attribute.attrWireLenForValue` that had already diverged: it tested `valueLen > 255` alone, where the owner tests `valueLen > 255 || attr.Flags().IsExtLength()` and `WriteHeaderTo` emits the four-octet header on the FLAG. The rail therefore under-allocated by one octet for any attribute whose sender set that bit over a short value, which RFC 4271 Section 4.3 permits and `NewOpaqueAttribute` preserves. Both local sizers are deleted. `packAttributesWithASPath` now calls `attribute.AttrWireLen`, the newly exported `attribute.AttrWireLenWithContext`, and `attribute.AttributesSizeWithContext`, the last of which also replaces a hand-rolled summing loop. Fixed in `50a4d9591e`, covered by `TestPackAttributesSizesTheExtLengthHeaderItWrites` (`internal/component/bgp/rib/commit_extlength_test.go`).
- The same fix re-homed `attribute.AttributesSizeWithContext`, which the deletion had orphaned: its only non-test caller was the removed `packAttributesWithContext`. Deleting it would have removed the correct implementation while the defective duplicate stayed on the live path.

### Documentation Updates
- `docs/architecture/route-types.md` -- the `rib.Route` row and the two prose paragraphs now say "value handed to `CommitService`" rather than "core engine storage", and the columns drop `wire cache, refcount`. Anchor `<!-- source: internal/component/bgp/rib/route.go -- rib.Route (core engine) -->` retained.
- `docs/architecture/pool-architecture.md` -- the `Engine OutgoingRIB` row is removed from the per-route memory table, and the exclusion paragraph states the 96-byte measurement instead.
- `docs/architecture/buffer-architecture.md` -- the "RIB Storage" section is now "The Engine Route Value" and its Go block shows the five surviving fields. Anchor `<!-- source: internal/component/bgp/rib/route.go -- Route struct -->` retained.
- `docs/architecture/encoding-context.md` -- "Route Wire Cache" is now "Where the Wire Bytes Live", pointing at `WireUpdate` and `forwardUpdateCore`. Three new anchors: `wireu/wire_update.go -- WireUpdate, SourceCtxID`, `rib/route.go -- Route, NewRouteWithASPath`, `reactor/reactor_api_forward.go -- forwardUpdateCore`.
- `docs/architecture/rib-transition.md` -- the diagram box names the `bgp-rib` plugin, "Adj-RIB-In per peer" and "ribOut entries for replay".
- `docs/architecture/api/architecture.md` -- the `internal/component/bgp/rib/outgoing.go` row is removed from the file table.
- `docs/architecture/update-building.md` -- no longer describes the removed wire-byte cluster; the `CanForwardDirect` source anchor moved with the prose (`50a4d9591e`).
- `./le doc check links` and `./le docs-to-code index-check` were run. Both carry reds, and neither red names a file, a symbol, or a page this spec touched. Evidence in Pre-Commit Verification below.

### Deviations from Plan
- The plan named `test/weakened.md`, a shared file. The repository moved to per-session shards while this spec was open, so the rows landed in `test/weakened/8c4ad6c3.md` (78 lines) and the RFC-tag rename rows in `test/rfc-changed/8c4ad6c3.md`. R-4 aimed at the shared-file contention that no longer exists.
- The plan listed three architecture pages. Seven were edited: `api/architecture.md`, `buffer-architecture.md` and `encoding-context.md` also described the removed cluster, and the Documentation Update Checklist above now names all seven.
- The plan expected no code change beyond the deletion. One was needed: the independent review found the sizer divergence the deletion exposed, and `ai/rules/completion.md` puts a defect on the path the work depends on in scope. `50a4d9591e` fixes it.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The spec planned a pure deletion and treated `AttributesSizeWithContext` as surface the deletion would orphan | The orphaned function was the CORRECT owner of a fact the live rail had copied and got wrong. Deleting it would have left the defective duplicate on the path | Review run 1, reading the deletion's blast radius rather than the diff | `50a4d9591e` deletes the duplicate and re-homes the owner. The lesson is recorded in `plan/journal/helper-bypassed-by-an-open-coded-copy.md` |
| approach | The spec was written without a Deliverables Checklist or a Security Review Checklist, the two sections `/ze-close` steps 1 and 2 consume | `plan/TEMPLATE.md` carries both. The Acceptance Criteria table's "How it is proven" column supplied the substance, so closure ran the deliverable verification off that column instead | `/ze-close` step 1, which reads the spec's own section list | Closure proceeded on the AC table and applied the generic security checks from step 2. Reported to the main thread rather than silently absorbed |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Delete the dead surface | Done | `internal/component/bgp/rib/` holds 13 files, none of them `incoming.go` or `outgoing.go` | 2763 deletions against 194 insertions in `fa9faf5d94` |
| Delete the fields the dead surface existed to serve | Done | `route.go:24-32`, five fields | `grep -rn 'refCount\|wireBytes\|nlriWireBytes\|sourceCtxID' internal/component/bgp/rib/` returns nothing |
| `Route` costs what it uses | Done | `TestStructSizes` | 96 bytes over 16+24+24+8+24, no padding |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `ls internal/component/bgp/rib/`; `git grep -w -E 'IncomingRIB\|OutgoingRIB\|CanForwardDirect'` | No `.go` file names any of the three. The surviving hits are this spec, one journal row, and the two weakened shards, all prose about the removal |
| AC-2 | Done | `gopls symbols internal/component/bgp/rib/route.go` | `Route` declares `nlri`, `nextHop`, `attributes`, `asPath`, `indexCache` and nothing else |
| AC-3 | Done | `TestStructSizes` | `Route struct: 96 bytes`, down from the 160 recorded on 2026-08-18 |
| AC-4 | Done | `route.go:37-45` | `NewRouteWithASPath` returns a composite literal of the four input fields. No counter |
| AC-5 | Done | `./le repository tracked-build check` | OK on six flavors at HEAD `50a4d9591e`: distro, test-runner, appliance, setup, host, installer |
| AC-6 | Done | Both gates run; `pool-architecture.md` line 1047 states 96 bytes | Both gates carry reds, and none of them names a file or a symbol this spec touched. See Documentation Verified below |
| AC-7 | Done | Six non-test importers listed, each reaching only the five surviving symbols | The six flavors compile, which is the proof the AC names |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestStructSizes` | Done | `internal/component/bgp/rib/sizeof_test.go` | PASS, reports 96 bytes over five components |
| `TestHeapBytesPerRoute` | Done | `internal/component/bgp/rib/sizeof_test.go` | PASS, 272 bytes per route with typical attributes, 6.0 mallocs |
| existing `route_test.go` cases | Done | `internal/component/bgp/rib/route_test.go` | Pruned to the surviving accessors and `Index`; no surviving assertion changed |
| `TestPackAttributesSizesTheExtLengthHeaderItWrites` | Changed | `internal/component/bgp/rib/commit_extlength_test.go` | Not in the plan. Added by `50a4d9591e` for the defect the review found. Discrimination observed: with the `IsExtLength` term removed it reads "attribute runs past the block", 28 against 27 |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/bgp/rib/route.go` | Done | 231 lines removed |
| `internal/component/bgp/rib/incoming.go` | Done | Deleted |
| `internal/component/bgp/rib/outgoing.go` | Done | Deleted |
| `internal/component/bgp/rib/rib_test.go` | Done | Deleted |
| `internal/component/bgp/rib/outgoing_test.go` | Done | Deleted |
| `internal/component/bgp/rib/transaction_test.go` | Done | Deleted |
| `internal/component/bgp/rib/route_iter_test.go` | Changed | Not in the plan's list. Every case drove the deleted iterators, so it went with them |
| `internal/component/bgp/rib/sizeof_test.go` | Done | Field lines and the `OutgoingRIB` case dropped, `Route` reporter kept |
| `internal/component/bgp/rib/route_test.go` | Done | 447 lines removed |
| `docs/architecture/update-building.md` | Done | Landed in `50a4d9591e`, not `fa9faf5d94`, because the review closed it with the sizer fix |
| `docs/architecture/pool-architecture.md` | Done | Figures state the measured 96 bytes |
| `docs/architecture/rib-transition.md` | Done | Diagram corrected |
| `test/weakened.md` | Changed | The shared file no longer exists. Rows landed in `test/weakened/8c4ad6c3.md` |
| `internal/component/bgp/rib/commit.go` | Changed | Not in the plan. The review's sizer fix |
| `internal/core/bgp/attribute/attribute.go`, `origin.go` | Changed | Not in the plan. `AttrWireLenWithContext` exported and its caller updated |
| `docs/architecture/route-types.md`, `buffer-architecture.md`, `encoding-context.md`, `api/architecture.md` | Changed | Not in the plan's file list, but four of them described the removed cluster |

### Audit Summary
- **Total items:** 30 (3 requirements, 7 ACs, 4 tests, 16 files)
- **Done:** 22
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 8 (each recorded in Deviations from Plan)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Delete the dead surface | functional (compilation over the whole tracked tree) | `./le repository tracked-build check` OK on six flavors at `50a4d9591e`, 775-776 packages each. The deletion removed 2763 lines and nothing outside the package needed an edit, which is what "no production caller" predicted |
| Delete the fields the dead surface existed to serve | benchmark (struct measurement) | `TestStructSizes`: `Route struct: 96 bytes`, `nlri 16`, `nextHop 24`, `attributes 24`, `asPath 8`, `indexCache 24`. The five sum to 96 exactly, so the type carries no padding and no unread field |
| `Route` costs what it uses | benchmark (heap measurement) | `TestHeapBytesPerRoute`: 272 bytes per route over 100000 routes with typical attributes, 6.0 mallocs. `TestHeapBytesPerRouteMinimal`: 152 bytes, 2.0 mallocs. `docs/architecture/pool-architecture.md` line 1047 publishes the 96-byte figure, so the page and the reporter agree |
| No user-visible behavior changes (the spec's own Functional Tests section) | functional | The six-flavor tracked build plus the unchanged suites. Every removed symbol had no non-test caller, verified per symbol by the independent review at run 1 with a whole-tree word-boundary grep at HEAD and `git grep` at `HEAD^` for the historical caller |
| Interop | not applicable | The spec changes no wire-visible behavior: no symbol removed reached a wire path. The one wire-adjacent change, `50a4d9591e`, makes the rail allocate the buffer its writers already filled, and `TestPackAttributesSizesTheExtLengthHeaderItWrites` walks the packed block with the rules a peer's parser uses. `ai/rules/interop-and-goal-validation.md` exempts a pure internal refactor with no wire-visible change |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| none | Every AC is Done and every file in the plan is Done or Changed with the reason recorded above | - |

## Review Gate

| Run | BLOCKER | ISSUE | Notes |
|-----|---------|-------|-------|
| 1 | 0 | 4 | Independent review of `fa9faf5d94` against its parent. Deletion verified safe per symbol: whole-tree word-boundary grep at HEAD, `git grep` at HEAD^ for the historical caller, `go vet ./...` as the typecheck backstop because vet reads test files and `go build` does not. Every removed name returns zero on a `rib.`-selector grep; the surviving `Release`/`Acquire` hits belong to a local variable named `rib` holding a `*FamilyRIB` in the storage plugin, which does not import this package. Test edits are the pure `NewRoute` to `NewRouteWithASPath(.., nil)` rename in all seven files, no changed expectation and no removed assertion. `Route` measured 96 bytes by `TestStructSizes` over five fields with no padding. Nine of the weakened rows spot-checked against their subjects; none false. `tracked-build check` OK on six flavors. |
| 2 | 0 | 0 | All four ISSUEs closed. (1) The duplicate uncommitted shard `test/weakened/c7ef7dc3.md` is superseded by the committed `8c4ad6c3.md`. (2) `attribute.AttributesSizeWithContext` was orphaned by the deletion of its only non-test caller; it is re-homed rather than deleted, because the review found the rib rail held a hand-rolled duplicate of the header-size fact that had already diverged. `rib.attrSize` and `rib.attrSizeWithContext` tested `valueLen > 255` alone while `attrWireLenForValue` tests `valueLen > 255 \|\| attr.Flags().IsExtLength()`, and `WriteHeaderTo` emits the four-octet header on the FLAG, so the rail under-allocated by one octet for an attribute whose flags carry the bit over a short value. Both local sizers are deleted and the rail now calls `AttrWireLen`, the newly exported `AttrWireLenWithContext`, and `AttributesSizeWithContext`, which also replaces a hand-rolled summing loop. (3) and (4) the stale comment at `attribute.go` and the stale PREVENTS clause name live producers again. `TestPackAttributesSizesTheExtLengthHeaderItWrites` is added on the rail that was wrong: with the `IsExtLength` term removed it reads "attribute runs past the block", 28 against 27. `docs/architecture/update-building.md` no longer describes the removed cluster. |

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/rib-package-dead-surface-d968bfd4-b259-491c-bb73-eb71fca885a8.md` (21 files, verdict=clean) |
| `./le spec session review check` | OK (0 code files, clean, hashes match) |
| Rounds | 2. Round 1 raised four ISSUEs, round 2 read the fixes and closed them at 0/0. Neither round needed the cap, so no `rounds-reason` and no `owner-authorised` |
| Reviewer lenses used | logic+wiring; deletion blast radius per removed symbol; `docs/contributing/ze-go-style.md` pass over the changed Go; security (allocation, bounds, peer-reachable panic) |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | `test/weakened/c7ef7dc3.md` is an uncommitted duplicate of the committed shard for the same removals | `test/weakened/c7ef7dc3.md` | Closed as superseded by `test/weakened/8c4ad6c3.md`, which `fa9faf5d94` committed. The duplicate belongs to another session and is left in place untouched |
| 2 | ISSUE | The rib commit rail's `attrSize` and `attrSizeWithContext` were an open-coded copy of `attribute.attrWireLenForValue` that had dropped the `IsExtLength` term, so the rail under-allocated by one octet. Deleting the orphaned `AttributesSizeWithContext` would have removed the correct owner and left the defective copy on the live path | `internal/component/bgp/rib/commit.go`, `packAttributesWithASPath` | `50a4d9591e`: both local sizers deleted, the rail calls `attribute.AttrWireLen`, `attribute.AttrWireLenWithContext` (newly exported) and `attribute.AttributesSizeWithContext`. `TestPackAttributesSizesTheExtLengthHeaderItWrites` covers it, discrimination observed at 28 against 27 |
| 3 | ISSUE | The `WriteToWithContext` doc comment listed sizer call sites that the deletion had removed | `internal/core/bgp/attribute/attribute.go`, the `WriteToWithContext` doc comment | `50a4d9591e`: the list names the live callers again |
| 4 | ISSUE | `docs/architecture/update-building.md` still described the wire-byte cluster the engine route no longer holds | `docs/architecture/update-building.md` | `50a4d9591e`: the page describes `WireUpdate` as the forward path, and the `CanForwardDirect` source anchor moved with the prose |

The closure context re-read the complete diff of both commits, ran the style pass
over every changed Go file, and found nothing above NOTE. One NOTE, not fixed and
not blocking: the substituted comment line at `commit.go:505` now runs past the
100-column target, which `docs/contributing/ze-go-style.md` calls advisory.
No `panic()` exists in any changed file, so no peer can reach one. The one failure
path added, `off != totalLen` in `packAttributesWithASPath`, logs and returns an
error rather than a zero value.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/bgp/rib/` after the deletion | Yes | `ls -1`: `commit_edge_test.go commit_extlength_test.go commit.go commit_nexthop_test.go commit_test.go commit_wire_test.go doc.go grouping.go grouping_test.go rfc6793_aggregator_test.go route.go route_test.go sizeof_test.go` |
| `internal/component/bgp/rib/incoming.go` | No, by design | `ls: cannot access ...: No such file or directory` |
| `internal/component/bgp/rib/outgoing.go` | No, by design | `ls: cannot access ...: No such file or directory` |
| `internal/component/bgp/rib/commit_extlength_test.go` | Yes | Present in the `ls -1` above; added by `50a4d9591e` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | The two files are gone and no symbol they declared is referenced | `ls` above; `git grep -w -E 'IncomingRIB\|OutgoingRIB\|CanForwardDirect'` returns 0 hits in any `.go` file. The surviving hits are this spec, `plan/journal/gate-excludes-part-of-its-population.md`, and the two weakened shards |
| AC-2 | `Route` declares exactly five fields | `gopls symbols internal/component/bgp/rib/route.go`: `asPath`, `attributes`, `indexCache`, `nextHop`, `nlri` |
| AC-3 | `Route` measures 96 bytes | `go test -run TestStructSizes -v ./internal/component/bgp/rib/`: `Route struct: 96 bytes`, PASS |
| AC-4 | No reference-count write | `grep -rn 'refCount\|wireBytes\|nlriWireBytes\|sourceCtxID' internal/component/bgp/rib/` exits 1 with no output |
| AC-5 | Every flavor compiles | `./le repository tracked-build check`: OK on distro, test-runner, appliance, setup, host, installer at HEAD `50a4d9591e` |
| AC-6 | No documentation cites a deleted symbol or a stale byte figure | `docs/architecture/pool-architecture.md:1047` states 96 bytes. Both gates were run; see Documentation Verified |
| AC-7 | Every production symbol is still reachable | Six non-test importers, each reaching only `Route`, `NewRouteWithASPath`, `RouteJSON`, `NewCommitService`, `CommitOptions`. `go vet ./internal/component/bgp/... ./internal/core/bgp/...` exits 0, and vet typechecks test files |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `SendRoutes` building a route for a named commit | none, and none is owed | The spec's Functional Tests section declares that no user-visible behavior changes and names the tracked-build check as the evidence. `reactor_api_batch.go:415` calls `rib.NewRouteWithASPath(n, nextHop, attrs, asPath)` on the queue path and `reactor_api_batch.go:1566` declares `SendRoutes`. Both compile in six flavors, and `TestStructSizes` records the post-change size |
| A plugin reading routes through the reactor API | none, and none is owed | `bgptypes.ReactorAPI` declares `RIBInRoutes(peerID string) []rib.RouteJSON` (`internal/component/bgp/types/reactor.go:95`). Seven mock implementations across the BGP plugin packages satisfy it, and `go vet` over `./internal/component/bgp/...` typechecks all of them at exit 0 |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | The package path is `internal/component/bgp/rib`, so the Go toolchain forbids an importer outside this module. The six importers listed under AC-7 are all in-module |
| A-2 | confirmed | `internal/component/bgp/types/reactor.go:95` names `[]rib.RouteJSON` in the `ReactorAPI` interface. `RouteJSON` and its `MarshalJSON` survive at `route.go:137` and `route.go:144` |
| A-3 | confirmed | `sizeof_test.go` before the change contained 0 occurrences of `t.Error`, `t.Fatal`, `t.Fail`, `require.` or `assert.`, and it still contains 0. It reports through `t.Logf`, so pruning it removed no assertion |
| A-4 | confirmed | The compiler. Six build flavors green after the four fields were removed, and `go vet` green with test files included |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/features.md` -- no feature added or removed | `git grep -n 'IncomingRIB\|OutgoingRIB\|CanForwardDirect' docs/features.md` returns nothing; the spec removes no user-visible feature | Yes, no update needed |
| Command, API and plugin docs | No CLI verb, RPC, YANG leaf or plugin surface changed. `git grep` for the removed names across `docs/features/` and `docs/guide/` returns nothing | Yes, no update needed |
| `docs/architecture/update-building.md` | Re-read after `50a4d9591e`: the page describes `WireUpdate` as the forward path, and no `CanForwardDirect` anchor survives | Yes |
| `docs/architecture/pool-architecture.md` | Line 1047 reads "a 96-byte value the named-commit path hands to `CommitService`", which equals the AC-3 measurement | Yes |
| `docs/architecture/rib-transition.md` | The diagram box names `internal/component/bgp/plugins/rib/`, "Adj-RIB-In per peer" and "ribOut entries for replay" | Yes |
| `docs/architecture/route-types.md` | The `rib.Route` row reads "Route value handed to `CommitService`" with columns "NLRI, NextHop, Attrs, ASPath", matching the five fields `gopls symbols` reports | Yes |
| `docs/architecture/buffer-architecture.md` | The "The Engine Route Value" Go block lists the five surviving fields, and the Phase 4 row records the iterators as retired | Yes |
| `docs/architecture/encoding-context.md` | "Where the Wire Bytes Live" points at `wireu/wire_update.go`, `reactor_api_forward.go` and `AttributesWire.PackFor`, each with a source anchor | Yes |
| `docs/architecture/api/architecture.md` | The `internal/component/bgp/rib/outgoing.go` row is gone from the file table | Yes |
| Doctor checks | The change adds no file path, socket, kernel module, listen port, external binary or TLS cert. No `ze doctor` check is owed | Yes, no update needed |
| RFC status | No RFC-level behavior is implemented, changed or newly proven. Two RFC-tagged tests took the `NewRoute` to `NewRouteWithASPath(.., nil)` rename with no changed claim, recorded in `test/rfc-changed/8c4ad6c3.md` | Yes, no `rfc/short/` edit owed |
| `./le docs-to-code index-check` | Red, 2 stale references, both `internal/component/l2tp/plugins/shaper/filter_rate.go` from `docs/architecture/l2tp/bng-1-radius-attributes.md:12` and `docs/comparison.md:617`. `git log --diff-filter=D` attributes that deletion to `2be6e0b199 feat(l2tp): enforce the subscriber upload rate with an ingress policer`, another session's commit. No reference this spec touched is stale | Yes, red is not this spec's |
| `./le doc check links` | Red, 28 broken references. None of them names a file, a directory or a page this spec changed. The nearest hits are `internal/component/bgp/plugins/redistribute_ingress` and `internal/component/bgp/attribute`, both directories this spec never held | Yes, red is not this spec's |
| `./le doc check verify` | Red. Every failure names a `gh-pages` command-equivalent page or a command-catalog index row: `show errors`, `show interface errors`, and the whole `clear bgp ... `, `request bgp ... `, `show bgp ... ` index population. This spec changes no CLI command, no RPC and no YANG leaf, and no failure names `internal/component/bgp/rib`, `internal/core/bgp/attribute`, or any of the seven architecture pages edited. Left red per `ai/rules/pre-release.md` | Yes, red is not this spec's |
| `./le verify worktree` | Red at the lint stage, on `pkg/plugin/sdk/journal.go:1: import cycle not allowed in test (typecheck)`, repeated for every linux flavor. `pkg/plugin/sdk/` imports neither `internal/component/bgp/rib` nor `internal/core/bgp/attribute` (grep exits 1), and `git log -1 -- pkg/plugin/sdk/` attributes the package's last change to `31003a0dbf feat(plugin): a plugin declares what its own failure means`. Left red per `ai/rules/pre-release.md` | Yes, red is not this spec's |

## Core Insight

Deleting dead code is not a neutral act. A dead symbol can be the last live
reference that KEPT a correct implementation reachable, and removing it promotes
whichever open-coded copy remains. `attribute.AttributesSizeWithContext` was
orphaned by this spec's deletion, and its open-coded rival on the live rib rail
had already dropped the `IsExtLength` term. Had the deletion followed its own
plan and removed the orphan, the tree would have kept exactly one sizer, the
wrong one, with no gate able to notice. So the question a deletion owes is not
"who still calls this" but "what does the tree own twice, and which copy am I
about to make the only one".
