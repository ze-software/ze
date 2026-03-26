# Scoped Commit

Create a commit with explicit scope verification.

## Steps

1. **Verify tests passed:** Check if `make ze-verify` has been run and passed this session. If not, run it (timeout 180s). If it fails, stop and report all failures. Do not proceed to commit.
2. **Show scope:** Run `git status` and `git diff --stat` to display all changed files
3. **Identify task scope:** Determine which files belong to the current task. If unclear, ask the user.
4. **Exclude unrelated changes:** If files outside the task scope are modified, explicitly list them and confirm with the user: "These files are outside the current task scope: [list]. Exclude from commit?"
5. **Stage only scoped files:** Use `git add <specific-files>` - never `git add .` or `git add -A`
6. **Verify staged content:** Run `git diff --cached --stat` to show exactly what will be committed
7. **Show recent commits:** Run `git log --oneline -5` to match commit message style
8. **Draft commit message:** Based on the actual changes (not the spec), write a concise commit message
9. **Confirm with user:** Present the staged files and commit message. Wait for approval before committing.
10. **Commit:** Run `git commit` with the approved message
11. **Verify:** Run `git status` to confirm clean state for committed files

## Rules

- Never include spec files unless the user explicitly asks
- Never include documentation changes unless they're part of the task
- If `make ze-lint` hasn't been run this session, run it before committing
- If in doubt about scope, ask. The cost of asking is low; the cost of a bad commit is high.
