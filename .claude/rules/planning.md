# Planning (Claude-Specific)

Extends `ai/rules/planning.md` with Claude Code session management.

## Spec Selection Tracking

Each session records the spec it is working on in its OWN per-session marker via
`./le spec session`. There is no shared file, so many agents editing
main concurrently never collide -- nothing to append, nothing to remove.

| Action | Command |
|--------|---------|
| Claim the spec you are about to work on | `./le spec session claim spec <spec-file>` |
| Read this session's claimed spec | `./le spec session current` |
| Release the claim when the spec is closed | `./le spec session release` |

`claim` also auto-transitions a `ready` spec to `in-progress`. The marker drives
the per-session state file name and post-compaction recovery.

## Plan File Location

Write plan files to project `.claude/plan/ze-plan-<name>`, NOT `~/.claude/plan`.
The `claude-plans` check in the native `pretool-writeedit` action
(`./le hook-check pretool-writeedit`, `internal/le/hookruntime/writeedit.go`)
enforces this.

## /ze-review Gate (Completion Checklist)

Invoke `/ze-review` to satisfy the Review Gate defined in `ai/rules/planning.md`.
Fill the `## Review Gate` section in the spec. Loop until 0 BLOCKER, 0 ISSUE.

## Spec Status Transitions

| Event | When exactly |
|-------|--------------|
| Start research | First action of `/ze-spec` Step 2 |
| Spec approved | After user approves in `/ze-spec` Step 4 |
| Start coding | First action of `/ze-implement`, before audit |
| Hand off for review (`Handoff: verify` specs only) | `/ze-implement` step 11: set `verification`, commit code + spec, release the claim, stop |
| Implementation complete | `/ze-close` step 6: commit A (code + spec + learned), then commit B (`git rm` spec) |

## Spec Closure (BLOCKING)

See `ai/rules/planning.md` "Spec Closure" for the full two-commit rule and banned patterns.

`/ze-status` flags in-progress specs with clean Review Gates as "completed but not closed."
