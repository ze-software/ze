# Spec Preservation

## Completed Specs Are Institutional Memory

When moving a spec to `docs/plan/done/`, preserve all reasoning, decisions, and architectural context. Completed specs are reference material for future work — strip the scaffolding, keep the knowledge.

## What to Keep

| Content | Why |
|---------|-----|
| Task description | What was built and why |
| Key insights | Hard-won understanding that prevents re-investigation |
| Data flow | How data moves through the system for this feature |
| Design decisions and rationale | Why this approach, not another |
| Integration points | What connects where |
| Boundary crossings | Architectural constraints respected |
| Files modified/created | What was touched |
| References to sub-specs, related specs, architecture docs | Navigation |

## What to Remove

| Content | Why |
|---------|-----|
| Empty audit tables | No longer actionable |
| Unchecked TDD checklists | Work is done |
| Post-compaction recovery instructions | No longer an active spec |
| "BLOCKING" enforcement markers | Already enforced during implementation |
| Blank status columns | Noise in a completed document |

## Principle

**Delete process scaffolding. Preserve knowledge.**

A future developer reading a completed spec should understand what was built, why it was built that way, and how it fits the architecture — without wading through empty checkboxes.
