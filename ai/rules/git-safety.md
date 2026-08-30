# Git Safety

**When:** before any git operation, and when writing or running a commit script
**Severity:** blocking

## Commit Rules

**`git commit`, `git add`, `git rm`, `git restore --staged` and `git stash` MUST NOT be invoked as a direct Bash tool call.** `ai/INSTRUCTIONS.md` carries the ban into every session, so it is not restated here.
**The same verbs inside the generated commit script are ALLOWED.** The ban is on the direct tool call, not on what the script does when you run it. `docs/contributing/committing.md` describes what the command writes and what it refuses.

**A shared single-file plan log cross-commits even with a correct, explicit
`file <path>` list.** The ban on bare staging verbs fixes staging *timing*; it
cannot fix staging *granularity*: the generated script stages the whole file,
including hunks another session left uncommitted in it. You MUST SHARD the log so each
session writes only files it owns and git merges disjoint creations without
conflict. **Both cross-spec logs are now sharded.** Deferrals live one file
per source under `plan/deferrals/` (`ai/rules/planning.md`), so one
`file plan/deferrals/<source>.md` pair stages only your row. Known failures live
one file per failure under `plan/known-failures/` (a
`<native-action>-<test-name>.md` shard, with `RESOLVED.md` archiving history), so
one `file plan/known-failures/<native-action>-<test-name>.md` pair stages only your
entry. A shared unsharded log lets concurrent sessions stage each other's entries.

**A cross-commit of a shared plan file is STRUCTURAL, not misconduct by the other session. With concurrent sessions and a shared single-file log it is expected, and you MUST NOT read it as a rule violation. Each situation MUST be handled as its row says:**

| Situation | Do |
|-----------|-----|
| Your rows in a shared plan file are already committed by someone else | Nothing. The content is correct and preserved; only attribution is off. NEVER rewrite history to reclaim it |
| You edited a shared plan file | Commit it promptly. The longer it sits, the likelier another session's commit absorbs it |
| Your commit omits a shared plan file you edited | Check `git log -1 -- <file>` before assuming the edit was lost: another session probably committed it already |
| You see foreign rows in a shared plan file's diff | That is expected. Do not "clean" them out; you would revert another session's work |

**A `file <path>` list is a list of PATHS, and a path carries whatever the file holds
at that moment. Before naming a path, you MUST run `git diff` over it and confirm
every hunk is yours.**

Checking that your own edit is ready is not the same as checking that the file is.
In a checkout several sessions share, a file you edited an hour ago can hold
another session's uncommitted work by the time you commit, and the helper takes
the whole file. The commit then lands correct content under a message that
describes something else, and the session whose work you carried loses the
attribution rather than the work.

This is distinct from the shared-plan-log case, which is structural and expected.
That one cannot be avoided by checking, because a single-file log is written by
several sessions by design. A source file, a doc page or a script has no such
excuse: the hunks are separable, they are visible in one command, and reading
them costs a second.

The two failures are told apart by what the diff shows, not by which directory
the file is in:

- Every hunk is yours: commit it.
- A hunk is another session's, and the file is a shared plan log or journal class
  file: carry it, say so, and do not clean it out.
- A hunk is another session's, and the file is anything else: drop the path from
  this commit and let the session that owns the hunk land it, or say plainly in
  the message that you carried it and why.

The repair when it goes wrong is disclosure, never history. Rewriting to reclaim
attribution is banned, and the content is already safe.

**Explicit commit requests are a fast path.** When the user asks for a
commit, the implementation/review phase is over. You MUST prepare the commit
script and run it immediately. You MUST NOT re-audit the implementation, run late
completeness/remaining-work tables, inspect speculative companion artifacts,
or rerun lint/tests just because commit was requested. You MUST inspect only enough
state to avoid staging unrelated, ignored, generated, or out-of-scope paths.
**One check is exempt, because it cannot run earlier: `./le repository tracked-build check`
after the script has run** (step 7). It judges the commit you just made, which no
run before that commit could see.

**Thomas ruled on this exemption on 2026-08-04: KEEP IT.** It is settled, so you
MUST NOT re-open it. The check is not a rerun because its input is a commit that
did not exist until the script ran. It is not a check on the working tree
because that tree can already hold the next change. The tracked build's cost is
bounded and one-shot, while the failure it prevents is unbounded.
If scope is ambiguous, ask one narrow question; otherwise proceed.

**Commit workflow:**
1. You MUST use `./le commit session` to create or reuse this harness session's eight-hex commit namespace.
2. You MUST use `./le commit create` to write one message file and one commit script. It is the sole staging and commit route, and there is no hand-written fallback. `docs/contributing/committing.md` carries the keywords, the refusals, and worked invocations.
3. **The `script=` line the command prints is the only authoritative path. You MUST copy it, and MUST NOT construct it from the session id.**
4. Before creating the script, you MUST read `.gitignore` and name only canonical sources.
5. You MUST read the generated message file before running the script, at the path on the `message=` line.
6. You MUST run the finished script yourself with `bash` and the printed path. For a commit carrying Go, module, or vendor paths, you MUST run `./le repository tracked-build check` immediately afterwards.
7. You MUST report the commit SHA, the included files, the message file, the script path, the push status, and the verification evidence or the skip reason.
**The command asks for no lesson artifact, and it MUST NOT be made to.** Apply a lesson by updating the surface that governs behaviour. Journal rows exist only to count recurrence of a problem class.

**A file modified during implementation and then removed MUST be committed in its current state first**, before the `remove` that deletes it (`ai/rules/planning.md`, Spec Closure). `./le commit create` already refuses a `remove` path that is not tracked.

**You MUST NOT suggest / ask / hint at committing.** Complete ALL work first
(testing, spec, docs, journal row), then report. User decides.
Banned phrases: `ready to commit?`, `shall I commit?`, `we could
commit now`, `want me to commit?`.

## Pushing (2026-08-05, owner amendment)

- **A bare `git push` from a Bash call stays forbidden; the hook enforces it.**
- **You MUST push only with `./le commit create ... push "<owner authorisation>"`; the generated script performs it after every commit succeeds.**
- **The owner orders a push; you MUST NOT add `push` on your own initiative.**
- **`git push --force` and `-f` stay forbidden; the native route never forces.**

**When a push goes wrong, you MUST take the action its row names:**

| Situation | Do |
|-----------|-----|
| The script's commit step failed | Nothing is pushed and nothing SHOULD be: `set -euo pipefail` stops the script before the push. Fix the cause (staged index, GPG), then re-run the script |
| The commits are made and no push was ordered | Stop and report the SHAs. A push nobody asked for is a push without authority, whatever the branch looks like |
| You are a worktree agent | Never push. Work on your branch and stop there (`ai/INSTRUCTIONS.md`) |
| The owner orders a push after your script already ran | Say so and let him push, or carry `push "<owner authorisation>"` on the next commit you prepare. Do not type the command to close the gap |

## Commit Granularity

**A commit MUST have a single focus: one logical change.** One system is one commit, carrying its feature, tests and docs together. Several unrelated changes are several commits, never one bundle. An unrelated bug fix is its own commit. The fixes from one review pass are one commit.

**A finished chunk MUST be committed when it finishes, not when the session does (owner directive, 2026-08-21).** Work that is done and green sits in one working tree, where the next `git clean`, checkout or crashed session destroys it, and where every later chunk has to be diffed around it. The question to answer after each piece of work is "does this stand on its own", never "am I finished for the day". A defect fix, a rule change, a gate repair and a spec's implementation are four commits, and the first three MUST NOT wait behind the fourth's review gate.

**Read the working tree's SPREAD before starting new work, and land what is already finished first.** `./le working-tree` reports the changed paths grouped by area. More than one area in flight means a chunk is waiting that could already be committed, and it MUST be landed before the next piece starts. The cost of getting this wrong compounds: an unrelated fix folded into a closing commit costs that commit its single focus and its review its scope, and it restarts gates that were already green (`ai/rules/rule-precedence.md`).

## Verify a Commit, Not the Working Tree

**The pre-commit gate MUST run against a COMMIT in a throwaway worktree, never against the working tree (owner directive, 2026-08-21).** `./le verify worktree` adds a detached worktree at HEAD, runs every native verification stage, and removes it on every exit path. `commit <revision>` selects another commit; `keep` leaves the tree for inspection when it goes red.

**An in-place run is void the moment the tree moves under it, and the run does NOT say so.** Each stage reads the bytes present when that stage starts, so an edit landing mid-run leaves earlier stages judging a tree that no longer exists and later stages judging one the earlier ones never saw. The result reads green or red for a tree that was never committed. A red from such a run MUST NOT be diagnosed as a defect and a green MUST NOT be cited as evidence: it is void, and the answer is to re-run against a commit.

**Verification is PERIODIC: a commit waits for its focused tests, never for the gate (owner directive, 2026-08-21).** The gate costs 25 to 53 minutes on this hardware, measured on 2026-08-21 at 1486s, 1574s and 3195s (`tmp/.ze-verify-duration.txt`). A session that verifies before every commit therefore batches its work until one run is worth the wait, and that accumulation is the thing this rule exists to stop. Each finished chunk MUST land when it is finished, with its focused tests green, and the worktree gate MUST run over the resulting commits on a cadence.

**The gate therefore gates PUSHING, not committing.** A commit that stays local costs nobody anything and a commit that never happens costs the work, so a stale verify records a verification-debt row and the commit proceeds. `push "<owner authorisation>"` refuses while any row is open, which is where the debt is actually owed: a push is what reaches users. The one thing still refused at commit time is a STRUCTURAL gate red charged to the commit, because that says the tree is broken rather than merely unverified.

## Worktree Cleanup

**A worktree MUST be removed as soon as its commits are merged, rebased, or pushed, and the removal MUST clear the registration with `git worktree prune --expire now`.** A bare `git worktree prune` respects a three-month expiry, so a stale entry survives it, holds its branch checked out against rebase and deletion, and stays invisible for a quarter.

**A worktree audit MUST read the `.git` file of every directory under `.claude/worktrees/` and test that the gitdir path it names exists; `git worktree list` alone MUST NOT be treated as the answer.** A repository that moved leaves each worktree pointing at the old path, so git reports a clean tree while the checkout and its disk remain.

## Commit Ownership in Parallel Sessions (2026-07-10, owner decision)

**A FAILED commit leaves the index STAGED, and the next session's commit inherits
it. You MUST clear it before you walk away.** The script stages first and commits second, so
a failed commit can leave foreign files staged in the shared index.

**The failure mode is invisible from the failed run.** It exits non-zero, prints
`failed to write commit object`, and reads as "nothing happened". The staging IS
what happened. After ANY failed commit in a shared checkout, you MUST read
`git diff --cached --name-only`, then either fix the cause and re-run at once or
unstage your own paths. A signing failure is the usual trigger precisely because it
fails LAST, after every gate has passed and every file is already staged.

**When several sessions work the same tree, each session MUST commit the features it is in charge of implementing. You MUST NOT leave your own finished work uncommitted for another session to sweep or strand.**
**You MUST scope every commit script to your own files, one `file <path>` keyword per file, and MUST verify `git diff --cached --name-only` shows nothing foreign before running it.**

**An uncommitted improvement written by ANOTHER agent MAY be included in your commit when it is IN SCOPE of the feature you own**: it edits a file your feature owns, or your acceptance criteria depend on it. **The inclusion and its origin MUST be named in the commit body.** An out-of-scope foreign file MUST be left untouched.

**The rule above governs CODE. A concurrent session's out-of-scope NON-CODE files (generated discovery indexes, docs, tracking markdown) MAY be swept into your commit when that keeps the tree consistent or unblocks you. Foreign source and test files MUST NOT be.** The inclusion and its origin MUST be named in the commit body.
**Sweeping a file does not clear a whole-tree closure gate.** The deferral homing check folds over every shard in `plan/deferrals/`, so a foreign entry lacking a real destination spec surfaces at closure whatever you commit. You MUST home it in its own shard rather than paper over it.

**This checkout is shared by several sessions ON PURPOSE, so that concurrent work needs no merge. A session working here MUST NOT move its own work into a worktree it creates:** that reintroduces the integration step the shared tree exists to avoid. A worktree AGENT launched by the owner is a different thing and keeps its own rules.
**`git worktree add --detach <scratch-path> HEAD` MAY be used to READ a clean tree**, to establish whether a build break or a red test predates your own change. It MUST be created outside the repository, MUST NOT be written to, and MUST be removed with `git worktree remove` as soon as the read is done: a registered worktree shows up in every other session's `git worktree list`. When a single file answers the question, `git show HEAD:<path>` costs nothing and registers nothing.

## Branch Changes Are Forbidden

**Stay on the branch you started on. You MUST NOT change, create, delete, rename, or integrate a branch from a tool call.**

**These branch-changing commands MUST NOT be run: `git switch`, `git checkout <branch>`, `git branch`, `git rebase`, `git merge`, `git cherry-pick`.**

**When branch movement or integration is needed, you MUST stop and ask the user to do it manually.**

## Before Destructive Actions

**The destructive git verbs MUST NOT be run.** `ai/INSTRUCTIONS.md`, "Destructive git commands are FORBIDDEN", carries the list into every session, so it is not restated here.

**Before anything destructive you MUST save a patch, write the destructive commands to `tmp/delete-SESSION.sh`, tell the user, and STOP.** The patch is `git diff > backups/work-$(date +%Y%m%d-%H%M%S).patch`.

## Forbidden Raw Output

**`git status` and `git diff --stat` MUST NOT be dumped raw into your output. Summarise them.**

## Branch Integration

**When the user integrates a worktree branch manually, it MUST land on main via `git rebase <branch>`, never `git merge`.** History stays linear.

## GPG Signing

**`--no-gpg-sign`, `-c commit.gpgsign=false` and `--no-verify` MUST NOT be used, and a hook MUST NOT be disabled to make a commit pass.** What to do when signing fails is in `docs/contributing/committing.md`.

## Codeberg CLI

**Pull requests and issues MUST go through `tea`:** `tea pr list`, `tea pr create`, `tea issue list`, `tea issue create`.
