# plan/ -- Backlog and Active Development

This directory is both the backlog and the active work tracker. One spec is one
work item, and its status lives in its own header table.

## The three buckets

A spec's directory says what it costs the FIRST RELEASE to leave it undone. The
test is a question about the shipped binary, never about how far along the work
is: a `skeleton` in `immediate/` outranks an `in-progress` spec here.

| Directory | The test it passes |
|-----------|--------------------|
| `plan/immediate/` | An operator on the first release meets this as a bug or a missing answer |
| `plan/pre-release/` | No operator meets it, but the release cannot go out until it is done |
| `plan/` (this level) | The release goes out without it |

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

## Release inventory

`./le spec roadmap update` generates the untracked `plan/roadmap.md` index from
committed `HEAD`. `./le spec roadmap list | json` returns the same inventory,
and `./le spec roadmap compare from <ref> to <ref>` reports endpoint changes.
Use `revision <ref>` after `list` or `update` to select a historical snapshot.
Pending edits appear after commit and regeneration.

The report is an inventory preview pending owner classification. Bucket and
ownership decisions must be resolved before public classification. The collector
preserves every bucket assignment and counts blocked, deferred, skeleton, and
malformed specs. Counts measure work items; they estimate neither effort nor
release readiness. `verification` remains open.

The endpoint comparison separates moves, status changes, additions, and removals.
A removal needs source verification before it supports a delivered claim. Items
added and removed entirely between endpoints do not appear in the comparison.

Release preparation includes hardening, race and security fixes, and evidence
from production traffic on real hardware. Configuration syntax must stabilize;
changes need automatic migration or a clear error. Upgrade paths follow the first
release. Optional specs can remain open when the release ships.

## Contents

| File | Purpose |
|------|---------|
| `spec-<name>.md` | One spec per work item, status in its header table |
| `immediate/`, `pre-release/` | The two buckets above, same spec format |
| `roadmap.md` | Generated committed-tree inventory; regenerate with `./le spec roadmap update` |
| `TEMPLATE.md` | Design-time spec format: everything that must exist BEFORE code |
| `TEMPLATE-CLOSURE.md` | Closure sections, appended by `/ze-close` at step 1 |
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
two commits from one `./le commit create` script. Commit A preserves the code,
tests, docs and edited spec; commit B removes the spec. Route any lesson to the
surface that governs it, and write a problem-class journal row only when no
surface governs that lesson yet (`ai/rules/planning.md`). Include any journal
rows owed by the work in commit A, but create no lesson artifact merely to
close the spec. Defect-recording obligations still apply. There is no `done/`
directory.

## Deferred work has no directory

A spec that cannot finish an item does not park it in a shard. It writes the
remainder as its own spec in the bucket that item belongs to, and names that spec
in its own text. `plan/deferrals/` existed until 2026-09-05 and held 103 live rows, <!-- doc-links: ignore (deleted on 2026-09-05 by `6fb9cd8814`; this section exists to say the directory is gone) -->
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
