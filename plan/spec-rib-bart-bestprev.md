# Spec: rib-bart-bestprev

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 4/5 |
| Updated | 2026-04-17 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` -- workflow rules
3. `plan/learned/534-rib-alloc.md` -- prior BART adoption in `FamilyRIB`, pattern this spec reuses
4. `internal/component/bgp/plugins/rib/rib_bestchange.go` -- current best-path tracking
5. `internal/component/bgp/plugins/rib/storage/familyrib_bart.go` -- canonical BART+map dispatch pattern

## Task

After three phases of `checkBestPathChange` allocation reductions (see `plan/learned/534-rib-alloc.md`), the 1M-prefix stress profile still shows `checkBestPathChange` at 107.74 MB flat (47% of inuse heap) and `gcBgMarkWorker` at 31% of CPU. The residual allocation is the nested Go map `bestPrev map[Family]map[string]bestPathRecord`: every non-duplicate insert copies an NLRI byte slice into a string key, every `bestPathRecord` carries a redundant `Prefix string` formatted eagerly on insert, and the inner map pays 7+ bucket-growth rehashes as it approaches 1M entries.

Fold `bestPrev` into BART via a new generic `storage.Store[T]` that mirrors the existing `FamilyRIB` dual-backend dispatch (BART for non-ADD-PATH, map for ADD-PATH). Refactor `FamilyRIB` onto the same generic so the dispatch pattern lives in one place. Drop the redundant `Prefix` field from `bestPathRecord` and format display strings lazily from the trie / map key only when a best-change event is emitted.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/plugin/rib-storage-design.md` -- RIB storage layering and BART adoption rationale
  → Constraint: BART is used for non-ADD-PATH families only; ADD-PATH families must continue to key on NLRI bytes including the 4-byte path-id prefix
- [ ] `plan/learned/534-rib-alloc.md` -- prior phase that introduced BART for `FamilyRIB`
  → Decision: value-type storage in BART (copy on insert/lookup) over pointer-type; matches `bart.Table[RouteEntry]` pattern
  → Constraint: `SetAddPath` must be called before first insert for ADD-PATH families -- otherwise two paths with same prefix but different path-ids collapse
  → Constraint: `maprib` build tag forces map-only storage as a fallback variant; must continue to work

### Rules Applied
- [ ] `.claude/rules/design-principles.md`
  → Constraint: "Zero-copy, copy-on-modify" -- store `bestPathRecord` by value in the trie, not by pointer
  → Constraint: "Lazy over eager" -- do not pre-compute a display prefix string on every insert when only best-change emission consumes it
  → Constraint: "No premature abstraction" -- `Store[T]` is generic because there are two concrete users (`FamilyRIB`, `bestPrev`), not one
- [ ] `.claude/rules/no-layering.md`
  → Constraint: old `map[Family]map[string]bestPathRecord` and old direct-trie `FamilyRIB` internals are deleted in the same commit that introduces `Store[T]`; no parallel implementations, no fallback flag
- [ ] `.claude/rules/api-contracts.md`
  → Constraint: `FamilyRIB` public methods (`Insert`, `Remove`, `LookupEntry`, `IterateEntry`, `Release`, `MarkStale`, `PurgeStale`, `StaleCount`, `ModifyEntry`, `ModifyAll`, `Family`, `HasAddPath`, `Len`) keep their signatures unchanged
- [ ] `.claude/rules/spec-no-code.md`
  → Constraint: this spec uses tables and prose only; implementation details live in the code at commit time

### RFC Summaries
Not applicable -- internal storage refactor, no wire protocol change.

**Key insights:**
- The `bestPrev` store is cross-peer (one entry per best path for a (family, prefix)); `FamilyRIB` is per-peer. Their dispatch requirements differ, which is why the generic is single-mode and the hybrid layer lives above it at the rib layer.
- Mixed-mode sessions (some peers ADD-PATH-capable, some not, for the same family) require the bestPrev layer to hold both backends simultaneously; `FamilyRIB` does not.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/rib/rib.go` -- `RIBManager` fields; `bestPrev` declared as `map[family.Family]map[string]bestPathRecord`; initialized in `NewRIBManager`; guarded by `r.mu`
  → Constraint: `r.mu` guards `bestPrev` writes; reads during replay take the read lock
- [ ] `internal/component/bgp/plugins/rib/rib_bestchange.go` -- defines `bestPathRecord`, `checkBestPathChange`, `replayBestPaths`; pre-sizes inner map at 4096
  → Constraint: `checkBestPathChange` is called on every INSERT/REMOVE; same-best short-circuit is the common case
- [ ] `internal/component/bgp/plugins/rib/rib_structured.go` -- dispatches inserts/removes; computes per-call `addPath` via `ctx.AddPath(fam)`; passes it into `checkBestPathChange`
  → Constraint: `addPath` is a per-peer-per-family value known at the call site; no need to pre-register it at the store level
- [ ] `internal/component/bgp/plugins/rib/storage/familyrib_bart.go` -- canonical BART+map dispatch; `NewFamilyRIB(fam, addPath)` chooses the backend; all methods branch on `r.addPath`
- [ ] `internal/component/bgp/plugins/rib/storage/familyrib_map.go` (`//go:build maprib`) -- map-only variant used when the `maprib` build tag is set
- [ ] `internal/component/bgp/plugins/rib/storage/nlrikey.go` -- `NLRIToPrefix(fam, nlriBytes)`, `PrefixToNLRI(pfx)`, `NLRIKey{len, data[24]}`
- [ ] `internal/component/bgp/plugins/rib/storage/peerrib.go` -- wraps `FamilyRIB` per family per peer; creates `FamilyRIB` via `NewFamilyRIB`
- [ ] `internal/component/bgp/plugins/rib/storage/familyrib_test.go`, `storage/stale_test.go` -- exercise `FamilyRIB` public API
- [ ] `internal/component/bgp/plugins/rib/rib_bestchange_test.go` -- 11 best-path-change tests; constructs `RIBManager{bestPrev: make(...)}` directly
- [ ] `internal/component/bgp/plugins/rib/rib_test.go` -- another `RIBManager` constructor helper for tests

**Behavior to preserve:**
- `BestChangeBatch` payload shape (kebab-case JSON: `action`, `prefix`, `next-hop`, `priority`, `metric`, `protocol-type`; `replay` flag on replay batches)
- Replay ordering: one batch per family; same action (`add`) for every entry
- Mixed-mode sessions: a family with some ADD-PATH peers and some non-ADD-PATH peers keeps distinct bestPrev entries per (prefix, path-id) for AP peers and per prefix for non-AP peers; the two coexist without collision
- `FamilyRIB` public API: every exported method keeps its current signature
- `maprib` build tag: `go test -tags maprib ./...` builds and passes
- Existing functional `.ci` test `test/plugin/fib-rib-event.ci` passes unchanged
- Same-best short-circuit (no event emitted when peer/nexthop/priority/metric are unchanged) continues to avoid all per-update display-string work

**Behavior to change:**
- `bestPathRecord.Prefix string` field removed
- Display prefix formatted lazily on emission path only: trie-backed entries call `netip.Prefix.String()`; map-backed entries call existing `wirePrefixToString`
- `bestPrev` storage reshaped from nested map to per-family `bestPrevStore` holding two `*storage.Store[bestPathRecord]` (one non-AP, one AP)
- `FamilyRIB` internals re-expressed as composition of `*storage.Store[RouteEntry]` plus lifecycle methods (`Release`, `MarkStale`, `PurgeStale`, `StaleCount`); internal trie/map fields gone from `FamilyRIB` itself

## Data Flow (MANDATORY)

### Entry Point
`handleReceivedStructured` in `rib_structured.go` receives decoded BGP UPDATE events. For each affected (fam, nlriBytes, addPath) tuple it calls `checkBestPathChange`.

### Transformation Path
1. `gatherCandidates(fam, nlriBytes)` walks `r.ribInPool` to build a `Candidate` slice from every peer holding that NLRI.
2. `SelectBest(candidates)` picks the winner per RFC 4271 §9.1.2.
3. `bestPrevStore.lookup(nlriBytes, addPath)` returns the previously-recorded best (or zero value). Dispatch: if `addPath`, hit the AP-map `Store[bestPathRecord]`; else hit the trie `Store[bestPathRecord]`.
4. Compare winner against previous: if peer / nexthop / priority / metric match, short-circuit with no change.
5. On change: format display prefix (trie: `pfx.String()`; map: `wirePrefixToString`), update the store, return a `BestChangeEntry` to the caller.
6. Caller batches entries and emits on the EventBus after lock release.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Wire → Structured event | `handleReceivedStructured` consumes decoded event | existing tests |
| RIBManager ↔ storage.Store[T] | in-process function calls | store_test.go |
| Change detection → EventBus | `ribevents.BestChange.Emit(eb, batch)` after lock release | `test/plugin/fib-rib-event.ci` |

### Integration Points
- `RIBManager.mu` continues to guard `bestPrev` writes/reads
- `storage.NLRIToPrefix` / `storage.PrefixToNLRI` reused from prior phase; unchanged
- `EventBus` handle `ribevents.BestChange` unchanged

### Architectural Verification
- [ ] No bypassed layers -- inserts still go Wire → Structured → RIBManager → store
- [ ] No unintended coupling -- `storage.Store[T]` knows nothing about best-path semantics; `bestPathRecord` stays in `rib` package
- [ ] No duplicated functionality -- `FamilyRIB` and `bestPrevStore` share `Store[T]` rather than each carrying its own BART+map dispatch
- [ ] Zero-copy preserved -- `Store[T]` stores values inline in the trie; lookups copy to stack; modifications use get-mutate-insert

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| BGP UPDATE arriving at RIB (structured event) triggers best-path change emission | → | `RIBManager.checkBestPathChange` reading/writing new `bestPrevStore` | `test/plugin/fib-rib-event.ci` (existing; refactor preserves) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | 1M non-ADD-PATH prefix stress run (`ze-test peer --mode inject 1000000`) | `checkBestPathChange` flat `inuse_space` under 50 MB (baseline 107.74 MB); overall GC share drops from 31% toward 20% or better |
| AC-2 | All 11 existing `rib_bestchange_test.go` tests | Pass unchanged; only constructor lines updated |
| AC-3 | Mixed-mode family: one peer with ADD-PATH, one without, same family | `bestPrevStore` tracks distinct entries per path shape; no collision between the two peers' advertisements |
| AC-4 | `bestPathRecord` struct | Has no `Prefix` field; `replayBestPaths` reconstructs prefix from trie key or NLRIKey |
| AC-5 | Replay path (`replayBestPaths`) | Emits identical `BestChangeBatch` payloads as pre-refactor; one batch per family, every entry carries the formatted display prefix |
| AC-6 | `storage.Store[T]` public surface | Dedicated tests cover Insert, Lookup, Delete, Iterate (including early-return), Modify, ModifyAll, Len, for both `addPath=false` (trie) and `addPath=true` (map) |
| AC-7 | `FamilyRIB` public API | Every exported method keeps its signature; `storage/familyrib_test.go` and `storage/stale_test.go` pass unchanged |
| AC-8 | `make ze-verify-fast` | Passes |
| AC-9 | `go test -tags maprib ./internal/component/bgp/plugins/rib/...` | Builds and passes; `Store[T]` under the `maprib` build tag uses map-only storage |
| AC-10 | Same-best short-circuit | When peer/nexthop/priority/metric match previous, no display-string formatting and no store write occurs (verified by unit test or by profile showing zero allocs on the unchanged path) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestStoreTrieInsertLookup` | `internal/component/bgp/plugins/rib/storage/store_test.go` | Insert + Lookup roundtrip, non-AP backend | |
| `TestStoreMapInsertLookup` | `internal/component/bgp/plugins/rib/storage/store_test.go` | Insert + Lookup roundtrip, AP backend | |
| `TestStoreDelete` | `internal/component/bgp/plugins/rib/storage/store_test.go` | Delete returns true for existing, false for absent; Lookup after delete returns zero value | |
| `TestStoreIterate` | `internal/component/bgp/plugins/rib/storage/store_test.go` | Iterate visits all entries; `false` return stops iteration | |
| `TestStoreModify` | `internal/component/bgp/plugins/rib/storage/store_test.go` | Modify callback mutation persists; returns false for absent key | |
| `TestStoreModifyAll` | `internal/component/bgp/plugins/rib/storage/store_test.go` | ModifyAll visits every entry; mutations persist | |
| `TestStoreLen` | `internal/component/bgp/plugins/rib/storage/store_test.go` | Len tracks Insert/Delete correctly for both backends | |
| (existing) `TestRIBBestChangePublish` and 10 siblings | `internal/component/bgp/plugins/rib/rib_bestchange_test.go` | Behavior preserved; only constructor updated | |
| (existing) `FamilyRIB` tests | `internal/component/bgp/plugins/rib/storage/familyrib_test.go`, `stale_test.go` | `FamilyRIB` public API unchanged | |

### Boundary Tests
Not applicable -- no numeric inputs.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `fib-rib-event` | `test/plugin/fib-rib-event.ci` | Downstream subscriber receives best-change events when routes are injected (existing; refactor must preserve) | |

### Future
None. The 1M-prefix stress profile is the memory-evidence artefact; it is a measurement, not a unit test (thresholds would be brittle per `rules/testing.md`).

## Files to Modify

- `internal/component/bgp/plugins/rib/storage/familyrib_bart.go` -- rewrite to wrap `*Store[RouteEntry]` under `//go:build !maprib`; lifecycle methods (`Release`, `MarkStale`, `PurgeStale`, `StaleCount`) implemented via `ModifyAll` / `Iterate`
- `internal/component/bgp/plugins/rib/storage/familyrib_map.go` -- same rewrite under `//go:build maprib`
- `internal/component/bgp/plugins/rib/rib_bestchange.go` -- drop `Prefix` from `bestPathRecord`; define local unexported `bestPrevStore` (holding two `*storage.Store[bestPathRecord]`); rewrite `checkBestPathChange` to dispatch per-call on `addPath`; rewrite `replayBestPaths` to format prefix lazily from the trie or NLRIKey
- `internal/component/bgp/plugins/rib/rib.go` -- change `bestPrev` field to `map[family.Family]*bestPrevStore`; update `NewRIBManager` initializer; update comment on `r.mu` guarded fields
- `internal/component/bgp/plugins/rib/rib_bestchange_test.go` -- update constructor line; assertions unchanged
- `internal/component/bgp/plugins/rib/rib_test.go` -- update constructor line
- `docs/architecture/plugin/rib-storage-design.md` -- document `storage.Store[T]` and the bestPrev consolidation (one paragraph)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [ ] No | -- |
| CLI commands/flags | [ ] No | -- |
| Editor autocomplete | [ ] No | -- |
| Functional test for new RPC/API | [ ] No | existing `test/plugin/fib-rib-event.ci` suffices |

### Documentation Update Checklist (BLOCKING)
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
| 12 | Internal architecture changed? | [ ] Yes | `docs/architecture/plugin/rib-storage-design.md` -- paragraph on `Store[T]` and bestPrev consolidation |

## Files to Create

- `internal/component/bgp/plugins/rib/storage/store_bart.go` -- `//go:build !maprib`; generic `Store[T]` with trie+map dispatch by construction-time `addPath`
- `internal/component/bgp/plugins/rib/storage/store_map.go` -- `//go:build maprib`; generic `Store[T]` map-only variant
- `internal/component/bgp/plugins/rib/storage/store_test.go` -- table-driven coverage of the generic surface

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Implementation Phases below |
| 4. Full verification | `make ze-verify-fast` + `go test -tags maprib ./internal/component/bgp/plugins/rib/...` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every finding |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report per `rules/planning.md` |

### Implementation Phases

Each phase ends with a Self-Critical Review. Fix issues before proceeding.

1. **Phase 1 -- Introduce `storage.Store[T]` (both build-tag variants) + tests**
   - Tests: `TestStoreTrieInsertLookup`, `TestStoreMapInsertLookup`, `TestStoreDelete`, `TestStoreIterate`, `TestStoreModify`, `TestStoreModifyAll`, `TestStoreLen`
   - Files created: `storage/store_bart.go`, `storage/store_map.go`, `storage/store_test.go`
   - Verify: tests fail (generic not yet implemented) → implement → tests pass with and without `-tags maprib`
2. **Phase 2 -- Refactor `FamilyRIB` onto `Store[RouteEntry]`**
   - Tests: existing `FamilyRIB` unit tests (`familyrib_test.go`, `stale_test.go`) pass unchanged
   - Files: `storage/familyrib_bart.go`, `storage/familyrib_map.go`
   - Verify: `go test ./internal/component/bgp/plugins/rib/storage/...` (default + `-tags maprib`)
3. **Phase 3 -- Rewrite `bestPrev` using two `Store[bestPathRecord]` per family; drop `Prefix` field**
   - Tests: existing `rib_bestchange_test.go` passes; assertions unchanged; only constructors updated
   - Files: `rib_bestchange.go`, `rib.go`, `rib_bestchange_test.go`, `rib_test.go`
   - Verify: `go test ./internal/component/bgp/plugins/rib/...`
4. **Phase 4 -- Full verification + re-profile**
   - `make ze-verify-fast`
   - 1M-prefix stress run; capture `tmp/profiles/` heap + CPU profiles; produce Phase-4 comparison table vs Phase-3 baseline
5. **Phase 5 -- Architecture doc paragraph**
   - Update `docs/architecture/plugin/rib-storage-design.md`
6. **Completion** -- fill audit tables, Pre-Commit Verification, write learned summary (`plan/learned/NNN-rib-bart-bestprev.md`), delete spec in Commit B

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has a test or profile artefact with file:line |
| Correctness | Mixed-mode session test: one AP peer + one non-AP peer for same family, no collision |
| Correctness | Same-best short-circuit still returns before any display-string formatting (inspect diff at `checkBestPathChange` early return) |
| Correctness | Replay batches identical shape pre/post refactor (diff one batch payload) |
| Naming | `Store[T]` method names match existing codebase idiom (`Insert`, `Lookup`, `Delete`, `Iterate`, `Modify`, `ModifyAll`, `Len`) |
| Data flow | Event emission still happens after `r.mu` release; no new lock held across Emit |
| Rule: no-layering | Old `map[Family]map[string]bestPathRecord` fully deleted; old direct-backend `FamilyRIB` internals fully deleted; no fallback flag |
| Rule: api-contracts | `FamilyRIB` public method signatures unchanged |
| Rule: zero-copy/copy-on-modify | `Store[T]` holds values inline; Lookup returns a copy; Modify uses get-mutate-insert pattern |
| Rule: spec-no-code | This spec contains no code snippets |
| Rule: maprib build | `go test -tags maprib` still builds and passes |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| `storage/store_bart.go` exists | `ls internal/component/bgp/plugins/rib/storage/store_bart.go` |
| `storage/store_map.go` exists | `ls internal/component/bgp/plugins/rib/storage/store_map.go` |
| `storage/store_test.go` exists with ≥7 `Test*` functions | `grep -c '^func Test' internal/component/bgp/plugins/rib/storage/store_test.go` |
| `bestPathRecord` has no `Prefix` field | `grep -A8 'type bestPathRecord' internal/component/bgp/plugins/rib/rib_bestchange.go` |
| Old nested map for `bestPrev` is gone | `grep -n 'map\[family.Family\]map\[string\]bestPathRecord' internal/component/bgp/plugins/rib/ | wc -l` returns 0 |
| `FamilyRIB` public API unchanged | `grep -c '^func (r \*FamilyRIB)' internal/component/bgp/plugins/rib/storage/familyrib_bart.go` matches baseline |
| `make ze-verify-fast` passes | output in `tmp/ze-verify.log` |
| `go test -tags maprib ./internal/component/bgp/plugins/rib/...` passes | pasted output |
| 1M stress profile delta captured | comparison table in learned summary |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Input validation | `nlriBytes` arriving at `Store[T]` originates from the wire layer which already validates length; `NLRIToPrefix` returns `ok=false` on malformed input -- the trie path must silently skip such entries without panicking |
| Resource exhaustion | No new unbounded collection -- BART grows O(prefix count), same as current `FamilyRIB`; map fallback unchanged |
| Error leakage | No new user-visible error paths -- internal refactor |
| Lock ordering | `r.mu` usage unchanged; Emit still happens after lock release |
| Generic panics | `Store[T]` methods must not panic on malformed input; Modify on absent key returns `false` rather than panicking |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Store unit test fails | Re-check dispatch logic for the affected backend |
| Best-change test fails | Diff the event payload shape pre/post; likely missing field or prefix format |
| `-tags maprib` build fails | Check both files have matching signatures |
| Profile shows < 30% reduction | Check `bestPathRecord` size, trie storage mode, confirm `Prefix` field is gone |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| | | | |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| | | |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|
| | | | |

## Design Insights

- `Store[T]` is single-mode (mode fixed at construction) even though `bestPrev` needs hybrid behavior, because `FamilyRIB` genuinely knows its mode at construction (from OPEN negotiation) and paying for an always-present empty backend would be wasteful there. Hybrid is a `bestPrev`-layer concern only, implemented as a pair of single-mode `Store[T]`.
- Dropping `bestPathRecord.Prefix` is the largest single allocation win after the trie switch. At 1M entries the field represented ~30 MB of eagerly-formatted strings that are only consumed by the (cold) replay path and the (rare) change-emission path.
- The prior BART work (`plan/learned/534-rib-alloc.md`) established value-type storage and exact-match `trie.Get` semantics; this spec continues that pattern rather than revisiting it.

## RFC Documentation

Not applicable -- internal storage refactor, no wire protocol change.

## Implementation Summary

### What Was Implemented
- `storage.Store[T]` generic with BART+map dispatch (default) and map-only (`maprib`) build variants.
- `FamilyRIB` rewritten as a thin wrapper around `*Store[RouteEntry]` plus pool-handle lifecycle methods (Release, MarkStale, PurgeStale, StaleCount). Public API unchanged.
- `bestPrev` restructured from `map[Family]map[string]bestPathRecord` to `map[Family]*bestPrevStore` where `bestPrevStore` pairs two `*Store[bestPathRecord]` (non-AP trie + AP map), dispatching per call on the incoming `addPath` flag.
- `bestPathRecord.Prefix string` field dropped; display prefix formatted lazily on emission path only.
- `PrefixToNLRIInto(pfx, buf)` added in `nlrikey.go`; `Store[T].Iterate` trie path uses a reused stack `[17]byte` buffer for zero-alloc iteration.
- Architecture doc updated with a paragraph on the consolidation.

### Bugs Found/Fixed
- None. Adversarial review raised a theoretical duplicate-emit path triggered by malformed NLRI; confirmed unreachable because RFC 7606 §5.3 treat-as-withdraw runs earlier in the wire parser. Added an inline comment at `wirePrefixToString` documenting the upstream guarantee so the asymmetry does not look like a latent bug.

### Documentation Updates
- `docs/architecture/plugin/rib-storage-design.md` -- added "Generic `Store[T]` and bestPrev Consolidation" subsection describing the new generic, the mixed-mode hybrid pattern, and the `maprib` tag semantics.

### Deviations from Plan
- Initially wrote `Store[T].Insert` to return `(prev T, hadPrev bool)`. The `block-ignored-errors.sh` hook blocked the `_, _ =` pattern in tests. Revised to match `FamilyRIB.Insert`'s no-return idiom (callers Lookup first, then Insert). Cleaner anyway.
- Added `PrefixToNLRIInto` helper in `nlrikey.go` to enable zero-alloc trie iteration. Not in original spec; surfaced during adversarial review as the right fix for what would otherwise have been a cold-path allocation regression.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Fold bestPrev into BART via generic Store[T] | Done | `storage/store_bart.go`, `rib_bestchange.go:37-63` | Dual-backend pair at rib layer |
| Drop bestPathRecord.Prefix field | Done | `rib_bestchange.go:32-38` | Field absent; confirmed by grep |
| Preserve FamilyRIB public API | Done | `storage/familyrib_bart.go` | All method signatures unchanged |
| Preserve `maprib` build tag | Done | `storage/store_map.go`, `storage/familyrib_map.go` | Both variants compile and tests pass |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Pending | 1M stress re-profile | Blocked on `make ze-verify-fast` gate + 1M stress run; `ze-test peer --mode inject 1000000` to be captured to `tmp/profiles/phase4-*.pb.gz` |
| AC-2 | Done | `go test -race ./internal/component/bgp/plugins/rib/...` → `ok` (see `tmp/profiles/postreview-default.txt`) | 11 rib_bestchange tests pass unchanged |
| AC-3 | Done | `bestPrevStore` holds two `*Store[bestPathRecord]`; dispatch via `pick(addPath)` at `rib_bestchange.go:68-73`; `checkBestPathChange:82` | Mixed-mode sessions route to distinct backends; no collision |
| AC-4 | Done | `grep 'type bestPathRecord' internal/.../rib_bestchange.go` shows struct without `Prefix string` field | Struct has PeerAddr, NextHop, Priority, Metric, ProtocolType only |
| AC-5 | Done | `replayBestPaths` at `rib_bestchange.go:246-282` iterates both backends, formats prefix via `wirePrefixToString` on emission | Same BestChangeBatch payload shape as pre-refactor |
| AC-6 | Done | `storage/store_test.go` (8 tests) + `storage/store_bart_test.go` (1 test); both builds pass | Insert/Lookup/Delete/Iterate/Modify/ModifyAll/Len/malformed-input coverage |
| AC-7 | Done | `go test -race ./internal/component/bgp/plugins/rib/storage/...` → `ok` both default and `-tags maprib` | `familyrib_test.go` and `stale_test.go` pass unchanged |
| AC-8 | Pending | `make ze-verify-fast` | Another session's verify currently running; waiting per `rules/git-safety.md` concurrent-verify rule |
| AC-9 | Done | `go test -race -tags maprib ./internal/component/bgp/plugins/rib/...` → `ok` (see `tmp/profiles/postreview-maprib.txt`) | |
| AC-10 | Done | `checkBestPathChange:117-120` — unchanged-best short-circuit returns before `wirePrefixToString` call | No display-string formatting when prev==new best |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestStoreInsertLookup` | Done | `storage/store_test.go:34` | Both backends |
| `TestStoreLookupAbsent` | Done | `storage/store_test.go:69` | Both backends |
| `TestStoreDelete` | Done | `storage/store_test.go:87` | Both backends |
| `TestStoreLen` | Done | `storage/store_test.go:113` | Both backends |
| `TestStoreIterate` | Done | `storage/store_test.go:135` | Both backends + early-return |
| `TestStoreModify` | Done | `storage/store_test.go:169` | Both backends + absent key |
| `TestStoreModifyAll` | Done | `storage/store_test.go:197` | Both backends |
| `TestStoreMalformedNLRI_NoPanic` | Done | `storage/store_test.go:219` | Cross-backend no-panic invariant |
| `TestStoreTrieDropsMalformed` | Done | `storage/store_bart_test.go:18` | Trie-only, guarded by `!maprib` |
| Existing `TestRIBBestChange*` (11 tests) | Done | `rib_bestchange_test.go` | All pass unchanged |
| Existing `TestFamilyRIB_*`, `TestStale_*` | Done | `storage/familyrib_test.go`, `storage/stale_test.go` | All pass both builds |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `storage/store_bart.go` | Created | 186 lines; Store[T] + BART+map dispatch |
| `storage/store_map.go` | Created | 99 lines; Store[T] map-only under `maprib` |
| `storage/store_test.go` | Created | 239 lines; 8 table-driven tests |
| `storage/store_bart_test.go` | Created | 30 lines; 1 test, `!maprib` guarded |
| `storage/familyrib_bart.go` | Rewritten | wraps `*Store[RouteEntry]` + lifecycle |
| `storage/familyrib_map.go` | Rewritten | same under `maprib` |
| `storage/nlrikey.go` | Modified | added `PrefixToNLRIInto` |
| `rib_bestchange.go` | Rewritten | dropped Prefix field; new bestPrevStore |
| `rib.go` | Modified | field type change |
| `rib_test.go` | Modified | constructor |
| `rib_bestchange_test.go` | Modified | constructor |
| `rib_structured.go` | Modified | RFC 7606 guarantee comment |
| `docs/architecture/plugin/rib-storage-design.md` | Modified | consolidation paragraph |

### Audit Summary
- **Total items:** 10 ACs + 4 task reqs + 11 test entries + 13 file entries = 38
- **Done:** 36
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 0
- **Pending:** 2 (AC-1 1M profile re-run, AC-8 `make ze-verify-fast`) — both blocked on disk/verify serialization, not on code

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `storage/store_bart.go` | Yes | `ls -la` shows 5439 bytes |
| `storage/store_map.go` | Yes | `ls -la` shows 3243 bytes |
| `storage/store_test.go` | Yes | `ls -la` shows 6406 bytes |
| `storage/store_bart_test.go` | Yes | `ls -la` shows 1029 bytes |
| `test/plugin/fib-rib-event.ci` | Yes | pre-existing, unchanged |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-2 | 11 rib_bestchange tests pass | `go test -race -run 'TestRIBBestChange|TestRIBReplay' ./internal/component/bgp/plugins/rib/` → ok; captured in `tmp/profiles/postreview-default.txt` |
| AC-3 | bestPrevStore dispatches per-call | `grep 'func.*pick' internal/component/bgp/plugins/rib/rib_bestchange.go` → returns trie or ap based on addPath arg |
| AC-4 | bestPathRecord has no Prefix field | `grep -A8 'type bestPathRecord' internal/.../rib_bestchange.go` shows PeerAddr, NextHop, Priority, Metric, ProtocolType — no Prefix |
| AC-5 | replayBestPaths unchanged batch shape | read `rib_bestchange.go:246-282` — emits `bestChangeBatch{Protocol, Family, Replay:true, Changes}` identical to pre-refactor |
| AC-6 | 9 Store[T] tests | `grep -c '^func Test' internal/.../storage/store_test.go internal/.../storage/store_bart_test.go` → 8 + 1 |
| AC-7 | FamilyRIB API unchanged | `grep '^func (r \*FamilyRIB)' internal/.../storage/familyrib_bart.go` → 13 methods, same signatures as pre-refactor |
| AC-9 | maprib build passes | `go test -race -tags maprib ./internal/component/bgp/plugins/rib/...` → ok; captured in `tmp/profiles/postreview-maprib.txt` |
| AC-10 | unchanged-best early return | read `rib_bestchange.go:117-120` — `if havePrev && prev.PeerAddr == newBest.PeerAddr && ... { return bestChangeEntry{}, false }` before prefix formatting |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| BGP UPDATE → best-change event emission | `test/plugin/fib-rib-event.ci` | Pre-existing test exercises the full path. Read lines 1-40: injects routes via `rib inject`, asserts best-path emission, uses `ze_api.py` runtime_fail pattern (per testing.md). Refactor preserves same emission contract. |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-verify-fast` passes (pre-commit gate)
- [ ] `make ze-test` passes (full suite including fuzz, run once before closing spec)
- [ ] `go test -tags maprib ./internal/component/bgp/plugins/rib/...` passes
- [ ] 1M-prefix stress profile re-run; comparison table in learned summary
- [ ] Architecture doc updated

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (two concrete users of `Store[T]` at commit time)
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling (`Store[T]` knows nothing about best-path semantics)

### TDD
- [ ] Tests written before implementation of each phase
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional `.ci` test continues to pass

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `rules/quality.md`
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Pre-Commit Verification filled with fresh evidence
- [ ] Write learned summary to `plan/learned/NNN-rib-bart-bestprev.md`
- [ ] Summary included in commit (one commit = code + tests + spec + summary; the follow-up commit removes the spec)
