# Spec: config-fmt preserves operator information

<!-- DESIGN-TIME template: everything that must exist BEFORE code is written.
     The closure half (Implementation Summary, Audit, Goal Validation, Review
     Gate, Pre-Commit Verification, Mistake Log) lives in
     plan/TEMPLATE-CLOSURE.md and is APPENDED by /ze-close at step 1.
     Do not copy it in advance: sections copied 300 lines ahead of their use
     reach closure untouched, the ones created when needed get filled. -->

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | cli, config |
| Depends | - |
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

Make `ze config fmt -w` safe for an operator's file. This task inherits G-2
and AC-4 from `plan/pre-release/spec-ze-config-fmt.md`: formatting must never
lose information the file carried. Preserve all comments and their association
with configuration statements, preserve configuration meaning, and make a
second formatting pass byte-identical to the first.

The current `tokenizer.skipWhitespaceAndComments`
(`internal/component/config/tokenizer.go`) discards comments before the tree is
built. `cmdFmt` (`internal/component/config/cli/cmd_fmt.go`) serialises that tree
and writes it over the operator's file. The design must address that source of
information loss and investigate the other information the current tree does
not represent. It must reuse the existing formatter rather than add a second one.

Retain the existing command actions, exit-code contract and supported input
forms. Prove preservation through operator-visible writeback, including comments
attached to statements and parse/format/parse equivalence. The preservation
mechanism remains to be designed; no implementation or passing evidence is
claimed by this skeleton.

The parent keeps canonical spelling, serializer consolidation and corpus-carrier
gate policy. Its gate must not recommend writeback until this defect's
preservation evidence is available. Those policy choices cannot waive G-2.
This split records immediate-defect ownership without authorising new scope.

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
