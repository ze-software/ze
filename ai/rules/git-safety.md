# Git Safety

**When:** before any git operation, and when writing or running a commit script
**Severity:** blocking

## Directives

Rationale: `ai/rationale/git-safety.md`

## Commit Rules

See: `ai/INSTRUCTIONS.md`, "git commit, git add, git rm: FORBIDDEN as bare Bash tool calls" -- the five banned verbs, the single add + delete + commit script, and "committing outside a script is not" reach every session from there, so this rule does not restate them.

**A shared single-file plan log cross-commits even with a correct, explicit
`--file` list.** The ban on the bare staging verbs fixes staging *timing*; it
cannot fix staging *granularity*. `git add <file>` stages the WHOLE file, including hunks
another session left uncommitted in it. You MUST SHARD the log so each
session writes only files it owns and git merges disjoint creations without
conflict. **Both cross-spec logs are now sharded.** Deferrals live one file
per source under `plan/deferrals/` (`ai/rules/planning.md`), so `git
add plan/deferrals/<source>.md` stages only your row. Known failures live one
file per failure under `plan/known-failures/` (a `<make-target>-<test-name>.md`
shard, with `RESOLVED.md` archiving the history and `README.md` holding the
logging instructions), so `git add plan/known-failures/<make-target>-<test-name>.md`
stages only your entry. A shared unsharded log lets concurrent sessions stage
each other's entries.

Consequences (they apply whenever two sessions touch the same tracked file, so
keep each shard single-writer), in order of importance:

| Situation | Do |
|-----------|-----|
| Your rows in a shared plan file are already committed by someone else | Nothing. The content is correct and preserved; only attribution is off. NEVER rewrite history to reclaim it (`git revert`/`reset` are forbidden anyway). |
| You edited a shared plan file | Commit it promptly. The longer it sits, the likelier another session's commit absorbs it. |
| Your commit omits a shared plan file you edited | Check `git log -1 -- <file>` before assuming the edit was lost: another session probably committed it already. |
| You see foreign rows in a shared plan file's diff | That is expected, not misconduct. Do not "clean" them out; you would revert another session's work. |

Do not read a cross-commit as a rule violation by the other session. With
concurrent sessions and a shared single-file log it is structural.

**Explicit commit requests are a fast path.** When the user asks for a
commit, the implementation/review phase is over. You MUST prepare the commit
script and run it immediately. You MUST NOT re-audit the implementation, run late
completeness/remaining-work tables, inspect speculative companion artifacts,
or rerun lint/tests just because commit was requested. You MUST inspect only enough
state to avoid staging unrelated, ignored, generated, or out-of-scope paths.
**One check is exempt, because it cannot run earlier: `make ze-repository-tracked-build-check`
after the script has run** (step 7). It judges the commit you just made, which no
run before that commit could see.

**Thomas ruled on this exemption on 2026-08-04: KEEP IT.** It is settled, so you
MUST NOT re-open it. The check is not a rerun because its input is a commit that
did not exist until the script ran. It is not a check on the working tree
because that tree can already hold the next change. The tracked build's cost is
bounded and one-shot, while the failure it prevents is unbounded.
If scope is ambiguous, ask one narrow question; otherwise proceed.

**Commit workflow:**
1. You MUST use `scripts/dev/commit_helper.py session` to create or reuse the 8-char session ID stored in `tmp/commit-session-id-<claude-session>` (keyed per Claude session so concurrent sessions never share a message or script namespace).
2. You MUST use `scripts/dev/commit_helper.py create` to write one message file and one commit script. You MUST pass `--file` once per explicit file and `--remove` for tracked deletions. The path is the `script=` line it prints (`ai/INSTRUCTIONS.md`). One session can run many subagents that share the session id, so `--push` adds a push after the commits only on an owner instruction (see "Pushing").
   `--append` adds a later commit block to a script you already prepared. You MUST pass `--script` with the path that create printed. Without `--script` it resolves only when the session has exactly one script, and otherwise refuses with the list. `--replace` rewrites the script `--script` names. It is refused when that script was prepared for a file set sharing nothing with yours. To start over, prepare a new one: a `create` without `--script` always gets its own path.
3. The helper writes executable scripts, uses `git commit -F <message-file>`, and rejects ignored/generated paths. It never writes over an existing script unless `--script` names it, with `--replace` or `--append`. It also **reads verify-status**, and since 2026-08-21 that reading gates the PUSH rather than the commit (see "Verify a Commit, Not the Working Tree"). `create` runs `verify-status.sh check <the commit's files>`; a stale answer records a verification-debt row and the commit PROCEEDS, because a commit that stays local costs nobody anything and a commit that never happens costs the work. `--push` then refuses while any row is open. The freshness question is scoped to the `--file` list, so an edit to a path this commit does not carry leaves the verdict FRESH. **One thing is still refused at commit time**: a STRUCTURAL gate red charged to this commit -- tier, lint, vet, plugin-boundary, iface-resolution, regen-check-readonly, wiring-docs, tracked-build (`ai/rules/precommit-verify.md`). Those are deterministic and say the tree is BROKEN, which is a different fact from unverified, and `--structural-red-ok` is the owner-only escape. Full-verify coverage over a commit carrying `.go`, `go.mod`, `go.sum` or `vendor/` is a debt row on the same footing.
   It further **gates on discovery-index freshness**: `create` refuses if a generated index (`ai/PACKAGE-MAP.md`) is stale (run `make ze-generated-files-update`), or if the commit changes an index-feeding source (a `register.go`, a `.go` with a `// Package` header) but omits the regenerated index. Override with `--stale-index-ok "<reason>"`. With no CI, this is the only place index freshness is enforced. `create` additionally **warns (non-blocking)** when HEAD's committed index does not match HEAD's committed sources, which catches a prior commit that bypassed the gate; it detects this by re-running the generators against a materialized copy of HEAD, so it works even when the working tree carries unrelated uncommitted changes.
4. If the helper cannot express the commit shape, you MUST hand-write the same script pattern and `chmod +x` it. You MUST give it a name no other agent will pick: `tmp/commit-<SESSION>-<tag>-<random>.sh`. You MUST NOT use heredocs. You MUST use `git commit -F <file>`.
5. You MUST NOT end an output line with `.`, `,`, `:`, or `)` directly after a path/URL/command -- users copy-paste; trailing punctuation breaks it. You MUST put path on its own line or follow with a space.
6. You MUST run the finished script yourself with `bash` and the helper's `script=` path. **When the commit contained any `.go`, `go.mod`, `go.sum`, or `vendor/` path, you MUST run `make ze-repository-tracked-build-check` immediately afterwards** (about 45s): it compiles what git now holds, and it is the only check that reads that population -- see "Your Working Tree Is Not What You Committed" below. You MUST then report the resulting commit SHA(s), included files, message file, script path, whether the script pushed, and verification evidence or skip reason. You MUST NOT add a late completeness or remaining-work review unless the user explicitly asked for one.
7. Before writing a commit script, you MUST read `.gitignore` and MUST NOT `git add` ignored paths. Key ignored paths: `CLAUDE.md`, `AGENTS.md`, `.claude/skills/`, `.codex/skills/`, `.agents/skills/`, `tmp/`, `/bin/`. You MUST only add canonical sources (e.g., `ai/skills/`, `ai/INSTRUCTIONS.md`).
**The helper asks for no lesson artifact, and it MUST NOT be made to (owner directive, 2026-08-10).** A lesson is applied by UPDATING the surface that governs behaviour, never by saving a summary beside the commit. Route it: a recurring trap to a rule under `ai/rules/`, a design decision to `docs/architecture/`, a subsystem's data flow to `ai/digests/`, a protocol obligation to `rfc/short/`. The journal row survives for its own reason, which is counting how often a PROBLEM class recurs (`ai/rules/planning.md`, "Writing Journal Rows").

`git commit`/`git add` inside the script is fine -- the ban is on
direct AI tool invocations, not on what the script does when it runs.
Run the finished script yourself with `bash` and the printed path.
The tool call is `bash <script>`, not a bare `git commit`, so it
passes the hook that blocks the raw verbs. `git restore --staged <file>`
is allowed inside a commit script only; all other `git restore` variants
remain forbidden.

**`git rm` safety:** before using `git rm` in a commit script, you MUST verify
the file is tracked (`git ls-files --error-unmatch <file>`). For files
modified during implementation (specs, stubs), you MUST use `git rm -f` to avoid
"has local modifications" errors. You MUST NOT `git rm -f` without first
committing the file's current state (see Spec Closure in planning rules).

**You MUST use this helper format:**
```bash
# Single commit (most common):
scripts/dev/commit_helper.py create \
  --replace \
  --subject "hook: allow tee pipe, per-session log paths" \
  --body "Explanation of why the change was made." \
  --file .claude/hooks/pretool-bash.py \
  --file ai/rules/commands.md

# Second commit in the same script:
scripts/dev/commit_helper.py create \
  --append \
  --subject "feat: add widget support" \
  --body "Implements widget rendering for the dashboard." \
  --file internal/component/web/widget.go \
  --file internal/component/web/widget_test.go

# Spec closure (remove spec file):
scripts/dev/commit_helper.py create \
  --append \
  --subject "spec: close spec-widget" \
  --remove plan/spec-widget.md

# With a journal row:
scripts/dev/commit_helper.py create \
  --replace \
  --subject "rules: add goroutine lifecycle rule" \
  --file ai/rules/goroutine-lifecycle.md \
  --file plan/journal/<class>.md
```

`scripts/dev/commit_helper.py` is invoked from Bash, so `--subject`, `--body`
and every override reason are shell words BEFORE Python sees them. Inside a
double-quoted argument a backtick opens command substitution, so a body reading
``the block declares `encoder json` `` runs `encoder json`, prints
`encoder: command not found` to stderr, and substitutes the EMPTY STRING into
the message. You MUST NOT put a backtick in any commit_helper argument.

The failure is silent in the only place that matters. The helper still writes
its message file and still prints `script=`, so a caller reading the tail of the
output sees success; the sentence is already mutilated, and running the script
commits it to permanent history. Quote code in a commit message with plain
double quotes, or name the thing without quoting it at all.

Two habits make it self-checking, and both are cheap next to a bad commit
message that cannot be corrected without rewriting history:

- Read the generated message file before running the script. `create` prints its
  path on the `message=` line. A blanked backtick span is obvious there and
  nowhere else.
- Treat `command not found` anywhere in the helper's output as a failed
  invocation, not as noise from an unrelated tool.

Repairing the message file in place is the fix, and it is allowed: the script
runs `git commit -F <message-file>`, so the file is read when the script runs,
not when `create` wrote it.

Key flags: `--replace` for the first commit in a session, `--append`
for subsequent commits. `--file` per path to add, `--remove` per
tracked path to delete.
Body lines are wrapped to 72 characters. Subjects are single-line and
are at most 72 characters.

The generated script has this shape:
```bash
#!/bin/bash
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

# Commit a: type: subject line
# Lesson: not needed - hook fix, no novel pattern
git add -- \
  file1.go \
  file2.go \
  file3_test.go
git commit -F tmp/commit-msg-<SESSION>-a.txt
```

**You MUST NOT suggest / ask / hint at committing.** Complete ALL work first
(testing, spec, docs, journal row), then report. User decides.
Banned phrases: `ready to commit?`, `shall I commit?`, `we could
commit now`, `want me to commit?`.

## Pushing (2026-08-05, owner amendment)

- **A bare `git push` from a Bash call stays forbidden; the hook enforces it.**
- **You MUST push only by passing `--push` to `scripts/dev/commit_helper.py create` (step 2); it runs from the script you run at step 7.**
- **The owner orders a push; you MUST NOT decide one yourself. `--push` on your own initiative is a push without authority.**
- **`git push --force` and `-f` stay forbidden; `--push` is no route to them.**

### Why the amendment, and what to do when a push goes wrong

Thomas amended the push ban on 2026-08-05: a push is allowed only from the
commit script and only when he has ordered it. One script bundles add, remove,
commit, and push so there is no partial-staging window.

The hook's refusal of the bare command is deliberate, not an oversight: it is
what forces every push through the script, where the mechanism stays visible to
the next reader. Writing a throwaway script that carries a push and deleting it
afterwards is banned for the same reason. It reaches the same remote by the same
hand and leaves no record of why the push happened, and `ai/INSTRUCTIONS.md`
carries that ban into every session. A remote history that needs rewriting is
the owner's decision, made at his own terminal, which is why `--force` and `-f`
have no `--push` path (`ai/INSTRUCTIONS.md`, "Destructive git commands are
FORBIDDEN").

| Situation | Do |
|-----------|-----|
| The script's commit step failed | Nothing is pushed and nothing should be: `set -euo pipefail` stops the script before the push. Fix the cause (staged index, GPG), then re-run the script |
| The commits are made and no push was ordered | Stop and report the SHA(s). A push nobody asked for is a push without authority, whatever the branch looks like |
| You are a worktree agent | Never push. Work on your branch and stop there (`ai/INSTRUCTIONS.md`, "Worktree agents must not touch main") |
| The owner orders a push after your script already ran | Say so and let him push, or carry `--push` on the next commit you prepare. Do not type the command to close the gap |

## Commit Granularity

Single-focus commits: one logical change per commit. Same system =
one commit (feature + tests + docs). Multiple unrelated changes =
multiple commits, not one bundle. Unrelated bug fix = separate commit.
Review fixes from a review pass = one commit.

**A finished chunk MUST be committed when it finishes, not when the session does (owner directive, 2026-08-21).** Work that is done and green sits in one working tree, where the next `git clean`, checkout or crashed session destroys it, and where every later chunk has to be diffed around it. The question to answer after each piece of work is "does this stand on its own", never "am I finished for the day". A defect fix, a rule change, a gate repair and a spec's implementation are four commits, and the first three MUST NOT wait behind the fourth's review gate.

**Read the working tree's SPREAD before starting new work, and land what is already finished first.** `make ze-working-tree-check` reports the changed paths grouped by area. More than one area in flight means a chunk is waiting that could already be committed, and it MUST be landed before the next piece starts. The cost of getting this wrong compounds: an unrelated fix folded into a closing commit costs that commit its single focus and its review its scope, and it restarts gates that were already green (`ai/rules/rule-precedence.md`).

## Verify a Commit, Not the Working Tree

**The pre-commit gate MUST run against a COMMIT in a throwaway worktree, never against the working tree (owner directive, 2026-08-21).** `make ze-verify-worktree` does it: it adds a detached worktree at the commit, runs `ze-precommit-verify` there, and removes it on every exit path. `COMMIT=<rev>` picks the commit and defaults to HEAD; `KEEP=1` leaves the tree for inspection when it goes red.

**An in-place run is void the moment the tree moves under it, and the run does NOT say so.** Each stage reads the bytes present when that stage starts, so an edit landing mid-run leaves earlier stages judging a tree that no longer exists and later stages judging one the earlier ones never saw. The result reads green or red for a tree that was never committed. A red from such a run MUST NOT be diagnosed as a defect and a green MUST NOT be cited as evidence: it is void, and the answer is to re-run against a commit.

**Verification is PERIODIC: a commit waits for its focused tests, never for the gate (owner directive, 2026-08-21).** The gate costs 25 to 53 minutes on this hardware, measured on 2026-08-21 at 1486s, 1574s and 3195s (`tmp/.ze-verify-duration.txt`). A session that verifies before every commit therefore batches its work until one run is worth the wait, and that accumulation is the thing this rule exists to stop. Each finished chunk MUST land when it is finished, with its focused tests green, and the worktree gate MUST run over the resulting commits on a cadence.

**The gate therefore gates PUSHING, not committing.** A commit that stays local costs nobody anything and a commit that never happens costs the work, so a stale verify records a verification-debt row and the commit proceeds. `--push` refuses while any row is open, which is where the debt is actually owed: a push is what reaches users. The one thing still refused at commit time is a STRUCTURAL gate red charged to the commit -- tier, lint, vet, plugin-boundary, iface-resolution, regen-check-readonly, wiring-docs, tracked-build -- because those are deterministic and say the tree is BROKEN, which is a different fact from unverified.

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

When several sessions work the same tree, each session MUST commit the
features it is in charge of implementing -- never leave your own finished
work uncommitted for another session to sweep or strand. Scope every commit
script to your own files (explicit `--file` lists; verify
`git diff --cached --name-only` shows nothing foreign before running it).

Exception, per owner direction: an uncommitted improvement written by
ANOTHER agent may be included in your commit when it is IN SCOPE of the
feature you own (it edits a file your feature owns, or your ACs depend on
it). Name the inclusion and its origin in the commit body. Out-of-scope
foreign files stay untouched, always.

Refinement, per owner direction (2026-07-17): the "always" above governs
CODE. A concurrent session's out-of-scope NON-CODE files -- generated
discovery indexes, docs, tracking markdown -- MAY be swept into your commit
when doing so keeps the tree consistent or unblocks you; foreign source and
test files are never included. Name the inclusion and its origin in the
commit body. This does not clear the whole-tree closure gates: the deferral
homing check folds over every shard in `plan/deferrals/`, so a foreign entry
in any shard that lacks a real destination spec is surfaced at closure
regardless of what you commit -- home it in its own shard, do not paper over
it (the check is advisory, but the obligation is not).

This checkout is shared by several sessions ON PURPOSE, so that concurrent
work needs no merge. A session working here MUST NOT move its own work into
a worktree it creates: that reintroduces the integration step the shared tree
exists to avoid. (A worktree AGENT, launched into `.claude/worktrees/` by the <!-- doc-links: ignore (a local worktree root, excluded via .git/info/exclude; it never exists in a clean checkout) -->
owner, is a different thing and keeps its own rules above.)

`git worktree add --detach <scratch-path> HEAD` MAY be used to READ a clean
tree -- to establish whether a build break or a red test predates your own
change, which a dirty shared tree cannot answer on its own. It MUST be
created outside the repository, it MUST NOT be written to, and it MUST be
removed with `git worktree remove` as soon as the read is done: a registered
worktree shows up in every other session's `git worktree list`, so one left
behind is a change to shared state nobody asked for. When a single file
answers the question, `git show HEAD:<path>` costs nothing and registers
nothing.

## Branch Changes Are Forbidden

Stay on the branch you started on. Never change branches, create
branches, delete branches, rename branches, or integrate branches from
an AI tool call.

Forbidden branch-changing commands include `git switch`, `git checkout
<branch>`, `git branch`, `git rebase`, `git merge`, and
`git cherry-pick`.

If branch movement or integration is needed, stop and ask the user to do
it manually.

## Before Destructive Actions

See: `ai/INSTRUCTIONS.md`, "Destructive git commands are FORBIDDEN" -- `git reset`, `git checkout -- <file>`, `git restore`, `git clean`, `git revert`, `git push --force`, `git push -f`, `git stash drop` and `git stash clear` are banned there, in every session, and are not restated here.

Save: `git diff > backups/work-$(date +%Y%m%d-%H%M%S).patch`,
then write the destructive command(s) to `tmp/delete-SESSION.sh`,
tell the user, STOP.

## Forbidden Raw Output

`git diff --stat` / `git status` dumped raw in output -- summarise.

## Branch Integration

When the user integrates a worktree branch manually, it lands on main
via `git rebase <branch>`, never `git merge`. Linear history.

## Rebase Onto Diverged main: driving the bookkeeping conflicts

A rebase of local commits onto a diverged `origin/main` can re-conflict on
the one derivable bookkeeping file still tracked (`ai/PACKAGE-MAP.md`).
Regenerate with `make ze-discovery-index-update` at each rebase stop and continue.

Finish the rebase first, then repair bookkeeping -- never mid-rebase. Afterwards
regenerate the derived indexes with `make ze-discovery-index-update` and recompute any
derived ratchet the rebase loosened (e.g. `test/.ci-sleep-baseline` = actual
`time.sleep(` count in `test/**/*.ci`).

Gotcha: `git rebase --continue` refuses with a MISLEADING "You must edit all
merge conflicts" whenever there are unstaged tracked changes, not only when
index entries are unmerged (`builtin/rebase.c` ACTION_CONTINUE checks
`has_unstaged_changes()`). Read `git status` for the unstaged tracked files and
stage or discard them; the message names conflicts you do not have.
Per "Branch Changes Are Forbidden" above, the AI never runs `git rebase`
itself; the script only resolves conflicts within a rebase the user started.

## GPG Signing

Never `--no-gpg-sign` / `-c commit.gpgsign=false`.
Never `--no-verify`. Never disable a hook to make a commit pass.

On `gpg failed to sign` / `cannot open /dev/tty`, ask the user to run
`! echo test | gpg --clearsign` to unlock the agent, then re-run the
commit script.

## Codeberg CLI

`tea` for PRs/issues: `tea pr list`, `tea pr create`, `tea issue list`, `tea issue create`.
