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
