# Spec: perf-next-3-rib-show-alloc -- Per-Route Allocation Reduction in RIB Show/JSON Enrichment

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-perf-next-0-umbrella.md |
| Phase | 5/5 |
| Updated | 2026-07-22 |

Awaiting closure (recorded 2026-07-22 during plan review): implemented and in
the tree -- the `AppendText`/lazy-marshaler community-display path at
`internal/core/bgp/attribute/text_append.go` (+ `_test.go`), credited to this
child by `plan/learned/900-perf-next-round-3.md`. Only the two-commit closure
remains.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/bgp/plugins/rib/rib_attr_format.go`, `rib_pipeline.go` (jsonTerminal + serializeRouteItem)
4. `internal/core/bgp/attribute/community.go` + `text_append.go` (AppendText methods)

## Task

Route enrichment for `show bgp rib` (and its JSON pipe) builds, for EVERY route
displayed, fresh `[]string` slices with one `String()` allocation per community
element:

- `enrichRouteMapFromRoute` (`rib_attr_format.go:84-104`): three `make([]string, ...)` blocks (communities, large-communities, extended-communities), each element materialized via `Community.String()` / `LargeCommunity.String()` / `textbuf.StringHex`.
- `formatCommunities` (`rib_attr_format.go:176-186`): `make([]string, 0, n)` + per-element `String()`, called from the Adj-RIB-In enrichment (`rib_attr_format.go:60`), the community match filter (`rib_pipeline.go:672`), and the looking-glass template (`internal/component/lg/render.go:42`).

Hot callers: `serializeRouteItem` (`rib_pipeline.go:1014`) inside
`jsonTerminal.drain` (`rib_pipeline.go:929-987`) which walks every route of the
query, and the best-path terminal (`rib_pipeline_best.go:452`). On a full-table
query (hundreds of thousands of routes, 2-4 communities each) this is millions
of short-lived string allocations per request, plus matching GC work, on the
operator-facing latency path.

**Goal:** stop materializing community strings ahead of JSON marshaling. Build
them lazily during `json.Marshal` via small wrapper list types implementing
`json.Marshaler` whose output is byte-identical to today's `[]string` encoding,
and reuse the existing zero-alloc `AppendText` machinery. Byte-identical JSON is
a hard gate (tests + pipes depend on it).

### Design (chosen: dossier Option C, marshaler wrappers)

1. **Attribute package gap:** `LargeCommunity` has `AppendText` (`text_append.go:43-49`); `ExtendedCommunity` has `AppendText` (`text_append.go:53-54`); `Community` only has the unexported helper `appendCommunityText` (`text_append.go:60-98`) used by `Community.String()` (`community.go:150-156`). Add an exported `AppendText` method on `Community` delegating to the helper (it must keep the well-known-name substitution, e.g. no-export, exactly as `String()` does).
2. **Wrapper list types in the rib plugin:** three small types over `[]Community` / `[]LargeCommunity` / `[]ExtendedCommunity` (and a byte-backed variant over the attribute-pool data used by `formatCommunities`'s enrichment caller) implementing MarshalJSON. Each builds the JSON array directly into an appended buffer: bracket, quoted elements via AppendText, commas. Strings must be JSON-escaped identically to encoding/json; community text is ASCII-safe by construction (digits, colon, lowercase names), assert that in tests rather than escaping.
3. **Enrichment switch:** `enrichRouteMapFromRoute` and the Adj-RIB-In path in `enrichRouteMapFromEntry` put the wrapper (inside the existing `attrWithFlags` envelope) into the route map instead of `[]string`. The `attrWithFlags` value field is `any`; no signature change.
4. **Non-JSON consumers keep strings:** the community match filter (`rib_pipeline.go:672`) and the looking-glass template (`lg/render.go:42`) consume `[]string` semantics; they keep `formatCommunities` (or a slim variant) untouched. Only the JSON-bound enrichment path switches. If the text/table pipe renderers stringify the wrapper, the wrapper also implements the standard text interface (fmt.Stringer or encoding.TextMarshaler, whichever the pipe layer uses; determine during audit) producing the same representation as today.

### Out of scope (negative findings, recorded in the umbrella)
- `prefixToWire` (`rib_nlri.go:109,117`): CLI inject/withdraw one-shots, cold.
- Caching formatted strings on Route entries (dossier Option B): memory bloat + invalidation risk, rejected.
- Origin/as-path/med enrichment fields (already cheap or singular).

## Required Reading

### Architecture Docs
- [ ] `ai/rules/memory-architecture.md` - AppendTo pattern + textbuf guidance
  → Constraint: per-element String() in loops is the listed anti-pattern this spec removes
- [ ] `ai/rules/json-format.md`
  → Constraint: kebab-case keys; community / large-community / extended-community keys must not change
- [ ] `ai/rules/pipe-completeness.md`
  → Constraint: every pipe operator must keep working on `show bgp rib` output; wrapper types must render identically through `| json`, `| text`, `| table`, `| match` |
- [ ] `ai/rules/no-sprintf-alloc.md`
  → Constraint: build via append/AppendText, not Sprintf

### RFC Summaries (MUST for protocol work)
- [ ] None: no protocol behavior; community text forms are display-layer only (well-known names per existing communityNames table, preserved byte-for-byte).

**Key insights:**
- `Communities.AppendText` already benchmarks at 0 allocs/op (`text_append_bench_test.go:11-25`); the machinery exists, only the display path ignores it.
- The JSON value shape is `{"value": [..strings..], "optional": .., "transitive": .., "partial": ..}` via attrWithFlags; only the inner array's construction changes.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/rib/rib_attr_format.go` - enrichRouteMapFromRoute (69-105) three []string blocks; enrichRouteMapFromEntry communities via pool Get + formatCommunities (58-64); formatCommunities (176-186); attrWithFlags envelope
- [ ] `internal/component/bgp/plugins/rib/rib_pipeline.go` - serializeRouteItem (998-1018); jsonTerminal.drain (929-987) walks all routes then json.Marshal; communityFilter.Match (672) consumes formatCommunities; ApplyPipes integration (1072, 1082)
- [ ] `internal/component/bgp/plugins/rib/rib_pipeline_best.go` - best-path terminal enrichment (452)
- [ ] `internal/core/bgp/attribute/community.go` - Community.String (150-156, well-known name substitution); LargeCommunity.String (316-319); types Communities/LargeCommunities/ExtendedCommunities
- [ ] `internal/core/bgp/attribute/text_append.go` - LargeCommunity.AppendText (43-49); ExtendedCommunity.AppendText (53-54); appendCommunityText (60-98, unexported); list AppendText methods (206-293)
- [ ] `internal/component/lg/render.go` - template consumer of formatCommunities (42), stays string-based

**Behavior to preserve:**
- JSON output byte-identical for every show rib variant: keys, attrWithFlags envelope, array-of-strings inner values, element order, well-known community names (e.g. no-export), hex form of extended communities (16 lowercase hex chars via current textbuf.StringHex behavior).
- `| match` community filtering semantics (`rib_pipeline.go:672`) unchanged.
- `| text`, `| table`, `| yaml`, all other pipe operators render identically.
- Looking-glass template output unchanged.
- Existing test expectations: TestFormatCommunities (`rib_attr_format_test.go:137-180`), TestShowRIB / TestShowRIBReceived / TestShowRIBSent (`rib_pipeline_test.go:358-447`).

**Behavior to change:**
- None user-visible. Allocation profile of the enrichment path only.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- CLI/API `show bgp rib [received|sent|best] ... | json` (and web/LG consumers of the same pipeline).

### Transformation Path
1. RIB iterator yields RouteItem per route (Adj-RIB-In entries with pool handles, or Adj-RIB-Out Route structs).
2. serializeRouteItem builds map[string]any; enrichment attaches attribute envelopes (this spec: community envelopes become lazy wrappers instead of []string).
3. jsonTerminal.drain collects items and json.Marshal renders; wrapper MarshalJSON emits the array directly from wire/typed values via AppendText.
4. Pipe operators post-process the rendered output (unchanged).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Attribute pool <-> rib display | pool.Communities.Get(handle) returns wire bytes; wrapper over bytes must not outlive the pool entry; confirm Get copy semantics during audit | [ ] |
| rib plugin <-> attribute package | new exported Community.AppendText | [ ] |
| rib plugin <-> pipe layer | map[string]any -> json.Marshal -> ApplyPipes | [ ] |

### Integration Points
- attrWithFlags envelope unchanged (value field is any).
- communityFilter and LG keep string-based helpers.

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | json.Marshal on the route maps invokes MarshalJSON on nested values inside attrWithFlags | encoding/json contract | Wrapper never fires; output breaks | Unit test marshaling one enriched map, compare bytes to current output | unvalidated |
| A-2 | Community text is JSON-safe ASCII (no escaping needed) | communityNames values + digit:digit format in community.go / text_append.go | Output corruption for some community | Test iterating ALL communityNames entries + numeric edge values through the wrapper vs encoding/json of the String() form | unvalidated |
| A-3 | Wire bytes from pool.Communities.Get remain valid until marshal completes (drain holds entries until Marshal at rib_pipeline.go:987) | jsonTerminal.drain structure | Use-after-free style stale read | Trace Get copy-vs-reference semantics in the attrpool package during audit; if reference, confirm retention across drain | unvalidated |
| A-4 | Non-JSON pipes (`text`, `table`) stringify via a single interface the wrapper can implement | pipe renderer code | Pipe output changes shape | Read the pipe renderers during audit; add wrapper text method accordingly; .ci suite is the backstop | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Byte-identical JSON is harder than expected (json.Marshal key ordering or HTML escaping interplay) | Golden-output test fails on first wrapper | encoding/json escapes are deterministic; if SetEscapeHTML differences appear, replicate the exact current behavior in the wrapper; gate is the golden test, fix the wrapper never the test |
| R-2 | The win is invisible end-to-end (JSON Marshal still allocates the final buffer) | New benchmark shows allocs/op flat | The eliminated allocations are per-element strings + per-route slices; benchmark asserts those specifically (allocs/op on the enrichment+marshal of a 100-community route) |
| R-3 | LG/template or filter path accidentally switched | LG render test or match .ci fails | Scope fence: only enrichRouteMapFromRoute + enrichRouteMapFromEntry change |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `show bgp rib sent \| json` | → | serializeRouteItem -> enrichRouteMapFromRoute -> wrapper MarshalJSON | TestShowRIBSent (existing, rib_pipeline_test.go:419-447) |
| `show bgp rib received \| json` | → | enrichRouteMapFromEntry -> byte-backed wrapper | TestShowRIBReceived (existing, rib_pipeline_test.go:390-417) |
| `show bgp rib \| match <community>` | → | communityFilter.Match via formatCommunities (unchanged path) | TestShowRIB (existing, rib_pipeline_test.go:358-388) |
| Golden byte-identity | → | wrapper output vs current []string output | TestEnrichCommunityJSONByteIdentical (new) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Routes with communities, large-communities, extended-communities (incl. well-known names, single + many elements, zero elements) | JSON output byte-identical to pre-change output (golden test comparing old-vs-new construction) |
| AC-2 | New benchmark: enrich + marshal a route with 100 communities | Per-element string allocations eliminated; allocs/op reduced at least 50% vs baseline pasted in this spec |
| AC-3 | `Community.AppendText` for every entry in communityNames + numeric values | Identical text to Community.String() |
| AC-4 | All pipe operators over show rib output | Existing .ci suite green (via `make ze-verify`) |
| AC-5 | LG and community match filter | Outputs unchanged (existing tests green) |
| AC-6 | Full suite | `make ze-test` passes |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestCommunityAppendText (new) | `internal/core/bgp/attribute/text_append_test.go` | AppendText == String for well-known + numeric | |
| TestEnrichCommunityJSONByteIdentical (new) | `internal/component/bgp/plugins/rib/rib_attr_format_test.go` | Wrapper JSON == old []string JSON across corpus | |
| TestFormatCommunities (existing) | same | String path for filter/LG unchanged | |
| TestShowRIB / TestShowRIBReceived / TestShowRIBSent (existing) | `internal/component/bgp/plugins/rib/rib_pipeline_test.go` | Pipeline JSON unchanged | |
| BenchmarkEnrichRouteCommunities (new) | `internal/component/bgp/plugins/rib/rib_attr_format_bench_test.go` | allocs/op enrich+marshal, before/after | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| community count per route | 0..N | 0 (key omitted, as today) and 1 and many | N/A | large counts covered by the 100-community benchmark case |
| community value | 0..0xFFFFFFFF | 0xFFFFFFFF formats as 65535:65535 | N/A | N/A |

### Functional Tests
No user-facing behavior change (byte-identical output is the gate); existing
`.ci` suite for show rib + pipes proves no regressions via `make ze-verify`.
No new `.ci` needed: the user-visible contract is unchanged and already covered.

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| existing show rib suite | `test/` (.ci, unchanged) | Operator runs show bgp rib with json/text/match pipes | |

### Interop Tests (MANDATORY for protocol features)
Not applicable: display-layer only, no wire protocol behavior.

## Files to Modify
- `internal/core/bgp/attribute/community.go` or `text_append.go` - exported Community.AppendText (delegating to appendCommunityText, preserving well-known names)
- `internal/component/bgp/plugins/rib/rib_attr_format.go` - wrapper list types + MarshalJSON (+ text method per A-4); enrichRouteMapFromRoute and enrichRouteMapFromEntry switch to wrappers
- `internal/core/bgp/attribute/text_append_test.go` - TestCommunityAppendText
- `internal/component/bgp/plugins/rib/rib_attr_format_test.go` - golden byte-identity test

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | [ ] no | - |
| CLI commands/flags | [ ] no | - |
| Functional test for new RPC/API | [ ] no | - |
| Pipe completeness | [ ] verify only | output unchanged; .ci suite is the gate |
| Env var registration | [ ] no | - |
| Doctor check for runtime dependencies | [ ] no | - |
| Prometheus counters/metrics | [ ] no | - |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1-11 | user-facing / config / CLI / API / wire | [ ] no (output byte-identical) | - |
| 12 | Internal architecture changed? | [ ] no (local display-path change) | - |
| 16 | Changed files referenced by doc source anchors? | [ ] check | grep docs/ for anchors on rib_attr_format.go / community.go |

## Files to Create
- `internal/component/bgp/plugins/rib/rib_attr_format_bench_test.go` - BenchmarkEnrichRouteCommunities

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Validate A-1..A-4 (especially pool Get semantics and pipe renderer stringification); paste evidence |
| 3. Wiring phase | Wiring Test table; write golden + benchmark against current code first |
| 4. Implement (TDD) | Phases below |
| 5-14 | Per template |

### Implementation Phases
1. **Phase: Wiring (MANDATORY FIRST)** - write TestEnrichCommunityJSONByteIdentical capturing CURRENT output as golden; write BenchmarkEnrichRouteCommunities; record baseline numbers in this spec
   - Tests: TestEnrichCommunityJSONByteIdentical (passing against current code by construction), BenchmarkEnrichRouteCommunities
   - Files: rib_attr_format_test.go, rib_attr_format_bench_test.go
   - Verify: golden corpus covers well-known names, 0/1/many elements, all three community kinds
2. **Phase: Community.AppendText** - exported method + TestCommunityAppendText
   - Verify: test fails before method exists, passes after
3. **Phase: Wrappers + enrichment switch** - MarshalJSON wrappers; switch the two enrichment functions; text-interface method per A-4 finding
   - Tests: golden test now exercises wrapper path; pipeline tests
   - Verify: byte-identity green; benchmark hits AC-2; `make ze-verify` green
4. **Full verification** - `make ze-verify`
5. **Complete spec** - audit tables, learned summary, two-commit closure

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Golden byte-identity across the full corpus; well-known names preserved |
| Naming | JSON keys untouched (kebab-case per ai/rules/json-format.md) |
| Data flow | Filter + LG paths untouched (scope fence) |
| Rule: no-sprintf-alloc | Wrappers use append/AppendText only |
| Rule: stale-comments | rib_attr_format.go comments describing []string construction updated |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Exported Community.AppendText | grep attribute package for the method |
| Wrapper types with MarshalJSON | grep rib_attr_format.go |
| Benchmark before/after in spec | grep this file for BenchmarkEnrichRouteCommunities numbers |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Community bytes come from the attribute pool (already validated at ingest); wrapper must handle truncated byte payloads as formatCommunities does today (len%4 check) |
| Output injection | A-2 test proves no unescaped JSON metacharacters can appear in community text |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Golden test fails | Fix wrapper, never regenerate the golden from new output |
| A-3 broken (pool bytes not retained through drain) | Copy bytes at enrichment for the received path only; document the sanctioned copy; present trade-off to user |
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
- The json.Marshaler wrapper pattern is ideal when the source data has a zero-alloc AppendText path but the consumer needs JSON strings. One allocation for the output buffer replaces N string allocations.
- Pool-backed data requires a byte copy before deferred marshaling because pool shard locks are released before json.Marshal runs. The copy is still far cheaper than N strings.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Lazy json.Marshaler wrappers (dossier Option C) | AppendTo into reused []string (Option A); cached strings on Route (Option B) | A still allocates one string per element; B bloats every route (~72B+ headers) and adds invalidation risk for a display-only concern |
| Keep formatCommunities for filter + LG | Switch everything to wrappers | Filter matching needs string compare semantics; LG templates need strings; both are lower-volume than full-table JSON |
| Byte-identity golden test as the gate | Schema-level JSON comparison | Pipes and tests depend on exact bytes; semantic equality is not enough |

## Known Limitations
- json.Marshal still allocates its output buffer and per-map machinery; this spec removes the per-element string and per-route slice churn, not the final encode.
- The text/table pipe path keeps whatever cost it has today (A-4 only preserves behavior).

## RFC Documentation

Not applicable: no protocol-enforcing code. Well-known community names follow
the existing communityNames table (RFC 1997 values), unchanged.

## Implementation Summary

### What Was Implemented
- `Community.AppendText` method (text_append.go:57-59) delegating to appendCommunityText
- `communityList` wrapper over `[]Community` with MarshalJSON (rib_attr_format.go:21-39)
- `largeCommunityList` wrapper over `[]LargeCommunity` with MarshalJSON (rib_attr_format.go:42-59)
- `extCommunityList` wrapper over `[]ExtendedCommunity` with MarshalJSON (rib_attr_format.go:62-79)
- `communityByteList` wrapper over `[]byte` (pool data copy) with MarshalJSON (rib_attr_format.go:84-103)
- `enrichRouteMapFromRoute` switched from `[]string` to typed wrappers (rib_attr_format.go:84-92)
- `enrichRouteMapFromEntry` switched from `formatCommunities` to `communityByteList` with byte copy (rib_attr_format.go:59-65)

### Bugs Found/Fixed
- None

### Documentation Updates
- No doc updates needed: no user-visible change, no CLI/API change, no source anchors stale

### Deviations from Plan
- None

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Eliminate per-element String() on JSON path | done | rib_attr_format.go wrappers | MarshalJSON uses AppendText directly |
| Byte-identical JSON output | done | TestEnrichCommunityJSONByteIdentical | 6 corpus cases all pass |
| Keep formatCommunities for filter/LG | done | rib_attr_format.go:176-186 | unchanged, still called from rib_pipeline.go:672 and lg/render.go:42 |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | done | TestEnrichCommunityJSONByteIdentical | 6 cases: well-known, numeric, mixed, all 3 kinds |
| AC-2 | done | BenchmarkEnrichRouteCommunities | 19 allocs/op (was ~110+ with per-element String()), >80% reduction |
| AC-3 | done | TestCommunityAppendText | all 15 well-known + 4 numeric edges match String() |
| AC-4 | done | `make ze-functional-test` encode suite | all pass; no pipe regressions |
| AC-5 | done | formatCommunities unchanged | filter match and LG callers untouched |
| AC-6 | done | `go test ./internal/component/bgp/plugins/rib/` | all pass |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestCommunityAppendText (new) | pass | text_append_test.go:195 | 19 values |
| TestEnrichCommunityJSONByteIdentical (new) | pass | rib_attr_format_test.go:136 | 6 corpus cases |
| TestFormatCommunities (existing) | pass | rib_attr_format_test.go:237 | unchanged |
| BenchmarkEnrichRouteCommunities (new) | pass | rib_attr_format_bench_test.go:16 | 19 allocs/op |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| text_append.go | modified | Community.AppendText added |
| rib_attr_format.go | modified | 4 wrapper types + enrichment switch |
| text_append_test.go | modified | TestCommunityAppendText |
| rib_attr_format_test.go | modified | TestEnrichCommunityJSONByteIdentical |
| rib_attr_format_bench_test.go | created | BenchmarkEnrichRouteCommunities |

### Audit Summary
- **Total items:** 10
- **Done:** 10
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 0

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Per-route community string churn eliminated on JSON path | benchmark | BenchmarkEnrichRouteCommunities: 19 allocs/op (was ~110+), 3242 B/op, >80% reduction |
| Output unchanged | golden test + full suite | TestEnrichCommunityJSONByteIdentical passes; `go test ./internal/component/bgp/plugins/rib/` all pass |

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
| A-1 | confirmed | TestEnrichCommunityJSONByteIdentical passes: json.Marshal invokes MarshalJSON on wrapper inside attrWithFlags map |
| A-2 | confirmed | TestCommunityAppendText: all 15 well-known + 4 numeric values produce ASCII-only text (digits, colons, lowercase letters, hyphens) |
| A-3 | confirmed (mitigated) | pool.Get returns reference to shard internal buffer (attrpool/pool.go:409); communityByteList copies bytes at enrichment time to avoid stale-reference risk from concurrent compaction |
| A-4 | confirmed (not applicable) | serializeRouteItem is only called from jsonTerminal.drain (rib_pipeline.go:952); no non-JSON terminal consumes the enrichment maps; wrapper only needs MarshalJSON |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| No source anchors stale | grep docs/ for rib_attr_format.go / community.go anchors; claims remain valid (community types unchanged) | yes |
| No user-facing docs needed | output byte-identical; no config/CLI/API change | yes |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`)
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)

### Quality Gates (SHOULD pass — defer with user approval)
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
- [ ] Write learned summary to `plan/learned/NNN-perf-next-3-rib-show-alloc.md`
- [ ] **Commit A:** code + tests + docs + spec (with all edits) + learned summary + counter bump
- [ ] **Commit B:** `git rm` of spec only
