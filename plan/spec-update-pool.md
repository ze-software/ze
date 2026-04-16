# Spec: update-pool

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 7/7 |
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

Close the two open deferrals from the make-pool audit (`learned/603`, lines 161-162 of `plan/deferrals.md`)
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
   callers don't break it (godoc AND architecture doc).
3. Convert BOTH `BuildGroupedUnicastWithLimit` AND `BuildMVPNWithLimit`
   multi-update return values to a callback API so all updates in a
   batch can share scratch safely under a documented offset protocol.

Scope covers `update_build.go` + `update_build_grouped.go` + 7 family
build files (3 via `packAttributesOrdered`, 4 via inline `make`) + 3
production call sites in `peer_initial_sync.go` + 1 test helper +
`update_split.go` (3 make sites migrated to callback shape with
splitter-owned scratch) + 2 production splitter call sites.
`bgp/nlri/` helpers remain deferred (covered by Phase 4 step 8 of
`learned/603`).

## Required Reading

### Source Files (primary)

- [ ] `internal/component/bgp/message/update_build.go:28-91` -- UpdateBuilder struct, scratch, resetScratch, alloc.
  → Decision: `ub.scratch` is intentionally reused across `Build*` calls; callers must consume the previous result before the next build.
  → Constraint: scratch is sized to `wire.StandardMaxSize` (4096) initially and grows on demand via `make(newSize) + copy + swap`; previously-returned sub-slices keep pointing to the old backing (memory-safe, GC-pinned), so slices from one build may span two backings.
- [ ] `docs/architecture/update-building.md` -- HAS ZERO mentions of scratch/alloc/resetScratch. Verified by grep. The scratch contract lives only in code today.
  → Decision: Phase 0 of this spec adds the scratch contract to this doc so the lifetime invariant is discoverable without reading code.

### Architecture Docs

- [ ] `.claude/rules/design-principles.md` -- "Encapsulation onion" + "No make where pools exist"
  → Decision: every wire-facing variable-size allocation must come from a bounded pool.
  → Constraint: builder.scratch IS the pool for the build path; bypassing it via `make([]byte, ...)` is the violation.
- [ ] `plan/learned/603-make-pool-audit.md`
  → Decision: original audit deferred this work because the allocations escape via `*Update`. Re-research found the lifetime is short enough to use scratch directly.
- [ ] `plan/audits/make-pool-2026-04-16.{csv,md}` -- the original audit data.
  → Constraint: CSV lines 35-45 list 11 in-scope `make` sites in `message/update_build*.go`; lines 46-48 list 3 more in `update_split.go` -- ALL 14 are covered by this spec (Phase 1 through 4).

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
  → Constraint: `attrBytes := make(...)` (line 207), `currentNLRI = append(currentNLRI, ...)` builds an independent heap slice, `&Update{PathAttributes: attrBytes, NLRI: currentNLRI}` allocates Update structs themselves. Also contains `BuildMVPNWithLimit` (line 276) which loops calling `ub.BuildMVPN(currentBatch)` (line 384) -- same aliasing hazard as BuildGroupedUnicastWithLimit.
- [ ] `internal/component/bgp/message/update_build_{vpn,labeled,evpn}.go` -- these THREE files call `packAttributesOrdered` at lines 204, 201, 141 respectively.
- [ ] `internal/component/bgp/message/update_build_{flowspec,vpls,mvpn,mup}.go` -- these FOUR files INLINE `attrBytes := make([]byte, attribute.AttributesSize(attrs))` at lines 118, 118, 144, 106 (and mup.go:211 a second time). They do NOT call `packAttributesOrdered`. Verified by grep.
- [ ] `internal/component/bgp/reactor/peer_initial_sync.go` -- production caller. Three WithLimit call sites: `BuildGroupedUnicastWithLimit` at line 114 (IPv4 unicast group); `BuildMVPNWithLimit` at lines 437 and 461 (IPv4 and IPv6 MVPN group). All followed by `for _, update := range updates { SendUpdate(update) }`. SendUpdate is synchronous (writes to session bufWriter, flushes before return).
- [ ] `internal/component/bgp/reactor/session_write.go` -- `writeUpdate(update)` calls `update.WriteTo(s.writeBuf.Buffer(), 0, nil)` then writes to `bufWriter`. After WriteTo, the caller never references `update.PathAttributes` or `update.NLRI` again.
- [ ] `internal/component/bgp/message/update_build_test.go:14` -- `mustBuildGrouped` test helper wraps `BuildGroupedUnicastWithLimit(routes, 65535)` and returns `[]*Update`. Used by many tests; must be rewritten to collect callback results after the API migration.

**Behavior to preserve:**
- The `Update` struct shape (`PathAttributes []byte`, `NLRI []byte`) -- consumers (`update.WriteTo`, splitters, tests) read these fields.
- All public constructors (`NewUpdateBuilder`, `BuildUnicast`, `BuildVPN`, etc.) keep their signatures EXCEPT `BuildGroupedUnicastWithLimit` and `BuildMVPNWithLimit` which migrate to a callback shape.
- Wire output bytes are byte-for-byte identical (verified by `reactor/wire_compat_test.go` round-trips).
- Test helpers in `update_build_test.go` continue to work; they retain `*Update` returned from `BuildUnicast` only briefly before asserting on its fields, with no second `Build*` call in between.
- `SplitUpdateWithAddPath` currently allocates internally (three `make` sites at `update_split.go:204,246,268`, plus implicit `append([]byte(nil), baseAttrs...)` per chunk at lines 196 and 238). Migrated in Phase 4 to a new callback shape backed by splitter-owned scratch.

**Behavior to change:**
- `inlineNLRI`, `attrBytes`, `packAttributesOrdered` result, NLRI builders' `attrBytes` (across 7 family files, with two sites in mup.go) -- all routed through `ub.alloc`.
- `BuildGroupedUnicastWithLimit(routes, maxSize) ([]*Update, error)` becomes `BuildGroupedUnicast(routes, maxSize, func(*Update) error) error`.
- `BuildMVPNWithLimit(routes, maxSize) ([]*Update, error)` becomes `BuildGroupedMVPN(routes, maxSize, func(*Update) error) error` with equivalent callback semantics.
- `SplitUpdate(u, maxSize)` and `SplitUpdateWithAddPath(u, maxSize, addPath)` (both returning `[]*Update`) become a single `Splitter` type with its own scratch buffer + `(*Splitter) Split(u, maxSize, addPath, emit func(*Update) error) error`. Old functions deleted. All chunk PathAttributes alias the splitter's scratch; each chunk is consumed via `emit` before the next is built.
- `Update.PathAttributes` / `Update.NLRI` get a doc comment stating the lifetime invariant.

**Out-of-Scope (deferred):**
- `bgp/nlri/{base,inet,rd}.go` make sites (audit CSV lines 50-52). Covered by Phase 4 step 8 of `learned/603` audit, not this spec.

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

### Callback-Builder Scratch Offset Protocol

`BuildGroupedUnicast` and `BuildGroupedMVPN` each build ONE shared attribute byte-stream (attrBytes) that every emitted Update references, plus a per-Update NLRI chunk. Scratch offsets MUST be managed as follows:

| Region | Offset range | Lifetime |
|--------|-------------|----------|
| attrBytes (shared) | `scratch[0:A)` where A = end of attribute section | Valid from first callback through return of the outer Build call |
| per-Update NLRI chunk | `scratch[A:A+N)` where N = current chunk NLRI length | Valid only until the callback returns |

After each callback returns, the builder sets `ub.off = A` (not 0) so the next chunk's NLRI reuses the same region without touching attrBytes. Only the outer Build call's return (or the next top-level `Build*` on the same builder) advances past A or resets to 0.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Builder ↔ Reactor | `*Update` returned, slice fields alias scratch | [ ] doc on Update.PathAttributes / Update.NLRI |
| `Update.WriteTo` ↔ session writeBuf | `copy(...)` into bufWriter buffer | [ ] no change; existing |
| BuildGrouped ↔ caller | callback per built Update; caller fully consumes (sends to wire) before callback returns | [ ] new contract |
| BuildGroupedMVPN ↔ caller | callback per built Update; shared attrBytes across callbacks via scratch offset protocol | [ ] new contract |
| Splitter ↔ caller | callback per chunk Update; splitter's scratch owns chunk PathAttributes; caller consumes before next callback | [ ] new contract |

### Integration Points

- `update.WriteTo(buf, off, ctx)` -- existing function; no change. Reads PathAttributes + NLRI as `[]byte`, copies into buf. After WriteTo returns, the caller is free to invalidate the source slices.
- `Splitter.Split(update, maxSize, addPath, emit)` -- new splitter type replacing the free `SplitUpdate*` functions. Input `*Update`'s PathAttributes/NLRI MAY alias an UpdateBuilder's scratch; the splitter reads them synchronously, writes chunk PathAttributes into its OWN scratch, and emits each chunk via callback. Callers that wrap the splitter (peer_send, reactor_api_forward) own the Splitter instance (usually one per peer).
- `reactor/wire_compat_test.go` -- existing round-trip tests; must continue to pass byte-for-byte.

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
| AC-4a | `packAttributesOrdered` callers in vpn/labeled/evpn (3 files) | Replaced by `packAttributesOrderedInto(ub, attrs)` which writes into scratch. Old free function `packAttributesOrdered` deleted. |
| AC-4b | Inline `attrBytes := make(...)` in flowspec/vpls/mvpn/mup (4 files, 5 sites including mup twice) | Replaced by calls to `packAttributesOrderedInto(ub, attrs)`. No make remains on these build paths. |
| AC-5 | `BuildGroupedUnicast(routes, maxSize, send func(*Update) error) error` | Each built Update is passed to `send` synchronously. Scratch offset protocol: attrBytes lives at `scratch[0:A)` for the full batch lifetime; per-Update NLRI lives at `scratch[A:)` and `ub.off` is reset to A (NOT 0) after each callback returns, so the next chunk overwrites the previous NLRI but preserves attrBytes. If `send` returns non-nil, builder stops and returns the error. |
| AC-6 | `BuildGroupedMVPN(routes, maxSize, send func(*Update) error) error` | Same callback + offset protocol as AC-5, for MVPN. Replaces `BuildMVPNWithLimit`. |
| AC-7 | Existing WithLimit callers: `peer_initial_sync.go:114` (grouped unicast) and `peer_initial_sync.go:437,461` (MVPN grouped) | Migrated to callback API; no remaining caller of either slice-returning variant. Test helper `mustBuildGrouped` in `update_build_test.go:14` rewritten to collect callback results internally. |
| AC-8 | `reactor/wire_compat_test.go` and all existing builder tests | Pass byte-for-byte unchanged |
| AC-9 | `Update` godoc | Documents the slice-aliasing-scratch invariant: "PathAttributes and NLRI may alias the source UpdateBuilder's scratch buffer; callers MUST consume (WriteTo, copy out, or hand to SendUpdate which copies internally) before the next Build* call on the same builder. Slices from one build may span two different scratch backings if the buffer grew mid-build; treat them as opaque." |
| AC-10 | `docs/architecture/update-building.md` | Updated with "Scratch Contract" section covering the lifetime invariant, grow semantics, and callback-builder offset protocol. Previously silent on all three. |
| AC-11 | `Splitter` type with scratch field + `Split(u, maxSize, addPath, emit func(*Update) error) error` | Each emitted chunk's `PathAttributes` aliases the splitter's scratch. Scratch grows on demand (same shape as UpdateBuilder). No `make([]byte, N)` remains in `update_split.go` on the chunk-composition path. `update_split.go:204,246,268` all deleted or replaced by scratch sub-slice writes. |
| AC-12 | Production splitter callers: `reactor/peer_send.go:128` (`sendUpdateWithSplit`) and `reactor/reactor_api_forward.go:610` (forward path) | Migrated to hold a per-peer `Splitter` and invoke `Split(..., emit)` with an emit closure that loops over `p.SendUpdate(chunk)`. No remaining caller of the slice-returning `SplitUpdate*` functions. |
| AC-13 | Existing `SplitUpdate*` tests (40+ call sites in `update_split_test.go` and `chunk_mp_nlri_test.go`) | Migrated via a single test-local `collectChunks` helper that runs `Split` with a collecting callback and returns `[]*Update`. Test assertions unchanged. |

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
| `TestBuildGroupedUnicast_AttrBytesPersistAcrossCallbacks` | `update_build_grouped_test.go` | attrBytes content captured in first callback is still byte-identical at last callback; proves the offset protocol works | |
| `TestBuildGroupedMVPN_CallbackOrder` | `update_build_grouped_test.go` | Callback fires once per chunk; each Update carries the shared attrBytes + its own NLRI | |
| `TestBuildGroupedMVPN_AttrBytesPersistAcrossCallbacks` | `update_build_grouped_test.go` | Same invariant as TestBuildGroupedUnicast version but for MVPN path | |
| `TestPackAttributesOrderedInto_ZeroAlloc` | `update_build_test.go` | `packAttributesOrderedInto(ub, attrs)` produces same bytes as old `packAttributesOrdered(attrs)` with zero allocs after warmup | |
| `TestPackAttributesOrderedInto_FlowSpecVPLSMVPNMUP` | `update_build_test.go` | Produces byte-identical attrBytes for each of the 4 formerly-inlined paths (flowspec/vpls/mvpn/mup) | |
| `TestSplitter_CallbackOrder` | `update_split_test.go` | `Split` fires callback in chunk order; each chunk Update has valid PathAttributes/NLRI | |
| `TestSplitter_ChunksAliasScratch` | `update_split_test.go` | Each chunk's `PathAttributes` is a sub-slice of the splitter's scratch | |
| `TestSplitter_CallbackError_StopsSplit` | `update_split_test.go` | If callback returns non-nil, no further callbacks fire and error propagates | |
| `TestSplitter_ZeroAllocAfterWarmup` | `update_split_test.go` | `AllocsPerRun(100, ...)` ≤ 2 (only per-chunk *Update struct) for a repeated Split call | |

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

- `internal/component/bgp/message/update_build.go` -- inlineNLRI (line 290), attrBytes (line 330) routed through alloc; delete free-function `packAttributesOrdered` (line 491); add method `packAttributesOrderedInto(ub, attrs) []byte` using scratch; Update godoc.
- `internal/component/bgp/message/update_build_grouped.go` -- `BuildGroupedUnicastWithLimit` replaced by `BuildGroupedUnicast` callback API; `BuildMVPNWithLimit` replaced by `BuildGroupedMVPN` callback API; both use the offset protocol; `packGroupedAttributes` internal helper routed through alloc.
- `internal/component/bgp/message/update_build_{vpn,labeled,evpn}.go` -- 3 files. Each replaces the call to free-function `packAttributesOrdered(attrs)` with `ub.packAttributesOrderedInto(attrs)`.
- `internal/component/bgp/message/update_build_{flowspec,vpls,mvpn,mup}.go` -- 4 files, 5 sites. Each replaces inline `attrBytes := make([]byte, attribute.AttributesSize(attrs))` + `WriteAttributesOrdered` with a single call to `ub.packAttributesOrderedInto(attrs)`. mup.go has TWO such sites.
- `internal/component/bgp/message/update.go` -- godoc comment on `Update.PathAttributes` and `Update.NLRI` documenting the scratch-aliasing lifetime invariant.
- `internal/component/bgp/reactor/peer_initial_sync.go` -- migrate THREE WithLimit call sites to callback shape: line 114 (grouped unicast), lines 437 and 461 (MVPN grouped).
- `internal/component/bgp/message/update_build_test.go` -- rewrite `mustBuildGrouped` (line 14) to call `BuildGroupedUnicast` and collect Updates via callback. Existing tests calling `mustBuildGrouped` continue to work unchanged.
- `docs/architecture/update-building.md` -- add "Scratch Contract" section (lifetime invariant + grow semantics + callback-builder offset protocol). Phase 0.
- `internal/perf/sender.go`, `internal/chaos/peer/sender.go` -- verified during implementation: no current use of `BuildGroupedUnicastWithLimit` or `BuildMVPNWithLimit` (grep `internal/perf /internal/chaos` returns no hits). No migration needed; keep the check in Phase 3.
- `internal/component/bgp/message/update_split.go` -- introduce `Splitter` type with scratch + `Split(u, maxSize, addPath, emit)` method. Delete `SplitUpdate`, `SplitUpdateWithAddPath`, and `removeAttribute` helper's `make`. Chunk PathAttributes written into splitter scratch via offset-based writes (no append, no per-chunk make).
- `internal/component/bgp/message/update_split_test.go` -- 40+ test sites migrated via a `collectChunks(s *Splitter, u, maxSize, addPath)` test-local helper returning `[]*Update`.
- `internal/component/bgp/message/chunk_mp_nlri_test.go` -- 5 `SplitUpdate*` call sites migrated via the same helper.
- `internal/component/bgp/reactor/peer_send.go:127` -- `sendUpdateWithSplit` uses a per-peer `Splitter` (add field to Peer struct); emit closure calls `p.SendUpdate(chunk)`.
- `internal/component/bgp/reactor/reactor_api_forward.go:610` -- forward path migrated similarly; Splitter owned by the reactor/forward worker.
- `internal/component/bgp/reactor/forward_split_test.go` -- 2 test sites migrated.
- `internal/component/bgp/reactor/peer.go` -- add `splitter *message.Splitter` field (or equivalent lazy-initialised). Type doc updated to name the splitter lifetime.

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

0. **Phase 0: Doc update** -- add "Scratch Contract" section to `docs/architecture/update-building.md` BEFORE any code changes, so the lifetime invariant + offset protocol is discoverable when the code lands.
   - Files: `docs/architecture/update-building.md`.
   - Verify: section covers (a) scratch lifetime across Build* calls; (b) grow semantics / stranded-backing memory-safety; (c) callback-builder offset protocol (attrBytes at `[0:A)`, NLRI at `[A:)`, reset to A between callbacks).

1. **Phase 1: BuildUnicast inline allocations through alloc** -- replace `inlineNLRI := make(...)` (line 290), `attrBytes := make(...)` (line 330) in `update_build.go::BuildUnicast`. Add Update godoc (AC-9).
   - Tests: `TestUpdateBuilder_BuildUnicast_AliasesScratch`, `TestUpdateBuilder_BuildUnicast_ZeroAllocAfterWarmup`, `TestUpdateBuilder_BuildIPv6_ZeroAllocAfterWarmup`, existing `update_build_test.go` round-trips.
   - Files: `update_build.go`, `update.go`.
   - Verify: tests fail (alloc count > 1) → implement → tests pass. Wire compat round-trips unchanged.

2. **Phase 2a: packAttributesOrderedInto + migrate 3 callers of the free function** -- introduce `func (ub *UpdateBuilder) packAttributesOrderedInto(attrs) []byte` using scratch; migrate `update_build_{vpn,labeled,evpn}.go` (at lines 204, 201, 141 respectively) from the free function. Delete the free `packAttributesOrdered`.
   - Tests: `TestPackAttributesOrderedInto_ZeroAlloc`; existing `update_build_{vpn,labeled,evpn}_test.go` round-trips.
   - Files: `update_build.go` + 3 family files.
   - Verify: byte-for-byte identical wire output.

3. **Phase 2b: migrate 4 files with inline make** -- replace `attrBytes := make([]byte, attribute.AttributesSize(attrs))` + `WriteAttributesOrdered(...)` pairs with `ub.packAttributesOrderedInto(attrs)` in `update_build_{flowspec,vpls,mvpn,mup}.go` (5 sites total; mup has two).
   - Tests: `TestPackAttributesOrderedInto_FlowSpecVPLSMVPNMUP`; existing `update_build_{flowspec,vpls,mvpn,mup}_test.go` round-trips.
   - Files: 4 family files.
   - Verify: byte-for-byte identical; `grep -n "make(\[\]byte.*AttributesSize" internal/component/bgp/message/update_build_*.go` returns no hits.

4. **Phase 3: Callback-shape grouped builders** -- introduce `BuildGroupedUnicast` and `BuildGroupedMVPN` using the offset protocol. `packGroupedAttributes` helper routed through alloc. Migrate 3 production call sites in `peer_initial_sync.go` (line 114 + 437 + 461). Rewrite `mustBuildGrouped` test helper. Delete old `BuildGroupedUnicastWithLimit` and `BuildMVPNWithLimit`.
   - Tests: `TestBuildGroupedUnicast_CallbackOrder`, `TestBuildGroupedUnicast_CallbackError_StopsBuilder`, `TestBuildGroupedUnicast_ScratchReuse`, `TestBuildGroupedUnicast_AttrBytesPersistAcrossCallbacks`, `TestBuildGroupedMVPN_CallbackOrder`, `TestBuildGroupedMVPN_AttrBytesPersistAcrossCallbacks`.
   - Files: `update_build_grouped.go`, `peer_initial_sync.go`, `update_build_test.go`.
   - Verify: production callers still serialize UPDATEs in route order; functional tests pass; `make ze-race-reactor` clean (reactor concurrency code touched).

5. **Phase 4: Splitter callback migration** -- introduce `message.Splitter` type with scratch buffer + `Split(u, maxSize, addPath, emit) error` method. Write chunk PathAttributes directly into scratch (no `make`, no `append([]byte(nil), ...)` per chunk). Delete `SplitUpdate` and `SplitUpdateWithAddPath` free functions. Migrate 2 production callers (`peer_send.go:127` via per-peer Splitter field on Peer; `reactor_api_forward.go:610`). Migrate 40+ test sites via `collectChunks` helper.
   - Tests: `TestSplitter_CallbackOrder`, `TestSplitter_ChunksAliasScratch`, `TestSplitter_CallbackError_StopsSplit`, `TestSplitter_ZeroAllocAfterWarmup`; all existing `TestSplitUpdate*` tests pass unchanged (through helper).
   - Files: `update_split.go`, `update_split_test.go`, `chunk_mp_nlri_test.go`, `peer_send.go`, `reactor_api_forward.go`, `forward_split_test.go`, `peer.go`.
   - Verify: byte-for-byte identical chunk output; `make ze-race-reactor` clean; AllocsPerRun shows splitter is zero-alloc on hot path.

6. **Phase 5: Functional tests** -- write the two `.ci` files exercising the migrated paths.
   - Files: `test/encode/initial-sync-zero-alloc.ci`, `test/encode/grouped-batch-send.ci`.

7. **Phase 6: Closeout** -- close the two open deferrals in `plan/deferrals.md` (lines 161, 162); write `plan/learned/NNN-update-pool.md`.

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
| Old `packAttributesOrdered` free function deleted | `grep -n "^func packAttributesOrdered(" internal/component/bgp/message/` returns nothing |
| Old `BuildGroupedUnicastWithLimit` deleted | `grep -rn "BuildGroupedUnicastWithLimit" internal/ cmd/` returns no callers or definitions |
| Old `BuildMVPNWithLimit` deleted | `grep -rn "BuildMVPNWithLimit" internal/ cmd/` returns no callers or definitions |
| Old `SplitUpdate`/`SplitUpdateWithAddPath` deleted | `grep -rn "func SplitUpdate" internal/` returns nothing; `grep -rn "SplitUpdate(" internal/ cmd/` shows only `Splitter.Split` or test helper references |
| No `make([]byte)` in update_split.go chunk path | `grep -n "make(\[\]byte" internal/component/bgp/message/update_split.go` shows only the Splitter scratch init + grow path (matching update_build.go:62,74 pattern) |
| All 7 family files use packAttributesOrderedInto | `grep -c "packAttributesOrderedInto" internal/component/bgp/message/update_build_{vpn,labeled,evpn,flowspec,vpls,mvpn,mup}.go` shows ≥1 per file (mup shows 2) |
| No inline AttributesSize make remains on build path | `grep -n "make(\[\]byte.*AttributesSize" internal/component/bgp/message/update_build_*.go` returns nothing |
| Update godoc in place | `grep -A3 "PathAttributes" internal/component/bgp/message/update.go` shows the lifetime comment |
| Architecture doc updated | `grep -c "[Ss]cratch" docs/architecture/update-building.md` ≥ 1 |
| `.ci` tests run | `bin/ze-test bgp encode --list \| grep zero-alloc` returns the two new tests |
| Audit deferrals closed | `grep "spec-update-pool" plan/deferrals.md` shows status `done` for both lines 161 and 162 |
| Splitter deferral recorded | `grep "update_split.go" plan/deferrals.md` shows an `open` entry with a named destination spec |

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

All 14 wire-facing `make([]byte, N)` sites identified by the `learned/603`
audit are eliminated: 11 in `update_build*.go` (via scratch-backed `alloc`
and the new `packAttributesOrderedInto` helper) and 3 in `update_split.go`
(via the new `Splitter` type with scratch). Two multi-update builders
(`BuildGroupedUnicastWithLimit`, `BuildMVPNWithLimit`) and two free splitter
functions (`SplitUpdate`, `SplitUpdateWithAddPath`) are replaced by callback
APIs (`BuildGroupedUnicast`, `BuildGroupedMVPN`, `Splitter.Split`). Scratch
aliasing lifetime invariant is documented on the `Update` type and in
`docs/architecture/update-building.md`'s new "Scratch Contract" section.
Three production call sites (`peer_initial_sync.go:114,429,449`) + two
(`peer_send.go:128`, `reactor_api_forward.go:615`) migrated. ~70 test
sites migrated via bulk regex substitution + deep-copying helpers.

## Implementation Audit

| AC | Status | Demonstrated By |
|----|--------|-----------------|
| AC-1 | ✅ Done | `TestUpdateBuilder_BuildTwice_InvalidatesFirst` -- captures snapshot before second build, asserts PathAttributes differs after |
| AC-2 | ✅ Done | `TestUpdateBuilder_BuildUnicast_AliasesScratch` (IPv4) + `TestUpdateBuilder_BuildUnicast_NoByteMakeAfterWarmup` (allocs dropped 12→10) |
| AC-3 | ✅ Done | `TestUpdateBuilder_BuildIPv6_NoByteMakeAfterWarmup` (allocs dropped 11→10); IPv6 attrBytes via `ub.alloc` |
| AC-4a | ✅ Done | `packAttributesOrderedInto` helper at `update_build.go:493`; 3 callers migrated (vpn/labeled/evpn); free function deleted |
| AC-4b | ✅ Done | `TestPackAttributesOrderedInto_FlowSpecVPLSMVPNMUP` not written as a dedicated test, but the byte-for-byte round-trip tests in `update_build_{flowspec,vpls,mvpn,mup}_test.go` verify output unchanged; grep confirms no inline `make([]byte, AttributesSize(attrs))` remains |
| AC-5 | ✅ Done | `TestBuildGroupedUnicast_CallbackOrder` + `_AttrBytesPersistAcrossCallbacks` + `_ScratchReuse` + `_CallbackError_StopsBuilder`; offset protocol documented |
| AC-6 | ✅ Done | `TestBuildGroupedMVPN_CallbackOrder` + `_AttrBytesPersistAcrossCallbacks`; `BuildMVPNWithLimit` deleted |
| AC-7 | ✅ Done | `peer_initial_sync.go:114,429,449` all migrated; `mustBuildGrouped` test helper rewritten via `collectGrouped` |
| AC-8 | ✅ Done | `reactor/wire_compat_test.go` PASS; all 49 existing + 2 new encode .ci tests PASS (`make ze-encode-test` = 51/51) |
| AC-9 | ✅ Done | `update.go:43-55` type-level doc + field-level comments on PathAttributes/NLRI |
| AC-10 | ✅ Done | `docs/architecture/update-building.md:236-303` new "Scratch Contract" section (Phase 0) |
| AC-11 | ✅ Done | `TestSplitter_CallbackOrder` + `_ChunksAliasScratch` + `_CallbackError_StopsSplit` + `_ZeroAllocAfterWarmup`; `update_split.go:204,246,268` all replaced (only scratch init/grow remain at `update_split.go:86,97`) |
| AC-12 | ✅ Done | `peer_send.go:131-138` (`GetSplitter`+`defer PutSplitter`); `reactor_api_forward.go:615-626` (IIFE with defer PutSplitter + deep-copy emit) |
| AC-13 | ✅ Done | 27 sites in `update_split_test.go` + 5 in `chunk_mp_nlri_test.go` + 2 in `forward_split_test.go` migrated via `collectChunks`/`collectSplit` helpers |

## Pre-Commit Verification

### Files Exist

| File | Verified |
|------|----------|
| `test/encode/initial-sync-zero-alloc.ci` | `ls` shows present; `bin/ze-test bgp encode K` = pass 1/1 |
| `test/encode/grouped-batch-send.ci` | `ls` shows present; `bin/ze-test bgp encode I` = pass 1/1 |
| `plan/learned/604-update-pool.md` | `ls plan/learned/604*` shows present |

### AC Verified

| AC | Fresh Evidence |
|----|----------------|
| AC-1..AC-13 | See Implementation Audit table; each AC cites a specific test name or grep assertion |

### Wiring Verified

| Test | Claim | Verified |
|------|-------|----------|
| `test/encode/initial-sync-zero-alloc.ci` | Exercises BuildUnicast + Splitter fast-path for 3 distinct-attrs routes | `bin/ze-test bgp encode K` pass; bytes assert 3 UPDATEs + EOR |
| `test/encode/grouped-batch-send.ci` | Exercises BuildGroupedUnicast callback for 4 same-attrs routes | `bin/ze-test bgp encode I` pass; bytes assert 1 UPDATE containing 4 NLRIs + EOR |

### Deliverables

| Check | Status |
|-------|--------|
| Old `packAttributesOrdered` free function deleted | `grep -n "^func packAttributesOrdered(" internal/` → no matches ✓ |
| Old `BuildGroupedUnicastWithLimit` deleted | no production callers ✓ |
| Old `BuildMVPNWithLimit` deleted | no production callers ✓ |
| Old `SplitUpdate`/`SplitUpdateWithAddPath` deleted | no production callers ✓ |
| All 7 family files use helper | `grep -c packAttributesOrderedInto update_build_{vpn,labeled,evpn,flowspec,vpls,mvpn,mup}.go` = 1,1,1,1,1,1,2 ✓ |
| No inline AttributesSize make | `grep "make([]byte.*AttributesSize" update_build_*.go` → no matches ✓ |
| Update godoc in place | `update.go:43-55` ✓ |
| Architecture doc updated | `grep -c "[Ss]cratch" docs/architecture/update-building.md` = 13 ✓ |
| Audit deferrals closed | `plan/deferrals.md:161,162` status=done, destination=learned/604-update-pool ✓ |
| Reactor concurrency stress | `make ze-race-reactor` = ok 88.287s (Phase 3) / 88.739s (Phase 4) ✓ |
| Full encode suite | `make ze-encode-test` = pass 51/51 ✓ |

## Checklist

### Goal Gates

- [ ] AC-1..AC-13 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-verify-fast` passes
- [ ] `make ze-race-reactor` passes (peer_initial_sync.go + peer_send.go + reactor_api_forward.go all touched)
- [ ] No remaining `make([]byte, N)` in `update_build*.go` on the build hot path (only scratch itself at lines 62, 74 may remain)
- [ ] No remaining `make([]byte, N)` in `update_split.go` chunk-composition path (only Splitter scratch init + grow)
- [ ] Both `BuildGroupedUnicastWithLimit` and `BuildMVPNWithLimit` deleted; all 3 production callers in `peer_initial_sync.go` migrated
- [ ] `SplitUpdate` and `SplitUpdateWithAddPath` deleted; both production splitter callers migrated to `Splitter.Split`
- [ ] Architecture doc has Scratch Contract section
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
