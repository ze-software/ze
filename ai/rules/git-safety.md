# Git Safety

**When:** before any git operation, and when writing or running a commit script
**Severity:** blocking

## Directives

**Every commit MUST go through `./le commit create`, which writes one message file and one commit script; `git commit`, `git add`, `git rm`, `git restore --staged` and `git stash` MUST NOT be invoked as a direct Bash call.** `ai/INSTRUCTIONS.md` carries that ban into every session, and the same verbs inside the generated script are allowed.
**The printed `script=` line is the only authoritative path: MUST copy it, and MUST NOT construct it from the session id.** Read the message file first, name only canonical sources, run the script yourself with `bash`, then report the SHA, the files and the verification evidence. `docs/contributing/committing.md` carries the keywords and the refusals.

**A push MUST be ordered by the owner and MUST NOT be added on your own initiative; it MUST go through `./le commit create ... push "<owner authorisation>"`, which the generated script performs after every commit succeeds (owner amendment, 2026-08-05).** A bare `git push` from a Bash call stays forbidden and the hook enforces it, `--force` and `-f` are never used, and a worktree agent never pushes at all.

**A finished chunk MUST be committed when it finishes, not when the session does, and one commit carries one logical change (owner directive, 2026-08-21).** The question after each piece of work is "does this stand on its own", never "am I finished for the day". A defect fix, a rule change, a gate repair and a spec's implementation are four commits, and the first three MUST NOT wait behind the fourth's review gate.

**A branch MUST NOT be changed, created, deleted, renamed or integrated from a tool call: stay on the branch you started on and ask the user to move it.** When the user integrates a worktree branch it lands on main via `git rebase <branch>`, never `git merge`, so history stays linear.

**`--no-gpg-sign`, `-c commit.gpgsign=false` and `--no-verify` MUST NOT be used, and a hook MUST NOT be disabled to make a commit pass.** What to do when signing fails is `docs/contributing/committing.md`.

**Pull requests and issues MUST go through `gh`:** `gh pr list`, `gh pr create`, `gh issue list`, `gh issue create`. Development moved off Codeberg in July 2026 and the repository is now only at github.com/ze-software/ze, so `tea`, the Gitea client this rule named until 2026-08-30, addresses a forge Ze no longer publishes to.

## Before Destructive Actions

**The destructive git verbs MUST NOT be run.** `ai/INSTRUCTIONS.md`, "Destructive git commands are FORBIDDEN", carries the list into every session, so it is not restated here.
**Before anything destructive you MUST save a patch, write the destructive commands to `tmp/delete-SESSION.sh`, tell the user, and STOP.** The patch is `git diff > backups/work-$(date +%Y%m%d-%H%M%S).patch`.
