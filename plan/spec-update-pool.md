# Spec: update-pool

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-04-16 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` -- workflow rules
3. `.claude/rules/design-principles.md` -- "No make where pools exist", "Pool strategy by goroutine shape", "Encapsulation onion"
4. `plan/learned/603-make-pool-audit.md` -- the audit that surfaced these allocations
5. `internal/component/bgp/message/update_build.go` -- core builder + scratch
6. `internal/component/bgp/message/update_build_grouped.go` -- batch builder
7. `internal/component/bgp/reactor/peer_initial_sync.go` -- highest-volume caller
8. `internal/component/bgp/reactor/session_write.go` -- where *Update is consumed

## Task

Close the two open deferrals from the make-pool audit (`learned/603`)
by routing every `Update.PathAttributes` and `Update.NLRI` allocation
through the existing `UpdateBuilder.scratch` buffer instead of
`make([]byte, N)`.

The allocations were classified as needing API redesign in the audit,
but research shows the current `*Update` lifetime in production callers
is already short and synchronous (Build → SendUpdate → discard, with
no cross-iteration retention and no caching). The "redesign" reduces to:

1. Make every PathAttributes / NLRI / inline-NLRI / result allocation
   come from `ub.alloc()` (which sub-slices scratch).
2. Document the implicit lifetime invariant on `Update` so future
   callers don't break it.
3. Convert `BuildGroupedUnicastWithLimit`'s multi-update return value
   to a callback API so all updates in a batch can share scratch
   safely.

## Required Reading

### Architecture Docs

- [ ] `docs/architecture/update-building.md` -- UpdateBuilder shape, scratch contract.
  → Decision: `ub.scratch` is intentionally reused across `Build*` calls; callers must consume the previous result before the next build.
  → Constraint: scratch is sized to `wire.StandardMaxSize` (4096) initially and grows on demand for extended messages.
- [ ] `.claude/rules/design-principles.md` -- "Encapsulation onion" + "No make where pools exist"
  → Decision: every wire-facing variable-size allocation must come from a bounded pool.
  → Constraint: builder.scratch IS the pool for the build path; bypassing it via `make([]byte, ...)` is the violation.
- [ ] `plan/learned/603-make-pool-audit.md`
  → Decision: original audit deferred this work because the allocations escape via `*Update`. Re-research found the lifetime is short enough to use scratch directly.

### RFC Summaries

- [ ] `rfc/short/rfc4271.md` -- UPDATE message format (Section 4.3)
  → Constraint: PathAttributes max 4096-23 = 4073 bytes for standard messages, 65535-23 for RFC 8654 Extended Message peers.
- [ ] `rfc/short/rfc4760.md` -- MP_REACH/MP_UNREACH (multi-protocol)
  → Constraint: NLRI section size bounded by total message size minus header + attributes.
- [ ] `rfc/short/rfc8654.md` -- Extended Message
  → Constraint: scratch must grow to 65535 bytes when negotiated.

**Key insights:**
- `ub.scratch` is the de-facto pool for the build path -- it already
  exists, is reused, and grows on demand. The "API redesign" the audit
  feared is not needed; the missing piece is *discipline* (route all
  builds through `alloc`) plus *documentation* (state the lifetime
  invariant on `Update`).
- The only API change required is `BuildGroupedUnicastWithLimit`,
  which returns `[]*Update`. Multiple Updates sharing the same
  scratch are safe TODAY only because the caller's `for ... SendUpdate`
  loop happens between resets -- but future callers that interleave
  builds across multiple builders, or that retain the Update beyond
  the next build, would break silently.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/message/update_build.go` -- core UpdateBuilder, scratch, alloc, BuildUnicast
  → Constraint: `inlineNLRI := make(...)` (line 290), `attrBytes := make(...)` (line 330), `packAttributesOrdered` returns `result := make(...)` (line 496) -- three sites bypass `alloc`.
- [ ] `internal/component/bgp/message/update_build_grouped.go` -- multi-update batch builder
  → Constraint: `attrBytes := make(...)` (line 207), `currentNLRI = append(currentNLRI, ...)` builds an independent heap slice, `&Update{PathAttributes: attrBytes, NLRI: currentNLRI}` allocates Update structs themselves.
- [ ] `internal/component/bgp/message/update_build_{vpn,labeled,evpn,vpls,flowspec,mvpn,mup}.go` -- each calls `packAttributesOrdered` returning a fresh `[]byte`.
- [ ] `internal/component/bgp/reactor/peer_initial_sync.go` -- production caller; `ub := message.NewUpdateBuilder(...)` followed by Build / SendUpdate sequences. SendUpdate is synchronous (writes to session bufWriter, flushes later).
- [ ] `internal/component/bgp/reactor/session_write.go` -- `writeUpdate(update)` calls `update.WriteTo(s.writeBuf.Buffer(), 0, nil)` then writes to `bufWriter`. After WriteTo, the caller never references `update.PathAttributes` or `update.NLRI` again.

**Behavior to preserve:**
- The `Update` struct shape (`PathAttributes []byte`, `NLRI []byte`) -- consumers (`update.WriteTo`, splitters, tests) read these fields.
- All public constructors (`NewUpdateBuilder`, `BuildUnicast`, `BuildVPN`, etc.) keep their signatures EXCEPT `BuildGroupedUnicastWithLimit` which migrates to a callback shape.
- Wire output bytes are byte-for-byte identical (verified by `wire_compat_test.go` round-trips).
- Test helpers in `update_build_test.go` continue to work; they retain `*Update` returned from `BuildUnicast` only briefly before asserting on its fields, with no second `Build*` call in between.

**Behavior to change:**
- `inlineNLRI`, `attrBytes`, `packAttributesOrdered` result, NLRI builders' `attrBytes` -- all routed through `ub.alloc`.
- `BuildGroupedUnicastWithLimit(routes, maxSize) ([]*Update, error)` becomes `BuildGroupedUnicast(routes, maxSize, func(*Update) error) error` so each update is consumed before the next build advances scratch.
- `Update.PathAttributes` / `Update.NLRI` get a doc comment stating the lifetime invariant.

## Data Flow (MANDATORY)

### Entry Point

- Production: `peer_initial_sync.go` and `peer_static_routes.go` create an `UpdateBuilder`, build an `*Update`, hand it to `SendUpdate(update)`.
- Format at entry: in-memory `UnicastParams` / `VPNParams` / etc.

### Transformation Path

1. `NewUpdateBuilder(...)` constructs a builder with empty scratch.
2. First `Build*(p)` call invokes `resetScratch()` which lazy-allocates `scratch = make([]byte, wire.StandardMaxSize)` if nil, sets `off = 0`.
3. `Build*` calls `ub.alloc(n)` for every sub-slice it needs (NLRI bytes, AS_PATH bytes, MPReach NLRI bytes, ...). After this spec, ALL allocations including `inlineNLRI`, `attrBytes`, `packAttributesOrdered` result also go through `alloc`.
4. `Build*` returns `&Update{PathAttributes: scratchSlice, NLRI: scratchSlice}` -- both slices alias `ub.scratch`.
5. Caller passes `*Update` to `SendUpdate(update)`.
6. `session.writeUpdate(update)` calls `update.WriteTo(s.writeBuf.Buffer(), 0, nil)` -- copies wire bytes from `update.PathAttributes` and `update.NLRI` into session's writeBuf.
7. Caller may now call `Build*` again -- `resetScratch()` invalidates the previous Update's slices.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Builder ↔ Reactor | `*Update` returned, slice fields alias scratch | [ ] doc on Update.PathAttributes / Update.NLRI |
| `Update.WriteTo` ↔ session writeBuf | `copy(...)` into bufWriter buffer | [ ] no change; existing |
| BuildGrouped ↔ caller | callback per built Update; caller fully consumes (sends to wire) before callback returns | [ ] new contract |

### Integration Points

- `update.WriteTo(buf, off, ctx)` -- existing function; no change. Reads PathAttributes + NLRI as `[]byte`, copies into buf. After WriteTo returns, the caller is free to invalidate the source slices.
- `SplitUpdateWithAddPath(update, maxSize, addPath)` -- existing splitter. Returns multiple `*Update`. Currently allocates fresh slices per chunk. After this spec it accepts a builder + writes chunks via callback so chunk slices alias scratch.
- `wire_compat_test.go` -- existing round-trip tests; must continue to pass byte-for-byte.

### Architectural Verification

- [ ] No bypassed layers: every `make([]byte, N)` in `update_build*.go` is replaced by `ub.alloc(n)` or removed.
- [ ] No unintended coupling: `*Update` shape and field types unchanged; consumers of `Update.PathAttributes`/`NLRI` still see a `[]byte`.
- [ ] No duplicated functionality: the existing scratch + alloc IS the pool; no new pool type added.
- [ ] Zero-copy preserved: PathAttributes and NLRI alias scratch (no copy from build buffer to Update struct).

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Reactor `peer_initial_sync` builds + sends UPDATE | → | `BuildUnicast` → `Update.WriteTo` → `bufWriter` | `test/encode/initial-sync-zero-alloc.ci` (allocs/op assertion) |
| Reactor multi-route batch send | → | `BuildGroupedUnicast(routes, maxSize, send)` callback fires per Update | `test/encode/grouped-zero-alloc.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ub.BuildUnicast(p)` called twice on the same builder, no SendUpdate between | Second build's PathAttributes / NLRI fully overwrite the first; consuming the first Update after the second build is a documented bug (caller error, not a builder bug) |
| AC-2 | `inlineNLRI` for IPv4 unicast | Comes from `ub.alloc`, not `make` -- verified by a benchmark/AllocsPerRun assertion showing 0 allocs/op for the build path (excluding the params struct + the *Update struct) |
| AC-3 | `attrBytes` in `BuildUnicast` for IPv6 / non-unicast SAFI | Comes from `ub.alloc` (writing via WriteAttributesOrdered into a pre-sized scratch sub-slice) |
| AC-4 | `packAttributesOrdered` in vpn/labeled/evpn/vpls/flowspec/mvpn/mup paths | Replaced by `packAttributesOrderedInto(ub *UpdateBuilder, attrs)` which writes into scratch |
| AC-5 | `BuildGroupedUnicast(routes, maxSize, send func(*Update) error) error` | Each built Update is passed to `send` synchronously; after `send` returns nil, the next Update may reuse scratch space; if `send` returns non-nil, builder stops and returns the error |
| AC-6 | Existing `BuildGroupedUnicastWithLimit` callers (perf, chaos, peer_initial_sync) | Migrated to the callback API; no remaining caller of the slice-returning variant |
| AC-7 | `wire_compat_test.go` and all existing builder tests | Pass byte-for-byte unchanged |
| AC-8 | `Update` godoc | Documents the slice-aliasing-scratch invariant: "PathAttributes and NLRI may alias the source UpdateBuilder's scratch buffer; callers MUST consume (WriteTo, copy out, or hand to SendUpdate which copies internally) before the next Build* call on the same builder." |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestUpdateBuilder_BuildUnicast_AliasesScratch` | `update_build_test.go` | `update.PathAttributes` is a sub-slice of `ub.scratch`; mutating scratch changes PathAttributes | |
| `TestUpdateBuilder_BuildTwice_InvalidatesFirst` | `update_build_test.go` | After two builds without WriteTo between, the first Update's PathAttributes content is overwritten | |
| `TestUpdateBuilder_BuildUnicast_ZeroAllocAfterWarmup` | `update_build_test.go` | `testing.AllocsPerRun(100, func)` ≤ 1 (only the *Update struct itself) for IPv4 unicast | |
| `TestUpdateBuilder_BuildIPv6_ZeroAllocAfterWarmup` | `update_build_test.go` | Same assertion for IPv6 (MP_REACH path) | |
| `TestBuildGroupedUnicast_CallbackOrder` | `update_build_grouped_test.go` | Callback fires in route order; each Update has correct NLRI for its chunk | |
| `TestBuildGroupedUnicast_CallbackError_StopsBuilder` | `update_build_grouped_test.go` | If callback returns error, no further callbacks fire and the error propagates | |
| `TestBuildGroupedUnicast_ScratchReuse` | `update_build_grouped_test.go` | Across N callbacks, total scratch growth is bounded (no per-update growth) | |
| `TestPackAttributesOrderedInto_ZeroAlloc` | `update_build_test.go` | `packAttributesOrderedInto(ub, attrs)` produces same bytes as old `packAttributesOrdered(attrs)` with zero allocs after warmup | |

### Boundary Tests

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Standard message scratch | <=4096 bytes | 4073 (PathAttributes max) | N/A | scratch grows to 65535 (Extended Message) |
| Extended message scratch | <=65535 bytes | 65512 (PathAttributes max) | N/A | grow logic handles arbitrary `newSize` |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `initial-sync-zero-alloc` | `test/encode/initial-sync-zero-alloc.ci` | Sends 100 IPv4 unicast UPDATEs from peer_initial_sync; asserts wire bytes match expected and (via debug log) builder allocates ≤1 per build | |
| `grouped-batch-send` | `test/encode/grouped-batch-send.ci` | Sends 1000-route batch via BuildGroupedUnicast callback; verifies all routes appear in wire output, in order, split across UPDATEs respecting maxSize | |

### Future (deferred)

- Benchmarks in `internal/component/bgp/message/update_build_bench_test.go` showing allocs/op delta -- deferred until the spec lands; benchmarks belong in a perf-focused follow-up.

## Files to Modify

- `internal/component/bgp/message/update_build.go` -- inlineNLRI, attrBytes, packAttributesOrdered routed through alloc; Update godoc.
- `internal/component/bgp/message/update_build_grouped.go` -- BuildGroupedUnicastWithLimit replaced by BuildGroupedUnicast callback API; currentNLRI built into scratch.
- `internal/component/bgp/message/update_build_{vpn,labeled,evpn,vpls,flowspec,mvpn,mup}.go` -- each calls `packAttributesOrderedInto(ub, attrs)` instead of `packAttributesOrdered(attrs)`.
- `internal/component/bgp/message/update.go` -- godoc comment on `Update.PathAttributes` and `Update.NLRI` documenting the scratch-aliasing lifetime invariant.
- `internal/component/bgp/reactor/peer_initial_sync.go` -- migrate the one `BuildGroupedUnicastWithLimit` caller (line 114) to the callback shape.
- `internal/perf/sender.go`, `internal/chaos/peer/sender.go` -- if they use BuildGrouped*, migrate too (verify during implementation).

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [ ] No | -- |
| CLI commands/flags | [ ] No | -- |
| Editor autocomplete | [ ] No | -- |
| Functional test for new RPC/API | [ ] Yes | `test/encode/*.ci` |

### Documentation Update Checklist

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] No | -- |
| 2 | Config syntax changed? | [ ] No | -- |
| 3 | CLI command added/changed? | [ ] No | -- |
| 4 | API/RPC added/changed? | [ ] No | -- |
| 5 | Plugin added/changed? | [ ] No | -- |
| 6 | Has a user guide page? | [ ] No | -- |
| 7 | Wire format changed? | [ ] No | -- |
| 8 | Plugin SDK/protocol changed? | [ ] No | -- |
| 9 | RFC behavior implemented? | [ ] No | -- |
| 10 | Test infrastructure changed? | [ ] No | -- |
| 11 | Affects daemon comparison? | [ ] No | -- |
| 12 | Internal architecture changed? | [ ] Yes | `docs/architecture/update-building.md` (lifetime invariant on Update slices) |

## Files to Create

- `test/encode/initial-sync-zero-alloc.ci` -- functional test exercising the migrated path
- `test/encode/grouped-batch-send.ci` -- functional test for the callback API

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Implementation phases below |
| 4. Full verification | `make ze-verify-fast` + `make ze-race-reactor` |
| 5. Critical review | Critical Review Checklist below |
| 6-11. Standard | -- |
| 12. Present summary | Executive Summary Report |

### Implementation Phases

1. **Phase 1: route inline allocations through alloc** -- replace `inlineNLRI := make(...)`, `attrBytes := make(...)` in `update_build.go::BuildUnicast`. Add Update godoc.
   - Tests: `TestUpdateBuilder_BuildUnicast_AliasesScratch`, `TestUpdateBuilder_BuildUnicast_ZeroAllocAfterWarmup`, existing `update_build_test.go` round-trips.
   - Files: `update_build.go`, `update.go`.
   - Verify: tests fail (alloc count > 1) → implement → tests pass.

2. **Phase 2: packAttributesOrderedInto** -- introduce the scratch-writing variant; migrate vpn/labeled/evpn/vpls/flowspec/mvpn/mup callers; delete the old `packAttributesOrdered` if no callers remain.
   - Tests: `TestPackAttributesOrderedInto_ZeroAlloc`; existing `update_build_{vpn,labeled,evpn,vpls,flowspec,mvpn,mup}_test.go`.
   - Files: `update_build.go` + 7 nlri-family build files.
   - Verify: byte-for-byte identical wire output, zero allocs.

3. **Phase 3: BuildGroupedUnicast callback API** -- introduce `BuildGroupedUnicast(routes, maxSize, send func(*Update) error) error`; migrate the one production caller (`peer_initial_sync.go:114`); migrate perf/chaos if needed; delete `BuildGroupedUnicastWithLimit`.
   - Tests: `TestBuildGroupedUnicast_CallbackOrder`, `TestBuildGroupedUnicast_CallbackError_StopsBuilder`, `TestBuildGroupedUnicast_ScratchReuse`.
   - Files: `update_build_grouped.go`, `peer_initial_sync.go`, perf/chaos senders.
   - Verify: production caller still serializes UPDATEs in route order; functional tests pass; `make ze-race-reactor` clean.

4. **Phase 4: Functional tests** -- write the two `.ci` files exercising the migrated paths.
   - Files: `test/encode/initial-sync-zero-alloc.ci`, `test/encode/grouped-batch-send.ci`.

5. **Phase 5: Architecture doc + closeout** -- update `docs/architecture/update-building.md` with the lifetime invariant; close the two open deferrals in `plan/deferrals.md`; write `plan/learned/NNN-update-pool.md`.

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | wire_compat_test.go round-trips byte-identical; no test allocates `make([]byte, N)` in update_build*.go on the hot path |
| Naming | `BuildGroupedUnicast` (not `WithLimit`); `packAttributesOrderedInto` parallels `WriteAttributesOrdered` |
| Data flow | All Update.PathAttributes / Update.NLRI alias scratch; no caller retains *Update across a Build* call |
| Rule: no-layering | `BuildGroupedUnicastWithLimit` and `packAttributesOrdered` deleted, not kept as wrappers |
| Rule: buffer-first | Every variable-size allocation routed through alloc |
| Rule: ze-race-reactor | passes after Phase 3 (peer_initial_sync touched) |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Old `packAttributesOrdered` deleted | `grep -rn "packAttributesOrdered\b" internal/` returns no `func` definitions, only `packAttributesOrderedInto` |
| Old `BuildGroupedUnicastWithLimit` deleted | `grep -rn "BuildGroupedUnicastWithLimit" internal/ cmd/` returns no callers |
| Update godoc in place | `grep -A3 "PathAttributes" internal/component/bgp/message/update.go` shows the lifetime comment |
| `.ci` tests run | `bin/ze-test bgp encode --list \| grep zero-alloc` returns the two new tests |
| Audit deferrals closed | `grep "spec-update-pool" plan/deferrals.md` shows status `done` |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | Routes / params already validated upstream; no new untrusted input |
| Slice aliasing | A test asserting `update.PathAttributes` aliases scratch -- if a caller ever modifies returned PathAttributes, it modifies scratch. Add `// note: read-only` to the godoc. |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error after Phase 2 | Fix per-family build files in same phase |
| Wire output diff | wire_compat_test.go is ground truth -- fix the implementation, not the test |
| `ze-race-reactor` flake | Investigate with `-count=20`; do not skip |
| Callback API breaks a non-obvious caller | Add the missing migration to Phase 3 before continuing |

## Mistake Log

(empty -- to be filled during implementation)

## Design Insights

(empty -- to be filled during implementation)

## RFC Documentation

No new RFC enforcement; preserves existing behavior. RFC 4271 / 4760 / 8654 references stay where they are in the build code.

## Implementation Summary

(filled at completion)

## Implementation Audit

(filled at completion)

## Pre-Commit Verification

(filled at completion)

## Checklist

### Goal Gates

- [ ] AC-1..AC-8 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-verify-fast` passes
- [ ] `make ze-race-reactor` passes (peer_initial_sync.go touched)
- [ ] No remaining `make([]byte, N)` in update_build*.go on the build hot path
- [ ] Architecture doc updated
- [ ] Critical Review passes

### Quality Gates

- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design

- [ ] No premature abstraction (the callback shape is needed for scratch reuse, not speculation)
- [ ] No speculative features
- [ ] Single responsibility (alloc handles ALL allocations from scratch)
- [ ] Explicit > implicit (Update godoc states the lifetime contract)
- [ ] Minimal coupling (no new pool type; reuses existing scratch)

### TDD

- [ ] Tests written before implementation
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] `make ze-test` (full suite) passes once feature complete
- [ ] Functional tests exist
- [ ] Wire round-trips byte-identical

### Completion

- [ ] Critical Review passes -- all 6 checks in `rules/quality.md`
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Learned summary written
- [ ] Summary included in commit
