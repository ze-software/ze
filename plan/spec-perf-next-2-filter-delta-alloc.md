# Spec: perf-next-2-filter-delta-alloc -- Allocation Reduction in the Filter-Delta Modify Path

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-perf-next-0-umbrella.md |
| Phase | 5/5 |
| Updated | 2026-07-22 |

Awaiting closure (recorded 2026-07-22 during plan review): Phase A landed --
`filterAttrID`/`filterAttrs` (fixed struct + bitset replacing
`map[string]string`) at `internal/component/bgp/reactor/filter_chain.go:28,79`,
per `plan/learned/900-perf-next-round-3.md`. Phase B (pooled scratch for the 14
encoder sites) was deliberately deferred in learned 900; at closure, home that
deferral in a `plan/deferrals/` shard with a destination spec so it is not
lost. Only the two-commit closure (plus that deferral row) remains.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/bgp/reactor/filter_delta.go`, `filter_chain.go` (parseFilterAttrs), `filter_delta_handlers.go`
4. `plan/learned/875-filter-delta-parse-once.md` (prior optimization of the same path)

## Task

When an external policy filter modifies an UPDATE, the reactor converts the
filter's text delta into wire attribute operations. This path currently costs
**~24 allocs per modified UPDATE** (measured by BenchmarkFilterModifyEgress
after spec filter-delta-parse-once reduced it from 34, see
`plan/learned/875-filter-delta-parse-once.md`). It fires:

- Import: per received UPDATE when `peer.settings.ImportFilters` is non-empty AND the filter changed the text (`reactor_notify.go:460-504`).
- Export: per DESTINATION PEER per forwarded UPDATE when export filters are configured and modify the text (`reactor_api_forward.go:475-508`). The export path multiplies the cost by fan-out, making it the valuable half.

The unmodified path is already 0 allocs/op (BenchmarkFilterDispatch_ZeroAlloc).
This spec attacks the modified path's two allocation families:

1. **Parse maps:** `parseFilterAttrs` (`filter_chain.go:118`) allocates a `map[string]string` plus a `strings.Fields` slice plus joined value strings, twice per modified UPDATE (original + modified text). The key set is CLOSED: 16 names defined by `isPolicyAttrName` (`filter_chain.go:167-178`) + `policySingleToken` (`filter_chain.go:103-108`).
2. **Encoder buffers:** every `encode*Value` helper allocates its own `make` (filter_delta.go lines 80, 338, 356, 395, 414, 428, 446, 496, 505, 556, 629, 645, 651, 654), violating `ai/rules/buffer-first.md` (helpers must write into a caller-provided buffer).

**Goal:** cut the modified-UPDATE path to roughly 10 allocs/op or fewer without
changing any produced wire bytes, op sequences, or filter text contracts.

### Design (chosen, two phases)

**Phase A: struct parse output.** Replace the `map[string]string` return of
`parseFilterAttrs` with a fixed struct of 16 string fields (origin, as-path,
next-hop, med, local-preference, atomic-aggregate, aggregator, community,
originator-id, cluster-list, extended-community, aigp, large-community,
as-path-prepend, remove-private, nlri) plus presence tracking (an empty-string
ambiguity exists for atomic-aggregate, which is stored with empty value today,
so presence must be explicit, e.g. a bitset field). The three consumers
(`textDeltaToModOps`, `ExtractRemovePrivateASOps`, `ExtractASPathPrependOps`)
and `extractLegacyNLRIOverride` switch to struct field access. Saves the two map
bucket allocations and all key hashing; field strings still alias `strings.Fields`
output as today. This also satisfies `ai/rules/enum-over-string.md` for this path.

**Phase B: encoder scratch.** Give the encode helpers a caller-provided scratch
that lives exactly as long as the ops do. The ops' `Buf` slices
(`filterapi.AttrOp.Buf`) are consumed synchronously by `buildModifiedPayload`
and discarded with the `ModAccumulator` (stack-allocated at both call sites:
`reactor_notify.go:480`, `reactor_api_forward.go:491`), so a single pooled
scratch buffer per modify block can back ALL encoded values: each encoder
carves its value bytes from the scratch (append-and-slice), and the scratch is
released after `buildModifiedPayload` returns. Sized at 4096 with bounded grow,
mirroring the existing `modBufPool` pattern in `forward_build.go:363-379`.
This removes ~6-10 makes per modified UPDATE and brings the helpers into
buffer-first compliance.

**Scratch invariant (load-bearing, must be a code comment + a test):** within one
modify block the scratch is APPEND-ONLY. Carving always advances the offset; it
never rewinds or reuses space over an earlier op's region, and it is reset only on
Release after `buildModifiedPayload` returns. This is required because handlers
hold several carved `Buf` slices live simultaneously: `filter_delta_handlers.go`
accumulates `prependBufs = append(prependBufs, ops[i].Buf)` across multiple ops
(lines 143, 385) and selects `setBuf = ops[i].Buf` (lines 145, 246, 405) before
consuming them. Those slices are non-overlapping sub-slices of the same backing
array and stay valid for reads as long as nothing rewinds over them. If the
scratch grows mid-block, earlier slices orphan onto the old array — still correct
via GC, but the alloc win is silently lost, so 4096 must cover the common case and
the grow path is a correctness fallback, not a normal path. A rewind/reuse of
scratch space within a block would corrupt the held prepend slices and is
forbidden.

Out of scope: `PolicyFilterChain` RPC cost (sanctioned external-plugin
serialization boundary per `ai/rules/memory-architecture.md`), the unmodified
fast path (already zero-alloc), and `rewritePrivateASSegments` semantic changes
(its slices move to the scratch arena but its logic is untouched).

### Why this is safe

- Lifetime: ops never escape the synchronous modify block. `buildModifiedPayload` copies attribute bytes into the output payload buffer (per-peer pool or modBufPool); after it returns, no reference to the scratch remains. Validate during audit (A-1).
- The dry-run path (`policy_dryrun.go:224`) uses the same extractors; it must adopt the same struct/scratch signatures (it is cold, correctness only).
- Golden-corpus equivalence already exists: `TestFilterDeltaParseOnceEquivalence` compares op multisets across refactors; it gates Phase A. `TestFilterDeltaParseCallCount` (atomic counter `parseFilterAttrsCalls`) keeps the 2-parses-per-modify invariant.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/buffer-first.md` - banned `make` in encoding helpers
  → Constraint: encoders must write into caller-provided buffers; this spec brings filter_delta.go into compliance
- [ ] `ai/rules/memory-architecture.md` - sanctioned copies + sync.Pool guidance
  → Constraint: filter modify IS a sanctioned copy boundary; the goal is fewer allocations, not zero copies
- [ ] `ai/rules/enum-over-string.md` - string keys on hot paths
  → Decision: closed 16-name key set becomes struct fields (typed identity)
- [ ] `plan/learned/875-filter-delta-parse-once.md` - prior round on this exact path
  → Decision: parse exactly twice per modify; extractors share read-only; preserve the call-count test
- [ ] `plan/learned/859-perf-hot-alloc-reduction.md`
  → Decision: value-type struct keys over interning; same principle applies to parse output

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4271.md` - attribute wire formats the encoders produce
  → Constraint: encoded value bytes must remain byte-identical (AS_PATH segments, communities, aggregator, etc.)

**Key insights:**
- The closed key set is the load-bearing fact for Phase A; if a new filter
  directive is added later, the struct gains a field (compile-time visible)
  instead of a silent map key.
- The synchronous-consumption lifetime is the load-bearing fact for Phase B.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/filter_delta.go` - all encode helpers + extractors; make() sites at lines 80, 338, 356, 395, 414, 428, 446, 496, 505, 556, 629, 645, 651, 654; encodeAttrValue dispatcher (284-314); textDeltaToModOps (202-264)
- [ ] `internal/component/bgp/reactor/filter_chain.go` - parseFilterAttrs (118+, map alloc at 120, Fields at 125, textbuf.Join for multi-token + nlri); isPolicyAttrName (167-178); policySingleToken (103-108); parseFilterAttrsCalls counter (113)
- [ ] `internal/component/bgp/reactor/filter_delta_handlers.go` - handlers consume ops[i].Buf and write into the output buffer; they do not retain Buf
- [ ] `internal/component/bgp/reactor/reactor_notify.go` - import call site (460-504): two parses at 483-484, extractors 485-489, buildModifiedPayload 492
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go` - export call site (475-508), same shape, per destination peer
- [ ] `internal/component/bgp/reactor/policy_dryrun.go` - dry-run consumer of textDeltaToModOps (224)
- [ ] `internal/component/bgp/reactor/forward_build.go` - buildModifiedPayload consumes ops; modBufPool pattern (363-379) to mirror for the scratch pool

**Behavior to preserve:**
- Produced wire bytes: every encoder's output bytes byte-identical for identical input text (golden corpus + unit tests).
- Op semantics: same (Code, Action, Buf-content) multiset for a given (original, modified) text pair; `TestFilterDeltaParseOnceEquivalence` is the gate.
- Parse-call invariant: exactly 2 `parseFilterAttrs`-equivalent parses per modified UPDATE (`TestFilterDeltaParseCallCount`; keep the counter, renamed only if the function is renamed).
- Filter text contract with plugins: untouched (this spec starts AFTER PolicyFilterChain returns).
- atomic-aggregate presence semantics (key present, empty value).
- Zero-alloc unmodified path: BenchmarkFilterDispatch_ZeroAlloc stays 0 allocs/op.

**Behavior to change:**
- None functional. Allocation count and internal signatures only.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- A received (import) or about-to-forward (export) UPDATE whose configured external filter returned modified text (modifiedText != updateText).

### Transformation Path
1. UPDATE serialized to filter text into a 64K stack array (AppendUpdateForFilter), passed to PolicyFilterChain (unchanged).
2. Both texts parsed into the new FilterAttrs struct (Phase A), twice per modify.
3. textDeltaToModOps + ExtractRemovePrivateASOps + ExtractASPathPrependOps emit AttrOps whose value bytes are carved from the pooled scratch (Phase B).
4. buildModifiedPayload groups ops by code and writes the modified payload into a per-peer pool buffer or modBufPool buffer (unchanged).
5. Scratch released; ModAccumulator goes out of scope (stack).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine <-> external filter plugin | text + RPC (unchanged, out of scope) | [ ] |
| filter_delta <-> forward_build | filterapi.AttrOp value slices, consumed synchronously | [ ] |
| Scratch pool <-> modify block | Get at block entry, Release after buildModifiedPayload returns | [ ] |

### Integration Points
- `filterapi.ModAccumulator` gains scratch ownership (or a sibling arena type passed alongside); both production call sites + dry-run updated.

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | AttrOp.Buf slices never outlive buildModifiedPayload (no handler stores them in a struct/global/channel) | VERIFIED 2026-06-12: all 11 `.Buf` readers in filter_delta_handlers.go are `len()`, `copy(buf[...], Buf)` into the output, or accumulation into function-local `prependBufs`/`setBuf` (lines 44,58,69,115,126,142-145,246-247,384-385,405); none escape the synchronous call | Scratch reuse corrupts a retained slice | Done (grep). Remaining nuance: `prependBufs` (143/385) holds MULTIPLE carved slices live at once — guarded by the append-only scratch invariant + the new multi-prepend test case; audit confirms the `prependBufs`/`setBuf` locals do not escape their handler | likely |
| A-2 | The filter attribute name set is closed at 16 (no dynamic names) | isPolicyAttrName switch + policySingleToken map are the only producers | Struct misses a key; directive silently dropped | grep producers of the filter text (AppendUpdateForFilter + plugin contract docs) for attribute names; cross-check list | unvalidated |
| A-3 | Dry-run path tolerates the new signatures (cold, no perf constraint) | policy_dryrun.go:224 uses same extractors | Compile break or behavior drift in dry-run | Compile + existing dry-run tests | unvalidated |
| A-4 | ~24 allocs/op current baseline still holds | learned/875 benchmark result | Improvement targets misstated | Re-run BenchmarkFilterModifyEgress before coding; paste numbers here | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Scratch sizing too small for pathological deltas (many large communities) | Grow path hit frequently in benchmark with adversarial input | Bounded grow mirroring modBufPool; worst case falls back to a fresh make for the oversized op only (documented pool-fallback) |
| R-2 | Struct refactor changes op ORDER (map iteration order was previously non-deterministic; tests may have relied on sorting) | TestFilterDeltaParseOnceEquivalence or ordering-sensitive tests fail | Equivalence test compares sorted multisets (filter_delta_test.go:1097 comment notes map order was never guaranteed); keep that property |
| R-3 | Export-path gains are eaten by PolicyFilterChain RPC cost (~20% of path) | Benchmark improves allocs but not ns/op materially | Allocation reduction is the stated goal (GC pressure at fan-out); record ns/op honestly either way |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Received UPDATE with import filter modifying attributes | → | parseFilterAttrs (struct) + textDeltaToModOps + buildModifiedPayload | TestFilterDeltaParseCallCount (existing, filter_delta_test.go ~1085) |
| Forwarded UPDATE with export filter modifying attributes | → | export path in reactor_api_forward.go | BenchmarkFilterModifyEgress (existing; doubles as wiring proof of the egress chain) |
| Filter delta with as-path-prepend + remove-private directives | → | ExtractASPathPrependOps + ExtractRemovePrivateASOps over scratch | TestExtractASPathPrependOps + TestExtractRemovePrivateASOps (existing) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Golden corpus (original, modified) text pairs | Identical op multisets before/after refactor (TestFilterDeltaParseOnceEquivalence green, corpus extended with at least one case per directive: every one of the 16 names exercised) |
| AC-2 | BenchmarkFilterModifyEgress after Phase A | Alloc count reduced by at least the 2 map allocations vs the baseline pasted in this spec |
| AC-3 | BenchmarkFilterModifyEgress after Phase B | Total at or below ~12 allocs/op (target 10; hard gate: at least 40% reduction from the re-measured baseline) |
| AC-4 | Unmodified UPDATE through filters | BenchmarkFilterDispatch_ZeroAlloc remains 0 allocs/op |
| AC-5 | Parse-count invariant | Exactly 2 parses per modified UPDATE (counter test green) |
| AC-6 | Full suite | `make ze-test` passes; `make ze-race-reactor` passes (reactor files touched) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestFilterDeltaParseOnceEquivalence (existing, corpus extended) | `internal/component/bgp/reactor/filter_delta_test.go` | Op multiset equivalence across refactor | |
| TestFilterDeltaParseCallCount (existing) | same | 2 parses per modify | |
| TestParseFilterAttrsStruct (new) | same | Struct parse: every directive name, single + multi token, nlri block, atomic-aggregate presence | |
| TestEncodeValuesIntoScratch (new) | same | Each encoder writes byte-identical values via scratch vs old make path (table over all 14 encoders); INCLUDES a multi-prepend case that holds two scratch-carved Buf slices live simultaneously and asserts both still read correctly (guards the append-only invariant) | |
| TestExtractRemovePrivateASOps / TestExtractASPathPrependOps / TestTextDeltaToModOps / TestExtractLegacyNLRIOverride (existing) | same | Unchanged behavior | |
| BenchmarkFilterModifyEgress (existing) | `internal/component/bgp/reactor/filter_format_bench_test.go` area | allocs/op + ns/op before/after each phase | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| as-path-prepend count | 1..32 (existing cap at filter_delta.go:556 area) | 32 | 0 (no-op) | 33 rejected/capped per existing behavior (preserve exactly) |
| scratch grow bound | initial 4096, grow bounded | bound value | N/A | oversized op falls back per R-1, asserted in TestEncodeValuesIntoScratch |

### Functional Tests
No user-facing behavior change and no new RPC surface; existing test suite
passes (`make ze-verify`) including the policy-filter `.ci` scenarios proving
filters still modify routes end-to-end. No new `.ci` needed: the contract
exercised by users (filter text in, modified UPDATE out) is unchanged and
already covered.

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| existing policy filter suite | `test/` (.ci, unchanged) | Plugin filter modifies communities/as-path; peer receives modified UPDATE | |

### Interop Tests (MANDATORY for protocol features)
Wire bytes byte-identical (AC-1 + TestEncodeValuesIntoScratch); no interop
re-run beyond existing suite. Justification: allocation-shape-only change.

## Files to Modify
- `internal/component/bgp/reactor/filter_chain.go` - parseFilterAttrs returns the FilterAttrs struct; keep the call counter
- `internal/component/bgp/reactor/filter_delta.go` - extractors + encoders take struct + scratch; all listed make() sites removed or routed through scratch
- `internal/component/bgp/reactor/filter_delta_handlers.go` - only if AttrOp plumbing changes signatures (expected: none)
- `internal/component/bgp/reactor/reactor_notify.go` - import call site adopts struct + scratch acquire/release
- `internal/component/bgp/reactor/reactor_api_forward.go` - export call site, same
- `internal/component/bgp/reactor/policy_dryrun.go` - dry-run call site, same
- `internal/component/bgp/reactor/filterapi/filterapi.go` - ModAccumulator scratch ownership (or sibling arena type), if chosen over parameter threading

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | [ ] no | - |
| CLI commands/flags | [ ] no | - |
| Functional test for new RPC/API | [ ] no | - |
| Env var registration | [ ] no | - |
| Doctor check for runtime dependencies | [ ] no | - |
| Prometheus counters/metrics | [ ] no | - |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1-11 | user-facing / config / CLI / API / wire / plugin SDK | [ ] no (filter text contract unchanged) | - |
| 12 | Internal architecture changed? | [ ] check | grep docs/ for anchors on filter_delta.go / filter_chain.go; update if any describe the map-based parse |
| 16 | Changed files referenced by doc source anchors? | [ ] check | same grep |

## Files to Create
- None expected (tests live in existing _test.go files).

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + learned/875 |
| 2. Audit | Validate A-1..A-4; re-run baseline benchmark; paste numbers |
| 3. Wiring phase | Wiring Test table (existing tests) + new failing unit tests |
| 4. Implement (TDD) | Phases below |
| 5-14 | Per template |

### Implementation Phases
1. **Phase: Wiring (MANDATORY FIRST)** - extend the golden corpus to cover all 16 directives; write TestParseFilterAttrsStruct + TestEncodeValuesIntoScratch against intended new signatures (failing); re-run BenchmarkFilterModifyEgress and paste the baseline
   - Tests: TestParseFilterAttrsStruct, TestEncodeValuesIntoScratch
   - Files: filter_delta_test.go
   - Verify: new tests fail (signatures absent); corpus equivalence green on current code
2. **Phase: Struct parse (Phase A)** - FilterAttrs struct + presence bits; migrate extractors + both call sites + dry-run
   - Tests: TestParseFilterAttrsStruct, equivalence, call-count
   - Verify: tests pass; benchmark re-run shows the map allocs gone (paste)
3. **Phase: Encoder scratch (Phase B)** - pooled scratch threading; all encoders carve from it; rewritePrivateASSegments slices included
   - GATE (before starting Phase B): confirm the umbrella baseline profile (umbrella AC-1) actually surfaces `parseFilterAttrs`/`encode*Value` frames AND the re-measured BenchmarkFilterModifyEgress baseline (A-4) confirms ~24 allocs/op. Phase B is the largest refactor surface for the smallest win in this round; if those frames do NOT appear near the top of the profile, STOP and present a scope reconsideration to the user before refactoring the 14 encoders.
   - Tests: TestEncodeValuesIntoScratch (incl. multi-prepend case), all existing delta tests
   - Verify: tests pass; benchmark at AC-3 target; ze-race-reactor green
4. **Functional tests** - re-run existing policy-filter .ci suite via `make ze-verify`
5. **Full verification** - `make ze-verify`
6. **Complete spec** - audit tables, learned summary, two-commit closure

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All 14 listed make() sites accounted for (removed, scratch-routed, or justified in a table here) |
| Correctness | Byte-identical encoder output proven per encoder (table test) |
| Data flow | Scratch released exactly once per modify block on every path including early returns and errors |
| Rule: scratch append-only | No carve rewinds or reuses scratch space within a block; multi-prepend slices stay valid (TestEncodeValuesIntoScratch multi-prepend case green); invariant stated as a code comment |
| Rule: buffer-first | No remaining make() in encode helpers without a pool-fallback justification comment |
| Rule: stale-comments | parseFilterAttrs doc comment + spec references updated |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| FilterAttrs struct with 16 fields + presence | grep filter_chain.go for the type |
| Benchmark before/after per phase in spec | grep this file for BenchmarkFilterModifyEgress numbers |
| make() inventory disposition table | grep this file for the table filled at stage 6 |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Filter text is plugin-supplied: token parsing must keep existing bounds (token counts, ASN ranges, community formats); no new panics on malformed text (encoders return errors as today) |
| Resource exhaustion | Scratch grow is bounded; adversarial filter text cannot force unbounded memory |

### Failure Routing
| Failure | Route To |
|---------|----------|
| A-1 broken (an op escapes) | STOP. Scratch design invalid for that op class; carve only non-escaping ops, heap the escaping one, document; present to user |
| Equivalence corpus failure | Fix the refactor, never the corpus (ai/rules/testing.md) |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights
- [filled during implementation]

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Struct with presence bits over map[string]string | Key-code map (uint8 keys); span-based zero-alloc parse | Closed 16-name set; struct is stack-allocated and compile-time checked. Span parse rejected: nlri block spans complicate lifetimes for ~2 allocs more saved (dossier Option D) |
| One pooled scratch per modify block | Per-encoder pools; caller (reactor) threading its own buffer | Single Get/Release pair; lifetime exactly matches ops; mirrors modBufPool idiom |
| Keep textbuf.Join for multi-token values in Phase A | Spans into Fields slice | Join allocs are per-multi-token-attribute (small count); deferred unless benchmark shows them dominant after Phase B |

## Known Limitations
- PolicyFilterChain RPC cost (text serialization + plugin round trip) is untouched; it is the sanctioned external-plugin boundary.
- The unmodified path is untouched (already optimal).

## RFC Documentation

Encoders carry existing RFC 4271 wire-format references; preserve them. No new
protocol-enforcing code.

## Implementation Summary

### What Was Implemented
- [filled at completion]

### Bugs Found/Fixed
- [filled at completion]

### Documentation Updates
- [filled at completion]

### Deviations from Plan
- [filled at completion]

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Modified-UPDATE path allocation cut ~50% | benchmark | BenchmarkFilterModifyEgress before/after [filled at completion] |
| No wire-byte change | unit test | TestEncodeValuesIntoScratch + equivalence corpus [filled at completion] |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- [filled during review]

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] `make ze-race-reactor` passes
- [ ] Feature code integrated (`internal/*`)
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC constraint comments intact
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-perf-next-2-filter-delta-alloc.md`
- [ ] **Commit A:** code + tests + docs + spec (with all edits) + learned summary + counter bump
- [ ] **Commit B:** `git rm` of spec only
