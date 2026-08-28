# plan/ -- Backlog and Active Development

This directory is both the backlog and the active work tracker. Specs at
`skeleton` or `design` status are captured intent (they may sit here for a
long time by design); `ready` and `in-progress` specs are current work.

## Contents

| File | Purpose |
|------|---------|
| `spec-<name>.md` | One spec per work item, status in its header table |
| `TEMPLATE.md` | Design-time spec format: everything that must exist BEFORE code |
| `TEMPLATE-CLOSURE.md` | Closure sections (audit, goal validation, review gate, pre-commit evidence), appended by `/ze-implement` at stage 11 |
| `journal/` | One file per problem class, one row per occurrence (`plan/journal/README.md`) |
| `learned/` | The hand-written meta-indexes `RECURRING-PATTERNS.md`, `DESIGN-HISTORY.md`, `HOOK-FRICTION.md` |
| `deferrals/` (sharded per source), `known-failures/` (sharded per failure) | Cross-spec tracking |
| `future/` | Work that does not block the first release. A defect never goes there (`plan/future/README.md`) |

## Lifecycle

Statuses: `skeleton` -> `design` -> `ready` -> `in-progress` -> closed.
`blocked` and `deferred` are parking states. The full workflow rules live in
`ai/rules/planning.md`; the spec format lives in `plan/TEMPLATE.md`.

`skeleton` is the one status allowed to carry template placeholders: a deferral
holder fills `## Task` and leaves the rest for whoever picks the work up
(`ai/rules/planning.md`). From `design` onward the native validation hook in
`internal/le/hookruntime/lifecycle.go` blocks placeholders, because the author
is then claiming those sections are written.

A spec that passes its Review Gate is not done until it is **deleted** from
`plan/`: closure is two commits (commit A: code + spec + the problem record;
commit B: `git rm` the spec). The problem record is a row in
`plan/journal/<class>.md` whose `Spec` cell names the spec
(`plan/journal/README.md`). There is no `done/` directory.

## Working With Specs

- `/ze-status` shows a cross-project attention view (statuses, stalls).
- `/ze-spec` creates or evolves a spec; `/ze-implement` executes one;
  `/ze-review` runs the completion gate.
- Each session records its spec with `./le spec-session claim spec <stem>`
  (see `ai/rules/planning.md`).
