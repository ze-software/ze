# Git Safety

**When:** before any git operation, and when writing or running a commit script
**Severity:** blocking

## Directives

**Never edit behind the upstream. Before its first edit in each checkout it uses, including a separate site or wiki checkout, a top-level session MUST establish that the current branch is not behind its upstream: run `git fetch`, then `git rev-list --count HEAD..@{u}`.** A count of zero means HEAD already contains the upstream tip, and the session MUST proceed without pulling; a dirty working tree or index is not a blocker then, because a shared checkout always carries other sessions' work. If the fetch fails or the upstream is missing, the check has no answer and the session MUST stop before editing and report it. A count above zero means the session MUST run `git pull --rebase --no-autostash` before editing; if the checkout is dirty, a merge or rebase is in progress, another writer makes the pull unsafe, or the pull fails or conflicts, the session MUST stop before editing and report the blocker, because a rebase under uncommitted work either refuses or carries that work through the rebase. It MUST NOT stash, discard work, or change branches to get past it. A subagent sharing that checkout MUST rely on its parent's check and MUST NOT pull underneath the session's edits.

**Every commit MUST go through `./le commit create`, which writes one message file and one commit script; `git commit`, `git add`, `git rm`, `git restore --staged` and `git stash` MUST NOT be invoked as a direct Bash call.** `ai/INSTRUCTIONS.md` carries that ban into every session, and the same verbs inside the generated script are allowed.
**The printed `script=` line is the only authoritative path: MUST copy it, and MUST NOT construct it from the session id.** Read the message file first, name only canonical sources, run the script yourself with `bash`, then report the SHA, the files and the verification evidence. `docs/contributing/committing.md` carries the keywords and the refusals.

**A push MUST be ordered by the owner and MUST NOT be added on your own initiative; it MUST go through `./le commit push "<owner authorisation>"`, which writes a push-only script that re-checks the push debt and pushes the prepared commit by SHA.** The push owes no local verification: GitHub CI verifies the pushed commit. A bare `git push` from a Bash call stays forbidden and the hook enforces it, `--force` and `-f` are never used, and a worktree agent never pushes at all.

**A finished chunk MUST be committed when it finishes, not when the session does, and one commit carries one logical change.** The question after each piece of work is "does this stand on its own", never "am I finished for the day". A defect fix, a rule change, a gate repair and each piece of a spec's implementation are separate commits, and none of them MUST wait behind the spec's review gate: a piece that improves the software, compiles and works lands then, while a change too small to stand on its own rides with the piece it belongs to.

**Concurrent edits MUST NOT make a file unavailable for your commit. When the affected code compiles, commit the whole changed files even when they contain other sessions' changes.** Do not wait for a clean file or sole authorship. Acknowledge the concurrent changes in the commit body, naming their owners when known.

**The committed snapshot MUST include the dependencies it needs.** Judge it against HEAD plus the selected files. A compiling working tree does not prove a selected snapshot compiles when an unselected file supplies a new function or type. Carry the required files in the same coherent chunk or coordinate their publication, rather than treating another session's authorship as a blocker.

**Hunk selection MUST NOT be used merely to isolate authorship.** It remains available when a genuinely separate change cannot land in the same coherent chunk. Use `hunks <path> <patch-file>` without removing anyone's edits from the working-tree file. The content gates and review pin judge HEAD plus that patch, while the working-tree file keeps every session's work (`docs/contributing/committing.md`, "Committing some hunks of a shared file").

**You MUST NOT leave completed work uncommitted on the assumption that another session will land the shared file.** Commit the compiling chunk through the native named-file route. Shared files are normal in this checkout, and another session's edits alone require neither permission nor a handoff.

**A branch MUST NOT be changed, created, deleted, renamed or integrated from a tool call, except for the same-branch `git pull --rebase` that "Never edit behind the upstream" requires of a branch behind its upstream, and for the new branch of the worktree an agent opens for work also tested on Thomas's other development machine (`ai/owner/machines.md`): stay on the branch you started on and ask the user to move it.** When the user integrates a worktree branch it lands on main via `git rebase <branch>`, never `git merge`, so history stays linear.

**`--no-gpg-sign`, `-c commit.gpgsign=false` and `--no-verify` MUST NOT be used, and a hook MUST NOT be disabled to make a commit pass.** What to do when signing fails is `docs/contributing/committing.md`.

**Secrets, private keys, and local home-directory paths MUST NOT be committed.** API keys, credential files (`.env`, `.netrc`, `.npmrc`, `id_rsa`, `id_ed25519`, `settings.local.json`, `credentials.json`), private keys outside test fixtures, live secret-named environment values, and the committer's own home directory path are refused at commit time by `commit secrets check` (`internal/le/commit/secrets`). A deliberate test fixture carrying a synthetic token or path marks that line with `secret-scan: allow`.

**Pull requests and issues MUST go through `gh`:** `gh pr list`, `gh pr create`, `gh issue list`, `gh issue create`. The repository is only at github.com/ze-software/ze, so `tea`, the Gitea client, does not reach it.

## Before Destructive Actions

**The destructive git verbs MUST NOT be run.** `ai/INSTRUCTIONS.md`, "Destructive git commands are FORBIDDEN", carries the list into every session, so it is not restated here.
**Before anything destructive you MUST save a patch, write the destructive commands to `tmp/delete-SESSION.sh`, tell the user, and stop.** The patch is `git diff > backups/work-$(date +%Y%m%d-%H%M%S).patch`.
