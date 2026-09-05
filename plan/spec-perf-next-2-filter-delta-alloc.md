# Spec: perf-next-2-filter-delta-alloc -- Allocation Reduction in the Filter-Delta Modify Path

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-perf-next-0-umbrella.md |
| Phase | 5/5 |
| Updated | 2026-09-05 |

## Decision, 2026-09-05

Thomas answered the question this spec was blocked on since 2026-07-22:
implement Phase B and meet AC-3. Dropping Phase B is off the table, so there is
no deferral shard and no corrected AC-3.

Phase A landed in `b5ad2cabe`: `filterAttrID`/`filterAttrs` (fixed struct plus
bitset, replacing `map[string]string`) in
`internal/component/bgp/reactor/filter_chain.go`.

The Phase B GATE the Implementation Phases section states was answered by
MEASUREMENT rather than by `ze-perf-bench`, which never exercises the filter
path. A `-memprofile` run of `BenchmarkFilterModifyEgress` itself named every
allocation site on that path with its per-op count, which is what the gate
wanted a profile for. The numbers are in the Benchmarks section below, and they
moved the work: the encoder sites were four allocations of the twenty, not the
whole remainder the gate expected.

## Benchmarks

`go test -run '^$' -bench '^(BenchmarkFilterModifyEgress|BenchmarkFilterDispatch_ZeroAlloc)$'
-benchmem -count=6 ./internal/component/bgp/reactor/`, run through
`./le job run`, on an Apple M4 Max (darwin/arm64).

| Stage | allocs/op | B/op | ns/op |
|-------|-----------|------|-------|
| Baseline, re-measured 2026-09-05 (A-4) | 20 | 3432 | 1671-2007 |
| Value arena alone (the spec's Phase B) | 14 | 3325 | 1556-1686 |
| Plus the non-allocating parse | 6 | 1931 | 1721-1744 |
| Final, with the ASCII whitespace table | 6 | 1931 | 1360-1472 |

`BenchmarkFilterDispatch_ZeroAlloc` reads 0 allocs/op at every stage (AC-4).

AC-3 asked for 12 or fewer, targeted 10, and gated on a 40% cut from the
re-measured baseline, which is 12. The result is 6: a 70% cut, 44% fewer bytes,
and about 19% less wall time.

R-3 said to record the ns/op honestly whichever way it went, and the honest
account has two steps. The parse rewrite alone spent CPU to save allocations
and landed at 1730 against a 1720 baseline median, inside the noise. Reading
whitespace through the 256-entry ASCII table `strings.Fields` uses, rather than
through `unicode.IsSpace` on a decoded rune, took it to 1390.

WHERE THE FIRST TWENTY WENT, read per line from a `-memprofile` run at
`-memprofilerate 1`. Eight were the parse (`textbuf.Join` 4, `strings.Fields`
2, `&filterAttrs{}` 2), four were the encoder sites this spec's Phase B names,
four were `attribute.ParseASPath` inside the remove-private rewrite, and the
rest were the AS path rewrite's own slices. The four in `ParseASPath` are the
bulk of what remains: they live in `internal/core/bgp/attribute`, which this
spec does not touch.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/bgp/reactor/filter_delta.go`, `filter_chain.go` (parseFilterAttrs), `filter_delta_handlers.go`
4. `internal/component/bgp/reactor/filter_delta_parse_test.go` - the call-count test the prior optimization of this path left behind

## Task

When an external policy filter modifies an UPDATE, the reactor converts the
filter's text delta into wire attribute operations. This path currently costs
**~24 allocs per modified UPDATE** (measured by BenchmarkFilterModifyEgress
after spec filter-delta-parse-once reduced it from 34). It fires:

- Import: per received UPDATE when `peer.settings.ImportFilters` is non-empty AND the filter changed the text (`reactor_notify.go`).
- Export: per DESTINATION PEER per forwarded UPDATE when export filters are configured and modify the text (`reactor_api_forward.go`). The export path multiplies the cost by fan-out, making it the valuable half.

The unmodified path is already 0 allocs/op (BenchmarkFilterDispatch_ZeroAlloc).
This spec attacks the modified path's two allocation families:

1. **Parse maps:** `parseFilterAttrs` (`filter_chain.go`) allocates a `map[string]string` plus a `strings.Fields` slice plus joined value strings, twice per modified UPDATE (original + modified text). The key set is CLOSED: 16 names defined by `isPolicyAttrName` (`filter_chain.go`) + `policySingleToken` (`filter_chain.go`).
2. **Encoder buffers:** every `encode*Value` helper allocates its own `make` (filter_delta.go lines 80, 338, 356, 395, 414, 428, 446, 496, 505, 556, 629, 645, 651, 654), violating `ai/rules/performance.md` (helpers must write into a caller-provided buffer).

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
output as today. This also satisfies `ai/rules/go-standards.md` for this path.

**Phase B: encoder scratch.** Give the encode helpers a caller-provided scratch
that lives exactly as long as the ops do. The ops' `Buf` slices
(`filterapi.AttrOp.Buf`) are consumed synchronously by `buildModifiedPayload`
and discarded with the `ModAccumulator` (stack-allocated at both call sites:
`reactor_notify.go`, `reactor_api_forward.go`), so a single pooled
scratch buffer per modify block can back ALL encoded values: each encoder
carves its value bytes from the scratch (append-and-slice), and the scratch is
released after `buildModifiedPayload` returns. Sized at 4096 with bounded grow,
mirroring the existing `modBufPool` pattern in `forward_build.go`.
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
serialization boundary per `ai/rules/performance.md`), the unmodified
fast path (already zero-alloc), and `rewritePrivateASSegments` semantic changes
(its slices move to the scratch arena but its logic is untouched).

### Why this is safe

- Lifetime: ops never escape the synchronous modify block. `buildModifiedPayload` copies attribute bytes into the output payload buffer (per-peer pool or modBufPool); after it returns, no reference to the scratch remains. Validate during audit (A-1).
- The dry-run path (`policy_dryrun.go`) uses the same extractors; it must adopt the same struct/scratch signatures (it is cold, correctness only).
- Golden-corpus equivalence already exists: `TestFilterDeltaParseOnceEquivalence` compares op multisets across refactors; it gates Phase A. `TestFilterDeltaParseCallCount` (atomic counter `parseFilterAttrsCalls`) keeps the 2-parses-per-modify invariant.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - the canonical architecture reference: the design principles all new code follows
- [ ] `ai/rules/performance.md` - banned `make` in encoding helpers
  → Constraint: encoders must write into caller-provided buffers; this spec brings filter_delta.go into compliance
- [ ] `ai/rules/performance.md` - sanctioned copies + sync.Pool guidance
  → Constraint: filter modify IS a sanctioned copy boundary; the goal is fewer allocations, not zero copies
- [ ] `ai/rules/go-standards.md` - string keys on hot paths
  → Decision: closed 16-name key set becomes struct fields (typed identity)
- [ ] `docs/architecture/perf-round-3.md` - the campaign this round belongs to
  → Decision: parse exactly twice per modify; extractors share read-only; preserve the call-count test
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

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

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
| A-1 | AttrOp.Buf slices never outlive buildModifiedPayload (no handler stores them in a struct/global/channel) | VERIFIED 2026-06-12 against the handler shape of that date | Scratch reuse corrupts a retained slice | Re-derived 2026-09-05 and the SHAPE HAS CHANGED: `prependBufs`/`setBuf` are gone. `aspathHandler` (filter_delta_handlers.go) now records `p.Op(i)` for every prepend plus the Set, and `EditSet.write` (filterapi/editset.go) reads `ops[oi].Buf` for each planned fragment when it materializes the payload. So EVERY operation's Buf is live at once, until buildModifiedPayload returns, which makes the append-only invariant more load-bearing rather than less. Still no escape past that return | confirmed |
| A-2 | The filter attribute name set is closed at 16 (no dynamic names) | isPolicyAttrName switch + policySingleToken map are the only producers | Struct misses a key; directive silently dropped | grep producers of the filter text (AppendUpdateForFilter + plugin contract docs) for attribute names; cross-check list | confirmed 2026-09-05, at 23 rather than 16: `filterAttrNames` (filter_chain.go) is the closed list Phase A built from the `filterAttrID` enum, `filterAttrNameToID` is derived from it at init, and `isPolicyAttrName` names the same set. Phase B added no name |
| A-3 | Dry-run path tolerates the new signatures (cold, no perf constraint) | policy_dryrun.go uses same extractors | Compile break or behavior drift in dry-run | Compile + existing dry-run tests | confirmed: `computeWireChanges` (policy_dryrun.go) acquires and releases a scratch of its own; the reactor package's dry-run tests pass |
| A-4 | ~24 allocs/op current baseline still holds | learned/875 benchmark result | Improvement targets misstated | Re-run BenchmarkFilterModifyEgress before coding; paste numbers here | BROKEN. Re-measured 2026-09-05: 20 allocs/op, not 24 and not the 22 the Phase A page records. Targets are computed from 20 (see Benchmarks and the Mistake Log) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Scratch sizing too small for pathological deltas (many large communities) | Grow path hit frequently in benchmark with adversarial input | Bounded grow mirroring modBufPool; worst case falls back to a fresh make for the oversized op only (documented pool-fallback) |
| R-2 | Struct refactor changes op ORDER (map iteration order was previously non-deterministic; tests may have relied on sorting) | TestFilterDeltaParseOnceEquivalence or ordering-sensitive tests fail | Equivalence test compares sorted multisets (filter_delta_test.go comment notes map order was never guaranteed); keep that property |
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
| AC-6 | Full suite | `./le verify current mode full` passes; `go test -race ./internal/component/bgp/reactor/...` passes (reactor files touched) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestFilterDeltaParseOnceEquivalence (existing, corpus NOT extended) | `internal/component/bgp/reactor/filter_delta_test.go` | Op multiset equivalence across refactor | green. The corpus was left exactly as it was, which is what makes it evidence: a corpus edited in the same change proves nothing about the change |
| TestFilterDeltaParseCallCount (existing) | same | 2 parses per modify | green |
| TestParseFilterAttrsStruct (new) | `internal/component/bgp/reactor/filter_chain_test.go` | Every directive name, single + multi token, nlri block, both valueless tokens, the unknown-name record, and the separator cases that decide whether a value is the window or a rewrite | green. It lives beside the parse it covers rather than in filter_delta_test.go |
| TestEncodeValuesIntoScratch (new) | `internal/component/bgp/reactor/filter_delta_test.go` | Every name `encodeAttrValue` dispatches, encoded into ONE scratch and read after the last carve; the multi-prepend case that holds two carved Buf slices live and drives them through buildModifiedPayload; the R-1 grow case | green |
| TestValueScratchCarvesWindowsThatDoNotOverlap (new) | `internal/component/bgp/reactor/filter_scratch_test.go` | The arena itself: windows that do not overlap, a capped window an append cannot escape into, a carve zeroed against the previous block's bytes, and the segment/ASN reservations | green |
| TestExtractRemovePrivateASOps / TestExtractASPathPrependOps / TestTextDeltaToModOps / TestExtractLegacyNLRIOverride (existing) | same | Unchanged behavior | green |
| BenchmarkFilterModifyEgress (existing) | `internal/component/bgp/reactor/filter_delta_test.go` | allocs/op + ns/op before/after each phase | green; see Benchmarks |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| as-path-prepend count | 1..32 (existing cap at filter_delta.go area) | 32 | 0 (no-op) | 33 rejected/capped per existing behavior (preserve exactly) |
| scratch grow bound | initial 4096, grow bounded | bound value | N/A | oversized op falls back per R-1, asserted in TestEncodeValuesIntoScratch |

### Functional Tests
No user-facing behavior change and no new RPC surface; existing test suite
passes (`./le verify current mode full`) including the policy-filter `.ci` scenarios proving
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
| 12 | Internal architecture changed? | [ ] yes | `docs/architecture/perf-round-3.md`, DONE. Section 4 added; the Phase A paragraph that named the 14 encoder sites as the whole remainder is gone, because it was wrong about the code |
| 16 | Changed files referenced by doc source anchors? | [ ] yes | `ai/CODE-TO-DOCS.md` maps `filter_chain.go` to `perf-round-3.md`, `process-protocol.md`, `egress-attribute-rules.md`, `irr-filtering.md` and `redistribution.md`, and `filter_delta.go` to `egress-attribute-rules.md` and `plugins.md`. Only `perf-round-3.md` states anything this change makes wrong: the others describe the filter text contract and the med-remove ordering, both untouched. Two new anchors added for `filter_scratch.go` and `parseFilterAttrsInto` |
| 16a | Is the generated index in step with the new anchor? | [ ] owed at closure | `./le docs-to-code index-update` ran and wrote no change, because `filter_scratch.go` is still untracked and the walk reads what git holds. Run it again in the closure commit's tree so the new file gets its row |

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

### Phase B as built (2026-09-05)

The design the spec chose is what landed, with one addition the measurement
forced. The arena is a sibling type rather than a field on `ModAccumulator`,
which is the alternative the Integration Points row already allowed:
`filterapi` is a near-leaf package and cannot name `attribute.ASPathSegment`,
and the AS path rewrite reserves segments and ASNs from the same block.

The addition is the parse. The spec's Key Design Decisions row kept
`textbuf.Join` "deferred unless benchmark shows them dominant after Phase B",
and after Phase B they were the largest family left, so the row's own condition
made the work due. AC-3 could not be met without it: the arena alone reaches 14.

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
   - Verify: tests pass; benchmark at AC-3 target; ze-unit-reactor-test-race green
4. **Functional tests** - re-run existing policy-filter .ci suite via `./le verify current mode full`
5. **Full verification** - `./le verify current mode full`
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
| A-4: the modify path costs ~24 allocs/op, 22 after Phase A | 20 allocs/op, 3432 B/op | Re-ran `BenchmarkFilterModifyEgress` before writing any code, as A-4 required | The AC-3 gate moved: 40% of 20 is 12, not 13. Both figures are recorded rather than the remembered one |
| The remaining allocations are the 14 encoder `make(` sites (`docs/architecture/perf-round-3.md`, written at Phase A) | The encoder sites are FOUR of the twenty. Eight are the parse (`textbuf.Join` 4, `strings.Fields` 2, `&filterAttrs{}` 2) and four are `attribute.ParseASPath` inside the remove-private rewrite | `-memprofile` with `-memprofilerate 1`, read per line with `go tool pprof -lines` | Phase B alone reaches 14 allocs/op, which misses AC-3. The parse work the spec's own Key Design Decisions row deferred "unless benchmark shows them dominant after Phase B" became required, and the page's claim was corrected |
| The handlers hold carved buffers in function-local `prependBufs`/`setBuf` slices (A-1, 2026-06-12) | Those locals are gone. `aspathHandler` records `p.Op(i)` per operation and `EditSet.write` reads `ops[i].Buf` for every fragment at write time | Re-derived the `.Buf` readers before relying on A-1 | The append-only invariant covers MORE than the spec expected: every operation's buffer is live until the rebuild finishes, not just the prepend run |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights
- THE PROFILE MOVED THE WORK, and the design that was written 14 months of
  commits earlier could not have known where. The spec named 14 `make(` sites
  and expected them to be most of the cost; they were four allocations of
  twenty. Re-measuring before coding, which A-4 demanded, is what turned a
  plausible plan into the right one.
- ESCAPE ANALYSIS HIDES SMALL ENCODERS. `make([]byte, 4)` in
  `encodeUint32Value` allocated 51 times in 20,000 iterations, because the
  accumulator it flows into is a stack local and the compiler could see the
  whole chain. Counting `make(` calls therefore over-counts the cost, and
  reading the profile per line is the only way to know which ones are real.
- AN ARENA IS A LIFETIME STATEMENT, not a buffer. What makes `valueScratch`
  correct is not its size but the sentence "nothing rewinds until the block
  releases it", because the rebuild reads every operation's `Buf` at write time.
  A rewinding arena of any size would corrupt the payload silently.
- THE CHEAPEST WAY TO READ A BYTE IS A TABLE. Replacing `strings.Fields` with a
  rune-decoding scan removed six allocations and gave the CPU back. The
  256-entry ASCII table `strings.Fields` itself uses recovered 340 ns of 1730,
  which is where the win actually came from.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Struct with presence bits over map[string]string | Key-code map (uint8 keys); span-based zero-alloc parse | Closed 16-name set; struct is stack-allocated and compile-time checked. Span parse rejected: nlri block spans complicate lifetimes for ~2 allocs more saved (dossier Option D) |
| One pooled scratch per modify block | Per-encoder pools; caller (reactor) threading its own buffer | Single Get/Release pair; lifetime exactly matches ops; mirrors modBufPool idiom |
| Keep textbuf.Join for multi-token values in Phase A | Spans into Fields slice | Join allocs are per-multi-token-attribute (small count); deferred unless benchmark shows them dominant after Phase B |
| DISCHARGED 2026-09-05: the Join went, and so did `strings.Fields`. The benchmark showed them dominant after Phase B (6 of the 14 remaining), which is the condition the row above set | Keep them and miss AC-3; or the span-in-struct parse this spec rejected | A multi-token value is now the WINDOW its tokens already sit in, which is a plain `string` aliasing the text exactly as every single-token value already did. No new lifetime constraint, so this is not the rejected span parse. A run whose separators are not single spaces still goes through `joinFilterTokens`, so the value is byte-identical either way |

## Known Limitations
- PolicyFilterChain RPC cost (text serialization + plugin round trip) is untouched; it is the sanctioned external-plugin boundary.
- The unmodified path is untouched (already optimal).

## RFC Documentation

Encoders carry existing RFC 4271 wire-format references; preserve them. No new
protocol-enforcing code.

## make() Inventory Disposition (Phase B)

The spec listed 14 `make(` sites in `filter_delta.go` by line number, and those
numbers had moved. The table names them by SYMBOL, which is what a later reader
can still find. Every one of them now carves from the block's `valueScratch`
(`filter_scratch.go`).

| # | Site | Allocated | Disposition |
|---|------|-----------|-------------|
| 1 | `extractLegacyNLRIOverride` | NLRI prefix bytes | `carveBytes` (upper bound reserved, written prefix returned) |
| 2 | `encodeASPathValue` | `[]uint32` of parsed ASNs | `carveASNs` |
| 3 | `encodeASPathValue` | segment bytes | `carveBytes` |
| 4 | `encodeUint32Value` | 4 bytes (MED, LOCAL_PREF) | `carveBytes` |
| 5 | `encodeAggregatorValue` | 8 bytes | `carveBytes` |
| 6 | `encodeCommunityValue` | 4 bytes per community | `carveBytes` |
| 7 | `encodeLargeCommunityValue` | 12 bytes per community | `carveBytes` |
| 8 | `encodeAIGPValue` | 11-byte TLV | `carveBytes` |
| 9 | `encodeClusterListValue` | 4 bytes per cluster ID | `carveBytes` |
| 10 | `ExtractASPathPrependOps` | prepend segment | `carveBytes` |
| 11 | `rewriteASPathRemovePrivate` | rewritten AS_PATH | `carveBytes` |
| 12 | `rewriteAS4PathRemovePrivate` | rewritten AS4_PATH | `carveBytes` |
| 13 | `rewritePrivateASSegments` | `[]ASPathSegment` | `carveSegments` |
| 14 | `rewritePrivateASSegments` | `[]uint32` per segment | `carveASNs` |

Three sites that were not `make(` but still allocated are carved too:
`encodeOriginValue` returned a `[]byte{n}` literal, and `encodeNextHopValue`
and `encodeIPv4Value` returned `ip4[:]` over an escaping array.

ONE ALLOCATION IS LEFT AND IT IS NOT IN THIS PACKAGE. `encodeExtCommunityValue`
calls `attribute.NewBuilder()` and `Build()`, which is the only public parser
for extended communities and builds a whole attribute before the value can be
taken out of it. The value is copied into the scratch so every operation buffer
still has one lifetime. Removing it needs an append-style parser in
`internal/core/bgp/attribute`, which this spec does not touch.

## Implementation Summary

### What Was Implemented
- `valueScratch` (`internal/component/bgp/reactor/filter_scratch.go`, new): the
  pooled, append-only value arena, with `carveBytes`, `carveSegments` and
  `carveASNs`, an `acquireValueScratch`/`releaseValueScratch` pair, and bounded
  retention so one adversarial delta cannot pin an oversized buffer in the pool.
- Every filter-delta encoder and extractor takes the arena as its first
  argument (`filter_delta.go`). The disposition table above accounts for all 14
  `make(` sites the spec listed, plus three more that allocated without one.
- A non-allocating parse (`filter_chain.go`): `filterTokens` scans the text
  once and keeps offsets, `parseFilterAttrRun` takes a multi-token value as the
  window its tokens sit in, `joinFilterTokens` is the fallback for a text whose
  separators are not single spaces, and `parseFilterAttrsInto` writes into
  caller-provided storage.
- Both production modify blocks (`filter_ordered.go`, import and export) and
  the dry-run (`policy_dryrun.go`) acquire the arena, parse into their own
  structs, and release the arena after `buildModifiedPayload` returns.

### Bugs Found/Fixed
- Three doc comments in `filter_delta.go` still called the parse output a
  "map", which Phase A replaced with a struct, and two of them named
  `reactor_notify.go` and `reactor_api_forward.go` as the call sites when both
  had moved to `filter_ordered.go`. Fixed in the functions this phase edited.

### Documentation Updates
- `docs/architecture/perf-round-3.md`: section 4 added, and the Phase A
  paragraph claiming the remaining 22 allocations were the 14 encoder `make()`
  sites was removed. The profile says four of twenty, so the claim was wrong
  about the code and would have sent the next reader to the wrong place.

### Deviations from Plan
- [filled at completion]

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Cut the modified-UPDATE path to roughly 10 allocs/op or fewer | met | Benchmarks | 6 |
| Without changing any produced wire bytes | met | `TestFilterDeltaParseOnceEquivalence` over an unedited corpus, `TestEncodeValuesIntoScratch` | |
| Without changing op sequences | met | same | The equivalence test compares sorted multisets, which is the property R-2 asked to keep |
| Without changing filter text contracts | met | `TestParseFilterAttrsStruct`, and the plugin `.ci` scenarios that drive a modifying filter (`community-strip`, `community-cumulative`, `aspath-filter-*`, `med-removal-*`) | The parse takes a value as a window into the text; a run whose separators are not single spaces is still normalized, so the contract is byte-identical |
| Family 1, parse maps | met | `parseFilterAttrsInto`, `filterTokens` (filter_chain.go) | Phase A removed the map; this removed what was left |
| Family 2, encoder buffers | met | `valueScratch` (filter_scratch.go) and the disposition table above | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | met | `TestFilterDeltaParseOnceEquivalence`, `TestEncodeValuesIntoScratch` | The corpus is unchanged and still green, so the op multiset survived both refactors. The new table encodes every one of the 13 attribute names `encodeAttrValue` dispatches, with its wire bytes as hex |
| AC-2 | met | Benchmarks table | Phase A's own claim is superseded: the re-measured baseline was 20, and the value arena took it to 14 |
| AC-3 | met | Benchmarks table | 6 allocs/op against a gate of 12 and a target of 10. 70% below the re-measured baseline |
| AC-4 | met | `BenchmarkFilterDispatch_ZeroAlloc` | 0 allocs/op at every stage |
| AC-5 | met | `TestFilterDeltaParseCallCount` | The counter moved into `parseFilterAttrsInto`, which both entry points go through, so it still counts one per parse |
| AC-6 | partial, and the gap is not this spec's | `go test -race -count=1 ./internal/component/bgp/reactor/...`: no data race. `./le verify current mode full` was NOT run | Two tests in the package fail, both in files this spec did not touch and both explained by another session's uncommitted work in this shared checkout: `TestPeerRawEntryPointNeedsTheRawSendWord` (the `send` root-verb spec, `internal/component/plugin/server/send_test.go` is modified) and `TestNoConfigFeedsSentUpdatesToAReceivedOnlyPlugin` (108 config documents refused by the YANG parser, and `internal/component/config/yang/cli/tree.go` is modified) |

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
- [ ] `./le verify worktree` passes (lint + all ze tests)
- [ ] `go test -race ./internal/component/bgp/reactor/...` passes
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
