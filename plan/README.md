# plan/ -- Backlog and Active Development

This directory is both the backlog and the active work tracker. Specs at
`skeleton` or `design` status are captured intent (they may sit here for a
long time by design); `ready` and `in-progress` specs are current work.

## Contents

| File | Purpose |
|------|---------|
| `spec-<name>.md` | One spec per work item, status in its header table |
| `TEMPLATE.md` | Spec format: status taxonomy, checklists, Review Gate |
| `learned/` | Learned summaries of completed specs (`NNN-<name>.md`) plus the meta-indexes `RECURRING-PATTERNS.md`, `DESIGN-HISTORY.md`, `HOOK-FRICTION.md` |
| `deferrals/` (sharded per source), `known-failures.md` | Cross-spec tracking |

## Lifecycle

Statuses: `skeleton` -> `design` -> `ready` -> `in-progress` -> closed.
`blocked` and `deferred` are parking states. The full workflow rules live in
`ai/rules/planning.md`; the spec format lives in `plan/TEMPLATE.md`.

A spec that passes its Review Gate is not done until it is **deleted** from
`plan/`: closure is two commits (commit A: code + spec + learned summary;
commit B: `git rm` the spec). Completed knowledge survives as
`plan/learned/NNN-<name>.md`, indexed in `ai/LEARNED-INDEX.md`. There is no
`done/` directory.

## Working With Specs

- `/ze-status` shows a cross-project attention view (statuses, stalls).
- `/ze-spec` creates or evolves a spec; `/ze-implement` executes one;
  `/ze-review` runs the completion gate.
- Each session records its spec in its own marker via
  `scripts/dev/spec-session.sh` (see `.claude/rules/planning.md`).
