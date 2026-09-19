# Spec: followup-pki-chain

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-09-03 |

Both inherited product items are resolved elsewhere. `spec-lg-pki-certificate`
closed in `7ef6960947` on 2026-09-04; the current listener contract is
`docs/architecture/pki/tls-listeners.md`. `buildLGService` consumes
`listenerTLSMaterial`, which resolves a configured name through
`pki.ServerTLSMaterial`. `pki.chainPEM` emits the leaf followed by every stored
intermediate.

This file is a candidate for code-free closure, not another implementation.
The deferral directory was removed on 2026-09-05, so retaining a target for a
deferral shard is no longer a reason to keep it open. Closure still needs its
own review and is not performed by this reconciliation.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `docs/contributing/spec-workflow.md` - current closure requirements
3. `docs/architecture/pki/tls-listeners.md` - the delivered listener and chain contract

## Task

There is no outstanding product scope here. Confirm the two recorded dispositions
for a code-free closure: looking-glass certificate selection was delivered by
`spec-lg-pki-certificate`, and multiple stored intermediates are emitted by the
shared PKI chain assembler. Do not duplicate either implementation.

The July work items and design scaffolding below are retained as historical
planning evidence. Their missing-API and single-intermediate premises were
superseded; they are not instructions to restart implementation.

### Work items (re-homed 2026-07-16 from `plan/deferrals.md`)

- **Looking-glass TLS serves a PKI-stored chain (from spec-pki-full-chain design, 2026-07-10)** -
  `cmd/ze/hub/service_lg.go` keeps the self-signed-only `LoadOrGenerateCert` path. Extend
  it by consuming `pki.ServerTLSMaterial` like web/DoT/DoH. This is a THIRD consumer of the
  same pattern the base spec generalizes; the base spec's own Task says "same pattern applies
  cleanly later".
- **Multi-intermediate chains (from spec-pki-full-chain design, 2026-07-10)** - `intermediate`
  holds a single certificate (`pki/config.go`), so a 4-tier CA (leaf + 2
  intermediates) cannot be expressed. Extend `intermediate` to a list. Single-intermediate
  covers the common case, and the base spec's doctor chain check reports AKI/SKI mismatch, so
  the gap stays visible meanwhile.

-> Constraint: neither item is urgent. Single-intermediate is the common deployment, and the
looking-glass falling back to self-signed is the same behavior it has always had. Do not let
this spec's existence imply the base spec is incomplete without it.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/pki/tls-listeners.md` - what `pki.ServerTLSMaterial` delivers and which consumers it covers
  → Constraint: (fill during research) do NOT design a second chain-assembly path; consume the base spec's.

**Key insights:** (fill during research)

## Current Behavior (MANDATORY)

**Source files read:** (fill during research -- entry points, not yet read)
- [ ] `cmd/ze/hub/service_lg.go` and `service_tls.go` - the delivered looking-glass certificate selection
- [ ] `internal/component/pki/tls.go` - `ServerTLSMaterial` and `chainPEM`, including every stored intermediate

**Behavior to preserve:** self-signed fallback semantics for any listener with no PKI entry
configured; the base spec's chain assembly.

**Behavior to change:** (fill during design)

## Data Flow (MANDATORY)

### Entry Point
(fill during research)

### Transformation Path
1. (fill during research)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| (fill during research) | | [ ] |

### Integration Points
- (fill during research)

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)
- [ ] Registration over hardcoding

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The shared TLS material API is available to the looking glass | `ServerTLSMaterial`, `listenerTLSMaterial`, `buildLGService` | The recorded looking-glass disposition would need reopening | Read the delivered selector and consumer | confirmed by source, 2026-09-19; successor closed in `7ef6960947` |
| A-2 | Multiple intermediates need a new config representation | Original July single-certificate premise | Duplicate implementation of an existing leaf-list | `parseDeviceCert` iterates `GetSlice("intermediate")`; `chainPEM` emits every stored intermediate | broken; the premise was already superseded by the September 3 amendment |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Historical July instructions are mistaken for current implementation scope | Work starts on either resolved item | Use the delivered TLS architecture and successor closure; this holder only awaits closure review |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| (fill during design) | → | (fill during design) | (fill during design) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| (fill during design) | | |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (fill during design) | | | |

### Functional Tests
<!-- Provisional -- confirmed at the DESIGN gate, after the base spec lands. -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `pki-lg-chain` | `test/plugin/pki-lg-chain.ci` | An operator points the looking-glass listener at a PKI store entry and the served chain includes leaf + intermediates, not a self-signed cert. | planned |

## Files to Modify
- (fill during design)

## Implementation Steps
- (fill during design)

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

- **A deferral pointed at a spec dies with that spec.** These two items were parked with the
  prose destination "none yet (small follow-up once pki-full-chain lands)", which is honest
  but unactionable: `commit_helper.py` rejects it, and nothing would have re-raised them once
  the base closed. On 2026-07-16 two other deferrals were found in exactly that state --
  `spec-followup-web-cli-ux` and `spec-fixit-appliance-evidence-config` both closed correctly
  (work done, learned summary, file removed) while leaving live rows naming the deleted files,
  which blocked commits repo-wide until re-homed. A follow-up spec that OUTLIVES the base is
  the structure that survives closure.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| A separate follow-up spec, not extra scope on `spec-pki-full-chain` | Add both items to the base spec | The base spec explicitly bounded itself "to keep the spec reviewable", and it is already `ready` -- expanding a reviewed scope re-opens its design gate. It would also orphan these rows again at its closure. |
| Historical prerequisite: `spec-pki-full-chain` | Leave Depends empty at creation | Both original items consumed that API; the prerequisite is now fulfilled and Depends is clear |

## Known Limitations
- No product work remains assigned here. The historical prerequisite was fulfilled;
  closure review remains separate from this planning correction.

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied

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
- [ ] AC-1..AC-N all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `./le verify worktree` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md` — no failures)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] Abstract when you can (2+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec (with all edits) + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/<spec>` only
