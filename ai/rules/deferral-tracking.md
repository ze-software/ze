# Deferral Tracking

**BLOCKING:** Every decision to not perform in-scope work MUST be recorded.
Rationale: Untracked deferrals are invisible scope reductions. They accumulate silently across sessions.

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
| Destination | Receiving spec filename, "cancelled", or "user-approved-drop" |
| Status | `open`, `done`, `cancelled` |

## Rules

| Rule | Detail |
|------|--------|
| No empty Destination for open items | Every open deferral must name where the work will land |
| No vague What | "Edge cases" is not acceptable. Name the specific case |
| Record immediately | Do not batch. Record when the decision is made |
| Review at session end | Check open deferrals before ending |

## Destination Spec Missing -> Create a Skeleton (BLOCKING)

Every open deferral needs a destination spec (see Rules). When the follow-up
work has no existing spec to land in, do not leave the Destination empty and do
not drop the work: create a skeleton spec so the information survives the commit.

At commit time, before finalising the commit:

| Step | Action |
|------|--------|
| 1 | List what was deferred or left unimplemented from this commit's scope |
| 2 | For each item with no existing destination spec, create `plan/spec-<name>.md` from `plan/TEMPLATE.md` with `Status \| skeleton` |
| 3 | Fill only the `## Task` section with the points to complete, plus any constraint already known. Leave the rest as template placeholders |
| 4 | Record the deferral in `plan/deferrals.md` with the new skeleton spec as Destination |

Keep it small. The goal is zero lost work, not a finished design -- a skeleton is
captured intent, not a designed spec. It moves to `design` when someone picks it
up (status table in `ai/rules/planning.md`).

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
