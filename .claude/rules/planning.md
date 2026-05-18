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
| Implementation complete | `/ze-implement` step 15: commit A (code + spec + learned), then commit B (`git rm` spec) |

## Spec Closure (BLOCKING)

**A spec that passes its Review Gate is not done until it is deleted from `plan/`.**

The lifecycle is: `in-progress` -> Review Gate clean -> write learned summary -> `git rm` spec.
Leaving a completed spec in `plan/` causes every future session to count it as open work.

**TWO commits, ONE script.** The spec is edited during implementation (design notes,
status updates, corrected assumptions). Those edits are valuable design history.
`git rm` destroys the working copy. If the edited spec is never committed before
deletion, the design work is lost from git history forever.

The commit script MUST produce two commits:
1. **Commit A (implementation + spec):** `git add` all code, tests, docs, learned summary,
   LEARNED-INDEX, counter bump, AND the spec file itself (with all edits from implementation).
2. **Commit B (spec closure):** `git rm plan/<spec>` only.

This preserves the final spec state in git history. `git log -p -- plan/<spec>` shows
the full design record. The deletion in commit B is a clean removal of a file whose
final state is already committed.

**Never `git rm -f` a spec without committing it first.** The `-f` flag silently
discards uncommitted edits. If the spec was modified during implementation (it
almost always is), those modifications must be committed before deletion.

| Banned | Why |
|--------|-----|
| "I'll close it later" | Later never comes. Other sessions see it as in-progress. |
| "The user will handle it" | The user asked us to implement. Closure is part of implementation. |
| `git rm` in the same commit as implementation | Spec edits are lost from history. Two commits required. |
| `git rm -f` without a prior commit of the spec | Destroys uncommitted design work. |
| "Run the commit, then I'll prepare closure" | The user will not ask. One script, one run, done. |

`/ze-status` flags in-progress specs with clean Review Gates as "completed but not closed."
