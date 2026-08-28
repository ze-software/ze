# Spec: followup-pki-chain

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | spec-pki-full-chain |
| Phase | - |
| Updated | 2026-07-16 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-pki-full-chain.md` - the base spec this extends (Status: ready)
4. `plan/deferrals.md` - the two rows that point here

## Task

`spec-pki-full-chain` deliberately bounded itself to two TLS consumers (web/API HTTPS,
and the dnsserver DoT/DoH listeners used by as112 and geodns) "to keep the spec
reviewable". Two extensions were deferred out of it on 2026-07-10 and had **prose
destinations** ("none yet (small follow-up once pki-full-chain lands)"), which
`commit_helper.py` rejects as "live deferrals without a destination spec". This spec is
that destination. It exists so the items survive the base spec's closure -- pointing them
at `spec-pki-full-chain` would orphan them again the moment it is `git rm`-ed, which is
exactly how the web-cli-ux and appliance-evidence deferrals were lost (see Design Insights).

**Blocked on `spec-pki-full-chain`.** Both items extend machinery that spec introduces
(`pki.ServerTLSMaterial`, chain assembly, per-listener YANG referencing a PKI entry).
Starting before it lands means designing against an API that does not exist yet.

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
- [ ] `plan/spec-pki-full-chain.md` - the base: what `pki.ServerTLSMaterial` will look like and which consumers it covers
  → Constraint: (fill during research) do NOT design a second chain-assembly path; consume the base spec's.

**Key insights:** (fill during research)

## Current Behavior (MANDATORY)

**Source files read:** (fill during research -- entry points, not yet read)
- [ ] `cmd/ze/hub/service_lg.go` - the looking-glass TLS listener still on `LoadOrGenerateCert` (:78)
- [ ] `internal/component/pki/config.go` - `intermediate` as a single certificate (:147-158)

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
| A-1 | `spec-pki-full-chain` will land a reusable `pki.ServerTLSMaterial` that a third consumer can adopt without redesign | That spec's Task names it and generalizes across two consumers already | If the base lands a web-specific shape, the looking-glass item becomes a refactor, not an adoption | Read the base spec's delivered API at its closure | unvalidated |
| A-2 | Extending `intermediate` to a list is backwards-compatible for existing single-intermediate configs | `pki/config.go` currently parses one certificate | If YANG cannot express both forms compatibly, existing configs break on upgrade | Read the YANG leaf and the parser before designing | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | This spec is started before the base lands and designs against a hypothetical API | Design references symbols that do not exist yet | `Depends: spec-pki-full-chain` is set; do not move past `skeleton` until the base closes |

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
| `Depends: spec-pki-full-chain` | Leave Depends empty | Both items consume machinery the base introduces; starting first means designing against an API that does not exist. |

## Known Limitations
- Cannot start until `spec-pki-full-chain` lands. Until then this file exists to hold the
  items, not to be worked.

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
- [ ] `./le verify current mode full` passes (lint + all ze tests)
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
