# Git Safety

Rationale: `.claude/rationale/git-safety.md`

## Commit Rules

**FORBIDDEN from the Bash tool:** `git commit`, `git add`, `git rm`,
`git restore --staged`, `git stash`. Sessions share staging -- a direct
`git add` is visible to every other session's `git commit` and files
cross-commit. Package add + commit into a single user-triggered script.

**Commit workflow:**
1. Pick an 8-char session ID (`head -c4 /dev/urandom | xxd -p`); reuse for every script + message file this session.
2. One `tmp/commit-msg-<SESSION>-<tag>.txt` per commit (plain text). **No heredocs** -- macOS bash 3.2 mis-parses backticks inside `$(cat <<'EOF')`. Always `git commit -F <file>`.
3. `tmp/commit-<SESSION>.sh`: per logical commit, one `git add <explicit files>` then `git commit -F <msg>`. Spec-preservation = two pairs (code first; `git rm <spec>` + `git add <summary>` second).
4. `chmod +x` every script you hand the user (commit, delete, helper) at creation. User runs it directly (`./tmp/...`), not via `bash`.
5. Never end a chat-output line with `.`, `,`, `:`, or `)` directly after a path/URL/command -- users copy-paste; trailing punctuation breaks it. Put path on its own line or follow with a space.
6. Report what was done and what is left. User decides when to commit.

`git commit`/`git add` inside the script is fine -- the ban is on MY
direct Bash invocations, not on what the script does when the user
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

Same system = one commit (feature + tests + docs). Disjoint systems
= separate commits. Unrelated bug fix = separate commit.

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
| `.claude/**/*.md`, `plan/**/*.md`, `docs/**/*.md`, `README.md` | NO |

Mixed commit: one YES row -> run. Do not split a commit to skip.
Decision rule: "could this make a Go test fail or break the build?"
No = skip and note in commit summary. Unsure = run.

### Step 1: If `ze-verify` applies (BLOCKING)

`make ze-verify-fast` (timeout 240s). Not `go test`, not any subset.
Race detector is in `make ze-verify` (full sequential), NOT
`ze-verify-fast` -- run the full variant when touching reactor
concurrency or race-sensitive code. Output auto-captures to
`tmp/ze-verify.log`; failure index in `tmp/ze-verify-failures.log`
(read that FIRST).

```
[ ] 0. `scripts/dev/verify-status.sh check`. FRESH -> skip step 1, note timestamp. STALE -> continue.
[ ] 1. `make ze-verify-fast` (240s). On failure read tmp/ze-verify-failures.log, then tmp/ze-verify.log.
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

### Running ze-verify in the Background (BLOCKING)

Foreground with 240s timeout. No `run_in_background`, no polling
loop, no `pgrep`/`stat`/`flock`-watch. Foreground = tool result IS
completion signal. Legitimate background cases + full anti-pattern
table: `.claude/rationale/git-safety.md`.

### Step 2: Always

```
[ ] 3. Spec completion gate (if driven by a plan/ spec):
      [ ] Learned summary written to plan/learned/NNN-<name>.md
      [ ] Spec file staged for deletion (git rm)
      Not done -> STOP.
[ ] 4. Executive Summary Report (rules/planning.md). What was done, what is left. Do NOT ask to commit.
```

Forbidden: `git diff --stat` / `git status` dumped raw in output --
summarise. Never commit with lint issues. Never commit without test
evidence when code changed. `make ze-test` includes fuzz tests; run
only when needed.

## Forbidden Without Permission

`git reset`, `git revert`, `git checkout -- <file>`, `git restore`,
`git stash drop`, `git push --force`.

## Before Destructive Actions

Save: `git diff > .claude/backups/work-$(date +%Y%m%d-%H%M%S).patch`,
then write the destructive command(s) to `tmp/delete-SESSION.sh`,
tell the user, STOP.

## Branch Integration

Worktree branches land on main via `git rebase <branch>`, never
`git merge`. Linear history.

## GPG Signing Recovery

On `gpg failed to sign` / `cannot open /dev/tty`, ask user to run
`! echo test | gpg --clearsign` to unlock the agent, then retry.
Never `--no-gpg-sign` / `-c commit.gpgsign=false`.

## Codeberg CLI

`tea` for PRs/issues: `tea pr list`, `tea pr create`, `tea issue list`, `tea issue create`.
