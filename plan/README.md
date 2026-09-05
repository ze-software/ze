# plan/ -- Backlog and Active Development

This directory is both the backlog and the active work tracker. One spec is one
work item, and its status lives in its own header table.

## The three buckets

A spec's directory says what it costs the FIRST RELEASE to leave it undone. The
test is a question about the shipped binary, never about how far along the work
is: a `skeleton` in `immediate/` outranks an `in-progress` spec here.

| Directory | The test it passes | Count on 2026-09-05 |
|-----------|--------------------|---------------------|
| `plan/immediate/` | An operator on the first release meets this as a bug or a missing answer | 77 |
| `plan/pre-release/` | No operator meets it, but the release cannot go out until it is done | 27 |
| `plan/` (this level) | The release goes out without it | 173 |

`immediate/` is wire correctness, config fidelity, authentication, routing
correctness, a crash, and a CLI surface that answers wrongly. `pre-release/` is
packaging, appliance boot, onboarding documentation, release audit, and the RFC
evidence Ze owes a reader outside this repository. Everything else sits here.

**Moving a defect out of `immediate/` to shrink its count is banned.** The count
measures the release, and a defect that moves still ships. `ai/rules/completion.md`
governs it: recording a problem is never addressing it.

A spec moves between buckets when the owner re-reads the test above, and the move
is a relocation rather than a closure. `./le commit create` enforces that
difference, so a triage sweep cannot bank a closure it did not earn.

## Contents

| File | Purpose |
|------|---------|
| `spec-<name>.md` | One spec per work item, status in its header table |
| `immediate/`, `pre-release/` | The two buckets above, same spec format |
| `TEMPLATE.md` | Design-time spec format: everything that must exist BEFORE code |
| `TEMPLATE-CLOSURE.md` | Closure sections, appended by `/ze-implement` at stage 11 |
| `journal/` | One file per problem class, one row per occurrence (`plan/journal/README.md`) |
| `learned/` | The hand-written meta-indexes `RECURRING-PATTERNS.md`, `DESIGN-HISTORY.md`, `HOOK-FRICTION.md` |
| `known-failures/` | One shard per failure nobody could reproduce |

## Lifecycle

Statuses: `skeleton` -> `design` -> `ready` -> `in-progress` -> closed.
`blocked` and `deferred` are parking states. The workflow rules live in
`ai/rules/planning.md`; the spec format lives in `plan/TEMPLATE.md`.

`skeleton` is the one status allowed to carry template placeholders. From
`design` onward the native validation hook in `internal/le/hookruntime/lifecycle.go`
blocks them, because the author is then claiming those sections are written.

A spec that passes its Review Gate is not done until it is **deleted**: closure is
two commits (commit A: code + spec + the problem record; commit B: `git rm` the
spec). The problem record is a row in `plan/journal/<class>.md` whose `Spec` cell
names the spec (`plan/journal/README.md`). There is no `done/` directory.

## Deferred work has no directory

A spec that cannot finish an item does not park it in a shard. It writes the
remainder as its own spec in the bucket that item belongs to, and names that spec
in its own text. `plan/deferrals/` existed until 2026-09-05 and held 103 live rows,
29 of which named no destination at all, so that work was invisible to every count
above. A row nobody can count is a row nobody schedules. All 29 became specs in the
same piece of work, and the other 74 were copied into the spec each one named, under
a `Work Inherited From a Deferral Row` heading.

## Working With Specs

- `/ze-status` shows a cross-project attention view (statuses, buckets, stalls).
- `/ze-spec` creates or evolves a spec; `/ze-implement` executes one;
  `/ze-review` runs the completion gate.
- Each session records its spec with `./le spec session claim spec <stem>`
  (see `ai/rules/planning.md`).
