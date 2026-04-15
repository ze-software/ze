# Status

Unified attention view across all project concerns. Shows what needs attention, with suggested next actions.

See also: `/ze-recap` (deep dive on current session/spec)

## Steps

1. **Selected spec:** Read `tmp/session/selected-spec`. If set, read spec metadata (Status, Phase, Updated).
2. **Open specs:** Scan `plan/spec-*.md` for all specs. For each, extract Status from metadata table. Present:

| Spec | Status | Updated |
|------|--------|---------|
| spec-name | design/skeleton/in-progress/blocked | date |

3. **Git state:** Run `git status` and `git log --oneline -5`. Summarize:
   - Current branch
   - Uncommitted changes (count and key files)
   - Recent commits (last 5)
4. **Deferrals:** Read `plan/deferrals.md`. Count open items. List any that reference the selected spec.
5. **Test state:** Check `tmp/ze-verify.log`. If it exists, check its age and whether it shows failures.
   - Fresh (<1h) and passing: "Tests: PASS (Nh ago)"
   - Fresh and failing: "Tests: FAIL -- [count] failures (Nh ago)"
   - Stale (>1h) or missing: "Tests: not run recently"
6. **Present the status** using this format:

```
## Status

**Spec:** [selected spec name and status, or "none selected"]
**Branch:** [branch] | **Uncommitted:** [count] files
**Tests:** [PASS/FAIL/not run] | **Deferrals:** [count] open

### Open Specs
[table from step 2, or "none"]

### Attention
[prioritized list of items needing action, each with a suggested command]
```

### Attention Items (prioritized)

Generate the attention list by checking these conditions in order:

| Condition | Attention Item | Suggested Action |
|-----------|---------------|------------------|
| Tests failing | "N test failures in last run" | `/ze-debug` |
| Spec in-progress with uncommitted changes | "Uncommitted work on [spec]" | `/ze-verify` then `/ze-commit` |
| Spec in skeleton/design status | "[spec] needs implementation" | `/ze-implement` |
| Spec blocked | "[spec] blocked on [dependency]" | Check dependency |
| Deferrals referencing selected spec | "N deferred items for [spec]" | Review `plan/deferrals.md` |
| No spec selected but specs exist | "No spec selected" | `/ze-spec` to resume or create |
| Stale test results | "Tests not run recently" | `/ze-verify` |

Only show items that actually apply. If nothing needs attention, say "Nothing pending."

## Rules

- Do NOT edit anything. Read-only.
- Do NOT propose a plan or start working. Just report status and suggest commands.
- Keep it concise -- this is a dashboard, not a narrative.
- Attention items should be actionable: each one links to the command that resolves it.
