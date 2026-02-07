# Scoped Commit

Create a commit with explicit scope verification.

## Steps

1. **Show scope:** Run `git status` and `git diff --stat` to display all changed files
2. **Identify task scope:** Determine which files belong to the current task. If unclear, ask the user.
3. **Exclude unrelated changes:** If files outside the task scope are modified, explicitly list them and confirm with the user: "These files are outside the current task scope: [list]. Exclude from commit?"
4. **Stage only scoped files:** Use `git add <specific-files>` - never `git add .` or `git add -A`
5. **Verify staged content:** Run `git diff --cached --stat` to show exactly what will be committed
6. **Show recent commits:** Run `git log --oneline -5` to match commit message style
7. **Draft commit message:** Based on the actual changes (not the spec), write a concise commit message
8. **Confirm with user:** Present the staged files and commit message. Wait for approval before committing.
9. **Commit:** Run `git commit` with the approved message
10. **Verify:** Run `git status` to confirm clean state for committed files

## Rules

- Never include spec files unless the user explicitly asks
- Never include documentation changes unless they're part of the task
- If `make lint` hasn't been run this session, run it before committing
- If in doubt about scope, ask. The cost of asking is low; the cost of a bad commit is high.
