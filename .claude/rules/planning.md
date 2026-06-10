# Planning (Claude-Specific)

Extends `ai/rules/planning.md` with Claude Code session management.

## Spec Selection Tracking

Tracked in `tmp/session/selected-spec` (one filename per line).
**Append** your spec filename when selecting. **Remove your line** after writing summary to `plan/learned/`.
Multiple lines means multiple Claude sessions are working concurrently -- do not overwrite their entries.

## Plan File Location

Write plan files to project `.claude/plan/ze-plan-<name>`, NOT `~/.claude/plan`.
The `claude-plans` check in `.claude/hooks/pretool-writeedit.py` enforces this.

## /ze-review Gate (Completion Checklist)

Invoke `/ze-review` to satisfy the Review Gate defined in `ai/rules/planning.md`.
Fill the `## Review Gate` section in the spec. Loop until 0 BLOCKER, 0 ISSUE.

## Spec Status Transitions

| Event | When exactly |
|-------|--------------|
| Start research | First action of `/ze-spec` Step 2 |
| Spec approved | After user approves in `/ze-spec` Step 4 |
| Start coding | First action of `/ze-implement`, before audit |
| Implementation complete | `/ze-implement` step 15: commit A (code + spec + learned), then commit B (`git rm` spec) |

## Spec Closure (BLOCKING)

See `ai/rules/planning.md` "Spec Closure" for the full two-commit rule and banned patterns.

`/ze-status` flags in-progress specs with clean Review Gates as "completed but not closed."
