# Git Safety

Rationale: `.claude/rationale/git-safety.md`

## Commit Rules

**NEVER run `git commit` or `git push`.** These commands are in the deny list and blocked by hooks.

**Commit workflow:**
1. Run `git add` to stage task-related files only.
2. Write the commit message to `/tmp/commit-msg-SESSION.txt` where SESSION = 8-char unique ID
   (e.g., `head -c4 /dev/urandom | xxd -p` at session start). File lives in /tmp to avoid
   permission prompts. Unique name prevents parallel sessions from clobbering each other.
3. Report what was done and what is left (including deferred).
4. The user runs `git commit -F /tmp/commit-msg-XXXX.txt` themselves.

**Unstaging files:** `git restore --staged <file>` is allowed. All other `git restore` variants are forbidden.

**Never suggest, ask about, or hint at committing.** Complete ALL work first (testing, spec,
docs, learned summary). Then report what was done and what is left (including deferred).
The user decides when to commit. Banned phrases: "ready to commit?", "shall I commit?",
"we could commit now", "want me to commit?".

## Commit Granularity

Changes to the same system go in one commit. Disjoint systems (e.g., CLI and BGP encoding)
get separate commits. A feature + its tests + its docs = one commit. A bug fix in an
unrelated package = separate commit.

## Before Any Commit (BLOCKING)

**`make ze-verify` (timeout 180s) — not `go test`, not any subset.**

**BLOCKING:** Never ask to commit without reporting ALL test failures to the user first. If any test failed, list every failure explicitly before any commit discussion. Hiding, omitting, or glossing over failures is forbidden.

```
[ ] 1. Run `make ze-verify` — capture to tmp/ze-test.log. ANY failure: STOP and report.
[ ] 2. Report test result: pass/fail. If failures: list every one. No omissions.
      Ask user how to proceed — the right call depends on context and risk.
[ ] 3. Spec completion gate — if work was driven by a spec in plan/:
      [ ] Learned summary written to plan/learned/NNN-<name>.md
      [ ] Spec file staged for deletion (git rm)
      If not done: STOP. Do it before proceeding.
[ ] 4. Executive Summary Report (rules/planning.md) — present what was done and what is left.
      Do NOT ask to commit. The user will tell you when.
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
