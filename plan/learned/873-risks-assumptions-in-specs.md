# 873 -- Risks & Assumptions in Specs

## Context

Assumptions and risks surfaced during spec design were ephemeral. The /ze-spec
gates forced them to be spoken (assumption challenge, Failure Mode Analysis),
but the answers lived only in the gate conversation and died with the session.
The template recorded assumptions only after they broke (Mistake Log "Wrong
Assumptions") and risks only at the very end (Executive Summary "Risks &
observations"), when they could no longer influence the work. The goal was to
capture both at design time, in the spec, and keep them live through
implementation.

## Decisions

- Added a live `## Risks & Assumptions` section to `plan/TEMPLATE.md`:
  Assumptions table (A-N: Basis, If wrong, Validated by, Status) and Risks
  table (R-N: early signal, mitigation).
- Status lifecycle `unvalidated` -> `confirmed` or `broken`; chose a named
  validation method per assumption over a likelihood/impact scoring matrix,
  which was rejected as enterprise-register overhead with no consumer.
- /ze-spec gate concerns MUST be written into the tables, not just spoken;
  /ze-implement validates cheap assumptions during the audit, before coding.
- A broken assumption gets a Mistake Log row and STOPs work if it invalidates
  the approved design; no assumption may be `unvalidated` at Pre-Commit
  Verification ("Assumptions Resolved" table).
- New specs only: the 33 open specs are exempt; the validate-spec.sh check is
  a WARNING, not an ERROR, so existing specs are not blocked mid-edit.

## Consequences

- Executive Summary "Risks & observations" becomes a copy-forward of surviving
  R-N rows instead of an end-of-work invention.
- The validator warning can be promoted to a hard error once the pre-rule spec
  backlog turns over.
- Spec independence improves: gate concerns now survive into implementation
  sessions and compaction.

## Gotchas

- `.claude/skills/*/SKILL.md` are generated; the wiring lives in
  `ai/skills/ze-spec.md` and `ai/skills/ze-implement.md` plus `make ze-ai-sync`.
- `ai/rules/INDEX.md` regeneration produced no diff: it indexes per-file
  summary lines, not section headings, so adding a rule section does not
  change it.

## Files

- `plan/TEMPLATE.md` -- new section, Assumptions Resolved table, Goal Gate line
- `ai/rules/planning.md` -- lifecycle rule, Pre-Spec/Pre-Commit lines
- `ai/skills/ze-spec.md`, `ai/skills/ze-implement.md` -- gate and audit wiring
- `.claude/hooks/validate-spec.sh` -- non-blocking warning
- `ai/INDEX.md` -- discovery row
