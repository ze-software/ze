# Git Safety

Rationale: `ai/rationale/git-safety.md`

## Commit Rules

**FORBIDDEN from AI tool calls:** `git commit`, `git add`, `git rm`,
`git restore --staged`, `git stash`. Sessions share staging -- a direct
`git add` is visible to every other session's `git commit` and files
cross-commit. Package add + commit into a single user-triggered script.

**Commit workflow:**
1. Pick an 8-char session ID (`head -c4 /dev/urandom | xxd -p`); reuse for every script + message file this session.
2. One `tmp/commit-msg-<SESSION>-<tag>.txt` per commit (plain text). **No heredocs** -- macOS bash 3.2 mis-parses backticks inside `$(cat <<'EOF')`. Always `git commit -F <file>`.
3. `tmp/commit-<SESSION>.sh`: per logical commit, one `git add <explicit files>` then `git commit -F <msg>`. Spec-preservation = two pairs (code first; `git rm <spec>` + `git add <summary>` second).
4. `chmod +x` every script you hand the user (commit, delete, helper) at creation. User runs it directly (`./tmp/...`), not via `bash`.
5. Never end an output line with `.`, `,`, `:`, or `)` directly after a path/URL/command -- users copy-paste; trailing punctuation breaks it. Put path on its own line or follow with a space.
6. Report what was done and what is left. User decides when to commit.

`git commit`/`git add` inside the script is fine -- the ban is on
direct AI tool invocations, not on what the script does when the user
runs it. `git restore --staged <file>` is allowed inside a commit
script only; all other `git restore` variants remain forbidden.

**Script format:**
```bash
#!/bin/bash
set -e
cd "$(git rev-parse --show-toplevel)"

# Commit A
git add file1.go file2.go file3_test.go
git commit -F tmp/commit-msg-<SESSION>-a.txt

# Commit B (optional; e.g., spec-preservation)
git rm plan/spec-<name>.md
git add plan/learned/NNN-<name>.md
git commit -F tmp/commit-msg-<SESSION>-b.txt
```

**Never suggest / ask / hint at committing.** Complete ALL work first
(testing, spec, docs, learned summary), then report. User decides.
Banned phrases: "ready to commit?", "shall I commit?", "we could
commit now", "want me to commit?".

**Never bypass hooks.** No `--no-verify`, no `--no-gpg-sign`. On GPG
failure (`gpg failed to sign` / `cannot open /dev/tty`), ask the user
to run `! echo test | gpg --clearsign` to unlock the agent, then
re-run the script.

## Commit Granularity

Single-focus commits: one logical change per commit. Same system =
one commit (feature + tests + docs). Multiple unrelated changes =
multiple commits, not one bundle. Unrelated bug fix = separate commit.
Review fixes from a review pass = one commit.

## Before Any Commit

### Step 0: Does `ze-verify` apply?

BLOCKING only when the commit could plausibly affect build, tests, or
generated code.

| Files in commit | Run `ze-verify`? |
|-----------------|------------------|
| Any `.go`, `go.mod`, `go.sum`, `vendor/**` | YES |
| `Makefile`, `scripts/**`, build/CI config | YES |
| `*.yang`, generated code, codegen templates | YES |
| Anything that runs at build time or affects a binary | YES |
| `ai/**/*.md`, `.claude/**/*.md`, `plan/**/*.md`, `docs/**/*.md`, `README.md` | NO |

Mixed commit: one YES row -> run. Do not split a commit to skip.
Decision rule: "could this make a Go test fail or break the build?"
No = skip and note in commit summary. Unsure = run.

### Step 1: If `ze-verify` applies (BLOCKING)

`make ze-verify` (timeout 240s). Not `go test`, not any subset.
`ze-verify` uses a two-pass strategy: cached full pass (no `-race`) +
`-race` only on component groups with changed `.go` files. For reactor
concurrency changes, also run `make ze-race-reactor`. Output
auto-captures to `tmp/ze-verify.log`; failure index in
`tmp/ze-verify-failures.log` (read that FIRST).

```
[ ] 0. `scripts/dev/verify-status.sh check`. FRESH -> skip step 1, note timestamp. STALE -> continue.
[ ] 1. `make ze-verify` (240s). On failure read tmp/ze-verify-failures.log, then tmp/ze-verify.log.
[ ] 2. Failure from current work: fix + re-run. Pre-existing: fix after primary task in separate commit; if >10 min, log to `plan/known-failures.md`.
```

### Concurrent Verify Runs (BLOCKING)

One `make ze-verify*` (or `ze-chaos-verify`) at a time repo-wide --
parallel runs share build cache + ports + `bin/ze` processes and
trash each other. All variants are wrapped by
`scripts/dev/verify-lock.sh` (`flock` on `tmp/.ze-verify.lock`); a
second invocation blocks automatically.

| Do | Don't |
|----|-------|
| Let the second invocation block | Kill the running verify |
| If the run is yours (same tree), read `tmp/ze-verify.log` instead of re-running | Delete the lockfile |
| If "waiting for lock" appears, do other work | Start `go test` / `golangci-lint` / `bin/ze-test` in parallel (bypasses lock) |

Lock releases when the command exits. `flock` is fd-backed, not
PID-backed -- no cleanup after a crash.

### Running ze-verify

Foreground with 240s timeout. No background execution, no polling
loops. Wait for completion.

### Step 2: Always

```
[ ] 3. Spec completion gate (if driven by a plan/ spec):
      [ ] Learned summary written to plan/learned/NNN-<name>.md
      [ ] Spec file staged for deletion (git rm)
      Not done -> STOP.
[ ] 4. Executive Summary Report (rules/planning.md). What was done, what is left.
```

Never commit with lint issues. Never commit without test evidence when code changed.

## Forbidden Without Permission

`git reset`, `git revert`, `git checkout -- <file>`, `git restore`,
`git stash drop`, `git push --force`.

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

Save: `git diff > backups/work-$(date +%Y%m%d-%H%M%S).patch`,
then write the destructive command(s) to `tmp/delete-SESSION.sh`,
tell the user, STOP.

## Forbidden Raw Output

`git diff --stat` / `git status` dumped raw in output -- summarise.

## Branch Integration

When the user integrates a worktree branch manually, it lands on main
via `git rebase <branch>`, never `git merge`. Linear history.

## GPG Signing

Never `--no-gpg-sign` / `-c commit.gpgsign=false`.
Never `--no-verify`.

On `gpg failed to sign` / `cannot open /dev/tty`, ask user to unlock
the agent, then retry.

## Codeberg CLI

`tea` for PRs/issues: `tea pr list`, `tea pr create`, `tea issue list`, `tea issue create`.
