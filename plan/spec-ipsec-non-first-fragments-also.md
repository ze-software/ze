# Spec: ipsec-non-first-fragments-also

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-21 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

RFC 4301 Section 7.3 negotiates whether non-initial fragments may travel on a tunnel
mode SA whose selector names ports. Ze negotiates port selectors
(`internal/component/ike/engine/ts_narrow.go`) and installs them as exact-port XFRM
selectors (`internal/component/ike/dataplane/xfrm_linux.go::xfrmSelectorPort`), but it
never sends, parses or answers the IKE notify `NON_FIRST_FRAGMENTS_ALSO` (16395,
`internal/component/ike/wire/payload_notify.go` names the number and nothing produces
or consumes it). The feature is the notify on both sides, and the dataplane state that
makes Ze's own behavior match what it negotiated. It is one feature, so it is one spec
(owner ruling, 2026-09-21: every MUST is a requirement; a feature Ze does not offer is
scheduled here so the owner can decline it in one word).

| Requirement | RFC text (verbatim) | Producer or absence |
|---|---|---|
| RFC4301-7.3-1 | "Implementations that will transmit non-initial fragments on a tunnel mode SA that makes use of non-trivial port (or ICMP type/code or MH type) selectors MUST notify a peer via the IKE NOTIFY NON_FIRST_FRAGMENTS_ALSO payload." (Section 7.3) | No producer in `internal/component/ike/wire` or `engine` builds the notify. Whether Linux XFRM transmits a non-initial fragment on an exact-port policy Ze installs is unverified; the spec's first step establishes it in the QEMU lab before choosing whether Ze sends the notify |
| RFC4301-7.3-2 | "The peer MUST reject this proposal if it will not accept non-initial fragments in this context." (Section 7.3) | No parser for the notify; an inbound `NON_FIRST_FRAGMENTS_ALSO` is ignored. RFC 7296 Section 3.10.1 makes the rejection the absence of the notify in the response, so the proof is a response builder that never echoes it unless Ze accepts such fragments |

RFC4301-7.3-3, RFC4301-7.3-4 and RFC4301-7.3-5 are proven at Ze's boundary in
`internal/component/ike/dataplane/rfc4301_boundary_linux_test.go` (the exact-port
selector Ze installs is what the kernel refuses a portless fragment against); this
spec's QEMU step is what turns that boundary proof into an observed drop.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` - the policy selector Ze installs

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4301.md` - Section 7.3
- [ ] `rfc/short/rfc7296.md` - Section 3.10.1, the notify's status semantics

## Current Behavior (MANDATORY)

- [ ] `internal/component/ike/wire/payload_notify.go` - names 16395 and nothing builds or reads it
- [ ] `internal/component/ike/dataplane/xfrm_linux.go` - `xfrmSelectorPort` installs one exact port

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
Skeleton: filled at design.

### Transformation Path
Skeleton: filled at design.

### Boundaries Crossed
Skeleton: filled at design.

### Integration Points
Skeleton: filled at design.

### Architectural Verification
Skeleton: filled at design.

## Risks & Assumptions

### Assumptions
Skeleton: filled at design.

### Risks
Skeleton: filled at design.

## Blast Radius

Skeleton: filled at design.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| CREATE_CHILD_SA with a port selector | → | the notify builder and parser | `TestRFC4301NonFirstFragmentsAlsoIsNegotiated` |

## Acceptance Criteria

| AC | Criterion | Assertion |
|----|-----------|-----------|
| AC-1 | RFC4301-7.3-1 and RFC4301-7.3-2 each carry a tagged positive and negative test, or the owner's decline on their rows | `./le rfc check` prints no line for them |

## End-to-End User Stories

Skeleton: filled at design.

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC4301NonFirstFragmentsAlsoIsNegotiated` | `internal/component/ike/engine/rfc4301_non_first_fragments_test.go` | the notify is sent only when Ze will transmit such fragments and is never echoed when Ze will not accept them | |

### Boundary Tests (numeric inputs)
Skeleton: filled at design.

### Functional Tests
Skeleton: filled at design.

### Interop Tests (Scope: protocol)
Skeleton: filled at design.

## Files to Modify

- `internal/component/ike/wire/payload_notify.go` - the notify builder and parser
- `internal/component/ike/engine/child.go` - where the notify joins the Child SA exchange
- `rfc/short/rfc4301.md` - the two rows lose their `{gap}`

## Files to Create

### Integration Checklist
Skeleton: filled at design.

### Documentation Update Checklist (BLOCKING)
Skeleton: filled at design.

## Implementation Steps

Skeleton: filled at design.

### Critical Review Checklist
Skeleton: filled at design.

### Deliverables Checklist
Skeleton: filled at design.

### Security Review Checklist
Skeleton: filled at design.

### Failure Routing
Skeleton: filled at design.

## Design Insights

Skeleton: filled at design.

## Key Design Decisions

Skeleton: filled at design.

## Known Limitations

Skeleton: filled at design.

## RFC Documentation (Scope: protocol)

`rfc/short/rfc4301.md` rows RFC4301-7.3-1 and RFC4301-7.3-2.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test (N-A when Scope is tooling or docs, which delete that section)
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied: lint fixed rather than disabled, the focused check for the changed behavior run once with its OUTPUT PASTED, and any red named with the one-line reason it is scaffolding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script (commit A preserves the spec in history)
