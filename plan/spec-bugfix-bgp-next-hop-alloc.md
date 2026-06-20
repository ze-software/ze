# Spec: bugfix-bgp-next-hop-alloc

| Field | Value |
|-------|-------|
| Status | done |
| Depends | - |
| Phase | - |
| Updated | 2026-06-20 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/review-bug-review-bgp-engine.md` finding BENG-005
3. `internal/component/bgp/reactor/reactor_api_forward.go`
4. `internal/component/bgp/filterapi/filterapi.go`
5. `ai/rules/buffer-first.md`
6. `ai/rules/memory-architecture.md`
7. `ai/rules/no-sprintf-alloc.md`

## Task

Fix BENG-005 if allocation evidence confirms the escape. IPv6 link-local `next-hop self` forwarding must not allocate a heap slice on every forwarded UPDATE. Preserve wire output while moving the 32-byte next-hop construction into caller-owned storage or an existing pooled modification buffer.

## Required Reading

### Source Finding
- [ ] `plan/review-bug-review-bgp-engine.md` - BENG-005 evidence and allocation caveat
  -> Decision: this is accepted as an actionable performance bug because the hot path contains `make([]byte, 32)`.
  -> Constraint: confirm escape behavior before changing more than the local allocation path.

### Architecture and Rules
- [ ] `ai/rules/no-sprintf-alloc.md`
  -> Constraint: forwarding hot paths must not allocate per route for avoidable byte construction.
- [ ] `ai/rules/memory-architecture.md`
  -> Constraint: use buffer-first ownership and avoid retaining stack slices past their lifetime.
- [ ] `ai/rules/buffer-first.md`
  -> Constraint: caller-owned buffers and pooled writes are preferred over fresh slices.

## Current Behavior

**Source files to read:**
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go:773-856` - `applyNextHopMod` builds next-hop modification values.
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go:819` - IPv6 global plus link-local path allocates `make([]byte, 32)`.
- [ ] `internal/component/bgp/filterapi/filterapi.go:44-90` - `ModAccumulator` stores operation values and determines whether stack-backed bytes can be passed safely.

**Behavior to preserve:**
- IPv4 `next-hop self` output remains unchanged.
- IPv6 global-only next-hop output remains unchanged.
- IPv6 global plus link-local next-hop output remains 32 bytes, global followed by link-local.
- `filterapi.ModAccumulator` lifetime rules remain safe.

**Behavior to change:**
- The per-forward IPv6 link-local branch must not allocate a fresh heap slice if the value can be copied into existing modification storage.
- If `ModAccumulator.Op` retains the passed slice, add or use an API that copies from a stack array immediately.

## Data Flow

### Entry Point
- BGP forwarding applies `next-hop self` for an IPv6 peer that has a link-local address.

### Transformation Path
1. Forwarding chooses a destination peer.
2. `applyNextHopMod` determines next-hop-self output.
3. Fixed code constructs the 32-byte value in stack or pooled storage.
4. `filterapi.ModAccumulator` records a safe copy or reference according to its ownership contract.
5. Forwarding applies the mod while building outbound UPDATE bytes.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| peer address state -> next-hop mod value | `applyNextHopMod` | [ ] byte equality test for global plus link-local IPv6 |
| stack/caller storage -> filter accumulator | `mods.Op` or new helper | [ ] allocation test and race-safe lifetime review |
| mod accumulator -> UPDATE builder | existing mod application | [ ] existing forwarding tests still pass |

### Architectural Verification
- [ ] No heap allocation for the 32-byte next-hop construction on the tested hot path.
- [ ] No unsafe retention of stack memory after `applyNextHopMod` returns.
- [ ] No new string formatting or IP text conversion in forwarding.
- [ ] No protocol output change.

## Risks and Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|------------|-------|----------|--------------|--------|
| A-1 | The `make([]byte, 32)` escapes in the current code | source passes slice to accumulator | if compiler keeps it stack, only a small cleanup may be needed | failing `AllocsPerRun` before fix, passing no-alloc test after fix | confirmed |
| A-2 | `ModAccumulator` can copy from caller-owned storage without broad API changes | accumulator already centralizes mod values | add a narrow `OpCopy` helper | `TestModAccumulator_OpCopyOwnsBuffer`, next-hop no-alloc test | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-1 | Stack array is retained unsafely | corrupted next-hop in test or race/lifetime review fail | copy into accumulator-owned buffer before return |
| R-2 | Allocation moves to a hidden append | `AllocsPerRun` still reports allocations | inspect escape output and accumulator append paths |
| R-3 | Test is brittle due to unrelated setup allocations | noisy allocation count | isolate `applyNextHopMod` or accumulator helper |

## Wiring Test

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| IPv6 peer with `next-hop self` and link-local address | -> | `applyNextHopMod` | `TestApplyNextHopModIPv6LinkLocalNoAlloc` |
| same path with output bytes checked | -> | next-hop mod value | `TestApplyNextHopModIPv6LinkLocalBytes` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | IPv6 local and link-local addresses, `next-hop self` enabled | mod value is 32 bytes, global IPv6 then link-local IPv6 |
| AC-2 | `testing.AllocsPerRun` around the fixed helper | zero heap allocations for value construction, or a documented lower bound for unavoidable accumulator setup |
| AC-3 | IPv4 and IPv6 global-only paths | unchanged output and no new allocations |
| AC-4 | compiler escape analysis | no escaping allocation from local 32-byte buffer path |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestApplyNextHopModIPv6LinkLocalBytes` | `internal/component/bgp/reactor/reactor_api_forward_test.go` | AC-1 | |
| `TestApplyNextHopModIPv6LinkLocalNoAlloc` | same | AC-2, AC-4 | |
| existing next-hop-self tests | existing reactor tests | AC-3 | |

### Benchmarks / Allocation Checks
| Check | Command | Expected |
|-------|---------|----------|
| allocation unit | `go test -run TestApplyNextHopModIPv6LinkLocalNoAlloc ./internal/component/bgp/reactor` | pass |
| optional escape check | `go test -run TestApplyNextHopModIPv6LinkLocalNoAlloc -gcflags=-m ./internal/component/bgp/reactor` | no escape from 32-byte buffer path |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| not required if unit covers exact wire bytes | Go unit | route server forwards IPv6 next-hop-self route | |

### Interop Tests
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| not required | - | - | performance-only change, output bytes covered by unit tests | |

## Files to Modify

- `internal/component/bgp/reactor/reactor_api_forward.go` - remove per-forward heap allocation.
- `internal/component/bgp/filterapi/filterapi.go` only if a safe copy helper is required.
- `internal/component/bgp/reactor/reactor_api_forward_test.go` or existing next-hop tests - byte and allocation coverage.

## Files to Create

- None expected unless no suitable test file exists.

## Implementation Steps

1. Add a focused byte-output test for IPv6 global plus link-local next-hop-self.
2. Add an allocation test around the smallest helper that currently allocates.
3. Inspect `ModAccumulator` ownership and either pass stack storage safely through an immediate-copy helper or write directly into accumulator-owned storage.
4. Re-run byte-output and allocation tests.
5. Run targeted reactor tests and `make ze-lint-changed` after code changes.

## Critical Review Checklist

| Check | What to verify |
|-------|----------------|
| Correctness | next-hop bytes identical before and after the allocation fix |
| Memory | no retained pointer to stack buffer |
| Performance | allocation count reduced on target hot path |
| Scope | no unrelated forwarding policy changes |

## Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| IPv6 link-local byte regression | `go test -run TestApplyNextHopModIPv6LinkLocalBytes ./internal/component/bgp/reactor` |
| allocation regression | `go test -run TestApplyNextHopModIPv6LinkLocalNoAlloc ./internal/component/bgp/reactor` |
| lint | `make ze-lint-changed` |

## Security Review Checklist

| Check | What to look for |
|-------|------------------|
| Memory safety | no stack slice retained after return |
| Route correctness | no next-hop byte order or length regression |
| DoS/GC pressure | hot path no longer allocates per forwarded route |

## Failure Routing

| Failure | Route To |
|---------|----------|
| compiler proves no heap allocation today | downgrade to cleanup note unless benchmark still shows allocs |
| accumulator requires copying by design | implement narrow copy helper rather than unsafe reference |

## Design Insights

- The fast path may still be correct on the wire while violating Ze's allocation contract.

## Core Insight

Constructing fixed-size BGP next-hop bytes should be stack or pool based, never a fresh heap slice per forwarded route.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Confirm allocation before broad changes | immediate refactor | avoids unnecessary churn if escape analysis already eliminates the heap allocation |
| Keep fix local to value construction | redesign mod accumulator | preserves forwarding policy and lowers risk |

## Known Limitations

- Implemented. Allocation evidence confirmed the source-level suspicion before the fix.
- The legacy helper now copies the 32-byte value into accumulator-owned inline storage.

## Implementation Summary

### What Was Implemented
- Added `ModAccumulator.OpCopy` and changed IPv6 global plus link-local next-hop-self construction to stack bytes copied into accumulator-owned storage.

### Bugs Found/Fixed
- BENG-005 documented as an accepted allocation investigation and fix target.

### Documentation Updates
- No user docs required.

### Deviations from Plan
- None.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Create actionable fix plan for BENG-005 | Done | this spec | Includes allocation confirmation gate |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1..AC-4 | Done | `TestApplyNextHopModIPv6LinkLocalBytes`, `TestApplyNextHopModIPv6LinkLocalNoAlloc`, focused reactor tests | 32-byte value correct and zero allocations after accumulator warm-up |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| next-hop byte and allocation tests | Done | `internal/component/bgp/reactor/filter_delta_handlers_test.go` | Passing |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/bgp/reactor/reactor_api_forward.go` | Done | implementation target updated |

### Audit Summary
- Total items: 1 plausible finding accepted into an allocation-confirming fix spec.
- Done: allocation fix and byte/alloc regression tests.
- Partial: none.
- Skipped: no approved scope reduction.
- Changed: `filterapi.ModAccumulator`, next-hop helper, and tests.

## Goal Validation

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| BENG-005 has a safe remediation path | spec artifact | this file names allocation proof, output tests, and memory lifetime checks |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | generated from BENG-005 | `plan/review-bug-review-bgp-engine.md` | fix spec created |

### Fixes applied
- None, review program creates fix specs only.

### Final status
- [ ] `/ze-review` re-run by implementation owner after code changes
- [x] Fix spec contains allocation proof gate and no unsafe stack-retention plan

## Pre-Commit Verification

### Files Exist
| File | Exists | Evidence |
|------|--------|----------|
| `plan/spec-bugfix-bgp-next-hop-alloc.md` | yes | created by review program |

### AC Verified
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| review deliverable | fix spec exists | this file |

### Wiring Verified
| Entry Point | Test | Verified |
|-------------|------|----------|
| IPv6 next-hop-self forwarding | planned unit tests | pending implementation |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1, A-2 | unresolved for implementation | listed in spec |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| No user docs needed | internal performance fix only | yes |
