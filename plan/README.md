# plan/ -- Backlog and Active Development

This directory is both the backlog and the active work tracker. One spec is one
work item, and its status lives in its own header table.

## Start here

Every work item lives in a subfolder. Its folder answers what the first release
owes; its `Status` answers how far the work has progressed. A release defect
does not become optional because its spec is only a skeleton.

| Folder | Work it owns |
|--------|--------------|
| `release/fixes/` | Incorrect shipped behavior: security, protocol, routing, configuration and lifecycle defects |
| `release/validation/` | Evidence that shipped behavior works, including interoperability, failure recovery and accurate support claims |
| `release/delivery/` | Release artifacts, signing, installation, appliance boot and essential onboarding |
| `post-release/features/` | Additional capabilities, absent RFC features and broader language or platform support |
| `post-release/engineering/` | Refactors, tooling and optimizations that do not block release correctness or proof |
| `proposals/` | Unapproved ideas, visible in the inventory but not a release commitment |

The current category declaration is `internal/le/spec/path/specpath.go`.
Inventory tools derive their population from it. Do not add a new work folder
without updating that declaration and its consumers.

## Release boundary and order

Apply the owner's release-timing and plan-structure rules in `ai/META-DECISIONS.md`. Full RFC coverage is
not a release prerequisite. A shipped capability still owes every requirement
needed for its correct behavior. Partial implementation must not be announced
as full RFC support, and disclosure does not excuse a known defect.

| Priority | First-release risk |
|----------|--------------------|
| P0 | Security, data loss or a fail-open boundary |
| P1 | Routing, configuration, startup and lifecycle correctness |
| P2 | Release validation and delivery |
| P3 | Post-release work |

Each spec records `Priority`, `Release reason` and `Depends` in its header,
and `RFC gaps` when it closes requirements of an RFC Ze implements.
Priority orders attention within a category; a dependency takes precedence.
Freeze the shipped capability and platform set first. Fix security and
configuration boundaries, then routing and recovery. Prepare test hosts and
release artifacts in parallel. Finish with evidence over the actual release
candidate, not a count of closed plans.

## Split fixes from features

Split a mixed spec when its obligations belong in different release categories.
Assign every acceptance criterion, dependency and remaining implementation step
to an owning spec, with reciprocal references. Preserve historical evidence as
history; do not turn it into a new claim about the product.

An absent feature can remain post-release. Accepted but ineffective
configuration is not an absent feature: its incorrect behavior still needs a
release owner. A fix does not close merely because an enhancement moved away.
Relocations and splits are not completion, and a lower release count is not
evidence that the product improved.

## Release inventory

`./le spec roadmap update` generates the untracked `plan/release/roadmap.md` index from
committed `HEAD`. `./le spec roadmap list | json` returns the same inventory,
and `./le spec roadmap compare from <ref> to <ref>` reports endpoint changes.
Use `revision <ref>` after `list` or `update` to select a historical snapshot.
Pending edits appear after commit and regeneration.

The report preserves category assignments and counts blocked, deferred, skeleton
and malformed specs, including proposals. Classification is a planning decision,
not proof of correctness. Counts measure work items; they estimate neither effort
nor release readiness. `verification` remains open. Historical snapshots retain
the categories and paths that existed at their source revision.

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
| `<work-folder>/spec-<name>.md` | One spec per work item, with lifecycle, release priority, rationale and dependencies in its header |
| `release/roadmap.md` | Generated committed-tree inventory; regenerate with `./le spec roadmap update` |
| `templates/TEMPLATE.md` | Design-time spec format: everything that must exist before code |
| `templates/TEMPLATE-CLOSURE.md` | Closure sections, appended by `/ze-close` at step 1 |
| `records/` | Historical reports and analysis formerly mixed with root work items |
| `journal/` | One file per problem class, one row per occurrence (`plan/journal/README.md`) |
| `learned/` | The hand-written meta-indexes `RECURRING-PATTERNS.md`, `DESIGN-HISTORY.md`, `HOOK-FRICTION.md` |
| `known-failures/` | One shard per failure nobody could reproduce |
| `doc-reviewed/` | One shard per commit session: docs claims a reader reviewed after a change that preserves behavior (`docs/contributing/documentation-testing.md`) |
| `handover/`, `audits/`, `design/`, `comparison/` | Existing named supporting records; these are not extra work-item buckets |
| `verification-debt/` | Outstanding verification obligations, not optional feature work |

## Lifecycle

Statuses: `skeleton` -> `design` -> `ready` -> `in-progress` -> closed.
`blocked` and `deferred` are parking states. The workflow rules live in
`ai/rules/planning.md`; the spec format lives in `plan/templates/TEMPLATE.md`.

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

## Deferred work remains a visible spec

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
- Each session records its spec with `./le spec claim spec <stem>`
  (see `ai/rules/planning.md`).
