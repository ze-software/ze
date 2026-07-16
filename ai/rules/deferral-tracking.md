# Deferral Tracking

**BLOCKING:** Every decision to not perform in-scope work MUST be recorded AND land in a destination spec.
Rationale: Untracked deferrals are invisible scope reductions. They accumulate silently across sessions.
A deferral whose destination is prose ("later", "future work") is a deletion with a polite name.

## Central Log

`plan/deferrals.md` -- the single source of truth for all deferred work.

## When to Record

| Trigger | Action |
|---------|--------|
| Deciding work is "out of scope" | Record with reason |
| Moving work to another spec | Record with destination spec |
| Skipping a task item from a spec | Record with reason |
| Postponing for any reason | Record with reason |
| User asks to skip something | Record (user-requested, still tracked) |

## Table Format

```
| Date | Source | What | Reason | Destination | Status |
```

| Column | Content |
|--------|---------|
| Date | YYYY-MM-DD |
| Source | Spec filename, task description, or "ad-hoc" |
| What | Specific work being deferred (not vague) |
| Reason | Why it is being deferred |
| Destination | Receiving spec filename (`plan/spec-*.md`), "cancelled", or "user-approved-drop" |
| Status | See Status Vocabulary below |

## Status Vocabulary (the gate reads this)

`deferral_unassigned_problems` (`scripts/dev/commit_helper.py`) checks the
Destination of every row whose Status is NOT terminal. The terminal set is
`DEFERRAL_TERMINAL_STATUSES` in that file:

| Status | Meaning | Destination checked? |
|--------|---------|----------------------|
| `open` | Live. Work has no home yet | YES |
| `deferred` | Live. Work has no home yet | YES |
| `done` | Terminal. Implemented, or moved into a spec | no |
| `cancelled` | Terminal. User decided not to do it | no |
| `resolved` | Terminal. Closed with evidence (learned summary) | no |

`open` and `deferred` are synonyms; prefer `deferred`. **Any other word is treated
as live and checked**, deliberately: the gate is a denylist of terminal states, not
an allowlist of live ones, so a status nobody has invented yet fails closed rather
than slipping through silently (`ai/rules/fail-closed-guards.md`).

This table and `DEFERRAL_TERMINAL_STATUSES` must not drift apart. They did once,
and it cost: the gate tested only `status == "open"` while this rule's own prose
taught the word `deferred`, so rows written correctly per the rule were never
looked at. 23 live rows without a home had accumulated behind that hole.

## Rules

| Rule | Detail |
|------|--------|
| Always a destination spec | Every live deferral names a `plan/spec-*.md` that exists on disk. Only a terminal Status may name no spec |
| No prose destinations | "later", "future work", "a follow-up", "TBD" are not destinations. A destination is a filename |
| No vague What | "Edge cases" is not acceptable. Name the specific case |
| Record immediately | Do not batch. Record when the decision is made, not at commit time |
| Review at session end | Check open deferrals before ending |

The gate is one notch wider than this rule on purpose: it accepts any existing
`plan/**.md`, not only `plan/spec-*.md`. The one sanctioned non-spec destination
is `plan/known-failures.md`, for a test that stays red and is tracked there
rather than fixed. Everything else lands in a spec. The gate cannot tell a
deliberate `known-failures.md` row from a lazy one, so the judgement stays here,
in the rule: if you are pointing a deferral anywhere other than a spec, be able
to say why the work is not spec-shaped.

## Choosing the Destination Spec (BLOCKING)

Deferred work ALWAYS has a destination spec. Decide which one in this order, at
the moment the deferral is made:

| Order | Action | Detail |
|-------|--------|--------|
| 1 | Find an existing spec that already covers the topic | `grep -l "<topic>" plan/spec-*.md`, and scan `make ze-spec-status`. Prefer a `spec-finish-<subsystem>` / `spec-followup-<subsystem>` umbrella when one owns the area |
| 2 | If one exists, add the work to its `## Task` section | The spec becomes the tracker. Record the deferral with that spec as Destination and Status `done` (moved to another spec, see "Resolving Deferrals") |
| 3 | Only if no spec covers the topic, create a deferral spec | Named `plan/spec-<source>-deferred-<subtask>.md` (see below) |

An existing spec is preferred over a new file. Do not create a deferral spec to
avoid the grep.

### Deferral Spec Naming (BLOCKING)

A spec created solely to hold work deferred out of another spec is named:

```
plan/spec-<source>-deferred-<subtask>.md
```

| Part | Content | Example |
|------|---------|---------|
| `<source>` | Stem of the spec the work was deferred FROM, without the `spec-` prefix | `bgp-rib-flush` |
| `<subtask>` | Short kebab-case name of the specific deferred work | `ipv6-coverage` |
| Result | | `plan/spec-bgp-rib-flush-deferred-ipv6-coverage.md` |

- One subtask per file. Two deferrals from the same source spec are two files,
  not one file with two tasks.
- The name carries the provenance: a reader knows what dropped it and why the
  file exists without opening it.
- For ad-hoc deferrals with no source spec, `<source>` is the subsystem
  (`plan/spec-l2tp-deferred-session-teardown-race.md`).
- **A source spec does not outlive the deferral.** Spec closure `git rm`s the
  spec (`ai/rules/planning.md` "Spec Closure"), so `<source>` will usually name
  a file that no longer exists by the time someone picks the work up. That is
  correct and intended: the provenance lives in git history, and the deferral
  spec is the tracker now. But when the source spec is ALREADY closed at the
  moment you write the deferral spec (homing an old row), name `<source>` for
  the subsystem instead: a filename pointing at a spec nobody can open reads as
  a broken reference rather than as provenance. Record the closed source spec in
  the `## Task` section either way.
- This naming applies only to deferral holders. Specs written as intended work
  keep the normal `spec-<task>.md` / `spec-<prefix>-<N>-<name>.md` names
  (`ai/rules/planning.md` "Spec Sets").

### Creating the Deferral Spec

| Step | Action |
|------|--------|
| 1 | Create the file from `plan/TEMPLATE.md` with `Status \| skeleton` |
| 2 | Fill only the `## Task` section with the points to complete, plus any constraint already known. Leave the rest as template placeholders |
| 3 | Name the source spec in the `## Task` section so the provenance survives |
| 4 | Record the deferral in `plan/deferrals.md` with the new spec as Destination |

Keep it small. The goal is zero lost work, not a finished design -- a skeleton is
captured intent, not a designed spec. It moves to `design` when someone picks it
up (status table in `ai/rules/planning.md`).

The commit gate `deferral_unassigned_problems` (`scripts/dev/commit_helper.py`)
blocks any commit while a LIVE deferral (any non-terminal Status, see Status
Vocabulary) names no destination or names a spec file that does not exist. It
also reports rows it cannot parse rather than skipping them: a row the gate
cannot read is a row it cannot enforce.

## Verify Before Deferring (BLOCKING)

Never claim "requires infrastructure that doesn't exist" without grepping for it first.
Before writing "deferred -- requires X" in any spec or summary, grep for X. If it exists,
implement it. If genuinely missing, name the specific thing that is missing and where it
would need to be added.

## What Is NOT a Deferral

- Completing work that was never in scope (no record needed)
- Choosing between two valid approaches (design decision, not deferral)
- Go `defer` keyword (language construct, excluded from pattern matching)

## Resolving Deferrals

| To close as | Set Status to | Set Destination to |
|-------------|---------------|--------------------|
| Implemented | `done` | Spec or commit where implemented |
| User decided not to do it | `cancelled` | `user-approved-drop` |
| Moved to another spec | `done` | Receiving spec filename |
