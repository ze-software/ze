# Spec: filter-delta-parse-once -- parse filter text once on the modify egress path

| Field | Value |
|-------|-------|
| Status | done |
| Depends | - |
| Phase | 7/7 |
| Updated | 2026-06-10 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/rules/no-sprintf-alloc.md` (Hot Path Rule), `ai/rules/memory-architecture.md`
4. `internal/component/bgp/reactor/filter_delta.go`, `filter_chain.go`, `reactor_api_forward.go`, `reactor_notify.go`

## Task

When a policy filter modifies a route, the reactor turns the text delta into wire-level modification ops via three functions in `filter_delta.go`: `textDeltaToModOps`, `ExtractRemovePrivateASOps`, and `ExtractASPathPrependOps`. Each of the three independently calls `parseFilterAttrs(modified)` (and `textDeltaToModOps` also parses `original`). So for every UPDATE that a filter actually changes, the modified filter text is tokenised and parsed into a `map[string]string` **three times**, and the original once: four `parseFilterAttrs` calls where two distinct parses (original + modified) would suffice.

This runs on the per-UPDATE egress hot path (`reactor_api_forward.go`) and the ingress path (`reactor_notify.go`), and on the cold dry-run path (`policy_dryrun.go`). `parseFilterAttrs` allocates a map plus a `strings.Fields` slice plus `strings.Join` strings each call, so the redundancy is measurable allocation and CPU per modified route.

Goal: parse `original` and `modified` once each, pass the resulting attribute maps to all three extractors, eliminating the redundant re-parses without changing any output. This was surfaced as a deep-review follow-up during pol-4 (`plan/learned/814-pol-4-explain.md`) and deferred as out-of-scope for a CLI dry-run change.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as → Decision: / → Constraint: annotations — these survive compaction. -->
<!-- Track reading progress in session-state.md, not here. -->
- [ ] `ai/rules/no-sprintf-alloc.md` - Hot Path Rule / banned allocation patterns
  → Constraint: `reactor_api_forward.go` and `reactor_notify.go` are per-UPDATE hot paths; redundant per-call allocation must be removed, not tolerated.
- [ ] `ai/rules/memory-architecture.md` - caller-owned buffers, "callee allocates what caller could provide"
  → Constraint: prefer parse-once-and-pass over each callee re-parsing the same input.
- [ ] `docs/architecture/` policy/filter doc (whichever `filter_delta.go` `// Design:` points to)
  → Decision: the text-delta -> ModAccumulator translation is the single source of wire ops; output must stay byte-identical.

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc6793.md` - AS4_PATH / 4-octet AS handling (touched by `ExtractRemovePrivateASOps`/`ExtractASPathPrependOps`)
  → Constraint: AS-path manipulation semantics must not change; this is a pure refactor of *how* the text is parsed, not *what* ops result.

**Key insights:** (summary of all checkpoint lines — minimal context to resume after compaction)
- `parseFilterAttrs` (`filter_chain.go:112`) returns `map[string]string` keyed by attribute name; deterministic, no side effects, safe to compute once and share read-only.
- The three extractors only *read* the parsed map; none mutate it. That makes "parse once, pass the map" safe with no aliasing risk.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
<!-- Same rule: never tick [ ] to [x]. Write → Constraint: annotations instead. -->
- [ ] `internal/component/bgp/reactor/filter_chain.go` (parseFilterAttrs, line 112) - `parseFilterAttrs(text string) map[string]string`: `strings.Fields` tokenises, walks tokens, multi-token values joined via `strings.Join`, NLRI section captured specially. Pure function.
  → Constraint: output is a fresh map per call; callers treat it read-only.
- [ ] `internal/component/bgp/reactor/filter_delta.go` (textDeltaToModOps, lines 198-200) - `textDeltaToModOps(original, modified, mods)` calls `parseFilterAttrs(original)` AND `parseFilterAttrs(modified)`.
  → Constraint: needs BOTH maps (it diffs them).
- [ ] `internal/component/bgp/reactor/filter_delta.go` (ExtractASPathPrependOps, lines 539-540) - `ExtractASPathPrependOps(modified, localAS, mods)` calls `parseFilterAttrs(modified)`.
  → Constraint: needs only the modified map.
- [ ] `internal/component/bgp/reactor/filter_delta.go` (ExtractRemovePrivateASOps, lines 572, 602) - `ExtractRemovePrivateASOps(modified, attrs, asn4, peerAS, mods)` calls `parseFilterAttrs(modified)`.
  → Constraint: needs only the modified map (note: its `attrs` param is the wire `*attribute.AttributesWire`, unrelated to the text map).
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go` (egress, lines 483-489) - hot path: inside `if modifiedText != updateText`, calls all three in sequence -> modified parsed 3x, original 1x per modified UPDATE.
  → Constraint: this is the primary hot path the refactor must speed up.
- [ ] `internal/component/bgp/reactor/reactor_notify.go` (ingress, lines 437-441) - identical three-call sequence.
  → Constraint: must be updated symmetrically with egress.
- [ ] `internal/component/bgp/reactor/policy_dryrun.go` (dry-run, lines 212-214) - cold path: identical sequence (pol-4 `computeWireChanges`).
  → Constraint: must be updated for consistency; cold path so perf is secondary but correctness/symmetry matters.

**Behavior to preserve:** (unless user explicitly said to change)
- The `ModAccumulator` ops produced for any (original, modified) pair MUST be byte-identical before and after the refactor.
- Public signatures of the three extractors are exported (`ExtractRemovePrivateASOps`, `ExtractASPathPrependOps`); changing them ripples to all call sites — all three call sites are in-repo (`reactor_api_forward.go`, `reactor_notify.go`, `policy_dryrun.go`) plus the filter_modify plugin doc-references.
- `parseFilterAttrs` itself stays a pure function (may keep it for other callers in `filter_chain.go`/`policy_dryrun.go`).

**Behavior to change:** (only if user explicitly requested)
- Extractors accept already-parsed attribute maps instead of re-parsing text, OR a shared helper parses once and calls the three. Exact shape is a design decision (see Key Design Decisions).

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- An UPDATE arrives; a policy filter chain runs and returns `modifiedText != updateText`.
- Format at entry: filter text (the `AppendUpdateForFilter` rendering) for original and modified.

### Transformation Path
1. Reactor renders `updateText` (original) via `AppendUpdateForFilter`.
2. Filter chain produces `modifiedText`.
3. `textDeltaToModOps(original, modified, &mods)` diffs the two parsed maps -> set/remove ops.
4. `ExtractRemovePrivateASOps(modified, ...)` and `ExtractASPathPrependOps(modified, ...)` append further ops from the modified map.
5. `buildModifiedPayload` applies the accumulated ops to the wire payload.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Filter text <-> ModAccumulator | the three extractors | [ ] |
| ModAccumulator <-> wire payload | `buildModifiedPayload` (unchanged) | [ ] |

### Integration Points
- `ModAccumulator` (`registry`) - the ops sink; unchanged.
- `parseFilterAttrs` - the parse primitive; called fewer times after the change.

### Architectural Verification
- [ ] No bypassed layers (ops still flow text-delta -> ModAccumulator -> payload)
- [ ] No unintended coupling (extractors stay in `filter_delta.go`)
- [ ] No duplicated functionality (the three redundant parses collapse to two distinct parses)
- [ ] Zero-copy preserved where applicable (parsed map shared read-only, not copied per extractor)

## Wiring Test (MANDATORY — NOT deferrable)

<!-- BLOCKING: Proves the feature is reachable from its intended entry point. -->
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Modified UPDATE on egress | → | parse-once path in `reactor_api_forward.go` produces identical ops | existing `test/plugin/policy-test-remove-private-as.ci` + new `test/encode/filter-modify-egress.ci` (or existing egress encode test) |
| Modified UPDATE on ingress | → | parse-once path in `reactor_notify.go` | existing ingress filter functional test |
| Refactored extractors | → | `textDeltaToModOps`/`ExtractRemovePrivateASOps`/`ExtractASPathPrependOps` accept parsed maps | `TestFilterDeltaParseOnceEquivalence` (filter_delta_test.go) |

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Any (original, modified) filter-text pair | `ModAccumulator` ops produced after the refactor are byte-identical to those produced before (equivalence test over a representative corpus, including remove-private-as and as-path-prepend cases). |
| AC-2 | A modified UPDATE on the egress hot path | `modified` filter text is parsed exactly once (not three times); `original` parsed exactly once. |
| AC-3 | A modified UPDATE on the ingress path | Same parse-once guarantee as egress. |
| AC-4 | Dry-run path (`computeWireChanges`) | Same parse-once guarantee; `show policy test` wire-changes output unchanged. |
| AC-5 | AS4_PATH / remove-private-as / as-path-prepend directives present | Output ops unchanged (RFC 6793 semantics preserved); covered by existing pol-4 tests. |
| AC-6 | Allocation count on the modify egress path | Measurably fewer `parseFilterAttrs`-attributable allocations per modified UPDATE (benchmark before/after, or test asserting parse-call count via a counter/seam). |
| AC-7 | Full verification | `make ze-verify` passes. |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestFilterDeltaParseOnceEquivalence` | `internal/component/bgp/reactor/filter_delta_test.go` | refactored path yields identical ops to a golden reference over a corpus | |
| `TestFilterDeltaParseCallCount` | `internal/component/bgp/reactor/filter_delta_test.go` | `modified` parsed once, `original` once per modify (via test seam/counter) | |
| `BenchmarkFilterModifyEgress` | `internal/component/bgp/reactor/filter_delta_test.go` | allocs/op drop after refactor (AC-6 evidence) | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A (no new numeric inputs) | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `policy-test-remove-private-as` | `test/plugin/policy-test-remove-private-as.ci` (existing) | remove-private-as still rewrites AS_PATH correctly after refactor | |
| `policy-test-as4path-suppress` | `test/plugin/policy-test-as4path-suppress.ci` (existing) | AS4_PATH suppression unchanged | |
| egress modify encode test | `test/encode/` (existing or new) | a modified route encodes the same wire bytes after refactor | |

### Interop Tests (MANDATORY for protocol features)
<!-- Skip with justification for non-protocol features. -->
- N/A as a new requirement: this is a pure internal refactor that preserves wire output. Existing AS-path manipulation behavior is already covered by functional encode tests; no new interop scenario is needed because no observable protocol behavior changes. Justification recorded per `ai/rules/interop-and-goal-validation.md` (output-equivalence refactor).

### Future (if deferring any tests)
- None.

## Files to Modify
<!-- MUST include feature code, not only test files -->
- `internal/component/bgp/reactor/filter_delta.go` - change the three extractors to accept parsed maps (or add a shared parse-once driver), keeping ops output identical.
- `internal/component/bgp/reactor/reactor_api_forward.go` (483-489) - parse once, pass maps.
- `internal/component/bgp/reactor/reactor_notify.go` (437-441) - parse once, pass maps.
- `internal/component/bgp/reactor/policy_dryrun.go` (212-214) - parse once, pass maps.
- `internal/component/bgp/reactor/filter_chain.go` - `parseFilterAttrs` may stay; verify other callers unaffected.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | No | - |
| YANG validation constraints | No | - |
| YANG custom validators | No | - |
| CLI commands/flags | No | - |
| CLI grammar (action before identifier) | No | - |
| Editor autocomplete | No | - |
| Functional test for new RPC/API | No (existing tests cover behavior) | - |
| Pipe completeness | No | - |
| Env var registration | No | - |
| Doctor check for runtime dependencies | No | - |
| Prometheus counters/metrics | No | - |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | - |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No (output byte-identical) | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No (preserved, not changed) | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Maybe | `filter_delta.go` `// Design:`/`// Related:` annotations if the extractor signatures change |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |

## Files to Create
- None expected (tests added to existing `filter_delta_test.go`). A new encode `.ci` only if no existing egress-modify encode test covers the path.

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-verify` |
| 7-10. Critical review loop | Critical Review Checklist |
| 11. Deliverables review | Deliverables Checklist |
| 12. Security review | Security Review Checklist |
| 13. Re-verify | `make ze-verify` |
| 14. Present summary | Executive Summary |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring + golden equivalence (MANDATORY FIRST)** — capture current behavior as a reference
   - Tests: `TestFilterDeltaParseOnceEquivalence` written against the CURRENT code to establish the golden ops for a corpus (must pass before refactor, then keep passing after).
   - Files: `filter_delta_test.go`
   - Verify: golden table captures ops for diverse pairs (set, remove, prepend, remove-private-as, NLRI override, no-op).
2. **Phase: Refactor extractors** — accept parsed maps
   - Tests: `TestFilterDeltaParseOnceEquivalence` still green; add `TestFilterDeltaParseCallCount`.
   - Files: `filter_delta.go`
   - Verify: ops identical; parse calls reduced.
3. **Phase: Update call sites** — egress, ingress, dry-run
   - Files: `reactor_api_forward.go`, `reactor_notify.go`, `policy_dryrun.go`
   - Verify: each parses once and passes maps; `make ze-lint-changed` clean.
4. **Phase: Benchmark** — `BenchmarkFilterModifyEgress` before/after for AC-6 evidence.
5. **Functional tests** → run existing `policy-test-*` and egress encode tests; confirm unchanged.
6. **Full verification** → `make ze-verify`.
7. **Complete spec** → audit tables + learned summary; two commits (A: code+spec+learned+counter; B: `git rm` spec).

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | ops byte-identical (golden equivalence test green) |
| Data flow | extractors read shared map read-only; no mutation/aliasing |
| Performance | modified parsed once on hot path; benchmark shows fewer allocs |
| Rule: memory-architecture | parse-once-and-pass, callee no longer re-parses caller input |
| Rule: no-layering | redundant parse calls removed, not wrapped |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Extractors accept parsed maps | `grep -n "func textDeltaToModOps\|func Extract" filter_delta.go` shows map params |
| Single parse on egress | read `reactor_api_forward.go` 483-489: one `parseFilterAttrs(modified)` |
| Equivalence test green | `go test ./internal/component/bgp/reactor/ -run TestFilterDeltaParseOnceEquivalence` |
| Benchmark improvement | `go test -bench BenchmarkFilterModifyEgress -benchmem` before/after |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | parsed map from untrusted filter text already bounded by existing parser; no new allocation sized by attacker input |
| Resource | refactor reduces allocations; ensure no accidental retention of the parsed map beyond the modify scope |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Equivalence test fails | ops diverged — re-read extractor logic, the map must carry the same keys the text parse did |
| Benchmark shows no improvement | re-check that all three extractors stopped re-parsing |
| Functional encode test fails | wire output changed — refactor is incorrect, revert to DESIGN |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

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
<!-- LIVE — write IMMEDIATELY when you learn something -->

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Pass parsed `map[string]string` into the three extractors | (a) internal memo/cache keyed by text; (b) a shared driver function that parses once then calls the three | Explicit map params are the simplest, allocation-free-of-cache approach and make the read-only sharing obvious. A cache adds hidden state on a hot path. A driver is acceptable if it reads cleaner; decide during implementation but keep parse-once. |
| Keep `parseFilterAttrs` as a pure exported-internal helper | inline parsing into each extractor | Other callers (`filter_chain.go`, `policy_dryrun.go`) still use it; keep the single parser, call it fewer times. |
| Preserve exact ops output (equivalence test as the gate) | trust manual reasoning | A refactor on AS-path manipulation must be proven non-behavioral; a golden corpus is the only trustworthy gate. |

## Known Limitations
- Ingress (`reactor_notify.go`) and dry-run (`policy_dryrun.go`) are updated for symmetry even though only egress is the hottest path; leaving them inconsistent would invite future divergence.
- Benchmark numbers depend on filter mix; AC-6 evidence is relative (before/after on the same corpus), not an absolute target.

## RFC Documentation
- `// RFC 6793` comments on `ExtractRemovePrivateASOps`/`ExtractASPathPrependOps` must remain accurate after the signature change (semantics unchanged).

## Implementation Summary

### What Was Implemented
- `textDeltaToModOps`, `ExtractASPathPrependOps`, `ExtractRemovePrivateASOps`, and `extractRemovePrivateASMode` now accept parsed `map[string]string` attribute maps instead of raw filter text (`filter_delta.go`).
- All three call sites parse each filter text exactly once and share the maps read-only: egress `reactor_api_forward.go:479-487`, ingress `reactor_notify.go:481-489`, dry-run `policy_dryrun.go:187-196`.
- Dry-run additionally hoists the parse above `computeChangedAttrs` + `computeWireChanges`, collapsing 6 parses per dry-run to 2 (both helpers now take maps).
- Test seam `parseFilterAttrsCalls` (atomic counter) added in `filter_chain.go` for the parse-count proof.
- Tests: `TestFilterDeltaParseOnceEquivalence` (12-case golden corpus captured from pre-refactor code), `TestFilterDeltaParseCallCount` (TDD red at 4 parses, green at 2), `BenchmarkFilterModifyEgress`.
- Benchmark (Apple M4 Max, count 5): 2447 ns/op, 3000 B/op, 34 allocs/op before; 1501 ns/op, 1704 B/op, 24 allocs/op after (-39% time, -43% bytes, -29% allocs per modified UPDATE).

### Bugs Found/Fixed
- None; output proven byte-identical by golden corpus.

### Documentation Updates
- `filter_delta.go` header: `// RFC:` lines added for rfc6996/rfc6793 (hook convention), `// Related:` updated for parseFilterAttrs.
- `docs/guide/plugins.md` anchors checked (`extractLegacyNLRIOverride`, `ExtractASPathPrependOps`, `ExtractRemovePrivateASOps`): claims are behavioral and remain accurate; no doc change needed.

### Deviations from Plan
- `computeChangedAttrs` (dry-run) also converted to maps; spec listed only `computeWireChanges`. Same parse-once principle, keeps the dry-run caller to one parse per text.
- Spec's line references drifted (egress 483-489 -> 479-483 pre-change); structure matched exactly.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Parse original and modified once each | Done | `reactor_api_forward.go:482-483`, `reactor_notify.go:484-485`, `policy_dryrun.go:191-192` | one `parseFilterAttrs` per text per site |
| Pass parsed maps to all three extractors | Done | `filter_delta.go:202` (textDeltaToModOps), `:541` (ExtractASPathPrependOps), `:574` (ExtractRemovePrivateASOps) | map params, read-only |
| Output unchanged | Done | `TestFilterDeltaParseOnceEquivalence` | golden corpus captured pre-refactor |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestFilterDeltaParseOnceEquivalence` 12/12 green pre- and post-refactor | sorted-multiset comparison (op order from map iteration was never deterministic) |
| AC-2 | Done | `TestFilterDeltaParseCallCount` (red at 4, green at 2) + `reactor_api_forward.go:482-483` | |
| AC-3 | Done | same test seam; `reactor_notify.go:484-485` | symmetric with egress |
| AC-4 | Done | `policy_dryrun.go:191-192`; `policy_dryrun_test.go` green | dry-run now 2 parses (was 6) |
| AC-5 | Done | `make ze-plugin-test` 404/404 (includes policy-test-remove-private-as, policy-test-as4path-suppress) | |
| AC-6 | Done | `BenchmarkFilterModifyEgress`: 2447->1501 ns/op, 34->24 allocs/op (count 5) | logs: tmp/filter-delta-bench-before.log / -after.log |
| AC-7 | See Pre-Commit Verification | `make ze-verify` run on shared tree | sole failure is concurrent session's in-flight leaf-list work (config/cli), independent of this change |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestFilterDeltaParseOnceEquivalence | Done | `filter_delta_test.go` | golden corpus, 12 cases |
| TestFilterDeltaParseCallCount | Done | `filter_delta_test.go` | retry-3 hardened against background-goroutine counter noise |
| BenchmarkFilterModifyEgress | Done | `filter_delta_test.go` | b.Loop, ReportAllocs |
| policy-test-remove-private-as.ci | Done (existing) | `test/plugin/` | green in plugin suite |
| policy-test-as4path-suppress.ci | Done (existing) | `test/plugin/` | green in plugin suite |
| egress modify encode | Done (existing) | `test/encode/` suite 51/51 | no new .ci needed |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `filter_delta.go` | Done | extractors take maps; doc comments + RFC header updated |
| `reactor_api_forward.go` | Done | parse once, pass maps |
| `reactor_notify.go` | Done | parse once, pass maps |
| `policy_dryrun.go` | Done | parse hoisted to caller; computeChangedAttrs also converted (deviation, same principle) |
| `filter_chain.go` | Done | parseFilterAttrs unchanged except atomic call counter (test seam) |

### Audit Summary
- **Total items:** 17
- **Done:** 17
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (computeChangedAttrs map conversion beyond spec list)

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Output unchanged | equivalence + functional test | `TestFilterDeltaParseOnceEquivalence` 12/12 green across refactor; `make ze-plugin-test` 404/404 incl. policy-test-remove-private-as.ci, policy-test-as4path-suppress.ci; `make ze-encode-test` 51/51 |
| Fewer parses on hot path | benchmark + call-count test | `BenchmarkFilterModifyEgress` 2447->1501 ns/op, 34->24 allocs/op; `TestFilterDeltaParseCallCount` proves exactly 2 parses (was 4) |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | NOTE | `parseFilterAttrsCalls` adds one uncontended atomic add to every parse (incl. applyFilterDelta); ~1ns vs ~700ns parse cost, exists as AC-2 test seam | `filter_chain.go:110-119` | acknowledged (documented in code comment) |
| 2 | NOTE | `extractLegacyNLRIOverride` still re-tokenizes the nlri block from raw text at all three call sites; pre-existing, block extraction not a full parseFilterAttrs; future cleanup could derive from parsed maps' "nlri" key | `filter_delta.go:49` | acknowledged (outside spec scope) |

### Fixes applied
- None required (0 BLOCKER, 0 ISSUE on run 1).

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| - | - | run 1 was clean | - | - |

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
  → Decision: run 1 found 0 BLOCKER, 0 ISSUE; gate clean first pass.
- [ ] All NOTEs recorded above (or explicitly "none")
  → Decision: both NOTEs recorded and acknowledged above.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/bgp/reactor/filter_delta.go` | yes | git diff stat (52 lines changed) |
| `internal/component/bgp/reactor/filter_chain.go` | yes | git diff stat (7 lines added) |
| `internal/component/bgp/reactor/reactor_api_forward.go` | yes | git diff stat (10 lines) |
| `internal/component/bgp/reactor/reactor_notify.go` | yes | git diff stat (10 lines) |
| `internal/component/bgp/reactor/policy_dryrun.go` | yes | git diff stat (32 lines) |
| `internal/component/bgp/reactor/filter_delta_test.go` | yes | git diff stat (250+ lines added) |
| `internal/component/bgp/reactor/policy_dryrun_test.go` | yes | git diff stat (8 lines) |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | ops byte-identical | `go test -run TestFilterDeltaParseOnceEquivalence` 12/12 PASS post-refactor (tmp/filter-delta-test-final.log) |
| AC-2/3 | 2 parses per modify | `TestFilterDeltaParseCallCount` PASS; grep shows exactly one `parseFilterAttrs(updateText)` + one `parseFilterAttrs(modifiedText)` per call site |
| AC-4 | dry-run parse-once, output unchanged | `policy_dryrun.go:191-192`; reactor package suite green |
| AC-5 | RFC semantics preserved | plugin suite 404/404 (policy-test-remove-private-as, policy-test-as4path-suppress green) |
| AC-6 | fewer allocs | bench before 34 allocs/op vs after 24 allocs/op (tmp/filter-delta-bench-{before,after}.log) |
| AC-7 | full verification | `make ze-lint-changed` 0 issues; reactor suite green; plugin 404/404; encode 51/51; `make ze-verify` result recorded below |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Modified UPDATE on egress (export filter) | `test/plugin/policy-test-remove-private-as.ci` (existing) | green in `make ze-plugin-test` 404/404 |
| Modified UPDATE on ingress (import filter) | `test/plugin/policy-test-configured-import.ci` (existing) | green in `make ze-plugin-test` 404/404 |
| Dry-run (`show policy test`) | exercised via `policy_dryrun_test.go` + plugin suite | green |

### ze-verify (AC-7)
- `make ze-verify` run 2026-06-10T22:12Z on the shared working tree: all functional, exabgp, lint, and build stages PASS; the only failures are `ze-unit-test-cached` and `ze-unit-test-race-changed`, both on the single test `TestCmdDeactivateLeafListValue` (`internal/component/config/cli`).
- That test is broken by a concurrent session's uncommitted leaf-list work in `internal/component/config/` (parser.go, parser_list.go, serialize.go modified in tree). No import path from `config/cli` to `bgp/reactor`; this spec's changes are confined to `internal/component/bgp/reactor/`, whose suite passes including under -race.
- Full logs: `tmp/ze-verify.log`, `tmp/ze-verify-failures.log`.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated (or N/A)
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments still accurate
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (N/A)
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-filter-delta-parse-once.md`
- [ ] **Commit A:** code + tests + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-filter-delta-parse-once.md`
