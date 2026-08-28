# Spec: the rest of the dead surface in the bgp rib package

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | plugin |
| Depends | - |
| Phase | DESIGN |
| Deferral shard | - |
| Handoff | - |
| Updated | 2026-08-18 |

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
| AC-5 | All four flavors of the committed tree compile | `./le repository-tracked-build check` |
| AC-6 | No documentation cites a deleted symbol or a stale byte figure | `./le docs-to-code index-check` and `./le doc-check links` both green, and the `pool-architecture.md` figures equal the AC-3 measurement |
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
`./le repository-tracked-build check` green across all four flavors and the
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
7. Run `./le changed scope`, `go test -race ./...` for the package, `./le docs-to-code index-check` and `./le doc-check links`
8. Write `test/weakened.md` rows against this commit's removals only
9. Commit, then `./le repository-tracked-build check`

## Checklist

- [ ] Tests written
- [ ] Tests FAIL before implementation
- [ ] Tests PASS after implementation
- [ ] `./le verify current mode full` green

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
- [ ] Source anchors elsewhere - checked: `route-types.md`, `buffer-architecture.md` and `guide/route-injection.md` name `Route` or `commit.go` and survive

## Review Gate

| Run | BLOCKER | ISSUE | Notes |
|-----|---------|-------|-------|
| | | | |
