---
name: ze-status
description: Status
---

# Status

Unified attention view across all project concerns. Shows what needs attention, with suggested next actions.

See also: `/ze-debrief` (deep dive on current session/spec)

## Steps

1. **Selected spec:** Run `./le spec current`. If set, read spec metadata (Status, Phase, Updated).
2. **Open specs:** Run `./le spec status`. Its declared folders include release fixes, validation and delivery, post-release features and engineering, and proposals (`plan/README.md`). Read priority and dependencies separately from lifecycle status. Present:

| Spec | Bucket | Priority | Status | Updated |
|------|--------|----------|--------|---------|
| spec-name | category from inventory | declared priority | declared status | date |

2b. **Work in flight:** Run `./le spec wip`. Report the count against the cap and the three stalest. Every rule in `ai/rules/` governs how well ONE spec is executed; the cap is the only thing that limits how many are open at once, so an over-cap count is an attention item in its own right, not background noise.
3. **Git state:** Run `git status` and `git log --oneline -5`. Summarize:
   - Current branch
   - Uncommitted changes (count and key files)
   - Recent commits (last 5)
4. **Release categories:** Use the inventory's category counts, not an independent set of directory globs. Show release fixes first, with validation and delivery alongside their prerequisites. Then read the selected spec's **Work Not Done** table and list any row whose owning spec is absent from the inventory.
5. **Test state:** Check `tmp/ze-verify.log`. If it exists, report its age and whether it shows failures.
   - Passing: "Tests: PASS (Nh ago)"
   - Failing: "Tests: FAIL -- [count] failures (Nh ago)"
   - Missing: "Tests: no run recorded"
   An old or missing run is not an attention item. During development only
   the targeted tests for what changed run, and the full gate runs once per
   piece of work, over its final commit.
6. **Present the status** using this format:

```
## Status

**Spec:** [selected spec name and status, or "none selected"]
**Branch:** [branch] | **Uncommitted:** [count] files
**Tests:** [PASS/FAIL/not run] | **Categories:** [counts from inventory] | **In flight:** [n]/[cap] specs

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
| `./le spec wip` count over `ZE_SPEC_WIP_CAP` | "N specs in flight (cap M) -- no new spec can be started" | `/ze-close` the stalest, or agree a new cap with the user |
| Tests failing | "N test failures in last run" | `/ze-debug` |
| Spec in-progress with uncommitted changes | "Uncommitted work on [spec]" | `/ze-verify` then `/ze-commit` |
| Spec in skeleton/design status | "[spec] needs implementation" | `/ze-implement` |
| Spec blocked | "[spec] blocked on [dependency]" | Check dependency |
| A **Work Not Done** row naming a spec that is not on disk | "[spec] routes N items at a spec nobody wrote" | Write the spec in the bucket that item belongs to |
| No spec selected but specs exist | "No spec selected" | `/ze-spec` to resume or create |

Only show items that actually apply. If nothing needs attention, say "Nothing pending."

## Rules

- Do NOT edit anything. Read-only.
- Do NOT propose a plan or start working. Just report status and suggest commands.
- Keep it concise -- this is a dashboard, not a narrative.
- Attention items should be actionable: each one links to the command that resolves it.
