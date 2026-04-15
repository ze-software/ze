# Recap

Summarize the current session state so the user can quickly understand where things stand.

See also: `/ze-status` (wider cross-project view), `/ze-handoff` (prepare for next session)

## Steps

1. **Selected spec:** Read `tmp/session/selected-spec`. If a spec is selected, read it and extract:
   - Spec title and metadata (Status, Phase, Depends)
   - Task section (what the work aims to achieve)
   - Acceptance Criteria summary (total count, how many have evidence)
   - Implementation Phases (which phases exist, any phase markers)
2. **Session state:** Find the most recent `tmp/session/session-state-<spec-stem>-*.md` file for this spec (per-session files). Extract the most recent session entry.
3. **Git state:** Run `git status`, `git diff --stat`, and `git log --oneline -10`. Summarize:
   - Current branch
   - Uncommitted changes (count, key files, and scale from diff stat)
   - Recent commits relevant to the current work
4. **Tasks:** Check for in-progress tasks in this conversation using TaskList (skip if none exist).
5. **Formal deferrals:** Read `plan/deferrals.md`. List open deferrals, especially any from the selected spec.
6. **Session skips:** Review the conversation history for anything skipped, postponed, or left incomplete during this session that is NOT yet logged in `plan/deferrals.md`. Look for:
   - Edge cases noticed but not handled
   - TODOs or FIXMEs added in code
   - Test coverage gaps acknowledged but not filled
   - Work described as "later", "next", "for now", or "good enough"
   - Decisions to simplify or cut scope
7. **Present the recap** using this format:

```
## Recap

**Spec:** [name and status, or "none selected"]
**Branch:** [branch name]
**Goal:** [1-2 sentences from spec Task section, or from session context]

### Done
- [completed items, from commits, checked tasks, spec phases]

### In Progress
- [current work, uncommitted changes, active tasks]

### Remaining
- [unfinished spec items, unchecked ACs, remaining phases]

### Deferred (formal)
- [open deferrals from plan/deferrals.md, or "none"]

### Skipped This Session
- [things noticed/postponed/simplified during this session, or "none"]
```

## Rules

- Do NOT edit anything. Read-only.
- Do NOT propose next steps or a plan. Just report the state.
- If no spec is selected and no session state exists, say so plainly and summarize only git state.
- Keep it concise. The goal is orientation, not a full spec re-read.
