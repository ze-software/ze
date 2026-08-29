---
name: ze-status
description: Status
---

# Status

Unified attention view across all project concerns. Shows what needs attention, with suggested next actions.

See also: `/ze-debrief` (deep dive on current session/spec)

## Steps

1. **Selected spec:** Run `./le spec session current`. If set, read spec metadata (Status, Phase, Updated).
2. **Open specs:** Scan `plan/spec-*.md` for all specs. For each, extract Status from metadata table. Present:

| Spec | Status | Updated |
|------|--------|---------|
| spec-name | design/skeleton/in-progress/blocked | date |

2b. **Work in flight:** Run `./le spec session wip`. Report the count against the cap and the three stalest. Every rule in `ai/rules/` governs how well ONE spec is executed; the cap is the only thing that limits how many are open at once, so an over-cap count is an attention item in its own right, not background noise.
3. **Git state:** Run `git status` and `git log --oneline -5`. Summarize:
   - Current branch
   - Uncommitted changes (count and key files)
   - Recent commits (last 5)
4. **Deferrals:** Read the shards under `plan/deferrals/` (one file per source; the live backlog is a fold over the directory, never a stored file). Count open items across all shards. List any that reference the selected spec.
5. **Test state:** Check `tmp/ze-verify.log`. If it exists, check its age and whether it shows failures.
   - Fresh (<1h) and passing: "Tests: PASS (Nh ago)"
   - Fresh and failing: "Tests: FAIL -- [count] failures (Nh ago)"
   - Stale (>1h) or missing: "Tests: not run recently"
6. **Present the status** using this format:

```
## Status

**Spec:** [selected spec name and status, or "none selected"]
**Branch:** [branch] | **Uncommitted:** [count] files
**Tests:** [PASS/FAIL/not run] | **Deferrals:** [count] open | **In flight:** [n]/[cap] specs

### Open Specs
[table from step 2, or "none"]

### Attention
[prioritized list of items needing action, each with a suggested command]
```

### Attention Items (prioritized)

Generate the attention list by checking these conditions in order:

| Condition | Attention Item | Suggested Action |
|-----------|---------------|------------------|
| Spec with `Status \| done` | "[spec] passed its gate but was never closed -- closure violation" | `/ze-close` (step 6 prepares the two closure commits) |
| Spec listed by `./le spec status closure list` | "[spec] completed but not closed" | `/ze-close` (step 6 prepares the two closure commits) |
| `./le spec session wip` count over `ZE_SPEC_WIP_CAP` | "N specs in flight (cap M) -- no new spec can be started" | `/ze-close` the stalest, or agree a new cap with the user |
| Tests failing | "N test failures in last run" | `/ze-debug` |
| Spec in-progress with uncommitted changes | "Uncommitted work on [spec]" | `/ze-verify` then `/ze-commit` |
| Spec in skeleton/design status | "[spec] needs implementation" | `/ze-implement` |
| Spec blocked | "[spec] blocked on [dependency]" | Check dependency |
| Deferrals referencing selected spec | "N deferred items for [spec]" | Review `plan/deferrals/` |
| No spec selected but specs exist | "No spec selected" | `/ze-spec` to resume or create |
| Stale test results | "Tests not run recently" | `/ze-verify` |

Only show items that actually apply. If nothing needs attention, say "Nothing pending."

## Rules

- Do NOT edit anything. Read-only.
- Do NOT propose a plan or start working. Just report status and suggest commands.
- Keep it concise -- this is a dashboard, not a narrative.
- Attention items should be actionable: each one links to the command that resolves it.
