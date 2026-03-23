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

## Hook Enforcement

| Hook | Event | Blocking | What it checks |
|------|-------|----------|----------------|
| `check-deferral-in-diff.sh` | PreToolUse/Bash | exit 2 | Deferral language in staged diff without `plan/deferrals.md` also staged |
| `check-deferral-unassigned.sh` | PreToolUse/Bash | exit 2 | Open deferrals with empty/placeholder Destination |
| `warn-deferral-in-edit.sh` | PostToolUse/Write\|Edit | exit 1 | Deferral language in spec/doc edits (immediate reminder) |
| `session-end-deferrals.sh` | Stop | advisory | Prints open deferrals at session close |

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
