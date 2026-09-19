# Spec: release-audit-5-plugins-rib

<!-- DESIGN-TIME template: everything that must exist BEFORE code is written.
     The closure half (Implementation Summary, Audit, Goal Validation, Review
     Gate, Pre-Commit Verification, Mistake Log) lives in
     plan/TEMPLATE-CLOSURE.md and is APPENDED by /ze-close at step 1.
     Do not copy it in advance: sections copied 300 lines ahead of their use
     reach closure untouched, the ones created when needed get filled. -->

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | docs |
| Depends | spec-release-audit-1-surface-inventory.md |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-19 |

<!-- Handoff: `verify` splits the work over two sessions -- the implementation session commits and stops at Status `verification`, a later Opus 5 session reviews that commit and closes. `-` closes in the same session. -->

<!-- Scope drives which optional blocks below apply. Say which one this is, so
     an absent section reads as "inapplicable" rather than "skipped".
     The file's DIRECTORY carries the release bucket: plan/immediate/ for a defect
     an operator meets, plan/pre-release/ for work the release cannot go out
     without, plan/ for everything else (plan/README.md). -->

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Audit plugin registry, startup tiers, RIB, route selection, route reflection and plugin lifecycle before the first user-facing release.
The audit domain and deliverable come from the Child Audit Set in
`plan/pre-release/spec-release-audit-0-umbrella.md`. The required output is a
plugin/RIB finding list.

Use the surface inventory in
`plan/pre-release/spec-release-audit-1-surface-inventory.md` and the umbrella's
finding schema. Trace each user entry point to its producer, check positive and
negative evidence, and record impact, reproduction, missing tests, requested
verification and the owning implementation spec for each surviving finding.
Existing findings are dated leads to recheck, not proof of a current defect.
Where no implementation owner exists, record the ownership decision for approval.

This child owns audit evidence only. Product code, test additions, schemas and
user-documentation repairs require separate approved implementation ownership.
Coordinate overlapping security findings with `plan/spec-rbac-audit.md` rather
than duplicating its implementation scope.

`plan/pre-release/spec-release-distribution.md` requires this exact child ID in
its dependency-closure policy before any public stable or nightly activation.
Creation of this skeleton records the already-defined audit scope. Research,
audit execution, findings disposition and closure evidence remain outstanding.
No audit completion or publication approval is claimed here.

## Required Reading

(fill during design)

### Architecture Docs

(fill during design)

### RFC Summaries (Scope: protocol)

(fill during design)

## Current Behavior (MANDATORY)

(fill during design)

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

(fill during design)

### Entry Point

(fill during design)

### Transformation Path

(fill during design)

### Boundaries Crossed

(fill during design)

### Integration Points

(fill during design)

### Architectural Verification

(fill during design)

## Risks & Assumptions

(fill during design)

### Assumptions

(fill during design)

### Risks

(fill during design)

## Blast Radius

(fill during design)

## Wiring Test (MANDATORY -- NOT deferrable)

(fill during design)

## Acceptance Criteria

(fill during design)

## End-to-End User Stories

(fill during design)

## 🧪 TDD Test Plan

(fill during design)

### Unit Tests

(fill during design)

### Boundary Tests (numeric inputs)

(fill during design)

### Functional Tests

(fill during design)

### Interop Tests (Scope: protocol)

(fill during design)

## Files to Modify

(fill during design)

## Files to Create

(fill during design)

### Integration Checklist

(fill during design)

### Documentation Update Checklist (BLOCKING)

(fill during design)

## Implementation Steps

(fill during design)

### Critical Review Checklist

(fill during design)

### Deliverables Checklist

(fill during design)

### Security Review Checklist

(fill during design)

### Failure Routing

(fill during design)

## Design Insights

(fill during design)

## Key Design Decisions

(fill during design)

## Known Limitations

(fill during design)

## RFC Documentation (Scope: protocol)

(fill during design)

## Checklist

(fill during design)

### Pre-Spec Verification (before the design is presented)

(fill during design)

### Goal Gates (MUST pass)

(fill during design)

### TDD

(fill during design)

### Closure

(fill during design)
