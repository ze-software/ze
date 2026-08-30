# Spec: wire-edit-5-fanout-dedup-deferred-small-fanout-regression -- the 3% cost of dedup when every destination is its own group

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Updated | 2026-08-02 |

Deferral holder created at the closure of the wire-edit-5 fan-out dedup spec on 2026-08-02
(`ai/rules/planning.md`, "Creating the Deferral Spec"). The source spec
was removed by its closure commit, so the work below lives here.

## Task

AC-11 of the wire-edit-5 fan-out dedup spec asked for per-destination cost
"no worse at 1 and 2". It is met at fan-out 2 within one group and NOT met at
fan-out 2 with 2 groups.

Interleaved A/B, medians of 6, nanoseconds per destination:

| Fan-out, groups | Off | On | Delta |
|-----------------|-----|-----|-------|
| 1, 1 | 1632 | 1605 | control |
| 2, 1 | 1338 | 1198 | -10.5% |
| 2, 2 | 1330 | 1374 | **+3.3%** |
| 10, 2 | 1210 | 1036 | -14.4% |
| 100, 2 | 1158 | 827 | -28.6% |
| 100, 100 | 1174 | 1207 | +2.8% |

The cost is the digest that nothing can amortize when no two destinations share a
group. Allocations per operation are identical in both arms at every point.

**No adaptive threshold was added, on purpose.** Any cutoff L silently disables
sharing for G >= L, which is the silent cap `ai/rules/completion.md` and the
wire-edit umbrella both ban. The trade is +3% worst case for -29% best case.

The work here is to find a recovery that does not introduce a silent cap: a
cheaper digest, a digest computed once per edit set rather than per destination,
or a group-count signal the forward loop already has.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress. -->

- [ ] `ai/rules/completion.md` - why a silent cap is banned
- [ ] `docs/architecture/bgp/fanout-dedup.md` - the measurement and why no threshold was added

## Current Behavior (MANDATORY)

**Source files read:** (re-read at design time; verify before trusting)

- [ ] `internal/component/bgp/filterapi/fingerprint.go` (the digest whose cost this is)
- [ ] `internal/component/bgp/reactor/forward_dedup.go` (`fwdDedupTable.begin`, the per-destination call)

**Behavior to preserve:** the identity must stay complete: fingerprint is a hint, full equality decides. No recovery may weaken that.

## Data Flow (MANDATORY)

### Entry Point
`forwardUpdateCore`'s destination loop, once per destination with a non-empty edit set.

### Transformation Path
(fill during design)

### Boundaries Crossed
| From | To | Format |
|------|----|--------|
| (fill during design) | (fill during design) | (fill during design) |

### Integration Points
| Point | Component |
|-------|-----------|
| (fill during design) | (fill during design) |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The digest is the whole 3%, and no other per-destination work was added. | Allocations per operation are identical in both arms at every measured point. | The recovery targets the wrong code and the benchmark does not move. | (fill during design) | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A cheaper digest with a higher collision rate costs more in equality checks than it saves. | (fill during design) | (fill during design) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| One route fanned out to peers that share no policy group | -> | the digest cost is recovered without disabling sharing anywhere | `BenchmarkFanoutDedup` at (2,2) and (100,100) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Fan-out 2 in 2 groups | Per-destination cost is no worse than dedup off |
| AC-2 | Fan-out 100 in 2 groups | The -28.6% win is preserved |
| AC-3 | Any fan-out or group count | Sharing is never silently disabled by a threshold |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| BenchmarkFanoutDedup | `internal/component/bgp/reactor/forward_dedup_bench_test.go` | AC-1, AC-2 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| existing `test/plugin/bgp-rs-fastpath-ebgp-shared.ci` and `test/plugin/nexthop-self.ci` | `test/plugin/` | the wire is unchanged; only per-destination cost moves | |

## Files to Modify
- `internal/component/bgp/filterapi/fingerprint.go`
- `internal/component/bgp/reactor/forward_dedup.go`

## Implementation Steps

1. (fill during design)

## Checklist

### Goal Gates (MUST pass)
- [ ] Every AC demonstrated
- [ ] `./le verify worktree` passes
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
