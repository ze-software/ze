---
paths:
  - "*"
---

# Git Safety

Rationale: `.claude/rationale/git-safety.md`

## Commit Rules

Only commit when user says "commit". Only include task-related files.
Output format: list staged files, excluded files, commit message, and ask — no git diff/stat output.
Post-commit: hash, file count, clean state confirmation.

## Before Any Commit (BLOCKING)

**`make ze-verify` (timeout 120s) — not `go test`, not any subset.**

**BLOCKING:** Never ask to commit without reporting ALL test failures to the user first. If any test failed, list every failure explicitly before any commit discussion. Hiding, omitting, or glossing over failures is forbidden.

```
[ ] 1. Run `make ze-verify` — capture to tmp/ze-test.log. ANY failure: STOP.
[ ] 2. Report test result: pass/fail. If failures: list every one. No omissions.
[ ] 3. Spec completion gate — if work was driven by a spec in plan/:
      [ ] Learned summary written to plan/learned/NNN-<name>.md
      [ ] Spec file staged for deletion (git rm)
      [ ] Both included in THIS commit (one commit = code + tests + summary)
      If not done: STOP. Do it before proceeding to step 4.
[ ] 4. Present what will be committed — concise table, not raw git output:
      | File | Change |
      staged files with one-line description of what changed and why
      Mention any unstaged/untracked files excluded from commit.
[ ] 5. Executive Summary Report (rules/planning.md) — present to user
[ ] 6. ASK user: "Ready to commit?" — WAIT for explicit yes
```

**Forbidden:** `git diff --stat`, `git status` dumped raw into output. Summarise for the user.
Never commit with lint issues. Never commit without test evidence.
`make ze-test` includes fuzz tests — use only when specifically needed.

## Forbidden Without Permission

`git reset`, `git revert`, `git checkout -- <file>`, `git restore`, `git stash drop`, `git push --force`

## Before Destructive Actions

Save: `git diff > .claude/backups/work-$(date +%Y%m%d-%H%M%S).patch` — then ASK user.

## Codeberg CLI

Use `tea` for PRs/issues: `tea pr list`, `tea pr create`, `tea issue list`, `tea issue create`
