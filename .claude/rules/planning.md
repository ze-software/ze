# Planning (Claude-Specific)

Extends `ai/rules/planning.md` with Claude Code session management.

## Spec Selection Tracking

Tracked in `tmp/session/selected-spec` (one filename per line).
**Append** your spec filename when selecting. **Remove your line** after writing summary to `plan/learned/`.
Multiple lines means multiple Claude sessions are working concurrently -- do not overwrite their entries.

## Plan File Location

Write plan files to project `.claude/plan/ze-plan-<name>`, NOT `~/.claude/plan`.
Hook `block-claude-plans.sh` enforces this.

## /ze-review Gate (Completion Checklist)

Before final testing/verify, invoke `/ze-review`. Fill the "## Review Gate"
section in the spec with the findings list. If ANY finding is severity BLOCKER
or ISSUE (anything above NOTE), fix it and re-run `/ze-review`. Loop until
the review returns only NOTEs (or nothing). Paste the final clean review
output into the spec. NOTE-only findings do NOT block.

## Spec Status Transitions

| Event | When exactly |
|-------|--------------|
| Start research | First action of `/ze-spec` Step 2 |
| Spec approved | After user approves in `/ze-spec` Step 4 |
| Start coding | First action of `/ze-implement`, before audit |
| Implementation complete | `/ze-implement` stage 14: set `done`, write learned summary, `git rm` spec |

## Spec Closure (BLOCKING)

**A spec that passes its Review Gate is not done until it is deleted from `plan/`.**

The lifecycle is: `in-progress` -> Review Gate clean -> write learned summary -> `git rm` spec.
All three happen in `/ze-implement` stage 14, in the same session that finished the work.
Leaving a completed spec in `plan/` causes every future session to count it as open work.

| Banned | Why |
|--------|-----|
| "I'll close it later" | Later never comes. Other sessions see it as in-progress. |
| "The user will handle it" | The user asked us to implement. Closure is part of implementation. |
| "It's just a status change" | A spec in `plan/` with status `done` is worse than `in-progress` -- it is invisible to `/ze-status` staleness checks but still occupies the spec list. |

`/ze-status` flags in-progress specs with clean Review Gates as "completed but not closed."
