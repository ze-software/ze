# Spec: perf-next-0-umbrella -- Third Hot-Path Optimization Round

| Field | Value |
|-------|-------|
| Status | blocked |
| Depends | spec-perf-next-2-filter-delta-alloc.md |
| Phase | 5/5 |
| Updated | 2026-08-03 |

## Blocked

Blocked on the same decision by Thomas that blocks child 2. This umbrella
cannot close before its children do, and `spec-perf-next-2-filter-delta-alloc`
is now `blocked` awaiting his answer on Phase B. Children 1 and 3 are complete
(`ebgpWireSlot` in `internal/component/bgp/reactor/received_update.go`,
`Community.AppendText` in `internal/core/bgp/attribute/text_append.go`).

Two of this umbrella's own criteria need the same answer. AC-1 asks for a fresh
`ze-perf-bench PPROF=1` profile and AC-3 for a recorded re-run, but
`ze-perf-bench` exercises none of the three paths this round touched
(`docs/architecture/perf-round-3.md`), and `Dockerfile.ze` was recorded stale
when the round closed.
R-1 in this spec already pre-authorizes per-child Go benchmarks as the proof, so
Thomas can either waive AC-1 and AC-3 under R-1 or ask for `Dockerfile.ze` to be
repaired and the harness run.

Awaiting closure (recorded 2026-07-22 during plan review): all three children
shipped and the round's design record ALREADY EXISTS as
`docs/architecture/perf-round-3.md` (child 1 `ebgpWireSlot` lock-free slots
in `received_update.go,89`; child 2 `filterAttrs`/`filterAttrID` in
`filter_chain.go,79`, Phase B scratch-pool deliberately deferred there;
child 3 `Community.AppendText` in
`internal/core/bgp/attribute/text_append.go`). Remaining work is the
two-commit closure of the umbrella and its three children. Note:
the pol-4 `filter-delta-parse-once` follow-up is PRIOR work, not
child 2's completion signal.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. Child specs: `plan/spec-perf-next-1-ebgp-wire-lockfree.md`, `plan/spec-perf-next-2-filter-delta-alloc.md`, `spec-perf-next-3-rib-show-alloc` (closed)

## Task

Coordinate the third performance optimization round (after campaigns 771 and 859).
A June 2026 audit (3 parallel code audits + 5 research dossiers, cross-verified
against source) identified three remaining evidence-backed hot-path improvements
and several candidates that turned out NOT to be worth doing. This umbrella:

1. Records the measurement baseline and the profiling-first methodology.
2. Lists the three child specs in execution order.
3. Records the **negative findings** so future sessions do not re-investigate them.

### Baseline (ze-perf, 2026-06-05, 100K IPv4/unicast routes, 4 GB VM, darwin/arm64 + Colima)

| DUT | Convergence | Throughput | p99 |
|-----|-------------|------------|-----|
| ze | 62ms +/- 10ms | 1,612,903 r/s | 43ms |
| bird | 65ms +/- 0ms | 1,538,461 r/s | 28ms |

History: 91ms (pre-771) -> 71ms (post-771) -> 62ms (post-859). Remaining gap to
BIRD's best recorded run (44ms) was attributed by the first campaign
to architecture (Go GC vs slab allocation, buffered vs in-place parsing,
socket-layer write coalescing), not to remaining low-hanging fruit.

### Child specs (execution order)

| # | Spec | Target | Expected effect |
|---|------|--------|-----------------|
| 1 | `spec-perf-next-1-ebgp-wire-lockfree.md` | Mutex on every `EBGPWire` cache hit | ~15M lock ops/sec removed at 100K UPDATE/s route-server fan-out |
| 2 | `spec-perf-next-2-filter-delta-alloc.md` | ~24 allocs per filter-modified UPDATE | Roughly halve allocations on the policy-modify path (per destination peer on export) |
| 3 | `spec-perf-next-3-rib-show-alloc.md` | Per-route []string + String() in show/JSON enrichment | Full-table `show bgp rib` drops millions of string allocations per request |

### Methodology (BLOCKING for every child)

1. **Profile before coding.** Run `make ze-perf-bench PERF_DUT=ze PPROF=1` and
   capture CPU + alloc profiles into `tmp/perf-run/pprof`. Campaign 771 rejected
   3 plausible proposals after profiling; the same gate applies here.
   **Scope gate (not a formality):** the three children were designed from audit
   reasoning + arithmetic (15M lock ops/s; 24 allocs/op x fan-out), NOT from a
   fresh top-frame profile. AC-1's profile is therefore a real gate on scope —
   most acutely for child 2 (largest refactor, smallest win). For each child,
   locate its target frames in the captured profile before implementing it; if a
   child's frames are absent near the top, STOP and present the evidence to the
   user before doing that child. Child 1's RS-fan-out win is not expected to
   appear in this single-DUT 100K-route baseline at all — its parallel
   micro-benchmark is the proof, and that is acceptable per R-1/the child spec.
2. **Benchmark gate per child.** Each child defines a Go benchmark asserting the
   before/after allocs/op or ns/op. The benchmark is written FIRST and its
   "before" numbers are pasted into the child spec.
3. **Re-measure after.** Re-run `make ze-perf-bench PERF_DUT=ze` after each child
   lands; record convergence/throughput movement in the child's Implementation
   Summary. Movement within noise is acceptable for child 3 (its path is not the
   convergence path); the Go benchmark is its proof.

### Negative findings (do NOT re-investigate without new evidence)

| Candidate | Verdict | Evidence |
|-----------|---------|----------|
| Engine event dispatch slice copy (`internal/component/plugin/server/engine_event.go`) | NOT hot. No spec. | BGP events never reach engine subscribers; only config-transaction events do (~10-30 handler registrations per config reload, dispatch rate ~0.1/s operational). Verified via `deliverEvent` flow in `internal/component/plugin/server/dispatch.go`. |
| UPDATE builder pooling (the old spec-604 deferral) | ALREADY DONE. | Commit 233ff1726 (2026-04-16). All 14 make() sites eliminated; `GetUpdateBuilder`/`PutUpdateBuilder` pool exists; BuildUnicast measured at ~10 allocs/op. Reactor forward path never used builders. |
| `forward_build.go` pool-fallback make() (lines 278, 352, 376-378) | Deliberate design, keep. | Tiered escalation per-peer pool -> modBufPool -> make only for oversized payload on pool miss; commented `// pool-fallback` at each site. |
| RFC 7606 validation cache (`docs/research/optimisation-findings.md`) | Stale, unmeasured. | Document dated 2025-12-22 pre-dates both campaigns; explicitly requires measurement that was never done. Act only if a fresh profile shows validation frames at the top. |
| `prefixToWire` allocations (`internal/component/bgp/plugins/rib/rib_nlri.go,117`) | Cold path. No change. | Callers are CLI `inject`/`withdraw` one-shots (`rib_commands.go,383`) and tests; not per-route. |
| seqmap compaction (`internal/core/seqmap/seqmap.go`) | Sound design, infrequent. | O(n log n) only when dead > len/2 and len > 256; mutation-tested; not worth latency-quantile work without evidence. |
| Looking-glass error-path JSON (`internal/component/lg/server.go,550`) | Cold (error responses only). | Not worth touching. |

## Required Reading

### Architecture Docs
- [ ] `ai/rules/performance.md` - allocation strategy and pool inventory
  → Constraint: copies happen only at sanctioned boundaries (pool entry, ContextID mismatch, filter modify, JSON for external plugins)
- [ ] `docs/architecture/perf-round-3.md` - the third campaign, and the two before it in outline
  → Decision: profile-first; reject proposals that profiling shows are stack-allocated already
  → Decision: value-type struct keys over interned strings; one commit for bisection safety
- [ ] `mk/perf.mk` - ze-perf-bench / PPROF / report targets
  → Constraint: results land in `test/perf/results/`, profiles in `tmp/perf-run/pprof`

### RFC Summaries (MUST for protocol work)
- [ ] None at umbrella level (children carry their own; child 1 references `rfc/short/rfc4271.md`)

**Key insights:**
- Both prior campaigns delivered exactly what profiling showed and nothing speculative.
- The "sum of small wins" principle: 20ns x 200 peers x 100K UPDATE/s = 400ms CPU/s.

## Current Behavior (MANDATORY)

**Source files read:** (audit evidence behind the child selection)
- [ ] `internal/component/bgp/reactor/received_update.go` - EBGPWire mutex on every call including cache hits (child 1)
- [ ] `internal/component/bgp/reactor/filter_delta.go` - 14 make() sites + map[string]string parse per modified UPDATE (child 2)
- [ ] `internal/component/bgp/plugins/rib/rib_attr_format.go` - per-route []string + String() loops (child 3)
- [ ] `internal/component/bgp/reactor/forward_build.go` - verified pool fallbacks are deliberate (negative finding)
- [ ] `internal/component/plugin/server/engine_event.go` - verified dispatch is cold (negative finding)
- [ ] `internal/component/bgp/message/update_build.go` - verified builder pooling already done (negative finding)

**Behavior to preserve:**
- All wire formats, JSON output shapes, CLI output, and RFC semantics are unchanged by every child.
- `make ze-precommit-verify` green; `make ze-unit-reactor-test-race` green for reactor changes.

**Behavior to change:**
- None user-visible. Performance characteristics only.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Inbound BGP UPDATE wire bytes (children 1, 2); CLI/API `show bgp rib` request (child 3).

### Transformation Path
1. TCP read -> WireUpdate (lazy) -> ReceivedUpdate cached in RecentUpdateCache (child 1 touches the EBGP wire variant cache on this object).
2. Policy filter chain text round-trip -> filter delta parse -> wire attribute ops -> buildModifiedPayload (child 2 touches the parse + encode steps).
3. RIB entry -> route enrichment map -> json.Marshal -> pipe operators (child 3 touches the enrichment step).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Wire <-> reactor cache | WireUpdate referencing pool buffers, BufHandle ownership | [ ] |
| Engine <-> external filter plugin | text UPDATE serialization + RPC (sanctioned copy point) | [ ] |
| RIB <-> CLI/web | map[string]any -> JSON -> pipes | [ ] |

### Integration Points
- Children integrate into existing functions only; no new components, no new registries.

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The 2026-06-05 ze-perf baseline is reproducible on this machine | `test/perf/results/` JSON files | Before/after deltas are noise | Re-run `make ze-perf-bench PERF_DUT=ze` before child 1 | broken (Docker build infra stale: Dockerfile.ze references cmd/ze-test as separate directory; existing June 5 baseline used; per-child Go benchmarks are the proof per R-1) |
| A-2 | No other session lands conflicting reactor changes mid-round, and the round starts from a clean committed base | git status at spec time | Rebase/benchmark churn; before/after deltas and `ze-unit-reactor-test-race` muddied by unrelated in-flight edits | Check `tmp/session/selected-spec` + git log before each child. NOTE at spec time the working tree had ~48 uncommitted files (cos/iface/l2tp/plugin-registry, none in reactor) — run this round on a branch off a committed base so benchmark deltas and the race gate are attributable to the child only | unvalidated |
| A-3 | The negative findings hold (no new callers appeared) | Dossiers dated 2026-06-11 | A "cold" path may have become hot | Fresh grep for callers during each child's audit step | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Micro-wins don't move ze-perf numbers (within noise) | Post-child re-measure shows no delta | Go benchmarks are the per-child proof; ze-perf movement is a bonus for children 1-2 and not expected for child 3 |
| R-2 | Optimization introduces a data race | `make ze-unit-reactor-test-race` failure | Race gate is BLOCKING in children touching reactor |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Umbrella has no feature code of its own; each child carries wiring rows | → | child specs 1-3 | existing test suite per child Wiring Test tables (e.g. TestReceivedUpdate_EBGPWireConcurrent) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Before child 1 starts | Fresh `make ze-perf-bench PERF_DUT=ze PPROF=1` run captured; baseline numbers pasted into this spec; each child's target frames located in the profile (or their absence noted and the child's scope reconsidered with the user per the Methodology scope gate) |
| AC-2 | Each child completes | Child's Go benchmark shows the asserted improvement; child's Review Gate clean |
| AC-3 | All children complete | `make ze-perf-bench PERF_DUT=ze` re-run; final numbers recorded here and in `docs/performance.md` if changed |
| AC-4 | Umbrella closure | Negative-findings table copied into the learned summary so future sessions inherit it |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| Per-child benchmarks and unit tests | see child specs | Child-level proof | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| None at umbrella level (no numeric inputs added) | - | - | - | - |

### Functional Tests
No user-facing behavior change at the umbrella level; existing test suite passes
(`make ze-precommit-verify`) is the umbrella-level functional gate. Children reference the
specific existing `.ci` suites that prove no regression on their paths.

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| existing suite | `test/` (.ci, unchanged) | No regressions across BGP forward/show paths | |

### Interop Tests (MANDATORY for protocol features)
No wire protocol behavior changes in any child; interop not required (children
preserve RFC 4271 semantics byte-for-byte, asserted by existing unit tests).

## Files to Modify
- `internal/component/bgp/reactor/received_update.go` - via child 1
- `internal/component/bgp/reactor/filter_delta.go` - via child 2
- `internal/component/bgp/plugins/rib/rib_attr_format.go` - via child 3
- `docs/performance.md` - regenerate if final ze-perf numbers change

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
| 1 | New user-facing feature? | [ ] no | - |
| 2 | Config syntax changed? | [ ] no | - |
| 3 | CLI command added/changed? | [ ] no | - |
| 4 | API/RPC added/changed? | [ ] no | - |
| 11 | Affects daemon comparison? | [ ] yes, if final numbers move | `docs/performance.md` (regenerated via `make ze-perf-report`) |
| 12 | Internal architecture changed? | [ ] possibly (child 1 cache concurrency note) | `docs/architecture/buffer-architecture.md` per child 1 |

## Files to Create
- `plan/spec-perf-next-1-ebgp-wire-lockfree.md` - child 1 (created with this umbrella)
- `plan/spec-perf-next-2-filter-delta-alloc.md` - child 2 (created with this umbrella)
- `spec-perf-next-3-rib-show-alloc` - child 3 (created with this umbrella, closed 2026-08-12)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + the child being implemented |
| 2. Audit | Re-validate the child's assumptions (A-N) against current source |
| 3. Wiring phase | Child Wiring Test table |
| 4. Implement (TDD) | Child Implementation Phases |
| 5-14 | Per child spec |

### Implementation Phases
1. **Phase: Baseline (MANDATORY FIRST)** - run `make ze-perf-bench PERF_DUT=ze PPROF=1`; paste numbers + top pprof frames here
   - Tests: n/a (measurement)
   - Files: this spec (baseline section)
   - Verify: profile files exist under `tmp/perf-run/pprof`
2. **Phase: Child 1** - implement `spec-perf-next-1-ebgp-wire-lockfree.md`
3. **Phase: Child 2** - implement `spec-perf-next-2-filter-delta-alloc.md`
4. **Phase: Child 3** - implement `spec-perf-next-3-rib-show-alloc.md`
5. **Phase: Re-measure + close** - re-run ze-perf, update docs, write learned summary, close children then umbrella

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every child closed or explicitly deferred with user approval |
| Correctness | Final ze-perf re-run recorded; no regression vs 62ms baseline |
| Rule: no-speculative-features | Negative-findings table untouched (nothing from it implemented) |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Baseline + final ze-perf numbers in spec | grep this file for the results table |
| Three children closed | `ls plan/spec-perf-next-*.md` shows which files remain open |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | No new inputs at umbrella level |

### Failure Routing
| Failure | Route To |
|---------|----------|
| ze-perf baseline not reproducible | STOP; report environment delta to user before children |
| Child benchmark shows no win | Mark child blocked, present evidence, ask user |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| (research phase) update-builder pooling was open work | Done in commit 233ff1726 | Read the update-pool records during research | Child spec dropped before writing |
| (research phase) engine event dispatch was hot | Config-transaction-only, ~0.1/s | Traced deliverEvent callers | Candidate rejected |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights
- An audit agent rating ("CRITICAL") is not evidence; tracing the actual caller
  chain reversed two of five candidates. Caller-chain verification is the gate.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Three small children over one mega-spec | Single combined spec | Matches 859 set pattern; independent files, independent bisection |
| Record negative findings in the umbrella | Drop them silently | Future sessions otherwise re-audit the same cold paths |

## Known Limitations
- This round does not attempt the architectural items (in-place parse, slab/arena
  RIB storage, socket-layer batching) that would close the remaining gap to
  BIRD's 44ms best run. Those need their own spec set with user buy-in on scope.

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
| Reduce remaining hot-path overhead with evidence | benchmark + ze-perf run | [filled at completion] |

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
- [ ] AC-1..AC-4 all demonstrated
- [ ] Wiring Test table complete (per child)
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-standard-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`)
- [ ] Documentation Update Checklist answered Yes/No with source evidence

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
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-perf-next-umbrella.md`
- [ ] **Commit A:** code + tests + docs + spec (with all edits) + learned summary + counter bump
- [ ] **Commit B:** `git rm` of spec only
