# Spec: bugfix-bgp-forward-split-context

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | - |
| Phase | - |
| Updated | 2026-06-19 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/review-bug-review-bgp-engine.md` finding BENG-002
3. `docs/architecture/core-design.md:496-600`
4. `internal/component/bgp/reactor/forward_body.go`
5. `internal/component/bgp/reactor/reactor_api_forward.go`
6. `internal/component/bgp/reactor/forward_rs.go`
7. `internal/component/bgp/message/update_build.go`
8. `rfc/short/rfc8654.md`, `rfc/short/rfc7911.md`, `rfc/short/rfc6793.md`

## Task

Fix BENG-002. Oversized BGP UPDATE forwarding must not split raw source-context bytes before converting to the destination peer's negotiated encoding context. Context conversion, filtering, and message-size splitting must be equivalent across slow cache-forward, DirectBridge, and RS inline fast paths.

## Required Reading

### Architecture Docs
- [ ] `plan/review-bug-review-bgp-engine.md` - source finding and regression plan
  -> Decision: BENG-002 is a protocol-corruption blocker.
  -> Constraint: same ContextID remains the only zero-copy forwarding condition.
- [ ] `docs/architecture/core-design.md:536-578` - forwarding path parity
  -> Decision: slow path, DirectBridge, and RS inline path must share semantic invariants.
  -> Constraint: egress filters, AS-PATH/next-hop policy, and context encoding must be equivalent.
- [ ] `ai/rules/buffer-first.md` and `ai/rules/memory-architecture.md`
  -> Decision: fix must not allocate parsed structures per destination when same-context zero-copy is valid.
  -> Constraint: use pooled/caller-owned buffers and existing splitter/build patterns.

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc8654.md` - Extended Messages and peer max message size
  -> Constraint: outbound UPDATE chunks must honor recipient max message size.
- [ ] `rfc/short/rfc7911.md` - ADD-PATH negotiation
  -> Constraint: path IDs are only present when negotiated for the destination.
- [ ] `rfc/short/rfc6793.md` - ASN4 encoding
  -> Constraint: AS_PATH width must match destination ASN4 capability.

**Key insights:**
- `buildFwdBody` checks `updateSize > maxMsgSize` before checking source and destination context IDs.
- The oversized branch calls `wireu.SplitWireUpdate` with `srcCtx`, then forwards split payloads as raw bodies.
- The context conversion branch is skipped entirely for oversized updates.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/forward_body.go:40-54` - oversized branch splits source-context wire bytes and appends raw payloads.
- [ ] `internal/component/bgp/reactor/forward_body.go:56-96` - non-oversized branch checks ContextID and reparses/re-encodes when contexts differ.
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go:632-646` - slow and DirectBridge shared forwarding path writes raw bodies or parsed updates.
- [ ] `internal/component/bgp/reactor/forward_rs.go:416-438` - RS inline path uses the same body result.

**Behavior to preserve:**
- Same-context, in-size forwarding remains zero-copy.
- Same-context oversized forwarding may split raw wire bytes when recipient context matches.
- Egress filters, next-hop policy, AS-PATH handling, send-community filtering, retain/release, and supersede behavior remain unchanged.

**Behavior to change:**
- If `srcCtxID != destCtxID`, forwarding must parse and re-encode for destination context before splitting.
- Oversized split output must be encoded for the destination context and max message size.
- Slow path, DirectBridge, and RS inline path must share the fix.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Plugin or route-server forwards a cached UPDATE to a destination peer whose negotiated context differs from the source.

### Transformation Path
1. Reactor receives UPDATE and stores `WireUpdate` plus `SourceCtxID`.
2. Forwarding chooses destination peer and max message size.
3. Egress filters and modifications are applied.
4. Fixed code checks context before raw splitting.
5. If contexts differ, UPDATE is parsed once and re-encoded or split for destination context.
6. Peer sends destination-context wire bytes.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| cached source wire -> destination peer wire | `buildFwdBody` | [ ] ADD-PATH mismatch test decodes destination output with no path IDs |
| DirectBridge -> reactor forwarding | `ForwardUpdatesDirect` | [ ] DirectBridge test uses same helper path |
| RS inline -> reactor forwarding | `reactorForwardRS` | [ ] RS fast path test or shared helper unit covers same branch |

### Integration Points
- `internal/component/bgp/reactor/forward_body.go`
- `internal/component/bgp/reactor/reactor_api_forward.go`
- `internal/component/bgp/reactor/forward_rs.go`
- `internal/component/bgp/message` splitter/builders
- `internal/component/bgp/context` registry

### Architectural Verification
- [ ] No bypassed layers: all forwarding paths use the same context-aware body builder.
- [ ] No unintended coupling: no plugin-specific logic in core forwarding.
- [ ] No duplicated functionality: split/re-encode behavior is shared.
- [ ] Zero-copy preserved where applicable: same-context raw forwarding remains raw.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|------------|-------|----------|--------------|--------|
| A-1 | Existing parsed update splitter can encode destination ADD-PATH state correctly | `message.Splitter` and `addPathForUpdate` path | new writer needed | ADD-PATH mismatch unit test | unvalidated |
| A-2 | One parse per source wire can be cached across destination peers | existing `fwdParseCache` | performance regression | allocation and benchmark comparison | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Fix regresses same-context zero-copy | same-context tests show parsed updates used | keep context check before parse for same-context case |
| R-2 | Re-encoded split exceeds max size | test sees chunk over destination max | split after destination encoding and assert size |
| R-3 | AS4/ASN2 conversion loses attributes | ASN4 mismatch test fails | use existing context-aware attribute writers |

## Wiring Test (MANDATORY - NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| DirectBridge forward with ADD-PATH source and non-ADD-PATH destination | -> | context conversion before split | `TestForwardSplitConvertsAddPathContext` |
| slow cache-forward with ASN4 source and ASN2 destination | -> | context conversion before split | `TestForwardSplitConvertsASN4Context` |
| same-context oversized UPDATE | -> | raw split still works | `TestForwardSplitSameContextKeepsRawSplit` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Oversized cached UPDATE with ADD-PATH in source context forwarded to destination without ADD-PATH | outbound chunks decode with destination context and contain no path IDs |
| AC-2 | Oversized cached UPDATE with ASN4 source forwarded to ASN2 destination | AS_PATH/AS4_PATH encoding matches destination capability rules |
| AC-3 | Oversized cached UPDATE with identical source and destination ContextID | zero-copy raw splitting remains available |
| AC-4 | Slow path, DirectBridge, and RS inline path all use fixed semantics | tests cover shared helper or each entry point |
| AC-5 | Every outbound chunk size is <= destination max message size | boundary assertion in tests |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Route server forwards large ADD-PATH UPDATE to a non-ADD-PATH peer | receive -> cache -> forward -> context convert -> split -> send | `TestForwardSplitConvertsAddPathContext` |
| 2 | Plugin forwards cached UPDATE through DirectBridge to a lower-capability peer | DirectBridge -> `forwardUpdateCore` -> send | `TestForwardSplitConvertsASN4Context` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestForwardSplitConvertsAddPathContext` | `internal/component/bgp/reactor/forward_body_test.go` | AC-1, AC-4, AC-5 | |
| `TestForwardSplitConvertsASN4Context` | same | AC-2 and AC-5 | |
| `TestForwardSplitSameContextKeepsRawSplit` | same | AC-3 | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| BGP message size | 19..4096 or 19..65535 | destination max | 18 | max+1 |
| ADD-PATH path ID | uint32 | max uint32 | N/A | absent when not negotiated |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| route-server forwarding regression if existing harness can generate large UPDATE | `test/plugin/` or `test/interop/scenarios/` | peer receives destination-context chunks | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| Large UPDATE forwarding with different ADD-PATH capability | `test/interop/scenarios/` if feasible | FRR or GoBGP | peer accepts forwarded chunks without NOTIFICATION | |

### Future (if deferring any tests)
- No deferral approved. At minimum, Go tests must prove byte-level destination-context output.

## Files to Modify

- `internal/component/bgp/reactor/forward_body.go` - reorder context conversion and split logic.
- `internal/component/bgp/reactor/forward_body_test.go` or existing forwarding tests - regression coverage.
- Possibly `internal/component/bgp/message` splitter helpers if destination-context splitting needs a shared API.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | |
| CLI commands/flags | No | |
| Functional test for new RPC/API | No | existing forwarding API path |
| Env var registration | No | |
| Doctor check for runtime dependencies | No | |
| Prometheus counters/metrics | No | |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | bug fix |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | Yes | RFC behavior corrected, code comments required |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | Yes | RFC comments in changed code |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | unless forwarding algorithm doc changes |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or runtime inventory changed? | No | |
| 16 | Any changed source file is referenced by existing doc source anchors? | Yes | grep docs for `forward_body.go` and changed files |
| 17 | Existing docs show config/CLI/API examples for this area? | No | |

## Files to Create

- `internal/component/bgp/reactor/forward_body_test.go` if no suitable test file exists.

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | current forwarding tests and helper contracts |
| 3. Wiring phase | failing helper/entry tests |
| 4. Implement | context-aware split fix |
| 5. Review gate | critical review |
| 6. Full verification | targeted tests, `make ze-test-bgp`, final gate |

### Implementation Phases
1. **Phase: failing tests** - construct oversized source-context wires and assert destination decode behavior.
2. **Phase: helper fix** - change `buildFwdBody` so context mismatch re-encodes before split.
3. **Phase: parity** - cover slow, DirectBridge, and RS inline shared behavior.
4. **Phase: verification** - run targeted and BGP component tests.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Correctness | destination context controls encoding before size split |
| Performance | same-context zero-copy not regressed |
| RFC | ADD-PATH, ASN4, and max message size rules cited |
| Tests | boundary at destination max covered |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| AddPath mismatch regression | `go test -run TestForwardSplitConvertsAddPathContext ./internal/component/bgp/reactor` |
| ASN4 mismatch regression | `go test -run TestForwardSplitConvertsASN4Context ./internal/component/bgp/reactor` |
| Same-context regression | `go test -run TestForwardSplitSameContextKeepsRawSplit ./internal/component/bgp/reactor` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Peer corruption | no source-context bytes sent to destination mismatch |
| Resource use | no avoidable per-peer allocations for same-context forwarding |
| DoS | oversized updates still capped by destination max |

### Failure Routing
| Failure | Route To |
|---------|----------|
| existing splitter cannot handle destination context | add context-aware split API in message package |
| RS inline path needs separate handling | keep shared helper semantics and add RS test |

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

- Size splitting is not safe until the output encoding context is known.

## Core Insight

Context conversion comes before chunking unless source and destination ContextID are identical.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Fix in shared body builder | patch each forwarding caller | all three forwarding paths depend on the same invariant |

## Known Limitations

- This spec does not implement the fix.

## RFC Documentation

Add RFC comments for size, ADD-PATH, and ASN4 enforcement changed by the fix.

## Implementation Summary

### What Was Implemented
- Fix spec only. Production code is unchanged.
- The spec captures BENG-002 from `plan/review-bug-review-bgp-engine.md`.
- Regression expectations cover destination-context conversion before oversized splitting across slow, DirectBridge, and RS inline paths.

### Bugs Found/Fixed
- BENG-002 documented for implementation.
- No production bug was fixed by this review program.

### Documentation Updates
- No user docs required for the fix-spec artifact.

### Deviations from Plan
- None.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Create actionable fix plan for BENG-002 | done | this spec | context conversion and split ordering covered |
| Include parity regression plan | done | Wiring Test and TDD sections | slow, DirectBridge, and RS inline semantics covered |
| Include RFC-linked constraints | done | RFC sections | ADD-PATH, ASN4, and max-message-size constraints named |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1..AC-5 | planned | acceptance criteria table | to be satisfied by implementation owner |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| ADD-PATH context mismatch split | planned | `internal/component/bgp/reactor` | not run by review program |
| ASN4 context mismatch split | planned | `internal/component/bgp/reactor` | not run by review program |
| same-context raw split preservation | planned | `internal/component/bgp/reactor` | not run by review program |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/bgp/reactor/forward_body.go` | planned | implementation target |
| `internal/component/bgp/reactor/forward_body_test.go` | planned | test target |
| `internal/component/bgp/message` splitter helpers | possible | only if destination-context split needs helper change |

### Audit Summary
- Total items: 1 accepted finding converted to a fix spec.
- Done: fix spec created.
- Partial: implementation pending by design.
- Skipped: no production code changes in review program.
- Changed: new spec file.

## Goal Validation
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Destination-context forwarding for oversized updates | spec artifact | this file names source evidence, ACs, RFC constraints, and regression tests for BENG-002 |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | BLOCKER | BENG-002 oversized forwarding bypasses destination context conversion | child 3 report | fix spec created |

### Fixes applied
- None, review program creates fix specs only.

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| - | - | Fix-spec artifact now has summary, audit, goal validation, review gate, and pre-commit sections | this spec | no action |

### Final status
- [ ] `/ze-review` re-run by implementation owner after code changes
- [x] Fix spec contains source evidence, RFC constraints, ACs, and regression plan

## Pre-Commit Verification

### Files Exist
| File | Exists | Evidence |
|------|--------|----------|
| `plan/spec-bugfix-bgp-forward-split-context.md` | yes | created by review program |

### AC Verified
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| review deliverable | fix spec exists | this file |

### Wiring Verified
| Entry Point | Test | Verified |
|-------------|------|----------|
| slow cache-forward | planned unit tests | pending implementation |
| DirectBridge forwarding | planned unit tests | pending implementation |
| RS inline forwarding | planned shared-helper or entry test | pending implementation |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1..A-2 | unresolved for implementation | listed in spec |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| RFC comments required later | protocol forwarding fix spec | yes |
